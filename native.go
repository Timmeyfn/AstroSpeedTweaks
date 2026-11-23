//go:build windows

package main

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Native replacements for things that used to need PowerShell or powercfg, so the app
// starts without launching any other program.

var (
	powrprof                  = windows.NewLazySystemDLL("powrprof.dll")
	procPowerGetActiveScheme  = powrprof.NewProc("PowerGetActiveScheme")
	procPowerReadACValueIndex = powrprof.NewProc("PowerReadACValueIndex")
	procPowerReadDCValueIndex = powrprof.NewProc("PowerReadDCValueIndex")
	procGetFirmwareTable      = kernel32.NewProc("GetSystemFirmwareTable")
	procGetFirmwareType       = kernel32.NewProc("GetFirmwareType")
	procLocalFree             = kernel32.NewProc("LocalFree")
)

func guidOf(s string) (windows.GUID, bool) {
	if !strings.HasPrefix(s, "{") {
		s = "{" + s + "}"
	}
	g, err := windows.GUIDFromString(s)
	return g, err == nil
}

func nativeActiveScheme() string {
	var p *windows.GUID
	r, _, _ := procPowerGetActiveScheme.Call(0, uintptr(unsafe.Pointer(&p)))
	if r != 0 || p == nil {
		return ""
	}
	s := strings.ToLower(strings.Trim(p.String(), "{}"))
	procLocalFree.Call(uintptr(unsafe.Pointer(p)))
	return s
}

func nativeReadPower(scheme, sub, set string, ac bool) (int, bool) {
	gs, ok1 := guidOf(scheme)
	gsub, ok2 := guidOf(sub)
	gset, ok3 := guidOf(set)
	if !ok1 || !ok2 || !ok3 {
		return 0, false
	}
	var v uint32
	proc := procPowerReadACValueIndex
	if !ac {
		proc = procPowerReadDCValueIndex
	}
	r, _, _ := proc.Call(0, uintptr(unsafe.Pointer(&gs)), uintptr(unsafe.Pointer(&gsub)), uintptr(unsafe.Pointer(&gset)), uintptr(unsafe.Pointer(&v)))
	if r != 0 {
		return 0, false
	}
	return int(v), true
}

func smbiosRaw() []byte {
	const rsmb = 0x52534D42
	n, _, _ := procGetFirmwareTable.Call(rsmb, 0, 0, 0)
	if n == 0 || n > 1<<20 {
		return nil
	}
	buf := make([]byte, n)
	got, _, _ := procGetFirmwareTable.Call(rsmb, 0, uintptr(unsafe.Pointer(&buf[0])), n)
	if got == 0 || got > n {
		return nil
	}
	return buf[:got]
}

func firmwareType() string {
	var t uint32
	r, _, _ := procGetFirmwareType.Call(uintptr(unsafe.Pointer(&t)))
	if r == 0 {
		return ""
	}
	switch t {
	case 1:
		return "Legacy BIOS"
	case 2:
		return "UEFI"
	}
	return ""
}

// probeNative fills in hardware details from the registry and SMBIOS. It never fails and never
// starts another process.
func probeNative(e *Env) {
	defer func() { _ = recover() }()
	bios := `HARDWARE\DESCRIPTION\System\BIOS`
	e.Board = strings.TrimSpace(regGetString(registry.LOCAL_MACHINE, bios, "BaseBoardManufacturer") + " " + regGetString(registry.LOCAL_MACHINE, bios, "BaseBoardProduct"))
	e.BIOS = strings.TrimSpace(regGetString(registry.LOCAL_MACHINE, bios, "BIOSVersion"))
	e.Firmware = firmwareType()
	if v, ok := regGetDword(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\SecureBoot\State`, "UEFISecureBootEnabled"); ok {
		b := v == 1
		e.SecureBoot = &b
	}
	e.VBS = 0
	if v, ok := regGetDword(registry.LOCAL_MACHINE, pDevGuard, "EnableVirtualizationBasedSecurity"); ok && v == 1 {
		e.VBS = 1
	}
	if v, ok := regGetDword(registry.LOCAL_MACHINE, pHVCI, "Enabled"); ok && v == 1 {
		e.HVCI = true
		e.VBS = 1
	}
	m := parseSMBIOSMemory(smbiosRaw())
	e.RAMSpeed, e.RAMConf, e.RAMSticks, e.RAMType = m.Speed, m.Configured, m.Sticks, m.Type
	e.RAMXMP = xmpVerdict(m)
	e.BitLocker = -1 // checked only when a boot setting is about to change
	e.Probed = true
}
