package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func loadBranches(t *testing.T, r *testRepo, base string) map[string]Branch {
	t.Helper()
	rp := &repo{git: &gitRunner{dir: r.dir}, remote: "origin"}
	ref, err := rp.resolveBase(base, false)
	if err != nil {
		t.Fatal(err)
	}
	branches, err := rp.listRefs("refs/heads/")
	if err != nil {
		t.Fatal(err)
	}
	if err := rp.classify(branches, ref, false); err != nil {
		t.Fatal(err)
	}
	m := map[string]Branch{}
	for _, b := range branches {
		m[b.Name] = b
	}
	return m
}

func TestClassify(t *testing.T) {
	r := fixture(t)
	got := loadBranches(t, r, "main")

	tests := []struct {
		name          string
		merged        bool
		ahead, behind int
		age           time.Duration
	}{
		{"main", true, 0, 0, 1 * day},
		{"feature/merged", true, 0, 2, 150 * day},
		{"feature/old", false, 1, 3, 120 * day},
		{"feature/new", false, 1, 3, 2 * day},
	}
	if len(got) != len(tests) {
		t.Fatalf("got %d branches, want %d", len(got), len(tests))
	}
	for _, tt := range tests {
		b, ok := got[tt.name]
		if !ok {
			t.Errorf("branch %s missing", tt.name)
			continue
		}
		if b.Merged != tt.merged || b.Ahead != tt.ahead || b.Behind != tt.behind {
			t.Errorf("%s: merged=%v ahead=%d behind=%d, want merged=%v ahead=%d behind=%d",
				tt.name, b.Merged, b.Ahead, b.Behind, tt.merged, tt.ahead, tt.behind)
		}
		if want := refTime.Add(-tt.age); !b.Date.Equal(want) {
			t.Errorf("%s: date %v, want %v", tt.name, b.Date, want)
		}
		if b.Author != "Test User" || b.Email != "test@example.com" {
			t.Errorf("%s: author %q <%s>", tt.name, b.Author, b.Email)
		}
	}
	if !got["main"].Current {
		t.Error("main should be the current branch")
	}
}

func TestBranchesSortedOldestFirst(t *testing.T) {
	r := fixture(t)
	out, _, code := r.runTool("--all")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	want := []string{"feature/merged", "feature/old", "feature/new", "main"}
	if got := tableNames(out); !slices.Equal(got, want) {
		t.Errorf("order %v, want %v", got, want)
	}
}

func TestDetectBase(t *testing.T) {
	t.Run("main", func(t *testing.T) {
		r := newTestRepo(t)
		r.commit("a", day)
		assertBase(t, r, "main")
	})
	t.Run("master", func(t *testing.T) {
		r := newTestRepo(t)
		r.commit("a", day)
		r.git("branch", "-m", "master")
		assertBase(t, r, "master")
	})
	t.Run("origin HEAD wins", func(t *testing.T) {
		r := newTestRepo(t)
		r.commit("a", day)
		r.git("branch", "trunk")
		r.git("update-ref", "refs/remotes/origin/trunk", "trunk")
		r.git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")
		assertBase(t, r, "trunk")
	})
	t.Run("none", func(t *testing.T) {
		r := newTestRepo(t)
		r.commit("a", day)
		r.git("branch", "-m", "trunk")
		rp := &repo{git: &gitRunner{dir: r.dir}, remote: "origin"}
		if _, err := rp.detectBase(); err == nil {
			t.Fatal("expected an error")
		}
		_, stderr, code := r.runTool()
		if code != exitError || !strings.Contains(stderr, "--base") {
			t.Errorf("exit %d, stderr %q", code, stderr)
		}
	})
}

func assertBase(t *testing.T, r *testRepo, want string) {
	t.Helper()
	rp := &repo{git: &gitRunner{dir: r.dir}, remote: "origin"}
	got, err := rp.detectBase()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("base %q, want %q", got, want)
	}
}

func TestBaseFlag(t *testing.T) {
	r := fixture(t)
	r.git("branch", "develop", "feature/old")
	got := loadBranches(t, r, "develop")
	if !got["feature/old"].Merged {
		t.Error("feature/old should be merged into develop")
	}
	if got["feature/merged"].Merged {
		t.Error("feature/merged should not be merged into develop")
	}

	_, stderr, code := r.runTool("--base", "nope")
	if code != exitError || !strings.Contains(stderr, `"nope" not found`) {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}

func TestNotARepo(t *testing.T) {
	r := &testRepo{t: t, dir: t.TempDir()}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(r.dir))
	_, stderr, code := r.runTool()
	if code != exitError {
		t.Errorf("exit %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr, "not a git repository") {
		t.Errorf("stderr %q", stderr)
	}
}
