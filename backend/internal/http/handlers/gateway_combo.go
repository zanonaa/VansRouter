package handlers

import (
	"bytes"
	"encoding/json"
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
	models := []string{}
	for _, entry := range entries {
		model, ok := entry.(string)
		if !ok || !strings.Contains(model, "/") {
			g.webError(w, 400, "Invalid combo model entry")
			return true
		}
		models = append(models, model)
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
	if strategy != "fallback" && strategy != "round-robin" {
		g.webError(w, 501, "Combo strategy not implemented: "+strategy)
		return true
	}
	models = providers.RotateCombo(models, name, strategy, int(pxpipeNum(settings["comboStickyRoundRobinLimit"])))
	var last *comboAttempt
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
		last = attempt
		if attempt.status == 400 || attempt.status == 401 || attempt.status == 403 {
			break
		}
	}
	if last != nil {
		last.finalError()
	} else {
		g.webError(w, 503, "All accounts unavailable")
	}
	return true
}
