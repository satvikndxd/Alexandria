import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { getWork } from "@/lib/data";
import { SectionHeading } from "@/components/Ornament";

export const metadata: Metadata = { title: "Purchase Options" };

/**
 * The purchase flow is deliberately Book → Purchase Options → Vendor.
 * Amazon never appears inline in reading or discovery surfaces; affiliate
 * disclosure is shown before any outbound link.
 */
export default function PurchasePage({ params }: { params: { slug: string } }) {
  const work = getWork(params.slug);
  if (!work) notFound();

  return (
    <div className="mx-auto max-w-xl space-y-8">
      <SectionHeading caption={work.title} title="Purchase Options" />

      <p className="text-[0.97rem] leading-relaxed text-ink-soft">
        Alexandria shows purchase links only when you ask for them. Consider your local bookshop
        and public library first — then, if it helps, the links below support the platform.
      </p>

      <ul className="space-y-4">
        {work.isPublicDomain && work.gutenbergId && (
          <li className="manuscript-card p-4">
            <p className="engraved-label">Free · Public Domain</p>
            <a href={`https://www.gutenberg.org/ebooks/${work.gutenbergId}`} className="mt-1 block font-body text-lg font-semibold text-botanical underline underline-offset-4">
              Read on Project Gutenberg
            </a>
          </li>
        )}
        <li className="manuscript-card p-4">
          <p className="engraved-label">Independent bookshops</p>
          <span className="mt-1 block font-body text-lg font-semibold text-ink">Bookshop.org (affiliate) — coming soon</span>
        </li>
        <li className="manuscript-card p-4">
          <p className="engraved-label">Amazon (affiliate)</p>
          <span className="mt-1 block font-body text-lg font-semibold text-ink">Amazon — link generated per edition and region</span>
          <p className="mt-2 text-xs italic text-ink-faint">
            Disclosure: as an Amazon Associate, Alexandria earns from qualifying purchases.
          </p>
        </li>
      </ul>

      <p>
        <Link href={`/books/${work.slug}`} className="text-sm italic text-botanical underline underline-offset-4">
          ← Back to the work
        </Link>
      </p>
    </div>
  );
}
