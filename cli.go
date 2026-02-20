package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime/debug"
	"strings"
	"time"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = ""

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

const usageText = `stale-branches - list old or merged git branches

Usage:
  stale-branches [flags]

Lists stale local branches with the age of their last commit, its author,
whether the branch is merged into the base branch, and how far ahead and
behind the base it is. A branch is stale when it is merged into the base or
its last commit is more than 90 days old.

Selection:
      --base BRANCH      compare against BRANCH
                         (default: target of origin/HEAD, else main, else master)
      --all              list every branch, not just stale ones
      --merged           only branches merged into the base
      --unmerged         only branches not merged into the base
      --older-than AGE   only branches whose last commit is older than AGE,
                         e.g. 90d, 6w, 3mo, 1y

  --merged, --unmerged and --older-than replace the default stale rule and
  are combined with AND.

Other:
  -C DIR                 run as if started in DIR
  -h, --help             show this help
      --version          print the version and exit

Exit status: 0 ok, 1 git error or not a git repository, 2 usage error.
`

// env is everything run needs from the outside world.
type env struct {
	stdout io.Writer
	stderr io.Writer
	now    func() time.Time
}

type options struct {
	dir     string
	base    string
	version bool
	filter  filter
}

func parseArgs(args []string) (*options, error) {
	opts := &options{}
	fs := flag.NewFlagSet("stale-branches", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.dir, "C", "", "")
	fs.StringVar(&opts.base, "base", "", "")
	fs.BoolVar(&opts.version, "version", false, "")
	fs.BoolVar(&opts.filter.all, "all", false, "")
	fs.BoolVar(&opts.filter.merged, "merged", false, "")
	fs.BoolVar(&opts.filter.unmerged, "unmerged", false, "")
	olderThan := fs.String("older-than", "", "")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if opts.filter.merged && opts.filter.unmerged {
		return nil, errors.New("--merged and --unmerged are mutually exclusive")
	}
	if *olderThan != "" {
		age, err := parseAge(*olderThan)
		if err != nil {
			return nil, fmt.Errorf("--older-than: %v", err)
		}
		opts.filter.olderThan = age
	}
	return opts, nil
}

func run(args []string, e *env) int {
	opts, err := parseArgs(args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(e.stdout, usageText)
		return exitOK
	}
	if err != nil {
		fmt.Fprintf(e.stderr, "stale-branches: %v\nRun 'stale-branches --help' for usage.\n", err)
		return exitUsage
	}
	if opts.version {
		fmt.Fprintf(e.stdout, "stale-branches %s\n", buildVersion())
		return exitOK
	}
	if err := execute(opts, e); err != nil {
		fmt.Fprintf(e.stderr, "stale-branches: %v\n", err)
		return exitError
	}
	return exitOK
}

func execute(opts *options, e *env) error {
	r := &repo{git: &gitRunner{dir: opts.dir}, remote: "origin"}
	if _, err := r.git.output("rev-parse", "--git-dir"); err != nil {
		return notRepoError(err)
	}
	baseName := opts.base
	if baseName == "" {
		var err error
		if baseName, err = r.detectBase(); err != nil {
			return err
		}
	}
	base, err := r.resolveBase(baseName)
	if err != nil {
		return err
	}
	branches, err := r.listRefs("refs/heads/")
	if err != nil {
		return err
	}
	if err := r.classify(branches, base); err != nil {
		return err
	}
	now := e.now()
	var selected []Branch
	for _, b := range branches {
		// The base is trivially merged into itself; only --all shows it.
		if b.Ref == base && !opts.filter.all {
			continue
		}
		if opts.filter.match(b, now) {
			selected = append(selected, b)
		}
	}
	if len(selected) == 0 {
		fmt.Fprintln(e.stderr, "no matching branches")
		return nil
	}
	writeTable(e.stdout, selected, now)
	return nil
}

// notRepoError turns a failed rev-parse into a short message.
func notRepoError(err error) error {
	var ge *gitError
	if errors.As(err, &ge) && ge.stderr != "" {
		return errors.New(strings.TrimPrefix(strings.TrimSpace(ge.stderr), "fatal: "))
	}
	return err
}

func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
