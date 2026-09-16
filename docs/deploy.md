# Deploy guide

Fastest path from a fresh host to serving audio. Every variable and
endpoint below is verified against `proxy/cmd/server/main.go` and
`proxy/internal/http/server.go`. Nothing here is invented.

## Prerequisites

- A container runtime (Docker with compose, or any orchestrator that
  can run an image plus a writable volume).
- A TTS provider key for an ElevenLabs-compatible upstream
  (Fish Audio format by default). The proxy never runs without one:
  cache misses are synthesized upstream.
- A shell that can reach the host on the configured port.

## Config reference (env only)

No config files. Every knob is an environment variable, read in
`proxy/cmd/server/main.go`.

| Variable | Default | Meaning |
|---|---|---|
| `UPSTREAM_API_KEY` | none (required) | Provider key used on cache misses. |
| `UPSTREAM_BASE_URL` | `https://api.fish.audio/compat/elevenlabs/v1` | Upstream base URL. |
| `PORT` | `8080` | Listen port. |
| `CACHE_DIR` | `./data` | Cache dir. Holds audio plus `tokens.json`. Back this up, nothing else. |
| `DEFAULT_MODEL` | `fish-audio/s2.1-pro` | Model when the request leaves `model_id` empty. |
| `DEFAULT_FORMAT` | `mp3_44100_128` | Format when the request leaves `format` empty. |
| `DEFAULT_LANG` | `en` | Language when the request leaves `language` empty. |
| `UPSTREAM_TIMEOUT_SEC` | `30` | Per-sentence upstream timeout. |
| `REQUEST_TIMEOUT_SEC` | `60` | Whole-request deadline. Past it the server answers `504`. |
| `MAX_TEXT_CHARS` | `5000` | Longer texts are rejected with `413`. Counted in runes. |
| `RATE_PER_MIN` | `60` | Requests per app token per minute. Overage gets `429` with `Retry-After: 60`. |
| `INVITE_CODES` | empty | Comma-separated invitation codes. Registration without a listed code gets `403`. |
| `ADMIN_TOKEN` | empty | Static admin secret. Revocation without it gets `403`. |
| `CACHE_MAX_BYTES` | `0` (unlimited) | Byte cap over cached audio. Past it the least-recently-used clips are evicted. |
| `REGION` | empty | Region label. Returned as the `X-Region` header and as `region` in metrics. |

`proxy/docker-compose.yml` and `proxy/deploy/deployment.yaml` list
exactly these variables, same names, same defaults.

## Path A: single host with compose

Minimal topology: one container, one volume.

```sh
cd proxy
UPSTREAM_API_KEY=<provider-key> \
INVITE_CODES=<pick-a-code> \
ADMIN_TOKEN=<pick-a-secret> \
REGION=<for-example-eu-west> \
docker compose up --build -d
curl localhost:8080/healthz
```

`curl` answers `ok`. The named volume `tts-data` (mounted at
`CACHE_DIR`) is the only state. Everything else can be recreated.

## Path B: managed orchestrator with manifests

`proxy/deploy/` holds portable manifests mirroring the same image,
env-only config, and a writable cache volume:

- `deployment.yaml` — one replica, non-root user, liveness and
  readiness probes on `GET /healthz`, cache volume at `/app/data`.
- `service.yaml` — cluster-internal `Service` on port `80`.
- `pvc.yaml` — `10Gi` claim `tts-cache-data` for the cache.

Steps:

1. Build the image from `proxy/` and push it to your registry.
2. Point `image:` in `deployment.yaml` at your registry. No registry
   push or cloud credentials live in this repo.
3. Fill the `env:` block (same table as above) and apply in order:
   `pvc.yaml`, then `deployment.yaml`, then `service.yaml`.
4. Check `GET /healthz` through the service.

Stay at one replica per region for now. A second replica needs the
shared-storage change in
`docs/decisions/0009-shared-storage.md`, which is designed but not
built.

## First run

Register (needs an invitation code, returns the app token once):

```sh
curl -s -X POST localhost:8080/v1/register \
  -H 'Content-Type: application/json' \
  -d '{"invitation_code": "<code>", "app_name": "my-app"}'
```

Response is `201` with `token_id`, `app_token`, `app_name`. Store
`app_token`; the server keeps only its hash in
`$CACHE_DIR/tokens.json`.

Synthesize (token goes in the header, not the body):

```sh
curl -s -X POST localhost:8080/v1/synthesize \
  -H "Authorization: Bearer <app-token>" \
  -H 'Content-Type: application/json' \
  -d '{"text": "Hello. World.", "voice_id": "<voice>"}' \
  -D - -o out.mp3
```

Response is `audio/mpeg` with `X-Sentence-Count`,
`X-Sentence-Statuses` (per-sentence `hit` / `synthesized` / `error`
against the server-authoritative sentence list), `X-Region`, and
`X-Cache-Key-Version`. One failed sentence gives `206` with the audio
that is available; all failed gives `502`. Empty or over-long input
gives `400` / `413`; bad token gives `401`.

Per-sentence status rides in the `X-Sentence-Statuses` header rather
than the body because the body carries audio. The header grows with
the sentence count, but `MAX_TEXT_CHARS` caps the text length and
therefore the sentence count, so the header size stays bounded.

Check metrics:

```sh
curl -s localhost:8080/metrics
```

JSON with `cache_hits`, `cache_misses`, `chars_saved`, `hit_rate`,
`auth_errors`, `rate_limited`, `evictions`, `cache_bytes`,
`key_version`, `region`. Rising `cache_hits` on the second identical
request proves the cache is working.

## Token rotation

There are two secrets and they rotate differently.

- `ADMIN_TOKEN` (the single static token): pick a new value, update
  the env in compose or the manifest, restart or roll the workload.
  Old value stops working on restart. While `ADMIN_TOKEN` is empty,
  revocation always answers `403`, so never run production without it.
- App tokens: revoking needs the admin token and answers `204`:

```sh
curl -s -X DELETE localhost:8080/v1/tokens/<token-id> \
  -H "Authorization: Bearer <admin-token>" -o /dev/null -w '%{http_code}\n'
```

Then register a fresh token for the app via `POST /v1/register`.
Revocation is recorded in `tokens.json`, so it survives restarts as
long as the cache volume survives.

## Upgrade and versioning

Audio identity is pinned by the cache key scheme, currently `v1`
(`proxy/internal/cache/key.go`, mirrored in every SDK). Timeouts, API
keys, cache size, and retries never touch the key.

- Normal upgrade: pull or rebuild the image, restart. The cache
  volume is compatible across restarts on the same key scheme.
- Key scheme bump (for example `v1` to `v2`): old clips stop matching
  and the cache rewarms itself from upstream. No manual cleanup.
  Rollback to an older scheme revalidates the older clips still on
  disk, so keep the volume when rolling back.
