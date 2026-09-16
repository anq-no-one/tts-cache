# Changelog

## Unreleased

## 0.1.0 - 2026-09-16

- Auth: apps register with an invitation code and get a revocable
  bearer token (`POST /v1/register`, `DELETE /v1/tokens/{id}`).
  Synthesis needs the token; revocation needs the admin token.
  Per-token rate limits (`429` past `RATE_PER_MIN`).
- Partial results: one failed sentence no longer fails the request.
  Available audio ships with `206` plus per-sentence statuses; total
  failure is `502`. The server sentence list is authoritative.
- Cache cap: `CACHE_MAX_BYTES` bounds disk usage with LRU eviction,
  surfaced as `evictions` and `cache_bytes` in `GET /metrics`.
- Regions: `REGION` labels every response (`X-Region`) and metrics.
  Run one stack per region with independent caches; SDKs probe
  endpoints by latency and fail over before going direct.
- SDKs: Swift reference client with latency-ranked failover and
  audio-source tagging, plus Kotlin and JavaScript ports pinned to
  the same normalization vectors and golden key.
- Open source: MIT license (`LICENSE`) and contributor notes
  (`CONTRIBUTING.md`).
