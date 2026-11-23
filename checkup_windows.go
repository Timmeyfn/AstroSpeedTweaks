//go:build windows

package main

import (
	"encoding/binary"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// PC checkup: read-only facts the dashboard can't see from tweaks alone.
// Nothing here changes anything.

var (
	procEnumDisplayDevices  = user32.NewProc("EnumDisplayDevicesW")
	procEnumDisplaySettings = user32.NewProc("EnumDisplaySettingsW")
	procPowerReadFriendly   = powrprof.NewProc("PowerReadFriendlyName")
)

type displayInfo struct {
	Name    string `json:"name"`
	GPU     string `json:"gpu"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Hz      int    `json:"hz"`
	MaxHz   int    `json:"maxHz"`
	Primary bool   `json:"primary"`
}

type checkupInfo struct {
	Displays    []displayInfo `json:"displays"`
	PowerPlan   string        `json:"powerPlan"`
	UptimeHours float64       `json:"uptimeHours"`
	MemLoad     int           `json:"memLoad"`
}

const devmodeSize = 220

func displaySettings(dev *uint16, mode int32) (w, h, hz int, ok bool) {
	var dm [devmodeSize]byte
	binary.LittleEndian.PutUint16(dm[68:], devmodeSize) // dmSize
	r, _, _ := procEnumDisplaySettings.Call(uintptr(unsafe.Pointer(dev)), uintptr(mode), uintptr(unsafe.Pointer(&dm[0])))
	if r == 0 {
		return 0, 0, 0, false
	}
	return int(binary.LittleEndian.Uint32(dm[172:])), int(binary.LittleEndian.Uint32(dm[176:])), int(binary.LittleEndian.Uint32(dm[184:])), true
}

func listDisplays() []displayInfo {
	var out []displayInfo
	for i := 0; i < 16; i++ {
		var dd [840]byte
		binary.LittleEndian.PutUint32(dd[0:], 840)
		if r, _, _ := procEnumDisplayDevices.Call(0, uintptr(i), uintptr(unsafe.Pointer(&dd[0])), 0); r == 0 {
			break
		}
		flags := binary.LittleEndian.Uint32(dd[324:])
		if flags&0x1 == 0 { // not attached to the desktop
			continue
		}
		devName := (*[32]uint16)(unsafe.Pointer(&dd[4]))[:]
		gpu := windows.UTF16ToString((*[128]uint16)(unsafe.Pointer(&dd[68]))[:])
		w, h, hz, ok := displaySettings(&devName[0], -1) // ENUM_CURRENT_SETTINGS
		if !ok || w == 0 {
			continue
		}
		maxHz := hz
		for m := int32(0); m < 2000; m++ {
			mw, mh, mhz, ok := displaySettings(&devName[0], m)
			if !ok {
				break
			}
			if mw == w && mh == h && mhz > maxHz && mhz < 1000 {
				maxHz = mhz
			}
		}
		out = append(out, displayInfo{Name: "Display " + strconv.Itoa(len(out)+1), GPU: gpu, Width: w, Height: h, Hz: hz, MaxHz: maxHz, Primary: flags&0x4 != 0})
	}
	return out
}

func activePowerPlan() string {
	var g *windows.GUID
	if r, _, _ := procPowerGetActiveScheme.Call(0, uintptr(unsafe.Pointer(&g))); r != 0 || g == nil {
		return ""
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(g)))
	buf := make([]uint16, 256)
	size := uint32(len(buf) * 2)
	if r, _, _ := procPowerReadFriendly.Call(0, uintptr(unsafe.Pointer(g)), 0, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r != 0 {
		return ""
	}
	return strings.TrimSpace(windows.UTF16ToString(buf))
}

func checkup() checkupInfo {
	defer func() { _ = recover() }()
	c := checkupInfo{Displays: listDisplays(), PowerPlan: activePowerPlan()}
	up, _, _ := procGetTickCount64.Call()
	c.UptimeHours = float64(time.Duration(up)*time.Millisecond) / float64(time.Hour)
	ms := memoryStatusEx{}
	ms.Length = uint32(unsafe.Sizeof(ms))
	if r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms))); r != 0 {
		c.MemLoad = int(ms.MemoryLoad)
	}
	if c.Displays == nil {
		c.Displays = []displayInfo{}
	}
	return c
}

// pingOnce sends one ping to a Fortnite region, for the live ping graph.
func pingOnce(id string) pingResult {
	for _, reg := range pingRegions {
		if reg.ID == id {
			r := pingHost(reg.Host, 1)
			r.ID, r.Name = reg.ID, reg.Name
			return r
		}
	}
	return pingResult{ID: id, Error: "Unknown region", Loss: 100}
}
