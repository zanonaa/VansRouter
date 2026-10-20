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
	r.Get("/api/health", h.Health)
	r.Options("/api/health", h.HealthOptions)
	r.Get("/api/ready", h.Readiness)
	r.Get("/api/init", h.Init)
	r.Post("/api/locale", h.Locale)
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
