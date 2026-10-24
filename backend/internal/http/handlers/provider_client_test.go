package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderClientSanitizesAndPaginates(t *testing.T) {
	store := &availabilityStub{items: []map[string]any{
		{"id": "a", "provider": "kiro", "authType": "api_key", "name": strings.Repeat("a", 40), "accessToken": "SECRET", "priority": float64(2), "isActive": false, "providerSpecificData": map[string]any{"region": "us-east-1", "clientSecret": "SECRET"}},
		{"id": "b", "provider": "claude", "authType": "oauth", "priority": float64(1)},
		{"id": "c", "provider": "unknown", "authType": "oauth"},
	}}
	h := &ProviderClient{Connections: store}
	w := httptest.NewRecorder()
	h.Get(w, httptest.NewRequest("GET", "/?provider=kiro&page=100&pageSize=1&accountStatus=inactive", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), "SECRET") {
		t.Fatalf("status or secret leak: %d", w.Code)
	}
	var result struct {
		Connections []map[string]any `json:"connections"`
		Pagination  map[string]int   `json:"pagination"`
		Totals      map[string]int   `json:"totals"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Connections) != 1 || result.Connections[0]["name"] != "aaaaaaaa***" || result.Pagination["page"] != 1 || result.Totals["eligibleConnections"] != 2 {
		t.Fatalf("unexpected response: %v", result)
	}
}
