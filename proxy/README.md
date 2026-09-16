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
  returns `audio/mpeg`, sentences concatenated in order. Requires
  `Authorization: Bearer <app_token>`.
  Every response carries `X-Sentence-Count` and `X-Sentence-Statuses`
  (JSON per sentence: index, cache key, `hit` / `synthesized` /
  `error` plus error text). The server sentence list is authoritative.
  One failed sentence gives `206 Partial Content` with the available
  audio; all failed gives `502`.
- `POST /v1/register` with JSON
  `{"invitation_code": "...", "app_name": "..."}` issues an app token.
  Registration needs an invitation code; using a token does not.
  Returns `201` with `token_id`, `app_token`, `app_name`. The raw
  token is shown once.
- `DELETE /v1/tokens/{id}` revokes a token. Requires
  `Authorization: Bearer <ADMIN_TOKEN>`, returns `204`.
- `GET /healthz` returns `ok`.
- `GET /metrics` returns JSON with `region` plus the counters below.
- Every `POST /v1/synthesize` response carries an `X-Region` header
  with the same value as `region` in metrics (empty when `REGION`
  is unset), so a client failing over across regions can tag which
  region served the audio.

## Metrics

- `cache_hits` / `cache_misses`: per-sentence cache outcomes.
- `chars_saved`: sum of characters served from cache instead of the
  upstream, the billing-relevant saving.
- `hit_rate`: `cache_hits / (cache_hits + cache_misses)`, `0` when
  nothing was requested yet.
- `auth_errors`: requests rejected for bad or missing credentials
  (`401` on synthesize, `403` on register/revoke).
- `rate_limited`: requests rejected with `429` after exceeding
  `RATE_PER_MIN`.
- `evictions`: audio files removed by the `CACHE_MAX_BYTES` LRU cap.
- `cache_bytes`: current cached audio size in bytes.
- `key_version`: cache key scheme, pinned against
  `vectors/vectors.json`.
- `region`: the `REGION` value, so dashboards can split one
  federated view per stack.

## Config (env)

- `PORT`, default `8080`
- `CACHE_DIR`, default `./data`
- `UPSTREAM_BASE_URL`, default Fish Audio compat endpoint
- `UPSTREAM_API_KEY`, no default
- `DEFAULT_MODEL`, `DEFAULT_FORMAT`, `DEFAULT_LANG`
- `UPSTREAM_TIMEOUT_SEC`, default `30`
- `MAX_TEXT_CHARS`, default `5000`, longer texts get `413`
- `REQUEST_TIMEOUT_SEC`, default `60`, whole-request deadline
- `RATE_PER_MIN`, default `60` requests per token, overage gets `429`
- `INVITE_CODES`, comma-separated invitation codes for registration
- `ADMIN_TOKEN`, bearer token allowed to revoke app tokens
- `CACHE_MAX_BYTES`, default `0` (unlimited), byte-size LRU cap over cached audio
- `REGION`, default empty. Labels this stack: returned as the
  `X-Region` response header and as `region` in metrics. Set a
  distinct value per stack (for example `eu-west`, `us-east`).

## Deploy notes

Start with one host and one volume:

```sh
UPSTREAM_API_KEY=your-key INVITE_CODES=welcome-1 ADMIN_TOKEN=secret-1 REGION=eu-west docker compose up --build -d
```

All config is env-only, see `docker-compose.yml` for the full list.
The cache volume (`tts-data`, mounted at `CACHE_DIR`) is the only
state; everything else can be recreated. The image runs as a non-root
user and needs nothing but a writable cache dir, so it also runs
under a read-only root filesystem with the cache path mounted
writable.

For managed orchestrators, `deploy/` holds plain YAML mirroring the
same image, env-only config, and a writable cache volume
(`deployment.yaml`, `service.yaml`, `pvc.yaml`). Build and push the
image yourself, then point `image:` at your registry; no registry
push or cloud credentials are part of this repo.

The service is stateless except the cache dir, so scaling later means
shared object storage for that dir plus two or more hosts behind any
TCP load balancer. If two regions are needed, run one stack per
region with a distinct `REGION` value, no shared state required.
