import Link from "next/link";
import { PageFrame } from "@/components/frame/PageFrame";
import { MarkAllRead } from "@/components/social/MarkAllRead";
import { me, requestCookie } from "@/lib/session";
import { tryGet } from "@/lib/api";

export const dynamic = "force-dynamic";

type Note = {
  id: string;
  kind: string;
  payload: Record<string, unknown>;
  read_at: string | null;
  created_at: string;
};

/** The notification centre: addressed notes only — replies to your words,
 *  verdicts on your scholarship, new followers. Never broadcasts, never
 *  "people like you also…". */
export default async function NotificationsPage() {
  const cookie = requestCookie();
  const session = await me();
  if (!session.authenticated) {
    return (
      <PageFrame pathname="" epigraph="." attribution=".">
        <h1 className="section-title !text-[1.7rem]">Alerts</h1>
        <p className="pullquote mt-4">Sign in to see your notifications.</p>
        <Link href="/signin" className="btn-solid mt-5">Sign in</Link>
      </PageFrame>
    );
  }
  const notes = (await tryGet<{ notifications: Note[] }>("/v1/me/notifications?limit=50", { cookie }))?.notifications ?? [];

  return (
    <PageFrame pathname="" epigraph="A reply is a hand raised across the room." attribution="Alexandria">
      <div className="flex flex-wrap items-baseline justify-between gap-3">
        <h1 className="section-title !text-[1.7rem]">Alerts</h1>
        <MarkAllRead />
      </div>
      {notes.length === 0 ? (
        <p className="pullquote mt-5 max-w-[58ch]">
          Nothing yet. Notifications here are addressed: someone answered your review, reviewed
          your note, or followed your shelves. There is no broadcast channel, by design.
        </p>
      ) : (
        <ul className="mt-5 flex flex-col">
          {notes.map((n) => (
            <li key={n.id} className={`hairline py-4 first:border-t-0 ${n.read_at ? "opacity-60" : ""}`}>
              <p className="text-[0.95rem] text-ink">
                <span className="engraved-label mr-2">{n.kind.replace(/_/g, " ")}</span>
                {describe(n)}
              </p>
              <p className="mt-1 text-xs text-ink-faint">{new Date(n.created_at).toLocaleString()}</p>
            </li>
          ))}
        </ul>
      )}
    </PageFrame>
  );
}

function describe(n: Note): string {
  const p = n.payload ?? {};
  const actor = String(p.actor ?? p.reviewer ?? "someone");
  switch (n.kind) {
    case "review_comment":
      return `${actor} replied to your review.`;
    case "new_follower":
      return `${actor} now follows your shelves.`;
    case "note_reviewed":
      return `${actor} ${p.approved ? "approved" : "rejected"} your note${p.status === "published" ? " — it is now published" : ""}.`;
    default:
      return JSON.stringify(p);
  }
}
