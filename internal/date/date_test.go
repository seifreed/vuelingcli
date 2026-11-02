package date

import (
	"strconv"
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	now := time.Date(2026, 10, 31, 16, 0, 0, 0, time.UTC)
	for input, want := range map[string]string{
		"today": "2026-10-31", "tomorrow": "2026-11-01", "+14": "2026-11-14", "hoy": "2026-10-31", "Mañana": "2026-11-01",
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

func TestParseDateRejectsRelativeDateOverflow(t *testing.T) {
	now := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	input := "+" + strconv.Itoa(int(^uint(0)>>1))
	if got, err := ParseDate(input, now); err == nil {
		t.Fatalf("ParseDate(%q) = %v, nil; want a range error", input, got)
	}
}

func TestParseDateRejectsCalendarDateOverflow(t *testing.T) {
	now := time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
	for _, input := range []string{"tomorrow", "monday"} {
		if got, err := ParseDate(input, now); err == nil {
			t.Errorf("ParseDate(%q) = %v, nil; want a range error", input, got)
		}
	}
}

func TestParseDateAllowingPast(t *testing.T) {
	now := time.Date(2026, 10, 31, 16, 0, 0, 0, time.UTC)
	for _, input := range []string{"yesterday", "Ayer"} {
		if got, err := ParseDateAllowingPast(input, now); err != nil || got.Format(time.DateOnly) != "2026-10-30" {
			t.Errorf("ParseDateAllowingPast(%q) = %v, %v; want 2026-10-30", input, got, err)
		}
		if _, err := ParseDate(input, now); err == nil {
			t.Errorf("ParseDate(%q) accepted a past day", input)
		}
	}
	if _, err := ParseDate("2026-10-29", now); err == nil {
		t.Fatal("ParseDate accepted a past date")
	}
	got, err := ParseDateAllowingPast("29/10/2026", now)
	if err != nil || got.Format(time.DateOnly) != "2026-10-29" {
		t.Fatalf("ParseDateAllowingPast = %v, %v; want 2026-10-29", got, err)
	}
	if got, err := ParseDateAllowingPast("hoy", now); err != nil || got.Format(time.DateOnly) != "2026-10-31" {
		t.Fatalf("ParseDateAllowingPast(hoy) = %v, %v", got, err)
	}
}
