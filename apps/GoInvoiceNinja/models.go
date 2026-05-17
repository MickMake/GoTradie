package goinvoiceninja

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type Meta struct {
	CurrentPage int    `json:"current_page,omitempty"`
	From        int    `json:"from,omitempty"`
	LastPage    int    `json:"last_page,omitempty"`
	Path        string `json:"path,omitempty"`
	PerPage     int    `json:"per_page,omitempty"`
	To          int    `json:"to,omitempty"`
	Total       int    `json:"total,omitempty"`
}

type APIError struct {
	StatusCode int                 `json:"-"`
	Message    string              `json:"message,omitempty"`
	Errors     map[string][]string `json:"errors,omitempty"`
	Raw        json.RawMessage     `json:"-"`
}

func (e *APIError) Error() string {
	if e == nil {
		return "invoice ninja API error"
	}
	parts := []string{"invoice ninja API error"}
	if e.StatusCode > 0 {
		parts = append(parts, fmt.Sprintf("status=%d", e.StatusCode))
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	if len(e.Errors) > 0 {
		parts = append(parts, "validation: "+formatValidationErrors(e.Errors))
	}
	return strings.Join(parts, ": ")
}

func formatValidationErrors(errors map[string][]string) string {
	keys := make([]string, 0, len(errors))
	for key := range errors {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		messages := strings.Join(errors[key], "; ")
		if messages == "" {
			parts = append(parts, key)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, messages))
	}
	return strings.Join(parts, "; ")
}

type Entity struct {
	ID             string `json:"id,omitempty"`
	UserID         string `json:"user_id,omitempty"`
	AssignedUserID string `json:"assigned_user_id,omitempty"`
	CreatedAt      int64  `json:"created_at,omitempty"`
	UpdatedAt      int64  `json:"updated_at,omitempty"`
	ArchivedAt     int64  `json:"archived_at,omitempty"`
	IsDeleted      bool   `json:"is_deleted,omitempty"`
	EntityType     string `json:"entity_type,omitempty"`
}

type Contact struct {
	ID           string `json:"id,omitempty"`
	FirstName    string `json:"first_name,omitempty"`
	LastName     string `json:"last_name,omitempty"`
	Email        string `json:"email,omitempty"`
	Phone        string `json:"phone,omitempty"`
	CustomValue1 string `json:"custom_value1,omitempty"`
	CustomValue2 string `json:"custom_value2,omitempty"`
	CustomValue3 string `json:"custom_value3,omitempty"`
	CustomValue4 string `json:"custom_value4,omitempty"`
}

type ClientEntity struct {
	Entity
	Name          string    `json:"name,omitempty"`
	DisplayName   string    `json:"display_name,omitempty"`
	Number        string    `json:"number,omitempty"`
	IDNumber      string    `json:"id_number,omitempty"`
	VATNumber     string    `json:"vat_number,omitempty"`
	Website       string    `json:"website,omitempty"`
	Phone         string    `json:"phone,omitempty"`
	Address1      string    `json:"address1,omitempty"`
	Address2      string    `json:"address2,omitempty"`
	City          string    `json:"city,omitempty"`
	State         string    `json:"state,omitempty"`
	PostalCode    string    `json:"postal_code,omitempty"`
	CountryID     string    `json:"country_id,omitempty"`
	PrivateNotes  string    `json:"private_notes,omitempty"`
	PublicNotes   string    `json:"public_notes,omitempty"`
	Contacts      []Contact `json:"contacts,omitempty"`
	Balance       float64   `json:"balance,omitempty"`
	PaidToDate    float64   `json:"paid_to_date,omitempty"`
	CreditBalance float64   `json:"credit_balance,omitempty"`
	CustomValue1  string    `json:"custom_value1,omitempty"`
	CustomValue2  string    `json:"custom_value2,omitempty"`
	CustomValue3  string    `json:"custom_value3,omitempty"`
	CustomValue4  string    `json:"custom_value4,omitempty"`
}

type LineItem struct {
	ProductKey       string  `json:"product_key,omitempty"`
	Notes            string  `json:"notes,omitempty"`
	Cost             float64 `json:"cost,omitempty"`
	ProductCost      float64 `json:"product_cost,omitempty"`
	Quantity         float64 `json:"quantity,omitempty"`
	Discount         float64 `json:"discount,omitempty"`
	IsAmountDiscount bool    `json:"is_amount_discount,omitempty"`
	TaxName1         string  `json:"tax_name1,omitempty"`
	TaxRate1         float64 `json:"tax_rate1,omitempty"`
	TaxName2         string  `json:"tax_name2,omitempty"`
	TaxRate2         float64 `json:"tax_rate2,omitempty"`
	TaxName3         string  `json:"tax_name3,omitempty"`
	TaxRate3         float64 `json:"tax_rate3,omitempty"`
	SortID           int     `json:"sort_id,omitempty"`
	LineTotal        float64 `json:"line_total,omitempty"`
	GrossLineTotal   float64 `json:"gross_line_total,omitempty"`
	TypeID           string  `json:"type_id,omitempty"`
	Date             string  `json:"date,omitempty"`
	CustomValue1     string  `json:"custom_value1,omitempty"`
	CustomValue2     string  `json:"custom_value2,omitempty"`
	CustomValue3     string  `json:"custom_value3,omitempty"`
	CustomValue4     string  `json:"custom_value4,omitempty"`
}

type InvoiceStatus int

const (
	InvoiceDraft         InvoiceStatus = 1
	InvoiceSent          InvoiceStatus = 2
	InvoicePartiallyPaid InvoiceStatus = 3
	InvoicePaid          InvoiceStatus = 4
)

type Invoice struct {
	Entity
	ClientID           string        `json:"client_id,omitempty"`
	ProjectID          string        `json:"project_id,omitempty"`
	VendorID           string        `json:"vendor_id,omitempty"`
	SubscriptionID     string        `json:"subscription_id,omitempty"`
	Number             string        `json:"number,omitempty"`
	StatusID           string        `json:"status_id,omitempty"`
	Amount             float64       `json:"amount,omitempty"`
	Balance            float64       `json:"balance,omitempty"`
	PaidToDate         float64       `json:"paid_to_date,omitempty"`
	Discount           float64       `json:"discount,omitempty"`
	PO                 string        `json:"po_number,omitempty"`
	Date               string        `json:"date,omitempty"`
	DueDate            string        `json:"due_date,omitempty"`
	PublicNotes        string        `json:"public_notes,omitempty"`
	PrivateNotes       string        `json:"private_notes,omitempty"`
	Terms              string        `json:"terms,omitempty"`
	Footer             string        `json:"footer,omitempty"`
	UsesInclusiveTaxes bool          `json:"uses_inclusive_taxes,omitempty"`
	TaxName1           string        `json:"tax_name1,omitempty"`
	TaxRate1           float64       `json:"tax_rate1,omitempty"`
	TaxName2           string        `json:"tax_name2,omitempty"`
	TaxRate2           float64       `json:"tax_rate2,omitempty"`
	TaxName3           string        `json:"tax_name3,omitempty"`
	TaxRate3           float64       `json:"tax_rate3,omitempty"`
	TotalTaxes         float64       `json:"total_taxes,omitempty"`
	Partial            float64       `json:"partial,omitempty"`
	PartialDueDate     string        `json:"partial_due_date,omitempty"`
	ExchangeRate       float64       `json:"exchange_rate,omitempty"`
	LineItems          []LineItem    `json:"line_items,omitempty"`
	Client             *ClientEntity `json:"client,omitempty"`
	Invitations        []Invitation  `json:"invitations,omitempty"`
	Documents          []Document    `json:"documents,omitempty"`
}

type Quote Invoice
type RecurringInvoice Invoice
type Credit Invoice

type Invitation struct {
	ID              string `json:"id,omitempty"`
	ClientContactID string `json:"client_contact_id,omitempty"`
	Key             string `json:"key,omitempty"`
	Link            string `json:"link,omitempty"`
	SentDate        string `json:"sent_date,omitempty"`
	ViewedDate      string `json:"viewed_date,omitempty"`
	OpenedDate      string `json:"opened_date,omitempty"`
	EmailStatus     string `json:"email_status,omitempty"`
	EmailError      string `json:"email_error,omitempty"`
}

type Document struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
	Size int64  `json:"size,omitempty"`
	URL  string `json:"url,omitempty"`
}

type Product struct {
	Entity
	AssignedUserID             string     `json:"assigned_user_id,omitempty"`
	ProjectID                  string     `json:"project_id,omitempty"`
	VendorID                   string     `json:"vendor_id,omitempty"`
	ProductKey                 string     `json:"product_key,omitempty"`
	Notes                      string     `json:"notes,omitempty"`
	Cost                       float64    `json:"cost,omitempty"`
	Price                      float64    `json:"price,omitempty"`
	Quantity                   float64    `json:"quantity,omitempty"`
	InStockQuantity            int        `json:"in_stock_quantity,omitempty"`
	StockNotification          bool       `json:"stock_notification,omitempty"`
	StockNotificationThreshold int        `json:"stock_notification_threshold,omitempty"`
	MaxQuantity                int        `json:"max_quantity,omitempty"`
	ProductImage               string     `json:"product_image,omitempty"`
	TaxID                      string     `json:"tax_id,omitempty"`
	IncomeAccountID            string     `json:"income_account_id,omitempty"`
	TaxName1                   string     `json:"tax_name1,omitempty"`
	TaxRate1                   float64    `json:"tax_rate1,omitempty"`
	TaxName2                   string     `json:"tax_name2,omitempty"`
	TaxRate2                   float64    `json:"tax_rate2,omitempty"`
	TaxName3                   string     `json:"tax_name3,omitempty"`
	TaxRate3                   float64    `json:"tax_rate3,omitempty"`
	CustomValue1               string     `json:"custom_value1,omitempty"`
	CustomValue2               string     `json:"custom_value2,omitempty"`
	CustomValue3               string     `json:"custom_value3,omitempty"`
	CustomValue4               string     `json:"custom_value4,omitempty"`
	Documents                  []Document `json:"documents,omitempty"`
}
type Payment struct {
	Entity
	ClientID             string        `json:"client_id,omitempty"`
	InvoiceID            string        `json:"invoice_id,omitempty"`
	Amount               float64       `json:"amount,omitempty"`
	Applied              float64       `json:"applied,omitempty"`
	Refunded             float64       `json:"refunded,omitempty"`
	Date                 string        `json:"date,omitempty"`
	TransactionReference string        `json:"transaction_reference,omitempty"`
	PrivateNotes         string        `json:"private_notes,omitempty"`
	PaymentTypeID        string        `json:"type_id,omitempty"`
	Client               *ClientEntity `json:"client,omitempty"`
	Invoices             []Invoice     `json:"invoices,omitempty"`
}
