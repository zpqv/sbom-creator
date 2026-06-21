// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

var dataRe = regexp.MustCompile(`(?s)const DATA = (\[.*?\]);\nconst MISSING`)

func TestRenderRoundTrip(t *testing.T) {
	comps := []Component{
		{Name: "jq", Version: "1.8", Category: "Developer Tools", Source: "Homebrew (formula)", Vendor: "jq", Desc: "JSON processor", License: "MIT", PURL: "pkg:brew/jq@1.8", Deps: []string{"oniguruma"}, UsedBy: 0},
	}
	miss := []MissingTool{{Name: "cargo (Rust)", Install: "brew install rust", Site: "https://rustup.rs"}}

	html := renderHTML(comps, miss, "host1", "macOS 15 (arm64)", "2026-06-20 10:00")

	for _, ph := range []string{"__DATA__", "__MISSING__", "__TOTAL__", "__HOST__", "__OSLABEL__", "__DATE__"} {
		if strings.Contains(html, ph) {
			t.Errorf("placeholder %s was not substituted", ph)
		}
	}
	if !strings.Contains(html, "host1") || !strings.Contains(html, "macOS 15 (arm64)") {
		t.Error("header values missing from output")
	}
	if !strings.Contains(html, "brew install rust") {
		t.Error("missing-tool install hint not rendered")
	}
	m := dataRe.FindStringSubmatch(html)
	if m == nil {
		t.Fatal("could not locate embedded DATA array")
	}
	var got []Component
	if err := json.Unmarshal([]byte(m[1]), &got); err != nil {
		t.Fatalf("embedded JSON does not parse: %v", err)
	}
	if len(got) != 1 || got[0].Name != "jq" || got[0].License != "MIT" || got[0].PURL != "pkg:brew/jq@1.8" {
		t.Errorf("round-tripped data wrong: %+v", got)
	}
}

// Injection safety: malicious component fields must not break out of the inline
// <script>. Go's json encoder escapes <, > and & to \u00xx, so the literal
// "</script>" must appear exactly once (the template's own closing tag).
func TestRenderInjectionSafety(t *testing.T) {
	comps := []Component{{
		Name:   `evil</script><script>alert(1)</script>`,
		Desc:   `<img src=x onerror=alert(1)> & "quote"`,
		Vendor: `</SCRIPT >`,
		Source: "Direct install", Category: "Applications",
	}}
	html := renderHTML(comps, nil, `host</script>`, `os</script>`, `date</script>`)
	if n := strings.Count(html, "</script>"); n != 1 {
		t.Errorf("found %d literal </script> in output, want exactly 1 (injection!)", n)
	}
	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Error("unescaped injected <script> survived into output")
	}
}

func TestItoa(t *testing.T) {
	cases := map[int]string{0: "0", 7: "7", 42: "42", -1: "-1", -256: "-256", 1000: "1000"}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Errorf("itoa(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestHTMLEscape(t *testing.T) {
	if got := htmlEscape(`a&b<c>d"e`); got != "a&amp;b&lt;c&gt;d&quot;e" {
		t.Errorf("htmlEscape = %q", got)
	}
}
