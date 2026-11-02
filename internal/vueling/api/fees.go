package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/seifreed/vuelingcli/internal/vueling"
)

const feesPath = "/es/servicios-vueling/tarifas-de-los-servicios-adicionales-al-vuelo"

var (
	feeTable     = regexp.MustCompile(`<vy-spa-table settings='([^']*)'`)
	blockTag     = regexp.MustCompile(`(?i)<br\s*/?>|</p>|</li>`)
	missingPrice = map[string]bool{"--": true, "-": true}
)

// Fees reads the price ranges of seats, bags, changes and other extras from Vueling's fees page.
func (c *Client) Fees(ctx context.Context) ([]vueling.Fee, error) {
	var page []byte
	if err := c.getJSON(ctx, c.WWW+feesPath, wwwOrigin, &page); err != nil {
		return nil, fmt.Errorf("read Vueling's fees page: %w", err)
	}
	match := feeTable.FindSubmatch(page)
	if match == nil {
		return nil, errors.New("Vueling's fees page has no fee table")
	}
	var table struct {
		DataSource struct {
			BodyData []map[string]struct {
				ContentText string `json:"contentText"`
			} `json:"bodyData"`
		} `json:"dataSource"`
	}
	if err := json.Unmarshal([]byte(html.UnescapeString(string(match[1]))), &table); err != nil {
		return nil, fmt.Errorf("decode Vueling's fee table: %w", err)
	}
	if len(table.DataSource.BodyData) == 0 {
		return nil, errors.New("Vueling's fee table has no fee rows")
	}
	fees := make([]vueling.Fee, 0, len(table.DataSource.BodyData))
	for _, row := range table.DataSource.BodyData {
		service := cellText(row["0"].ContentText)
		if service == "" {
			return nil, errors.New("Vueling's fee table has a row without a service")
		}
		fees = append(fees, vueling.Fee{
			Service: service,
			Online:  cellText(row["1"].ContentText),
			Phone:   cellText(row["2"].ContentText),
			Airport: cellText(row["3"].ContentText),
		})
	}
	return fees, nil
}

// cellText flattens a cell's HTML; inline tags split amounts like "5<span>,0</span>", so only block tags become spaces.
func cellText(content string) string {
	text := htmlTag.ReplaceAllString(blockTag.ReplaceAllString(content, " "), "")
	text = strings.Join(strings.Fields(strings.ReplaceAll(html.UnescapeString(text), " ", " ")), " ")
	if missingPrice[text] {
		return ""
	}
	return text
}
