package goinvoiceninja

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCurrentCompanyReadsVendorPaidNotificationSetting(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"company1","settings":{"currency_id":"company-currency"},"notify_vendor_when_paid":false}}`))
	}))
	defer ts.Close()

	client, err := New("token", WithBaseURL(ts.URL), WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatal(err)
	}
	company, err := client.Companies.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/companies/current" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if company.NotifyVendorWhenPaid == nil || *company.NotifyVendorWhenPaid {
		t.Fatalf("notification setting = %#v", company.NotifyVendorWhenPaid)
	}
	if company.Settings.CurrencyID != "company-currency" {
		t.Fatalf("currency ID = %q", company.Settings.CurrencyID)
	}
}

func TestCurrentCompanyPreservesMissingVendorPaidNotificationSetting(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"company1","settings":{"currency_id":"company-currency"}}}`))
	}))
	defer ts.Close()

	client, err := New("token", WithBaseURL(ts.URL), WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatal(err)
	}
	company, err := client.Companies.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if company.NotifyVendorWhenPaid != nil {
		t.Fatalf("missing setting decoded as %#v", company.NotifyVendorWhenPaid)
	}
}
