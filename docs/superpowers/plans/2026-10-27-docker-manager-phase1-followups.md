# Docker Manager phase 1 — review follow-ups

Collected during the phase 1 execution (2026-10-27). Important findings were fixed on the branch; these are the decisions and the minor items left for phase 2 or later.


## Final review (4 Important): fixing 1 remote remove self/endpoint protection, 2 TestNow keeps failure count, 3 draft-test 400 invalid_host; REJECTED 4 mTLS key after address change (server cert verified vs stored CA/system roots before client cert is sent; CertificateVerify bound to transcript → no relay). Also fixing now: stacks_dir/socket path charset + name control chars, 409 dialog copy.
Decisions kept: host CRUD allowed under docker.readonly (plan Part C deviation 1, spec line 183 says all mutating routes 403 → spec to be aligned); draft tests unguarded (each click = one deliberate attempt); UI rejects port 0 (server treats as default).

## Review A5–A9 (3 Important → fix agent in .worktrees/docker-manager-fix: SSH auth backoff + dial gate + TestNow, identity-change secret reuse, cache generation; plus TLSMaterial String, Close flag, write mutex, Create rollback error)
Minors left for the final review:
- registry.go:231-258/312-329: SecretSet/Test can write a stale flag/status after a concurrent Update (cosmetic).
- internal/remote/remote.go:74-82: passphrase-protected key fails ParsePrivateKey and is sent as a password (pre-existing).
- host.go:162-174: stacks_dir / host_socket_path accept ':', ',', '=', quotes, spaces — restrict before phase 2 builds bind strings.
- host.go ValidateHost name: no printable-character check.
- tools/docker_inspect_export.go:19: "state" passes full State incl. Health.Log unredacted (pre-existing projection).
- health.go:101 "open /" matches before the DNS check; health.go:165 empty ApiVersion → api_too_old with misleading text.

## Review part C (XSS clean; 3 Important → UI fix agent on docker-manager: form cleared on close, SSH identity rule mirrored, docker_readonly on GET /hosts)
Minors left:
- store.js:181 pollOnce returns early while disabled → recovers from docker_disabled only via Refresh/visibility change.
- model.js:119-123/168 portItems/haystack deref null entries; paint() has no guard.
- dialogs.js:243-249 showProblem focuses fields hidden in a closed <details>.
- dialogs.js:69-71 backdrop click closes the whole stack, discards unsaved input.
- views.js:279 data-tab-body/aria-labelledby interpolate detail.tab unescaped (constant today).
- docker-manager.js:412 Delete handled with focus on a row button (opens confirm; harmless).
- docker-manager.js:85-98 toasts raised while a dialog is open stay hidden after it closes.
- store.js:249-252 failed mutation does not reload the list (waits for 5 s poll).
- Inspect Env: server projection redacts env (redactDockerInspectEnv) — OK.

## Review A1–A4 (no Critical/Important)
1. internal/dockerutil/client.go:63 — error string starts with a capital ("Docker client ...").
2. internal/tools/docker_inspect_export.go:64-70 — RedactDockerLabels doc says "returns a new map" but returns the argument for nil/empty input.
3. internal/dockerhost/transport/tls.go:22-26 — TLSMaterial has no redacting String/GoString; %v of SecretInput (host.go:67-71) would print the PEM key. Cheap fix: redacting String() + GoString().
4. internal/invasion/connector_docker.go:471 — docker_ssh passes nest.Host unstripped; bracketed IPv6 becomes "[[::1]]:22" (pre-existing; Docker Manager uses bareAddress()).
5. internal/dockerutil/client.go:55-75 — no MaxConnsPerHost: concurrent requests open parallel SSH logins; idle SSH clients live up to 1 min after a client is dropped without CloseIdleConnections. Check the registry (A8) closes idle conns on host update/delete and consider a per-host conn cap.
