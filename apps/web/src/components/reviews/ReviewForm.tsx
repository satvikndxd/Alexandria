"use client";

import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import Link from "next/link";

const MIN_CHARS = 150;

/**
 * The review form is where Alexandria's friction becomes visible instead of
 * hidden: the counter, the minimum, and the "why" prompt are all on the page,
 * explained, before submission. Friction you can see is respect; friction you
 * cannot is a trap.
 */
export function ReviewForm({ slug, title }: { slug: string; title: string }) {
  const router = useRouter();
  const [halfStars, setHalfStars] = useState(0); // 1..10
  const [reviewTitle, setReviewTitle] = useState("");
  const [body, setBody] = useState("");
  const [why, setWhy] = useState("");
  const [spoilers, setSpoilers] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const runes = useMemo(() => Array.from(body.trim()).length, [body]);
  const distinctWords = useMemo(
    () =>
      new Set(
        body
          .toLowerCase()
          .replace(/[.,;:!?()"''\-–—]/g, " ")
          .split(/\s+/)
          .filter(Boolean),
      ).size,
    [body],
  );
  const lengthOk = runes >= MIN_CHARS && runes <= 20000;
  const fillerRisk = lengthOk && distinctWords < 12;

  function csrf(): string | null {
    const m = document.cookie.match(/(?:^|;\s*)alexandria_csrf=([^;]+)/);
    return m ? decodeURIComponent(m[1]) : null;
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/v1/works/${slug}/reviews`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf() ?? "" },
        credentials: "same-origin",
        body: JSON.stringify({
          rating: halfStars / 2,
          title: reviewTitle,
          body,
          prompt_why: why,
          has_spoilers: spoilers,
        }),
      });
      const json = await res.json().catch(() => ({}));
      if (!res.ok) {
        setError(json?.error?.message ?? `The archive refused that (${res.status}).`);
        return;
      }
      router.push(`/books/${slug}`);
      router.refresh();
    } catch {
      setError("The archive is momentarily unreachable. Your words are still here — try again.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="panel mt-6 px-6 py-6 sm:px-8">
      <fieldset>
        <legend className="engraved-label">Your rating, in half-stars</legend>
        <div className="mt-2 flex flex-wrap gap-1.5">
          {Array.from({ length: 10 }, (_, i) => i + 1).map((h) => (
            <button
              key={h}
              type="button"
              aria-pressed={halfStars === h}
              aria-label={`${(h / 2).toFixed(1)} stars`}
              onClick={() => setHalfStars(h)}
              className={`border-[1.5px] border-ink px-2 py-1 oldstyle text-sm ${
                halfStars === h ? "bg-ink text-parchment-light" : "hover:bg-ink/10"
              }`}
            >
              {(h / 2).toFixed(1)}
            </button>
          ))}
        </div>
        {halfStars === 0 ? <p className="mt-2 text-xs text-vermilion">A review requires a rating.</p> : null}
      </fieldset>

      <label className="engraved-label mt-6 block" htmlFor="rtitle">
        Title (optional, ≤ 200)
      </label>
      <input
        id="rtitle"
        className="field mt-1"
        value={reviewTitle}
        maxLength={200}
        onChange={(e) => setReviewTitle(e.target.value)}
        placeholder="A line you would put in a margin"
      />

      <label className="engraved-label mt-5 block" htmlFor="rbody">
        Your review — minimum {MIN_CHARS} characters
      </label>
      <textarea
        id="rbody"
        className="field mt-1 min-h-[180px] leading-relaxed"
        value={body}
        onChange={(e) => setBody(e.target.value)}
        placeholder="What did the book do, and what did it leave undone? Quote it. Argue with it."
      />
      <p className={`mt-1 text-xs ${lengthOk ? "text-botanical" : "text-ink-faint"}`} aria-live="polite">
        {runes} / {MIN_CHARS} characters
        {fillerRisk ? " · this reads as repetition — vary your words or quote the book" : ""}
      </p>

      <label className="engraved-label mt-5 block" htmlFor="rwhy">
        Why this rating? (the prompt that keeps hot takes honest)
      </label>
      <input
        id="rwhy"
        className="field mt-1"
        value={why}
        onChange={(e) => setWhy(e.target.value)}
        placeholder="One sentence is enough."
      />

      <label className="mt-5 flex items-center gap-2 text-sm text-ink-soft">
        <input
          type="checkbox"
          checked={spoilers}
          onChange={(e) => setSpoilers(e.target.checked)}
          className="h-4 w-4 accent-[#171A14]"
        />
        This review discusses endings or turns (readers will see it veiled until they choose otherwise)
      </label>

      {error ? (
        <p role="alert" className="mt-4 border-l-2 border-vermilion pl-3 text-sm text-vermilion">
          {error}
        </p>
      ) : null}

      <div className="mt-6 flex flex-wrap items-center gap-3">
        <button className="btn-solid" disabled={busy || halfStars === 0 || !lengthOk} type="submit">
          {busy ? "Sealing…" : "Publish review"}
        </button>
        <Link href={`/books/${slug}`} className="btn-print">
          Cancel
        </Link>
        <p className="text-xs text-ink-faint">
          One review per book; rereading revises it. New accounts: two reviews a day.
        </p>
      </div>
    </form>
  );
}
