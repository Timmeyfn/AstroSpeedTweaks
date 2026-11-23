//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type customHandler struct {
	apply  func(a Action, env Env) (Backup, int, error) // int = number of changes made
	revert func(b Backup, env Env) error
	check  func(a Action, env Env) int
}

var customHandlers map[string]customHandler

func init() {
	customHandlers = map[string]customHandler{
		"powerplan":       {applyPowerPlan, revertPowerPlan, checkPowerPlan},
		"coreparking":     {applyPowerValues(coreParkSettings), revertPowerValues, checkPowerValues(coreParkSettings)},
		"idledisable":     {applyPowerValues(idleSettings), revertPowerValues, checkPowerValues(idleSettings)},
		"nicpower":        {applyNicPower, revertNic, nil},
		"nicadv":          {applyNicAdv, revertNic, nil},
		"usbpower":        {applyUSBPower, revertUSBPower, nil},
		"mmagent":         {applyMMAgent, revertMMAgent, nil},
		"discord_hwaccel": {applyDiscordHW, revertDiscordHW, checkDiscordHW},
	}
}

// ───────────────────────── power plans ─────────────────────────

const (
	sgPlan        = "5747a0a7-91d3-4c6e-b1f0-8a7e9c2d3e41"
	planUltimate  = "e9a42b02-d5df-448d-aa00-03f14749eb61"
	planHigh      = "8c5e7fda-e8bf-4a96-9a85-cf2fa2c4ff2a"
	planBalanced  = "381b4222-f694-41f0-9685-ff5bb260df2e"
	subProcessor  = "54533251-82be-4824-96c1-47b60b740d00"
	subPCIe       = "501a4d13-42af-4429-9fd1-a8218c268e20"
	subUSB        = "2a737441-1930-4402-8d77-b2bebba308a3"
	subDisk       = "0012ee47-9041-4b5d-9b77-535fba8b1442"
	subWireless   = "19cbb8fa-5279-450e-9fac-8a3d5fedd0c1"
	subVideo      = "7516b95f-f776-4464-8c53-06167f40cc99"
	subSleep      = "238c9fa8-0aad-41ed-83f4-97be242c8f20"
	setVideoIdle  = "3c0bc021-c8a8-4e07-a973-6b14cbcb2b7e"
	setSleepIdle  = "29f6c1db-86da-48c5-9fdb-f2b67b1f44da"
	setHibernIdle = "9d7815a6-7ee4-497e-8888-515a05f02364"
)

type pset struct {
	Sub, Set string
	Val      int
}

var planSettings = []pset{
	{subProcessor, "893dee8e-2bef-41e0-89c6-b55d0929964c", 100}, // min processor state
	{subProcessor, "bc5038f7-23e0-4960-96da-33abaf5935ec", 100}, // max processor state
	{subProcessor, "0cc5b647-c1df-4637-891a-dec35c318583", 100}, // core parking min cores
	{subProcessor, "ea062031-0e34-4ff1-9b6d-eb1059334028", 100}, // core parking max cores
	{subProcessor, "be337238-0d82-4146-a960-4f3749d470c7", 2},   // boost mode aggressive
	{subProcessor, "36687f9e-e3a5-4dbf-b1dc-15eb381c6863", 0},   // energy perf preference = performance
	{subProcessor, "94d3a615-a899-4ac5-ae2b-e4d8f634367f", 1},   // active cooling
	{subPCIe, "ee12f906-d277-404b-b6da-e5fa1a576df5", 0},        // PCIe ASPM off
	{subUSB, "48e6b7a6-50f5-4782-a5d4-53bb8f07e226", 0},         // USB selective suspend off
	{subDisk, "6738e2c4-e8a5-4a42-b16a-e040e769756e", 0},        // disk never idles
	{subWireless, "12bbebe6-58d6-4636-95bb-3217ef867c1a", 0},    // Wi-Fi max performance
}

var coreParkSettings = []pset{
	{subProcessor, "0cc5b647-c1df-4637-891a-dec35c318583", 100},
	{subProcessor, "ea062031-0e34-4ff1-9b6d-eb1059334028", 100},
}
var idleSettings = []pset{
	{subProcessor, "5d76a2ca-e8c0-402f-a133-2158492d58ad", 1},
}

func powercfg(args ...string) (string, error) {
	return runExe(30*time.Second, filepath.Join(os.Getenv("SystemRoot"), `System32\powercfg.exe`), args...)
}

func activeScheme() string {
	if g := nativeActiveScheme(); g != "" {
		return g
	}
	out, err := powercfg("/getactivescheme")
	if err != nil {
		return ""
	}
	return strings.ToLower(guidRe.FindString(out))
}

func schemeExists(g string) bool {
	out, err := powercfg("/list")
	return err == nil && strings.Contains(strings.ToLower(out), strings.ToLower(g))
}

// queryPower returns the AC and DC values of a setting (the last two hex numbers in powercfg /query).
func queryPower(scheme, sub, set string) (int, int, bool) {
	if ac, ok := nativeReadPower(scheme, sub, set, true); ok {
		if dc, ok2 := nativeReadPower(scheme, sub, set, false); ok2 {
			return ac, dc, true
		}
	}
	out, err := powercfg("/query", scheme, sub, set)
	if err != nil {
		return 0, 0, false
	}
	h := hexRe.FindAllString(out, -1)
	if len(h) < 2 {
		return 0, 0, false
	}
	ac, e1 := strconv.ParseInt(h[len(h)-2][2:], 16, 64)
	dc, e2 := strconv.ParseInt(h[len(h)-1][2:], 16, 64)
	if e1 != nil || e2 != nil {
		return 0, 0, false
	}
	return int(ac), int(dc), true
}

func applyPowerPlan(a Action, env Env) (Backup, int, error) {
	prev := activeScheme()
	if prev == "" {
		return Backup{}, 0, errors.New("could not read the active power plan")
	}
	if prev == sgPlan {
		return Backup{}, 0, nil
	}
	if !schemeExists(sgPlan) {
		created := false
		for _, base := range []string{planUltimate, planHigh, planBalanced} {
			if _, err := powercfg("/duplicatescheme", base, sgPlan); err == nil && schemeExists(sgPlan) {
				created = true
				logf("    created power plan from %s", base)
				break
			}
		}
		if !created {
			return Backup{}, 0, errors.New("Windows would not create a new power plan on this PC")
		}
	}
	_, _ = powercfg("/changename", sgPlan, "AstroSpeed Performance", "Maximum performance while plugged in. Created by AstroSpeed Tweaks; undo in the app to remove.")
	n := 0
	for _, s := range planSettings {
		if _, err := powercfg("/setacvalueindex", sgPlan, s.Sub, s.Set, strconv.Itoa(s.Val)); err == nil {
			n++
		}
		// battery behaviour copies your current plan
		if _, dc, ok := queryPower(prev, s.Sub, s.Set); ok {
			_, _ = powercfg("/setdcvalueindex", sgPlan, s.Sub, s.Set, strconv.Itoa(dc))
		}
	}
	// keep your screen-off and sleep timers exactly as they were
	for _, s := range [][2]string{{subVideo, setVideoIdle}, {subSleep, setSleepIdle}, {subSleep, setHibernIdle}} {
		if ac, dc, ok := queryPower(prev, s[0], s[1]); ok {
			_, _ = powercfg("/setacvalueindex", sgPlan, s[0], s[1], strconv.Itoa(ac))
			_, _ = powercfg("/setdcvalueindex", sgPlan, s[0], s[1], strconv.Itoa(dc))
		}
	}
	if _, err := powercfg("/setactive", sgPlan); err != nil {
		return Backup{}, 0, err
	}
	return Backup{ID: "custom|powerplan", Kind: "custom", Handler: "powerplan", Arg: prev}, n + 1, nil
}

func revertPowerPlan(b Backup, env Env) error {
	prev := b.Arg
	if prev == "" || !schemeExists(prev) {
		prev = planBalanced
	}
	if _, err := powercfg("/setactive", prev); err != nil {
		return err
	}
	_, _ = powercfg("/delete", sgPlan)
	return nil
}

func checkPowerPlan(a Action, env Env) int {
	if activeScheme() == sgPlan {
		return 1
	}
	return 0
}

type powerBackup struct {
	Scheme string `json:"scheme"`
	Sub    string `json:"sub"`
	Set    string `json:"set"`
	AC     int    `json:"ac"`
	Known  bool   `json:"known"`
}

func applyPowerValues(list []pset) func(Action, Env) (Backup, int, error) {
	return func(a Action, env Env) (Backup, int, error) {
		scheme := activeScheme()
		if scheme == "" {
			return Backup{}, 0, errors.New("could not read the active power plan")
		}
		var bk []powerBackup
		n := 0
		for _, s := range list {
			ac, _, ok := queryPower(scheme, s.Sub, s.Set)
			if ok && ac == s.Val {
				continue
			}
			if _, err := powercfg("/setacvalueindex", scheme, s.Sub, s.Set, strconv.Itoa(s.Val)); err != nil {
				return Backup{}, 0, err
			}
			bk = append(bk, powerBackup{scheme, s.Sub, s.Set, ac, ok})
			n++
		}
		if n == 0 {
			return Backup{}, 0, nil
		}
		_, _ = powercfg("/setactive", scheme)
		extra, _ := json.Marshal(bk)
		return Backup{ID: "custom|" + a.Handler + "|" + scheme, Kind: "custom", Handler: a.Handler, Extra: extra}, n, nil
	}
}

func revertPowerValues(b Backup, env Env) error {
	var bk []powerBackup
	_ = json.Unmarshal(b.Extra, &bk)
	scheme := ""
	for _, p := range bk {
		if !p.Known {
			continue
		}
		scheme = p.Scheme
		if _, err := powercfg("/setacvalueindex", p.Scheme, p.Sub, p.Set, strconv.Itoa(p.AC)); err != nil && schemeExists(p.Scheme) {
			return err
		}
	}
	if scheme != "" && scheme == activeScheme() {
		_, _ = powercfg("/setactive", scheme)
	}
	return nil
}

func checkPowerValues(list []pset) func(Action, Env) int {
	return func(a Action, env Env) int {
		scheme := activeScheme()
		if scheme == "" {
			return -1
		}
		for _, s := range list {
			ac, _, ok := queryPower(scheme, s.Sub, s.Set)
			if !ok {
				return -1
			}
			if ac != s.Val {
				return 0
			}
		}
		return 1
	}
}

// ───────────────────────── network adapters ─────────────────────────

func runNic(handler string, pairs [][2]string, pm bool) (Backup, int, error) {
	out, err := runPS(psNic(pairs, pm), 120*time.Second)
	if err != nil {
		return Backup{}, 0, err
	}
	if strings.Contains(out, "NOADAPTER") {
		return Backup{}, 0, errSkip
	}
	var bk nicBackup
	if i := strings.Index(out, "{"); i >= 0 {
		_ = json.Unmarshal([]byte(out[i:]), &bk)
	}
	n := len(bk.Props) + len(bk.PM)
	if n == 0 {
		return Backup{}, 0, nil
	}
	for _, p := range bk.Props {
		logf("    ✓ %s: %s %s → %s", p.N, p.K, p.V, pairVal(pairs, p.K))
	}
	extra, _ := json.Marshal(bk)
	keys := []string{}
	for _, p := range pairs {
		keys = append(keys, p[0])
	}
	return Backup{ID: "custom|" + handler + "|" + strings.ToLower(strings.Join(keys, ",")), Kind: "custom", Handler: handler, Extra: extra}, n, nil
}

func pairVal(pairs [][2]string, k string) string {
	for _, p := range pairs {
		if p[0] == k {
			return p[1]
		}
	}
	return "?"
}

func applyNicPower(a Action, env Env) (Backup, int, error) {
	var pairs [][2]string
	for _, k := range []string{"*EEE", "EEELinkAdvertisement", "AdvancedEEE", "EnableGreenEthernet", "GigaLite", "PowerSavingMode", "ULPMode", "*SelectiveSuspend", "AutoPowerSaveModeEnabled"} {
		pairs = append(pairs, [2]string{k, "0"})
	}
	return runNic("nicpower", pairs, true)
}

func applyNicAdv(a Action, env Env) (Backup, int, error) {
	var pairs [][2]string
	for _, kv := range strings.Split(a.Arg, ",") {
		if i := strings.Index(kv, "="); i > 0 {
			pairs = append(pairs, [2]string{kv[:i], kv[i+1:]})
		}
	}
	return runNic("nicadv", pairs, false)
}

func revertNic(b Backup, env Env) error {
	var bk nicBackup
	_ = json.Unmarshal(b.Extra, &bk)
	if len(bk.Props) == 0 && len(bk.PM) == 0 {
		return nil
	}
	_, err := runPS(psRevertNic(bk), 120*time.Second)
	return err
}

// ───────────────────────── USB / HID power ─────────────────────────

func applyUSBPower(a Action, env Env) (Backup, int, error) {
	out, err := runPS(psUSBApply(), 120*time.Second)
	if err != nil {
		return Backup{}, 0, err
	}
	var names []string
	parseJSONList(out, &names)
	if len(names) == 0 {
		return Backup{}, 0, nil
	}
	extra, _ := json.Marshal(names)
	return Backup{ID: "custom|usbpower", Kind: "custom", Handler: "usbpower", Extra: extra}, len(names), nil
}

func revertUSBPower(b Backup, env Env) error {
	var names []string
	_ = json.Unmarshal(b.Extra, &names)
	if len(names) == 0 {
		return nil
	}
	_, err := runPS(psUSBRevert(names), 120*time.Second)
	return err
}

// ───────────────────────── memory manager ─────────────────────────

func applyMMAgent(a Action, env Env) (Backup, int, error) {
	feat := a.Arg
	if feat != "MemoryCompression" && feat != "PageCombining" {
		return Backup{}, 0, errors.New("unsupported memory feature")
	}
	out, err := runPS(psMMApply(feat), 60*time.Second)
	if err != nil {
		return Backup{}, 0, err
	}
	if strings.Contains(out, "WAS_OFF") {
		return Backup{}, 0, nil
	}
	return Backup{ID: "custom|mmagent|" + strings.ToLower(feat), Kind: "custom", Handler: "mmagent", Arg: feat}, 1, nil
}

func revertMMAgent(b Backup, env Env) error {
	if b.Arg != "MemoryCompression" && b.Arg != "PageCombining" {
		return nil
	}
	_, err := runPS(psMMRevert(b.Arg), 60*time.Second)
	return err
}

// ───────────────────────── Discord ─────────────────────────

func discordSettingsPath() string { return expandEnv(`%APPDATA%\discord\settings.json`) }

type discordBackup struct {
	Had  bool            `json:"had"`
	Prev json.RawMessage `json:"prev,omitempty"`
}

func applyDiscordHW(a Action, env Env) (Backup, int, error) {
	if processRunning("discord.exe") {
		return Backup{}, 0, errors.New("Discord is running. Quit it from the tray icon (right-click → Quit Discord), then apply again")
	}
	p := discordSettingsPath()
	b, err := os.ReadFile(p)
	if err != nil {
		return Backup{}, 0, errSkip
	}
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &m); err != nil {
		return Backup{}, 0, fmt.Errorf("couldn't read Discord's settings file: %v", err)
	}
	prev, had := m["enableHardwareAcceleration"]
	if had && strings.TrimSpace(string(prev)) == "false" {
		return Backup{}, 0, nil
	}
	_ = os.WriteFile(p+".astrospeed.bak", b, 0o644)
	m["enableHardwareAcceleration"] = json.RawMessage("false")
	nb, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(p, nb, 0o644); err != nil {
		return Backup{}, 0, err
	}
	extra, _ := json.Marshal(discordBackup{Had: had, Prev: prev})
	return Backup{ID: "custom|discord_hwaccel", Kind: "custom", Handler: "discord_hwaccel", Extra: extra}, 1, nil
}

func revertDiscordHW(b Backup, env Env) error {
	if processRunning("discord.exe") {
		return errors.New("Discord is running. Quit it from the tray icon, then undo again")
	}
	var bk discordBackup
	_ = json.Unmarshal(b.Extra, &bk)
	p := discordSettingsPath()
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	m := map[string]json.RawMessage{}
	if json.Unmarshal(raw, &m) != nil {
		return errors.New("Discord's settings file couldn't be read")
	}
	if bk.Had && len(bk.Prev) > 0 {
		m["enableHardwareAcceleration"] = bk.Prev
	} else {
		delete(m, "enableHardwareAcceleration")
	}
	nb, _ := json.MarshalIndent(m, "", "  ")
	return os.WriteFile(p, nb, 0o644)
}

func checkDiscordHW(a Action, env Env) int {
	b, err := os.ReadFile(discordSettingsPath())
	if err != nil {
		return -2
	}
	m := map[string]json.RawMessage{}
	if json.Unmarshal(b, &m) != nil {
		return -1
	}
	if v, ok := m["enableHardwareAcceleration"]; ok && strings.TrimSpace(string(v)) == "false" {
		return 1
	}
	return 0
}
