package main

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Branch is one branch together with what we learned about it.
type Branch struct {
	Name     string // short name, e.g. feature/login
	Ref      string // full ref, e.g. refs/heads/feature/login
	Commit   string
	Date     time.Time // committer date of the tip
	Author   string
	Email    string
	Current  bool   // checked out in this worktree
	Worktree string // path of the worktree that has it checked out, if any
	Merged   bool   // tip is an ancestor of the base
	Squashed bool   // not merged, but its changes are on the base (--squash)
	Ahead    int    // commits on the branch that are not on the base
	Behind   int    // commits on the base that are not on the branch

	Protected string // why the branch is protected, or ""
}

// repo wraps the git queries the tool needs.
type repo struct {
	git    *gitRunner
	remote string
}

// detectBase picks the base branch name: the target of <remote>/HEAD,
// else main, else master.
func (r *repo) detectBase() (string, error) {
	head := "refs/remotes/" + r.remote + "/HEAD"
	if out, err := r.git.output("symbolic-ref", "--quiet", head); err == nil {
		target := strings.TrimSpace(out)
		if name, ok := strings.CutPrefix(target, "refs/remotes/"+r.remote+"/"); ok && name != "" {
			return name, nil
		}
	}
	for _, name := range []string{"main", "master"} {
		if r.git.refExists("refs/heads/"+name) || r.git.refExists("refs/remotes/"+r.remote+"/"+name) {
			return name, nil
		}
	}
	return "", errors.New("cannot detect the base branch (no " + r.remote + "/HEAD, main or master); use --base")
}

// resolveBase turns a base branch name into a ref to compare against,
// preferring the local branch (the remote-tracking one with remoteMode),
// then the other one, then whatever git makes of the name.
func (r *repo) resolveBase(name string, remoteMode bool) (string, error) {
	local, tracking := "refs/heads/"+name, "refs/remotes/"+r.remote+"/"+name
	candidates := []string{local, tracking, name}
	if remoteMode {
		candidates = []string{tracking, local, name}
	}
	for _, ref := range candidates {
		if r.git.refExists(ref) {
			return ref, nil
		}
	}
	return "", fmt.Errorf("base branch %q not found", name)
}

// shortRef strips refs/heads/ or refs/remotes/ from a ref for display.
func shortRef(ref string) string {
	for _, p := range []string{"refs/heads/", "refs/remotes/"} {
		if s, ok := strings.CutPrefix(ref, p); ok {
			return s
		}
	}
	return ref
}

// checkRemote fails unless r.remote is a configured remote.
func (r *repo) checkRemote() error {
	out, err := r.git.output("remote")
	if err != nil {
		return err
	}
	for _, name := range strings.Fields(out) {
		if name == r.remote {
			return nil
		}
	}
	return fmt.Errorf("no such remote %q", r.remote)
}

// remoteCurrent returns the branch names on r.remote that belong to the
// checked-out branch: the same name, and its upstream if that lives on
// r.remote. It is empty on a detached HEAD.
func (r *repo) remoteCurrent() (map[string]bool, error) {
	out, err := r.git.output("symbolic-ref", "--quiet", "HEAD")
	if err != nil {
		return nil, nil
	}
	head := strings.TrimSpace(out)
	names := map[string]bool{strings.TrimPrefix(head, "refs/heads/"): true}
	up, err := r.git.output("for-each-ref", "--format=%(upstream)", head)
	if err != nil {
		return nil, err
	}
	if name, ok := strings.CutPrefix(strings.TrimSpace(up), "refs/remotes/"+r.remote+"/"); ok && name != "" {
		names[name] = true
	}
	return names, nil
}

const refFormat = "%(refname)%00%(objectname)%00%(committerdate:unix)%00%(authorname)%00%(authoremail)%00%(HEAD)%00%(symref)%00%(worktreepath)"

// listRefs reads branch tips under prefix (e.g. refs/heads/), skipping
// symbolic refs such as refs/remotes/origin/HEAD.
func (r *repo) listRefs(prefix string) ([]Branch, error) {
	out, err := r.git.output("for-each-ref", "--format="+refFormat, prefix)
	if err != nil {
		return nil, err
	}
	var branches []Branch
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\x00")
		if len(f) != 8 {
			return nil, fmt.Errorf("unexpected for-each-ref output: %q", line)
		}
		if f[6] != "" {
			continue
		}
		unix, err := strconv.ParseInt(f[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bad commit date for %s: %v", f[0], err)
		}
		branches = append(branches, Branch{
			Name:     strings.TrimPrefix(f[0], prefix),
			Ref:      f[0],
			Commit:   f[1],
			Date:     time.Unix(unix, 0),
			Author:   f[3],
			Email:    strings.Trim(f[4], "<>"),
			Current:  f[5] == "*",
			Worktree: f[7],
		})
	}
	sort.SliceStable(branches, func(i, j int) bool {
		if !branches[i].Date.Equal(branches[j].Date) {
			return branches[i].Date.Before(branches[j].Date)
		}
		return branches[i].Name < branches[j].Name
	})
	return branches, nil
}

// landed reports whether the branch's work is on the base, by a real
// merge or (with --squash) a squash or rebase merge.
func (b Branch) landed() bool { return b.Merged || b.Squashed }

// classify fills in merge status and ahead/behind counts for each branch
// relative to base. With squash it also looks for squash-merged branches.
func (r *repo) classify(branches []Branch, base string, squash bool) error {
	for i := range branches {
		b := &branches[i]
		merged, err := r.git.check("merge-base", "--is-ancestor", b.Ref, base)
		if err != nil {
			return err
		}
		b.Merged = merged
		if b.Behind, b.Ahead, err = r.aheadBehind(base, b.Ref); err != nil {
			return err
		}
		if squash && !merged {
			if b.Squashed, err = r.squashMerged(base, b.Ref); err != nil {
				return err
			}
		}
	}
	return nil
}

// aheadBehind counts commits only on base (left) and only on ref (right).
func (r *repo) aheadBehind(base, ref string) (left, right int, err error) {
	out, err := r.git.output("rev-list", "--left-right", "--count", base+"..."+ref)
	if err != nil {
		return 0, 0, err
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list output: %q", out)
	}
	if left, err = strconv.Atoi(f[0]); err != nil {
		return 0, 0, err
	}
	if right, err = strconv.Atoi(f[1]); err != nil {
		return 0, 0, err
	}
	return left, right, nil
}
