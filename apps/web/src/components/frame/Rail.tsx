import Link from "next/link";
import { Sprig, RuleWithFleuron, TreeVignette } from "@/components/Engravings";
import {
  IconColumns,
  IconCompass,
  IconLibrary,
  IconQuill,
  IconReader,
  IconCompass as IconSearch,
  IconFolio,
} from "@/components/Ornament";

/**
 * The left rail, per the reference sheet: sprig at the head, engraved nav
 * with the active item as a solid ink block, and the house motto set in
 * stacked capitals at the foot.
 */
const NAV = [
  { href: "/", label: "Home", icon: IconColumns },
  { href: "/discover", label: "Discover", icon: IconCompass },
  { href: "/library", label: "My Library", icon: IconLibrary },
  { href: "/notes", label: "Notes", icon: IconQuill },
  { href: "/clubs", label: "Communities", icon: IconReader },
  { href: "/search", label: "Search", icon: IconSearch },
  { href: "/settings", label: "Settings", icon: IconFolio },
] as const;

export function SideRail({ pathname }: { pathname: string }) {
  const isActive = (href: string) => (href === "/" ? pathname === "/" : pathname.startsWith(href));
  return (
    <nav aria-label="Primary" className="flex h-full flex-col border-r border-ink/70">
      <div className="px-6 pb-4 pt-6">
        <Sprig className="h-16 w-11" />
      </div>
      <ul className="flex flex-col gap-1 px-2">
        {NAV.map(({ href, label, icon: Icon }) => (
          <li key={href}>
            <Link href={href} className="rail-link" aria-current={isActive(href) ? "page" : undefined}>
              <Icon size={17} />
              <span>{label}</span>
            </Link>
          </li>
        ))}
      </ul>
      <div className="mt-auto flex flex-col items-center gap-3 px-4 pb-6 pt-10">
        <Sprig className="h-14 w-10" />
        <p className="engraved-label text-center leading-[1.7] text-ink-soft">
          Slow
          <br />
          Reading
          <br />
          Brighter
          <br />
          Minds
        </p>
        <RuleWithFleuron className="w-16" />
      </div>
    </nav>
  );
}

export function MobileBar({ pathname }: { pathname: string }) {
  const isActive = (href: string) => (href === "/" ? pathname === "/" : pathname.startsWith(href));
  return (
    <nav
      aria-label="Primary"
      className="fixed inset-x-0 bottom-0 z-40 border-t-2 border-ink bg-parchment-page lg:hidden"
    >
      <ul className="flex">
        {NAV.slice(0, 5).map(({ href, label, icon: Icon }) => (
          <li key={href} className="flex-1">
            <Link
              href={href}
              aria-current={isActive(href) ? "page" : undefined}
              className={`flex flex-col items-center gap-1 py-2 text-[0.58rem] uppercase tracking-engraved ${
                isActive(href) ? "bg-ink text-parchment-light" : "text-ink-soft"
              }`}
            >
              <Icon size={19} />
              {label}
            </Link>
          </li>
        ))}
      </ul>
    </nav>
  );
}

export function Masthead({ epigraph, attribution }: { epigraph: string; attribution: string }) {
  return (
    <header className="flex items-start justify-between gap-6 px-6 pb-5 pt-6 sm:px-8">
      <div>
        <Link href="/" className="wordmark block leading-none">
          Alexandria
        </Link>
        <p className="engraved-label mt-2 text-ink-soft">Books · People · Ideas · Forever</p>
      </div>
      <div className="hidden max-w-xs text-right md:block">
        <p className="epigraph">“{epigraph}”</p>
        <p className="engraved-label mt-1">— {attribution}</p>
      </div>
    </header>
  );
}

export type RightRailData = {
  streakDays: number;
  streakCells: ("filled" | "empty" | "today")[];
  year: { books: number; pages: number; notes: number };
  circles: { slug: string; name: string; members: string }[];
  quote: { text: string; by: string };
};

/** The right rail: streak, year totals, circles, and the botanical panel. */
export function RightRail({ data }: { data: RightRailData }) {
  return (
    <aside aria-label="Your reading life" className="hidden h-full flex-col border-l border-ink/70 xl:flex">
      <section className="border-b border-ink/60 px-5 py-5">
        <div className="flex items-start justify-between gap-3">
          <div>
            <h2 className="section-title !text-[1.05rem]">Reading Streak</h2>
            <p className="mt-1 text-ink">
              <span className="oldstyle text-4xl">{data.streakDays}</span>{" "}
              <span className="text-base">days</span>
            </p>
          </div>
          <Sprig className="h-20 w-12 shrink-0" />
        </div>
        <div className="mt-3 grid grid-cols-7 gap-[5px]" role="img" aria-label={`${data.streakDays} day reading streak`}>
          {data.streakCells.map((c, i) => (
            <span key={i} className={`streak-cell ${c === "filled" ? "filled" : c === "today" ? "today" : ""}`} />
          ))}
        </div>
        <p className="pullquote mt-3">Knowledge compounds.</p>
        <RuleWithFleuron className="mt-4" />
      </section>

      <section className="border-b border-ink/60 px-5 py-5">
        <h2 className="section-title !text-[1.05rem]">This Year</h2>
        <div className="mt-3 flex">
          <div className="stat-col flex-1 px-2 text-center first:pl-0">
            <p className="oldstyle text-2xl text-ink">{data.year.books}</p>
            <p className="engraved-label mt-1">Books</p>
          </div>
          <div className="stat-col flex-1 px-2 text-center">
            <p className="oldstyle text-2xl text-ink">{data.year.pages.toLocaleString()}</p>
            <p className="engraved-label mt-1">Pages</p>
          </div>
          <div className="stat-col flex-1 px-2 text-center last:pr-0">
            <p className="oldstyle text-2xl text-ink">{data.year.notes}</p>
            <p className="engraved-label mt-1">Notes</p>
          </div>
        </div>
        <RuleWithFleuron className="mt-4" />
      </section>

      <section className="border-b border-ink/60 px-5 py-5">
        <h2 className="section-title !text-[1.05rem]">Currently in Your Circles</h2>
        <ul className="mt-3 flex flex-col gap-3">
          {data.circles.map((c) => (
            <li key={c.slug}>
              <Link href={`/clubs/${c.slug}`} className="group flex items-center gap-3">
                <span className="medallion h-10 w-10 shrink-0 text-ink">
                  <TempleMark />
                </span>
                <span>
                  <span className="block text-[0.95rem] text-ink group-hover:underline">{c.name}</span>
                  <span className="block text-xs text-ink-faint">{c.members} members</span>
                </span>
              </Link>
            </li>
          ))}
        </ul>
      </section>

      <section className="relative flex-1 px-5 py-5">
        <TreePanel quote={data.quote} />
      </section>
    </aside>
  );
}

function TempleMark() {
  return (
    <svg viewBox="0 0 48 48" className="h-8 w-8" aria-hidden fill="none" stroke="currentColor" strokeWidth="1.4">
      <path d="M10 20 L24 11 L38 20 Z" fill="currentColor" stroke="none" />
      <path d="M12 22 h24 M13 35 h22 M11 37 h26" />
      <path d="M17 24 v10 M23 24 v10 M29 24 v10" />
    </svg>
  );
}

function TreePanel({ quote }: { quote: { text: string; by: string } }) {
  return (
    <div className="flex h-full flex-col">
      <TreeVignette className="mx-auto w-full max-w-[190px] flex-1" />
      <blockquote className="pullquote mt-3 text-center !text-[1.02rem] leading-snug">
        “{quote.text}”
      </blockquote>
      <p className="engraved-label mt-2 text-center">— {quote.by}</p>
    </div>
  );
}
