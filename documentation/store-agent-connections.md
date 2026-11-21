# Store agent connections

Use **Connect to agent** on an installed Software Store app. Finish the app's
own setup first, save a dedicated account or access token, select permissions,
then test the saved connection. The same dialog supports existing volumes.
Installation alone does not enable agent access. Connection tests only read
capabilities and never execute an app action.

Credentials are encrypted in the Vault and responses expose only whether a
credential is stored. Empty credential input preserves it; removal is explicit.
AdGuard and Uptime Kuma retain their existing configuration sections and Vault
keys. Replacing a manual integration requires selecting the Store instance.
Settings use revisions: a stale dialog cannot overwrite a more recent save.

Native AuraGo uses the actual published host port. Containerized AuraGo needs a
selected Docker network shared with the app; an explicit Store update applies
network changes. A remote Docker host requires an address reachable from AuraGo.

Store operations exclude connection changes and tests. Stopping, updating,
removing, changing configuration or revoking permissions cancels active access.
Restart and successful updates recheck the saved connection. Normal uninstall
keeps settings; `delete_data` removes this binding and its owned credentials,
without deleting a different manual integration.

## AdGuard Home

Complete AdGuard's initial setup with permanent admin port **3000**. Supply its
username and password, not a Kuma-style API key. Start with reading enabled;
execution is separate and the existing AdGuard read-only gate still applies.

An existing configuration using admin port **80** requires a Store update to
add its port mapping. Select that admin port and test it. The verified endpoint
is then used when opening the app. No existing AdGuard configuration is reset.

Optional DNS publishing takes a specific LAN address and an explicit Store
update. Both **53/TCP and 53/UDP** use that exact address and port. A Docker bind
conflict fails the update and retains rollback; no random DNS port is chosen.
Router, DHCP and operating-system DNS settings are never changed automatically.

## Uptime Kuma

Create an API key in Kuma and enter it once. AuraGo verifies the existing
authenticated `/metrics` interface. **Report status changes to the agent** is a
separate option, initially off unless an existing choice is preserved. It uses
the existing deduplicated poller and isolated agent session. No Socket.IO account
or key provisioning is performed.

## Administrative API

`GET`, `PUT`, `DELETE /api/desktop/store/apps/{id}/integration` and
`POST /api/desktop/store/apps/{id}/integration/test` retain Desktop admin,
authentication, CSRF and Store operation permissions. `DELETE` disables the
binding; app uninstall with `delete_data` owns permanent cleanup. Agent requests
also obey current Desktop agent-control, network and app-specific grants, even
when the caller captured an older configuration snapshot.

## Open WebUI and the internal model relay

The relay serves only DeepTutor and Open WebUI, with independent Vault tokens,
model alias `aurago`, streaming, budget accounting and optional embeddings. It
never executes AuraGo tools. The main HTTPS server and login remain unchanged.
Port `store_llm_relay.port` defaults to 8099 and can be changed in Open WebUI's
connection dialog. Native Linux binds the Docker bridge address, Docker Desktop
binds loopback, and containerized AuraGo binds its selected application network
IP. No LAN, Tailscale or Desktop proxy publication is added. A remote Docker
Engine, missing shared network or occupied port produces an explicit error.

New Open WebUI installs receive initial provider defaults. Complete its normal
account setup, then use **Connect to agent**, enter an administrator access token,
save and test. Existing installs first use Store Update to provision their
app-scoped relay token. Saving reads the current OpenAI provider list and adds
only AuraGo; other providers, chats and persistent settings survive. The test
only reads configuration. Concurrent provider edits made directly in Open WebUI
cannot be version-checked by its API; avoid editing that list during connection
setup. An interrupted update is never automatically retried.

The relay token never appears in connection responses. Stop, disconnect,
permission revocation and configuration changes cancel running calls. Native
Docker reachability and real provider chat/embedding behavior require separate
live acceptance; fixture tests cover authentication, limits and preservation.

Upstream: [Open WebUI configuration](https://docs.openwebui.com/reference/env-configuration/).

## Workflows and snippets

n8n uses its instance-level Streamable HTTP MCP endpoint `/mcp-server/http`.
Enable it in n8n, expose existing workflows and save the dedicated MCP token.
The exact allowlist is `search_workflows`, `get_workflow_details`, and optionally
`execute_workflow`. Editing and deletion are never granted. Search previews may
include other workflows visible to the account; n8n controls MCP sharing for
details and execution.

ByteStash uses `/mcp` with a dedicated user API key. Its exact read allowlist is
`list_snippets`, `get_snippet`, `list_metadata`; the separate create grant adds
`create_snippet`. Updates and deletion stay blocked. Both connections obey global
MCP/network/Desktop gates. Missing capabilities require a compatible Store update.
Reserved managed names are `aurago-store-n8n`, `aurago-store-bytestash` and
`aurago-store-velxio`; rename a colliding manual connection first. No manual
connection is overwritten.

Node-RED requires `adminAuth` and an access token. Select Inject IDs in the dialog.
`node_red.list_flows` returns tab names/IDs and selected active Inject nodes, with
no source or payload. `trigger` sends no override and means accepted, not completed.
There is no flow editing, deployment or runtime restart operation.

OliveTin requires a dedicated user with a bearer key and selected binding IDs.
`olivetin` exposes `list_actions`, `execute`, `get_execution`. AuraGo checks the
current binding, choices and argument names, then uses upstream argument
validation before starting. Status must match both action and execution ID.
Unknown outcomes after connection loss are never automatically replayed; inspect
the returned tracking ID when available.

Upstream: [n8n MCP](https://docs.n8n.io/connect/connect-to-n8n-mcp-server/),
[Node-RED security](https://nodered.org/docs/user-guide/runtime/securing-node-red),
[OliveTin keys](https://docs.olivetin.app/security/api_keys.html),
[ByteStash MCP](https://github.com/jordan-dalby/ByteStash).

## Velxio (Arduino simulator)

Velxio uses its built-in MCP server behind a generated bearer token. Install
creates the token, stores it in the Vault and fills in the connection details.
Enable **Read**, optionally **Execute**, then test the connection. The reserved
managed name is `aurago-store-velxio`.

The exact Read allowlist is `create_circuit`, `update_circuit`,
`import_wokwi_json`, `export_wokwi_json` and `generate_code_files`. Execute adds
`compile_project` and `run_project`. In read-only Virtual Desktop mode the
compile tools are withheld, even when Execute is saved.

The URL is always derived and any client-sent URL or credential is ignored.
Native AuraGo uses the published loopback port of the agent endpoint.
Containerized AuraGo uses the Docker network selected in this dialog, so select
it and run an explicit Store update. A remote Docker Engine is refused, because
the loopback port on that host cannot be reached. The agent port is never shown
as an openable port and never placed on Tailscale.

The connection test reads `tools/list` only and checks that every allowed tool
exists. It never compiles.

The MCP server can start later than the web UI, for example after a restart,
an update, an AuraGo start or a Docker host reboot. When the normal check after
such an operation fails, AuraGo retries in the background for up to five
minutes. It checks `tools/list` every 10 seconds without holding the Store
operation slot. Once the server answers, it runs the normal verified test and
activates the saved connection.

Tool results go to the agent. They do not open in an editor.

## Metrics, files and PDFs

Beszel uses a normal `users` account, never PocketBase superuser credentials.
Limit that account to the intended systems. `beszel` lists systems and reads the
latest one-minute host/container records, with timestamps, units and stale flags
(older than five minutes, missing or invalid time). Existing optional Store agent
setup remains the only agent provisioning path; no new polling or alerts are added.

FileBrowser Quantum uses a dedicated API user with exactly one selected source
backed by the Store `/folder` volume. Configure only View/Download source rights,
API access, no admin or sharing; save the user API token and exact source name.
AuraGo rechecks these rights on every operation and never mounts that volume in
its agent workspace. The administrator owns the source-to-volume mapping inside
FileBrowser. Small text (64 KiB) is inline; larger/binary reads use a unique Shared
path and require file-write permission. View tokens and upstream settings are not
returned to the agent. Incompatible source-permission APIs require a Store update.

Stirling PDF requires an API key and explicit processing permission for OCR,
splitting and merging. OCR checks installed language packs before upload; missing
languages are reported by name. Inputs are saved Desktop PDFs, up to 20 for merging.
Total input/output obey the Desktop file maximum, capped at 64 MiB; outputs are new
files in `Shared/Store/stirling-pdf/`. Originals are never replaced. Split ZIPs are
bounded and validated before publication. Existing signatures may become invalid
when a document is processed or merged; original copies remain unchanged.

Signing is independently disabled by default. Configure the server certificate
in Stirling, inspect its public DER certificate and enter its SHA-256 fingerprint
in the Store dialog. The API key must be allowed to read Stirling's administrator
public-certificate endpoint. AuraGo checks validity and fingerprint before and
after `certType=SERVER` signing; any observed change blocks/withholds the result.
It never uploads or reads private keys, creates certificates or changes keystores.
A server signature identifies its certificate, not a qualified personal signature.
Connection tests inspect API capabilities/languages/public certificate only.

Upstream: [Beszel REST API](https://beszel.dev/guide/rest-api),
[FileBrowser users](https://filebrowserquantum.com/en/docs/configuration/users/),
[Stirling API](https://docs.stirlingpdf.com/Developers/API/).

## Ziro exports

Connect Ziro and grant execution to enable `ziro_export`. No URL or app credential
is required. The capability test prepares the fixed KiCad 10.0.8 worker image and
checks its version/CLI without exporting. Docker write and Desktop publication
permissions remain mandatory. Unsaved browser edits are excluded.

Use a saved `.kicad_pro`, `.kicad_sch` (netlist) or `.kicad_pcb` (manufacturing)
in its own directory below Shared. Include relative libraries and hierarchical
sheets in that directory. The snapshot cap is 2048 entries/32 levels and the
configured publication size limit, capped at 64 MiB. Missing/escaping references
or incompatible formats fail with an explicit error; originals stay unchanged.

The worker has no network, no host project mount, a read-only root and only its
private job directory. One job runs at a time with a five-minute deadline. Revoke
permissions or stop the Store app to cancel it and kill its container. Native
Docker uses the local job path. Containerized AuraGo resolves its proven bind or
named-volume mount and exposes only the job subdirectory (named volumes require
Docker API 1.45). Remote engines without local snapshot access are refused.

Netlists use KiCad S-expression format. Manufacturing ZIPs contain Gerber-X2,
Excellon and a manifest with source/output SHA-256 hashes and KiCad version.
Results appear under a new `Shared/Store/ziro/<job>/` path. Export does not certify
electrical correctness or fabrication readiness.

The worker build uses the official signed KiCad release PPA, KiCad 10.0.8 package
checksums for amd64/arm64 and a pinned Ubuntu base. `docker-publish.yml` gates
publication on real netlist/manufacturing smokes on both architectures. Release
maintainers must pin the resulting verified image digest after these smokes;
the version tag is not a claim that this new image has already been published.
