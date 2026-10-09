package bas

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

func Workbook(report Report) ([]byte, error) {
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	if err := book.SetSheetName("Sheet1", "Summary"); err != nil {
		return nil, err
	}
	for _, sheet := range []string{"Sales", "Purchases", "Exceptions"} {
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
	statusStyle, err := book.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: statusColour(report.Status)},
	})
	if err != nil {
		return nil, err
	}

	if err := writeSummary(book, report, headerStyle, moneyStyle, statusStyle); err != nil {
		return nil, err
	}
	if err := writeEvents(book, "Sales", report.Sales, headerStyle, moneyStyle); err != nil {
		return nil, err
	}
	if err := writeEvents(book, "Purchases", report.Purchases, headerStyle, moneyStyle); err != nil {
		return nil, err
	}
	if err := writeExceptions(book, report.Exceptions, headerStyle); err != nil {
		return nil, err
	}

	buffer, err := book.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeSummary(book *excelize.File, report Report, headerStyle, moneyStyle, statusStyle int) error {
	headers := []string{
		"Financial Year", "Reporting Period Type", "Reporting Period", "Reporting Period Number",
		"Period Start", "Period End", "GST Basis", "Generated Timestamp", "Source", "Report Status",
		"G1", "1A", "1B", "Net GST Position",
	}
	if err := writeHeader(book, "Summary", headers, headerStyle); err != nil {
		return err
	}
	for index, summary := range report.Summaries {
		row := index + 2
		periodNumber := any(summary.Period.Number)
		if summary.Period.ReportingPeriod == "yearly" {
			periodNumber = ""
		}
		values := []any{
			fmt.Sprintf("FY%d", summary.Period.FY), summary.Period.ReportingPeriod, summary.Period.Key, periodNumber,
			summary.Period.Start.Format("2006-01-02"), summary.Period.End.Format("2006-01-02"), summary.GSTBasis,
			summary.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"), summary.Source, summary.Status,
			money(summary.G1Cents), money(summary.OneACents), money(summary.OneBCents), money(summary.NetGSTCents),
		}
		if err := writeRow(book, "Summary", row, values); err != nil {
			return err
		}
		if err := book.SetCellStyle("Summary", fmt.Sprintf("J%d", row), fmt.Sprintf("J%d", row), statusStyle); err != nil {
			return err
		}
		if err := book.SetCellStyle("Summary", fmt.Sprintf("K%d", row), fmt.Sprintf("N%d", row), moneyStyle); err != nil {
			return err
		}
	}
	_ = book.SetColWidth("Summary", "A", "N", 18)
	_ = book.SetColWidth("Summary", "H", "H", 26)
	return freezeHeader(book, "Summary")
}

func writeEvents(book *excelize.File, sheet string, rows []EventRow, headerStyle, moneyStyle int) error {
	headers := []string{
		"Period", "Recognition Date", "Source Type", "Source ID", "Related Source Type", "Related Source ID",
		"Number", "Party ID", "Party", "Gross", "Net", "GST", "Business Use %",
	}
	if err := writeHeader(book, sheet, headers, headerStyle); err != nil {
		return err
	}
	for index, row := range rows {
		businessUse := any("")
		if row.Event.BusinessUsePercent != nil {
			businessUse = *row.Event.BusinessUsePercent
		}
		values := []any{
			row.Period, row.Event.Date, row.Event.SourceType, row.Event.SourceID,
			row.Event.RelatedSourceType, row.Event.RelatedSourceID, row.Event.Number,
			row.Event.PartyID, row.Event.PartyName, money(row.Event.Amounts.GrossCents),
			money(row.Event.Amounts.NetCents), money(row.Event.Amounts.GSTCents), businessUse,
		}
		rowNumber := index + 2
		if err := writeRow(book, sheet, rowNumber, values); err != nil {
			return err
		}
		if err := book.SetCellStyle(sheet, fmt.Sprintf("J%d", rowNumber), fmt.Sprintf("L%d", rowNumber), moneyStyle); err != nil {
			return err
		}
	}
	_ = book.SetColWidth(sheet, "A", "M", 18)
	_ = book.SetColWidth(sheet, "D", "F", 28)
	_ = book.SetColWidth(sheet, "I", "I", 28)
	return freezeHeader(book, sheet)
}

func writeExceptions(book *excelize.File, rows []ExceptionRow, headerStyle int) error {
	headers := []string{"Severity", "Period", "Date", "Source Type", "Source ID", "Message"}
	if err := writeHeader(book, "Exceptions", headers, headerStyle); err != nil {
		return err
	}
	for index, row := range rows {
		if err := writeRow(book, "Exceptions", index+2, []any{
			string(row.Exception.Severity), row.Period, row.Exception.Date, row.Exception.SourceType, row.Exception.SourceID, row.Exception.Message,
		}); err != nil {
			return err
		}
	}
	_ = book.SetColWidth("Exceptions", "A", "E", 20)
	_ = book.SetColWidth("Exceptions", "F", "F", 70)
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

func money(cents int64) float64 {
	return float64(cents) / 100
}

func statusColour(status string) string {
	if status == "INCOMPLETE" {
		return "C00000"
	}
	return "008000"
}
