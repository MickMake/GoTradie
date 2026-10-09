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
	"testing"
	"time"
)

func TestParseBASExportArgs(t *testing.T) {
	options, force, err := parseBASExportArgs([]string{"--fy", "2027", "--period=Sep", "--force"})
	if err != nil {
		t.Fatal(err)
	}
	if options.FY != "2027" || options.Period != "Sep" || !force {
		t.Fatalf("options=%#v force=%v", options, force)
	}
	for _, args := range [][]string{{"output.xlsx"}, {"--from", "2026-01-01"}, {"--all"}, {"--month", "Jul"}, {"--quarter", "1"}} {
		if _, _, err := parseBASExportArgs(args); err == nil {
			t.Fatalf("args %#v should fail", args)
		}
	}
}

func TestWriteBASFileHonoursForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "FY2027-BAS-Q1.xlsx")
	if err := writeBASFile(path, []byte("first"), false); err != nil {
		t.Fatal(err)
	}
	if err := writeBASFile(path, []byte("second"), false); err == nil {
		t.Fatal("expected overwrite refusal")
	}
	if err := writeBASFile(path, []byte("second"), true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "second" {
		t.Fatalf("file = %q", got)
	}
}

func TestBASExportWritesCompleteWorkbookFromInvoiceNinja(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/companies/current":
			if r.Method != http.MethodPost {
				t.Fatalf("company method = %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"data":{"id":"company-1","settings":{"currency_id":"1"}}}`))
		case "/api/v1/invoices":
			if r.URL.Query().Get("with_trashed") != "true" {
				t.Fatalf("invoice query = %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"invoice-1","number":"INV-1","status_id":"2","client_id":"client-1","date":"2026-08-01","amount":110,"total_taxes":10,"tax_name1":"GST","client":{"id":"client-1","name":"Example","settings":{"currency_id":"1"}}}]}`))
		case "/api/v1/payments", "/api/v1/expenses", "/api/v1/bank_transactions":
			if r.URL.Query().Get("with_trashed") != "true" {
				t.Fatalf("%s query = %s", r.URL.Path, r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	home := t.TempDir()
	exportDirectory := filepath.Join(home, "exports")
	configDirectory := filepath.Join(home, ".GoTradie")
	if err := os.MkdirAll(configDirectory, 0755); err != nil {
		t.Fatal(err)
	}
	configBody := fmt.Sprintf(`invoice_ninja:
  url: %s
  token: token
bas:
  reporting_period: quarterly
  gst_basis: accrual
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
`, server.URL, exportDirectory)
	if err := os.WriteFile(filepath.Join(configDirectory, "config.yaml"), []byte(configBody), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	var stdout, stderr bytes.Buffer
	application := App{Out: &stdout, Err: &stderr, Now: func() time.Time {
		return time.Date(2026, time.October, 9, 12, 0, 0, 0, time.Local)
	}}
	if code := application.Run(context.Background(), []string{"ninja", "export", "bas"}); code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	path := filepath.Join(exportDirectory, "FY2027-BAS-Q1.xlsx")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("workbook: %v", err)
	}
	if !strings.Contains(stdout.String(), "Report Status: COMPLETE") || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
