# Phase 4: Vercel Integration for Web Publishing - Context

**Gathered:** 2026-04-19
**Status:** Ready for planning
**Source:** Local architecture review + official Vercel docs

<domain>
## Phase Boundary

Add a native Vercel publishing path to AuraGo's existing web publishing stack so the agent can:
- configure Vercel in the Config UI with vault-backed credentials
- create or look up Vercel projects
- deploy homepage workspace projects to Vercel
- manage aliases, project domains, and environment variables behind permission gates
- log Vercel deploys through the homepage registry and keep manuals/tests current

This phase is for the AuraGo backend, Config UI, prompts, manuals, and tests. It is not a general-purpose website redesign phase.

</domain>

<decisions>
## Implementation Decisions

### Deployment Model
- **D-40:** Use a hybrid Vercel approach: Vercel CLI for homepage deploy execution, Vercel REST API for structured management operations
- **D-41:** Keep `homepage` as the authoring/build workflow and add a new `homepage` operation `deploy_vercel` instead of creating a second website authoring tool
- **D-42:** Treat Vercel MVP as static-first: HTML, Vite, Astro, Nuxt static output, and Next.js static export are in scope; SSR/Edge runtime support is deferred

### Config and Secrets
- **D-43:** Store the Vercel access token only in the vault using the key `vercel_token`
- **D-44:** Add a dedicated `vercel` config block with explicit permission gates mirroring the Netlify pattern
- **D-45:** Add Vercel secrets to the Python secret export denylist so web-publishing credentials never leak into generated Python tools

### Agent and Tooling
- **D-46:** Add a dedicated `vercel` integration tool for project, deployment, alias, domain, and environment operations
- **D-47:** `homepage deploy_vercel` should be the recommended publishing path for homepage workspace projects, similar to `deploy_netlify`
- **D-48:** Tool prompts and manuals must explicitly steer the agent toward `homepage deploy_vercel` for homepage workspace deployments

### UX and Auditability
- **D-49:** Add a Config UI section under Web Publishing with token save, status, and connection test
- **D-50:** Update all existing config translation files for the new Vercel section
- **D-51:** Homepage registry logging remains mandatory after homepage edits and deploys; Vercel deploys must log live URL and project identity

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing AuraGo patterns
- `internal/tools/homepage.go` — homepage container, framework scaffolding, and existing web publishing workflow
- `internal/tools/homepage_deploy.go` — current deploy flows, publish_local flow, and Netlify deploy integration point
- `internal/tools/netlify.go` — existing SaaS publishing provider implementation pattern
- `internal/server/netlify_handlers.go` — status/test endpoint pattern for provider-backed config sections
- `internal/server/server_routes_config.go` — config-related API route registration
- `internal/agent/dispatch_cloud.go` — cloud integration dispatch and permission gating
- `internal/agent/native_tools_integrations.go` — native tool schema registration
- `config_template.yaml` — config surface and defaults
- `ui/cfg/homepage.js` — homepage web publishing config UI
- `ui/cfg/netlify.js` — SaaS publishing provider config UI pattern
- `ui/lang/config/netlify/*.json` — translation pattern for provider-backed config sections
- `prompts/tools_homepage.md` — homepage workflow guidance
- `prompts/tools_netlify.md` — provider-specific prompt guidance

### Official Vercel references
- `https://vercel.com/docs/rest-api` — API basics, auth, team scoping, and rate limiting
- `https://vercel.com/docs/cli` — CLI authentication and automation model
- `https://vercel.com/docs/cli/deploy` — non-interactive deploy behavior, `--prod`, `--yes`, and stdout deployment URL
- `https://vercel.com/docs/rest-api/reference/endpoints/projects/create-a-new-project` — project creation API
- `https://vercel.com/docs/rest-api/reference/endpoints/projects/create-one-or-more-environment-variables` — project env API
- `https://vercel.com/docs/rest-api/reference/endpoints/aliases/assign-an-alias` — alias assignment API
- `https://vercel.com/docs/rest-api/reference/endpoints/projects/retrieve-project-domains-by-project-by-id-or-name` — project domains API

</canonical_refs>

<codebase_context>
## Existing Code Insights

### Homepage stack
- Homepage already builds a Docker dev image with both `vercel` and `netlify-cli` installed
- Homepage deploy flows already separate "build project" from "ship to provider"
- Homepage prompts already enforce workspace-relative file operations and registry logging

### Provider integration pattern
- Netlify already has:
  - config block
  - vault token loading
  - Config UI section
  - server handlers for status/test
  - tool schema and dispatch gating
  - manuals and prompt guidance
- Vercel should fit this pattern instead of inventing a one-off integration style

</codebase_context>

<specifics>
## Specific Ideas

- Prefer `team_id` for deterministic API scoping and keep `team_slug` as a UX helper
- Return deployment URL, project ID/name, target, and alias/domain information from `deploy_vercel`
- If a homepage project has no Vercel project configured, allow `deploy_vercel` to create it when project management is enabled
- Make Config UI messaging explicit about vault storage and permission-gated mutations
- Add dashboard visibility if the existing integrations overview already surfaces Netlify/Homepage status

</specifics>

<deferred>
## Deferred Ideas

- Git-connected Vercel projects and deploy hooks as a first-class homepage path
- SSR/ISR/Edge runtime support
- Monorepo root-directory orchestration
- Advanced Vercel protection-bypass and observability APIs

</deferred>

---

*Phase: 04-vercel-integration-for-web-publishing*
*Context gathered: 2026-04-19*
