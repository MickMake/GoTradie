# GoBunnings

GoBunnings is a reusable Go SDK for the Bunnings API suite.

It is deliberately **not** a natural-language layer, estimator, CLI workflow, or carpentry-specific rules engine. It is a clean API client package that other tools can build on top of without waking the kraken.

## Scope

Included:

- OAuth2 client-credentials token source
- shared HTTP client
- environment-aware base URLs
- typed service wrappers
- typed request/response structs for common operations
- `x-version-api` handling per API family
- HATEOAS link helpers
- RFC7807/problem-detail error handling
- retry/backoff handling for 429 and transient 5xx responses
- query option helpers for `$select`, `$filter`, `$orderby`, `$skip`, `$top`, continuation tokens, and `$fullPage`

Not included:

- natural-language search
- estimating logic
- bill-of-material builders
- job-costing rules
- CLI-specific behaviour
- caching/persistence decisions

Those belong in separate packages or applications using GoBunnings as the transport/domain SDK.

## Supported API groups

| Service | Version | Scope examples |
|---|---:|---|
| Query Item | 1.4 | `itm:details` |
| Query Location | 1.0 | `loc:pub` |
| Query Inventory | 1.0 | `inv:pub` |
| Query Pricing | 1.0 | `pri:pub` |
| Order Status | 1.1 | `ord:limited` |

## Install

```bash
```

## Basic usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

)

func main() {
    ctx := context.Background()

    ts, err := gobunnings.NewClientCredentialsTokenSource(
        gobunnings.EnvSandbox,
        os.Getenv("BUNNINGS_CLIENT_ID"),
        os.Getenv("BUNNINGS_CLIENT_SECRET"),
        []string{"itm:details", "pri:pub", "inv:pub", "loc:pub", "ord:limited"},
    )
    if err != nil {
        log.Fatal(err)
    }

    client, err := gobunnings.New(gobunnings.EnvSandbox, ts)
    if err != nil {
        log.Fatal(err)
    }

    res, err := client.Item.Search(ctx, gobunnings.CountryAU, gobunnings.ItemSearchRequest{
        Query:  "lawn",
        SortBy: "relevancy",
        Filters: &gobunnings.ItemSearchFilters{
            LocationCode: "6401",
        },
    }, "")
    if err != nil {
        log.Fatal(err)
    }

    for _, item := range res.Results {
        fmt.Println(item.ItemNumber, item.Title)
    }
}
```

## Examples

### Product search

```go
items, err := client.Item.Search(ctx, gobunnings.CountryAU, gobunnings.ItemSearchRequest{
    Query: "merbau decking",
    Filters: &gobunnings.ItemSearchFilters{LocationCode: "6401"},
    SortBy: "relevancy",
}, "")
```

### Item details

```go
item, err := client.Item.Detail(ctx, gobunnings.CountryAU, "0123456", gobunnings.QueryOptions{})
```

### Pricing

```go
prices, err := client.Pricing.CatalogPrices(ctx, gobunnings.CatalogPriceRequest{
    Context: gobunnings.PriceContext{Country: gobunnings.CountryAU, Location: "7040"},
    Items: []gobunnings.PriceItem{{ItemNumber: "0123456"}},
}, gobunnings.QueryOptions{})
```

### Inventory

```go
stock, err := client.Inventory.Stock(ctx, gobunnings.CountryAU, "2010", "0123456", gobunnings.QueryOptions{})
```

### Locations

```go
stores, err := client.Location.Search(ctx, gobunnings.QueryOptions{
    Filter: "countryCode eq 'AU' and isStore eq true",
    Top: gobunnings.Int(100),
})
```

### Orders

```go
orders, err := client.Order.Search(ctx, gobunnings.OrderSearchParams{
    TransactionReference: "W300079786",
    Country: "AU",
}, gobunnings.QueryOptions{Top: gobunnings.Int(20)})
```

## Error handling

```go
res, err := client.Pricing.CatalogPrices(ctx, req, gobunnings.QueryOptions{})
if err != nil {
    var apiErr *gobunnings.APIError
    if errors.As(err, &apiErr) {
        fmt.Println(apiErr.StatusCode)
        if apiErr.Problem != nil {
            fmt.Println(apiErr.Problem.Title, apiErr.Problem.Detail)
        }
    }
    return err
}
_ = res
```

## Design principle

GoBunnings wraps vendor-specific API behaviour but does not invent business workflows. Higher-level tools can compose it, but the SDK stays boring, reusable, and testable. Boring software is good software; exciting software tends to require a mop.

## Version

Current package version: `v0.3`.

See `CHANGES.md` for the cumulative release history.

## Raw product JSON and image URLs

`Item` keeps typed fields for normal use and preserves the original decoded product object in `Item.Raw` for forward compatibility.

```go
item, err := client.Item.Detail(ctx, gobunnings.CountryAU, "0123456", gobunnings.QueryOptions{})
if err != nil {
    return err
}

fmt.Println(item.ItemNumber)
fmt.Println(item.ImageURL)
fmt.Println(item.Raw) // fallback for Bunnings fields not yet modelled by the SDK
```

`Item.ImageURL` is populated from the best available Bunnings media field. The preferred source is `enrichedItem.picture.primaryAssetURL`, with fallbacks for related media URL fields.

## Website retrieval

Use `WebsiteService` when you need website-derived lookup/search data (without using the Bunnings API).

