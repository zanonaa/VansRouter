package handlers

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

func normalizeSearch(provider, kind string, data map[string]any, max int, now time.Time) ([]map[string]any, any, error) {
	var items []any
	var total any
	switch provider {
	case "serper":
		field := "organic"
		if kind == "news" {
			field = "news"
		}
		items, _ = data[field].([]any)
		params, _ := data["searchParameters"].(map[string]any)
		if number, ok := params["totalResults"].(float64); ok {
			total = number
		}
	case "brave-search":
		field := "web"
		if kind == "news" {
			field = "news"
		}
		container, _ := data[field].(map[string]any)
		if container == nil && kind == "news" {
			container = data
		}
		items, _ = container["results"].([]any)
		total = container["totalCount"]
	case "tavily", "searxng":
		items, _ = data["results"].([]any)
		total = len(items)
	default:
		return nil, nil, fmt.Errorf("Search normalizer not implemented for provider: %s", provider)
	}
	results := []map[string]any{}
	for index, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("Invalid search result")
		}
		target, snippet, published, favicon, fullText, source, image := "", "", any(nil), any(nil), "", any(nil), any(nil)
		switch provider {
		case "serper":
			target = ttsString(item["link"])
			snippet = ttsString(firstNonEmpty(item["snippet"], item["description"]))
			published = item["date"]
		case "brave-search":
			target = ttsString(item["url"])
			snippet = ttsString(item["description"])
			published = firstNonEmpty(item["page_age"], item["age"])
			meta, _ := item["meta_url"].(map[string]any)
			favicon = firstNonEmpty(meta["favicon"], item["favicon"])
		case "tavily":
			target = ttsString(item["url"])
			snippet = ttsString(item["content"])
			published = item["published_date"]
			fullText = ttsString(item["raw_content"])
		case "searxng":
			target = ttsString(item["url"])
			snippet = ttsString(firstNonEmpty(item["content"], item["snippet"]))
			published = firstNonEmpty(item["publishedDate"], item["published_date"])
			source = firstNonEmpty(item["engine"], item["category"])
			if engines, ok := item["engines"].([]any); ok {
				names := []string{}
				for _, name := range engines {
					names = append(names, ttsString(name))
				}
				source = strings.Join(names, ", ")
			}
			image = firstNonEmpty(item["thumbnail"], item["img_src"])
		}
		var score any
		if value, ok := item["score"].(float64); ok && provider == "tavily" {
			if value < 0 {
				value = 0
			}
			if value > 1 {
				value = 1
			}
			score = value
		}
		var content any
		if fullText != "" {
			content = map[string]any{"format": "text", "text": fullText, "length": jsTextLength(fullText)}
		}
		result := map[string]any{"title": ttsString(item["title"]), "url": target, "snippet": snippet, "position": index + 1, "score": score, "published_at": published, "favicon_url": favicon, "content": content, "metadata": map[string]any{"author": nil, "language": nil, "source_type": source, "image_url": image}, "citation": map[string]any{"provider": provider, "retrieved_at": now.UTC().Format("2006-01-02T15:04:05.000Z"), "rank": index + 1}, "provider_raw": nil}
		if target != "" {
			display := strings.TrimPrefix(strings.TrimPrefix(target, "https://"), "http://")
			display = strings.TrimPrefix(display, "www.")
			display = strings.SplitN(display, "?", 2)[0]
			result["display_url"] = display
		}
		results = append(results, result)
	}
	if max > 0 && len(results) > max {
		results = results[:max]
	}
	return results, total, nil
}

func buildBasicSearch(provider, base, token, query, kind string, max int, body map[string]any) (string, string, map[string]string, map[string]any, error) {
	headers := map[string]string{"Content-Type": "application/json"}
	payload := map[string]any{}
	switch provider {
	case "serper":
		path := "/search"
		if kind == "news" {
			path = "/news"
		}
		payload = map[string]any{"q": query, "num": max}
		if value := ttsString(body["country"]); value != "" {
			payload["gl"] = strings.ToLower(value)
		}
		if value := ttsString(body["language"]); value != "" {
			payload["hl"] = value
		}
		headers["X-API-Key"] = token
		return "POST", base + path, headers, payload, nil
	case "brave-search":
		path := "/web/search"
		if kind == "news" {
			path = "/news/search"
		}
		values := url.Values{"q": {query}, "count": {fmt.Sprint(max)}}
		if value := ttsString(body["country"]); value != "" {
			values.Set("country", value)
		}
		if value := ttsString(body["language"]); value != "" {
			values.Set("search_lang", value)
		}
		return "GET", base + path + "?" + values.Encode(), map[string]string{"Accept": "application/json", "X-Subscription-Token": token}, nil, nil
	case "tavily":
		topic := "general"
		if kind == "news" {
			topic = "news"
		}
		payload = map[string]any{"query": query, "max_results": max, "topic": topic}
		includes, excludes := []any{}, []any{}
		domains, _ := body["domain_filter"].([]any)
		for _, raw := range domains {
			domain := ttsString(raw)
			if strings.HasPrefix(domain, "-") {
				excludes = append(excludes, strings.TrimPrefix(domain, "-"))
			} else if domain != "" {
				includes = append(includes, domain)
			}
		}
		if len(includes) > 0 {
			payload["include_domains"] = includes
		}
		if len(excludes) > 0 {
			payload["exclude_domains"] = excludes
		}
		if value := ttsString(body["country"]); value != "" {
			payload["country"] = value
		}
		headers["Authorization"] = "Bearer " + token
		return "POST", base, headers, payload, nil
	case "searxng":
		values := url.Values{"q": {query}, "format": {"json"}}
		return "GET", base + "?" + values.Encode(), map[string]string{}, nil, nil
	default:
		return "", "", nil, nil, fmt.Errorf("Search request builder not implemented for provider: %s", provider)
	}
}
