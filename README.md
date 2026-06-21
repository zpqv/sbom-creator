# SBOM Creator

A standalone, cross-platform tool that scans everything installed on a machine
and writes a single self-contained webpage — a Software Bill of Materials. It
lists each component with what it does, its version, its dependencies, the
provider/website it comes from, and a category.

- **One binary per OS, no runtime to install.** Pure Go, standard library only.
- **No network, no API.** Everything comes from local package-manager metadata.
  No data leaves the machine.
- **The output is the webpage.** Open `sbom.html` in any browser — search,
  filter by source/category, sort, expand dependency lists, export JSON/CSV.
- **Tells you what's missing.** If a package source isn't installed (e.g. no
  Homebrew, no Rust), the tool names it and prints the exact install command,
  then continues with what it can read.

## Usage

```
sbom-creator                 # scan, write ./sbom.html, open it in the browser
sbom-creator -o report.html  # choose the output path
sbom-creator -no-open        # don't auto-open the browser
sbom-creator -quiet          # no progress output
```

Just run the binary for your platform — there is nothing else to install.

## What it scans

| Platform | Sources |
|----------|---------|
| macOS    | Homebrew (formulae + casks), npm globals, pip, gem, cargo, `/Applications` + system apps (with Mac App Store detection) |
| Linux    | Homebrew, npm, pip, gem, cargo, dpkg, rpm, Flatpak, Snap |
| Windows  | npm, pip, gem, cargo, registry "Installed apps", winget, Chocolatey, Scoop |

Each source is probed first; only the ones present on the machine run. The rest
are reported as "not installed" with an install hint.

## How the data is sourced (offline)

| Field | Where it comes from |
|-------|---------------------|
| description | `brew info --json`, npm `package.json`, `pip show`, `gem list -d`, dpkg/rpm metadata; curated map for GUI apps (plists carry none) |
| version | the package manager's installed-version record |
| dependencies | declared direct deps from the same metadata |
| provider/website | each component's published homepage (Windows uses the registry `Publisher`); the project/foundation is inferred from the domain |
| category | name overrides + Homebrew's canonical formula map + keyword heuristics over name/description |
| "used by" | reverse-dependency count computed across the inventoried set |

## Build from source

Requires Go 1.21+.

```
go build -o sbom-creator .      # current platform
./build.sh 1.0.0                # cross-compile all platforms into dist/
```

`build.sh` produces darwin-arm64, darwin-amd64, linux-amd64, linux-arm64 and
windows-amd64 with `CGO_ENABLED=0` (no C toolchain needed).

## Testing

The validation harness is documented in [VALIDATION.md](VALIDATION.md) and run
with one command:

```
./test.sh                 # gofmt, vet, staticcheck, govulncheck, race tests,
                          # 100% coverage gate, fuzzing, cross-compile
./test.sh --mutation      # also run mutation testing (gremlins)
```

Current status: **100% statement coverage**, **100% mutation efficacy**,
race-clean, zero third-party dependencies, no known vulnerabilities. CI
(`.github/workflows/ci.yml`) runs the suite plus an end-to-end job on
macOS/Linux/Windows so the real per-OS scanners are exercised natively.

### Pre-commit hook

A version-controlled hook scans staged changes for secrets (gitleaks) and
checks formatting (gofmt). Enable it once per clone:

```
git config core.hooksPath hooks
brew install gitleaks   # or https://github.com/gitleaks/gitleaks/releases
```

## License

Apache License 2.0 — see [LICENSE](LICENSE). Every source file carries an
`SPDX-License-Identifier: Apache-2.0` header. Copyright 2026 ZPQV, Inc.

## Project layout

```
main.go        scanner registry, orchestration, CLI flags, dedupe, browser open
model.go       Component / Scanner / MissingTool types
util.go        exec helpers, OS detection
classify.go    category + provider inference (works on any machine)
appmeta.go     curated GUI-app descriptions
scan_brew.go   Homebrew (mac + linux)
scan_lang.go   npm, pip, gem, cargo (all platforms)
scan_macos.go  macOS .app bundles + App Store detection
scan_win.go    Windows registry + winget/choco/scoop
scan_nix.go    dpkg, rpm, flatpak, snap
render.go      embeds template.html, injects JSON
template.html  the SBOM webpage (embedded via go:embed)
```

## Notes & limitations

- Provider names name the project/foundation behind a component (e.g. "GNU
  Project", "X.Org Foundation"), not always a commercial entity.
- GUI-app descriptions exist only for apps in the curated map; others render
  without one rather than with a fabricated description.
- Ruby's default stdlib gems are included as they are reported by `gem list`.
- The macOS `Utilities` subfolder is not enumerated.
- A component installed both directly and via a package manager (e.g. a brew
  cask that also lands in `/Applications`) is de-duplicated to the richer
  package-manager record.
