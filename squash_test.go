package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// squashFixture extends fixture with:
//
//	feature/squashed  two commits, squash-merged into main
//	feature/rebased   two commits, both cherry-picked onto main
//	feature/partial   two commits, only the first cherry-picked
func squashFixture(t *testing.T) *testRepo {
	t.Helper()
	r := fixture(t)
	r.git("switch", "-q", "-c", "feature/squashed")
	r.commit("squash.txt", 10*day)
	r.commit("squash.txt", 9*day)
	r.git("switch", "-q", "main")
	r.gitEnv(dateEnv(0), "merge", "-q", "--squash", "feature/squashed")
	r.gitEnv(dateEnv(0), "commit", "-q", "-m", "squash feature/squashed")

	r.git("switch", "-q", "-c", "feature/rebased", "main~1")
	r.commit("rebased.txt", 8*day)
	r.commit("rebased.txt", 7*day)
	r.git("switch", "-q", "-c", "feature/partial", "main~1")
	r.commit("partial-a.txt", 6*day)
	r.commit("partial-b.txt", 5*day)
	r.git("switch", "-q", "main")
	r.gitEnv(dateEnv(0), "cherry-pick", "feature/rebased~1", "feature/rebased", "feature/partial~1")
	return r
}

func TestSquashDetection(t *testing.T) {
	r := squashFixture(t)

	out, _, _ := r.runTool("--porcelain")
	if got, want := strings.Fields(out), []string{"feature/merged", "feature/old"}; !slices.Equal(got, want) {
		t.Errorf("without --squash: got %v, want %v", got, want)
	}

	out, stderr, code := r.runTool("--porcelain", "--squash")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if got, want := strings.Fields(out), []string{"feature/merged", "feature/old", "feature/squashed", "feature/rebased"}; !slices.Equal(got, want) {
		t.Errorf("with --squash: got %v, want %v", got, want)
	}

	out, _, _ = r.runTool("--porcelain", "--squash", "--unmerged")
	if got, want := strings.Fields(out), []string{"feature/old", "feature/partial", "feature/new"}; !slices.Equal(got, want) {
		t.Errorf("--unmerged: got %v, want %v", got, want)
	}

	out, _, _ = r.runTool("--json", "--squash", "--all")
	var got jsonOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"main":             "merged",
		"feature/merged":   "merged",
		"feature/old":      "unmerged",
		"feature/new":      "unmerged",
		"feature/squashed": "squashed",
		"feature/rebased":  "squashed",
		"feature/partial":  "unmerged",
	}
	for _, b := range got.Branches {
		if b.Status != want[b.Name] {
			t.Errorf("%s: status %q, want %q", b.Name, b.Status, want[b.Name])
		}
		if b.Squashed != (b.Status == "squashed") || b.Merged != (b.Status == "merged") {
			t.Errorf("%s: merged=%v squashed=%v", b.Name, b.Merged, b.Squashed)
		}
	}
}

func TestSquashDelete(t *testing.T) {
	r := squashFixture(t)
	out, stderr, _ := r.runTool("--squash", "-n")
	if out != "git branch -d feature/merged\n" {
		t.Errorf("dry run %q", out)
	}
	if !strings.Contains(stderr, "skipping feature/squashed: squash-merged into main; deleting it needs --force") {
		t.Errorf("stderr %q", stderr)
	}

	out, _, _ = r.runTool("--squash", "--merged", "-n", "--force")
	if out != "git branch -D feature/merged feature/squashed feature/rebased\n" {
		t.Errorf("dry run --force %q", out)
	}

	if _, stderr, code := r.runTool("--squash", "--merged", "--delete", "-y", "--force"); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if got, want := r.branchNames(), []string{"feature/new", "feature/old", "feature/partial", "main"}; !slices.Equal(got, want) {
		t.Errorf("branches %v, want %v", got, want)
	}
}

func TestSquashProbeNeedsNoIdentity(t *testing.T) {
	r := squashFixture(t)
	r.git("config", "--unset", "user.name")
	r.git("config", "--unset", "user.email")
	r.git("config", "user.useConfigOnly", "true")
	out, stderr, code := r.runTool("--porcelain", "--squash", "--merged")
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.Contains(out, "feature/squashed") {
		t.Errorf("out %q", out)
	}

	// The probe commit is deterministic, so running again adds no objects.
	before := r.git("count-objects")
	r.runTool("--porcelain", "--squash")
	if after := r.git("count-objects"); after != before {
		t.Errorf("objects grew: %q -> %q", before, after)
	}
}

func TestAllApplied(t *testing.T) {
	tests := map[string]bool{
		"":                 false,
		"- abc\n":          true,
		"- abc\n- def\n":   true,
		"- abc\n+ def\n":   false,
		"+ abc\n":          false,
		"\n- abc\n- def\n": true,
	}
	for in, want := range tests {
		if got := allApplied(in); got != want {
			t.Errorf("allApplied(%q) = %v, want %v", in, got, want)
		}
	}
}
