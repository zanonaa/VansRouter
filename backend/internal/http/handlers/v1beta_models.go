package handlers

// Espelho de src/app/api/v1beta/models/route.js (listagem formato Gemini).
import (
	"net/http"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

// V1BetaModels expõe GET /v1beta/models; teste injeta Catalog.
type V1BetaModels struct {
	Catalog func() providers.ModelsCatalog
}

func (v *V1BetaModels) catalog() providers.ModelsCatalog {
	if v.Catalog != nil {
		return v.Catalog()
	}
	return providers.CatalogModels()
}

func (v *V1BetaModels) Get(w http.ResponseWriter, r *http.Request) {
	catalog := v.catalog()
	models := []map[string]any{}
	seen := map[string]bool{}
	addModel := func(name, displayName, description string, methods []string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		models = append(models, map[string]any{
			"name":                       name,
			"displayName":                displayName,
			"description":                description,
			"supportedGenerationMethods": methods,
			"inputTokenLimit":            128000,
			"outputTokenLimit":           8192,
		})
	}
	// Ordem POR provider de primeira aparição (Object.entries da origem).
	order := []string{}
	seenProvider := map[string]bool{}
	for _, entry := range catalog.Models {
		provider := ttsString(entry["provider"])
		if provider == "" || seenProvider[provider] {
			continue
		}
		seenProvider[provider] = true
		order = append(order, provider)
	}
	for _, provider := range order {
		for _, entry := range catalog.Models {
			if ttsString(entry["provider"]) != provider {
				continue
			}
			id := ttsString(entry["model"])
			if id == "" {
				continue
			}
			display := ttsString(entry["name"])
			if display == "" {
				display = id
			}
			addModel("models/"+provider+"/"+id, display, provider+" model: "+display, []string{"generateContent"})
			if provider == "gemini" {
				addModel("models/"+id, display, "Gemini model: "+display, []string{"generateContent", "streamGenerateContent"})
			}
		}
	}
	nodeReply(w, 200, map[string]any{"models": models})
}
