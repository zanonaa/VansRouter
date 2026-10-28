package handlers

import (
	"context"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

func TestDeleteProxyPoolRemovesNordRestoreRecord(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := database.Open(ctx, dir, "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = database.Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	pools := database.ProxyPools{DB: db}
	pool, err := pools.Create(ctx, map[string]any{"name": "NordVPN BR 1", "proxyUrl": "socks5://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO nordvpnTunnels(id,poolId,country) VALUES(?,?,?)`, "nord-br-1", pool["id"], "br"); err != nil {
		t.Fatal(err)
	}
	previous := nordManager
	nordManager = &NordManager{db: db, pools: pools, tunnels: map[string]*NordTunnel{"nord-br-1": nil}}
	defer func() { nordManager = previous }()
	if err = deleteProxyPool(ctx, pools, pool["id"].(string)); err != nil {
		t.Fatal(err)
	}
	if p, e := pools.Get(ctx, pool["id"].(string)); e != nil || p != nil {
		t.Fatalf("pool remains: %v %v", p, e)
	}
	records, err := nordSavedRecords(ctx)
	if err != nil || len(records) != 0 {
		t.Fatalf("restore records remain: %v %v", records, err)
	}
	if _, exists := nordManager.tunnels["nord-br-1"]; exists {
		t.Fatal("live tunnel registration remains")
	}
}
