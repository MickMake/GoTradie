package productsync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	gobunnings "github.com/MickMake/GoBunnings"
	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/bunnings"
	"github.com/MickMake/GoTradie/internal/config"
	"github.com/xuri/excelize/v2"
)

func TestLegacyBunningsProductMigratesInPlace(t *testing.T) {
	ninja := &fakeNinja{
		products: []invoiceninja.Product{{
			Entity: invoiceninja.Entity{ID: "legacy"}, ProductKey: "BUNNINGS-0123456",
			CustomValue1: "0123456", CustomValue2: "https://old.example/image.jpg",
			CustomValue3: "user pack text",
		}},
		vendors: []invoiceninja.Vendor{vendor("vendor-bunnings", "Bunnings")},
	}
	provider := &fakeBunnings{products: map[string]bunnings.Product{
		"0123456": {ItemNumber: "0123456", Title: "Hammer", Unit: "Each", Price: 12.5},
	}}
	service := bunningsService(ninja, provider, true)
	service.Config.Providers["bunnings"] = config.ProviderConfig{
		Name: "Bunnings", Type: "api", Location: "Castle Hill",
	}

	results, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "updated" || results[0].Error != nil {
		t.Fatalf("results = %#v", results)
	}
	if len(ninja.updates) != 1 || len(ninja.creates) != 0 {
		t.Fatalf("updates=%#v creates=%#v", ninja.updates, ninja.creates)
	}
	request := ninja.updates[0].request.Values
	if ninja.updates[0].id != "legacy" || request.VendorID != "vendor-bunnings" || request.ProductKey != "0123456" {
		t.Fatalf("identity migration = %#v", ninja.updates[0])
	}
	if request.ProductImage != "https://old.example/image.jpg" {
		t.Fatalf("native product_image = %q", request.ProductImage)
	}
	if request.CustomValue1 != "Castle Hill" || request.CustomValue2 != "false" || request.CustomValue4 != "2026-10-10" {
		t.Fatalf("fixed custom fields = %#v", request)
	}
	if contains(results[0].Changes, "custom_value3") {
		t.Fatalf("free-form Supply Unit was overwritten: %#v", results[0].Changes)
	}
}

func TestLegacyCustomIdentifierAndAliasVendorResolveToCanonicalVendor(t *testing.T) {
	ninja := &fakeNinja{
		products: []invoiceninja.Product{{
			Entity: invoiceninja.Entity{ID: "legacy-custom"}, VendorID: "alias-vendor",
			ProductKey: "old-key", CustomValue1: "7654321",
		}},
		vendors: []invoiceninja.Vendor{
			vendor("canonical-vendor", "Bunnings"), vendor("alias-vendor", "Bunnings Warehouse"),
		},
	}
	provider := &fakeBunnings{products: map[string]bunnings.Product{"7654321": {ItemNumber: "7654321"}}}
	service := bunningsService(ninja, provider, true)
	service.Config.Providers["bunnings"] = config.ProviderConfig{
		Name: "Bunnings", Type: "api", Aliases: []string{"Bunnings Warehouse"}, Location: "Dural",
	}

	results, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "updated" || len(ninja.updates) != 1 {
		t.Fatalf("results=%#v updates=%#v", results, ninja.updates)
	}
	request := ninja.updates[0].request.Values
	if request.VendorID != "canonical-vendor" || request.ProductKey != "7654321" || request.CustomValue1 != "Dural" {
		t.Fatalf("canonical migration = %#v", request)
	}
}

func TestNumericStoreIsNotReinterpretedAsLegacyItemNumber(t *testing.T) {
	ninja := &fakeNinja{
		products: []invoiceninja.Product{{
			Entity: invoiceninja.Entity{ID: "current"}, VendorID: "vendor-bunnings", ProductKey: "0123456",
			CustomValue1: "9473", CustomValue2: "false", CustomValue4: "2026-10-10",
		}},
		vendors: []invoiceninja.Vendor{vendor("vendor-bunnings", "Bunnings")},
	}
	provider := &fakeBunnings{products: map[string]bunnings.Product{"0123456": {ItemNumber: "0123456"}}}
	service := bunningsService(ninja, provider, true)
	service.Config.Providers["bunnings"] = config.ProviderConfig{Name: "Bunnings", Type: "api", Location: "9473"}

	results, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(provider.calls, []string{"0123456"}) || len(results) != 1 || contains(results[0].Changes, "product_key") {
		t.Fatalf("results=%#v calls=%v", results, provider.calls)
	}
}

func TestBunningsAvailabilityAndErrorsRemainDistinct(t *testing.T) {
	ninja := &fakeNinja{
		products: []invoiceninja.Product{
			{Entity: invoiceninja.Entity{ID: "missing"}, VendorID: "vendor-bunnings", ProductKey: "11111", CustomValue2: "false", CustomValue4: "2026-01-01"},
			{Entity: invoiceninja.Entity{ID: "broken"}, VendorID: "vendor-bunnings", ProductKey: "22222", CustomValue2: "false", CustomValue4: "2026-01-02"},
			{Entity: invoiceninja.Entity{ID: "reappeared"}, VendorID: "vendor-bunnings", ProductKey: "33333", CustomValue2: "true", CustomValue4: "2026-01-03"},
		},
		vendors: []invoiceninja.Vendor{vendor("vendor-bunnings", "Bunnings Warehouse")},
	}
	provider := &fakeBunnings{errors: map[string]error{
		"11111": &gobunnings.APIError{StatusCode: http.StatusNotFound},
		"22222": errors.New("temporary provider failure"),
	}}
	service := bunningsService(ninja, provider, true)
	service.Config.Providers["bunnings"] = config.ProviderConfig{
		Name: "Bunnings", Type: "api", Aliases: []string{"Bunnings Warehouse"},
	}

	results, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 || results[0].ProductKey != "11111" || results[0].Action != "updated" || results[1].Action != "error" || results[2].Action != "updated" {
		t.Fatalf("results = %#v", results)
	}
	if len(ninja.updates) != 2 || ninja.updates[0].request.Values.CustomValue2 != "true" || ninja.updates[0].request.Values.CustomValue4 != "2026-10-10" || ninja.updates[1].request.Values.CustomValue2 != "false" {
		t.Fatalf("updates = %#v", ninja.updates)
	}
}

func TestBunningsPricingFailureDoesNotChangeExistingProduct(t *testing.T) {
	ninja := &fakeNinja{
		products: []invoiceninja.Product{{
			Entity: invoiceninja.Entity{ID: "priced"}, VendorID: "vendor-bunnings", ProductKey: "0123456",
			Price: 42.5, Quantity: 7, CustomValue2: "false", CustomValue4: "2026-01-01",
		}},
		vendors: []invoiceninja.Vendor{vendor("vendor-bunnings", "Bunnings")},
	}
	provider := &fakeBunnings{errors: map[string]error{
		"0123456": &bunnings.PricingError{
			ItemNumber: "0123456",
			Err:        &gobunnings.APIError{StatusCode: http.StatusNotFound, Body: []byte("pricing unavailable")},
		},
	}}
	service := bunningsService(ninja, provider, true)

	results, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "error" || results[0].Error == nil || !strings.Contains(results[0].Error.Error(), "pricing unavailable") {
		t.Fatalf("results = %#v", results)
	}
	if len(ninja.updates) != 0 {
		t.Fatalf("pricing failure updated Product: %#v", ninja.updates)
	}
	product := ninja.products[0]
	if product.Price != 42.5 || product.CustomValue2 != "false" || product.CustomValue4 != "2026-01-01" {
		t.Fatalf("existing Product changed = %#v", product)
	}
}

func TestProductUpdatePreservesOrChangesQuantity(t *testing.T) {
	service := Service{Now: func() time.Time { return time.Date(2026, 10, 10, 12, 0, 0, 0, time.Local) }}
	provider := config.ProviderConfig{Name: "Generic Supplier"}
	vendor := vendor("vendor-1", "Generic Supplier")

	tests := []struct {
		name             string
		existingQuantity float64
		quantity         *float64
		wantQuantity     float64
		wantQuantityDiff bool
	}{
		{name: "metadata only", existingQuantity: 7, wantQuantity: 7},
		{name: "metadata only with zero", wantQuantity: 0},
		{name: "provider quantity", existingQuantity: 7, quantity: floatPointer(3), wantQuantity: 3, wantQuantityDiff: true},
		{name: "provider zero quantity", existingQuantity: 7, quantity: floatPointer(0), wantQuantity: 0, wantQuantityDiff: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			existing := invoiceninja.Product{
				Entity: invoiceninja.Entity{ID: "p1"}, VendorID: "vendor-1", ProductKey: "SKU-1",
				Notes: "Old description", Quantity: test.existingQuantity, CustomValue4: "2026-01-01",
			}
			request, changes := service.updateRequest(provider, vendor, existing, productWork{observation: observation{
				Key: "SKU-1", Description: stringPointer("New description"), Quantity: test.quantity,
			}})
			if contains(changes, "quantity") != test.wantQuantityDiff {
				t.Fatalf("changes = %v", changes)
			}
			var payload map[string]json.RawMessage
			encoded, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &payload); err != nil {
				t.Fatal(err)
			}
			var gotQuantity float64
			if err := json.Unmarshal(payload["quantity"], &gotQuantity); err != nil {
				t.Fatalf("quantity payload = %s: %v", payload["quantity"], err)
			}
			if gotQuantity != test.wantQuantity {
				t.Fatalf("quantity = %v; want %v", gotQuantity, test.wantQuantity)
			}
		})
	}
}

func TestBunningsObservationDoesNotInventQuantity(t *testing.T) {
	got := bunningsObservation(bunnings.Product{ItemNumber: "0123456", Title: "Hammer"}, "0123456")
	if got.Quantity != nil {
		t.Fatalf("Bunnings quantity = %v; want nil", *got.Quantity)
	}
}

func TestRefreshPreviewOrdersMissingThenOldestWithoutWriting(t *testing.T) {
	ninja := &fakeNinja{
		products: []invoiceninja.Product{
			{Entity: invoiceninja.Entity{ID: "newer"}, VendorID: "v1", ProductKey: "33333", CustomValue4: "2026-09-01"},
			{Entity: invoiceninja.Entity{ID: "missing"}, VendorID: "v1", ProductKey: "11111"},
			{Entity: invoiceninja.Entity{ID: "older"}, VendorID: "v1", ProductKey: "22222", CustomValue4: "2026-01-01"},
		},
		vendors: []invoiceninja.Vendor{vendor("v1", "Bunnings")},
	}
	provider := &fakeBunnings{products: map[string]bunnings.Product{
		"11111": {ItemNumber: "11111"}, "22222": {ItemNumber: "22222"}, "33333": {ItemNumber: "33333"},
	}}
	service := bunningsService(ninja, provider, false)

	results, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(provider.calls, []string{"11111", "22222", "33333"}) {
		t.Fatalf("refresh order = %v", provider.calls)
	}
	if len(ninja.updates) != 0 || len(ninja.creates) != 0 {
		t.Fatalf("preview mutated Invoice Ninja: updates=%d creates=%d", len(ninja.updates), len(ninja.creates))
	}
	for _, result := range results {
		if result.Action != "would-update" {
			t.Fatalf("preview result = %#v", result)
		}
	}
}

func TestRefreshUsesGlobalExistingQueueBeforeMissingDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("SKU,Description\nEXISTING,Existing row\nMISSING,Missing row\n"))
	}))
	defer server.Close()
	ninja := &fakeNinja{
		products: []invoiceninja.Product{
			{Entity: invoiceninja.Entity{ID: "bunnings-product"}, VendorID: "bunnings-vendor", ProductKey: "12345", CustomValue4: "2026-09-01"},
			{Entity: invoiceninja.Entity{ID: "generic-product"}, VendorID: "generic-vendor", ProductKey: "EXISTING"},
		},
		vendors: []invoiceninja.Vendor{
			vendor("bunnings-vendor", "Bunnings"), vendor("generic-vendor", "Generic Supplier"),
		},
	}
	service := bunningsService(ninja, &fakeBunnings{products: map[string]bunnings.Product{"12345": {ItemNumber: "12345"}}}, true)
	service.Config.Providers["generic"] = config.ProviderConfig{
		Name: "Generic Supplier", Type: "csv", URL: server.URL,
		Fields: config.ProviderFields{Product: "SKU", Description: "Description"},
	}
	service.Cache = &memoryCache{entries: make(map[string]Fingerprint)}
	service.HTTPClient = server.Client()

	results, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if hasError(results) {
		t.Fatalf("results = %#v", results)
	}
	want := []string{"update:generic-product", "update:bunnings-product", "create:MISSING"}
	if !reflect.DeepEqual(ninja.events, want) {
		t.Fatalf("write order = %v; want %v", ninja.events, want)
	}
	if len(ninja.creates) != 1 || ninja.creates[0].VendorID != "generic-vendor" || ninja.creates[0].ProductKey != "MISSING" || ninja.creates[0].CustomValue2 != "false" || ninja.creates[0].CustomValue4 != "2026-10-10" {
		t.Fatalf("missing Product create = %#v", ninja.creates)
	}
}

func TestLegacyMigrationCollisionIsExplicit(t *testing.T) {
	ninja := &fakeNinja{
		products: []invoiceninja.Product{
			{Entity: invoiceninja.Entity{ID: "legacy"}, ProductKey: "BUNNINGS-12345", CustomValue1: "12345"},
			{Entity: invoiceninja.Entity{ID: "native"}, VendorID: "v1", ProductKey: "12345"},
		},
		vendors: []invoiceninja.Vendor{vendor("v1", "Bunnings")},
	}
	provider := &fakeBunnings{products: map[string]bunnings.Product{"12345": {ItemNumber: "12345"}}}
	service := bunningsService(ninja, provider, true)

	results, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Action != "error" || !strings.Contains(results[0].Error.Error(), "multiple Invoice Ninja Products") {
		t.Fatalf("results = %#v", results)
	}
	if len(ninja.creates) != 0 {
		t.Fatalf("collision created a duplicate: %#v", ninja.creates)
	}
}

func TestExplicitImportDoesNotDuplicateConflictingLegacyIdentity(t *testing.T) {
	ninja := &fakeNinja{
		products: []invoiceninja.Product{{
			Entity: invoiceninja.Entity{ID: "conflict"}, ProductKey: "BUNNINGS-12345", CustomValue1: "67890",
		}},
		vendors: []invoiceninja.Vendor{vendor("v1", "Bunnings")},
	}
	provider := &fakeBunnings{products: map[string]bunnings.Product{"12345": {ItemNumber: "12345"}}}
	service := bunningsService(ninja, provider, true)

	results, err := service.RefreshBunningsKeys(context.Background(), []string{"12345"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "error" || !strings.Contains(results[0].Error.Error(), "conflicting Bunnings identifiers") || len(provider.calls) != 0 || len(ninja.creates) != 0 {
		t.Fatalf("results=%#v calls=%v creates=%#v", results, provider.calls, ninja.creates)
	}
}

func TestFileProviderFingerprintOnlyAdvancesAfterCompleteSuccess(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("ETag", `"catalogue-v2"`)
		_, _ = w.Write([]byte("SKU,Description,Cost,Price,Quantity,Image\nA,Alpha,1,2,3,https://example/A.jpg\nB,Beta,4,5,6,https://example/B.jpg\n"))
	}))
	defer server.Close()

	ninja := &fakeNinja{
		products: []invoiceninja.Product{
			{Entity: invoiceninja.Entity{ID: "A"}, VendorID: "vendor-generic", ProductKey: "A"},
			{Entity: invoiceninja.Entity{ID: "B"}, VendorID: "vendor-generic", ProductKey: "B"},
		},
		vendors:     []invoiceninja.Vendor{vendor("vendor-generic", "Generic Supplier")},
		failUpdates: map[string]int{"B": 1},
	}
	cache := &memoryCache{entries: map[string]Fingerprint{"generic": {Provider: "generic", Source: server.URL, Hash: "old"}}}
	service := fileService(ninja, cache, server.URL, "csv")

	first, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !hasError(first) || cache.puts != 0 || cache.entries["generic"].Hash != "old" {
		t.Fatalf("partial result=%#v cache=%#v puts=%d", first, cache.entries, cache.puts)
	}
	second, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if hasError(second) || cache.puts != 1 || cache.entries["generic"].Hash == "old" {
		t.Fatalf("retry result=%#v cache=%#v puts=%d", second, cache.entries, cache.puts)
	}
	if ninja.updateCount["A"] != 2 || ninja.updateCount["B"] != 2 || requests != 2 {
		t.Fatalf("full retry counts=%#v requests=%d", ninja.updateCount, requests)
	}
}

func TestUnchangedFileHashSkipsParsingAndProductWrites(t *testing.T) {
	data := []byte("not,a,valid,mapped,catalogue\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(data) }))
	defer server.Close()
	downloaded, err := downloadCatalogue(context.Background(), server.Client(), config.ProviderConfig{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	cache := &memoryCache{entries: map[string]Fingerprint{"generic": {Provider: "generic", Source: server.URL, Hash: downloaded.Hash}}}
	ninja := &fakeNinja{vendors: []invoiceninja.Vendor{vendor("v1", "Generic Supplier")}}
	service := fileService(ninja, cache, server.URL, "csv")
	service.HTTPClient = server.Client()

	results, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "source-unchanged" || len(ninja.updates) != 0 || len(ninja.creates) != 0 {
		t.Fatalf("results=%#v updates=%d creates=%d", results, len(ninja.updates), len(ninja.creates))
	}
}

func TestFilePreviewDoesNotWriteProductsOrSuccessfulFingerprint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("SKU,Description,Cost,Price,Quantity,Image\nA,Alpha,,,,\n"))
	}))
	defer server.Close()
	cache := &memoryCache{entries: make(map[string]Fingerprint)}
	ninja := &fakeNinja{vendors: []invoiceninja.Vendor{vendor("vendor-generic", "Generic Supplier")}}
	service := fileService(ninja, cache, server.URL, "csv")
	service.Commit = false
	service.HTTPClient = server.Client()

	results, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "would-create" || len(ninja.creates) != 0 || cache.puts != 0 {
		t.Fatalf("results=%#v creates=%d cache puts=%d", results, len(ninja.creates), cache.puts)
	}
}

func TestFileAbsenceDoesNotMarkExistingProductUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("SKU,Description,Cost,Price,Quantity,Image\nPRESENT,Present,,,,\n"))
	}))
	defer server.Close()
	cache := &memoryCache{entries: make(map[string]Fingerprint)}
	ninja := &fakeNinja{
		products: []invoiceninja.Product{{
			Entity: invoiceninja.Entity{ID: "absent"}, VendorID: "vendor-generic", ProductKey: "ABSENT", CustomValue2: "false", CustomValue4: "2026-01-01",
		}},
		vendors: []invoiceninja.Vendor{vendor("vendor-generic", "Generic Supplier")},
	}
	service := fileService(ninja, cache, server.URL, "csv")
	service.HTTPClient = server.Client()

	results, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if hasError(results) || len(ninja.updates) != 0 || len(ninja.creates) != 1 || ninja.creates[0].ProductKey != "PRESENT" || cache.puts != 1 {
		t.Fatalf("results=%#v updates=%#v creates=%#v cache puts=%d", results, ninja.updates, ninja.creates, cache.puts)
	}
}

func TestArchivedFileIdentityBlocksDuplicateAndFingerprint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("SKU,Description,Cost,Price,Quantity,Image\nA,Alpha,,,,\n"))
	}))
	defer server.Close()
	cache := &memoryCache{entries: make(map[string]Fingerprint)}
	ninja := &fakeNinja{
		products: []invoiceninja.Product{{
			Entity: invoiceninja.Entity{ID: "archived", ArchivedAt: 1}, VendorID: "vendor-generic", ProductKey: "A",
		}},
		vendors: []invoiceninja.Vendor{vendor("vendor-generic", "Generic Supplier")},
	}
	service := fileService(ninja, cache, server.URL, "csv")
	service.HTTPClient = server.Client()

	results, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "error" || !strings.Contains(results[0].Error.Error(), "archived or deleted") || len(ninja.creates) != 0 || cache.puts != 0 {
		t.Fatalf("results=%#v creates=%d cache puts=%d", results, len(ninja.creates), cache.puts)
	}
}

func TestArchivedAliasProductBlocksActiveCanonicalCollision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("SKU,Description,Cost,Price,Quantity,Image\nA,Alpha,,,,\n"))
	}))
	defer server.Close()
	cache := &memoryCache{entries: make(map[string]Fingerprint)}
	ninja := &fakeNinja{
		products: []invoiceninja.Product{
			{Entity: invoiceninja.Entity{ID: "active"}, VendorID: "canonical-vendor", ProductKey: "A"},
			{Entity: invoiceninja.Entity{ID: "archived", ArchivedAt: 1}, VendorID: "alias-vendor", ProductKey: "A"},
		},
		vendors: []invoiceninja.Vendor{
			vendor("canonical-vendor", "Generic Supplier"), vendor("alias-vendor", "Generic Wholesale"),
		},
	}
	service := fileService(ninja, cache, server.URL, "csv")
	provider := service.Config.Providers["generic"]
	provider.Aliases = []string{"Generic Wholesale"}
	service.Config.Providers["generic"] = provider
	service.HTTPClient = server.Client()

	results, err := service.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "error" || !strings.Contains(results[0].Error.Error(), "collides") || len(ninja.updates) != 0 || cache.puts != 0 {
		t.Fatalf("results=%#v updates=%d cache puts=%d", results, len(ninja.updates), cache.puts)
	}
}

func TestGenericXLSXMappingUsesConfiguredHeaders(t *testing.T) {
	workbook := excelize.NewFile()
	defer workbook.Close()
	rows := [][]any{{"Code", "Text", "Wholesale"}, {"SKU-1", "Generic widget", 12.5}}
	for rowIndex, row := range rows {
		for columnIndex, value := range row {
			cell, _ := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+1)
			if err := workbook.SetCellValue("Sheet1", cell, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	var encoded bytes.Buffer
	if err := workbook.Write(&encoded); err != nil {
		t.Fatal(err)
	}
	provider := config.ProviderConfig{Type: "xlsx", Fields: config.ProviderFields{Product: "Code", Description: "Text", Cost: "Wholesale"}}
	parsed, err := parseCatalogue(encoded.Bytes(), provider)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 1 || parsed[0].Key != "SKU-1" || parsed[0].Description == nil || *parsed[0].Description != "Generic widget" || parsed[0].Cost == nil || *parsed[0].Cost != 12.5 {
		t.Fatalf("parsed rows = %#v", parsed)
	}
}

func TestConflictingDuplicateFileRowsAreRejected(t *testing.T) {
	provider := config.ProviderConfig{Type: "csv", Fields: config.ProviderFields{Product: "SKU", Price: "Price"}}
	_, err := parseCatalogue([]byte("SKU,Price\nA,1\nA,2\n"), provider)
	if err == nil || !strings.Contains(err.Error(), "conflicting duplicate Product") {
		t.Fatalf("error = %v", err)
	}
}

type fakeNinja struct {
	products    []invoiceninja.Product
	vendors     []invoiceninja.Vendor
	creates     []invoiceninja.CreateProductRequest
	updates     []capturedUpdate
	failUpdates map[string]int
	updateCount map[string]int
	events      []string
}

type capturedUpdate struct {
	id      string
	request invoiceninja.SparseProductUpdateRequest
}

func (f *fakeNinja) ListAllProducts(context.Context) ([]invoiceninja.Product, error) {
	return append([]invoiceninja.Product(nil), f.products...), nil
}

func (f *fakeNinja) ListAllVendors(context.Context) ([]invoiceninja.Vendor, error) {
	return append([]invoiceninja.Vendor(nil), f.vendors...), nil
}

func (f *fakeNinja) CreateCatalogProduct(_ context.Context, request invoiceninja.CreateProductRequest) (invoiceninja.Product, error) {
	f.creates = append(f.creates, request)
	f.events = append(f.events, "create:"+request.ProductKey)
	return invoiceninja.Product{Entity: invoiceninja.Entity{ID: request.ProductKey}, VendorID: request.VendorID, ProductKey: request.ProductKey}, nil
}

func (f *fakeNinja) UpdateCatalogProduct(_ context.Context, id string, request invoiceninja.SparseProductUpdateRequest) (invoiceninja.Product, error) {
	if f.updateCount == nil {
		f.updateCount = make(map[string]int)
	}
	f.updateCount[id]++
	f.updates = append(f.updates, capturedUpdate{id: id, request: request})
	f.events = append(f.events, "update:"+id)
	if f.failUpdates[id] > 0 {
		f.failUpdates[id]--
		return invoiceninja.Product{}, errors.New("injected Product update failure")
	}
	return invoiceninja.Product{Entity: invoiceninja.Entity{ID: id}}, nil
}

type fakeBunnings struct {
	products map[string]bunnings.Product
	errors   map[string]error
	calls    []string
}

func (f *fakeBunnings) GetProduct(_ context.Context, key string) (bunnings.Product, error) {
	f.calls = append(f.calls, key)
	if err := f.errors[key]; err != nil {
		return bunnings.Product{}, err
	}
	return f.products[key], nil
}

type memoryCache struct {
	entries map[string]Fingerprint
	puts    int
}

func (m *memoryCache) Get(provider string) (Fingerprint, bool, error) {
	value, ok := m.entries[provider]
	return value, ok, nil
}

func (m *memoryCache) Put(value Fingerprint) error {
	m.puts++
	m.entries[value.Provider] = value
	return nil
}

func bunningsService(ninja *fakeNinja, provider *fakeBunnings, commit bool) Service {
	return Service{
		Config: config.Config{
			Tax:         config.TaxConfig{Name: "GST", Rate: 10},
			ProductSync: config.ProductSyncConfig{CustomFields: config.ProductCustomFields{BunningsIN: 1, ImageURL: 2}},
			Providers:   map[string]config.ProviderConfig{"bunnings": {Name: "Bunnings", Type: "api"}},
		},
		Ninja: ninja, Bunnings: provider, Commit: commit,
		Now: func() time.Time { return time.Date(2026, 10, 10, 12, 0, 0, 0, time.Local) },
	}
}

func fileService(ninja *fakeNinja, cache *memoryCache, source, providerType string) Service {
	return Service{
		Config: config.Config{
			Tax: config.TaxConfig{Name: "GST", Rate: 10},
			Providers: map[string]config.ProviderConfig{"generic": {
				Name: "Generic Supplier", Type: providerType, URL: source,
				Fields: config.ProviderFields{Product: "SKU", Description: "Description", Cost: "Cost", Price: "Price", Quantity: "Quantity", ImageURL: "Image"},
			}},
		},
		Ninja: ninja, Cache: cache, Commit: true,
		Now: func() time.Time { return time.Date(2026, 10, 10, 12, 0, 0, 0, time.Local) },
	}
}

func vendor(id, name string) invoiceninja.Vendor {
	return invoiceninja.Vendor{Entity: invoiceninja.Entity{ID: id}, Name: name}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func hasError(results []Result) bool {
	for _, result := range results {
		if result.Error != nil {
			return true
		}
	}
	return false
}
