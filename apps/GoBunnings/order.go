package gobunnings

import (
	"context"
	"net/http"
)

type OrderService struct{ client *Client }

const orderAPIVersion = "1.1"

type OrderSearchParams struct {
	TransactionReference string `json:"transactionReference,omitempty"`
	OrderType            string `json:"orderType,omitempty"`
	EmailAddress         string `json:"emailAddress,omitempty"`
	Country              string `json:"country,omitempty"`
}
type OrderSearchResults struct {
	Results []OrderSearchResult `json:"results,omitempty"`
	Request OrderSearchParams   `json:"request,omitempty"`
	Meta    map[string]any      `json:"_meta,omitempty"`
	Links   []HateOASLink       `json:"_links,omitempty"`
}
type OrderSearchResult struct {
	OrderID             int64          `json:"orderId,omitempty"`
	OrderNumber         string         `json:"orderNumber,omitempty"`
	OrderType           string         `json:"orderType,omitempty"`
	CountryCode         CountryCode    `json:"countryCode,omitempty"`
	OrderStatus         string         `json:"orderStatus,omitempty"`
	CreationTime        string         `json:"creationTime,omitempty"`
	LocationCode        string         `json:"locationCode,omitempty"`
	LocationName        string         `json:"locationName,omitempty"`
	CustomerName        string         `json:"customerName,omitempty"`
	CustomerAccount     string         `json:"customerAccount,omitempty"`
	CustomerOrderNumber string         `json:"customerOrderNumber,omitempty"`
	Lines               int            `json:"lines,omitempty"`
	TotalValue          float64        `json:"totalValue,omitempty"`
	OutstandingAmount   float64        `json:"outstandingAmount,omitempty"`
	Invoices            []string       `json:"invoices,omitempty"`
	Meta                map[string]any `json:"_meta,omitempty"`
	Links               []HateOASLink  `json:"_links,omitempty"`
}

func (s *OrderService) Discovery(ctx context.Context) (*EntryPoint, error) {
	var out EntryPoint
	err := s.client.do(ctx, s.client.BaseURLs.Ordering, orderAPIVersion, http.MethodGet, "/discovery", nil, nil, &out, nil)
	return &out, err
}
func (s *OrderService) Search(ctx context.Context, req OrderSearchParams, opt QueryOptions) (*OrderSearchResults, error) {
	var out OrderSearchResults
	err := s.client.do(ctx, s.client.BaseURLs.Ordering, orderAPIVersion, http.MethodPost, "/search", opt.Values(), req, &out, nil)
	return &out, err
}
