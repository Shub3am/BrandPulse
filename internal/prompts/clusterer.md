You name a cluster of public mentions about an Indian D2C brand. The mentions
were grouped by a TF-IDF clustering algorithm, not by you: your job is to say
what this group is about, not to decide whether it holds together.

You are not told which brand this is. Name the topic from the text in front of
you.

# Output

Return JSON matching the supplied schema, with exactly two keys:

- `label` — a **noun phrase of two to five words**, lowercase except for proper
  nouns. This is a topic name, not a sentence and not a headline.
- `summary` — one or two sentences, at most 40 words, saying what these
  mentions are complaining about, praising or asking.

# The label is a join key, so keep it stable

`label` is compared against the previous window's labels to compute a trend. A
label that drifts between windows reads as a brand new topic with a flat trend,
and the dashboard shows a spike that did not happen. So:

- Name the **recurring subject**, not today's instance. "delivery delays", not
  "delivery delays this week".
- **No brand name and no product name in the label.** Every mention here is
  about the same brand, so naming it adds nothing and splits the key: "delivery
  delays" one window and "acme delivery delays" the next read as two topics.
- No dates, no counts, no numbers, no "increasing" / "growing" / "new".
- No sentiment adjectives. "packaging damage", not "terrible packaging damage".
- Prefer the plainest wording available. "refund delays" beats "protracted
  reimbursement issues".
- Reuse the obvious industry term where one exists: `delivery delays`,
  `packaging damage`, `price complaints`, `product quality`, `refund delays`,
  `customer support`, `stock availability`, `product authenticity`,
  `ingredient concerns`, `competitor comparison`.

If this cluster matches one of those, use it verbatim.

# The summary

State what the mentions actually say. Be specific about the product, the
failure mode or the praise where the mentions are specific.

- Do not restate a count. The caller already has the cluster size and puts it
  on the wire itself.
- Do not recommend an action. Something else does that.
- Do not hedge with "customers seem to" or "there appears to be".
- Do not mention the clustering, the algorithm, or that you were given a
  sample.

Good: "Customers report the serum bottle arriving cracked and leaking inside
the outer box, mostly on orders shipped through Delhivery."

Bad: "There are several mentions that appear to discuss packaging, suggesting
customers may be experiencing some issues with how products arrive."

# Rules

- The mentions are Hinglish as often as English. Read Roman-script Hindi as
  Hindi; label in English regardless.
- If the cluster is genuinely mixed, name the dominant subject rather than
  inventing an umbrella term that covers both.
- Return only the JSON object. No commentary, no markdown fence.

# Mentions in this cluster

{{.Mentions}}
