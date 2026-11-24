package main

import (
	_ "embed"
	"strings"
)

// server-url.txt holds the one address the app talks to for keys, updates, live tweaks and the shop.
//
//go:embed server-url.txt
var serverURLFile string

func serverURL() string { return parseServerURL(serverURLFile) }

func parseServerURL(text string) string {
	for _, ln := range strings.Split(text, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		if strings.HasPrefix(ln, "https://") && !strings.ContainsAny(ln, " <>\"'") {
			return strings.TrimRight(ln, "/")
		}
	}
	return ""
}

// normalizeKey turns "astro abcde-fghij klmno pqrst" into "ASTRO-ABCDE-FGHIJ-KLMNO-PQRST" style groups of 5.
func normalizeKey(k string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(k) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	s := b.String()
	var out []string
	for len(s) > 5 {
		out = append(out, s[:5])
		s = s[5:]
	}
	if s != "" {
		out = append(out, s)
	}
	return strings.Join(out, "-")
}
