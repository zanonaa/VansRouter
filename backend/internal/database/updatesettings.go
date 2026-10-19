package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// UpdateSettings reproduz updateSettings: leitura-mesclagem-escrita atômica
// com incremento de revision. Cria as tabelas se ausentes. Chaves com valor
// nil removem a chave do JSON (equivalente ao delete do fluxo oidcClientSecret).
func UpdateSettings(ctx context.Context, db *sql.DB, updates map[string]any) (map[string]any, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS _meta (key TEXT PRIMARY KEY,value TEXT NOT NULL)`); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS settings (id INTEGER PRIMARY KEY CHECK (id = 1),data TEXT NOT NULL)`); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO _meta(key,value) VALUES ('settings_revision','1') ON CONFLICT(key) DO UPDATE SET value=CAST(CAST(value AS INTEGER)+1 AS TEXT)`); err != nil {
		return nil, err
	}
	settings := map[string]any{}
	var raw string
	err = tx.QueryRowContext(ctx, `SELECT data FROM settings WHERE id=1`).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		if err = json.Unmarshal([]byte(raw), &settings); err != nil || settings == nil {
			return nil, errors.New("configurações inválidas")
		}
	}
	for key, value := range updates {
		if value == nil {
			delete(settings, key)
			continue
		}
		settings[key] = value
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO settings(id,data) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data`, string(data)); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return settings, nil
}
