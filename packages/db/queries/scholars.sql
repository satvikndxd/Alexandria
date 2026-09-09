-- Scholar system: verified contributors, peer-reviewed notes, citations.
--
-- Two invariants the schema and these queries enforce together:
--   1. a published note has at least one citation
--   2. a note reaches `published` only after two approvals from distinct
--      verified scholars, and never from its own author

-- name: UpsertScholarProfile :one
INSERT INTO scholar_profiles (user_id, field, affiliation, orcid, coi_statement, status)
VALUES (@user_id, @field, sqlc.narg('affiliation'), sqlc.narg('orcid'), @coi_statement, 'pending')
ON CONFLICT (user_id) DO UPDATE SET
  field         = EXCLUDED.field,
  affiliation   = EXCLUDED.affiliation,
  orcid         = EXCLUDED.orcid,
  coi_statement = EXCLUDED.coi_statement
RETURNING *;

-- name: GetScholarProfile :one
SELECT * FROM scholar_profiles WHERE user_id = @user_id;

-- name: SetScholarStatus :execrows
UPDATE scholar_profiles
   SET status = @status,
       verified_by = CASE WHEN @status = 'verified' THEN sqlc.narg('verified_by') ELSE verified_by END,
       verified_at = CASE WHEN @status = 'verified' THEN now() ELSE verified_at END
 WHERE user_id = @user_id;

-- name: ListPendingScholarProfiles :many
SELECT sp.*, u.username, u.email
  FROM scholar_profiles sp JOIN users u ON u.id = sp.user_id
 WHERE sp.status = 'pending'
 ORDER BY sp.created_at ASC
 LIMIT @lim OFFSET @off;

-- name: IsVerifiedScholar :one
SELECT EXISTS (SELECT 1 FROM scholar_profiles
                WHERE user_id = @user_id AND status = 'verified') AS verified;

-- name: CreateScholarNote :one
INSERT INTO scholar_notes (work_id, author_id, chapter_ref, anchor_quote, title, body, kind, status, is_community)
VALUES (@work_id, @author_id, @chapter_ref, @anchor_quote, @title, @body, @kind,
        @status, @is_community)
RETURNING *;

-- name: GetScholarNote :one
SELECT n.*, u.username, p.display_name,
       sp.field, sp.affiliation, sp.status AS scholar_status,
       w.slug AS work_slug, w.title AS work_title
  FROM scholar_notes n
  JOIN users u ON u.id = n.author_id
  LEFT JOIN profiles p ON p.user_id = u.id
  LEFT JOIN scholar_profiles sp ON sp.user_id = n.author_id
  JOIN works w ON w.id = n.work_id
 WHERE n.id = @id;

-- name: ListNotesForWork :many
-- Readers see published notes, plus their own drafts so an author can resume
-- editing. Community-contributor notes are returned with is_community set, and
-- the UI labels them differently from verified-scholar notes — no elitism, but
-- no false equivalence either.
SELECT n.*, u.username, p.display_name,
       sp.field, sp.affiliation, sp.status AS scholar_status,
       (SELECT count(*)::bigint FROM note_citations c WHERE c.note_id = n.id) AS citation_count
  FROM scholar_notes n
  JOIN users u ON u.id = n.author_id
  LEFT JOIN profiles p ON p.user_id = u.id
  LEFT JOIN scholar_profiles sp ON sp.user_id = n.author_id
 WHERE n.work_id = @work_id
   AND (n.status = 'published'
        OR (sqlc.narg('viewer_id')::uuid IS NOT NULL AND n.author_id = sqlc.narg('viewer_id')
            AND n.status IN ('draft', 'in_review')))
   AND (sqlc.narg('chapter_ref')::text IS NULL OR n.chapter_ref = sqlc.narg('chapter_ref'))
 ORDER BY n.chapter_ref, n.created_at DESC
 LIMIT @lim OFFSET @off;

-- name: UpdateScholarNote :one
UPDATE scholar_notes
   SET title = @title, body = @body, chapter_ref = @chapter_ref,
       anchor_quote = @anchor_quote, kind = @kind,
       version = scholar_notes.version + 1
 WHERE id = @id AND author_id = @author_id
RETURNING *;

-- name: SetNoteStatus :execrows
UPDATE scholar_notes SET status = @status WHERE id = @id;

-- name: RecordNoteRevision :exec
-- Append-only history: every edit is recoverable, which is what makes
-- retracting or disputing a scholarly claim possible.
INSERT INTO note_revisions (note_id, version, title, body, edited_by)
VALUES (@note_id, @version, @title, @body, sqlc.narg('edited_by'));

-- name: ListNoteRevisions :many
SELECT * FROM note_revisions WHERE note_id = @note_id ORDER BY version DESC;

-- name: AddNoteCitation :exec
INSERT INTO note_citations (note_id, citation, url, position)
VALUES (@note_id, @citation, sqlc.narg('url'), @position);

-- name: ListNoteCitations :many
SELECT * FROM note_citations WHERE note_id = @note_id ORDER BY position, citation;

-- name: DeleteNoteCitation :execrows
DELETE FROM note_citations WHERE id = @id AND note_id = @note_id;

-- name: CountNoteCitations :one
SELECT count(*) FROM note_citations WHERE note_id = @note_id;

-- name: ReviewNote :exec
INSERT INTO note_reviews (note_id, reviewer_id, approved, comments)
VALUES (@note_id, @reviewer_id, @approved, @comments)
ON CONFLICT (note_id, reviewer_id) DO UPDATE SET
  approved = EXCLUDED.approved, comments = EXCLUDED.comments;

-- name: CountNoteApprovals :one
-- Distinct verified scholars, excluding the author: the peer-review gate.
SELECT count(*) FROM note_reviews nr
  JOIN scholar_profiles sp ON sp.user_id = nr.reviewer_id AND sp.status = 'verified'
 WHERE nr.note_id = @note_id AND nr.approved AND nr.reviewer_id <> @author_id;

-- name: ListNoteReviews :many
SELECT nr.*, u.username, sp.field
  FROM note_reviews nr
  JOIN users u ON u.id = nr.reviewer_id
  LEFT JOIN scholar_profiles sp ON sp.user_id = nr.reviewer_id
 WHERE nr.note_id = @note_id
 ORDER BY nr.created_at;

-- name: ListNotesAwaitingReview :many
SELECT n.*, u.username, w.title AS work_title, w.slug AS work_slug
  FROM scholar_notes n
  JOIN users u ON u.id = n.author_id
  JOIN works w ON w.id = n.work_id
 WHERE n.status = 'in_review'
 ORDER BY n.created_at ASC
 LIMIT @lim OFFSET @off;
