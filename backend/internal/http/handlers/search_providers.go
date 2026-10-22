package handlers

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

func searchSetting(body map[string]any, key string) string {
	options, _ := body["provider_options"].(map[string]any)
	if value := strings.TrimSpace(ttsString(options[key])); value != "" {
		return value
	}
	metadata, _ := body["_providerSpecificData"].(map[string]any)
	return strings.TrimSpace(ttsString(metadata[key]))
}
func searchDomains(body map[string]any) ([]string, []string) {
	includes, excludes := []string{}, []string{}
	entries, _ := body["domain_filter"].([]any)
	for _, raw := range entries {
		domain := ttsString(raw)
		if strings.HasPrefix(domain, "-") {
			excludes = append(excludes, strings.TrimPrefix(domain, "-"))
		} else {
			includes = append(includes, domain)
		}
	}
	return includes, excludes
}
func buildAdditionalSearch(provider, base, token, query, kind string, maxResults int, body map[string]any) (string, string, map[string]string, map[string]any, error) {
	headers := map[string]string{"Accept": "application/json"}
	values := url.Values{}
	includes, excludes := searchDomains(body)
	country, language, timeRange := ttsString(body["country"]), ttsString(body["language"]), ttsString(body["time_range"])
	offset := int(pxpipeNum(body["offset"]))
	switch provider {
	case "google-pse":
		cx := searchSetting(body, "cx")
		if token == "" || cx == "" {
			return "", "", nil, nil, fmt.Errorf("Google Programmable Search requires both apiKey and cx")
		}
		values = url.Values{"key": {token}, "cx": {cx}, "q": {query}, "num": {fmt.Sprint(min(maxResults, 10))}}
		if country != "" {
			values.Set("gl", strings.ToLower(country))
		}
		if language != "" {
			values.Set("hl", language)
		}
		if period := map[string]string{"day": "d1", "week": "w1", "month": "m1", "year": "y1"}[timeRange]; period != "" {
			values.Set("dateRestrict", period)
		}
		if offset > 0 {
			values.Set("start", fmt.Sprint(min(offset+1, 91)))
		}
	case "searchapi":
		if token == "" {
			return "", "", nil, nil, fmt.Errorf("SearchAPI requires an API key")
		}
		engine := "google"
		if kind == "news" {
			engine = "google_news"
		}
		values = url.Values{"engine": {engine}, "q": {query}, "api_key": {token}}
		if country != "" {
			values.Set("gl", strings.ToLower(country))
		}
		if language != "" {
			values.Set("hl", language)
		}
		if offset > 0 && maxResults > 0 {
			values.Set("page", fmt.Sprint(offset/maxResults+1))
		}
	case "youcom":
		if token == "" {
			return "", "", nil, nil, fmt.Errorf("You.com Search requires an API key")
		}
		values = url.Values{"query": {query}, "count": {fmt.Sprint(min(maxResults, 100))}}
		headers["X-API-Key"] = token
		if timeRange != "" && timeRange != "any" {
			values.Set("freshness", timeRange)
		}
		if offset > 0 && maxResults > 0 {
			values.Set("offset", fmt.Sprint(min(offset/maxResults, 9)))
		}
		if country != "" {
			values.Set("country", country)
		}
		if language != "" {
			values.Set("language", language)
		}
		if len(includes) > 0 {
			values.Set("include_domains", strings.Join(includes, ","))
		}
		if len(excludes) > 0 {
			values.Set("exclude_domains", strings.Join(excludes, ","))
		}
		options, _ := body["content_options"].(map[string]any)
		if nodeTruthy(options["full_page"]) {
			crawl := "web"
			if kind == "news" {
				crawl = "news"
			}
			values.Set("livecrawl", crawl)
			format := "html"
			if options["format"] == "markdown" {
				format = "markdown"
			}
			values.Set("livecrawl_formats", format)
		}
	case "tinyfish":
		if kind != "" && kind != "web" && kind != "news" && kind != "research_paper" {
			return "", "", nil, nil, fmt.Errorf("Unsupported TinyFish search type")
		}
		values.Set("query", query)
		if kind != "" && kind != "web" {
			values.Set("domain_type", kind)
		}
		if country != "" {
			values.Set("location", country)
		}
		if language != "" {
			values.Set("language", language)
		}
		if len(includes) > 0 {
			values.Set("include_domains", strings.Join(includes, ","))
		}
		if len(excludes) > 0 {
			values.Set("exclude_domains", strings.Join(excludes, ","))
		}
		if offset > 0 {
			if offset >= 110 {
				return "", "", nil, nil, fmt.Errorf("TinyFish search offset exceeds available pages")
			}
			if offset%10+maxResults > 10 {
				return "", "", nil, nil, fmt.Errorf("TinyFish search offset and max_results must fit within one page")
			}
			values.Set("page", fmt.Sprint(offset/10))
		}
		headers["X-API-Key"] = token
	case "xquik":
		if token == "" {
			return "", "", nil, nil, fmt.Errorf("Xquik requires an API key")
		}
		queryType := searchSetting(body, "queryType")
		if queryType != "" && queryType != "Latest" && queryType != "Top" {
			return "", "", nil, nil, fmt.Errorf("Xquik queryType must be Latest or Top")
		}
		values = url.Values{"q": {query}, "limit": {fmt.Sprint(maxResults)}}
		if cursor := searchSetting(body, "cursor"); cursor != "" {
			values.Set("cursor", cursor)
		}
		if queryType != "" {
			values.Set("queryType", queryType)
		}
		if language != "" {
			values.Set("language", language)
		}
		headers["x-api-key"] = token
	case "ollama-search":
		payload := map[string]any{"query": query, "max_results": maxResults}
		if country != "" {
			payload["country"] = country
		}
		if language != "" {
			payload["language"] = language
		}
		headers = map[string]string{"Content-Type": "application/json"}
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
		return "POST", base, headers, payload, nil
	case "glm":
		headers = map[string]string{"Content-Type": "application/json"}
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
		return "POST", base, headers, map[string]any{"jsonrpc": "2.0", "id": "9r-search", "method": "tools/call", "params": map[string]any{"name": "web_search_prime", "arguments": map[string]any{"search_query": query, "count": maxResults}}}, nil
	case "linkup":
		if token == "" {
			return "", "", nil, nil, fmt.Errorf("Linkup Search requires an API key")
		}
		depth := searchSetting(body, "depth")
		if depth != "fast" && depth != "deep" && depth != "standard" {
			depth = "standard"
		}
		payload := map[string]any{"q": query, "depth": depth, "outputType": "searchResults", "maxResults": maxResults}
		if len(includes) > 0 {
			payload["includeDomains"] = includes
		}
		if len(excludes) > 0 {
			payload["excludeDomains"] = excludes
		}
		if timeRange != "" && timeRange != "any" {
			now := time.Now().UTC()
			from := now
			switch timeRange {
			case "day":
				from = now.AddDate(0, 0, -1)
			case "week":
				from = now.AddDate(0, 0, -7)
			case "month":
				from = now.AddDate(0, -1, 0)
			case "year":
				from = now.AddDate(-1, 0, 0)
			}
			payload["fromDate"] = from.Format("2006-01-02")
			payload["toDate"] = now.Format("2006-01-02")
		}
		return "POST", base, map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + token}, payload, nil
	default:
		return "", "", nil, nil, fmt.Errorf("Search request builder not implemented for provider: %s", provider)
	}
	return "GET", base + "?" + values.Encode(), headers, nil, nil
}
