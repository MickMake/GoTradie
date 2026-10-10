package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFileRequiresMandatoryFile(t *testing.T) {
	_, err := loadFile(filepath.Join(t.TempDir(), "missing.yaml"), emptyEnv)
	if err == nil || !strings.Contains(err.Error(), "open mandatory config") {
		t.Fatalf("error = %v; want mandatory-file error", err)
	}
}

func TestLoadIgnoresLegacyConfigEnvironment(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	legacyPath := filepath.Join(t.TempDir(), "legacy.conf")
	if err := os.WriteFile(legacyPath, []byte("INVOICE_NINJA_TOKEN=legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOTRADIE_CONFIG", legacyPath)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), filepath.Join(".GoTradie", "config.yaml")) {
		t.Fatalf("Load() error = %v; want mandatory ~/.GoTradie/config.yaml failure", err)
	}
}

func TestExampleConfigurationLoads(t *testing.T) {
	path := filepath.Join("..", "..", "config.yaml.example")
	if _, err := loadFile(path, emptyEnv); err != nil {
		t.Fatalf("load %s: %v", path, err)
	}
}

func TestLoadFileRejectsInvalidDocuments(t *testing.T) {
	tests := map[string]string{
		"malformed":      "bas: [",
		"legacy flat":    "INVOICE_NINJA_TOKEN=legacy",
		"unknown":        validQuarterlyYAML() + "\nmystery: true\n",
		"product prefix": validQuarterlyYAML() + "\nproduct_prefix: BUNNINGS-\n",
		"duplicate":      validQuarterlyYAML() + "\ntax:\n  name: GST\n  name: VAT\n",
		"multiple":       validQuarterlyYAML() + "\n---\nfoo: bar\n",
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := loadYAML(t, body, emptyEnv); err == nil {
				t.Fatal("expected config error")
			}
		})
	}
}

func TestLoadFileRequiresAccountingConfiguration(t *testing.T) {
	tests := map[string]struct {
		old  string
		next string
		want string
	}{
		"reporting period": {"reporting_period: quarterly", "reporting_period: \"\"", "bas.reporting_period is required"},
		"gst basis":        {"gst_basis: cash", "gst_basis: \"\"", "bas.gst_basis is required"},
		"eofy basis":       {"accounting_basis: cash", "accounting_basis: \"\"", "eofy.accounting_basis is required"},
		"asset threshold":  {"instant_asset_writeoff_threshold: 20000", "instant_asset_writeoff_threshold: 0", "instant_asset_writeoff_threshold"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			body := strings.Replace(validQuarterlyYAML(), test.old, test.next, 1)
			_, err := loadYAML(t, body, emptyEnv)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v; want %q", err, test.want)
			}
		})
	}
}

func TestLoadFileAcceptsCompleteBASCadences(t *testing.T) {
	tests := map[string]string{
		"quarterly": validQuarterlyYAML(),
		"monthly":   validMonthlyYAML(),
		"yearly":    validYearlyYAML(),
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			cfg, err := loadYAML(t, body, emptyEnv)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.BAS.ReportingPeriod != name {
				t.Fatalf("reporting period = %q; want %q", cfg.BAS.ReportingPeriod, name)
			}
		})
	}
}

func TestLoadFileRejectsIncompleteOrInconsistentBASCadence(t *testing.T) {
	tests := map[string]struct {
		body string
		want string
	}{
		"missing periods": {
			`bas:
  reporting_period: quarterly
  gst_basis: cash
eofy:
  accounting_basis: cash
  instant_asset_writeoff_threshold: 20000
`,
			"bas.periods is required",
		},
		"missing period": {
			strings.Replace(validQuarterlyYAML(), quarterlyQ4(), "", 1),
			"exactly Q1, Q2, Q3, Q4",
		},
		"missing monthly period": {
			strings.Replace(validMonthlyYAML(), `    Jun: {bas_begin: "06-01", bas_end: "06-last", submit_begin: "07-01", submit_end: "07-21"}
`, "", 1),
			"exactly Jul, Aug, Sep, Oct, Nov, Dec, Jan, Feb, Mar, Apr, May, Jun",
		},
		"extra period": {
			strings.Replace(validQuarterlyYAML(), quarterlyQ4(), quarterlyQ4()+`    Q5:
      bas_begin: "07-01"
      bas_end: "09-30"
      submit_begin: "10-01"
      submit_end: "10-28"
`, 1),
			"exactly Q1, Q2, Q3, Q4",
		},
		"wrong boundary": {
			strings.Replace(validQuarterlyYAML(), "bas_end: \"09-30\"", "bas_end: \"09-29\"", 1),
			"must cover 07-01 through 09-30",
		},
		"monthly end must remain calendar-relative": {
			strings.Replace(validMonthlyYAML(), `Feb: {bas_begin: "02-01", bas_end: "02-last"`, `Feb: {bas_begin: "02-01", bas_end: "02-28"`, 1),
			"must cover 02-01 through 02-last",
		},
		"late submission start": {
			strings.Replace(validQuarterlyYAML(), "submit_begin: \"10-01\"", "submit_begin: \"10-02\"", 1),
			"must begin on the day after",
		},
		"last outside bas end": {
			strings.Replace(validQuarterlyYAML(), "submit_end: \"10-28\"", "submit_end: \"10-last\"", 1),
			"MM-last is only allowed for bas_end",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := loadYAML(t, test.body, emptyEnv)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v; want %q", err, test.want)
			}
		})
	}
}

func TestLoadFileAppliesOnlySupportedSecretOverrides(t *testing.T) {
	env := map[string]string{
		"INVOICE_NINJA_TOKEN":    "env-token",
		"BUNNINGS_CLIENT_SECRET": "env-secret",
		"INVOICE_NINJA_URL":      "https://ignored.example",
		"BUNNINGS_CLIENT_ID":     "ignored-client",
		"TAX_NAME":               "Ignored Tax",
		"TAX_RATE":               "99",
	}
	cfg, err := loadYAML(t, validQuarterlyYAML(), func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.InvoiceNinja.Token != "env-token" {
		t.Fatalf("token = %q; want env-token", cfg.InvoiceNinja.Token)
	}
	if cfg.Providers["bunnings"].ClientSecret != "env-secret" {
		t.Fatalf("secret = %q; want env-secret", cfg.Providers["bunnings"].ClientSecret)
	}
	if cfg.InvoiceNinja.URL != "https://yaml.example.test" ||
		cfg.Providers["bunnings"].ClientID != "yaml-client" ||
		cfg.Tax.Name != "GST" || cfg.Tax.Rate != 10 {
		t.Fatalf("generic environment value overrode YAML: %#v", cfg)
	}
}

func TestLoadFilePreservesDefaultsAndProviderMapping(t *testing.T) {
	body := strings.Replace(validQuarterlyYAML(), `tax:
  name: GST
  rate: 10

`, "", 1)
	body = strings.Replace(body, `product_sync:
  custom_fields:
    bunnings_in: 1
    image_url: 2

`, "", 1)
	body = strings.Replace(body, "    environment: live\n", "", 1)
	body = strings.Replace(body, "    country: AU\n", "", 1)

	cfg, err := loadYAML(t, body, emptyEnv)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tax.Name != "GST" || cfg.Tax.Rate != 10 {
		t.Fatalf("tax defaults = %#v", cfg.Tax)
	}
	if cfg.ProductSync.CustomFields.BunningsIN != 1 || cfg.ProductSync.CustomFields.ImageURL != 2 {
		t.Fatalf("product sync defaults = %#v", cfg.ProductSync.CustomFields)
	}
	bunnings := cfg.Providers["bunnings"]
	if bunnings.Environment != "live" || bunnings.Country != "AU" {
		t.Fatalf("Bunnings defaults = %#v", bunnings)
	}
	if got := cfg.CanonicalProviderName(" bUnNiNgS warehouse "); got != "Bunnings" {
		t.Fatalf("canonical provider = %q; want Bunnings", got)
	}
	if got := cfg.CanonicalProviderName(" Other Supplier "); got != "Other Supplier" {
		t.Fatalf("unknown provider = %q; want trimmed input", got)
	}
}

func TestLoadFileAcceptsGenericXLSXProviderWithExactMappings(t *testing.T) {
	body := validQuarterlyYAML() + `  generic:
    name: Generic Supplier
    type: xlsx
    url: https://supplier.example/catalogue.xlsx
    fields:
      product: SKU
      description: Description
`
	cfg, err := loadYAML(t, body, emptyEnv)
	if err != nil {
		t.Fatal(err)
	}
	if provider := cfg.Providers["generic"]; provider.Type != "xlsx" || provider.Fields.Product != "SKU" {
		t.Fatalf("generic provider = %#v", provider)
	}
}

func TestLoadFileRejectsUnsafeProviderAndProductMappings(t *testing.T) {
	tests := map[string]struct {
		body string
		want string
	}{
		"alias collision": {
			validQuarterlyYAML() + `  other:
      name: Other
      type: csv
      aliases: [Bunnings Warehouse]
      url: https://other.example.test/catalog.csv
      fields:
        product: SKU
`,
			"ambiguous",
		},
		"generic provider missing product": {
			validQuarterlyYAML() + `  other:
      name: Other
      type: csv
      url: https://other.example.test/catalog.csv
`,
			"fields.product is required",
		},
		"invalid provider URL": {
			validQuarterlyYAML() + `  other:
      name: Other
      type: csv
      url: /local/catalog.csv
      fields:
        product: SKU
`,
			"absolute http or https URL",
		},
		"duplicate product custom field": {
			strings.Replace(validQuarterlyYAML(), "image_url: 2", "image_url: 1", 1),
			"must use different custom fields",
		},
		"out of range product custom field": {
			strings.Replace(validQuarterlyYAML(), "bunnings_in: 1", "bunnings_in: 5", 1),
			"must be between 1 and 4",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := loadYAML(t, test.body, emptyEnv)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v; want %q", err, test.want)
			}
		})
	}
}

func TestCommandSpecificCredentialValidation(t *testing.T) {
	cfg, err := loadYAML(t, validQuarterlyYAML(), emptyEnv)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("complete sync config: %v", err)
	}
	cfg.InvoiceNinja.Token = ""
	if err := cfg.ValidateInvoiceNinja(); err == nil {
		t.Fatal("expected missing Invoice Ninja token error")
	}
	cfg.InvoiceNinja.Token = "token"
	bunnings := cfg.Providers["bunnings"]
	bunnings.ClientSecret = ""
	cfg.Providers["bunnings"] = bunnings
	if err := cfg.ValidateBunnings(); err == nil {
		t.Fatal("expected missing Bunnings secret error")
	}
}

func loadYAML(t *testing.T, body string, getenv func(string) string) (Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return loadFile(path, getenv)
}

func emptyEnv(string) string { return "" }

func validQuarterlyYAML() string {
	return `invoice_ninja:
  url: https://yaml.example.test
  token: yaml-token

tax:
  name: GST
  rate: 10

bas:
  reporting_period: quarterly
  gst_basis: cash
  periods:
    Q1:
      bas_begin: "07-01"
      bas_end: "09-30"
      submit_begin: "10-01"
      submit_end: "10-28"
    Q2:
      bas_begin: "10-01"
      bas_end: "12-31"
      submit_begin: "01-01"
      submit_end: "02-28"
    Q3:
      bas_begin: "01-01"
      bas_end: "03-31"
      submit_begin: "04-01"
      submit_end: "04-28"
` + quarterlyQ4() + `eofy:
  accounting_basis: cash
  instant_asset_writeoff_threshold: 20000

exports:
  directory: ~/Documents/GoTradie

product_sync:
  custom_fields:
    bunnings_in: 1
    image_url: 2

providers:
  bunnings:
    name: Bunnings
    type: api
    aliases:
      - Bunnings
      - Bunnings Warehouse
      - Bunnings Trade
    environment: live
    client_id: yaml-client
    client_secret: yaml-secret
    scopes: []
    country: AU
    location: ""
`
}

func quarterlyQ4() string {
	return `    Q4:
      bas_begin: "04-01"
      bas_end: "06-30"
      submit_begin: "07-01"
      submit_end: "07-28"
`
}

func validMonthlyYAML() string {
	return `bas:
  reporting_period: monthly
  gst_basis: accrual
  periods:
    Jul: {bas_begin: "07-01", bas_end: "07-last", submit_begin: "08-01", submit_end: "08-21"}
    Aug: {bas_begin: "08-01", bas_end: "08-last", submit_begin: "09-01", submit_end: "09-21"}
    Sep: {bas_begin: "09-01", bas_end: "09-last", submit_begin: "10-01", submit_end: "10-21"}
    Oct: {bas_begin: "10-01", bas_end: "10-last", submit_begin: "11-01", submit_end: "11-21"}
    Nov: {bas_begin: "11-01", bas_end: "11-last", submit_begin: "12-01", submit_end: "12-21"}
    Dec: {bas_begin: "12-01", bas_end: "12-last", submit_begin: "01-01", submit_end: "01-21"}
    Jan: {bas_begin: "01-01", bas_end: "01-last", submit_begin: "02-01", submit_end: "02-21"}
    Feb: {bas_begin: "02-01", bas_end: "02-last", submit_begin: "03-01", submit_end: "03-21"}
    Mar: {bas_begin: "03-01", bas_end: "03-last", submit_begin: "04-01", submit_end: "04-21"}
    Apr: {bas_begin: "04-01", bas_end: "04-last", submit_begin: "05-01", submit_end: "05-21"}
    May: {bas_begin: "05-01", bas_end: "05-last", submit_begin: "06-01", submit_end: "06-21"}
    Jun: {bas_begin: "06-01", bas_end: "06-last", submit_begin: "07-01", submit_end: "07-21"}
eofy:
  accounting_basis: accrual
  instant_asset_writeoff_threshold: 1000
`
}

func validYearlyYAML() string {
	return `bas:
  reporting_period: yearly
  gst_basis: cash
  periods:
    FY:
      bas_begin: "07-01"
      bas_end: "06-30"
      submit_begin: "07-01"
      submit_end: "10-31"
eofy:
  accounting_basis: cash
  instant_asset_writeoff_threshold: 20000
`
}
