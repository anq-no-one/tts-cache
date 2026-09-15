package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

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
	cfg := proxyhttp.Config{
		CacheDir:      env("CACHE_DIR", "./data"),
		UpstreamBase:  env("UPSTREAM_BASE_URL", "https://api.fish.audio/compat/elevenlabs/v1"),
		UpstreamKey:   os.Getenv("UPSTREAM_API_KEY"),
		DefaultModel:  env("DEFAULT_MODEL", "fish-audio/s2.1-pro"),
		DefaultFormat: env("DEFAULT_FORMAT", "mp3_44100_128"),
		DefaultLang:   env("DEFAULT_LANG", "en"),
		TimeoutSec:    timeout,
	}
	srv := proxyhttp.NewServer(cfg, cache.NewStore(cfg.CacheDir), upstream.ElevenLabsCompat{})
	log.Printf("tts-cache-proxy listening on :%s cache=%s", port, cfg.CacheDir)
	log.Fatal(http.ListenAndServe(":"+port, srv.Handler()))
}
