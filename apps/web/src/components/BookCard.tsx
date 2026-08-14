import Link from "next/link";
import type { Work } from "@/lib/data";
import { WoodcutCover } from "./WoodcutCover";
import { Stars } from "./Stars";

export function BookCard({ work }: { work: Work }) {
  return (
    <Link
      href={`/books/${work.slug}`}
      className="manuscript-card group block p-3 focus-visible:outline focus-visible:outline-2 focus-visible:outline-vermilion"
    >
      <WoodcutCover work={work} />
      <div className="mt-3">
        <h3 className="font-body text-base font-semibold leading-tight text-ink group-hover:text-botanical">
          {work.title}
        </h3>
        <p className="mt-0.5 text-sm italic text-ink-faint">{work.author.name}</p>
        <div className="mt-2 flex items-center justify-between">
          <Stars rating={work.rating} size={13} />
          <span className="oldstyle text-xs text-ink-faint">
            {work.firstPublished > 0 ? work.firstPublished : `${-work.firstPublished} BCE`}
          </span>
        </div>
      </div>
    </Link>
  );
}
