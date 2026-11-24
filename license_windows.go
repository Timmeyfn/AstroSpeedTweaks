//go:build windows

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/windows/registry"
)

// Keys are checked against your server (server-url.txt). The result is saved locally, tied to this PC,
// so the app opens instantly and keeps working offline for a while.
//
// Rules:
//   - A saved key only counts on the PC it was activated on.
//   - If the server can't be reached (no internet, server down, misconfigured), the saved key keeps
//     working for offlineGrace since the last successful check. A server problem never wipes a key.
//   - Only a clear answer from the server ("disabled", "used on another PC", "invalid") locks it.

const offlineGrace = 14 * 24 * time.Hour

type licenseFile struct {
	Key    string `json:"key"`
	HWID   string `json:"hwid,omitempty"`
	LastOK int64  `json:"lastOK,omitempty"` // unix time of the last successful online check
	State  string `json:"state,omitempty"`  // "", "disabled", "other_pc", "invalid"
}

type licenseStatus struct {
	Required  bool   `json:"required"`          // a server is configured, so pro features need a key
	Activated bool   `json:"activated"`         // pro features are unlocked on this PC
	Key       string `json:"key"`               // masked
	Offline   bool   `json:"offline"`           // using the saved result because the server didn't answer
	Problem   string `json:"problem,omitempty"` // why a saved key stopped working
}

var (
	licMu       sync.Mutex
	licCur      licenseStatus
	proUnlocked = true
)

func licensePath() string { return filepath.Join(dataDir, "license.json") }

// hwid is a stable, anonymous id for this PC (a hash of the Windows machine GUID).
func hwid() string {
	id := ""
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY); err == nil {
		if v, _, err := k.GetStringValue("MachineGuid"); err == nil {
			id = v
		}
		k.Close()
	}
	if id == "" {
		id = os.Getenv("COMPUTERNAME") + "|" + os.Getenv("PROCESSOR_IDENTIFIER")
	}
	sum := sha256.Sum256([]byte("astrospeed:" + id))
	return hex.EncodeToString(sum[:])[:32]
}

func maskKey(k string) string {
	k = normalizeKey(k)
	if len(k) < 7 {
		return ""
	}
	return k[:6] + "•••••-•••••-" + k[len(k)-5:]
}

func problemText(state string) string {
	switch state {
	case "disabled":
		return "This key has been disabled."
	case "other_pc":
		return "This key is activated on a different PC."
	case "invalid":
		return "This key isn't valid any more."
	}
	return ""
}

func readLicense() licenseFile {
	var lf licenseFile
	if b, err := os.ReadFile(licensePath()); err == nil {
		_ = json.Unmarshal(b, &lf)
	}
	return lf
}

func writeLicense(lf licenseFile) {
	b, _ := json.Marshal(lf)
	tmp := licensePath() + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, licensePath())
	}
}

// evaluate works out the status from the saved file alone (no network).
func evaluate(lf licenseFile) licenseStatus {
	if serverURL() == "" {
		return licenseStatus{Required: false, Activated: true}
	}
	st := licenseStatus{Required: true}
	if lf.Key == "" {
		return st
	}
	st.Key = maskKey(lf.Key)
	me := hwid()
	switch {
	case lf.HWID != "" && lf.HWID != me:
		st.Problem = "This key was activated on a different PC."
	case lf.State != "":
		st.Problem = problemText(lf.State)
	case lf.LastOK == 0: // saved by an older version: allowed until the first check answers
		st.Activated = true
	case time.Since(time.Unix(lf.LastOK, 0)) < offlineGrace:
		st.Activated = true
	default:
		st.Problem = "AstroSpeed needs to go online to confirm your key."
	}
	return st
}

func setLicense(st licenseStatus) (changed bool) {
	licMu.Lock()
	defer licMu.Unlock()
	changed = st != licCur
	licCur = st
	proUnlocked = !st.Required || st.Activated
	return changed
}

// loadLicense runs at startup, before the window opens. It never touches the network.
func loadLicense() { setLicense(evaluate(readLicense())) }

// isProUnlocked is the thread-safe way for the tweak engine to ask.
func isProUnlocked() bool {
	licMu.Lock()
	defer licMu.Unlock()
	return proUnlocked
}

func licenseStatusNow() licenseStatus {
	licMu.Lock()
	defer licMu.Unlock()
	return licCur
}

func postJSON(url string, in interface{}, timeout time.Duration) (map[string]interface{}, int, error) {
	body, _ := json.Marshal(in)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AstroSpeed/"+AppVersion)
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&out); err != nil {
		return nil, resp.StatusCode, err
	}
	return out, resp.StatusCode, nil
}

// refreshLicenseOnline asks the server about the saved key and updates the window if anything changed.
func refreshLicenseOnline() {
	defer func() { _ = recover() }()
	srv := serverURL()
	lf := readLicense()
	if srv == "" || lf.Key == "" || (lf.HWID != "" && lf.HWID != hwid()) {
		return
	}
	res, code, err := postJSON(srv+"/check", map[string]string{"key": lf.Key, "hwid": hwid()}, 8*time.Second)
	st := evaluate(lf)
	if err != nil || code != 200 || res == nil {
		// no answer: keep the saved result
		st.Offline = st.Activated
		logf("Key check: server didn't answer (%v, HTTP %d); using the saved result", err, code)
	} else if ok, _ := res["ok"].(bool); ok {
		lf.HWID, lf.LastOK, lf.State = hwid(), time.Now().Unix(), ""
		writeLicense(lf)
		st = evaluate(lf)
	} else {
		reason, _ := res["reason"].(string)
		if reason == "disabled" || reason == "other_pc" || reason == "invalid" {
			lf.State = reason
			writeLicense(lf)
			st = evaluate(lf)
			logf("Key check: %s", reason)
		} else if reason != "not_activated" {
			st.Offline = st.Activated
		}
	}
	if setLicense(st) {
		emit("license", st)
	}
}

// activateKey is called from the "enter your key" box.
func activateKey(key string) (licenseStatus, error) {
	srv := serverURL()
	if srv == "" {
		return licenseStatus{Required: false, Activated: true}, nil
	}
	key = normalizeKey(key)
	if len(key) < 10 {
		return licenseStatusNow(), errors.New("That doesn't look like a full key. It looks like ASTRO-XXXXX-XXXXX-XXXXX.")
	}
	res, code, err := postJSON(srv+"/activate", map[string]string{"key": key, "hwid": hwid()}, 15*time.Second)
	if err != nil || res == nil {
		return licenseStatusNow(), errors.New("Couldn't reach the activation server. Check your internet connection and try again.")
	}
	if ok, _ := res["ok"].(bool); !ok {
		reason, _ := res["reason"].(string)
		if reason == "" {
			reason = "That key didn't work."
		}
		if code >= 500 {
			reason = "The activation server had a problem. Try again in a minute."
		}
		return licenseStatusNow(), errors.New(reason)
	}
	lf := licenseFile{Key: key, HWID: hwid(), LastOK: time.Now().Unix()}
	writeLicense(lf)
	st := evaluate(lf)
	setLicense(st)
	logf("Key activated: %s", maskKey(key))
	return st, nil
}

// removeKey forgets the saved key (Settings > Your key > Remove).
func removeKey() licenseStatus {
	_ = os.Remove(licensePath())
	st := evaluate(licenseFile{})
	setLicense(st)
	return st
}
