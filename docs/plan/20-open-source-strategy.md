# 20 — Open-Source Strategy

**Status:** living · licenses in repo roots

## Licensing split
- **AGPL-3.0-only**: backend, web, infrastructure (`LICENSE`). If someone runs
  Alexandria as a service, their modifications to the server remain open —
  the clause exists precisely for a platform whose value is its community
  data model.
- **MIT**: Flutter client and design tokens (`apps/mobile/LICENSE`), so
  adoption in other readers/apps is frictionless and the design language can
  travel.

## Contributor ergonomics
- One `docker compose up` + one `go run` + one `npm run dev`: the whole
  platform on a laptop ([30](30-developer-setup.md)).
- sqlc output committed so review reads what ships; CI fails on drift.
- Integration tests skip politely without services; the smoke script proves
  the loop when they exist.
- ADRs are the memory: every structural decision has one, amended in place
  with dates rather than rewritten.

## Governance (grows with contributors)
- Maintainers merge; ADRs require one reviewer from a different surface
  (server/web/mobile).
- RFC label for changes touching schema, auth, or the friction constants —
  those three are the constitution; everything else is legislation.
- Security reports: SECURITY.md with a monitored address and a 72-hour ack
  target (⏳ to be published with the first external release).

## What we will not accept
Features that require third-party trackers, that monetize attention, that
weaken the friction constants, or that assume copyrighted assets are free
because an API returned them.
