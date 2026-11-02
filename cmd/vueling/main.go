// Command vueling reads public Vueling fares, flights, and flight status.
package main

import (
	"fmt"
	"os"

	"github.com/seifreed/vuelingcli/internal/browser"
	"github.com/seifreed/vuelingcli/internal/cli"
	"github.com/seifreed/vuelingcli/internal/observability"
	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/ports"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

var version = "0.1.0"
var newAPIClient = api.New
var exitProcess = os.Exit

func newPageReader(endpoint string) ports.PageReader {
	return browser.NewChromeReader(endpoint)
}

func main() {
	config, warnings := observability.ConfigFromEnv(os.Getenv)
	logger := observability.New(os.Stderr, config)
	processLogger := logger.With("component", "process")
	processLogger.Info("Process started", "event_name", "process.started", "operation", "startup", "version", version)
	for _, setting := range warnings {
		processLogger.Warn("Invalid logging configuration; using the default", "event_name", "logging.configuration.invalid", "operation", "logger_setup", "setting", setting)
	}
	program := cli.New(version, os.Stdout, os.Stderr, newPageReader).
		WithWebPageSource(web.Source{}).
		WithLinkOpener(browser.OpenURL).
		WithLogger(logger.With("component", "cli"))
	if client, err := newAPIClient(); err == nil {
		client.Logger = logger.With("component", "api")
		program.WithAPI(client)
	} else {
		processLogger.Warn("HTTP API client is unavailable", "event_name", "api_client.unavailable", "operation", "api_client_init", "error_type", fmt.Sprintf("%T", err))
		program.WithAPIError(err)
	}
	exitCode := program.Run(os.Args[1:])
	outcome := "success"
	if exitCode != 0 {
		outcome = "error"
	}
	processLogger.Info("Process stopped", "event_name", "process.stopped", "operation", "shutdown", "outcome", outcome, "exit_code", exitCode)
	exitProcess(exitCode)
}
