// Package cli runs commands for reading public Vueling fares, flights, and flight status.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/seifreed/vuelingcli/internal/usecase"
	"github.com/seifreed/vuelingcli/internal/vueling"
	"github.com/seifreed/vuelingcli/internal/vueling/ports"
)

// VuelingAPI provides the data operations used by CLI commands.
type VuelingAPI interface {
	Alert(context.Context, string) (vueling.Alert, error)
	Calendar(context.Context, string, string, string) ([]vueling.CalendarDay, error)
	Deals(context.Context, string, []string, time.Time, int, string) ([]vueling.Deal, error)
	Extras(context.Context, usecase.FlightQuery, usecase.FlightPick, string) (vueling.Extras, error)
	Fees(context.Context) ([]vueling.Fee, error)
	Flights(context.Context, usecase.FlightQuery) ([]vueling.Flight, error)
	FlightStatus(context.Context, string, string, string, time.Time) ([]vueling.FlightStatus, error)
	QuoteTrip(context.Context, usecase.FlightQuery, usecase.FlightPick, usecase.FlightPick) ([]vueling.Quote, error)
	Routes(context.Context, string) ([]vueling.Route, error)
	Schedule(context.Context, string, string, string, time.Time, int) ([]vueling.ScheduledDay, error)
	StoredSession() (vueling.Session, error)
	RefreshSession(context.Context) (vueling.Session, error)
	ClearSession(context.Context) error
}

// PageReaderFactory creates a reader for the requested browser endpoint.
type PageReaderFactory func(endpoint string) ports.PageReader

// LinkOpener opens a URL using the host system.
type LinkOpener func(link string) error

// CLI runs commands with caller-provided output and browser dependencies.
type CLI struct {
	version    string
	stdout     io.Writer
	stderr     io.Writer
	newReader  PageReaderFactory
	webSource  ports.WebPageSource
	linkOpener LinkOpener
	logger     *slog.Logger
	api        VuelingAPI
	apiErr     error
}

// New creates a CLI configured with its version, streams, and optional page-reader factory.
func New(version string, stdout, stderr io.Writer, newReader PageReaderFactory) *CLI {
	return &CLI{version: version, stdout: terminalSafeWriter{stdout}, stderr: stderr, newReader: newReader}
}

// WithAPI sets the HTTP client used by commands that call Vueling's JSON APIs instead of Chrome.
func (cli *CLI) WithAPI(client VuelingAPI) *CLI {
	cli.api = client
	cli.apiErr = nil
	return cli
}

// WithAPIError records why the API adapter could not be initialized.
func (cli *CLI) WithAPIError(err error) *CLI {
	cli.apiErr = err
	return cli
}

func (cli *CLI) requireAPI() error {
	if cli.api != nil {
		return nil
	}
	if cli.apiErr != nil {
		return fmt.Errorf("Vueling's HTTP API could not be initialised: %w", cli.apiErr)
	}
	return errors.New("Vueling's HTTP API could not be initialised")
}

// WithWebPageSource sets the adapter used to read Vueling's rendered pages.
func (cli *CLI) WithWebPageSource(source ports.WebPageSource) *CLI {
	cli.webSource = source
	return cli
}

// WithLinkOpener sets the host-system URL opener used by --open.
func (cli *CLI) WithLinkOpener(opener LinkOpener) *CLI {
	cli.linkOpener = opener
	return cli
}

// WithLogger sets the structured logger used for command and browser events.
func (cli *CLI) WithLogger(logger *slog.Logger) *CLI {
	cli.logger = logger
	return cli
}

func (cli *CLI) openLink(link string) error {
	if cli.linkOpener == nil {
		return errors.New("system link opener is not configured")
	}
	started := time.Now()
	err := cli.linkOpener(link)
	if cli.logger != nil {
		outcome := "success"
		attrs := []slog.Attr{
			slog.String("event_name", "browser.open.finished"), slog.String("operation", "open_url"),
			slog.Int64("duration_ms", time.Since(started).Milliseconds()),
		}
		if err != nil {
			outcome = "error"
			attrs = append(attrs, slog.String("error_type", errorType(err)))
		}
		attrs = append(attrs, slog.String("outcome", outcome))
		cli.logger.LogAttrs(context.Background(), slog.LevelDebug, "System browser open completed", attrs...)
	}
	return err
}

func (cli *CLI) webPageSource() (ports.WebPageSource, error) {
	if cli.webSource == nil {
		return nil, errors.New("browser page source is not configured")
	}
	return cli.webSource, nil
}

func (cli *CLI) pageReader(endpoint string) (ports.PageReader, error) {
	if cli.newReader == nil {
		return nil, errors.New("browser page reader is not configured")
	}
	reader := cli.newReader(endpoint)
	if reader == nil {
		return nil, errors.New("browser page reader is not configured")
	}
	if cli.logger != nil {
		return observedPageReader{reader: reader, logger: cli.logger}, nil
	}
	return reader, nil
}

type observedPageReader struct {
	reader ports.PageReader
	logger *slog.Logger
}

func (reader observedPageReader) ReadPage(ctx context.Context, pageURL, script string) ([]byte, error) {
	started := time.Now()
	output, err := reader.reader.ReadPage(ctx, pageURL, script)
	outcome := "success"
	attrs := []slog.Attr{
		slog.String("event_name", "browser.page_read.finished"), slog.String("operation", "read_page"),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
	}
	if err != nil {
		outcome = "error"
		if errors.Is(err, context.Canceled) {
			outcome = "cancelled"
		}
		attrs = append(attrs, slog.String("error_type", errorType(err)))
	}
	attrs = append(attrs, slog.String("outcome", outcome))
	reader.logger.LogAttrs(ctx, slog.LevelDebug, "Browser page read completed", attrs...)
	return output, err
}

func isHelp(args []string) bool {
	if len(args) != 1 {
		return false
	}
	return args[0] == "--help" || args[0] == "-h"
}
