package cli

import (
	"context"
	"log/slog"
	"time"
)

func watchPoll[T any](ctx context.Context, every time.Duration, read func(context.Context) (T, error), onValue func(T, bool) (bool, error), onError func(error) error, wait func(context.Context, time.Duration) bool) error {
	first := true
	for {
		value, err := read(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			if first {
				return err
			}
			if err := onError(err); err != nil {
				return err
			}
		} else if done, err := onValue(value, first); err != nil || done {
			return err
		}
		first = false
		if !wait(ctx, every) {
			return nil
		}
	}
}

func (cli *CLI) writeWatchWarning(command string, every time.Duration, readErr error) error {
	if cli.logger != nil {
		attrs := []slog.Attr{
			slog.String("event_name", "watch.retry"), slog.String("operation", command),
			slog.Int64("retry_in_ms", every.Milliseconds()), slog.String("outcome", "retrying"),
			slog.String("error_type", errorType(readErr)),
		}
		if code := httpStatusCode(readErr); code != 0 {
			attrs = append(attrs, slog.Int("status_code", code))
		}
		cli.logger.LogAttrs(context.Background(), slog.LevelWarn, "Watch read failed; retry scheduled", attrs...)
	}
	return nil
}

func (cli *CLI) logWatchCancellation(command string) {
	if cli.logger != nil {
		cli.logger.Info("Watch stopped after cancellation", "event_name", "watch.stopped", "operation", command, "outcome", "cancelled", "error_code", "context_canceled")
	}
}
