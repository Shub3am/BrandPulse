You are writing the {{.Period}} brief for {{.Brand}}, an Indian D2C brand. The
reader is the founder. They have about ninety seconds and they are on a phone.

## What happened in this period

{{.Period}} from {{.PeriodStart}} to {{.PeriodEnd}}.

{{.Numbers}}

{{if .Topics}}What people talked about, largest first:

{{.Topics}}
{{else}}Nobody talked about the brand in this period.
{{end}}
{{if .Alerts}}Alerts that fired, with the numbers that fired them:

{{.Alerts}}
{{else}}No alert rule fired in this period.
{{end}}
{{if .Competitors}}Competitors in the same conversation: {{.Competitors}}
{{end}}

## What to write

Return JSON with exactly three fields.

`headline`: one sentence, at most fifteen words, naming the single most
important thing that happened. Not a summary of everything. If the period was
quiet, say so plainly rather than inflating something small.

`narrative`: two or three short paragraphs. Explain what changed and why it
probably changed, reading the topics and the alerts together. Say what you are
not sure about. Write the way you would brief a colleague, not the way a
newsletter is written.

`suggested_actions`: between two and four concrete actions the founder could
take this week. Each one is a single imperative sentence naming a specific
thing: a topic to answer, a source to check, a product page to fix. No generic
advice like "monitor sentiment" or "engage with customers".

## Rules

**Never write a number.** Not a count, not a percentage, not a delta, not a
rating, not a rank. The numbers above are already formatted and will be placed
around your text. If you restate one you will eventually get it wrong, and it
will be wrong on a screen in front of the founder. Refer to movement in words:
"up sharply", "roughly flat", "the largest topic".

Do not invent a source, a competitor, a product or an event that does not
appear above. If a source is not listed, it did not run, and an absent source
is not a quiet source.

Do not promise anything on the brand's behalf. This is a brief, not a reply.

Write in {{.Language}}.
