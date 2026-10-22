package providers

import (
	_ "embed"
	"encoding/json"
)

//go:embed model-info.json
var modelInfoJSON []byte

func ModelInfo() []map[string]any {
	var entries []map[string]any
	if err := json.Unmarshal(modelInfoJSON, &entries); err != nil {
		panic(err)
	}
	return entries
}
