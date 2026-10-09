package syncer

import (
	"context"
	"testing"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/bunnings"
)

func TestAddByINPreservesBunningsSyncInputs(t *testing.T) {
	bunningsClient := &fakeBunnings{product: bunnings.Product{
		ItemNumber: "0123456",
		Title:      "Hammer",
		ImageURL:   "https://images.example/hammer.jpg",
		Price:      12.50,
	}}
	ninjaClient := &fakeNinja{}
	service := Service{Bunnings: bunningsClient, Ninja: ninjaClient}

	result := service.AddByIN(context.Background(), "0123456")

	if result.Error != nil || result.Action != "created" || result.ProductKey != "BUNNINGS-0123456" {
		t.Fatalf("result = %#v", result)
	}
	if ninjaClient.itemNumber != "0123456" || ninjaClient.imageURL != "https://images.example/hammer.jpg" || ninjaClient.price != 12.50 {
		t.Fatalf("upsert inputs changed: %#v", ninjaClient)
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
	itemNumber string
	imageURL   string
	price      float64
}

func (f *fakeNinja) ListProducts(context.Context) ([]invoiceninja.Product, error) { return nil, nil }

func (f *fakeNinja) FindByBunningsIN(context.Context, string) (*invoiceninja.Product, error) {
	return nil, invoiceninja.ErrNotFound
}

func (f *fakeNinja) UpsertProduct(_ context.Context, itemNumber, _ string, imageURL string, price float64) (invoiceninja.Product, bool, []string, error) {
	f.itemNumber = itemNumber
	f.imageURL = imageURL
	f.price = price
	return invoiceninja.Product{ProductKey: f.ProductKey(itemNumber)}, true, []string{"created"}, nil
}

func (f *fakeNinja) ProductKey(itemNumber string) string { return "BUNNINGS-" + itemNumber }

func (f *fakeNinja) CustomValue(invoiceninja.Product, int) string { return "" }
