package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type fusionAnswer struct{ model, text string }

func fusionText(data map[string]any) string {
	choices, _ := data["choices"].([]any)
	if len(choices) > 0 {
		choice, _ := choices[0].(map[string]any)
		message, _ := choice["message"].(map[string]any)
		if text := strings.TrimSpace(ttsString(message["content"])); text != "" {
			return text
		}
		if text := strings.TrimSpace(ttsString(choice["text"])); text != "" {
			return text
		}
	}
	if text := strings.TrimSpace(ttsString(data["content"])); text != "" {
		return text
	}
	candidates, _ := data["candidates"].([]any)
	if len(candidates) > 0 {
		candidate, _ := candidates[0].(map[string]any)
		content, _ := candidate["content"].(map[string]any)
		parts, _ := content["parts"].([]any)
		text := ""
		for _, raw := range parts {
			part, _ := raw.(map[string]any)
			text += ttsString(part["text"])
		}
		if strings.TrimSpace(text) != "" {
			return text
		}
	}
	output, _ := data["output"].([]any)
	text := ""
	for _, raw := range output {
		item, _ := raw.(map[string]any)
		content, _ := item["content"].([]any)
		for _, rawPart := range content {
			part, _ := rawPart.(map[string]any)
			text += ttsString(part["text"])
		}
	}
	return strings.TrimSpace(text)
}

func flattenFusionHistory(messages []any) []any {
	result := []any{}
	for _, raw := range messages {
		message, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		role := ttsString(message["role"])
		if role == "tool" || role == "function" {
			result = append(result, map[string]any{"role": "assistant", "content": "[Tool result: " + ttsString(message["content"]) + "]"})
			continue
		}
		copied := map[string]any{}
		for key, value := range message {
			if key != "tool_calls" {
				copied[key] = value
			}
		}
		if calls, ok := message["tool_calls"].([]any); ok {
			names := []string{}
			for _, raw := range calls {
				call, _ := raw.(map[string]any)
				function, _ := call["function"].(map[string]any)
				names = append(names, ttsString(firstNonEmpty(function["name"], call["name"], "tool")))
			}
			base := ttsString(copied["content"])
			if base != "" {
				base += "\n"
			}
			copied["content"] = base + "[Called tools: " + strings.Join(names, ", ") + "]"
		}
		result = append(result, copied)
	}
	return result
}

func fusionPrompt(answers []fusionAnswer) string {
	parts := []string{}
	for index, answer := range answers {
		parts = append(parts, fmt.Sprintf("[Source %d]\n%s", index+1, answer.text))
	}
	return strings.Join([]string{
		fmt.Sprintf("You are the JUDGE in a model-fusion panel. %d expert models independently answered the user's most recent request. Their responses are below, anonymized by source.", len(answers)),
		"",
		"Do NOT mention that multiple models were used, and do NOT refer to the sources. Produce ONE authoritative final answer addressed directly to the user.",
		"",
		"First, internally analyze the panel along these dimensions: consensus (points most sources agree on — treat as higher-confidence), contradictions (where they disagree — resolve with your own judgment), partial coverage, unique insights only one source surfaced, and blind spots every source missed. Then write the best possible final answer grounded in that analysis — more complete and correct than any single response, with no filler.",
		"",
		"=== PANEL RESPONSES ===",
		strings.Join(parts, "\n\n"),
		"=== END PANEL RESPONSES ===",
		"",
		"Now write the final answer to the user's original request.",
	}, "\n")
}

func (g *Gateway) fusionCombo(w http.ResponseWriter, r *http.Request, models []string, name string, config, body map[string]any) {
	if len(models) == 1 {
		// Sem trocar o model, o body ainda aponta para o nome do combo e o
		// OpenAIChat reentra no combo indefinidamente. A origem chama
		// handleSingleModel(body, panel[0]) justamente para fixar o membro.
		body["model"] = models[0]
		encoded, _ := json.Marshal(body)
		request := r.Clone(r.Context())
		request.Body = io.NopCloser(bytes.NewReader(encoded))
		g.OpenAIChat(w, request)
		return
	}
	tuning, _ := config["fusionTuning"].(map[string]any)
	minPanel := int(pxpipeNum(firstNonEmpty(tuning["minPanel"], 2)))
	if minPanel < 2 {
		minPanel = 2
	}
	if minPanel > len(models) {
		minPanel = len(models)
	}
	grace := time.Duration(pxpipeNum(firstNonEmpty(tuning["stragglerGraceMs"], 8000))) * time.Millisecond
	hard := time.Duration(pxpipeNum(firstNonEmpty(tuning["panelHardTimeoutMs"], 90000))) * time.Millisecond
	ctx, cancel := context.WithTimeout(r.Context(), hard)
	defer cancel()
	results := make([]fusionAnswer, len(models))
	var mu sync.Mutex
	// allDone fecha quando todos os painéis terminaram (não há motivo para
	// esperar o grace); quorum fecha quando já há respostas suficientes e
	// inicia a janela para os stragglers, como collectPanel na origem.
	allDone := make(chan struct{})
	quorum := make(chan struct{})
	successes := 0
	settled := 0
	signal := func(ch chan struct{}) {
		select {
		case <-ch:
		default:
			close(ch)
		}
	}
	for index, model := range models {
		go func(index int, model string) {
			panel := map[string]any{}
			for key, value := range body {
				if key != "tools" && key != "tool_choice" && key != "stream_options" {
					panel[key] = value
				}
			}
			panel["model"] = model
			panel["stream"] = false
			if messages, ok := panel["messages"].([]any); ok {
				panel["messages"] = flattenFusionHistory(messages)
			}
			encoded, _ := json.Marshal(panel)
			request := r.Clone(ctx)
			request.Body = io.NopCloser(bytes.NewReader(encoded))
			capture := &fusionCapture{}
			g.OpenAIChat(capture, request)
			var parsed map[string]any
			text := ""
			if capture.status >= 200 && capture.status < 300 && json.Unmarshal(capture.body.Bytes(), &parsed) == nil {
				text = fusionText(parsed)
			}
			mu.Lock()
			defer mu.Unlock()
			settled++
			if text != "" {
				results[index] = fusionAnswer{model: model, text: text}
				successes++
			}
			if settled == len(models) {
				signal(allDone)
			} else if successes >= minPanel {
				signal(quorum)
			}
		}(index, model)
	}
	select {
	case <-allDone:
	case <-ctx.Done():
	case <-quorum:
		timer := time.NewTimer(grace)
		select {
		case <-allDone:
		case <-ctx.Done():
		case <-timer.C:
		}
		timer.Stop()
	}
	mu.Lock()
	answers := []fusionAnswer{}
	for _, answer := range results {
		if answer.text != "" {
			answers = append(answers, answer)
		}
	}
	mu.Unlock()
	if len(answers) == 0 {
		g.webError(w, 503, "All fusion panel models failed")
		return
	}
	if len(answers) == 1 {
		body["model"] = answers[0].model
		encoded, _ := json.Marshal(body)
		request := r.Clone(r.Context())
		request.Body = io.NopCloser(bytes.NewReader(encoded))
		g.OpenAIChat(w, request)
		return
	}
	judge := strings.TrimSpace(ttsString(config["judgeModel"]))
	if judge == "" {
		judge = models[0]
	}
	next := map[string]any{}
	for key, value := range body {
		next[key] = value
	}
	next["model"] = judge
	messages, _ := next["messages"].([]any)
	next["messages"] = append(append([]any{}, messages...), map[string]any{"role": "user", "content": fusionPrompt(answers)})
	encoded, _ := json.Marshal(next)
	request := r.Clone(r.Context())
	request.Body = io.NopCloser(bytes.NewReader(encoded))
	g.OpenAIChat(w, request)
}

type fusionCapture struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (f *fusionCapture) Header() http.Header {
	if f.header == nil {
		f.header = http.Header{}
	}
	return f.header
}
func (f *fusionCapture) WriteHeader(status int) {
	if f.status == 0 {
		f.status = status
	}
}
func (f *fusionCapture) Write(body []byte) (int, error) {
	if f.status == 0 {
		f.status = 200
	}
	return f.body.Write(body)
}
