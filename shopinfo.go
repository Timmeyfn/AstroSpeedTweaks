//go:build windows

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

// shopInfo is the price shown in the upgrade window, read from your website's /config.json
// so changing the price in admin > Settings updates the app too. Cached for 30 seconds.
var (
	shopMu   sync.Mutex
	shopVal  map[string]interface{}
	shopWhen time.Time
)

func shopInfo() map[string]interface{} {
	shopMu.Lock()
	defer shopMu.Unlock()
	if shopVal != nil && time.Since(shopWhen) < 30*time.Second {
		return shopVal
	}
	out := map[string]interface{}{}
	srv := serverURL()
	if srv == "" {
		return out
	}
	req, _ := http.NewRequest("GET", srv+"/config.json", nil)
	req.Header.Set("User-Agent", "AstroSpeed/"+AppVersion)
	resp, err := (&http.Client{Timeout: 6 * time.Second}).Do(req)
	if err != nil {
		return out
	}
	defer resp.Body.Close()
	var cfg map[string]interface{}
	if resp.StatusCode == 200 && json.NewDecoder(io.LimitReader(resp.Body, 32<<10)).Decode(&cfg) == nil {
		for _, k := range []string{"priceLabel", "priceShort", "buyEnabled"} {
			if v, ok := cfg[k]; ok {
				out[k] = v
			}
		}
		shopVal, shopWhen = out, time.Now()
	}
	return out
}
