package financial

import (
	"fmt"
	"strings"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/accounting"
	"github.com/xuri/excelize/v2"
)

func Workbook(report Report) ([]byte, error) {
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	if err := book.SetSheetName("Sheet1", "Summary"); err != nil {
		return nil, err
	}
	for _, sheet := range []string{
		"Income", "Expenses", "Payments", "Supplier Transactions", "Customers", "Vendors",
		"Products", "Projects-Jobs", "Accounting Facts", "Exceptions",
	} {
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
	statusStyle, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: statusColour(report.Status)}})
	if err != nil {
		return nil, err
	}
	writers := []func() error{
		func() error { return writeSummary(book, report, headerStyle, statusStyle) },
		func() error { return writeIncome(book, report.Invoices, headerStyle, moneyStyle) },
		func() error { return writeExpenses(book, report.Expenses, headerStyle, moneyStyle) },
		func() error { return writePayments(book, report.Payments, headerStyle, moneyStyle) },
		func() error {
			return writeSupplierTransactions(book, report.SupplierTransactions, headerStyle, moneyStyle)
		},
		func() error { return writeCustomers(book, report.Customers, headerStyle, moneyStyle) },
		func() error { return writeVendors(book, report.Vendors, headerStyle) },
		func() error { return writeProducts(book, report.Products, headerStyle, moneyStyle) },
		func() error { return writeProjects(book, report.Projects, headerStyle, moneyStyle) },
		func() error { return writeAccountingFacts(book, report, headerStyle, moneyStyle) },
		func() error { return writeExceptions(book, report.Exceptions, headerStyle) },
	}
	for _, write := range writers {
		if err := write(); err != nil {
			return nil, err
		}
	}
	buffer, err := book.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeSummary(book *excelize.File, report Report, headerStyle, statusStyle int) error {
	if err := writeHeader(book, "Summary", []string{"Field", "Value"}, headerStyle); err != nil {
		return err
	}
	start, end := "", ""
	if report.Selection.Start != nil {
		start = formatDate(*report.Selection.Start)
	}
	if report.Selection.End != nil {
		end = formatDate(*report.Selection.End)
	}
	fy := ""
	if report.Selection.FY != 0 {
		fy = fmt.Sprintf("FY%d", report.Selection.FY)
	}
	rows := [][]any{
		{"Financial Year", fy}, {"Range Start", start}, {"Range End", end},
		{"Generated Timestamp", report.GeneratedAt.Format("2006-01-02T15:04:05Z07:00")},
		{"Source", report.Source}, {"Report Status", report.Status},
		{"Company Currency ID", report.CompanyCurrencyID},
		{"Income Records", len(report.Invoices)}, {"Expense Records", len(report.Expenses)},
		{"Payment Records", len(report.Payments)}, {"Supplier Transaction Records", len(report.SupplierTransactions)},
		{"Customer Records", len(report.Customers)}, {"Vendor Records", len(report.Vendors)},
		{"Product Records", len(report.Products)}, {"Project/Job Records", len(report.Projects)},
	}
	for index, row := range rows {
		if err := writeRow(book, "Summary", index+2, row); err != nil {
			return err
		}
	}
	if err := book.SetCellStyle("Summary", "B7", "B7", statusStyle); err != nil {
		return err
	}
	_ = book.SetColWidth("Summary", "A", "A", 34)
	_ = book.SetColWidth("Summary", "B", "B", 30)
	return freezeHeader(book, "Summary")
}

func writeIncome(book *excelize.File, invoices []invoiceninja.Invoice, headerStyle, moneyStyle int) error {
	headers := []string{
		"Invoice ID", "Number", "Date", "Due Date", "Client ID", "Client", "Project ID", "Status ID",
		"Archived At", "Deleted", "Amount", "Total Tax", "Balance", "Paid To Date", "Discount", "Inclusive Tax",
		"PO Number", "Public Notes", "Private Notes", "Line Number", "Product Key", "Line Description",
		"Quantity", "Unit Cost", "Product Cost", "Discount", "Line Net", "Line Gross", "Line Tax", "Line Date",
		"Customer Currency ID", "Exchange Rate", "Invoice Tax Name 1", "Invoice Tax Rate 1", "Invoice Tax Name 2", "Invoice Tax Rate 2",
		"Invoice Tax Name 3", "Invoice Tax Rate 3", "Line Tax Name 1", "Line Tax Rate 1", "Line Tax Name 2", "Line Tax Rate 2",
		"Line Tax Name 3", "Line Tax Rate 3", "Invoice Discount Is Amount", "Line Discount Is Amount",
	}
	if err := writeHeader(book, "Income", headers, headerStyle); err != nil {
		return err
	}
	rowNumber := 2
	for _, invoice := range invoices {
		lines := invoice.LineItems
		if len(lines) == 0 {
			lines = []invoiceninja.LineItem{{}}
		}
		for index, line := range lines {
			values := []any{
				invoice.ID, invoice.Number, invoice.Date, invoice.DueDate, invoice.ClientID, clientName(invoice.Client), invoice.ProjectID,
				invoice.StatusID, invoice.ArchivedAt, invoice.IsDeleted, invoice.Amount, invoice.TotalTaxes, invoice.Balance,
				invoice.PaidToDate, invoice.Discount, invoice.UsesInclusiveTaxes, invoice.PO, invoice.PublicNotes, invoice.PrivateNotes,
				optionalRowNumber(index, invoice.LineItems), line.ProductKey, line.Notes, line.Quantity, line.Cost, line.ProductCost,
				line.Discount, line.LineTotal, line.GrossLineTotal, float64(line.TaxAmount), line.Date,
				clientCurrency(invoice.Client), invoice.ExchangeRate, invoice.TaxName1, invoice.TaxRate1, invoice.TaxName2, invoice.TaxRate2,
				invoice.TaxName3, invoice.TaxRate3, line.TaxName1, line.TaxRate1, line.TaxName2, line.TaxRate2, line.TaxName3, line.TaxRate3,
				invoice.IsAmountDiscount, line.IsAmountDiscount,
			}
			if err := writeRow(book, "Income", rowNumber, values); err != nil {
				return err
			}
			if err := book.SetCellStyle("Income", fmt.Sprintf("K%d", rowNumber), fmt.Sprintf("N%d", rowNumber), moneyStyle); err != nil {
				return err
			}
			if err := book.SetCellStyle("Income", fmt.Sprintf("X%d", rowNumber), fmt.Sprintf("Y%d", rowNumber), moneyStyle); err != nil {
				return err
			}
			if err := book.SetCellStyle("Income", fmt.Sprintf("AA%d", rowNumber), fmt.Sprintf("AC%d", rowNumber), moneyStyle); err != nil {
				return err
			}
			rowNumber++
		}
	}
	return finishSheet(book, "Income", "A", "AT")
}

func writeExpenses(book *excelize.File, expenses []invoiceninja.Expense, headerStyle, moneyStyle int) error {
	headers := []string{
		"Expense ID", "Number", "Date", "Payment Date", "Vendor ID", "Vendor", "Client ID", "Project ID", "Project",
		"Category ID", "Category", "Currency ID", "Amount", "Foreign Amount", "Exchange Rate", "Total Tax", "Tax Name 1",
		"Tax Rate 1", "Payment Type ID", "Transaction ID", "Transaction Reference", "Inclusive Tax", "Calculate Tax By Amount",
		"Archived At", "Deleted", "Public Notes", "Private Notes", "Custom Value 1", "Custom Value 2", "Business Use %", "Custom Value 4",
		"Tax Amount 1", "Tax Name 2", "Tax Rate 2", "Tax Amount 2", "Tax Name 3", "Tax Rate 3", "Tax Amount 3",
	}
	if err := writeHeader(book, "Expenses", headers, headerStyle); err != nil {
		return err
	}
	for index, expense := range expenses {
		rowNumber := index + 2
		values := []any{
			expense.ID, firstText(expense.Number, expense.TransactionReference), expense.Date, expense.PaymentDate,
			expense.VendorID, vendorName(expense.Vendor), expense.ClientID, expense.ProjectID, projectName(expense.Project),
			expense.CategoryID, categoryName(expense.Category), expense.CurrencyID, expense.Amount, expense.ForeignAmount,
			expense.ExchangeRate, expense.TaxAmount1 + expense.TaxAmount2 + expense.TaxAmount3, expense.TaxName1, expense.TaxRate1,
			expense.PaymentTypeID, expense.TransactionID, expense.TransactionReference, expense.UsesInclusiveTaxes,
			expense.CalculateTaxByAmount, expense.ArchivedAt, expense.IsDeleted, expense.PublicNotes, expense.PrivateNotes,
			expense.CustomValue1, expense.CustomValue2, expense.CustomValue3, expense.CustomValue4,
			expense.TaxAmount1, expense.TaxName2, expense.TaxRate2, expense.TaxAmount2, expense.TaxName3, expense.TaxRate3, expense.TaxAmount3,
		}
		if err := writeRow(book, "Expenses", rowNumber, values); err != nil {
			return err
		}
		if err := book.SetCellStyle("Expenses", fmt.Sprintf("M%d", rowNumber), fmt.Sprintf("N%d", rowNumber), moneyStyle); err != nil {
			return err
		}
		if err := book.SetCellStyle("Expenses", fmt.Sprintf("P%d", rowNumber), fmt.Sprintf("P%d", rowNumber), moneyStyle); err != nil {
			return err
		}
		for _, column := range []string{"AF", "AI", "AL"} {
			if err := book.SetCellStyle("Expenses", fmt.Sprintf("%s%d", column, rowNumber), fmt.Sprintf("%s%d", column, rowNumber), moneyStyle); err != nil {
				return err
			}
		}
	}
	return finishSheet(book, "Expenses", "A", "AL")
}

func writePayments(book *excelize.File, payments []invoiceninja.Payment, headerStyle, moneyStyle int) error {
	headers := []string{
		"Payment ID", "Number", "Date", "Client ID", "Client", "Amount", "Applied", "Refunded", "Transaction Reference",
		"Payment Type ID", "Exchange Currency ID", "Exchange Rate", "Category ID", "Manual", "Archived At", "Deleted",
		"Private Notes", "Related Type", "Related ID", "Related Number", "Paymentable ID", "Relationship Amount", "Relationship Refunded",
	}
	if err := writeHeader(book, "Payments", headers, headerStyle); err != nil {
		return err
	}
	rowNumber := 2
	for _, payment := range payments {
		relationships := paymentRelationships(payment)
		if len(relationships) == 0 {
			relationships = []paymentRelationship{{}}
		}
		for _, relationship := range relationships {
			values := []any{
				payment.ID, payment.Number, payment.Date, payment.ClientID, clientName(payment.Client), payment.Amount, payment.Applied,
				payment.Refunded, payment.TransactionReference, payment.PaymentTypeID, payment.ExchangeCurrencyID, payment.ExchangeRate,
				payment.CategoryID, payment.IsManual, payment.ArchivedAt, payment.IsDeleted, payment.PrivateNotes,
				relationship.kind, relationship.id, relationship.number, relationship.paymentableID, relationship.amount, relationship.refunded,
			}
			if err := writeRow(book, "Payments", rowNumber, values); err != nil {
				return err
			}
			if err := book.SetCellStyle("Payments", fmt.Sprintf("F%d", rowNumber), fmt.Sprintf("H%d", rowNumber), moneyStyle); err != nil {
				return err
			}
			if err := book.SetCellStyle("Payments", fmt.Sprintf("V%d", rowNumber), fmt.Sprintf("W%d", rowNumber), moneyStyle); err != nil {
				return err
			}
			rowNumber++
		}
	}
	return finishSheet(book, "Payments", "A", "W")
}

func writeSupplierTransactions(book *excelize.File, rows []invoiceninja.BankTransaction, headerStyle, moneyStyle int) error {
	headers := []string{
		"Transaction ID", "Date", "Bank Integration ID", "Bank Transaction Number", "Base Type", "Amount", "Currency ID",
		"Description", "Participant", "Participant Name", "Expense ID", "Vendor ID", "Status ID", "Archived At", "Deleted",
	}
	if err := writeHeader(book, "Supplier Transactions", headers, headerStyle); err != nil {
		return err
	}
	for index, row := range rows {
		rowNumber := index + 2
		if err := writeRow(book, "Supplier Transactions", rowNumber, []any{
			row.ID, row.Date, row.BankIntegrationID, row.TransactionID, row.BaseType, row.Amount, row.CurrencyID,
			row.Description, row.Participant, row.ParticipantName, row.ExpenseID, row.VendorID, row.StatusID, row.ArchivedAt, row.IsDeleted,
		}); err != nil {
			return err
		}
		if err := book.SetCellStyle("Supplier Transactions", fmt.Sprintf("F%d", rowNumber), fmt.Sprintf("F%d", rowNumber), moneyStyle); err != nil {
			return err
		}
	}
	return finishSheet(book, "Supplier Transactions", "A", "O")
}

func writeCustomers(book *excelize.File, customers []invoiceninja.ClientEntity, headerStyle, moneyStyle int) error {
	headers := []string{
		"Customer ID", "Number", "Name", "Display Name", "ID Number", "VAT Number", "Phone", "Website", "Address 1", "Address 2",
		"City", "State", "Postal Code", "Country ID", "Currency ID", "Balance", "Paid To Date", "Credit Balance", "Archived At", "Deleted",
		"Contact ID", "Contact First Name", "Contact Last Name", "Contact Email", "Contact Phone", "Primary Contact", "Contact Archived At", "Contact Deleted",
	}
	if err := writeHeader(book, "Customers", headers, headerStyle); err != nil {
		return err
	}
	rowNumber := 2
	for _, customer := range customers {
		contacts := customer.Contacts
		if len(contacts) == 0 {
			contacts = []invoiceninja.Contact{{}}
		}
		for _, contact := range contacts {
			if err := writeRow(book, "Customers", rowNumber, []any{
				customer.ID, customer.Number, customer.Name, customer.DisplayName, customer.IDNumber, customer.VATNumber,
				customer.Phone, customer.Website, customer.Address1, customer.Address2, customer.City, customer.State,
				customer.PostalCode, customer.CountryID, customer.Settings.CurrencyID, customer.Balance, customer.PaidToDate,
				customer.CreditBalance, customer.ArchivedAt, customer.IsDeleted, contact.ID, contact.FirstName, contact.LastName,
				contact.Email, contact.Phone, contact.IsPrimary, contact.ArchivedAt, contact.IsDeleted,
			}); err != nil {
				return err
			}
			if err := book.SetCellStyle("Customers", fmt.Sprintf("P%d", rowNumber), fmt.Sprintf("R%d", rowNumber), moneyStyle); err != nil {
				return err
			}
			rowNumber++
		}
	}
	return finishSheet(book, "Customers", "A", "AB")
}

func writeVendors(book *excelize.File, vendors []invoiceninja.Vendor, headerStyle int) error {
	headers := []string{"Vendor ID", "Number", "Name", "Display Name", "Currency ID", "Archived At", "Deleted", "Public Notes", "Private Notes", "Custom Value 1", "Custom Value 2", "Custom Value 3", "Custom Value 4"}
	if err := writeHeader(book, "Vendors", headers, headerStyle); err != nil {
		return err
	}
	for index, vendor := range vendors {
		if err := writeRow(book, "Vendors", index+2, []any{
			vendor.ID, vendor.Number, vendor.Name, vendor.DisplayName, vendor.CurrencyID, vendor.ArchivedAt, vendor.IsDeleted,
			vendor.PublicNotes, vendor.PrivateNotes, vendor.CustomValue1, vendor.CustomValue2, vendor.CustomValue3, vendor.CustomValue4,
		}); err != nil {
			return err
		}
	}
	return finishSheet(book, "Vendors", "A", "M")
}

func writeProducts(book *excelize.File, products []invoiceninja.Product, headerStyle, moneyStyle int) error {
	headers := []string{
		"Product ID", "Product Key", "Description", "Project ID", "Vendor ID", "Cost", "Price", "Quantity", "In Stock",
		"Max Quantity", "Tax Name 1", "Tax Rate 1", "Income Account ID", "Archived At", "Deleted",
		"Custom Value 1", "Custom Value 2", "Custom Value 3", "Custom Value 4", "Tax Name 2", "Tax Rate 2", "Tax Name 3", "Tax Rate 3",
	}
	if err := writeHeader(book, "Products", headers, headerStyle); err != nil {
		return err
	}
	for index, product := range products {
		rowNumber := index + 2
		if err := writeRow(book, "Products", rowNumber, []any{
			product.ID, product.ProductKey, product.Notes, product.ProjectID, product.VendorID, product.Cost, product.Price,
			product.Quantity, product.InStockQuantity, product.MaxQuantity, product.TaxName1, product.TaxRate1,
			product.IncomeAccountID, product.ArchivedAt, product.IsDeleted,
			product.CustomValue1, product.CustomValue2, product.CustomValue3, product.CustomValue4,
			product.TaxName2, product.TaxRate2, product.TaxName3, product.TaxRate3,
		}); err != nil {
			return err
		}
		if err := book.SetCellStyle("Products", fmt.Sprintf("F%d", rowNumber), fmt.Sprintf("G%d", rowNumber), moneyStyle); err != nil {
			return err
		}
	}
	return finishSheet(book, "Products", "A", "W")
}

func writeProjects(book *excelize.File, projects []invoiceninja.Project, headerStyle, moneyStyle int) error {
	headers := []string{
		"Project ID", "Number", "Name", "Customer ID", "Task Rate", "Budgeted Hours", "Budgeted Amount", "Archived At", "Deleted",
		"Public Notes", "Private Notes", "Custom Value 1", "Custom Value 2", "Custom Value 3", "Custom Value 4",
	}
	if err := writeHeader(book, "Projects-Jobs", headers, headerStyle); err != nil {
		return err
	}
	for index, project := range projects {
		rowNumber := index + 2
		if err := writeRow(book, "Projects-Jobs", rowNumber, []any{
			project.ID, project.Number, project.Name, project.ClientID, project.TaskRate, project.BudgetedHours,
			project.BudgetedAmount, project.ArchivedAt, project.IsDeleted, project.PublicNotes, project.PrivateNotes,
			project.CustomValue1, project.CustomValue2, project.CustomValue3, project.CustomValue4,
		}); err != nil {
			return err
		}
		if err := book.SetCellStyle("Projects-Jobs", fmt.Sprintf("E%d", rowNumber), fmt.Sprintf("E%d", rowNumber), moneyStyle); err != nil {
			return err
		}
		if err := book.SetCellStyle("Projects-Jobs", fmt.Sprintf("G%d", rowNumber), fmt.Sprintf("G%d", rowNumber), moneyStyle); err != nil {
			return err
		}
	}
	return finishSheet(book, "Projects-Jobs", "A", "O")
}

func writeAccountingFacts(book *excelize.File, report Report, headerStyle, moneyStyle int) error {
	headers := []string{
		"Fact Type", "Source Type", "Source ID", "Source Date", "Number", "Party ID", "Party", "Category ID", "Category",
		"Gross", "Net", "GST", "Business Use %", "Tax Known", "Payment Status", "Supplier Account", "Archived/Deleted",
		"Related Type", "Related ID", "Related Date", "Related Amount", "Source Currency ID", "Related Currency ID",
	}
	if err := writeHeader(book, "Accounting Facts", headers, headerStyle); err != nil {
		return err
	}
	rowNumber := 2
	for _, sale := range report.SalesFacts {
		allocations := sale.Payments
		if len(allocations) == 0 {
			allocations = []accounting.Allocation{{}}
		}
		for _, allocation := range allocations {
			if err := writeRow(book, "Accounting Facts", rowNumber, []any{
				"sale", "invoice", sale.SourceID, sale.Date, sale.Number, sale.CustomerID, sale.CustomerName, "", "",
				money(sale.Amounts.GrossCents), money(sale.Amounts.NetCents), money(sale.Amounts.GSTCents), "", sale.TaxKnown,
				"", false, sale.ArchivedOrDeleted, allocation.SourceType, allocation.SourceID, allocation.Date, money(allocation.AmountCents),
				sale.CurrencyID, allocation.CurrencyID,
			}); err != nil {
				return err
			}
			if err := styleFactMoney(book, rowNumber, moneyStyle); err != nil {
				return err
			}
			rowNumber++
		}
	}
	for _, purchase := range report.PurchaseFacts {
		allocations := purchase.Settlements
		if len(allocations) == 0 {
			allocations = []accounting.Allocation{{}}
		}
		businessUse := any("")
		if purchase.BusinessUsePercent != nil {
			businessUse = *purchase.BusinessUsePercent
		}
		for _, allocation := range allocations {
			if err := writeRow(book, "Accounting Facts", rowNumber, []any{
				"purchase", "expense", purchase.SourceID, purchase.Date, purchase.Number, purchase.VendorID, purchase.VendorName,
				purchase.CategoryID, purchase.CategoryName, money(purchase.Amounts.GrossCents), money(purchase.Amounts.NetCents),
				money(purchase.Amounts.GSTCents), businessUse, purchase.TaxKnown, string(purchase.PaymentStatus), purchase.SupplierAccount,
				purchase.ArchivedOrDeleted, allocation.SourceType, allocation.SourceID, allocation.Date, money(allocation.AmountCents),
				purchase.CurrencyID, allocation.CurrencyID,
			}); err != nil {
				return err
			}
			if err := styleFactMoney(book, rowNumber, moneyStyle); err != nil {
				return err
			}
			rowNumber++
		}
	}
	return finishSheet(book, "Accounting Facts", "A", "W")
}

func styleFactMoney(book *excelize.File, row, moneyStyle int) error {
	if err := book.SetCellStyle("Accounting Facts", fmt.Sprintf("J%d", row), fmt.Sprintf("L%d", row), moneyStyle); err != nil {
		return err
	}
	return book.SetCellStyle("Accounting Facts", fmt.Sprintf("U%d", row), fmt.Sprintf("U%d", row), moneyStyle)
}

func writeExceptions(book *excelize.File, exceptions []accounting.Exception, headerStyle int) error {
	headers := []string{"Severity", "Date", "Source Type", "Source ID", "Cash Basis Only", "Message"}
	if err := writeHeader(book, "Exceptions", headers, headerStyle); err != nil {
		return err
	}
	for index, exception := range exceptions {
		if err := writeRow(book, "Exceptions", index+2, []any{
			string(exception.Severity), exception.Date, exception.SourceType, exception.SourceID, exception.CashBasisOnly, exception.Message,
		}); err != nil {
			return err
		}
	}
	return finishSheet(book, "Exceptions", "A", "F")
}

type paymentRelationship struct {
	kind          string
	id            string
	number        string
	paymentableID string
	amount        any
	refunded      any
}

func paymentRelationships(payment invoiceninja.Payment) []paymentRelationship {
	allocations := append(invoiceninja.Paymentables(nil), payment.Paymentables...)
	seenAllocations := make(map[string]bool, len(allocations))
	for _, allocation := range allocations {
		seenAllocations[paymentableTargetKey(allocation)] = true
	}
	for _, allocation := range payment.InvoiceAllocations {
		key := paymentableTargetKey(allocation)
		if !seenAllocations[key] {
			allocations = append(allocations, allocation)
			seenAllocations[key] = true
		}
	}
	if len(allocations) == 0 && strings.TrimSpace(payment.InvoiceID) != "" {
		amount := payment.Applied
		if amount == 0 {
			amount = payment.Amount
		}
		allocations = invoiceninja.Paymentables{{InvoiceID: payment.InvoiceID, Amount: invoiceninja.FlexibleFloat(amount)}}
	}
	result := make([]paymentRelationship, 0, len(allocations)+len(payment.Credits))
	seenCredits := make(map[string]bool)
	for _, allocation := range allocations {
		kind, id := "invoice", allocation.InvoiceID
		if id == "" {
			kind, id = "credit", allocation.CreditID
			seenCredits[id] = id != ""
		}
		if id == "" {
			kind, id = "paymentable", allocation.ID
		}
		result = append(result, paymentRelationship{
			kind: kind, id: id, number: relatedInvoiceNumber(payment, allocation.InvoiceID), paymentableID: allocation.ID,
			amount: float64(allocation.Amount), refunded: float64(allocation.Refunded),
		})
	}
	if len(allocations) == 0 && strings.TrimSpace(payment.InvoiceID) == "" {
		for _, invoice := range payment.Invoices {
			if strings.TrimSpace(invoice.ID) == "" {
				continue
			}
			result = append(result, paymentRelationship{
				kind: "invoice", id: invoice.ID, number: invoice.Number, amount: "", refunded: "",
			})
		}
	}
	for _, credit := range payment.Credits {
		if seenCredits[credit.CreditID] {
			continue
		}
		result = append(result, paymentRelationship{kind: "credit", id: credit.CreditID, amount: float64(credit.Amount)})
	}
	return result
}

func paymentableTargetKey(allocation invoiceninja.Paymentable) string {
	return fmt.Sprintf("%s\x00%s\x00%g\x00%g", allocation.InvoiceID, allocation.CreditID, allocation.Amount, allocation.Refunded)
}

func relatedInvoiceNumber(payment invoiceninja.Payment, invoiceID string) string {
	for _, invoice := range payment.Invoices {
		if invoice.ID == invoiceID {
			return invoice.Number
		}
	}
	return ""
}

func optionalRowNumber(index int, lines []invoiceninja.LineItem) any {
	if len(lines) == 0 {
		return ""
	}
	return index + 1
}

func clientName(client *invoiceninja.ClientEntity) string {
	if client == nil {
		return ""
	}
	return firstText(client.DisplayName, client.Name)
}

func clientCurrency(client *invoiceninja.ClientEntity) string {
	if client == nil {
		return ""
	}
	return client.Settings.CurrencyID
}

func vendorName(vendor *invoiceninja.Vendor) string {
	if vendor == nil {
		return ""
	}
	return firstText(vendor.DisplayName, vendor.Name)
}

func projectName(project *invoiceninja.Project) string {
	if project == nil {
		return ""
	}
	return project.Name
}

func categoryName(category *invoiceninja.ExpenseCategory) string {
	if category == nil {
		return ""
	}
	return category.Name
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func writeHeader(book *excelize.File, sheet string, headers []string, style int) error {
	values := make([]any, len(headers))
	for index, header := range headers {
		values[index] = header
	}
	if err := writeRow(book, sheet, 1, values); err != nil {
		return err
	}
	last, _ := excelize.CoordinatesToCellName(len(headers), 1)
	return book.SetCellStyle(sheet, "A1", last, style)
}

func writeRow(book *excelize.File, sheet string, row int, values []any) error {
	cell, err := excelize.CoordinatesToCellName(1, row)
	if err != nil {
		return err
	}
	return book.SetSheetRow(sheet, cell, &values)
}

func finishSheet(book *excelize.File, sheet, first, last string) error {
	_ = book.SetColWidth(sheet, first, last, 18)
	return freezeHeader(book, sheet)
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
