package ninja

import (
	"context"
	"encoding/json"
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
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/statics":
			_, _ = w.Write([]byte(`{"payment_types":[{"id":"5","name":"Visa Card","gateway_type_id":1}]}`))
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
		"Date,Supplier,Store,Document Type,Payment Type,Tax Treatment,Category,Option,Total Inc GST,Business %,Business Amount,Business GST,Job Number,File Name",
		"2/10/2026,Bunnings,Dural,Invoice,Visa Card,Expense - Materials,Materials,Consumables,110,100,110,10,1234,",
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

func TestImportExistingUnpaidPurchaseClearsLegacyPaidState(t *testing.T) {
	header := []string{
		"Date", "Supplier", "Store", "Document Type", "Payment Type", "Tax Treatment", "Category", "Option",
		"Total Inc GST", "Business %", "Business Amount", "Business GST", "Invoice Number",
	}
	rec := []string{"2/10/2026", "Bunnings", "Dural", "Invoice", "", "Expense - Materials", "Materials", "Consumables", "110", "100", "110", "10", "INV-1"}
	marker := purchaseSourceMarker(rec, headerIndex(header), "2026-10-02")
	var paymentStatus invoiceninja.ExpensePaymentStatusRequest
	var writeCount int
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
			body := fmt.Sprintf(`{"data":[{"id":"expense1","vendor_id":"vendor1","date":"2026-10-02","payment_date":"2026-10-02","payment_type_id":"5","private_notes":%q}],"meta":{"pagination":{"total_pages":1}}}`, marker)
			_, _ = w.Write([]byte(body))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/quotes":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/statics":
			_, _ = w.Write([]byte(`{"payment_types":[{"id":"5","name":"Visa Card","gateway_type_id":1}]}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/expenses/expense1":
			writeCount++
			if err := json.NewDecoder(r.Body).Decode(&paymentStatus); err != nil {
				t.Error(err)
			}
			_, _ = w.Write([]byte(`{"data":{"id":"expense1","payment_date":"","payment_type_id":""}}`))
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
}

func TestAccountPaymentCommitDoesNotCreateFakeExpense(t *testing.T) {
	var writeCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			writeCount++
		}
		switch r.URL.Path {
		case "/api/v1/vendors", "/api/v1/expense_categories", "/api/v1/projects", "/api/v1/expenses", "/api/v1/quotes":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case "/api/v1/statics":
			_, _ = w.Write([]byte(`{"payment_types":[{"id":"5","name":"Visa Card","gateway_type_id":1}]}`))
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
		"Date,Supplier,Store,Document Type,Payment Type,Tax Treatment,Category,Option,Total Inc GST,Business %,Business Amount,Business GST,Invoice Number,File Name",
		"1/11/2026,Bunnings,Dural,Account Payment,Visa Card,formula,formula,formula,71.84,formula,formula,formula,PAY-1,payment.pdf",
	}, "\n")
	results, err := service.ImportExpensesCSV(context.Background(), strings.NewReader(csv), false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "allocated" || results[0].UnappliedAmount != 71.84 || writeCount != 0 {
		t.Fatalf("unexpected result=%#v writes=%d", results, writeCount)
	}
	if !containsChange(results[0].Changes, "supporting-document:unattached:payment.pdf") {
		t.Fatalf("supporting document not reported: %#v", results[0])
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
	first := service.importExpenseRow(context.Background(), state, idx, firstRow, true)
	second := service.importExpenseRow(context.Background(), state, idx, secondRow, true)
	for _, want := range []string{"vendor:create:Bunnings - Dural", "category:create:Materials", "project:create:1234"} {
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

	allocateAccountPayments([]*preparedExpenseImportRow{purchaseTwo, payment, purchaseOne})

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

	allocateAccountPayments([]*preparedExpenseImportRow{future, otherSupplier, payment})
	if len(payment.allocations) != 0 || payment.unappliedCents != 3000 {
		t.Fatalf("payment should be unapplied: %#v", payment)
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

	allocateAccountPayments([]*preparedExpenseImportRow{first, duplicate, payment})
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

	allocateAccountPayments([]*preparedExpenseImportRow{paypal, purchase, visa})
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
	result := accountPaymentResult(row, true)
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
	if row.err == nil || !strings.Contains(row.err.Error(), "Adjustment is deferred") {
		t.Fatalf("adjustment error = %v", row.err)
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
