// Package router — servidor estático da SPA pela mesma origem (Fase 4B).
// Fallback para index.html APENAS nas rotas da SPA; caminhos de API
// inexistentes e rotas fora da SPA continuam 404, nunca HTML.
package router

import (
	"bytes"
	"net/http"
	"path"
	"strings"
)

type spaServer struct {
	root string
}

// Rotas de página da SPA (App Router): fallback válido para index.html.
// Nada de /api/*, /v1*, /codex* ou caminhos fora do app.
var spaRoutes = []string{"/login", "/masuk", "/callback", "/landing", "/dashboard"}

func isSPARoute(urlPath string) bool {
	if urlPath == "/" {
		return true
	}
	for _, prefix := range spaRoutes {
		if urlPath == prefix || strings.HasPrefix(urlPath, prefix+"/") {
			return true
		}
	}
	return false
}

func (s *spaServer) serveIndex(w http.ResponseWriter, r *http.Request) {
	index, err := readSPAFile(s.root, "index.html")
	if err != nil {
		http.Error(w, "SPA index.html não encontrado", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "index.html", index.modTime, bytes.NewReader(index.data))
}

func (s *spaServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	if isSPARoute(r.URL.Path) {
		s.serveIndex(w, r)
		return
	}
	file, ok := lookupSPAFile(s.root, r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	// Assets do Vite têm hash no nome: cache imutável. O resto do público
	// (favicon, logo, ícones, i18n) pode mudar entre builds e não tem hash no
	// nome — sem revalidação, uma troca de logo só aparece depois de expirar o
	// max-age (ou nunca, com o disco cache do navegador). no-cache força
	// revalidação por ETag/Last-Modified a cada carga, sem perder o cache.
	cache := "no-cache"
	if strings.HasPrefix(r.URL.Path, "/assets/") {
		cache = "public, max-age=31536000, immutable"
	}
	w.Header().Set("Cache-Control", cache)
	http.ServeContent(w, r, path.Base(r.URL.Path), file.modTime, bytes.NewReader(file.data))
}
