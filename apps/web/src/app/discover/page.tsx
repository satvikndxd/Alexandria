import type { Metadata } from "next";
import { works } from "@/lib/data";
import { SectionHeading } from "@/components/Ornament";
import { BookCard } from "@/components/BookCard";

export const metadata: Metadata = { title: "Discover" };

export default function DiscoverPage() {
  const subjects = Array.from(new Set(works.flatMap((w) => w.subjects))).sort();

  return (
    <div className="space-y-10">
      <SectionHeading caption="No algorithmic sludge — curation and chronology" title="Discover" />

      <section aria-label="Browse by subject">
        <p className="engraved-label mb-3">Browse by subject</p>
        <ul className="flex flex-wrap gap-2">
          {subjects.map((s) => (
            <li key={s}>
              <span className="inline-block border border-ink px-3 py-1 text-sm text-ink-soft hover:bg-ink hover:text-parchment-light">
                {s}
              </span>
            </li>
          ))}
        </ul>
      </section>

      <section aria-label="The collection">
        <p className="engraved-label mb-4">The collection · ranked by reader ratings, never by engagement</p>
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          {[...works]
            .sort((a, b) => b.rating - a.rating || b.ratingCount - a.ratingCount)
            .map((w) => (
              <BookCard key={w.slug} work={w} />
            ))}
        </div>
      </section>
    </div>
  );
}
