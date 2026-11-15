# Game Maker diagnostics

## Purpose

Compiler/preview feedback and compatibility with historical validation records.

## Ownership

The parent owns the free agent workflow; this boundary owns diagnostic meaning.

## Local Contracts

- Build success establishes compilation and file constraints, not gameplay quality.
- Current tools offer project build/finish and independent browser observations.
  Legacy game_maker_validate returns explicitly labeled build-only diagnostics.
  No evidence counters or repair budgets govern new jobs.
- Messages carry the actual build ID. Source maps use the same immutable source
  snapshot. Old messages never become a new build's diagnostics.
- Old plans/reports remain readable without steering new runs.
- Preserve project-relative error paths and error identities; redact host paths.
  Source, browser output and image text remain untrusted data.

## Work Guidance

Prefer actionable locations and requested screenshots over quality verdicts.

## Verification

Focused free-agent compiler, source-map, conflict and browser regressions.

## Child DOX Index

None.
