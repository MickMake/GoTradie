# GoInvoiceNinja

Version: **v0.2**

Idiomatic Go client for Invoice Ninja v5, focused on the endpoint groups needed by GoTradie:

- Clients
- Products
- Quotes
- Invoices
- Payments

The rest of Invoice Ninja's API remains available through `Raw`, so the public package stays small and reusable instead of becoming a hydra with a BAS statement.

## Requirements

- Go **1.22**
- Invoice Ninja v5 API token

## Features

- Token auth via `X-API-TOKEN`
- Hosted or self-hosted Invoice Ninja support
- Context-aware HTTP requests
- Typed CRUD services for clients, products, quotes, invoices, and payments
- Typed query structs for common filters
- Pagination, all-page helpers, and `include` relationship support
- Invoice and quote actions such as email/clone, plus invoice mark-sent
- Product custom fields for external identifiers
- Product max quantity and stock-related fields
- Product image URL field via `product_image`
- Product document/image upload helper via `/api/v1/products/{id}/upload`
- Concise, actionable API validation errors
- `Raw` escape hatch for less common endpoints

## Install

```bash
go get github.com/MickMake/GoInvoiceNinja
```

## Create a client

```go
ctx := context.Background()

c, err := goinvoiceninja.New(
    os.Getenv("INVOICE_NINJA_TOKEN"),
    goinvoiceninja.WithBaseURL(os.Getenv("INVOICE_NINJA_URL")), // optional
)
if err != nil {
    log.Fatal(err)
}
```

## Clients

```go
client, err := c.Clients.Create(ctx, goinvoiceninja.CreateClientRequest{
    Name: "MickMake Carpentry",
    Contacts: []goinvoiceninja.Contact{{
        FirstName: "Mick",
        Email:     "mick@example.com",
    }},
})

found, err := c.Clients.FindByEmail(ctx, "mick@example.com")
```

## Products

```go
product, err := c.Products.Create(ctx, goinvoiceninja.CreateProductRequest{
    ProductKey: "LABOUR-CARPENTRY",
    Notes:      "Carpentry labour",
    Price:      95,
    Quantity:   1,
    TaxName1:   "GST",
    TaxRate1:   10,
})

sameProduct, err := c.Products.FindByKey(ctx, "LABOUR-CARPENTRY")
```

### Bunnings product sync

Use `Product.CustomValue1` / `custom_value1` as the canonical Bunnings item number / IN storage field.

Keep `ProductKey` human-readable and stable for Invoice Ninja users, but do **not** depend on it as the only external identifier.

```go
bunningsProduct, err := c.Products.Create(ctx, goinvoiceninja.CreateProductRequest{
    ProductKey:   "BUN-0123456",
    Notes:        "Merbau decking oil",
    Price:        42.50,
    CustomValue1: "0123456", // Bunnings IN
})

existing, err := c.Products.FindByBunningsIN(ctx, "0123456")

products, err := c.Products.List(ctx, goinvoiceninja.ProductQuery{
    CustomValue1: "0123456",
})

allActiveProducts, err := c.Products.ListAll(ctx, goinvoiceninja.ProductQuery{
    ListOptions: goinvoiceninja.ListOptions{
        PerPage: 100,
        Status:  "active",
    },
})

updated, err := c.Products.Update(ctx, bunningsProduct.ID, goinvoiceninja.UpdateProductRequest{
    ProductKey:   bunningsProduct.ProductKey,
    Notes:        "Updated Bunnings description",
    Price:        44.95,
    CustomValue1: "0123456",
})
```

`Product`, `CreateProductRequest`, and `UpdateProductRequest` expose `CustomValue1` through `CustomValue4`. List and get responses populate these fields from Invoice Ninja, and `ProductQuery` supports filtering by them where the server accepts those query parameters.

If your Invoice Ninja instance does not filter directly by a custom value, use `ListOptions.Filter` as a fallback search value and then compare `Product.CustomValue1` client-side.

### Product max quantity and stock fields

Invoice Ninja v5 exposes product quantity and inventory-related fields. GoInvoiceNinja now models:

- `Quantity` / `quantity` — default product quantity
- `InStockQuantity` / `in_stock_quantity` — current stock quantity
- `StockNotification` / `stock_notification`
- `StockNotificationThreshold` / `stock_notification_threshold`
- `MaxQuantity` / `max_quantity` — maximum orderable quantity

Example:

```go
product, err := c.Products.Create(ctx, goinvoiceninja.CreateProductRequest{
    ProductKey:                 "BUN-0123456",
    Quantity:                   1,
    InStockQuantity:            25,
    StockNotification:          true,
    StockNotificationThreshold: 3,
    MaxQuantity:                10,
})
```

Invoice Ninja documents `in_stock_quantity` as requiring `?update_in_stock_quantity=true` when manually updating that value. Use `Raw` for that specialised update until a dedicated helper is needed.

### Product image URLs and documents

Invoice Ninja v5 exposes `product_image` as a product image URL. GoInvoiceNinja models this as `ProductImage` on:

- `Product`
- `CreateProductRequest`
- `UpdateProductRequest`

```go
product, err := c.Products.Create(ctx, goinvoiceninja.CreateProductRequest{
    ProductKey:   "BUN-0123456",
    ProductImage: "https://example.test/bunnings-image.jpg",
})
```

For GoTradie CSV workflows, the preferred storage order is:

1. Use `ProductImage` / `product_image` when your Invoice Ninja instance supports it.
2. Also allow the sync client's configurable custom-field fallback, defaulting to `custom_value2`, for compatibility with older or stricter instances.

Product documents/images can also be uploaded to Invoice Ninja using the product upload endpoint:

```go
product, err := c.Products.UploadDocumentFile(ctx, productID, "image.jpg")
```

For streaming content:

```go
product, err := c.Products.UploadDocument(ctx, productID, "image.jpg", reader)
```

`UploadDocument` uses the multipart form field `documents`. If your Invoice Ninja deployment expects a different field name, use `UploadDocumentWithField`.

## All-page list helpers

Invoice Ninja list endpoints are paginated. `List` returns one page; `ListAll` follows every available page and returns the combined slice. Use `ListAll` for CSV export/import and sync jobs where missing page two would cause the sort of trouble normally reserved for small goblins with clipboards.

Available helpers:

- `Clients.ListAll(ctx, ClientQuery{...})`
- `Products.ListAll(ctx, ProductQuery{...})`
- `Quotes.ListAll(ctx, QuoteQuery{...})`
- `Invoices.ListAll(ctx, InvoiceQuery{...})`
- `Payments.ListAll(ctx, PaymentQuery{...})`

If `PerPage` is not set, `ListAll` defaults it to `100`. Existing query values such as `status`, `include`, `filter`, custom product values, and typed resource filters are preserved on every page request.

Example product export:

```go
products, err := c.Products.ListAll(ctx, goinvoiceninja.ProductQuery{
    ListOptions: goinvoiceninja.ListOptions{
        PerPage: 100,
        Status:  "active",
    },
})
```

Example client export:

```go
clients, err := c.Clients.ListAll(ctx, goinvoiceninja.ClientQuery{
    ListOptions: goinvoiceninja.ListOptions{
        PerPage: 100,
        Status:  "active",
    },
})
```

## Quotes

```go
quote, err := c.Quotes.Create(ctx, goinvoiceninja.CreateQuoteRequest{
    ClientID: client.ID,
    Date:     "2026-05-17",
    DueDate:  "2026-05-31",
    LineItems: []goinvoiceninja.LineItem{{
        ProductKey: product.ProductKey,
        Notes:      "Deck repair estimate",
        Quantity:   8,
        Cost:       95,
        TaxName1:   "GST",
        TaxRate1:   10,
    }},
})

_, err = c.Quotes.Email(ctx, quote.ID)
```

## Invoices

```go
invoice, err := c.Invoices.Create(ctx, goinvoiceninja.CreateInvoiceRequest{
    ClientID: client.ID,
    Date:     "2026-05-17",
    DueDate:  "2026-05-31",
    Terms:    "Payment due within 14 days.",
    LineItems: []goinvoiceninja.LineItem{{
        ProductKey: "LABOUR-CARPENTRY",
        Notes:      "Deck repair",
        Quantity:   8,
        Cost:       95,
        TaxName1:   "GST",
        TaxRate1:   10,
    }},
})

payable, err := c.Invoices.List(ctx, goinvoiceninja.InvoiceQuery{
    ListOptions: goinvoiceninja.ListOptions{
        PerPage: 50,
        Include: []string{"client"},
    },
    Payable: true,
})

_, err = c.Invoices.MarkSent(ctx, invoice.ID)
_, err = c.Invoices.Email(ctx, invoice.ID)
```

## Payments

```go
payment, err := c.Payments.Create(ctx, goinvoiceninja.CreatePaymentRequest{
    ClientID:             client.ID,
    InvoiceID:            invoice.ID,
    Amount:               invoice.Balance,
    Date:                 "2026-05-17",
    TransactionReference: "bank-transfer-123",
    IsManual:             true,
})

payments, err := c.Payments.List(ctx, goinvoiceninja.PaymentQuery{
    ClientID: client.ID,
    ListOptions: goinvoiceninja.ListOptions{
        Include: []string{"client", "invoices"},
    },
})
```

## API errors

`APIError.Error()` includes the HTTP status code, the main message when provided, and validation details when Invoice Ninja returns an `errors` object.

Example shape:

```text
invoice ninja API error: status=422: validation failed: validation: custom_value1=The custom value is required.; price=The price must be numeric.
```

## Unsupported or rare endpoints

```go
var out map[string]any
err := c.Raw(ctx, http.MethodGet, "statics", nil, nil, &out)
```

## Validation

```bash
go test ./...
go vet ./...
```
