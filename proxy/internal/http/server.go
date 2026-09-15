package http

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/anq-no-one/tts-cache/proxy/internal/cache"
	"github.com/anq-no-one/tts-cache/proxy/internal/upstream"
)

type Config struct {
	CacheDir      string
	UpstreamBase  string
	UpstreamKey   string
	DefaultModel  string
	DefaultFormat string
	DefaultLang   string
	TimeoutSec    int
}

type Server struct {
	cfg   Config
	store *cache.Store
	synth upstream.Synthesizer

	hits  atomic.Int64
	miss  atomic.Int64
	saved atomic.Int64
}

func NewServer(cfg Config, store *cache.Store, synth upstream.Synthesizer) *Server {
	return &Server{cfg: cfg, store: store, synth: synth}
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
	})
}

func hitRate(h, m int64) float64 {
	if h+m == 0 {
		return 0
	}
	return float64(h) / float64(h+m)
}

func (s *Server) handleSynthesize(w http.ResponseWriter, r *http.Request) {
	var req synthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Text) == "" || strings.TrimSpace(req.VoiceID) == "" {
		http.Error(w, "text and voice_id are required", http.StatusBadRequest)
		return
	}
	model := or(req.ModelID, s.cfg.DefaultModel)
	format := or(req.Format, s.cfg.DefaultFormat)
	lang := or(req.Language, s.cfg.DefaultLang)
	speed := req.Speed
	if speed == 0 {
		speed = 1
	}

	sentences := SplitSentences(req.Text)
	var combined []byte
	for _, sent := range sentences {
		key := cache.Key(cache.Params{
			Text: sent, VoiceID: req.VoiceID, ModelID: model,
			Speed: speed, OutputFormat: format, Language: lang,
		})
		if audio, ok := s.store.Get(key); ok {
			s.hits.Add(1)
			s.saved.Add(int64(len(sent)))
			combined = append(combined, audio...)
			continue
		}
		s.miss.Add(1)
		timeout := time.Duration(s.cfg.TimeoutSec) * time.Second
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		audio, err := s.synth.Synthesize(upstream.Request{
			Text: sent, VoiceID: req.VoiceID, ModelID: model,
			Speed: speed, Format: format,
			APIKey: s.cfg.UpstreamKey, BaseURL: s.cfg.UpstreamBase,
			Timeout: timeout,
		})
		if err != nil {
			http.Error(w, "upstream: "+err.Error(), http.StatusBadGateway)
			return
		}
		_ = s.store.Put(key, audio, cache.Meta{
			Text: sent, VoiceID: req.VoiceID, ModelID: model,
		})
		combined = append(combined, audio...)
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("X-Cache-Key-Version", cache.KeyVersion)
	w.Write(combined)
}

func or(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
