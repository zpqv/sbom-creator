// SPDX-License-Identifier: Apache-2.0

package main

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed template.html
var templateHTML string

// renderHTML produces the full self-contained webpage as a string. The template
// is embedded in the binary and the component data is injected as JSON into an
// inline <script>; Go's encoding/json escapes <, > and & to \u00xx, so the
// payload cannot break out of the script element. Kept pure (no file I/O) so it
// can be fuzzed for injection safety. json.Marshal of these concrete types
// (only string/[]string/int fields) is infallible, so its error is discarded.
func renderHTML(comps []Component, missing []MissingTool, host, osLabel, date string) string {
	data, _ := json.Marshal(comps)
	miss, _ := json.Marshal(missing)
	out := templateHTML
	out = strings.ReplaceAll(out, "__DATA__", string(data))
	out = strings.ReplaceAll(out, "__MISSING__", string(miss))
	out = strings.ReplaceAll(out, "__TOTAL__", itoa(len(comps)))
	out = strings.ReplaceAll(out, "__HOST__", htmlEscape(host))
	out = strings.ReplaceAll(out, "__OSLABEL__", htmlEscape(osLabel))
	out = strings.ReplaceAll(out, "__DATE__", htmlEscape(date))
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
