"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";

export type AnnotationWire = {
  id: string;
  kind: string;
  body: string;
  start_off: number;
  end_off: number;
};

type Theme = "parchment" | "sepia" | "ink";
type Size = "s" | "m" | "l" | "xl";

const SIZE_PX: Record<Size, number> = { s: 16, m: 18, l: 20, xl: 23 };
const THEME_BG: Record<Theme, string> = {
  parchment: "#ECE5D3",
  sepia: "#E4D6BC",
  ink: "#171A14",
};
const THEME_FG: Record<Theme, string> = {
  parchment: "#171A14",
  sepia: "#2A2418",
  ink: "#E8DDC4",
};

function csrf(): string | null {
  const m = document.cookie.match(/(?:^|;\s*)alexandria_csrf=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : null;
}

/**
 * The reading surface: a typeset edition, not an ebook widget.
 *
 * Typography controls persist per reader in localStorage (they are a
 * preference, not data). Annotations anchor as (chapter_idx, start_off,
 * end_off) in offsets over the chapter string the API served, so a highlight
 * survives reflow, theme and font changes — the server defines the text, the
 * DOM merely renders it.
 */
export function ReaderChapter({
  editionId,
  chapterIndex,
  text,
  annotations,
  signedIn,
  licenseNote,
}: {
  editionId: string;
  chapterIndex: number;
  text: string;
  annotations: AnnotationWire[];
  signedIn: boolean;
  licenseNote: string;
}) {
  const [size, setSize] = useState<Size>("m");
  const [leading, setLeading] = useState(1.7);
  const [margin, setMargin] = useState(1);
  const [theme, setTheme] = useState<Theme>("parchment");
  const [note, setNote] = useState<string | null>(null);
  const [pending, setPending] = useState<{ start: number; end: number } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [refresh, setRefresh] = useState(0);
  const artRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    try {
      const raw = localStorage.getItem("alexandria-reader");
      if (raw) {
        const p = JSON.parse(raw);
        if (p.size) setSize(p.size);
        if (p.leading) setLeading(p.leading);
        if (p.margin !== undefined) setMargin(p.margin);
        if (p.theme) setTheme(p.theme);
      }
    } catch {
      /* preferences are best-effort */
    }
  }, []);
  useEffect(() => {
    localStorage.setItem("alexandria-reader", JSON.stringify({ size, leading, margin, theme }));
  }, [size, leading, margin, theme]);

  const paragraphs = useMemo(() => text.split("\n\n"), [text]);
  const paraStart = useMemo(() => {
    const starts: number[] = [];
    let acc = 0;
    for (const p of paragraphs) {
      starts.push(acc);
      acc += p.length + 2; // the "\n\n" the server's slice implies
    }
    return starts;
  }, [paragraphs]);

  function closestPara(node: Node): HTMLElement | null {
    const el = node.nodeType === Node.ELEMENT_NODE ? (node as HTMLElement) : node.parentElement;
    return el?.closest("p[data-idx]") ?? null;
  }

  function selectionOffsets(): { start: number; end: number } | null {
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed || !artRef.current) return null;
    const range = sel.getRangeAt(0);
    const startP = closestPara(range.startContainer);
    const endP = closestPara(range.endContainer);
    if (!startP || !endP) return null;
    const si = Number(startP.getAttribute("data-idx"));
    const ei = Number(endP.getAttribute("data-idx"));
    return {
      start: paraStart[si] + range.startOffset,
      end: paraStart[ei] + range.endOffset,
    };
  }

  async function saveAnnotation(kind: "highlight" | "note") {
    if (!pending) return;
    setError(null);
    const res = await fetch("/api/v1/me/annotations", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf() ?? "" },
      credentials: "same-origin",
      body: JSON.stringify({
        edition_id: editionId,
        chapter_idx: chapterIndex,
        start_off: pending.start,
        end_off: pending.end,
        kind,
        body: kind === "note" ? (note ?? "") : "",
      }),
    });
    if (!res.ok) {
      const j = await res.json().catch(() => ({}));
      setError(j?.error?.message ?? "Could not keep that margin note.");
      return;
    }
    setPending(null);
    setNote(null);
    setRefresh((r) => r + 1);
  }

  // Inline highlighting: split each paragraph around the annotations it owns.
  function renderParagraph(p: string, i: number) {
    const base = paraStart[i];
    const owned = annotations
      .filter((a) => a.start_off >= base && a.end_off <= base + p.length && a.start_off < a.end_off)
      .sort((a, b) => a.start_off - b.start_off);
    if (owned.length === 0) return <span>{p}</span>;
    const nodes: React.ReactNode[] = [];
    let cursor = 0;
    owned.forEach((a, k) => {
      const s = a.start_off - base;
      const e = a.end_off - base;
      if (s > cursor) nodes.push(<span key={`t${k}`}>{p.slice(cursor, s)}</span>);
      nodes.push(
        <mark
          key={`a${k}`}
          title={a.body || a.kind}
          style={{ background: "rgba(199,149,34,0.35)", color: "inherit" }}
        >
          {p.slice(s, e)}
        </mark>,
      );
      cursor = e;
    });
    if (cursor < p.length) nodes.push(<span key="tail">{p.slice(cursor)}</span>);
    return <>{nodes}</>;
  }

  return (
    <div>
      {/* ——— typography controls ——— */}
      <div className="panel flex flex-wrap items-center gap-x-5 gap-y-2 px-4 py-2.5" role="group" aria-label="Typography">
        <span className="engraved-label">Type</span>
        {(["s", "m", "l", "xl"] as Size[]).map((s) => (
          <button key={s} className={`btn-print !px-2 !py-0.5 !text-[0.72rem] ${size === s ? "!bg-ink !text-parchment-light" : ""}`} onClick={() => setSize(s)}>
            {s.toUpperCase()}
          </button>
        ))}
        <label className="engraved-label" htmlFor="leading">Leading</label>
        <input id="leading" type="range" min={1.4} max={2.2} step={0.05} value={leading}
          onChange={(e) => setLeading(Number(e.target.value))} className="w-24 accent-[#171A14]" />
        <label className="engraved-label" htmlFor="margin">Margin</label>
        <input id="margin" type="range" min={0} max={2} step={0.25} value={margin}
          onChange={(e) => setMargin(Number(e.target.value))} className="w-24 accent-[#171A14]" />
        {(["parchment", "sepia", "ink"] as Theme[]).map((t) => (
          <button key={t} className={`btn-print !px-2 !py-0.5 !text-[0.72rem] ${theme === t ? "!bg-ink !text-parchment-light" : ""}`} onClick={() => setTheme(t)}>
            {t}
          </button>
        ))}
      </div>

      {/* ——— the setting ——— */}
      <article
        ref={artRef as React.RefObject<HTMLElement>}
        onKeyUp={() => setPending(selectionOffsets())}
        onMouseUp={() => setPending(selectionOffsets())}
        className="mt-5 border px-6 py-8 transition-colors sm:px-10"
        style={{
          background: THEME_BG[theme],
          color: THEME_FG[theme],
          borderColor: theme === "ink" ? "#E8DDC4" : "#171A14",
          maxWidth: `${46 + margin * 14}ch`,
        }}
      >
        {paragraphs.map((p, i) => (
          <p
            key={i}
            data-idx={i}
            style={{ fontSize: SIZE_PX[size], lineHeight: leading, marginBottom: "0.9em" }}
            className={i === 0 ? "first-letter:float-left first-letter:mr-2 first-letter:font-display first-letter:text-[3.2em] first-letter:leading-[0.85] first-letter:text-vermilion" : ""}
          >
            {renderParagraph(p, i)}
          </p>
        ))}
      </article>

      {/* ——— margin actions ——— */}
      {pending ? (
        <div className="panel mt-4 px-4 py-3">
          <p className="engraved-label">
            Margin · offsets {pending.start}–{pending.end}
          </p>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <button className="btn-solid !py-1 !text-[0.8rem]" onClick={() => saveAnnotation("highlight")}>
              Keep as highlight
            </button>
            <input
              className="field !w-auto flex-1 min-w-[200px]"
              placeholder="…or write a note in the margin"
              value={note ?? ""}
              onChange={(e) => setNote(e.target.value)}
            />
            <button className="btn-print !py-1 !text-[0.8rem]" onClick={() => saveAnnotation("note")}>
              Keep as note
            </button>
            <button className="btn-print !py-1 !text-[0.8rem]" onClick={() => { setPending(null); setNote(null); }}>
              Discard
            </button>
          </div>
          {error ? <p className="mt-2 text-sm text-vermilion">{error}</p> : null}
          {!signedIn ? <p className="mt-2 text-xs">Sign in to keep margins — they are private by database law.</p> : null}
        </div>
      ) : null}

      {/* ——— the reader's margin, listed ——— */}
      {annotations.length ? (
        <section className="mt-7" aria-label="Your margin notes" key={refresh}>
          <h2 className="engraved-label">Your margin</h2>
          <ul className="mt-2 flex flex-col gap-2">
            {annotations.map((a) => (
              <li key={a.id} className="hairline py-2 text-sm">
                <span className="engraved-label mr-2">{a.kind}</span>
                <em>{text.slice(a.start_off, a.end_off).slice(0, 140)}</em>
                {a.body ? <span className="pullquote block">{a.body}</span> : null}
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      <p className="engraved-label mt-8 max-w-[70ch] leading-relaxed">{licenseNote}</p>
      <p className="mt-1 text-xs">
        <Link href="/" className="underline">Return to the Atrium</Link>
      </p>
    </div>
  );
}
