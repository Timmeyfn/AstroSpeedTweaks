//go:build windows

package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

// license-url.txt holds the address of your Cloudflare key server, e.g.
//
//	https://astrospeed-keys.yourname.workers.dev
//
// Leave it blank and the whole app stays free with every feature unlocked.
//
//go:embed license-url.txt
var licenseURLFile string

func licenseServer() string {
	for _, ln := range strings.Split(licenseURLFile, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		if strings.HasPrefix(ln, "https://") {
			return strings.TrimRight(ln, "/ ")
		}
	}
	return ""
}

type licenseFile struct {
	Key string `json:"key"`
}

type licenseStatus struct {
	Required  bool   `json:"required"`  // a key server is configured
	Activated bool   `json:"activated"` // this PC has a working key
	Key       string `json:"key"`       // masked
	Offline   bool   `json:"offline"`   // couldn't reach the server this launch
}

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
	k = strings.TrimSpace(k)
	if len(k) < 5 {
		return ""
	}
	parts := strings.Split(k, "-")
	if len(parts) > 1 {
		return parts[0] + strings.Repeat("-•••••", len(parts)-1)
	}
	return k[:3] + "•••••"
}

func readLicenseKey() string {
	b, err := os.ReadFile(licensePath())
	if err != nil {
		return ""
	}
	var lf licenseFile
	if json.Unmarshal(b, &lf) != nil {
		return ""
	}
	return lf.Key
}

func saveLicenseKey(key string) {
	b, _ := json.Marshal(licenseFile{Key: key})
	_ = os.WriteFile(licensePath(), b, 0o644)
}

func postJSON(url string, in interface{}) (map[string]interface{}, error) {
	body, _ := json.Marshal(in)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AstroSpeed/"+AppVersion)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out, nil
}

// proUnlocked is read by the tweak engine; true when no server is set (all free) or the key checks out.
var proUnlocked = true

func refreshLicense() licenseStatus {
	srv := licenseServer()
	if srv == "" {
		proUnlocked = true
		return licenseStatus{Required: false, Activated: true}
	}
	key := readLicenseKey()
	if key == "" {
		proUnlocked = false
		return licenseStatus{Required: true, Activated: false}
	}
	res, err := postJSON(srv+"/check", map[string]string{"key": key, "hwid": hwid()})
	if err != nil {
		// Can't reach the server: trust the saved key for this launch so no internet doesn't lock people out.
		proUnlocked = true
		return licenseStatus{Required: true, Activated: true, Key: maskKey(key), Offline: true}
	}
	ok, _ := res["ok"].(bool)
	if !ok {
		proUnlocked = false
		_ = os.Remove(licensePath()) // disabled or invalid: drop it so they can enter another
		return licenseStatus{Required: true, Activated: false}
	}
	proUnlocked = true
	return licenseStatus{Required: true, Activated: true, Key: maskKey(key)}
}

// activateKey is called from the activation screen.
func activateKey(key string) (licenseStatus, error) {
	srv := licenseServer()
	key = strings.ToUpper(strings.TrimSpace(key))
	if srv == "" {
		return licenseStatus{Required: false, Activated: true}, nil
	}
	res, err := postJSON(srv+"/activate", map[string]string{"key": key, "hwid": hwid()})
	if err != nil {
		return licenseStatus{Required: true, Activated: false}, errOf("Couldn't reach the activation server. Check your internet connection.")
	}
	if ok, _ := res["ok"].(bool); !ok {
		reason, _ := res["reason"].(string)
		if reason == "" {
			reason = "That key didn't work."
		}
		return licenseStatus{Required: true, Activated: false}, errOf(reason)
	}
	saveLicenseKey(key)
	proUnlocked = true
	return licenseStatus{Required: true, Activated: true, Key: maskKey(key)}, nil
}

func errOf(s string) error { return &simpleErr{s} }

type simpleErr struct{ s string }

func (e *simpleErr) Error() string { return e.s }
