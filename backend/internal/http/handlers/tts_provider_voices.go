package handlers

// Porta de src/app/api/media-providers/tts/{deepgram,elevenlabs,inworld,minimax}/voices.
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// TTSVoices expõe as rotas por provider de voz; Connections reusa o store.
type TTSVoices struct {
	Connections NodeConnections
	Client      *http.Client
}

func (v TTSVoices) client() *http.Client {
	if v.Client != nil {
		return v.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (v TTSVoices) firstAPIKey(ctx context.Context, provider, message string) (string, error) {
	if v.Connections == nil {
		return "", fmt.Errorf("%s", message)
	}
	active := true
	connections, err := v.Connections.List(ctx, provider, &active)
	if err != nil || len(connections) == 0 {
		return "", fmt.Errorf("%s", message)
	}
	key, _ := connections[0]["apiKey"].(string)
	return key, nil
}

// ttsLangGroup com dedupe por rota (JS usa Map de ids).
type ttsLangGroup struct {
	Code   string
	Name   string
	Voices []map[string]any
	seen   map[string]bool
}

func newLangGroup(code string) *ttsLangGroup {
	return &ttsLangGroup{Code: code, Name: voiceLanguage(code), Voices: []map[string]any{}, seen: map[string]bool{}}
}

func (g *ttsLangGroup) addVoice(id string, voice map[string]any) bool {
	if g.seen[id] {
		return false
	}
	g.seen[id] = true
	g.Voices = append(g.Voices, voice)
	return true
}

// groupVoices monta {languages, byLang} ordenado por nome (customLast replica
// o sort do MiniMax, que coloca "Custom" por último).
func groupVoices(langFilter string, byLang map[string]*ttsLangGroup) map[string]any {
	return groupVoicesCustom(langFilter, byLang, false)
}

func groupVoicesCustom(langFilter string, byLang map[string]*ttsLangGroup, customLast bool) map[string]any {
	if langFilter != "" {
		if group, ok := byLang[langFilter]; ok {
			return map[string]any{"voices": group.Voices}
		}
		return map[string]any{"voices": []map[string]any{}}
	}
	groups := make([]*ttsLangGroup, 0, len(byLang))
	for _, group := range byLang {
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool {
		// JS usa localeCompare; Go compara bytes (divergência documentada).
		if customLast {
			lastI, lastJ := groups[i].Code == "Custom", groups[j].Code == "Custom"
			if lastI != lastJ {
				return lastJ
			}
		}
		return strings.Compare(fmt.Sprint(groups[i].Name), fmt.Sprint(groups[j].Name)) < 0
	})
	languages := make([]map[string]any, 0, len(groups))
	byLangJSON := map[string]any{}
	for _, group := range groups {
		languages = append(languages, map[string]any{"code": group.Code, "name": group.Name, "voices": group.Voices})
		byLangJSON[group.Code] = map[string]any{"code": group.Code, "name": group.Name, "voices": group.Voices}
	}
	// Sem lang a origem não devolve "voices" no topo.
	return map[string]any{"languages": languages, "byLang": byLangJSON}
}

func readUpstreamBody(res *http.Response) (int, []byte) {
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	return res.StatusCode, body
}

// ttsString diferencia ausência de "null": String(null) do JS não deve
// contaminar ids/nomes vindos do payload.
func ttsString(value any) string {
	if !nodeTruthy(value) {
		return ""
	}
	return jsString(value)
}

func ttsUpstreamError(w http.ResponseWriter, prefix string, status int, body []byte) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		text = "Failed"
	}
	nodeReply(w, 502, map[string]string{"error": fmt.Sprintf("%s API %d: %s", prefix, status, text)})
}

// ── Deepgram ────────────────────────────────────────────────────────────────
func (v TTSVoices) Deepgram(w http.ResponseWriter, r *http.Request) {
	langFilter := r.URL.Query().Get("lang")
	apiKey, err := v.firstAPIKey(r.Context(), "deepgram", "No Deepgram connection found")
	if err != nil {
		nodeReply(w, 400, map[string]string{"error": err.Error()})
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), "GET", "https://api.deepgram.com/v1/models", nil)
	if err != nil {
		nodeReply(w, 502, map[string]string{"error": "Failed to fetch voices"})
		return
	}
	req.Header.Set("Authorization", "Token "+apiKey)
	res, err := v.client().Do(req)
	if err != nil {
		nodeReply(w, 502, map[string]string{"error": "Failed to fetch voices"})
		return
	}
	status, body := readUpstreamBody(res)
	_ = res.Body.Close()
	if status < 200 || status >= 300 {
		ttsUpstreamError(w, "Deepgram", status, body)
		return
	}
	var data struct {
		TTS []map[string]any `json:"tts"`
	}
	if json.Unmarshal(body, &data) != nil {
		nodeReply(w, 502, map[string]string{"error": "Failed to fetch voices"})
		return
	}
	byLang := map[string]*ttsLangGroup{}
	genderTags := map[string]bool{"masculine": true, "feminine": true}
	for _, model := range data.TTS {
		codes := []string{}
		if rawLanguages, ok := model["languages"].([]any); ok && len(rawLanguages) > 0 {
			for _, raw := range rawLanguages {
				if code := ttsString(raw); code != "" {
					codes = append(codes, code)
				}
			}
		}
		if len(codes) == 0 {
			canonical := ttsString(model["canonical_name"])
			parts := strings.Split(canonical, "-")
			suffix := "en"
			if canonical != "" {
				suffix = parts[len(parts)-1]
			}
			codes = append(codes, suffix)
		}
		voiceID := ttsString(model["canonical_name"])
		if voiceID == "" {
			voiceID = ttsString(model["name"])
		}
		if voiceID == "" {
			continue
		}
		name := ttsString(model["name"])
		if name == "" {
			name = voiceID
		}
		gender := ""
		if metadata, ok := model["metadata"].(map[string]any); ok {
			if tags, ok := metadata["tags"].([]any); ok {
				for _, tag := range tags {
					if genderTags[ttsString(tag)] {
						gender = ttsString(tag)
						break
					}
				}
			}
		}
		for _, code := range codes {
			group, ok := byLang[code]
			if !ok {
				group = newLangGroup(code)
				byLang[code] = group
			}
			group.addVoice(voiceID, map[string]any{"id": voiceID, "name": name, "gender": gender, "lang": code})
		}
	}
	nodeReply(w, 200, groupVoices(langFilter, byLang))
}

// ── ElevenLabs ──────────────────────────────────────────────────────────────
type elevenVoicesCacheEntry struct {
	voices  []map[string]any
	fetched time.Time
}

var (
	elevenVoicesCacheMu sync.Mutex
	elevenVoicesCache   = map[string]elevenVoicesCacheEntry{}
)

// voicesWithCache espelha o cache de 24h do open-sse fetchElevenLabsVoices.
func voicesWithCache(ctx context.Context, client *http.Client, apiKey string) ([]map[string]any, error) {
	elevenVoicesCacheMu.Lock()
	entry, ok := elevenVoicesCache[apiKey]
	elevenVoicesCacheMu.Unlock()
	if ok && time.Since(entry.fetched) < 24*time.Hour {
		return entry.voices, nil
	}
	voices, err := fetchElevenVoices(ctx, client, apiKey)
	if err != nil {
		return nil, err
	}
	elevenVoicesCacheMu.Lock()
	elevenVoicesCache[apiKey] = elevenVoicesCacheEntry{voices: voices, fetched: time.Now()}
	elevenVoicesCacheMu.Unlock()
	return voices, nil
}

func (v TTSVoices) ElevenLabs(w http.ResponseWriter, r *http.Request) {
	langFilter := r.URL.Query().Get("lang")
	apiKey, err := v.firstAPIKey(r.Context(), "elevenlabs", "No ElevenLabs connection found")
	if err != nil {
		nodeReply(w, 400, map[string]string{"error": err.Error()})
		return
	}
	voices, err := voicesWithCache(r.Context(), v.client(), apiKey)
	if err != nil {
		nodeReply(w, 502, map[string]string{"error": err.Error()})
		return
	}
	byLang := map[string]*ttsLangGroup{}
	groupFor := func(code string) *ttsLangGroup {
		if group, ok := byLang[code]; ok {
			return group
		}
		group := newLangGroup(code)
		byLang[code] = group
		return group
	}
	for _, voice := range voices {
		voiceID := ttsString(voice["voice_id"])
		if voiceID == "" {
			continue
		}
		primary := "en"
		if labels, ok := voice["labels"].(map[string]any); ok {
			if language := ttsString(labels["language"]); nodeTruthy(language) {
				primary = language
			}
		}
		build := func(code string) map[string]any {
			gender := ""
			if labels, ok := voice["labels"].(map[string]any); ok {
				if value := ttsString(labels["gender"]); nodeTruthy(value) {
					gender = value
				}
			}
			freeUsersAllowed := false
			if jsString(voice["category"]) == "premade" {
				freeUsersAllowed = true
			}
			if owner, exists := voice["is_owner"]; exists && owner == true {
				freeUsersAllowed = true
			}
			return map[string]any{"id": voiceID, "name": voice["name"], "gender": gender,
				"lang": code, "free_users_allowed": freeUsersAllowed}
		}
		groupFor(primary).addVoice(voiceID, build(primary))
		if verified, ok := voice["verified_languages"].([]any); ok {
			for _, item := range verified {
				entry, _ := item.(map[string]any)
				language := ttsString(entry["language"])
				if nodeTruthy(language) && language != primary {
					groupFor(language).addVoice(voiceID, build(language))
				}
			}
		}
	}
	nodeReply(w, 200, groupVoices(langFilter, byLang))
}

// ── Inworld ─────────────────────────────────────────────────────────────────
func (v TTSVoices) Inworld(w http.ResponseWriter, r *http.Request) {
	langFilter := r.URL.Query().Get("lang")
	apiKey, err := v.firstAPIKey(r.Context(), "inworld", "No Inworld connection found")
	if err != nil {
		nodeReply(w, 400, map[string]string{"error": err.Error()})
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), "GET", "https://api.inworld.ai/tts/v1/voices", nil)
	if err != nil {
		nodeReply(w, 502, map[string]string{"error": "Failed to fetch voices"})
		return
	}
	req.Header.Set("Authorization", "Basic "+apiKey)
	res, err := v.client().Do(req)
	if err != nil {
		nodeReply(w, 502, map[string]string{"error": "Failed to fetch voices"})
		return
	}
	status, body := readUpstreamBody(res)
	_ = res.Body.Close()
	if status < 200 || status >= 300 {
		ttsUpstreamError(w, "Inworld", status, body)
		return
	}
	var data struct {
		Voices []map[string]any `json:"voices"`
	}
	if json.Unmarshal(body, &data) != nil {
		nodeReply(w, 502, map[string]string{"error": "Failed to fetch voices"})
		return
	}
	byLang := map[string]*ttsLangGroup{}
	for _, voice := range data.Voices {
		voiceID := ttsString(voice["voiceId"])
		if voiceID == "" {
			continue
		}
		codes := []string{}
		if raw, ok := voice["languages"].([]any); ok && len(raw) > 0 {
			for _, language := range raw {
				if code := jsString(language); code != "" {
					codes = append(codes, code)
				}
			}
		}
		if len(codes) == 0 {
			codes = append(codes, "en")
		}
		name := ""
		if nodeTruthy(voice["displayName"]) {
			name = ttsString(voice["displayName"])
		}
		if !nodeTruthy(name) {
			name = voiceID
		}
		gender := ttsString(voice["gender"])
		for _, code := range codes {
			group, ok := byLang[code]
			if !ok {
				group = newLangGroup(code)
				byLang[code] = group
			}
			group.addVoice(voiceID, map[string]any{"id": voiceID, "name": name, "gender": gender, "lang": code})
		}
	}
	nodeReply(w, 200, groupVoices(langFilter, byLang))
}

// ── MiniMax ─────────────────────────────────────────────────────────────────
var minimaxVoiceEndpoints = map[string]string{
	"minimax":    "https://api.minimax.io/v1/get_voice",
	"minimax-cn": "https://api.minimaxi.com/v1/get_voice",
}

var minimaxVoiceGroups = []struct {
	Key   string
	Label string
}{
	{"system_voice", "System"},
	{"voice_cloning", "Cloned"},
	{"voice_generation", "Generated"},
	{"music_generation", "Music"},
}

func minimaxInferLanguage(voiceID string) string {
	value := strings.TrimSpace(voiceID)
	if !strings.Contains(value, "_") {
		return "Custom"
	}
	parts := strings.SplitN(value, "_", 2)
	if parts[0] == "" {
		return "Custom"
	}
	return parts[0]
}

func normalizeMiniMaxVoices(data map[string]any) map[string]*ttsLangGroup {
	byLang := map[string]*ttsLangGroup{}
	for _, entry := range minimaxVoiceGroups {
		items, _ := data[entry.Key].([]any)
		for _, raw := range items {
			item, _ := raw.(map[string]any)
			if !nodeTruthy(item["voice_id"]) && !nodeTruthy(item["voiceId"]) {
				continue
			}
			id := firstNonEmptyString(item["voice_id"], item["voiceId"])
			lang := minimaxInferLanguage(id)
			if entry.Key != "system_voice" {
				lang = "Custom"
			}
			display := firstNonEmptyString(item["voice_name"], item["voiceName"], id)
			if entry.Key != "system_voice" {
				display = display + " · " + entry.Label
			}
			langGroup, ok := byLang[lang]
			if !ok {
				// Na origem o name do grupo é o próprio código.
				langGroup = &ttsLangGroup{Code: lang, Name: lang, Voices: []map[string]any{}, seen: map[string]bool{}}
				byLang[lang] = langGroup
			}
			langGroup.addVoice(id, map[string]any{"id": id, "name": display, "lang": lang, "category": entry.Key})
		}
	}
	return byLang
}

func (v TTSVoices) MiniMax(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	if provider != "minimax-cn" {
		provider = "minimax"
	}
	voiceType := r.URL.Query().Get("voice_type")
	if voiceType == "" {
		voiceType = "all"
	}
	langFilter := r.URL.Query().Get("lang")
	apiKey, err := v.firstAPIKey(r.Context(), provider, fmt.Sprintf("No %s connection found", provider))
	if err != nil {
		nodeReply(w, 400, map[string]string{"error": err.Error()})
		return
	}
	target := minimaxVoiceEndpoints[provider]
	if target == "" {
		target = minimaxVoiceEndpoints["minimax"]
	}
	payload, _ := json.Marshal(map[string]string{"voice_type": voiceType})
	req, err := http.NewRequestWithContext(r.Context(), "POST", target, strings.NewReader(string(payload)))
	if err != nil {
		nodeReply(w, 502, map[string]string{"error": "Failed to fetch MiniMax voices"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := v.client().Do(req)
	if err != nil {
		nodeReply(w, 502, map[string]string{"error": "Failed to fetch MiniMax voices"})
		return
	}
	status, raw := readUpstreamBody(res)
	_ = res.Body.Close()
	data := map[string]any{}
	if strings.TrimSpace(string(raw)) != "" && json.Unmarshal(raw, &data) != nil {
		data = map[string]any{}
	}
	statusCode := minimaxStatus(data["base_resp"], data["baseResp"])
	statusMessage := minimaxStatusMessage(data["base_resp"], data["baseResp"], data["message"])
	if status < 200 || status >= 300 {
		text := strings.TrimSpace(statusMessage)
		if text == "" {
			text = strings.TrimSpace(string(raw))
		}
		if text == "" {
			text = "Failed"
		}
		nodeReply(w, 502, map[string]string{"error": fmt.Sprintf("MiniMax API %d: %s", status, text)})
		return
	}
	if statusCode != 0 {
		message := statusMessage
		if message == "" {
			message = "MiniMax voice API error"
		}
		nodeReply(w, 502, map[string]string{"error": message})
		return
	}
	byLang := normalizeMiniMaxVoices(data)
	for _, group := range byLang {
		voices := group.Voices
		sort.Slice(voices, func(i, j int) bool {
			return strings.Compare(fmt.Sprint(voices[i]["name"]), fmt.Sprint(voices[j]["name"])) < 0
		})
	}
	if langFilter != "" {
		group, ok := byLang[langFilter]
		if !ok {
			nodeReply(w, 200, map[string]any{"voices": []map[string]any{}})
			return
		}
		nodeReply(w, 200, map[string]any{"voices": group.Voices})
		return
	}
	nodeReply(w, 200, groupVoicesCustom("", byLang, true))
}

func minimaxStatus(options ...any) float64 {
	for _, option := range options {
		base, ok := option.(map[string]any)
		if !ok {
			continue
		}
		if value, ok := base["status_code"]; ok {
			return minimaxNumber(value)
		}
		if value, ok := base["statusCode"]; ok {
			return minimaxNumber(value)
		}
	}
	return 0
}

func minimaxStatusMessage(options ...any) string {
	for _, option := range options {
		switch value := option.(type) {
		case map[string]any:
			if message := jsString(value["status_msg"]); nodeTruthy(message) {
				return message
			}
			if message := jsString(value["statusMsg"]); nodeTruthy(message) {
				return message
			}
			if message := jsString(value["message"]); nodeTruthy(message) {
				return message
			}
		case string:
			if nodeTruthy(value) {
				return value
			}
		}
	}
	return ""
}

func minimaxNumber(value any) float64 {
	if number, ok := value.(float64); ok {
		return number
	}
	return 0
}
