//go:build windows

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// ───────────────────────── processes ─────────────────────────

type procInfo struct {
	PID  uint32
	Name string
}

func listProcesses() []procInfo {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	var out []procInfo
	if err := windows.Process32First(snap, &pe); err != nil {
		return nil
	}
	for {
		out = append(out, procInfo{PID: pe.ProcessID, Name: windows.UTF16ToString(pe.ExeFile[:])})
		if err := windows.Process32Next(snap, &pe); err != nil {
			break
		}
	}
	return out
}

func processRunning(name string) bool {
	for _, p := range listProcesses() {
		if strings.EqualFold(p.Name, name) {
			return true
		}
	}
	return false
}

func fortniteRunning() bool { return processRunning("FortniteClient-Win64-Shipping.exe") }

var (
	psapi                    = windows.NewLazySystemDLL("psapi.dll")
	procGetProcessMemoryInfo = psapi.NewProc("GetProcessMemoryInfo")
)

type processMemoryCounters struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

func processMemMB(pid uint32) float64 {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)
	var c processMemoryCounters
	c.CB = uint32(unsafe.Sizeof(c))
	r, _, _ := procGetProcessMemoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&c)), uintptr(c.CB))
	if r == 0 {
		return 0
	}
	return float64(c.WorkingSetSize) / (1 << 20)
}

// Known background apps the Game Booster can close. Mouse/keyboard/headset
// software is deliberately NOT here: closing it can reset DPI or lighting.
type bgApp struct {
	Name    string   `json:"name"`
	Exes    []string `json:"-"`
	Default bool     `json:"default"`
	Note    string   `json:"note,omitempty"`
}

var bgApps = []bgApp{
	{"OneDrive", []string{"OneDrive.exe"}, true, ""},
	{"Microsoft Teams", []string{"ms-teams.exe", "Teams.exe"}, true, ""},
	{"Microsoft Edge", []string{"msedge.exe"}, false, "Closes all Edge windows"},
	{"Google Chrome", []string{"chrome.exe"}, false, "Closes all Chrome windows"},
	{"Brave", []string{"brave.exe"}, false, "Closes all Brave windows"},
	{"Opera / Opera GX", []string{"opera.exe"}, false, "Closes all Opera windows"},
	{"Firefox", []string{"firefox.exe"}, false, "Closes all Firefox windows"},
	{"Spotify", []string{"Spotify.exe"}, false, "Stops your music"},
	{"Steam", []string{"steam.exe", "steamwebhelper.exe"}, true, ""},
	{"EA app", []string{"EADesktop.exe", "EABackgroundService.exe"}, true, ""},
	{"Battle.net", []string{"Battle.net.exe"}, true, ""},
	{"Ubisoft Connect", []string{"upc.exe", "UbisoftConnect.exe"}, true, ""},
	{"Riot Client", []string{"RiotClientServices.exe", "RiotClientUx.exe"}, true, "Does not touch Vanguard"},
	{"Overwolf", []string{"Overwolf.exe", "OverwolfBrowser.exe"}, true, ""},
	{"Medal", []string{"Medal.exe"}, false, "Stops clipping"},
	{"Adobe Creative Cloud", []string{"Creative Cloud.exe", "CCXProcess.exe", "AdobeIPCBroker.exe", "CoreSync.exe"}, true, ""},
	{"Dropbox", []string{"Dropbox.exe"}, true, ""},
	{"Google Drive", []string{"GoogleDriveFS.exe"}, true, ""},
	{"iCloud", []string{"iCloudDrive.exe", "iCloudServices.exe"}, true, ""},
	{"Phone Link", []string{"PhoneExperienceHost.exe"}, true, ""},
	{"Widgets", []string{"Widgets.exe", "WidgetService.exe"}, true, ""},
	{"Wallpaper Engine", []string{"wallpaper32.exe", "wallpaper64.exe"}, true, ""},
	{"Lively Wallpaper", []string{"Lively.exe"}, true, ""},
	{"Discord", []string{"Discord.exe"}, false, "Leaves your call"},
	{"OBS Studio", []string{"obs64.exe"}, false, "Stops recording/streaming"},
	{"WhatsApp", []string{"WhatsApp.exe"}, false, ""},
	{"Skype", []string{"Skype.exe"}, true, ""},
	{"Cortana", []string{"Cortana.exe"}, true, ""},
}

type runningApp struct {
	Name    string  `json:"name"`
	Default bool    `json:"default"`
	Note    string  `json:"note,omitempty"`
	MemMB   float64 `json:"memMB"`
	Count   int     `json:"count"`
}

func runningBackgroundApps() []runningApp {
	procs := listProcesses()
	var out []runningApp
	for _, a := range bgApps {
		ra := runningApp{Name: a.Name, Default: a.Default, Note: a.Note}
		for _, p := range procs {
			for _, x := range a.Exes {
				if strings.EqualFold(p.Name, x) {
					ra.Count++
					ra.MemMB += processMemMB(p.PID)
				}
			}
		}
		if ra.Count > 0 {
			out = append(out, ra)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MemMB > out[j].MemMB })
	return out
}

func closeApps(names []string) (int, float64) {
	want := map[string]bool{}
	for _, n := range names {
		for _, a := range bgApps {
			if a.Name == n {
				for _, x := range a.Exes {
					want[strings.ToLower(x)] = true
				}
			}
		}
	}
	killed := 0
	freed := 0.0
	for _, p := range listProcesses() {
		if !want[strings.ToLower(p.Name)] {
			continue
		}
		mem := processMemMB(p.PID)
		h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, p.PID)
		if err != nil {
			continue
		}
		if windows.TerminateProcess(h, 0) == nil {
			killed++
			freed += mem
		}
		windows.CloseHandle(h)
	}
	logf("Game Booster closed %d process(es), about %.0f MB", killed, freed)
	return killed, freed
}

const highPriorityClass = 0x00000080

// boostFortniteNow raises whichever supported game is running to high priority.
func boostFortniteNow() (bool, error) {
	g := runningGame(listProcesses())
	if g == nil {
		return false, nil
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_INFORMATION, false, g.pid)
	if err != nil {
		return true, err
	}
	defer windows.CloseHandle(h)
	return true, windows.SetPriorityClass(h, highPriorityClass)
}

// ───────────────────────── memory standby list ─────────────────────────

var (
	ntdll                        = windows.NewLazySystemDLL("ntdll.dll")
	procNtSetSystemInformation   = ntdll.NewProc("NtSetSystemInformation")
	procNtSetTimerResolution     = ntdll.NewProc("NtSetTimerResolution")
	procNtQueryTimerResolution   = ntdll.NewProc("NtQueryTimerResolution")
	procNtQuerySystemInformation = ntdll.NewProc("NtQuerySystemInformation")
)

func enablePrivilege(name string) error {
	var tok windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &tok); err != nil {
		return err
	}
	defer tok.Close()
	var luid windows.LUID
	n, _ := windows.UTF16PtrFromString(name)
	if err := windows.LookupPrivilegeValue(nil, n, &luid); err != nil {
		return err
	}
	tp := windows.Tokenprivileges{PrivilegeCount: 1}
	tp.Privileges[0] = windows.LUIDAndAttributes{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}
	return windows.AdjustTokenPrivileges(tok, false, &tp, 0, nil, nil)
}

const (
	systemMemoryListInformation = 80
	memoryPurgeStandbyList      = 4
	memoryEmptyWorkingSets      = 2
)

// systemMemoryListInfo mirrors SYSTEM_MEMORY_LIST_INFORMATION (we only need the standby counts).
type systemMemoryListInfo struct {
	ZeroPageCount             uintptr
	FreePageCount             uintptr
	ModifiedPageCount         uintptr
	ModifiedNoWritePageCount  uintptr
	BadPageCount              uintptr
	PageCountByPriority       [8]uintptr
	RepurposedPagesByPriority [8]uintptr
	ModifiedPageCountPageFile uintptr
}

func standbyMB() float64 {
	var info systemMemoryListInfo
	r, _, _ := procNtQuerySystemInformation.Call(systemMemoryListInformation, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info), 0)
	if r != 0 {
		return -1
	}
	var pages uintptr
	for _, p := range info.PageCountByPriority {
		pages += p
	}
	return float64(pages) * 4096 / (1 << 20)
}

func purgeStandby() (float64, error) {
	before := memStatus().AvailPhys
	if err := enablePrivilege("SeProfileSingleProcessPrivilege"); err != nil {
		return 0, err
	}
	_ = enablePrivilege("SeIncreaseQuotaPrivilege")
	cmd := uint32(memoryEmptyWorkingSets)
	procNtSetSystemInformation.Call(systemMemoryListInformation, uintptr(unsafe.Pointer(&cmd)), 4)
	cmd = memoryPurgeStandbyList
	r, _, _ := procNtSetSystemInformation.Call(systemMemoryListInformation, uintptr(unsafe.Pointer(&cmd)), 4)
	if r != 0 {
		return 0, fmt.Errorf("Windows refused the memory purge (0x%X)", r)
	}
	after := memStatus().AvailPhys
	freed := (float64(after) - float64(before)) / (1 << 20)
	logf("Purged standby memory, %.0f MB more free", freed)
	return freed, nil
}

// ───────────────────────── timer resolution ─────────────────────────

var (
	timerMu   sync.Mutex
	timerHeld bool
)

func timerResolution() (minR, maxR, cur float64) {
	var mn, mx, c uint32
	procNtQueryTimerResolution.Call(uintptr(unsafe.Pointer(&mn)), uintptr(unsafe.Pointer(&mx)), uintptr(unsafe.Pointer(&c)))
	return float64(mn) / 10000, float64(mx) / 10000, float64(c) / 10000
}

func setTimer(on bool) float64 {
	timerMu.Lock()
	defer timerMu.Unlock()
	var cur uint32
	if on && !timerHeld {
		_, mx, _ := timerResolution()
		want := uint32(mx * 10000)
		if want < 5000 {
			want = 5000
		}
		procNtSetTimerResolution.Call(uintptr(want), 1, uintptr(unsafe.Pointer(&cur)))
		timerHeld = true
		logf("Timer resolution held at %.3f ms while Astro Tweaks is open", float64(cur)/10000)
	} else if !on && timerHeld {
		procNtSetTimerResolution.Call(0, 0, uintptr(unsafe.Pointer(&cur)))
		timerHeld = false
		logf("Timer resolution released")
	}
	_, _, c := timerResolution()
	return c
}

// ───────────────────────── cleaner ─────────────────────────

type cleanTarget struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Desc    string   `json:"desc"`
	Default bool     `json:"default"`
	Paths   []string `json:"-"`
	Pattern string   `json:"-"` // optional filename glob inside each path
	Close   string   `json:"close,omitempty"`
}

var cleanTargets = []cleanTarget{
	{ID: "user-temp", Title: "Your temporary files", Desc: "Leftovers apps write to your Temp folder.", Default: true, Paths: []string{`%TEMP%`}},
	{ID: "win-temp", Title: "Windows temporary files", Desc: "Installer and update leftovers in Windows\\Temp.", Default: true, Paths: []string{`%SystemRoot%\Temp`}},
	{ID: "fn-logs", Title: "Fortnite logs and crash reports", Desc: "Old log files and crash dumps. Your Fortnite settings, keybinds and replays are not touched.", Default: true, Paths: []string{`%LOCALAPPDATA%\FortniteGame\Saved\Logs`, `%LOCALAPPDATA%\FortniteGame\Saved\Crashes`}},
	{ID: "fn-web", Title: "Fortnite web cache", Desc: "Cached in-game web pages (news, Item Shop). Re-downloaded when needed.", Default: true, Paths: []string{`%LOCALAPPDATA%\FortniteGame\Saved\webcache`, `%LOCALAPPDATA%\FortniteGame\Saved\webcache_4147`, `%LOCALAPPDATA%\FortniteGame\Saved\webcache_4430`, `%LOCALAPPDATA%\FortniteGame\Saved\webcache_6478`}, Close: "Fortnite"},
	{ID: "epic-web", Title: "Epic Games Launcher cache and logs", Desc: "The launcher's web cache and old logs. Your launcher settings are kept.", Default: true, Paths: []string{`%LOCALAPPDATA%\EpicGamesLauncher\Saved\webcache`, `%LOCALAPPDATA%\EpicGamesLauncher\Saved\webcache_4147`, `%LOCALAPPDATA%\EpicGamesLauncher\Saved\webcache_4430`, `%LOCALAPPDATA%\EpicGamesLauncher\Saved\Logs`}, Close: "Epic Games Launcher"},
	{ID: "dx-cache", Title: "DirectX shader cache", Desc: "Compiled shaders. Clear after a GPU driver update; your first match afterwards may stutter while shaders rebuild.", Default: false, Paths: []string{`%LOCALAPPDATA%\D3DSCache`}},
	{ID: "nv-cache", Title: "NVIDIA shader cache", Desc: "NVIDIA's compiled shader cache. Same trade-off as above.", Default: false, Paths: []string{`%LOCALAPPDATA%\NVIDIA\DXCache`, `%LOCALAPPDATA%\NVIDIA\GLCache`, `%ProgramData%\NVIDIA Corporation\NV_Cache`, `%LOCALAPPDATA%Low\NVIDIA\PerDriverVersion\DXCache`}},
	{ID: "amd-cache", Title: "AMD shader cache", Desc: "AMD's compiled shader cache. Same trade-off as above.", Default: false, Paths: []string{`%LOCALAPPDATA%\AMD\DxCache`, `%LOCALAPPDATA%\AMD\DxcCache`, `%LOCALAPPDATA%\AMD\GLCache`, `%LOCALAPPDATA%\AMD\VkCache`}},
	{ID: "discord-cache", Title: "Discord cache", Desc: "Cached images and code. Discord re-downloads what it needs.", Default: true, Paths: []string{`%APPDATA%\discord\Cache`, `%APPDATA%\discord\Code Cache`, `%APPDATA%\discord\GPUCache`}, Close: "Discord"},
	{ID: "wu-cache", Title: "Windows Update download cache", Desc: "Update files that have already been installed.", Default: true, Paths: []string{`%SystemRoot%\SoftwareDistribution\Download`}},
	{ID: "do-cache", Title: "Delivery Optimization cache", Desc: "Update pieces kept to share with other PCs.", Default: true, Paths: []string{`%SystemRoot%\ServiceProfiles\NetworkService\AppData\Local\Microsoft\Windows\DeliveryOptimization\Cache`}},
	{ID: "wer", Title: "Error reports", Desc: "Queued and archived Windows crash reports.", Default: true, Paths: []string{`%ProgramData%\Microsoft\Windows\WER\ReportArchive`, `%ProgramData%\Microsoft\Windows\WER\ReportQueue`, `%LOCALAPPDATA%\CrashDumps`}},
	{ID: "dumps", Title: "System crash dumps", Desc: "Memory dumps from blue screens. Keep these if you're troubleshooting crashes.", Default: false, Paths: []string{`%SystemRoot%\Minidump`, `%SystemRoot%\LiveKernelReports`}},
	{ID: "thumbs", Title: "Thumbnail cache", Desc: "Picture previews Explorer rebuilds automatically.", Default: false, Paths: []string{`%LOCALAPPDATA%\Microsoft\Windows\Explorer`}, Pattern: "thumbcache_*.db"},
	{ID: "browser", Title: "Browser caches (Chrome, Edge, Brave)", Desc: "Cached web files. Logins and history are kept. Close your browsers first.", Default: false, Paths: []string{`%LOCALAPPDATA%\Google\Chrome\User Data\*\Cache`, `%LOCALAPPDATA%\Google\Chrome\User Data\*\Code Cache`, `%LOCALAPPDATA%\Microsoft\Edge\User Data\*\Cache`, `%LOCALAPPDATA%\Microsoft\Edge\User Data\*\Code Cache`, `%LOCALAPPDATA%\BraveSoftware\Brave-Browser\User Data\*\Cache`}},
	{ID: "recycle", Title: "Recycle Bin", Desc: "Permanently deletes everything in the Recycle Bin.", Default: false},
}

// safePath refuses anything that could be a drive root, a profile root or a Fortnite config folder.
func safePath(p string) bool {
	clean := filepath.Clean(p)
	low := strings.ToLower(clean)
	if len(clean) < 12 || strings.Contains(p, "%") || strings.Contains(p, "$") {
		return false
	}
	if vol := filepath.VolumeName(clean); strings.EqualFold(clean, vol+`\`) || strings.EqualFold(clean, vol) {
		return false
	}
	for _, bad := range []string{`\fortnitegame\saved\config`, `\fortnitegame\saved\cloud`, `\fortnitegame\saved\demos`, `\epicgameslauncher\saved\config`} {
		if strings.Contains(low, bad) {
			return false
		}
	}
	for _, root := range []string{os.Getenv("USERPROFILE"), os.Getenv("LOCALAPPDATA"), os.Getenv("APPDATA"), os.Getenv("SystemRoot"), os.Getenv("ProgramData"), os.Getenv("ProgramFiles")} {
		if root != "" && strings.EqualFold(clean, filepath.Clean(root)) {
			return false
		}
	}
	return true
}

func resolveTargetPaths(t cleanTarget) []string {
	var out []string
	for _, p := range t.Paths {
		ep := expandEnv(p)
		if strings.Contains(ep, "*") {
			m, _ := filepath.Glob(ep)
			out = append(out, m...)
		} else {
			out = append(out, ep)
		}
	}
	var safe []string
	for _, p := range out {
		if safePath(p) {
			safe = append(safe, p)
		}
	}
	return safe
}

func walkTarget(t cleanTarget, del bool) (size int64, files int, failed int) {
	cutoff := time.Now().Add(-2 * time.Minute) // never touch files written in the last 2 minutes
	for _, root := range resolveTargetPaths(t) {
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		var dirs []string
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
				return nil // never follow links or junctions out of the folder
			}
			if d.IsDir() {
				if p != root {
					dirs = append(dirs, p)
				}
				return nil
			}
			if t.Pattern != "" {
				if ok, _ := filepath.Match(t.Pattern, d.Name()); !ok {
					return nil
				}
			}
			fi, err := d.Info()
			if err != nil {
				return nil
			}
			if fi.ModTime().After(cutoff) {
				return nil
			}
			if del {
				if os.Remove(p) == nil {
					size += fi.Size()
					files++
				} else {
					failed++
				}
			} else {
				size += fi.Size()
				files++
			}
			return nil
		})
		if del && t.Pattern == "" {
			sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
			for _, d := range dirs {
				_ = os.Remove(d) // only removes if empty
			}
		}
	}
	return
}

var (
	shell32               = windows.NewLazySystemDLL("shell32.dll")
	procSHQueryRecycleBin = shell32.NewProc("SHQueryRecycleBinW")
	procSHEmptyRecycleBin = shell32.NewProc("SHEmptyRecycleBinW")
)

type shQueryRBInfo struct {
	Size      uint32
	_         uint32
	TotalSize int64
	NumItems  int64
}

func recycleBinSize() (int64, int64) {
	var info shQueryRBInfo
	info.Size = uint32(unsafe.Sizeof(info))
	r, _, _ := procSHQueryRecycleBin.Call(0, uintptr(unsafe.Pointer(&info)))
	if r != 0 {
		return 0, 0
	}
	return info.TotalSize, info.NumItems
}

type cleanResult struct {
	ID     string `json:"id"`
	Bytes  int64  `json:"bytes"`
	Files  int    `json:"files"`
	Failed int    `json:"failed"`
}

func cleanerScan() []cleanResult {
	var out []cleanResult
	for _, t := range cleanTargets {
		if t.ID == "recycle" {
			s, n := recycleBinSize()
			out = append(out, cleanResult{ID: t.ID, Bytes: s, Files: int(n)})
			continue
		}
		s, n, _ := walkTarget(t, false)
		out = append(out, cleanResult{ID: t.ID, Bytes: s, Files: n})
	}
	return out
}

func cleanerRun(ids []string) []cleanResult {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []cleanResult
	for _, t := range cleanTargets {
		if !want[t.ID] {
			continue
		}
		if t.ID == "recycle" {
			s, n := recycleBinSize()
			const noConfirm, noProgress, noSound = 0x1, 0x2, 0x4
			procSHEmptyRecycleBin.Call(0, 0, noConfirm|noProgress|noSound)
			out = append(out, cleanResult{ID: t.ID, Bytes: s, Files: int(n)})
			continue
		}
		s, n, f := walkTarget(t, true)
		logf("Cleaner: %s → removed %d files (%.1f MB), %d in use and skipped", t.Title, n, float64(s)/(1<<20), f)
		out = append(out, cleanResult{ID: t.ID, Bytes: s, Files: n, Failed: f})
	}
	return out
}

// ───────────────────────── startup apps ─────────────────────────

type startupItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Command  string `json:"command"`
	Location string `json:"location"`
	Enabled  bool   `json:"enabled"`
	Scope    string `json:"scope"` // you / all users
}

type startupSource struct {
	root     registry.Key
	runPath  string
	approved string
	label    string
	scope    string
}

var startupSources = []startupSource{
	{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`, "Registry", "You"},
	{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, `SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`, "Registry", "All users"},
	{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`, `SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run32`, "Registry (32-bit)", "All users"},
}

func approvedEnabled(root registry.Key, path, name string) bool {
	v := readRaw(root, path, name)
	if !v.Exists || len(v.Data) == 0 {
		return true
	}
	return v.Data[0]&1 == 0 // 02/06 enabled, 03/07 disabled
}

func listStartup() []startupItem {
	var out []startupItem
	for si, s := range startupSources {
		k, err := registry.OpenKey(s.root, s.runPath, registry.QUERY_VALUE|registry.WOW64_64KEY)
		if err != nil {
			continue
		}
		names, _ := k.ReadValueNames(-1)
		for _, n := range names {
			if n == "" {
				continue
			}
			cmd, _, _ := k.GetStringValue(n)
			out = append(out, startupItem{ID: fmt.Sprintf("r%d|%s", si, n), Name: n, Command: cmd, Location: s.label, Scope: s.scope, Enabled: approvedEnabled(s.root, s.approved, n)})
		}
		k.Close()
	}
	folders := []struct{ dir, scope string }{
		{expandEnv(`%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup`), "You"},
		{expandEnv(`%ProgramData%\Microsoft\Windows\Start Menu\Programs\StartUp`), "All users"},
	}
	for fi, f := range folders {
		ents, _ := os.ReadDir(f.dir)
		for _, e := range ents {
			if e.IsDir() || strings.EqualFold(e.Name(), "desktop.ini") {
				continue
			}
			root := registry.CURRENT_USER
			if fi == 1 {
				root = registry.LOCAL_MACHINE
			}
			path := `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\StartupFolder`
			out = append(out, startupItem{ID: fmt.Sprintf("f%d|%s", fi, e.Name()), Name: strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())), Command: filepath.Join(f.dir, e.Name()), Location: "Startup folder", Scope: f.scope, Enabled: approvedEnabled(root, path, e.Name())})
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// setStartup flips the same switch Task Manager uses, so it's fully reversible there too.
func setStartup(id string, enabled bool) error {
	parts := strings.SplitN(id, "|", 2)
	if len(parts) != 2 || len(parts[0]) < 2 {
		return errors.New("unknown startup item")
	}
	var root registry.Key
	var path string
	switch parts[0][0] {
	case 'r':
		idx := int(parts[0][1] - '0')
		if idx < 0 || idx >= len(startupSources) {
			return errors.New("unknown startup item")
		}
		root, path = startupSources[idx].root, startupSources[idx].approved
	case 'f':
		root = registry.CURRENT_USER
		if parts[0] == "f1" {
			root = registry.LOCAL_MACHINE
		}
		path = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\StartupFolder`
	default:
		return errors.New("unknown startup item")
	}
	data := make([]byte, 12)
	if enabled {
		data[0] = 2
	} else {
		data[0] = 3
		ft := windows.NsecToFiletime(time.Now().UnixNano())
		binary.LittleEndian.PutUint32(data[4:], ft.LowDateTime)
		binary.LittleEndian.PutUint32(data[8:], ft.HighDateTime)
	}
	k, _, err := registry.CreateKey(root, path, regAccess)
	if err != nil {
		return err
	}
	defer k.Close()
	logf("Startup: %s → %v", parts[1], enabled)
	return k.SetBinaryValue(parts[1], data)
}

// ───────────────────────── ping ─────────────────────────

var (
	iphlpapi            = windows.NewLazySystemDLL("iphlpapi.dll")
	procIcmpCreateFile  = iphlpapi.NewProc("IcmpCreateFile")
	procIcmpSendEcho    = iphlpapi.NewProc("IcmpSendEcho")
	procIcmpCloseHandle = iphlpapi.NewProc("IcmpCloseHandle")
)

type icmpEchoReply struct {
	Address       uint32
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	DataPtr       uintptr
	Options       [16]byte
}

type pingRegion struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Host string `json:"host"`
}

var pingRegions = []pingRegion{
	{"oce", "Oceania", "ping-oce.ds.on.epicgames.com"},
	{"asia", "Asia", "ping-asia.ds.on.epicgames.com"},
	{"me", "Middle East", "ping-me.ds.on.epicgames.com"},
	{"eu", "Europe", "ping-eu.ds.on.epicgames.com"},
	{"nae", "NA East", "ping-nae.ds.on.epicgames.com"},
	{"nac", "NA Central", "ping-nac.ds.on.epicgames.com"},
	{"naw", "NA West", "ping-naw.ds.on.epicgames.com"},
	{"br", "Brazil", "ping-br.ds.on.epicgames.com"},
}

type pingResult struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Avg    float64 `json:"avg"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Jitter float64 `json:"jitter"`
	Loss   int     `json:"loss"`
	Error  string  `json:"error,omitempty"`
}

func pingHost(host string, count int) pingResult {
	r := pingResult{}
	ips, err := net.LookupIP(host)
	var ip4 net.IP
	for _, ip := range ips {
		if v := ip.To4(); v != nil {
			ip4 = v
			break
		}
	}
	if err != nil || ip4 == nil {
		r.Error = "Server not reachable (DNS)"
		r.Loss = 100
		return r
	}
	h, _, _ := procIcmpCreateFile.Call()
	if h == 0 || h == ^uintptr(0) {
		r.Error = "ICMP unavailable"
		return r
	}
	defer procIcmpCloseHandle.Call(h)
	addr := binary.LittleEndian.Uint32(ip4)
	data := []byte("astrospeed-ping")
	buf := make([]byte, int(unsafe.Sizeof(icmpEchoReply{}))+len(data)+8+64)
	var times []float64
	lost := 0
	for i := 0; i < count; i++ {
		n, _, _ := procIcmpSendEcho.Call(h, uintptr(addr), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 1500)
		if n == 0 {
			lost++
			continue
		}
		rep := (*icmpEchoReply)(unsafe.Pointer(&buf[0]))
		if rep.Status != 0 {
			lost++
			continue
		}
		times = append(times, float64(rep.RoundTripTime))
		time.Sleep(120 * time.Millisecond)
	}
	r.Loss = lost * 100 / count
	if len(times) == 0 {
		r.Error = "No reply (server may block ping)"
		return r
	}
	r.Min, r.Max = times[0], times[0]
	sum := 0.0
	for i, t := range times {
		sum += t
		if t < r.Min {
			r.Min = t
		}
		if t > r.Max {
			r.Max = t
		}
		if i > 0 {
			d := t - times[i-1]
			if d < 0 {
				d = -d
			}
			r.Jitter += d
		}
	}
	r.Avg = sum / float64(len(times))
	if len(times) > 1 {
		r.Jitter /= float64(len(times) - 1)
	}
	return r
}

func pingAll() []pingResult {
	out := make([]pingResult, len(pingRegions))
	var wg sync.WaitGroup
	for i, reg := range pingRegions {
		wg.Add(1)
		go func(i int, reg pingRegion) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					out[i] = pingResult{ID: reg.ID, Name: reg.Name, Error: "Ping failed", Loss: 100}
				}
			}()
			r := pingHost(reg.Host, 6)
			r.ID, r.Name = reg.ID, reg.Name
			out[i] = r
		}(i, reg)
	}
	wg.Wait()
	return out
}

// ───────────────────────── DNS ─────────────────────────

var dnsPresets = map[string][]string{
	"cloudflare": {"1.1.1.1", "1.0.0.1"},
	"google":     {"8.8.8.8", "8.8.4.4"},
	"quad9":      {"9.9.9.9", "149.112.112.112"},
}

func getDNS() string {
	out, err := runPS(psDNSGet(), 30*time.Second)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func setDNS(preset string) error {
	var ips []string
	if preset != "auto" {
		var ok bool
		if ips, ok = dnsPresets[preset]; !ok {
			return errors.New("unknown DNS preset")
		}
	}
	_, err := runPS(psDNSSet(ips), 60*time.Second)
	logf("DNS set to %s", preset)
	return err
}

// ───────────────────────── restore point / reboot ─────────────────────────

func createRestorePoint(label string) error {
	script := psRestorePoint(label)
	out, err := runPS(script, 6*time.Minute)
	ok := err == nil && strings.Contains(out, "OK")
	stateMu.Lock()
	appState.RestorePoints = append(appState.RestorePoints, RestorePoint{When: time.Now(), Label: label, OK: ok})
	saveStateLocked()
	stateMu.Unlock()
	if !ok {
		if err == nil {
			err = errors.New(firstLine(out))
		}
		logf("Restore point failed: %v", err)
		return err
	}
	logf("Restore point created: %s", label)
	return nil
}

func restartPC(toFirmware bool) error {
	if toFirmware {
		_, err := runExe(20*time.Second, filepath.Join(os.Getenv("SystemRoot"), `System32\shutdown.exe`), "/r", "/fw", "/t", "5")
		if err != nil {
			return errors.New("This PC can't restart straight into BIOS (it may use legacy BIOS). Restart and press Del or F2 while it boots")
		}
		return nil
	}
	_, err := runExe(20*time.Second, filepath.Join(os.Getenv("SystemRoot"), `System32\shutdown.exe`), "/r", "/t", "5")
	return err
}
