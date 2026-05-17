// Package gobunnings provides a reusable Go SDK for the Bunnings API suite.
//
// The package is intentionally limited to API-client responsibilities: auth,
// HTTP transport, typed request/response models, pagination helpers, HATEOAS
// links, retry handling, and domain service wrappers.
package gobunnings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

type Env string

const (
	EnvSandbox Env = "sandbox"
	EnvTest    Env = "test"
	EnvLive    Env = "live"
)

type CountryCode string

const (
	CountryAU CountryCode = "AU"
	CountryNZ CountryCode = "NZ"
)

type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

type TokenSourceFunc func(ctx context.Context) (string, error)

func (f TokenSourceFunc) Token(ctx context.Context) (string, error) { return f(ctx) }

type Client struct {
	HTTP        *http.Client
	TokenSource TokenSource
	BaseURLs    BaseURLs
	UserAgent   string
	Retry       RetryConfig

	Item      *ItemService
	Location  *LocationService
	Inventory *InventoryService
	Pricing   *PricingService
	Order     *OrderService
}

type BaseURLs struct {
	Item      string
	Location  string
	Inventory string
	Pricing   string
	Ordering  string
}

func DefaultBaseURLs(env Env) (BaseURLs, error) {
	sub := ""
	switch env {
	case EnvSandbox:
		sub = ".sandbox"
	case EnvTest:
		sub = ".stg"
	case EnvLive:
	default:
		return BaseURLs{}, fmt.Errorf("unknown environment %q", env)
	}
	return BaseURLs{
		Item:      fmt.Sprintf("https://item%s.api.bunnings.com.au/item", sub),
		Location:  fmt.Sprintf("https://location%s.api.bunnings.com.au/location", sub),
		Inventory: fmt.Sprintf("https://inventory%s.api.bunnings.com.au/inventory", sub),
		Pricing:   fmt.Sprintf("https://pricing%s.api.bunnings.com.au/pricing", sub),
		Ordering:  fmt.Sprintf("https://ordering%s.api.bunnings.com.au/ordering", sub),
	}, nil
}

type RetryConfig struct {
	MaxAttempts int
	MinWait     time.Duration
	MaxWait     time.Duration
	Statuses    map[int]bool
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		MinWait:     250 * time.Millisecond,
		MaxWait:     3 * time.Second,
		Statuses: map[int]bool{
			http.StatusTooManyRequests:     true,
			http.StatusInternalServerError: true,
			http.StatusBadGateway:          true,
			http.StatusServiceUnavailable:  true,
			http.StatusGatewayTimeout:      true,
		},
	}
}

type Option func(*Client)

func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.HTTP = h } }
func WithBaseURLs(b BaseURLs) Option       { return func(c *Client) { c.BaseURLs = b } }
func WithUserAgent(ua string) Option       { return func(c *Client) { c.UserAgent = ua } }
func WithRetry(r RetryConfig) Option       { return func(c *Client) { c.Retry = r } }
func WithoutRetry() Option                 { return func(c *Client) { c.Retry.MaxAttempts = 1 } }

func New(env Env, ts TokenSource, opts ...Option) (*Client, error) {
	if ts == nil {
		return nil, errors.New("token source is required")
	}
	base, err := DefaultBaseURLs(env)
	if err != nil {
		return nil, err
	}
	c := &Client{
		HTTP:        &http.Client{Timeout: 30 * time.Second},
		TokenSource: ts,
		BaseURLs:    base,
		UserAgent:   "GoBunnings/0.4",
		Retry:       DefaultRetryConfig(),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	if c.Retry.MaxAttempts <= 0 {
		c.Retry.MaxAttempts = 1
	}
	c.Item = &ItemService{client: c}
	c.Location = &LocationService{client: c}
	c.Inventory = &InventoryService{client: c}
	c.Pricing = &PricingService{client: c}
	c.Order = &OrderService{client: c}
	return c, nil
}

type QueryOptions struct {
	Select            string
	OrderBy           string
	Filter            string
	Skip              *int
	Top               *int
	FullPage          *bool
	ContinuationToken string
}

func (o QueryOptions) Values() url.Values {
	v := url.Values{}
	if o.Select != "" {
		v.Set("$select", o.Select)
	}
	if o.OrderBy != "" {
		v.Set("$orderby", o.OrderBy)
	}
	if o.Filter != "" {
		v.Set("$filter", o.Filter)
	}
	if o.Skip != nil {
		v.Set("$skip", strconv.Itoa(*o.Skip))
	}
	if o.Top != nil {
		v.Set("$top", strconv.Itoa(*o.Top))
	}
	if o.FullPage != nil {
		v.Set("$fullPage", strconv.FormatBool(*o.FullPage))
	}
	if o.ContinuationToken != "" {
		v.Set("continuationToken", o.ContinuationToken)
	}
	return v
}

type APIError struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Problem    *ProblemDetails
}

func (e *APIError) Error() string {
	if e.Problem != nil && (e.Problem.Title != "" || e.Problem.Detail != "") {
		return fmt.Sprintf("bunnings API error: status=%d title=%q detail=%q", e.StatusCode, e.Problem.Title, e.Problem.Detail)
	}
	return fmt.Sprintf("bunnings API error: status=%d body=%s", e.StatusCode, strings.TrimSpace(string(e.Body)))
}

func (e *APIError) RetryAfter() time.Duration {
	if e == nil || e.Header == nil {
		return 0
	}
	if v := e.Header.Get("Retry-After"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Second
		}
		if t, err := http.ParseTime(v); err == nil {
			return time.Until(t)
		}
	}
	return 0
}

func (c *Client) do(ctx context.Context, baseURL, apiVersion, method, p string, query url.Values, body any, out any, header http.Header) error {
	attempts := c.Retry.MaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		err := c.doOnce(ctx, baseURL, apiVersion, method, p, query, body, out, header)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt == attempts || !c.shouldRetry(err) {
			return err
		}
		wait := c.retryWait(err, attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return lastErr
}

func (c *Client) doOnce(ctx context.Context, baseURL, apiVersion, method, p string, query url.Values, body any, out any, header http.Header) error {
	tok, err := c.TokenSource.Token(ctx)
	if err != nil {
		return fmt.Errorf("get token: %w", err)
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(p, "/")
	if query != nil {
		u.RawQuery = query.Encode()
	}
	var r io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		r = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), r)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-version-api", apiVersion)
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, vals := range header {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		apiErr := &APIError{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Body: data}
		var pd ProblemDetails
		if json.Unmarshal(data, &pd) == nil && (pd.Title != "" || pd.Detail != "" || pd.Status != 0 || len(pd.Errors) > 0) {
			apiErr.Problem = &pd
		}
		return apiErr
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode response: %w: %s", err, string(data))
	}
	return nil
}

func (c *Client) shouldRetry(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return c.Retry.Statuses[apiErr.StatusCode]
	}
	return true
}

func (c *Client) retryWait(err error, attempt int) time.Duration {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if d := apiErr.RetryAfter(); d > 0 {
			return minDuration(d, c.Retry.MaxWait)
		}
	}
	minW := c.Retry.MinWait
	maxW := c.Retry.MaxWait
	if minW <= 0 {
		minW = 250 * time.Millisecond
	}
	if maxW <= 0 {
		maxW = 3 * time.Second
	}
	wait := minW * time.Duration(1<<(attempt-1))
	if wait > maxW {
		wait = maxW
	}
	jitter := time.Duration(rand.Int63n(int64(wait / 2)))
	return wait/2 + jitter
}

func minDuration(a, b time.Duration) time.Duration {
	if b <= 0 || a < b {
		return a
	}
	return b
}
func joinCSV(vals []string) string { return strings.Join(vals, ",") }
func cleanElem(s string) string    { return path.Clean("/" + s)[1:] }
func Int(v int) *int               { return &v }
func Bool(v bool) *bool            { return &v }
func Float64(v float64) *float64   { return &v }
