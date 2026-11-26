//go:build windows

package main

import "sync"

// Windows updates (and some other apps) quietly put settings back. The app spots tweaks that were
// switched on but are now off ("reset"), tells you, and can fix them by itself for key holders.

var (
	buildMu        sync.Mutex
	buildChanged   bool
	buildCheckDone bool
)

// windowsUpdatedSinceLastRun reports whether the Windows build number changed since the app last ran.
func windowsUpdatedSinceLastRun() bool {
	buildMu.Lock()
	defer buildMu.Unlock()
	if !buildCheckDone {
		b := getEnv().Build
		stateMu.Lock()
		if appState.LastBuild != 0 && b != 0 && appState.LastBuild != b {
			buildChanged = true
		}
		if b != 0 && appState.LastBuild != b {
			appState.LastBuild = b
			saveStateLocked()
		}
		stateMu.Unlock()
		buildCheckDone = b != 0 // try again later if Windows' version wasn't read yet
	}
	return buildChanged
}

func resetTweakIDs() []string {
	env := getEnv()
	var ids []string
	for _, t := range Catalog {
		stateMu.Lock()
		st := appState.Tweaks[t.ID]
		stateMu.Unlock()
		if st == nil || !st.Applied {
			continue
		}
		engine.mu.Lock()
		s, _ := engine.Status(t, env, st)
		engine.mu.Unlock()
		if s == "reset" {
			ids = append(ids, t.ID)
		}
	}
	return ids
}

func reapplyOnStart() {
	defer func() { _ = recover() }()
	updated := windowsUpdatedSinceLastRun()
	stateMu.Lock()
	auto := !appState.AutoReapplyOff
	stateMu.Unlock()
	if !auto || !isProUnlocked() || !isElevated() {
		return
	}
	ids := resetTweakIDs()
	if len(ids) == 0 {
		return
	}
	env := getEnv()
	byID := catalogByID()
	fixed := 0
	for _, id := range ids {
		t, ok := byID[id]
		if !ok || (isPro(t) && !isProUnlocked()) {
			continue
		}
		engine.mu.Lock()
		r := engine.ApplyTweak(t, env)
		engine.mu.Unlock()
		if r.Status == "applied" || r.Status == "already" || r.Status == "ok" {
			fixed++
		}
	}
	logf("Re-applied %d of %d tweaks that were changed back (Windows updated: %v)", fixed, len(ids), updated)
	emit("reapplied", map[string]interface{}{"count": fixed, "afterUpdate": updated})
}
