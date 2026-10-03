package goinvoiceninja

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPaymentTypesReadStatics(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payment_types":[{"id":"5","name":"Visa Card","gateway_type_id":1},{"id":"13","name":"PayPal","gateway_type_id":3}]}`))
	}))
	defer ts.Close()

	c, err := New("token", WithBaseURL(ts.URL), WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatal(err)
	}
	types, err := c.Statics.PaymentTypes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodGet || gotPath != "/api/v1/statics" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if len(types) != 2 || types[0].ID != "5" || types[0].Name != "Visa Card" || types[0].GatewayTypeID == nil || *types[0].GatewayTypeID != 1 {
		t.Fatalf("unexpected payment types: %#v", types)
	}
}
