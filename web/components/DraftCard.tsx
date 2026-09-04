// A suggested reply from bp-responder, awaiting a human.
//
// There is no send button and there will not be one. The approve action is a
// handoff to DronaHQ; nothing in this product posts to a platform, and the line
// that says so is part of the card rather than a footnote, because "nothing
// auto-posts" is the reason a brand owner trusts a drafted reply at all.
//
// The blocked phrases are chips, one per phrase. Joining them into a sentence
// is what crashed this page when the list arrived null.

import type { ReplyDraft } from "@/lib/types";
import { humanLabel } from "@/lib/format";
import { listOf, textOf } from "@/lib/wire";

// None of the three has a route in the BFF, so all three are disabled and say
// why. An enabled button that does nothing is a worse claim than a disabled one.
const NO_ROUTE = "Not wired up yet. The BFF has no route behind this action.";

function Guardrails({ phrases }: { phrases: string[] | null | undefined }) {
  const blocked = listOf(phrases)
    .map((phrase) => textOf(phrase))
    .filter((phrase): phrase is string => phrase !== null);

  return (
    <div className="guard">
      <div className="guard-title">Do not say, honoured while drafting</div>
      {blocked.length > 0 ? (
        <div className="chips">
          {blocked.map((phrase) => (
            <span className="guard-chip" key={phrase}>
              {phrase}
            </span>
          ))}
        </div>
      ) : (
        <p className="note" style={{ marginTop: 6 }}>
          This brand&rsquo;s voice carries no blocked phrases, so none were enforced on this
          draft.
        </p>
      )}
    </div>
  );
}

export function DraftCard({ draft }: { draft: ReplyDraft }) {
  const channel = textOf(draft.channel);
  const status = textOf(draft.status);
  const tone = textOf(draft.tone);

  return (
    <div className="card">
      <div className="draft-head">
        {channel && <span className="pill pill-accent">{humanLabel(channel)}</span>}
        {status && <span className="pill">{status}</span>}
        {tone && <span className="note push-right">tone: {tone}</span>}
      </div>

      <div className="draft-body">
        {textOf(draft.text) ?? "bp-responder returned this draft with no text."}
      </div>

      <Guardrails phrases={draft.do_not_say} />

      <div className="draft-actions">
        <button className="btn btn-primary" disabled title={NO_ROUTE}>
          Approve
        </button>
        <button className="btn" disabled title={NO_ROUTE}>
          Edit
        </button>
        <button className="btn" disabled title={NO_ROUTE}>
          Reject
        </button>
      </div>

      {draft.requires_human_approval && (
        <p className="no-post">
          <i className="dot" />
          Approval sends this to your phone to copy. BrandPulse never posts on your behalf.
        </p>
      )}
    </div>
  );
}

/** Every draft the payload carried, newest first as the BFF returned them. */
export function DraftPanel({ drafts }: { drafts: ReplyDraft[] }) {
  const rows = listOf(drafts);
  return (
    <div className="drafts">
      {rows.map((draft, index) => (
        <DraftCard key={textOf(draft.id) ?? `draft-${index}`} draft={draft} />
      ))}
    </div>
  );
}
