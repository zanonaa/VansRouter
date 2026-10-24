package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

// comboAttempt delays commitment until a successful response. Successful bytes
// stream directly; failed candidates cannot leak an error before fallback.
type comboAttempt struct {
	destination http.ResponseWriter
	header      http.Header
	status      int
	failure     bytes.Buffer
	committed   bool
}

func (c *comboAttempt) Header() http.Header {
	if c.header == nil {
		c.header = http.Header{}
	}
	return c.header
}
func (c *comboAttempt) WriteHeader(status int) {
	if c.status == 0 {
		c.status = status
	}
}
func (c *comboAttempt) Write(body []byte) (int, error) {
	if c.status == 0 {
		c.status = 200
	}
	if c.status >= 200 && c.status < 300 {
		c.commit()
		return c.destination.Write(body)
	}
	if c.failure.Len()+len(body) > 1<<20 {
		return 0, io.ErrShortBuffer
	}
	return c.failure.Write(body)
}
func (c *comboAttempt) commit() {
	if c.committed {
		return
	}
	for k, v := range c.header {
		c.destination.Header()[k] = v
	}
	c.destination.WriteHeader(c.status)
	c.committed = true
}
func (c *comboAttempt) Flush() {
	if c.status >= 200 && c.status < 300 {
		c.commit()
		if f, ok := c.destination.(http.Flusher); ok {
			f.Flush()
		}
	}
}
func (c *comboAttempt) finalError() {
	for k, v := range c.header {
		c.destination.Header()[k] = v
	}
	status := c.status
	if status == 0 {
		status = 502
	}
	c.destination.WriteHeader(status)
	c.destination.Write(c.failure.Bytes())
}

// expandComboEntries expande as entradas de um combo. Entradas sem "/" são
// referência a outro combo (combos aninhados, como a origem resolve em
// getComboModelsFromData); entradas sem barra que não são combo passam
// adiante para falharem como model downstream. Ciclos são rejeitados.
// comboShouldFallback reproduz a política essencial de
// open-sse/services/accountFallback.js (ERROR_RULES): erros de conta
// (401/402/403/404/429) e de servidor (5xx) seguem para o próximo model do
// combo; erros do pedido (400/422/499) e bloqueios de conteúdo param no
// primeiro — a origem devolve esses últimos ao cliente.
var comboNoFallbackTexts = []string{
	"content-blocked", "content_blocked", "content exists risk",
	"sensitive words detected", "sensitive content", "unapproved channel",
	"illegal api invocation",
}

func comboShouldFallback(status int, errorText string) bool {
	lower := strings.ToLower(errorText)
	for _, text := range comboNoFallbackTexts {
		if strings.Contains(lower, text) {
			return false
		}
	}
	// accountFallback.js: erros 4xx sem regra explícita são do pedido e não
	// devem consumir outro candidato do combo. 401/402/403/404/429 têm regra
	// própria (fallback) e continuam para o próximo model.
	if status >= 400 && status < 500 &&
		status != 401 && status != 402 && status != 403 && status != 404 && status != 429 {
		return false
	}
	return true
}

// comboFailureMessage reproduz a extração de lastError em
// open-sse/services/combo.js: error.message, error (string) ou message do
// corpo do último candidato. Os executores do port embrulham o corpo do
// upstream em "details" atrás de um "Provider error: <status>" genérico; nesse
// caso a mensagem específica é a que deve chegar ao cliente.
func comboFailureMessage(body []byte, status int) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 {
		var payload map[string]any
		if json.Unmarshal(trimmed, &payload) == nil {
			if details, ok := payload["details"].(string); ok && strings.TrimSpace(details) != "" {
				if message := comboMessageFromJSON([]byte(details)); message != "" {
					return message
				}
			}
			if message := comboMessageFromJSON(trimmed); message != "" {
				return message
			}
		} else if !bytes.HasPrefix(trimmed, []byte("data:")) {
			return string(trimmed)
		}
	}
	return fmt.Sprintf("Provider error: %d", status)
}

func comboMessageFromJSON(raw []byte) string {
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return ""
	}
	switch value := payload["error"].(type) {
	case string:
		if message := strings.TrimSpace(value); message != "" && !comboGenericProviderError(message) {
			return message
		}
	case map[string]any:
		if message := strings.TrimSpace(ttsString(value["message"])); message != "" {
			return message
		}
	}
	if message := strings.TrimSpace(ttsString(payload["message"])); message != "" {
		return message
	}
	if message, ok := payload["error"].(string); ok {
		message = strings.TrimSpace(message)
		if message != "" && !comboGenericProviderError(message) {
			return message
		}
	}
	return ""
}

func comboGenericProviderError(message string) bool {
	return strings.HasPrefix(message, "Provider error: ")
}

func (g *Gateway) expandComboEntries(r *http.Request, name string, entries []any) ([]string, error) {
	models := []string{}
	var expand func(entry string, depth int, visited map[string]bool) error
	expand = func(entry string, depth int, visited map[string]bool) error {
		if visited[entry] || depth > 8 {
			return fmt.Errorf("Circular or too-deep combo reference: %s", entry)
		}
		nested, err := (database.Combos{DB: g.DB}).ByName(r.Context(), entry)
		if err != nil {
			return err
		}
		if nested == nil {
			models = append(models, entry)
			return nil
		}
		visited[entry] = true
		nestedEntries, _ := nested["models"].([]any)
		for _, raw := range nestedEntries {
			model, ok := raw.(string)
			if !ok || model == "" {
				return fmt.Errorf("Invalid combo model entry")
			}
			if strings.Contains(model, "/") {
				models = append(models, model)
				continue
			}
			if err := expand(model, depth+1, visited); err != nil {
				return err
			}
		}
		return nil
	}
	for _, raw := range entries {
		model, ok := raw.(string)
		if !ok || model == "" {
			return nil, fmt.Errorf("Invalid combo model entry")
		}
		if strings.Contains(model, "/") {
			models = append(models, model)
			continue
		}
		if err := expand(model, 0, map[string]bool{name: true}); err != nil {
			return nil, err
		}
	}
	return models, nil
}

func (g *Gateway) chatCombo(w http.ResponseWriter, r *http.Request, key, body map[string]any) bool {
	name := ttsString(body["model"])
	var exists int
	if err := g.DB.QueryRowContext(r.Context(), "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='combos'").Scan(&exists); err != nil {
		g.webError(w, 500, "Failed to load combos")
		return true
	}
	if exists == 0 {
		return false
	}
	combo, err := (database.Combos{DB: g.DB}).ByName(r.Context(), name)
	if err != nil {
		g.webError(w, 500, "Failed to load combos")
		return true
	}
	if combo == nil {
		return false
	}
	if !gatewayPermission(key, "allowedCombos", name) {
		g.webError(w, 403, `Combo "`+name+`" is not allowed for this API key`)
		return true
	}
	entries, _ := combo["models"].([]any)
	// A origem resolve entradas sem "/" como referência a outro combo
	// (combos aninhados, ex.: zn-glm → zn-gpt); expandimos com proteção de
	// ciclo — sem isso o combo inteiro era rejeitado com 400.
	models, err := g.expandComboEntries(r, name, entries)
	if err != nil {
		g.webError(w, 400, err.Error())
		return true
	}
	if len(models) == 0 {
		g.webError(w, 400, "No models configured for combo: "+name)
		return true
	}
	settings, err := database.ReadAuthSettings(r.Context(), g.DB)
	if err != nil {
		g.webError(w, 500, "Failed to load settings")
		return true
	}
	strategy := ttsString(settings["comboStrategy"])
	if strategy == "" {
		strategy = "fallback"
	}
	strategies, _ := settings["comboStrategies"].(map[string]any)
	custom, _ := strategies[name].(map[string]any)
	if override := ttsString(custom["fallbackStrategy"]); override != "" {
		strategy = override
	}
	if strategy == "fusion" {
		g.fusionCombo(w, r, models, name, custom, body)
		return true
	}
	if strategy != "fallback" && strategy != "round-robin" {
		g.webError(w, 501, "Combo strategy not implemented: "+strategy)
		return true
	}
	models = providers.RotateCombo(models, name, strategy, int(pxpipeNum(settings["comboStickyRoundRobinLimit"])))
	var last, firstFailure *comboAttempt
	for _, model := range models {
		if r.Context().Err() != nil {
			return true
		}
		candidate := map[string]any{}
		for k, v := range body {
			candidate[k] = v
		}
		candidate["model"] = model
		encoded, err := json.Marshal(candidate)
		if err != nil {
			g.webError(w, 400, err.Error())
			return true
		}
		request := r.Clone(r.Context())
		request.Body = io.NopCloser(bytes.NewReader(encoded))
		attempt := &comboAttempt{destination: w}
		g.OpenAIChat(attempt, request)
		if attempt.committed || attempt.status >= 200 && attempt.status < 300 {
			if !attempt.committed {
				attempt.commit()
			}
			return true
		}
		if firstFailure == nil {
			firstFailure = attempt
		}
		last = attempt
		if !comboShouldFallback(attempt.status, attempt.failure.String()) {
			// A origem devolve a resposta do candidato que bloqueou o fallback
			// (400/422/499 ou conteúdo bloqueado), não a de um anterior.
			last.finalError()
			return true
		}
	}
	if last != nil {
		// Todos falharam: a origem mantém o status do PRIMEIRO candidato que
		// falhou e a mensagem do ÚLTIMO (lastStatus/lastError em combo.js).
		// Sem isso, um fallback terminal (ex.: 402 do Grok) sobrescrevia o
		// 429 real do modelo primário do combo.
		status := 502
		if firstFailure != nil && firstFailure.status != 0 {
			status = firstFailure.status
		}
		g.webError(w, status, comboFailureMessage(last.failure.Bytes(), last.status))
	} else {
		g.webError(w, 503, "All accounts unavailable")
	}
	return true
}
