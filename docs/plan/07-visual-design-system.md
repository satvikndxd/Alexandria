# 07 — Visual Design System

**Status:** implemented · tokens in `apps/web/tailwind.config.ts`, sheet in
`apps/web/src/app/globals.css`, engravings in `src/components/Engravings.tsx`

## Palette (flat ink, flat colour; texture from paper grain only)
| Token | Hex | Use |
|---|---|---|
| parchment.page | #ECE5D3 | sheet ground |
| parchment.panel | #F0EAD9 | panels, fields |
| parchment / light / dark / deep | #E8DDC4 / #F0E7D2 / #D9CCAE / #C9B992 | surfaces, covers |
| ink / soft / faint | #171A14 / #3C4036 / #6B6F62 | text, rules |
| sepia | #5F584A | engraved line-art |
| botanical | #0D3B2E | verified scholars, quiet accents |
| vermilion | #D9471F | the reader's own marks: ratings, spoiler labels, selection |
| gold | #C79522 | community-contributor provenance, illumination |

Vermilion and gold are **reserve colours**: chrome is monochrome ink; colour
appears where a human made a mark. The reference sheet is nearly monochrome
and we keep it so.

## Typography
- **EB Garamond** everywhere: body, and (as tracked capitals) the wordmark and
  section titles — exactly as the reference sets them. Old-style numerals for
  statistics (`.oldstyle`).
- **UnifrakturMaguntia** for illuminated initials and drop caps only; never
  running text.
- Engraved labels: 0.66rem capitals at 0.22em tracking, ink-faint.

## Geometry & depth
- Radii: 0 and 2px only. Borders: 1px hairlines, 1.5px panels, 2px strong
  frames. No soft shadows: depth is the hard offset `4px 4px 0 0 ink`
  ("print-block"), used on hover lift only.
- Rules: hairline, double-rule (title-page pair), fleuron chapter breaks.

## Components (named, reviewed against the reference)
sheet-frame · rail-link (ink block active) · panel / panel-strong ·
progress-track (ruled track, solid ink fill) · streak-cell (filled/empty/
hatched-today) · stat-col (vertical hairlines) · medallion (portrait & temple
emblems) · cover-plate · btn-print / btn-solid (invert on hover, translate on
press) · field · tab-link (single ink underline) · pullquote · spoiler-veil ·
manuscript-card.

## Engraved iconography
All icons and vignettes are inline stroke-based SVG (`Engravings.tsx`,
`Ornament.tsx`): sprigs, fleurons, ruins vignette, great tree, portrait
medallions, temple emblems, and the line-icon set (columns, compass, folio,
quill, reader, laurel). No emoji, no icon font, no raster, no network.
Deterministic drop caps (`DropCap.tsx`) remain swappable for hand-drawn
OFL/CC0 initials without touching call sites.

## Accessibility is not negotiable
Visible focus = 2px ink outline; `prefers-reduced-motion` kills transitions;
spoiler veils are keyboard-activatable with `role="button"`; engravings are
`aria-hidden`; progress tracks carry `role="progressbar"` with real values;
colour is never the only channel (spoilers also carry a label).
