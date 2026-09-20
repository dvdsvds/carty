// Package cache stores the data repository's index.json and individual
// config files locally so browsing/searching keeps working offline, per
// the README's caching section.
package cache

import (
	"os"
	"path/filepath"
)

func IndexPath(cacheDir string) string {
	return filepath.Join(cacheDir, "index.json")
}

func SaveIndex(cacheDir string, data []byte) error {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(IndexPath(cacheDir), data, 0644)
}

func LoadIndex(cacheDir string) ([]byte, error) {
	return os.ReadFile(IndexPath(cacheDir))
}

func filePath(cacheDir, itemFilePath string) string {
	return filepath.Join(cacheDir, "files", filepath.FromSlash(itemFilePath))
}

func SaveFile(cacheDir, itemFilePath string, data []byte) error {
	dst := filePath(cacheDir, itemFilePath)
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func LoadFile(cacheDir, itemFilePath string) ([]byte, error) {
	return os.ReadFile(filePath(cacheDir, itemFilePath))
}
