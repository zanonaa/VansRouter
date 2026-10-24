package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestExaSearchBuildsValidatedPayload(t *testing.T) {
	body := map[string]any{"search_type": "news", "domain_filter": []any{"example.com", "-blocked.com"}, "exa_options": map[string]any{"type": "deep", "userLocation": "br", "additionalQueries": []any{"related"}, "contents": map[string]any{"text": map[string]any{"maxCharacters": float64(500), "verbosity": "compact"}}}}
	payload, err := buildExaBody("query", "news", 5, body)
	if err != nil {
		t.Fatal(err)
	}
	if payload["type"] != "deep" || payload["category"] != "news" || payload["userLocation"] != "BR" || payload["numResults"] != 5 {
		t.Fatalf("%v", payload)
	}
	contents := payload["contents"].(map[string]any)
	if contents["highlights"] != true || contents["text"].(map[string]any)["maxCharacters"] != float64(500) {
		t.Fatal(contents)
	}
	for _, invalid := range []map[string]any{{"type": "unknown"}, {"type": "auto", "additionalQueries": []any{"extra"}}, {"category": "people", "excludeDomains": []any{"example.com"}}, {"stream": true}, {"compliance": "hipaa", "type": "instant"}} {
		if _, err := buildExaBody("query", "web", 5, map[string]any{"exa_options": invalid}); err == nil {
			t.Fatalf("accepted invalid options %v", invalid)
		}
	}
}

func TestExaSearchNormalizesCostAndResult(t *testing.T) {
	var data map[string]any
	json.Unmarshal([]byte(`{"requestId":"req","resolvedSearchType":"deep","costDollars":{"total":0.021},"results":[{"title":"Title","url":"https://www.example.com/path?q=1","highlights":["first"],"text":"`+strings.Repeat("a", 400)+`","score":1.4,"author":"Author","publishedDate":"2026-10-22"}]}`), &data)
	results, total, metadata, _, err := normalizeSearch("exa", "web", data, 5, time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC))
	if err != nil || total != 1 || metadata["requestId"] != "req" || metadata["costDollars"].(map[string]any)["total"] != 0.021 {
		t.Fatal(results, total, metadata, err)
	}
	if results[0]["snippet"] != "first" || results[0]["score"] != float64(1) || results[0]["display_url"] != "example.com/path" || results[0]["content"].(map[string]any)["length"] != 400 {
		t.Fatalf("%v", results[0])
	}
	raw := results[0]["provider_raw"].(map[string]any)
	if raw["author"] != "Author" || raw["summary"] != nil {
		t.Fatal(raw)
	}
}
