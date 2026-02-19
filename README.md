# stale-branches

List old or merged git branches.

`stale-branches` shows every local branch with the age of its last commit,
the last author, and whether it is already merged into the base branch, so you
can see at a glance what is safe to clean up.

## Install

```sh
go install github.com/dan2756/stale-branches@latest
```

## Usage

Run it anywhere inside a git repository:

```console
$ stale-branches
BRANCH        AGE           AUTHOR  STATUS
feature/old   7 months ago  Alice   merged
main          2 hours ago   Alice   merged
wip           2 hours ago   Alice   unmerged
```

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
