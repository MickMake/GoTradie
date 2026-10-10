package productsync

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MickMake/GoTradie/internal/config"
)

func TestFileFingerprintStoreRoundTripsPrivateAtomicCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "fingerprints.json")
	store := FileFingerprintStore{Path: path}
	want := Fingerprint{
		Provider: "generic", Supplier: "Generic Supplier", Aliases: []string{"Generic Wholesale"},
		Source: "https://example.test/catalogue.csv", Hash: "abc123",
		Fields: config.ProviderFields{Product: "SKU", Price: "RetailPrice"},
		ETag:   `"v1"`, LastSuccessfulCheck: "2026-10-10T12:00:00+11:00",
	}
	if err := store.Put(want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.Get("generic")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("fingerprint = %#v, %v; want %#v", got, ok, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("cache mode = %o; want 600", mode)
	}
}
