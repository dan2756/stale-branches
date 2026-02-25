package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"regexp"
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
      --author REGEX     only branches whose last commit author matches REGEX
                         (matched against "Name <email>")
      --pattern GLOB     only branches matching GLOB, e.g. 'feature/*';
                         * also matches '/'; repeat to match any of several

  --merged, --unmerged and --older-than replace the default stale rule and
  are combined with AND. --author and --pattern always narrow the result.

Protection:
      --protect LIST     comma-separated globs of branches never to touch
                         (default: git config stale-branches.protect, else
                         main,master,develop,release/*)

  The current branch and the base branch are always protected. Protected
  branches are only listed with --all.

Deleting:
      --delete           delete the listed branches after asking [y/N]
  -y, --yes              do not ask; required when stdin is not a terminal
      --force            also delete unmerged branches (git branch -D)
  -n, --dry-run          print the git commands --delete would run and exit

  Without --force only merged branches are deleted, with git branch -d.

Output:
      --json             print JSON instead of a table
      --porcelain        print only branch names, one per line

  The table is colored when stdout is a terminal, unless NO_COLOR is set.

Other:
  -C DIR                 run as if started in DIR
  -h, --help             show this help
      --version          print the version and exit

Exit status: 0 ok, 1 git error or not a git repository, 2 usage error.
`

// env is everything run needs from the outside world.
type env struct {
	stdin     io.Reader
	stdout    io.Writer
	stderr    io.Writer
	stdinTTY  bool
	stdoutTTY bool
	getenv    func(string) string
	now       func() time.Time
}

// color reports whether the table should be colored.
func (e *env) color() bool {
	return e.stdoutTTY && e.getenv("NO_COLOR") == "" && e.getenv("TERM") != "dumb"
}

type options struct {
	dir     string
	base    string
	version bool
	filter  filter
	protect *string // nil when --protect was not given

	delete bool
	yes    bool
	force  bool
	dryRun bool

	format outputFormat
}

// usageError is a problem with how the tool was invoked (exit status 2).
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

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
	author := fs.String("author", "", "")
	var patterns stringList
	fs.Var(&patterns, "pattern", "")
	protect := fs.String("protect", "", "")
	fs.BoolVar(&opts.delete, "delete", false, "")
	fs.BoolVar(&opts.yes, "y", false, "")
	fs.BoolVar(&opts.yes, "yes", false, "")
	fs.BoolVar(&opts.force, "force", false, "")
	fs.BoolVar(&opts.dryRun, "n", false, "")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "")
	jsonOut := fs.Bool("json", false, "")
	porcelain := fs.Bool("porcelain", false, "")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	switch {
	case *jsonOut && *porcelain:
		return nil, errors.New("--json and --porcelain are mutually exclusive")
	case *jsonOut:
		opts.format = formatJSON
	case *porcelain:
		opts.format = formatPorcelain
	}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "protect" {
			opts.protect = protect
		}
	})
	if opts.protect != nil {
		if _, err := compileGlobs(splitList(*opts.protect)); err != nil {
			return nil, fmt.Errorf("--protect: %v", err)
		}
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
	if *author != "" {
		re, err := regexp.Compile(*author)
		if err != nil {
			return nil, fmt.Errorf("--author: %v", err)
		}
		opts.filter.author = re
	}
	for _, p := range patterns {
		re, err := compileGlob(p)
		if err != nil {
			return nil, fmt.Errorf("--pattern: %v", err)
		}
		opts.filter.patterns = append(opts.filter.patterns, re)
	}
	if opts.dryRun {
		opts.delete = true
	}
	if opts.delete && opts.filter.all {
		return nil, errors.New("--all cannot be combined with --delete")
	}
	return opts, nil
}

// stringList is a flag that may be given more than once.
type stringList []string

func (l *stringList) String() string     { return strings.Join(*l, ",") }
func (l *stringList) Set(v string) error { *l = append(*l, v); return nil }

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
		var ue *usageError
		if errors.As(err, &ue) {
			return exitUsage
		}
		return exitError
	}
	return exitOK
}

func execute(opts *options, e *env) error {
	r := &repo{git: &gitRunner{dir: opts.dir}, remote: "origin"}
	if _, err := r.git.output("rev-parse", "--git-dir"); err != nil {
		return notRepoError(err)
	}
	now := e.now()
	selected, baseName, err := selectBranches(r, opts, now)
	if err != nil {
		return err
	}
	if opts.delete {
		return deleteBranches(r, selected, baseName, opts, e)
	}
	if len(selected) == 0 && opts.format == formatTable {
		fmt.Fprintln(e.stderr, "no matching branches")
		return nil
	}
	return writeBranches(e.stdout, opts.format, selected, baseName, now, e.color())
}

// selectBranches loads, classifies and filters the branches. It also
// returns the base branch name.
func selectBranches(r *repo, opts *options, now time.Time) ([]Branch, string, error) {
	baseName := opts.base
	if baseName == "" {
		var err error
		if baseName, err = r.detectBase(); err != nil {
			return nil, "", err
		}
	}
	base, err := r.resolveBase(baseName)
	if err != nil {
		return nil, "", err
	}
	globs, err := r.protectGlobs(opts.protect)
	if err != nil {
		return nil, "", err
	}
	prot := &protection{base: strings.TrimPrefix(baseName, r.remote+"/"), globs: globs}

	branches, err := r.listRefs("refs/heads/")
	if err != nil {
		return nil, "", err
	}
	if err := r.classify(branches, base); err != nil {
		return nil, "", err
	}
	var selected []Branch
	for _, b := range branches {
		b.Protected = prot.reason(b)
		if b.Protected != "" && !opts.filter.all {
			continue
		}
		if opts.filter.match(b, now) {
			selected = append(selected, b)
		}
	}
	return selected, baseName, nil
}

// deleteBranches shows what will be deleted, asks, and deletes it.
func deleteBranches(r *repo, selected []Branch, baseName string, opts *options, e *env) error {
	del, skipped := planDeletion(selected, opts.force)
	for _, b := range skipped {
		fmt.Fprintf(e.stderr, "skipping %s: not merged into %s (use --force to delete it)\n", b.Name, baseName)
	}
	if len(del) == 0 {
		fmt.Fprintln(e.stderr, "nothing to delete")
		return nil
	}
	args := deleteArgs(del, opts.force)
	if opts.dryRun {
		fmt.Fprintln(e.stdout, formatCommand(args))
		return nil
	}
	if !opts.yes && !e.stdinTTY {
		return &usageError{"refusing to delete without confirmation: stdin is not a terminal (use -y)"}
	}
	if err := writeBranches(e.stdout, opts.format, del, baseName, e.now(), e.color()); err != nil {
		return err
	}
	if !opts.yes && !confirm(e.stdin, e.stderr, "Delete "+pluralBranches(len(del))+"?") {
		fmt.Fprintln(e.stderr, "aborted")
		return nil
	}
	if err := r.git.run(e.stdout, e.stderr, args...); err != nil {
		if !opts.force {
			fmt.Fprintf(e.stderr, "hint: git branch -d checks merges against HEAD or the branch's upstream, not %s;\n"+
				"hint: run from %s, or use --force once you are sure\n", baseName, baseName)
		}
		return err
	}
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
