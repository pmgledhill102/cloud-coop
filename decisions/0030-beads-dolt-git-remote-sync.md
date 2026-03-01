# ADR-0030: Beads Dolt Sync via Git Remote

## Status

Proposed

## Context

The beads migration to Dolt (for concurrent multi-agent writes) created a sync problem: the
database is local-only, cloud-coop VMs are ephemeral, and there is no way to create or manage
issues from mobile/web Claude Code sessions. Every environment that runs agents needs access to the
same issue tracker.

Options considered:

1. **GitHub Issues as inbox** — agents create issues via `gh`, a sync script imports them into
   beads. Adds a second source of truth and requires polling.
2. **DoltHub remote** — use DoltHub's hosted service as the remote. Adds a dependency on a third
   party and requires separate authentication.
3. **Git remote** — Dolt v1.81.10 added native Git remote support (built specifically for beads).
   The existing GitHub repo serves as the Dolt remote via `refs/dolt/data`. No new infrastructure,
   no new authentication, no new services.

## Decision

Use Dolt's Git remote support to sync the beads database through the existing GitHub repository.

The database state is stored on `refs/dolt/data` in the Git repo. This ref is not fetched by normal
`git clone` and does not affect working copies. It is managed entirely by `dolt push` / `dolt pull`
and uses existing Git/SSH authentication.

**Push** happens as part of the session completion protocol (alongside `git push`). **Pull** happens
at session/VM startup (alongside `git clone`).

A bootstrap script (`scripts/beads-bootstrap.sh`) handles Dolt + beads installation, database
clone, and server startup for new environments (VMs, web/mobile sessions).

See `docs/dolt-strategy.md` for the full implementation plan including multi-agent concurrency,
conflict handling, and sync diagrams.

## Consequences

### Positive

- No new infrastructure — reuses the existing GitHub repo
- No new authentication — reuses Git/SSH credentials
- Every environment gets native `bd` commands — no degraded experience
- Database is backed up alongside code in the same repository
- Supports concurrent multi-agent writes via Dolt server mode

### Negative

- Requires Dolt v1.81.10+ (recent feature)
- Push/pull is manual (not real-time sync) — acceptable for issue tracking cadence
- Conflict resolution needed if two environments push concurrently (rare for issue tracking)
- Bootstrap script adds setup complexity for new environments
