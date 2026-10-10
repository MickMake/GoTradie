package syncer

import (
	"context"
	"testing"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/bunnings"
	"github.com/MickMake/GoTradie/internal/config"
	"github.com/MickMake/GoTradie/internal/productsync"
)

func TestAddByINUsesV057ProductIdentity(t *testing.T) {
	bunningsClient := &fakeBunnings{product: bunnings.Product{
		ItemNumber: "0123456", Title: "Hammer", ImageURL: "https://images.example/hammer.jpg", Price: 12.50,
	}}
	ninjaClient := &fakeNinja{vendors: []invoiceninja.Vendor{{Entity: invoiceninja.Entity{ID: "vendor-1"}, Name: "Bunnings"}}}
	productService := &productsync.Service{
		Config: config.Config{
			Tax:       config.TaxConfig{Name: "GST", Rate: 10},
			Providers: map[string]config.ProviderConfig{"bunnings": {Name: "Bunnings", Type: "api"}},
		},
		Bunnings: bunningsClient, Ninja: ninjaClient, Commit: true,
	}
	service := Service{Bunnings: bunningsClient, ProductSync: productService}

	result := service.AddByIN(context.Background(), "0123456")

	if result.Error != nil || result.Action != "created" || result.ProductKey != "0123456" {
		t.Fatalf("result = %#v", result)
	}
	if ninjaClient.created.ProductKey != "0123456" || ninjaClient.created.VendorID != "" || ninjaClient.created.CustomValue1 != "Bunnings" || ninjaClient.created.ProductImage != "https://images.example/hammer.jpg" {
		t.Fatalf("create request = %#v", ninjaClient.created)
	}
}

type fakeBunnings struct {
	product bunnings.Product
}

func (f *fakeBunnings) GetProduct(context.Context, string) (bunnings.Product, error) {
	return f.product, nil
}

func (f *fakeBunnings) Search(context.Context, string, int) ([]bunnings.Product, error) {
	return nil, nil
}

func (f *fakeBunnings) Hydrate(context.Context, bunnings.Product) (bunnings.Product, error) {
	return f.product, nil
}

func (f *fakeBunnings) WithWeb(bool) *bunnings.Service { return nil }

type fakeNinja struct {
	products []invoiceninja.Product
	vendors  []invoiceninja.Vendor
	created  invoiceninja.CreateProductRequest
}

func (f *fakeNinja) ListAllProducts(context.Context) ([]invoiceninja.Product, error) {
	return f.products, nil
}

func (f *fakeNinja) ListAllVendors(context.Context) ([]invoiceninja.Vendor, error) {
	return f.vendors, nil
}

func (f *fakeNinja) CreateCatalogProduct(_ context.Context, request invoiceninja.CreateProductRequest) (invoiceninja.Product, error) {
	f.created = request
	return invoiceninja.Product{ProductKey: request.ProductKey, VendorID: request.VendorID}, nil
}

func (f *fakeNinja) UpdateCatalogProduct(context.Context, string, invoiceninja.SparseProductUpdateRequest) (invoiceninja.Product, error) {
	return invoiceninja.Product{}, nil
}
