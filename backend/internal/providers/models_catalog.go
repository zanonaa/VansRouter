package providers

import (
	_ "embed"
	"encoding/json"
)

//go:embed models-catalog.json
var modelsCatalogJSON []byte

type ModelsCatalog struct {
	Models      []map[string]any `json:"models"`
	Passthrough []string         `json:"passthrough"`
}

func CatalogModels() ModelsCatalog {
	var c ModelsCatalog
	if err := json.Unmarshal(modelsCatalogJSON, &c); err != nil {
		panic(err)
	}
	return c
}
