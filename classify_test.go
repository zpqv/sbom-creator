// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestClassifyCategory(t *testing.T) {
	cases := []struct {
		name, desc, source, want string
	}{
		// 1. exact name override wins over everything
		{"Google Chrome", "", "Direct install", "Browsers"},
		{"claude-code", "irrelevant", "npm (global)", "AI / LLM Tools"},
		// 2. formula map (only when source is a brew formula)
		{"openssl@3", "", "Homebrew (formula)", "Cryptography & Security"},
		{"libpng", "", "Homebrew (formula)", "Image & Graphics"},
		// name "git" is in nameCat -> Version Control regardless of source
		{"git", "", "Homebrew (formula)", "Version Control"},
		// 3. keyword heuristics over name+desc (each rule exercised)
		{"libfoo", "a TLS and x509 certificate library", "X", "Cryptography & Security"},
		{"squish", "generic-purpose lossless compression", "X", "Compression"},
		{"vidthing", "an AV1 video codec encoder", "X", "Media & Codecs"},
		{"textthing", "OpenType font glyph shaping", "X", "Fonts & Text"},
		{"imgthing", "PNG and JPEG image manipulation", "X", "Image & Graphics"},
		{"i18nthing", "Unicode locale and localization data", "X", "Internationalization"},
		{"netthing", "an HTTP and DNS client over TCP", "X", "Networking & Protocols"},
		{"dbthing", "an embedded SQL database engine", "X", "Databases"},
		{"buildthing", "a C compiler and linker", "X", "Developer Tools"},
		{"cloudthing", "deploy to kubernetes on aws", "X", "Cloud & DevOps CLI"},
		{"langthing", "a programming language runtime", "X", "Languages & Runtimes"},
		{"aithing", "an ai assistant coding agent (LLM)", "X", "AI / LLM Tools"},
		// 4. source-based defaults (no name/keyword hit)
		{"somepylib", "", "pip (Python)", "Python Library"},
		{"somegem", "", "gem (Ruby)", "Ruby Library"},
		{"somecrate", "", "cargo (Rust)", "Developer Tools"},
		{"someformula", "", "Homebrew (formula)", "Developer Libraries"},
		{"Some Built-in", "", "macOS (built-in)", "macOS (built-in)"},
		// 5. final fallback
		{"MysteryApp", "", "Direct install", "Applications"},
	}
	for _, c := range cases {
		got := classifyCategory(Component{Name: c.name, Desc: c.desc, Source: c.source})
		if got != c.want {
			t.Errorf("classifyCategory(%q,%q,%q) = %q, want %q", c.name, c.desc, c.source, got, c.want)
		}
	}
}

func TestVendorFromHomepage(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"://bad url with space", ""}, // unparseable / no host
		{"not-a-url", ""},             // no host
		{"https://github.com/google/brotli", "Google"},              // override substring before gh-org
		{"https://github.com/BurntSushi/ripgrep", "BurntSushi"},     // github org fallback
		{"https://gitlab.com/AOMediaCodec/SVT-AV1", "AOMediaCodec"}, // org case preserved
		{"https://ollama.com/", "Ollama"},                           // non-github override
		{"https://www.example.co.uk/path", "co.uk"},                 // two-label domain fallback
		{"https://localhost", "localhost"},                          // single-label host
		{"https://nodejs.org/", "Node.js (OpenJS Foundation)"},
	}
	for _, c := range cases {
		if got := vendorFromHomepage(c.in); got != c.want {
			t.Errorf("vendorFromHomepage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestVendorFromBundleID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"com.apple.Safari", "Apple"},    // prefix map
		{"com.acme.WidgetPro", "acme"},   // reverse-domain heuristic
		{"singlelabel", ""},              // no dot -> empty
		{"", ""},                         // empty
		{"io.warp.WarpTerminal", "Warp"}, // prefix map (io.warp)
	}
	for _, c := range cases {
		if got := vendorFromBundleID(c.in); got != c.want {
			t.Errorf("vendorFromBundleID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDescForApp(t *testing.T) {
	if got := descForApp("Safari"); got == "" {
		t.Error("expected a description for Safari")
	}
	if got := descForApp("Totally Unknown App"); got != "" {
		t.Errorf("expected empty desc for unknown app, got %q", got)
	}
	// case-insensitive
	if descForApp("safari") != descForApp("Safari") {
		t.Error("descForApp should be case-insensitive")
	}
}
