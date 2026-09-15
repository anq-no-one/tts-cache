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
- `vectors/vectors.json` — shared normalization vectors. Both test
  suites read this one file. Change it and both suites fail until Go
  and Swift agree again.

## Cache key

Six fields decide the sound, so six fields decide the key:
normalized text, voice id, model id, speed, output format, language.
Everything else (timeouts, API keys, cache size, retries) never
touches the key. Scheme is `v1`.

## Quick start

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
