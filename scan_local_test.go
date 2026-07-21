// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestParseUvToolList(t *testing.T) {
	// Real `uv tool list` shape: flush-left "name vX.Y", executables indented.
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
	// Executable ("- ...") and blank lines must not become components.
	if c := parseUvToolList("- aider\n\n   indented\n"); len(c) != 0 {
		t.Errorf("expected no components from non-tool lines, got %+v", c)
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

	// Legacy Home-page header populates homepage; UNKNOWN license is dropped.
	var c2 Component
	applyPyMetadata(&c2, "Home-page: https://example.org\nLicense: UNKNOWN\n\n")
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
	if pmOwned("/Users/x/.amp/bin/amp") {
		t.Error("standalone installer path should not be pm-owned")
	}
	// A bun-global binary must NOT be treated as pm-owned (we inventory it here).
	if pmOwned("/Users/x/.bun/bin/omp") {
		t.Error("bun bin should not be pm-owned")
	}
}
