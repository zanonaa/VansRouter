package handlers

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// CircuitOptions porta os parâmetros usados pelo gateway; predicados permitem
// excluir rate limits e erros de lifecycle local sem acoplar transportes.
type CircuitOptions struct {
	FailureThreshold, DegradationThreshold, HalfOpenRequests, MaxBackoffMultiplier, BackoffEscalationCount int
	ResetTimeout, FailureWindow                                                                            time.Duration
	IsFailure                                                                                              func(error) bool
	Classify                                                                                               func(error) string
	KindThresholds                                                                                         map[string]struct {
		Threshold     int
		ImmediateOpen bool
	}
}
type CircuitBreaker struct {
	mu                                     sync.Mutex
	name, state                            string
	failures, successes, probes, remaining int
	lastFailure, openedAt                  int64
	timestamps                             []int64
	transitions                            []map[string]any
	options                                CircuitOptions
}

func newCircuit(name string, options CircuitOptions) *CircuitBreaker {
	if options.FailureThreshold == 0 {
		options.FailureThreshold = 5
	}
	if options.ResetTimeout == 0 {
		options.ResetTimeout = 30 * time.Second
	}
	if options.HalfOpenRequests == 0 {
		options.HalfOpenRequests = 1
	}
	if options.MaxBackoffMultiplier == 0 {
		options.MaxBackoffMultiplier = 16
	}
	if options.BackoffEscalationCount == 0 {
		options.BackoffEscalationCount = 3
	}
	if options.DegradationThreshold == 0 {
		options.DegradationThreshold = int(math.Floor(float64(options.FailureThreshold) * 0.6))
	}
	return &CircuitBreaker{name: name, state: "CLOSED", options: options, transitions: []map[string]any{}}
}
func (b *CircuitBreaker) transition(state string, now int64) {
	b.transitions = append(b.transitions, map[string]any{"from": b.state, "to": state, "at": time.UnixMilli(now).UTC().Format("2006-01-02T15:04:05.000Z")})
	if len(b.transitions) > 20 {
		b.transitions = b.transitions[1:]
	}
	b.state = state
	switch state {
	case "OPEN":
		b.openedAt = now
		b.remaining = 0
	case "HALF_OPEN":
		b.remaining = b.options.HalfOpenRequests
	case "CLOSED":
		b.failures = 0
		b.successes = 0
		b.probes = 0
		b.openedAt = 0
	}
}
func (b *CircuitBreaker) retry(now int64) int64 {
	if b.state != "OPEN" || b.openedAt == 0 {
		return 0
	}
	multiplier := math.Min(math.Pow(2, math.Floor(float64(b.probes)/float64(b.options.BackoffEscalationCount))), float64(b.options.MaxBackoffMultiplier))
	remaining := int64(float64(b.options.ResetTimeout.Milliseconds())*multiplier) - (now - b.openedAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}
func (b *CircuitBreaker) CanExecute() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now().UnixMilli()
	switch b.state {
	case "OPEN":
		if b.retry(now) == 0 {
			b.transition("HALF_OPEN", now)
			return true
		}
		return false
	case "HALF_OPEN":
		if b.remaining > 0 {
			b.remaining--
			return true
		}
		return false
	}
	return true
}
func (b *CircuitBreaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.successes++
	if b.state == "HALF_OPEN" || b.state == "DEGRADED" && b.successes >= b.options.FailureThreshold {
		b.transition("CLOSED", time.Now().UnixMilli())
	}
}
func (b *CircuitBreaker) Failure(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.options.IsFailure != nil && !b.options.IsFailure(err) {
		return
	}
	now := time.Now().UnixMilli()
	b.failures++
	b.lastFailure = now
	if b.options.FailureWindow > 0 {
		b.timestamps = append(b.timestamps, now)
		cut := now - b.options.FailureWindow.Milliseconds()
		i := 0
		for i < len(b.timestamps) && b.timestamps[i] < cut {
			i++
		}
		b.timestamps = b.timestamps[i:]
	}
	if b.state == "HALF_OPEN" {
		b.probes++
		b.transition("OPEN", now)
		return
	}
	if b.state == "OPEN" {
		return
	}
	kind := "transient"
	if b.options.Classify != nil {
		kind = b.options.Classify(err)
	}
	rule := b.options.KindThresholds[kind]
	if rule.ImmediateOpen {
		b.transition("OPEN", now)
		return
	}
	threshold := rule.Threshold
	if threshold == 0 {
		threshold = b.options.FailureThreshold
	}
	count := b.failures
	if b.options.FailureWindow > 0 {
		count = len(b.timestamps)
	}
	if count >= threshold {
		b.transition("OPEN", now)
	} else if count >= b.options.DegradationThreshold && b.state == "CLOSED" {
		b.transition("DEGRADED", now)
	}
}
func (b *CircuitBreaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.transition("CLOSED", time.Now().UnixMilli())
}
func (b *CircuitBreaker) Status() map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	transitions := b.transitions
	if len(transitions) > 5 {
		transitions = transitions[len(transitions)-5:]
	}
	copy := append([]map[string]any{}, transitions...)
	return map[string]any{"name": b.name, "state": b.state, "failureCount": b.failures, "successCount": b.successes, "lastFailureTime": nilOrNil(b.lastFailure), "retryAfterMs": b.retry(time.Now().UnixMilli()), "openedAt": nilOrNil(b.openedAt), "transitions": copy}
}

type CircuitRegistry struct {
	mu    sync.Mutex
	items map[string]*CircuitBreaker
	order []string
}

func (r *CircuitRegistry) Get(name string, options *CircuitOptions) *CircuitBreaker {
	if name == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if b := r.items[name]; b != nil {
		return b
	}
	if options == nil {
		return nil
	}
	if r.items == nil {
		r.items = map[string]*CircuitBreaker{}
	}
	b := newCircuit(name, *options)
	r.items[name] = b
	r.order = append(r.order, name)
	return b
}
func (r *CircuitRegistry) GetHTTP(w http.ResponseWriter, request *http.Request) {
	r.mu.Lock()
	list := make([]*CircuitBreaker, 0, len(r.order))
	for _, name := range r.order {
		list = append(list, r.items[name])
	}
	r.mu.Unlock()
	statuses := []map[string]any{}
	for _, b := range list {
		statuses = append(statuses, b.Status())
	}
	nodeReply(w, 200, map[string]any{"statuses": statuses})
}
func (r *CircuitRegistry) ResetHTTP(w http.ResponseWriter, request *http.Request) {
	name := chi.URLParam(request, "name")
	if name == "" {
		nodeReply(w, 400, map[string]any{"error": "Circuit breaker name is required"})
		return
	}
	decoded, err := url.PathUnescape(name)
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": "Failed to reset circuit breaker"})
		return
	}
	if b := r.Get(decoded, nil); b != nil {
		b.Reset()
	}
	nodeReply(w, 200, map[string]any{"success": true, "message": fmt.Sprintf("Circuit breaker %q reset to CLOSED", name)})
}
