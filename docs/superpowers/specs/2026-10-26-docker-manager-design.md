# Docker Manager desktop app — design

Date: 2026-10-26. Status: approved by the user section by section (own host list, stacks on each target host, full
agent integration, all four v1 extras, compose runner container, five phases).

## Goal

A built-in desktop app **Docker Manager** that manages Docker on the local engine and on remote engines with a
polished, theme-aware UI. Its core is Dockge-like: every application is a **stack** made from one compose file (plus an
optional `.env`), created by pasting or dropping that file, and managed with up, down, restart, pull and update, live
logs and a terminal. On top of that, v1 ships an image update check, live resource figures, views for images, volumes
and networks with a cleanup assistant, stack templates, crash notifications, and an agent tool.

## Decisions taken with the user

| Topic | Decision |
| --- | --- |
| Remote hosts | Own host list in the Docker Manager: local, TCP, TLS/mTLS, SSH. The transport is extracted from Invasion into a shared package; Docker nests can be imported with one click. |
| Stack storage | On each target host, one folder per stack, default `/opt/stacks/<name>/compose.yaml` (Dockge layout, per-host setting). Existing Dockge stacks appear automatically. |
| Agent | Full: a new `docker_stacks` agent tool plus AI help in the editor (generate, explain, fix, convert). |
| v1 extras | Image update check; live resources; images, volumes and networks with cleanup; templates and notifications. |
| Compose engine | Approach A, the compose runner container (below). |
| Auto-updates | Not in v1 (user confirmed). |
| Agent deletes | Not in v1: the agent cannot delete volumes or stack folders (user confirmed). |

## Constraints that shape the design

- The AuraGo Docker image ships no Docker CLI and no compose plugin (`Dockerfile:55-68`). Today compose runs only as
  `docker compose` via `exec.CommandContext` in the agent tool (`internal/tools/docker_management.go:947-997`), so it
  fails inside a Docker install.
- The admin container API knows exactly one engine, `docker.host` (`internal/server/containers_handlers.go:19-23`).
  TLS/mTLS and SSH transports exist only inside Invasion (`internal/invasion/connector_docker.go:379-545`,
  `internal/invasion/docker_tls.go:46-83`).
- All Engine traffic is raw `net/http` pinned to API v1.45 by `dockerutil.VersionTransport`
  (`internal/dockerutil/version_transport.go:25-151`). There is no Docker SDK in `go.mod`, and none is added.
- There is no Engine events stream anywhere. The live container list is a 5 s poll
  (`internal/server/server_routes.go:164-197`). Logs are a one-shot tail capped at 8000 bytes
  (`internal/tools/docker.go:1300-1326`).
- The vendored CodeMirror bundle has no YAML mode (`scripts/build-codemirror.js:16-28`). xterm.js and its fit addon
  are vendored (`scripts/build-browser-vendor.js:52-55`).
- User rule carried over from the Docker audit: **no existing feature may be restricted as a side effect.**

## Approach: compose runner container

Every compose operation runs inside a short-lived **runner container** on the target engine, created from the
official `docker:<version>-cli` image (it contains the compose plugin), pinned by digest.

- **Binds:** the host's stacks directory at the *same path* (`/opt/stacks:/opt/stacks`), so that relative binds such
  as `./data` resolve to real host paths, plus the host's Docker socket (`/var/run/docker.sock` by default,
  per-host setting for rootless Docker).
- **Command:** `docker compose -p <name> -f <file> [--env-file .env] <subcommand>` with working directory
  `/opt/stacks/<name>`. Machine-readable progress (`--progress json` when the pinned compose version supports it,
  `plain` otherwise) is streamed to AuraGo by attaching to the container.
- **Registry credentials** (optional, from the vault; phase 4) reach the runner as a generated `config.json` through
  its stdin, because its read-only root filesystem cannot receive a file through the archive API before it starts.
  The runner itself needs no network, because pulls and builds are done by the daemon.

File access uses the same image as a **parked IO container** per host (`aurago-docker-manager-io`): created, never
started, with the stacks directory bound at the same path. The Engine archive API (`GET/PUT
/containers/{id}/archive`, already wrapped by `DockerGetArchiveContext` in `internal/tools/docker_archive.go:70`) works
on stopped containers, so AuraGo can list, read and write stack files on every host type without SSH or SFTP.

Stack folders are discovered with a short-lived **lister** container from the same image (`find` over the stacks
directory, bound read-only), because the archive API can only stream whole trees. The runner image and the IO
container are created by an explicit **prepare** step per host (or as the first step of a stack operation); reads
never pull.

This gives real compose semantics everywhere (build, profiles, `depends_on` conditions, `include`, `extends`). It is
the same mechanism for the local socket, TCP, TLS and SSH, and it needs nothing installed in AuraGo's own image. Cost:
one image per host (size measured in plan 2; it is larger than the "~25 MB" first quoted, because the image carries
the CLI, buildx and compose), about a second of start-up per operation, and a socket bind inside the runner.

When a host cannot reach a registry, AuraGo copies the runner image from the local engine (`GET /images/{name}/get` →
`POST /images/load`). When the local engine lacks it too, the error names the exact `docker pull` command.

Rejected:

- **Own Go compose engine** (compose-go plus our own reconciler on the Engine API): large effort and permanent
  compatibility gaps (build, profiles, `depends_on` conditions, `include`, secrets and configs, recreate rules). Remote
  file access would still be needed.
- **CLI where it lives** (CLI in the AuraGo image, SSH exec on remote hosts): plain TCP/TLS hosts cannot work because
  their stack files are not reachable. The image grows, and every remote host needs the compose plugin.

## Architecture

```
Desktop app docker-manager ─┐
Agent tool docker_stacks ───┴─▶ HTTP API /api/docker-manager
                                   │
            ┌──────────────────────┼───────────────────────┐
     internal/dockerstacks   internal/dockerhost     internal/dockerwatch
     files, ops, lint,       registry, transport,    events, stats,
     runner, journal         engine clients          updates, notifications
                                   │
                         target host: Engine · runner · IO container · /opt/stacks
```

### `internal/dockerhost` (new): hosts and transport

- **Host record**:
  - SQLite table `docker_hosts` in a new `docker_manager.db`, opened through `internal/dbutil` like the other
    per-feature stores.
  - Fields: `id`, `name`, `kind` (`local`, `tcp`, `tls`, `mtls`, `ssh`), `address`, `port`, `stacks_dir` (default
    `/opt/stacks`), `host_socket_path` (default `/var/run/docker.sock`), `color`, `ssh_user`, `sort`, `created_at`.
  - Secrets live in the vault: `docker_host_ssh_<id>` (key or password) and `docker_host_tls_<id>` (CA, cert, key
    JSON, same shape as `nest_docker_tls_<id>`).
- **Local host**: always present with id `local` and not deletable. It follows `docker.host` and falls back to
  `dockerutil.DefaultHost()`.
- **Transport**: the TLS config builder and the SSH socket dialer move from Invasion into `dockerhost/transport`.
  `internal/invasion` then calls the shared code, with behaviour unchanged and guarded by the existing Invasion tests
  plus new regression tests for the moved code.
- **Engine client**: `Client(ctx, hostID)` returns an `*http.Client` and a base URL. The API version is negotiated
  once per connection from `/version` as min(server, 1.45); engines below 1.41 (Docker 20.10) are refused with a clear
  message.
- **Health**: ping plus a classified error (`dns`, `refused`, `timeout`, `tls`, `auth`, `socket_missing`,
  `api_too_old`), cached per host with backoff.
- **Nest import**: creates a host from a nest with deploy method `docker_remote`, `docker_ssh` or `docker_local`. The
  vault secret is copied, not shared, so deleting either side leaves the other intact.

### `internal/dockerstacks` (new): stacks service

- **Discovery**: list the stacks directory through the IO container. A folder is a stack when it contains
  `compose.yaml`, `compose.yml`, `docker-compose.yml` or `docker-compose.yaml` (Dockge's order). Status comes from
  containers labelled `com.docker.compose.project=<name>`:
  - `draft`: files exist, no containers;
  - `running`: all services running;
  - `partial`: some services down, unhealthy or restarting;
  - `stopped`;
  - `error`: exited with a non-zero code.
- **External projects**: compose projects whose `com.docker.compose.project.working_dir` label is outside the stacks
  directory. They support start, stop, restart and logs, but no editing. *Adopt* (phase 4) is offered only when the
  working directory is inside the stacks directory of that host; moving a project folder re-resolves its relative
  binds, so adopt needs its own design.
- **Files**: only the compose file and `.env` are editable. Reads return content plus a revision (SHA-256 of the
  bytes). Writes carry `If-Match: <revision>`; a mismatch returns 409 with both versions.
- **Names**: stack names follow compose project rules, `^[a-z0-9][a-z0-9_-]{0,62}$`. Path traversal is impossible by
  construction.
- **Lint** (fast, while typing, pure Go on `gopkg.in/yaml.v3`): YAML syntax with line numbers; services; port
  conflicts against ports published on that host; named volumes vs. binds vs. relative paths; `${VAR}` without a
  default and without an `.env` value. Hints: `latest` or no tag, `privileged`, Docker socket bind, `network_mode:
  host`, missing `restart:`, `cap_add`. Lint never blocks the admin.
- **Operations**: `config`, `up` (`up -d --remove-orphans`), `down`, `stop`, `restart`, `pull` and `update` (pull,
  then `up -d`). The UI alone may additionally delete a stack (with or without volumes).
  - One lock per (host, stack): a second request gets 409 `operation_in_progress` with the running operation's id.
  - Every operation is recorded in table `docker_operations` (id, host, stack, kind, state, runner container id, exit
    code, timestamps, last 64 KiB of output).
  - **Restart-safe**: runners are created without auto-remove and carry labels `aurago.docker-manager.role=runner`
    and `aurago.docker-manager.operation=<id>`. After an AuraGo restart, operations still marked running are reattached
    by label, or finalised from the container's exit code. The runner is removed only after its result is recorded.
- **Health wait** after `up`: services with a healthcheck are watched until healthy or 120 s, then the operation
  finishes with the published ports as links.
- **Per-stack settings** (table `docker_stack_settings`: host, stack, `ignore_updates`, `notify`) and **own
  templates** (table `docker_templates`: name, description, compose, env skeleton) live in `docker_manager.db`, never
  in the stack folder, so stack files stay plain compose.
- **Runner hardening**: `NetworkMode: none`, `CapDrop: ALL`, read-only rootfs with a tmpfs `HOME`,
  `no-new-privileges`, image pinned by digest. The IO container gets the same settings and is never started. Both are
  treated as protected containers and never appear in cleanup.

### `internal/dockerwatch` (new): live data

- **Events**:
  - One long-lived `GET /events` per host that has a subscriber (types container, image, volume, network), with
    reconnect plus backoff and a full resync after a reconnect. The watcher runs only while a stream client or a
    project log follower needs it and stops 30 s after the last one.
  - If events fail repeatedly while the host itself answers, the host falls back to a 10 s list poll and the UI shows
    "live updates paused"; a host that does not answer is `unreachable` with the classified reason.
- **Stats**:
  - The UI subscribes with the container ids it currently shows (at most 50 per host). The server samples only those,
    with `/containers/{id}/stats?stream=false` (plus `one-shot=true` on API 1.41 and later) every 3 s, at most 4 calls
    in flight per host, and keeps a 60-sample ring per container. Container CPU is a percent of one core, as in
    `docker stats`.
  - Host load comes from `/info` and the container sums (every running container, at most 100, every 10 s while the
    gauges are open). `/system/df` is fetched only on demand and at most every 5 min; the disk gauge shows Docker's own
    usage in bytes, because the Engine reports no file-system size.
- **Streams**: at most 16 streaming Engine connections per host (events, followed logs, terminals). Logs are followed
  per container and merged for a stack or project (new containers are attached from the watcher's `start` events);
  lines are capped at 16 KiB and batched. The terminal opens its exec through the host's own transport, so it works on
  TLS and SSH hosts too.
- **Update check**:
  - For each image used by a stack service, the local `RepoDigests` entry is compared with the registry manifest
    digest from a `HEAD /v2/<repo>/manifests/<tag>`, using the anonymous token flow, or credentials from the vault for
    private registries.
  - Docker Hub, GHCR, Quay, LSCR and self-hosted v2 registries are covered. Multi-arch images are compared by index
    digest.
  - The check runs every 6 h and on demand. Results are stored in table `docker_image_updates`. Stacks can be marked
    "ignore updates".
- **Notifications**:
  - Triggers:
    - `die` with a non-zero exit code that no own operation explains (correlated with `docker_operations`);
    - `oom`;
    - `health_status: unhealthy`;
    - a restart loop (3 restarts in 5 min).
  - Delivered as a desktop notification with an *Open logs* action. Debounced per container and switchable per stack;
    default on for own stacks, off for external projects and single containers.

### HTTP API `/api/docker-manager`

Authentication and authorisation match `/api/containers`: browser sessions, or bearer tokens with admin scope
(`internal/server/auth.go:1002-1016, 1192-1193`). `docker.readonly` turns every route that changes containers,
stacks or helper resources into 403; host management stays allowed, and lint and convert change nothing. Protected
AuraGo containers reuse `containerActionAllowed` and `?confirm=protected`
(`internal/server/containers_protection.go:683-698`).

| Area | Routes |
| --- | --- |
| Hosts | `GET/POST /hosts`, `PUT/DELETE /hosts/{host}`, `POST /hosts/{host}/test` (engine, runner, IO container), `POST /hosts/import-nest/{nest}`, `GET /hosts/{host}/overview`, `GET /hosts/{host}/helpers`, `POST /hosts/{host}/prepare`, `GET /hosts/{host}/df` (Docker disk usage, cached 5 min) |
| Stacks | `GET/POST /hosts/{host}/stacks`, `GET/DELETE /hosts/{host}/stacks/{name}`, `PUT /hosts/{host}/stacks/{name}/files`, `GET /hosts/{host}/stacks/{name}/volumes`, `GET /hosts/{host}/stacks/{name}/logs?follow=1` (SSE), `POST /hosts/{host}/lint`, `POST /hosts/{host}/stacks/{name}/ops`, `POST /hosts/{host}/projects/{project}/{start,stop,restart}`, `GET /hosts/{host}/projects/{project}/logs?follow=1` (SSE), `POST /hosts/{host}/stacks/{name}/adopt` (phase 4) |
| Operations | `GET /operations`, `GET /operations/{id}`, `GET /operations/{id}/stream` (SSE) |
| Containers | `GET /hosts/{host}/containers`, `POST .../containers/{id}/{start,stop,restart,pause,unpause,kill}`, `DELETE .../containers/{id}`, `GET .../inspect` (secret env redacted), `GET .../logs?follow=1` (SSE), `GET .../terminal` (WebSocket, same protocol as `containers_terminal.go`; refused under `docker.readonly`) |
| Resources | `GET /hosts/{host}/{images,volumes,networks}`, `POST .../images/pull`, `DELETE .../images/{id}`, `DELETE .../volumes/{name}`, `POST/DELETE .../networks` |
| Cleanup | `POST /hosts/{host}/cleanup/preview`, `POST /hosts/{host}/cleanup` (explicit ids only) |
| Updates | `GET /updates`, `POST /updates/check` |
| Live | `GET /stream` (SSE: events, stats, host stats, host status, operation state changes; one per desktop client, identified by its `stream_id`), `PUT /stream/subscriptions` |
| Helpers | `POST /convert/docker-run`, `POST /assist`, `GET/POST/DELETE /templates` |

Errors follow the existing envelope (`{status:"error", code, message}`) with stable codes, for example
`host_unreachable`, `operation_in_progress`, `revision_conflict`, `stack_name_invalid`, `runner_image_missing`.

### Desktop app `docker-manager`

- **Module split** as in Infra and RuView (`ui/js/desktop/apps/AGENTS.md:106-139`): every file under 1100 lines, no
  DOM work at load.
  - `docker-manager-model.js`: pure state and derivations.
  - `docker-manager-store.js`: API calls, SSE, subscriptions, optimistic state.
  - `docker-manager-views.js`: rail, list, detail.
  - `docker-manager-editor.js`: CodeMirror with YAML, lint gutter, diff.
  - `docker-manager-live.js`: the live stream client (subscriptions, stats rings, host stats).
  - `docker-manager-charts.js`: sparklines and gauges as SVG.
  - `docker-manager-logs.js`: the log follow panel.
  - `docker-manager-terminal.js`: the xterm terminal panel.
  - `docker-manager-panes.js`: stack, project and container panes with their live parts.
  - `docker-manager-resources.js`: images, volumes, networks, cleanup.
  - `docker-manager-dialogs.js`: hosts, new stack, confirmations, conflicts.
  - `docker-manager-palette.js`
  - `docker-manager.js`: composition root.
- **Styles**: `ui/css/desktop-app-docker-manager.css`, design tokens only, with reduced-motion gating and no `cc-`
  prefix.
- **YAML in CodeMirror**: add `@codemirror/lang-yaml` to `scripts/build-codemirror.js`. The bundle stays vendored and
  offline.
- **Registration**:
  - `BuiltinApps()` entry in `internal/desktop/types.go:795` (category `system`, icon `docker-manager`). The app shows
    only when Docker is enabled, via a new `docker` requirement key in `internal/desktop/capabilities.go:24`.
  - `DESKTOP_APP_ASSETS` and `APP_I18N_SECTIONS` in `ui/js/desktop/core/module-loader.js:22,631`, and a
    `renderAppContent` branch in `ui/js/desktop/core/menus-and-routing.js:1550`, followed by `npm run build:ui` and
    `npm run check:ui`.
  - Icons in every icon theme manifest (`ui/img/{papirus,whitesur,buuf-nestort,tela,candy,fluent,windows-xp}`).
  - Strings in all 16 files under `ui/lang/desktop/`.

## User experience

- **Main window**: three panes.
  - **Rail**: host switcher with a status dot, including *All hosts*; navigation for Stacks, Containers, Images,
    Volumes, Networks and Templates, each with a count; host gauges for CPU, RAM and disk at the bottom.
  - **List**: search, a *New stack* button, and three groups: own stacks, external compose projects, single
    containers. Protected AuraGo containers carry a lock.
  - **Detail**:
    - Header with name, host, path and uptime; icon buttons for restart and stop; one accent button for the most
      useful next action (*Update* when updates exist, *Start* when stopped).
    - Tabs: Overview (services, ports as links, health, sparklines), Compose, .env, Logs, Terminal.
- **Status colours**: green when all services run; amber when partial, unhealthy or restarting; red when exited with
  an error; grey when stopped or a draft. A colour always comes with text or an icon.
- **All hosts**: stacks of every host in one list with a coloured host tag. An unreachable host stays in the list,
  greyed out, with the classified reason and a *Diagnose* button.
- **New stack**:
  - Entry points: paste, or drop `compose.yaml` (and `.env`) on the window; a template; *Convert docker run*; *Describe
    with AI*.
  - The editor shows a live preview next to the code: services, port check against the target host, storage
    (volumes to create, absolute and relative host paths), missing variables (with *Create random value in .env*), and
    lint hints.
  - *Save draft* writes the files only. *Deploy* writes the files and then runs check (`config`), pull with
    per-image progress, `up -d`, and the health wait.
  - On failure the editor reopens with the affected line marked and an *Explain* action.
- **Progress**: every operation streams compose output into a collapsible drawer at the bottom of the window. Leaving
  the stack does not cancel it. Coming back reattaches.
- **Destructive actions** (`down -v`, delete stack, remove volume): the confirm dialog names exactly what disappears
  (volumes with sizes, folder path, images). Removing a volume needs its name typed.
- **Cleanup assistant**: sections for stopped containers, unused images, unused volumes and unused networks, with
  per-item checkboxes and sizes and the total to be freed.
  - Volumes are never preselected. Protected resources and Docker's default networks are never listed.
  - Only the checked ids are removed, not a blanket `prune`. A closing summary lists every item whose state changed
    since the preview.
- **Keyboard**: Ctrl+K opens the command palette ("restart immich", "logs paperless", "new stack on nas"); Ctrl+S
  saves; Ctrl+Enter saves and deploys; arrow keys move through lists; Esc closes the drawer and dialogs.
- **Themes**: tokens only, verified in all 14 themes and in light and dark; reduced motion respected; no external
  assets.

## Agent integration

- **Tool `docker_stacks`**: registered when Docker is enabled (next to the existing `docker` tool,
  `internal/agent/native_tools_integrations.go:623-656`).
  - Operations: `list_hosts`, `list_stacks`, `get_stack`, `create_stack`, `update_stack`, `deploy`, `stop`, `down`
    (without volumes), `restart`, `pull`, `update`, `logs`, `status`, `check_updates`.
  - `get_stack` redacts `.env` values whose keys match `*_KEY`, `*_PASSWORD`, `*_TOKEN` or `*_SECRET`. Writes return
    lint results and a diff.
- **Policy**: agent-authored compose content is rendered in the runner (`docker compose config --format json`), and
  the resolved model goes through the existing host-access evaluation
  (`EvaluateDockerComposeHostAccess`, `internal/tools/docker_compose_host_access.go:373`). The bind root is the stack
  directory. With `docker.allow_host_access` false, the host tier blocks the socket, `privileged`, devices and binds
  outside the stack directory; the always-tier (AuraGo state, master key) stays enforced. A refused deploy leaves the
  previous files in place.
- **v1 limits**: the agent never deletes volumes or stack folders and never touches protected AuraGo containers.
- **Editor AI help** (`POST /assist`): generate a stack from a description, explain an operation error, fix a lint
  hint, and convert `docker run`. It follows the nearest existing in-app assistant pattern (CYD Creator). The result is
  always a diff that the user accepts into the editor; it is never deployed directly.

## Security

- Host secrets never leave the vault towards the UI; the UI sees only "set". Plain-TCP hosts get a warning badge, as
  for Invasion (`internal/server/invasion_docker_security.go:13`).
- Runner and IO containers are hardened (see above) and use a pinned digest.
- Inspect output redacts secret environment values, as the existing redaction does.
- Lint hints inform the admin and never block the admin. Blocking applies only to the agent, as today.

## Error handling

- Unreachable host: classified reason, backoff, a non-blocking banner, retry on focus.
- Failed operation: exit code, the last lines and the marked line in the editor where the error names one. Files stay
  saved, and the journal entry stays.
- Concurrent operation: 409 with the running operation id; the UI attaches to its stream instead of showing an error.
- Revision conflict: 409 with both versions; a dialog shows "yours" against "on the host" as a diff.
- AuraGo restarts mid-operation: reattached by runner label, or finalised from the exit code.
- Runner image missing: copied from the local engine; otherwise an exact command in the error.

## Preserved behaviour

`/containers`, `/api/containers`, the SSE `container_update` event, the `docker` agent tool and Invasion's nest
deployment all keep working as before. The transport extraction must not change any Invasion result; the existing
Invasion tests must pass unchanged.

## Testing

Lean, per the user's preference: essential tests only and one combined review per batch.

- **Go unit tests**:
  - lint and the `docker run` converter (table tests);
  - name validation;
  - revision conflicts;
  - update check against a fake registry (`httptest`);
  - runner and IO payload golden tests (binds, hardening, labels);
  - operation reattach after restart;
  - transport regression tests for the code moved out of Invasion.
- **Handler tests** against a fake Engine (`httptest`) for the routes the UI uses.
- **Live tests**, gated by environment variables:
  - against local Docker Desktop, following `reference_local_docker_sidecar_probe`;
  - against aurago-test, with a dedicated stacks directory and prefixed project names. Existing containers, apps
    and data there are never touched.
- **JS**:
  - Node VM tests for model and store, as in `scripts/test-infra-ui.mjs`;
  - `ui/desktop_docker_manager_browser_test.go` on `desktopAuditBrowserWith` with a fake API;
  - theme sweep coverage.
  - The existing guards apply automatically: i18n keys in all languages, no external assets, no `cc-` prefix, and the
    theme CSS contract.

## Phases

Each phase gets its own implementation plan and leaves a usable state.

1. **Foundation**:
   - `internal/dockerhost` (registry, vault secrets, transport extraction with Invasion switched over, version
     negotiation, health);
   - host API and read API for containers, images, volumes and networks;
   - app skeleton: registration, icons, 16 locales, window, host switcher and dialogs, Containers view.
2. **Stacks**:
   - runner and IO container, `internal/dockerstacks` (discovery, files with revisions, lint, operations, journal,
     reattach), stack API;
   - UI: list, detail (Overview, Compose, .env), new-stack flow (paste, drop, `docker run` converter), progress drawer,
     command palette;
   - CodeMirror YAML.
3. **Live**: `internal/dockerwatch` events and stats, log follow, terminal with host parameter; UI Logs and Terminal
   tabs, sparklines, host gauges.
4. **Care**: images, volumes and networks views, cleanup assistant, update check, notifications, registry
   credentials, adopt.
5. **Agent and templates**: `docker_stacks` tool, editor AI help, templates (generated from
   `desktopstore.DefaultCatalog()` plus own templates), manual chapter in `documentation/`.

## Out of scope for v1

Auto-updates; Swarm and Kubernetes; editing files other than the compose file and `.env`; a volume file browser;
registry management beyond stored credentials; per-user permissions; backups of stack data.

## To verify in the plans

- Runner image: exact tag, digest, size and compose version; whether `--progress json` is available. *Plan 2:*
  `docker:29.9.2-cli`, index digest `sha256:5e6baf67325a79f64ed3bed6863954d5e05d0f3d4e57cbdb14809346814bbfdb`,
  about 64 MB compressed per arch, Compose 5.6.1; the `--progress json` field names are confirmed by the plan 2 live
  test. Stack routes need Engine API 1.41.
- Docker Desktop (Windows, macOS): behaviour of a host-path stacks directory inside the Desktop VM. Still open: the
  plan 2 live test ran on a Linux engine (aurago-test, Engine 29.8.2), where it passed.
- Live data (*plan 3*): that the Engine flushes the `/events` headers before the first event, the stats JSON on
  cgroup v1 and v2 and with `one-shot`, `tail=N&since=T` and `tail=0&follow=1` on log follow, and an exec round trip
  over each transport kind; checked by the plan 3 live tests.
- Rootless Docker: socket path detection, or a documented per-host setting.
