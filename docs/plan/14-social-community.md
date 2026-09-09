# 14 — Social & Community Architecture

**Status:** implemented (graph, feed, notifications, clubs text) · voice/video ⏳

## The graph
Follows are the entire ranking signal. `follows(follower_id, followee_id)`
with a self-follow CHECK; blocks dissolve follow edges in both directions and
exclude the blocked actor from feeds in both directions (tested).

## Feed = chronology, not optimisation
`GET /v1/feed`: UNION over followed accounts' reviews and **public**
shelf-adds, `ORDER BY created_at DESC`, keyset cursor. No score column exists
to tune. `GET /v1/community-feed` is the logged-out view: newest substantive
non-spoiler reviews platform-wide, labelled as such.

## Recommendations that can explain themselves
"Readers of this also shelved": co-shelving counts weighted by inverse
popularity (`ListCoOccurringWorks`), private shelves excluded from the
statistic, and the UI prints the explanation ("co-shelving counts, weighted
against popularity"). Collaborative filtering over co-occurrence is
infrastructure; it is not a behavioural model of the reader.

## Indie authors
Author records are claimable (`authors.is_claimed`, `claimed_by`); claimed
profiles get bio and links. Discovery is never purchasable: no paid ranking,
no promoted slots, no trending-for-money. Promotion is a free author page and
the author's own reviews/discussions.

## Clubs
Channels per book and per chapter; `spoiler_threshold_bp` gates messages
server-side against the member's progress (inside their RLS context, because
the gate reads private progress). Roles: member/moderator/organizer; founder
seeded organizer. Announcements channel kind reserved for one-way posts.
Voice/video: LiveKit rooms keyed by channel, Phase 4 ([16](16-realtime.md)).

## Notifications
`notifications` table, RLS-private, granular kinds; unread count on `/v1/me`.
Push ⏳ (Web Push when the reader opts in; tokens are personal data,
[19](19-privacy-model.md)).
