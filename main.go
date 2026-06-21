// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// version is stamped at build time via -ldflags "-X main.version=...". It stays
// "dev" for a plain `go build` / `go install` without that flag.
var version = "dev"

// scanners is the full registry. Each entry is gated to the platforms where it
// makes sense; a single binary carries them all and selects at runtime.
func scanners() []Scanner {
	return []Scanner{
		{
			Name: "Homebrew", OS: []string{"darwin", "linux"}, Site: "https://brew.sh",
			Install: map[string]string{
				"darwin": `/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`,
				"linux":  `/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`,
			},
			Probe: func() bool { return have("brew") }, Scan: scanBrew,
		},
		{
			Name: "npm (global)", Site: "https://nodejs.org",
			Install: map[string]string{"darwin": "brew install node", "linux": "install Node.js from your package manager or https://nodejs.org", "windows": "winget install OpenJS.NodeJS"},
			Probe:   func() bool { return have("npm") }, Scan: scanNpm,
		},
		{
			Name: "pip (Python)", Site: "https://www.python.org/downloads/",
			Install: map[string]string{"darwin": "brew install python", "linux": "install python3-pip from your package manager", "windows": "winget install Python.Python.3.13"},
			Probe:   func() bool { return pipBin() != "" }, Scan: scanPip,
		},
		{
			Name: "gem (Ruby)", Site: "https://www.ruby-lang.org/",
			Install: map[string]string{"darwin": "brew install ruby", "linux": "install ruby from your package manager", "windows": "winget install RubyInstallerTeam.Ruby.3.3"},
			Probe:   func() bool { return have("gem") }, Scan: scanGem,
		},
		{
			Name: "cargo (Rust)", Site: "https://rustup.rs",
			Install: map[string]string{"darwin": "brew install rust  # or rustup", "linux": `curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh`, "windows": "winget install Rustlang.Rustup"},
			Probe:   func() bool { return have("cargo") }, Scan: scanCargo,
		},
		// macOS applications: no external tool needed (uses built-in plutil).
		{
			Name: "macOS applications", OS: []string{"darwin"},
			Probe: func() bool { return runtime.GOOS == "darwin" }, Scan: scanMacApps,
		},
		// Windows
		{
			Name: "Windows installed apps (registry)", OS: []string{"windows"},
			Probe: func() bool { return have("powershell") || have("pwsh") }, Scan: scanWindowsRegistry,
			Site: "https://learn.microsoft.com/powershell/", Install: map[string]string{"windows": "PowerShell ships with Windows; install PowerShell 7 from the Microsoft Store if missing"},
		},
		{
			Name: "winget", OS: []string{"windows"}, Site: "https://learn.microsoft.com/windows/package-manager/",
			Install: map[string]string{"windows": "install 'App Installer' from the Microsoft Store"},
			Probe:   func() bool { return have("winget") }, Scan: scanWinget,
		},
		{
			Name: "Chocolatey", OS: []string{"windows"}, Site: "https://chocolatey.org/install",
			Install: map[string]string{"windows": "see https://chocolatey.org/install"},
			Probe:   func() bool { return have("choco") }, Scan: scanChoco,
		},
		{
			Name: "Scoop", OS: []string{"windows"}, Site: "https://scoop.sh",
			Install: map[string]string{"windows": `iwr -useb get.scoop.sh | iex`},
			Probe:   func() bool { return have("scoop") }, Scan: scanScoop,
		},
		// Linux
		{
			Name: "dpkg (Debian/Ubuntu)", OS: []string{"linux"},
			Probe: func() bool { return have("dpkg-query") }, Scan: scanDpkg,
		},
		{
			Name: "rpm (Fedora/RHEL/SUSE)", OS: []string{"linux"},
			Probe: func() bool { return have("rpm") }, Scan: scanRpm,
		},
		{
			Name: "Flatpak", OS: []string{"linux"}, Site: "https://flatpak.org/setup/",
			Install: map[string]string{"linux": "install flatpak from your package manager"},
			Probe:   func() bool { return have("flatpak") }, Scan: scanFlatpak,
		},
		{
			Name: "Snap", OS: []string{"linux"}, Site: "https://snapcraft.io",
			Install: map[string]string{"linux": "install snapd from your package manager"},
			Probe:   func() bool { return have("snap") }, Scan: scanSnap,
		},
	}
}

// osExit is a seam so the main entrypoint itself is testable (a test overrides
// it to capture the code instead of terminating the process).
var osExit = os.Exit

func main() { osExit(realMain(os.Args[1:], os.Stdout, scanners())) }

// realMain is the testable program body: parse flags, run the relevant
// scanners, build the inventory, write the report, and report results. It
// returns a process exit code (0 ok, 1 runtime error, 2 bad flags).
func realMain(args []string, stdout io.Writer, scs []Scanner) int {
	fs := flag.NewFlagSet("sbom-creator", flag.ContinueOnError)
	fs.SetOutput(stdout)
	out := fs.String("o", "sbom.html", "output file path, or - for stdout")
	format := fs.String("format", "html", "output format: "+formatList())
	noOpen := fs.Bool("no-open", false, "do not open the report in a browser when done")
	quiet := fs.Bool("quiet", false, "suppress progress output")
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}
	gen, ok := generators[*format]
	if !ok {
		fmt.Fprintf(stdout, "unknown -format %q (want: %s)\n", *format, formatList())
		return 2
	}

	// Where does output go? Explicit -o wins; otherwise non-HTML formats stream
	// to stdout (pipe-friendly, e.g. `… -format cyclonedx | grype`) while HTML
	// defaults to a file we can open in a browser.
	oSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "o" {
			oSet = true
		}
	})
	toStdout := *out == "-" || (!oSet && *format != "html")

	goos := runtime.GOOS
	logf := func(format string, a ...any) {
		if !*quiet && !toStdout { // keep stdout clean when it carries the SBOM
			fmt.Fprintf(stdout, format+"\n", a...)
		}
	}

	logf("SBOM Creator — scanning %s", osLabel())
	logf(strings.Repeat("-", 52))

	var raw []Component
	var missing []MissingTool
	relevantCount := 0

	for _, s := range scs {
		if !relevant(s, goos) {
			continue
		}
		relevantCount++
		if s.Probe != nil && !s.Probe() {
			missing = append(missing, MissingTool{Name: s.Name, Install: s.Install[goos], Site: s.Site})
			logf("  x %-34s not found", s.Name)
			continue
		}
		comps := s.Scan()
		logf("  + %-34s %d components", s.Name, len(comps))
		raw = append(raw, comps...)
	}

	all := buildInventory(raw)

	host, _ := os.Hostname()
	now := time.Now()
	content := gen(genCtx{
		comps: all, missing: missing, host: host, osLabel: osLabel(),
		date: now.Format("2006-01-02 15:04"), timestamp: now.Format(time.RFC3339),
	})

	if toStdout {
		stdout.Write(content)
		return 0
	}

	outPath, _ := filepath.Abs(*out)
	if err := os.WriteFile(outPath, content, 0o644); err != nil {
		fmt.Fprintln(stdout, "error writing report:", err)
		return 1
	}

	logf(strings.Repeat("-", 52))
	logf("Inventoried %d components from %d/%d sources.", len(all), relevantCount-len(missing), relevantCount)
	logf("Report (%s): %s", *format, outPath)

	if len(missing) > 0 && !*quiet {
		fmt.Fprintln(stdout, "\nNot installed on this machine (install to include them, then re-run):")
		for _, m := range missing {
			fmt.Fprintf(stdout, "  - %s\n", m.Name)
			if m.Install != "" {
				fmt.Fprintf(stdout, "      install: %s\n", m.Install)
			}
			if m.Site != "" {
				fmt.Fprintf(stdout, "      docs:    %s\n", m.Site)
			}
		}
	}

	if *format == "html" && !*noOpen {
		openInBrowser(outPath)
	}
	return 0
}

// buildInventory post-processes raw scan results: dedupe GUI apps that are also
// package-manager-managed, compute reverse-dependency counts, and sort.
func buildInventory(raw []Component) []Component {
	// A GUI app installed via a package manager also shows up in the
	// /Applications scan. Keep the package-manager record (richer: desc,
	// homepage, deps) and drop the bare app bundle of the same name.
	managed := map[string]bool{}
	for _, c := range raw {
		if c.Source == "Homebrew (cask)" {
			managed[strings.ToLower(c.Name)] = true
		}
	}
	all := raw
	if len(managed) > 0 {
		kept := raw[:0]
		for _, c := range raw {
			if (c.Source == "Direct install" || c.Source == "Mac App Store") && managed[strings.ToLower(c.Name)] {
				continue
			}
			kept = append(kept, c)
		}
		all = kept
	}

	// Reverse-dependency counts within the inventoried set.
	rev := map[string]int{}
	for _, c := range all {
		for _, d := range dedupeStrings(c.Deps) {
			rev[d]++
		}
	}
	for i := range all {
		all[i].UsedBy = rev[all[i].Name]
		all[i].Deps = dedupeStrings(all[i].Deps)
		all[i].PURL = purlFor(all[i])
	}

	sort.Slice(all, func(i, j int) bool {
		if strings.EqualFold(all[i].Category, all[j].Category) {
			return strings.ToLower(all[i].Name) < strings.ToLower(all[j].Name)
		}
		return strings.ToLower(all[i].Category) < strings.ToLower(all[j].Category)
	})
	return all
}

// browserCommand builds the platform-appropriate "open this file" command.
// Pure and testable; openInBrowser is the thin side-effecting wrapper.
func browserCommand(goos, path string) *exec.Cmd {
	switch goos {
	case "darwin":
		return execCommand(context.Background(), "open", path)
	case "windows":
		return execCommand(context.Background(), "rundll32", "url.dll,FileProtocolHandler", path)
	default:
		return execCommand(context.Background(), "xdg-open", path)
	}
}

func openInBrowser(path string) { _ = browserCommand(runtime.GOOS, path).Start() }
