# Google Workspace through gog

AuraGo can use its managed gog helper alongside the native Google Workspace
integration. Their accounts, credentials and permissions are independent. No
mail application is installed: Gmail is available through the agent.

## Setup

1. Open **Google Workspace through gog** in Configuration and explicitly install
   the helper. This downloads the approved package for this host and checks both
   the ZIP and executable against AuraGo's embedded SHA-256 manifest.
2. Enable the integration, add an account and save. Account IDs are stable;
   display names can change. Enable agent access, Desktop access, or both.
3. In Google Cloud Console, select or create a project, configure the OAuth
   consent screen and add your test users if the project is in testing mode.
   Enable the APIs you need: Gmail, Calendar, Drive, Docs, Sheets, Slides, People
   and Tasks. Create a **Web application** OAuth client. Enter its client ID and
   save the secret with the separate Vault button.
4. Select each service and its read/create/update/delete grants. Gmail sending
   has its own grant. The integration starts disabled and read-only; leaving
   read-only mode requires an explicit configuration change. Save before login.
5. Start login. Add the exact displayed redirect URI to the OAuth client's
   authorized redirect URIs, then open the login link and grant consent. Google
   requires HTTPS for remote web callbacks; localhost development is the
   exception. For a remote/headless setup, use AuraGo's normal HTTPS address.
   If the automatic callback cannot reach AuraGo, paste its complete returned
   URL into the manual callback field. It must match that same redirect URI.
6. Click **Test connection** for each selected service. Docs, Sheets and Slides
   require an existing test object's ID. These tests only read. Merely opening
   Configuration or listing accounts never contacts Google.

OAuth still needs a Google Cloud project, enabled APIs and user consent. gog
does not remove those requirements. Adding scopes later requires another login.
Testing-mode refresh-token lifetime and organization restrictions remain subject
to Google's OAuth policies.

Execution requires AuraGo's shell and network permissions and the selected
sandbox/host execution policy, including during connection tests. On Windows,
host execution must be explicitly permitted. Missing permissions are reported;
the integration never enables them. Installation additionally requires network
and filesystem-write permission. The helper runs in the AuraGo process user's
environment, inside the same container in Docker, with no sidecar. Persist the
normal data directory to retain the installed helper across container updates.

## Accounts, grants and credentials

Secrets live only in Vault keys `oauth_gog_client_<id>` and `oauth_gog_<id>`.
These use the existing OAuth secret export-deny policy. Processes receive a
short-lived access token, an isolated `GOG_HOME`, and a minimal environment;
they cannot use a pre-existing local gog login. Refreshes are serialized per
account and use Vault compare-and-swap. Disconnecting or changing permissions
cancels active work and prevents stale refreshes from restoring a login.

Office operations sometimes also need Drive: export needs Drive read; Docs and
Slides creation need Drive create; Sheets creation in a specified folder needs
Drive update. Uploading an Office file as a new Google document needs both Drive
create and the corresponding document service's create grant.

## Agent and Desktop

`gog_accounts` lists locally configured agent-visible accounts. The typed tools
`gog_gmail`, `gog_calendar`, `gog_drive`, `gog_docs`, `gog_sheets`, `gog_slides`,
`gog_contacts` and `gog_tasks` require `account_id` on every resource request.
Their manuals describe the 68 admitted operations. There is no arbitrary CLI,
flag, URL or Google API interface. Google responses are isolated as external
data. Native Google tool names remain unchanged.

* **Files:** select a Drive source per account. Object IDs identify entries even
  when their names are identical. Replacement of a binary file uses its observed
  Google version. Google documents open through the explicit Office-copy flow.
* **Calendar:** choose the local source or a Google account/calendar. Edits go
  directly to that source. A recurring event requires a choice of this event,
  this and future events, or the whole series. End dates for all-day events are
  exclusive; timed events include their UTC offset and selected time zone.
* **Todo / People:** Google task lists and contacts are separate sources. Local
  priorities, reminders, contacts and knowledge-graph data are not synchronized.
* **Writer / Sheets:** the File menu imports a Google document as a new local
  DOCX/XLSX file, or exports the saved document as a new Google file. Existing
  Google originals and local destination files are never overwritten by this
  flow. Conversion can lose formatting, formulas or unsupported features.
* **Slides:** the agent reads/edits presentations and creates slides from text
  Markdown. File/PDF export uses the existing viewer. Embedded Markdown images,
  external renderers and diagrams are intentionally rejected.

Cross-source moves retain a Google source because gog exposes no conditional
delete based on the observed version. A confirmed copy may therefore exist at
the destination while the source remains; inspect the result before any manual
cleanup. Transfers stage privately, are cancellable and are limited to 64 MiB.
CLI JSON output is limited to 4 MiB and individual calls to two minutes.

If a started mutation fails or is interrupted, its result can be
`external_status_unknown`. Check the selected Google account before attempting
another write. AuraGo never automatically repeats it.

## Maintainer contract

The approved helper is `0.43.0-aurago1`, upstream commit
`3b5122f4c81c5df6df38ee48e01327f1034364ee`. The versioned patch in
`internal/gog/build/single-attempt.patch` disables retries for non-read HTTP
methods and Google SDK resumable upload retries on the admitted Drive upload
paths. Do not upgrade the CLI independently of its catalog and these tests.
Review new commands for their own SDK retries and secondary network calls.

Run `go run scripts/build-gog.go` with Go 1.27.2. The builder requires an exact,
clean source checkout, applies the patch, runs transport/upload retry tests,
builds Linux/Windows/macOS on amd64/arm64 and generates deterministic ZIPs and
`internal/gog/build/artifacts.json`. The regular release scripts run this before
compiling AuraGo, include the packages in release checksums and publish them as
release assets. Building this helper alone does not publish a release.

To change the admitted catalog, dump `gog schema --json` from the pinned helper
and run `python scripts/generate-gog-catalog.py <schema.json>`. Review the runtime
validators, permissions, manuals, native schema and the curated contracts in
`antibyte/agotrain` together. Preserve the existing native tool catalog.

Admin routes are under `/api/gog/`; Desktop routes are under
`/api/desktop/integrations/gog/`. All probes and mutations use POST. Desktop
storage reuses `/api/desktop/storage/` with additive account/object IDs.

Run focused Go tests matching `Gog`, existing Desktop storage regressions, locale
checks, `node scripts/build-ui-bundles.js --check`, and
`node scripts/test-gog-desktop.mjs` (Playwright; `PLAYWRIGHT_CHANNEL=msedge` is
supported). The browser fixtures use synthetic responses, not Google accounts.
The separate live acceptance requires a named test account and explicit test
objects for writes in all eight services, including Office conversion. Six
cross-compiled binaries alone do not establish native execution on six hosts.
