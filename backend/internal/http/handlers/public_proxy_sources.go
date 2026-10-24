package handlers

// Porta de src/lib/network/publicProxySources.js + src/shared/constants/publicProxySources.js.
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	freeproxydbSearchURL = "https://freeproxydb.com/api/proxy/search"
	vpslabBaseURL        = "https://raw.githubusercontent.com/VPSLabCloud/VPSLab-Free-Proxy-List/main"
	sourcesFetchTimeout  = 20 * time.Second
)

// publicProxyFetcher hook para testes (default: HTTP real).
type publicProxyFetcher func(ctx context.Context, sourceURL, sourceLabel, accept string) (int, string, []byte, error)

func publicProxySourceMeta(sourceID string) map[string]any {
	switch sourceID {
	case "freeproxydb":
		return map[string]any{"label": "FreeProxyDB"}
	case "vpslab":
		return map[string]any{"label": "VPSLab"}
	}
	return nil
}

func publicProxySourceCaps(sourceID string) (protocols []string, supportsCountry, supportsAnonymity, supportsHTTPSOnly, supportsMaxSpeed, supportsOrderBy bool, minCount, maxCount, defaultCount int) {
	switch sourceID {
	case "freeproxydb":
		return []string{"http", "socks5"}, true, true, true, true, true, 1, 100, 50
	case "vpslab":
		return []string{"http", "socks5"}, false, true, true, false, false, 1, 500, 100
	}
	return nil, false, false, false, false, false, 0, 0, 0
}

func publicProxyAnonymityLevels() []string {
	return []string{"elite", "anonymous", "transparent"}
}

// normalizeProxyURLKey espelha normalizeProxyUrlKey (URL.toString() do JS:
// http/https especiais recebem "/" trailing; outros mantêm o caminho vazio).
func normalizeProxyURLKey(value any) string {
	trimmed := strings.TrimSpace(valueStringOrEmpty(value))
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	out := parsed.String()
	if (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != "" && parsed.Path == "" && parsed.RawPath == "" {
		out += "/"
	}
	return out
}

func valueStringOrEmpty(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

// resolveVpsLabFile espelha resolveVpsLabFile.
func resolveVpsLabFile(protocol string, anonymity string, httpsOnly bool) string {
	if protocol == "socks5" {
		return "socks5_all.txt"
	}
	sslKey := "any"
	if httpsOnly {
		sslKey = "ssl"
	}
	anonymityKey := "any"
	for _, level := range publicProxyAnonymityLevels() {
		if level == anonymity {
			anonymityKey = anonymity
		}
	}
	switch sslKey {
	case "ssl":
		switch anonymityKey {
		case "elite":
			return "http_ssl_elite.txt"
		case "anonymous":
			return "http_ssl_anonymous.txt"
		default:
			return "http_ssl.txt"
		}
	default:
		switch anonymityKey {
		case "elite":
			return "http_elite.txt"
		case "anonymous":
			return "http_anonymous.txt"
		case "transparent":
			return "http_transparent.txt"
		default:
			return "http_all.txt"
		}
	}
}

var (
	vpslabIPOrPortPattern = regexp.MustCompile(`^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3}):(\d{1,5})$`)
	countryListPattern    = regexp.MustCompile(`^[A-Za-z]{2}(,[A-Za-z]{2})*$`)
)

func validIPOctets(octets []string) bool {
	for _, part := range octets {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 || number > 255 {
			return false
		}
	}
	return true
}

// parseVpsLabText: linhas ip:port válidas, dedupe, corte em count.
func parseVpsLabText(text string, protocol string, count int) []map[string]any {
	scheme := "http"
	if protocol == "socks5" {
		scheme = "socks5"
	}
	seen := map[string]bool{}
	entries := []map[string]any{}
	for _, rawLine := range splitLines(text) {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		match := vpslabIPOrPortPattern.FindStringSubmatch(line)
		if match == nil || !validIPOctets(match[1:5]) {
			continue
		}
		proxyURL := normalizeProxyURLKey(scheme + "://" + line)
		if seen[proxyURL] {
			continue
		}
		seen[proxyURL] = true
		entries = append(entries, map[string]any{"name": "VPSLab " + line, "proxyUrl": proxyURL, "type": "http"})
		if len(entries) >= count {
			break
		}
	}
	return entries
}

// buildFreeProxyDbURL: seek freshest página cheia (ranking client-side).
func buildFreeProxyDbURL(count int, protocol, country, anonymity string, httpsOnly bool, maxSpeed float64) string {
	params := url.Values{}
	if protocol == "" {
		protocol = "http"
	}
	params.Set("protocol", protocol)
	if country != "" {
		params.Set("country", country)
	}
	if anonymity != "" {
		params.Set("anonymity", anonymity)
	}
	if httpsOnly {
		params.Set("https", "1")
	}
	if maxSpeed > 0 {
		params.Set("speed", fmt.Sprintf("0,%s", formatFloat(maxSpeed)))
	}
	pageSize := count
	if pageSize < 100 {
		pageSize = 100
	}
	params.Set("page_size", strconv.Itoa(pageSize))
	params.Set("order_by", "last_checked")
	params.Set("order_dir", "desc")
	return freeproxydbSearchURL + "?" + params.Encode()
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// freeproxydbRow ordenável (success/latency/recent).
func rankFreeproxyDB(rows []map[string]any, orderBy string) {
	switch orderBy {
	case "success":
		sort.SliceStable(rows, func(i, j int) bool {
			return pxpipeNum(rows[i]["check_success_count"]) > pxpipeNum(rows[j]["check_success_count"])
		})
	case "latency":
		sort.SliceStable(rows, func(i, j int) bool {
			return latencyKey(rows[i]) < latencyKey(rows[j])
		})
	}
}

func latencyKey(row map[string]any) float64 {
	speed := pxpipeNum(row["speed"])
	if speed > 0 {
		return speed
	}
	return math.MaxFloat64
}

// parseFreeproxyDbPayload: válidos, dedupe, rank, corte e labels.
func parseFreeproxyDbPayload(payload map[string]any, count int, orderBy string) []map[string]any {
	var rows []map[string]any
	if data, ok := payload["data"].(map[string]any); ok {
		if inner, ok := data["data"].([]any); ok {
			for _, item := range inner {
				if row, ok := item.(map[string]any); ok {
					rows = append(rows, row)
				}
			}
		}
	}
	filtered := []map[string]any{}
	seen := map[string]bool{}
	for _, row := range rows {
		// A busca traz inválidos: só proxies atualmente verificados entram.
		isValid, _ := row["is_valid"].(float64)
		if isValid != 1 {
			continue
		}
		connectString := strings.TrimSpace(valueStringOrEmpty(row["connect_string"]))
		if connectString == "" {
			continue
		}
		scheme := connectString
		if index := strings.Index(connectString, "://"); index >= 0 {
			scheme = connectString[:index]
		} else {
			continue
		}
		if scheme != "http" && scheme != "socks5" {
			continue
		}
		proxyURL := normalizeProxyURLKey(connectString)
		if proxyURL == "" || seen[proxyURL] {
			continue
		}
		seen[proxyURL] = true
		filtered = append(filtered, row)
	}
	rankFreeproxyDB(filtered, orderBy)
	if len(filtered) > count {
		filtered = filtered[:count]
	}
	out := []map[string]any{}
	for _, row := range filtered {
		connectString := strings.TrimSpace(valueStringOrEmpty(row["connect_string"]))
		proxyURL := normalizeProxyURLKey(connectString)
		bits := []string{"FreeProxyDB"}
		if country := valueStringOrEmpty(row["country"]); country != "" {
			bits = append(bits, country)
		}
		anonymity := valueStringOrEmpty(row["anonymity"])
		for _, level := range publicProxyAnonymityLevels() {
			if level == anonymity {
				bits = append(bits, anonymity)
				break
			}
		}
		if speed := pxpipeNum(row["speed"]); speed > 0 {
			bits = append(bits, formatFloat(speed)+"s")
		}
		host := ""
		if ip := valueStringOrEmpty(row["ip"]); ip != "" {
			if port := valueStringOrEmpty(row["port"]); port != "" {
				host = ip + ":" + port
			} else {
				host = ip
			}
		}
		if host != "" {
			bits = append(bits, host)
		}
		out = append(out, map[string]any{"name": strings.Join(bits, " "), "proxyUrl": proxyURL, "type": "http"})
	}
	return out
}

// fetchUpstreamREAL com erros idênticos (HTTP status com cut 200 e rate limit).
func fetchUpstreamReal(ctx context.Context, sourceURL, sourceLabel, accept string) (int, string, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, sourcesFetchTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, "GET", sourceURL, nil)
	if err != nil {
		return 0, "", nil, err
	}
	request.Header.Set("Accept", accept)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return 0, "", nil, &publicProxyError{Status: 504, Msg: fmt.Sprintf("%s request timed out after %dms.", sourceLabel, sourcesFetchTimeout.Milliseconds())}
		}
		return 0, "", nil, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	status := response.StatusCode
	if status < 200 || status >= 300 {
		trimmed := strings.TrimSpace(string(body))
		if len(trimmed) > 200 {
			trimmed = trimmed[:200]
		}
		detail := ""
		if trimmed != "" {
			detail = ": " + trimmed
		}
		hint := ""
		if status == 429 {
			hint = fmt.Sprintf(" %s public rate limit reached — retry in a minute or lower the count.", sourceLabel)
		}
		return status, "", nil, &publicProxyError{Status: status, Msg: fmt.Sprintf("%s returned HTTP %d%s.%s", sourceLabel, status, detail, hint)}
	}
	contentType := response.Header.Get("content-type")
	return status, contentType, body, nil
}

type publicProxyError struct {
	Status int
	Msg    string
}

func (e *publicProxyError) Error() string { return e.Msg }

// parseImportRequest espelha a validação por source.
type importOptions struct {
	Protocol  string
	Count     int
	Country   string
	Anonymity string
	HTTPSOnly bool
	MaxSpeed  float64
	OrderBy   string
	Error     string
}

func parseImportRequest(body map[string]any, sourceID string) *importOptions {
	protocols, supportsCountry, supportsAnonymity, _, supportsMaxSpeed, supportsOrderBy, minCount, maxCount, defaultCount := publicProxySourceCaps(sourceID)
	protocol := "http"
	if requested := valueStringOrEmpty(body["protocol"]); requested != "" {
		for _, allowed := range protocols {
			if allowed == requested {
				protocol = requested
			}
		}
	}
	count := defaultCount
	if rawCount := pxpipeNum(body["count"]); rawCount > 0 {
		count = int(rawCount)
		if count < minCount {
			count = minCount
		}
		if count > maxCount {
			count = maxCount
		}
	}
	options := &importOptions{Protocol: protocol, Count: count, OrderBy: "success"}
	if rawCountry := strings.TrimSpace(valueStringOrEmpty(body["country"])); rawCountry != "" {
		country := strings.ToUpper(rawCountry)
		if !supportsCountry || !countryListPattern.MatchString(country) {
			options.Error = fmt.Sprintf("Invalid country filter for %s", publicProxySourceMeta(sourceID)["label"])
			return options
		}
		options.Country = country
	}
	if rawAnonymity := strings.TrimSpace(valueStringOrEmpty(body["anonymity"])); rawAnonymity != "" {
		anonymity := strings.ToLower(rawAnonymity)
		allowed := false
		for _, level := range publicProxyAnonymityLevels() {
			if level == anonymity {
				allowed = true
			}
		}
		if !allowed || !supportsAnonymity {
			options.Error = "Invalid anonymity filter"
			return options
		}
		options.Anonymity = anonymity
	}
	if rawHTTPSOnly, ok := body["httpsOnly"].(bool); ok && rawHTTPSOnly {
		options.HTTPSOnly = true
	}
	rawSpeed := floatAnyOrZero(body["maxSpeed"])
	if rawSpeed > 0 {
		if !supportsMaxSpeed {
			options.Error = fmt.Sprintf("Speed filter is not supported for %s", publicProxySourceMeta(sourceID)["label"])
			return options
		}
		options.MaxSpeed = rawSpeed
	}
	if requested := strings.TrimSpace(valueStringOrEmpty(body["orderBy"])); requested != "" {
		options.OrderBy = requested
	}
	if !supportsOrderBy && options.OrderBy != "success" {
		options.Error = "Ordering is not supported for " + valueStringOrEmpty(publicProxySourceMeta(sourceID)["label"])
		return options
	}
	return options
}

// fetchPublicProxyEntries espelha fetchPublicProxyEntries.
func fetchPublicProxyEntries(ctx context.Context, fetcher publicProxyFetcher, sourceID string, options *importOptions) ([]map[string]any, error) {
	label := ""
	if meta := publicProxySourceMeta(sourceID); meta != nil {
		label, _ = meta["label"].(string)
	}
	switch sourceID {
	case "vpslab":
		sourceURL := vpslabBaseURL + "/" + resolveVpsLabFile(options.Protocol, options.Anonymity, options.HTTPSOnly)
		_, _, body, err := fetcher(ctx, sourceURL, label, "application/json, text/plain, */*")
		if err != nil {
			return nil, err
		}
		return parseVpsLabText(string(body), options.Protocol, options.Count), nil
	case "freeproxydb":
		sourceURL := buildFreeProxyDbURL(options.Count, options.Protocol, options.Country, options.Anonymity, options.HTTPSOnly, options.MaxSpeed)
		status, _, body, err := fetcher(ctx, sourceURL, label, "application/json, text/plain, */*")
		if err != nil {
			return nil, err
		}
		_ = status
		var payload map[string]any
		if json.Unmarshal(body, &payload) == nil {
			return parseFreeproxyDbPayload(payload, options.Count, options.OrderBy), nil
		}
		return nil, &publicProxyError{Status: 502, Msg: "FreeProxyDB returned a non-JSON payload."}
	}
	return []map[string]any{}, nil
}

func floatAnyOrZero(value any) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case int:
		return float64(number)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(number), 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}
