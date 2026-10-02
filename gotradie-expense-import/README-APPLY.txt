GoTradie expense import slice - corrected package
=================================================

Base inspected: MickMake/GoTradie main @ 652683457f554fea93422636e33cbd8300dfe15a
Suggested branch: feature/ninja-expense-import

This replacement ZIP fixes the packaging omission that made the expenses CLI
unreachable when only the overlay files were copied. The authoritative install
method is changes.patch (or APPLY.sh), which includes app.go CLI wiring.

Changes included in this replacement
------------------------------------
- CLI wiring for:
    GoTradie ninja import expenses purchases.csv
    GoTradie ninja import expenses --commit purchases.csv
- Vendor names now use:
    Supplier + " - " + Store
  Example: Bunnings - Dural
  If Store is blank, the vendor is just Supplier.
- The spreadsheet Date is written to BOTH expense date and payment_date.
  For this source spreadsheet Date is the actual purchase/payment date.
- Preview remains the default; --commit performs Invoice Ninja writes.
- Missing vendors and broad expense categories are created on commit.
- Job Number is treated as the parent project number. Existing projects are
  linked; a missing project is created only when the number can be matched to
  an existing Invoice Ninja invoice/quote client.
- Stable source-row markers make rerunning the same CSV idempotent.

Expense field mapping
---------------------
Invoice Ninja Expense Category <- Category
custom_value1                 <- Tax Treatment
custom_value2                 <- Option / Tax Detail
custom_value3                 <- Business %
custom_value4                 <- BAS Treatment
transaction_reference         <- Invoice Number
amount                        <- Business Amount
payment_date                  <- Date
date                          <- Date
vendor                        <- Supplier + " - " + Store (or Supplier if blank)
tax_amount1                   <- Business GST
private_notes                 <- source/audit detail including filename, item,
                                 original totals/currency, job refs and notes

BAS Treatment import rule
-------------------------
Business % <= 0 or personal/non-deductible -> Private/Non-deductible
Business GST > 0                           -> GST Credit
Otherwise                                  -> Review

Deliberate limit in this slice
------------------------------
Receipt files are NOT uploaded yet. File Name is preserved in private notes so
nothing is lost and receipt attachment can be added as a later small slice.

Apply - recommended
-------------------
1. From the GoTradie repository root, make sure you are on latest main and create:
     git switch main
     git pull
     git switch -c feature/ninja-expense-import

2. From the unpacked bundle run:
     ./APPLY.sh /path/to/your/GoTradie

   Or manually from the repository root:
     git apply --check /path/to/bundle/changes.patch
     git apply /path/to/bundle/changes.patch

3. Inspect:
     git status
     git diff

4. Test GoInvoiceNinja:
     cd apps/GoInvoiceNinja
     gofmt -w .
     go test ./...
     go vet ./...
     go build ./...

5. Test GoTradie and rebuild the root binary:
     cd ../GoTradie
     gofmt -w .
     go test ./...
     go vet ./...
     go build ./...
     go build -o ../../GoTradie ./cmd/GoTradie
     cd ../..

6. Preview only first:
     ./GoTradie ninja import expenses spreadsheet.csv

   Do NOT add --commit until the preview output looks right.

Package layout
--------------
changes.patch              Complete patch, including app.go CLI wiring.
APPLY.sh                   Small wrapper for git apply --check + git apply.
patches/app.go.patch       CLI wiring patch surfaced separately for inspection.
overlay/...                New/replaced implementation files for easy review.

Validation performed here
-------------------------
- gofmt run on modified Go files.
- Isolated GoInvoiceNinja expense-service tests passed.
- Isolated GoTradie expense-import helper tests passed, including vendor naming.
- changes.patch parses successfully with git apply --numstat.

Full repo tests remain for Mick's machine after application. CI is the adult in
the room, although it occasionally leaves its glasses in the fridge.
