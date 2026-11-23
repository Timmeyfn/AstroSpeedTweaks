//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// errSkip marks an action that doesn't apply to this PC (missing service, no adapter…).
var errSkip = errors.New("not present on this PC")

type TweakResult struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Status  string `json:"status"` // ok, already, partial, failed, skipped, reverted, nothing
	Msg     string `json:"msg,omitempty"`
	Changes int    `json:"changes"`
	Reboot  bool   `json:"reboot"`
}

type Engine struct {
	mu                 sync.Mutex
	bcdCache           map[string]string
	bcdLoaded          time.Time
	bitlockerSuspended bool
	rebootNeeded       bool
}

var engine = &Engine{}

// ───────────────────────── action expansion ─────────────────────────

// expand turns dynamic actions into concrete registry actions for this PC.
func expand(a Action, env Env) ([]Action, error) {
	if a.Kind != "dyn" {
		return []Action{a}, nil
	}
	switch a.Handler {
	case "nagle":
		base := `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces`
		var out []Action
		for _, g := range regSubkeys(registry.LOCAL_MACHINE, base) {
			p := base + `\` + g
			if !interfaceHasIP(p) {
				continue
			}
			out = append(out, RegD(HKLM, p, "TcpAckFrequency", 1), RegD(HKLM, p, "TCPNoDelay", 1), RegD(HKLM, p, "TcpDelAckTicks", 0))
		}
		if len(out) == 0 {
			return nil, errSkip
		}
		return out, nil

	case "fn_gpupref", "gpupref":
		exe := gameExe(env, a.Arg)
		if exe == "" {
			return nil, errSkip
		}
		p := `Software\Microsoft\DirectX\UserGpuPreferences`
		cur := regGetString(registry.CURRENT_USER, p, exe)
		return []Action{RegS(HKCU, p, exe, setKV(cur, "GpuPreference", "2"))}, nil

	case "swapeffect":
		p := `Software\Microsoft\DirectX\UserGpuPreferences`
		cur := regGetString(registry.CURRENT_USER, p, "DirectXUserGlobalSettings")
		return []Action{RegS(HKCU, p, "DirectXUserGlobalSettings", setKV(cur, "SwapEffectUpgradeEnable", "1"))}, nil

	case "fn_fso", "fso":
		exe := gameExe(env, a.Arg)
		if exe == "" {
			return nil, errSkip
		}
		p := `Software\Microsoft\Windows NT\CurrentVersion\AppCompatFlags\Layers`
		cur := strings.TrimSpace(regGetString(registry.CURRENT_USER, p, exe))
		flag := "DISABLEDXMAXIMIZEDWINDOWEDMODE"
		val := cur
		switch {
		case strings.Contains(strings.ToUpper(cur), flag):
		case cur == "":
			val = "~ " + flag
		default:
			val = cur + " " + flag
		}
		return []Action{RegS(HKCU, p, exe, val)}, nil

	case "accesskeys":
		defs := []struct {
			path string
			def  int
		}{
			{`Control Panel\Accessibility\StickyKeys`, 510},
			{`Control Panel\Accessibility\ToggleKeys`, 62},
			{`Control Panel\Accessibility\Keyboard Response`, 126},
		}
		var out []Action
		for _, d := range defs {
			v, err := strconv.Atoi(strings.TrimSpace(regGetString(registry.CURRENT_USER, d.path, "Flags")))
			if err != nil {
				v = d.def
			}
			out = append(out, RegS(HKCU, d.path, "Flags", strconv.Itoa(v&^4)))
		}
		return out, nil

	case "svchostsplit":
		kb := memStatus().TotalPhys / 1024
		if kb == 0 || kb > 0xFFFFFFFF {
			return nil, errSkip
		}
		return []Action{RegD(HKLM, `SYSTEM\CurrentControlSet\Control`, "SvcHostSplitThresholdInKB", kb)}, nil

	case "gpu_msi":
		var out []Action
		base := `SYSTEM\CurrentControlSet\Enum\PCI`
		for _, dev := range regSubkeys(registry.LOCAL_MACHINE, base) {
			for _, inst := range regSubkeys(registry.LOCAL_MACHINE, base+`\`+dev) {
				p := base + `\` + dev + `\` + inst
				if strings.EqualFold(regGetString(registry.LOCAL_MACHINE, p, "ClassGUID"), "{4d36e968-e325-11ce-bfc1-08002be10318}") {
					out = append(out, RegD(HKLM, p+`\Device Parameters\Interrupt Management\MessageSignaledInterruptProperties`, "MSISupported", 1))
				}
			}
		}
		if len(out) == 0 {
			return nil, errSkip
		}
		return out, nil

	case "amd_ulps":
		var out []Action
		for _, sub := range regSubkeys(registry.LOCAL_MACHINE, displayClass) {
			if len(sub) != 4 || sub[0] < '0' || sub[0] > '9' {
				continue
			}
			p := displayClass + `\` + sub
			prov := strings.ToUpper(regGetString(registry.LOCAL_MACHINE, p, "ProviderName"))
			match := strings.ToUpper(regGetString(registry.LOCAL_MACHINE, p, "MatchingDeviceId"))
			if strings.Contains(prov, "ADVANCED MICRO DEVICES") || strings.Contains(prov, "ATI ") || strings.Contains(match, "VEN_1002") {
				out = append(out, RegD(HKLM, p, "EnableUlps", 0))
			}
		}
		if len(out) == 0 {
			return nil, errSkip
		}
		return out, nil

	case "qos_policy":
		parts := strings.Split(a.Arg, "|")
		if len(parts) != 3 {
			return nil, fmt.Errorf("bad QoS argument")
		}
		p := pQoSPol + `\` + parts[0]
		vals := [][2]string{
			{"Version", "1.0"}, {"Application Name", parts[1]}, {"Protocol", "*"},
			{"Local Port", "*"}, {"Local IP", "*"}, {"Local IP Prefix Length", "*"},
			{"Remote Port", "*"}, {"Remote IP", "*"}, {"Remote IP Prefix Length", "*"},
			{"DSCP Value", parts[2]}, {"Throttle Rate", "-1"},
		}
		var out []Action
		for _, v := range vals {
			out = append(out, RegS(HKLM, p, v[0], v[1]))
		}
		return out, nil
	}
	return nil, fmt.Errorf("unknown handler %s", a.Handler)
}

func interfaceHasIP(p string) bool {
	if ip := regGetString(registry.LOCAL_MACHINE, p, "DhcpIPAddress"); ip != "" && ip != "0.0.0.0" {
		return true
	}
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, p, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return false
	}
	defer k.Close()
	ips, _, err := k.GetStringsValue("IPAddress")
	if err != nil {
		return false
	}
	for _, ip := range ips {
		if ip != "" && ip != "0.0.0.0" {
			return true
		}
	}
	return false
}

// ───────────────────────── apply single actions ─────────────────────────

func applyReg(a Action) (Backup, bool, error) {
	root, err := rootKey(a.Root)
	if err != nil {
		return Backup{}, false, err
	}
	prev := readRaw(root, a.Path, a.Name)
	want := targetValue(a)
	if regEqual(prev, want) {
		return Backup{}, false, nil
	}
	created := missingKeys(root, a.Path)
	if err := writeRaw(root, a.Path, a.Name, want); err != nil {
		return Backup{}, false, err
	}
	return Backup{ID: "reg|" + a.Root + "|" + strings.ToLower(a.Path) + "|" + strings.ToLower(a.Name), Kind: "reg",
		Root: a.Root, Path: a.Path, Name: a.Name, Prev: prev, Created: created}, true, nil
}

func revertReg(b Backup) error {
	root, err := rootKey(b.Root)
	if err != nil {
		return err
	}
	if b.Prev.Exists {
		err = writeRaw(root, b.Path, b.Name, b.Prev)
	} else {
		err = deleteValue(root, b.Path, b.Name)
	}
	for i := len(b.Created) - 1; i >= 0; i-- {
		deleteKeyIfEmpty(root, b.Created[i])
	}
	return err
}

func applyRegDel(a Action) (Backup, bool, error) {
	root, err := rootKey(a.Root)
	if err != nil {
		return Backup{}, false, err
	}
	prev := readRaw(root, a.Path, a.Name)
	if !prev.Exists {
		return Backup{}, false, nil
	}
	if err := deleteValue(root, a.Path, a.Name); err != nil {
		return Backup{}, false, err
	}
	return Backup{ID: "reg|" + a.Root + "|" + strings.ToLower(a.Path) + "|" + strings.ToLower(a.Name), Kind: "reg",
		Root: a.Root, Path: a.Path, Name: a.Name, Prev: prev}, true, nil
}

// ───────────────────────── services ─────────────────────────

func startTypeOf(s string) uint32 {
	switch s {
	case "disabled":
		return windows.SERVICE_DISABLED
	case "auto":
		return windows.SERVICE_AUTO_START
	}
	return windows.SERVICE_DEMAND_START
}

type delayedInfo struct{ IsDelayed uint32 }

func openService(name string, access uint32) (windows.Handle, windows.Handle, error) {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return 0, 0, err
	}
	n, _ := windows.UTF16PtrFromString(name)
	h, err := windows.OpenService(scm, n, access)
	if err != nil {
		windows.CloseServiceHandle(scm)
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return 0, 0, errSkip
		}
		return 0, 0, err
	}
	return scm, h, nil
}

func serviceStart(name string) (start uint32, delayed bool, running bool, err error) {
	m, err := mgr.Connect()
	if err != nil {
		return 0, false, false, err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return 0, false, false, errSkip
		}
		return 0, false, false, err
	}
	defer s.Close()
	cfg, err := s.Config()
	if err != nil {
		return 0, false, false, err
	}
	st, err := s.Query()
	if err == nil {
		running = st.State == svc.Running
	}
	return cfg.StartType, cfg.DelayedAutoStart, running, nil
}

func setServiceStart(name string, start uint32, delayed bool) error {
	scm, h, err := openService(name, windows.SERVICE_CHANGE_CONFIG|windows.SERVICE_QUERY_CONFIG)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(scm)
	defer windows.CloseServiceHandle(h)
	if err := windows.ChangeServiceConfig(h, windows.SERVICE_NO_CHANGE, start, windows.SERVICE_NO_CHANGE, nil, nil, nil, nil, nil, nil, nil); err != nil {
		return err
	}
	if start == windows.SERVICE_AUTO_START {
		info := delayedInfo{}
		if delayed {
			info.IsDelayed = 1
		}
		_ = windows.ChangeServiceConfig2(h, windows.SERVICE_CONFIG_DELAYED_AUTO_START_INFO, (*byte)(unsafe.Pointer(&info)))
	}
	return nil
}

func controlService(name string, stop bool) {
	m, err := mgr.Connect()
	if err != nil {
		return
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return
	}
	defer s.Close()
	if stop {
		_, _ = s.Control(svc.Stop)
	} else {
		_ = s.Start()
	}
}

func applySvc(a Action) (Backup, bool, error) {
	start, delayed, running, err := serviceStart(a.Svc)
	if err != nil {
		return Backup{}, false, err
	}
	want := startTypeOf(a.Start)
	if start == want && !(want == windows.SERVICE_AUTO_START && delayed) {
		return Backup{}, false, nil
	}
	if err := setServiceStart(a.Svc, want, false); err != nil {
		return Backup{}, false, err
	}
	if want == windows.SERVICE_DISABLED && running {
		controlService(a.Svc, true)
	}
	return Backup{ID: "svc|" + strings.ToLower(a.Svc), Kind: "svc", Svc: a.Svc, PrevStart: start, PrevDelay: delayed, WasRunning: running}, true, nil
}

func revertSvc(b Backup) error {
	if err := setServiceStart(b.Svc, b.PrevStart, b.PrevDelay); err != nil {
		if errors.Is(err, errSkip) {
			return nil
		}
		return err
	}
	if b.WasRunning && b.PrevStart != windows.SERVICE_DISABLED {
		controlService(b.Svc, false)
	}
	return nil
}

// ───────────────────────── scheduled tasks ─────────────────────────

func applyTasks(paths []string) (Backup, int, error) {
	script := psApplyTasks(paths)
	out, err := runPS(script, 90*time.Second)
	if err != nil {
		return Backup{}, 0, err
	}
	var recs []taskRec
	parseJSONList(out, &recs)
	changed, failed := 0, 0
	var keep []taskRec
	for _, r := range recs {
		if r.S == "Disabled" {
			continue
		}
		if r.E != "" {
			failed++
			logf("    task %s%s: %s", r.P, r.N, r.E)
			continue
		}
		changed++
		keep = append(keep, r)
	}
	if len(recs) == 0 {
		return Backup{}, 0, errSkip
	}
	extra, _ := json.Marshal(keep)
	b := Backup{ID: "task|" + strings.ToLower(strings.Join(paths, ";")), Kind: "task", Extra: extra}
	if failed > 0 && changed == 0 {
		return b, 0, fmt.Errorf("%d task(s) could not be disabled (protected by Windows)", failed)
	}
	return b, changed, nil
}

func revertTasks(b Backup) error {
	var recs []taskRec
	_ = json.Unmarshal(b.Extra, &recs)
	if len(recs) == 0 {
		return nil
	}
	_, err := runPS(psRevertTasks(recs), 90*time.Second)
	return err
}

// ───────────────────────── boot configuration ─────────────────────────

func bcdeditPath() string { return filepath.Join(os.Getenv("SystemRoot"), `System32\bcdedit.exe`) }

func (e *Engine) loadBCD(force bool) map[string]string {
	if !force && e.bcdCache != nil && time.Since(e.bcdLoaded) < 5*time.Second {
		return e.bcdCache
	}
	m := map[string]string{}
	out, err := runExe(20*time.Second, bcdeditPath(), "/enum", "{current}")
	if err == nil {
		for _, ln := range strings.Split(out, "\n") {
			f := strings.Fields(ln)
			if len(f) >= 2 {
				m[strings.ToLower(f[0])] = strings.Join(f[1:], " ")
			}
		}
	}
	e.bcdCache, e.bcdLoaded = m, time.Now()
	return m
}

func (e *Engine) guardBitLocker(env Env) error {
	if e.bitlockerSuspended || env.BitLocker == 0 {
		return nil
	}
	_, err := runPS(psBitLocker(), 60*time.Second)
	if err != nil {
		if env.BitLocker == 1 {
			return errors.New("BitLocker is on and could not be paused, so this boot setting was skipped to avoid a recovery-key prompt")
		}
		return nil // BitLocker module missing → not in use
	}
	e.bitlockerSuspended = true
	logf("    BitLocker paused for one restart so boot settings can change safely")
	return nil
}

func (e *Engine) applyBCD(a Action, env Env) (Backup, bool, error) {
	m := e.loadBCD(true)
	cur, had := m[strings.ToLower(a.Key)]
	if bcdMatches(cur, had, a.Val) {
		return Backup{}, false, nil
	}
	if err := e.guardBitLocker(env); err != nil {
		return Backup{}, false, err
	}
	var err error
	if a.Val == "" {
		_, err = runExe(20*time.Second, bcdeditPath(), "/deletevalue", "{current}", a.Key)
	} else {
		_, err = runExe(20*time.Second, bcdeditPath(), "/set", "{current}", a.Key, a.Val)
	}
	if err != nil {
		return Backup{}, false, err
	}
	e.bcdCache = nil
	return Backup{ID: "bcd|" + strings.ToLower(a.Key), Kind: "bcd", BcdKey: a.Key, BcdHad: had, BcdPrev: cur}, true, nil
}

func (e *Engine) revertBCD(b Backup, env Env) error {
	if err := e.guardBitLocker(env); err != nil {
		return err
	}
	e.bcdCache = nil
	if !b.BcdHad {
		_, err := runExe(20*time.Second, bcdeditPath(), "/deletevalue", "{current}", b.BcdKey)
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil
		}
		return err
	}
	prev := strings.ToLower(strings.TrimSpace(b.BcdPrev))
	if bcdNo[prev] {
		prev = "no"
	}
	if _, err := runExe(20*time.Second, bcdeditPath(), "/set", "{current}", b.BcdKey, prev); err != nil {
		// unknown/localized value: fall back to the Windows default
		_, err2 := runExe(20*time.Second, bcdeditPath(), "/deletevalue", "{current}", b.BcdKey)
		return err2
	}
	return nil
}

// ───────────────────────── commands ─────────────────────────

func applyCommand(a Action) (Backup, bool, error) {
	var err error
	var out string
	if a.Kind == "ps" {
		out, err = runPS(a.Apply, 120*time.Second)
	} else {
		out, err = runCmd(a.Apply, 120*time.Second)
	}
	if err != nil {
		return Backup{}, false, err
	}
	if out != "" {
		logf("    %s", firstLine(out))
	}
	b := Backup{ID: a.Kind + "|" + strings.ToLower(a.Apply), Kind: a.Kind}
	if a.Kind == "ps" {
		b.RevertPS = a.Revert
	} else {
		b.RevertCmd = a.Revert
	}
	return b, true, nil
}

// ───────────────────────── status ─────────────────────────

// liveCheck: 1 = matches target, 0 = doesn't, -1 = can't tell (rely on saved state), -2 = not present on PC.
func (e *Engine) liveCheck(a Action, env Env) int {
	switch a.Kind {
	case "reg":
		root, err := rootKey(a.Root)
		if err != nil {
			return -1
		}
		if regEqual(readRaw(root, a.Path, a.Name), targetValue(a)) {
			return 1
		}
		return 0
	case "regdel":
		root, _ := rootKey(a.Root)
		if readRaw(root, a.Path, a.Name).Exists {
			return 0
		}
		return 1
	case "svc":
		start, delayed, _, err := serviceStart(a.Svc)
		if errors.Is(err, errSkip) {
			return -2
		}
		if err != nil {
			return -1
		}
		want := startTypeOf(a.Start)
		if start == want && !(want == windows.SERVICE_AUTO_START && delayed) {
			return 1
		}
		return 0
	case "bcd":
		// Only use boot settings already read; reading them launches bcdedit, which the app
		// avoids doing at startup.
		m := e.bcdCache
		if len(m) == 0 {
			return -1
		}
		cur, had := m[strings.ToLower(a.Key)]
		if bcdMatches(cur, had, a.Val) {
			return 1
		}
		return 0
	case "dyn":
		acts, err := expand(a, env)
		if errors.Is(err, errSkip) {
			return -2
		}
		if err != nil || len(acts) == 0 {
			return -1
		}
		for _, c := range acts {
			if r := e.liveCheck(c, env); r == 0 {
				return 0
			}
		}
		return 1
	case "custom":
		if h, ok := customHandlers[a.Handler]; ok && h.check != nil {
			return h.check(a, env)
		}
	}
	return -1
}

// Status returns on, native, off, reset, na (with a reason).
func (e *Engine) Status(t Tweak, env Env, st *TweakState) (string, string) {
	if ok, why := available(t, env); !ok {
		return "na", why
	}
	applied := st != nil && st.Applied
	anyKnown, allTrue, unknown, present := false, true, false, false
	for _, a := range t.Actions {
		r := e.liveCheck(a, env)
		switch r {
		case 1:
			anyKnown, present = true, true
		case 0:
			anyKnown, present = true, true
			allTrue = false
		case -1:
			unknown, present = true, true
		case -2:
		}
	}
	if !present {
		return "na", "Not present on this PC"
	}
	if !anyKnown {
		if applied {
			return "on", ""
		}
		return "off", ""
	}
	if !allTrue {
		if applied {
			return "reset", "Something changed this back since it was applied"
		}
		return "off", ""
	}
	if applied {
		return "on", ""
	}
	if unknown {
		return "off", ""
	}
	return "native", "Already set on this PC"
}

// ───────────────────────── apply / revert a tweak ─────────────────────────

func backupIndex(bs []Backup) map[string]bool {
	m := map[string]bool{}
	for _, b := range bs {
		m[b.ID] = true
	}
	return m
}

func (e *Engine) ApplyTweak(t Tweak, env Env) TweakResult {
	res := TweakResult{ID: t.ID, Title: t.Title, Reboot: t.Reboot}
	if ok, why := available(t, env); !ok {
		res.Status, res.Msg = "skipped", why
		return res
	}
	stateMu.Lock()
	st := appState.Tweaks[t.ID]
	stateMu.Unlock()
	if st == nil {
		st = &TweakState{}
	}
	have := backupIndex(st.Backups)
	var newB []Backup
	var errs []string
	skips, total := 0, 0

	var taskPaths []string
	for _, a := range t.Actions {
		if a.Kind == "task" {
			taskPaths = append(taskPaths, a.Task)
			continue
		}
		acts, err := expand(a, env)
		if err != nil {
			if errors.Is(err, errSkip) {
				skips++
			} else {
				errs = append(errs, err.Error())
			}
			total++
			continue
		}
		for _, c := range acts {
			total++
			var b Backup
			var changed bool
			var err error
			switch c.Kind {
			case "reg":
				b, changed, err = applyReg(c)
			case "regdel":
				b, changed, err = applyRegDel(c)
			case "svc":
				b, changed, err = applySvc(c)
			case "bcd":
				b, changed, err = e.applyBCD(c, env)
			case "cmd", "ps":
				b, changed, err = applyCommand(c)
			case "custom":
				h, ok := customHandlers[c.Handler]
				if !ok {
					err = fmt.Errorf("unknown handler %s", c.Handler)
					break
				}
				var n int
				b, n, err = h.apply(c, env)
				changed = n > 0
				if n > 1 {
					res.Changes += n - 1
				}
			default:
				err = fmt.Errorf("unknown action %s", c.Kind)
			}
			if err != nil {
				if errors.Is(err, errSkip) {
					skips++
					continue
				}
				errs = append(errs, err.Error())
				logf("    ✗ %s: %v", c.Describe(), err)
				continue
			}
			if changed {
				res.Changes++
				logf("    ✓ %s", c.Describe())
				if !have[b.ID] {
					newB = append(newB, b)
					have[b.ID] = true
				}
			}
		}
	}
	if len(taskPaths) > 0 {
		total += len(taskPaths)
		b, n, err := applyTasks(taskPaths)
		if err != nil && !errors.Is(err, errSkip) {
			errs = append(errs, err.Error())
		} else if errors.Is(err, errSkip) {
			skips += len(taskPaths)
		}
		if n > 0 {
			res.Changes += n
			logf("    ✓ disabled %d scheduled task(s)", n)
			if !have[b.ID] {
				newB = append(newB, b)
			} else {
				// merge task records into the existing backup
				for i := range st.Backups {
					if st.Backups[i].ID == b.ID {
						var a1, a2 []taskRec
						_ = json.Unmarshal(st.Backups[i].Extra, &a1)
						_ = json.Unmarshal(b.Extra, &a2)
						st.Backups[i].Extra, _ = json.Marshal(append(a1, a2...))
					}
				}
			}
		}
	}

	stateMu.Lock()
	defer stateMu.Unlock()
	switch {
	case len(errs) > 0 && res.Changes == 0:
		res.Status, res.Msg = "failed", firstLine(strings.Join(errs, "; "))
	case skips == total && total > 0:
		res.Status, res.Msg = "skipped", "Not present on this PC"
	case res.Changes == 0 && len(newB) == 0 && !st.Applied:
		res.Status, res.Msg = "already", "Already set on this PC"
	default:
		res.Status = "ok"
		if len(errs) > 0 {
			res.Status, res.Msg = "partial", firstLine(strings.Join(errs, "; "))
		}
		appState.Seq++
		st.Applied = true
		st.AppliedAt = time.Now()
		st.Seq = appState.Seq
		st.Backups = append(st.Backups, newB...)
		appState.Tweaks[t.ID] = st
		appState.TotalChanges += res.Changes
		if t.Reboot && res.Changes > 0 {
			e.rebootNeeded = true
		}
	}
	if res.Status == "failed" {
		// persist any backups we did take so nothing is lost
		if len(newB) > 0 {
			st.Backups = append(st.Backups, newB...)
			st.Applied = true
			appState.Tweaks[t.ID] = st
		}
	}
	saveStateLocked()
	return res
}

func (e *Engine) RevertTweak(t Tweak, env Env) TweakResult {
	res := TweakResult{ID: t.ID, Title: t.Title, Reboot: t.Reboot}
	stateMu.Lock()
	st := appState.Tweaks[t.ID]
	stateMu.Unlock()
	if st == nil || (!st.Applied && len(st.Backups) == 0) {
		res.Status, res.Msg = "nothing", "Nothing to undo"
		return res
	}
	var failed []Backup
	var errs []string
	for i := len(st.Backups) - 1; i >= 0; i-- {
		b := st.Backups[i]
		// A registry value shared with another applied tweak (e.g. "Do not use NLA") is handed
		// over to that tweak instead of being restored now, so the other tweak keeps working.
		if b.Kind == "reg" && handOverBackup(t.ID, b) {
			res.Changes++
			continue
		}
		var err error
		switch b.Kind {
		case "reg":
			err = revertReg(b)
		case "svc":
			err = revertSvc(b)
		case "task":
			err = revertTasks(b)
		case "bcd":
			err = e.revertBCD(b, env)
		case "cmd":
			if b.RevertCmd != "" {
				_, err = runCmd(b.RevertCmd, 120*time.Second)
			}
		case "ps":
			if b.RevertPS != "" {
				_, err = runPS(b.RevertPS, 120*time.Second)
			}
		case "custom":
			if h, ok := customHandlers[b.Handler]; ok {
				err = h.revert(b, env)
			}
		}
		if err != nil {
			failed = append([]Backup{b}, failed...)
			errs = append(errs, err.Error())
			logf("    ✗ undo %s: %v", b.ID, err)
		} else {
			res.Changes++
		}
	}
	stateMu.Lock()
	defer stateMu.Unlock()
	if len(failed) > 0 {
		st.Backups = failed
		res.Status, res.Msg = "partial", firstLine(strings.Join(errs, "; "))
	} else {
		st.Backups = nil
		st.Applied = false
		res.Status = "reverted"
	}
	appState.Tweaks[t.ID] = st
	if t.Reboot && res.Changes > 0 {
		e.rebootNeeded = true
	}
	saveStateLocked()
	return res
}

// handOverBackup gives a registry backup to another applied tweak that sets the same value.
func handOverBackup(from string, b Backup) bool {
	stateMu.Lock()
	defer stateMu.Unlock()
	for _, other := range Catalog {
		if other.ID == from {
			continue
		}
		ost := appState.Tweaks[other.ID]
		if ost == nil || !ost.Applied {
			continue
		}
		for _, a := range other.Actions {
			if a.Kind == "reg" && strings.EqualFold(a.Root, b.Root) && strings.EqualFold(a.Path, b.Path) && strings.EqualFold(a.Name, b.Name) {
				if !backupIndex(ost.Backups)[b.ID] {
					ost.Backups = append(ost.Backups, b)
				}
				return true
			}
		}
	}
	return false
}

var guidRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
var hexRe = regexp.MustCompile(`0x[0-9a-fA-F]+`)
