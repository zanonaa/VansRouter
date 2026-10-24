package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type ModelAliases struct{ DB *sql.DB }

func (m ModelAliases) All(ctx context.Context, scope string) (map[string]any, error) {
	rows, err := m.DB.QueryContext(ctx, "SELECT key,value FROM kv WHERE scope = ?", scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]any{}
	for rows.Next() {
		var key, raw string
		if err = rows.Scan(&key, &raw); err != nil {
			return nil, err
		}
		var v any
		_ = json.Unmarshal([]byte(raw), &v)
		result[key] = v
	}
	return result, rows.Err()
}
func (m ModelAliases) Set(ctx context.Context, alias, model any) error {
	raw, err := json.Marshal(model)
	if err != nil {
		return err
	}
	_, err = m.DB.ExecContext(ctx, `INSERT INTO kv(scope,key,value) VALUES('modelAliases',?,?) ON CONFLICT(scope,key) DO UPDATE SET value=excluded.value`, alias, string(raw))
	return err
}
func (m ModelAliases) Delete(ctx context.Context, scope, key string) error {
	_, err := m.DB.ExecContext(ctx, "DELETE FROM kv WHERE scope = ? AND key = ?", scope, key)
	return err
}
func customKey(data map[string]any) string {
	kind, exists := data["type"]
	if !exists {
		kind = "llm"
	}
	return fmt.Sprint(data["providerAlias"]) + "|" + fmt.Sprint(data["id"]) + "|" + fmt.Sprint(kind)
}
func (m ModelAliases) CustomList(ctx context.Context) ([]any, error) {
	rows, err := m.DB.QueryContext(ctx, `SELECT value FROM kv WHERE scope='customModels'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []any{}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var v any
		_ = json.Unmarshal([]byte(raw), &v)
		result = append(result, v)
	}
	return result, rows.Err()
}
func (m ModelAliases) AddCustom(ctx context.Context, data map[string]any) (bool, error) {
	count, err := m.customWrite(ctx, []map[string]any{data}, false)
	return count > 0, err
}
func (m ModelAliases) CustomBulk(ctx context.Context, items []map[string]any) (int, error) {
	return m.customWrite(ctx, items, true)
}

// AddCustom e CustomBulk compartilham transação, mas não política de upsert.
func (m ModelAliases) customWrite(ctx context.Context, items []map[string]any, bulk bool) (int, error) {
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	added := 0
	for _, data := range items {
		if bulk && (!jsonTruthy(data["providerAlias"]) || !jsonTruthy(data["id"])) {
			continue
		}
		key := customKey(data)
		var raw string
		err = tx.QueryRowContext(ctx, `SELECT value FROM kv WHERE scope='customModels' AND key=?`, key).Scan(&raw)
		if err != nil && err != sql.ErrNoRows {
			return 0, err
		}
		exists := err == nil
		if exists && bulk {
			continue
		}
		var value map[string]any
		if exists {
			_ = json.Unmarshal([]byte(raw), &value)
			if value == nil {
				value = map[string]any{}
			}
			for _, field := range []string{"name", "caps", "transport"} {
				if jsonTruthy(data[field]) {
					value[field] = data[field]
				}
			}
		} else {
			kind, ok := data["type"]
			if !ok {
				kind = "llm"
			}
			name := data["name"]
			if !jsonTruthy(name) {
				name = data["id"]
			}
			value = map[string]any{"providerAlias": data["providerAlias"], "id": data["id"], "type": kind, "name": name}
			if !bulk {
				for _, field := range []string{"caps", "transport"} {
					if jsonTruthy(data[field]) {
						value[field] = data[field]
					}
				}
			}
			added++
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return 0, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO kv(scope,key,value) VALUES('customModels',?,?) ON CONFLICT(scope,key) DO UPDATE SET value=excluded.value`, key, string(encoded)); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return added, nil
}
