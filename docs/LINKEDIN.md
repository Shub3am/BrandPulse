# LinkedIn post

For posting after the hackathon. Three drafts at different lengths, then the
rules that produced them.

**Do not post any of these until the numbers are real.** Every bracketed
placeholder is a blank that `go run ./eval/cost` fills. A post with an invented
figure in it is worse than no post, because it is permanent and searchable.

---

## Draft A, the one to post (about 180 words)

> A two-person skincare brand doing ₹40 lakh a month found out about a batch of
> leaking caps four days late, from a customer who called to ask why nobody had
> replied to her Reddit thread.
>
> The tools that would have caught it start around ₹77,000 a month on an annual
> enterprise contract. Sprinklr discontinued the only self-serve tier in the
> market this April. So brands that size use nothing.
>
> We built BrandPulse over a weekend: nine agents that read seven public
> sources, group mentions into topics, and fire alerts on five statistical
> rules. [COST] per brand-day.
>
> Three decisions I would defend outside a hackathon:
>
> The agent that decides whether something is a crisis makes no model call. It
> ships the numbers that fired it, so you can check its arithmetic at 11pm.
>
> It drafts replies and cannot post them. There is no posting code in the
> repository. Approve hands it to a human.
>
> Four of the eight sources the brief assumed turned out not to exist in the
> data catalogue. We shipped seven real ones and wrote down what we lost.
>
> Repo and the full source strategy in the comments.

---

## Draft B, short (about 70 words)

> Built BrandPulse this weekend: social listening for Indian D2C brands at
> [COST] per brand-day, against a ₹77,000/month floor from the incumbents.
>
> Nine A2A agents in Go. The one that decides what counts as a crisis makes no
> model call, because an alert you cannot audit at 11pm is not an alert.
>
> It drafts replies and cannot post them. That is a decision, not a gap.
>
> Repo in the comments.

---

## Draft C, the engineering angle (about 130 words)

> We ran Nasiko's CLI hard enough to find a spec bug in it, and sent the fix
> back: github.com/Nasiko-Labs/nasiko/pull/176
>
> A2A 1.0 moved `url`, `protocolVersion` and the transport out of the AgentCard
> root into `supportedInterfaces[]`, renaming the transport on the way. Their
> validator only read the root, so a card serialised by any current A2A SDK
> failed validation with a message naming three fields that were all present.
>
> The fix accepts either placement. The fallback mapping is not my reading of
> the spec, it is what a2a-go's own 1.0-to-0.2.x downgrade does, which is the
> difference between a patch a maintainer can merge and one they have to go
> verify.
>
> This came out of building BrandPulse: nine agents in Go, 14.6MB containers,
> [COST] per brand-day. The bug found us, not the other way around.

---

## What goes in the first comment

The repository link, plus one line:

> Source strategy, including the four sources the brief assumed that do not
> actually exist in the catalogue: <link to docs/SOURCE-STRATEGY.md>

LinkedIn suppresses posts with outbound links in the body. The link goes in the
first comment, posted by you, within a minute of the post.

---

## The rules these follow

**Lead with the person, not the stack.** The leaking caps and the four days are
the hook. Nobody outside this industry has an opinion about A2A.

**One number, and it has to be real.** `[COST]` is the only figure in drafts A
and B, and it stays a placeholder until `eval/cost` produces it. If the number
lands above the ₹15 target, post the real number and say so; the comparison
still works at ₹30 and the credibility is worth more than the roundness.

**Say what you did not build.** "It cannot post" is the line people reply to.
It signals that somebody thought about the failure mode, which is rarer in an
AI post than another feature list.

**Say what you lost.** The four missing sources belong in the post, not just in
the repository. Anyone who knows the catalogue will know, and hearing it from
you is what makes the other seven believable.

**No em dashes, no "excited to share", no rocket.** Write it the way you would
say it.

**No screenshots of a dashboard nobody can use.** If there is a visual, it is
the alert with its evidence numbers visible, because that is the thing that is
actually unusual.
