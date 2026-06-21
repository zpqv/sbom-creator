// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"regexp"
	"strings"
)

// genCtx carries everything a serializer might need. Keeping it in one struct
// lets every output format share a single dispatch signature.
type genCtx struct {
	comps     []Component
	missing   []MissingTool
	host      string
	osLabel   string
	date      string // human display, e.g. "2026-06-21 16:00"
	timestamp string // RFC3339, for machine formats
}

// generators maps an output format to its serializer. realMain validates the
// requested -format against these keys, so there is no unreachable default.
var generators = map[string]func(genCtx) []byte{
	"html":      func(c genCtx) []byte { return []byte(renderHTML(c.comps, c.missing, c.host, c.osLabel, c.date)) },
	"json":      func(c genCtx) []byte { b, _ := json.MarshalIndent(c.comps, "", "  "); return b },
	"cyclonedx": func(c genCtx) []byte { return cycloneDX(c) },
	"spdx":      func(c genCtx) []byte { return spdx(c) },
}

// ---------------------------------------------------------------------------
// CycloneDX 1.6 (JSON)
// ---------------------------------------------------------------------------

type cdxBOM struct {
	BOMFormat   string    `json:"bomFormat"`
	SpecVersion string    `json:"specVersion"`
	Version     int       `json:"version"`
	Metadata    cdxMeta   `json:"metadata"`
	Components  []cdxComp `json:"components"`
}

type cdxMeta struct {
	Timestamp string     `json:"timestamp,omitempty"`
	Tools     cdxTools   `json:"tools"`
	Component cdxSubject `json:"component"`
}

type cdxTools struct {
	Components []cdxComp `json:"components"`
}

type cdxSubject struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type cdxComp struct {
	Type         string       `json:"type"`
	Name         string       `json:"name"`
	Version      string       `json:"version,omitempty"`
	Description  string       `json:"description,omitempty"`
	PURL         string       `json:"purl,omitempty"`
	Licenses     []cdxLicense `json:"licenses,omitempty"`
	ExternalRefs []cdxExtRef  `json:"externalReferences,omitempty"`
	Properties   []cdxProp    `json:"properties,omitempty"`
}

type cdxLicense struct {
	License cdxLicenseName `json:"license"`
}
type cdxLicenseName struct {
	Name string `json:"name"`
}
type cdxExtRef struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}
type cdxProp struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// cdxType classifies a component: anything addressable as a package (it has a
// purl) is a library; everything else (GUI apps, OS registry entries) is an
// application.
func cdxType(c Component) string {
	if c.PURL != "" {
		return "library"
	}
	return "application"
}

func cycloneDX(c genCtx) []byte {
	bom := cdxBOM{
		BOMFormat:   "CycloneDX",
		SpecVersion: "1.6",
		Version:     1,
		Metadata: cdxMeta{
			Timestamp: c.timestamp,
			Tools:     cdxTools{Components: []cdxComp{{Type: "application", Name: "sbom-creator", Version: version}}},
			Component: cdxSubject{Type: "device", Name: c.host},
		},
		Components: make([]cdxComp, 0, len(c.comps)),
	}
	for _, comp := range c.comps {
		cc := cdxComp{
			Type:        cdxType(comp),
			Name:        comp.Name,
			Version:     comp.Version,
			Description: comp.Desc,
			PURL:        comp.PURL,
			Properties:  props(comp),
		}
		if comp.License != "" {
			cc.Licenses = []cdxLicense{{License: cdxLicenseName{Name: comp.License}}}
		}
		if comp.Homepage != "" {
			cc.ExternalRefs = []cdxExtRef{{Type: "website", URL: comp.Homepage}}
		}
		bom.Components = append(bom.Components, cc)
	}
	b, _ := json.MarshalIndent(bom, "", "  ")
	return b
}

// props records the inventory's source/category/vendor as CycloneDX properties,
// skipping any that are empty.
func props(c Component) []cdxProp {
	var p []cdxProp
	for _, kv := range [][2]string{
		{"sbom-creator:source", c.Source},
		{"sbom-creator:category", c.Category},
		{"sbom-creator:vendor", c.Vendor},
	} {
		if kv[1] != "" {
			p = append(p, cdxProp{Name: kv[0], Value: kv[1]})
		}
	}
	return p
}

// ---------------------------------------------------------------------------
// SPDX 2.3 (JSON)
// ---------------------------------------------------------------------------

type spdxDoc struct {
	SPDXVersion       string       `json:"spdxVersion"`
	DataLicense       string       `json:"dataLicense"`
	SPDXID            string       `json:"SPDXID"`
	Name              string       `json:"name"`
	DocumentNamespace string       `json:"documentNamespace"`
	CreationInfo      spdxCreation `json:"creationInfo"`
	Packages          []spdxPkg    `json:"packages"`
}

type spdxCreation struct {
	Created  string   `json:"created"`
	Creators []string `json:"creators"`
}

type spdxPkg struct {
	SPDXID           string       `json:"SPDXID"`
	Name             string       `json:"name"`
	VersionInfo      string       `json:"versionInfo,omitempty"`
	DownloadLocation string       `json:"downloadLocation"`
	LicenseConcluded string       `json:"licenseConcluded"`
	LicenseDeclared  string       `json:"licenseDeclared"`
	Supplier         string       `json:"supplier,omitempty"`
	Homepage         string       `json:"homepage,omitempty"`
	ExternalRefs     []spdxExtRef `json:"externalRefs,omitempty"`
}

type spdxExtRef struct {
	ReferenceCategory string `json:"referenceCategory"`
	ReferenceType     string `json:"referenceType"`
	ReferenceLocator  string `json:"referenceLocator"`
}

// simpleSPDXExpr matches a license string that is safe to emit as an SPDX
// license expression (ids joined by AND/OR/WITH). Anything else (free text,
// full license bodies) becomes NOASSERTION so validators don't choke.
var simpleSPDXExpr = regexp.MustCompile(`^[A-Za-z0-9.+_-]+( (OR|AND|WITH) [A-Za-z0-9.+_-]+)*$`)

func spdxLicense(raw string) string {
	if raw != "" && simpleSPDXExpr.MatchString(raw) {
		return raw
	}
	return "NOASSERTION"
}

func spdx(c genCtx) []byte {
	host := c.host
	if host == "" {
		host = "host"
	}
	doc := spdxDoc{
		SPDXVersion:       "SPDX-2.3",
		DataLicense:       "CC0-1.0",
		SPDXID:            "SPDXRef-DOCUMENT",
		Name:              host + "-sbom",
		DocumentNamespace: "https://github.com/zpqv/sbom-creator/spdx/" + purlEscape(host) + "/" + purlEscape(c.timestamp),
		CreationInfo: spdxCreation{
			Created:  c.timestamp,
			Creators: []string{"Tool: sbom-creator-" + version},
		},
		Packages: make([]spdxPkg, 0, len(c.comps)),
	}
	for i, comp := range c.comps {
		p := spdxPkg{
			SPDXID:           "SPDXRef-Package-" + itoa(i+1),
			Name:             comp.Name,
			VersionInfo:      comp.Version,
			DownloadLocation: "NOASSERTION",
			LicenseConcluded: "NOASSERTION",
			LicenseDeclared:  spdxLicense(comp.License),
			Homepage:         comp.Homepage,
		}
		if comp.Vendor != "" {
			p.Supplier = "Organization: " + comp.Vendor
		}
		if comp.PURL != "" {
			p.ExternalRefs = []spdxExtRef{{
				ReferenceCategory: "PACKAGE-MANAGER",
				ReferenceType:     "purl",
				ReferenceLocator:  comp.PURL,
			}}
		}
		doc.Packages = append(doc.Packages, p)
	}
	b, _ := json.MarshalIndent(doc, "", "  ")
	return b
}

// formatList is the valid -format values in a stable order, for help/errors.
func formatList() string {
	var out []string
	for _, f := range []string{"html", "json", "cyclonedx", "spdx"} {
		if _, ok := generators[f]; ok {
			out = append(out, f)
		}
	}
	return strings.Join(out, ", ")
}
