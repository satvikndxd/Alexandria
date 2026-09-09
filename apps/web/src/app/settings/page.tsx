"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { PageFrame } from "@/components/frame/PageFrame";
import { RuleWithFleuron } from "@/components/Engravings";

function csrf(): string | null {
  const m = document.cookie.match(/(?:^|;\s*)alexandria_csrf=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : null;
}

type Report = {
  shelved?: number;
  ratings_set?: number;
  reviews_created?: number;
  highlights_aligned?: number;
  skips?: { row: number; reason: string; detail: string }[];
};

/**
 * Portability: your history arrives with you, and leaves with you.
 * Imports are reported row by row — including what we refused and why —
 * because an importer that silently drops half your shelves is worse than
 * none. Export is one command; deletion is one command.
 */
export default function SettingsPage() {
  const [authed, setAuthed] = useState<boolean | null>(null);
  const [tab, setTab] = useState<"goodreads" | "storygraph" | "kindle">("goodreads");
  const [payload, setPayload] = useState("");
  const [busy, setBusy] = useState(false);
  const [report, setReport] = useState<Report | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);

  useEffect(() => {
    fetch("/api/v1/me", { credentials: "same-origin" })
      .then((r) => r.json())
      .then((j) => setAuthed(Boolean(j?.authenticated)))
      .catch(() => setAuthed(false));
  }, []);

  async function runImport() {
    setBusy(true);
    setError(null);
    setReport(null);
    const res = await fetch(`/api/v1/imports/${tab}`, {
      method: "POST",
      headers: { "X-CSRF-Token": csrf() ?? "" },
      credentials: "same-origin",
      body: payload,
    });
    setBusy(false);
    if (!res.ok) {
      const j = await res.json().catch(() => ({}));
      setError(j?.error?.message ?? "The import was refused.");
      return;
    }
    const j = await res.json();
    setReport(j.report as Report);
  }

  async function deleteAccount() {
    setBusy(true);
    const res = await fetch("/api/v1/account/delete", {
      method: "POST",
      headers: { "X-CSRF-Token": csrf() ?? "" },
      credentials: "same-origin",
      body: "{}",
    });
    setBusy(false);
    if (res.ok) window.location.href = "/";
  }

  if (authed === null) return <PageFrame pathname="" epigraph="." attribution="."><p className="pullquote">…</p></PageFrame>;
  if (!authed) {
    return (
      <PageFrame pathname="" epigraph="Carry your shelves with you." attribution="Alexandria">
        <h1 className="section-title !text-[1.7rem]">Settings & portability</h1>
        <p className="pullquote mt-4">Sign in to import your history, export your life, or close your account.</p>
        <Link href="/signin" className="btn-solid mt-5">Sign in</Link>
      </PageFrame>
    );
  }

  return (
    <PageFrame pathname="" epigraph="What is borrowed returns; what is yours follows." attribution="Alexandria">
      <h1 className="section-title !text-[1.7rem]">Settings & portability</h1>
      <RuleWithFleuron className="mt-4 max-w-[150px]" />

      <div role="tablist" aria-label="Import source" className="mt-6 flex gap-6 border-b border-ink/40">
        {(["goodreads", "storygraph", "kindle"] as const).map((t) => (
          <button key={t} role="tab" aria-selected={tab === t} className="tab-link" onClick={() => { setTab(t); setReport(null); setError(null); }}>
            {t === "goodreads" ? "Goodreads CSV" : t === "storygraph" ? "StoryGraph CSV" : "Kindle clippings"}
          </button>
        ))}
      </div>

      <div className="panel mt-5 px-6 py-6">
        <p className="pullquote max-w-[68ch]">
          {tab === "kindle"
            ? "Paste the contents of My Clippings.txt (or your notebook export). Highlights are aligned against our cached public-domain texts; clippings that cannot be anchored exactly are reported, never guessed. We never touch Amazon's clients or cloud."
            : "Paste the contents of your export CSV. Ratings without qualifying prose become private shelf ratings; imported prose publishes only if it clears the same 150-character bar as everything else. Unmatched books are reported, not invented."}
        </p>
        <textarea
          className="field mt-4 min-h-[180px] font-mono !text-[0.8rem]"
          value={payload}
          onChange={(e) => setPayload(e.target.value)}
          placeholder={tab === "kindle" ? "Title (Author)\n- Your Highlight at location … | Added on …\n\npassage\n==========" : "Book Id,Title,Author,ISBN13,My Rating,Exclusive Shelf,…"}
        />
        <div className="mt-4 flex flex-wrap gap-3">
          <button className="btn-solid" disabled={busy || !payload.trim()} onClick={runImport}>
            {busy ? "Reading your history…" : "Import"}
          </button>
          <span className="text-xs text-ink-faint self-center">Three imports per hour, by design.</span>
        </div>
        {error ? <p role="alert" className="mt-3 text-sm text-vermilion">{error}</p> : null}
        {report ? (
          <div className="mt-5 border-t border-ink/40 pt-4">
            <p className="engraved-label">Report</p>
            <p className="mt-1 text-sm text-ink-soft">
              shelved {report.shelved ?? 0} · ratings {report.ratings_set ?? 0} · reviews published {report.reviews_created ?? 0} ·
              highlights aligned {report.highlights_aligned ?? 0}
            </p>
            {report.skips?.length ? (
              <ul className="mt-2 flex flex-col gap-1">
                {report.skips.slice(0, 20).map((s, i) => (
                  <li key={i} className="text-xs text-ink-faint">row {s.row + 1}: {s.reason}{s.detail ? ` — ${s.detail}` : ""}</li>
                ))}
              </ul>
            ) : null}
          </div>
        ) : null}
      </div>

      <section className="mt-9">
        <h2 className="section-title">Your data</h2>
        <div className="mt-3 flex flex-wrap gap-3">
          <a className="btn-print" href="/api/v1/export">Download everything (JSON)</a>
          {!confirmDelete ? (
            <button className="btn-print !border-vermilion !text-vermilion" onClick={() => setConfirmDelete(true)}>
              Delete my account
            </button>
          ) : (
            <span className="flex flex-wrap items-center gap-3">
              <span className="text-sm text-vermilion">This removes you from the library. It cannot be undone here.</span>
              <button className="btn-solid !bg-vermilion !border-vermilion" disabled={busy} onClick={deleteAccount}>
                Yes, delete everything
              </button>
              <button className="btn-print" onClick={() => setConfirmDelete(false)}>Keep my account</button>
            </span>
          )}
        </div>
        <p className="mt-3 text-xs text-ink-faint">
          Deletion is immediate from your point of view; the rows are purged after the appeal window
          documented in the privacy model.
        </p>
      </section>
    </PageFrame>
  );
}
