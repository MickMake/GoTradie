package productsync

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	gobunnings "github.com/MickMake/GoBunnings"
	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/bunnings"
	"github.com/MickMake/GoTradie/internal/config"
)

type providerResolution struct {
	vendor invoiceninja.Vendor
}

type productWork struct {
	product     *invoiceninja.Product
	observation observation
	error       error
}

type providerState struct {
	id       string
	provider config.ProviderConfig
	file     *fileProviderState
}

type fileProviderState struct {
	downloaded downloadedCatalogue
	rows       map[string]observation
	matched    map[string]bool
	cache      FingerprintStore
	successful bool
}

type queuedWork struct {
	state *providerState
	work  productWork
}

func (s Service) Refresh(ctx context.Context) ([]Result, error) {
	if s.Ninja == nil {
		return nil, fmt.Errorf("Invoice Ninja Product service is required")
	}
	products, err := s.Ninja.ListAllProducts(ctx)
	if err != nil {
		return nil, fmt.Errorf("load Invoice Ninja Products: %w", err)
	}
	vendors, err := s.Ninja.ListAllVendors(ctx)
	if err != nil {
		return nil, fmt.Errorf("load Invoice Ninja Vendors: %w", err)
	}

	results := make([]Result, 0)
	states := make([]*providerState, 0, len(s.Config.Providers))
	for _, providerID := range sortedProviderIDs(s.Config.Providers) {
		provider := s.Config.Providers[providerID]
		if !(providerID == "bunnings" && provider.Type == "api") && provider.Type != "csv" && provider.Type != "xlsx" {
			results = append(results, Result{Provider: provider.Name, Action: "skipped", Changes: []string{"no supported Product Sync adapter"}})
			continue
		}
		_, err := resolveProviderVendor(provider, vendors)
		if err != nil {
			results = append(results, Result{Provider: provider.Name, Action: "error", Error: err})
			continue
		}
		state := &providerState{id: providerID, provider: provider}
		if provider.Type == "csv" || provider.Type == "xlsx" {
			fileState, result := s.prepareFileProvider(ctx, providerID, provider)
			if result != nil {
				results = append(results, *result)
				continue
			}
			state.file = fileState
		}
		states = append(states, state)
	}

	queue := make([]queuedWork, 0, len(products))
	claims := make(map[string][]int)
	for productIndex := range products {
		product := &products[productIndex]
		if !activeProduct(*product) {
			continue
		}
		for _, state := range states {
			if !recognizesSupplier(state.provider, product.CustomValue1) {
				continue
			}
			work := productWork{product: product}
			key := strings.TrimSpace(product.ProductKey)
			if key == "" {
				work.error = fmt.Errorf("Product %q has a blank supplier Product identifier", product.ID)
			}
			if state.id == "bunnings" && state.provider.Type == "api" {
				work.observation.Key = key
			} else {
				row, exists := state.file.rows[key]
				if !exists {
					continue
				}
				work.observation = row
				state.file.matched[key] = true
			}
			collision := findProviderIdentityCollision(products, state.provider, work.observation.Key, product.ID)
			if collision != nil {
				work.error = fmt.Errorf("Product %q collides with Product %q at identity (%s, %s)", product.ID, collision.ID, canonicalSupplier(state.provider), work.observation.Key)
			}
			queue = append(queue, queuedWork{state: state, work: work})
			claim := state.id + "\x00" + work.observation.Key
			claims[claim] = append(claims[claim], len(queue)-1)
			break
		}
	}
	for _, indexes := range claims {
		if len(indexes) < 2 {
			continue
		}
		for _, index := range indexes {
			work := &queue[index]
			work.work.error = fmt.Errorf("multiple Invoice Ninja Products resolve to identity (%s, %s)", work.state.provider.Name, work.work.observation.Key)
		}
	}
	sort.SliceStable(queue, func(left, right int) bool {
		leftDate, leftValid := workDate(queue[left].work)
		rightDate, rightValid := workDate(queue[right].work)
		if leftValid != rightValid {
			return !leftValid
		}
		if leftValid && !leftDate.Equal(rightDate) {
			return leftDate.Before(rightDate)
		}
		if queue[left].state.provider.Name != queue[right].state.provider.Name {
			return queue[left].state.provider.Name < queue[right].state.provider.Name
		}
		return queue[left].work.observation.Key < queue[right].work.observation.Key
	})
	for _, queued := range queue {
		result := Result{Provider: queued.state.provider.Name, ProductKey: queued.work.observation.Key, ItemNumber: queued.work.observation.Key}
		if queued.work.error != nil {
			result.Action, result.Error = "error", queued.work.error
		} else if queued.state.id == "bunnings" && queued.state.provider.Type == "api" {
			result = s.processBunningsWork(ctx, queued.state.provider, []productWork{queued.work})[0]
		} else {
			result = s.syncProduct(ctx, queued.state.provider, queued.work)
		}
		if queued.state.file != nil && result.Error != nil {
			queued.state.file.successful = false
		}
		results = append(results, result)
	}

	// Existing Invoice Ninja Products are always processed before catalogue rows
	// that would create missing Products.
	for _, state := range states {
		if state.file == nil {
			continue
		}
		keys := make([]string, 0, len(state.file.rows))
		for key := range state.file.rows {
			if !state.file.matched[key] {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			row := state.file.rows[key]
			product, findErr := findProviderProduct(products, state.provider, key)
			result := Result{Provider: state.provider.Name, ProductKey: key, ItemNumber: key}
			if findErr != nil {
				result.Action, result.Error = "error", findErr
			} else if product != nil {
				result.Action = "error"
				result.Error = fmt.Errorf("Product %q at identity (%s, %s) is archived or deleted; restore or explicitly resolve it", product.ID, state.provider.Name, key)
			} else {
				result = s.syncProduct(ctx, state.provider, productWork{observation: row})
			}
			if result.Error != nil {
				state.file.successful = false
			}
			results = append(results, result)
		}
	}

	for _, state := range states {
		if state.file == nil || !s.Commit || !state.file.successful {
			continue
		}
		fingerprint := Fingerprint{
			Provider: state.id, Supplier: canonicalSupplier(state.provider), Aliases: fingerprintAliases(state.provider),
			Source: state.provider.URL, Hash: state.file.downloaded.Hash,
			Fields: state.provider.Fields,
			ETag:   state.file.downloaded.ETag, LastModified: state.file.downloaded.LastModified,
			LastSuccessfulCheck: s.now().Format(time.RFC3339),
		}
		if err := state.file.cache.Put(fingerprint); err != nil {
			results = append(results, Result{Provider: state.provider.Name, Action: "error", Error: err})
		}
	}
	return results, nil
}

func (s Service) prepareFileProvider(ctx context.Context, providerID string, provider config.ProviderConfig) (*fileProviderState, *Result) {
	downloaded, err := downloadCatalogue(ctx, s.HTTPClient, provider)
	if err != nil {
		return nil, &Result{Provider: provider.Name, Action: "error", Error: err}
	}
	cache := s.Cache
	if cache == nil {
		path, err := DefaultFingerprintPath()
		if err != nil {
			return nil, &Result{Provider: provider.Name, Action: "error", Error: err}
		}
		cache = FileFingerprintStore{Path: path}
	}
	previous, found, err := cache.Get(providerID)
	if err != nil {
		return nil, &Result{Provider: provider.Name, Action: "error", Error: err}
	}
	if found && previous.Supplier == canonicalSupplier(provider) && slices.Equal(previous.Aliases, fingerprintAliases(provider)) &&
		previous.Source == provider.URL && previous.Hash == downloaded.Hash && previous.Fields == provider.Fields {
		return nil, &Result{Provider: provider.Name, Action: "source-unchanged"}
	}
	rows, err := parseCatalogue(downloaded.Data, provider)
	if err != nil {
		return nil, &Result{Provider: provider.Name, Action: "error", Error: err}
	}
	byKey := make(map[string]observation, len(rows))
	for _, row := range rows {
		byKey[row.Key] = row
	}
	return &fileProviderState{
		downloaded: downloaded, rows: byKey, matched: make(map[string]bool),
		cache: cache, successful: true,
	}, nil
}

// RefreshBunningsKeys syncs explicitly selected Bunnings item numbers using
// the same Supplier and Product Key identity rules as a full refresh.
func (s Service) RefreshBunningsKeys(ctx context.Context, keys []string) ([]Result, error) {
	provider, ok := s.Config.Providers["bunnings"]
	if !ok {
		return nil, fmt.Errorf("missing required configuration: providers.bunnings")
	}
	products, err := s.Ninja.ListAllProducts(ctx)
	if err != nil {
		return nil, fmt.Errorf("load Invoice Ninja Products: %w", err)
	}
	vendors, err := s.Ninja.ListAllVendors(ctx)
	if err != nil {
		return nil, fmt.Errorf("load Invoice Ninja Vendors: %w", err)
	}
	_, err = resolveProviderVendor(provider, vendors)
	if err != nil {
		return nil, err
	}
	works := make([]productWork, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			works = append(works, productWork{error: fmt.Errorf("Bunnings item number is required")})
			continue
		}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		works = append(works, findProviderWork(products, provider, key))
	}
	return s.processBunningsWork(ctx, provider, works), nil
}

func (s Service) processBunningsWork(ctx context.Context, provider config.ProviderConfig, works []productWork) []Result {
	results := make([]Result, 0, len(works))
	for _, work := range works {
		result := Result{Provider: provider.Name, ProductKey: work.observation.Key, ItemNumber: work.observation.Key}
		if work.error != nil {
			result.Action, result.Error = "error", work.error
			results = append(results, result)
			continue
		}
		if s.Bunnings == nil {
			result.Action = "error"
			result.Error = fmt.Errorf("Bunnings Product Sync adapter is unavailable")
			results = append(results, result)
			continue
		}
		product, err := s.Bunnings.GetProduct(ctx, work.observation.Key)
		if err != nil {
			if !isBunningsNotFound(err) {
				result.Action, result.Error = "error", err
				results = append(results, result)
				continue
			}
			if work.product == nil {
				result.Action = "error"
				result.Error = fmt.Errorf("Bunnings Product %q was not found", work.observation.Key)
				results = append(results, result)
				continue
			}
			work.observation.Availability = availabilityNotAvailable
		} else {
			if returnedKey := strings.TrimSpace(product.ItemNumber); returnedKey != "" && returnedKey != work.observation.Key {
				result.Action = "error"
				result.Error = fmt.Errorf("Bunnings returned Product %q for requested identifier %q", returnedKey, work.observation.Key)
				results = append(results, result)
				continue
			}
			work.observation = bunningsObservation(product, work.observation.Key)
		}
		result = s.syncProduct(ctx, provider, work)
		results = append(results, result)
	}
	return results
}

func (s Service) syncProduct(ctx context.Context, provider config.ProviderConfig, work productWork) Result {
	result := Result{Provider: provider.Name, ItemNumber: work.observation.Key, ProductKey: work.observation.Key}
	if work.product == nil {
		request := s.createRequest(provider, work)
		result.Changes = []string{"created"}
		if !s.Commit {
			result.Action = "would-create"
			return result
		}
		if _, err := s.Ninja.CreateCatalogProduct(ctx, request); err != nil {
			result.Action, result.Error = "error", err
			return result
		}
		result.Action = "created"
		return result
	}

	request, changes := s.updateRequest(provider, *work.product, work)
	result.Changes = changes
	if len(changes) == 0 {
		result.Action = "unchanged"
		return result
	}
	if !s.Commit {
		result.Action = "would-update"
		return result
	}
	if _, err := s.Ninja.UpdateCatalogProduct(ctx, work.product.ID, request); err != nil {
		result.Action, result.Error = "error", err
		return result
	}
	result.Action = "updated"
	return result
}

func (s Service) createRequest(provider config.ProviderConfig, work productWork) invoiceninja.CreateProductRequest {
	observation := work.observation
	request := invoiceninja.CreateProductRequest{
		ProductKey: observation.Key,
		TaxName1:   s.Config.Tax.Name, TaxRate1: s.Config.Tax.Rate,
		CustomValue1: canonicalSupplier(provider),
		CustomValue2: strings.TrimSpace(provider.Location),
		CustomValue3: availabilityValue(observation.Availability),
		CustomValue4: s.today(),
	}
	if observation.Description != nil {
		request.Notes = *observation.Description
	}
	if observation.Cost != nil {
		request.Cost = *observation.Cost
	}
	if observation.Price != nil {
		request.Price = *observation.Price
	}
	if observation.Quantity != nil {
		request.Quantity = *observation.Quantity
	}
	if observation.ImageURL != nil {
		request.ProductImage = *observation.ImageURL
	}
	return request
}

func (s Service) updateRequest(provider config.ProviderConfig, existing invoiceninja.Product, work productWork) (invoiceninja.SparseProductUpdateRequest, []string) {
	values := invoiceninja.CreateProductRequest{}
	fields := make([]string, 0)
	addString := func(field, old, next string, target *string) {
		if old == next {
			return
		}
		*target = next
		fields = append(fields, field)
	}
	addFloat := func(field string, old float64, next *float64, target *float64) {
		if next == nil || old == *next {
			return
		}
		*target = *next
		fields = append(fields, field)
	}

	addString("product_key", existing.ProductKey, work.observation.Key, &values.ProductKey)
	if work.observation.Description != nil {
		addString("notes", existing.Notes, *work.observation.Description, &values.Notes)
	}
	addFloat("cost", existing.Cost, work.observation.Cost, &values.Cost)
	addFloat("price", existing.Price, work.observation.Price, &values.Price)
	values.Quantity = existing.Quantity
	addFloat("quantity", existing.Quantity, work.observation.Quantity, &values.Quantity)

	nextImage := work.observation.ImageURL
	if nextImage != nil {
		addString("product_image", existing.ProductImage, *nextImage, &values.ProductImage)
	}

	store := existing.CustomValue2
	if store == "" {
		store = strings.TrimSpace(provider.Location)
	}
	addString("custom_value1", existing.CustomValue1, canonicalSupplier(provider), &values.CustomValue1)
	addString("custom_value2", existing.CustomValue2, store, &values.CustomValue2)
	addString("custom_value3", existing.CustomValue3, availabilityValue(work.observation.Availability), &values.CustomValue3)
	addString("custom_value4", existing.CustomValue4, s.today(), &values.CustomValue4)

	request := invoiceninja.NewSparseProductUpdateRequest(values).WithExplicitFields(fields...)
	if len(fields) > 0 {
		request = request.WithExplicitFields("quantity")
	}
	return request, fields
}

func resolveProviderVendor(provider config.ProviderConfig, vendors []invoiceninja.Vendor) (providerResolution, error) {
	resolution := providerResolution{}
	canonical := make([]invoiceninja.Vendor, 0)
	aliases := make([]invoiceninja.Vendor, 0)
	for _, vendor := range vendors {
		name := strings.TrimSpace(vendor.Name)
		isCanonical := strings.EqualFold(name, strings.TrimSpace(provider.Name))
		isAlias := false
		if !isCanonical {
			for _, alias := range provider.Aliases {
				if strings.EqualFold(name, strings.TrimSpace(alias)) {
					isAlias = true
					break
				}
			}
		}
		if !isCanonical && !isAlias {
			continue
		}
		if vendor.IsDeleted || vendor.ArchivedAt != 0 {
			continue
		}
		if isCanonical {
			canonical = append(canonical, vendor)
		} else {
			aliases = append(aliases, vendor)
		}
	}
	selected := canonical
	if len(selected) == 0 {
		selected = aliases
	}
	if len(selected) == 0 {
		return providerResolution{}, fmt.Errorf("no active Invoice Ninja Vendor matches Provider %q or its aliases", provider.Name)
	}
	if len(selected) != 1 {
		return providerResolution{}, fmt.Errorf("multiple active Invoice Ninja Vendors match Provider %q; resolve the ambiguity", provider.Name)
	}
	resolution.vendor = selected[0]
	return resolution, nil
}

func findProviderWork(products []invoiceninja.Product, provider config.ProviderConfig, key string) productWork {
	var found *productWork
	for index := range products {
		product := &products[index]
		if !recognizesSupplier(provider, product.CustomValue1) || strings.TrimSpace(product.ProductKey) != key {
			continue
		}
		if found != nil {
			return productWork{observation: observation{Key: key}, error: fmt.Errorf("multiple Invoice Ninja Products resolve to identity (%s, %s)", canonicalSupplier(provider), key)}
		}
		found = &productWork{product: product, observation: observation{Key: key}}
	}
	if found == nil {
		return productWork{observation: observation{Key: key}}
	}
	if !activeProduct(*found.product) {
		found.error = fmt.Errorf("Product %q is archived or deleted; restore or explicitly resolve it", found.product.ID)
	}
	return *found
}

func findProviderProduct(products []invoiceninja.Product, provider config.ProviderConfig, key string) (*invoiceninja.Product, error) {
	var found *invoiceninja.Product
	for index := range products {
		product := &products[index]
		if !recognizesSupplier(provider, product.CustomValue1) || strings.TrimSpace(product.ProductKey) != key {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("multiple Invoice Ninja Products resolve to identity (%s, %s)", canonicalSupplier(provider), key)
		}
		found = product
	}
	return found, nil
}

func findProviderIdentityCollision(products []invoiceninja.Product, provider config.ProviderConfig, key, excludeID string) *invoiceninja.Product {
	for index := range products {
		product := &products[index]
		if product.ID != excludeID && recognizesSupplier(provider, product.CustomValue1) && strings.TrimSpace(product.ProductKey) == key {
			return product
		}
	}
	return nil
}

func bunningsObservation(product bunnings.Product, requestedKey string) observation {
	key := strings.TrimSpace(product.ItemNumber)
	if key == "" {
		key = requestedKey
	}
	description := productNotes(product, key)
	row := observation{
		Key: key, Description: stringPointer(description),
		Availability: availabilityAvailable,
	}
	if product.Price != 0 {
		row.Price = floatPointer(product.Price)
	}
	if strings.TrimSpace(product.ImageURL) != "" {
		row.ImageURL = stringPointer(strings.TrimSpace(product.ImageURL))
	}
	return row
}

func productNotes(product bunnings.Product, key string) string {
	parts := make([]string, 0, 3)
	if strings.TrimSpace(product.Title) != "" {
		parts = append(parts, strings.TrimSpace(product.Title))
	}
	if description := strings.TrimSpace(product.Description); description != "" && description != strings.TrimSpace(product.Title) {
		parts = append(parts, description)
	}
	parts = append(parts, "Bunnings IN: "+key)
	return strings.Join(parts, "\n\n")
}

func isBunningsNotFound(err error) bool {
	var pricingError *bunnings.PricingError
	if errors.As(err, &pricingError) {
		return false
	}
	var apiError *gobunnings.APIError
	return errors.As(err, &apiError) && apiError.StatusCode == http.StatusNotFound
}

func workDate(work productWork) (time.Time, bool) {
	if work.product == nil {
		return time.Time{}, false
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(work.product.CustomValue4))
	return parsed, err == nil
}

func activeProduct(product invoiceninja.Product) bool {
	return !product.IsDeleted && product.ArchivedAt == 0
}

func availabilityValue(value availability) string {
	if value == availabilityNotAvailable {
		return "true"
	}
	if value == availabilityAvailable {
		return "false"
	}
	return ""
}

func canonicalSupplier(provider config.ProviderConfig) string {
	return strings.TrimSpace(provider.Name)
}

func recognizesSupplier(provider config.ProviderConfig, supplier string) bool {
	supplier = strings.TrimSpace(supplier)
	if supplier == "" {
		return false
	}
	if strings.EqualFold(supplier, canonicalSupplier(provider)) {
		return true
	}
	for _, alias := range provider.Aliases {
		if strings.EqualFold(supplier, strings.TrimSpace(alias)) {
			return true
		}
	}
	return false
}

func fingerprintAliases(provider config.ProviderConfig) []string {
	aliases := make([]string, 0, len(provider.Aliases))
	for _, alias := range provider.Aliases {
		aliases = append(aliases, strings.TrimSpace(alias))
	}
	sort.Strings(aliases)
	return aliases
}

func sortedProviderIDs(providers map[string]config.ProviderConfig) []string {
	ids := make([]string, 0, len(providers))
	for id := range providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s Service) today() string {
	return s.now().In(time.Local).Format("2006-01-02")
}
