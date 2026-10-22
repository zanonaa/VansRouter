package handlers

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitLifecycleAndReset(t *testing.T) {
	registry := &CircuitRegistry{}
	breaker := registry.Get("provider", &CircuitOptions{FailureThreshold: 2, DegradationThreshold: 1, ResetTimeout: time.Millisecond})
	breaker.Failure(errors.New("upstream"))
	if breaker.Status()["state"] != "DEGRADED" {
		t.Fatal("not degraded")
	}
	breaker.Failure(errors.New("upstream"))
	if breaker.Status()["state"] != "OPEN" {
		t.Fatal("not open")
	}
	breaker.mu.Lock()
	breaker.openedAt = time.Now().Add(-time.Second).UnixMilli()
	breaker.mu.Unlock()
	if !breaker.CanExecute() || breaker.Status()["state"] != "HALF_OPEN" {
		t.Fatal("no recovery probe")
	}
	breaker.Success()
	if breaker.Status()["state"] != "CLOSED" || breaker.Status()["failureCount"] != 0 {
		t.Fatal("not recovered")
	}
	breaker.Failure(errors.New("upstream"))
	breaker.Reset()
	if breaker.Status()["state"] != "CLOSED" {
		t.Fatal("not reset")
	}
	if registry.Get("unknown", nil) != nil {
		t.Fatal("lookup created breaker")
	}
}
