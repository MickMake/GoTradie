package ninja

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/config"
)

func TestBunningsProductKeyRemainsStable(t *testing.T) {
	service := &Service{}
	if got := service.ProductKey(" 0123456 "); got != "BUNNINGS-0123456" {
		t.Fatalf("ProductKey() = %q; want BUNNINGS-0123456", got)
	}
}

func TestFindByBunningsINRetainsKeyAndCustomFieldLookup(t *testing.T) {
	tests := []struct {
		name       string
		keyResult  string
		listResult string
		wantID     string
		wantCalls  int
	}{
		{
			name:       "existing product key",
			keyResult:  `{"data":[{"id":"by-key","product_key":"BUNNINGS-0123456"}],"meta":{"pagination":{"total_pages":1}}}`,
			listResult: `{"data":[],"meta":{"pagination":{"total_pages":1}}}`,
			wantID:     "by-key",
			wantCalls:  1,
		},
		{
			name:       "configured Bunnings custom field fallback",
			keyResult:  `{"data":[],"meta":{"pagination":{"total_pages":1}}}`,
			listResult: `{"data":[{"id":"by-custom","product_key":"older-key","custom_value3":"0123456"}],"meta":{"pagination":{"total_pages":1}}}`,
			wantID:     "by-custom",
			wantCalls:  2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path != "/api/v1/products" {
					http.Error(w, r.URL.Path, http.StatusNotFound)
					return
				}
				if key := r.URL.Query().Get("product_key"); key != "" {
					if key != "BUNNINGS-0123456" {
						t.Errorf("product_key = %q", key)
					}
					_, _ = w.Write([]byte(test.keyResult))
					return
				}
				_, _ = w.Write([]byte(test.listResult))
			}))
			defer server.Close()

			service := newProductTestService(t, server, productTestConfig(3, 2))
			product, err := service.FindByBunningsIN(context.Background(), "0123456")
			if err != nil {
				t.Fatal(err)
			}
			if product.ID != test.wantID || calls != test.wantCalls {
				t.Fatalf("product=%#v calls=%d; want id=%q calls=%d", product, calls, test.wantID, test.wantCalls)
			}
		})
	}
}

func TestUpsertProductRetainsBunningsAndImageCustomFields(t *testing.T) {
	var created invoiceninja.CreateProductRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/products":
			_, _ = w.Write([]byte(`{"data":[],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/products":
			if err := json.NewDecoder(r.Body).Decode(&created); err != nil {
				t.Error(err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id":            "p1",
				"product_key":   created.ProductKey,
				"custom_value1": created.CustomValue1,
				"custom_value2": created.CustomValue2,
			}})
		default:
			http.Error(w, r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	service := newProductTestService(t, server, productTestConfig(1, 2))
	product, wasCreated, _, err := service.UpsertProduct(context.Background(), "0123456", "Hammer", "https://images.example/hammer.jpg", 12.50)
	if err != nil {
		t.Fatal(err)
	}
	if !wasCreated || product.ProductKey != "BUNNINGS-0123456" {
		t.Fatalf("product=%#v created=%v", product, wasCreated)
	}
	if created.ProductKey != "BUNNINGS-0123456" || created.CustomValue1 != "0123456" || created.CustomValue2 != "https://images.example/hammer.jpg" {
		t.Fatalf("create request changed Bunnings identity or fields: %#v", created)
	}
	if created.TaxName1 != "GST" || created.TaxRate1 != 10 {
		t.Fatalf("tax fields = %q %.2f; want GST 10", created.TaxName1, created.TaxRate1)
	}
}

func TestProductCSVRetainsConfiguredImageURLField(t *testing.T) {
	var update invoiceninja.UpdateProductRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/products" && r.URL.Query().Get("status") == "active":
			_, _ = w.Write([]byte(`{"data":[{"id":"p1","product_key":"BUNNINGS-0123456","notes":"Hammer","price":12.5,"quantity":1,"custom_value2":"https://images.example/old.jpg"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/products/p1":
			_, _ = w.Write([]byte(`{"data":{"id":"p1","product_key":"BUNNINGS-0123456","notes":"Hammer","price":12.5,"quantity":1,"custom_value1":"0123456","custom_value2":"https://images.example/old.jpg"}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/products/p1":
			if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
				t.Error(err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": update})
		default:
			http.Error(w, r.Method+" "+r.URL.String(), http.StatusNotFound)
		}
	}))
	defer server.Close()

	service := newProductTestService(t, server, productTestConfig(1, 2))
	var exported bytes.Buffer
	if err := service.ExportProductsCSV(context.Background(), &exported); err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(strings.NewReader(exported.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[1][6] != "https://images.example/old.jpg" {
		t.Fatalf("exported records = %#v", records)
	}

	input := strings.Join([]string{
		strings.Join(productCSVHeader, ","),
		"p1,BUNNINGS-0123456,Hammer,12.5,1,,https://images.example/new.jpg",
	}, "\n")
	results, err := service.ImportProductsCSV(context.Background(), strings.NewReader(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "updated" {
		t.Fatalf("import results = %#v", results)
	}
	if update.CustomValue1 != "0123456" || update.CustomValue2 != "https://images.example/new.jpg" {
		t.Fatalf("update custom fields = %#v", update)
	}
}

func productTestConfig(bunningsField, imageField int) config.Config {
	return config.Config{
		InvoiceNinja: config.InvoiceNinjaConfig{Token: "token"},
		Tax:          config.TaxConfig{Name: "GST", Rate: 10},
		ProductSync: config.ProductSyncConfig{CustomFields: config.ProductCustomFields{
			BunningsIN: bunningsField,
			ImageURL:   imageField,
		}},
	}
}

func newProductTestService(t *testing.T, server *httptest.Server, cfg config.Config) *Service {
	t.Helper()
	client, err := invoiceninja.New("token", invoiceninja.WithBaseURL(server.URL), invoiceninja.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	return &Service{client: client, cfg: cfg}
}
