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

// compileGlob turns a shell-style glob into an anchored regexp. Unlike
// path.Match, * also matches '/', so feature/* covers feature/a/b.
// ? matches one character and [...] (or [!...]) a character class.
func compileGlob(pattern string) (*regexp.Regexp, error) {
	var sb strings.Builder
	sb.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; c {
		case '*':
			sb.WriteString(".*")
		case '?':
			sb.WriteString(".")
		case '[':
			end := strings.IndexByte(pattern[i+1:], ']')
			if end < 0 {
				return nil, fmt.Errorf("invalid pattern %q: unterminated [", pattern)
			}
			class := pattern[i+1 : i+1+end]
			if rest, ok := strings.CutPrefix(class, "!"); ok {
				class = "^" + rest
			}
			sb.WriteString("[" + class + "]")
			i += end + 1
		default:
			sb.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	sb.WriteString("$")
	re, err := regexp.Compile(sb.String())
	if err != nil {
		return nil, fmt.Errorf("invalid pattern %q", pattern)
	}
	return re, nil
}

// filter decides which branches are shown.
type filter struct {
	all       bool             // no default stale rule
	merged    bool             // only merged (or squash-merged) branches
	unmerged  bool             // only unmerged branches
	olderThan time.Duration    // only branches older than this; 0 means unset
	author    *regexp.Regexp   // matched against "Name <email>"
	patterns  []*regexp.Regexp // branch name must match one of these
}

// explicit reports whether any selection flag replaces the default rule.
func (f *filter) explicit() bool {
	return f.merged || f.unmerged || f.olderThan > 0
}

// match reports whether b is selected. Without explicit flags a branch is
// stale when it is merged or older than defaultStaleAge; explicit flags
// are combined with AND. --author and --pattern always narrow further.
func (f *filter) match(b Branch, now time.Time) bool {
	if f.author != nil && !f.author.MatchString(b.Author+" <"+b.Email+">") {
		return false
	}
	if len(f.patterns) > 0 && !f.matchesPattern(b.Name) {
		return false
	}
	age := now.Sub(b.Date)
	if !f.explicit() {
		return f.all || b.landed() || age > defaultStaleAge
	}
	if f.merged && !b.landed() {
		return false
	}
	if f.unmerged && b.landed() {
		return false
	}
	if f.olderThan > 0 && age <= f.olderThan {
		return false
	}
	return true
}

func (f *filter) matchesPattern(name string) bool {
	for _, re := range f.patterns {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}
