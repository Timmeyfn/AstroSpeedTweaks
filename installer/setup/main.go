//go:build windows

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

//go:embed app_payload.bin
var appExe []byte

//go:embed ui.html
var uiHTML string

var (
	user32         = windows.NewLazySystemDLL("user32.dll")
	procMessageBox = user32.NewProc("MessageBoxW")
	shell32        = windows.NewLazySystemDLL("shell32.dll")
	procShellExec  = shell32.NewProc("ShellExecuteW")
	procSetAppID   = shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
	wv             webview2.WebView
)

func messageBox(text, title string, flags uint32) int {
	t, _ := windows.UTF16PtrFromString(text)
	c, _ := windows.UTF16PtrFromString(title)
	r, _, _ := procMessageBox.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), uintptr(flags))
	return int(r)
}

func shellOpen(path string) {
	v, _ := windows.UTF16PtrFromString("open")
	p, _ := windows.UTF16PtrFromString(path)
	procShellExec.Call(0, uintptr(unsafe.Pointer(v)), uintptr(unsafe.Pointer(p)), 0, 0, 1 /*SW_SHOWNORMAL*/)
}

func main() {
	for _, a := range os.Args[1:] {
		if strings.EqualFold(a, "--uninstall") {
			if messageBox("Remove "+appName+" from this PC?\n\nYour saved backups in ProgramData are kept, so you can still undo tweaks if you reinstall.", "Uninstall "+appName, 0x24 /*yesno+question*/) != 6 {
				return
			}
			doUninstall()
			messageBox(appName+" has been removed.", appName, 0x40)
			return
		}
	}

	appID, _ := windows.UTF16PtrFromString("AstroSpeed.Setup")
	procSetAppID.Call(uintptr(unsafe.Pointer(appID)))

	wv = webview2.NewWithOptions(webview2.WebViewOptions{
		Debug: false,
		WindowOptions: webview2.WindowOptions{
			Title:  appName + " Setup",
			Width:  560,
			Height: 470,
			Center: true,
			IconId: 1,
		},
	})
	if wv == nil {
		// No WebView2 runtime: install without the wizard so it still works.
		basicInstall()
		return
	}
	defer wv.Destroy()
	wv.SetSize(560, 470, webview2.HintFixed)

	wv.Bind("setupInvoke", func(method string, argJSON string) interface{} {
		switch method {
		case "install":
			var a struct {
				Desktop bool `json:"desktop"`
			}
			_ = json.Unmarshal([]byte(argJSON), &a)
			target, err := doInstall(a.Desktop)
			if err != nil {
				return map[string]interface{}{"ok": false, "error": err.Error()}
			}
			installedTarget = target
			return map[string]interface{}{"ok": true, "dir": installDir()}
		case "finish":
			var a struct {
				Launch bool `json:"launch"`
			}
			_ = json.Unmarshal([]byte(argJSON), &a)
			if a.Launch && installedTarget != "" {
				shellOpen(installedTarget)
			}
			wv.Terminate()
		case "cancel":
			wv.Terminate()
		}
		return map[string]interface{}{"ok": true}
	})

	html := strings.ReplaceAll(uiHTML, "{{VERSION}}", appVersion)
	wv.SetHtml(html)
	wv.Run()
}

var installedTarget string

// basicInstall is the no-WebView2 fallback.
func basicInstall() {
	if messageBox("Install "+appName+" "+appVersion+" ?\n\nIt will be added to your Start Menu.", appName+" Setup", 0x24) != 6 {
		return
	}
	target, err := doInstall(true)
	if err != nil {
		messageBox("Install failed: "+err.Error(), appName, 0x10)
		return
	}
	if messageBox(appName+" is installed.\n\nOpen it now?", appName, 0x24) == 6 {
		shellOpen(target)
	}
}

// keep fmt/syscall referenced for future use
var _ = fmt.Sprintf
var _ = syscall.Syscall
