import Link from "next/link";
import { PageFrame } from "@/components/frame/PageFrame";
import { BookCard } from "@/components/books/BookCard";
import { requestCookie } from "@/lib/session";
import { tryGet } from "@/lib/api";
import { listWorks } from "@/lib/content";
import { works as fixtures } from "@/lib/data";

export const dynamic = "force-dynamic";

/** Discover: the catalogue, browsable by subject. No ranking magic — the
 *  subject list is popularity-of-shelving, and within a subject the order is
 *  the Bayesian rating the schema maintains. */
export default async function Discover({ searchParams }: { searchParams: { subject?: string } }) {
  const cookie = requestCookie();
  const subject = searchParams.subject ?? "";
  const works = subject
    ? await listWorks(cookie, subject)
    : await listWorks(cookie);
  const subjects =
    (await tryGet<{ subjects: { slug: string; name: string; work_count: number }[] }>("/v1/subjects?limit=24", { cookie }))
      ?.subjects ?? [];

  const authors: Record<string, string> = {};
  for (const w of works) authors[w.slug] = w.author.name || fixtures.find((f) => f.slug === w.slug)?.author.name || "";

  return (
    <PageFrame pathname="/discover" epigraph="There is no frigate like a book to take us lands away." attribution="Emily Dickinson">
      <h1 className="section-title !text-[1.6rem]">Discover</h1>
      <p className="pullquote mt-2 max-w-[62ch]">
        The catalogue is browsed, not fed to you. Choose a subject, or take the shelves in the order
        readers' own ratings imply — a Bayesian mean, so one enthusiastic stranger cannot outrank four
        hundred considered ones.
      </p>

      {subjects.length ? (
        <ul className="mt-5 flex flex-wrap gap-2">
          {subjects.map((s) => (
            <li key={s.slug}>
              <Link
                href={`/discover?subject=${encodeURIComponent(s.slug)}`}
                className={`btn-print !px-3 !py-1 !text-[0.8rem] ${subject === s.slug ? "!bg-ink !text-parchment-light" : ""}`}
              >
                {s.name}
              </Link>
            </li>
          ))}
        </ul>
      ) : null}

      <ul className="mt-7 grid grid-cols-2 gap-x-5 gap-y-7 sm:grid-cols-3 lg:grid-cols-5">
        {works.map((w) => (
          <li key={w.slug}>
            <BookCard work={w} author={authors[w.slug]} />
          </li>
        ))}
      </ul>
    </PageFrame>
  );
}
