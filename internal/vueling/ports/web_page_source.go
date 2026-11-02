package ports

import (
	"context"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

// WebPageSource reads Vueling data from its rendered website through a page reader.
type WebPageSource interface {
	CalendarPricesFromPage(context.Context, PageReader, string, string) (map[string]string, error)
	CalendarMonthFromPage(context.Context, PageReader, string, string) (vueling.CalendarMonth, error)
	FlightsFromPage(context.Context, PageReader, string, string, string) ([]vueling.Flight, error)
	FlightStatusFromPage(context.Context, PageReader, string, string, string, time.Time) ([]vueling.FlightStatus, error)
}
