package web

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling/ports"
)

// PageReader reads rendered Vueling pages through the browser port.
type PageReader = ports.PageReader

// Source reads and decodes Vueling's rendered web pages.
type Source struct{}

func readPage(ctx context.Context, reader PageReader, pageURL, script string, timeout time.Duration, timeoutMessage string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := reader.ReadPage(ctx, pageURL, script)
	if errors.Is(err, context.DeadlineExceeded) {
		return nil, fmt.Errorf("%s: %w", timeoutMessage, err)
	}
	return output, err
}
