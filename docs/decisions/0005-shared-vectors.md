# 0005 — Shared vectors as client-server contract

Status: accepted.

Context: normalization and key code exist twice, once per
platform. A one-character difference means silent cache misses that
look like the proxy simply not working.

Decision: `vectors/vectors.json` is the single contract both test
suites read, plus a golden key both sides assert
(`v1-feab4a17bc7070e8`). Any normalization change must update the
vectors, bump the key scheme, and keep both suites green.

Consequences: adding a client in any language starts by reproducing
these vectors. No vectors parity, no new client.
