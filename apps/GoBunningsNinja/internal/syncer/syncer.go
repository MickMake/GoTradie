package syncer

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/MickMake/GoBunningsNinja/internal/bunnings"
	invoiceninja "github.com/MickMake/GoInvoiceNinja"
)

type Bunnings interface {
	GetProduct(context.Context, string) (bunnings.Product, error)
	Search(context.Context, string, int) ([]bunnings.Product, error)
	Hydrate(context.Context, bunnings.Product) (bunnings.Product, error)
	WithWeb(bool) *bunnings.Service
}

type Ninja interface {
	ListProducts(context.Context) ([]invoiceninja.Product, error)
	FindByBunningsIN(context.Context, string) (*invoiceninja.Product, error)
	UpsertProduct(context.Context, string, string, string, float64) (invoiceninja.Product, bool, []string, error)
	ProductKey(string) string
	CustomValue(invoiceninja.Product, int) string
}

type Service struct {
	Bunnings       Bunnings
	Ninja          Ninja
	BunningsCustom int
	DryRun         bool
}

type Result struct {
	ItemNumber string
	ProductKey string
	Action     string
	Changes    []string
	Error      error
}

func (s Service) SyncExisting(ctx context.Context) ([]Result, error) {
	products, err := s.Ninja.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(products))
	for _, p := range products {
		in := s.Ninja.CustomValue(p, s.BunningsCustom)
		if in == "" {
			in = inferItemNumber(p.ProductKey)
		}
		if in == "" {
			continue
		}
		res := s.syncOne(ctx, in)
		res.ProductKey = p.ProductKey
		results = append(results, res)
	}
	return results, nil
}

func (s Service) AddByIN(ctx context.Context, itemNumber string) Result {
	return s.syncOne(ctx, itemNumber)
}

func (s Service) Search(ctx context.Context, query string, limit int) ([]bunnings.Product, error) {
	return s.Bunnings.Search(ctx, query, limit)
}

func (s Service) AddProducts(ctx context.Context, products []bunnings.Product) []Result {
	results := make([]Result, 0, len(products))
	for _, p := range products {
		results = append(results, s.syncOne(ctx, p.ItemNumber))
	}
	return results
}

func (s Service) syncOne(ctx context.Context, itemNumber string) Result {
	itemNumber = strings.TrimSpace(itemNumber)
	res := Result{ItemNumber: itemNumber, ProductKey: s.Ninja.ProductKey(itemNumber)}
	prod, err := s.Bunnings.GetProduct(ctx, itemNumber)
	if err != nil {
		res.Action = "error"
		res.Error = err
		return res
	}
	notes := productNotes(prod)
	if s.DryRun {
		existing, err := s.Ninja.FindByBunningsIN(ctx, itemNumber)
		if err != nil && !errors.Is(err, invoiceninja.ErrNotFound) {
			res.Action = "error"
			res.Error = err
			return res
		}
		if errors.Is(err, invoiceninja.ErrNotFound) || existing == nil {
			res.Action = "would-create"
			return res
		}
		res.Action = "would-check-update"
		return res
	}
	_, created, changes, err := s.Ninja.UpsertProduct(ctx, prod.ItemNumber, notes, prod.ImageURL, prod.Price)
	if err != nil {
		res.Action = "error"
		res.Error = err
		return res
	}
	res.Changes = changes
	if created {
		res.Action = "created"
	} else if len(changes) > 0 {
		res.Action = "updated"
	} else {
		res.Action = "unchanged"
	}
	return res
}

func productNotes(p bunnings.Product) string {
	parts := []string{}
	if p.Title != "" {
		parts = append(parts, p.Title)
	}
	if p.Description != "" && p.Description != p.Title {
		parts = append(parts, p.Description)
	}
	parts = append(parts, fmt.Sprintf("Bunnings IN: %s", p.ItemNumber))
	return strings.Join(parts, "\n\n")
}

var digits = regexp.MustCompile(`\d{5,}`)

func inferItemNumber(productKey string) string {
	matches := digits.FindAllString(productKey, -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}
