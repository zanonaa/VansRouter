package database

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func normalizeConnectionIDs(v any, ids map[string]string) any {
	switch value := v.(type) {
	case map[string]any:
		for k, item := range value {
			if k == "id" {
				if id, ok := item.(string); ok {
					if name, exists := ids[id]; exists {
						value[k] = name
					}
				}
			} else {
				value[k] = normalizeConnectionIDs(item, ids)
			}
		}
	case []any:
		for i := range value {
			value[i] = normalizeConnectionIDs(value[i], ids)
		}
	}
	return v
}
func TestConnectionsAgainstOriginalJS(t *testing.T) {
	raw, err := os.ReadFile("testdata/connections-original.json")
	if err != nil {
		t.Fatal(err)
	}
	var steps []struct {
		Method   string `json:"method"`
		Args     []any  `json:"args"`
		Save     string `json:"save"`
		Expected any    `json:"expected"`
	}
	if err = json.Unmarshal(raw, &steps); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, dir, "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	repo := Connections{DB: db}
	references := map[string]string{}
	ids := map[string]string{}
	for i, step := range steps {
		args := append([]any{}, step.Args...)
		for j, v := range args {
			if name, ok := v.(string); ok && strings.HasPrefix(name, "$") {
				args[j] = references[name[1:]]
			}
		}
		var got any
		switch step.Method {
		case "createProviderConnection":
			got, err = repo.Create(ctx, args[0].(map[string]any))
		case "getProviderConnectionById":
			got, err = repo.Get(ctx, args[0].(string))
		case "updateProviderConnection":
			got, err = repo.Update(ctx, args[0].(string), args[1].(map[string]any))
		case "getProviderConnections":
			filter := args[0].(map[string]any)
			provider, _ := filter["provider"].(string)
			var active *bool
			if value, ok := filter["isActive"].(bool); ok {
				active = &value
			}
			got, err = repo.List(ctx, provider, active)
		case "createProviderConnectionsBulk":
			items := []map[string]any{}
			for _, v := range args[0].([]any) {
				items = append(items, v.(map[string]any))
			}
			got, err = repo.CreateBulk(ctx, items)
		case "deleteProviderConnection":
			got, err = repo.Delete(ctx, args[0].(string))
		case "cleanupProviderConnections":
			got, err = repo.Cleanup(ctx)
		default:
			t.Fatal(step.Method)
		}
		if err != nil {
			t.Fatalf("step %d %s: %v", i, step.Method, err)
		}
		if step.Save != "" {
			id := got.(map[string]any)["id"].(string)
			references[step.Save] = id
			ids[id] = step.Save
		}
		encoded, _ := json.Marshal(got)
		var value any
		if err = json.Unmarshal(encoded, &value); err != nil {
			t.Fatal(err)
		}
		value = normalizeConnectionIDs(stripNodeTimes(value), ids)
		if !reflect.DeepEqual(value, step.Expected) {
			t.Fatalf("step %d %s: Go=%v original=%v", i, step.Method, value, step.Expected)
		}
	}
}
