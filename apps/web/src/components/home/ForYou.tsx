"use client";

import { useState } from "react";
import type { Work } from "@/lib/data";
import { BookCard } from "@/components/books/BookCard";
import { ArrowRight } from "./ContinueReading";

const TABS = [
  "Recommended",
  "Public Domain",
  "Classics",
  "Philosophy",
  "Literature",
] as const;
type Tab = (typeof TABS)[number];

/**
 * "For You" without an engagement engine: the tabs are plain, explainable
 * filters over the catalogue — public-domain status, publication era, and
 * subject. There is no score, no personalization model, and nothing to farm.
 */
export function ForYou({ works, authors }: { works: Work[]; authors: Record<string, string> }) {
  const [tab, setTab] = useState<Tab>("Recommended");

  const visible = works.filter((w) => {
    switch (tab) {
      case "Public Domain":
        return w.isPublicDomain;
      case "Classics":
        return w.firstPublished > 0 && w.firstPublished < 1900;
      case "Philosophy":
        return w.subjects.some((s) => /philosoph/i.test(s));
      case "Literature":
        return w.subjects.some((s) => /literature|fiction|novel|poetry|epic/i.test(s));
      default:
        return true;
    }
  }).slice(0, 10);

  return (
    <section aria-labelledby="foryou-heading" className="mt-10">
      <div className="flex items-baseline justify-between">
        <h2 id="foryou-heading" className="section-title">
          For You
        </h2>
        <ArrowRight />
      </div>

      <div role="tablist" aria-label="Catalogue filters" className="mt-3 flex flex-wrap gap-6 border-b border-ink/40 pb-0">
        {TABS.map((t) => (
          <button
            key={t}
            role="tab"
            aria-selected={tab === t}
            className="tab-link"
            onClick={() => setTab(t)}
          >
            {t}
          </button>
        ))}
      </div>

      <ul className="mt-5 grid grid-cols-2 gap-x-5 gap-y-7 sm:grid-cols-3 lg:grid-cols-5">
        {visible.map((w) => (
          <li key={w.slug}>
            <BookCard work={w} author={authors[w.slug] ?? w.author.name} />
          </li>
        ))}
      </ul>
      {visible.length === 0 ? (
        <p className="pullquote mt-4">Nothing on this shelf yet — the ingestion workers are still shelving.</p>
      ) : null}
    </section>
  );
}
