# Phase 4 UAT: Vercel Integration for Web Publishing

## Goal

Verify that AuraGo can configure Vercel safely, deploy homepage projects to Vercel, and maintain the resulting project through the agent workflow.

## User Acceptance Scenarios

### UAT-01: Configure Vercel in the Config UI
- Open Config -> Web Publishing -> Vercel
- Save a Vercel token to the vault
- Run the connection test
- Expected:
  - token is not written into `config.yaml`
  - status shows connected account/team info
  - the section remains translated and usable in every shipped config language file

### UAT-02: Create or select a Vercel project
- Ask the agent to list Vercel projects
- Ask the agent to create a new Vercel project when project management is enabled
- Expected:
  - the agent uses the `vercel` tool
  - the response includes project ID/name
  - readonly or missing permission modes block mutations with a clear error

### UAT-03: Deploy a homepage project to preview
- Initialize or select a homepage workspace project
- Ask the agent to deploy it to Vercel preview
- Expected:
  - the agent uses `homepage deploy_vercel`
  - build output is taken from the homepage workspace, not `agent_workspace/workdir`
  - response includes a preview deployment URL

### UAT-04: Promote or deploy to production
- Ask the agent to create a production deployment for the same project
- Expected:
  - deployment succeeds only when deploy permission is enabled
  - returned data includes the production deployment URL and project identity
  - homepage registry records the deploy URL

### UAT-05: Maintain aliases, domains, and env vars
- Ask the agent to set an env var, assign an alias, and add or verify a domain
- Expected:
  - the agent uses the `vercel` tool
  - permission gates are enforced separately for env/domain management
  - output is normalized and actionable, including verification instructions when a domain is not yet verified

## Regression Checks

- `go test ./internal/...`
- Vercel integration hidden when `vercel.enabled=false`
- No Vercel credentials appear in prompt logs, config payloads, or Python-exportable secret lists
