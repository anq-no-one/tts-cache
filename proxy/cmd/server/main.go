package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/anq-no-one/tts-cache/proxy/internal/auth"
	"github.com/anq-no-one/tts-cache/proxy/internal/cache"
	proxyhttp "github.com/anq-no-one/tts-cache/proxy/internal/http"
	"github.com/anq-no-one/tts-cache/proxy/internal/upstream"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	port := env("PORT", "8080")
	timeout, _ := strconv.Atoi(env("UPSTREAM_TIMEOUT_SEC", "30"))
	maxChars, _ := strconv.Atoi(env("MAX_TEXT_CHARS", "5000"))
	reqTimeout, _ := strconv.Atoi(env("REQUEST_TIMEOUT_SEC", "60"))
	ratePerMin, _ := strconv.Atoi(env("RATE_PER_MIN", "60"))
	maxBytes, _ := strconv.ParseInt(env("CACHE_MAX_BYTES", "0"), 10, 64)
	cfg := proxyhttp.Config{
		CacheDir:          env("CACHE_DIR", "./data"),
		UpstreamBase:      env("UPSTREAM_BASE_URL", "https://api.fish.audio/compat/elevenlabs/v1"),
		UpstreamKey:       os.Getenv("UPSTREAM_API_KEY"),
		DefaultModel:      env("DEFAULT_MODEL", "fish-audio/s2.1-pro"),
		DefaultFormat:     env("DEFAULT_FORMAT", "mp3_44100_128"),
		DefaultLang:       env("DEFAULT_LANG", "en"),
		TimeoutSec:        timeout,
		MaxTextChars:      maxChars,
		RequestTimeoutSec: reqTimeout,
		RatePerMin:        ratePerMin,
		AdminToken:        os.Getenv("ADMIN_TOKEN"),
		Region:            os.Getenv("REGION"),
	}
	var invites []string
	for _, code := range strings.Split(os.Getenv("INVITE_CODES"), ",") {
		if strings.TrimSpace(code) != "" {
			invites = append(invites, strings.TrimSpace(code))
		}
	}
	store := cache.NewStore(cfg.CacheDir)
	store.MaxBytes = maxBytes
	tokensFile := env("TOKENS_FILE", filepath.Join(cfg.CacheDir, "tokens.json"))
	srv := proxyhttp.NewServer(cfg, store, upstream.ElevenLabsCompat{}, auth.NewStore(tokensFile, invites))
	log.Printf("tts-cache-proxy listening on :%s cache=%s", port, cfg.CacheDir)
	log.Fatal(http.ListenAndServe(":"+port, srv.Handler()))
}
