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
	procCreateSolidBrush        = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW             = gdi32.NewProc("CreateFontW")
	procSelectObject            = gdi32.NewProc("SelectObject")
	procSetTextColor            = gdi32.NewProc("SetTextColor")
	procSetBkMode               = gdi32.NewProc("SetBkMode")
	procTextOutW                = gdi32.NewProc("TextOutW")
	overlayWndProcPtr           = syscall.NewCallback(overlayWndProc)
	overlayMu                   sync.Mutex
	overlayHwnd                 uintptr
	overlayLines                []overlayLine
	overlayFontBig, overlayFont uintptr
	overlayBrush                uintptr
	overlayClassDone            bool
)

type overlayLine struct {
	text  string
	color uint32 // 0x00BBGGRR
	big   bool
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
	wmAppRefresh  = 0x8001
	overlayW      = 210
	overlayH      = 96
	hwndTopmost   = ^uintptr(0) // (HWND)-1
	swpNoMove     = 0x0002
	swpNoSize     = 0x0001
	swpNoActivate = 0x0010
	swpShowWindow = 0x0040
)

func overlayOn() bool {
	overlayMu.Lock()
	defer overlayMu.Unlock()
	return overlayHwnd != 0
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
	case wmAppRefresh:
		procInvalidateRect.Call(hwnd, 0, 1)
		return 0
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		rc := [4]int32{0, 0, overlayW, overlayH}
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), overlayBrush)
		procSetBkMode.Call(hdc, 1) // transparent
		overlayMu.Lock()
		lines := append([]overlayLine(nil), overlayLines...)
		overlayMu.Unlock()
		y := int32(8)
		for _, l := range lines {
			f := overlayFont
			h := int32(20)
			if l.big {
				f, h = overlayFontBig, 30
			}
			procSelectObject.Call(hdc, f)
			procSetTextColor.Call(hdc, uintptr(l.color))
			t, _ := windows.UTF16FromString(l.text)
			procTextOutW.Call(hdc, 12, uintptr(y), uintptr(unsafe.Pointer(&t[0])), uintptr(len(t)-1))
			y += h
		}
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return r
}

// overlayCorner: 0 top-left, 1 top-right, 2 bottom-left, 3 bottom-right
func overlayPos(corner int) (int32, int32) {
	sw, _, _ := procGetSystemMetrics.Call(0) // SM_CXSCREEN
	sh, _, _ := procGetSystemMetrics.Call(1)
	x, y := int32(14), int32(14)
	if corner == 1 || corner == 3 {
		x = int32(sw) - overlayW - 14
	}
	if corner == 2 || corner == 3 {
		y = int32(sh) - overlayH - 60
	}
	return x, y
}

func overlayShow(corner int) error {
	overlayMu.Lock()
	if overlayHwnd != 0 {
		h := overlayHwnd
		overlayMu.Unlock()
		x, y := overlayPos(corner)
		procSetWindowPos.Call(h, hwndTopmost, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoActivate|swpShowWindow)
		return nil
	}
	overlayLines = []overlayLine{{"Astro Tweaks", 0x00FF7CA0, false}, {"Waiting for a game…", 0x00E6E6E6, false}}
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
			overlayBrush, _, _ = procCreateSolidBrush.Call(0x00201614)
			face, _ := windows.UTF16PtrFromString("Segoe UI")
			overlayFontBig, _, _ = procCreateFontW.Call(^uintptr(25), 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face))) // height -26
			overlayFont, _, _ = procCreateFontW.Call(^uintptr(15), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))    // height -16
		}
		x, y := overlayPos(corner)
		const exStyle = 0x00000008 | 0x00080000 | 0x00000020 | 0x00000080 | 0x08000000 // TOPMOST|LAYERED|TRANSPARENT|TOOLWINDOW|NOACTIVATE
		title, _ := windows.UTF16PtrFromString("Astro Tweaks overlay")
		hwnd, _, err := procCreateWindowExW.Call(exStyle, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)), 0x80000000, // WS_POPUP
			uintptr(x), uintptr(y), overlayW, overlayH, 0, 0, inst, 0)
		if hwnd == 0 {
			ready <- fmt.Errorf("couldn't show the overlay (%v)", err)
			return
		}
		procSetLayeredWindowAttribs.Call(hwnd, 0, 215, 2) // LWA_ALPHA: slightly see-through
		procShowWindow.Call(hwnd, 4)                      // SW_SHOWNOACTIVATE
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

func overlayUpdate(s benchSample) {
	overlayMu.Lock()
	h := overlayHwnd
	if h == 0 {
		overlayMu.Unlock()
		return
	}
	const purple, white, grey = 0x00FF7CA0, 0x00F2F2F2, 0x00B4AAA0
	if s.PID == 0 || s.FPS == 0 {
		overlayLines = []overlayLine{{"Astro Tweaks", purple, false}, {"Waiting for a game…", white, false}}
	} else {
		temp := ""
		if s.Sensors.TempC >= 0 {
			temp = fmt.Sprintf("  %.0f°C", s.Sensors.TempC)
		}
		gpu := "—"
		if s.GPU >= 0 {
			gpu = fmt.Sprintf("%.0f%%", s.GPU)
		}
		overlayLines = []overlayLine{
			{fmt.Sprintf("%.0f FPS", s.FPS), white, true},
			{fmt.Sprintf("1%% low %.0f   %.1f ms", s.Low1, s.FrameMs), purple, false},
			{fmt.Sprintf("CPU %.0f%%   GPU %s%s", s.CPU, gpu, temp), grey, false},
		}
	}
	overlayMu.Unlock()
	procPostMessageW.Call(h, wmAppRefresh, 0, 0)
}

// setOverlay is the switch on the Benchmark page.
func setOverlay(on bool, corner int) error {
	stateMu.Lock()
	appState.Overlay, appState.OverlayCorner = on, corner
	saveStateLocked()
	stateMu.Unlock()
	if !on {
		overlayHide()
		return nil
	}
	if err := benchStart(); err != nil {
		return err
	}
	return overlayShow(corner)
}

// overlayOnStart brings the overlay back if it was on last time.
func overlayOnStart() {
	defer func() { _ = recover() }()
	stateMu.Lock()
	on, corner := appState.Overlay, appState.OverlayCorner
	stateMu.Unlock()
	if !on || !isProUnlocked() || !isElevated() {
		return
	}
	time.Sleep(3 * time.Second)
	if err := setOverlay(true, corner); err != nil {
		logf("Overlay didn't start: %v", err)
	}
}
