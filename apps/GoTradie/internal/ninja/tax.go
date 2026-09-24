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

	paymentDates := map[string][]string{}
	for _, p := range payments {
		date := strings.TrimSpace(p.Date)
		if date == "" {
			continue
		}
		ids := paymentInvoiceIDs(p)
		for _, id := range ids {
			paymentDates[id] = append(paymentDates[id], date)
		}
	}
	for id := range paymentDates {
		sort.Strings(paymentDates[id])
		paymentDates[id] = uniqueStrings(paymentDates[id])
	}

	var invoicesBuf bytes.Buffer
	iw := csv.NewWriter(&invoicesBuf)
	if err := iw.Write([]string{
		"Invoice", "Customer ID", "Date", "Due Date", "Paid On", "Total",
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
			strings.Join(paymentDates[inv.ID], " | "),
			formatFloat(inv.Amount),
		}); err != nil {
			return TaxExport{}, err
		}

		for i, line := range inv.LineItems {
			if err := dw.Write([]string{
				line.TypeID,
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

	var customersBuf bytes.Buffer
	cw := csv.NewWriter(&customersBuf)
	if err := cw.Write([]string{"Customer ID", "Name", "Address", "Email"}); err != nil {
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
			clientName(client),
			clientAddress(client),
			clientEmail(client),
		}); err != nil {
			return TaxExport{}, err
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return TaxExport{}, err
	}

	return TaxExport{Invoices: invoicesBuf.Bytes(), Detail: detailBuf.Bytes(), Customers: customersBuf.Bytes()}, nil
}

func paymentInvoiceIDs(p invoiceninja.Payment) []string {
	seen := map[string]bool{}
	var ids []string
	if strings.TrimSpace(p.InvoiceID) != "" {
		seen[p.InvoiceID] = true
		ids = append(ids, p.InvoiceID)
	}
	for _, inv := range p.Invoices {
		if strings.TrimSpace(inv.ID) == "" || seen[inv.ID] {
			continue
		}
		seen[inv.ID] = true
		ids = append(ids, inv.ID)
	}
	return ids
}

func uniqueStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if len(out) == 0 || out[len(out)-1] != v {
			out = append(out, v)
		}
	}
	return out
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
