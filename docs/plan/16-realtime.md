# 16 — Realtime Communication Architecture

**Status:** research/decision document · text chat implemented over HTTP;
voice/video ⏳ Phase 4

## Text chat (implemented shape)
Club messages are persisted in Postgres (`messages`) and read over HTTP with
keyset cursors and spoiler gating. We deliberately did **not** build a
WebSocket fan-out for MVP: clubs of 20–200 readers are well served by
cursor-paginated HTTP plus a short poll, and every message stays
moderation-auditable in the same transaction as its report.

## Voice/video decision: LiveKit
Evaluated: raw WebRTC (we would own SFU, TURN, scaling — rejected), Janus
(powerful, C, heavy ops), mediasoup (excellent, Node-native, ops burden),
Matrix (federated identity we do not need), LiveKit (open-source SFU, Go SDK,
first-class Flutter/Web SDKs, self-hostable, token auth we can mint from club
roles). **Decision: LiveKit**, self-hosted on K3s in production, single node
in compose for dev.

## Architecture (Phase 4)
- Mint short-lived LiveKit tokens from club membership + role; voice channel
  ↔ LiveKit room 1:1.
- Spoiler gating extends to rooms: a gated voice channel mutes (cannot join)
  below threshold — the gate is already data (`channels.spoiler_threshold_bp`).
- TURN: self-hosted coturn; bandwidth cost modelled in [29](29-cost-model.md).
- Moderation: room events logged; moderator kick/mute via LiveKit server SDK;
  recordings **off by default** and only with all-party consent banner (legal
  review, [21](21-licensing-legal.md)).
- Presence: ephemeral, never persisted, never a metric.

## What we will not build
Read-receipts, typing indicators as engagement hooks, "active now" badges on
profiles. Presence serves coordination, not anxiety.
