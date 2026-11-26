//go:build windows

package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// In-app self-update: download the new exe with a progress bar, then swap it in and relaunch.
// No cmd.exe / PowerShell is spawned — it's plain file operations plus one relaunch.

var updateMu sync.Mutex
var updateReady string // path of a downloaded, verified new exe waiting to be applied

// cleanupOldUpdate removes the ".old" copy left by a previous update. Called at startup.
func cleanupOldUpdate() {
	if self, err := os.Executable(); err == nil {
		_ = os.Remove(self + ".old")
		_ = os.Remove(self + ".new")
	}
}

type progressReader struct {
	r        io.Reader
	total    int64
	received int64
	last     time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.received += int64(n)
	if time.Since(p.last) > 120*time.Millisecond || err == io.EOF {
		p.last = time.Now()
		pct := 0
		if p.total > 0 {
			pct = int(p.received * 100 / p.total)
		}
		ev := map[string]interface{}{"pct": pct, "receivedMB": float64(p.received) / (1 << 20)}
		if p.total > 0 {
			ev["totalMB"] = float64(p.total) / (1 << 20)
		}
		emit("updateProgress", ev)
	}
	return n, err
}

// downloadUpdate fetches the new exe next to the current one, showing progress.
func downloadUpdate() (interface{}, error) {
	updateMu.Lock()
	defer updateMu.Unlock()

	info, _ := checkUpdate()
	if !info.Available || info.Download == "" {
		return nil, errors.New("no update available")
	}
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	dest := self + ".new"

	req, _ := http.NewRequest("GET", info.Download, nil)
	req.Header.Set("User-Agent", "AstroSpeed/"+AppVersion)
	resp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return nil, errors.New("couldn't reach the download. Check your internet connection")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("the download isn't available right now")
	}

	out, err := os.Create(dest)
	if err != nil {
		return nil, err
	}
	pr := &progressReader{r: resp.Body, total: resp.ContentLength, last: time.Now()}
	_, err = io.Copy(out, pr)
	out.Close()
	if err != nil {
		_ = os.Remove(dest)
		return nil, errors.New("the download was interrupted")
	}

	// sanity check: a real Windows program starts with "MZ"
	f, err := os.Open(dest)
	if err != nil {
		return nil, err
	}
	head := make([]byte, 2)
	_, _ = f.Read(head)
	f.Close()
	fi, _ := os.Stat(dest)
	if string(head) != "MZ" || fi == nil || fi.Size() < 1<<20 {
		_ = os.Remove(dest)
		return nil, errors.New("the downloaded file doesn't look right, so it was discarded")
	}
	// the new exe carries its own version; refuse anything that isn't exactly the version offered
	if b, err := os.ReadFile(dest); err != nil || !bytes.Contains(b, []byte("ASTROSPEED_VERSION="+info.Latest+";")) {
		_ = os.Remove(dest)
		return nil, errors.New("the downloaded file isn't version " + info.Latest + ", so it wasn't installed. Try again in a minute")
	}

	updateReady = dest
	logf("Update downloaded: %s (%.1f MB)", filepath.Base(dest), float64(fi.Size())/(1<<20))
	emit("updateProgress", map[string]interface{}{"pct": 100, "done": true})
	return map[string]interface{}{"ok": true}, nil
}

// applyUpdate swaps the downloaded exe in and relaunches. The current process then exits.
func applyUpdate() (interface{}, error) {
	updateMu.Lock()
	defer updateMu.Unlock()
	if updateReady == "" {
		return nil, errors.New("nothing to install yet")
	}
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	old := self + ".old"
	_ = os.Remove(old)
	// Windows lets a running exe be renamed, so move the current one aside...
	if err := os.Rename(self, old); err != nil {
		return nil, errors.New("couldn't replace the old version (is it write-protected?)")
	}
	// ...then put the new one in its place.
	if err := os.Rename(updateReady, self); err != nil {
		_ = os.Rename(old, self) // put it back on failure
		return nil, errors.New("couldn't install the new version")
	}
	logf("Update applied, relaunching")
	markCleanExit() // a planned restart, not a crash
	stopFrameCounter()
	overlayHide()
	if instanceMutex != 0 {
		windows.CloseHandle(instanceMutex) // let the new copy start
		instanceMutex = 0
	}
	verb, _ := windows.UTF16PtrFromString("open")
	file, _ := windows.UTF16PtrFromString(self)
	args, _ := windows.UTF16PtrFromString("--after-update")
	_ = windows.ShellExecute(0, verb, file, args, nil, windows.SW_SHOWNORMAL)
	go func() {
		time.Sleep(600 * time.Millisecond)
		os.Exit(0)
	}()
	return map[string]interface{}{"ok": true}, nil
}
