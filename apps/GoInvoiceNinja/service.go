package goinvoiceninja

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

type Service[T any] struct {
	client *Client
	path   string
}

func NewService[T any](c *Client, path string) *Service[T] { return &Service[T]{client: c, path: path} }

type ListOptions struct {
	Page    int
	PerPage int
	Include []string
	Sort    string
	Filter  string
	Status  string
	Params  url.Values
}

func (o ListOptions) Values() url.Values {
	q := url.Values{}
	if o.Params != nil {
		for k, vv := range o.Params {
			for _, v := range vv {
				q.Add(k, v)
			}
		}
	}
	if o.Page > 0 {
		q.Set("page", itoa(o.Page))
	}
	if o.PerPage > 0 {
		q.Set("per_page", itoa(o.PerPage))
	}
	if len(o.Include) > 0 {
		q.Set("include", joinComma(o.Include))
	}
	if o.Sort != "" {
		q.Set("sort", o.Sort)
	}
	if o.Filter != "" {
		q.Set("filter", o.Filter)
	}
	if o.Status != "" {
		q.Set("status", o.Status)
	}
	return q
}

type ListResult[T any] struct {
	Data []T
	Meta Meta
}

func (s *Service[T]) List(ctx context.Context, opts *ListOptions) (*ListResult[T], error) {
	q := url.Values{}
	if opts != nil {
		q = opts.Values()
	}
	req, err := s.client.NewRequest(ctx, http.MethodGet, s.path, q, nil)
	if err != nil {
		return nil, err
	}
	raw, err := rawDo(s.client, req)
	if err != nil {
		return nil, err
	}
	data, meta, err := decodeListEnvelope[T](raw)
	if err != nil {
		return nil, err
	}
	return &ListResult[T]{Data: data, Meta: meta}, nil
}

// ListAll fetches every available page for the service, preserving the supplied
// list options. If PerPage is not set, it defaults to 100 to reduce round trips.
func (s *Service[T]) ListAll(ctx context.Context, opts *ListOptions) ([]T, error) {
	q := url.Values{}
	if opts != nil {
		q = opts.Values()
	}
	return listAllWithValues[T](ctx, s.client, s.path, q)
}

func (s *Service[T]) Get(ctx context.Context, id string, include ...string) (*T, error) {
	q := url.Values{}
	if len(include) > 0 {
		q.Set("include", joinComma(include))
	}
	req, err := s.client.NewRequest(ctx, http.MethodGet, addID(s.path, id), q, nil)
	if err != nil {
		return nil, err
	}
	raw, err := rawDo(s.client, req)
	if err != nil {
		return nil, err
	}
	item, err := decodeEnvelope[T](raw)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service[T]) Create(ctx context.Context, payload any) (*T, error) {
	req, err := s.client.NewRequest(ctx, http.MethodPost, s.path, nil, payload)
	if err != nil {
		return nil, err
	}
	raw, err := rawDo(s.client, req)
	if err != nil {
		return nil, err
	}
	item, err := decodeEnvelope[T](raw)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service[T]) Update(ctx context.Context, id string, payload any) (*T, error) {
	req, err := s.client.NewRequest(ctx, http.MethodPut, addID(s.path, id), nil, payload)
	if err != nil {
		return nil, err
	}
	raw, err := rawDo(s.client, req)
	if err != nil {
		return nil, err
	}
	item, err := decodeEnvelope[T](raw)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service[T]) Delete(ctx context.Context, id string) error {
	req, err := s.client.NewRequest(ctx, http.MethodDelete, addID(s.path, id), nil, nil)
	if err != nil {
		return err
	}
	return s.client.Do(req, nil)
}

func (s *Service[T]) Archive(ctx context.Context, id string) (*T, error) {
	return s.Action(ctx, id, "archive", nil)
}
func (s *Service[T]) Restore(ctx context.Context, id string) (*T, error) {
	return s.Action(ctx, id, "restore", nil)
}

func (s *Service[T]) Action(ctx context.Context, id, action string, payload any) (*T, error) {
	req, err := s.client.NewRequest(ctx, http.MethodPost, actionPath(s.path, id, action), nil, payload)
	if err != nil {
		return nil, err
	}
	raw, err := rawDo(s.client, req)
	if err != nil {
		return nil, err
	}
	item, err := decodeEnvelope[T](raw)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func listAllWithValues[T any](ctx context.Context, c *Client, path string, q url.Values) ([]T, error) {
	query := cloneValues(q)
	perPage := intFromQuery(query.Get("per_page"))
	if perPage <= 0 {
		perPage = 100
		query.Set("per_page", strconv.Itoa(perPage))
	}
	page := intFromQuery(query.Get("page"))
	if page <= 0 {
		page = 1
	}

	var all []T
	for {
		query.Set("page", strconv.Itoa(page))
		result, err := listWithQuery[T](ctx, c, path, query)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Data...)

		if len(result.Data) == 0 {
			break
		}
		if result.Meta.LastPage > 0 {
			current := result.Meta.CurrentPage
			if current <= 0 {
				current = page
			}
			if current >= result.Meta.LastPage {
				break
			}
		} else if result.Meta.Total > 0 && result.Meta.To > 0 {
			if result.Meta.To >= result.Meta.Total {
				break
			}
		} else if len(result.Data) < perPage {
			break
		}
		page++
	}
	return all, nil
}

func cloneValues(src url.Values) url.Values {
	dst := url.Values{}
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
	return dst
}

func intFromQuery(value string) int {
	if value == "" {
		return 0
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}

func (s *Service[T]) Endpoint() string { return s.path }
