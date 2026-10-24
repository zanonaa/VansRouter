package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type vertexServiceAccount struct {
	Type         string `json:"type"`
	ProjectID    string `json:"project_id"`
	ClientEmail  string `json:"client_email"`
	PrivateKey   string `json:"private_key"`
	TokenURI     string `json:"token_uri"`
	QuotaProject string `json:"quota_project_id"`
}

type vertexADC struct {
	Type         string `json:"type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RefreshToken string `json:"refresh_token"`
	QuotaProject string `json:"quota_project_id"`
}

type vertexToken struct {
	value  string
	expiry time.Time
}

var vertexTokenCache = struct {
	sync.Mutex
	items map[string]vertexToken
}{items: map[string]vertexToken{}}

func vertexJSONCredential(apiKey string) (sa *vertexServiceAccount, adc *vertexADC) {
	raw := strings.TrimSpace(apiKey)
	if !strings.HasPrefix(raw, "{") {
		return nil, nil
	}
	var probe struct {
		Type string `json:"type"`
	}
	if json.Unmarshal([]byte(raw), &probe) != nil {
		return nil, nil
	}
	switch probe.Type {
	case "service_account":
		sa = &vertexServiceAccount{}
		if json.Unmarshal([]byte(raw), sa) != nil {
			return nil, nil
		}
	case "authorized_user":
		adc = &vertexADC{}
		if json.Unmarshal([]byte(raw), adc) != nil {
			return nil, nil
		}
	}
	return sa, adc
}

func vertexCachedToken(key string, mint func() (string, time.Time, error)) (string, error) {
	vertexTokenCache.Lock()
	cached, ok := vertexTokenCache.items[key]
	vertexTokenCache.Unlock()
	if ok && time.Until(cached.expiry) > 2*time.Minute {
		return cached.value, nil
	}
	value, expiry, err := mint()
	if err != nil {
		return "", err
	}
	vertexTokenCache.Lock()
	vertexTokenCache.items[key] = vertexToken{value: value, expiry: expiry}
	vertexTokenCache.Unlock()
	return value, nil
}

func vertexFormToken(ctx context.Context, client *http.Client, requestURL string, form url.Values) (string, time.Time, error) {
	request, err := http.NewRequestWithContext(ctx, "POST", requestURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", time.Time{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return "", time.Time{}, err
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("Vertex token error: %d %s", response.StatusCode, string(raw))
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", time.Time{}, err
	}
	token := ttsString(payload["access_token"])
	if token == "" {
		return "", time.Time{}, fmt.Errorf("Vertex token response missing access_token")
	}
	seconds := int(pxpipeNum(payload["expires_in"]))
	if seconds <= 0 {
		seconds = 3600
	}
	return token, time.Now().Add(time.Duration(seconds) * time.Second), nil
}

func vertexServiceAccountToken(ctx context.Context, client *http.Client, sa *vertexServiceAccount) (string, error) {
	hash := sha256.Sum256([]byte(sa.PrivateKey))
	return vertexCachedToken("sa:"+sa.ClientEmail+":"+sa.ProjectID+":"+hex.EncodeToString(hash[:8]), func() (string, time.Time, error) {
		key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(sa.PrivateKey))
		if err != nil {
			return "", time.Time{}, fmt.Errorf("Vertex service account private key: %w", err)
		}
		tokenURI := sa.TokenURI
		if tokenURI == "" {
			tokenURI = "https://oauth2.googleapis.com/token"
		}
		now := time.Now()
		claims := jwt.MapClaims{
			"iss":   sa.ClientEmail,
			"scope": "https://www.googleapis.com/auth/cloud-platform",
			"aud":   tokenURI,
			"iat":   now.Unix(),
			"exp":   now.Add(time.Hour).Unix(),
		}
		signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
		if err != nil {
			return "", time.Time{}, err
		}
		return vertexFormToken(ctx, client, tokenURI, url.Values{
			"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
			"assertion":  {signed},
		})
	})
}

func vertexADCToken(ctx context.Context, client *http.Client, adc *vertexADC) (string, error) {
	hash := sha256.Sum256([]byte(adc.RefreshToken))
	return vertexCachedToken("adc:"+adc.ClientID+":"+hex.EncodeToString(hash[:8]), func() (string, time.Time, error) {
		return vertexFormToken(ctx, client, "https://oauth2.googleapis.com/token", url.Values{
			"grant_type":    {"refresh_token"},
			"client_id":     {adc.ClientID},
			"client_secret": {adc.ClientSecret},
			"refresh_token": {adc.RefreshToken},
		})
	})
}

// vertexPostProcess replica postProcessForVertex: remove IDs que o Vertex
// rejeita nos functionCall/functionResponse.
func vertexPostProcess(body map[string]any) map[string]any {
	contents, _ := body["contents"].([]any)
	for _, rawTurn := range contents {
		turn, _ := rawTurn.(map[string]any)
		parts, _ := turn["parts"].([]any)
		for _, rawPart := range parts {
			part, _ := rawPart.(map[string]any)
			if call, ok := part["functionCall"].(map[string]any); ok {
				delete(call, "id")
			}
			if result, ok := part["functionResponse"].(map[string]any); ok {
				delete(result, "id")
			}
		}
	}
	return body
}

func (t *TranslatorSend) vertexCredentials(ctx context.Context, connection map[string]any) (token, project, location string, rawKey string, err error) {
	apiKey := ttsString(connection["apiKey"])
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	project = firstNonEmptyString(metadata["projectId"], metadata["quota_project_id"], connection["projectId"])
	location = firstNonEmptyString(metadata["location"], "us-central1")
	if access := ttsString(connection["accessToken"]); access != "" {
		return access, project, location, "", nil
	}
	sa, adc := vertexJSONCredential(apiKey)
	if sa != nil {
		token, err = vertexServiceAccountToken(ctx, t.Client, sa)
		if project == "" {
			project = firstNonEmptyString(sa.ProjectID, sa.QuotaProject)
		}
		return token, project, location, "", err
	}
	if adc != nil {
		token, err = vertexADCToken(ctx, t.Client, adc)
		if project == "" {
			project = adc.QuotaProject
		}
		return token, project, location, "", err
	}
	return "", project, location, apiKey, nil
}

// SendVertex porta vertex.js. O corpo é Gemini; a URL/auth variam entre API
// key (publishers + ?key=) e OAuth/SA/ADC (path com project/location).
func (t *TranslatorSend) SendVertex(w http.ResponseWriter, r *http.Request, provider, model string, body map[string]any, source string, stream bool) {
	fail := func(status int, message string) {
		nodeReply(w, status, map[string]any{"success": false, "error": message})
	}
	connections, err := t.Connections.List(r.Context(), provider, nil)
	if err != nil {
		fail(500, err.Error())
		return
	}
	var connection map[string]any
	for _, candidate := range connections {
		if candidate["isActive"] != false {
			connection = candidate
			break
		}
	}
	if connection == nil {
		fail(400, "No active connection for provider: "+provider)
		return
	}
	usageSelected(r.Context(), connection)
	intermediate := body
	if source != "openai" {
		intermediate, err = translateSource(body, source)
		if err != nil {
			fail(400, err.Error())
			return
		}
	}
	geminiBody, err := translateTarget(intermediate, "gemini", model)
	if err != nil {
		fail(400, err.Error())
		return
	}
	outbound := vertexPostProcess(geminiBody)
	t.TokenSaver.Apply(r.Context(), r, outbound, "vertex", model, provider)
	encoded, err := json.Marshal(outbound)
	if err != nil {
		fail(500, err.Error())
		return
	}
	token, project, location, rawKey, err := t.vertexCredentials(r.Context(), connection)
	if err != nil {
		fail(502, err.Error())
		return
	}
	action := "generateContent"
	if stream {
		action = "streamGenerateContent"
	}
	var target string
	if rawKey != "" {
		target = fmt.Sprintf("https://aiplatform.googleapis.com/v1/publishers/google/models/%s:%s", url.PathEscape(model), action)
		if stream {
			target += "?alt=sse&key=" + url.QueryEscape(rawKey)
		} else {
			target += "?key=" + url.QueryEscape(rawKey)
		}
	} else {
		if project == "" {
			fail(400, "Vertex OAuth/ADC requires a project_id. Add it to provider settings or use a Service Account JSON.")
			return
		}
		target = fmt.Sprintf("https://aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:%s", url.PathEscape(project), url.PathEscape(location), url.PathEscape(model), action)
		if stream {
			target += "?alt=sse"
		}
	}
	request, err := http.NewRequestWithContext(r.Context(), "POST", target, bytes.NewReader(encoded))
	if err != nil {
		fail(500, err.Error())
		return
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if stream {
		request.Header.Set("Accept", "text/event-stream")
	} else {
		request.Header.Set("Accept", "application/json")
	}
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	client, relay, cleanup, err := connectionHTTP(r.Context(), metadata, t.GetPool, t.Client)
	if err != nil {
		fail(500, err.Error())
		return
	}
	defer cleanup()
	if err := rewriteConnectionRelay(request, relay); err != nil {
		fail(500, err.Error())
		return
	}
	response, err := client.Do(request)
	if err != nil {
		fail(500, err.Error())
		return
	}
	defer response.Body.Close()
	if contentType := response.Header.Get("Content-Type"); response.StatusCode >= 200 && response.StatusCode < 300 && isHTMLContentType(contentType) {
		nodeReply(w, 502, map[string]any{"success": false, "error": fmt.Sprintf("Provider %s returned HTML instead of a chat response (content-type: %s)", provider, contentType)})
		return
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		details, _ := io.ReadAll(response.Body)
		nodeReply(w, response.StatusCode, map[string]any{"success": false, "error": fmt.Sprintf("Provider error: %d", response.StatusCode), "details": string(details)})
		return
	}
	if stream {
		conversion := &streamConversion{destination: w, source: source, target: "gemini", model: model, textBlock: -1, thinkingBlock: -1, serverTool: -1}
		_, _ = io.Copy(conversion, response.Body)
		conversion.finish()
	} else {
		conversion := &convertedResponse{destination: w, source: source, target: "gemini", model: model}
		_, _ = io.Copy(conversion, response.Body)
		conversion.finish()
	}
}
