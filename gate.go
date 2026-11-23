package main

import "strings"

// isPro reports whether a tweak is part of the paid (key-unlocked) set.
// Free users get the safe Recommended tweaks, the booster, cleaner and checkup;
// everything below needs a key. Change this one function to change the split.
func isPro(t Tweak) bool {
	if t.Profile == 2 { // Competitive profile
		return true
	}
	if strings.HasPrefix(t.Cat, "game-") { // per-game tweaks
		return true
	}
	switch t.Cat {
	case "kernel", "gpu", "network", "online": // BIOS/kernel, advanced GPU & network, live tweaks
		return true
	}
	return false
}

// online (live) tweaks are a paid perk too
func init() {} // keep file importable without extra deps
