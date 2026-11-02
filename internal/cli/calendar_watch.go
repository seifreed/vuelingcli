package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

const minCalendarWatchMinutes = 30

// calendarWatch describes a price watch; now and wait are injected so tests need no real clock.
type calendarWatch struct {
	every     time.Duration
	route     string
	link      string
	currency  string
	threshold float64
	asJSON    bool
	now       func() time.Time
	wait      func(context.Context, time.Duration) bool
}

// watchedFare is one reading of the watched fare: the outbound fare, plus the return fare when there is one.
type watchedFare struct {
	Time       string  `json:"time"`
	Route      string  `json:"route"`
	Date       string  `json:"date"`
	Fare       string  `json:"fare"`
	ReturnDate string  `json:"returnDate,omitempty"`
	ReturnFare string  `json:"returnFare,omitempty"`
	Total      float64 `json:"total"`
	Currency   string  `json:"currency"`
	Previous   float64 `json:"previous,omitempty"`
	Threshold  float64 `json:"threshold,omitempty"`
	Below      bool    `json:"below"`
	URL        string  `json:"url"`
}

// watchCalendar prints the first reading and every price change until the fare reaches the
// threshold, the context ends, or (without a threshold) forever.
func (cli *CLI) watchCalendar(ctx context.Context, watch calendarWatch, read func(context.Context) (map[string]string, error)) error {
	encoder := json.NewEncoder(cli.stdout)
	var previous float64
	err := watchPoll(ctx, watch.every,
		func(ctx context.Context) (watchedFare, error) { return readWatchedFare(ctx, read) },
		func(fare watchedFare, first bool) (bool, error) {
			if !first && fare.Total == previous {
				return false, nil
			}
			if !first {
				fare.Previous = previous
			}
			at := watch.now()
			fare.Time, fare.Route, fare.Currency, fare.URL = at.Format(time.RFC3339), watch.route, watch.currency, watch.link
			fare.Threshold = watch.threshold
			fare.Below = watch.threshold > 0 && fare.Total <= watch.threshold
			if err := cli.writeWatchedFare(encoder, watch.asJSON, at, fare); err != nil {
				return false, err
			}
			if fare.Below {
				return true, nil
			}
			previous = fare.Total
			return false, nil
		},
		func(err error) error { return cli.writeWatchWarning("calendar", watch.every, err) },
		watch.wait,
	)
	if ctx.Err() != nil {
		cli.logWatchCancellation("calendar")
	}
	return err
}

func readWatchedFare(ctx context.Context, read func(context.Context) (map[string]string, error)) (watchedFare, error) {
	price, err := read(ctx)
	if err != nil {
		return watchedFare{}, err
	}
	fare := watchedFare{Date: price["date"], Fare: price["fare"], ReturnDate: price["returnDate"], ReturnFare: price["returnFare"]}
	fares := []string{fare.Fare}
	if fare.ReturnDate != "" {
		fares = append(fares, fare.ReturnFare)
	}
	for _, text := range fares {
		amount, ok := vueling.ParseFare(text)
		if !ok {
			return watchedFare{}, fmt.Errorf("Vueling returned an unreadable fare %q", text)
		}
		fare.Total += amount
	}
	fare.Total = vueling.RoundCents(fare.Total)
	return fare, nil
}

func (cli *CLI) writeWatchedFare(encoder *json.Encoder, asJSON bool, at time.Time, fare watchedFare) error {
	if asJSON {
		return encoder.Encode(fare)
	}
	dates := fare.Date
	if fare.ReturnDate != "" {
		dates += " + " + fare.ReturnDate
	}
	price := vueling.FormatFare(fare.Total, fare.Currency)
	if fare.ReturnDate != "" {
		price += " (" + fare.Fare + " + " + fare.ReturnFare + ")"
	}
	if fare.Previous > 0 {
		price = vueling.FormatFare(fare.Previous, fare.Currency) + " → " + price
	}
	line := fmt.Sprintf("%s %s %s: %s", at.Format("15:04"), fare.Route, dates, price)
	if fare.Below {
		line += fmt.Sprintf("  BELOW %s %s", strconv.FormatFloat(fare.Threshold, 'f', -1, 64), fare.Currency)
	}
	_, err := fmt.Fprintln(cli.stdout, line)
	return err
}

func sleepOrDone(ctx context.Context, every time.Duration) bool {
	timer := time.NewTimer(every)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
