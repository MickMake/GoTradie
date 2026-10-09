package eofy

import (
	"fmt"

	"github.com/MickMake/GoTradie/internal/accounting"
	"github.com/xuri/excelize/v2"
)

func Workbook(report Report) ([]byte, error) {
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	if err := book.SetSheetName("Sheet1", "Summary"); err != nil {
		return nil, err
	}
	for _, sheet := range []string{"Income", "Expenses", "Capital-Asset Review", "GST Reconciliation", "Exceptions", "Supporting Detail"} {
		if _, err := book.NewSheet(sheet); err != nil {
			return nil, err
		}
	}

	headerStyle, err := book.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"1F4E78"}},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}
	moneyStyle, err := book.NewStyle(&excelize.Style{NumFmt: 4})
	if err != nil {
		return nil, err
	}
	percentageStyle, err := book.NewStyle(&excelize.Style{NumFmt: 2})
	if err != nil {
		return nil, err
	}
	statusStyle, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: statusColour(report.Status)}})
	if err != nil {
		return nil, err
	}

	if err := writeSummary(book, report, headerStyle, moneyStyle, statusStyle); err != nil {
		return nil, err
	}
	if err := writeEvents(book, "Income", report.Income, headerStyle, moneyStyle, percentageStyle, false); err != nil {
		return nil, err
	}
	if err := writeEvents(book, "Expenses", report.Expenses, headerStyle, moneyStyle, percentageStyle, false); err != nil {
		return nil, err
	}
	if err := writeAssets(book, report.Assets, headerStyle, moneyStyle, percentageStyle); err != nil {
		return nil, err
	}
	if err := writeGSTReconciliation(book, report, headerStyle, moneyStyle); err != nil {
		return nil, err
	}
	if err := writeExceptions(book, report.Exceptions, headerStyle); err != nil {
		return nil, err
	}
	if err := writeEvents(book, "Supporting Detail", report.Supporting, headerStyle, moneyStyle, percentageStyle, true); err != nil {
		return nil, err
	}

	buffer, err := book.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeSummary(book *excelize.File, report Report, headerStyle, moneyStyle, statusStyle int) error {
	if err := writeHeader(book, "Summary", []string{"Field", "Value"}, headerStyle); err != nil {
		return err
	}
	rows := [][]any{
		{"Financial Year", fmt.Sprintf("FY%d", report.Period.FY)},
		{"Period Start", report.Period.Start.Format("2006-01-02")},
		{"Period End", report.Period.End.Format("2006-01-02")},
		{"EOFY Accounting Basis", report.AccountingBasis},
		{"Instant Asset Write-off Threshold", money(report.ThresholdCents)},
		{"Generated Timestamp", report.GeneratedAt.Format("2006-01-02T15:04:05Z07:00")},
		{"Source", report.Source},
		{"Report Status", report.Status},
		{"Income Gross", money(report.Summary.IncomeGrossCents)},
		{"Income Net", money(report.Summary.IncomeNetCents)},
		{"Income GST", money(report.Summary.IncomeGSTCents)},
		{"Expense Gross", money(report.Summary.ExpenseGrossCents)},
		{"Expense Net", money(report.Summary.ExpenseNetCents)},
		{"Expense GST", money(report.Summary.ExpenseGSTCents)},
		{"Net Business Income", money(report.Summary.NetIncomeCents)},
		{"Net GST Position", money(report.Summary.NetGSTCents)},
	}
	for index, row := range rows {
		if err := writeRow(book, "Summary", index+2, row); err != nil {
			return err
		}
	}
	if err := book.SetCellStyle("Summary", "B6", "B6", moneyStyle); err != nil {
		return err
	}
	if err := book.SetCellStyle("Summary", "B9", "B9", statusStyle); err != nil {
		return err
	}
	if err := book.SetCellStyle("Summary", "B10", "B17", moneyStyle); err != nil {
		return err
	}
	_ = book.SetColWidth("Summary", "A", "A", 36)
	_ = book.SetColWidth("Summary", "B", "B", 30)
	return freezeHeader(book, "Summary")
}

func writeEvents(book *excelize.File, sheet string, rows []accounting.GSTEvent, headerStyle, moneyStyle, percentageStyle int, includeKind bool) error {
	headers := []string{
		"Recognition Date", "Source Date", "Source Type", "Source ID", "Related Source Type", "Related Source ID",
		"Number", "Party ID", "Party", "Category ID", "Expense Category", "Description", "Capital Check", "Gross", "Net", "GST", "Business Use %",
	}
	if includeKind {
		headers = append([]string{"Kind"}, headers...)
	}
	if err := writeHeader(book, sheet, headers, headerStyle); err != nil {
		return err
	}
	for index, event := range rows {
		businessUse := any("")
		if event.BusinessUsePercent != nil {
			businessUse = *event.BusinessUsePercent
		}
		values := []any{
			event.Date, event.SourceDate, event.SourceType, event.SourceID, event.RelatedSourceType, event.RelatedSourceID,
			event.Number, event.PartyID, event.PartyName, event.CategoryID, event.CategoryName, event.Description,
			event.CapitalCheck, money(event.Amounts.GrossCents), money(event.Amounts.NetCents), money(event.Amounts.GSTCents), businessUse,
		}
		if includeKind {
			values = append([]any{event.Kind}, values...)
		}
		rowNumber := index + 2
		if err := writeRow(book, sheet, rowNumber, values); err != nil {
			return err
		}
		moneyStart := "N"
		percentageColumn := "Q"
		if includeKind {
			moneyStart = "O"
			percentageColumn = "R"
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("%s%d", moneyStart, rowNumber), fmt.Sprintf("%s%d", columnOffset(moneyStart, 2), rowNumber), moneyStyle); err != nil {
			return err
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("%s%d", percentageColumn, rowNumber), fmt.Sprintf("%s%d", percentageColumn, rowNumber), percentageStyle); err != nil {
			return err
		}
	}
	lastColumn := "Q"
	if includeKind {
		lastColumn = "R"
	}
	_ = book.SetColWidth(sheet, "A", lastColumn, 18)
	_ = book.SetColWidth(sheet, "D", "F", 28)
	_ = book.SetColWidth(sheet, "I", "L", 28)
	return freezeHeader(book, sheet)
}

func writeAssets(book *excelize.File, rows []AssetRow, headerStyle, moneyStyle, percentageStyle int) error {
	headers := []string{
		"First Used Date", "Supplier", "Description", "Source Cost", "Source GST", "Claimable GST Credit",
		"Threshold-Test Cost", "Configured Threshold", "Business Use %", "Business-Use-Adjusted Amount",
		"Category ID", "Expense Category", "Source Type", "Source ID", "Capital Check", "Classification / Result",
	}
	if err := writeHeader(book, "Capital-Asset Review", headers, headerStyle); err != nil {
		return err
	}
	for index, row := range rows {
		values := []any{
			row.FirstUsedDate, row.Supplier, row.Description, optionalMoney(row.SourceCostCents), optionalMoney(row.SourceGSTCents),
			optionalMoney(row.ClaimableGSTCents), optionalMoney(row.ThresholdCostCents), money(row.ThresholdCents),
			optionalFloat(row.BusinessUsePercent), optionalMoney(row.BusinessAdjustedCents), row.CategoryID, row.CategoryName,
			row.SourceType, row.SourceID, row.CapitalCheck, row.Classification,
		}
		rowNumber := index + 2
		if err := writeRow(book, "Capital-Asset Review", rowNumber, values); err != nil {
			return err
		}
		if err := book.SetCellStyle("Capital-Asset Review", fmt.Sprintf("D%d", rowNumber), fmt.Sprintf("H%d", rowNumber), moneyStyle); err != nil {
			return err
		}
		if err := book.SetCellStyle("Capital-Asset Review", fmt.Sprintf("I%d", rowNumber), fmt.Sprintf("I%d", rowNumber), percentageStyle); err != nil {
			return err
		}
		if err := book.SetCellStyle("Capital-Asset Review", fmt.Sprintf("J%d", rowNumber), fmt.Sprintf("J%d", rowNumber), moneyStyle); err != nil {
			return err
		}
	}
	_ = book.SetColWidth("Capital-Asset Review", "A", "P", 20)
	_ = book.SetColWidth("Capital-Asset Review", "B", "C", 30)
	_ = book.SetColWidth("Capital-Asset Review", "L", "L", 28)
	_ = book.SetColWidth("Capital-Asset Review", "N", "P", 30)
	return freezeHeader(book, "Capital-Asset Review")
}

func writeGSTReconciliation(book *excelize.File, report Report, headerStyle, moneyStyle int) error {
	if err := writeHeader(book, "GST Reconciliation", []string{"Record Type", "Gross", "Net", "GST"}, headerStyle); err != nil {
		return err
	}
	rows := [][]any{
		{"Income", money(report.Summary.IncomeGrossCents), money(report.Summary.IncomeNetCents), money(report.Summary.IncomeGSTCents)},
		{"Expenses", money(report.Summary.ExpenseGrossCents), money(report.Summary.ExpenseNetCents), money(report.Summary.ExpenseGSTCents)},
		{"Net GST Position", "", "", money(report.Summary.NetGSTCents)},
	}
	for index, row := range rows {
		rowNumber := index + 2
		if err := writeRow(book, "GST Reconciliation", rowNumber, row); err != nil {
			return err
		}
		if err := book.SetCellStyle("GST Reconciliation", fmt.Sprintf("B%d", rowNumber), fmt.Sprintf("D%d", rowNumber), moneyStyle); err != nil {
			return err
		}
	}
	_ = book.SetColWidth("GST Reconciliation", "A", "D", 24)
	return freezeHeader(book, "GST Reconciliation")
}

func writeExceptions(book *excelize.File, rows []accounting.Exception, headerStyle int) error {
	if err := writeHeader(book, "Exceptions", []string{"Severity", "Date", "Source Type", "Source ID", "Message"}, headerStyle); err != nil {
		return err
	}
	for index, row := range rows {
		if err := writeRow(book, "Exceptions", index+2, []any{string(row.Severity), row.Date, row.SourceType, row.SourceID, row.Message}); err != nil {
			return err
		}
	}
	_ = book.SetColWidth("Exceptions", "A", "D", 20)
	_ = book.SetColWidth("Exceptions", "E", "E", 70)
	return freezeHeader(book, "Exceptions")
}

func writeHeader(book *excelize.File, sheet string, values []string, style int) error {
	row := make([]any, len(values))
	for index, value := range values {
		row[index] = value
	}
	if err := writeRow(book, sheet, 1, row); err != nil {
		return err
	}
	last, _ := excelize.CoordinatesToCellName(len(values), 1)
	return book.SetCellStyle(sheet, "A1", last, style)
}

func writeRow(book *excelize.File, sheet string, row int, values []any) error {
	cell, err := excelize.CoordinatesToCellName(1, row)
	if err != nil {
		return err
	}
	return book.SetSheetRow(sheet, cell, &values)
}

func freezeHeader(book *excelize.File, sheet string) error {
	return book.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
}

func optionalMoney(cents *int64) any {
	if cents == nil {
		return ""
	}
	return money(*cents)
}

func optionalFloat(value *float64) any {
	if value == nil {
		return ""
	}
	return *value
}

func money(cents int64) float64 {
	return float64(cents) / 100
}

func statusColour(status string) string {
	if status == "INCOMPLETE" {
		return "C00000"
	}
	return "008000"
}

func columnOffset(column string, offset int) string {
	number, _ := excelize.ColumnNameToNumber(column)
	name, _ := excelize.ColumnNumberToName(number + offset)
	return name
}
