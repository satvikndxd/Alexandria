import Link from "next/link";
import { works, reviews, scholarNotes, clubs, demoLibrary, getWork } from "@/lib/data";
import { IlluminatedParagraph } from "@/components/DropCap";
import { Divider, SectionHeading, Fleuron } from "@/components/Ornament";
import { BookCard } from "@/components/BookCard";
import { ReviewCard } from "@/components/ReviewCard";
import { ScholarNoteCard } from "@/components/ScholarNoteCard";
import { WoodcutCover } from "@/components/WoodcutCover";

export default function HomePage() {
  const continueReading = demoLibrary.reading
    .map((r) => ({ ...r, work: getWork(r.slug)! }))
    .filter((r) => r.work);

  return (
    <div className="space-y-14">
      {/* Frontispiece */}
      <section aria-labelledby="frontispiece">
        <p className="engraved-label">Anno MMXXVI · Open Source · Ad-Free</p>
        <h2 id="frontispiece" className="mt-2 font-display text-5xl leading-tight text-ink md:text-6xl">
          The Atrium
        </h2>
        <div className="double-rule mt-4" aria-hidden />
        <div className="mt-6 max-w-2xl">
          <IlluminatedParagraph text="Books are written by humans, discussed by humans, explained by humans, and read by humans. Alexandria is a quiet room in a loud age: a library, a margin to write in, and a table of good company — with no advertisements, no engagement bait, and no machine posing as a reader." />
        </div>
      </section>

      {/* Continue Reading */}
      <section aria-labelledby="continue-reading">
        <SectionHeading caption="Where you left the ribbon" title="Continue Reading" />
        <div className="mt-6 grid gap-4 sm:grid-cols-2">
          {continueReading.map(({ work, progress, format }) => (
            <Link
              key={work.slug}
              href={`/books/${work.slug}`}
              className="manuscript-card flex gap-4 p-4"
            >
              <div className="w-20 shrink-0">
                <WoodcutCover work={work} />
              </div>
              <div className="min-w-0 flex-1">
                <h3 className="font-body text-lg font-semibold leading-tight text-ink">{work.title}</h3>
                <p className="text-sm italic text-ink-faint">{work.author.name}</p>
                <p className="mt-1 text-xs text-ink-faint">{format}</p>
                <div className="mt-3">
                  <div className="h-2 border border-ink bg-parchment">
                    <div className="h-full bg-botanical" style={{ width: `${progress * 100}%` }} />
                  </div>
                  <p className="oldstyle mt-1 text-xs text-ink-faint">
                    {Math.round(progress * 100)}% through
                  </p>
                </div>
              </div>
            </Link>
          ))}
        </div>
      </section>

      {/* Public Domain Classics */}
      <section aria-labelledby="classics">
        <SectionHeading caption="Free to every reader, forever" title="Public Domain Classics" />
        <div className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          {works.slice(0, 4).map((w) => (
            <BookCard key={w.slug} work={w} />
          ))}
        </div>
        <p className="mt-4 text-right">
          <Link href="/discover" className="text-sm italic text-botanical underline underline-offset-4 hover:text-vermilion">
            Browse the whole collection →
          </Link>
        </p>
      </section>

      <Divider />

      {/* Human Reviews */}
      <section aria-labelledby="human-reviews">
        <SectionHeading caption="Written by people, at length, on purpose" title="Human Reviews" />
        <div className="mt-6 space-y-5">
          {reviews.slice(0, 2).map((r) => (
            <ReviewCard key={r.id} review={r} />
          ))}
        </div>
      </section>

      {/* Scholar Notes */}
      <section aria-labelledby="scholarship">
        <SectionHeading caption="Marginalia with citations" title="From the Scriptorium" />
        <div className="mt-6 grid gap-5 lg:grid-cols-2">
          {scholarNotes.map((n) => (
            <ScholarNoteCard key={n.id} note={n} />
          ))}
        </div>
      </section>

      <Divider />

      {/* Book Clubs */}
      <section aria-labelledby="clubs">
        <SectionHeading caption="Good company for long books" title="Book Clubs" />
        <div className="mt-6 grid gap-4 md:grid-cols-3">
          {clubs.map((c) => (
            <Link key={c.slug} href={`/clubs/${c.slug}`} className="manuscript-card block p-5">
              <Fleuron size={18} className="text-vermilion" />
              <h3 className="mt-2 font-display text-2xl text-ink">{c.name}</h3>
              <p className="mt-2 text-sm leading-relaxed text-ink-soft">{c.description}</p>
              <p className="oldstyle mt-3 text-xs uppercase tracking-engraved text-botanical">
                {c.members} members
              </p>
            </Link>
          ))}
        </div>
      </section>
    </div>
  );
}
