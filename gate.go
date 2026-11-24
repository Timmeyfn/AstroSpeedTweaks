package main

import "encoding/json"

// Free vs paid.
//
// Free: the dashboard, the PC Checkup scan, the ping test, Undo (always), and the few sample
// tweaks below so people can see AstroSpeed working. Everything else needs a key.
// To change what's free, edit freeTweaks.
var freeTweaks = map[string]bool{
	"gamemode":   true, // Turn on Game Mode
	"gamedvr":    true, // Turn off Game DVR background recording
	"telemetry":  true, // Turn down Windows telemetry
	defaultBatID: true, // the built-in default .bat (Settings > bottom)
}

// isPro reports whether a tweak needs a key.
func isPro(t Tweak) bool { return !freeTweaks[t.ID] }

// paidTool reports whether an app action needs a key. Scans and read-only checks stay free
// (so the locked pages can show what they found); the actions that change the PC don't.
// Turning things OFF and Undo are never gated.
func paidTool(method string, args []byte) bool {
	var a struct {
		Key string `json:"key"`
		On  bool   `json:"on"`
	}
	_ = json.Unmarshal(args, &a)
	switch method {
	case "cleanRun", "startupSet", "boostPurge", "boostPriority", "bgClose", "dnsSet":
		return true
	case "boostTimer", "setAutoBoost":
		return a.On
	case "setSetting":
		return (a.Key == "hotkey" || a.Key == "autoClean" || a.Key == "memGuard") && a.On
	}
	return false
}
