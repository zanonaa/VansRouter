package handlers

import (
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Ordem derivada de REGISTRY.features.usage; não expõe tokens ao dashboard.
var usageProviderOrder = []string{"antigravity", "claude", "codebuddy-cn", "codex", "deepseek", "gemini-cli", "github", "glm-cn", "glm", "grok-cli", "kimi", "kiro", "minimax-cn", "minimax", "ollama", "opencode-go", "opencode-zen", "qoder", "vercel-ai-gateway", "codebuddy-intl", "freebuff", "zed", "qoder-cn"}
var usageAPIKeyProviders = map[string]bool{"codebuddy-cn": true, "deepseek": true, "glm-cn": true, "glm": true, "kimi": true, "kiro": true, "minimax-cn": true, "minimax": true, "ollama": true, "opencode-go": true, "opencode-zen": true, "qoder": true, "vercel-ai-gateway": true, "codebuddy-intl": true, "qoder-cn": true}
var clientSafeFields = strings.Fields("id provider authType name email displayName priority globalPriority isActive defaultModel testStatus lastError lastErrorAt errorCode expiresAt lastUsedAt consecutiveUseCount createdAt updatedAt")
var clientSafePSD = strings.Fields("baseUrl azureEndpoint deployment apiVersion accountId region projectId resourceUrl proxyPoolId connectionProxyEnabled connectionProxyUrl connectionNoProxy githubLogin githubName githubEmail githubUserId username firstName lastName authMethod authKind profileArn")
var clientTokenName = regexp.MustCompile(`[a-zA-Z0-9_-]{32,}`)
var clientInteger = regexp.MustCompile(`^[+-]?\d+`)

type ProviderClient struct {
	Connections NodeConnections
	mu          sync.Mutex
	backfilled  bool
}

func providerUsageIndex(provider string) int {
	for i, value := range usageProviderOrder {
		if provider == value {
			return i
		}
	}
	return -1
}
func clientPositiveInt(value string, fallback int) int {
	number, err := strconv.Atoi(clientInteger.FindString(strings.TrimSpace(value)))
	if err != nil || number <= 0 {
		return fallback
	}
	return number
}
func sanitizeProviderClient(connection map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range clientSafeFields {
		if value, exists := connection[key]; exists {
			out[key] = value
		}
	}
	if name, ok := out["name"].(string); ok && len(name) > 16 && clientTokenName.MatchString(name) {
		out["name"] = name[:8] + "***"
	}
	if nodeTruthy(connection["providerSpecificData"]) {
		data, _ := connection["providerSpecificData"].(map[string]any)
		safe := map[string]any{}
		for _, key := range clientSafePSD {
			if value, exists := data[key]; exists {
				safe[key] = value
			}
		}
		out["providerSpecificData"] = safe
	}
	return out
}
func (p *ProviderClient) backfill(r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.backfilled {
		return
	}
	p.backfilled = true
	connections, err := p.Connections.List(r.Context(), "", nil)
	if err != nil {
		p.backfilled = false
		return
	}
	for _, connection := range connections {
		metadata, _ := connection["providerSpecificData"].(map[string]any)
		if connection["provider"] != "codex" || connection["authType"] != "oauth" || !nodeTruthy(connection["idToken"]) || nodeTruthy(connection["email"]) && nodeTruthy(metadata["chatgptAccountId"]) {
			continue
		}
		claims := decodeJWTClaims(ttsString(connection["idToken"]))
		auth, _ := claims["https://api.openai.com/auth"].(map[string]any)
		email, account, plan := claims["email"], auth["chatgpt_account_id"], auth["chatgpt_plan_type"]
		if !nodeTruthy(email) && !nodeTruthy(account) {
			continue
		}
		patch := map[string]any{}
		if !nodeTruthy(connection["email"]) && nodeTruthy(email) {
			patch["email"] = email
		}
		if nodeTruthy(account) || nodeTruthy(plan) {
			copy := map[string]any{}
			for k, v := range metadata {
				copy[k] = v
			}
			copy["chatgptAccountId"] = account
			copy["chatgptPlanType"] = plan
			patch["providerSpecificData"] = copy
		}
		if len(patch) > 0 {
			if _, err := p.Connections.Update(r.Context(), ttsString(connection["id"]), patch); err != nil {
				p.backfilled = false
				return
			}
		}
	}
}
func (p *ProviderClient) Get(w http.ResponseWriter, r *http.Request) {
	p.backfill(r)
	connections, err := p.Connections.List(r.Context(), "", nil)
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": "Failed to fetch providers"})
		return
	}
	query := r.URL.Query()
	provider := query.Get("provider")
	if provider == "" {
		provider = "all"
	}
	page, size := clientPositiveInt(query.Get("page"), 1), clientPositiveInt(query.Get("pageSize"), 20)
	if size > 500 {
		size = 500
	}
	eligible := 0
	providerFiltered := 0
	optionsSet := map[string]bool{}
	filtered := []map[string]any{}
	for _, connection := range connections {
		id := ttsString(connection["provider"])
		if providerUsageIndex(id) < 0 || connection["authType"] != "oauth" && !usageAPIKeyProviders[id] {
			continue
		}
		eligible++
		optionsSet[id] = true
		if provider != "all" && id != provider {
			continue
		}
		providerFiltered++
		active := true
		if connection["isActive"] != nil {
			active = nodeTruthy(connection["isActive"])
		}
		if query.Get("accountStatus") == "active" && !active || query.Get("accountStatus") == "inactive" && active {
			continue
		}
		filtered = append(filtered, connection)
	}
	priority := func(c map[string]any) float64 {
		if c["priority"] == nil {
			return 9007199254740991
		}
		return pxpipeNum(c["priority"])
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := ttsString(filtered[i]["provider"]), ttsString(filtered[j]["provider"])
		if query.Get("sort") == "provider" {
			if providerUsageIndex(a) != providerUsageIndex(b) {
				return providerUsageIndex(a) < providerUsageIndex(b)
			}
		} else if priority(filtered[i]) != priority(filtered[j]) {
			return priority(filtered[i]) < priority(filtered[j])
		}
		return a < b
	})
	total := len(filtered)
	pages := (total + size - 1) / size
	if pages < 1 {
		pages = 1
	}
	if page > pages {
		page = pages
	}
	offset := (page - 1) * size
	end := offset + size
	if end > total {
		end = total
	}
	result := []map[string]any{}
	for _, connection := range filtered[offset:end] {
		result = append(result, sanitizeProviderClient(connection))
	}
	options := []string{}
	for key := range optionsSet {
		options = append(options, key)
	}
	sort.Strings(options)
	nodeReply(w, 200, map[string]any{"connections": result, "providerOptions": options, "pagination": map[string]any{"page": page, "pageSize": size, "total": total, "totalPages": pages}, "totals": map[string]any{"eligibleConnections": eligible, "providerFilteredConnections": providerFiltered}})
}
