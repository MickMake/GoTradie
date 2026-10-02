package goinvoiceninja

import (
	"encoding/json"
	"testing"
)

func TestPaymentablesAcceptObjectAndArrayShapes(t *testing.T) {
	tests := []struct {
		name string
		json string
		want int
	}{
		{name: "object", json: `{"paymentables":{"invoice_id":"inv-1","amount":"12.50"}}`, want: 1},
		{name: "array", json: `{"paymentables":[{"invoice_id":"inv-1","amount":12.5},{"invoice_id":"inv-2","amount":"7.50"}]}`, want: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var payment Payment
			if err := json.Unmarshal([]byte(test.json), &payment); err != nil {
				t.Fatal(err)
			}
			if len(payment.Paymentables) != test.want {
				t.Fatalf("got %d paymentables; want %d", len(payment.Paymentables), test.want)
			}
			if got := float64(payment.Paymentables[0].Amount); got != 12.5 {
				t.Fatalf("amount = %v; want 12.5", got)
			}
		})
	}
}

func TestClientMigrationFieldsDecode(t *testing.T) {
	raw := `{
		"id":"client-1",
		"shipping_address1":"2 Shipping Rd",
		"shipping_city":"Sydney",
		"shipping_country_id":"36",
		"group_settings_id":"group-1",
		"settings":{"currency_id":"4","payment_terms":14},
		"contacts":[{"id":"contact-1","is_primary":true,"archived_at":3}]
	}`
	var client ClientEntity
	if err := json.Unmarshal([]byte(raw), &client); err != nil {
		t.Fatal(err)
	}
	if client.ShippingAddress1 != "2 Shipping Rd" || client.Settings.CurrencyID != "4" || client.Settings.PaymentTerms != 14 {
		t.Fatalf("migration fields not decoded: %#v", client)
	}
	if len(client.Contacts) != 1 || !client.Contacts[0].IsPrimary || client.Contacts[0].ArchivedAt != 3 {
		t.Fatalf("contact migration fields not decoded: %#v", client.Contacts)
	}
}

func TestPaymentInvoicesAcceptAllocationAmountsAsStrings(t *testing.T) {
	raw := `{"id":"payment-1","amount":20,"invoices":[{"invoice_id":"invoice-1","amount":"12.50"},{"invoice_id":"invoice-2","amount":7.5}]}`
	var payment Payment
	if err := json.Unmarshal([]byte(raw), &payment); err != nil {
		t.Fatal(err)
	}
	if len(payment.InvoiceAllocations) != 2 {
		t.Fatalf("got %d allocations; want 2", len(payment.InvoiceAllocations))
	}
	if got := float64(payment.InvoiceAllocations[0].Amount); got != 12.5 {
		t.Fatalf("first amount = %v; want 12.5", got)
	}
}
