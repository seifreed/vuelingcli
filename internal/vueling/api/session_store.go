package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

type storedSession struct {
	Token        string    `json:"token"`
	Expires      time.Time `json:"expires"`
	TokenExpires time.Time `json:"tokenExpires,omitzero"`
}

// DefaultSessionFile is where the CLI keeps its anonymous session between runs, or "" when the
// user has no cache directory.
func DefaultSessionFile() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "vuelingcli", "session.json")
}

// StoredSession returns the session saved by an earlier run, without calling Vueling.
func (c *Client) StoredSession() (session Session, err error) {
	if c.SessionFile == "" {
		return Session{}, nil
	}
	started, cacheHit := time.Now(), false
	defer func() {
		c.logOperation(context.Background(), slog.LevelDebug, "session.cache.read", "Session cache read completed", "session_cache_read", started, err, slog.Bool("cache_hit", cacheHit))
	}()
	data, err := os.ReadFile(c.SessionFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Session{}, nil
		}
		return Session{}, operationError{operation: "read the stored Vueling session", cause: err}
	}
	cacheHit = true
	var stored storedSession
	if err := json.Unmarshal(data, &stored); err != nil {
		return Session{}, fmt.Errorf("decode the stored Vueling session: %w", err)
	}
	return Session{Token: stored.Token, Expires: stored.Expires, TokenExpires: stored.TokenExpires}, nil
}

func (c *Client) storeSession(ctx context.Context, session Session) (err error) {
	if c.SessionFile == "" {
		return nil
	}
	started := time.Now()
	defer func() {
		c.logOperation(ctx, slog.LevelDebug, "session.cache.write", "Session cache write completed", "session_cache_write", started, err)
	}()
	return storeSessionFile(c.SessionFile, session, os.CreateTemp)
}

func storeSessionFile(path string, session Session, createTemp func(string, string) (*os.File, error)) error {
	if path == "" {
		return nil
	}
	data, err := json.Marshal(storedSession{Token: session.Token, Expires: session.Expires, TokenExpires: session.TokenExpires})
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return operationError{operation: "store the Vueling session", cause: err}
	}
	file, err := createTemp(dir, "session-*.json")
	if err != nil {
		return operationError{operation: "store the Vueling session", cause: err}
	}
	cleanup := func(cause error) error {
		if err := os.Remove(file.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.Join(cause, operationError{operation: "remove the temporary Vueling session file", cause: err})
		}
		return cause
	}
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return cleanup(operationError{operation: "store the Vueling session", cause: err})
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return cleanup(operationError{operation: "store the Vueling session", cause: err})
	}
	return nil
}
