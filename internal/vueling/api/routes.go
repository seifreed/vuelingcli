package api

import (
	"context"
	"fmt"
	"sort"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

type station struct {
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	CountryCode   string    `json:"countryCode"`
	CountryName   string    `json:"countryName"`
	IsMac         bool      `json:"isMac"`
	IsConnection  bool      `json:"isConnection"`
	IsResidents   bool      `json:"isResidents"`
	IsLargeFamily bool      `json:"isLargeFamily"`
	Routes        []station `json:"routes"`
}

// Routes returns every destination Vueling sells from origin, direct or with a connection, sorted by code.
func (c *Client) Routes(ctx context.Context, origin string) ([]vueling.Route, error) {
	var network struct {
		Stations struct {
			Value []station `json:"value"`
		} `json:"stations"`
	}
	if err := c.getJSON(ctx, c.Static+"/corporativeCMS/Stations/es-ES.json", wwwOrigin, &network); err != nil {
		return nil, fmt.Errorf("read Vueling's route network: %w", err)
	}
	for _, from := range network.Stations.Value {
		if from.Code != origin {
			continue
		}
		var routes []vueling.Route
		for _, to := range from.Routes {
			// City groups such as PAR or LON repeat their airports' routes.
			if to.IsMac {
				continue
			}
			routes = append(routes, vueling.Route{
				Code: to.Code, Name: to.Name, CountryCode: to.CountryCode, Country: to.CountryName,
				Direct: !to.IsConnection, Residents: to.IsResidents, LargeFamily: to.IsLargeFamily,
			})
		}
		sort.Slice(routes, func(i, j int) bool { return routes[i].Code < routes[j].Code })
		return routes, nil
	}
	return nil, fmt.Errorf("Vueling's route network has no flights from %s", origin)
}
