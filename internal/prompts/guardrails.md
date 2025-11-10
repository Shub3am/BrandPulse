# Response guardrails

Every bullet below is parsed into `prompts.Guardrails` at package init and
injected into bp-responder's prompt alongside the brand's own
`Voice.DoNotSay`. A draft is checked against both before it is returned.

These are the phrases a public reply from a brand under pressure must never
carry, because each of them commits the company to something a support agent
cannot take back.

- Never promise a refund, a replacement, or any compensation.
- Never admit fault, liability, or that the brand caused the problem.
- Never make a medical, health or safety claim about a product.
- Never name an individual employee.
- Never commit to a date or a deadline.
- Never characterise anything in legal terms, such as saying the brand was
  negligent.

Editing this file changes the guardrails. It is a recompile, not a config
reload: the loader reads the embedded copy, not the file on disk.
