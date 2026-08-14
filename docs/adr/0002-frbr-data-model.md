# ADR 0002 — FRBR bibliographic model (Work → Edition)

**Status:** Accepted · 2026-08-14

## Context
"A book" is ambiguous. *Crime and Punishment* is one literary creation with
hundreds of ISBNs, translations, and a Project Gutenberg text. Goodreads'
edition-duplication mess is a direct result of flattening this.

## Decision
Follow the Functional Requirements for Bibliographic Records:

- **`works`** — the abstract creation. Reviews, ratings, scholar notes, and
  club reads attach here, so a review of the Penguin paperback and one of the
  Gutenberg text aggregate together.
- **`editions`** — concrete manifestations (ISBN, format, publisher,
  Gutenberg etext number). Covers, page counts, and reading progress attach
  here. Flexible per-edition facts (translator, binding) live in `metadata
  JSONB`.

External identity: `works.openlibrary_id` (OL…W) and `editions.openlibrary_id`
(OL…M) + `external_identifiers` for everything else. Ingestion upserts on
these keys, which is what makes the event pipeline idempotent.

## Consequences
- No duplicate works; editions dedupe on ISBN-13 / Gutenberg ID.
- Slightly more complex queries (a work page joins editions + covers), paid
  once in SQL and amortized forever in data quality.
