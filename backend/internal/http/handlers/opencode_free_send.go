package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
//
// ip-limit: o free tier limita por IP de saída (janela diária UTC). Quando o
// upstream responde 429/403 FreeUsageLimitError, a origem marca o pool como
// unfit no escopo provider::modelo (retryAfter quando disponível) e re-tenta
// com o próximo pool do pool-set (MAX_POOL_RETRIES=2). Sem mais pools,
// devolve o 429 com Retry-After para o cliente.
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
	t.TokenSaver.Apply(r.Context(), r, outbound, "openai", model, provider)

	token := ttsString(firstNonEmpty(connection["apiKey"], connection["accessToken"]))
	if token == "" {
		// Executor original: pedido sem credencial cai em "public" — o upstream
		// lê outra coisa como credencial ausente e responde 401.
		token = "public"
	}
	// Upstream do free tier conforme o transport registrado do provider.
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

	metadata, _ := connection["providerSpecificData"].(map[string]any)
	const maxPoolRetries = 2
	excluded := []string{}
	var lastStatus int
	var lastDetails string
	var lastRetryAfter string
	for attempt := 0; ; attempt++ {
		attemptMetadata := metadata
		if len(excluded) > 0 {
			copied := map[string]any{}
			for key, value := range metadata {
				copied[key] = value
			}
			// proxyPoolIds vira a lista candidata sem os pools já reprovados;
			// a resolução continua respeitando estratégia/targets/fitness.
			if remaining := mergePoolCandidates(metadata, excluded); len(remaining) > 0 {
				copied["proxyPoolIds"] = remaining
				copied["excludeProxyPoolIds"] = excluded
				if single := strings.TrimSpace(ttsString(copied["proxyPoolId"])); single != "" && single != "__none__" {
					copied["proxyPoolId"] = "__none__"
				}
				copied["proxyPoolScope"] = openCodeIPLimitScope(provider, model)
			}
			attemptMetadata = copied
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(encoded))
		if err != nil {
			cancel()
			fail(500, err.Error())
			return
		}
		for key, value := range headers {
			request.Header.Set(key, value)
		}
		client, relay, poolID, cleanup, err := resolveConnectionHTTP(ctx, attemptMetadata, t.GetPool, t.Client)
		if err != nil {
			cancel()
			fail(502, err.Error())
			return
		}
		if err := rewriteConnectionRelay(request, relay); err != nil {
			cancel()
			fail(502, err.Error())
			return
		}
		auditOpenCodeIdentity(r.Header, headers, poolID, model, attempt+1)
		response, err := client.Do(request)
		if err != nil {
			response = nil
			if poolID != "" && attempt < maxPoolRetries && len(mergePoolCandidates(metadata, append(append([]string{}, excluded...), poolID))) > 0 {
				// Falha de transporte no proxy marcado: tenta o próximo pool
				// antes de devolver erro ao cliente.
				excluded = append(excluded, poolID)
				markPoolUnfit(r.Context(), poolID, openCodeIPLimitScope(provider, model), nil, "transport-error")
				cancel()
				cleanup()
				continue
			}
			cancel()
			cleanup()
			fail(502, err.Error())
			return
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			defer cleanup()
			defer cancel()
			t.openCodeRelayResponse(w, response, clientWantsStream, model, renames)
			return
		}
		details, _ := io.ReadAll(io.LimitReader(response.Body, 8<<10))
		response.Body.Close()
		cancel()
		cleanup()
		lastStatus = response.StatusCode
		lastDetails = string(details)
		lastRetryAfter = response.Header.Get("Retry-After")
		if poolID != "" && openCodeIsIPLimit(response.StatusCode, lastDetails) {
			until := ipLimitRetryAfter(lastRetryAfter)
			markPoolUnfit(r.Context(), poolID, openCodeIPLimitScope(provider, model), until, "ip-limit")
			excluded = append(excluded, poolID)
			if attempt < maxPoolRetries && len(mergePoolCandidates(metadata, excluded)) > 0 {
				continue
			}
		}
		break
	}
	if lastRetryAfter != "" {
		w.Header().Set("Retry-After", lastRetryAfter)
	}
	nodeReply(w, lastStatus, map[string]any{"success": false, "error": fmt.Sprintf("Provider error: %d", lastStatus), "details": lastDetails})
}

// openCodeRelayResponse escreve a resposta de sucesso: stream dobrado de volta
// em chat.completion para clientes JSON, ou SSE repassado com nomes de tools
// restaurados.
func (t *TranslatorSend) openCodeRelayResponse(w http.ResponseWriter, response *http.Response, clientWantsStream bool, model string, renames map[string]string) {
	defer response.Body.Close()
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
		nodeReply(w, 502, map[string]any{"success": false, "error": err.Error()})
		return
	}
	nodeReply(w, 200, completion)
}

// openCodeIPLimitScope é o escopo de fitness do free tier (provider::modelo),
// como o proxyScope do chatCore original.
func openCodeIPLimitScope(provider, model string) string {
	return provider + "::" + model
}

// openCodeIsIPLimit reproduz parseError do executor original: 429/403 com
// corpo de limite (limit|rate|quota|exhausted|capacity|too many|retry) é um
// erro escopado ao pool (ip-limit).
func openCodeIsIPLimit(status int, body string) bool {
	if status != 429 && status != 403 {
		return false
	}
	lower := strings.ToLower(body)
	for _, marker := range []string{"limit", "rate", "quota", "exhausted", "capacity", "too many", "retry"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// ipLimitRetryAfter converte o Retry-After do upstream em timestamp de
// expiração do fitness; sem valor usa 24h (a janela do free tier é diária).
func ipLimitRetryAfter(retryAfter string) *int64 {
	if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && seconds > 0 {
		until := time.Now().Add(time.Duration(seconds) * time.Second).UnixMilli()
		return &until
	}
	fallback := time.Now().Add(24 * time.Hour).UnixMilli()
	return &fallback
}

// mergePoolCandidates devolve a lista de pools candidatos menos os excluídos,
// preservando a ordem. Inclui o pool único quando não há pool-set — a origem
// re-resolve por resolveProxyConfig com excludePoolIds.
func mergePoolCandidates(metadata map[string]any, excluded []string) []string {
	ids := dedupeStrings(stringSlice(metadata["proxyPoolIds"]))
	if len(ids) == 0 {
		if single := strings.TrimSpace(ttsString(metadata["proxyPoolId"])); single != "" && single != "__none__" {
			ids = []string{single}
		}
	}
	result := make([]string, 0, len(ids))
	skip := map[string]bool{}
	for _, id := range excluded {
		skip[id] = true
	}
	for _, id := range ids {
		if !skip[id] {
			result = append(result, id)
		}
	}
	return result
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
