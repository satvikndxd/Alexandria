import Link from "next/link";
import { PageFrame } from "@/components/frame/PageFrame";
import { me, requestCookie } from "@/lib/session";
import { tryGet } from "@/lib/api";
import { relativeWhen } from "@/lib/content";

export const dynamic = "force-dynamic";

type Note = {
  id: string;
  kind: string;
  body: string;
  chapter_idx: number;
  work_slug: string;
  work_title: string;
  edition_title: string;
  updated_at: string;
};

/** Notes: the reader's margin, across every edition. RLS-scoped by the API —
 *  this page can only ever render the signed-in reader's own annotations. */
export default async function NotesPage() {
  const cookie = requestCookie();
  const session = await me();
  if (!session.authenticated) {
    return (
      <PageFrame pathname="/notes" epigraph="Marginalia is the reader's signature." attribution="Alexandria">
        <h1 className="section-title !text-[1.7rem]">Notes</h1>
        <p className="pullquote mt-3 max-w-[58ch]">
          Highlights, bookmarks and margin notes live here, private by default and private by
          database law. Sign in to read your own.
        </p>
        <Link href="/signin" className="btn-solid mt-5">
          Sign in
        </Link>
      </PageFrame>
    );
  }

  const notes = (await tryGet<{ annotations: Note[] }>("/v1/me/annotations?limit=100", { cookie }))?.annotations ?? [];

  return (
    <PageFrame pathname="/notes" epigraph="Marginalia is the reader's signature." attribution="Alexandria">
      <h1 className="section-title !text-[1.7rem]">Notes</h1>
      {notes.length === 0 ? (
        <p className="pullquote mt-4 max-w-[58ch]">
          Your margin is empty. When the reader arrives (Phase 3), selections you highlight in
          public-domain texts will be kept here with chapter and offset, so a note always points
          back to the exact line that provoked it.
        </p>
      ) : (
        <ul className="mt-5 flex flex-col">
          {notes.map((n) => (
            <li key={n.id} className="hairline py-4 first:border-t-0">
              <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <span className="engraved-label">{n.kind}</span>
                <Link href={`/books/${n.work_slug}`} className="text-[1rem] font-semibold text-ink hover:underline">
                  {n.work_title}
                </Link>
                <span className="text-xs text-ink-faint">
                  {n.edition_title} · ch. {n.chapter_idx + 1} · {relativeWhen(n.updated_at)}
                </span>
              </div>
              {n.body ? <p className="pullquote mt-2 whitespace-pre-line">{n.body}</p> : null}
            </li>
          ))}
        </ul>
      )}
    </PageFrame>
  );
}
