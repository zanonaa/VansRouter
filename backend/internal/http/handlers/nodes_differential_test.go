package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

type differentialNodeStore struct{}

func (differentialNodeStore) List(context.Context, string) ([]map[string]any, error) { return nil, nil }
func (differentialNodeStore) Create(_ context.Context, data map[string]any) (map[string]any, error) {
	return data, nil
}
func TestNodeCreateAgainstOriginalHandler(t *testing.T) {
	raw, err := os.ReadFile("testdata/node-http-original.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Input  map[string]any `json:"input"`
		Status int            `json:"status"`
		Body   map[string]any `json:"body"`
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	handler := Nodes{Store: differentialNodeStore{}}
	for i, fixture := range fixtures {
		payload, _ := json.Marshal(fixture.Input)
		response := httptest.NewRecorder()
		handler.Create(response, httptest.NewRequest("POST", "/api/provider-nodes", strings.NewReader(string(payload))))
		var body map[string]any
		if err = json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if node, ok := body["node"].(map[string]any); ok {
			kind := node["type"].(string)
			prefix := kind + "-"
			if kind == "openai-compatible" {
				prefix += node["apiType"].(string) + "-"
			}
			id, _ := node["id"].(string)
			if !strings.HasPrefix(id, prefix) {
				t.Fatalf("bad UUID prefix: %s", id)
			}
			node["id"] = prefix + "fixture-uuid"
		}
		if response.Code != fixture.Status || !reflect.DeepEqual(body, fixture.Body) {
			t.Fatalf("case %d input=%v: Go=%d %v original=%d %v", i, fixture.Input, response.Code, body, fixture.Status, fixture.Body)
		}
	}
}
