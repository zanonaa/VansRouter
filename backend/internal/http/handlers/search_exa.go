package handlers

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

var exaSearchTypes = map[string]bool{"instant": true, "fast": true, "auto": true, "deep-lite": true, "deep": true, "deep-reasoning": true}
var exaCategories = map[string]bool{"company": true, "people": true, "publication": true, "news": true, "personal site": true, "financial report": true}
var exaSections = map[string]bool{"header": true, "navigation": true, "banner": true, "body": true, "sidebar": true, "footer": true, "metadata": true}
var exaCountry = regexp.MustCompile(`^[A-Za-z]{2}$`)

func exaOptions(body map[string]any) map[string]any {
	if options, ok := body["exa_options"].(map[string]any); ok {
		return options
	}
	if options, ok := body["exa"].(map[string]any); ok {
		return options
	}
	if options, ok := body["provider_options"].(map[string]any); ok {
		if nested, ok := options["exa"].(map[string]any); ok {
			return nested
		}
	}
	return body
}

func exaInvalid(name, message string) error { return fmt.Errorf("Exa %s: %s", name, message) }

func exaOptionalString(value any, name string, max int) (string, bool, error) {
	if value == nil {
		return "", false, nil
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return "", false, exaInvalid(name, "must be a non-empty string")
	}
	if max > 0 && len(text) > max {
		return "", false, exaInvalid(name, fmt.Sprintf("must be at most %d characters", max))
	}
	return text, true, nil
}

func exaInteger(value any, name string, min, max int) (int, error) {
	number, ok := value.(float64)
	if !ok || math.Trunc(number) != number || number < float64(min) || number > float64(max) {
		return 0, exaInvalid(name, fmt.Sprintf("must be an integer from %d to %d", min, max))
	}
	return int(number), nil
}

func exaBoolean(value any, name string) (bool, bool, error) {
	if value == nil {
		return false, false, nil
	}
	flag, ok := value.(bool)
	if !ok {
		return false, false, exaInvalid(name, "must be boolean")
	}
	return flag, true, nil
}

func exaDate(value any, name string) (string, bool, error) {
	text, present, err := exaOptionalString(value, name, 0)
	if err != nil || !present {
		return "", present, err
	}
	if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
		if _, err = time.Parse("2006-01-02", text); err != nil {
			return "", false, exaInvalid(name, "must be an ISO 8601 date")
		}
	}
	return text, true, nil
}

func exaStringList(value any, name string, minItems, maxItems, maxLength int) ([]any, bool, error) {
	if value == nil {
		return nil, false, nil
	}
	items, ok := value.([]any)
	if !ok || len(items) < minItems || len(items) > maxItems {
		return nil, false, exaInvalid(name, fmt.Sprintf("must contain %d-%d non-empty strings", minItems, maxItems))
	}
	for _, item := range items {
		text, ok := item.(string)
		if !ok || strings.TrimSpace(text) == "" || (maxLength > 0 && len(text) > maxLength) {
			return nil, false, exaInvalid(name, fmt.Sprintf("must contain %d-%d non-empty strings", minItems, maxItems))
		}
	}
	return items, true, nil
}

func exaDomainList(value any, name string) ([]any, bool, error) {
	items, present, err := exaStringList(value, name, 0, 1200, 0)
	if err != nil {
		return nil, false, exaInvalid(name, "must be an array of up to 1200 non-empty strings")
	}
	return items, present, nil
}

func validateExaSchema(schema any, name string) error {
	root, ok := schema.(map[string]any)
	if !ok {
		return exaInvalid(name, "must be an object")
	}
	if root["type"] != "text" && root["type"] != "object" {
		return exaInvalid(name, "type must be text or object")
	}
	count := 0
	var visit func(any, int) error
	visit = func(value any, depth int) error {
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		if depth > 2 {
			return exaInvalid(name, "nesting depth must be at most 2")
		}
		if properties, ok := object["properties"].(map[string]any); ok {
			count += len(properties)
			if count > 10 {
				return exaInvalid(name, "supports at most 10 properties")
			}
			for _, child := range properties {
				if err := visit(child, depth+1); err != nil {
					return err
				}
			}
		}
		if err := visit(object["items"], depth+1); err != nil {
			return err
		}
		if required, exists := object["required"]; exists && required != nil {
			items, ok := required.([]any)
			if !ok {
				return exaInvalid(name, "required must be an array of strings")
			}
			for _, item := range items {
				if _, ok := item.(string); !ok {
					return exaInvalid(name, "required must be an array of strings")
				}
			}
		}
		return nil
	}
	return visit(root, 0)
}

func buildExaContents(options map[string]any) (map[string]any, error) {
	contents := map[string]any{}
	for _, key := range []string{"text", "highlights", "summary", "context", "livecrawl", "livecrawlTimeout", "maxAgeHours", "subpages", "subpageTarget", "extras"} {
		if value, ok := options[key]; ok {
			contents[key] = value
		}
	}
	if value, ok := contents["text"]; ok && value != nil {
		if object, isObject := value.(map[string]any); isObject {
			if raw, exists := object["maxCharacters"]; exists {
				if _, err := exaInteger(raw, "contents.text.maxCharacters", 1, 10000); err != nil {
					return nil, err
				}
			}
			if _, _, err := exaBoolean(object["includeHtmlTags"], "contents.text.includeHtmlTags"); err != nil {
				return nil, err
			}
			if verbosity, exists := object["verbosity"]; exists && verbosity != "compact" && verbosity != "standard" && verbosity != "full" {
				return nil, exaInvalid("contents.text.verbosity", "is invalid")
			}
			for _, key := range []string{"includeSections", "excludeSections"} {
				if sections, exists := object[key]; exists && sections != nil {
					items, ok := sections.([]any)
					if !ok {
						return nil, exaInvalid("contents.text."+key, "contains an invalid section")
					}
					for _, section := range items {
						if !exaSections[ttsString(section)] {
							return nil, exaInvalid("contents.text."+key, "contains an invalid section")
						}
					}
				}
			}
		} else if _, ok := value.(bool); !ok {
			return nil, exaInvalid("contents.text", "must be boolean or object")
		}
	}
	if value, ok := contents["highlights"]; ok && value != nil {
		if object, isObject := value.(map[string]any); isObject {
			if _, _, err := exaOptionalString(object["query"], "contents.highlights.query", 0); err != nil {
				return nil, err
			}
			for _, field := range []struct{ key, name string }{{"maxCharacters", "contents.highlights.maxCharacters"}, {"numSentences", "contents.highlights.numSentences"}, {"highlightsPerUrl", "contents.highlights.highlightsPerUrl"}} {
				if raw, exists := object[field.key]; exists {
					if _, err := exaInteger(raw, field.name, 1, 10000); err != nil {
						return nil, err
					}
				}
			}
		} else if _, ok := value.(bool); !ok {
			return nil, exaInvalid("contents.highlights", "must be boolean or object")
		}
	}
	if value, ok := contents["summary"]; ok && value != nil {
		object, isObject := value.(map[string]any)
		if !isObject {
			return nil, exaInvalid("contents.summary", "must be an object")
		}
		if _, _, err := exaOptionalString(object["query"], "contents.summary.query", 0); err != nil {
			return nil, err
		}
		if schema, exists := object["schema"]; exists {
			if err := validateExaSchema(schema, "contents.summary.schema"); err != nil {
				return nil, err
			}
		}
	}
	for _, field := range []struct{ key, name string }{{"context", "contents.context"}, {"livecrawlTimeout", "contents.livecrawlTimeout"}, {"maxAgeHours", "contents.maxAgeHours"}, {"subpages", "contents.subpages"}} {
		value, exists := contents[field.key]
		if !exists || value == nil {
			continue
		}
		switch field.key {
		case "context":
			if object, ok := value.(map[string]any); ok {
				if raw, exists := object["maxCharacters"]; exists {
					if _, err := exaInteger(raw, field.name+".maxCharacters", 1, 10000); err != nil {
						return nil, err
					}
				}
			} else if _, ok := value.(bool); !ok {
				return nil, exaInvalid(field.name, "must be boolean or object")
			}
		case "livecrawlTimeout":
			if _, err := exaInteger(value, field.name, 1, 90000); err != nil {
				return nil, err
			}
		case "maxAgeHours":
			if _, err := exaInteger(value, field.name, -1, 720); err != nil {
				return nil, err
			}
		case "subpages":
			if _, err := exaInteger(value, field.name, 0, 100); err != nil {
				return nil, err
			}
		}
	}
	if value, exists := contents["livecrawl"]; exists && value != "never" && value != "always" && value != "fallback" && value != "preferred" {
		return nil, exaInvalid("contents.livecrawl", "is invalid")
	}
	if _, exists := contents["subpageTarget"]; exists {
		if text, ok := contents["subpageTarget"].(string); ok {
			if _, _, err := exaOptionalString(text, "contents.subpageTarget", 100); err != nil {
				return nil, err
			}
		} else if _, _, err := exaStringList(contents["subpageTarget"], "contents.subpageTarget", 1, 100, 100); err != nil {
			return nil, err
		}
	}
	if contents["livecrawl"] != nil && contents["maxAgeHours"] != nil {
		return nil, exaInvalid("contents", "livecrawl and maxAgeHours cannot be combined")
	}
	if extras, exists := contents["extras"]; exists {
		object, ok := extras.(map[string]any)
		if !ok {
			return nil, exaInvalid("contents.extras", "must be an object")
		}
		copied := map[string]any{}
		for key, value := range object {
			copied[key] = value
		}
		for _, key := range []string{"links", "imageLinks", "richImageLinks", "richLinks", "codeBlocks"} {
			if value, exists := copied[key]; exists {
				if _, err := exaInteger(value, "contents.extras."+key, 0, 1000); err != nil {
					return nil, err
				}
			}
		}
		contents["extras"] = copied
	}
	if len(contents) == 0 {
		return nil, nil
	}
	return contents, nil
}

func buildExaBody(query, kind string, max int, body map[string]any) (map[string]any, error) {
	options := map[string]any{}
	for key, value := range exaOptions(body) {
		options[key] = value
	}
	explicit, hasType := options["type"]
	searchType := "auto"
	if hasType {
		searchType = ttsString(explicit)
	} else if kind != "web" && kind != "news" && kind != "" {
		searchType = kind
	}
	if !exaSearchTypes[searchType] {
		return nil, exaInvalid("type", "is invalid")
	}
	if _, err := exaInteger(float64(max), "numResults", 1, 100); err != nil {
		return nil, err
	}
	payload := map[string]any{"query": query, "type": searchType, "numResults": max}
	if category, exists := options["category"]; exists {
		text, _, err := exaOptionalString(category, "category", 0)
		if err != nil {
			return nil, err
		}
		if !exaCategories[text] && len(text) > 100 {
			return nil, exaInvalid("category", "is too long")
		}
		payload["category"] = text
	} else if kind == "news" {
		payload["category"] = "news"
	}
	if location, exists := options["userLocation"]; exists {
		text, _, err := exaOptionalString(location, "userLocation", 0)
		if err != nil {
			return nil, err
		}
		if !exaCountry.MatchString(text) {
			return nil, exaInvalid("userLocation", "must be a two-letter ISO country code")
		}
		payload["userLocation"] = strings.ToUpper(text)
	}
	includes, excludes := searchDomains(body)
	includeValues, excludeValues := []any{}, []any{}
	for _, domain := range includes {
		includeValues = append(includeValues, domain)
	}
	for _, domain := range excludes {
		excludeValues = append(excludeValues, domain)
	}
	for _, field := range []struct {
		key   string
		value []any
	}{{"includeDomains", includeValues}, {"excludeDomains", excludeValues}} {
		source := any(field.value)
		if explicit, exists := options[field.key]; exists {
			source = explicit
		}
		list, present, err := exaDomainList(source, field.key)
		if err != nil {
			return nil, err
		}
		if present && len(list) > 0 {
			payload[field.key] = list
		}
	}
	for _, key := range []string{"startPublishedDate", "endPublishedDate", "startCrawlDate", "endCrawlDate"} {
		date, present, err := exaDate(options[key], key)
		if err != nil {
			return nil, err
		}
		if present {
			payload[key] = date
		}
	}
	if value, exists := options["context"]; exists {
		if object, ok := value.(map[string]any); ok {
			if raw, exists := object["maxCharacters"]; exists {
				if _, err := exaInteger(raw, "context.maxCharacters", 1, 10000); err != nil {
					return nil, err
				}
			}
		} else if _, ok := value.(bool); !ok {
			return nil, exaInvalid("context", "must be boolean or object")
		}
		payload["context"] = value
	}
	if moderation, present, err := exaBoolean(options["moderation"], "moderation"); err != nil {
		return nil, err
	} else if present {
		payload["moderation"] = moderation
	}
	queries, present, err := exaStringList(options["additionalQueries"], "additionalQueries", 1, 10, 0)
	if err != nil {
		return nil, err
	}
	if present {
		if !strings.HasPrefix(searchType, "deep") {
			return nil, exaInvalid("additionalQueries", "requires a deep search type")
		}
		payload["additionalQueries"] = queries
	}
	for _, key := range []string{"systemPrompt", "compliance"} {
		text, present, err := exaOptionalString(options[key], key, 0)
		if err != nil {
			return nil, err
		}
		if present {
			payload[key] = text
		}
	}
	if payload["compliance"] != nil && payload["compliance"] != "hipaa" {
		return nil, exaInvalid("compliance", "must be hipaa")
	}
	if schema, exists := options["outputSchema"]; exists {
		if err := validateExaSchema(schema, "outputSchema"); err != nil {
			return nil, err
		}
		payload["outputSchema"] = schema
	}
	stream, present, err := exaBoolean(options["stream"], "stream")
	if err != nil {
		return nil, err
	}
	if present {
		payload["stream"] = stream
		if stream && payload["outputSchema"] == nil {
			return nil, exaInvalid("stream", "requires outputSchema")
		}
	}
	contentsOptions := map[string]any{"highlights": true}
	if nested, ok := options["contents"].(map[string]any); ok {
		for key, value := range nested {
			contentsOptions[key] = value
		}
	}
	for _, key := range []string{"text", "highlights", "summary", "context", "livecrawl", "livecrawlTimeout", "maxAgeHours", "subpages", "subpageTarget", "extras"} {
		if value, exists := options["contents."+key]; exists {
			contentsOptions[key] = value
		}
	}
	contents, err := buildExaContents(contentsOptions)
	if err != nil {
		return nil, err
	}
	if contents != nil {
		payload["contents"] = contents
	}
	if (payload["category"] == "company" || payload["category"] == "people") && (payload["excludeDomains"] != nil || payload["startPublishedDate"] != nil || payload["endPublishedDate"] != nil) {
		return nil, exaInvalid("category", "does not support excludeDomains or publication dates")
	}
	contentMap, _ := payload["contents"].(map[string]any)
	if payload["compliance"] == "hipaa" && (searchType != "instant" && searchType != "fast" || contentMap["maxAgeHours"] != float64(-1) || contentMap["summary"] != nil || contentMap["livecrawl"] != nil) {
		return nil, exaInvalid("compliance", "hipaa requires instant/fast, cache-only content, and no summary/livecrawl")
	}
	return payload, nil
}
