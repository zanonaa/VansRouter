package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Combos struct{ DB *sql.DB }

const comboColumns = "id,name,kind,models,context_length,createdAt,updatedAt"

func scanCombo(row scanner) (map[string]any, error) {
	var id, name, raw, created, updated string
	var kind sql.NullString
	var length sql.NullFloat64
	if err := row.Scan(&id, &name, &kind, &raw, &length, &created, &updated); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	var models any = []any{}
	if json.Unmarshal([]byte(raw), &models) != nil {
		models = []any{}
	}
	var contextLength any
	if length.Valid {
		contextLength = length.Float64
	}
	return map[string]any{"id": id, "name": name, "kind": nullString(kind), "models": models, "context_length": contextLength, "createdAt": created, "updatedAt": updated}, nil
}
func (c Combos) List(ctx context.Context) ([]map[string]any, error) {
	rows, err := c.DB.QueryContext(ctx, "SELECT "+comboColumns+" FROM combos ORDER BY createdAt ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		item, err := scanCombo(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}
func (c Combos) Get(ctx context.Context, id string) (map[string]any, error) {
	return scanCombo(c.DB.QueryRowContext(ctx, "SELECT "+comboColumns+" FROM combos WHERE id = ?", id))
}
func (c Combos) ByName(ctx context.Context, name any) (map[string]any, error) {
	return scanCombo(c.DB.QueryRowContext(ctx, "SELECT "+comboColumns+" FROM combos WHERE name = ?", name))
}
func (c Combos) Create(ctx context.Context, data map[string]any) (map[string]any, error) {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	models := data["models"]
	if !jsonTruthy(models) {
		models = []any{}
	}
	kind := data["kind"]
	if !jsonTruthy(kind) {
		kind = nil
	}
	combo := map[string]any{"id": uuid.NewString(), "name": data["name"], "kind": kind, "models": models, "context_length": data["context_length"], "createdAt": now, "updatedAt": now}
	raw, err := json.Marshal(models)
	if err != nil {
		return nil, err
	}
	_, err = c.DB.ExecContext(ctx, `INSERT INTO combos(id,name,kind,models,context_length,createdAt,updatedAt) VALUES(?,?,?,?,?,?,?)`, combo["id"], combo["name"], kind, string(raw), combo["context_length"], now, now)
	if err != nil {
		return nil, err
	}
	return combo, nil
}
func (c Combos) Update(ctx context.Context, id string, patch map[string]any) (map[string]any, error) {
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	value, err := scanCombo(tx.QueryRowContext(ctx, "SELECT "+comboColumns+" FROM combos WHERE id = ?", id))
	if err != nil || value == nil {
		return nil, err
	}
	for key, v := range patch {
		value[key] = v
	}
	value["updatedAt"] = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	models := value["models"]
	if !jsonTruthy(models) {
		models = []any{}
	}
	raw, err := json.Marshal(models)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE combos SET name=?,kind=?,models=?,context_length=?,updatedAt=? WHERE id=?`, value["name"], value["kind"], string(raw), value["context_length"], value["updatedAt"], id)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return value, nil
}
func (c Combos) Delete(ctx context.Context, id string) (bool, error) {
	result, err := c.DB.ExecContext(ctx, "DELETE FROM combos WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
