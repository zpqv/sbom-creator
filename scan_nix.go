// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"time"
)

// Linux native package inventories. Each is offline and carries enough
// metadata (name, version, summary, homepage) to populate the SBOM well.

func scanDpkg() []Component {
	// dpkg-query exposes Homepage and the short description directly.
	out, ok := runIn(60*time.Second, "dpkg-query", "-W",
		"-f=${Package}\t${Version}\t${binary:Summary}\t${Homepage}\n")
	if !ok {
		return nil
	}
	return parseDpkg(out)
}

func parseDpkg(out string) []Component {
	return parseTabbed(out, "dpkg (Debian/Ubuntu)", "Distribution package")
}

func scanRpm() []Component {
	out, ok := runIn(60*time.Second, "rpm", "-qa", "--qf",
		"%{NAME}\t%{VERSION}-%{RELEASE}\t%{SUMMARY}\t%{URL}\n")
	if !ok {
		return nil
	}
	return parseRpm(out)
}

func parseRpm(out string) []Component {
	return parseTabbed(out, "rpm (Fedora/RHEL/SUSE)", "Distribution package")
}

// parseTabbed handles the shared "name\tversion\tsummary\thomepage" layout that
// dpkg-query and rpm both emit, differing only in source label.
func parseTabbed(out, source, fallbackVendor string) []Component {
	var comps []Component
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(line, "\t")
		if len(f) < 2 || f[0] == "" {
			continue
		}
		desc, home := "", ""
		if len(f) >= 3 {
			desc = f[2]
		}
		if len(f) >= 4 {
			home = f[3]
		}
		c := Component{
			Name: f[0], Version: f[1], Source: source,
			Desc: desc, Homepage: home,
			Vendor: firstNonEmpty(vendorFromHomepage(home), fallbackVendor),
		}
		c.Category = classifyCategory(c)
		comps = append(comps, c)
	}
	return comps
}

func scanFlatpak() []Component {
	out, ok := runIn(40*time.Second, "flatpak", "list", "--app",
		"--columns=name,application,version,description")
	if !ok {
		return nil
	}
	return parseFlatpak(out)
}

func parseFlatpak(out string) []Component {
	var comps []Component
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(line, "\t")
		if len(f) < 2 || f[0] == "" {
			continue
		}
		ver, desc := "", ""
		if len(f) >= 3 {
			ver = f[2]
		}
		if len(f) >= 4 {
			desc = f[3]
		}
		c := Component{Name: f[0], Version: ver, Source: "Flatpak", Desc: desc, Vendor: f[1]} // f[1] = application id
		c.Category = classifyCategory(c)
		comps = append(comps, c)
	}
	return comps
}

func scanSnap() []Component {
	out, ok := runIn(40*time.Second, "snap", "list")
	if !ok {
		return nil
	}
	return parseSnap(out)
}

func parseSnap(out string) []Component {
	var comps []Component
	for i, line := range strings.Split(out, "\n") {
		if i == 0 || strings.TrimSpace(line) == "" { // header
			continue
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		vendor := "Snap / community"
		if len(f) >= 5 {
			vendor = f[4] // publisher column
		}
		c := Component{Name: f[0], Version: f[1], Source: "Snap", Vendor: vendor}
		c.Category = classifyCategory(c)
		comps = append(comps, c)
	}
	return comps
}
