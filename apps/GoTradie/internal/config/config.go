package config

import (
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

const (
	configDirectory = ".GoTradie"
	configFilename  = "config.yaml"
)

type Config struct {
	InvoiceNinja InvoiceNinjaConfig        `yaml:"invoice_ninja"`
	Tax          TaxConfig                 `yaml:"tax"`
	BAS          BASConfig                 `yaml:"bas"`
	EOFY         EOFYConfig                `yaml:"eofy"`
	Exports      ExportsConfig             `yaml:"exports"`
	ProductSync  ProductSyncConfig         `yaml:"product_sync"`
	Providers    map[string]ProviderConfig `yaml:"providers"`
}

type InvoiceNinjaConfig struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

type TaxConfig struct {
	Name string  `yaml:"name"`
	Rate float64 `yaml:"rate"`
}

type BASConfig struct {
	ReportingPeriod string               `yaml:"reporting_period"`
	GSTBasis        string               `yaml:"gst_basis"`
	Periods         map[string]BASPeriod `yaml:"periods"`
}

type BASPeriod struct {
	BASBegin    string `yaml:"bas_begin"`
	BASEnd      string `yaml:"bas_end"`
	SubmitBegin string `yaml:"submit_begin"`
	SubmitEnd   string `yaml:"submit_end"`
}

type EOFYConfig struct {
	AccountingBasis               string  `yaml:"accounting_basis"`
	InstantAssetWriteoffThreshold float64 `yaml:"instant_asset_writeoff_threshold"`
}

type ExportsConfig struct {
	Directory *string `yaml:"directory"`
}

type ProductSyncConfig struct {
	CustomFields ProductCustomFields `yaml:"custom_fields"`
}

type ProductCustomFields struct {
	// BunningsIN and ImageURL identify the pre-v0.5.7 custom fields during
	// migration only. Product Sync uses the fixed v0.5.7 allocation thereafter.
	BunningsIN int `yaml:"bunnings_in"`
	ImageURL   int `yaml:"image_url"`
}

type ProviderConfig struct {
	Name         string         `yaml:"name"`
	Type         string         `yaml:"type"`
	Aliases      []string       `yaml:"aliases"`
	Environment  string         `yaml:"environment"`
	ClientID     string         `yaml:"client_id"`
	ClientSecret string         `yaml:"client_secret"`
	Scopes       []string       `yaml:"scopes"`
	Country      string         `yaml:"country"`
	Location     string         `yaml:"location"`
	URL          string         `yaml:"url"`
	Fields       ProviderFields `yaml:"fields"`
}

type ProviderFields struct {
	Product     string `yaml:"product"`
	Description string `yaml:"description"`
	Cost        string `yaml:"cost"`
	Price       string `yaml:"price"`
	Quantity    string `yaml:"quantity"`
	ImageURL    string `yaml:"image_url"`
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("resolve home directory: empty path")
	}
	return filepath.Join(home, configDirectory, configFilename), nil
}

func Load() (Config, error) {
	path, err := DefaultPath()
	if err != nil {
		return Config{}, err
	}
	return loadFile(path, os.Getenv)
}

func loadFile(path string, getenv func(string) string) (Config, error) {
	cfg := defaultConfig()
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open mandatory config %s: %w", path, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		if err == io.EOF {
			return Config{}, fmt.Errorf("decode config %s: file is empty", path)
		}
		return Config{}, fmt.Errorf("decode config %s: %w", path, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, fmt.Errorf("decode config %s: multiple YAML documents are not supported", path)
		}
		return Config{}, fmt.Errorf("decode config %s: %w", path, err)
	}

	applyProviderDefaults(&cfg)
	applySecretOverrides(&cfg, getenv)
	if err := cfg.ValidateConfiguration(); err != nil {
		return Config{}, fmt.Errorf("validate config %s: %w", path, err)
	}
	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		Tax: TaxConfig{Name: "GST", Rate: 10},
		ProductSync: ProductSyncConfig{CustomFields: ProductCustomFields{
			BunningsIN: 1,
			ImageURL:   2,
		}},
		Providers: make(map[string]ProviderConfig),
	}
}

func applyProviderDefaults(cfg *Config) {
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderConfig)
	}
	bunnings, ok := cfg.Providers["bunnings"]
	if !ok {
		return
	}
	if strings.TrimSpace(bunnings.Environment) == "" {
		bunnings.Environment = "live"
	}
	if strings.TrimSpace(bunnings.Country) == "" {
		bunnings.Country = "AU"
	}
	cfg.Providers["bunnings"] = bunnings
}

func applySecretOverrides(cfg *Config, getenv func(string) string) {
	if value := strings.TrimSpace(getenv("INVOICE_NINJA_TOKEN")); value != "" {
		cfg.InvoiceNinja.Token = value
	}
	if value := strings.TrimSpace(getenv("BUNNINGS_CLIENT_SECRET")); value != "" {
		bunnings, ok := cfg.Providers["bunnings"]
		if ok {
			bunnings.ClientSecret = value
			cfg.Providers["bunnings"] = bunnings
		}
	}
}

func (c Config) ValidateConfiguration() error {
	if err := validateHTTPURL("invoice_ninja.url", c.InvoiceNinja.URL, false); err != nil {
		return err
	}
	if strings.TrimSpace(c.Tax.Name) == "" {
		return fmt.Errorf("tax.name must not be blank")
	}
	if math.IsNaN(c.Tax.Rate) || math.IsInf(c.Tax.Rate, 0) || c.Tax.Rate < 0 {
		return fmt.Errorf("tax.rate must be a finite non-negative number")
	}
	if err := validateBAS(c.BAS); err != nil {
		return err
	}
	switch c.EOFY.AccountingBasis {
	case "cash", "accrual":
	default:
		if strings.TrimSpace(c.EOFY.AccountingBasis) == "" {
			return fmt.Errorf("eofy.accounting_basis is required")
		}
		return fmt.Errorf("eofy.accounting_basis must be cash or accrual")
	}
	threshold := c.EOFY.InstantAssetWriteoffThreshold
	if math.IsNaN(threshold) || math.IsInf(threshold, 0) || threshold <= 0 {
		return fmt.Errorf("eofy.instant_asset_writeoff_threshold must be a finite number greater than zero")
	}
	if c.Exports.Directory != nil && strings.TrimSpace(*c.Exports.Directory) == "" {
		return fmt.Errorf("exports.directory must not be blank when present")
	}
	if err := validateProductSync(c.ProductSync); err != nil {
		return err
	}
	return validateProviders(c.Providers)
}

func (c Config) Validate() error {
	if err := c.ValidateInvoiceNinja(); err != nil {
		return err
	}
	return c.ValidateBunnings()
}

func (c Config) ValidateInvoiceNinja() error {
	if strings.TrimSpace(c.InvoiceNinja.Token) == "" {
		return fmt.Errorf("missing required configuration: invoice_ninja.token or INVOICE_NINJA_TOKEN")
	}
	return nil
}

func (c Config) ValidateBunnings() error {
	provider, ok := c.Providers["bunnings"]
	if !ok {
		return fmt.Errorf("missing required configuration: providers.bunnings")
	}
	var missing []string
	if strings.TrimSpace(provider.ClientID) == "" {
		missing = append(missing, "providers.bunnings.client_id")
	}
	if strings.TrimSpace(provider.ClientSecret) == "" {
		missing = append(missing, "providers.bunnings.client_secret or BUNNINGS_CLIENT_SECRET")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}
	return nil
}

func (c Config) CanonicalProviderName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	match := strings.ToLower(value)
	for _, provider := range c.Providers {
		name := strings.TrimSpace(provider.Name)
		if strings.ToLower(name) == match {
			return name
		}
		for _, alias := range provider.Aliases {
			if strings.ToLower(strings.TrimSpace(alias)) == match {
				return name
			}
		}
	}
	return value
}

func validateProductSync(sync ProductSyncConfig) error {
	indexes := map[string]int{
		"product_sync.custom_fields.bunnings_in": sync.CustomFields.BunningsIN,
		"product_sync.custom_fields.image_url":   sync.CustomFields.ImageURL,
	}
	seen := make(map[int]string, len(indexes))
	for name, index := range indexes {
		if index < 1 || index > 4 {
			return fmt.Errorf("%s must be between 1 and 4", name)
		}
		if previous, exists := seen[index]; exists {
			return fmt.Errorf("%s and %s must use different custom fields", previous, name)
		}
		seen[index] = name
	}
	return nil
}

func validateProviders(providers map[string]ProviderConfig) error {
	aliases := make(map[string]string)
	ids := make([]string, 0, len(providers))
	for id := range providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		provider := providers[id]
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("provider identifier must not be blank")
		}
		name := strings.TrimSpace(provider.Name)
		if name == "" {
			return fmt.Errorf("providers.%s.name is required", id)
		}
		switch provider.Type {
		case "api", "csv", "xlsx":
		default:
			if strings.TrimSpace(provider.Type) == "" {
				return fmt.Errorf("providers.%s.type is required", id)
			}
			return fmt.Errorf("providers.%s.type must be api, csv, or xlsx", id)
		}
		if id == "bunnings" {
			if provider.Type != "api" {
				return fmt.Errorf("providers.bunnings.type must be api")
			}
			switch provider.Environment {
			case "live", "test", "sandbox":
			default:
				return fmt.Errorf("providers.bunnings.environment must be live, test, or sandbox")
			}
			switch provider.Country {
			case "AU", "NZ":
			default:
				return fmt.Errorf("providers.bunnings.country must be AU or NZ")
			}
		} else if strings.TrimSpace(provider.Fields.Product) == "" {
			return fmt.Errorf("providers.%s.fields.product is required for a configurable syncing provider", id)
		}
		if provider.Type == "csv" || provider.Type == "xlsx" {
			if err := validateHTTPURL("providers."+id+".url", provider.URL, true); err != nil {
				return err
			}
		} else if err := validateHTTPURL("providers."+id+".url", provider.URL, false); err != nil {
			return err
		}

		for _, alias := range append([]string{name}, provider.Aliases...) {
			alias = strings.TrimSpace(alias)
			if alias == "" {
				return fmt.Errorf("providers.%s.aliases must not contain blank values", id)
			}
			key := strings.ToLower(alias)
			if owner, exists := aliases[key]; exists && owner != id {
				return fmt.Errorf("provider name or alias %q is ambiguous between %q and %q", alias, owner, id)
			}
			aliases[key] = id
		}
	}
	return nil
}

func validateHTTPURL(name, value string, required bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return fmt.Errorf("%s is required", name)
		}
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s must be an absolute http or https URL", name)
	}
	return nil
}

type cadenceDefinition struct {
	names  []string
	ranges map[string][2]string
}

func validateBAS(cfg BASConfig) error {
	var definition cadenceDefinition
	switch cfg.ReportingPeriod {
	case "monthly":
		definition = monthlyCadence()
	case "quarterly":
		definition = quarterlyCadence()
	case "yearly":
		definition = yearlyCadence()
	default:
		if strings.TrimSpace(cfg.ReportingPeriod) == "" {
			return fmt.Errorf("bas.reporting_period is required")
		}
		return fmt.Errorf("bas.reporting_period must be monthly, quarterly, or yearly")
	}
	switch cfg.GSTBasis {
	case "cash", "accrual":
	default:
		if strings.TrimSpace(cfg.GSTBasis) == "" {
			return fmt.Errorf("bas.gst_basis is required")
		}
		return fmt.Errorf("bas.gst_basis must be cash or accrual")
	}
	if len(cfg.Periods) == 0 {
		return fmt.Errorf("bas.periods is required for %s reporting", cfg.ReportingPeriod)
	}
	if len(cfg.Periods) != len(definition.names) {
		return fmt.Errorf("bas.periods for %s reporting must contain exactly %s", cfg.ReportingPeriod, strings.Join(definition.names, ", "))
	}
	for name := range cfg.Periods {
		if _, ok := definition.ranges[name]; !ok {
			return fmt.Errorf("bas.periods contains unexpected %s period %q", cfg.ReportingPeriod, name)
		}
	}

	var previousEnd time.Time
	for i, name := range definition.names {
		period, ok := cfg.Periods[name]
		if !ok {
			return fmt.Errorf("bas.periods.%s is required for %s reporting", name, cfg.ReportingPeriod)
		}
		expected := definition.ranges[name]
		if period.BASBegin != expected[0] || period.BASEnd != expected[1] {
			return fmt.Errorf("bas.periods.%s must cover %s through %s for %s reporting", name, expected[0], expected[1], cfg.ReportingPeriod)
		}
		start, err := financialYearDate(period.BASBegin, false)
		if err != nil {
			return fmt.Errorf("bas.periods.%s.bas_begin: %w", name, err)
		}
		end, err := financialYearDate(period.BASEnd, true)
		if err != nil {
			return fmt.Errorf("bas.periods.%s.bas_end: %w", name, err)
		}
		if end.Before(start) {
			return fmt.Errorf("bas.periods.%s reporting range ends before it begins", name)
		}
		if i == 0 {
			if start.Month() != time.July || start.Day() != 1 {
				return fmt.Errorf("bas.periods must begin on 07-01")
			}
		} else if !start.Equal(previousEnd.AddDate(0, 0, 1)) {
			return fmt.Errorf("bas.periods.%s is not contiguous with the preceding period", name)
		}
		previousEnd = end

		submitBegin, err := dateAfter(period.SubmitBegin, end)
		if err != nil {
			return fmt.Errorf("bas.periods.%s.submit_begin: %w", name, err)
		}
		submitEnd, err := dateOnOrAfter(period.SubmitEnd, submitBegin)
		if err != nil {
			return fmt.Errorf("bas.periods.%s.submit_end: %w", name, err)
		}
		if !submitBegin.Equal(end.AddDate(0, 0, 1)) {
			return fmt.Errorf("bas.periods.%s submission window must begin on the day after the reporting period", name)
		}
		if submitEnd.Before(submitBegin) {
			return fmt.Errorf("bas.periods.%s submission window ends before it begins", name)
		}
	}
	if previousEnd.Month() != time.June || previousEnd.Day() != 30 {
		return fmt.Errorf("bas.periods must end on 06-30")
	}
	return nil
}

func quarterlyCadence() cadenceDefinition {
	return cadenceDefinition{
		names: []string{"Q1", "Q2", "Q3", "Q4"},
		ranges: map[string][2]string{
			"Q1": {"07-01", "09-30"},
			"Q2": {"10-01", "12-31"},
			"Q3": {"01-01", "03-31"},
			"Q4": {"04-01", "06-30"},
		},
	}
}

func monthlyCadence() cadenceDefinition {
	names := []string{"Jul", "Aug", "Sep", "Oct", "Nov", "Dec", "Jan", "Feb", "Mar", "Apr", "May", "Jun"}
	ranges := make(map[string][2]string, len(names))
	for i, name := range names {
		month := ((i + 6) % 12) + 1
		ranges[name] = [2]string{fmt.Sprintf("%02d-01", month), fmt.Sprintf("%02d-last", month)}
	}
	return cadenceDefinition{names: names, ranges: ranges}
}

func yearlyCadence() cadenceDefinition {
	return cadenceDefinition{
		names:  []string{"FY"},
		ranges: map[string][2]string{"FY": {"07-01", "06-30"}},
	}
}

func financialYearDate(value string, allowLast bool) (time.Time, error) {
	month, day, last, err := parseMonthDay(value, allowLast)
	if err != nil {
		return time.Time{}, err
	}
	year := 2001
	if month >= int(time.July) {
		year = 2000
	}
	if last {
		day = time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), nil
}

func dateAfter(value string, after time.Time) (time.Time, error) {
	date, err := templateDate(value, after.Year())
	if err != nil {
		return time.Time{}, err
	}
	if !date.After(after) {
		date, err = templateDate(value, after.Year()+1)
		if err != nil {
			return time.Time{}, err
		}
	}
	return date, nil
}

func dateOnOrAfter(value string, start time.Time) (time.Time, error) {
	date, err := templateDate(value, start.Year())
	if err != nil {
		return time.Time{}, err
	}
	if date.Before(start) {
		date, err = templateDate(value, start.Year()+1)
		if err != nil {
			return time.Time{}, err
		}
	}
	return date, nil
}

func templateDate(value string, year int) (time.Time, error) {
	month, day, _, err := parseMonthDay(value, false)
	if err != nil {
		return time.Time{}, err
	}
	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if int(date.Month()) != month || date.Day() != day {
		return time.Time{}, fmt.Errorf("must be a valid MM-DD calendar date")
	}
	return date, nil
}

func parseMonthDay(value string, allowLast bool) (month, day int, last bool, err error) {
	value = strings.TrimSpace(value)
	if len(value) < 5 || len(value) > 7 || value[2] != '-' {
		return 0, 0, false, fmt.Errorf("must use MM-DD%s", lastSuffix(allowLast))
	}
	month, err = strconv.Atoi(value[:2])
	if err != nil || month < 1 || month > 12 {
		return 0, 0, false, fmt.Errorf("must use a valid month in MM-DD%s", lastSuffix(allowLast))
	}
	dayText := value[3:]
	if dayText == "last" {
		if !allowLast {
			return 0, 0, false, fmt.Errorf("MM-last is only allowed for bas_end")
		}
		return month, 0, true, nil
	}
	if len(dayText) != 2 {
		return 0, 0, false, fmt.Errorf("must use MM-DD%s", lastSuffix(allowLast))
	}
	day, err = strconv.Atoi(dayText)
	if err != nil || day < 1 {
		return 0, 0, false, fmt.Errorf("must use a valid day in MM-DD%s", lastSuffix(allowLast))
	}
	date := time.Date(2001, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if int(date.Month()) != month || date.Day() != day {
		return 0, 0, false, fmt.Errorf("must be a valid MM-DD calendar date")
	}
	return month, day, false, nil
}

func lastSuffix(allow bool) string {
	if allow {
		return " or MM-last"
	}
	return ""
}
