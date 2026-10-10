package syncer

import (
	"context"
	"fmt"

	"github.com/MickMake/GoTradie/internal/bunnings"
	"github.com/MickMake/GoTradie/internal/productsync"
)

type Bunnings interface {
	Search(context.Context, string, int) ([]bunnings.Product, error)
	WithWeb(bool) *bunnings.Service
}

type Service struct {
	Bunnings    Bunnings
	ProductSync *productsync.Service
	DryRun      bool
}

type Result = productsync.Result

func (s Service) SyncExisting(ctx context.Context) ([]Result, error) {
	s.ProductSync.Commit = !s.DryRun
	return s.ProductSync.Refresh(ctx)
}

func (s Service) AddByIN(ctx context.Context, itemNumber string) Result {
	s.ProductSync.Commit = !s.DryRun
	results, err := s.ProductSync.RefreshBunningsKeys(ctx, []string{itemNumber})
	if err != nil {
		return Result{Provider: "Bunnings", ItemNumber: itemNumber, ProductKey: itemNumber, Action: "error", Error: err}
	}
	if len(results) == 0 {
		return Result{Provider: "Bunnings", ItemNumber: itemNumber, ProductKey: itemNumber, Action: "error", Error: fmt.Errorf("Product Sync returned no result")}
	}
	return results[0]
}

func (s Service) Search(ctx context.Context, query string, limit int) ([]bunnings.Product, error) {
	return s.Bunnings.Search(ctx, query, limit)
}

func (s Service) AddProducts(ctx context.Context, products []bunnings.Product) []Result {
	keys := make([]string, 0, len(products))
	for _, product := range products {
		keys = append(keys, product.ItemNumber)
	}
	s.ProductSync.Commit = !s.DryRun
	results, err := s.ProductSync.RefreshBunningsKeys(ctx, keys)
	if err != nil {
		return []Result{{Provider: "Bunnings", Action: "error", Error: err}}
	}
	return results
}
