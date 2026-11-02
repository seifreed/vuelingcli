package cli

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

func (cli *CLI) alerts(args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "Read Vueling's current disruption notice (strikes, weather, outages)\n\nUsage:\n  vueling alerts [--culture es-ES|en-GB|…] [--json|--toon]\n")
		return err
	}
	format, culture := "", "es-ES"
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
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
			if key != "culture" || !culturePattern.MatchString(value) {
				return fmt.Errorf("unknown option --%s or invalid culture %q", key, value)
			}
			culture, i = value, next
		default:
			return errors.New("usage: vueling alerts [--culture code] [--json|--toon]")
		}
	}
	if err := cli.requireAPI(); err != nil {
		return err
	}
	alert, err := cli.api.Alert(context.Background(), culture)
	if err != nil {
		return err
	}
	if format != "" {
		return cli.writeStructured(alert, format)
	}
	if !alert.Active {
		_, err = fmt.Fprintln(cli.stdout, "No active Vueling disruption notice")
		return err
	}
	_, err = fmt.Fprintln(cli.stdout, strings.TrimSpace(alert.Text+"\n"+alert.URL))
	return err
}

// culturePattern accepts codes such as es-ES so the value is safe to put in the request URL.
var culturePattern = regexp.MustCompile(`^[A-Za-z]{2}-[A-Za-z]{2}$`)
