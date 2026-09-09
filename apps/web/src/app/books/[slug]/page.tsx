import Link from "next/link";
import { notFound } from "next/navigation";
import { PageFrame } from "@/components/frame/PageFrame";
import { WoodcutCover } from "@/components/WoodcutCover";
import { Stars } from "@/components/Stars";
import { ShelfControls } from "@/components/library/ShelfControls";
import { ReviewRow } from "@/components/reviews/ReviewRow";
import { RuleWithFleuron, Sprig } from "@/components/Engravings";
import { me, requestCookie } from "@/lib/session";
import { tryGet } from "@/lib/api";
import { listNotes, listReviews, workDetail } from "@/lib/content";

export const dynamic = "force-dynamic";

/**
 * The book page, in the order a reader actually wants it: the object itself,
 * then what it is about, then what scholarship says, then what readers say —
 * with editions and related works in the context column. Covers carry their
 * license line; nothing here is an advertisement.
 */
export default async function BookPage({ params }: { params: { slug: string } }) {
  const cookie = requestCookie();
  const session = await me();
  const detail = await workDetail(params.slug, cookie);
  if (!detail) notFound();

  const [reviews, notes, related] = await Promise.all([
    listReviews(params.slug, cookie),
    listNotes(params.slug, cookie),
    tryGet<{ related: { slug: string; title: string; first_published?: number | null }[] }>(
      `/v1/works/${params.slug}/related`,
      { cookie },
    ),
  ]);

  const w = detail.work;
  const authorLine = detail.authors.map((a) => a.name).filter(Boolean).join(", ") || w.author.name;

  return (
    <PageFrame
      pathname=""
      epigraph="A book is a garden carried in the pocket."
      attribution="Chinese proverb"
    >
      {/* ——— the object ——— */}
      <header className="flex flex-col gap-7 md:flex-row">
        <div className="w-[168px] shrink-0">
          <WoodcutCover work={w} />
          {w.isPublicDomain ? (
            <p className="engraved-label mt-2 text-center">Public domain</p>
          ) : null}
        </div>
        <div className="min-w-0 flex-1">
          <p className="engraved-label">Work</p>
          <h1 className="mt-1 text-[clamp(1.6rem,3vw,2.2rem)] leading-tight text-ink">{w.title}</h1>
          <p className="mt-1 text-[1.05rem] italic text-ink-soft">{authorLine}</p>
          <div className="mt-3 flex flex-wrap items-center gap-4">
            <Stars rating={w.rating} size={16} />
            <span className="oldstyle text-sm text-ink-soft">
              {w.rating > 0 ? w.rating.toFixed(1) : "—"} · {w.ratingCount} rating{w.ratingCount === 1 ? "" : "s"} ·{" "}
              {detail.reviewCount} review{detail.reviewCount === 1 ? "" : "s"}
            </span>
            {w.firstPublished ? (
              <span className="oldstyle text-sm text-ink-faint">first published {w.firstPublished}</span>
            ) : null}
          </div>

          <div className="mt-5">
            {session.authenticated ? (
              <ShelfControls workSlug={w.slug} current={detail.myLibrary?.shelf_kind ?? null} />
            ) : (
              <Link href="/signin" className="btn-print">
                Sign in to shelve this book
              </Link>
            )}
          </div>
          <div className="mt-4 flex flex-wrap gap-3">
            <Link href={`/books/${w.slug}/review`} className="btn-solid">
              Write a review
            </Link>
            <Link href={`/books/${w.slug}/purchase`} className="btn-print">
              Purchase options
            </Link>
            {readableEdition(detail) ? (
              <Link href={`/read/${readableEdition(detail)}`} className="btn-print">
                Read the public-domain edition
              </Link>
            ) : null}
          </div>
        </div>
      </header>

      <RuleWithFleuron className="my-8" />

      <div className="grid gap-10 lg:grid-cols-[minmax(0,1fr)_260px]">
        <div className="min-w-0">
          {/* ——— about ——— */}
          <section aria-labelledby="about-h">
            <h2 id="about-h" className="section-title">
              About the Work
            </h2>
            <p className="mt-3 max-w-[68ch] text-[1.02rem] leading-relaxed text-ink-soft">
              {w.description || "No description has been ingested for this work yet."}
            </p>
            {w.subjects.length ? (
              <ul className="mt-4 flex flex-wrap gap-2">
                {w.subjects.map((s) => (
                  <li key={s} className="border border-ink/60 px-2 py-0.5 text-[0.78rem] text-ink-soft">
                    {s}
                  </li>
                ))}
              </ul>
            ) : null}
          </section>

          {/* ——— scholarship ——— */}
          <section aria-labelledby="notes-h" className="mt-10">
            <h2 id="notes-h" className="section-title">
              Scholarship
            </h2>
            <p className="pullquote mt-2 max-w-[62ch]">
              Notes are written by people, reviewed by two other verified scholars before publication,
              and carry their citations. Community contributors' notes are labelled as such — different
              provenance, not lesser worth.
            </p>
            {notes.length === 0 ? (
              <p className="mt-4 text-sm text-ink-faint">
                No notes yet for this work. Scholars and community contributors may submit the first.
              </p>
            ) : (
              <ul className="mt-4 flex flex-col gap-5">
                {notes.map((n) => (
                  <li key={n.id} className="panel px-5 py-4">
                    <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                      <span className="engraved-label">{n.kind}</span>
                      {n.chapterRef ? <span className="text-sm italic text-ink-soft">{n.chapterRef}</span> : null}
                      <span
                        className={`engraved-label ${n.verified ? "!text-botanical" : "!text-gold"}`}
                        title={n.verified ? "Verified scholar, peer-reviewed" : "Community contributor"}
                      >
                        {n.verified ? "Verified scholar" : "Community contributor"}
                      </span>
                    </div>
                    <h3 className="mt-1 text-[1.08rem] font-semibold text-ink">{n.title}</h3>
                    <p className="mt-1 text-sm text-ink-faint">
                      {n.scholar}
                      {n.field ? ` — ${n.field}` : ""}
                    </p>
                    <p className="mt-2 whitespace-pre-line text-[0.98rem] leading-relaxed text-ink-soft">{n.body}</p>
                  </li>
                ))}
              </ul>
            )}
          </section>

          {/* ——— reviews ——— */}
          <section aria-labelledby="reviews-h" className="mt-10">
            <div className="flex items-baseline justify-between">
              <h2 id="reviews-h" className="section-title">
                Reviews
              </h2>
              <Link href={`/books/${w.slug}/review`} className="text-sm text-ink-soft hover:text-ink hover:underline">
                Write yours
              </Link>
            </div>
            {reviews.length === 0 ? (
              <p className="pullquote mt-3">
                No reviews yet. Alexandria asks for at least 150 characters and a reason — the first
                review of a book is a small act of patronage.
              </p>
            ) : (
              <ul className="mt-2">
                {reviews.map((r) => (
                  <ReviewRow
                    key={r.id}
                    username={r.username}
                    displayName={r.displayName}
                    rating={r.rating}
                    title={r.title}
                    body={r.body}
                    hasSpoilers={r.hasSpoilers}
                    likeCount={r.likeCount}
                    createdAt={r.createdAt}
                  />
                ))}
              </ul>
            )}
          </section>
        </div>

        {/* ——— context column ——— */}
        <aside className="min-w-0">
          <section aria-labelledby="hist-h">
            <h2 id="hist-h" className="engraved-label">
              Ratings
            </h2>
            <Histogram distribution={detail.ratingDistribution} />
          </section>

          {detail.editions.length ? (
            <section aria-labelledby="ed-h" className="mt-7">
              <h2 id="ed-h" className="engraved-label">
                Editions
              </h2>
              <ul className="mt-2 flex flex-col gap-3">
                {detail.editions.map((e) => (
                  <li key={e.id} className="border border-ink/50 px-3 py-2">
                    <p className="text-[0.92rem] font-semibold text-ink">{e.title}</p>
                    <p className="text-xs text-ink-faint">
                      {[e.publisher, e.language, e.page_count ? `${e.page_count} pp.` : null, e.isbn13]
                        .filter(Boolean)
                        .join(" · ")}
                    </p>
                    {e.cover_attribution ? (
                      <p className="mt-1 text-[0.68rem] italic text-ink-faint">Cover: {e.cover_attribution}</p>
                    ) : null}
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          {related?.related?.length ? (
            <section aria-labelledby="rel-h" className="mt-7">
              <h2 id="rel-h" className="engraved-label">
                Readers of this also shelved
              </h2>
              <ul className="mt-2 flex flex-col gap-2">
                {related.related.slice(0, 5).map((r) => (
                  <li key={r.slug}>
                    <Link href={`/books/${r.slug}`} className="text-[0.92rem] text-ink hover:underline">
                      {r.title}
                    </Link>
                  </li>
                ))}
              </ul>
              <p className="mt-2 text-[0.68rem] italic text-ink-faint">
                Co-shelving counts, weighted against popularity. No engagement model involved.
              </p>
            </section>
          ) : null}

          <Sprig className="mt-8 h-20 w-14 opacity-80" />
        </aside>
      </div>
    </PageFrame>
  );
}

/** The first edition with a cached public-domain text, if any. */
function readableEdition(detail: { editions: { id: string; gutenberg_id?: number | null }[] }): string | null {
  const e = detail.editions.find((x) => x.gutenberg_id != null);
  return e ? e.id : null;
}

function Histogram({ distribution }: { distribution: { rating: number; n: number }[] }) {
  const max = Math.max(1, ...distribution.map((d) => d.n));
  const rows = [10, 9, 8, 7, 6, 5, 4, 3, 2, 1];
  return (
    <ul className="mt-2 flex flex-col gap-1" aria-label="Rating distribution in half-stars">
      {rows.map((half) => {
        const d = distribution.find((x) => x.rating === half);
        const n = d?.n ?? 0;
        return (
          <li key={half} className="flex items-center gap-2">
            <span className="oldstyle w-7 text-right text-xs text-ink-faint">{(half / 2).toFixed(1)}</span>
            <span className="h-[9px] flex-1 border border-ink/50">
              <span className="block h-full bg-ink" style={{ width: `${(n / max) * 100}%` }} />
            </span>
            <span className="w-6 text-xs text-ink-faint">{n}</span>
          </li>
        );
      })}
    </ul>
  );
}
