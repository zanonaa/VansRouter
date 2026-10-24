package handlers

// Porta fiel de open-sse/rtk (index.js, autodetect.js, filters/*.js): comprime
// o conteúdo de resultados de ferramentas. Nunca toca em tool_calls,
// tool_call_id, function.arguments nem em blocos de erro.

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	rtkRawCap          = 10 * 1024 * 1024
	rtkMinCompressSize = 500
	rtkDetectWindow    = 1024
	rtkMaxBodyBytes    = 512 * 1024

	rtkGitLogMaxLines       = 200
	rtkGitDiffHunkMaxLines  = 100
	rtkDedupLineMax         = 2000
	rtkGrepPerFileMax       = 10
	rtkFindPerDirMax        = 10
	rtkFindTotalDirMax      = 20
	rtkStatusMaxFiles       = 10
	rtkStatusMaxUntracked   = 10
	rtkLsExtSummaryTop      = 5
	rtkTreeMaxLines         = 200
	rtkSearchListPerDirMax  = 10
	rtkSearchListTotalDirs  = 20
	rtkSmartTruncateHead    = 120
	rtkSmartTruncateTail    = 60
	rtkSmartTruncateMinLine = 250
	rtkReadNumberedMinRatio = 0.7
	rtkDeprecationKeep      = 3
)

var rtkLsNoiseDirs = map[string]bool{
	"node_modules": true, ".git": true, "target": true, "__pycache__": true,
	".next": true, "dist": true, "build": true, ".cache": true, ".turbo": true,
	".vercel": true, ".pytest_cache": true, ".mypy_cache": true, ".tox": true,
	".venv": true, "venv": true, "env": true, "coverage": true, ".nyc_output": true,
	".DS_Store": true, "Thumbs.db": true, ".idea": true, ".vscode": true, ".vs": true,
	"*.egg-info": true, ".eggs": true,
}

var (
	rtkReGitDiff          = regexp.MustCompile(`(?m)^diff --git `)
	rtkReGitDiffHunk      = regexp.MustCompile(`(?m)^@@ `)
	rtkReGitStatus        = regexp.MustCompile(`(?m)^On branch |^nothing to commit|^Changes (not |to be )|^Untracked files:`)
	rtkReGitLogHeader     = regexp.MustCompile(`(?m)^[*|/\\ ]*commit [0-9a-f]{7,40}$`)
	rtkRePorcelain        = regexp.MustCompile(`^[ MADRCU?!][ MADRCU?!] \S`)
	rtkReBuildOutput      = regexp.MustCompile(`(?im)^(npm (warn|error|ERR!)|yarn (warn|error)|\s*Compiling\s+\S+|\s*Downloading\s+\S+|added \d+ package|\[ERROR\]|BUILD (SUCCESS|FAILED)|\s*Finished\s+|Successfully (installed|built)|ERROR:)`)
	rtkReTreeGlyph        = regexp.MustCompile(`[├└]──|│  `)
	rtkReLsRow            = regexp.MustCompile(`(?m)^[-dlbcps][rwx-]{9}`)
	rtkReLsTotal          = regexp.MustCompile(`(?m)^total \d+$`)
	rtkReSearchListHeader = regexp.MustCompile(`^Result of search in '[^']*' \(total (\d+) files?\):`)
	rtkReReadNumberedLine = regexp.MustCompile(`^\s*\d+\|`)
	rtkReDigits           = regexp.MustCompile(`^[0-9]+$`)
	rtkReWindowsDrive     = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
	rtkReLsDate           = regexp.MustCompile(`\s+(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+(\d{4}|\d{2}:\d{2})\s+`)
	rtkReStatusBranch     = regexp.MustCompile(`^On branch (\S+)`)
	rtkReStatusLong       = regexp.MustCompile(`^\s*(modified|new file|deleted|renamed|both modified):\s+(.+)$`)
	rtkReLogCommit        = regexp.MustCompile(`(?i)^commit [0-9a-f]{7,40}$`)
	rtkReLogGraphCommit   = regexp.MustCompile(`(?i)^[*|/\\ ]+commit [0-9a-f]{7,40}`)
	rtkReLogAuthor        = regexp.MustCompile(`(?i)^[*|/\\ ]*(Author|Date):`)
	rtkReLogSubject       = regexp.MustCompile(`^[*|/\\ ]*    \S`)
	rtkReLogStat          = regexp.MustCompile(`^\d+ file\w* changed`)
	rtkReLogGraphLine     = regexp.MustCompile(`(?i)^[*|/\\ ]+([0-9a-f]{7,40}\s+.+)`)
	rtkReLogOneline       = regexp.MustCompile(`(?i)^[0-9a-f]{7,40}\s+`)
	rtkReLogPureGraph     = regexp.MustCompile(`^[*|/\\ ]+$`)

	rtkReBuildCargoCont   = regexp.MustCompile(`^\s*(-->|\||\d+\s*\||=)`)
	rtkReBuildNpmErr      = regexp.MustCompile(`(?i)^npm (ERR!|error)`)
	rtkReBuildYarnErr     = regexp.MustCompile(`(?i)^yarn error`)
	rtkReBuildDeprecated  = regexp.MustCompile(`(?i)^npm warn deprecated`)
	rtkReBuildNpmWarn     = regexp.MustCompile(`(?i)^npm warn`)
	rtkReBuildYarnWarn    = regexp.MustCompile(`(?i)^yarn warn`)
	rtkReBuildError       = regexp.MustCompile(`(?i)^error(\[|:)`)
	rtkReBuildWarning     = regexp.MustCompile(`(?i)^warning(\[|:)`)
	rtkReBuildMavenErr    = regexp.MustCompile(`^ERROR:`)
	rtkReBuildBracketErr  = regexp.MustCompile(`^\[ERROR\]`)
	rtkReBuildFailed      = regexp.MustCompile(`^BUILD FAILED`)
	rtkReBuildBracketWarn = regexp.MustCompile(`^\[WARNING\]`)
	rtkReBuildCompiling   = regexp.MustCompile(`(?i)^\s*Compiling\s+\S+`)
	rtkReBuildDownloading = regexp.MustCompile(`(?i)^\s*Downloading\s+\S+`)
	rtkReBuildFetching    = regexp.MustCompile(`^Fetching\s+`)
	rtkReBuildSummaries   = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^(added|removed|changed|audited|installed)\s+\d+\s+package`),
		regexp.MustCompile(`(?i)^\s*Finished\s+`),
		regexp.MustCompile(`(?i)^BUILD SUCCESS`),
		regexp.MustCompile(`(?i)^\d+\s+(vulnerabilities|packages?|warnings?|errors?)`),
		regexp.MustCompile(`(?i)^Successfully (installed|built)`),
		regexp.MustCompile(`(?i)^To address .* issues`),
		regexp.MustCompile("(?i)^Run `npm (audit|fund)`"),
		regexp.MustCompile(`(?i)packages are looking for funding`),
	}
)

type rtkHit struct {
	Shape  string
	Filter string
	Saved  int
}

type rtkStats struct {
	BytesBefore int
	BytesAfter  int
	Hits        []rtkHit
}

// rtkFilter espelha uma função exportada dos filters JS (fn + filterName).
type rtkFilter struct {
	name string
	run  func(string) string
}

func compressRTKMessages(body map[string]any) *rtkStats {
	if body == nil {
		return nil
	}
	if estimateRTKBodyBytes(body) > rtkMaxBodyBytes {
		return nil
	}
	if _, ok := body["conversationState"].(map[string]any); ok {
		return compressRTKKiro(body)
	}
	items, ok := body["messages"].([]any)
	if !ok {
		items, ok = body["input"].([]any)
	}
	if !ok {
		return nil
	}

	stats := &rtkStats{}
	for _, raw := range items {
		msg, ok := raw.(map[string]any)
		if !ok || msg == nil {
			continue
		}

		// OpenAI Responses: {type:"function_call_output", output: string|[{type:"input_text",text}]}
		if ttsString(msg["type"]) == "function_call_output" {
			switch output := msg["output"].(type) {
			case string:
				msg["output"] = compressRTKText(output, stats, "openai-responses-string")
			case []any:
				for _, rawPart := range output {
					part, ok := rawPart.(map[string]any)
					if !ok || ttsString(part["type"]) != "input_text" {
						continue
					}
					if text, ok := part["text"].(string); ok {
						part["text"] = compressRTKText(text, stats, "openai-responses-array")
					}
				}
			}
			continue
		}

		if ttsString(msg["role"]) == "tool" {
			if content, ok := msg["content"].(string); ok {
				msg["content"] = compressRTKText(content, stats, "openai-tool")
				continue
			}
		}

		content, ok := msg["content"].([]any)
		if !ok {
			continue
		}

		// OpenAI tool message com content em blocos de texto.
		if ttsString(msg["role"]) == "tool" {
			for _, rawPart := range content {
				part, ok := rawPart.(map[string]any)
				if !ok || ttsString(part["type"]) != "text" {
					continue
				}
				if text, ok := part["text"].(string); ok {
					part["text"] = compressRTKText(text, stats, "openai-tool-array")
				}
			}
			continue
		}

		for _, rawBlock := range content {
			block, ok := rawBlock.(map[string]any)
			if !ok || ttsString(block["type"]) != "tool_result" {
				continue
			}
			if isError, ok := block["is_error"].(bool); ok && isError {
				continue // preserva traces de erro
			}
			switch blockContent := block["content"].(type) {
			case string:
				block["content"] = compressRTKText(blockContent, stats, "claude-string")
			case []any:
				for _, rawPart := range blockContent {
					part, ok := rawPart.(map[string]any)
					if !ok || ttsString(part["type"]) != "text" {
						continue
					}
					if text, ok := part["text"].(string); ok {
						part["text"] = compressRTKText(text, stats, "claude-array")
					}
				}
			}
		}
	}
	return stats
}

// compressRTKKiro porta compressKiroFormat (history + currentMessage).
func compressRTKKiro(body map[string]any) *rtkStats {
	state, _ := body["conversationState"].(map[string]any)
	stats := &rtkStats{}
	all := []any{}
	if history, ok := state["history"].([]any); ok {
		all = append(all, history...)
	}
	if current, ok := state["currentMessage"]; ok && current != nil {
		all = append(all, current)
	}
	for _, raw := range all {
		msg, _ := raw.(map[string]any)
		user, _ := msg["userInputMessage"].(map[string]any)
		context, _ := user["userInputMessageContext"].(map[string]any)
		results, _ := context["toolResults"].([]any)
		for _, rawResult := range results {
			result, _ := rawResult.(map[string]any)
			if ttsString(result["status"]) == "error" {
				continue
			}
			parts, _ := result["content"].([]any)
			for _, rawPart := range parts {
				part, _ := rawPart.(map[string]any)
				if text, ok := part["text"].(string); ok {
					part["text"] = compressRTKText(text, stats, "kiro-tool-result")
				}
			}
		}
	}
	return stats
}

func compressRTKText(text string, stats *rtkStats, shape string) string {
	bytesIn := len(text)
	stats.BytesBefore += bytesIn
	if bytesIn < rtkMinCompressSize || bytesIn > rtkRawCap {
		stats.BytesAfter += bytesIn
		return text
	}
	filter := autoDetectRTKFilter(text)
	if filter == nil {
		stats.BytesAfter += bytesIn
		return text
	}
	out := safeApplyRTK(filter, text)
	// Safety: nunca vazio, nunca maior que a entrada.
	if out == "" || len(out) >= bytesIn {
		stats.BytesAfter += bytesIn
		return text
	}
	stats.BytesAfter += len(out)
	stats.Hits = append(stats.Hits, rtkHit{Shape: shape, Filter: filter.name, Saved: bytesIn - len(out)})
	return out
}

func safeApplyRTK(filter *rtkFilter, text string) (out string) {
	defer func() {
		if recover() != nil {
			out = text
		}
	}()
	if filter == nil || filter.run == nil {
		return text
	}
	return filter.run(text)
}

func formatRTKLog(stats *rtkStats) string {
	if stats == nil || len(stats.Hits) == 0 {
		return ""
	}
	saved := stats.BytesBefore - stats.BytesAfter
	pct := 0.0
	if stats.BytesBefore > 0 {
		pct = float64(saved) / float64(stats.BytesBefore) * 100
	}
	seen := map[string]bool{}
	filters := []string{}
	for _, hit := range stats.Hits {
		if seen[hit.Filter] {
			continue
		}
		seen[hit.Filter] = true
		filters = append(filters, hit.Filter)
	}
	return fmt.Sprintf("saved %dB / %dB (%.1f%%) via [%s] hits=%d", saved, stats.BytesBefore, pct, strings.Join(filters, ","), len(stats.Hits))
}

func estimateRTKBodyBytes(body map[string]any) int {
	var items []any
	if messages, ok := body["messages"].([]any); ok {
		items = messages
	} else if input, ok := body["input"].([]any); ok {
		items = input
	}
	if items == nil {
		return estimateRTKValueBytes(body)
	}
	total := 0
	for _, item := range items {
		total += 32 + estimateRTKValueBytes(item)
	}
	return total
}

func estimateRTKValueBytes(value any) int {
	switch typed := value.(type) {
	case string:
		return len(typed)
	case map[string]any:
		total := 0
		for _, child := range typed {
			total += estimateRTKValueBytes(child)
		}
		return total
	case []any:
		total := 0
		for _, child := range typed {
			total += estimateRTKValueBytes(child)
		}
		return total
	default:
		return 0
	}
}

func autoDetectRTKFilter(text string) *rtkFilter {
	head := text
	if runes := []rune(text); len(runes) > rtkDetectWindow {
		head = string(runes[:rtkDetectWindow])
	}

	if rtkReGitLogHeader.MatchString(head) {
		return &rtkFilter{name: "git-log", run: rtkGitLog}
	}
	if rtkReGitDiff.MatchString(head) || rtkReGitDiffHunk.MatchString(head) {
		return &rtkFilter{name: "git-diff", run: rtkGitDiff}
	}
	if rtkReGitStatus.MatchString(head) {
		return &rtkFilter{name: "git-status", run: rtkGitStatus}
	}
	// Build output ANTES do porcelain: evita "Compiling" virar git-status.
	if rtkReBuildOutput.MatchString(head) {
		return &rtkFilter{name: "build-output", run: rtkBuildOutput}
	}
	if rtkIsMostlyPorcelain(head) {
		return &rtkFilter{name: "git-status", run: rtkGitStatus}
	}

	lines := strings.Split(head, "\n")
	nonEmpty := []string{}
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}
	first5 := nonEmpty
	if len(first5) > 5 {
		first5 = first5[:5]
	}
	for _, line := range first5 {
		if rtkIsGrepLine(line) {
			return &rtkFilter{name: "grep", run: rtkGrep}
		}
	}
	if len(nonEmpty) >= 3 && rtkAllPathLike(nonEmpty) {
		return &rtkFilter{name: "find", run: rtkFind}
	}
	if rtkReTreeGlyph.MatchString(head) {
		return &rtkFilter{name: "tree", run: rtkTree}
	}
	if rtkReLsTotal.MatchString(head) || rtkCountMatches(head, rtkReLsRow) >= 3 {
		return &rtkFilter{name: "ls", run: rtkLS}
	}
	if rtkReSearchListHeader.MatchString(head) {
		return &rtkFilter{name: "search-list", run: rtkSearchList}
	}
	if len(lines) >= rtkSmartTruncateMinLine && rtkIsLineNumbered(lines) {
		return &rtkFilter{name: "read-numbered", run: rtkReadNumbered}
	}
	if len(nonEmpty) >= 5 {
		return &rtkFilter{name: "dedup-log", run: rtkDedupLog}
	}
	if len(strings.Split(text, "\n")) >= rtkSmartTruncateMinLine {
		return &rtkFilter{name: "smart-truncate", run: rtkSmartTruncate}
	}
	return nil
}

func rtkIsGrepLine(line string) bool {
	first := strings.Index(line, ":")
	if first == -1 {
		return false
	}
	second := strings.Index(line[first+1:], ":")
	if second == -1 {
		return false
	}
	second += first + 1
	return rtkReDigits.MatchString(line[first+1 : second])
}

func rtkIsPathLike(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if rtkReWindowsDrive.MatchString(trimmed) {
		return true
	}
	if strings.Contains(trimmed, ":") {
		return false
	}
	return strings.HasPrefix(trimmed, ".") || strings.HasPrefix(trimmed, "/") || strings.Contains(trimmed, "/")
}

func rtkAllPathLike(lines []string) bool {
	for _, line := range lines {
		if !rtkIsPathLike(line) {
			return false
		}
	}
	return true
}

func rtkIsMostlyPorcelain(head string) bool {
	lines := []string{}
	for _, line := range strings.Split(head, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) < 3 {
		return false
	}
	hits := 0
	for _, line := range lines {
		if rtkRePorcelain.MatchString(line) {
			hits++
		}
	}
	return float64(hits)/float64(len(lines)) >= 0.6
}

func rtkIsLineNumbered(lines []string) bool {
	hits, nonEmpty := 0, 0
	sample := lines
	if len(sample) > 100 {
		sample = sample[:100]
	}
	for _, line := range sample {
		if len(line) == 0 {
			continue
		}
		nonEmpty++
		if rtkReReadNumberedLine.MatchString(line) {
			hits++
		}
	}
	if nonEmpty < 5 {
		return false
	}
	return float64(hits)/float64(nonEmpty) >= rtkReadNumberedMinRatio
}

func rtkCountMatches(text string, re *regexp.Regexp) int {
	return len(re.FindAllStringIndex(text, -1))
}

func rtkSmartTruncate(input string) string {
	lines := strings.Split(input, "\n")
	if len(lines) < rtkSmartTruncateMinLine {
		return input
	}
	head := lines[:rtkSmartTruncateHead]
	tail := lines[len(lines)-rtkSmartTruncateTail:]
	cut := len(lines) - len(head) - len(tail)
	return strings.Join(append(append(append([]string{}, head...), fmt.Sprintf("... +%d lines truncated", cut)), tail...), "\n")
}

func rtkGitDiff(input string) string {
	result := []string{}
	currentFile := ""
	added, removed := 0, 0
	inHunk := false
	hunkShown, hunkSkipped := 0, 0
	wasTruncated := false
	lines := strings.Split(input, "\n")

	flushSkipped := func() {
		if hunkSkipped > 0 {
			result = append(result, fmt.Sprintf("  ... (%d lines truncated)", hunkSkipped))
			wasTruncated = true
			hunkSkipped = 0
		}
	}

outer:
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git"):
			flushSkipped()
			if currentFile != "" && (added > 0 || removed > 0) {
				result = append(result, fmt.Sprintf("  +%d -%d", added, removed))
			}
			parts := strings.Split(line, " b/")
			if len(parts) > 1 {
				currentFile = strings.Join(parts[1:], " b/")
			} else {
				currentFile = "unknown"
			}
			result = append(result, "\n"+currentFile)
			added, removed = 0, 0
			inHunk = false
			hunkShown = 0
		case strings.HasPrefix(line, "@@"):
			flushSkipped()
			inHunk = true
			hunkShown = 0
			result = append(result, "  "+line)
		case inHunk:
			switch {
			case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
				added++
				if hunkShown < rtkGitDiffHunkMaxLines {
					result = append(result, "  "+line)
					hunkShown++
				} else {
					hunkSkipped++
				}
			case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
				removed++
				if hunkShown < rtkGitDiffHunkMaxLines {
					result = append(result, "  "+line)
					hunkShown++
				} else {
					hunkSkipped++
				}
			case hunkShown < rtkGitDiffHunkMaxLines && !strings.HasPrefix(line, "\\"):
				if hunkShown > 0 {
					result = append(result, "  "+line)
					hunkShown++
				}
			}
		}
		if len(result) >= 500 {
			result = append(result, "\n... (more changes truncated)")
			wasTruncated = true
			break outer
		}
	}
	flushSkipped()
	if currentFile != "" && (added > 0 || removed > 0) {
		result = append(result, fmt.Sprintf("  +%d -%d", added, removed))
	}
	if wasTruncated {
		result = append(result, "[full diff: rtk git diff --no-compact]")
	}
	return strings.Join(result, "\n")
}

func rtkGitStatus(input string) string {
	lines := strings.Split(input, "\n")
	if len(lines) == 0 || (len(lines) == 1 && strings.TrimSpace(lines[0]) == "") {
		return "Clean working tree"
	}
	branch := ""
	stagedFiles, modifiedFiles, untrackedFiles := []string{}, []string{}, []string{}
	staged, modified, untracked, conflicts := 0, 0, 0, 0

	for _, raw := range lines {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if match := rtkReStatusBranch.FindStringSubmatch(raw); match != nil {
			branch = match[1]
			continue
		}
		if strings.HasPrefix(raw, "##") {
			branch = strings.TrimLeft(strings.TrimPrefix(raw, "##"), " \t")
			continue
		}
		if len(raw) >= 3 && rtkRePorcelain.MatchString(raw) {
			x, y := raw[0], raw[1]
			file := raw[3:]
			if raw[:2] == "??" {
				untracked++
				untrackedFiles = append(untrackedFiles, file)
				continue
			}
			if strings.ContainsRune("MADRC", rune(x)) {
				staged++
				stagedFiles = append(stagedFiles, file)
			} else if x == 'U' {
				conflicts++
			}
			if y == 'M' || y == 'D' {
				modified++
				modifiedFiles = append(modifiedFiles, file)
			}
			continue
		}
		if match := rtkReStatusLong.FindStringSubmatch(raw); match != nil {
			kind, path := match[1], strings.TrimSpace(match[2])
			switch kind {
			case "both modified":
				conflicts++
			case "modified", "deleted":
				modified++
				modifiedFiles = append(modifiedFiles, path)
			case "new file", "renamed":
				staged++
				stagedFiles = append(stagedFiles, path)
			}
		}
	}

	var out strings.Builder
	if branch != "" {
		out.WriteString("* " + branch + "\n")
	}
	writeGroup := func(label string, count int, files []string, limit int) {
		if count <= 0 {
			return
		}
		out.WriteString(fmt.Sprintf("%s: %d files\n", label, count))
		show := files
		if len(show) > limit {
			show = show[:limit]
		}
		for _, file := range show {
			out.WriteString("   " + file + "\n")
		}
		if len(files) > limit {
			out.WriteString(fmt.Sprintf("   ... +%d more\n", len(files)-limit))
		}
	}
	writeGroup("+ Staged", staged, stagedFiles, rtkStatusMaxFiles)
	writeGroup("~ Modified", modified, modifiedFiles, rtkStatusMaxFiles)
	writeGroup("? Untracked", untracked, untrackedFiles, rtkStatusMaxUntracked)
	if conflicts > 0 {
		out.WriteString(fmt.Sprintf("conflicts: %d files\n", conflicts))
	}
	if staged == 0 && modified == 0 && untracked == 0 && conflicts == 0 {
		out.WriteString("clean — nothing to commit\n")
	}
	return strings.TrimRight(out.String(), "\n")
}

func rtkGitLog(input string) string {
	if input == "" {
		return ""
	}
	lines := strings.Split(input, "\n")
	out := []string{}
	skipped := 0
	inCommit := false
	subjectSeen := false

	pushLine := func(line string) bool {
		if len(out) < rtkGitLogMaxLines {
			out = append(out, line)
			return true
		}
		skipped++
		return false
	}

	reCommit := rtkReLogCommit
	reGraphCommit := rtkReLogGraphCommit
	reAuthor := rtkReLogAuthor
	reSubject := rtkReLogSubject
	reStat := rtkReLogStat
	reGraphLine := rtkReLogGraphLine
	reOneline := rtkReLogOneline
	rePureGraph := rtkReLogPureGraph

	for _, raw := range lines {
		line := strings.TrimRightFunc(raw, unicode.IsSpace)
		trimmed := strings.TrimSpace(line)

		if reCommit.MatchString(trimmed) || reGraphCommit.MatchString(trimmed) {
			inCommit = true
			subjectSeen = false
			pushLine(line)
			continue
		}
		if inCommit {
			if reAuthor.MatchString(trimmed) {
				pushLine(trimmed)
				continue
			}
			if trimmed == "" {
				continue
			}
			if !subjectSeen && reSubject.MatchString(line) {
				pushLine("  Subject: " + trimmed)
				subjectSeen = true
				continue
			}
			if reStat.MatchString(trimmed) {
				pushLine("  " + trimmed)
				continue
			}
			if strings.HasPrefix(trimmed, "diff --git ") {
				pushLine("  ... diff body omitted")
				continue
			}
			continue
		}
		if match := reGraphLine.FindStringSubmatch(trimmed); match != nil {
			pushLine(match[1])
			continue
		}
		if reOneline.MatchString(trimmed) {
			pushLine(trimmed)
			continue
		}
		if rePureGraph.MatchString(trimmed) && strings.ContainsAny(trimmed, "*|/\\") {
			continue
		}
		pushLine(trimmed)
	}
	if skipped > 0 {
		out = append(out, fmt.Sprintf("... (%d more lines)", skipped))
	}
	result := strings.Join(out, "\n")
	if result == "" && input != "" {
		return input
	}
	if len(result) > len(input) {
		return input
	}
	return result
}

func rtkBuildOutput(input string) string {
	lines := strings.Split(input, "\n")
	if len(lines) == 0 {
		return input
	}
	reCargoCont := rtkReBuildCargoCont
	reNpmErr := rtkReBuildNpmErr
	reYarnErr := rtkReBuildYarnErr
	reNpmDeprecated := rtkReBuildDeprecated
	reNpmWarn := rtkReBuildNpmWarn
	reYarnWarn := rtkReBuildYarnWarn
	reError := rtkReBuildError
	reWarning := rtkReBuildWarning
	reMavenErr := rtkReBuildMavenErr
	reBracketErr := rtkReBuildBracketErr
	reBuildFailed := rtkReBuildFailed
	reBracketWarn := rtkReBuildBracketWarn
	reCompiling := rtkReBuildCompiling
	reDownloading := rtkReBuildDownloading
	reFetching := rtkReBuildFetching
	reSummary := rtkReBuildSummaries

	errors := []string{}
	warnings := []string{}
	deprecations := []string{}
	summary := ""
	compilingCount, downloadingCount := 0, 0
	inCargoError := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if inCargoError {
			if trimmed == "" {
				inCargoError = false
				continue
			}
			if reCargoCont.MatchString(line) {
				errors = append(errors, line)
				continue
			}
			inCargoError = false
		}
		if trimmed == "" {
			continue
		}
		switch {
		case reNpmErr.MatchString(trimmed) || reYarnErr.MatchString(trimmed):
			errors = append(errors, line)
		case reNpmDeprecated.MatchString(trimmed):
			deprecations = append(deprecations, line)
		case reNpmWarn.MatchString(trimmed) || reYarnWarn.MatchString(trimmed):
			warnings = append(warnings, line)
		case reError.MatchString(trimmed) || strings.HasPrefix(trimmed, "error -->"):
			errors = append(errors, line)
			inCargoError = true
		case reWarning.MatchString(trimmed) || strings.HasPrefix(trimmed, "warning -->"):
			warnings = append(warnings, line)
			inCargoError = true
		case reMavenErr.MatchString(trimmed):
			errors = append(errors, line)
		case reBracketErr.MatchString(trimmed) || reBuildFailed.MatchString(trimmed):
			errors = append(errors, line)
		case reBracketWarn.MatchString(trimmed):
			warnings = append(warnings, line)
		case reCompiling.MatchString(trimmed):
			compilingCount++
		case reDownloading.MatchString(trimmed) || reFetching.MatchString(trimmed):
			downloadingCount++
		default:
			for _, re := range reSummary {
				if re.MatchString(trimmed) {
					if summary == "" {
						summary = line
					} else {
						summary += "\n" + line
					}
					break
				}
			}
		}
	}

	var out strings.Builder
	keepDep := deprecations
	if len(keepDep) > rtkDeprecationKeep {
		keepDep = keepDep[:rtkDeprecationKeep]
	}
	for _, dep := range keepDep {
		out.WriteString(dep + "\n")
	}
	if len(deprecations) > rtkDeprecationKeep {
		out.WriteString(fmt.Sprintf("... +%d more deprecated packages\n", len(deprecations)-rtkDeprecationKeep))
	}
	if compilingCount > 0 {
		out.WriteString(fmt.Sprintf("Compiled %d packages\n", compilingCount))
	}
	if downloadingCount > 0 {
		out.WriteString(fmt.Sprintf("Downloaded %d packages\n", downloadingCount))
	}
	for _, line := range errors {
		out.WriteString(line + "\n")
	}
	keepWarnings := warnings
	if len(keepWarnings) > 5 {
		keepWarnings = keepWarnings[:5]
	}
	for _, line := range keepWarnings {
		out.WriteString(line + "\n")
	}
	if len(warnings) > 5 {
		out.WriteString(fmt.Sprintf("... +%d more warnings\n", len(warnings)-5))
	}
	if summary != "" {
		out.WriteString(summary + "\n")
	}
	result := strings.TrimRight(out.String(), "\n")
	if result == "" {
		return input
	}
	return result
}

func rtkGrep(input string) string {
	type match struct {
		lineNo  string
		content string
	}
	byFile := map[string][]match{}
	order := []string{}
	total := 0
	for _, line := range strings.Split(input, "\n") {
		first := strings.Index(line, ":")
		if first == -1 {
			continue
		}
		second := strings.Index(line[first+1:], ":")
		if second == -1 {
			continue
		}
		second += first + 1
		file := line[:first]
		lineNo := line[first+1 : second]
		content := line[second+1:]
		if !rtkReDigits.MatchString(lineNo) {
			continue
		}
		total++
		if _, exists := byFile[file]; !exists {
			order = append(order, file)
		}
		byFile[file] = append(byFile[file], match{lineNo: lineNo, content: content})
	}
	if total == 0 {
		return input
	}
	sort.Strings(order)
	var out strings.Builder
	out.WriteString(fmt.Sprintf("%d matches in %dF:\n\n", total, len(order)))
	for _, file := range order {
		matches := byFile[file]
		out.WriteString(fmt.Sprintf("[file] %s (%d):\n", file, len(matches)))
		show := matches
		if len(show) > rtkGrepPerFileMax {
			show = show[:rtkGrepPerFileMax]
		}
		for _, match := range show {
			out.WriteString(fmt.Sprintf("  %4s: %s\n", match.lineNo, strings.TrimSpace(match.content)))
		}
		if len(matches) > rtkGrepPerFileMax {
			out.WriteString(fmt.Sprintf("  +%d\n", len(matches)-rtkGrepPerFileMax))
		}
		out.WriteString("\n")
	}
	return out.String()
}

func rtkFind(input string) string {
	lines := []string{}
	for _, line := range strings.Split(input, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return input
	}
	byDir := map[string][]string{}
	order := []string{}
	for _, path := range lines {
		lastSep := strings.LastIndex(path, "/")
		if backslash := strings.LastIndex(path, "\\"); backslash > lastSep {
			lastSep = backslash
		}
		dir, basename := ".", path
		if lastSep != -1 {
			dir = path[:lastSep]
			if dir == "" {
				dir = "/"
			}
			basename = path[lastSep+1:]
		}
		if _, exists := byDir[dir]; !exists {
			order = append(order, dir)
		}
		byDir[dir] = append(byDir[dir], basename)
	}
	sort.Strings(order)
	var out strings.Builder
	out.WriteString(fmt.Sprintf("%d files in %d dirs:\n\n", len(lines), len(order)))
	showDirs := order
	if len(showDirs) > rtkFindTotalDirMax {
		showDirs = showDirs[:rtkFindTotalDirMax]
	}
	for _, dir := range showDirs {
		files := byDir[dir]
		out.WriteString(fmt.Sprintf("%s/  (%d)\n", strings.ReplaceAll(dir, "\\", "/"), len(files)))
		showFiles := files
		if len(showFiles) > rtkFindPerDirMax {
			showFiles = showFiles[:rtkFindPerDirMax]
		}
		for _, file := range showFiles {
			out.WriteString("  " + file + "\n")
		}
		if len(files) > rtkFindPerDirMax {
			out.WriteString(fmt.Sprintf("  +%d\n", len(files)-rtkFindPerDirMax))
		}
	}
	if len(order) > rtkFindTotalDirMax {
		out.WriteString(fmt.Sprintf("\n+%d more dirs\n", len(order)-rtkFindTotalDirMax))
	}
	return out.String()
}

func rtkDedupLog(input string) string {
	lines := strings.Split(input, "\n")
	out := []string{}
	prev := ""
	hasPrev := false
	runCount := 0
	blankStreak := 0

	flushRun := func() {
		if hasPrev && runCount > 1 {
			out = append(out, fmt.Sprintf("  ... (%d duplicate lines)", runCount-1))
		}
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if blankStreak < 1 {
				out = append(out, line)
			}
			blankStreak++
			flushRun()
			prev, hasPrev, runCount = "", false, 0
			continue
		}
		blankStreak = 0
		if hasPrev && line == prev {
			runCount++
			continue
		}
		flushRun()
		out = append(out, line)
		prev, hasPrev, runCount = line, true, 1
		if len(out) >= rtkDedupLineMax {
			out = append(out, fmt.Sprintf("... (truncated at %d lines)", rtkDedupLineMax))
			return strings.Join(out, "\n")
		}
	}
	flushRun()
	return strings.Join(out, "\n")
}

func rtkHumanSize(bytes int64) string {
	if bytes >= 1048576 {
		return fmt.Sprintf("%.1fM", float64(bytes)/1048576)
	}
	if bytes >= 1024 {
		return fmt.Sprintf("%.1fK", float64(bytes)/1024)
	}
	return fmt.Sprintf("%dB", bytes)
}

func rtkParseLSLine(line string) (fileType string, size int64, name string, ok bool) {
	indexes := rtkReLsDate.FindStringIndex(line)
	if indexes == nil {
		return "", 0, "", false
	}
	name = line[indexes[1]:]
	beforeDate := line[:indexes[0]]
	beforeParts := strings.Fields(beforeDate)
	if len(beforeParts) < 4 {
		return "", 0, "", false
	}
	perms := beforeParts[0]
	fileType = perms[:1]
	for index := len(beforeParts) - 1; index >= 0; index-- {
		number, err := strconv.ParseInt(beforeParts[index], 10, 64)
		if err == nil && strconv.FormatInt(number, 10) == beforeParts[index] {
			size = number
			break
		}
	}
	return fileType, size, name, true
}

func rtkLS(input string) string {
	dirs := []string{}
	files := []struct {
		name string
		size string
	}{}
	byExt := map[string]int{}
	extOrder := []string{}
	for _, line := range strings.Split(input, "\n") {
		if strings.HasPrefix(line, "total ") || len(line) == 0 {
			continue
		}
		fileType, size, name, ok := rtkParseLSLine(line)
		if !ok || name == "." || name == ".." {
			continue
		}
		if rtkLsNoiseDirs[name] {
			continue
		}
		switch fileType {
		case "d":
			dirs = append(dirs, name)
		case "-", "l":
			dot := strings.LastIndex(name, ".")
			ext := "no ext"
			if dot > 0 {
				ext = name[dot:]
			}
			if _, exists := byExt[ext]; !exists {
				extOrder = append(extOrder, ext)
			}
			byExt[ext]++
			files = append(files, struct {
				name string
				size string
			}{name: name, size: rtkHumanSize(size)})
		}
	}
	if len(dirs) == 0 && len(files) == 0 {
		return input
	}
	var out strings.Builder
	for _, dir := range dirs {
		out.WriteString(dir + "/\n")
	}
	for _, file := range files {
		out.WriteString(file.name + "  " + file.size + "\n")
	}
	summary := fmt.Sprintf("\nSummary: %d files, %d dirs", len(files), len(dirs))
	if len(byExt) > 0 {
		sort.SliceStable(extOrder, func(i, j int) bool {
			return byExt[extOrder[i]] > byExt[extOrder[j]]
		})
		show := extOrder
		if len(show) > rtkLsExtSummaryTop {
			show = show[:rtkLsExtSummaryTop]
		}
		parts := []string{}
		for _, ext := range show {
			parts = append(parts, fmt.Sprintf("%d %s", byExt[ext], ext))
		}
		summary += " (" + strings.Join(parts, ", ")
		if len(extOrder) > rtkLsExtSummaryTop {
			summary += fmt.Sprintf(", +%d more", len(extOrder)-rtkLsExtSummaryTop)
		}
		summary += ")"
	}
	return out.String() + summary
}

func rtkTree(input string) string {
	lines := strings.Split(input, "\n")
	if len(lines) == 0 {
		return input
	}
	filtered := []string{}
	for _, line := range lines {
		if strings.Contains(line, "director") && strings.Contains(line, "file") {
			continue
		}
		if strings.TrimSpace(line) == "" && len(filtered) == 0 {
			continue
		}
		filtered = append(filtered, line)
	}
	for len(filtered) > 0 && strings.TrimSpace(filtered[len(filtered)-1]) == "" {
		filtered = filtered[:len(filtered)-1]
	}
	if len(filtered) > rtkTreeMaxLines {
		cut := len(filtered) - rtkTreeMaxLines
		return strings.Join(filtered[:rtkTreeMaxLines], "\n") + fmt.Sprintf("\n... +%d more lines", cut)
	}
	return strings.Join(filtered, "\n")
}

func rtkReadNumbered(input string) string {
	lines := strings.Split(input, "\n")
	if len(lines) < rtkSmartTruncateMinLine {
		return input
	}
	head := lines[:rtkSmartTruncateHead]
	tail := lines[len(lines)-rtkSmartTruncateTail:]
	cut := len(lines) - len(head) - len(tail)
	combined := append([]string{}, head...)
	combined = append(combined, fmt.Sprintf("... +%d lines truncated (file continues)", cut))
	combined = append(combined, tail...)
	return strings.Join(combined, "\n")
}

func rtkSearchList(input string) string {
	lines := strings.Split(input, "\n")
	if len(lines) == 0 {
		return input
	}
	header := lines[0]
	paths := []string{}
	for _, raw := range lines[1:] {
		trimmed := strings.TrimSpace(raw)
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		paths = append(paths, trimmed[2:])
	}
	if len(paths) == 0 {
		return input
	}
	byDir := map[string][]string{}
	order := []string{}
	for _, path := range paths {
		slash := strings.LastIndex(path, "/")
		dir, name := ".", path
		if slash != -1 {
			dir = path[:slash]
			if dir == "" {
				dir = "/"
			}
			name = path[slash+1:]
		}
		if _, exists := byDir[dir]; !exists {
			order = append(order, dir)
		}
		byDir[dir] = append(byDir[dir], name)
	}
	sort.Strings(order)
	var out strings.Builder
	out.WriteString(fmt.Sprintf("%s\n%d files in %d dirs:\n\n", header, len(paths), len(order)))
	showDirs := order
	if len(showDirs) > rtkSearchListTotalDirs {
		showDirs = showDirs[:rtkSearchListTotalDirs]
	}
	for _, dir := range showDirs {
		names := byDir[dir]
		out.WriteString(fmt.Sprintf("%s/ (%d):\n", dir, len(names)))
		showNames := names
		if len(showNames) > rtkSearchListPerDirMax {
			showNames = showNames[:rtkSearchListPerDirMax]
		}
		for _, name := range showNames {
			out.WriteString("  " + name + "\n")
		}
		if len(names) > rtkSearchListPerDirMax {
			out.WriteString(fmt.Sprintf("  +%d\n", len(names)-rtkSearchListPerDirMax))
		}
		out.WriteString("\n")
	}
	if len(order) > rtkSearchListTotalDirs {
		out.WriteString(fmt.Sprintf("+%d more dirs\n", len(order)-rtkSearchListTotalDirs))
	}
	return strings.TrimRight(out.String(), "\n")
}
