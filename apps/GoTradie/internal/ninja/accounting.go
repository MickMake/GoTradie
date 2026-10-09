package ninja

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/accounting"
)

func (s *Service) BuildAccountingDataset(ctx context.Context) (accounting.Dataset, error) {
	company, err := s.client.Companies.Current(ctx)
	if err != nil {
		return accounting.Dataset{}, fmt.Errorf("load Invoice Ninja company: %w", err)
	}
	invoices, err := s.client.Invoices.ListAll(ctx, invoiceninja.InvoiceQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Include: []string{"client"}, Status: "active,archived,deleted"},
		WithTrashed: true,
	})
	if err != nil {
		return accounting.Dataset{}, fmt.Errorf("list Invoice Ninja invoices: %w", err)
	}
	payments, err := s.client.Payments.ListAll(ctx, invoiceninja.PaymentQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Include: []string{"client", "invoices"}, Status: "active,archived,deleted"},
		WithTrashed: true,
	})
	if err != nil {
		return accounting.Dataset{}, fmt.Errorf("list Invoice Ninja payments: %w", err)
	}
	expenses, err := s.client.Expenses.ListAll(ctx, invoiceninja.ExpenseQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Include: []string{"vendor", "category"}, Status: "active,archived,deleted"},
		WithTrashed: true,
	})
	if err != nil {
		return accounting.Dataset{}, fmt.Errorf("list Invoice Ninja expenses: %w", err)
	}
	transactions, err := s.client.BankTransactions.ListAll(ctx, invoiceninja.BankTransactionQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Status: "active,archived,deleted"},
		WithTrashed: true,
	})
	if err != nil {
		return accounting.Dataset{}, fmt.Errorf("list Invoice Ninja bank transactions: %w", err)
	}

	dataset := accounting.Dataset{Source: "Invoice Ninja", CompanyCurrencyID: strings.TrimSpace(company.Settings.CurrencyID)}
	if dataset.CompanyCurrencyID == "" {
		dataset.Exceptions = append(dataset.Exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: "company", SourceID: company.ID, Message: "Invoice Ninja company currency is missing"})
	}
	dataset.Sales, dataset.Exceptions = buildSalesFacts(invoices, payments, dataset.Exceptions)
	dataset.Purchases, dataset.Exceptions = buildPurchaseFacts(expenses, transactions, dataset.Exceptions)
	return dataset, nil
}

func buildSalesFacts(invoices []invoiceninja.Invoice, payments []invoiceninja.Payment, exceptions []accounting.Exception) ([]accounting.Sale, []accounting.Exception) {
	sales := make([]accounting.Sale, 0, len(invoices))
	byID := make(map[string]int, len(invoices))
	for _, invoice := range invoices {
		if invoice.StatusID == "1" {
			continue
		}
		if invoice.StatusID != "" && invoice.StatusID != "2" && invoice.StatusID != "3" && invoice.StatusID != "4" {
			exceptions = append(exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: "invoice", SourceID: invoice.ID, Date: invoice.Date, Message: fmt.Sprintf("unsupported invoice status_id %q", invoice.StatusID)})
			continue
		}
		customerName, currencyID := "", ""
		if invoice.Client != nil {
			customerName = firstText(invoice.Client.DisplayName, invoice.Client.Name)
			currencyID = strings.TrimSpace(invoice.Client.Settings.CurrencyID)
		}
		gst := moneyCents(invoice.TotalTaxes)
		gross := moneyCents(invoice.Amount)
		sale := accounting.Sale{
			SourceID: invoice.ID, Number: invoice.Number, CustomerID: invoice.ClientID,
			CustomerName: customerName, CurrencyID: currencyID, Date: invoice.Date,
			Amounts:  accounting.Amounts{GrossCents: gross, NetCents: gross - gst, GSTCents: gst},
			TaxKnown: invoiceTaxKnown(invoice), ArchivedOrDeleted: invoice.IsDeleted || invoice.ArchivedAt != 0,
		}
		byID[invoice.ID] = len(sales)
		sales = append(sales, sale)
	}

	for _, payment := range payments {
		allocations := invoicePaymentAllocations(payment)
		if payment.Refunded != 0 {
			exceptions = append(exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: "payment", SourceID: payment.ID, Message: "customer payment refund has no dated allocation suitable for BAS calculation", CashBasisOnly: true})
		}
		if len(allocations) == 0 && moneyCents(payment.Applied) != 0 {
			exceptions = append(exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: "payment", SourceID: payment.ID, Date: payment.Date, Message: "applied customer payment has no invoice allocation", CashBasisOnly: true})
		}
		for _, allocation := range allocations {
			index, ok := byID[allocation.InvoiceID]
			if !ok {
				exceptions = append(exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: "payment", SourceID: payment.ID, Date: payment.Date, Message: fmt.Sprintf("invoice allocation references unavailable invoice %q", allocation.InvoiceID), CashBasisOnly: true})
				continue
			}
			sales[index].Payments = append(sales[index].Payments, accounting.Allocation{
				SourceType: "payment", SourceID: payment.ID, Date: payment.Date,
				AmountCents: moneyCents(float64(allocation.Amount)), CurrencyID: payment.ExchangeCurrencyID,
				ArchivedOrDeleted: payment.IsDeleted || payment.ArchivedAt != 0,
			})
		}
	}
	return sales, exceptions
}

func invoicePaymentAllocations(payment invoiceninja.Payment) invoiceninja.Paymentables {
	if allocations := invoiceOnlyAllocations(payment.Paymentables); len(allocations) > 0 {
		return allocations
	}
	if allocations := invoiceOnlyAllocations(payment.InvoiceAllocations); len(allocations) > 0 {
		return allocations
	}
	if strings.TrimSpace(payment.InvoiceID) == "" {
		return nil
	}
	amount := payment.Applied
	if amount == 0 {
		amount = payment.Amount
	}
	return invoiceninja.Paymentables{{InvoiceID: payment.InvoiceID, Amount: invoiceninja.FlexibleFloat(amount)}}
}

func invoiceOnlyAllocations(allocations invoiceninja.Paymentables) invoiceninja.Paymentables {
	result := make(invoiceninja.Paymentables, 0, len(allocations))
	for _, allocation := range allocations {
		if strings.TrimSpace(allocation.InvoiceID) != "" {
			result = append(result, allocation)
		}
	}
	return result
}

func invoiceTaxKnown(invoice invoiceninja.Invoice) bool {
	if invoice.TotalTaxes != 0 || invoice.TaxName1 != "" || invoice.TaxName2 != "" || invoice.TaxName3 != "" {
		return true
	}
	for _, line := range invoice.LineItems {
		if float64(line.TaxAmount) != 0 || line.TaxName1 != "" || line.TaxName2 != "" || line.TaxName3 != "" {
			return true
		}
	}
	return false
}

func buildPurchaseFacts(expenses []invoiceninja.Expense, transactions []invoiceninja.BankTransaction, exceptions []accounting.Exception) ([]accounting.Purchase, []accounting.Exception) {
	purchases := make([]accounting.Purchase, 0, len(expenses))
	settlementPurchases := make(map[string][]*settlementPurchase)
	purchaseIndex := make(map[*settlementPurchase]int)
	legacyUnpaid := make([]struct {
		expenseID string
		supplier  string
		date      string
	}, 0)

	for _, expense := range expenses {
		gst := moneyCents(expense.TaxAmount1 + expense.TaxAmount2 + expense.TaxAmount3)
		gross := moneyCents(expense.Amount)
		notes := privateNoteValues(expense.PrivateNotes)
		supplierMarker := supplierAccountMarkerFromText(expense.PrivateNotes)
		purchaseMarker := settlementPurchaseMarkerFromText(expense.PrivateNotes)
		supplierAccount := supplierMarker != "" || purchaseMarker != ""
		paymentStatus := expensePaymentStatus(expense)
		vendorName := ""
		if expense.Vendor != nil {
			vendorName = firstText(expense.Vendor.DisplayName, expense.Vendor.Name)
		}
		categoryName := ""
		if expense.Category != nil {
			categoryName = strings.TrimSpace(expense.Category.Name)
		}
		sourceGross, sourceGrossErr := optionalExpenseMoneyCents(notes["Source total inc GST"])
		sourceGST, sourceGSTErr := optionalExpenseMoneyCents(notes["Source GST"])
		var sourceEvidenceErrors []string
		if sourceGrossErr != nil {
			sourceEvidenceErrors = append(sourceEvidenceErrors, "invalid Source total inc GST: "+sourceGrossErr.Error())
		}
		if sourceGSTErr != nil {
			sourceEvidenceErrors = append(sourceEvidenceErrors, "invalid Source GST: "+sourceGSTErr.Error())
		}
		businessUse, businessUseErr := parseBusinessUse(expense.CustomValue3)
		if businessUseErr != nil {
			exceptions = append(exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: "expense", SourceID: expense.ID, Date: expense.Date, Message: businessUseErr.Error()})
		}
		purchase := accounting.Purchase{
			SourceID: expense.ID, Number: firstText(expense.Number, expense.TransactionReference),
			VendorID: expense.VendorID, VendorName: vendorName,
			CategoryID: expense.CategoryID, CategoryName: categoryName,
			Description: firstText(notes["Item description"], expense.PublicNotes), CapitalCheck: notes["Capital check"],
			SourceGrossCents: sourceGross, SourceGSTCents: sourceGST, SourceEvidenceError: strings.Join(sourceEvidenceErrors, "; "),
			CurrencyID: expense.CurrencyID,
			Date:       expense.Date, PaymentDate: expense.PaymentDate, PaymentStatus: paymentStatus,
			Amounts:            accounting.Amounts{GrossCents: gross, NetCents: gross - gst, GSTCents: gst},
			BusinessUsePercent: businessUse, TaxKnown: expenseTaxKnown(expense), SupplierAccount: supplierAccount,
			ArchivedOrDeleted: expense.IsDeleted || expense.ArchivedAt != 0,
		}
		index := len(purchases)
		purchases = append(purchases, purchase)

		if supplierAccount {
			if expense.IsDeleted || expense.ArchivedAt != 0 {
				exceptions = append(exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: "expense", SourceID: expense.ID, Message: "archived or deleted marked supplier-account expense affects settlement reconstruction", CashBasisOnly: true})
				continue
			}
			if supplierMarker == "" || purchaseMarker == "" {
				exceptions = append(exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: "expense", SourceID: expense.ID, Message: "supplier-account expense is missing a durable supplier or purchase marker", CashBasisOnly: true})
				continue
			}
			total, parseErr := parseExpenseMoney(notes["Source total inc GST"])
			settlementGross := moneyCents(total)
			purchases[index].SettlementBaseCents = settlementGross
			if parseErr != nil || settlementGross == 0 {
				exceptions = append(exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: "expense", SourceID: expense.ID, Message: "marked supplier-account expense has no valid non-zero Source total inc GST", CashBasisOnly: true})
				continue
			}
			expenseCopy := expense
			record := &settlementPurchase{
				expense: &expenseCopy, supplierAccount: supplierMarker, sourceID: purchaseMarker,
				date: expense.Date, grossCents: settlementGross, remainingCents: max(settlementGross, int64(0)),
			}
			settlementPurchases[purchaseMarker] = append(settlementPurchases[purchaseMarker], record)
			purchaseIndex[record] = index
			continue
		}
		if sourceMarkerFromNotes(expense.PrivateNotes) != "" && paymentStatus == accounting.PaymentStatusUnpaid && !expense.IsDeleted && expense.ArchivedAt == 0 {
			legacyUnpaid = append(legacyUnpaid, struct {
				expenseID string
				supplier  string
				date      string
			}{expenseID: expense.ID, supplier: vendorName, date: expense.Date})
		}
	}

	var validPurchases []*settlementPurchase
	for marker, records := range settlementPurchases {
		if len(records) == 1 {
			validPurchases = append(validPurchases, records[0])
			continue
		}
		for _, record := range records {
			exceptions = append(exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: "expense", SourceID: record.expense.ID, Message: fmt.Sprintf("supplier purchase marker %q is duplicated", marker), CashBasisOnly: true})
		}
	}

	settlementPayments := make(map[string][]*settlementPayment)
	for _, transaction := range transactions {
		paymentMarker := accountPaymentMarkerFromText(transaction.Description)
		supplierMarker := supplierAccountMarkerFromText(transaction.Description)
		if paymentMarker == "" && supplierMarker == "" {
			continue
		}
		if transaction.IsDeleted || transaction.ArchivedAt != 0 {
			exceptions = append(exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: "bank_transaction", SourceID: transaction.ID, Message: "archived or deleted marked supplier settlement affects reconstruction", CashBasisOnly: true})
			continue
		}
		if paymentMarker == "" || supplierMarker == "" {
			exceptions = append(exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: "bank_transaction", SourceID: transaction.ID, Message: "marked supplier settlement is missing a durable payment or supplier marker", CashBasisOnly: true})
			continue
		}
		date, amountCents, durableErr := durableSupplierSettlementValues(transaction)
		if durableErr != nil {
			exceptions = append(exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: "bank_transaction", SourceID: transaction.ID, Message: durableErr.Error(), CashBasisOnly: true})
			continue
		}
		transactionCopy := transaction
		settlementPayments[paymentMarker] = append(settlementPayments[paymentMarker], &settlementPayment{
			transaction: &transactionCopy, supplierAccount: supplierMarker, sourceID: paymentMarker,
			date: date, amountCents: amountCents,
		})
	}

	var validPayments []*settlementPayment
	for marker, records := range settlementPayments {
		if len(records) == 1 {
			validPayments = append(validPayments, records[0])
			continue
		}
		for _, record := range records {
			exceptions = append(exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: "bank_transaction", SourceID: record.transaction.ID, Message: fmt.Sprintf("supplier payment marker %q is duplicated", marker), CashBasisOnly: true})
		}
	}

	for _, legacy := range legacyUnpaid {
		for _, payment := range validPayments {
			if legacy.date > payment.date {
				continue
			}
			if legacy.supplier != "" && supplierAccountMarker(legacy.supplier) != payment.supplierAccount {
				continue
			}
			exceptions = append(exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: "expense", SourceID: legacy.expenseID, Message: "older unpaid imported expense has no durable supplier-account marker", CashBasisOnly: true})
			payment.amountCents = 0
		}
	}

	allocateSettlementRecords(validPurchases, validPayments)
	transactionByMarker := make(map[string]invoiceninja.BankTransaction, len(validPayments))
	for _, payment := range validPayments {
		if payment.transaction != nil {
			transactionByMarker[payment.sourceID] = *payment.transaction
		}
		if payment.unappliedCents > 0 && payment.transaction != nil {
			exceptions = append(exceptions, accounting.Exception{Severity: accounting.SeverityError, SourceType: "bank_transaction", SourceID: payment.transaction.ID, Message: fmt.Sprintf("unapplied supplier payment amount %.2f is required for calculation", centsAmount(payment.unappliedCents)), CashBasisOnly: true})
		}
	}
	for _, purchase := range validPurchases {
		index := purchaseIndex[purchase]
		for _, allocation := range purchase.allocations {
			transaction := transactionByMarker[allocation.PaymentSourceID]
			purchases[index].Settlements = append(purchases[index].Settlements, accounting.Allocation{
				SourceType: "bank_transaction", SourceID: transaction.ID, Date: allocation.PaymentDate,
				AmountCents: moneyCents(allocation.Amount), CurrencyID: transaction.CurrencyID,
			})
		}
	}

	sort.SliceStable(purchases, func(i, j int) bool {
		if purchases[i].Date != purchases[j].Date {
			return purchases[i].Date < purchases[j].Date
		}
		return purchases[i].SourceID < purchases[j].SourceID
	})
	return purchases, exceptions
}

func expensePaymentStatus(expense invoiceninja.Expense) accounting.PaymentStatus {
	if strings.TrimSpace(expense.PaymentDate) != "" || strings.TrimSpace(expense.PaymentTypeID) != "" || strings.TrimSpace(expense.TransactionID) != "" {
		return accounting.PaymentStatusPaid
	}
	return accounting.PaymentStatusUnpaid
}

func expenseTaxKnown(expense invoiceninja.Expense) bool {
	return expense.TaxAmount1 != 0 || expense.TaxAmount2 != 0 || expense.TaxAmount3 != 0 ||
		expense.TaxName1 != "" || expense.TaxName2 != "" || expense.TaxName3 != "" ||
		expense.CustomValue1 != "" || expense.CustomValue4 != ""
}

func parseBusinessUse(value string) (*float64, error) {
	value = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "%"))
	if value == "" {
		return nil, nil
	}
	percentage, err := strconv.ParseFloat(value, 64)
	if err != nil || percentage < 0 || percentage > 100 {
		return nil, fmt.Errorf("invalid business-use percentage %q", value)
	}
	return &percentage, nil
}

func optionalExpenseMoneyCents(value string) (*int64, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	amount, err := parseExpenseMoney(value)
	if err != nil {
		return nil, err
	}
	cents := moneyCents(amount)
	return &cents, nil
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
