package goinvoiceninja

import (
	"context"
	"net/http"
	"net/url"
)

type ClientService struct{ *Service[ClientEntity] }
type ProductService struct{ *Service[Product] }
type QuoteService struct{ *Service[Quote] }
type InvoiceService struct{ *Service[Invoice] }
type PaymentService struct{ *Service[Payment] }

// ClientQuery covers the fields normally useful when looking up customers.
type ClientQuery struct {
	ListOptions
	Email       string
	Number      string
	Phone       string
	Name        string
	IDNumber    string
	VATNumber   string
	WithTrashed bool
	IsDeleted   *bool
}

func (q ClientQuery) Values() url.Values {
	v := q.ListOptions.Values()
	setIf(v, "email", q.Email)
	setIf(v, "number", q.Number)
	setIf(v, "phone", q.Phone)
	setIf(v, "name", q.Name)
	setIf(v, "id_number", q.IDNumber)
	setIf(v, "vat_number", q.VATNumber)
	setBoolIf(v, "with_trashed", q.WithTrashed)
	setBoolPtr(v, "is_deleted", q.IsDeleted)
	return v
}

func (s *ClientService) List(ctx context.Context, q ClientQuery) (*ListResult[ClientEntity], error) {
	return listWithQuery[ClientEntity](ctx, s.client, s.path, q.Values())
}

// ListAll fetches all client pages for the supplied query.
func (s *ClientService) ListAll(ctx context.Context, q ClientQuery) ([]ClientEntity, error) {
	return listAllWithValues[ClientEntity](ctx, s.client, s.path, q.Values())
}

func (s *ClientService) Create(ctx context.Context, req CreateClientRequest) (*ClientEntity, error) {
	return s.Service.Create(ctx, req)
}

func (s *ClientService) Update(ctx context.Context, id string, req UpdateClientRequest) (*ClientEntity, error) {
	return s.Service.Update(ctx, id, req)
}

func (s *ClientService) FindByEmail(ctx context.Context, email string) (*ClientEntity, error) {
	result, err := s.List(ctx, ClientQuery{ListOptions: ListOptions{PerPage: 1}, Email: email})
	if err != nil {
		return nil, err
	}
	if len(result.Data) == 0 {
		return nil, ErrNotFound
	}
	return &result.Data[0], nil
}

// ProductQuery covers product catalogue searches.
type ProductQuery struct {
	ListOptions
	ProductKey   string
	CustomValue1 string
	CustomValue2 string
	CustomValue3 string
	CustomValue4 string
	WithTrashed  bool
	IsDeleted    *bool
}

func (q ProductQuery) Values() url.Values {
	v := q.ListOptions.Values()
	setIf(v, "product_key", q.ProductKey)
	setIf(v, "custom_value1", q.CustomValue1)
	setIf(v, "custom_value2", q.CustomValue2)
	setIf(v, "custom_value3", q.CustomValue3)
	setIf(v, "custom_value4", q.CustomValue4)
	setBoolIf(v, "with_trashed", q.WithTrashed)
	setBoolPtr(v, "is_deleted", q.IsDeleted)
	return v
}

func (s *ProductService) List(ctx context.Context, q ProductQuery) (*ListResult[Product], error) {
	return listWithQuery[Product](ctx, s.client, s.path, q.Values())
}

// ListAll fetches all product pages for the supplied query.
func (s *ProductService) ListAll(ctx context.Context, q ProductQuery) ([]Product, error) {
	return listAllWithValues[Product](ctx, s.client, s.path, q.Values())
}

func (s *ProductService) Create(ctx context.Context, req CreateProductRequest) (*Product, error) {
	return s.Service.Create(ctx, req)
}

func (s *ProductService) Update(ctx context.Context, id string, req UpdateProductRequest) (*Product, error) {
	return s.Service.Update(ctx, id, req)
}

func (s *ProductService) FindByKey(ctx context.Context, key string) (*Product, error) {
	result, err := s.List(ctx, ProductQuery{ListOptions: ListOptions{PerPage: 1}, ProductKey: key})
	if err != nil {
		return nil, err
	}
	if len(result.Data) == 0 {
		return nil, ErrNotFound
	}
	return &result.Data[0], nil
}

// FindByBunningsIN returns the first product whose recommended Bunnings IN field
// matches the supplied value. By convention this package stores the Bunnings
// item number / IN in product custom_value1.
func (s *ProductService) FindByBunningsIN(ctx context.Context, in string) (*Product, error) {
	return s.FindByCustomValue1(ctx, in)
}

// FindByCustomValue1 returns the first product matching custom_value1.
func (s *ProductService) FindByCustomValue1(ctx context.Context, value string) (*Product, error) {
	result, err := s.List(ctx, ProductQuery{ListOptions: ListOptions{PerPage: 1}, CustomValue1: value})
	if err != nil {
		return nil, err
	}
	if len(result.Data) == 0 {
		return nil, ErrNotFound
	}
	return &result.Data[0], nil
}

// QuoteQuery mirrors the invoice-style query options normally used for sales documents.
type QuoteQuery struct {
	ListOptions
	Number       string
	ClientID     string
	ClientStatus []InvoiceStatus
	WithTrashed  bool
	IsDeleted    *bool
}

func (q QuoteQuery) Values() url.Values {
	v := q.ListOptions.Values()
	setIf(v, "number", q.Number)
	setIf(v, "client_id", q.ClientID)
	if len(q.ClientStatus) > 0 {
		v.Set("client_status", statuses(q.ClientStatus))
	}
	setBoolIf(v, "with_trashed", q.WithTrashed)
	setBoolPtr(v, "is_deleted", q.IsDeleted)
	return v
}

func (s *QuoteService) List(ctx context.Context, q QuoteQuery) (*ListResult[Quote], error) {
	return listWithQuery[Quote](ctx, s.client, s.path, q.Values())
}

// ListAll fetches all quote pages for the supplied query.
func (s *QuoteService) ListAll(ctx context.Context, q QuoteQuery) ([]Quote, error) {
	return listAllWithValues[Quote](ctx, s.client, s.path, q.Values())
}

func (s *QuoteService) Create(ctx context.Context, req CreateQuoteRequest) (*Quote, error) {
	return s.Service.Create(ctx, req)
}

func (s *QuoteService) Update(ctx context.Context, id string, req UpdateQuoteRequest) (*Quote, error) {
	return s.Service.Update(ctx, id, req)
}

func (s *QuoteService) Email(ctx context.Context, id string) (*Quote, error) {
	return s.Action(ctx, id, "email", nil)
}

func (s *QuoteService) Clone(ctx context.Context, id string) (*Quote, error) {
	return s.Action(ctx, id, "clone", nil)
}

// InvoiceQuery covers the most common Invoice Ninja invoice filters.
type InvoiceQuery struct {
	ListOptions
	Number                string
	ClientID              string
	ClientStatus          []InvoiceStatus
	WithoutDeletedClients bool
	WithTrashed           bool
	Upcoming              bool
	Overdue               bool
	Payable               bool
	PrivateNotes          string
	CreatedAt             int64
	UpdatedAt             int64
	IsDeleted             *bool
}

func (q InvoiceQuery) Values() url.Values {
	v := q.ListOptions.Values()
	setIf(v, "number", q.Number)
	setIf(v, "client_id", q.ClientID)
	if len(q.ClientStatus) > 0 {
		v.Set("client_status", statuses(q.ClientStatus))
	}
	setBoolIf(v, "without_deleted_clients", q.WithoutDeletedClients)
	setBoolIf(v, "with_trashed", q.WithTrashed)
	setBoolIf(v, "upcoming", q.Upcoming)
	setBoolIf(v, "overdue", q.Overdue)
	setBoolIf(v, "payable", q.Payable)
	setIf(v, "private_notes", q.PrivateNotes)
	if q.CreatedAt > 0 {
		v.Set("created_at", itoa64(q.CreatedAt))
	}
	if q.UpdatedAt > 0 {
		v.Set("updated_at", itoa64(q.UpdatedAt))
	}
	setBoolPtr(v, "is_deleted", q.IsDeleted)
	return v
}

func (s *InvoiceService) List(ctx context.Context, q InvoiceQuery) (*ListResult[Invoice], error) {
	return listWithQuery[Invoice](ctx, s.client, s.path, q.Values())
}

// ListAll fetches all invoice pages for the supplied query.
func (s *InvoiceService) ListAll(ctx context.Context, q InvoiceQuery) ([]Invoice, error) {
	return listAllWithValues[Invoice](ctx, s.client, s.path, q.Values())
}

func (s *InvoiceService) Create(ctx context.Context, req CreateInvoiceRequest) (*Invoice, error) {
	return s.Service.Create(ctx, req)
}

func (s *InvoiceService) Update(ctx context.Context, id string, req UpdateInvoiceRequest) (*Invoice, error) {
	return s.Service.Update(ctx, id, req)
}

func (s *InvoiceService) Email(ctx context.Context, id string) (*Invoice, error) {
	return s.Action(ctx, id, "email", nil)
}

func (s *InvoiceService) MarkSent(ctx context.Context, id string) (*Invoice, error) {
	return s.Action(ctx, id, "mark_sent", nil)
}

func (s *InvoiceService) Clone(ctx context.Context, id string) (*Invoice, error) {
	return s.Action(ctx, id, "clone", nil)
}

// PaymentQuery covers payment lookups by client, invoice, transaction, and date.
type PaymentQuery struct {
	ListOptions
	ClientID             string
	InvoiceID            string
	TransactionReference string
	Date                 string
	WithTrashed          bool
	IsDeleted            *bool
}

func (q PaymentQuery) Values() url.Values {
	v := q.ListOptions.Values()
	setIf(v, "client_id", q.ClientID)
	setIf(v, "invoice_id", q.InvoiceID)
	setIf(v, "transaction_reference", q.TransactionReference)
	setIf(v, "date", q.Date)
	setBoolIf(v, "with_trashed", q.WithTrashed)
	setBoolPtr(v, "is_deleted", q.IsDeleted)
	return v
}

func (s *PaymentService) List(ctx context.Context, q PaymentQuery) (*ListResult[Payment], error) {
	return listWithQuery[Payment](ctx, s.client, s.path, q.Values())
}

// ListAll fetches all payment pages for the supplied query.
func (s *PaymentService) ListAll(ctx context.Context, q PaymentQuery) ([]Payment, error) {
	return listAllWithValues[Payment](ctx, s.client, s.path, q.Values())
}

func (s *PaymentService) Create(ctx context.Context, req CreatePaymentRequest) (*Payment, error) {
	return s.Service.Create(ctx, req)
}

func (s *PaymentService) Update(ctx context.Context, id string, req UpdatePaymentRequest) (*Payment, error) {
	return s.Service.Update(ctx, id, req)
}

func listWithQuery[T any](ctx context.Context, c *Client, path string, q url.Values) (*ListResult[T], error) {
	req, err := c.NewRequest(ctx, http.MethodGet, path, q, nil)
	if err != nil {
		return nil, err
	}
	raw, err := rawDo(c, req)
	if err != nil {
		return nil, err
	}
	data, meta, err := decodeListEnvelope[T](raw)
	if err != nil {
		return nil, err
	}
	return &ListResult[T]{Data: data, Meta: meta}, nil
}

func setIf(v url.Values, key, value string) {
	if value != "" {
		v.Set(key, value)
	}
}

func setBoolIf(v url.Values, key string, value bool) {
	if value {
		v.Set(key, "true")
	}
}

func setBoolPtr(v url.Values, key string, value *bool) {
	if value == nil {
		return
	}
	if *value {
		v.Set(key, "true")
		return
	}
	v.Set(key, "false")
}
