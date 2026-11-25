//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

var defaultBatRun sync.Mutex // one run at a time

func defaultBatCachePath() string { return filepath.Join(dataDir, "default-bat.json") }

func loadDefaultBatCache() {
	var b defaultBat
	if raw, err := os.ReadFile(defaultBatCachePath()); err == nil && json.Unmarshal(raw, &b) == nil && b.Hash != "" {
		setCurrentDefaultBat(&b)
	}
}

// fetchDefaultBat gets the current .bat from your server (admin > Live tweaks > Optimiser .bat).
func fetchDefaultBat() error {
	srv := serverURL()
	if srv == "" {
		return nil
	}
	req, _ := http.NewRequest("GET", srv+"/default-bat", nil)
	req.Header.Set("User-Agent", "AstroSpeed/"+AppVersion)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}
	var d struct {
		Found bool   `json:"found"`
		Name  string `json:"name"`
		About string `json:"about"`
		Text  string `json:"text"`
		Hash  string `json:"hash"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 512<<10)).Decode(&d); err != nil {
		return err
	}
	if !d.Found || strings.TrimSpace(d.Text) == "" {
		setCurrentDefaultBat(nil) // removed in admin: back to the built-in one (usually nothing)
		_ = os.Remove(defaultBatCachePath())
		return nil
	}
	b := defaultBat{Name: d.Name, About: d.About, Text: d.Text, Hash: d.Hash}
	if b.Hash == "" {
		b.Hash = batHash(b.Name, b.About, b.Text)
	}
	setCurrentDefaultBat(&b)
	if raw, err := json.Marshal(b); err == nil {
		_ = os.WriteFile(defaultBatCachePath(), raw, 0o644)
	}
	return nil
}

func defaultBatNow() (b defaultBat, ask, on, updated bool) {
	b = currentDefaultBat()
	stateMu.Lock()
	answered, off := appState.DefaultBatAnswered, appState.DefaultBatOff
	stateMu.Unlock()
	ask, on, updated = defaultBatDecision(b.Hash, answered, off)
	return
}

// defaultBatInfo is what the "Allow?" window and the Settings switch show.
func defaultBatInfo() map[string]interface{} {
	b, ask, on, updated := defaultBatNow()
	cmds := batCommands(b.Text)
	if len(cmds) == 0 {
		return map[string]interface{}{"available": false}
	}
	if len(cmds) > 80 {
		cmds = append(cmds[:80], "…")
	}
	_, undoable := defaultBatTweak(b)
	return map[string]interface{}{"available": true, "asked": !ask, "on": on, "updated": updated, "title": b.Name, "desc": b.About, "commands": cmds, "undoable": undoable, "hash": b.Hash}
}

// startDefaultBat runs at startup: get the latest .bat, then run it if the person allowed this version.
func startDefaultBat() {
	defer func() { _ = recover() }()
	loadDefaultBatCache()
	before := currentDefaultBat().Hash
	if err := fetchDefaultBat(); err != nil {
		logf("Optimiser .bat: couldn't check for a new version (%v); using the saved one", err)
	}
	if currentDefaultBat().Hash != before {
		emit("defaultBatInfo", defaultBatInfo()) // changed: the window may need to ask
	}
	b, _, on, _ := defaultBatNow()
	if !on || len(batCommands(b.Text)) == 0 || !isElevated() {
		return
	}
	time.Sleep(2 * time.Second) // let the window open first
	runDefaultBat(b)
}

// runDefaultBat backs up and applies the undoable part, then runs the whole .bat.
func runDefaultBat(b defaultBat) {
	defer func() { _ = recover() }()
	if !defaultBatRun.TryLock() {
		return
	}
	defer defaultBatRun.Unlock()
	emit("defaultBat", map[string]interface{}{"status": "running", "title": b.Name})

	if t, ok := defaultBatTweak(b); ok {
		engine.mu.Lock()
		engine.ApplyTweak(t, getEnv()) // records the original values so switching off can put them back
		engine.mu.Unlock()
	}

	path := filepath.Join(dataDir, "default.bat")
	text := strings.ReplaceAll(strings.ReplaceAll(b.Text, "\r\n", "\n"), "\n", "\r\n")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		logf("%s: couldn't write the .bat: %v", b.Name, err)
		emit("defaultBat", map[string]interface{}{"status": "failed", "title": b.Name})
		return
	}
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = `C:\Windows\System32\cmd.exe`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	c := exec.CommandContext(ctx, comspec)
	// no console window, "pause" can't hang it; any app the .bat opens still shows normally
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow, CmdLine: `"` + comspec + `" /d /c ""` + path + `" < nul"`}
	c.Dir = dataDir
	var out bytes.Buffer
	c.Stdout, c.Stderr = &out, &out
	start := time.Now()
	err := c.Run()
	tail := strings.TrimSpace(decodeConsole(out.Bytes()))
	if len(tail) > 3000 {
		tail = "…" + tail[len(tail)-3000:]
	}
	status := "done"
	if ctx.Err() == context.DeadlineExceeded {
		status = "failed"
		logf("%s: stopped after 15 minutes", b.Name)
	} else if err != nil {
		logf("%s: finished with an error code (%v)", b.Name, err) // common for .bat files; not fatal
	}
	logf("%s: ran in %s\n%s", b.Name, time.Since(start).Round(time.Second), tail)
	emit("defaultBat", map[string]interface{}{"status": status, "title": b.Name})
}

// undoDefaultBat switches it off and puts back the Windows settings it changed.
func undoDefaultBat() {
	stateMu.Lock()
	st := appState.Tweaks[defaultBatID]
	if appState.DefaultBatAnswered != "" {
		appState.DefaultBatOff = true
		saveStateLocked()
	}
	stateMu.Unlock()
	if st == nil || !isElevated() {
		return
	}
	engine.mu.Lock()
	engine.RevertTweak(Tweak{ID: defaultBatID, Title: currentDefaultBat().Name}, getEnv())
	engine.mu.Unlock()
}

// setDefaultBat is the "Allow?" answer and the Settings switch.
func setDefaultBat(on bool) error {
	b := currentDefaultBat()
	if len(batCommands(b.Text)) == 0 {
		return errors.New("there's nothing to run right now")
	}
	if on && !isElevated() {
		return errors.New("Astro Tweaks needs to run as administrator for this")
	}
	stateMu.Lock()
	appState.DefaultBatAnswered = b.Hash
	appState.DefaultBatOff = !on
	saveStateLocked()
	stateMu.Unlock()
	logf("%s (%s): %s", b.Name, b.Hash, map[bool]string{true: "allowed", false: "not allowed"}[on])
	if on {
		go runDefaultBat(b)
		return nil
	}
	undoDefaultBat()
	return nil
}
