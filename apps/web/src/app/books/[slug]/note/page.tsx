"use client";

import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import Link from "next/link";
import { PageFrame } from "@/components/frame/PageFrame";

const KINDS = ["context", "linguistic", "historical", "interpretive", "textual"] as const;

function csrf(): string | null {
  const m = document.cookie.match(/(?:^|;\s*)alexandria_csrf=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : null;
}

type Citation = { citation: string; url: string };

/**
 * Note submission with the friction visible: 200-character minimum, required
 * citations, and the peer-review rule stated before submission rather than
 * discovered after it.
 */
export default function NewNotePage() {
  const params = useParams<{ slug: string }>();
  const router = useRouter();
  const [title, setTitle] = useState("");
  const [kind, setKind] = useState<(typeof KINDS)[number]>("context");
  const [chapterRef, setChapterRef] = useState("");
  const [anchor, setAnchor] = useState("");
  const [body, setBody] = useState("");
  const [citations, setCitations] = useState<Citation[]>([{ citation: "", url: "" }]);
  const [submitForReview, setSubmitForReview] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const runes = Array.from(body.trim()).length;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const res = await fetch(`/api/v1/works/${params.slug}/notes`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf() ?? "" },
      credentials: "same-origin",
      body: JSON.stringify({
        title, kind, chapter_ref: chapterRef, anchor_quote: anchor, body,
        citations: citations.filter((c) => c.citation.trim()),
        submit: submitForReview,
      }),
    });
    setBusy(false);
    if (!res.ok) {
      const j = await res.json().catch(() => ({}));
      setError(j?.error?.message ?? "The note could not be kept.");
      return;
    }
    const j = await res.json();
    router.push(`/scholar/notes/${j.note.id}`);
    router.refresh();
  }

  return (
    <PageFrame pathname="" epigraph="A note is a hand extended to the next reader." attribution="Alexandria">
      <p className="engraved-label">
        <Link href={`/books/${params.slug}`} className="underline">the work</Link> · scholar note
      </p>
      <h1 className="mt-1 text-[clamp(1.4rem,2.6vw,1.9rem)] text-ink">Submit a note</h1>
      <form className="panel mt-5 px-6 py-6" onSubmit={submit}>
        <label className="engraved-label" htmlFor="t">Title (3–200)</label>
        <input id="t" className="field mt-1" value={title} onChange={(e) => setTitle(e.target.value)} required />
        <div className="mt-4 flex flex-wrap gap-4">
          <div>
            <label className="engraved-label" htmlFor="k">Kind</label>
            <select id="k" className="field mt-1 !w-auto" value={kind} onChange={(e) => setKind(e.target.value as typeof kind)}>
              {KINDS.map((k) => <option key={k} value={k}>{k}</option>)}
            </select>
          </div>
          <div className="flex-1 min-w-[200px]">
            <label className="engraved-label" htmlFor="ch">Chapter / passage reference</label>
            <input id="ch" className="field mt-1" value={chapterRef} onChange={(e) => setChapterRef(e.target.value)} placeholder="Inferno, Canto I" />
          </div>
        </div>
        <label className="engraved-label mt-4 block" htmlFor="aq">Anchor quote (the passage the note explains)</label>
        <input id="aq" className="field mt-1" value={anchor} onChange={(e) => setAnchor(e.target.value)} />
        <label className="engraved-label mt-4 block" htmlFor="b">Note — minimum 200 characters</label>
        <textarea id="b" className="field mt-1 min-h-[180px] leading-relaxed" value={body} onChange={(e) => setBody(e.target.value)} />
        <p className={`mt-1 text-xs ${runes >= 200 ? "text-botanical" : "text-ink-faint"}`}>{runes} / 200 characters</p>

        <fieldset className="mt-5">
          <legend className="engraved-label">Citations (at least one; required for publication)</legend>
          {citations.map((c, i) => (
            <div key={i} className="mt-2 flex flex-wrap gap-2">
              <input
                className="field flex-1 min-w-[220px]"
                placeholder="Author, Title, Press, Year."
                value={c.citation}
                onChange={(e) => setCitations(citations.map((x, k) => (k === i ? { ...x, citation: e.target.value } : x)))}
              />
              <input
                className="field flex-1 min-w-[180px]"
                placeholder="https:// (optional)"
                value={c.url}
                onChange={(e) => setCitations(citations.map((x, k) => (k === i ? { ...x, url: e.target.value } : x)))}
              />
            </div>
          ))}
          <button type="button" className="btn-print mt-2 !py-1 !text-[0.78rem]"
            onClick={() => setCitations([...citations, { citation: "", url: "" }])}>
            Add citation
          </button>
        </fieldset>

        <label className="mt-5 flex items-center gap-2 text-sm text-ink-soft">
          <input type="checkbox" checked={submitForReview} onChange={(e) => setSubmitForReview(e.target.checked)} className="h-4 w-4 accent-[#171A14]" />
          Submit for peer review now (two distinct verified scholars must approve)
        </label>

        {error ? <p role="alert" className="mt-3 text-sm text-vermilion">{error}</p> : null}
        <div className="mt-5 flex gap-3">
          <button className="btn-solid" disabled={busy} type="submit">{busy ? "Sealing…" : "Keep the note"}</button>
          <Link href={`/books/${params.slug}`} className="btn-print">Cancel</Link>
        </div>
      </form>
    </PageFrame>
  );
}
