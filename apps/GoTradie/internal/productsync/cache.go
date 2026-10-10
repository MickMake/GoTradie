package productsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MickMake/GoTradie/internal/config"
)

type Fingerprint struct {
	Provider            string                `json:"provider"`
	Supplier            string                `json:"supplier"`
	Aliases             []string              `json:"aliases,omitempty"`
	Source              string                `json:"source"`
	Hash                string                `json:"hash"`
	Fields              config.ProviderFields `json:"fields"`
	ETag                string                `json:"etag,omitempty"`
	LastModified        string                `json:"last_modified,omitempty"`
	LastSuccessfulCheck string                `json:"last_successful_check"`
}

type FingerprintStore interface {
	Get(provider string) (Fingerprint, bool, error)
	Put(Fingerprint) error
}

type FileFingerprintStore struct {
	Path string
}

func DefaultFingerprintPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("resolve home directory: empty path")
	}
	return filepath.Join(home, ".GoTradie", "product-sync-fingerprints.json"), nil
}

func (s FileFingerprintStore) Get(provider string) (Fingerprint, bool, error) {
	entries, err := s.read()
	if err != nil {
		return Fingerprint{}, false, err
	}
	fingerprint, ok := entries[provider]
	return fingerprint, ok, nil
}

func (s FileFingerprintStore) Put(fingerprint Fingerprint) error {
	entries, err := s.read()
	if err != nil {
		return err
	}
	entries[fingerprint.Provider] = fingerprint
	encoded, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Product Sync fingerprint cache: %w", err)
	}
	encoded = append(encoded, '\n')
	directory := filepath.Dir(s.Path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create Product Sync cache directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".product-sync-fingerprints-*")
	if err != nil {
		return fmt.Errorf("create Product Sync cache temporary file: %w", err)
	}
	temporaryName := temporary.Name()
	removeTemporary := true
	defer func() {
		_ = temporary.Close()
		if removeTemporary {
			_ = os.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("secure Product Sync cache temporary file: %w", err)
	}
	if _, err := temporary.Write(encoded); err != nil {
		return fmt.Errorf("write Product Sync cache: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync Product Sync cache: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close Product Sync cache: %w", err)
	}
	if err := os.Rename(temporaryName, s.Path); err != nil {
		return fmt.Errorf("replace Product Sync cache: %w", err)
	}
	removeTemporary = false
	return nil
}

func (s FileFingerprintStore) read() (map[string]Fingerprint, error) {
	entries := make(map[string]Fingerprint)
	encoded, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return entries, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read Product Sync fingerprint cache: %w", err)
	}
	if err := json.Unmarshal(encoded, &entries); err != nil {
		return nil, fmt.Errorf("decode Product Sync fingerprint cache: %w", err)
	}
	return entries, nil
}
