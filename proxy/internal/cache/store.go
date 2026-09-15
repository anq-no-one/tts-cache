package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Meta struct {
	Key        string    `json:"key"`
	Text       string    `json:"text"`
	VoiceID    string    `json:"voice_id"`
	ModelID    string    `json:"model_id"`
	CreatedAt  time.Time `json:"created_at"`
	LastAccess time.Time `json:"last_access"`
	Hits       int64     `json:"hits"`
	Bytes      int64     `json:"bytes"`
}

type Store struct {
	mu  sync.Mutex
	dir string
	mem map[string][]byte
}

func NewStore(dir string) *Store {
	return &Store{dir: dir, mem: map[string][]byte{}}
}

func (s *Store) Get(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.mem[key]; ok {
		return b, true
	}
	p := filepath.Join(s.dir, Filename(key))
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	s.mem[key] = b
	s.bumpHitsLocked(key)
	return b, true
}

func (s *Store) Put(key string, audio []byte, meta Meta) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	meta.Key = key
	meta.Bytes = int64(len(audio))
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now().UTC()
	}
	meta.LastAccess = time.Now().UTC()
	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.dir, Filename(key)), audio, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.dir, MetaFilename(key)), raw, 0o644); err != nil {
		return err
	}
	s.mu.Lock()
	s.mem[key] = audio
	s.mu.Unlock()
	return nil
}

func (s *Store) bumpHitsLocked(key string) {
	p := filepath.Join(s.dir, MetaFilename(key))
	raw, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var m Meta
	if err := json.Unmarshal(raw, &m); err != nil {
		return
	}
	m.Hits++
	m.LastAccess = time.Now().UTC()
	out, err := json.Marshal(m)
	if err != nil {
		return
	}
	_ = os.WriteFile(p, out, 0o644)
}
