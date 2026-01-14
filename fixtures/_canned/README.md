# fixtures/_canned

Hand-written payloads for `internal/anakin`'s own tests. **Nothing here is a
recording**, and no counter, chart or sentence in the product may be fed from
this directory: every file says so in a `_canned` key, and the bodies say
CANNED FIXTURE in the text an adapter would read.

They exist to prove that the five `Client` methods route through the cache, the
budget and the fixture reader. That is the Phase 1 gate and it cannot wait for
B2's live session.

Real recordings live one level up, in `fixtures/<source>/`, written by
`BP_FIXTURE_MODE=record`. The field names in here are **not** the real Anakin
field names; the shapes follow `docs/research/anakin.md`, which marks the Wire,
map and crawl payload names UNVERIFIED.

The filenames are `query_hash` values, so they are pinned to the exact arguments
in `internal/anakin/anakin_test.go`. Change a query there and the test fails
naming the path it wanted, which is the path to create.
