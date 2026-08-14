import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { getWork, getReviewsFor, getNotesFor, works } from "@/lib/data";
import { WoodcutCover } from "@/components/WoodcutCover";
import { Stars } from "@/components/Stars";
import { Divider, SectionHeading } from "@/components/Ornament";
import { IlluminatedParagraph } from "@/components/DropCap";
import { ReviewCard } from "@/components/ReviewCard";
import { ScholarNoteCard } from "@/components/ScholarNoteCard";
import { BookCard } from "@/components/BookCard";

export function generateStaticParams() {
  return works.map((w) => ({ slug: w.slug }));
}

export function generateMetadata({ params }: { params: { slug: string } }): Metadata {
  const work = getWork(params.slug);
  return { title: work ? `${work.title} — ${work.author.name}` : "Not found" };
}

export default function BookPage({ params }: { params: { slug: string } }) {
  const work = getWork(params.slug);
  if (!work) notFound();

  const reviews = getReviewsFor(work.slug);
  const notes = getNotesFor(work.slug);
  const related = works.filter((w) => w.slug !== work.slug && w.subjects.some((s) => work.subjects.includes(s))).slice(0, 4);

  return (
    <div className="space-y-12">
      {/* Plate + metadata */}
      <section className="grid gap-8 md:grid-cols-[240px_1fr]">
        <div>
          <WoodcutCover work={work} className="shadow-block" />
          <p className="mt-2 text-center text-[0.65rem] italic text-ink-faint">
            House cover plate — shown when no rights-cleared cover exists
          </p>
        </div>

        <div>
          <p className="engraved-label">
            {work.subjects.join(" · ")}
          </p>
          <h2 className="mt-2 font-display text-4xl leading-tight text-ink md:text-5xl">{work.title}</h2>
          <p className="mt-2 text-lg italic text-ink-soft">
            {work.author.name} <span className="oldstyle text-ink-faint">({work.author.years})</span>
          </p>

          <div className="mt-4 flex flex-wrap items-center gap-3">
            <Stars rating={work.rating} size={18} />
            <span className="oldstyle text-sm text-ink-faint">
              {work.rating} · {work.ratingCount} readers
            </span>
            <span className="oldstyle text-sm text-ink-faint">
              First published {work.firstPublished > 0 ? work.firstPublished : `${-work.firstPublished} BCE`}
            </span>
          </div>

          <div className="mt-6 flex flex-wrap gap-3">
            <button className="border-2 border-ink bg-ink px-5 py-2 text-sm uppercase tracking-engraved text-parchment-light shadow-block-vermilion hover:bg-botanical">
              Add to Library
            </button>
            {work.isPublicDomain && work.gutenbergId && (
              <a
                href={`https://www.gutenberg.org/ebooks/${work.gutenbergId}`}
                className="border-2 border-botanical px-5 py-2 text-sm uppercase tracking-engraved text-botanical hover:bg-botanical hover:text-parchment-light"
              >
                Read — Public Domain
              </a>
            )}
            <Link
              href={`/books/${work.slug}/purchase`}
              className="border-2 border-ink px-5 py-2 text-sm uppercase tracking-engraved text-ink hover:bg-ink hover:text-parchment-light"
            >
              Purchase Options
            </Link>
          </div>
        </div>
      </section>

      {/* About the work */}
      <section aria-labelledby="about">
        <SectionHeading caption="About the work" title="Argument" />
        <div className="mt-6 max-w-2xl">
          <IlluminatedParagraph text={work.description} />
        </div>
      </section>

      {/* Scholarship */}
      {notes.length > 0 && (
        <section aria-labelledby="scholarship">
          <SectionHeading caption="Peer-reviewed, cited, human" title="Scholarship" />
          <div className="mt-6 grid gap-5 lg:grid-cols-2">
            {notes.map((n) => (
              <ScholarNoteCard key={n.id} note={n} />
            ))}
          </div>
        </section>
      )}

      <Divider />

      {/* Reviews */}
      <section aria-labelledby="reviews">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <SectionHeading caption="Minimum 150 characters — tell us why" title="Reviews" />
          <Link
            href={`/books/${work.slug}/review`}
            className="border-2 border-vermilion px-4 py-2 text-sm uppercase tracking-engraved text-vermilion hover:bg-vermilion hover:text-parchment-light"
          >
            Write a review
          </Link>
        </div>
        <div className="mt-6 space-y-5">
          {reviews.length === 0 ? (
            <p className="italic text-ink-faint">
              No reviews yet. The first considered opinion sets the tone — take your time.
            </p>
          ) : (
            reviews.map((r) => <ReviewCard key={r.id} review={r} />)
          )}
        </div>
      </section>

      {/* Related */}
      {related.length > 0 && (
        <section aria-labelledby="related">
          <SectionHeading caption="Shelved nearby" title="Related Works" />
          <div className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
            {related.map((w) => (
              <BookCard key={w.slug} work={w} />
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
