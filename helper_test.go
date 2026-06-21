// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

// TestHelperProcess is not a real test. It is the child process used to stand
// in for external commands (brew, npm, plutil, …). It echoes the stdout passed
// via the environment and exits with the requested code, optionally sleeping
// first so the runIn timeout path can be exercised. This is the standard
// os/exec testing technique: os.Exit bypasses the test framework's own output,
// so the parent captures exactly HELPER_STDOUT and nothing else.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if ms := os.Getenv("HELPER_SLEEP_MS"); ms != "" {
		d, _ := strconv.Atoi(ms)
		time.Sleep(time.Duration(d) * time.Millisecond)
	}
	fmt.Fprint(os.Stdout, os.Getenv("HELPER_STDOUT"))
	code, _ := strconv.Atoi(os.Getenv("HELPER_EXIT"))
	os.Exit(code)
}

type fakeCmd struct {
	stdout  string
	exit    int
	sleepMS int
}

// fakeExecFunc swaps execCommand for one that derives a canned result from the
// full (name, args) invocation. This is the general form; fakeExec is the
// common name-keyed convenience built on top. Restored automatically.
func fakeExecFunc(t *testing.T, fn func(name string, args []string) fakeCmd) {
	t.Helper()
	orig := execCommand
	t.Cleanup(func() { execCommand = orig })
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		fc := fn(name, args)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestHelperProcess")
		cmd.Env = []string{
			"GO_WANT_HELPER_PROCESS=1",
			"HELPER_STDOUT=" + fc.stdout,
			"HELPER_EXIT=" + strconv.Itoa(fc.exit),
		}
		if fc.sleepMS > 0 {
			cmd.Env = append(cmd.Env, "HELPER_SLEEP_MS="+strconv.Itoa(fc.sleepMS))
		}
		return cmd
	}
}

// fakeExec routes by binary name. Unknown binaries behave as a failed command
// (exit 1, no output), modeling "tool not installed / errored".
func fakeExec(t *testing.T, byBin map[string]fakeCmd) {
	t.Helper()
	fakeExecFunc(t, func(name string, _ []string) fakeCmd {
		if fc, ok := byBin[name]; ok {
			return fc
		}
		return fakeCmd{exit: 1}
	})
}

// ok is a fakeCmd that succeeds with the given stdout.
func ok(stdout string) fakeCmd { return fakeCmd{stdout: stdout, exit: 0} }

// fakeLookPath overrides binary discovery so the listed names report present
// and everything else absent — deterministic on every OS, unlike dropping real
// shell scripts on PATH (which Windows' LookPath ignores). Restored on cleanup.
func fakeLookPath(t *testing.T, present ...string) {
	t.Helper()
	set := map[string]bool{}
	for _, p := range present {
		set[p] = true
	}
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(bin string) (string, error) {
		if set[bin] {
			return "/fake/" + bin, nil
		}
		return "", exec.ErrNotFound
	}
}
