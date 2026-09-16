package http

import (
	"sync"
	"time"
)

type rateWindow struct {
	start time.Time
	count int
}

type rateLimiter struct {
	mu      sync.Mutex
	perMin  int
	windows map[string]rateWindow
}

func newRateLimiter(perMin int) *rateLimiter {
	return &rateLimiter{perMin: perMin, windows: map[string]rateWindow{}}
}

func (l *rateLimiter) Allow(key string) bool {
	if l.perMin <= 0 {
		return true
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	w, ok := l.windows[key]
	if !ok || now.Sub(w.start) >= time.Minute {
		l.windows[key] = rateWindow{start: now, count: 1}
		return true
	}
	if w.count >= l.perMin {
		return false
	}
	w.count++
	l.windows[key] = w
	return true
}
