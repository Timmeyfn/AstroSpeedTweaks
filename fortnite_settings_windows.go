//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// The Fortnite settings tuner: writes Fortnite's own settings file (not the game) with a backup
// first, so "Put my old settings back" restores it exactly.

func fnSettingsPath() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "FortniteGame", "Saved", "Config", "WindowsClient", "GameUserSettings.ini")
}

func fnBackupDir() string { return filepath.Join(dataDir, "backups", "fortnite-settings") }

func fnBackups() []string {
	m, _ := filepath.Glob(filepath.Join(fnBackupDir(), "GameUserSettings-*.ini"))
	sort.Strings(m)
	return m
}

func fnSettingsInfo() map[string]interface{} {
	p := fnSettingsPath()
	b, err := os.ReadFile(p)
	out := map[string]interface{}{"found": err == nil, "running": fortniteRunning(), "backups": len(fnBackups())}
	if err == nil {
		out["current"] = fnSummary(string(b))
	}
	hz := 0
	for _, d := range listDisplays() {
		if d.Primary || hz == 0 {
			hz = d.Hz
		}
	}
	out["refreshHz"] = hz
	return out
}

func fnApply(preset string, fpsCap int) (map[string]interface{}, error) {
	if fortniteRunning() {
		return nil, errors.New("Close Fortnite first. It saves its own settings when it quits and would undo these")
	}
	edits := fnPreset(preset, fpsCap)
	if edits == nil {
		return nil, errors.New("unknown preset")
	}
	if fpsCap < 0 || fpsCap > 1000 {
		return nil, errors.New("that FPS cap doesn't look right")
	}
	p := fnSettingsPath()
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, errors.New("Fortnite's settings file wasn't found. Open Fortnite once, close it, then try again")
	}
	if err := os.MkdirAll(fnBackupDir(), 0o755); err != nil {
		return nil, err
	}
	bk := filepath.Join(fnBackupDir(), "GameUserSettings-"+time.Now().Format("20060102-150405")+".ini")
	if err := os.WriteFile(bk, b, 0o644); err != nil {
		return nil, errors.New("couldn't back up the settings, so nothing was changed")
	}
	// keep the 10 newest backups
	if all := fnBackups(); len(all) > 10 {
		for _, old := range all[:len(all)-10] {
			_ = os.Remove(old)
		}
	}
	_ = os.Chmod(p, 0o666) // clear read-only if someone set it
	tmp := p + ".astro.tmp"
	if err := os.WriteFile(tmp, []byte(iniSet(string(b), edits)), 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return nil, err
	}
	logf("Fortnite settings: applied %q (cap %d), backup %s", preset, fpsCap, filepath.Base(bk))
	return fnSettingsInfo(), nil
}

func fnRestore() (map[string]interface{}, error) {
	if fortniteRunning() {
		return nil, errors.New("Close Fortnite first")
	}
	all := fnBackups()
	if len(all) == 0 {
		return nil, errors.New("there's no backup to put back")
	}
	// the oldest backup from the latest run of changes = the settings from before Astro Tweaks
	b, err := os.ReadFile(all[0])
	if err != nil {
		return nil, err
	}
	p := fnSettingsPath()
	_ = os.Chmod(p, 0o666)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		return nil, err
	}
	for _, f := range all {
		_ = os.Remove(f)
	}
	logf("Fortnite settings: restored the original file")
	return fnSettingsInfo(), nil
}
