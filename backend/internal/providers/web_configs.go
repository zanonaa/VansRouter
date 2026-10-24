package providers

import (
	_ "embed"
	"encoding/json"
)

//go:embed web-configs.json
var webConfigsJSON []byte

type WebConfig struct {
	Alias  string         `json:"alias"`
	NoAuth bool           `json:"noAuth"`
	Search map[string]any `json:"search"`
	Fetch  map[string]any `json:"fetch"`
	Video  map[string]any `json:"video"`
}

func WebConfigs() map[string]WebConfig {
	var configs map[string]WebConfig
	if err := json.Unmarshal(webConfigsJSON, &configs); err != nil {
		panic(err)
	}
	return configs
}
