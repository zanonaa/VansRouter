package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
)

type DatabaseExport struct {
	DB             *sql.DB
	VerifyPassword func(string) bool
	Settings       func(context.Context) (map[string]any, error)
}

func (d DatabaseExport) tables(ctx context.Context) ([]string, error) {
	rows, err := d.DB.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []string{}
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, err
		}
		list = append(list, name)
	}
	return list, rows.Err()
}
func (d DatabaseExport) authorized(r *http.Request) bool {
	if r.Header.Get("X-9r-cli-token") != "" {
		return true
	}
	return d.VerifyPassword != nil && d.VerifyPassword(r.Header.Get("X-9r-password"))
}
func (d DatabaseExport) Get(w http.ResponseWriter, r *http.Request) {
	if !d.authorized(r) {
		nodeReply(w, 401, map[string]string{"error": "Invalid password"})
		return
	}
	names, err := d.tables(r.Context())
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to export database"})
		return
	}
	payload := map[string]any{}
	for _, name := range names {
		rows, err := d.DB.QueryContext(r.Context(), "SELECT * FROM "+quoted(name))
		if err != nil {
			nodeReply(w, 500, map[string]string{"error": "Failed to export database"})
			return
		}
		cols, err := rows.Columns()
		if err != nil {
			rows.Close()
			nodeReply(w, 500, map[string]string{"error": "Failed to export database"})
			return
		}
		records := []map[string]any{}
		for rows.Next() {
			values := make([]any, len(cols))
			holders := make([]any, len(cols))
			for i := range values {
				holders[i] = &values[i]
			}
			if err = rows.Scan(holders...); err != nil {
				rows.Close()
				nodeReply(w, 500, map[string]string{"error": "Failed to export database"})
				return
			}
			record := map[string]any{}
			for i, column := range cols {
				record[column] = normalizeSQLite(values[i])
			}
			records = append(records, record)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			nodeReply(w, 500, map[string]string{"error": "Failed to export database"})
			return
		}
		payload[name] = records
	}
	nodeReply(w, 200, payload)
}
func normalizeSQLite(v any) any {
	switch value := v.(type) {
	case []byte:
		if json.Valid(value) {
			var decoded any
			if json.Unmarshal(value, &decoded) == nil {
				return decoded
			}
		}
		return string(value)
	default:
		return v
	}
}
func (d DatabaseExport) Post(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 400, map[string]string{"error": "Failed to import database"})
		return
	}
	password, _ := body["password"].(string)
	delete(body, "password")
	if r.Header.Get("X-9r-cli-token") == "" && (d.VerifyPassword == nil || !d.VerifyPassword(password)) {
		nodeReply(w, 401, map[string]string{"error": "Invalid password"})
		return
	}
	names, err := d.tables(r.Context())
	if err != nil {
		nodeReply(w, 400, map[string]string{"error": err.Error()})
		return
	}
	tx, err := d.DB.BeginTx(r.Context(), nil)
	if err != nil {
		nodeReply(w, 400, map[string]string{"error": err.Error()})
		return
	}
	defer tx.Rollback()
	for _, name := range names {
		if _, err = tx.ExecContext(r.Context(), "DELETE FROM "+quoted(name)); err != nil {
			nodeReply(w, 400, map[string]string{"error": err.Error()})
			return
		}
	}
	for name, raw := range body {
		records, ok := raw.([]any)
		if !ok {
			nodeReply(w, 400, map[string]string{"error": "invalid table payload: " + name})
			return
		}
		allowed := map[string]bool{}
		for _, existing := range names {
			allowed[existing] = true
		}
		if !allowed[name] {
			nodeReply(w, 400, map[string]string{"error": "unknown table: " + name})
			return
		}
		for _, recordRaw := range records {
			record, ok := recordRaw.(map[string]any)
			if !ok {
				nodeReply(w, 400, map[string]string{"error": "invalid row in " + name})
				return
			}
			columns := make([]string, 0, len(record))
			for column := range record {
				columns = append(columns, column)
			}
			if len(columns) == 0 {
				continue
			}
			// _meta preserva chaves fora do payload; inserção simples sem upsert por PK composta desconhecida.
			placeholders := strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",")
			values := make([]any, len(columns))
			for i, column := range columns {
				values[i] = record[column]
			}
			statement := "INSERT INTO " + quoted(name) + " (" + strings.Join(mapQuoted(columns), ",") + ") VALUES (" + placeholders + ")"
			if _, err = tx.ExecContext(r.Context(), statement, values...); err != nil {
				nodeReply(w, 400, map[string]string{"error": err.Error()})
				return
			}
		}
	}
	if err = tx.Commit(); err != nil {
		nodeReply(w, 400, map[string]string{"error": err.Error()})
		return
	}
	nodeReply(w, 200, map[string]bool{"success": true})
}
func mapQuoted(names []string) []string {
	result := make([]string, len(names))
	for i, name := range names {
		result[i] = quoted(name)
	}
	return result
}
