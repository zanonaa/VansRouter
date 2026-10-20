package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

func (c Connections) Delete(ctx context.Context, id string) (bool, error) {
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var provider string
	err = tx.QueryRowContext(ctx, "SELECT provider FROM providerConnections WHERE id = ?", id).Scan(&provider)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM providerConnections WHERE id = ?", id); err != nil {
		return false, err
	}
	if err = reorderConnections(ctx, tx, provider); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
func (c Connections) Reorder(ctx context.Context, provider string) error {
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = reorderConnections(ctx, tx, provider); err != nil {
		return err
	}
	return tx.Commit()
}

// CreateBulk reproduz createProviderConnectionsBulk: um único commit para
// itens válidos, erros por item para campos ausentes, rollback em erro de DB/JSON.
// Não aplica healthPatch no upsert bulk, assim como a origem.
func (c Connections) CreateBulk(ctx context.Context, items []map[string]any) ([]map[string]any, error) {
	if len(items) == 0 || len(items) > 500 {
		return nil, errors.New("Bulk connection batch must contain 1-500 items")
	}
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	results := []map[string]any{}
	providers := []any{}
	seen := map[any]bool{}
	for _, data := range items {
		provider := data["provider"]
		// Provider IDs são valores escalares no contrato de entrada.
		if _, ok := provider.(string); provider != nil && !ok {
			return nil, errors.New("invalid provider type")
		}
		if !seen[provider] {
			seen[provider] = true
			providers = append(providers, provider)
		}
		if !jsonTruthy(provider) || !jsonTruthy(data["apiKey"]) || !jsonTruthy(data["name"]) {
			name := data["name"]
			if !jsonTruthy(name) {
				name = nil
			}
			results = append(results, map[string]any{"name": name, "ok": false, "error": "provider, apiKey, and name are required"})
			continue
		}
		existing, err := scanConnection(tx.QueryRowContext(ctx, "SELECT "+connectionColumns+" FROM providerConnections WHERE provider = ? AND authType = 'apikey' AND name = ? LIMIT 1", provider, data["name"]))
		if err != nil {
			return nil, err
		}
		now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		updated := existing != nil
		connection := existing
		if updated {
			for key, value := range data {
				connection[key] = value
			}
			connection["authType"] = "apikey"
			connection["updatedAt"] = now
		} else {
			priority := data["priority"]
			if !jsonTruthy(priority) {
				priority = float64(1)
			}
			status := data["testStatus"]
			if !jsonTruthy(status) {
				status = "unknown"
			}
			connection = map[string]any{"id": uuid.NewString(), "provider": provider, "authType": "apikey", "name": data["name"], "priority": priority, "isActive": true, "createdAt": now, "updatedAt": now, "apiKey": data["apiKey"], "testStatus": status}
			if value, ok := data["providerSpecificData"]; ok {
				connection["providerSpecificData"] = value
			}
		}
		if err = writeConnection(ctx, tx, connection); err != nil {
			return nil, err
		}
		results = append(results, map[string]any{"name": data["name"], "ok": true, "id": connection["id"], "updated": updated})
	}
	for _, provider := range providers {
		if err = reorderConnections(ctx, tx, provider); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return results, nil
}
