import type { Metadata } from "next";
import Link from "next/link";
import { demoLibrary, getWork } from "@/lib/data";
import { SectionHeading, Divider } from "@/components/Ornament";
import { BookCard } from "@/components/BookCard";
import { WoodcutCover } from "@/components/WoodcutCover";

export const metadata: Metadata = { title: "Library" };

export default function LibraryPage() {
  const reading = demoLibrary.reading.map((r) => ({ ...r, work: getWork(r.slug)! })).filter((r) => r.work);
  const wantToRead = demoLibrary.wantToRead.map(getWork).filter(Boolean);
  const read = demoLibrary.read.map(getWork).filter(Boolean);

  return (
    <div className="space-y-12">
      <SectionHeading caption="Your shelves, your pace, your business" title="Personal Library" />

      <section aria-labelledby="currently-reading">
        <p className="engraved-label mb-4">Currently reading</p>
        <div className="grid gap-4 sm:grid-cols-2">
          {reading.map(({ work, progress, format }) => (
            <Link key={work.slug} href={`/books/${work.slug}`} className="manuscript-card flex gap-4 p-4">
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
                  <p className="oldstyle mt-1 text-xs text-ink-faint">{Math.round(progress * 100)}%</p>
                </div>
              </div>
            </Link>
          ))}
        </div>
      </section>

      <Divider />

      <section aria-labelledby="want-to-read">
        <p className="engraved-label mb-4">Want to read</p>
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          {wantToRead.map((w) => w && <BookCard key={w.slug} work={w} />)}
        </div>
      </section>

      <section aria-labelledby="read-shelf">
        <p className="engraved-label mb-4">Read</p>
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          {read.map((w) => w && <BookCard key={w.slug} work={w} />)}
        </div>
      </section>
    </div>
  );
}
