package gobunnings

import (
	"context"
	"net/http"
)

type PricingService struct{ client *Client }

const pricingAPIVersion = "1.0"

type CatalogPriceRequest struct {
	Context PriceContext `json:"context"`
	Items   []PriceItem  `json:"items"`
}
type PriceContext struct {
	Country  CountryCode `json:"country"`
	Location string      `json:"location"`
}
type PriceItem struct {
	ItemNumber string `json:"itemNumber"`
}
type CatalogPrices struct {
	Prices []CatalogPriceResult `json:"prices,omitempty"`
}
type CatalogPriceResult struct {
	ItemNumber    string            `json:"itemNumber,omitempty"`
	UnitPrice     float64           `json:"unitPrice,omitempty"`
	LineUnitPrice float64           `json:"lineUnitPrice,omitempty"`
	PriceID       string            `json:"priceId,omitempty"`
	Meta          map[string]string `json:"_meta,omitempty"`
	Links         []HateOASLink     `json:"_links,omitempty"`
}

func (s *PricingService) Discovery(ctx context.Context) (*EntryPoint, error) {
	var out EntryPoint
	err := s.client.do(ctx, s.client.BaseURLs.Pricing, pricingAPIVersion, http.MethodGet, "/discovery", nil, nil, &out, nil)
	return &out, err
}
func (s *PricingService) CatalogPrices(ctx context.Context, req CatalogPriceRequest, opt QueryOptions) (*CatalogPrices, error) {
	var out CatalogPrices
	err := s.client.do(ctx, s.client.BaseURLs.Pricing, pricingAPIVersion, http.MethodPost, "/catalog/prices", opt.Values(), req, &out, nil)
	return &out, err
}
