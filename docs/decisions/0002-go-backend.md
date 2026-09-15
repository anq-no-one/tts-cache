# 0002 — Go for the proxy

Status: accepted.

Context: the proxy must be cheap to run anywhere as a single
deployable with a disk cache.

Decision: Go single binary, no framework, standard library HTTP.
Python was rejected: easier to start, but a heavier image and
runtime for a cache-and-forward service.

Consequences: small image, trivial cross-compile, stricter
discipline needed around error handling. Contributors need Go
toolchain familiarity.
