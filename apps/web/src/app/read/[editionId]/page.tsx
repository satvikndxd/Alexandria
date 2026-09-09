import Link from "next/link";
import { notFound } from "next/navigation";
import { PageFrame } from "@/components/frame/PageFrame";
import { ReaderChapter, type AnnotationWire } from "@/components/reader/ReaderChapter";
import { me, requestCookie } from "@/lib/session";
import { tryGet } from "@/lib/api";

export const dynamic = "force-dynamic";

type Meta = {
  edition_id: string;
  work_slug: string;
  work_title: string;
  title: string;
  language: string;
  license_note: string;
  trademark_note: string;
  chapters: { index: number; title: string }[];
};
type ChapterPayload = { index: number; text: string; annotations?: AnnotationWire[] };

/**
 * The reader: a typeset public-domain edition with the reader's own
 * typography, and a margin that is private by database law.
 *
 * The text is served from our immutable cache (never hotlinked from
 * gutenberg.org), boilerplate stripped and the license notice printed at the
 * foot of every chapter — the trademark travels with the text.
 */
export default async function ReadPage({
  params,
  searchParams,
}: {
  params: { editionId: string };
  searchParams: { ch?: string };
}) {
  const cookie = requestCookie();
  const session = await me();
  const meta = await tryGet<Meta>(`/v1/editions/${params.editionId}/reader`, { cookie });
  if (!meta) notFound();

  const chapterIndex = Number(searchParams.ch ?? "0") || 0;
  const chapter = await tryGet<ChapterPayload>(
    `/v1/editions/${params.editionId}/reader/chapter/${chapterIndex}`,
    { cookie },
  );
  if (!chapter) notFound();

  const prev = meta.chapters.find((c) => c.index === chapterIndex - 1);
  const next = meta.chapters.find((c) => c.index === chapterIndex + 1);

  return (
    <PageFrame
      pathname=""
      epigraph="A book must travel in the reader's own type."
      attribution="Alexandria"
    >
      <header className="flex flex-wrap items-baseline justify-between gap-3">
        <div>
          <p className="engraved-label">
            <Link href={`/books/${meta.work_slug}`} className="underline">
              {meta.work_title}
            </Link>{" "}
            · public-domain edition
          </p>
          <h1 className="mt-1 text-[clamp(1.4rem,2.6vw,1.9rem)] text-ink">{chapterTitle(meta, chapterIndex)}</h1>
        </div>
        <nav aria-label="Chapters" className="flex items-center gap-3 text-sm">
          {prev ? (
            <Link className="btn-print !py-1 !text-[0.8rem]" href={`/read/${params.editionId}?ch=${prev.index}`}>
              ← {prev.title.slice(0, 22)}
            </Link>
          ) : (
            <span />
          )}
          {next ? (
            <Link className="btn-print !py-1 !text-[0.8rem]" href={`/read/${params.editionId}?ch=${next.index}`}>
              {next.title.slice(0, 22)} →
            </Link>
          ) : null}
        </nav>
      </header>

      {meta.chapters.length > 1 ? (
        <details className="mt-4">
          <summary className="engraved-label cursor-pointer">Contents · {meta.chapters.length} chapters</summary>
          <ul className="mt-2 grid gap-1 sm:grid-cols-2 lg:grid-cols-3">
            {meta.chapters.map((c) => (
              <li key={c.index}>
                <Link
                  href={`/read/${params.editionId}?ch=${c.index}`}
                  className={`block truncate px-2 py-1 text-sm hover:bg-ink/5 ${c.index === chapterIndex ? "bg-ink text-parchment-light" : "text-ink-soft"}`}
                >
                  {c.index + 1}. {c.title}
                </Link>
              </li>
            ))}
          </ul>
        </details>
      ) : null}

      <div className="mt-5">
        <ReaderChapter
          editionId={params.editionId}
          chapterIndex={chapterIndex}
          text={chapter.text}
          annotations={chapter.annotations ?? []}
          signedIn={session.authenticated}
          licenseNote={`${meta.license_note} ${meta.trademark_note}`}
        />
      </div>
    </PageFrame>
  );
}

function chapterTitle(meta: Meta, idx: number): string {
  return meta.chapters.find((c) => c.index === idx)?.title ?? meta.title;
}
