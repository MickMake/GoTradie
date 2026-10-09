package bas

import (
	"bytes"
	"testing"
	"time"

	"github.com/MickMake/GoTradie/internal/accounting"
	"github.com/xuri/excelize/v2"
)

func TestBuildCashBASUsesPartialCustomerAndSupplierAllocations(t *testing.T) {
	dataset := accounting.Dataset{
		Source: "Invoice Ninja",
		Sales: []accounting.Sale{{
			SourceID: "invoice-1", Number: "INV-1", Date: "2026-07-01", TaxKnown: true,
			Amounts: accounting.Amounts{GrossCents: 11000, NetCents: 10000, GSTCents: 1000},
			Payments: []accounting.Allocation{
				{SourceType: "payment", SourceID: "payment-1", Date: "2026-09-30", AmountCents: 5500},
				{SourceType: "payment", SourceID: "payment-2", Date: "2026-10-01", AmountCents: 5500},
			},
		}},
		Purchases: []accounting.Purchase{
			{SourceID: "expense-ordinary", Date: "2026-09-01", PaymentDate: "2026-09-01", TaxKnown: true, Amounts: accounting.Amounts{GrossCents: 2200, NetCents: 2000, GSTCents: 200}},
			{SourceID: "expense-account", Date: "2026-08-01", TaxKnown: true, SupplierAccount: true, SettlementBaseCents: 11000, Amounts: accounting.Amounts{GrossCents: 8800, NetCents: 8000, GSTCents: 800}, Settlements: []accounting.Allocation{
				{SourceType: "bank_transaction", SourceID: "transaction-1", Date: "2026-09-30", AmountCents: 5500},
				{SourceType: "bank_transaction", SourceID: "transaction-2", Date: "2026-10-01", AmountCents: 5500},
			}},
		},
	}
	report, err := Build(dataset, quarterlyConfig(), Options{FY: "2027", Period: "1", Now: mustDate(t, "2026-10-09")}, mustDate(t, "2026-10-09"))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Summaries) != 1 {
		t.Fatalf("summaries = %d", len(report.Summaries))
	}
	summary := report.Summaries[0]
	if summary.G1Cents != 5500 || summary.OneACents != 500 || summary.OneBCents != 600 || summary.NetGSTCents != -100 {
		t.Fatalf("summary = %#v", summary)
	}
	if len(report.Sales) != 1 || report.Sales[0].Event.RelatedSourceID != "payment-1" {
		t.Fatalf("sales = %#v", report.Sales)
	}
	if len(report.Purchases) != 2 {
		t.Fatalf("purchases = %#v", report.Purchases)
	}
}

func TestBuildMarksAccountingErrorsIncomplete(t *testing.T) {
	dataset := accounting.Dataset{Source: "Invoice Ninja", Sales: []accounting.Sale{{
		SourceID: "invoice-1", Date: "2026-08-01", Amounts: accounting.Amounts{GrossCents: 11000, NetCents: 11000},
	}}}
	cfg := quarterlyConfig()
	cfg.GSTBasis = "accrual"
	report, err := Build(dataset, cfg, Options{FY: "2027", Period: "1", Now: mustDate(t, "2026-10-09")}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "INCOMPLETE" || len(report.Exceptions) != 1 || report.Exceptions[0].Exception.Message != "missing GST treatment" {
		t.Fatalf("report status/exceptions = %q, %#v", report.Status, report.Exceptions)
	}
}

func TestWorkbookContainsRequiredSheetsTotalsAndTraceability(t *testing.T) {
	dataset := accounting.Dataset{Source: "Invoice Ninja", Sales: []accounting.Sale{{
		SourceID: "invoice-1", Number: "INV-1", Date: "2026-08-01", TaxKnown: true,
		Amounts: accounting.Amounts{GrossCents: 11000, NetCents: 10000, GSTCents: 1000},
	}}}
	cfg := quarterlyConfig()
	cfg.GSTBasis = "accrual"
	report, err := Build(dataset, cfg, Options{FY: "2027", Period: "1", Now: mustDate(t, "2026-10-09")}, mustDate(t, "2026-10-09"))
	if err != nil {
		t.Fatal(err)
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
	wantSheets := []string{"Summary", "Sales", "Purchases", "Exceptions"}
	if got := book.GetSheetList(); len(got) != len(wantSheets) {
		t.Fatalf("sheets = %#v", got)
	} else {
		for index := range wantSheets {
			if got[index] != wantSheets[index] {
				t.Fatalf("sheets = %#v", got)
			}
		}
	}
	if got, _ := book.GetCellValue("Summary", "K2"); got != "110.00" {
		t.Fatalf("G1 = %q; want 110.00", got)
	}
	if got, _ := book.GetCellValue("Sales", "D2"); got != "invoice-1" {
		t.Fatalf("sales source ID = %q", got)
	}
}
