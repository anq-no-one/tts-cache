# 0006 — Fallback chain with loud proxy errors

Status: accepted.

Context: speech must never break the app, even if every backend is
down. But a silent fallback to direct synthesis hides lost savings.

Decision: client order is device cache, proxy, direct provider,
system voice. The SDK reports the audio source with every result,
and proxy failures carry host, code, and latency for analytics and
non-fatal crash reports. Direct provider access stays possible
because the Shred app already holds a provider key; the proxy only
saves money, it never gates the feature.

Consequences: proxy downtime costs money, never users. Every direct
call is a visible savings metric, not a silent miss.
