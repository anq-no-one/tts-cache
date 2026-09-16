# Contributing

## Ground rules

- The shared contract is `vectors/vectors.json` plus the golden key. Any
  normalization change must update the vectors, bump the key scheme, and
  keep every client and server suite green.
- The `/v1/synthesize` request shape never breaks: additive fields and
  headers only.
- Every new client starts by reproducing the vectors and the golden key
  before any networking code.
- Decisions that change behavior go into `docs/decisions/` as a new
  numbered file. Never rewrite an accepted decision; supersede it.

## Checks

```sh
cd proxy && go test ./...
cd swift && swift test
```

Both stay green after every slice. New proxy behavior needs a test
against a fake upstream; new SDK behavior needs a vectors-parity test
first.

## Pull requests

One work-plan unit per PR, in plan order. Small, reviewed, with updated
docs when the contract or the deploy guide changes.
