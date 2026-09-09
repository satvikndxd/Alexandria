"use client";

import { useState } from "react";

const REASONS = [
  "spam", "ai_slop", "harassment", "hate", "threat", "sexual_content",
  "misinformation", "copyright", "impersonation", "fake_review", "other",
] as const;

function csrf(): string | null {
  const m = document.cookie.match(/(?:^|;\s*)alexandria_csrf=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : null;
}

/**
 * Reporting is a human act with a human queue at the other end. The reason
 * vocabulary is the moderation enum itself, so a report routes without
 * translation; details are optional prose, and the reporter hears nothing
 * back except the eventual moderation rationale chain.
 */
export function ReportButton({ subjectType, subjectId }: { subjectType: string; subjectId: string }) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState<(typeof REASONS)[number]>("spam");
  const [details, setDetails] = useState("");
  const [done, setDone] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    setError(null);
    const res = await fetch("/api/v1/reports", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf() ?? "" },
      credentials: "same-origin",
      body: JSON.stringify({ subject_type: subjectType, subject_id: subjectId, reason, details }),
    });
    if (!res.ok) {
      const j = await res.json().catch(() => ({}));
      setError(j?.error?.message ?? "The report could not be filed.");
      return;
    }
    setDone(true);
  }

  if (done) return <span className="engraved-label !text-botanical">filed — a moderator will read it</span>;
  if (!open) {
    return (
      <button type="button" className="engraved-label underline underline-offset-2" onClick={() => setOpen(true)}>
        report
      </button>
    );
  }
  return (
    <span className="inline-flex flex-wrap items-center gap-2">
      <select className="field !w-auto !py-1 !text-[0.78rem]" value={reason} onChange={(e) => setReason(e.target.value as typeof reason)}>
        {REASONS.map((r) => (
          <option key={r} value={r}>{r.replace("_", " ")}</option>
        ))}
      </select>
      <input
        className="field !w-auto !py-1 !text-[0.78rem]"
        placeholder="details (optional)"
        value={details}
        onChange={(e) => setDetails(e.target.value)}
      />
      <button type="button" className="btn-print !px-2 !py-1 !text-[0.75rem]" onClick={submit}>
        file report
      </button>
      {error ? <span className="text-xs text-vermilion">{error}</span> : null}
    </span>
  );
}
