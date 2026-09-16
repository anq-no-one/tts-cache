# tts-cache JS client

Zero-dependency Node.js client for the tts-cache proxy. Contract order
is vectors and golden key first, networking second.

## Test

```sh
npm test
```

`node --test test/` works too. Files are numbered so contract tests
run before networking tests: `00-vectors`, `01-golden-key`,
`10-client`.

## API

```js
import { normalize, speedKey, cacheKey, splitSentences, buildRequest, Client } from './src/tts-cache.mjs';
```

Request options use one canonical shape everywhere, in
`cacheKey`, `buildRequest`, and `Client.synthesize`:

```js
{ voiceId, modelId, speed, format, language }
```

Older aliases (`voiceID`, `modelID`, `outputFormat`, `lang`)
are not read. Pass the canonical names.

- `normalize(text)` collapses whitespace, fixes space before `.`
  and `,`, lowercases. Pinned by `vectors/vectors.json`.
- `speedKey(speed)` formats speed with three decimals, `0`
  means `1`.
- `cacheKey({ text, voiceId, modelId, speed, format, language })`
  returns `v1-` plus the first 16 hex chars of SHA-256 over the
  pipe-joined parts. Golden key `v1-feab4a17bc7070e8`.
- `splitSentences(text)` mirrors the server `[.!?]+\s*` split.
- `buildRequest(text, options)` returns the `POST /v1/synthesize`
  JSON body.
- `new Client({ endpoints, appToken, fetchImpl, directFetch })`
  probes `GET /healthz` per endpoint with `measureLatency()`,
  then `synthesize(text, options)` tries endpoints in order with
  `Authorization: Bearer <app_token>`, parses `X-Sentence-Count`
  and `X-Sentence-Statuses`, and returns
  `{ audio, source, statuses, sentenceCount, key, endpoint, region }`
  plus `latencyMs` on proxy hits or `failures` on fallback paths.

Failover tries the next endpoint only on transport errors,
timeouts, HTTP `429`, and `5xx`. Any other `4xx` returns
immediately as a `proxy` result with empty audio and the
failure detail in `failures`, with no further endpoints and
no direct fallback. This matches the Swift client rule.

`directFetch` is an optional `async (text) => audioBytes` hook
for a direct provider call. The app owns its provider key
inside that closure; the client never stores provider keys.
When every endpoint fails, a set `directFetch` is called and
its audio returns with `source: 'direct'`; when unset, or when
`directFetch` itself throws, empty audio returns with
`source: 'system'`.

`source` is one of `memory`, `disk`, `proxy`, `direct`, `system`.
This client produces `memory` (repeat request in this process),
`proxy` (server audio, including an immediate `4xx` failure
with empty audio), `direct` (via `directFetch`), and `system`
(empty audio when nothing else served). `disk` stays in the
vocabulary for the app layer above (on-device file cache) so
analytics can tag every fallback step of the chain.
