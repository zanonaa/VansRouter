package providers

import (
	_ "embed"
	"encoding/json"
)

//go:embed transports.json
var transportsJSON []byte

type Transport struct {
	BaseURL string            `json:"baseUrl"`
	Format  string            `json:"format"`
	Headers map[string]string `json:"headers"`
	Auth    *struct {
		Header string `json:"header"`
		Scheme string `json:"scheme"`
	} `json:"auth"`
	NoAuth bool `json:"noAuth"`
}

func OpenAITransport(id string) *Transport {
	var entries map[string]Transport
	if err := json.Unmarshal(transportsJSON, &entries); err != nil {
		panic(err)
	}
	entry, ok := entries[id]
	if !ok {
		return nil
	}
	return &entry
}
