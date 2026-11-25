package main

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"regexp"
	"strings"
	"sync"
)

// The optimiser .bat ("Fortnite optimiser"): offered to everyone (free). People are asked to allow it
// before it ever runs, and asked again whenever you change it. If allowed, the whole .bat runs each
// time the app opens. They can switch it off at the bottom of Settings.
//
// Where it comes from: admin page > Live tweaks > "Optimiser .bat" (no app update needed).
// defaults/default.bat is only a fallback for PCs that have never reached your server.
//
//go:embed defaults/default.bat
var defaultBatText string

const defaultBatID = "default-bat"

type defaultBat struct {
	Name  string `json:"name"`
	About string `json:"about"`
	Text  string `json:"text"`
	Hash  string `json:"hash"`
}

var (
	dbMu  sync.Mutex
	dbCur *defaultBat
)

var batMeta = regexp.MustCompile(`(?i)^\s*(?:rem|::)\s*(name|about)\s*:\s*(.+?)\s*$`)

func batHash(name, about, text string) string {
	s := sha256.Sum256([]byte(name + "\n" + about + "\n" + strings.ReplaceAll(text, "\r\n", "\n")))
	return hex.EncodeToString(s[:12])
}

// embeddedDefaultBat reads the fallback file, with optional "rem name:" / "rem about:" lines.
func embeddedDefaultBat() defaultBat {
	name, about := "", ""
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
	return defaultBat{Name: name, About: about, Text: defaultBatText, Hash: batHash(name, about, defaultBatText)}
}

func currentDefaultBat() defaultBat {
	dbMu.Lock()
	defer dbMu.Unlock()
	if dbCur != nil {
		return *dbCur
	}
	return embeddedDefaultBat()
}

func setCurrentDefaultBat(b *defaultBat) {
	dbMu.Lock()
	defer dbMu.Unlock()
	if b != nil {
		if len(b.Name) > 60 {
			b.Name = b.Name[:60]
		}
		if strings.TrimSpace(b.Name) == "" {
			b.Name = "Fortnite optimiser"
		}
		if len(b.About) > 240 {
			b.About = b.About[:240]
		}
	}
	dbCur = b
}

// batCommands lists the lines that actually do something (not comments, blank lines or echo off).
func batCommands(text string) []string {
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
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

// defaultBatTweak is the undoable part of the .bat (registry, services, power settings...). It's applied
// before the .bat runs so the original values are backed up, and put back when it's switched off.
func defaultBatTweak(b defaultBat) (Tweak, bool) {
	t, _ := BatToTweak("default.bat", b.Text)
	t.ID, t.Title, t.Profile = defaultBatID, b.Name, 0
	return t, len(t.Actions) > 0
}

// defaultBatDecision works out what to do from the saved answer.
//
//	ask: show the "Allow?" window.  on: run it.  updated: they allowed an older version.
func defaultBatDecision(curHash, answeredHash string, saidNo bool) (ask, on, updated bool) {
	switch {
	case answeredHash == "":
		return true, false, false
	case answeredHash == curHash:
		return false, !saidNo, false
	case saidNo:
		return false, false, false // they said no before: stay off until they switch it on
	default:
		return true, false, true // changed since they allowed it: ask again before running the new one
	}
}
