package main

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// status is the human-readable merge state of a branch.
func status(b Branch) string {
	if b.Merged {
		return "merged"
	}
	return "unmerged"
}

// relativeAge renders how long ago t was, git-style.
func relativeAge(t, now time.Time) string {
	d := now.Sub(t)
	const day = 24 * time.Hour
	switch {
	case d < 0:
		return "in the future"
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d/time.Minute), "minute") + " ago"
	case d < day:
		return plural(int(d/time.Hour), "hour") + " ago"
	case d < 14*day:
		return plural(int(d/day), "day") + " ago"
	case d < 60*day:
		return plural(int(d/(7*day)), "week") + " ago"
	case d < 365*day:
		return plural(int(d/(30*day)), "month") + " ago"
	default:
		return plural(int(d/(365*day)), "year") + " ago"
	}
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

// writeTable prints branches as an aligned table.
func writeTable(w io.Writer, branches []Branch, now time.Time) {
	rows := [][]string{{"BRANCH", "AGE", "AUTHOR", "STATUS"}}
	for _, b := range branches {
		rows = append(rows, []string{b.Name, relativeAge(b.Date, now), b.Author, status(b)})
	}
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], utf8.RuneCountInString(cell))
		}
	}
	for _, row := range rows {
		var sb strings.Builder
		for i, cell := range row {
			sb.WriteString(cell)
			if i < len(row)-1 {
				sb.WriteString(strings.Repeat(" ", widths[i]-utf8.RuneCountInString(cell)+2))
			}
		}
		fmt.Fprintln(w, sb.String())
	}
}
