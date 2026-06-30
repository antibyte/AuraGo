# Homepage Registry Hard Identity Design

## Context

AuraGo tracks managed homepage projects in `data/homepage_registry.db`. The registry already has fields for local identity (`project_dir`) and deployment state (`homepage_deploy_targets`, `homepage_deployments`, `last_deploy_url`), but manual registry creation can currently produce entries that are later hard to map back to a concrete workspace folder or deployed Caddy/local target.

The failure mode is operationally risky: when the user asks the agent to clean up all homepage files except the active local Caddy site, the agent must be able to identify the protected project and its deployed target without guessing.

## Decision

Make new homepage registry writes fail hard unless they can be tied to a canonical homepage workspace directory. Make successful deploy or publish operations fail visibly to the agent if AuraGo cannot persist the resulting deployment target.

Existing entries are out of scope. There is no migration and no backfill. The new contract applies only to future writes through AuraGo code paths.

## Goals

- Every new homepage project entry has a non-empty, workspace-relative `project_dir`.
- Agent-created entries cannot use an ambiguous homepage workspace root as a normal project identity.
- Every successful deploy or local publish records a machine-readable deploy target.
- Cleanup or destructive flows can query the registry and determine which local folder, build output, and deployed target must be preserved.
- Failures are explicit and actionable for the agent, not hidden as warnings.

## Non-Goals

- No migration for old rows.
- No cleanup or repair of existing incomplete rows.
- No direct SQLite trigger or schema rebuild for existing databases.
- No broad redesign of the homepage registry UI.

## Project Identity Contract

`project_dir` is the canonical identity for managed homepage projects.

Rules for new rows:

- `project_dir` is required for `homepage_registry register`.
- `RegisterProject` rejects empty `ProjectDir`.
- Agent-created projects reject `project_dir == "."` unless the operation is explicitly designed to manage the homepage workspace root.
- `project_dir` must be normalized with the same workspace-relative rules used by the homepage ledger.
- Absolute paths are converted to workspace-relative paths only when they are inside `homepage.workspace_path`.
- `..`, unsafe path components, and paths outside the homepage workspace are rejected.
- Duplicate detection continues to prefer `project_dir` when present.

`name` remains useful display metadata, but it is not the project identity.

## Deployment Target Contract

After a successful deploy or local publish, AuraGo must persist a deploy target row.

Required deploy target fields:

- `project_id`
- `provider`
- `url` or `remote_path`
- `build_dir`
- provider-specific target ID when available
- provider deployment ID when available
- artifact hash when build output can be hashed

Provider behavior:

- `publish_local` and Caddy/local serving record provider `local` or `caddy`, the served URL, the resolved `project_dir`, the resolved `build_dir`, and the local served path.
- SFTP deploy records provider `sftp`, configured host or URL, remote path when available, and build output.
- Netlify records provider `netlify`, verified URL, site ID, deploy ID, build output, and artifact hash. Unverified Netlify deploys do not update the ledger.
- Vercel records provider `vercel`, deployment URL, project/deployment IDs when available, build output, and artifact hash.

If the deploy result does not contain enough information to record a deploy target, the dispatch layer must return an error-style tool result to the agent. The agent must not treat the deployment as safe for later cleanup decisions.

## Write Paths

Update these write paths:

- `DispatchHomepageRegistry` `register`: validate `project_dir` before calling `RegisterProject`.
- `RegisterProject`: enforce the same invariant so other callers cannot bypass the dispatch check.
- `EnsureHomepageProjectForDir`: keep deriving a canonical path for focused homepage operations, but reject ambiguous root identities unless explicitly allowed.
- Homepage focused `init_project`: continue deriving `project_dir` from tool output or project name, then create the registry row only after validation.
- Homepage deploy dispatch for `deploy`, `publish_local`, `deploy_netlify`, and `deploy_vercel`: treat ledger recording failure as a hard failure in the tool output.
- `RecordHomepageDeploymentFromResult`: expose fatal recording errors distinctly from non-fatal metadata warnings, or replace the warning-only contract with a strict variant used by deploy dispatch.

## Error Handling

Errors should tell the agent exactly how to fix the call.

Examples:

- `project_dir is required to register a homepage project`
- `project_dir must be relative to the homepage workspace`
- `project_dir "." is ambiguous for new homepage projects`
- `deployment target could not be recorded; project_dir and deploy URL or remote_path are required`
- `deployment result was not verified; registry target was not updated`

The dispatch response must keep the existing JSON style:

```json
{"status":"error","message":"project_dir is required to register a homepage project"}
```

## Agent Safety Behavior

The agent should be able to protect active local deployments by querying managed sites and deploy targets.

For cleanup requests, the safe source of truth is:

- project row: `project_dir`
- project state: `local_root`, revision and drift state
- deploy targets: provider, URL, remote path, last seen time
- deployments: build directory, artifact hash, provider deployment IDs

The agent should never infer the protected Caddy/local site from filenames alone when a deploy target exists.

## Tests

Add focused tests for:

- `homepage_registry register` rejects missing `project_dir`.
- `RegisterProject` rejects empty `ProjectDir`.
- unsafe or absolute-outside-workspace `project_dir` is rejected.
- `init_project` still auto-registers with a valid derived `project_dir`.
- deploy dispatch returns a hard error when deployment recording fails.
- local publish records a deploy target with provider, URL or path, `project_dir`, and build output.
- Netlify/Vercel recording keeps provider IDs and deployment URLs when verified.

Run at minimum:

```bash
go test ./internal/tools/... ./internal/agent/...
```

## Documentation

Update `prompts/tools_manuals/homepage_registry.md` so the agent manual states that `project_dir` is required for registration and that deploy targets are mandatory after successful deployment.

Update homepage tool manual text where deploy or publish examples imply that `project_dir` is optional for operations that mutate or publish a project.

No UI translation updates are required unless implementation adds or changes visible UI text.

## Acceptance Criteria

- A new registry entry cannot be created through AuraGo without a valid `project_dir`.
- A successful local publish or remote deploy cannot silently omit deploy target persistence.
- The agent can identify the local Caddy/published site from registry data without guessing.
- Existing incomplete entries remain untouched.
- Tests cover the hard failure behavior and the successful deploy target recording path.
