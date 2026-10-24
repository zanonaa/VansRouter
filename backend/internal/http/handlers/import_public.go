package handlers

// Porta de src/app/api/proxy-pools/import-public/route.js.
import (
	"net/http"
)

// ImportPublic importa listas públicas de proxies para o pool.
type ImportPublic struct {
	Store   PoolStore
	Fetcher publicProxyFetcher
}

func (s *ImportPublic) fetchHandler() publicProxyFetcher {
	if s.Fetcher != nil {
		return s.Fetcher
	}
	return fetchUpstreamReal
}

// ImportPublic: POST /api/proxy-pools/import-public.
func (s *ImportPublic) Post(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		body = map[string]any{}
	}
	sourceID := valueStringOrEmpty(body["source"])
	if _, ok := map[string]bool{"freeproxydb": true, "vpslab": true}[sourceID]; !ok || sourceID == "" {
		nodeReply(w, 400, map[string]any{"error": "Unknown proxy source"})
		return
	}
	options := parseImportRequest(body, sourceID)
	if options.Error != "" {
		nodeReply(w, 400, map[string]any{"error": options.Error})
		return
	}
	entries, err := fetchPublicProxyEntries(r.Context(), s.fetchHandler(), sourceID, options)
	if err != nil {
		status := 502
		if coded, ok := err.(*publicProxyError); ok {
			if coded.Status == 429 {
				status = 429
			} else if coded.Status == 504 {
				status = 502
			}
		}
		nodeReply(w, status, map[string]any{"error": err.Error()})
		return
	}
	existing, err := s.Store.List(r.Context(), nil, "")
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": "Failed to import public proxies"})
		return
	}
	existingKeys := map[string]bool{}
	for _, pool := range existing {
		if key := normalizeProxyURLKey(pool["proxyUrl"]); key != "" {
			existingKeys[key] = true
		}
	}
	imported := 0
	duplicates := 0
	failed := 0
	created := []map[string]any{}
	for _, entry := range entries {
		key := valueStringOrEmpty(entry["proxyUrl"])
		if existingKeys[key] {
			duplicates++
			continue
		}
		existingKeys[key] = true
		createdPool, err := s.Store.Create(r.Context(), map[string]any{
			"name":     entry["name"],
			"proxyUrl": entry["proxyUrl"],
			"noProxy":  "",
			"isActive": true,
			"type":     "http",
		})
		if err != nil {
			failed++
			continue
		}
		imported++
		created = append(created, map[string]any{"id": createdPool["id"], "name": createdPool["name"], "proxyUrl": createdPool["proxyUrl"]})
	}
	response := map[string]any{
		"source":     sourceID,
		"file":       nilOrNilStringValue(""),
		"fetched":    len(entries),
		"imported":   imported,
		"duplicates": duplicates,
		"failed":     failed,
		"created":    created,
	}
	if sourceID == "vpslab" {
		response["file"] = resolveVpsLabFile(options.Protocol, options.Anonymity, options.HTTPSOnly)
	}
	if len(entries) == 0 {
		response["reason"] = "No currently-valid proxies matched these filters — relax the filters, switch the ordering, or try again later (the list refreshes continuously)."
	}
	nodeReply(w, 200, response)
}
