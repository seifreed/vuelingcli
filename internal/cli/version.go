package cli

import (
	"encoding/json"
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
			if format != "" && format != arg[2:] {
				return fmt.Errorf("--json and --toon cannot be used together")
			}
			format = arg[2:]
		default:
			return fmt.Errorf("unknown option %s", arg)
		}
	}
	value := map[string]string{"version": cli.version}
	switch format {
	case "json":
		return json.NewEncoder(cli.stdout).Encode(value)
	case "toon":
		return cli.writeTOON(value)
	default:
		_, err := fmt.Fprintf(cli.stdout, "vueling %s\n", cli.version)
		return err
	}
}
