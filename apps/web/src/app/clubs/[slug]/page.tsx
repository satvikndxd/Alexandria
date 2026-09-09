import Link from "next/link";
import { notFound } from "next/navigation";
import { PageFrame } from "@/components/frame/PageFrame";
import { TempleEmblem } from "@/components/Engravings";
import { RoomJoin } from "@/components/clubs/RoomJoin";
import { me, requestCookie } from "@/lib/session";
import { tryGet } from "@/lib/api";
import { listClubs, relativeWhen } from "@/lib/content";
import { getClub } from "@/lib/data";

export const dynamic = "force-dynamic";

type Channel = { id: string; kind: string; name: string; topic: string; spoiler_threshold_bp: number | null; viewer_progress_bp: number };
type Message = { id: string; username: string; body: string; has_spoilers: boolean; created_at: string };

/**
 * A club room: channels down the left, messages in the folio. Spoiler-gated
 * channels refuse readers below the threshold — the API returns 403 and this
 * page renders the door, closed, with the reason on it.
 */
export default async function ClubPage({
  params,
  searchParams,
}: {
  params: { slug: string };
  searchParams: { ch?: string };
}) {
  const cookie = requestCookie();
  const session = await me();

  const club =
    (await tryGet<{ club: { slug: string; name: string; description: string; member_count: number }; current_read: { work_title?: string } | null }>(
      `/v1/clubs/${params.slug}`,
      { cookie },
    ))?.club ?? null;

  if (!club) {
    const fixture = getClub(params.slug);
    if (!fixture) notFound();
    return (
      <PageFrame pathname="/clubs" epigraph="A book club is a conspiracy of readers." attribution="Alexandria">
        <h1 className="section-title !text-[1.7rem]">{fixture.name}</h1>
        <p className="pullquote mt-3 max-w-[62ch]">{fixture.description}</p>
        <ul className="mt-6 flex flex-col gap-2">
          {fixture.channels.map((ch) => (
            <li key={ch.name} className="hairline py-2 text-[0.95rem] text-ink-soft">
              <span className="engraved-label mr-3">{ch.kind}</span>
              {ch.name}
              {ch.gated ? <span className="ml-2 text-xs text-vermilion">{ch.gated}</span> : null}
            </li>
          ))}
        </ul>
        <p className="engraved-label mt-6">Demonstration seed — start the API to enter live rooms.</p>
      </PageFrame>
    );
  }

  const channels =
    (await tryGet<{ channels: Channel[] }>(`/v1/clubs/${params.slug}/channels`, { cookie }))?.channels ?? [];
  const activeId = searchParams.ch ?? channels[0]?.id ?? "";
  const active = channels.find((c) => c.id === activeId);

  let messages: Message[] = [];
  let gated: { blocked: boolean; reason?: string } = { blocked: false };
  if (active) {
    const res = await tryGet<{ messages: Message[] }>(`/v1/channels/${active.id}/messages?limit=50`, { cookie });
    if (res?.messages) messages = res.messages;
    else gated = { blocked: true, reason: "This door opens further into the book." };
  }

  return (
    <PageFrame pathname="/clubs" epigraph="A book club is a conspiracy of readers." attribution="Alexandria">
      <header className="flex items-start gap-4">
        <span className="medallion h-14 w-14 shrink-0 text-ink">
          <TempleEmblem className="h-14 w-14" />
        </span>
        <div>
          <h1 className="section-title !text-[1.7rem]">{club.name}</h1>
          <p className="mt-1 max-w-[62ch] text-[0.98rem] text-ink-soft">{club.description}</p>
          <p className="engraved-label mt-2">{Number(club.member_count)} members</p>
        </div>
      </header>

      <div className="mt-8 grid gap-8 lg:grid-cols-[210px_minmax(0,1fr)]">
        <nav aria-label="Channels">
          <ul className="flex flex-col gap-1">
            {channels.map((ch) => (
              <li key={ch.id}>
                <Link
                  href={`/clubs/${params.slug}?ch=${ch.id}`}
                  className={`rail-link !px-3 !py-2 !text-[0.9rem] ${ch.id === activeId ? "" : ""}`}
                  aria-current={ch.id === activeId ? "page" : undefined}
                >
                  <span className="engraved-label w-10 shrink-0">{ch.kind === "text" ? "#" : ch.kind === "voice" ? "♪" : "▶"}</span>
                  <span className="truncate">{ch.name}</span>
                  {ch.spoiler_threshold_bp != null ? <span className="ml-auto text-xs text-vermilion" title="Spoiler-gated">◈</span> : null}
                </Link>
              </li>
            ))}
          </ul>
          <p className="mt-4 text-xs leading-relaxed text-ink-faint">
            Voice and video rooms open in Phase 4 (LiveKit). Gated channels stay closed until your
            progress passes their threshold.
          </p>
        </nav>

        <section aria-label={active ? `Messages in ${active.name}` : "Messages"}>
          {active && (active.kind === "voice" || active.kind === "video") ? (
            <RoomJoin channelId={active.id} kind={active.kind} />
          ) : gated.blocked ? (
            <div className="panel px-6 py-8 text-center">
              <p className="engraved-label !text-vermilion">Spoiler gate</p>
              <p className="pullquote mt-3 max-w-[46ch] mx-auto">
                {gated.reason} Alexandria protects endings: this channel opens when your reading
                progress passes its threshold.
              </p>
            </div>
          ) : (
            <ul className="flex flex-col">
              {messages.length === 0 ? (
                <li className="pullquote">Quiet room. Say the first thing the chapter made you feel.</li>
              ) : (
                messages.map((m) => (
                  <li key={m.id} className="hairline py-3 first:border-t-0">
                    <div className="flex items-baseline justify-between gap-3">
                      <span className="font-semibold text-ink">{m.username}</span>
                      <span className="text-xs text-ink-faint">{relativeWhen(m.created_at)}</span>
                    </div>
                    <p className={`mt-1 text-[0.97rem] leading-relaxed text-ink-soft ${m.has_spoilers ? "spoiler-veil" : ""}`}>
                      {m.body}
                    </p>
                  </li>
                ))
              )}
            </ul>
          )}
          {!session.authenticated ? (
            <p className="engraved-label mt-5">
              <Link href="/signin" className="underline">
                Sign in
              </Link>{" "}
              to post in this room.
            </p>
          ) : null}
        </section>
      </div>
    </PageFrame>
  );
}
