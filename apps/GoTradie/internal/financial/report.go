package financial

import (
	"fmt"
	"sort"
	"strings"
	"time"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/accounting"
	"github.com/MickMake/GoTradie/internal/config"
)

type Report struct {
	Selection            Selection
	GeneratedAt          time.Time
	Source               string
	CompanyCurrencyID    string
	Status               string
	Invoices             []invoiceninja.Invoice
	Payments             []invoiceninja.Payment
	Expenses             []invoiceninja.Expense
	SupplierTransactions []invoiceninja.BankTransaction
	Customers            []invoiceninja.ClientEntity
	Vendors              []invoiceninja.Vendor
	Products             []invoiceninja.Product
	Projects             []invoiceninja.Project
	SalesFacts           []accounting.Sale
	PurchaseFacts        []accounting.Purchase
	Exceptions           []accounting.Exception
}

func Build(data SourceData, cfg config.BASConfig, options Options, generatedAt time.Time) (Report, error) {
	selection, err := ResolveSelection(cfg, options)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		Selection: selection, GeneratedAt: generatedAt, Source: data.Source,
		CompanyCurrencyID: data.Accounting.CompanyCurrencyID, Status: "COMPLETE",
		Customers: append([]invoiceninja.ClientEntity(nil), data.Customers...),
		Vendors:   append([]invoiceninja.Vendor(nil), data.Vendors...),
		Products:  append([]invoiceninja.Product(nil), data.Products...),
		Projects:  append([]invoiceninja.Project(nil), data.Projects...),
	}

	selectedInvoices := make(map[string]bool)
	for _, invoice := range data.Invoices {
		include, exception := includeTransactional(selection, "invoice", invoice.ID, invoice.Date)
		if exception != nil {
			report.Exceptions = append(report.Exceptions, *exception)
		}
		if include {
			report.Invoices = append(report.Invoices, invoice)
			selectedInvoices[invoice.ID] = true
		}
	}
	selectedPayments := make(map[string]bool)
	for _, payment := range data.Payments {
		include, exception := includeTransactional(selection, "payment", payment.ID, payment.Date)
		if exception != nil {
			report.Exceptions = append(report.Exceptions, *exception)
		}
		if include {
			report.Payments = append(report.Payments, payment)
			selectedPayments[payment.ID] = true
		}
	}
	selectedExpenses := make(map[string]bool)
	for _, expense := range data.Expenses {
		include, exception := includeTransactional(selection, "expense", expense.ID, expense.Date)
		if exception != nil {
			report.Exceptions = append(report.Exceptions, *exception)
		}
		if include {
			report.Expenses = append(report.Expenses, expense)
			selectedExpenses[expense.ID] = true
		}
	}
	selectedTransactions := make(map[string]bool)
	for _, transaction := range data.SupplierTransactions {
		include, exception := includeTransactional(selection, "bank_transaction", transaction.ID, transaction.Date)
		if exception != nil {
			report.Exceptions = append(report.Exceptions, *exception)
		}
		if include {
			report.SupplierTransactions = append(report.SupplierTransactions, transaction)
			selectedTransactions[transaction.ID] = true
		}
	}

	for _, sale := range data.Accounting.Sales {
		if selectedInvoices[sale.SourceID] {
			report.SalesFacts = append(report.SalesFacts, sale)
		}
	}
	for _, purchase := range data.Accounting.Purchases {
		if selectedExpenses[purchase.SourceID] {
			report.PurchaseFacts = append(report.PurchaseFacts, purchase)
		}
	}
	for _, exception := range data.Accounting.Exceptions {
		if includeAccountingException(exception, selectedInvoices, selectedPayments, selectedExpenses, selectedTransactions) {
			report.Exceptions = append(report.Exceptions, exception)
		}
	}

	sortReport(&report)
	report.Exceptions = uniqueExceptions(report.Exceptions)
	sortExceptions(report.Exceptions)
	for _, exception := range report.Exceptions {
		if exception.Severity == accounting.SeverityError {
			report.Status = "INCOMPLETE"
			break
		}
	}
	return report, nil
}

func includeTransactional(selection Selection, sourceType, sourceID, dateText string) (bool, *accounting.Exception) {
	if !selection.HasDateFilter() {
		return true, nil
	}
	dateText = strings.TrimSpace(dateText)
	if dateText == "" {
		return true, &accounting.Exception{
			Severity: accounting.SeverityError, SourceType: sourceType, SourceID: sourceID,
			Message: "missing source date required for Financial export filtering",
		}
	}
	date, err := time.ParseInLocation("2006-01-02", dateText, time.Local)
	if err != nil {
		return true, &accounting.Exception{
			Severity: accounting.SeverityError, SourceType: sourceType, SourceID: sourceID, Date: dateText,
			Message: fmt.Sprintf("invalid source date %q required for Financial export filtering", dateText),
		}
	}
	return selection.Includes(date), nil
}

func includeAccountingException(
	exception accounting.Exception,
	invoices, payments, expenses, transactions map[string]bool,
) bool {
	switch exception.SourceType {
	case "invoice":
		return invoices[exception.SourceID]
	case "payment":
		return payments[exception.SourceID]
	case "expense":
		return expenses[exception.SourceID]
	case "bank_transaction":
		return transactions[exception.SourceID]
	default:
		return true
	}
}

func sortReport(report *Report) {
	sort.SliceStable(report.Invoices, func(i, j int) bool {
		return datedIDLess(report.Invoices[i].Date, report.Invoices[i].ID, report.Invoices[j].Date, report.Invoices[j].ID)
	})
	sort.SliceStable(report.Payments, func(i, j int) bool {
		return datedIDLess(report.Payments[i].Date, report.Payments[i].ID, report.Payments[j].Date, report.Payments[j].ID)
	})
	sort.SliceStable(report.Expenses, func(i, j int) bool {
		return datedIDLess(report.Expenses[i].Date, report.Expenses[i].ID, report.Expenses[j].Date, report.Expenses[j].ID)
	})
	sort.SliceStable(report.SupplierTransactions, func(i, j int) bool {
		return datedIDLess(report.SupplierTransactions[i].Date, report.SupplierTransactions[i].ID, report.SupplierTransactions[j].Date, report.SupplierTransactions[j].ID)
	})
	sort.SliceStable(report.Customers, func(i, j int) bool {
		return namedIDLess(report.Customers[i].Name, report.Customers[i].ID, report.Customers[j].Name, report.Customers[j].ID)
	})
	sort.SliceStable(report.Vendors, func(i, j int) bool {
		return namedIDLess(report.Vendors[i].Name, report.Vendors[i].ID, report.Vendors[j].Name, report.Vendors[j].ID)
	})
	sort.SliceStable(report.Products, func(i, j int) bool {
		return namedIDLess(report.Products[i].ProductKey, report.Products[i].ID, report.Products[j].ProductKey, report.Products[j].ID)
	})
	sort.SliceStable(report.Projects, func(i, j int) bool {
		return namedIDLess(report.Projects[i].Name, report.Projects[i].ID, report.Projects[j].Name, report.Projects[j].ID)
	})
	sort.SliceStable(report.SalesFacts, func(i, j int) bool {
		return datedIDLess(report.SalesFacts[i].Date, report.SalesFacts[i].SourceID, report.SalesFacts[j].Date, report.SalesFacts[j].SourceID)
	})
	sort.SliceStable(report.PurchaseFacts, func(i, j int) bool {
		return datedIDLess(report.PurchaseFacts[i].Date, report.PurchaseFacts[i].SourceID, report.PurchaseFacts[j].Date, report.PurchaseFacts[j].SourceID)
	})
}

func datedIDLess(firstDate, firstID, secondDate, secondID string) bool {
	if firstDate != secondDate {
		return firstDate < secondDate
	}
	return firstID < secondID
}

func namedIDLess(firstName, firstID, secondName, secondID string) bool {
	if firstName != secondName {
		return firstName < secondName
	}
	return firstID < secondID
}

func uniqueExceptions(exceptions []accounting.Exception) []accounting.Exception {
	seen := make(map[string]bool, len(exceptions))
	result := make([]accounting.Exception, 0, len(exceptions))
	for _, exception := range exceptions {
		key := strings.Join([]string{
			string(exception.Severity), exception.SourceType, exception.SourceID, exception.Date,
			exception.Message, fmt.Sprint(exception.CashBasisOnly),
		}, "\x00")
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
		if exceptions[i].SourceType != exceptions[j].SourceType {
			return exceptions[i].SourceType < exceptions[j].SourceType
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
