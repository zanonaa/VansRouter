package handlers

import (
	"bufio"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ConsoleBuffer replica o buffer global de logs e a entrega por SSE.
type ConsoleBuffer struct {
	mu          sync.Mutex
	lines       []string
	subscribers map[chan string]bool
	max         int
}

func NewConsoleBuffer() *ConsoleBuffer {
	return &ConsoleBuffer{subscribers: map[chan string]bool{}, max: 1000}
}
func (b *ConsoleBuffer) Append(line string) {
	b.mu.Lock()
	b.lines = append(b.lines, line)
	if len(b.lines) > b.max {
		b.lines = b.lines[len(b.lines)-b.max:]
	}
	for ch := range b.subscribers {
		select {
		case ch <- line:
		default:
		}
	}
	b.mu.Unlock()
}
func (b *ConsoleBuffer) Logs() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string{}, b.lines...)
}
func (b *ConsoleBuffer) Clear() { b.mu.Lock(); b.lines = nil; b.mu.Unlock() }
func (b *ConsoleBuffer) Get(w http.ResponseWriter, r *http.Request) {
	nodeReply(w, 200, map[string]any{"success": true, "logs": b.Logs(), "pm2Logs": []any{}, "pm2Info": map[string]any{"available": false, "errorLogFile": nil, "outLogFile": nil, "errorCount": 0}, "dockerLogs": []any{}, "dockerInfo": map[string]any{"isDocker": false, "available": false}})
}
func (b *ConsoleBuffer) Delete(w http.ResponseWriter, r *http.Request) {
	b.Clear()
	nodeReply(w, 200, map[string]any{"success": true})
}
func (b *ConsoleBuffer) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		nodeReply(w, 500, map[string]any{"success": false, "error": "Streaming unsupported"})
		return
	}
	ch := make(chan string, 50)
	b.mu.Lock()
	b.subscribers[ch] = true
	b.mu.Unlock()
	defer func() { b.mu.Lock(); delete(b.subscribers, ch); b.mu.Unlock() }()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	for _, line := range b.Logs() {
		fmt.Fprintf(w, "data: %s\n\n", line)
	}
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case line := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", line)
			flusher.Flush()
		}
	}
}

var _ = bufio.NewReader
var _ = time.Second
