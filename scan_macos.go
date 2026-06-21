// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// macOS GUI applications. We enumerate .app bundles in the standard locations
// and read each Info.plist via `plutil` (handles binary and XML plists, always
// present on macOS). Provenance:
//   - bundle under /System/Applications        -> "macOS (built-in)"
//   - bundle contains Contents/_MASReceipt     -> "Mac App Store"
//   - otherwise                                -> "Direct install"
// Brew casks are scanned separately; to avoid double-counting we skip apps
// whose names already came from a cask (handled by the caller via dedupe).

var bundleVendor = map[string]string{
	"com.apple": "Apple", "com.microsoft": "Microsoft", "com.google": "Google",
	"com.openai": "OpenAI", "com.anthropic": "Anthropic", "com.tinyspeck.slackmacgap": "Slack",
	"com.tailscale": "Tailscale", "company.thebrowser": "The Browser Company", "org.mozilla": "Mozilla",
	"com.brave": "Brave", "com.todesktop": "ToDesktop app", "io.warp": "Warp", "com.mitchellh.ghostty": "Ghostty",
	"org.libreoffice": "The Document Foundation", "com.docker": "Docker", "dev.orbstack": "OrbStack",
}

func vendorFromBundleID(id string) string {
	id = strings.ToLower(id)
	for prefix, v := range bundleVendor {
		if strings.HasPrefix(id, prefix) {
			return v
		}
	}
	// reverse-domain heuristic: com.acme.app -> acme
	parts := strings.Split(id, ".")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func plistValue(plistJSON map[string]any, key string) string {
	if v, ok := plistJSON[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

type appDir struct {
	path   string
	source string
}

// defaultMacAppDirs are the standard bundle locations scanned in production.
func defaultMacAppDirs() []appDir {
	home, _ := os.UserHomeDir()
	return []appDir{
		{"/Applications", "Direct install"},
		{filepath.Join(home, "Applications"), "Direct install"},
		{"/System/Applications", "macOS (built-in)"},
	}
}

func scanMacApps() []Component { return scanAppsIn(defaultMacAppDirs()) }

// scanAppsIn enumerates .app bundles across the given directories. Splitting it
// out from the hardcoded paths lets tests point it at a temporary bundle tree.
func scanAppsIn(dirs []appDir) []Component {
	var comps []Component
	seen := map[string]bool{}
	for _, d := range dirs {
		entries, err := os.ReadDir(d.path)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".app") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".app")
			if seen[name] {
				continue
			}
			seen[name] = true
			comps = append(comps, appComponent(filepath.Join(d.path, e.Name()), name, d.source))
		}
	}
	return comps
}

// plistReader parses an .app bundle's Info.plist into a dict. It is a package
// var so tests can supply canned data on any OS — the default implementation
// shells out to `plutil`, which only exists on macOS, and routing through this
// seam keeps the parsing branch testable (and coverage uniform) everywhere.
var plistReader = plutilPlist

// plutilPlist is the production plist reader: `plutil -convert json` handles
// both binary and XML plists and is always present on macOS.
func plutilPlist(infoPlist string) (map[string]any, bool) {
	out, ok := run("plutil", "-convert", "json", "-o", "-", infoPlist)
	if !ok {
		return nil, false
	}
	var pj map[string]any
	if json.Unmarshal([]byte(out), &pj) != nil {
		return nil, false
	}
	return pj, true
}

// appComponent reads one .app bundle's Info.plist and classifies it.
func appComponent(app, name, source string) Component {
	if _, err := os.Stat(filepath.Join(app, "Contents", "_MASReceipt", "receipt")); err == nil {
		source = "Mac App Store"
	}
	version, vendor := "", ""
	if pj, ok := plistReader(filepath.Join(app, "Contents", "Info.plist")); ok {
		version = firstNonEmpty(plistValue(pj, "CFBundleShortVersionString"), plistValue(pj, "CFBundleVersion"))
		vendor = vendorFromBundleID(plistValue(pj, "CFBundleIdentifier"))
	}
	if source == "macOS (built-in)" {
		vendor = "Apple"
	}
	c := Component{
		Name: name, Version: firstNonEmpty(version, "(bundled)"), Source: source,
		Vendor: firstNonEmpty(vendor, "Unknown"), Desc: descForApp(name),
	}
	// classifyCategory already maps the "macOS (built-in)" source to its own
	// category, so no post-hoc recategorization is needed here.
	c.Category = classifyCategory(c)
	return c
}
