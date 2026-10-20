package database

import (
	"context"
	"math"
	"testing"
)

func TestProxyFitness(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE proxyPoolFitness(poolId TEXT NOT NULL,scope TEXT NOT NULL,until INTEGER NOT NULL,reason TEXT,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL,PRIMARY KEY(poolId,scope))`)
	if err != nil {
		t.Fatal(err)
	}
	repo := ProxyFitness{DB: db}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		item, err := repo.Upsert(ctx, "p", "s", value, "")
		if err != nil || item != nil {
			t.Fatalf("invalid time: %v %v", item, err)
		}
	}
	if item, err := repo.Upsert(ctx, "", "s", 1, ""); err != nil || item != nil {
		t.Fatal("empty pool accepted")
	}
	first, err := repo.Upsert(ctx, "b", "openai::model", 1790000000123, "rate-limit")
	if err != nil {
		t.Fatal(err)
	}
	if first.Until != 1790000000123 || first.Reason != "rate-limit" {
		t.Fatalf("units: %v", first)
	}
	if _, err = db.Exec(`UPDATE proxyPoolFitness SET createdAt='2020-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	updated, err := repo.Upsert(ctx, "b", "openai::model", 1790000000999, "")
	if err != nil || updated.CreatedAt != "2020-01-01T00:00:00Z" || updated.Until != 1790000000999 || updated.Reason != "" {
		t.Fatalf("upsert: %v %v", updated, err)
	}
	for _, entry := range []struct{ pool, scope string }{{"a", "other::*"}, {"a", "openai::*"}, {"b", "openai-other::model"}} {
		if _, err = repo.Upsert(ctx, entry.pool, entry.scope, 1, ""); err != nil {
			t.Fatal(err)
		}
	}
	list, err := repo.List(ctx, nil)
	if err != nil || len(list) != 4 || list[0].PoolID != "a" || list[0].Scope != "openai::*" {
		t.Fatalf("order: %v %v", list, err)
	}
	pool := "b"
	list, err = repo.List(ctx, &pool)
	if err != nil || len(list) != 2 {
		t.Fatalf("filter: %v %v", list, err)
	}
	deleted, err := repo.Clear(ctx, "openai")
	if err != nil || deleted != 2 {
		t.Fatalf("clear provider: %d %v", deleted, err)
	}
	list, err = repo.List(ctx, nil)
	if err != nil || len(list) != 2 {
		t.Fatalf("unrelated scopes: %v %v", list, err)
	}
	deleted, err = repo.Delete(ctx, "a", "other::*")
	if err != nil || deleted != 1 {
		t.Fatalf("delete: %d %v", deleted, err)
	}
	deleted, err = repo.DeleteByPool(ctx, "b")
	if err != nil || deleted != 1 {
		t.Fatalf("delete pool: %d %v", deleted, err)
	}
	if _, err = repo.Upsert(ctx, "p", "s", 0, ""); err != nil {
		t.Fatal(err)
	}
	deleted, err = repo.Clear(ctx, "")
	if err != nil || deleted != 1 {
		t.Fatalf("clear all: %d %v", deleted, err)
	}
}
