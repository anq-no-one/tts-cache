package http

import "sync"

type call struct {
	done  chan struct{}
	value []byte
	err   error
}

type callGroup struct {
	mu    sync.Mutex
	calls map[string]*call
}

func (g *callGroup) Do(key string, fn func() ([]byte, error)) ([]byte, error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = map[string]*call{}
	}
	if c, ok := g.calls[key]; ok {
		g.mu.Unlock()
		<-c.done
		return c.value, c.err
	}
	c := &call{done: make(chan struct{})}
	g.calls[key] = c
	g.mu.Unlock()

	c.value, c.err = fn()
	close(c.done)

	g.mu.Lock()
	delete(g.calls, key)
	g.mu.Unlock()
	return c.value, c.err
}
