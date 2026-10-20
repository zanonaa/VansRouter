package database

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func stripNodeTimes(value any) any {
	switch v := value.(type) {
	case map[string]any:
		result := map[string]any{}
		for key, item := range v {
			if key != "createdAt" && key != "updatedAt" {
				result[key] = stripNodeTimes(item)
			}
		}
		return result
	case []any:
		for i := range v {
			v[i] = stripNodeTimes(v[i])
		}
		return v
	}
	return value
}
func TestNodesAgainstOriginalJS(t *testing.T) {
	raw, err := os.ReadFile("testdata/nodes-original.json")
	if err != nil {
		t.Fatal(err)
	}
	var steps []struct {
		Method   string `json:"method"`
		Args     []any  `json:"args"`
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
	repo := Nodes{DB: db}
	for _, step := range steps {
		var got any
		switch step.Method {
		case "createProviderNode":
			got, err = repo.Create(ctx, step.Args[0].(map[string]any))
		case "getProviderNodeById":
			got, err = repo.Get(ctx, step.Args[0].(string))
		case "updateProviderNode":
			got, err = repo.Update(ctx, step.Args[0].(string), step.Args[1].(map[string]any))
		case "getProviderNodes":
			filter := step.Args[0].(map[string]any)
			kind, _ := filter["type"].(string)
			got, err = repo.List(ctx, kind)
		case "deleteProviderNode":
			got, err = repo.Delete(ctx, step.Args[0].(string))
		default:
			t.Fatalf("unknown step: %s", step.Method)
		}
		if err != nil {
			t.Fatalf("%s: %v", step.Method, err)
		}
		normalized, _ := json.Marshal(got)
		var value any
		if err = json.Unmarshal(normalized, &value); err != nil {
			t.Fatal(err)
		}
		value = stripNodeTimes(value)
		if !reflect.DeepEqual(value, step.Expected) {
			t.Fatalf("%s: Go=%v original=%v", step.Method, value, step.Expected)
		}
	}
}
