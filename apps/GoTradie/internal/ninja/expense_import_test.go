package ninja

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
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

func TestPurchaseSourceMarkerIgnoresControlAndAnalyticalCorrections(t *testing.T) {
	header := []string{
		"Date", "Supplier", "Store", "Invoice Number", "Item Number", "Item Description",
		"Qty", "Unit", "Unit Price", "Total Ex GST", "GST", "Total Inc GST", "$ Currency",
		"Document Type", "Payment Type", "Category", "Job Number", "Business %", "Business Amount", "Business GST",
	}
	idx := headerIndex(header)
	one := []string{"2/10/2026", "Bunnings", "Dural", "INV-1", "123", "Nails", "2", "EA", "$5.00", "$9.09", "$0.91", "$10.00", "AUD", "TAX INVOICE", "", "Old", "100", "50", "5", "0.45"}
	two := []string{"02/10/2026", "Bunnings", "Dural", "INV-1", "123", "Nails", "2.0", "EA", "5", "9.09", ".91", "10", "AUD", "Invoice", "Visa Card", "Materials", "200", "100", "10", "0.91"}
	markerOne := purchaseSourceMarker(one, idx, "2026-10-02")
	markerTwo := purchaseSourceMarker(two, idx, "2026-10-02")
	if markerOne != markerTwo || !strings.HasPrefix(markerOne, "[GoTradie source:v2:") {
		t.Fatalf("stable markers differ: %q %q", markerOne, markerTwo)
	}
	two[idx["Invoice Number"]] = "INV-2"
	if changed := purchaseSourceMarker(two, idx, "2026-10-02"); changed == markerOne {
		t.Fatalf("distinct purchase retained marker %q", changed)
	}
}

func TestPurchaseSourceIdentityKeepsSplitRowsDistinct(t *testing.T) {
	header := []string{
		"Date", "Supplier", "Store", "Document Type", "Payment Type", "Tax Treatment", "Category", "Option",
		"Total Inc GST", "Business %", "Business Amount", "Business GST", "Invoice Number", "Item Number",
		"Item Description", "Qty", "Unit", "Unit Price", "Total Ex GST", "GST", "$ Currency", "Job Number",
	}
	idx := headerIndex(header)
	state := &expenseImportState{paymentTypes: map[string]invoiceninja.PaymentType{
		"Visa Card": {ID: "5", Name: "Visa Card"},
	}}
	row := func(category, option, job string) []string {
		return []string{"2/10/2026", "Bunnings", "Dural", "Invoice", "", "Expense", category, option, "10", "100", "10", "0.91", "INV-1", "123", "Nails", "2", "EA", "5", "9.09", "0.91", "AUD", job}
	}

	first := prepareExpenseImportRow(state, nil, idx, row("Materials", "Consumables", "100"), 2)
	second := prepareExpenseImportRow(state, nil, idx, row("Hardware", "Fixings", "200"), 3)
	assignExpenseRowIdentities([]*preparedExpenseImportRow{first, second})
	if first.err != nil || second.err != nil || first.sourceMarker == second.sourceMarker {
		t.Fatalf("split identities collapsed: first=%#v second=%#v", first, second)
	}

	correctedFirst := prepareExpenseImportRow(state, nil, idx, row("Corrected A", "Corrected option A", "300"), 2)
	correctedSecond := prepareExpenseImportRow(state, nil, idx, row("Corrected B", "Corrected option B", "400"), 3)
	assignExpenseRowIdentities([]*preparedExpenseImportRow{correctedFirst, correctedSecond})
	if correctedFirst.sourceMarker != first.sourceMarker || correctedSecond.sourceMarker != second.sourceMarker {
		t.Fatalf("analytical corrections changed identities: before=%q,%q after=%q,%q", first.sourceMarker, second.sourceMarker, correctedFirst.sourceMarker, correctedSecond.sourceMarker)
	}

	original := row("Materials", "Consumables", "100")
	controlChanged := row("Materials", "Consumables", "100")
	controlChanged[idx["Document Type"]] = "Receipt"
	controlChanged[idx["Payment Type"]] = "Visa Card"
	originalRow := prepareExpenseImportRow(state, nil, idx, original, 2)
	controlChangedRow := prepareExpenseImportRow(state, nil, idx, controlChanged, 3)
	assignExpenseRowIdentities([]*preparedExpenseImportRow{originalRow, controlChangedRow})
	if controlChangedRow.duplicateRow != originalRow.rowNo {
		t.Fatalf("control-only correction was not treated as the same source row: %#v", controlChangedRow)
	}
}

func TestAccountPaymentIdentityDisambiguation(t *testing.T) {
	header := []string{
		"Date", "Supplier", "Document Type", "Payment Type", "Tax Treatment", "Category", "Option",
		"Total Inc GST", "Business %", "Business Amount", "Business GST", "Invoice Number", "File Name", "Notes",
	}
	idx := headerIndex(header)
	state := &expenseImportState{paymentTypes: map[string]invoiceninja.PaymentType{
		"Visa Card": {ID: "5", Name: "Visa Card"},
		"PayPal":    {ID: "13", Name: "PayPal"},
	}}
	payment := func(method, reference, file, notes string) []string {
		return []string{"1/11/2026", "Bunnings", "Account Payment", method, "", "", "", "71.84", "", "", "", reference, file, notes}
	}

	t.Run("same facts use stable source order", func(t *testing.T) {
		first := prepareExpenseImportRow(state, nil, idx, payment("Visa Card", "PAY-1", "one.pdf", "first"), 2)
		second := prepareExpenseImportRow(state, nil, idx, payment("Visa Card", "PAY-1", "two.pdf", "second"), 3)
		assignExpenseRowIdentities([]*preparedExpenseImportRow{first, second})
		if first.err != nil || second.err != nil || first.sourceMarker == second.sourceMarker {
			t.Fatalf("legitimate payments collided: first=%#v second=%#v", first, second)
		}
	})

	t.Run("blank references remain distinct", func(t *testing.T) {
		first := prepareExpenseImportRow(state, nil, idx, payment("Visa Card", "", "one.pdf", "first"), 2)
		second := prepareExpenseImportRow(state, nil, idx, payment("Visa Card", "", "two.pdf", "second"), 3)
		assignExpenseRowIdentities([]*preparedExpenseImportRow{first, second})
		if first.err != nil || second.err != nil || first.sourceMarker == second.sourceMarker {
			t.Fatalf("blank-reference payments collided: first=%#v second=%#v", first, second)
		}
	})

	t.Run("payment method participates in identity", func(t *testing.T) {
		visa := prepareExpenseImportRow(state, nil, idx, payment("Visa Card", "PAY-1", "", ""), 2)
		paypal := prepareExpenseImportRow(state, nil, idx, payment("PayPal", "PAY-1", "", ""), 3)
		assignExpenseRowIdentities([]*preparedExpenseImportRow{visa, paypal})
		if visa.err != nil || paypal.err != nil || visa.sourceMarker == paypal.sourceMarker {
			t.Fatalf("different methods collided: visa=%#v paypal=%#v", visa, paypal)
		}
	})

	t.Run("identical input is ambiguous", func(t *testing.T) {
		rec := payment("Visa Card", "", "", "")
		first := prepareExpenseImportRow(state, nil, idx, append([]string(nil), rec...), 2)
		second := prepareExpenseImportRow(state, nil, idx, append([]string(nil), rec...), 3)
		assignExpenseRowIdentities([]*preparedExpenseImportRow{first, second})
		if first.err == nil || second.err == nil || !strings.Contains(first.err.Error(), "ambiguous") {
			t.Fatalf("identical payments were not rejected: first=%v second=%v", first.err, second.err)
		}
	})
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

func TestImportIDMarkerRoundTripsAndIgnoresRowPosition(t *testing.T) {
	marker := importIDMarker("EXP-2020-00471")
	if got, ok := importIDFromMarker(marker); !ok || got != "EXP-2020-00471" {
		t.Fatalf("marker round trip = %q, %v", got, ok)
	}
	if got := sourceMarkerFromNotes("historical\n" + marker); got != marker {
		t.Fatalf("marker extraction = %q", got)
	}

	idx := headerIndex(strings.Split(v052ExpenseCSVHeader(), ","))
	state := &expenseImportState{paymentTypes: map[string]invoiceninja.PaymentType{}}
	first := prepareExpenseImportRow(state, nil, idx, strings.Split("EXP-1,2/10/2026,Bunnings,Invoice,,Expense,Materials,Consumables,110,100,110,10", ","), 2)
	corrected := prepareExpenseImportRow(state, nil, idx, strings.Split("EXP-1,3/10/2026,Other Supplier,Invoice,,Expense,Materials,Consumables,220,50,110,10", ","), 947)
	assignExpenseRowIdentities([]*preparedExpenseImportRow{first})
	assignExpenseRowIdentities([]*preparedExpenseImportRow{corrected})
	if first.sourceMarker != corrected.sourceMarker || first.sourceMarker != importIDMarker("EXP-1") {
		t.Fatalf("Import ID identity changed with row data: %q %q", first.sourceMarker, corrected.sourceMarker)
	}
}

func TestImportIDExpenseMatchTakesPrecedenceOverLegacyFallback(t *testing.T) {
	row := &preparedExpenseImportRow{
		importID:     "EXP-1",
		sourceMarker: importIDMarker("EXP-1"),
		legacyMarker: "[GoTradie source:legacy]",
	}
	state := &expenseImportState{expenseByMarker: map[string]invoiceninja.Expense{
		row.sourceMarker: {Entity: invoiceninja.Entity{ID: "import-id-expense"}},
		row.legacyMarker: {Entity: invoiceninja.Entity{ID: "legacy-expense"}},
	}}

	got, exists, ambiguous := existingExpenseForRow(state, row)
	if !exists || ambiguous || got.ID != "import-id-expense" {
		t.Fatalf("existing expense = %#v, exists=%v ambiguous=%v", got, exists, ambiguous)
	}
}

func TestExpensePreflightRejectsIdentityErrorsBeforeRemoteWork(t *testing.T) {
	csv := strings.Join([]string{
		v052ExpenseCSVHeader(),
		",2/10/2026,Bunnings,Invoice,,Expense,Materials,Consumables,110,100,110,10",
		"EXP-1,3/10/2026,Bunnings,Invoice,,Expense,Materials,Consumables,110,100,110,10",
		"EXP-1,4/10/2026,Bunnings,Invoice,,Expense,Materials,Consumables,110,100,110,10",
	}, "\n")
	var report ExpenseImportPreflight
	_, err := (&Service{}).ImportExpensesCSVWithOptions(context.Background(), strings.NewReader(csv), ExpenseImportOptions{
		DryRun:      true,
		OnPreflight: func(got ExpenseImportPreflight) { report = got },
	})
	var preflightErr *ExpenseImportPreflightError
	if !errors.As(err, &preflightErr) {
		t.Fatalf("preflight error = %T %v", err, err)
	}
	if report.Rows != 3 || len(report.Errors) != 2 {
		t.Fatalf("preflight report = %#v", report)
	}
	if !strings.Contains(report.Errors[0].String(), "Import ID is required") || !strings.Contains(report.Errors[1].String(), "duplicate Import ID") {
		t.Fatalf("preflight errors = %#v", report.Errors)
	}
}

func TestExpensePreflightReportsArithmeticWarnings(t *testing.T) {
	header := v052ExpenseCSVHeader() + ",Total Ex GST,GST,Qty,Unit Price"
	csv := strings.Join([]string{
		header,
		"EXP-1,2/10/2026,Bunnings,Invoice,,Expense,Materials,Consumables,110,50,90,8,90,9,2,60",
	}, "\n")
	preflight := preflightExpenseSource(readCSVForTest(t, csv), "", 0)
	if len(preflight.report.Errors) != 0 {
		t.Fatalf("preflight errors = %#v", preflight.report.Errors)
	}
	if len(preflight.report.Warnings) < 4 {
		t.Fatalf("expected arithmetic warnings, got %#v", preflight.report.Warnings)
	}
}

func TestExpenseImportEmitsProgressAndTrueBatchSummaries(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if serveExpenseImportReferenceData(w, r) {
			return
		}
		http.Error(w, r.Method+" "+r.URL.String(), http.StatusNotFound)
	}))
	defer ts.Close()
	csv := strings.Join([]string{
		v052ExpenseCSVHeader(),
		"EXP-1,2/10/2026,Bunnings,Invoice,Visa Card,Expense,Materials,Consumables,110,100,110,10",
		"EXP-2,3/10/2026,Bunnings,Invoice,Visa Card,Expense,Materials,Consumables,20,100,20,1.82",
		"EXP-3,4/10/2026,Bunnings,Invoice,Visa Card,Expense,Materials,Consumables,30,100,30,2.73",
	}, "\n")
	var report ExpenseImportPreflight
	var progress []ExpenseImportProgress
	var batches []ExpenseImportBatchSummary
	results, err := newExpenseImportTestService(t, ts).ImportExpensesCSVWithOptions(context.Background(), strings.NewReader(csv), ExpenseImportOptions{
		DryRun:      true,
		BatchSize:   2,
		OnPreflight: func(got ExpenseImportPreflight) { report = got },
		OnProgress:  func(got ExpenseImportProgress) { progress = append(progress, got) },
		OnBatchComplete: func(got ExpenseImportBatchSummary) error {
			batches = append(batches, got)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 || len(progress) != 3 || len(batches) != 2 || report.Mode != ExpenseImportModeBatched {
		t.Fatalf("results=%d progress=%d batches=%#v report=%#v", len(results), len(progress), batches, report)
	}
	if batches[0].Processed != 2 || batches[0].Last || batches[1].Processed != 1 || !batches[1].Last {
		t.Fatalf("batch summaries = %#v", batches)
	}
}

func v052ExpenseCSVHeader() string {
	return "Import ID,Date,Supplier,Document Type,Payment Type,Tax Treatment,Category,Option,Total Inc GST,Business %,Business Amount,Business GST"
}

func readCSVForTest(t *testing.T, value string) [][]string {
	t.Helper()
	records, err := csv.NewReader(strings.NewReader(value)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return records
}

func TestExpenseVendorName(t *testing.T) {
	tests := []struct {
		supplier string
		store    string
		want     string
	}{
		{"Bunnings", "Dural", "Bunnings"},
		{"DigitalOcean", "Online", "DigitalOcean"},
		{"Acme", "", "Acme"},
	}
	for _, tt := range tests {
		if got := expenseVendorName(tt.supplier, tt.store); got != tt.want {
			t.Fatalf("expenseVendorName(%q, %q) = %q; want %q", tt.supplier, tt.store, got, tt.want)
		}
	}
}

func TestPrepareExpenseImportRowUsesCanonicalProviderWithoutStoreInVendor(t *testing.T) {
	idx := headerIndex(strings.Split("Import ID,Date,Supplier,Store,Document Type,Payment Type,Tax Treatment,Category,Option,Total Inc GST,Business %,Business Amount,Business GST", ","))
	state := &expenseImportState{
		canonicalProviderName: func(value string) string {
			if strings.EqualFold(strings.TrimSpace(value), "Bunnings Warehouse") {
				return "Bunnings"
			}
			return strings.TrimSpace(value)
		},
	}
	rec := strings.Split("EXP-1,1/10/2026,Bunnings Warehouse,Dural,Invoice,,GST,Materials,Consumables,110,100,110,10", ",")

	row := prepareExpenseImportRow(state, nil, idx, rec, 2)

	if row.err != nil {
		t.Fatal(row.err)
	}
	if row.supplier != "Bunnings" || row.vendorName != "Bunnings" {
		t.Fatalf("supplier=%q vendor=%q; want canonical Bunnings", row.supplier, row.vendorName)
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

func TestReceiptContentIsUploadedOnceWithDurableOwner(t *testing.T) {
	receiptsRoot := t.TempDir()
	receiptPath := filepath.Join(receiptsRoot, "receipt.pdf")
	if err := os.WriteFile(receiptPath, []byte("one physical receipt"), 0o644); err != nil {
		t.Fatal(err)
	}

	var expenseCreates, expenseUpdates, uploads, ownerUpdates int
	expenses := make(map[string]*invoiceninja.Expense)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/vendors":
			_, _ = w.Write([]byte(`{"data":[{"id":"vendor1","name":"Bunnings"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/expense_categories":
			_, _ = w.Write([]byte(`{"data":[{"id":"category1","name":"Materials"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/projects" || r.URL.Path == "/api/v1/quotes"):
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/expenses":
			current := make([]invoiceninja.Expense, 0, len(expenses))
			for _, expense := range expenses {
				current = append(current, *expense)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": current,
				"meta": map[string]any{"pagination": map[string]any{"total_pages": 1}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/statics":
			_, _ = w.Write([]byte(`{"payment_types":[{"id":"5","name":"Visa Card"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/companies/current":
			_, _ = w.Write([]byte(`{"data":{"id":"company1","notify_vendor_when_paid":false}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/expenses":
			expenseCreates++
			id := fmt.Sprintf("expense%d", expenseCreates)
			var request invoiceninja.CreateExpenseRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			expense := &invoiceninja.Expense{
				Entity:               invoiceninja.Entity{ID: id},
				VendorID:             request.VendorID,
				ProjectID:            request.ProjectID,
				CategoryID:           request.CategoryID,
				Amount:               request.Amount,
				Date:                 request.Date,
				PaymentDate:          request.PaymentDate,
				PaymentTypeID:        request.PaymentTypeID,
				PrivateNotes:         request.PrivateNotes,
				TransactionReference: request.TransactionReference,
				TaxName1:             request.TaxName1,
				TaxRate1:             request.TaxRate1,
				TaxAmount1:           request.TaxAmount1,
				UsesInclusiveTaxes:   request.UsesInclusiveTaxes,
				CalculateTaxByAmount: request.CalculateTaxByAmount,
				CustomValue1:         request.CustomValue1,
				CustomValue2:         request.CustomValue2,
				CustomValue3:         request.CustomValue3,
				CustomValue4:         request.CustomValue4,
			}
			expenses[id] = expense
			_ = json.NewEncoder(w).Encode(map[string]any{"data": expense})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/upload"):
			uploads++
			parts := strings.Split(r.URL.Path, "/")
			id := parts[len(parts)-2]
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
			}
			files := r.MultipartForm.File["documents[]"]
			if len(files) != 1 {
				t.Errorf("upload files = %#v", files)
			} else {
				expenses[id].Documents = []invoiceninja.Document{{Name: files[0].Filename}}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": expenses[id]})
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/api/v1/expenses/"):
			var raw map[string]json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
				t.Error(err)
			}
			encoded, err := json.Marshal(raw)
			if err != nil {
				t.Error(err)
			}
			id := strings.TrimPrefix(r.URL.Path, "/api/v1/expenses/")
			if _, fullUpdate := raw["vendor_id"]; fullUpdate {
				expenseUpdates++
				var request invoiceninja.UpdateExpenseRequest
				if err := json.Unmarshal(encoded, &request); err != nil {
					t.Error(err)
				}
				applyExpenseUpdateState(expenses[id], request, expenses[id].Documents)
			} else {
				var request invoiceninja.ExpensePaymentStatusRequest
				if err := json.Unmarshal(encoded, &request); err != nil {
					t.Error(err)
				}
				if request.PrivateNotes == nil || !strings.Contains(*request.PrivateNotes, receiptOwnerMarker) {
					t.Errorf("receipt owner update missing marker: %#v", request)
				} else {
					expenses[id].PrivateNotes = *request.PrivateNotes
				}
				expenses[id].PaymentDate = request.PaymentDate
				expenses[id].PaymentTypeID = request.PaymentTypeID
				ownerUpdates++
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": expenses[id]})
		default:
			http.Error(w, r.Method+" "+r.URL.String(), http.StatusNotFound)
		}
	}))
	defer ts.Close()

	csv := strings.Join([]string{
		v052ExpenseCSVHeader() + ",File Name",
		"EXP-A,2/10/2026,Bunnings,Invoice,Visa Card,Expense,Materials,Consumables,55,100,55,5,receipt.pdf",
		"EXP-B,2/10/2026,Bunnings,Invoice,Visa Card,Expense,Materials,Consumables,55,100,55,5,receipt.pdf",
	}, "\n")
	service := newExpenseImportTestService(t, ts)
	results, err := service.ImportExpensesCSVWithOptions(context.Background(), strings.NewReader(csv), ExpenseImportOptions{
		ReceiptsRoot: receiptsRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || expenseCreates != 2 || uploads != 1 || ownerUpdates != 1 {
		t.Fatalf("results=%#v creates=%d uploads=%d owner updates=%d", results, expenseCreates, uploads, ownerUpdates)
	}

	results, err = service.ImportExpensesCSVWithOptions(context.Background(), strings.NewReader(csv), ExpenseImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Action != "unchanged" || results[1].Action != "unchanged" {
		t.Fatalf("rerun results=%#v", results)
	}
	if expenseCreates != 2 || expenseUpdates != 0 || uploads != 1 || ownerUpdates != 1 {
		t.Fatalf("creates=%d updates=%d uploads=%d owner updates=%d", expenseCreates, expenseUpdates, uploads, ownerUpdates)
	}

	var receiptKey string
	owners := 0
	for id, expense := range expenses {
		key := receiptKeyFromText(expense.PrivateNotes)
		if key == "" {
			t.Fatalf("%s missing receipt key: %q", id, expense.PrivateNotes)
		}
		if receiptKey == "" {
			receiptKey = key
		} else if key != receiptKey {
			t.Fatalf("%s receipt key = %q; want %q", id, key, receiptKey)
		}
		if strings.Contains(expense.PrivateNotes, receiptOwnerMarker) {
			owners++
		}
	}
	if owners != 1 {
		t.Fatalf("durable owners = %d; expenses=%#v", owners, expenses)
	}
}

func TestReceiptRenameRemainsReentrantWithoutDuplicateUpload(t *testing.T) {
	receiptsRoot := t.TempDir()
	originalPath := filepath.Join(receiptsRoot, "original.pdf")
	renamedPath := filepath.Join(receiptsRoot, "renamed.pdf")
	if err := os.WriteFile(originalPath, []byte("same physical receipt"), 0o644); err != nil {
		t.Fatal(err)
	}

	var current *invoiceninja.Expense
	var expenseCreates, uploads int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/vendors":
			_, _ = w.Write([]byte(`{"data":[{"id":"vendor1","name":"Bunnings"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/expense_categories":
			_, _ = w.Write([]byte(`{"data":[{"id":"category1","name":"Materials"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/projects" || r.URL.Path == "/api/v1/quotes"):
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/expenses":
			expenses := []invoiceninja.Expense(nil)
			if current != nil {
				expenses = append(expenses, *current)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": expenses,
				"meta": map[string]any{"pagination": map[string]any{"total_pages": 1}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/statics":
			_, _ = w.Write([]byte(`{"payment_types":[{"id":"5","name":"Visa Card"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/companies/current":
			_, _ = w.Write([]byte(`{"data":{"id":"company1","notify_vendor_when_paid":false}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/expenses":
			expenseCreates++
			var request invoiceninja.CreateExpenseRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			current = &invoiceninja.Expense{
				Entity:               invoiceninja.Entity{ID: "expense1"},
				VendorID:             request.VendorID,
				ProjectID:            request.ProjectID,
				CategoryID:           request.CategoryID,
				Amount:               request.Amount,
				Date:                 request.Date,
				PaymentDate:          request.PaymentDate,
				PaymentTypeID:        request.PaymentTypeID,
				PrivateNotes:         request.PrivateNotes,
				TransactionReference: request.TransactionReference,
				TaxName1:             request.TaxName1,
				TaxRate1:             request.TaxRate1,
				TaxAmount1:           request.TaxAmount1,
				UsesInclusiveTaxes:   request.UsesInclusiveTaxes,
				CalculateTaxByAmount: request.CalculateTaxByAmount,
				CustomValue1:         request.CustomValue1,
				CustomValue2:         request.CustomValue2,
				CustomValue3:         request.CustomValue3,
				CustomValue4:         request.CustomValue4,
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": current})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/expenses/expense1/upload":
			uploads++
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
			}
			files := r.MultipartForm.File["documents[]"]
			if len(files) != 1 {
				t.Errorf("upload files = %#v", files)
			} else {
				current.Documents = []invoiceninja.Document{{Name: files[0].Filename}}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": current})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/expenses/expense1":
			var raw map[string]json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
				t.Error(err)
			}
			encoded, err := json.Marshal(raw)
			if err != nil {
				t.Error(err)
			}
			if _, fullUpdate := raw["vendor_id"]; fullUpdate {
				var request invoiceninja.UpdateExpenseRequest
				if err := json.Unmarshal(encoded, &request); err != nil {
					t.Error(err)
				}
				applyExpenseUpdateState(current, request, current.Documents)
			} else {
				var request invoiceninja.ExpensePaymentStatusRequest
				if err := json.Unmarshal(encoded, &request); err != nil {
					t.Error(err)
				}
				current.PaymentDate = request.PaymentDate
				current.PaymentTypeID = request.PaymentTypeID
				if request.PrivateNotes != nil {
					current.PrivateNotes = *request.PrivateNotes
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": current})
		default:
			http.Error(w, r.Method+" "+r.URL.String(), http.StatusNotFound)
		}
	}))
	defer ts.Close()

	service := newExpenseImportTestService(t, ts)
	run := func(filename string) CSVImportResult {
		t.Helper()
		csv := strings.Join([]string{
			v052ExpenseCSVHeader() + ",File Name",
			"EXP-RECEIPT,2/10/2026,Bunnings,Invoice,Visa Card,Expense,Materials,Consumables,55,100,55,5," + filename,
		}, "\n")
		results, err := service.ImportExpensesCSVWithOptions(context.Background(), strings.NewReader(csv), ExpenseImportOptions{
			ReceiptsRoot: receiptsRoot,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 {
			t.Fatalf("results = %#v", results)
		}
		return results[0]
	}

	if result := run("original.pdf"); result.Action != "created" {
		t.Fatalf("original import = %#v", result)
	}
	if err := os.Rename(originalPath, renamedPath); err != nil {
		t.Fatal(err)
	}
	if result := run("renamed.pdf"); result.Action != "updated" {
		t.Fatalf("renamed import = %#v", result)
	}
	if result := run("renamed.pdf"); result.Action != "unchanged" {
		t.Fatalf("renamed rerun = %#v", result)
	}
	if expenseCreates != 1 || uploads != 1 {
		t.Fatalf("creates=%d uploads=%d; want one of each", expenseCreates, uploads)
	}
	if current == nil || len(current.Documents) != 1 || current.Documents[0].Name != "original.pdf" {
		t.Fatalf("durable receipt document = %#v", current)
	}
	if got := privateNoteValues(current.PrivateNotes)["Source file"]; got != "renamed.pdf" {
		t.Fatalf("source filename = %q; want renamed.pdf", got)
	}
}

func TestExistingReceiptOwnerWithoutDocumentFailsSafely(t *testing.T) {
	state := &expenseImportState{
		receiptOwnerByKey:     map[string]invoiceninja.Expense{},
		receiptOwnerAmbiguous: map[string]bool{},
	}
	expense := invoiceninja.Expense{
		Entity:       invoiceninja.Entity{ID: "expense1"},
		PrivateNotes: receiptMarker("abc") + "\n" + receiptOwnerMarker,
	}
	if err := registerExistingReceiptState(state, expense); err == nil || !strings.Contains(err.Error(), "no expected receipt document") {
		t.Fatalf("receipt state error = %v", err)
	}
}

func TestExistingReceiptOwnerRejectsContentChangeAndAcceptsRename(t *testing.T) {
	expense := invoiceninja.Expense{
		Entity:       invoiceninja.Entity{ID: "expense1"},
		PrivateNotes: "Source file: old.pdf\n" + receiptMarker("abc") + "\n" + receiptOwnerMarker,
		Documents:    []invoiceninja.Document{{Name: "old.pdf"}},
	}
	newContent := &preparedExpenseImportRow{
		documentType:    documentTypeInvoice,
		receiptName:     "old.pdf",
		receiptKey:      "different",
		existingExpense: &expense,
	}
	state := &expenseImportState{
		receiptOwnerByKey:     map[string]invoiceninja.Expense{"abc": expense},
		receiptOwnerAmbiguous: map[string]bool{},
	}
	if err := validateAndAssignReceiptOwnership(state, []*preparedExpenseImportRow{newContent}); err == nil || !strings.Contains(err.Error(), "changed from receipt key") {
		t.Fatalf("content-change error = %v", err)
	}

	renamed := &preparedExpenseImportRow{
		documentType:    documentTypeInvoice,
		receiptName:     "renamed.pdf",
		receiptKey:      "abc",
		existingExpense: &expense,
	}
	if err := validateAndAssignReceiptOwnership(state, []*preparedExpenseImportRow{renamed}); err != nil {
		t.Fatal(err)
	}
	if !renamed.receiptOwner || !renamed.receiptHasDocument {
		t.Fatalf("renamed receipt owner = %#v", renamed)
	}

	withoutRoot := &preparedExpenseImportRow{
		documentType:    documentTypeInvoice,
		receiptName:     "old.pdf",
		existingExpense: &expense,
	}
	if err := validateAndAssignReceiptOwnership(state, []*preparedExpenseImportRow{withoutRoot}); err != nil {
		t.Fatal(err)
	}
	if withoutRoot.receiptKey != "abc" || !withoutRoot.receiptOwner || !withoutRoot.receiptHasDocument {
		t.Fatalf("receipt without root = %#v", withoutRoot)
	}
}

func TestImportExpenseCreatesProjectFromNumericMasterQuote(t *testing.T) {
	var projectRequest invoiceninja.CreateProjectRequest
	var expenseRequest map[string]json.RawMessage
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/vendors":
			_, _ = w.Write([]byte(`{"data":[{"id":"vendor1","name":"Bunnings"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/expense_categories":
			_, _ = w.Write([]byte(`{"data":[{"id":"category1","name":"Materials"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/expenses":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/quotes":
			_, _ = w.Write([]byte(`{"data":[{"id":"quote1","number":"1234","client_id":"client1"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/statics":
			_, _ = w.Write([]byte(`{"payment_types":[{"id":"5","name":"Visa Card","gateway_type_id":1}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/companies/current":
			_, _ = w.Write([]byte(`{"data":{"id":"company1","settings":{"currency_id":"company-currency"},"notify_vendor_when_paid":false}}`))
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
		"Import ID,Date,Supplier,Store,Document Type,Payment Type,Tax Treatment,Category,Option,Total Inc GST,Business %,Business Amount,Business GST,Job Number,File Name",
		"expense-1,2/10/2026,Bunnings,Dural,Invoice,Visa Card,Expense - Materials,Materials,Consumables,110,100,110,10,1234,",
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
	assertJSONText(t, expenseRequest, "payment_type_id", "5")
	assertJSONText(t, expenseRequest, "vendor_id", "vendor1")
	if _, exists := expenseRequest["should_be_invoiced"]; exists {
		t.Fatalf("should_be_invoiced must be omitted: %#v", expenseRequest)
	}
}

func TestImportIDCorrectionUpdatesSameExpenseIncludingExplicitZero(t *testing.T) {
	marker := importIDMarker("EXP-1")
	var update invoiceninja.UpdateExpenseRequest
	var updateRaw map[string]json.RawMessage
	var creates int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/vendors":
			_, _ = w.Write([]byte(`{"data":[{"id":"vendor1","name":"Old Supplier"},{"id":"vendor2","name":"New Supplier"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/expense_categories":
			_, _ = w.Write([]byte(`{"data":[{"id":"category1","name":"Old"},{"id":"category2","name":"Hardware"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/projects" || r.URL.Path == "/api/v1/quotes"):
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/expenses":
			body := fmt.Sprintf(`{"data":[{"id":"expense1","vendor_id":"vendor1","category_id":"category1","amount":10,"date":"2026-10-01","tax_name1":"GST","tax_rate1":10,"tax_amount1":0.91,"private_notes":%q}],"meta":{"pagination":{"total_pages":1}}}`, marker)
			_, _ = w.Write([]byte(body))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/statics":
			_, _ = w.Write([]byte(`{"payment_types":[]}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/expenses/expense1":
			if err := json.NewDecoder(r.Body).Decode(&updateRaw); err != nil {
				t.Error(err)
			}
			encoded, err := json.Marshal(updateRaw)
			if err != nil {
				t.Error(err)
			}
			if err := json.Unmarshal(encoded, &update); err != nil {
				t.Error(err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "expense1"}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/expenses":
			creates++
			http.Error(w, "unexpected create", http.StatusInternalServerError)
		default:
			http.Error(w, r.Method+" "+r.URL.String(), http.StatusNotFound)
		}
	}))
	defer ts.Close()

	csv := strings.Join([]string{
		v052ExpenseCSVHeader(),
		"EXP-1,2/10/2026,New Supplier,Invoice,,Expense,Hardware,Corrected,20,0,0,0",
	}, "\n")
	results, err := newExpenseImportTestService(t, ts).ImportExpensesCSV(context.Background(), strings.NewReader(csv), false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "updated" || results[0].ID != "expense1" || creates != 0 {
		t.Fatalf("results=%#v creates=%d", results, creates)
	}
	if update.VendorID != "vendor2" || update.CategoryID != "category2" || update.Amount != 0 || update.Date != "2026-10-02" || update.TaxAmount1 != 0 || update.TaxName1 != "" || update.TaxRate1 != 0 {
		t.Fatalf("correction update = %#v", update)
	}
	for _, field := range []string{"amount", "tax_name1", "tax_rate1", "tax_amount1", "payment_date", "payment_type_id"} {
		if _, ok := updateRaw[field]; !ok {
			t.Fatalf("explicit correction field %q was omitted: %#v", field, updateRaw)
		}
	}
	if !strings.Contains(update.PrivateNotes, marker) {
		t.Fatalf("Import ID marker was not retained: %q", update.PrivateNotes)
	}
}

func TestImportExistingUnpaidPurchaseClearsLegacyPaidState(t *testing.T) {
	header := []string{
		"Import ID", "Date", "Supplier", "Store", "Document Type", "Payment Type", "Tax Treatment", "Category", "Option",
		"Total Inc GST", "Business %", "Business Amount", "Business GST", "Invoice Number",
	}
	rec := []string{"expense-1", "2/10/2026", "Bunnings", "Dural", "Invoice", "", "Expense - Materials", "Materials", "Consumables", "110", "100", "110", "10", "INV-1"}
	marker := purchaseSourceMarker(rec, headerIndex(header), "2026-10-02")
	var paymentStatus invoiceninja.ExpensePaymentStatusRequest
	var writeCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/vendors":
			_, _ = w.Write([]byte(`{"data":[{"id":"vendor1","name":"Bunnings"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/expense_categories":
			_, _ = w.Write([]byte(`{"data":[{"id":"category1","name":"Materials"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/expenses":
			body := fmt.Sprintf(`{"data":[{"id":"expense1","vendor_id":"vendor1","date":"2026-10-02","payment_date":"2026-10-02","payment_type_id":"5","private_notes":%q}],"meta":{"pagination":{"total_pages":1}}}`, marker)
			_, _ = w.Write([]byte(body))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/quotes":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/statics":
			_, _ = w.Write([]byte(`{"payment_types":[{"id":"5","name":"Visa Card","gateway_type_id":1}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_integrations":
			_, _ = w.Write([]byte(`{"data":[{"id":"bank1","bank_account_name":"GoTradie","integration_type":"","auto_sync":false}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_transactions":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_transaction_rules":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/expenses/expense1":
			writeCount++
			if err := json.NewDecoder(r.Body).Decode(&paymentStatus); err != nil {
				t.Error(err)
			}
			if err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id":              "expense1",
				"payment_date":    paymentStatus.PaymentDate,
				"payment_type_id": paymentStatus.PaymentTypeID,
				"private_notes":   paymentStatus.PrivateNotes,
			}}); err != nil {
				t.Error(err)
			}
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
	csv := strings.Join([]string{strings.Join(header, ","), strings.Join(rec, ",")}, "\n")
	results, err := service.ImportExpensesCSV(context.Background(), strings.NewReader(csv), false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "updated" || writeCount != 1 {
		t.Fatalf("unexpected result=%#v writes=%d", results, writeCount)
	}
	if paymentStatus.PaymentDate != "" || paymentStatus.PaymentTypeID != "" {
		t.Fatalf("paid state was not cleared: %#v", paymentStatus)
	}
	if paymentStatus.PrivateNotes == nil || !strings.Contains(*paymentStatus.PrivateNotes, importIDMarker("expense-1")) {
		t.Fatalf("legacy Expense did not gain durable Import ID: %#v", paymentStatus)
	}
}

func TestAccountPaymentCommitCreatesWithdrawalWithoutFakeExpenseOrCustomerPayment(t *testing.T) {
	var transactionRequest invoiceninja.CreateBankTransactionRequest
	var transactionCreates, expenseCreates, customerPaymentCreates int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_integrations":
			_, _ = w.Write([]byte(`{"data":[{"id":"bank1","bank_account_name":"GoTradie","integration_type":"","auto_sync":false}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_transactions":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_transaction_rules":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/companies/current":
			_, _ = w.Write([]byte(`{"data":{"id":"company1","settings":{"currency_id":"company-currency"},"notify_vendor_when_paid":false}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/bank_transactions":
			transactionCreates++
			if err := json.NewDecoder(r.Body).Decode(&transactionRequest); err != nil {
				t.Error(err)
			}
			_, _ = w.Write([]byte(`{"data":{"id":"transaction1","bank_integration_id":"bank1","amount":71.84,"base_type":"DEBIT","date":"2026-11-01"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/expenses":
			expenseCreates++
			http.Error(w, "unexpected expense", http.StatusInternalServerError)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/payments":
			customerPaymentCreates++
			http.Error(w, "unexpected payment", http.StatusInternalServerError)
		case r.Method == http.MethodGet:
			switch r.URL.Path {
			case "/api/v1/vendors", "/api/v1/expense_categories", "/api/v1/projects", "/api/v1/expenses", "/api/v1/quotes":
				_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
			case "/api/v1/statics":
				_, _ = w.Write([]byte(`{"payment_types":[{"id":"5","name":"Visa Card","gateway_type_id":1}]}`))
			default:
				http.Error(w, r.Method+" "+r.URL.String(), http.StatusNotFound)
			}
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
		"Import ID,Date,Supplier,Store,Document Type,Payment Type,Tax Treatment,Category,Option,Total Inc GST,Business %,Business Amount,Business GST,Invoice Number,File Name",
		"payment-1,1/11/2026,Bunnings,Dural,Account Payment,Visa Card,formula,formula,formula,71.84,formula,formula,formula,PAY-1,payment.pdf",
	}, "\n")
	results, err := service.ImportExpensesCSV(context.Background(), strings.NewReader(csv), false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "created-transaction" || results[0].ID != "transaction1" || results[0].UnappliedAmount != 71.84 {
		t.Fatalf("unexpected result=%#v", results)
	}
	if transactionCreates != 1 || expenseCreates != 0 || customerPaymentCreates != 0 {
		t.Fatalf("unexpected writes: transactions=%d expenses=%d customer payments=%d", transactionCreates, expenseCreates, customerPaymentCreates)
	}
	if transactionRequest.BankIntegrationID != "bank1" || transactionRequest.CurrencyID != "company-currency" || transactionRequest.BaseType != "DEBIT" || transactionRequest.Amount != 71.84 || transactionRequest.Date != "2026-11-01" || transactionRequest.ParticipantName != "Bunnings" {
		t.Fatalf("unexpected transaction request: %#v", transactionRequest)
	}
	if !strings.Contains(transactionRequest.Description, importIDMarkerPrefix) || !strings.Contains(transactionRequest.Description, supplierAccountMarker("Bunnings")) {
		t.Fatalf("transaction markers missing: %q", transactionRequest.Description)
	}
	if !containsChange(results[0].Changes, "supporting-document:unattached:payment.pdf") {
		t.Fatalf("supporting document not reported: %#v", results[0])
	}
}

func TestNegativeSupplierReturnCreatesExpenseWithoutBankTransaction(t *testing.T) {
	var expenseRequest invoiceninja.CreateExpenseRequest
	var expenseCreates, transactionCreates int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/vendors":
			_, _ = w.Write([]byte(`{"data":[{"id":"vendor1","name":"Bunnings"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/expense_categories":
			_, _ = w.Write([]byte(`{"data":[{"id":"category1","name":"Materials"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/projects" || r.URL.Path == "/api/v1/expenses" || r.URL.Path == "/api/v1/quotes"):
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/statics":
			_, _ = w.Write([]byte(`{"payment_types":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/expenses":
			expenseCreates++
			if err := json.NewDecoder(r.Body).Decode(&expenseRequest); err != nil {
				t.Error(err)
			}
			_, _ = w.Write([]byte(`{"data":{"id":"expense-return"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/bank_transactions":
			transactionCreates++
			http.Error(w, "unexpected bank transaction", http.StatusInternalServerError)
		default:
			http.Error(w, r.Method+" "+r.URL.String(), http.StatusNotFound)
		}
	}))
	defer ts.Close()

	client, err := invoiceninja.New("token", invoiceninja.WithBaseURL(ts.URL), invoiceninja.WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatal(err)
	}
	csv := strings.Join([]string{
		"Import ID,Date,Supplier,Store,Document Type,Payment Type,Tax Treatment,Category,Option,Total Inc GST,Business %,Business Amount,Business GST,Invoice Number",
		"return-1,2/1/2026,Bunnings,Dural,Invoice,,Expense - Materials,Materials,Consumables,-20,100,-20,-1.82,RETURN-1",
	}, "\n")
	results, err := (&Service{client: client}).ImportExpensesCSV(context.Background(), strings.NewReader(csv), false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "created" || results[0].ID != "expense-return" {
		t.Fatalf("return import results = %#v", results)
	}
	if expenseCreates != 1 || transactionCreates != 0 {
		t.Fatalf("writes: expenses=%d bank transactions=%d", expenseCreates, transactionCreates)
	}
	if expenseRequest.Amount != -20 || !strings.Contains(expenseRequest.PrivateNotes, supplierAccountMarkerPrefix) || !strings.Contains(expenseRequest.PrivateNotes, settlementPurchaseMarkerPrefix) {
		t.Fatalf("return expense request = %#v", expenseRequest)
	}
}

func TestExpensePreviewTracksPlannedDependencies(t *testing.T) {
	idx := headerIndex([]string{
		"Date", "Supplier", "Store", "Document Type", "Payment Type", "Tax Treatment", "Category", "Option",
		"Total Inc GST", "Business %", "Business Amount", "Business GST",
		"Job Number", "Invoice Number",
	})
	state := &expenseImportState{
		vendors:          map[string]invoiceninja.Vendor{},
		vendorByID:       map[string]invoiceninja.Vendor{},
		categories:       map[string]invoiceninja.ExpenseCategory{},
		projects:         map[string]invoiceninja.Project{},
		clientByQuote:    map[string]string{"1234": "client1"},
		paymentTypes:     map[string]invoiceninja.PaymentType{},
		expenseByMarker:  map[string]invoiceninja.Expense{},
		ambiguousMarkers: map[string]bool{},
		seenMarkers:      map[string]int{},
	}
	service := &Service{}
	row := func(invoice string) []string {
		return []string{"2/10/2026", "Bunnings", "Dural", "Invoice", "", "Expense - Materials", "Materials", "Consumables", "110", "100", "110", "10", "1234", invoice}
	}

	firstRow := prepareExpenseImportRow(state, nil, idx, row("INV-1"), 2)
	secondRow := prepareExpenseImportRow(state, nil, idx, row("INV-2"), 3)
	first := service.importExpenseRow(context.Background(), state, idx, firstRow, true, false)
	second := service.importExpenseRow(context.Background(), state, idx, secondRow, true, false)
	for _, want := range []string{"vendor:create:Bunnings", "category:create:Materials", "project:create:1234"} {
		if !containsChange(first.Changes, want) {
			t.Fatalf("first preview changes %v do not contain %q", first.Changes, want)
		}
		if containsChange(second.Changes, want) {
			t.Fatalf("second preview repeats %q in %v", want, second.Changes)
		}
	}
}

func TestAllocateAccountPaymentsOldestFirstAndLeavesPartialUnpaid(t *testing.T) {
	purchaseOne := &preparedExpenseImportRow{
		rowNo: 2, documentType: documentTypeInvoice, supplier: "Bunnings", date: "2026-09-18",
		sourceMarker: "purchase-1", grossCents: 7184, remainingCents: 7184,
	}
	purchaseTwo := &preparedExpenseImportRow{
		rowNo: 3, documentType: documentTypeInvoice, supplier: "Bunnings", date: "2026-09-18",
		sourceMarker: "purchase-2", grossCents: 2870, remainingCents: 2870,
	}
	payment := &preparedExpenseImportRow{
		rowNo: 4, documentType: documentTypeAccountPayment, supplier: "Bunnings", date: "2026-11-01",
		sourceMarker: "payment-1", grossCents: 8000, paymentType: "Visa Card", paymentTypeID: "5", reference: "PAY-1",
	}

	mustAllocateAccountPayments(t, &expenseImportState{}, []*preparedExpenseImportRow{purchaseTwo, payment, purchaseOne})

	if purchaseOne.remainingCents != 0 || purchaseOne.desiredPaymentDate != "2026-11-01" || purchaseOne.desiredPaymentTypeID != "5" {
		t.Fatalf("first purchase settlement = %#v", purchaseOne)
	}
	if purchaseTwo.remainingCents != 2054 || purchaseTwo.desiredPaymentDate != "" || purchaseTwo.desiredPaymentTypeID != "" {
		t.Fatalf("partial purchase settlement = %#v", purchaseTwo)
	}
	if len(payment.allocations) != 2 || payment.allocations[0].PurchaseSourceID != "purchase-1" || payment.allocations[0].Amount != 71.84 || payment.allocations[1].Amount != 8.16 {
		t.Fatalf("unexpected allocations: %#v", payment.allocations)
	}
	if payment.unappliedCents != 0 {
		t.Fatalf("unapplied cents = %d", payment.unappliedCents)
	}
}

func TestAllocateAccountPaymentsUsesEligibilityAndReportsRemainder(t *testing.T) {
	future := &preparedExpenseImportRow{
		rowNo: 3, documentType: documentTypeReceipt, supplier: "Bunnings", date: "2026-11-02",
		sourceMarker: "future", grossCents: 5000, remainingCents: 5000,
	}
	otherSupplier := &preparedExpenseImportRow{
		rowNo: 2, documentType: documentTypeInvoice, supplier: "Other", date: "2026-09-01",
		sourceMarker: "other", grossCents: 2000, remainingCents: 2000,
	}
	payment := &preparedExpenseImportRow{
		rowNo: 4, documentType: documentTypeAccountPayment, supplier: "Bunnings", date: "2026-11-01",
		sourceMarker: "payment", grossCents: 3000, paymentType: "PayPal", paymentTypeID: "13",
	}

	mustAllocateAccountPayments(t, &expenseImportState{}, []*preparedExpenseImportRow{future, otherSupplier, payment})
	if len(payment.allocations) != 0 || payment.unappliedCents != 3000 {
		t.Fatalf("payment should be unapplied: %#v", payment)
	}
}

func TestAllocateAccountPaymentsBlocksOnInvalidEarlierPurchase(t *testing.T) {
	invalid := &preparedExpenseImportRow{
		rowNo: 2, documentType: documentTypeInvoice, supplier: "Bunnings", date: "2026-09-01",
		grossCents: 5000, grossKnown: true, err: fmt.Errorf("missing category"),
	}
	later := &preparedExpenseImportRow{
		rowNo: 3, documentType: documentTypeInvoice, supplier: "Bunnings", date: "2026-09-02",
		sourceMarker: "later", grossCents: 5000, grossKnown: true, remainingCents: 5000,
	}
	payment := &preparedExpenseImportRow{
		rowNo: 4, documentType: documentTypeAccountPayment, supplier: "Bunnings", date: "2026-10-01",
		sourceMarker: "payment", grossCents: 5000, paymentType: "Visa Card", paymentTypeID: "5",
	}

	mustAllocateAccountPayments(t, &expenseImportState{}, []*preparedExpenseImportRow{invalid, later, payment})
	if payment.err == nil || !strings.Contains(payment.err.Error(), "row 2") || len(payment.allocations) != 0 || later.remainingCents != 5000 {
		t.Fatalf("invalid earlier purchase did not block allocation: later=%#v payment=%#v", later, payment)
	}
}

func TestAllocateAccountPaymentsBlocksOnMissingOlderImportedPurchase(t *testing.T) {
	state := &expenseImportState{importedExpenses: []importedExpenseState{{
		expense:  invoiceninja.Expense{Entity: invoiceninja.Entity{ID: "expense-old"}},
		supplier: "Bunnings",
		date:     "2026-09-01",
	}}}
	later := &preparedExpenseImportRow{
		rowNo: 2, documentType: documentTypeInvoice, supplier: "Bunnings", date: "2026-09-02",
		sourceMarker: "later", grossCents: 5000, grossKnown: true, remainingCents: 5000,
	}
	payment := &preparedExpenseImportRow{
		rowNo: 3, documentType: documentTypeAccountPayment, supplier: "Bunnings", date: "2026-10-01",
		sourceMarker: "payment", grossCents: 5000, paymentType: "Visa Card", paymentTypeID: "5",
	}

	mustAllocateAccountPayments(t, state, []*preparedExpenseImportRow{later, payment})
	if payment.err == nil || !strings.Contains(payment.err.Error(), "expense-old") || len(payment.allocations) != 0 || later.remainingCents != 5000 {
		t.Fatalf("missing older imported purchase did not block allocation: later=%#v payment=%#v", later, payment)
	}
}

func TestDuplicatePurchaseDoesNotConsumePaymentTwice(t *testing.T) {
	first := &preparedExpenseImportRow{
		rowNo: 2, documentType: documentTypeInvoice, supplier: "Bunnings", date: "2026-09-18",
		sourceMarker: "same-purchase", grossCents: 5000, remainingCents: 5000,
	}
	duplicate := &preparedExpenseImportRow{
		rowNo: 3, documentType: documentTypeInvoice, supplier: "Bunnings", date: "2026-09-18",
		sourceMarker: "same-purchase", grossCents: 5000, remainingCents: 5000,
	}
	payment := &preparedExpenseImportRow{
		rowNo: 4, documentType: documentTypeAccountPayment, supplier: "Bunnings", date: "2026-11-01",
		sourceMarker: "payment", grossCents: 7500, paymentType: "Visa Card", paymentTypeID: "5",
	}

	duplicate.duplicateRow = first.rowNo
	mustAllocateAccountPayments(t, &expenseImportState{}, []*preparedExpenseImportRow{first, duplicate, payment})
	if len(payment.allocations) != 1 || first.remainingCents != 0 || duplicate.remainingCents != 5000 || payment.unappliedCents != 2500 {
		t.Fatalf("duplicate affected allocation: first=%#v duplicate=%#v payment=%#v", first, duplicate, payment)
	}
}

func TestMultiplePaymentMethodsDoNotInventExpensePaymentType(t *testing.T) {
	purchase := &preparedExpenseImportRow{
		rowNo: 2, documentType: documentTypeInvoice, supplier: "Bunnings", date: "2026-09-18",
		sourceMarker: "purchase", grossCents: 10000, remainingCents: 10000,
	}
	visa := &preparedExpenseImportRow{
		rowNo: 3, documentType: documentTypeAccountPayment, supplier: "Bunnings", date: "2026-10-01",
		sourceMarker: "visa", grossCents: 4000, paymentType: "Visa Card", paymentTypeID: "5",
	}
	paypal := &preparedExpenseImportRow{
		rowNo: 4, documentType: documentTypeAccountPayment, supplier: "Bunnings", date: "2026-11-01",
		sourceMarker: "paypal", grossCents: 6000, paymentType: "PayPal", paymentTypeID: "13",
	}

	mustAllocateAccountPayments(t, &expenseImportState{}, []*preparedExpenseImportRow{paypal, purchase, visa})
	if purchase.desiredPaymentDate != "2026-11-01" || purchase.desiredPaymentTypeID != "" {
		t.Fatalf("mixed settlement = date %q type %q", purchase.desiredPaymentDate, purchase.desiredPaymentTypeID)
	}
	if len(purchase.allocations) != 2 {
		t.Fatalf("allocation detail = %#v", purchase.allocations)
	}
}

func TestPrepareAccountPaymentIgnoresPurchaseAnalyticsAndLeavesDocumentUnattached(t *testing.T) {
	header := []string{
		"Date", "Supplier", "Store", "Document Type", "Payment Type", "Tax Treatment", "Category", "Option",
		"Total Inc GST", "Business %", "Business Amount", "Business GST", "Invoice Number", "File Name",
	}
	idx := headerIndex(header)
	state := &expenseImportState{paymentTypes: map[string]invoiceninja.PaymentType{
		"Visa Card": {ID: "5", Name: "Visa Card"},
	}}
	rec := []string{"1/11/2026", "Bunnings", "Dural", "Account Payment", "Visa Card", "formula", "formula", "formula", "71.84", "not-a-percent", "not-money", "not-money", "PAY-1", "payment.pdf"}
	row := prepareExpenseImportRow(state, receiptIndex{"payment.pdf": {"/would/not/be/uploaded"}}, idx, rec, 2)
	if row.err != nil {
		t.Fatal(row.err)
	}
	if row.grossCents != 7184 || row.paymentTypeID != "5" || row.receiptPath != "" {
		t.Fatalf("unexpected account payment row: %#v", row)
	}
	result := refreshAccountPaymentResult(CSVImportResult{Name: row.name, Action: "would-create-transaction"}, row)
	if !containsChange(result.Changes, "supporting-document:unattached:payment.pdf") {
		t.Fatalf("supporting document not reported: %#v", result)
	}
}

func TestPrepareRowsRejectUnknownPaymentTypeAndDefersAdjustment(t *testing.T) {
	header := []string{
		"Date", "Supplier", "Document Type", "Payment Type", "Tax Treatment", "Category", "Option",
		"Total Inc GST", "Business %", "Business Amount", "Business GST",
	}
	idx := headerIndex(header)
	state := &expenseImportState{paymentTypes: map[string]invoiceninja.PaymentType{
		"Visa Card": {ID: "5", Name: "Visa Card"},
	}}
	unknown := []string{"1/11/2026", "Bunnings", "Invoice", "visa card", "Expense", "Materials", "", "10", "100", "10", "0.91"}
	row := prepareExpenseImportRow(state, nil, idx, unknown, 2)
	if row.err == nil || !strings.Contains(row.err.Error(), "unknown Invoice Ninja Payment Type") {
		t.Fatalf("unknown payment type error = %v", row.err)
	}
	adjustment := []string{"1/11/2026", "Bunnings", "Adjustment", "", "Expense", "Materials", "", "-10", "100", "-10", "-0.91"}
	row = prepareExpenseImportRow(state, nil, idx, adjustment, 3)
	if row.err != nil || !row.deferred {
		t.Fatalf("adjustment was not deferred: %#v", row)
	}
	result := deferredAdjustmentResult(row)
	if result.Action != "deferred" || result.Error != nil {
		t.Fatalf("adjustment result = %#v", result)
	}
}

func TestLegacyExpenseIdentityReconstructsStableMarker(t *testing.T) {
	values := purchaseIdentityValues("2026-10-02", "Bunnings", "Dural", "INV-1", "123", "Nails", "2", "EA", "5", "9.09", ".91", "10", "AUD")
	want := purchaseSourceMarkerFromValues(values)
	expense := invoiceninja.Expense{
		VendorID:             "vendor1",
		Date:                 "2026-10-02",
		TransactionReference: "INV-1",
		PrivateNotes: strings.Join([]string{
			"Store: Dural", "Item number: 123", "Item description: Nails", "Qty: 2.0", "Unit: EA", "Unit price: $5.00",
			"Source total ex GST: $9.09", "Source GST: $0.91", "Source total inc GST: $10.00", "Source currency: AUD",
			"[GoTradie source:0123456789abcdef01234567]",
		}, "\n"),
	}
	got := legacyExpenseSourceMarker(expense, map[string]invoiceninja.Vendor{"vendor1": {Name: "Bunnings - Dural"}})
	if got != want {
		t.Fatalf("legacy marker = %q; want %q", got, want)
	}
}

func TestLegacySplitExpenseIdentitiesUseStoredSourceOrder(t *testing.T) {
	state := &expenseImportState{
		expenseByMarker:  map[string]invoiceninja.Expense{},
		ambiguousMarkers: map[string]bool{},
		ambiguousBases:   map[string]bool{},
		importedExpenses: []importedExpenseState{
			{expense: invoiceninja.Expense{Entity: invoiceninja.Entity{ID: "expense-two"}}, baseMarker: "base", sourceRow: 3},
			{expense: invoiceninja.Expense{Entity: invoiceninja.Entity{ID: "expense-one"}}, baseMarker: "base", sourceRow: 2},
		},
	}
	indexImportedExpenseIdentities(state)
	if got := state.expenseByMarker[purchaseOccurrenceMarker("base", 1)].ID; got != "expense-one" {
		t.Fatalf("first legacy split expense = %q", got)
	}
	if got := state.expenseByMarker[purchaseOccurrenceMarker("base", 2)].ID; got != "expense-two" {
		t.Fatalf("second legacy split expense = %q", got)
	}
}

func TestExpensePaymentNotificationPreflightFailsBeforeWrites(t *testing.T) {
	tests := []struct {
		name        string
		companyBody string
	}{
		{name: "enabled", companyBody: `{"data":{"id":"company1","settings":{"currency_id":"company-currency"},"notify_vendor_when_paid":true}}`},
		{name: "missing", companyBody: `{"data":{"id":"company1","settings":{"currency_id":"company-currency"}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mutationCount int
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/api/v1/vendors":
					_, _ = w.Write([]byte(`{"data":[{"id":"vendor1","name":"Bunnings"}],"meta":{"pagination":{"total_pages":1}}}`))
				case r.Method == http.MethodGet && r.URL.Path == "/api/v1/expense_categories":
					_, _ = w.Write([]byte(`{"data":[{"id":"category1","name":"Materials"}],"meta":{"pagination":{"total_pages":1}}}`))
				case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/projects" || r.URL.Path == "/api/v1/expenses" || r.URL.Path == "/api/v1/quotes"):
					_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
				case r.Method == http.MethodGet && r.URL.Path == "/api/v1/statics":
					_, _ = w.Write([]byte(`{"payment_types":[{"id":"5","name":"Visa Card"}]}`))
				case r.Method == http.MethodPost && r.URL.Path == "/api/v1/companies/current":
					_, _ = w.Write([]byte(tt.companyBody))
				default:
					mutationCount++
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
				"Import ID,Date,Supplier,Document Type,Payment Type,Tax Treatment,Category,Option,Total Inc GST,Business %,Business Amount,Business GST",
				"expense-1,2/10/2026,Bunnings,Invoice,Visa Card,Expense,Materials,Consumables,110,100,110,10",
			}, "\n")
			if _, err := service.ImportExpensesCSV(context.Background(), strings.NewReader(csv), false, ""); err == nil || !strings.Contains(err.Error(), "notify_vendor_when_paid") {
				t.Fatalf("preflight error = %v", err)
			}
			if mutationCount != 0 {
				t.Fatalf("writes occurred before failed preflight: %d", mutationCount)
			}
		})
	}
}

func TestAdjustmentIsDeferredAfterUnrelatedCommit(t *testing.T) {
	var expenseCreates int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/vendors":
			_, _ = w.Write([]byte(`{"data":[{"id":"vendor1","name":"Bunnings"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/expense_categories":
			_, _ = w.Write([]byte(`{"data":[{"id":"category1","name":"Materials"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/projects" || r.URL.Path == "/api/v1/expenses" || r.URL.Path == "/api/v1/quotes"):
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/statics":
			_, _ = w.Write([]byte(`{"payment_types":[]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_integrations":
			_, _ = w.Write([]byte(`{"data":[{"id":"bank1","bank_account_name":"GoTradie","integration_type":"","auto_sync":false}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_transactions":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_transaction_rules":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/expenses":
			expenseCreates++
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
		"Import ID,Date,Supplier,Document Type,Payment Type,Tax Treatment,Category,Option,Total Inc GST,Business %,Business Amount,Business GST",
		"expense-1,2/10/2026,Bunnings,Invoice,,Expense,Materials,Consumables,110,100,110,10",
		"adjustment-1,3/10/2026,Bunnings,Adjustment,,Expense,Materials,Consumables,-10,100,-10,-0.91",
	}, "\n")
	results, err := service.ImportExpensesCSV(context.Background(), strings.NewReader(csv), false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Action != "created" || results[1].Action != "deferred" || expenseCreates != 1 {
		t.Fatalf("unexpected results=%#v expense creates=%d", results, expenseCreates)
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

func mustAllocateAccountPayments(t *testing.T, state *expenseImportState, rows []*preparedExpenseImportRow) {
	t.Helper()
	if _, err := allocateAccountPayments(state, rows, false); err != nil {
		t.Fatal(err)
	}
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
