package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	for _, method := range []string{"GET", "OPTIONS"} {
		t.Run(method, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(method, "/api/health", nil)
			h := Bootstrap{}
			if method == "OPTIONS" {
				h.HealthOptions(w, r)
			} else {
				h.Health(w, r)
			}
			want := 200
			if method == "OPTIONS" {
				want = 204
			}
			if w.Code != want {
				t.Fatalf("status %d", w.Code)
			}
			if w.Header().Get("Access-Control-Allow-Origin") != "*" || w.Header().Get("Access-Control-Allow-Methods") != "GET, OPTIONS" || w.Header().Get("Access-Control-Allow-Headers") != "*" {
				t.Fatal("CORS divergente")
			}
			if method == "GET" {
				var body map[string]bool
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || !body["ok"] {
					t.Fatal("payload divergente")
				}
			} else if w.Body.Len() != 0 {
				t.Fatal("OPTIONS deve ter corpo vazio")
			}
		})
	}
}

func TestReadiness(t *testing.T) {
	for _, failure := range []bool{false, true} {
		called := false
		h := Bootstrap{Ready: func(ctx context.Context) error {
			called = true
			if failure {
				return errors.New("detalhe privado")
			}
			return nil
		}}
		w := httptest.NewRecorder()
		h.Readiness(w, httptest.NewRequest("GET", "/api/ready", nil))
		want := 200
		state := "ready"
		if failure {
			want = 503
			state = "unavailable"
		}
		if !called || w.Code != want || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("contrato de prontidão divergente")
		}
		var body struct {
			OK       bool   `json:"ok"`
			Database string `json:"database"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Database != state || body.OK == failure {
			t.Fatal("payload de prontidão divergente")
		}
		if strings.Contains(w.Body.String(), "privado") {
			t.Fatal("erro interno exposto")
		}
	}
	w := httptest.NewRecorder()
	Bootstrap{}.Readiness(w, httptest.NewRequest("GET", "/api/ready", nil))
	if w.Code != 503 {
		t.Fatal("dependência ausente deve falhar fechada")
	}
}

func TestInit(t *testing.T) {
	w := httptest.NewRecorder()
	Bootstrap{}.Init(w, httptest.NewRequest("GET", "/api/init", nil))
	if w.Code != 200 || w.Body.String() != "Initialized" || w.Header().Get("Content-Type") != "text/plain;charset=UTF-8" {
		t.Fatal("contrato de inicialização divergente")
	}
}

func TestLocale(t *testing.T) {
	tests := []struct {
		body    string
		status  int
		message string
	}{
		{`{"locale":"pt-BR"}`, 200, ""},
		{`{"locale":"zh"}`, 400, "Invalid locale"},
		{`{"locale":"PT-br"}`, 400, "Invalid locale"},
		{`{}`, 400, "Invalid locale"},
		{`{"locale":null}`, 400, "Invalid locale"},
		{`{"locale":false}`, 400, "Invalid locale"},
		{`[]`, 400, "Invalid locale"},
		{`1`, 400, "Invalid locale"},
		{`null`, 500, "Failed to set locale"},
		{`{`, 500, "Failed to set locale"},
		{``, 500, "Failed to set locale"},
		{`{"locale":"pt-BR"} {}`, 500, "Failed to set locale"},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			w := httptest.NewRecorder()
			Bootstrap{}.Locale(w, httptest.NewRequest(http.MethodPost, "/api/locale", strings.NewReader(tt.body)))
			if w.Code != tt.status {
				t.Fatalf("status %d, esperado %d", w.Code, tt.status)
			}
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			cookies := w.Result().Cookies()
			if tt.status == 200 {
				if body["success"] != true || body["locale"] != "pt-BR" {
					t.Fatal("payload divergente")
				}
				if len(cookies) != 1 {
					t.Fatal("cookie ausente")
				}
				c := cookies[0]
				if c.Name != "locale" || c.Value != "pt-BR" || c.Path != "/" || c.MaxAge != 31536000 || c.HttpOnly || c.Secure || c.SameSite != 0 {
					t.Fatal("atributos do cookie divergentes")
				}
			} else {
				if body["error"] != tt.message || len(cookies) != 0 {
					t.Fatal("erro ou cookie divergente")
				}
			}
		})
	}
}
