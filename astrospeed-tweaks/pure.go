package main

import "strings"

// setKV edits a "Key=Value;Key=Value;" string, keeping everything else intact.
func setKV(s, key, val string) string {
	var parts []string
	found := false
	for _, kv := range strings.Split(s, ";") {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		if i := strings.Index(kv, "="); i > 0 && strings.EqualFold(kv[:i], key) {
			parts = append(parts, key+"="+val)
			found = true
			continue
		}
		parts = append(parts, kv)
	}
	if !found {
		parts = append(parts, key+"="+val)
	}
	return strings.Join(parts, ";") + ";"
}

var bcdNo = map[string]bool{"no": true, "nein": true, "non": true, "não": true, "nao": true, "nie": true, "нет": true, "hayır": true, "ne": true, "nej": true, "ei": true, "nee": true, "off": true, "false": true}

func bcdMatches(cur string, had bool, want string) bool {
	if want == "" {
		return !had
	}
	if !had {
		return false
	}
	c := strings.ToLower(strings.TrimSpace(cur))
	w := strings.ToLower(want)
	if c == w {
		return true
	}
	if w == "yes" {
		return !bcdNo[c]
	}
	if w == "no" {
		return bcdNo[c]
	}
	return false
}
