---
phase: 04-vercel-integration-for-web-publishing
plan: 02
status: completed
completed: "2026-05-03"
requirements:
  - WEB-VERCEL-01
  - WEB-VERCEL-03
---

# 04-02 Summary: Backend Vercel Tool and Agent Wiring

## Outcome

AuraGo now exposes a dedicated `vercel` native tool with permission-gated project, deployment, environment, domain, and alias operations.

## Completed Work

- Added `internal/tools/vercel.go` with token-authenticated Vercel REST API operations and normalized JSON responses.
- Added support for team scoping via `teamId` and team slug query parameters.
- Wired `VercelEnabled` through native tool feature flags and prompt/tool schema registration.
- Added `vercel` cloud dispatch with integration-enabled, read-only, deploy, project management, environment management, and domain management gates.
- Added Vercel prompt guidance and tool manual documentation.

## Verification

- `go test ./internal/agent/...`
- `go test ./internal/tools/...`

## Notes

The agent guidance keeps the split explicit: use `vercel` for provider management and `homepage deploy_vercel` for homepage workspace publishing.
