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
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/companies/current":
			_, _ = w.Write([]byte(`{"data":{"id":"company1","notify_vendor_when_paid":false}}`))
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
	if len(results) != 1 || results[0].Action != "updated" || writeCount != 2 {
		t.Fatalf("unexpected result=%#v writes=%d", results, writeCount)
	}
	if paymentStatus.PaymentDate != "" || paymentStatus.PaymentTypeID != "" {
		t.Fatalf("paid state was not cleared: %#v", paymentStatus)
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
		"Date,Supplier,Store,Document Type,Payment Type,Tax Treatment,Category,Option,Total Inc GST,Business %,Business Amount,Business GST,Invoice Number,File Name",
		"1/11/2026,Bunnings,Dural,Account Payment,Visa Card,formula,formula,formula,71.84,formula,formula,formula,PAY-1,payment.pdf",
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
	if transactionRequest.BankIntegrationID != "bank1" || transactionRequest.BaseType != "DEBIT" || transactionRequest.Amount != 71.84 || transactionRequest.Date != "2026-11-01" || transactionRequest.ParticipantName != "Bunnings" {
		t.Fatalf("unexpected transaction request: %#v", transactionRequest)
	}
	if !strings.Contains(transactionRequest.Description, "[GoTradie account-payment:") || !strings.Contains(transactionRequest.Description, supplierAccountMarker("Bunnings")) {
		t.Fatalf("transaction markers missing: %q", transactionRequest.Description)
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
	first := service.importExpenseRow(context.Background(), state, idx, firstRow, true, false)
	second := service.importExpenseRow(context.Background(), state, idx, secondRow, true, false)
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
		{name: "enabled", companyBody: `{"data":{"id":"company1","notify_vendor_when_paid":true}}`},
		{name: "missing", companyBody: `{"data":{"id":"company1"}}`},
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
				"Date,Supplier,Document Type,Payment Type,Tax Treatment,Category,Option,Total Inc GST,Business %,Business Amount,Business GST",
				"2/10/2026,Bunnings,Invoice,Visa Card,Expense,Materials,Consumables,110,100,110,10",
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
		"Date,Supplier,Document Type,Payment Type,Tax Treatment,Category,Option,Total Inc GST,Business %,Business Amount,Business GST",
		"2/10/2026,Bunnings,Invoice,,Expense,Materials,Consumables,110,100,110,10",
		"3/10/2026,Bunnings,Adjustment,,Expense,Materials,Consumables,-10,100,-10,-0.91",
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
