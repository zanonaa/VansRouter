package handlers

// Porta de src/lib/pxpipe/events.js (append, leitura e agregações).
import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

func pxpipeEventsFile(dataDir string) string {
	return filepath.Join(pxpipeDir(dataDir), "events.jsonl")
}

func pxpipeRotatedFile(dataDir string) string {
	return pxpipeEventsFile(dataDir) + ".1"
}

// PxpipeAppendEvent: fire-and-forget, rotação em 5 MB, ts primeiro (igual JS).
func PxpipeAppendEvent(dataDir string, event map[string]any) {
	defer func() { _ = recover() }()
	if err := os.MkdirAll(pxpipeDir(dataDir), 0o755); err != nil {
		return
	}
	if info, err := os.Stat(pxpipeEventsFile(dataDir)); err == nil && info.Size() > 5*1024*1024 {
		_ = os.Rename(pxpipeEventsFile(dataDir), pxpipeRotatedFile(dataDir))
	}
	payload := map[string]any{"ts": time.Now().UnixMilli()}
	for key, value := range event {
		payload[key] = value
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return
	}
	handle, err := os.OpenFile(pxpipeEventsFile(dataDir), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer handle.Close()
	_, _ = handle.Write(append(encoded, '\n'))
}

// PxpipeReadEvents: rotacionado + atual, ordenado por ts; since/limit opcionais.
func PxpipeReadEvents(dataDir string, sinceMs *float64, limit int) []map[string]any {
	var events []map[string]any
	for _, fileName := range []string{pxpipeRotatedFile(dataDir), pxpipeEventsFile(dataDir)} {
		raw, err := os.ReadFile(fileName)
		if err != nil {
			continue
		}
		for _, line := range splitLines(string(raw)) {
			if line == "" {
				continue
			}
			var ev map[string]any
			if json.Unmarshal([]byte(line), &ev) != nil {
				continue
			}
			if sinceMs != nil && pxpipeNum(ev["ts"]) < *sinceMs {
				continue
			}
			events = append(events, ev)
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		return pxpipeNum(events[i]["ts"]) < pxpipeNum(events[j]["ts"])
	})
	if limit > 0 && len(events) > limit {
		events = events[len(events)-limit:]
	}
	return events
}

func splitLines(raw string) []string {
	var lines []string
	start := 0
	for index := 0; index < len(raw); index++ {
		if raw[index] == '\n' {
			lines = append(lines, stringsTrimCRLF(raw[start:index]))
			start = index + 1
		}
	}
	lines = append(lines, stringsTrimCRLF(raw[start:]))
	return lines
}

func stringsTrimCRLF(value string) string {
	for len(value) > 0 && (value[len(value)-1] == '\r' || value[len(value)-1] == ' ') {
		value = value[:len(value)-1]
	}
	return value
}

func pxpipeNum(value any) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case int:
		return float64(number)
	case int64:
		return float64(number)
	case json.Number:
		parsed, _ := number.Float64()
		return parsed
	default:
		return 0
	}
}

type pxpipeTotals struct {
	Requests          int     `json:"requests"`
	Compressed        int     `json:"compressed"`
	Bypassed          int     `json:"bypassed"`
	Errors            int     `json:"errors"`
	TokensBeforeEst   int     `json:"tokensBeforeEst"`
	TokensAfterEst    int     `json:"tokensAfterEst"`
	TokensSavedEst    int     `json:"tokensSavedEst"`
	SavedPct          float64 `json:"savedPct"`
	ImagesGenerated   int     `json:"imagesGenerated"`
	CompressionTimeMs int     `json:"compressionTimeMs"`
	AvgCompressionMs  int     `json:"avgCompressionMs"`
}

func pxpipeAccumulate(totals *pxpipeTotals, ev map[string]any) {
	totals.Requests++
	if applied, _ := ev["applied"].(bool); applied {
		totals.Compressed++
		totals.TokensBeforeEst += int(pxpipeNum(ev["tokensBeforeEst"]))
		totals.TokensAfterEst += int(pxpipeNum(ev["tokensAfterEst"]))
		totals.TokensSavedEst += int(pxpipeNum(ev["tokensSavedEst"]))
		totals.ImagesGenerated += int(pxpipeNum(ev["imageCount"]))
		totals.CompressionTimeMs += int(pxpipeNum(ev["durationMs"]))
	} else if reason := jsString(ev["reason"]); reason == "transform_error" || reason == "timeout" {
		totals.Errors++
	} else {
		totals.Bypassed++
	}
}

func pxpipeFinalize(totals *pxpipeTotals) {
	if totals.TokensBeforeEst > 0 {
		pct := float64(totals.TokensSavedEst) / float64(totals.TokensBeforeEst) * 100
		// toFixed(2) do JS: arredonda e converte de volta para número.
		parsed, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", pct), 64)
		totals.SavedPct = parsed
	}
	if totals.Compressed > 0 {
		totals.AvgCompressionMs = int(math.Round(float64(totals.CompressionTimeMs) / float64(totals.Compressed)))
	}
}

// PxpipeStats espelha getPxpipeStats: janelas all/today/yesterday/last7d/last30d,
// timeline diária UTC (últimos timelineDays dias) e recentes invertidos.
func PxpipeStats(dataDir string, timelineDays, recentLimit int) map[string]any {
	if timelineDays <= 0 {
		timelineDays = 30
	}
	if recentLimit <= 0 {
		recentLimit = 100
	}
	events := PxpipeReadEvents(dataDir, nil, 0)
	now := time.Now()
	nowMs := now.UnixMilli()
	// setHours(0,0,0,0) do JS usa o fuso local.
	local := now.Local()
	startOfToday := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location()).UnixMilli()
	dayMs := int64(86400000)
	windows := map[string]*pxpipeTotals{
		"all": {}, "today": {}, "yesterday": {}, "last7d": {}, "last30d": {},
	}
	timeline := map[string]*pxpipeTimelineEntry{}
	for index := timelineDays - 1; index >= 0; index-- {
		day := startOfToday - int64(index)*dayMs
		key := pxpipeDayKey(day)
		timeline[key] = &pxpipeTimelineEntry{Date: key}
	}
	for _, ev := range events {
		ts := int64(pxpipeNum(ev["ts"]))
		pxpipeAccumulate(windows["all"], ev)
		if ts >= startOfToday {
			pxpipeAccumulate(windows["today"], ev)
		} else if ts >= startOfToday-dayMs {
			pxpipeAccumulate(windows["yesterday"], ev)
		}
		if ts >= nowMs-7*dayMs {
			pxpipeAccumulate(windows["last7d"], ev)
		}
		if ts >= nowMs-30*dayMs {
			pxpipeAccumulate(windows["last30d"], ev)
		}
		if bucket, ok := timeline[pxpipeDayKey(ts)]; ok {
			bucket.Requests++
			if applied, _ := ev["applied"].(bool); applied {
				bucket.Compressed++
				bucket.TokensSavedEst += int(pxpipeNum(ev["tokensSavedEst"]))
			}
		}
	}
	for _, totals := range windows {
		pxpipeFinalize(totals)
	}
	// Map preserva a ordem de inserção no loop de criação.
	timelineList := []map[string]any{}
	for index := timelineDays - 1; index >= 0; index-- {
		key := pxpipeDayKey(startOfToday - int64(index)*dayMs)
		timelineList = append(timelineList, timeline[key].toMap())
	}
	recent := events
	if len(recent) > recentLimit {
		recent = recent[len(recent)-recentLimit:]
	}
	reversed := make([]map[string]any, 0, len(recent))
	for index := len(recent) - 1; index >= 0; index-- {
		reversed = append(reversed, recent[index])
	}
	return map[string]any{"windows": windows, "timeline": timelineList, "recent": reversed}
}

// pxpipeDayKey: toISOString().slice(0, 10) — data UTC.
func pxpipeDayKey(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02")
}

type pxpipeTimelineEntry struct {
	Date           string
	TokensSavedEst int
	Compressed     int
	Requests       int
}

func (e *pxpipeTimelineEntry) toMap() map[string]any {
	return map[string]any{"date": e.Date, "tokensSavedEst": e.TokensSavedEst, "compressed": e.Compressed, "requests": e.Requests}
}
