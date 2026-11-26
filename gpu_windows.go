//go:build windows

package main

import (
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// GPU usage comes from Windows' own performance counters (the same numbers Task Manager shows).
// NVIDIA cards also report temperature, clock and power through the driver's NVML library.

var (
	pdh                             = windows.NewLazySystemDLL("pdh.dll")
	procPdhOpenQueryW               = pdh.NewProc("PdhOpenQueryW")
	procPdhAddEnglishCounterW       = pdh.NewProc("PdhAddEnglishCounterW")
	procPdhCollectQueryData         = pdh.NewProc("PdhCollectQueryData")
	procPdhGetFormattedCounterArray = pdh.NewProc("PdhGetFormattedCounterArrayW")

	gpuMu      sync.Mutex
	gpuQuery   uintptr
	gpuCounter uintptr
	gpuReady   bool
)

const (
	pdhFmtDouble   = 0x00000200
	pdhMoreData    = 0x800007D2
	pdhCstatusOK   = 0
	pdhCstatusNew  = 1
	pdhNoCountData = 0x800007D8
)

type pdhFmtItem struct {
	Name    *uint16
	CStatus uint32
	_       uint32
	Value   float64
}

func gpuInit() bool {
	if gpuReady {
		return true
	}
	if procPdhOpenQueryW.Find() != nil {
		return false
	}
	var q uintptr
	if r, _, _ := procPdhOpenQueryW.Call(0, 0, uintptr(unsafe.Pointer(&q))); r != 0 {
		return false
	}
	path, _ := windows.UTF16PtrFromString(`\GPU Engine(*engtype_3D)\Utilization Percentage`)
	var c uintptr
	if r, _, _ := procPdhAddEnglishCounterW.Call(q, uintptr(unsafe.Pointer(path)), 0, uintptr(unsafe.Pointer(&c))); r != 0 {
		return false
	}
	procPdhCollectQueryData.Call(q)
	gpuQuery, gpuCounter, gpuReady = q, c, true
	return true
}

// gpuUsage returns the total 3D engine use (0-100), and the part used by one process (pid, or 0).
func gpuUsage(pid uint32) (total, forPid float64) {
	defer func() { _ = recover() }()
	gpuMu.Lock()
	defer gpuMu.Unlock()
	if !gpuInit() {
		return -1, -1
	}
	if r, _, _ := procPdhCollectQueryData.Call(gpuQuery); r != 0 {
		return -1, -1
	}
	var size, count uint32
	r, _, _ := procPdhGetFormattedCounterArray.Call(gpuCounter, pdhFmtDouble, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)), 0)
	if uint32(r) != pdhMoreData || size == 0 {
		return -1, -1
	}
	buf := make([]byte, size)
	r, _, _ = procPdhGetFormattedCounterArray.Call(gpuCounter, pdhFmtDouble, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&buf[0])))
	if r != 0 {
		return -1, -1
	}
	items := unsafe.Slice((*pdhFmtItem)(unsafe.Pointer(&buf[0])), count)
	tag := ""
	if pid != 0 {
		tag = "pid_" + itoa(int(pid)) + "_"
	}
	for _, it := range items {
		if it.CStatus != pdhCstatusOK && it.CStatus != pdhCstatusNew {
			continue
		}
		total += it.Value
		if tag != "" && strings.Contains(windows.UTF16PtrToString(it.Name), tag) {
			forPid += it.Value
		}
	}
	if total > 100 {
		total = 100
	}
	if forPid > 100 {
		forPid = 100
	}
	return total, forPid
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// ── NVIDIA: temperature, clock, power ──

var (
	nvmlOnce   sync.Once
	nvmlOK     bool
	nvmlDev    uintptr
	nvmlTemp   *windows.LazyProc
	nvmlClock  *windows.LazyProc
	nvmlPower  *windows.LazyProc
	nvmlMemory *windows.LazyProc
)

func nvmlInit() bool {
	nvmlOnce.Do(func() {
		defer func() { _ = recover() }()
		dll := windows.NewLazySystemDLL("nvml.dll") // installed with NVIDIA drivers in System32
		if dll.Load() != nil {
			return
		}
		if r, _, _ := dll.NewProc("nvmlInit_v2").Call(); r != 0 {
			return
		}
		var dev uintptr
		if r, _, _ := dll.NewProc("nvmlDeviceGetHandleByIndex_v2").Call(0, uintptr(unsafe.Pointer(&dev))); r != 0 {
			return
		}
		nvmlDev = dev
		nvmlTemp = dll.NewProc("nvmlDeviceGetTemperature")
		nvmlClock = dll.NewProc("nvmlDeviceGetClockInfo")
		nvmlPower = dll.NewProc("nvmlDeviceGetPowerUsage")
		nvmlMemory = dll.NewProc("nvmlDeviceGetMemoryInfo")
		nvmlOK = true
	})
	return nvmlOK
}

type gpuSensors struct {
	TempC    float64 `json:"tempC"`
	ClockMHz float64 `json:"clockMHz"`
	PowerW   float64 `json:"powerW"`
	VRAMUsed float64 `json:"vramUsedGB"`
	VRAMTot  float64 `json:"vramTotalGB"`
}

func readGPUSensors() gpuSensors {
	s := gpuSensors{TempC: -1, ClockMHz: -1, PowerW: -1, VRAMUsed: -1, VRAMTot: -1}
	defer func() { _ = recover() }()
	if !nvmlInit() {
		return s
	}
	var t uint32
	if r, _, _ := nvmlTemp.Call(nvmlDev, 0, uintptr(unsafe.Pointer(&t))); r == 0 {
		s.TempC = float64(t)
	}
	var mhz uint32
	if r, _, _ := nvmlClock.Call(nvmlDev, 0, uintptr(unsafe.Pointer(&mhz))); r == 0 { // 0 = graphics clock
		s.ClockMHz = float64(mhz)
	}
	var mw uint32
	if r, _, _ := nvmlPower.Call(nvmlDev, uintptr(unsafe.Pointer(&mw))); r == 0 {
		s.PowerW = float64(mw) / 1000
	}
	var mem struct{ Total, Free, Used uint64 }
	if r, _, _ := nvmlMemory.Call(nvmlDev, uintptr(unsafe.Pointer(&mem))); r == 0 && mem.Total > 0 {
		s.VRAMUsed, s.VRAMTot = float64(mem.Used)/(1<<30), float64(mem.Total)/(1<<30)
	}
	return s
}
