package ninja

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"sort"
	"strings"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
)

// TaxExport is a deliberately narrow accounting export.
//
// It is kept separate from the general Invoice Ninja CSV exporters because Mick
// does not want to add two kitchens to an otherwise well-designed carport.
type TaxExport struct {
	Invoices  []byte
	Detail    []byte
	Customers []byte
	Payments  []byte
}

func (s *Service) BuildTaxExport(ctx context.Context) (TaxExport, error) {
	invoices, err := s.ListInvoices(ctx)
	if err != nil {
		return TaxExport{}, fmt.Errorf("list invoices: %w", err)
	}
	payments, err := s.ListPayments(ctx)
	if err != nil {
		return TaxExport{}, fmt.Errorf("list payments: %w", err)
	}

	var invoicesBuf bytes.Buffer
	iw := csv.NewWriter(&invoicesBuf)
	if err := iw.Write([]string{
		"Invoice", "Customer ID", "Date", "Due Date", "Payment Status", "Total", "Job Notes", "Job Description",
	}); err != nil {
		return TaxExport{}, err
	}

	var detailBuf bytes.Buffer
	dw := csv.NewWriter(&detailBuf)
	if err := dw.Write([]string{
		"Type", "Invoice", "Invoice Line #", "Item #", "Description", "Unit Rate", "Quantity", "Line Total",
	}); err != nil {
		return TaxExport{}, err
	}

	customers := map[string]*invoiceninja.ClientEntity{}

	for _, inv := range invoices {
		if inv.Client != nil && strings.TrimSpace(inv.ClientID) != "" {
			customers[inv.ClientID] = inv.Client
		}
		if err := iw.Write([]string{
			inv.Number,
			inv.ClientID,
			inv.Date,
			inv.DueDate,
			invoicePaymentStatus(inv),
			formatFloat(inv.Amount),
			inv.CustomValue4,
			inv.PublicNotes,
		}); err != nil {
			return TaxExport{}, err
		}

		for i, line := range inv.LineItems {
			if err := dw.Write([]string{
				classifyTaxLineType(line),
				inv.Number,
				fmt.Sprintf("%d", i+1),
				line.ProductKey,
				line.Notes,
				formatFloat(line.Cost),
				formatFloat(line.Quantity),
				formatFloat(line.LineTotal),
			}); err != nil {
				return TaxExport{}, err
			}
		}
	}

	iw.Flush()
	if err := iw.Error(); err != nil {
		return TaxExport{}, err
	}
	dw.Flush()
	if err := dw.Error(); err != nil {
		return TaxExport{}, err
	}

	invoiceNumberByID := make(map[string]string, len(invoices))
	for _, inv := range invoices {
		invoiceNumberByID[inv.ID] = inv.Number
	}

	var paymentsBuf bytes.Buffer
	pw := csv.NewWriter(&paymentsBuf)
	if err := pw.Write([]string{"Invoice", "Customer ID", "Paid On", "Amount"}); err != nil {
		return TaxExport{}, err
	}
	for _, p := range payments {
		invoiceNumber := invoiceNumberByID[p.InvoiceID]
		if invoiceNumber == "" && len(p.Invoices) > 0 {
			invoiceNumber = p.Invoices[0].Number
		}
		if err := pw.Write([]string{
			invoiceNumber,
			p.ClientID,
			p.Date,
			formatFloat(p.Amount),
		}); err != nil {
			return TaxExport{}, err
		}
	}
	pw.Flush()
	if err := pw.Error(); err != nil {
		return TaxExport{}, err
	}

	var customersBuf bytes.Buffer
	cw := csv.NewWriter(&customersBuf)
	if err := cw.Write([]string{"CustomerID", "Client Type", "Name", "Address", "Suburb", "State", "Postcode", "Country", "Email", "Phone", "Latitude", "Longitude"}); err != nil {
		return TaxExport{}, err
	}
	customerIDs := make([]string, 0, len(customers))
	for id := range customers {
		customerIDs = append(customerIDs, id)
	}
	sort.Strings(customerIDs)
	for _, id := range customerIDs {
		client := customers[id]
		if err := cw.Write([]string{
			id,
			strings.TrimSpace(client.CustomValue1),
			clientName(client),
			clientStreetAddress(client),
			client.City,
			client.State,
			client.PostalCode,
			clientCountry(client),
			clientEmail(client),
			clientPhone(client),
			clientLatitude(client),
			clientLongitude(client),
		}); err != nil {
			return TaxExport{}, err
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return TaxExport{}, err
	}

	return TaxExport{Invoices: invoicesBuf.Bytes(), Detail: detailBuf.Bytes(), Customers: customersBuf.Bytes(), Payments: paymentsBuf.Bytes()}, nil
}

func invoicePaymentStatus(inv invoiceninja.Invoice) string {
	if inv.PaidToDate <= 0 {
		return "None"
	}
	if inv.Balance <= 0.01 || inv.PaidToDate >= inv.Amount-0.01 {
		return "Full"
	}
	return "Partial"
}

func classifyTaxLineType(line invoiceninja.LineItem) string {
	// Invoice Ninja type_id 2 is a task, which maps cleanly to labour.
	if line.TypeID == "2" {
		return "Labour"
	}

	// Products are materials by default. Some fixed-price jobs were entered as
	// products simply because they were priced as a lump sum rather than hourly,
	// so only override when the description strongly looks like a scope of work.
	if looksLikeScopeOfWork(line.Notes) {
		return "Labour"
	}
	return "Materials"
}

func looksLikeScopeOfWork(description string) bool {
	text := strings.ToLower(strings.TrimSpace(description))
	if text == "" {
		return false
	}

	lines := strings.Split(text, "\n")
	bulletLines := 0
	actionLines := 0
	actionWords := []string{
		"remove", "install", "repair", "replace", "level", "jack", "cut", "fit",
		"fix", "paint", "sand", "route", "plane", "trim", "adjust", "seal", "glue",
		"drill", "hang", "build", "construct", "assemble", "demolish", "prepare",
		"measure", "set out", "pack", "lower", "raise", "refit", "reinstall",
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "*") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "•") {
			bulletLines++
			line = strings.TrimSpace(strings.TrimLeft(line, "*-•"))
		}
		for _, action := range actionWords {
			if strings.HasPrefix(line, action+" ") || strings.Contains(line, " "+action+" ") {
				actionLines++
				break
			}
		}
	}

	// A heading followed by several task bullets is the strongest signal and
	// matches the way Mick writes fixed-price labour scopes in Invoice Ninja.
	if bulletLines >= 2 && actionLines >= 2 {
		return true
	}

	// Also catch prose scopes with several distinct work actions, while keeping
	// single product descriptions classified as Material.
	return actionLines >= 3
}

func clientStreetAddress(c *invoiceninja.ClientEntity) string {
	if c == nil {
		return ""
	}
	parts := make([]string, 0, 2)
	if strings.TrimSpace(c.Address1) != "" {
		parts = append(parts, strings.TrimSpace(c.Address1))
	}
	if strings.TrimSpace(c.Address2) != "" {
		parts = append(parts, strings.TrimSpace(c.Address2))
	}
	return strings.Join(parts, ", ")
}

func clientCountry(c *invoiceninja.ClientEntity) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.CountryID)
}

func clientPhone(c *invoiceninja.ClientEntity) string {
	if c == nil {
		return ""
	}
	if strings.TrimSpace(c.Phone) != "" {
		return strings.TrimSpace(c.Phone)
	}
	for _, ct := range c.Contacts {
		if strings.TrimSpace(ct.Phone) != "" {
			return strings.TrimSpace(ct.Phone)
		}
	}
	return ""
}

func clientLatitude(c *invoiceninja.ClientEntity) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.CustomValue2)
}

func clientLongitude(c *invoiceninja.ClientEntity) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.CustomValue3)
}

func clientAddress(c *invoiceninja.ClientEntity) string {
	if c == nil {
		return ""
	}
	return formatAddress(*c)
}

func clientEmail(c *invoiceninja.ClientEntity) string {
	if c == nil || len(c.Contacts) == 0 {
		return ""
	}
	for _, ct := range c.Contacts {
		if strings.TrimSpace(ct.Email) != "" {
			return strings.TrimSpace(ct.Email)
		}
	}
	return ""
}
