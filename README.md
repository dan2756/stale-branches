# stale-branches

List old or merged git branches.

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
| `--all` | list every branch, not just stale ones |
| `--merged` | only branches merged into the base |
| `--unmerged` | only branches not merged into the base |
| `--older-than AGE` | only branches whose last commit is older than `AGE` |
| `-C DIR` | run as if started in `DIR` |
| `--version` | print the version |

`--merged`, `--unmerged` and `--older-than` replace the default stale rule and
are combined with AND, so `--unmerged --older-than 6mo` finds abandoned work.
Ages are a number followed by `h`, `d`, `w`, `mo` (30 days) or `y` (365 days):
`90d`, `6w`, `3mo`, `1y`.

Exit status is 0 on success, 1 on a git error or outside a repository, and 2
on a usage error.

## License

MIT
