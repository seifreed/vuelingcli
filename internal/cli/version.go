package cli

import (
	"fmt"
)

func (cli *CLI) showVersion(args []string) error {
	format := ""
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			if _, err := fmt.Fprint(cli.stdout, "Show the CLI version\n\nUsage:\n  vueling version [--json|--toon]\n"); err != nil {
				return err
			}
			return nil
		case "--json", "--toon":
			requested, err := setOutputFormat(format, arg)
			if err != nil {
				return err
			}
			format = requested
		default:
			return fmt.Errorf("unknown option %s", arg)
		}
	}
	value := map[string]string{"version": cli.version}
	if format != "" {
		return cli.writeStructured(value, format)
	}
	_, err := fmt.Fprintf(cli.stdout, "vueling %s\n", cli.version)
	return err
}
