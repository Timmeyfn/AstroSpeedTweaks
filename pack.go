package main

// Online packs: extra tweaks, sections and games that AstroSpeed downloads from your GitHub
// repository, so you can add things without rebuilding the app.
//
//   astrospeed.json       sections, tweaks, games, game pictures and an optional message
//   tweaks/*.bat          each .bat file becomes one switch (same rules as build.bat)
//
// Everything downloaded goes through the same safety rules as the built-in catalog before
// it is shown, and nothing is ever applied until someone switches it on or runs Optimize.

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type RemotePack struct {
	Message    string            `json:"message"`
	Categories []Category        `json:"categories"`
	Tweaks     []Tweak           `json:"tweaks"`
	Games      []RemoteGame      `json:"games"`
	Images     map[string]string `json:"images"`
}

type RemoteGame struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Studio   string   `json:"studio"`
	Exes     []string `json:"exes"`
	Color    string   `json:"color"`
	SteamDir []string `json:"steamDir"`
	EpicApp  string   `json:"epicApp"`
	Riot     string   `json:"riot"`
	Uninst   []string `json:"apps"`
	Paths    []string `json:"paths"`
	Rel      []string `json:"rel"`
	Note     string   `json:"note"`
	Image    string   `json:"image"`
}

// PackResult is a checked pack, ready to merge into the catalog.
type PackResult struct {
	Message    string            `json:"message"`
	Categories []Category        `json:"categories"`
	Tweaks     []Tweak           `json:"tweaks"`
	Games      []GameDef         `json:"games"`
	Images     map[string]string `json:"images"`
	Problems   []string          `json:"problems"`
	Fetched    string            `json:"fetched"`
	Source     string            `json:"source"`
}

// ───────────────────────── safety rules ─────────────────────────

// Areas AstroSpeed promises never to touch (shared with catalog_test.go).
var protectedAreas = []string{
	`control panel\mouse`, `control panel\keyboard`, `control panel\cursors`,
	`mouclass`, `kbdclass`, `mousesensitivity`, `mousespeed`, `mousethreshold`, `smoothmouse`, `enhance pointer`,
	`fortnitegame\saved\config`, `gameusersettings`, `epicgameslauncher\saved\config`,
	`control\graphicsdrivers\configuration`, `control\graphicsdrivers\connectivity`,
	`wpcmonsvc`, `windefend`, `wuauserv`, `securityhealth`, `mpssvc`, `enablelua`, `consentpromptbehavior`,
}

// Commands an online pack may never run, because they can destroy data or lock people out.
var dangerousCmd = regexp.MustCompile(`(?i)(^|[\s&|;"'(])(format(\.com)?\s|diskpart|vssadmin|wbadmin|cipher\s+/w|shutdown|takeown|icacls|bcdedit|net\s+user|net\s+localgroup|cacls|sdelete|reagentc|manage-bde|invoke-webrequest|iwr\s|curl\s|wget\s|bitsadmin|certutil|start-bitstransfer|downloadstring|downloadfile|-encodedcommand|-enc\s|frombase64string|set-mppreference|add-mppreference)`)
var rmRoot = regexp.MustCompile(`(?i)\b(rd|rmdir|del|erase|remove-item)\b.*(\s/s\b|-recurse).*?([a-z]:\\?(\s|$|"|\*)|\\windows\b|\\system32\b|\\program files|\\users\\[^\\]+\\?(\s|$|"))`)

func actionBlob(a Action) string {
	return strings.ToLower(a.Root + `\` + a.Path + `\` + a.Name + " " + a.Svc + " " + a.Task + " " + a.Apply + " " + a.Revert + " " + a.Key)
}

// protectedReason says why an action can't be used, or "" when it's fine.
func protectedReason(a Action) string {
	blob := actionBlob(a)
	for _, f := range protectedAreas {
		if strings.Contains(blob, f) {
			return "touches " + f + ", which AstroSpeed promises never to change"
		}
	}
	return ""
}

func checkRemoteAction(a Action) (Action, string) {
	switch a.Kind {
	case "reg", "regdel":
		a.Root = strings.ToUpper(a.Root)
		if a.Root != HKLM && a.Root != HKCU {
			return a, "registry root must be HKLM or HKCU"
		}
		a.Path = strings.Trim(a.Path, `\`)
		if a.Path == "" || a.Name == "" && a.Kind == "regdel" {
			return a, "registry path and value name are needed"
		}
		if a.Kind == "reg" {
			a.Type = strings.ToUpper(a.Type)
			switch a.Type {
			case "DWORD", "QWORD", "SZ":
			case "":
				a.Type = "DWORD"
			default:
				return a, "registry type must be DWORD, QWORD or SZ"
			}
		}
	case "svc":
		if a.Svc == "" || (a.Start != "disabled" && a.Start != "manual" && a.Start != "auto") {
			return a, `service needs a name and start "disabled", "manual" or "auto"`
		}
	case "task":
		if a.Task == "" {
			return a, "scheduled task path is missing"
		}
	case "bcd":
		if !regexp.MustCompile(`^[a-z]+$`).MatchString(a.Key) || strings.ContainsAny(a.Val, `&|;"'<>`) {
			return a, "boot setting name or value isn't allowed"
		}
	case "cmd", "ps":
		if strings.TrimSpace(a.Apply) == "" {
			return a, "command is empty"
		}
		for _, c := range []string{a.Apply, a.Revert} {
			if dangerousCmd.MatchString(c) || rmRoot.MatchString(c) {
				return a, "this command isn't allowed in online packs"
			}
		}
		if a.Revert == "" {
			a.Revert = "rem AstroSpeed: no automatic undo for this line"
		}
	default:
		return a, "unknown action kind " + strconv.Quote(a.Kind)
	}
	if r := protectedReason(a); r != "" {
		return a, r
	}
	return a, ""
}

var idRe = regexp.MustCompile(`[^a-z0-9]+`)

func cleanID(s string) string {
	return strings.Trim(idRe.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// CheckPack validates everything in a downloaded pack against the built-in catalog.
func CheckPack(p RemotePack, bats map[string]string, baseCats []Category, baseTweaks []Tweak, baseGames []GameDef) PackResult {
	res := PackResult{Message: strings.TrimSpace(p.Message), Images: map[string]string{}}
	if len(res.Message) > 400 {
		res.Message = res.Message[:400]
	}
	taken := map[string]bool{}
	for _, t := range baseTweaks {
		taken[t.ID] = true
	}
	catOK := map[string]bool{}
	for _, c := range baseCats {
		catOK[c.ID] = true
	}
	gameTaken := map[string]bool{}
	for _, g := range baseGames {
		gameTaken[g.ID] = true
	}
	problem := func(f string, a ...interface{}) { res.Problems = append(res.Problems, fmt.Sprintf(f, a...)) }

	for _, c := range p.Categories {
		c.ID = cleanID(c.ID)
		if c.ID == "" || c.Title == "" {
			problem("A section is missing its id or title")
			continue
		}
		if catOK[c.ID] {
			continue // built-in section: tweaks can still be added to it
		}
		if c.Icon == "" {
			c.Icon = "spark"
		}
		c.Game = ""
		catOK[c.ID] = true
		res.Categories = append(res.Categories, c)
	}

	addTweak := func(t Tweak, origin string) {
		t.ID = cleanID(t.ID)
		if t.ID == "" || t.Title == "" {
			problem("%s: a tweak is missing its id or title", origin)
			return
		}
		if taken[t.ID] {
			problem("%s: the id %q is already used", origin, t.ID)
			return
		}
		if t.Cat == "" || !catOK[t.Cat] {
			t.Cat = "online"
		}
		if t.Desc == "" {
			t.Desc = "A live tweak, delivered without an app update."
		}
		if t.Benefit == "" {
			t.Benefit = "Applied with the rest of Optimize, and undone like every other tweak."
		}
		if t.Group == "" {
			t.Group = "Latest"
		}
		if t.Impact < 1 || t.Impact > 3 {
			t.Impact = 2
		}
		if t.Risk != "safe" && t.Risk != "moderate" && t.Risk != "expert" {
			t.Risk = "moderate"
		}
		if t.Risk == "expert" || t.Profile < 0 || t.Profile > 2 {
			t.Profile = 0
		}
		var acts []Action
		for i, a := range t.Actions {
			ca, why := checkRemoteAction(a)
			if why != "" {
				problem("%s › %s, step %d: %s", origin, t.Title, i+1, why)
				continue
			}
			acts = append(acts, ca)
		}
		if len(acts) == 0 {
			problem("%s › %s: nothing left to apply, so it was left out", origin, t.Title)
			return
		}
		t.Actions = acts
		var when []string
		for _, w := range t.When {
			switch {
			case w == "nvidia" || w == "amd" || w == "discord" || w == "ram16" || w == "win11" || w == "fortnite":
				when = append(when, w)
			case strings.HasPrefix(w, "game:"):
				when = append(when, w)
			}
		}
		t.When = when
		taken[t.ID] = true
		res.Tweaks = append(res.Tweaks, t)
	}
	for _, t := range p.Tweaks {
		addTweak(t, "astrospeed.json")
	}
	names := make([]string, 0, len(bats))
	for n := range bats {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		t, skipped := BatToTweak(n, bats[n])
		for _, s := range skipped {
			problem("%s: %s", n, s)
		}
		if len(t.Actions) > 0 {
			addTweak(t, n)
		}
	}
	needOnline := false
	for _, t := range res.Tweaks {
		if t.Cat == "online" {
			needOnline = true
		}
	}
	if needOnline && !catOK["online"] {
		res.Categories = append(res.Categories, Category{ID: "online", Title: "Live tweaks", Nav: "Live", Subtitle: "Fresh tweaks delivered straight to the app. New ones appear here by themselves, no update needed.", Icon: "spark"})
	}

	for _, rg := range p.Games {
		id := cleanID(rg.ID)
		if id == "" || rg.Name == "" || len(rg.Exes) == 0 {
			problem("A game is missing its id, name or exe")
			continue
		}
		if gameTaken[id] {
			problem("The game id %q is already used", id)
			continue
		}
		var exes []string
		for _, e := range rg.Exes {
			if strings.HasSuffix(strings.ToLower(e), ".exe") && !strings.ContainsAny(e, `\/:*?"<>|`) {
				exes = append(exes, e)
			}
		}
		if len(exes) == 0 {
			problem("%s: exes must be file names like Game.exe", rg.Name)
			continue
		}
		color := rg.Color
		if !regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`).MatchString(color) {
			color = "#6B7280"
		}
		g := GameDef{ID: id, Name: rg.Name, Studio: rg.Studio, Exes: exes, Color: color, SteamDir: rg.SteamDir, EpicApp: rg.EpicApp,
			Riot: rg.Riot, Uninst: rg.Uninst, Paths: rg.Paths, Rel: rg.Rel, Note: rg.Note, Online: true}
		if strings.HasPrefix(rg.Image, "https://") {
			g.Image = rg.Image
		}
		gameTaken[id] = true
		res.Games = append(res.Games, g)
	}
	for id, url := range p.Images {
		if strings.HasPrefix(url, "https://") && !strings.ContainsAny(url, `"'<> `) {
			res.Images[cleanID(id)] = url
		}
	}
	return res
}

// MergePack returns the catalog with a checked pack added on top of the built-in one.
func MergePack(baseCats []Category, baseTweaks []Tweak, baseGames []GameDef, p PackResult) ([]Category, []Tweak, []GameDef) {
	cats := append([]Category{}, baseCats...)
	tweaks := append([]Tweak{}, baseTweaks...)
	games := append([]GameDef{}, baseGames...)
	cats = append(cats, p.Categories...)
	for i := range p.Tweaks {
		p.Tweaks[i].Online = true
	}
	tweaks = append(tweaks, p.Tweaks...)
	for _, g := range p.Games {
		c, tw := gameTweaks(g)
		cats = append(cats, c)
		for i := range tw {
			tw[i].Online = true
		}
		tweaks = append(tweaks, tw...)
		games = append(games, g)
	}
	return cats, tweaks, games
}

// ParsePack reads astrospeed.json, tolerating // comment lines.
func ParsePack(b []byte) (RemotePack, error) {
	var lines []string
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "//") {
			continue
		}
		lines = append(lines, ln)
	}
	var p RemotePack
	err := json.Unmarshal([]byte(strings.Join(lines, "\n")), &p)
	if err != nil {
		return p, fmt.Errorf("astrospeed.json has a mistake: %v", err)
	}
	return p, nil
}

// ───────────────────────── .bat → tweak (same rules as tools/bat2tweak.py) ─────────────────────────

var (
	batSilent  = map[string]bool{"echo": true, "cls": true, "title": true, "color": true, "setlocal": true, "endlocal": true, "chcp": true, "mode": true, "exit": true, "pause": true, "prompt": true, "cd": true, "pushd": true, "popd": true, "@echo": true}
	batSkip    = map[string]string{"goto": "only works inside a .bat file", "call": "only works inside a .bat file", "set": "only works inside a .bat file", "timeout": "only waits", "choice": "asks the user a question", "start": "opens programs or windows", "shutdown": "would restart the PC in the middle of Optimize", "if": "only works inside a .bat file", "for": "only works inside a .bat file", "else": "only works inside a .bat file", "ping": "usually only used as a wait"}
	batRedir   = regexp.MustCompile(`(?i)\s*(>>?|2>&1|2>)\s*nul\b`)
	batVars    = regexp.MustCompile(`(?i)%%|%~|%[0-9*]|![A-Za-z_]+!|%errorlevel%`)
	batArgs    = regexp.MustCompile(`"[^"]*"|\S+`)
	batRoots   = map[string]string{"HKLM": HKLM, "HKEY_LOCAL_MACHINE": HKLM, "HKCU": HKCU, "HKEY_CURRENT_USER": HKCU}
	batSCStart = map[string]string{"disabled": "disabled", "demand": "manual", "auto": "auto", "delayed-auto": "auto"}
)

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func batFlag(args []string, name string) (string, bool) {
	for i, a := range args {
		if strings.EqualFold(a, name) && i+1 < len(args) {
			return unquote(args[i+1]), true
		}
	}
	return "", false
}

func batConvert(line string) (kind string, act Action, why string) {
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "@")
	s = strings.TrimSpace(s)
	low := strings.ToLower(s)
	if s == "" || strings.HasPrefix(s, "::") || low == "rem" || strings.HasPrefix(low, "rem ") || strings.HasPrefix(s, ":") || s == "(" || s == ")" {
		return "blank", act, ""
	}
	s = strings.TrimSpace(batRedir.ReplaceAllString(s, ""))
	if s == "" {
		return "blank", act, ""
	}
	fields := strings.Fields(s)
	word := strings.ToLower(fields[0])
	if batSilent[word] || (word == "net" && len(fields) > 1 && strings.EqualFold(fields[1], "session")) {
		return "blank", act, ""
	}
	if r, ok := batSkip[word]; ok {
		return "skip", act, r
	}
	if batVars.MatchString(s) {
		return "skip", act, "uses .bat-only variables"
	}
	if strings.Contains(s, "&&") || strings.Contains(s, "||") || regexp.MustCompile(`(^|\s)\|\s`).MatchString(s) {
		return "skip", act, "joins several commands together; put each on its own line"
	}
	args := batArgs.FindAllString(s, -1)
	switch word {
	case "reg", "reg.exe":
		if len(args) >= 3 {
			key := strings.Trim(unquote(args[2]), `\`)
			head, rest, _ := strings.Cut(key, `\`)
			root := batRoots[strings.ToUpper(head)]
			name, hasName := batFlag(args, "/v")
			if root != "" && rest != "" && hasName {
				if strings.EqualFold(args[1], "delete") {
					return "undo", RegDel(root, rest, name), ""
				}
				if strings.EqualFold(args[1], "add") {
					typ, _ := batFlag(args, "/t")
					typ = strings.ToUpper(typ)
					if typ == "" {
						typ = "REG_SZ"
					}
					data, _ := batFlag(args, "/d")
					switch typ {
					case "REG_DWORD", "REG_QWORD":
						n, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(data), "0x"), map[bool]int{true: 16, false: 10}[strings.HasPrefix(strings.ToLower(data), "0x")], 64)
						if err == nil && (typ == "REG_QWORD" || n <= 0xFFFFFFFF) {
							if typ == "REG_DWORD" {
								return "undo", RegD(root, rest, name, n), ""
							}
							return "undo", RegQ(root, rest, name, n), ""
						}
					case "REG_SZ":
						return "undo", RegS(root, rest, name, data), ""
					}
				}
			}
		}
	case "sc", "sc.exe":
		if len(args) >= 4 && strings.EqualFold(args[1], "config") {
			for i, a := range args {
				la := strings.ToLower(a)
				if la == "start=" && i+1 < len(args) {
					if st, ok := batSCStart[strings.ToLower(args[i+1])]; ok {
						return "undo", Svc(unquote(args[2]), st), ""
					}
				} else if strings.HasPrefix(la, "start=") && len(la) > 6 {
					if st, ok := batSCStart[la[6:]]; ok {
						return "undo", Svc(unquote(args[2]), st), ""
					}
				}
			}
		}
	case "schtasks", "schtasks.exe":
		ls := strings.ToLower(s)
		if strings.Contains(ls, "/change") && strings.Contains(ls, "/disable") {
			if tn, ok := batFlag(args, "/tn"); ok {
				return "undo", Task(tn), ""
			}
		}
	case "bcdedit", "bcdedit.exe":
		var rest []string
		for _, a := range args[1:] {
			if !strings.EqualFold(a, "{current}") {
				rest = append(rest, a)
			}
		}
		if len(rest) == 3 && strings.EqualFold(rest[0], "/set") {
			return "undo", BCD(strings.ToLower(rest[1]), rest[2]), ""
		}
		if len(rest) == 2 && strings.EqualFold(rest[0], "/deletevalue") {
			return "undo", BCD(strings.ToLower(rest[1]), ""), ""
		}
	}
	return "cmd", Cmd(s, "rem AstroSpeed: no automatic undo for this line"), ""
}

// BatToTweak turns one .bat file into a tweak; the second value lists lines that were left out.
func BatToTweak(fileName, text string) (Tweak, []string) {
	stem := strings.TrimSuffix(strings.TrimSuffix(fileName, ".bat"), ".cmd")
	title := strings.TrimSpace(regexp.MustCompile(`[_\-]+`).ReplaceAllString(stem, " "))
	if title == "" {
		title = "Custom tweaks"
	}
	title = strings.ToUpper(title[:1]) + title[1:]
	t := Tweak{ID: "bat-" + cleanID(stem), Cat: "online", Group: "Latest", Title: title,
		Benefit: "Applied with the rest of Optimize, and undone like every other tweak.", Impact: 2, Risk: "moderate", Profile: 2}
	var skipped []string
	text = strings.ReplaceAll(text, "\r\n", "\n")
	raw := strings.Split(text, "\n")
	var lines []string
	buf := ""
	for _, ln := range raw {
		ln = strings.TrimRight(ln, " \t\r")
		if strings.HasSuffix(ln, "^") {
			buf += strings.TrimSuffix(ln, "^")
			continue
		}
		lines = append(lines, buf+ln)
		buf = ""
	}
	if buf != "" {
		lines = append(lines, buf)
	}
	depth, nCmd, nUndo := 0, 0, 0
	for i, ln := range lines {
		st := strings.TrimLeft(strings.TrimSpace(ln), "@")
		low := strings.ToLower(st)
		if t.Desc == "" && (strings.HasPrefix(low, "rem ") || strings.HasPrefix(st, "::")) {
			d := strings.TrimSpace(strings.TrimLeft(strings.TrimPrefix(st, "::"), ": "))
			if strings.HasPrefix(low, "rem ") {
				d = strings.TrimSpace(st[4:])
			}
			if len(d) > 8 && strings.Trim(d, "=-*#~ ") != "" {
				t.Desc = d
			}
		}
		opens := strings.HasSuffix(st, "(")
		closes := strings.HasPrefix(st, ")")
		isBlock := regexp.MustCompile(`^(if|for)\b`).MatchString(low)
		if depth > 0 || (opens && isBlock) {
			if closes {
				depth--
			}
			if opens {
				depth++
			}
			first := ""
			if f := strings.Fields(low); len(f) > 0 {
				first = f[0]
			}
			if st != ")" && st != "(" && !regexp.MustCompile(`^\)\s*else\s*\($`).MatchString(st) && !batSilent[first] {
				skipped = append(skipped, fmt.Sprintf("line %d skipped (inside an if/for block)", i+1))
			}
			continue
		}
		kind, act, why := batConvert(ln)
		switch kind {
		case "undo":
			t.Actions = append(t.Actions, act)
			nUndo++
		case "cmd":
			t.Actions = append(t.Actions, act)
			nCmd++
		case "skip":
			skipped = append(skipped, fmt.Sprintf("line %d skipped (%s)", i+1, why))
		}
	}
	if t.Desc == "" {
		t.Desc = "A live tweak, delivered without an app update."
	}
	if nCmd > 0 {
		t.Note = fmt.Sprintf("%d of its %d changes can't be undone automatically.", nCmd, nCmd+nUndo)
	}
	return t, skipped
}
