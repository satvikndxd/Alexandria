import { PageFrame } from "@/components/frame/PageFrame";
import { BookCard } from "@/components/books/BookCard";
import { requestCookie } from "@/lib/session";
import { tryGet } from "@/lib/api";
import { hueFor, type Work } from "@/lib/content";
import { works as fixtures } from "@/lib/data";

export const dynamic = "force-dynamic";

type Hit = {
  kind: string;
  slug: string;
  title?: string;
  name?: string;
  authors?: { name: string }[];
  rating?: number;
  rating_count?: number;
};

/**
 * Global search. The form is a plain GET so results are linkable and
 * crawlable; the server asks Meilisearch (through the monolith) and degrades
 * to the catalogue when the index is cold.
 */
export default async function SearchPage({ searchParams }: { searchParams: { q?: string; type?: string } }) {
  const q = (searchParams.q ?? "").trim();
  const cookie = requestCookie();

  let hits: Hit[] = [];
  let source = "none";
  if (q) {
    const res = await tryGet<{ source: string; hits: Hit[] }>(
      `/v1/search?q=${encodeURIComponent(q)}&limit=24${searchParams.type ? `&type=${searchParams.type}` : ""}`,
      { cookie },
    );
    if (res?.hits) {
      hits = res.hits;
      source = res.source;
    } else {
      const needle = q.toLowerCase();
      hits = fixtures
        .filter((w) => w.title.toLowerCase().includes(needle) || w.author.name.toLowerCase().includes(needle))
        .map((w) => ({ kind: "work", slug: w.slug, title: w.title, authors: [{ name: w.author.name }] }));
      source = "fixtures";
    }
  }

  return (
    <PageFrame pathname="/search" epigraph="The whole art of knowledge is to know what to ask." attribution="Jean-Paul">
      <h1 className="section-title !text-[1.6rem]">Search the Library</h1>
      <form action="/search" method="GET" className="mt-4 flex flex-wrap items-center gap-3">
        <label className="engraved-label" htmlFor="q">
          Query
        </label>
        <input id="q" name="q" defaultValue={q} className="field max-w-md" placeholder="Title, author, or subject…" />
        <select name="type" defaultValue={searchParams.type ?? ""} className="field !w-auto" aria-label="Search index">
          <option value="">Works</option>
          <option value="authors">Authors</option>
          <option value="clubs">Clubs</option>
        </select>
        <button className="btn-solid" type="submit">
          Search
        </button>
      </form>

      {q ? (
        <>
          <p className="engraved-label mt-5">
            {hits.length} result{hits.length === 1 ? "" : "s"} for “{q}”
            {source === "postgres_fallback" ? " · index degraded, catalogue scan" : ""}
          </p>
          <ul className="mt-5 grid grid-cols-2 gap-x-5 gap-y-7 sm:grid-cols-3 lg:grid-cols-5">
            {hits.map((h) => (
              <li key={`${h.kind}-${h.slug}`}>
                <BookCard
                  work={toWork(h)}
                  author={h.authors?.[0]?.name ?? (h.kind === "author" ? `${h.name}` : "")}
                />
              </li>
            ))}
          </ul>
          {hits.length === 0 ? (
            <p className="pullquote mt-5">
              Nothing answers to “{q}”. Try an author's surname, or browse the catalogue from Discover.
            </p>
          ) : null}
        </>
      ) : (
        <p className="pullquote mt-5">
          Search reaches works, authors and clubs. It is a lookup, not a recommendation engine: the
          order is relevance to your words, never to an advertiser's.
        </p>
      )}
    </PageFrame>
  );
}

function toWork(h: Hit): Work {
  return {
    slug: h.slug,
    title: h.title ?? h.name ?? h.slug,
    author: { id: "", name: h.authors?.[0]?.name ?? "", years: "" },
    firstPublished: 0,
    description: "",
    isPublicDomain: false,
    subjects: [],
    rating: h.rating ?? 0,
    ratingCount: h.rating_count ?? 0,
    coverHue: hueFor(h.slug),
  };
}
