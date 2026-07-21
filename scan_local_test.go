// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeHome / failHome override the userHomeDir seam so the filesystem scanners
// run against a temp dir (or a forced error) deterministically on every OS.
func fakeHome(t *testing.T, dir string) {
	t.Helper()
	orig := userHomeDir
	t.Cleanup(func() { userHomeDir = orig })
	userHomeDir = func() (string, error) { return dir, nil }
}

func failHome(t *testing.T) {
	t.Helper()
	orig := userHomeDir
	t.Cleanup(func() { userHomeDir = orig })
	userHomeDir = func() (string, error) { return "", os.ErrNotExist }
}

// writeUvMetadata drops a dist-info METADATA for tool `pkg` under a fake uv
// tools tree rooted at home, matching enrichUvTool's glob.
func writeUvMetadata(t *testing.T, home, tool, pkg, body string) {
	t.Helper()
	dir := filepath.Join(home, ".local", "share", "uv", "tools", tool, "lib", "python3.12", "site-packages", pkg+".dist-info")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "METADATA"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseUvToolList(t *testing.T) {
	out := "aider-chat v0.86.2\n- aider\nsweagent v1.1.0\n- sweagent\n"
	got := parseUvToolList(out)
	if len(got) != 2 {
		t.Fatalf("got %d components, want 2: %+v", len(got), got)
	}
	if got[0].Name != "aider-chat" || got[0].Version != "0.86.2" || got[0].Source != "uv (tool)" {
		t.Errorf("first = %+v", got[0])
	}
	if got[1].Name != "sweagent" || got[1].Version != "1.1.0" {
		t.Errorf("second = %+v", got[1])
	}
	// Non-tool lines produce nothing: "- exe", blank, indented, single-field,
	// and a second field that is not a "vX" version.
	if c := parseUvToolList("- aider\n\n   indented\nsolo\ntool notaversion\n"); len(c) != 0 {
		t.Errorf("expected no components from non-tool lines, got %+v", c)
	}
}

func TestScanUvTools(t *testing.T) {
	home := t.TempDir()
	fakeHome(t, home)
	writeUvMetadata(t, home, "aider-chat", "aider_chat-0.86.2",
		"Name: aider-chat\nSummary: Aider is AI pair programming in your terminal\n"+
			"Project-URL: Homepage, https://github.com/Aider-AI/aider\n\nbody\n")

	fakeExec(t, map[string]fakeCmd{"uv": ok("aider-chat v0.86.2\n- aider\n")})
	got := scanUvTools()
	if len(got) != 1 {
		t.Fatalf("scanUvTools = %d comps, want 1", len(got))
	}
	if got[0].Category != "AI / LLM Tools" {
		t.Errorf("category = %q", got[0].Category)
	}
	if got[0].Desc == "" || got[0].Homepage == "" {
		t.Errorf("metadata not enriched: %+v", got[0])
	}

	// `uv` errors -> nil.
	fakeExec(t, map[string]fakeCmd{})
	if scanUvTools() != nil {
		t.Error("scanUvTools failure should be nil")
	}
}

func TestEnrichUvTool(t *testing.T) {
	// home lookup error -> no-op.
	failHome(t)
	c := Component{Name: "x"}
	enrichUvTool(&c)
	if c.Desc != "" {
		t.Error("expected no enrichment when home errors")
	}

	// no matching dist-info -> no-op.
	home := t.TempDir()
	fakeHome(t, home)
	c = Component{Name: "ghost"}
	enrichUvTool(&c)
	if c.Desc != "" {
		t.Error("expected no enrichment when METADATA absent")
	}

	// METADATA path exists but is unreadable (a directory) -> no-op, no panic.
	badMeta := filepath.Join(home, ".local", "share", "uv", "tools", "dirtool", "lib", "py", "site-packages", "dirtool-1.0.dist-info", "METADATA")
	if err := os.MkdirAll(badMeta, 0o755); err != nil { // METADATA itself is a dir
		t.Fatal(err)
	}
	c = Component{Name: "dirtool"}
	enrichUvTool(&c)
	if c.Desc != "" {
		t.Error("expected no enrichment when METADATA is unreadable")
	}
}

// fakeResolve overrides the resolveSymlink seam so a path's "resolved" target
// is deterministic on every OS (no real symlinks, which are privileged on
// Windows). Restored on cleanup.
func fakeResolve(t *testing.T, fn func(string) string) {
	t.Helper()
	orig := resolveSymlink
	t.Cleanup(func() { resolveSymlink = orig })
	resolveSymlink = fn
}

func TestResolveSymlink(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "real")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Existing path resolves to something that still exists.
	if _, err := os.Stat(resolveSymlink(f)); err != nil {
		t.Errorf("resolveSymlink(existing) -> unstatable: %v", err)
	}
	// Nonexistent path: EvalSymlinks errors, so the input is returned unchanged.
	ghost := filepath.Join(tmp, "ghost")
	if got := resolveSymlink(ghost); got != ghost {
		t.Errorf("resolveSymlink(missing) = %q, want %q", got, ghost)
	}
}

func TestScanPathApps(t *testing.T) {
	home := t.TempDir()
	fakeHome(t, home)
	localBin := filepath.Join(home, ".local", "bin")
	ocBin := filepath.Join(home, ".opencode", "bin")
	for _, d := range []string{localBin, ocBin} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeExe := func(p string) {
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Curated: droid, amp, kavith, opencode. Plus a non-curated file and a
	// duplicate droid (in ocBin) to exercise the "seen" and "!ok" skips. No real
	// symlinks — resolveSymlink is stubbed below.
	for _, n := range []string{"droid", "amp", "kavith", "opencode", "randomtool"} {
		writeExe(filepath.Join(localBin, n))
	}
	writeExe(filepath.Join(ocBin, "droid"))

	// opencode "resolves" into a uv/tools tree -> pmOwned -> skipped. Others
	// resolve to themselves. Uses a forward-slash marker to prove pmOwned is
	// separator-agnostic even when the rest of the path is OS-native.
	fakeResolve(t, func(p string) string {
		if strings.HasSuffix(p, "opencode") {
			return "/somewhere/uv/tools/opencode/bin/opencode"
		}
		return p
	})

	// --version succeeds for everything except kavith (exercises the empty path).
	fakeExecFunc(t, func(name string, _ []string) fakeCmd {
		if strings.HasSuffix(name, "kavith") {
			return fakeCmd{exit: 1}
		}
		return ok("1.2.3\n")
	})

	got := scanPathApps()
	byName := map[string]Component{}
	for _, c := range got {
		byName[c.Name] = c
	}
	if len(got) != 3 {
		t.Fatalf("scanPathApps = %d comps, want 3 (droid, amp, kavith): %v", len(got), byName)
	}
	if _, ok := byName["OpenCode"]; ok {
		t.Error("uv-owned opencode symlink should have been skipped")
	}
	if byName["Droid"].Version != "1.2.3" {
		t.Errorf("droid version = %q, want 1.2.3", byName["Droid"].Version)
	}
	if byName["kavith"].Version != "" {
		t.Errorf("kavith version = %q, want empty (failed --version)", byName["kavith"].Version)
	}
	if byName["Droid"].Category != "AI / LLM Tools" {
		t.Errorf("droid category = %q", byName["Droid"].Category)
	}

	// home lookup error -> nil.
	failHome(t)
	if scanPathApps() != nil {
		t.Error("scanPathApps should be nil when home errors")
	}
}

func TestApplyPyMetadata(t *testing.T) {
	meta := "Metadata-Version: 2.4\n" +
		"Name: aider-chat\n" +
		"Summary: Aider is AI pair programming in your terminal\n" +
		"License: Apache-2.0\n" +
		"Project-URL: Homepage, https://github.com/Aider-AI/aider\n" +
		"\n" +
		"Summary: this line is body text and must be ignored\n"
	var c Component
	applyPyMetadata(&c, meta)
	if c.Desc != "Aider is AI pair programming in your terminal" {
		t.Errorf("desc = %q", c.Desc)
	}
	if c.Homepage != "https://github.com/Aider-AI/aider" {
		t.Errorf("homepage = %q", c.Homepage)
	}
	if c.License != "Apache-2.0" {
		t.Errorf("license = %q", c.License)
	}

	// Legacy Home-page header populates homepage; UNKNOWN license is dropped;
	// a non-Homepage Project-URL is ignored.
	var c2 Component
	applyPyMetadata(&c2, "Home-page: https://example.org\nLicense: UNKNOWN\nProject-URL: Funding, https://x\n\n")
	if c2.Homepage != "https://example.org" || c2.License != "" {
		t.Errorf("legacy parse = %+v", c2)
	}
}

func TestParseVersionToken(t *testing.T) {
	cases := []struct{ in, want string }{
		{"droid 0.170.0", "0.170.0"},
		{"2026.07.16-899851b", "2026.07.16-899851b"},
		{"v1.2.3", "1.2.3"},
		{"GitHub Copilot CLI 1.0.34.", "1.0.34"},
		{"0.0.1781305812-g78bbf9", "0.0.1781305812-g78bbf9"},
		{"no version here", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := parseVersionToken(c.in); got != c.want {
			t.Errorf("parseVersionToken(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPmOwned(t *testing.T) {
	if !pmOwned("/Users/x/.local/share/uv/tools/aider-chat/bin/aider") {
		t.Error("uv tool shim should be pm-owned")
	}
	// Backslash (Windows-style) path must still match after ToSlash.
	if !pmOwned(`C:\Users\x\uv\tools\aider-chat\bin\aider.exe`) {
		t.Error("windows-style uv tool path should be pm-owned")
	}
	if !pmOwned("/Users/x/.local/share/uv/python/cpython-3.12/bin/python3.12") {
		t.Error("uv python shim should be pm-owned")
	}
	if pmOwned("/Users/x/.amp/bin/amp") {
		t.Error("standalone installer path should not be pm-owned")
	}
	if pmOwned("/Users/x/.bun/bin/omp") {
		t.Error("bun bin should not be pm-owned")
	}
}
