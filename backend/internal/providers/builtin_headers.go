package providers

import (
	_ "embed"
	"encoding/json"
)

//go:embed builtin-headers.json
var builtinHeaders []byte

func BuiltinHeaders(id string) map[string]any {
	var all map[string]map[string]any
	if err := json.Unmarshal(builtinHeaders, &all); err != nil {
		panic(err)
	}
	if value := all[id]; value != nil {
		return value
	}
	return map[string]any{}
}
func ResolveAlias(id string) string {
	for _, entry := range Catalog() {
		if entry.ID == id {
			return id
		}
		if entry.Alias != nil && *entry.Alias == id {
			return entry.ID
		}
		for _, alias := range entry.Aliases {
			if alias == id {
				return entry.ID
			}
		}
	}
	return id
}
