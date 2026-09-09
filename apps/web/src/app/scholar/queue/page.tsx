import Link from "next/link";
import { PageFrame } from "@/components/frame/PageFrame";
import { me, requestCookie } from "@/lib/session";
import { tryGet } from "@/lib/api";

export const dynamic = "force-dynamic";

type QueueRow = {
  id: string;
  title: string;
  kind: string;
  chapter_ref: string;
  username: string;
  work_title: string;
  work_slug: string;
  is_community: boolean;
};

/** The review queue: visible only to verified scholars and moderators. */
export default async function ScholarQueuePage() {
  const cookie = requestCookie();
  const session = await me();
  const queue = session.authenticated
    ? (await tryGet<{ queue: QueueRow[] }>("/v1/scholar/queue", { cookie }))?.queue ?? null
    : null;

  return (
    <PageFrame pathname="" epigraph="Peer review is scholarship's immune system." attribution="Alexandria">
      <h1 className="section-title !text-[1.7rem]">Review queue</h1>
      {!session.authenticated ? (
        <p className="pullquote mt-4">Sign in to see the queue.</p>
      ) : queue === null ? (
        <p className="pullquote mt-4 max-w-[60ch]">
          The queue is reserved for verified scholars and moderators. If you have applied, a
          moderator's decision is pending; community notes may be submitted meanwhile.
        </p>
      ) : queue.length === 0 ? (
        <p className="pullquote mt-4">Nothing awaits review. The republic of letters is momentarily at peace.</p>
      ) : (
        <ul className="mt-5 flex flex-col">
          {queue.map((q) => (
            <li key={q.id} className="hairline py-4 first:border-t-0">
              <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <Link href={`/scholar/notes/${q.id}`} className="text-[1.05rem] font-semibold text-ink hover:underline">
                  {q.title}
                </Link>
                <span className="engraved-label">{q.kind}</span>
                {q.chapter_ref ? <span className="text-sm italic text-ink-soft">{q.chapter_ref}</span> : null}
                {q.is_community ? <span className="engraved-label !text-gold">community contributor</span> : null}
              </div>
              <p className="mt-1 text-sm text-ink-faint">
                {q.username} on <Link className="italic underline" href={`/books/${q.work_slug}`}>{q.work_title}</Link>
              </p>
            </li>
          ))}
        </ul>
      )}
    </PageFrame>
  );
}
