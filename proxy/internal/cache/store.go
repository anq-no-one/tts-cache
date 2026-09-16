package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

type Store interface {
	Get(key string) ([]byte, bool)
	Put(key string, audio []byte, meta Meta) error
	Evictions() int64
	Bytes() int64
}

type DiskStore struct {
	mu  sync.Mutex
	dir string
	mem map[string][]byte

	MaxBytes int64

	sizes      map[string]int64
	lastAccess map[string]time.Time
	totalBytes int64
	evictions  int64
}

var _ Store = (*DiskStore)(nil)

func NewStore(dir string) *DiskStore {
	s := &DiskStore{
		dir:        dir,
		mem:        map[string][]byte{},
		sizes:      map[string]int64{},
		lastAccess: map[string]time.Time{},
	}
	s.rebuildLocked()
	return s
}

func (s *DiskStore) Get(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.mem[key]; ok {
		s.touchLocked(key)
		return b, true
	}
	p := filepath.Join(s.dir, Filename(key))
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	s.mem[key] = b
	if _, ok := s.sizes[key]; !ok {
		s.totalBytes += int64(len(b))
		s.sizes[key] = int64(len(b))
	}
	s.touchLocked(key)
	return b, true
}

func (s *DiskStore) Put(key string, audio []byte, meta Meta) error {
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
	defer s.mu.Unlock()
	if old, ok := s.sizes[key]; ok {
		s.totalBytes -= old
	}
	s.mem[key] = audio
	s.sizes[key] = int64(len(audio))
	s.lastAccess[key] = meta.LastAccess
	s.totalBytes += int64(len(audio))
	s.evictLocked(key)
	return nil
}

func (s *DiskStore) Evictions() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.evictions
}

func (s *DiskStore) Bytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.totalBytes
}

func (s *DiskStore) evictLocked(except string) {
	if s.MaxBytes <= 0 {
		return
	}
	for s.totalBytes > s.MaxBytes {
		victim := ""
		for k, t := range s.lastAccess {
			if k == except {
				continue
			}
			if victim == "" || t.Before(s.lastAccess[victim]) ||
				(t.Equal(s.lastAccess[victim]) && k < victim) {
				victim = k
			}
		}
		if victim == "" {
			return
		}
		s.removeLocked(victim)
		s.evictions++
	}
}

func (s *DiskStore) removeLocked(key string) {
	delete(s.mem, key)
	if sz, ok := s.sizes[key]; ok {
		s.totalBytes -= sz
		delete(s.sizes, key)
	}
	delete(s.lastAccess, key)
	_ = os.Remove(filepath.Join(s.dir, Filename(key)))
	_ = os.Remove(filepath.Join(s.dir, MetaFilename(key)))
}

func (s *DiskStore) rebuildLocked() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".mp3") {
			continue
		}
		key := strings.TrimSuffix(name, ".mp3")
		info, err := e.Info()
		var size int64
		var mtime time.Time
		if err == nil {
			size = info.Size()
			mtime = info.ModTime()
		} else {
			st, err := os.Stat(filepath.Join(s.dir, name))
			if err != nil {
				continue
			}
			size = st.Size()
			mtime = st.ModTime()
		}
		last := mtime
		if raw, err := os.ReadFile(filepath.Join(s.dir, MetaFilename(key))); err == nil {
			var m Meta
			if json.Unmarshal(raw, &m) == nil && !m.LastAccess.IsZero() {
				last = m.LastAccess
			}
		}
		s.sizes[key] = size
		s.lastAccess[key] = last
		s.totalBytes += size
	}
}

func (s *DiskStore) touchLocked(key string) {
	now := time.Now().UTC()
	s.lastAccess[key] = now
	s.bumpHitsLocked(key, now)
}

func (s *DiskStore) bumpHitsLocked(key string, now time.Time) {
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
	m.LastAccess = now
	out, err := json.Marshal(m)
	if err != nil {
		return
	}
	_ = os.WriteFile(p, out, 0o644)
}
