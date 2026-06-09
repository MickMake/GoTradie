#!/bin/bash

cd ./apps/GoTradie
go test ./...
go build -o ../../GoTradie ./cmd/GoTradie

