package handlers

// Token Saver: porta de open-sse/handlers/chatCore.js (bloco "Token savers").
// As transformações são aplicadas no corpo FINAL de cada executor, na mesma
// ordem da origem: RTK → Headroom → system prompt → Caveman → Ponytail → Pxpipe.
// Tudo é fail-open: erro de configuração/transformação nunca impede o envio;
// porém nunca é silencioso — cada motivo é registrado no log.

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	tokenSaverHeader = "x-9router-token-saver"

	defaultTokenSaverHeadroomURL = "http://localhost:8787"
	defaultHeadroomTimeout       = 3000 * time.Millisecond
	defaultPxpipeMinChars        = 25000
	defaultPxpipeTimeout         = 15000 * time.Millisecond
	headroomCompressEndpoint     = "/v1/compress"
	tokenSaverEstCharsPerToken   = 4
)

// PxpipeTransformer é o contrato mínimo usado pelo TokenSaver. *PxpipeLoader
// implementa via subprocesso Node; testes podem injetar um fake.
type PxpipeTransformer interface {
	Transform(ctx context.Context, body []byte, model string, minChars int) (*pxpipeTransformed, error)
}

// TokenSaver lê as settings persistidas pela tela dashboard/token-saver e
// aplica as transformações no corpo que será enviado ao provider.
type TokenSaver struct {
	Read       func(context.Context) (map[string]any, error)
	HTTPClient *http.Client
	Pxpipe     PxpipeTransformer
	// PxpipeDataDir é usado para os eventos do dashboard (events.jsonl).
	PxpipeDataDir string
	// PxpipeTransform permite substituir o transformador (testes/host).
	PxpipeTransform func(ctx context.Context, body []byte, model string, minChars int) (*pxpipeTransformed, error)
}

type tokenSaverSettings struct {
	rtkEnabled           bool
	headroomEnabled      bool
	headroomURL          string
	headroomUserMessages bool
	headroomTimeout      time.Duration
	cavemanEnabled       bool
	cavemanLevel         string
	ponytailEnabled      bool
	ponytailLevel        string
	pxpipeEnabled        bool
	pxpipeMinChars       int
	pxpipeTimeout        time.Duration
}

// Apply transforma body in-place. format é o formato final do corpo enviado ao
// upstream (openai, claude, gemini, openai-responses, ollama, gemini-cli,
// vertex...). provider/model são usados somente em logs/eventos.
func (s *TokenSaver) Apply(ctx context.Context, r *http.Request, body map[string]any, format, model, provider string) {
	if s == nil || body == nil {
		return
	}
	if r != nil && strings.EqualFold(r.Header.Get(tokenSaverHeader), "off") {
		slog.Debug("TOKEN_SAVER bypassed by header", "header", tokenSaverHeader, "provider", provider, "model", model)
		return
	}
	settings, err := s.settings(ctx)
	if err != nil {
		// Não silenciar: a configuração não pôde ser lida, então o request
		// segue intacto (fail-open) e o operador vê o motivo.
		slog.Warn("TOKEN_SAVER settings unavailable; request sent unchanged", "provider", provider, "model", model, "error", err)
		return
	}
	cfg := parseTokenSaverSettings(settings)
	format = normalizeTokenSaverFormat(format)

	if cfg.rtkEnabled {
		stats := compressRTKMessages(body)
		if line := formatRTKLog(stats); line != "" {
			slog.Info("RTK", "detail", line, "provider", provider, "model", model)
		}
	}

	if cfg.headroomEnabled {
		diag := &headroomDiagnostics{}
		stats := s.compressWithHeadroom(ctx, body, cfg, format, model, diag)
		if line := formatHeadroomLog(stats); line != "" {
			size := formatHeadroomSizeLog(diag)
			if size != "" {
				slog.Info("HEADROOM", "detail", line, "sizes", size, "provider", provider, "model", model)
			} else {
				slog.Info("HEADROOM", "detail", line, "provider", provider, "model", model)
			}
			if headroomPhantomSavings(stats, diag) {
				slog.Warn("HEADROOM reported token delta, but outbound JSON shrank <5%; provider may bill near-original payload", "sizes", size, "provider", provider, "model", model)
			}
		} else {
			slog.Warn("HEADROOM skipped", "reason", diag.reasonOr("compression unavailable"), "endpoint", diag.Endpoint, "provider", provider, "model", model)
		}
	}

	if cfg.cavemanEnabled {
		if prompt := cavemanPrompts[cfg.cavemanLevel]; prompt != "" {
			injectSystemPrompt(body, format, prompt)
			slog.Debug("CAVEMAN", "level", cfg.cavemanLevel, "format", format, "provider", provider, "model", model)
		} else {
			slog.Warn("CAVEMAN skipped: unknown level", "level", cfg.cavemanLevel, "provider", provider, "model", model)
		}
	}

	if cfg.ponytailEnabled {
		if prompt := ponytailPrompts[cfg.ponytailLevel]; prompt != "" {
			injectSystemPrompt(body, format, prompt)
			slog.Debug("PONYTAIL", "level", cfg.ponytailLevel, "format", format, "provider", provider, "model", model)
		} else {
			slog.Warn("PONYTAIL skipped: unknown level", "level", cfg.ponytailLevel, "provider", provider, "model", model)
		}
	}

	if cfg.pxpipeEnabled {
		s.applyPxpipe(ctx, body, cfg, format, model, provider)
	}
}

func (s *TokenSaver) settings(ctx context.Context) (map[string]any, error) {
	if s.Read == nil {
		return map[string]any{}, nil
	}
	return s.Read(ctx)
}

func normalizeTokenSaverFormat(format string) string {
	switch format {
	case "gemini_cli":
		return "gemini-cli"
	case "":
		return "openai"
	}
	return format
}

func parseTokenSaverSettings(settings map[string]any) tokenSaverSettings {
	cfg := tokenSaverSettings{
		// Defaults de settingsRepo.js: RTK ligado, demais desligados.
		rtkEnabled:           true,
		cavemanLevel:         "full",
		ponytailLevel:        "full",
		headroomURL:          defaultHeadroomURLFromEnv(),
		headroomTimeout:      defaultHeadroomTimeout,
		pxpipeMinChars:       defaultPxpipeMinChars,
		pxpipeTimeout:        defaultPxpipeTimeout,
		headroomUserMessages: false,
	}
	if value, ok := settings["rtkEnabled"]; ok {
		cfg.rtkEnabled = tokenSaverTruthy(value)
	}
	if value, ok := settings["headroomEnabled"]; ok {
		cfg.headroomEnabled = tokenSaverTruthy(value)
	}
	if value, ok := settings["headroomUrl"]; ok {
		if text := ttsString(value); text != "" {
			cfg.headroomURL = text
		}
	}
	if value, ok := settings["headroomCompressUserMessages"]; ok {
		cfg.headroomUserMessages = tokenSaverTruthy(value)
	}
	if value, ok := settings["headroomTimeoutMs"]; ok {
		if ms := tokenSaverInt(value, int64(defaultHeadroomTimeout/time.Millisecond)); ms > 0 {
			cfg.headroomTimeout = time.Duration(ms) * time.Millisecond
		}
	}
	if value, ok := settings["cavemanEnabled"]; ok {
		cfg.cavemanEnabled = tokenSaverTruthy(value)
	}
	if value, ok := settings["cavemanLevel"]; ok {
		if text := ttsString(value); text != "" {
			cfg.cavemanLevel = text
		}
	}
	if value, ok := settings["ponytailEnabled"]; ok {
		cfg.ponytailEnabled = tokenSaverTruthy(value)
	}
	if value, ok := settings["ponytailLevel"]; ok {
		if text := ttsString(value); text != "" {
			cfg.ponytailLevel = text
		}
	}
	if value, ok := settings["pxpipeEnabled"]; ok {
		cfg.pxpipeEnabled = tokenSaverTruthy(value)
	}
	if value, ok := settings["pxpipeMinChars"]; ok {
		if n := tokenSaverInt(value, defaultPxpipeMinChars); n > 0 {
			cfg.pxpipeMinChars = int(n)
		}
	}
	if value, ok := settings["pxpipeTimeoutMs"]; ok {
		if ms := tokenSaverInt(value, int64(defaultPxpipeTimeout/time.Millisecond)); ms > 0 {
			cfg.pxpipeTimeout = time.Duration(ms) * time.Millisecond
		}
	}
	return cfg
}

func defaultHeadroomURLFromEnv() string {
	if value := strings.TrimSpace(os.Getenv("HEADROOM_URL")); value != "" {
		return value
	}
	return defaultTokenSaverHeadroomURL
}

// tokenSaverTruthy reproduz a coerção booleana do JS usada nas settings.
func tokenSaverTruthy(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		return typed != ""
	case float64:
		return typed != 0
	case float32:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case json.Number:
		number, err := typed.Float64()
		return err == nil && number != 0
	default:
		return true
	}
}

func tokenSaverInt(value any, fallback int64) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case int:
		return int64(typed)
	case int64:
		return typed
	case json.Number:
		number, err := typed.Int64()
		if err == nil {
			return number
		}
		parsed, err := typed.Float64()
		if err == nil {
			return int64(parsed)
		}
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err == nil {
			return int64(parsed)
		}
	}
	return fallback
}
