# 28 — Risk Register

**Status:** living · reviewed each phase gate

| Risk | Impact | Likelihood | Mitigation | Owner |
|---|---|---|---|---|
| AI spam flood | High | High | Structural friction (caps, minimums, ledger), citation gates, human queue; no "AI detector" theatre | server |
| Copyright takedown (covers/texts) | High | Medium | Rights-aware pipeline, woodcut default, cache-only-cleared, takedown path, per-work jurisdiction notes | server+legal |
| Amazon Associates ban/term change | Medium | Medium | Disclosure everywhere, PA-API discipline, Bookshop.org second vendor, affiliate is <1 surface | product |
| Open Library rate limits | Medium | Medium | 1 req/s pacing, identified UA, JetStream backoff, local caches, PG catalog as PD backbone | server |
| RLS bypass via superuser DSN in prod | High | Low | Least-privilege role documented + CI-provisioned; FORCE policies; integration test proves cross-reader invisibility | server |
| Migration drift between environments | High | Low | Checksummed forward-only migrations; `status` deploy gate; CI applies to fresh PG | server |
| LiveKit/TURN bandwidth cost at scale | Medium | Medium | Rooms only in clubs, no recording by default, coturn self-host, cost model per tier ([29](29-cost-model.md)) | infra |
| Scholar verification capture (elitism or fraud) | Medium | Low | Manual review + ORCID + COI statement; community-contributor lane; revocation path with rationale | product |
| Single-maintainer bus factor | Medium | Medium | ADRs, this package, committed sqlc output, compose-runnable stack, MIT client for adoption | all |
| Gutenberg feed format change | Low | Low | Parser validated against live header; malformed-row tolerance; catalog is one documented URL | server |
| WebAuthn browser fragmentation | Low | Medium | Magic-link fallback forever; ceremony errors are human sentences | web |
| Instance-operator legal exposure (federation) | Medium | Low | Each operator is their own DMCA agent; documented notice templates; AGPL clarity | legal |
