// Package goinvoiceninja provides an idiomatic Go client for the Invoice Ninja v5 API.
package goinvoiceninja

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://invoicing.co"
	DefaultAPIPath = "/api/v1"
)

// Client is an Invoice Ninja API client focused on the day-to-day sales workflow:
// clients, products, quotes, invoices, and payments.
type Client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
	userAgent  string

	Clients  *ClientService
	Products *ProductService
	Quotes   *QuoteService
	Invoices *InvoiceService
	Payments *PaymentService
}

// Option customises the client.
type Option func(*Client) error

func WithBaseURL(raw string) Option {
	return func(c *Client) error {
		if raw == "" {
			return errors.New("base URL cannot be empty")
		}
		u, err := url.Parse(strings.TrimRight(raw, "/"))
		if err != nil {
			return err
		}
		if !strings.HasSuffix(u.Path, DefaultAPIPath) {
			u.Path = strings.TrimRight(u.Path, "/") + DefaultAPIPath
		}
		u.Path = strings.TrimRight(u.Path, "/") + "/"
		c.baseURL = u
		return nil
	}
}

func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) error {
		if h == nil {
			return errors.New("http client cannot be nil")
		}
		c.httpClient = h
		return nil
	}
}

func WithUserAgent(ua string) Option {
	return func(c *Client) error {
		c.userAgent = ua
		return nil
	}
}

func New(token string, opts ...Option) (*Client, error) {
	if token == "" {
		return nil, errors.New("Invoice Ninja API token is required")
	}
	u, _ := url.Parse(DefaultBaseURL + DefaultAPIPath + "/")
	c := &Client{
		baseURL:    u,
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  "GoInvoiceNinja/v0.2",
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	c.Clients = &ClientService{Service: NewService[ClientEntity](c, "clients")}
	c.Products = &ProductService{Service: NewService[Product](c, "products")}
	c.Quotes = &QuoteService{Service: NewService[Quote](c, "quotes")}
	c.Invoices = &InvoiceService{Service: NewService[Invoice](c, "invoices")}
	c.Payments = &PaymentService{Service: NewService[Payment](c, "payments")}
	return c, nil
}

func (c *Client) NewRequest(ctx context.Context, method, path string, query url.Values, body any) (*http.Request, error) {
	rel := &url.URL{Path: strings.TrimLeft(path, "/")}
	if query != nil {
		rel.RawQuery = query.Encode()
	}
	u := c.baseURL.ResolveReference(rel)
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-TOKEN", c.token)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return req, nil
}

func (c *Client) Do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return parseAPIError(resp.StatusCode, data)
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

func parseAPIError(status int, data []byte) error {
	trimmed := bytes.TrimSpace(data)
	var e APIError
	if len(trimmed) > 0 && json.Unmarshal(trimmed, &e) == nil {
		e.StatusCode = status
		e.Raw = append(json.RawMessage(nil), trimmed...)
		if e.Message != "" || len(e.Errors) > 0 {
			return &e
		}
	}
	return &APIError{StatusCode: status, Message: string(trimmed), Raw: append(json.RawMessage(nil), trimmed...)}
}

// Raw performs a custom request for Invoice Ninja endpoints not yet wrapped by a typed service.
func (c *Client) Raw(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	req, err := c.NewRequest(ctx, method, path, query, body)
	if err != nil {
		return err
	}
	return c.Do(req, out)
}

func joinPath(parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, "/")
		if p != "" {
			clean = append(clean, p)
		}
	}
	return strings.Join(clean, "/")
}

func addID(path, id string) string              { return joinPath(path, url.PathEscape(id)) }
func actionPath(path, id, action string) string { return joinPath(path, url.PathEscape(id), action) }

func decodeEnvelope[T any](raw json.RawMessage) (T, error) {
	var zero T
	var env struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && len(raw) > 0 {
		var probe map[string]json.RawMessage
		_ = json.Unmarshal(raw, &probe)
		if _, ok := probe["data"]; ok {
			return env.Data, nil
		}
	}
	if err := json.Unmarshal(raw, &zero); err != nil {
		return zero, err
	}
	return zero, nil
}

func decodeListEnvelope[T any](raw json.RawMessage) ([]T, Meta, error) {
	var env struct {
		Data []T  `json:"data"`
		Meta Meta `json:"meta"`
	}
	if err := json.Unmarshal(raw, &env); err == nil {
		var probe map[string]json.RawMessage
		_ = json.Unmarshal(raw, &probe)
		if _, ok := probe["data"]; ok {
			return env.Data, env.Meta, nil
		}
	}
	var items []T
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, Meta{}, err
	}
	return items, Meta{}, nil
}

func rawDo(c *Client, req *http.Request) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.Do(req, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func String(v string) *string    { return &v }
func Float64(v float64) *float64 { return &v }
func Bool(v bool) *bool          { return &v }
func Int(v int) *int             { return &v }

var _ = fmt.Sprintf
