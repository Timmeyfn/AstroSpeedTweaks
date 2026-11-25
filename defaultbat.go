package main

import (
	_ "embed"
	"regexp"
	"strings"
)

// The default .bat: built into the exe and offered to everyone (free). The FIRST time Astro Tweaks
// opens, people are asked whether to allow it. Only if they allow it does the whole .bat run, and
// then it runs each time the app opens. They can switch it off at the bottom of Settings at any time.
//
// To change it, replace defaults/default.bat and run build.bat. Optional lines at the top:
//
//	rem name: Fortnite optimiser          <- the name shown to people
//	rem about: What it does, in one line   <- shown in the "Allow?" window
//
//go:embed defaults/default.bat
var defaultBatText string

const defaultBatID = "default-bat"

var batMeta = regexp.MustCompile(`(?i)^\s*(?:rem|::)\s*(name|about)\s*:\s*(.+?)\s*$`)

// defaultBatCommands lists the lines that actually do something (not comments, blank lines or echo off).
func defaultBatCommands() []string {
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(defaultBatText, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(l)
		lt := strings.ToLower(t)
		if t == "" || strings.HasPrefix(lt, "rem") && (len(lt) == 3 || lt[3] == ' ' || lt[3] == '\t') || strings.HasPrefix(t, "::") ||
			lt == "@echo off" || lt == "echo off" || lt == "exit" || lt == "exit /b" || lt == "exit /b 0" || strings.HasPrefix(lt, "pause") || strings.HasPrefix(lt, "title ") || lt == "cls" {
			continue
		}
		out = append(out, t)
	}
	return out
}

func defaultBatAvailable() bool { return len(defaultBatCommands()) > 0 }

// defaultBatNameAbout reads "rem name:" and "rem about:" from the .bat.
func defaultBatNameAbout() (name, about string) {
	for _, l := range strings.Split(strings.ReplaceAll(defaultBatText, "\r\n", "\n"), "\n") {
		if m := batMeta.FindStringSubmatch(l); m != nil {
			if strings.EqualFold(m[1], "name") && name == "" {
				name = m[2]
			} else if strings.EqualFold(m[1], "about") && about == "" {
				about = m[2]
			}
		}
	}
	if name == "" {
		name = "Fortnite optimiser"
	}
	if len(name) > 60 {
		name = name[:60]
	}
	if len(about) > 240 {
		about = about[:240]
	}
	return
}

// defaultBatTweak is the undoable part of the .bat (registry, services, power settings...).
// It's applied before the .bat runs so the original values are backed up, and switched back
// when the person turns the .bat off. ok is false when nothing in it can be undone.
func defaultBatTweak() (Tweak, []string, bool) {
	t, skipped := BatToTweak("default.bat", defaultBatText)
	if len(t.Actions) == 0 {
		return t, skipped, false
	}
	name, about := defaultBatNameAbout()
	t.ID = defaultBatID
	t.Cat = "default" // not a section in the menu: it lives in Settings
	t.Group = "Default"
	t.Title = name
	if about != "" {
		t.Desc = about
	} else if strings.TrimSpace(t.Desc) == "" {
		t.Desc = "Built into Astro Tweaks."
	}
	t.Benefit = "Runs each time Astro Tweaks opens, if you allowed it."
	t.Profile = 0 // never part of Optimize; it has its own switch
	t.Online = false
	return t, skipped, true
}

func init() {
	if t, _, ok := defaultBatTweak(); ok {
		Catalog = append(Catalog, t)
	}
}
