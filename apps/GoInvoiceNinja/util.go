package goinvoiceninja

import (
	"errors"
	"strconv"
	"strings"
)

var ErrNotFound = errors.New("invoice ninja resource not found")

func itoa(i int) string           { return strconv.Itoa(i) }
func itoa64(i int64) string       { return strconv.FormatInt(i, 10) }
func joinComma(v []string) string { return strings.Join(v, ",") }
func statuses(v []InvoiceStatus) string {
	parts := make([]string, 0, len(v))
	for _, s := range v {
		parts = append(parts, strconv.Itoa(int(s)))
	}
	return strings.Join(parts, ",")
}
