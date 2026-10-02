package goinvoiceninja

import (
	"strconv"
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

const expenseDocumentFormField = DefaultDocumentFormField + "[]"

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
	return uploadDocument[Product](ctx, s.client, http.MethodPost, s.path, id, fieldName, filename, nil, r)
}

// UploadDocument uploads a receipt or other document to an Invoice Ninja expense via
// PUT /api/v1/expenses/{id}/upload. Invoice Ninja validates documents as an array,
// so the multipart field is encoded as "documents[]".
func (s *ExpenseService) UploadDocument(ctx context.Context, id, filename string, r io.Reader) (*Expense, error) {
	return s.UploadDocumentWithField(ctx, id, expenseDocumentFormField, filename, r)
}

// UploadDocumentWithField uploads an expense document using a caller-supplied multipart form field.
func (s *ExpenseService) UploadDocumentWithField(ctx context.Context, id, fieldName, filename string, r io.Reader) (*Expense, error) {
	isPublic := false
	return uploadDocument[Expense](ctx, s.client, http.MethodPut, s.path, id, fieldName, filename, &isPublic, r)
}

func uploadDocument[T any](ctx context.Context, client *Client, method, servicePath, id, fieldName, filename string, isPublic *bool, r io.Reader) (*T, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if isPublic != nil {
		if err := mw.WriteField("is_public", strconv.FormatBool(*isPublic)); err != nil {
			return nil, err
		}
	}

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

	req, err := client.newMultipartRequest(ctx, method, actionPath(servicePath, id, "upload"), nil, &body, mw.FormDataContentType())
	if err != nil {
		return nil, err
	}
	raw, err := rawDo(client, req)
	if err != nil {
		return nil, err
	}
	entity, err := decodeEnvelope[T](raw)
	if err != nil {
		return nil, err
	}
	return &entity, nil
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

// UploadDocumentFile opens filename and uploads it to an expense.
func (s *ExpenseService) UploadDocumentFile(ctx context.Context, id, filename string) (*Expense, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return s.UploadDocument(ctx, id, filename, f)
}
