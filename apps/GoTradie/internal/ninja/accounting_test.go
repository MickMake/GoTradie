package ninja

import (
	"fmt"
	"testing"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/accounting"
)

func TestBuildSalesFactsPreservesPartialInvoiceAllocations(t *testing.T) {
	invoices := []invoiceninja.Invoice{{
		Entity: invoiceninja.Entity{ID: "invoice-1"}, Number: "INV-1", ClientID: "client-1",
		Date: "2026-07-01", Amount: 110, TotalTaxes: 10, TaxName1: "GST",
	}}
	payments := []invoiceninja.Payment{{
		Entity: invoiceninja.Entity{ID: "payment-1"}, Date: "2026-09-30",
		Paymentables: invoiceninja.Paymentables{{InvoiceID: "invoice-1", Amount: 55}},
	}}
	sales, exceptions := buildSalesFacts(invoices, payments, nil)
	if len(exceptions) != 0 || len(sales) != 1 || len(sales[0].Payments) != 1 {
		t.Fatalf("sales=%#v exceptions=%#v", sales, exceptions)
	}
	if sales[0].Payments[0].AmountCents != 5500 || sales[0].Payments[0].SourceID != "payment-1" {
		t.Fatalf("allocation = %#v", sales[0].Payments[0])
	}
}

func TestBuildSalesFactsExcludesDraftsAndCreditOnlyPaymentables(t *testing.T) {
	invoices := []invoiceninja.Invoice{
		{Entity: invoiceninja.Entity{ID: "draft"}, StatusID: "1", Date: "2026-07-01", Amount: 110, TotalTaxes: 10, TaxName1: "GST"},
		{Entity: invoiceninja.Entity{ID: "sent"}, StatusID: "2", Date: "2026-07-01", Amount: 110, TotalTaxes: 10, TaxName1: "GST"},
	}
	payments := []invoiceninja.Payment{{
		Entity: invoiceninja.Entity{ID: "payment-1"}, InvoiceID: "sent", Date: "2026-09-30", Amount: 55,
		Paymentables: invoiceninja.Paymentables{{CreditID: "credit-1", Amount: 5}},
	}}
	sales, exceptions := buildSalesFacts(invoices, payments, nil)
	if len(exceptions) != 0 || len(sales) != 1 || sales[0].SourceID != "sent" {
		t.Fatalf("sales=%#v exceptions=%#v", sales, exceptions)
	}
	if len(sales[0].Payments) != 1 || sales[0].Payments[0].AmountCents != 5500 {
		t.Fatalf("payments = %#v", sales[0].Payments)
	}
}

func TestBuildPurchaseFactsReusesFIFOSettlementAllocation(t *testing.T) {
	supplier := supplierAccountMarker("Bunnings")
	expenses := []invoiceninja.Expense{
		accountExpense("expense-1", "purchase-1", supplier, "2026-07-01", 80, 8, 100),
		accountExpense("expense-2", "purchase-2", supplier, "2026-07-02", 50, 5, 100),
	}
	transaction := invoiceninja.BankTransaction{
		Entity: invoiceninja.Entity{ID: "transaction-1"}, Amount: 150, BaseType: "DEBIT", Date: "2026-09-30",
		Description: supplier + "\n[GoTradie import-id:v1:payment-1]",
	}
	purchases, exceptions := buildPurchaseFacts(expenses, []invoiceninja.BankTransaction{transaction}, nil)
	if len(exceptions) != 0 {
		t.Fatalf("exceptions = %#v", exceptions)
	}
	if len(purchases) != 2 || len(purchases[0].Settlements) != 1 || len(purchases[1].Settlements) != 1 {
		t.Fatalf("purchases = %#v", purchases)
	}
	if purchases[0].Settlements[0].AmountCents != 10000 || purchases[1].Settlements[0].AmountCents != 5000 {
		t.Fatalf("FIFO allocations = %#v / %#v", purchases[0].Settlements, purchases[1].Settlements)
	}
	dataset := accounting.Dataset{Purchases: purchases}
	events, eventExceptions := dataset.GSTEvents("cash")
	if len(eventExceptions) != 0 || len(events) != 2 {
		t.Fatalf("events=%#v exceptions=%#v", events, eventExceptions)
	}
	if events[0].Amounts.GrossCents != 8000 || events[0].Amounts.GSTCents != 800 || events[1].Amounts.GrossCents != 2500 || events[1].Amounts.GSTCents != 250 {
		t.Fatalf("proportional purchase events = %#v", events)
	}
}

func TestBuildPurchaseFactsReportsUnappliedSettlement(t *testing.T) {
	supplier := supplierAccountMarker("Bunnings")
	expense := accountExpense("expense-1", "purchase-1", supplier, "2026-07-01", 80, 8, 100)
	transaction := invoiceninja.BankTransaction{
		Entity: invoiceninja.Entity{ID: "transaction-1"}, Amount: 120, BaseType: "DEBIT", Date: "2026-09-30",
		Description: supplier + "\n[GoTradie import-id:v1:payment-1]",
	}
	_, exceptions := buildPurchaseFacts([]invoiceninja.Expense{expense}, []invoiceninja.BankTransaction{transaction}, nil)
	if len(exceptions) != 1 || exceptions[0].SourceID != "transaction-1" {
		t.Fatalf("exceptions = %#v", exceptions)
	}
}

func TestBuildPurchaseFactsReportsAmbiguousAndArchivedSettlementEvidence(t *testing.T) {
	supplier := supplierAccountMarker("Bunnings")
	first := accountExpense("expense-1", "purchase-1", supplier, "2026-07-01", 80, 8, 100)
	second := accountExpense("expense-2", "purchase-1", supplier, "2026-07-02", 50, 5, 100)
	archived := accountExpense("expense-3", "purchase-3", supplier, "2025-07-01", 50, 5, 100)
	archived.ArchivedAt = 1
	_, exceptions := buildPurchaseFacts([]invoiceninja.Expense{first, second, archived}, nil, nil)
	if len(exceptions) != 3 {
		t.Fatalf("exceptions = %#v", exceptions)
	}
	for _, exception := range exceptions {
		if exception.Severity != accounting.SeverityError || exception.Date != "" {
			t.Fatalf("settlement exception should be globally blocking: %#v", exception)
		}
	}
}

func accountExpense(id, purchaseID, supplier, date string, amount, gst, sourceTotal float64) invoiceninja.Expense {
	return invoiceninja.Expense{
		Entity: invoiceninja.Entity{ID: id}, VendorID: "vendor-1", Date: date, Amount: amount,
		TaxAmount1: gst, TaxName1: "GST", CustomValue3: "80", CurrencyID: "1",
		PrivateNotes: fmt.Sprintf("Source total inc GST: %.2f\n%s\n[GoTradie supplier-purchase:v1:%s]", sourceTotal, supplier, purchaseID),
	}
}
