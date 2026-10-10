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

func TestProductCSVUsesNativeProductImage(t *testing.T) {
	var update invoiceninja.UpdateProductRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/products" && r.URL.Query().Get("status") == "active":
			_, _ = w.Write([]byte(`{"data":[{"id":"p1","product_key":"0123456","notes":"Hammer","price":12.5,"quantity":1,"product_image":"https://images.example/old.jpg"}],"meta":{"pagination":{"total_pages":1}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/products/p1":
			_, _ = w.Write([]byte(`{"data":{"id":"p1","product_key":"0123456","notes":"Hammer","price":12.5,"quantity":1,"product_image":"https://images.example/old.jpg","custom_value1":"Castle Hill","custom_value2":"false"}}`))
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
		"p1,0123456,Hammer,12.5,1,,https://images.example/new.jpg",
	}, "\n")
	results, err := service.ImportProductsCSV(context.Background(), strings.NewReader(input), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "updated" {
		t.Fatalf("import results = %#v", results)
	}
	if update.ProductImage != "https://images.example/new.jpg" || update.CustomValue1 != "" || update.CustomValue2 != "" {
		t.Fatalf("update request = %#v", update)
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
