package database

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed schema.json
var schemaJSON []byte

type tableDefinition struct {
	Columns    map[string]string `json:"columns"`
	PrimaryKey string            `json:"primaryKey"`
	Indexes    []string          `json:"indexes"`
}
type schemaDefinition struct {
	Version int                        `json:"version"`
	Tables  map[string]tableDefinition `json:"tables"`
}

func quoted(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }
func schemaColumns(ctx context.Context, tx *sql.Tx, table string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, "PRAGMA table_info("+quoted(table)+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]bool{}
	for rows.Next() {
		var cid, notNull, pk int
		var name, kind string
		var fallback any
		if err = rows.Scan(&cid, &name, &kind, &notNull, &fallback, &pk); err != nil {
			return nil, err
		}
		result[name] = true
	}
	return result, rows.Err()
}

// Migrate inicializa schema atual e aplica as transformações versionadas da
// origem. Nunca elimina tabelas/colunas desconhecidas. Backup SQLite consistente
// antes de qualquer modificação em banco existente (inclui WAL via VACUUM INTO).
func Migrate(ctx context.Context, db *sql.DB, dataDir string) error {
	var schema schemaDefinition
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return err
	}
	// Não começar uma instalação vazia por cima de uma instalação JSON antiga.
	var tables int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tables); err != nil {
		return err
	}
	legacy, err := readLegacy(dataDir)
	if err != nil {
		return err
	}
	if tables > 0 && len(legacy) > 0 {
		var imported int
		if err = db.QueryRowContext(ctx, `SELECT count(*) FROM _meta WHERE key='goLegacyImported'`).Scan(&imported); err != nil {
			return fmt.Errorf("banco existente com JSON legado exige revisão antes de importar: %w", err)
		}
		if imported == 0 {
			for _, table := range []string{"settings", "providerConnections", "providerNodes", "proxyPools", "apiKeys", "combos", "usageHistory", "usageDaily", "kv", "requestDetails"} {
				var count int
				if err = db.QueryRowContext(ctx, "SELECT count(*) FROM "+quoted(table)).Scan(&count); err != nil {
					return err
				}
				if count > 0 {
					return fmt.Errorf("importação legada recusada: banco existente contém dados em %s", table)
				}
			}
		}
	}

	needsChange, err := schemaNeedsChange(ctx, db, schema)
	if err != nil {
		return err
	}
	if !needsChange {
		return importLegacy(ctx, db, dataDir, legacy)
	}
	if len(legacy) > 0 {
		if err = os.MkdirAll(filepath.Join(dataDir, "db", "backups"), 0700); err != nil {
			return err
		}
	}
	if tables > 0 {
		dir := filepath.Join(dataDir, "db", "backups")
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		// Arquivo exclusivo evita sobrescrever backups de execuções concorrentes.
		file, err := os.CreateTemp(dir, "go-pre-schema-*.sqlite")
		if err != nil {
			return err
		}
		path := file.Name()
		if err = file.Close(); err != nil {
			return err
		}
		if err = os.Remove(path); err != nil {
			return err
		}
		if _, err = db.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
			return fmt.Errorf("backup pré-schema: %w", err)
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS _meta(key TEXT PRIMARY KEY,value TEXT NOT NULL)`); err != nil {
		return err
	}
	version := 0
	var raw string
	err = tx.QueryRowContext(ctx, "SELECT value FROM _meta WHERE key='schemaVersion'").Scan(&raw)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if raw != "" {
		if _, err = fmt.Sscan(raw, &version); err != nil {
			return fmt.Errorf("schemaVersion inválida: %q", raw)
		}
	}
	if version > schema.Version {
		return fmt.Errorf("schemaVersion %d superior à suportada %d; banco preservado", version, schema.Version)
	}
	// Migration 002 somente para bancos que ainda não a aplicaram. Não converter
	// [] de instalações atuais: hoje significa nenhuma permissão, não todas.
	if version < 2 {
		columns, err := schemaColumns(ctx, tx, "apiKeys")
		if err != nil {
			return err
		}
		for _, column := range []string{"allowedProviders", "allowedCombos", "allowedKinds"} {
			if columns[column] {
				if _, err = tx.ExecContext(ctx, "UPDATE apiKeys SET "+quoted(column)+" = NULL WHERE "+quoted(column)+" = '[]'"); err != nil {
					return err
				}
			}
		}
	}
	names := []string{}
	for name := range schema.Tables {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		def := schema.Tables[name]
		columns := []string{}
		keys := []string{}
		for key := range def.Columns {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			columns = append(columns, quoted(key)+" "+def.Columns[key])
		}
		if def.PrimaryKey != "" {
			columns = append(columns, def.PrimaryKey)
		}
		if _, err = tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS "+quoted(name)+" ("+strings.Join(columns, ",")+")"); err != nil {
			return err
		}
		existing, err := schemaColumns(ctx, tx, name)
		if err != nil {
			return err
		}
		for _, key := range keys {
			if !existing[key] {
				definition := strings.ReplaceAll(strings.ReplaceAll(def.Columns[key], "PRIMARY KEY", ""), "UNIQUE", "")
				if _, err = tx.ExecContext(ctx, "ALTER TABLE "+quoted(name)+" ADD COLUMN "+quoted(key)+" "+definition); err != nil {
					return fmt.Errorf("adicionar %s.%s: %w", name, key, err)
				}
			}
		}
		for _, index := range def.Indexes {
			if _, err = tx.ExecContext(ctx, index); err != nil {
				return err
			}
		}
	}
	// Migration 003: ACL também não está declarada em TABLES.
	acl, err := schemaColumns(ctx, tx, "apiKeys")
	if err != nil {
		return err
	}
	for _, key := range []string{"allowedProviders", "allowedCombos", "allowedKinds"} {
		if !acl[key] {
			if _, err = tx.ExecContext(ctx, "ALTER TABLE apiKeys ADD COLUMN "+quoted(key)+" TEXT"); err != nil {
				return err
			}
		}
	}
	// Migration 004 adiciona campos não incluídos no TABLES da origem.
	cols, err := schemaColumns(ctx, tx, "requestDetails")
	if err != nil {
		return err
	}
	for _, key := range []string{"apiKey", "apiKeyName"} {
		if !cols[key] {
			if _, err = tx.ExecContext(ctx, "ALTER TABLE requestDetails ADD COLUMN "+quoted(key)+" TEXT"); err != nil {
				return err
			}
		}
	}
	for key, value := range map[string]string{"schemaVersion": fmt.Sprint(schema.Version), "backupSchemaVersion": fmt.Sprint(schema.Version), "goSchemaCheckedAt": time.Now().UTC().Format(time.RFC3339Nano)} {
		if _, err = tx.ExecContext(ctx, `INSERT INTO _meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return importLegacy(ctx, db, dataDir, legacy)
}
