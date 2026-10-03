package ninja

import (
	"context"
	"crypto/sha256"
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
	bankCurrencyID              string
	bankTransactions            []invoiceninja.BankTransaction
	transactionByMarker         map[string]invoiceninja.BankTransaction
	ambiguousTransactionMarkers map[string]bool
	bankTransactionRules        []invoiceninja.BankTransactionRule
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
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	recs, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, fmt.Errorf("CSV is empty")
	}

	idx := headerIndex(recs[0])
	for _, name := range expenseImportRequiredColumns {
		if _, ok := idx[name]; !ok {
			return nil, fmt.Errorf("missing required column %q", name)
		}
	}
	if strings.TrimSpace(receiptsRoot) != "" {
		if _, ok := idx["File Name"]; !ok {
			return nil, fmt.Errorf("missing required column %q when --receipts-root is used", "File Name")
		}
	}

	receipts, err := indexReceipts(receiptsRoot)
	if err != nil {
		return nil, err
	}

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
		rows = append(rows, prepareExpenseImportRow(state, receipts, idx, rec, rowNo+2))
	}
	assignExpenseRowIdentities(rows)
	validateExistingExpenseIdentities(state, rows)
	validateExistingTransactionIdentities(state, rows)
	settlementActions, err := allocateAccountPayments(state, rows, false)
	if err != nil {
		return nil, err
	}
	if err := preflightBankTransactionRules(state, rows); err != nil {
		return nil, err
	}
	if err := s.preflightExpensePaymentNotifications(ctx, state, rows, settlementActions, dryRun); err != nil {
		return nil, err
	}

	if dryRun {
		results := make([]CSVImportResult, len(rows))
		for i, row := range rows {
			results[i] = s.previewExpenseImportRow(ctx, state, idx, row)
		}
		return append(results, previewRemoteSettlementActions(settlementActions)...), nil
	}

	// Commit in three stages: purchases without speculative supplier settlement,
	// durable withdrawal Transactions, then derived Paid/Unpaid state.
	results := make([]CSVImportResult, len(rows))
	for i, row := range rows {
		if row.err != nil || row.duplicateRow != 0 || row.documentType == documentTypeAdjustment {
			results[i] = s.previewExpenseImportRow(ctx, state, idx, row)
			continue
		}
		if isPurchaseRow(row) {
			results[i] = s.importExpenseRow(ctx, state, idx, row, false, true)
			continue
		}
		if row.documentType != documentTypeAccountPayment {
			results[i] = unsupportedExpenseImportRow(row)
		}
	}
	for i, row := range rows {
		if row.documentType != documentTypeAccountPayment || row.err != nil || row.duplicateRow != 0 {
			continue
		}
		results[i] = s.importAccountPaymentRow(ctx, state, row, false)
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
	expenses, err := s.client.Expenses.ListAll(ctx, invoiceninja.ExpenseQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Include: []string{"documents"}},
		WithTrashed: false,
	})
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
		if expense.IsDeleted {
			continue
		}
		if marker := sourceMarkerFromNotes(expense.PrivateNotes); marker != "" {
			addExpenseMarker(state, marker, expense)
			state.importedExpenses = append(state.importedExpenses, importedExpenseState{
				expense:                  expense,
				marker:                   marker,
				baseMarker:               expensePurchaseBaseMarker(expense, state.vendorByID),
				supplier:                 expenseSourceSupplier(expense, state.vendorByID),
				date:                     expense.Date,
				sourceRow:                sourceRowFromNotes(expense.PrivateNotes),
				supplierAccountMarker:    supplierAccountMarkerFromText(expense.PrivateNotes),
				settlementPurchaseMarker: settlementPurchaseMarkerFromText(expense.PrivateNotes),
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
	if marker == "" || strings.HasPrefix(marker, "[GoTradie source:v2:") {
		return ""
	}
	return expensePurchaseBaseMarker(expense, vendors)
}

func expensePurchaseBaseMarker(expense invoiceninja.Expense, vendors map[string]invoiceninja.Vendor) string {
	if sourceMarkerFromNotes(expense.PrivateNotes) == "" {
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
		documentType: strings.TrimSpace(cell(rec, idx, "Document Type")),
		supplier:     supplier,
		name:         name,
		vendorName:   vendorName,
		categoryName: cell(rec, idx, "Category"),
		paymentType:  strings.TrimSpace(cell(rec, idx, "Payment Type")),
		reference:    cell(rec, idx, "Invoice Number"),
		receiptName:  strings.TrimSpace(cell(rec, idx, "File Name")),
	}
	if row.documentType == documentTypeInvoice || row.documentType == documentTypeReceipt {
		row.legacyMarker = sourceMarker(rec)
		row.duplicateMarker = purchaseDuplicateMarker(rec, idx)
	}
	if row.documentType == documentTypeAdjustment {
		row.name = strings.TrimSpace(supplier + " adjustment")
		row.deferred = true
		return row
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
		if row.grossCents <= 0 {
			row.err = fmt.Errorf("row %d: Account Payment Total Inc GST must be greater than zero", rowNo)
			return row
		}
		row.unappliedCents = row.grossCents
		row.sourceBaseMarker = accountPaymentBaseMarker(row)
		row.sourceMarker = row.sourceBaseMarker
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
	row.sourceBaseMarker = purchaseSourceMarker(row.rec, idx, row.date)
	row.sourceMarker = row.sourceBaseMarker
	row.legacyMarker = sourceMarker(row.rec)
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

	purchaseGroups := make(map[string][]*preparedExpenseImportRow)
	for _, row := range rows {
		if !isPurchaseRow(row) || row.duplicateRow != 0 || row.sourceBaseMarker == "" {
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
		if row.documentType != documentTypeAccountPayment || row.err != nil || row.sourceBaseMarker == "" {
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
		if row.documentType != documentTypeAccountPayment || row.err != nil || row.sourceBaseMarker == "" {
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
		if !isPurchaseRow(row) || row.duplicateMarker == "" {
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
	if row.legacyMarker != "" {
		if existing, ok := state.expenseByMarker[row.legacyMarker]; ok {
			return existing, true, false
		}
		if state.ambiguousMarkers[row.legacyMarker] {
			return invoiceninja.Expense{}, false, true
		}
	}
	if existing, ok := state.expenseByMarker[row.sourceMarker]; ok {
		return existing, true, false
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
	return CSVImportResult{
		Name:    row.name,
		Action:  "deferred",
		Changes: []string{"adjustment:deferred:no-safe-settlement-rule"},
	}
}

func (s *Service) preflightExpensePaymentNotifications(ctx context.Context, state *expenseImportState, rows []*preparedExpenseImportRow, actions []expenseSettlementAction, dryRun bool) error {
	if dryRun || !couldSetExpensePaymentDate(state, rows, actions) {
		return nil
	}
	company, err := s.client.Companies.Current(ctx)
	if err != nil {
		return fmt.Errorf("verify Invoice Ninja notify_vendor_when_paid before writes: %w", err)
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
	res := CSVImportResult{Name: row.name}
	if row.existingTransaction != nil {
		res.ID = row.existingTransaction.ID
		res.Action = "unchanged"
		res.Changes = []string{"transaction:already-imported", "bank-account:" + goTradieBankAccountName, "transaction-marker:" + row.sourceMarker}
		if dryRun {
			return refreshAccountPaymentResult(res, row)
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
		CurrencyID:        state.bankCurrencyID,
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
	res := CSVImportResult{
		Name:        row.name,
		Allocations: append([]ExpensePaymentAllocation(nil), row.allocations...),
	}
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
	if exists {
		res.ID = existing.ID
		changes := []string{"already-imported"}
		targetPaymentDate := row.desiredPaymentDate
		targetPaymentTypeID := row.desiredPaymentTypeID
		if deferSupplierSettlement && row.paymentType == "" {
			targetPaymentDate = existing.PaymentDate
			targetPaymentTypeID = existing.PaymentTypeID
		}
		updatedNotes := expenseNotesForRow(existing.PrivateNotes, row)
		notesChanged := updatedNotes != existing.PrivateNotes
		if notesChanged {
			markerChange := "supplier-account-marker:add"
			if row.paymentType != "" {
				markerChange = "supplier-account-marker:remove"
			}
			changes = append(changes, markerChange)
		}
		paymentChanged := existing.PaymentDate != targetPaymentDate || existing.PaymentTypeID != targetPaymentTypeID
		if paymentChanged {
			changes = append(changes, paymentChangeDescription(row))
		}
		receiptChanged := row.receiptPath != "" && !hasDocument(existing.Documents, row.receiptName)
		if receiptChanged {
			changes = append(changes, "receipt:upload:"+row.receiptName)
		}
		if summary := allocationSummary(row); summary != "" {
			changes = append(changes, summary)
		}
		res.Changes = changes
		if !paymentChanged && !notesChanged && !receiptChanged {
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
		if paymentChanged || notesChanged {
			var privateNotes *string
			if notesChanged {
				privateNotes = &updatedNotes
			}
			updatedExpense, err := s.client.Expenses.UpdatePaymentStatus(ctx, existing.ID, invoiceninja.ExpensePaymentStatusRequest{
				PaymentDate:   targetPaymentDate,
				PaymentTypeID: targetPaymentTypeID,
				PrivateNotes:  privateNotes,
			})
			if err != nil {
				res.Action = "error"
				res.Error = fmt.Errorf("row %d update expense payment status: %w", row.rowNo, err)
				return res
			}
			updatedExpense.PaymentDate = targetPaymentDate
			updatedExpense.PaymentTypeID = targetPaymentTypeID
			if privateNotes != nil {
				updatedExpense.PrivateNotes = *privateNotes
			}
			updated = updatedExpense
		}
		if receiptChanged {
			updatedExpense, err := s.client.Expenses.UploadDocumentFile(ctx, existing.ID, row.receiptPath)
			if err != nil {
				res.Action = "error"
				res.Error = fmt.Errorf("row %d upload receipt %q: %w", row.rowNo, row.receiptName, err)
				return res
			}
			updated = updatedExpense
		}
		rememberExpense(state, row, *updated)
		res.Action = "updated"
		return res
	}
	changes := make([]string, 0, 7)
	vendor, vendorExists := state.vendors[key(row.vendorName)]
	if !vendorExists {
		changes = append(changes, "vendor:create:"+row.vendorName)
		if dryRun {
			vendor = invoiceninja.Vendor{Name: row.vendorName}
			state.vendors[key(row.vendorName)] = vendor
		} else {
			created, err := s.client.Vendors.Create(ctx, invoiceninja.CreateVendorRequest{Name: row.vendorName})
			if err != nil {
				res.Action = "error"
				res.Error = fmt.Errorf("row %d create vendor %q: %w", row.rowNo, row.vendorName, err)
				return res
			}
			vendor = *created
			state.vendors[key(row.vendorName)] = vendor
			state.vendorByID[vendor.ID] = vendor
		}
	}

	category, categoryExists := state.categories[key(row.categoryName)]
	if !categoryExists {
		changes = append(changes, "category:create:"+row.categoryName)
		if dryRun {
			category = invoiceninja.ExpenseCategory{Name: row.categoryName}
			state.categories[key(row.categoryName)] = category
		} else {
			created, err := s.client.ExpenseCategories.Create(ctx, invoiceninja.CreateExpenseCategoryRequest{Name: row.categoryName})
			if err != nil {
				res.Action = "error"
				res.Error = fmt.Errorf("row %d create category %q: %w", row.rowNo, row.categoryName, err)
				return res
			}
			category = *created
			state.categories[key(row.categoryName)] = category
		}
	}

	projectID := ""
	jobNumber := projectNumber(row.rec, idx)
	if jobNumber != "" {
		if project, ok := state.projects[key(jobNumber)]; ok {
			projectID = project.ID
		} else if clientID := state.clientByQuote[key(jobNumber)]; clientID != "" {
			changes = append(changes, "project:create:"+jobNumber)
			if dryRun {
				state.projects[key(jobNumber)] = invoiceninja.Project{ClientID: clientID, Name: "Job " + jobNumber, Number: jobNumber}
			} else {
				created, err := s.client.Projects.Create(ctx, invoiceninja.CreateProjectRequest{
					ClientID: clientID,
					Name:     "Job " + jobNumber,
					Number:   jobNumber,
					TaskRate: 0,
				})
				if err != nil {
					res.Action = "error"
					res.Error = fmt.Errorf("row %d create project %q: %w", row.rowNo, jobNumber, err)
					return res
				}
				state.projects[key(jobNumber)] = *created
				projectID = created.ID
			}
		} else {
			changes = append(changes, "project:unresolved:"+jobNumber)
		}
	}

	basTreatment := deriveBASTreatment(cell(row.rec, idx, "Tax Treatment"), row.businessPct, row.businessGST)
	invoiceDocuments := false
	targetPaymentDate := row.desiredPaymentDate
	targetPaymentTypeID := row.desiredPaymentTypeID
	if deferSupplierSettlement && row.paymentType == "" {
		targetPaymentDate = ""
		targetPaymentTypeID = ""
	}
	payload := invoiceninja.CreateExpenseRequest{
		VendorID:             vendor.ID,
		ProjectID:            projectID,
		CategoryID:           category.ID,
		Amount:               row.businessAmount,
		Date:                 row.date,
		PaymentDate:          targetPaymentDate,
		PaymentTypeID:        targetPaymentTypeID,
		PrivateNotes:         expenseNotesForRow(expensePrivateNotes(row.rec, idx, row.rowNo, row.sourceMarker), row),
		TransactionReference: row.reference,
		TaxAmount1:           row.businessGST,
		UsesInclusiveTaxes:   true,
		CalculateTaxByAmount: true,
		InvoiceDocuments:     &invoiceDocuments,
		CustomValue1:         cell(row.rec, idx, "Tax Treatment"),
		CustomValue2:         cell(row.rec, idx, "Option"),
		CustomValue3:         cell(row.rec, idx, "Business %"),
		CustomValue4:         basTreatment,
	}
	if row.businessGST != 0 {
		payload.TaxName1 = "GST"
		payload.TaxRate1 = 10
	}

	changes = append(changes, "expense:create")
	if row.paymentType == "" && row.supplierAccountMarker != "" && row.settlementPurchaseMarker != "" {
		changes = append(changes, "supplier-account-marker:add")
	}
	changes = append(changes, paymentChangeDescription(row))
	if summary := allocationSummary(row); summary != "" {
		changes = append(changes, summary)
	}
	if row.receiptPath != "" {
		changes = append(changes, "receipt:upload:"+row.receiptName)
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
	res.ID = created.ID
	rememberExpense(state, row, *created)
	if row.receiptPath != "" {
		updated, err := s.client.Expenses.UploadDocumentFile(ctx, created.ID, row.receiptPath)
		if err != nil {
			res.Action = "error"
			res.Error = fmt.Errorf("row %d upload receipt %q: %w", row.rowNo, row.receiptName, err)
			return res
		}
		rememberExpense(state, row, *updated)
	}
	res.Action = "created"
	return res
}

func rememberExpense(state *expenseImportState, row *preparedExpenseImportRow, expense invoiceninja.Expense) {
	state.expenseByMarker[row.sourceMarker] = expense
	if row.legacyMarker != "" {
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
	lines = append(lines, fmt.Sprintf("GoTradie source row: %d", rowNo), marker)
	return strings.Join(lines, "\n")
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
