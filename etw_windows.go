//go:build windows

package main

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Frame counting without touching the game: Windows itself reports every frame a program shows
// ("Present" events from DirectX) through Event Tracing for Windows (ETW). This is the same source
// Microsoft's PresentMon and most FPS tools use. Nothing is injected into the game, so anti-cheat
// has nothing to see. Needs administrator (the app already runs as admin).
//
// Covers DirectX 9, 10, 11 and 12 (DXGI). Vulkan and OpenGL games aren't counted.

var (
	advapi32etw          = windows.NewLazySystemDLL("advapi32.dll")
	procStartTraceW      = advapi32etw.NewProc("StartTraceW")
	procControlTraceW    = advapi32etw.NewProc("ControlTraceW")
	procEnableTraceEx2   = advapi32etw.NewProc("EnableTraceEx2")
	procOpenTraceW       = advapi32etw.NewProc("OpenTraceW")
	procProcessTrace     = advapi32etw.NewProc("ProcessTrace")
	procCloseTrace       = advapi32etw.NewProc("CloseTrace")
	etwSessionName       = "AstroTweaksFrameCounter"
	guidDXGI             = windows.GUID{Data1: 0xCA11C036, Data2: 0x0102, Data3: 0x4A2D, Data4: [8]byte{0xA6, 0xAD, 0xF0, 0x3C, 0xFE, 0xD5, 0xD3, 0xC9}}
	guidD3D9             = windows.GUID{Data1: 0x783ACA0A, Data2: 0x790E, Data3: 0x4D7F, Data4: [8]byte{0x84, 0x51, 0xAA, 0x85, 0x05, 0x11, 0xC6, 0xB9}}
	invalidTraceHandle   = ^uint64(0)
	etwRecordCallbackPtr = syscall.NewCallback(etwOnEvent)
)

const (
	wnodeFlagTracedGUID       = 0x00020000
	eventTraceRealTimeMode    = 0x00000100
	eventTraceControlStop     = 1
	enableProviderCode        = 1
	traceLevelVerbose         = 5
	processTraceRealTime      = 0x00000100
	processTraceEventRecord   = 0x10000000
	errAlreadyExists          = 183
	dxgiPresentStart          = 42
	dxgiPresentMultiplaneOver = 55
	d3d9PresentStart          = 1
)

// EVENT_TRACE_PROPERTIES (120 bytes on 64-bit) followed by room for the session name.
type etwProps struct {
	// WNODE_HEADER
	BufferSize        uint32
	ProviderID        uint32
	HistoricalContext uint64
	TimeStamp         int64
	GUID              windows.GUID
	ClientContext     uint32
	Flags             uint32
	// rest
	BufferSizeKB        uint32
	MinimumBuffers      uint32
	MaximumBuffers      uint32
	MaximumFileSize     uint32
	LogFileMode         uint32
	FlushTimer          uint32
	EnableFlags         uint32
	AgeLimit            int32
	NumberOfBuffers     uint32
	FreeBuffers         uint32
	EventsLost          uint32
	BuffersWritten      uint32
	LogBuffersLost      uint32
	RealTimeBuffersLost uint32
	LoggerThreadID      uintptr
	LogFileNameOffset   uint32
	LoggerNameOffset    uint32
}

type etwPropsBuf struct {
	P    etwProps
	Name [256]uint16
}

func newEtwProps() *etwPropsBuf {
	b := &etwPropsBuf{}
	b.P.BufferSize = uint32(unsafe.Sizeof(*b))
	b.P.Flags = wnodeFlagTracedGUID
	b.P.ClientContext = 1 // QueryPerformanceCounter timestamps
	b.P.LogFileMode = eventTraceRealTimeMode
	b.P.BufferSizeKB = 64
	b.P.MinimumBuffers = 8
	b.P.MaximumBuffers = 64
	b.P.FlushTimer = 1
	b.P.LoggerNameOffset = uint32(unsafe.Offsetof(b.Name))
	return b
}

// EVENT_TRACE_LOGFILEW (448 bytes on 64-bit).
type etwLogfile struct {
	LogFileName         *uint16
	LoggerName          *uint16
	CurrentTime         int64
	BuffersRead         uint32
	ProcessTraceMode    uint32
	CurrentEvent        [88]byte
	LogfileHeader       [280]byte
	BufferCallback      uintptr
	BufferSize          uint32
	Filled              uint32
	EventsLost          uint32
	EventRecordCallback uintptr
	IsKernelTrace       uint32
	Context             uintptr
}

// The start of EVENT_RECORD: EVENT_HEADER (80 bytes). Only these fields are read.
type etwEventHeader struct {
	Size          uint16
	HeaderType    uint16
	Flags         uint16
	EventProperty uint16
	ThreadID      uint32
	ProcessID     uint32
	TimeStamp     int64 // 100 ns units (converted by ETW)
	ProviderID    windows.GUID
	ID            uint16
	Version       uint8
	Channel       uint8
	Level         uint8
	Opcode        uint8
	Task          uint16
	Keyword       uint64
}

func init() {
	// guard against a wrong layout ever shipping
	if unsafe.Sizeof(etwProps{}) != 120 || unsafe.Sizeof(etwLogfile{}) != 448 || unsafe.Offsetof(etwEventHeader{}.ID) != 40 {
		etwLayoutOK = false
	}
}

var etwLayoutOK = true

// ── frame log: present timestamps per process ──

type frameLog struct {
	ts []int64 // 100 ns
}

var (
	framesMu  sync.Mutex
	framesBy  = map[uint32]*frameLog{}
	etwActive bool
	etwHandle uint64 // session
	etwTrace  uint64 // consumer
	etwErr    string
)

const maxFramesKept = 60000 // ~5 minutes at 200 FPS

func etwOnEvent(rec *etwEventHeader) uintptr {
	if rec == nil {
		return 0
	}
	p := rec.ProviderID
	ok := (p == guidDXGI && (rec.ID == dxgiPresentStart || rec.ID == dxgiPresentMultiplaneOver)) ||
		(p == guidD3D9 && rec.ID == d3d9PresentStart)
	if !ok {
		return 0
	}
	framesMu.Lock()
	fl := framesBy[rec.ProcessID]
	if fl == nil {
		fl = &frameLog{}
		framesBy[rec.ProcessID] = fl
	}
	fl.ts = append(fl.ts, rec.TimeStamp)
	if len(fl.ts) > maxFramesKept {
		fl.ts = append(fl.ts[:0], fl.ts[len(fl.ts)-maxFramesKept/2:]...)
	}
	framesMu.Unlock()
	return 0
}

func stopStaleSession() {
	p := newEtwProps()
	name, _ := windows.UTF16PtrFromString(etwSessionName)
	procControlTraceW.Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(p)), eventTraceControlStop)
}

// startFrameCounter starts the ETW session (once). Safe to call many times.
func startFrameCounter() error {
	framesMu.Lock()
	if etwActive {
		framesMu.Unlock()
		return nil
	}
	framesMu.Unlock()
	if !etwLayoutOK {
		return errors.New("the frame counter isn't supported on this system")
	}
	if !isElevated() {
		return errors.New("Astro Tweaks needs to run as administrator to count frames")
	}
	stopStaleSession()
	name, _ := windows.UTF16PtrFromString(etwSessionName)
	p := newEtwProps()
	var h uint64
	r, _, _ := procStartTraceW.Call(uintptr(unsafe.Pointer(&h)), uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(p)))
	if r == errAlreadyExists {
		stopStaleSession()
		p = newEtwProps()
		r, _, _ = procStartTraceW.Call(uintptr(unsafe.Pointer(&h)), uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(p)))
	}
	if r != 0 {
		return fmt.Errorf("couldn't start the frame counter (error %d)", r)
	}
	for _, g := range []windows.GUID{guidDXGI, guidD3D9} {
		g := g
		procEnableTraceEx2.Call(uintptr(h), uintptr(unsafe.Pointer(&g)), enableProviderCode, traceLevelVerbose, ^uintptr(0), 0, 0, 0)
	}
	lf := &etwLogfile{LoggerName: name, ProcessTraceMode: processTraceRealTime | processTraceEventRecord, EventRecordCallback: etwRecordCallbackPtr}
	th, _, _ := procOpenTraceW.Call(uintptr(unsafe.Pointer(lf)))
	if uint64(th) == invalidTraceHandle {
		procControlTraceW.Call(uintptr(h), 0, uintptr(unsafe.Pointer(newEtwProps())), eventTraceControlStop)
		return errors.New("couldn't open the frame counter")
	}
	framesMu.Lock()
	etwActive, etwHandle, etwTrace, etwErr = true, h, uint64(th), ""
	framesBy = map[uint32]*frameLog{}
	framesMu.Unlock()
	go func() {
		runtime.LockOSThread()
		defer func() { _ = recover() }()
		t := uint64(th)
		r, _, _ := procProcessTrace.Call(uintptr(unsafe.Pointer(&t)), 1, 0, 0) // blocks until stopped
		framesMu.Lock()
		etwActive = false
		if r != 0 && r != 1223 { // 1223 = cancelled (normal stop)
			etwErr = fmt.Sprintf("the frame counter stopped (error %d)", r)
		}
		framesMu.Unlock()
		_ = lf // keep the logfile struct alive while ProcessTrace runs
	}()
	logf("Frame counter started")
	return nil
}

func stopFrameCounter() {
	framesMu.Lock()
	active, h, t := etwActive, etwHandle, etwTrace
	framesMu.Unlock()
	if !active {
		return
	}
	procCloseTrace.Call(uintptr(t))
	procControlTraceW.Call(uintptr(h), 0, uintptr(unsafe.Pointer(newEtwProps())), eventTraceControlStop)
	framesMu.Lock()
	etwActive = false
	framesBy = map[uint32]*frameLog{}
	framesMu.Unlock()
	logf("Frame counter stopped")
}

// frameTimesSince returns frame times (ms) for a process from the given timestamp on.
func frameTimesSince(pid uint32, since int64) (ms []float64, last int64) {
	framesMu.Lock()
	defer framesMu.Unlock()
	fl := framesBy[pid]
	if fl == nil || len(fl.ts) < 2 {
		return nil, since
	}
	prev := int64(0)
	for _, t := range fl.ts {
		if t <= since {
			prev = t
			continue
		}
		if prev != 0 {
			ms = append(ms, float64(t-prev)/10000)
		}
		prev = t
	}
	return ms, fl.ts[len(fl.ts)-1]
}

// presentingProcesses lists processes that showed frames recently, with their FPS over the last second.
func presentingProcesses(now int64) map[uint32]float64 {
	framesMu.Lock()
	defer framesMu.Unlock()
	out := map[uint32]float64{}
	for pid, fl := range framesBy {
		n := 0
		for i := len(fl.ts) - 1; i >= 0 && fl.ts[i] > now-10_000_000; i-- {
			n++
		}
		if n > 0 {
			out[pid] = float64(n)
		}
	}
	return out
}
