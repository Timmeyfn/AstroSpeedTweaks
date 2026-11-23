package main

import (
	"strconv"
	"strings"
)

func parseRepo(text string) string {
	for _, ln := range strings.Split(text, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		ln = strings.TrimPrefix(strings.TrimPrefix(ln, "https://github.com/"), "github.com/")
		ln = strings.Trim(ln, "/ ")
		if strings.Count(ln, "/") == 1 && !strings.ContainsAny(ln, " <>") {
			return ln
		}
	}
	return ""
}

// versionParts turns "v1.4.2" into [1 4 2].
func versionParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(strings.ToLower(v)), "v")
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	var out []int
	for _, p := range strings.Split(v, ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out
}

func newerVersion(latest, current string) bool {
	a, b := versionParts(latest), versionParts(current)
	for i := 0; i < len(a) || i < len(b); i++ {
		x, y := 0, 0
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}
