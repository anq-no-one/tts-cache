# 0003 — Sentence-level caching

Status: accepted.

Context: splitting text into single words would maximize cache hits
but destroys prosody. A word synthesized alone carries the wrong
intonation, and stitching words sounds robotic.

Decision: split on sentence boundaries, synthesize and cache whole
sentences, concatenate in order. This matches consumer apps that
already cache sentence clips and play them with a short gap.

Consequences: fewer hits than word-level, but quality identical to
direct synthesis. Sentence splitting differs per platform, so every
client must prove parity through the shared vectors (see 0005).
