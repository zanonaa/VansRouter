package database

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestCombosOriginal(t *testing.T) {
	raw, err := os.ReadFile("testdata/combos-original.json")
	if err != nil {
		t.Fatal(err)
	}
	var steps []struct {
		Method   string
		Args     []any
		Expected any
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
	repo := Combos{DB: db}
	id := ""
	for _, step := range steps {
		args := append([]any{}, step.Args...)
		if len(args) > 0 && args[0] == "fixture-id" {
			args[0] = id
		}
		var got any
		switch step.Method {
		case "getCombos":
			got, err = repo.List(ctx)
		case "createCombo":
			got, err = repo.Create(ctx, args[0].(map[string]any))
			if err == nil {
				id = got.(map[string]any)["id"].(string)
			}
		case "getComboByName":
			got, err = repo.ByName(ctx, args[0])
		case "getComboById":
			got, err = repo.Get(ctx, args[0].(string))
		case "updateCombo":
			got, err = repo.Update(ctx, args[0].(string), args[1].(map[string]any))
		case "deleteCombo":
			got, err = repo.Delete(ctx, args[0].(string))
		}
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(got)
		var normalized any
		_ = json.Unmarshal(encoded, &normalized)
		normalized = normalizeConnectionIDs(stripNodeTimes(normalized), map[string]string{id: "fixture-id"})
		if !reflect.DeepEqual(normalized, step.Expected) {
			t.Fatalf("%s: %v want %v", step.Method, normalized, step.Expected)
		}
	}
}
