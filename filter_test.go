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
		"292y": 292 * 365 * day,
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
	// Ages beyond what a time.Duration holds must not wrap around.
	for _, in := range []string{"293y", "300y", "600y", "3000000h", "99999999999999999999d"} {
		if _, err := parseAge(in); err == nil || !strings.Contains(err.Error(), "too large") {
			t.Errorf("parseAge(%q): got %v, want a too large error", in, err)
		}
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
		{"--older-than", "300y"},
	} {
		_, stderr, code := r.runTool(args...)
		if code != exitUsage {
			t.Errorf("%v: exit %d, want %d (stderr %q)", args, code, exitUsage, stderr)
		}
	}
}

func TestCompileGlob(t *testing.T) {
	tests := []struct {
		pattern, name string
		want          bool
	}{
		{"feature/*", "feature/login", true},
		{"feature/*", "feature/a/b", true},
		{"feature/*", "feature", false},
		{"feature/*", "my-feature/x", false},
		{"release/*", "release/1.2", true},
		{"release/*", "release-1.2", false},
		{"v1.?", "v1.2", true},
		{"v1.?", "v1x2", false},
		{"v1.?", "v1.23", false},
		{"[ab]*", "bugfix", true},
		{"[!ab]*", "bugfix", false},
		{"[!ab]*", "chore", true},
		{"[!a-c]", "b", false},
		{"[!a-c]", "d", true},
		{"[!^a]", "^", false},
		{"[!^a]", "a", false},
		{"[!^a]", "b", true},
		// '^' negates in a regexp class, but in this glob it is literal.
		{"[^a]", "^", true},
		{"[^a]", "a", true},
		{"[^a]", "b", false},
		{"[^f]*", "feature/login", true},
		{"[^f]*", "main", false},
		// '\' is literal too; [\d] is not "any digit".
		{`[\d]`, `d`, true},
		{`[\d]`, `\`, true},
		{`[\d]`, "5", false},
		{"[a-z]", "m", true},
		{"[a-z]", "M", false},
		{"main", "main", true},
		{"main", "main2", false},
		{"*", "anything/at/all", true},
	}
	for _, tt := range tests {
		re, err := compileGlob(tt.pattern)
		if err != nil {
			t.Fatalf("compileGlob(%q): %v", tt.pattern, err)
		}
		if got := re.MatchString(tt.name); got != tt.want {
			t.Errorf("%q matches %q = %v, want %v", tt.pattern, tt.name, got, tt.want)
		}
	}
	for _, bad := range []string{"feat[ure", "[]"} {
		if _, err := compileGlob(bad); err == nil {
			t.Errorf("compileGlob(%q) should fail", bad)
		}
	}
}

func TestAuthorAndPattern(t *testing.T) {
	r := fixture(t)
	r.git("switch", "-q", "-c", "bugfix/crash", "main~2")
	r.commitBy("Alice Example <alice+dev@corp.example>", "crash.txt", 200*day)
	r.git("switch", "-q", "main")

	tests := []struct {
		args []string
		want []string
	}{
		{[]string{"--author", "Alice"}, []string{"bugfix/crash"}},
		{[]string{"--author", "@corp\\.example>"}, []string{"bugfix/crash"}},
		{[]string{"--author", `\Qalice+dev@corp.example\E`}, []string{"bugfix/crash"}},
		{[]string{"--author", "^Test User"}, []string{"feature/merged", "feature/old"}},
		{[]string{"--author", "(?i)alice", "--merged"}, nil},
		{[]string{"--pattern", "feature/*"}, []string{"feature/merged", "feature/old"}},
		{[]string{"--pattern", "feature/*", "--all"}, []string{"feature/merged", "feature/old", "feature/new"}},
		{[]string{"--pattern", "bugfix/*", "--pattern", "feature/o*"}, []string{"bugfix/crash", "feature/old"}},
		{[]string{"--pattern", "*", "--unmerged", "--author", "Test"}, []string{"feature/old", "feature/new"}},
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
	}

	for _, args := range [][]string{{"--author", "("}, {"--pattern", "feat[ure"}} {
		if _, _, code := r.runTool(args...); code != exitUsage {
			t.Errorf("%v: exit %d, want %d", args, code, exitUsage)
		}
	}
}
