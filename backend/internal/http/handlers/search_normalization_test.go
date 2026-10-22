package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSearchNormalizesSerperAndTavily(t *testing.T) {
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	var payload map[string]any
	json.Unmarshal([]byte(`{"organic":[{"title":"Result","link":"https://www.example.com/path?x=1","snippet":"text","date":"today"}],"searchParameters":{"totalResults":9}}`), &payload)
	results, total, _, err := normalizeSearch("serper", "web", payload, 10, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0]["url"] != "https://www.example.com/path?x=1" || results[0]["display_url"] != "example.com/path" || results[0]["position"] != 1 || total != float64(9) {
		t.Fatal(results)
	}
	json.Unmarshal([]byte(`{"results":[{"title":"T","url":"https://example.com","content":"snippet","raw_content":"full","score":2}]}`), &payload)
	results, total, _, err = normalizeSearch("tavily", "web", payload, 10, now)
	if err != nil {
		t.Fatal(err)
	}
	if results[0]["score"] != float64(1) || results[0]["content"].(map[string]any)["text"] != "full" || total != 1 {
		t.Fatal(results)
	}
}
func TestSearchRequestAppliesFiltersAndLimits(t *testing.T) {
	g := webGateway(t, nil)
	_, err := g.DB.Exec(`INSERT INTO providerConnections VALUES('t','tavily','apikey',NULL,NULL,1,1,'{"apiKey":"secret"}','now','now')`)
	if err != nil {
		t.Fatal(err)
	}
	method, _, headers, payload, err := buildBasicSearch("tavily", "https://api.tavily.com/search", "secret", "golang", "news", 5, map[string]any{"domain_filter": []any{"example.com", "-blocked.com"}, "country": "br"})
	if err != nil || method != "POST" || payload["topic"] != "news" || headers["Authorization"] != "Bearer secret" {
		t.Fatal(payload)
	}
	domains := payload["exclude_domains"].([]any)
	if domains[0] != "blocked.com" {
		t.Fatal(payload)
	}
	g.Client = &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) { return nil, fmt.Errorf("blocked") })}
	w := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/", strings.NewReader(`{"provider":"tavily","query":"x"}`))
	request.Header.Set("Authorization", "Bearer secret")
	g.Search(w, request)
	if w.Code != 502 {
		t.Fatal(w.Body.String())
	}
}
