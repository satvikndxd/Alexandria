"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

function csrf(): string | null {
  const m = document.cookie.match(/(?:^|;\s*)alexandria_csrf=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : null;
}

/**
 * The acts a note permits: its author may retract (correction in public, never
 * silent deletion); a verified scholar who is not the author may approve or
 * reject with comments. The server re-checks every one of these.
 */
export function NoteActions({
  noteId,
  authorId,
  sessionId,
  status,
}: {
  noteId: string;
  authorId: string;
  sessionId: string;
  status: string;
}) {
  const router = useRouter();
  const [comments, setComments] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const isAuthor = authorId === sessionId;

  async function act(path: string, body: unknown) {
    setBusy(true);
    setError(null);
    const res = await fetch(`/api/v1/notes/${noteId}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf() ?? "" },
      credentials: "same-origin",
      body: JSON.stringify(body),
    });
    setBusy(false);
    if (!res.ok) {
      const j = await res.json().catch(() => ({}));
      setError(j?.error?.message ?? "The act was refused.");
      return;
    }
    router.refresh();
  }

  return (
    <div className="panel mt-8 px-5 py-4">
      {isAuthor ? (
        <div className="flex flex-wrap gap-3">
          {status === "draft" ? (
            <button className="btn-solid !py-1 !text-[0.82rem]" disabled={busy} onClick={() => act("/submit", {})}>
              Submit for review
            </button>
          ) : null}
          {status !== "retracted" ? (
            <button className="btn-print !py-1 !text-[0.82rem]" disabled={busy} onClick={() => act("/retract", {})}>
              Retract (keeps the record)
            </button>
          ) : null}
        </div>
      ) : (
        <div>
          <label className="engraved-label" htmlFor="rc">Review comments</label>
          <input id="rc" className="field mt-1" value={comments} onChange={(e) => setComments(e.target.value)} placeholder="Defend or contest the note in prose." />
          <div className="mt-3 flex flex-wrap gap-3">
            <button className="btn-solid !py-1 !text-[0.82rem]" disabled={busy} onClick={() => act("/review", { approved: true, comments })}>
              Approve
            </button>
            <button className="btn-print !py-1 !text-[0.82rem]" disabled={busy} onClick={() => act("/review", { approved: false, comments })}>
              Reject
            </button>
          </div>
          <p className="mt-2 text-xs text-ink-faint">
            Approval is a claim you can defend; two distinct verified approvals publish the note.
          </p>
        </div>
      )}
      {error ? <p role="alert" className="mt-2 text-sm text-vermilion">{error}</p> : null}
    </div>
  );
}
