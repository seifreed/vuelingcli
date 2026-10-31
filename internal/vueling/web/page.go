package web

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// PageReader reads the result of a script executed in a rendered page.
type PageReader interface {
	ReadPage(ctx context.Context, pageURL, script string) ([]byte, error)
}

func readPage(ctx context.Context, reader PageReader, pageURL, script string, timeout time.Duration, timeoutMessage string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := reader.ReadPage(ctx, pageURL, script)
	if errors.Is(err, context.DeadlineExceeded) {
		return nil, fmt.Errorf("%s: %w", timeoutMessage, err)
	}
	return output, err
}
