// Package providers contém metadados públicos derivados do registry original.
package providers

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
)

//go:embed catalog.json
var catalogJSON []byte

type Entry struct {
	ID           string   `json:"id"`
	Category     string   `json:"category"`
	Alias        *string  `json:"alias"`
	Aliases      []string `json:"aliases"`
	Name         string   `json:"name"`
	DisplayName  string   `json:"displayName"`
	NoAuth       bool     `json:"noAuth"`
	ServiceKinds []string `json:"serviceKinds"`
	AuthModes    []string `json:"authModes"`
}

// Catalog devolve uma cópia: consumidores não podem corromper o snapshot global.
func Catalog() []Entry {
	var entries []Entry
	if err := json.Unmarshal(catalogJSON, &entries); err != nil {
		panic(err)
	}
	return entries
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func NormalizeID(value string) string {
	trimmed := strings.TrimSpace(value)
	entries := Catalog()
	for _, entry := range entries {
		if entry.ID == trimmed {
			return trimmed
		}
	}
	slug := strings.Trim(slugPattern.ReplaceAllString(strings.ToLower(trimmed), "-"), "-")
	for _, entry := range entries {
		if entry.ID == slug {
			return slug
		}
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name, trimmed) {
			return entry.ID
		}
	}
	return trimmed
}
func AliasMap() map[string]string {
	result := map[string]string{}
	for _, entry := range Catalog() {
		if entry.Alias != nil && *entry.Alias != entry.ID {
			result[*entry.Alias] = entry.ID
		}
		for _, alias := range entry.Aliases {
			result[alias] = entry.ID
		}
	}
	return result
}
func (entry Entry) AcceptsAPIKey() bool {
	if entry.Category == "apikey" || entry.Category == "freeTier" || entry.Category == "webCookie" {
		return true
	}
	for _, mode := range entry.AuthModes {
		if mode == "apikey" {
			return true
		}
	}
	return false
}
