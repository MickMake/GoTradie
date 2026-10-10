package productsync

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	gobunnings "github.com/MickMake/GoBunnings"
	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/bunnings"
	"github.com/MickMake/GoTradie/internal/config"
)

type providerResolution struct {
	vendor        invoiceninja.Vendor
	recognizedIDs map[string]struct{}
}

type productWork struct {
	product     *invoiceninja.Product
	observation observation
	legacy      bool
	legacyImage string
	error       error
}

type providerState struct {
	id         string
	provider   config.ProviderConfig
	resolution providerResolution
	file       *fileProviderState
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
		resolution, err := resolveProviderVendor(provider, vendors)
		if err != nil {
			results = append(results, Result{Provider: provider.Name, Action: "error", Error: err})
			continue
		}
		state := &providerState{id: providerID, provider: provider, resolution: resolution}
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
			_, recognized := state.resolution.recognizedIDs[product.VendorID]
			work := productWork{product: product}
			if state.id == "bunnings" && state.provider.Type == "api" {
				key, legacy, legacyImage, err := legacyBunningsDetails(*product, recognized, s.Config.ProductSync.CustomFields, state.provider.Location)
				if !recognized && !legacy {
					continue
				}
				if !legacy {
					key = strings.TrimSpace(product.ProductKey)
				}
				work.observation.Key, work.legacy, work.legacyImage, work.error = key, legacy, legacyImage, err
				if err == nil && key == "" {
					work.error = fmt.Errorf("Product %q has a blank supplier Product identifier", product.ID)
				}
			} else {
				if !recognized {
					continue
				}
				key := strings.TrimSpace(product.ProductKey)
				row, exists := state.file.rows[key]
				if !exists {
					continue
				}
				work.observation = row
				state.file.matched[key] = true
			}
			var collision *invoiceninja.Product
			if state.id == "bunnings" && state.provider.Type == "api" {
				collision = findBunningsIdentityCollision(products, state.resolution, work.observation.Key, product.ID, s.Config.ProductSync.CustomFields, state.provider.Location)
			} else {
				collision = findProviderIdentityCollision(products, state.resolution, work.observation.Key, product.ID)
			}
			if collision != nil {
				work.error = fmt.Errorf("Product %q migration collides with Product %q at identity (%s, %s)", product.ID, collision.ID, state.provider.Name, work.observation.Key)
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
			result = s.processBunningsWork(ctx, queued.state.provider, queued.state.resolution, []productWork{queued.work})[0]
		} else {
			result = s.syncProduct(ctx, queued.state.provider, queued.state.resolution.vendor, queued.work)
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
			product, findErr := findProviderProduct(products, state.resolution, key)
			result := Result{Provider: state.provider.Name, ProductKey: key, ItemNumber: key}
			if findErr != nil {
				result.Action, result.Error = "error", findErr
			} else if product != nil {
				result.Action = "error"
				result.Error = fmt.Errorf("Product %q at identity (%s, %s) is archived or deleted; restore or explicitly resolve it", product.ID, state.provider.Name, key)
			} else {
				result = s.syncProduct(ctx, state.provider, state.resolution.vendor, productWork{observation: row})
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
			Provider: state.id, Source: state.provider.URL, Hash: state.file.downloaded.Hash,
			ETag: state.file.downloaded.ETag, LastModified: state.file.downloaded.LastModified,
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
	if found && previous.Source == provider.URL && previous.Hash == downloaded.Hash {
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
// the same identity and migration rules as a full refresh.
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
	resolution, err := resolveProviderVendor(provider, vendors)
	if err != nil {
		return nil, err
	}
	works := make([]productWork, 0, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			works = append(works, productWork{error: fmt.Errorf("Bunnings item number is required")})
			continue
		}
		works = append(works, findBunningsWork(products, resolution, key, s.Config.ProductSync.CustomFields, provider.Location))
	}
	return s.processBunningsWork(ctx, provider, resolution, works), nil
}

func (s Service) processBunningsWork(ctx context.Context, provider config.ProviderConfig, resolution providerResolution, works []productWork) []Result {
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
		result = s.syncProduct(ctx, provider, resolution.vendor, work)
		results = append(results, result)
	}
	return results
}

func (s Service) syncProduct(ctx context.Context, provider config.ProviderConfig, vendor invoiceninja.Vendor, work productWork) Result {
	result := Result{Provider: provider.Name, ItemNumber: work.observation.Key, ProductKey: work.observation.Key}
	if work.product == nil {
		request := s.createRequest(provider, vendor, work)
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

	request, changes := s.updateRequest(provider, vendor, *work.product, work)
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

func (s Service) createRequest(provider config.ProviderConfig, vendor invoiceninja.Vendor, work productWork) invoiceninja.CreateProductRequest {
	observation := work.observation
	request := invoiceninja.CreateProductRequest{
		VendorID: vendor.ID, ProductKey: observation.Key,
		TaxName1: s.Config.Tax.Name, TaxRate1: s.Config.Tax.Rate,
		CustomValue1: strings.TrimSpace(provider.Location),
		CustomValue2: availabilityValue(observation.Availability),
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
	} else if work.legacyImage != "" {
		request.ProductImage = work.legacyImage
	}
	if observation.SupplyUnit != nil {
		request.CustomValue3 = *observation.SupplyUnit
	}
	return request
}

func (s Service) updateRequest(provider config.ProviderConfig, vendor invoiceninja.Vendor, existing invoiceninja.Product, work productWork) (invoiceninja.SparseProductUpdateRequest, []string) {
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

	addString("vendor_id", existing.VendorID, vendor.ID, &values.VendorID)
	addString("product_key", existing.ProductKey, work.observation.Key, &values.ProductKey)
	if work.observation.Description != nil {
		addString("notes", existing.Notes, *work.observation.Description, &values.Notes)
	}
	addFloat("cost", existing.Cost, work.observation.Cost, &values.Cost)
	addFloat("price", existing.Price, work.observation.Price, &values.Price)
	addFloat("quantity", existing.Quantity, work.observation.Quantity, &values.Quantity)

	nextImage := work.observation.ImageURL
	if nextImage == nil && existing.ProductImage == "" && work.legacyImage != "" {
		nextImage = stringPointer(work.legacyImage)
	}
	if nextImage != nil {
		addString("product_image", existing.ProductImage, *nextImage, &values.ProductImage)
	}

	store := existing.CustomValue1
	if work.legacy && legacySlot(s.Config.ProductSync.CustomFields, customStore) {
		store = ""
	}
	if store == "" {
		store = strings.TrimSpace(provider.Location)
	}
	supplyUnit := existing.CustomValue3
	if work.legacy && legacySlot(s.Config.ProductSync.CustomFields, customSupplyUnit) {
		supplyUnit = ""
	}
	if supplyUnit == "" && work.observation.SupplyUnit != nil {
		supplyUnit = *work.observation.SupplyUnit
	}
	addString("custom_value1", existing.CustomValue1, store, &values.CustomValue1)
	addString("custom_value2", existing.CustomValue2, availabilityValue(work.observation.Availability), &values.CustomValue2)
	addString("custom_value3", existing.CustomValue3, supplyUnit, &values.CustomValue3)
	addString("custom_value4", existing.CustomValue4, s.today(), &values.CustomValue4)

	return invoiceninja.NewSparseProductUpdateRequest(values).WithExplicitFields(fields...), fields
}

func resolveProviderVendor(provider config.ProviderConfig, vendors []invoiceninja.Vendor) (providerResolution, error) {
	resolution := providerResolution{recognizedIDs: make(map[string]struct{})}
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
		resolution.recognizedIDs[vendor.ID] = struct{}{}
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

func legacyBunningsDetails(product invoiceninja.Product, recognizedVendor bool, fields config.ProductCustomFields, configuredStore string) (string, bool, string, error) {
	hasPrefix := strings.HasPrefix(product.ProductKey, "BUNNINGS-")
	prefixed := ""
	var detailError error
	if hasPrefix {
		prefixed = strings.TrimSpace(strings.TrimPrefix(product.ProductKey, "BUNNINGS-"))
		if prefixed == "" {
			detailError = fmt.Errorf("legacy Product %q has an empty BUNNINGS- identifier", product.ID)
		}
	}
	custom := strings.TrimSpace(productCustom(product, fields.BunningsIN))
	_, dateError := time.Parse("2006-01-02", strings.TrimSpace(product.CustomValue4))
	currentLayout := (product.CustomValue2 == "true" || product.CustomValue2 == "false") && dateError == nil
	useCustom := hasPrefix || custom == strings.TrimSpace(product.ProductKey) ||
		decimalDigits(custom) && (product.VendorID == "" || recognizedVendor && !currentLayout && custom != strings.TrimSpace(configuredStore))
	if !useCustom {
		custom = ""
	}
	if prefixed != "" && custom != "" && prefixed != custom {
		detailError = fmt.Errorf("legacy Product %q has conflicting Bunnings identifiers %q and %q", product.ID, prefixed, custom)
	}
	key := prefixed
	if key == "" {
		key = custom
	}
	legacy := hasPrefix || custom != ""
	legacyImage := ""
	if legacy {
		legacyImage = strings.TrimSpace(productCustom(product, fields.ImageURL))
	}
	return key, legacy, legacyImage, detailError
}

func findBunningsWork(products []invoiceninja.Product, resolution providerResolution, key string, fields config.ProductCustomFields, configuredStore string) productWork {
	var found *productWork
	for index := range products {
		product := &products[index]
		_, recognized := resolution.recognizedIDs[product.VendorID]
		legacyKey, legacy, legacyImage, err := legacyBunningsDetails(*product, recognized, fields, configuredStore)
		matches := recognized && strings.TrimSpace(product.ProductKey) == key || legacy && legacyKey == key
		if !matches {
			continue
		}
		if found != nil {
			return productWork{observation: observation{Key: key}, error: fmt.Errorf("multiple Invoice Ninja Products resolve to Bunnings Product %q", key)}
		}
		found = &productWork{
			product: product, observation: observation{Key: key}, legacy: legacy,
			legacyImage: legacyImage, error: err,
		}
	}
	if found == nil {
		return productWork{observation: observation{Key: key}}
	}
	if !activeProduct(*found.product) {
		found.error = fmt.Errorf("Product %q is archived or deleted; restore or explicitly resolve it", found.product.ID)
	}
	if collision := findBunningsIdentityCollision(products, resolution, key, found.product.ID, fields, configuredStore); collision != nil {
		found.error = fmt.Errorf("Product %q migration collides with Product %q at Bunnings Product %q", found.product.ID, collision.ID, key)
	}
	return *found
}

func decimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func findProviderProduct(products []invoiceninja.Product, resolution providerResolution, key string) (*invoiceninja.Product, error) {
	var found *invoiceninja.Product
	for index := range products {
		product := &products[index]
		if _, ok := resolution.recognizedIDs[product.VendorID]; !ok || strings.TrimSpace(product.ProductKey) != key {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("multiple Invoice Ninja Products use supplier Product identifier %q", key)
		}
		found = product
	}
	return found, nil
}

func findProviderIdentityCollision(products []invoiceninja.Product, resolution providerResolution, key, excludeID string) *invoiceninja.Product {
	for index := range products {
		product := &products[index]
		_, recognized := resolution.recognizedIDs[product.VendorID]
		if product.ID != excludeID && recognized && strings.TrimSpace(product.ProductKey) == key {
			return product
		}
	}
	return nil
}

func findBunningsIdentityCollision(products []invoiceninja.Product, resolution providerResolution, key, excludeID string, fields config.ProductCustomFields, configuredStore string) *invoiceninja.Product {
	for index := range products {
		product := &products[index]
		if product.ID == excludeID {
			continue
		}
		_, recognized := resolution.recognizedIDs[product.VendorID]
		legacyKey, legacy, _, _ := legacyBunningsDetails(*product, recognized, fields, configuredStore)
		resolvedKey := strings.TrimSpace(product.ProductKey)
		if legacy {
			resolvedKey = legacyKey
		} else if !recognized {
			continue
		}
		if resolvedKey == key {
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
		Key: key, Description: stringPointer(description), Quantity: floatPointer(1),
		Availability: availabilityAvailable,
	}
	if product.Price != 0 {
		row.Price = floatPointer(product.Price)
	}
	if strings.TrimSpace(product.ImageURL) != "" {
		row.ImageURL = stringPointer(strings.TrimSpace(product.ImageURL))
	}
	if strings.TrimSpace(product.Unit) != "" {
		row.SupplyUnit = stringPointer(strings.TrimSpace(product.Unit))
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

func productCustom(product invoiceninja.Product, field int) string {
	switch field {
	case 1:
		return product.CustomValue1
	case 2:
		return product.CustomValue2
	case 3:
		return product.CustomValue3
	case 4:
		return product.CustomValue4
	default:
		return ""
	}
}

func legacySlot(fields config.ProductCustomFields, slot int) bool {
	return fields.BunningsIN == slot || fields.ImageURL == slot
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
