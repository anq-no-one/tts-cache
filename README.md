# tts-cache

Open source caching proxy for TTS providers (ElevenLabs-compatible
APIs, Fish Audio by default) plus a thin Swift client. Repeat
sentences are served from disk, new ones are synthesized once and
stored. Goal: cut TTS spend on repeated phrases without changing how
the app sounds.

## Layout

- `proxy/` — Go service. Splits text into sentences, caches audio on
  disk, forwards misses upstream. See `proxy/README.md`.
- `swift/` — Swift package `TTSCache`. Sentence splitting,
  normalization, key computation, request building. See
  `swift/README.md`.
- `kotlin/` — JVM SDK mirroring the same contract. See
  `kotlin/README.md`.
- `js/` — zero-dependency Node.js client. See `js/README.md`.
- `vectors/vectors.json` — shared normalization vectors. Every test
  suite reads this one file. Change it and all suites fail until Go,
  Swift, Kotlin, and JS agree again.
- `docs/deploy.md` — fastest path from a fresh host to serving.
- `docs/architecture.md` — code and deployment architecture.
- `CHANGELOG.md` — user-visible changes.
- `CONTRIBUTING.md` — contributor notes.
- `LICENSE` — MIT.

## Cache key

Six fields decide the sound, so six fields decide the key:
normalized text, voice id, model id, speed, output format, language.
Everything else (timeouts, API keys, cache size, retries) never
touches the key. Scheme is `v1`.

## Docs

- `docs/idea.md` — what the project is and why.
- `docs/decisions/` — accepted decisions 0001 to 0008.
- `docs/plan-next.md` — plan for the next session.

## Quick start

New host? Skip this section and follow `docs/deploy.md`, the fastest
path from zero to serving.

Proxy:

```sh
cd proxy
UPSTREAM_API_KEY=dummy go run ./cmd/server
```

SDK tests:

```sh
cd swift
swift test
```
