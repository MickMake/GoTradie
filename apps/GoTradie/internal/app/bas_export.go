package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MickMake/GoTradie/internal/bas"
	"github.com/MickMake/GoTradie/internal/config"
	"github.com/MickMake/GoTradie/internal/ninja"
)

func (a App) runNinjaBASExport(ctx context.Context, svc *ninja.Service, cfg config.Config, args []string) int {
	options, force, err := parseBASExportArgs(args)
	if err != nil {
		fmt.Fprintln(a.Err, err)
		fmt.Fprintln(a.Err, "usage: GoTradie ninja export bas [--fy YYYY] [--period VALUE] [--force]")
		return 2
	}
	now := time.Now()
	if a.Now != nil {
		now = a.Now()
	}
	options.Now = now
	periods, err := bas.ResolvePeriods(cfg.BAS, options)
	if err != nil {
		fmt.Fprintln(a.Err, "BAS selection error:", err)
		return 2
	}
	directory, err := generatedExportDirectory(cfg.Exports)
	if err != nil {
		fmt.Fprintln(a.Err, "BAS export error:", err)
		return 1
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		fmt.Fprintln(a.Err, "BAS export error:", err)
		return 1
	}
	path := filepath.Join(directory, bas.Filename(periods))
	if !force {
		if _, statErr := os.Stat(path); statErr == nil {
			fmt.Fprintf(a.Err, "refusing to overwrite existing file %s; use --force\n", path)
			return 1
		} else if !errors.Is(statErr, os.ErrNotExist) {
			fmt.Fprintln(a.Err, "BAS export error:", statErr)
			return 1
		}
	}

	dataset, err := svc.BuildAccountingDataset(ctx)
	if err != nil {
		fmt.Fprintln(a.Err, "BAS export error:", err)
		return 1
	}
	report, err := bas.Build(dataset, cfg.BAS, options, now)
	if err != nil {
		fmt.Fprintln(a.Err, "BAS export error:", err)
		return 1
	}
	workbook, err := bas.Workbook(report)
	if err != nil {
		fmt.Fprintln(a.Err, "BAS workbook error:", err)
		return 1
	}
	if err := writeGeneratedReportFile(path, workbook, force); err != nil {
		fmt.Fprintln(a.Err, "BAS export error:", err)
		return 1
	}
	for _, row := range report.Exceptions {
		fmt.Fprintf(a.Err, "%s %s %s: %s\n", row.Exception.Severity, row.Exception.SourceType, row.Exception.SourceID, row.Exception.Message)
	}
	fmt.Fprintf(a.Out, "wrote %s\n", path)
	fmt.Fprintf(a.Out, "Report Status: %s\n", report.Status)
	if report.HasErrors() {
		return 1
	}
	return 0
}

func parseBASExportArgs(args []string) (bas.Options, bool, error) {
	fs := flag.NewFlagSet("ninja export bas", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fy := fs.String("fy", "", "Australian financial year ending in YYYY")
	period := fs.String("period", "", "BAS reporting period")
	force := fs.Bool("force", false, "overwrite an existing workbook")
	if err := fs.Parse(args); err != nil {
		return bas.Options{}, false, err
	}
	if fs.NArg() != 0 {
		return bas.Options{}, false, fmt.Errorf("BAS export does not accept a positional output path")
	}
	return bas.Options{FY: *fy, Period: *period}, *force, nil
}

func generatedExportDirectory(cfg config.ExportsConfig) (string, error) {
	if cfg.Directory == nil {
		return ".", nil
	}
	directory := strings.TrimSpace(*cfg.Directory)
	if directory == "~" || strings.HasPrefix(directory, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		if directory == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(directory, "~/")), nil
	}
	return directory, nil
}

func writeGeneratedReportFile(path string, data []byte, force bool) error {
	if force {
		temporary, err := os.CreateTemp(filepath.Dir(path), ".gotradie-report-*.xlsx")
		if err != nil {
			return err
		}
		temporaryPath := temporary.Name()
		defer func() { _ = os.Remove(temporaryPath) }()
		if err := temporary.Chmod(0644); err != nil {
			_ = temporary.Close()
			return err
		}
		if _, err := temporary.Write(data); err != nil {
			_ = temporary.Close()
			return err
		}
		if err := temporary.Close(); err != nil {
			return err
		}
		return os.Rename(temporaryPath, path)
	}

	flags := os.O_WRONLY | os.O_CREATE
	flags |= os.O_EXCL
	file, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("refusing to overwrite existing file %s; use --force", path)
		}
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}
