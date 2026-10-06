package goinvoiceninja

import (
	"context"
	"encoding/json"
	"errors"
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
	if c.Vendors == nil || c.Projects == nil || c.ExpenseCategories == nil || c.Expenses == nil || c.Statics == nil || c.Companies == nil {
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
		PaymentDate:          "2026-10-02",
		PaymentTypeID:        "5",
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
	if gotBody["category_id"] != "category1" {
		t.Fatalf("category_id = %#v in %#v", gotBody["category_id"], gotBody)
	}
	if gotBody["payment_type_id"] != "5" {
		t.Fatalf("payment_type_id = %#v in %#v", gotBody["payment_type_id"], gotBody)
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
	if got := values.Get("category_id"); got != "category1" {
		t.Fatalf("expense category query = %q", got)
	}
}

func TestExpenseDecodesExpenseCategoryID(t *testing.T) {
	var expense Expense
	if err := json.Unmarshal([]byte(`{"id":"expense1","category_id":"category1","payment_type_id":"5"}`), &expense); err != nil {
		t.Fatal(err)
	}
	if expense.CategoryID != "category1" {
		t.Fatalf("category ID = %q", expense.CategoryID)
	}
	if expense.PaymentTypeID != "5" {
		t.Fatalf("payment type ID = %q", expense.PaymentTypeID)
	}
}

func TestExpenseUpdatePaymentStatusCanClearFields(t *testing.T) {
	var gotMethod, gotPath string
	var got ExpensePaymentStatusRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"expense1","payment_date":"","payment_type_id":""}}`))
	}))
	defer ts.Close()

	c, err := New("token", WithBaseURL(ts.URL), WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatal(err)
	}
	updated, err := c.Expenses.UpdatePaymentStatus(context.Background(), "expense1", ExpensePaymentStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPut || gotPath != "/api/v1/expenses/expense1" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if got.PaymentDate != "" || got.PaymentTypeID != "" {
		t.Fatalf("unexpected payment status payload: %#v", got)
	}
	if updated.ID != "expense1" {
		t.Fatalf("unexpected updated expense: %#v", updated)
	}
}

func TestExpenseUpdateCanPersistExplicitZeroAccountingValues(t *testing.T) {
	var raw map[string]json.RawMessage
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/expenses/expense1" {
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"expense1","amount":0,"tax_amount1":0}}`))
	}))
	defer ts.Close()

	c, err := New("token", WithBaseURL(ts.URL), WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatal(err)
	}
	request := UpdateExpenseRequest{
		VendorID:   "vendor1",
		CategoryID: "category1",
		Amount:     0,
		Date:       "2026-10-02",
		TaxAmount1: 0,
	}.WithExplicitFields("amount", "tax_amount1", "payment_date", "payment_type_id")
	if _, err := c.Expenses.Update(context.Background(), "expense1", request); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"amount", "tax_amount1", "payment_date", "payment_type_id"} {
		value, ok := raw[field]
		if !ok {
			t.Fatalf("explicit field %q was omitted: %#v", field, raw)
		}
		if field == "amount" || field == "tax_amount1" {
			if string(value) != "0" {
				t.Fatalf("%s = %s; want 0", field, value)
			}
		} else if string(value) != `""` {
			t.Fatalf("%s = %s; want empty string", field, value)
		}
	}
}

func TestExpenseUpdateRequestRemainsSparse(t *testing.T) {
	encoded, err := json.Marshal(UpdateExpenseRequest{PrivateNotes: "corrected note"})
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 1 || string(raw["private_notes"]) != `"corrected note"` {
		t.Fatalf("sparse update JSON = %s", encoded)
	}
	for _, field := range []string{"amount", "tax_amount1", "payment_date", "payment_type_id", "uses_inclusive_taxes"} {
		if _, ok := raw[field]; ok {
			t.Fatalf("unrelated zero field %q was serialized: %s", field, encoded)
		}
	}
}

func TestExpenseUploadDocument(t *testing.T) {
	var gotMethod, gotPath, gotField, gotFilename, gotContent, gotIsPublic string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatal(err)
		}
		for {
			part, err := mr.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(part)
			if err != nil {
				t.Fatal(err)
			}
			if part.FormName() == "is_public" {
				gotIsPublic = string(body)
			} else {
				gotField = part.FormName()
				gotFilename = part.FileName()
				gotContent = string(body)
			}
			_ = part.Close()
		}
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
	if gotMethod != http.MethodPut || gotPath != "/api/v1/expenses/expense1/upload" || gotField != "documents[]" || gotFilename != "receipt.pdf" || gotContent != "receipt body" || gotIsPublic != "false" {
		t.Fatalf("unexpected upload: method=%q path=%q field=%q filename=%q content=%q is_public=%q", gotMethod, gotPath, gotField, gotFilename, gotContent, gotIsPublic)
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
