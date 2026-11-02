package vueling

import (
	"math"
	"net/url"
	"testing"
	"time"
)

func TestQuoteTripTotalRoundsCombinedFareToCents(t *testing.T) {
	if got, want := QuoteTripTotal([]Quote{{Total: 0.1}, {Total: 0.2}}), 0.3; math.Abs(got-want) > 1e-9 {
		t.Fatalf("QuoteTripTotal = %v, want %v", got, want)
	}
}

func TestQuoteSelectionURLUsesFirstFlightOfEachConnection(t *testing.T) {
	date := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	link, err := QuoteSelectionURL(LinkOptions{
		Origin: "BCN", Destination: "BIO", Date: date, ReturnDate: date.AddDate(0, 0, 4), Adults: 1,
	}, []Quote{{Flight: "VY1460+VY1461"}, {Flight: "VY2000+VY2001"}})
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if got := u.Query().Get("ofn"); got != "VY1460" {
		t.Errorf("outbound flight = %q, want VY1460", got)
	}
	if got := u.Query().Get("rfn"); got != "VY2000" {
		t.Errorf("return flight = %q, want VY2000", got)
	}
}

func TestQuoteSelectionURLRejectsUnexpectedQuoteCount(t *testing.T) {
	if _, err := QuoteSelectionURL(LinkOptions{}, nil); err == nil {
		t.Fatal("QuoteSelectionURL accepted no quotes")
	}
	if _, err := QuoteSelectionURL(LinkOptions{}, []Quote{{}, {}, {}}); err == nil {
		t.Fatal("QuoteSelectionURL accepted more than two quotes")
	}
}
