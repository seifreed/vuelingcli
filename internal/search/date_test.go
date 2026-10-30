package search

import (
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	now := time.Date(2026, 10, 31, 16, 0, 0, 0, time.UTC)
	for input, want := range map[string]string{
		"today": "2026-10-31", "tomorrow": "2026-11-01", "+14": "2026-11-14",
		"2026-12-01": "2026-12-01", "02/12/2026": "2026-12-02",
		"saturday": "2026-11-07", "viernes": "2026-11-06", "sat": "2026-11-07",
	} {
		got, err := ParseDate(input, now)
		if err != nil || got.Format(time.DateOnly) != want {
			t.Errorf("ParseDate(%q) = %v, %v; want %s", input, got, err, want)
		}
	}
	if _, err := ParseDate("yesterday", now); err == nil {
		t.Fatal("invalid date was accepted")
	}
}
