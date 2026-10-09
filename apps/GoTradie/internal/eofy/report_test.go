package eofy

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/MickMake/GoTradie/internal/accounting"
	"github.com/MickMake/GoTradie/internal/config"
	"github.com/xuri/excelize/v2"
)

func TestBuildUsesEOFYAccountingBasisIndependently(t *testing.T) {
	dataset := accounting.Dataset{Source: "Invoice Ninja", Sales: []accounting.Sale{{
		SourceID: "invoice-1", Date: "2025-06-30", TaxKnown: true,
		Amounts:  accounting.Amounts{GrossCents: 11000, NetCents: 10000, GSTCents: 1000},
		Payments: []accounting.Allocation{{SourceType: "payment", SourceID: "payment-1", Date: "2025-07-01", AmountCents: 11000}},
	}}}
	options := Options{FY: "2026", Now: mustDate(t, "2026-07-01")}
	generated := mustDate(t, "2026-07-01")

	cash, err := Build(dataset, eofyConfig("cash"), options, generated)
	if err != nil {
		t.Fatal(err)
	}
	accrual, err := Build(dataset, eofyConfig("accrual"), options, generated)
	if err != nil {
		t.Fatal(err)
	}
	if cash.AccountingBasis != "cash" || len(cash.Income) != 1 || cash.Income[0].RelatedSourceID != "payment-1" {
		t.Fatalf("cash report = %#v", cash)
	}
	if accrual.AccountingBasis != "accrual" || len(accrual.Income) != 0 {
		t.Fatalf("accrual report = %#v", accrual)
	}
}

func TestBuildCapitalReviewAppliesGSTBeforeBusinessUseAndHonoursBoundary(t *testing.T) {
	dataset := accounting.Dataset{Source: "Invoice Ninja", Purchases: []accounting.Purchase{
		assetPurchase("no-credit", "Review", 1900000, 100000, 0, 50),
		assetPurchase("full-credit", "Review >$300", 2100000, 190909, 190909, 100),
		assetPurchase("partial-credit-boundary", "Review", 2050000, 100000, 50000, 10),
	}}
	report, err := Build(dataset, eofyConfig("cash"), Options{FY: "2027"}, mustDate(t, "2027-07-01"))
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "COMPLETE" || len(report.Assets) != 3 || len(report.Exceptions) != 0 {
		t.Fatalf("status=%q assets=%#v exceptions=%#v", report.Status, report.Assets, report.Exceptions)
	}
	byID := make(map[string]AssetRow)
	for _, row := range report.Assets {
		byID[row.SourceID] = row
	}
	noCredit := byID["no-credit"]
	if *noCredit.ThresholdCostCents != 1900000 || *noCredit.BusinessAdjustedCents != 950000 || noCredit.Classification != assetCandidate {
		t.Fatalf("no credit = %#v", noCredit)
	}
	fullCredit := byID["full-credit"]
	if *fullCredit.ThresholdCostCents != 1909091 || fullCredit.CapitalCheck != "Review >$300" || fullCredit.Classification != assetCandidate {
		t.Fatalf("full credit = %#v", fullCredit)
	}
	partial := byID["partial-credit-boundary"]
	if *partial.ThresholdCostCents != 2000000 || *partial.BusinessAdjustedCents != 200000 || partial.Classification != assetReview {
		t.Fatalf("partial credit boundary = %#v", partial)
	}
}

func TestBuildCapitalReviewUsesPurchaseDateAndErrorsOnMissingEvidence(t *testing.T) {
	dataset := accounting.Dataset{Source: "Invoice Ninja", Purchases: []accounting.Purchase{
		{
			SourceID: "missing", Date: "2026-07-01", CapitalCheck: "Review", CategoryID: "cat-1", CategoryName: "Tools",
			PaymentStatus: accounting.PaymentStatusUnpaid,
		},
		assetPurchaseWithDate("other-year", "2025-06-30"),
	}}
	report, err := Build(dataset, eofyConfig("cash"), Options{FY: "2027"}, mustDate(t, "2027-07-01"))
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "INCOMPLETE" || len(report.Assets) != 1 || report.Assets[0].FirstUsedDate != "2026-07-01" || report.Assets[0].Classification != assetIncomplete {
		t.Fatalf("report = %#v", report)
	}
	messages := make([]string, 0, len(report.Exceptions))
	for _, exception := range report.Exceptions {
		if exception.Severity != accounting.SeverityError {
			t.Fatalf("exception = %#v", exception)
		}
		messages = append(messages, exception.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, want := range []string{"missing Source total inc GST", "missing Source GST", "missing GST credit entitlement", "missing business-use percentage"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("exceptions %q do not contain %q", joined, want)
		}
	}
}

func TestBuildPreservesCategoryAndSourceTraceability(t *testing.T) {
	percent := 80.0
	dataset := accounting.Dataset{Source: "Invoice Ninja", Purchases: []accounting.Purchase{{
		SourceID: "expense-1", Number: "BUN-1", VendorID: "vendor-1", VendorName: "Bunnings",
		CategoryID: "cat-1", CategoryName: "Materials", Description: "Timber", CapitalCheck: "No", Date: "2026-08-01",
		PaymentStatus: accounting.PaymentStatusPaid, PaymentDate: "2026-09-01", TaxKnown: true,
		BusinessUsePercent: &percent, Amounts: accounting.Amounts{GrossCents: 8800, NetCents: 8000, GSTCents: 800},
	}}}
	report, err := Build(dataset, eofyConfig("cash"), Options{FY: "2027"}, mustDate(t, "2027-07-01"))
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "COMPLETE" || len(report.Expenses) != 1 || len(report.Supporting) != 1 {
		t.Fatalf("report = %#v", report)
	}
	event := report.Expenses[0]
	if event.SourceID != "expense-1" || event.SourceDate != "2026-08-01" || event.Date != "2026-09-01" || event.CategoryName != "Materials" || event.Description != "Timber" || event.CapitalCheck != "No" {
		t.Fatalf("expense = %#v", event)
	}
}

func TestBuildCapitalReviewRejectsArchivedAndForeignCurrencyEvidence(t *testing.T) {
	archived := assetPurchase("archived", "Review", 100000, 9091, 9091, 100)
	archived.ArchivedOrDeleted = true
	foreign := assetPurchase("foreign", "Review", 100000, 9091, 9091, 100)
	foreign.CurrencyID = "2"
	dataset := accounting.Dataset{Source: "Invoice Ninja", CompanyCurrencyID: "1", Purchases: []accounting.Purchase{archived, foreign}}
	report, err := Build(dataset, eofyConfig("cash"), Options{FY: "2027"}, mustDate(t, "2027-07-01"))
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "INCOMPLETE" || len(report.Assets) != 2 {
		t.Fatalf("report = %#v", report)
	}
	messages := make([]string, 0, len(report.Exceptions))
	for _, exception := range report.Exceptions {
		messages = append(messages, exception.Message)
	}
	joined := strings.Join(messages, "\n")
	if !strings.Contains(joined, "archived or deleted") || !strings.Contains(joined, "unsupported foreign currency") {
		t.Fatalf("exceptions = %q", joined)
	}
	for _, row := range report.Assets {
		if row.Classification != assetIncomplete {
			t.Fatalf("asset = %#v", row)
		}
	}
}

func TestBuildWarningRemainsComplete(t *testing.T) {
	report, err := Build(accounting.Dataset{Source: "Invoice Ninja", Exceptions: []accounting.Exception{{
		Severity: accounting.SeverityWarning, SourceType: "expense", SourceID: "expense-1", Date: "2026-07-01", Message: "review note",
	}}}, eofyConfig("accrual"), Options{FY: "2027"}, mustDate(t, "2027-07-01"))
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "COMPLETE" || len(report.Exceptions) != 1 {
		t.Fatalf("report = %#v", report)
	}
}

func TestWorkbookContainsRequiredSheetsAndEvidence(t *testing.T) {
	dataset := accounting.Dataset{Source: "Invoice Ninja", Purchases: []accounting.Purchase{
		assetPurchase("asset-1", "Review", 1900000, 100000, 0, 50),
	}}
	report, err := Build(dataset, eofyConfig("accrual"), Options{FY: "2027"}, mustDate(t, "2027-07-01"))
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
	wantSheets := []string{"Summary", "Income", "Expenses", "Capital-Asset Review", "GST Reconciliation", "Exceptions", "Supporting Detail"}
	if got := book.GetSheetList(); strings.Join(got, "|") != strings.Join(wantSheets, "|") {
		t.Fatalf("sheets = %#v", got)
	}
	if got, _ := book.GetCellValue("Summary", "B2"); got != "FY2027" {
		t.Fatalf("financial year = %q", got)
	}
	if got, _ := book.GetCellValue("Summary", "B9"); got != "COMPLETE" {
		t.Fatalf("status = %q", got)
	}
	if got, _ := book.GetCellValue("Capital-Asset Review", "N2"); got != "asset-1" {
		t.Fatalf("asset source ID = %q", got)
	}
	if got, _ := book.GetCellValue("Capital-Asset Review", "O2"); got != "Review" {
		t.Fatalf("capital check = %q", got)
	}
}

func eofyConfig(basis string) config.EOFYConfig {
	return config.EOFYConfig{AccountingBasis: basis, InstantAssetWriteoffThreshold: 20000}
}

func assetPurchase(id, capitalCheck string, sourceGross, sourceGST, claimableGST int64, businessUse float64) accounting.Purchase {
	return accounting.Purchase{
		SourceID: id, VendorName: "Supplier", CategoryID: "cat-1", CategoryName: "Tools", Description: "Tool",
		Date: "2026-07-01", CapitalCheck: capitalCheck, SourceGrossCents: centsPointer(sourceGross), SourceGSTCents: centsPointer(sourceGST),
		PaymentStatus: accounting.PaymentStatusUnpaid, TaxKnown: true, BusinessUsePercent: floatPointer(businessUse),
		Amounts: accounting.Amounts{GrossCents: sourceGross - sourceGST, NetCents: sourceGross - sourceGST - claimableGST, GSTCents: claimableGST},
	}
}

func assetPurchaseWithDate(id, date string) accounting.Purchase {
	purchase := assetPurchase(id, "Review", 100000, 9091, 9091, 100)
	purchase.Date = date
	return purchase
}

func centsPointer(value int64) *int64     { return &value }
func floatPointer(value float64) *float64 { return &value }

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatal(err)
	}
	return date
}
