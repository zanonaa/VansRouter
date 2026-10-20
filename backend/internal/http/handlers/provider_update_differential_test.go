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

func TestProviderIDAgainstOriginal(t *testing.T) {
	raw, err := os.ReadFile("testdata/provider-update-original.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Method, ID string
		Input      any
		Status     int
		Body       any
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	existing := func() map[string]any {
		return map[string]any{"id": "conn", "provider": "openai", "authType": "apikey", "name": "Existing", "apiKey": "secret", "accessToken": "access", "refreshToken": "refresh", "idToken": "id-secret", "providerSpecificData": map[string]any{"custom": true, "proxyPoolId": "old"}}
	}
	h := Providers{GetConnection: func(_ context.Context, id string) (map[string]any, error) {
		if id == "missing" {
			return nil, nil
		}
		return existing(), nil
	}, GetPool: func(_ context.Context, id string) (map[string]any, error) {
		if id == "pool" {
			return map[string]any{"id": id}, nil
		}
		return nil, nil
	}, UpdateConnection: func(_ context.Context, _ string, patch map[string]any) (map[string]any, error) {
		value := existing()
		for k, v := range patch {
			value[k] = v
		}
		return value, nil
	}, DeleteConnection: func(_ context.Context, id string) (bool, error) { return id != "missing", nil }}
	for i, tt := range cases {
		payload, _ := json.Marshal(tt.Input)
		r := httptest.NewRequest(tt.Method, "/api/providers/"+tt.ID, strings.NewReader(string(payload)))
		route := chi.NewRouteContext()
		route.URLParams.Add("id", tt.ID)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
		w := httptest.NewRecorder()
		switch tt.Method {
		case "GET":
			h.Get(w, r)
		case "PUT":
			h.Update(w, r)
		case "DELETE":
			h.Delete(w, r)
		}
		var got any
		if err = json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if w.Code != tt.Status || !reflect.DeepEqual(got, tt.Body) {
			t.Fatalf("case %d: Go=%d %v original=%d %v", i, w.Code, got, tt.Status, tt.Body)
		}
	}
}
