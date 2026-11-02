package api

import (
	"context"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

var htmlTag = regexp.MustCompile(`<[^>]*>`)

// Alert reads the website's disruption banner in the given culture, such as es-ES or en-GB.
func (c *Client) Alert(ctx context.Context, culture string) (vueling.Alert, error) {
	var banner struct {
		Pae  *bool  `json:"pae"`
		Text string `json:"text"`
		URL  string `json:"url"`
	}
	if err := c.getJSON(ctx, c.WWW+"/v1/api/AlertsApi/getPaeAlert?culture="+culture, wwwOrigin, &banner); err != nil {
		return vueling.Alert{}, fmt.Errorf("read Vueling's alert banner: %w", err)
	}
	if banner.Pae == nil {
		return vueling.Alert{}, errors.New("Vueling's alert response has no pae status")
	}
	text := strings.Join(strings.Fields(html.UnescapeString(htmlTag.ReplaceAllString(banner.Text, " "))), " ")
	if *banner.Pae && text == "" && banner.URL == "" {
		return vueling.Alert{}, errors.New("Vueling's active alert response has no text or URL")
	}
	return vueling.Alert{Active: *banner.Pae, Text: text, URL: banner.URL}, nil
}
