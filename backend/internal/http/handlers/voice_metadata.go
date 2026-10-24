package handlers

import (
	"context"
	_ "embed"
	"encoding/json"
)

//go:embed voice-metadata.json
var voiceMetadataJSON []byte
var voiceMetadata = func() struct {
	Languages, Regions map[string]string
	Gemini             []map[string]any
} {
	var result struct {
		Languages, Regions map[string]string
		Gemini             []map[string]any
	}
	if err := json.Unmarshal(voiceMetadataJSON, &result); err != nil {
		panic(err)
	}
	return result
}()

func voiceLanguage(code string) string {
	if value, ok := voiceMetadata.Languages[code]; ok {
		return value
	}
	return code
}
func voiceRegion(code string) string {
	if value, ok := voiceMetadata.Regions[code]; ok {
		return value
	}
	return code
}
func FetchGeminiVoicesPublic(context.Context, string) ([]map[string]any, error) {
	// Cópia profunda impede que consumidores alterem o snapshot compartilhado.
	var result struct{ Gemini []map[string]any }
	if err := json.Unmarshal(voiceMetadataJSON, &result); err != nil {
		return nil, err
	}
	return result.Gemini, nil
}
