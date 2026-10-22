package handlers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

func normalizeSearch(provider, kind string, data map[string]any, max int, now time.Time) ([]map[string]any, any, map[string]any, map[string]any, error) {
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
	case "tavily", "searxng", "linkup", "exa":
		items, _ = data["results"].([]any)
		total = len(items)
	case "tinyfish":
		items, _ = data["results"].([]any)
		total = data["total_results"]
	case "google-pse":
		items, _ = data["items"].([]any)
		info, _ := data["searchInformation"].(map[string]any)
		if info["totalResults"] != nil {
			total = usageNumber(info["totalResults"])
		}
	case "searchapi":
		items, _ = data["organic_results"].([]any)
		if items == nil {
			items, _ = data["top_stories"].([]any)
		}
		info, _ := data["search_information"].(map[string]any)
		total = len(items)
		if info["total_results"] != nil {
			total = usageNumber(info["total_results"])
		}
	case "youcom":
		container, _ := data["results"].(map[string]any)
		field := "web"
		if kind == "news" {
			field = "news"
		}
		items, _ = container[field].([]any)
		total = len(items)
	case "ollama-search":
		items, _ = data["results"].([]any)
		total = len(items)
	case "xquik":
		items, _ = data["tweets"].([]any)
	case "glm":
		payload := data
		content, _ := data["result"].(map[string]any)
		parts, _ := content["content"].([]any)
		if len(parts) > 0 {
			part, _ := parts[0].(map[string]any)
			if text, ok := part["text"].(string); ok {
				var decoded any
				if json.Unmarshal([]byte(text), &decoded) == nil {
					switch value := decoded.(type) {
					case map[string]any:
						payload = value
					case []any:
						items = value
					}
				}
			}
		}
		if items == nil {
			items, _ = payload["results"].([]any)
			if items == nil {
				items, _ = payload["items"].([]any)
			}
		}
		total = len(items)
	default:
		return nil, nil, nil, nil, fmt.Errorf("Search normalizer not implemented for provider: %s", provider)
	}
	results := []map[string]any{}
	for index, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, nil, nil, fmt.Errorf("Invalid search result")
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
		case "exa":
			target = ttsString(item["url"])
			if highlights, ok := item["highlights"].([]any); ok && len(highlights) > 0 {
				snippet = ttsString(highlights[0])
			}
			if snippet == "" {
				text := ttsString(item["text"])
				if len([]rune(text)) > 300 {
					text = string([]rune(text)[:300])
				}
				snippet = text
			}
			published = item["publishedDate"]
			favicon = item["favicon"]
			image = item["image"]
			fullText = ttsString(item["text"])
		case "google-pse", "searchapi":
			target = ttsString(item["link"])
			snippet = ttsString(firstNonEmpty(item["snippet"], item["description"]))
			published = firstNonEmpty(item["date"], item["published_at"])
			favicon = item["favicon"]
			image = item["thumbnail"]
		case "linkup":
			target = ttsString(item["url"])
			snippet = ttsString(firstNonEmpty(item["content"], item["snippet"]))
			fullText = ttsString(item["content"])
			source = firstNonEmpty(item["type"], "web")
			image = firstNonEmpty(item["image_url"], item["imageUrl"])
		case "tinyfish":
			target = ttsString(item["url"])
			snippet = ttsString(item["snippet"])
			published = item["date"]
			source = item["publisher"]
		case "youcom":
			target = ttsString(item["url"])
			snippet = ttsString(item["description"])
			if snippets, ok := item["snippets"].([]any); ok {
				for _, raw := range snippets {
					if value, ok := raw.(string); ok {
						snippet = value
						break
					}
				}
			}
			fullText = ttsString(firstNonEmpty(item["markdown"], item["html"]))
			published = item["page_age"]
			favicon = item["favicon_url"]
			image = item["thumbnail_url"]
			source = kind
		case "ollama-search":
			target = ttsString(item["url"])
			snippet = ttsString(item["content"])
			fullText = snippet
			published = item["published_at"]
		case "glm":
			target = ttsString(firstNonEmpty(item["link"], item["url"]))
			snippet = ttsString(item["content"])
			fullText = snippet
			published = firstNonEmpty(item["publish_time"], item["published_at"])
			image = item["icon"]
			source = "mcp_web_search_prime"
		case "xquik":
			author, _ := item["author"].(map[string]any)
			username, tweetID := ttsString(author["username"]), ttsString(item["id"])
			if username != "" && tweetID != "" {
				target = "https://x.com/" + url.PathEscape(username) + "/status/" + url.PathEscape(tweetID)
			} else if tweetID != "" {
				target = "https://x.com/i/web/status/" + url.PathEscape(tweetID)
			}
			label := ""
			if username != "" {
				label = "@" + username
			} else {
				label = ttsString(author["name"])
			}
			item["title"] = "X post"
			if label != "" {
				item["title"] = label + " on X"
			}
			snippet = ttsString(item["text"])
			fullText = snippet
			published = item["createdAt"]
			source = "x_post"
			if media, ok := item["media"].([]any); ok {
				for _, raw := range media {
					entry, _ := raw.(map[string]any)
					if value, ok := entry["mediaUrl"].(string); ok {
						image = value
						break
					}
				}
			}
			item["_xquikAuthor"] = label
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
		if value, ok := item["score"].(float64); ok && (provider == "tavily" || provider == "exa") {
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
			format := "text"
			if provider == "youcom" {
				format = "html"
				if _, ok := item["markdown"].(string); ok {
					format = "markdown"
				}
			}
			content = map[string]any{"format": format, "text": fullText, "length": jsTextLength(fullText)}
		}
		title := ttsString(item["title"])
		if provider == "linkup" {
			title = ttsString(firstNonEmpty(item["name"], item["title"]))
		}
		result := map[string]any{"title": title, "url": target, "snippet": snippet, "position": index + 1, "score": score, "published_at": published, "favicon_url": favicon, "content": content, "metadata": map[string]any{"author": nil, "language": nil, "source_type": source, "image_url": image}, "citation": map[string]any{"provider": provider, "retrieved_at": now.UTC().Format("2006-01-02T15:04:05.000Z"), "rank": index + 1}, "provider_raw": nil}
		metadata := result["metadata"].(map[string]any)
		if provider == "exa" {
			metadata["author"] = item["author"]
			result["provider_raw"] = map[string]any{"id": item["id"], "publishedDate": item["publishedDate"], "author": item["author"], "image": item["image"], "favicon": item["favicon"], "highlightScores": item["highlightScores"], "summary": item["summary"], "subpages": item["subpages"], "extras": item["extras"]}
			for key, value := range result["provider_raw"].(map[string]any) {
				if value == nil {
					result["provider_raw"].(map[string]any)[key] = nil
				}
			}
		}
		if provider == "tinyfish" {
			if authors, ok := item["authors"].([]any); ok {
				names := []string{}
				for _, name := range authors {
					names = append(names, ttsString(name))
				}
				metadata["author"] = strings.Join(names, ", ")
			}
		}
		if provider == "xquik" {
			if label := ttsString(item["_xquikAuthor"]); label != "" {
				metadata["author"] = label
			}
		}
		if provider == "glm" {
			metadata["author"] = item["source"]
		}
		if provider == "searchapi" {
			metadata["author"] = item["source"]
		}
		if provider == "google-pse" {
			pagemap, _ := item["pagemap"].(map[string]any)
			for _, key := range []string{"cse_image", "cse_thumbnail", "metatags"} {
				entries, _ := pagemap[key].([]any)
				if len(entries) > 0 {
					entry, _ := entries[0].(map[string]any)
					value := entry["src"]
					if key == "metatags" {
						value = entry["og:image"]
					}
					if nodeTruthy(value) {
						metadata["image_url"] = value
						break
					}
				}
			}
		}
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
	var metadata map[string]any
	if provider == "exa" {
		metadata = map[string]any{"requestId": data["requestId"], "resolvedSearchType": firstNonEmpty(data["resolvedSearchType"], data["searchType"]), "costDollars": data["costDollars"], "output": data["output"]}
		if items == nil {
			results = []map[string]any{}
			total = nil
		}
	}
	var pagination map[string]any
	if provider == "xquik" {
		cursor := ttsString(data["next_cursor"])
		var next any
		if cursor != "" {
			next = cursor
		}
		pagination = map[string]any{"has_more": data["has_next_page"] == true, "next_cursor": next}
	}
	return results, total, metadata, pagination, nil
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
	case "exa":
		payload, err := buildExaBody(query, kind, max, body)
		if err != nil {
			return "", "", nil, nil, err
		}
		return "POST", base, map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + token}, payload, nil
	default:
		return buildAdditionalSearch(provider, base, token, query, kind, max, body)
	}
}
