package financial

import (
	"bytes"
	"testing"
	"time"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/accounting"
	"github.com/xuri/excelize/v2"
)

func TestBuildFiltersEachTransactionByItsOwnSourceDate(t *testing.T) {
	data := SourceData{
		Source: "Invoice Ninja",
		Invoices: []invoiceninja.Invoice{
			{Entity: entity("invoice-june"), Date: "2026-06-28"},
			{Entity: entity("invoice-july"), Date: "2026-07-01"},
			{Entity: entity("invoice-missing")},
		},
		Payments: []invoiceninja.Payment{
			{Entity: entity("payment-july"), InvoiceID: "invoice-june", Date: "2026-07-05"},
			{Entity: entity("payment-august"), Date: "2026-08-01"},
		},
		Expenses: []invoiceninja.Expense{
			{Entity: entity("expense-july"), Date: "2026-07-31"},
			{Entity: entity("expense-june"), Date: "2026-06-30"},
		},
		SupplierTransactions: []invoiceninja.BankTransaction{
			{Entity: entity("transaction-july"), Date: "2026-07-15"},
			{Entity: entity("transaction-invalid"), Date: "not-a-date"},
		},
		Customers: []invoiceninja.ClientEntity{{Entity: entity("customer-unrelated")}},
		Vendors:   []invoiceninja.Vendor{{Entity: entity("vendor-unrelated")}},
		Products:  []invoiceninja.Product{{Entity: entity("product-unrelated")}},
		Projects:  []invoiceninja.Project{{Entity: entity("project-unrelated")}},
		Accounting: accounting.Dataset{
			Sales: []accounting.Sale{
				{SourceID: "invoice-june"}, {SourceID: "invoice-july"}, {SourceID: "invoice-missing"},
			},
			Purchases: []accounting.Purchase{{SourceID: "expense-july"}, {SourceID: "expense-june"}},
			Exceptions: []accounting.Exception{
				{Severity: accounting.SeverityWarning, SourceType: "invoice", SourceID: "invoice-june", Message: "outside range"},
				{Severity: accounting.SeverityWarning, SourceType: "payment", SourceID: "payment-july", Message: "selected relationship"},
			},
		},
	}

	report, err := Build(data, quarterlyConfig(), Options{From: "2026-07-01", To: "2026-07-31"}, mustDate(t, "2026-10-09"))
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, invoiceIDs(report.Invoices), "invoice-missing", "invoice-july")
	assertIDs(t, paymentIDs(report.Payments), "payment-july")
	assertIDs(t, expenseIDs(report.Expenses), "expense-july")
	assertIDs(t, transactionIDs(report.SupplierTransactions), "transaction-july", "transaction-invalid")
	assertIDs(t, saleIDs(report.SalesFacts), "invoice-july", "invoice-missing")
	assertIDs(t, purchaseIDs(report.PurchaseFacts), "expense-july")
	if len(report.Customers) != 1 || len(report.Vendors) != 1 || len(report.Products) != 1 || len(report.Projects) != 1 {
		t.Fatalf("master data was date-filtered: customers=%d vendors=%d products=%d projects=%d", len(report.Customers), len(report.Vendors), len(report.Products), len(report.Projects))
	}
	if report.Status != "INCOMPLETE" || !hasException(report.Exceptions, "invoice", "invoice-missing", accounting.SeverityError) || !hasException(report.Exceptions, "bank_transaction", "transaction-invalid", accounting.SeverityError) {
		t.Fatalf("status=%q exceptions=%#v", report.Status, report.Exceptions)
	}
	if hasException(report.Exceptions, "invoice", "invoice-june", accounting.SeverityWarning) || !hasException(report.Exceptions, "payment", "payment-july", accounting.SeverityWarning) {
		t.Fatalf("accounting exception scope=%#v", report.Exceptions)
	}
}

func TestBuildUnrestrictedIncludesMissingDatesWithoutInventingErrors(t *testing.T) {
	data := SourceData{
		Invoices:             []invoiceninja.Invoice{{Entity: entity("invoice-1")}},
		Payments:             []invoiceninja.Payment{{Entity: entity("payment-1")}},
		Expenses:             []invoiceninja.Expense{{Entity: entity("expense-1")}},
		SupplierTransactions: []invoiceninja.BankTransaction{{Entity: entity("transaction-1")}},
	}
	report, err := Build(data, quarterlyConfig(), Options{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "COMPLETE" || len(report.Exceptions) != 0 {
		t.Fatalf("status=%q exceptions=%#v", report.Status, report.Exceptions)
	}
	if len(report.Invoices) != 1 || len(report.Payments) != 1 || len(report.Expenses) != 1 || len(report.SupplierTransactions) != 1 {
		t.Fatalf("unrestricted rows were omitted: %#v", report)
	}
}

func TestWorkbookPreservesRawRelationshipsAndStatus(t *testing.T) {
	report := Report{
		GeneratedAt:       time.Date(2026, time.October, 9, 12, 0, 0, 0, time.Local),
		Source:            "Invoice Ninja",
		CompanyCurrencyID: "1",
		Status:            "COMPLETE",
		Invoices: []invoiceninja.Invoice{{
			Entity: entity("invoice-1"), Number: "INV-1", Date: "2026-07-01", ProjectID: "project-1",
			TaxName1: "GST", TaxRate1: 10, IsAmountDiscount: true,
			Client: &invoiceninja.ClientEntity{Settings: invoiceninja.ClientSettings{CurrencyID: "1"}},
			LineItems: []invoiceninja.LineItem{{
				ProductKey: "BUNNINGS-123", Notes: "Timber", Quantity: 2, Cost: 12.5,
				TaxName1: "GST", TaxRate1: 10, IsAmountDiscount: true,
			}},
		}},
		Payments: []invoiceninja.Payment{{
			Entity: entity("payment-1"), Number: "PAY-1", Date: "2026-07-05", Amount: 25, Applied: 25,
			Paymentables: invoiceninja.Paymentables{{InvoiceID: "invoice-1", Amount: 25}},
			Invoices:     []invoiceninja.Invoice{{Entity: entity("invoice-1"), Number: "INV-1"}},
		}},
		Expenses:             []invoiceninja.Expense{{Entity: entity("expense-1"), Date: "2026-07-02", Amount: 11}},
		SupplierTransactions: []invoiceninja.BankTransaction{{Entity: entity("transaction-1"), Date: "2026-07-03", ExpenseID: "expense-1"}},
		Customers:            []invoiceninja.ClientEntity{{Entity: entity("customer-1"), Name: "Arthur Dent"}},
		Vendors:              []invoiceninja.Vendor{{Entity: entity("vendor-1"), Name: "Mostly Harmless Supplies"}},
		Products:             []invoiceninja.Product{{Entity: entity("product-1"), ProductKey: "BUNNINGS-123"}},
		Projects:             []invoiceninja.Project{{Entity: entity("project-1"), Name: "Bypass"}},
	}
	data, err := Workbook(report)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = book.Close() }()
	wantSheets := []string{"Summary", "Income", "Expenses", "Payments", "Supplier Transactions", "Customers", "Vendors", "Products", "Projects-Jobs", "Accounting Facts", "Exceptions"}
	if got := book.GetSheetList(); !equalStrings(got, wantSheets) {
		t.Fatalf("sheets=%#v; want %#v", got, wantSheets)
	}
	assertCell(t, book, "Summary", "B7", "COMPLETE")
	assertCell(t, book, "Summary", "B8", "1")
	assertCell(t, book, "Income", "U2", "BUNNINGS-123")
	assertCell(t, book, "Income", "AE2", "1")
	assertCell(t, book, "Income", "AG2", "GST")
	assertCell(t, book, "Income", "AM2", "GST")
	assertCell(t, book, "Income", "AS2", "TRUE")
	assertCell(t, book, "Income", "AT2", "TRUE")
	assertCell(t, book, "Payments", "S2", "invoice-1")
	assertCell(t, book, "Payments", "T2", "INV-1")
	assertCell(t, book, "Supplier Transactions", "K2", "expense-1")
	assertDifferentStyle(t, book, "Income", "K2", "O2")
	assertDifferentStyle(t, book, "Income", "X2", "Z2")
	assertDifferentStyle(t, book, "Expenses", "M2", "O2")
	assertDifferentStyle(t, book, "Projects-Jobs", "E2", "F2")
}

func TestPaymentRelationshipsDeduplicateAlternateInvoiceShape(t *testing.T) {
	payment := invoiceninja.Payment{
		Paymentables:       invoiceninja.Paymentables{{ID: "paymentable-1", InvoiceID: "invoice-1", Amount: 25}},
		InvoiceAllocations: invoiceninja.Paymentables{{InvoiceID: "invoice-1", Amount: 25}},
	}
	relationships := paymentRelationships(payment)
	if len(relationships) != 1 || relationships[0].id != "invoice-1" || relationships[0].paymentableID != "paymentable-1" {
		t.Fatalf("relationships=%#v", relationships)
	}
}

func TestPaymentRelationshipsRetainIncludedInvoicesWithoutAllocations(t *testing.T) {
	payment := invoiceninja.Payment{Invoices: []invoiceninja.Invoice{{Entity: entity("invoice-1"), Number: "INV-1"}}}
	relationships := paymentRelationships(payment)
	if len(relationships) != 1 || relationships[0].id != "invoice-1" || relationships[0].number != "INV-1" || relationships[0].amount != "" {
		t.Fatalf("relationships=%#v", relationships)
	}
}

func entity(id string) invoiceninja.Entity { return invoiceninja.Entity{ID: id} }

func invoiceIDs(rows []invoiceninja.Invoice) []string {
	result := make([]string, len(rows))
	for index := range rows {
		result[index] = rows[index].ID
	}
	return result
}

func paymentIDs(rows []invoiceninja.Payment) []string {
	result := make([]string, len(rows))
	for index := range rows {
		result[index] = rows[index].ID
	}
	return result
}

func expenseIDs(rows []invoiceninja.Expense) []string {
	result := make([]string, len(rows))
	for index := range rows {
		result[index] = rows[index].ID
	}
	return result
}

func transactionIDs(rows []invoiceninja.BankTransaction) []string {
	result := make([]string, len(rows))
	for index := range rows {
		result[index] = rows[index].ID
	}
	return result
}

func saleIDs(rows []accounting.Sale) []string {
	result := make([]string, len(rows))
	for index := range rows {
		result[index] = rows[index].SourceID
	}
	return result
}

func purchaseIDs(rows []accounting.Purchase) []string {
	result := make([]string, len(rows))
	for index := range rows {
		result[index] = rows[index].SourceID
	}
	return result
}

func assertIDs(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !equalStrings(got, want) {
		t.Fatalf("ids=%#v; want %#v", got, want)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func hasException(rows []accounting.Exception, sourceType, sourceID string, severity accounting.Severity) bool {
	for _, row := range rows {
		if row.SourceType == sourceType && row.SourceID == sourceID && row.Severity == severity {
			return true
		}
	}
	return false
}

func assertCell(t *testing.T, book *excelize.File, sheet, cell, want string) {
	t.Helper()
	got, err := book.GetCellValue(sheet, cell)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s!%s=%q; want %q", sheet, cell, got, want)
	}
}

func assertDifferentStyle(t *testing.T, book *excelize.File, sheet, first, second string) {
	t.Helper()
	firstStyle, err := book.GetCellStyle(sheet, first)
	if err != nil {
		t.Fatal(err)
	}
	secondStyle, err := book.GetCellStyle(sheet, second)
	if err != nil {
		t.Fatal(err)
	}
	if firstStyle == secondStyle {
		t.Fatalf("%s!%s and %s unexpectedly use the same style %d", sheet, first, second, firstStyle)
	}
}
