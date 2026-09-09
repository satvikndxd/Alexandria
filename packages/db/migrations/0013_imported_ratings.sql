-- Alexandria · Migration 0013
-- Imported ratings that carry no qualifying review.
--
-- Alexandria ties a public rating to a considered review (one opinion per
-- reader per work, >=150 characters). Importers must not invent review prose
-- to satisfy that, so a rating arriving without qualifying text lands on the
-- shelf item instead: it is the reader's own rating, kept privately on their
-- shelf, promotable to a review whenever they choose to write one.
--
-- Work-level aggregates remain review-driven on purpose: a public average is
-- a public claim, and only considered opinions make it.

ALTER TABLE shelf_items
  ADD COLUMN rating integer CHECK (rating BETWEEN 1 AND 10);

-- Import provenance: where a shelf item or annotation came from, so a reader
-- (or a moderator) can always tell imported history from Alexandria history.
ALTER TABLE shelf_items
  ADD COLUMN imported_from text;
ALTER TABLE annotations
  ADD COLUMN imported_from text;

-- Import throttling reuses the auth_attempts bucket table (hashed key,
-- trailing window) with its own kind: a flood of imports is the same shape of
-- abuse as a flood of logins, and gets the same structural answer.
ALTER TYPE auth_challenge_kind ADD VALUE IF NOT EXISTS 'import';
