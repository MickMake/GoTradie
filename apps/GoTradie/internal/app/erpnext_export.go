package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MickMake/GoTradie/internal/ninja"
)

func (a App) runNinjaERPNextExport(ctx context.Context, svc *ninja.Service, args []string) int {
	dir, commit, err := parseERPNextExportArgs(args)
	if err != nil {
		fmt.Fprintln(a.Err, err)
		fmt.Fprintln(a.Err, "usage: GoTradie ninja export erpnext <directory> [--commit]")
		return 2
	}

	export, err := svc.BuildERPNextExport(ctx)
	if err != nil {
		fmt.Fprintln(a.Err, "ERPNext export error:", err)
		return 1
	}

	paths, err := writeERPNextExport(dir, export.Files, commit)
	if err != nil {
		fmt.Fprintln(a.Err, "ERPNext export error:", err)
		return 1
	}
	for _, path := range paths {
		fmt.Fprintf(a.Out, "wrote %s\n", path)
	}
	return 0
}

func parseERPNextExportArgs(args []string) (string, bool, error) {
	commit := false
	var paths []string
	for _, arg := range args {
		switch {
		case arg == "--commit":
			commit = true
		case strings.HasPrefix(arg, "-"):
			return "", false, fmt.Errorf("unknown ERPNext export flag %q", arg)
		default:
			paths = append(paths, arg)
		}
	}
	if len(paths) != 1 {
		return "", false, fmt.Errorf("expected exactly one export directory, got %d", len(paths))
	}
	dir := strings.TrimSpace(paths[0])
	if dir == "" {
		return "", false, fmt.Errorf("ERPNext export directory is required")
	}
	return dir, commit, nil
}

type stagedERPNextFile struct {
	temporary string
	final     string
}

func writeERPNextExport(dir string, files []ninja.ERPNextFile, commit bool) ([]string, error) {
	for _, file := range files {
		path := filepath.Join(dir, file.Name)
		if !commit {
			if _, err := os.Stat(path); err == nil {
				return nil, fmt.Errorf("refusing to overwrite existing file %s; use --commit", path)
			} else if !os.IsNotExist(err) {
				return nil, err
			}
		}
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	staged := make([]stagedERPNextFile, 0, len(files))
	cleanup := func() {
		for _, file := range staged {
			_ = os.Remove(file.temporary)
		}
	}
	for _, file := range files {
		tmp, err := os.CreateTemp(dir, ".gotradie-erpnext-*")
		if err != nil {
			cleanup()
			return nil, err
		}
		tmpName := tmp.Name()
		if err := tmp.Chmod(0644); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			cleanup()
			return nil, err
		}
		if _, err := tmp.Write(file.Data); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			cleanup()
			return nil, err
		}
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmpName)
			cleanup()
			return nil, err
		}
		staged = append(staged, stagedERPNextFile{temporary: tmpName, final: filepath.Join(dir, file.Name)})
	}

	paths := make([]string, 0, len(staged))
	for _, file := range staged {
		if err := os.Rename(file.temporary, file.final); err != nil {
			cleanup()
			return nil, err
		}
		paths = append(paths, file.final)
	}
	return paths, nil
}
