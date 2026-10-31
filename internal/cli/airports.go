package cli

import (
	"fmt"
	"strings"

	"github.com/seifreed/vuelingcli/internal/airports"
)

func (cli *CLI) airportSearch(args []string) error {
	format := ""
	var query []string
	for _, arg := range args {
		switch {
		case arg == "--json" || arg == "--toon":
			requested, err := setOutputFormat(format, arg)
			if err != nil {
				return err
			}
			format = requested
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option %s", arg)
		default:
			query = append(query, arg)
		}
	}
	if len(query) > 1 {
		return fmt.Errorf("use one city, airport, or IATA code")
	}
	term := ""
	if len(query) == 1 {
		term = query[0]
	}
	results := airports.Search(term)
	if format != "" {
		return cli.writeStructured(results, format)
	}
	for _, airport := range results {
		if _, err := fmt.Fprintf(cli.stdout, "%-4s %s\n", airport.Code, airport.City); err != nil {
			return err
		}
	}
	return nil
}

func resolveAirport(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) == 3 {
		return strings.ToUpper(value), nil
	}
	matches := airports.Search(value)
	switch len(matches) {
	case 1:
		return matches[0].Code, nil
	case 0:
		return "", fmt.Errorf("%q is not in the airport catalogue; use an IATA code or 'vueling airports'", value)
	default:
		codes := make([]string, len(matches))
		for i, match := range matches {
			codes[i] = match.Code
		}
		return "", fmt.Errorf("%q matches multiple airports (%s); use an IATA code", value, strings.Join(codes, ", "))
	}
}
