// Package observability configures the CLI's structured stderr logger.
package observability

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"unicode"
)

// TraceLevel is the conventional slog level below DEBUG.
const TraceLevel = slog.Level(-8)

// Config controls the process logger's level, encoding, and environment label.
type Config struct {
	Environment string
	Level       slog.Level
	Format      string
}

// ConfigFromEnv reads the VUELING logging environment variables; warnings name invalid settings only.
func ConfigFromEnv(getenv func(string) string) (Config, []string) {
	if getenv == nil {
		getenv = os.Getenv
	}
	cfg := Config{Environment: strings.TrimSpace(getenv("VUELING_ENV")), Level: slog.LevelInfo}
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}
	var warnings []string
	if value := getenv("VUELING_LOG_LEVEL"); value != "" {
		level, ok := parseLevel(value)
		if ok {
			cfg.Level = level
		} else {
			warnings = append(warnings, "VUELING_LOG_LEVEL")
		}
	}
	cfg.Format = "text"
	if strings.EqualFold(cfg.Environment, "production") {
		cfg.Format = "json"
	}
	if value := getenv("VUELING_LOG_FORMAT"); value != "" {
		format := strings.ToLower(strings.TrimSpace(value))
		if format == "json" || format == "text" {
			cfg.Format = format
		} else {
			warnings = append(warnings, "VUELING_LOG_FORMAT")
		}
	}
	return cfg, warnings
}

// New creates a concurrent-safe structured logger that writes only to w.
func New(w io.Writer, cfg Config) *slog.Logger {
	if w == nil {
		w = io.Discard
	}
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}
	opts := &slog.HandlerOptions{Level: cfg.Level, ReplaceAttr: replaceAttr}
	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}
	return slog.New(handler).With("service", "vuelingcli", "environment", cfg.Environment)
}

func parseLevel(value string) (slog.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "trace":
		return TraceLevel, true
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn", "warning":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return 0, false
	}
}

func replaceAttr(groups []string, attr slog.Attr) slog.Attr {
	for _, group := range groups {
		if sensitiveKey(group) {
			attr.Value = slog.StringValue("[REDACTED]")
			return attr
		}
	}
	if attr.Value.Kind() == slog.KindGroup {
		return attr
	}
	switch attr.Key {
	case slog.TimeKey:
		attr.Key = "timestamp"
		attr.Value = slog.TimeValue(attr.Value.Time().UTC())
	case slog.LevelKey:
		attr.Key = "level"
		if level, ok := attr.Value.Any().(slog.Level); ok {
			attr.Value = slog.StringValue(levelName(level))
		}
	case slog.MessageKey:
		attr.Key = "message"
	}
	if sensitiveKey(attr.Key) {
		attr.Value = slog.StringValue("[REDACTED]")
	} else if attr.Value.Kind() == slog.KindString {
		attr.Value = slog.StringValue(cleanString(attr.Value.String()))
	}
	return attr
}

func levelName(level slog.Level) string {
	switch level {
	case TraceLevel:
		return "trace"
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	default:
		return strings.ToLower(level.String())
	}
}

func sensitiveKey(key string) bool {
	key = strings.ToLower(strings.NewReplacer("-", "_", ".", "_").Replace(key))
	for _, part := range []string{"token", "password", "passwd", "authorization", "cookie", "secret", "api_key", "apikey", "private_key", "privatekey", "credential", "session"} {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}

func cleanString(value string) string {
	const maxLength = 256
	var b strings.Builder
	for _, r := range value {
		if b.Len() >= maxLength {
			b.WriteString("…")
			break
		}
		if unicode.IsControl(r) {
			r = ' '
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
