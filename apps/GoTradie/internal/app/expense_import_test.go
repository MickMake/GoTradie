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
