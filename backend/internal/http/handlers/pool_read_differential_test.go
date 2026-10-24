package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

type poolReadStore struct{ differentialPoolStore }

func (poolReadStore) List(_ context.Context, active *bool, _ string) ([]map[string]any, error) {
	values := []map[string]any{{"id": "a", "name": "A", "isActive": true}, {"id": "b", "name": "B", "isActive": false}}
	result := []map[string]any{}
	for _, value := range values {
		if active == nil || value["isActive"] == *active {
			result = append(result, value)
		}
	}
	return result, nil
}
func (poolReadStore) Get(_ context.Context, id string) (map[string]any, error) {
	switch id {
	case "a":
		return map[string]any{"id": "a", "name": "A", "isActive": true}, nil
	case "b":
		return map[string]any{"id": "b", "name": "B", "isActive": false}, nil
	case "free":
		return map[string]any{"id": "free", "name": "Free", "isActive": true}, nil
	}
	return nil, nil
}

type poolReadConnections struct{ NodeConnections }

func (poolReadConnections) List(context.Context, string, *bool) ([]map[string]any, error) {
	return []map[string]any{
		{"id": "1", "providerSpecificData": map[string]any{"proxyPoolId": "a", "proxyPoolIds": []any{"a", "a", "b"}}},
		{"id": "2", "providerSpecificData": map[string]any{"proxyPoolIds": []any{"a"}}},
		{"id": "3", "providerSpecificData": map[string]any{}},
	}, nil
}
func TestPoolReadAgainstOriginal(t *testing.T) {
	raw, err := os.ReadFile("testdata/pool-read-original.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Method, Path string
		Status       int
		Body         any
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	h := ProxyPools{Store: poolReadStore{}, Connections: poolReadConnections{}}
	for i, tt := range cases {
		r := httptest.NewRequest(tt.Method, tt.Path, nil)
		route := chi.NewRouteContext()
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) == 4 {
			route.URLParams.Add("id", parts[3])
		}
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
		w := httptest.NewRecorder()
		if tt.Method == "DELETE" {
			h.Delete(w, r)
		} else if len(parts) == 4 {
			h.Get(w, r)
		} else {
			h.List(w, r)
		}
		var got any
		if err = json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if w.Code != tt.Status || !reflect.DeepEqual(got, tt.Body) {
			t.Fatalf("case %d %s %s: Go=%d %v original=%d %v", i, tt.Method, tt.Path, w.Code, got, tt.Status, tt.Body)
		}
	}
}
