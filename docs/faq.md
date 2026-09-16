# FAQ

## How does this save money?

Upstream providers bill per character. The proxy splits each request into sentences and serves repeats from disk, so a repeated sentence costs zero upstream characters. `GET /metrics` reports the billing-relevant saving as `chars_saved` (characters served from cache) plus `cache_hits`, `cache_misses`, and `hit_rate`. The repo gives no percentage or payout figure; run your own traffic and read those counters.

## Does cached audio sound different from direct synthesis?

No. A cache hit returns the exact bytes the upstream produced for the same six sound fields, concatenated in sentence order. Keeping the voice identical to direct synthesis is an explicit project goal (see `docs/idea.md`). Timeouts, API keys, cache size, and retries never enter the cache key, so transport tweaks cannot change the sound.

## What about prosody across sentence boundaries?

The server synthesizes and stores per-sentence clips, then concatenates them in order. The repo says nothing about cross-sentence intonation smoothing: expect each sentence to sound as it would synthesized alone. If your copy needs flowing multi-sentence prosody, that is a current limitation, not a tuned feature.

## Which audio formats are supported?

The only format the repo documents is `mp3_44100_128` (the `DEFAULT_FORMAT` default), served back as `audio/mpeg`. `format` is one of the six cache-key fields, so any format string you send is keyed separately and forwarded upstream as `output_format`, but no other format values are listed or tested anywhere in the tree.

## How does auth work?

Three secrets with three jobs (`proxy/internal/http/server.go`, `proxy/internal/auth/`). An invitation code from `INVITE_CODES` gates `POST /v1/register`, which issues a per-app Bearer [REDACTED] shown once and stored hashed in `$CACHE_DIR/tokens.json`. The app token gates `POST /v1/synthesize` (`401` without it). The static `ADMIN_TOKEN` gates `DELETE /v1/tokens/{id}` (`403` without it, `204` on revoke). Per-token rate limits answer `429` with `Retry-After: 60` past `RATE_PER_MIN`. While `ADMIN_TOKEN` is empty, revocation always answers `403`, so production needs it set.

## Do apps share cached audio with each other?

Yes. The cache key holds only the six sound fields (normalized text, voice, model, speed, format, language), deliberately excluding anything app-specific, so identical phrases share clips across apps and contexts (see `docs/decisions/0004-cache-key.md`). Sharing is at the audio level only: app tokens stay per-app and revocable by id.

## What happens when the disk fills up?

Set `CACHE_MAX_BYTES` to bound cached audio with least-recently-used eviction; `0` (the default) means unlimited. Evictions and current size are visible as `evictions` and `cache_bytes` in `GET /metrics`. Eviction only removes clips, never tokens: the next request for an evicted sentence re-synthesizes it upstream and the cache rewarms itself.

## How do regions work?

Each stack sets its own `REGION`, served back as the `X-Region` response header and the `region` metrics field. Regions share nothing and each rewarms its own cache from the same upstream (see `docs/decisions/0007-regions.md` and `0009-shared-storage.md`). SDKs hold an ordered endpoint list, probe `GET /healthz` for latency, and fail over to the next region before falling back to direct synthesis. A second live region is still gated behind an explicit rollout decision; one region keeps roughly the 99.5 posture of decision 0007, with the client fallback chain keeping playback alive.

## Do the Swift, Kotlin, and JS SDKs behave the same?

They agree on the contract, not on features. All three reproduce the vectors in `vectors/vectors.json` and the golden key `v1-feab4a17bc7070e8`, checked by every suite, so keys computed anywhere match. Networking differs: Swift (`swift/`) is the reference client with latency-ranked failover and audio-source tagging (`TTSClient`), JS (`js/`) is a full client with in-process memory cache, failover, and a `directFetch` hook (`Client`), Kotlin (`kotlin/`) has contract parity plus a single-shot `fetchSynthesize` over `HttpURLConnection` with no failover client yet. One known asymmetry: Swift splits sentences with `NLTokenizer` while the server splits on `[.!?]+\s*`; client splitting is UX only because the server sentence list is authoritative.

## What happens when the cache key scheme is upgraded?

Audio identity is pinned by the scheme, currently `v1`. A bump (for example `v1` to `v2`) makes old clips stop matching, so the cache rewarms itself from upstream with no manual cleanup; rolling back revalidates the older clips still on disk, so keep the volume on rollback (see `docs/deploy.md`). Any normalization change must update `vectors/vectors.json`, bump the scheme, and keep every suite green (`CONTRIBUTING.md`).

## What do I need to self-host?

A container runtime, a provider key for an ElevenLabs-compatible upstream (`UPSTREAM_API_KEY`, the proxy never runs without one because misses are synthesized upstream), and a shell that can reach the host. The minimal topology is one container plus one named volume (`tts-data` at `CACHE_DIR`), started with env-only config per `proxy/docker-compose.yml`; `proxy/deploy/` mirrors it for orchestrators. The image runs as non-root and needs only a writable cache dir, and the cache volume is the only state worth backing up.
