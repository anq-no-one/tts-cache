# 0008 — HTTP-first universal clients

Status: accepted.

Context: the first client is iOS, but the proxy must serve any app.
Tying the project to iOS-only APIs would cap its open source value.

Decision: the proxy contract is plain HTTP (`POST /v1/synthesize`,
`GET /healthz`, `GET /metrics`) and any client can use it with zero
native code. Native SDKs (Swift now, Kotlin and JavaScript next) are
thin quality improvements — sentence splitting with platform APIs,
key computation, source-tagged results — never a requirement.
Splitting quality may vary per platform; the vectors (see 0005) set
the minimum bar.

Consequences: Android and web clients can start with raw HTTP today
and gain an SDK later without server changes.
