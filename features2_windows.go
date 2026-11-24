//go:build windows

package main

import "time"

// Weekly auto-clean: once Astro Tweaks has been open and it's been 7 days, it clears the usual
// temp files and caches by itself. The Recycle Bin is never emptied automatically.
func autoCleanOnStart() {
	defer func() { _ = recover() }()
	time.Sleep(20 * time.Second) // stay out of the way while the app opens
	stateMu.Lock()
	on, last := appState.AutoClean, appState.LastAutoClean
	stateMu.Unlock()
	if !on || !isProUnlocked() || time.Since(last) < 7*24*time.Hour {
		return
	}
	var ids []string
	for _, t := range cleanTargets {
		if t.Default && t.ID != "recycle" {
			ids = append(ids, t.ID)
		}
	}
	var freed int64
	files := 0
	for _, r := range cleanerRun(ids) {
		freed += r.Bytes
		files += r.Files
	}
	stateMu.Lock()
	appState.LastAutoClean = time.Now()
	saveStateLocked()
	stateMu.Unlock()
	logf("Weekly clean: removed %d files (%.1f MB)", files, float64(freed)/(1<<20))
	emit("autoCleaned", map[string]interface{}{"freedMB": float64(freed) / (1 << 20), "files": files})
}

// Memory guard: while a supported game is running and RAM is nearly full, clear Windows'
// standby memory (at most every 5 minutes) so the game isn't starved mid-match.
func memGuardLoop() {
	defer func() { _ = recover() }()
	var lastPurge time.Time
	for {
		time.Sleep(time.Minute)
		stateMu.Lock()
		on := appState.MemGuard
		stateMu.Unlock()
		if !on || !isProUnlocked() || time.Since(lastPurge) < 5*time.Minute {
			continue
		}
		if memStatus().MemoryLoad < 85 || runningGame(listProcesses()) == nil {
			continue
		}
		freed, err := purgeStandby()
		lastPurge = time.Now()
		if err == nil {
			logf("Memory guard: cleared %.0f MB of standby memory during a game", freed)
			emit("memGuard", map[string]interface{}{"freedMB": freed})
		}
	}
}
