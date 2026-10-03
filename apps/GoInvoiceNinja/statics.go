package goinvoiceninja

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// PaymentType is an Invoice Ninja payment type returned by /statics. Name is
// localized by the target Invoice Ninja instance and ID is the API value used
// by expenses and payments.
type PaymentType struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	GatewayTypeID *int   `json:"gateway_type_id"`
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
