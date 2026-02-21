package main

import (
	"fmt"
	"regexp"
	"strings"
)

// defaultProtect is used when neither --protect nor the
// stale-branches.protect git config is set.
var defaultProtect = []string{"main", "master", "develop", "release/*"}

// protection decides which branches must never be listed as stale or
// deleted. The current branch and the base are always protected; the globs
// come from --protect, git config, or defaultProtect.
type protection struct {
	base  string
	globs []*regexp.Regexp
}

// reason returns why b is protected, or "" if it is not.
func (p *protection) reason(b Branch) string {
	switch {
	case b.Current:
		return "current"
	case b.Name == p.base:
		return "base"
	}
	for _, re := range p.globs {
		if re.MatchString(b.Name) {
			return "protected"
		}
	}
	return ""
}

// splitList splits comma-separated values, dropping blanks.
func splitList(values ...string) []string {
	var out []string
	for _, v := range values {
		for _, item := range strings.Split(v, ",") {
			if item = strings.TrimSpace(item); item != "" {
				out = append(out, item)
			}
		}
	}
	return out
}

func compileGlobs(patterns []string) ([]*regexp.Regexp, error) {
	globs := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := compileGlob(p)
		if err != nil {
			return nil, err
		}
		globs = append(globs, re)
	}
	return globs, nil
}

// protectGlobs returns the configured protection globs: flagValue when the
// flag was given, else stale-branches.protect from git config, else the
// defaults.
func (r *repo) protectGlobs(flagValue *string) ([]*regexp.Regexp, error) {
	if flagValue != nil {
		return compileGlobs(splitList(*flagValue))
	}
	values, err := r.git.configValues("stale-branches.protect")
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return compileGlobs(defaultProtect)
	}
	globs, err := compileGlobs(splitList(values...))
	if err != nil {
		return nil, fmt.Errorf("git config stale-branches.protect: %v", err)
	}
	return globs, nil
}
