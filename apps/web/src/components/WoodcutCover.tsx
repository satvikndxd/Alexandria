import type { Work } from "@/lib/data";
import { DropCap } from "./DropCap";

/**
 * WoodcutCover — a license-safe generated cover in the house style.
 *
 * Alexandria never hotlinks cover images whose license is unverified.
 * When an edition has no rights-cleared cover asset, the platform renders
 * this typographic woodcut plate instead: parchment field, double frame,
 * illuminated initial, letterpress title. (The production pipeline swaps in
 * cached Open Library / Wikimedia covers once their license row clears.)
 */

const HUES: Record<Work["coverHue"], { field: string; frame: string; accent: string }> = {
  vermilion: { field: "#D9471F", frame: "#111713", accent: "#F0E7D2" },
  botanical: { field: "#0D3B2E", frame: "#111713", accent: "#E8DDC4" },
  gold: { field: "#C79522", frame: "#111713", accent: "#111713" },
  ink: { field: "#111713", frame: "#0D3B2E", accent: "#E8DDC4" },
};

export function WoodcutCover({ work, className = "" }: { work: Work; className?: string }) {
  const hue = HUES[work.coverHue];
  const initial = work.title.replace(/^(The|A|An)\s+/i, "").slice(0, 1);

  return (
    <div
      className={`relative aspect-[2/3] w-full select-none overflow-hidden border-2 border-ink ${className}`}
      style={{ backgroundColor: hue.field }}
      role="img"
      aria-label={`Generated cover plate for ${work.title}`}
    >
      {/* double frame */}
      <div className="absolute inset-2 border" style={{ borderColor: hue.accent, opacity: 0.9 }} />
      <div className="absolute inset-3 border" style={{ borderColor: hue.accent, opacity: 0.5 }} />

      <div className="relative flex h-full flex-col items-center justify-between px-3 py-5 text-center">
        <p
          className="text-[0.5rem] uppercase tracking-engraved"
          style={{ color: hue.accent, opacity: 0.85 }}
        >
          {work.isPublicDomain ? "Public Domain" : "Edition"}
        </p>

        <div className="flex flex-col items-center gap-2">
          <DropCap letter={initial} size={64} />
          <h3
            className="font-body text-sm font-semibold uppercase leading-snug tracking-[0.12em]"
            style={{ color: hue.accent }}
          >
            {work.title}
          </h3>
        </div>

        <div className="flex flex-col items-center gap-1">
          <span className="block h-px w-10" style={{ backgroundColor: hue.accent, opacity: 0.7 }} />
          <p className="font-body text-[0.65rem] italic" style={{ color: hue.accent, opacity: 0.9 }}>
            {work.author.name}
          </p>
        </div>
      </div>
    </div>
  );
}
