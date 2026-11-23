//go:build windows

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// ───────────────────────── finding installed games ─────────────────────────

var (
	gamesMu    sync.Mutex
	gamesFound = map[string]string{} // id → full exe path
	gamesReady = make(chan struct{})
	gamesOnce  sync.Once
	envVarRe   = regexp.MustCompile(`%([A-Za-z0-9_()]+)%`)
)

func expandAll(p string) string {
	return envVarRe.ReplaceAllStringFunc(p, func(m string) string {
		if v := os.Getenv(m[1 : len(m)-1]); v != "" {
			return v
		}
		return m
	})
}

func fixedDrives() []string {
	var out []string
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return []string{"C:"}
	}
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		d := string(rune('A'+i)) + `:\`
		p, _ := windows.UTF16PtrFromString(d)
		if windows.GetDriveType(p) == windows.DRIVE_FIXED {
			out = append(out, d[:2])
		}
	}
	return out
}

func steamLibraries() []string {
	sp := regGetString(registry.CURRENT_USER, `Software\Valve\Steam`, "SteamPath")
	if sp == "" {
		sp = regGetString(registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Valve\Steam`, "InstallPath")
	}
	if sp == "" {
		return nil
	}
	sp = filepath.Clean(strings.ReplaceAll(sp, "/", `\`))
	libs := []string{sp}
	b, err := os.ReadFile(filepath.Join(sp, `steamapps\libraryfolders.vdf`))
	if err == nil {
		for _, m := range regexp.MustCompile(`"path"\s+"([^"]+)"`).FindAllStringSubmatch(string(b), -1) {
			p := filepath.Clean(strings.ReplaceAll(m[1], `\\`, `\`))
			if !containsFold(libs, p) {
				libs = append(libs, p)
			}
		}
	}
	return libs
}

func containsFold(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

func epicInstalls() map[string]string {
	out := map[string]string{}
	var list struct {
		InstallationList []struct {
			InstallLocation string `json:"InstallLocation"`
			AppName         string `json:"AppName"`
		} `json:"InstallationList"`
	}
	if b, err := os.ReadFile(expandAll(`%ProgramData%\Epic\UnrealEngineLauncher\LauncherInstalled.dat`)); err == nil && json.Unmarshal(b, &list) == nil {
		for _, i := range list.InstallationList {
			out[strings.ToLower(i.AppName)] = i.InstallLocation
		}
	}
	return out
}

func riotInstall(product string) string {
	b, err := os.ReadFile(expandAll(`%ProgramData%\Riot Games\Metadata\` + product + `\` + product + `.product_settings.yaml`))
	if err != nil {
		return ""
	}
	m := regexp.MustCompile(`product_install_full_path:\s*"?([^"\r\n]+)"?`).FindStringSubmatch(string(b))
	if m == nil {
		return ""
	}
	return filepath.Clean(strings.ReplaceAll(strings.TrimSpace(m[1]), "/", `\`))
}

type uninstEntry struct{ name, loc string }

func uninstallEntries() []uninstEntry {
	var out []uninstEntry
	roots := []struct {
		k registry.Key
		p string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall`},
	}
	for _, r := range roots {
		for _, sub := range regSubkeys(r.k, r.p) {
			p := r.p + `\` + sub
			name := regGetString(r.k, p, "DisplayName")
			loc := regGetString(r.k, p, "InstallLocation")
			if name != "" && loc != "" {
				out = append(out, uninstEntry{name, filepath.Clean(strings.Trim(loc, `"`))})
			}
		}
	}
	return out
}

// findExe looks for any of the exe names under dir: the usual spots first, then a bounded search.
func findExe(dir string, g GameDef) string {
	if dir == "" {
		return ""
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return ""
	}
	for _, r := range g.Rel {
		if p := filepath.Join(dir, r); fileExists(p) {
			return p
		}
	}
	found, visited := "", 0
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		visited++
		if visited > 25000 || found != "" {
			return fs.SkipAll
		}
		if d.IsDir() {
			if strings.Count(strings.TrimPrefix(p, dir), `\`) > 7 {
				return fs.SkipDir
			}
			return nil
		}
		for _, e := range g.Exes {
			if strings.EqualFold(d.Name(), e) {
				found = p
				return fs.SkipAll
			}
		}
		return nil
	})
	return found
}

func detectGames(fortniteExe string) map[string]string {
	out := map[string]string{}
	if fortniteExe != "" {
		out["fortnite"] = fortniteExe
	}
	libs := steamLibraries()
	epic := epicInstalls()
	uninst := uninstallEntries()
	drives := fixedDrives()
	for _, g := range Games {
		if g.ID == "fortnite" {
			continue
		}
		var dirs []string
		if g.Riot != "" {
			dirs = append(dirs, riotInstall(g.Riot))
		}
		if g.EpicApp != "" {
			dirs = append(dirs, epic[strings.ToLower(g.EpicApp)])
		}
		for _, lib := range libs {
			for _, sd := range g.SteamDir {
				dirs = append(dirs, filepath.Join(lib, `steamapps\common`, sd))
			}
		}
		for _, u := range uninst {
			for _, n := range g.Uninst {
				if strings.EqualFold(u.name, n) || strings.HasPrefix(strings.ToLower(u.name), strings.ToLower(n)+" ") {
					dirs = append(dirs, u.loc)
				}
			}
		}
		for _, p := range g.Paths {
			p = expandAll(p)
			var bases []string
			if strings.HasPrefix(p, "?:") {
				for _, d := range drives {
					bases = append(bases, d+p[2:])
				}
			} else {
				bases = []string{p}
			}
			for _, b := range bases {
				if strings.Contains(b, "*") {
					m, _ := filepath.Glob(b)
					// newest folder first (Roblox keeps old versions around for a while)
					for i := len(m) - 1; i >= 0; i-- {
						dirs = append(dirs, m[i])
					}
				} else {
					dirs = append(dirs, b)
				}
			}
		}
		seen := map[string]bool{}
		for _, d := range dirs {
			if d == "" || seen[strings.ToLower(d)] || strings.Contains(d, "%") {
				continue
			}
			seen[strings.ToLower(d)] = true
			if exe := findExe(d, g); exe != "" {
				out[g.ID] = exe
				break
			}
		}
	}
	return out
}

// startGameDetection runs once in the background at startup; rescanGames runs it again on demand.
func startGameDetection() {
	gamesOnce.Do(func() {
		go func() {
			defer close(gamesReady)
			defer func() { _ = recover() }()
			rescanGames()
		}()
	})
}

func rescanGames() map[string]string {
	t0 := time.Now()
	found := detectGames(getEnv().FortniteExe)
	gamesMu.Lock()
	gamesFound = found
	gamesMu.Unlock()
	envMu.Lock()
	curEnv.Games = copyMap(found)
	envMu.Unlock()
	names := []string{}
	for id := range found {
		names = append(names, id)
	}
	logf("Games found in %s: %s", time.Since(t0).Round(time.Millisecond), strings.Join(names, ", "))
	return found
}

func waitGames(d time.Duration) {
	select {
	case <-gamesReady:
	case <-time.After(d):
	}
}

func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func gameExe(env Env, id string) string {
	if id == "" || id == "fortnite" {
		return env.FortniteExe
	}
	return env.Games[id]
}

// ───────────────────────── the game's own icon, read from its exe ─────────────────────────

var (
	gdi32                = windows.NewLazySystemDLL("gdi32.dll")
	procSHDefExtractIcon = windows.NewLazySystemDLL("shell32.dll").NewProc("SHDefExtractIconW")
	procGetIconInfo      = user32.NewProc("GetIconInfo")
	procDestroyIcon      = user32.NewProc("DestroyIcon")
	procGetDC            = user32.NewProc("GetDC")
	procReleaseDC        = user32.NewProc("ReleaseDC")
	procGetDIBits        = gdi32.NewProc("GetDIBits")
	procGetObject        = gdi32.NewProc("GetObjectW")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
	iconMu               sync.Mutex
	iconCache            = map[string]string{}
)

type iconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  windows.Handle
	HbmColor windows.Handle
}

type bitmapObj struct {
	Type       int32
	Width      int32
	Height     int32
	WidthBytes int32
	Planes     uint16
	BitsPixel  uint16
	Bits       uintptr
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [256]uint32
}

func dibBits(hdc uintptr, hbm windows.Handle, w, h int32) ([]byte, error) {
	var bi bitmapInfo
	bi.Header = bitmapInfoHeader{Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), Width: w, Height: -h, Planes: 1, BitCount: 32}
	buf := make([]byte, int(w)*int(h)*4)
	r, _, _ := procGetDIBits.Call(hdc, uintptr(hbm), 0, uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bi)), 0)
	if r == 0 {
		return nil, errors.New("GetDIBits failed")
	}
	return buf, nil
}

func exeIconPNG(path string, size int) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("icon: %v", r)
		}
	}()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var hicon windows.Handle
	hr, _, _ := procSHDefExtractIcon.Call(uintptr(unsafe.Pointer(p)), 0, 0, uintptr(unsafe.Pointer(&hicon)), 0, uintptr(size&0xFFFF))
	if hr != 0 || hicon == 0 {
		return nil, fmt.Errorf("no icon (0x%X)", hr)
	}
	defer procDestroyIcon.Call(uintptr(hicon))
	var ii iconInfo
	if r, _, _ := procGetIconInfo.Call(uintptr(hicon), uintptr(unsafe.Pointer(&ii))); r == 0 {
		return nil, errors.New("GetIconInfo failed")
	}
	if ii.HbmMask != 0 {
		defer procDeleteObject.Call(uintptr(ii.HbmMask))
	}
	if ii.HbmColor == 0 {
		return nil, errors.New("monochrome icon")
	}
	defer procDeleteObject.Call(uintptr(ii.HbmColor))
	var bm bitmapObj
	if r, _, _ := procGetObject.Call(uintptr(ii.HbmColor), unsafe.Sizeof(bm), uintptr(unsafe.Pointer(&bm))); r == 0 || bm.Width <= 0 || bm.Height <= 0 || bm.Width > 512 {
		return nil, errors.New("bad icon size")
	}
	w, h := bm.Width, bm.Height
	hdc, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, hdc)
	col, err := dibBits(hdc, ii.HbmColor, w, h)
	if err != nil {
		return nil, err
	}
	hasAlpha := false
	for i := 3; i < len(col); i += 4 {
		if col[i] != 0 {
			hasAlpha = true
			break
		}
	}
	var mask []byte
	if !hasAlpha && ii.HbmMask != 0 {
		mask, _ = dibBits(hdc, ii.HbmMask, w, h)
	}
	img := image.NewNRGBA(image.Rect(0, 0, int(w), int(h)))
	for i := 0; i < int(w*h); i++ {
		b, g, r, a := col[i*4], col[i*4+1], col[i*4+2], col[i*4+3]
		if !hasAlpha {
			a = 255
			if mask != nil && mask[i*4] != 0 {
				a = 0
			}
		}
		img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = r, g, b, a
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// gameIcons returns a data URL for every installed game whose exe has a readable icon.
func gameIcons() map[string]string {
	env := getEnv()
	out := map[string]string{}
	for _, g := range Games {
		exe := gameExe(env, g.ID)
		if exe == "" {
			continue
		}
		iconMu.Lock()
		cached, ok := iconCache[exe]
		iconMu.Unlock()
		if !ok {
			b, err := exeIconPNG(exe, 256)
			if err != nil {
				logf("Icon for %s not readable: %v", g.Name, err)
			} else {
				cached = "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
			}
			iconMu.Lock()
			iconCache[exe] = cached
			iconMu.Unlock()
		}
		if cached != "" {
			out[g.ID] = cached
		}
	}
	return out
}

// ───────────────────────── live system monitor ─────────────────────────

var (
	procGetSystemTimes           = kernel32.NewProc("GetSystemTimes")
	procGetTickCount64           = kernel32.NewProc("GetTickCount64")
	cpuMu                        sync.Mutex
	lastIdle, lastKern, lastUser uint64
)

func ft64(f windows.Filetime) uint64 { return uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime) }

func cpuPercent() float64 {
	var idle, kern, user windows.Filetime
	if r, _, _ := procGetSystemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&kern)), uintptr(unsafe.Pointer(&user))); r == 0 {
		return -1
	}
	cpuMu.Lock()
	defer cpuMu.Unlock()
	i, k, u := ft64(idle), ft64(kern), ft64(user)
	di, dk, du := i-lastIdle, k-lastKern, u-lastUser
	first := lastKern == 0
	lastIdle, lastKern, lastUser = i, k, u
	total := dk + du // kernel time includes idle time
	if first || total == 0 {
		return -1
	}
	busy := float64(total-di) / float64(total) * 100
	if busy < 0 {
		busy = 0
	}
	if busy > 100 {
		busy = 100
	}
	return busy
}

type liveStats struct {
	CPU       float64 `json:"cpu"`
	RAMPct    float64 `json:"ramPct"`
	RAMUsedGB float64 `json:"ramUsedGB"`
	RAMGB     float64 `json:"ramGB"`
	Timer     float64 `json:"timer"`
	Procs     int     `json:"procs"`
	UptimeH   float64 `json:"uptimeH"`
	Game      string  `json:"game"`
}

func readLive() liveStats {
	m := memStatus()
	_, _, cur := timerResolution()
	up, _, _ := procGetTickCount64.Call()
	procs := listProcesses()
	s := liveStats{
		CPU: cpuPercent(), RAMPct: float64(m.MemoryLoad), RAMGB: float64(m.TotalPhys) / (1 << 30),
		RAMUsedGB: float64(m.TotalPhys-m.AvailPhys) / (1 << 30), Timer: cur, Procs: len(procs), UptimeH: float64(up) / 3.6e6,
	}
	if g := runningGame(procs); g != nil {
		s.Game = g.def.Name
	}
	return s
}

func runningGameName() string {
	if g := runningGame(listProcesses()); g != nil {
		return g.def.Name
	}
	return ""
}

// ───────────────────────── auto-boost when a game starts ─────────────────────────

type runningG struct {
	def GameDef
	pid uint32
}

func runningGame(procs []procInfo) *runningG {
	for _, p := range procs {
		for _, g := range Games {
			for _, e := range g.Exes {
				if strings.EqualFold(p.Name, e) {
					return &runningG{g, p.PID}
				}
			}
		}
	}
	return nil
}

var (
	autoMu      sync.Mutex
	autoRunning bool
)

func startAutoBoost() {
	autoMu.Lock()
	if autoRunning {
		autoMu.Unlock()
		return
	}
	autoRunning = true
	autoMu.Unlock()
	go func() {
		defer func() { _ = recover() }()
		boosted := map[uint32]bool{}
		for {
			time.Sleep(3 * time.Second)
			stateMu.Lock()
			on := appState.AutoBoost
			stateMu.Unlock()
			if !on {
				autoMu.Lock()
				autoRunning = false
				autoMu.Unlock()
				return
			}
			g := runningGame(listProcesses())
			if g == nil || boosted[g.pid] {
				continue
			}
			boosted[g.pid] = true
			setTimer(true)
			freed, _ := purgeStandby()
			if h, err := windows.OpenProcess(windows.PROCESS_SET_INFORMATION, false, g.pid); err == nil {
				_ = windows.SetPriorityClass(h, highPriorityClass)
				windows.CloseHandle(h)
			}
			logf("Auto-boost: %s started, boosted (%.0f MB memory freed)", g.def.Name, freed)
			emit("autoboost", map[string]interface{}{"game": g.def.Name, "freedMB": freed})
		}
	}()
}
