package app

import "fmt"

func (a App) extendedUsage() {
	fmt.Fprintln(a.Out, `bunnings-ninja extended command help

Commands preview or refuse risky writes by default.
Use --commit to make persistent changes.

The normal help is intentionally short. This command is the longer field guide: more plumage, more footprints, fewer surprises.

Global options

  --config <path>
      Use a specific key=value configuration file.

      Config is loaded in this order:
        1. --config <path>
        2. GOBUNNINGSNINJA_CONFIG
        3. ./gobunningsninja.conf, if present

      Config file values override environment variables.
      This option does not imply --commit.

      Example:
        bunnings-ninja --config ./gobunningsninja.conf ninja export products products.csv

Top-level commands

  help
      Show the short command summary.

      Example:
        bunnings-ninja help

  commands
      Show this extended command help.

      Aliases:
        extended-help
        manual

      Example:
        bunnings-ninja commands

  version
      Print the application version.
      This command does not read config and does not require credentials.

      Example:
        bunnings-ninja version

      Example output:
        v0.5

Bunnings commands

  bunnings find <query> [--web] [--limit=N]
      Search Bunnings using a fuzzy/natural-ish query and print CSV.
      This is read-only discovery.
      It does not require Invoice Ninja credentials.
      It does not accept --commit.

      Example:
        bunnings-ninja bunnings find "merbau decking" --limit=3

      Example output:
        IN,Description,Unit,PricePerUnit,ImageURL
        0123456,90 x 19mm Merbau Decking LM,LM,7.45,https://...
        0987654,140 x 19mm Merbau Decking LM,LM,11.20,https://...
        0456789,Merbau Screening 42 x 19mm LM,LM,4.80,https://...

  bunnings get <IN...> [--web]
      Fetch one or more exact Bunnings item numbers and print CSV.
      This is read-only exact lookup.
      Errors are returned as row data where practical so one bad item does not spoil the whole basket.

      Example:
        bunnings-ninja bunnings get 0123456 0987654

      Example output:
        IN,Description,Unit,PricePerUnit,ImageURL
        0123456,90 x 19mm Merbau Decking LM,LM,7.45,https://...
        0987654,140 x 19mm Merbau Decking LM,LM,11.20,https://...

  bunnings lookup <IN...> [--web]
      Fetch one or more exact Bunnings item numbers and print human-readable detail blocks.
      This is the readable sibling of bunnings get.
      It does not produce CSV and does not accept --commit.

      Example:
        bunnings-ninja bunnings lookup 0123456

      Example output:
        IN: 0123456
        Title: 90 x 19mm Merbau Decking
        Description: 90 x 19mm Merbau Decking LM
        Unit: LM
        PricePerUnit: 7.45
        ImageURL: https://...

Sync commands

  sync refresh [--web] [--commit]
      Refresh existing Invoice Ninja products already linked to Bunnings item numbers.
      Also allowed as shorthand:
        bunnings-ninja sync

      Without --commit, this previews changes only.
      With --commit, matching Invoice Ninja products may be updated.

      Example preview:
        bunnings-ninja sync refresh

      Example preview output:
        IN        ProductKey        Action        Changes/Error
        0123456   BUNNINGS-0123456  would-update  price 7.20 -> 7.45
        0987654   BUNNINGS-0987654  unchanged

      Example commit:
        bunnings-ninja sync refresh --commit

      Example commit output:
        IN        ProductKey        Action        Changes/Error
        0123456   BUNNINGS-0123456  updated       price 7.20 -> 7.45
        0987654   BUNNINGS-0987654  unchanged

  sync import <IN> [--web] [--commit]
      Add or refresh a single Invoice Ninja product from a known Bunnings item number.
      Without --commit, this previews create/update behaviour only.
      With --commit, the matching Invoice Ninja product may be created or updated.

      Example preview:
        bunnings-ninja sync import 0123456

      Example preview output:
        IN        ProductKey        Action        Changes/Error
        0123456   BUNNINGS-0123456  would-update  price 7.20 -> 7.45

      Example commit:
        bunnings-ninja sync import --commit 0123456

      Example commit output:
        IN        ProductKey        Action        Changes/Error
        0123456   BUNNINGS-0123456  updated       price 7.20 -> 7.45

      Legacy alias:
        bunnings-ninja add-in 0123456

      The alias prints a deprecation notice and routes to sync import.

  sync search <query> [--web] [--limit=N]
      Search Bunnings and show candidate products.
      This mode cannot update Invoice Ninja.

      Example:
        bunnings-ninja sync search "merbau decking" --limit=3

      Example output:
        Bunnings search results
        IN        Title
        0123456   90 x 19mm Merbau Decking
        0987654   140 x 19mm Merbau Decking
        0456789   Merbau Screening 42 x 19mm

        Preview only. To import, re-run with --create --select=IN1,IN2 --commit

  sync search <query> --create --select=IN1,IN2 [--commit]
      Search Bunnings, select specific results, then preview or commit imports into Invoice Ninja.
      Without --commit, still preview only.

      Example preview:
        bunnings-ninja sync search "merbau decking" --create --select=0123456,0987654

      Example preview output:
        IN        ProductKey        Action        Changes/Error
        0123456   BUNNINGS-0123456  would-update  price 7.20 -> 7.45
        0987654   BUNNINGS-0987654  would-update  description changed

      Example commit:
        bunnings-ninja sync search "merbau decking" --create --select=0123456,0987654 --commit

      Example commit output:
        IN        ProductKey        Action        Changes/Error
        0123456   BUNNINGS-0123456  updated       price 7.20 -> 7.45
        0987654   BUNNINGS-0987654  updated       description changed

  sync search <query> --create --all --yes [--commit]
      Select all returned search results, subject to the result cap.
      --all requires --yes, even with --commit, because bulk imports are where spreadsheet goblins learn to breed.

      Example:
        bunnings-ninja sync search "merbau decking" --create --all --yes --commit

Invoice Ninja export commands

  ninja export products <file|-> [--commit]
      Export active Invoice Ninja products as CSV.
      Writes to a new file by default, or stdout when the path is -.
      Refuses to overwrite an existing file unless --commit is supplied.

      Example:
        bunnings-ninja ninja export products products.csv

      Example stdout:
        bunnings-ninja ninja export products -

      Example overwrite:
        bunnings-ninja ninja export products products.csv --commit

      Example output:
        ID,Product,Description,Price,Default Quantity,Max Quantity,Image URL
        abc123,BUNNINGS-0123456,90 x 19mm Merbau Decking,7.45,1,,https://...

  ninja export clients <file|-> [--commit]
      Export active Invoice Ninja clients and contacts as CSV.
      Contact columns expand to match the maximum contact count found.
      Refuses to overwrite an existing file unless --commit is supplied.

      Example:
        bunnings-ninja ninja export clients clients.csv

      Example output:
        ID,Name,Address,Contact 1 First Name,Contact 1 Last Name,Contact 1 Email,Contact 1 Phone
        abc123,Example Client,"1 Sample St",Ada,Lovelace,ada@example.com,0400000000

  ninja export quotes <file|-> [--commit]
      Export active Invoice Ninja quotes as CSV.
      Export only; quote imports are not supported.
      Refuses to overwrite an existing file unless --commit is supplied.

      Example:
        bunnings-ninja ninja export quotes quotes.csv

      Example output:
        ID,Number,Client ID,Client Name,Status,Date,Valid Until,Subtotal,Discount,Tax,Total,Balance,Public Notes,Private Notes
        q123,QU-0001,c123,Example Client,sent,2026-05-19,2026-06-18,100.00,0.00,10.00,110.00,110.00,,

  ninja export invoices <file|-> [--commit]
      Export active Invoice Ninja invoices as CSV.
      Export only; invoice imports are not supported.
      Refuses to overwrite an existing file unless --commit is supplied.

      Example:
        bunnings-ninja ninja export invoices invoices.csv

      Example output:
        ID,Number,Client ID,Client Name,Status,Date,Due Date,Subtotal,Discount,Tax,Total,Balance,Paid To Date,Public Notes,Private Notes
        i123,INV-0001,c123,Example Client,sent,2026-05-19,2026-06-02,100.00,0.00,10.00,110.00,110.00,0.00,,

  ninja export payments <file|-> [--commit]
      Export active Invoice Ninja payments as CSV.
      Export only; payment imports are not supported.
      Refuses to overwrite an existing file unless --commit is supplied.

      Example:
        bunnings-ninja ninja export payments payments.csv

      Example output:
        ID,Client ID,Client Name,Invoice ID,Invoice Number,Date,Amount,Applied,Refunded,Transaction Reference,Payment Type,Status,Private Notes
        p123,c123,Example Client,i123,INV-0001,2026-05-19,110.00,110.00,0.00,TXN-123,bank_transfer,completed,

Invoice Ninja import commands

  ninja import products <file|-> [--commit]
      Preview or apply product CSV changes into Invoice Ninja.
      Without --commit, this previews differences only.
      With --commit, changed existing products are updated.
      Blank IDs are not created and product IDs are not guessed.

      Example preview:
        bunnings-ninja ninja import products products.csv

      Example preview output:
        ID        Name              Action        Changes/Error
        abc123    BUNNINGS-0123456  would-update  price 7.20 -> 7.45
        def456    BUNNINGS-0987654  unchanged

      Example commit:
        bunnings-ninja ninja import products --commit products.csv

      Example commit output:
        ID        Name              Action        Changes/Error
        abc123    BUNNINGS-0123456  updated       price 7.20 -> 7.45
        def456    BUNNINGS-0987654  unchanged

      Stdin example:
        cat products.csv | bunnings-ninja ninja import products -

  ninja import clients <file|-> [--commit]
      Preview or apply client/contact CSV changes into Invoice Ninja.
      Without --commit, this previews differences only.
      With --commit, changed existing clients are updated.
      Blank IDs are not created and client IDs are not guessed.

      Example preview:
        bunnings-ninja ninja import clients clients.csv

      Example preview output:
        ID        Name            Action        Changes/Error
        abc123    Example Client  would-update  contact email changed
        def456    Other Client    unchanged

      Example commit:
        bunnings-ninja ninja import clients --commit clients.csv

      Example commit output:
        ID        Name            Action        Changes/Error
        abc123    Example Client  updated       contact email changed
        def456    Other Client    unchanged

      Stdin example:
        cat clients.csv | bunnings-ninja ninja import clients -

Deprecated or rejected command forms

  add-in <IN>
      Deprecated alias for sync import <IN>.

  search <query>
      Deprecated top-level form.
      Use sync search for guarded import workflow, or bunnings find for pure Bunnings discovery.

  ninja-products-export, ninja-products-import, ninja-clients-export, ninja-clients-import
      Rejected legacy forms.
      Use grouped commands instead:
        bunnings-ninja ninja export ...
        bunnings-ninja ninja import ...

Configuration summary

  Required for ninja commands:
    INVOICE_NINJA_TOKEN

  Additional required for Bunnings and sync commands:
    BUNNINGS_CLIENT_ID
    BUNNINGS_CLIENT_SECRET

  Useful optional configuration:
    INVOICE_NINJA_URL
    BUNNINGS_ENV
    BUNNINGS_COUNTRY
    BUNNINGS_LOCATION
    BUNNINGS_SCOPES
    PRODUCT_PREFIX
    BUNNINGS_IN_CUSTOM_FIELD
    BUNNINGS_IMAGE_CUSTOM_FIELD
    TAX_NAME
    TAX_RATE`)
}
