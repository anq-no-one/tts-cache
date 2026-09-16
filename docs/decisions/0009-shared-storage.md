# 0009 — Storage seam now, shared storage later

Status: accepted. Supersedes 0007.

## Context

0007 chose independent per-region disk caches with no shared state.
The program plan keeps that as the running topology but requires the
code to be ready for shared cache storage inside a region: a second
replica or a host replacement should not force a redesign of the
cache layer. Cross-region replication stays out of scope; regions
rewarm from the same upstream.

## Decision

- The cache behind `proxy/internal/cache` keeps its current
  interface. Local disk is the implementation today.
- Shared object storage arrives later as a second implementation of
  the same interface, used only inside one region. No caller changes
  when it lands.
- Single host plus compose remains the minimal supported topology.
- A second live region stays gated behind an explicit rollout
  decision. The `REGION` label and the SDK endpoint list already ship,
  so enabling it needs no redesign.

## Consequences

- One region keeps the availability posture of 0007 (around 99.5,
  with the SDK fallback chain keeping the feature alive). True 99.9
  waits for the gated second region.
- Cache writes never cross regions, so the shared-storage work has
  no consistency story beyond one region.
- Horizontal scale inside a region becomes a storage-swap plus a load
  balancer, not a rewrite.
