package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestHasLegacyConfigFlag(t *testing.T) {
	for _, args := range [][]string{
		{"--config", "old.conf", "version"},
		{"--config=old.conf", "version"},
		{"ninja", "export", "products", "-", "-config"},
	} {
		if !hasLegacyConfigFlag(args) {
			t.Fatalf("hasLegacyConfigFlag(%q) = false; want true", args)
		}
	}
	if hasLegacyConfigFlag([]string{"ninja", "export", "products", "-"}) {
		t.Fatal("ordinary command was treated as legacy config usage")
	}
}

func TestRunRejectsLegacyConfigFlag(t *testing.T) {
	var stderr bytes.Buffer
	code := (App{Out: &bytes.Buffer{}, Err: &stderr}).Run(context.Background(), []string{
		"ninja", "export", "products", "-", "--config=old.conf",
	})
	if code != 2 || !strings.Contains(stderr.String(), "--config was removed") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}
