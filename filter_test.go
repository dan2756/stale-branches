package main

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParseAge(t *testing.T) {
	good := map[string]time.Duration{
		"12h":  12 * time.Hour,
		"90d":  90 * day,
		"6w":   42 * day,
		"3mo":  90 * day,
		"1y":   365 * day,
		"100d": 100 * day,
	}
	for in, want := range good {
		got, err := parseAge(in)
		if err != nil || got != want {
			t.Errorf("parseAge(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "90", "d", "3m", "1.5y", "-1d", "0d", "90 d", "1yr"} {
		if _, err := parseAge(in); err == nil {
			t.Errorf("parseAge(%q) should fail", in)
		}
	}
	if _, err := parseAge("3m"); err == nil || !strings.Contains(err.Error(), `"mo"`) {
		t.Errorf("3m should suggest mo, got %v", err)
	}
}

func TestFilters(t *testing.T) {
	r := fixture(t)
	tests := []struct {
		args []string
		want []string
	}{
		{nil, []string{"feature/merged", "feature/old"}},
		{[]string{"--all"}, []string{"feature/merged", "feature/old", "feature/new", "main"}},
		{[]string{"--merged"}, []string{"feature/merged"}},
		{[]string{"--unmerged"}, []string{"feature/old", "feature/new"}},
		{[]string{"--older-than", "100d"}, []string{"feature/merged", "feature/old"}},
		{[]string{"--older-than", "20w"}, []string{"feature/merged"}},
		{[]string{"--older-than=1d"}, []string{"feature/merged", "feature/old", "feature/new"}},
		{[]string{"--unmerged", "--older-than", "1mo"}, []string{"feature/old"}},
		{[]string{"--merged", "--older-than", "1y"}, nil},
	}
	for _, tt := range tests {
		out, stderr, code := r.runTool(tt.args...)
		if code != exitOK {
			t.Errorf("%v: exit %d: %s", tt.args, code, stderr)
			continue
		}
		if got := tableNames(out); !slices.Equal(got, tt.want) {
			t.Errorf("%v: got %v, want %v", tt.args, got, tt.want)
		}
		if tt.want == nil && !strings.Contains(stderr, "no matching branches") {
			t.Errorf("%v: stderr %q", tt.args, stderr)
		}
	}
}

func TestFilterUsageErrors(t *testing.T) {
	r := fixture(t)
	for _, args := range [][]string{
		{"--merged", "--unmerged"},
		{"--older-than", "soon"},
		{"--older-than", "3m"},
	} {
		_, stderr, code := r.runTool(args...)
		if code != exitUsage {
			t.Errorf("%v: exit %d, want %d (stderr %q)", args, code, exitUsage, stderr)
		}
	}
}
