package vueling

import (
	"reflect"
	"testing"
)

func TestFilterDealsAppliesDirectPriceAndLimitInOrder(t *testing.T) {
	deals := []Deal{
		{Destination: "BIO", Price: 20, Direct: true},
		{Destination: "AMS", Price: 30, Direct: false},
		{Destination: "MAD", Price: 25, Direct: true},
	}
	got := FilterDeals(deals, DealFilter{Direct: true, MaxPrice: 25, Limit: 1})
	want := []Deal{deals[0]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterDeals = %+v, want %+v", got, want)
	}
}

func TestFilterDealsWithoutFiltersKeepsEveryDeal(t *testing.T) {
	deals := []Deal{{Destination: "BIO"}, {Destination: "AMS", Direct: true}}
	got := FilterDeals(deals, DealFilter{})
	if !reflect.DeepEqual(got, deals) {
		t.Fatalf("FilterDeals without filters = %+v, want %+v", got, deals)
	}
	priced := []Deal{{Destination: "BIO", Price: 20}, {Destination: "AMS", Price: 30}}
	if got := FilterDeals(priced, DealFilter{MaxPrice: 25}); !reflect.DeepEqual(got, priced[:1]) {
		t.Fatalf("FilterDeals with max price = %+v, want %+v", got, priced[:1])
	}
}
