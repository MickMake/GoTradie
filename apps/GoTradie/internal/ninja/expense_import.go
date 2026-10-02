package ninja

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
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
	"Tax Treatment",
	"Category",
	"Option",
	"Total Inc GST",
	"Business %",
	"Business Amount",
	"Business GST",
}

type expenseImportState struct {
	vendors         map[string]invoiceninja.Vendor
	categories      map[string]invoiceninja.ExpenseCategory
	projects        map[string]invoiceninja.Project
	clientByQuote   map[string]string
	expenseByMarker map[string]invoiceninja.Expense
	seenMarkers     map[string]int
}

type receiptIndex map[string][]string

// ImportExpensesCSV imports the purchase ledger shape used by Mick's master
// spreadsheet. Preview is the default; pass dryRun=false only for --commit.
//
// The Invoice Ninja expense amount and tax amount use the spreadsheet's
// Business Amount / Business GST values. The original supplier totals, currency,
// item detail and source filename remain in private notes for tax/audit tracing.
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

	state, err := s.loadExpenseImportState(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]CSVImportResult, 0, len(recs)-1)
	for rowNo, rec := range recs[1:] {
		if emptyRecord(rec) {
			continue
		}
		results = append(results, s.importExpenseRow(ctx, state, receipts, idx, rec, rowNo+2, dryRun))
	}
	return results, nil
}

func (s *Service) loadExpenseImportState(ctx context.Context) (*expenseImportState, error) {
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
		WithTrashed: true,
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

	state := &expenseImportState{
		vendors:         make(map[string]invoiceninja.Vendor, len(vendors)),
		categories:      make(map[string]invoiceninja.ExpenseCategory, len(categories)),
		projects:        make(map[string]invoiceninja.Project, len(projects)),
		clientByQuote:   make(map[string]string, len(quotes)),
		expenseByMarker: make(map[string]invoiceninja.Expense, len(expenses)),
		seenMarkers:     make(map[string]int),
	}
	for _, vendor := range vendors {
		state.vendors[key(vendor.Name)] = vendor
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
	for _, expense := range expenses {
		if marker := sourceMarkerFromNotes(expense.PrivateNotes); marker != "" {
			state.expenseByMarker[marker] = expense
		}
	}
	return state, nil
}

func (s *Service) importExpenseRow(ctx context.Context, state *expenseImportState, receipts receiptIndex, idx map[string]int, rec []string, rowNo int, dryRun bool) CSVImportResult {
	supplier := cell(rec, idx, "Supplier")
	store := cell(rec, idx, "Store")
	vendorName := expenseVendorName(supplier, store)
	description := cell(rec, idx, "Item Description")
	name := vendorName
	if description != "" {
		name += " / " + description
	}
	res := CSVImportResult{Name: name}

	if supplier == "" {
		res.Action = "error"
		res.Error = fmt.Errorf("row %d: Supplier is required", rowNo)
		return res
	}
	categoryName := cell(rec, idx, "Category")
	if categoryName == "" {
		res.Action = "error"
		res.Error = fmt.Errorf("row %d: Category is required", rowNo)
		return res
	}
	date, err := parseExpenseDate(cell(rec, idx, "Date"))
	if err != nil {
		res.Action = "error"
		res.Error = fmt.Errorf("row %d date: %w", rowNo, err)
		return res
	}
	businessPct, err := parsePercent(cell(rec, idx, "Business %"))
	if err != nil {
		res.Action = "error"
		res.Error = fmt.Errorf("row %d business %%: %w", rowNo, err)
		return res
	}
	businessAmount, err := parseExpenseMoney(cell(rec, idx, "Business Amount"))
	if err != nil {
		res.Action = "error"
		res.Error = fmt.Errorf("row %d business amount: %w", rowNo, err)
		return res
	}
	businessGST, err := parseExpenseMoney(cell(rec, idx, "Business GST"))
	if err != nil {
		res.Action = "error"
		res.Error = fmt.Errorf("row %d business GST: %w", rowNo, err)
		return res
	}
	if businessAmount == 0 && businessPct > 0 {
		total, totalErr := parseExpenseMoney(cell(rec, idx, "Total Inc GST"))
		if totalErr != nil {
			res.Action = "error"
			res.Error = fmt.Errorf("row %d total inc GST: %w", rowNo, totalErr)
			return res
		}
		businessAmount = total * businessPct / 100
	}

	receiptName := strings.TrimSpace(cell(rec, idx, "File Name"))
	receiptPath, err := receipts.match(receiptName)
	if err != nil {
		res.Action = "error"
		res.Error = fmt.Errorf("row %d receipt: %w", rowNo, err)
		return res
	}

	marker := sourceMarker(rec)
	if existing, ok := state.expenseByMarker[marker]; ok {
		res.ID = existing.ID
		if receiptPath != "" && !hasDocument(existing.Documents, receiptName) {
			res.Changes = []string{"already-imported", "receipt:upload:" + receiptName}
			if dryRun {
				res.Action = "would-update"
				return res
			}
			updated, uploadErr := s.client.Expenses.UploadDocumentFile(ctx, existing.ID, receiptPath)
			if uploadErr != nil {
				res.Action = "error"
				res.Error = fmt.Errorf("row %d upload receipt %q: %w", rowNo, receiptName, uploadErr)
				return res
			}
			state.expenseByMarker[marker] = *updated
			res.Action = "updated"
			return res
		}
		res.Action = "unchanged"
		res.Changes = []string{"already-imported"}
		return res
	}
	if previousRow, ok := state.seenMarkers[marker]; ok {
		res.Action = "unchanged"
		res.Changes = []string{fmt.Sprintf("duplicate-source-row:%d", previousRow)}
		return res
	}
	state.seenMarkers[marker] = rowNo

	changes := make([]string, 0, 4)
	vendor, vendorExists := state.vendors[key(vendorName)]
	if !vendorExists {
		changes = append(changes, "vendor:create:"+vendorName)
		if !dryRun {
			created, createErr := s.client.Vendors.Create(ctx, invoiceninja.CreateVendorRequest{Name: vendorName})
			if createErr != nil {
				res.Action = "error"
				res.Error = fmt.Errorf("row %d create vendor %q: %w", rowNo, vendorName, createErr)
				return res
			}
			vendor = *created
			state.vendors[key(vendorName)] = vendor
		}
	}

	category, categoryExists := state.categories[key(categoryName)]
	if !categoryExists {
		changes = append(changes, "category:create:"+categoryName)
		if !dryRun {
			created, createErr := s.client.ExpenseCategories.Create(ctx, invoiceninja.CreateExpenseCategoryRequest{Name: categoryName})
			if createErr != nil {
				res.Action = "error"
				res.Error = fmt.Errorf("row %d create category %q: %w", rowNo, categoryName, createErr)
				return res
			}
			category = *created
			state.categories[key(categoryName)] = category
		}
	}

	projectID := ""
	jobNumber := projectNumber(rec, idx)
	if jobNumber != "" {
		if project, ok := state.projects[key(jobNumber)]; ok {
			projectID = project.ID
		} else if clientID := state.clientByQuote[key(jobNumber)]; clientID != "" {
			changes = append(changes, "project:create:"+jobNumber)
			if !dryRun {
				created, createErr := s.client.Projects.Create(ctx, invoiceninja.CreateProjectRequest{
					ClientID: clientID,
					Name:     "Job " + jobNumber,
					Number:   jobNumber,
					TaskRate: 0,
				})
				if createErr != nil {
					res.Action = "error"
					res.Error = fmt.Errorf("row %d create project %q: %w", rowNo, jobNumber, createErr)
					return res
				}
				state.projects[key(jobNumber)] = *created
				projectID = created.ID
			}
		} else {
			changes = append(changes, "project:unresolved:"+jobNumber)
		}
	}

	basTreatment := deriveBASTreatment(cell(rec, idx, "Tax Treatment"), businessPct, businessGST)
	payload := invoiceninja.CreateExpenseRequest{
		VendorID:             vendor.ID,
		ProjectID:            projectID,
		CategoryID:           category.ID,
		Amount:               businessAmount,
		Date:                 date,
		PaymentDate:          date,
		PrivateNotes:         expensePrivateNotes(rec, idx, rowNo, marker),
		TransactionReference: cell(rec, idx, "Invoice Number"),
		TaxAmount1:           businessGST,
		UsesInclusiveTaxes:   true,
		CalculateTaxByAmount: true,
		CustomValue1:         cell(rec, idx, "Tax Treatment"),
		CustomValue2:         cell(rec, idx, "Option"),
		CustomValue3:         cell(rec, idx, "Business %"),
		CustomValue4:         basTreatment,
	}
	if businessGST != 0 {
		payload.TaxName1 = "GST"
		payload.TaxRate1 = 10
	}

	changes = append(changes, "expense:create")
	if receiptPath != "" {
		changes = append(changes, "receipt:upload:"+receiptName)
	}
	res.Changes = changes
	if dryRun {
		res.Action = "would-create"
		return res
	}

	created, err := s.client.Expenses.Create(ctx, payload)
	if err != nil {
		res.Action = "error"
		res.Error = fmt.Errorf("row %d create expense: %w", rowNo, err)
		return res
	}
	res.ID = created.ID
	state.expenseByMarker[marker] = *created
	if receiptPath != "" {
		updated, uploadErr := s.client.Expenses.UploadDocumentFile(ctx, created.ID, receiptPath)
		if uploadErr != nil {
			res.Action = "error"
			res.Error = fmt.Errorf("row %d upload receipt %q: %w", rowNo, receiptName, uploadErr)
			return res
		}
		state.expenseByMarker[marker] = *updated
	}
	res.Action = "created"
	return res
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

func sourceMarker(rec []string) string {
	// Hash the full source row so rerunning the same export is safe. Delimiters are
	// included so neighbouring cells cannot accidentally collapse into one value.
	h := sha256.New()
	for _, value := range rec {
		_, _ = h.Write([]byte(strings.TrimSpace(value)))
		_, _ = h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	return "[GoTradie source:" + hex.EncodeToString(sum[:12]) + "]"
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

func key(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
