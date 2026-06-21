// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// byName indexes components for convenient assertions.
func byName(comps []Component) map[string]Component {
	m := map[string]Component{}
	for _, c := range comps {
		m[c.Name] = c
	}
	return m
}

const brewJSON = `{
 "formulae":[
   {"name":"jq","desc":"JSON processor","homepage":"https://jqlang.github.io/jq/","license":"MIT","dependencies":["oniguruma"],"installed":[{"version":"1.8.1"}]},
   {"name":"orphan","desc":"no installed record","homepage":"","dependencies":[],"installed":[]}
 ],
 "casks":[
   {"token":"warp","name":["Warp"],"desc":"Rust terminal","homepage":"https://www.warp.dev/","version":"1.2","depends_on":{"formula":["ripgrep"]}},
   {"token":"noname","name":[],"desc":"","homepage":"","version":"9","depends_on":{}}
 ]
}`

func TestParseBrew(t *testing.T) {
	comps := parseBrew(brewJSON)
	m := byName(comps)
	if len(comps) != 4 {
		t.Fatalf("want 4 components, got %d", len(comps))
	}
	if jq := m["jq"]; jq.Version != "1.8.1" || jq.Category != "Developer Tools" || len(jq.Deps) != 1 || jq.License != "MIT" {
		t.Errorf("jq parsed wrong: %+v", jq)
	}
	if m["orphan"].Version != "" {
		t.Errorf("orphan should have empty version, got %q", m["orphan"].Version)
	}
	if w := m["Warp"]; w.Source != "Homebrew (cask)" || w.Category != "Terminals" {
		t.Errorf("Warp cask parsed wrong: %+v", w)
	}
	if _, ok := m["noname"]; !ok { // empty name array falls back to token
		t.Error("cask with empty name should use token")
	}
	if parseBrew("{not json") != nil {
		t.Error("invalid brew JSON should parse to nil")
	}
}

func TestParseNpmRoot(t *testing.T) {
	root := t.TempDir()
	write := func(dir, content string) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "cli"), `{"name":"cli","version":"1.0","description":"a tool","homepage":"https://github.com/acme/cli","license":"Apache-2.0","dependencies":{"dep-a":"1","dep-b":"2"}}`)
	write(filepath.Join(root, "@scope", "pkg"), `{"name":"@scope/pkg","version":"2.0","license":{"type":"ISC"}}`)
	// noise that must be ignored:
	os.MkdirAll(filepath.Join(root, ".bin"), 0o755)
	os.MkdirAll(filepath.Join(root, "empty"), 0o755) // dir with no package.json
	os.WriteFile(filepath.Join(root, "afile"), []byte("x"), 0o644)

	comps := parseNpmRoot(root)
	m := byName(comps)
	if len(comps) != 2 {
		t.Fatalf("want 2 npm components, got %d: %+v", len(comps), comps)
	}
	if c := m["cli"]; c.Vendor != "acme" || len(c.Deps) != 2 || c.License != "Apache-2.0" {
		t.Errorf("cli parsed wrong: %+v", c)
	}
	if c, ok := m["@scope/pkg"]; !ok || c.License != "ISC" { // legacy object license form
		t.Errorf("scoped package parsed wrong: %+v", c)
	}
	if parseNpmRoot(filepath.Join(root, "does-not-exist")) != nil {
		t.Error("missing root should parse to nil")
	}
}

func TestNpmComponentBadInputs(t *testing.T) {
	dir := t.TempDir()
	if _, ok := npmComponent(dir); ok { // no package.json
		t.Error("missing package.json should be ok=false")
	}
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{bad"), 0o644)
	if _, ok := npmComponent(dir); ok {
		t.Error("invalid package.json should be ok=false")
	}
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"version":"1"}`), 0o644)
	if _, ok := npmComponent(dir); ok {
		t.Error("package.json without name should be ok=false")
	}
}

const pipList = `[{"name":"requests","version":"2.33.1"},{"name":"certifi","version":"2026.1.1"}]`
const pipShow = `Name: requests
Version: ignored
Summary: Python HTTP for Humans.
Home-page: https://requests.readthedocs.io
License: Apache-2.0
Requires: certifi, idna
---
Name: certifi
Summary: UNKNOWN
Home-page:
License: UNKNOWN
Requires:
`

func TestParsePip(t *testing.T) {
	comps := parsePip(pipList, pipShow, true)
	m := byName(comps)
	if len(comps) != 2 {
		t.Fatalf("want 2 pip components, got %d", len(comps))
	}
	// "Python HTTP for Humans." matches the http/networking keyword rule, which
	// runs before the pip source-default — generic categories are more useful
	// than a blanket "Python Library".
	if r := m["requests"]; r.Version != "2.33.1" || len(r.Deps) != 2 || r.Category != "Networking & Protocols" || r.License != "Apache-2.0" {
		t.Errorf("requests parsed wrong: %+v", r)
	}
	if c := m["certifi"]; c.Desc != "" || c.License != "" { // UNKNOWN normalized to empty
		t.Errorf("certifi summary/license should be empty, got desc=%q license=%q", c.Desc, c.License)
	}

	// fallback path when `pip show` is unavailable
	fb := parsePip(pipList, "", false)
	if len(fb) != 2 || fb[0].Desc != "" {
		t.Errorf("fallback pip parse wrong: %+v", fb)
	}
	if parsePip("{bad", "", false) != nil {
		t.Error("invalid pip list JSON should parse to nil")
	}
}

func TestPipNames(t *testing.T) {
	names, ok := pipNames(pipList)
	if !ok || len(names) != 2 {
		t.Errorf("pipNames = %v,%v", names, ok)
	}
	if _, ok := pipNames("[]"); ok {
		t.Error("empty list should be ok=false")
	}
	if _, ok := pipNames("{bad"); ok {
		t.Error("invalid JSON should be ok=false")
	}
}

const gemDetails = `nokogiri (1.13.8)
    Authors: foo, bar
    Homepage: https://nokogiri.org
    License: MIT
    Installed at: /x
    HTML, XML, SAX and Reader parser.

rake (12.3.3)
    Homepage: https://github.com/ruby/rake
    Licenses: MIT, Apache-2.0
    Rake is a Make-like build utility.
`

func TestParseGem(t *testing.T) {
	comps := parseGem(gemDetails)
	m := byName(comps)
	if len(comps) != 2 {
		t.Fatalf("want 2 gems, got %d: %+v", len(comps), comps)
	}
	if n := m["nokogiri"]; n.Version != "1.13.8" || n.Homepage != "https://nokogiri.org" || n.Desc == "" || n.License != "MIT" {
		t.Errorf("nokogiri parsed wrong: %+v", n)
	}
	if r := m["rake"]; r.Vendor != "ruby" || r.License != "MIT, Apache-2.0" { // github org fallback + plural Licenses:
		t.Errorf("rake parsed wrong: %+v", r)
	}
	if parseGem("") != nil {
		t.Error("empty gem output should parse to nil")
	}
}

func TestParseCargo(t *testing.T) {
	comps := parseCargo("ripgrep v14.0.0:\n    rg\nbat v0.24.0:\n    bat\nshortline\n")
	if len(comps) != 2 {
		t.Fatalf("want 2 cargo crates, got %d: %+v", len(comps), comps)
	}
	if comps[0].Name != "ripgrep" || comps[0].Version != "14.0.0" {
		t.Errorf("cargo first crate wrong: %+v", comps[0])
	}
	if parseCargo("") != nil {
		t.Error("empty cargo output should parse to nil")
	}
}

func TestParseTabbed(t *testing.T) {
	out := "vim\t2:9.1\tVi IMproved text editor\thttps://www.vim.org/\n" +
		"bare\t1.0\n" + // only name+version
		"\t\t\n" + // empty name -> skipped
		"\n"
	comps := parseDpkg(out)
	if len(comps) != 2 {
		t.Fatalf("want 2 dpkg comps, got %d: %+v", len(comps), comps)
	}
	m := byName(comps)
	if m["vim"].Desc == "" || m["vim"].Source != "dpkg (Debian/Ubuntu)" || m["vim"].License != "" {
		t.Errorf("vim parsed wrong (dpkg has no license field): %+v", m["vim"])
	}
	if m["bare"].Vendor != "Distribution package" {
		t.Errorf("bare vendor = %q", m["bare"].Vendor)
	}
	// parseRpm shares the engine but adds a 5th license field.
	r := parseRpm("zlib\t1.3-1\tcompression library\thttps://zlib.net\tZlib\n")
	if len(r) != 1 || r[0].Source != "rpm (Fedora/RHEL/SUSE)" || r[0].License != "Zlib" {
		t.Errorf("parseRpm wrong: %+v", r)
	}
}

func TestNpmLicense(t *testing.T) {
	cases := []struct{ in, want string }{
		{`"MIT"`, "MIT"},                    // modern string form
		{`{"type":"ISC","url":"x"}`, "ISC"}, // legacy object form
		{`["MIT"]`, ""},                     // array form (unsupported) -> empty
		{``, ""},                            // absent
	}
	for _, c := range cases {
		if got := npmLicense([]byte(c.in)); got != c.want {
			t.Errorf("npmLicense(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseFlatpak(t *testing.T) {
	out := "GIMP\torg.gimp.GIMP\t2.10\tImage editor\n" +
		"Mini\torg.x.Mini\n" + // only name+app
		"\t\n"
	comps := parseFlatpak(out)
	if len(comps) != 2 {
		t.Fatalf("want 2 flatpak comps, got %d: %+v", len(comps), comps)
	}
	if comps[0].Vendor != "org.gimp.GIMP" || comps[0].Version != "2.10" {
		t.Errorf("flatpak GIMP wrong: %+v", comps[0])
	}
}

func TestParseSnap(t *testing.T) {
	out := "Name  Version  Rev  Tracking  Publisher  Notes\n" +
		"core  16-2.61  100  latest    canonical  base\n" + // 6 fields -> publisher
		"mini  1.0\n" + // 2 fields -> default vendor
		"solo\n" + // 1 field -> skipped
		"\n"
	comps := parseSnap(out)
	m := byName(comps)
	if len(comps) != 2 {
		t.Fatalf("want 2 snap comps, got %d: %+v", len(comps), comps)
	}
	if m["core"].Vendor != "canonical" {
		t.Errorf("snap core publisher wrong: %+v", m["core"])
	}
	if m["mini"].Vendor != "Snap / community" {
		t.Errorf("snap mini default vendor wrong: %+v", m["mini"])
	}
}

func TestParseWindowsRegistry(t *testing.T) {
	arr := `[
 {"DisplayName":"Acme App","DisplayVersion":"1.0","Publisher":"Acme Inc","URLInfoAbout":"https://acme.example"},
 {"DisplayName":"Acme App","DisplayVersion":"1.0","Publisher":"Acme Inc"},
 {"DisplayName":"   ","DisplayVersion":"x"},
 {"DisplayName":"UrlOnly","DisplayVersion":"2","URLInfoAbout":"https://github.com/foo/bar"}
]`
	comps := parseWindowsRegistry(arr)
	if len(comps) != 2 { // duplicate + blank-name dropped
		t.Fatalf("want 2 registry comps, got %d: %+v", len(comps), comps)
	}
	m := byName(comps)
	if m["Acme App"].Vendor != "Acme Inc" {
		t.Errorf("publisher vendor wrong: %+v", m["Acme App"])
	}
	if m["UrlOnly"].Vendor != "foo" { // no publisher -> from homepage (github org)
		t.Errorf("url-derived vendor wrong: %+v", m["UrlOnly"])
	}
	// single object form gets wrapped
	single := parseWindowsRegistry(`{"DisplayName":"Solo","DisplayVersion":"1"}`)
	if len(single) != 1 || single[0].Name != "Solo" {
		t.Errorf("single-object parse wrong: %+v", single)
	}
	if parseWindowsRegistry("   ") != nil {
		t.Error("blank registry output should parse to nil")
	}
	if parseWindowsRegistry("not json") != nil {
		t.Error("invalid registry JSON should parse to nil")
	}
}

func TestParseWinget(t *testing.T) {
	j := `{"Sources":[{"Packages":[{"PackageIdentifier":"Microsoft.VisualStudioCode","Version":"1.90"},{"PackageIdentifier":"Git.Git","Version":"2.45"}]}]}`
	comps := parseWinget(j)
	if len(comps) != 2 {
		t.Fatalf("want 2 winget comps, got %d", len(comps))
	}
	if comps[0].Vendor != "Microsoft" {
		t.Errorf("winget vendor wrong: %+v", comps[0])
	}
	if parseWinget("{bad") != nil {
		t.Error("invalid winget JSON should parse to nil")
	}
}

func TestParseChoco(t *testing.T) {
	comps := parseChoco("git|2.45.0\nnodejs|22.0.0\ngarbage line\n\n")
	if len(comps) != 2 {
		t.Fatalf("want 2 choco comps, got %d: %+v", len(comps), comps)
	}
	if comps[0].Name != "git" || comps[0].Version != "2.45.0" {
		t.Errorf("choco git wrong: %+v", comps[0])
	}
}

func TestParseScoop(t *testing.T) {
	comps := parseScoop(`{"apps":[{"Name":"7zip","Version":"23.01","Source":"main"},{"Name":"fzf","Version":"0.5"}]}`)
	if len(comps) != 2 {
		t.Fatalf("want 2 scoop comps, got %d", len(comps))
	}
	if comps[0].Vendor != "main" || comps[1].Vendor != "Scoop / community" {
		t.Errorf("scoop vendor wrong: %+v", comps)
	}
	if parseScoop("{bad") != nil {
		t.Error("invalid scoop JSON should parse to nil")
	}
}
