package main

import "strings"

// probeEnv gives the throwaway squash commit a fixed identity and date, so
// it works without user.name/user.email and is the same object every run.
var probeEnv = []string{
	"GIT_AUTHOR_NAME=stale-branches",
	"GIT_AUTHOR_EMAIL=stale-branches@localhost",
	"GIT_AUTHOR_DATE=2000-01-01T00:00:00Z",
	"GIT_COMMITTER_NAME=stale-branches",
	"GIT_COMMITTER_EMAIL=stale-branches@localhost",
	"GIT_COMMITTER_DATE=2000-01-01T00:00:00Z",
}

// squashMerged reports whether the changes on ref already landed on base
// without ref itself being merged: either every commit was rebased or
// cherry-picked onto base, or the whole branch was squashed into one
// commit there.
//
// The squash check is the commit-tree trick: build a dangling commit with
// ref's tree on top of the merge base, which is exactly what a squash
// merge of ref would look like, and ask git cherry whether base already
// has a commit with the same patch. It misses squash merges whose diff
// differs from the branch's, e.g. after resolving conflicts.
func (r *repo) squashMerged(base, ref string) (bool, error) {
	out, err := r.git.output("cherry", base, ref)
	if err != nil {
		return false, err
	}
	if allApplied(out) {
		return true, nil
	}

	mb, err := r.git.output("merge-base", base, ref)
	if err != nil {
		if exitStatus(err) == 1 {
			return false, nil // no common history
		}
		return false, err
	}
	probe, err := r.git.outputEnv(probeEnv, "commit-tree", "--no-gpg-sign",
		ref+"^{tree}", "-p", strings.TrimSpace(mb), "-m", "stale-branches squash probe")
	if err != nil {
		return false, err
	}
	out, err = r.git.output("cherry", base, strings.TrimSpace(probe))
	if err != nil {
		return false, err
	}
	return allApplied(out), nil
}

// allApplied reports whether git cherry output lists at least one commit
// and marks every one as already upstream ("-").
func allApplied(cherry string) bool {
	cherry = strings.TrimSpace(cherry)
	if cherry == "" {
		return false
	}
	for _, line := range strings.Split(cherry, "\n") {
		if !strings.HasPrefix(line, "- ") {
			return false
		}
	}
	return true
}
