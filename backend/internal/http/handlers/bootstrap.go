// Package handlers implementa os contratos HTTP migrados do backend original.
package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// Readiness consulta o banco real; o handler não confunde disponibilidade HTTP com persistência.
type Readiness func(context.Context) error

type Bootstrap struct{ Ready Readiness }

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func healthCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
}

func (Bootstrap) Health(w http.ResponseWriter, r *http.Request) {
	healthCORS(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (Bootstrap) HealthOptions(w http.ResponseWriter, r *http.Request) {
	healthCORS(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h Bootstrap) Readiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.Ready == nil || h.Ready(r.Context()) != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "database": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "database": "ready"})
}

func (Bootstrap) Init(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain;charset=UTF-8")
	_, _ = io.WriteString(w, "Initialized")
}

var locales = map[string]bool{
	"en": true, "vi": true, "zh-CN": true, "zh-TW": true, "ja": true, "pt-BR": true, "pt-PT": true, "ko": true, "es": true, "de": true, "fr": true, "he": true, "ar": true, "ru": true, "pl": true, "cs": true, "nl": true, "tr": true, "uk": true, "tl": true, "id": true, "th": true, "hi": true, "bn": true, "ur": true, "fa": true, "ro": true, "sv": true, "it": true, "el": true, "hu": true, "fi": true, "da": true, "no": true,
}

func (Bootstrap) Locale(w http.ResponseWriter, r *http.Request) {
	// Mantém o comportamento do handler original: JSON malformado/null gera 500,
	// enquanto locale ausente, não textual ou não suportado gera 400.
	var raw json.RawMessage
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&raw); err != nil || string(raw) == "null" {
		writeJSON(w, 500, map[string]string{"error": "Failed to set locale"})
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeJSON(w, 500, map[string]string{"error": "Failed to set locale"})
		return
	}
	var body map[string]json.RawMessage
	_ = json.Unmarshal(raw, &body) // Primitivos e listas não têm propriedade locale no código original.
	var locale string
	if err := json.Unmarshal(body["locale"], &locale); err != nil || !locales[locale] {
		writeJSON(w, 400, map[string]string{"error": "Invalid locale"})
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "locale", Value: locale, Path: "/", MaxAge: 31536000})
	writeJSON(w, 200, map[string]any{"success": true, "locale": locale})
}
