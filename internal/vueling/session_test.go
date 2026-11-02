package vueling

import (
	"testing"
	"time"
)

func TestSessionActive(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		s    Session
		want bool
	}{
		{"active", Session{Token: "token", Expires: at.Add(time.Second)}, true},
		{"expired", Session{Token: "token", Expires: at}, false},
		{"missing token", Session{Expires: at.Add(time.Second)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.Active(at); got != tc.want {
				t.Fatalf("Active(%v) = %v, want %v", at, got, tc.want)
			}
		})
	}
}

func TestSessionExpiresAt(t *testing.T) {
	at := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	validity := time.Hour
	for _, tc := range []struct {
		name         string
		tokenExpires time.Time
		want         time.Time
	}{
		{"validity limit", time.Time{}, at.Add(validity)},
		{"earlier token expiry", at.Add(30 * time.Minute), at.Add(30 * time.Minute)},
		{"later token expiry", at.Add(2 * time.Hour), at.Add(validity)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := Session{TokenExpires: tc.tokenExpires}
			if got := s.ExpiresAt(at, validity); !got.Equal(tc.want) {
				t.Fatalf("ExpiresAt = %v, want %v", got, tc.want)
			}
		})
	}
}
