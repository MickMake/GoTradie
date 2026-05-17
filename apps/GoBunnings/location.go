package gobunnings

import (
	"context"
	"fmt"
	"net/http"
)

type LocationService struct{ client *Client }

const locationAPIVersion = "1.0"

type LocationsResponse struct {
	Locations []Location     `json:"locations,omitempty"`
	Meta      map[string]any `json:"_meta,omitempty"`
	Links     []HateOASLink  `json:"_links,omitempty"`
}
type Location struct {
	LocationGln  string         `json:"locationGln,omitempty"`
	LocationCode string         `json:"locationCode,omitempty"`
	Name         string         `json:"name,omitempty"`
	CountryCode  CountryCode    `json:"countryCode,omitempty"`
	Region       CodeName       `json:"region,omitempty"`
	Address      Address        `json:"address,omitempty"`
	Phone        string         `json:"phone,omitempty"`
	Email        string         `json:"email,omitempty"`
	IsStore      bool           `json:"isStore,omitempty"`
	IsActive     bool           `json:"isActive,omitempty"`
	IsTrading    bool           `json:"isTrading,omitempty"`
	FriendlyName string         `json:"friendlyName,omitempty"`
	GeoLocation  GeoLocation    `json:"geoLocation,omitempty"`
	Meta         map[string]any `json:"_meta,omitempty"`
	Links        []HateOASLink  `json:"_links,omitempty"`
}
type CodeName struct {
	Code string `json:"code,omitempty"`
	Name string `json:"name,omitempty"`
}
type Address struct {
	Line1    string `json:"line1,omitempty"`
	Line2    string `json:"line2,omitempty"`
	TownCity string `json:"townCity,omitempty"`
	PostCode string `json:"postCode,omitempty"`
	State    string `json:"state,omitempty"`
	Country  string `json:"country,omitempty"`
}
type GeoLocation struct {
	Lat  float64 `json:"lat,omitempty"`
	Long float64 `json:"long,omitempty"`
}

func (s *LocationService) Discovery(ctx context.Context) (*EntryPoint, error) {
	var out EntryPoint
	err := s.client.do(ctx, s.client.BaseURLs.Location, locationAPIVersion, http.MethodGet, "/discovery", nil, nil, &out, nil)
	return &out, err
}
func (s *LocationService) Search(ctx context.Context, opt QueryOptions) (*LocationsResponse, error) {
	var out LocationsResponse
	err := s.client.do(ctx, s.client.BaseURLs.Location, locationAPIVersion, http.MethodGet, "/locations", opt.Values(), nil, &out, nil)
	return &out, err
}
func (s *LocationService) Get(ctx context.Context, serverState string) (*Location, error) {
	var out Location
	err := s.client.do(ctx, s.client.BaseURLs.Location, locationAPIVersion, http.MethodGet, "/locations/"+cleanElem(serverState), nil, nil, &out, nil)
	return &out, err
}
func (s *LocationService) Nearest(ctx context.Context, lat, long string, diameter, maxResults int, opt QueryOptions) (*LocationsResponse, error) {
	q := opt.Values()
	q.Set("latitude", lat)
	q.Set("longitude", long)
	if diameter > 0 {
		q.Set("diameter", fmt.Sprint(diameter))
	}
	if maxResults > 0 {
		q.Set("maxResults", fmt.Sprint(maxResults))
	}
	var out LocationsResponse
	err := s.client.do(ctx, s.client.BaseURLs.Location, locationAPIVersion, http.MethodGet, "/locations/nearest", q, nil, &out, nil)
	return &out, err
}
