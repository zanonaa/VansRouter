package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Usage struct{ DB *sql.DB }

const usageColumns = "timestamp,provider,model,connectionId,apiKey,endpoint,cost,status,tokens"

func scanUsageRows(rows *sql.Rows) ([]map[string]any, error) {
	list := []map[string]any{}
	defer rows.Close()
	for rows.Next() {
		var timestamp string
		var provider, model, connectionID, apiKey, endpoint, status, raw sql.NullString
		var cost sql.NullFloat64
		if err := rows.Scan(&timestamp, &provider, &model, &connectionID, &apiKey, &endpoint, &cost, &status, &raw); err != nil {
			return nil, err
		}
		var tokens any = map[string]any{}
		if json.Unmarshal([]byte(raw.String), &tokens) != nil {
			tokens = map[string]any{}
		}
		var masked any
		if apiKey.Valid && apiKey.String != "" {
			key := []rune(apiKey.String)
			if len(key) <= 12 {
				masked = string(key[:1]) + "***"
			} else {
				masked = string(key[:8]) + "***" + string(key[len(key)-4:])
			}
		}
		var costValue any
		if cost.Valid {
			costValue = cost.Float64
		}
		list = append(list, map[string]any{"timestamp": timestamp, "provider": nullString(provider), "model": nullString(model), "connectionId": nullString(connectionID), "apiKeyMasked": masked, "endpoint": nullString(endpoint), "cost": costValue, "status": nullString(status), "tokens": tokens})
	}
	return list, rows.Err()
}

// History retorna ordem de inserção ASC, tokens JSON e chave mascarada como a origem.
func (u Usage) History(ctx context.Context, provider, model, start, end string) ([]map[string]any, error) {
	query := "SELECT " + usageColumns + " FROM usageHistory"
	where := []string{}
	args := []any{}
	if provider != "" {
		where = append(where, "provider = ?")
		args = append(args, provider)
	}
	if model != "" {
		where = append(where, "model = ?")
		args = append(args, model)
	}
	if start != "" {
		where = append(where, "timestamp >= ?")
		value := detailDate(start)
		if value == "" {
			return nil, fmt.Errorf("Invalid time value")
		}
		args = append(args, value)
	}
	if end != "" {
		where = append(where, "timestamp <= ?")
		value := detailDate(end)
		if value == "" {
			return nil, fmt.Errorf("Invalid time value")
		}
		args = append(args, value)
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY id ASC"
	rows, err := u.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return scanUsageRows(rows)
}

func hourlyLabel(t time.Time) string { return t.Format("15:04") }

func (u Usage) hourlyChart(ctx context.Context, start time.Time, clampToNow bool) ([]map[string]any, error) {
	now := time.Now()
	buckets := make([]map[string]any, 24)
	for i := range buckets {
		buckets[i] = map[string]any{"label": hourlyLabel(start.Add(time.Duration(i) * time.Hour)), "tokens": int64(0), "cost": float64(0), "requests": int64(0)}
	}
	rows, err := u.DB.QueryContext(ctx, "SELECT timestamp,promptTokens,completionTokens,cost FROM usageHistory WHERE timestamp >= ?", start.UTC().Format("2006-01-02T15:04:05.000Z"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type rec struct {
		ts                 string
		prompt, completion sql.NullInt64
		cost               sql.NullFloat64
	}
	for rows.Next() {
		var r rec
		if err := rows.Scan(&r.ts, &r.prompt, &r.completion, &r.cost); err != nil {
			return nil, err
		}
		t, err := time.Parse("2006-01-02T15:04:05.000Z", strings.Replace(r.ts, "Z", "Z", 1))
		if err != nil {
			t, err = time.Parse(time.RFC3339, r.ts)
			if err != nil {
				continue
			}
		}
		end := start.Add(24 * time.Hour)
		if clampToNow && t.After(now) {
			continue
		}
		if t.Before(start) || t.After(end) {
			continue
		}
		idx := int(t.Sub(start) / time.Hour)
		if idx < 0 {
			idx = 0
		}
		if idx > 23 {
			if clampToNow {
				idx = 23
			} else {
				continue
			}
		}
		buckets[idx]["tokens"] = buckets[idx]["tokens"].(int64) + r.prompt.Int64 + r.completion.Int64
		buckets[idx]["cost"] = buckets[idx]["cost"].(float64) + r.cost.Float64
		buckets[idx]["requests"] = buckets[idx]["requests"].(int64) + 1
	}
	return buckets, rows.Err()
}

type daySummary struct {
	Prompt, Completion int64
	Cost               float64
	Requests           int64
}

func (u Usage) dailyMap(ctx context.Context, limit int) (map[string]daySummary, []string, error) {
	query := "SELECT dateKey,data FROM usageDaily"
	args := []any{}
	if limit > 0 {
		query += " WHERE dateKey >= ?"
		now := time.Now()
		cutoff := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -limit+1)
		args = append(args, cutoff.Format("2006-01-02"))
	} else {
		query += " ORDER BY dateKey ASC"
	}
	rows, err := u.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	result := map[string]daySummary{}
	keys := []string{}
	for rows.Next() {
		var dateKey, raw string
		if err := rows.Scan(&dateKey, &raw); err != nil {
			return nil, nil, err
		}
		var data map[string]any
		_ = json.Unmarshal([]byte(raw), &data)
		if data == nil {
			data = map[string]any{}
		}
		s := daySummary{}
		if v, ok := data["promptTokens"].(float64); ok {
			s.Prompt = int64(v)
		}
		if v, ok := data["completionTokens"].(float64); ok {
			s.Completion = int64(v)
		}
		if v, ok := data["cost"].(float64); ok {
			s.Cost = v
		}
		if v, ok := data["requests"].(float64); ok {
			s.Requests = int64(v)
		}
		result[dateKey] = s
		keys = append(keys, dateKey)
	}
	return result, keys, rows.Err()
}

// Chart reproduz getChartData: hoje/24h em buckets horários; demais períodos
// diários a partir de usageDaily, com dias zerados na janela fixa.
func (u Usage) Chart(ctx context.Context, period string) ([]map[string]any, error) {
	now := time.Now()
	if period == "today" {
		y, m, d := now.Date()
		return u.hourlyChart(ctx, time.Date(y, m, d, 0, 0, 0, 0, now.Location()), false)
	}
	if period == "24h" {
		return u.hourlyChart(ctx, now.Add(-24*time.Hour), true)
	}
	bucketCount := 7
	if period == "30d" {
		bucketCount = 30
	} else if period == "60d" {
		bucketCount = 60
	}
	limit := 0
	if period != "all" {
		limit = bucketCount
	}
	dayMap, keys, err := u.dailyMap(ctx, limit)
	if err != nil {
		return nil, err
	}
	var start time.Time
	if period == "all" {
		if len(keys) == 0 {
			return []map[string]any{}, nil
		}
		start, _ = time.ParseInLocation("2006-01-02", keys[0], now.Location())
	} else {
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		start = today.AddDate(0, 0, -(bucketCount - 1))
	}
	dayCount := int(now.Sub(start).Hours()/24) + 1
	if dayCount < 1 {
		dayCount = 1
	}
	list := []map[string]any{}
	for i := 0; i < dayCount; i++ {
		day := start.AddDate(0, 0, i)
		key := day.Format("2006-01-02")
		s, ok := dayMap[key]
		entry := map[string]any{"label": day.Format("Jan 2"), "tokens": s.Prompt + s.Completion, "cost": s.Cost, "requests": s.Requests}
		if !ok {
			entry = map[string]any{"label": day.Format("Jan 2"), "tokens": int64(0), "cost": float64(0), "requests": int64(0)}
		}
		list = append(list, entry)
	}
	return list, nil
}

func (u Usage) DistinctProviders(ctx context.Context) ([]string, error) {
	rows, err := u.DB.QueryContext(ctx, "SELECT DISTINCT provider FROM requestDetails WHERE provider IS NOT NULL ORDER BY provider ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []string{}
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			return nil, err
		}
		list = append(list, provider)
	}
	return list, rows.Err()
}
