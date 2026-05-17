package gobunnings

import (
	"context"
	"fmt"
	"net/http"
)

type InventoryService struct{ client *Client }

const inventoryAPIVersion = "1.0"

type StockLevel struct {
	ItemNumber             string            `json:"itemNumber,omitempty"`
	CountryCode            CountryCode       `json:"countryCode,omitempty"`
	LocationCode           string            `json:"locationCode,omitempty"`
	LevelIndicator         string            `json:"levelIndicator,omitempty"`
	ExpectedStockAvailable []string          `json:"expectedStockAvailable,omitempty"`
	Meta                   map[string]string `json:"_meta,omitempty"`
	Links                  []HateOASLink     `json:"_links,omitempty"`
}

func (s *InventoryService) Discovery(ctx context.Context) (*EntryPoint, error) {
	var out EntryPoint
	err := s.client.do(ctx, s.client.BaseURLs.Inventory, inventoryAPIVersion, http.MethodGet, "/discovery", nil, nil, &out, nil)
	return &out, err
}
func (s *InventoryService) Stock(ctx context.Context, country CountryCode, locationCode, itemNumber string, opt QueryOptions) (*StockLevel, error) {
	var out StockLevel
	err := s.client.do(ctx, s.client.BaseURLs.Inventory, inventoryAPIVersion, http.MethodGet, fmt.Sprintf("/itemStock/%s/%s/%s", country, cleanElem(locationCode), cleanElem(itemNumber)), opt.Values(), nil, &out, nil)
	return &out, err
}
func (s *InventoryService) Stocks(ctx context.Context, country CountryCode, locationCodes, itemNumbers []string, opt QueryOptions) ([]StockLevel, error) {
	q := opt.Values()
	q.Set("locationCodes", joinCSV(locationCodes))
	q.Set("itemNumbers", joinCSV(itemNumbers))
	var out []StockLevel
	err := s.client.do(ctx, s.client.BaseURLs.Inventory, inventoryAPIVersion, http.MethodGet, fmt.Sprintf("/itemStock/inventory/%s", country), q, nil, &out, nil)
	return out, err
}
