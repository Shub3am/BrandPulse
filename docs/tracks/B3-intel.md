# Track B3: clustering, enricher, clusterer, share-of-voice, eval

**Branch:** `track/b3-intel`
**Owns:** `internal/cluster/`, `agents/bp-enricher/`, `agents/bp-clusterer/`,
`agents/bp-sov/`, `internal/prompts/enricher.md`,
`internal/prompts/clusterer.md`, `eval/`
**Blocked by:** B1's signature commit to compile, B1's Phase 1 gate to run.
Task 5 onward needs B2's fixtures.
**Blocks:** nothing, but `eval/` produces the cost number the pitch depends on,
and `internal/cluster` is the one package in this repo that no library gives us.

Read [PLAN.md](../../PLAN.md), [CONTRACTS.md](../CONTRACTS.md),
[decisions/001-go-for-agents.md](../decisions/001-go-for-agents.md) and
[research/nasiko.md](../research/nasiko.md) §6. The ADR prices your Task 1 and
§6 Finding B is why that task exists at all. Do not skip either.

---

## What this track owns

Turning raw mentions into meaning: sentiment, intent, aspects, topics and
share-of-voice. Plus the clustering algorithm itself, because Go has no
scikit-learn and nobody else is going to write it. Plus the only honest numbers
in the pitch, classifier accuracy and real cost per brand-day.

## What it must not know about

Where mentions came from, and what happens to topics downstream. You read
`Mention` and `EnrichedMention` and emit `Enrichment`, `Topic`, `ShareOfVoice`.
No Anakin call in this track, at all. `internal/cluster` goes further: it knows
about `[]string` and nothing else, no `models` import, no context, no I/O.

---

## Task 1: internal/cluster

This is first because everything in Task 4 depends on it and because an
algorithm proven on its own is worth three days of debugging it through an
agent. The ADR prices it at **240 to 320 lines**. It is the single largest
piece of real work the language choice cost us, and it is the demo's
centrepiece.

**Files:** `internal/cluster/tfidf.go`, `internal/cluster/agglomerative.go`,
`internal/cluster/stopwords.go`, `internal/cluster/tfidf_test.go`,
`internal/cluster/agglomerative_test.go`, `internal/cluster/CLAUDE.md`

Publish this surface first, before the bodies, so Task 4 compiles against it:

```go
// Package cluster is hand-rolled TF-IDF and average-linkage agglomerative
// clustering over short documents. It imports only the standard library.
package cluster

// Matrix is one corpus as L2-normalised TF-IDF row vectors. Rows[i] is the
// vector for docs[i] and keeps the caller's input order. Terms is the
// vocabulary in column order, sorted, so the column layout is reproducible.
type Matrix struct {
	Rows  [][]float64
	Terms []string
}

// TFIDF tokenises, drops stopwords, and builds the normalised term-frequency
// times inverse-document-frequency matrix for docs.
func TFIDF(docs []string) Matrix

// Agglomerative merges rows of m by average linkage until the closest pair of
// clusters exceeds cutoff cosine distance, then returns only the clusters with
// at least minSize members. Member indices index into m.Rows. Members are
// sorted ascending; clusters are sorted by size descending, ties broken by
// smallest member index. Two runs over the same Matrix return the identical
// slice of slices.
func Agglomerative(m Matrix, cutoff float64, minSize int) [][]int

// CosineDistance is 1 minus the dot product of two L2-normalised vectors.
func CosineDistance(a, b []float64) float64

// Tokenize lowercases, splits on non-letter non-digit runes, drops stopwords
// and tokens shorter than two runes.
func Tokenize(text string) []string
```

- [ ] `Tokenize`: lowercase, split on anything that is not a letter or a digit
      (`unicode.IsLetter`, `unicode.IsDigit`, so Devanagari survives), drop
      tokens of one rune, drop stopwords.
- [ ] `stopwords.go` carries **English plus common Hinglish**. Hinglish is the
      register Indian D2C discourse actually runs in, and without it every
      cluster centres on `hai`, `ka`, `nahi`, `yaar`. Seed list: `hai, hain,
      ka, ki, ke, ko, se, me, mein, aur, bhi, nahi, nahin, kya, kyun, yaar,
      bhai, toh, tha, thi, par, pe, wala, wali, ye, yeh, woh, vo, kar, karo,
      kaise, sab, bahut, bohot`. A package-level `map[string]struct{}`, built
      once in `init`, not rebuilt per call.
- [ ] `TFIDF`: document frequency over the corpus, `idf = log(N / (1 + df))`,
      then L2-normalise each row so cosine similarity is a plain dot product.
      Vocabulary is collected into a map and then **sorted** into `Terms`
      before any column index is assigned.
- [ ] **Determinism is the requirement, not a nicety.** Go randomises map
      iteration order on purpose. Every place this package touches a map, the
      keys get sorted before they influence output: the vocabulary, the
      document-frequency walk, the final cluster ordering. Two runs over the
      same corpus must produce the same topic list in the same order, or the
      demo is not reproducible and the eval is noise rather than a measurement.
      This is the trap in this package. Treat a `for k := range m` that feeds
      output as a bug.
- [ ] `Agglomerative`: start with one cluster per row, precompute the full
      pairwise cosine distance matrix, then repeatedly merge the closest pair
      by **average linkage** (mean distance across all member pairs) until the
      closest pair exceeds `cutoff`. At 400 mentions the matrix is 160,000
      floats, so the naive implementation is fine and a smarter one is not
      worth the reading cost. Say so in the file header rather than optimising.
- [ ] Ties in the closest-pair search resolve to the lowest `(i, j)` index
      pair. Floating point plus an arbitrary tie-break is the other way this
      package stops being deterministic.
- [ ] `minSize` filters at the end: clusters smaller than it are not returned.
      The caller derives noise as the complement of what came back, which is
      what `TopicSet.Unclustered` is.
- [ ] The cutoff constant lives in `bp-clusterer`, not here. This package takes
      it as an argument and has no opinion about the right value.
- [ ] **Test against a fixed toy corpus** where the right answer is obvious:
      fifteen short documents, three clearly separate topics (delivery
      complaints, price praise, packaging damage), at least two of them
      Hinglish. Assert three clusters and assert the exact membership.
- [ ] Determinism test: build the `Matrix` once, call `Agglomerative` twenty
      times, assert every result is deeply equal to the first. Run it with
      `-count=5` in the definition of done so a flake surfaces.
- [ ] Table-driven tests for `Tokenize` and `CosineDistance`. Stdlib `testing`
      only. No testify.
- [ ] Benchmark `TFIDF` and `Agglomerative` at 400 documents and paste the real
      `-benchmem` output in the PR. This is the hot path of the pipeline and
      the ADR's cost claim is unverified until there is a number next to it.
- [ ] `internal/cluster/CLAUDE.md`: what it owns, that it imports only the
      standard library, the determinism invariant, and that the cutoff is the
      caller's choice.
- [ ] Commit.

**Fallback if quality is poor:** group by shared matched keyword and top
aspect. It is worse and we would say so, but the contract does not change,
because `bp-clusterer` still returns a `TopicSet`. Do not spend the demo window
tuning the cutoff.

## Task 2: bp-sov

Still the first **agent** to ship. It is pure counting, has no LLM and no
network, and B5 deploys it first to prove the Nasiko path works end to end. It
only moved behind Task 1 because Task 4 cannot start until the clustering
exists.

**Files:** `agents/bp-sov/main.go`, `agents/bp-sov/handler.go`,
`agents/bp-sov/handler_test.go`, `agents/bp-sov/AgentCard.json`,
`agents/bp-sov/Dockerfile`

- [ ] `SOVHandler` with one method
      `Handle(ctx context.Context, in models.SOVInput) (models.ShareOfVoice, error)`,
      exactly per CONTRACTS §2 and §4. `main.go` is `package main` and its body
      is `obs.Setup` plus `a2a.Serve(card, SOVHandler{})` and nothing else.
- [ ] Construct the result with `models.NewShareOfVoice`, never a struct
      literal. A literal leaves `CompetitorShares` and `BySource` as nil maps
      and the first write panics.
- [ ] Counting rule: a mention counts for the brand when `IsAboutBrand` is true
      and `AboutCompetitor` is empty; for a competitor when `AboutCompetitor`
      matches a name in `Profile.Competitors`. Mentions matching neither are
      **excluded from the denominator**, not counted as brand mentions. Write
      that as a test, it is the easiest thing to get quietly wrong.
- [ ] Competitor matching is case-insensitive against `Profile.Competitors`. An
      `AboutCompetitor` value that matches nothing in the profile is an
      unknown, so it is excluded too, not silently added as a new competitor.
- [ ] `BySource` breaks the same percentages down per `models.Source`.
- [ ] Zero total mentions returns the zeroed struct with empty maps, not a
      division by zero producing `NaN`. `NaN` marshals as `json: unsupported
      value`, so this is a serialisation failure, not a cosmetic one. Test it.
- [ ] `AgentCard.json` with `protocolVersion: "1.0"` and no LLM provider. This
      agent makes zero LLM calls and the card should say so.
- [ ] Table-driven tests over hand-built `[]models.EnrichedMention`. No
      fixtures needed, so this task is not blocked on B2.
- [ ] `go vet ./agents/bp-sov/...` clean. Commit.

## Task 3: bp-enricher

**Files:** `agents/bp-enricher/main.go`, `agents/bp-enricher/handler.go`,
`agents/bp-enricher/batch.go`, `agents/bp-enricher/handler_test.go`,
`agents/bp-enricher/AgentCard.json`, `agents/bp-enricher/Dockerfile`,
`internal/prompts/enricher.md`

- [ ] `EnricherHandler.Handle(ctx context.Context, in models.EnrichInput) (models.EnrichmentBatch, error)`.
- [ ] `BatchSize` of 0 means 50, applied by the agent, per the pattern
      CONTRACTS §2 sets out under `bp-onboarder`. Do not add a `*int`.
- [ ] Batch at most 50 mentions per `llm.ChatJSON` call with a JSON schema
      constraining the output to a list of `Enrichment` objects. `openai-go`
      supports strict schema response formats, which is most of why the batch
      shape is safe.
- [ ] **Redact before the prompt.** Every `Mention.Text` goes through
      `redact.PII` first. Assert it in a test with a stub LLM that captures the
      prompt and fails if an email or an Indian phone number reaches it. This
      is a guardrail in PLAN.md, not a nicety, and it is not the model's job.
- [ ] Cache by `Mention.ContentHash` so a re-run over the same corpus costs
      zero tokens. That property is what makes the warm cost number in Task 7
      real. Test that a second `Handle` over the same input makes zero LLM
      calls and reports `CacheHits` equal to the input length.
- [ ] **Join on `MentionID`.** The order of `Enrichments` is not guaranteed and
      the contract says so explicitly. Never zip the response against the
      request by index. Write a test whose stub returns the batch reversed.
- [ ] The prompt must handle **Hinglish**. Roman-script Hindi mixed with
      English is the norm here, and a classifier that reads "bakwas product
      yaar" as neutral English is useless. Put Hinglish examples in
      `internal/prompts/enricher.md` and in the eval set.
- [ ] `IsAboutBrand` is the disambiguation field: "Mamaearth" the brand against
      "mama earth" in an unrelated sentence. Give the model
      `Profile.Products`, `Profile.Keywords` and `Profile.NegativeKeywords` as
      context so it can judge.
- [ ] Set `AboutCompetitor` when the mention is about a competitor instead.
- [ ] **Return `IsAboutBrand == false` mentions, do not drop them.** The
      clusterer and bp-sov filter them; the enricher's job is to label, not to
      delete. A dropped mention is a mention the eval cannot score.
- [ ] A malformed batch response retries once, then returns neutral
      enrichments for that batch with an entry in `Errors`. One bad batch must
      not fail a run. Per CONTRACTS §1, errors populate `Errors` and the task
      still succeeds.
- [ ] Record real `TokensUsed` and `CostPaise` from `llm.Usage`. The cost
      dashboard reads these; an invented number poisons the pitch.
- [ ] The prompt is `//go:embed`ed through `prompts.Load("enricher")` and never
      a string literal in `handler.go`. CONTRACTS §4.
- [ ] Tests use a stub `llm` call. No live call in CI, no key, no network.
- [ ] `go vet` clean. Commit.

## Task 4: bp-clusterer

**Files:** `agents/bp-clusterer/main.go`, `agents/bp-clusterer/handler.go`,
`agents/bp-clusterer/label.go`, `agents/bp-clusterer/handler_test.go`,
`agents/bp-clusterer/AgentCard.json`, `agents/bp-clusterer/Dockerfile`,
`internal/prompts/clusterer.md`

- [ ] `ClustererHandler.Handle(ctx context.Context, in models.ClusterInput) (models.TopicSet, error)`.
- [ ] Filter `IsAboutBrand == false` out before clustering. Those mentions go
      nowhere, they are not `Unclustered` noise.
- [ ] Vectorise with `cluster.TFIDF` over the surviving `Mention.Text` values,
      cluster with `cluster.Agglomerative`. The distance cutoff and the
      8-cluster cap are named constants at the top of `handler.go`, not
      scattered literals.
- [ ] **`llm.Embed` exists and this agent does not call it.** TF-IDF is enough
      at 400 mentions, costs zero tokens, needs no network, and runs free in
      CI. `research/nasiko.md` §6 Finding B is that no embeddings endpoint on
      the Nasiko router is verified at all. Do not add embeddings to hit a
      quality bar nobody has measured. If someone later confirms
      `/v1/embeddings` on a live cluster, that is a new task with a number
      attached, not a quiet swap.
- [ ] `MinClusterSize` of 0 means 3. Mentions in groups below it go to
      `Unclustered` as mention ids.
- [ ] **One LLM call per cluster** for `Label` and `Summary`. That is the part
      that genuinely needs a model. Cap at 8 clusters per window so a noisy day
      cannot fan out into 40 calls; clusters past the cap are dropped into
      `Unclustered` with a note in `Errors`. Sum the usage into
      `TopicSet.TokensUsed`.
- [ ] Build every topic with `models.NewTopic(ids.New("top"), brandID)`, then
      set fields. A `models.Topic{...}` literal gives you `Trend: 0`, a nil
      `SentimentMix` that panics on first write, and a `Validate()` failure.
- [ ] **`Trend` is the zero-value trap of this agent.** It is
      `size / PriorWindowCounts[label]`, defaulting to **1.0** when there is no
      prior window. In Go a missing map key returns 0, so the naive expression
      divides by zero and gives `+Inf`, which then fails to marshal. Use the
      comma-ok form:

      ```go
      trend := 1.0
      if prior, ok := in.PriorWindowCounts[t.Label]; ok && prior > 0 {
          trend = float64(size) / float64(prior)
      }
      ```

      `Topic.Validate()` rejects `Trend <= 0` because a zero trend means
      "nobody set the field", not "this topic vanished". Call `Validate()` on
      every topic before returning and write a test that a window with an empty
      `PriorWindowCounts` produces `Trend == 1.0` for every topic.
- [ ] `PriorWindowCounts` is keyed by **label**, and labels come from the LLM
      after clustering. A label that drifts between windows reads as a brand
      new topic with a flat trend. That is a known limitation of the contract,
      not a bug to fix here: keep the labelling prompt terse and noun-phrase
      shaped so drift is small, and note it in `HACKATHON_NOTES.md`.
- [ ] `TopExamples` holds at most 3 mentions, chosen by **`Engagement.Total()`**
      descending. Call the method, do not re-derive the sum: `Total()` excludes
      `Views` on purpose, and a 2M-view video would otherwise outrank every
      real complaint. Ties break on `PostedAt` so the choice is deterministic.
- [ ] `SentimentMix` counts `SentimentLabel` values across the cluster.
- [ ] Topic output order is whatever `cluster.Agglomerative` returned, which is
      size descending with a defined tie-break. Do not re-sort by a map walk.
- [ ] Test with the same shape as the Task 1 toy corpus but as
      `[]models.EnrichedMention`: fifteen mentions, three obvious topics,
      stubbed labelling. Assert three topics, assert the member counts, assert
      every `Trend` is 1.0, assert two consecutive runs return identical topic
      ids ordering.
- [ ] `internal/prompts/clusterer.md` loaded by `prompts.Load("clusterer")`.
- [ ] `go vet` clean. Commit.

## Task 5: label the eval set

**Files:** `fixtures/labelled/mentions.jsonl`

Blocked until B2 posts in `HACKATHON_NOTES.md` that fixtures are ready.

- [ ] Hand-label the set: `sentiment_label`, `intent`, `is_about_brand`. The
      path and the JSONL format are fixed by CONTRACTS §4.
- [ ] Label honestly, including the ones the model will get wrong. An eval set
      curated to make the model look good is worse than no eval set.
- [ ] Include Hinglish, sarcasm and brand-name collisions deliberately. Those
      are where a classifier for Indian D2C actually fails, and an eval that
      omits them measures nothing we care about.
- [ ] Commit.

## Task 6: eval/accuracy

**Files:** `eval/accuracy/main.go`, `eval/RESULTS.md`

`go run ./eval/accuracy` needs `package main` in its own directory, which is
why this is `eval/accuracy/main.go` and not `eval/accuracy.go`.

- [ ] Run `EnricherHandler` over the labelled set, report per-class precision,
      recall and F1 for sentiment and intent, accuracy for `is_about_brand`,
      plus a confusion matrix.
- [ ] Print a table to stdout and write `eval/RESULTS.md`.
- [ ] **Paste the real output into the PR.** If sentiment accuracy is 71%, the
      README says 71%. We do not round up and we do not quietly drop the
      hardest class from the report.
- [ ] Commit.

## Task 7: eval/cost

**Files:** `eval/cost/main.go`

This produces the number the entire pricing argument rests on.

- [ ] Measure a full brand-day: credits consumed per source, tokens per agent,
      and the rupee total. Read real values from `RunRecord.CreditsUsed`,
      `RunRecord.TokensUsed` and `Enrichment.CostPaise`, never from a constant.
- [ ] Report cold (no cache) and warm (cache hit) cost separately. The honest
      headline is the cold number.
- [ ] Compare against the ₹15 per brand-day target and against the ₹2,999 per
      month price. State the gross margin that falls out.
- [ ] If the real number exceeds ₹15, **say so** and list what would bring it
      down: fewer sources, bigger enrichment batches, longer cache buckets. Do
      not retune the target to match the result.
- [ ] Commit, and post the number in `HACKATHON_NOTES.md`. B5 needs it for
      `PRICING.md` and the pitch.

---

## Definition of done

```bash
go build ./... && go vet ./...

BP_FIXTURE_MODE=replay go test -v \
  ./internal/cluster/... \
  ./agents/bp-enricher/... \
  ./agents/bp-clusterer/... \
  ./agents/bp-sov/...

# determinism is the demo's reproducibility, so it gets its own run
go test ./internal/cluster/ -run Determinism -count=5

# the ADR's 240-320 line claim is unverified without a number next to it
go test ./internal/cluster/ -bench=. -benchmem

go run ./eval/accuracy
go run ./eval/cost
```

Every one of these runs clean, and their **real output** is pasted in the PR.
No claim of "passing" or "fast enough" without the output shown.
