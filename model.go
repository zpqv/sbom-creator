// SPDX-License-Identifier: Apache-2.0

package main

// Component is one inventoried piece of software. The json tags match the
// field names the embedded webpage's JavaScript expects, so the same struct
// both drives Go logic and serializes straight into the page.
type Component struct {
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Category string   `json:"category"`
	Source   string   `json:"source"`
	Vendor   string   `json:"vendor"`
	Desc     string   `json:"desc"`
	Homepage string   `json:"homepage"`
	Deps     []string `json:"deps"`
	UsedBy   int      `json:"usedBy"` // reverse-dependency count, computed after all scans
}

// MissingTool records a scanner whose backing package manager is relevant to
// the current OS but is not installed. We surface these to the user with an
// install hint rather than failing — "ask the user to install what's required".
type MissingTool struct {
	Name    string `json:"name"`
	Install string `json:"install"`
	Site    string `json:"site"`
}

// Scanner describes one source of installed software. Everything is gated at
// runtime by GOOS (no build tags) so a single binary cross-compiles cleanly
// and every scanner function compiles on every platform.
type Scanner struct {
	Name    string             // human label, e.g. "Homebrew"
	OS      []string           // GOOS values where relevant; empty = all
	Install map[string]string  // per-OS install command/hint
	Site    string             // homepage for the tool, shown with the hint
	Probe   func() bool        // reports whether the backing tool is present
	Scan    func() []Component // performs the scan; returns nil on any failure
}
