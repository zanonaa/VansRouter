package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateFreshAndUpgrade(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, dir, "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	if _, err = (Nodes{DB: db}).Create(ctx, map[string]any{"name": "N", "type": "openai-compatible"}); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO apiKeys(id,key,createdAt,allowedProviders) VALUES('k','secret','now','[]'); CREATE TABLE user_work(id TEXT); INSERT INTO user_work VALUES('preserved')`); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	var permission string
	if err = db.QueryRow(`SELECT allowedProviders FROM apiKeys WHERE id='k'`).Scan(&permission); err != nil || permission != "[]" {
		t.Fatalf("modern ACL changed: %s %v", permission, err)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM user_work`).Scan(&count); err != nil || count != 1 {
		t.Fatal("unknown table lost")
	}
	backups, err := filepath.Glob(filepath.Join(dir, "db", "backups", "*.sqlite"))
	if err != nil || len(backups) != 0 {
		t.Fatalf("unchanged boot created backup: %v %v", backups, err)
	}
	if _, err = db.Exec(`UPDATE _meta SET value='1' WHERE key='schemaVersion'`); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	backups, _ = filepath.Glob(filepath.Join(dir, "db", "backups", "*.sqlite"))
	if len(backups) != 1 {
		t.Fatalf("upgrade backup count: %d", len(backups))
	}
	if err = Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	backups, _ = filepath.Glob(filepath.Join(dir, "db", "backups", "*.sqlite"))
	if len(backups) != 1 {
		t.Fatal("repeat boot created backup")
	}
	var value any
	if err = db.QueryRow(`SELECT allowedProviders FROM apiKeys WHERE id='k'`).Scan(&value); err != nil || value != nil {
		t.Fatalf("old ACL migration not applied: %v %v", value, err)
	}
}
func TestMigrateLegacyJSONRefused(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "db.json"), []byte(`{"settings":{"password":"existing"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, dir, "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = Migrate(ctx, db, dir); err == nil {
		t.Fatal("legacy JSON silently ignored")
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("schema mutated before legacy import")
	}
}
func TestMigrateRollbackAndBackup(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, dir, "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE providerConnections(id TEXT); INSERT INTO providerConnections VALUES('existing')`); err != nil {
		t.Fatal(err)
	}
	// Colunas NOT NULL não podem ser adicionadas sem default a tabela populada.
	if err = Migrate(ctx, db, dir); err == nil {
		t.Fatal("incompatible schema should fail")
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM providerConnections`).Scan(&count); err != nil || count != 1 {
		t.Fatal("existing rows lost")
	}
	if err = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='_meta'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("transaction not rolled back")
	}
	backups, _ := filepath.Glob(filepath.Join(dir, "db", "backups", "*.sqlite"))
	if len(backups) != 1 {
		t.Fatal("backup missing")
	}
}
