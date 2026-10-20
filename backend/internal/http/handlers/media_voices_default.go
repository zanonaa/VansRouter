package handlers

import (
	"context"
	"net/http"
	"time"
)

var mediaVoiceClient = &http.Client{Timeout: 10 * time.Second}

func FetchEdgeVoicesPublic(ctx context.Context) ([]map[string]any, error) {
	return fetchEdgeVoices(ctx, mediaVoiceClient)
}

func FetchElevenVoicesPublic(ctx context.Context, apiKey string) ([]map[string]any, error) {
	return fetchElevenVoices(ctx, mediaVoiceClient, apiKey)
}
