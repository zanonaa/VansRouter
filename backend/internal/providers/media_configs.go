package providers

import (
	_ "embed"
	"encoding/json"
)

//go:embed media-configs.json
var mediaConfigsJSON []byte

type MediaConfig struct {
	BaseURL    string `json:"baseUrl"`
	AuthType   string `json:"authType"`
	AuthHeader string `json:"authHeader"`
	Format     string `json:"format"`
}

type MediaProvider struct {
	TTS *MediaConfig `json:"tts"`
	STT *MediaConfig `json:"stt"`
}

func MediaConfigs() map[string]MediaProvider {
	var configs map[string]MediaProvider
	if err := json.Unmarshal(mediaConfigsJSON, &configs); err != nil {
		panic(err)
	}
	return configs
}
