package handlers

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

// resolvedProxy é o resultado da resolução de proxy de uma conexão.
// Segue resolveConnectionProxyConfig do original: pool tem prioridade sobre o
// proxy legado da conexão e o pool escolhido define strictProxy.
type resolvedProxy struct {
	proxyURL string
	noProxy  string
	relay    string
	strict   bool
	intended bool
	poolID   string
}

type proxyPoolCursorState struct {
	mu    sync.Mutex
	index map[string]int
}

var proxyPoolCursors = proxyPoolCursorState{index: map[string]int{}}

type proxyFitnessState struct {
	mu     sync.RWMutex
	lookup func(context.Context, string) ([]database.Fitness, error)
	mark   func(context.Context, string, string, float64, string) error
}

var proxyFitness proxyFitnessState

// SetProxyFitnessLookup liga a consulta de fitness usada pela estratégia smart.
// É opcional: sem lookup a seleção smart se comporta como round-robin.
func SetProxyFitnessLookup(lookup func(context.Context, string) ([]database.Fitness, error)) {
	proxyFitness.mu.Lock()
	proxyFitness.lookup = lookup
	proxyFitness.mu.Unlock()
}

// SetProxyFitnessMarker liga a persistência de fitness (markPoolUnfit da
// origem). O executor opencode free usa para marcar ip-limit e girar o pool.
func SetProxyFitnessMarker(mark func(context.Context, string, string, float64, string) error) {
	proxyFitness.mu.Lock()
	proxyFitness.mark = mark
	proxyFitness.mu.Unlock()
}

// markPoolUnfit persiste a marcação de indisponibilidade do pool para o
// escopo (provider::modelo) até o timestamp; nil usa o default de 5 min.
// Sem marker ligado é no-op (como a origem sem banco configurado).
func markPoolUnfit(ctx context.Context, poolID, scope string, until *int64, reason string) {
	if poolID == "" || scope == "" {
		return
	}
	proxyFitness.mu.RLock()
	mark := proxyFitness.mark
	proxyFitness.mu.RUnlock()
	if mark == nil {
		return
	}
	target := float64(time.Now().Add(5 * time.Minute).UnixMilli())
	if until != nil {
		target = float64(*until)
	}
	_ = mark(ctx, poolID, scope, target, reason)
}

func fitnessLookup() func(context.Context, string) ([]database.Fitness, error) {
	proxyFitness.mu.RLock()
	defer proxyFitness.mu.RUnlock()
	return proxyFitness.lookup
}

func stringSlice(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		if values, ok := value.([]string); ok {
			return values
		}
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok {
			if text = strings.TrimSpace(text); text != "" {
				result = append(result, text)
			}
		}
	}
	return result
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func filterTargetPools(ids, targets []string) []string {
	if len(targets) == 0 {
		return ids
	}
	allowed := map[string]bool{}
	for _, target := range targets {
		allowed[target] = true
	}
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if allowed[id] {
			result = append(result, id)
		}
	}
	return result
}

// poolIsFit reproduz isPoolFit: qualquer entrada ativa para o escopo ou para o
// curinga do provider marca o pool como indisponível.
func poolIsFit(ctx context.Context, lookup func(context.Context, string) ([]database.Fitness, error), poolID, scope string, now float64) bool {
	if poolID == "" || scope == "" || lookup == nil {
		return true
	}
	entries, err := lookup(ctx, poolID)
	if err != nil {
		return true // fail-open como no original
	}
	wildcard := ""
	if separator := strings.Index(scope, "::"); separator >= 0 {
		wildcard = scope[:separator] + "::*"
	}
	for _, entry := range entries {
		if entry.Scope != scope && (wildcard == "" || entry.Scope != wildcard) {
			continue
		}
		if entry.Until > now {
			return false
		}
	}
	return true
}

func selectProxyPoolID(ctx context.Context, data map[string]any, lookup connectionPoolLookup) (string, map[string]any) {
	single := strings.TrimSpace(ttsString(data["proxyPoolId"]))
	if single == "__none__" {
		single = ""
	}
	ids := dedupeStrings(stringSlice(data["proxyPoolIds"]))
	strategy := strings.ToLower(strings.TrimSpace(ttsString(data["proxyRotationStrategy"])))
	if strategy == "" {
		strategy = "none"
	}
	if len(ids) == 0 {
		if single == "" || lookup == nil {
			return "", nil
		}
		pool, err := lookup(ctx, single)
		if err != nil {
			return "", nil
		}
		return single, pool
	}

	eligible := filterTargetPools(ids, dedupeStrings(stringSlice(data["targetProxyPoolIds"])))
	// excludePoolIds é setado pelo retry-loop do executor (ip-limit): pools
	// já marcados como unfit nesta request não são reconsiderados.
	if excluded := stringSlice(data["excludeProxyPoolIds"]); len(excluded) > 0 {
		skip := map[string]bool{}
		for _, id := range excluded {
			skip[id] = true
		}
		kept := make([]string, 0, len(eligible))
		for _, id := range eligible {
			if !skip[id] {
				kept = append(kept, id)
			}
		}
		eligible = kept
	}
	// Sem pool-set, o pool único também pode ser excluído pelo retry-loop.
	if single != "" && len(eligible) == 0 && len(ids) == 0 {
		if excluded := stringSlice(data["excludeProxyPoolIds"]); len(excluded) > 0 {
			for _, id := range excluded {
				if id == single {
					return "", nil
				}
			}
		}
	}
	pools := map[string]map[string]any{}
	active := make([]string, 0, len(eligible))
	for _, id := range eligible {
		if lookup == nil {
			break
		}
		pool, err := lookup(ctx, id)
		if err != nil || pool == nil {
			continue
		}
		pools[id] = pool
		if pool["isActive"] == true && strings.TrimSpace(ttsString(pool["proxyUrl"])) != "" {
			active = append(active, id)
		}
	}
	if len(active) == 0 {
		return "", nil
	}

	// Smart filtra pools marcados como indisponíveis para o escopo
	// provider::modelo. Sem escopo/lookup mantém a ordem original.
	scope := strings.TrimSpace(ttsString(data["proxyPoolScope"]))
	fit := active
	if strategy == "smart" && scope != "" {
		now := float64(time.Now().UnixMilli())
		fit = fit[:0:0]
		for _, id := range active {
			if poolIsFit(ctx, fitnessLookup(), id, scope, now) {
				fit = append(fit, id)
			}
		}
	}
	if len(fit) == 0 {
		return "", nil
	}

	switch strategy {
	case "fill-first":
		return fit[0], pools[fit[0]]
	case "round-robin", "smart":
		connectionID := strings.TrimSpace(ttsString(data["id"]))
		key := connectionID + ":" + strategy + ":" + strings.Join(fit, ",")
		proxyPoolCursors.mu.Lock()
		index := proxyPoolCursors.index[key] % len(fit)
		proxyPoolCursors.index[key] = (index + 1) % len(fit)
		proxyPoolCursors.mu.Unlock()
		return fit[index], pools[fit[index]]
	case "random":
		id := fit[rand.Intn(len(fit))]
		return id, pools[id]
	default:
		return fit[0], pools[fit[0]]
	}
}

func resolveConnectionProxy(ctx context.Context, data map[string]any, lookup connectionPoolLookup) resolvedProxy {
	result := resolvedProxy{}
	if data == nil {
		return result
	}
	if poolID, pool := selectProxyPoolID(ctx, data, lookup); poolID != "" {
		result.poolID = poolID
		result.intended = true
		if pool != nil {
			result.strict = pool["strictProxy"] == true
			if pool["isActive"] == true {
				raw := strings.TrimSpace(ttsString(pool["proxyUrl"]))
				if raw != "" {
					result.noProxy = ttsString(pool["noProxy"])
					switch pool["type"] {
					case "vercel", "cloudflare", "deno":
						result.relay = raw
					default:
						result.proxyURL = raw
					}
					return result
				}
			}
		}
	}

	enabled := data["connectionProxyEnabled"] == true
	legacyURL := strings.TrimSpace(ttsString(data["connectionProxyUrl"]))
	if enabled {
		result.intended = true
	}
	if enabled && legacyURL != "" {
		result.proxyURL = legacyURL
		result.noProxy = strings.TrimSpace(ttsString(data["connectionNoProxy"]))
		return result
	}
	if scope := strings.TrimSpace(ttsString(data["proxyPoolScope"])); strings.HasPrefix(scope, "freebuff::") {
		// Sem pool elegível, o executor freebuff exige falha em vez de sair
		// pelo IP direto. Sem proxy configurado, strict sozinho não bloqueia
		// chamadas comuns (mesma condição proxyIntended da origem).
		result.strict = true
	}
	return result
}

func proxyIntended(data map[string]any) bool {
	if data == nil {
		return false
	}
	if strings.TrimSpace(ttsString(data["proxyPoolId"])) != "" && ttsString(data["proxyPoolId"]) != "__none__" {
		return true
	}
	if len(dedupeStrings(stringSlice(data["proxyPoolIds"]))) > 0 {
		return true
	}
	if data["connectionProxyEnabled"] == true {
		return true
	}
	if strings.TrimSpace(ttsString(data["connectionProxyUrl"])) != "" {
		return true
	}
	return false
}

// ---------- transporte direto com proxy de ambiente dinâmico ----------

func normalizeEnvProxy(raw string) *url.URL {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return nil
	}
	return parsed
}

func envValue(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

// dynamicProxyFromEnvironment lê as variáveis a cada requisição. O
// ProxyFromEnvironment do net/http memoriza o ambiente na primeira chamada;
// com ele, alterar as Settings de proxy global exigiria reiniciar o processo.
func dynamicProxyFromEnvironment(request *http.Request) (*url.URL, error) {
	if request == nil || request.URL == nil {
		return nil, nil
	}
	noProxy := envValue("NO_PROXY", "no_proxy")
	if connectionNoProxyMatches(request.URL.Hostname(), noProxy) {
		return nil, nil
	}
	var raw string
	if strings.EqualFold(request.URL.Scheme, "https") {
		raw = envValue("HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy")
	} else {
		raw = envValue("HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy")
	}
	return normalizeEnvProxy(raw), nil
}

var (
	directTransportOnce sync.Once
	directTransport     *http.Transport
)

func sharedDirectTransport() *http.Transport {
	directTransportOnce.Do(func() {
		base, ok := directTransportBase().(*http.Transport)
		if ok {
			directTransport = base.Clone()
		} else {
			directTransport = &http.Transport{}
		}
		directTransport.Proxy = dynamicProxyFromEnvironment
	})
	return directTransport
}

var (
	proxyTransportMu    sync.Mutex
	proxyTransportCache = map[string]*http.Transport{}
)

func cachedProxyTransport(proxyURL, noProxy string) (*http.Transport, *url.URL, error) {
	parsed, err := url.Parse(proxyURL)
	if err != nil || parsed.Host == "" {
		return nil, nil, err
	}
	key := proxyURL + "\x00" + noProxy
	proxyTransportMu.Lock()
	defer proxyTransportMu.Unlock()
	if transport, ok := proxyTransportCache[key]; ok {
		return transport, parsed, nil
	}
	transport := sharedDirectTransport().Clone()
	transport.Proxy = func(request *http.Request) (*url.URL, error) {
		if connectionNoProxyMatches(request.URL.Hostname(), noProxy) {
			return nil, nil
		}
		return parsed, nil
	}
	proxyTransportCache[key] = transport
	return transport, parsed, nil
}

// proxyFallbackTransport reproduz o fallback da origem: um erro de transporte
// no proxy cai para a conexão direta, exceto em strictProxy.
type proxyFallbackTransport struct {
	proxy  http.RoundTripper
	direct http.RoundTripper
}

func (t proxyFallbackTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.proxy.RoundTrip(request)
	if err == nil {
		return response, nil
	}
	retry := request.Clone(request.Context())
	if request.Body != nil && request.Body != http.NoBody {
		if request.GetBody == nil {
			return nil, err
		}
		body, bodyErr := request.GetBody()
		if bodyErr != nil {
			return nil, err
		}
		retry.Body = body
	}
	return t.direct.RoundTrip(retry)
}

// relayTransport usa o transporte direto sem proxy de ambiente, como o
// originalFetch no caminho de relay da origem.
func relayTransport() *http.Transport {
	transport := sharedDirectTransport().Clone()
	transport.Proxy = nil
	return transport
}

var (
	relayTransportOnce sync.Once
	sharedRelay        *http.Transport
)

func sharedRelayTransport() *http.Transport {
	relayTransportOnce.Do(func() {
		sharedRelay = relayTransport()
	})
	return sharedRelay
}

var errProxyRequired = errors.New("proxy required but none resolved (strictProxy=true)")

// dialCheck garante que o endereço de proxy tenha host válido antes de usar.
func dialCheck(proxyURL string) error {
	parsed, err := url.Parse(proxyURL)
	if err != nil || parsed.Host == "" {
		return errors.New("invalid proxy URL")
	}
	if _, _, err = net.SplitHostPort(parsed.Host); err != nil {
		return errors.New("invalid proxy URL")
	}
	return nil
}
