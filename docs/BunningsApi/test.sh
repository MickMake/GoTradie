#!/bin/bash

TOKEN=$(curl -fsS \
  -u "$BUNNINGS_CLIENT_ID:$BUNNINGS_CLIENT_SECRET" \
  -d "grant_type=client_credentials" \
  --data-urlencode "scope=itm:details" \
  https://connect.sandbox.api.bunnings.com.au/connect/token \
  | jq -r '.access_token')

# live https://authorisation.api.bunnings.com.au/connect/token

echo "TOKEN: $TOKEN"

# live "https://item.api.bunnings.com.au/item/detail/AU/2582847"

curl -fsS \
  "https://item.sandbox.api.bunnings.com.au/item/detail/AU/2582847" \
  -H "Authorization: Bearer $TOKEN" \
  -H "x-version-api: 1.4" \
  -H "Accept: application/json" \
  | jq . | tee product.json


