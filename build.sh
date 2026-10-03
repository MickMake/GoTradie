#!/bin/bash

cd ./apps/GoTradie
go test ./...
go build -o ../../GoTradie ./cmd/GoTradie

echo "Import:
./GoTradie ninja import expenses spreadsheet.csv --receipts-root ~/Documents/Work/Tax/Receipts --commit
"

