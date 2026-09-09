"use client";

import { useState } from "react";
import { PageFrame } from "@/components/frame/PageFrame";
import { RuleWithFleuron } from "@/components/Engravings";

function csrf(): string | null {
  const m = document.cookie.match(/(?:^|;\s*)alexandria_csrf=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : null;
}

/**
 * Scholar application. Verification is a human act — a moderator checks the
 * ORCID or institutional address — so this form only makes the application
 * legible: field, affiliation, ORCID, and a conflict-of-interest statement
 * that is displayed with every published note.
 */
export default function ScholarApplyPage() {
  const [field, setField] = useState("");
  const [affiliation, setAffiliation] = useState("");
  const [orcid, setOrcid] = useState("");
  const [coi, setCoi] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const res = await fetch("/api/v1/me/scholar-profile", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf() ?? "" },
      credentials: "same-origin",
      body: JSON.stringify({ field, affiliation, orcid, coi_statement: coi }),
    });
    setBusy(false);
    if (!res.ok) {
      const j = await res.json().catch(() => ({}));
      setError(j?.error?.message ?? "The application could not be filed.");
      return;
    }
    setDone(true);
  }

  return (
    <PageFrame pathname="" epigraph="Scholarship is a conversation across centuries." attribution="Alexandria">
      <h1 className="section-title !text-[1.7rem]">Apply for verification</h1>
      <RuleWithFleuron className="mt-4 max-w-[150px]" />
      <p className="pullquote mt-4 max-w-[64ch]">
        Verified scholars review each other's notes before publication. Community contributors
        write alongside you, labelled as such — different provenance, not lesser worth. An ORCID
        iD or institutional address speeds verification; neither is a substitute for a moderator's
        judgement.
      </p>
      {done ? (
        <p className="panel mt-6 px-5 py-4 pullquote">
          Filed. A moderator will review the application; your notes may be submitted as a
          community contributor in the meantime.
        </p>
      ) : (
        <form className="panel mt-6 max-w-xl px-6 py-6" onSubmit={submit}>
          <label className="engraved-label" htmlFor="f">Field</label>
          <input id="f" className="field mt-1" value={field} onChange={(e) => setField(e.target.value)} placeholder="Classical Literature" required />
          <label className="engraved-label mt-4 block" htmlFor="a">Affiliation (optional)</label>
          <input id="a" className="field mt-1" value={affiliation} onChange={(e) => setAffiliation(e.target.value)} placeholder="University, institute, or 'independent'" />
          <label className="engraved-label mt-4 block" htmlFor="o">ORCID iD (optional)</label>
          <input id="o" className="field mt-1" value={orcid} onChange={(e) => setOrcid(e.target.value)} placeholder="0000-0002-1825-0097" />
          <label className="engraved-label mt-4 block" htmlFor="c">Conflict-of-interest statement (required)</label>
          <textarea id="c" className="field mt-1 min-h-[90px]" value={coi} onChange={(e) => setCoi(e.target.value)} placeholder="‘none’, or disclose editions you have edited, presses you advise, …" required />
          {error ? <p role="alert" className="mt-3 text-sm text-vermilion">{error}</p> : null}
          <button className="btn-solid mt-5" disabled={busy} type="submit">
            {busy ? "Filing…" : "File application"}
          </button>
        </form>
      )}
    </PageFrame>
  );
}
