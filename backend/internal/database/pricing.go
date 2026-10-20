package database

import (
	"context"
	"database/sql"
	"encoding/json"
)

type Pricing struct{ DB *sql.DB }

func (p Pricing) User(ctx context.Context) (map[string]any, error) {
	return (ModelAliases{DB: p.DB}).All(ctx, "pricing")
}
func (p Pricing) Update(ctx context.Context, updates map[string]any) (map[string]any, error) {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for provider, models := range updates {
		var raw string
		err = tx.QueryRowContext(ctx, `SELECT value FROM kv WHERE scope='pricing' AND key=?`, provider).Scan(&raw)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		current := map[string]any{}
		_ = json.Unmarshal([]byte(raw), &current)
		if current == nil {
			current = map[string]any{}
		}
		for model, value := range models.(map[string]any) {
			current[model] = value
		}
		encoded, err := json.Marshal(current)
		if err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO kv(scope,key,value) VALUES('pricing',?,?) ON CONFLICT(scope,key) DO UPDATE SET value=excluded.value`, provider, string(encoded)); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return p.User(ctx)
}
func (p Pricing) Reset(ctx context.Context, provider, model string) error {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if provider == "" {
		_, err = tx.ExecContext(ctx, `DELETE FROM kv WHERE scope='pricing'`)
	} else if model == "" {
		_, err = tx.ExecContext(ctx, `DELETE FROM kv WHERE scope='pricing' AND key=?`, provider)
	} else {
		var raw string
		err = tx.QueryRowContext(ctx, `SELECT value FROM kv WHERE scope='pricing' AND key=?`, provider).Scan(&raw)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		current := map[string]any{}
		_ = json.Unmarshal([]byte(raw), &current)
		delete(current, model)
		if len(current) == 0 {
			_, err = tx.ExecContext(ctx, `DELETE FROM kv WHERE scope='pricing' AND key=?`, provider)
		} else {
			encoded, _ := json.Marshal(current)
			_, err = tx.ExecContext(ctx, `UPDATE kv SET value=? WHERE scope='pricing' AND key=?`, string(encoded), provider)
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
