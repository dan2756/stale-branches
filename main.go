// Command stale-branches lists and deletes old or merged git branches.
package main

import (
	"os"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], &env{
		stdout: os.Stdout,
		stderr: os.Stderr,
		now:    time.Now,
	}))
}
