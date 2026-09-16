## Goal

Turn the current MVP into a proxy the new session can deploy to one
host and connect to a consumer app, with Android and web clients able
to follow without server changes.

## Success Criteria

- One host serves repeated sentences from disk and synthesizes new
  ones once, with per-sentence isolation so one slow sentence never
  fails the whole request.
- The Swift SDK reports the audio source with every result and fails
  over across an endpoint list.
- A Kotlin SDK and a JavaScript client reproduce the shared vectors
  and the golden key.
- A deploy guide takes a fresh host from zero to serving with one
  volume and env-only config.

## Context And Current Facts

- Monorepo `tts-cache`: `proxy/` Go service, `swift/` SDK,
  `vectors/vectors.json` shared contract.
- `go test ./...` green in `proxy/`, `swift test` 5 of 5 in `swift/`,
  golden key `v1-feab4a17bc7070e8` asserted on both sides.
- Current `POST /v1/synthesize` fails the whole request on the first
  upstream error and has no auth, no rate limits, no request
  coalescing.
- Binding decisions live in `docs/decisions/0001` through `0008`.

## Constraints And Non-goals

- Speech must never break the app (decision 0006). Savings are
  secondary to availability.
- No second region and no shared object storage in this round
  (decision 0007). Ship region-ready, not multi-region.
- No breaking change to the `/v1/synthesize` request shape; additive
  fields and headers only.
- No provider keys on devices beyond what the consumer app already holds.

## Key Decisions

- Harden the single-host proxy before any client expansion. A second
  SDK against a fragile server multiplies failure modes.
- Kotlin SDK before JavaScript client. Mobile parity proves the
  vectors process on a second native platform; the JS client then
  follows as a thinner port.
- SDK-measured endpoint selection first, DNS routing later. It needs
  no infrastructure and keeps the project self-hostable.
- Per-sentence partial results over all-or-nothing. Matches the
  fallback chain: play what the proxy has, fill gaps per the
  product's voice policy.

## Recommended Approach

Work in order: proxy hardening first, then Swift SDK result and
endpoint work, then Kotlin SDK, then JavaScript client plus deploy
guide. Each unit lands tested and documented before the next starts.
New clients start by reproducing `vectors/vectors.json` and the
golden key before any networking code.

## Work Plan

1. Proxy per-sentence isolation. Return concatenated audio for
   available sentences plus a per-sentence status map; never fail the
   whole batch on one upstream error.
2. Proxy protections. Upstream timeout plus whole-request deadline,
   text size limit, per-app tokens with revocation, basic rate
   limits, singleflight so concurrent identical sentences cause one
   upstream call.
3. Region label. `REGION` env plumbed into a response header and the
   metrics output. No routing yet.
4. Swift SDK source-tagged results. Result type carries the audio
   source (memory, disk, proxy, direct, system) and proxy failure
   detail (host, code, latency) for analytics.
5. Swift SDK endpoint list. Ordered endpoints with startup latency
   measurement and failover to the next endpoint before direct.
6. Kotlin SDK. Normalize, split, key, request builder against the
   shared vectors and golden key, then minimal fetch and playback
   wiring.
7. JavaScript client. Same contract order: vectors and golden key
   first, then fetch wrapper returning audio plus source tags.
8. Deploy guide and open source hygiene. Single-host compose guide,
   metrics meaning, license file, contributing notes.

## Validation Plan

- `go test ./...` in `proxy/` and `swift test` in `swift/` stay
  green after every slice.
- Fault injection against a fake upstream: one slow sentence must
  not block others; proxy down must surface a tagged direct fallback,
  never silence.
- New clients fail without vectors parity: vectors and golden key
  tests run before networking tests in each SDK.
- Manual listening check with a real provider key on a fixed phrase
  set before calling quality done.

## Risks / Rollback

- Partial-audio stitching may expose mp3 frame-boundary clicks.
  Mitigation: keep sentence order concatenation and verify by ear;
  rollback is server-side only, clients unchanged.
- Per-app tokens add ops burden. Mitigation: single static token
  documented for single-host use, rotation procedure in the guide.
- Highest-risk validation is the ear check on stitched audio. If it
  fails, sentence granularity is revisited before client expansion.

## Open Questions

- Partial success voice policy: mix proxy audio with system voice
  per sentence, or fall back to one voice for the whole phrase.
- Endpoint selection: SDK-measured latency as default, DNS-based
  routing as a later option. No DNS work in this round.
