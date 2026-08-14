/** Ornamental hardware: dividers, fleurons, section headings, engraved icons. */

export function Fleuron({ size = 18, className = "" }: { size?: number; className?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden className={className}>
      <g fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round">
        <path d="M12 3 C 12 8, 12 16, 12 21" />
        <path d="M12 8 C 8 6, 5 8, 4 12 C 7 12, 10 11, 12 8 Z" fill="currentColor" stroke="none" />
        <path d="M12 8 C 16 6, 19 8, 20 12 C 17 12, 14 11, 12 8 Z" fill="currentColor" stroke="none" />
        <path d="M12 14 C 9 13, 7 14, 6 17 C 9 17, 11 16, 12 14 Z" fill="currentColor" stroke="none" />
        <path d="M12 14 C 15 13, 17 14, 18 17 C 15 17, 13 16, 12 14 Z" fill="currentColor" stroke="none" />
      </g>
    </svg>
  );
}

/** Divider — hairline · fleuron · hairline, the classic chapter break. */
export function Divider({ className = "" }: { className?: string }) {
  return (
    <div className={`flex items-center gap-4 text-botanical ${className}`} aria-hidden>
      <span className="h-px flex-1 bg-ink/60" />
      <Fleuron />
      <span className="h-px flex-1 bg-ink/60" />
    </div>
  );
}

/** SectionHeading — engraved caption over a blackletter title, ruled beneath. */
export function SectionHeading({
  caption,
  title,
  className = "",
}: {
  caption: string;
  title: string;
  className?: string;
}) {
  return (
    <header className={className}>
      <p className="engraved-label">{caption}</p>
      <h2 className="mt-1 font-display text-3xl text-ink">{title}</h2>
      <div className="double-rule mt-3" aria-hidden />
    </header>
  );
}

/* ————— Engraved line icons (never emojis) ————— */

type IconProps = { size?: number; className?: string };

const iconBase = (props: IconProps) => ({
  width: props.size ?? 20,
  height: props.size ?? 20,
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.5,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
  "aria-hidden": true,
});

export function IconLibrary(p: IconProps) {
  return (
    <svg {...iconBase(p)} className={p.className}>
      <path d="M4 20 V6 l3 -2 v16 M10 20 V4 h4 v16 M17 20 l2.5 -15 2 .4 L19 20.2" />
      <path d="M2 20.5 h20" />
    </svg>
  );
}

export function IconCompass(p: IconProps) {
  return (
    <svg {...iconBase(p)} className={p.className}>
      <circle cx="12" cy="12" r="9" />
      <path d="M15.5 8.5 L13.2 13.2 L8.5 15.5 L10.8 10.8 Z" fill="currentColor" stroke="none" />
      <circle cx="12" cy="12" r="1" fill="currentColor" stroke="none" />
    </svg>
  );
}

export function IconFolio(p: IconProps) {
  return (
    <svg {...iconBase(p)} className={p.className}>
      <path d="M12 5 C 9.5 3.5, 6 3.5, 4 4.5 V19 c2 -1, 5.5 -1, 8 .5 c2.5 -1.5, 6 -1.5, 8 -.5 V4.5 c-2 -1, -5.5 -1, -8 .5 Z" />
      <path d="M12 5 V19.5" />
    </svg>
  );
}

export function IconColumns(p: IconProps) {
  return (
    <svg {...iconBase(p)} className={p.className}>
      <path d="M3 20.5 h18 M4.5 18 h15 M12 4 l7 3.5 H5 Z" />
      <path d="M7 10.5 V18 M12 10.5 V18 M17 10.5 V18 M5.5 10.5 h13" />
    </svg>
  );
}

export function IconQuill(p: IconProps) {
  return (
    <svg {...iconBase(p)} className={p.className}>
      <path d="M20 4 C 14 4, 8 8, 6.5 14 L 5 19 l5 -1.5 C 16 16, 20 10, 20 4 Z" />
      <path d="M5 19 C 9 13, 13 9, 17 6.5" />
    </svg>
  );
}

export function IconReader(p: IconProps) {
  return (
    <svg {...iconBase(p)} className={p.className}>
      <circle cx="12" cy="8" r="3.5" />
      <path d="M5 20 c0 -4, 3 -6.5, 7 -6.5 s7 2.5, 7 6.5" />
    </svg>
  );
}

export function IconLaurel(p: IconProps) {
  return (
    <svg {...iconBase(p)} className={p.className}>
      <path d="M12 21 C 7 17, 5.5 11, 7 4 C 10 7, 11.5 12, 12 21 Z" />
      <path d="M12 21 C 17 17, 18.5 11, 17 4 C 14 7, 12.5 12, 12 21 Z" />
    </svg>
  );
}
