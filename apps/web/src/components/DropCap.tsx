/**
 * DropCap — the illuminated initial, Alexandria's signature ornament.
 *
 * Modeled directly on the woodcut reference: a deep-ink block with a rough
 * hand-printed edge, a vermilion blackletter initial, forest-green vines and
 * gold flowers. Assets are deterministic SVG — each letter always produces
 * the same ornament (seeded by its char code), so this is a reusable asset
 * system, not dynamic generation: the same letter renders identically
 * everywhere, and individual letters can later be replaced by hand-drawn
 * OFL/CC0 artwork without touching call sites.
 */

const VINE_VARIANTS = [
  // Each variant: [path d, mirrored?] gentle curls anchored to block corners.
  "M12 88 C 8 70, 20 62, 14 46 C 10 34, 22 28, 18 14",
  "M10 86 C 22 76, 10 60, 22 48 C 32 38, 20 26, 30 16",
  "M14 90 C 6 72, 26 66, 16 50 C 8 38, 26 34, 16 18",
];

const SPRIG_VARIANTS = [
  "M88 14 C 80 26, 92 34, 84 46 C 78 56, 90 64, 84 78",
  "M86 12 C 92 28, 78 36, 86 50 C 92 60, 80 70, 88 82",
];

function leaf(x: number, y: number, angle: number, key: string) {
  return (
    <path
      key={key}
      d="M0 0 C 4 -6, 10 -6, 12 0 C 10 6, 4 6, 0 0 Z"
      fill="#1B5B42"
      transform={`translate(${x} ${y}) rotate(${angle}) scale(0.9)`}
    />
  );
}

function flower(x: number, y: number, scale: number, key: string) {
  const petals = [0, 72, 144, 216, 288].map((a) => (
    <ellipse key={a} cx="0" cy="-4.2" rx="2.6" ry="4.2" fill="#D4A62A" transform={`rotate(${a})`} />
  ));
  return (
    <g key={key} transform={`translate(${x} ${y}) scale(${scale})`}>
      {petals}
      <circle r="2.2" fill="#D9471F" />
    </g>
  );
}

export function DropCap({
  letter,
  size = 96,
  className = "",
}: {
  letter: string;
  size?: number;
  className?: string;
}) {
  const ch = letter.slice(0, 1).toUpperCase();
  const seed = ch.charCodeAt(0);
  const vine = VINE_VARIANTS[seed % VINE_VARIANTS.length];
  const sprig = SPRIG_VARIANTS[seed % SPRIG_VARIANTS.length];
  const dots = Array.from({ length: 7 }, (_, i) => ({
    cx: 12 + ((seed * (i + 3)) % 76),
    cy: 10 + ((seed * (i + 7) * 13) % 80),
    r: 0.9 + ((seed + i) % 3) * 0.35,
  }));

  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 100 100"
      role="img"
      aria-label={`Illuminated initial ${ch}`}
      className={className}
    >
      <defs>
        <filter id={`rough-${seed}`} x="-4%" y="-4%" width="108%" height="108%">
          <feTurbulence type="fractalNoise" baseFrequency="0.12" numOctaves="2" seed={seed} result="n" />
          <feDisplacementMap in="SourceGraphic" in2="n" scale="3.2" />
        </filter>
      </defs>

      {/* Ink block with hand-printed edge */}
      <rect x="3" y="3" width="94" height="94" fill="#111713" filter={`url(#rough-${seed})`} />
      {/* Inner hairline frame */}
      <rect x="8.5" y="8.5" width="83" height="83" fill="none" stroke="#E8DDC4" strokeWidth="0.8" opacity="0.5" />

      {/* Botanical ornament */}
      <g fill="none" stroke="#1B5B42" strokeWidth="2.4" strokeLinecap="round">
        <path d={vine} />
        <path d={sprig} />
      </g>
      {leaf(16, 62, -30 + (seed % 40), "l1")}
      {leaf(20, 34, 20 + (seed % 30), "l2")}
      {leaf(84, 40, 140 + (seed % 30), "l3")}
      {leaf(82, 68, 200 - (seed % 40), "l4")}
      {flower(18, 20, 0.9, "f1")}
      {flower(84, 24, 0.75, "f2")}
      {flower(16, 82, 0.7, "f3")}

      {/* Speckle — ink flecks of the press */}
      <g fill="#E8DDC4" opacity="0.35">
        {dots.map((d, i) => (
          <circle key={i} cx={d.cx} cy={d.cy} r={d.r} />
        ))}
      </g>

      {/* The initial itself */}
      <text
        x="50"
        y="54"
        textAnchor="middle"
        dominantBaseline="central"
        fontFamily="var(--font-blackletter), serif"
        fontSize="60"
        fill="#D9471F"
        stroke="#E84B1C"
        strokeWidth="0.5"
      >
        {ch}
      </text>
    </svg>
  );
}

/**
 * IlluminatedParagraph — prose with a drop cap occupying ~3 lines,
 * exactly as on a manuscript folio.
 */
export function IlluminatedParagraph({ text, className = "" }: { text: string; className?: string }) {
  const first = text.slice(0, 1);
  const rest = text.slice(1);
  return (
    <p className={`text-[1.05rem] leading-relaxed text-ink-soft ${className}`}>
      <span className="float-left mr-3 mt-1">
        <DropCap letter={first} size={84} />
      </span>
      {rest}
    </p>
  );
}
