package goinvoiceninja

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
