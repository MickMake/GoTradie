package goinvoiceninja

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &gotBody); err != nil {
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
	if gotBody["expense_category_id"] != "category1" {
		t.Fatalf("expense_category_id = %#v in %#v", gotBody["expense_category_id"], gotBody)
	}
	if _, ok := gotBody["category_id"]; ok {
		t.Fatalf("unexpected category_id in %#v", gotBody)
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
	values := (ExpenseQuery{CategoryID: "category1"}).Values()
	if got := values.Get("expense_category_id"); got != "category1" {
		t.Fatalf("expense category query = %q", got)
	}
	if got := values.Get("category_id"); got != "" {
		t.Fatalf("unexpected category_id query = %q", got)
	}
}

func TestExpenseDecodesExpenseCategoryID(t *testing.T) {
	var expense Expense
	if err := json.Unmarshal([]byte(`{"id":"expense1","expense_category_id":"category1"}`), &expense); err != nil {
		t.Fatal(err)
	}
	if expense.CategoryID != "category1" {
		t.Fatalf("category ID = %q", expense.CategoryID)
	}
}

func TestExpenseUploadDocument(t *testing.T) {
	var gotPath, gotFilename, gotContent string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		file, header, err := r.FormFile(DefaultDocumentFormField)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		body, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		gotFilename = header.Filename
		gotContent = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"expense1","documents":[{"name":"receipt.pdf"}]}}`))
	}))
	defer ts.Close()

	c, err := New("token", WithBaseURL(ts.URL), WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatal(err)
	}
	updated, err := c.Expenses.UploadDocument(context.Background(), "expense1", "nested/receipt.pdf", strings.NewReader("receipt body"))
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v1/expenses/expense1/upload" || gotFilename != "receipt.pdf" || gotContent != "receipt body" {
		t.Fatalf("unexpected upload: path=%q filename=%q content=%q", gotPath, gotFilename, gotContent)
	}
	if len(updated.Documents) != 1 || updated.Documents[0].Name != "receipt.pdf" {
		t.Fatalf("unexpected updated expense: %#v", updated)
	}
}

func TestCreateExpenseRequestOmitsShouldBeInvoicedByDefault(t *testing.T) {
	raw, err := json.Marshal(CreateExpenseRequest{Amount: 12.34})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "should_be_invoiced") {
		t.Fatalf("unexpected should_be_invoiced field in %s", raw)
	}
}
