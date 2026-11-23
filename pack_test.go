package main

import (
	"os"
	"strings"
	"testing"
)

func TestPackChecks(t *testing.T) {
	raw := `// comment line
{
  "message": "Hello",
  "categories": [{"id": "my-stuff", "title": "My stuff"}],
  "tweaks": [
    {"id": "good", "cat": "my-stuff", "title": "Good one", "actions": [{"kind": "reg", "root": "HKLM", "path": "SOFTWARE\\Test", "name": "A", "num": 1}]},
    {"id": "mouse", "title": "Bad mouse", "actions": [{"kind": "reg", "root": "HKCU", "path": "Control Panel\\Mouse", "name": "MouseSpeed", "type": "SZ", "str": "0"}]},
    {"id": "wipe", "title": "Bad cmd", "actions": [{"kind": "cmd", "apply": "format C: /q"}]},
    {"id": "dl", "title": "Download", "actions": [{"kind": "ps", "apply": "Invoke-WebRequest http://x -OutFile a.exe"}]},
    {"id": "rd", "title": "Remove", "actions": [{"kind": "cmd", "apply": "rd /s /q C:\\Windows"}]},
    {"id": "fn-priority", "title": "Duplicate", "actions": [{"kind": "cmd", "apply": "echo hi"}]}
  ],
  "games": [{"id": "halo", "name": "Halo Infinite", "exes": ["HaloInfinite.exe"], "steamDir": ["Halo Infinite"], "image": "https://example.com/h.jpg"}],
  "images": {"rust": "https://example.com/r.jpg", "bad": "javascript:alert(1)"}
}`
	p, err := ParsePack([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	bats := map[string]string{"my_network.bat": "@echo off\r\nrem Lowers network latency a bit\r\nreg add \"HKLM\\SOFTWARE\\X\" /v Y /t REG_DWORD /d 1 /f\r\nnetsh int tcp set global ecncapability=disabled\r\nif exist a (\r\n echo x\r\n)\r\npause\r\n"}
	res := CheckPack(p, bats, Categories, Catalog, Games)
	ids := map[string]bool{}
	for _, tw := range res.Tweaks {
		ids[tw.ID] = true
	}
	if !ids["good"] || !ids["bat-my-network"] {
		t.Fatalf("expected tweaks missing: %v / %v", ids, res.Problems)
	}
	for _, bad := range []string{"mouse", "wipe", "dl", "rd", "fn-priority"} {
		if ids[bad] {
			t.Errorf("%s should have been rejected", bad)
		}
	}
	if len(res.Games) != 1 || res.Games[0].Image == "" || !res.Games[0].Online {
		t.Errorf("game not accepted: %+v", res.Games)
	}
	if res.Images["rust"] == "" || res.Images["bad"] != "" {
		t.Errorf("images wrong: %v", res.Images)
	}
	var bat Tweak
	for _, tw := range res.Tweaks {
		if tw.ID == "bat-my-network" {
			bat = tw
		}
	}
	if len(bat.Actions) != 2 || bat.Actions[0].Kind != "reg" || bat.Actions[1].Kind != "cmd" || bat.Desc != "Lowers network latency a bit" {
		t.Errorf("bat conversion wrong: %+v", bat)
	}
	if !strings.Contains(strings.Join(res.Problems, "\n"), "if/for") {
		t.Errorf("expected a skipped-block note: %v", res.Problems)
	}
	cats, tw, games := MergePack(Categories, Catalog, Games, res)
	if len(games) != len(Games)+1 || len(tw) <= len(Catalog) || len(cats) <= len(Categories) {
		t.Error("merge didn't add things")
	}
	hasOnline := false
	for _, c := range cats {
		if c.ID == "online" {
			hasOnline = true
		}
	}
	if !hasOnline {
		t.Error("online section missing")
	}
	// the built-in catalog itself must still pass the runtime rules
	for _, c := range Catalog {
		for _, a := range c.Actions {
			if r := protectedReason(a); r != "" {
				t.Errorf("built-in %s: %s", c.ID, r)
			}
		}
	}
}

func TestStarterPack(t *testing.T) {
	b, err := os.ReadFile("github-repo-starter/astrospeed.json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParsePack(b)
	if err != nil {
		t.Fatal(err)
	}
	bat, _ := os.ReadFile("github-repo-starter/tweaks/example_network.bat")
	res := CheckPack(p, map[string]string{"example_network.bat": string(bat)}, Categories, Catalog, Games)
	if len(res.Problems) > 0 || len(res.Tweaks) != 3 || len(res.Games) != 1 {
		t.Fatalf("starter pack: %d tweaks, %d games, problems %v", len(res.Tweaks), len(res.Games), res.Problems)
	}
}
