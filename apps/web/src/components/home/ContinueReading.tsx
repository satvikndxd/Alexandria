import Link from "next/link";
import { Sprig } from "@/components/Engravings";
import { WoodcutCover } from "@/components/WoodcutCover";
import type { Work } from "@/lib/data";

export type ContinueItem = {
  work: Work;
  author: string;
  progressBp: number; // 0..10000
  formatLine: string;
  /** The reader's own words about this book, if they have written any. */
  ownExcerpt?: string;
};

/**
 * "Continue Reading": cover plate, title, author, a ruled progress track with
 * its percentage, the edition/format line, and — when the reader has written
 * about the book — their own excerpt as the pull-quote. We quote the reader,
 * never a copyrighted translation we have no rights to.
 */
export function ContinueReading({ item }: { item: ContinueItem | null }) {
  return (
    <section aria-labelledby="continue-heading" className="mt-9">
      <div className="flex items-baseline justify-between">
        <h2 id="continue-heading" className="section-title">
          Continue Reading
        </h2>
        <Link href="/library" className="text-ink-soft hover:text-ink" aria-label="Go to your library">
          <ArrowRight />
        </Link>
      </div>

      {item ? (
        <div className="panel mt-4 grid gap-7 px-6 py-6 md:grid-cols-[132px_minmax(0,1fr)_220px]">
          <Link href={`/books/${item.work.slug}`} className="block w-[132px] shrink-0">
            <WoodcutCover work={item.work} />
          </Link>

          <div className="min-w-0">
            <h3 className="text-[1.35rem] font-semibold leading-tight text-ink">{item.work.title}</h3>
            <p className="mt-0.5 text-[0.98rem] text-ink-soft">{item.author}</p>

            <div className="mt-4 flex items-center gap-3">
              <div className="progress-track flex-1" role="progressbar"
                aria-valuenow={Math.round(item.progressBp / 100)} aria-valuemin={0} aria-valuemax={100}
                aria-label={`Progress through ${item.work.title}`}>
                <span className="progress-fill" style={{ width: `${item.progressBp / 100}%` }} />
              </div>
              <span className="oldstyle text-sm text-ink-soft">{Math.round(item.progressBp / 100)}%</span>
            </div>
            <p className="mt-2 text-[0.85rem] text-ink-faint">{item.formatLine}</p>

            {item.ownExcerpt ? (
              <blockquote className="pullquote mt-4 max-w-[46ch]">“{item.ownExcerpt}”</blockquote>
            ) : null}

            <Link href={`/books/${item.work.slug}`} className="btn-print mt-5">
              Continue Reading <ArrowRight small />
            </Link>
          </div>

          <div className="hidden flex-col items-center justify-center gap-4 md:flex">
            <Sprig className="h-24 w-16" />
            <p className="pullquote text-center">
              {item.ownExcerpt ? "Your margin note, kept where you left it." : "Pick up where you left off."}
            </p>
          </div>
        </div>
      ) : (
        <div className="panel mt-4 px-6 py-6">
          <p className="pullquote">
            Nothing is open on your desk. Choose a book from the public-domain shelf, or mark
            something <em>want to read</em> and it will wait for you here.
          </p>
          <Link href="/discover" className="btn-print mt-4">
            Browse the shelves <ArrowRight small />
          </Link>
        </div>
      )}
    </section>
  );
}

export function ArrowRight({ small = false }: { small?: boolean }) {
  return (
    <svg viewBox="0 0 24 12" width={small ? 18 : 22} height={small ? 9 : 11} aria-hidden
      fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round">
      <path d="M1 6 h20 M16 1.5 L21.5 6 L16 10.5" />
    </svg>
  );
}
