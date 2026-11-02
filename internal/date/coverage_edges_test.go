package date

import (
	"testing"
	"time"
)

func TestParseDateRejectsRelativeDatesOutsideSupportedRange(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		text string
	}{
		{"current year beyond range", time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC), "+1"},
		{"relative offset exceeds range", time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), "+367"},
		{"computed date exceeds range", time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), "+366"},
		{"non-numeric offset", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "+days"},
		{"negative offset", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "+-1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseDate(test.text, test.now); err == nil {
				t.Fatalf("ParseDate(%q, %s) succeeded beyond year 9999", test.text, test.now.Format(time.DateOnly))
			}
		})
	}
}
