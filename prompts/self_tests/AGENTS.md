# Self-test prompts

## Purpose

Curated German test instructions embedded with AuraGo.

## Ownership

`catalog.md` is the runtime source for the 320 prompt templates. Historical plans are not a second runtime catalog. The runner and safety contract belong to `internal/selftest/AGENTS.md`.

## Local Contracts

- Keep one complete prompt per row with a unique ID, target selector and R/W/X/H/L profile.
- Profile metadata is a selection prerequisite, never an authorization. The runner owns effective permissions, fixed targets, budgets, assertions and cleanup.
- Runtime discovery determines enablement. New, changed or unreviewed operations remain visibly uncovered until a reviewed adapter or exact human binding exists.
- No credentials, private records or actual test reports belong here.

## Work Guidance

- Update the parser and catalog checks together if the row format changes.
- Keep user-facing prompts in German and catalog guidance in English.

## Verification

- `go test ./internal/selftest -run SelfTestCatalog`

## Child DOX Index

None.
