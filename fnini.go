package main

import (
	"strings"
)

// Editing Fortnite's own settings file (GameUserSettings.ini) line by line, keeping everything
// else in it exactly as it was. Sensitivity, keybinds and the rest are never touched.

type iniEdit struct {
	Section string
	Key     string
	Value   string
}

// iniGet reads a value ("" if missing).
func iniGet(text, section, key string) string {
	cur := ""
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		l := strings.TrimSpace(raw)
		if strings.HasPrefix(l, "[") && strings.HasSuffix(l, "]") {
			cur = l[1 : len(l)-1]
			continue
		}
		if strings.EqualFold(cur, section) {
			if i := strings.Index(l, "="); i > 0 && strings.EqualFold(strings.TrimSpace(l[:i]), key) {
				return strings.TrimSpace(l[i+1:])
			}
		}
	}
	return ""
}

// iniSet applies edits: changes existing keys in place, adds missing keys to their section,
// and adds missing sections at the end. Line endings stay as they were (CRLF for Fortnite).
func iniSet(text string, edits []iniEdit) string {
	nl := "\n"
	if strings.Contains(text, "\r\n") || text == "" {
		nl = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	done := make([]bool, len(edits))
	// change keys that exist
	cur := ""
	for i, raw := range lines {
		l := strings.TrimSpace(raw)
		if strings.HasPrefix(l, "[") && strings.HasSuffix(l, "]") {
			cur = l[1 : len(l)-1]
			continue
		}
		eq := strings.Index(l, "=")
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(l[:eq])
		for j, e := range edits {
			if !done[j] && strings.EqualFold(cur, e.Section) && strings.EqualFold(k, e.Key) {
				lines[i] = k + "=" + e.Value
				done[j] = true
			}
		}
	}
	// add missing keys after their section header
	for j, e := range edits {
		if done[j] {
			continue
		}
		at := -1
		for i, raw := range lines {
			if strings.EqualFold(strings.TrimSpace(raw), "["+e.Section+"]") {
				at = i
				break
			}
		}
		if at < 0 {
			for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
				lines = lines[:len(lines)-1]
			}
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, "["+e.Section+"]", e.Key+"="+e.Value, "")
		} else {
			lines = append(lines[:at+1], append([]string{e.Key + "=" + e.Value}, lines[at+1:]...)...)
		}
		done[j] = true
	}
	return strings.Join(lines, nl)
}

const (
	fnSec    = "/Script/FortniteGame.FortGameUserSettings"
	fnSG     = "ScalabilityGroups"
	fnRHI    = "D3DRHIPreference"
	fnSecRHI = "/Script/FortniteGame.FortGameUserSettings"
)

// fnPreset returns the edits for a preset. fpsCap 0 = unlimited.
func fnPreset(name string, fpsCap int) []iniEdit {
	capS := "0.000000"
	if fpsCap > 0 {
		capS = strings.TrimRight(strings.TrimRight(ftoa6(float64(fpsCap)), "0"), ".") + ".000000"
	}
	common := []iniEdit{
		{fnSec, "bUseVSync", "False"},
		{fnSec, "FrameRateLimit", capS},
		{fnSec, "bMotionBlur", "False"},
		{fnSec, "bShowGrass", "False"},
	}
	switch name {
	case "max": // Performance mode, everything low: the most FPS
		return append(common,
			iniEdit{fnRHI, "PreferredRHI", "dx11"},
			iniEdit{fnRHI, "PreferredFeatureLevel", "es31"},
			iniEdit{fnSG, "sg.ResolutionQuality", "100.000000"},
			iniEdit{fnSG, "sg.ViewDistanceQuality", "1"},
			iniEdit{fnSG, "sg.AntiAliasingQuality", "0"},
			iniEdit{fnSG, "sg.ShadowQuality", "0"},
			iniEdit{fnSG, "sg.GlobalIlluminationQuality", "0"},
			iniEdit{fnSG, "sg.ReflectionQuality", "0"},
			iniEdit{fnSG, "sg.PostProcessQuality", "0"},
			iniEdit{fnSG, "sg.TextureQuality", "0"},
			iniEdit{fnSG, "sg.EffectsQuality", "0"},
			iniEdit{fnSG, "sg.FoliageQuality", "0"},
			iniEdit{fnSG, "sg.ShadingQuality", "0"},
		)
	case "balanced": // keeps your rendering mode and textures, drops the expensive effects
		return append(common,
			iniEdit{fnSG, "sg.ResolutionQuality", "100.000000"},
			iniEdit{fnSG, "sg.ViewDistanceQuality", "2"},
			iniEdit{fnSG, "sg.AntiAliasingQuality", "1"},
			iniEdit{fnSG, "sg.ShadowQuality", "0"},
			iniEdit{fnSG, "sg.GlobalIlluminationQuality", "0"},
			iniEdit{fnSG, "sg.ReflectionQuality", "0"},
			iniEdit{fnSG, "sg.PostProcessQuality", "0"},
			iniEdit{fnSG, "sg.TextureQuality", "2"},
			iniEdit{fnSG, "sg.EffectsQuality", "1"},
			iniEdit{fnSG, "sg.FoliageQuality", "0"},
			iniEdit{fnSG, "sg.ShadingQuality", "1"},
		)
	}
	return nil
}

func ftoa6(f float64) string {
	n := int64(f*1e6 + 0.5)
	ip, fp := n/1000000, n%1000000
	s := itoaI64(fp)
	for len(s) < 6 {
		s = "0" + s
	}
	return itoaI64(ip) + "." + s
}

func itoaI64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [24]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// fnSummary describes the current file in plain words for the window.
func fnSummary(text string) map[string]string {
	q := func(v string) string {
		switch v {
		case "0":
			return "Low"
		case "1":
			return "Medium"
		case "2":
			return "High"
		case "3":
			return "Epic"
		case "4":
			return "Cinematic"
		case "":
			return "—"
		}
		return v
	}
	mode := "DirectX 12"
	rhi := strings.ToLower(iniGet(text, fnRHI, "PreferredRHI"))
	lvl := strings.ToLower(iniGet(text, fnRHI, "PreferredFeatureLevel"))
	switch {
	case lvl == "es31":
		mode = "Performance mode"
	case rhi == "dx11":
		mode = "DirectX 11"
	case rhi == "dx12" || rhi == "":
		mode = "DirectX 12"
	}
	capV := iniGet(text, fnSec, "FrameRateLimit")
	capS := "Unlimited"
	if capV != "" && !strings.HasPrefix(capV, "0.") && capV != "0" {
		capS = strings.SplitN(capV, ".", 2)[0] + " FPS"
	}
	vs := "Off"
	if strings.EqualFold(iniGet(text, fnSec, "bUseVSync"), "True") {
		vs = "On"
	}
	return map[string]string{
		"mode": mode, "fpsCap": capS, "vsync": vs,
		"viewDistance": q(iniGet(text, fnSG, "sg.ViewDistanceQuality")),
		"shadows":      q(iniGet(text, fnSG, "sg.ShadowQuality")),
		"textures":     q(iniGet(text, fnSG, "sg.TextureQuality")),
		"effects":      q(iniGet(text, fnSG, "sg.EffectsQuality")),
	}
}
