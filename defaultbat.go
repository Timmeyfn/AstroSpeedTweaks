package main

import (
	_ "embed"
	"strings"
)

// The default .bat: built into the exe, free for everyone, applied every time Astro Tweaks opens,
// and shown as a switch at the bottom of Settings so people can turn it off.
// To change it, replace defaults/default.bat and run build.bat.
//
//go:embed defaults/default.bat
var defaultBatText string

const defaultBatID = "default-bat"

// defaultBatTweak turns the embedded .bat into a tweak. ok is false when the file has no commands.
func defaultBatTweak() (Tweak, []string, bool) {
	t, skipped := BatToTweak("default.bat", defaultBatText)
	if len(t.Actions) == 0 {
		return t, skipped, false
	}
	t.ID = defaultBatID
	t.Cat = "default" // not a section in the menu: it lives in Settings
	t.Group = "Default"
	t.Title = "Astro Tweaks default tweaks"
	if strings.TrimSpace(t.Desc) == "" {
		t.Desc = "Built into Astro Tweaks and applied each time it opens."
	}
	t.Benefit = "Runs every time Astro Tweaks opens. Turning it off undoes it."
	t.Profile = 0 // never part of Optimize; it has its own switch
	t.Online = false
	return t, skipped, true
}

func init() {
	if t, _, ok := defaultBatTweak(); ok {
		Catalog = append(Catalog, t)
	}
}
