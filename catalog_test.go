package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Things AstroSpeed promises never to change.
var forbidden = []string{
	`control panel\mouse`,    // pointer speed, acceleration, double-click
	`control panel\keyboard`, // repeat rate/delay
	`control panel\cursors`,  // cursor scheme
	`mouclass`, `kbdclass`,   // mouse/keyboard driver buffers
	`mousesensitivity`, `mousespeed`, // sensitivity
	`mousethreshold`, `smoothmouse`, // acceleration curves
	`enhance pointer`,                       //
	`fortnitegame\saved\config`,             // Fortnite settings & keybinds
	`gameusersettings`,                      //
	`epicgameslauncher\saved\config`,        // launcher settings
	`control\graphicsdrivers\configuration`, // resolution / refresh rate
	`control\graphicsdrivers\connectivity`,
	`wpcmonsvc`,                                         // parental controls must never be disabled
	`windefend`, `wuauserv`, `securityhealth`, `mpssvc`, // Defender, Windows Update, firewall
	`enablelua`, `consentpromptbehavior`, // UAC
}

func TestCatalogNeverTouchesProtectedSettings(t *testing.T) {
	for _, tw := range Catalog {
		for _, a := range tw.Actions {
			blob := strings.ToLower(a.Root + `\` + a.Path + `\` + a.Name + " " + a.Svc + " " + a.Apply + " " + a.Revert + " " + a.Describe())
			for _, f := range forbidden {
				if strings.Contains(blob, f) {
					t.Errorf("%s touches protected area %q: %s", tw.ID, f, a.Describe())
				}
			}
		}
	}
}

func TestCatalogIsWellFormed(t *testing.T) {
	cats := map[string]bool{}
	for _, c := range Categories {
		cats[c.ID] = true
	}
	ids := map[string]bool{}
	idempotentNoRevert := map[string]bool{"tcp-rss": true, "tcp-autotune": true}
	for _, tw := range Catalog {
		if ids[tw.ID] {
			t.Errorf("duplicate id %s", tw.ID)
		}
		ids[tw.ID] = true
		if !cats[tw.Cat] && tw.ID != defaultBatID { // the default .bat lives in Settings, not a section
			t.Errorf("%s has unknown category %s", tw.ID, tw.Cat)
		}
		if tw.Title == "" || tw.Desc == "" || tw.Benefit == "" {
			t.Errorf("%s is missing copy", tw.ID)
		}
		if tw.Impact < 1 || tw.Impact > 3 {
			t.Errorf("%s impact out of range", tw.ID)
		}
		if tw.Risk != "safe" && tw.Risk != "moderate" && tw.Risk != "expert" {
			t.Errorf("%s has bad risk %q", tw.ID, tw.Risk)
		}
		if tw.Risk == "expert" && tw.Profile != 0 {
			t.Errorf("%s is expert but would run in one-click Optimize", tw.ID)
		}
		if tw.Cat == "appearance" && tw.Profile != 0 {
			t.Errorf("%s changes appearance but would run in one-click Optimize", tw.ID)
		}
		if len(tw.Actions) == 0 {
			t.Errorf("%s has no actions", tw.ID)
		}
		for _, a := range tw.Actions {
			switch a.Kind {
			case "cmd", "ps":
				if a.Revert == "" && !idempotentNoRevert[tw.ID] {
					t.Errorf("%s runs a command with no undo: %s", tw.ID, a.Apply)
				}
			case "reg", "regdel":
				if a.Root != HKLM && a.Root != HKCU {
					t.Errorf("%s bad root %s", tw.ID, a.Root)
				}
				if strings.HasPrefix(a.Path, `\`) || strings.HasSuffix(a.Path, `\`) {
					t.Errorf("%s bad path %s", tw.ID, a.Path)
				}
			case "svc":
				if a.Start != "disabled" && a.Start != "manual" && a.Start != "auto" {
					t.Errorf("%s bad start type", tw.ID)
				}
			case "dyn", "custom":
				if _, ok := handlerDescriptions[a.Handler]; !ok {
					t.Errorf("%s uses handler %s with no description", tw.ID, a.Handler)
				}
			case "task", "bcd":
			default:
				t.Errorf("%s unknown action kind %s", tw.ID, a.Kind)
			}
		}
	}
	n, acts := CatalogStats()
	t.Logf("%d tweaks, %d static actions", n, acts)
	prof := map[int]int{}
	for _, tw := range Catalog {
		prof[tw.Profile]++
	}
	t.Logf("profiles: recommended=%d competitive=%d manual=%d", prof[1], prof[2], prof[0])
}

// Writes the catalog for the browser preview of the UI.
func TestDumpCatalog(t *testing.T) {
	out := os.Getenv("SG_DUMP")
	if out == "" {
		t.Skip()
	}
	type tweakUI struct {
		Tweak
		Changes []string `json:"changes"`
	}
	var tw []tweakUI
	for _, x := range Catalog {
		u := tweakUI{Tweak: x}
		for _, a := range x.Actions {
			u.Changes = append(u.Changes, a.Describe())
		}
		tw = append(tw, u)
	}
	n, acts := CatalogStats()
	b, _ := json.Marshal(map[string]interface{}{"categories": Categories, "catalog": tw, "tweakCount": n, "actionCount": acts, "games": Games})
	if err := os.WriteFile(out, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
