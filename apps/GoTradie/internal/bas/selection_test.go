package bas

import (
	"fmt"
	"testing"
	"time"

	"github.com/MickMake/GoTradie/internal/config"
)

func TestDefaultSelectsMostRecentlyCompletedPeriodAcrossFYBoundary(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.BASConfig
		now  string
		fy   int
		key  string
	}{
		{"monthly before July completion", monthlyConfig(), "2026-07-30", 2026, "Jun"},
		{"monthly on period end", monthlyConfig(), "2026-07-31", 2027, "Jul"},
		{"quarterly before September completion", quarterlyConfig(), "2026-09-29", 2026, "Q4"},
		{"quarterly on period end", quarterlyConfig(), "2026-09-30", 2027, "Q1"},
		{"yearly before financial year end", yearlyConfig(), "2026-06-29", 2025, "FY"},
		{"yearly on financial year end", yearlyConfig(), "2026-06-30", 2026, "FY"},
		{"yearly after financial year rollover", yearlyConfig(), "2026-07-01", 2026, "FY"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			periods, err := ResolvePeriods(test.cfg, Options{Now: mustDate(t, test.now)})
			if err != nil {
				t.Fatal(err)
			}
			if len(periods) != 1 || periods[0].FY != test.fy || periods[0].Key != test.key {
				t.Fatalf("periods = %#v; want FY%d %s", periods, test.fy, test.key)
			}
		})
	}
}

func TestExplicitSelectionsTakePrecedence(t *testing.T) {
	cfg := quarterlyConfig()
	now := mustDate(t, "2026-07-01")

	periods, err := ResolvePeriods(cfg, Options{Now: now, FY: "2027"})
	if err != nil {
		t.Fatal(err)
	}
	if len(periods) != 1 || periods[0].FY != 2027 || periods[0].Key != "Q1" {
		t.Fatalf("explicit current FY = %#v; want in-progress FY2027 Q1", periods)
	}

	periods, err = ResolvePeriods(cfg, Options{Now: now, FY: "2025"})
	if err != nil {
		t.Fatal(err)
	}
	if len(periods) != 4 || periods[0].FY != 2025 || periods[3].Key != "Q4" {
		t.Fatalf("historic FY = %#v", periods)
	}

	periods, err = ResolvePeriods(cfg, Options{Now: now, FY: "2027", Period: "April"})
	if err != nil {
		t.Fatal(err)
	}
	if len(periods) != 1 || periods[0].Key != "Q4" {
		t.Fatalf("explicit month quarter = %#v; want Q4", periods)
	}
}

func TestPeriodParsingAndFilenames(t *testing.T) {
	periods, err := ResolvePeriods(monthlyConfig(), Options{Now: mustDate(t, "2026-10-09"), FY: "2027", Period: "September"})
	if err != nil {
		t.Fatal(err)
	}
	if periods[0].Key != "Sep" || Filename(periods) != "FY2027-BAS-Sep.xlsx" {
		t.Fatalf("monthly selection = %#v, %q", periods, Filename(periods))
	}
	if _, err := ResolvePeriods(yearlyConfig(), Options{Now: mustDate(t, "2026-10-09"), Period: "1"}); err == nil {
		t.Fatal("yearly --period should fail")
	}
	if _, err := ResolvePeriods(quarterlyConfig(), Options{Now: mustDate(t, "2026-10-09"), FY: "27"}); err == nil {
		t.Fatal("short FY should fail")
	}
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
