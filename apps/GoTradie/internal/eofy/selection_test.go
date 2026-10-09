package eofy

import (
	"testing"
	"time"
)

func TestResolvePeriodDefaultsToMostRecentlyCompletedFinancialYear(t *testing.T) {
	for _, test := range []struct {
		name string
		now  time.Time
		want int
	}{
		{name: "day before year end", now: time.Date(2026, time.June, 29, 12, 0, 0, 0, time.Local), want: 2025},
		{name: "year end is complete", now: time.Date(2026, time.June, 30, 12, 0, 0, 0, time.Local), want: 2026},
		{name: "first day of next year", now: time.Date(2026, time.July, 1, 12, 0, 0, 0, time.Local), want: 2026},
	} {
		t.Run(test.name, func(t *testing.T) {
			period, err := ResolvePeriod(Options{Now: test.now})
			if err != nil {
				t.Fatal(err)
			}
			if period.FY != test.want {
				t.Fatalf("FY = %d; want %d", period.FY, test.want)
			}
		})
	}
}

func TestResolvePeriodExplicitFYTakesPrecedence(t *testing.T) {
	period, err := ResolvePeriod(Options{FY: "2027", Now: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.Local)})
	if err != nil {
		t.Fatal(err)
	}
	if period.FY != 2027 {
		t.Fatalf("period = %#v", period)
	}
	if got := period.Start.Format("2006-01-02"); got != "2026-07-01" {
		t.Fatalf("period = %#v", period)
	}
	if got := Filename(period); got != "FY2027-EOFY.xlsx" {
		t.Fatalf("filename = %q", got)
	}
}

func TestResolvePeriodRejectsMalformedFY(t *testing.T) {
	for _, value := range []string{"27", "FY2027", "2027-28", "abcd"} {
		if _, err := ResolvePeriod(Options{FY: value}); err == nil {
			t.Fatalf("FY %q should fail", value)
		}
	}
}
