package handlers

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"time"
)

func oauthPlatform() int {
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return 2
		}
		return 1
	case "linux":
		if runtime.GOARCH == "arm64" {
			return 4
		}
		return 3
	case "windows":
		return 5
	}
	return 0
}

// Não depende do contexto HTTP, pois a origem faz onboarding após responder.
// Prazo global evita goroutines penduradas indefinidamente.
func (h *OAuthEngine) antigravityOnboard(access, tier, target, userAgent string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	payload, _ := json.Marshal(map[string]any{"tierId": tier, "metadata": map[string]int{"ideType": 9, "platform": oauthPlatform(), "pluginType": 2}})
	headers := map[string]string{"Authorization": "Bearer " + access, "Content-Type": "application/json", "User-Agent": userAgent, "x-request-source": "local"}
	for i := 0; i < 10; i++ {
		status, data, _, err := h.deviceJSON(ctx, "POST", target, headers, strings.NewReader(string(payload)))
		if err != nil {
			return
		}
		if status >= 200 && status < 300 && data["done"] == true {
			return
		}
		if err = h.museWait(ctx, 5*time.Second); err != nil {
			return
		}
	}
}
