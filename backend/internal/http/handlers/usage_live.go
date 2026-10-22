package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type LiveUsageHistory interface {
	History(context.Context, string, string, string, string) ([]map[string]any, error)
}
type liveUsageKey struct{ Connection, Model, Provider string }
type UsageLive struct {
	History       LiveUsageHistory
	Connections   NodeConnections
	mu            sync.Mutex
	pending       map[liveUsageKey]int
	timers        map[liveUsageKey]*time.Timer
	subscribers   map[chan struct{}]bool
	errorProvider string
	errorAt       time.Time
}

// Track mantém os contadores reais de requests para SSE e para o gateway.
func (u *UsageLive) Track(model, provider, connection string, started, failed bool) {
	if connection == "" {
		connection = "no-auth"
	}
	key := liveUsageKey{connection, model, provider}
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.pending == nil {
		u.pending = map[liveUsageKey]int{}
		u.timers = map[liveUsageKey]*time.Timer{}
	}
	if timer := u.timers[key]; timer != nil {
		timer.Stop()
		delete(u.timers, key)
	}
	if started {
		u.pending[key]++
		u.timers[key] = time.AfterFunc(time.Minute, func() { u.mu.Lock(); u.pending[key] = 0; delete(u.timers, key); u.notifyLocked(); u.mu.Unlock() })
	} else {
		u.pending[key]--
		if u.pending[key] <= 0 {
			delete(u.pending, key)
		}
		if failed && provider != "" {
			u.errorProvider = strings.ToLower(provider)
			u.errorAt = time.Now()
		}
	}
	u.notifyLocked()
}
func (u *UsageLive) notifyLocked() {
	for ch := range u.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
func (u *UsageLive) Notify() { u.mu.Lock(); u.notifyLocked(); u.mu.Unlock() }
func (u *UsageLive) Snapshot(ctx context.Context) (map[string]any, error) {
	names := map[string]string{}
	if u.Connections != nil {
		if connections, err := u.Connections.List(ctx, "", nil); err == nil {
			for _, connection := range connections {
				names[ttsString(connection["id"])] = ttsString(firstNonEmpty(connection["name"], connection["email"], connection["id"]))
			}
		}
	}
	u.mu.Lock()
	byModel := map[string]int{}
	byAccount := map[string]map[string]int{}
	active := []map[string]any{}
	keys := []liveUsageKey{}
	for key := range u.pending {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j]) })
	for _, key := range keys {
		count := u.pending[key]
		modelKey := key.Model
		if key.Provider != "" {
			modelKey += " (" + key.Provider + ")"
		}
		byModel[modelKey] += count
		if byAccount[key.Connection] == nil {
			byAccount[key.Connection] = map[string]int{}
		}
		byAccount[key.Connection][modelKey] = count
		if count > 0 {
			name := names[key.Connection]
			if key.Connection == "no-auth" {
				name = "Free / No Auth"
			} else if name == "" {
				prefix := key.Connection
				if len(prefix) > 8 {
					prefix = prefix[:8]
				}
				name = "Account " + prefix + "..."
			}
			provider := key.Provider
			if provider == "" {
				provider = "unknown"
			}
			active = append(active, map[string]any{"model": key.Model, "provider": provider, "account": name, "count": count})
		}
	}
	errorProvider := ""
	if time.Since(u.errorAt) < 10*time.Second {
		errorProvider = u.errorProvider
	}
	u.mu.Unlock()
	recent := []map[string]any{}
	if u.History != nil {
		rows, err := u.History.History(ctx, "", "", "", "")
		if err != nil {
			return nil, err
		}
		if len(rows) > 50 {
			rows = rows[len(rows)-50:]
		}
		sort.SliceStable(rows, func(i, j int) bool { return ttsString(rows[i]["timestamp"]) > ttsString(rows[j]["timestamp"]) })
		seen := map[string]bool{}
		for _, row := range rows {
			tokens, _ := row["tokens"].(map[string]any)
			prompt := firstNonEmpty(tokens["prompt_tokens"], tokens["input_tokens"], 0)
			completion := firstNonEmpty(tokens["completion_tokens"], tokens["output_tokens"], 0)
			if pxpipeNum(prompt) == 0 && pxpipeNum(completion) == 0 {
				continue
			}
			timestamp := ttsString(row["timestamp"])
			minute := timestamp
			if len(minute) > 16 {
				minute = minute[:16]
			}
			key := fmt.Sprintf("%v|%v|%v|%v|%s", row["model"], row["provider"], prompt, completion, minute)
			if seen[key] {
				continue
			}
			seen[key] = true
			recent = append(recent, map[string]any{"timestamp": row["timestamp"], "model": row["model"], "provider": ttsString(row["provider"]), "promptTokens": prompt, "completionTokens": completion, "status": firstNonEmpty(row["status"], "ok")})
			if len(recent) == 20 {
				break
			}
		}
	}
	return map[string]any{"activeRequests": active, "recentRequests": recent, "errorProvider": errorProvider, "pending": map[string]any{"byModel": byModel, "byAccount": byAccount}}, nil
}
func (u *UsageLive) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		nodeReply(w, 500, map[string]any{"error": "Streaming unsupported"})
		return
	}
	ch := make(chan struct{}, 1)
	u.mu.Lock()
	if u.subscribers == nil {
		u.subscribers = map[chan struct{}]bool{}
	}
	u.subscribers[ch] = true
	u.mu.Unlock()
	defer func() { u.mu.Lock(); delete(u.subscribers, ch); u.mu.Unlock() }()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	send := func() bool {
		payload, err := u.Snapshot(r.Context())
		if err != nil {
			return false
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return false
		}
		if _, err = fmt.Fprintf(w, "data: %s\n\n", encoded); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if !send() {
		return
	}
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			if !send() {
				return
			}
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
