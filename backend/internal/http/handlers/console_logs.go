package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ConsoleBuffer replica o buffer global de logs e a entrega por SSE da origem
// (src/lib/consoleLogBuffer.js + src/app/api/translator/console-logs/*).
// O protocolo do stream é JSON: {type:"init",logs}, {type:"lines",lines} e
// {type:"clear"}; o cliente do dashboard faz JSON.parse em cada evento.
type ConsoleBuffer struct {
	mu          sync.Mutex
	lines       []string
	pending     []string
	timer       *time.Timer
	subscribers map[chan consoleEvent]bool
	max         int
	// DataDir é /app/data no container; usado para ler os .log do volume
	// (aba Docker da origem, src/lib/systemLogFetcher.js).
	DataDir string
}

type consoleEvent struct {
	Kind  string   `json:"type"`
	Logs  []string `json:"logs,omitempty"`
	Lines []string `json:"lines,omitempty"`
}

var consoleANSI = regexp.MustCompile("\x1b\\[[0-9;]*m")

const (
	consoleFlushInterval = 100 * time.Millisecond
	consoleFlushLines    = 50
	consoleKeepAlive     = 25 * time.Second
)

func NewConsoleBuffer() *ConsoleBuffer {
	return &ConsoleBuffer{subscribers: map[chan consoleEvent]bool{}, max: 1000}
}

// Append adiciona uma linha ao buffer e agenda o lote de 100ms (máx. 50
// linhas), como o flushPendingLines da origem.
func (b *ConsoleBuffer) Append(line string) {
	line = strings.TrimRight(consoleANSI.ReplaceAllString(line, ""), "\r\n")
	if strings.TrimSpace(line) == "" {
		return
	}
	b.mu.Lock()
	b.lines = append(b.lines, line)
	if len(b.lines) > b.max {
		b.lines = b.lines[len(b.lines)-b.max:]
	}
	b.pending = append(b.pending, line)
	if len(b.pending) >= consoleFlushLines {
		b.flushLocked()
	} else if b.timer == nil {
		b.timer = time.AfterFunc(consoleFlushInterval, b.flush)
	}
	b.mu.Unlock()
}

func (b *ConsoleBuffer) flush() {
	b.mu.Lock()
	b.flushLocked()
	b.mu.Unlock()
}

func (b *ConsoleBuffer) flushLocked() {
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
	if len(b.pending) == 0 {
		return
	}
	lines := append([]string{}, b.pending...)
	b.pending = nil
	b.broadcastLocked(consoleEvent{Kind: "lines", Lines: lines})
}

func (b *ConsoleBuffer) broadcastLocked(event consoleEvent) {
	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
			// Assinante lento: descarta para não bloquear o produtor.
		}
	}
}

func (b *ConsoleBuffer) Logs() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string{}, b.lines...)
}

func (b *ConsoleBuffer) Clear() {
	b.mu.Lock()
	b.lines = nil
	b.pending = nil
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
	b.broadcastLocked(consoleEvent{Kind: "clear"})
	b.mu.Unlock()
}

// Writer é o destino do slog: cada registro vira uma linha da aba App Console.
func (b *ConsoleBuffer) Writer() io.Writer { return consoleLogWriter{buffer: b} }

type consoleLogWriter struct{ buffer *ConsoleBuffer }

func (w consoleLogWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		w.buffer.Append(line)
	}
	return len(p), nil
}

func (b *ConsoleBuffer) Get(w http.ResponseWriter, r *http.Request) {
	dockerLogs, isDocker, available := b.dockerLogs(250)
	nodeReply(w, 200, map[string]any{
		"success":    true,
		"logs":       b.Logs(),
		"pm2Logs":    []any{},
		"pm2Info":    map[string]any{"available": false, "errorLogFile": nil, "outLogFile": nil, "errorCount": 0},
		"dockerLogs": dockerLogs,
		"dockerInfo": map[string]any{"isDocker": isDocker, "available": available},
	})
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
	ch := make(chan consoleEvent, 50)
	b.mu.Lock()
	b.subscribers[ch] = true
	initial := append([]string{}, b.lines...)
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.subscribers, ch)
		b.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	send := func(value any) bool {
		encoded, err := json.Marshal(value)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", encoded); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	// A origem só envia init quando há buffer; enviar sempre libera os headers
	// imediatamente e evita uma conexão SSE "muda" enquanto não há logs.
	if !send(consoleEvent{Kind: "init", Logs: initial}) {
		return
	}
	keepAlive := time.NewTicker(consoleKeepAlive)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event := <-ch:
			if !send(event) {
				return
			}
		}
	}
}

// Middleware registra as chamadas do gateway no console. São poucas e são o
// equivalente Go aos logs de stdout que o PM2 exibia na origem.
func (b *ConsoleBuffer) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/") && !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			next.ServeHTTP(w, r)
			return
		}
		started := time.Now()
		recorder := &consoleStatusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		slog.Info("requisição",
			"metodo", r.Method,
			"caminho", r.URL.Path,
			"status", recorder.status,
			"duracao", time.Since(started).Round(time.Millisecond).String(),
		)
	})
}

type consoleStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *consoleStatusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *consoleStatusWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *consoleStatusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// dockerLogs lê os .log do volume e prefixa com [DOCKER:nome], igual a
// getDockerLogs. PM2 não existe no container e permanece indisponível.
func (b *ConsoleBuffer) dockerLogs(maxLines int) ([]string, bool, bool) {
	isDocker := false
	if _, err := os.Stat("/.dockerenv"); err == nil {
		isDocker = true
	}
	if filepath.Clean(b.DataDir) == "/app/data" {
		isDocker = true
	}
	directory := filepath.Join(b.DataDir, "logs")
	if _, err := os.Stat(directory); err != nil {
		return []string{}, isDocker, false
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return []string{}, isDocker, false
	}
	logs := []string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".log")
		for _, line := range consoleReadTail(filepath.Join(directory, entry.Name()), 256*1024, maxLines/2) {
			logs = append(logs, "[DOCKER:"+name+"] "+line)
		}
	}
	if len(logs) > maxLines {
		logs = logs[len(logs)-maxLines:]
	}
	return logs, isDocker, len(logs) > 0
}

func consoleReadTail(path string, maxBytes int64, maxLines int) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() == 0 {
		return nil
	}
	size := info.Size()
	offset := int64(0)
	if size > maxBytes {
		offset = size - maxBytes
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil
	}
	text := consoleANSI.ReplaceAllString(string(data), "")
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines
}
