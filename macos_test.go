// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlutilPlist covers the production plist reader on every OS by mocking the
// `plutil` invocation: valid JSON, a failed command, and unparseable output.
func TestPlutilPlist(t *testing.T) {
	fakeExec(t, map[string]fakeCmd{"plutil": ok(`{"CFBundleShortVersionString":"9.9","CFBundleIdentifier":"com.x.Y"}`)})
	pj, okk := plutilPlist("/whatever/Info.plist")
	if !okk || plistValue(pj, "CFBundleShortVersionString") != "9.9" {
		t.Fatalf("plutil success path wrong: %v %v", pj, okk)
	}
	fakeExec(t, map[string]fakeCmd{}) // plutil missing -> command fails
	if _, okk := plutilPlist("/x"); okk {
		t.Error("plutil failure should be ok=false")
	}
	fakeExec(t, map[string]fakeCmd{"plutil": ok("not json")}) // ran, bad output
	if _, okk := plutilPlist("/x"); okk {
		t.Error("invalid plist JSON should be ok=false")
	}
}

func TestPlistValue(t *testing.T) {
	m := map[string]any{"s": "hello", "n": 42}
	if plistValue(m, "s") != "hello" {
		t.Error("string value not returned")
	}
	if plistValue(m, "n") != "" { // non-string -> empty
		t.Error("non-string should yield empty")
	}
	if plistValue(m, "missing") != "" {
		t.Error("missing key should yield empty")
	}
}

// makeApp creates a minimal .app bundle with an XML Info.plist (plutil reads
// XML on every platform that has plutil; the dir-walking logic runs anywhere).
func makeApp(t *testing.T, base, name, bundleID, version string, appStore bool) {
	t.Helper()
	contents := filepath.Join(base, name+".app", "Contents")
	if err := os.MkdirAll(contents, 0o755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>` + bundleID + `</string>
<key>CFBundleShortVersionString</key><string>` + version + `</string>
</dict></plist>`
	os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(plist), 0o644)
	if appStore {
		receipt := filepath.Join(contents, "_MASReceipt")
		os.MkdirAll(receipt, 0o755)
		os.WriteFile(filepath.Join(receipt, "receipt"), []byte("x"), 0o644)
	}
}

func TestScanAppsIn(t *testing.T) {
	direct := t.TempDir()
	system := t.TempDir()

	makeApp(t, direct, "WidgetPro", "com.acme.WidgetPro", "3.1", false) // Direct install
	makeApp(t, direct, "StoreApp", "com.vendor.StoreApp", "1.0", true)  // Mac App Store
	makeApp(t, system, "Freeform", "com.apple.freeform", "2.0", false)  // built-in -> Apple
	// noise: a non-.app entry, and a duplicate name in the system dir
	os.WriteFile(filepath.Join(direct, "notanapp.txt"), []byte("x"), 0o644)
	makeApp(t, system, "WidgetPro", "com.acme.WidgetPro", "9.9", false) // duplicate -> skipped
	// an app with no readable plist -> version/vendor fall back to placeholders
	os.MkdirAll(filepath.Join(direct, "Broken.app", "Contents"), 0o755)

	// Override the plist reader so the parse branch runs deterministically on
	// every OS (the real plutil exists only on macOS). Keyed by bundle path.
	orig := plistReader
	t.Cleanup(func() { plistReader = orig })
	plistReader = func(p string) (map[string]any, bool) {
		switch {
		case strings.Contains(p, "WidgetPro"):
			return map[string]any{"CFBundleShortVersionString": "3.1", "CFBundleIdentifier": "com.acme.WidgetPro"}, true
		case strings.Contains(p, "StoreApp"):
			return map[string]any{"CFBundleShortVersionString": "1.0", "CFBundleIdentifier": "com.vendor.StoreApp"}, true
		case strings.Contains(p, "Freeform"):
			return map[string]any{"CFBundleShortVersionString": "2.0", "CFBundleIdentifier": "com.apple.freeform"}, true
		default:
			return nil, false // Broken.app
		}
	}

	dirs := []appDir{{direct, "Direct install"}, {system, "macOS (built-in)"}}
	comps := scanAppsIn(dirs)
	m := byName(comps)

	if w := m["WidgetPro"]; w.Version != "3.1" || w.Vendor != "acme" {
		t.Errorf("WidgetPro plist parse wrong: %+v", w)
	}
	if m["StoreApp"].Source != "Mac App Store" { // detected via _MASReceipt on disk
		t.Errorf("StoreApp source = %q, want Mac App Store", m["StoreApp"].Source)
	}
	if m["Freeform"].Vendor != "Apple" { // built-in source overrides plist vendor
		t.Errorf("built-in vendor = %q, want Apple", m["Freeform"].Vendor)
	}
	if m["Broken"].Version != "(bundled)" || m["Broken"].Vendor != "Unknown" {
		t.Errorf("Broken app should have placeholder version/vendor: %+v", m["Broken"])
	}
	// duplicate WidgetPro from system dir must not double-count
	count := 0
	for _, c := range comps {
		if c.Name == "WidgetPro" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("WidgetPro counted %d times, want 1", count)
	}

	// missing directory is skipped silently
	if got := scanAppsIn([]appDir{{filepath.Join(direct, "nope"), "Direct install"}}); got != nil {
		t.Errorf("missing dir should yield nil, got %+v", got)
	}
}

func TestScanMacAppsWrapper(t *testing.T) {
	// Exercise the production entry point and default dirs. On non-macOS this
	// simply returns whatever the (mostly absent) dirs hold; we assert no panic.
	_ = scanMacApps()
	if defaultMacAppDirs() == nil {
		t.Error("defaultMacAppDirs returned nil")
	}
}
