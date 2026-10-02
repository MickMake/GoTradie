package ninja

import (
	"encoding/csv"
	"strings"
	"testing"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/config"
)

func TestBuildERPNextExportProducesImportFiles(t *testing.T) {
	cfg := testERPNextConfig()
	clients := []invoiceninja.ClientEntity{{
		Entity:       invoiceninja.Entity{ID: "client-1"},
		Name:         "Example Client",
		Address1:     "1 Billing St",
		City:         "Sydney",
		State:        "NSW",
		PostalCode:   "2000",
		CountryID:    "36",
		Phone:        "0400000000",
		Contacts:     []invoiceninja.Contact{{ID: "contact-1", FirstName: "Ada", LastName: "Lovelace", Email: "Ada@EXAMPLE.COM", IsPrimary: true}},
		Settings:     invoiceninja.ClientSettings{CurrencyID: "4"},
		PrivateNotes: "<b>Important</b>",
	}}
	products := []invoiceninja.Product{{Entity: invoiceninja.Entity{ID: "product-1"}, ProductKey: "ITEM-1", Notes: "Example item", Price: 12.5}}
	quotes := []invoiceninja.Quote{invoiceninja.Quote(invoiceninja.Invoice{
		Entity: invoiceninja.Entity{ID: "quote-1"}, ClientID: "client-1", Number: "Q-1", StatusID: "2", Date: "2026-01-01", DueDate: "2026-01-31",
		LineItems: []invoiceninja.LineItem{{ProductKey: "ITEM-1", Notes: "Quoted work", Cost: 12.5, Quantity: 2}},
	})}
	invoices := []invoiceninja.Invoice{{
		Entity: invoiceninja.Entity{ID: "invoice-1"}, ClientID: "client-1", ClientContactID: "contact-1", Client: &clients[0], Number: "INV-1", StatusID: "4", Date: "2026-02-01", DueDate: "2026-02-14", TotalTaxes: 2.5,
		LineItems: []invoiceninja.LineItem{{ProductKey: "ITEM-1", Notes: "Completed work", Cost: 12.5, Quantity: 2, TaxRate1: 10}},
	}}
	payments := []invoiceninja.Payment{{
		Entity: invoiceninja.Entity{ID: "payment-1"}, ClientID: "client-1", Number: "PAY-1", Date: "2026-02-10", Amount: 27.5,
		Paymentables: invoiceninja.Paymentables{{InvoiceID: "invoice-1", Amount: invoiceninja.FlexibleFloat(27.5)}},
	}}

	export, err := buildERPNextExport(cfg, clients, products, quotes, invoices, payments)
	if err != nil {
		t.Fatal(err)
	}
	if len(export.Files) != 8 {
		t.Fatalf("got %d files; want 8", len(export.Files))
	}
	files := map[string]string{}
	for _, file := range export.Files {
		files[file.Name] = string(file.Data)
		if _, err := csv.NewReader(strings.NewReader(string(file.Data))).ReadAll(); err != nil {
			t.Fatalf("%s is invalid CSV: %v", file.Name, err)
		}
	}
	assertContains(t, files["Customer.csv"], "IN-CUST-client-1")
	assertContains(t, files["Address.csv"], "IN-ADDR-B-client-1")
	assertContains(t, files["Contact.csv"], "Ada@example.com")
	assertContains(t, files["Item.csv"], "ITEM-1")
	assertContains(t, files["Quotation.csv"], "IN-Q-quote-1")
	assertContains(t, files["Sales Invoice.csv"], "IN-SINV-invoice-1")
	assertContains(t, files["Payment Entry.csv"], "IN-PAY-payment-1")
	assertContains(t, files["Payment Entry.csv"], "27.5")
}

func TestBuildERPNextExportReportsUnsafePayment(t *testing.T) {
	cfg := testERPNextConfig()
	clients := []invoiceninja.ClientEntity{{Entity: invoiceninja.Entity{ID: "client-1"}, Name: "Example Client"}}
	payments := []invoiceninja.Payment{{Entity: invoiceninja.Entity{ID: "payment-1"}, ClientID: "client-1", Number: "PAY-1", Date: "2026-02-10", Amount: 10, Refunded: 10}}

	export, err := buildERPNextExport(cfg, clients, nil, nil, nil, payments)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, file := range export.Files {
		files[file.Name] = string(file.Data)
	}
	if strings.Contains(files["Payment Entry.csv"], "IN-PAY-payment-1") {
		t.Fatal("refunded payment was included in Payment Entry.csv")
	}
	assertContains(t, files["Migration Report.csv"], "refund or credit allocation")
}

func testERPNextConfig() config.Config {
	return config.Config{
		ERPNextCompany:           "Example Company",
		ERPNextCustomerGroup:     "Commercial",
		ERPNextTerritory:         "Australia",
		ERPNextItemGroup:         "Services",
		ERPNextUOM:               "Nos",
		ERPNextSellingPriceList:  "Standard Selling",
		ERPNextCurrency:          "AUD",
		ERPNextCountry:           "Australia",
		ERPNextIncomeAccount:     "Sales - EX",
		ERPNextReceivableAccount: "Debtors - EX",
		ERPNextBankAccount:       "Bank - EX",
		ERPNextModeOfPayment:     "Bank Transfer",
		ERPNextTaxTemplate:       "GST 10% - EX",
	}
}

func assertContains(t *testing.T, value, substring string) {
	t.Helper()
	if !strings.Contains(value, substring) {
		t.Fatalf("expected %q to contain %q", value, substring)
	}
}
