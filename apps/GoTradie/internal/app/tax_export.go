package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MickMake/GoTradie/internal/ninja"
)

func (a App) runNinjaTaxExport(ctx context.Context, svc *ninja.Service, args []string) int {
	dir, commit, err := parseTaxExportArgs(args)
	if err != nil {
		fmt.Fprintln(a.Err, err)
		fmt.Fprintln(a.Err, "usage: GoTradie ninja export tax [directory] [--commit]")
		return 2
	}

	export, err := svc.BuildTaxExport(ctx)
	if err != nil {
		fmt.Fprintln(a.Err, "tax export error:", err)
		return 1
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintln(a.Err, "tax export error:", err)
		return 1
	}

	invoicesPath := filepath.Join(dir, "Invoices.csv")
	detailPath := filepath.Join(dir, "Detail.csv")
	customersPath := filepath.Join(dir, "Customers.csv")
	if !commit {
		for _, path := range []string{invoicesPath, detailPath, customersPath} {
			if _, err := os.Stat(path); err == nil {
				fmt.Fprintf(a.Err, "refusing to overwrite existing file %s; use --commit\n", path)
				return 1
			} else if !os.IsNotExist(err) {
				fmt.Fprintln(a.Err, "tax export error:", err)
				return 1
			}
		}
	}

	if err := os.WriteFile(invoicesPath, export.Invoices, 0644); err != nil {
		fmt.Fprintln(a.Err, "tax export error:", err)
		return 1
	}
	if err := os.WriteFile(detailPath, export.Detail, 0644); err != nil {
		fmt.Fprintln(a.Err, "tax export error:", err)
		return 1
	}
	if err := os.WriteFile(customersPath, export.Customers, 0644); err != nil {
		fmt.Fprintln(a.Err, "tax export error:", err)
		return 1
	}

	fmt.Fprintf(a.Out, "wrote %s\n", invoicesPath)
	fmt.Fprintf(a.Out, "wrote %s\n", detailPath)
	fmt.Fprintf(a.Out, "wrote %s\n", customersPath)
	return 0
}

func parseTaxExportArgs(args []string) (string, bool, error) {
	dir := "."
	commit := false
	var paths []string
	for _, arg := range args {
		switch {
		case arg == "--commit":
			commit = true
		case strings.HasPrefix(arg, "-"):
			return "", false, fmt.Errorf("unknown tax export flag %q", arg)
		default:
			paths = append(paths, arg)
		}
	}
	if len(paths) > 1 {
		return "", false, fmt.Errorf("expected zero or one export directory, got %d", len(paths))
	}
	if len(paths) == 1 {
		dir = paths[0]
	}
	if strings.TrimSpace(dir) == "" {
		return "", false, fmt.Errorf("tax export directory is required")
	}
	return dir, commit, nil
}
