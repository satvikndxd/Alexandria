import Link from "next/link";
import { PageFrame } from "@/components/frame/PageFrame";
import { WoodcutCover } from "@/components/WoodcutCover";
import { ShelfControls } from "@/components/library/ShelfControls";
import { me, requestCookie } from "@/lib/session";
import { tryGet } from "@/lib/api";
import { hueFor, type Work } from "@/lib/content";

export const dynamic = "force-dynamic";

type ShelfRow = { id: string; kind: string; name: string; is_private: boolean; item_count: number };
type ItemRow = {
  work_slug: string;
  work_title: string;
  shelf_kind: string;
  primary_author: string;
  progress_bp: number;
  format?: string | null;
  is_public_domain: boolean;
};

const SHELL: Record<string, string> = {
  reading: "reading",
  want_to_read: "want_to_read",
  read: "read",
  dnf: "dnf",
  favorites: "favorites",
};

/** My Library: the reader's shelves, exactly as Row-Level Security scopes
 *  them — this page can only ever show the signed-in reader's own rows. */
export default async function LibraryPage({ searchParams }: { searchParams: { shelf?: string } }) {
  const cookie = requestCookie();
  const session = await me();
  if (!session.authenticated) {
    return (
      <PageFrame pathname="/library" epigraph="My library is my paradise." attribution="Voltaire">
        <h1 className="section-title !text-[1.6rem]">My Library</h1>
        <p className="pullquote mt-3 max-w-[58ch]">
          Your shelves are private by database law, not by courtesy: Postgres refuses every row that
          is not yours. Sign in to see them.
        </p>
        <Link href="/signin" className="btn-solid mt-5">
          Sign in
        </Link>
      </PageFrame>
    );
  }

  const shelves = (await tryGet<{ shelves: ShelfRow[] }>("/v1/me/shelves", { cookie }))?.shelves ?? [];
  const active = searchParams.shelf && SHELL[searchParams.shelf] ? SHELL[searchParams.shelf] : "reading";
  const items =
    (await tryGet<{ items: ItemRow[] }>(`/v1/me/library?shelf=${active}&limit=48`, { cookie }))?.items ?? [];

  return (
    <PageFrame pathname="/library" epigraph="My library is my paradise." attribution="Voltaire">
      <h1 className="section-title !text-[1.6rem]">My Library</h1>

      <ul className="mt-4 flex flex-wrap gap-6 border-b border-ink/40">
        {shelves.map((s) => (
          <li key={s.id}>
            <Link
              href={`/library?shelf=${s.kind}`}
              className="tab-link"
              aria-selected={s.kind === active}
            >
              {s.name}
              <span className="ml-2 text-xs text-ink-faint">{Number(s.item_count)}</span>
              {s.is_private ? <span className="ml-1 text-xs text-ink-faint" title="Private shelf">· private</span> : null}
            </Link>
          </li>
        ))}
      </ul>

      {items.length === 0 ? (
        <p className="pullquote mt-6 max-w-[58ch]">
          This shelf is empty. Shelve something from a book page and it will keep its place here —
          progress, edition, format and all.
        </p>
      ) : (
        <ul className="mt-6 flex flex-col gap-6">
          {items.map((it) => (
            <li key={it.work_slug} className="hairline flex gap-5 pt-5 first:border-t-0 first:pt-0">
              <Link href={`/books/${it.work_slug}`} className="w-[74px] shrink-0">
                <WoodcutCover work={miniWork(it)} />
              </Link>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-baseline justify-between gap-2">
                  <h2 className="text-[1.1rem] font-semibold text-ink">
                    <Link href={`/books/${it.work_slug}`} className="hover:underline">
                      {it.work_title}
                    </Link>
                  </h2>
                  <span className="text-sm text-ink-faint">{it.primary_author}</span>
                </div>
                {it.progress_bp > 0 ? (
                  <div className="mt-2 flex max-w-sm items-center gap-3">
                    <div className="progress-track flex-1">
                      <span className="progress-fill" style={{ width: `${it.progress_bp / 100}%` }} />
                    </div>
                    <span className="oldstyle text-xs text-ink-soft">{Math.round(it.progress_bp / 100)}%</span>
                  </div>
                ) : null}
                <p className="mt-1 text-xs text-ink-faint">{it.format ?? "—"}</p>
                <div className="mt-3">
                  <ShelfControls workSlug={it.work_slug} current={it.shelf_kind} compact />
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}
    </PageFrame>
  );
}

function miniWork(it: ItemRow): Work {
  return {
    slug: it.work_slug,
    title: it.work_title,
    author: { id: "", name: it.primary_author, years: "" },
    firstPublished: 0,
    description: "",
    isPublicDomain: it.is_public_domain,
    subjects: [],
    rating: 0,
    ratingCount: 0,
    coverHue: hueFor(it.work_slug),
  };
}
