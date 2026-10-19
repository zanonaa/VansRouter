package auth

import (
	"math"
	"sync"
	"time"
)

type attempt struct {
	fails, level int
	until, last  time.Time
}
type Limiter struct {
	mu      sync.Mutex
	entries map[string]attempt
	now     func() time.Time
}

func NewLimiter() *Limiter { return &Limiter{entries: map[string]attempt{}, now: time.Now} }
func (l *Limiter) entry(ip string) attempt {
	e := l.entries[ip]
	if !e.last.IsZero() && l.now().Sub(e.last) > time.Hour && !l.now().Before(e.until) {
		delete(l.entries, ip)
		return attempt{}
	}
	return e
}
func (l *Limiter) Check(ip string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entry(ip)
	remaining := e.until.Sub(l.now())
	if remaining <= 0 {
		return false, 0
	}
	return true, int(math.Ceil(remaining.Seconds()))
}
func (l *Limiter) Fail(ip string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entry(ip)
	e.fails++
	e.last = l.now()
	if e.fails >= 5 {
		steps := []time.Duration{30 * time.Second, 120 * time.Second, 600 * time.Second, 1800 * time.Second}
		level := e.level
		if level >= len(steps) {
			level = len(steps) - 1
		}
		e.until = l.now().Add(steps[level])
		e.level++
		e.fails = 0
	}
	l.entries[ip] = e
	return 5 - e.fails
}
func (l *Limiter) Success(ip string) { l.mu.Lock(); defer l.mu.Unlock(); delete(l.entries, ip) }
