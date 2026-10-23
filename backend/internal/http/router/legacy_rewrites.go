package router

import (
	"net/http"
	"strings"
)

// legacyRewrites reproduz os rewrites do next.config.mjs da origem Node.
//
// Clientes OpenAI-compatíveis chamam /v1/chat/completions, /v1/models etc.;
// o next.config.mjs reescrevia esses caminhos para /api/v1/*. O mesmo valia
// para /codex/* e /responses (Codex CLI) e para /v1beta/* (formato Gemini).
// Sem isso, quem usava o endpoint público recebia 404 do roteador, mesmo com
// os handlers existindo — foi a regressão observada no Dev em 2026-10-23.
//
// O middleware roda antes do roteamento (e da autenticação), então o restante
// do servidor enxerga sempre o caminho canônico /api/v1…, exatamente como o
// roteador do Next entregava ao handler.
func legacyRewrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if target, ok := legacyPath(r.URL.Path); ok {
			r = r.Clone(r.Context())
			r.URL.Path = target
			r.URL.RawPath = ""
		}
		next.ServeHTTP(w, r)
	})
}

// legacyPath mapeia um caminho público para o canônico. A ordem importa:
// /v1/v1/* precisa ser tratado antes de /v1/*.
func legacyPath(path string) (string, bool) {
	switch {
	case path == "/v1" || path == "/v1/":
		return "/api/v1", true
	case path == "/v1/v1" || path == "/v1/v1/":
		return "/api/v1", true
	case strings.HasPrefix(path, "/v1/v1/"):
		return "/api/v1/" + strings.TrimPrefix(path, "/v1/v1/"), true
	case strings.HasPrefix(path, "/v1/"):
		return "/api/v1/" + strings.TrimPrefix(path, "/v1/"), true
	case path == "/codex" || strings.HasPrefix(path, "/codex/"):
		// O rewrite do Next manda qualquer subcaminho para o endpoint de
		// responses; o corpo é o mesmo, só o path muda.
		return "/api/v1/responses", true
	case path == "/responses":
		return "/api/v1/responses", true
	case path == "/v1beta" || path == "/v1beta/":
		return "/api/v1beta", true
	case strings.HasPrefix(path, "/v1beta/"):
		return "/api/v1beta/" + strings.TrimPrefix(path, "/v1beta/"), true
	}
	return "", false
}
