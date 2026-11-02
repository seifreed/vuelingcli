package cli

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

func (cli *CLI) selectFlight(args []string) error {
	if isHelp(args) {
		_, err := fmt.Fprint(cli.stdout, "Price a flight the way Vueling's website does when you click its fare, without booking anything\n\nUsage:\n  vueling select <origin city or IATA> <destination city or IATA> --date <date> (--flight VY1234 | --cheapest) [--return <date> (--return-flight VY1234 | --return-cheapest)] [--direct] [--adults N] [--children N] [--infants N] [--discount 0-5] [--currency code] [--json|--toon]\n\nShows the price breakdown of the flight's cheapest fare (fare, taxes and fees, the infant fee for lap infants, total for every passenger) and the total with each bundle the website offers: Fly Light, Fly, Fly (25 kg bag), Fly Pro and Fly Grande. --flight takes the first flight number of a connection, or every number joined with +, such as VY1383+VY1050. --cheapest picks the day's cheapest bookable flight instead, and --direct limits every cheapest pick to direct flights.\n\nWith --return, pick the return flight the same way with --return-flight or --return-cheapest. Each direction gets its own breakdown and bundle totals, since the website picks a bundle per journey, and a last line adds both cheapest fares into the trip total.\n\nThis command only reads prices: it never selects, sells, or books a fare. Confirm and pay on vueling.com through the printed link.\n")
		return err
	}
	options, outbound, inbound, err := parseSelectOptions(args)
	if err != nil {
		return err
	}
	if err := cli.requireAPI(); err != nil {
		return err
	}
	quotes, err := cli.api.QuoteTrip(context.Background(), options.flightQuery(), outbound, inbound)
	if err != nil {
		return err
	}
	link, err := selectLink(options, quotes)
	if err != nil {
		return err
	}
	if options.format != "" {
		result := map[string]any{"quote": quotes[0], "url": link, "booked": false}
		if len(quotes) == 2 {
			result["returnQuote"], result["total"] = quotes[1], vueling.QuoteTripTotal(quotes)
		}
		return cli.writeStructured(result, options.format)
	}
	for _, quote := range quotes {
		if err := cli.printQuote(quote); err != nil {
			return err
		}
	}
	if len(quotes) == 2 {
		if _, err := fmt.Fprintf(cli.stdout, "trip total %s %s\n", vueling.FormatAmount(vueling.QuoteTripTotal(quotes)), quotes[0].Currency); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(cli.stdout, "Nothing was booked. Confirm on vueling.com: %s\n", link)
	return err
}

func (cli *CLI) printQuote(quote vueling.Quote) error {
	bundles := make([]string, 0, len(quote.Bundles))
	for _, bundle := range quote.Bundles {
		bundles = append(bundles, bundle.Name+" "+vueling.FormatAmount(bundle.Total))
	}
	_, err := fmt.Fprintf(cli.stdout, "%s %s %s %s → %s %s\nfare %s + taxes and fees %s%s%s = %s %s\n%s\n",
		quote.Flight, quote.Date, quote.Departure, quote.Origin, vueling.ArrivalClock(quote.Arrival, quote.Date, quote.ArrivalDate), quote.Destination,
		vueling.FormatAmount(quote.Fare), vueling.FormatAmount(quote.Taxes), taxSummary(quote.TaxDetail), infantSummary(quote), vueling.FormatAmount(quote.Total), quote.Currency,
		strings.Join(bundles, " · "))
	return err
}

// selectLink builds the website link that selects the quoted flights, naming a connection by its first flight.
func selectLink(options flightOptions, quotes []vueling.Quote) (string, error) {
	return vueling.QuoteSelectionURL(options.linkOptions(), quotes)
}

// infantSummary adds the lap-infant fare and fees, such as " + infant fee 47.96 (1 infant)".
func infantSummary(quote vueling.Quote) string {
	if quote.InfantFee == 0 {
		return ""
	}
	noun := "infant"
	if quote.Infants != 1 {
		noun = "infants"
	}
	return fmt.Sprintf(" + infant fee %s (%d %s)", vueling.FormatAmount(quote.InfantFee), quote.Infants, noun)
}

// taxSummary lists the tax codes inside taxes and fees, such as " (ES 5.91, JD 13.98)".
func taxSummary(taxes map[string]float64) string {
	if len(taxes) == 0 {
		return ""
	}
	codes := make([]string, 0, len(taxes))
	for code := range taxes {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	parts := make([]string, 0, len(codes))
	for _, code := range codes {
		parts = append(parts, code+" "+vueling.FormatAmount(taxes[code]))
	}
	return " (" + strings.Join(parts, ", ") + ")"
}
