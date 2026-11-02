package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

// reuseMargin is how much life a stored session needs left to be reused without extending it.
const reuseMargin = 2 * time.Minute

const maxSessionValiditySeconds = int64(^uint64(0)>>1) / int64(time.Second)

// Session aliases the application-level anonymous session type.
type Session = vueling.Session

// RefreshSession extends the stored session, or opens a new one when it has expired or cannot
// be extended, and stores the result.
func (c *Client) RefreshSession(ctx context.Context) (Session, error) {
	if c.SessionFile == "" {
		return Session{}, errors.New("no cache directory to keep the session in")
	}
	session, err := c.StoredSession()
	if err != nil {
		return Session{}, err
	}
	session, err = c.renewSession(ctx, session)
	if err != nil {
		return Session{}, err
	}
	return session, c.storeSession(ctx, session)
}

// ClearSession ends the remote session and removes the local copy.
func (c *Client) ClearSession(ctx context.Context) error {
	session, endErr := c.StoredSession()
	if endErr == nil && session.Active(time.Now()) {
		if err := c.do(ctx, http.MethodDelete, c.AMS+"/asm/v1/Auth", ticketsOrigin, session.Token, nil, new([]byte)); err != nil {
			endErr = fmt.Errorf("end the Vueling session: %w", err)
		}
	}
	var forgetErr error
	if c.SessionFile != "" {
		started := time.Now()
		removeErr := os.Remove(c.SessionFile)
		if errors.Is(removeErr, os.ErrNotExist) {
			removeErr = nil
		}
		c.logOperation(ctx, slog.LevelDebug, "session.cache.remove", "Session cache removal completed", "session_cache_remove", started, removeErr)
		if removeErr != nil {
			forgetErr = operationError{operation: "forget the Vueling session", cause: removeErr}
		}
	}
	return errors.Join(endErr, forgetErr)
}

// anonymousToken reuses the stored session while it has life left, then extends or replaces it.
func (c *Client) anonymousToken(ctx context.Context) (string, error) {
	session, err := c.StoredSession()
	if err != nil {
		return "", err
	}
	if session.Active(time.Now().Add(reuseMargin)) {
		return session.Token, nil
	}
	session, err = c.renewSession(ctx, session)
	if err != nil {
		return "", err
	}
	if err := c.storeSession(ctx, session); err != nil {
		return "", err
	}
	return session.Token, nil
}

// withSession runs call with the reused session and, if Vueling rejects that token before its
// recorded expiry, once more with a new session.
func withSession[T any](ctx context.Context, c *Client, call func(token string) (T, error)) (T, error) {
	var zero T
	token, err := c.anonymousToken(ctx)
	if err != nil {
		return zero, err
	}
	result, err := call(token)
	var status *StatusError
	if !errors.As(err, &status) || status.Status != http.StatusUnauthorized {
		return result, err
	}
	session, err := c.openSession(ctx)
	if err != nil {
		return zero, err
	}
	if err := c.storeSession(ctx, session); err != nil {
		return zero, err
	}
	return call(session.Token)
}

func (c *Client) renewSession(ctx context.Context, session Session) (Session, error) {
	if session.Active(time.Now()) {
		extended, extendErr := c.extendSession(ctx, session)
		if extendErr == nil && extended.Active(time.Now().Add(reuseMargin)) {
			return extended, nil
		}
		opened, openErr := c.openSession(ctx)
		if openErr != nil {
			return Session{}, errors.Join(extendErr, openErr)
		}
		return opened, nil
	}
	return c.openSession(ctx)
}

func (c *Client) openSession(ctx context.Context) (Session, error) {
	var opened struct {
		AccessToken string `json:"accessToken"`
		Expiration  int64  `json:"expiration"`
	}
	if err := c.postJSON(ctx, c.AMS+"/asm/v1/Auth", ticketsOrigin, "", map[string]string{"profileId": anonymousProfileID}, &opened); err != nil {
		return Session{}, fmt.Errorf("open an anonymous Vueling session: %w", err)
	}
	if opened.AccessToken == "" {
		return Session{}, errors.New("Vueling did not return an anonymous session token")
	}
	validity, err := sessionValidity(opened.Expiration)
	if err != nil {
		return Session{}, fmt.Errorf("open an anonymous Vueling session: %w", err)
	}
	session := Session{Token: opened.AccessToken, TokenExpires: tokenExpiry(opened.AccessToken)}
	session.Expires = session.ExpiresAt(time.Now(), validity)
	return session, nil
}

// extendSession keeps the session alive the way the website does; Vueling answers with the
// seconds the session now has left and keeps the same token.
func (c *Client) extendSession(ctx context.Context, session Session) (Session, error) {
	var extended struct {
		Expires int64 `json:"expires"`
	}
	if err := c.do(ctx, http.MethodPut, c.AMS+"/asm/v1/Auth", ticketsOrigin, session.Token, struct{}{}, &extended); err != nil {
		return Session{}, fmt.Errorf("extend the Vueling session: %w", err)
	}
	validity, err := sessionValidity(extended.Expires)
	if err != nil {
		return Session{}, fmt.Errorf("extend the Vueling session: %w", err)
	}
	session.Expires = session.ExpiresAt(time.Now(), validity)
	return session, nil
}

func sessionValidity(seconds int64) (time.Duration, error) {
	if seconds < 0 || seconds > maxSessionValiditySeconds {
		return 0, errors.New("Vueling returned an invalid session duration")
	}
	return time.Duration(seconds) * time.Second, nil
}

// tokenExpiry reads a JWT's exp claim without verifying it, or returns zero when it has none.
func tokenExpiry(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp == 0 {
		return time.Time{}
	}
	return time.Unix(claims.Exp, 0)
}
