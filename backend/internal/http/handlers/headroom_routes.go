package handlers

import (
	"context"
	"net/http"
)

// HeadroomRoutes espelha src/app/api/headroom/* (status/restart/start/stop/extras).
type HeadroomRoutes struct {
	H    *Headroom
	Read func(context.Context) (map[string]any, error)
}

func headroomPortOr(url string, fallback int) int {
	if port := headroomPortFromURL(url); port > 0 {
		return port
	}
	return fallback
}

func (h HeadroomRoutes) settings(r *http.Request) map[string]any {
	if h.Read == nil {
		return map[string]any{}
	}
	settings, err := h.Read(r.Context())
	if err != nil {
		return map[string]any{}
	}
	return settings
}

func (h HeadroomRoutes) url(r *http.Request) string {
	value, _ := h.settings(r)["headroomUrl"].(string)
	if value == "" {
		value = defaultHeadroomURL()
	}
	return value
}

func (h HeadroomRoutes) flags(r *http.Request) (int, bool, bool) {
	settings := h.settings(r)
	port := headroomPortFromURL(h.url(r))
	if port == 0 {
		port = headroomDefaultPort
	}
	codeAware, _ := settings["headroomCodeAware"].(bool)
	kompress := true
	if konpressOK, exists := settings["headroomKompress"]; exists && konpressOK == false {
		kompress = false
	}
	return port, codeAware, !kompress
}

func (h HeadroomRoutes) Start(w http.ResponseWriter, r *http.Request) {
	url := h.url(r)
	if !isLoopbackHeadroomURL(url) {
		nodeReply(w, 400, map[string]any{"error": "External Headroom proxies must be started outside 9Router", "code": "EXTERNAL_PROXY"})
		return
	}
	port, codeAware, disableKompress := h.flags(r)
	pid, already, err := h.H.startHeadroomProxy(port, codeAware, disableKompress)
	if err != nil {
		replyHeadroomError(w, err)
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "pid": pid, "alreadyRunning": already})
}

func (h HeadroomRoutes) Restart(w http.ResponseWriter, r *http.Request) {
	url := h.url(r)
	if !isLoopbackHeadroomURL(url) {
		nodeReply(w, 400, map[string]any{"error": "External Headroom proxies must be started outside 9Router", "code": "EXTERNAL_PROXY"})
		return
	}
	port, codeAware, disableKompress := h.flags(r)
	pid, already, err := h.H.restartHeadroomProxy(port, codeAware, disableKompress)
	if err != nil {
		replyHeadroomError(w, err)
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "pid": pid, "alreadyRunning": already})
}

func (h HeadroomRoutes) Stop(w http.ResponseWriter, r *http.Request) {
	result := h.H.stopHeadroomProxy()
	status := 200
	if stopped, _ := result["stopped"].(bool); !stopped {
		status = 409
	}
	nodeReply(w, status, result)
}

func (h HeadroomRoutes) Get(w http.ResponseWriter, r *http.Request) {
	nodeReply(w, 200, h.H.managedStatus(h.url(r)))
}

func (h HeadroomRoutes) ExtrasGet(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("log") == "1" {
		nodeReply(w, 200, map[string]any{"log": h.H.getLogTail(h.H.installFile(), headroomInstallTail)})
		return
	}
	python := h.H.python()
	binary := h.H.binary()
	installed, version, extras := false, "", map[string]bool{"code": false, "ml": false}
	if binary != "" {
		installed, version, extras = getInstalledHeadroomExtras(python)
	}
	nodeReply(w, 200, map[string]any{"available": headroomExtras, "installed": installed, "version": version, "extras": extras})
}

func (h HeadroomRoutes) ExtrasPost(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		body = map[string]any{}
	}
	result, installErr := h.H.installHeadroomExtras(headroomRequestedExtras(body))
	if installErr != nil {
		replyHeadroomExtrasError(w, installErr, "NOT_INSTALLED", "NO_PYTHON")
		return
	}
	nodeReply(w, 200, result)
}

func (h HeadroomRoutes) ExtrasDelete(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		body = map[string]any{}
	}
	result, uninstallErr := h.H.uninstallHeadroomExtras(headroomRequestedExtras(body))
	if uninstallErr != nil {
		replyHeadroomExtrasError(w, uninstallErr, "NO_PYTHON", "INVALID_EXTRAS")
		return
	}
	nodeReply(w, 200, result)
}

func replyHeadroomError(w http.ResponseWriter, err error) {
	headErr, ok := err.(*headroomError)
	if !ok {
		nodeReply(w, 500, map[string]any{"error": err.Error(), "code": nil})
		return
	}
	status := 500
	if headErr.Code == "NOT_INSTALLED" {
		// start/restart: apenas NOT_INSTALLED é 400 (rota original).
		status = 400
	}
	nodeReply(w, status, map[string]any{"error": headErr.Message, "code": headErr.Code})
}

// replyHeadroomExtrasError aplica o mapeamento 400/500 específico por rota.
func replyHeadroomExtrasError(w http.ResponseWriter, err error, badRequestCodes ...string) {
	headErr, ok := err.(*headroomError)
	if !ok {
		nodeReply(w, 500, map[string]any{"error": err.Error(), "code": nil})
		return
	}
	status := 500
	for _, code := range badRequestCodes {
		if headErr.Code == code {
			status = 400
		}
	}
	nodeReply(w, status, map[string]any{"error": headErr.Message, "code": headErr.Code})
}

func headroomRequestedExtras(body map[string]any) []string {
	items, _ := body["extras"].([]any)
	requested := make([]string, 0, len(items))
	for _, item := range items {
		if value := jsString(item); nodeTruthy(value) {
			requested = append(requested, value)
		}
	}
	return requested
}
