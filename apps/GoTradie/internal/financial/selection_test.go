package financial

import (
	"fmt"
	"testing"
	"time"

	"github.com/MickMake/GoTradie/internal/config"
)

func TestResolveSelectionDefaultsToAll(t *testing.T) {
	selection, err := ResolveSelection(quarterlyConfig(), Options{Now: mustDate(t, "2026-10-09")})
	if err != nil {
		t.Fatal(err)
	}
	if selection.HasDateFilter() || selection.FY != 0 || Filename(selection) != "Financial-All.xlsx" {
		t.Fatalf("selection=%#v filename=%q", selection, Filename(selection))
	}
}

func TestResolveSelectionUsesBASCalendar(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.BASConfig
		options  Options
		wantFY   int
		wantFrom string
		wantTo   string
		filename string
	}{
		{
			name: "financial year", cfg: quarterlyConfig(),
			options: Options{FY: "2027", Now: mustDate(t, "2026-10-09")},
			wantFY:  2027, wantFrom: "2026-07-01", wantTo: "2027-06-30", filename: "FY2027-Financial.xlsx",
		},
		{
			name: "monthly name", cfg: monthlyConfig(),
			options: Options{FY: "2027", Period: "September", Now: mustDate(t, "2026-10-09")},
			wantFY:  2027, wantFrom: "2026-09-01", wantTo: "2026-09-30", filename: "Financial-2026-09-01-to-2026-09-30.xlsx",
		},
		{
			name: "quarter number", cfg: quarterlyConfig(),
			options: Options{FY: "2027", Period: "2", Now: mustDate(t, "2026-10-09")},
			wantFY:  2027, wantFrom: "2026-10-01", wantTo: "2026-12-31", filename: "Financial-2026-10-01-to-2026-12-31.xlsx",
		},
		{
			name: "period defaults to current FY", cfg: quarterlyConfig(),
			options: Options{Period: "April", Now: mustDate(t, "2026-10-09")},
			wantFY:  2027, wantFrom: "2027-04-01", wantTo: "2027-06-30", filename: "Financial-2027-04-01-to-2027-06-30.xlsx",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selection, err := ResolveSelection(test.cfg, test.options)
			if err != nil {
				t.Fatal(err)
			}
			if selection.FY != test.wantFY || dateText(selection.Start) != test.wantFrom || dateText(selection.End) != test.wantTo {
				t.Fatalf("selection=%#v; want FY%d %s..%s", selection, test.wantFY, test.wantFrom, test.wantTo)
			}
			if got := Filename(selection); got != test.filename {
				t.Fatalf("filename=%q; want %q", got, test.filename)
			}
		})
	}
}

func TestResolveSelectionExplicitDateRanges(t *testing.T) {
	tests := []struct {
		options  Options
		filename string
	}{
		{Options{From: "2025-01-01", To: "2025-06-30"}, "Financial-2025-01-01-to-2025-06-30.xlsx"},
		{Options{From: "2025-01-01"}, "Financial-from-2025-01-01.xlsx"},
		{Options{To: "2025-06-30"}, "Financial-to-2025-06-30.xlsx"},
	}
	for _, test := range tests {
		selection, err := ResolveSelection(quarterlyConfig(), test.options)
		if err != nil {
			t.Fatal(err)
		}
		if got := Filename(selection); got != test.filename {
			t.Fatalf("filename=%q; want %q", got, test.filename)
		}
	}
}

func TestResolveSelectionRejectsInvalidSelectors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.BASConfig
		options Options
	}{
		{"mixed FY and range", quarterlyConfig(), Options{FY: "2027", From: "2026-07-01"}},
		{"mixed period and range", quarterlyConfig(), Options{Period: "2", To: "2026-12-31"}},
		{"bad from", quarterlyConfig(), Options{From: "01-07-2026"}},
		{"bad to", quarterlyConfig(), Options{To: "2026-02-30"}},
		{"reverse range", quarterlyConfig(), Options{From: "2026-08-01", To: "2026-07-01"}},
		{"yearly period", yearlyConfig(), Options{FY: "2027", Period: "1", Now: mustDate(t, "2026-10-09")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ResolveSelection(test.cfg, test.options); err == nil {
				t.Fatal("expected selection error")
			}
		})
	}
}

func dateText(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02")
}

func quarterlyConfig() config.BASConfig {
	return config.BASConfig{ReportingPeriod: "quarterly", GSTBasis: "cash", Periods: map[string]config.BASPeriod{
		"Q1": {BASBegin: "07-01", BASEnd: "09-30"},
		"Q2": {BASBegin: "10-01", BASEnd: "12-31"},
		"Q3": {BASBegin: "01-01", BASEnd: "03-31"},
		"Q4": {BASBegin: "04-01", BASEnd: "06-30"},
	}}
}

func monthlyConfig() config.BASConfig {
	keys := []string{"Jul", "Aug", "Sep", "Oct", "Nov", "Dec", "Jan", "Feb", "Mar", "Apr", "May", "Jun"}
	periods := make(map[string]config.BASPeriod, len(keys))
	for index, key := range keys {
		month := ((index + 6) % 12) + 1
		periods[key] = config.BASPeriod{BASBegin: fmt.Sprintf("%02d-01", month), BASEnd: fmt.Sprintf("%02d-last", month)}
	}
	return config.BASConfig{ReportingPeriod: "monthly", GSTBasis: "cash", Periods: periods}
}

func yearlyConfig() config.BASConfig {
	return config.BASConfig{ReportingPeriod: "yearly", GSTBasis: "cash", Periods: map[string]config.BASPeriod{
		"FY": {BASBegin: "07-01", BASEnd: "06-30"},
	}}
}

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	date, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	return date
}
