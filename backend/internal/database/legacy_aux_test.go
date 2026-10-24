package database

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLegacyAuxiliaryCollections(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	files := map[string]string{
		"db.json":              `{"customModels":[{"providerAlias":"oa","id":"custom","type":"embedding","extra":true}],"pricing":{"openai":{"custom":1.25}},"mitmAlias":{"tool":{"from":"to"}},"proxyPools":[{"id":"pool","name":"Pool","proxyUrl":"http://example.test","strictProxy":true,"unknown":"keep"}]}`,
		"usage.json":           `{"history":[{"tokens":{"prompt_tokens":3,"completion_tokens":4},"cost":0.5,"endpoint":"/v1/chat/completions"}],"dailySummary":{"2026-01-01":{"requests":2}},"totalRequestsLifetime":99}`,
		"disabledModels.json":  `{"disabled":{"openai":["blocked"],"other":[]}}`,
		"request-details.json": `{"records":[{"id":"detail","timestamp":"2026-01-01T00:00:00Z","provider":"openai","model":"custom","connectionId":"conn","status":"ok","extra":{"nested":true}}]}`,
	}
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
	}
	db, err := Open(ctx, dir, "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ scope, key, want string }{
		{"customModels", "oa|custom|embedding", `{"providerAlias":"oa","id":"custom","type":"embedding","extra":true}`},
		{"pricing", "openai", `{"custom":1.25}`}, {"mitmAlias", "tool", `{"from":"to"}`}, {"disabledModels", "openai", `["blocked"]`}, {"disabledModels", "other", `[]`},
	} {
		var raw string
		if err = db.QueryRow(`SELECT value FROM kv WHERE scope=? AND key=?`, tt.scope, tt.key).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var got, want any
		_ = json.Unmarshal([]byte(raw), &got)
		_ = json.Unmarshal([]byte(tt.want), &want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s/%s: %v want %v", tt.scope, tt.key, got, want)
		}
	}
	var daily string
	if err = db.QueryRow(`SELECT data FROM usageDaily WHERE dateKey='2026-01-01'`).Scan(&daily); err != nil || daily != `{"requests":2}` {
		t.Fatalf("daily: %s %v", daily, err)
	}
	var total string
	if err = db.QueryRow(`SELECT value FROM _meta WHERE key='totalRequestsLifetime'`).Scan(&total); err != nil || total != "99" {
		t.Fatalf("total: %s %v", total, err)
	}
	var prompt, completion int
	var cost float64
	if err = db.QueryRow(`SELECT promptTokens,completionTokens,cost FROM usageHistory`).Scan(&prompt, &completion, &cost); err != nil || prompt != 3 || completion != 4 || cost != 0.5 {
		t.Fatalf("usage: %d %d %f %v", prompt, completion, cost, err)
	}
	var detail string
	if err = db.QueryRow(`SELECT data FROM requestDetails WHERE id='detail'`).Scan(&detail); err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	_ = json.Unmarshal([]byte(detail), &record)
	if record["extra"].(map[string]any)["nested"] != true {
		t.Fatal("detail extras lost")
	}
	pool, err := (ProxyPools{DB: db}).Get(ctx, "pool")
	if err != nil || pool["strictProxy"] != true || pool["unknown"] != "keep" {
		t.Fatalf("pool: %v %v", pool, err)
	}
	backups, err := filepath.Glob(filepath.Join(dir, "db", "backups", "legacy-json-*"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups: %v %v", backups, err)
	}
	for name, want := range files {
		raw, err := os.ReadFile(filepath.Join(backups[0], name))
		if err != nil || string(raw) != want {
			t.Fatalf("backup %s differs: %v", name, err)
		}
	}
}

func TestLegacyChangedBeforeBackup(t *testing.T) {
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
	path := filepath.Join(dir, "db.json")
	if err = os.WriteFile(path, []byte(`{"settings":{"name":"old"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	files, err := readLegacy(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte(`{"settings":{"name":"new"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(dir, "db", "backups"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = importLegacy(ctx, db, dir, files); err == nil {
		t.Fatal("changed source silently imported")
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM settings`).Scan(&count); err != nil || count != 0 {
		t.Fatal("changed source mutated DB")
	}
}
