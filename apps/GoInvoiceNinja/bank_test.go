package goinvoiceninja

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBankServicesAreRegistered(t *testing.T) {
	c, err := New("token")
	if err != nil {
		t.Fatal(err)
	}
	if c.BankIntegrations == nil || c.BankTransactions == nil || c.BankTransactionRules == nil {
		t.Fatal("bank services were not registered")
	}
}

func TestCreateBankTransactionRequest(t *testing.T) {
	var gotMethod, gotPath string
	var got CreateBankTransactionRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"transaction1","bank_integration_id":"bank1","amount":394,"base_type":"DEBIT"}}`))
	}))
	defer ts.Close()

	c, err := New("token", WithBaseURL(ts.URL), WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatal(err)
	}
	created, err := c.BankTransactions.Create(context.Background(), CreateBankTransactionRequest{
		BankIntegrationID: "bank1",
		Amount:            394,
		BaseType:          "DEBIT",
		Date:              "2026-11-01",
		Description:       "historical supplier settlement",
		ParticipantName:   "BlueCarve",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/bank_transactions" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if got.BankIntegrationID != "bank1" || got.Amount != 394 || got.BaseType != "DEBIT" || got.Date != "2026-11-01" || got.ParticipantName != "BlueCarve" {
		t.Fatalf("unexpected request body: %#v", got)
	}
	if created.ID != "transaction1" {
		t.Fatalf("unexpected transaction: %#v", created)
	}
}

func TestBankQueriesPreserveSafetyFilters(t *testing.T) {
	if got := (BankIntegrationQuery{ListOptions: ListOptions{Status: "active"}}).Values().Get("status"); got != "active" {
		t.Fatalf("bank integration status = %q", got)
	}
	if got := (BankTransactionQuery{BankIntegrationID: "bank1"}).Values().Get("bank_integration_ids"); got != "bank1" {
		t.Fatalf("bank integration filter = %q", got)
	}
	if got := (BankTransactionRuleQuery{ListOptions: ListOptions{Status: "active"}}).Values().Get("status"); got != "active" {
		t.Fatalf("bank rule status = %q", got)
	}
}
