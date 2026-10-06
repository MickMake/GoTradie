package ninja

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
)

const (
	ExpenseImportModeWholeFile      = "whole-file"
	ExpenseImportModeBatched        = "batched"
	ExpenseImportModeSettlementSafe = "settlement-safe whole-file"
)

// ExpenseImportIssue is a source-file problem found before remote execution.
type ExpenseImportIssue struct {
	RowNo    int
	ImportID string
	Message  string
}

func (i ExpenseImportIssue) String() string {
	context := make([]string, 0, 2)
	if i.RowNo > 0 {
		context = append(context, fmt.Sprintf("row %d", i.RowNo))
	}
	if i.ImportID != "" {
		context = append(context, fmt.Sprintf("Import ID %q", i.ImportID))
	}
	if len(context) == 0 {
		return i.Message
	}
	return strings.Join(context, ", ") + ": " + i.Message
}

// ExpenseImportPreflight is the complete local-source validation result.
type ExpenseImportPreflight struct {
	Rows     int
	Mode     string
	Errors   []ExpenseImportIssue
	Warnings []ExpenseImportIssue
}

// ExpenseImportPreflightError reports a failed whole-file local preflight.
type ExpenseImportPreflightError struct {
	Report ExpenseImportPreflight
}

func (e *ExpenseImportPreflightError) Error() string {
	return fmt.Sprintf("expense import preflight failed with %d error(s)", len(e.Report.Errors))
}

// ExpenseImportProgress is emitted as each source row finishes execution.
type ExpenseImportProgress struct {
	Index  int
	Total  int
	Result CSVImportResult
}

// ExpenseImportBatchSummary describes one completed ordinary-Expense batch.
type ExpenseImportBatchSummary struct {
	Number    int
	Processed int
	New       int
	Existing  int
	Updated   int
	Deferred  int
	Errors    int
	Last      bool
}

// ExpenseImportOptions controls v0.5.2 execution without adding durable state.
type ExpenseImportOptions struct {
	DryRun          bool
	ReceiptsRoot    string
	BatchSize       int
	OnPreflight     func(ExpenseImportPreflight)
	OnProgress      func(ExpenseImportProgress)
	OnBatchComplete func(ExpenseImportBatchSummary) error
}

type expenseSourcePreflight struct {
	report      ExpenseImportPreflight
	index       map[string]int
	receipts    receiptIndex
	receiptKeys map[string]string
}

type expenseReceiptUse struct {
	rowNo    int
	importID string
	supplier string
	date     string
	name     string
	key      string
}

func preflightExpenseSource(recs [][]string, receiptsRoot string, batchSize int) expenseSourcePreflight {
	result := expenseSourcePreflight{
		report: ExpenseImportPreflight{Mode: ExpenseImportModeWholeFile},
	}
	if len(recs) == 0 {
		result.report.Errors = append(result.report.Errors, ExpenseImportIssue{Message: "CSV is empty"})
		return result
	}

	result.index = headerIndex(recs[0])
	for _, name := range expenseImportRequiredColumns {
		if _, ok := result.index[name]; !ok {
			result.report.Errors = append(result.report.Errors, ExpenseImportIssue{Message: fmt.Sprintf("missing required column %q", name)})
		}
	}
	if strings.TrimSpace(receiptsRoot) != "" {
		if _, ok := result.index["File Name"]; !ok {
			result.report.Errors = append(result.report.Errors, ExpenseImportIssue{Message: "missing required column \"File Name\" when --receipts-root is used"})
		}
	}
	if len(result.report.Errors) > 0 {
		return result
	}

	receipts, err := indexReceipts(receiptsRoot)
	if err != nil {
		result.report.Errors = append(result.report.Errors, ExpenseImportIssue{Message: err.Error()})
		return result
	}
	result.receipts = receipts
	result.receiptKeys = make(map[string]string)

	seenIDs := make(map[string]int)
	seenRecords := make(map[string]struct {
		rowNo    int
		importID string
	})
	var receiptUses []expenseReceiptUse
	hasAccountPayment := false

	for recordIndex, rec := range recs[1:] {
		if emptyRecord(rec) {
			continue
		}
		result.report.Rows++
		rowNo := recordIndex + 2
		importID := strings.TrimSpace(cell(rec, result.index, "Import ID"))
		if importID == "" {
			result.report.Errors = append(result.report.Errors, ExpenseImportIssue{RowNo: rowNo, Message: "Import ID is required"})
		} else if previous, ok := seenIDs[importID]; ok {
			result.report.Errors = append(result.report.Errors, ExpenseImportIssue{
				RowNo:    rowNo,
				ImportID: importID,
				Message:  fmt.Sprintf("duplicate Import ID; first used at row %d", previous),
			})
		} else {
			seenIDs[importID] = rowNo
		}

		documentType := strings.TrimSpace(cell(rec, result.index, "Document Type"))
		if documentType == documentTypeAccountPayment {
			hasAccountPayment = true
		}
		validateExpenseSourceRow(&result.report, result.index, receipts, result.receiptKeys, rec, rowNo, importID)

		fingerprintValues := append([]string(nil), rec...)
		fingerprintValues[result.index["Import ID"]] = ""
		fingerprint := strings.Join(trimmedValues(fingerprintValues), "\x00")
		if previous, ok := seenRecords[fingerprint]; ok {
			result.report.Warnings = append(result.report.Warnings, ExpenseImportIssue{
				RowNo:    rowNo,
				ImportID: importID,
				Message:  fmt.Sprintf("record duplicates row %d (Import ID %q) except for Import ID", previous.rowNo, previous.importID),
			})
		} else {
			seenRecords[fingerprint] = struct {
				rowNo    int
				importID string
			}{rowNo: rowNo, importID: importID}
		}

		filename := strings.TrimSpace(cell(rec, result.index, "File Name"))
		if path, matchErr := receipts.match(filename); matchErr == nil && path != "" {
			receiptUses = append(receiptUses, expenseReceiptUse{
				rowNo:    rowNo,
				importID: importID,
				supplier: strings.TrimSpace(cell(rec, result.index, "Supplier")),
				date:     strings.TrimSpace(cell(rec, result.index, "Date")),
				name:     filename,
				key:      result.receiptKeys[path],
			})
		}
	}

	if hasAccountPayment {
		result.report.Mode = ExpenseImportModeSettlementSafe
	} else if batchSize > 0 && result.report.Rows > batchSize {
		result.report.Mode = ExpenseImportModeBatched
	}
	result.report.Warnings = append(result.report.Warnings, receiptSourceWarnings(receiptUses)...)
	return result
}

func validateExpenseSourceRow(report *ExpenseImportPreflight, idx map[string]int, receipts receiptIndex, receiptKeys map[string]string, rec []string, rowNo int, importID string) {
	addError := func(message string) {
		report.Errors = append(report.Errors, ExpenseImportIssue{RowNo: rowNo, ImportID: importID, Message: message})
	}
	addWarning := func(message string) {
		report.Warnings = append(report.Warnings, ExpenseImportIssue{RowNo: rowNo, ImportID: importID, Message: message})
	}

	supplier := strings.TrimSpace(cell(rec, idx, "Supplier"))
	if supplier == "" {
		addError("Supplier is required")
	}
	if _, err := parseExpenseDate(cell(rec, idx, "Date")); err != nil {
		addError("Date: " + err.Error())
	}

	documentType := strings.TrimSpace(cell(rec, idx, "Document Type"))
	switch documentType {
	case documentTypeInvoice, documentTypeReceipt:
		if strings.TrimSpace(cell(rec, idx, "Category")) == "" {
			addError("Category is required")
		}
		validatePurchaseArithmetic(addError, addWarning, idx, rec)
		filename := strings.TrimSpace(cell(rec, idx, "File Name"))
		path, err := receipts.match(filename)
		if err != nil {
			addError("receipt: " + err.Error())
		} else if path != "" {
			if _, ok := receiptKeys[path]; !ok {
				key, hashErr := receiptContentKey(path)
				if hashErr != nil {
					addError(fmt.Sprintf("receipt %q: %v", filename, hashErr))
				} else {
					receiptKeys[path] = key
				}
			}
		}
	case documentTypeAccountPayment:
		if strings.TrimSpace(cell(rec, idx, "Payment Type")) == "" {
			addError("Payment Type is required for Account Payment")
		}
		amount, err := parseExpenseMoney(cell(rec, idx, "Total Inc GST"))
		if err != nil {
			addError("Total Inc GST: " + err.Error())
		} else if moneyCents(amount) <= 0 {
			addError("Account Payment Total Inc GST must be greater than zero")
		}
	case documentTypeAdjustment:
		// Adjustment remains explicitly deferred by the accepted accounting design.
	case "":
		addError("Document Type is required")
	default:
		addError(fmt.Sprintf("unknown Document Type %q", documentType))
	}
}

func validatePurchaseArithmetic(addError, addWarning func(string), idx map[string]int, rec []string) {
	pct, err := parsePercent(cell(rec, idx, "Business %"))
	if err != nil {
		addError("Business %: " + err.Error())
		return
	}
	inc, incSet, err := optionalExpenseMoney(cell(rec, idx, "Total Inc GST"))
	if err != nil {
		addError("Total Inc GST: " + err.Error())
		return
	}
	ex, exSet, err := optionalExpenseMoney(cell(rec, idx, "Total Ex GST"))
	if err != nil {
		addError("Total Ex GST: " + err.Error())
	}
	gst, gstSet, err := optionalExpenseMoney(cell(rec, idx, "GST"))
	if err != nil {
		addError("GST: " + err.Error())
	}
	businessAmount, businessAmountSet, err := optionalExpenseMoney(cell(rec, idx, "Business Amount"))
	if err != nil {
		addError("Business Amount: " + err.Error())
	}
	businessGST, businessGSTSet, err := optionalExpenseMoney(cell(rec, idx, "Business GST"))
	if err != nil {
		addError("Business GST: " + err.Error())
	}
	if exSet && gstSet && incSet && !moneyNear(ex+gst, inc) {
		addWarning(fmt.Sprintf("Total Ex GST + GST (%.2f) does not match Total Inc GST (%.2f)", ex+gst, inc))
	}
	qty, qtySet, qtyErr := optionalNumber(cell(rec, idx, "Qty"))
	unitPrice, priceSet, priceErr := optionalExpenseMoney(cell(rec, idx, "Unit Price"))
	if qtyErr != nil {
		addWarning("Qty cannot be checked: " + qtyErr.Error())
	}
	if priceErr != nil {
		addWarning("Unit Price cannot be checked: " + priceErr.Error())
	}
	if qtySet && priceSet {
		expected := inc
		expectedSet := incSet
		if exSet {
			expected = ex
			expectedSet = true
		}
		if expectedSet && !moneyNear(qty*unitPrice, expected) {
			addWarning(fmt.Sprintf("Quantity x Unit Price (%.2f) does not match source line total (%.2f)", qty*unitPrice, expected))
		}
	}
	if businessAmountSet && incSet && !moneyNear(businessAmount, inc*pct/100) {
		addWarning(fmt.Sprintf("Business Amount (%.2f) does not match Total Inc GST x Business %% (%.2f)", businessAmount, inc*pct/100))
	}
	if businessGSTSet && gstSet && !moneyNear(businessGST, gst*pct/100) {
		addWarning(fmt.Sprintf("Business GST (%.2f) does not match GST x Business %% (%.2f)", businessGST, gst*pct/100))
	}
	if businessAmountSet && incSet && businessAmount > inc+0.01 {
		addWarning("Business Amount exceeds Total Inc GST")
	}
	if businessGSTSet && gstSet && businessGST > gst+0.01 {
		addWarning("Business GST exceeds source GST")
	}
}

func optionalExpenseMoney(value string) (float64, bool, error) {
	if strings.TrimSpace(value) == "" {
		return 0, false, nil
	}
	parsed, err := parseExpenseMoney(value)
	return parsed, true, err
}

func optionalNumber(value string) (float64, bool, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	if value == "" {
		return 0, false, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	return parsed, true, err
}

func moneyNear(left, right float64) bool {
	return absInt64(moneyCents(left)-moneyCents(right)) <= 1
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func receiptContentKey(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func receiptMarker(key string) string {
	return receiptMarkerPrefix + strings.TrimSpace(key) + "]"
}

func receiptKeyFromText(text string) string {
	marker := markerFromText(text, receiptMarkerPrefix)
	if marker == "" {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(marker, receiptMarkerPrefix), "]")
}

func registerExistingReceiptState(state *expenseImportState, expense invoiceninja.Expense) error {
	key := receiptKeyFromText(expense.PrivateNotes)
	isOwner := strings.Contains(expense.PrivateNotes, receiptOwnerMarker)
	if isOwner && key == "" {
		return fmt.Errorf("Invoice Ninja Expense %q has a receipt-owner marker without a receipt key", expense.ID)
	}
	if !isOwner {
		return nil
	}
	if expense.IsDeleted || expense.ArchivedAt != 0 {
		return fmt.Errorf("receipt owner Expense %q is archived or deleted; restore or explicitly resolve it before importing again", expense.ID)
	}
	if !hasExpectedReceiptDocument(expense) {
		return fmt.Errorf("durable receipt state inconsistent: owner Expense %q has no expected receipt document", expense.ID)
	}
	if existing, ok := state.receiptOwnerByKey[key]; ok && existing.ID != expense.ID {
		state.receiptOwnerAmbiguous[key] = true
		return fmt.Errorf("durable receipt state inconsistent: receipt key %q has multiple owners (%q and %q)", key, existing.ID, expense.ID)
	}
	state.receiptOwnerByKey[key] = expense
	return nil
}

func hasExpectedReceiptDocument(expense invoiceninja.Expense) bool {
	// The durable content key and owner marker identify the physical receipt.
	// Source file is provenance only: a later import may use the same content
	// under a different local filename while the Invoice Ninja attachment keeps
	// its original name.
	return len(expense.Documents) > 0
}

func validateAndAssignReceiptOwnership(state *expenseImportState, rows []*preparedExpenseImportRow) error {
	groups := make(map[string][]*preparedExpenseImportRow)
	for _, row := range rows {
		if row.existingExpense != nil {
			existingKey := receiptKeyFromText(row.existingExpense.PrivateNotes)
			existingName := privateNoteValues(row.existingExpense.PrivateNotes)["Source file"]
			if existingKey != "" && row.receiptKey == "" && row.receiptName != "" && row.receiptName == existingName {
				// A rerun may omit --receipts-root. Retain the already-validated
				// durable key for owners and siblings when the source filename is
				// unchanged.
				row.receiptKey = existingKey
			}
			if strings.Contains(row.existingExpense.PrivateNotes, receiptOwnerMarker) && existingKey != row.receiptKey {
				return fmt.Errorf("durable receipt state inconsistent: owner Expense %q changed from receipt key %q to %q; resolve the existing document explicitly", row.existingExpense.ID, existingKey, row.receiptKey)
			}
		}
		if !isPurchaseRow(row) || row.err != nil || row.receiptKey == "" {
			continue
		}
		groups[row.receiptKey] = append(groups[row.receiptKey], row)
	}
	for key, group := range groups {
		if state.receiptOwnerAmbiguous[key] {
			return fmt.Errorf("durable receipt state inconsistent: receipt key %q has multiple owners", key)
		}
		if owner, ok := state.receiptOwnerByKey[key]; ok {
			for _, row := range group {
				if row.existingExpense != nil && row.existingExpense.ID == owner.ID {
					row.receiptOwner = true
					row.receiptHasDocument = true
				}
			}
			continue
		}

		sort.SliceStable(group, func(i, j int) bool { return group[i].sourceMarker < group[j].sourceMarker })
		owner := group[0]
		for _, candidate := range group {
			if candidate.existingExpense != nil && hasDocument(candidate.existingExpense.Documents, candidate.receiptName) {
				owner = candidate
				break
			}
		}
		owner.receiptOwner = true
		owner.receiptHasDocument = owner.existingExpense != nil && hasDocument(owner.existingExpense.Documents, owner.receiptName)
	}
	return nil
}

func receiptNotesForRow(notes string, row *preparedExpenseImportRow, includeOwner bool) string {
	if row.receiptKey == "" {
		return notes
	}
	lines := []string{strings.TrimSpace(notes)}
	marker := receiptMarker(row.receiptKey)
	if !strings.Contains(notes, marker) {
		lines = append(lines, marker)
	}
	if includeOwner && !strings.Contains(notes, receiptOwnerMarker) {
		lines = append(lines, receiptOwnerMarker)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func identityNotesForRow(notes string, row *preparedExpenseImportRow) string {
	if row.importID == "" || strings.Contains(notes, row.sourceMarker) {
		return notes
	}
	lines := []string{strings.TrimSpace(notes)}
	if !strings.Contains(notes, "GoTradie Import ID: ") {
		lines = append(lines, "GoTradie Import ID: "+row.importID)
	}
	lines = append(lines, row.sourceMarker)
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (s *Service) persistReceiptOwner(ctx context.Context, state *expenseImportState, row *preparedExpenseImportRow, expense invoiceninja.Expense) (*invoiceninja.Expense, error) {
	if !row.receiptOwner || row.receiptKey == "" {
		return &expense, nil
	}
	if !row.receiptHasDocument && !hasDocument(expense.Documents, row.receiptName) {
		return nil, fmt.Errorf("durable receipt state inconsistent: selected owner Expense %q has no receipt document %q", expense.ID, row.receiptName)
	}
	updatedNotes := receiptNotesForRow(expense.PrivateNotes, row, true)
	updated := &expense
	if updatedNotes != expense.PrivateNotes {
		var err error
		updated, err = s.client.Expenses.UpdatePaymentStatus(ctx, expense.ID, invoiceninja.ExpensePaymentStatusRequest{
			PaymentDate:   expense.PaymentDate,
			PaymentTypeID: expense.PaymentTypeID,
			PrivateNotes:  &updatedNotes,
		})
		if err != nil {
			return nil, err
		}
		updated.PrivateNotes = updatedNotes
		if len(updated.Documents) == 0 {
			updated.Documents = expense.Documents
		}
	}
	row.receiptHasDocument = true
	state.receiptOwnerByKey[row.receiptKey] = *updated
	return updated, nil
}

func expenseResultForRow(row *preparedExpenseImportRow) CSVImportResult {
	return CSVImportResult{
		Name:           row.name,
		RowNo:          row.rowNo,
		ImportID:       row.importID,
		Date:           row.date,
		Supplier:       row.supplier,
		BusinessAmount: row.businessAmount,
		ReceiptName:    row.receiptName,
		DocumentType:   row.documentType,
	}
}

type expenseImportReferences struct {
	vendor    invoiceninja.Vendor
	category  invoiceninja.ExpenseCategory
	projectID string
	changes   []string
}

func (s *Service) resolveExpenseImportReferences(ctx context.Context, state *expenseImportState, idx map[string]int, row *preparedExpenseImportRow, dryRun bool) (expenseImportReferences, error) {
	var refs expenseImportReferences
	vendor, exists := state.vendors[key(row.vendorName)]
	if !exists {
		refs.changes = append(refs.changes, "vendor:create:"+row.vendorName)
		if dryRun {
			vendor = invoiceninja.Vendor{Entity: invoiceninja.Entity{ID: "preview-vendor:" + key(row.vendorName)}, Name: row.vendorName}
			state.vendors[key(row.vendorName)] = vendor
		} else {
			created, err := s.client.Vendors.Create(ctx, invoiceninja.CreateVendorRequest{Name: row.vendorName})
			if err != nil {
				return refs, fmt.Errorf("row %d create vendor %q: %w", row.rowNo, row.vendorName, err)
			}
			vendor = *created
			state.vendors[key(row.vendorName)] = vendor
			state.vendorByID[vendor.ID] = vendor
		}
	}
	refs.vendor = vendor

	category, exists := state.categories[key(row.categoryName)]
	if !exists {
		refs.changes = append(refs.changes, "category:create:"+row.categoryName)
		if dryRun {
			category = invoiceninja.ExpenseCategory{Entity: invoiceninja.Entity{ID: "preview-category:" + key(row.categoryName)}, Name: row.categoryName}
			state.categories[key(row.categoryName)] = category
		} else {
			created, err := s.client.ExpenseCategories.Create(ctx, invoiceninja.CreateExpenseCategoryRequest{Name: row.categoryName})
			if err != nil {
				return refs, fmt.Errorf("row %d create category %q: %w", row.rowNo, row.categoryName, err)
			}
			category = *created
			state.categories[key(row.categoryName)] = category
		}
	}
	refs.category = category

	jobNumber := projectNumber(row.rec, idx)
	if jobNumber == "" {
		return refs, nil
	}
	if project, ok := state.projects[key(jobNumber)]; ok {
		refs.projectID = project.ID
		return refs, nil
	}
	clientID := state.clientByQuote[key(jobNumber)]
	if clientID == "" {
		refs.changes = append(refs.changes, "project:unresolved:"+jobNumber)
		return refs, nil
	}
	refs.changes = append(refs.changes, "project:create:"+jobNumber)
	if dryRun {
		project := invoiceninja.Project{Entity: invoiceninja.Entity{ID: "preview-project:" + jobNumber}, ClientID: clientID, Name: "Job " + jobNumber, Number: jobNumber}
		state.projects[key(jobNumber)] = project
		refs.projectID = project.ID
		return refs, nil
	}
	created, err := s.client.Projects.Create(ctx, invoiceninja.CreateProjectRequest{
		ClientID: clientID,
		Name:     "Job " + jobNumber,
		Number:   jobNumber,
		TaskRate: 0,
	})
	if err != nil {
		return refs, fmt.Errorf("row %d create project %q: %w", row.rowNo, jobNumber, err)
	}
	state.projects[key(jobNumber)] = *created
	refs.projectID = created.ID
	return refs, nil
}

func expenseCreateRequest(row *preparedExpenseImportRow, idx map[string]int, refs expenseImportReferences, paymentDate, paymentTypeID string) invoiceninja.CreateExpenseRequest {
	basTreatment := deriveBASTreatment(cell(row.rec, idx, "Tax Treatment"), row.businessPct, row.businessGST)
	invoiceDocuments := false
	request := invoiceninja.CreateExpenseRequest{
		VendorID:             refs.vendor.ID,
		ProjectID:            refs.projectID,
		CategoryID:           refs.category.ID,
		Amount:               row.businessAmount,
		Date:                 row.date,
		PaymentDate:          paymentDate,
		PaymentTypeID:        paymentTypeID,
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
		request.TaxName1 = "GST"
		request.TaxRate1 = 10
	}
	return request
}

func expenseUpdateRequest(create invoiceninja.CreateExpenseRequest) invoiceninja.UpdateExpenseRequest {
	request := invoiceninja.UpdateExpenseRequest{
		VendorID:             create.VendorID,
		ProjectID:            create.ProjectID,
		CategoryID:           create.CategoryID,
		Amount:               create.Amount,
		Date:                 create.Date,
		PaymentDate:          create.PaymentDate,
		PaymentTypeID:        create.PaymentTypeID,
		PrivateNotes:         create.PrivateNotes,
		TransactionReference: create.TransactionReference,
		TaxName1:             create.TaxName1,
		TaxRate1:             create.TaxRate1,
		TaxAmount1:           create.TaxAmount1,
		UsesInclusiveTaxes:   create.UsesInclusiveTaxes,
		CalculateTaxByAmount: create.CalculateTaxByAmount,
		InvoiceDocuments:     create.InvoiceDocuments,
		CustomValue1:         create.CustomValue1,
		CustomValue2:         create.CustomValue2,
		CustomValue3:         create.CustomValue3,
		CustomValue4:         create.CustomValue4,
	}
	return request.WithExplicitFields(
		"vendor_id",
		"project_id",
		"category_id",
		"amount",
		"date",
		"payment_date",
		"payment_type_id",
		"private_notes",
		"transaction_reference",
		"tax_name1",
		"tax_rate1",
		"tax_amount1",
		"uses_inclusive_taxes",
		"calculate_tax_by_amount",
		"custom_value1",
		"custom_value2",
		"custom_value3",
		"custom_value4",
	)
}

func expenseCorrectionChanges(existing invoiceninja.Expense, desired invoiceninja.UpdateExpenseRequest) []string {
	var changes []string
	add := func(condition bool, description string) {
		if condition {
			changes = append(changes, description)
		}
	}
	add(existing.VendorID != desired.VendorID, "vendor:update")
	add(existing.ProjectID != desired.ProjectID, "project:update")
	add(existing.CategoryID != desired.CategoryID, "category:update")
	add(moneyCents(existing.Amount) != moneyCents(desired.Amount), "amount:update")
	add(existing.Date != desired.Date, "date:update")
	add(existing.PaymentDate != desired.PaymentDate || existing.PaymentTypeID != desired.PaymentTypeID, "payment:update")
	add(existing.PrivateNotes != desired.PrivateNotes, "private-notes:update")
	add(existing.TransactionReference != desired.TransactionReference, "reference:update")
	add(existing.TaxName1 != desired.TaxName1 || existing.TaxRate1 != desired.TaxRate1 || moneyCents(existing.TaxAmount1) != moneyCents(desired.TaxAmount1), "tax:update")
	add(existing.UsesInclusiveTaxes != desired.UsesInclusiveTaxes || existing.CalculateTaxByAmount != desired.CalculateTaxByAmount, "tax-mode:update")
	add(existing.CustomValue1 != desired.CustomValue1 || existing.CustomValue2 != desired.CustomValue2 || existing.CustomValue3 != desired.CustomValue3 || existing.CustomValue4 != desired.CustomValue4, "classification:update")
	return changes
}

func mergeExpensePrivateNotes(existing, desired string) string {
	managedLabels := []string{
		"Store: ", "Source file: ", "Job number: ", "Child job number: ",
		"Capital check: ", "Item number: ", "Item description: ", "Qty: ",
		"Unit: ", "Unit price: ", "Source total ex GST: ", "Source GST: ",
		"Source total inc GST: ", "Source currency: ", "Business amount: ",
		"Business GST: ", "Notes: ", "GoTradie Import ID: ",
		"GoTradie source row: ", "GoTradie supplier account: ",
	}
	managedMarkers := []string{
		"[GoTradie source:", importIDMarkerPrefix, supplierAccountMarkerPrefix,
		settlementPurchaseMarkerPrefix, receiptMarkerPrefix, receiptOwnerMarker,
	}
	var preserved []string
	for _, line := range strings.Split(existing, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		managed := false
		for _, prefix := range managedLabels {
			if strings.HasPrefix(trimmed, prefix) {
				managed = true
				break
			}
		}
		if !managed {
			for _, prefix := range managedMarkers {
				if strings.HasPrefix(trimmed, prefix) {
					managed = true
					break
				}
			}
		}
		if !managed {
			preserved = append(preserved, line)
		}
	}
	preserved = append(preserved, strings.TrimSpace(desired))
	return strings.TrimSpace(strings.Join(preserved, "\n"))
}

func applyExpenseUpdateState(expense *invoiceninja.Expense, desired invoiceninja.UpdateExpenseRequest, documents []invoiceninja.Document) {
	expense.VendorID = desired.VendorID
	expense.ProjectID = desired.ProjectID
	expense.CategoryID = desired.CategoryID
	expense.Amount = desired.Amount
	expense.Date = desired.Date
	expense.PaymentDate = desired.PaymentDate
	expense.PaymentTypeID = desired.PaymentTypeID
	expense.PrivateNotes = desired.PrivateNotes
	expense.TransactionReference = desired.TransactionReference
	expense.TaxName1 = desired.TaxName1
	expense.TaxRate1 = desired.TaxRate1
	expense.TaxAmount1 = desired.TaxAmount1
	expense.UsesInclusiveTaxes = desired.UsesInclusiveTaxes
	expense.CalculateTaxByAmount = desired.CalculateTaxByAmount
	expense.CustomValue1 = desired.CustomValue1
	expense.CustomValue2 = desired.CustomValue2
	expense.CustomValue3 = desired.CustomValue3
	expense.CustomValue4 = desired.CustomValue4
	if len(expense.Documents) == 0 {
		expense.Documents = documents
	}
}

func receiptSourceWarnings(uses []expenseReceiptUse) []ExpenseImportIssue {
	byName := make(map[string][]int)
	byKey := make(map[string][]int)
	for i := range uses {
		byName[uses[i].name] = append(byName[uses[i].name], i)
		if uses[i].key != "" {
			byKey[uses[i].key] = append(byKey[uses[i].key], i)
		}
	}
	var warnings []ExpenseImportIssue
	for name, indexes := range byName {
		if len(indexes) < 2 {
			continue
		}
		first := uses[indexes[0]]
		for _, index := range indexes[1:] {
			use := uses[index]
			if use.supplier != first.supplier || use.date != first.date {
				warnings = append(warnings, ExpenseImportIssue{RowNo: use.rowNo, ImportID: use.importID, Message: fmt.Sprintf("receipt filename %q is also used at row %d for a different supplier or date", name, first.rowNo)})
			}
		}
	}
	for _, indexes := range byKey {
		if len(indexes) < 2 {
			continue
		}
		first := uses[indexes[0]]
		for _, index := range indexes[1:] {
			use := uses[index]
			if use.name != first.name {
				warnings = append(warnings, ExpenseImportIssue{RowNo: use.rowNo, ImportID: use.importID, Message: fmt.Sprintf("receipt content matches row %d but filenames differ (%q and %q)", first.rowNo, first.name, use.name)})
			}
		}
	}
	sort.SliceStable(warnings, func(i, j int) bool { return warnings[i].RowNo < warnings[j].RowNo })
	return warnings
}

func summarizeExpenseBatch(number int, results []CSVImportResult, last bool) ExpenseImportBatchSummary {
	summary := ExpenseImportBatchSummary{Number: number, Processed: len(results), Last: last}
	for _, result := range results {
		switch {
		case result.Error != nil || result.Action == "error":
			summary.Errors++
		case result.Action == "created" || result.Action == "created-transaction" || result.Action == "would-create" || result.Action == "would-create-transaction":
			summary.New++
		case result.Action == "updated" || result.Action == "would-update" || result.Action == "would-update-transaction":
			summary.Updated++
		case result.Action == "deferred":
			summary.Deferred++
		default:
			summary.Existing++
		}
	}
	return summary
}

func effectiveExpenseBatchSize(total, requested int, settlementEnabled bool) int {
	if total <= 0 {
		return 1
	}
	if settlementEnabled || requested <= 0 || requested > total {
		return total
	}
	return requested
}

func completeExpenseBatch(options ExpenseImportOptions, mode string, number int, results []CSVImportResult, last bool) error {
	if mode != ExpenseImportModeBatched || options.OnBatchComplete == nil {
		return nil
	}
	return options.OnBatchComplete(summarizeExpenseBatch(number, results, last))
}

func emitExpenseProgress(options ExpenseImportOptions, index, total int, result CSVImportResult) {
	if options.OnProgress != nil {
		options.OnProgress(ExpenseImportProgress{Index: index, Total: total, Result: result})
	}
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
