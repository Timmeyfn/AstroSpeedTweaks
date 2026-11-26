//go:build windows

package main

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The FPS overlay: a small always-on-top, click-through window in a corner of the screen. It's a
// normal window on the desktop, not something injected into the game, so it's safe with anti-cheat.
// It shows over games running in windowed or borderless ("windowed fullscreen") mode. Exclusive
// fullscreen hides it, like every non-injected overlay.

var (
	procRegisterClassExW        = user32.NewProc("RegisterClassExW")
	procCreateWindowExW         = user32.NewProc("CreateWindowExW")
	procDefWindowProcW          = user32.NewProc("DefWindowProcW")
	procDestroyWindow           = user32.NewProc("DestroyWindow")
	procTranslateMessage        = user32.NewProc("TranslateMessage")
	procDispatchMessageW        = user32.NewProc("DispatchMessageW")
	procPostMessageW            = user32.NewProc("PostMessageW")
	procPostQuitMessage         = user32.NewProc("PostQuitMessage")
	procBeginPaint              = user32.NewProc("BeginPaint")
	procEndPaint                = user32.NewProc("EndPaint")
	procInvalidateRect          = user32.NewProc("InvalidateRect")
	procSetWindowPos            = user32.NewProc("SetWindowPos")
	procSetLayeredWindowAttribs = user32.NewProc("SetLayeredWindowAttributes")
	procFillRect                = user32.NewProc("FillRect")
	procSetTimer                = user32.NewProc("SetTimer")
	procIsWindowVisible         = user32.NewProc("IsWindowVisible")
	procCreateSolidBrush        = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW             = gdi32.NewProc("CreateFontW")
	procCreatePen               = gdi32.NewProc("CreatePen")
	procSelectObject            = gdi32.NewProc("SelectObject")
	procSetTextColor            = gdi32.NewProc("SetTextColor")
	procSetBkMode               = gdi32.NewProc("SetBkMode")
	procTextOutW                = gdi32.NewProc("TextOutW")
	procPolyline                = gdi32.NewProc("Polyline")
	procGetTextExtentPoint32W   = gdi32.NewProc("GetTextExtentPoint32W")
	overlayWndProcPtr           = syscall.NewCallback(overlayWndProc)
	overlayMu                   sync.Mutex
	overlayHwnd                 uintptr
	overlayClassDone            bool
	ov                          overlayView
)

// overlayView is what the window draws; overlayUpdate fills it in once a second.
type overlayView struct {
	opts    overlayOpts
	waiting bool
	fps     string
	low     string
	rows    []ovRow
	graph   []float64 // recent frame times (ms), oldest first
	w, h    int32     // window size for these rows
	// fonts and brushes for the current size
	sizeMade                int
	fBig, fVal, fLbl, fUnit uintptr
	bg, graphBg             uintptr
	penLine, penGrid        uintptr
}

type ovRow struct {
	label string
	cells []ovCell
}

type ovCell struct {
	val, unit string
	color     uint32 // 0 = normal
}

// overlayOpts: what the overlay shows, set on the Benchmark page.
type overlayOpts struct {
	Corner int             `json:"corner"`
	Size   int             `json:"size"`           // 0 small, 1 normal, 2 large
	Hide   map[string]bool `json:"hide,omitempty"` // rows switched off: fps, graph, gpu, vram, cpu, ram
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type paintStruct struct {
	Hdc       uintptr
	Erase     int32
	Rc        [4]int32
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}

const (
	wmDestroy     = 0x0002
	wmPaint       = 0x000F
	wmClose       = 0x0010
	wmEraseBkgnd  = 0x0014
	wmNcHitTest   = 0x0084
	wmTimer       = 0x0113
	wmHotkey      = 0x0312
	wmAppRefresh  = 0x8001
	hwndTopmost   = ^uintptr(0) // (HWND)-1
	swpNoMove     = 0x0002
	swpNoSize     = 0x0001
	swpNoActivate = 0x0010
	swpShowWindow = 0x0040
	ovHotkeyID    = 0x4154 // Ctrl + Shift + O shows/hides the overlay
	// colours are 0x00BBGGRR
	ovWhite = 0x00F4F2F0
	ovLabel = 0x00FF8CB4 // purple
	ovUnit  = 0x00A09890
	ovGood  = 0x0078E08C // green
	ovWarm  = 0x0040B4FF // orange
	ovHot   = 0x005A5AFF // red
	ovGraph = 0x0078E08C
)

func overlayOn() bool {
	overlayMu.Lock()
	defer overlayMu.Unlock()
	return overlayHwnd != 0
}

func ovScale(size int) float64 { return []float64{0.85, 1, 1.25}[clampInt(size, 0, 2)] }

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func sc(v float64, k float64) int32 { return int32(v*k + 0.5) }

// makeGDI creates fonts and brushes for the overlay size (on the overlay's own thread).
func (v *overlayView) makeGDI() {
	if v.fBig != 0 && v.sizeMade == v.opts.Size {
		return
	}
	for _, o := range []uintptr{v.fBig, v.fVal, v.fLbl, v.fUnit} {
		if o != 0 {
			procDeleteObject.Call(o)
		}
	}
	k := ovScale(v.opts.Size)
	face, _ := windows.UTF16PtrFromString("Bahnschrift")
	mk := func(px float64, weight uintptr) uintptr {
		f, _, _ := procCreateFontW.Call(uintptr(-sc(px, k)), 0, 0, 0, weight, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
		return f
	}
	v.fBig, v.fVal, v.fLbl, v.fUnit = mk(30, 700), mk(17, 600), mk(14, 700), mk(12, 400)
	if v.bg == 0 {
		v.bg, _, _ = procCreateSolidBrush.Call(0x00140F0E)
		v.graphBg, _, _ = procCreateSolidBrush.Call(0x00221A19)
		v.penLine, _, _ = procCreatePen.Call(0, 2, ovGraph)
		v.penGrid, _, _ = procCreatePen.Call(2, 1, 0x00403634) // dotted
	}
	v.sizeMade = v.opts.Size
}

// layout works out the window size for the current rows.
func (v *overlayView) layout() {
	k := ovScale(v.opts.Size)
	h := 10.0
	if v.waiting {
		h += 44
	} else {
		if !v.opts.Hide["fps"] {
			h += 36
		}
		h += float64(len(v.rows)) * 22
		if !v.opts.Hide["graph"] && len(v.graph) > 2 {
			h += 50
		}
	}
	v.w, v.h = sc(340, k), sc(h+6, k)
}

func ovText(hdc uintptr, x, y int32, s string, font uintptr, color uint32) int32 {
	if s == "" {
		return 0
	}
	procSelectObject.Call(hdc, font)
	procSetTextColor.Call(hdc, uintptr(color))
	t, _ := windows.UTF16FromString(s)
	procTextOutW.Call(hdc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&t[0])), uintptr(len(t)-1))
	var sz [2]int32
	procGetTextExtentPoint32W.Call(hdc, uintptr(unsafe.Pointer(&t[0])), uintptr(len(t)-1), uintptr(unsafe.Pointer(&sz)))
	return sz[0]
}

func (v *overlayView) paint(hdc uintptr) {
	k := ovScale(v.opts.Size)
	rc := [4]int32{0, 0, v.w, v.h}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), v.bg)
	procSetBkMode.Call(hdc, 1) // transparent
	x0, y := sc(12, k), sc(8, k)
	if v.waiting {
		ovText(hdc, x0, y, "ASTRO TWEAKS", v.fLbl, ovLabel)
		ovText(hdc, x0, y+sc(20, k), "Waiting for a game…", v.fVal, ovWhite)
		return
	}
	if !v.opts.Hide["fps"] {
		w := ovText(hdc, x0, y, v.fps, v.fBig, ovWhite)
		w2 := ovText(hdc, x0+w+sc(6, k), y+sc(13, k), "FPS", v.fLbl, ovLabel)
		ovText(hdc, x0+w+w2+sc(16, k), y+sc(13, k), v.low, v.fUnit, ovUnit)
		y += sc(36, k)
	}
	for _, r := range v.rows {
		ovText(hdc, x0, y+sc(2, k), r.label, v.fLbl, ovLabel)
		x := x0 + sc(52, k)
		for _, c := range r.cells {
			col := uint32(ovWhite)
			if c.color != 0 {
				col = c.color
			}
			w := ovText(hdc, x, y, c.val, v.fVal, col)
			w2 := ovText(hdc, x+w+sc(2, k), y+sc(3, k), c.unit, v.fUnit, ovUnit)
			x += w + w2 + sc(14, k)
		}
		y += sc(22, k)
	}
	if !v.opts.Hide["graph"] && len(v.graph) > 2 {
		gx, gy, gw, gh := x0, y+sc(4, k), v.w-2*x0, sc(40, k)
		grc := [4]int32{gx, gy, gx + gw, gy + gh}
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&grc)), v.graphBg)
		top := 0.0
		for _, f := range v.graph {
			if f > top {
				top = f
			}
		}
		if top < 20 {
			top = 20
		}
		if top > 60 {
			top = 60
		}
		pts := make([][2]int32, len(v.graph))
		for i, f := range v.graph {
			if f > top {
				f = top
			}
			pts[i] = [2]int32{gx + int32(float64(gw-1)*float64(i)/float64(len(v.graph)-1)), gy + gh - 2 - int32(float64(gh-4)*f/top)}
		}
		procSelectObject.Call(hdc, v.penLine)
		procPolyline.Call(hdc, uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
		ovText(hdc, gx+sc(4, k), gy+sc(2, k), "FRAME TIME", v.fUnit, ovUnit)
	}
}

func overlayWndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmNcHitTest:
		return ^uintptr(0) // HTTRANSPARENT: clicks go to the game
	case wmEraseBkgnd:
		return 1
	case wmTimer:
		procSetWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate) // stay on top of the game
		return 0
	case wmHotkey:
		if wparam == ovHotkeyID {
			if vis, _, _ := procIsWindowVisible.Call(hwnd); vis != 0 {
				procShowWindow.Call(hwnd, 0) // SW_HIDE
			} else {
				procShowWindow.Call(hwnd, 4) // SW_SHOWNOACTIVATE
			}
		}
		return 0
	case wmAppRefresh:
		overlayMu.Lock()
		ov.makeGDI()
		oldW, oldH := ov.w, ov.h
		ov.layout()
		w, h, corner := ov.w, ov.h, ov.opts.Corner
		overlayMu.Unlock()
		if w != oldW || h != oldH {
			x, y := overlayPos(corner, w, h)
			procSetWindowPos.Call(hwnd, hwndTopmost, uintptr(x), uintptr(y), uintptr(w), uintptr(h), swpNoActivate)
		}
		procInvalidateRect.Call(hwnd, 0, 1)
		return 0
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		overlayMu.Lock()
		ov.makeGDI()
		ov.paint(hdc)
		overlayMu.Unlock()
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmClose:
		procUnregisterHotKey.Call(hwnd, ovHotkeyID)
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return r
}

// overlayPos: corner 0 top-left, 1 top-right, 2 bottom-left, 3 bottom-right
func overlayPos(corner int, w, h int32) (int32, int32) {
	sw, _, _ := procGetSystemMetrics.Call(0) // SM_CXSCREEN
	sh, _, _ := procGetSystemMetrics.Call(1)
	x, y := int32(14), int32(14)
	if corner == 1 || corner == 3 {
		x = int32(sw) - w - 14
	}
	if corner == 2 || corner == 3 {
		y = int32(sh) - h - 60
	}
	return x, y
}

func overlayShow(o overlayOpts) error {
	overlayMu.Lock()
	ov.opts = o
	if overlayHwnd != 0 {
		h := overlayHwnd
		overlayMu.Unlock()
		procPostMessageW.Call(h, wmAppRefresh, 0, 0)
		procShowWindow.Call(h, 4)
		return nil
	}
	ov.waiting = true
	ov.layout()
	w, h := ov.w, ov.h
	overlayMu.Unlock()
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer func() {
			if r := recover(); r != nil {
				logf("Overlay stopped: %v", r)
			}
			overlayMu.Lock()
			overlayHwnd = 0
			overlayMu.Unlock()
		}()
		inst, _, _ := procGetModuleHandleW.Call(0)
		cls, _ := windows.UTF16PtrFromString("AstroTweaksOverlay")
		if !overlayClassDone {
			wc := wndClassEx{WndProc: overlayWndProcPtr, Instance: inst, ClassName: cls}
			wc.Size = uint32(unsafe.Sizeof(wc))
			if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
				ready <- fmt.Errorf("couldn't make the overlay (%v)", err)
				return
			}
			overlayClassDone = true
		}
		x, y := overlayPos(o.Corner, w, h)
		const exStyle = 0x00000008 | 0x00080000 | 0x00000020 | 0x00000080 | 0x08000000 // TOPMOST|LAYERED|TRANSPARENT|TOOLWINDOW|NOACTIVATE
		title, _ := windows.UTF16PtrFromString("Astro Tweaks overlay")
		hwnd, _, err := procCreateWindowExW.Call(exStyle, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)), 0x80000000, // WS_POPUP
			uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0, 0, inst, 0)
		if hwnd == 0 {
			ready <- fmt.Errorf("couldn't show the overlay (%v)", err)
			return
		}
		procSetLayeredWindowAttribs.Call(hwnd, 0, 225, 2) // LWA_ALPHA: slightly see-through
		const modCtrl, modShift, modNoRepeat = 0x2, 0x4, 0x4000
		if r, _, _ := procRegisterHotKey.Call(hwnd, ovHotkeyID, modCtrl|modShift|modNoRepeat, 'O'); r == 0 {
			logf("Overlay: Ctrl+Shift+O is used by another app, so the show/hide key is off")
		}
		procShowWindow.Call(hwnd, 4) // SW_SHOWNOACTIVATE
		procSetTimer.Call(hwnd, 1, 2000, 0)
		overlayMu.Lock()
		overlayHwnd = hwnd
		overlayMu.Unlock()
		ready <- nil
		var m winMsg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}()
	select {
	case err := <-ready:
		return err
	case <-time.After(3 * time.Second):
		return errors.New("the overlay didn't start")
	}
}

func overlayHide() {
	overlayMu.Lock()
	h := overlayHwnd
	overlayMu.Unlock()
	if h != 0 {
		procPostMessageW.Call(h, wmClose, 0, 0)
	}
}

func tempColor(c, warm, hot float64) uint32 {
	switch {
	case c >= hot:
		return ovHot
	case c >= warm:
		return ovWarm
	}
	return ovGood
}

func f0(v float64) string { return fmt.Sprintf("%.0f", v) }

// overlayUpdate turns a monitor sample into the rows the overlay draws (Afterburner-style).
func overlayUpdate(s benchSample) {
	overlayMu.Lock()
	h := overlayHwnd
	if h == 0 {
		overlayMu.Unlock()
		return
	}
	hide := ov.opts.Hide
	ov.waiting = s.PID == 0 || s.FPS == 0
	if !ov.waiting {
		ov.fps = f0(s.FPS)
		ov.low = ""
		if s.Low1 > 0 {
			ov.low = "1% LOW " + f0(s.Low1) + "   " + fmt.Sprintf("%.1f ms", s.FrameMs)
		}
		var rows []ovRow
		if !hide["gpu"] {
			r := ovRow{label: "GPU"}
			if s.GPU >= 0 {
				r.cells = append(r.cells, ovCell{val: f0(s.GPU), unit: "%"})
			} else {
				r.cells = append(r.cells, ovCell{val: "–"})
			}
			if t := s.Sensors.TempC; t >= 0 {
				r.cells = append(r.cells, ovCell{val: f0(t), unit: "°C", color: tempColor(t, 75, 85)})
			}
			if s.Sensors.ClockMHz > 0 {
				r.cells = append(r.cells, ovCell{val: f0(s.Sensors.ClockMHz), unit: "MHz"})
			}
			if s.Sensors.PowerW > 0 {
				r.cells = append(r.cells, ovCell{val: f0(s.Sensors.PowerW), unit: "W"})
			}
			rows = append(rows, r)
		}
		if !hide["vram"] && s.Sensors.VRAMUsed >= 0 {
			r := ovRow{label: "VRAM", cells: []ovCell{{val: fmt.Sprintf("%.1f", s.Sensors.VRAMUsed), unit: "GB"}}}
			if s.Sensors.VRAMTot > 0 {
				r.cells[0].unit = fmt.Sprintf("/ %.0f GB", s.Sensors.VRAMTot)
			}
			rows = append(rows, r)
		}
		if !hide["cpu"] {
			r := ovRow{label: "CPU", cells: []ovCell{{val: f0(s.CPU), unit: "%"}}}
			if s.CPUTemp >= 0 {
				r.cells = append(r.cells, ovCell{val: f0(s.CPUTemp), unit: "°C", color: tempColor(s.CPUTemp, 80, 90)})
			}
			rows = append(rows, r)
		}
		if !hide["ram"] {
			rows = append(rows, ovRow{label: "RAM", cells: []ovCell{{val: fmt.Sprintf("%.1f", s.RAMUsed), unit: "GB"}, {val: f0(s.RAMPct), unit: "%"}}})
		}
		ov.rows = rows
		ov.graph = append(ov.graph, s.Frames...)
		if len(ov.graph) > 600 {
			ov.graph = ov.graph[len(ov.graph)-600:]
		}
	} else {
		ov.graph = nil
	}
	overlayMu.Unlock()
	procPostMessageW.Call(h, wmAppRefresh, 0, 0)
}

func overlayOptsNow() overlayOpts {
	stateMu.Lock()
	defer stateMu.Unlock()
	return overlayOpts{Corner: appState.OverlayCorner, Size: appState.OverlaySize, Hide: appState.OverlayHide}
}

// setOverlay is the switch and options on the Benchmark page.
func setOverlay(on bool, o overlayOpts) error {
	stateMu.Lock()
	appState.Overlay, appState.OverlayCorner, appState.OverlaySize, appState.OverlayHide = on, o.Corner, o.Size, o.Hide
	saveStateLocked()
	stateMu.Unlock()
	if !on {
		overlayHide()
		return nil
	}
	if err := benchStart(); err != nil {
		return err
	}
	return overlayShow(o)
}

// overlayOnStart brings the overlay back if it was on last time.
func overlayOnStart() {
	defer func() { _ = recover() }()
	stateMu.Lock()
	on := appState.Overlay
	stateMu.Unlock()
	if !on || !isProUnlocked() || !isElevated() {
		return
	}
	time.Sleep(3 * time.Second)
	if err := setOverlay(true, overlayOptsNow()); err != nil {
		logf("Overlay didn't start: %v", err)
	}
}
