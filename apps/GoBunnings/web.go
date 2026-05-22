package gobunnings

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type WebsiteService struct {
	BaseURL string
	Client  *http.Client
}

func NewWebsiteService(client *http.Client) *WebsiteService {
	if client == nil {
		client = http.DefaultClient
	}
	return &WebsiteService{BaseURL: "https://www.bunnings.com.au", Client: client}
}

var (
	productHrefRE = regexp.MustCompile(`href="([^"]+_p(\d{5,}))"`)
	titleRE       = regexp.MustCompile(`title="([^"]+)"`)
	ldJSONRE      = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
)

type WebsiteProduct struct {
	ItemNumber  string
	Title       string
	Description string
	Unit        string
	ImageURL    string
	Price       float64
}

func (s *WebsiteService) Search(ctx context.Context, query string, limit int) ([]WebsiteProduct, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search query is required")
	}
	if limit <= 0 {
		limit = 10
	}
	searchURL := s.BaseURL + "/search/products?q=" + url.QueryEscape(query)
	body, err := s.fetch(ctx, searchURL)
	if err != nil {
		return nil, err
	}
	matches := productHrefRE.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("website search returned no products for query %q", query)
	}
	out := make([]WebsiteProduct, 0, limit)
	seen := map[string]bool{}
	for _, m := range matches {
		if len(out) >= limit {
			break
		}
		in := m[2]
		if seen[in] {
			continue
		}
		seen[in] = true
		t := ""
		if tt := titleRE.FindStringSubmatch(m[0]); len(tt) > 1 {
			t = htmlUnescape(tt[1])
		}
		out = append(out, WebsiteProduct{ItemNumber: in, Title: t})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("website search returned no usable products for query %q", query)
	}
	return out, nil
}

func (s *WebsiteService) Get(ctx context.Context, itemNumber string) (WebsiteProduct, error) {
	itemNumber = strings.TrimSpace(itemNumber)
	if itemNumber == "" {
		return WebsiteProduct{}, fmt.Errorf("Bunnings item number is required")
	}
	searchURL := s.BaseURL + "/search/products?q=" + url.QueryEscape(itemNumber)
	body, err := s.fetch(ctx, searchURL)
	if err != nil {
		return WebsiteProduct{}, err
	}
	m := regexp.MustCompile(`href="([^"]+_p` + regexp.QuoteMeta(itemNumber) + `)"`).FindStringSubmatch(body)
	if len(m) < 2 {
		return WebsiteProduct{}, fmt.Errorf("website lookup failed for item %s", itemNumber)
	}
	prodBody, err := s.fetch(ctx, s.BaseURL+m[1])
	if err != nil {
		return WebsiteProduct{}, err
	}
	p, err := parseLDJSONProduct(prodBody)
	if err != nil {
		return WebsiteProduct{}, fmt.Errorf("website product parse failed for item %s: %w", itemNumber, err)
	}
	if p.ItemNumber == "" {
		p.ItemNumber = itemNumber
	}
	return p, nil
}

func parseLDJSONProduct(body string) (WebsiteProduct, error) {
	for _, m := range ldJSONRE.FindAllStringSubmatch(body, -1) {
		var v map[string]any
		if json.Unmarshal([]byte(m[1]), &v) != nil {
			continue
		}
		t, _ := v["@type"].(string)
		if !strings.EqualFold(t, "Product") {
			continue
		}
		p := WebsiteProduct{Title: getString(v, "name"), Description: getString(v, "description")}
		if sku := getString(v, "sku"); sku != "" {
			p.ItemNumber = sku
		}
		if img, ok := v["image"].(string); ok {
			p.ImageURL = img
		}
		if offers, ok := v["offers"].(map[string]any); ok {
			if price, _ := strconv.ParseFloat(getString(offers, "price"), 64); price > 0 {
				p.Price = price
			}
		}
		if p.Title == "" && p.Description == "" && p.ItemNumber == "" {
			continue
		}
		return p, nil
	}
	return WebsiteProduct{}, fmt.Errorf("no Product structured data found")
}

func getString(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
func htmlUnescape(v string) string {
	r := strings.NewReplacer("&amp;", "&", "&quot;", "\"", "&#39;", "'", "&lt;", "<", "&gt;", ">")
	return r.Replace(v)
}
func (s *WebsiteService) fetch(ctx context.Context, u string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("User-Agent", "GoBunnings/website")
	resp, err := s.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("website request failed: %s", resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
