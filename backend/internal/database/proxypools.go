package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"

	"github.com/google/uuid"
)

// ProxyPools porta proxyPoolsRepo.js sem criar schema ou registrar rotas.
type ProxyPools struct{ DB *sql.DB }

const poolColumns = "id, isActive, testStatus, data, createdAt, updatedAt"

func scanPool(row scanner) (map[string]any, error) {
	var id, raw, created, updated string
	var active sql.NullInt64
	var status sql.NullString
	if err := row.Scan(&id, &active, &status, &raw, &created, &updated); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	pool := map[string]any{}
	if json.Unmarshal([]byte(raw), &pool) != nil || pool == nil {
		pool = map[string]any{}
	}
	pool["id"] = id
	pool["isActive"] = active.Valid && active.Int64 == 1
	pool["testStatus"] = nullString(status)
	pool["createdAt"] = created
	pool["updatedAt"] = updated
	return pool, nil
}
func (p ProxyPools) Get(ctx context.Context, id string) (map[string]any, error) {
	return scanPool(p.DB.QueryRowContext(ctx, "SELECT "+poolColumns+" FROM proxyPools WHERE id = ?", id))
}
func (p ProxyPools) List(ctx context.Context, active *bool, status string) ([]map[string]any, error) {
	query := "SELECT " + poolColumns + " FROM proxyPools WHERE 1=1"
	args := []any{}
	if active != nil {
		query += " AND isActive = ?"
		value := 0
		if *active {
			value = 1
		}
		args = append(args, value)
	}
	if status != "" {
		query += " AND testStatus = ?"
		args = append(args, status)
	}
	rows, err := p.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		pool, err := scanPool(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, pool)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(list, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339, stringValue(list[i]["updatedAt"]))
		b, _ := time.Parse(time.RFC3339, stringValue(list[j]["updatedAt"]))
		return a.After(b)
	})
	return list, nil
}
func writePool(ctx context.Context, tx *sql.Tx, pool map[string]any) error {
	extra := map[string]any{}
	for key, value := range pool {
		switch key {
		case "id", "isActive", "testStatus", "createdAt", "updatedAt":
		default:
			extra[key] = value
		}
	}
	raw, err := json.Marshal(extra)
	if err != nil {
		return err
	}
	active := 1
	if pool["isActive"] == false {
		active = 0
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO proxyPools(id,isActive,testStatus,data,createdAt,updatedAt) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET isActive=excluded.isActive,testStatus=excluded.testStatus,data=excluded.data,updatedAt=excluded.updatedAt`, pool["id"], active, pool["testStatus"], string(raw), pool["createdAt"], pool["updatedAt"])
	return err
}

// jsonTruthy reproduz a coerção booleana JS para entradas JSON.
func jsonTruthy(v any) bool {
	switch value := v.(type) {
	case nil:
		return false
	case bool:
		return value
	case string:
		return value != ""
	case float64:
		return value != 0
	default:
		return true
	}
}
func (p ProxyPools) Create(ctx context.Context, data map[string]any) (map[string]any, error) {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	id := data["id"]
	if !jsonTruthy(id) {
		id = uuid.NewString()
	}
	pool := map[string]any{"id": id, "strictProxy": data["strictProxy"] == true, "createdAt": now, "updatedAt": now}
	for _, key := range []string{"name", "proxyUrl"} {
		if value, ok := data[key]; ok {
			pool[key] = value
		}
	}
	for key, fallback := range map[string]any{"noProxy": "", "type": "http", "testStatus": "unknown", "lastTestedAt": nil, "lastError": nil} {
		value := data[key]
		if !jsonTruthy(value) {
			value = fallback
		}
		pool[key] = value
	}
	pool["isActive"] = true
	if value, ok := data["isActive"]; ok {
		pool["isActive"] = value
	}
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = writePool(ctx, tx, pool); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return pool, nil
}
func (p ProxyPools) Update(ctx context.Context, id string, patch map[string]any) (map[string]any, error) {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	pool, err := scanPool(tx.QueryRowContext(ctx, "SELECT "+poolColumns+" FROM proxyPools WHERE id = ?", id))
	if err != nil || pool == nil {
		return nil, err
	}
	for key, value := range patch {
		pool[key] = value
	}
	pool["updatedAt"] = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	if err = writePool(ctx, tx, pool); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return pool, nil
}
func (p ProxyPools) Delete(ctx context.Context, id string) (map[string]any, error) {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	pool, err := scanPool(tx.QueryRowContext(ctx, "SELECT "+poolColumns+" FROM proxyPools WHERE id = ?", id))
	if err != nil || pool == nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM nordvpnTunnels WHERE poolId = ?", id); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM proxyPoolFitness WHERE poolId = ?", id); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM proxyPools WHERE id = ?", id); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return pool, nil
}
