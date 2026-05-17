# Weird stuff and fix prompts

These are the things noticed while integrating the uploaded packages and building version `0.1` of `GoBunningsNinja`.

## Needs fixing before reliable image syncing

### GoBunnings: `Item.Raw` is never populated

`Item` has:

```go
Raw map[string]any `json:"-"`
```

Because of the `json:"-"` tag, `encoding/json` ignores it. That means downstream clients cannot inspect unmodelled fields such as image URLs, brand names, web URLs, or any useful surprise parcels the Bunnings API sends back.

Prompt to issue in the **GoBunnings** chat:

```text
Please update GoBunnings so product/item responses preserve the original raw JSON.

Currently Item.Raw is tagged json:"-", so it is never populated. I need Raw to contain the decoded product object so downstream clients can safely inspect fields that are not yet modelled, especially image URLs.

Please implement this cleanly using a custom UnmarshalJSON on Item, or an equivalent approach, so:
1. Normal typed fields such as IN, Name, Description, Price, URL, etc. still unmarshal as before.
2. Raw is populated with the original decoded JSON object.
3. Existing tests still pass.
4. Add tests proving Raw is populated.
5. Do not break the public API.
```

### GoBunnings: add typed image URL support

The client now has an `Image URL` CSV column and sync wiring, but reliable Bunnings image data still depends on `GoBunnings` exposing image fields properly.

Prompt to issue in the **GoBunnings** chat:

```text
Please add typed image URL support to GoBunnings product/item models.

The sync client needs to refresh InvoiceNinja product image URLs from Bunnings product data. Please inspect actual Bunnings product JSON and expose the best available image URL field as Item.ImageURL or equivalent.

Requirements:
1. Preserve backwards compatibility.
2. Add ImageURL to the item/product model.
3. Populate it from the correct Bunnings JSON field.
4. Keep Raw populated as a fallback.
5. Add tests for products with and without images.
```

## GoInvoiceNinja gaps affecting this client

### Product max quantity is not modelled

The new product CSV command includes the requested `Max Quantity` column, but the uploaded `GoInvoiceNinja` product model and create/update payloads do not expose a max quantity field. Version `0.1` therefore exports it blank and ignores it on import.

Prompt to issue in the **GoInvoiceNinja** chat:

```text
Please investigate whether Invoice Ninja v5 products support a max quantity or stock/quantity limit field through the API.

If supported, update GoInvoiceNinja so Product, CreateProductRequest, and UpdateProductRequest expose the correct field with the correct JSON tag. Add tests proving list/get/create/update round-trip the value. If unsupported by Invoice Ninja, document that explicitly in the README so downstream CSV tooling can leave the Max Quantity column read-only/ignored.
```

### Product image URL has no first-class field

Invoice Ninja products may not have a native image URL field. `GoBunningsNinja` currently maps image URL to the configured product custom field, default `custom_value2`.

Prompt to issue in the **GoInvoiceNinja** chat:

```text
Please investigate whether Invoice Ninja v5 products support product images or related documents through the API.

If supported, add typed methods to GoInvoiceNinja for uploading/attaching product images or setting image URLs. If unsupported, document the recommended custom field approach in the README. The Bunnings sync client currently stores image URLs in a configurable product custom field, default custom_value2.
```

### Client CSV address import is lossy if used heavily

The requested client CSV shape has one `Address` column. Invoice Ninja stores billing address as structured fields: `address1`, `address2`, `city`, `state`, `postal_code`, and `country_id`. Version `0.1` exports a combined address and, if changed on import, writes it to `address1` while preserving the other structured fields where possible.

This is safe enough for light edits, but not ideal for serious address hygiene. A single address column is a tidy-looking box containing several annoyed cats.

Potential prompt for this chat, if you want a future client change:

```text
Please revise GoBunningsNinja client CSV import/export to use structured billing address columns instead of one combined Address column.

Use columns: ID, Name, Address 1, Address 2, City, State, Postal Code, Country ID, followed by the repeated contact columns. Keep backwards compatibility by still accepting the older single Address column where possible.
```

## Existing package cleanups noticed earlier

### GoBunnings: `ProblemDetails.Extra` is never populated

Prompt:

```text
Please add custom UnmarshalJSON support for ProblemDetails so unknown/problem-extension fields are captured into Extra. Keep the known fields Type, Title, Status, Detail, Instance, and Errors as typed fields. Add tests for RFC7807-style extension fields.
```

### Artificial import keepalive lines

There are harmless but odd lines such as:

```go
var _ = url.Values{}
var _ = fmt.Sprintf
```

Prompt:

```text
Please remove artificial import keepalive lines such as var _ = url.Values{} and clean up the affected imports. Run gofmt and go test ./... afterwards.
```

### Possible barcode JSON casing issue

`Barcode.AdditionalDescription` uses:

```go
json:"AdditionalDescription,omitempty"
```

Most other fields use lower camel case. If the API returns `additionalDescription`, this field will not decode.

Prompt:

```text
Please verify the JSON casing for Barcode.AdditionalDescription against actual Bunnings API responses. If the API uses additionalDescription, change the tag from AdditionalDescription to additionalDescription and add a regression test.
```

### GoInvoiceNinja: API errors could be more useful

Prompt:

```text
Please improve GoInvoiceNinja APIError.Error() so it includes the HTTP status code and validation error details when present. Keep it concise, but make 4xx validation responses actionable. Add tests for message-only, errors-only, and empty-body responses.
```
