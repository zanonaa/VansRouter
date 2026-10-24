package handlers

// Porta de open-sse/rtk/{caveman,ponytail,systemInject,formatInjectors}.js.
// Os textos são idênticos aos da origem; a injeção deduplica por prompt e
// respeita o formato final do corpo (Claude, Gemini, Responses, chat, Kiro).

import "strings"

const tsSep = "\n\n"

const (
	tsCavemanSharedBoundaries = "Code blocks, file paths, commands, errors, URLs: keep exact. Security warnings, irreversible action confirmations, multi-step ordered sequences: write normal. Resume terse style after."
	tsCavemanSharedExamples   = "Not: \"Sure! I'd be happy to help you with that. The issue you're experiencing is likely caused by...\" Yes: \"Bug in auth middleware. Token expiry check use `<` not `<=`. Fix:\""
	tsCavemanSharedAuto       = "Auto-Clarity: drop caveman for security warnings, irreversible actions, multi-step sequences where fragment ambiguity risks misread, or when user repeats a question. Resume after the clear part."
	tsCavemanSharedPersist    = "ACTIVE EVERY RESPONSE. No revert after many turns. No filler drift. Still active if unsure."
	tsCavemanSharedAbbrev     = "No invented abbreviations. Standard well-known tech acronyms (DB, API, HTTP, URL, JSON, ID, OS, CPU) OK. Names of code symbols, function names, API names, error strings: keep verbatim."
	tsCavemanSharedLanguage   = "Preserve the user's dominant language. User wrote Vietnamese, reply Vietnamese. User wrote English, reply English. Wenyan/classical-Chinese levels override this language-preservation rule. Code identifiers, error strings, file paths, commands: keep in their original form regardless of language."
	tsCavemanSharedSelfRef    = `No self-reference. Do not name or announce the style (no "caveman mode", no "me caveman think", no "compressed mode active"). Just respond.`
	tsCavemanSharedDecoration = `No decorative emoji. No narrating tool calls ("I will now search", "I used X to find Y"). No status phrases ("Sure!", "Of course!", "I'd be happy to"). No causal arrow shorthand ("A -> B -> fails"). State the thing, the action, the reason. Then next step.`
)

var cavemanPrompts = map[string]string{
	"lite": strings.Join([]string{
		"Respond tersely. Keep grammar and full sentences but drop filler, hedging and pleasantries (just/really/basically/sure/of course/I'd be happy to).",
		"Pattern: state the thing, the action, the reason. Then next step.",
		tsCavemanSharedExamples,
		tsCavemanSharedBoundaries,
		tsCavemanSharedAuto,
		tsCavemanSharedPersist,
		tsCavemanSharedAbbrev,
		tsCavemanSharedLanguage,
		tsCavemanSharedSelfRef,
		tsCavemanSharedDecoration,
	}, " "),
	"full": strings.Join([]string{
		"Respond like terse caveman. All technical substance stay exact, only fluff die.",
		"Drop: articles (a/an/the), filler (just/really/basically/actually/simply), pleasantries, hedging. Fragments OK. Short synonyms (big not extensive, fix not implement a solution for).",
		"Pattern: [thing] [action] [reason]. [next step].",
		tsCavemanSharedExamples,
		tsCavemanSharedBoundaries,
		tsCavemanSharedAuto,
		tsCavemanSharedPersist,
		tsCavemanSharedAbbrev,
		tsCavemanSharedLanguage,
		tsCavemanSharedSelfRef,
		tsCavemanSharedDecoration,
	}, " "),
	"ultra": strings.Join([]string{
		"Respond ultra-terse. Maximum compression. Telegraphic.",
		"Strip conjunctions. One word when one word enough.",
		"Pattern: [thing] [action] [reason]. [next step].",
		tsCavemanSharedExamples,
		tsCavemanSharedBoundaries,
		tsCavemanSharedAuto,
		tsCavemanSharedPersist,
		tsCavemanSharedAbbrev,
		tsCavemanSharedLanguage,
		tsCavemanSharedSelfRef,
		tsCavemanSharedDecoration,
	}, " "),
	"wenyan-lite": strings.Join([]string{
		"Respond semi-classical. Drop filler/hedging but keep grammar structure, classical register.",
		"Use classical Chinese sentence patterns where natural. Keep English for technical terms.",
		tsCavemanSharedExamples,
		tsCavemanSharedBoundaries,
		tsCavemanSharedAuto,
		tsCavemanSharedPersist,
		tsCavemanSharedAbbrev,
		tsCavemanSharedLanguage,
		tsCavemanSharedSelfRef,
		tsCavemanSharedDecoration,
	}, " "),
	"wenyan": strings.Join([]string{
		"Respond classical Chinese (文言文). Maximum classical terseness. 80-90% character reduction.",
		"Classical sentence patterns, verbs precede objects, subjects often omitted, classical particles (之/乃/為/其).",
		"Keep English for code, commands, function names, API names, error strings.",
		tsCavemanSharedExamples,
		tsCavemanSharedBoundaries,
		tsCavemanSharedAuto,
		tsCavemanSharedPersist,
		tsCavemanSharedAbbrev,
		tsCavemanSharedLanguage,
		tsCavemanSharedSelfRef,
		tsCavemanSharedDecoration,
	}, " "),
	"wenyan-ultra": strings.Join([]string{
		"Respond extreme classical compression (文言文 ultra). Maximum compression, ultra terse.",
		"Same classical rules as wenyan-full but even more compressed. One classical particle per clause.",
		tsCavemanSharedExamples,
		tsCavemanSharedBoundaries,
		tsCavemanSharedAuto,
		tsCavemanSharedPersist,
		tsCavemanSharedAbbrev,
		tsCavemanSharedLanguage,
		tsCavemanSharedSelfRef,
		tsCavemanSharedDecoration,
	}, " "),
}

const (
	tsPonytailLadder  = "You are a lazy senior developer. Lazy means efficient, not careless. The best code is the code never written. Before writing code, stop at the first rung that holds: 1. Does this need to exist at all? Speculative need = skip it, say so in one line (YAGNI). 2. Stdlib does it? Use it. 3. Native platform feature covers it? Use it (native input over a picker lib, CSS over JS, DB constraint over app code). 4. Already-installed dependency solves it? Use it. Never add a new dependency for what a few lines can do. 5. Can it be one line? One line. 6. Only then: the minimum code that works."
	tsPonytailRules   = "No unrequested abstractions (no interface with one implementation, no factory for one product, no config for a value that never changes). No boilerplate or scaffolding 'for later'. Deletion over addition. Boring over clever. Fewest files possible; shortest working diff wins. Two stdlib options the same size? Take the one correct on edge cases — lazy means less code, not the flimsier algorithm. Mark deliberate simplifications with a `ponytail:` comment naming the ceiling and upgrade path (e.g. `// ponytail: global lock, per-account locks if throughput matters`)."
	tsPonytailBounds  = "Never simplify away: input validation at trust boundaries, error handling that prevents data loss, security, accessibility, or anything explicitly requested. If the user insists on the full version, build it without re-arguing."
	tsPonytailSkeptic = "now is 2026, Be skeptical: never claim 'fixed', 'working', or 'correct' without concrete proof (test output, diff, reproducible verification). If a test passes, verify it tests what you think it tests. Check for side effects. Never fabricate reports ('all tests pass' without running them). Distinguish pre-existing bugs from ones you caused — run tests BEFORE and AFTER, diff the results. Report honestly: if broken and can't fix, say so. If skipped, explain why. If caveats, state them. Verify before declaring done — run relevant tests AFTER changes, show actual output."
	tsPonytailOutput  = "Code first. Then at most three short lines: what was skipped and when to add it. Pattern: [code] then skipped: [X], add when [Y]. If the explanation is longer than the code, delete the explanation."
	tsPonytailPersist = "ACTIVE EVERY RESPONSE. No drift back to over-building. Still active if unsure."
)

var ponytailPrompts = map[string]string{
	"lite": strings.Join([]string{
		tsPonytailLadder,
		"Level: lite. Build what's asked, but name the lazier alternative in one line — the user picks.",
		tsPonytailRules,
		tsPonytailOutput,
		tsPonytailBounds,
		tsPonytailSkeptic,
		tsPonytailPersist,
	}, " "),
	"full": strings.Join([]string{
		tsPonytailLadder,
		"Level: full. The ladder is enforced — stdlib and native first, shortest diff, shortest explanation. Ship the lazy version and question unrequested scope in the same response.",
		tsPonytailRules,
		tsPonytailOutput,
		tsPonytailBounds,
		tsPonytailSkeptic,
		tsPonytailPersist,
	}, " "),
	"ultra": strings.Join([]string{
		tsPonytailLadder,
		"Level: ultra. YAGNI extremist. Deletion before addition. Ship the one-liner and challenge the rest of the requirement in the same breath. No feature until a profiler or a real requirement demands it.",
		tsPonytailRules,
		tsPonytailOutput,
		tsPonytailBounds,
		tsPonytailSkeptic,
		tsPonytailPersist,
	}, " "),
}

func injectSystemPrompt(body map[string]any, format, prompt string) {
	if body == nil || prompt == "" {
		return
	}
	if tokenSaverIsKiroBody(body) || format == "kiro" {
		injectKiroSystem(body, prompt)
		return
	}
	switch format {
	case "claude":
		injectClaudeSystem(body, prompt)
		return
	case "gemini", "gemini-cli", "vertex", "antigravity":
		injectGeminiSystem(body, prompt)
		return
	}
	if _, ok := body["instructions"].(string); ok {
		injectInstructionsSystem(body, prompt)
		return
	}
	if _, ok := body["messages"].([]any); ok {
		injectChatSystem(body, prompt)
		return
	}
	if _, ok := body["input"].([]any); ok {
		injectResponsesInputSystem(body, prompt)
	}
}

func tokenSaverHasPrompt(haystack, prompt string) bool {
	if haystack == "" {
		return false
	}
	if haystack == prompt {
		return true
	}
	for _, part := range strings.Split(haystack, tsSep) {
		if part == prompt {
			return true
		}
	}
	return false
}

func tokenSaverDedupAppend(current, prompt string) string {
	if current == "" {
		return prompt
	}
	if tokenSaverHasPrompt(current, prompt) {
		return current
	}
	return current + tsSep + prompt
}

func injectInstructionsSystem(body map[string]any, prompt string) {
	current, ok := body["instructions"].(string)
	if !ok || tokenSaverHasPrompt(current, prompt) {
		return
	}
	body["instructions"] = tokenSaverDedupAppend(current, prompt)
}

func injectChatSystem(body map[string]any, prompt string) {
	messages, ok := body["messages"].([]any)
	if !ok || tokenSaverContainsPromptInMessages(messages, prompt) {
		return
	}
	index := -1
	for i, raw := range messages {
		message, _ := raw.(map[string]any)
		role := ttsString(message["role"])
		if role == "system" || role == "developer" {
			index = i
			break
		}
	}
	if index >= 0 {
		message, _ := messages[index].(map[string]any)
		tokenSaverAppendChatMessage(message, prompt)
		return
	}
	body["messages"] = append([]any{map[string]any{"role": "system", "content": prompt}}, messages...)
}

func tokenSaverContainsPromptInMessages(messages []any, prompt string) bool {
	for _, raw := range messages {
		message, _ := raw.(map[string]any)
		role := ttsString(message["role"])
		if role != "system" && role != "developer" {
			continue
		}
		if content, ok := message["content"].(string); ok && tokenSaverHasPrompt(content, prompt) {
			return true
		}
		if parts, ok := message["content"].([]any); ok {
			for _, rawPart := range parts {
				part, _ := rawPart.(map[string]any)
				if text, ok := part["text"].(string); ok && tokenSaverHasPrompt(text, prompt) {
					return true
				}
			}
		}
	}
	return false
}

func tokenSaverAppendChatMessage(message map[string]any, prompt string) {
	if message == nil {
		return
	}
	switch content := message["content"].(type) {
	case string:
		message["content"] = tokenSaverDedupAppend(content, prompt)
	case []any:
		for _, rawPart := range content {
			part, _ := rawPart.(map[string]any)
			if text, ok := part["text"].(string); ok && text == prompt {
				return
			}
		}
		message["content"] = append(content, map[string]any{"type": "text", "text": prompt})
	default:
		message["content"] = prompt
	}
}

func injectResponsesInputSystem(body map[string]any, prompt string) {
	input, ok := body["input"].([]any)
	if !ok || tokenSaverContainsPromptInResponses(input, prompt) {
		return
	}
	index := -1
	for i, raw := range input {
		item, _ := raw.(map[string]any)
		role := ttsString(item["role"])
		if ttsString(item["type"]) == "message" && (role == "system" || role == "developer") {
			index = i
			break
		}
	}
	if index >= 0 {
		item, _ := input[index].(map[string]any)
		tokenSaverAppendResponsesMessage(item, prompt)
		return
	}
	body["input"] = append([]any{map[string]any{
		"type": "message", "role": "system",
		"content": []any{map[string]any{"type": "input_text", "text": prompt}},
	}}, input...)
}

func tokenSaverContainsPromptInResponses(input []any, prompt string) bool {
	for _, raw := range input {
		item, _ := raw.(map[string]any)
		if ttsString(item["type"]) != "message" {
			continue
		}
		role := ttsString(item["role"])
		if role != "system" && role != "developer" {
			continue
		}
		if content, ok := item["content"].(string); ok && tokenSaverHasPrompt(content, prompt) {
			return true
		}
		if parts, ok := item["content"].([]any); ok {
			for _, rawPart := range parts {
				part, _ := rawPart.(map[string]any)
				if text, ok := part["text"].(string); ok && tokenSaverHasPrompt(text, prompt) {
					return true
				}
			}
		}
	}
	return false
}

func tokenSaverAppendResponsesMessage(item map[string]any, prompt string) {
	if item == nil {
		return
	}
	switch content := item["content"].(type) {
	case string:
		item["content"] = tokenSaverDedupAppend(content, prompt)
	case []any:
		for _, rawPart := range content {
			part, _ := rawPart.(map[string]any)
			if text, ok := part["text"].(string); ok && text == prompt {
				return
			}
		}
		item["content"] = append(content, map[string]any{"type": "input_text", "text": prompt})
	default:
		item["content"] = []any{map[string]any{"type": "input_text", "text": prompt}}
	}
}

func injectClaudeSystem(body map[string]any, prompt string) {
	switch system := body["system"].(type) {
	case string:
		if tokenSaverHasPrompt(system, prompt) {
			return
		}
		body["system"] = tokenSaverDedupAppend(system, prompt)
	case []any:
		for _, rawBlock := range system {
			block, _ := rawBlock.(map[string]any)
			if text, ok := block["text"].(string); ok && text == prompt {
				return
			}
		}
		block := map[string]any{"type": "text", "text": prompt}
		lastCache := -1
		for index := len(system) - 1; index >= 0; index-- {
			existing, _ := system[index].(map[string]any)
			if existing == nil {
				continue
			}
			if cache, ok := existing["cache_control"]; ok && tokenSaverTruthy(cache) {
				lastCache = index
				break
			}
		}
		if lastCache >= 0 {
			system = append(system, nil)
			copy(system[lastCache+1:], system[lastCache:])
			system[lastCache] = block
			body["system"] = system
		} else {
			body["system"] = append(system, block)
		}
	default:
		body["system"] = prompt
	}
}

func injectGeminiSystem(body map[string]any, prompt string) {
	target := body
	if request, ok := body["request"].(map[string]any); ok {
		target = request
	}
	key := "systemInstruction"
	if _, exists := target["system_instruction"]; exists {
		key = "system_instruction"
	}
	system, _ := target[key].(map[string]any)
	if system != nil {
		if parts, ok := system["parts"].([]any); ok {
			for _, rawPart := range parts {
				part, _ := rawPart.(map[string]any)
				if text, ok := part["text"].(string); ok && text == prompt {
					return
				}
			}
			system["parts"] = append(parts, map[string]any{"text": prompt})
			return
		}
	}
	target[key] = map[string]any{"parts": []any{map[string]any{"text": prompt}}}
}

func tokenSaverIsKiroBody(body map[string]any) bool {
	state, ok := body["conversationState"].(map[string]any)
	if !ok {
		return false
	}
	if history, ok := state["history"].([]any); ok {
		for _, raw := range history {
			item, _ := raw.(map[string]any)
			if item == nil {
				continue
			}
			if _, ok := item["userInputMessage"].(map[string]any); ok {
				return true
			}
			if _, ok := item["assistantResponseMessage"].(map[string]any); ok {
				return true
			}
		}
	}
	current, _ := state["currentMessage"].(map[string]any)
	if current != nil {
		if _, ok := current["userInputMessage"].(map[string]any); ok {
			return true
		}
	}
	return false
}

func injectKiroSystem(body map[string]any, prompt string) {
	state, _ := body["conversationState"].(map[string]any)
	if state == nil {
		return
	}
	var target map[string]any
	if history, ok := state["history"].([]any); ok {
		for _, raw := range history {
			item, _ := raw.(map[string]any)
			if user, ok := item["userInputMessage"].(map[string]any); ok {
				target = user
				break
			}
		}
	}
	if target == nil {
		if current, ok := state["currentMessage"].(map[string]any); ok {
			target, _ = current["userInputMessage"].(map[string]any)
		}
	}
	if target == nil {
		return
	}
	content, _ := target["content"].(string)
	next := tokenSaverDedupAppend(content, prompt)
	if next == content {
		return
	}
	target["content"] = next
}
