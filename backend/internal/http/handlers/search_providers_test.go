package handlers

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"
)

func TestAdditionalSearchBuilders(t *testing.T) {
	for _, provider := range []string{"google-pse", "searchapi", "youcom", "tinyfish", "linkup"} {
		t.Run(provider, func(t *testing.T) {
			body := map[string]any{"provider_options": map[string]any{"cx": "engine", "depth": "deep"}, "country": "BR", "language": "pt", "domain_filter": []any{"example.com", "-blocked.com"}, "offset": float64(10)}
			method, target, headers, payload, err := buildAdditionalSearch(provider, "https://provider.example/search", "token", "query", "news", 5, body)
			if err != nil {
				t.Fatal(err)
			}
			parsed, _ := url.Parse(target)
			if provider == "linkup" {
				if method != "POST" || payload["depth"] != "deep" || payload["maxResults"] != 5 {
					t.Fatal(payload)
				}
				return
			}
			if method != "GET" {
				t.Fatal(method)
			}
			switch provider {
			case "google-pse":
				if parsed.Query().Get("cx") != "engine" || parsed.Query().Get("start") != "11" {
					t.Fatal(target)
				}
			case "searchapi":
				if parsed.Query().Get("engine") != "google_news" || parsed.Query().Get("page") != "3" {
					t.Fatal(target)
				}
			case "youcom":
				if headers["X-API-Key"] != "token" || parsed.Query().Get("exclude_domains") != "blocked.com" {
					t.Fatal(target, headers)
				}
			case "tinyfish":
				if parsed.Query().Get("page") != "1" || parsed.Query().Get("domain_type") != "news" {
					t.Fatal(target)
				}
			}
		})
	}
}
func TestAdditionalSearchNormalization(t *testing.T) {
	for _, test := range []struct{ provider, payload string }{{"google-pse", `{"items":[{"title":"title","link":"https://example.com","snippet":"snippet"}],"searchInformation":{"totalResults":"10"}}`}, {"searchapi", `{"organic_results":[{"title":"title","link":"https://example.com","snippet":"snippet"}]}`}, {"linkup", `{"results":[{"name":"title","url":"https://example.com","content":"snippet"}]}`}, {"youcom", `{"results":{"web":[{"title":"title","url":"https://example.com","snippets":["snippet"]}]}}`}, {"tinyfish", `{"results":[{"title":"title","url":"https://example.com","snippet":"snippet"}],"total_results":9}`}} {
		var data map[string]any
		json.Unmarshal([]byte(test.payload), &data)
		results, _, err := normalizeSearch(test.provider, "web", data, 5, time.Now())
		if err != nil || len(results) != 1 || results[0]["title"] != "title" || results[0]["snippet"] != "snippet" {
			t.Fatalf("%s %v %v", test.provider, results, err)
		}
	}
}
