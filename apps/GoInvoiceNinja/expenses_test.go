package goinvoiceninja

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExpenseServicesAreRegistered(t *testing.T) {
	c, err := New("token")
	if err != nil {
		t.Fatal(err)
	}
	if c.Vendors == nil || c.Projects == nil || c.ExpenseCategories == nil || c.Expenses == nil {
		t.Fatal("expense-related services were not registered")
	}
	if c.Expenses.Endpoint() != "expenses" {
		t.Fatalf("unexpected expense endpoint %q", c.Expenses.Endpoint())
	}
}

func TestCreateExpenseRequest(t *testing.T) {
	var gotPath string
	var got CreateExpenseRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"expense1","amount":99.90,"tax_amount1":9.08}}`))
	}))
	defer ts.Close()

	c, err := New("token", WithBaseURL(ts.URL), WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatal(err)
	}
	created, err := c.Expenses.Create(context.Background(), CreateExpenseRequest{
		VendorID:             "vendor1",
		CategoryID:           "category1",
		Amount:               99.90,
		Date:                 "2026-10-02",
		TaxName1:             "GST",
		TaxRate1:             10,
		TaxAmount1:           9.08,
		UsesInclusiveTaxes:   true,
		CalculateTaxByAmount: true,
		CustomValue1:         "Expense - Consumables",
		CustomValue2:         "Consumable: Nails",
		CustomValue3:         "100%",
		CustomValue4:         "GST Credit",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v1/expenses" {
		t.Fatalf("unexpected path %q", gotPath)
	}
	if got.VendorID != "vendor1" || got.CategoryID != "category1" {
		t.Fatalf("unexpected relationship fields: %#v", got)
	}
	if got.TaxAmount1 != 9.08 || !got.CalculateTaxByAmount {
		t.Fatalf("unexpected tax fields: %#v", got)
	}
	if created.ID != "expense1" {
		t.Fatalf("unexpected created expense %#v", created)
	}
}

func TestVendorProjectAndCategoryQueries(t *testing.T) {
	if got := (VendorQuery{Name: "Bunnings"}).Values().Get("name"); got != "Bunnings" {
		t.Fatalf("vendor name query = %q", got)
	}
	if got := (ProjectQuery{Number: "1234"}).Values().Get("number"); got != "1234" {
		t.Fatalf("project number query = %q", got)
	}
	if got := (ExpenseCategoryQuery{Name: "Materials"}).Values().Get("name"); got != "Materials" {
		t.Fatalf("category name query = %q", got)
	}
}
