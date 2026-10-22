package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSPADirValidadoEAceito(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := load(func(key string) string {
		if key == "SPA_DIR" {
			return dir
		}
		return ""
	}, func() (string, error) { return t.TempDir(), nil }, "linux")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SPADir != dir {
		t.Fatalf("SPADir %q != %q", loaded.SPADir, dir)
	}
}
