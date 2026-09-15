# 0004 — Cache key fields

Status: accepted.

Context: the key must change exactly when the sound would change,
and never for transport-only tweaks, or the cache either lies or
misses forever.

Decision: six fields form the key — normalized text, voice id, model
id, speed, output format, language — hashed as SHA256 under scheme
`v1`. Timeouts, API keys, cache size, retries, and streaming mode
never enter the key. App-specific fields such as template id or
sentence index are excluded so identical phrases share clips across
contexts. Bumping the scheme retires the old cache without manual
cleanup.

Consequences: key changes are explicit and reviewable. Language is an
explicit parameter, never the device locale, so one phrase maps to
one clip worldwide.
