package ninja

import (
	"context"
	"fmt"
	"sort"
	"strings"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
)

const (
	supplierAccountMarkerPrefix    = "[GoTradie supplier-account:v1:"
	settlementPurchaseMarkerPrefix = "[GoTradie supplier-purchase:v1:"
)

type expenseSettlementAction struct {
	expense       invoiceninja.Expense
	sourceID      string
	desiredDate   string
	desiredTypeID string
	allocations   []ExpensePaymentAllocation
}

type settlementPurchase struct {
	row             *preparedExpenseImportRow
	expense         *invoiceninja.Expense
	supplierAccount string
	sourceID        string
	date            string
	grossCents      int64
	remainingCents  int64
	creditApplied   bool
	settledDate     string
	allocations     []ExpensePaymentAllocation
}

type settlementPayment struct {
	row             *preparedExpenseImportRow
	transaction     *invoiceninja.BankTransaction
	supplierAccount string
	sourceID        string
	date            string
	amountCents     int64
	paymentType     string
	paymentTypeID   string
	reference       string
	allocations     []ExpensePaymentAllocation
	unappliedCents  int64
}

func expenseCSVNeedsSettlement(records [][]string, idx map[string]int) bool {
	for _, rec := range records {
		if strings.TrimSpace(cell(rec, idx, "Document Type")) == documentTypeAccountPayment {
			return true
		}
	}
	return false
}

func (s *Service) loadSupplierSettlementState(ctx context.Context, state *expenseImportState) error {
	integrations, err := s.client.BankIntegrations.ListAll(ctx, invoiceninja.BankIntegrationQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Status: "active,archived,deleted"},
		WithTrashed: true,
	})
	if err != nil {
		return fmt.Errorf("list Invoice Ninja bank accounts: %w", err)
	}

	var named, active []invoiceninja.BankIntegration
	for _, integration := range integrations {
		if integration.BankAccountName != goTradieBankAccountName {
			continue
		}
		named = append(named, integration)
		if !integration.IsDeleted && integration.ArchivedAt == 0 {
			active = append(active, integration)
		}
	}
	if len(active) == 0 {
		if len(named) > 0 {
			return fmt.Errorf("Invoice Ninja bank account %q exists but is archived or deleted; restore one active manual account with that exact name", goTradieBankAccountName)
		}
		return fmt.Errorf("Invoice Ninja manual bank account %q was not found; create it in Invoice Ninja with sync disabled before importing supplier Account Payments", goTradieBankAccountName)
	}
	if len(active) != 1 {
		return fmt.Errorf("Invoice Ninja has %d active bank accounts named %q; keep exactly one manual account with that exact name", len(active), goTradieBankAccountName)
	}
	integration := active[0]
	if strings.TrimSpace(integration.IntegrationType) != "" {
		return fmt.Errorf("Invoice Ninja bank account %q is remote-backed (%s); create a manual account with that exact name instead", goTradieBankAccountName, integration.IntegrationType)
	}
	if integration.AutoSync {
		return fmt.Errorf("Invoice Ninja bank account %q has auto sync enabled; disable sync before importing supplier Account Payments", goTradieBankAccountName)
	}

	company, err := s.client.Companies.Current(ctx)
	if err != nil {
		return fmt.Errorf("resolve current Invoice Ninja company currency before importing supplier Account Payments: %w", err)
	}
	companyCurrencyID := strings.TrimSpace(company.Settings.CurrencyID)
	if companyCurrencyID == "" {
		return fmt.Errorf("current Invoice Ninja company has no usable default currency_id in settings; configure the company currency before importing supplier Account Payments")
	}

	state.bankIntegration = &integration
	state.currentCompany = company
	state.companyCurrencyID = companyCurrencyID

	transactions, err := s.client.BankTransactions.ListAll(ctx, invoiceninja.BankTransactionQuery{
		ListOptions:       invoiceninja.ListOptions{PerPage: 100, Status: "active,archived,deleted"},
		WithTrashed:       true,
		BankIntegrationID: integration.ID,
	})
	if err != nil {
		return fmt.Errorf("list %s bank transactions: %w", goTradieBankAccountName, err)
	}
	state.bankTransactions = transactions
	for _, transaction := range transactions {
		marker := accountPaymentMarkerFromText(transaction.Description)
		if marker == "" {
			continue
		}
		if transaction.IsDeleted || transaction.ArchivedAt != 0 {
			return fmt.Errorf("marked GoTradie bank transaction %q is archived or deleted; restore or resolve it before re-import", transaction.ID)
		}
		if state.ambiguousTransactionMarkers[marker] {
			continue
		}
		if existing, ok := state.transactionByMarker[marker]; ok && existing.ID != transaction.ID {
			return fmt.Errorf("GoTradie Account Payment identity %q is present on multiple Invoice Ninja bank transactions (%q and %q); resolve the duplicate before importing", marker, existing.ID, transaction.ID)
		}
		state.transactionByMarker[marker] = transaction
	}

	rules, err := s.client.BankTransactionRules.ListAll(ctx, invoiceninja.BankTransactionRuleQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Status: "active"},
	})
	if err != nil {
		return fmt.Errorf("list Invoice Ninja bank transaction rules: %w", err)
	}
	state.bankTransactionRules = rules
	return nil
}

func preflightBankTransactionRules(state *expenseImportState, rows []*preparedExpenseImportRow) error {
	hasPayment := false
	for _, row := range rows {
		if row.documentType == documentTypeAccountPayment && row.err == nil && row.duplicateRow == 0 {
			hasPayment = true
			break
		}
	}
	if !hasPayment {
		return nil
	}
	for _, rule := range state.bankTransactionRules {
		if !rule.IsDeleted && rule.ArchivedAt == 0 && rule.AutoConvert && strings.EqualFold(strings.TrimSpace(rule.AppliesTo), "DEBIT") {
			return fmt.Errorf("refusing supplier Account Payment import: Invoice Ninja auto-convert DEBIT rule %q is active and could create a duplicate Expense", rule.Name)
		}
	}
	return nil
}

func validateExistingTransactionIdentities(state *expenseImportState, rows []*preparedExpenseImportRow) {
	for _, row := range rows {
		if row.documentType != documentTypeAccountPayment || row.err != nil || row.duplicateRow != 0 {
			continue
		}
		if state.ambiguousTransactionMarkers[row.sourceMarker] {
			row.err = fmt.Errorf("row %d: stable Account Payment identity matches multiple Invoice Ninja Transactions", row.rowNo)
			continue
		}
		if transaction, ok := state.transactionByMarker[row.sourceMarker]; ok {
			date, amountCents, err := durableSupplierSettlementValues(transaction)
			if err != nil {
				row.err = fmt.Errorf("row %d: marked Invoice Ninja Transaction %q is invalid: %w", row.rowNo, transaction.ID, err)
				continue
			}
			if amountCents != row.grossCents {
				row.err = fmt.Errorf("row %d: marked Invoice Ninja Transaction %q amount drift: durable amount %.2f does not match source amount %.2f", row.rowNo, transaction.ID, centsAmount(amountCents), centsAmount(row.grossCents))
				continue
			}
			if date != row.date {
				row.err = fmt.Errorf("row %d: marked Invoice Ninja Transaction %q date drift: durable date %q does not match source date %q", row.rowNo, transaction.ID, date, row.date)
				continue
			}
			transactionCopy := transaction
			row.existingTransaction = &transactionCopy
		}
	}
}

func durableSupplierSettlementValues(transaction invoiceninja.BankTransaction) (string, int64, error) {
	if strings.TrimSpace(transaction.BaseType) != "DEBIT" {
		return "", 0, fmt.Errorf("base_type is %q; want DEBIT", transaction.BaseType)
	}
	amountCents := moneyCents(transaction.Amount)
	if amountCents <= 0 {
		return "", 0, fmt.Errorf("amount %.2f is not a positive withdrawal", transaction.Amount)
	}
	date := strings.TrimSpace(transaction.Date)
	if date == "" {
		return "", 0, fmt.Errorf("date is empty")
	}
	return date, amountCents, nil
}

func allocateAccountPayments(state *expenseImportState, rows []*preparedExpenseImportRow, committedOnly bool) ([]expenseSettlementAction, error) {
	resetSettlementRows(rows)
	blockPaymentsWithInvalidPurchases(rows)

	representedExpenses := make(map[string]bool)
	representedTransactions := make(map[string]bool)
	var purchases []*settlementPurchase
	var payments []*settlementPayment

	for _, row := range rows {
		if row.documentType == documentTypeAccountPayment {
			if transaction, ok := state.transactionByMarker[row.sourceMarker]; ok {
				representedTransactions[transaction.ID] = true
			}
		}
		if row.err != nil || row.duplicateRow != 0 {
			continue
		}
		if isPurchaseRow(row) && row.existingExpense != nil {
			representedExpenses[row.existingExpense.ID] = true
		}
		switch {
		case isPurchaseRow(row) && row.paymentType == "" && row.grossCents != 0:
			if committedOnly && row.existingExpense == nil {
				continue
			}
			supplierMarker := row.supplierAccountMarker
			if supplierMarker == "" {
				supplierMarker = supplierAccountMarker(row.supplier)
			}
			purchaseMarker := row.settlementPurchaseMarker
			if purchaseMarker == "" {
				purchaseMarker = row.sourceMarker
			}
			purchase := &settlementPurchase{
				row:             row,
				expense:         row.existingExpense,
				supplierAccount: supplierMarker,
				sourceID:        purchaseMarker,
				date:            row.date,
				grossCents:      row.grossCents,
				remainingCents:  max(row.grossCents, int64(0)),
			}
			purchases = append(purchases, purchase)
		case row.documentType == documentTypeAccountPayment:
			if committedOnly && row.existingTransaction == nil {
				continue
			}
			supplierMarker := row.supplierAccountMarker
			if supplierMarker == "" {
				supplierMarker = supplierAccountMarker(row.supplier)
			}
			paymentDate := row.date
			paymentAmountCents := row.grossCents
			if row.existingTransaction != nil {
				var err error
				paymentDate, paymentAmountCents, err = durableSupplierSettlementValues(*row.existingTransaction)
				if err != nil {
					row.err = fmt.Errorf("row %d: marked Invoice Ninja Transaction %q is invalid: %w", row.rowNo, row.existingTransaction.ID, err)
					continue
				}
			}
			payment := &settlementPayment{
				row:             row,
				transaction:     row.existingTransaction,
				supplierAccount: supplierMarker,
				sourceID:        row.sourceMarker,
				date:            paymentDate,
				amountCents:     paymentAmountCents,
				paymentType:     row.paymentType,
				paymentTypeID:   row.paymentTypeID,
				reference:       row.reference,
			}
			payments = append(payments, payment)
		}
	}

	for _, imported := range state.importedExpenses {
		if representedExpenses[imported.expense.ID] || imported.supplierAccountMarker == "" {
			continue
		}
		if imported.settlementPurchaseMarker == "" {
			return nil, fmt.Errorf("marked supplier-account Expense %q has no stable GoTradie supplier-purchase identity", imported.expense.ID)
		}
		total, err := parseExpenseMoney(privateNoteValues(imported.expense.PrivateNotes)["Source total inc GST"])
		totalCents := moneyCents(total)
		if err != nil || totalCents == 0 {
			return nil, fmt.Errorf("marked supplier-account Expense %q has no valid non-zero Source total inc GST", imported.expense.ID)
		}
		expenseCopy := imported.expense
		purchases = append(purchases, &settlementPurchase{
			expense:         &expenseCopy,
			supplierAccount: imported.supplierAccountMarker,
			sourceID:        imported.settlementPurchaseMarker,
			date:            imported.date,
			grossCents:      totalCents,
			remainingCents:  max(totalCents, int64(0)),
		})
	}

	for _, transaction := range state.bankTransactions {
		if representedTransactions[transaction.ID] {
			continue
		}
		paymentMarker := accountPaymentMarkerFromText(transaction.Description)
		if paymentMarker == "" {
			continue
		}
		supplierMarker := supplierAccountMarkerFromText(transaction.Description)
		if supplierMarker == "" {
			return nil, fmt.Errorf("marked GoTradie bank transaction %q has no supplier-account marker", transaction.ID)
		}
		date, amountCents, err := durableSupplierSettlementValues(transaction)
		if err != nil {
			return nil, fmt.Errorf("marked GoTradie bank transaction %q is not a valid dated withdrawal: %w", transaction.ID, err)
		}
		values := privateNoteValues(transaction.Description)
		paymentType := values["Payment type"]
		paymentTypeID := ""
		if resolved, ok := state.paymentTypes[paymentType]; ok {
			paymentTypeID = resolved.ID
		}
		transactionCopy := transaction
		payments = append(payments, &settlementPayment{
			transaction:     &transactionCopy,
			supplierAccount: supplierMarker,
			sourceID:        paymentMarker,
			date:            date,
			amountCents:     amountCents,
			paymentType:     paymentType,
			paymentTypeID:   paymentTypeID,
			reference:       values["Payment reference"],
		})
	}

	if err := blockPaymentsWithUnmarkedLegacyExpenses(state, representedExpenses, payments); err != nil {
		return nil, err
	}

	sort.Slice(purchases, func(i, j int) bool {
		if purchases[i].date == purchases[j].date {
			return purchases[i].sourceID < purchases[j].sourceID
		}
		return purchases[i].date < purchases[j].date
	})
	sort.Slice(payments, func(i, j int) bool {
		if payments[i].date == payments[j].date {
			return payments[i].sourceID < payments[j].sourceID
		}
		return payments[i].date < payments[j].date
	})

	// Purchase-side events become eligible in date/identity order before any
	// payments on that date, preserving the existing on-or-before rule.
	eligiblePurchases := make([]*settlementPurchase, 0, len(purchases))
	creditBalances := make(map[string]int64)
	applyCredit := func(supplier string, creditCents int64, date string) int64 {
		for _, purchase := range eligiblePurchases {
			if creditCents == 0 {
				break
			}
			if purchase.supplierAccount != supplier || purchase.remainingCents <= 0 {
				continue
			}
			applied := minInt64(creditCents, purchase.remainingCents)
			purchase.remainingCents -= applied
			purchase.creditApplied = true
			creditCents -= applied
			if purchase.remainingCents == 0 {
				purchase.settledDate = date
			}
		}
		return creditCents
	}
	processPurchase := func(purchase *settlementPurchase) {
		if purchase.grossCents < 0 {
			remainingCredit := applyCredit(purchase.supplierAccount, -purchase.grossCents, purchase.date)
			creditBalances[purchase.supplierAccount] += remainingCredit
			return
		}
		if creditCents := creditBalances[purchase.supplierAccount]; creditCents > 0 {
			applied := minInt64(creditCents, purchase.remainingCents)
			purchase.remainingCents -= applied
			purchase.creditApplied = true
			creditBalances[purchase.supplierAccount] -= applied
			if purchase.remainingCents == 0 {
				purchase.settledDate = purchase.date
			}
		}
		eligiblePurchases = append(eligiblePurchases, purchase)
	}
	purchaseIndex := 0
	processPurchasesThrough := func(date string) {
		for purchaseIndex < len(purchases) && purchases[purchaseIndex].date <= date {
			processPurchase(purchases[purchaseIndex])
			purchaseIndex++
		}
	}

	for _, payment := range payments {
		if payment.row != nil && payment.row.err != nil {
			continue
		}
		processPurchasesThrough(payment.date)
		remaining := payment.amountCents
		for _, purchase := range eligiblePurchases {
			if remaining == 0 {
				break
			}
			if purchase.supplierAccount != payment.supplierAccount || purchase.date > payment.date || purchase.remainingCents <= 0 {
				continue
			}
			allocated := minInt64(remaining, purchase.remainingCents)
			allocation := ExpensePaymentAllocation{
				PurchaseSourceID: purchase.sourceID,
				PaymentSourceID:  payment.sourceID,
				Amount:           centsAmount(allocated),
				PaymentDate:      payment.date,
				PaymentType:      payment.paymentType,
				PaymentTypeID:    payment.paymentTypeID,
				PaymentReference: payment.reference,
			}
			purchase.allocations = append(purchase.allocations, allocation)
			payment.allocations = append(payment.allocations, allocation)
			purchase.remainingCents -= allocated
			remaining -= allocated
			if purchase.remainingCents == 0 {
				purchase.settledDate = payment.date
			}
		}
		payment.unappliedCents = remaining
		if payment.row != nil {
			payment.row.allocations = append([]ExpensePaymentAllocation(nil), payment.allocations...)
			payment.row.unappliedCents = remaining
		}
	}
	for purchaseIndex < len(purchases) {
		processPurchase(purchases[purchaseIndex])
		purchaseIndex++
	}

	var actions []expenseSettlementAction
	for _, purchase := range purchases {
		if purchase.grossCents < 0 {
			if purchase.row != nil {
				purchase.row.remainingCents = 0
			}
			continue
		}
		desiredDate, desiredTypeID := finalSettlementState(purchase)
		if purchase.row != nil {
			purchase.row.remainingCents = purchase.remainingCents
			purchase.row.allocations = append([]ExpensePaymentAllocation(nil), purchase.allocations...)
			purchase.row.desiredPaymentDate = desiredDate
			purchase.row.desiredPaymentTypeID = desiredTypeID
			continue
		}
		if purchase.expense != nil {
			actions = append(actions, expenseSettlementAction{
				expense:       *purchase.expense,
				sourceID:      purchase.sourceID,
				desiredDate:   desiredDate,
				desiredTypeID: desiredTypeID,
				allocations:   append([]ExpensePaymentAllocation(nil), purchase.allocations...),
			})
		}
	}
	return actions, nil
}

func resetSettlementRows(rows []*preparedExpenseImportRow) {
	for _, row := range rows {
		row.allocations = nil
		row.unappliedCents = row.grossCents
		if isPurchaseRow(row) && row.paymentType == "" {
			row.remainingCents = max(row.grossCents, int64(0))
			row.desiredPaymentDate = ""
			row.desiredPaymentTypeID = ""
		}
	}
}

func finalSettlementState(purchase *settlementPurchase) (string, string) {
	if purchase.remainingCents != 0 || purchase.settledDate == "" {
		return "", ""
	}
	if purchase.creditApplied || len(purchase.allocations) == 0 {
		return purchase.settledDate, ""
	}
	typeID := purchase.allocations[0].PaymentTypeID
	for _, allocation := range purchase.allocations[1:] {
		if allocation.PaymentTypeID == "" || allocation.PaymentTypeID != typeID {
			typeID = ""
			break
		}
	}
	return purchase.settledDate, typeID
}

func blockPaymentsWithInvalidPurchases(rows []*preparedExpenseImportRow) {
	for _, payment := range rows {
		if payment.documentType != documentTypeAccountPayment || payment.err != nil || payment.duplicateRow != 0 {
			continue
		}
		for _, purchase := range rows {
			if !isPurchaseRow(purchase) || purchase.err == nil || purchase.duplicateRow != 0 || strings.TrimSpace(purchase.paymentType) != "" {
				continue
			}
			if purchase.supplier != "" && strings.TrimSpace(purchase.supplier) != strings.TrimSpace(payment.supplier) {
				continue
			}
			if purchase.date != "" && purchase.date > payment.date {
				continue
			}
			payment.err = fmt.Errorf("row %d: allocation blocked by unresolved earlier purchase row %d: %v", payment.rowNo, purchase.rowNo, purchase.err)
			break
		}
	}
}

func blockPaymentsWithUnmarkedLegacyExpenses(state *expenseImportState, represented map[string]bool, payments []*settlementPayment) error {
	for _, imported := range state.importedExpenses {
		if represented[imported.expense.ID] || imported.supplierAccountMarker != "" || imported.expense.PaymentDate != "" {
			continue
		}
		for _, payment := range payments {
			if imported.supplier != "" && supplierAccountMarker(imported.supplier) != payment.supplierAccount {
				continue
			}
			if imported.date != "" && imported.date > payment.date {
				continue
			}
			err := fmt.Errorf("allocation blocked by older unpaid imported Expense %q without a durable supplier-account marker; include its source purchase row to migrate it safely", imported.expense.ID)
			if payment.row != nil {
				payment.row.err = err
				continue
			}
			return err
		}
	}
	return nil
}

func supplierAccountMarker(supplier string) string {
	return hashedMarker(supplierAccountMarkerPrefix, []string{strings.TrimSpace(supplier)})
}

func settlementPurchaseMarker(sourceMarker string) string {
	return hashedMarker(settlementPurchaseMarkerPrefix, []string{sourceMarker})
}

func supplierAccountMarkerFromText(text string) string {
	return markerFromText(text, supplierAccountMarkerPrefix)
}

func settlementPurchaseMarkerFromText(text string) string {
	return markerFromText(text, settlementPurchaseMarkerPrefix)
}

func accountPaymentMarkerFromText(text string) string {
	for _, prefix := range []string{"[GoTradie account-payment:v3:", "[GoTradie account-payment:v2:"} {
		if marker := markerFromText(text, prefix); marker != "" {
			return marker
		}
	}
	return ""
}

func markerFromText(text, prefix string) string {
	start := strings.Index(text, prefix)
	if start < 0 {
		return ""
	}
	end := strings.Index(text[start:], "]")
	if end < 0 {
		return ""
	}
	return text[start : start+end+1]
}

func supplierAccountNotes(notes, supplier, supplierMarker, purchaseMarker string) string {
	lines := []string{strings.TrimSpace(notes)}
	if !strings.Contains(notes, "GoTradie supplier account: ") {
		lines = append(lines, "GoTradie supplier account: "+strings.TrimSpace(supplier))
	}
	if supplierAccountMarkerFromText(notes) == "" {
		lines = append(lines, supplierMarker)
	}
	if settlementPurchaseMarkerFromText(notes) == "" {
		lines = append(lines, purchaseMarker)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func accountPaymentDescription(row *preparedExpenseImportRow) string {
	lines := []string{"GoTradie historical supplier settlement"}
	add := func(label, value string) {
		if value = strings.TrimSpace(value); value != "" {
			lines = append(lines, label+": "+value)
		}
	}
	add("Supplier account", row.supplier)
	add("Payment reference", row.reference)
	add("Payment type", row.paymentType)
	add("Source file", row.receiptName)
	lines = append(lines, row.supplierAccountMarker, row.sourceMarker)
	return strings.Join(lines, "\n")
}

func expenseNotesForRow(notes string, row *preparedExpenseImportRow) string {
	if !isPurchaseRow(row) {
		return notes
	}
	if row.paymentType != "" {
		return withoutSupplierAccountNotes(notes)
	}
	if row.supplierAccountMarker == "" || row.settlementPurchaseMarker == "" {
		return notes
	}
	return supplierAccountNotes(notes, row.supplier, row.supplierAccountMarker, row.settlementPurchaseMarker)
}

func withoutSupplierAccountNotes(notes string) string {
	lines := make([]string, 0, strings.Count(notes, "\n")+1)
	for _, line := range strings.Split(notes, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "GoTradie supplier account: ") ||
			strings.HasPrefix(trimmed, supplierAccountMarkerPrefix) ||
			strings.HasPrefix(trimmed, settlementPurchaseMarkerPrefix) {
			continue
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (s *Service) previewExpenseImportRow(ctx context.Context, state *expenseImportState, idx map[string]int, row *preparedExpenseImportRow) CSVImportResult {
	if row.err != nil {
		return CSVImportResult{Name: row.name, Action: "error", Error: row.err}
	}
	if row.duplicateRow != 0 {
		return CSVImportResult{
			Name:    row.name,
			Action:  "unchanged",
			Changes: []string{fmt.Sprintf("duplicate-source-row:%d", row.duplicateRow)},
		}
	}
	switch row.documentType {
	case documentTypeInvoice, documentTypeReceipt:
		return s.importExpenseRow(ctx, state, idx, row, true, false)
	case documentTypeAccountPayment:
		return s.importAccountPaymentRow(ctx, state, row, true)
	case documentTypeAdjustment:
		return deferredAdjustmentResult(row)
	default:
		return unsupportedExpenseImportRow(row)
	}
}

func unsupportedExpenseImportRow(row *preparedExpenseImportRow) CSVImportResult {
	return CSVImportResult{
		Name:   row.name,
		Action: "error",
		Error:  fmt.Errorf("row %d: unsupported Document Type %q", row.rowNo, row.documentType),
	}
}

func previewRemoteSettlementActions(actions []expenseSettlementAction) []CSVImportResult {
	var results []CSVImportResult
	for _, action := range actions {
		if action.expense.PaymentDate == action.desiredDate && action.expense.PaymentTypeID == action.desiredTypeID {
			continue
		}
		change := "payment:unpaid"
		if action.desiredDate != "" {
			change = "payment:paid:" + action.desiredDate
		}
		results = append(results, CSVImportResult{
			ID:          action.expense.ID,
			Name:        "existing supplier-account Expense " + action.sourceID,
			Action:      "would-update",
			Changes:     []string{change},
			Allocations: append([]ExpensePaymentAllocation(nil), action.allocations...),
		})
	}
	return results
}

func (s *Service) applyCurrentExpenseSettlement(ctx context.Context, row *preparedExpenseImportRow) error {
	if row.existingExpense == nil {
		return nil
	}
	existing := *row.existingExpense
	if existing.PaymentDate == row.desiredPaymentDate && existing.PaymentTypeID == row.desiredPaymentTypeID {
		return nil
	}
	updated, err := s.client.Expenses.UpdatePaymentStatus(ctx, existing.ID, invoiceninja.ExpensePaymentStatusRequest{
		PaymentDate:   row.desiredPaymentDate,
		PaymentTypeID: row.desiredPaymentTypeID,
	})
	if err != nil {
		return fmt.Errorf("row %d update derived supplier settlement state: %w", row.rowNo, err)
	}
	updated.PaymentDate = row.desiredPaymentDate
	updated.PaymentTypeID = row.desiredPaymentTypeID
	row.existingExpense = updated
	return nil
}

func (s *Service) applyRemoteSettlementActions(ctx context.Context, actions []expenseSettlementAction) []CSVImportResult {
	var results []CSVImportResult
	for _, action := range actions {
		if action.expense.PaymentDate == action.desiredDate && action.expense.PaymentTypeID == action.desiredTypeID {
			continue
		}
		result := CSVImportResult{
			ID:          action.expense.ID,
			Name:        "existing supplier-account Expense " + action.sourceID,
			Action:      "updated",
			Allocations: append([]ExpensePaymentAllocation(nil), action.allocations...),
		}
		if action.desiredDate == "" {
			result.Changes = []string{"payment:unpaid"}
		} else {
			result.Changes = []string{"payment:paid:" + action.desiredDate}
		}
		if _, err := s.client.Expenses.UpdatePaymentStatus(ctx, action.expense.ID, invoiceninja.ExpensePaymentStatusRequest{
			PaymentDate:   action.desiredDate,
			PaymentTypeID: action.desiredTypeID,
		}); err != nil {
			result.Action = "error"
			result.Error = fmt.Errorf("update derived supplier settlement state for Expense %q: %w", action.expense.ID, err)
		}
		results = append(results, result)
	}
	return results
}

func appendSettlementChanges(changes []string, row *preparedExpenseImportRow) []string {
	for _, change := range []string{paymentChangeDescription(row), allocationSummary(row)} {
		if change == "" || containsString(changes, change) {
			continue
		}
		changes = append(changes, change)
	}
	return changes
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
