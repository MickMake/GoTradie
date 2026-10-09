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

	"github.com/xuri/excelize/v2"
)

func TestParseEOFYExportArgs(t *testing.T) {
	options, force, err := parseEOFYExportArgs([]string{"--fy", "2027", "--force"})
	if err != nil {
		t.Fatal(err)
	}
	if options.FY != "2027" || !force {
		t.Fatalf("options=%#v force=%v", options, force)
	}
	for _, args := range [][]string{
		{"output.xlsx"}, {"--period", "Q1"}, {"--from", "2026-01-01"}, {"--to", "2026-06-30"},
		{"--all"}, {"--month", "Jul"}, {"--quarter", "1"},
	} {
		if _, _, err := parseEOFYExportArgs(args); err == nil {
			t.Fatalf("args %#v should fail", args)
		}
	}
}

func TestEOFYExportUsesCompletedFYAndHonoursOverwriteSafety(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/companies/current":
			_, _ = w.Write([]byte(`{"data":{"id":"company-1","settings":{"currency_id":"1"}}}`))
		case "/api/v1/invoices":
			_, _ = w.Write([]byte(`{"data":[{"id":"invoice-1","number":"INV-1","status_id":"2","client_id":"client-1","date":"2025-08-01","amount":110,"total_taxes":10,"tax_name1":"GST","client":{"id":"client-1","name":"Example","settings":{"currency_id":"1"}}}]}`))
		case "/api/v1/expenses":
			_, _ = w.Write([]byte(`{"data":[{"id":"expense-1","number":"EXP-1","vendor_id":"vendor-1","category_id":"category-1","currency_id":"1","date":"2025-08-02","amount":55,"tax_amount1":5,"tax_name1":"GST","custom_value3":"100","private_notes":"Capital check: No\nItem description: Supplies","vendor":{"id":"vendor-1","name":"Supplier"},"category":{"id":"category-1","name":"Materials"}}]}`))
		case "/api/v1/payments", "/api/v1/bank_transactions":
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
  reporting_period: yearly
  gst_basis: cash
  periods:
    FY: {bas_begin: "07-01", bas_end: "06-30", submit_begin: "07-01", submit_end: "10-31"}
eofy:
  accounting_basis: accrual
  instant_asset_writeoff_threshold: 20000
exports:
  directory: %s
`, server.URL, exportDirectory)
	if err := os.WriteFile(filepath.Join(configDirectory, "config.yaml"), []byte(configBody), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	now := func() time.Time { return time.Date(2026, time.October, 9, 12, 0, 0, 0, time.Local) }

	var stdout, stderr bytes.Buffer
	application := App{Out: &stdout, Err: &stderr, Now: now}
	if code := application.Run(context.Background(), []string{"ninja", "export", "eofy"}); code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	path := filepath.Join(exportDirectory, "FY2026-EOFY.xlsx")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("workbook: %v", err)
	}
	book, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := book.GetCellValue("Summary", "B10"); got != "110.00" {
		_ = book.Close()
		t.Fatalf("income gross = %q; EOFY accrual basis should be independent of BAS cash basis", got)
	}
	if err := book.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Report Status: COMPLETE") || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := application.Run(context.Background(), []string{"ninja", "export", "eofy"}); code != 1 || !strings.Contains(stderr.String(), "use --force") {
		t.Fatalf("overwrite exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := application.Run(context.Background(), []string{"ninja", "export", "eofy", "--force"}); code != 0 {
		t.Fatalf("force exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
