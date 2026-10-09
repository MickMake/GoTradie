package accounting

import (
	"strings"
	"testing"
)

func TestCashAllocationRoundingPreservesSourceTotals(t *testing.T) {
	dataset := Dataset{Sales: []Sale{{
		SourceID: "invoice-1", TaxKnown: true,
		Amounts: Amounts{GrossCents: 100, NetCents: 91, GSTCents: 9},
		Payments: []Allocation{
			{SourceType: "payment", SourceID: "payment-1", Date: "2026-07-01", AmountCents: 33},
			{SourceType: "payment", SourceID: "payment-2", Date: "2026-07-02", AmountCents: 67},
		},
	}}}
	events, exceptions := dataset.GSTEvents("cash")
	if len(exceptions) != 0 {
		t.Fatalf("exceptions = %#v", exceptions)
	}
	if len(events) != 2 {
		t.Fatalf("events = %#v", events)
	}
	var gross, net, gst int64
	for _, event := range events {
		gross += event.Amounts.GrossCents
		net += event.Amounts.NetCents
		gst += event.Amounts.GSTCents
	}
	if gross != 100 || net != 91 || gst != 9 {
		t.Fatalf("allocated totals = gross %d net %d GST %d", gross, net, gst)
	}
}

func TestCashAllocationOverSourceIsIncomplete(t *testing.T) {
	dataset := Dataset{Sales: []Sale{{
		SourceID: "invoice-1", Date: "2026-07-01", TaxKnown: true,
		Amounts:  Amounts{GrossCents: 100, NetCents: 91, GSTCents: 9},
		Payments: []Allocation{{SourceType: "payment", SourceID: "payment-1", Date: "2026-10-01", AmountCents: 101}},
	}}}
	_, exceptions := dataset.GSTEvents("cash")
	if len(exceptions) != 1 || exceptions[0].Severity != SeverityError || exceptions[0].Date != "2026-10-01" {
		t.Fatalf("exceptions = %#v", exceptions)
	}
}

func TestCashPurchaseAllocationProblemUsesSettlementDate(t *testing.T) {
	dataset := Dataset{Purchases: []Purchase{{
		SourceID: "expense-1", Date: "2026-07-01", TaxKnown: true, SupplierAccount: true,
		Amounts: Amounts{GrossCents: 100, NetCents: 91, GSTCents: 9}, SettlementBaseCents: 100,
		Settlements: []Allocation{{SourceType: "bank_transaction", SourceID: "transaction-1", Date: "2026-10-02", AmountCents: 101}},
	}}}
	_, exceptions := dataset.GSTEvents("cash")
	if len(exceptions) != 1 || exceptions[0].Date != "2026-10-02" {
		t.Fatalf("exceptions = %#v", exceptions)
	}
}

func TestCashAllocationValidatesRowsAfterSourceTotalIsReached(t *testing.T) {
	dataset := Dataset{Sales: []Sale{{
		SourceID: "invoice-1", TaxKnown: true,
		Amounts: Amounts{GrossCents: 100, NetCents: 91, GSTCents: 9},
		Payments: []Allocation{
			{SourceType: "payment", SourceID: "payment-1", Date: "2026-07-01", AmountCents: 100},
			{SourceType: "payment", SourceID: "payment-duplicate", Date: "2026-07-02", AmountCents: 1},
		},
	}}}
	events, exceptions := dataset.GSTEvents("cash")
	if len(events) != 1 || events[0].Amounts != (Amounts{GrossCents: 100, NetCents: 91, GSTCents: 9}) {
		t.Fatalf("events = %#v", events)
	}
	if len(exceptions) != 1 || exceptions[0].Date != "2026-07-02" || !strings.Contains(exceptions[0].Message, "exceed source gross amount") {
		t.Fatalf("exceptions = %#v", exceptions)
	}
}

func TestGSTEventsFiltersCashBasisOnlySourceExceptionsFromAccrual(t *testing.T) {
	dataset := Dataset{Exceptions: []Exception{
		{Severity: SeverityError, SourceType: "payment", SourceID: "payment-1", Message: "cash-only", CashBasisOnly: true},
		{Severity: SeverityError, SourceType: "invoice", SourceID: "invoice-1", Message: "all bases"},
	}}
	_, cashExceptions := dataset.GSTEvents("cash")
	if len(cashExceptions) != 2 {
		t.Fatalf("cash exceptions = %#v", cashExceptions)
	}
	_, accrualExceptions := dataset.GSTEvents("accrual")
	if len(accrualExceptions) != 1 || accrualExceptions[0].Message != "all bases" {
		t.Fatalf("accrual exceptions = %#v", accrualExceptions)
	}
}

func TestCashOrdinaryUnpaidPurchaseHasNoEventOrError(t *testing.T) {
	dataset := Dataset{Purchases: []Purchase{{
		SourceID: "expense-historical-unpaid", Date: "2021-03-14", PaymentStatus: PaymentStatusUnpaid,
		TaxKnown: true, Amounts: Amounts{GrossCents: 11000, NetCents: 10000, GSTCents: 1000},
	}}}
	events, exceptions := dataset.GSTEvents("cash")
	if len(events) != 0 || len(exceptions) != 0 {
		t.Fatalf("events=%#v exceptions=%#v", events, exceptions)
	}
}

func TestCashPaidOrdinaryPurchaseStillRequiresPaymentDate(t *testing.T) {
	dataset := Dataset{Purchases: []Purchase{{
		SourceID: "expense-paid", Date: "2026-07-01", PaymentStatus: PaymentStatusPaid,
		TaxKnown: true, Amounts: Amounts{GrossCents: 11000, NetCents: 10000, GSTCents: 1000},
	}}}
	events, exceptions := dataset.GSTEvents("cash")
	if len(events) != 0 || len(exceptions) != 1 || !strings.Contains(exceptions[0].Message, "missing payment date") {
		t.Fatalf("events=%#v exceptions=%#v", events, exceptions)
	}
}

func TestCashPartiallySettledPurchaseStillRequiresSettlementDate(t *testing.T) {
	dataset := Dataset{Purchases: []Purchase{{
		SourceID: "expense-partial", Date: "2026-07-01", SupplierAccount: true,
		SettlementBaseCents: 11000, TaxKnown: true,
		Amounts:     Amounts{GrossCents: 11000, NetCents: 10000, GSTCents: 1000},
		Settlements: []Allocation{{SourceType: "bank_transaction", SourceID: "transaction-1", AmountCents: 5500}},
	}}}
	events, exceptions := dataset.GSTEvents("cash")
	if len(events) != 1 || events[0].Amounts.GrossCents != 5500 {
		t.Fatalf("events=%#v", events)
	}
	if len(exceptions) != 1 || !strings.Contains(exceptions[0].Message, "missing accounting date") {
		t.Fatalf("exceptions=%#v", exceptions)
	}
}
