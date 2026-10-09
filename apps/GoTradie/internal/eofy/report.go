package eofy

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/MickMake/GoTradie/internal/accounting"
	"github.com/MickMake/GoTradie/internal/config"
)

const (
	assetCandidate  = "INSTANT ASSET WRITE-OFF CANDIDATE"
	assetReview     = "CAPITAL/DEPRECIATION REVIEW"
	assetIncomplete = "REVIEW REQUIRED"
)

type Summary struct {
	IncomeGrossCents  int64
	IncomeNetCents    int64
	IncomeGSTCents    int64
	ExpenseGrossCents int64
	ExpenseNetCents   int64
	ExpenseGSTCents   int64
	NetIncomeCents    int64
	NetGSTCents       int64
}

type AssetRow struct {
	FirstUsedDate         string
	Supplier              string
	Description           string
	SourceCostCents       *int64
	SourceGSTCents        *int64
	ClaimableGSTCents     *int64
	ThresholdCostCents    *int64
	ThresholdCents        int64
	BusinessUsePercent    *float64
	BusinessAdjustedCents *int64
	CategoryID            string
	CategoryName          string
	SourceType            string
	SourceID              string
	CapitalCheck          string
	Classification        string
}

type Report struct {
	Period          Period
	AccountingBasis string
	ThresholdCents  int64
	GeneratedAt     time.Time
	Source          string
	Status          string
	Summary         Summary
	Income          []accounting.GSTEvent
	Expenses        []accounting.GSTEvent
	Assets          []AssetRow
	Exceptions      []accounting.Exception
	Supporting      []accounting.GSTEvent
}

func Build(dataset accounting.Dataset, cfg config.EOFYConfig, options Options, generatedAt time.Time) (Report, error) {
	period, err := ResolvePeriod(options)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		Period: period, AccountingBasis: cfg.AccountingBasis,
		ThresholdCents: int64(math.Round(cfg.InstantAssetWriteoffThreshold * 100)),
		GeneratedAt:    generatedAt, Source: dataset.Source, Status: "COMPLETE",
	}
	events, exceptions := dataset.AccountingEvents(cfg.AccountingBasis)
	for _, event := range events {
		date, parseErr := time.Parse("2006-01-02", event.Date)
		if parseErr != nil {
			exceptions = append(exceptions, accounting.Exception{
				Severity: accounting.SeverityError, SourceType: event.SourceType, SourceID: event.SourceID,
				Date: event.Date, Message: fmt.Sprintf("invalid accounting date %q", event.Date),
			})
			continue
		}
		if !contains(period, date) {
			continue
		}
		report.Supporting = append(report.Supporting, event)
		if event.Kind == "sale" {
			report.Income = append(report.Income, event)
			report.Summary.IncomeGrossCents += event.Amounts.GrossCents
			report.Summary.IncomeNetCents += event.Amounts.NetCents
			report.Summary.IncomeGSTCents += event.Amounts.GSTCents
			continue
		}
		report.Expenses = append(report.Expenses, event)
		report.Summary.ExpenseGrossCents += event.Amounts.GrossCents
		report.Summary.ExpenseNetCents += event.Amounts.NetCents
		report.Summary.ExpenseGSTCents += event.Amounts.GSTCents
		if strings.TrimSpace(event.CategoryName) == "" {
			exceptions = append(exceptions, accounting.Exception{
				Severity: accounting.SeverityError, SourceType: event.SourceType, SourceID: event.SourceID,
				Date: event.Date, Message: "missing Invoice Ninja Expense Category",
			})
		}
	}
	report.Summary.NetIncomeCents = report.Summary.IncomeNetCents - report.Summary.ExpenseNetCents
	report.Summary.NetGSTCents = report.Summary.IncomeGSTCents - report.Summary.ExpenseGSTCents

	assetRows, assetExceptions := buildAssetRows(dataset.Purchases, period, report.ThresholdCents, dataset.CompanyCurrencyID)
	report.Assets = assetRows
	exceptions = append(exceptions, assetExceptions...)
	for _, exception := range uniqueExceptions(exceptions) {
		if !exceptionInPeriod(period, exception.Date) {
			continue
		}
		report.Exceptions = append(report.Exceptions, exception)
		if exception.Severity == accounting.SeverityError {
			report.Status = "INCOMPLETE"
		}
	}
	sortExceptions(report.Exceptions)
	return report, nil
}

func buildAssetRows(purchases []accounting.Purchase, period Period, thresholdCents int64, companyCurrencyID string) ([]AssetRow, []accounting.Exception) {
	var rows []AssetRow
	var exceptions []accounting.Exception
	for _, purchase := range purchases {
		capitalCheck := strings.TrimSpace(purchase.CapitalCheck)
		if capitalCheck == "" || capitalCheck == "No" {
			continue
		}
		firstUsed, dateErr := time.Parse("2006-01-02", purchase.Date)
		if dateErr == nil && !contains(period, firstUsed) {
			continue
		}
		if capitalCheck != "Review" && capitalCheck != "Review >$300" {
			exceptions = append(exceptions, assetException(purchase, fmt.Sprintf("invalid Capital check classification %q", capitalCheck)))
			continue
		}

		row := AssetRow{
			FirstUsedDate: purchase.Date, Supplier: purchase.VendorName, Description: purchase.Description,
			SourceCostCents: purchase.SourceGrossCents, SourceGSTCents: purchase.SourceGSTCents,
			ThresholdCents: thresholdCents, BusinessUsePercent: purchase.BusinessUsePercent,
			CategoryID: purchase.CategoryID, CategoryName: purchase.CategoryName,
			SourceType: "expense", SourceID: purchase.SourceID, CapitalCheck: capitalCheck,
			Classification: assetIncomplete,
		}
		var problems []string
		if dateErr != nil {
			problems = append(problems, fmt.Sprintf("invalid first-used date %q", purchase.Date))
		}
		if purchase.SourceEvidenceError != "" {
			problems = append(problems, purchase.SourceEvidenceError)
		}
		if purchase.SourceGrossCents == nil {
			problems = append(problems, "missing Source total inc GST")
		} else if *purchase.SourceGrossCents <= 0 {
			problems = append(problems, "Source total inc GST must be greater than zero")
		}
		if purchase.SourceGSTCents == nil {
			problems = append(problems, "missing Source GST")
		} else if *purchase.SourceGSTCents < 0 {
			problems = append(problems, "Source GST must not be negative")
		}
		if !purchase.TaxKnown {
			problems = append(problems, "missing GST credit entitlement")
		} else {
			claimable := purchase.Amounts.GSTCents
			row.ClaimableGSTCents = &claimable
			if claimable < 0 {
				problems = append(problems, "claimable GST credit must not be negative")
			} else if purchase.SourceGSTCents != nil && claimable > *purchase.SourceGSTCents {
				problems = append(problems, "claimable GST credit exceeds Source GST")
			}
		}
		if purchase.BusinessUsePercent == nil {
			problems = append(problems, "missing business-use percentage")
		}
		if strings.TrimSpace(purchase.CategoryName) == "" {
			problems = append(problems, "missing Invoice Ninja Expense Category")
		}
		if purchase.ArchivedOrDeleted {
			problems = append(problems, "archived or deleted accounting record affects the asset review")
		}
		if strings.TrimSpace(companyCurrencyID) != "" && strings.TrimSpace(purchase.CurrencyID) != "" && purchase.CurrencyID != companyCurrencyID {
			problems = append(problems, fmt.Sprintf("unsupported foreign currency %q; company currency is %q", purchase.CurrencyID, companyCurrencyID))
		}

		if len(problems) == 0 {
			thresholdCost := *purchase.SourceGrossCents - *row.ClaimableGSTCents
			if thresholdCost < 0 {
				problems = append(problems, "threshold-test cost must not be negative")
			} else {
				row.ThresholdCostCents = &thresholdCost
				adjusted := int64(math.Round(float64(thresholdCost) * *purchase.BusinessUsePercent / 100))
				row.BusinessAdjustedCents = &adjusted
				if thresholdCost < thresholdCents {
					row.Classification = assetCandidate
				} else {
					row.Classification = assetReview
				}
			}
		}
		for _, problem := range problems {
			exceptions = append(exceptions, assetException(purchase, problem))
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].FirstUsedDate != rows[j].FirstUsedDate {
			return rows[i].FirstUsedDate < rows[j].FirstUsedDate
		}
		return rows[i].SourceID < rows[j].SourceID
	})
	return rows, exceptions
}

func assetException(purchase accounting.Purchase, message string) accounting.Exception {
	return accounting.Exception{
		Severity: accounting.SeverityError, SourceType: "expense", SourceID: purchase.SourceID,
		Date: purchase.Date, Message: message,
	}
}

func contains(period Period, date time.Time) bool {
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, period.Start.Location())
	return !date.Before(period.Start) && !date.After(period.End)
}

func exceptionInPeriod(period Period, value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return true
	}
	return contains(period, date)
}

func uniqueExceptions(exceptions []accounting.Exception) []accounting.Exception {
	seen := make(map[string]bool, len(exceptions))
	result := make([]accounting.Exception, 0, len(exceptions))
	for _, exception := range exceptions {
		key := strings.Join([]string{string(exception.Severity), exception.SourceType, exception.SourceID, exception.Date, exception.Message}, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, exception)
	}
	return result
}

func sortExceptions(exceptions []accounting.Exception) {
	severityRank := map[accounting.Severity]int{
		accounting.SeverityError: 3, accounting.SeverityWarning: 2, accounting.SeverityInfo: 1,
	}
	sort.SliceStable(exceptions, func(i, j int) bool {
		if severityRank[exceptions[i].Severity] != severityRank[exceptions[j].Severity] {
			return severityRank[exceptions[i].Severity] > severityRank[exceptions[j].Severity]
		}
		if exceptions[i].Date != exceptions[j].Date {
			return exceptions[i].Date < exceptions[j].Date
		}
		if exceptions[i].SourceID != exceptions[j].SourceID {
			return exceptions[i].SourceID < exceptions[j].SourceID
		}
		return exceptions[i].Message < exceptions[j].Message
	})
}

func (r Report) HasErrors() bool {
	return r.Status == "INCOMPLETE"
}
