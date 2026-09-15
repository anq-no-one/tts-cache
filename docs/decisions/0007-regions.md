# 0007 — Regions without shared state

Status: accepted.

Context: users in Asia should not wait on a US host, but
cross-region cache replication is expensive and unnecessary: every
region can warm its own cache from the same upstream.

Decision: each region runs an identical stateless stack with its own
disk cache and a region label in responses and metrics. No shared
state between regions. SDKs hold an endpoint list, pick the fastest
at start, and fail over to the next region before going direct.
Start with one region; the label and endpoint list ship now so the
second region needs no redesign.

Consequences: one region cannot promise beyond roughly 99.5, which
is acceptable because the fallback chain (see 0006) keeps the
feature alive. True 99.9 waits for two regions behind latency
routing.
