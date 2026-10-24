package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ExportTransfer reproduz exportDb, não uma cópia de tabelas internas.
func ExportTransfer(ctx context.Context, db *sql.DB) (map[string]any, error) {
	settings, err := ReadAuthSettings(ctx, db)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"settings": settings}
	connections, err := (Connections{DB: db}).List(ctx, "", nil)
	if err != nil {
		return nil, err
	}
	out["providerConnections"] = connections
	nodes, err := (Nodes{DB: db}).List(ctx, "")
	if err != nil {
		return nil, err
	}
	out["providerNodes"] = nodes
	pools, err := (ProxyPools{DB: db}).List(ctx, nil, "")
	if err != nil {
		return nil, err
	}
	out["proxyPools"] = pools
	fitness, err := (ProxyFitness{DB: db}).List(ctx, nil)
	if err != nil {
		return nil, err
	}
	out["proxyPoolFitness"] = fitness
	keys, err := GetApiKeys(ctx, db)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		delete(key, "allowedProviders")
		delete(key, "allowedCombos")
		delete(key, "allowedKinds")
	}
	out["apiKeys"] = keys
	combos, err := (Combos{DB: db}).List(ctx)
	if err != nil {
		return nil, err
	}
	out["combos"] = combos
	kv := ModelAliases{DB: db}
	for _, scope := range []string{"modelAliases", "mitmAlias", "pricing"} {
		values, err := kv.All(ctx, scope)
		if err != nil {
			return nil, err
		}
		out[scope] = values
	}
	custom, err := kv.CustomList(ctx)
	if err != nil {
		return nil, err
	}
	out["customModels"] = custom
	return out, nil
}

// ImportTransfer altera somente as coleções lógicas gerenciadas na origem.
// Histórico, detalhes, disabledModels e _meta são preservados.
func ImportTransfer(ctx context.Context, db *sql.DB, payload map[string]any) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, table := range []string{"settings", "providerConnections", "providerNodes", "proxyPools", "proxyPoolFitness", "apiKeys", "combos"} {
		if _, err = tx.ExecContext(ctx, "DELETE FROM "+quoted(table)); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM kv WHERE scope IN ('modelAliases','customModels','mitmAlias','pricing')`); err != nil {
		return err
	}
	if settings := payload["settings"]; jsonTruthy(settings) {
		encoded, err := json.Marshal(settings)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO settings(id,data) VALUES(1,?)`, string(encoded)); err != nil {
			return err
		}
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	for _, table := range []string{"providerConnections", "providerNodes", "proxyPools", "proxyPoolFitness", "apiKeys", "combos"} {
		rows, err := legacyArray(payload, table)
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
				copy["type"] = legacyOr(row["type"], nil)
				copy["name"] = legacyOr(row["name"], nil)
				err = writeNode(ctx, tx, copy)
			case "proxyPools":
				copy["testStatus"] = legacyOr(row["testStatus"], "unknown")
				err = writePool(ctx, tx, copy)
			case "proxyPoolFitness":
				_, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO proxyPoolFitness(poolId,scope,until,reason,createdAt,updatedAt) VALUES(?,?,?,?,?,?)`, row["poolId"], row["scope"], row["until"], legacyOr(row["reason"], ""), copy["createdAt"], copy["updatedAt"])
			case "apiKeys":
				active := 1
				if row["isActive"] == false {
					active = 0
				}
				_, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO apiKeys(id,key,name,machineId,isActive,createdAt) VALUES(?,?,?,?,?,?)`, row["id"], row["key"], legacyOr(row["name"], nil), legacyOr(row["machineId"], nil), active, copy["createdAt"])
			case "combos":
				_, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO combos(id,name,kind,models,context_length,createdAt,updatedAt) VALUES(?,?,?,?,?,?,?)`, row["id"], row["name"], legacyOr(row["kind"], nil), legacyJSON(legacyOr(row["models"], []any{})), row["context_length"], copy["createdAt"], copy["updatedAt"])
			}
			if err != nil {
				return fmt.Errorf("importar %s: %w", table, err)
			}
		}
	}
	put := func(scope, key string, value any) error {
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO kv(scope,key,value) VALUES(?,?,?)`, scope, key, string(encoded))
		return err
	}
	for _, scope := range []string{"modelAliases", "mitmAlias", "pricing"} {
		if value := payload[scope]; value != nil {
			items, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("invalid %s payload", scope)
			}
			for key, v := range items {
				if err = put(scope, key, v); err != nil {
					return err
				}
			}
		}
	}
	custom, err := legacyArray(payload, "customModels")
	if err != nil {
		return err
	}
	for _, model := range custom {
		key := fmt.Sprint(model["providerAlias"]) + "|" + fmt.Sprint(model["id"]) + "|" + fmt.Sprint(legacyOr(model["type"], "llm"))
		if err = put("customModels", key, model); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO _meta(key,value) VALUES('settings_revision','1') ON CONFLICT(key) DO UPDATE SET value=CAST(CAST(value AS INTEGER)+1 AS TEXT)`); err != nil {
		return err
	}
	return tx.Commit()
}
