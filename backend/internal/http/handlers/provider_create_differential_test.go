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

func TestProviderCreateAgainstOriginal(t *testing.T) {
	raw, err := os.ReadFile("testdata/provider-create-original.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Input  any
		Status int
		Body   any
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	h := Providers{CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) { return data, nil }, GetPool: func(_ context.Context, id string) (map[string]any, error) {
		if id == "pool" {
			return map[string]any{"id": id}, nil
		}
		return nil, nil
	}, GetNode: func(_ context.Context, id string) (map[string]any, error) {
		if strings.HasSuffix(id, "-known") {
			return map[string]any{"id": id, "name": "Node", "prefix": "n", "baseUrl": "https://node.test/v1", "apiType": "chat"}, nil
		}
		return nil, nil
	}}
	for i, tt := range cases {
		payload, _ := json.Marshal(tt.Input)
		w := httptest.NewRecorder()
		h.Create(w, httptest.NewRequest("POST", "/api/providers", strings.NewReader(string(payload))))
		var got any
		if err = json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if w.Code != tt.Status || !reflect.DeepEqual(got, tt.Body) {
			t.Fatalf("case %d: Go=%d %v original=%d %v", i, w.Code, got, tt.Status, tt.Body)
		}
	}
}
