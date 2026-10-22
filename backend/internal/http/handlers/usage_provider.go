package handlers

import (
	"fmt"
	"net/http"
	"strings"
)

func (u *UsageContracts) providerUsage(w http.ResponseWriter, r *http.Request, connection map[string]any) {
	provider := ttsString(connection["provider"])
	token := ttsString(firstNonEmpty(connection["apiKey"], connection["accessToken"]))
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	switch provider {
	case "claude":
		nodeReply(w, 200, u.claudeUsage(r, ttsString(connection["accessToken"])))
	case "codex":
		status, data, err := u.resetJSON(r.Context(), "GET", "https://chatgpt.com/backend-api/wham/usage", map[string]string{"Authorization": "Bearer " + ttsString(connection["accessToken"]), "Accept": "application/json"}, nil)
		if err != nil {
			nodeReply(w, 500, map[string]any{"error": "Failed to fetch Codex usage: " + err.Error()})
			return
		}
		if status < 200 || status >= 300 {
			nodeReply(w, 200, map[string]any{"message": fmt.Sprintf("Codex connected. Usage API temporarily unavailable (%d).", status)})
			return
		}
		nodeReply(w, 200, normalizeCodexUsage(data))
	case "qwen":
		message := "Qwen connected. Usage tracked per request."
		if strings.TrimSpace(ttsString(metadata["resourceUrl"])) == "" {
			message = "Qwen connected. No resource URL available."
		}
		nodeReply(w, 200, map[string]any{"message": message})
	case "iflow":
		nodeReply(w, 200, map[string]any{"message": "iFlow connected. Usage tracked per request."})
	case "deepseek":
		if token == "" {
			nodeReply(w, 200, map[string]any{"message": "DeepSeek API key not available. Add a key to view usage."})
			return
		}
		status, data, err := u.resetJSON(r.Context(), "GET", "https://api.deepseek.com/user/balance", map[string]string{"Authorization": "Bearer " + token, "Accept": "application/json"}, nil)
		if err != nil {
			nodeReply(w, 500, map[string]any{"error": err.Error()})
			return
		}
		if status == 401 || status == 403 {
			nodeReply(w, 200, map[string]any{"plan": "DeepSeek", "message": "DeepSeek authentication failed. Check the API key."})
			return
		}
		if status < 200 || status >= 300 {
			nodeReply(w, 200, map[string]any{"plan": "DeepSeek", "message": fmt.Sprintf("DeepSeek balance API error (%d)", status)})
			return
		}
		nodeReply(w, 200, normalizeDeepSeekUsage(data))
	case "ollama":
		if token == "" {
			nodeReply(w, 200, map[string]any{"message": "Ollama Cloud API key not available."})
			return
		}
		status, data, err := u.resetJSON(r.Context(), "GET", "https://ollama.com/api/usage", map[string]string{"Authorization": "Bearer " + token, "Accept": "application/json"}, nil)
		if err != nil {
			nodeReply(w, 500, map[string]any{"error": err.Error()})
			return
		}
		if status == 401 || status == 403 {
			nodeReply(w, 200, map[string]any{"message": "Ollama Cloud API key invalid or expired."})
			return
		}
		if status < 200 || status >= 300 {
			nodeReply(w, 200, map[string]any{"message": fmt.Sprintf("Ollama Cloud usage API error (%d).", status)})
			return
		}
		profileStatus, profile, profileErr := u.resetJSON(r.Context(), "POST", "https://ollama.com/api/me", map[string]string{"Authorization": "Bearer " + token, "Accept": "application/json", "Content-Length": "0"}, nil)
		if profileErr != nil || profileStatus < 200 || profileStatus >= 300 {
			profile = nil
		}
		nodeReply(w, 200, normalizeOllamaUsage(data, profile))
	case "vercel-ai-gateway":
		if token == "" {
			nodeReply(w, 200, map[string]any{"message": "Vercel AI Gateway API key not available."})
			return
		}
		status, data, err := u.resetJSON(r.Context(), "GET", "https://ai-gateway.vercel.sh/v1/credits", map[string]string{"Authorization": "Bearer " + token, "Accept": "application/json"}, nil)
		if err != nil {
			nodeReply(w, 200, map[string]any{"message": "Vercel AI Gateway error: " + err.Error()})
			return
		}
		if status == 401 || status == 403 {
			nodeReply(w, 200, map[string]any{"message": "Vercel AI Gateway API key invalid or expired."})
			return
		}
		if status < 200 || status >= 300 {
			nodeReply(w, 200, map[string]any{"message": fmt.Sprintf("Vercel AI Gateway credits API error (%d)", status)})
			return
		}
		nodeReply(w, 200, normalizeVercelUsage(data))
	default:
		nodeReply(w, 200, map[string]any{"message": "Usage API not implemented for " + provider})
	}
}
