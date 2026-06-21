// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Each scanX wrapper is covered for both outcomes: the backing tool errors
// (-> nil) and the tool succeeds (-> parsed components). The success path also
// exercises the real run/runIn code through the helper-process mock.

func TestScanBrew(t *testing.T) {
	fakeExec(t, map[string]fakeCmd{"brew": ok(brewJSON)})
	if got := scanBrew(); len(got) != 4 {
		t.Errorf("scanBrew success = %d comps", len(got))
	}
	fakeExec(t, map[string]fakeCmd{})
	if scanBrew() != nil {
		t.Error("scanBrew failure should be nil")
	}
}

func TestScanNpm(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "p"), 0o755)
	os.WriteFile(filepath.Join(root, "p", "package.json"), []byte(`{"name":"p","version":"1"}`), 0o644)
	fakeExec(t, map[string]fakeCmd{"npm": ok(root + "\n")})
	if got := scanNpm(); len(got) != 1 {
		t.Errorf("scanNpm success = %d comps", len(got))
	}
	fakeExec(t, map[string]fakeCmd{})
	if scanNpm() != nil {
		t.Error("scanNpm failure should be nil")
	}
}

func TestScanPip(t *testing.T) {
	fakeLookPath(t, "pip3") // pipBin discovers pip3 deterministically on any OS

	// Route list vs show to different canned outputs by inspecting args.
	fakeExecFunc(t, func(name string, args []string) fakeCmd {
		if name != "pip3" {
			return fakeCmd{exit: 1}
		}
		if len(args) > 0 && args[0] == "list" {
			return ok(pipList)
		}
		return ok(pipShow) // show
	})
	if got := scanPip(); len(got) != 2 {
		t.Errorf("scanPip success = %d comps", len(got))
	}

	// list fails -> nil
	fakeExec(t, map[string]fakeCmd{}) // pip3 invocation fails
	if scanPip() != nil {
		t.Error("scanPip with failing list should be nil")
	}
	// list succeeds but is unparseable -> pipNames false -> nil
	fakeExec(t, map[string]fakeCmd{"pip3": ok("not json")})
	if scanPip() != nil {
		t.Error("scanPip with invalid list JSON should be nil")
	}
	// pipBin == "" path (no python on PATH)
	fakeLookPath(t)
	if scanPip() != nil {
		t.Error("scanPip with no pip should be nil")
	}
}

func TestScanGemCargo(t *testing.T) {
	fakeExec(t, map[string]fakeCmd{"gem": ok(gemDetails)})
	if got := scanGem(); len(got) != 2 {
		t.Errorf("scanGem = %d", len(got))
	}
	fakeExec(t, map[string]fakeCmd{})
	if scanGem() != nil {
		t.Error("scanGem failure should be nil")
	}

	fakeExec(t, map[string]fakeCmd{"cargo": ok("ripgrep v14.0.0:\n    rg\n")})
	if got := scanCargo(); len(got) != 1 {
		t.Errorf("scanCargo = %d", len(got))
	}
	fakeExec(t, map[string]fakeCmd{})
	if scanCargo() != nil {
		t.Error("scanCargo failure should be nil")
	}
}

func TestScanLinux(t *testing.T) {
	fakeExec(t, map[string]fakeCmd{"dpkg-query": ok("vim\t9.1\teditor\thttps://vim.org\n")})
	if got := scanDpkg(); len(got) != 1 {
		t.Errorf("scanDpkg = %d", len(got))
	}
	fakeExec(t, map[string]fakeCmd{})
	if scanDpkg() != nil {
		t.Error("scanDpkg failure should be nil")
	}

	fakeExec(t, map[string]fakeCmd{"rpm": ok("zlib\t1.3\tlib\thttps://zlib.net\n")})
	if got := scanRpm(); len(got) != 1 {
		t.Errorf("scanRpm = %d", len(got))
	}
	fakeExec(t, map[string]fakeCmd{})
	if scanRpm() != nil {
		t.Error("scanRpm failure should be nil")
	}

	fakeExec(t, map[string]fakeCmd{"flatpak": ok("GIMP\torg.gimp.GIMP\t2.10\tEditor\n")})
	if got := scanFlatpak(); len(got) != 1 {
		t.Errorf("scanFlatpak = %d", len(got))
	}
	fakeExec(t, map[string]fakeCmd{})
	if scanFlatpak() != nil {
		t.Error("scanFlatpak failure should be nil")
	}

	fakeExec(t, map[string]fakeCmd{"snap": ok("Name Version Rev Tracking Publisher Notes\ncore 16 1 latest canonical base\n")})
	if got := scanSnap(); len(got) != 1 {
		t.Errorf("scanSnap = %d", len(got))
	}
	fakeExec(t, map[string]fakeCmd{})
	if scanSnap() != nil {
		t.Error("scanSnap failure should be nil")
	}
}

func TestScanWindows(t *testing.T) {
	// psRun discovers powershell via the lookPath seam (deterministic on any OS).
	fakeLookPath(t, "powershell")
	fakeExec(t, map[string]fakeCmd{"powershell": ok(`[{"DisplayName":"A","DisplayVersion":"1","Publisher":"P"}]`)})
	if got := scanWindowsRegistry(); len(got) != 1 {
		t.Errorf("scanWindowsRegistry = %d", len(got))
	}
	// No powershell/pwsh present -> psRun false -> nil
	fakeLookPath(t)
	if scanWindowsRegistry() != nil {
		t.Error("scanWindowsRegistry without powershell should be nil")
	}
	fakeLookPath(t, "powershell") // restore presence for the remaining checks

	fakeExec(t, map[string]fakeCmd{"winget": ok(`{"Sources":[{"Packages":[{"PackageIdentifier":"Git.Git","Version":"2"}]}]}`)})
	if got := scanWinget(); len(got) != 1 {
		t.Errorf("scanWinget = %d", len(got))
	}
	fakeExec(t, map[string]fakeCmd{})
	if scanWinget() != nil {
		t.Error("scanWinget failure should be nil")
	}

	// choco: primary command fails, fallback succeeds
	fakeExecFunc(t, func(name string, args []string) fakeCmd {
		if name == "choco" && !strings.Contains(strings.Join(args, " "), "local-only") {
			return ok("git|2.45\n")
		}
		return fakeCmd{exit: 1}
	})
	if got := scanChoco(); len(got) != 1 {
		t.Errorf("scanChoco fallback = %d", len(got))
	}
	fakeExec(t, map[string]fakeCmd{}) // both choco invocations fail
	if scanChoco() != nil {
		t.Error("scanChoco total failure should be nil")
	}

	fakeExec(t, map[string]fakeCmd{"scoop": ok(`{"apps":[{"Name":"fzf","Version":"0.5"}]}`)})
	if got := scanScoop(); len(got) != 1 {
		t.Errorf("scanScoop = %d", len(got))
	}
	fakeExec(t, map[string]fakeCmd{})
	if scanScoop() != nil {
		t.Error("scanScoop failure should be nil")
	}
}
