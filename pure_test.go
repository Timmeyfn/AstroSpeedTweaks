package main

import "testing"

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
