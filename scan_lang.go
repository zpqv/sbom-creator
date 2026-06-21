// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// npm globals — read each package's local package.json for desc/homepage/deps
// (fully offline; the registry is never contacted).
// ---------------------------------------------------------------------------

func scanNpm() []Component {
	root, ok := run("npm", "root", "-g")
	if !ok {
		return nil
	}
	return parseNpmRoot(strings.TrimSpace(root))
}

// parseNpmRoot walks a global node_modules directory and reads each package's
// package.json. It handles scoped (@scope/name) packages and skips .bin.
func parseNpmRoot(root string) []Component {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var comps []Component
	add := func(dir string) {
		c, ok := npmComponent(dir)
		if ok {
			comps = append(comps, c)
		}
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), "@") { // scoped: one level deeper
			subs, _ := os.ReadDir(filepath.Join(root, e.Name()))
			for _, s := range subs {
				if s.IsDir() {
					add(filepath.Join(root, e.Name(), s.Name()))
				}
			}
			continue
		}
		if e.Name() == ".bin" {
			continue
		}
		add(filepath.Join(root, e.Name()))
	}
	return comps
}

// npmComponent builds a Component from a single package directory's
// package.json, returning ok=false when the file is missing or unparseable.
func npmComponent(dir string) (Component, bool) {
	b, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return Component{}, false
	}
	var p struct {
		Name         string            `json:"name"`
		Version      string            `json:"version"`
		Description  string            `json:"description"`
		Homepage     string            `json:"homepage"`
		Dependencies map[string]string `json:"dependencies"`
	}
	if json.Unmarshal(b, &p) != nil || p.Name == "" {
		return Component{}, false
	}
	var deps []string
	for d := range p.Dependencies {
		deps = append(deps, d)
	}
	c := Component{
		Name: p.Name, Version: p.Version, Source: "npm (global)",
		Desc: p.Description, Homepage: p.Homepage, Deps: deps,
		Vendor: firstNonEmpty(vendorFromHomepage(p.Homepage), "npm / community"),
	}
	c.Category = classifyCategory(c)
	return c, true
}

// ---------------------------------------------------------------------------
// pip — `pip list` for the set, `pip show` for summary/homepage/requires.
// Both read local installed metadata only.
// ---------------------------------------------------------------------------

func pipBin() string {
	for _, b := range []string{"pip3", "pip"} {
		if have(b) {
			return b
		}
	}
	return ""
}

func scanPip() []Component {
	bin := pipBin()
	if bin == "" {
		return nil
	}
	listOut, ok := run(bin, "list", "--format=json")
	if !ok {
		return nil
	}
	names, ok := pipNames(listOut)
	if !ok {
		return nil
	}
	showRaw, showOK := runIn(60*time.Second, bin, append([]string{"show"}, names...)...)
	return parsePip(listOut, showRaw, showOK)
}

// pipNames extracts the package names from `pip list --format=json` output,
// returning ok=false when the JSON is invalid or empty.
func pipNames(listJSON string) ([]string, bool) {
	var list []struct {
		Name string `json:"name"`
	}
	if json.Unmarshal([]byte(listJSON), &list) != nil || len(list) == 0 {
		return nil, false
	}
	var names []string
	for _, p := range list {
		names = append(names, p.Name)
	}
	return names, true
}

// parsePip combines `pip list` (versions) with `pip show` (summary, homepage,
// requires). When show output is unavailable it falls back to name/version.
func parsePip(listJSON, showRaw string, showOK bool) []Component {
	var list []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if json.Unmarshal([]byte(listJSON), &list) != nil {
		return nil
	}
	verOf := map[string]string{}
	for _, p := range list {
		verOf[strings.ToLower(p.Name)] = p.Version
	}
	if !showOK {
		var comps []Component
		for _, p := range list {
			c := Component{Name: p.Name, Version: p.Version, Source: "pip (Python)", Vendor: "PyPI / community"}
			c.Category = classifyCategory(c)
			comps = append(comps, c)
		}
		return comps
	}
	var comps []Component
	for _, block := range strings.Split(showRaw, "---") {
		var name, summary, home, requires string
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "Name:"):
				name = strings.TrimSpace(line[5:])
			case strings.HasPrefix(line, "Summary:"):
				summary = strings.TrimSpace(line[8:])
			case strings.HasPrefix(line, "Home-page:"):
				home = strings.TrimSpace(line[10:])
			case strings.HasPrefix(line, "Requires:"):
				requires = strings.TrimSpace(line[9:])
			}
		}
		if name == "" {
			continue
		}
		var deps []string
		for _, d := range strings.Split(requires, ",") {
			if t := strings.TrimSpace(d); t != "" {
				deps = append(deps, t)
			}
		}
		if summary == "UNKNOWN" {
			summary = ""
		}
		c := Component{
			Name: name, Version: verOf[strings.ToLower(name)], Source: "pip (Python)",
			Desc: summary, Homepage: home, Deps: deps,
			Vendor: firstNonEmpty(vendorFromHomepage(home), "PyPI / community"),
		}
		c.Category = classifyCategory(c)
		comps = append(comps, c)
	}
	return comps
}

// ---------------------------------------------------------------------------
// Ruby gems — `gem list -d` prints homepage + summary inline (local specs).
// ---------------------------------------------------------------------------

func scanGem() []Component {
	out, ok := runIn(40*time.Second, "gem", "list", "--local", "--details")
	if !ok {
		return nil
	}
	return parseGem(out)
}

func parseGem(out string) []Component {
	var comps []Component
	var cur *Component
	flush := func() {
		if cur != nil && cur.Name != "" {
			cur.Category = classifyCategory(*cur)
			comps = append(comps, *cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		// A new gem entry starts at column 0 as "name (versions)".
		if line != "" && line[0] != ' ' && strings.Contains(line, "(") {
			flush()
			name := strings.TrimSpace(line[:strings.Index(line, "(")])
			ver := strings.TrimSuffix(line[strings.Index(line, "(")+1:], ")")
			cur = &Component{Name: name, Version: strings.TrimSpace(ver), Source: "gem (Ruby)", Vendor: "RubyGems / community"}
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(trimmed, "Homepage:") {
			cur.Homepage = strings.TrimSpace(trimmed[len("Homepage:"):])
			if v := vendorFromHomepage(cur.Homepage); v != "" {
				cur.Vendor = v
			}
		} else if cur.Desc == "" && trimmed != "" && !strings.HasPrefix(trimmed, "Author") &&
			!strings.HasPrefix(trimmed, "Installed at") && !strings.HasPrefix(trimmed, "License") {
			cur.Desc = trimmed
		}
	}
	flush()
	return comps
}

// ---------------------------------------------------------------------------
// Cargo (Rust) — `cargo install --list` (name + version only, offline).
// ---------------------------------------------------------------------------

func scanCargo() []Component {
	out, ok := run("cargo", "install", "--list")
	if !ok {
		return nil
	}
	return parseCargo(out)
}

func parseCargo(out string) []Component {
	var comps []Component
	for _, line := range strings.Split(out, "\n") {
		if line == "" || line[0] == ' ' {
			continue // indented lines are the installed binaries
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ver := strings.TrimSuffix(strings.TrimPrefix(fields[1], "v"), ":")
		c := Component{Name: fields[0], Version: ver, Source: "cargo (Rust)", Vendor: "crates.io / community"}
		c.Category = classifyCategory(c)
		comps = append(comps, c)
	}
	return comps
}
