/**
 * @alexandria/core — the single source of truth for constants shared by
 * the web client, the design system, and (via codegen) the Flutter app.
 * The Go backend mirrors these in internal/domain; a CI check (planned)
 * diffs the two.
 */

export const tokens = {
  color: {
    parchment: "#E8DDC4",
    parchmentLight: "#F0E7D2",
    parchmentDark: "#D9CCAE",
    ink: "#111713",
    inkSoft: "#2A322B",
    inkFaint: "#4A5348",
    botanical: "#0D3B2E",
    botanicalMid: "#124A38",
    botanicalLight: "#1B5B42",
    vermilion: "#D9471F",
    vermilionBright: "#E84B1C",
    gold: "#C79522",
    goldBright: "#D4A62A",
  },
  radius: { none: 0, nick: 2 },
  shadow: { block: "4px 4px 0 0 #111713" },
} as const;

/** Anti-slop friction — mirrored by apps/api/internal/domain.FrictionPolicy. */
export const friction = {
  reviewMinChars: 150,
  reviewMaxChars: 20000,
  newAccountDailyReviews: 2,
  trustedDailyReviews: 10,
  trustedReputation: 100,
  scholarNoteMinChars: 200,
} as const;

/** Ratings are half-stars stored as integers 1..10. */
export const rating = {
  min: 1,
  max: 10,
  toStars: (halfStars: number): number => halfStars / 2,
  fromStars: (stars: number): number => Math.round(stars * 2),
} as const;

export type ShelfKind = "want_to_read" | "reading" | "read" | "dnf" | "favorites" | "custom";
export type ReadFormat = "physical" | "ebook" | "audiobook" | "public_domain" | "library_copy" | "other";
export type NoteKind = "context" | "linguistic" | "historical" | "interpretive" | "textual";
export type CoverLicense =
  | "public_domain"
  | "cc0"
  | "cc_by"
  | "cc_by_sa"
  | "fair_use_thumbnail"
  | "user_upload"
  | "unknown";
