package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConsoleLogsBufferAndClear(t *testing.T) {
	b := NewConsoleBuffer()
	b.Append("one")
	b.Append("\x1b[31mtwo\x1b[0m")
	w := httptest.NewRecorder()
	b.Get(w, httptest.NewRequest("GET", "/", nil))
	body := w.Body.String()
	if !strings.Contains(body, "one") || !strings.Contains(body, "two") {
		t.Fatal(body)
	}
	if strings.Contains(body, "\x1b[") {
		t.Fatalf("ANSI não foi removido: %q", body)
	}
	b.Delete(w, httptest.NewRequest("DELETE", "/", nil))
	if len(b.Logs()) != 0 {
		t.Fatal("not cleared")
	}
}

func TestConsoleLogsStreamProtocol(t *testing.T) {
	b := NewConsoleBuffer()
	b.Append("before")
	b.flush() // fecha o lote anterior: o init carrega "before" e o SSE não o repete
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		b.Stream(recorder, request)
		close(done)
	}()

	deadline := time.Now().Add(time.Second)
	for {
		b.mu.Lock()
		subscribed := len(b.subscribers) > 0
		b.mu.Unlock()
		if subscribed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("stream não assinou o buffer")
		}
		time.Sleep(time.Millisecond)
	}

	b.Append("live")
	b.flush()
	b.Clear()
	// O event loop do SSE pode escolher o cancelamento antes do clear se os
	// dois estiverem prontos; dá uma janela curta para o evento ser escrito.
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	body := recorder.Body.String()
	for _, expected := range []string{
		`"type":"init"`,
		`"logs":["before"]`,
		`"type":"lines"`,
		`"lines":["live"]`,
		`"type":"clear"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("stream sem %s:\n%s", expected, body)
		}
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("content-type: %s", contentType)
	}
}

func TestConsoleLogsDockerTabReadsLogFiles(t *testing.T) {
	directory := t.TempDir()
	logsDirectory := filepath.Join(directory, "logs")
	if err := os.MkdirAll(logsDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logsDirectory, "gateway.log"), []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := NewConsoleBuffer()
	b.DataDir = directory
	w := httptest.NewRecorder()
	b.Get(w, httptest.NewRequest("GET", "/", nil))
	body := w.Body.String()
	if !strings.Contains(body, `"available":true`) {
		t.Fatalf("docker deveria estar disponível: %s", body)
	}
	if !strings.Contains(body, "[DOCKER:gateway] line1") || !strings.Contains(body, "[DOCKER:gateway] line2") {
		t.Fatalf("linhas do volume ausentes: %s", body)
	}
}

func TestConsoleLogsMiddlewareLogsGatewayRequests(t *testing.T) {
	previous := slog.Default()
	b := NewConsoleBuffer()
	slog.SetDefault(slog.New(slog.NewTextHandler(b.Writer(), nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	handler := b.Middleware(next)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/v1/chat/completions", nil))
	b.flush()
	logs := b.Logs()
	if len(logs) != 1 || !strings.Contains(logs[0], "status=201") || !strings.Contains(logs[0], "caminho=/v1/chat/completions") {
		t.Fatalf("request log ausente: %#v", logs)
	}

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/health", nil))
	b.flush()
	if len(b.Logs()) != 1 {
		t.Fatalf("rota fora do gateway não deveria logar: %#v", b.Logs())
	}
}
