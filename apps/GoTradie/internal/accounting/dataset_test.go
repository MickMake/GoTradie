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
		SourceID: "invoice-1", TaxKnown: true,
		Amounts:  Amounts{GrossCents: 100, NetCents: 91, GSTCents: 9},
		Payments: []Allocation{{SourceType: "payment", SourceID: "payment-1", Date: "2026-07-01", AmountCents: 101}},
	}}}
	_, exceptions := dataset.GSTEvents("cash")
	if len(exceptions) != 1 || exceptions[0].Severity != SeverityError {
		t.Fatalf("exceptions = %#v", exceptions)
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
