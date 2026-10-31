# Kaggle and the Notebook IDE

AuraGo provides one Kaggle account per installation, seven native agent tools and the **Kaggle** Desktop app. The app and chat use the same services, grants, project files and durable job receipts.

## Setup

1. Open **Configuration → Kaggle** (`/config#kaggle`). Enable the integration and save the policy with the main configuration Save button.
2. Store a current Kaggle API token with **Save credentials**. Alternatively select **Legacy credentials**, save the username in configuration and store the legacy key. Blank secret fields retain existing values; the separate delete action removes a secret.
3. Test the saved credentials. The test reads account identity/quota only. It does not create, publish, execute or submit anything.
4. Enable only the required capabilities. The integration and local runtime start disabled; Kaggle starts read-only. Downloads additionally require filesystem writes. Python and writable Docker access are required for local kernels.

Secrets live in the encrypted Vault (`kaggle_api_token`, `kaggle_legacy_key`, opaque upload receipts under `kaggle_upload_`). They are registered for scrubbing and excluded from Python export. AuraGo never imports credentials from user home directories, exposes them to the browser or mounts them into a kernel. There is no OAuth flow or Kaggle chat-provider registration.

## Resource workspace

The Desktop app contains Overview, Datasets, Models, Notebooks, Competitions, Benchmarks, Forums and Jobs. Choose an operation to display its typed fields. Defined API values use dropdowns; optional empty values use the operation's default. Details display external text inertly. Pagination follows Kaggle cursors. Projects hold imported notebooks, selected datasets and downloaded results. Text/log/CSV/JSON files have bounded previews; binary files can be exported.

Search uses one query field; expand **More filters and options** for the remaining API fields. Creation dialogs retain invalid or conflicting input so you can correct it. Closing a dialog returns keyboard focus to its originating control. A passive configured status means credentials are stored; only the explicit connection test verifies access.

| Area | Supported scope |
| --- | --- |
| Datasets | Search, metadata, file/version listing, download, upload, private creation, metadata changes, new versions, publication through metadata and deletion. |
| Models | Search, model/variant/version metadata, uploads/downloads, creation, changes and supported deletions. |
| Notebooks | Search/import, editable copies, revisioned local saves, whole-version Kaggle saves/runs, status, logs, outputs, publication and deletion. |
| Competitions | Overview/rules/pages, files, leaderboard, limits, downloads, file submission and pinned notebook-version submission, result status. |
| Benchmarks | Task creation/updates/publication, model selection, scheduling, quota/status/log/output access and tracked-run cancellation. |
| Forums | Read-only forums, topics and comments. |

Competition rules and participation requirements must be accepted by the user on Kaggle. Organizer administration, forum posting and unsupported benchmark deletion are not exposed. Benchmark scheduling accepts one task and one model per durable job; benchmark agent execution is outside this integration.

## Permissions and limits

Remote capabilities are separately controlled by `allow_download`, `allow_create`, `allow_update`, `allow_publish`, `allow_delete`, `allow_submit`, `allow_remote_execute`, `allow_gpu`, `allow_tpu` and `allow_internet`. Read-only blocks remote changes and both execution modes. Project editing remains possible when filesystem writes are granted. Existing global agent, Desktop and Docker gates remain authoritative.

Writes are restricted to the authenticated owner unless `allowed_owners` explicitly includes another owner. Public creation and changes to existing public resources require publication permission. New resources default to private where supported; deleting old dataset versions also requires deletion permission. The granted agent operations need no extra per-action confirmation, subject to the existing global action policy.

| Default | Limit |
| --- | --- |
| One transfer | 2 GiB |
| Total managed project storage | 10 GiB |
| Concurrent remote compute | 1 job |
| Kaggle notebook run | CPU, no internet, up to 60 minutes |
| Local sessions | 1 |
| Local session resources | 2 CPU cores, 4 GiB RAM |
| Local idle timeout | 15 minutes |
| Local cell deadline | 30 minutes |

The limits are configurable. The configuration editor rejects values above 300 seconds per request, 720 minutes per remote run, 8 local sessions, 240 minutes idle and 720 minutes per cell before saving; these maxima match backend normalization. Transfers stream into temporary regular files, enforce actual byte/disk budgets, then publish atomically. Destination collisions are refused. Paths cannot traverse projects or access symlinks/special files. ZIP archives are kept opaque, never implicitly extracted. Redirected storage requests carry no Kaggle authorization or cookies.

## Local Python versus Kaggle execution

**Local – Jupyter** starts an AuraGo-managed container. Select the project files to copy into the session. Python cells share variables until restart, stop, revocation or idle expiry. Shift+Enter executes a code cell; the toolbar also offers Run all, Interrupt and Restart. The agent can stop a session through `kaggle_runtime.stop`; disabling local execution stops affected sessions. Internet is independently opt-in (`internet: true` for an authorized runtime start); the Desktop's normal start is offline.

Outputs received during a save remain unsaved until a later save includes them. Editing a cell during its prerequisite save prevents that execution, so an older saved source cannot masquerade as the current draft. A successful save retires the unchanged recovered draft without removing newer work from another window. If Docker cannot confirm container removal, cleanup remains pending and retains its session slot until a retry succeeds.

**Run on Kaggle** freezes the displayed notebook content and lets you explicitly select Kaggle data sources. It submits a whole private CPU version. Local variables and unselected files are not transferred. Native `save_kernel` uses `kernelExecutionType: QUICK_SAVE` to save without execution and `SAVE_AND_RUN_ALL` for a whole run. Interactive remote Kaggle cells are unsupported.

The IDE preserves nbformat 4 metadata, unknown fields, attachments and original output bundles. Saves require the current content revision. A conflict preserves the local draft for export; it never replaces a newer server revision. Each output carries session, notebook revision, cell and execution identity. Edited content does not receive late output. HTML is sanitized and displayed in an opaque sandboxed iframe; JavaScript and widget extensions remain inert while their original data survives export.

Closing a window detaches observers and timers. Jobs continue, and local sessions remain available until their idle limit. Backend shutdown removes owned containers and loses variables. After restart AuraGo reconciles known remote references and marks interrupted uncertain operations explicitly. Neither reconnect nor restart automatically reruns code. SSE replay retains at most 10,000 events/64 MiB; an expired replay window is reported rather than silently appearing complete.

### Cancellation and uncertain outcomes

Mutations are never retried after a lost response. An optional 32-hex request `id` binds the account and canonical arguments; repeating it returns the existing receipt. `unknown` remains visible and retains a remote compute slot when applicable. Do not create a replacement job merely to hide uncertainty.

An administrator can open **Jobs → Release slot** for an `unknown` receipt after checking on Kaggle that no execution remains active. The confirmation checkbox is required. This changes only the local status to `resolved_unknown`: it does **not** stop a Kaggle job or establish success/cancellation. The original account, request, idempotency key, result and error remain intact, and the release emits a job event. Repeating the release returns the same receipt; other job states cannot be released this way. The action works with Kaggle disabled/read-only, without credentials or network access. There is no automatic release or agent-tool equivalent.

Benchmark cancellation uses recorded run IDs only. Its request is durably claimed before sending, and completion requires a remote terminal state. A configured remote timeout requests cancellation of a tracked benchmark run. Failed/uncertain cancellations remain visible and are not automatically repeated.

Remote cancel buttons show a disabled reason when credentials or global network access are missing. Local job cancellation stays available; administrative cancellation does not require Kaggle execution/write grants to remain enabled.

The pinned SDK's `SaveKernel` response does **not** promise a `kernelSessionId`, while its cancellation endpoint requires one. AuraGo does not derive or guess that ID from a kernel ID or version number. A run without a verifiable session ID offers **Cancel on Kaggle**. If a session ID is supplied, AuraGo can request cancellation and follow the confirmed state. Notebook timeouts are also sent in the run request. This upstream limitation must not be presented as automatic cancellation support.

## Agent and HTTP contracts

Tools: `kaggle_datasets`, `kaggle_models`, `kaggle_notebooks`, `kaggle_competitions`, `kaggle_benchmarks`, `kaggle_forums`, `kaggle_runtime`. The six platform tools take a fixed `operation`, its `parameters`, optional idempotency `id`, and `project_id`/`file` for transfers. Operation-specific structures with incompatible shapes use JSON strings in the agent schema; the backend decodes and validates the selected SDK structure. Successful uploads return `upload:<job-id>` references, never storage tokens.

The runtime tool manages projects, revisioned notebook files and local sessions. `events` takes an `after` cursor and returns up to 100 events plus `next_cursor` and `gap`; use it to observe local cell output in chat. `read_file` returns a bounded UTF-8 preview. `jobs` returns durable remote receipts. All external results pass the agent's existing protection/isolation path.

HTTP routes under `/api/kaggle/` include `status`, `capabilities`, `test`, `quotas`, `operations`, `runtime`, `projects`, `jobs` and `events`. They use existing Desktop authentication/scopes and browser CSRF rules. POST operation classification is semantic: Kaggle also uses POST for reads. Credential changes require global administrator permission. Notebook PUT uses `If-Match` or create-only `If-None-Match: *`. SSE supports `Last-Event-ID`; credentials are rechecked during long connections.

`POST /api/kaggle/jobs/{id}/resolve-unknown` accepts `{"confirmed_inactive":true}`. It requires Desktop write access (`desktop:write` or `desktop:admin` for tokens) **and** global `admin`; authenticated browser owners use the existing admin identity and same-origin CSRF check. `status.can_resolve_unknown` exposes this capability. Missing confirmation returns 400, a missing receipt 404 and any state other than `unknown`/`resolved_unknown` 409. It performs no Kaggle request.

State is held in `data/kaggle.db` and `data/kaggle/projects/`. The database is part of the existing protected SQLite inventory and backup handling. Upload receipt secrets remain in the Vault. Preserve both the project files and normal encrypted Vault/database backups when migrating an installation.

## Runtime delivery and verification

`internal/notebooks/docker.go` pins the published runtime OCI index `ghcr.io/antibyte/aurago-kaggle-notebook@sha256:591a7ef45ba78931aa9468bb55957c2918d7fefb271aeb70d734d12e0ca60eac` (protocol tag `protocol-1-20261026`). Inspection, pulling and container creation use this exact digest; a missing or failed image never falls back to a tag. Its Dockerfile uses the 2026-10-26 scipy-notebook multiarch OCI digest `sha256:0698ec1bc3ecf0ff30e571af74816459f1a2947b560236af0d28e13900a3b296`. Each session uses UID/GID 1000:100, a read-only root, dropped capabilities, no-new-privileges, CPU/RAM/PID limits, bounded private tmpfs, no published port and no host or Docker-socket mounts. A fixed `jupyter_client` bridge communicates over Docker exec attach with bounded JSON frames.

The existing Docker publication workflow builds and smokes amd64/arm64 before publishing/signing the notebook image. Manual dispatch can select `kaggle-notebook`; normal release tags include it. The image must be published before an installation can pull it. Bump the runtime image tag when the bridge protocol/image changes, verify the workflow signature and both platform manifests, then update the runtime index digest; do not silently replace a published contract. The full Desktop resources remain part of the backend's version-bound asset package.

Useful checks:

```sh
python scripts/generate-kaggle-contracts.py --check
go test ./internal/kaggle ./internal/notebooks ./internal/config
go test ./internal/server ./internal/agent ./internal/tools -run Kaggle
docker build -t aurago/kaggle-notebook:test deploy/docker/kaggle-notebook
python scripts/test-kaggle-runtime.py --image aurago/kaggle-notebook:test
# Optional real Docker manager and browser fixtures:
AURAGO_KAGGLE_DOCKER_SMOKE=1 AURAGO_KAGGLE_TEST_IMAGE=aurago/kaggle-notebook:test go test ./internal/notebooks
AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/server -run 'TestKaggle.*Browser'
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run Kaggle
# Linux:
go test -race -p 1 ./internal/kaggle ./internal/notebooks
node scripts/build-ui-bundles.js --check
```

Training contracts and the manual router live in agotrain; follow [training.md](training.md) for sequential source/training pins. Fixture tests establish request formats, state transitions and permissions. Actual Kaggle writes require dedicated test resources and credentials; public submissions are never an automatic CI smoke.

Sources: [Kaggle API](https://www.kaggle.com/docs/api), [official notebook client documentation](https://github.com/Kaggle/kaggle-cli/blob/main/docs/kernels.md), [official SDK](https://github.com/Kaggle/kaggle-sdk-python), [Jupyter client API](https://jupyter-client.readthedocs.io/en/stable/api/jupyter_client.html). Contract generation pins `kagglesdk 0.1.38` and verifies its wheel SHA-256; no Kaggle CLI is installed at runtime.
