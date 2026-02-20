package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// defaultStaleAge is how old an unmerged branch must be to count as stale
// when no filter flags are given.
const defaultStaleAge = 90 * 24 * time.Hour

var ageRE = regexp.MustCompile(`^(\d+)(h|d|w|mo|y)$`)

// parseAge parses ages such as 12h, 90d, 6w, 3mo and 1y. A month is 30
// days and a year is 365 days.
func parseAge(s string) (time.Duration, error) {
	m := ageRE.FindStringSubmatch(s)
	if m == nil {
		hint := ""
		if strings.HasSuffix(s, "m") {
			hint = `; use "mo" for months`
		}
		return 0, fmt.Errorf("invalid age %q (want a number and h, d, w, mo or y, e.g. 90d)%s", s, hint)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid age %q: must be a positive number", s)
	}
	const day = 24 * time.Hour
	unit := map[string]time.Duration{
		"h":  time.Hour,
		"d":  day,
		"w":  7 * day,
		"mo": 30 * day,
		"y":  365 * day,
	}[m[2]]
	return time.Duration(n) * unit, nil
}

// filter decides which branches are shown.
type filter struct {
	all       bool          // no default stale rule
	merged    bool          // only merged branches
	unmerged  bool          // only unmerged branches
	olderThan time.Duration // only branches older than this; 0 means unset
}

// explicit reports whether any selection flag replaces the default rule.
func (f *filter) explicit() bool {
	return f.merged || f.unmerged || f.olderThan > 0
}

// match reports whether b is selected. Without explicit flags a branch is
// stale when it is merged or older than defaultStaleAge; explicit flags
// are combined with AND.
func (f *filter) match(b Branch, now time.Time) bool {
	age := now.Sub(b.Date)
	if !f.explicit() {
		return f.all || b.Merged || age > defaultStaleAge
	}
	if f.merged && !b.Merged {
		return false
	}
	if f.unmerged && b.Merged {
		return false
	}
	if f.olderThan > 0 && age <= f.olderThan {
		return false
	}
	return true
}
