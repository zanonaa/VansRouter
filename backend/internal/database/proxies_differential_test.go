package database

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestProxiesAgainstOriginalJS(t *testing.T) {
	raw, err := os.ReadFile("testdata/proxies-original.json")
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
	pools := ProxyPools{DB: db}
	fitness := ProxyFitness{DB: db}
	for i, step := range steps {
		var got any
		switch step.Method {
		case "createProxyPool":
			got, err = pools.Create(ctx, step.Args[0].(map[string]any))
		case "getProxyPoolById":
			got, err = pools.Get(ctx, step.Args[0].(string))
		case "updateProxyPool":
			got, err = pools.Update(ctx, step.Args[0].(string), step.Args[1].(map[string]any))
		case "getProxyPools":
			filter := step.Args[0].(map[string]any)
			var active *bool
			if value, ok := filter["isActive"].(bool); ok {
				active = &value
			}
			status, _ := filter["testStatus"].(string)
			got, err = pools.List(ctx, active, status)
		case "deleteProxyPool":
			got, err = pools.Delete(ctx, step.Args[0].(string))
		case "upsertProxyPoolFitness":
			got, err = fitness.Upsert(ctx, step.Args[0].(string), step.Args[1].(string), step.Args[2].(float64), step.Args[3].(string))
		case "listProxyPoolFitness":
			var pool *string
			if len(step.Args) > 0 {
				value := step.Args[0].(string)
				pool = &value
			}
			got, err = fitness.List(ctx, pool)
		case "deleteProxyPoolFitness":
			got, err = fitness.Delete(ctx, step.Args[0].(string), step.Args[1].(string))
		case "deleteProxyPoolFitnessByPool":
			got, err = fitness.DeleteByPool(ctx, step.Args[0].(string))
		case "clearProxyPoolFitness":
			provider := ""
			if len(step.Args) > 0 {
				provider = step.Args[0].(string)
			}
			got, err = fitness.Clear(ctx, provider)
		default:
			t.Fatalf("unknown operation %s", step.Method)
		}
		if err != nil {
			t.Fatalf("step %d %s: %v", i, step.Method, err)
		}
		encoded, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var value any
		if err = json.Unmarshal(encoded, &value); err != nil {
			t.Fatal(err)
		}
		value = stripNodeTimes(value)
		if !reflect.DeepEqual(value, step.Expected) {
			t.Fatalf("step %d %s: Go=%v original=%v", i, step.Method, value, step.Expected)
		}
	}
}
