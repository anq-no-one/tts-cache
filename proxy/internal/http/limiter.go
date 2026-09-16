package http

import (
	"sync"
	"time"
)

type window struct {
	start time.Time
	count int
}

type rateLimiter struct {
	mu     sync.Mutex
	perMin int
	hits   map[string]window
}

func newRateLimiter(perMin int) *rateLimiter {
	return &rateLimiter{perMin: perMin, hits: map[string]window{}}
}

func (l *rateLimiter) Allow(key string) bool {
	if l.perMin <= 0 {
		return true
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	w, ok := l.hits[key]
	if !ok || now.Sub(w.start) >= time.Minute {
		l.hits[key] = window{start: now, count: 1}
		return true
	}
	if w.count >= l.perMin {
		return false
	}
	w.count++
	l.hits[key] = w
	return true
}
