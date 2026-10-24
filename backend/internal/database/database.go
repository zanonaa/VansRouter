// Package database prepara a conexão SQLite sem alterar o schema da aplicação.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, dataDir, synchronous string) (*sql.DB, error) {
	switch synchronous {
	case "OFF", "NORMAL", "FULL", "EXTRA":
	default:
		return nil, fmt.Errorf("SQLITE_SYNCHRONOUS inválido")
	}
	dir := filepath.Join(dataDir, "db")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("criar diretório do banco: %w", err)
	}
	file, err := filepath.Abs(filepath.Join(dir, "data.sqlite"))
	if err != nil {
		return nil, err
	}
	// Os pragmas do DSN são aplicados a toda nova conexão, inclusive após reconexão.
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(file)}
	q := u.Query()
	for _, pragma := range []string{"busy_timeout(5000)", "foreign_keys(ON)", "synchronous(" + synchronous + ")", "temp_store(MEMORY)", "mmap_size(30000000)", "cache_size(-64000)"} {
		q.Add("_pragma", pragma)
	}
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, fmt.Errorf("abrir SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	fail := func(err error) (*sql.DB, error) { _ = db.Close(); return nil, err }
	if err := db.PingContext(ctx); err != nil {
		return fail(fmt.Errorf("conectar ao SQLite: %w", err))
	}
	rows, err := db.QueryContext(ctx, "PRAGMA quick_check")
	if err != nil {
		return fail(fmt.Errorf("verificar integridade: %w", err))
	}
	valid := false
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			_ = rows.Close()
			return fail(err)
		}
		if result != "ok" {
			_ = rows.Close()
			return fail(fmt.Errorf("integridade SQLite inválida; arquivos preservados"))
		}
		valid = true
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return fail(err)
	}
	if !valid {
		return fail(fmt.Errorf("verificação de integridade sem resultado"))
	}
	var journal string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&journal); err != nil {
		return fail(fmt.Errorf("configurar WAL: %w", err))
	}
	if journal != "wal" {
		return fail(fmt.Errorf("SQLite não ativou WAL"))
	}
	return db, nil
}
