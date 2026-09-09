# Alexandria

**An open-source, ad-free, human-first social reading platform.**
Books are written by humans, discussed by humans, explained by humans, and
read by humans.

Alexandria is "Letterboxd for books" elevated with scholarly rigor: personal
libraries, long-form reviews, peer-reviewed scholar notes, spoiler-gated book
clubs, and an in-app reader for public-domain classics — wrapped in an
illuminated-manuscript / botanical-woodcut design language. No ads, no
engagement algorithms, no AI-generated literary content, ever.

## Monorepo

```
alexandria/
├── apps/
│   ├── api/          Go 1.24 modular monolith (chi · pgx · sqlc · NATS JetStream)
│   │   ├── cmd/api       HTTP edge (passkey auth, library, reviews, clubs, search)
│   │   ├── cmd/worker    ingestion: Open Library, Gutenberg catalog, mail, index
│   │   ├── cmd/migrate   embedded, checksummed, forward-only migrations
│   │   └── cmd/reindex   rebuild the Meilisearch projection from Postgres
│   ├── web/          Next.js 14 App Router (custom Tailwind design system)
│   └── mobile/       Flutter 3 (CustomPainter drop caps, no Material chrome)
├── packages/
│   ├── db/           SQL schema: migrations (embedded) + sqlc queries (source of truth)
│   ├── core/         Shared tokens, friction constants, types
│   └── ui/           Shared React components (graduating from apps/web)
├── infrastructure/   docker-compose, Dockerfiles, CI
└── docs/             ADRs · setup guide
```

## Architecture

```mermaid
flowchart TD
    subgraph Clients
        Web[Next.js Web]
        Mobile[Flutter Mobile/Desktop]
    end
    subgraph Monolith["Go Modular Monolith"]
        API[cmd/api — HTTP edge]
        Worker[cmd/worker — ingestion · mail · indexing]
        Migrate[cmd/migrate — embedded schema]
    end
    subgraph Data
        PG[(PostgreSQL 16 + RLS)]
        NATS[(NATS JetStream)]
        Meili[(Meilisearch)]
        MinIO[(MinIO)]
    end
    Web -->|cookies + CSRF| API
    Mobile --> API
    API -->|tx: domain write + outbox| PG
    API -->|outbox relay| NATS
    NATS -->|ingest.>| Worker
    Worker -->|paced, identified requests| OL[Open Library]
    Worker -->|official offline catalog| PG2[Project Gutenberg feeds]
    Worker --> PG
    Worker --> Meili
    Worker --> MinIO
    Migrate --> PG
```

**Key decisions** (full rationale in [`docs/adr/`](docs/adr/)):

| Decision | ADR |
|---|---|
| Modular monolith, not microservices | [0001](docs/adr/0001-modular-monolith.md) |
| FRBR: `works` → `editions`, no duplicate books | [0002](docs/adr/0002-frbr-data-model.md) |
| sqlc + pgx, explicit SQL, no ORM; generated code committed | [0003](docs/adr/0003-sqlc-over-orm.md) |
| Event-driven ingestion (JetStream + transactional outbox) | [0004](docs/adr/0004-event-driven-ingestion.md) |
| Anti-AI-slop = structural friction, never "AI detection" | [0005](docs/adr/0005-friction-not-detection.md) |
| Passkey-first auth (WebAuthn), magic-link fallback, no passwords | [0006](docs/adr/0006-passkey-first-auth.md) |
| Row-Level Security as the privacy backstop | [0007](docs/adr/0007-rls-privacy-backstop.md) |
| Forward-only, checksummed, embedded migrations | [0008](docs/adr/0008-embedded-forward-only-migrations.md) |

## Identity & privacy, concretely

- **Passkeys (WebAuthn)** are the primary factor; ceremonies are stateless
  across nodes (state lives in `auth_challenges`, single-use, self-expiring).
  **Email magic links** are the fallback and oracles nothing: unknown addresses
  get the same 202 and no mail.
- Sessions are opaque 256-bit tokens; only SHA-256 hashes are stored. Cookies
  are `HttpOnly` + `SameSite=Lax`, with a double-submit CSRF token HMAC-bound
  to the session.
- **Row-Level Security** protects shelves, shelf items, reading sessions,
  annotations and notifications. The request path runs as a least-privilege
  role and sets `app.user_id` per transaction with `SET LOCAL`; superusers
  bypass RLS by definition, so production never connects as one. Private
  shelves are invisible to Postgres itself under another reader's context —
  the integration suite proves it.
- Spoilers are withheld **server-side**: a spoiler-tagged review's bytes never
  leave the API until the client reveals them, and chapter-gated club channels
  refuse readers below the threshold with `403 spoiler_gated`.

## The friction system (anti-slop)

Enforced at **three layers** so no buggy handler can bypass it:

- **Schema:** reviews ≥ 150 chars (`CHECK`), one review per user per work,
  scholar notes ≥ 200 chars and citation-gated, moderation actions require a
  written rationale.
- **Domain (Go, unit-tested):** half-star rating bounds, rune-based length,
  filler heuristics (repeated glyphs, low vocabulary).
- **Policy:** new accounts post 2 reviews/day; Trusted Readers (reputation ≥
  100) post 10; caps computed from an append-only `contribution_ledger`.

## Design system

Parchment `#E8DDC4` · Deep Ink `#111713` · Botanical Green `#0D3B2E` ·
Vermilion `#D9471F` · Gold `#C79522`. UnifrakturMaguntia for display, EB
Garamond for body. Sharp editorial corners (radii: 0 and 2px only), hard
offset print-block shadows, engraved SVG icons, and a deterministic
**generative drop-cap asset system** (SVG on web, `CustomPainter` on Flutter)
modeled on botanical woodcut initials — the same letter always renders the
same ornament, and any letter can be swapped for hand-drawn OFL/CC0 art
without touching call sites.

## Quick start

```sh
docker compose -f infrastructure/docker-compose.yml up -d   # Postgres+NATS+Meili+MinIO
cd apps/api && go run ./cmd/migrate up                      # apply embedded schema
go run ./cmd/api &                                          # :8080
go run ./cmd/worker &                                       # ingestion · mail · index
npm install && npm run dev --workspace @alexandria/web      # :3000 (fixtures without API)
```

Tests:

```sh
cd apps/api
go test ./...                                   # unit tests, no services needed
ALEXANDRIA_TEST_DATABASE_URL=postgres://alexandria:alexandria@localhost:5432/alexandria?sslmode=disable \
  go test ./internal/httpapi/                   # integration: real Postgres, RLS, CSRF, friction
```

The integration suite runs against a real PostgreSQL 16 cluster (migrations
applied through the production runner) and covers: magic-link flow and session
revocation, single-use tokens, mailbox non-oracle, CSRF enforcement, the review
friction pipeline including the daily cap, spoiler byte-withholding,
shelf/progress agreement, RLS private shelves through the least-privilege role,
club spoiler gating, search degradation, and follow/block feed semantics.

Full guide: [`docs/SETUP.md`](docs/SETUP.md).

## Sourcing & compliance

- **Open Library:** metadata via API with identifying User-Agent, 1 req/s
  pacing, JetStream backoff on 429s.
- **Project Gutenberg:** official offline catalog only
  (`pg_catalog.csv.gz`, ~90k rows, one request); texts cached once to our own
  object storage, never hotlinked; license and trademark notices stored as
  data with every record.
- **Covers:** rights-aware pipeline — every `cover_assets` row separates
  *source* from *license* from *cache policy*; unverified covers are replaced
  by generated house "woodcut plates."
- **Monetization:** affiliate links only, on an explicit
  Book → Purchase Options page, with disclosure. Zero display advertising.

## Licensing

- Backend, web, infrastructure: **AGPL-3.0-only** ([LICENSE](LICENSE))
- Flutter client & design tokens: **MIT** ([apps/mobile/LICENSE](apps/mobile/LICENSE))

## Roadmap

1. **Foundation** ✅ — schema, monolith, ingestion pipeline, design system, web shell
2. **MVP** ✅ (backend) — passkey + magic-link auth, sessions + CSRF, Meilisearch
   search with Postgres fallback, live library/progress/reviews/clubs APIs,
   RLS, integration suite · *(web client wiring in progress)*
3. **Reader** — EPUB/TXT renderer with parchment typography, annotations
4. **Community** — scholar verification + peer review, clubs with LiveKit voice/video

See [`CONTRIBUTING.md`](CONTRIBUTING.md) before opening a PR.
