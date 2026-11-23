//go:build windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const appName = "AstroSpeed Tweaks"
const appVersion = "1.6.0"
const exeName = "AstroSpeed Tweaks.exe"
const uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\AstroSpeedTweaks`

func installDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = os.Getenv("APPDATA")
	}
	return filepath.Join(base, "Programs", "AstroSpeed Tweaks")
}

func startMenuLnk() string {
	return filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs`, appName+".lnk")
}
func desktopLnk() string {
	home := os.Getenv("USERPROFILE")
	return filepath.Join(home, "Desktop", appName+".lnk")
}

// doInstall copies the app, makes shortcuts and registers the uninstaller.
func doInstall(desktop bool) (string, error) {
	dir := installDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(dir, exeName)
	// close any running copy is the app's job; just overwrite
	if err := os.WriteFile(target, appExe, 0o755); err != nil {
		return "", err
	}
	// the installer copies itself in as the uninstaller
	if self, err := os.Executable(); err == nil {
		if b, err := os.ReadFile(self); err == nil {
			_ = os.WriteFile(filepath.Join(dir, "uninstall.exe"), b, 0o755)
		}
	}
	_ = makeShortcut(startMenuLnk(), target, dir)
	if desktop {
		_ = makeShortcut(desktopLnk(), target, dir)
	}
	writeUninstallKey(dir, target)
	return target, nil
}

func doUninstall() {
	dir := installDir()
	_ = os.Remove(startMenuLnk())
	_ = os.Remove(desktopLnk())
	deleteUninstallKey()
	// remove everything except the running uninstaller, then schedule its own deletion
	entries, _ := os.ReadDir(dir)
	self, _ := os.Executable()
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if p != self {
			_ = os.RemoveAll(p)
		}
	}
	scheduleSelfDelete(self)
}

// ───────────────────────── registry ─────────────────────────

func writeUninstallKey(dir, target string) {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, uninstallKey, registry.WRITE)
	if err != nil {
		return
	}
	defer k.Close()
	k.SetStringValue("DisplayName", appName)
	k.SetStringValue("DisplayVersion", appVersion)
	k.SetStringValue("Publisher", "AstroSpeed")
	k.SetStringValue("DisplayIcon", target)
	k.SetStringValue("InstallLocation", dir)
	k.SetStringValue("UninstallString", `"`+filepath.Join(dir, "uninstall.exe")+`" --uninstall`)
	k.SetDWordValue("NoModify", 1)
	k.SetDWordValue("NoRepair", 1)
}

func deleteUninstallKey() {
	registry.DeleteKey(registry.CURRENT_USER, uninstallKey)
}

// ───────────────────────── shortcut (IShellLinkW via COM) ─────────────────────────

var (
	ole32            = windows.NewLazySystemDLL("ole32.dll")
	procCoInitialize = ole32.NewProc("CoInitializeEx")
	procCoCreate     = ole32.NewProc("CoCreateInstance")
)

// GUIDs
var (
	clsidShellLink = guid{0x00021401, 0x0000, 0x0000, [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidShellLinkW  = guid{0x000214F9, 0x0000, 0x0000, [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidPersistFile = guid{0x0000010b, 0x0000, 0x0000, [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

func makeShortcut(lnkPath, target, workdir string) error {
	procCoInitialize.Call(0, 6) // COINIT_APARTMENTTHREADED|DISABLE_OLE1DDE
	var psl uintptr
	r, _, _ := procCoCreate.Call(uintptr(unsafe.Pointer(&clsidShellLink)), 0, 1 /*CLSCTX_INPROC_SERVER*/, uintptr(unsafe.Pointer(&iidShellLinkW)), uintptr(unsafe.Pointer(&psl)))
	if r != 0 || psl == 0 {
		return syscall.EINVAL
	}
	vt := *(*[21]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(psl))))
	defer syscall.Syscall(vt[2], 1, psl, 0, 0) // Release
	tp, _ := windows.UTF16PtrFromString(target)
	wp, _ := windows.UTF16PtrFromString(workdir)
	syscall.Syscall(vt[20], 2, psl, uintptr(unsafe.Pointer(tp)), 0)  // SetPath
	syscall.Syscall(vt[9], 2, psl, uintptr(unsafe.Pointer(wp)), 0)   // SetWorkingDirectory
	syscall.Syscall6(vt[17], 3, psl, uintptr(unsafe.Pointer(tp)), 0, 0, 0, 0) // SetIconLocation

	// QueryInterface for IPersistFile
	var ppf uintptr
	syscall.Syscall(vt[0], 3, psl, uintptr(unsafe.Pointer(&iidPersistFile)), uintptr(unsafe.Pointer(&ppf)))
	if ppf == 0 {
		return syscall.EINVAL
	}
	pvt := *(*[8]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(ppf))))
	defer syscall.Syscall(pvt[2], 1, ppf, 0, 0) // Release
	lp, _ := windows.UTF16PtrFromString(lnkPath)
	syscall.Syscall(pvt[6], 3, ppf, uintptr(unsafe.Pointer(lp)), 1) // Save(path, fRemember)
	return nil
}

// scheduleSelfDelete removes the uninstaller after it exits, with a tiny detached cmd.
func scheduleSelfDelete(self string) {
	dir := filepath.Dir(self)
	cmd := exeName // unused placeholder to keep imports tidy
	_ = cmd
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = `C:\Windows\System32\cmd.exe`
	}
	// ping delay then delete the exe and the folder
	line := `/c ping 127.0.0.1 -n 3 >nul & del /q "` + self + `" & rmdir "` + dir + `"`
	argv, _ := windows.UTF16PtrFromString(`"` + comspec + `" ` + line)
	var si windows.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	si.Flags = windows.STARTF_USESHOWWINDOW // hidden
	var pi windows.ProcessInformation
	cp, _ := windows.UTF16PtrFromString(comspec)
	windows.CreateProcess(cp, argv, nil, nil, false, windows.CREATE_NO_WINDOW|windows.DETACHED_PROCESS, nil, nil, &si, &pi)
}
