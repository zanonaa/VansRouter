package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type nordTunnelRecord struct{ ID, PoolID, Country string }

// Restores only explicit managed records. Legacy BR/CO pools are adopted by
// their exact generated name and loopback SOCKS address, never public proxies.
func StartNordTunnelPersistence(ctx context.Context, db *sql.DB) error {
	nordManager.ops.Lock()
	defer nordManager.ops.Unlock()
	nordManager.db = db
	pools, err := nordManager.pools.List(ctx, nil, "")
	if err != nil {
		return err
	}
	for _, pool := range pools {
		name := ttsString(pool["name"])
		url := ttsString(pool["proxyUrl"])
		if !strings.HasPrefix(name, "NordVPN ") || !strings.HasPrefix(url, "socks5://127.0.0.1:") {
			continue
		}
		var country string
		var replica int
		if _, err := fmt.Sscanf(name, "NordVPN %s %d", &country, &replica); err != nil || len(country) != 2 || replica < 1 {
			continue
		}
		id := fmt.Sprintf("nord-%s-%d", strings.ToLower(country), replica)
		if _, err = db.ExecContext(ctx, `INSERT OR IGNORE INTO nordvpnTunnels(id,poolId,country) VALUES(?,?,?)`, id, pool["id"], strings.ToLower(country)); err != nil {
			return err
		}
	}
	records, err := nordSavedRecords(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		if _, err = nordManager.pools.Update(ctx, record.PoolID, map[string]any{"isActive": false, "strictProxy": true, "testStatus": "unknown", "lastError": "NordVPN tunnel reconnecting"}); err != nil {
			return err
		}
	}
	go func() {
		for {
			nordManager.ops.Lock()
			records, err := nordSavedRecords(ctx)
			if err == nil {
				for _, record := range records {
					nordManager.mu.RLock()
					running := nordManager.tunnels[record.ID] != nil
					nordManager.mu.RUnlock()
					if running {
						continue
					}
					restoreCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
					_, restoreErr := nordConnectRecord(restoreCtx, record, "")
					cancel()
					if restoreErr != nil {
						slog.Warn("NordVPN restore failed", "tunnel", record.ID)
						_, _ = nordManager.pools.Update(ctx, record.PoolID, map[string]any{"isActive": false, "strictProxy": true, "testStatus": "error", "lastError": "NordVPN tunnel unavailable; reconnect will retry"})
					}
				}
			}
			nordManager.ops.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Minute):
			}
		}
	}()
	return nil
}
func nordSavedRecords(ctx context.Context) ([]nordTunnelRecord, error) {
	rows, err := nordManager.db.QueryContext(ctx, `SELECT id,poolId,country FROM nordvpnTunnels ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []nordTunnelRecord
	for rows.Next() {
		var r nordTunnelRecord
		if err = rows.Scan(&r.ID, &r.PoolID, &r.Country); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// Creates/validates a replacement before atomically publishing its address in
// the existing pool. Old connections remain usable until their tunnel closes.
func nordConnectRecord(ctx context.Context, record nordTunnelRecord, excludeHost string) (map[string]any, error) {
	nordManager.mu.RLock()
	client := nordManager.client
	old := nordManager.tunnels[record.ID]
	nordManager.mu.RUnlock()
	if client == nil {
		return nil, fmt.Errorf("nordvpn: authentication required")
	}
	creds, err := client.Credentials(ctx)
	if err != nil {
		return nil, err
	}
	servers, err := client.ServersByCountry(ctx, record.Country, 20)
	if err != nil {
		return nil, err
	}
	for _, server := range servers {
		if server.Hostname == excludeHost {
			continue
		}
		used := false
		nordManager.mu.RLock()
		for id, t := range nordManager.tunnels {
			if id != record.ID && t.hostname == server.Hostname {
				used = true
			}
		}
		nordManager.mu.RUnlock()
		if used {
			continue
		}
		tunnel, err := StartNordTunnel(ctx, NordTunnelSpec{ID: record.ID, Country: record.Country, Hostname: server.Hostname, Endpoint: server.endpoint(), PrivateKey: creds.NordlynxPrivateKey, PublicKey: server.wireguardKey()})
		if err != nil {
			return nil, err
		}
		if _, err = tunnel.CheckIP(ctx); err != nil {
			tunnel.Close()
			return nil, fmt.Errorf("nordvpn: tunnel IP verification failed")
		}
		if _, err = nordManager.pools.Update(ctx, record.PoolID, map[string]any{"proxyUrl": tunnel.Address(), "strictProxy": true, "isActive": true, "testStatus": "active", "lastError": nil}); err != nil {
			tunnel.Close()
			return nil, err
		}
		nordManager.mu.Lock()
		nordManager.tunnels[record.ID] = tunnel
		nordManager.mu.Unlock()
		if old != nil {
			old.Close()
		}
		status := tunnel.Status()
		status["poolId"] = record.PoolID
		return status, nil
	}
	return nil, fmt.Errorf("nordvpn: no distinct server available in %s", record.Country)
}
