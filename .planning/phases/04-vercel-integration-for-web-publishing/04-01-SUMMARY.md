---
phase: 04-vercel-integration-for-web-publishing
plan: 01
status: completed
completed: "2026-05-03"
requirements:
  - WEB-VERCEL-01
  - WEB-VERCEL-04
---

# 04-01 Summary: Config, Vault, and UI Foundation

## Outcome

Vercel now has a dedicated configuration surface, vault-backed token handling, status/test endpoints, and a translated Config UI section under Web Publishing.

## Completed Work

- Added the `vercel` config block with disabled-by-default integration state, read-only mode, deployment permission, project management permission, environment management permission, domain management permission, default project ID, team ID, and team slug.
- Wired `vercel.token` to the vault key `vercel_token` so the access token stays out of `config.yaml`.
- Added `vercel_token` to the Python secret denylist.
- Added `/api/vercel/status` and `/api/vercel/test-connection` handlers.
- Added the `ui/cfg/vercel.js` Config UI section with enable/read-only/permission toggles, project/team defaults, vault token save, status banner, and connection test.
- Added Vercel Config translations and section labels for all shipped Config languages.

## Verification

- `go test ./internal/server/...`
- `go test ./internal/tools/...`
- `node --check ui\cfg\vercel.js`

## Notes

Live connection testing still requires a real Vercel token in the vault and remains part of user-facing UAT.
