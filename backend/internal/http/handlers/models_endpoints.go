package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
)

type DisabledModels struct{ DB *sql.DB }

func (d DisabledModels) All(ctx context.Context) (map[string][]string, error) {
	rows, err := d.DB.QueryContext(ctx, `SELECT key,value FROM kv WHERE scope='disabledModels'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string][]string{}
	for rows.Next() {
		var key, raw string
		if err = rows.Scan(&key, &raw); err != nil {
			return nil, err
		}
		var ids []string
		_ = json.Unmarshal([]byte(raw), &ids)
		result[key] = ids
	}
	return result, rows.Err()
}
func (d DisabledModels) Set(ctx context.Context, provider string, ids []string) error {
	raw, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	_, err = d.DB.ExecContext(ctx, `INSERT INTO kv(scope,key,value) VALUES('disabledModels',?,?) ON CONFLICT(scope,key) DO UPDATE SET value=excluded.value`, provider, string(raw))
	return err
}
func (d DisabledModels) Remove(ctx context.Context, provider string) error {
	_, err := d.DB.ExecContext(ctx, `DELETE FROM kv WHERE scope='disabledModels' AND key=?`, provider)
	return err
}

type ModelsEndpoints struct {
	Disabled DisabledModels
	Aliases  AliasStore
}

func (h ModelsEndpoints) DisabledGet(w http.ResponseWriter, r *http.Request) {
	all, err := h.Disabled.All(r.Context())
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch disabled models"})
		return
	}
	if provider := r.URL.Query().Get("providerAlias"); provider != "" {
		ids := all[provider]
		if ids == nil {
			ids = []string{}
		}
		nodeReply(w, 200, map[string]any{"ids": ids})
		return
	}
	nodeReply(w, 200, map[string]any{"disabled": all})
}
func (h ModelsEndpoints) DisabledPost(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to disable models"})
		return
	}
	provider, ok := body["providerAlias"].(string)
	ids, idsOk := body["ids"].([]any)
	if !ok || provider == "" || !idsOk {
		nodeReply(w, 400, map[string]string{"error": "providerAlias and ids[] required"})
		return
	}
	values := []string{}
	for _, id := range ids {
		if s, ok := id.(string); ok {
			values = append(values, s)
		}
	}
	if err = h.Disabled.mergeDisabled(r.Context(), provider, values, false); err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to disable models"})
		return
	}
	nodeReply(w, 200, map[string]bool{"success": true})
}
func (h ModelsEndpoints) DisabledDelete(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("providerAlias")
	if provider == "" {
		nodeReply(w, 400, map[string]string{"error": "providerAlias required"})
		return
	}
	id := r.URL.Query().Get("id")
	var err error
	if id == "" {
		if err = h.Disabled.Remove(r.Context(), provider); err != nil {
			nodeReply(w, 500, map[string]string{"error": "Failed to enable models"})
			return
		}
		nodeReply(w, 200, map[string]bool{"success": true})
		return
	}
	if err = h.Disabled.mergeDisabled(r.Context(), provider, []string{id}, true); err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to enable models"})
		return
	}
	nodeReply(w, 200, map[string]bool{"success": true})
}
func (h ModelsEndpoints) List(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]string{"error": "Failed to fetch models"}) }
	aliases, err := h.Aliases.All(r.Context(), "modelAliases")
	if err != nil {
		fail()
		return
	}
	disabled, err := h.Disabled.All(r.Context())
	if err != nil {
		fail()
		return
	}
	catalog := providersModels()
	models := []map[string]any{}
	for _, m := range catalog.Models {
		provider, _ := m["provider"].(string)
		model, _ := m["model"].(string)
		if containsString(disabled[provider], model) {
			continue
		}
		item := map[string]any{}
		for k, v := range m {
			item[k] = v
		}
		full := m["fullModel"].(string)
		alias := model
		if value, ok := aliases[full].(string); ok {
			alias = value
		}
		item["alias"] = alias
		models = append(models, item)
	}
	nodeReply(w, 200, map[string]any{"models": models})
}
func containsString(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
func (h ModelsEndpoints) SetAlias(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]string{"error": "Failed to update alias"}) }
	body, err := poolInput(r)
	if err != nil || body == nil {
		fail()
		return
	}
	model, okM := body["model"].(string)
	alias, okA := body["alias"].(string)
	if !okM || model == "" || !okA || alias == "" {
		nodeReply(w, 400, map[string]string{"error": "Model and alias required"})
		return
	}
	aliases, err := h.Aliases.All(r.Context(), "modelAliases")
	if err != nil {
		fail()
		return
	}
	for key, value := range aliases {
		if s, ok := value.(string); ok && s == alias && key != model {
			nodeReply(w, 400, map[string]string{"error": "Alias already in use"})
			return
		}
	}
	if err = h.Aliases.Set(r.Context(), model, alias); err != nil {
		fail()
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "model": model, "alias": alias})
}
