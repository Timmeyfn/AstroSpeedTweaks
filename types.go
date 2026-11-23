package main

import (
	"fmt"
	"strings"
)

// Action is one concrete change a tweak makes. Every action kind knows how to
// back up what was there before, so a tweak can always be reverted exactly.
type Action struct {
	Kind string `json:"kind"` // reg, regdel, svc, task, cmd, ps, bcd, dyn, custom

	// Registry
	Root string `json:"root,omitempty"` // HKLM or HKCU
	Path string `json:"path,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"` // DWORD, QWORD, SZ
	Num  uint64 `json:"num"`
	Str  string `json:"str,omitempty"`

	// Service start type: disabled, manual, auto
	Svc   string `json:"svc,omitempty"`
	Start string `json:"start,omitempty"`

	// Scheduled task path, e.g. \Microsoft\Windows\Autochk\Proxy (wildcards allowed in the name)
	Task string `json:"task,omitempty"`

	// Commands (cmd = cmd.exe, ps = PowerShell). Revert runs on undo.
	Apply  string `json:"apply,omitempty"`
	Revert string `json:"revert,omitempty"`

	// Boot configuration (bcdedit). Empty Val means "delete this value".
	Key string `json:"key,omitempty"`
	Val string `json:"val,omitempty"`

	// Dynamic / custom handlers resolved at run time on the PC.
	Handler string `json:"handler,omitempty"`
	Arg     string `json:"arg,omitempty"`
}

// Tweak is one switch in the UI.
type Tweak struct {
	ID      string   `json:"id"`
	Cat     string   `json:"cat"`
	Group   string   `json:"group"`
	Title   string   `json:"title"`
	Desc    string   `json:"desc"`
	Benefit string   `json:"benefit"`
	Note    string   `json:"note,omitempty"`
	Impact  int      `json:"impact"`  // 1 low, 2 medium, 3 high
	Risk    string   `json:"risk"`    // safe, moderate, expert
	Profile int      `json:"profile"` // 1 = Recommended, 2 = Competitive, 0 = manual only
	Reboot  bool     `json:"reboot"`
	When    []string `json:"when,omitempty"` // conditions: nvidia, amd, fortnite, discord, ram16, win11
	Actions []Action `json:"actions"`
	Online  bool     `json:"online,omitempty"` // added from the GitHub pack
}

type Category struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Icon     string `json:"icon"`
	Nav      string `json:"nav,omitempty"`  // short name for the side menu
	Game     string `json:"game,omitempty"` // set when this section belongs to a game
}

// ---- builders keep the catalog readable ----

func RegD(root, path, name string, v uint64) Action {
	return Action{Kind: "reg", Root: root, Path: path, Name: name, Type: "DWORD", Num: v}
}
func RegQ(root, path, name string, v uint64) Action {
	return Action{Kind: "reg", Root: root, Path: path, Name: name, Type: "QWORD", Num: v}
}
func RegS(root, path, name, v string) Action {
	return Action{Kind: "reg", Root: root, Path: path, Name: name, Type: "SZ", Str: v}
}
func RegDel(root, path, name string) Action {
	return Action{Kind: "regdel", Root: root, Path: path, Name: name}
}
func Svc(name, start string) Action { return Action{Kind: "svc", Svc: name, Start: start} }
func Task(path string) Action       { return Action{Kind: "task", Task: path} }
func Cmd(apply, revert string) Action {
	return Action{Kind: "cmd", Apply: apply, Revert: revert}
}
func PS(apply, revert string) Action { return Action{Kind: "ps", Apply: apply, Revert: revert} }
func BCD(key, val string) Action     { return Action{Kind: "bcd", Key: key, Val: val} }
func Dyn(handler, arg string) Action { return Action{Kind: "dyn", Handler: handler, Arg: arg} }
func Custom(handler, arg string) Action {
	return Action{Kind: "custom", Handler: handler, Arg: arg}
}

// Describe returns a human-readable line shown under "What it changes".
func (a Action) Describe() string {
	switch a.Kind {
	case "reg":
		v := ""
		switch a.Type {
		case "DWORD", "QWORD":
			if a.Num > 9 {
				v = fmt.Sprintf("%d (0x%X)", a.Num, a.Num)
			} else {
				v = fmt.Sprintf("%d", a.Num)
			}
		default:
			v = fmt.Sprintf("%q", a.Str)
		}
		return fmt.Sprintf("%s\\%s → %s = %s", a.Root, a.Path, a.Name, v)
	case "regdel":
		return fmt.Sprintf("%s\\%s → remove %s", a.Root, a.Path, a.Name)
	case "svc":
		return fmt.Sprintf("Service %s → start type %s", a.Svc, a.Start)
	case "task":
		return fmt.Sprintf("Scheduled task %s → disabled", a.Task)
	case "cmd", "ps":
		return a.Apply
	case "bcd":
		if a.Val == "" {
			return fmt.Sprintf("bcdedit /deletevalue %s", a.Key)
		}
		return fmt.Sprintf("bcdedit /set %s %s", a.Key, a.Val)
	case "dyn", "custom":
		if d, ok := handlerDescriptions[a.Handler]; ok {
			if a.Arg != "" {
				return strings.ReplaceAll(d, "{arg}", a.Arg)
			}
			return d
		}
		return a.Handler
	}
	return a.Kind
}

var handlerDescriptions = map[string]string{
	"nagle":           `Every active network interface → TcpAckFrequency = 1, TCPNoDelay = 1, TcpDelAckTicks = 0`,
	"fn_gpupref":      `HKCU\Software\Microsoft\DirectX\UserGpuPreferences → <Fortnite exe> = "GpuPreference=2;"`,
	"fn_fso":          `HKCU\...\AppCompatFlags\Layers → <Fortnite exe> += "DISABLEDXMAXIMIZEDWINDOWEDMODE"`,
	"swapeffect":      `HKCU\Software\Microsoft\DirectX\UserGpuPreferences → DirectXUserGlobalSettings += "SwapEffectUpgradeEnable=1;"`,
	"accesskeys":      `HKCU\Control Panel\Accessibility → StickyKeys / ToggleKeys / Keyboard Response: clear only the "shortcut" bit`,
	"svchostsplit":    `HKLM\SYSTEM\CurrentControlSet\Control → SvcHostSplitThresholdInKB = your installed RAM in KB`,
	"gpu_msi":         `Each graphics card → Device Parameters\Interrupt Management\MessageSignaledInterruptProperties → MSISupported = 1`,
	"powerplan":       `powercfg: create "AstroSpeed Performance" from Ultimate Performance, CPU min/max 100%, boost aggressive, EPP 0, PCIe ASPM off, USB suspend off, disk never sleeps, Wi-Fi max performance (plugged in only), then make it active`,
	"coreparking":     `powercfg (active plan, plugged in): Processor core parking min cores = 100%, max cores = 100%`,
	"idledisable":     `powercfg (active plan, plugged in): Processor idle disable = 1`,
	"nicpower":        `Every physical network adapter → turn off Energy-Efficient Ethernet, Green Ethernet, power saving mode, ultra-low-power mode, and "allow the computer to turn off this device"`,
	"nicadv":          `Every physical network adapter → {arg}`,
	"usbpower":        `Device Manager → every USB hub, USB controller and HID device: untick "Allow the computer to turn off this device to save power"`,
	"mmagent":         `Disable-MMAgent -{arg}`,
	"discord_hwaccel": `%APPDATA%\discord\settings.json → enableHardwareAcceleration = false`,
	"qos_policy":      `Local QoS policy → {arg}`,
	"gpupref":         `HKCU\Software\Microsoft\DirectX\UserGpuPreferences → <the game's exe> = "GpuPreference=2;"`,
	"fso":             `HKCU\...\AppCompatFlags\Layers → <the game's exe> += "DISABLEDXMAXIMIZEDWINDOWEDMODE"`,
	"amd_ulps":        `Each AMD display adapter → EnableUlps = 0`,
}
