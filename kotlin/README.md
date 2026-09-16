# tts-cache-kotlin

JVM SDK for `tts-cache-proxy`. It owns sentence splitting,
normalization, and key computation on the client, so the app can
predict cache keys without sending provider credentials from the device.

## Layout

- `src/main/kotlin/ttscache/TtsCache.kt` — `TtsCache` object plus
  `TtsCacheConfig`, `SynthesizeRequest`, and `SynthesizeResponse`.
  No external dependencies, JDK only (`java.net`, `java.security`,
  `java.util`).
  - `normalize` mirrors Go `Normalize` (trim, collapse whitespace,
    fix space before `.` and `,`, lowercase).
  - `speedKey` mirrors Go (`0` becomes `"1"`, else 3-decimal format).
  - `cacheKey` mirrors Go (`v1-` plus first 16 hex chars of SHA-256
    over `KeyVersion|normalized|voice|model|speed|format|language`).
  - `splitSentences` mirrors the server regex split on `[.!?]+\s*`.
  - `synthesizeRequest` builds the POST `/v1/synthesize` URL and JSON body.
  - `fetchSynthesize` sends that request with `HttpURLConnection`,
    Bearer app token, and configurable connect/read timeouts, then
    returns the status code, audio bytes, and the
    `X-Sentence-Statuses` / `X-Region` headers. Connection failures
    surface as `IOException`.
- `src/test/kotlin/ttscache/ContractCheck.kt` — dependency-free
  contract runner (plain `main` with `check` assertions, no test
  framework). Loads `vectors/vectors.json`, asserts every
  normalization vector and the golden key `v1-feab4a17bc7070e8`,
  then exercises `fetchSynthesize` against a JDK built-in
  `com.sun.net.httpserver` fake: 200 path, 206 partial path with
  statuses parsing, and the connection-failure path.

## Compile and run the contract checks

Needs `kotlinc` and JDK 17. From this directory:

```sh
kotlinc src/main/kotlin/ttscache/TtsCache.kt src/test/kotlin/ttscache/ContractCheck.kt -include-runtime -d /tmp/tts-kotlin.jar
java -jar /tmp/tts-kotlin.jar
```

The runner expects the repo checkout layout, so run it with
`kotlin/` as the working directory (it reads `../vectors/vectors.json`).
CI does the same in `.github/workflows/kotlin.yml`.

## Networking state

Minimal networking is in: `fetchSynthesize` posts the
`synthesizeRequest` URL and JSON body with the app token and returns
the status code, audio bytes, and the `X-Sentence-Statuses` /
`X-Region` headers, so callers can read per-sentence `hit` /
`synthesized` / `error` outcomes including the 206 partial case.
