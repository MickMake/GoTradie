package goinvoiceninja

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// PaymentType is an Invoice Ninja payment type returned by /statics. Name is
// localized by the target Invoice Ninja instance and ID is the API value used
// by expenses and payments.
type PaymentType struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	GatewayTypeID *int   `json:"gateway_type_id"`
}

// Currency is an Invoice Ninja currency returned by /statics.
type Currency struct {
	ID   FlexibleInt `json:"id"`
	Code string      `json:"code"`
}

// StaticService reads Invoice Ninja's instance-localized static values.
type StaticService struct {
	client *Client
}

// PaymentTypes returns the exact payment-type IDs and labels exposed by the
// target Invoice Ninja instance.
func (s *StaticService) PaymentTypes(ctx context.Context) ([]PaymentType, error) {
	req, err := s.client.NewRequest(ctx, http.MethodGet, "statics", nil, nil)
	if err != nil {
		return nil, err
	}
	raw, err := rawDo(s.client, req)
	if err != nil {
		return nil, err
	}
	var response struct {
		PaymentTypes []PaymentType `json:"payment_types"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("decode statics: %w", err)
	}
	return response.PaymentTypes, nil
}

// CurrencyID resolves an ISO currency code to the Invoice Ninja currency ID
// exposed by /statics.
func (s *StaticService) CurrencyID(ctx context.Context, code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", fmt.Errorf("currency code is required")
	}

	req, err := s.client.NewRequest(ctx, http.MethodGet, "statics", nil, nil)
	if err != nil {
		return "", err
	}
	raw, err := rawDo(s.client, req)
	if err != nil {
		return "", err
	}

	var response struct {
		Currencies []Currency `json:"currencies"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", fmt.Errorf("decode statics: %w", err)
	}

	for _, currency := range response.Currencies {
		if strings.EqualFold(strings.TrimSpace(currency.Code), code) {
			return strconv.Itoa(int(currency.ID)), nil
		}
	}

	return "", fmt.Errorf("Invoice Ninja currency %q was not found in /statics", code)
}
