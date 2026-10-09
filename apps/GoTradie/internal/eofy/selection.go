package eofy

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Options struct {
	FY  string
	Now time.Time
}

type Period struct {
	FY    int
	Start time.Time
	End   time.Time
}

func ResolvePeriod(options Options) (Period, error) {
	today := dateOnly(options.Now)
	if strings.TrimSpace(options.FY) != "" {
		fy, err := parseFY(options.FY)
		if err != nil {
			return Period{}, err
		}
		return periodForFY(fy, today.Location()), nil
	}

	currentFY := financialYear(today)
	period := periodForFY(currentFY, today.Location())
	if period.End.After(today) {
		period = periodForFY(currentFY-1, today.Location())
	}
	return period, nil
}

func Filename(period Period) string {
	return fmt.Sprintf("FY%d-EOFY.xlsx", period.FY)
}

func periodForFY(fy int, location *time.Location) Period {
	return Period{
		FY:    fy,
		Start: time.Date(fy-1, time.July, 1, 0, 0, 0, 0, location),
		End:   time.Date(fy, time.June, 30, 0, 0, 0, 0, location),
	}
}

func financialYear(date time.Time) int {
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
