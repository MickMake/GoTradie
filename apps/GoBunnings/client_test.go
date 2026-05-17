package gobunnings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPricingCatalogPrices(t *testing.T) {
	var gotVersion string
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get("x-version-api")
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/pricing/catalog/prices" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var req CatalogPriceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Context.Country != CountryAU || req.Context.Location != "7040" {
			t.Fatalf("bad request: %+v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"prices":[{"itemNumber":"0123456","unitPrice":1.56,"priceId":"S1:AU:test"}]}`))
	}))
	defer srv.Close()

	c, err := New(EnvSandbox, TokenSourceFunc(func(context.Context) (string, error) { return "token", nil }), WithBaseURLs(BaseURLs{Pricing: srv.URL + "/pricing"}), WithoutRetry())
	if err != nil {
		t.Fatal(err)
	}

	out, err := c.Pricing.CatalogPrices(context.Background(), CatalogPriceRequest{Context: PriceContext{Country: CountryAU, Location: "7040"}, Items: []PriceItem{{ItemNumber: "0123456"}}}, QueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if gotVersion != pricingAPIVersion {
		t.Fatalf("version = %q", gotVersion)
	}
	if gotAuth != "Bearer token" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if len(out.Prices) != 1 || out.Prices[0].UnitPrice != 1.56 {
		t.Fatalf("bad response: %+v", out)
	}
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"title":"Bad Request","status":400,"detail":"Nope"}`))
	}))
	defer srv.Close()

	c, err := New(EnvSandbox, TokenSourceFunc(func(context.Context) (string, error) { return "token", nil }), WithBaseURLs(BaseURLs{Pricing: srv.URL}), WithoutRetry())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Pricing.Discovery(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Problem == nil || apiErr.Problem.Detail != "Nope" {
		t.Fatalf("bad problem: %+v", apiErr.Problem)
	}
}

func TestItemRawIsPopulatedOnUnmarshal(t *testing.T) {
	payload := []byte(`{
		"itemNumber":"0123456",
		"description":{"sectionDescription":"TIMBER","productDescription":"MERBAU DECKING"},
		"itemType":"RETAIL",
		"enrichedItem":{"name":"Merbau Decking","picture":{"url":"https://example.test/image.jpg"}}
	}`)

	var item Item
	if err := json.Unmarshal(payload, &item); err != nil {
		t.Fatal(err)
	}

	if item.ItemNumber != "0123456" {
		t.Fatalf("ItemNumber = %q", item.ItemNumber)
	}
	if item.Description.ProductDescription != "MERBAU DECKING" {
		t.Fatalf("ProductDescription = %q", item.Description.ProductDescription)
	}
	if item.Raw == nil {
		t.Fatal("Raw was nil")
	}
	if got := item.Raw["itemNumber"]; got != "0123456" {
		t.Fatalf("Raw[itemNumber] = %#v", got)
	}
	enriched, ok := item.Raw["enrichedItem"].(map[string]any)
	if !ok {
		t.Fatalf("Raw[enrichedItem] = %#v", item.Raw["enrichedItem"])
	}
	picture, ok := enriched["picture"].(map[string]any)
	if !ok {
		t.Fatalf("Raw[enrichedItem][picture] = %#v", enriched["picture"])
	}
	if got := picture["url"]; got != "https://example.test/image.jpg" {
		t.Fatalf("picture url = %#v", got)
	}
}

func TestItemDetailPopulatesRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/item/detail/AU/0123456" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"itemNumber":"0123456","description":{"productDescription":"MERBAU DECKING"},"web":{"imageUrl":"https://example.test/detail.jpg"}}`))
	}))
	defer srv.Close()

	c, err := New(EnvSandbox, TokenSourceFunc(func(context.Context) (string, error) { return "token", nil }), WithBaseURLs(BaseURLs{Item: srv.URL + "/item"}), WithoutRetry())
	if err != nil {
		t.Fatal(err)
	}

	item, err := c.Item.Detail(context.Background(), CountryAU, "0123456", QueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if item.Raw == nil {
		t.Fatal("Raw was nil")
	}
	web, ok := item.Raw["web"].(map[string]any)
	if !ok {
		t.Fatalf("Raw[web] = %#v", item.Raw["web"])
	}
	if got := web["imageUrl"]; got != "https://example.test/detail.jpg" {
		t.Fatalf("imageUrl = %#v", got)
	}
}

func TestItemImageURLFromEnrichedPicture(t *testing.T) {
	payload := []byte(`{
		"itemNumber":"0123456",
		"description":{"productDescription":"MERBAU DECKING"},
		"enrichedItem":{
			"name":"Merbau Decking",
			"picture":{"primaryAssetURL":"https://media.example.test/merbau.jpg"},
			"otherImages":[{"primaryAssetURL":"https://media.example.test/other.jpg"}]
		}
	}`)

	var item Item
	if err := json.Unmarshal(payload, &item); err != nil {
		t.Fatal(err)
	}

	if item.ImageURL != "https://media.example.test/merbau.jpg" {
		t.Fatalf("ImageURL = %q", item.ImageURL)
	}
	if item.Raw == nil {
		t.Fatal("Raw was nil")
	}
}

func TestItemImageURLFallsBackToOtherImages(t *testing.T) {
	payload := []byte(`{
		"itemNumber":"0123456",
		"enrichedItem":{
			"otherImages":[{"primaryAssetURL":"https://media.example.test/fallback.jpg"}]
		}
	}`)

	var item Item
	if err := json.Unmarshal(payload, &item); err != nil {
		t.Fatal(err)
	}

	if item.ImageURL != "https://media.example.test/fallback.jpg" {
		t.Fatalf("ImageURL = %q", item.ImageURL)
	}
}

func TestItemWithoutImageLeavesImageURLEmpty(t *testing.T) {
	payload := []byte(`{
		"itemNumber":"0123456",
		"description":{"productDescription":"MERBAU DECKING"},
		"enrichedItem":{"name":"Merbau Decking"}
	}`)

	var item Item
	if err := json.Unmarshal(payload, &item); err != nil {
		t.Fatal(err)
	}

	if item.ImageURL != "" {
		t.Fatalf("ImageURL = %q", item.ImageURL)
	}
	if item.Raw == nil {
		t.Fatal("Raw was nil")
	}
}

func TestProblemDetailsCapturesExtensionFields(t *testing.T) {
	payload := []byte(`{
		"type":"https://example.test/problem",
		"title":"Bad Request",
		"status":400,
		"detail":"Nope",
		"instance":"/request/123",
		"errors":{"itemNumber":["required"]},
		"traceId":"abc-123",
		"retryable":false,
		"nested":{"code":"BUNNINGS_TEST"}
	}`)

	var problem ProblemDetails
	if err := json.Unmarshal(payload, &problem); err != nil {
		t.Fatal(err)
	}

	if problem.Type != "https://example.test/problem" || problem.Title != "Bad Request" || problem.Status != 400 || problem.Detail != "Nope" || problem.Instance != "/request/123" {
		t.Fatalf("known fields not populated correctly: %+v", problem)
	}
	if got := problem.Errors["itemNumber"][0]; got != "required" {
		t.Fatalf("errors not populated: %+v", problem.Errors)
	}
	if _, ok := problem.Extra["type"]; ok {
		t.Fatal("known field type leaked into Extra")
	}
	if string(problem.Extra["traceId"]) != `"abc-123"` {
		t.Fatalf("traceId extra = %s", problem.Extra["traceId"])
	}
	if string(problem.Extra["retryable"]) != `false` {
		t.Fatalf("retryable extra = %s", problem.Extra["retryable"])
	}
	if string(problem.Extra["nested"]) != `{"code":"BUNNINGS_TEST"}` {
		t.Fatalf("nested extra = %s", problem.Extra["nested"])
	}
}

func TestAPIErrorProblemDetailsCapturesExtensionFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"title":"Too Many Requests","status":429,"detail":"Back off","traceId":"trace-429"}`))
	}))
	defer srv.Close()

	c, err := New(EnvSandbox, TokenSourceFunc(func(context.Context) (string, error) { return "token", nil }), WithBaseURLs(BaseURLs{Pricing: srv.URL}), WithoutRetry())
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Pricing.Discovery(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Problem == nil {
		t.Fatal("Problem was nil")
	}
	if string(apiErr.Problem.Extra["traceId"]) != `"trace-429"` {
		t.Fatalf("traceId extra = %s", apiErr.Problem.Extra["traceId"])
	}
}

func TestBarcodeAdditionalDescriptionUsesBunningsSchemaCasing(t *testing.T) {
	payload := []byte(`{"number":"9341136008791","level":"Consumer","AdditionalDescription":"Botanical name"}`)

	var barcode Barcode
	if err := json.Unmarshal(payload, &barcode); err != nil {
		t.Fatal(err)
	}

	if barcode.AdditionalDescription != "Botanical name" {
		t.Fatalf("AdditionalDescription = %q", barcode.AdditionalDescription)
	}
}
