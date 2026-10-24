package handlers

import "net/http"

type ProxyTestSettings struct{ Probe ProxyProbe }

func (h ProxyTestSettings) Post(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 500, map[string]any{"ok": false, "error": "Proxy test failed"})
		return
	}
	probe := h.Probe
	if testURL, ok := body["testUrl"].(string); ok && testURL != "" {
		probe.Targets = []string{testURL}
	}
	result := probe.probe(r.Context(), map[string]any{"proxyUrl": body["proxyUrl"]})
	if result.OK {
		nodeReply(w, 200, map[string]any{"ok": true, "status": result.Status, "statusText": result.StatusText, "elapsedMs": result.Elapsed})
		return
	}
	status := result.Status
	if status == 0 {
		status = 500
	}
	nodeReply(w, status, map[string]any{"ok": false, "error": result.Error})
}
