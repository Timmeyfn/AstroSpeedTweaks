//go:build windows

package main

import (
	"errors"
	"time"
)

func defaultBatFromCatalog() (Tweak, bool) {
	t, ok := catalogByID()[defaultBatID]
	return t, ok
}

// defaultBatInfo is what the Settings switch shows.
func defaultBatInfo() map[string]interface{} {
	t, ok := defaultBatFromCatalog()
	if !ok {
		return map[string]interface{}{"available": false}
	}
	stateMu.Lock()
	off := appState.DefaultBatOff
	stateMu.Unlock()
	return map[string]interface{}{"available": true, "on": !off, "title": t.Title, "desc": t.Desc, "changes": len(t.Actions)}
}

// applyDefaultBatOnStart makes sure the default .bat is applied each time the app opens,
// unless the user switched it off. It re-applies anything Windows changed back.
func applyDefaultBatOnStart() {
	defer func() { _ = recover() }()
	t, ok := defaultBatFromCatalog()
	if !ok {
		return
	}
	stateMu.Lock()
	off := appState.DefaultBatOff
	st := appState.Tweaks[t.ID]
	stateMu.Unlock()
	if off || !isElevated() {
		return
	}
	time.Sleep(2 * time.Second) // let the window open first
	env := getEnv()
	engine.mu.Lock()
	status, _ := engine.Status(t, env, st)
	if status == "on" || status == "native" || status == "na" {
		engine.mu.Unlock()
		return
	}
	r := engine.ApplyTweak(t, env)
	engine.mu.Unlock()
	logf("Default tweaks: %s (%d change(s))", r.Status, r.Changes)
	emit("defaultBat", map[string]interface{}{"status": r.Status})
}

// setDefaultBat is the Settings switch: on applies it now, off undoes it.
func setDefaultBat(on bool) error {
	t, ok := defaultBatFromCatalog()
	if !ok {
		return errors.New("there are no default tweaks in this version")
	}
	if !isElevated() {
		return errors.New("AstroSpeed needs to run as administrator to change this")
	}
	env := getEnv()
	engine.mu.Lock()
	var r TweakResult
	if on {
		r = engine.ApplyTweak(t, env)
	} else {
		r = engine.RevertTweak(t, env)
	}
	engine.mu.Unlock()
	if r.Status == "failed" {
		return errors.New(firstNonEmpty(r.Msg, "it couldn't be changed"))
	}
	stateMu.Lock()
	appState.DefaultBatOff = !on
	saveStateLocked()
	stateMu.Unlock()
	logf("Default tweaks switched %v: %s", on, r.Status)
	return nil
}
