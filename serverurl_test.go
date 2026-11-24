package main

import "testing"

func TestServerURLAndKeys(t *testing.T) {
	if got := parseServerURL("# note\n\n  https://x.workers.dev/ \n"); got != "https://x.workers.dev" {
		t.Fatalf("parseServerURL: %q", got)
	}
	if parseServerURL("# only comments\nhttp://insecure.example\n") != "" {
		t.Fatal("http should be refused")
	}
	if serverURL() == "" {
		t.Fatal("server-url.txt should hold the live server")
	}
	cases := map[string]string{
		"ASTRO-ABCDE-FGHIJ-KLMNO":    "ASTRO-ABCDE-FGHIJ-KLMNO",
		"astro abcde fghij klmno":    "ASTRO-ABCDE-FGHIJ-KLMNO",
		" astro-abcde-fghij-klmno\n": "ASTRO-ABCDE-FGHIJ-KLMNO",
	}
	for in, want := range cases {
		if got := normalizeKey(in); got != want {
			t.Errorf("normalizeKey(%q) = %q, want %q", in, got, want)
		}
	}
}
