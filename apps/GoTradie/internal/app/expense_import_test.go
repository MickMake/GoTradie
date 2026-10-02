package app

import "testing"

func TestParseExpenseImportArgs(t *testing.T) {
	path, receipts, dryRun, err := parseExpenseImportArgs([]string{
		"purchases.csv",
		"--receipts-root",
		"receipts",
		"--commit",
	})
	if err != nil {
		t.Fatal(err)
	}
	if path != "purchases.csv" || receipts != "receipts" || dryRun {
		t.Fatalf("unexpected parse result: path=%q receipts=%q dryRun=%v", path, receipts, dryRun)
	}
}

func TestParseExpenseImportArgsRejectsBlankReceiptsRoot(t *testing.T) {
	if _, _, _, err := parseExpenseImportArgs([]string{"purchases.csv", "--receipts-root="}); err == nil {
		t.Fatal("expected blank receipts root error")
	}
}

func TestImportParsersAcceptStdinPath(t *testing.T) {
	path, dryRun, err := parseImportArgs([]string{"-"})
	if err != nil || path != "-" || !dryRun {
		t.Fatalf("generic import parse: path=%q dryRun=%v err=%v", path, dryRun, err)
	}
	path, receipts, dryRun, err := parseExpenseImportArgs([]string{"-"})
	if err != nil || path != "-" || receipts != "" || !dryRun {
		t.Fatalf("expense import parse: path=%q receipts=%q dryRun=%v err=%v", path, receipts, dryRun, err)
	}
}
