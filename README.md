# stale-branches

List old or merged git branches.

`stale-branches` shows every local branch with the age of its last commit,
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
BRANCH       AGE           AUTHOR  STATUS    AHEAD  BEHIND
feature/old  7 months ago  Alice   merged        0      14
main         2 hours ago   Alice   merged        0       0
wip          2 hours ago   Alice   unmerged      3       1
```

Branches are listed oldest first. `AHEAD` counts commits only on the branch,
`BEHIND` counts commits only on the base.

The base branch is the target of `origin/HEAD`, else `main`, else `master`.
Use `--base develop` to compare against something else.

| Flag | Meaning |
| --- | --- |
| `--base BRANCH` | branch to compare against |
| `-C DIR` | run as if started in `DIR` |
| `--version` | print the version |

Exit status is 0 on success, 1 on a git error or outside a repository, and 2
on a usage error.

## License

MIT
