package main

import (
	"strings"
	"testing"
)

func TestUsageAndVersion(t *testing.T) {
	r := newTestRepo(t)
	for _, arg := range []string{"-h", "--help"} {
		out, _, code := r.runTool(arg)
		if code != exitOK || !strings.HasPrefix(out, "stale-branches - ") {
			t.Errorf("%s: exit %d, out %q", arg, code, out)
		}
	}
	out, _, code := r.runTool("--version")
	if code != exitOK || !strings.HasPrefix(out, "stale-branches ") {
		t.Errorf("--version: exit %d, out %q", code, out)
	}
}

func TestUsageErrors(t *testing.T) {
	r := fixture(t)
	for _, args := range [][]string{
		{"--no-such-flag"},
		{"extra-arg"},
		{"--base"},
	} {
		_, stderr, code := r.runTool(args...)
		if code != exitUsage {
			t.Errorf("%v: exit %d, want %d (stderr %q)", args, code, exitUsage, stderr)
		}
	}
}
