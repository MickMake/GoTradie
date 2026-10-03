package goinvoiceninja

import (
	"context"
	"net/http"
)

// Company contains the company-level fields needed by reusable API clients.
// A pointer preserves the distinction between false and a response which did
// not expose the setting at all.
type Company struct {
	Entity
	Settings             CompanySettings `json:"settings"`
	NotifyVendorWhenPaid *bool           `json:"notify_vendor_when_paid"`
}

// CompanySettings contains the company defaults exposed by the current
// company response.
type CompanySettings struct {
	CurrencyID string `json:"currency_id"`
}

// CompanyService reads company-level configuration.
type CompanyService struct {
	client *Client
}

// Current returns the company selected by the API token. Invoice Ninja exposes
// this read operation as POST /companies/current.
func (s *CompanyService) Current(ctx context.Context) (*Company, error) {
	req, err := s.client.NewRequest(ctx, http.MethodPost, "companies/current", nil, nil)
	if err != nil {
		return nil, err
	}
	raw, err := rawDo(s.client, req)
	if err != nil {
		return nil, err
	}
	company, err := decodeEnvelope[Company](raw)
	if err != nil {
		return nil, err
	}
	return &company, nil
}
