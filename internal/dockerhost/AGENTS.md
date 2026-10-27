# Docker Manager hosts

## Purpose and ownership

`internal/dockerhost` owns the Docker Manager's host registry (the local Engine and remote Engines over plain TCP, TLS, mutual TLS or SSH), their vault secrets, one Engine client per host, connection health, and the Engine reads and container actions that `/api/docker-manager` serves. `transport/` owns the TLS client configuration and the SSH Engine dialer that Invasion shares. The HTTP API lives in `internal/server`, the desktop app in `ui/js/desktop/apps/docker-manager*.js`; their contracts are in the design `docs/superpowers/specs/2026-10-26-docker-manager-design.md`.

## Local contracts

- The built-in host `local` is synthesized from `docker.host` (the OS default when empty), is never stored and cannot be changed or deleted (`ErrLocalHostFixed`). Remote hosts live in table `docker_hosts` of `<data dir>/docker_manager.db` with ids `h_` plus 12 hex characters. Names are unique ignoring case and never "Local". `ValidateHost` judges a `Host.Normalized()` host.
- Host secrets live only in the vault: `docker_host_ssh_<id>` (SSH private key or password) and `docker_host_tls_<id>` (JSON `ca`/`cert`/`key`). They never reach the database, an API answer or a log, and the prefix `docker_host_` stays in `tools.blockedSecretPrefixes`. `SecretInput` is write-only: nil or blank fields keep what is stored, `Clear` empties both first, secrets the kind does not use are removed. A host whose secrets cannot be stored is not created; a failed update puts the vault back.
- Verification is never disabled. `tls`/`mtls` use TLS 1.2 or newer, the stored CA or the system roots and `ServerName` = the host address; unusable material fails every request and never falls back to plain HTTP. `tcp` is the only unencrypted kind.
- `Registry.Client` caches one `dockerutil.Client` per host, keyed by a fingerprint of the connection settings; `Update` and `Delete` drop it. Clients dial nothing until the first request. SSH hosts pool connections (one SSH client per Engine connection, closed with it); `Registry.Close` releases them.
- `Registry.Test` (GET /version, then /info) caches a `HostStatus`; `ClassifyError` maps failures to `dns`, `refused`, `timeout`, `tls`, `auth`, `ssh_host_key`, `socket_missing`, `api_too_old` or `unknown`. Engines below API 1.41 are refused (`api_too_old`).
- Engine reads and actions return a `*StatusError` for a non-2xx answer; classify by `StatusOf(err)`, never by message text. Container ids are validated (`ErrInvalidContainerRef`) before they enter a path, actions are start, stop, restart, pause, unpause and kill (`ErrInvalidAction`), HTTP 304 counts as success, removal sends `v=true`.
- Redaction reuses the admin inspect: `InspectContainer` goes through `tools.ProjectDockerInspect`, list labels through `tools.RedactDockerLabels`, bind sources through `tools.RedactDockerMountSource` (last path element only). `dockerhost` imports `internal/tools` for this; `internal/tools` must never import `dockerhost`.
- `transport.TLSClientConfig`, `transport.TLSMaterial` and `transport.DialEngineOverSSH` are the former Invasion helpers. Their error texts and tunnel semantics (known_hosts through `remote.DialSSH`, bounded socket open, closing the conn closes the SSH client) are visible in API answers and pinned by the transport tests and Invasion's Docker tests.

## Work guidance

- Add Engine calls through `engineJSON` (`dockerutil.Client.DoJSON`); stream bodies are read with `Client.HTTPClient()` and bounded.
- Tests use the fake Engine in `fakeengine_test.go` (it serves `/version` and the `/v1.45` routes of the fixtures) and the in-memory vault in `registry_test.go`; no test needs Docker.

## Verification

```
go test ./internal/dockerhost/... ./internal/dockerutil/... -count=1
go test ./internal/invasion/ -run 'Docker|TLS' -count=1
go test ./internal/tools/ -run 'TestDockerInspect|TestProjectDockerInspect|TestRedactDocker|TestIsPythonAccessibleSecret' -count=1
```

## Child DOX Index

None.
