// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestHave(t *testing.T) {
	if !have(os.Args[0]) { // the test binary is an executable path
		t.Error("have(test binary) should be true")
	}
	if have("definitely-not-a-real-binary-zzz999") {
		t.Error("have(nonexistent) should be false")
	}
}

func TestRun(t *testing.T) {
	fakeExec(t, map[string]fakeCmd{"brew": ok("hello world")})
	if out, okk := run("brew", "anything"); !okk || out != "hello world" {
		t.Errorf("run success = %q,%v", out, okk)
	}
	if _, okk := run("missing-tool"); okk {
		t.Error("run of failing command should report ok=false")
	}
}

func TestRunInSuccessAndTimeout(t *testing.T) {
	fakeExec(t, map[string]fakeCmd{
		"fast": ok("done"),
		"slow": {stdout: "late", exit: 0, sleepMS: 400},
	})
	if out, okk := runIn(2*time.Second, "fast"); !okk || out != "done" {
		t.Errorf("runIn success = %q,%v", out, okk)
	}
	if _, okk := runIn(5*time.Millisecond, "slow"); okk {
		t.Error("runIn should time out and report ok=false")
	}
}

func TestRelevant(t *testing.T) {
	if !relevant(Scanner{}, "linux") {
		t.Error("empty OS list should be relevant everywhere")
	}
	if !relevant(Scanner{OS: []string{"linux", "darwin"}}, "darwin") {
		t.Error("matching OS should be relevant")
	}
	if relevant(Scanner{OS: []string{"linux"}}, "windows") {
		t.Error("non-matching OS should not be relevant")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "   ", "  x ", "y"); got != "x" {
		t.Errorf("firstNonEmpty = %q, want x", got)
	}
	if got := firstNonEmpty("", "  "); got != "" {
		t.Errorf("firstNonEmpty(all empty) = %q, want empty", got)
	}
}

func TestDedupeStrings(t *testing.T) {
	if got := dedupeStrings([]string{"a", "a", "", "b", "b"}); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("dedupeStrings = %v", got)
	}
	if got := dedupeStrings(nil); got != nil {
		t.Errorf("dedupeStrings(nil) = %v, want nil", got)
	}
}

func TestOSLabelFor(t *testing.T) {
	// darwin: with and without sw_vers
	fakeExec(t, map[string]fakeCmd{"sw_vers": ok("15.0\n")})
	if got := osLabelFor("darwin", "arm64"); got != "macOS 15.0 (arm64)" {
		t.Errorf("darwin label = %q", got)
	}
	fakeExec(t, map[string]fakeCmd{}) // sw_vers missing -> failure
	if got := osLabelFor("darwin", "arm64"); got != "macOS (arm64)" {
		t.Errorf("darwin fallback label = %q", got)
	}
	// windows is pure
	if got := osLabelFor("windows", "amd64"); got != "Windows (amd64)" {
		t.Errorf("windows label = %q", got)
	}
	// linux: os-release present, then empty -> fallback
	fakeExec(t, map[string]fakeCmd{"sh": ok("Ubuntu 24.04 LTS\n")})
	if got := osLabelFor("linux", "amd64"); got != "Ubuntu 24.04 LTS (amd64)" {
		t.Errorf("linux label = %q", got)
	}
	fakeExec(t, map[string]fakeCmd{"sh": ok("\n")}) // empty PRETTY_NAME
	if got := osLabelFor("linux", "amd64"); got != "Linux (amd64)" {
		t.Errorf("linux fallback label = %q", got)
	}
	// unknown OS default branch
	if got := osLabelFor("plan9", "arm"); got != "plan9 (arm)" {
		t.Errorf("default label = %q", got)
	}
	// the production wrapper should not panic and should mention the arch
	if osLabel() == "" {
		t.Error("osLabel() returned empty")
	}
}

func TestPipBin(t *testing.T) {
	fakeLookPath(t) // nothing present
	if got := pipBin(); got != "" {
		t.Errorf("pipBin with no python = %q, want empty", got)
	}
	fakeLookPath(t, "pip3")
	if got := pipBin(); got != "pip3" {
		t.Errorf("pipBin = %q, want pip3", got)
	}
	fakeLookPath(t, "pip") // only the fallback name present
	if got := pipBin(); got != "pip" {
		t.Errorf("pipBin fallback = %q, want pip", got)
	}
}
