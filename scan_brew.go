// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"time"
)

// brew info --json=v2 --installed carries description, homepage, dependencies
// and the installed version for every formula and cask — all offline.
type brewInfo struct {
	Formulae []struct {
		Name         string   `json:"name"`
		Desc         string   `json:"desc"`
		Homepage     string   `json:"homepage"`
		License      string   `json:"license"` // SPDX expression, may be null
		Dependencies []string `json:"dependencies"`
		Installed    []struct {
			Version string `json:"version"`
		} `json:"installed"`
	} `json:"formulae"`
	Casks []struct {
		Token     string   `json:"token"`
		Name      []string `json:"name"`
		Desc      string   `json:"desc"`
		Homepage  string   `json:"homepage"`
		Version   string   `json:"version"`
		DependsOn struct {
			Formula []string `json:"formula"`
		} `json:"depends_on"`
	} `json:"casks"`
}

func scanBrew() []Component {
	out, ok := runIn(60*time.Second, "brew", "info", "--json=v2", "--installed")
	if !ok {
		return nil
	}
	return parseBrew(out)
}

// parseBrew is the pure parser for `brew info --json=v2 --installed` output.
func parseBrew(out string) []Component {
	var bi brewInfo
	if err := json.Unmarshal([]byte(out), &bi); err != nil {
		return nil
	}
	var comps []Component
	for _, f := range bi.Formulae {
		ver := ""
		if len(f.Installed) > 0 {
			ver = f.Installed[0].Version
		}
		c := Component{
			Name: f.Name, Version: ver, Source: "Homebrew (formula)",
			Desc: f.Desc, Homepage: f.Homepage, Deps: f.Dependencies,
			Vendor: vendorFromHomepage(f.Homepage), License: f.License,
		}
		c.Category = classifyCategory(c)
		comps = append(comps, c)
	}
	for _, k := range bi.Casks {
		name := k.Token
		if len(k.Name) > 0 && k.Name[0] != "" {
			name = k.Name[0]
		}
		c := Component{
			Name: name, Version: k.Version, Source: "Homebrew (cask)",
			Desc: k.Desc, Homepage: k.Homepage, Deps: k.DependsOn.Formula,
			Vendor: vendorFromHomepage(k.Homepage),
		}
		c.Category = classifyCategory(c)
		comps = append(comps, c)
	}
	return comps
}
