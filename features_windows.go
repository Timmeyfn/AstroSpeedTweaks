//go:build windows

package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ───────────────────────── update checks (self-hosted) ─────────────────────────

// update-url.txt holds the address of a small JSON file on your own server that says
// what the latest version is, for example:
//
//	https://astrospeed.dupelication.workers.dev/version.json
//
// Nothing here touches GitHub. Leave it blank to switch update checks off.
//
//go:embed update-url.txt
var updateURLFile string

func updateCheckURL() string {
	for _, ln := range strings.Split(updateURLFile, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		if strings.HasPrefix(ln, "https://") {
			return ln
		}
	}
	return ""
}

// updateRepo is still used by the online tweak pack.
//
//go:embed update-repo.txt
var updateRepoFile string

func updateRepo() string { return parseRepo(updateRepoFile) }

type updateInfo struct {
	Configured bool   `json:"configured"`
	Available  bool   `json:"available"`
	Current    string `json:"current"`
	Latest     string `json:"latest"`
	Title      string `json:"title"`
	Notes      string `json:"notes"`
	Download   string `json:"download"`
	Page       string `json:"page"`
	Published  string `json:"published"`
}

func checkUpdate() (updateInfo, error) {
	info := updateInfo{Current: AppVersion}
	u := updateCheckURL()
	if u == "" {
		return info, nil
	}
	info.Configured = true
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "AstroSpeed/"+AppVersion)
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return info, errors.New("couldn't check for updates. Check your internet connection")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		info.Latest = AppVersion
		return info, nil
	}
	var m struct {
		Version   string `json:"version"`
		Notes     string `json:"notes"`
		Title     string `json:"title"`
		Download  string `json:"download"`
		Published string `json:"published"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&m); err != nil {
		return info, errors.New("couldn't read the update information")
	}
	info.Latest = strings.TrimPrefix(strings.TrimSpace(m.Version), "v")
	info.Notes, info.Published = m.Notes, m.Published
	info.Title = m.Title
	if info.Title == "" && info.Latest != "" {
		info.Title = "Version " + info.Latest
	}
	info.Download, info.Page = m.Download, m.Download
	info.Available = info.Latest != "" && newerVersion(info.Latest, AppVersion)
	logf("Update check: running %s, latest %s, available: %v", AppVersion, info.Latest, info.Available)
	return info, nil
}

// ───────────────────────── start with Windows ─────────────────────────

const startupTask = "AstroSpeed Tweaks"

func schtasksPath() string { return os.Getenv("SystemRoot") + `\System32\schtasks.exe` }

func startupEnabled() bool {
	_, err := runExe(15*time.Second, schtasksPath(), "/query", "/tn", startupTask)
	return err == nil
}

// setStartup runs AstroSpeed at sign-in, minimised, with admin rights (via Task Scheduler,
// so there's no admin prompt every time Windows starts).
func setStartupEnabled(on bool) error {
	if !on {
		_, err := runExe(15*time.Second, schtasksPath(), "/delete", "/tn", startupTask, "/f")
		if err != nil && !startupEnabled() {
			return nil
		}
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	_, err = runExe(15*time.Second, schtasksPath(), "/create", "/tn", startupTask, "/tr", `"`+exe+`" --minimized`, "/sc", "onlogon", "/rl", "highest", "/f")
	if err == nil {
		logf("AstroSpeed will start with Windows")
	}
	return err
}

func startMinimized() bool {
	for _, a := range os.Args[1:] {
		if strings.EqualFold(a, "--minimized") {
			return true
		}
	}
	return false
}

// ───────────────────────── boost hotkey (Ctrl + Alt + B) ─────────────────────────

var (
	procRegisterHotKey    = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey  = user32.NewProc("UnregisterHotKey")
	procGetMessageW       = user32.NewProc("GetMessageW")
	procPostThreadMessage = user32.NewProc("PostThreadMessageW")
	procMessageBeep       = user32.NewProc("MessageBeep")
	hotkeyMu              sync.Mutex
	hotkeyThread          uint32
)

type winMsg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	PtX     int32
	PtY     int32
	Private uint32
}

// quickBoost is what the hotkey does: the same as "Boost now", minus closing apps.
func quickBoost() {
	defer func() { _ = recover() }()
	setTimer(true)
	freed, _ := purgeStandby()
	_, _ = runCmd("ipconfig /flushdns", 20*time.Second)
	running, _ := boostFortniteNow()
	procMessageBeep.Call(0)
	game := ""
	if running {
		game = runningGameName()
	}
	logf("Hotkey boost: %.0f MB freed, game: %q", freed, game)
	emit("hotkeyboost", map[string]interface{}{"freedMB": freed, "game": game})
}

func setHotkey(on bool) error {
	hotkeyMu.Lock()
	defer hotkeyMu.Unlock()
	if !on {
		if hotkeyThread != 0 {
			procPostThreadMessage.Call(uintptr(hotkeyThread), 0x0012, 0, 0) // WM_QUIT
			hotkeyThread = 0
		}
		return nil
	}
	if hotkeyThread != 0 {
		return nil
	}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		const modAlt, modCtrl, modNoRepeat = 0x1, 0x2, 0x4000
		if r, _, err := procRegisterHotKey.Call(0, 1, modAlt|modCtrl|modNoRepeat, 'B'); r == 0 {
			ready <- fmt.Errorf("Ctrl+Alt+B is already used by another app (%v)", err)
			return
		}
		defer procUnregisterHotKey.Call(0, 1)
		hotkeyMu2(windows.GetCurrentThreadId())
		ready <- nil
		var m winMsg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			if m.Message == 0x0312 { // WM_HOTKEY
				go quickBoost()
			}
		}
	}()
	select {
	case err := <-ready:
		return err
	case <-time.After(3 * time.Second):
		return errors.New("the hotkey couldn't be set up")
	}
}

// hotkeyMu2 records the listener thread; called with hotkeyMu already held by setHotkey.
func hotkeyMu2(id uint32) { hotkeyThread = id }

// ───────────────────────── storage ─────────────────────────

var procGetDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")

type driveInfo struct {
	Letter string  `json:"letter"`
	Label  string  `json:"label"`
	FreeGB float64 `json:"freeGB"`
	SizeGB float64 `json:"sizeGB"`
}

func drives() []driveInfo {
	var out []driveInfo
	for _, d := range fixedDrives() {
		root := d + `\`
		p, _ := windows.UTF16PtrFromString(root)
		var free, total, totalFree uint64
		if r, _, _ := procGetDiskFreeSpaceEx.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&free)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&totalFree))); r == 0 || total == 0 {
			continue
		}
		label := make([]uint16, 64)
		_ = windows.GetVolumeInformation(p, &label[0], uint32(len(label)), nil, nil, nil, nil, 0)
		out = append(out, driveInfo{Letter: d, Label: windows.UTF16ToString(label), FreeGB: float64(free) / (1 << 30), SizeGB: float64(total) / (1 << 30)})
	}
	return out
}
