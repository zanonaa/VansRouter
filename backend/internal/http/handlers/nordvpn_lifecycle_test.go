package handlers

import (
	"context"
	"database/sql"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

func TestNordMigrationKeepsPoolIdentity(t *testing.T) {
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
	_, err = db.Exec(`INSERT INTO nordvpnTunnels(id,poolId,country) VALUES(?,?,?)`, "nord-br-1", pool["id"], "br")
	if err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	var id string
	if err = db.QueryRow(`SELECT poolId FROM nordvpnTunnels WHERE id='nord-br-1'`).Scan(&id); err != nil || id != pool["id"] {
		t.Fatal("pool identity lost")
	}
	_, err = db.Exec(`INSERT INTO nordvpnTunnels(id,poolId,country) VALUES(?,?,?)`, "duplicate", id, "br")
	if err == nil {
		t.Fatal("duplicate pool association accepted")
	}
	_ = sql.ErrNoRows
}

func TestNordInactivePoolSetNeverFallsBackDirect(t *testing.T) {
	lookup := func(context.Context, string) (map[string]any, error) {
		return map[string]any{"name": "NordVPN BR 1", "isActive": false, "strictProxy": true, "proxyUrl": "socks5://127.0.0.1:1"}, nil
	}
	for _, data := range []map[string]any{
		{"proxyPoolId": "nord"},
		{"proxyPoolIds": []any{"nord"}, "proxyRotationStrategy": "smart"},
	} {
		data["connectionProxyEnabled"] = true
		data["connectionProxyUrl"] = "http://legacy.invalid:80"
		_, _, _, err := connectionHTTP(context.Background(), data, lookup, nil)
		if err != errProxyRequired {
			t.Fatalf("expected fail-closed, got %v", err)
		}
	}
}

func TestProxyProbeActuallyUsesSOCKS(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	hit := make(chan struct{}, 1)
	go func() {
		client, e := listener.Accept()
		if e != nil {
			return
		}
		defer client.Close()
		client.SetDeadline(time.Now().Add(5 * time.Second))
		greeting := make([]byte, 3)
		if _, e = io.ReadFull(client, greeting); e != nil {
			return
		}
		client.Write([]byte{5, 0})
		header := make([]byte, 4)
		if _, e = io.ReadFull(client, header); e != nil {
			return
		}
		switch header[3] {
		case 1:
			io.ReadFull(client, make([]byte, 4))
		case 3:
			var length [1]byte
			io.ReadFull(client, length[:])
			io.ReadFull(client, make([]byte, int(length[0])))
		case 4:
			io.ReadFull(client, make([]byte, 16))
		}
		io.ReadFull(client, make([]byte, 2))
		remote, e := net.Dial("tcp", target.Host)
		if e != nil {
			return
		}
		defer remote.Close()
		client.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
		hit <- struct{}{}
		go io.Copy(remote, client)
		io.Copy(client, remote)
	}()
	result := (ProxyProbe{Targets: []string{upstream.URL}}).probe(context.Background(), map[string]any{"proxyUrl": "socks5://" + listener.Addr().String(), "type": "socks5"})
	if !result.OK {
		t.Fatalf("SOCKS probe failed: %+v", result)
	}
	select {
	case <-hit:
	default:
		t.Fatal("probe bypassed SOCKS")
	}
	result = (ProxyProbe{Targets: []string{upstream.URL}}).probe(context.Background(), map[string]any{"proxyUrl": "socks5://127.0.0.1:1"})
	if result.OK || !strings.Contains(result.Error, "failed") {
		t.Fatal("dead SOCKS should not fall back direct")
	}
}
