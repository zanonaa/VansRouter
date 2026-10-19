package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// Connections porta as operações necessárias à propagação de providerNodes.
// Criação/deduplicação/bulk/reordenação explícita ainda não foram portadas.
type Connections struct{ DB *sql.DB }

const connectionColumns = "id, provider, authType, name, email, priority, isActive, data, createdAt, updatedAt"

func scanConnection(row scanner) (map[string]any, error) {
	var id, provider, authType, raw, created, updated string
	var name, email sql.NullString
	var priority sql.NullFloat64
	var active sql.NullInt64
	if err := row.Scan(&id, &provider, &authType, &name, &email, &priority, &active, &raw, &created, &updated); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	result := map[string]any{}
	if json.Unmarshal([]byte(raw), &result) != nil || result == nil {
		result = map[string]any{}
	}
	result["id"] = id
	result["provider"] = provider
	result["authType"] = authType
	result["name"] = nullString(name)
	result["email"] = nullString(email)
	result["priority"] = nil
	if priority.Valid {
		result["priority"] = priority.Float64
	}
	result["isActive"] = active.Valid && active.Int64 == 1
	result["createdAt"] = created
	result["updatedAt"] = updated
	return result, nil
}
func (c Connections) Get(ctx context.Context, id string) (map[string]any, error) {
	return scanConnection(c.DB.QueryRowContext(ctx, "SELECT "+connectionColumns+" FROM providerConnections WHERE id = ?", id))
}
func (c Connections) List(ctx context.Context, provider string, active *bool) ([]map[string]any, error) {
	query := "SELECT " + connectionColumns + " FROM providerConnections"
	where := []string{}
	args := []any{}
	if provider != "" {
		where = append(where, "provider = ?")
		args = append(args, provider)
	}
	if active != nil {
		where = append(where, "isActive = ?")
		v := 0
		if *active {
			v = 1
		}
		args = append(args, v)
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	rows, err := c.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		item, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(result, func(i, j int) bool { return connectionPriority(result[i], 999) < connectionPriority(result[j], 999) })
	return result, nil
}
func connectionPriority(c map[string]any, fallback float64) float64 {
	v, _ := c["priority"].(float64)
	if v == 0 {
		return fallback
	}
	return v
}
func writeConnection(ctx context.Context, tx *sql.Tx, c map[string]any) error {
	extra := map[string]any{}
	for k, v := range c {
		switch k {
		case "id", "provider", "authType", "name", "email", "priority", "isActive", "createdAt", "updatedAt":
		default:
			extra[k] = v
		}
	}
	raw, err := json.Marshal(extra)
	if err != nil {
		return err
	}
	active := 1
	if c["isActive"] == false {
		active = 0
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO providerConnections(id,provider,authType,name,email,priority,isActive,data,createdAt,updatedAt) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET provider=excluded.provider,authType=excluded.authType,name=excluded.name,email=excluded.email,priority=excluded.priority,isActive=excluded.isActive,data=excluded.data,updatedAt=excluded.updatedAt`, c["id"], c["provider"], c["authType"], c["name"], c["email"], c["priority"], active, string(raw), c["createdAt"], c["updatedAt"])
	return err
}
func reorderConnections(ctx context.Context, tx *sql.Tx, provider any) error {
	rows, err := tx.QueryContext(ctx, "SELECT "+connectionColumns+" FROM providerConnections WHERE provider = ?", provider)
	if err != nil {
		return err
	}
	list := []map[string]any{}
	for rows.Next() {
		item, err := scanConnection(rows)
		if err != nil {
			rows.Close()
			return err
		}
		list = append(list, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	sort.SliceStable(list, func(i, j int) bool {
		a, b := connectionPriority(list[i], 0), connectionPriority(list[j], 0)
		if a != b {
			return a < b
		}
		at, _ := time.Parse(time.RFC3339, stringValue(list[i]["updatedAt"]))
		bt, _ := time.Parse(time.RFC3339, stringValue(list[j]["updatedAt"]))
		return at.After(bt)
	})
	for i, item := range list {
		want := float64(i + 1)
		if connectionPriority(item, 0) != want {
			if _, err = tx.ExecContext(ctx, "UPDATE providerConnections SET priority = ? WHERE id = ?", want, item["id"]); err != nil {
				return err
			}
		}
	}
	return nil
}
func stringValue(v any) string { s, _ := v.(string); return s }
func (c Connections) Update(ctx context.Context, id string, patch map[string]any) (map[string]any, error) {
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	existing, err := scanConnection(tx.QueryRowContext(ctx, "SELECT "+connectionColumns+" FROM providerConnections WHERE id = ?", id))
	if err != nil || existing == nil {
		return nil, err
	}
	provider := existing["provider"]
	normalized := map[string]any{}
	for k, v := range patch {
		normalized[k] = v
	}
	if patch["testStatus"] == "active" {
		for _, k := range []string{"lastError", "lastErrorAt"} {
			if _, ok := patch[k]; !ok {
				normalized[k] = nil
			}
		}
		for _, k := range []string{"errorCode", "rateLimitedUntil"} {
			normalized[k] = nil
		}
		normalized["backoffLevel"] = float64(0)
		for k := range existing {
			if strings.HasPrefix(k, "modelLock_") {
				normalized[k] = nil
			}
		}
	}
	for k, v := range normalized {
		existing[k] = v
	}
	existing["updatedAt"] = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	if err = writeConnection(ctx, tx, existing); err != nil {
		return nil, err
	}
	if _, ok := patch["priority"]; ok {
		if err = reorderConnections(ctx, tx, provider); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return existing, nil
}
func (c Connections) DeleteByProvider(ctx context.Context, provider string) (int64, error) {
	// O total de linhas removidas corresponde ao contador retornado pela origem.
	result, err := c.DB.ExecContext(ctx, "DELETE FROM providerConnections WHERE provider = ?", provider)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
