package database

import (
	"context"
	"testing"
)

func TestNodesRepository(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE providerNodes(id TEXT PRIMARY KEY,type TEXT,name TEXT,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	repo := Nodes{DB: db}
	node, err := repo.Create(ctx, map[string]any{"id": "openai-compatible-test", "type": "openai-compatible", "name": "Teste", "prefix": "test", "baseUrl": "https://example.test/v1", "ignored": "not-stored"})
	if err != nil {
		t.Fatal(err)
	}
	if node["ignored"] != nil || node["id"] != "openai-compatible-test" {
		t.Fatalf("create: %v", node)
	}
	got, err := repo.Get(ctx, node["id"].(string))
	if err != nil || got["baseUrl"] != "https://example.test/v1" {
		t.Fatalf("get: %v %v", got, err)
	}
	if _, err = db.Exec(`UPDATE providerNodes SET data='{"baseUrl":"https://example.test/v1","unknown":{"preserved":true}}'`); err != nil {
		t.Fatal(err)
	}
	updated, err := repo.Update(ctx, node["id"].(string), map[string]any{"name": "Novo", "prefix": nil})
	if err != nil {
		t.Fatal(err)
	}
	if updated["name"] != "Novo" || updated["unknown"] == nil || updated["prefix"] != nil {
		t.Fatalf("merge: %v", updated)
	}
	got, err = repo.Get(ctx, node["id"].(string))
	if err != nil || got["unknown"] == nil {
		t.Fatalf("extras: %v %v", got, err)
	}
	list, err := repo.List(ctx, "openai-compatible")
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %v", list, err)
	}
	list, err = repo.List(ctx, "other")
	if err != nil || len(list) != 0 {
		t.Fatalf("filter: %v %v", list, err)
	}
	missing, err := repo.Update(ctx, "missing", map[string]any{"name": "x"})
	if err != nil || missing != nil {
		t.Fatalf("missing update: %v %v", missing, err)
	}
	removed, err := repo.Delete(ctx, node["id"].(string))
	if err != nil || removed["name"] != "Novo" {
		t.Fatalf("delete: %v %v", removed, err)
	}
	missing, err = repo.Get(ctx, node["id"].(string))
	if err != nil || missing != nil {
		t.Fatalf("missing get: %v %v", missing, err)
	}
	missing, err = repo.Delete(ctx, "missing")
	if err != nil || missing != nil {
		t.Fatalf("missing delete: %v %v", missing, err)
	}
}

func TestNodesDoNotCreateSchema(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := (Nodes{DB: db}).List(ctx, ""); err == nil {
		t.Fatal("schema ausente deve falhar, não simular sucesso")
	}
}
