package vueling

// Deal is the cheapest Vueling fare from an origin to one destination within the searched months.
type Deal struct {
	Destination string  `json:"destination"`
	City        string  `json:"city,omitempty"`
	Date        string  `json:"date"`
	Price       float64 `json:"price"`
	Currency    string  `json:"currency"`
	Direct      bool    `json:"direct"`
	CalendarURL string  `json:"calendarUrl,omitempty"`
}

// DealFilter selects deal destinations by price, route type, and result count.
type DealFilter struct {
	Direct   bool
	MaxPrice float64
	Limit    int
}

// FilterDeals retains matching deals in their original order.
func FilterDeals(deals []Deal, filter DealFilter) []Deal {
	var kept []Deal
	for _, deal := range deals {
		if (filter.Direct && !deal.Direct) || (filter.MaxPrice > 0 && deal.Price > filter.MaxPrice) {
			continue
		}
		kept = append(kept, deal)
		if filter.Limit > 0 && len(kept) == filter.Limit {
			break
		}
	}
	return kept
}
