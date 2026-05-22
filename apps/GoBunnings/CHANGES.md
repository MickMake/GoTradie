
- Added a reusable website-backed Bunnings retrieval helper (`WebsiteService`) for item lookup and search without API fallback.
# GoBunnings Changes

## v0.5 - Workspace context documentation

- Added `CHATGPT_CONTEXT.md` to document package boundaries, dependency rules, Go version, and local multi-repo development guidance.

## v0.4 - Sync-client compatibility improvements

- Preserved `Item.Raw` and `Item.ImageURL` support from prior versions for GoBunningsNinja product syncing.
- Added `ProblemDetails.UnmarshalJSON` tests proving RFC7807 extension fields are captured in `ProblemDetails.Extra`.
- Added API error coverage proving problem extension fields survive through HTTP error responses.
- Removed the artificial `url.Values{}` import keepalive from `item.go` and cleaned imports.
- Verified `Barcode.AdditionalDescription` uses the capitalized `AdditionalDescription` JSON key used by the Bunnings Query Item OpenAPI schema and response examples.
- Added a regression test for `Barcode.AdditionalDescription` decoding.
- Updated the default user agent to `GoBunnings/0.4`.
- Verified with `go test ./...` and `go vet ./...` on Go 1.22.

## v0.3 - Typed item image URL support

- Added `Item.ImageURL` as a backwards-compatible field for downstream clients that need a product image URL.
- Populates `Item.ImageURL` from Bunnings media JSON using `enrichedItem.picture.primaryAssetURL` as the preferred source.
- Added fallback image extraction from related media fields such as `enrichedItem.otherImages`.
- Kept `Item.Raw` populated as a fallback for unmodelled product fields.
- Added tests for products with a primary image, products with fallback images, and products without images.
- Verified with `go test ./...` on Go 1.22.

## v0.2 - Raw JSON preservation for items

- Added custom `UnmarshalJSON` support for `Item`.
- `Item.Raw` now preserves the decoded original product/item JSON object.
- Existing typed fields continue to unmarshal normally.
- Added tests proving `Raw` is populated through direct unmarshalling and item detail responses.
- Preserved public API compatibility.

## v0.1 - Initial reusable SDK scaffold

- Created reusable Go SDK/API client structure for Bunnings APIs.
- Added shared HTTP client, auth/token support, common request handling, retry configuration, and error handling.
- Added typed service areas for item/product, pricing, inventory, location, and order status APIs.
- Added initial tests and basic usage example.
- Set module target to Go 1.22.
