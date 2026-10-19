# Isolated code execution

Foreground shell, Python, saved Python tools and supported skill scripts use
the existing llm-sandbox supervisor when unsafe host execution has not been
explicitly granted. Missing Docker/Podman, missing pinned Python dependencies,
or an unsupported language image is an execution error. There is no automatic
host fallback. The legacy Linux/macOS `allow_unsandboxed_shell` setting still
opts shell calls into host execution; Python requires
`agent.allow_unsafe_host_execution`.

The supervisor requires `llm-sandbox[mcp-docker]==0.3.45` (or `mcp-podman`),
including its MCP dependencies. An operator must provision this package when
automatic provisioning is disabled. The package version is checked before startup.
The trusted supervisor has its own Python environment at
`<data_dir>/sandbox-supervisor/venv`. It never adopts the agent's existing
`workdir/venv`, whose startup hooks and packages may already be agent-modified.
With `sandbox.auto_install: true`, trusted system Python creates this private
environment and installs the pinned package. Otherwise provision that exact
environment explicitly; installing into the agent venv does not make it ready.
Supervisor probes and launches use Python's isolated `-I` mode, so the current
directory, `PYTHONPATH` and user-site hooks cannot inject host code. Native tools,
archives and their rooted wrappers cannot read or modify the private runtime or
rename/delete its parent. These protections survive configuration replacement.
Docker socket proxies retain `BUILD=0`: these workers pull reviewed images and
never build a runtime image or commit an executed container.

## Image manifest

The Python and POSIX-shell default is the official multi-platform manifest:

```
python:3.13-slim-bookworm@sha256:a1165e272e578941b84abc79e4ab38a0305cd12803a5c4247979ac7655f4d641
```

The digest was resolved on 2026-10-18 from Docker Hub's
`library/python/manifests/3.13-slim-bookworm` manifest endpoint, with OCI and
Docker manifest-list media types. It is retained in source; runtime calls do
not resolve the mutable tag. Administrator-supplied `sandbox.image` and
per-language `sandbox.images` references must also contain a SHA-256 digest.
JavaScript and other languages require an operator-provisioned image with the
required runtime already installed. Libraries cannot be installed by a code
request; include them in a reviewed pinned image.

## Worker boundary

Every worker has a read-only image, fresh bounded `/sandbox` and `/tmp` tmpfs,
no host bind mounts, no Docker socket, and no inherited host environment.
Defaults are 512 MiB RAM, one CPU, 64 processes and 1 MiB captured output.
`no-new-privileges` is set. All capabilities are dropped, with only
`DAC_OVERRIDE` added because upstream's private injected files can retain the
supervisor's UID. This is a limited-capability root container, not a claim of
zero capabilities. A PID-1 wall-clock deadline and automatic removal also bound
workers if their supervisor disappears. Each run gets a new container.

Workers use `network_mode=none`, including when the legacy
`sandbox.network_enabled` flag is true. A general Docker bridge would expose
host and control-network services. Future network access needs a separate
egress policy. Generic workers receive no Vault secrets or credentials.

## Tool Bridge

An explicitly enabled foreground Python Tool Bridge uses an expiring,
run-bound grant issued by the server. A worker's `aurago.call_tool` SDK sends
bounded framed requests over captured stdout. The trusted supervisor calls
the local bridge with that grant and publishes a private, fixed-name response
file inside the worker. The worker receives neither the grant token nor an
HTTP connection to AuraGo. The server rechecks current permissions and the
captured originating run's narrower policy for each call.

Code and broker response files are bounded and published through Docker Exec
stdin into fixed, validated `/sandbox` filenames. Publication requires a
confirmed successful exit; it does not use the Engine archive-write API or
make the worker root filesystem writable.

Frames are limited to 64 KiB, responses to 1 MiB and grants to at most 32 calls.
The supervisor accepts only a loopback bridge origin and refuses redirects.
Cancelling a foreground caller cancels its MCP request and retires that
connection. A required supervisor remains fail-closed.

Daemon skills use the existing stdout/stdin IPC and one SDK reader for tool
results, Stop and configuration commands. The trusted Go supervisor owns the
bridge grant; daemon workers receive neither an authentication token nor a
bridge URL. Calls retain the original `daemon:<skill>:<generation>` session and
are cancelled with that generation or server shutdown. Tool access intersects
the manifest, startup configuration and current configuration; SQL connections
must satisfy both startup and current allowlists. Each grant permits at most
32 calls in five minutes; exhaustion cannot trigger an early refill. Actions
requiring authorization need matching background action rules. Existing daemon
execution gates still apply: this IPC does not grant unsafe host execution or
provide a sandbox for long-running daemons.

Background execution uses the existing durable Background Task Manager and
returns a task ID. System Tasks shows bounded output, status and cancellation;
completion notifications are enabled. The queued job captures the originating
shell/Python permissions, canonical call and role/scope. Before the supervisor
starts, it intersects those permissions with current gates and rechecks action
rules. A revoked or legacy unidentified action becomes blocked without an
approval request or execution identity.
Cancellation and server shutdown reach its supervisor call. An interrupted
running job becomes a failed uncertain outcome after restart and is never
automatically replayed. Opted-in host background execution retains the existing
managed process/PID path.

## Validation

`python internal/tools/sandbox_launcher_security_test.py` exercises the fixed
container specification, split broker frames, denied tools, origin checks,
protocol limits and output termination without Docker. Focused Go tests cover
default routing, cancellation, rooted publication and lint argument safety.
Real Linux Docker checks cover the pinned Python image, actual worker limits
and isolation, zero and nonzero exits, stdout/stderr, output limits, cancellation
during execution and a broker round trip against a synthetic local service.
These exercise the launcher, installed SDK and Engine. Podman and full Go/MCP
transport with server-issued grants remain separate acceptance checks.
