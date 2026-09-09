/**
 * Engraved line-art vignettes — the botanical woodcut vocabulary of the
 * reference sheet. All stroke-based, all currentColor, all inline SVG:
 * no image assets, no network, no AI-generated raster art.
 *
 * These frame content; they never carry information, so every one is
 * aria-hidden and degrades to nothing for assistive technology.
 */

type V = { className?: string };

/** A leafy sprig: the mark at the head of the left rail and between panels. */
export function Sprig({ className = "" }: V) {
  return (
    <svg viewBox="0 0 64 96" className={`engraving ${className}`} aria-hidden fill="none"
      stroke="currentColor" strokeWidth="1.1" strokeLinecap="round">
      <path d="M32 92 C 31 70, 31 46, 33 10" />
      {/* alternating leaves, each two arcs meeting at a point */}
      <path d="M32 78 C 22 76, 15 70, 13 61 C 22 62, 29 68, 32 78 Z" />
      <path d="M33 66 C 43 64, 50 58, 52 49 C 43 50, 36 56, 33 66 Z" />
      <path d="M32 54 C 23 52, 17 46, 15 38 C 24 39, 30 45, 32 54 Z" />
      <path d="M33 42 C 42 40, 48 34, 50 26 C 41 27, 35 33, 33 42 Z" />
      <path d="M32 30 C 25 28, 20 23, 19 16 C 26 17, 31 22, 32 30 Z" />
      <path d="M33 20 C 39 18, 43 14, 44 8 C 38 9, 34 13, 33 20 Z" />
      {/* veins */}
      <path d="M32 78 C 26 72, 20 67, 15 63 M33 66 C 39 60, 45 55, 50 51 M32 54 C 27 48, 21 43, 17 40 M33 42 C 38 36, 44 31, 48 28" strokeWidth="0.7" />
      {/* berries at the tip */}
      <circle cx="33" cy="8" r="1.6" fill="currentColor" stroke="none" />
      <circle cx="29" cy="12" r="1.2" fill="currentColor" stroke="none" />
      <circle cx="37" cy="12" r="1.2" fill="currentColor" stroke="none" />
    </svg>
  );
}

/** Chapter break: hairline · fleuron · hairline. */
export function RuleWithFleuron({ className = "" }: V) {
  return (
    <div className={`flex items-center gap-3 text-ink ${className}`} aria-hidden>
      <span className="h-px flex-1 bg-ink/60" />
      <svg viewBox="0 0 24 12" width="26" height="13" fill="currentColor">
        <path d="M12 1 L15 6 L12 11 L9 6 Z" />
        <path d="M7 6 C 5 4, 3 4, 1 6 C 3 8, 5 8, 7 6 Z" />
        <path d="M17 6 C 19 4, 21 4, 23 6 C 21 8, 19 8, 17 6 Z" />
      </svg>
      <span className="h-px flex-1 bg-ink/60" />
    </div>
  );
}

/**
 * Classical ruins in line-work: the hero vignette. Fluted columns, a broken
 * entablature, cypress trees, and hatched ground — the reference's engraved
 * landscape, redrawn as vector.
 */
export function RuinsVignette({ className = "" }: V) {
  return (
    <svg viewBox="0 0 320 120" className={`engraving ${className}`} aria-hidden fill="none"
      stroke="currentColor" strokeWidth="0.9" strokeLinecap="round">
      {/* sky hatch */}
      <g strokeWidth="0.4" opacity="0.5">
        <path d="M8 18 h60 M20 26 h70 M6 34 h50 M250 16 h60 M236 24 h70 M252 32 h52" />
      </g>
      {/* distant hills */}
      <path d="M0 74 C 40 66, 70 68, 96 74 M210 72 C 244 62, 284 64, 320 72" strokeWidth="0.7" />
      {/* colonnade */}
      <g>
        <path d="M96 40 h128" strokeWidth="1.4" />
        <path d="M100 44 h120" strokeWidth="0.8" />
        {[104, 128, 152, 176, 200].map((x) => (
          <g key={x}>
            <path d={`M${x} 46 v40 M${x + 9} 46 v40`} />
            <path d={`M${x + 3} 48 v36 M${x + 6} 48 v36`} strokeWidth="0.45" />
            <path d={`M${x - 2} 46 h13 M${x - 2} 86 h13`} strokeWidth="1" />
          </g>
        ))}
        {/* broken entablature to the right */}
        <path d="M212 40 l14 -6 l10 4 l12 -8" strokeWidth="1.2" />
        {/* fallen drum */}
        <ellipse cx="248" cy="88" rx="10" ry="4" />
        <path d="M238 88 h20" strokeWidth="0.6" />
      </g>
      {/* steps */}
      <path d="M92 90 h140 M86 95 h152 M80 100 h164" strokeWidth="0.8" />
      {/* cypresses */}
      <g>
        <path d="M40 96 C 34 84, 34 62, 40 44 C 46 62, 46 84, 40 96 Z" />
        <path d="M40 92 v-40" strokeWidth="0.5" />
        <path d="M60 96 C 56 88, 56 72, 60 58 C 64 72, 64 88, 60 96 Z" />
        <path d="M292 96 C 286 82, 286 58, 292 40 C 298 58, 298 82, 292 96 Z" />
        <path d="M292 92 v-44" strokeWidth="0.5" />
      </g>
      {/* ground hatch */}
      <g strokeWidth="0.4" opacity="0.65">
        <path d="M10 106 h44 M64 108 h52 M128 106 h60 M200 109 h48 M258 106 h52 M24 113 h70 M120 114 h84 M232 113 h64" />
      </g>
    </svg>
  );
}

/** The great tree of the right-rail panel: trunk, forks, clustered foliage. */
export function TreeVignette({ className = "" }: V) {
  const foliage = [
    [40, 26], [52, 20], [66, 22], [78, 30], [86, 42], [82, 54], [68, 60], [54, 62],
    [40, 58], [30, 48], [28, 36], [46, 38], [60, 36], [72, 42], [58, 48], [44, 48],
  ] as const;
  return (
    <svg viewBox="0 0 120 150" className={`engraving ${className}`} aria-hidden fill="none"
      stroke="currentColor" strokeWidth="0.9" strokeLinecap="round">
      {/* trunk and forks */}
      <path d="M58 142 C 57 120, 56 104, 54 88 C 52 74, 50 66, 46 56 M54 88 C 60 76, 66 68, 74 60 M56 100 C 64 94, 72 90, 82 86 M55 108 C 47 102, 40 98, 32 94" />
      <path d="M62 142 C 61 122, 60 106, 58 92" strokeWidth="0.6" />
      {/* foliage as clustered short strokes, woodcut fashion */}
      <g strokeWidth="0.55">
        {foliage.map(([x, y], i) => (
          <path key={i} d={`M${x} ${y} c 3 -3, 7 -3, 10 0 M${x + 1} ${y + 3} c 3 -3, 6 -3, 8 0 M${x - 2} ${y + 6} c 3 -2, 6 -2, 9 0`} />
        ))}
      </g>
      {/* ground and distant mountains */}
      <path d="M18 142 h88" strokeWidth="0.8" />
      <path d="M8 138 C 18 130, 26 130, 34 136 M84 136 C 94 128, 104 128, 114 136" strokeWidth="0.6" />
      <g strokeWidth="0.4" opacity="0.6">
        <path d="M24 146 h28 M60 147 h34" />
      </g>
    </svg>
  );
}

/** An engraved portrait medallion: ring + bust in profile with hatch shade. */
export function PortraitMedallion({ className = "" }: V) {
  return (
    <svg viewBox="0 0 48 48" className={className} aria-hidden fill="none"
      stroke="currentColor" strokeWidth="1">
      <circle cx="24" cy="24" r="22.5" />
      <circle cx="24" cy="24" r="20" strokeWidth="0.5" />
      {/* bust */}
      <path d="M24 10 c 5 0, 8 4, 8 9 c 0 3 -1 5 -2 6 c 2 2, 6 3, 8 6 c 1 2, 2 5, 2 8 h-32 c 0 -3, 1 -6, 2 -8 c 2 -3, 6 -4, 8 -6 c -1 -1 -2 -3 -2 -6 c 0 -5, 3 -9, 8 -9 Z" fill="currentColor" opacity="0.85" stroke="none" />
      {/* hatch over the shoulders */}
      <g strokeWidth="0.4" opacity="0.7">
        <path d="M12 34 l6 -5 M16 36 l7 -6 M21 37 l7 -6 M26 37 l6 -5" stroke="#ECE5D3" />
      </g>
    </svg>
  );
}

/** A club emblem: temple in a ring, per the reference's circle marks. */
export function TempleEmblem({ className = "" }: V) {
  return (
    <svg viewBox="0 0 48 48" className={className} aria-hidden fill="none"
      stroke="currentColor" strokeWidth="1">
      <circle cx="24" cy="24" r="22.5" />
      <circle cx="24" cy="24" r="20" strokeWidth="0.5" />
      <g strokeWidth="0.9">
        <path d="M12 20 L24 13 L36 20 Z" fill="currentColor" opacity="0.85" stroke="none" />
        <path d="M13 21 h22 M14 34 h20 M12 36 h24" />
        <path d="M17 23 v10 M22 23 v10 M27 23 v10 M32 23 v10" />
      </g>
    </svg>
  );
}

/** Small botanical corner flourish for panels. */
export function CornerFlourish({ className = "" }: V) {
  return (
    <svg viewBox="0 0 40 40" className={`engraving ${className}`} aria-hidden fill="none"
      stroke="currentColor" strokeWidth="0.9" strokeLinecap="round">
      <path d="M2 38 C 4 24, 10 12, 24 6 C 30 4, 35 3, 38 3" />
      <path d="M10 30 C 6 28, 4 25, 4 21 C 8 22, 10 25, 10 30 Z" />
      <path d="M16 22 C 13 20, 12 17, 12 13 C 15 14, 17 17, 16 22 Z" />
      <path d="M24 14 C 22 12, 21 9, 21 6 C 24 7, 25 10, 24 14 Z" />
      <circle cx="30" cy="9" r="1.3" fill="currentColor" stroke="none" />
    </svg>
  );
}
