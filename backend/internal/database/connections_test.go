package database

import (
	"context"
	"testing"
)

func TestConnectionMergeHealthAndOrdering(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT NOT NULL,authType TEXT NOT NULL,name TEXT,email TEXT,priority INTEGER,isActive INTEGER,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL);
 INSERT INTO providerConnections VALUES('a','p','oauth','A',NULL,0,1,'{"testStatus":"error","lastError":"old","modelLock_x":"locked","unknown":true}','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z');
 INSERT INTO providerConnections VALUES('b','p','apikey','B',NULL,2,0,'{}','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z');`)
	if err != nil {
		t.Fatal(err)
	}
	repo := Connections{DB: db}
	list, err := repo.List(ctx, "p", nil)
	if err != nil || len(list) != 2 || list[0]["id"] != "b" {
		t.Fatalf("order: %v %v", list, err)
	}
	active := true
	list, err = repo.List(ctx, "p", &active)
	if err != nil || len(list) != 1 || list[0]["id"] != "a" {
		t.Fatalf("filter: %v %v", list, err)
	}
	updated, err := repo.Update(ctx, "a", map[string]any{"testStatus": "active", "lastError": "explicit"})
	if err != nil {
		t.Fatal(err)
	}
	if updated["unknown"] != true || updated["lastError"] != "explicit" || updated["modelLock_x"] != nil || updated["rateLimitedUntil"] != nil || updated["backoffLevel"] != float64(0) {
		t.Fatalf("health: %v", updated)
	}
	if _, err = repo.Update(ctx, "b", map[string]any{"priority": float64(5)}); err != nil {
		t.Fatal(err)
	}
	a, err := repo.Get(ctx, "a")
	if err != nil || a["priority"] != float64(1) {
		t.Fatalf("reorder: %v %v", a, err)
	}
	missing, err := repo.Update(ctx, "missing", nil)
	if err != nil || missing != nil {
		t.Fatalf("missing: %v %v", missing, err)
	}
}
