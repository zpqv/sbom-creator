// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildInventoryDedupeAndReverseDeps(t *testing.T) {
	raw := []Component{
		{Name: "Warp", Source: "Homebrew (cask)", Category: "Terminals"},
		{Name: "Warp", Source: "Direct install", Category: "Terminals"}, // dup of cask -> dropped
		{Name: "node", Source: "Homebrew (formula)", Category: "Languages & Runtimes", Deps: []string{"icu4c", "icu4c"}},
		{Name: "icu4c", Source: "Homebrew (formula)", Category: "Internationalization"},
		// two same-category entries so the within-category name sort runs
		{Name: "Zebra", Source: "Direct install", Category: "Applications"},
		{Name: "Apple", Source: "Mac App Store", Category: "Applications"},
	}
	got := buildInventory(raw)
	// within "Applications", Apple must sort before Zebra
	var order []string
	for _, c := range got {
		if c.Category == "Applications" {
			order = append(order, c.Name)
		}
	}
	if len(order) != 2 || order[0] != "Apple" || order[1] != "Zebra" {
		t.Errorf("within-category sort wrong: %v", order)
	}

	warps := 0
	var icu Component
	for _, c := range got {
		if c.Name == "Warp" {
			warps++
		}
		if c.Name == "icu4c" {
			icu = c
		}
	}
	if warps != 1 {
		t.Errorf("Warp appears %d times, want 1 (dedupe failed)", warps)
	}
	if icu.UsedBy != 1 {
		t.Errorf("icu4c UsedBy = %d, want 1", icu.UsedBy)
	}
	// node.Deps must be de-duplicated to a single icu4c
	for _, c := range got {
		if c.Name == "node" && len(c.Deps) != 1 {
			t.Errorf("node deps = %v, want one entry", c.Deps)
		}
	}
	// sorted by category then name: verify non-decreasing category order
	for i := 1; i < len(got); i++ {
		if strings.ToLower(got[i-1].Category) > strings.ToLower(got[i].Category) {
			t.Errorf("not sorted by category at %d: %q then %q", i, got[i-1].Category, got[i].Category)
		}
	}
}

func TestBuildInventoryNoCasks(t *testing.T) {
	// No cask present -> the dedupe block is skipped (managed empty).
	raw := []Component{{Name: "Solo", Source: "Direct install", Category: "Applications"}}
	if got := buildInventory(raw); len(got) != 1 {
		t.Errorf("want 1 component, got %d", len(got))
	}
}

func TestBrowserCommand(t *testing.T) {
	cases := []struct {
		goos string
		want []string
	}{
		{"darwin", []string{"open", "/x"}},
		{"windows", []string{"rundll32", "url.dll,FileProtocolHandler", "/x"}},
		{"linux", []string{"xdg-open", "/x"}},
		{"freebsd", []string{"xdg-open", "/x"}}, // default branch
	}
	for _, c := range cases {
		got := browserCommand(c.goos, "/x").Args
		if strings.Join(got, " ") != strings.Join(c.want, " ") {
			t.Errorf("browserCommand(%q).Args = %v, want %v", c.goos, got, c.want)
		}
	}
}

func TestOpenInBrowser(t *testing.T) {
	// Fake every possible launcher so Start() spawns the harmless helper.
	fakeExec(t, map[string]fakeCmd{"open": ok(""), "xdg-open": ok(""), "rundll32": ok("")})
	openInBrowser("/tmp/whatever") // must not panic; error is intentionally ignored
}

func TestScannersRegistry(t *testing.T) {
	scs := scanners()
	if len(scs) == 0 {
		t.Fatal("no scanners registered")
	}
	// Calling each Probe covers the probe closures without depending on which
	// tools happen to be installed.
	for _, s := range scs {
		if s.Name == "" {
			t.Error("scanner with empty name")
		}
		if s.Probe != nil {
			_ = s.Probe()
		}
	}
}

// fakeScanners builds a controllable scanner set for realMain tests.
func fakeScanners() []Scanner {
	goos := runtime.GOOS
	return []Scanner{
		{ // available, contributes a cask + an app to exercise dedupe
			Name: "fake-available", Probe: func() bool { return true },
			Scan: func() []Component {
				return []Component{
					{Name: "Tool", Source: "Homebrew (cask)", Category: "Developer Tools"},
					{Name: "Tool", Source: "Direct install", Category: "Developer Tools"},
				}
			},
		},
		{ // missing -> drives the install-hint output
			Name: "fake-missing", Probe: func() bool { return false },
			Install: map[string]string{goos: "install me"}, Site: "https://example.test",
		},
		{ // no Probe -> scanned directly
			Name: "fake-noprobe",
			Scan: func() []Component { return []Component{{Name: "X", Source: "x", Category: "Applications"}} },
		},
		{ // irrelevant to this OS -> filtered out
			Name: "fake-other-os", OS: []string{"plan9"},
			Scan: func() []Component { return []Component{{Name: "never"}} },
		},
	}
}

func probeByName(scs []Scanner, name string) func() bool {
	for _, s := range scs {
		if s.Name == name {
			return s.Probe
		}
	}
	return nil
}

// TestScannerProbes asserts probe return values under controlled conditions so
// that mutating their boolean logic (==/!=, ||/&&) is caught — these closures
// were the only mutants surviving the suite otherwise.
func TestScannerProbes(t *testing.T) {
	scs := scanners()

	// macOS applications probe must agree with the host OS exactly.
	macProbe := probeByName(scs, "macOS applications")
	if macProbe == nil {
		t.Fatal("macOS applications scanner not found")
	}
	if macProbe() != (runtime.GOOS == "darwin") {
		t.Errorf("macOS probe = %v, want %v", macProbe(), runtime.GOOS == "darwin")
	}

	pipProbe := probeByName(scs, "pip (Python)")
	winProbe := probeByName(scs, "Windows installed apps (registry)")

	// Nothing present -> both probes false.
	fakeLookPath(t)
	if pipProbe() {
		t.Error("pip probe should be false when no python present")
	}
	if winProbe() {
		t.Error("windows registry probe should be false when no powershell present")
	}
	// Tools present -> both true (winProbe: exactly powershell -> || true; && would fail).
	fakeLookPath(t, "pip3", "powershell")
	if !pipProbe() {
		t.Error("pip probe should be true when pip3 present")
	}
	if !winProbe() {
		t.Error("windows probe should be true when powershell present")
	}
}

func TestRealMainSuccess(t *testing.T) {
	out := filepath.Join(t.TempDir(), "r.html")
	var buf bytes.Buffer
	code := realMain([]string{"-o", out, "-no-open", "-quiet"}, &buf, fakeScanners())
	if code != 0 {
		t.Fatalf("realMain returned %d, want 0", code)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("report not written: %v", err)
	}
}

func TestRealMainMissingToolOutput(t *testing.T) {
	out := filepath.Join(t.TempDir(), "r.html")
	var buf bytes.Buffer
	code := realMain([]string{"-o", out, "-no-open"}, &buf, fakeScanners())
	if code != 0 {
		t.Fatalf("realMain returned %d, want 0", code)
	}
	s := buf.String()
	if !strings.Contains(s, "Not installed") || !strings.Contains(s, "install me") {
		t.Errorf("missing-tool guidance not printed:\n%s", s)
	}
	if !strings.Contains(s, "fake-available") { // non-quiet progress line
		t.Error("expected progress output in non-quiet mode")
	}
}

func TestRealMainOpensBrowser(t *testing.T) {
	fakeExec(t, map[string]fakeCmd{"open": ok(""), "xdg-open": ok(""), "rundll32": ok("")})
	out := filepath.Join(t.TempDir(), "r.html")
	var buf bytes.Buffer
	if code := realMain([]string{"-o", out, "-quiet"}, &buf, fakeScanners()); code != 0 {
		t.Fatalf("realMain returned %d, want 0", code)
	}
}

func TestMainEntrypoint(t *testing.T) {
	oldArgs, oldExit := os.Args, osExit
	t.Cleanup(func() { os.Args, osExit = oldArgs, oldExit })
	var code int
	osExit = func(c int) { code = c }
	// A bad flag makes realMain return 2 immediately (no machine scan), so this
	// deterministically exercises the main() shim end to end.
	os.Args = []string{"sbom-creator", "-this-flag-does-not-exist"}
	main()
	if code != 2 {
		t.Errorf("main() exit code = %d, want 2", code)
	}
}

func TestRealMainVersion(t *testing.T) {
	var buf bytes.Buffer
	if code := realMain([]string{"-version"}, &buf, nil); code != 0 {
		t.Fatalf("-version returned %d, want 0", code)
	}
	if got := strings.TrimSpace(buf.String()); got != version {
		t.Errorf("-version printed %q, want %q", got, version)
	}
}

func TestRealMainFormatsToFile(t *testing.T) {
	dir := t.TempDir()
	// CycloneDX to an explicit file; no -no-open, but a non-HTML format must
	// never try to open a browser (covers the format=="html" guard on open).
	cdx := filepath.Join(dir, "bom.cdx.json")
	var buf bytes.Buffer
	if code := realMain([]string{"-format", "cyclonedx", "-o", cdx, "-quiet"}, &buf, fakeScanners()); code != 0 {
		t.Fatalf("cyclonedx returned %d", code)
	}
	b, err := os.ReadFile(cdx)
	if err != nil || !strings.Contains(string(b), "CycloneDX") {
		t.Errorf("cyclonedx file wrong: err=%v", err)
	}
}

func TestRealMainFormatsToStdout(t *testing.T) {
	// Non-HTML format with no -o streams to stdout (pipe-friendly) and emits
	// nothing but the document (logs suppressed).
	var buf bytes.Buffer
	if code := realMain([]string{"-format", "spdx"}, &buf, fakeScanners()); code != 0 {
		t.Fatalf("spdx returned %d", code)
	}
	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "{") || !strings.Contains(out, "SPDX-2.3") {
		t.Errorf("spdx stdout not clean document: %.40q", out)
	}
	if strings.Contains(out, "Inventoried") || strings.Contains(out, "scanning") {
		t.Error("progress logs leaked into stdout document")
	}

	// Explicit -o - also routes to stdout, for any format.
	var buf2 bytes.Buffer
	if code := realMain([]string{"-format", "json", "-o", "-"}, &buf2, fakeScanners()); code != 0 {
		t.Fatalf("json -o - returned %d", code)
	}
	var arr []Component
	if err := json.Unmarshal(buf2.Bytes(), &arr); err != nil {
		t.Errorf("json -o - not valid JSON array: %v", err)
	}
}

func TestRealMainUnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	if code := realMain([]string{"-format", "bogus"}, &buf, fakeScanners()); code != 2 {
		t.Errorf("unknown format returned %d, want 2", code)
	}
	if !strings.Contains(buf.String(), "unknown -format") {
		t.Errorf("expected error message, got %q", buf.String())
	}
}

func TestRealMainBadFlag(t *testing.T) {
	var buf bytes.Buffer
	if code := realMain([]string{"-nonsense"}, &buf, nil); code != 2 {
		t.Errorf("bad flag returned %d, want 2", code)
	}
}

func TestRealMainRenderError(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "missing-dir", "r.html")
	var buf bytes.Buffer
	code := realMain([]string{"-o", bad, "-no-open", "-quiet"}, &buf, fakeScanners())
	if code != 1 {
		t.Errorf("render error returned %d, want 1", code)
	}
	if !strings.Contains(buf.String(), "error writing report") {
		t.Errorf("expected error message, got: %s", buf.String())
	}
}
