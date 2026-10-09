package ninja

import (
	"context"
	"errors"
	"fmt"
	"strings"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/config"
)

type Service struct {
	client *invoiceninja.Client
	cfg    config.Config
}

// bunningsProductPrefix preserves the established Product identity until the
// v0.5.7 Product identity and four-custom-field design is implemented.
const bunningsProductPrefix = "BUNNINGS-"

func New(cfg config.Config) (*Service, error) {
	opts := []invoiceninja.Option{invoiceninja.WithUserAgent("GoTradie/v0.5.4")}
	if cfg.InvoiceNinja.URL != "" {
		opts = append(opts, invoiceninja.WithBaseURL(cfg.InvoiceNinja.URL))
	}
	c, err := invoiceninja.New(cfg.InvoiceNinja.Token, opts...)
	if err != nil {
		return nil, err
	}
	return &Service{client: c, cfg: cfg}, nil
}

func (s *Service) ListProducts(ctx context.Context) ([]invoiceninja.Product, error) {
	return s.client.Products.ListAll(ctx, invoiceninja.ProductQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Status: "active"},
	})
}

func (s *Service) FindByBunningsIN(ctx context.Context, itemNumber string) (*invoiceninja.Product, error) {
	itemNumber = strings.TrimSpace(itemNumber)
	key := s.ProductKey(itemNumber)
	p, err := s.client.Products.FindByKey(ctx, key)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, invoiceninja.ErrNotFound) {
		return nil, err
	}
	products, err := s.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	for i := range products {
		if s.CustomValue(products[i], s.cfg.ProductSync.CustomFields.BunningsIN) == itemNumber {
			return &products[i], nil
		}
	}
	return nil, invoiceninja.ErrNotFound
}

func (s *Service) UpsertProduct(ctx context.Context, itemNumber, notes, imageURL string, price float64) (invoiceninja.Product, bool, []string, error) {
	itemNumber = strings.TrimSpace(itemNumber)
	if itemNumber == "" {
		return invoiceninja.Product{}, false, nil, fmt.Errorf("Bunnings item number is required")
	}
	existing, err := s.FindByBunningsIN(ctx, itemNumber)
	if err != nil && !errors.Is(err, invoiceninja.ErrNotFound) {
		return invoiceninja.Product{}, false, nil, err
	}
	payload := invoiceninja.CreateProductRequest{
		ProductKey: s.ProductKey(itemNumber),
		Notes:      notes,
		Price:      price,
		Quantity:   1,
		TaxName1:   s.cfg.Tax.Name,
		TaxRate1:   s.cfg.Tax.Rate,
	}
	s.SetCustomCreate(&payload, s.cfg.ProductSync.CustomFields.BunningsIN, itemNumber)
	s.SetCustomCreate(&payload, s.cfg.ProductSync.CustomFields.ImageURL, imageURL)
	if errors.Is(err, invoiceninja.ErrNotFound) {
		created, err := s.client.Products.Create(ctx, payload)
		if err != nil {
			return invoiceninja.Product{}, false, nil, err
		}
		return *created, true, []string{"created"}, nil
	}
	changes := diffProduct(*existing, payload, s.cfg.ProductSync.CustomFields.BunningsIN, s.cfg.ProductSync.CustomFields.ImageURL)
	if len(changes) == 0 {
		return *existing, false, nil, nil
	}
	updated, err := s.client.Products.Update(ctx, existing.ID, invoiceninja.UpdateProductRequest(payload))
	if err != nil {
		return invoiceninja.Product{}, false, changes, err
	}
	return *updated, false, changes, nil
}

func (s *Service) ProductKey(itemNumber string) string {
	return bunningsProductPrefix + strings.TrimSpace(itemNumber)
}

func (s *Service) CustomValue(p invoiceninja.Product, idx int) string {
	switch idx {
	case 1:
		return p.CustomValue1
	case 2:
		return p.CustomValue2
	case 3:
		return p.CustomValue3
	case 4:
		return p.CustomValue4
	default:
		return ""
	}
}

func (s *Service) SetCustomCreate(p *invoiceninja.CreateProductRequest, idx int, val string) {
	switch idx {
	case 1:
		p.CustomValue1 = val
	case 2:
		p.CustomValue2 = val
	case 3:
		p.CustomValue3 = val
	case 4:
		p.CustomValue4 = val
	}
}

func diffProduct(existing invoiceninja.Product, next invoiceninja.CreateProductRequest, bunningsCustom, imageCustom int) []string {
	var changes []string
	if existing.ProductKey != next.ProductKey {
		changes = append(changes, "product_key")
	}
	if strings.TrimSpace(existing.Notes) != strings.TrimSpace(next.Notes) {
		changes = append(changes, "notes")
	}
	if next.Price != 0 && existing.Price != next.Price {
		changes = append(changes, "price")
	}
	if existing.TaxName1 != next.TaxName1 || existing.TaxRate1 != next.TaxRate1 {
		changes = append(changes, "tax")
	}
	custom := func(p invoiceninja.Product, idx int) string {
		switch idx {
		case 1:
			return p.CustomValue1
		case 2:
			return p.CustomValue2
		case 3:
			return p.CustomValue3
		case 4:
			return p.CustomValue4
		default:
			return ""
		}
	}
	payloadCustom := func(p invoiceninja.CreateProductRequest, idx int) string {
		switch idx {
		case 1:
			return p.CustomValue1
		case 2:
			return p.CustomValue2
		case 3:
			return p.CustomValue3
		case 4:
			return p.CustomValue4
		default:
			return ""
		}
	}
	if custom(existing, bunningsCustom) != payloadCustom(next, bunningsCustom) {
		changes = append(changes, "bunnings_in")
	}
	if custom(existing, imageCustom) != payloadCustom(next, imageCustom) {
		changes = append(changes, "image_url")
	}
	return changes
}
