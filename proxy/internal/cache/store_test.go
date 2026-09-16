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

func TestOversizeEntryNotStored(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	s.MaxBytes = 10

	putForTest(t, s, "small", 5)
	putForTest(t, s, "big", 100)

	if _, ok := s.Get("big"); ok {
		t.Fatal("oversize entry must not be cached")
	}
	if got := s.Bytes(); got > s.MaxBytes {
		t.Fatalf("store over cap after oversize put: %d bytes", got)
	}
	for _, name := range []string{Filename("big"), MetaFilename("big")} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("oversize file %s must be absent from disk", name)
		}
	}
	if _, ok := s.Get("small"); !ok {
		t.Fatal("small entry must survive the rejected oversize put")
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

func TestExternalFileCountedOnFirstRead(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	dropped := make([]byte, 25)
	if err := os.WriteFile(filepath.Join(dir, Filename("ext")), dropped, 0o644); err != nil {
		t.Fatalf("drop file: %v", err)
	}
	got, ok := s.Get("ext")
	if !ok || len(got) != len(dropped) {
		t.Fatalf("external file must be served, ok=%v len=%d", ok, len(got))
	}
	if s.Bytes() != int64(len(dropped)) {
		t.Fatalf("bytes must account the external file, got %d", s.Bytes())
	}
	if _, ok := s.Get("missing"); ok {
		t.Fatal("missing key must miss")
	}
}

func TestDiskHitWithoutMetaStillServes(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	putForTest(t, s, "nometa", 12)
	if err := os.Remove(filepath.Join(dir, MetaFilename("nometa"))); err != nil {
		t.Fatalf("remove meta: %v", err)
	}
	fresh := NewStore(dir)
	if _, ok := fresh.Get("nometa"); !ok {
		t.Fatal("disk hit without meta must still serve")
	}
}

func TestCorruptMetaFallsBackToMtime(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	putForTest(t, s, "corrupt", 12)
	if err := os.WriteFile(filepath.Join(dir, MetaFilename("corrupt")), []byte("not-json"), 0o644); err != nil {
		t.Fatalf("corrupt meta: %v", err)
	}
	fresh := NewStore(dir)
	got, ok := fresh.Get("corrupt")
	if !ok || len(got) != 12 {
		t.Fatalf("corrupt meta must fall back to serving the file, ok=%v", ok)
	}
	if fresh.Bytes() != 12 {
		t.Fatalf("bytes must count the file, got %d", fresh.Bytes())
	}
}
