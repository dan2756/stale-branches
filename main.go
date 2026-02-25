// Command stale-branches lists and deletes old or merged git branches.
package main

import (
	"os"
	"time"

	"golang.org/x/term"
)

func main() {
	os.Exit(run(os.Args[1:], &env{
		stdin:    os.Stdin,
		stdout:   os.Stdout,
		stderr:   os.Stderr,
		stdinTTY: term.IsTerminal(int(os.Stdin.Fd())),
		now:      time.Now,
	}))
}
