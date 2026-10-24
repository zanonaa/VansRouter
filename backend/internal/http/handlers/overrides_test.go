package handlers

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestOverrideHeaders(t *testing.T) {
	state := map[string]any{}
	h := Overrides{Read: func(context.Context) (map[string]any, error) { return state, nil }, Write: func(_ context.Context, updates map[string]any) (map[string]any, error) {
		for k, v := range updates {
			state[k] = v
		}
		return state, nil
	}}
	call := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PUT", "/api/providers/openai/overrides", strings.NewReader(body))
		route := chi.NewRouteContext()
		route.URLParams.Add("id", "openai")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
		w := httptest.NewRecorder()
		h.Put(w, r)
		return w
	}
	for _, body := range []string{`{"headers":[]}`, `{"headers":{"Host":"x"}}`, `{"headers":{"Authorization":"x"}}`, `{"headers":{"x-bad":"x\r\ny"}}`, `{"headers":{"x token":"x"}}`, `{"headers":{"x-number":12}}`} {
		if w := call(body); w.Code != 400 {
			t.Fatalf("%s: %d %s", body, w.Code, w.Body.String())
		}
	}
	if w := call(`{"headers":{"X-Custom":"ok","X-Empty":""}}`); w.Code != 200 || !strings.Contains(w.Body.String(), "ok") {
		t.Fatal(w.Code, w.Body.String())
	}
	if state["providerOverrides"].(map[string]any)["openai"] == nil {
		t.Fatal("not persisted")
	}
	if w := call(`{}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if len(state["providerOverrides"].(map[string]any)) != 0 {
		t.Fatal("not cleared")
	}
}
