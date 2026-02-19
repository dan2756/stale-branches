package main

import (
	"bytes"
	"testing"
	"time"
)

func TestRelativeAge(t *testing.T) {
	tests := []struct {
		ago  time.Duration
		want string
	}{
		{-time.Hour, "in the future"},
		{10 * time.Second, "just now"},
		{time.Minute, "1 minute ago"},
		{45 * time.Minute, "45 minutes ago"},
		{5 * time.Hour, "5 hours ago"},
		{day, "1 day ago"},
		{13 * day, "13 days ago"},
		{15 * day, "2 weeks ago"},
		{59 * day, "8 weeks ago"},
		{90 * day, "3 months ago"},
		{364 * day, "12 months ago"},
		{400 * day, "1 year ago"},
		{800 * day, "2 years ago"},
	}
	for _, tt := range tests {
		if got := relativeAge(refTime.Add(-tt.ago), refTime); got != tt.want {
			t.Errorf("relativeAge(%v) = %q, want %q", tt.ago, got, tt.want)
		}
	}
}

func TestWriteTable(t *testing.T) {
	branches := []Branch{
		{Name: "feature/login", Date: refTime.Add(-120 * day), Author: "Alice", Merged: true, Behind: 12},
		{Name: "wip", Date: refTime.Add(-3 * time.Hour), Author: "Bob Smith", Ahead: 104, Behind: 3},
	}
	var buf bytes.Buffer
	writeTable(&buf, branches, refTime)
	want := "" +
		"BRANCH         AGE           AUTHOR     STATUS    AHEAD  BEHIND\n" +
		"feature/login  4 months ago  Alice      merged        0      12\n" +
		"wip            3 hours ago   Bob Smith  unmerged    104       3\n"
	if got := buf.String(); got != want {
		t.Errorf("table:\n%s\nwant:\n%s", got, want)
	}
}
