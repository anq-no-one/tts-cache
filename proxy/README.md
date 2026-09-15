# tts-cache-proxy

Caching proxy in front of TTS providers with an ElevenLabs-compatible
upstream (Fish Audio by default). Repeat sentences are served from disk,
new ones are synthesized once and stored.

## Why

TTS billing is per character. In apps with repeated coach phrases the
same sentences are synthesized over and over. This service sits between
the app and the provider, splits text into sentences, and reuses audio
it already generated.

## Cache key

Six fields decide the sound, so six fields decide the key.

- normalized text
- voice id
- model id
- speed
- output format
- language

Everything else (timeouts, API keys, cache size, retries) never touches
the key. Key scheme is `v1`, see `internal/cache/key.go`. The file
`vectors/vectors.json` at the repo root pins normalization. The Swift
SDK reads the same file, so client and server can never drift apart
silently.

## Run locally

```sh
UPSTREAM_API_KEY=dummy go run ./cmd/server
curl localhost:8080/healthz
```

With docker:

```sh
UPSTREAM_API_KEY=your-key docker compose up --build
```

## API

- `POST /v1/synthesize` with JSON
  `{"text": "...", "voice_id": "...", "model_id": "", "speed": 1.08, "format": "", "language": "en"}`
  returns `audio/mpeg`, sentences concatenated in order.
- `GET /healthz` returns `ok`.
- `GET /metrics` returns `cache_hits`, `cache_misses`, `chars_saved`,
  `hit_rate`.

## Config (env)

- `PORT`, default `8080`
- `CACHE_DIR`, default `./data`
- `UPSTREAM_BASE_URL`, default Fish Audio compat endpoint
- `UPSTREAM_API_KEY`, no default
- `DEFAULT_MODEL`, `DEFAULT_FORMAT`, `DEFAULT_LANG`
- `UPSTREAM_TIMEOUT_SEC`, default `30`

## Deploy notes

Start with one host and one volume. The service is stateless except the
cache dir, so scaling later means shared object storage for that dir
plus two or more hosts behind any TCP load balancer. If two regions are
needed, run one stack per region, no shared state required.
