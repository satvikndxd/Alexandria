# 16 — Realtime Communication Architecture

**Status:** rooms implemented · `internal/realtime` (stdlib HS256 LiveKit
tokens, unit-tested), `POST /v1/channels/{id}/room/token`, web `RoomJoin`
with a dynamic livekit-client import · text chat remains HTTP by design

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

## Architecture (implemented)
- `realtime.Config.MintRoomToken` signs short-lived HS256 JWTs with the
  LiveKit video grant (roomJoin, canPublish, canSubscribe; **canRecord always
  false**). Built on the standard library: a room token is three claims and a
  signature.
- **LiveKit is the SFU, not the authority.** The endpoint checks, in order:
  channel kind is voice/video (text channels are read, not joined), caller is
  a club member, and the channel's spoiler gate is passed — inside the
  caller's RLS context, because the gate reads private reading progress. Only
  then is a token signed; a leaked room name grants nothing.
- Room names derive deterministically from club slug + channel id
  (`realtime.RoomName`), so rooms and channels cannot drift.
- Unconfigured instances answer 503 `rooms_disabled` instead of minting
  tokens for a server that is not there.
- Web loads `livekit-client` via dynamic import: readers who never join a
  room never download an SFU SDK.
- TURN: self-hosted coturn; bandwidth cost modelled in [29](29-cost-model.md).
- Moderation of rooms (kick/mute via the server SDK) and consent banners for
  any future recording remain ⏳; recording itself is ungrantable today.
- Presence: ephemeral, never persisted, never a metric.

## What we will not build
Read-receipts, typing indicators as engagement hooks, "active now" badges on
profiles. Presence serves coordination, not anxiety.
