package ninja

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
)

var productCSVHeader = []string{"ID", "Product", "Description", "Price", "Default Quantity", "Max Quantity", "Image URL"}

type CSVImportResult struct {
	ID      string
	Name    string
	Action  string
	Changes []string
	Error   error
}

func (s *Service) ExportProductsCSV(ctx context.Context, w io.Writer) error {
	products, err := s.ListProducts(ctx)
	if err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(productCSVHeader); err != nil {
		return err
	}
	for _, p := range products {
		row := []string{p.ID, p.ProductKey, p.Notes, formatFloat(p.Price), formatFloat(p.Quantity), "", s.CustomValue(p, s.cfg.ImageURLCustom)}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func (s *Service) ImportProductsCSV(ctx context.Context, r io.Reader, dryRun bool) ([]CSVImportResult, error) {
	recs, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, fmt.Errorf("CSV is empty")
	}
	idx := headerIndex(recs[0])
	for _, name := range productCSVHeader[:5] {
		if _, ok := idx[name]; !ok {
			return nil, fmt.Errorf("missing required column %q", name)
		}
	}
	var results []CSVImportResult
	for rowNo, rec := range recs[1:] {
		if emptyRecord(rec) {
			continue
		}
		id := cell(rec, idx, "ID")
		name := cell(rec, idx, "Product")
		res := CSVImportResult{ID: id, Name: name}
		if id == "" {
			res.Action = "error"
			res.Error = fmt.Errorf("row %d: ID is required", rowNo+2)
			results = append(results, res)
			continue
		}
		existing, err := s.client.Products.Get(ctx, id)
		if err != nil {
			res.Action = "error"
			res.Error = err
			results = append(results, res)
			continue
		}
		price, err := parseFloat(cell(rec, idx, "Price"))
		if err != nil {
			res.Action = "error"
			res.Error = fmt.Errorf("row %d price: %w", rowNo+2, err)
			results = append(results, res)
			continue
		}
		qty, err := parseFloat(cell(rec, idx, "Default Quantity"))
		if err != nil {
			res.Action = "error"
			res.Error = fmt.Errorf("row %d default quantity: %w", rowNo+2, err)
			results = append(results, res)
			continue
		}
		payload := invoiceninja.UpdateProductRequest(invoiceninja.CreateProductRequest{
			ProductKey:   cell(rec, idx, "Product"),
			Notes:        cell(rec, idx, "Description"),
			Cost:         existing.Cost,
			Price:        price,
			Quantity:     qty,
			TaxName1:     existing.TaxName1,
			TaxRate1:     existing.TaxRate1,
			TaxName2:     existing.TaxName2,
			TaxRate2:     existing.TaxRate2,
			CustomValue1: existing.CustomValue1,
			CustomValue2: existing.CustomValue2,
			CustomValue3: existing.CustomValue3,
			CustomValue4: existing.CustomValue4,
		})
		setProductCustom(&payload, s.cfg.ImageURLCustom, cell(rec, idx, "Image URL"))
		changes := diffProductCSV(*existing, payload, s.cfg.ImageURLCustom)
		res.Changes = changes
		if len(changes) == 0 {
			res.Action = "unchanged"
			results = append(results, res)
			continue
		}
		if dryRun {
			res.Action = "would-update"
			results = append(results, res)
			continue
		}
		if _, err := s.client.Products.Update(ctx, id, payload); err != nil {
			res.Action = "error"
			res.Error = err
		} else {
			res.Action = "updated"
		}
		results = append(results, res)
	}
	return results, nil
}

func (s *Service) ListClients(ctx context.Context) ([]invoiceninja.ClientEntity, error) {
	return s.client.Clients.ListAll(ctx, invoiceninja.ClientQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Include: []string{"contacts"}, Status: "active"},
	})
}

func (s *Service) ListQuotes(ctx context.Context) ([]invoiceninja.Quote, error) {
	return s.client.Quotes.ListAll(ctx, invoiceninja.QuoteQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Include: []string{"client"}, Status: "active"},
	})
}

func (s *Service) ListInvoices(ctx context.Context) ([]invoiceninja.Invoice, error) {
	return s.client.Invoices.ListAll(ctx, invoiceninja.InvoiceQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Include: []string{"client"}, Status: "active"},
	})
}

func (s *Service) ListPayments(ctx context.Context) ([]invoiceninja.Payment, error) {
	return s.client.Payments.ListAll(ctx, invoiceninja.PaymentQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Include: []string{"client", "invoices"}, Status: "active"},
	})
}

func (s *Service) ExportClientsCSV(ctx context.Context, w io.Writer) error {
	clients, err := s.ListClients(ctx)
	if err != nil {
		return err
	}
	maxContacts := 0
	for _, c := range clients {
		if len(c.Contacts) > maxContacts {
			maxContacts = len(c.Contacts)
		}
	}
	header := clientHeader(maxContacts)
	cw := csv.NewWriter(w)
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, c := range clients {
		row := []string{c.ID, c.Name, formatAddress(c)}
		for _, ct := range c.Contacts {
			row = append(row, ct.FirstName, ct.LastName, ct.Email, ct.Phone)
		}
		for len(row) < len(header) {
			row = append(row, "")
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func (s *Service) ImportClientsCSV(ctx context.Context, r io.Reader, dryRun bool) ([]CSVImportResult, error) {
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
	for _, name := range []string{"ID", "Name", "Address"} {
		if _, ok := idx[name]; !ok {
			return nil, fmt.Errorf("missing required column %q", name)
		}
	}
	contactCount := countContactColumns(recs[0])
	var results []CSVImportResult
	for rowNo, rec := range recs[1:] {
		if emptyRecord(rec) {
			continue
		}
		id := cell(rec, idx, "ID")
		name := cell(rec, idx, "Name")
		res := CSVImportResult{ID: id, Name: name}
		if id == "" {
			res.Action = "error"
			res.Error = fmt.Errorf("row %d: ID is required", rowNo+2)
			results = append(results, res)
			continue
		}
		existing, err := s.client.Clients.Get(ctx, id, "contacts")
		if err != nil {
			res.Action = "error"
			res.Error = err
			results = append(results, res)
			continue
		}
		payload := invoiceninja.UpdateClientRequest(invoiceninja.CreateClientRequest{
			Name:         name,
			Phone:        existing.Phone,
			Website:      existing.Website,
			Address1:     cell(rec, idx, "Address"),
			Address2:     existing.Address2,
			City:         existing.City,
			State:        existing.State,
			PostalCode:   existing.PostalCode,
			CountryID:    existing.CountryID,
			PrivateNotes: existing.PrivateNotes,
			PublicNotes:  existing.PublicNotes,
			IDNumber:     existing.IDNumber,
			VATNumber:    existing.VATNumber,
			CustomValue1: existing.CustomValue1,
			CustomValue2: existing.CustomValue2,
			CustomValue3: existing.CustomValue3,
			CustomValue4: existing.CustomValue4,
		})
		contacts := make([]invoiceninja.Contact, 0, contactCount)
		for i := 1; i <= contactCount; i++ {
			ct := invoiceninja.Contact{}
			if i-1 < len(existing.Contacts) {
				ct = existing.Contacts[i-1]
			}
			ct.FirstName = cell(rec, idx, fmt.Sprintf("Contact %d First Name", i))
			ct.LastName = cell(rec, idx, fmt.Sprintf("Contact %d Last Name", i))
			ct.Email = cell(rec, idx, fmt.Sprintf("Contact %d Email", i))
			ct.Phone = cell(rec, idx, fmt.Sprintf("Contact %d Phone", i))
			if ct.FirstName == "" && ct.LastName == "" && ct.Email == "" && ct.Phone == "" {
				continue
			}
			contacts = append(contacts, ct)
		}
		payload.Contacts = contacts
		changes := diffClientCSV(*existing, payload)
		res.Changes = changes
		if len(changes) == 0 {
			res.Action = "unchanged"
			results = append(results, res)
			continue
		}
		if dryRun {
			res.Action = "would-update"
			results = append(results, res)
			continue
		}
		if _, err := s.client.Clients.Update(ctx, id, payload); err != nil {
			res.Action = "error"
			res.Error = err
		} else {
			res.Action = "updated"
		}
		results = append(results, res)
	}
	return results, nil
}

var salesDocumentCSVHeader = []string{"ID", "Number", "Client ID", "Client Name", "Status", "Date", "Due Date", "Subtotal", "Discount", "Tax", "Total", "Balance", "Paid To Date", "Public Notes", "Private Notes"}
var quoteCSVHeader = []string{"ID", "Number", "Client ID", "Client Name", "Status", "Date", "Valid Until", "Subtotal", "Discount", "Tax", "Total", "Balance", "Public Notes", "Private Notes"}
var paymentCSVHeader = []string{"ID", "Client ID", "Client Name", "Invoice ID", "Invoice Number", "Date", "Amount", "Applied", "Refunded", "Transaction Reference", "Payment Type", "Status", "Private Notes"}

func (s *Service) ExportQuotesCSV(ctx context.Context, w io.Writer) error {
	quotes, err := s.ListQuotes(ctx)
	if err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(quoteCSVHeader); err != nil {
		return err
	}
	for _, q := range quotes {
		doc := invoiceninja.Invoice(q)
		row := []string{
			doc.ID,
			doc.Number,
			doc.ClientID,
			clientName(doc.Client),
			doc.StatusID,
			doc.Date,
			doc.DueDate,
			formatFloat(subtotal(doc)),
			formatFloat(doc.Discount),
			formatFloat(doc.TotalTaxes),
			formatFloat(doc.Amount),
			formatFloat(doc.Balance),
			doc.PublicNotes,
			doc.PrivateNotes,
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func (s *Service) ExportInvoicesCSV(ctx context.Context, w io.Writer) error {
	invoices, err := s.ListInvoices(ctx)
	if err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(salesDocumentCSVHeader); err != nil {
		return err
	}
	for _, inv := range invoices {
		row := []string{
			inv.ID,
			inv.Number,
			inv.ClientID,
			clientName(inv.Client),
			inv.StatusID,
			inv.Date,
			inv.DueDate,
			formatFloat(subtotal(inv)),
			formatFloat(inv.Discount),
			formatFloat(inv.TotalTaxes),
			formatFloat(inv.Amount),
			formatFloat(inv.Balance),
			formatFloat(inv.PaidToDate),
			inv.PublicNotes,
			inv.PrivateNotes,
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func (s *Service) ExportPaymentsCSV(ctx context.Context, w io.Writer) error {
	payments, err := s.ListPayments(ctx)
	if err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(paymentCSVHeader); err != nil {
		return err
	}
	for _, p := range payments {
		invoiceNumber := ""
		if len(p.Invoices) > 0 {
			invoiceNumber = p.Invoices[0].Number
		}
		row := []string{
			p.ID,
			p.ClientID,
			clientName(p.Client),
			p.InvoiceID,
			invoiceNumber,
			p.Date,
			formatFloat(p.Amount),
			formatFloat(p.Applied),
			formatFloat(p.Refunded),
			p.TransactionReference,
			p.PaymentTypeID,
			"",
			p.PrivateNotes,
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func clientName(c *invoiceninja.ClientEntity) string {
	if c == nil {
		return ""
	}
	if strings.TrimSpace(c.DisplayName) != "" {
		return c.DisplayName
	}
	return c.Name
}

func subtotal(doc invoiceninja.Invoice) float64 {
	return doc.Amount - doc.TotalTaxes
}

func clientHeader(n int) []string {
	header := []string{"ID", "Name", "Address"}
	for i := 1; i <= n; i++ {
		header = append(header, fmt.Sprintf("Contact %d First Name", i), fmt.Sprintf("Contact %d Last Name", i), fmt.Sprintf("Contact %d Email", i), fmt.Sprintf("Contact %d Phone", i))
	}
	return header
}

func countContactColumns(header []string) int {
	max := 0
	for _, h := range header {
		h = strings.TrimSpace(h)
		if strings.HasPrefix(h, "Contact ") && strings.HasSuffix(h, " Email") {
			parts := strings.Fields(h)
			if len(parts) >= 3 {
				if n, err := strconv.Atoi(parts[1]); err == nil && n > max {
					max = n
				}
			}
		}
	}
	return max
}

func formatAddress(c invoiceninja.ClientEntity) string {
	parts := []string{c.Address1, c.Address2, c.City, c.State, c.PostalCode}
	var kept []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, ", ")
}

func headerIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[strings.TrimSpace(h)] = i
	}
	return idx
}

func cell(rec []string, idx map[string]int, name string) string {
	i, ok := idx[name]
	if !ok || i >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[i])
}

func emptyRecord(rec []string) bool {
	for _, c := range rec {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(strings.TrimPrefix(s, "$"))
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func floatsEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.000001
}

func setProductCustom(p *invoiceninja.UpdateProductRequest, idx int, val string) {
	switch idx {
	case 1:
		p.CustomValue1 = val
	case 2:
		p.CustomValue2 = val
	case 3:
		p.CustomValue3 = val
	case 4:
		p.CustomValue4 = val
	}
}

func diffProductCSV(existing invoiceninja.Product, next invoiceninja.UpdateProductRequest, imageCustom int) []string {
	n := invoiceninja.CreateProductRequest(next)
	var changes []string
	if strings.TrimSpace(existing.ProductKey) != strings.TrimSpace(n.ProductKey) {
		changes = append(changes, "product")
	}
	if strings.TrimSpace(existing.Notes) != strings.TrimSpace(n.Notes) {
		changes = append(changes, "description")
	}
	if !floatsEqual(existing.Price, n.Price) {
		changes = append(changes, "price")
	}
	if !floatsEqual(existing.Quantity, n.Quantity) {
		changes = append(changes, "default_quantity")
	}
	if customValueProduct(existing, imageCustom) != customValueCreate(n, imageCustom) {
		changes = append(changes, "image_url")
	}
	return changes
}

func customValueProduct(p invoiceninja.Product, idx int) string {
	switch idx {
	case 1:
		return p.CustomValue1
	case 2:
		return p.CustomValue2
	case 3:
		return p.CustomValue3
	case 4:
		return p.CustomValue4
	default:
		return ""
	}
}

func customValueCreate(p invoiceninja.CreateProductRequest, idx int) string {
	switch idx {
	case 1:
		return p.CustomValue1
	case 2:
		return p.CustomValue2
	case 3:
		return p.CustomValue3
	case 4:
		return p.CustomValue4
	default:
		return ""
	}
}

func diffClientCSV(existing invoiceninja.ClientEntity, next invoiceninja.UpdateClientRequest) []string {
	n := invoiceninja.CreateClientRequest(next)
	var changes []string
	if strings.TrimSpace(existing.Name) != strings.TrimSpace(n.Name) {
		changes = append(changes, "name")
	}
	if strings.TrimSpace(formatAddress(existing)) != strings.TrimSpace(n.Address1) {
		changes = append(changes, "address")
	}
	if len(existing.Contacts) != len(n.Contacts) {
		changes = append(changes, "contacts")
	} else {
		for i := range n.Contacts {
			if existing.Contacts[i].FirstName != n.Contacts[i].FirstName || existing.Contacts[i].LastName != n.Contacts[i].LastName || existing.Contacts[i].Email != n.Contacts[i].Email || existing.Contacts[i].Phone != n.Contacts[i].Phone {
				changes = append(changes, "contacts")
				break
			}
		}
	}
	return changes
}
