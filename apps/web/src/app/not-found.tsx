import Link from "next/link";
import { DropCap } from "@/components/DropCap";

export default function NotFound() {
  return (
    <div className="flex flex-col items-center py-24 text-center">
      <DropCap letter="L" size={120} />
      <h2 className="mt-6 font-display text-4xl text-ink">Lost Folio</h2>
      <p className="mt-3 max-w-md italic text-ink-soft">
        This page is not in the catalogue. Perhaps it burned with the first Alexandria — or perhaps
        the shelfmark was mistyped.
      </p>
      <Link
        href="/"
        className="mt-8 border-2 border-ink px-5 py-2 text-sm uppercase tracking-engraved text-ink hover:bg-ink hover:text-parchment-light"
      >
        Return to the Atrium
      </Link>
    </div>
  );
}
