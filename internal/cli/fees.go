package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

func (cli *CLI) fees(args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "Read Vueling's price ranges for bags, seats, changes, pets and other extras\n\nUsage:\n  vueling fees [search text] [--json|--toon]\n\nPrices are ranges that vary with route, date and demand; online, customer-service and airport prices differ.\n")
		return err
	}
	format, terms, err := parseSearchTerms(args)
	if err != nil {
		return err
	}
	if err := cli.requireAPI(); err != nil {
		return err
	}
	fees, err := cli.api.Fees(context.Background())
	if err != nil {
		return err
	}
	query := foldAccents(strings.Join(terms, " "))
	var kept []vueling.Fee
	for _, fee := range fees {
		if strings.Contains(foldAccents(fee.Service), query) {
			kept = append(kept, fee)
		}
	}
	if len(kept) == 0 {
		return fmt.Errorf("no Vueling fee matches %q", strings.Join(terms, " "))
	}
	if format != "" {
		return cli.writeStructured(kept, format)
	}
	for _, fee := range kept {
		if _, err := fmt.Fprintf(cli.stdout, "%s\n  online: %s · atención al cliente: %s · aeropuerto: %s\n", fee.Service, orDash(fee.Online), orDash(fee.Phone), orDash(fee.Airport)); err != nil {
			return err
		}
	}
	return nil
}

func foldAccents(s string) string {
	return strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u").Replace(strings.ToLower(s))
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
