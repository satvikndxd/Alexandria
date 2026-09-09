# 29 — Cost Model

**Status:** living estimates (2026 prices, EUR/USD-agnostic) · self-hosted
open-source stack throughout; the expensive items are named honestly

| Users | Compute | Voice/video (LiveKit+TURN) | DB/storage/bandwidth | Total/mo |
|---|---|---|---|---|
| 100 | $5 (1 VPS) | $0 (none running) | $0 | **$5** |
| 1 000 | $15 (2 vCPU VPS) | $0 | $5 | **$20** |
| 10 000 | $40 (4 vCPU VPS + worker) | $20 (coturn bandwidth) | $20 | **$80** |
| 100 000 | $150 (K3s ×3) | $100 (SFU + TURN) | $150 (S3+CDN) | **$400** |
| 1 000 000 | $600 (K3s ×6 + replicas) | $500 (SFU cluster + CDN TURN) | $600 | **$1 700+** |

## Why the curve stays flat
- No ad network means no tracking infrastructure; no engagement model means no
  feature store; no ML inference at request time, ever.
- Search is Meilisearch (single node, RAM-bound, rebuildable) not an ES
  cluster; the index is derived state we can regenerate.
- Texts and covers are cache-once objects behind a CDN with lifecycle expiry;
  readers never hotlink upstreams.
- Voice/video is opt-in per club room, off by default, never recorded: SFU
  cost tracks rooms-in-use, not users.

## The line we watch
Bandwidth for cached texts at 1M readers is the first number that can
surprise us; the mitigation is CDN caching with immutable keys (texts never
change), which turns it into a one-time egress per object.
