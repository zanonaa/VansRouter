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

func NewWithNodes(ready handlers.Readiness, authentication *auth.HTTP, nodes *handlers.Nodes) http.Handler {
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
		r.Get("/api/auth/oidc/start", authentication.OIDCStart)
		r.Head("/api/auth/oidc/start", withoutBody(authentication.OIDCStart))
		r.Get("/api/auth/oidc/callback", authentication.OIDCCallback)
		r.Post("/api/auth/oidc/test", authentication.OIDCTest)
	}
	if authentication != nil && nodes != nil {
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
