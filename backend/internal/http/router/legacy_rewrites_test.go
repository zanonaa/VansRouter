package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLegacyPath(t *testing.T) {
	cases := map[string]string{
		"/v1":                       "/api/v1",
		"/v1/":                      "/api/v1",
		"/v1/models":                "/api/v1/models",
		"/v1/chat/completions":      "/api/v1/chat/completions",
		"/v1/v1":                    "/api/v1",
		"/v1/v1/models":             "/api/v1/models",
		"/v1/v1/chat/completions":   "/api/v1/chat/completions",
		"/responses":                "/api/v1/responses",
		"/codex":                    "/api/v1/responses",
		"/codex/responses":          "/api/v1/responses",
		"/v1beta":                   "/api/v1beta",
		"/v1beta/":                  "/api/v1beta",
		"/v1beta/models":            "/api/v1beta/models",
		"/v1beta/models/gemini:gen": "/api/v1beta/models/gemini:gen",
		"/api/v1/chat/completions":  "",
		"/dashboard":                "",
		"/landing":                  "",
		"/v1betamodels":             "",
		"/v11/models":               "",
	}
	for in, want := range cases {
		got, ok := legacyPath(in)
		if want == "" {
			if ok {
				t.Fatalf("%s não deveria ser reescrito (virou %s)", in, got)
			}
			continue
		}
		if !ok || got != want {
			t.Fatalf("%s → %s (quer %s, ok=%v)", in, got, want, ok)
		}
	}
}

func TestLegacyRewritesPreservaQueryENaoMutaOriginal(t *testing.T) {
	var seenPath, seenQuery string
	h := legacyRewrites(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath, seenQuery = r.URL.Path, r.URL.RawQuery
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions?stream=true", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status %d", w.Code)
	}
	if seenPath != "/api/v1/chat/completions" || seenQuery != "stream=true" {
		t.Fatalf("handler viu %s?%s", seenPath, seenQuery)
	}
	if req.URL.Path != "/v1/chat/completions" {
		t.Fatalf("requisição original mutada: %s", req.URL.Path)
	}
}

// O prefixo público continua atendido no roteador real depois do rewrite.
func TestLegacyPrefixoChegaAoRoteador(t *testing.T) {
	r := NewWithSPA(nil, nil, nil, nil, "")
	req := httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
	req.RemoteAddr = "127.0.0.1:5000"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/v1beta/models %d %s", w.Code, w.Body.String())
	}
}
