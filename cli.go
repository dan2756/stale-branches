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

Lists local branches with the age of their last commit, its author, and
whether the branch is merged into the base branch.

Flags:
      --base BRANCH   compare against BRANCH
                      (default: target of origin/HEAD, else main, else master)
  -C DIR              run as if started in DIR
  -h, --help          show this help
      --version       print the version and exit

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
}

func parseArgs(args []string) (*options, error) {
	opts := &options{}
	fs := flag.NewFlagSet("stale-branches", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.dir, "C", "", "")
	fs.StringVar(&opts.base, "base", "", "")
	fs.BoolVar(&opts.version, "version", false, "")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument %q", fs.Arg(0))
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
	writeTable(e.stdout, branches, e.now())
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
