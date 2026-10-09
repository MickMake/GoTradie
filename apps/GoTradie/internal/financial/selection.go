package financial

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MickMake/GoTradie/internal/bas"
	"github.com/MickMake/GoTradie/internal/config"
)

type Options struct {
	FY     string
	Period string
	From   string
	To     string
	Now    time.Time
}

type Selection struct {
	FY    int
	Start *time.Time
	End   *time.Time
}

func ResolveSelection(cfg config.BASConfig, options Options) (Selection, error) {
	hasBASSelection := strings.TrimSpace(options.FY) != "" || strings.TrimSpace(options.Period) != ""
	hasDateSelection := strings.TrimSpace(options.From) != "" || strings.TrimSpace(options.To) != ""
	if hasBASSelection && hasDateSelection {
		return Selection{}, fmt.Errorf("--fy/--period cannot be combined with --from/--to")
	}
	if hasDateSelection {
		start, err := optionalDate("--from", options.From)
		if err != nil {
			return Selection{}, err
		}
		end, err := optionalDate("--to", options.To)
		if err != nil {
			return Selection{}, err
		}
		if start != nil && end != nil && start.After(*end) {
			return Selection{}, fmt.Errorf("--from must be on or before --to")
		}
		return Selection{Start: start, End: end}, nil
	}
	if !hasBASSelection {
		return Selection{}, nil
	}

	now := dateOnly(options.Now)
	fy := bas.FinancialYear(now)
	if strings.TrimSpace(options.FY) != "" {
		parsed, err := parseFY(options.FY)
		if err != nil {
			return Selection{}, err
		}
		fy = parsed
	}
	if strings.TrimSpace(options.Period) != "" {
		periods, err := bas.ResolvePeriods(cfg, bas.Options{FY: strconv.Itoa(fy), Period: options.Period, Now: now})
		if err != nil {
			return Selection{}, err
		}
		start, end := periods[0].Start, periods[0].End
		return Selection{FY: fy, Start: &start, End: &end}, nil
	}
	periods, err := bas.PeriodsForFY(cfg, fy)
	if err != nil {
		return Selection{}, err
	}
	start, end := periods[0].Start, periods[len(periods)-1].End
	return Selection{FY: fy, Start: &start, End: &end}, nil
}

func Filename(selection Selection) string {
	if selection.FY != 0 && selection.Start != nil && selection.End != nil && isCompleteFinancialYear(selection) {
		return fmt.Sprintf("FY%d-Financial.xlsx", selection.FY)
	}
	switch {
	case selection.Start != nil && selection.End != nil:
		return fmt.Sprintf("Financial-%s-to-%s.xlsx", formatDate(*selection.Start), formatDate(*selection.End))
	case selection.Start != nil:
		return fmt.Sprintf("Financial-from-%s.xlsx", formatDate(*selection.Start))
	case selection.End != nil:
		return fmt.Sprintf("Financial-to-%s.xlsx", formatDate(*selection.End))
	default:
		return "Financial-All.xlsx"
	}
}

func (s Selection) HasDateFilter() bool {
	return s.Start != nil || s.End != nil
}

func (s Selection) Includes(date time.Time) bool {
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
	if s.Start != nil && date.Before(*s.Start) {
		return false
	}
	return s.End == nil || !date.After(*s.End)
}

func isCompleteFinancialYear(selection Selection) bool {
	if selection.FY == 0 {
		return false
	}
	wantStart := time.Date(selection.FY-1, time.July, 1, 0, 0, 0, 0, selection.Start.Location())
	wantEnd := time.Date(selection.FY, time.June, 30, 0, 0, 0, 0, selection.End.Location())
	return selection.Start.Equal(wantStart) && selection.End.Equal(wantEnd)
}

func optionalDate(name, value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	date, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return nil, fmt.Errorf("%s must use YYYY-MM-DD", name)
	}
	return &date, nil
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

func formatDate(value time.Time) string {
	return value.Format("2006-01-02")
}
