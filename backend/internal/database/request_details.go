package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// RecentLogs mantém o fallback [] da origem quando a leitura falha.
func (u Usage) RecentLogs(ctx context.Context, limit int, names func() map[string]string) ([]string, error) {
	empty := []string{}
	rows, err := u.DB.QueryContext(ctx, `SELECT timestamp,provider,model,connectionId,promptTokens,completionTokens,status,tokens FROM usageHistory ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return empty, nil
	}
	type record struct {
		ts                                    string
		provider, model, conn, status, tokens sql.NullString
		prompt, completion                    sql.NullInt64
	}
	records := []record{}
	for rows.Next() {
		var r record
		if err = rows.Scan(&r.ts, &r.provider, &r.model, &r.conn, &r.prompt, &r.completion, &r.status, &r.tokens); err != nil {
			rows.Close()
			return empty, nil
		}
		records = append(records, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return empty, nil
	}
	// Fechar rows antes de consultar connections: pool de uma conexão SQLite.
	if len(records) == 0 {
		return empty, nil
	}
	accounts := map[string]string{}
	if names != nil {
		accounts = names()
	}
	logs := []string{}
	for _, r := range records {
		ts := "NaN-NaN-NaN NaN:NaN:NaN"
		if parsed, err := time.Parse(time.RFC3339Nano, r.ts); err == nil {
			ts = parsed.In(time.Local).Format("02-01-2006 15:04:05")
		}
		provider := strings.ToUpper(r.provider.String)
		if provider == "" {
			provider = "-"
		}
		model := r.model.String
		if model == "" {
			model = "-"
		}
		account := accounts[r.conn.String]
		if account == "" {
			account = r.conn.String
			if len(account) > 8 {
				account = account[:8]
			}
			if account == "" {
				account = "-"
			}
		}
		tokens := map[string]any{}
		_ = json.Unmarshal([]byte(r.tokens.String), &tokens)
		var sent, received any = "-", "-"
		if r.prompt.Valid {
			sent = r.prompt.Int64
		} else if v := tokens["prompt_tokens"]; v != nil {
			sent = v
		}
		if r.completion.Valid {
			received = r.completion.Int64
		} else if v := tokens["completion_tokens"]; v != nil {
			received = v
		}
		status := r.status.String
		if status == "" {
			status = "-"
		}
		logs = append(logs, fmt.Sprintf("%s | %s | %s | %s | %v | %v | %s", ts, model, provider, account, sent, received, status))
	}
	return logs, nil
}

type RequestDetailsFilter struct {
	Page, PageSize                                    int
	Provider, Model, ConnectionID, Status, Start, End string
}

func detailDate(value string) string {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02", "2006-01-02T15:04:05", "2006-01-02T15:04"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC().Format("2006-01-02T15:04:05.000Z")
		}
	}
	return ""
}
func (u Usage) RequestDetails(ctx context.Context, f RequestDetailsFilter) ([]any, int, error) {
	where := []string{}
	args := []any{}
	for _, field := range []struct{ key, value string }{{"provider", f.Provider}, {"model", f.Model}, {"connectionId", f.ConnectionID}, {"status", f.Status}} {
		if field.value != "" {
			where = append(where, field.key+" = ?")
			args = append(args, field.value)
		}
	}
	if value := detailDate(f.Start); value != "" {
		where = append(where, "timestamp >= ?")
		args = append(args, value)
	}
	if value := detailDate(f.End); value != "" {
		where = append(where, "timestamp <= ?")
		args = append(args, value)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := u.DB.QueryRowContext(ctx, "SELECT count(*) FROM requestDetails"+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := u.DB.QueryContext(ctx, "SELECT data FROM requestDetails"+clause+" ORDER BY timestamp DESC LIMIT ? OFFSET ?", append(args, f.PageSize, (f.Page-1)*f.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := []any{}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return nil, 0, err
		}
		var value any
		if json.Unmarshal([]byte(raw), &value) != nil {
			value = map[string]any{}
		}
		list = append(list, value)
	}
	return list, total, rows.Err()
}
func (u Usage) RequestDetailByID(ctx context.Context, id string) (any, error) {
	var raw string
	err := u.DB.QueryRowContext(ctx, "SELECT data FROM requestDetails WHERE id = ?", id).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var value any
	_ = json.Unmarshal([]byte(raw), &value)
	return value, nil
}
