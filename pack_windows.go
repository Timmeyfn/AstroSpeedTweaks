//go:build windows

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// The online pack lives in the same GitHub repository as the updates (update-repo.txt):
//   astrospeed.json   and/or   tweaks/*.bat
// It is downloaded at start and every 30 minutes, checked, cached in ProgramData\Astro Tweaks
// and merged into the catalog. If GitHub can't be reached, the last good copy is used.

type packCache struct {
	Hash  string            `json:"hash"`
	JSON  string            `json:"json"`
	Bats  map[string]string `json:"bats"`
	When  string            `json:"when"`
	Repo  string            `json:"repo"`
	Found bool              `json:"found"`
}

type packInfo struct {
	Configured bool              `json:"configured"`
	Repo       string            `json:"repo"`
	Found      bool              `json:"found"`
	Tweaks     int               `json:"tweaks"`
	Games      int               `json:"games"`
	Sections   int               `json:"sections"`
	Message    string            `json:"message"`
	Problems   []string          `json:"problems"`
	Images     map[string]string `json:"images"`
	Checked    string            `json:"checked"`
	Error      string            `json:"error"`
}

var (
	packMu         sync.Mutex
	packCur        PackResult
	packCached     packCache
	packChecked    time.Time
	packErr        string
	baseCategories []Category
	baseCatalog    []Tweak
	baseGamesList  []GameDef
	baseOnce       sync.Once
)

func packFile() string { return filepath.Join(dataDir, "online-pack.json") }

func snapshotBase() {
	baseOnce.Do(func() {
		baseCategories = append([]Category{}, Categories...)
		baseCatalog = append([]Tweak{}, Catalog...)
		baseGamesList = append([]GameDef{}, Games...)
	})
}

// loadCachedPack runs once at start, before games are detected, so the last pack is ready instantly.
func loadCachedPack() {
	snapshotBase()
	b, err := os.ReadFile(packFile())
	if err != nil {
		return
	}
	var c packCache
	if json.Unmarshal(b, &c) != nil || c.Repo != serverURL() {
		return
	}
	applyPack(c, false)
	logf("Online pack loaded from cache (%d tweaks, %d games)", len(packCur.Tweaks), len(packCur.Games))
}

func applyPack(c packCache, live bool) {
	var p RemotePack
	var problems []string
	if strings.TrimSpace(c.JSON) != "" {
		var err error
		p, err = ParsePack([]byte(c.JSON))
		if err != nil {
			problems = append(problems, err.Error())
		}
	}
	res := CheckPack(p, c.Bats, baseCategories, baseCatalog, baseGamesList)
	res.Problems = append(problems, res.Problems...)
	res.Fetched = c.When
	cats, tw, games := MergePack(baseCategories, baseCatalog, baseGamesList, res)

	if live {
		jobMu.Lock() // never swap the catalog in the middle of Optimize or Undo
		defer jobMu.Unlock()
	}
	gamesBefore := len(Games)
	Categories, Catalog, Games = cats, tw, games
	packMu.Lock()
	packCur, packCached = res, c
	packMu.Unlock()
	for _, pr := range res.Problems {
		logf("Online pack: %s", pr)
	}
	if live && (len(games) != gamesBefore || len(res.Games) > 0) {
		go rescanGames()
	}
}

func packStatus() packInfo {
	packMu.Lock()
	defer packMu.Unlock()
	info := packInfo{Repo: serverURL(), Found: packCached.Found, Tweaks: len(packCur.Tweaks), Games: len(packCur.Games),
		Sections: len(packCur.Categories), Message: packCur.Message, Problems: packCur.Problems, Images: packCur.Images, Error: packErr}
	info.Configured = info.Repo != ""
	if !packChecked.IsZero() {
		info.Checked = packChecked.Format(time.RFC3339)
	}
	if info.Problems == nil {
		info.Problems = []string{}
	}
	return info
}

func refreshPackLoop() {
	defer func() { _ = recover() }()
	time.Sleep(4 * time.Second) // let the window open first
	for {
		if _, err := refreshPack(false); err != nil {
			logf("Online pack: %v", err)
		}
		time.Sleep(30 * time.Minute)
	}
}

var packFetchMu sync.Mutex

// refreshPack downloads the pack; when it changed, it is applied and the window is told.
func refreshPack(manual bool) (packInfo, error) {
	packFetchMu.Lock()
	defer packFetchMu.Unlock()
	repo := serverURL()
	if repo == "" {
		return packStatus(), nil
	}
	snapshotBase()
	c, err := fetchPack(repo)
	packMu.Lock()
	packChecked = time.Now()
	if err != nil {
		packErr = err.Error()
	} else {
		packErr = ""
	}
	same := err == nil && c.Hash == packCached.Hash && packCached.Repo == repo
	packMu.Unlock()
	if err != nil {
		if manual {
			return packStatus(), err
		}
		return packStatus(), err
	}
	if !same {
		applyPack(c, true)
		if b, e := json.Marshal(c); e == nil {
			tmp := packFile() + ".tmp"
			if os.WriteFile(tmp, b, 0o644) == nil {
				_ = os.Rename(tmp, packFile())
			}
		}
		logf("Online pack updated: %d tweaks, %d games, %d sections", len(packCur.Tweaks), len(packCur.Games), len(packCur.Categories))
		emit("packUpdated", packStatus())
	}
	return packStatus(), nil
}

func fetchPack(server string) (packCache, error) {
	c := packCache{Repo: server, Bats: map[string]string{}, When: time.Now().Format(time.RFC3339)}
	req, _ := http.NewRequest("GET", server+"/pack", nil)
	req.Header.Set("User-Agent", "AstroSpeed/"+AppVersion)
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := (&http.Client{Timeout: 12 * time.Second}).Do(req)
	if err != nil {
		return c, errors.New("couldn't connect. Check your internet connection")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return c, fmt.Errorf("the tweak server answered %d", resp.StatusCode)
	}
	var d struct {
		Found bool              `json:"found"`
		JSON  string            `json:"json"`
		Bats  map[string]string `json:"bats"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&d); err != nil {
		return c, errors.New("couldn't read the live tweaks")
	}
	c.Found, c.JSON = d.Found, d.JSON
	n := 0
	for name, body := range d.Bats {
		low := strings.ToLower(name)
		if !(strings.HasSuffix(low, ".bat") || strings.HasSuffix(low, ".cmd")) || len(body) > 256<<10 {
			continue
		}
		if n++; n > 30 {
			break
		}
		c.Bats[name] = body
	}

	h := sha256.New()
	h.Write([]byte(c.JSON))
	names := make([]string, 0, len(c.Bats))
	for n := range c.Bats {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		h.Write([]byte("\x00" + n + "\x00" + c.Bats[n]))
	}
	c.Hash = hex.EncodeToString(h.Sum(nil))
	return c, nil
}
