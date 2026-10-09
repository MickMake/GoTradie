package app

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestParseFinancialExportArgs(t *testing.T) {
	options, force, err := parseFinancialExportArgs([]string{
		"--fy", "2027", "--period=Sep", "--from", "", "--to", "", "--force",
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.FY != "2027" || options.Period != "Sep" || options.From != "" || options.To != "" || !force {
		t.Fatalf("options=%#v force=%v", options, force)
	}
	for _, args := range [][]string{{"output.xlsx"}, {"--all"}, {"--month", "Jul"}, {"--quarter", "1"}} {
		if _, _, err := parseFinancialExportArgs(args); err == nil {
			t.Fatalf("args %#v should fail", args)
		}
	}
}

func TestFinancialExportFiltersBySourceDateAndPreservesRelationships(t *testing.T) {
	var mu sync.Mutex
	requested := make(map[string]bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v1/companies/current" {
			if r.URL.Query().Get("with_trashed") != "true" {
				t.Errorf("%s missing with_trashed: %s", r.URL.Path, r.URL.RawQuery)
			}
			if r.URL.Query().Get("status") != "active,archived,deleted" {
				t.Errorf("%s status=%q", r.URL.Path, r.URL.Query().Get("status"))
			}
			mu.Lock()
			requested[r.URL.Path] = true
			mu.Unlock()
		}
		switch r.URL.Path {
		case "/api/v1/companies/current":
			_, _ = w.Write([]byte(`{"data":{"id":"company-1","settings":{"currency_id":"1"}}}`))
		case "/api/v1/invoices":
			_, _ = w.Write([]byte(`{"data":[{"id":"invoice-june","number":"INV-JUNE","status_id":"2","client_id":"customer-1","date":"2026-06-28","amount":25,"total_taxes":0,"tax_name1":"GST Free","line_items":[{"product_key":"BUNNINGS-123","notes":"Timber","quantity":2,"cost":12.5}],"client":{"id":"customer-1","name":"Arthur Dent","settings":{"currency_id":"1"}}}]}`))
		case "/api/v1/payments":
			_, _ = w.Write([]byte(`{"data":[{"id":"payment-july","number":"PAY-1","client_id":"customer-1","date":"2026-07-05","amount":25,"applied":25,"paymentables":[{"invoice_id":"invoice-june","amount":25}],"invoices":[{"id":"invoice-june","number":"INV-JUNE"}],"client":{"id":"customer-1","name":"Arthur Dent"}}]}`))
		case "/api/v1/expenses":
			_, _ = w.Write([]byte(`{"data":[{"id":"expense-july","number":"EXP-1","vendor_id":"vendor-1","category_id":"category-1","currency_id":"1","date":"2026-07-10","payment_date":"2026-07-11","amount":11,"tax_amount1":1,"tax_name1":"GST","custom_value3":"100","vendor":{"id":"vendor-1","name":"Mostly Harmless Supplies"},"category":{"id":"category-1","name":"Materials"},"project":{"id":"project-1","name":"Bypass"}}]}`))
		case "/api/v1/bank_transactions":
			_, _ = w.Write([]byte(`{"data":[{"id":"transaction-july","date":"2026-07-11","amount":-11,"base_type":"DEBIT","expense_id":"expense-july","vendor_id":"vendor-1"}]}`))
		case "/api/v1/clients":
			_, _ = w.Write([]byte(`{"data":[{"id":"customer-1","name":"Arthur Dent","contacts":[{"id":"contact-1","first_name":"Arthur","is_primary":true}]}]}`))
		case "/api/v1/vendors":
			_, _ = w.Write([]byte(`{"data":[{"id":"vendor-1","name":"Mostly Harmless Supplies"}]}`))
		case "/api/v1/products":
			_, _ = w.Write([]byte(`{"data":[{"id":"product-1","product_key":"BUNNINGS-123","price":12.5}]}`))
		case "/api/v1/projects":
			_, _ = w.Write([]byte(`{"data":[{"id":"project-1","client_id":"customer-1","name":"Bypass"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	home := t.TempDir()
	exportDirectory := filepath.Join(home, "exports")
	writeFinancialTestConfig(t, home, server.URL, exportDirectory)
	t.Setenv("HOME", home)
	now := func() time.Time { return time.Date(2026, time.October, 9, 12, 0, 0, 0, time.Local) }

	var stdout, stderr bytes.Buffer
	application := App{Out: &stdout, Err: &stderr, Now: now}
	args := []string{"ninja", "export", "financial", "--from", "2026-07-01", "--to", "2026-07-31"}
	if code := application.Run(context.Background(), args); code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	path := filepath.Join(exportDirectory, "Financial-2026-07-01-to-2026-07-31.xlsx")
	book, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = book.Close() }()
	if rows, err := book.GetRows("Income"); err != nil || len(rows) != 1 {
		t.Fatalf("income rows=%#v err=%v; June invoice should be excluded", rows, err)
	}
	if got, _ := book.GetCellValue("Payments", "S2"); got != "invoice-june" {
		t.Fatalf("payment invoice relationship=%q", got)
	}
	if got, _ := book.GetCellValue("Payments", "T2"); got != "INV-JUNE" {
		t.Fatalf("payment invoice number=%q", got)
	}
	if got, _ := book.GetCellValue("Summary", "B7"); got != "COMPLETE" {
		t.Fatalf("report status=%q", got)
	}
	if got, _ := book.GetCellValue("Summary", "B8"); got != "1" {
		t.Fatalf("company currency=%q", got)
	}
	if !strings.Contains(stdout.String(), "Report Status: COMPLETE") || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	for _, endpoint := range []string{"/api/v1/invoices", "/api/v1/payments", "/api/v1/expenses", "/api/v1/bank_transactions", "/api/v1/clients", "/api/v1/vendors", "/api/v1/products", "/api/v1/projects"} {
		mu.Lock()
		seen := requested[endpoint]
		mu.Unlock()
		if !seen {
			t.Errorf("endpoint %s was not requested", endpoint)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if code := application.Run(context.Background(), args); code != 1 || !strings.Contains(stderr.String(), "use --force") {
		t.Fatalf("overwrite exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := application.Run(context.Background(), append(args, "--force")); code != 0 {
		t.Fatalf("force exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestFinancialExportRejectsMixedSelectionBeforeRemoteReads(t *testing.T) {
	home := t.TempDir()
	writeFinancialTestConfig(t, home, "http://127.0.0.1:1", filepath.Join(home, "exports"))
	t.Setenv("HOME", home)
	var stdout, stderr bytes.Buffer
	application := App{Out: &stdout, Err: &stderr}
	code := application.Run(context.Background(), []string{
		"ninja", "export", "financial", "--fy", "2027", "--from", "2026-07-01",
	})
	if code != 2 || !strings.Contains(stderr.String(), "cannot be combined") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func writeFinancialTestConfig(t *testing.T, home, serverURL, exportDirectory string) {
	t.Helper()
	configDirectory := filepath.Join(home, ".GoTradie")
	if err := os.MkdirAll(configDirectory, 0755); err != nil {
		t.Fatal(err)
	}
	configBody := fmt.Sprintf(`invoice_ninja:
  url: %s
  token: token
bas:
  reporting_period: quarterly
  gst_basis: cash
  periods:
    Q1: {bas_begin: "07-01", bas_end: "09-30", submit_begin: "10-01", submit_end: "10-28"}
    Q2: {bas_begin: "10-01", bas_end: "12-31", submit_begin: "01-01", submit_end: "02-28"}
    Q3: {bas_begin: "01-01", bas_end: "03-31", submit_begin: "04-01", submit_end: "04-28"}
    Q4: {bas_begin: "04-01", bas_end: "06-30", submit_begin: "07-01", submit_end: "07-28"}
eofy:
  accounting_basis: cash
  instant_asset_writeoff_threshold: 20000
exports:
  directory: %s
`, serverURL, exportDirectory)
	if err := os.WriteFile(filepath.Join(configDirectory, "config.yaml"), []byte(configBody), 0600); err != nil {
		t.Fatal(err)
	}
}
