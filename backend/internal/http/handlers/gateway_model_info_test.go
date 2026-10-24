package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGatewayModelInfoContract(t *testing.T) {
	g := &Gateway{}
	for _, test := range []struct {
		query string
		code  int
		text  string
	}{{"", 400, "Missing required query param"}, {"?id=unknown/no-model", 404, "Model not found"}, {"?id=openai/dall-e-3", 200, "/v1/images/generations"}} {
		w := httptest.NewRecorder()
		g.ModelInfo(w, httptest.NewRequest("GET", "/api/v1/models/info"+test.query, nil))
		if w.Code != test.code || !strings.Contains(w.Body.String(), test.text) {
			t.Fatalf("query %s status %d body %s", test.query, w.Code, w.Body.String())
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatal("missing CORS")
		}
	}
	w := httptest.NewRecorder()
	g.ModelInfo(w, httptest.NewRequest("GET", "/api/v1/models/info?id=openai/dall-e-3", nil))
	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["provider"] != nil || body["alias"] != nil {
		t.Fatal("internal metadata leaked")
	}
}
