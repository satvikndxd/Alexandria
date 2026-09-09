import Link from "next/link";
import { PageFrame } from "@/components/frame/PageFrame";
import { TempleEmblem } from "@/components/Engravings";
import { requestCookie } from "@/lib/session";
import { listClubs } from "@/lib/content";

export const dynamic = "force-dynamic";

/** Communities: book clubs with channel structure and spoiler-gated doors. */
export default async function ClubsPage() {
  const clubs = await listClubs(requestCookie());
  return (
    <PageFrame pathname="/clubs" epigraph="No man reads alone who reads in a club." attribution="Alexandria">
      <h1 className="section-title !text-[1.7rem]">Communities</h1>
      <p className="pullquote mt-2 max-w-[64ch]">
        Clubs are reading rooms, not feeds: channels per book and per chapter, voice and video rooms
        for the weekly discussion, and doors that stay shut until you have read that far.
      </p>
      <ul className="mt-7 grid gap-6 md:grid-cols-2">
        {clubs.map((c) => (
          <li key={c.slug}>
            <Link href={`/clubs/${c.slug}`} className="panel flex gap-4 px-5 py-5 hover:bg-parchment-light">
              <span className="medallion h-12 w-12 shrink-0 text-ink">
                <TempleEmblem className="h-12 w-12" />
              </span>
              <span className="min-w-0">
                <span className="block text-[1.1rem] font-semibold text-ink">{c.name}</span>
                <span className="mt-1 block text-sm leading-relaxed text-ink-soft">{c.description}</span>
                <span className="engraved-label mt-2 block">
                  {c.members} members
                  {c.currentReadTitle ? ` · reading ${c.currentReadTitle}` : ""}
                  {c.isSeed ? " · demonstration seed" : ""}
                </span>
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </PageFrame>
  );
}
