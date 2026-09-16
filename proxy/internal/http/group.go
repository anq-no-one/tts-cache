package http

import "sync"

type flight struct {
	done  chan struct{}
	value []byte
	err   error
}

type flightGroup struct {
	mu      sync.Mutex
	flights map[string]*flight
}

func (g *flightGroup) Coalesce(key string, fn func() ([]byte, error)) ([]byte, error) {
	g.mu.Lock()
	if g.flights == nil {
		g.flights = map[string]*flight{}
	}
	if f, ok := g.flights[key]; ok {
		g.mu.Unlock()
		<-f.done
		return f.value, f.err
	}
	f := &flight{done: make(chan struct{})}
	g.flights[key] = f
	g.mu.Unlock()

	f.value, f.err = fn()
	close(f.done)

	g.mu.Lock()
	delete(g.flights, key)
	g.mu.Unlock()
	return f.value, f.err
}
