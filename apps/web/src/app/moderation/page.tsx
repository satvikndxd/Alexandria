"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { PageFrame } from "@/components/frame/PageFrame";
import { RuleWithFleuron } from "@/components/Engravings";

function csrf(): string | null {
  const m = document.cookie.match(/(?:^|;\s*)alexandria_csrf=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : null;
}

type Report = {
  id: string;
  subject_type: string;
  subject_id: string;
  reason: string;
  details: string;
  status: string;
  reporter_username?: string | null;
  created_at: string;
};
type Pending = { user_id: string; username: string; email: string; field: string; affiliation?: string | null; orcid?: string | null; coi_statement: string };

const ACTIONS = ["warning", "content_removal", "temp_suspension", "permanent_ban", "restore", "note"] as const;

/**
 * The human moderation queue. Every action demands a written rationale — the
 * schema enforces it, and this form makes it the centre of the act rather
 * than an afterthought: a moderation decision that cannot be explained to the
 * person it affects does not get made here.
 */
export default function ModerationPage() {
  const [role, setRole] = useState<string | null>(null);
  const [reports, setReports] = useState<Report[]>([]);
  const [pending, setPending] = useState<Pending[]>([]);
  const [tab, setTab] = useState<"reports" | "scholars">("reports");
  const [rationale, setRationale] = useState<Record<string, string>>({});
  const [kind, setKind] = useState<Record<string, (typeof ACTIONS)[number]>>({});
  const [hours, setHours] = useState<Record<string, number>>({});
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    const me = await fetch("/api/v1/me", { credentials: "same-origin" }).then((r) => r.json()).catch(() => null);
    if (!me?.authenticated) { setRole("anonymous"); return; }
    setRole(me.user?.role ?? "reader");
    if (me.user?.role !== "moderator" && me.user?.role !== "admin") return;
    const [r, s] = await Promise.all([
      fetch("/api/v1/moderation/reports?limit=50", { credentials: "same-origin" }).then((x) => x.json()).catch(() => null),
      fetch("/api/v1/moderation/scholars?limit=50", { credentials: "same-origin" }).then((x) => x.json()).catch(() => null),
    ]);
    setReports(r?.reports ?? []);
    setPending(s?.pending ?? []);
  }, []);

  useEffect(() => { void load(); }, [load]);

  async function act(reportId: string, targetUser?: string) {
    setError(null);
    const res = await fetch(`/api/v1/moderation/reports/${reportId}/action`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf() ?? "" },
      credentials: "same-origin",
      body: JSON.stringify({
        kind: kind[reportId] ?? "warning",
        rationale: rationale[reportId] ?? "",
        suspend_hours: (kind[reportId] ?? "warning") === "temp_suspension" ? (hours[reportId] ?? 24) : undefined,
      }),
    });
    if (!res.ok) {
      const j = await res.json().catch(() => ({}));
      setError(j?.error?.message ?? "The action was refused.");
      return;
    }
    void load();
    void targetUser;
  }

  async function decideScholar(userId: string, status: "verified" | "rejected") {
    setError(null);
    const res = await fetch(`/api/v1/moderation/scholars/${userId}`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf() ?? "" },
      credentials: "same-origin",
      body: JSON.stringify({ status }),
    });
    if (!res.ok) {
      const j = await res.json().catch(() => ({}));
      setError(j?.error?.message ?? "The decision was refused.");
      return;
    }
    void load();
  }

  if (role === null) return <PageFrame pathname="" epigraph="." attribution="."><p className="pullquote">…</p></PageFrame>;
  if (role === "anonymous" || (role !== "moderator" && role !== "admin")) {
    return (
      <PageFrame pathname="" epigraph="Moderation is a duty, not a power." attribution="Alexandria">
        <h1 className="section-title !text-[1.7rem]">Moderation</h1>
        <p className="pullquote mt-4">The queue is for appointed moderators. Reports filed by readers arrive here.</p>
      </PageFrame>
    );
  }

  return (
    <PageFrame pathname="" epigraph="Moderation is a duty, not a power." attribution="Alexandria">
      <h1 className="section-title !text-[1.7rem]">Moderation queue</h1>
      <RuleWithFleuron className="mt-4 max-w-[150px]" />
      <div role="tablist" className="mt-5 flex gap-6 border-b border-ink/40">
        <button role="tab" aria-selected={tab === "reports"} className="tab-link" onClick={() => setTab("reports")}>
          reports · {reports.length}
        </button>
        <button role="tab" aria-selected={tab === "scholars"} className="tab-link" onClick={() => setTab("scholars")}>
          scholar applications · {pending.length}
        </button>
      </div>

      {error ? <p role="alert" className="mt-3 text-sm text-vermilion">{error}</p> : null}

      {tab === "reports" ? (
        reports.length === 0 ? (
          <p className="pullquote mt-5">The queue is empty. A quiet day is a good day.</p>
        ) : (
          <ul className="mt-5 flex flex-col gap-6">
            {reports.map((r) => (
              <li key={r.id} className="panel px-5 py-4">
                <p className="engraved-label">
                  {r.reason.replace(/_/g, " ")} · {r.subject_type} · {r.status}
                </p>
                <p className="mt-1 text-sm text-ink-soft">
                  subject <code className="text-xs">{r.subject_id}</code>
                  {r.details ? <> — “{r.details}”</> : null}
                </p>
                <div className="mt-3 flex flex-wrap items-center gap-2">
                  <select
                    className="field !w-auto !py-1 !text-[0.78rem]"
                    value={kind[r.id] ?? "warning"}
                    onChange={(e) => setKind({ ...kind, [r.id]: e.target.value as (typeof ACTIONS)[number] })}
                  >
                    {ACTIONS.map((a) => <option key={a} value={a}>{a.replace(/_/g, " ")}</option>)}
                  </select>
                  {(kind[r.id] ?? "warning") === "temp_suspension" ? (
                    <label className="flex items-center gap-1 text-xs text-ink-faint">
                      hours
                      <input
                        type="number" min={1} max={8760} className="field !w-20 !py-1 !text-[0.78rem]"
                        value={hours[r.id] ?? 24}
                        onChange={(e) => setHours({ ...hours, [r.id]: Number(e.target.value) })}
                      />
                    </label>
                  ) : null}
                  <input
                    className="field flex-1 min-w-[220px] !py-1 !text-[0.8rem]"
                    placeholder="written rationale — required, ≥10 characters, shown in the audit trail"
                    value={rationale[r.id] ?? ""}
                    onChange={(e) => setRationale({ ...rationale, [r.id]: e.target.value })}
                  />
                  <button className="btn-solid !py-1 !text-[0.78rem]" onClick={() => act(r.id)}>
                    record action
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )
      ) : pending.length === 0 ? (
        <p className="pullquote mt-5">No applications awaiting verification.</p>
      ) : (
        <ul className="mt-5 flex flex-col gap-5">
          {pending.map((p) => (
            <li key={p.user_id} className="panel px-5 py-4">
              <p className="text-[1.02rem] font-semibold text-ink">{p.username} <span className="text-sm font-normal text-ink-faint">· {p.field}</span></p>
              <p className="mt-1 text-sm text-ink-soft">
                {p.affiliation ?? "independent"}{p.orcid ? ` · ORCID ${p.orcid}` : ""}
              </p>
              <p className="pullquote mt-2">COI: {p.coi_statement}</p>
              <div className="mt-3 flex gap-2">
                <button className="btn-solid !py-1 !text-[0.78rem]" onClick={() => decideScholar(p.user_id, "verified")}>verify</button>
                <button className="btn-print !py-1 !text-[0.78rem]" onClick={() => decideScholar(p.user_id, "rejected")}>reject</button>
              </div>
            </li>
          ))}
        </ul>
      )}
      <p className="mt-8 text-xs text-ink-faint">
        Actions are recorded with your id and rationale; suspensions expire on their own.{" "}
        <Link href="/notifications" className="underline">back to alerts</Link>
      </p>
    </PageFrame>
  );
}
