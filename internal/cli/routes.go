package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

func (cli *CLI) routes(args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "List every destination Vueling flies to from an origin\n\nUsage:\n  vueling routes <origin city or IATA> [--direct] [--country code-or-name] [--json|--toon]\n\nReads the route network Vueling's website uses for its search form. Destinations reached only with a connection are marked \"conexión\".\n")
		return err
	}
	var positional []string
	format, country, direct := "", "", false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--direct":
			direct = true
		case a == "--json" || a == "--toon":
			requested, err := setOutputFormat(format, a)
			if err != nil {
				return err
			}
			format = requested
		case strings.HasPrefix(a, "--"):
			key, value, next, err := parseLongOption(args, i)
			if err != nil {
				return err
			}
			if key != "country" {
				return fmt.Errorf("unknown option --%s", key)
			}
			country, i = value, next
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 1 {
		return errors.New("usage: vueling routes <origin> [--direct] [--country code-or-name]")
	}
	origin, err := resolveAirport(positional[0])
	if err != nil {
		return err
	}
	if err := cli.requireAPI(); err != nil {
		return err
	}
	routes, err := cli.api.Routes(context.Background(), origin)
	if err != nil {
		return err
	}
	var kept []vueling.Route
	for _, route := range routes {
		if (direct && !route.Direct) || (country != "" && !strings.EqualFold(route.CountryCode, country) && !strings.EqualFold(route.Country, country)) {
			continue
		}
		kept = append(kept, route)
	}
	if len(kept) == 0 {
		return fmt.Errorf("Vueling flies to no destination from %s matching those options", origin)
	}
	if format != "" {
		return cli.writeStructured(kept, format)
	}
	for _, route := range kept {
		kind := "directo"
		if !route.Direct {
			kind = "conexión"
		}
		if _, err := fmt.Fprintf(cli.stdout, "%-4s %-28s %-16s %s\n", route.Code, route.Name, route.Country, kind); err != nil {
			return err
		}
	}
	return nil
}
