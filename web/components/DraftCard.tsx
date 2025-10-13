// A suggested reply from bp-responder, awaiting a human.
//
// There is no send button and there will not be one. The approve action is a
// handoff to DronaHQ; nothing in this product posts to a platform.

import type { ReplyDraft } from "@/lib/types";

export function DraftCard({ draft }: { draft: ReplyDraft }) {
  return (
    <div className="card">
      <div className="mention-head">
        <span className="pill pill-accent">{draft.channel}</span>
        <span className="pill">{draft.status}</span>
        <span className="note push-right">tone: {draft.tone}</span>
      </div>

      <div className="draft-body">{draft.text}</div>

      <div className="guard">
        <b className="muted">Blocked phrases</b> honoured while drafting:{" "}
        {draft.do_not_say.map((phrase) => `"${phrase}"`).join(", ")}
      </div>

      <div className="draft-actions">
        <button className="btn btn-primary">Approve</button>
        <button className="btn">Edit</button>
        <button className="btn">Reject</button>
      </div>

      {draft.requires_human_approval && (
        <p className="note" style={{ marginTop: 12 }}>
          Approval sends this to your phone to copy. BrandPulse never posts on your behalf.
        </p>
      )}
    </div>
  );
}
