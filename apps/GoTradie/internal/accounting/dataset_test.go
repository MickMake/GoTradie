package accounting

import "testing"

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
