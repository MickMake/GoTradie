package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	gobunnings "github.com/MickMake/GoBunnings"
)

const DefaultConfigPath = "gobunningsninja.conf"

type Config struct {
	InvoiceNinjaURL   string
	InvoiceNinjaToken string
	BunningsEnv       gobunnings.Env
	BunningsClientID  string
	BunningsSecret    string
	BunningsScopes    []string
	Country           gobunnings.CountryCode
	LocationCode      string
	ProductPrefix     string
	BunningsCustom    int
	ImageURLCustom    int
	TaxName           string
	TaxRate           float64
}

func FromEnv() (Config, error) {
	cfg := defaultsFromEnv()
	return cfg, nil
}

func FromEnvAndFile(path string) (Config, error) {
	cfg := defaultsFromEnv()
	if path == "" {
		path = strings.TrimSpace(os.Getenv("GOBUNNINGSNINJA_CONFIG"))
	}
	if path == "" {
		if _, err := os.Stat(DefaultConfigPath); err == nil {
			path = DefaultConfigPath
		}
	}
	if path == "" {
		return cfg, nil
	}
	if err := applyFile(&cfg, path); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func defaultsFromEnv() Config {
	return Config{
		InvoiceNinjaURL:   getenv("INVOICE_NINJA_URL", ""),
		InvoiceNinjaToken: getenv("INVOICE_NINJA_TOKEN", ""),
		BunningsEnv:       gobunnings.Env(getenv("BUNNINGS_ENV", "live")),
		BunningsClientID:  getenv("BUNNINGS_CLIENT_ID", ""),
		BunningsSecret:    getenv("BUNNINGS_CLIENT_SECRET", ""),
		BunningsScopes:    fields(getenv("BUNNINGS_SCOPES", "")),
		Country:           gobunnings.CountryCode(getenv("BUNNINGS_COUNTRY", "AU")),
		LocationCode:      getenv("BUNNINGS_LOCATION", ""),
		ProductPrefix:     getenv("PRODUCT_PREFIX", "BUNNINGS-"),
		BunningsCustom:    getenvInt("BUNNINGS_IN_CUSTOM_FIELD", 1),
		ImageURLCustom:    getenvInt("BUNNINGS_IMAGE_CUSTOM_FIELD", 2),
		TaxName:           getenv("TAX_NAME", "GST"),
		TaxRate:           getenvFloat("TAX_RATE", 10),
	}
}

func (c Config) Validate() error {
	if err := c.ValidateInvoiceNinja(); err != nil {
		return err
	}
	return c.ValidateBunnings()
}

func (c Config) ValidateBunnings() error {
	var missing []string
	if c.BunningsClientID == "" {
		missing = append(missing, "BUNNINGS_CLIENT_ID")
	}
	if c.BunningsSecret == "" {
		missing = append(missing, "BUNNINGS_CLIENT_SECRET")
	}
	if c.BunningsEnv != gobunnings.EnvLive && c.BunningsEnv != gobunnings.EnvTest && c.BunningsEnv != gobunnings.EnvSandbox {
		return fmt.Errorf("BUNNINGS_ENV must be live, test, or sandbox")
	}
	if c.Country != gobunnings.CountryAU && c.Country != gobunnings.CountryNZ {
		return fmt.Errorf("BUNNINGS_COUNTRY must be AU or NZ")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}
	return nil
}

func (c Config) ValidateInvoiceNinja() error {
	var missing []string
	if c.InvoiceNinjaToken == "" {
		missing = append(missing, "INVOICE_NINJA_TOKEN")
	}
	if c.BunningsCustom < 1 || c.BunningsCustom > 4 || c.ImageURLCustom < 1 || c.ImageURLCustom > 4 {
		return errors.New("custom field indexes must be between 1 and 4")
	}
	if c.BunningsCustom == c.ImageURLCustom {
		return errors.New("BUNNINGS_IN_CUSTOM_FIELD and BUNNINGS_IMAGE_CUSTOM_FIELD must be different")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}
	return nil
}

func applyFile(cfg *Config, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open config %s: %w", path, err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	lineNo := 0
	for s.Scan() {
		lineNo++
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("config %s:%d: expected key=value", path, lineNo)
		}
		key = normalizeKey(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, "\"'")
		if err := set(cfg, key, value); err != nil {
			return fmt.Errorf("config %s:%d: %w", path, lineNo, err)
		}
	}
	if err := s.Err(); err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	return nil
}

func normalizeKey(k string) string {
	k = strings.TrimSpace(k)
	k = strings.ReplaceAll(k, "-", "_")
	k = strings.ReplaceAll(k, ".", "_")
	return strings.ToUpper(k)
}

func set(cfg *Config, key, value string) error {
	switch key {
	case "INVOICE_NINJA_URL":
		cfg.InvoiceNinjaURL = value
	case "INVOICE_NINJA_TOKEN":
		cfg.InvoiceNinjaToken = value
	case "BUNNINGS_ENV":
		cfg.BunningsEnv = gobunnings.Env(value)
	case "BUNNINGS_CLIENT_ID":
		cfg.BunningsClientID = value
	case "BUNNINGS_CLIENT_SECRET":
		cfg.BunningsSecret = value
	case "BUNNINGS_SCOPES":
		cfg.BunningsScopes = fields(value)
	case "BUNNINGS_COUNTRY":
		cfg.Country = gobunnings.CountryCode(value)
	case "BUNNINGS_LOCATION":
		cfg.LocationCode = value
	case "PRODUCT_PREFIX":
		cfg.ProductPrefix = value
	case "BUNNINGS_IN_CUSTOM_FIELD":
		n, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		cfg.BunningsCustom = n
	case "BUNNINGS_IMAGE_CUSTOM_FIELD":
		n, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		cfg.ImageURLCustom = n
	case "TAX_NAME":
		cfg.TaxName = value
	case "TAX_RATE":
		n, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		cfg.TaxRate = n
	default:
		return fmt.Errorf("unknown key %q", key)
	}
	return nil
}

func getenv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func getenvInt(k string, def int) int {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getenvFloat(k string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return n
}

func fields(v string) []string {
	return strings.Fields(strings.ReplaceAll(v, ",", " "))
}
