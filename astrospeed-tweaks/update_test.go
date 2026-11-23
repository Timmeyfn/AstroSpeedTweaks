package main

import "testing"

func TestUpdateHelpers(t *testing.T) {
	if !newerVersion("v1.4.0", "1.3.2") || newerVersion("1.3.2", "1.3.2") || newerVersion("v1.3.1", "1.3.2") || !newerVersion("2.0", "1.9.9") || !newerVersion("1.3.10", "1.3.9") {
		t.Fatal("version compare")
	}
	if parseRepo("# comment\n\n  https://github.com/tommy/astro-speed/ \n") != "tommy/astro-speed" {
		t.Fatal("repo parse")
	}
	if parseRepo("# only comments\n") != "" || parseRepo("not a repo") != "" {
		t.Fatal("empty repo")
	}
}
