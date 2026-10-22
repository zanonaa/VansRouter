package handlers

import "testing"

func TestFetchNormalizesProviderContent(t *testing.T) {
	for _, test := range []struct{ id, payload, text, title string }{
		{"firecrawl", `{"data":{"markdown":"extracted content","metadata":{"title":"Page"}}}`, "extracted content", "Page"},
		{"tavily", `{"results":[{"raw_content":"tavily text"}]}`, "tavily text", ""},
		{"exa", `{"results":[{"text":"exa text","title":"Exa"}]}`, "exa text", "Exa"},
		{"tinyfish", `{"results":[{"text":"tiny text","title":"Tiny","author":"Author","links":["https://example.com"]}]}`, "tiny text", "Tiny"},
		{"jina-reader", "Title: Jina\n\n# heading\nbody", "Title: Jina\n\n# heading\nbody", "Jina"},
	} {
		t.Run(test.id, func(t *testing.T) {
			out, err := normalizeFetch(test.id, "https://example.com", "markdown", []byte(test.payload), 0, nil, 10)
			if err != nil {
				t.Fatal(err)
			}
			content := out["content"].(map[string]any)
			if content["text"] != test.text || content["length"] != jsTextLength(test.text) {
				t.Fatal(out)
			}
			if test.title != "" && out["title"] != test.title {
				t.Fatal(out)
			}
			if out["metadata"] == nil || out["usage"] == nil || out["metrics"] == nil {
				t.Fatal("missing shape")
			}
		})
	}
}
func TestFetchTruncatesAndRejectsInvalidPayload(t *testing.T) {
	out, err := normalizeFetch("exa", "https://example.com", "markdown", []byte(`{"results":[{"text":"abcdef"}]}`), 3, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if out["content"].(map[string]any)["text"] != "abc" {
		t.Fatal(out)
	}
	if _, err := normalizeFetch("tinyfish", "url", "markdown", []byte(`{"results":[]}`), 0, nil, 0); err == nil {
		t.Fatal("empty content accepted")
	}
	if jsTextLength("a😀") != 3 {
		t.Fatal("JS length mismatch")
	}
}
