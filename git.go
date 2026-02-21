package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// gitRunner runs git commands in a fixed directory.
type gitRunner struct {
	dir string
}

// gitError describes a failed git invocation, keeping git's own message.
type gitError struct {
	args   []string
	stderr string
	err    error
}

func (e *gitError) Error() string {
	msg := strings.TrimSpace(e.stderr)
	msg = strings.TrimPrefix(msg, "fatal: ")
	if msg == "" {
		msg = e.err.Error()
	}
	return fmt.Sprintf("git %s: %s", strings.Join(e.args, " "), msg)
}

func (e *gitError) Unwrap() error { return e.err }

func (g *gitRunner) command(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.dir
	cmd.Env = os.Environ()
	return cmd
}

// output runs git and returns its stdout.
func (g *gitRunner) output(args ...string) (string, error) {
	cmd := g.command(args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &gitError{args: args, stderr: stderr.String(), err: err}
	}
	return stdout.String(), nil
}

// check runs a git command whose exit status is the answer: status 0 is
// true, status 1 is false, anything else is an error.
func (g *gitRunner) check(args ...string) (bool, error) {
	cmd := g.command(args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, &gitError{args: args, stderr: stderr.String(), err: err}
}

// refExists reports whether ref names an existing commit.
func (g *gitRunner) refExists(ref string) bool {
	_, err := g.output("rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return err == nil
}

// configValues returns all values of a git config key, or nil if unset.
func (g *gitRunner) configValues(key string) ([]string, error) {
	out, err := g.output("config", "--get-all", key)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	return strings.Split(strings.TrimRight(out, "\n"), "\n"), nil
}
