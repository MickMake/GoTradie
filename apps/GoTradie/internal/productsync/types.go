package productsync

import (
	"context"
	"net/http"
	"time"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/bunnings"
	"github.com/MickMake/GoTradie/internal/config"
)

type InvoiceNinja interface {
	ListAllProducts(context.Context) ([]invoiceninja.Product, error)
	ListAllVendors(context.Context) ([]invoiceninja.Vendor, error)
	CreateCatalogProduct(context.Context, invoiceninja.CreateProductRequest) (invoiceninja.Product, error)
	UpdateCatalogProduct(context.Context, string, invoiceninja.SparseProductUpdateRequest) (invoiceninja.Product, error)
}

type Bunnings interface {
	GetProduct(context.Context, string) (bunnings.Product, error)
}

type Service struct {
	Config     config.Config
	Ninja      InvoiceNinja
	Bunnings   Bunnings
	HTTPClient *http.Client
	Cache      FingerprintStore
	Now        func() time.Time
	Commit     bool
}

type Result struct {
	Provider   string
	ItemNumber string
	ProductKey string
	Action     string
	Changes    []string
	Error      error
}

type availability int

const (
	availabilityUnknown availability = iota
	availabilityAvailable
	availabilityNotAvailable
)

type observation struct {
	Key          string
	Description  *string
	Cost         *float64
	Price        *float64
	Quantity     *float64
	ImageURL     *string
	Availability availability
}

func stringPointer(value string) *string  { return &value }
func floatPointer(value float64) *float64 { return &value }
