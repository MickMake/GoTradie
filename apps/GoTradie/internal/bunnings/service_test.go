package bunnings

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gobunnings "github.com/MickMake/GoBunnings"
	"github.com/MickMake/GoTradie/internal/config"
)

func TestNewUsesHierarchicalBunningsSettings(t *testing.T) {
	service, err := New(config.Config{Providers: map[string]config.ProviderConfig{
		"bunnings": {
			Environment:  "test",
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			Scopes:       []string{"item.read", "pricing.read"},
			Country:      "NZ",
			Location:     "9473",
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if service.country != "NZ" || service.location != "9473" {
		t.Fatalf("country=%q location=%q", service.country, service.location)
	}
	if service.client.UserAgent != "GoTradie/v0.5.7" {
		t.Fatalf("user agent = %q", service.client.UserAgent)
	}
	if got := service.client.BaseURLs.Item; got != "https://item.stg.api.bunnings.com.au/item" {
		t.Fatalf("test environment Item URL = %q", got)
	}
}

func TestGetProductReturnsConfiguredPricingFailure(t *testing.T) {
	pricingCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/item/detail/AU/0123456":
			_, _ = w.Write([]byte(`{"itemNumber":"0123456","description":{"productDescription":"Hammer"}}`))
		case "/pricing/catalog/prices":
			pricingCalls++
			http.Error(w, "pricing unavailable", http.StatusNotFound)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := gobunnings.New(
		gobunnings.EnvSandbox,
		gobunnings.TokenSourceFunc(func(context.Context) (string, error) { return "token", nil }),
		gobunnings.WithBaseURLs(gobunnings.BaseURLs{Item: server.URL + "/item", Pricing: server.URL + "/pricing"}),
		gobunnings.WithoutRetry(),
	)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{client: client, country: gobunnings.CountryAU, location: "7040"}

	_, err = service.GetProduct(context.Background(), "0123456")
	if err == nil || !strings.Contains(err.Error(), `Bunnings pricing for Product "0123456"`) {
		t.Fatalf("error = %v", err)
	}
	var pricingError *PricingError
	if !errors.As(err, &pricingError) {
		t.Fatalf("error type = %T; want *PricingError", err)
	}
	if pricingCalls != 1 {
		t.Fatalf("pricing calls = %d; want 1", pricingCalls)
	}
}

func TestGetProductSkipsPricingWhenLocationIsNotConfigured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/item/detail/AU/0123456" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"itemNumber":"0123456","description":{"productDescription":"Hammer"}}`))
	}))
	defer server.Close()

	client, err := gobunnings.New(
		gobunnings.EnvSandbox,
		gobunnings.TokenSourceFunc(func(context.Context) (string, error) { return "token", nil }),
		gobunnings.WithBaseURLs(gobunnings.BaseURLs{Item: server.URL + "/item", Pricing: server.URL + "/pricing"}),
		gobunnings.WithoutRetry(),
	)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{client: client, country: gobunnings.CountryAU}

	product, err := service.GetProduct(context.Background(), "0123456")
	if err != nil {
		t.Fatal(err)
	}
	if product.ItemNumber != "0123456" || product.Price != 0 {
		t.Fatalf("product = %#v", product)
	}
}
