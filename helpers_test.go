package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// refTime is "now" for every test, so ages are deterministic.
var refTime = time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

const day = 24 * time.Hour

func TestMain(m *testing.M) {
	// Keep the developer's own git config out of the tests.
	empty, err := os.CreateTemp("", "stale-branches-gitconfig")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	empty.Close()
	os.Setenv("GIT_CONFIG_GLOBAL", empty.Name())
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	code := m.Run()
	os.Remove(empty.Name())
	os.Exit(code)
}

type testRepo struct {
	t   *testing.T
	dir string
	seq int
}

// newTestRepo creates an empty repository on main with a local identity.
func newTestRepo(t *testing.T) *testRepo {
	t.Helper()
	r := &testRepo{t: t, dir: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	r.git("config", "user.name", "Test User")
	r.git("config", "user.email", "test@example.com")
	r.git("config", "commit.gpgsign", "false")
	return r
}

func (r *testRepo) gitEnv(env []string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *testRepo) git(args ...string) string {
	r.t.Helper()
	return r.gitEnv(nil, args...)
}

func dateEnv(age time.Duration) []string {
	date := refTime.Add(-age).Format(time.RFC3339)
	return []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}
}

// commit changes file and commits it, dated age before refTime.
func (r *testRepo) commit(file string, age time.Duration) {
	r.t.Helper()
	r.commitBy("Test User <test@example.com>", file, age)
}

func (r *testRepo) commitBy(author, file string, age time.Duration) {
	r.t.Helper()
	r.seq++
	path := filepath.Join(r.dir, file)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		r.t.Fatal(err)
	}
	fmt.Fprintf(f, "change %d\n", r.seq)
	f.Close()
	r.git("add", file)
	r.gitEnv(dateEnv(age), "commit", "-q", "--author", author, "-m", fmt.Sprintf("change %d to %s", r.seq, file))
}

// merge merges branch into the current branch with a merge commit.
func (r *testRepo) merge(branch string, age time.Duration) {
	r.t.Helper()
	r.gitEnv(dateEnv(age), "merge", "-q", "--no-ff", "-m", "merge "+branch, branch)
}

// fixture builds a repo with:
//
//	main            last commit 1 day ago
//	feature/merged  merged into main, last commit 150 days ago
//	feature/old     unmerged, last commit 120 days ago
//	feature/new     unmerged, last commit 2 days ago
func fixture(t *testing.T) *testRepo {
	t.Helper()
	r := newTestRepo(t)
	r.commit("README", 200*day)
	for _, b := range []string{"feature/merged", "feature/old", "feature/new"} {
		r.git("branch", b)
	}
	r.git("switch", "-q", "feature/merged")
	r.commit("merged.txt", 150*day)
	r.git("switch", "-q", "feature/old")
	r.commit("old.txt", 120*day)
	r.git("switch", "-q", "feature/new")
	r.commit("new.txt", 2*day)
	r.git("switch", "-q", "main")
	r.merge("feature/merged", 140*day)
	r.commit("README", 1*day)
	return r
}

// runTool runs the CLI against the repo and returns its output and status.
func (r *testRepo) runTool(args ...string) (stdout, stderr string, code int) {
	r.t.Helper()
	var out, errOut bytes.Buffer
	e := &env{
		stdout: &out,
		stderr: &errOut,
		now:    func() time.Time { return refTime },
	}
	code = run(append([]string{"-C", r.dir}, args...), e)
	return out.String(), errOut.String(), code
}

// tableNames returns the BRANCH column of a table, skipping the header.
func tableNames(out string) []string {
	var names []string
	for i, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if i == 0 || line == "" {
			continue
		}
		names = append(names, strings.Fields(line)[0])
	}
	return names
}
