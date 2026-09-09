import Link from "next/link";
import { notFound } from "next/navigation";
import { PageFrame } from "@/components/frame/PageFrame";
import { RuleWithFleuron } from "@/components/Engravings";
import { NoteActions } from "@/components/scholar/NoteActions";
import { me, requestCookie } from "@/lib/session";
import { tryGet } from "@/lib/api";

export const dynamic = "force-dynamic";

type NoteWire = {
  id: string;
  title: string;
  body: string;
  kind: string;
  status: string;
  version: number;
  chapter_ref: string;
  anchor_quote: string;
  is_community: boolean;
  author_id: string;
  username: string;
  display_name?: string | null;
  field?: string | null;
  affiliation?: string | null;
  scholar_status?: string | null;
  work_slug: string;
  work_title: string;
};
type Citation = { id: string; citation: string; url?: string | null };
type Revision = { id: string; version: number; created_at: string; edited_by?: string | null };
type Review = { reviewer_id: string; username: string; approved: boolean; comments: string; field?: string | null };

/** A note as public record: provenance, citations, revision history, and the
 *  peer-review trail. Retraction keeps the record; correction happens in view. */
export default async function NotePage({ params }: { params: { id: string } }) {
  const cookie = requestCookie();
  const session = await me();
  const res = await tryGet<{ note: NoteWire; citations: Citation[]; revisions: Revision[]; reviews: Review[] }>(
    `/v1/notes/${params.id}`,
    { cookie },
  );
  if (!res) notFound();
  const { note, citations, revisions, reviews } = res;

  return (
    <PageFrame pathname="" epigraph="Correct in public, or do not claim the page." attribution="Alexandria">
      <p className="engraved-label">
        <Link href={`/books/${note.work_slug}`} className="underline">{note.work_title}</Link> · scholar note
      </p>
      <h1 className="mt-1 text-[clamp(1.4rem,2.6vw,1.9rem)] leading-tight text-ink">{note.title}</h1>
      <p className="mt-2 text-sm text-ink-faint">
        {note.display_name ?? note.username}
        {note.field ? ` — ${note.field}` : ""}
        {note.affiliation ? `, ${note.affiliation}` : ""} ·{" "}
        <span className={note.is_community ? "text-gold" : "text-botanical"}>
          {note.is_community ? "community contributor" : `verified scholar (${note.scholar_status ?? "verified"})`}
        </span>{" "}
        · <span className="engraved-label">{note.kind}</span>
        {note.chapter_ref ? <span className="italic"> · {note.chapter_ref}</span> : null}
      </p>
      <p className="engraved-label mt-1">status: {note.status} · revision {note.version}</p>

      {note.anchor_quote ? (
        <blockquote className="mt-5 border-l-2 border-ink/60 pl-4 pullquote">“{note.anchor_quote}”</blockquote>
      ) : null}
      <p className="mt-5 max-w-[70ch] whitespace-pre-line text-[1.02rem] leading-relaxed text-ink-soft">{note.body}</p>

      <RuleWithFleuron className="my-7 max-w-[180px]" />

      <div className="grid gap-8 lg:grid-cols-2">
        <section aria-label="Citations">
          <h2 className="engraved-label">Citations</h2>
          <ul className="mt-2 flex flex-col gap-2">
            {citations.map((c) => (
              <li key={c.id} className="text-sm text-ink-soft">
                {c.url ? <a className="underline" href={c.url}>{c.citation}</a> : c.citation}
              </li>
            ))}
            {citations.length === 0 ? <li className="text-sm text-vermilion">none — this note cannot publish without one</li> : null}
          </ul>
        </section>
        <section aria-label="Peer review">
          <h2 className="engraved-label">Peer review</h2>
          <ul className="mt-2 flex flex-col gap-2">
            {reviews.map((r, i) => (
              <li key={i} className="text-sm text-ink-soft">
                <span className={r.approved ? "text-botanical" : "text-vermilion"}>
                  {r.approved ? "approved" : "rejected"}
                </span>{" "}
                by {r.username}
                {r.comments ? <span className="pullquote block">“{r.comments}”</span> : null}
              </li>
            ))}
            {reviews.length === 0 ? <li className="text-sm text-ink-faint">awaiting reviewers</li> : null}
          </ul>
        </section>
      </div>

      <section aria-label="Revision history" className="mt-8">
        <h2 className="engraved-label">Revision history</h2>
        <ul className="mt-2 flex flex-col gap-1">
          {revisions.map((v) => (
            <li key={v.id} className="text-sm text-ink-faint">
              v{v.version} · {new Date(v.created_at).toLocaleDateString()}
            </li>
          ))}
        </ul>
      </section>

      {session.authenticated ? (
        <NoteActions
          noteId={note.id}
          authorId={note.author_id}
          sessionId={session.user?.id ?? ""}
          status={note.status}
        />
      ) : null}
    </PageFrame>
  );
}
