# 06 — UX Architecture

**Status:** implemented in `apps/web` (16 routes) · reference sheet is the source of truth

## The folio
One composition, three columns, inside a double-ruled sheet frame
(`components/frame/PageFrame.tsx`):

```
┌─ sheet frame ────────────────────────────────────────────────┐
│ rail 172px │ reading column                    │ rail 268px  │
│ sprig      │ masthead: ALEXANDRIA + epigraph   │ streak      │
│ nav (ink   │ ─ hairline ─                      │ year totals │
│  block =  │ greeting hero + ruins vignette    │ circles     │
│  active)  │ continue reading (cover, %, note) │ tree+quote  │
│ motto      │ for-you filters · activity        │             │
└──────────────────────────────────────────────────────────────┘
```

- **Left rail**: Home, Discover, My Library, Notes, Communities, Search.
  Active item is a solid ink block (never a pill, never a colour wash).
  Foot: sprig + "SLOW READING BRIGHTER MINDS" in stacked capitals.
- **Right rail** (≥1280px): the reader's own numbers only — streak squares,
  year books/pages/notes, circles, botanical quote panel. Anonymous visitors
  get zeros and circles, never fabricated figures.
- **<1024px**: rail collapses to a five-item bottom bar; right rail yields its
  space to the reading column. We do not stretch the mobile layout onto
  desktop, nor bury desktop information on mobile.

## Information density
Metadata-dense where a scholar wants it (book page: editions table with
license lines, rating histogram in half-star rows, provenance labels on
notes), spacious where a reader wants it (review bodies at ~68ch, pull-quotes
in italic garamond).

## Friction as interface
The review form shows its rules before submission: live rune counter against
150, filler warning, the "why this rating?" prompt, and the daily-cap sentence.
Friction you can see is respect; friction you cannot is a trap.

## Navigation semantics
Every list is paginated with visible caps (no infinite scroll). Feeds are
chronological and say so. Degraded states are labelled: search answers
`source: postgres_fallback` when Meilisearch is cold, and the UI prints
"index degraded, catalogue scan" rather than pretending.

## Empty states
Written as invitations, never as dead ends: "Nothing is open on your desk…"
with a path forward. Seeded demonstration data is always labelled
"Demonstration seeds" so the platform never lies about its own liveliness.
