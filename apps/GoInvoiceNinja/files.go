package goinvoiceninja

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

const DefaultDocumentFormField = "documents"

func (c *Client) Download(ctx context.Context, path string, query url.Values, w io.Writer) error {
	req, err := c.NewRequest(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(resp.Body)
		return parseAPIError(resp.StatusCode, b)
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

func (c *Client) DownloadInvoicePDF(ctx context.Context, invoiceID, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return c.Download(ctx, joinPath("invoice", invoiceID, "download"), nil, f)
}

func (c *Client) newMultipartRequest(ctx context.Context, method, path string, query url.Values, body *bytes.Buffer, contentType string) (*http.Request, error) {
	rel := &url.URL{Path: path}
	if query != nil {
		rel.RawQuery = query.Encode()
	}
	u := c.baseURL.ResolveReference(rel)
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-API-TOKEN", c.token)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return req, nil
}

// UploadDocument uploads a document or image file to an Invoice Ninja product via
// POST /api/v1/products/{id}/upload and returns the updated product object.
func (s *ProductService) UploadDocument(ctx context.Context, id, filename string, r io.Reader) (*Product, error) {
	return s.UploadDocumentWithField(ctx, id, DefaultDocumentFormField, filename, r)
}

// UploadDocumentWithField uploads a product document using a caller-supplied multipart form field.
// The default field used by UploadDocument is "documents".
func (s *ProductService) UploadDocumentWithField(ctx context.Context, id, fieldName, filename string, r io.Reader) (*Product, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile(fieldName, filepath.Base(filename))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, r); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	req, err := s.client.newMultipartRequest(ctx, http.MethodPost, actionPath(s.path, id, "upload"), nil, &body, mw.FormDataContentType())
	if err != nil {
		return nil, err
	}
	raw, err := rawDo(s.client, req)
	if err != nil {
		return nil, err
	}
	product, err := decodeEnvelope[Product](raw)
	if err != nil {
		return nil, err
	}
	return &product, nil
}

// UploadDocumentFile opens filename and uploads it to a product.
func (s *ProductService) UploadDocumentFile(ctx context.Context, id, filename string) (*Product, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return s.UploadDocument(ctx, id, filename, f)
}
