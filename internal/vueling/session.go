package vueling

import "time"

// Session tracks the anonymous bearer token and its effective expiry.
type Session struct {
	Token        string
	Expires      time.Time
	TokenExpires time.Time
}

// Active reports whether the session has a token that remains valid at at.
func (s Session) Active(at time.Time) bool {
	return s.Token != "" && at.Before(s.Expires)
}

// ExpiresAt returns the earlier of the session validity and token expiry.
func (s Session) ExpiresAt(at time.Time, validity time.Duration) time.Time {
	expires := at.Add(validity)
	if !s.TokenExpires.IsZero() && s.TokenExpires.Before(expires) {
		return s.TokenExpires
	}
	return expires
}
