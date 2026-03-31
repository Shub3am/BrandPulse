# internal/cluster — hand-rolled TF-IDF and agglomerative clustering

The one package under `internal/` that B1 does not own. It exists because Go
has no scikit-learn: pkg.go.dev returns exactly one agglomerative clustering
package, unreleased since 2020, and gonum has no clustering at all. Reasoning
in [decisions/001-go-for-agents.md](../../docs/decisions/001-go-for-agents.md).

## What this module owns

Turning a slice of short strings into groups of indices. Tokenisation,
stopwords, the TF-IDF matrix, cosine distance, and average-linkage merging.

## What it must not know about

Everything. It imports only the standard library: `math`, `sort`, `strings`,
`unicode`. No `models`, no `context`, no I/O, no logging, no configuration. It
speaks `[]string` in and `[][]int` out, and that is the whole interface.

It also has **no opinion about the cutoff or the cluster count**. Both are
arguments. The distance cutoff and the 8-cluster cap are named constants in
`bp-clusterer`, where the product decision belongs.

## Invariants and gotchas

- **Determinism is a hard requirement, not a nicety.** Two runs over the same
  corpus must return the identical `[][]int`. The demo's reproducibility and
  every number in `eval/` depend on it. Go randomises map iteration, so every
  map here has its keys sorted before they influence output: the vocabulary is
  sorted into `Matrix.Terms` before any column index is assigned, and the
  final cluster list is sorted by size then smallest member index. **A
  `for k := range m` that feeds output is a bug in this package**, not a style
  point. `TestDeterminism` runs twenty iterations and is run with `-count=5`.
- **Closest-pair ties resolve to the lowest `(i, j)`.** The scan walks the
  upper triangle in index order and compares with a strict `<`, so the first
  pair at the minimum wins. Changing that to `<=` silently takes the last tie
  and the package stops being deterministic under floating-point equality.
- **`idf = log(N / (1 + df))` goes to zero and below.** A term in exactly N-1
  documents scores 0; a term in all N scores negative. A document whose every
  term is corpus-wide therefore vectorises to **all zeros**, and `normalise`
  leaves it at zero rather than dividing. Without that guard the row becomes
  `NaN`, which does not fail here: it fails three agents downstream as
  `json: unsupported value`. `CosineDistance` reports a zero row as distance 1
  from everything, which is the honest answer.
- **`CosineDistance` can exceed 1.** Negative IDF weights mean two rows can be
  opposed, giving up to 2. A cutoff above 1.0 therefore merges the entire
  corpus into one cluster. Measured band for a well-separated corpus is
  0.80–0.98; `TestAgglomerativeCutoffBandIsWide` pins it.
- **Combining marks are part of a token.** Devanagari vowel signs are `Mn`
  marks, not letters, so splitting on `IsLetter || IsDigit` alone tears
  `डिलीवरी` into `वर`. `Tokenize` keeps `unicode.IsMark` for that reason.
- **The stopword list is English plus Hinglish and that is load-bearing.**
  Without the Hinglish half every cluster centres on `hai`, `ka`, `nahi`,
  `yaar`. Content words that happen to be Hinglish (`accha`, `bekaar`,
  `bakwas`) are deliberately absent: they are sentiment, and dropping them
  would cost the clusterer its best signal on exactly the mentions that matter.
- **The naive O(n²) implementation is the intended one.** At 400 documents the
  distance matrix is 160,000 floats and the whole merge runs in ~11 ms. A
  nearest-neighbour chain would be faster and materially harder to read, and
  this is the one algorithm in the repo that cannot be checked against a
  library. Do not optimise it without a measurement that says it matters.

## Measured, on an Apple M5, 400 documents

```
BenchmarkTFIDF400-10            5092     236604 ns/op    401363 B/op   1216 allocs/op
BenchmarkAgglomerative400-10     100   10774839 ns/op   1311606 B/op    858 allocs/op
```

## Who calls it

`agents/bp-clusterer` only. Nothing else in the repo imports it, which is why
it is B3's rather than B1's.
