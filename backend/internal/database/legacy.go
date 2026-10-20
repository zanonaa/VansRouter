package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"
)

type legacyFiles map[string]map[string]any

func readLegacy(dir string) (legacyFiles, error) {
	result := legacyFiles{}
	if _, err := os.Stat(filepath.Join(dir, "db", ".migrated-from-json")); err == nil {
		return result, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	for _, name := range []string{"db.json", "usage.json", "disabledModels.json", "request-details.json"} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var data map[string]any
		if err = json.Unmarshal(raw, &data); err != nil || data == nil {
			return nil, fmt.Errorf("JSON legado inválido: %s; arquivo preservado", name)
		}
		result[name] = data
	}
	return result, nil
}
func legacyArray(data map[string]any, key string) ([]map[string]any, error) {
	raw, exists := data[key]
	if !exists || raw == nil {
		return nil, nil
	}
	values, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("lista legada inválida: %s", key)
	}
	result := []map[string]any{}
	for _, v := range values {
		item, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("registro legado inválido: %s", key)
		}
		result = append(result, item)
	}
	return result, nil
}
func legacyJSON(v any) string { raw, _ := json.Marshal(v); return string(raw) }
func legacyOr(v, fallback any) any {
	if jsonTruthy(v) {
		return v
	}
	return fallback
}
func importLegacy(ctx context.Context, db *sql.DB, dir string, files legacyFiles) error {
	if len(files) == 0 {
		return nil
	}
	var done int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM _meta WHERE key='goLegacyImported'`).Scan(&done); err != nil {
		return err
	}
	if done > 0 {
		return os.WriteFile(filepath.Join(dir, "db", ".migrated-from-json"), []byte("committed\n"), 0600)
	}
	backupDir, err := os.MkdirTemp(filepath.Join(dir, "db", "backups"), "legacy-json-*")
	if err != nil {
		return err
	}
	for name, expected := range files {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		var current map[string]any
		if json.Unmarshal(raw, &current) != nil || !reflect.DeepEqual(current, expected) {
			return fmt.Errorf("arquivo legado alterado durante migração: %s; reinicie com escritor legado parado", name)
		}
		if err = os.WriteFile(filepath.Join(backupDir, name), raw, 0600); err != nil {
			return err
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	main := files["db.json"]
	if settings, ok := main["settings"]; ok && settings != nil {
		if _, err = tx.ExecContext(ctx, `INSERT INTO settings(id,data) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data`, legacyJSON(settings)); err != nil {
			return err
		}
	}
	for _, table := range []string{"providerConnections", "providerNodes", "proxyPools", "apiKeys", "combos"} {
		rows, err := legacyArray(main, table)
		if err != nil {
			return err
		}
		for _, row := range rows {
			copy := map[string]any{}
			for k, v := range row {
				copy[k] = v
			}
			copy["createdAt"] = legacyOr(row["createdAt"], now)
			copy["updatedAt"] = legacyOr(row["updatedAt"], now)
			switch table {
			case "providerConnections":
				copy["authType"] = legacyOr(row["authType"], "oauth")
				copy["name"] = legacyOr(row["name"], nil)
				copy["email"] = legacyOr(row["email"], nil)
				copy["priority"] = legacyOr(row["priority"], nil)
				err = writeConnection(ctx, tx, copy)
			case "providerNodes":
				err = writeNode(ctx, tx, copy)
			case "proxyPools":
				copy["testStatus"] = legacyOr(row["testStatus"], "unknown")
				err = writePool(ctx, tx, copy)
			case "apiKeys":
				active := 1
				if row["isActive"] == false {
					active = 0
				}
				_, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO apiKeys(id,key,name,machineId,isActive,createdAt) VALUES(?,?,?,?,?,?)`, row["id"], row["key"], legacyOr(row["name"], nil), legacyOr(row["machineId"], nil), active, copy["createdAt"])
			case "combos":
				models := legacyOr(row["models"], []any{})
				_, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO combos(id,name,kind,models,context_length,createdAt,updatedAt) VALUES(?,?,?,?,?,?,?)`, row["id"], row["name"], legacyOr(row["kind"], nil), legacyJSON(models), row["context_length"], copy["createdAt"], copy["updatedAt"])
			}
			if err != nil {
				return fmt.Errorf("importar %s: %w", table, err)
			}
		}
		var count int
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM "+quoted(table)).Scan(&count); err != nil {
			return err
		}
		if count != len(rows) {
			return fmt.Errorf("contagem legada divergente em %s: esperado %d, obtido %d", table, len(rows), count)
		}
	}
	putKV := func(scope, key string, value any) error {
		_, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO kv(scope,key,value) VALUES(?,?,?)`, scope, key, legacyJSON(value))
		return err
	}
	for _, scope := range []string{"modelAliases", "mitmAlias", "pricing"} {
		if values, ok := main[scope].(map[string]any); ok {
			for key, value := range values {
				if err = putKV(scope, key, value); err != nil {
					return err
				}
			}
		}
	}
	custom, err := legacyArray(main, "customModels")
	if err != nil {
		return err
	}
	for _, model := range custom {
		key := fmt.Sprint(model["providerAlias"]) + "|" + fmt.Sprint(model["id"]) + "|" + fmt.Sprint(legacyOr(model["type"], "llm"))
		if err = putKV("customModels", key, model); err != nil {
			return err
		}
	}
	usage := files["usage.json"]
	history, err := legacyArray(usage, "history")
	if err != nil {
		return err
	}
	for _, entry := range history {
		tokens, _ := entry["tokens"].(map[string]any)
		if tokens == nil {
			tokens = map[string]any{}
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO usageHistory(timestamp,provider,model,connectionId,apiKey,endpoint,promptTokens,completionTokens,cost,status,tokens,meta) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, legacyOr(entry["timestamp"], now), legacyOr(entry["provider"], nil), legacyOr(entry["model"], nil), legacyOr(entry["connectionId"], nil), legacyOr(entry["apiKey"], nil), legacyOr(entry["endpoint"], nil), legacyOr(tokens["prompt_tokens"], legacyOr(tokens["input_tokens"], 0)), legacyOr(tokens["completion_tokens"], legacyOr(tokens["output_tokens"], 0)), legacyOr(entry["cost"], 0), legacyOr(entry["status"], "ok"), legacyJSON(tokens), "{}")
		if err != nil {
			return err
		}
	}
	if daily, ok := usage["dailySummary"].(map[string]any); ok {
		for key, value := range daily {
			if _, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO usageDaily(dateKey,data) VALUES(?,?)`, key, legacyJSON(value)); err != nil {
				return err
			}
		}
	}
	if total, ok := usage["totalRequestsLifetime"].(float64); ok {
		if _, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO _meta(key,value) VALUES('totalRequestsLifetime',?)`, fmt.Sprint(total)); err != nil {
			return err
		}
	}
	if disabled, ok := files["disabledModels.json"]["disabled"].(map[string]any); ok {
		for key, value := range disabled {
			if err = putKV("disabledModels", key, legacyOr(value, []any{})); err != nil {
				return err
			}
		}
	}
	records, err := legacyArray(files["request-details.json"], "records")
	if err != nil {
		return err
	}
	for _, record := range records {
		if _, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO requestDetails(id,timestamp,provider,model,connectionId,status,data) VALUES(?,?,?,?,?,?,?)`, record["id"], legacyOr(record["timestamp"], now), legacyOr(record["provider"], nil), legacyOr(record["model"], nil), legacyOr(record["connectionId"], nil), legacyOr(record["status"], nil), legacyJSON(record)); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO _meta(key,value) VALUES('goLegacyImported',?)`, now); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "db", ".migrated-from-json"), []byte(now), 0600)
}
