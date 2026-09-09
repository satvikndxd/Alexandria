import Link from "next/link";
import type { Work } from "@/lib/data";
import { WoodcutCover } from "@/components/WoodcutCover";

/**
 * The book plate, per the reference sheet: the cover is the anchor, set in a
 * ruled plate; beneath it the title in serif and the author in smaller type.
 * No card chrome, no buttons, no badges — chrome stays off the cover.
 */
export function BookCard({ work, author }: { work: Work; author?: string }) {
  const by = author ?? work.author.name;
  return (
    <Link href={`/books/${work.slug}`} className="group block focus-visible:outline focus-visible:outline-2 focus-visible:outline-ink">
      <WoodcutCover work={work} />
      <h3 className="mt-2 text-[0.95rem] font-semibold leading-tight text-ink group-hover:underline">
        {work.title}
      </h3>
      <p className="mt-0.5 text-[0.85rem] text-ink-faint">{by}</p>
    </Link>
  );
}
