package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestAliasesAgainstOriginalJS(t *testing.T) {
	raw, err := os.ReadFile("testdata/aliases-original.json")
	if err != nil {
		t.Fatal(err)
	}
	var steps []struct {
		Method   string `json:"method"`
		Args     []any  `json:"args"`
		Expected any    `json:"expected"`
		After    struct {
			Aliases []struct{ Scope, Key, Value string } `json:"aliases"`
		} `json:"after"`
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
	repo := ModelAliases{DB: db}
	for i, step := range steps {
		var got any
		switch step.Method {
		case "getModelAliases":
			got, err = repo.All(ctx, "modelAliases")
		case "setModelAlias":
			err = repo.Set(ctx, step.Args[0], step.Args[1])
		case "deleteModelAlias":
			err = repo.Delete(ctx, "modelAliases", step.Args[0].(string))
		case "getCustomModels":
			got, err = repo.CustomList(ctx)
		case "addCustomModel":
			data := map[string]any{}
			for k, v := range step.Args[0].(map[string]any) {
				data[k] = v
			}
			if _, ok := data["type"]; !ok {
				data["type"] = "llm"
			}
			got, err = repo.AddCustom(ctx, data)
		case "addCustomModelsBulk":
			items := []map[string]any{}
			for _, v := range step.Args[0].([]any) {
				data := map[string]any{}
				for k, value := range v.(map[string]any) {
					data[k] = value
				}
				items = append(items, data)
			}
			got, err = repo.CustomBulk(ctx, items)
		case "deleteCustomModel":
			data := step.Args[0].(map[string]any)
			kind := "llm"
			if v, ok := data["type"].(string); ok {
				kind = v
			}
			err = repo.Delete(ctx, "customModels", data["providerAlias"].(string)+"|"+data["id"].(string)+"|"+kind)
		default:
			t.Fatalf("unknown %s", step.Method)
		}
		if err != nil {
			t.Fatalf("step %d %s: %v", i, step.Method, err)
		}
		// Modelo pode retornar nil/valor; normalizar via JSON.
		encoded, _ := json.Marshal(got)
		var normalized any
		_ = json.Unmarshal(encoded, &normalized)
		if !reflect.DeepEqual(normalized, step.Expected) {
			t.Fatalf("step %d %s: Go=%v original=%v", i, step.Method, normalized, step.Expected)
		}
		rows, err := db.Query(`SELECT scope,key,value FROM kv ORDER BY scope,key`)
		if err != nil {
			t.Fatal(err)
		}
		after := []struct {
			Scope, Key string
			Value      any
		}{}
		for rows.Next() {
			var s, k, v string
			if err = rows.Scan(&s, &k, &v); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			var parsed any
			_ = json.Unmarshal([]byte(v), &parsed)
			after = append(after, struct {
				Scope, Key string
				Value      any
			}{s, k, parsed})
		}
		rows.Close()
		expected := []struct {
			Scope, Key string
			Value      any
		}{}
		for _, row := range step.After.Aliases {
			var parsed any
			_ = json.Unmarshal([]byte(row.Value), &parsed)
			expected = append(expected, struct {
				Scope, Key string
				Value      any
			}{row.Scope, row.Key, parsed})
		}
		if len(after) != len(expected) {
			t.Fatalf("step %d %s kv: Go=%v original=%v", i, step.Method, after, expected)
		}
		for j := range after {
			if after[j].Scope != expected[j].Scope || after[j].Key != expected[j].Key || !reflect.DeepEqual(after[j].Value, expected[j].Value) {
				t.Fatalf("step %d %s kv row %d: Go=%+v original=%+v", i, step.Method, j, after[j], expected[j])
			}
		}
	}
	_ = sql.ErrNoRows
}
