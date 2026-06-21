// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// execCommand is the single seam through which all external commands are
// spawned. It is context-aware so timeouts are enforced safely by os/exec
// (which kills the process in coordination with Wait — no manual Kill race).
// Tests swap it for a stub that returns canned tool output.
var execCommand = exec.CommandContext

// lookPath is the seam for binary discovery. Tests override it so a tool's
// presence is deterministic on every OS (real exec.LookPath honors PATHEXT on
// Windows but not POSIX shell scripts, which would make coverage OS-dependent).
var lookPath = exec.LookPath

// have reports whether a binary is resolvable on PATH.
func have(bin string) bool {
	_, err := lookPath(bin)
	return err == nil
}

// run executes a command and returns its stdout, or ("", false) on error.
// All scanning is local: these commands query already-installed package
// metadata and never touch the network.
func run(name string, args ...string) (string, bool) {
	out, err := execCommand(context.Background(), name, args...).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// runIn is run() with a deadline for commands that can be slow (registry walks,
// large `pip show` batches). On timeout the context cancellation kills the
// process and Output returns an error, which we map to ok=false.
func runIn(timeout time.Duration, name string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := execCommand(ctx, name, args...).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

func relevant(s Scanner, goos string) bool {
	if len(s.OS) == 0 {
		return true
	}
	for _, o := range s.OS {
		if o == goos {
			return true
		}
	}
	return false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range in {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// osLabel returns a friendly OS string for the report header.
func osLabel() string { return osLabelFor(runtime.GOOS, runtime.GOARCH) }

// osLabelFor is the testable core of osLabel, parameterized on the target OS
// and architecture so every branch is reachable from a single host.
func osLabelFor(goos, goarch string) string {
	switch goos {
	case "darwin":
		if v, ok := run("sw_vers", "-productVersion"); ok {
			return "macOS " + strings.TrimSpace(v) + " (" + goarch + ")"
		}
		return "macOS (" + goarch + ")"
	case "windows":
		return "Windows (" + goarch + ")"
	case "linux":
		if v, ok := run("sh", "-c", ". /etc/os-release 2>/dev/null && echo $PRETTY_NAME"); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v) + " (" + goarch + ")"
		}
		return "Linux (" + goarch + ")"
	}
	return goos + " (" + goarch + ")"
}
