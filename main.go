//go:build windows

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

const AppVersion = "1.9.2"

// versionMarker lets the admin page read the version straight out of an uploaded exe.
const versionMarker = "ASTROSPEED_VERSION=" + AppVersion + ";"

// launchedAfterUpdate is true when this copy was started by the in-app updater.
var launchedAfterUpdate bool

//go:embed ui/dist/index.html
var indexHTML string

// instanceMutex keeps other copies out; the updater releases it just before relaunching.
var instanceMutex windows.Handle

var (
	wv       webview2.WebView
	jobMu    sync.Mutex
	user32   = windows.NewLazySystemDLL("user32.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")
	shell32b = windows.NewLazySystemDLL("shell32.dll")

	procShowWindow       = user32.NewProc("ShowWindow")
	procSendMessage      = user32.NewProc("SendMessageW")
	procLoadImage        = user32.NewProc("LoadImageW")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procMessageBox       = user32.NewProc("MessageBoxW")
	procFindWindow       = user32.NewProc("FindWindowW")
	procSetForeground    = user32.NewProc("SetForegroundWindow")
	procDwmSetAttr       = dwmapi.NewProc("DwmSetWindowAttribute")
	procSetAppID         = shell32b.NewProc("SetCurrentProcessExplicitAppUserModelID")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

func messageBox(title, text string, flags uintptr) int {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(text)
	r, _, _ := procMessageBox.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), flags)
	return int(r)
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			logf("CRASH: %v\n%s", r, debug.Stack())
			messageBox("AstroSpeed Tweaks", fmt.Sprintf("AstroSpeed hit an unexpected error and needs to close.\n\n%v\n\nYour backups are safe in %s", r, dataDir), 0x10)
		}
	}()

	// one window at a time
	name, _ := windows.UTF16PtrFromString("Global\\AstroSpeedTweaksSingleInstance")
	afterUpdate := false
	for _, a := range os.Args[1:] {
		if a == "--after-update" {
			afterUpdate = true
			launchedAfterUpdate = true
		}
	}
	for tries := 0; ; tries++ {
		h, err := windows.CreateMutex(nil, false, name)
		if err != windows.ERROR_ALREADY_EXISTS {
			instanceMutex = h
			break
		}
		if h != 0 {
			windows.CloseHandle(h)
		}
		// Just updated: the old copy is still closing, so wait for it instead of bowing out.
		if afterUpdate && tries < 50 {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		title, _ := windows.UTF16PtrFromString("AstroSpeed Tweaks")
		if w, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(title))); w != 0 {
			procShowWindow.Call(w, 9) // SW_RESTORE
			procSetForeground.Call(w)
		}
		return
	}

	appID, _ := windows.UTF16PtrFromString("AstroSpeed.Tweaks")
	procSetAppID.Call(uintptr(unsafe.Pointer(appID)))

	initDataDir()
	cleanupOldUpdate()
	setupCrashLog()
	defer markCleanExit()
	loadState()
	logf("AstroSpeed Tweaks %s started (admin: %v)", AppVersion, isElevated())

	curEnv = detectEnv()
	curEnv.Games = map[string]string{}
	if curEnv.FortniteExe != "" {
		curEnv.Games["fortnite"] = curEnv.FortniteExe
	}
	lastProbe = time.Now()
	loadLicense()
	loadCachedPack()
	go refreshLicenseOnline()
	go watchTrial()
	logf("%s", versionMarker)
	startGameDetection()
	go refreshPackLoop()
	stateMu.Lock()
	ab := appState.AutoBoost
	stateMu.Unlock()
	if ab && isProUnlocked() {
		startAutoBoost()
	}
	stateMu.Lock()
	hk := appState.Hotkey
	stateMu.Unlock()
	if hk && isProUnlocked() {
		if err := setHotkey(true); err != nil {
			logf("Hotkey: %v", err)
		}
	}
	logf("Detected: %s | %s | %.0f GB RAM | Fortnite: %v", curEnv.OSName, curEnv.CPU, curEnv.RAMGB, curEnv.FortniteExe != "")

	_ = os.Setenv("WEBVIEW2_DEFAULT_BACKGROUND_COLOR", "FF000000")
	profileDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "AstroSpeed", "WebView2")
	_ = os.MkdirAll(profileDir, 0o755)

	wv = webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		DataPath:  profileDir,
		WindowOptions: webview2.WindowOptions{
			Title:  "AstroSpeed Tweaks",
			Width:  1920,
			Height: 1080,
			IconId: 1,
			Center: true,
		},
	})
	if wv == nil {
		if messageBox("AstroSpeed Tweaks", "AstroSpeed needs the Microsoft Edge WebView2 Runtime, which is missing on this PC.\n\nClick OK to open Microsoft's download page, install the Evergreen Runtime, then open AstroSpeed again.", 0x31) == 1 {
			_ = shellOpen("https://go.microsoft.com/fwlink/p/?LinkId=2124703")
		}
		return
	}
	defer wv.Destroy()

	logf("Window created")
	hwnd := uintptr(wv.Window())
	styleWindow(hwnd)
	wv.SetSize(1280, 760, webview2.HintMin)

	_ = wv.Bind("sgInvoke", func(method string, args string, id int) bool {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logf("panic in %s: %v\n%s", method, r, debug.Stack())
					resolve(id, nil, fmt.Errorf("internal error: %v", r))
				}
			}()
			res, err := handle(method, json.RawMessage(args))
			resolve(id, res, err)
		}()
		return true
	})
	wv.SetHtml(indexHTML)
	wv.Run()
	setTimer(false)
	logf("AstroSpeed closed")
}

var lastCrash string

// setupCrashLog sends Go runtime crashes and library log output to files, and notices if the
// previous run ended without closing normally.
func setupCrashLog() {
	logs := filepath.Join(dataDir, "logs")
	marker := filepath.Join(dataDir, "running.flag")
	crashFile := filepath.Join(logs, "crash.log")
	if _, err := os.Stat(marker); err == nil {
		// previous run didn't exit cleanly
		tail := ""
		if b, err := os.ReadFile(crashFile); err == nil && len(b) > 0 {
			if len(b) > 4000 {
				b = b[len(b)-4000:]
			}
			tail = string(b)
		}
		if tail == "" {
			tail = "The app was closed without an error message. This usually means Windows Security or another antivirus stopped it. Check Windows Security > Virus & threat protection > Protection history."
		}
		lastCrash = tail
		_ = os.Rename(crashFile, filepath.Join(logs, "crash-"+time.Now().Format("20060102-150405")+".log"))
	}
	_ = os.WriteFile(marker, []byte(time.Now().Format(time.RFC3339)), 0o644)
	if f, err := os.OpenFile(crashFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		_ = debug.SetCrashOutput(f, debug.CrashOptions{})
		log.SetOutput(f) // the WebView2 library reports fatal errors through the standard logger
		log.SetFlags(log.LstdFlags)
	}
}

func markCleanExit() { _ = os.Remove(filepath.Join(dataDir, "running.flag")) }

// styleWindow gives the native title bar the app's dark colour, sets the taskbar icon and maximises.
func styleWindow(hwnd uintptr) {
	dark := uint32(1)
	procDwmSetAttr.Call(hwnd, 20, uintptr(unsafe.Pointer(&dark)), 4) // DWMWA_USE_IMMERSIVE_DARK_MODE
	caption := uint32(0x00000000)                                    // black
	procDwmSetAttr.Call(hwnd, 35, uintptr(unsafe.Pointer(&caption)), 4)
	text := uint32(0x00FFF0EA)
	procDwmSetAttr.Call(hwnd, 36, uintptr(unsafe.Pointer(&text)), 4)

	hinst, _, _ := procGetModuleHandleW.Call(0)
	for _, s := range []struct{ which, metric uintptr }{{1, 11}, {0, 49}} { // ICON_BIG/SM_CXICON, ICON_SMALL/SM_CXSMICON
		sz, _, _ := procGetSystemMetrics.Call(s.metric)
		if s.which == 1 {
			sz *= 2
		}
		if ic, _, _ := procLoadImage.Call(hinst, 1, 1, sz, sz, 0); ic != 0 {
			procSendMessage.Call(hwnd, 0x0080, s.which, ic) // WM_SETICON
		}
	}
	if startMinimized() {
		procShowWindow.Call(hwnd, 7) // SW_SHOWMINNOACTIVE: started with Windows
	} else {
		procShowWindow.Call(hwnd, 3) // SW_MAXIMIZE
	}
}

func jsString(v interface{}) string { b, _ := json.Marshal(v); return string(b) }

func resolve(id int, res interface{}, err error) {
	var payload string
	if err != nil {
		payload = jsString(map[string]interface{}{"error": err.Error()})
	} else {
		payload = jsString(map[string]interface{}{"ok": res})
	}
	payload = strings.ReplaceAll(payload, "\x00", "")
	wv.Dispatch(func() {
		wv.Eval(fmt.Sprintf("window.__sg&&window.__sg.done(%d,%s)", id, payload))
	})
}

func emit(event string, payload interface{}) {
	p := strings.ReplaceAll(jsString(payload), "\x00", "")
	wv.Dispatch(func() {
		wv.Eval(fmt.Sprintf("window.__sg&&window.__sg.event(%s,%s)", jsString(event), p))
	})
}

// ───────────────────────── API ─────────────────────────

func catalogByID() map[string]Tweak {
	m := map[string]Tweak{}
	for _, t := range Catalog {
		m[t.ID] = t
	}
	return m
}

type statusUI struct {
	S   string `json:"s"`
	Why string `json:"why,omitempty"`
}

func allStatuses(withBCD bool) map[string]statusUI {
	env := getEnv()
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if withBCD {
		engine.loadBCD(true)
	}
	out := map[string]statusUI{}
	for _, t := range Catalog {
		stateMu.Lock()
		st := appState.Tweaks[t.ID]
		var cp *TweakState
		if st != nil {
			c := *st
			cp = &c
		}
		stateMu.Unlock()
		s, why := engine.Status(t, env, cp)
		out[t.ID] = statusUI{s, why}
	}
	return out
}

func parseArgs(args json.RawMessage, v interface{}) error {
	if len(args) == 0 || string(args) == "null" {
		return nil
	}
	return json.Unmarshal(args, v)
}

func handle(method string, args json.RawMessage) (interface{}, error) {
	if paidTool(method, args) && !isProUnlocked() {
		return nil, fmt.Errorf("this needs an AstroSpeed key")
	}
	switch method {
	case "init":
		var tw []tweakUI
		for _, t := range Catalog {
			u := tweakUI{Tweak: t}
			for _, a := range t.Actions {
				u.Changes = append(u.Changes, a.Describe())
			}
			u.Pro = isPro(t)
			if u.Pro && !isProUnlocked() {
				u = redactTweak(u) // locked: the real name, details and changes never reach the window
			}
			tw = append(tw, u)
		}
		n, acts := CatalogStats()
		stateMu.Lock()
		profile, total, last := appState.Profile, appState.TotalChanges, appState.LastOptimize
		var rps []RestorePoint
		rps = append(rps, appState.RestorePoints...)
		stateMu.Unlock()
		stateMu.Lock()
		autoBoost, hotkey, autoUpdate := appState.AutoBoost, appState.Hotkey, !appState.NoAutoUpdate
		stateMu.Unlock()
		return map[string]interface{}{
			"version": AppVersion, "license": licenseStatusNow(), "afterUpdate": launchedAfterUpdate, "categories": Categories, "catalog": tw, "games": Games, "autoBoost": autoBoost, "hotkey": hotkey, "autoUpdate": autoUpdate, "server": serverURL(), "updateConfigured": updateCheckURL() != "", "pack": packStatus(),
			"tweakCount": n, "actionCount": acts, "env": getEnv(), "admin": isElevated(),
			"profile": profile, "totalChanges": total, "lastOptimize": last, "restorePoints": rps,
			"cleanTargets": cleanTargets, "dataDir": dataDir, "lastCrash": lastCrash,
		}, nil

	case "states":
		var a struct {
			BCD bool `json:"bcd"`
		}
		_ = parseArgs(args, &a)
		waitGames(8 * time.Second)
		st := allStatuses(a.BCD)
		return map[string]interface{}{"states": st, "reboot": engine.rebootNeeded}, nil

	case "env":
		refreshEnvFast()
		return getEnv(), nil

	case "probe":
		var a struct {
			Force bool `json:"force"`
		}
		_ = parseArgs(args, &a)
		probeOnce(a.Force)
		return getEnv(), nil

	case "apply", "revert", "revertAll":
		var a struct {
			IDs     []string `json:"ids"`
			Restore bool     `json:"restore"`
			Label   string   `json:"label"`
			Profile int      `json:"profile"`
		}
		_ = parseArgs(args, &a)
		if !jobMu.TryLock() {
			return nil, fmt.Errorf("another job is still running")
		}
		defer jobMu.Unlock()
		return runJob(method, a.IDs, a.Restore, a.Label, a.Profile)

	case "restorePoint":
		if !jobMu.TryLock() {
			return nil, fmt.Errorf("another job is still running")
		}
		defer jobMu.Unlock()
		err := createRestorePoint("AstroSpeed Tweaks - manual restore point")
		return map[string]bool{"ok": err == nil}, err

	case "shopInfo":
		return shopInfo(), nil
	case "cleanScan":
		return cleanerScan(), nil
	case "cleanRun":
		var a struct {
			IDs []string `json:"ids"`
		}
		_ = parseArgs(args, &a)
		return cleanerRun(a.IDs), nil

	case "startupList":
		return listStartup(), nil
	case "startupSet":
		var a struct {
			ID      string `json:"id"`
			Enabled bool   `json:"enabled"`
		}
		_ = parseArgs(args, &a)
		return nil, setStartup(a.ID, a.Enabled)

	case "boostStatus":
		mn, mx, cur := timerResolution()
		m := memStatus()
		timerMu.Lock()
		held := timerHeld
		timerMu.Unlock()
		return map[string]interface{}{
			"timerHeld": held, "timerCur": cur, "timerMin": mn, "timerMax": mx,
			"standbyMB": standbyMB(), "freeMB": float64(m.AvailPhys) / (1 << 20), "totalMB": float64(m.TotalPhys) / (1 << 20),
			"memLoad": m.MemoryLoad, "fortnite": runningGame(listProcesses()) != nil, "game": runningGameName(),
		}, nil
	case "boostTimer":
		var a struct {
			On bool `json:"on"`
		}
		_ = parseArgs(args, &a)
		return setTimer(a.On), nil
	case "boostPurge":
		return purgeStandby()
	case "boostPriority":
		running, err := boostFortniteNow()
		return map[string]bool{"running": running}, err
	case "bgApps":
		return runningBackgroundApps(), nil
	case "bgClose":
		var a struct {
			Names []string `json:"names"`
		}
		_ = parseArgs(args, &a)
		n, mb := closeApps(a.Names)
		return map[string]interface{}{"closed": n, "freedMB": mb}, nil

	case "flushDNS":
		_, err := runCmd("ipconfig /flushdns", 30*time.Second)
		logf("DNS cache flushed")
		return true, err
	case "ping":
		return pingAll(), nil
	case "dnsGet":
		return getDNS(), nil
	case "dnsSet":
		var a struct {
			Preset string `json:"preset"`
		}
		_ = parseArgs(args, &a)
		return nil, setDNS(a.Preset)

	case "restart":
		var a struct {
			Firmware bool `json:"firmware"`
		}
		_ = parseArgs(args, &a)
		logf("Restart requested (firmware: %v)", a.Firmware)
		err := restartPC(a.Firmware)
		if err == nil {
			markCleanExit()
		}
		return nil, err

	case "open":
		var a struct {
			Target string `json:"target"`
		}
		_ = parseArgs(args, &a)
		return nil, openTarget(a.Target)

	case "log":
		return recentLog(400), nil

	case "games":
		waitGames(8 * time.Second)
		return getEnv().Games, nil
	case "rescanGames":
		waitGames(8 * time.Second)
		return rescanGames(), nil
	case "gameIcons":
		waitGames(8 * time.Second)
		return gameIcons(), nil
	case "live":
		return readLive(), nil
	case "drives":
		return drives(), nil
	case "licenseStatus":
		go refreshLicenseOnline()
		return licenseStatusNow(), nil
	case "removeKey":
		return removeKey(), nil
	case "trialReverted":
		st := markTrialReverted()
		emit("license", st)
		return st, nil
	case "activate":
		var a struct {
			Key string `json:"key"`
		}
		_ = parseArgs(args, &a)
		return activateKey(a.Key)
	case "checkUpdate":
		return checkUpdate()
	case "downloadUpdate":
		return downloadUpdate()
	case "applyUpdate":
		return applyUpdate()
	case "checkup":
		return checkup(), nil
	case "pingOnce":
		var a struct {
			ID string `json:"id"`
		}
		_ = parseArgs(args, &a)
		return pingOnce(a.ID), nil
	case "packStatus":
		return packStatus(), nil
	case "refreshPack":
		return refreshPack(true)
	case "startupGet":
		return startupEnabled(), nil
	case "setSetting":
		var a struct {
			Key string `json:"key"`
			On  bool   `json:"on"`
		}
		_ = parseArgs(args, &a)
		var err error
		switch a.Key {
		case "startup":
			err = setStartupEnabled(a.On)
		case "hotkey":
			err = setHotkey(a.On)
			if err == nil {
				stateMu.Lock()
				appState.Hotkey = a.On
				saveStateLocked()
				stateMu.Unlock()
			}
		case "autoUpdate":
			stateMu.Lock()
			appState.NoAutoUpdate = !a.On
			saveStateLocked()
			stateMu.Unlock()
		default:
			err = fmt.Errorf("unknown setting")
		}
		logf("Setting %s: %v", a.Key, a.On)
		return a.On, err
	case "setAutoBoost":
		var a struct {
			On bool `json:"on"`
		}
		_ = parseArgs(args, &a)
		stateMu.Lock()
		appState.AutoBoost = a.On
		saveStateLocked()
		stateMu.Unlock()
		if a.On {
			startAutoBoost()
		}
		logf("Auto-boost when a game starts: %v", a.On)
		return a.On, nil

	case "ready":
		logf("Interface ready")
		return true, nil

	case "setProfile":
		var a struct {
			Profile int `json:"profile"`
		}
		_ = parseArgs(args, &a)
		stateMu.Lock()
		appState.Profile = a.Profile
		saveStateLocked()
		stateMu.Unlock()
		return true, nil
	}
	return nil, fmt.Errorf("unknown method %s", method)
}

func openTarget(t string) error {
	switch t {
	case "logs":
		return shellOpen(filepath.Join(dataDir, "logs"))
	case "display":
		return shellOpen("ms-settings:display-advanced")
	case "graphics":
		return shellOpen("ms-settings:display-advancedgraphics")
	case "restore":
		return shellOpen(filepath.Join(os.Getenv("SystemRoot"), `System32\rstrui.exe`))
	case "nvcp":
		if err := shellOpen(`shell:AppsFolder\NVIDIACorp.NVIDIAControlPanel_56jybvy8sckqj!NVIDIACorp.NVIDIAControlPanel`); err == nil {
			return nil
		}
		return shellOpen(`C:\Program Files\NVIDIA Corporation\Control Panel Client\nvcplui.exe`)
	case "nvapp":
		return shellOpen(`C:\Program Files\NVIDIA Corporation\NVIDIA app\CEF\NVIDIA app.exe`)
	case "amd":
		return shellOpen(`C:\Program Files\AMD\CNext\CNext\RadeonSoftware.exe`)
	case "discord":
		upd := expandEnv(`%LOCALAPPDATA%\Discord\Update.exe`)
		if fileExists(upd) {
			_, err := runExe(5*time.Second, upd, "--processStart", "Discord.exe")
			return err
		}
		return shellOpen("discord://")
	case "fortnite":
		return shellOpen("com.epicgames.launcher://apps/fn%3A4fe75bbc5a674f4f9b356b5c90567da5%3AFortnite?action=launch&silent=true")
	}
	if strings.HasPrefix(t, "game:") || strings.HasPrefix(t, "gamedir:") {
		id := t[strings.Index(t, ":")+1:]
		if id == "fortnite" && strings.HasPrefix(t, "game:") {
			return openTarget("fortnite")
		}
		exe := gameExe(getEnv(), id)
		if exe == "" {
			return fmt.Errorf("that game wasn't found on this PC")
		}
		if strings.HasPrefix(t, "gamedir:") {
			return shellOpen(filepath.Dir(exe))
		}
		return shellOpen(exe)
	}
	switch t {
	case "gamemode":
		return shellOpen("ms-settings:gaming-gamemode")
	case "graphics":
		return shellOpen("ms-settings:display-advancedgraphics")
	case "power":
		return shellOpen("control.exe powercfg.cpl")
	case "startup":
		return shellOpen("ms-settings:startupapps")
	case "coreisolation":
		return shellOpen("windowsdefender://coreisolation")
	}
	if strings.HasPrefix(t, "https://") {
		return shellOpen(t)
	}
	return fmt.Errorf("unknown target")
}

func runJob(kind string, ids []string, restore bool, label string, profile int) (interface{}, error) {
	byID := catalogByID()
	env := getEnv()
	var list []Tweak

	if kind == "revertAll" {
		type seqT struct {
			t   Tweak
			seq int
		}
		var s []seqT
		stateMu.Lock()
		for id, st := range appState.Tweaks {
			if st.Applied || len(st.Backups) > 0 {
				t, ok := byID[id]
				if !ok { // removed from the GitHub pack: undo still works from the saved backups
					t = Tweak{ID: id, Title: "Removed online tweak (" + id + ")", Cat: "online"}
				}
				s = append(s, seqT{t, st.Seq})
			}
		}
		stateMu.Unlock()
		sort.Slice(s, func(i, j int) bool { return s[i].seq > s[j].seq })
		for _, x := range s {
			list = append(list, x.t)
		}
	} else {
		// keep catalog order so related tweaks run in a sensible sequence
		want := map[string]bool{}
		for _, id := range ids {
			want[id] = true
		}
		for _, t := range Catalog {
			if want[t.ID] {
				list = append(list, t)
			}
		}
		if kind == "revert" {
			for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
				list[i], list[j] = list[j], list[i]
			}
		}
	}

	total := len(list)
	results := []TweakResult{}
	if restore && kind == "apply" {
		emit("progress", map[string]interface{}{"phase": "restore", "i": 0, "n": total, "title": "Creating a System Restore point"})
		if err := createRestorePoint(firstNonEmpty(label, "AstroSpeed Tweaks - before optimize")); err != nil {
			emit("progress", map[string]interface{}{"phase": "restore-failed", "i": 0, "n": total, "title": "Restore point couldn't be created", "msg": firstLine(err.Error())})
		} else {
			emit("progress", map[string]interface{}{"phase": "restore-ok", "i": 0, "n": total, "title": "Restore point created"})
		}
	}
	verb := map[string]string{"apply": "Applying", "revert": "Undoing", "revertAll": "Undoing"}[kind]
	logf("── %s %d tweak(s) ──", strings.ToLower(verb), total)
	changes := 0
	for i, t := range list {
		emit("progress", map[string]interface{}{"phase": "start", "i": i, "n": total, "id": t.ID, "title": t.Title})
		if kind == "apply" && isPro(t) && !isProUnlocked() {
			results = append(results, TweakResult{ID: t.ID, Title: t.Title, Status: "locked", Msg: "Needs a key"})
			emit("progress", map[string]interface{}{"phase": "done", "i": i + 1, "n": total, "id": t.ID, "title": t.Title, "status": "locked", "msg": "Needs a key"})
			continue
		}
		logf("%s: %s", verb, t.Title)
		engine.mu.Lock()
		var r TweakResult
		if kind == "apply" {
			r = engine.ApplyTweak(t, env)
		} else {
			r = engine.RevertTweak(t, env)
		}
		engine.mu.Unlock()
		changes += r.Changes
		results = append(results, r)
		emit("progress", map[string]interface{}{"phase": "done", "i": i + 1, "n": total, "id": t.ID, "title": t.Title, "status": r.Status, "msg": r.Msg, "changes": r.Changes})
	}
	if kind == "apply" && profile > 0 {
		stateMu.Lock()
		appState.LastOptimize = time.Now()
		appState.Profile = profile
		saveStateLocked()
		stateMu.Unlock()
	}
	logf("── finished: %d change(s) ──", changes)
	return map[string]interface{}{"results": results, "changes": changes, "reboot": engine.rebootNeeded}, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
