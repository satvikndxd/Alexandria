import type { Metadata } from "next";
import Link from "next/link";
import { clubs, getWork } from "@/lib/data";
import { SectionHeading, Fleuron } from "@/components/Ornament";
import { WoodcutCover } from "@/components/WoodcutCover";

export const metadata: Metadata = { title: "Book Clubs" };

export default function ClubsPage() {
  return (
    <div className="space-y-10">
      <SectionHeading caption="Reading rooms with spoiler-gated doors" title="Book Clubs" />

      <div className="grid gap-5 md:grid-cols-2">
        {clubs.map((club) => {
          const current = getWork(club.currentRead);
          return (
            <Link key={club.slug} href={`/clubs/${club.slug}`} className="manuscript-card flex gap-5 p-5">
              {current && (
                <div className="w-24 shrink-0">
                  <WoodcutCover work={current} />
                </div>
              )}
              <div className="min-w-0">
                <Fleuron size={16} className="text-vermilion" />
                <h3 className="mt-1 font-display text-2xl text-ink">{club.name}</h3>
                <p className="mt-2 text-sm leading-relaxed text-ink-soft">{club.description}</p>
                <p className="oldstyle mt-3 text-xs uppercase tracking-engraved text-botanical">
                  {club.members} members · reading {current?.title}
                </p>
              </div>
            </Link>
          );
        })}
      </div>
    </div>
  );
}
