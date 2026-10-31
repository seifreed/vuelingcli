package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

func (cli *CLI) flightLink(args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "Build a Vueling flight-selection link\n\nUsage:\n  vueling link <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--adults N] [--children N] [--infants N] [--discount 0-5] [--culture code] [--currency code] [--flight VY1234] [--fare-basis CODE] [--return-flight VY5678] [--return-fare-basis CODE] [--token VALUE] [--open] [--json|--toon]\n")
		return err
	}
	options, err := parseFlightLinkOptions(args)
	if err != nil {
		return err
	}
	link, err := vueling.FlightURL(options.link)
	if err != nil {
		return err
	}
	if options.open {
		if err := openURL(link); err != nil {
			return fmt.Errorf("open flight link: %w", err)
		}
	}
	if options.format == "json" {
		return json.NewEncoder(cli.stdout).Encode(map[string]string{"url": link})
	}
	if options.format == "toon" {
		return cli.writeTOON(map[string]string{"url": link})
	}
	_, err = fmt.Fprintln(cli.stdout, link)
	return err
}

func openURL(link string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", link)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", link)
	case "linux":
		cmd = exec.Command("xdg-open", link)
	default:
		return fmt.Errorf("opening links is not supported on %s", runtime.GOOS)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
