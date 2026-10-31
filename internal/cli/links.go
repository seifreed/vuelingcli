package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	dateparse "github.com/seifreed/vuelingcli/internal/date"
	"github.com/seifreed/vuelingcli/internal/vueling"
)

func (cli *CLI) flightLink(args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "Build a Vueling flight-selection link\n\nUsage:\n  vueling link <origin city or IATA> <destination city or IATA> --date <date> [--return <date>] [--adults N] [--children N] [--infants N] [--discount 0-5] [--culture code] [--currency code] [--flight VY1234] [--fare-basis CODE] [--return-flight VY5678] [--return-fare-basis CODE] [--token VALUE] [--open] [--json|--toon]\n")
		return err
	}
	values := map[string]string{}
	var positional []string
	format := ""
	open := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--open" {
			open = true
			continue
		}
		if a == "--json" || a == "--toon" {
			if format != "" {
				return fmt.Errorf("--json and --toon cannot be used together")
			}
			format = a[2:]
			continue
		}
		if len(a) > 2 && a[:2] == "--" {
			k, v, hasValue := strings.Cut(a[2:], "=")
			if !hasValue {
				remaining := args[i:]
				if len(remaining) < 2 {
					return fmt.Errorf("--%s requires a value", k)
				}
				v = remaining[1]
				i++
			}
			switch k {
			case "date", "return", "adults", "children", "infants", "discount", "culture", "currency", "flight", "fare-basis", "return-flight", "return-fare-basis", "token":
				values[k] = v
			default:
				return fmt.Errorf("unknown option --%s", k)
			}
			continue
		}
		positional = append(positional, a)
	}
	if len(positional) != 2 || values["date"] == "" {
		return fmt.Errorf("usage: vueling link <origin city or IATA> <destination city or IATA> --date <date> [options]")
	}
	origin, err := resolveAirport(positional[0])
	if err != nil {
		return fmt.Errorf("origin: %w", err)
	}
	destination, err := resolveAirport(positional[1])
	if err != nil {
		return fmt.Errorf("destination: %w", err)
	}
	parseDate := func(key string) (time.Time, error) {
		if values[key] == "" {
			return time.Time{}, nil
		}
		return dateparse.ParseDate(values[key], time.Now())
	}
	date, err := parseDate("date")
	if err != nil {
		return err
	}
	returnDate, err := parseDate("return")
	if err != nil {
		return err
	}
	integer := func(key string, fallback int) (int, error) {
		if values[key] == "" {
			return fallback, nil
		}
		return strconv.Atoi(values[key])
	}
	adults, err := integer("adults", 1)
	if err != nil {
		return fmt.Errorf("invalid --adults: %w", err)
	}
	children, err := integer("children", 0)
	if err != nil {
		return fmt.Errorf("invalid --children: %w", err)
	}
	infants, err := integer("infants", 0)
	if err != nil {
		return fmt.Errorf("invalid --infants: %w", err)
	}
	discount, err := integer("discount", 0)
	if err != nil {
		return fmt.Errorf("invalid --discount: %w", err)
	}
	link, err := vueling.FlightURL(vueling.LinkOptions{Origin: origin, Destination: destination, Date: date, ReturnDate: returnDate, Adults: adults, Children: children, Infants: infants, Discount: discount, Culture: values["culture"], Currency: values["currency"], OutboundFlight: values["flight"], OutboundFareBasis: values["fare-basis"], ReturnFlight: values["return-flight"], ReturnFareBasis: values["return-fare-basis"], METToken: values["token"]})
	if err != nil {
		return err
	}
	if open {
		if err := openURL(link); err != nil {
			return fmt.Errorf("open flight link: %w", err)
		}
	}
	if format == "json" {
		return json.NewEncoder(cli.stdout).Encode(map[string]string{"url": link})
	}
	if format == "toon" {
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
