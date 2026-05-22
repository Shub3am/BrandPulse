# Global guardrails

Phrases no drafted reply may contain, whatever the brand's own voice says. The
brand's `do_not_say` list is layered on top of this one; this list is the floor.

Every bullet below is a **literal phrase**, lowercased. `prompts.Guardrails`
parses the bullets and nothing else, so the headings and this prose are free to
explain why each group exists. The responder matches case-insensitively, so
write the phrase once in lower case.

Keep the phrases short. A long phrase is a phrase a model varies its way past.

## No refund or compensation promised

A refund is a commercial decision a support team makes per order, not something
a social reply commits the company to across every reader of the thread.

- refund everyone
- full refund
- refund you immediately
- compensate everyone
- free replacement for all

## No admission of fault or liability

An admission in public writing is evidence. The holding statement acknowledges
the experience, not the blame.

- we are at fault
- this is our fault
- we were negligent
- we admit liability
- we broke the law
- we are responsible for the damage

## No medical or safety claims

Regulated language. A wrong claim here is a regulatory problem, not a PR one.

- clinically proven
- completely safe
- no side effects
- cures
- doctor recommended
- guaranteed safe

## No naming an individual employee

Naming a person turns a product complaint into a pile-on directed at someone
who cannot answer back.

- our employee
- the staff member responsible
- was fired
- has been terminated

## No commitment to a date

A date given under pressure is a date the founder has not checked with anyone.

- by tomorrow
- within 24 hours
- fixed by monday
- shipping this week
- guaranteed delivery by

## No legal characterisation

Calling something defamation, fraud or negligence in public is a legal position
taken by a social account rather than by a lawyer.

- defamation
- fraudulent
- legal action against
- we will sue

Editing this file changes the guardrails. It is a recompile, not a config
reload: the loader reads the embedded copy, not the file on disk.
