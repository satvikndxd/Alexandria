import Link from "next/link";
import { PageFrame } from "@/components/frame/PageFrame";
import { DropCap } from "@/components/DropCap";

/** The lost-folio page: an illuminated initial, a line of apology, a way home. */
export default function NotFound() {
  return (
    <PageFrame pathname="" epigraph="Even the great library lost scrolls." attribution="Alexandria">
      <div className="flex flex-col items-center py-16 text-center">
        <DropCap letter="L" size={112} />
        <h1 className="mt-5 text-[1.8rem] text-ink">Lost Folio</h1>
        <p className="pullquote mt-3 max-w-[46ch]">
          The shelf you asked for is not in this catalogue — mis-shelved, mis-copied, or never
          ingested. Nothing is lost that a search cannot find.
        </p>
        <div className="mt-6 flex gap-3">
          <Link href="/" className="btn-solid">
            Return to the Atrium
          </Link>
          <Link href="/search" className="btn-print">
            Search the catalogue
          </Link>
        </div>
      </div>
    </PageFrame>
  );
}
