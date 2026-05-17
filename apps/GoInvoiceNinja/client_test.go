package goinvoiceninja

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientListInvoicesBuildsExpectedRequest(t *testing.T) {
	var gotPath, gotQuery, gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotToken = r.Header.Get("X-API-TOKEN")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"id": "inv1", "number": "INV-1", "balance": 123.45}},
			"meta": map[string]any{"total": 1},
		})
	}))
	defer srv.Close()

	c, err := New("secret", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	result, err := c.Invoices.List(context.Background(), InvoiceQuery{
		ListOptions: ListOptions{PerPage: 50, Include: []string{"client"}},
		Payable:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 1 || result.Data[0].Number != "INV-1" {
		t.Fatalf("unexpected invoices: %#v", result.Data)
	}
	if gotPath != "/api/v1/invoices" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotToken != "secret" {
		t.Fatalf("token header = %q", gotToken)
	}
	if gotQuery != "include=client&payable=true&per_page=50" {
		t.Fatalf("query = %q", gotQuery)
	}
}

func TestProductFindByKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/products" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("product_key") != "LABOUR" {
			t.Fatalf("product_key = %q", r.URL.Query().Get("product_key"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"id": "p1", "product_key": "LABOUR", "price": 95}},
		})
	}))
	defer srv.Close()

	c, err := New("secret", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	product, err := c.Products.FindByKey(context.Background(), "LABOUR")
	if err != nil {
		t.Fatal(err)
	}
	if product.ID != "p1" || product.ProductKey != "LABOUR" {
		t.Fatalf("product = %#v", product)
	}
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "validation failed"})
	}))
	defer srv.Close()

	c, err := New("secret", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Clients.Get(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity || apiErr.Message != "validation failed" {
		t.Fatalf("apiErr = %#v", apiErr)
	}
}

func TestProductCreateWithBunningsINCustomField(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody CreateProductRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":            "p-bunnings-1",
				"product_key":   gotBody.ProductKey,
				"notes":         gotBody.Notes,
				"price":         gotBody.Price,
				"custom_value1": gotBody.CustomValue1,
			},
		})
	}))
	defer srv.Close()

	c, err := New("secret", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	product, err := c.Products.Create(context.Background(), CreateProductRequest{
		ProductKey:   "BUN-0123456",
		Notes:        "Merbau decking oil",
		Price:        42.50,
		CustomValue1: "0123456",
	})
	if err != nil {
		t.Fatal(err)
	}

	if gotMethod != http.MethodPost || gotPath != "/api/v1/products" {
		t.Fatalf("%s %s", gotMethod, gotPath)
	}
	if gotBody.CustomValue1 != "0123456" {
		t.Fatalf("custom_value1 request = %q", gotBody.CustomValue1)
	}
	if product.CustomValue1 != "0123456" {
		t.Fatalf("custom_value1 response = %q", product.CustomValue1)
	}
}

func TestProductUpdateWithBunningsINCustomField(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody UpdateProductRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":            "p-bunnings-1",
				"product_key":   gotBody.ProductKey,
				"custom_value1": gotBody.CustomValue1,
			},
		})
	}))
	defer srv.Close()

	c, err := New("secret", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	product, err := c.Products.Update(context.Background(), "p-bunnings-1", UpdateProductRequest{
		ProductKey:   "BUN-7654321",
		CustomValue1: "7654321",
	})
	if err != nil {
		t.Fatal(err)
	}

	if gotMethod != http.MethodPut || gotPath != "/api/v1/products/p-bunnings-1" {
		t.Fatalf("%s %s", gotMethod, gotPath)
	}
	if gotBody.CustomValue1 != "7654321" {
		t.Fatalf("custom_value1 request = %q", gotBody.CustomValue1)
	}
	if product.CustomValue1 != "7654321" {
		t.Fatalf("custom_value1 response = %q", product.CustomValue1)
	}
}

func TestProductListAndGetPopulateCustomFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/products":
			if r.URL.Query().Get("custom_value1") != "0123456" {
				t.Fatalf("custom_value1 query = %q", r.URL.Query().Get("custom_value1"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "p1", "product_key": "BUN-0123456", "custom_value1": "0123456", "custom_value2": "Bunnings"}},
			})
		case "/api/v1/products/p1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"id": "p1", "product_key": "BUN-0123456", "custom_value1": "0123456", "custom_value3": "Trade"},
			})
		default:
			t.Fatalf("unexpected path = %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	c, err := New("secret", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	listed, err := c.Products.List(context.Background(), ProductQuery{CustomValue1: "0123456"})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Data) != 1 || listed.Data[0].CustomValue1 != "0123456" || listed.Data[0].CustomValue2 != "Bunnings" {
		t.Fatalf("listed products = %#v", listed.Data)
	}

	got, err := c.Products.Get(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.CustomValue1 != "0123456" || got.CustomValue3 != "Trade" {
		t.Fatalf("got product = %#v", got)
	}
}

func TestProductFindByBunningsINUsesCustomValue1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/products" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("custom_value1") != "0123456" {
			t.Fatalf("custom_value1 = %q", r.URL.Query().Get("custom_value1"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"id": "p1", "product_key": "BUN-0123456", "custom_value1": "0123456"}},
		})
	}))
	defer srv.Close()

	c, err := New("secret", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	product, err := c.Products.FindByBunningsIN(context.Background(), "0123456")
	if err != nil {
		t.Fatal(err)
	}
	if product.ID != "p1" || product.CustomValue1 != "0123456" {
		t.Fatalf("product = %#v", product)
	}
}

func TestProductCreateUpdateAndReadMaxQuantityAndProductImage(t *testing.T) {
	var createBody CreateProductRequest
	var updateBody UpdateProductRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/products":
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("decode create: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id":                           "p1",
				"product_key":                  createBody.ProductKey,
				"quantity":                     createBody.Quantity,
				"max_quantity":                 createBody.MaxQuantity,
				"product_image":                createBody.ProductImage,
				"in_stock_quantity":            createBody.InStockQuantity,
				"stock_notification":           createBody.StockNotification,
				"stock_notification_threshold": createBody.StockNotificationThreshold,
			}})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/products/p1":
			if err := json.NewDecoder(r.Body).Decode(&updateBody); err != nil {
				t.Fatalf("decode update: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id":            "p1",
				"max_quantity":  updateBody.MaxQuantity,
				"product_image": updateBody.ProductImage,
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/products/p1":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id":                           "p1",
				"product_key":                  "BUN-0123456",
				"quantity":                     1.0,
				"max_quantity":                 10,
				"product_image":                "https://example.test/image.jpg",
				"in_stock_quantity":            25,
				"stock_notification":           true,
				"stock_notification_threshold": 3,
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/products":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
				"id":            "p1",
				"max_quantity":  10,
				"product_image": "https://example.test/image.jpg",
			}}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c, err := New("secret", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	created, err := c.Products.Create(context.Background(), CreateProductRequest{
		ProductKey:                 "BUN-0123456",
		Quantity:                   1,
		MaxQuantity:                10,
		ProductImage:               "https://example.test/image.jpg",
		InStockQuantity:            25,
		StockNotification:          true,
		StockNotificationThreshold: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if createBody.MaxQuantity != 10 || createBody.ProductImage != "https://example.test/image.jpg" {
		t.Fatalf("create body = %#v", createBody)
	}
	if created.MaxQuantity != 10 || created.ProductImage != "https://example.test/image.jpg" || !created.StockNotification {
		t.Fatalf("created = %#v", created)
	}

	updated, err := c.Products.Update(context.Background(), "p1", UpdateProductRequest{MaxQuantity: 12, ProductImage: "https://example.test/new.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if updateBody.MaxQuantity != 12 || updated.ProductImage != "https://example.test/new.jpg" {
		t.Fatalf("updated = %#v body=%#v", updated, updateBody)
	}

	got, err := c.Products.Get(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.MaxQuantity != 10 || got.ProductImage != "https://example.test/image.jpg" || got.InStockQuantity != 25 {
		t.Fatalf("got = %#v", got)
	}

	listed, err := c.Products.List(context.Background(), ProductQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Data) != 1 || listed.Data[0].MaxQuantity != 10 || listed.Data[0].ProductImage == "" {
		t.Fatalf("listed = %#v", listed.Data)
	}
}

func TestProductUploadDocument(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/products/p1/upload" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("multipart reader: %v", err)
		}
		var sawFile bool
		for {
			part, err := mr.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("next part: %v", err)
			}
			if part.FormName() == DefaultDocumentFormField && part.FileName() == "image.jpg" {
				b, _ := io.ReadAll(part)
				if string(b) != "fake image" {
					t.Fatalf("upload body = %q", string(b))
				}
				sawFile = true
			}
		}
		if !sawFile {
			t.Fatal("did not see uploaded document part")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"id":        "p1",
			"documents": []map[string]any{{"id": "d1", "name": "image.jpg", "type": "image/jpeg"}},
		}})
	}))
	defer srv.Close()

	c, err := New("secret", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	product, err := c.Products.UploadDocument(context.Background(), "p1", "image.jpg", strings.NewReader("fake image"))
	if err != nil {
		t.Fatal(err)
	}
	if len(product.Documents) != 1 || product.Documents[0].Name != "image.jpg" {
		t.Fatalf("product = %#v", product)
	}
}

func TestAPIErrorMessageOnlyResponse(t *testing.T) {
	err := parseAPIError(http.StatusBadRequest, []byte(`{"message":"bad request"}`))
	got := err.Error()
	if !strings.Contains(got, "status=400") || !strings.Contains(got, "bad request") {
		t.Fatalf("error string = %q", got)
	}
}

func TestAPIErrorErrorsOnlyResponse(t *testing.T) {
	err := parseAPIError(http.StatusUnprocessableEntity, []byte(`{"errors":{"product_key":["The product key has already been taken."]}}`))
	got := err.Error()
	if !strings.Contains(got, "status=422") || !strings.Contains(got, "product_key=The product key has already been taken.") {
		t.Fatalf("error string = %q", got)
	}
}

func TestAPIErrorMessagePlusErrorsResponse(t *testing.T) {
	err := parseAPIError(http.StatusUnprocessableEntity, []byte(`{"message":"validation failed","errors":{"custom_value1":["The custom value is required."],"price":["The price must be numeric."]}}`))
	got := err.Error()
	if !strings.Contains(got, "status=422") || !strings.Contains(got, "validation failed") || !strings.Contains(got, "custom_value1=The custom value is required.") || !strings.Contains(got, "price=The price must be numeric.") {
		t.Fatalf("error string = %q", got)
	}
}

func TestAPIErrorEmptyBodyResponse(t *testing.T) {
	err := parseAPIError(http.StatusInternalServerError, nil)
	got := err.Error()
	if !strings.Contains(got, "status=500") || strings.Contains(got, "validation:") {
		t.Fatalf("error string = %q", got)
	}
}

func TestProductListAllFetchesEveryPageAndPreservesQuery(t *testing.T) {
	var pages []string
	var statuses []string
	var perPages []string
	var customValues []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/products" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		q := r.URL.Query()
		pages = append(pages, q.Get("page"))
		statuses = append(statuses, q.Get("status"))
		perPages = append(perPages, q.Get("per_page"))
		customValues = append(customValues, q.Get("custom_value1"))

		switch q.Get("page") {
		case "1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "p1", "product_key": "BUN-1", "custom_value1": "BUN"},
					{"id": "p2", "product_key": "BUN-2", "custom_value1": "BUN"},
				},
				"meta": map[string]any{"current_page": 1, "last_page": 3, "per_page": 2, "total": 5},
			})
		case "2":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "p3", "product_key": "BUN-3", "custom_value1": "BUN"},
					{"id": "p4", "product_key": "BUN-4", "custom_value1": "BUN"},
				},
				"meta": map[string]any{"current_page": 2, "last_page": 3, "per_page": 2, "total": 5},
			})
		case "3":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "p5", "product_key": "BUN-5", "custom_value1": "BUN"}},
				"meta": map[string]any{"current_page": 3, "last_page": 3, "per_page": 2, "total": 5},
			})
		default:
			t.Fatalf("unexpected page %q", q.Get("page"))
		}
	}))
	defer srv.Close()

	c, err := New("secret", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	products, err := c.Products.ListAll(context.Background(), ProductQuery{
		ListOptions:  ListOptions{PerPage: 2, Status: "active"},
		CustomValue1: "BUN",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 5 {
		t.Fatalf("len(products) = %d", len(products))
	}
	if strings.Join(pages, ",") != "1,2,3" {
		t.Fatalf("pages = %#v", pages)
	}
	for i := range statuses {
		if statuses[i] != "active" || perPages[i] != "2" || customValues[i] != "BUN" {
			t.Fatalf("query values status=%#v per_page=%#v custom=%#v", statuses, perPages, customValues)
		}
	}
}

func TestClientListAllFetchesEveryPage(t *testing.T) {
	var pages []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/clients" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		q := r.URL.Query()
		pages = append(pages, q.Get("page"))
		if q.Get("per_page") != "2" {
			t.Fatalf("per_page = %q", q.Get("per_page"))
		}
		if q.Get("status") != "active" {
			t.Fatalf("status = %q", q.Get("status"))
		}

		switch q.Get("page") {
		case "1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "c1", "name": "One"}, {"id": "c2", "name": "Two"}},
				"meta": map[string]any{"current_page": 1, "last_page": 2, "per_page": 2, "total": 3},
			})
		case "2":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "c3", "name": "Three"}},
				"meta": map[string]any{"current_page": 2, "last_page": 2, "per_page": 2, "total": 3},
			})
		default:
			t.Fatalf("unexpected page %q", q.Get("page"))
		}
	}))
	defer srv.Close()

	c, err := New("secret", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}

	clients, err := c.Clients.ListAll(context.Background(), ClientQuery{ListOptions: ListOptions{PerPage: 2, Status: "active"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 3 {
		t.Fatalf("len(clients) = %d", len(clients))
	}
	if strings.Join(pages, ",") != "1,2" {
		t.Fatalf("pages = %#v", pages)
	}
}

func TestListAllDefaultsPerPageWhenUnset(t *testing.T) {
	var gotPerPage string
	var gotPage string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPerPage = r.URL.Query().Get("per_page")
		gotPage = r.URL.Query().Get("page")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"id": "p1", "product_key": "BUN-1"}},
			"meta": map[string]any{"current_page": 1, "last_page": 1},
		})
	}))
	defer srv.Close()

	c, err := New("secret", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	products, err := c.Products.ListAll(context.Background(), ProductQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 1 {
		t.Fatalf("len(products) = %d", len(products))
	}
	if gotPerPage != "100" || gotPage != "1" {
		t.Fatalf("per_page=%q page=%q", gotPerPage, gotPage)
	}
}
