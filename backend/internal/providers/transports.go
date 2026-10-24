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
		APIKey *struct {
			Header string `json:"header"`
			Scheme string `json:"scheme"`
		} `json:"apiKey"`
		OAuth *struct {
			Header string `json:"header"`
			Scheme string `json:"scheme"`
		} `json:"oauth"`
	} `json:"auth"`
	NoAuth bool `json:"noAuth"`
}

func TransportFor(id string) *Transport {
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

// OpenAITransport keeps its original format restriction. A registry URL alone
// does not mean a provider accepts the OpenAI wire protocol.
func OpenAITransport(id string) *Transport {
	entry := TransportFor(id)
	if entry == nil || entry.Format != "openai" {
		return nil
	}
	return entry
}
