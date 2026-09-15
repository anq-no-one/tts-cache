# 0001 — Monorepo

Status: accepted.

Context: the project has a Go service and a Swift SDK that must agree
on normalization and key computation byte for byte.

Decision: one repository, `proxy/` for the service, `swift/` for the
SDK, `vectors/` for the shared contract. One release, versioned
through the cache key scheme.

Consequences: atomic changes across client and server, one CI run,
no drift between copies. Independent versioning is sacrificed; it is
not needed while the key scheme ties both sides together.
