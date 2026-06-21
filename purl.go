// SPDX-License-Identifier: Apache-2.0

package main

import (
	"net/url"
	"strings"
)

// purlType maps an inventory source to its Package URL type (purl-spec). Only
// sources backed by a real package ecosystem get a purl; GUI apps, OS registry
// entries and the like are not addressable as packages, so they get none.
var purlType = map[string]string{
	"Homebrew (formula)":     "brew",
	"Homebrew (cask)":        "brew",
	"npm (global)":           "npm",
	"pip (Python)":           "pypi",
	"gem (Ruby)":             "gem",
	"cargo (Rust)":           "cargo",
	"dpkg (Debian/Ubuntu)":   "deb",
	"rpm (Fedora/RHEL/SUSE)": "rpm",
}

// purlFor builds a Package URL for a component, or "" when its source is not a
// package ecosystem or it lacks a version. The format is
// pkg:<type>/<namespace>/<name>@<version>, per github.com/package-url/purl-spec.
func purlFor(c Component) string {
	typ, ok := purlType[c.Source]
	if !ok || c.Name == "" {
		return ""
	}
	ns, name := purlNamespaceName(typ, c.Name)
	p := "pkg:" + typ + "/"
	if ns != "" {
		p += purlEscape(ns) + "/"
	}
	p += purlEscape(name)
	if v := strings.TrimSpace(c.Version); v != "" {
		p += "@" + purlEscape(v)
	}
	return p
}

// purlNamespaceName splits an ecosystem-specific name into (namespace, name)
// and normalizes it per that ecosystem's purl rules.
func purlNamespaceName(typ, raw string) (ns, name string) {
	switch typ {
	case "npm":
		// Scoped package "@scope/pkg" -> namespace "@scope", name "pkg".
		// The purl spec keeps the leading "@" in the namespace and lowercases.
		low := strings.ToLower(raw)
		if strings.HasPrefix(low, "@") {
			if i := strings.Index(low, "/"); i > 0 {
				return low[:i], low[i+1:]
			}
		}
		return "", low
	case "pypi":
		// PyPI normalizes: lowercase, runs of [-_.] collapse to a single "-".
		return "", normalizePyPI(raw)
	default:
		return "", raw
	}
}

// normalizePyPI applies PEP 503 name normalization.
func normalizePyPI(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevSep := false
	for _, r := range s {
		if r == '-' || r == '_' || r == '.' {
			if !prevSep {
				b.WriteByte('-')
				prevSep = true
			}
			continue
		}
		b.WriteRune(r)
		prevSep = false
	}
	return strings.Trim(b.String(), "-")
}

// purlEscape percent-encodes a purl segment. Spaces and most punctuation are
// encoded; "@" inside a version is handled by the caller, not here.
func purlEscape(s string) string {
	// url.PathEscape leaves "@" and ":" unescaped but encodes spaces and most
	// reserved characters, which is sufficient for purl segments.
	return strings.ReplaceAll(url.PathEscape(s), "@", "%40")
}
