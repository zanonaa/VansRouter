package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Nodes implementa nodesRepo.js; não cria schema nem executa migrations.
type Nodes struct{ DB *sql.DB }

const nodeColumns = "id, type, name, data, createdAt, updatedAt"

type scanner interface{ Scan(...any) error }

func scanNode(row scanner) (map[string]any, error) {
	var id, raw, created, updated string
	var kind, name sql.NullString
	if err := row.Scan(&id, &kind, &name, &raw, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	extra := map[string]any{}
	// parseJson de origem usa {} como fallback para JSON inválido.
	if json.Unmarshal([]byte(raw), &extra) != nil || extra == nil {
		extra = map[string]any{}
	}
	extra["id"] = id
	extra["type"] = nullString(kind)
	extra["name"] = nullString(name)
	extra["createdAt"] = created
	extra["updatedAt"] = updated
	return extra, nil
}
func (n Nodes) List(ctx context.Context, kind string) ([]map[string]any, error) {
	query := "SELECT " + nodeColumns + " FROM providerNodes"
	args := []any{}
	if kind != "" {
		query += " WHERE type = ?"
		args = append(args, kind)
	}
	rows, err := n.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		node, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, node)
	}
	return result, rows.Err()
}
func (n Nodes) Get(ctx context.Context, id string) (map[string]any, error) {
	return scanNode(n.DB.QueryRowContext(ctx, "SELECT "+nodeColumns+" FROM providerNodes WHERE id = ?", id))
}
func writeNode(ctx context.Context, tx *sql.Tx, node map[string]any) error {
	extra := map[string]any{}
	for k, v := range node {
		switch k {
		case "id", "type", "name", "createdAt", "updatedAt":
		default:
			extra[k] = v
		}
	}
	raw, err := json.Marshal(extra)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO providerNodes(id,type,name,data,createdAt,updatedAt) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET type=excluded.type,name=excluded.name,data=excluded.data,updatedAt=excluded.updatedAt`, node["id"], node["type"], node["name"], string(raw), node["createdAt"], node["updatedAt"])
	return err
}
func (n Nodes) Create(ctx context.Context, data map[string]any) (map[string]any, error) {
	id, ok := data["id"].(string)
	if !ok || id == "" {
		id = uuid.NewString()
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	node := map[string]any{"id": id, "createdAt": now, "updatedAt": now}
	for _, key := range []string{"type", "name", "prefix", "apiType", "baseUrl"} {
		if v, ok := data[key]; ok {
			node[key] = v
		}
	}
	tx, err := n.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = writeNode(ctx, tx, node); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return node, nil
}
func (n Nodes) Update(ctx context.Context, id string, patch map[string]any) (map[string]any, error) {
	tx, err := n.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	node, err := scanNode(tx.QueryRowContext(ctx, "SELECT "+nodeColumns+" FROM providerNodes WHERE id = ?", id))
	if err != nil || node == nil {
		return nil, err
	}
	for k, v := range patch {
		node[k] = v
	}
	node["updatedAt"] = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	if err = writeNode(ctx, tx, node); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return node, nil
}
func (n Nodes) Delete(ctx context.Context, id string) (map[string]any, error) {
	tx, err := n.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	node, err := scanNode(tx.QueryRowContext(ctx, "SELECT "+nodeColumns+" FROM providerNodes WHERE id = ?", id))
	if err != nil || node == nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM providerNodes WHERE id = ?", id); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return node, nil
}
