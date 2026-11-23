//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const createNoWindow = 0x08000000

// runCmd runs a command line through cmd.exe with no console window.
func runCmd(cmdline string, timeout time.Duration) (string, error) {
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = `C:\Windows\System32\cmd.exe`
	}
	c := exec.CommandContext(ctx, comspec)
	c.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
		CmdLine:       fmt.Sprintf(`"%s" /d /s /c "%s"`, comspec, cmdline),
	}
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	err := c.Run()
	s := strings.TrimSpace(decodeConsole(out.Bytes()))
	if ctx.Err() == context.DeadlineExceeded {
		return s, fmt.Errorf("timed out after %s", timeout)
	}
	if err != nil {
		if s != "" {
			return s, fmt.Errorf("%v: %s", err, firstLine(s))
		}
		return s, err
	}
	return s, nil
}

// runExe runs a program directly with arguments, hidden.
func runExe(timeout time.Duration, name string, args ...string) (string, error) {
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c := exec.CommandContext(ctx, name, args...)
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	err := c.Run()
	s := strings.TrimSpace(decodeConsole(out.Bytes()))
	if ctx.Err() == context.DeadlineExceeded {
		return s, fmt.Errorf("timed out after %s", timeout)
	}
	if err != nil {
		if s != "" {
			return s, fmt.Errorf("%v: %s", err, firstLine(s))
		}
		return s, err
	}
	return s, nil
}

// runPS runs a PowerShell script (UTF-8 output) with no window.
func runPS(script string, timeout time.Duration) (string, error) {
	// Passed as a plain -Command (no -EncodedCommand, no execution-policy bypass): those
	// switches are what antivirus heuristics look for, and plain commands need neither.
	full := psPrefix + strings.ReplaceAll(script, "\r\n", "\n")
	ps := filepath.Join(os.Getenv("SystemRoot"), `System32\WindowsPowerShell\v1.0\powershell.exe`)
	if _, err := os.Stat(ps); err != nil {
		ps = "powershell.exe"
	}
	if timeout == 0 {
		timeout = 90 * time.Second
	}
	out, err := runExe(timeout, ps, "-NoLogo", "-NoProfile", "-NonInteractive", "-InputFormat", "None", "-Command", full)
	return out, err
}

// decodeConsole converts console output to a string. PowerShell is forced to
// UTF-8; native tools (netsh, bcdedit, powercfg) use the OEM code page.
func decodeConsole(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	if s, err := oemToString(b); err == nil && s != "" {
		return s
	}
	return strings.ToValidUTF8(string(b), "?")
}

var (
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procMultiByteToWideChar = kernel32.NewProc("MultiByteToWideChar")
	procGetOEMCP            = kernel32.NewProc("GetOEMCP")
)

func oemToString(b []byte) (string, error) {
	if len(b) == 0 {
		return "", nil
	}
	cp, _, _ := procGetOEMCP.Call()
	n, _, err := procMultiByteToWideChar.Call(cp, 0, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0, 0)
	if n == 0 {
		return "", err
	}
	buf := make([]uint16, n)
	n, _, err = procMultiByteToWideChar.Call(cp, 0, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), uintptr(unsafe.Pointer(&buf[0])), n)
	if n == 0 {
		return "", err
	}
	return string(utf16.Decode(buf[:n])), nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	if len(s) > 220 {
		s = s[:220] + "…"
	}
	return s
}

// ───────────────────────── registry helpers ─────────────────────────

func rootKey(r string) (registry.Key, error) {
	switch strings.ToUpper(r) {
	case "HKLM":
		return registry.LOCAL_MACHINE, nil
	case "HKCU":
		return registry.CURRENT_USER, nil
	}
	return 0, fmt.Errorf("unknown registry root %q", r)
}

const regAccess = registry.QUERY_VALUE | registry.SET_VALUE | registry.ENUMERATE_SUB_KEYS | registry.WOW64_64KEY

type regValue struct {
	Exists bool   `json:"exists"`
	Type   uint32 `json:"type"`
	Data   []byte `json:"data"`
}

func keyExists(root registry.Key, path string) bool {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return false
	}
	k.Close()
	return true
}

func readRaw(root registry.Key, path, name string) regValue {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return regValue{}
	}
	defer k.Close()
	n, t, err := k.GetValue(name, nil)
	if err != nil && !errors.Is(err, registry.ErrShortBuffer) {
		return regValue{}
	}
	buf := make([]byte, n)
	if n > 0 {
		n, t, err = k.GetValue(name, buf)
		if err != nil {
			return regValue{}
		}
	}
	return regValue{Exists: true, Type: t, Data: buf[:n]}
}

// missingKeys returns the chain of keys (shallowest first) that do not yet exist.
func missingKeys(root registry.Key, path string) []string {
	parts := strings.Split(path, `\`)
	var missing []string
	for i := 1; i <= len(parts); i++ {
		p := strings.Join(parts[:i], `\`)
		if !keyExists(root, p) {
			for j := i; j <= len(parts); j++ {
				missing = append(missing, strings.Join(parts[:j], `\`))
			}
			break
		}
	}
	return missing
}

func writeRaw(root registry.Key, path, name string, v regValue) error {
	k, _, err := registry.CreateKey(root, path, regAccess)
	if err != nil {
		return err
	}
	defer k.Close()
	switch v.Type {
	case registry.DWORD:
		if len(v.Data) < 4 {
			return errors.New("bad DWORD data")
		}
		return k.SetDWordValue(name, binary.LittleEndian.Uint32(v.Data))
	case registry.QWORD:
		if len(v.Data) < 8 {
			return errors.New("bad QWORD data")
		}
		return k.SetQWordValue(name, binary.LittleEndian.Uint64(v.Data))
	case registry.SZ:
		return k.SetStringValue(name, utf16BytesToString(v.Data))
	case registry.EXPAND_SZ:
		return k.SetExpandStringValue(name, utf16BytesToString(v.Data))
	case registry.MULTI_SZ:
		s := utf16BytesToString(v.Data)
		parts := strings.Split(strings.TrimRight(s, "\x00"), "\x00")
		return k.SetStringsValue(name, parts)
	default:
		return k.SetBinaryValue(name, v.Data)
	}
}

func deleteValue(root registry.Key, path, name string) error {
	k, err := registry.OpenKey(root, path, registry.SET_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return nil
	}
	defer k.Close()
	err = k.DeleteValue(name)
	if err != nil && errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	return err
}

// deleteKeyIfEmpty removes a key only when it has no values and no subkeys left.
func deleteKeyIfEmpty(root registry.Key, path string) {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS|registry.WOW64_64KEY)
	if err != nil {
		return
	}
	st, err := k.Stat()
	k.Close()
	if err != nil || st.ValueCount > 0 || st.SubKeyCount > 0 {
		return
	}
	_ = registry.DeleteKey(root, path)
}

func utf16BytesToString(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	for len(u) > 0 && u[len(u)-1] == 0 {
		u = u[:len(u)-1]
	}
	return string(utf16.Decode(u))
}

func dwordBytes(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}
func qwordBytes(v uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	return b
}
func szBytes(s string) []byte {
	u := utf16.Encode([]rune(s + "\x00"))
	b := make([]byte, len(u)*2)
	for i, r := range u {
		binary.LittleEndian.PutUint16(b[i*2:], r)
	}
	return b
}

func targetValue(a Action) regValue {
	switch a.Type {
	case "DWORD":
		return regValue{Exists: true, Type: registry.DWORD, Data: dwordBytes(uint32(a.Num))}
	case "QWORD":
		return regValue{Exists: true, Type: registry.QWORD, Data: qwordBytes(a.Num)}
	default:
		return regValue{Exists: true, Type: registry.SZ, Data: szBytes(a.Str)}
	}
}

func regEqual(cur, want regValue) bool {
	if !cur.Exists || !want.Exists {
		return cur.Exists == want.Exists
	}
	if want.Type == registry.SZ || want.Type == registry.EXPAND_SZ {
		if cur.Type != registry.SZ && cur.Type != registry.EXPAND_SZ {
			return false
		}
		return utf16BytesToString(cur.Data) == utf16BytesToString(want.Data)
	}
	if want.Type == registry.DWORD && cur.Type == registry.DWORD && len(cur.Data) >= 4 {
		return binary.LittleEndian.Uint32(cur.Data) == binary.LittleEndian.Uint32(want.Data)
	}
	if want.Type == registry.QWORD && cur.Type == registry.QWORD && len(cur.Data) >= 8 {
		return binary.LittleEndian.Uint64(cur.Data) == binary.LittleEndian.Uint64(want.Data)
	}
	return false
}

func regGetString(root registry.Key, path, name string) string {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return ""
	}
	defer k.Close()
	s, _, err := k.GetStringValue(name)
	if err != nil {
		return ""
	}
	return s
}

func regGetDword(root registry.Key, path, name string) (uint32, bool) {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return 0, false
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue(name)
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}

func regSubkeys(root registry.Key, path string) []string {
	k, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return nil
	}
	defer k.Close()
	names, _ := k.ReadSubKeyNames(-1)
	return names
}

func regValueNames(root registry.Key, path string) []string {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return nil
	}
	defer k.Close()
	names, _ := k.ReadValueNames(-1)
	return names
}

// ───────────────────────── misc ─────────────────────────

func isElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

func expandEnv(s string) string {
	return os.ExpandEnv(strings.NewReplacer(
		"%LOCALAPPDATA%", "${LOCALAPPDATA}", "%APPDATA%", "${APPDATA}", "%TEMP%", "${TEMP}",
		"%SystemRoot%", "${SystemRoot}", "%ProgramData%", "${ProgramData}", "%SystemDrive%", "${SystemDrive}",
		"%USERPROFILE%", "${USERPROFILE}", "%WINDIR%", "${windir}",
	).Replace(s))
}

func shellOpen(target string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	t, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, t, nil, nil, windows.SW_SHOWNORMAL)
}
