package database

import (
	"context"
	"testing"
)

func TestProxyPoolsRepository(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE proxyPools(id TEXT PRIMARY KEY,isActive INTEGER DEFAULT 1,testStatus TEXT,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL);
 CREATE TABLE proxyPoolFitness(poolId TEXT,marker TEXT);
 CREATE TABLE nordvpnTunnels(id TEXT PRIMARY KEY,poolId TEXT); INSERT INTO nordvpnTunnels VALUES('nord-a','pool-a'),('nord-b','pool-b');`)
	if err != nil {
		t.Fatal(err)
	}
	repo := ProxyPools{DB: db}
	pool, err := repo.Create(ctx, map[string]any{"id": "pool-a", "name": "A", "proxyUrl": "http://proxy.example.test", "strictProxy": true})
	if err != nil {
		t.Fatal(err)
	}
	if pool["type"] != "http" || pool["noProxy"] != "" || pool["testStatus"] != "unknown" || pool["isActive"] != true || pool["lastError"] != nil {
		t.Fatalf("defaults: %v", pool)
	}
	if _, err = repo.Create(ctx, map[string]any{"id": "pool-b", "name": "B", "isActive": false}); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`UPDATE proxyPools SET data='{"name":"A","unknown":{"nested":true},"strictProxy":true}',updatedAt='2026-01-01T00:00:00Z' WHERE id='pool-a'; UPDATE proxyPools SET updatedAt='2026-02-01T00:00:00Z' WHERE id='pool-b';
 INSERT INTO proxyPoolFitness VALUES('pool-a','a'),('pool-b','b');`)
	if err != nil {
		t.Fatal(err)
	}
	list, err := repo.List(ctx, nil, "")
	if err != nil || len(list) != 2 || list[0]["id"] != "pool-b" {
		t.Fatalf("sort: %v %v", list, err)
	}
	active := true
	list, err = repo.List(ctx, &active, "unknown")
	if err != nil || len(list) != 1 || list[0]["id"] != "pool-a" {
		t.Fatalf("filter: %v %v", list, err)
	}
	updated, err := repo.Update(ctx, "pool-a", map[string]any{"testStatus": "active", "lastError": nil})
	if err != nil || updated["unknown"] == nil || updated["strictProxy"] != true {
		t.Fatalf("merge: %v %v", updated, err)
	}
	got, err := repo.Get(ctx, "pool-a")
	if err != nil || got["testStatus"] != "active" || got["unknown"] == nil {
		t.Fatalf("get: %v %v", got, err)
	}
	missing, err := repo.Update(ctx, "missing", nil)
	if err != nil || missing != nil {
		t.Fatalf("missing update: %v %v", missing, err)
	}
	removed, err := repo.Delete(ctx, "pool-a")
	if err != nil || removed["id"] != "pool-a" {
		t.Fatalf("delete: %v %v", removed, err)
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM nordvpnTunnels WHERE poolId='pool-a'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("NordVPN record orphan: %d %v", count, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM proxyPoolFitness WHERE poolId='pool-a'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("fitness orphan: %d %v", count, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM proxyPoolFitness WHERE poolId='pool-b'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("unrelated fitness: %d %v", count, err)
	}
	missing, err = repo.Get(ctx, "pool-a")
	if err != nil || missing != nil {
		t.Fatalf("deleted get: %v %v", missing, err)
	}
	missing, err = repo.Delete(ctx, "missing")
	if err != nil || missing != nil {
		t.Fatalf("missing delete: %v %v", missing, err)
	}
	// Uma falha na segunda exclusão deve reverter a primeira (fitness).
	_, err = db.Exec(`CREATE TRIGGER block_pool_delete BEFORE DELETE ON proxyPools BEGIN SELECT RAISE(ABORT,'synthetic failure'); END;`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Delete(ctx, "pool-b"); err == nil {
		t.Fatal("expected deletion failure")
	}
	if err = db.QueryRow(`SELECT count(*) FROM nordvpnTunnels WHERE poolId='pool-b'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("rollback NordVPN record: %d %v", count, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM proxyPoolFitness WHERE poolId='pool-b'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("rollback fitness: %d %v", count, err)
	}
}

func TestDeleteWithUnlinkAtomic(t *testing.T) {
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
	repo := ProxyPools{DB: db}
	if _, err = repo.Create(ctx, map[string]any{"id": "p"}); err != nil {
		t.Fatal(err)
	}
	raw := `{"providerSpecificData":{"proxyPoolId":"p","proxyPoolIds":["p","other"],"keep":"yes"}}`
	_, err = db.Exec(`INSERT INTO providerConnections(id,provider,authType,data,createdAt,updatedAt) VALUES('c','test','apikey',?,'now','now')`, raw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TRIGGER block_delete BEFORE DELETE ON proxyPools BEGIN SELECT RAISE(ABORT,'test'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.DeleteWithUnlink(ctx, "p"); err == nil {
		t.Fatal("expected rollback")
	}
	var got string
	if err = db.QueryRow(`SELECT data FROM providerConnections WHERE id='c'`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != raw {
		t.Fatal("failed deletion changed bindings")
	}
	if _, err = db.Exec(`DROP TRIGGER block_delete`); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.DeleteWithUnlink(ctx, "p"); err != nil {
		t.Fatal(err)
	}
	c, err := (Connections{DB: db}).Get(ctx, "c")
	if err != nil {
		t.Fatal(err)
	}
	data := c["providerSpecificData"].(map[string]any)
	ids := data["proxyPoolIds"].([]any)
	if data["proxyPoolId"] != "__none__" || len(ids) != 1 || ids[0] != "other" || data["keep"] != "yes" {
		t.Fatalf("unexpected metadata: %v", data)
	}
}
