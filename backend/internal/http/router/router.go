// Package router registra os contratos já migrados para Go.
package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/zanonaa/VansRouter/backend/internal/auth"
	"github.com/zanonaa/VansRouter/backend/internal/http/handlers"
)

func New(readiness ...handlers.Readiness) http.Handler {
	var ready handlers.Readiness
	if len(readiness) > 0 {
		ready = readiness[0]
	}
	return NewWithAuth(ready, nil)
}

func NewWithAuth(ready handlers.Readiness, authentication *auth.HTTP) http.Handler {
	return NewWithNodes(ready, authentication, nil)
}

func NewWithNodes(ready handlers.Readiness, authentication *auth.HTTP, nodes *handlers.Nodes, providerAPI ...*handlers.Providers) http.Handler {
	h := handlers.Bootstrap{}
	h.Ready = ready
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	if authentication != nil {
		r.Use(authentication.Middleware)
	}
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte("Hello World\n"))
	})
	version := handlers.NewVersion()
	versionUpdate := handlers.NewVersionUpdate()
	r.Get("/api/version", version.Get)
	r.Head("/api/version", withoutBody(version.Get))
	r.Options("/api/version", options("GET, HEAD, OPTIONS"))
	r.Post("/api/version/update", versionUpdate.Post)
	r.Options("/api/version/update", options("POST, OPTIONS"))
	r.Get("/api/health", h.Health)
	r.Options("/api/health", h.HealthOptions)
	r.Get("/api/ready", h.Readiness)
	r.Get("/api/init", h.Init)
	r.Post("/api/locale", h.Locale)
	r.Get("/api/tags", handlers.Tags)
	r.Head("/api/tags", withoutBody(handlers.Tags))
	r.Options("/api/tags", handlers.TagsOptions)
	// Next.js fornece HEAD para GET e OPTIONS/Allow para métodos exportados.
	r.Head("/api/health", withoutBody(h.Health))
	r.Head("/api/ready", withoutBody(h.Readiness))
	r.Head("/api/init", withoutBody(h.Init))
	r.Options("/api/ready", options("GET, HEAD, OPTIONS"))
	r.Options("/api/init", options("GET, HEAD, OPTIONS"))
	r.Options("/api/locale", options("OPTIONS, POST"))
	if authentication != nil {
		r.Post("/api/auth/login", authentication.Login)
		r.Post("/api/auth/logout", authentication.Logout)
		r.Get("/api/auth/status", authentication.Status)
		r.Head("/api/auth/status", withoutBody(authentication.Status))
		// auth/check aplica sua própria barreira antes de reproduzir a resposta do handler.
		r.Get("/api/auth/check", authentication.Check)
		r.Head("/api/auth/check", withoutBody(authentication.Check))
		// Local-only do dashboardGuard; token CLI ainda não implementado.
		r.Post("/api/auth/reset-password", authentication.LocalOnlyGuard(authentication.ResetPassword))
		r.Options("/api/auth/reset-password", authentication.LocalOnlyGuard(options("POST, OPTIONS")))
		r.Get("/api/settings", authentication.SettingsGuard(authentication.SettingsGet))
		r.Head("/api/settings", withoutBody(authentication.SettingsGuard(authentication.SettingsGet)))
		r.Patch("/api/settings", authentication.SettingsGuard(authentication.SettingsPatch))
		r.Options("/api/settings", authentication.SettingsGuard(options("GET, HEAD, PATCH, OPTIONS")))
		r.Route("/api/keys", func(r chi.Router) {
			r.Get("/", authentication.APIKeysGet)
			r.Post("/", authentication.APIKeysPost)
			r.Get("/{id}", authentication.APIKeyGet)
			r.Put("/{id}", authentication.APIKeyPut)
			r.Delete("/{id}", authentication.APIKeyDelete)
		})
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Models != nil {
			models := providerAPI[0].Models
			r.Get("/api/models", models.List)
			r.Head("/api/models", withoutBody(models.List))
			r.Put("/api/models", models.SetAlias)
			r.Options("/api/models", options("GET, HEAD, OPTIONS, PUT"))
			r.Get("/api/models/disabled", models.DisabledGet)
			r.Head("/api/models/disabled", withoutBody(models.DisabledGet))
			r.Post("/api/models/disabled", models.DisabledPost)
			r.Delete("/api/models/disabled", models.DisabledDelete)
			r.Options("/api/models/disabled", options("DELETE, GET, HEAD, OPTIONS, POST"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Aliases != nil {
			aliases := providerAPI[0].Aliases
			r.Get("/api/models/alias", aliases.List)
			r.Head("/api/models/alias", withoutBody(aliases.List))
			r.Put("/api/models/alias", aliases.Set)
			r.Delete("/api/models/alias", aliases.Delete)
			r.Options("/api/models/alias", options("DELETE, GET, HEAD, OPTIONS, PUT"))
			r.Get("/api/models/custom", aliases.CustomList)
			r.Head("/api/models/custom", withoutBody(aliases.CustomList))
			r.Post("/api/models/custom", aliases.CustomAdd)
			r.Delete("/api/models/custom", aliases.CustomDelete)
			r.Options("/api/models/custom", options("DELETE, GET, HEAD, OPTIONS, POST"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Headroom != nil {
			headroom := providerAPI[0].Headroom
			r.Get("/api/headroom/status", authentication.LocalOnlyGuard(headroom.Get))
			r.Head("/api/headroom/status", withoutBody(authentication.LocalOnlyGuard(headroom.Get)))
			r.Options("/api/headroom/status", authentication.LocalOnlyGuard(options("GET, HEAD, OPTIONS")))
			r.Post("/api/headroom/start", authentication.LocalOnlyGuard(headroom.Start))
			r.Options("/api/headroom/start", authentication.LocalOnlyGuard(options("POST, OPTIONS")))
			r.Post("/api/headroom/stop", authentication.LocalOnlyGuard(headroom.Stop))
			r.Options("/api/headroom/stop", authentication.LocalOnlyGuard(options("POST, OPTIONS")))
			r.Post("/api/headroom/restart", authentication.LocalOnlyGuard(headroom.Restart))
			r.Options("/api/headroom/restart", authentication.LocalOnlyGuard(options("POST, OPTIONS")))
			r.Get("/api/headroom/extras", authentication.LocalOnlyGuard(headroom.ExtrasGet))
			r.Head("/api/headroom/extras", withoutBody(authentication.LocalOnlyGuard(headroom.ExtrasGet)))
			r.Post("/api/headroom/extras", authentication.LocalOnlyGuard(headroom.ExtrasPost))
			r.Delete("/api/headroom/extras", authentication.LocalOnlyGuard(headroom.ExtrasDelete))
			r.Options("/api/headroom/extras", authentication.LocalOnlyGuard(options("GET, HEAD, DELETE, OPTIONS, POST")))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Usage != nil {
			usage := providerAPI[0].Usage
			r.Get("/api/usage/history", usage.History)
			r.Head("/api/usage/history", withoutBody(usage.History))
			r.Options("/api/usage/history", options("GET, HEAD, OPTIONS"))
			r.Get("/api/usage/chart", usage.Chart)
			r.Head("/api/usage/chart", withoutBody(usage.Chart))
			r.Options("/api/usage/chart", options("GET, HEAD, OPTIONS"))
			r.Get("/api/usage/providers", usage.Providers)
			r.Head("/api/usage/providers", withoutBody(usage.Providers))
			r.Options("/api/usage/providers", options("GET, HEAD, OPTIONS"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Logs != nil {
			logs := providerAPI[0].Logs
			for _, path := range []string{"/api/usage/logs", "/api/usage/request-logs"} {
				r.Get(path, logs.RecentLogs)
				r.Head(path, withoutBody(logs.RecentLogs))
				r.Options(path, options("GET, HEAD, OPTIONS"))
			}
			r.Get("/api/usage/request-details", logs.RequestDetails)
			r.Head("/api/usage/request-details", withoutBody(logs.RequestDetails))
			r.Options("/api/usage/request-details", options("GET, HEAD, OPTIONS"))
			r.Get("/api/usage/request-details/{id}", logs.RequestDetailByID)
			r.Head("/api/usage/request-details/{id}", withoutBody(logs.RequestDetailByID))
			r.Options("/api/usage/request-details/{id}", options("GET, HEAD, OPTIONS"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Pricing != nil {
			pricing := providerAPI[0].Pricing
			r.Get("/api/pricing", pricing.Get)
			r.Head("/api/pricing", withoutBody(pricing.Get))
			r.Patch("/api/pricing", pricing.Patch)
			r.Delete("/api/pricing", pricing.Delete)
			r.Options("/api/pricing", options("DELETE, GET, HEAD, OPTIONS, PATCH"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Shutdown != nil {
			r.Post("/api/shutdown", providerAPI[0].Shutdown.Post)
			r.Options("/api/shutdown", options("OPTIONS, POST"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Overrides != nil {
			overrides := providerAPI[0].Overrides
			r.Get("/api/providers/{id}/overrides", overrides.Get)
			r.Head("/api/providers/{id}/overrides", withoutBody(overrides.Get))
			r.Put("/api/providers/{id}/overrides", overrides.Put)
			r.Options("/api/providers/{id}/overrides", options("GET, HEAD, OPTIONS, PUT"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Database != nil {
			dbExport := providerAPI[0].Database
			r.Get("/api/settings/database", dbExport.Get)
			r.Head("/api/settings/database", withoutBody(dbExport.Get))
			r.Post("/api/settings/database", dbExport.Post)
			r.Options("/api/settings/database", options("GET, HEAD, OPTIONS, POST"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil {
			r.Post("/api/settings/proxy-test", handlers.ProxyTestSettings{}.Post)
			r.Options("/api/settings/proxy-test", options("OPTIONS, POST"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Media != nil {
			media := providerAPI[0].Media
			r.Get("/api/media-providers/tts/voices", media.Get)
			r.Head("/api/media-providers/tts/voices", withoutBody(media.Get))
			r.Options("/api/media-providers/tts/voices", options("GET, HEAD, OPTIONS"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].TTSVoices != nil {
			ttsVoices := providerAPI[0].TTSVoices
			r.Get("/api/media-providers/tts/deepgram/voices", ttsVoices.Deepgram)
			r.Head("/api/media-providers/tts/deepgram/voices", withoutBody(ttsVoices.Deepgram))
			r.Options("/api/media-providers/tts/deepgram/voices", options("GET, HEAD, OPTIONS"))
			r.Get("/api/media-providers/tts/elevenlabs/voices", ttsVoices.ElevenLabs)
			r.Head("/api/media-providers/tts/elevenlabs/voices", withoutBody(ttsVoices.ElevenLabs))
			r.Options("/api/media-providers/tts/elevenlabs/voices", options("GET, HEAD, OPTIONS"))
			r.Get("/api/media-providers/tts/inworld/voices", ttsVoices.Inworld)
			r.Head("/api/media-providers/tts/inworld/voices", withoutBody(ttsVoices.Inworld))
			r.Options("/api/media-providers/tts/inworld/voices", options("GET, HEAD, OPTIONS"))
			r.Get("/api/media-providers/tts/minimax/voices", ttsVoices.MiniMax)
			r.Head("/api/media-providers/tts/minimax/voices", withoutBody(ttsVoices.MiniMax))
			r.Options("/api/media-providers/tts/minimax/voices", options("GET, HEAD, OPTIONS"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].OAuth != nil {
			oauthAPI := providerAPI[0].OAuth
			r.Post("/api/oauth/iflow/cookie", oauthAPI.IflowCookie)
			r.Options("/api/oauth/iflow/cookie", options("OPTIONS, POST"))
			r.Post("/api/oauth/zed/import", oauthAPI.ZedImport)
			r.Options("/api/oauth/zed/import", options("OPTIONS, POST"))
			r.Get("/api/oauth/cursor/import", handlers.CursorInstructions)
			r.Head("/api/oauth/cursor/import", withoutBody(handlers.CursorInstructions))
			r.Post("/api/oauth/cursor/import", oauthAPI.CursorImport)
			r.Options("/api/oauth/cursor/import", options("GET, HEAD, OPTIONS, POST"))
			r.Post("/api/oauth/kiro/api-key", oauthAPI.KiroAPIKey)
			r.Options("/api/oauth/kiro/api-key", options("OPTIONS, POST"))
			r.Post("/api/oauth/grok-cli/bulk-import", oauthAPI.GrokBulkImport)
			r.Options("/api/oauth/grok-cli/bulk-import", options("OPTIONS, POST"))
			r.Post("/api/oauth/codex/bulk-import", oauthAPI.CodexBulkImport)
			r.Options("/api/oauth/codex/bulk-import", options("OPTIONS, POST"))
			r.Post("/api/oauth/codex/import-token", oauthAPI.CodexImportToken)
			r.Options("/api/oauth/codex/import-token", options("OPTIONS, POST"))
			r.Post("/api/oauth/gitlab/pat", oauthAPI.GitlabPAT)
			r.Options("/api/oauth/gitlab/pat", options("OPTIONS, POST"))
			r.Get("/api/oauth/{provider}/{action}", oauthAPI.Route)
			r.Post("/api/oauth/{provider}/{action}", oauthAPI.Route)
			r.Options("/api/oauth/{provider}/{action}", options("GET, HEAD, OPTIONS, POST"))
			r.Head("/api/oauth/{provider}/{action}", withoutBody(oauthAPI.Route))
		}
		r.Get("/api/auth/oidc/start", authentication.OIDCStart)
		r.Head("/api/auth/oidc/start", withoutBody(authentication.OIDCStart))
		r.Get("/api/auth/oidc/callback", authentication.OIDCCallback)
		r.Post("/api/auth/oidc/test", authentication.OIDCTest)
	}
	if authentication != nil && nodes != nil {
		providerHandlers := handlers.Providers{Connections: nodes.Connections, Nodes: nodes.Store}
		if nodes.Connections != nil {
			r.Get("/api/providers", providerHandlers.List)
			r.Head("/api/providers", withoutBody(providerHandlers.List))
		}
		validator := handlers.NodeValidation{Local: auth.IsLocalRequest}
		r.Post("/api/provider-nodes/validate", validator.Validate)
		r.Options("/api/provider-nodes/validate", options("OPTIONS, POST"))
		r.Get("/api/provider-nodes", nodes.List)
		r.Head("/api/provider-nodes", withoutBody(nodes.List))
		r.Post("/api/provider-nodes", nodes.Create)
		r.Options("/api/provider-nodes", options("GET, HEAD, OPTIONS, POST"))
		if nodes.Mutations != nil && nodes.Connections != nil {
			r.Put("/api/provider-nodes/{id}", nodes.Update)
			r.Delete("/api/provider-nodes/{id}", nodes.Delete)
			r.Options("/api/provider-nodes/{id}", options("DELETE, OPTIONS, PUT"))
		}
	}
	if authentication != nil && len(providerAPI) > 0 && providerAPI[0] != nil {
		if combos := providerAPI[0].Combos; combos != nil {
			r.Get("/api/combos", combos.List)
			r.Head("/api/combos", withoutBody(combos.List))
			r.Post("/api/combos", combos.Create)
			r.Options("/api/combos", options("GET, HEAD, OPTIONS, POST"))
			r.Get("/api/combos/{id}", combos.Get)
			r.Head("/api/combos/{id}", withoutBody(combos.Get))
			r.Put("/api/combos/{id}", combos.Update)
			r.Delete("/api/combos/{id}", combos.Delete)
			r.Options("/api/combos/{id}", options("DELETE, GET, HEAD, OPTIONS, PUT"))
		}
		if pools := providerAPI[0].Pools; pools != nil {
			r.Get("/api/proxy-pools", pools.List)
			r.Head("/api/proxy-pools", withoutBody(pools.List))
			r.Post("/api/proxy-pools", pools.Create)
			r.Options("/api/proxy-pools", options("GET, HEAD, OPTIONS, POST"))
			r.Post("/api/proxy-pools/{id}/test", pools.Test(handlers.ProxyProbe{}))
			r.Options("/api/proxy-pools/{id}/test", options("OPTIONS, POST"))
			r.Get("/api/proxy-pools/{id}", pools.Get)
			r.Head("/api/proxy-pools/{id}", withoutBody(pools.Get))
			r.Put("/api/proxy-pools/{id}", pools.Update)
			r.Delete("/api/proxy-pools/{id}", pools.Delete)
			r.Options("/api/proxy-pools/{id}", options("DELETE, GET, HEAD, OPTIONS, PUT"))
		}
		r.Post("/api/providers", providerAPI[0].Create)
		if providerAPI[0].CreateBulk != nil {
			r.Post("/api/providers/bulk", providerAPI[0].Bulk)
			r.Options("/api/providers/bulk", options("OPTIONS, POST"))
		}
		r.Options("/api/providers", options("GET, HEAD, OPTIONS, POST"))
		if providerAPI[0].GetConnection != nil && providerAPI[0].UpdateConnection != nil && providerAPI[0].DeleteConnection != nil {
			r.Get("/api/providers/{id}", providerAPI[0].Get)
			r.Head("/api/providers/{id}", withoutBody(providerAPI[0].Get))
			r.Put("/api/providers/{id}", providerAPI[0].Update)
			r.Delete("/api/providers/{id}", providerAPI[0].Delete)
			r.Options("/api/providers/{id}", options("DELETE, GET, HEAD, OPTIONS, PUT"))
		}
	}
	return r
}

func options(allow string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", allow)
		w.WriteHeader(http.StatusNoContent)
	}
}

type bodyless struct{ http.ResponseWriter }

func (w bodyless) Write(p []byte) (int, error) { return len(p), nil }
func withoutBody(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { handler(bodyless{w}, r) }
}
