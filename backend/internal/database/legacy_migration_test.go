package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeSQLite(t *testing.T, path string, valid bool) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if valid {
		if _, err := db.Exec("CREATE TABLE settings(id INTEGER PRIMARY KEY, data TEXT); INSERT INTO settings VALUES(1,'{}')"); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("isto não é um sqlite válido"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMigrateLegacyVolumeNoSourceWritesMarker(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("MIGRATION_DATA_DIR", filepath.Join(dataDir, "sem-fonte"))
	if _, err := MigrateLegacyVolume(dataDir); err != nil {
		t.Fatal(err)
	}
	if !fileExists(filepath.Join(dataDir, "db", ".legacy-volume-migrated")) {
		t.Fatal("marker ausente sem fonte de migração")
	}
	// Idempotente: segunda execução também passa.
	if _, err := MigrateLegacyVolume(dataDir); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateLegacyVolumeMovesValidDatabase(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "migration")
	makeSQLite(t, filepath.Join(source, "db", "data.sqlite"), true)
	if err := os.WriteFile(filepath.Join(source, "extra.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(root, "data")
	t.Setenv("MIGRATION_DATA_DIR", source)
	if _, err := MigrateLegacyVolume(dataDir); err != nil {
		t.Fatal(err)
	}
	if !legacyValidateSQLite(filepath.Join(dataDir, "db", "data.sqlite")) {
		t.Fatal("banco migrado inválido")
	}
	if !fileExists(filepath.Join(dataDir, "extra.txt")) {
		t.Fatal("arquivo acessório não copiado")
	}
	if !fileExists(filepath.Join(dataDir, "db", ".legacy-volume-migrated")) {
		t.Fatal("marker ausente")
	}
	// Execuções seguintes preservam o banco e apenas completam arquivos.
	if _, err := MigrateLegacyVolume(dataDir); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateLegacyVolumePreservesValidDestination(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "migration")
	makeSQLite(t, filepath.Join(source, "db", "data.sqlite"), true)
	dataDir := filepath.Join(root, "data")
	makeSQLite(t, filepath.Join(dataDir, "db", "data.sqlite"), true)
	t.Setenv("MIGRATION_DATA_DIR", source)
	if _, err := MigrateLegacyVolume(dataDir); err != nil {
		t.Fatal(err)
	}
	if !legacyValidateSQLite(filepath.Join(dataDir, "db", "data.sqlite")) {
		t.Fatal("destino válido foi sobrescrito")
	}
}

func TestMigrateLegacyVolumeRejectsSidecarAndInvalidSource(t *testing.T) {
	for _, test := range []struct {
		name, sidecar string
		sourceInvalid bool
	}{
		{"sidecar wal", "-wal", false},
		{"sidecar shm", "-shm", false},
		{"fonte inválida", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "migration")
			makeSQLite(t, filepath.Join(source, "db", "data.sqlite"), !test.sourceInvalid)
			if test.sidecar != "" {
				if err := os.WriteFile(filepath.Join(source, "db", "data.sqlite"+test.sidecar), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			dataDir := filepath.Join(root, "data")
			t.Setenv("MIGRATION_DATA_DIR", source)
			_, err := MigrateLegacyVolume(dataDir)
			if err == nil {
				t.Fatal("esperava erro")
			}
			if !strings.Contains(err.Error(), "Migration source") {
				t.Fatalf("erro inesperado: %v", err)
			}
		})
	}
}

func TestMigrateLegacyVolumeInvalidDestinationWithoutSourceFails(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	makeSQLite(t, filepath.Join(dataDir, "db", "data.sqlite"), false)
	t.Setenv("MIGRATION_DATA_DIR", filepath.Join(root, "sem-fonte"))
	_, err := MigrateLegacyVolume(dataDir)
	if err == nil || !strings.Contains(err.Error(), "Canonical database is invalid") {
		t.Fatalf("esperava erro de banco canônico inválido, obtido %v", err)
	}
}
