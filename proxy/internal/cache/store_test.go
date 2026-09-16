package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func putForTest(t *testing.T, s *DiskStore, key string, size int) {
	t.Helper()
	audio := make([]byte, size)
	for i := range audio {
		audio[i] = byte(len(key) + i)
	}
	if err := s.Put(key, audio, Meta{Text: key}); err != nil {
		t.Fatalf("put %s: %v", key, err)
	}
}

func TestEvictsLeastRecentlyUsedFirst(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	s.MaxBytes = 20

	putForTest(t, s, "a", 10)
	putForTest(t, s, "b", 10)
	if _, ok := s.Get("a"); !ok {
		t.Fatal("a must be present")
	}
	putForTest(t, s, "c", 10)

	if _, ok := s.Get("b"); ok {
		t.Fatal("b must have been evicted as least-recently-used")
	}
	if _, ok := s.Get("a"); !ok {
		t.Fatal("a must survive after recent Get")
	}
	if _, ok := s.Get("c"); !ok {
		t.Fatal("c must be present")
	}
	if got := s.Evictions(); got != 1 {
		t.Fatalf("expected 1 eviction, got %d", got)
	}
}

func TestCapTotalAccounting(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	s.MaxBytes = 100

	putForTest(t, s, "a", 10)
	putForTest(t, s, "b", 15)
	if got := s.Bytes(); got != 25 {
		t.Fatalf("expected 25 bytes, got %d", got)
	}
	putForTest(t, s, "a", 12)
	if got := s.Bytes(); got != 27 {
		t.Fatalf("expected 27 bytes after overwrite, got %d", got)
	}
}

func TestReloadRebuildsIndex(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	putForTest(t, s, "a", 10)
	putForTest(t, s, "b", 10)
	want := s.Bytes()

	r := NewStore(dir)
	if got := r.Bytes(); got != want {
		t.Fatalf("rebuilt bytes = %d, want %d", got, want)
	}
	r.MaxBytes = 20
	putForTest(t, r, "c", 10)
	if _, ok := r.Get("a"); ok {
		t.Fatal("a must have been evicted after rebuild")
	}
	if got := r.Evictions(); got != 1 {
		t.Fatalf("expected 1 eviction after reload, got %d", got)
	}
}

func TestOversizeSingleEntryStillStored(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	s.MaxBytes = 10

	putForTest(t, s, "small", 5)
	putForTest(t, s, "big", 100)

	got, ok := s.Get("big")
	if !ok || len(got) != 100 {
		t.Fatalf("oversize entry must still be stored, ok=%v len=%d", ok, len(got))
	}
	if _, ok := s.Get("small"); ok {
		t.Fatal("small entry must have been evicted to fit oversize entry")
	}
}

func TestEvictionDeletesDiskFiles(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	s.MaxBytes = 10

	putForTest(t, s, "a", 10)
	putForTest(t, s, "b", 10)

	for _, name := range []string{Filename("a"), MetaFilename("a")} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("evicted file %s must be deleted", name)
		}
	}
	for _, name := range []string{Filename("b"), MetaFilename("b")} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("surviving file %s must exist: %v", name, err)
		}
	}
}

func TestUnlimitedKeepsCurrentBehavior(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	putForTest(t, s, "a", 10)
	putForTest(t, s, "b", 10)
	if got := s.Evictions(); got != 0 {
		t.Fatalf("unlimited store must not evict, got %d", got)
	}
	if _, ok := s.Get("a"); !ok {
		t.Fatal("a must be present")
	}
	if _, ok := s.Get("b"); !ok {
		t.Fatal("b must be present")
	}
}
