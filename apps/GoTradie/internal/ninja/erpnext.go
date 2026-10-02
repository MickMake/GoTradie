package ninja

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/config"
)

const allEntityStatuses = "active,archived,deleted"

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

type ERPNextFile struct {
	Name string
	Data []byte
}

type ERPNextExport struct {
	Files []ERPNextFile
}

type migrationIssue struct {
	Severity   string
	EntityType string
	SourceID   string
	SourceNo   string
	Issue      string
	Action     string
}

func (s *Service) BuildERPNextExport(ctx context.Context) (ERPNextExport, error) {
	if err := s.cfg.ValidateERPNextExport(); err != nil {
		return ERPNextExport{}, err
	}

	clients, err := s.client.Clients.ListAll(ctx, invoiceninja.ClientQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Include: []string{"contacts"}, Status: allEntityStatuses},
		WithTrashed: true,
	})
	if err != nil {
		return ERPNextExport{}, fmt.Errorf("list clients: %w", err)
	}
	products, err := s.client.Products.ListAll(ctx, invoiceninja.ProductQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Status: allEntityStatuses},
		WithTrashed: true,
	})
	if err != nil {
		return ERPNextExport{}, fmt.Errorf("list products: %w", err)
	}
	quotes, err := s.client.Quotes.ListAll(ctx, invoiceninja.QuoteQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Include: []string{"client"}, Status: allEntityStatuses},
		WithTrashed: true,
	})
	if err != nil {
		return ERPNextExport{}, fmt.Errorf("list quotes: %w", err)
	}
	invoices, err := s.client.Invoices.ListAll(ctx, invoiceninja.InvoiceQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Include: []string{"client"}, Status: allEntityStatuses},
		WithTrashed: true,
	})
	if err != nil {
		return ERPNextExport{}, fmt.Errorf("list invoices: %w", err)
	}
	payments, err := s.client.Payments.ListAll(ctx, invoiceninja.PaymentQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Include: []string{"client", "invoices"}, Status: allEntityStatuses},
		WithTrashed: true,
	})
	if err != nil {
		return ERPNextExport{}, fmt.Errorf("list payments: %w", err)
	}

	return buildERPNextExport(s.cfg, clients, products, quotes, invoices, payments)
}

func buildERPNextExport(cfg config.Config, clients []invoiceninja.ClientEntity, products []invoiceninja.Product, quotes []invoiceninja.Quote, invoices []invoiceninja.Invoice, payments []invoiceninja.Payment) (ERPNextExport, error) {
	var issues []migrationIssue
	sort.SliceStable(clients, func(i, j int) bool { return clients[i].ID < clients[j].ID })
	sort.SliceStable(products, func(i, j int) bool { return itemCode(products[i]) < itemCode(products[j]) })
	sort.SliceStable(quotes, func(i, j int) bool { return quoteNumber(quotes[i]) < quoteNumber(quotes[j]) })
	sort.SliceStable(invoices, func(i, j int) bool { return invoices[i].Number < invoices[j].Number })
	sort.SliceStable(payments, func(i, j int) bool {
		if payments[i].Date == payments[j].Date {
			return payments[i].ID < payments[j].ID
		}
		return payments[i].Date < payments[j].Date
	})

	clientByID := make(map[string]invoiceninja.ClientEntity, len(clients))
	productCodes := make(map[string]bool, len(products))
	for _, client := range clients {
		clientByID[client.ID] = client
	}
	for _, product := range products {
		productCodes[itemCode(product)] = true
	}

	customers, err := customerCSV(cfg, clients, &issues)
	if err != nil {
		return ERPNextExport{}, err
	}
	addresses, err := addressCSV(cfg, clients, &issues)
	if err != nil {
		return ERPNextExport{}, err
	}
	contacts, err := contactCSV(clients, &issues)
	if err != nil {
		return ERPNextExport{}, err
	}
	items, err := itemCSV(cfg, products, quotes, invoices, &issues)
	if err != nil {
		return ERPNextExport{}, err
	}
	quotations, err := quotationCSV(cfg, quotes, productCodes, &issues)
	if err != nil {
		return ERPNextExport{}, err
	}
	salesInvoices, err := salesInvoiceCSV(cfg, invoices, productCodes, &issues)
	if err != nil {
		return ERPNextExport{}, err
	}
	paymentEntries, err := paymentEntryCSV(cfg, payments, clientByID, &issues)
	if err != nil {
		return ERPNextExport{}, err
	}
	report, err := migrationReportCSV(issues)
	if err != nil {
		return ERPNextExport{}, err
	}

	return ERPNextExport{Files: []ERPNextFile{
		{Name: "Customer.csv", Data: customers},
		{Name: "Address.csv", Data: addresses},
		{Name: "Contact.csv", Data: contacts},
		{Name: "Item.csv", Data: items},
		{Name: "Quotation.csv", Data: quotations},
		{Name: "Sales Invoice.csv", Data: salesInvoices},
		{Name: "Payment Entry.csv", Data: paymentEntries},
		{Name: "Migration Report.csv", Data: report},
	}}, nil
}

func customerCSV(cfg config.Config, clients []invoiceninja.ClientEntity, issues *[]migrationIssue) ([]byte, error) {
	header := []string{"ID", "Customer Name", "Customer Type", "Customer Group", "Territory", "Billing Currency", "Website", "Tax ID", "Customer Details", "Mobile No", "Disabled"}
	rows := make([][]string, 0, len(clients))
	for _, client := range clients {
		name := strings.TrimSpace(clientName(&client))
		if name == "" && len(client.Contacts) > 0 {
			name = strings.TrimSpace(strings.Join([]string{client.Contacts[0].FirstName, client.Contacts[0].LastName}, " "))
		}
		if name == "" {
			name = "Invoice Ninja Customer " + client.ID
			addIssue(issues, "WARNING", "Client", client.ID, client.Number, "Customer name was blank", "Generated a readable fallback name")
		}
		taxID := firstNonBlank(client.VATNumber, client.IDNumber)
		rows = append(rows, []string{
			customerID(client.ID), name, "Company", cfg.ERPNextCustomerGroup, cfg.ERPNextTerritory,
			cfg.ERPNextCurrency, strings.TrimSpace(client.Website), strings.TrimSpace(taxID), cleanText(client.PrivateNotes),
			strings.TrimSpace(client.Phone), boolCell(client.IsDeleted || client.ArchivedAt > 0),
		})
	}
	return encodeCSV(header, rows)
}

func addressCSV(cfg config.Config, clients []invoiceninja.ClientEntity, issues *[]migrationIssue) ([]byte, error) {
	header := []string{"ID", "Address Title", "Address Type", "Address Line 1", "Address Line 2", "City/Town", "State/Province", "Country", "Postal Code", "Phone", "Preferred Billing Address", "Preferred Shipping Address", "Links:Link DocType", "Links:Link Name"}
	var rows [][]string
	for _, client := range clients {
		name := firstNonBlank(strings.TrimSpace(clientName(&client)), "Invoice Ninja Customer "+client.ID)
		if strings.TrimSpace(client.Address1) != "" {
			if strings.TrimSpace(client.City) == "" {
				addIssue(issues, "ERROR", "Client", client.ID, client.Number, "Billing address has no city", "Address row excluded")
			} else {
				rows = append(rows, []string{
					"IN-ADDR-B-" + client.ID, name + " - Billing", "Billing", strings.TrimSpace(client.Address1), strings.TrimSpace(client.Address2),
					strings.TrimSpace(client.City), strings.TrimSpace(client.State), countryName(client.CountryID, cfg.ERPNextCountry), strings.TrimSpace(client.PostalCode), strings.TrimSpace(client.Phone),
					"1", "0", "Customer", customerID(client.ID),
				})
			}
		}
		if hasShippingAddress(client) && !shippingMatchesBilling(client) {
			if strings.TrimSpace(client.ShippingAddress1) == "" || strings.TrimSpace(client.ShippingCity) == "" {
				addIssue(issues, "ERROR", "Client", client.ID, client.Number, "Shipping address is missing address line 1 or city", "Address row excluded")
			} else {
				rows = append(rows, []string{
					"IN-ADDR-S-" + client.ID, name + " - Shipping", "Shipping", strings.TrimSpace(client.ShippingAddress1), strings.TrimSpace(client.ShippingAddress2),
					strings.TrimSpace(client.ShippingCity), strings.TrimSpace(client.ShippingState), countryName(client.ShippingCountryID, cfg.ERPNextCountry), strings.TrimSpace(client.ShippingPostalCode), strings.TrimSpace(client.Phone),
					"0", "1", "Customer", customerID(client.ID),
				})
			}
		}
	}
	return encodeCSV(header, rows)
}

func contactCSV(clients []invoiceninja.ClientEntity, issues *[]migrationIssue) ([]byte, error) {
	header := []string{"ID", "First Name", "Last Name", "Is Primary Contact", "Email IDs:Email ID", "Email IDs:Is Primary", "Contact Numbers:Phone", "Contact Numbers:Is Primary Phone", "Links:Link DocType", "Links:Link Name"}
	var rows [][]string
	for _, client := range clients {
		primaryWritten := false
		for _, contact := range client.Contacts {
			if contact.IsDeleted || contact.ArchivedAt > 0 {
				addIssue(issues, "INFO", "Contact", contact.ID, "", "Contact is archived or deleted", "Contact row excluded")
				continue
			}
			first := strings.TrimSpace(contact.FirstName)
			last := strings.TrimSpace(contact.LastName)
			emailAddress := normalizeEmail(contact.Email)
			if first == "" {
				first = firstNonBlank(emailLocalPart(emailAddress), "Contact")
			}
			primary := !primaryWritten
			if contact.IsPrimary && !primaryWritten {
				primary = true
			}
			if primary {
				primaryWritten = true
			}
			rows = append(rows, []string{
				"IN-CON-" + firstNonBlank(contact.ID, client.ID+"-"+strconv.Itoa(len(rows)+1)), first, last, boolCell(primary),
				emailAddress, boolCell(emailAddress != ""), strings.TrimSpace(contact.Phone), boolCell(strings.TrimSpace(contact.Phone) != ""),
				"Customer", customerID(client.ID),
			})
		}
	}
	return encodeCSV(header, rows)
}

func itemCSV(cfg config.Config, products []invoiceninja.Product, quotes []invoiceninja.Quote, invoices []invoiceninja.Invoice, issues *[]migrationIssue) ([]byte, error) {
	header := []string{"ID", "Item Code", "Item Name", "Item Group", "Default Unit of Measure", "Description", "Standard Selling Rate", "Maintain Stock", "Is Sales Item", "Is Purchase Item", "Disabled", "Item Defaults:Company", "Item Defaults:Income Account"}
	rows := make([][]string, 0, len(products)+1)
	seen := map[string]bool{}
	for _, product := range products {
		code := itemCode(product)
		if seen[code] {
			addIssue(issues, "ERROR", "Product", product.ID, code, "Duplicate ERPNext Item Code", "Later duplicate product row excluded")
			continue
		}
		seen[code] = true
		name := firstNonBlank(strings.TrimSpace(product.ProductKey), firstLine(cleanText(product.Notes)), code)
		rows = append(rows, []string{
			code, code, name, cfg.ERPNextItemGroup, cfg.ERPNextUOM, cleanText(product.Notes), formatFloat(product.Price),
			"0", "1", "0", boolCell(product.IsDeleted || product.ArchivedAt > 0), cfg.ERPNextCompany, cfg.ERPNextIncomeAccount,
		})
	}
	if needsMiscItem(quotes, invoices, seen) {
		rows = append(rows, []string{"IN-MISC", "IN-MISC", "Miscellaneous", cfg.ERPNextItemGroup, cfg.ERPNextUOM, "Migrated free-typed Invoice Ninja line", "0", "0", "1", "0", "0", cfg.ERPNextCompany, cfg.ERPNextIncomeAccount})
	}
	return encodeCSV(header, rows)
}

func quotationCSV(cfg config.Config, quotes []invoiceninja.Quote, productCodes map[string]bool, issues *[]migrationIssue) ([]byte, error) {
	header := append(documentHeader("Quotation"), itemChildHeader("Items")...)
	var rows [][]string
	for _, quote := range quotes {
		doc := invoiceninja.Invoice(quote)
		if doc.IsDeleted {
			addIssue(issues, "INFO", "Quote", doc.ID, doc.Number, "Quote is deleted", "Quotation excluded")
			continue
		}
		if strings.TrimSpace(doc.ClientID) == "" || len(doc.LineItems) == 0 {
			addIssue(issues, "ERROR", "Quote", doc.ID, doc.Number, "Quote has no customer or line items", "Quotation excluded")
			continue
		}
		if documentHasTax(doc) && strings.TrimSpace(cfg.ERPNextTaxTemplate) == "" {
			addIssue(issues, "ERROR", "Quote", doc.ID, doc.Number, "Taxed quote has no ERPNEXT_TAX_TEMPLATE", "Tax template left blank; totals require review")
		}
		parent := documentRow(cfg, doc, "IN-Q-", "Quote "+doc.Number, "Quotation")
		rows = appendDocumentRows(rows, parent, doc.LineItems, cfg, productCodes, issues, "Quote", doc.ID, doc.Number)
	}
	return encodeCSV(header, rows)
}

func salesInvoiceCSV(cfg config.Config, invoices []invoiceninja.Invoice, productCodes map[string]bool, issues *[]migrationIssue) ([]byte, error) {
	header := append(documentHeader("Sales Invoice"), itemChildHeader("Items")...)
	var rows [][]string
	for _, doc := range invoices {
		if doc.IsDeleted || doc.StatusID == "5" || doc.StatusID == "6" {
			addIssue(issues, "WARNING", "Invoice", doc.ID, doc.Number, "Invoice is deleted, cancelled, or reversed", "Sales Invoice excluded for manual review")
			continue
		}
		if strings.TrimSpace(doc.ClientID) == "" || len(doc.LineItems) == 0 {
			addIssue(issues, "ERROR", "Invoice", doc.ID, doc.Number, "Invoice has no customer or line items", "Sales Invoice excluded")
			continue
		}
		if documentHasTax(doc) && strings.TrimSpace(cfg.ERPNextTaxTemplate) == "" {
			addIssue(issues, "ERROR", "Invoice", doc.ID, doc.Number, "Taxed invoice has no ERPNEXT_TAX_TEMPLATE", "Tax template left blank; totals require review")
		}
		parent := documentRow(cfg, doc, "IN-SINV-", "Invoice "+doc.Number, "Sales Invoice")
		rows = appendDocumentRows(rows, parent, doc.LineItems, cfg, productCodes, issues, "Invoice", doc.ID, doc.Number)
	}
	return encodeCSV(header, rows)
}

func paymentEntryCSV(cfg config.Config, payments []invoiceninja.Payment, clients map[string]invoiceninja.ClientEntity, issues *[]migrationIssue) ([]byte, error) {
	header := []string{"ID", "Payment Type", "Posting Date", "Company", "Mode of Payment", "Party Type", "Party", "Account Paid From", "Account Paid To", "Paid Amount", "Received Amount", "Source Exchange Rate", "Target Exchange Rate", "Reference No", "Reference Date", "Remarks", "Submit After Import", "References:Type", "References:Name", "References:Allocated"}
	var rows [][]string
	for _, payment := range payments {
		if payment.IsDeleted {
			addIssue(issues, "WARNING", "Payment", payment.ID, payment.Number, "Payment is deleted", "Payment Entry excluded")
			continue
		}
		if payment.Refunded > 0 || len(payment.Credits) > 0 {
			addIssue(issues, "ERROR", "Payment", payment.ID, payment.Number, "Payment contains a refund or credit allocation", "Payment Entry excluded for explicit reversal or credit handling")
			continue
		}
		if strings.TrimSpace(payment.Date) == "" || strings.TrimSpace(payment.ClientID) == "" {
			addIssue(issues, "ERROR", "Payment", payment.ID, payment.Number, "Payment has no date or customer", "Payment Entry excluded")
			continue
		}
		if strings.TrimSpace(payment.ExchangeCurrencyID) != "" && payment.ExchangeRate != 0 && payment.ExchangeRate != 1 {
			addIssue(issues, "ERROR", "Payment", payment.ID, payment.Number, "Foreign-currency payment requires exchange-direction verification", "Payment Entry excluded")
			continue
		}
		if _, ok := clients[payment.ClientID]; !ok {
			addIssue(issues, "ERROR", "Payment", payment.ID, payment.Number, "Payment customer was not returned by Invoice Ninja", "Payment Entry excluded")
			continue
		}
		allocations := paymentAllocations(payment)
		if len(allocations) == 0 && len(payment.Invoices) > 1 {
			addIssue(issues, "ERROR", "Payment", payment.ID, payment.Number, "Multi-invoice payment has no allocation amounts", "Payment Entry excluded")
			continue
		}
		if len(allocations) == 0 && len(payment.Invoices) == 1 {
			allocations = []invoiceninja.Paymentable{{InvoiceID: payment.Invoices[0].ID, Amount: invoiceninja.FlexibleFloat(firstNonZero(payment.Applied, payment.Amount))}}
		}
		if len(allocations) == 0 && strings.TrimSpace(payment.InvoiceID) != "" {
			allocations = []invoiceninja.Paymentable{{InvoiceID: payment.InvoiceID, Amount: invoiceninja.FlexibleFloat(firstNonZero(payment.Applied, payment.Amount))}}
		}
		reference := firstNonBlank(strings.TrimSpace(payment.TransactionReference), strings.TrimSpace(payment.Number), "IN-PAY-"+payment.ID)
		remarks := cleanText(payment.PrivateNotes)
		if payment.IsManual {
			remarks = joinNotes(remarks, "Migrated manual payment")
		}
		if payment.CompanyGatewayID != "" {
			remarks = joinNotes(remarks, "Invoice Ninja gateway: "+payment.CompanyGatewayID)
		}
		parent := []string{
			"IN-PAY-" + payment.ID, "Receive", payment.Date, cfg.ERPNextCompany, cfg.ERPNextModeOfPayment, "Customer", customerID(payment.ClientID),
			cfg.ERPNextReceivableAccount, cfg.ERPNextBankAccount, formatFloat(payment.Amount), formatFloat(payment.Amount), "1", "1", reference, payment.Date, remarks, "1",
		}
		if len(allocations) == 0 {
			rows = append(rows, append(parent, "", "", ""))
			addIssue(issues, "WARNING", "Payment", payment.ID, payment.Number, "Payment has no invoice allocation", "Imported as an unallocated customer receipt")
			continue
		}
		for i, allocation := range allocations {
			invoiceID := strings.TrimSpace(allocation.InvoiceID)
			if invoiceID == "" || allocation.CreditID != "" {
				continue
			}
			row := blankRow(len(parent))
			if i == 0 {
				row = append([]string(nil), parent...)
			}
			row = append(row, "Sales Invoice", "IN-SINV-"+invoiceID, formatFloat(float64(allocation.Amount)))
			rows = append(rows, row)
		}
	}
	return encodeCSV(header, rows)
}

func migrationReportCSV(issues []migrationIssue) ([]byte, error) {
	header := []string{"Severity", "Entity Type", "Invoice Ninja ID", "Source Number", "Issue", "Exporter Action"}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Severity != issues[j].Severity {
			return issues[i].Severity < issues[j].Severity
		}
		if issues[i].EntityType != issues[j].EntityType {
			return issues[i].EntityType < issues[j].EntityType
		}
		return issues[i].SourceID < issues[j].SourceID
	})
	rows := make([][]string, 0, len(issues))
	for _, issue := range issues {
		rows = append(rows, []string{issue.Severity, issue.EntityType, issue.SourceID, issue.SourceNo, issue.Issue, issue.Action})
	}
	return encodeCSV(header, rows)
}

func documentHeader(kind string) []string {
	if kind == "Quotation" {
		return []string{"ID", "Title", "Quotation To", "Party", "Date", "Valid Till", "Company", "Order Type", "Currency", "Exchange Rate", "Price List", "Terms", "Additional Discount Percentage", "Additional Discount Amount", "Taxes and Charges", "Submit After Import"}
	}
	return []string{"ID", "Title", "Customer", "Company", "Posting Date", "Payment Due Date", "Customer's Purchase Order", "Currency", "Exchange Rate", "Price List", "Terms", "Remarks", "Additional Discount Percentage", "Additional Discount Amount", "Taxes and Charges", "Update Stock", "Customer Address", "Shipping Address Name", "Contact Person", "Submit After Import"}
}

func itemChildHeader(prefix string) []string {
	return []string{prefix + ":Item Code", prefix + ":Item Name", prefix + ":Description", prefix + ":Quantity", prefix + ":Rate", prefix + ":UOM", prefix + ":UOM Conversion Factor", prefix + ":Discount Percentage", prefix + ":Discount Amount", prefix + ":Item Tax Template"}
}

func documentRow(cfg config.Config, doc invoiceninja.Invoice, prefix, title, kind string) []string {
	exchangeRate := doc.ExchangeRate
	if exchangeRate == 0 {
		exchangeRate = 1
	}
	discountPercentage, discountAmount := "", ""
	if doc.Discount != 0 {
		if doc.IsAmountDiscount {
			discountAmount = formatFloat(doc.Discount)
		} else {
			discountPercentage = formatFloat(doc.Discount)
		}
	}
	taxTemplate := ""
	if documentHasTax(doc) {
		taxTemplate = cfg.ERPNextTaxTemplate
	}
	terms := joinNotes(cleanText(doc.Terms), cleanText(doc.PublicNotes))
	submit := "0"
	if doc.StatusID == "2" || doc.StatusID == "3" || doc.StatusID == "4" {
		submit = "1"
	}
	if kind == "Quotation" {
		return []string{prefix + doc.ID, title, "Customer", customerID(doc.ClientID), doc.Date, doc.DueDate, cfg.ERPNextCompany, "Sales", cfg.ERPNextCurrency, formatFloat(exchangeRate), cfg.ERPNextSellingPriceList, terms, discountPercentage, discountAmount, taxTemplate, submit}
	}
	billingAddress, shippingAddress := "", ""
	if doc.Client != nil {
		if strings.TrimSpace(doc.Client.Address1) != "" && strings.TrimSpace(doc.Client.City) != "" {
			billingAddress = "IN-ADDR-B-" + doc.ClientID
		}
		if hasShippingAddress(*doc.Client) && !shippingMatchesBilling(*doc.Client) && strings.TrimSpace(doc.Client.ShippingAddress1) != "" && strings.TrimSpace(doc.Client.ShippingCity) != "" {
			shippingAddress = "IN-ADDR-S-" + doc.ClientID
		}
	}
	contact := ""
	if doc.ClientContactID != "" {
		contact = "IN-CON-" + doc.ClientContactID
	}
	remarks := cleanText(doc.PrivateNotes)
	return []string{prefix + doc.ID, title, customerID(doc.ClientID), cfg.ERPNextCompany, doc.Date, doc.DueDate, strings.TrimSpace(doc.PO), cfg.ERPNextCurrency, formatFloat(exchangeRate), cfg.ERPNextSellingPriceList, terms, remarks, discountPercentage, discountAmount, taxTemplate, "0", billingAddress, shippingAddress, contact, submit}
}

func appendDocumentRows(rows [][]string, parent []string, lines []invoiceninja.LineItem, cfg config.Config, productCodes map[string]bool, issues *[]migrationIssue, entityType, sourceID, sourceNumber string) [][]string {
	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].SortID == lines[j].SortID {
			return i < j
		}
		return lines[i].SortID < lines[j].SortID
	})
	for i, line := range lines {
		code := strings.TrimSpace(line.ProductKey)
		if code == "" || !productCodes[code] {
			if code != "" && !productCodes[code] {
				addIssue(issues, "WARNING", entityType, sourceID, sourceNumber, "Line item references missing product "+code, "Mapped line to IN-MISC")
			}
			code = "IN-MISC"
		}
		name := firstNonBlank(strings.TrimSpace(line.ProductKey), firstLine(cleanText(line.Notes)), "Miscellaneous")
		quantity := line.Quantity
		if quantity == 0 {
			addIssue(issues, "WARNING", entityType, sourceID, sourceNumber, "Line item quantity is zero", "Preserved explicit zero quantity")
		}
		discountPercentage, discountAmount := "", ""
		if line.Discount != 0 {
			if line.IsAmountDiscount {
				discountAmount = formatFloat(line.Discount)
			} else {
				discountPercentage = formatFloat(line.Discount)
			}
		}
		row := blankRow(len(parent))
		if i == 0 {
			row = append([]string(nil), parent...)
		}
		row = append(row, code, name, cleanText(line.Notes), formatFloat(quantity), formatFloat(line.Cost), cfg.ERPNextUOM, "1", discountPercentage, discountAmount, "")
		rows = append(rows, row)
	}
	return rows
}

func paymentAllocations(payment invoiceninja.Payment) []invoiceninja.Paymentable {
	var rows []invoiceninja.Paymentable
	for _, paymentable := range payment.Paymentables {
		if paymentable.InvoiceID != "" && paymentable.CreditID == "" && float64(paymentable.Amount) != 0 {
			rows = append(rows, paymentable)
		}
	}
	if len(rows) == 0 {
		for _, allocation := range payment.InvoiceAllocations {
			if allocation.InvoiceID != "" && float64(allocation.Amount) != 0 {
				rows = append(rows, allocation)
			}
		}
	}
	return rows
}

func encodeCSV(header []string, rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(header); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if len(row) != len(header) {
			return nil, fmt.Errorf("CSV row has %d columns; expected %d", len(row), len(header))
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func addIssue(issues *[]migrationIssue, severity, entityType, sourceID, sourceNumber, issue, action string) {
	*issues = append(*issues, migrationIssue{Severity: severity, EntityType: entityType, SourceID: sourceID, SourceNo: sourceNumber, Issue: issue, Action: action})
}

func itemCode(product invoiceninja.Product) string {
	return firstNonBlank(strings.TrimSpace(product.ProductKey), "IN-PROD-"+product.ID)
}

func quoteNumber(quote invoiceninja.Quote) string {
	doc := invoiceninja.Invoice(quote)
	return doc.Number
}

func customerID(id string) string { return "IN-CUST-" + strings.TrimSpace(id) }

func needsMiscItem(quotes []invoiceninja.Quote, invoices []invoiceninja.Invoice, products map[string]bool) bool {
	for _, quote := range quotes {
		for _, line := range invoiceninja.Invoice(quote).LineItems {
			if strings.TrimSpace(line.ProductKey) == "" || !products[strings.TrimSpace(line.ProductKey)] {
				return true
			}
		}
	}
	for _, invoice := range invoices {
		for _, line := range invoice.LineItems {
			if strings.TrimSpace(line.ProductKey) == "" || !products[strings.TrimSpace(line.ProductKey)] {
				return true
			}
		}
	}
	return false
}

func documentHasTax(doc invoiceninja.Invoice) bool {
	if doc.TotalTaxes != 0 || doc.TaxRate1 != 0 || doc.TaxRate2 != 0 || doc.TaxRate3 != 0 {
		return true
	}
	for _, line := range doc.LineItems {
		if line.TaxRate1 != 0 || line.TaxRate2 != 0 || line.TaxRate3 != 0 || line.TaxAmount != 0 {
			return true
		}
	}
	return false
}

func countryName(id, fallback string) string {
	switch strings.TrimSpace(id) {
	case "36", "AU", "AUS", "Australia":
		return "Australia"
	case "554", "NZ", "NZL", "New Zealand":
		return "New Zealand"
	case "":
		return fallback
	default:
		return fallback
	}
}

func hasShippingAddress(client invoiceninja.ClientEntity) bool {
	return strings.TrimSpace(strings.Join([]string{client.ShippingAddress1, client.ShippingAddress2, client.ShippingCity, client.ShippingState, client.ShippingPostalCode, client.ShippingCountryID}, "")) != ""
}

func shippingMatchesBilling(client invoiceninja.ClientEntity) bool {
	billing := []string{client.Address1, client.Address2, client.City, client.State, client.PostalCode, client.CountryID}
	shipping := []string{client.ShippingAddress1, client.ShippingAddress2, client.ShippingCity, client.ShippingState, client.ShippingPostalCode, client.ShippingCountryID}
	for i := range billing {
		if !strings.EqualFold(strings.TrimSpace(billing[i]), strings.TrimSpace(shipping[i])) {
			return false
		}
	}
	return true
}

func normalizeEmail(value string) string {
	value = strings.TrimSpace(value)
	local, domain, ok := strings.Cut(value, "@")
	if !ok {
		return value
	}
	return local + "@" + strings.ToLower(domain)
}

func emailLocalPart(value string) string {
	local, _, _ := strings.Cut(value, "@")
	return strings.TrimSpace(local)
}

func cleanText(value string) string {
	value = html.UnescapeString(value)
	value = htmlTagPattern.ReplaceAllString(value, " ")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	for i := range lines {
		lines[i] = strings.Join(strings.Fields(lines[i]), " ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func firstLine(value string) string {
	line, _, _ := strings.Cut(value, "\n")
	return strings.TrimSpace(line)
}

func joinNotes(parts ...string) string {
	var kept []string
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			kept = append(kept, strings.TrimSpace(part))
		}
	}
	return strings.Join(kept, "\n\n")
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonZero(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func boolCell(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func blankRow(length int) []string { return make([]string, length) }
