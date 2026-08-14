-- Alexandria · Migration 0005
-- Reviews with structural anti-slop friction enforced AT THE DATABASE LAYER:
--   * ratings are half-stars stored as integers 1..10 (0.5..5.0 stars)
--   * review body must be >= 150 characters (enforced by CHECK)
--   * one review per user per work (rereads update the review)
-- Application layer adds per-account rate limits on top.

CREATE TABLE reviews (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  work_id      uuid NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  edition_id   uuid REFERENCES editions(id) ON DELETE SET NULL,
  rating       integer NOT NULL CHECK (rating BETWEEN 1 AND 10),  -- half-stars
  title        text NOT NULL DEFAULT '' CHECK (length(title) <= 200),
  body         text NOT NULL CHECK (length(body) >= 150 AND length(body) <= 20000),
  has_spoilers boolean NOT NULL DEFAULT false,
  -- structured prompts; nudge substance over hot takes
  prompt_why   text NOT NULL DEFAULT '',           -- "Why did you rate it this way?"
  like_count   integer NOT NULL DEFAULT 0,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,
  UNIQUE (user_id, work_id)
);
CREATE TRIGGER reviews_updated_at BEFORE UPDATE ON reviews
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX reviews_work_idx ON reviews (work_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX reviews_user_idx ON reviews (user_id, created_at DESC) WHERE deleted_at IS NULL;

CREATE TABLE review_likes (
  review_id  uuid NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (review_id, user_id)
);

CREATE TABLE review_comments (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  review_id  uuid NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  body       text NOT NULL CHECK (length(body) BETWEEN 2 AND 5000),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE TRIGGER review_comments_updated_at BEFORE UPDATE ON review_comments
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX review_comments_review_idx ON review_comments (review_id, created_at);

-- Keep work rating aggregates in sync without full-table scans.
CREATE OR REPLACE FUNCTION apply_review_rating() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'INSERT' THEN
    UPDATE works SET rating_sum = rating_sum + NEW.rating, rating_count = rating_count + 1
      WHERE id = NEW.work_id;
  ELSIF TG_OP = 'UPDATE' AND NEW.deleted_at IS NULL AND OLD.deleted_at IS NULL THEN
    UPDATE works SET rating_sum = rating_sum - OLD.rating + NEW.rating WHERE id = NEW.work_id;
  ELSIF TG_OP = 'UPDATE' AND NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL THEN
    UPDATE works SET rating_sum = rating_sum - OLD.rating, rating_count = rating_count - 1
      WHERE id = NEW.work_id;
  ELSIF TG_OP = 'DELETE' AND OLD.deleted_at IS NULL THEN
    UPDATE works SET rating_sum = rating_sum - OLD.rating, rating_count = rating_count - 1
      WHERE id = OLD.work_id;
  END IF;
  RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER reviews_rating_aggregate
  AFTER INSERT OR UPDATE OR DELETE ON reviews
  FOR EACH ROW EXECUTE FUNCTION apply_review_rating();
