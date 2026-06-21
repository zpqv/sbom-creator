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

## Install

Download the archive for your platform from the
[latest release](https://github.com/zpqv/sbom-creator/releases/latest) and
extract it. Each archive contains a single `sbom-creator` binary; the release
also ships `SHA256SUMS` to verify your download.

| Platform | Asset |
|----------|-------|
| macOS (Apple Silicon) | `sbom-creator-darwin-arm64.tar.gz` |
| macOS (Intel)         | `sbom-creator-darwin-amd64.tar.gz` |
| Linux (x86-64)        | `sbom-creator-linux-amd64.tar.gz` |
| Linux (ARM64)         | `sbom-creator-linux-arm64.tar.gz` |
| Windows (x86-64)      | `sbom-creator-windows-amd64.zip` |

**macOS / Linux** — extract with the `tar` command (not by double-clicking in
Finder) and run. The binary is unsigned, but a binary extracted by command-line
`tar` is not quarantined, so it runs without a Gatekeeper prompt:

```sh
# Apple Silicon shown; swap in your platform's filename.
curl -L https://github.com/zpqv/sbom-creator/releases/latest/download/sbom-creator-darwin-arm64.tar.gz | tar xz
./sbom-creator
```

If you do download a bare binary instead and macOS blocks it, clear the
quarantine flag once: `xattr -c sbom-creator && chmod +x sbom-creator`.

**Windows** (PowerShell) — extract the `.zip`, then run. SmartScreen may warn on
an unsigned binary; choose *More info → Run anyway*:

```powershell
Expand-Archive sbom-creator-windows-amd64.zip
.\sbom-creator-windows-amd64\sbom-creator.exe
```

**Verify the download** (optional, recommended):

```sh
sha256sum -c SHA256SUMS --ignore-missing      # Linux
shasum -a 256 -c SHA256SUMS --ignore-missing  # macOS
```

**With Go installed**, you can skip the download:

```sh
go install github.com/zpqv/sbom-creator@latest
```

## Usage

```
sbom-creator                 # scan, write ./sbom.html, open it in the browser
sbom-creator -o report.html  # choose the output path
sbom-creator -no-open        # don't auto-open the browser
sbom-creator -quiet          # no progress output
sbom-creator -version        # print the version and exit
```

Run the binary for your platform — there is nothing else to install. The tool
writes `sbom.html` and opens it in your default browser.

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
windows-amd64 with `CGO_ENABLED=0` (no C toolchain needed). The version passed
to `build.sh` is stamped into the binary (`sbom-creator -version`).

## Releasing

Releases are cut by `.github/workflows/release.yml` when a semver tag is pushed:

```
git tag v1.2.3
git push origin v1.2.3
```

The workflow cross-compiles all five targets, generates `SHA256SUMS`,
smoke-tests the Linux binary, and publishes a GitHub Release with the assets and
auto-generated notes. You can also run it manually from the Actions tab
(*Run workflow* → version) without pushing a tag.

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
