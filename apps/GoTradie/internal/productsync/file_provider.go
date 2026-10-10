package productsync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/MickMake/GoTradie/internal/config"
	"github.com/xuri/excelize/v2"
)

type downloadedCatalogue struct {
	Data         []byte
	Hash         string
	ETag         string
	LastModified string
}

func downloadCatalogue(ctx context.Context, client *http.Client, provider config.ProviderConfig) (downloadedCatalogue, error) {
	if client == nil {
		client = http.DefaultClient
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.URL, nil)
	if err != nil {
		return downloadedCatalogue{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return downloadedCatalogue{}, fmt.Errorf("download catalogue: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return downloadedCatalogue{}, fmt.Errorf("download catalogue: HTTP %s", response.Status)
	}
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return downloadedCatalogue{}, fmt.Errorf("read catalogue: %w", err)
	}
	digest := sha256.Sum256(data)
	return downloadedCatalogue{
		Data: data, Hash: hex.EncodeToString(digest[:]),
		ETag: response.Header.Get("ETag"), LastModified: response.Header.Get("Last-Modified"),
	}, nil
}

func parseCatalogue(data []byte, provider config.ProviderConfig) ([]observation, error) {
	var records [][]string
	var err error
	switch provider.Type {
	case "csv":
		records, err = csv.NewReader(bytes.NewReader(data)).ReadAll()
	case "xlsx":
		records, err = xlsxRecords(data)
	default:
		return nil, fmt.Errorf("unsupported file Provider type %q", provider.Type)
	}
	if err != nil {
		return nil, fmt.Errorf("parse %s catalogue: %w", provider.Type, err)
	}
	return mapCatalogueRecords(records, provider.Fields)
}

func xlsxRecords(data []byte) ([][]string, error) {
	workbook, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer workbook.Close()
	sheets := workbook.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("workbook has no worksheets")
	}
	return workbook.GetRows(sheets[0])
}

func mapCatalogueRecords(records [][]string, fields config.ProviderFields) ([]observation, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("catalogue is empty")
	}
	headers := make(map[string]int, len(records[0]))
	headerCounts := make(map[string]int, len(records[0]))
	for index, header := range records[0] {
		header = strings.TrimSpace(header)
		headers[header] = index
		headerCounts[header]++
	}
	mappings := []string{fields.Product, fields.Description, fields.Cost, fields.Price, fields.Quantity, fields.ImageURL}
	for _, mapping := range mappings {
		if mapping == "" {
			continue
		}
		if _, ok := headers[mapping]; !ok {
			return nil, fmt.Errorf("catalogue is missing configured column %q", mapping)
		}
		if headerCounts[mapping] != 1 {
			return nil, fmt.Errorf("catalogue contains duplicate configured column %q", mapping)
		}
	}

	rows := make([]observation, 0, len(records)-1)
	seen := make(map[string]observation)
	for index, record := range records[1:] {
		if emptyRecord(record) {
			continue
		}
		rowNumber := index + 2
		key := strings.TrimSpace(recordValue(record, headers[fields.Product]))
		if key == "" {
			return nil, fmt.Errorf("catalogue row %d: configured Product value is blank", rowNumber)
		}
		row := observation{Key: key, Availability: availabilityAvailable}
		if fields.Description != "" {
			row.Description = stringPointer(strings.TrimSpace(recordValue(record, headers[fields.Description])))
		}
		if fields.ImageURL != "" {
			row.ImageURL = stringPointer(strings.TrimSpace(recordValue(record, headers[fields.ImageURL])))
		}
		var err error
		if fields.Cost != "" {
			row.Cost, err = numericField(recordValue(record, headers[fields.Cost]))
			if err != nil {
				return nil, fmt.Errorf("catalogue row %d column %q: %w", rowNumber, fields.Cost, err)
			}
		}
		if fields.Price != "" {
			row.Price, err = numericField(recordValue(record, headers[fields.Price]))
			if err != nil {
				return nil, fmt.Errorf("catalogue row %d column %q: %w", rowNumber, fields.Price, err)
			}
		}
		if fields.Quantity != "" {
			row.Quantity, err = numericField(recordValue(record, headers[fields.Quantity]))
			if err != nil {
				return nil, fmt.Errorf("catalogue row %d column %q: %w", rowNumber, fields.Quantity, err)
			}
		}
		if previous, ok := seen[key]; ok {
			if !sameObservation(previous, row) {
				return nil, fmt.Errorf("catalogue contains conflicting duplicate Product %q", key)
			}
			continue
		}
		seen[key] = row
		rows = append(rows, row)
	}
	return rows, nil
}

func numericField(value string) (*float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return nil, fmt.Errorf("invalid number %q", value)
	}
	return floatPointer(parsed), nil
}

func recordValue(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return record[index]
}

func emptyRecord(record []string) bool {
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func sameObservation(left, right observation) bool {
	return left.Key == right.Key && equalString(left.Description, right.Description) &&
		equalFloat(left.Cost, right.Cost) && equalFloat(left.Price, right.Price) &&
		equalFloat(left.Quantity, right.Quantity) && equalString(left.ImageURL, right.ImageURL)
}

func equalString(left, right *string) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func equalFloat(left, right *float64) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}
