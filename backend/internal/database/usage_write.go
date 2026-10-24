package database

import (
	"context"
	"encoding/json"
	"time"
)

// RecordUsage commits history, daily aggregates and lifetime counter together.
func (u Usage) RecordUsage(ctx context.Context, entry map[string]any) error {
	tx, err := u.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tokens, _ := entry["tokens"].(map[string]any)
	number := func(v any) float64 {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		case int64:
			return float64(n)
		}
		return 0
	}
	raw, err := json.Marshal(tokens)
	if err != nil {
		return err
	}
	now := time.Now()
	stamp := now.UTC().Format("2006-01-02T15:04:05.000Z")
	prompt, completion, cached, cost := number(tokens["prompt_tokens"]), number(tokens["completion_tokens"]), number(tokens["cached_tokens"]), number(entry["cost"])
	_, err = tx.ExecContext(ctx, `INSERT INTO usageHistory(timestamp,provider,model,connectionId,apiKey,endpoint,promptTokens,completionTokens,cost,status,tokens,meta) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, stamp, entry["provider"], entry["model"], entry["connectionId"], entry["apiKey"], entry["endpoint"], prompt, completion, cost, "ok", string(raw), "{}")
	if err != nil {
		return err
	}
	dayKey := now.Format("2006-01-02")
	// SQL JSON arithmetic avoids a read-modify-write race between concurrent requests.
	_, err = tx.ExecContext(ctx, `INSERT INTO usageDaily(dateKey,data) VALUES(?,'{}') ON CONFLICT(dateKey) DO NOTHING`, dayKey)
	if err != nil {
		return err
	}
	var dayRaw string
	if err = tx.QueryRowContext(ctx, `SELECT data FROM usageDaily WHERE dateKey=?`, dayKey).Scan(&dayRaw); err != nil {
		return err
	}
	day := map[string]any{}
	_ = json.Unmarshal([]byte(dayRaw), &day)
	if day == nil {
		day = map[string]any{}
	}
	values := map[string]float64{"requests": 1, "promptTokens": prompt, "completionTokens": completion, "cachedTokens": cached, "cost": cost}
	add := func(target map[string]any) {
		for k, v := range values {
			target[k] = number(target[k]) + v
		}
	}
	add(day)
	provider, _ := entry["provider"].(string)
	model, _ := entry["model"].(string)
	connection, _ := entry["connectionId"].(string)
	key, _ := entry["apiKey"].(string)
	endpoint, _ := entry["endpoint"].(string)
	localKey := key
	if localKey == "" {
		localKey = "local-no-key"
	}
	groups := map[string]string{"byProvider": provider, "byModel": model + "|" + provider, "byAccount": connection, "byApiKey": localKey + "|" + model + "|" + provider, "byEndpoint": endpoint + "|" + model + "|" + provider}
	for group, k := range groups {
		if k == "" {
			continue
		}
		bucket, _ := day[group].(map[string]any)
		if bucket == nil {
			bucket = map[string]any{}
			day[group] = bucket
		}
		counter, _ := bucket[k].(map[string]any)
		if counter == nil {
			counter = map[string]any{}
			bucket[k] = counter
		}
		add(counter)
		if group != "byProvider" {
			counter["rawModel"] = model
			counter["provider"] = provider
		}
		if group == "byApiKey" {
			counter["apiKey"] = entry["apiKey"]
		}
		if group == "byEndpoint" {
			counter["endpoint"] = endpoint
		}
	}
	encoded, err := json.Marshal(day)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE usageDaily SET data=? WHERE dateKey=?`, string(encoded), dayKey); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO _meta(key,value) VALUES('totalRequestsLifetime','1') ON CONFLICT(key) DO UPDATE SET value=CAST(CAST(value AS INTEGER)+1 AS TEXT)`); err != nil {
		return err
	}
	return tx.Commit()
}
