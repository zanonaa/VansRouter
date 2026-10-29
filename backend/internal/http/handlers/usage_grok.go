package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func grokNumber(value any) (float64, bool) {
	if object, ok := value.(map[string]any); ok {
		if wrapped, exists := object["val"]; exists {
			return usageFinite(wrapped)
		}
	}
	return usageFinite(value)
}

func grokQuota(used, total float64, reset any) map[string]any {
	used, total = math.Max(0, used), math.Max(0, total)
	percentage := 0.0
	unlimited := total == 0
	if total > 0 {
		percentage = math.Max(0, total-used) / total * 100
	} else if unlimited {
		percentage = 100
	}
	return map[string]any{"used": used, "total": total, "remainingPercentage": percentage, "resetAt": reset, "unlimited": unlimited}
}

func parseGrokUsage(billing, user map[string]any) map[string]any {
	config, _ := billing["config"].(map[string]any)
	if config == nil {
		config = billing
	}
	period, _ := config["currentPeriod"].(map[string]any)
	reset := usageResetDate(firstNonEmpty(config["billingPeriodEnd"], config["billing_period_end"], period["end"], config["resetAt"], config["resetsAt"], config["periodEnd"], billing["billingPeriodEnd"], billing["resetAt"]))
	tier := strings.TrimSpace(ttsString(firstNonEmpty(user["subscriptionTier"], user["subscription_tier"], config["subscriptionTier"], config["subscription_tier"])))
	subscription, _ := user["subscription"].(map[string]any)
	if tier == "" {
		tier = strings.TrimSpace(ttsString(subscription["tier"]))
	}
	quotas := map[string]any{}
	monthly, monthlyOK := grokNumber(firstNonEmpty(config["monthlyLimit"], config["monthly_limit"], billing["monthlyLimit"], billing["monthly_limit"]))
	included, includedOK := grokNumber(firstNonEmpty(config["includedUsed"], config["included_used"], billing["includedUsed"], billing["included_used"]))
	totalUsed, totalOK := grokNumber(firstNonEmpty(config["totalUsed"], config["total_used"], billing["totalUsed"], billing["total_used"]))
	if monthlyOK && monthly > 0 {
		used := 0.0
		if includedOK {
			used = included
		} else if totalOK {
			used = totalUsed
		}
		quotas["Monthly included"] = grokQuota(used, monthly, reset)
	}
	cap, capOK := grokNumber(firstNonEmpty(config["onDemandCap"], billing["onDemandCap"]))
	spent, spentOK := grokNumber(firstNonEmpty(config["onDemandUsed"], billing["onDemandUsed"]))
	subscriptionAccess := tier != "" && !regexp.MustCompile(`(?i)^(free|none|null)$`).MatchString(tier)
	if capOK && cap > 0 {
		used := 0.0
		if spentOK {
			used = math.Max(0, spent)
		}
		quotas["On-demand"] = grokQuota(used, cap, reset)
	} else if !subscriptionAccess && capOK && cap == 0 && spentOK {
		quotas["On-demand"] = map[string]any{"used": 1, "total": 1, "remainingPercentage": 0, "resetAt": reset, "unlimited": false}
	}
	if prepaid, ok := grokNumber(firstNonEmpty(config["prepaidBalance"], billing["prepaidBalance"])); ok && prepaid > 0 {
		quotas["Prepaid"] = map[string]any{"used": 0, "total": prepaid, "remainingPercentage": 100, "resetAt": nil, "unlimited": false}
	}
	if percent, ok := grokNumber(firstNonEmpty(config["creditUsagePercent"], config["credit_usage_percent"], billing["creditUsagePercent"])); ok && percent >= 0 {
		quotas["Weekly SuperGrok"] = grokQuota(math.Max(0, math.Min(100, percent)), 100, reset)
	}
	plan := "Grok Build"
	if tier != "" {
		words := strings.Fields(regexp.MustCompile(`[_-]+`).ReplaceAllString(tier, " "))
		for index, word := range words {
			words[index] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
		plan = strings.Join(words, " ")
	} else if user["hasGrokCodeAccess"] == true {
		plan = "Grok Code"
	}
	return map[string]any{"plan": plan, "quotas": quotas, "subscriptionAccess": subscriptionAccess}
}

func (u *UsageContracts) grokUsage(r *http.Request, connection map[string]any) map[string]any {
	token := ttsString(connection["accessToken"])
	if token == "" {
		return map[string]any{"message": "Grok CLI access token not available."}
	}
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	headers := map[string]string{"Authorization": "Bearer " + token, "Accept": "application/json", "User-Agent": "grok-shell/1.0.44 (linux; x86_64)", "x-xai-token-auth": "xai-grok-cli", "x-grok-client-identifier": "grok-shell", "x-grok-client-version": "1.0.44", "x-grok-client-mode": "headless"}
	if email := ttsString(metadata["email"]); email != "" {
		headers["x-email"] = email
	}
	if user := ttsString(firstNonEmpty(metadata["userId"], metadata["principalId"])); user != "" {
		headers["x-userid"] = user
	}
	status, billing, err := u.resetJSON(r.Context(), "GET", "https://cli-chat-proxy.grok.com/v1/billing?format=credits", headers, nil)
	if err != nil {
		return map[string]any{"message": "Grok CLI usage error: " + err.Error()}
	}
	if status == 401 || status == 403 {
		// O access token expira em 6h; a origem renova no próprio endpoint de
		// usage antes de repetir. Sem isso a tela ficava presa em
		// "authentication expired" mesmo com refresh token válido.
		if patch, refreshErr := oauthRefreshPatch(r.Context(), u.Client, "grok-cli", connection, nil); refreshErr == nil && patch != nil {
			if refreshed := ttsString(patch["accessToken"]); refreshed != "" {
				if _, updateErr := u.Connections.Update(r.Context(), ttsString(connection["id"]), patch); updateErr == nil {
					token = refreshed
					headers["Authorization"] = "Bearer " + token
					status, billing, err = u.resetJSON(r.Context(), "GET", "https://cli-chat-proxy.grok.com/v1/billing?format=credits", headers, nil)
					if err != nil {
						return map[string]any{"message": "Grok CLI usage error: " + err.Error()}
					}
				}
			}
		}
	}
	if status == 401 || status == 403 {
		return map[string]any{"message": "Grok CLI authentication expired. Please re-authorize."}
	}
	if status < 200 || status >= 300 {
		raw, _ := json.Marshal(billing)
		suffix := ""
		if len(raw) > 0 && string(raw) != "null" {
			suffix = ": " + string(raw)
			if len(suffix) > 202 {
				suffix = suffix[:202]
			}
		}
		return map[string]any{"message": fmt.Sprintf("Grok CLI billing API error (%d)%s", status, suffix)}
	}
	if billing == nil {
		return map[string]any{"message": "Grok CLI billing response was not JSON."}
	}
	_, user, _ := u.resetJSON(r.Context(), "GET", "https://cli-chat-proxy.grok.com/v1/user?include=subscription", headers, nil)
	parsed := parseGrokUsage(billing, user)
	if len(parsed["quotas"].(map[string]any)) == 0 {
		decoded, err := u.grokCredits(r.Context(), token)
		// Payload sem ratio (contas GrokPro/unified billing) não é erro: a origem
		// JS trata como null e cai na mensagem informativa abaixo.
		if err != nil {
			return map[string]any{"message": "Grok CLI usage error: " + err.Error()}
		}
		if decoded != nil {
			return map[string]any{"plan": parsed["plan"], "quotas": map[string]any{"Weekly SuperGrok": grokQuota(decoded.percent, 100, decoded.reset)}}
		}
		message := "Grok Build connected, but no credit allotment was returned. Free promo may be exhausted."
		if parsed["subscriptionAccess"] == true {
			message = "Subscription access is active; Grok does not expose a numeric included quota."
		}
		return map[string]any{"plan": parsed["plan"], "message": message, "quotas": map[string]any{}}
	}
	return map[string]any{"plan": parsed["plan"], "quotas": parsed["quotas"]}
}

type grokCredits struct {
	percent float64
	reset   any
}

func (u *UsageContracts) grokCredits(ctx context.Context, token string) (*grokCredits, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, "POST", "https://grok.com/grok_api_v2.GrokBuildBilling/GetGrokCreditsConfig", bytes.NewReader([]byte{0, 0, 0, 0, 0}))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/grpc-web+proto")
	request.Header.Set("X-Grpc-Web", "1")
	request.Header.Set("Accept", "application/grpc-web+proto")
	client := u.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		if response != nil {
			response.Body.Close()
		}
		return nil, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return decodeGrokCredits(raw)
}

func decodeGrokCredits(raw []byte) (*grokCredits, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	payload := raw
	if len(raw) >= 5 && (raw[0] == 0 || raw[0] == 1) {
		size := binary.BigEndian.Uint32(raw[1:5])
		if int(size)+5 > len(raw) {
			return nil, fmt.Errorf("invalid Grok credits frame")
		}
		payload = raw[5 : 5+size]
	}
	fields, err := protobufFields(payload)
	if err != nil {
		return nil, err
	}
	info, ok := fields[1].([]byte)
	if !ok {
		return nil, fmt.Errorf("Grok credits payload missing usage")
	}
	nested, err := protobufFields(info)
	if err != nil {
		return nil, err
	}
	ratio, ok := nested[1].(float64)
	if !ok || math.IsNaN(ratio) || ratio < 0 {
		// shortcut: upstream dropped the ratio field for unified-billing accounts,
		// return no quota instead of an error so the UI shows the fallback message
		return nil, nil
	}
	var reset any
	if timestamp, ok := nested[5].([]byte); ok {
		parts, err := protobufFields(timestamp)
		if err == nil {
			seconds, _ := parts[1].(uint64)
			nanos, _ := parts[2].(uint64)
			reset = time.Unix(int64(seconds), int64(nanos)).UTC().Format(time.RFC3339Nano)
		}
	}
	return &grokCredits{percent: math.Min(100, ratio*100), reset: reset}, nil
}

func protobufFields(raw []byte) (map[int]any, error) {
	fields := map[int]any{}
	for len(raw) > 0 {
		key, next, err := protobufVarint(raw)
		if err != nil {
			return nil, err
		}
		raw = raw[next:]
		number, wire := int(key>>3), key&7
		switch wire {
		case 0:
			value, next, err := protobufVarint(raw)
			if err != nil {
				return nil, err
			}
			fields[number] = value
			raw = raw[next:]
		case 5:
			if len(raw) < 4 {
				return nil, fmt.Errorf("truncated fixed32")
			}
			fields[number] = float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[:4])))
			raw = raw[4:]
		case 2:
			size, next, err := protobufVarint(raw)
			if err != nil || uint64(len(raw[next:])) < size {
				return nil, fmt.Errorf("truncated protobuf bytes")
			}
			start := next
			fields[number] = append([]byte(nil), raw[start:start+int(size)]...)
			raw = raw[start+int(size):]
		default:
			return nil, fmt.Errorf("unsupported protobuf wire type")
		}
	}
	return fields, nil
}

func protobufVarint(raw []byte) (uint64, int, error) {
	var value uint64
	for index, b := range raw {
		if index > 9 {
			return 0, 0, fmt.Errorf("protobuf varint too long")
		}
		value |= uint64(b&0x7f) << (7 * index)
		if b < 0x80 {
			return value, index + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("truncated protobuf varint")
}

func grokJWTPlan(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims map[string]any
	if json.Unmarshal(raw, &claims) != nil {
		return ""
	}
	names := map[int]string{0: "Free", 1: "SuperGrok", 2: "X Basic", 3: "X Premium", 4: "X Premium Plus", 5: "SuperGrok Heavy", 6: "SuperGrok Lite"}
	return names[int(usageNumber(claims["tier"]))]
}
