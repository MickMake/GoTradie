# Changes

## v0.3

### Added

- Added `CHATGPT_CONTEXT.md` to document package boundaries, dependency rules, Go version, and local multi-repo development guidance.

## v0.2

### Added

- Added all-page list helpers for paginated Invoice Ninja exports and sync operations:
  - `Clients.ListAll`
  - `Products.ListAll`
  - `Quotes.ListAll`
  - `Invoices.ListAll`
  - `Payments.ListAll`
- Added generic `Service.ListAll` support.
- Added `ListOptions.Status` so callers can request active, archived, or deleted entities where Invoice Ninja supports status filters.
- Added pagination tests proving product and client exports fetch every page while preserving query parameters such as `status`, `per_page`, and product custom fields.

### Changed

- Updated package version metadata and user agent to `v0.2`.
- Documented that `List` returns one page and `ListAll` should be used for CSV export/import and sync jobs.

## v0.1.0

Initial development version for GoBunningsNinja integration work.

### Added

- Created the initial GoInvoiceNinja Go client package.
- Focused the public API on the endpoint groups currently needed:
  - clients
  - products
  - invoices
  - payments
  - quotes
- Added a context-aware HTTP client with Invoice Ninja API token authentication.
- Added hosted and self-hosted Invoice Ninja base URL support.
- Added typed services:
  - `ClientService`
  - `ProductService`
  - `InvoiceService`
  - `PaymentService`
  - `QuoteService`
- Added typed query structs for clients, products, invoices, payments, and quotes.
- Added product custom field support on models, create requests, update requests, list responses, and get responses.
- Added `Products.FindByBunningsIN`, using product `custom_value1` by convention.
- Documented `custom_value1` as the recommended canonical Bunnings item number / IN field.
- Added product max quantity support via `MaxQuantity` / `max_quantity`.
- Added product image URL support via `ProductImage` / `product_image`.
- Added stock-related product fields:
  - `InStockQuantity`
  - `StockNotification`
  - `StockNotificationThreshold`
- Added product document/image upload helpers for `/api/v1/products/{id}/upload`.
- Added concise validation-aware API error formatting.
- Added a `VERSION` file.
- Set the Go version to 1.22.

### Tested

- Added tests for creating and updating products with a Bunnings IN in `custom_value1`.
- Added tests for product list/get custom field population.
- Added tests for querying products by Bunnings IN.
- Added tests for product max quantity, stock fields, and product image URL round-tripping.
- Added tests for product document upload request construction and response decoding.
- Added tests for API error formatting:
  - message-only response
  - errors-only response
  - message plus errors response
  - empty-body response

### Validation

- `go test ./...` passes.
- `go vet ./...` passes.
