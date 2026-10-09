package bas

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MickMake/GoTradie/internal/config"
)

type Options struct {
	FY     string
	Period string
	Now    time.Time
}

type Period struct {
	FY              int
	ReportingPeriod string
	Key             string
	Number          int
	Start           time.Time
	End             time.Time
}

func ResolvePeriods(cfg config.BASConfig, options Options) ([]Period, error) {
	today := dateOnly(options.Now)
	currentFY := FinancialYear(today)
	selectedFY := currentFY
	fyExplicit := strings.TrimSpace(options.FY) != ""
	if fyExplicit {
		parsed, err := parseFY(options.FY)
		if err != nil {
			return nil, err
		}
		selectedFY = parsed
	}

	if strings.TrimSpace(options.Period) != "" {
		periods, err := PeriodsForFY(cfg, selectedFY)
		if err != nil {
			return nil, err
		}
		period, err := selectExplicitPeriod(cfg.ReportingPeriod, periods, options.Period)
		if err != nil {
			return nil, err
		}
		return []Period{period}, nil
	}

	if fyExplicit && selectedFY != currentFY {
		return PeriodsForFY(cfg, selectedFY)
	}
	if fyExplicit {
		periods, err := PeriodsForFY(cfg, selectedFY)
		if err != nil {
			return nil, err
		}
		if completed, ok := latestCompleted(periods, today); ok {
			return []Period{completed}, nil
		}
		for _, period := range periods {
			if !today.Before(period.Start) && !today.After(period.End) {
				return []Period{period}, nil
			}
		}
		return nil, fmt.Errorf("FY%d has no reporting period applicable on %s", selectedFY, today.Format("2006-01-02"))
	}

	currentPeriods, err := PeriodsForFY(cfg, currentFY)
	if err != nil {
		return nil, err
	}
	previousPeriods, err := PeriodsForFY(cfg, currentFY-1)
	if err != nil {
		return nil, err
	}
	candidates := append(previousPeriods, currentPeriods...)
	completed, ok := latestCompleted(candidates, today)
	if !ok {
		return nil, fmt.Errorf("no completed %s BAS period exists on or before %s", cfg.ReportingPeriod, today.Format("2006-01-02"))
	}
	return []Period{completed}, nil
}

// PeriodsForFY resolves the configured BAS calendar for a complete financial year.
func PeriodsForFY(cfg config.BASConfig, fy int) ([]Period, error) {
	keys, err := periodKeys(cfg.ReportingPeriod)
	if err != nil {
		return nil, err
	}
	periods := make([]Period, 0, len(keys))
	for index, key := range keys {
		template, ok := cfg.Periods[key]
		if !ok {
			return nil, fmt.Errorf("bas.periods.%s is required", key)
		}
		start, err := periodDate(fy, template.BASBegin, false)
		if err != nil {
			return nil, fmt.Errorf("bas.periods.%s.bas_begin: %w", key, err)
		}
		end, err := periodDate(fy, template.BASEnd, true)
		if err != nil {
			return nil, fmt.Errorf("bas.periods.%s.bas_end: %w", key, err)
		}
		periods = append(periods, Period{FY: fy, ReportingPeriod: cfg.ReportingPeriod, Key: key, Number: index + 1, Start: start, End: end})
	}
	return periods, nil
}

func periodKeys(reportingPeriod string) ([]string, error) {
	switch reportingPeriod {
	case "monthly":
		return []string{"Jul", "Aug", "Sep", "Oct", "Nov", "Dec", "Jan", "Feb", "Mar", "Apr", "May", "Jun"}, nil
	case "quarterly":
		return []string{"Q1", "Q2", "Q3", "Q4"}, nil
	case "yearly":
		return []string{"FY"}, nil
	default:
		return nil, fmt.Errorf("unsupported BAS reporting period %q", reportingPeriod)
	}
}

func periodDate(fy int, value string, allowLast bool) (time.Time, error) {
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("must use MM-DD")
	}
	month, err := strconv.Atoi(parts[0])
	if err != nil || month < 1 || month > 12 {
		return time.Time{}, fmt.Errorf("invalid month in %q", value)
	}
	year := fy
	if month >= int(time.July) {
		year = fy - 1
	}
	day := 0
	if parts[1] == "last" {
		if !allowLast {
			return time.Time{}, fmt.Errorf("MM-last is not valid here")
		}
		day = time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.Local).Day()
	} else {
		day, err = strconv.Atoi(parts[1])
		if err != nil || day < 1 || day > 31 {
			return time.Time{}, fmt.Errorf("invalid day in %q", value)
		}
	}
	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
	if int(date.Month()) != month || date.Day() != day {
		return time.Time{}, fmt.Errorf("invalid calendar date %q", value)
	}
	return date, nil
}

func selectExplicitPeriod(reportingPeriod string, periods []Period, value string) (Period, error) {
	if reportingPeriod == "yearly" {
		return Period{}, fmt.Errorf("--period is invalid for yearly BAS reporting")
	}
	value = strings.ToLower(strings.TrimSpace(value))
	if number, err := strconv.Atoi(value); err == nil {
		if number < 1 || number > len(periods) {
			return Period{}, fmt.Errorf("--period must be between 1 and %d for %s reporting", len(periods), reportingPeriod)
		}
		return periods[number-1], nil
	}
	month, ok := monthNumber(value)
	if !ok {
		return Period{}, fmt.Errorf("unsupported --period %q for %s reporting", value, reportingPeriod)
	}
	index := (month + 5) % 12
	if reportingPeriod == "quarterly" {
		index /= 3
	}
	return periods[index], nil
}

func monthNumber(value string) (int, bool) {
	months := []struct {
		short string
		full  string
	}{{"jan", "january"}, {"feb", "february"}, {"mar", "march"}, {"apr", "april"}, {"may", "may"}, {"jun", "june"}, {"jul", "july"}, {"aug", "august"}, {"sep", "september"}, {"oct", "october"}, {"nov", "november"}, {"dec", "december"}}
	for index, month := range months {
		if value == month.short || value == month.full {
			return index + 1, true
		}
	}
	return 0, false
}

func latestCompleted(periods []Period, today time.Time) (Period, bool) {
	var selected Period
	found := false
	for _, period := range periods {
		if period.End.After(today) {
			continue
		}
		if !found || period.End.After(selected.End) {
			selected = period
			found = true
		}
	}
	return selected, found
}

// FinancialYear returns the Australian financial year ending in date's year,
// or the following year for dates from July onward.
func FinancialYear(date time.Time) int {
	if date.Month() >= time.July {
		return date.Year() + 1
	}
	return date.Year()
}

func parseFY(value string) (int, error) {
	value = strings.TrimSpace(value)
	if len(value) != 4 {
		return 0, fmt.Errorf("--fy must use exactly YYYY")
	}
	fy, err := strconv.Atoi(value)
	if err != nil || fy < 1000 || fy > 9999 {
		return 0, fmt.Errorf("--fy must use exactly YYYY")
	}
	return fy, nil
}

func dateOnly(value time.Time) time.Time {
	if value.IsZero() {
		value = time.Now()
	}
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func (p Period) Label() string {
	if p.ReportingPeriod == "yearly" {
		return fmt.Sprintf("FY%d", p.FY)
	}
	return fmt.Sprintf("FY%d %s", p.FY, p.Key)
}

func Filename(periods []Period) string {
	if len(periods) == 0 {
		return "BAS.xlsx"
	}
	if len(periods) > 1 || periods[0].ReportingPeriod == "yearly" {
		return fmt.Sprintf("FY%d-BAS.xlsx", periods[0].FY)
	}
	return fmt.Sprintf("FY%d-BAS-%s.xlsx", periods[0].FY, periods[0].Key)
}
