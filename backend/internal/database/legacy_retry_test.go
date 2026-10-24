package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyImportIntoInitializedEmptyDatabase(t *testing.T) {
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
	if err = os.WriteFile(filepath.Join(dir, "db.json"), []byte(`{"settings":{"requireLogin":false}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	settings, err := ReadAuthSettings(ctx, db)
	if err != nil || settings["requireLogin"] != false {
		t.Fatalf("late import: %v %v", settings, err)
	}
	backups, _ := filepath.Glob(filepath.Join(dir, "db", "backups", "legacy-json-*"))
	if len(backups) != 1 {
		t.Fatal("backup missing")
	}
}
func TestLegacyRetryAfterDataFailure(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "db.json")
	if err := os.WriteFile(path, []byte(`{"providerNodes":[{"id":"n"},{"id":"n"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, dir, "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = Migrate(ctx, db, dir); err == nil {
		t.Fatal("expected duplicate failure")
	}
	if err = os.WriteFile(path, []byte(`{"providerNodes":[{"id":"n","name":"Fixed"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	node, err := (Nodes{DB: db}).Get(ctx, "n")
	if err != nil || node["name"] != "Fixed" {
		t.Fatalf("retry: %v %v", node, err)
	}
}
func TestLegacyDoesNotOverwritePopulatedDatabase(t *testing.T) {
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
	if _, err = UpdateSettings(ctx, db, map[string]any{"password": "keep"}); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "db.json"), []byte(`{"settings":{"password":"replace"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db, dir); err == nil {
		t.Fatal("populated DB overwritten")
	}
	settings, err := ReadAuthSettings(ctx, db)
	if err != nil || settings["password"] != "keep" {
		t.Fatalf("existing data changed: %v %v", settings, err)
	}
}
