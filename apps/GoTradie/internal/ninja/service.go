package ninja

import (
	"context"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/config"
)

type Service struct {
	client *invoiceninja.Client
	cfg    config.Config
}

func New(cfg config.Config) (*Service, error) {
	opts := []invoiceninja.Option{invoiceninja.WithUserAgent("GoTradie/v0.5.7")}
	if cfg.InvoiceNinja.URL != "" {
		opts = append(opts, invoiceninja.WithBaseURL(cfg.InvoiceNinja.URL))
	}
	c, err := invoiceninja.New(cfg.InvoiceNinja.Token, opts...)
	if err != nil {
		return nil, err
	}
	return &Service{client: c, cfg: cfg}, nil
}

func (s *Service) ListAllProducts(ctx context.Context) ([]invoiceninja.Product, error) {
	return s.client.Products.ListAll(ctx, invoiceninja.ProductQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100},
		WithTrashed: true,
	})
}

func (s *Service) ListAllVendors(ctx context.Context) ([]invoiceninja.Vendor, error) {
	return s.client.Vendors.ListAll(ctx, invoiceninja.VendorQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100},
		WithTrashed: true,
	})
}

func (s *Service) CreateCatalogProduct(ctx context.Context, req invoiceninja.CreateProductRequest) (invoiceninja.Product, error) {
	product, err := s.client.Products.Create(ctx, req)
	if err != nil {
		return invoiceninja.Product{}, err
	}
	return *product, nil
}

func (s *Service) UpdateCatalogProduct(ctx context.Context, id string, req invoiceninja.SparseProductUpdateRequest) (invoiceninja.Product, error) {
	product, err := s.client.Products.UpdateSparse(ctx, id, req)
	if err != nil {
		return invoiceninja.Product{}, err
	}
	return *product, nil
}

func (s *Service) ListProducts(ctx context.Context) ([]invoiceninja.Product, error) {
	return s.client.Products.ListAll(ctx, invoiceninja.ProductQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Status: "active"},
	})
}
