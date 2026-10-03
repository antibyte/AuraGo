# Desktop services

## Purpose

Desktop file operations, rooted mutations and local presentation assets.

## Ownership

The Service owns authorization, read-only state, mutation locks and cache invalidation; HTTP adapters delegate these operations.

## Local Contracts

- Archive create/extract use rooted operations and the same read-only/Notes gates as ordinary writes. Preflight every ZIP name, mode, size/count, namespace conflict and existing target before writing; reject escaping symlinks, special files, UNC/drive and traversal names.
- Read actual decoded bytes within budgets. Check ZIP close, file sync and close, and publish each file atomically with Windows replacement retries. Validation failures preserve destinations; unrelated I/O failure does not imply a multi-file rollback.

## Work Guidance

Keep temporary files private and clean them on failure.

## Verification

`go test ./internal/desktop` and server Desktop archive/read-only tests.

## Child DOX Index

- `pets_assets/AGENTS.md` owns OpenPets sprites, licensing and asset validation.
