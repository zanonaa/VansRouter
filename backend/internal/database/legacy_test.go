package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyImportAndRetry(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	raw := []byte(`{"settings":{"password":"existing"},"providerConnections":[{"id":"a","provider":"openai","authType":"apikey","apiKey":"secret","name":"A"}],"providerNodes":[{"id":"node","name":"N","baseUrl":"https://example.test"}],"apiKeys":[{"id":"k","key":"key"}],"combos":[{"id":"c","name":"Combo","models":[]}],"modelAliases":{"x":"y"}}`)
	if err := os.WriteFile(filepath.Join(dir, "db.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "usage.json"), []byte(`{"history":[{"tokens":{"input_tokens":5},"provider":"openai"}],"totalRequestsLifetime":3}`), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, dir, "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	connection, err := (Connections{DB: db}).Get(ctx, "a")
	if err != nil || connection["apiKey"] != "secret" {
		t.Fatalf("connection: %v %v", connection, err)
	}
	settings, err := ReadAuthSettings(ctx, db)
	if err != nil || settings["password"] != "existing" {
		t.Fatalf("settings: %v %v", settings, err)
	}
	if _, err = os.Stat(filepath.Join(dir, "db", ".migrated-from-json")); err != nil {
		t.Fatal(err)
	}
	preserved, err := os.ReadFile(filepath.Join(dir, "db.json"))
	if err != nil || string(preserved) != string(raw) {
		t.Fatal("original changed")
	}
	// Simula marcador não persistido após commit: não reimportar history.
	if err = os.Remove(filepath.Join(dir, "db", ".migrated-from-json")); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM usageHistory`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate history: %d %v", count, err)
	}
}
func TestLegacyDuplicateRollsBack(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "db.json"), []byte(`{"settings":{"password":"p"},"providerNodes":[{"id":"n"},{"id":"n"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := Open(ctx, dir, "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = Migrate(ctx, db, dir); err == nil {
		t.Fatal("duplicate IDs not rejected")
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM settings`).Scan(&count); err != nil || count != 0 {
		t.Fatal("import partially committed")
	}
	if _, err = os.Stat(filepath.Join(dir, "db", ".migrated-from-json")); !os.IsNotExist(err) {
		t.Fatal("marker written on failure")
	}
}
