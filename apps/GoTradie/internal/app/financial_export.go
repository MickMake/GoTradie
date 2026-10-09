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
	"github.com/MickMake/GoTradie/internal/financial"
	"github.com/MickMake/GoTradie/internal/ninja"
)

func (a App) runNinjaFinancialExport(ctx context.Context, svc *ninja.Service, cfg config.Config, args []string) int {
	options, force, err := parseFinancialExportArgs(args)
	if err != nil {
		fmt.Fprintln(a.Err, err)
		fmt.Fprintln(a.Err, "usage: GoTradie ninja export financial [--fy YYYY] [--period VALUE] [--from YYYY-MM-DD] [--to YYYY-MM-DD] [--force]")
		return 2
	}
	now := time.Now()
	if a.Now != nil {
		now = a.Now()
	}
	options.Now = now
	selection, err := financial.ResolveSelection(cfg.BAS, options)
	if err != nil {
		fmt.Fprintln(a.Err, "Financial selection error:", err)
		return 2
	}
	directory, err := generatedExportDirectory(cfg.Exports)
	if err != nil {
		fmt.Fprintln(a.Err, "Financial export error:", err)
		return 1
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		fmt.Fprintln(a.Err, "Financial export error:", err)
		return 1
	}
	path := filepath.Join(directory, financial.Filename(selection))
	if !force {
		if _, statErr := os.Stat(path); statErr == nil {
			fmt.Fprintf(a.Err, "refusing to overwrite existing file %s; use --force\n", path)
			return 1
		} else if !errors.Is(statErr, os.ErrNotExist) {
			fmt.Fprintln(a.Err, "Financial export error:", statErr)
			return 1
		}
	}

	source, err := svc.LoadFinancialSource(ctx)
	if err != nil {
		fmt.Fprintln(a.Err, "Financial export error:", err)
		return 1
	}
	report, err := financial.Build(source, cfg.BAS, options, now)
	if err != nil {
		fmt.Fprintln(a.Err, "Financial export error:", err)
		return 1
	}
	workbook, err := financial.Workbook(report)
	if err != nil {
		fmt.Fprintln(a.Err, "Financial workbook error:", err)
		return 1
	}
	if err := writeGeneratedReportFile(path, workbook, force); err != nil {
		fmt.Fprintln(a.Err, "Financial export error:", err)
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

func parseFinancialExportArgs(args []string) (financial.Options, bool, error) {
	fs := flag.NewFlagSet("ninja export financial", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fy := fs.String("fy", "", "Australian financial year ending in YYYY")
	period := fs.String("period", "", "BAS reporting period")
	from := fs.String("from", "", "inclusive source-date lower bound")
	to := fs.String("to", "", "inclusive source-date upper bound")
	force := fs.Bool("force", false, "overwrite an existing workbook")
	if err := fs.Parse(args); err != nil {
		return financial.Options{}, false, err
	}
	if fs.NArg() != 0 {
		return financial.Options{}, false, fmt.Errorf("Financial export does not accept a positional output path")
	}
	return financial.Options{FY: *fy, Period: *period, From: *from, To: *to}, *force, nil
}
