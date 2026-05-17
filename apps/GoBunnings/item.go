package gobunnings

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type ItemService struct{ client *Client }

const itemAPIVersion = "1.4"

type ItemSearchRequest struct {
	Query   string             `json:"query,omitempty"`
	Filters *ItemSearchFilters `json:"filters,omitempty"`
	SortBy  string             `json:"sortBy,omitempty"`
}

type ItemSearchFilters struct {
	Facets                      []ItemSearchFacet `json:"facets,omitempty"`
	LocationCode                string            `json:"locationCode,omitempty"`
	AvailableInStoreAllProducts *AvailableInStore `json:"availableInStoreAllProducts,omitempty"`
}

type AvailableInStore struct {
	AllProducts  bool `json:"allProducts"`
	InStoreToday bool `json:"inStoreToday"`
}

type ItemSearchFacet struct {
	ID           string   `json:"id,omitempty"`
	Value        string   `json:"value,omitempty"`
	StartRange   *float64 `json:"startRange,omitempty"`
	EndRange     *float64 `json:"endRange,omitempty"`
	EndInclusive *bool    `json:"endInclusive,omitempty"`
}

type ItemSearchResponse struct {
	Results []ItemSearchResult `json:"results,omitempty"`
	Facets  []Facet            `json:"facets,omitempty"`
	Meta    map[string]any     `json:"_meta,omitempty"`
	Links   []HateOASLink      `json:"_links,omitempty"`
}

type ItemSearchResult struct {
	Title      string         `json:"title,omitempty"`
	ItemNumber string         `json:"itemNumber,omitempty"`
	Meta       map[string]any `json:"_meta,omitempty"`
	Links      []HateOASLink  `json:"_links,omitempty"`
}
type Facet struct {
	ID          string       `json:"id,omitempty"`
	DisplayName string       `json:"displayName,omitempty"`
	Type        string       `json:"type,omitempty"`
	Values      []FacetValue `json:"values,omitempty"`
}
type FacetValue struct {
	DisplayName  string   `json:"displayName,omitempty"`
	IsSelected   bool     `json:"isSelected,omitempty"`
	ResultCount  int      `json:"resultCount,omitempty"`
	Value        string   `json:"value,omitempty"`
	StartRange   *float64 `json:"startRange,omitempty"`
	EndRange     *float64 `json:"endRange,omitempty"`
	EndInclusive *bool    `json:"endInclusive,omitempty"`
}

type ItemDetails struct {
	Items []Item         `json:"items,omitempty"`
	Meta  map[string]any `json:"_meta,omitempty"`
	Links []HateOASLink  `json:"_links,omitempty"`
}
type Item struct {
	ItemNumber        string      `json:"itemNumber,omitempty"`
	Brand             any         `json:"brand,omitempty"`
	Description       Description `json:"description,omitempty"`
	FamilyTree        any         `json:"familyTree,omitempty"`
	ItemType          string      `json:"itemType,omitempty"`
	SaleUnitOfMeasure string      `json:"saleUnitOfMeasure,omitempty"`
	Barcodes          []Barcode   `json:"barcodes,omitempty"`
	// ImageURL is the best product image URL exposed by the Bunnings product data.
	// It is derived from enrichedItem.picture.primaryAssetURL when present, with
	// fallback paths for related media fields. Raw remains available for callers
	// that need fields not yet modelled by this SDK.
	ImageURL string         `json:"imageUrl,omitempty"`
	Raw      map[string]any `json:"-"`
}

func (i *Item) UnmarshalJSON(data []byte) error {
	type itemAlias Item
	var typed itemAlias
	if err := json.Unmarshal(data, &typed); err != nil {
		return err
	}

	var raw map[string]any
	if string(data) != "null" {
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
	}

	*i = Item(typed)
	i.Raw = raw
	if imageURL := bestItemImageURL(raw); imageURL != "" {
		i.ImageURL = imageURL
	}
	return nil
}

func bestItemImageURL(raw map[string]any) string {
	if raw == nil {
		return ""
	}

	paths := [][]string{
		{"enrichedItem", "picture", "primaryAssetURL"},
		{"enrichedItem", "picture", "primaryAssetUrl"},
		{"enrichedItem", "picture", "url"},
		{"picture", "primaryAssetURL"},
		{"picture", "primaryAssetUrl"},
		{"picture", "url"},
		{"imageUrl"},
		{"imageURL"},
	}
	for _, path := range paths {
		if value := stringAt(raw, path...); value != "" {
			return value
		}
	}

	if value := firstMediaURL(raw, "enrichedItem", "otherImages"); value != "" {
		return value
	}
	if value := firstMediaURL(raw, "otherImages"); value != "" {
		return value
	}

	return ""
}

func stringAt(raw map[string]any, path ...string) string {
	var cur any = raw
	for _, part := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[part]
	}
	value, _ := cur.(string)
	return value
}

func firstMediaURL(raw map[string]any, path ...string) string {
	var cur any = raw
	for _, part := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[part]
	}

	items, ok := cur.([]any)
	if !ok {
		return ""
	}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if value := stringAt(m, "primaryAssetURL"); value != "" {
			return value
		}
		if value := stringAt(m, "primaryAssetUrl"); value != "" {
			return value
		}
		if value := stringAt(m, "url"); value != "" {
			return value
		}
	}

	return ""
}

type Description struct {
	SectionDescription string `json:"sectionDescription,omitempty"`
	ProductDescription string `json:"productDescription,omitempty"`
}
type Barcode struct {
	Number string `json:"number,omitempty"`
	Level  string `json:"level,omitempty"`
	// AdditionalDescription intentionally uses the capitalized JSON key from the
	// Bunnings Query Item OpenAPI schema and response examples.
	AdditionalDescription string `json:"AdditionalDescription,omitempty"`
	SupplierItemNumber    string `json:"supplierItemNumber,omitempty"`
}

func (s *ItemService) Discovery(ctx context.Context) (*EntryPoint, error) {
	var out EntryPoint
	err := s.client.do(ctx, s.client.BaseURLs.Item, itemAPIVersion, http.MethodGet, "/discovery", nil, nil, &out, nil)
	return &out, err
}
func (s *ItemService) Search(ctx context.Context, country CountryCode, req ItemSearchRequest, continuationToken string) (*ItemSearchResponse, error) {
	var out ItemSearchResponse
	h := http.Header{}
	if continuationToken != "" {
		h.Set("continuationToken", continuationToken)
	}
	err := s.client.do(ctx, s.client.BaseURLs.Item, itemAPIVersion, http.MethodPost, fmt.Sprintf("/search/%s", country), nil, req, &out, h)
	return &out, err
}
func (s *ItemService) Detail(ctx context.Context, country CountryCode, itemNumber string, opt QueryOptions) (*Item, error) {
	var out Item
	err := s.client.do(ctx, s.client.BaseURLs.Item, itemAPIVersion, http.MethodGet, fmt.Sprintf("/detail/%s/%s", country, cleanElem(itemNumber)), opt.Values(), nil, &out, nil)
	return &out, err
}
func (s *ItemService) Details(ctx context.Context, country CountryCode, itemNumbers []string, opt QueryOptions) (*ItemDetails, error) {
	q := opt.Values()
	if len(itemNumbers) > 0 {
		q.Set("itemNumbers", joinCSV(itemNumbers))
	}
	var out ItemDetails
	err := s.client.do(ctx, s.client.BaseURLs.Item, itemAPIVersion, http.MethodGet, fmt.Sprintf("/details/%s", country), q, nil, &out, nil)
	return &out, err
}

func (s *ItemService) Locations(ctx context.Context, country CountryCode, itemNumbers, locationCodes []string, includeItemDetail, includeMessages bool, opt QueryOptions) ([]ItemLocation, error) {
	q := opt.Values()
	q.Set("itemNumbers", joinCSV(itemNumbers))
	q.Set("locationCodes", joinCSV(locationCodes))
	q.Set("includeItemDetail", fmt.Sprint(includeItemDetail))
	q.Set("includeMessages", fmt.Sprint(includeMessages))
	var out []ItemLocation
	err := s.client.do(ctx, s.client.BaseURLs.Item, itemAPIVersion, http.MethodGet, fmt.Sprintf("/locations/%s", country), q, nil, &out, nil)
	return out, err
}
func (s *ItemService) LocationSearch(ctx context.Context, country CountryCode, itemNumbers, locationCodes []string, includeMessages bool, opt QueryOptions) ([]ItemLocation, error) {
	q := opt.Values()
	q.Set("itemNumbers", joinCSV(itemNumbers))
	q.Set("locationCodes", joinCSV(locationCodes))
	q.Set("includeMessages", fmt.Sprint(includeMessages))
	var out []ItemLocation
	err := s.client.do(ctx, s.client.BaseURLs.Item, itemAPIVersion, http.MethodGet, fmt.Sprintf("/locations/%s/search", country), q, nil, &out, nil)
	return out, err
}
func (s *ItemService) LocationMessages(ctx context.Context, country CountryCode, itemNumber, locationCode string, includeMessages bool, opt QueryOptions) ([]ItemLocation, error) {
	q := opt.Values()
	q.Set("itemNumber", itemNumber)
	q.Set("locationCode", locationCode)
	q.Set("includeMessages", fmt.Sprint(includeMessages))
	var out []ItemLocation
	err := s.client.do(ctx, s.client.BaseURLs.Item, itemAPIVersion, http.MethodGet, fmt.Sprintf("/locations/%s/messages", country), q, nil, &out, nil)
	return out, err
}

type ItemLocation struct {
	ItemNumber        string         `json:"itemNumber,omitempty"`
	LocationCode      string         `json:"locationCode,omitempty"`
	ItemStatus        string         `json:"itemStatus,omitempty"`
	IsOnRecall        bool           `json:"isOnRecall,omitempty"`
	IsRanged          bool           `json:"isRanged,omitempty"`
	PrimaryBarcode    string         `json:"primaryBarcode,omitempty"`
	IsSellableInStore bool           `json:"isSellableInStore,omitempty"`
	Meta              map[string]any `json:"_meta,omitempty"`
	Links             []HateOASLink  `json:"_links,omitempty"`
}
