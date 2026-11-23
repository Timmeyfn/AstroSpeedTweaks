//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

const displayClass = `SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}`

type GPUInfo struct {
	Name       string  `json:"name"`
	Vendor     string  `json:"vendor"`
	VRAMGB     float64 `json:"vramGB"`
	Driver     string  `json:"driver"`
	DriverDate string  `json:"driverDate"`
}

type Env struct {
	OSName      string            `json:"osName"`
	OSVersion   string            `json:"osVersion"`
	Build       int               `json:"build"`
	CPU         string            `json:"cpu"`
	Threads     int               `json:"threads"`
	RAMGB       float64           `json:"ramGB"`
	GPUs        []GPUInfo         `json:"gpus"`
	Vendors     []string          `json:"vendors"`
	FortniteDir string            `json:"fortniteDir"`
	FortniteExe string            `json:"fortniteExe"`
	Games       map[string]string `json:"games"`
	Discord     bool              `json:"discord"`
	Admin       bool              `json:"admin"`
	Laptop      bool              `json:"laptop"`

	// filled in by the slower probe
	Probed     bool   `json:"probed"`
	BitLocker  int    `json:"bitlocker"` // 1 on, 0 off, -1 unknown
	RAMSpeed   int    `json:"ramSpeed"`
	RAMConf    int    `json:"ramConf"`
	RAMType    string `json:"ramType"`
	RAMSticks  int    `json:"ramSticks"`
	RAMXMP     string `json:"ramXmp"` // ok, likely_off, below_rated, unknown
	Board      string `json:"board"`
	BIOS       string `json:"bios"`
	Firmware   string `json:"firmware"`
	SecureBoot *bool  `json:"secureBoot"`
	VBS        int    `json:"vbs"`  // 0 off, 1 configured, 2 running, -1 unknown
	HVCI       bool   `json:"hvci"` // memory integrity running
}

var (
	envMu     sync.Mutex
	curEnv    Env
	probeMu   sync.Mutex
	lastProbe time.Time
)

// probeOnce re-reads hardware details (all native, takes a few milliseconds).
func probeOnce(force bool) {
	probeMu.Lock()
	defer probeMu.Unlock()
	if !force && time.Since(lastProbe) < 10*time.Second {
		return
	}
	refreshEnvFast()
	lastProbe = time.Now()
}

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

var procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")

func memStatus() memoryStatusEx {
	var m memoryStatusEx
	m.Length = uint32(unsafe.Sizeof(m))
	procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m)))
	return m
}

func detectEnv() Env {
	e := Env{Threads: runtime.NumCPU(), Admin: isElevated(), BitLocker: -1, VBS: -1, RAMXMP: "unknown"}

	cv := `SOFTWARE\Microsoft\Windows NT\CurrentVersion`
	e.Build, _ = strconv.Atoi(regGetString(registry.LOCAL_MACHINE, cv, "CurrentBuildNumber"))
	e.OSName = regGetString(registry.LOCAL_MACHINE, cv, "ProductName")
	if e.Build >= 22000 {
		e.OSName = strings.Replace(e.OSName, "Windows 10", "Windows 11", 1)
	}
	e.OSVersion = regGetString(registry.LOCAL_MACHINE, cv, "DisplayVersion")
	if ubr, ok := regGetDword(registry.LOCAL_MACHINE, cv, "UBR"); ok {
		e.OSVersion = strings.TrimSpace(e.OSVersion + " (build " + strconv.Itoa(e.Build) + "." + strconv.Itoa(int(ubr)) + ")")
	}

	e.CPU = strings.TrimSpace(regGetString(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "ProcessorNameString"))
	m := memStatus()
	e.RAMGB = float64(m.TotalPhys) / (1 << 30)

	seen := map[string]bool{}
	for _, sub := range regSubkeys(registry.LOCAL_MACHINE, displayClass) {
		if len(sub) != 4 || sub[0] < '0' || sub[0] > '9' {
			continue
		}
		p := displayClass + `\` + sub
		desc := regGetString(registry.LOCAL_MACHINE, p, "DriverDesc")
		if desc == "" {
			continue
		}
		match := strings.ToUpper(regGetString(registry.LOCAL_MACHINE, p, "MatchingDeviceId"))
		if strings.Contains(strings.ToLower(desc), "basic display") || strings.Contains(strings.ToLower(desc), "remote") || strings.Contains(strings.ToLower(desc), "virtual") {
			continue
		}
		g := GPUInfo{Name: desc, Driver: regGetString(registry.LOCAL_MACHINE, p, "DriverVersion"), DriverDate: regGetString(registry.LOCAL_MACHINE, p, "DriverDate")}
		switch {
		case strings.Contains(match, "VEN_10DE") || strings.Contains(strings.ToUpper(desc), "NVIDIA"):
			g.Vendor = "nvidia"
		case strings.Contains(match, "VEN_1002") || strings.Contains(strings.ToUpper(desc), "AMD") || strings.Contains(strings.ToUpper(desc), "RADEON"):
			g.Vendor = "amd"
		case strings.Contains(match, "VEN_8086") || strings.Contains(strings.ToUpper(desc), "INTEL"):
			g.Vendor = "intel"
		}
		if v := readRaw(registry.LOCAL_MACHINE, p, "HardwareInformation.qwMemorySize"); v.Exists && len(v.Data) >= 8 {
			g.VRAMGB = float64(leU64(v.Data)) / (1 << 30)
		} else if v := readRaw(registry.LOCAL_MACHINE, p, "HardwareInformation.MemorySize"); v.Exists && len(v.Data) >= 4 {
			g.VRAMGB = float64(leU32(v.Data)) / (1 << 30)
		}
		if seen[desc] {
			continue
		}
		seen[desc] = true
		e.GPUs = append(e.GPUs, g)
		if g.Vendor != "" && !contains(e.Vendors, g.Vendor) {
			e.Vendors = append(e.Vendors, g.Vendor)
		}
	}

	e.FortniteDir, e.FortniteExe = findFortnite()
	e.Discord = fileExists(expandEnv(`%APPDATA%\discord\settings.json`)) || fileExists(expandEnv(`%LOCALAPPDATA%\Discord\Update.exe`))

	// laptop if a battery is present
	e.Laptop = hasBattery()
	probeNative(&e)
	return e
}

func leU64(b []byte) uint64 {
	var v uint64
	for i := 7; i >= 0; i-- {
		v = v<<8 | uint64(b[i])
	}
	return v
}
func leU32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func findFortnite() (dir, exe string) {
	type inst struct {
		InstallLocation string `json:"InstallLocation"`
		AppName         string `json:"AppName"`
	}
	var list struct {
		InstallationList []inst `json:"InstallationList"`
	}
	if b, err := os.ReadFile(expandEnv(`%ProgramData%\Epic\UnrealEngineLauncher\LauncherInstalled.dat`)); err == nil {
		if json.Unmarshal(b, &list) == nil {
			for _, i := range list.InstallationList {
				if strings.EqualFold(i.AppName, "Fortnite") {
					dir = i.InstallLocation
				}
			}
		}
	}
	candidates := []string{dir}
	for _, d := range []string{"C", "D", "E", "F"} {
		candidates = append(candidates, d+`:\Program Files\Epic Games\Fortnite`, d+`:\Epic Games\Fortnite`, d+`:\Games\Fortnite`)
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		x := filepath.Join(c, `FortniteGame\Binaries\Win64\FortniteClient-Win64-Shipping.exe`)
		if fileExists(x) {
			return c, x
		}
	}
	return "", ""
}

type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

var procGetSystemPowerStatus = kernel32.NewProc("GetSystemPowerStatus")

func hasBattery() bool {
	var s systemPowerStatus
	r, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&s)))
	if r == 0 {
		return false
	}
	return s.BatteryFlag != 128 && s.BatteryFlag != 255
}

func getEnv() Env {
	envMu.Lock()
	defer envMu.Unlock()
	return curEnv
}

func refreshEnvFast() {
	e := detectEnv()
	envMu.Lock()
	e.Games = curEnv.Games
	if e.FortniteExe != "" {
		if e.Games == nil {
			e.Games = map[string]string{}
		}
		e.Games["fortnite"] = e.FortniteExe
	}
	curEnv = e
	envMu.Unlock()
}

// available reports whether a tweak's conditions are met on this PC.
func available(t Tweak, e Env) (bool, string) {
	for _, w := range t.When {
		if strings.HasPrefix(w, "game:") {
			if e.Games[w[5:]] == "" {
				g, _ := gameByID(w[5:])
				return false, g.Name + " not found on this PC"
			}
			continue
		}
		switch w {
		case "fortnite":
			if e.FortniteExe == "" {
				return false, "Fortnite not found on this PC"
			}
		case "discord":
			if !e.Discord {
				return false, "Discord not installed"
			}
		case "nvidia":
			if !contains(e.Vendors, "nvidia") {
				return false, "Needs an NVIDIA GPU"
			}
		case "amd":
			if !contains(e.Vendors, "amd") {
				return false, "Needs an AMD GPU"
			}
		case "ram16":
			if e.RAMGB < 15.0 {
				return false, "Needs 16 GB of RAM or more"
			}
		case "win11":
			if e.Build < 22000 {
				return false, "Windows 11 only"
			}
		}
	}
	return true, ""
}
