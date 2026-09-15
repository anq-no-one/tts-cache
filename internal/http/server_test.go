package http_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anq-no-one/tts-cache-proxy/internal/cache"
	proxyhttp "github.com/anq-no-one/tts-cache-proxy/internal/http"
	"github.com/anq-no-one/tts-cache-proxy/internal/upstream"
)

type fakeSynth struct {
	calls int
}

func (f *fakeSynth) Synthesize(r upstream.Request) ([]byte, error) {
	f.calls++
	return []byte("AUDIO:" + r.Text), nil
}

func post(t *testing.T, srv *proxyhttp.Server, text string) []byte {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"text": text, "voice_id": "v1"})
	req := httptest.NewRequest(http.MethodPost, "/v1/synthesize", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	out, _ := io.ReadAll(rec.Result().Body)
	return out
}

func TestSecondIdenticalRequestHitsCache(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	dir := t.TempDir()
	fake := &fakeSynth{}
	srv := proxyhttp.NewServer(proxyhttp.Config{
		CacheDir: dir, DefaultModel: "m", DefaultFormat: "mp3_44100_128", DefaultLang: "en",
	}, cache.NewStore(dir), fake)

	first := post(t, srv, "Rest for 30 seconds. Next up is push ups.")
	second := post(t, srv, "Rest for 30 seconds. Next up is push ups.")
	if !bytes.Equal(first, second) {
		t.Fatal("cached response must equal first response")
	}
	if fake.calls != 2 {
		t.Fatalf("expected 2 upstream calls for 2 sentences, got %d", fake.calls)
	}
	third := post(t, srv, "Rest for 30 seconds.")
	if fake.calls != 2 {
		t.Fatalf("repeated sentence must not call upstream again, calls=%d", fake.calls)
	}
	_ = third
}

func TestSplitSentences(t *testing.T) {
	got := proxyhttp.SplitSentences("Hello. How are you? Fine")
	if len(got) != 3 {
		t.Fatalf("expected 3 sentences, got %v", got)
	}
}
