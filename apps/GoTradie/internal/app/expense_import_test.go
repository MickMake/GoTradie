package app

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/MickMake/GoTradie/internal/ninja"
)

func TestParseExpenseImportArgs(t *testing.T) {
	path, receipts, batchSize, pause, dryRun, err := parseExpenseImportArgs([]string{
		"purchases.csv",
		"--receipts-root",
		"receipts",
		"--batch-size=100",
		"--pause",
		"--commit",
	})
	if err != nil {
		t.Fatal(err)
	}
	if path != "purchases.csv" || receipts != "receipts" || batchSize != 100 || !pause || dryRun {
		t.Fatalf("unexpected parse result: path=%q receipts=%q batch=%d pause=%v dryRun=%v", path, receipts, batchSize, pause, dryRun)
	}
}

func TestParseExpenseImportArgsRejectsBlankReceiptsRoot(t *testing.T) {
	if _, _, _, _, _, err := parseExpenseImportArgs([]string{"purchases.csv", "--receipts-root="}); err == nil {
		t.Fatal("expected blank receipts root error")
	}
}

func TestExpenseImportRejectsStdinPath(t *testing.T) {
	path, dryRun, err := parseImportArgs([]string{"-"})
	if err != nil || path != "-" || !dryRun {
		t.Fatalf("generic import parse: path=%q dryRun=%v err=%v", path, dryRun, err)
	}
	if _, _, _, _, _, err := parseExpenseImportArgs([]string{"-"}); err == nil {
		t.Fatal("expected Expense import stdin rejection")
	}
}

func TestParseExpenseImportArgsRejectsInvalidBatchSize(t *testing.T) {
	for _, value := range []string{"0", "-1", "nope"} {
		if _, _, _, _, _, err := parseExpenseImportArgs([]string{"purchases.csv", "--batch-size=" + value}); err == nil {
			t.Fatalf("expected invalid batch size %q to fail", value)
		}
	}
}

func TestParseExpenseImportArgsRejectsPauseWithoutBatchSize(t *testing.T) {
	if _, _, _, _, _, err := parseExpenseImportArgs([]string{"purchases.csv", "--pause"}); err == nil {
		t.Fatal("expected --pause without --batch-size to fail")
	}
}

func TestReadExpenseBatchDecisionReusesBufferedInputAndStopsOnEOF(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("yes\nno\n"))
	for index, want := range []bool{true, false} {
		got, err := readExpenseBatchDecision(reader)
		if err != nil || got != want {
			t.Fatalf("decision %d = %v, %v; want %v", index+1, got, err, want)
		}
	}
	if got, err := readExpenseBatchDecision(bufio.NewReader(strings.NewReader(""))); err != nil || got {
		t.Fatalf("EOF decision = %v, %v; want stop", got, err)
	}
}

func TestExpenseImportPreviewSummaryDoesNotImplyWrites(t *testing.T) {
	var out bytes.Buffer
	printExpenseImportSummary(&out, []ninja.CSVImportResult{
		{RowNo: 2, Action: "would-create"},
		{RowNo: 3, Action: "would-update"},
	}, true, true)
	got := out.String()
	for _, want := range []string{"Would create: 1", "Would update: 1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("preview summary %q does not contain %q", got, want)
		}
	}
	if strings.Contains(got, "\nNew:") || strings.Contains(got, "\nUpdated:") {
		t.Fatalf("preview summary implies writes: %q", got)
	}
}
