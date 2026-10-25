# Managed websites

AuraGo stores website identity, provenance, locations, change reasons, revisions,
deployment targets and observations in data/homepage_registry.db (or the
configured sqlite.homepage_registry_path). Homepage Studio and the agent use the
same persistent inventory. Disabling the Homepage container runtime does not hide
existing inventory or disable its registry lookup tool.

## Find a website

The homepage_registry tool provides:

- overview: complete inventory with limit/offset pagination and optional query.
- get_site: detail by internal id or project_dir, including local availability,
  provenance, deployment targets, current revision and recent events.
- list_events: changes and reasons for id, with limit/offset pagination.
- discover_workspace: read-only candidates with index.html or package.json.
- adopt: register an existing project_dir with an explicit reason.
- relocate: bind an existing id to an existing new project_dir with reason,
  preserving history. This changes the known location, without moving files.
- audit_revisions: credential-like filenames in legacy revisions, never values.

The internal numeric ID remains stable. Project paths are workspace-relative;
the workspace root is pinned when the project is first managed. Configuration
changes cannot silently reinterpret an existing project in a different folder.
The longest registered directory boundary wins for nested paths. Metadata updates
cannot move a bound project; use relocate, serialized with workspace mutations.
Provider site_id/project_id fields are distinct from managed_site_id.
The workspace root itself is intentionally not a new project.

Provenance distinguishes agent_created, adopted and unknown. Legacy dates,
actors and reasons are not invented. Missing local directories and archived
entries retain their known locations and history. Registry delete archives a
record rather than cascading its history.

## Change recording

Every agent Homepage mutation requires a short reason describing its purpose.
All focused tools accept it. Missing reasons fail before execution. System
lifecycle calls supply their known trigger.

The common execution boundary writes intent before filesystem/provider effects
and then records actual changes, revision and final outcome. It covers file
editors, scaffold, dependency install, build, workspace commands, Git, restore
and publication. Exec requires project_dir and starts there; other known sites
and newly discovered projects are observed after workspace commands too.

Related events share operation_id. Outcomes distinguish success, failure,
partial effects and uncertainty. Failed commands can still create revisions.
Uncertain results carry retry_safe:false and must not be blindly replayed.
Unfinished intents become explicit unknown outcomes after restart.

`ledger_updated: true` confirms that the attempt was recorded, including a failed
attempt. It does not report a successful file edit or deployment; use `status`
and the recorded outcome, then verify the resulting file before publication.

For targeted text changes use `homepage_file` with `operation: "edit_file"` and
`sub_operation: "str_replace"`, `str_replace_all`, `insert_after`, `insert_before`,
`append`, `prepend` or `delete_lines`. Replacement uses `old`/`new`; insertion
uses `marker`/`content` and requires the marker to match exactly one line. Docker
file reads and edits transfer complete files up to 2 MiB and reject incomplete,
non-regular or oversized inputs before writing. `write_file` also requires
complete content up to 2 MiB and rejects empty writes. Archive uploads assign
files to the configured container user so later edits can write them directly.
An upload also corrects ownership when replacing an older root-owned file;
untouched legacy files may still need a project-scoped ownership correction.

Ordinary changes do not require duplicate log_edit/add_history calls. History
notes remain useful for decisions, questions and additional explanations.
Externally observed changes retain unknown original reasons.

The agent inventory is a required request-local prompt addendum independent of
reuse hints, including stable/prepared prompts and minimal loops with the
relevant scope. It refreshes for every physical model request. Active-session
projects take priority in the compact context; paginated tools expose the rest.
The required preview is bounded after external-data escaping. Names, paths and
reasons may be shortened in the preview; get_site/list_events retain full values.
Preexisting scaffold files are snapshotted with unknown original provenance
before initialization; they never become newly agent-created history.

## Deployment and observation

Each publication keeps provider IDs, URL, source revision and artifact hash when
available. Account/environment distinguish targets, including simultaneous
preview and production. Immutable deployment URLs remain distinct from aliases.
Draft/failed/unverified attempts never become verified live sites.
Native here.now mutations share the operation journal. A duplicated website gets
its own managed ID, agent_created provenance and a link to the source ID; remote-only
sites retain provider locators without an invented local folder.
Temporary Cloudflare targets become inactive when their publication closes.
Stopping development does not imply stopping a separate Caddy deployment.

The reconciler starts with the server even if Homepage is initially disabled.
Each pass checks current configuration under its lock. It runs at startup and
every 15 minutes; local state is additionally observed before mutations.
Remote availability describes the last observation, not permanent liveness.
Public observations use strict DNS-pinned SSRF protection. Explicitly configured
local/Caddy/SFTP origins permit bounded same-origin redirects.

Additive HTTP interfaces:

- GET /api/homepage/sites
- GET /api/homepage/sites/{id}
- GET /api/homepage/sites/{id}/events?limit=20&after=0
- POST /api/homepage/sites/{id}/reconcile

## Revision safety and upgrades

Revision content excludes credential-like paths and generated artifact
manifests. Hashing streams large files. Content is bounded to 500 KiB per file
and 64 MiB per scan, with at most 10,000 inventoried files. Missing, sensitive
or incomplete content is never silently restored. Historical reconstruction
uses the complete revision chain in stable ID order.

Restore validates the desired state before writing, preserves unknown external
files and records a new revision. Complete temporary preimages protect affected
files; a partial failure returns the recovery backup path. Rooted atomic file
replacement keeps individual writes complete. Empty and binary files use bytes.
Permissions-only changes are revisioned and restored too. Restore preflights
nonlocal paths, symlinks and file/directory conflicts before changing any file,
and honors cancellation between writes and at atomic replacement. Legacy
sensitive content is neither copied into new revisions nor written by restore.

Schema upgrades create a pre-migration backup of the underlying SQLite file
(including URI-backed paths), with private permissions set before copying.
They preserve deployment foreign keys and advance PRAGMA user_version transactionally.
Backups can contain legacy sensitive material and require restricted handling.
Schema v2 merges duplicate project directories only after v1 has assigned target
keys. Targets merge only when both provider and target key match; distinct
preview/production targets keep their IDs and deployment/observation links.
The entire migration rolls back if merging or foreign-key verification fails.
An already migrated v2 registry is not rewritten. If an older migration lost a
target, compare a copy of the current registry with its `.homepage-v2-*.bak`
backup before a targeted recovery. Replacing the whole database would discard
newer events, revisions and deployments; missing identities cannot be inferred
reliably without the backup.
There is no automatic pruning of delta revisions: retention needs verified
checkpoints before old deltas can be removed.

## Verification

Homepage regression tests cover migration and backup contents, multiple targets,
partial commands, restart recovery, credential exclusion, safe restore, nested
identity, workspace pinning and more than 1,000 revisions. Rendered-prompt tests
verify independence from reuse policy. Live provider publication and deployed
restart acceptance remain separate from local fixtures.
