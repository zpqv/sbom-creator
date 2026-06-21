// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"strings"
	"time"
)

// Windows installed-software inventory.
//
// The authoritative offline source is the registry Uninstall hive, which the
// Windows Settings "Installed apps" list itself reads. It carries DisplayName,
// DisplayVersion, Publisher (the provider) and an info URL — no network needed.
// winget/choco/scoop are added on top when present for package-manager context.

const psUninstall = `
$paths = @(
 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*',
 'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*',
 'HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*')
Get-ItemProperty $paths -ErrorAction SilentlyContinue |
 Where-Object { $_.DisplayName -and -not $_.SystemComponent } |
 Select-Object DisplayName, DisplayVersion, Publisher, URLInfoAbout |
 ConvertTo-Json -Compress`

func psRun(script string) (string, bool) {
	for _, sh := range []string{"powershell", "pwsh"} {
		if have(sh) {
			return runIn(90*time.Second, sh, "-NoProfile", "-NonInteractive", "-Command", script)
		}
	}
	return "", false
}

func scanWindowsRegistry() []Component {
	out, ok := psRun(psUninstall)
	if !ok {
		return nil
	}
	return parseWindowsRegistry(out)
}

func parseWindowsRegistry(out string) []Component {
	out = strings.TrimSpace(out)
	if out == "" {
		return nil
	}
	// ConvertTo-Json yields an object for a single result, an array otherwise.
	if strings.HasPrefix(out, "{") {
		out = "[" + out + "]"
	}
	var rows []struct {
		DisplayName    string `json:"DisplayName"`
		DisplayVersion string `json:"DisplayVersion"`
		Publisher      string `json:"Publisher"`
		URLInfoAbout   string `json:"URLInfoAbout"`
	}
	if json.Unmarshal([]byte(out), &rows) != nil {
		return nil
	}
	seen := map[string]bool{}
	var comps []Component
	for _, r := range rows {
		name := strings.TrimSpace(r.DisplayName)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		c := Component{
			Name: name, Version: r.DisplayVersion, Source: "Windows (registry)",
			Vendor:   firstNonEmpty(r.Publisher, vendorFromHomepage(r.URLInfoAbout), "Unknown"),
			Homepage: r.URLInfoAbout,
		}
		c.Category = classifyCategory(c)
		comps = append(comps, c)
	}
	return comps
}

func scanWinget() []Component {
	// `winget export` emits JSON of winget-managed packages without prompting.
	out, ok := runIn(60*time.Second, "winget", "export", "-o", "-", "--accept-source-agreements")
	if !ok {
		return nil
	}
	return parseWinget(out)
}

func parseWinget(out string) []Component {
	var doc struct {
		Sources []struct {
			Packages []struct {
				PackageIdentifier string `json:"PackageIdentifier"`
				Version           string `json:"Version"`
			} `json:"Packages"`
		} `json:"Sources"`
	}
	if json.Unmarshal([]byte(out), &doc) != nil {
		return nil
	}
	var comps []Component
	for _, s := range doc.Sources {
		for _, p := range s.Packages {
			c := Component{
				Name: p.PackageIdentifier, Version: p.Version, Source: "winget",
				Vendor: strings.SplitN(p.PackageIdentifier, ".", 2)[0],
			}
			c.Category = classifyCategory(c)
			comps = append(comps, c)
		}
	}
	return comps
}

func scanChoco() []Component {
	out, ok := runIn(40*time.Second, "choco", "list", "--limit-output", "--local-only")
	if !ok {
		// newer choco drops --local-only
		out, ok = runIn(40*time.Second, "choco", "list", "--limit-output")
		if !ok {
			return nil
		}
	}
	return parseChoco(out)
}

func parseChoco(out string) []Component {
	var comps []Component
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(strings.TrimSpace(line), "|")
		if len(parts) != 2 {
			continue
		}
		c := Component{Name: parts[0], Version: parts[1], Source: "Chocolatey", Vendor: "Chocolatey / community"}
		c.Category = classifyCategory(c)
		comps = append(comps, c)
	}
	return comps
}

func scanScoop() []Component {
	out, ok := runIn(40*time.Second, "scoop", "export")
	if !ok {
		return nil
	}
	return parseScoop(out)
}

func parseScoop(out string) []Component {
	var doc struct {
		Apps []struct {
			Name    string `json:"Name"`
			Version string `json:"Version"`
			Source  string `json:"Source"`
		} `json:"apps"`
	}
	if json.Unmarshal([]byte(out), &doc) != nil {
		return nil
	}
	var comps []Component
	for _, a := range doc.Apps {
		c := Component{Name: a.Name, Version: a.Version, Source: "Scoop", Vendor: firstNonEmpty(a.Source, "Scoop / community")}
		c.Category = classifyCategory(c)
		comps = append(comps, c)
	}
	return comps
}
