package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

type calendarReaderFunc func(context.Context, string, string, string) ([]vueling.CalendarDay, error)

func (read calendarReaderFunc) Calendar(ctx context.Context, origin, destination, currency string) ([]vueling.CalendarDay, error) {
	return read(ctx, origin, destination, currency)
}

func TestFindTripsReadsBothDirectionsAndCalculatesTrips(t *testing.T) {
	var calls []string
	read := calendarReaderFunc(func(_ context.Context, origin, destination, currency string) ([]vueling.CalendarDay, error) {
		calls = append(calls, fmt.Sprintf("%s-%s:%s", origin, destination, currency))
		if origin == "BCN" {
			return []vueling.CalendarDay{{Date: "01/12/2026", Fare: "10"}}, nil
		}
		return []vueling.CalendarDay{{Date: "03/12/2026", Fare: "12"}}, nil
	})
	search := vueling.TripSearch{
		Origin: "BCN", Destination: "BIO", From: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		To: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), MinNights: 2, MaxNights: 2, Currency: "EUR",
	}

	trips, err := FindTrips(context.Background(), read, search)
	if err != nil || len(trips) != 1 || trips[0].Total != 22 {
		t.Fatalf("FindTrips = %+v, %v; want one trip totaling 22", trips, err)
	}
	if fmt.Sprint(calls) != "[BCN-BIO:EUR BIO-BCN:EUR]" {
		t.Fatalf("calendar calls = %v", calls)
	}
}

func TestFindTripsPropagatesCalendarErrors(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(fmt.Sprint(failAt), func(t *testing.T) {
			wantErr := errors.New("calendar failed")
			calls := 0
			read := calendarReaderFunc(func(context.Context, string, string, string) ([]vueling.CalendarDay, error) {
				calls++
				if calls == failAt {
					return nil, wantErr
				}
				return nil, nil
			})
			trips, err := FindTrips(context.Background(), read, vueling.TripSearch{})
			if !errors.Is(err, wantErr) || trips != nil || calls != failAt {
				t.Fatalf("FindTrips = %v, %v after %d calls; want the calendar error", trips, err, calls)
			}
		})
	}
}
