// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

// Fuzz targets double as robustness tests: their seed corpus runs on every
// `go test`, and `go test -fuzz` explores further. The contract for parsers is
// "never panic on arbitrary input"; for the renderer it is the stronger
// invariant that no input can break out of the inline <script>.

func FuzzVendorFromHomepage(f *testing.F) {
	for _, s := range []string{"", "https://github.com/a/b", "://x", "not a url", "https://x.y.z/p?q=1#f"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) { _ = vendorFromHomepage(s) })
}

func FuzzClassifyCategory(f *testing.F) {
	f.Add("name", "desc", "pip (Python)")
	f.Add("openssl@3", "tls library", "Homebrew (formula)")
	f.Fuzz(func(t *testing.T, name, desc, source string) {
		_ = classifyCategory(Component{Name: name, Desc: desc, Source: source})
	})
}

func FuzzParseBrew(f *testing.F) {
	f.Add(brewJSON)
	f.Add("{")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) { _ = parseBrew(s) })
}

func FuzzParsePip(f *testing.F) {
	f.Add(pipList, pipShow)
	f.Add("[]", "")
	f.Fuzz(func(t *testing.T, list, show string) {
		_ = parsePip(list, show, true)
		_ = parsePip(list, show, false)
	})
}

func FuzzParseWindowsRegistry(f *testing.F) {
	f.Add(`[{"DisplayName":"A"}]`)
	f.Add(`{"DisplayName":"B"}`)
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) { _ = parseWindowsRegistry(s) })
}

func FuzzParseGem(f *testing.F) {
	f.Add(gemDetails)
	f.Add("nokogiri (1)\n  Homepage: x\n")
	f.Fuzz(func(t *testing.T, s string) { _ = parseGem(s) })
}

// FuzzRenderInjectionSafety is the security guardrail: regardless of what a
// scanner reports (names from package metadata are attacker-influenceable in
// principle), the rendered page must never gain a second literal </script>.
func FuzzRenderInjectionSafety(f *testing.F) {
	f.Add("name", "desc", "vendor")
	f.Add("</script>", "<script>x</script>", "</SCRIPT>")
	f.Fuzz(func(t *testing.T, name, desc, vendor string) {
		comps := []Component{{Name: name, Desc: desc, Vendor: vendor, Source: "s", Category: "c"}}
		html := renderHTML(comps, nil, name, desc, vendor)
		if n := strings.Count(html, "</script>"); n != 1 {
			t.Fatalf("injection: %d literal </script> for name=%q desc=%q vendor=%q", n, name, desc, vendor)
		}
	})
}
