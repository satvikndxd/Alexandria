# 01 — Product Vision

**Status:** living document · owner: product · last reviewed 2026-09-09

## The sentence
Alexandria is an open-source, ad-free, human-first social reading platform:
*Letterboxd for books*, elevated with scholarly rigor and public-domain
accessibility.

## The philosophy
Books are written by humans, discussed by humans, explained by humans, and
read by humans. Every product decision is tested against that sentence:

- **No generative AI literary content.** No machine-written summaries,
  reviews, criticism, or "scholar" notes. AI may assist infrastructure
  (spam heuristics, duplicate detection) and must never impersonate a reader.
  Enforced culturally and structurally — see [ADR 0005](../adr/0005-friction-not-detection.md)
  and [17 — Moderation & Trust](17-moderation-trust.md).
- **No advertising.** Zero display ads, zero sponsored placement, zero
  tracking-based revenue. Monetization is a single, disclosed, opt-in surface:
  affiliate purchase links on an explicit *Purchase options* page
  (`apps/web/src/app/books/[slug]/purchase/`).
- **No engagement engineering.** Feeds are chronological; recommendations are
  explainable co-shelving counts; follower counts are set in small type at the
  foot of a profile. There is no score column anywhere in the schema
  (`packages/db/migrations/`) that a growth team could later tune.

## What Alexandria is not
Not a store. Not an audiobook host (initially). Not a summary mill. Not a
Discord clone that happens to mention books. Not a dashboard.

## The test we apply to every feature
> "Would this look like Alexandria?"

If a proposed component would feel at home in a generic SaaS template — pills,
glass, purple gradients, infinite scroll — it is redesigned until it belongs
on the folio: ink on parchment, ruled edges, engraved botanicals, sharp
corners. See [07 — Visual Design System](07-visual-design-system.md).

## Success metrics (and the metrics we refuse)
We measure: reading completion rates, thread depth in clubs, scholar-note
citations, public-domain reads, review substance (median characters).
We do **not** measure: DAU, session length, scroll depth, notification
click-through. A metric we would be embarrassed to publish in our README is a
metric we do not collect.
