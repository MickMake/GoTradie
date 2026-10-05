package ninja

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
)

var expenseImportRequiredColumns = []string{
	"Import ID",
	"Date",
	"Supplier",
	"Document Type",
	"Payment Type",
	"Tax Treatment",
	"Category",
	"Option",
	"Total Inc GST",
	"Business %",
	"Business Amount",
	"Business GST",
}

const (
	documentTypeInvoice        = "Invoice"
	documentTypeReceipt        = "Receipt"
	documentTypeAdjustment     = "Adjustment"
	documentTypeAccountPayment = "Account Payment"
	goTradieBankAccountName    = "GoTradie"
	importIDMarkerPrefix       = "[GoTradie import-id:v1:"
	receiptMarkerPrefix        = "[GoTradie receipt:v1:"
	receiptOwnerMarker         = "[GoTradie receipt-owner:v1]"
)

// ExpensePaymentAllocation is the deterministic, in-memory link between a
// supplier-account payment and a source purchase. It is deliberately not a
// datastore record.
type ExpensePaymentAllocation struct {
	PurchaseSourceID string
	PaymentSourceID  string
	Amount           float64
	PaymentDate      string
	PaymentType      string
	PaymentTypeID    string
	PaymentReference string
}

type expenseImportState struct {
	vendors                     map[string]invoiceninja.Vendor
	vendorByID                  map[string]invoiceninja.Vendor
	categories                  map[string]invoiceninja.ExpenseCategory
	projects                    map[string]invoiceninja.Project
	clientByQuote               map[string]string
	paymentTypes                map[string]invoiceninja.PaymentType
	expenseByMarker             map[string]invoiceninja.Expense
	ambiguousMarkers            map[string]bool
	ambiguousBases              map[string]bool
	importedExpenses            []importedExpenseState
	seenMarkers                 map[string]int
	bankIntegration             *invoiceninja.BankIntegration
	currentCompany              *invoiceninja.Company
	companyCurrencyID           string
	bankTransactions            []invoiceninja.BankTransaction
	transactionByMarker         map[string]invoiceninja.BankTransaction
	ambiguousTransactionMarkers map[string]bool
	bankTransactionRules        []invoiceninja.BankTransactionRule
	receiptOwnerByKey           map[string]invoiceninja.Expense
	receiptOwnerAmbiguous       map[string]bool
}

type importedExpenseState struct {
	expense                  invoiceninja.Expense
	marker                   string
	baseMarker               string
	supplier                 string
	date                     string
	sourceRow                int
	supplierAccountMarker    string
	settlementPurchaseMarker string
}

type preparedExpenseImportRow struct {
	rec                      []string
	rowNo                    int
	importID                 string
	documentType             string
	supplier                 string
	date                     string
	name                     string
	vendorName               string
	categoryName             string
	paymentType              string
	paymentTypeID            string
	sourceBaseMarker         string
	sourceMarker             string
	legacyMarker             string
	duplicateMarker          string
	reference                string
	receiptName              string
	receiptPath              string
	receiptKey               string
	receiptOwner             bool
	receiptHasDocument       bool
	businessPct              float64
	businessAmount           float64
	businessGST              float64
	grossCents               int64
	grossKnown               bool
	remainingCents           int64
	desiredPaymentDate       string
	desiredPaymentTypeID     string
	allocations              []ExpensePaymentAllocation
	unappliedCents           int64
	duplicateRow             int
	deferred                 bool
	supplierAccountMarker    string
	settlementPurchaseMarker string
	existingExpense          *invoiceninja.Expense
	existingTransaction      *invoiceninja.BankTransaction
	err                      error
}

type receiptIndex map[string][]string

// ImportExpensesCSV imports the purchase ledger shape used by Mick's master
// spreadsheet. Preview is the default; pass dryRun=false only for --commit.
//
// Purchase rows remain the authoritative analytical records. Account Payment
// rows become marked withdrawal Transactions; deterministic allocations may
// only change the paid fields of a fully covered purchase Expense.
func (s *Service) ImportExpensesCSV(ctx context.Context, r io.Reader, dryRun bool, receiptsRoot string) ([]CSVImportResult, error) {
	return s.ImportExpensesCSVWithOptions(ctx, r, ExpenseImportOptions{
		DryRun:       dryRun,
		ReceiptsRoot: receiptsRoot,
	})
}

// ImportExpensesCSVWithOptions implements the v0.5.2 whole-file preflight,
// progress and batching contract while retaining ImportExpensesCSV for callers
// that do not need operational callbacks.
func (s *Service) ImportExpensesCSVWithOptions(ctx context.Context, r io.Reader, options ExpenseImportOptions) ([]CSVImportResult, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	recs, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}

	preflight := preflightExpenseSource(recs, options.ReceiptsRoot, options.BatchSize)
	if options.OnPreflight != nil {
		options.OnPreflight(preflight.report)
	}
	if len(preflight.report.Errors) > 0 {
		return nil, &ExpenseImportPreflightError{Report: preflight.report}
	}
	idx := preflight.index
	receipts := preflight.receipts

	settlementEnabled := expenseCSVNeedsSettlement(recs[1:], idx)
	state, err := s.loadExpenseImportState(ctx, settlementEnabled)
	if err != nil {
		return nil, err
	}

	rows := make([]*preparedExpenseImportRow, 0, len(recs)-1)
	for rowNo, rec := range recs[1:] {
		if emptyRecord(rec) {
			continue
		}
		row := prepareExpenseImportRow(state, receipts, idx, rec, rowNo+2)
		if row.receiptPath != "" {
			row.receiptKey = preflight.receiptKeys[row.receiptPath]
		}
		rows = append(rows, row)
	}
	assignExpenseRowIdentities(rows)
	validateExistingExpenseIdentities(state, rows)
	validateExistingTransactionIdentities(state, rows)
	if err := validateAndAssignReceiptOwnership(state, rows); err != nil {
		return nil, err
	}
	settlementActions, err := allocateAccountPayments(state, rows, false)
	if err != nil {
		return nil, err
	}
	if err := preflightBankTransactionRules(state, rows); err != nil {
		return nil, err
	}
	if err := s.preflightExpensePaymentNotifications(ctx, state, rows, settlementActions, options.DryRun); err != nil {
		return nil, err
	}

	if options.DryRun {
		results := make([]CSVImportResult, len(rows))
		batchSize := effectiveExpenseBatchSize(len(rows), options.BatchSize, settlementEnabled)
		for start, batchNumber := 0, 1; start < len(rows); start, batchNumber = start+batchSize, batchNumber+1 {
			end := minInt(start+batchSize, len(rows))
			for i := start; i < end; i++ {
				results[i] = s.previewExpenseImportRow(ctx, state, idx, rows[i])
				emitExpenseProgress(options, i+1, len(rows), results[i])
			}
			if err := completeExpenseBatch(options, preflight.report.Mode, batchNumber, results[start:end], end == len(rows)); err != nil {
				return results[:end], err
			}
		}
		return append(results, previewRemoteSettlementActions(settlementActions)...), nil
	}
	if !settlementEnabled {
		results := make([]CSVImportResult, len(rows))
		batchSize := effectiveExpenseBatchSize(len(rows), options.BatchSize, false)
		for start, batchNumber := 0, 1; start < len(rows); start, batchNumber = start+batchSize, batchNumber+1 {
			end := minInt(start+batchSize, len(rows))
			for i := start; i < end; i++ {
				row := rows[i]
				switch {
				case row.err != nil || row.duplicateRow != 0 || row.documentType == documentTypeAdjustment:
					results[i] = s.previewExpenseImportRow(ctx, state, idx, row)
				case isPurchaseRow(row):
					results[i] = s.importExpenseRow(ctx, state, idx, row, false, false)
				default:
					results[i] = unsupportedExpenseImportRow(row)
				}
				emitExpenseProgress(options, i+1, len(rows), results[i])
			}
			if err := completeExpenseBatch(options, preflight.report.Mode, batchNumber, results[start:end], end == len(rows)); err != nil {
				return results[:end], err
			}
		}
		return results, nil
	}

	// Commit in three stages: purchases without speculative supplier settlement,
	// durable withdrawal Transactions, then derived Paid/Unpaid state.
	results := make([]CSVImportResult, len(rows))
	progressIndex := 0
	emitSettlementProgress := func(result CSVImportResult) {
		progressIndex++
		emitExpenseProgress(options, progressIndex, len(rows), result)
	}
	for i, row := range rows {
		if row.err != nil || row.duplicateRow != 0 || row.documentType == documentTypeAdjustment {
			results[i] = s.previewExpenseImportRow(ctx, state, idx, row)
			emitSettlementProgress(results[i])
			continue
		}
		if isPurchaseRow(row) {
			results[i] = s.importExpenseRow(ctx, state, idx, row, false, true)
			emitSettlementProgress(results[i])
			continue
		}
		if row.documentType != documentTypeAccountPayment {
			results[i] = unsupportedExpenseImportRow(row)
			emitSettlementProgress(results[i])
		}
	}
	for i, row := range rows {
		if isPurchaseRow(row) && results[i].Error != nil {
			return results, fmt.Errorf("settlement-safe execution stopped after purchase row %d failed; correct the error and rerun the original import: %w", row.rowNo, results[i].Error)
		}
	}
	for i, row := range rows {
		if row.documentType != documentTypeAccountPayment || row.err != nil || row.duplicateRow != 0 {
			continue
		}
		results[i] = s.importAccountPaymentRow(ctx, state, row, false)
		emitSettlementProgress(results[i])
		if results[i].Error != nil {
			return results, fmt.Errorf("settlement-safe execution stopped after Account Payment row %d failed; correct the error and rerun the original import: %w", row.rowNo, results[i].Error)
		}
	}

	committedActions, err := allocateAccountPayments(state, rows, true)
	if err != nil {
		return results, err
	}
	for i, row := range rows {
		if !isPurchaseRow(row) || row.paymentType != "" || results[i].Error != nil || row.existingExpense == nil {
			continue
		}
		if err := s.applyCurrentExpenseSettlement(ctx, row); err != nil {
			results[i].Action = "error"
			results[i].Error = err
			continue
		}
		results[i].Allocations = append([]ExpensePaymentAllocation(nil), row.allocations...)
		results[i].Changes = appendSettlementChanges(results[i].Changes, row)
	}
	for i, row := range rows {
		if row.documentType == documentTypeAccountPayment && row.err == nil && row.duplicateRow == 0 && results[i].Error == nil {
			results[i] = refreshAccountPaymentResult(results[i], row)
		}
	}
	remoteResults := s.applyRemoteSettlementActions(ctx, committedActions)
	return append(results, remoteResults...), nil
}

func (s *Service) loadExpenseImportState(ctx context.Context, settlementEnabled bool) (*expenseImportState, error) {
	vendors, err := s.client.Vendors.ListAll(ctx, invoiceninja.VendorQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, fmt.Errorf("list vendors: %w", err)
	}
	categories, err := s.client.ExpenseCategories.ListAll(ctx, invoiceninja.ExpenseCategoryQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, fmt.Errorf("list expense categories: %w", err)
	}
	projects, err := s.client.Projects.ListAll(ctx, invoiceninja.ProjectQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	expenseQuery := invoiceninja.ExpenseQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Include: []string{"documents"}, Status: "active,archived,deleted"},
		WithTrashed: true,
	}
	expenses, err := s.client.Expenses.ListAll(ctx, expenseQuery)
	if err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}
	quotes, err := s.client.Quotes.ListAll(ctx, invoiceninja.QuoteQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, fmt.Errorf("list quotes for project matching: %w", err)
	}
	paymentTypes, err := s.client.Statics.PaymentTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list payment types: %w", err)
	}

	state := &expenseImportState{
		vendors:                     make(map[string]invoiceninja.Vendor, len(vendors)),
		vendorByID:                  make(map[string]invoiceninja.Vendor, len(vendors)),
		categories:                  make(map[string]invoiceninja.ExpenseCategory, len(categories)),
		projects:                    make(map[string]invoiceninja.Project, len(projects)),
		clientByQuote:               make(map[string]string, len(quotes)),
		paymentTypes:                make(map[string]invoiceninja.PaymentType, len(paymentTypes)),
		expenseByMarker:             make(map[string]invoiceninja.Expense, len(expenses)*2),
		ambiguousMarkers:            make(map[string]bool),
		ambiguousBases:              make(map[string]bool),
		importedExpenses:            make([]importedExpenseState, 0, len(expenses)),
		seenMarkers:                 make(map[string]int),
		transactionByMarker:         make(map[string]invoiceninja.BankTransaction),
		ambiguousTransactionMarkers: make(map[string]bool),
		receiptOwnerByKey:           make(map[string]invoiceninja.Expense),
		receiptOwnerAmbiguous:       make(map[string]bool),
	}
	for _, vendor := range vendors {
		state.vendors[key(vendor.Name)] = vendor
		state.vendorByID[vendor.ID] = vendor
	}
	for _, category := range categories {
		state.categories[key(category.Name)] = category
	}
	for _, project := range projects {
		if strings.TrimSpace(project.Number) != "" {
			state.projects[key(project.Number)] = project
		}
	}
	for _, quote := range quotes {
		doc := invoiceninja.Invoice(quote)
		if strings.TrimSpace(doc.Number) != "" && strings.TrimSpace(doc.ClientID) != "" {
			state.clientByQuote[key(doc.Number)] = doc.ClientID
		}
	}
	for _, paymentType := range paymentTypes {
		name := strings.TrimSpace(paymentType.Name)
		if name == "" {
			continue
		}
		if existing, ok := state.paymentTypes[name]; ok && existing.ID != paymentType.ID {
			return nil, fmt.Errorf("payment type label %q maps to multiple IDs", name)
		}
		state.paymentTypes[name] = paymentType
	}
	for _, expense := range expenses {
		sourceMarker := sourceMarkerFromNotes(expense.PrivateNotes)
		supplierMarker := supplierAccountMarkerFromText(expense.PrivateNotes)
		purchaseMarker := settlementPurchaseMarkerFromText(expense.PrivateNotes)
		if err := registerExistingReceiptState(state, expense); err != nil {
			return nil, err
		}
		if (expense.IsDeleted || expense.ArchivedAt != 0) && strings.HasPrefix(sourceMarker, importIDMarkerPrefix) {
			return nil, fmt.Errorf("Import ID marker on Invoice Ninja Expense %q is archived or deleted; restore or explicitly resolve it before importing again", expense.ID)
		}
		if settlementEnabled && (expense.IsDeleted || expense.ArchivedAt != 0) && (supplierMarker != "" || purchaseMarker != "") {
			return nil, fmt.Errorf("marked supplier-account Expense %q is archived or deleted; restore or explicitly resolve it before importing again", expense.ID)
		}
		if expense.IsDeleted || expense.ArchivedAt != 0 {
			continue
		}
		if sourceMarker != "" {
			addExpenseMarker(state, sourceMarker, expense)
			state.importedExpenses = append(state.importedExpenses, importedExpenseState{
				expense:                  expense,
				marker:                   sourceMarker,
				baseMarker:               expensePurchaseBaseMarker(expense, state.vendorByID),
				supplier:                 expenseSourceSupplier(expense, state.vendorByID),
				date:                     expense.Date,
				sourceRow:                sourceRowFromNotes(expense.PrivateNotes),
				supplierAccountMarker:    supplierMarker,
				settlementPurchaseMarker: purchaseMarker,
			})
		}
	}
	indexImportedExpenseIdentities(state)
	if settlementEnabled {
		if err := s.loadSupplierSettlementState(ctx, state); err != nil {
			return nil, err
		}
	}
	return state, nil
}

func addExpenseMarker(state *expenseImportState, marker string, expense invoiceninja.Expense) {
	if marker == "" || state.ambiguousMarkers[marker] {
		return
	}
	if existing, ok := state.expenseByMarker[marker]; ok && existing.ID != expense.ID {
		delete(state.expenseByMarker, marker)
		state.ambiguousMarkers[marker] = true
		return
	}
	state.expenseByMarker[marker] = expense
}

func indexImportedExpenseIdentities(state *expenseImportState) {
	groups := make(map[string][]importedExpenseState)
	for _, imported := range state.importedExpenses {
		if imported.baseMarker != "" {
			groups[imported.baseMarker] = append(groups[imported.baseMarker], imported)
		}
	}
	for baseMarker, group := range groups {
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].sourceRow == group[j].sourceRow {
				return group[i].expense.ID < group[j].expense.ID
			}
			return group[i].sourceRow < group[j].sourceRow
		})
		if len(group) > 1 {
			for i, imported := range group {
				if imported.sourceRow <= 0 || (i > 0 && imported.sourceRow == group[i-1].sourceRow) {
					state.ambiguousBases[baseMarker] = true
					break
				}
			}
		}
		if state.ambiguousBases[baseMarker] {
			continue
		}
		for i, imported := range group {
			addExpenseMarker(state, purchaseOccurrenceMarker(baseMarker, i+1), imported.expense)
		}
	}
}

func legacyExpenseSourceMarker(expense invoiceninja.Expense, vendors map[string]invoiceninja.Vendor) string {
	marker := sourceMarkerFromNotes(expense.PrivateNotes)
	if marker == "" || strings.HasPrefix(marker, "[GoTradie source:v2:") || strings.HasPrefix(marker, importIDMarkerPrefix) {
		return ""
	}
	return expensePurchaseBaseMarker(expense, vendors)
}

func expensePurchaseBaseMarker(expense invoiceninja.Expense, vendors map[string]invoiceninja.Vendor) string {
	marker := sourceMarkerFromNotes(expense.PrivateNotes)
	if marker == "" || strings.HasPrefix(marker, importIDMarkerPrefix) {
		return ""
	}
	vendor, ok := vendors[expense.VendorID]
	if !ok || strings.TrimSpace(vendor.Name) == "" {
		return ""
	}
	notes := privateNoteValues(expense.PrivateNotes)
	store := notes["Store"]
	supplier := expenseSourceSupplier(expense, vendors)
	return purchaseSourceMarkerFromValues(purchaseIdentityValues(
		expense.Date,
		supplier,
		store,
		expense.TransactionReference,
		notes["Item number"],
		notes["Item description"],
		notes["Qty"],
		notes["Unit"],
		notes["Unit price"],
		notes["Source total ex GST"],
		notes["Source GST"],
		notes["Source total inc GST"],
		notes["Source currency"],
	))
}

func expenseSourceSupplier(expense invoiceninja.Expense, vendors map[string]invoiceninja.Vendor) string {
	vendor, ok := vendors[expense.VendorID]
	if !ok {
		return ""
	}
	supplier := strings.TrimSpace(vendor.Name)
	store := privateNoteValues(expense.PrivateNotes)["Store"]
	if store != "" {
		supplier = strings.TrimSuffix(supplier, " - "+store)
	}
	return strings.TrimSpace(supplier)
}

func sourceRowFromNotes(notes string) int {
	value := privateNoteValues(notes)["GoTradie source row"]
	row, err := strconv.Atoi(value)
	if err != nil || row <= 0 {
		return 0
	}
	return row
}

func prepareExpenseImportRow(state *expenseImportState, receipts receiptIndex, idx map[string]int, rec []string, rowNo int) *preparedExpenseImportRow {
	importID := strings.TrimSpace(cell(rec, idx, "Import ID"))
	supplier := cell(rec, idx, "Supplier")
	store := cell(rec, idx, "Store")
	vendorName := expenseVendorName(supplier, store)
	description := cell(rec, idx, "Item Description")
	name := vendorName
	if description != "" {
		name += " / " + description
	}
	row := &preparedExpenseImportRow{
		rec:          rec,
		rowNo:        rowNo,
		importID:     importID,
		documentType: strings.TrimSpace(cell(rec, idx, "Document Type")),
		supplier:     supplier,
		name:         name,
		vendorName:   vendorName,
		categoryName: cell(rec, idx, "Category"),
		paymentType:  strings.TrimSpace(cell(rec, idx, "Payment Type")),
		reference:    cell(rec, idx, "Invoice Number"),
		receiptName:  strings.TrimSpace(cell(rec, idx, "File Name")),
	}
	if importID != "" {
		row.sourceBaseMarker = importIDMarker(importID)
		row.sourceMarker = row.sourceBaseMarker
	}
	if row.documentType == documentTypeInvoice || row.documentType == documentTypeReceipt {
		row.legacyMarker = sourceMarker(rec)
		row.duplicateMarker = purchaseDuplicateMarker(rec, idx)
	}
	if supplier == "" {
		row.err = fmt.Errorf("row %d: Supplier is required", rowNo)
		return row
	}
	date, err := parseExpenseDate(cell(rec, idx, "Date"))
	if err != nil {
		row.err = fmt.Errorf("row %d date: %w", rowNo, err)
		return row
	}
	row.date = date
	if row.documentType == documentTypeAdjustment {
		row.name = strings.TrimSpace(supplier + " adjustment")
		row.deferred = true
		if amount, parseErr := parseExpenseMoney(cell(rec, idx, "Business Amount")); parseErr == nil {
			row.businessAmount = amount
		}
		return row
	}

	switch row.documentType {
	case documentTypeInvoice, documentTypeReceipt:
		return preparePurchaseRow(state, receipts, idx, row)
	case documentTypeAccountPayment:
		row.name = supplier + " account payment"
		row.supplierAccountMarker = supplierAccountMarker(supplier)
		if row.paymentType == "" {
			row.err = fmt.Errorf("row %d: Payment Type is required for Account Payment", rowNo)
			return row
		}
		paymentType, ok := state.paymentTypes[row.paymentType]
		if !ok {
			row.err = fmt.Errorf("row %d: unknown Invoice Ninja Payment Type %q", rowNo, row.paymentType)
			return row
		}
		row.paymentTypeID = paymentType.ID
		gross, err := parseExpenseMoney(cell(rec, idx, "Total Inc GST"))
		if err != nil {
			row.err = fmt.Errorf("row %d total inc GST: %w", rowNo, err)
			return row
		}
		row.grossCents = moneyCents(gross)
		row.grossKnown = true
		row.businessAmount = gross
		if row.grossCents <= 0 {
			row.err = fmt.Errorf("row %d: Account Payment Total Inc GST must be greater than zero", rowNo)
			return row
		}
		row.unappliedCents = row.grossCents
		if row.importID == "" {
			row.sourceBaseMarker = accountPaymentBaseMarker(row)
			row.sourceMarker = row.sourceBaseMarker
		} else {
			row.legacyMarker = accountPaymentBaseMarker(row)
		}
		return row
	case documentTypeAdjustment:
		return row
	case "":
		row.err = fmt.Errorf("row %d: Document Type is required", rowNo)
		return row
	default:
		row.err = fmt.Errorf("row %d: unknown Document Type %q", rowNo, row.documentType)
		return row
	}
}

func preparePurchaseRow(state *expenseImportState, receipts receiptIndex, idx map[string]int, row *preparedExpenseImportRow) *preparedExpenseImportRow {
	if row.importID == "" {
		row.sourceBaseMarker = purchaseSourceMarker(row.rec, idx, row.date)
		row.sourceMarker = row.sourceBaseMarker
	}
	if row.importID == "" {
		row.legacyMarker = sourceMarker(row.rec)
	} else {
		row.legacyMarker = purchaseSourceMarker(row.rec, idx, row.date)
	}
	if row.categoryName == "" {
		row.err = fmt.Errorf("row %d: Category is required", row.rowNo)
		return row
	}
	if row.paymentType != "" {
		paymentType, ok := state.paymentTypes[row.paymentType]
		if !ok {
			row.err = fmt.Errorf("row %d: unknown Invoice Ninja Payment Type %q", row.rowNo, row.paymentType)
			return row
		}
		row.paymentTypeID = paymentType.ID
		row.desiredPaymentDate = row.date
		row.desiredPaymentTypeID = paymentType.ID
	}

	businessPct, err := parsePercent(cell(row.rec, idx, "Business %"))
	if err != nil {
		row.err = fmt.Errorf("row %d business %%: %w", row.rowNo, err)
		return row
	}
	row.businessPct = businessPct
	businessAmountCell := cell(row.rec, idx, "Business Amount")
	businessAmount, err := parseExpenseMoney(businessAmountCell)
	if err != nil {
		row.err = fmt.Errorf("row %d business amount: %w", row.rowNo, err)
		return row
	}
	businessGST, err := parseExpenseMoney(cell(row.rec, idx, "Business GST"))
	if err != nil {
		row.err = fmt.Errorf("row %d business GST: %w", row.rowNo, err)
		return row
	}
	total, err := parseExpenseMoney(cell(row.rec, idx, "Total Inc GST"))
	if err != nil {
		row.err = fmt.Errorf("row %d total inc GST: %w", row.rowNo, err)
		return row
	}
	if shouldDeriveBusinessAmount(businessAmountCell, businessPct) {
		businessAmount = total * businessPct / 100
	}
	row.businessAmount = businessAmount
	row.businessGST = businessGST
	row.grossCents = moneyCents(total)
	row.grossKnown = true
	if row.paymentType == "" && row.grossCents > 0 {
		row.remainingCents = row.grossCents
	}

	receiptPath, err := receipts.match(row.receiptName)
	if err != nil {
		row.err = fmt.Errorf("row %d receipt: %w", row.rowNo, err)
		return row
	}
	row.receiptPath = receiptPath
	return row
}

func assignExpenseRowIdentities(rows []*preparedExpenseImportRow) {
	markExactPurchaseDuplicates(rows)
	for _, row := range rows {
		if row.importID == "" {
			continue
		}
		row.sourceBaseMarker = importIDMarker(row.importID)
		row.sourceMarker = row.sourceBaseMarker
		if isPurchaseRow(row) && row.paymentType == "" {
			row.supplierAccountMarker = supplierAccountMarker(row.supplier)
			row.settlementPurchaseMarker = settlementPurchaseMarker(row.sourceMarker)
		}
	}

	purchaseGroups := make(map[string][]*preparedExpenseImportRow)
	for _, row := range rows {
		if row.importID != "" || !isPurchaseRow(row) || row.duplicateRow != 0 || row.sourceBaseMarker == "" {
			continue
		}
		purchaseGroups[row.sourceBaseMarker] = append(purchaseGroups[row.sourceBaseMarker], row)
	}
	for baseMarker, group := range purchaseGroups {
		sort.SliceStable(group, func(i, j int) bool { return group[i].rowNo < group[j].rowNo })
		for i, row := range group {
			row.sourceMarker = purchaseOccurrenceMarker(baseMarker, i+1)
			if row.paymentType == "" {
				row.supplierAccountMarker = supplierAccountMarker(row.supplier)
				row.settlementPurchaseMarker = settlementPurchaseMarker(row.sourceMarker)
			}
		}
	}

	paymentGroups := make(map[string][]*preparedExpenseImportRow)
	exactPayments := make(map[string][]*preparedExpenseImportRow)
	for _, row := range rows {
		if row.importID != "" || row.documentType != documentTypeAccountPayment || row.err != nil || row.sourceBaseMarker == "" {
			continue
		}
		exactKey := strings.Join(trimmedValues(row.rec), "\x00")
		exactPayments[exactKey] = append(exactPayments[exactKey], row)
	}
	for _, group := range exactPayments {
		if len(group) < 2 {
			continue
		}
		rowNumbers := make([]string, 0, len(group))
		for _, row := range group {
			rowNumbers = append(rowNumbers, strconv.Itoa(row.rowNo))
		}
		for _, row := range group {
			row.err = fmt.Errorf("row %d: Account Payment identity is ambiguous; source rows %s are identical", row.rowNo, strings.Join(rowNumbers, ", "))
		}
	}
	for _, row := range rows {
		if row.importID != "" || row.documentType != documentTypeAccountPayment || row.err != nil || row.sourceBaseMarker == "" {
			continue
		}
		paymentGroups[row.sourceBaseMarker] = append(paymentGroups[row.sourceBaseMarker], row)
	}
	for baseMarker, group := range paymentGroups {
		sort.SliceStable(group, func(i, j int) bool { return group[i].rowNo < group[j].rowNo })
		for i, row := range group {
			row.sourceMarker = accountPaymentOccurrenceMarker(baseMarker, i+1)
		}
	}
}

func markExactPurchaseDuplicates(rows []*preparedExpenseImportRow) {
	seen := make(map[string]int)
	for _, row := range rows {
		if row.importID != "" || !isPurchaseRow(row) || row.duplicateMarker == "" {
			continue
		}
		if previous, ok := seen[row.duplicateMarker]; ok {
			row.duplicateRow = previous
			continue
		}
		seen[row.duplicateMarker] = row.rowNo
	}
}

func validateExistingExpenseIdentities(state *expenseImportState, rows []*preparedExpenseImportRow) {
	matched := make(map[string]*preparedExpenseImportRow)
	for _, row := range rows {
		if !isPurchaseRow(row) || row.err != nil || row.duplicateRow != 0 {
			continue
		}
		existing, exists, ambiguous := existingExpenseForRow(state, row)
		if ambiguous {
			row.err = fmt.Errorf("row %d: stable source identity matches multiple existing expenses", row.rowNo)
			continue
		}
		if !exists {
			continue
		}
		if previous, ok := matched[existing.ID]; ok {
			previous.err = fmt.Errorf("row %d: source identity also matches row %d and cannot be determined safely", previous.rowNo, row.rowNo)
			row.err = fmt.Errorf("row %d: source identity also matches row %d and cannot be determined safely", row.rowNo, previous.rowNo)
			continue
		}
		matched[existing.ID] = row
		existingCopy := existing
		row.existingExpense = &existingCopy
	}
}

func existingExpenseForRow(state *expenseImportState, row *preparedExpenseImportRow) (invoiceninja.Expense, bool, bool) {
	// The v0.5.1 legacy marker is only a migration fallback. Once an Import ID
	// marker exists, it is authoritative even when mutable source fields happen
	// to resemble a different legacy record.
	if row.importID != "" {
		if state.ambiguousMarkers[row.sourceMarker] {
			return invoiceninja.Expense{}, false, true
		}
		if existing, ok := state.expenseByMarker[row.sourceMarker]; ok {
			return existing, true, false
		}
	}
	if row.legacyMarker != "" {
		if existing, ok := state.expenseByMarker[row.legacyMarker]; ok {
			return existing, true, false
		}
		if state.ambiguousMarkers[row.legacyMarker] {
			return invoiceninja.Expense{}, false, true
		}
	}
	if row.importID == "" {
		if existing, ok := state.expenseByMarker[row.sourceMarker]; ok {
			return existing, true, false
		}
	}
	if state.ambiguousMarkers[row.sourceMarker] || state.ambiguousBases[row.sourceBaseMarker] {
		return invoiceninja.Expense{}, false, true
	}
	return invoiceninja.Expense{}, false, false
}

func isPurchaseRow(row *preparedExpenseImportRow) bool {
	return row.documentType == documentTypeInvoice || row.documentType == documentTypeReceipt
}

func deferredAdjustmentResult(row *preparedExpenseImportRow) CSVImportResult {
	result := expenseResultForRow(row)
	result.Action = "deferred"
	result.Changes = []string{"adjustment:deferred:no-safe-settlement-rule"}
	return result
}

func (s *Service) preflightExpensePaymentNotifications(ctx context.Context, state *expenseImportState, rows []*preparedExpenseImportRow, actions []expenseSettlementAction, dryRun bool) error {
	if dryRun || !couldSetExpensePaymentDate(state, rows, actions) {
		return nil
	}
	company := state.currentCompany
	if company == nil {
		var err error
		company, err = s.client.Companies.Current(ctx)
		if err != nil {
			return fmt.Errorf("verify Invoice Ninja notify_vendor_when_paid before writes: %w", err)
		}
	}
	if company.NotifyVendorWhenPaid == nil {
		return fmt.Errorf("verify Invoice Ninja notify_vendor_when_paid before writes: current company response did not expose the setting")
	}
	if *company.NotifyVendorWhenPaid {
		return fmt.Errorf("refusing expense import before any writes: Invoice Ninja notify_vendor_when_paid is enabled")
	}
	return nil
}

func couldSetExpensePaymentDate(state *expenseImportState, rows []*preparedExpenseImportRow, actions []expenseSettlementAction) bool {
	for _, row := range rows {
		if !isPurchaseRow(row) || row.err != nil || row.duplicateRow != 0 || row.desiredPaymentDate == "" {
			continue
		}
		existing, exists, ambiguous := existingExpenseForRow(state, row)
		if ambiguous {
			continue
		}
		if !exists || existing.PaymentDate != row.desiredPaymentDate {
			return true
		}
	}
	for _, action := range actions {
		if action.desiredDate != "" && action.expense.PaymentDate != action.desiredDate {
			return true
		}
	}
	return false
}

func (s *Service) importAccountPaymentRow(ctx context.Context, state *expenseImportState, row *preparedExpenseImportRow, dryRun bool) CSVImportResult {
	res := expenseResultForRow(row)
	if row.existingTransaction != nil {
		res.ID = row.existingTransaction.ID
		needsIdentityUpdate := row.importID != "" && !strings.Contains(row.existingTransaction.Description, row.sourceMarker)
		res.Action = "unchanged"
		res.Changes = []string{"transaction:already-imported", "bank-account:" + goTradieBankAccountName, "transaction-marker:" + row.sourceMarker}
		if needsIdentityUpdate {
			res.Action = "would-update-transaction"
			res.Changes = append(res.Changes, "transaction-marker:add")
		}
		if dryRun {
			return refreshAccountPaymentResult(res, row)
		}
		if needsIdentityUpdate {
			existing := *row.existingTransaction
			description := appendAccountPaymentIdentity(existing.Description, row)
			updated, err := s.client.BankTransactions.Update(ctx, existing.ID, invoiceninja.UpdateBankTransactionRequest{
				BankIntegrationID: existing.BankIntegrationID,
				CurrencyID:        existing.CurrencyID,
				Amount:            existing.Amount,
				BaseType:          existing.BaseType,
				Date:              existing.Date,
				Description:       description,
				Participant:       existing.Participant,
				ParticipantName:   existing.ParticipantName,
			})
			if err != nil {
				res.Action = "error"
				res.Error = fmt.Errorf("row %d persist Account Payment Import ID: %w", row.rowNo, err)
				return res
			}
			updated.Description = description
			row.existingTransaction = updated
			state.transactionByMarker[row.sourceMarker] = *updated
			res.Action = "updated"
		}
		return res
	}
	res.Action = "would-create-transaction"
	res.Changes = []string{"transaction:create:DEBIT", "bank-account:" + goTradieBankAccountName, "transaction-marker:" + row.sourceMarker}
	if dryRun {
		return refreshAccountPaymentResult(res, row)
	}
	if state.bankIntegration == nil {
		res.Action = "error"
		res.Error = fmt.Errorf("row %d: Invoice Ninja manual bank account %q is unavailable", row.rowNo, goTradieBankAccountName)
		return res
	}
	created, err := s.client.BankTransactions.Create(ctx, invoiceninja.CreateBankTransactionRequest{
		BankIntegrationID: state.bankIntegration.ID,
		CurrencyID:        state.companyCurrencyID,
		Amount:            centsAmount(row.grossCents),
		BaseType:          "DEBIT",
		Date:              row.date,
		Description:       accountPaymentDescription(row),
		ParticipantName:   row.supplier,
	})
	if err != nil {
		res.Action = "error"
		res.Error = fmt.Errorf("row %d create supplier withdrawal Transaction: %w", row.rowNo, err)
		return res
	}
	row.existingTransaction = created
	state.bankTransactions = append(state.bankTransactions, *created)
	state.transactionByMarker[row.sourceMarker] = *created
	res.ID = created.ID
	res.Action = "created-transaction"
	return res
}

func refreshAccountPaymentResult(res CSVImportResult, row *preparedExpenseImportRow) CSVImportResult {
	changes := make([]string, 0, len(row.allocations)+2)
	for _, allocation := range row.allocations {
		changes = append(changes, fmt.Sprintf("allocate:%s:%.2f", allocation.PurchaseSourceID, allocation.Amount))
	}
	changes = append(changes, fmt.Sprintf("unapplied:%.2f", centsAmount(row.unappliedCents)))
	if row.receiptName != "" {
		changes = append(changes, "supporting-document:unattached:"+row.receiptName)
	}
	res.Changes = append(res.Changes, changes...)
	res.Allocations = append([]ExpensePaymentAllocation(nil), row.allocations...)
	res.UnappliedAmount = centsAmount(row.unappliedCents)
	return res
}

func (s *Service) importExpenseRow(ctx context.Context, state *expenseImportState, idx map[string]int, row *preparedExpenseImportRow, dryRun, deferSupplierSettlement bool) CSVImportResult {
	res := expenseResultForRow(row)
	res.Allocations = append([]ExpensePaymentAllocation(nil), row.allocations...)
	if previousRow, ok := state.seenMarkers[row.sourceMarker]; ok {
		res.Action = "unchanged"
		res.Changes = []string{fmt.Sprintf("duplicate-source-row:%d", previousRow)}
		return res
	}
	state.seenMarkers[row.sourceMarker] = row.rowNo

	existing, exists, ambiguous := existingExpenseForRow(state, row)
	if ambiguous {
		res.Action = "error"
		res.Error = fmt.Errorf("row %d: stable source identity matches multiple existing expenses", row.rowNo)
		return res
	}
	refs, err := s.resolveExpenseImportReferences(ctx, state, idx, row, dryRun)
	if err != nil {
		res.Action = "error"
		res.Error = err
		return res
	}
	targetPaymentDate := row.desiredPaymentDate
	targetPaymentTypeID := row.desiredPaymentTypeID
	if deferSupplierSettlement && row.paymentType == "" {
		if exists {
			targetPaymentDate = existing.PaymentDate
			targetPaymentTypeID = existing.PaymentTypeID
		} else {
			targetPaymentDate = ""
			targetPaymentTypeID = ""
		}
	}
	payload := expenseCreateRequest(row, idx, refs, targetPaymentDate, targetPaymentTypeID)
	if exists {
		res.ID = existing.ID
		payload.PrivateNotes = mergeExpensePrivateNotes(existing.PrivateNotes, payload.PrivateNotes)
		desired := expenseUpdateRequest(payload)
		changes := append([]string{"already-imported"}, refs.changes...)
		corrections := expenseCorrectionChanges(existing, desired)
		changes = append(changes, corrections...)
		receiptChanged := row.receiptOwner && row.receiptPath != "" && !row.receiptHasDocument
		if receiptChanged {
			changes = append(changes, "receipt:upload:"+row.receiptName)
		}
		if summary := allocationSummary(row); summary != "" {
			changes = append(changes, summary)
		}
		res.Changes = changes
		if len(corrections) == 0 && !receiptChanged {
			res.Action = "unchanged"
			rememberExpense(state, row, existing)
			return res
		}
		if dryRun {
			res.Action = "would-update"
			rememberExpense(state, row, existing)
			return res
		}
		updated := &existing
		if len(corrections) > 0 {
			updatedExpense, err := s.client.Expenses.Update(ctx, existing.ID, desired)
			if err != nil {
				res.Action = "error"
				res.Error = fmt.Errorf("row %d update expense correction: %w", row.rowNo, err)
				return res
			}
			applyExpenseUpdateState(updatedExpense, desired, existing.Documents)
			updated = updatedExpense
		}
		if receiptChanged {
			updatedExpense, err := s.client.Expenses.UploadDocumentFile(ctx, existing.ID, row.receiptPath)
			if err != nil {
				res.Action = "error"
				res.Error = fmt.Errorf("row %d upload receipt %q: %w", row.rowNo, row.receiptName, err)
				return res
			}
			if updatedExpense.PrivateNotes == "" {
				updatedExpense.PrivateNotes = updated.PrivateNotes
			}
			if updatedExpense.PaymentDate == "" {
				updatedExpense.PaymentDate = updated.PaymentDate
			}
			if updatedExpense.PaymentTypeID == "" {
				updatedExpense.PaymentTypeID = updated.PaymentTypeID
			}
			row.receiptHasDocument = true
			updated = updatedExpense
		}
		if row.receiptOwner {
			updatedExpense, err := s.persistReceiptOwner(ctx, state, row, *updated)
			if err != nil {
				res.Action = "error"
				res.Error = fmt.Errorf("row %d persist receipt owner: %w", row.rowNo, err)
				return res
			}
			updated = updatedExpense
		}
		rememberExpense(state, row, *updated)
		res.Action = "updated"
		return res
	}
	changes := append([]string(nil), refs.changes...)
	changes = append(changes, "expense:create")
	if row.paymentType == "" && row.supplierAccountMarker != "" && row.settlementPurchaseMarker != "" {
		changes = append(changes, "supplier-account-marker:add")
	}
	changes = append(changes, paymentChangeDescription(row))
	if summary := allocationSummary(row); summary != "" {
		changes = append(changes, summary)
	}
	if row.receiptOwner && row.receiptPath != "" {
		changes = append(changes, "receipt:upload:"+row.receiptName)
	} else if row.receiptKey != "" {
		changes = append(changes, "receipt:shared:"+row.receiptName)
	}
	res.Changes = changes
	if dryRun {
		res.Action = "would-create"
		return res
	}

	created, err := s.client.Expenses.Create(ctx, payload)
	if err != nil {
		res.Action = "error"
		res.Error = fmt.Errorf("row %d create expense: %w", row.rowNo, err)
		return res
	}
	if created.PrivateNotes == "" {
		created.PrivateNotes = payload.PrivateNotes
	}
	if created.PaymentDate == "" {
		created.PaymentDate = payload.PaymentDate
	}
	if created.PaymentTypeID == "" {
		created.PaymentTypeID = payload.PaymentTypeID
	}
	res.ID = created.ID
	rememberExpense(state, row, *created)
	if row.receiptOwner && row.receiptPath != "" {
		updated, err := s.client.Expenses.UploadDocumentFile(ctx, created.ID, row.receiptPath)
		if err != nil {
			res.Action = "error"
			res.Error = fmt.Errorf("row %d upload receipt %q: %w", row.rowNo, row.receiptName, err)
			return res
		}
		if updated.PrivateNotes == "" {
			updated.PrivateNotes = created.PrivateNotes
		}
		if updated.PaymentDate == "" {
			updated.PaymentDate = created.PaymentDate
		}
		if updated.PaymentTypeID == "" {
			updated.PaymentTypeID = created.PaymentTypeID
		}
		row.receiptHasDocument = true
		rememberExpense(state, row, *updated)
		created = updated
	}
	if row.receiptOwner {
		updated, err := s.persistReceiptOwner(ctx, state, row, *created)
		if err != nil {
			res.Action = "error"
			res.Error = fmt.Errorf("row %d persist receipt owner: %w", row.rowNo, err)
			return res
		}
		rememberExpense(state, row, *updated)
	}
	res.Action = "created"
	return res
}

func rememberExpense(state *expenseImportState, row *preparedExpenseImportRow, expense invoiceninja.Expense) {
	state.expenseByMarker[row.sourceMarker] = expense
	if row.importID == "" && row.legacyMarker != "" {
		state.expenseByMarker[row.legacyMarker] = expense
	}
	expenseCopy := expense
	row.existingExpense = &expenseCopy
}

func paymentChangeDescription(row *preparedExpenseImportRow) string {
	if row.desiredPaymentDate == "" {
		return "payment:unpaid"
	}
	method := row.paymentType
	if len(row.allocations) > 0 {
		if row.desiredPaymentTypeID == "" {
			method = "mixed"
		} else {
			method = row.allocations[0].PaymentType
		}
	}
	return "payment:paid:" + row.desiredPaymentDate + ":" + method
}

func allocationSummary(row *preparedExpenseImportRow) string {
	if len(row.allocations) == 0 {
		return ""
	}
	var allocated int64
	for _, allocation := range row.allocations {
		allocated += moneyCents(allocation.Amount)
	}
	return fmt.Sprintf("allocated:%.2f/%.2f", centsAmount(allocated), centsAmount(row.grossCents))
}

func shouldDeriveBusinessAmount(businessAmount string, businessPct float64) bool {
	return strings.TrimSpace(businessAmount) == "" && businessPct > 0
}

func expenseVendorName(supplier, store string) string {
	supplier = strings.TrimSpace(supplier)
	store = strings.TrimSpace(store)
	if store == "" {
		return supplier
	}
	return supplier + " - " + store
}

func deriveBASTreatment(taxTreatment string, businessPct, businessGST float64) string {
	taxTreatment = strings.ToLower(strings.TrimSpace(taxTreatment))
	if businessPct <= 0 || strings.Contains(taxTreatment, "personal") || strings.Contains(taxTreatment, "non-deductible") {
		return "Private/Non-deductible"
	}
	if businessGST > 0 {
		return "GST Credit"
	}
	return "Review"
}

func projectNumber(rec []string, idx map[string]int) string {
	value := strings.TrimSpace(cell(rec, idx, "Job Number"))
	if value == "" {
		return ""
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return value
}

func indexReceipts(root string) (receiptIndex, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("receipts root %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("receipts root %q is not a directory", root)
	}
	index := make(receiptIndex)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		index[entry.Name()] = append(index[entry.Name()], path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("index receipts root %q: %w", root, err)
	}
	for name := range index {
		sort.Strings(index[name])
	}
	return index, nil
}

func (index receiptIndex) match(filename string) (string, error) {
	if index == nil || filename == "" {
		return "", nil
	}
	matches := index[filename]
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("%q was not found", filename)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("%q is ambiguous; exact matches: %s", filename, strings.Join(matches, ", "))
	}
}

func hasDocument(documents []invoiceninja.Document, filename string) bool {
	for _, document := range documents {
		if document.Name == filename {
			return true
		}
	}
	return false
}

func expensePrivateNotes(rec []string, idx map[string]int, rowNo int, marker string) string {
	var lines []string
	add := func(label, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			lines = append(lines, label+": "+value)
		}
	}
	add("Store", cell(rec, idx, "Store"))
	add("Source file", cell(rec, idx, "File Name"))
	add("Job number", cell(rec, idx, "Job Number"))
	add("Child job number", cell(rec, idx, "Child Job Number"))
	add("Capital check", cell(rec, idx, "Capital Check"))
	add("Item number", cell(rec, idx, "Item Number"))
	add("Item description", cell(rec, idx, "Item Description"))
	add("Qty", cell(rec, idx, "Qty"))
	add("Unit", cell(rec, idx, "Unit"))
	add("Unit price", cell(rec, idx, "Unit Price"))
	add("Source total ex GST", cell(rec, idx, "Total Ex GST"))
	add("Source GST", cell(rec, idx, "GST"))
	add("Source total inc GST", cell(rec, idx, "Total Inc GST"))
	add("Source currency", cell(rec, idx, "$ Currency"))
	add("Business amount", cell(rec, idx, "Business Amount"))
	add("Business GST", cell(rec, idx, "Business GST"))
	add("Notes", cell(rec, idx, "Notes"))
	add("GoTradie Import ID", cell(rec, idx, "Import ID"))
	lines = append(lines, fmt.Sprintf("GoTradie source row: %d", rowNo), marker)
	return strings.Join(lines, "\n")
}

func importIDMarker(importID string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(importID)))
	return importIDMarkerPrefix + encoded + "]"
}

func importIDFromMarker(marker string) (string, bool) {
	if !strings.HasPrefix(marker, importIDMarkerPrefix) || !strings.HasSuffix(marker, "]") {
		return "", false
	}
	encoded := strings.TrimSuffix(strings.TrimPrefix(marker, importIDMarkerPrefix), "]")
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || strings.TrimSpace(string(decoded)) == "" {
		return "", false
	}
	return string(decoded), true
}

func purchaseSourceMarker(rec []string, idx map[string]int, normalizedDate string) string {
	return purchaseSourceMarkerFromValues(purchaseIdentityValues(
		normalizedDate,
		cell(rec, idx, "Supplier"),
		cell(rec, idx, "Store"),
		cell(rec, idx, "Invoice Number"),
		cell(rec, idx, "Item Number"),
		cell(rec, idx, "Item Description"),
		cell(rec, idx, "Qty"),
		cell(rec, idx, "Unit"),
		cell(rec, idx, "Unit Price"),
		cell(rec, idx, "Total Ex GST"),
		cell(rec, idx, "GST"),
		cell(rec, idx, "Total Inc GST"),
		cell(rec, idx, "$ Currency"),
	))
}

func purchaseIdentityValues(date, supplier, store, invoiceNumber, itemNumber, itemDescription, qty, unit, unitPrice, totalExGST, gst, totalIncGST, currency string) []string {
	return []string{
		strings.TrimSpace(date),
		strings.TrimSpace(supplier),
		strings.TrimSpace(store),
		strings.TrimSpace(invoiceNumber),
		strings.TrimSpace(itemNumber),
		strings.TrimSpace(itemDescription),
		canonicalNumberIdentity(qty),
		strings.TrimSpace(unit),
		canonicalMoneyIdentity(unitPrice),
		canonicalMoneyIdentity(totalExGST),
		canonicalMoneyIdentity(gst),
		canonicalMoneyIdentity(totalIncGST),
		strings.TrimSpace(currency),
	}
}

func purchaseSourceMarkerFromValues(values []string) string {
	return hashedMarker("[GoTradie source:v2:", values)
}

func purchaseOccurrenceMarker(baseMarker string, occurrence int) string {
	if occurrence <= 1 {
		return baseMarker
	}
	return hashedMarker("[GoTradie source:v3:", []string{baseMarker, strconv.Itoa(occurrence)})
}

func accountPaymentBaseMarker(row *preparedExpenseImportRow) string {
	return hashedMarker("[GoTradie account-payment:v2:", []string{
		row.date,
		strings.TrimSpace(row.supplier),
		strconv.FormatInt(row.grossCents, 10),
		strings.TrimSpace(row.reference),
		strings.TrimSpace(row.paymentType),
	})
}

func accountPaymentOccurrenceMarker(baseMarker string, occurrence int) string {
	if occurrence <= 1 {
		return baseMarker
	}
	return hashedMarker("[GoTradie account-payment:v3:", []string{baseMarker, strconv.Itoa(occurrence)})
}

func hashedMarker(prefix string, values []string) string {
	h := sha256.New()
	for _, value := range values {
		_, _ = h.Write([]byte(value))
		_, _ = h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	return prefix + hex.EncodeToString(sum[:12]) + "]"
}

// sourceMarker is the legacy full-row marker retained for migration matching.
func sourceMarker(rec []string) string {
	return hashedMarker("[GoTradie source:", trimmedValues(rec))
}

func purchaseDuplicateMarker(rec []string, idx map[string]int) string {
	values := append([]string(nil), rec...)
	for _, name := range []string{"Document Type", "Payment Type"} {
		if column, ok := idx[name]; ok && column >= 0 && column < len(values) {
			values[column] = ""
		}
	}
	return hashedMarker("[GoTradie purchase-row:", trimmedValues(values))
}

func trimmedValues(values []string) []string {
	trimmed := make([]string, len(values))
	for i, value := range values {
		trimmed[i] = strings.TrimSpace(value)
	}
	return trimmed
}

func sourceMarkerFromNotes(notes string) string {
	if marker := markerFromText(notes, importIDMarkerPrefix); marker != "" {
		return marker
	}
	const prefix = "[GoTradie source:"
	start := strings.Index(notes, prefix)
	if start < 0 {
		return ""
	}
	end := strings.Index(notes[start:], "]")
	if end < 0 {
		return ""
	}
	return notes[start : start+end+1]
}

func privateNoteValues(notes string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(notes, "\n") {
		separator := strings.Index(line, ": ")
		if separator <= 0 {
			continue
		}
		values[line[:separator]] = strings.TrimSpace(line[separator+2:])
	}
	return values
}

func canonicalMoneyIdentity(value string) string {
	amount, err := parseExpenseMoney(value)
	if err != nil {
		return strings.TrimSpace(value)
	}
	return strconv.FormatInt(moneyCents(amount), 10)
}

func canonicalNumberIdentity(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	if value == "" {
		return ""
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return value
	}
	return strconv.FormatFloat(number, 'g', -1, 64)
}

func parseExpenseDate(value string) (string, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02", "2/1/2006", "02/01/2006", "2/01/2006", "02/1/2006"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("unsupported date %q", value)
}

func parsePercent(value string) (float64, error) {
	value = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "%"))
	if value == "" {
		return 100, nil
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", ""), 64)
	if err != nil {
		return 0, err
	}
	if v < 0 || v > 100 {
		return 0, fmt.Errorf("percentage %.4g outside 0..100", v)
	}
	return v, nil
}

func parseExpenseMoney(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	negative := strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")")
	value = strings.Trim(value, "()")
	for _, token := range []string{"$", "AUD", "USD", "NZD", " ", ","} {
		value = strings.ReplaceAll(value, token, "")
	}
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	if negative {
		v = -v
	}
	return v, nil
}

func moneyCents(amount float64) int64 {
	return int64(math.Round(amount * 100))
}

func centsAmount(cents int64) float64 {
	return float64(cents) / 100
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func key(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
