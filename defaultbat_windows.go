//go:build windows

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

var defaultBatMu sync.Mutex // one run at a time

func defaultBatState() (asked, on bool) {
	stateMu.Lock()
	defer stateMu.Unlock()
	return appState.DefaultBatAsked, appState.DefaultBatAsked && !appState.DefaultBatOff
}

// defaultBatInfo is what the "Allow?" window and the Settings switch show.
func defaultBatInfo() map[string]interface{} {
	if !defaultBatAvailable() {
		return map[string]interface{}{"available": false}
	}
	name, about := defaultBatNameAbout()
	asked, on := defaultBatState()
	cmds := defaultBatCommands()
	if len(cmds) > 60 {
		cmds = append(cmds[:60], "…")
	}
	_, undoable := catalogByID()[defaultBatID]
	return map[string]interface{}{"available": true, "asked": asked, "on": on, "title": name, "desc": about, "commands": cmds, "undoable": undoable}
}

// applyDefaultBatOnStart runs the .bat each time the app opens, but only if the person allowed it.
func applyDefaultBatOnStart() {
	defer func() { _ = recover() }()
	if !defaultBatAvailable() || !isElevated() {
		return
	}
	if _, on := defaultBatState(); !on {
		return // not asked yet (the window asks), or they said no / switched it off
	}
	time.Sleep(2 * time.Second) // let the window open first
	runDefaultBat()
}

// runDefaultBat backs up and applies the undoable part, then runs the whole .bat.
func runDefaultBat() {
	defer func() { _ = recover() }()
	if !defaultBatMu.TryLock() {
		return
	}
	defer defaultBatMu.Unlock()
	name, _ := defaultBatNameAbout()
	emit("defaultBat", map[string]interface{}{"status": "running", "title": name})

	if t, ok := catalogByID()[defaultBatID]; ok {
		env := getEnv()
		stateMu.Lock()
		st := appState.Tweaks[t.ID]
		stateMu.Unlock()
		engine.mu.Lock()
		if s, _ := engine.Status(t, env, st); s != "on" && s != "native" && s != "na" {
			engine.ApplyTweak(t, env) // records the original values so "off" can put them back
		}
		engine.mu.Unlock()
	}

	path := filepath.Join(dataDir, "default.bat")
	text := strings.ReplaceAll(strings.ReplaceAll(defaultBatText, "\r\n", "\n"), "\n", "\r\n")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		logf("%s: couldn't write the .bat: %v", name, err)
		emit("defaultBat", map[string]interface{}{"status": "failed", "title": name})
		return
	}
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = `C:\Windows\System32\cmd.exe`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	c := exec.CommandContext(ctx, comspec)
	// no console window; any app the .bat opens still shows normally
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
		logf("%s: stopped after 15 minutes", name)
	} else if err != nil {
		logf("%s: finished with an error (%v)", name, err) // .bat files often end with a non-zero code; not fatal
	}
	logf("%s: ran in %s\n%s", name, time.Since(start).Round(time.Second), tail)
	emit("defaultBat", map[string]interface{}{"status": status, "title": name})
}

// setDefaultBat is the "Allow?" answer and the Settings switch.
func setDefaultBat(on bool) error {
	if !defaultBatAvailable() {
		return errors.New("there's nothing to run in this version")
	}
	if on && !isElevated() {
		return errors.New("Astro Tweaks needs to run as administrator for this")
	}
	stateMu.Lock()
	appState.DefaultBatAsked = true
	appState.DefaultBatOff = !on
	saveStateLocked()
	stateMu.Unlock()
	name, _ := defaultBatNameAbout()
	logf("%s: %s", name, map[bool]string{true: "allowed", false: "not allowed"}[on])
	if on {
		go runDefaultBat()
		return nil
	}
	// switched off: put back the settings it changed that can be undone
	if t, ok := catalogByID()[defaultBatID]; ok && isElevated() {
		env := getEnv()
		engine.mu.Lock()
		engine.RevertTweak(t, env)
		engine.mu.Unlock()
	}
	return nil
}
