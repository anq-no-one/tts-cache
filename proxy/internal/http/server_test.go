package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anq-no-one/tts-cache/proxy/internal/auth"
	"github.com/anq-no-one/tts-cache/proxy/internal/cache"
	proxyhttp "github.com/anq-no-one/tts-cache/proxy/internal/http"
	"github.com/anq-no-one/tts-cache/proxy/internal/upstream"
)

type fakeSynth struct {
	calls   atomic.Int64
	failOn  map[string]bool
	delayOn map[string]time.Duration

	mu      sync.Mutex
	lastReq upstream.Request
}

func (f *fakeSynth) Synthesize(ctx context.Context, r upstream.Request) ([]byte, error) {
	f.calls.Add(1)
	f.mu.Lock()
	f.lastReq = r
	f.mu.Unlock()
	if d, ok := f.delayOn[r.Text]; ok {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(d):
		}
	}
	if f.failOn[r.Text] {
		return nil, errors.New("boom")
	}
	return []byte("AUDIO:" + r.Text), nil
}

func testConfig(dir string) proxyhttp.Config {
	return proxyhttp.Config{
		CacheDir: dir, DefaultModel: "m", DefaultFormat: "mp3_44100_128", DefaultLang: "en",
		AdminToken: "admin-1",
	}
}

func newTestServer(t *testing.T, cfg proxyhttp.Config, fake *fakeSynth) (*proxyhttp.Server, string) {
	t.Helper()
	tokens := auth.NewStore(filepath.Join(cfg.CacheDir, "tokens.json"), []string{"invite-1"})
	srv := proxyhttp.NewServer(cfg, cache.NewStore(cfg.CacheDir), fake, tokens)
	_, raw, err := tokens.Issue("invite-1", "test-app")
	if err != nil {
		t.Fatalf("issue test token: %v", err)
	}
	return srv, raw
}

func post(t *testing.T, srv *proxyhttp.Server, token, text string) []byte {
	t.Helper()
	code, _, out := postRaw(t, srv, token, text)
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, out)
	}
	return out
}

func TestSecondIdenticalRequestHitsCache(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	dir := t.TempDir()
	fake := &fakeSynth{}
	srv, token := newTestServer(t, testConfig(dir), fake)

	first := post(t, srv, token, "Rest for 30 seconds. Next up is push ups.")
	second := post(t, srv, token, "Rest for 30 seconds. Next up is push ups.")
	if !bytes.Equal(first, second) {
		t.Fatal("cached response must equal first response")
	}
	if fake.calls.Load() != 2 {
		t.Fatalf("expected 2 upstream calls for 2 sentences, got %d", fake.calls.Load())
	}
	third := post(t, srv, token, "Rest for 30 seconds.")
	if fake.calls.Load() != 2 {
		t.Fatalf("repeated sentence must not call upstream again, calls=%d", fake.calls.Load())
	}
	_ = third
}

func postRaw(t *testing.T, srv *proxyhttp.Server, token, text string) (int, http.Header, []byte) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"text": text, "voice_id": "v1"})
	req := httptest.NewRequest(http.MethodPost, "/v1/synthesize", bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	out, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, rec.Result().Header, out
}

type statusRow struct {
	Index  int    `json:"index"`
	Key    string `json:"key"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

func statusesOf(t *testing.T, h http.Header) []statusRow {
	t.Helper()
	var rows []statusRow
	if err := json.Unmarshal([]byte(h.Get("X-Sentence-Statuses")), &rows); err != nil {
		t.Fatalf("bad statuses header: %v", err)
	}
	return rows
}

func TestOneFailedSentenceReturnsPartialContent(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeSynth{failOn: map[string]bool{"Next up is push ups.": true}}
	srv, token := newTestServer(t, testConfig(dir), fake)

	code, header, out := postRaw(t, srv, token, "Rest for 30 seconds. Next up is push ups.")
	if code != http.StatusPartialContent {
		t.Fatalf("expected 206, got %d: %s", code, out)
	}
	if !bytes.Contains(out, []byte("AUDIO:Rest for 30 seconds.")) {
		t.Fatalf("available sentence must be in the body, got %q", out)
	}
	if bytes.Contains(out, []byte("push ups")) {
		t.Fatalf("failed sentence must not be in the body, got %q", out)
	}
	rows := statusesOf(t, header)
	if len(rows) != 2 || rows[0].Status != "synthesized" || rows[1].Status != "error" {
		t.Fatalf("unexpected statuses: %+v", rows)
	}
	if rows[1].Error == "" {
		t.Fatal("failed sentence must carry the error text")
	}
	if header.Get("X-Sentence-Count") != "2" {
		t.Fatalf("expected sentence count 2, got %q", header.Get("X-Sentence-Count"))
	}
}

func TestAllFailedSentencesKeepBadGateway(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeSynth{failOn: map[string]bool{"Only sentence.": true}}
	srv, token := newTestServer(t, testConfig(dir), fake)

	code, _, _ := postRaw(t, srv, token, "Only sentence.")
	if code != http.StatusBadGateway {
		t.Fatalf("expected 502 when nothing is available, got %d", code)
	}
}

func TestAllFailedKeepsStatusHeaders(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeSynth{failOn: map[string]bool{"Only sentence.": true}}
	cfg := testConfig(dir)
	cfg.Region = "eu-west"
	srv, token := newTestServer(t, cfg, fake)

	code, header, _ := postRaw(t, srv, token, "Only sentence.")
	if code != http.StatusBadGateway {
		t.Fatalf("expected 502 when nothing is available, got %d", code)
	}
	rows := statusesOf(t, header)
	if len(rows) != 1 || rows[0].Status != "error" || rows[0].Error == "" {
		t.Fatalf("502 must keep per-sentence statuses, got %+v", rows)
	}
	if got := header.Get("X-Region"); got != "eu-west" {
		t.Fatalf("502 must keep X-Region, got %q", got)
	}
}

func TestSlowSentenceDoesNotBlockOthers(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeSynth{delayOn: map[string]time.Duration{"Slow one.": 300 * time.Millisecond}}
	srv, token := newTestServer(t, testConfig(dir), fake)

	start := time.Now()
	code, _, out := postRaw(t, srv, token, "Slow one. Fast one.")
	elapsed := time.Since(start)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if elapsed >= 600*time.Millisecond {
		t.Fatalf("sentences look sequential, took %v", elapsed)
	}
	first := bytes.Index(out, []byte("AUDIO:Slow one."))
	second := bytes.Index(out, []byte("AUDIO:Fast one."))
	if first == -1 || second == -1 || first > second {
		t.Fatalf("audio must stay in sentence order, got %q", out)
	}
}

func TestCustomSoundParamsReachUpstream(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeSynth{}
	srv, token := newTestServer(t, testConfig(dir), fake)

	body, _ := json.Marshal(map[string]any{
		"text": "Hi.", "voice_id": "v1",
		"model_id": "custom-m", "format": "custom-f", "language": "fr", "speed": 1.5,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/synthesize", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.lastReq.ModelID != "custom-m" || fake.lastReq.Format != "custom-f" || fake.lastReq.Speed != 1.5 {
		t.Fatalf("custom params must reach upstream: %+v", fake.lastReq)
	}
}

func TestFreshMetricsReportZeroHitRate(t *testing.T) {
	dir := t.TempDir()
	srv, _ := newTestServer(t, testConfig(dir), &fakeSynth{})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad metrics: %v", err)
	}
	if out["hit_rate"] != float64(0) {
		t.Fatalf("fresh hit rate must be 0, got %v", out["hit_rate"])
	}
}

func TestSplitSentences(t *testing.T) {
	got := proxyhttp.SplitSentences("Hello. How are you? Fine")
	if len(got) != 3 {
		t.Fatalf("expected 3 sentences, got %v", got)
	}
}

func TestSynthesizeRequiresBearerToken(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.Region = "eu-west"
	srv, token := newTestServer(t, cfg, &fakeSynth{})

	if code, header, _ := postRaw(t, srv, "", "Hello."); code != http.StatusUnauthorized {
		t.Fatalf("missing token must be 401, got %d", code)
	} else if got := header.Get("X-Region"); got != "eu-west" {
		t.Fatalf("401 must keep X-Region, got %q", got)
	}
	if code, _, _ := postRaw(t, srv, "wrong", "Hello."); code != http.StatusUnauthorized {
		t.Fatalf("bad token must be 401, got %d", code)
	}
	if code, _, _ := postRaw(t, srv, token, "Hello."); code != http.StatusOK {
		t.Fatalf("valid token must be 200, got %d", code)
	}
}

func registerOverHTTP(t *testing.T, srv *proxyhttp.Server, invite, app string) (int, string, string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"invitation_code": invite, "app_name": app})
	req := httptest.NewRequest(http.MethodPost, "/v1/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		return rec.Code, "", ""
	}
	var out struct {
		TokenID  string `json:"token_id"`
		AppToken string `json:"app_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad register response: %v", err)
	}
	return rec.Code, out.TokenID, out.AppToken
}

func TestRegisterNeedsInvitation(t *testing.T) {
	dir := t.TempDir()
	srv, _ := newTestServer(t, testConfig(dir), &fakeSynth{})

	if code, _, _ := registerOverHTTP(t, srv, "wrong-code", "app-b"); code != http.StatusForbidden {
		t.Fatalf("bad invite must be 403, got %d", code)
	}
	code, _, raw := registerOverHTTP(t, srv, "invite-1", "app-b")
	if code != http.StatusCreated || raw == "" {
		t.Fatalf("good invite must issue a token, got %d", code)
	}
	if st, _, _ := postRaw(t, srv, raw, "Hello."); st != http.StatusOK {
		t.Fatalf("issued token must work, got %d", st)
	}
}

func TestRevokedTokenStopsWorking(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	srv, _ := newTestServer(t, cfg, &fakeSynth{})

	_, id, raw := registerOverHTTP(t, srv, "invite-1", "app-c")

	revoke := func(admin string) int {
		req := httptest.NewRequest(http.MethodDelete, "/v1/tokens/"+id, nil)
		if admin != "" {
			req.Header.Set("Authorization", "Bearer "+admin)
		}
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		return rec.Code
	}
	if code := revoke("wrong-admin"); code != http.StatusForbidden {
		t.Fatalf("wrong admin must be 403, got %d", code)
	}
	if code := revoke("admin-1"); code != http.StatusNoContent {
		t.Fatalf("admin revoke must be 204, got %d", code)
	}
	if code, _, _ := postRaw(t, srv, raw, "Hello."); code != http.StatusUnauthorized {
		t.Fatalf("revoked token must be 401, got %d", code)
	}
}

func TestRegisterRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	srv, _ := newTestServer(t, testConfig(dir), &fakeSynth{})

	for _, body := range []string{"not-json", `{"invitation_code":"invite-1"}`, `{"app_name":""}`} {
		req := httptest.NewRequest(http.MethodPost, "/v1/register", bytes.NewReader([]byte(body)))
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q must be 400, got %d", body, rec.Code)
		}
	}
}

func TestRevokeUnknownTokenIsNotFound(t *testing.T) {
	dir := t.TempDir()
	srv, _ := newTestServer(t, testConfig(dir), &fakeSynth{})

	req := httptest.NewRequest(http.MethodDelete, "/v1/tokens/nope", nil)
	req.Header.Set("Authorization", "Bearer admin-1")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown token must be 404, got %d", rec.Code)
	}
}

func TestOversizeTextRejected(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.MaxTextChars = 10
	srv, token := newTestServer(t, cfg, &fakeSynth{})

	if code, _, _ := postRaw(t, srv, token, "This text is way too long."); code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize text must be 413, got %d", code)
	}
}

func TestRateLimitExceeded(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.RatePerMin = 2
	srv, token := newTestServer(t, cfg, &fakeSynth{})

	for i := 0; i < 2; i++ {
		if code, _, _ := postRaw(t, srv, token, "Same text."); code != http.StatusOK {
			t.Fatalf("request %d must pass, got %d", i, code)
		}
	}
	if code, _, _ := postRaw(t, srv, token, "Same text."); code != http.StatusTooManyRequests {
		t.Fatalf("third request must be 429, got %d", code)
	}
}

func TestConcurrentIdenticalSentencesCallUpstreamOnce(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeSynth{delayOn: map[string]time.Duration{"Shared sentence.": 200 * time.Millisecond}}
	srv, token := newTestServer(t, testConfig(dir), fake)

	const callers = 10
	start := make(chan struct{})
	var wg sync.WaitGroup
	codes := make([]int, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			code, _, _ := postRaw(t, srv, token, "Shared sentence.")
			codes[n] = code
		}(i)
	}
	close(start)
	wg.Wait()
	for i, code := range codes {
		if code != http.StatusOK {
			t.Fatalf("caller %d got %d", i, code)
		}
	}
	if fake.calls.Load() != 1 {
		t.Fatalf("expected 1 upstream call, got %d", fake.calls.Load())
	}
}

type stubStore struct {
	data      map[string][]byte
	evictions int64
}

func (s *stubStore) Get(key string) ([]byte, bool) {
	b, ok := s.data[key]
	return b, ok
}

func (s *stubStore) Put(key string, audio []byte, meta cache.Meta) error {
	if s.data == nil {
		s.data = map[string][]byte{}
	}
	s.data[key] = audio
	return nil
}

func (s *stubStore) Evictions() int64 { return s.evictions }

func (s *stubStore) Bytes() int64 {
	var n int64
	for _, b := range s.data {
		n += int64(len(b))
	}
	return n
}

func TestServerWorksWithAnyStoreBackend(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	tokens := auth.NewStore(filepath.Join(cfg.CacheDir, "tokens.json"), []string{"invite-1"})
	srv := proxyhttp.NewServer(cfg, &stubStore{}, &fakeSynth{}, tokens)
	_, raw, err := tokens.Issue("invite-1", "stub-app")
	if err != nil {
		t.Fatalf("issue test token: %v", err)
	}
	if code, _, _ := postRaw(t, srv, raw, "Hello."); code != http.StatusOK {
		t.Fatalf("server must serve through a non-disk store, got %d", code)
	}
}

func TestRegionHeaderAndMetric(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.Region = "eu-west"
	fake := &fakeSynth{}
	srv, token := newTestServer(t, cfg, fake)

	_, header, _ := postRaw(t, srv, token, "Hello.")
	if got := header.Get("X-Region"); got != "eu-west" {
		t.Fatalf("expected X-Region eu-west, got %q", got)
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	var metrics map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &metrics); err != nil {
		t.Fatalf("bad metrics json: %v", err)
	}
	if metrics["region"] != "eu-west" {
		t.Fatalf("expected region eu-west in metrics, got %v", metrics["region"])
	}
}

func TestRequestDeadlineExceeded(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.RequestTimeoutSec = 1
	fake := &fakeSynth{delayOn: map[string]time.Duration{"Slow sentence.": 3 * time.Second}}
	srv, token := newTestServer(t, cfg, fake)

	if code, _, _ := postRaw(t, srv, token, "Slow sentence."); code != http.StatusGatewayTimeout {
		t.Fatalf("expired deadline must be 504, got %d", code)
	}
}
