package ninja

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
)

func TestParseExpenseDate(t *testing.T) {
	got, err := parseExpenseDate("15/01/2011")
	if err != nil {
		t.Fatal(err)
	}
	if got != "2011-01-15" {
		t.Fatalf("date = %q", got)
	}
}

func TestDeriveBASTreatment(t *testing.T) {
	tests := []struct {
		name string
		tax  string
		pct  float64
		gst  float64
		want string
	}{
		{"gst credit", "Expense - Materials", 100, 9.08, "GST Credit"},
		{"private", "Personal / Non-Deductible", 100, 0, "Private/Non-deductible"},
		{"zero business", "Expense - Materials", 0, 0, "Private/Non-deductible"},
		{"unknown no gst", "Admin / Financial", 100, 0, "Review"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveBASTreatment(tt.tax, tt.pct, tt.gst); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestSourceMarkerIsStable(t *testing.T) {
	rec := []string{"15/01/2011", "Bunnings", "$99.90"}
	one := sourceMarker(rec)
	two := sourceMarker(rec)
	if one != two || !strings.HasPrefix(one, "[GoTradie source:") {
		t.Fatalf("unstable marker %q %q", one, two)
	}
	if got := sourceMarkerFromNotes("hello\n" + one + "\nworld"); got != one {
		t.Fatalf("marker extraction = %q", got)
	}
}

func TestProjectNumberAcceptsOnlyNumericMasterQuoteNumber(t *testing.T) {
	idx := map[string]int{"Job Number": 0}
	for _, test := range []struct {
		value string
		want  string
	}{
		{"1234", "1234"},
		{"", ""},
		{"Business", ""},
		{"Q1234", ""},
		{"1234.0", ""},
	} {
		if got := projectNumber([]string{test.value}, idx); got != test.want {
			t.Fatalf("projectNumber(%q) = %q; want %q", test.value, got, test.want)
		}
	}
}

func TestParseExpenseMoney(t *testing.T) {
	got, err := parseExpenseMoney("$1,234.56")
	if err != nil {
		t.Fatal(err)
	}
	if got != 1234.56 {
		t.Fatalf("money = %v", got)
	}
}

func TestShouldDeriveBusinessAmount(t *testing.T) {
	tests := []struct {
		name   string
		amount string
		pct    float64
		want   bool
	}{
		{name: "blank amount", amount: "", pct: 100, want: true},
		{name: "whitespace amount", amount: "  ", pct: 50, want: true},
		{name: "explicit zero", amount: "0", pct: 100, want: false},
		{name: "formatted explicit zero", amount: "$0.00", pct: 100, want: false},
		{name: "zero business percentage", amount: "", pct: 0, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldDeriveBusinessAmount(tt.amount, tt.pct); got != tt.want {
				t.Fatalf("shouldDeriveBusinessAmount(%q, %v) = %v; want %v", tt.amount, tt.pct, got, tt.want)
			}
		})
	}
}

func TestExpenseVendorName(t *testing.T) {
	tests := []struct {
		supplier string
		store    string
		want     string
	}{
		{"Bunnings", "Dural", "Bunnings - Dural"},
		{"DigitalOcean", "Online", "DigitalOcean - Online"},
		{"Acme", "", "Acme"},
	}
	for _, tt := range tests {
		if got := expenseVendorName(tt.supplier, tt.store); got != tt.want {
			t.Fatalf("expenseVendorName(%q, %q) = %q; want %q", tt.supplier, tt.store, got, tt.want)
		}
	}
}

func TestReceiptIndexUsesExactFilenameAndReportsAmbiguity(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one")
	two := filepath.Join(root, "two")
	if err := os.MkdirAll(one, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(two, 0o755); err != nil {
		t.Fatal(err)
	}
	unique := filepath.Join(one, "Receipt-001.pdf")
	if err := os.WriteFile(unique, []byte("receipt"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(one, "duplicate.pdf"), filepath.Join(two, "duplicate.pdf")} {
		if err := os.WriteFile(path, []byte("receipt"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	index, err := indexReceipts(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := index.match("Receipt-001.pdf")
	if err != nil || got != unique {
		t.Fatalf("exact match = %q, %v; want %q", got, err, unique)
	}
	if _, err := index.match("receipt-001.pdf"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("case-mismatched filename error = %v", err)
	}
	if _, err := index.match("duplicate.pdf"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("duplicate filename error = %v", err)
	}
}

func TestImportExpenseCreatesProjectFromNumericMasterQuote(t *testing.T) {
	var projectRequest invoiceninja.CreateProjectRequest
	var expenseRequest map[string]json.RawMessage
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/vendors":
			_, _ = w.Write([]byte(`{"data":[{"id":"vendor1","name":"Bunnings - Dural"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/expense_categories":
			_, _ = w.Write([]byte(`{"data":[{"id":"category1","name":"Materials"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/expenses":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/quotes":
			_, _ = w.Write([]byte(`{"data":[{"id":"quote1","number":"1234","client_id":"client1"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/projects":
			if err := json.NewDecoder(r.Body).Decode(&projectRequest); err != nil {
				t.Error(err)
			}
			_, _ = w.Write([]byte(`{"data":{"id":"project1","number":"1234","client_id":"client1"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/expenses":
			if err := json.NewDecoder(r.Body).Decode(&expenseRequest); err != nil {
				t.Error(err)
			}
			_, _ = w.Write([]byte(`{"data":{"id":"expense1"}}`))
		default:
			http.Error(w, r.Method+" "+r.URL.String(), http.StatusNotFound)
		}
	}))
	defer ts.Close()

	client, err := invoiceninja.New("token", invoiceninja.WithBaseURL(ts.URL), invoiceninja.WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{client: client}
	csv := strings.Join([]string{
		"Date,Supplier,Store,Tax Treatment,Category,Option,Total Inc GST,Business %,Business Amount,Business GST,Job Number,File Name",
		"2/10/2026,Bunnings,Dural,Expense - Materials,Materials,Consumables,110,100,110,10,1234,",
	}, "\n")
	results, err := service.ImportExpensesCSV(context.Background(), strings.NewReader(csv), false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "created" || results[0].ID != "expense1" {
		t.Fatalf("unexpected results: %#v", results)
	}
	if projectRequest.Number != "1234" || projectRequest.ClientID != "client1" {
		t.Fatalf("unexpected project request: %#v", projectRequest)
	}
	assertJSONText(t, expenseRequest, "project_id", "project1")
	assertJSONText(t, expenseRequest, "payment_date", "2026-10-02")
	assertJSONText(t, expenseRequest, "vendor_id", "vendor1")
	if _, exists := expenseRequest["should_be_invoiced"]; exists {
		t.Fatalf("should_be_invoiced must be omitted: %#v", expenseRequest)
	}
}

func TestExpensePreviewTracksPlannedDependencies(t *testing.T) {
	idx := headerIndex([]string{
		"Date", "Supplier", "Store", "Tax Treatment", "Category", "Option",
		"Total Inc GST", "Business %", "Business Amount", "Business GST",
		"Job Number", "Invoice Number",
	})
	state := &expenseImportState{
		vendors:         map[string]invoiceninja.Vendor{},
		categories:      map[string]invoiceninja.ExpenseCategory{},
		projects:        map[string]invoiceninja.Project{},
		clientByQuote:   map[string]string{"1234": "client1"},
		expenseByMarker: map[string]invoiceninja.Expense{},
		seenMarkers:     map[string]int{},
	}
	service := &Service{}
	row := func(invoice string) []string {
		return []string{"2/10/2026", "Bunnings", "Dural", "Expense - Materials", "Materials", "Consumables", "110", "100", "110", "10", "1234", invoice}
	}

	first := service.importExpenseRow(context.Background(), state, nil, idx, row("INV-1"), 2, true)
	second := service.importExpenseRow(context.Background(), state, nil, idx, row("INV-2"), 3, true)
	for _, want := range []string{"vendor:create:Bunnings - Dural", "category:create:Materials", "project:create:1234"} {
		if !containsChange(first.Changes, want) {
			t.Fatalf("first preview changes %v do not contain %q", first.Changes, want)
		}
		if containsChange(second.Changes, want) {
			t.Fatalf("second preview repeats %q in %v", want, second.Changes)
		}
	}
}

func containsChange(changes []string, want string) bool {
	for _, change := range changes {
		if change == want {
			return true
		}
	}
	return false
}

func assertJSONText(t *testing.T, values map[string]json.RawMessage, field, want string) {
	t.Helper()
	var got string
	if err := json.Unmarshal(values[field], &got); err != nil {
		t.Fatalf("decode %s: %v", field, err)
	}
	if got != want {
		t.Fatalf("%s = %q; want %q", field, got, want)
	}
}
