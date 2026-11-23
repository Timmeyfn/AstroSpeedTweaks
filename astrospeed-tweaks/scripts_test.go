package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Parses every generated PowerShell script with the real PowerShell parser (if pwsh is available).
func TestPowerShellScriptsParse(t *testing.T) {
	pwsh := os.Getenv("SG_PWSH")
	if pwsh == "" {
		t.Skip("set SG_PWSH to a pwsh binary")
	}
	dir := t.TempDir()
	for name, s := range allScriptsForTest() {
		if err := os.WriteFile(filepath.Join(dir, name+".ps1"), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	check := `$bad=0; Get-ChildItem -Path '` + dir + `' -Filter *.ps1 | ForEach-Object { $t=$null; $e=$null; [void][System.Management.Automation.Language.Parser]::ParseFile($_.FullName,[ref]$t,[ref]$e); if($e.Count){ $bad++; Write-Output ("FAIL " + $_.Name + ": " + ($e | ForEach-Object { $_.Message + ' @' + $_.Extent.StartLineNumber }) -join '; ') } else { Write-Output ("ok " + $_.Name) } }; if($bad){ exit 1 }`
	out, err := exec.Command(pwsh, "-NoLogo", "-NoProfile", "-Command", check).CombinedOutput()
	t.Log(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("PowerShell parse errors: %v", err)
	}
}

func TestParseJSONList(t *testing.T) {
	var a []taskRec
	parseJSONList(`{"p":"\\x\\","n":"a","s":"Ready"}`, &a)
	if len(a) != 1 || a[0].N != "a" {
		t.Fatalf("object not wrapped: %+v", a)
	}
	var b []string
	parseJSONList("noise\r\n\"USB\\\\X\"", &b)
	if len(b) != 1 || b[0] != `USB\X` {
		t.Fatalf("string not wrapped: %+v", b)
	}
	var c []string
	parseJSONList(`["a","b"]`, &c)
	if len(c) != 2 {
		t.Fatal("array")
	}
}
