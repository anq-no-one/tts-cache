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
- `new Client({ endpoints, appToken, fetchImpl })` probes
  `GET /healthz` per endpoint with `measureLatency()`, then
  `synthesize(text, options)` tries endpoints in order with
  `Authorization: Bearer <app_token>`, parses `X-Sentence-Count`
  and `X-Sentence-Statuses`, fails over to the next endpoint,
  and returns `{ audio, source, statuses }`.

`source` is one of `memory`, `disk`, `proxy`, `direct`, `system`.
This client produces `memory` (repeat request in this process)
and `proxy` (fresh server audio). `disk`, `direct`, and `system`
belong to the app layer above: on-device file cache, direct
provider call with the app's own provider key, and platform
system voice. They stay in the vocabulary so analytics can tag
every fallback step of the chain.
