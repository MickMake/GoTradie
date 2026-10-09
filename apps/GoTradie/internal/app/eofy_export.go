package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/MickMake/GoTradie/internal/config"
	"github.com/MickMake/GoTradie/internal/eofy"
	"github.com/MickMake/GoTradie/internal/ninja"
)

func (a App) runNinjaEOFYExport(ctx context.Context, svc *ninja.Service, cfg config.Config, args []string) int {
	options, force, err := parseEOFYExportArgs(args)
	if err != nil {
		fmt.Fprintln(a.Err, err)
		fmt.Fprintln(a.Err, "usage: GoTradie ninja export eofy [--fy YYYY] [--force]")
		return 2
	}
	now := time.Now()
	if a.Now != nil {
		now = a.Now()
	}
	options.Now = now
	period, err := eofy.ResolvePeriod(options)
	if err != nil {
		fmt.Fprintln(a.Err, "EOFY selection error:", err)
		return 2
	}
	directory, err := generatedExportDirectory(cfg.Exports)
	if err != nil {
		fmt.Fprintln(a.Err, "EOFY export error:", err)
		return 1
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		fmt.Fprintln(a.Err, "EOFY export error:", err)
		return 1
	}
	path := filepath.Join(directory, eofy.Filename(period))
	if !force {
		if _, statErr := os.Stat(path); statErr == nil {
			fmt.Fprintf(a.Err, "refusing to overwrite existing file %s; use --force\n", path)
			return 1
		} else if !errors.Is(statErr, os.ErrNotExist) {
			fmt.Fprintln(a.Err, "EOFY export error:", statErr)
			return 1
		}
	}

	dataset, err := svc.BuildAccountingDataset(ctx)
	if err != nil {
		fmt.Fprintln(a.Err, "EOFY export error:", err)
		return 1
	}
	report, err := eofy.Build(dataset, cfg.EOFY, options, now)
	if err != nil {
		fmt.Fprintln(a.Err, "EOFY export error:", err)
		return 1
	}
	workbook, err := eofy.Workbook(report)
	if err != nil {
		fmt.Fprintln(a.Err, "EOFY workbook error:", err)
		return 1
	}
	if err := writeGeneratedReportFile(path, workbook, force); err != nil {
		fmt.Fprintln(a.Err, "EOFY export error:", err)
		return 1
	}
	for _, exception := range report.Exceptions {
		fmt.Fprintf(a.Err, "%s %s %s: %s\n", exception.Severity, exception.SourceType, exception.SourceID, exception.Message)
	}
	fmt.Fprintf(a.Out, "wrote %s\n", path)
	fmt.Fprintf(a.Out, "Report Status: %s\n", report.Status)
	if report.HasErrors() {
		return 1
	}
	return 0
}

func parseEOFYExportArgs(args []string) (eofy.Options, bool, error) {
	fs := flag.NewFlagSet("ninja export eofy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fy := fs.String("fy", "", "Australian financial year ending in YYYY")
	force := fs.Bool("force", false, "overwrite an existing workbook")
	if err := fs.Parse(args); err != nil {
		return eofy.Options{}, false, err
	}
	if fs.NArg() != 0 {
		return eofy.Options{}, false, fmt.Errorf("EOFY export does not accept a positional output path")
	}
	return eofy.Options{FY: *fy}, *force, nil
}
