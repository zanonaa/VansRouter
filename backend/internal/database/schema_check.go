package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// schemaNeedsChange é somente leitura. Não gerar backup em todo boot quando
// versão, colunas e índices já correspondem ao schema suportado.
func schemaNeedsChange(ctx context.Context, db *sql.DB, schema schemaDefinition) (bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	meta, err := schemaColumns(ctx, tx, "_meta")
	if err != nil {
		return false, err
	}
	if len(meta) == 0 {
		return true, nil
	}
	var raw string
	err = tx.QueryRowContext(ctx, "SELECT value FROM _meta WHERE key='schemaVersion'").Scan(&raw)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	version, err := strconv.Atoi(raw)
	if err != nil {
		return false, fmt.Errorf("schemaVersion inválida: %q", raw)
	}
	if version > schema.Version {
		return false, fmt.Errorf("schemaVersion %d superior à suportada %d; banco preservado", version, schema.Version)
	}
	if version < schema.Version {
		return true, nil
	}
	for name, table := range schema.Tables {
		columns, err := schemaColumns(ctx, tx, name)
		if err != nil {
			return false, err
		}
		for column := range table.Columns {
			if !columns[column] {
				return true, nil
			}
		}
		for _, statement := range table.Indexes {
			// Snapshot exportado usa CREATE INDEX IF NOT EXISTS <nome> ...
			fields := strings.Fields(statement)
			if len(fields) < 6 {
				return false, fmt.Errorf("definição de índice inválida: %s", statement)
			}
			var count int
			if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?", fields[5]).Scan(&count); err != nil {
				return false, err
			}
			if count == 0 {
				return true, nil
			}
		}
	}
	for table, extra := range map[string][]string{"apiKeys": {"allowedProviders", "allowedCombos", "allowedKinds"}, "requestDetails": {"apiKey", "apiKeyName"}} {
		columns, err := schemaColumns(ctx, tx, table)
		if err != nil {
			return false, err
		}
		for _, column := range extra {
			if !columns[column] {
				return true, nil
			}
		}
	}
	return false, nil
}
