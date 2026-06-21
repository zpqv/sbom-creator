// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func sampleCtx() genCtx {
	return genCtx{
		comps: []Component{
			{Name: "jq", Version: "1.8", Category: "Developer Tools", Source: "Homebrew (formula)",
				Vendor: "jq", Desc: "JSON processor", Homepage: "https://jqlang.github.io/jq/",
				License: "MIT", PURL: "pkg:brew/jq@1.8"},
			{Name: "Some App", Version: "2.0", Category: "Applications", Source: "Direct install",
				Vendor: "Acme"}, // no purl, no license, no homepage
		},
		host: "host1", osLabel: "macOS 15 (arm64)",
		date: "2026-06-21 16:00", timestamp: "2026-06-21T16:00:00Z",
	}
}

func TestCycloneDX(t *testing.T) {
	var bom map[string]any
	if err := json.Unmarshal(cycloneDX(sampleCtx()), &bom); err != nil {
		t.Fatalf("invalid CycloneDX JSON: %v", err)
	}
	if bom["bomFormat"] != "CycloneDX" || bom["specVersion"] != "1.6" {
		t.Errorf("header wrong: %v / %v", bom["bomFormat"], bom["specVersion"])
	}
	comps := bom["components"].([]any)
	if len(comps) != 2 {
		t.Fatalf("want 2 components, got %d", len(comps))
	}
	jq := comps[0].(map[string]any)
	if jq["type"] != "library" || jq["purl"] != "pkg:brew/jq@1.8" {
		t.Errorf("jq component wrong: %v", jq)
	}
	lic := jq["licenses"].([]any)[0].(map[string]any)["license"].(map[string]any)
	if lic["name"] != "MIT" {
		t.Errorf("license wrong: %v", lic)
	}
	if jq["externalReferences"].([]any)[0].(map[string]any)["url"] != "https://jqlang.github.io/jq/" {
		t.Errorf("externalReferences wrong: %v", jq["externalReferences"])
	}
	// the bare app has no purl -> type application, and no licenses/extRefs keys
	app := comps[1].(map[string]any)
	if app["type"] != "application" {
		t.Errorf("app type = %v, want application", app["type"])
	}
	if _, ok := app["licenses"]; ok {
		t.Error("app should have no licenses key")
	}
	if _, ok := app["purl"]; ok {
		t.Error("app should have no purl key")
	}
}

func TestCdxType(t *testing.T) {
	if cdxType(Component{PURL: "pkg:brew/x@1"}) != "library" {
		t.Error("purl -> library")
	}
	if cdxType(Component{}) != "application" {
		t.Error("no purl -> application")
	}
}

func TestProps(t *testing.T) {
	p := props(Component{Source: "s", Category: "c", Vendor: ""})
	if len(p) != 2 { // vendor empty -> skipped
		t.Fatalf("want 2 props, got %d: %v", len(p), p)
	}
	if p[0].Name != "sbom-creator:source" || p[0].Value != "s" {
		t.Errorf("first prop wrong: %v", p[0])
	}
	if props(Component{}) != nil {
		t.Error("all-empty component should yield no props")
	}
}

func TestSPDX(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(spdx(sampleCtx()), &doc); err != nil {
		t.Fatalf("invalid SPDX JSON: %v", err)
	}
	if doc["spdxVersion"] != "SPDX-2.3" || doc["dataLicense"] != "CC0-1.0" {
		t.Errorf("header wrong: %v", doc)
	}
	pkgs := doc["packages"].([]any)
	if len(pkgs) != 2 {
		t.Fatalf("want 2 packages, got %d", len(pkgs))
	}
	jq := pkgs[0].(map[string]any)
	if jq["SPDXID"] != "SPDXRef-Package-1" || jq["licenseDeclared"] != "MIT" {
		t.Errorf("jq package wrong: %v", jq)
	}
	if jq["supplier"] != "Organization: jq" {
		t.Errorf("supplier wrong: %v", jq["supplier"])
	}
	ext := jq["externalRefs"].([]any)[0].(map[string]any)
	if ext["referenceType"] != "purl" || ext["referenceLocator"] != "pkg:brew/jq@1.8" {
		t.Errorf("externalRefs wrong: %v", ext)
	}
	// bare app: no license -> NOASSERTION, no externalRefs
	app := pkgs[1].(map[string]any)
	if app["licenseDeclared"] != "NOASSERTION" {
		t.Errorf("app licenseDeclared = %v, want NOASSERTION", app["licenseDeclared"])
	}
	if _, ok := app["externalRefs"]; ok {
		t.Error("app should have no externalRefs")
	}
}

func TestSPDXEmptyHost(t *testing.T) {
	ctx := sampleCtx()
	ctx.host = ""
	var doc map[string]any
	json.Unmarshal(spdx(ctx), &doc)
	if doc["name"] != "host-sbom" {
		t.Errorf("empty host should fall back to 'host', got name=%v", doc["name"])
	}
}

func TestSPDXLicense(t *testing.T) {
	cases := map[string]string{
		"MIT":               "MIT",
		"Apache-2.0":        "Apache-2.0",
		"MIT OR Apache-2.0": "MIT OR Apache-2.0",
		"GPL-2.0-only WITH Classpath-exception-2.0": "GPL-2.0-only WITH Classpath-exception-2.0",
		"":                          "NOASSERTION",
		"BSD License, see LICENSE":  "NOASSERTION", // free text
		"This software is provided": "NOASSERTION",
	}
	for in, want := range cases {
		if got := spdxLicense(in); got != want {
			t.Errorf("spdxLicense(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGeneratorsAndFormatList(t *testing.T) {
	ctx := sampleCtx()
	if !strings.Contains(string(generators["html"](ctx)), "const DATA =") {
		t.Error("html generator missing DATA")
	}
	var arr []Component
	if err := json.Unmarshal(generators["json"](ctx), &arr); err != nil || len(arr) != 2 {
		t.Errorf("json generator wrong: %v len=%d", err, len(arr))
	}
	if !strings.Contains(string(generators["cyclonedx"](ctx)), "CycloneDX") {
		t.Error("cyclonedx generator wrong")
	}
	if !strings.Contains(string(generators["spdx"](ctx)), "SPDX-2.3") {
		t.Error("spdx generator wrong")
	}
	if formatList() != "html, json, cyclonedx, spdx" {
		t.Errorf("formatList = %q", formatList())
	}
}
