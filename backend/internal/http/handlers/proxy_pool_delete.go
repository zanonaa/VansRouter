package handlers

import (
	"context"
	"fmt"
)

// Serialize with restore/rotation: commit deletion before closing the live tunnel.
func deleteProxyPool(ctx context.Context, store PoolStore, poolID string) error {
	return deleteProxyPoolWithUnlink(ctx, store, poolID, false)
}

func deleteProxyPoolWithUnlink(ctx context.Context, store PoolStore, poolID string, unlink bool) error {
	nordManager.ops.Lock()
	defer nordManager.ops.Unlock()
	var tunnelIDs []string
	if nordManager.db != nil {
		rows, err := nordManager.db.QueryContext(ctx, `SELECT id FROM nordvpnTunnels WHERE poolId=?`, poolID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			tunnelIDs = append(tunnelIDs, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	remove := store.Delete
	if unlink {
		atomicStore, ok := store.(interface {
			DeleteWithUnlink(context.Context, string) (map[string]any, error)
		})
		if !ok {
			return fmt.Errorf("atomic unlink is unavailable")
		}
		remove = atomicStore.DeleteWithUnlink
	}
	if _, err := remove(ctx, poolID); err != nil {
		return err
	}
	nordManager.mu.Lock()
	var tunnels []*NordTunnel
	for _, id := range tunnelIDs {
		if t := nordManager.tunnels[id]; t != nil {
			tunnels = append(tunnels, t)
		}
		delete(nordManager.tunnels, id)
	}
	nordManager.mu.Unlock()
	for _, t := range tunnels {
		t.Close()
	}
	return nil
}
