// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// userHomeDir is the seam for locating $HOME. Tests override it so the
// filesystem scanners run against a temp dir deterministically on every OS
// (os.UserHomeDir reads $HOME on unix but %USERPROFILE% on Windows, which the
// CI matrix also runs).
var userHomeDir = os.UserHomeDir

// ---------------------------------------------------------------------------
// Sources that live outside the mainstream package managers. Modern developer
// CLIs increasingly ship via `uv tool install`, `bun add -g`, or their own
// `curl | sh` installers that drop a binary into ~/.local/bin. None of those
// are covered by scan_lang.go, so without these scanners a machine's AI/agent
// tooling is largely invisible. Everything here is read locally; no network.
// ---------------------------------------------------------------------------

// --- uv tools (`uv tool list`) --------------------------------------------

func scanUvTools() []Component {
	out, ok := run("uv", "tool", "list")
	if !ok {
		return nil
	}
	comps := parseUvToolList(out)
	for i := range comps {
		enrichUvTool(&comps[i]) // fills desc/homepage/license from the tool's dist-info
		comps[i].Vendor = firstNonEmpty(vendorFromHomepage(comps[i].Homepage), comps[i].Vendor)
		comps[i].Category = classifyCategory(comps[i])
	}
	return comps
}

// parseUvToolList reads `uv tool list`. Each tool is a flush-left "name vX.Y"
// line; the executables it provides follow as indented "- exe" lines, which we
// skip. Pure (no filesystem) so it is unit-tested directly.
func parseUvToolList(out string) []Component {
	var comps []Component
	for _, line := range strings.Split(out, "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '-' {
			continue // blank, indented, or an "- exe" line
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.HasPrefix(fields[1], "v") {
			continue
		}
		comps = append(comps, Component{
			Name:    fields[0],
			Version: strings.TrimPrefix(fields[1], "v"),
			Source:  "uv (tool)",
			Vendor:  "PyPI / community",
		})
	}
	return comps
}

// enrichUvTool reads the tool's own *.dist-info/METADATA (Python core metadata)
// for a description, homepage and license. It targets the dist-info whose name
// matches the tool (normalizing PyPI's '-'/'_' equivalence) so a dependency's
// metadata is never mistaken for the tool's own. Best-effort: leaves fields
// blank when the file is absent or unreadable.
func enrichUvTool(c *Component) {
	home, err := userHomeDir()
	if err != nil {
		return
	}
	base := filepath.Join(home, ".local", "share", "uv", "tools", c.Name, "lib")
	var metaPath string
	for _, nm := range []string{c.Name, strings.ReplaceAll(c.Name, "-", "_")} {
		if m, _ := filepath.Glob(filepath.Join(base, "*", "site-packages", nm+"-*.dist-info", "METADATA")); len(m) > 0 {
			metaPath = m[0]
			break
		}
	}
	if metaPath == "" {
		return
	}
	b, err := os.ReadFile(metaPath)
	if err != nil {
		return
	}
	applyPyMetadata(c, string(b))
}

// applyPyMetadata parses the RFC822-style header block of a Python METADATA
// file into a Component. It stops at the first blank line, which separates the
// headers from the long-description body. Pure, so it is unit-tested directly.
func applyPyMetadata(c *Component, meta string) {
	for _, line := range strings.Split(meta, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			break // headers end; the description body follows
		}
		switch {
		case strings.HasPrefix(line, "Summary:"):
			c.Desc = strings.TrimSpace(line[len("Summary:"):])
		case strings.HasPrefix(line, "Home-page:") && c.Homepage == "":
			c.Homepage = strings.TrimSpace(line[len("Home-page:"):])
		case strings.HasPrefix(line, "License:"):
			if l := strings.TrimSpace(line[len("License:"):]); l != "" && l != "UNKNOWN" {
				c.License = l
			}
		case strings.HasPrefix(line, "Project-URL:") && c.Homepage == "":
			// "Project-URL: Homepage, https://..." — take the URL when the label
			// is Homepage (the closest thing to a canonical homepage).
			v := strings.TrimSpace(line[len("Project-URL:"):])
			if i := strings.Index(v, ","); i >= 0 && strings.EqualFold(strings.TrimSpace(v[:i]), "Homepage") {
				c.Homepage = strings.TrimSpace(v[i+1:])
			}
		}
	}
}

// --- standalone CLIs on PATH (curated) -------------------------------------

// pathApp is curated metadata for a self-installed binary. These tools carry no
// package-manager manifest, so — like appDesc for GUI apps — we supply the
// details for the ones worth inventorying. A binary not listed here is ignored
// rather than guessed at, keeping noise (python shims, stray scripts) out.
type pathApp struct {
	display, desc, vendor, homepage, category string
}

var pathAppMeta = map[string]pathApp{
	"cursor-agent": {"Cursor CLI", "Terminal coding agent from the makers of the Cursor editor.", "Anysphere", "https://cursor.com", "AI / LLM Tools"},
	"amp":          {"Amp", "Agentic coding tool by Sourcegraph.", "Sourcegraph", "https://ampcode.com", "AI / LLM Tools"},
	"devin":        {"Devin", "CLI for Cognition's autonomous software engineer.", "Cognition", "https://devin.ai", "AI / LLM Tools"},
	"droid":        {"Droid", "Factory's autonomous software-development agent.", "Factory", "https://factory.ai", "AI / LLM Tools"},
	"opencode":     {"OpenCode", "Open-source, model-agnostic terminal coding agent.", "opencode.ai", "https://opencode.ai", "AI / LLM Tools"},
	"omp":          {"oh-my-pi", "Coding agent CLI (pi-coding-agent family).", "oh-my-pi", "https://www.npmjs.com/package/@oh-my-pi/pi-coding-agent", "AI / LLM Tools"},
	"kavith":       {"kavith", "DeepSeek-powered web-app generator CLI.", "local / custom", "", "AI / LLM Tools"},
}

// pathAppDirs are the standalone-installer bin directories we scan, relative to
// $HOME. Kept as a var so tests can point it at a temp dir.
var pathAppDirs = []string{
	filepath.Join(".local", "bin"),
	filepath.Join(".opencode", "bin"),
	filepath.Join(".bun", "bin"),
}

func scanPathApps() []Component {
	home, err := userHomeDir()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var comps []Component
	for _, rel := range pathAppDirs {
		entries, err := os.ReadDir(filepath.Join(home, rel))
		if err != nil {
			continue
		}
		for _, e := range entries {
			base := e.Name()
			meta, ok := pathAppMeta[base]
			if !ok || seen[base] {
				continue
			}
			full := filepath.Join(home, rel, base)
			if pmOwned(resolveSymlink(full)) {
				continue // a uv/pyenv shim — already inventoried by its real source
			}
			seen[base] = true
			c := Component{
				Name: meta.display, Version: pathAppVersion(full),
				Source: "Direct install (PATH)", Vendor: meta.vendor,
				Desc: meta.desc, Homepage: meta.homepage, Category: meta.category,
			}
			comps = append(comps, c)
		}
	}
	return comps
}

// pmOwned reports whether a resolved path belongs to another package manager
// that already inventories it (uv tool venvs, uv-managed Pythons). Prevents
// double-counting when such a manager also drops a shim into ~/.local/bin.
// Backslashes are normalized to forward slashes so a Windows-style resolved
// path matches the same way a POSIX one does.
func pmOwned(real string) bool {
	real = strings.ReplaceAll(real, `\`, "/")
	return strings.Contains(real, "/uv/tools/") || strings.Contains(real, "/uv/python/")
}

// resolveSymlink follows a symlink to its target. It is a var so tests can
// control resolution deterministically without creating real symlinks (which
// require elevated privileges on Windows).
var resolveSymlink = func(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// pathAppVersion best-effort runs `<bin> --version` (local, bounded) and pulls
// the first version-like token out. Empty when the tool errors or prints none.
func pathAppVersion(bin string) string {
	out, ok := runIn(6*time.Second, bin, "--version")
	if !ok {
		return ""
	}
	return parseVersionToken(out)
}

var verTokenRe = regexp.MustCompile(`\d+\.\d+(?:\.\d+)?[A-Za-z0-9.\-+_]*`)

// parseVersionToken extracts the first "1.2", "1.2.3" or calendar-style
// "2026.07.16-abc" token from arbitrary --version output, dropping any leading
// "v" and trailing punctuation. Pure; unit-tested directly.
func parseVersionToken(s string) string {
	m := verTokenRe.FindString(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "v")))
	return strings.Trim(m, ".")
}
