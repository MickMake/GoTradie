package goinvoiceninja

import (
	"encoding/json"
	"fmt"
)

// CreateClientRequest creates an Invoice Ninja client/customer.
type CreateClientRequest struct {
	Name         string    `json:"name,omitempty"`
	Contacts     []Contact `json:"contacts,omitempty"`
	Phone        string    `json:"phone,omitempty"`
	Website      string    `json:"website,omitempty"`
	Address1     string    `json:"address1,omitempty"`
	Address2     string    `json:"address2,omitempty"`
	City         string    `json:"city,omitempty"`
	State        string    `json:"state,omitempty"`
	PostalCode   string    `json:"postal_code,omitempty"`
	CountryID    string    `json:"country_id,omitempty"`
	PrivateNotes string    `json:"private_notes,omitempty"`
	PublicNotes  string    `json:"public_notes,omitempty"`
	IDNumber     string    `json:"id_number,omitempty"`
	VATNumber    string    `json:"vat_number,omitempty"`
	CustomValue1 string    `json:"custom_value1,omitempty"`
	CustomValue2 string    `json:"custom_value2,omitempty"`
	CustomValue3 string    `json:"custom_value3,omitempty"`
	CustomValue4 string    `json:"custom_value4,omitempty"`
}

// UpdateClientRequest updates a client. When updating clients, include existing contact IDs
// on contacts you want to preserve.
type UpdateClientRequest CreateClientRequest

// CreateProductRequest creates a catalogue product.
type CreateProductRequest struct {
	AssignedUserID             string  `json:"assigned_user_id,omitempty"`
	ProjectID                  string  `json:"project_id,omitempty"`
	VendorID                   string  `json:"vendor_id,omitempty"`
	ProductKey                 string  `json:"product_key,omitempty"`
	Notes                      string  `json:"notes,omitempty"`
	Cost                       float64 `json:"cost,omitempty"`
	Price                      float64 `json:"price,omitempty"`
	Quantity                   float64 `json:"quantity,omitempty"`
	InStockQuantity            int     `json:"in_stock_quantity,omitempty"`
	StockNotification          bool    `json:"stock_notification,omitempty"`
	StockNotificationThreshold int     `json:"stock_notification_threshold,omitempty"`
	MaxQuantity                int     `json:"max_quantity,omitempty"`
	ProductImage               string  `json:"product_image,omitempty"`
	TaxID                      string  `json:"tax_id,omitempty"`
	IncomeAccountID            string  `json:"income_account_id,omitempty"`
	TaxName1                   string  `json:"tax_name1,omitempty"`
	TaxRate1                   float64 `json:"tax_rate1,omitempty"`
	TaxName2                   string  `json:"tax_name2,omitempty"`
	TaxRate2                   float64 `json:"tax_rate2,omitempty"`
	TaxName3                   string  `json:"tax_name3,omitempty"`
	TaxRate3                   float64 `json:"tax_rate3,omitempty"`
	CustomValue1               string  `json:"custom_value1,omitempty"`
	CustomValue2               string  `json:"custom_value2,omitempty"`
	CustomValue3               string  `json:"custom_value3,omitempty"`
	CustomValue4               string  `json:"custom_value4,omitempty"`
}

type UpdateProductRequest CreateProductRequest

// SparseProductUpdateRequest preserves Product partial-update semantics while
// allowing callers to deliberately send selected empty or zero values.
type SparseProductUpdateRequest struct {
	Values CreateProductRequest

	explicitFields map[string]struct{}
}

// NewSparseProductUpdateRequest creates a sparse Product update from the
// supplied values. Non-zero values are encoded normally.
func NewSparseProductUpdateRequest(values CreateProductRequest) SparseProductUpdateRequest {
	return SparseProductUpdateRequest{Values: values}
}

// WithExplicitFields returns a copy that also serializes the named JSON fields
// when their values are empty or zero.
func (r SparseProductUpdateRequest) WithExplicitFields(fields ...string) SparseProductUpdateRequest {
	explicitFields := make(map[string]struct{}, len(r.explicitFields)+len(fields))
	for field := range r.explicitFields {
		explicitFields[field] = struct{}{}
	}
	for _, field := range fields {
		explicitFields[field] = struct{}{}
	}
	r.explicitFields = explicitFields
	return r
}

func (r SparseProductUpdateRequest) MarshalJSON() ([]byte, error) {
	encoded, err := json.Marshal(r.Values)
	if err != nil || len(r.explicitFields) == 0 {
		return encoded, err
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return nil, err
	}
	values := map[string]any{
		"assigned_user_id":             r.Values.AssignedUserID,
		"project_id":                   r.Values.ProjectID,
		"vendor_id":                    r.Values.VendorID,
		"product_key":                  r.Values.ProductKey,
		"notes":                        r.Values.Notes,
		"cost":                         r.Values.Cost,
		"price":                        r.Values.Price,
		"quantity":                     r.Values.Quantity,
		"in_stock_quantity":            r.Values.InStockQuantity,
		"stock_notification":           r.Values.StockNotification,
		"stock_notification_threshold": r.Values.StockNotificationThreshold,
		"max_quantity":                 r.Values.MaxQuantity,
		"product_image":                r.Values.ProductImage,
		"tax_id":                       r.Values.TaxID,
		"income_account_id":            r.Values.IncomeAccountID,
		"tax_name1":                    r.Values.TaxName1,
		"tax_rate1":                    r.Values.TaxRate1,
		"tax_name2":                    r.Values.TaxName2,
		"tax_rate2":                    r.Values.TaxRate2,
		"tax_name3":                    r.Values.TaxName3,
		"tax_rate3":                    r.Values.TaxRate3,
		"custom_value1":                r.Values.CustomValue1,
		"custom_value2":                r.Values.CustomValue2,
		"custom_value3":                r.Values.CustomValue3,
		"custom_value4":                r.Values.CustomValue4,
	}
	for field := range r.explicitFields {
		value, ok := values[field]
		if !ok {
			return nil, fmt.Errorf("unknown explicit Product update field %q", field)
		}
		payload[field], err = json.Marshal(value)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(payload)
}

// SalesDocumentRequest contains fields shared by quotes and invoices.
type SalesDocumentRequest struct {
	ClientID           string     `json:"client_id"`
	ProjectID          string     `json:"project_id,omitempty"`
	VendorID           string     `json:"vendor_id,omitempty"`
	Number             string     `json:"number,omitempty"`
	Discount           float64    `json:"discount,omitempty"`
	PONumber           string     `json:"po_number,omitempty"`
	Date               string     `json:"date,omitempty"`
	DueDate            string     `json:"due_date,omitempty"`
	PublicNotes        string     `json:"public_notes,omitempty"`
	PrivateNotes       string     `json:"private_notes,omitempty"`
	Terms              string     `json:"terms,omitempty"`
	Footer             string     `json:"footer,omitempty"`
	UsesInclusiveTaxes bool       `json:"uses_inclusive_taxes,omitempty"`
	TaxName1           string     `json:"tax_name1,omitempty"`
	TaxRate1           float64    `json:"tax_rate1,omitempty"`
	TaxName2           string     `json:"tax_name2,omitempty"`
	TaxRate2           float64    `json:"tax_rate2,omitempty"`
	TaxName3           string     `json:"tax_name3,omitempty"`
	TaxRate3           float64    `json:"tax_rate3,omitempty"`
	IsAmountDiscount   bool       `json:"is_amount_discount,omitempty"`
	Partial            float64    `json:"partial,omitempty"`
	PartialDueDate     string     `json:"partial_due_date,omitempty"`
	ExchangeRate       float64    `json:"exchange_rate,omitempty"`
	AutoBillEnabled    bool       `json:"auto_bill_enabled,omitempty"`
	AssignedUserID     string     `json:"assigned_user_id,omitempty"`
	CustomValue1       string     `json:"custom_value1,omitempty"`
	CustomValue2       string     `json:"custom_value2,omitempty"`
	CustomValue3       string     `json:"custom_value3,omitempty"`
	CustomValue4       string     `json:"custom_value4,omitempty"`
	LineItems          []LineItem `json:"line_items"`
}

type CreateInvoiceRequest SalesDocumentRequest
type UpdateInvoiceRequest SalesDocumentRequest
type CreateQuoteRequest SalesDocumentRequest
type UpdateQuoteRequest SalesDocumentRequest

// CreatePaymentRequest records a payment. Use InvoiceIDs for one or more invoices when needed.
type CreatePaymentRequest struct {
	ClientID             string   `json:"client_id,omitempty"`
	InvoiceID            string   `json:"invoice_id,omitempty"`
	InvoiceIDs           []string `json:"invoices,omitempty"`
	Amount               float64  `json:"amount,omitempty"`
	Applied              float64  `json:"applied,omitempty"`
	Date                 string   `json:"date,omitempty"`
	TransactionReference string   `json:"transaction_reference,omitempty"`
	PrivateNotes         string   `json:"private_notes,omitempty"`
	PaymentTypeID        string   `json:"type_id,omitempty"`
	IsManual             bool     `json:"is_manual,omitempty"`
}

type UpdatePaymentRequest CreatePaymentRequest
