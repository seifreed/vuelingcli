package cli

import (
	"fmt"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

func (cli *CLI) flightLink(command string, args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "Build a Vueling flight-selection link\n\nUsage:\n  vueling link <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--adults N] [--children N] [--infants N] [--discount 0-5] [--culture code] [--currency code] [--flight VY1234] [--fare-basis CODE] [--return-flight VY5678] [--return-fare-basis CODE] [--token VALUE] [--open] [--json|--toon]\n")
		return err
	}
	options, err := parseFlightLinkOptions(command, args)
	if err != nil {
		return err
	}
	link, err := vueling.FlightURL(options.link)
	if err != nil {
		return err
	}
	if options.open {
		if err := cli.openLink(link); err != nil {
			return fmt.Errorf("open flight link: %w", err)
		}
	}
	if options.format != "" {
		return cli.writeStructured(map[string]string{"url": link}, options.format)
	}
	_, err = fmt.Fprintln(cli.stdout, link)
	return err
}
