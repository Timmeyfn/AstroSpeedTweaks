//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Backup records exactly what was there before Astro Tweaks changed something.
type Backup struct {
	ID   string `json:"id"` // identity, e.g. reg|HKLM|path|name
	Kind string `json:"kind"`

	Root    string   `json:"root,omitempty"`
	Path    string   `json:"path,omitempty"`
	Name    string   `json:"name,omitempty"`
	Prev    regValue `json:"prev"`
	Created []string `json:"created,omitempty"`

	Svc        string `json:"svc,omitempty"`
	PrevStart  uint32 `json:"prevStart,omitempty"`
	PrevDelay  bool   `json:"prevDelay,omitempty"`
	WasRunning bool   `json:"wasRunning,omitempty"`

	BcdKey  string `json:"bcdKey,omitempty"`
	BcdHad  bool   `json:"bcdHad,omitempty"`
	BcdPrev string `json:"bcdPrev,omitempty"`

	RevertCmd string `json:"revertCmd,omitempty"`
	RevertPS  string `json:"revertPS,omitempty"`

	Handler string          `json:"handler,omitempty"`
	Arg     string          `json:"arg,omitempty"`
	Extra   json.RawMessage `json:"extra,omitempty"`
}

type TweakState struct {
	Applied   bool      `json:"applied"`
	AppliedAt time.Time `json:"appliedAt"`
	Seq       int       `json:"seq"`
	Backups   []Backup  `json:"backups"`
}

type RestorePoint struct {
	When  time.Time `json:"when"`
	Label string    `json:"label"`
	OK    bool      `json:"ok"`
}

type State struct {
	Version       int                    `json:"version"`
	FirstRun      time.Time              `json:"firstRun"`
	Seq           int                    `json:"seq"`
	Tweaks        map[string]*TweakState `json:"tweaks"`
	RestorePoints []RestorePoint         `json:"restorePoints"`
	TotalChanges  int                    `json:"totalChanges"`
	LastOptimize  time.Time              `json:"lastOptimize"`
	Profile       int                    `json:"profile"`
	AutoBoost     bool                   `json:"autoBoost"`
	Hotkey        bool                   `json:"hotkey"`
	NoAutoUpdate  bool                   `json:"noAutoUpdate"`
	DefaultBatOff bool                   `json:"defaultBatOff,omitempty"`
	DefaultBatAsked bool                 `json:"defaultBatAsked,omitempty"` // the "Allow?" question was answered
	AutoClean     bool                   `json:"autoClean,omitempty"`
	LastAutoClean time.Time              `json:"lastAutoClean,omitempty"`
	MemGuard      bool                   `json:"memGuard,omitempty"`
}

var (
	stateMu   sync.Mutex
	appState  *State
	dataDir   string
	logMu     sync.Mutex
	logLines  []string
	logFileFn string
)

func initDataDir() {
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	dataDir = filepath.Join(pd, "AstroSpeed")
	_ = os.MkdirAll(filepath.Join(dataDir, "logs"), 0o755)
	// carry over backups from when the app was called AstroSpeed, so Undo keeps working
	old := filepath.Join(pd, "StratoGlide")
	if _, err := os.Stat(filepath.Join(dataDir, "state.json")); err != nil {
		for _, f := range []string{"state.json", "state.backup.json"} {
			if b, err := os.ReadFile(filepath.Join(old, f)); err == nil {
				_ = os.WriteFile(filepath.Join(dataDir, f), b, 0o644)
			}
		}
	}
	logFileFn = filepath.Join(dataDir, "logs", time.Now().Format("2006-01-02")+".log")
}

func loadState() {
	stateMu.Lock()
	defer stateMu.Unlock()
	appState = &State{Version: 1, Tweaks: map[string]*TweakState{}, FirstRun: time.Now(), Profile: 2}
	b, err := os.ReadFile(filepath.Join(dataDir, "state.json"))
	if err != nil {
		return
	}
	var s State
	if json.Unmarshal(b, &s) == nil {
		if s.Tweaks == nil {
			s.Tweaks = map[string]*TweakState{}
		}
		if s.Profile == 0 {
			s.Profile = 2
		}
		appState = &s
	}
}

// saveStateLocked must be called with stateMu held.
func saveStateLocked() {
	b, err := json.MarshalIndent(appState, "", " ")
	if err != nil {
		return
	}
	fn := filepath.Join(dataDir, "state.json")
	tmp := fn + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, fn)
	}
	// keep a rolling copy so a corrupted write can never lose the backups
	_ = os.WriteFile(filepath.Join(dataDir, "state.backup.json"), b, 0o644)
}

func logf(format string, a ...interface{}) {
	line := time.Now().Format("15:04:05") + "  " + fmt.Sprintf(format, a...)
	logMu.Lock()
	logLines = append(logLines, line)
	if len(logLines) > 3000 {
		logLines = logLines[len(logLines)-3000:]
	}
	logMu.Unlock()
	f, err := os.OpenFile(logFileFn, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err == nil {
		_, _ = f.WriteString(strings.ReplaceAll(line, "\n", " ") + "\r\n")
		f.Close()
	}
}

func recentLog(n int) []string {
	logMu.Lock()
	defer logMu.Unlock()
	if n <= 0 || n > len(logLines) {
		n = len(logLines)
	}
	out := make([]string, n)
	copy(out, logLines[len(logLines)-n:])
	return out
}
