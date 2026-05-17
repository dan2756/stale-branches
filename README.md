# stale-branches

List and delete old or merged git branches.

`stale-branches` shows your stale local branches with the age of their last commit,
the last author, whether it is already merged into the base branch, and how
far ahead and behind the base it is, so you can see at a glance what is safe
to clean up.

## Install

```sh
go install github.com/dan2756/stale-branches@latest
```

## Usage

Run it anywhere inside a git repository:

```console
$ stale-branches
BRANCH          AGE           AUTHOR  STATUS    AHEAD  BEHIND
spike/parser    7 months ago  Alice   unmerged      4      58
feature/login   4 months ago  Bob     merged        0      31
fix/typo        2 days ago    Alice   merged        0       2
```

A branch is **stale** when it is merged into the base branch or its last
commit is more than 90 days old. `--all` lists every branch instead.
Branches are listed oldest first. `AHEAD` counts commits only on the branch,
`BEHIND` counts commits only on the base.

The base branch is the target of `origin/HEAD`, else `main`, else `master`.
Use `--base develop` to compare against something else.

| Flag | Meaning |
| --- | --- |
| `--base BRANCH` | branch to compare against |
| `--remote` | list the remote's branches instead of local ones |
| `--remote-name NAME` | remote for `--remote` and base detection (default `origin`) |
| `--all` | list every branch, not just stale ones |
| `--merged` | only branches merged into the base |
| `--unmerged` | only branches not merged into the base |
| `--older-than AGE` | only branches whose last commit is older than `AGE` |
| `--author REGEX` | only branches whose last commit author matches (against `Name <email>`) |
| `--pattern GLOB` | only branches matching `GLOB`; repeat for several |
| `--squash` | also treat squash- and rebase-merged branches as merged |
| `--protect LIST` | comma-separated globs of branches never to touch |
| `--delete` | delete the listed branches after asking `[y/N]` |
| `-y`, `--yes` | don't ask (required when stdin is not a terminal) |
| `--force` | also delete unmerged branches (`git branch -D`) |
| `-n`, `--dry-run` | print the git commands `--delete` would run |
| `--json` | print JSON instead of a table |
| `--porcelain` | print only branch names, one per line |
| `-C DIR` | run as if started in `DIR` |
| `--version` | print the version |

`--merged`, `--unmerged` and `--older-than` replace the default stale rule and
are combined with AND, so `--unmerged --older-than 6mo` finds abandoned work.
Ages are a number followed by `h`, `d`, `w`, `mo` (30 days) or `y` (365 days):
`90d`, `6w`, `3mo`, `1y`.

`--author` and `--pattern` always narrow the result further. `--author` uses
Go regexp syntax (prefix with `(?i)` for case-insensitive). In `--pattern`,
`*` also matches `/`, so `feature/*` covers `feature/auth/login` too.

## Output

The table is colored when stdout is a terminal; set `NO_COLOR` to turn that
off. For scripts there are two stable formats:

```console
$ stale-branches --porcelain --merged
feature/login
fix/typo

$ stale-branches --json --pattern 'feature/*'
{
  "base": "main",
  "branches": [
    {
      "name": "feature/login",
      "ref": "refs/heads/feature/login",
      "commit": "3f9c2e1d6b0a4c8e9f7a2b1c0d9e8f7a6b5c4d3e",
      "date": "2025-10-14T09:12:44Z",
      "age_days": 133,
      "author": "Bob",
      "email": "bob@example.com",
      "status": "merged",
      "merged": true,
      "squashed": false,
      "ahead": 0,
      "behind": 31,
      "current": false,
      "protected": false
    }
  ]
}
```

`status` is `merged`, `unmerged` or (with `--squash`) `squashed`. `note` is
added for protected branches (shown with `--all`) and says why:
`current`, `worktree`, `base` or `protected`.

## Deleting

`--delete` shows the branches it is about to delete and asks before doing
anything. Without a terminal on stdin it refuses unless you pass `-y`.

```console
$ stale-branches --delete
skipping spike/parser: not merged into main (use --force to delete it)
BRANCH         AGE           AUTHOR  STATUS  AHEAD  BEHIND
feature/login  4 months ago  Bob     merged      0      31
fix/typo       2 days ago    Alice   merged      0       2
Delete 2 branches? [y/N] y
Deleted branch feature/login (was 3f9c2e1).
Deleted branch fix/typo (was 81d07aa).
```

Only merged branches are deleted, with `git branch -d`. `--force` deletes
unmerged ones too, with `git branch -D`. `--dry-run` prints the exact
command instead of running it:

```console
$ stale-branches --dry-run --force
git branch -D spike/parser feature/login fix/typo
```

`git branch -d` checks merges against `HEAD` (or the branch's upstream), not
the base branch, so run deletions from the base branch.

## Squash and rebase merges

If your project merges pull requests with "squash and merge" or "rebase and
merge", the branch tips never become ancestors of the base, so they look
unmerged. `--squash` adds two checks for every unmerged branch and reports a
match as `squashed`:

1. **Rebased or cherry-picked:** `git cherry <base> <branch>` marks every
   commit of the branch as already upstream.
2. **Squashed:** a throwaway commit is built with the branch's tree on top of
   its merge base (`git commit-tree <branch>^{tree} -p <merge-base>`), which
   is what a squash merge of the branch would look like, and `git cherry`
   checks whether the base already has a commit with the same patch.

This is a heuristic. It misses squash merges whose diff differs from the
branch's, for example when conflicts were resolved during the merge or the
branch got more commits afterwards. The probe commit is never referenced by
anything and is cleaned up by `git gc`; it has a fixed author and date, so
repeated runs reuse the same object. Expect it to be slower on repositories
with many branches.

Squashed branches count as merged for the stale rule and `--merged`, but
`git branch -d` cannot see that, so deleting them needs `--force`:

```console
$ stale-branches --squash --merged --delete --force
```

## Remote branches

`--remote` looks at `refs/remotes/origin/*` instead of your local branches and
compares them against `origin/main` (or whatever `origin/HEAD` points to).
Fetch first so the remote-tracking refs are current:

```console
$ git fetch --prune
$ stale-branches --remote --merged
BRANCH         AGE           AUTHOR  STATUS  AHEAD  BEHIND
feature/login  4 months ago  Bob     merged      0      31
dependabot/x   3 weeks ago   bot     merged      0       9

$ stale-branches --remote --merged --dry-run
git push origin --delete feature/login dependabot/x
```

Remote deletions go out in a single `git push <remote> --delete`. The remote
branch matching your current branch (by name or as its upstream) is protected
like the current branch is locally. Use `--remote-name upstream` for a
different remote; it also picks which `<remote>/HEAD` is used to detect the
base.

## Protected branches

The current branch, branches checked out in another worktree (`git worktree`)
and the base branch are always protected. On top of that,
branches matching `main,master,develop,release/*` are protected by default.
Protected branches never show up as stale; `--all` lists them with a `NOTE`
saying why.

Replace the default list per repository (or globally with `--global`):

```sh
git config stale-branches.protect 'main,develop,release/*,hotfix/*'
```

The key may be given several times (`git config --add`). `--protect` on the
command line overrides both; `--protect ''` leaves only the current and base
branches protected.

## Exit status

Exit status is 0 on success, 1 on a git error or outside a repository, and 2
on a usage error.

## License

MIT
