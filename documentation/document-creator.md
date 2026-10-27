# Document Creator

`maroto` creates structured PDFs inside AuraGo. `gotenberg` adds HTML/Markdown,
public webpage capture, Office conversion, PDF merging and PNG screenshots.
Enable the tool and select its backend in Config → Document Creator.

## Rendering and files

Supplied HTML/Markdown is offline by default. The administrator checkbox
`tools.document_creator.allow_remote_resources` permits public external images,
fonts, stylesheets and scripts for `html_to_pdf`, `markdown_to_pdf`,
`screenshot_html` and Gotenberg `create_pdf`. A caller's `block_remote_content:
true` always enforces offline rendering. Public URL capture is independent of
this checkbox; Flows retain their existing offline restriction. Detective report
PDFs always use resource-free HTML with the offline restriction.

Both backends validate the same sections before rendering. Nonempty sections
take precedence over `content`; `[]` permits the plain content fallback. Titles
and section headers render once. Tables may have no rows; otherwise every row
must match a 1–12-column header. Errors identify the section and row.

Conversion requires one regular source file, merging at least two. The maximums
are 20 files, 32 MiB per file and 64 MiB total. Path protections and limits apply
before staging and again during reads, including direct backend calls. Generated
PDF/image responses remain limited to 10 MiB. Output creation never overwrites:
`report.pdf` is followed by `report-1.pdf` through `report-999.pdf`, then a clear
error. Responses and media registration use the actual filename. Unicode names
are retained; controls, NTFS streams and reserved device names are rejected.

Cancellation stops staging and section/table assembly. Maroto's synchronous
`Generate()` cannot be interrupted; cancellation discards its result. Canceled
jobs do not publish or register output. There is no automatic document retention,
read-only tool option or additional PDF layout mode.

## Private renderer and runtime proof

The URL defaults to `http://127.0.0.1:3000` natively and
`http://gotenberg:3000` inside Docker. Native loading normalizes the old exact
Docker default; custom hosts, ports and path prefixes remain unchanged. Empty
configuration selects the runtime default. The saved-settings connection test
returns `reachable`, `security_verified` and `restart_required`; unsaved UI
changes must be saved or discarded first.

Before **every conversion POST**, AuraGo retrieves `/debug` on the same origin
without a positive cache. Missing, oversized (>1 MiB), malformed or incomplete
diagnostics prevent upload. Only the version and required flag fields are used;
raw diagnostics never enter UI/model responses or logs. Supported versions are
8.38.0 or later within major version 8. `/health` proves reachability only.

Required settings, following [Gotenberg's outbound filtering](https://gotenberg.dev/docs/outbound-url-filtering):

- `CHROMIUM_DENY_PRIVATE_IPS`, `LIBREOFFICE_DENY_PRIVATE_IPS`,
  `WEBHOOK_DENY_PRIVATE_IPS` and `API_DOWNLOAD_FROM_DENY_PRIVATE_IPS` are true.
- All four allow-lists are empty. Additional restrictive deny-lists are allowed.
- Chromium proxy server and host resolver rules are empty; all four environment
  proxy options are false. Chromium web-security bypass, foreign-file access,
  shared browser context, ignored certificate errors and insecure-localhost
  bypass are false.
- `API_ENABLE_DEBUG_ROUTE=true`; keep this unauthenticated API/debug access
  private to AuraGo. External operators must provide this access themselves.

Private targets remain denied after the admin resource grant, including renderer
redirects and subresources. Conversion POSTs refuse redirects and are never
automatically retried.
Per-call timeouts use the current saved setting (120 seconds by default), with
the shared standard transport retaining connection reuse and same-origin
redirect checks for diagnostics and conversion.

## Managed profile and migration

Compose and native auto-start pin
`gotenberg/gotenberg:8.38.0@sha256:c36cdd730893e3941d27a196ff8b68675e473642d067ec7ee3668b9adaa37b75`
(amd64/arm64), drop all capabilities except `CHOWN`/`FOWNER`, enable
`no-new-privileges`, and allocate 1 CPU, 1 GiB RAM and 256 MiB shared memory.
Docker 8.38.0 matches the code in the [Go-module-only 8.38.1 release](https://github.com/gotenberg/gotenberg/releases/tag/v8.38.1).

Native management requires enabled, writable Docker access, a local Engine and
an HTTP loopback target without a path prefix; the URL selects the published
port. Remote engines/URLs and Compose service addresses are never replaced by a
local container. New containers carry `aurago.managed-by=document-creator` and
`aurago.gotenberg.profile=1`. Unrelated running Gotenberg images are not adopted.

At AuraGo startup an owned outdated container is replaced only after the pinned
image is available. The previous container is stopped and renamed as a recovery
object. The replacement must pass health and security proof before the old
container is removed, without deleting volumes. A failed pull leaves the old
container untouched; later failures retain it stopped. An unverified replacement
is stopped and conversions remain blocked. Recovery objects require explicit
operator review; they are never silently restarted.

Legacy ownership requires the exact former AuraGo name, image defaults,
capabilities, resources and loopback port binding. Compose labels, mounts and
ambiguous configuration prevent adoption. During runtime a config change may
create a missing sidecar, but replacement of an existing one reports **restart
required** so active jobs are not interrupted. Pull/start operations end with
server shutdown. Read-only and Docker permission gates also apply to migration.

## Verification

```powershell
go test ./internal/tools ./internal/agent ./internal/server ./internal/config -run 'DocumentCreator|Gotenberg|RenderReportHTMLPDF|TestC13' -count=1
$env:AURAGO_RUN_BROWSER_SMOKE='1'
go test ./ui -run 'TestDocumentCreatorPolicyAndDiagnosticsBrowser|TestConfigRerenderingTogglesKeepDraftBrowser' -count=1
$env:AURAGO_RUN_GOTENBERG_DOCKER='1'
go test ./internal/tools -run '^TestGotenbergIsolatedDockerAcceptance$' -v -count=1 -timeout=6m
```

The opt-in Docker test pulls only the pinned image and creates two uniquely
named internal networks and disposable containers. It checks real PDF/PNG
outputs, the offline default, public-resource opt-in, private targets, redirects
and subresources, then removes its own resources. It requires a running Linux
amd64/arm64 Docker daemon and unused subnets `93.184.216.0/24` and
`192.168.245.0/24`; it never starts the daemon or modifies existing containers.
Run the focused Go suite with `-race` on Linux for concurrency verification.
