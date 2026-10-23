package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func newOpenCodeStreamScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 16<<20)
	return scanner
}

// sendOpenCodeFree executa o contrato do free tier do OpenCode Zen:
// fingerprint do quarteto, IDs canônicos, stream forçado upstream, credencial
// "public" quando a conexão não tem chave, e fold do stream de volta a um
// chat.completion quando o cliente pediu JSON (com nomes de tools restaurados).
func (t *TranslatorSend) sendOpenCodeFree(w http.ResponseWriter, r *http.Request, provider, model string, body map[string]any, connection map[string]any) {
	fail := func(status int, message string) {
		nodeReply(w, status, map[string]any{"success": false, "error": message})
	}
	outbound := map[string]any{}
	for key, value := range body {
		outbound[key] = value
	}
	outbound["model"] = model

	clientWantsStream := outbound["stream"] == true
	outbound["stream"] = true
	renames := openCodeApplyFingerprint(outbound)

	token := ttsString(firstNonEmpty(connection["apiKey"], connection["accessToken"]))
	if token == "" {
		// Executor original: pedido sem credencial cai em "public" — o upstream
		// lê outra coisa como credencial ausente e responde 401.
		token = "public"
	}
	target := strings.TrimRight(providers.TransportFor(provider).BaseURL, "/")
	if !strings.HasSuffix(target, "/chat/completions") {
		target += "/chat/completions"
	}
	headers := openCodeHeaders(r.Header, token)

	encoded, err := json.Marshal(outbound)
	if err != nil {
		fail(500, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(encoded))
	if err != nil {
		fail(500, err.Error())
		return
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	client, relay, cleanup, err := connectionHTTP(ctx, metadata, t.GetPool, t.Client)
	if err != nil {
		fail(502, err.Error())
		return
	}
	defer cleanup()
	if err := rewriteConnectionRelay(request, relay); err != nil {
		fail(502, err.Error())
		return
	}
	response, err := client.Do(request)
	if err != nil {
		fail(502, err.Error())
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		details, _ := io.ReadAll(io.LimitReader(response.Body, 8<<10))
		nodeReply(w, response.StatusCode, map[string]any{"success": false, "error": fmt.Sprintf("Provider error: %d", response.StatusCode), "details": string(details)})
		return
	}
	if clientWantsStream {
		// Cliente pediu SSE: repassa o stream reescrevendo os nomes das tools
		// renomeadas em cada chunk.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, _ := w.(http.Flusher)
		scanner := newOpenCodeStreamScanner(response.Body)
		for scanner.Scan() {
			written := scanner.Bytes()
			if renamed, ok := openCodeRewriteChunk(written, renames); ok {
				written = renamed
			}
			line := append(append([]byte{}, written...), '\n')
			if _, err := w.Write(line); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		return
	}
	completion, err := openCodeFoldStream(response.Body, model, renames)
	if err != nil {
		fail(502, err.Error())
		return
	}
	nodeReply(w, 200, completion)
}

// openCodeRewriteChunk aplica os nomes originais em um chunk SSE de uma linha.
func openCodeRewriteChunk(line []byte, renames map[string]string) ([]byte, bool) {
	if len(renames) == 0 {
		return nil, false
	}
	trimmed := bytes.TrimSpace(line)
	if !bytes.HasPrefix(trimmed, []byte("data:")) {
		return nil, false
	}
	data := bytes.TrimSpace(bytes.TrimPrefix(trimmed, []byte("data:")))
	if len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) {
		return nil, false
	}
	var chunk map[string]any
	if json.Unmarshal(data, &chunk) != nil {
		return nil, false
	}
	openCodeRestoreNames(chunk, renames)
	rewritten, err := json.Marshal(chunk)
	if err != nil {
		return nil, false
	}
	return append([]byte("data: "), rewritten...), true
}
