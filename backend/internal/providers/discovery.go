package providers

import (
	_ "embed"
	"encoding/json"
)

//go:embed discovery.json
var discoveryJSON []byte

type DiscoveryConfig struct {
	URL        string `json:"url"`
	Public     bool   `json:"public"`
	AuthHeader string `json:"authHeader"`
	AuthPrefix string `json:"authPrefix"`
}

func Discovery(id string) *DiscoveryConfig {
	var entries map[string]DiscoveryConfig
	if err := json.Unmarshal(discoveryJSON, &entries); err != nil {
		panic(err)
	}
	config, ok := entries[id]
	if !ok {
		return nil
	}
	return &config
}
