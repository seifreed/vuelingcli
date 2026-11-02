package vueling

// Fee is one row of Vueling's published price ranges for services added to a flight.
type Fee struct {
	Service string `json:"service"`
	Online  string `json:"online"`
	Phone   string `json:"customerService"`
	Airport string `json:"airport"`
}
