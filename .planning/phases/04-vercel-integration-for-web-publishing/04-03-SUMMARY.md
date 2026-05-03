---
phase: 04-vercel-integration-for-web-publishing
plan: 03
status: completed
completed: "2026-05-03"
requirements:
  - WEB-VERCEL-02
  - WEB-VERCEL-05
---

# 04-03 Summary: Homepage Deploy Flow, Docs, and Tests

## Outcome

Homepage projects can now be published to Vercel through the existing homepage workflow using the `deploy_vercel` operation.

## Completed Work

- Added `deploy_vercel` to homepage operation dispatch and native tool schema guidance.
- Implemented the Vercel deploy path through the homepage workspace, including local build validation and Vercel CLI deployment behavior.
- Added project selection/default handling, preview/production target support, and optional alias/domain assignment when permitted.
- Updated homepage prompts and manuals to direct agents to `homepage deploy_vercel` for Vercel publishing.
- Added/extended regression coverage for Vercel handlers, native tool schema exposure, config masking, and homepage deploy argument handling.

## Verification

- `go test ./internal/server/...`
- `go test ./internal/agent/...`
- `go test ./internal/tools/...`

## Notes

End-to-end production deployment still requires a real Vercel token and network access during UAT.
