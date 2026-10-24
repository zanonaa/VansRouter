package handlers

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"
)

func TestXquikOllamaAndGLMSearch(t *testing.T) {
	_, target, headers, _, err := buildAdditionalSearch("xquik", "https://xquik.example/search", "token", "query", "x", 5, map[string]any{"provider_options": map[string]any{"queryType": "Latest", "cursor": "next"}, "language": "pt"})
	parsed, _ := url.Parse(target)
	if err != nil || headers["x-api-key"] != "token" || parsed.Query().Get("queryType") != "Latest" || parsed.Query().Get("cursor") != "next" {
		t.Fatal(target, headers, err)
	}
	if _, _, _, _, err = buildAdditionalSearch("xquik", "https://xquik.example", "token", "query", "x", 5, map[string]any{"provider_options": map[string]any{"queryType": "bad"}}); err == nil {
		t.Fatal("accepted invalid query type")
	}
	var tweets map[string]any
	json.Unmarshal([]byte(`{"tweets":[{"id":"1/2","text":"hello","author":{"username":"user name"},"createdAt":"today","media":[{"mediaUrl":"https://cdn.example/a"}]}],"has_next_page":true,"next_cursor":"cursor"}`), &tweets)
	results, total, _, pagination, err := normalizeSearch("xquik", "x", tweets, 5, time.Now())
	if err != nil || total != nil || results[0]["url"] != "https://x.com/user%20name/status/1%2F2" || results[0]["title"] != "@user name on X" || pagination["next_cursor"] != "cursor" {
		t.Fatal(results, total, pagination, err)
	}
	method, _, headers, payload, err := buildAdditionalSearch("glm", "https://glm.example", "token", "query", "web", 5, nil)
	if err != nil || method != "POST" || headers["Authorization"] != "Bearer token" || payload["method"] != "tools/call" {
		t.Fatal(method, headers, payload, err)
	}
	var glm map[string]any
	json.Unmarshal([]byte(`{"result":{"content":[{"type":"text","text":"{\"results\":[{\"title\":\"Title\",\"link\":\"https://example.com\",\"content\":\"snippet\",\"source\":\"Source\"}]}"}]}}`), &glm)
	results, total, _, _, err = normalizeSearch("glm", "web", glm, 5, time.Now())
	if err != nil || total != 1 || results[0]["url"] != "https://example.com" || results[0]["metadata"].(map[string]any)["author"] != "Source" {
		t.Fatal(results, total, err)
	}
}
