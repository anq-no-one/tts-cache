package http

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anq-no-one/tts-cache/proxy/internal/auth"
	"github.com/anq-no-one/tts-cache/proxy/internal/cache"
	"github.com/anq-no-one/tts-cache/proxy/internal/upstream"
)

type Config struct {
	CacheDir          string
	UpstreamBase      string
	UpstreamKey       string
	DefaultModel      string
	DefaultFormat     string
	DefaultLang       string
	TimeoutSec        int
	MaxTextChars      int
	RequestTimeoutSec int
	RatePerMin        int
	AdminToken        string
	Region            string
}

type Server struct {
	cfg     Config
	store   cache.Store
	synth   upstream.Synthesizer
	tokens  *auth.Store
	limiter *rateLimiter

	coalescer flightGroup

	hits        atomic.Int64
	miss        atomic.Int64
	saved       atomic.Int64
	authErrors  atomic.Int64
	rateLimited atomic.Int64
}

func NewServer(cfg Config, store cache.Store, synth upstream.Synthesizer, tokens *auth.Store) *Server {
	return &Server{cfg: cfg, store: store, synth: synth, tokens: tokens, limiter: newRateLimiter(cfg.RatePerMin)}
}

type synthRequest struct {
	Text     string  `json:"text"`
	VoiceID  string  `json:"voice_id"`
	ModelID  string  `json:"model_id"`
	Speed    float64 `json:"speed"`
	Format   string  `json:"format"`
	Language string  `json:"language"`
}

var sentenceEnd = regexp.MustCompile(`[.!?]+\s*`)

const (
	statusHit         = "hit"
	statusSynthesized = "synthesized"
	statusError       = "error"
)

type sentenceStatus struct {
	Index  int    `json:"index"`
	Key    string `json:"key"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func SplitSentences(text string) []string {
	var out []string
	start := 0
	idx := sentenceEnd.FindAllStringIndex(text, -1)
	for _, m := range idx {
		part := strings.TrimSpace(text[start:m[1]])
		if part != "" {
			out = append(out, part)
		}
		start = m[1]
	}
	if rest := strings.TrimSpace(text[start:]); rest != "" {
		out = append(out, rest)
	}
	if len(out) == 0 && strings.TrimSpace(text) != "" {
		out = []string{strings.TrimSpace(text)}
	}
	return out
}

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	mux.HandleFunc("POST /v1/register", s.handleRegister)
	mux.HandleFunc("DELETE /v1/tokens/{id}", s.handleRevoke)
	mux.HandleFunc("POST /v1/synthesize", s.handleSynthesize)
	return mux
}

func (s *Server) Handler() http.Handler { return s.routes() }

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{
		"cache_hits":   s.hits.Load(),
		"cache_misses": s.miss.Load(),
		"chars_saved":  s.saved.Load(),
		"hit_rate":     hitRate(s.hits.Load(), s.miss.Load()),
		"key_version":  cache.KeyVersion,
		"auth_errors":  s.authErrors.Load(),
		"rate_limited": s.rateLimited.Load(),
		"evictions":    s.store.Evictions(),
		"cache_bytes":  s.store.Bytes(),
		"region":       s.cfg.Region,
	})
}

func hitRate(h, m int64) float64 {
	if h+m == 0 {
		return 0
	}
	return float64(h) / float64(h+m)
}

func bearerToken(r *http.Request) string {
	raw := r.Header.Get("Authorization")
	kind, token, _ := strings.Cut(raw, " ")
	if !strings.EqualFold(kind, "bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InvitationCode string `json:"invitation_code"`
		AppName        string `json:"app_name"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.AppName) == "" {
		http.Error(w, "app_name is required", http.StatusBadRequest)
		return
	}
	id, token, err := s.tokens.Issue(req.InvitationCode, strings.TrimSpace(req.AppName))
	if err != nil {
		http.Error(w, "invitation required", http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"token_id":  id,
		"app_token": token,
		"app_name":  strings.TrimSpace(req.AppName),
	})
}

func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	if !auth.CheckSecret(bearerToken(r), s.cfg.AdminToken) {
		s.authErrors.Add(1)
		http.Error(w, "admin required", http.StatusForbidden)
		return
	}
	if !s.tokens.Revoke(r.PathValue("id")) {
		http.Error(w, "unknown token", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSynthesize(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Region", s.cfg.Region)
	token, ok := s.tokens.Validate(bearerToken(r))
	if !ok {
		s.authErrors.Add(1)
		http.Error(w, "valid bearer token required", http.StatusUnauthorized)
		return
	}
	if !s.limiter.Allow(token.ID) {
		s.rateLimited.Add(1)
		w.Header().Set("Retry-After", "60")
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	var req synthRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Text) == "" || strings.TrimSpace(req.VoiceID) == "" {
		http.Error(w, "text and voice_id are required", http.StatusBadRequest)
		return
	}
	maxChars := s.cfg.MaxTextChars
	if maxChars == 0 {
		maxChars = 5000
	}
	if len([]rune(req.Text)) > maxChars {
		http.Error(w, "text too long", http.StatusRequestEntityTooLarge)
		return
	}
	model := or(req.ModelID, s.cfg.DefaultModel)
	format := or(req.Format, s.cfg.DefaultFormat)
	lang := or(req.Language, s.cfg.DefaultLang)
	speed := req.Speed
	if speed == 0 {
		speed = 1
	}
	reqTimeout := time.Duration(s.cfg.RequestTimeoutSec) * time.Second
	if reqTimeout == 0 {
		reqTimeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), reqTimeout)
	defer cancel()

	sentences := SplitSentences(req.Text)
	audio, statuses := s.fetchSentences(ctx, sentenceParams{
		VoiceID: req.VoiceID, Model: model, Format: format, Language: lang, Speed: speed,
	}, sentences)
	if ctx.Err() != nil {
		http.Error(w, "request deadline exceeded", http.StatusGatewayTimeout)
		return
	}
	writeAudioResponse(w, audio, statuses)
}

type sentenceParams struct {
	VoiceID  string
	Model    string
	Format   string
	Language string
	Speed    float64
}

func (s *Server) fetchSentences(ctx context.Context, p sentenceParams, sentences []string) ([][]byte, []sentenceStatus) {
	audio := make([][]byte, len(sentences))
	statuses := make([]sentenceStatus, len(sentences))
	var wg sync.WaitGroup
	for i, sent := range sentences {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := cache.Key(cache.Params{
				Text: sent, VoiceID: p.VoiceID, ModelID: p.Model,
				Speed: p.Speed, OutputFormat: p.Format, Language: p.Language,
			})
			statuses[i] = sentenceStatus{Index: i, Key: key}
			if cached, ok := s.store.Get(key); ok {
				s.hits.Add(1)
				s.saved.Add(int64(len(sent)))
				audio[i] = cached
				statuses[i].Status = statusHit
				return
			}
			got, err := s.coalescer.Coalesce(key, func() ([]byte, error) {
				return s.synthesizeFresh(ctx, p, sent, key)
			})
			if err != nil {
				statuses[i].Status = statusError
				statuses[i].Error = err.Error()
				return
			}
			audio[i] = got
			statuses[i].Status = statusSynthesized
		}()
	}
	wg.Wait()
	return audio, statuses
}

func (s *Server) synthesizeFresh(ctx context.Context, p sentenceParams, sent, key string) ([]byte, error) {
	if cached, ok := s.store.Get(key); ok {
		return cached, nil
	}
	s.miss.Add(1)
	timeout := time.Duration(s.cfg.TimeoutSec) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	fresh, err := s.synth.Synthesize(ctx, upstream.Request{
		Text: sent, VoiceID: p.VoiceID, ModelID: p.Model,
		Speed: p.Speed, Format: p.Format,
		APIKey: s.cfg.UpstreamKey, BaseURL: s.cfg.UpstreamBase,
		Timeout: timeout,
	})
	if err != nil {
		return nil, err
	}
	_ = s.store.Put(key, fresh, cache.Meta{
		Text: sent, VoiceID: p.VoiceID, ModelID: p.Model,
	})
	return fresh, nil
}

func writeAudioResponse(w http.ResponseWriter, audio [][]byte, statuses []sentenceStatus) {
	var combined []byte
	failed := 0
	for i := range statuses {
		if statuses[i].Status == statusError {
			failed++
			continue
		}
		combined = append(combined, audio[i]...)
	}
	rawStatuses, _ := json.Marshal(statuses)
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("X-Cache-Key-Version", cache.KeyVersion)
	w.Header().Set("X-Sentence-Count", strconv.Itoa(len(statuses)))
	w.Header().Set("X-Sentence-Statuses", string(rawStatuses))
	if failed == len(statuses) {
		http.Error(w, "upstream: "+statuses[0].Error, http.StatusBadGateway)
		return
	}
	if failed > 0 {
		w.WriteHeader(http.StatusPartialContent)
	}
	w.Write(combined)
}

func or(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
