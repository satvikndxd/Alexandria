import Link from "next/link";
import { PortraitMedallion } from "@/components/Engravings";
import { relativeWhen, type Activity } from "@/lib/content";

/**
 * Recent Activity: a chronological stream of what followed readers and the
 * wider community actually did. Reverse-chronological, capped, and never
 * ranked — the absence of a scoring function is the feature.
 */
export function ActivityStream({ items }: { items: Activity[] }) {
  return (
    <section aria-labelledby="activity-heading" className="mt-10">
      <h2 id="activity-heading" className="section-title">
        Recent Activity
      </h2>
      <ul className="mt-4">
        {items.map((a, i) => (
          <li key={`${a.actor}-${a.workSlug}-${i}`} className="hairline flex gap-4 py-4 first:border-t-0">
            <span className="medallion h-11 w-11 shrink-0 text-ink" aria-hidden>
              <PortraitMedallion className="h-11 w-11" />
            </span>
            <div className="min-w-0 flex-1">
              <p className="text-[0.95rem] text-ink">
                <span className="font-semibold">{a.actor}</span>{" "}
                {a.kind === "shelf_add" ? "shelved" : a.kind === "note" ? "posted a note on" : "reviewed"}{" "}
                <Link href={`/books/${a.workSlug}`} className="italic underline decoration-ink/40 underline-offset-2 hover:decoration-ink">
                  {a.workTitle}
                </Link>
              </p>
              {a.excerpt ? <p className="pullquote mt-1 line-clamp-2">“{a.excerpt}”</p> : null}
            </div>
            <span className="shrink-0 text-xs text-ink-faint">{relativeWhen(a.when)}</span>
          </li>
        ))}
      </ul>
      {items.length === 0 ? (
        <p className="pullquote mt-3">
          Quiet so far. Follow a few readers and their reviews and shelvings will appear here, in
          the order they happened.
        </p>
      ) : null}
      {items.some((i) => i.isSeed) ? (
        <p className="engraved-label mt-4">Demonstration seeds — no live activity yet on this instance.</p>
      ) : null}
    </section>
  );
}
