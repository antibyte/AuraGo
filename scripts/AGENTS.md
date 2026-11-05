# Repository scripts

## Purpose

Developer builds, asset generation, checks, local start/stop, service installation and release entrypoints.

## Ownership

This folder owns the maintained helpers and the `start.*`, `kill_all.*`, `install_service_linux.sh`, `make_deploy.*` and `make_release.*` entrypoints. Root retains public `install.sh`, `install.ps1`, `update.sh` and `repair-update.sh` URLs. Release security and publication policy remain in root and `../internal/upkeep/AGENTS.md`.

## Local Contracts

- Resolve the repository root from the script location before using project-relative inputs or outputs; invoking an entrypoint from another working directory must not relocate state, locks or artifacts.
- Sibling helpers are relative to this folder; `deploy/`, `bin/`, `.env` and source inputs remain relative to the repository root.
- Source paths may change; release asset names and installed config/notice paths stay stable. The English third-party notices come from `../licences/`.
- Keep the Bluetooth kit synchronized into root installers/updater and `scripts/install_service_linux.sh` through `sync-bluetooth-kit.sh`.

## Work Guidance

Use the existing helper for each job. Check release scripts with harmless fixtures and syntax parsing; publication and real service/process operations are not verification steps.

## Verification

- Run `bash -n <path>` for each changed shell script.
- Parse changed PowerShell with `System.Management.Automation.Language.Parser`.
- Run `node scripts/test-easydrag.mjs` for EasyDrag UI module and pack wire-contract checks.
- `python scripts/test-prepare-phonon.py` and relevant `go test ./internal/audit` release/install/build contracts.

## Child DOX Index

None.
