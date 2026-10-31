// Command vueling reads public Vueling fares, flights, and flight status.
package main

import (
	"os"

	"github.com/seifreed/vuelingcli/internal/browser"
	"github.com/seifreed/vuelingcli/internal/cli"
	"github.com/seifreed/vuelingcli/internal/vueling/api"
	"github.com/seifreed/vuelingcli/internal/vueling/web"
)

var version = "0.1.0"

func main() {
	program := cli.New(version, os.Stdout, os.Stderr, func(endpoint string) web.PageReader {
		return browser.NewChromeReader(endpoint)
	})
	if client, err := api.New(); err == nil {
		program.WithAPI(client)
	}
	os.Exit(program.Run(os.Args[1:]))
}
