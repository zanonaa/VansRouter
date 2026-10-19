package database

import (
	"context"
	"testing"
)

func TestConnectionBulkAndDelete(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT NOT NULL,authType TEXT NOT NULL,name TEXT,email TEXT,priority INTEGER,isActive INTEGER,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	repo := Connections{DB: db}
	old, err := repo.Create(ctx, map[string]any{"provider": "p", "authType": "apikey", "name": "A", "apiKey": "old", "refreshToken": "preserve", "providerSpecificData": map[string]any{"custom": true}})
	if err != nil {
		t.Fatal(err)
	}
	for _, items := range [][]map[string]any{nil, make([]map[string]any, 501)} {
		if _, err = repo.CreateBulk(ctx, items); err == nil {
			t.Fatal("invalid batch accepted")
		}
	}
	result, err := repo.CreateBulk(ctx, []map[string]any{
		{"provider": "p", "name": "A", "apiKey": "replacement"},
		{"provider": "p", "name": "B", "apiKey": "second"},
		{"provider": "p", "name": "missing-key"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 || result[0]["updated"] != true || result[0]["id"] != old["id"] || result[1]["updated"] != false || result[2]["ok"] != false {
		t.Fatalf("bulk: %v", result)
	}
	got, err := repo.Get(ctx, old["id"].(string))
	if err != nil || got["apiKey"] != "replacement" || got["refreshToken"] != "preserve" || got["providerSpecificData"] == nil {
		t.Fatalf("preserve: %v %v", got, err)
	}
	list, err := repo.List(ctx, "p", nil)
	if err != nil || len(list) != 2 || list[0]["priority"] != float64(1) || list[1]["priority"] != float64(2) {
		t.Fatalf("reorder: %v %v", list, err)
	}
	// Falha no segundo item reverte também o primeiro item do lote.
	_, err = repo.CreateBulk(ctx, []map[string]any{{"provider": "p", "name": "C", "apiKey": "third"}, {"provider": "p", "name": "bad", "apiKey": "bad", "providerSpecificData": map[string]any{"unsupported": make(chan int)}}})
	if err == nil {
		t.Fatal("expected JSON error")
	}
	list, err = repo.List(ctx, "p", nil)
	if err != nil || len(list) != 2 {
		t.Fatalf("rollback: %v %v", list, err)
	}
	ok, err := repo.Delete(ctx, list[0]["id"].(string))
	if err != nil || !ok {
		t.Fatalf("delete: %v %v", ok, err)
	}
	list, err = repo.List(ctx, "p", nil)
	if err != nil || len(list) != 1 || list[0]["priority"] != float64(1) {
		t.Fatalf("delete reorder: %v %v", list, err)
	}
	ok, err = repo.Delete(ctx, "missing")
	if err != nil || ok {
		t.Fatalf("missing delete: %v %v", ok, err)
	}
	if err = repo.Reorder(ctx, "p"); err != nil {
		t.Fatal(err)
	}
}
