# tts-cache-swift

Thin iOS SDK for `tts-cache-proxy`. It owns sentence splitting,
normalization, and key computation on the client, so the app never
sends provider API keys from the device and the server can trust the
sentence boundaries it receives.

## Interface (four methods)

- `TTSCache.splitSentences(_:)` cuts text into sentences.
- `TTSCache.normalize(_:)` cleans one sentence for keying.
- `TTSCache.cacheKey(text:config:)` builds the `v1` cache key.
- `TTSCache.synthesizeRequest(text:config:)` builds the POST to
  `/v1/synthesize`.

`TTSCacheConfig` carries the six sound params (voice, model, speed,
format, language) plus the server `baseURL`. Transport knobs like
timeouts stay on `URLSession`, never in the key.

## Key parity with the server

Tests read the shared file `vectors/vectors.json` at the repo root,
the same file the Go server tests read. If normalization ever changes,
update that one file, bump `keyVersion`, and both test suites fail
until they agree again.
