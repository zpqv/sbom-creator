// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

func TestPurlFor(t *testing.T) {
	cases := []struct {
		name, version, source, want string
	}{
		{"jq", "1.8.1", "Homebrew (formula)", "pkg:brew/jq@1.8.1"},
		{"Warp", "1.2", "Homebrew (cask)", "pkg:brew/Warp@1.2"},
		{"cli", "1.0", "npm (global)", "pkg:npm/cli@1.0"},
		{"@angular/core", "17.0", "npm (global)", "pkg:npm/%40angular/core@17.0"},
		{"Flask", "3.0", "pip (Python)", "pkg:pypi/flask@3.0"},
		{"typing_extensions", "4.1", "pip (Python)", "pkg:pypi/typing-extensions@4.1"},
		{"nokogiri", "1.13.8", "gem (Ruby)", "pkg:gem/nokogiri@1.13.8"},
		{"ripgrep", "14.0.0", "cargo (Rust)", "pkg:cargo/ripgrep@14.0.0"},
		{"vim", "2:9.1", "dpkg (Debian/Ubuntu)", "pkg:deb/vim@2:9.1"},
		{"zlib", "1.3-1", "rpm (Fedora/RHEL/SUSE)", "pkg:rpm/zlib@1.3-1"},
		// version with a space gets percent-encoded
		{"weird", "1 2", "gem (Ruby)", "pkg:gem/weird@1%202"},
		// no version -> no @
		{"bare", "", "gem (Ruby)", "pkg:gem/bare"},
		// non-package sources -> no purl
		{"Google Chrome", "120", "Direct install", ""},
		{"Solo", "1.0", "Mac App Store", ""},
		{"Acme", "1", "Windows (registry)", ""},
		// empty name -> no purl even for a package source
		{"", "1.0", "npm (global)", ""},
	}
	for _, c := range cases {
		got := purlFor(Component{Name: c.name, Version: c.version, Source: c.source})
		if got != c.want {
			t.Errorf("purlFor(%q,%q,%q) = %q, want %q", c.name, c.version, c.source, got, c.want)
		}
	}
}

func TestNormalizePyPI(t *testing.T) {
	cases := map[string]string{
		"Flask":               "flask",
		"typing_extensions":   "typing-extensions",
		"a.b_c--d":            "a-b-c-d",
		"--leading-trailing-": "leading-trailing",
		"ALLCAPS":             "allcaps",
	}
	for in, want := range cases {
		if got := normalizePyPI(in); got != want {
			t.Errorf("normalizePyPI(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPurlNamespaceName(t *testing.T) {
	// npm scoped vs unscoped
	if ns, n := purlNamespaceName("npm", "@scope/pkg"); ns != "@scope" || n != "pkg" {
		t.Errorf("scoped npm = %q,%q", ns, n)
	}
	if ns, n := purlNamespaceName("npm", "Plain"); ns != "" || n != "plain" {
		t.Errorf("unscoped npm = %q,%q", ns, n)
	}
	// a leading @ with no slash is not a valid scope -> treated as plain name
	if ns, n := purlNamespaceName("npm", "@weird"); ns != "" || n != "@weird" {
		t.Errorf("malformed scope = %q,%q", ns, n)
	}
	// default branch (e.g. gem) returns name unchanged
	if ns, n := purlNamespaceName("gem", "Rails"); ns != "" || n != "Rails" {
		t.Errorf("gem ns/name = %q,%q", ns, n)
	}
}
