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
	return NewWithSPA(ready, authentication, nodes, providerAPI, "")
}

// NewWithSPA monta o roteador; spaDir não vazio serve o build da SPA pela
// mesma origem (Fase 4B): index apenas nas rotas da SPA, assets públicos e
// 404 para caminhos fora do app.
func NewWithSPA(ready handlers.Readiness, authentication *auth.HTTP, nodes *handlers.Nodes, providerAPI []*handlers.Providers, spaDir string) http.Handler {
	h := handlers.Bootstrap{}
	h.Ready = ready
	r := chi.NewRouter()
	r.Use(legacyRewrites)
	r.Use(middleware.Recoverer)
	if authentication != nil {
		r.Use(authentication.Middleware)
	}
	if spaDir != "" {
		spa := &spaServer{root: spaDir}
		r.Get("/", spa.serveIndex)
		r.NotFound(spa.ServeHTTP)
	} else {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write([]byte("Hello World\n"))
		})
		r.NotFound(http.NotFound)
	}
	version := handlers.NewVersion()
	versionUpdate := handlers.NewVersionUpdate()
	versionShutdown := handlers.NewVersionShutdown()
	v1betaModels := &handlers.V1BetaModels{}
	r.Get("/api/version", version.Get)
	r.Head("/api/version", withoutBody(version.Get))
	r.Options("/api/version", options("GET, HEAD, OPTIONS"))
	r.Post("/api/version/update", versionUpdate.Post)
	r.Options("/api/version/update", options("POST, OPTIONS"))
	r.Post("/api/version/shutdown", versionShutdown.Post)
	r.Options("/api/version/shutdown", options("POST, OPTIONS"))
	r.Get("/v1beta/models", v1betaModels.Get)
	r.Head("/v1beta/models", withoutBody(v1betaModels.Get))
	r.Options("/v1beta/models", options("GET, HEAD, OPTIONS"))
	r.Get("/api/v1beta/models", v1betaModels.Get)
	r.Head("/api/v1beta/models", withoutBody(v1betaModels.Get))
	r.Options("/api/v1beta/models", options("GET, HEAD, OPTIONS"))
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
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].TranslatorFiles != nil {
			files := providerAPI[0].TranslatorFiles
			r.Get("/api/translator/load", files.Load)
			r.Head("/api/translator/load", withoutBody(files.Load))
			r.Options("/api/translator/load", options("GET, HEAD, OPTIONS"))
			r.Post("/api/translator/save", files.Save)
			r.Options("/api/translator/save", options("POST, OPTIONS"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Circuits != nil {
			circuits := providerAPI[0].Circuits
			r.Get("/api/providers/circuit-breakers", circuits.GetHTTP)
			r.Head("/api/providers/circuit-breakers", withoutBody(circuits.GetHTTP))
			r.Options("/api/providers/circuit-breakers", options("GET, HEAD, OPTIONS"))
			r.Post("/api/providers/circuit-breakers/{name}/reset", authentication.SettingsGuard(circuits.ResetHTTP))
			r.Options("/api/providers/circuit-breakers/{name}/reset", options("POST, OPTIONS"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Validation != nil {
			validation := providerAPI[0].Validation
			r.Post("/api/providers/validate", validation.Post)
			r.Options("/api/providers/validate", options("POST, OPTIONS"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Suggestions != nil {
			suggestions := providerAPI[0].Suggestions
			r.Get("/api/providers/suggested-models", suggestions.Get)
			r.Head("/api/providers/suggested-models", withoutBody(suggestions.Get))
			r.Options("/api/providers/suggested-models", options("GET, HEAD, OPTIONS"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].ClientList != nil {
			clientList := providerAPI[0].ClientList
			r.Get("/api/providers/client", clientList.Get)
			r.Head("/api/providers/client", withoutBody(clientList.Get))
			r.Options("/api/providers/client", options("GET, HEAD, OPTIONS"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].KiloFree != nil {
			kilo := providerAPI[0].KiloFree
			r.Get("/api/providers/kilo/free-models", kilo.Get)
			r.Head("/api/providers/kilo/free-models", withoutBody(kilo.Get))
			r.Options("/api/providers/kilo/free-models", options("GET, HEAD, OPTIONS"))
		}
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
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].ModelAvailability != nil {
			availability := providerAPI[0].ModelAvailability
			r.Get("/api/models/availability", availability.Get)
			r.Head("/api/models/availability", withoutBody(availability.Get))
			r.Post("/api/models/availability", availability.Post)
			r.Options("/api/models/availability", options("GET, HEAD, OPTIONS, POST"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].CatalogSync != nil {
			catalogSync := providerAPI[0].CatalogSync
			r.Get("/api/models/catalog-sync", catalogSync.Get)
			r.Head("/api/models/catalog-sync", withoutBody(catalogSync.Get))
			r.Post("/api/models/catalog-sync", catalogSync.Post)
			r.Options("/api/models/catalog-sync", options("GET, HEAD, OPTIONS, POST"))
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
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Pxpipe != nil {
			pxpipe := providerAPI[0].Pxpipe
			r.Post("/api/pxpipe/health", pxpipe.Health)
			r.Get("/api/pxpipe/health", pxpipe.Health)
			r.Head("/api/pxpipe/health", withoutBody(pxpipe.Health))
			r.Options("/api/pxpipe/health", options("GET, HEAD, OPTIONS, POST"))
			r.Post("/api/pxpipe/install", pxpipe.Install)
			r.Options("/api/pxpipe/install", options("POST, OPTIONS"))
			r.Get("/api/pxpipe/status", pxpipe.Status)
			r.Head("/api/pxpipe/status", withoutBody(pxpipe.Status))
			r.Options("/api/pxpipe/status", options("GET, HEAD, OPTIONS"))
			r.Post("/api/pxpipe/start", pxpipe.Start)
			r.Options("/api/pxpipe/start", options("POST, OPTIONS"))
			r.Post("/api/pxpipe/stop", pxpipe.Stop)
			r.Options("/api/pxpipe/stop", options("POST, OPTIONS"))
			r.Post("/api/pxpipe/restart", pxpipe.Restart)
			r.Options("/api/pxpipe/restart", options("POST, OPTIONS"))
			r.Get("/api/pxpipe/logs", pxpipe.Logs)
			r.Head("/api/pxpipe/logs", withoutBody(pxpipe.Logs))
			r.Options("/api/pxpipe/logs", options("GET, HEAD, OPTIONS"))
			r.Get("/api/pxpipe/stats", pxpipe.Stats)
			r.Head("/api/pxpipe/stats", withoutBody(pxpipe.Stats))
			r.Options("/api/pxpipe/stats", options("GET, HEAD, OPTIONS"))
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
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].LiveUsage != nil {
			r.Get("/api/usage/stream", providerAPI[0].LiveUsage.Stream)
			r.Options("/api/usage/stream", options("GET, HEAD, OPTIONS"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Logs != nil {
			logs := providerAPI[0].Logs
			r.Get("/api/usage/logs", logs.RecentLogs)
			r.Head("/api/usage/logs", withoutBody(logs.RecentLogs))
			r.Options("/api/usage/logs", options("GET, HEAD, OPTIONS"))
			r.Get("/api/usage/request-logs", logs.RecentLogs)
			r.Head("/api/usage/request-logs", withoutBody(logs.RecentLogs))
			r.Options("/api/usage/request-logs", options("GET, HEAD, OPTIONS"))
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
			cursorAuto := providerAPI[0].CursorAutoImport
			if cursorAuto == nil {
				cursorAuto = handlers.NewCursorAutoImport()
			}
			kiroAuto := providerAPI[0].KiroAutoImport
			if kiroAuto == nil {
				kiroAuto = handlers.NewKiroAutoImport()
			}
			zedAuto := providerAPI[0].ZedAutoImport
			if zedAuto == nil {
				zedAuto = handlers.NewZedAutoImport()
			}
			r.Get("/api/oauth/cursor/auto-import", cursorAuto.Get)
			r.Head("/api/oauth/cursor/auto-import", withoutBody(cursorAuto.Get))
			r.Options("/api/oauth/cursor/auto-import", options("GET, HEAD, OPTIONS"))
			r.Get("/api/oauth/kiro/auto-import", kiroAuto.Get)
			r.Head("/api/oauth/kiro/auto-import", withoutBody(kiroAuto.Get))
			r.Options("/api/oauth/kiro/auto-import", options("GET, HEAD, OPTIONS"))
			r.Get("/api/oauth/zed/auto-import", zedAuto.Get)
			r.Head("/api/oauth/zed/auto-import", withoutBody(zedAuto.Get))
			r.Options("/api/oauth/zed/auto-import", options("GET, HEAD, OPTIONS"))
			r.Post("/api/oauth/iflow/cookie", oauthAPI.IflowCookie)
			r.Options("/api/oauth/iflow/cookie", options("OPTIONS, POST"))
			r.Post("/api/oauth/zed/import", oauthAPI.ZedImport)
			r.Options("/api/oauth/zed/import", options("OPTIONS, POST"))
			r.Get("/api/oauth/cursor/import", handlers.CursorInstructions)
			r.Head("/api/oauth/cursor/import", withoutBody(handlers.CursorInstructions))
			r.Post("/api/oauth/cursor/import", oauthAPI.CursorImport)
			r.Options("/api/oauth/cursor/import", options("GET, HEAD, OPTIONS, POST"))
			r.Get("/api/oauth/kiro/social-authorize", oauthAPI.KiroSocialAuthorize)
			r.Head("/api/oauth/kiro/social-authorize", withoutBody(oauthAPI.KiroSocialAuthorize))
			r.Options("/api/oauth/kiro/social-authorize", options("GET, HEAD, OPTIONS"))
			r.Post("/api/oauth/kiro/social-exchange", oauthAPI.KiroSocialExchange)
			r.Options("/api/oauth/kiro/social-exchange", options("OPTIONS, POST"))
			r.Post("/api/oauth/kiro/import", oauthAPI.KiroImport)
			r.Options("/api/oauth/kiro/import", options("OPTIONS, POST"))
			r.Post("/api/oauth/kiro/import-cli-proxy", oauthAPI.KiroCLIImport)
			r.Options("/api/oauth/kiro/import-cli-proxy", options("OPTIONS, POST"))
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
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].PoolFitness != nil {
			poolFitness := providerAPI[0].PoolFitness
			r.Get("/api/proxy-pools/fitness", poolFitness.Get)
			r.Head("/api/proxy-pools/fitness", withoutBody(poolFitness.Get))
			r.Options("/api/proxy-pools/fitness", options("GET, HEAD, OPTIONS"))
			r.Post("/api/proxy-pools/fitness/clear-all", poolFitness.ClearAll)
			r.Options("/api/proxy-pools/fitness/clear-all", options("POST, OPTIONS"))
			r.Post("/api/proxy-pools/{id}/fitness/clear", poolFitness.ClearByID)
			r.Options("/api/proxy-pools/{id}/fitness/clear", options("POST, OPTIONS"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].ImportPublic != nil {
			importPublic := providerAPI[0].ImportPublic
			r.Post("/api/proxy-pools/import-public", importPublic.Post)
			r.Options("/api/proxy-pools/import-public", options("POST, OPTIONS"))
		}
		if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Deploys != nil {
			deploys := providerAPI[0].Deploys
			r.Post("/api/proxy-pools/cloudflare-deploy", deploys.CloudflareDeploy)
			r.Options("/api/proxy-pools/cloudflare-deploy", options("POST, OPTIONS"))
			r.Post("/api/proxy-pools/deno-deploy", deploys.DenoDeploy)
			r.Options("/api/proxy-pools/deno-deploy", options("POST, OPTIONS"))
			r.Post("/api/proxy-pools/vercel-deploy", deploys.VercelDeploy)
			r.Options("/api/proxy-pools/vercel-deploy", options("POST, OPTIONS"))
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
	if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].ModelPing != nil {
		r.Post("/api/models/test", providerAPI[0].ModelPing.Post)
		r.Options("/api/models/test", options("POST, OPTIONS"))
	}
	if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].GCPProjects != nil {
		projects := providerAPI[0].GCPProjects
		r.Get("/api/providers/{id}/gcp-projects", projects.Get)
		r.Head("/api/providers/{id}/gcp-projects", withoutBody(projects.Get))
		r.Options("/api/providers/{id}/gcp-projects", options("GET, HEAD, OPTIONS"))
	}
	if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Discovery != nil {
		discovery := providerAPI[0].Discovery
		r.Get("/api/providers/{id}/models", discovery.Get)
		r.Head("/api/providers/{id}/models", withoutBody(discovery.Get))
		r.Options("/api/providers/{id}/models", options("GET, HEAD, OPTIONS"))
	}
	if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].ConnectionProbe != nil {
		r.Post("/api/providers/test-batch", providerAPI[0].ConnectionProbe.Batch)
		r.Options("/api/providers/test-batch", options("POST, OPTIONS"))
		r.Post("/api/providers/{id}/test", providerAPI[0].ConnectionProbe.Post)
		r.Options("/api/providers/{id}/test", options("POST, OPTIONS"))
	}
	if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].ModelTests != nil {
		r.Post("/api/providers/{id}/test-models", providerAPI[0].ModelTests.Post)
		r.Options("/api/providers/{id}/test-models", options("POST, OPTIONS"))
	}
	if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Console != nil {
		console := providerAPI[0].Console
		r.Get("/api/translator/console-logs", console.Get)
		r.Head("/api/translator/console-logs", withoutBody(console.Get))
		r.Options("/api/translator/console-logs", options("GET, HEAD, OPTIONS"))
		r.Delete("/api/translator/console-logs", console.Delete)
		r.Options("/api/translator/console-logs", options("DELETE, OPTIONS"))
		r.Get("/api/translator/console-logs/stream", console.Stream)
		r.Head("/api/translator/console-logs/stream", withoutBody(console.Stream))
		r.Options("/api/translator/console-logs/stream", options("GET, HEAD, OPTIONS"))
	}
	if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].TranslatorSend != nil {
		r.Post("/api/translator/send", providerAPI[0].TranslatorSend.Send)
		r.Options("/api/translator/send", options("POST, OPTIONS"))
	}
	if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Translator != nil {
		r.Post("/api/translator/translate", providerAPI[0].Translator.Post)
		r.Options("/api/translator/translate", options("POST, OPTIONS"))
	}
	if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].UsageContracts != nil {
		usageContracts := providerAPI[0].UsageContracts
		r.Post("/api/usage/{connectionId}/claude-reset", usageContracts.ClaudeReset)
		r.Get("/api/usage/{connectionId}/codex-reset-credits", usageContracts.CodexCredits)
		r.Head("/api/usage/{connectionId}/codex-reset-credits", withoutBody(usageContracts.CodexCredits))
		r.Post("/api/usage/{connectionId}/codex-reset-credits", usageContracts.CodexCredits)
		r.Get("/api/usage/{connectionId}", usageContracts.Get)
		r.Head("/api/usage/{connectionId}", withoutBody(usageContracts.Get))
		r.Get("/api/usage/stats", usageContracts.Stats)
		r.Head("/api/usage/stats", withoutBody(usageContracts.Stats))
	}
	r.Options("/api/v1/api/chat", options("POST, OPTIONS"))
	r.Options("/api/v1/audio/speech", options("POST, OPTIONS"))
	r.Options("/api/v1/audio/transcriptions", options("POST, OPTIONS"))
	if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].AudioVoices != nil {
		voices := providerAPI[0].AudioVoices
		r.Get("/api/v1/audio/voices", voices.Get)
		r.Head("/api/v1/audio/voices", withoutBody(voices.Get))
		r.Options("/api/v1/audio/voices", handlers.AudioVoicesOptions)
	}
	if len(providerAPI) > 0 && providerAPI[0] != nil && providerAPI[0].Gateway != nil {
		gateway := providerAPI[0].Gateway
		r.Options("/api/v1/chat/completions", handlers.GatewayModelsOptions)
		r.Options("/api/v1/embeddings", handlers.GatewayModelsOptions)
		r.Options("/api/v1/images/generations", handlers.GatewayModelsOptions)
		r.Post("/api/v1/api/chat", gateway.OpenAIChat)
		r.Post("/api/v1/audio/speech", gateway.Speech)
		r.Post("/api/v1/audio/transcriptions", gateway.Transcriptions)
		r.Post("/api/v1/chat/completions", gateway.OpenAIChat)
		r.Post("/api/v1/messages/count_tokens", gateway.CountTokens)
		r.Post("/api/v1/messages", gateway.OpenAIChat)
		r.Post("/api/v1/responses/compact", gateway.Responses)
		r.Post("/api/v1/responses", gateway.Responses)
		r.Post("/api/v1/search", gateway.Search)
		r.Get("/api/v1/videos/{id}", gateway.Video)
		r.Head("/api/v1/videos/{id}", withoutBody(gateway.Video))
		r.Post("/api/v1/videos/edits", gateway.Video)
		r.Post("/api/v1/videos/extensions", gateway.Video)
		r.Post("/api/v1/videos/generations", gateway.Video)
		r.Post("/api/v1/web/fetch", gateway.Fetch)
		r.Post("/api/v1beta/models/{...path}", gateway.GeminiNative)
		r.Post("/api/v1/embeddings", gateway.Embeddings)
		r.Post("/api/v1/images/generations", gateway.Images)
		r.Get("/api/v1/models", gateway.Models)
		r.Head("/api/v1/models", withoutBody(gateway.Models))
		r.Options("/api/v1/models", handlers.GatewayModelsOptions)
		r.Get("/api/v1/models/info", gateway.ModelInfo)
		r.Head("/api/v1/models/info", withoutBody(gateway.ModelInfo))
		r.Options("/api/v1/models/info", handlers.GatewayModelsOptions)
		r.Get("/api/v1/models/{kind}", gateway.ModelsByKind)
		r.Head("/api/v1/models/{kind}", withoutBody(gateway.ModelsByKind))
		r.Options("/api/v1/models/{kind}", handlers.GatewayModelsOptions)
	}
	r.Options("/api/v1/messages/count_tokens", options("POST, OPTIONS"))
	r.Options("/api/v1/messages", options("POST, OPTIONS"))
	r.Options("/api/v1/responses/compact", options("POST, OPTIONS"))
	r.Options("/api/v1/responses", options("POST, OPTIONS"))
	r.Options("/api/v1/search", options("POST, OPTIONS"))
	r.Options("/api/v1/videos/{id}", options("GET, HEAD, OPTIONS"))
	r.Options("/api/v1/videos/edits", options("POST, OPTIONS"))
	r.Options("/api/v1/videos/extensions", options("POST, OPTIONS"))
	r.Options("/api/v1/videos/generations", options("POST, OPTIONS"))
	r.Options("/api/v1/web/fetch", options("POST, OPTIONS"))
	r.Options("/api/v1beta/models/{...path}", options("POST, OPTIONS"))
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
