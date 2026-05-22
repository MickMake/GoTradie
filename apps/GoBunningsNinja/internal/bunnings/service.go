package bunnings

import (
	"context"
	"fmt"
	"strings"

	gobunnings "github.com/MickMake/GoBunnings"
	"github.com/MickMake/GoBunningsNinja/internal/config"
)

type Product struct {
	ItemNumber  string
	Title       string
	Description string
	Unit        string
	ImageURL    string
	Price       float64
	RawItem     *gobunnings.Item
}

type Service struct {
	client   *gobunnings.Client
	website  *gobunnings.WebsiteService
	useWeb   bool
	country  gobunnings.CountryCode
	location string
}

func New(cfg config.Config) (*Service, error) {
	ts, err := gobunnings.NewClientCredentialsTokenSource(cfg.BunningsEnv, cfg.BunningsClientID, cfg.BunningsSecret, cfg.BunningsScopes)
	if err != nil {
		return nil, err
	}
	client, err := gobunnings.New(cfg.BunningsEnv, ts, gobunnings.WithUserAgent("GoBunningsNinja/v0.3"))
	if err != nil {
		return nil, err
	}
	return &Service{client: client, website: gobunnings.NewWebsiteService(client.HTTP), country: cfg.Country, location: cfg.LocationCode}, nil
}

func (s *Service) WithWeb(enabled bool) *Service {
	s.useWeb = enabled
	return s
}

func (s *Service) GetProduct(ctx context.Context, itemNumber string) (Product, error) {
	itemNumber = strings.TrimSpace(itemNumber)
	if itemNumber == "" {
		return Product{}, fmt.Errorf("Bunnings item number is required")
	}
	if s.useWeb {
		wp, err := s.website.Get(ctx, itemNumber)
		if err != nil {
			return Product{}, err
		}
		return Product{ItemNumber: wp.ItemNumber, Title: wp.Title, Description: wp.Description, Unit: wp.Unit, ImageURL: wp.ImageURL, Price: wp.Price}, nil
	}
	item, err := s.client.Item.Detail(ctx, s.country, itemNumber, gobunnings.QueryOptions{})
	if err != nil {
		return Product{}, err
	}
	p := productFromItem(item)
	if p.ItemNumber == "" {
		p.ItemNumber = itemNumber
	}
	price, err := s.price(ctx, p.ItemNumber)
	if err == nil {
		p.Price = price
	}
	return p, nil
}

func (s *Service) Search(ctx context.Context, query string, limit int) ([]Product, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search query is required")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 25 {
		return nil, fmt.Errorf("search import guardrail: limit must be 25 or less")
	}
	if s.useWeb {
		rows, err := s.website.Search(ctx, query, limit)
		if err != nil {
			return nil, err
		}
		products := make([]Product, 0, len(rows))
		for _, r := range rows {
			products = append(products, Product{ItemNumber: r.ItemNumber, Title: r.Title, Description: r.Description, Unit: r.Unit, ImageURL: r.ImageURL, Price: r.Price})
		}
		return products, nil
	}
	resp, err := s.client.Item.Search(ctx, s.country, gobunnings.ItemSearchRequest{Query: query}, "")
	if err != nil {
		return nil, err
	}
	products := make([]Product, 0, min(limit, len(resp.Results)))
	for _, r := range resp.Results {
		if len(products) >= limit {
			break
		}
		p := Product{ItemNumber: strings.TrimSpace(r.ItemNumber), Title: strings.TrimSpace(r.Title)}
		products = append(products, p)
	}
	return products, nil
}

func (s *Service) Hydrate(ctx context.Context, p Product) (Product, error) {
	if p.ItemNumber == "" {
		return p, fmt.Errorf("Bunnings item number is required")
	}
	return s.GetProduct(ctx, p.ItemNumber)
}

func (s *Service) price(ctx context.Context, itemNumber string) (float64, error) {
	if s.location == "" {
		return 0, nil
	}
	out, err := s.client.Pricing.CatalogPrices(ctx, gobunnings.CatalogPriceRequest{
		Context: gobunnings.PriceContext{Country: s.country, Location: s.location},
		Items:   []gobunnings.PriceItem{{ItemNumber: itemNumber}},
	}, gobunnings.QueryOptions{})
	if err != nil {
		return 0, err
	}
	if len(out.Prices) == 0 {
		return 0, nil
	}
	if out.Prices[0].UnitPrice != 0 {
		return out.Prices[0].UnitPrice, nil
	}
	return out.Prices[0].LineUnitPrice, nil
}

func productFromItem(item *gobunnings.Item) Product {
	if item == nil {
		return Product{}
	}
	desc := strings.TrimSpace(item.Description.ProductDescription)
	if desc == "" {
		desc = strings.TrimSpace(item.Description.SectionDescription)
	}
	image := strings.TrimSpace(item.ImageURL)
	if image == "" {
		image = imageURL(item.Raw)
	}
	return Product{
		ItemNumber:  strings.TrimSpace(item.ItemNumber),
		Title:       strings.TrimSpace(item.Description.SectionDescription),
		Description: desc,
		Unit:        strings.TrimSpace(item.SaleUnitOfMeasure),
		ImageURL:    image,
		RawItem:     item,
	}
}

func imageURL(raw map[string]any) string {
	if raw == nil {
		return ""
	}
	keys := []string{"imageUrl", "imageURL", "image", "primaryImage", "productImageUrl", "productImageURL"}
	for _, k := range keys {
		if v, ok := raw[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
