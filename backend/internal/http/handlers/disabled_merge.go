package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
)

// mergeDisabled faz a mesclagem na mesma transação, sem perder chamadas
// consecutivas. DELETE do último ID remove a chave como na origem.
func (d DisabledModels) mergeDisabled(ctx context.Context, provider string, ids []string, remove bool) error {
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw string
	err = tx.QueryRowContext(ctx, `SELECT value FROM kv WHERE scope='disabledModels' AND key=?`, provider).Scan(&raw)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	current := []string{}
	_ = json.Unmarshal([]byte(raw), &current)
	values := []string{}
	seen := map[string]bool{}
	excluded := map[string]bool{}
	if remove {
		for _, id := range ids {
			excluded[id] = true
		}
	}
	for _, id := range current {
		if !excluded[id] && (!seen[id] || remove) {
			values = append(values, id)
			seen[id] = true
		}
	}
	if !remove {
		for _, id := range ids {
			if !seen[id] {
				values = append(values, id)
				seen[id] = true
			}
		}
	}
	if remove && len(values) == 0 {
		_, err = tx.ExecContext(ctx, `DELETE FROM kv WHERE scope='disabledModels' AND key=?`, provider)
	} else {
		encoded, _ := json.Marshal(values)
		_, err = tx.ExecContext(ctx, `INSERT INTO kv(scope,key,value) VALUES('disabledModels',?,?) ON CONFLICT(scope,key) DO UPDATE SET value=excluded.value`, provider, string(encoded))
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
