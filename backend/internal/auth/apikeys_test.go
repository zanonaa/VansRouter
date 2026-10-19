package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// apiKeysHTTP monta um *HTTP com stubs em memória para os handlers de keys.
func apiKeysHTTP(t *testing.T) (*HTTP, *[]map[string]any) {
	t.Helper()
	h := testHTTP(t)
	store := &[]map[string]any{}
	h.Machine = NewMachineID(t.TempDir())
	h.ListAPIKeys = func(context.Context) ([]map[string]any, error) { return *store, nil }
	h.GetAPIKey = func(_ context.Context, id string) (map[string]any, error) {
		for _, k := range *store {
			if k["id"] == id {
				return k, nil
			}
		}
		return nil, nil
	}
	h.CreateAPIKey = func(_ context.Context, name, machineId string) (map[string]any, error) {
		key := map[string]any{"id": "id-" + name, "key": "sk-" + machineId + "-aaaaaa-crc8crc8", "name": name, "machineId": machineId, "isActive": true, "createdAt": "agora"}
		*store = append(*store, key)
		return key, nil
	}
	h.UpdateAPIKey = func(ctx context.Context, id string, data map[string]any) (map[string]any, error) {
		current, _ := h.GetAPIKey(ctx, id)
		if current == nil {
			return nil, nil
		}
		for k, v := range data {
			current[k] = v
		}
		return current, nil
	}
	h.DeleteAPIKey = func(_ context.Context, id string) (bool, error) {
		for i, k := range *store {
			if k["id"] == id {
				*store = append((*store)[:i], (*store)[i+1:]...)
				return true, nil
			}
		}
		return false, nil
	}
	h.ValidateAPIKey = func(ctx context.Context, key string) (map[string]any, error) {
		for _, k := range *store {
			if k["key"] == key {
				if active, _ := k["isActive"].(bool); active {
					return k, nil
				}
			}
		}
		return nil, nil
	}
	return h, store
}

func TestAPIKeysHandlers(t *testing.T) {
	h, store := apiKeysHTTP(t)
	// Lista vazia retorna {"keys":[]}.
	w := httptest.NewRecorder()
	h.APIKeysGet(w, request("GET", "/api/keys", "", "127.0.0.1:1234"))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"keys":[]`) {
		t.Fatalf("lista vazia: %d %s", w.Code, w.Body.String())
	}
	// POST sem nome → 400.
	w = httptest.NewRecorder()
	h.APIKeysPost(w, request("POST", "/api/keys", `{}`, "127.0.0.1:1234"))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Name is required") {
		t.Fatalf("sem nome: %d %s", w.Code, w.Body.String())
	}
	// POST com nome em branco → 400.
	w = httptest.NewRecorder()
	h.APIKeysPost(w, request("POST", "/api/keys", `{"name":"   "}`, "127.0.0.1:1234"))
	if w.Code != 400 {
		t.Fatalf("nome em branco: %d", w.Code)
	}
	// POST válido → 201 com formato original.
	w = httptest.NewRecorder()
	h.APIKeysPost(w, request("POST", "/api/keys", `{"name":"minha"}`, "127.0.0.1:1234"))
	if w.Code != 201 || len(*store) != 1 {
		t.Fatalf("criação: %d %s", w.Code, w.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	for _, field := range []string{"key", "name", "id", "machineId"} {
		if created[field] == nil {
			t.Fatalf("campo %s ausente: %v", field, created)
		}
	}
	// GET por id.
	r := chiRoute("/api/keys/{id}", "id-minha", http.MethodGet)
	w = httptest.NewRecorder()
	h.APIKeyGet(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"key":"sk-`) {
		t.Fatalf("get por id: %d %s", w.Code, w.Body.String())
	}
	// GET id inexistente → 404.
	r = chiRoute("/api/keys/{id}", "id-nada", http.MethodGet)
	w = httptest.NewRecorder()
	h.APIKeyGet(w, r)
	if w.Code != 404 || !strings.Contains(w.Body.String(), "Key not found") {
		t.Fatalf("id inexistente: %d %s", w.Code, w.Body.String())
	}
	// PUT id inexistente → 404.
	r = chiRoute("/api/keys/{id}", "id-nada", http.MethodPut)
	w = httptest.NewRecorder()
	h.APIKeyPut(w, r)
	if w.Code != 404 {
		t.Fatalf("put inexistente: %d", w.Code)
	}
	// PUT válido: desativa e define allowedKinds=[].
	r = chiRoute("/api/keys/{id}", "id-minha", http.MethodPut)
	r.Body = io.NopCloser(strings.NewReader(`{"isActive":false,"allowedKinds":[]}`))
	w = httptest.NewRecorder()
	h.APIKeyPut(w, r)
	if w.Code != 200 {
		t.Fatalf("put: %d %s", w.Code, w.Body.String())
	}
	var updated map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &updated)
	keyRec := updated["key"].(map[string]any)
	if keyRec["isActive"] != false {
		t.Fatalf("update divergente: %v", keyRec)
	}
	if kinds, ok := keyRec["allowedKinds"].([]any); !ok || len(kinds) != 0 {
		t.Fatalf("allowedKinds [] divergente: %v", keyRec["allowedKinds"])
	}
	// DELETE válido e repetido.
	r = chiRoute("/api/keys/{id}", "id-minha", http.MethodDelete)
	w = httptest.NewRecorder()
	h.APIKeyDelete(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Key deleted successfully") {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.APIKeyDelete(w, r)
	if w.Code != 404 {
		t.Fatalf("delete duplicado: %d", w.Code)
	}
	if len(*store) != 0 {
		t.Fatal("store não vazia após delete")
	}
}

// chiRoute cria uma requisição com o param {id} resolvido pelo chi.
func chiRoute(pattern, id, method string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	var body string
	if method == http.MethodPut {
		body = `{}`
	}
	r := httptest.NewRequest(method, "/api/keys/"+id, strings.NewReader(body))
	return r.WithContext(context.WithValue(context.Background(), chi.RouteCtxKey, rctx))
}

func TestAPIKeyMiddlewareValidation(t *testing.T) {
	h, _ := apiKeysHTTP(t)
	created, _ := h.CreateAPIKey(context.Background(), "ativa", h.MachineID())
	rawKey := created["key"].(string)
	// Remoto com XFF: sem key → 401; com key válida → passa; inválida → 401.
	remote := func(header string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("X-Forwarded-For", "192.0.2.10")
		if header != "" {
			r.Header.Set("Authorization", "Bearer "+header)
		}
		return r
	}
	if h.canAccessPublicLLMAPI(remote("")) {
		t.Fatal("remoto sem key passou")
	}
	if !h.canAccessPublicLLMAPI(remote(rawKey)) {
		t.Fatal("key válida recusada")
	}
	if h.canAccessPublicLLMAPI(remote("sk-invalida")) {
		t.Fatal("key inválida passou")
	}
	// Local passa sem key.
	localReq := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	localReq.RemoteAddr = "127.0.0.1:1234"
	if !h.canAccessPublicLLMAPI(localReq) {
		t.Fatal("local bloqueado")
	}
}
