package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// ReadAuthSettings lê somente dados existentes. Não cria tabelas nem altera versões.
// Um banco novo permite o login inicial; tabelas da aplicação serão criadas pelas migrations.
func ReadAuthSettings(ctx context.Context, db *sql.DB) (map[string]any, error) {
	var exists int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='settings'").Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return map[string]any{}, nil
	}
	var data string
	err := db.QueryRowContext(ctx, "SELECT data FROM settings WHERE id=1").Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(data), &settings); err != nil || settings == nil {
		return nil, fmt.Errorf("configurações de autenticação inválidas")
	}
	return settings, nil
}
