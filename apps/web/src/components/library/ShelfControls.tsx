"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

const KINDS = [
  { kind: "want_to_read", label: "Want to Read" },
  { kind: "reading", label: "Currently Reading" },
  { kind: "read", label: "Read" },
  { kind: "dnf", label: "Did Not Finish" },
  { kind: "favorites", label: "Favorites" },
] as const;

function csrfFromCookie(): string | null {
  const m = document.cookie.match(/(?:^|;\s*)alexandria_csrf=([^;]+)/);
  return m ? decodeURIComponent(m[1]) : null;
}

/**
 * Shelf controls: the reader's own hands on their library. Mutations go to
 * the monolith with the CSRF double-submit token; the server's Row-Level
 * Security is what actually guarantees the shelf is theirs.
 */
export function ShelfControls({
  workSlug,
  current,
  compact = false,
}: {
  workSlug: string;
  current?: string | null;
  compact?: boolean;
}) {
  const router = useRouter();
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function move(kind: string) {
    setBusy(kind);
    setError(null);
    try {
      const res = await fetch("/api/v1/me/library", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": csrfFromCookie() ?? "",
        },
        credentials: "same-origin",
        body: JSON.stringify({ work_slug: workSlug, shelf_kind: kind }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body?.error?.message ?? `The library refused that (${res.status}).`);
      }
      router.refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Something broke; try again.");
    } finally {
      setBusy(null);
    }
  }

  return (
    <div>
      <ul className={`flex flex-wrap ${compact ? "gap-1.5" : "gap-2"}`} aria-label="Shelve this book">
        {KINDS.map(({ kind, label }) => (
          <li key={kind}>
            <button
              type="button"
              disabled={busy !== null}
              onClick={() => move(kind)}
              className={`${compact ? "btn-print !px-2.5 !py-1 !text-[0.75rem]" : "btn-print !py-1.5 !text-[0.82rem]"} ${
                current === kind ? "!bg-ink !text-parchment-light" : ""
              }`}
              aria-pressed={current === kind}
            >
              {busy === kind ? "…" : label}
            </button>
          </li>
        ))}
      </ul>
      {error ? <p className="mt-2 text-sm text-vermilion">{error}</p> : null}
    </div>
  );
}
