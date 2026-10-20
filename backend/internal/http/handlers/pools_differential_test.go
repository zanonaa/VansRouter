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

type differentialPoolStore struct{}

func (differentialPoolStore) List(context.Context, *bool, string) ([]map[string]any, error) {
	return nil, nil
}
func (differentialPoolStore) Create(_ context.Context, data map[string]any) (map[string]any, error) {
	return data, nil
}
func (differentialPoolStore) Get(_ context.Context, id string) (map[string]any, error) {
	if id == "missing" {
		return nil, nil
	}
	return map[string]any{"id": id, "name": "Existing", "proxyUrl": "http://existing.test", "isActive": true}, nil
}
func (differentialPoolStore) Update(_ context.Context, id string, data map[string]any) (map[string]any, error) {
	data["id"] = id
	return data, nil
}
func (differentialPoolStore) Delete(context.Context, string) (map[string]any, error) { return nil, nil }
func TestPoolHTTPAgainstOriginal(t *testing.T) {
	raw, err := os.ReadFile("testdata/pool-http-original.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Method string `json:"method"`
		ID     string `json:"id"`
		Input  any    `json:"input"`
		Status int    `json:"status"`
		Body   any    `json:"body"`
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	h := ProxyPools{Store: differentialPoolStore{}}
	for i, tt := range cases {
		payload, _ := json.Marshal(tt.Input)
		r := httptest.NewRequest(tt.Method, "/api/proxy-pools/"+tt.ID, strings.NewReader(string(payload)))
		route := chi.NewRouteContext()
		route.URLParams.Add("id", tt.ID)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
		w := httptest.NewRecorder()
		if tt.Method == "POST" {
			h.Create(w, r)
		} else {
			h.Update(w, r)
		}
		var got any
		if err = json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if w.Code != tt.Status || !reflect.DeepEqual(got, tt.Body) {
			t.Fatalf("case %d %s input=%v: Go=%d %v original=%d %v", i, tt.Method, tt.Input, w.Code, got, tt.Status, tt.Body)
		}
	}
}
