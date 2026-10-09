package goinvoiceninja

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// Vendor is an Invoice Ninja supplier/vendor.
type Vendor struct {
	Entity
	Name         string `json:"name,omitempty"`
	DisplayName  string `json:"display_name,omitempty"`
	Number       string `json:"number,omitempty"`
	CurrencyID   string `json:"currency_id,omitempty"`
	PrivateNotes string `json:"private_notes,omitempty"`
	PublicNotes  string `json:"public_notes,omitempty"`
	CustomValue1 string `json:"custom_value1,omitempty"`
	CustomValue2 string `json:"custom_value2,omitempty"`
	CustomValue3 string `json:"custom_value3,omitempty"`
	CustomValue4 string `json:"custom_value4,omitempty"`
}

// Project groups related quotes, invoices, tasks and expenses for a client job.
type Project struct {
	Entity
	ClientID       string  `json:"client_id,omitempty"`
	Name           string  `json:"name,omitempty"`
	Number         string  `json:"number,omitempty"`
	TaskRate       float64 `json:"task_rate,omitempty"`
	PrivateNotes   string  `json:"private_notes,omitempty"`
	PublicNotes    string  `json:"public_notes,omitempty"`
	CustomValue1   string  `json:"custom_value1,omitempty"`
	CustomValue2   string  `json:"custom_value2,omitempty"`
	CustomValue3   string  `json:"custom_value3,omitempty"`
	CustomValue4   string  `json:"custom_value4,omitempty"`
	BudgetedHours  float64 `json:"budgeted_hours,omitempty"`
	BudgetedAmount float64 `json:"budgeted_amount,omitempty"`
}

// ExpenseCategory is Invoice Ninja's broad expense grouping.
type ExpenseCategory struct {
	Entity
	Name  string `json:"name,omitempty"`
	Color string `json:"color,omitempty"`
}

// Expense models the fields needed for purchase/tax import and BAS reporting.
type Expense struct {
	Entity
	VendorID             string           `json:"vendor_id,omitempty"`
	ClientID             string           `json:"client_id,omitempty"`
	ProjectID            string           `json:"project_id,omitempty"`
	CategoryID           string           `json:"category_id,omitempty"`
	CurrencyID           string           `json:"currency_id,omitempty"`
	Number               string           `json:"number,omitempty"`
	Amount               float64          `json:"amount,omitempty"`
	ForeignAmount        float64          `json:"foreign_amount,omitempty"`
	ExchangeRate         float64          `json:"exchange_rate,omitempty"`
	Date                 string           `json:"date,omitempty"`
	PaymentDate          string           `json:"payment_date,omitempty"`
	PaymentTypeID        string           `json:"payment_type_id,omitempty"`
	TransactionID        string           `json:"transaction_id,omitempty"`
	PrivateNotes         string           `json:"private_notes,omitempty"`
	PublicNotes          string           `json:"public_notes,omitempty"`
	TransactionReference string           `json:"transaction_reference,omitempty"`
	TaxName1             string           `json:"tax_name1,omitempty"`
	TaxRate1             float64          `json:"tax_rate1,omitempty"`
	TaxName2             string           `json:"tax_name2,omitempty"`
	TaxRate2             float64          `json:"tax_rate2,omitempty"`
	TaxName3             string           `json:"tax_name3,omitempty"`
	TaxRate3             float64          `json:"tax_rate3,omitempty"`
	TaxAmount1           float64          `json:"tax_amount1,omitempty"`
	TaxAmount2           float64          `json:"tax_amount2,omitempty"`
	TaxAmount3           float64          `json:"tax_amount3,omitempty"`
	UsesInclusiveTaxes   bool             `json:"uses_inclusive_taxes,omitempty"`
	CalculateTaxByAmount bool             `json:"calculate_tax_by_amount,omitempty"`
	ShouldBeInvoiced     bool             `json:"should_be_invoiced,omitempty"`
	InvoiceDocuments     bool             `json:"invoice_documents,omitempty"`
	CustomValue1         string           `json:"custom_value1,omitempty"`
	CustomValue2         string           `json:"custom_value2,omitempty"`
	CustomValue3         string           `json:"custom_value3,omitempty"`
	CustomValue4         string           `json:"custom_value4,omitempty"`
	Documents            []Document       `json:"documents,omitempty"`
	Vendor               *Vendor          `json:"vendor,omitempty"`
	Project              *Project         `json:"project,omitempty"`
	Category             *ExpenseCategory `json:"category,omitempty"`
}

type CreateVendorRequest struct {
	Name         string `json:"name"`
	Number       string `json:"number,omitempty"`
	CurrencyID   string `json:"currency_id,omitempty"`
	PrivateNotes string `json:"private_notes,omitempty"`
	PublicNotes  string `json:"public_notes,omitempty"`
	CustomValue1 string `json:"custom_value1,omitempty"`
	CustomValue2 string `json:"custom_value2,omitempty"`
	CustomValue3 string `json:"custom_value3,omitempty"`
	CustomValue4 string `json:"custom_value4,omitempty"`
}

type UpdateVendorRequest CreateVendorRequest

type CreateProjectRequest struct {
	ClientID       string  `json:"client_id"`
	Name           string  `json:"name"`
	Number         string  `json:"number,omitempty"`
	TaskRate       float64 `json:"task_rate"`
	PrivateNotes   string  `json:"private_notes,omitempty"`
	PublicNotes    string  `json:"public_notes,omitempty"`
	CustomValue1   string  `json:"custom_value1,omitempty"`
	CustomValue2   string  `json:"custom_value2,omitempty"`
	CustomValue3   string  `json:"custom_value3,omitempty"`
	CustomValue4   string  `json:"custom_value4,omitempty"`
	BudgetedHours  float64 `json:"budgeted_hours,omitempty"`
	BudgetedAmount float64 `json:"budgeted_amount,omitempty"`
}

type UpdateProjectRequest CreateProjectRequest

type CreateExpenseCategoryRequest struct {
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

type UpdateExpenseCategoryRequest CreateExpenseCategoryRequest

type CreateExpenseRequest struct {
	VendorID             string  `json:"vendor_id,omitempty"`
	ClientID             string  `json:"client_id,omitempty"`
	ProjectID            string  `json:"project_id,omitempty"`
	CategoryID           string  `json:"category_id,omitempty"`
	CurrencyID           string  `json:"currency_id,omitempty"`
	Number               string  `json:"number,omitempty"`
	Amount               float64 `json:"amount,omitempty"`
	ForeignAmount        float64 `json:"foreign_amount,omitempty"`
	ExchangeRate         float64 `json:"exchange_rate,omitempty"`
	Date                 string  `json:"date,omitempty"`
	PaymentDate          string  `json:"payment_date,omitempty"`
	PaymentTypeID        string  `json:"payment_type_id,omitempty"`
	PrivateNotes         string  `json:"private_notes,omitempty"`
	PublicNotes          string  `json:"public_notes,omitempty"`
	TransactionReference string  `json:"transaction_reference,omitempty"`
	TaxName1             string  `json:"tax_name1,omitempty"`
	TaxRate1             float64 `json:"tax_rate1,omitempty"`
	TaxName2             string  `json:"tax_name2,omitempty"`
	TaxRate2             float64 `json:"tax_rate2,omitempty"`
	TaxName3             string  `json:"tax_name3,omitempty"`
	TaxRate3             float64 `json:"tax_rate3,omitempty"`
	TaxAmount1           float64 `json:"tax_amount1,omitempty"`
	TaxAmount2           float64 `json:"tax_amount2,omitempty"`
	TaxAmount3           float64 `json:"tax_amount3,omitempty"`
	UsesInclusiveTaxes   bool    `json:"uses_inclusive_taxes,omitempty"`
	CalculateTaxByAmount bool    `json:"calculate_tax_by_amount,omitempty"`
	ShouldBeInvoiced     bool    `json:"should_be_invoiced,omitempty"`
	InvoiceDocuments     *bool   `json:"invoice_documents,omitempty"`
	CustomValue1         string  `json:"custom_value1,omitempty"`
	CustomValue2         string  `json:"custom_value2,omitempty"`
	CustomValue3         string  `json:"custom_value3,omitempty"`
	CustomValue4         string  `json:"custom_value4,omitempty"`
}

// UpdateExpenseRequest omits zero values by default so partial updates do not
// clear unrelated Expense fields. Use WithExplicitFields when a zero value is
// itself the intended update.
type UpdateExpenseRequest struct {
	VendorID             string  `json:"vendor_id,omitempty"`
	ClientID             string  `json:"client_id,omitempty"`
	ProjectID            string  `json:"project_id,omitempty"`
	CategoryID           string  `json:"category_id,omitempty"`
	CurrencyID           string  `json:"currency_id,omitempty"`
	Number               string  `json:"number,omitempty"`
	Amount               float64 `json:"amount,omitempty"`
	ForeignAmount        float64 `json:"foreign_amount,omitempty"`
	ExchangeRate         float64 `json:"exchange_rate,omitempty"`
	Date                 string  `json:"date,omitempty"`
	PaymentDate          string  `json:"payment_date,omitempty"`
	PaymentTypeID        string  `json:"payment_type_id,omitempty"`
	PrivateNotes         string  `json:"private_notes,omitempty"`
	PublicNotes          string  `json:"public_notes,omitempty"`
	TransactionReference string  `json:"transaction_reference,omitempty"`
	TaxName1             string  `json:"tax_name1,omitempty"`
	TaxRate1             float64 `json:"tax_rate1,omitempty"`
	TaxName2             string  `json:"tax_name2,omitempty"`
	TaxRate2             float64 `json:"tax_rate2,omitempty"`
	TaxName3             string  `json:"tax_name3,omitempty"`
	TaxRate3             float64 `json:"tax_rate3,omitempty"`
	TaxAmount1           float64 `json:"tax_amount1,omitempty"`
	TaxAmount2           float64 `json:"tax_amount2,omitempty"`
	TaxAmount3           float64 `json:"tax_amount3,omitempty"`
	UsesInclusiveTaxes   bool    `json:"uses_inclusive_taxes,omitempty"`
	CalculateTaxByAmount bool    `json:"calculate_tax_by_amount,omitempty"`
	ShouldBeInvoiced     bool    `json:"should_be_invoiced,omitempty"`
	InvoiceDocuments     *bool   `json:"invoice_documents,omitempty"`
	CustomValue1         string  `json:"custom_value1,omitempty"`
	CustomValue2         string  `json:"custom_value2,omitempty"`
	CustomValue3         string  `json:"custom_value3,omitempty"`
	CustomValue4         string  `json:"custom_value4,omitempty"`

	explicitFields map[string]struct{}
}

// WithExplicitFields returns a copy that serializes the named JSON fields even
// when their values are empty or zero. Unknown field names fail during JSON
// encoding rather than silently broadening an update.
func (r UpdateExpenseRequest) WithExplicitFields(fields ...string) UpdateExpenseRequest {
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

// MarshalJSON preserves sparse update semantics while allowing selected zero
// values to be sent deliberately.
func (r UpdateExpenseRequest) MarshalJSON() ([]byte, error) {
	type sparseUpdateExpenseRequest UpdateExpenseRequest
	encoded, err := json.Marshal(sparseUpdateExpenseRequest(r))
	if err != nil || len(r.explicitFields) == 0 {
		return encoded, err
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return nil, err
	}
	values := map[string]any{
		"vendor_id":               r.VendorID,
		"client_id":               r.ClientID,
		"project_id":              r.ProjectID,
		"category_id":             r.CategoryID,
		"currency_id":             r.CurrencyID,
		"number":                  r.Number,
		"amount":                  r.Amount,
		"foreign_amount":          r.ForeignAmount,
		"exchange_rate":           r.ExchangeRate,
		"date":                    r.Date,
		"payment_date":            r.PaymentDate,
		"payment_type_id":         r.PaymentTypeID,
		"private_notes":           r.PrivateNotes,
		"public_notes":            r.PublicNotes,
		"transaction_reference":   r.TransactionReference,
		"tax_name1":               r.TaxName1,
		"tax_rate1":               r.TaxRate1,
		"tax_name2":               r.TaxName2,
		"tax_rate2":               r.TaxRate2,
		"tax_name3":               r.TaxName3,
		"tax_rate3":               r.TaxRate3,
		"tax_amount1":             r.TaxAmount1,
		"tax_amount2":             r.TaxAmount2,
		"tax_amount3":             r.TaxAmount3,
		"uses_inclusive_taxes":    r.UsesInclusiveTaxes,
		"calculate_tax_by_amount": r.CalculateTaxByAmount,
		"should_be_invoiced":      r.ShouldBeInvoiced,
		"invoice_documents":       r.InvoiceDocuments,
		"custom_value1":           r.CustomValue1,
		"custom_value2":           r.CustomValue2,
		"custom_value3":           r.CustomValue3,
		"custom_value4":           r.CustomValue4,
	}
	for field := range r.explicitFields {
		value, ok := values[field]
		if !ok {
			return nil, fmt.Errorf("unknown explicit Expense update field %q", field)
		}
		payload[field], err = json.Marshal(value)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(payload)
}

// ExpensePaymentStatusRequest deliberately includes empty values so callers can
// clear an expense's paid state as well as set it.
type ExpensePaymentStatusRequest struct {
	PaymentDate   string  `json:"payment_date"`
	PaymentTypeID string  `json:"payment_type_id"`
	PrivateNotes  *string `json:"private_notes,omitempty"`
}

type VendorService struct{ *Service[Vendor] }
type ProjectService struct{ *Service[Project] }
type ExpenseCategoryService struct{ *Service[ExpenseCategory] }
type ExpenseService struct{ *Service[Expense] }

type VendorQuery struct {
	ListOptions
	Name        string
	Number      string
	WithTrashed bool
	IsDeleted   *bool
}

func (q VendorQuery) Values() url.Values {
	v := q.ListOptions.Values()
	setIf(v, "name", q.Name)
	setIf(v, "number", q.Number)
	setBoolIf(v, "with_trashed", q.WithTrashed)
	setBoolPtr(v, "is_deleted", q.IsDeleted)
	return v
}

func (s *VendorService) List(ctx context.Context, q VendorQuery) (*ListResult[Vendor], error) {
	return listWithQuery[Vendor](ctx, s.client, s.path, q.Values())
}
func (s *VendorService) ListAll(ctx context.Context, q VendorQuery) ([]Vendor, error) {
	return listAllWithValues[Vendor](ctx, s.client, s.path, q.Values())
}
func (s *VendorService) Create(ctx context.Context, req CreateVendorRequest) (*Vendor, error) {
	return s.Service.Create(ctx, req)
}
func (s *VendorService) Update(ctx context.Context, id string, req UpdateVendorRequest) (*Vendor, error) {
	return s.Service.Update(ctx, id, req)
}

type ProjectQuery struct {
	ListOptions
	Number      string
	ClientID    string
	WithTrashed bool
	IsDeleted   *bool
}

func (q ProjectQuery) Values() url.Values {
	v := q.ListOptions.Values()
	setIf(v, "number", q.Number)
	setIf(v, "client_id", q.ClientID)
	setBoolIf(v, "with_trashed", q.WithTrashed)
	setBoolPtr(v, "is_deleted", q.IsDeleted)
	return v
}

func (s *ProjectService) List(ctx context.Context, q ProjectQuery) (*ListResult[Project], error) {
	return listWithQuery[Project](ctx, s.client, s.path, q.Values())
}
func (s *ProjectService) ListAll(ctx context.Context, q ProjectQuery) ([]Project, error) {
	return listAllWithValues[Project](ctx, s.client, s.path, q.Values())
}
func (s *ProjectService) Create(ctx context.Context, req CreateProjectRequest) (*Project, error) {
	return s.Service.Create(ctx, req)
}
func (s *ProjectService) Update(ctx context.Context, id string, req UpdateProjectRequest) (*Project, error) {
	return s.Service.Update(ctx, id, req)
}

type ExpenseCategoryQuery struct {
	ListOptions
	Name        string
	WithTrashed bool
	IsDeleted   *bool
}

func (q ExpenseCategoryQuery) Values() url.Values {
	v := q.ListOptions.Values()
	setIf(v, "name", q.Name)
	setBoolIf(v, "with_trashed", q.WithTrashed)
	setBoolPtr(v, "is_deleted", q.IsDeleted)
	return v
}

func (s *ExpenseCategoryService) List(ctx context.Context, q ExpenseCategoryQuery) (*ListResult[ExpenseCategory], error) {
	return listWithQuery[ExpenseCategory](ctx, s.client, s.path, q.Values())
}
func (s *ExpenseCategoryService) ListAll(ctx context.Context, q ExpenseCategoryQuery) ([]ExpenseCategory, error) {
	return listAllWithValues[ExpenseCategory](ctx, s.client, s.path, q.Values())
}
func (s *ExpenseCategoryService) Create(ctx context.Context, req CreateExpenseCategoryRequest) (*ExpenseCategory, error) {
	return s.Service.Create(ctx, req)
}
func (s *ExpenseCategoryService) Update(ctx context.Context, id string, req UpdateExpenseCategoryRequest) (*ExpenseCategory, error) {
	return s.Service.Update(ctx, id, req)
}

type ExpenseQuery struct {
	ListOptions
	VendorID             string
	ClientID             string
	ProjectID            string
	CategoryID           string
	TransactionReference string
	Date                 string
	PrivateNotes         string
	WithTrashed          bool
	IsDeleted            *bool
}

func (q ExpenseQuery) Values() url.Values {
	v := q.ListOptions.Values()
	setIf(v, "vendor_id", q.VendorID)
	setIf(v, "client_id", q.ClientID)
	setIf(v, "project_id", q.ProjectID)
	setIf(v, "category_id", q.CategoryID)
	setIf(v, "transaction_reference", q.TransactionReference)
	setIf(v, "date", q.Date)
	setIf(v, "private_notes", q.PrivateNotes)
	setBoolIf(v, "with_trashed", q.WithTrashed)
	setBoolPtr(v, "is_deleted", q.IsDeleted)
	return v
}

func (s *ExpenseService) List(ctx context.Context, q ExpenseQuery) (*ListResult[Expense], error) {
	return listWithQuery[Expense](ctx, s.client, s.path, q.Values())
}
func (s *ExpenseService) ListAll(ctx context.Context, q ExpenseQuery) ([]Expense, error) {
	return listAllWithValues[Expense](ctx, s.client, s.path, q.Values())
}
func (s *ExpenseService) Create(ctx context.Context, req CreateExpenseRequest) (*Expense, error) {
	return s.Service.Create(ctx, req)
}
func (s *ExpenseService) Update(ctx context.Context, id string, req UpdateExpenseRequest) (*Expense, error) {
	return s.Service.Update(ctx, id, req)
}

// UpdatePaymentStatus sets or clears the fields Invoice Ninja uses for an
// expense's paid state.
func (s *ExpenseService) UpdatePaymentStatus(ctx context.Context, id string, req ExpensePaymentStatusRequest) (*Expense, error) {
	return s.Service.Update(ctx, id, req)
}
