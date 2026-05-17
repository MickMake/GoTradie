# Weird stuff and upstream prompts

These are the remaining upstream package issues or questions noticed while building `GoBunningsNinja v0.3`.

Future zip files should keep this file updated and the relevant package prompts should be merged into one copy/paste prompt per package.

## GoBunnings prompt

Copy/paste this into the GoBunnings package chat:

```text
Please make the following GoBunnings package improvements needed by the GoBunningsNinja sync client.

Context:
GoBunningsNinja needs to sync Bunnings product data into Invoice Ninja. It needs reliable access to product identifiers, pricing, descriptions, URLs, image URLs, and any unmodelled JSON fields.

Please implement these changes cleanly without breaking the public API.

Requirements:

1. Preserve raw product/item JSON on Item

Currently Item.Raw is tagged json:"-", so it is never populated.

Please implement a custom UnmarshalJSON on Item, or an equivalent approach, so:
- Normal typed fields such as IN, Name, Description, Price, URL, etc. still unmarshal as before.
- Raw is populated with the original decoded JSON object.
- Raw contains fields that are not yet modelled.
- Existing tests still pass.
- Add tests proving Raw is populated.

2. Add typed image URL support to product/item models

Please inspect actual Bunnings product JSON and expose the best available image URL field as Item.ImageURL or equivalent.

Requirements:
- Preserve backwards compatibility.
- Add ImageURL to the item/product model.
- Populate it from the correct Bunnings JSON field.
- Keep Raw populated as a fallback.
- Add tests for products with and without images.

3. Populate ProblemDetails.Extra

Please add custom UnmarshalJSON support for ProblemDetails so unknown/problem-extension fields are captured into Extra.

Requirements:
- Keep known fields Type, Title, Status, Detail, Instance, and Errors as typed fields.
- Capture RFC7807-style extension fields into Extra.
- Add tests for extension fields.

4. Remove artificial import keepalive lines

Please remove artificial import keepalive lines such as:

var _ = url.Values{}
var _ = fmt.Sprintf

Clean up the affected imports afterwards.

5. Verify Barcode.AdditionalDescription JSON casing

Barcode.AdditionalDescription currently uses:

json:"AdditionalDescription,omitempty"

Please verify this against actual Bunnings API responses.

If the API uses additionalDescription:
- Change the tag to json:"additionalDescription,omitempty".
- Add a regression test proving it decodes correctly.

If the current casing is correct:
- Add a small test or comment explaining why.

6. Validation

Please run:

go test ./...
go vet ./...

If go vet reports existing unrelated issues, document them rather than hiding them under the carpet where they can breed.
```

## GoInvoiceNinja prompt

Copy/paste this into the GoInvoiceNinja package chat:

```text
Please make the following GoInvoiceNinja package improvements needed by the GoBunningsNinja sync client.

Context:
GoBunningsNinja exports/imports Invoice Ninja products and clients as CSV, exports quotes, invoices, and payments, syncs Bunnings product data into Invoice Ninja products, and safely maps external Bunnings item numbers / INs to Invoice Ninja records.

GoBunningsNinja now expects the v0.2 API additions already made, especially:
- Products.ListAll(ctx, ProductQuery{...})
- Clients.ListAll(ctx, ClientQuery{...})
- Quotes.ListAll(ctx, QuoteQuery{...})
- Invoices.ListAll(ctx, InvoiceQuery{...})
- Payments.ListAll(ctx, PaymentQuery{...})
- Service.ListAll(ctx, query)
- ListOptions.Status string

Please keep those APIs stable.

Additional requirements:

1. Investigate product max quantity support

GoBunningsNinja product CSV includes:

ID, Product, Description, Price, Default Quantity, Max Quantity, Image URL

The package does not currently appear to expose a max quantity field for products.

Please investigate whether Invoice Ninja v5 products support a max quantity, stock limit, inventory quantity limit, or equivalent field through the API.

If supported:
- Update Product, CreateProductRequest, and UpdateProductRequest to expose the correct field with the correct JSON tag.
- Add tests proving list/get/create/update round-trip the value.

If unsupported:
- Document that explicitly in the README so downstream CSV tooling can leave the Max Quantity column read-only or ignored.

2. Investigate product image support

GoBunningsNinja needs to sync Bunnings product image URLs into Invoice Ninja.

Please investigate whether Invoice Ninja v5 products support product images, related documents, file attachments, or image URLs through the API.

If supported:
- Add typed methods or fields to GoInvoiceNinja for uploading/attaching product images or setting image URLs.
- Add tests.

If unsupported:
- Document the recommended custom field approach in the README.
- The Bunnings sync client currently stores image URLs in a configurable product custom field, default custom_value2.

3. Confirm export-friendly include behaviour

GoBunningsNinja v0.3 uses ListAll with includes for richer exports:
- Clients.ListAll with include contacts
- Quotes.ListAll with include client
- Invoices.ListAll with include client
- Payments.ListAll with include client,invoices

Please verify those include values are correct for Invoice Ninja v5 and are passed through reliably by ListAll.

4. Improve API error messages

Please improve APIError.Error() so it includes:
- HTTP status code
- main message
- validation error details when present

Keep it concise but make 4xx validation responses actionable.

Add tests for:
- message-only response
- errors-only response
- message plus errors response
- empty-body response

5. Validation

Please run:

go test ./...
go vet ./...

If go vet reports existing unrelated issues, document them clearly.
```

## GoBunningsNinja internal notes

- Product `Max Quantity` remains exported blank and ignored on import until `GoInvoiceNinja` confirms a real API field.
- Product `Image URL` currently maps to the configured Invoice Ninja custom field, default `custom_value2`.
- Client `Address` remains a single combined billing address column. If heavier address editing becomes important, this should become structured columns: Address 1, Address 2, City, State, Postal Code, Country ID.
- Quote, invoice, and payment CSV support is export-only. This is intentional for now; importing financial documents in bulk is a surprisingly efficient way to summon accounting goblins.
