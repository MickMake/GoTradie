package ninja

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
)

func TestLoadSupplierSettlementStateRequiresDedicatedManualAccount(t *testing.T) {
	tests := []struct {
		name         string
		integrations string
		wantError    string
	}{
		{name: "missing", integrations: `[]`, wantError: `manual bank account "GoTradie" was not found`},
		{name: "archived", integrations: `[{
			"id":"bank1","bank_account_name":"GoTradie","archived_at":1
		}]`, wantError: "archived or deleted"},
		{name: "deleted", integrations: `[{
			"id":"bank1","bank_account_name":"GoTradie","is_deleted":true
		}]`, wantError: "archived or deleted"},
		{name: "duplicate", integrations: `[{
			"id":"bank1","bank_account_name":"GoTradie"
		},{
			"id":"bank2","bank_account_name":"GoTradie"
		}]`, wantError: `2 active bank accounts named "GoTradie"`},
		{name: "remote", integrations: `[{
			"id":"bank1","bank_account_name":"GoTradie","integration_type":"YODLEE"
		}]`, wantError: "remote-backed"},
		{name: "sync enabled", integrations: `[{
			"id":"bank1","bank_account_name":"GoTradie","auto_sync":true
		}]`, wantError: "auto sync enabled"},
		{name: "manual", integrations: `[{
			"id":"bank1","bank_account_name":"GoTradie","integration_type":"","auto_sync":false
		}]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nonGET int
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method != http.MethodGet {
					nonGET++
					http.Error(w, "unexpected mutation", http.StatusInternalServerError)
					return
				}
				switch r.URL.Path {
				case "/api/v1/bank_integrations":
					if r.URL.Query().Get("with_trashed") != "true" {
						t.Errorf("with_trashed = %q", r.URL.Query().Get("with_trashed"))
					}
					_, _ = fmt.Fprintf(w, `{"data":%s,"meta":{"pagination":{"total_pages":1}}}`, tt.integrations)
				case "/api/v1/bank_transactions", "/api/v1/bank_transaction_rules":
					_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
				default:
					http.Error(w, r.URL.String(), http.StatusNotFound)
				}
			}))
			defer ts.Close()

			client, err := invoiceninja.New("token", invoiceninja.WithBaseURL(ts.URL), invoiceninja.WithHTTPClient(ts.Client()))
			if err != nil {
				t.Fatal(err)
			}
			state := &expenseImportState{
				transactionByMarker:         make(map[string]invoiceninja.BankTransaction),
				ambiguousTransactionMarkers: make(map[string]bool),
			}
			err = (&Service{client: client}).loadSupplierSettlementState(context.Background(), state)
			if tt.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
				if state.bankIntegration == nil || state.bankIntegration.ID != "bank1" {
					t.Fatalf("resolved account = %#v", state.bankIntegration)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v; want containing %q", err, tt.wantError)
			}
			if nonGET != 0 {
				t.Fatalf("account resolution made %d mutations", nonGET)
			}
		})
	}
}

func TestSettlementAccountResolutionIsLimitedToAccountPaymentImports(t *testing.T) {
	idx := headerIndex([]string{"Document Type", "Payment Type"})
	if expenseCSVNeedsSettlement([][]string{{"Invoice", ""}}, idx) {
		t.Fatal("an unpaid purchase alone must not require the GoTradie bank account")
	}
	if !expenseCSVNeedsSettlement([][]string{{"Account Payment", "Visa Card"}}, idx) {
		t.Fatal("an Account Payment must require the GoTradie bank account")
	}
}

func TestLoadSupplierSettlementStateRejectsDuplicateMarkedTransactions(t *testing.T) {
	marker := "[GoTradie account-payment:v2:duplicate]"
	description := "GoTradie historical supplier settlement\n" + supplierAccountMarker("BlueCarve") + "\n" + marker
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/bank_integrations":
			_, _ = w.Write([]byte(`{"data":[{"id":"bank1","bank_account_name":"GoTradie"}],"meta":{"pagination":{"total_pages":1}}}`))
		case "/api/v1/bank_transactions":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "transaction1", "description": description},
					{"id": "transaction2", "description": description},
				},
				"meta": map[string]any{"pagination": map[string]any{"total_pages": 1}},
			})
		default:
			http.Error(w, r.URL.String(), http.StatusNotFound)
		}
	}))
	defer ts.Close()

	client, err := invoiceninja.New("token", invoiceninja.WithBaseURL(ts.URL), invoiceninja.WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatal(err)
	}
	state := &expenseImportState{
		transactionByMarker:         make(map[string]invoiceninja.BankTransaction),
		ambiguousTransactionMarkers: make(map[string]bool),
	}
	err = (&Service{client: client}).loadSupplierSettlementState(context.Background(), state)
	if err == nil || !strings.Contains(err.Error(), "multiple Invoice Ninja bank transactions") {
		t.Fatalf("duplicate marker error = %v", err)
	}
}

func TestAccountPaymentAutoConvertDebitRuleFailsBeforeWrites(t *testing.T) {
	var mutationCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if serveExpenseImportReferenceData(w, r) {
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_integrations":
			_, _ = w.Write([]byte(`{"data":[{"id":"bank1","bank_account_name":"GoTradie"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_transactions":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_transaction_rules":
			_, _ = w.Write([]byte(`{"data":[{"id":"rule1","name":"Auto expenses","auto_convert":true,"applies_to":"DEBIT"}],"meta":{"pagination":{"total_pages":1}}}`))
		default:
			mutationCount++
			http.Error(w, r.Method+" "+r.URL.String(), http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	service := newExpenseImportTestService(t, ts)
	_, err := service.ImportExpensesCSV(context.Background(), strings.NewReader(testAccountPaymentCSV()), false, "")
	if err == nil || !strings.Contains(err.Error(), "auto-convert DEBIT rule") {
		t.Fatalf("preflight error = %v", err)
	}
	if mutationCount != 0 {
		t.Fatalf("writes occurred before failed bank-rule preflight: %d", mutationCount)
	}
}

func TestSupplierSettlementNotificationPreflightFailsBeforeWrites(t *testing.T) {
	var mutationCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/vendors":
			_, _ = w.Write([]byte(`{"data":[{"id":"vendor1","name":"Bunnings - Dural"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/expense_categories":
			_, _ = w.Write([]byte(`{"data":[{"id":"category1","name":"Materials"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/projects" || r.URL.Path == "/api/v1/expenses" || r.URL.Path == "/api/v1/quotes"):
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/statics":
			_, _ = w.Write([]byte(`{"payment_types":[{"id":"5","name":"Visa Card"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_integrations":
			_, _ = w.Write([]byte(`{"data":[{"id":"bank1","bank_account_name":"GoTradie"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/bank_transactions" || r.URL.Path == "/api/v1/bank_transaction_rules"):
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/companies/current":
			_, _ = w.Write([]byte(`{"data":{"id":"company1","notify_vendor_when_paid":true}}`))
		default:
			mutationCount++
			http.Error(w, r.Method+" "+r.URL.String(), http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	csv := strings.Join([]string{
		"Date,Supplier,Store,Document Type,Payment Type,Tax Treatment,Category,Option,Total Inc GST,Business %,Business Amount,Business GST,Invoice Number",
		"1/10/2026,Bunnings,Dural,Invoice,,Expense - Materials,Materials,Consumables,71.84,100,71.84,6.53,INV-1",
		"1/11/2026,Bunnings,Dural,Account Payment,Visa Card,formula,formula,formula,71.84,formula,formula,formula,PAY-1",
	}, "\n")
	_, err := newExpenseImportTestService(t, ts).ImportExpensesCSV(context.Background(), strings.NewReader(csv), false, "")
	if err == nil || !strings.Contains(err.Error(), "notify_vendor_when_paid") {
		t.Fatalf("notification preflight error = %v", err)
	}
	if mutationCount != 0 {
		t.Fatalf("writes occurred before failed notification preflight: %d", mutationCount)
	}
}

func TestAccountPaymentPreviewMakesNoWrites(t *testing.T) {
	var mutationCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if serveExpenseImportReferenceData(w, r) {
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_integrations":
			_, _ = w.Write([]byte(`{"data":[{"id":"bank1","bank_account_name":"GoTradie"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/bank_transactions" || r.URL.Path == "/api/v1/bank_transaction_rules"):
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		default:
			mutationCount++
			http.Error(w, r.Method+" "+r.URL.String(), http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	results, err := newExpenseImportTestService(t, ts).ImportExpensesCSV(context.Background(), strings.NewReader(testAccountPaymentCSV()), true, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "would-create-transaction" {
		t.Fatalf("preview results = %#v", results)
	}
	for _, want := range []string{"transaction:create:DEBIT", "bank-account:GoTradie", "unapplied:71.84", "supporting-document:unattached:payment.pdf"} {
		if !containsChange(results[0].Changes, want) {
			t.Fatalf("preview changes %v do not contain %q", results[0].Changes, want)
		}
	}
	if mutationCount != 0 {
		t.Fatalf("preview made %d mutations", mutationCount)
	}
}

func TestAccountPaymentTransactionIsIdempotentAcrossImports(t *testing.T) {
	var created *invoiceninja.BankTransaction
	var createCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if serveExpenseImportReferenceData(w, r) {
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_integrations":
			_, _ = w.Write([]byte(`{"data":[{"id":"bank1","bank_account_name":"GoTradie"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_transactions":
			transactions := []invoiceninja.BankTransaction(nil)
			if created != nil {
				transactions = append(transactions, *created)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": transactions,
				"meta": map[string]any{"pagination": map[string]any{"total_pages": 1}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bank_transaction_rules":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/bank_transactions":
			createCount++
			var request invoiceninja.CreateBankTransactionRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			created = &invoiceninja.BankTransaction{
				Entity:            invoiceninja.Entity{ID: "transaction1"},
				BankIntegrationID: request.BankIntegrationID,
				Amount:            request.Amount,
				BaseType:          request.BaseType,
				Date:              request.Date,
				Description:       request.Description,
				ParticipantName:   request.ParticipantName,
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": created})
		default:
			http.Error(w, r.Method+" "+r.URL.String(), http.StatusNotFound)
		}
	}))
	defer ts.Close()

	service := newExpenseImportTestService(t, ts)
	for run := 1; run <= 2; run++ {
		results, err := service.ImportExpensesCSV(context.Background(), strings.NewReader(testAccountPaymentCSV()), false, "")
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		wantAction := "created-transaction"
		if run == 2 {
			wantAction = "unchanged"
		}
		if len(results) != 1 || results[0].Action != wantAction || results[0].ID != "transaction1" {
			t.Fatalf("run %d results = %#v", run, results)
		}
	}
	if createCount != 1 {
		t.Fatalf("transaction creates = %d; want 1", createCount)
	}
}

func TestAllocateBlueCarveCanonicalPaymentsInIntegerCents(t *testing.T) {
	purchaseOne := settlementTestPurchase("BlueCarve", "2026-01-01", "purchase-7014", 701400)
	purchaseTwo := settlementTestPurchase("BlueCarve", "2026-01-02", "purchase-380", 38000)
	paymentOne := settlementTestPayment("BlueCarve", "2026-02-01", "payment-2000", 200000)
	paymentTwo := settlementTestPayment("BlueCarve", "2026-02-02", "payment-5000", 500000)
	paymentThree := settlementTestPayment("BlueCarve", "2026-02-03", "payment-394", 39400)

	mustAllocateAccountPayments(t, &expenseImportState{}, []*preparedExpenseImportRow{
		paymentThree, purchaseTwo, paymentOne, purchaseOne, paymentTwo,
	})

	if purchaseOne.remainingCents != 0 || purchaseTwo.remainingCents != 0 {
		t.Fatalf("remaining cents: first=%d second=%d", purchaseOne.remainingCents, purchaseTwo.remainingCents)
	}
	if purchaseOne.desiredPaymentDate != "2026-02-03" || purchaseTwo.desiredPaymentDate != "2026-02-03" {
		t.Fatalf("paid dates: first=%q second=%q", purchaseOne.desiredPaymentDate, purchaseTwo.desiredPaymentDate)
	}
	if len(paymentThree.allocations) != 2 || moneyCents(paymentThree.allocations[0].Amount) != 1400 || moneyCents(paymentThree.allocations[1].Amount) != 38000 {
		t.Fatalf("final payment allocations = %#v", paymentThree.allocations)
	}
	if paymentOne.unappliedCents != 0 || paymentTwo.unappliedCents != 0 || paymentThree.unappliedCents != 0 {
		t.Fatalf("unapplied cents: %d, %d, %d", paymentOne.unappliedCents, paymentTwo.unappliedCents, paymentThree.unappliedCents)
	}
}

func TestAllocateIgnoresUnmarkedBankTransactions(t *testing.T) {
	supplierMarker := supplierAccountMarker("BlueCarve")
	markedPayment := "[GoTradie account-payment:v2:marked]"
	state := &expenseImportState{
		paymentTypes: map[string]invoiceninja.PaymentType{"Visa Card": {ID: "5", Name: "Visa Card"}},
		bankTransactions: []invoiceninja.BankTransaction{
			{Entity: invoiceninja.Entity{ID: "unmarked"}, Amount: 999, BaseType: "DEBIT", Date: "2026-02-01", Description: "BlueCarve"},
			{Entity: invoiceninja.Entity{ID: "marked"}, Amount: 10, BaseType: "DEBIT", Date: "2026-02-02", Description: "Payment type: Visa Card\n" + supplierMarker + "\n" + markedPayment},
		},
	}
	purchase := settlementTestPurchase("BlueCarve", "2026-01-01", "purchase", 2000)
	mustAllocateAccountPayments(t, state, []*preparedExpenseImportRow{purchase})
	if purchase.remainingCents != 1000 || len(purchase.allocations) != 1 || purchase.allocations[0].PaymentSourceID != markedPayment {
		t.Fatalf("purchase allocation = %#v", purchase)
	}
}

func TestImmediatelyPaidPurchaseRemovesSupplierSettlementMarkers(t *testing.T) {
	sourceMarker := "[GoTradie source:v2:source]"
	notes := strings.Join([]string{
		"Source notes: keep me",
		sourceMarker,
		"GoTradie supplier account: BlueCarve",
		supplierAccountMarker("BlueCarve"),
		settlementPurchaseMarker(sourceMarker),
	}, "\n")
	row := &preparedExpenseImportRow{documentType: documentTypeInvoice, paymentType: "Visa Card"}
	got := expenseNotesForRow(notes, row)
	if !strings.Contains(got, "Source notes: keep me") || !strings.Contains(got, sourceMarker) {
		t.Fatalf("purchase notes were lost: %q", got)
	}
	if strings.Contains(got, "GoTradie supplier account:") || supplierAccountMarkerFromText(got) != "" || settlementPurchaseMarkerFromText(got) != "" {
		t.Fatalf("supplier settlement markers remain: %q", got)
	}
}

func serveExpenseImportReferenceData(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	switch r.URL.Path {
	case "/api/v1/vendors", "/api/v1/expense_categories", "/api/v1/projects", "/api/v1/expenses", "/api/v1/quotes":
		_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		return true
	case "/api/v1/statics":
		_, _ = w.Write([]byte(`{"payment_types":[{"id":"5","name":"Visa Card","gateway_type_id":1}]}`))
		return true
	default:
		return false
	}
}

func newExpenseImportTestService(t *testing.T, ts *httptest.Server) *Service {
	t.Helper()
	client, err := invoiceninja.New("token", invoiceninja.WithBaseURL(ts.URL), invoiceninja.WithHTTPClient(ts.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return &Service{client: client}
}

func testAccountPaymentCSV() string {
	return strings.Join([]string{
		"Date,Supplier,Store,Document Type,Payment Type,Tax Treatment,Category,Option,Total Inc GST,Business %,Business Amount,Business GST,Invoice Number,File Name",
		"1/11/2026,Bunnings,Dural,Account Payment,Visa Card,formula,formula,formula,71.84,formula,formula,formula,PAY-1,payment.pdf",
	}, "\n")
}

func settlementTestPurchase(supplier, date, marker string, cents int64) *preparedExpenseImportRow {
	return &preparedExpenseImportRow{
		documentType: documentTypeInvoice,
		supplier:     supplier,
		date:         date,
		sourceMarker: marker,
		grossCents:   cents,
	}
}

func settlementTestPayment(supplier, date, marker string, cents int64) *preparedExpenseImportRow {
	return &preparedExpenseImportRow{
		documentType:  documentTypeAccountPayment,
		supplier:      supplier,
		date:          date,
		sourceMarker:  marker,
		grossCents:    cents,
		paymentType:   "Visa Card",
		paymentTypeID: "5",
	}
}
