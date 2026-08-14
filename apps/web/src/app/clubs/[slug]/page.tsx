import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { clubs, getClub, getWork } from "@/lib/data";
import { SectionHeading } from "@/components/Ornament";
import { WoodcutCover } from "@/components/WoodcutCover";

export function generateStaticParams() {
  return clubs.map((c) => ({ slug: c.slug }));
}

export function generateMetadata({ params }: { params: { slug: string } }): Metadata {
  return { title: getClub(params.slug)?.name ?? "Club" };
}

const KIND_GLYPH: Record<string, string> = { text: "#", voice: "♬", video: "▣", announcements: "!" };

export default function ClubPage({ params }: { params: { slug: string } }) {
  const club = getClub(params.slug);
  if (!club) notFound();
  const current = getWork(club.currentRead);

  return (
    <div className="space-y-10">
      <SectionHeading caption={`${club.members} members`} title={club.name} />
      <p className="max-w-2xl text-[0.97rem] leading-relaxed text-ink-soft">{club.description}</p>

      <div className="grid gap-8 md:grid-cols-[1fr_260px]">
        {/* Channels */}
        <section aria-label="Channels">
          <p className="engraved-label mb-3">Rooms</p>
          <ul className="divide-y divide-ink/30 border-2 border-ink bg-parchment-light">
            {club.channels.map((ch) => (
              <li key={ch.name} className="flex items-center justify-between gap-3 px-4 py-3">
                <span className="flex items-center gap-3">
                  <span aria-hidden className="w-5 text-center font-body text-botanical">
                    {KIND_GLYPH[ch.kind]}
                  </span>
                  <span className={`font-body ${ch.kind === "text" ? "" : "italic"} text-ink`}>{ch.name}</span>
                </span>
                {ch.gated ? (
                  <span className="border border-vermilion px-2 py-0.5 text-[0.6rem] uppercase tracking-engraved text-vermilion">
                    {ch.gated}
                  </span>
                ) : (
                  <span className="text-[0.6rem] uppercase tracking-engraved text-ink-faint">{ch.kind}</span>
                )}
              </li>
            ))}
          </ul>
          <p className="mt-3 text-xs italic leading-relaxed text-ink-faint">
            Spoiler-gated rooms unlock automatically as your reading progress on the current book
            passes each threshold. Voice and video rooms are LiveKit-backed in the full deployment.
          </p>
        </section>

        {/* Current read */}
        {current && (
          <aside aria-label="Current read">
            <p className="engraved-label mb-3">Currently reading</p>
            <Link href={`/books/${current.slug}`} className="block">
              <WoodcutCover work={current} className="shadow-block" />
            </Link>
            <p className="mt-3 text-center font-body font-semibold text-ink">{current.title}</p>
            <p className="text-center text-sm italic text-ink-faint">{current.author.name}</p>
          </aside>
        )}
      </div>
    </div>
  );
}
