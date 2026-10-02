package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MickMake/GoTradie/internal/ninja"
)

func TestWriteERPNextExportRefusesOverwriteWithoutCommit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Customer.csv")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	files := []ninja.ERPNextFile{{Name: "Customer.csv", Data: []byte("new")}}
	if _, err := writeERPNextExport(dir, files, false); err == nil {
		t.Fatal("expected overwrite refusal")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old" {
		t.Fatalf("existing file changed to %q", data)
	}
}

func TestWriteERPNextExportOverwritesWithCommit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Customer.csv")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	files := []ninja.ERPNextFile{{Name: "Customer.csv", Data: []byte("new")}}
	if _, err := writeERPNextExport(dir, files, true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("file = %q; want new", data)
	}
}

func TestParseERPNextExportArgs(t *testing.T) {
	dir, commit, err := parseERPNextExportArgs([]string{"output", "--commit"})
	if err != nil {
		t.Fatal(err)
	}
	if dir != "output" || !commit {
		t.Fatalf("got dir=%q commit=%v", dir, commit)
	}
}
