package main

import (
	"strings"
	"testing"
)

func TestSetKV(t *testing.T) {
	cases := [][3]string{
		{"", "GpuPreference=2;", "GpuPreference"},
		{"GpuPreference=1;", "GpuPreference=2;", "GpuPreference"},
		{"SwapEffectUpgradeEnable=0;VRROptimizeEnable=1;", "SwapEffectUpgradeEnable=1;VRROptimizeEnable=1;", "SwapEffectUpgradeEnable"},
		{"VRROptimizeEnable=0;", "VRROptimizeEnable=0;SwapEffectUpgradeEnable=1;", "SwapEffectUpgradeEnable"},
	}
	for _, c := range cases {
		val := "2"
		if c[2] == "SwapEffectUpgradeEnable" {
			val = "1"
		}
		if got := setKV(c[0], c[2], val); got != c[1] {
			t.Errorf("setKV(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

func TestBCDMatches(t *testing.T) {
	if !bcdMatches("Yes", true, "yes") || !bcdMatches("Ja", true, "yes") || bcdMatches("No", true, "yes") {
		t.Error("yes handling")
	}
	if !bcdMatches("", false, "") || bcdMatches("Yes", true, "") {
		t.Error("delete handling")
	}
	if !bcdMatches("Enhanced", true, "enhanced") || bcdMatches("Legacy", true, "enhanced") || bcdMatches("", false, "enhanced") {
		t.Error("enum handling")
	}
}

func TestStickyKeysBitOnlyClearsShortcut(t *testing.T) {
	for _, v := range []int{510, 511, 62, 63, 126, 127} {
		n := v &^ 4
		if n&4 != 0 || (n|4) != (v|4) {
			t.Errorf("%d -> %d changed more than the shortcut bit", v, n)
		}
	}
}

func TestRedact(t *testing.T) {
	u := redactTweak(tweakUI{Tweak: Tweak{ID: "nagle", Title: "Turn off Nagle", Desc: "Sends small packets now", Benefit: "Lower ping", Note: "x", Actions: []Action{RegD(HKLM, `a`, "b", 1)}}, Changes: []string{"Set a"}})
	if u.Title == "Turn off Nagle" || len(u.Title) != len("Turn off Nagle") || u.Changes != nil || u.Actions != nil || u.Note != "" || !u.Redacted {
		t.Fatalf("not redacted: %+v", u)
	}
}

func TestPaidTool(t *testing.T) {
	cases := []struct {
		m, a string
		want bool
	}{
		{"cleanRun", `{"ids":["x"]}`, true},
		{"cleanScan", ``, false},
		{"setAutoBoost", `{"on":true}`, true},
		{"setAutoBoost", `{"on":false}`, false},
		{"setSetting", `{"key":"hotkey","on":true}`, true},
		{"setSetting", `{"key":"startup","on":true}`, false},
		{"revert", `{"ids":["x"]}`, false},
	}
	for _, c := range cases {
		if got := paidTool(c.m, []byte(c.a)); got != c.want {
			t.Errorf("%s %s = %v", c.m, c.a, got)
		}
	}
}

func TestDefaultBatParse(t *testing.T) {
	text := "@echo off\r\nrem name: Fortnite optimiser\r\n:: comment\r\n\r\nstart \"\" \"C:\\Tools\\x.exe\"\r\npause\r\n"
	if cmds := batCommands(text); len(cmds) != 1 || !strings.HasPrefix(cmds[0], "start") {
		t.Fatalf("commands: %q", cmds)
	}
	if len(batCommands("@echo off\nrem just a note\n")) != 0 {
		t.Fatal("comments only should have no commands")
	}
	old := defaultBatText
	defer func() { defaultBatText = old }()
	defaultBatText = "rem name: My tool\nrem about: Does a thing.\nreg add HKCU\\X /v Y /t REG_DWORD /d 1 /f\n"
	b := embeddedDefaultBat()
	if b.Name != "My tool" || b.About != "Does a thing." || b.Hash == "" {
		t.Fatalf("embedded: %+v", b)
	}
	if tw, ok := defaultBatTweak(b); !ok || tw.ID != defaultBatID || isPro(tw) {
		t.Fatalf("tweak: %+v %v", tw, ok)
	}
}

func TestDefaultBatDecision(t *testing.T) {
	cases := []struct {
		cur, ans         string
		no               bool
		ask, on, updated bool
	}{
		{"h1", "", false, true, false, false},   // never asked
		{"h1", "h1", false, false, true, false}, // allowed this version
		{"h1", "h1", true, false, false, false}, // said no
		{"h2", "h1", false, true, false, true},  // changed since they allowed it: ask again
		{"h2", "h1", true, false, false, false}, // changed, but they'd said no: stay off
	}
	for _, c := range cases {
		a, o, u := defaultBatDecision(c.cur, c.ans, c.no)
		if a != c.ask || o != c.on || u != c.updated {
			t.Errorf("%+v -> %v %v %v", c, a, o, u)
		}
	}
}

func TestFrameStats(t *testing.T) {
	var ms []float64
	for i := 0; i < 990; i++ {
		ms = append(ms, 5) // 200 FPS
	}
	for i := 0; i < 10; i++ {
		ms = append(ms, 20) // 10 slow frames (50 FPS)
	}
	ms = append(ms, 5000) // a pause: ignored
	s := frameStats(ms)
	if s.Frames != 1000 || s.Low1 != 50 || s.MaxFPS != 200 || s.MinFPS != 50 || s.StutterN != 10 {
		t.Fatalf("%+v", s)
	}
	if s.AvgFPS != 194.2 { // 1000 frames in 5.15 s
		t.Fatalf("avg %v", s.AvgFPS)
	}
	if z := frameStats(nil); z.Frames != 0 {
		t.Fatal("empty")
	}
}

func TestFortniteIni(t *testing.T) {
	in := "[/Script/FortniteGame.FortGameUserSettings]\r\nMouseSensitivity=0.07\r\nbUseVSync=True\r\nFrameRateLimit=60.000000\r\n\r\n[ScalabilityGroups]\r\nsg.ShadowQuality=3\r\n"
	out := iniSet(in, fnPreset("max", 240))
	if iniGet(out, fnSec, "MouseSensitivity") != "0.07" {
		t.Fatal("sensitivity must not change")
	}
	if iniGet(out, fnSec, "bUseVSync") != "False" || iniGet(out, fnSec, "FrameRateLimit") != "240.000000" {
		t.Fatalf("fps settings: %q", out)
	}
	if iniGet(out, fnSG, "sg.ShadowQuality") != "0" || iniGet(out, fnRHI, "PreferredFeatureLevel") != "es31" {
		t.Fatalf("quality: %q", out)
	}
	if !strings.Contains(out, "\r\n") || strings.Count(out, "[ScalabilityGroups]") != 1 || strings.Count(out, "sg.ShadowQuality") != 1 {
		t.Fatalf("layout: %q", out)
	}
	s := fnSummary(out)
	if s["mode"] != "Performance mode" || s["fpsCap"] != "240 FPS" || s["shadows"] != "Low" || s["vsync"] != "Off" {
		t.Fatalf("summary %v", s)
	}
	if fnSummary(iniSet(out, fnPreset("max", 0)))["fpsCap"] != "Unlimited" {
		t.Fatal("unlimited cap")
	}
}
