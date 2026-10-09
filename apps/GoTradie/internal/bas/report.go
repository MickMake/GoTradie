package bas

import (
	"fmt"
	"sort"
	"time"

	"github.com/MickMake/GoTradie/internal/accounting"
	"github.com/MickMake/GoTradie/internal/config"
)

type Summary struct {
	Period      Period
	GSTBasis    string
	GeneratedAt time.Time
	Source      string
	Status      string
	G1Cents     int64
	OneACents   int64
	OneBCents   int64
	NetGSTCents int64
}

type EventRow struct {
	Period string
	Event  accounting.GSTEvent
}

type ExceptionRow struct {
	Period    string
	Exception accounting.Exception
}

type Report struct {
	Periods     []Period
	Summaries   []Summary
	Sales       []EventRow
	Purchases   []EventRow
	Exceptions  []ExceptionRow
	GeneratedAt time.Time
	Source      string
	Status      string
}

func Build(dataset accounting.Dataset, cfg config.BASConfig, options Options, generatedAt time.Time) (Report, error) {
	periods, err := ResolvePeriods(cfg, options)
	if err != nil {
		return Report{}, err
	}
	events, exceptions := dataset.GSTEvents(cfg.GSTBasis)
	report := Report{Periods: periods, GeneratedAt: generatedAt, Source: dataset.Source, Status: "COMPLETE"}

	for _, event := range events {
		date, parseErr := time.Parse("2006-01-02", event.Date)
		if parseErr != nil {
			exceptions = append(exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: event.SourceType, SourceID: event.SourceID, Date: event.Date, Message: fmt.Sprintf("invalid accounting date %q", event.Date)})
			continue
		}
		period, ok := containingPeriod(periods, date)
		if !ok {
			continue
		}
		row := EventRow{Period: period.Label(), Event: event}
		if event.Kind == "sale" {
			report.Sales = append(report.Sales, row)
		} else {
			report.Purchases = append(report.Purchases, row)
		}
	}

	for _, exception := range exceptions {
		label, include := exceptionPeriod(periods, exception.Date)
		if !include {
			continue
		}
		report.Exceptions = append(report.Exceptions, ExceptionRow{Period: label, Exception: exception})
		if exception.Severity == accounting.SeverityError {
			report.Status = "INCOMPLETE"
		}
	}

	for _, period := range periods {
		summary := Summary{Period: period, GSTBasis: cfg.GSTBasis, GeneratedAt: generatedAt, Source: dataset.Source, Status: report.Status}
		for _, row := range report.Sales {
			if row.Period == period.Label() {
				summary.G1Cents += row.Event.Amounts.GrossCents
				summary.OneACents += row.Event.Amounts.GSTCents
			}
		}
		for _, row := range report.Purchases {
			if row.Period == period.Label() {
				summary.OneBCents += row.Event.Amounts.GSTCents
			}
		}
		summary.NetGSTCents = summary.OneACents - summary.OneBCents
		report.Summaries = append(report.Summaries, summary)
	}
	sort.SliceStable(report.Exceptions, func(i, j int) bool {
		if report.Exceptions[i].Exception.Severity != report.Exceptions[j].Exception.Severity {
			return report.Exceptions[i].Exception.Severity > report.Exceptions[j].Exception.Severity
		}
		if report.Exceptions[i].Exception.Date != report.Exceptions[j].Exception.Date {
			return report.Exceptions[i].Exception.Date < report.Exceptions[j].Exception.Date
		}
		return report.Exceptions[i].Exception.SourceID < report.Exceptions[j].Exception.SourceID
	})
	return report, nil
}

func containingPeriod(periods []Period, date time.Time) (Period, bool) {
	for _, period := range periods {
		start := time.Date(period.Start.Year(), period.Start.Month(), period.Start.Day(), 0, 0, 0, 0, time.UTC)
		end := time.Date(period.End.Year(), period.End.Month(), period.End.Day(), 0, 0, 0, 0, time.UTC)
		if !date.Before(start) && !date.After(end) {
			return period, true
		}
	}
	return Period{}, false
}

func exceptionPeriod(periods []Period, dateText string) (string, bool) {
	if dateText == "" {
		return "", true
	}
	date, err := time.Parse("2006-01-02", dateText)
	if err != nil {
		return "", true
	}
	period, ok := containingPeriod(periods, date)
	if !ok {
		return "", false
	}
	return period.Label(), true
}

func (r Report) HasErrors() bool {
	return r.Status == "INCOMPLETE"
}
