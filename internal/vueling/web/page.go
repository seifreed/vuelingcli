package web

import "context"

// PageReader reads the result of a script executed in a rendered page.
type PageReader interface {
	ReadPage(ctx context.Context, pageURL, script string) ([]byte, error)
}
