"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { IconColumns, IconCompass, IconFolio, IconLibrary, Fleuron } from "./Ornament";

const NAV = [
  { href: "/", label: "Atrium", icon: IconColumns },
  { href: "/discover", label: "Discover", icon: IconCompass },
  { href: "/library", label: "Library", icon: IconLibrary },
  { href: "/clubs", label: "Clubs", icon: IconFolio },
];

function NavLink({
  href,
  label,
  icon: Icon,
  active,
  variant,
}: {
  href: string;
  label: string;
  icon: typeof IconColumns;
  active: boolean;
  variant: "side" | "bottom";
}) {
  if (variant === "bottom") {
    return (
      <Link
        href={href}
        aria-current={active ? "page" : undefined}
        className={`flex flex-1 flex-col items-center gap-0.5 py-2 text-[0.6rem] uppercase tracking-engraved ${
          active ? "text-vermilion" : "text-parchment-dark hover:text-parchment-light"
        }`}
      >
        <Icon size={20} />
        {label}
      </Link>
    );
  }
  return (
    <Link
      href={href}
      aria-current={active ? "page" : undefined}
      className={`flex items-center gap-3 border-l-2 px-4 py-2.5 text-sm uppercase tracking-engraved transition-colors ${
        active
          ? "border-vermilion bg-ink/5 text-vermilion"
          : "border-transparent text-ink-soft hover:border-botanical hover:text-botanical"
      }`}
    >
      <Icon size={18} />
      {label}
    </Link>
  );
}

export function Shell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const isActive = (href: string) => (href === "/" ? pathname === "/" : pathname.startsWith(href));

  return (
    <div className="mx-auto flex min-h-screen max-w-folio">
      {/* Desktop sidebar */}
      <aside className="sticky top-0 hidden h-screen w-56 shrink-0 flex-col border-r-2 border-ink md:flex">
        <Link href="/" className="block px-4 pb-4 pt-8">
          <h1 className="font-display text-4xl leading-none text-ink">Alexandria</h1>
          <p className="engraved-label mt-2">A Human Library</p>
          <div className="double-rule mt-3" aria-hidden />
        </Link>
        <nav aria-label="Primary" className="mt-2 flex flex-col">
          {NAV.map((n) => (
            <NavLink key={n.href} {...n} active={isActive(n.href)} variant="side" />
          ))}
        </nav>
        <div className="mt-auto px-4 pb-6 text-ink-faint">
          <Fleuron size={16} className="text-botanical" />
          <p className="mt-2 text-xs italic leading-snug">
            Ad-free, open-source, written and read by humans.
          </p>
        </div>
      </aside>

      {/* Content */}
      <div className="min-w-0 flex-1 pb-20 md:pb-8">
        {/* Mobile masthead */}
        <header className="border-b-2 border-ink px-4 py-4 md:hidden">
          <Link href="/">
            <h1 className="text-center font-display text-3xl text-ink">Alexandria</h1>
          </Link>
        </header>
        <main className="px-4 py-8 md:px-10">{children}</main>

        <footer className="mt-12 border-t border-ink/40 px-4 py-6 md:px-10">
          <p className="text-xs italic leading-relaxed text-ink-faint">
            Alexandria is open source (AGPL-3.0) and carries no advertising. Purchase links, where
            shown, are affiliate links — as an Amazon Associate, Alexandria earns from qualifying
            purchases. Public-domain texts courtesy of Project Gutenberg, ingested via its official
            offline catalogs.
          </p>
        </footer>
      </div>

      {/* Mobile bottom bar */}
      <nav
        aria-label="Primary"
        className="fixed inset-x-0 bottom-0 flex border-t-2 border-ink bg-ink md:hidden"
      >
        {NAV.map((n) => (
          <NavLink key={n.href} {...n} active={isActive(n.href)} variant="bottom" />
        ))}
      </nav>
    </div>
  );
}
