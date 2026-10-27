# Docker Manager phase 1 (foundation) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The foundation of the Docker Manager desktop app: a registry of local and remote Docker hosts (TCP, TLS/mTLS, SSH) with a shared transport, a `/api/docker-manager` API for hosts, containers, images, volumes and networks, and a desktop app with a host switcher, a host manager and a polished Containers view.

**Architecture:** A new `internal/dockerhost` package owns host records (SQLite `docker_manager.db`), vault secrets, one `dockerutil.Client` per host built on a per-kind dialer, health classification and the Engine reads and actions. The TLS and SSH transport code moves from `internal/invasion` into `internal/dockerhost/transport`, and Invasion delegates to it unchanged. `internal/server` exposes the API with the same auth, readonly and protection rules as `/api/containers`. The desktop app `docker-manager` follows the Infra and RuView module split (model, store, views, dialogs, composition root).

**Tech Stack:** Go (raw `net/http` to the Engine API through `dockerutil.VersionTransport`, `modernc` SQLite through `internal/dbutil`, `golang.org/x/crypto/ssh` through `internal/remote`), vanilla JS desktop modules, design tokens in CSS, Node VM tests, a go-rod browser smoke.

**Spec:** `docs/superpowers/specs/2026-10-26-docker-manager-design.md` (phase 1 section). Out of scope here: stacks, the runner, events, stats, the terminal, cleanup, updates, the agent tool (phases 2–5).

---

## How to execute

- **Worktree:** `.worktrees/docker-manager` on branch `docker-manager` from local `main`. Other sessions commit to `main` in the main checkout; land this branch by rebase plus `git merge --ff-only`, or with the app's `sync_with_base_branch`.
- **Order:** Part A (Tasks A*) → Part B (Tasks B*) → Part C (Tasks C*). C's registration, model and CSS tasks do not need A or B and may run in parallel with them. Its store, views and browser smoke code against the API contract and use a fake API, so they do not need a running backend either.
- **GitNexus:** before editing an existing function, run the impact step that each task lists. GitNexus may not resolve Go functions in this repo (`reference_gitnexus_index_gaps`); then the listed Grep is the caller check. Run `detect-changes` before each commit.
- **Go cache:** `GOCACHE` and `GOTMPDIR` live on `D:`. If a build fails with "not enough space on the disk", trim `D:\dev-cache\go-build` entries older than 3 days (fall back to 1 day) and clear `D:\tmp\go-build` before retrying. Keep `GOTMPDIR` short and do not point it at the scratchpad.
- **Lean testing (user preference):** the tests in this plan plus one test per bug fixed on the way. One combined review per batch of tasks; Critical and Important findings only.
- **Preserved behaviour:** `/containers`, `/api/containers`, the SSE `container_update` event, the `docker` agent tool and Invasion's nest deployment must behave exactly as before. The existing Invasion tests must pass without edits.
- **How the drafts were checked:** each part was checked against the real tree without changing it. Part A and part B were compiled and tested task by task with `go test -overlay`, part B against part A's code. Part C was applied to a scratch clone of `main` and run there: the Node test, the Go guards, `npm run check:ui` and the browser smoke. Line references are from that state of `main`. If a step's `old_string` no longer matches because `main` moved, re-read the file and apply the same change.

## Task overview

| Task | Result |
| --- | --- |
| A1 | `dockerhost/transport` TLS config; Invasion delegates (type alias plus wrapper) |
| A2 | `dockerhost/transport` SSH Engine dialer; Invasion delegates |
| A3 | `dockerutil.NewClientWithDialer` |
| A4 | `tools` exports the admin inspect redaction |
| A5 | `dockerhost` host model, validation, SQLite store |
| A6 | `dockerhost` Engine reads and actions, plus the shared fake Engine |
| A7 | `dockerhost` health: `HostStatus`, `ClassifyError`, probe |
| A8 | `dockerhost` registry, vault secrets, per-kind dialers; `docker_host_` blocked for Python tools |
| A9 | DOX closeout and regression sweep for part A |
| B1 | Server wiring: registry opened at startup, closed at shutdown |
| B2 | `/api/docker-manager/` router, admin gate, audit manifest entry |
| B3 | Host API: list, create, update, delete, connection test |
| B4 | Docker nests: list and import as hosts |
| B5 | Containers: list with protection, actions, inspect, logs, guarded remove |
| B6 | Overview, images, volumes, networks; part B check |
| C1 | App registration and the `docker` capability |
| C2 | App icons in every icon theme |
| C3 | Strings in all 16 locales |
| C4 | Model and store with the Node test |
| C5 | Views and stylesheet |
| C6 | Dialogs: host manager, host form with connection test, nest import, confirmations |
| C7 | Composition root and the remaining Node guards |
| C8 | Shell wiring, bundle, browser smoke |
| C9 | App documentation and the final sweep |

## Deviations from the spec (decided while planning)

- **API floor:** The spec named Engine API 1.41 as the floor. Phase 1 keeps `dockerutil.VersionTransport`'s existing floor (1.25) for every host, so behaviour matches the rest of AuraGo. Plan 2 decides whether the runner needs a higher floor.
- **Capability:** The `docker` capability is answered in `desktopCapabilities.HasCapability` (`internal/server/bluetooth_live.go`), because `internal/desktop/capabilities.go` only holds the generic filter (Task C1).
- **Untranslated locales:** No native speaker has reviewed the Greek, Hindi, Japanese and Chinese strings. They are real translations, but need a review before a release.

## Known failures that are not caused by this plan

- `internal/server` `TestGetDesktopServiceReusesCanonicalConfigWithRelativeDefaults` fails when `GOTMPDIR` is on `D:` and the checkout on `C:`. The cause is the environment.
- `ui` `TestDashboardCardsFollowSectionPlacementAndAnatomy` and `TestFrontend_StaticI18nKeysExistInEnglishBundle` (missing `help.chromecast.*` keys) already fail on `main`.
- Two full-package `internal/audit` tests fail in the main checkout because of a stray `.tmp_aurago/baseline-check` folder there. The plan runs `-run Manifest`.
- `TestDesktopInfraAppBrowser` fails on `main`; skip it in browser sweeps.

---

# Part A: hosts, transport and Engine access (`internal/dockerhost`)

## Part A notes

Part A is the Go foundation: `internal/dockerhost/transport`, `dockerutil.NewClientWithDialer` and `internal/dockerhost` (hosts, store, registry, health, Engine reads). Every name and shape of the contract is kept. What differs or is added, and why:

1. **`internal/tools` exports the inspect redaction (Task A4).** The admin inspect redaction was reachable only through `DockerInspectContainer(cfg DockerConfig, ...)`, which talks to `docker.host`. A4 moves its projection code unchanged into `dockerInspectProjection` and adds three small exported wrappers in the new file `internal/tools/docker_inspect_export.go`: `ProjectDockerInspect`, `RedactDockerLabels`, `RedactDockerMountSource`. `DockerInspectContainer` output stays byte-identical (a test compares both). Consequence: `internal/dockerhost` imports `internal/tools`, and `internal/tools` must never import `internal/dockerhost`.
2. **Vault prefix `docker_host_` is added to `tools.blockedSecretPrefixes` (Task A8)**, like `nest_`, so Python tools can never be handed a host's SSH key or TLS material (they are already not agent-readable; this is the defence in depth every system-managed prefix has).
3. **Exported names beyond the contract** (all additive): `Host.Normalized()`, `StatusError` and `StatusOf`, `ErrInvalidAction`, `ErrInvalidContainerRef`, the status constants `StateOK`/`StateError`/`StateUnknown`, the code constants `CodeDNS`, `CodeRefused`, `CodeTimeout`, `CodeTLS`, `CodeAuth`, `CodeSSHHostKey`, `CodeSocketMissing`, `CodeAPITooOld`, `CodeUnknown`, and `transport.DefaultEngineSocket` / `transport.DefaultSocketOpenTimeout`.
4. **Semantics the contract left open:**
   - `SecretInput`: nil fields keep the stored value, and so do a blank `SSHSecret` and empty fields of `TLS` (merged field by field), so a form that leaves its secret inputs untouched changes nothing. `Clear` empties both secrets first. Secrets that the kind does not use are removed (tls to tcp deletes the TLS entry). For kind `tls` a client certificate and key are dropped, because server-only TLS never presents one.
   - `ssh` needs a secret (`ValidationError{Field:"ssh_secret"}`), `tls`/`mtls` need usable PEM material (`ValidationError{Field:"tls"}`); both are checked before anything is written. A host whose secrets cannot be written is not created; a failed update puts the vault back.
   - Host names are unique ignoring case and may not be "Local" (`ValidationError{Field:"name"}`). An empty color on create takes the next entry of `Colors`.
   - `TestDraft(h, sec)`: when `h.ID` names a stored host, secrets that `sec` leaves out come from the vault (the edit dialog's "Test connection" works without retyping secrets). Invalid drafts return a `HostStatus` with `State:"error"`, not an error.
   - `Test` and `TestDraft` are bounded by 30 s each. `Test` of an unknown id returns an error status and caches nothing.
   - `HostStatus.CheckedAt` is a `time.Time`: the zero time (`0001-01-01T00:00:00Z`) means "never tested" (`omitempty` has no effect on structs).
   - The local host's client uses `NewClientWithDialer` with `dockerutil.DialContext`, like the other kinds (`Host()` returns the endpoint).
5. **Data shown by the list calls.** `ContainerSummary.Health` is read from the status text ("(healthy)", "(unhealthy)", "(health: starting)"), because the list API has no health field. `Mount.Source` of a bind keeps only its last path element and credential-like label values are masked, both with the admin inspect's rules (a volume's source stays complete). `compose_working_dir` is the full label value.
6. **Vault contract.** `SecretVault.ReadSecret` must return an error matching `security.ErrSecretNotFound` for a missing key; `*security.Vault` does. `*security.Vault` satisfies `SecretVault` as it is.
7. **DOX.** Task A9 adds `internal/dockerhost/AGENTS.md` and updates the root index and the dockerutil, invasion and tools guides.

### Hand-off to parts B and C

- Mapping for the HTTP layer: `*ValidationError` (use `Field`, `Message`; fields are `name`, `kind`, `address`, `port`, `stacks_dir`, `host_socket_path`, `color`, `ssh_user`, `ssh_secret`, `tls`) gives `invalid_host`; `ErrHostNotFound` gives `host_not_found`; `ErrLocalHostFixed` gives `local_host_fixed`; `ErrInvalidAction` gives `invalid_action`; `ErrInvalidContainerRef` is a 400; `dockerhost.StatusOf(err) == 404` on a container call gives `container_not_found`; any other Engine answer is `docker_error`; any other error from `Registry.Client` or an Engine call with `StatusOf(err) == 0` (nothing answered) gives `host_unreachable` with `reason = dockerhost.ClassifyError(err)`.
- `Registry.Test` blocks up to 30 s on an unreachable host. For `GET /hosts` call `Status(id)` (cached) and refresh with `Test` in goroutines, with `CheckedAt` as the cache age.
- HostView: `Registry.SecretSet(id)`, `Registry.Status(id)`, `builtin = (id == LocalHostID)`, `plaintext = (kind == KindTCP)`.
- Pass `Host.ID` to `TestDraft` when the dialog edits a stored host.
- Annotate the lists with `dockerhost.Annotate(containers, images, volumes, networks)` after fetching all four; pass the container list you already have (not nil) so the counts are real.
- The container list returns `[]` (never null) for names, ports, mounts, networks; labels are omitted when empty.
- `InspectContainer` returns the trimmed admin-inspect shape (`id`, `name`, `state`, `mounts`, `config`, `network`, `network_mode`, ...) without a `status` key.

## Part A task overview

| Task | Result |
| --- | --- |
| A1 | `dockerhost/transport` TLS config; Invasion delegates (alias plus wrapper) |
| A2 | `dockerhost/transport` SSH Engine dialer; Invasion delegates |
| A3 | `dockerutil.NewClientWithDialer` |
| A4 | `tools` exports the inspect redaction |
| A5 | `dockerhost` host model, validation and SQLite store |
| A6 | `dockerhost` Engine reads and actions, plus the shared fake Engine |
| A7 | `dockerhost` health: `HostStatus`, `ClassifyError`, probe |
| A8 | `dockerhost` registry, vault secrets, per-kind dialers; Python secret blocklist |
| A9 | DOX closeout and regression sweep |

Each task compiles and passes on top of the previous ones.

---

### Task A1: Transport package, TLS configuration, Invasion delegates

**Files:**
- Create: `internal/dockerhost/transport/tls.go`
- Test: `internal/dockerhost/transport/tls_test.go`
- Modify: `internal/invasion/docker_tls.go` (whole file)

The TLS builder moves out of Invasion with its error texts unchanged. Invasion keeps `DockerTLSMaterial` as a type alias, keeps its constants and keeps `dockerTLSClientConfig` as a one-line wrapper, so `internal/invasion/*_test.go` and the server's nest handlers compile and behave as before.

Source references (HEAD): `internal/invasion/docker_tls.go:18-24` (`DockerTLSMaterial`), `:37-44` (`ValidateDockerTLS`), `:46-83` (`dockerTLSClientConfig`, moved); callers `internal/invasion/connector_docker.go:410-430` (`dockerRemoteTLSTransport`) and the tests in `internal/invasion/connector_docker_tls_test.go`.

- [ ] **Step 1: Impact check before editing `dockerTLSClientConfig` and `DockerTLSMaterial`**

```powershell
npx gitnexus impact "dockerTLSClientConfig" --direction upstream --repo AuraGo
npx gitnexus impact "DockerTLSMaterial" --direction upstream --repo AuraGo
```

If GitNexus does not resolve the Go symbols, run the caller check instead:

```powershell
Select-String -Path internal\invasion\*.go,internal\server\*.go -Pattern 'dockerTLSClientConfig|DockerTLSMaterial' -List | Select-Object Path
```

Expected callers: `ValidateDockerTLS` and `dockerRemoteTLSTransport` (`internal/invasion`), the nest handlers and their tests in `internal/server` (they use `invasion.DockerTLSMaterial`). Report them. Risk: LOW, because no signature or behaviour changes and the alias keeps every use compiling.

- [ ] **Step 2: Write the failing transport test**

Create `internal/dockerhost/transport/tls_test.go`:

```go
package transport

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

// selfSignedPEM returns one self-signed certificate (usable as CA and as client
// certificate) with its PKCS#8 key, both PEM encoded.
func selfSignedPEM(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "AuraGo transport test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
}

// The error texts are the ones Invasion's nest validation showed before the
// code moved here; they are part of the API answers of the nest handlers.
func TestTLSClientConfigErrorTextsAreUnchanged(t *testing.T) {
	certPEM, keyPEM := selfSignedPEM(t)
	cases := []struct {
		name     string
		mode     string
		material TLSMaterial
		want     string // exact error text, or prefix when ending in "..."
	}{
		{"off", TLSOff, TLSMaterial{}, "docker_tls is off; no TLS configuration applies"},
		{"unknown mode", "starttls", TLSMaterial{}, `invalid docker_tls "starttls" (must be empty, tls or mtls)`},
		{"garbage CA", TLSServer, TLSMaterial{CA: "not a certificate"}, "docker_tls_ca contains no PEM certificate"},
		{"tls rejects client certificate", TLSServer, TLSMaterial{Cert: certPEM, Key: keyPEM}, "a client certificate requires docker_tls mtls"},
		{"mtls needs key", TLSMutual, TLSMaterial{Cert: certPEM}, "docker_tls mtls requires docker_tls_cert and docker_tls_key"},
		{"mtls needs cert", TLSMutual, TLSMaterial{Key: keyPEM}, "docker_tls mtls requires docker_tls_cert and docker_tls_key"},
		{"mtls bad pair", TLSMutual, TLSMaterial{Cert: "x", Key: "y"}, "docker_tls_cert/docker_tls_key: ..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := TLSClientConfig(tc.mode, tc.material)
			if err == nil || cfg != nil {
				t.Fatalf("TLSClientConfig = (%v, %v), want an error", cfg, err)
			}
			if prefix, ok := strings.CutSuffix(tc.want, "..."); ok {
				if !strings.HasPrefix(err.Error(), prefix) {
					t.Fatalf("error = %q, want prefix %q", err.Error(), prefix)
				}
				return
			}
			if err.Error() != tc.want {
				t.Fatalf("error = %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

func TestTLSClientConfigBuildsVerifyingConfigs(t *testing.T) {
	certPEM, keyPEM := selfSignedPEM(t)
	cases := []struct {
		name      string
		mode      string
		material  TLSMaterial
		wantRoots bool
		wantPairs int
	}{
		{"tls with system roots", TLSServer, TLSMaterial{}, false, 0},
		{"tls with CA", " tls ", TLSMaterial{CA: certPEM}, true, 0},
		{"mtls with pair", TLSMutual, TLSMaterial{CA: certPEM, Cert: certPEM, Key: keyPEM}, true, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := TLSClientConfig(tc.mode, tc.material)
			if err != nil {
				t.Fatalf("TLSClientConfig: %v", err)
			}
			if cfg.MinVersion != tls.VersionTLS12 || cfg.InsecureSkipVerify {
				t.Fatalf("config = TLS min %x, insecure %v; want TLS 1.2+ with verification", cfg.MinVersion, cfg.InsecureSkipVerify)
			}
			if (cfg.RootCAs != nil) != tc.wantRoots || len(cfg.Certificates) != tc.wantPairs {
				t.Fatalf("roots set = %v, client certificates = %d; want %v and %d", cfg.RootCAs != nil, len(cfg.Certificates), tc.wantRoots, tc.wantPairs)
			}
		})
	}
}
```

- [ ] **Step 3: Run it (expected FAIL)**

```powershell
go test ./internal/dockerhost/transport/ -count=1
```

Expected: `FAIL aurago/internal/dockerhost/transport [build failed]` with `undefined: TLSClientConfig` (and `TLSMaterial`).

- [ ] **Step 4: Create the transport package with the TLS builder**

Create `internal/dockerhost/transport/tls.go`:

```go
// Package transport holds the Docker Engine connection building blocks shared
// by Invasion and the Docker Manager: the TLS client configuration for HTTPS
// Engines and the dialer that reaches an Engine socket through SSH.
package transport

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"
)

// Docker TLS modes. The empty mode is the plain-HTTP transport.
const (
	TLSOff    = ""
	TLSServer = "tls"
	TLSMutual = "mtls"
)

// TLSMaterial holds the PEM blocks of an encrypted Docker Engine connection.
// It is stored as JSON in the vault and never in a database.
type TLSMaterial struct {
	CA   string `json:"ca,omitempty"`
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

// TLSClientConfig builds the client TLS configuration of an encrypted Engine
// connection: TLS 1.2 or newer, the stored CA (system roots when none is
// stored) and, for mtls, the client certificate. Verification is never
// disabled. The error texts are shared with Invasion's nest validation.
func TLSClientConfig(mode string, material TLSMaterial) (*tls.Config, error) {
	mode = strings.TrimSpace(mode)
	switch mode {
	case TLSServer, TLSMutual:
	case TLSOff:
		return nil, fmt.Errorf("docker_tls is off; no TLS configuration applies")
	default:
		return nil, fmt.Errorf("invalid docker_tls %q (must be empty, tls or mtls)", mode)
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if ca := strings.TrimSpace(material.CA); ca != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(ca)) {
			return nil, fmt.Errorf("docker_tls_ca contains no PEM certificate")
		}
		cfg.RootCAs = pool
	}
	cert, key := strings.TrimSpace(material.Cert), strings.TrimSpace(material.Key)
	switch mode {
	case TLSMutual:
		if cert == "" || key == "" {
			return nil, fmt.Errorf("docker_tls mtls requires docker_tls_cert and docker_tls_key")
		}
		pair, err := tls.X509KeyPair([]byte(cert), []byte(key))
		if err != nil {
			return nil, fmt.Errorf("docker_tls_cert/docker_tls_key: %w", err)
		}
		cfg.Certificates = []tls.Certificate{pair}
	case TLSServer:
		if cert != "" || key != "" {
			return nil, fmt.Errorf("a client certificate requires docker_tls mtls")
		}
	}
	return cfg, nil
}
```

- [ ] **Step 5: Run it (expected PASS)**

```powershell
go test ./internal/dockerhost/transport/ -count=1
```

Expected: `ok aurago/internal/dockerhost/transport`.

- [ ] **Step 6: Make Invasion delegate**

Replace the whole content of `internal/invasion/docker_tls.go` with:

```go
package invasion

import (
	"crypto/tls"
	"strings"

	"aurago/internal/dockerhost/transport"
)

// Docker TLS modes for docker_remote nests. The empty mode is the original
// plain-HTTP transport and stays the default for every existing nest.
const (
	DockerTLSOff    = transport.TLSOff
	DockerTLSServer = transport.TLSServer
	DockerTLSMutual = transport.TLSMutual
)

// DockerTLSMaterial holds the PEM blocks of an encrypted docker_remote nest.
// It lives in the vault under DockerTLSVaultKey, never in the invasion DB.
// The type is shared with the Docker Manager's host registry.
type DockerTLSMaterial = transport.TLSMaterial

// DockerTLSVaultKey names the vault entry that holds a nest's Docker TLS PEMs.
// The nest_ prefix keeps it out of Python tool secret access.
func DockerTLSVaultKey(nestID string) string {
	return "nest_docker_tls_" + nestID
}

// DockerRemoteUsesTLS reports whether a nest talks to its Docker Engine over HTTPS.
func DockerRemoteUsesTLS(nest NestRecord) bool {
	return nest.DeployMethod == "docker_remote" && strings.TrimSpace(nest.DockerTLS) != DockerTLSOff
}

// ValidateDockerTLS checks a mode and its PEM material without contacting anything.
func ValidateDockerTLS(mode string, material DockerTLSMaterial) error {
	if strings.TrimSpace(mode) == DockerTLSOff {
		return nil
	}
	_, err := dockerTLSClientConfig(mode, material)
	return err
}

// dockerTLSClientConfig builds the client TLS configuration of a docker_remote
// nest: TLS 1.2 or newer, the stored CA (system roots when none is stored)
// and, for mtls, the client certificate. Verification is never disabled. The
// work is done by transport.TLSClientConfig, which the Docker Manager shares.
func dockerTLSClientConfig(mode string, material DockerTLSMaterial) (*tls.Config, error) {
	return transport.TLSClientConfig(mode, material)
}
```

- [ ] **Step 7: Run Invasion's own TLS tests unchanged and compile the server tests**

```powershell
go test ./internal/invasion/ -run 'TLS' -count=1
go vet ./internal/invasion/ ./internal/server/
go test ./internal/server/ -run 'DockerTLS' -count=1
git diff --stat -- 'internal/invasion/*_test.go'
```

Expected: `ok aurago/internal/invasion`; `go vet` prints nothing and the server's nest tests (they use `invasion.DockerTLSMaterial`) still compile and pass (`ok aurago/internal/server`; compiling that test binary takes about a minute); the last command prints nothing (no Invasion test file changed).

- [ ] **Step 8: Graph check and commit**

```powershell
git add internal/dockerhost/transport/tls.go internal/dockerhost/transport/tls_test.go internal/invasion/docker_tls.go
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "refactor(docker-manager): move the Docker TLS client config into dockerhost/transport" -m "Invasion keeps DockerTLSMaterial as an alias and dockerTLSClientConfig as a wrapper; behaviour and error texts are unchanged." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A2: Transport SSH Engine dialer, Invasion delegates

**Files:**
- Create: `internal/dockerhost/transport/ssh.go`
- Test: `internal/dockerhost/transport/ssh_test.go`
- Modify: `internal/invasion/connector_docker.go` (imports, `dockerSSHProbeTimeout`, `dialDockerEngineOverSSH`, removal of `sshTunnelConn`)

`dialDockerEngineOverSSH` and `sshTunnelConn` move to `transport.DialEngineOverSSH`. Invasion keeps `dialDockerEngineOverSSH` as a wrapper that passes its own `dockerSSHSocketOpenTimeout` variable, so the Invasion tests that shorten that variable keep working. `dockerSSHProbeTimeout` becomes an alias of `transport.SSHProbeTimeout` (same 20 s; Invasion's budget test still reads the name). Invasion's tests fake sshd with an in-test SSH server; the transport test below uses a trimmed copy of that approach (password `fixture`, `remote.InsecureHostKey` switched on for the test, direct-streamlocal channels forwarded to an `httptest` backend).

Source references (HEAD): `internal/invasion/connector_docker.go:442-463` (budgets and the `dockerSSHSocketOpenTimeout` variable, which stay), `:465-485` (`dialDockerEngineOverSSH`, moved) and `:487-498` (`sshTunnelConn`, moved); the call site is `connector_docker.go:389` inside `httpClient`; `remote.DialSSH` is `internal/remote/dial_ssh.go:24`; the Invasion tests that must stay green are `internal/invasion/connector_docker_ssh_test.go` (the stalled-open test at `:326-349` shortens `dockerSSHSocketOpenTimeout`).

- [ ] **Step 1: Impact check before editing**

```powershell
npx gitnexus impact "dialDockerEngineOverSSH" --direction upstream --repo AuraGo
npx gitnexus impact "dockerSSHSocketOpenTimeout" --direction upstream --repo AuraGo
```

Fallback if the symbols do not resolve:

```powershell
Select-String -Path internal\invasion\*.go -Pattern 'dialDockerEngineOverSSH|sshTunnelConn|dockerSSHSocketOpenTimeout|dockerSSHProbeTimeout' | Select-Object Path,LineNumber,Line
```

Expected: `DockerConnector.httpClient` (`connector_docker.go`) calls `dialDockerEngineOverSSH`; the tests in `connector_docker_ssh_test.go` call it and set `dockerSSHSocketOpenTimeout`; `sshTunnelConn` is used only by the dialer. Risk LOW to MEDIUM (the Docker deploy path for `docker_ssh` nests); behaviour is unchanged and pinned by the existing SSH tests.

- [ ] **Step 2: Write the failing transport test**

Create `internal/dockerhost/transport/ssh_test.go`:

```go
package transport

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/remote"
	"aurago/internal/testutil"

	"golang.org/x/crypto/ssh"
)

type streamLocalOpenPayload struct {
	SocketPath string
	Reserved0  string
	Reserved1  uint32
}

// sshFixture is an SSH server (user "fixture", password "fixture") that
// forwards every direct-streamlocal channel to a backend TCP address, standing
// in for sshd plus the Docker socket. open counts the authenticated SSH
// connections that are still open, accepted all of them, sockets receives the
// requested socket paths. With holdChannels the server never answers a channel
// open, like an sshd that stalls on the socket.
type sshFixture struct {
	host         string
	port         int
	holdChannels bool
	open         atomic.Int64
	accepted     atomic.Int64
	sockets      chan string
}

func startSSHFixture(t *testing.T, backend string, holdChannels bool) *sshFixture {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if meta.User() == "fixture" && string(password) == "fixture" {
				return nil, nil
			}
			return nil, fmt.Errorf("access denied")
		},
	}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("IPv4 loopback listener unavailable in this test environment: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	fixture := &sshFixture{holdChannels: holdChannels, sockets: make(chan string, 16)}
	addr := listener.Addr().(*net.TCPAddr)
	fixture.host, fixture.port = addr.IP.String(), addr.Port
	go func() {
		for {
			raw, err := listener.Accept()
			if err != nil {
				return
			}
			go fixture.serve(raw, config, backend)
		}
	}()
	return fixture
}

func (f *sshFixture) serve(raw net.Conn, config *ssh.ServerConfig, backend string) {
	defer raw.Close()
	_, channels, requests, err := ssh.NewServerConn(raw, config)
	if err != nil {
		return
	}
	// open first: a caller that saw accepted >= 1 also sees this connection in open.
	f.open.Add(1)
	f.accepted.Add(1)
	defer f.open.Add(-1)
	go ssh.DiscardRequests(requests)
	var held []ssh.NewChannel
	for newChannel := range channels {
		if f.holdChannels {
			held = append(held, newChannel) // neither accepted nor rejected
			continue
		}
		if newChannel.ChannelType() != "direct-streamlocal@openssh.com" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "only stream-local forwarding")
			continue
		}
		var open streamLocalOpenPayload
		if err := ssh.Unmarshal(newChannel.ExtraData(), &open); err != nil {
			_ = newChannel.Reject(ssh.ConnectionFailed, "malformed stream-local request")
			continue
		}
		select {
		case f.sockets <- open.SocketPath:
		default:
		}
		upstream, err := net.Dial("tcp", backend)
		if err != nil {
			_ = newChannel.Reject(ssh.ConnectionFailed, err.Error())
			continue
		}
		channel, channelRequests, err := newChannel.Accept()
		if err != nil {
			_ = upstream.Close()
			continue
		}
		go ssh.DiscardRequests(channelRequests)
		go func() {
			defer channel.Close()
			defer upstream.Close()
			done := make(chan struct{}, 2)
			go func() { _, _ = io.Copy(upstream, channel); done <- struct{}{} }()
			go func() { _, _ = io.Copy(channel, upstream); done <- struct{}{} }()
			<-done
		}()
	}
}

// waitForClosedSSHConnections waits until the fixture holds no open SSH
// connection: closing the tunnel conn must close its SSH client.
func (f *sshFixture) waitForClosedSSHConnections(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for f.open.Load() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := f.open.Load(); n != 0 {
		t.Fatalf("%d SSH connections still open; closing the tunnel conn must close its SSH client", n)
	}
}

func useInsecureHostKeyForTest(t *testing.T) {
	t.Helper()
	prior := remote.InsecureHostKey
	remote.InsecureHostKey = true
	t.Cleanup(func() { remote.InsecureHostKey = prior })
}

func TestDialEngineOverSSHForwardsTheSocketAndCloseReleasesTheSSHClient(t *testing.T) {
	backend := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "pong")
	}))
	defer backend.Close()
	fixture := startSSHFixture(t, strings.TrimPrefix(backend.URL, "http://"), false)
	useInsecureHostKeyForTest(t)

	for _, tc := range []struct{ name, socket, wantSocket string }{
		{"default socket", "", "/var/run/docker.sock"},
		{"rootless socket", "/run/user/1000/docker.sock", "/run/user/1000/docker.sock"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			conn, err := DialEngineOverSSH(ctx, SSHTarget{Host: fixture.host, Port: fixture.port, User: "fixture", Secret: []byte("fixture"), Socket: tc.socket}, time.Second)
			if err != nil {
				t.Fatalf("DialEngineOverSSH: %v", err)
			}
			if _, err := io.WriteString(conn, "GET / HTTP/1.0\r\n\r\n"); err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(bufio.NewReader(conn))
			if err != nil || !strings.Contains(string(body), "pong") {
				t.Fatalf("answer through the tunnel = %q, %v; want the backend's pong", body, err)
			}
			if got := <-fixture.sockets; got != tc.wantSocket {
				t.Fatalf("forwarded socket = %q, want %q", got, tc.wantSocket)
			}
			_ = conn.Close() // the backend already ended the stream; EOF is expected
			fixture.waitForClosedSSHConnections(t)
		})
	}
}

func TestDialEngineOverSSHBoundsAStalledSocketOpen(t *testing.T) {
	fixture := startSSHFixture(t, "127.0.0.1:1", true)
	useInsecureHostKeyForTest(t)

	// net/http dials with a context that never ends; Background stands in for it.
	start := time.Now()
	conn, err := DialEngineOverSSH(context.Background(), SSHTarget{Host: fixture.host, Port: fixture.port, User: "fixture", Secret: []byte("fixture")}, 200*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatal("a socket open the server never answers succeeded")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("the stalled socket open took %v, want it bounded by the open budget", elapsed)
	}
	if !strings.Contains(err.Error(), "open /var/run/docker.sock") {
		t.Fatalf("error = %v, want the socket open failure", err)
	}
	if fixture.accepted.Load() != 1 {
		t.Fatalf("SSH connections = %d, want 1", fixture.accepted.Load())
	}
	fixture.waitForClosedSSHConnections(t)
}

func TestDialEngineOverSSHClosesTheClientWhenTheForwardFailsOrTheSecretIsWrong(t *testing.T) {
	// A backend address with nothing listening: sshd rejects the stream-local
	// channel, as it does for a missing socket or AllowStreamLocalForwarding no.
	closed, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("IPv4 loopback listener unavailable in this test environment: %v", err)
	}
	backend := closed.Addr().String()
	_ = closed.Close()
	fixture := startSSHFixture(t, backend, false)
	useInsecureHostKeyForTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = DialEngineOverSSH(ctx, SSHTarget{Host: fixture.host, Port: fixture.port, User: "fixture", Secret: []byte("fixture")}, time.Second)
	if err == nil || !strings.Contains(err.Error(), "docker over SSH: open /var/run/docker.sock") {
		t.Fatalf("rejected socket forward = %v, want the stream-local error", err)
	}
	fixture.waitForClosedSSHConnections(t)

	_, err = DialEngineOverSSH(ctx, SSHTarget{Host: fixture.host, Port: fixture.port, User: "fixture", Secret: []byte("wrong")}, time.Second)
	if err == nil || !strings.HasPrefix(err.Error(), "docker over SSH: ") || !strings.Contains(err.Error(), "ssh") {
		t.Fatalf("wrong password = %v, want an SSH authentication error behind the docker over SSH prefix", err)
	}
}
```

- [ ] **Step 3: Run it (expected FAIL)**

```powershell
go test ./internal/dockerhost/transport/ -run 'DialEngineOverSSH' -count=1
```

Expected: `FAIL ... [build failed]` with `undefined: DialEngineOverSSH` and `undefined: SSHTarget`.

- [ ] **Step 4: Create the SSH dialer**

Create `internal/dockerhost/transport/ssh.go`:

```go
package transport

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"aurago/internal/remote"

	"golang.org/x/crypto/ssh"
)

// SSHProbeTimeout bounds the Engine version probe for an Engine reached over
// SSH. The probe's first dial includes a full SSH login (10 s budget in
// remote.DialSSH), so 20 s leaves 10 s for the socket open and /version.
const SSHProbeTimeout = 20 * time.Second

// DefaultEngineSocket is the Engine socket on the remote host when a target
// names none.
const DefaultEngineSocket = "/var/run/docker.sock"

// DefaultSocketOpenTimeout is the socket-open budget used when the caller
// passes a non-positive timeout.
const DefaultSocketOpenTimeout = 10 * time.Second

// SSHTarget names one Engine socket behind SSH.
type SSHTarget struct {
	Host   string
	Port   int // <=0 means 22
	User   string
	Secret []byte // SSH private key or password (remote.DialSSH semantics)
	Socket string // remote Engine socket path, "" means /var/run/docker.sock
}

// DialEngineOverSSH opens one SSH connection (known_hosts verification and
// finite dial/handshake budget via remote.DialSSH) and forwards a stream to the
// remote Engine socket. socketOpenTimeout bounds the direct-streamlocal open:
// net/http detaches the dial from the request, so without it an authenticated
// sshd that never answers the open would keep the SSH client and its
// goroutines alive forever. Closing the returned conn closes the SSH client.
func DialEngineOverSSH(ctx context.Context, target SSHTarget, socketOpenTimeout time.Duration) (net.Conn, error) {
	port := target.Port
	if port <= 0 {
		port = 22
	}
	socket := target.Socket
	if socket == "" {
		socket = DefaultEngineSocket
	}
	if socketOpenTimeout <= 0 {
		socketOpenTimeout = DefaultSocketOpenTimeout
	}
	client, err := remote.DialSSH(ctx, target.Host, port, target.User, target.Secret)
	if err != nil {
		return nil, fmt.Errorf("docker over SSH: %w", err)
	}
	openCtx, cancel := context.WithTimeout(ctx, socketOpenTimeout)
	defer cancel()
	conn, err := client.DialContext(openCtx, "unix", socket)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("docker over SSH: open %s: %w", socket, err)
	}
	return &sshTunnelConn{Conn: conn, client: client}, nil
}

// sshTunnelConn is a forwarded Engine stream that owns its SSH client.
type sshTunnelConn struct {
	net.Conn
	client *ssh.Client
	once   sync.Once
}

func (c *sshTunnelConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() { _ = c.client.Close() })
	return err
}
```

- [ ] **Step 5: Run it (expected PASS)**

```powershell
go test ./internal/dockerhost/transport/ -count=1
```

Expected: `ok aurago/internal/dockerhost/transport`.

- [ ] **Step 6: Make Invasion delegate (three edits in `internal/invasion/connector_docker.go`)**

Edit 1, imports. `sync`, `aurago/internal/remote` and `golang.org/x/crypto/ssh` are used only by the code that moves; add the transport package:

old_string:

```go
	"strings"
	"sync"
	"time"

	"aurago/internal/dockerutil"
	"aurago/internal/remote"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
```

new_string:

```go
	"strings"
	"time"

	"aurago/internal/dockerhost/transport"
	"aurago/internal/dockerutil"
	"gopkg.in/yaml.v3"
```

Edit 2, the probe timeout becomes the shared value:

old_string:

```go
// dockerSSHProbeTimeout bounds the Engine version probe for docker_ssh. Its
// first dial includes a full SSH login, so it covers the SSH dial budget and
// leaves 10 s for the socket open and /version (other transports: 5 s total).
const dockerSSHProbeTimeout = 20 * time.Second
```

new_string:

```go
// dockerSSHProbeTimeout bounds the Engine version probe for docker_ssh. Its
// first dial includes a full SSH login, so it covers the SSH dial budget and
// leaves 10 s for the socket open and /version (other transports: 5 s total).
// The value lives in transport.SSHProbeTimeout so the Docker Manager shares it.
const dockerSSHProbeTimeout = transport.SSHProbeTimeout
```

Edit 3, the dialer and `sshTunnelConn` (the whole old block is removed; the wrapper replaces it):

old_string:

```go
// dialDockerEngineOverSSH opens one SSH connection (known_hosts verification
// and finite dial/handshake budget via remote.DialSSH) and forwards a stream
// to the remote Engine socket. Closing the returned conn closes the SSH client.
func dialDockerEngineOverSSH(ctx context.Context, nest NestRecord, secret []byte) (net.Conn, error) {
	port := nest.Port
	if port <= 0 {
		port = 22
	}
	client, err := remote.DialSSH(ctx, nest.Host, port, nest.Username, secret)
	if err != nil {
		return nil, fmt.Errorf("docker over SSH: %w", err)
	}
	openCtx, cancel := context.WithTimeout(ctx, dockerSSHSocketOpenTimeout)
	defer cancel()
	conn, err := client.DialContext(openCtx, "unix", dockerSSHEngineSocket)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("docker over SSH: open %s: %w", dockerSSHEngineSocket, err)
	}
	return &sshTunnelConn{Conn: conn, client: client}, nil
}

// sshTunnelConn is a forwarded Engine stream that owns its SSH client.
type sshTunnelConn struct {
	net.Conn
	client *ssh.Client
	once   sync.Once
}

func (c *sshTunnelConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() { _ = c.client.Close() })
	return err
}
```

new_string:

```go
// dialDockerEngineOverSSH opens one SSH connection (known_hosts verification
// and finite dial/handshake budget via remote.DialSSH) and forwards a stream
// to the remote Engine socket. Closing the returned conn closes the SSH client.
// The dialer is transport.DialEngineOverSSH; the socket-open budget is passed
// in so tests that shorten dockerSSHSocketOpenTimeout keep working.
func dialDockerEngineOverSSH(ctx context.Context, nest NestRecord, secret []byte) (net.Conn, error) {
	return transport.DialEngineOverSSH(ctx, transport.SSHTarget{
		Host:   nest.Host,
		Port:   nest.Port,
		User:   nest.Username,
		Secret: secret,
		Socket: dockerSSHEngineSocket,
	}, dockerSSHSocketOpenTimeout)
}
```

- [ ] **Step 7: Run Invasion's Docker and SSH tests unchanged**

```powershell
go build ./internal/invasion/
go test ./internal/invasion/ -run 'Docker' -count=1
git diff --stat -- 'internal/invasion/*_test.go'
```

Expected: build prints nothing; `ok aurago/internal/invasion` (about 15 s: one test waits out a 7 s SSH login on purpose); the diff stat prints nothing.

- [ ] **Step 8: Graph check and commit**

```powershell
git add internal/dockerhost/transport/ssh.go internal/dockerhost/transport/ssh_test.go internal/invasion/connector_docker.go
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "refactor(docker-manager): move the SSH Engine dialer into dockerhost/transport" -m "Invasion's dialDockerEngineOverSSH is now a wrapper that passes its own socket-open timeout; error texts and the closing behaviour are unchanged." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A3: `dockerutil.NewClientWithDialer`

**Files:**
- Modify: `internal/dockerutil/client.go` (additive: one constant and one function before `Endpoint`)
- Test: `internal/dockerutil/client_dialer_test.go`

`NewClient` and every existing symbol stay as they are, so no impact run is needed (nothing existing is modified). The new client wraps the dial function in an `http.Transport` with a one-minute idle timeout (an idle SSH tunnel then closes its SSH client) and puts a `VersionTransport` with the caller's probe budget on top.

Source references (HEAD): `NewClient` at `internal/dockerutil/client.go:23`, `DoJSON` at `:53`, `NewVersionTransportWithProbeTimeout` at `internal/dockerutil/version_transport.go:46`.

- [ ] **Step 1: Write the failing test**

Create `internal/dockerutil/client_dialer_test.go`:

```go
package dockerutil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClientWithDialerUsesTheDialFunctionAndNegotiatesTheVersion(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			fmt.Fprint(w, `{"ApiVersion":"1.41","MinAPIVersion":"1.24"}`)
			return
		}
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	var dials atomic.Int32
	client := NewClientWithDialer("test-engine", func(ctx context.Context) (net.Conn, error) {
		dials.Add(1)
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	}, 0, 0)
	defer client.CloseIdleConnections()

	if client.Host() != "test-engine" {
		t.Fatalf("Host() = %q, want the name passed in", client.Host())
	}
	var out struct {
		OK bool `json:"ok"`
	}
	code, err := client.DoJSON(context.Background(), http.MethodGet, "containers/json", nil, &out)
	if err != nil || code != http.StatusOK || !out.OK {
		t.Fatalf("DoJSON = (%d, %v, %+v), want an answer through the dial function", code, err, out)
	}
	mu.Lock()
	got := fmt.Sprint(paths)
	mu.Unlock()
	if got != "[/v1.41/containers/json]" {
		t.Fatalf("Engine saw %s, want the negotiated /v1.41 path", got)
	}
	if dials.Load() == 0 {
		t.Fatal("the dial function was never used")
	}
}

func TestNewClientWithDialerHonoursTheProbeTimeout(t *testing.T) {
	client := NewClientWithDialer("slow-engine", func(ctx context.Context) (net.Conn, error) {
		<-ctx.Done() // an SSH login that never finishes
		return nil, ctx.Err()
	}, 10*time.Second, 50*time.Millisecond)
	defer client.CloseIdleConnections()

	start := time.Now()
	code, err := client.DoJSON(context.Background(), http.MethodGet, "containers/json", nil, nil)
	if code != 0 || err == nil {
		t.Fatalf("DoJSON = (%d, %v), want status 0 with an error", code, err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("the version probe took %v, want it bounded by the 50ms probe timeout", elapsed)
	}
}

func TestNewClientWithDialerWithoutDialFunctionFailsInsteadOfPanicking(t *testing.T) {
	client := NewClientWithDialer("none", nil, time.Second, 0)
	defer client.CloseIdleConnections()
	if code, err := client.DoJSON(context.Background(), http.MethodGet, "containers/json", nil, nil); code != 0 || err == nil {
		t.Fatalf("DoJSON = (%d, %v), want status 0 with an error", code, err)
	}
}
```

- [ ] **Step 2: Run it (expected FAIL)**

```powershell
go test ./internal/dockerutil/ -run 'NewClientWithDialer' -count=1
```

Expected: `FAIL ... [build failed]` with `undefined: NewClientWithDialer`.

- [ ] **Step 3: Add the function**

In `internal/dockerutil/client.go`, insert before the `Endpoint` doc comment:

old_string:

```go
// Endpoint returns a versioned Docker Engine path.
```

new_string:

```go
// dialerIdleConnTimeout reaps idle connections of a NewClientWithDialer client.
// For an Engine reached through SSH this also closes the SSH client the idle
// connection owns.
const dialerIdleConnTimeout = time.Minute

// NewClientWithDialer is NewClient with a caller-supplied dial function and
// version-probe budget, for Engines reached through a transport that an
// endpoint string cannot express (TLS with client certificates, SSH tunnels).
// name is what Host reports. Requests still use Endpoint
// ("http://docker/v1.45/..."): a TLS dial function returns a *tls.Conn, so the
// URL stays plain http for every kind. timeout <= 0 keeps the 30 s request
// timeout and probeTimeout <= 0 the 5 s budget of the /version probe.
// Connections are pooled; CloseIdleConnections releases them at once.
func NewClientWithDialer(name string, dial func(ctx context.Context) (net.Conn, error), timeout, probeTimeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	transport := &http.Transport{
		IdleConnTimeout: dialerIdleConnTimeout,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			if dial == nil {
				return nil, fmt.Errorf("Docker client %q has no dial function", name)
			}
			return dial(ctx)
		},
	}
	return &Client{
		host: name,
		httpClient: &http.Client{
			Transport: NewVersionTransportWithProbeTimeout(transport, probeTimeout),
			Timeout:   timeout,
		},
	}
}

// Endpoint returns a versioned Docker Engine path.
```

- [ ] **Step 4: Run the package tests (expected PASS)**

```powershell
go test ./internal/dockerutil/ -count=1
```

Expected: `ok aurago/internal/dockerutil`.

- [ ] **Step 5: Graph check and commit**

```powershell
git add internal/dockerutil/client.go internal/dockerutil/client_dialer_test.go
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): dockerutil.NewClientWithDialer for TLS and SSH hosts" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A4: `tools` exports the admin inspect redaction

**Files:**
- Create: `internal/tools/docker_inspect_export.go`
- Test: `internal/tools/docker_inspect_export_test.go`
- Modify: `internal/tools/docker.go` (`dockerInspectContainer`: the projection code moves into the new file)

`dockerInspectContainer` builds the trimmed, redacted inspect result inline. The block moves unchanged into `dockerInspectProjection` (new file), `dockerInspectContainer` calls it and adds `"status":"ok"`, and three exported wrappers let `internal/dockerhost` apply the same rules to its own clients. The existing `TestDockerInspect*` suite and the new comparison test pin the output.

Source references (HEAD): `internal/tools/docker.go:705-765` (`dockerInspectContainer`; the block `:729-763` is replaced), the redaction helpers `:829-1028`, the existing tests `internal/tools/docker_inspect_redaction_test.go` (its `dockerInspectMountsFixture` is reused by the new test).

- [ ] **Step 1: Impact check before editing `dockerInspectContainer`**

```powershell
npx gitnexus impact "dockerInspectContainer" --direction upstream --repo AuraGo
```

Fallback:

```powershell
Select-String -Path internal\tools\*.go,internal\server\*.go,internal\agent\*.go -Pattern 'dockerInspectContainer\(|DockerInspectContainer\(|DockerInspectContainerWithMountSources\(' | Where-Object { $_.Path -notmatch '_test.go' } | Select-Object Path,LineNumber
```

Expected: `DockerInspectContainer` and `DockerInspectContainerWithMountSources` call it; their callers are `internal/server/containers_handlers.go` (admin inspect), `internal/server/containers_terminal.go`, `internal/agent/agent_dispatch_services.go` (agent inspect) and Code Studio's workspace check. Hub symbols read HIGH or CRITICAL in this index. Note that in the report: the edit is a pure extraction, the JSON output is identical (map keys marshal sorted) and both the existing inspect tests and the new comparison test run below.

- [ ] **Step 2: Write the failing test**

Create `internal/tools/docker_inspect_export_test.go`:

```go
package tools

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// The Docker Manager trims remote inspect documents with ProjectDockerInspect;
// it must give the admin inspect's answer byte for byte (minus "status").
func TestProjectDockerInspectMatchesTheAdminInspect(t *testing.T) {
	configureDockerSecurityTestPermissions(t, true)
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(dockerInspectMountsFixture))
	})
	admin := DockerInspectContainer(DockerConfig{Host: host}, "db")

	projected, err := ProjectDockerInspect([]byte(dockerInspectMountsFixture))
	if err != nil {
		t.Fatalf("ProjectDockerInspect: %v", err)
	}
	if _, present := projected["status"]; present {
		t.Fatal("the projection must not carry a status key")
	}
	projected["status"] = "ok"
	got, _ := json.Marshal(projected)
	if string(got) != admin {
		t.Fatalf("projection differs from the admin inspect\n got: %s\nwant: %s", got, admin)
	}
	for _, leaked := range []string{"hunter2", "masterauth-secret", "/srv/secret-dir"} {
		if strings.Contains(string(got), leaked) {
			t.Fatalf("projection leaked %q: %s", leaked, got)
		}
	}
	if _, err := ProjectDockerInspect([]byte("not json")); err == nil {
		t.Fatal("ProjectDockerInspect accepted a document that is not JSON")
	}
}

func TestRedactDockerLabelsMasksCredentialLabels(t *testing.T) {
	in := map[string]string{
		"com.example.db_password":    "label-secret",
		"com.example.backup-url":     "s3://key:label-url-secret@bucket",
		"com.docker.compose.service": "cache",
		"aurago.managed":             "go2rtc",
	}
	out := RedactDockerLabels(in)
	for _, leaked := range []string{"label-secret", "label-url-secret"} {
		for key, value := range out {
			if strings.Contains(value, leaked) {
				t.Fatalf("label %s leaked %q: %q", key, leaked, value)
			}
		}
	}
	if out["com.docker.compose.service"] != "cache" || out["aurago.managed"] != "go2rtc" {
		t.Fatalf("ownership labels must stay visible: %v", out)
	}
	if in["com.example.db_password"] != "label-secret" {
		t.Fatal("RedactDockerLabels must not modify its argument")
	}
	if got := RedactDockerLabels(nil); len(got) != 0 {
		t.Fatalf("RedactDockerLabels(nil) = %v, want empty", got)
	}
}

// RedactDockerMountSource must stay in step with the inspect's mount projection.
func TestRedactDockerMountSourceMatchesTheInspectProjection(t *testing.T) {
	for _, mount := range []map[string]interface{}{
		{"Type": "bind", "Source": "/srv/secret-dir"},
		{"Type": "bind", "Source": `C:\data\secret-dir`},
		{"Type": "volume", "Name": "pgdata", "Source": "/var/lib/docker/volumes/pgdata/_data"},
	} {
		projected, _ := projectDockerInspectMounts([]interface{}{mount}, false).([]interface{})
		want, _ := projected[0].(map[string]interface{})["source"].(string)
		if got := RedactDockerMountSource(mount["Type"].(string), mount["Source"].(string)); got != want {
			t.Fatalf("RedactDockerMountSource(%v) = %q, inspect projection gives %q", mount, got, want)
		}
	}
	if got := RedactDockerMountSource("bind", ""); got != "" {
		t.Fatalf("empty source = %q, want empty", got)
	}
}
```

- [ ] **Step 3: Run it (expected FAIL)**

```powershell
go test ./internal/tools/ -run 'TestProjectDockerInspect|TestRedactDocker' -count=1
```

Expected: `FAIL aurago/internal/tools [build failed]` with `undefined: ProjectDockerInspect`, `undefined: RedactDockerLabels`, `undefined: RedactDockerMountSource`.

- [ ] **Step 4: Create the export file**

Create `internal/tools/docker_inspect_export.go`:

```go
package tools

import (
	"encoding/json"
	"fmt"
	pathpkg "path"
	"strings"
)

// dockerInspectProjection trims a full Engine inspect document to the fields
// the agent and the UI may see: credential-like environment values, command
// line secrets and labels are masked, and bind-mount sources are reduced to
// their last path element unless fullBindSources is set. The result has no
// "status" key; dockerInspectContainer adds it.
func dockerInspectProjection(full map[string]interface{}, fullBindSources bool) map[string]interface{} {
	result := map[string]interface{}{
		"id":     full["Id"],
		"name":   full["Name"],
		"state":  full["State"],
		"mounts": projectDockerInspectMounts(full["Mounts"], fullBindSources),
		"config": nil,
	}
	if cfg, ok := full["Config"].(map[string]interface{}); ok {
		result["config"] = map[string]interface{}{
			"image":  cfg["Image"],
			"env":    redactDockerInspectEnv(cfg["Env"]),
			"cmd":    redactDockerInspectArgs(cfg["Cmd"]),
			"labels": redactDockerInspectLabels(cfg["Labels"]),
		}
	}
	if netSettings, ok := full["NetworkSettings"].(map[string]interface{}); ok {
		result["network"] = map[string]interface{}{
			"ip_address": netSettings["IPAddress"],
			"ports":      netSettings["Ports"],
		}
	}
	if hostConfig, ok := full["HostConfig"].(map[string]interface{}); ok {
		if mode, ok := hostConfig["NetworkMode"].(string); ok {
			result["network_mode"] = mode
		}
		if readonly, ok := hostConfig["ReadonlyRootfs"].(bool); ok && readonly {
			result["readonly_rootfs"] = true
		}
		if tmpfs := dockerInspectTmpfs(hostConfig["Tmpfs"]); len(tmpfs) > 0 {
			result["tmpfs"] = tmpfs
		}
	}
	return result
}

// ProjectDockerInspect applies to a raw Engine inspect document (the body of
// GET /containers/{id}/json) exactly the trimming and redaction that
// DockerInspectContainer applies for the agent and the admin UI, without the
// "status" key. The Docker Manager uses it for the hosts it reaches through
// its own clients. Bind-mount sources are reduced to their last path element.
func ProjectDockerInspect(raw []byte) (map[string]interface{}, error) {
	var full map[string]interface{}
	if err := json.Unmarshal(raw, &full); err != nil {
		return nil, fmt.Errorf("parse inspect data: %w", err)
	}
	return dockerInspectProjection(full, false), nil
}

// RedactDockerLabels masks label values whose key names a credential and
// scrubs URL credentials and key=value secrets in the others, with the rules
// of the admin inspect. It returns a new map.
func RedactDockerLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return labels
	}
	generic := make(map[string]interface{}, len(labels))
	for key, value := range labels {
		generic[key] = value
	}
	redacted, _ := redactDockerInspectLabels(generic).(map[string]interface{})
	out := make(map[string]string, len(labels))
	for key, value := range redacted {
		out[key], _ = value.(string)
	}
	return out
}

// RedactDockerMountSource applies the admin inspect's rule to one mount source:
// a volume's source (Docker's own storage) stays complete, any other source
// (a bind's host path) keeps only its last path element.
func RedactDockerMountSource(mountType, source string) string {
	if source == "" || mountType == "volume" {
		return source
	}
	return pathpkg.Base(strings.ReplaceAll(source, `\`, "/"))
}
```

- [ ] **Step 5: Let `dockerInspectContainer` use the shared projection**

In `internal/tools/docker.go`, `dockerInspectContainer` ends with the block below (after `json.Unmarshal(data, &full)`); replace it:

old_string:

```go
	// Extract the most useful fields
	result := map[string]interface{}{
		"status": "ok",
		"id":     full["Id"],
		"name":   full["Name"],
		"state":  full["State"],
		"mounts": projectDockerInspectMounts(full["Mounts"], fullBindSources),
		"config": nil,
	}
	if cfg, ok := full["Config"].(map[string]interface{}); ok {
		result["config"] = map[string]interface{}{
			"image":  cfg["Image"],
			"env":    redactDockerInspectEnv(cfg["Env"]),
			"cmd":    redactDockerInspectArgs(cfg["Cmd"]),
			"labels": redactDockerInspectLabels(cfg["Labels"]),
		}
	}
	if netSettings, ok := full["NetworkSettings"].(map[string]interface{}); ok {
		result["network"] = map[string]interface{}{
			"ip_address": netSettings["IPAddress"],
			"ports":      netSettings["Ports"],
		}
	}
	if hostConfig, ok := full["HostConfig"].(map[string]interface{}); ok {
		if mode, ok := hostConfig["NetworkMode"].(string); ok {
			result["network_mode"] = mode
		}
		if readonly, ok := hostConfig["ReadonlyRootfs"].(bool); ok && readonly {
			result["readonly_rootfs"] = true
		}
		if tmpfs := dockerInspectTmpfs(hostConfig["Tmpfs"]); len(tmpfs) > 0 {
			result["tmpfs"] = tmpfs
		}
	}
	out, _ := json.Marshal(result)
```

new_string:

```go
	// Extract the most useful fields. dockerInspectProjection is shared with
	// ProjectDockerInspect, which trims the inspect documents of the Docker
	// Manager's hosts the same way.
	result := dockerInspectProjection(full, fullBindSources)
	result["status"] = "ok"
	out, _ := json.Marshal(result)
```

- [ ] **Step 6: Run the inspect tests (expected PASS)**

```powershell
go test ./internal/tools/ -run 'TestDockerInspect|TestProjectDockerInspect|TestRedactDocker|TestDockerListContainerEntries' -count=1
go build ./internal/server/ ./internal/agent/
```

Expected: `ok aurago/internal/tools`; the build prints nothing.

- [ ] **Step 7: Graph check and commit**

```powershell
git add internal/tools/docker_inspect_export.go internal/tools/docker_inspect_export_test.go internal/tools/docker.go
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "refactor(docker-manager): export the admin inspect redaction for dockerhost" -m "dockerInspectContainer output is unchanged; ProjectDockerInspect, RedactDockerLabels and RedactDockerMountSource apply the same rules to the Docker Manager's hosts." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A5: Host model, validation and SQLite store

**Files:**
- Create: `internal/dockerhost/host.go`
- Create: `internal/dockerhost/store.go`
- Test: `internal/dockerhost/host_test.go`
- Test: `internal/dockerhost/store_test.go`

`host.go` holds the contract types (`Host`, `SecretInput`, the errors, `Colors`), `ValidateHost` and `Host.Normalized()`. `store.go` opens `docker_manager.db` through `dbutil.Open` (registering the pure-Go `modernc.org/sqlite` driver like the other stores) and keeps the `docker_hosts` table. Validation runs inside `Store.Create`/`Update` too, so the database never holds an invalid row.

Source references (HEAD): `dbutil.Open` at `internal/dbutil/open.go:27` (used with the `modernc.org/sqlite` driver import exactly like `internal/contacts/contacts.go:89`), the id style of the other stores.

- [ ] **Step 1: Write the failing tests**

Create `internal/dockerhost/host_test.go` (a validation table plus the normalization defaults):

```go
package dockerhost

import (
	"errors"
	"strings"
	"testing"
)

func validTCPHost() Host {
	return Host{Name: "NAS", Kind: KindTCP, Address: "nas.lan", Port: 2375, StacksDir: "/opt/stacks", HostSocketPath: "/var/run/docker.sock", Color: "teal"}
}

func TestValidateHost(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*Host)
		wantField string // "" = valid
	}{
		{"valid tcp", func(h *Host) {}, ""},
		{"valid tls with default port", func(h *Host) { h.Kind, h.Port = KindTLS, 0 }, ""},
		{"valid mtls", func(h *Host) { h.Kind = KindMTLS }, ""},
		{"valid ssh", func(h *Host) { h.Kind, h.SSHUser = KindSSH, "root" }, ""},
		{"valid IPv4", func(h *Host) { h.Address = "10.0.0.5" }, ""},
		{"valid bare IPv6", func(h *Host) { h.Address = "fd00::5" }, ""},
		{"valid bracketed IPv6", func(h *Host) { h.Address = "[fd00::5]" }, ""},
		{"valid name with underscore", func(h *Host) { h.Address = "my_nas.local" }, ""},
		{"empty name", func(h *Host) { h.Name = "" }, "name"},
		{"name of 65 characters", func(h *Host) { h.Name = strings.Repeat("a", 65) }, "name"},
		{"name of 64 multibyte characters", func(h *Host) { h.Name = strings.Repeat("\xc3\xa4", 64) }, ""},
		{"unknown kind", func(h *Host) { h.Kind = "http" }, "kind"},
		{"the local kind is built in", func(h *Host) { h.Kind = KindLocal }, "kind"},
		{"empty address", func(h *Host) { h.Address = "" }, "address"},
		{"address with scheme", func(h *Host) { h.Address = "tcp://nas.lan" }, "address"},
		{"address with path", func(h *Host) { h.Address = "nas.lan/docker" }, "address"},
		{"address with port", func(h *Host) { h.Address = "nas.lan:2375" }, "address"},
		{"address with user info", func(h *Host) { h.Address = "root@nas.lan" }, "address"},
		{"address with blank", func(h *Host) { h.Address = "nas lan" }, "address"},
		{"label starting with hyphen", func(h *Host) { h.Address = "-nas.lan" }, "address"},
		{"empty label", func(h *Host) { h.Address = "nas..lan" }, "address"},
		{"unclosed bracket", func(h *Host) { h.Address = "[fd00::5" }, "address"},
		{"bracketed IPv4", func(h *Host) { h.Address = "[10.0.0.5]" }, "address"},
		{"negative port", func(h *Host) { h.Port = -1 }, "port"},
		{"port above 65535", func(h *Host) { h.Port = 65536 }, "port"},
		{"relative stacks dir", func(h *Host) { h.StacksDir = "stacks" }, "stacks_dir"},
		{"stacks dir with ..", func(h *Host) { h.StacksDir = "/opt/../etc" }, "stacks_dir"},
		{"stacks dir is the root", func(h *Host) { h.StacksDir = "/" }, "stacks_dir"},
		{"stacks dir with backslash", func(h *Host) { h.StacksDir = `/opt\stacks` }, "stacks_dir"},
		{"empty stacks dir", func(h *Host) { h.StacksDir = "" }, "stacks_dir"},
		{"relative socket path", func(h *Host) { h.HostSocketPath = "docker.sock" }, "host_socket_path"},
		{"socket path with ..", func(h *Host) { h.HostSocketPath = "/run/../docker.sock" }, "host_socket_path"},
		{"unknown color", func(h *Host) { h.Color = "magenta" }, "color"},
		{"ssh without user", func(h *Host) { h.Kind, h.SSHUser = KindSSH, "" }, "ssh_user"},
		{"ssh user with blank", func(h *Host) { h.Kind, h.SSHUser = KindSSH, "ro ot" }, "ssh_user"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := validTCPHost()
			tc.mutate(&h)
			err := ValidateHost(h)
			if tc.wantField == "" {
				if err != nil {
					t.Fatalf("ValidateHost = %v, want valid", err)
				}
				return
			}
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != tc.wantField {
				t.Fatalf("ValidateHost = %v, want a ValidationError for %q", err, tc.wantField)
			}
		})
	}
}

func TestHostNormalizedAppliesDefaultsAndLeavesTraversalForValidation(t *testing.T) {
	got := Host{Name: "  NAS ", Kind: " TLS ", Address: " nas.lan ", Color: " Teal ", SSHUser: "root", StacksDir: "", HostSocketPath: " /var/run//docker.sock "}.Normalized()
	if got.Name != "NAS" || got.Kind != KindTLS || got.Address != "nas.lan" || got.Color != "teal" {
		t.Fatalf("trimmed fields = %+v", got)
	}
	if got.StacksDir != DefaultStacksDir || got.HostSocketPath != "/var/run/docker.sock" {
		t.Fatalf("paths = %q, %q; want the default stacks dir and the cleaned socket path", got.StacksDir, got.HostSocketPath)
	}
	if got.SSHUser != "" {
		t.Fatalf("SSHUser = %q, want it dropped for a tls host", got.SSHUser)
	}
	if ssh := (Host{Kind: "ssh", SSHUser: " root "}).Normalized(); ssh.SSHUser != "root" {
		t.Fatalf("ssh user = %q, want root", ssh.SSHUser)
	}
	if bad := (Host{StacksDir: "/opt/../etc"}).Normalized(); bad.StacksDir != "/opt/../etc" {
		t.Fatalf("a path with .. segments must reach ValidateHost as typed, got %q", bad.StacksDir)
	}
}
```

Create `internal/dockerhost/store_test.go` (CRUD, ordering, reopen, duplicate names, unique index mapping):

```go
package dockerhost

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func openTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data", "docker_manager.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

func TestStoreCRUD(t *testing.T) {
	ctx := context.Background()
	s, path := openTestStore(t)

	if hosts, err := s.List(ctx); err != nil || hosts == nil || len(hosts) != 0 {
		t.Fatalf("List on an empty store = (%v, %v), want an empty non-nil slice", hosts, err)
	}

	zeta := validTCPHost()
	zeta.Name, zeta.Sort = "zeta", 1
	alpha := validTCPHost()
	alpha.Name, alpha.Sort = "Alpha", 1
	first := validTCPHost()
	first.Name, first.Sort = "first", 0

	created := map[string]Host{}
	for _, h := range []Host{zeta, alpha, first} {
		got, err := s.Create(ctx, h)
		if err != nil {
			t.Fatalf("Create %s: %v", h.Name, err)
		}
		if !regexp.MustCompile(`^h_[0-9a-f]{12}$`).MatchString(got.ID) {
			t.Fatalf("id = %q, want h_ and 12 lowercase hex characters", got.ID)
		}
		if got.CreatedAt.IsZero() || !got.CreatedAt.Equal(got.UpdatedAt) {
			t.Fatalf("timestamps = %v / %v", got.CreatedAt, got.UpdatedAt)
		}
		created[h.Name] = got
	}

	hosts, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, h := range hosts {
		order = append(order, h.Name)
	}
	if want := "first,Alpha,zeta"; strings.Join(order, ",") != want {
		t.Fatalf("list order = %s, want %s (sort, then name ignoring case)", strings.Join(order, ","), want)
	}

	got, err := s.Get(ctx, created["zeta"].ID)
	if err != nil || got.Name != "zeta" || got.Address != "nas.lan" || got.Port != 2375 || got.Color != "teal" {
		t.Fatalf("Get = (%+v, %v)", got, err)
	}

	edit := got
	edit.Name, edit.Kind, edit.SSHUser, edit.Port = "zeta-ssh", KindSSH, "root", 2222
	updated, err := s.Update(ctx, edit)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !updated.CreatedAt.Equal(got.CreatedAt) || updated.UpdatedAt.Before(got.UpdatedAt) {
		t.Fatalf("Update timestamps: created %v -> %v, updated %v -> %v", got.CreatedAt, updated.CreatedAt, got.UpdatedAt, updated.UpdatedAt)
	}

	// The database survives a reopen.
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	again, err := reopened.Get(ctx, got.ID)
	if err != nil || again.Name != "zeta-ssh" || again.Kind != KindSSH || again.SSHUser != "root" || again.Port != 2222 {
		t.Fatalf("after reopen = (%+v, %v)", again, err)
	}

	if err := reopened.Delete(ctx, got.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := reopened.Get(ctx, got.ID); !errors.Is(err, ErrHostNotFound) {
		t.Fatalf("Get after Delete = %v, want ErrHostNotFound", err)
	}
	if err := reopened.Delete(ctx, got.ID); !errors.Is(err, ErrHostNotFound) {
		t.Fatalf("second Delete = %v, want ErrHostNotFound", err)
	}
	if _, err := reopened.Update(ctx, edit); !errors.Is(err, ErrHostNotFound) {
		t.Fatalf("Update of a deleted host = %v, want ErrHostNotFound", err)
	}
}

func TestStoreRejectsInvalidHostsAndDuplicateNames(t *testing.T) {
	ctx := context.Background()
	s, _ := openTestStore(t)

	bad := validTCPHost()
	bad.Address = "tcp://nas.lan"
	var ve *ValidationError
	if _, err := s.Create(ctx, bad); !errors.As(err, &ve) || ve.Field != "address" {
		t.Fatalf("Create with a scheme in the address = %v, want a ValidationError for address", err)
	}

	first, err := s.Create(ctx, validTCPHost())
	if err != nil {
		t.Fatal(err)
	}
	dup := validTCPHost()
	dup.Name = "nas" // same as "NAS" ignoring case
	if _, err := s.Create(ctx, dup); !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("Create with a duplicate name = %v, want a ValidationError for name", err)
	}

	other := validTCPHost()
	other.Name = "other"
	second, err := s.Create(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	clash := second
	clash.Name = "NAS"
	if _, err := s.Update(ctx, clash); !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("Update to a taken name = %v, want a ValidationError for name", err)
	}
	same := first
	same.Port = 2376
	if _, err := s.Update(ctx, same); err != nil {
		t.Fatalf("Update keeping the own name = %v, want success", err)
	}
}

// A concurrent writer can slip past the name check; the unique index then
// answers, and the store reports it like the check does.
func TestStoreMapsTheUniqueNameIndexToAValidationError(t *testing.T) {
	s, _ := openTestStore(t)
	if _, err := s.Create(context.Background(), validTCPHost()); err != nil {
		t.Fatal(err)
	}
	_, err := s.db.Exec(`INSERT INTO docker_hosts (id, name, kind, address, port, stacks_dir, host_socket_path, color, ssh_user, sort, created_at, updated_at)
		VALUES ('h_000000000000', 'nas', 'tcp', 'a', 0, '/a', '/b', 'teal', '', 0, '', '')`)
	if err == nil {
		t.Fatal("the unique name index let a duplicate name in")
	}
	var ve *ValidationError
	if mapped := storeError("store docker host", err); !errors.As(mapped, &ve) || ve.Field != "name" {
		t.Fatalf("storeError(%v) = %v, want a ValidationError for name", err, mapped)
	}
}
```

- [ ] **Step 2: Run them (expected FAIL)**

```powershell
go test ./internal/dockerhost/ -count=1
```

Expected: `FAIL aurago/internal/dockerhost [build failed]` with `undefined: Host`, `undefined: ValidateHost`, `undefined: OpenStore`.

- [ ] **Step 3: Create the host model**

Create `internal/dockerhost/host.go`:

```go
// Package dockerhost owns the Docker Manager's hosts: the registry of the
// local Engine and of remote Engines (plain TCP, TLS, mutual TLS, SSH), their
// vault secrets, one Engine client per host, connection health, and the Engine
// reads and container actions the Docker Manager API serves.
package dockerhost

import (
	"errors"
	"fmt"
	"net"
	"path"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/dockerhost/transport"
)

// Host kinds.
const (
	KindLocal = "local"
	KindTCP   = "tcp"
	KindTLS   = "tls"
	KindMTLS  = "mtls"
	KindSSH   = "ssh"
)

// LocalHostID is the id of the built-in local host. It is never stored.
const LocalHostID = "local"

// Defaults for the per-host settings.
const (
	DefaultStacksDir      = "/opt/stacks"
	DefaultHostSocketPath = "/var/run/docker.sock"
)

// Default ports per kind when a host stores port 0.
const (
	defaultTCPPort = 2375
	defaultTLSPort = 2376
	defaultSSHPort = 22
)

// Colors are the host colors the UI maps to theme tokens.
var Colors = []string{"blue", "teal", "purple", "coral", "pink", "amber", "green", "gray"}

// Host is one Docker Engine the Docker Manager manages.
type Host struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Kind           string    `json:"kind"`
	Address        string    `json:"address"` // local: engine endpoint (from docker.host, read-only); others: hostname/IP (IPv6 may be bracketed)
	Port           int       `json:"port"`    // 0 = default: 2375 tcp, 2376 tls/mtls, 22 ssh
	StacksDir      string    `json:"stacks_dir"`
	HostSocketPath string    `json:"host_socket_path"`
	Color          string    `json:"color"`
	SSHUser        string    `json:"ssh_user,omitempty"`
	Sort           int       `json:"sort"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// SecretInput is write-only. nil fields keep what is stored; Clear removes both
// secrets first. A blank SSHSecret or an empty TLS field also keeps the stored
// value, so a form that leaves the secret fields untouched changes nothing.
type SecretInput struct {
	SSHSecret *string                // ssh only
	TLS       *transport.TLSMaterial // tls/mtls only
	Clear     bool
}

var (
	// ErrHostNotFound is returned for an id that is not in the registry.
	ErrHostNotFound = errors.New("docker host not found")
	// ErrLocalHostFixed is returned when the built-in local host is edited or deleted.
	ErrLocalHostFixed = errors.New("the local Docker host cannot be changed or deleted")
)

// ValidationError names the field of a host that failed validation.
type ValidationError struct{ Field, Message string }

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }

func invalid(field, format string, args ...any) error {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// Normalized returns the host with surrounding blanks trimmed, the color
// lowercased, paths cleaned, empty paths defaulted and the SSH user dropped for
// kinds that do not use one. ValidateHost judges a normalized host.
func (h Host) Normalized() Host {
	h.Name = strings.TrimSpace(h.Name)
	h.Kind = strings.ToLower(strings.TrimSpace(h.Kind))
	h.Address = strings.TrimSpace(h.Address)
	h.Color = strings.ToLower(strings.TrimSpace(h.Color))
	h.SSHUser = strings.TrimSpace(h.SSHUser)
	h.StacksDir = normalizePath(h.StacksDir, DefaultStacksDir)
	h.HostSocketPath = normalizePath(h.HostSocketPath, DefaultHostSocketPath)
	if h.Kind != KindSSH {
		h.SSHUser = ""
	}
	return h
}

// normalizePath trims p and cleans an absolute path. A path with ".." segments
// is left as typed so ValidateHost rejects it instead of Clean resolving it.
func normalizePath(p, fallback string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return fallback
	}
	if strings.HasPrefix(p, "/") && !hasDotDot(p) {
		return path.Clean(p)
	}
	return p
}

func hasDotDot(p string) bool {
	return slices.Contains(strings.Split(p, "/"), "..")
}

// ValidateHost checks a normalized host that is about to be stored or tested:
// name 1..64 runes; kind one of tcp, tls, mtls, ssh (the local host is built
// in); a host name or IP address without scheme, path or port; port 0..65535;
// stacks_dir and host_socket_path absolute POSIX paths without ".." segments
// and not the root; a color from Colors; ssh_user for ssh.
func ValidateHost(h Host) error {
	if n := utf8.RuneCountInString(h.Name); n < 1 || n > 64 {
		return invalid("name", "must be 1 to 64 characters")
	}
	switch h.Kind {
	case KindTCP, KindTLS, KindMTLS, KindSSH:
	case KindLocal:
		return invalid("kind", "the local host is built in and cannot be added")
	default:
		return invalid("kind", "must be one of tcp, tls, mtls, ssh")
	}
	if !validAddress(h.Address) {
		return invalid("address", "must be a host name or IP address without scheme, path or port")
	}
	if h.Port < 0 || h.Port > 65535 {
		return invalid("port", "must be between 0 and 65535")
	}
	if err := validateAbsPath("stacks_dir", h.StacksDir); err != nil {
		return err
	}
	if err := validateAbsPath("host_socket_path", h.HostSocketPath); err != nil {
		return err
	}
	if !slices.Contains(Colors, h.Color) {
		return invalid("color", "must be one of %s", strings.Join(Colors, ", "))
	}
	if h.Kind == KindSSH {
		if h.SSHUser == "" || utf8.RuneCountInString(h.SSHUser) > 64 || strings.ContainsAny(h.SSHUser, " \t\r\n") {
			return invalid("ssh_user", "an SSH user name of up to 64 characters without blanks is required")
		}
	}
	return nil
}

func validateAbsPath(field, p string) error {
	switch {
	case p == "" || !strings.HasPrefix(p, "/"):
		return invalid(field, "must be an absolute path")
	case len(p) > 4096 || strings.ContainsAny(p, "\x00\\"):
		return invalid(field, "must be a POSIX path without backslashes")
	case hasDotDot(p):
		return invalid(field, "must not contain .. segments")
	case path.Clean(p) == "/":
		return invalid(field, "must not be the filesystem root")
	}
	return nil
}

// validAddress accepts a DNS name, an IPv4 address, or an IPv6 address bare or
// in brackets. A scheme, path, query, user info or port is rejected.
func validAddress(a string) bool {
	if a == "" || len(a) > 253 {
		return false
	}
	if strings.HasPrefix(a, "[") {
		if !strings.HasSuffix(a, "]") {
			return false
		}
		ip := net.ParseIP(a[1 : len(a)-1])
		return ip != nil && ip.To4() == nil
	}
	if net.ParseIP(a) != nil {
		return true
	}
	for _, label := range strings.Split(a, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
				return false
			}
		}
	}
	return true
}

// bareAddress strips the brackets of an IPv6 literal.
func bareAddress(a string) string {
	if strings.HasPrefix(a, "[") && strings.HasSuffix(a, "]") {
		return a[1 : len(a)-1]
	}
	return a
}

// effectivePort is the stored port or the default of the kind.
func effectivePort(h Host) int {
	if h.Port > 0 {
		return h.Port
	}
	switch h.Kind {
	case KindTLS, KindMTLS:
		return defaultTLSPort
	case KindSSH:
		return defaultSSHPort
	default:
		return defaultTCPPort
	}
}
```

- [ ] **Step 4: Create the store**

Create `internal/dockerhost/store.go`:

```go
package dockerhost

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/dbutil"

	_ "modernc.org/sqlite"
)

const hostSchema = `
CREATE TABLE IF NOT EXISTS docker_hosts (
	id               TEXT PRIMARY KEY,
	name             TEXT NOT NULL,
	kind             TEXT NOT NULL,
	address          TEXT NOT NULL,
	port             INTEGER NOT NULL DEFAULT 0,
	stacks_dir       TEXT NOT NULL,
	host_socket_path TEXT NOT NULL,
	color            TEXT NOT NULL,
	ssh_user         TEXT NOT NULL DEFAULT '',
	sort             INTEGER NOT NULL DEFAULT 0,
	created_at       TEXT NOT NULL,
	updated_at       TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_docker_hosts_name ON docker_hosts(name COLLATE NOCASE);
`

const hostColumns = "id, name, kind, address, port, stacks_dir, host_socket_path, color, ssh_user, sort, created_at, updated_at"

// Store persists the Docker Manager's remote hosts in docker_manager.db. The
// built-in local host is never stored.
type Store struct{ db *sql.DB }

// OpenStore opens (and creates) the database at path with the shared SQLite
// settings and makes sure the docker_hosts table exists.
func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create docker manager directory: %w", err)
	}
	db, err := dbutil.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open docker manager database: %w", err)
	}
	if _, err := db.Exec(hostSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create docker manager schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// List returns the stored hosts ordered by sort, then name (case-insensitive).
func (s *Store) List(ctx context.Context) ([]Host, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+hostColumns+" FROM docker_hosts ORDER BY sort, name COLLATE NOCASE")
	if err != nil {
		return nil, fmt.Errorf("list docker hosts: %w", err)
	}
	defer rows.Close()
	hosts := []Host{}
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

// Get returns one stored host or ErrHostNotFound.
func (s *Store) Get(ctx context.Context, id string) (Host, error) {
	h, err := scanHost(s.db.QueryRowContext(ctx, "SELECT "+hostColumns+" FROM docker_hosts WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return Host{}, ErrHostNotFound
	}
	return h, err
}

// Create validates and stores a new host under a fresh "h_" id.
func (s *Store) Create(ctx context.Context, h Host) (Host, error) {
	h = h.Normalized()
	if err := ValidateHost(h); err != nil {
		return Host{}, err
	}
	id, err := newHostID()
	if err != nil {
		return Host{}, err
	}
	if err := s.checkNameFree(ctx, id, h.Name); err != nil {
		return Host{}, err
	}
	now := time.Now().UTC()
	h.ID, h.CreatedAt, h.UpdatedAt = id, now, now
	_, err = s.db.ExecContext(ctx, "INSERT INTO docker_hosts ("+hostColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?)",
		h.ID, h.Name, h.Kind, h.Address, h.Port, h.StacksDir, h.HostSocketPath, h.Color, h.SSHUser, h.Sort,
		formatTime(h.CreatedAt), formatTime(h.UpdatedAt))
	if err != nil {
		return Host{}, storeError("store docker host", err)
	}
	return h, nil
}

// Update validates and replaces the mutable fields of a stored host. The id and
// the creation time are kept; ErrHostNotFound when the id is unknown.
func (s *Store) Update(ctx context.Context, h Host) (Host, error) {
	h = h.Normalized()
	if err := ValidateHost(h); err != nil {
		return Host{}, err
	}
	current, err := s.Get(ctx, h.ID)
	if err != nil {
		return Host{}, err
	}
	if err := s.checkNameFree(ctx, h.ID, h.Name); err != nil {
		return Host{}, err
	}
	h.CreatedAt, h.UpdatedAt = current.CreatedAt, time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `UPDATE docker_hosts SET name = ?, kind = ?, address = ?, port = ?, stacks_dir = ?,
		host_socket_path = ?, color = ?, ssh_user = ?, sort = ?, updated_at = ? WHERE id = ?`,
		h.Name, h.Kind, h.Address, h.Port, h.StacksDir, h.HostSocketPath, h.Color, h.SSHUser, h.Sort,
		formatTime(h.UpdatedAt), h.ID)
	if err != nil {
		return Host{}, storeError("update docker host", err)
	}
	return h, nil
}

// Delete removes a stored host or returns ErrHostNotFound.
func (s *Store) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM docker_hosts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete docker host: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrHostNotFound
	}
	return nil
}

// storeError turns the unique-name index violation of a concurrent writer into
// the same validation error checkNameFree gives.
func storeError(op string, err error) error {
	if strings.Contains(err.Error(), "UNIQUE constraint failed: docker_hosts.name") {
		return invalid("name", "another host already uses this name")
	}
	return fmt.Errorf("%s: %w", op, err)
}

// checkNameFree rejects a name another host already uses (case-insensitive).
func (s *Store) checkNameFree(ctx context.Context, id, name string) error {
	var other string
	err := s.db.QueryRowContext(ctx, "SELECT id FROM docker_hosts WHERE name = ? COLLATE NOCASE AND id <> ?", name, id).Scan(&other)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil
	case err != nil:
		return fmt.Errorf("check docker host name: %w", err)
	}
	return invalid("name", "another host already uses this name")
}

type rowScanner interface{ Scan(dest ...any) error }

func scanHost(row rowScanner) (Host, error) {
	var h Host
	var created, updated string
	if err := row.Scan(&h.ID, &h.Name, &h.Kind, &h.Address, &h.Port, &h.StacksDir, &h.HostSocketPath,
		&h.Color, &h.SSHUser, &h.Sort, &created, &updated); err != nil {
		return Host{}, err
	}
	h.CreatedAt = parseTime(created)
	h.UpdatedAt = parseTime(updated)
	return h, nil
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

// newHostID returns "h_" followed by 12 lowercase hex characters.
func newHostID() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate docker host id: %w", err)
	}
	return "h_" + hex.EncodeToString(b[:]), nil
}
```

- [ ] **Step 5: Run them (expected PASS)**

```powershell
go test ./internal/dockerhost/ -count=1
go vet ./internal/dockerhost/
```

Expected: `ok aurago/internal/dockerhost`; vet prints nothing.

- [ ] **Step 6: Graph check and commit**

```powershell
git add internal/dockerhost/host.go internal/dockerhost/store.go internal/dockerhost/host_test.go internal/dockerhost/store_test.go
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): docker host model, validation and SQLite store" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A6: Engine reads and container actions, with the shared fake Engine

**Files:**
- Create: `internal/dockerhost/engine.go`
- Test: `internal/dockerhost/fakeengine_test.go`
- Test: `internal/dockerhost/engine_test.go`

`engine.go` implements the contract functions on a `*dockerutil.Client`: `ListContainers`, `InspectContainer` (through `tools.ProjectDockerInspect`), `ContainerAction`, `RemoveContainer` (`v=true`, like `tools.DockerContainerAction`), `ContainerLogs` (stdout/stderr demultiplexed, TTY-aware through `Config.Tty`, 1 MiB cap keeping the end), `ListImages`, `ListVolumes`, `ListNetworks`, `Info` and `Annotate`. Non-2xx answers come back as `*StatusError` (`StatusOf(err)` gives the code), because the contract functions return only `error`.

**The shared fake Engine (`fakeengine_test.go`)** is used by A6, A7 and A8:

- `newFakeEngine(t)` starts a plain-HTTP `httptest` server on IPv4 loopback (`testutil.NewHTTPServer`); `newFakeTLSEngine(t)` starts an HTTPS one (`testutil.NewHTTPSServer`, handshake errors silenced). Both serve through one handler that looks routes up by `METHOD path`.
- `dockerutil.VersionTransport` probes `GET /version` first and rewrites `/v1.45/...` to the negotiated version. The fake answers `/version` and `/v1.45/version` with `ApiVersion "1.45"` (change it with `setVersion(api, engineVersion)`), serves every other route under `/v1.45/`, and maps any requested `/v1.NN/` prefix onto `/v1.45/` so a fake that reports an older API still works.
- `on(method, route, handler)` and `json(method, route, status, body)` register a route relative to `/v1.45`; unregistered routes answer `404 {"message":"page not found"}`. `withFixtures()` registers the containers, images, volumes, networks and one inspect document (container `solo`); `/info` is registered by default.
- `requests()` returns `METHOD /path?query` of every request in order (the `/version` probe included), for assertions on tail clamps, `v=true`, and so on.
- `host()`, `port()` and `caPEM()` (the server certificate, which is its own CA) build `Host` records; `client(t)` returns a `*dockerutil.Client` against the server.
- `muxFrame(stream, text)` builds one frame of the multiplexed log stream (8-byte header).
- Fixtures: three containers (`shop-web-1` healthy with a bind mount of a private directory, a credential label and two compose labels; `shop-db-1` with `health: starting`; `solo`, exited, no ports or labels), four images (one dangling), three volumes, four networks, `/info` of an x86_64 Debian host.

Source references (HEAD): `internal/tools/docker.go:1259-1302` (`DockerContainerAction`, `v=true` at `:1280`), `:1305-1356` (`DockerContainerLogs` and `stripDockerLogHeaders`, whose 8-byte frame header handling `ContainerLogs` follows), `internal/dockerutil/client.go:53` (`DoJSON` semantics), `internal/dockerutil/version_transport.go:151-170` (the `/v1.45/` rewrite the fake must survive).

- [ ] **Step 1: Write the failing tests**

Create `internal/dockerhost/fakeengine_test.go`:

```go
package dockerhost

import (
	"encoding/binary"
	"encoding/pem"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"

	"aurago/internal/dockerutil"
	"aurago/internal/testutil"
)

// reAPIPrefix matches the version segment the version transport negotiated, so
// a fake that reports an older API still serves the /v1.45 routes.
var reAPIPrefix = regexp.MustCompile(`^/v1\.\d+/`)

// fakeEngine is an httptest server that speaks the part of the Docker Engine
// API the host registry and the engine functions use. dockerutil's version
// transport probes GET /version first and rewrites "/v1.45/" paths to the
// negotiated version, so the fake answers /version with ApiVersion "1.45" and
// serves every other route under /v1.45/ (any /v1.NN/ request maps onto it).
//
//	fe := newFakeEngine(t)          // plain HTTP; newFakeTLSEngine(t) for HTTPS
//	fe.json("GET", "/containers/json", 200, `[...]`)   // route relative to /v1.45
//	fe.on("POST", "/containers/web/stop", func(w, r) { ... })
//	fe.withFixtures()                // containers, images, volumes, networks, info, one inspect
//	c := fe.client(t)                // *dockerutil.Client against the server
//	fe.requests()                    // "METHOD /path?query" of every request, in order
//
// Unregistered routes answer 404 with an Engine style {"message": ...} body.
// host() and port() give the listener address for Host records; caPEM() the
// server certificate (it is its own CA) of the TLS variant.
type fakeEngine struct {
	server *httptest.Server
	mu     sync.Mutex
	routes map[string]http.HandlerFunc
	log    []string
}

func newFakeEngine(t *testing.T) *fakeEngine {
	t.Helper()
	fe := &fakeEngine{routes: map[string]http.HandlerFunc{}}
	fe.server = testutil.NewHTTPServer(t, http.HandlerFunc(fe.serve))
	t.Cleanup(fe.server.Close)
	fe.defaults()
	return fe
}

func newFakeTLSEngine(t *testing.T) *fakeEngine {
	t.Helper()
	fe := &fakeEngine{routes: map[string]http.HandlerFunc{}}
	fe.server = testutil.NewHTTPSServer(t, http.HandlerFunc(fe.serve))
	fe.server.Config.ErrorLog = log.New(io.Discard, "", 0) // handshake errors of the negative tests
	t.Cleanup(fe.server.Close)
	fe.defaults()
	return fe
}

func (fe *fakeEngine) serve(w http.ResponseWriter, r *http.Request) {
	entry := r.Method + " " + r.URL.Path
	if r.URL.RawQuery != "" {
		entry += "?" + r.URL.RawQuery
	}
	fe.mu.Lock()
	fe.log = append(fe.log, entry)
	handler := fe.routes[r.Method+" "+reAPIPrefix.ReplaceAllString(r.URL.Path, "/v1.45/")]
	fe.mu.Unlock()
	if handler == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"page not found"}`)
		return
	}
	handler(w, r)
}

func (fe *fakeEngine) defaults() {
	fe.setVersion("1.45", "27.3.1")
	fe.json("GET", "/info", http.StatusOK, infoFixture)
}

// setVersion answers GET /version and GET /v1.45/version with the given API
// and engine version.
func (fe *fakeEngine) setVersion(api, engine string) {
	body := `{"Version":"` + engine + `","ApiVersion":"` + api + `","MinAPIVersion":"1.24","Os":"linux","Arch":"amd64"}`
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.routes["GET /version"] = handler
	fe.routes["GET /v1.45/version"] = handler
}

// on registers a handler for a route relative to /v1.45.
func (fe *fakeEngine) on(method, route string, handler http.HandlerFunc) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.routes[method+" /v1.45"+route] = handler
}

func (fe *fakeEngine) json(method, route string, status int, body string) {
	fe.on(method, route, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	})
}

func (fe *fakeEngine) requests() []string {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	return append([]string(nil), fe.log...)
}

func (fe *fakeEngine) host() string { return "127.0.0.1" }

func (fe *fakeEngine) port() int {
	return fe.server.Listener.Addr().(*net.TCPAddr).Port
}

func (fe *fakeEngine) caPEM() string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: fe.server.Certificate().Raw}))
}

func (fe *fakeEngine) client(t *testing.T) *dockerutil.Client {
	t.Helper()
	c := dockerutil.NewClient("tcp://127.0.0.1:"+strconv.Itoa(fe.port()), 5*time.Second)
	t.Cleanup(c.CloseIdleConnections)
	return c
}

func (fe *fakeEngine) withFixtures() {
	fe.json("GET", "/containers/json", http.StatusOK, containersFixture)
	fe.json("GET", "/images/json", http.StatusOK, imagesFixture)
	fe.json("GET", "/volumes", http.StatusOK, volumesFixture)
	fe.json("GET", "/networks", http.StatusOK, networksFixture)
	fe.json("GET", "/containers/solo/json", http.StatusOK, inspectFixture)
}

// muxFrame is one frame of the multiplexed log stream: stream 1 is stdout,
// stream 2 stderr.
func muxFrame(stream byte, text string) []byte {
	header := make([]byte, 8)
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:], uint32(len(text)))
	return append(header, text...)
}

const infoFixture = `{"Name":"nas","ServerVersion":"27.3.1","OperatingSystem":"Debian GNU/Linux 12 (bookworm)","OSType":"linux",` +
	`"KernelVersion":"6.1.0-26-amd64","Architecture":"x86_64","DockerRootDir":"/var/lib/docker","NCPU":4,"MemTotal":8589934592,` +
	`"Containers":3,"ContainersRunning":2,"ContainersPaused":0,"ContainersStopped":1,"Images":4}`

// Three containers: shop-web-1 and shop-db-1 belong to the compose project
// "shop", solo is a single exited container. The web container carries a
// credential-like label and a bind mount of a private host directory.
const containersFixture = `[
{"Id":"aaaaaaaaaaaa1111111111111111111111111111111111111111111111111111","Names":["/shop-web-1"],"Image":"nginx:1.27","ImageID":"sha256:img-nginx",
 "Created":1700000000,"State":"running","Status":"Up 2 hours (healthy)",
 "Ports":[{"IP":"::","PrivatePort":80,"PublicPort":8080,"Type":"tcp"},{"IP":"0.0.0.0","PrivatePort":80,"PublicPort":8080,"Type":"tcp"},{"PrivatePort":443,"Type":"tcp"}],
 "Labels":{"com.docker.compose.project":"shop","com.docker.compose.service":"web","com.docker.compose.project.working_dir":"/opt/stacks/shop","com.example.db_password":"label-secret"},
 "NetworkSettings":{"Networks":{"shop_default":{},"bridge":{}}},
 "Mounts":[{"Type":"bind","Source":"/srv/secret-dir/html","Destination":"/usr/share/nginx/html"},{"Type":"volume","Name":"shop_data","Source":"/var/lib/docker/volumes/shop_data/_data","Destination":"/data"}]},
{"Id":"bbbbbbbbbbbb2222222222222222222222222222222222222222222222222222","Names":["/shop-db-1"],"Image":"postgres:16","ImageID":"sha256:img-postgres",
 "Created":1700000100,"State":"running","Status":"Up 5 seconds (health: starting)","Ports":[],
 "Labels":{"com.docker.compose.project":"shop","com.docker.compose.service":"db"},
 "NetworkSettings":{"Networks":{"shop_default":{}}},
 "Mounts":[{"Type":"volume","Name":"shop_pg","Source":"/var/lib/docker/volumes/shop_pg/_data","Destination":"/var/lib/postgresql/data"}]},
{"Id":"cccccccccccc3333333333333333333333333333333333333333333333333333","Names":["/solo"],"Image":"redis:7","ImageID":"sha256:img-redis",
 "Created":1700000200,"State":"exited","Status":"Exited (1) 3 minutes ago","Ports":null,"Labels":null,
 "NetworkSettings":{"Networks":{"bridge":{}}},"Mounts":null}
]`

const imagesFixture = `[
{"Id":"sha256:img-nginx","RepoTags":["nginx:1.27","nginx:latest"],"RepoDigests":["nginx@sha256:aa"],"Created":1690000000,"Size":187000000,"Containers":-1},
{"Id":"sha256:img-postgres","RepoTags":["postgres:16"],"RepoDigests":["postgres@sha256:bb"],"Created":1690000100,"Size":430000000,"Containers":-1},
{"Id":"sha256:img-redis","RepoTags":["redis:7"],"RepoDigests":["redis@sha256:cc"],"Created":1690000200,"Size":117000000,"Containers":-1},
{"Id":"sha256:img-old","RepoTags":["<none>:<none>"],"RepoDigests":null,"Created":1600000000,"Size":50000000,"Containers":-1}
]`

const volumesFixture = `{"Volumes":[
{"Name":"shop_data","Driver":"local","Mountpoint":"/var/lib/docker/volumes/shop_data/_data","CreatedAt":"2024-01-01T10:00:00Z","Labels":{"com.docker.compose.project":"shop"}},
{"Name":"shop_pg","Driver":"local","Mountpoint":"/var/lib/docker/volumes/shop_pg/_data","CreatedAt":"2024-01-01T10:00:01Z","Labels":null},
{"Name":"orphan","Driver":"local","Mountpoint":"/var/lib/docker/volumes/orphan/_data","CreatedAt":"2024-02-01T10:00:00Z","Labels":{}}
],"Warnings":null}`

const networksFixture = `[
{"Id":"net-bridge","Name":"bridge","Driver":"bridge","Scope":"local","IPAM":{"Driver":"default","Config":[{"Subnet":"172.17.0.0/16","Gateway":"172.17.0.1"}]}},
{"Id":"net-host","Name":"host","Driver":"host","Scope":"local","IPAM":{"Driver":"default","Config":[]}},
{"Id":"net-none","Name":"none","Driver":"null","Scope":"local","IPAM":{"Driver":"default","Config":null}},
{"Id":"net-shop","Name":"shop_default","Driver":"bridge","Scope":"local","IPAM":{"Driver":"default","Config":[{"Subnet":"172.20.0.0/16"},{"Subnet":"fd00:1::/64"}]}}
]`

// inspectFixture is the inspect document of the container "solo": secret
// environment values, a credential flag in the command line, a secret label and
// a bind mount of a private host directory.
const inspectFixture = `{"Id":"cccccccccccc3333333333333333333333333333333333333333333333333333","Name":"/solo",` +
	`"State":{"Status":"exited","ExitCode":1},"Mounts":[{"Type":"bind","Source":"/srv/secret-dir","Destination":"/data","Mode":"ro","RW":false}],` +
	`"Config":{"Image":"redis:7","Tty":false,"Env":["REDIS_PASSWORD=hunter2","PATH=/usr/bin"],"Cmd":["redis-server","--requirepass","cmd-secret"],` +
	`"Labels":{"com.example.api_token":"label-secret","com.docker.compose.service":"cache"}},"HostConfig":{"NetworkMode":"bridge"}}`
```

Create `internal/dockerhost/engine_test.go`:

```go
package dockerhost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
)

func TestListContainersMapsTheEngineList(t *testing.T) {
	fe := newFakeEngine(t)
	fe.withFixtures()
	list, err := ListContainers(context.Background(), fe.client(t))
	if err != nil || len(list) != 3 {
		t.Fatalf("ListContainers = (%d containers, %v)", len(list), err)
	}
	if got := fe.requests(); !slices.Contains(got, "GET /v1.45/containers/json?all=true") {
		t.Fatalf("requests = %v, want the list with all=true", got)
	}

	web := list[0]
	if web.ID != "aaaaaaaaaaaa" || len(web.FullID) != 64 || !slices.Equal(web.Names, []string{"shop-web-1"}) ||
		web.Image != "nginx:1.27" || web.ImageID != "sha256:img-nginx" || web.State != "running" || web.Health != "healthy" || web.Created != 1700000000 {
		t.Fatalf("web = %+v", web)
	}
	if web.ComposeProject != "shop" || web.ComposeService != "web" || web.ComposeWorkingDir != "/opt/stacks/shop" {
		t.Fatalf("web compose fields = %q %q %q", web.ComposeProject, web.ComposeService, web.ComposeWorkingDir)
	}
	if !slices.Equal(web.Networks, []string{"bridge", "shop_default"}) {
		t.Fatalf("web networks = %v, want them sorted", web.Networks)
	}
	wantPorts := []Port{{PrivatePort: 80, PublicPort: 8080, Type: "tcp", IP: "0.0.0.0"}, {PrivatePort: 80, PublicPort: 8080, Type: "tcp", IP: "::"}, {PrivatePort: 443, Type: "tcp"}}
	if !slices.Equal(web.Ports, wantPorts) {
		t.Fatalf("web ports = %+v, want %+v", web.Ports, wantPorts)
	}
	if len(web.Mounts) != 2 || web.Mounts[0].Source != "html" || web.Mounts[0].Type != "bind" ||
		web.Mounts[1].Name != "shop_data" || web.Mounts[1].Destination != "/data" {
		t.Fatalf("web mounts = %+v; the bind source must keep only its last path element", web.Mounts)
	}
	if strings.Contains(web.Labels["com.example.db_password"], "label-secret") || web.Labels["com.example.db_password"] == "" {
		t.Fatalf("credential label = %q, want it masked", web.Labels["com.example.db_password"])
	}
	if web.Labels["com.docker.compose.service"] != "web" {
		t.Fatalf("compose label lost: %v", web.Labels)
	}

	if list[1].Health != "starting" || list[2].Health != "" || list[2].State != "exited" {
		t.Fatalf("health/state = %q, %q/%q", list[1].Health, list[2].Health, list[2].State)
	}
	// JSON arrays are never null, even for a container without ports, mounts or labels.
	raw, _ := json.Marshal(list[2])
	for _, field := range []string{`"ports":[]`, `"mounts":[]`, `"names":["solo"]`} {
		if !strings.Contains(string(raw), field) {
			t.Fatalf("solo JSON = %s, want %s", raw, field)
		}
	}
}

func TestInspectContainerRedactsLikeTheAdminInspect(t *testing.T) {
	fe := newFakeEngine(t)
	fe.withFixtures()
	c := fe.client(t)

	raw, err := InspectContainer(context.Background(), c, "solo")
	if err != nil {
		t.Fatalf("InspectContainer: %v", err)
	}
	out := string(raw)
	for _, leaked := range []string{"hunter2", "cmd-secret", "label-secret", "/srv/secret-dir"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("inspect leaked %q: %s", leaked, out)
		}
	}
	for _, kept := range []string{"PATH=/usr/bin", "redis-server", `"com.docker.compose.service":"cache"`, `"source":"secret-dir"`, `"network_mode":"bridge"`} {
		if !strings.Contains(out, kept) {
			t.Fatalf("inspect dropped %q: %s", kept, out)
		}
	}

	if _, err := InspectContainer(context.Background(), c, "../etc"); !errors.Is(err, ErrInvalidContainerRef) {
		t.Fatalf("InspectContainer with a path in the id = %v, want ErrInvalidContainerRef", err)
	}
	if _, err := InspectContainer(context.Background(), c, "ghost"); StatusOf(err) != http.StatusNotFound {
		t.Fatalf("InspectContainer of an unknown container = %v (status %d), want 404", err, StatusOf(err))
	}
}

func TestContainerActionAndRemove(t *testing.T) {
	fe := newFakeEngine(t)
	c := fe.client(t)
	ctx := context.Background()
	for _, action := range []string{"start", "stop", "restart", "pause", "unpause", "kill"} {
		fe.json("POST", "/containers/web/"+action, http.StatusNoContent, "")
	}
	fe.json("POST", "/containers/idle/start", http.StatusNotModified, "")
	fe.json("DELETE", "/containers/web", http.StatusNoContent, "")
	fe.json("DELETE", "/containers/busy", http.StatusConflict, `{"message":"You cannot remove a running container. Stop the container before attempting removal or force remove"}`)

	for _, action := range []string{"start", "stop", "restart", "pause", "unpause", "kill"} {
		if err := ContainerAction(ctx, c, "web", action); err != nil {
			t.Fatalf("ContainerAction %s: %v", action, err)
		}
	}
	requests := strings.Join(fe.requests(), "\n")
	for _, want := range []string{
		"POST /v1.45/containers/web/start", "POST /v1.45/containers/web/stop?t=10", "POST /v1.45/containers/web/restart?t=10",
		"POST /v1.45/containers/web/pause", "POST /v1.45/containers/web/unpause", "POST /v1.45/containers/web/kill",
	} {
		if !strings.Contains(requests+"\n", want+"\n") {
			t.Fatalf("requests =\n%s\nwant %s", requests, want)
		}
	}

	if err := ContainerAction(ctx, c, "idle", "start"); err != nil {
		t.Fatalf("an already started container (304) must count as success: %v", err)
	}
	if err := ContainerAction(ctx, c, "ghost", "start"); StatusOf(err) != http.StatusNotFound {
		t.Fatalf("ContainerAction on an unknown container = %v, want 404", err)
	}
	before := len(fe.requests())
	if err := ContainerAction(ctx, c, "web", "explode"); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("unknown action = %v, want ErrInvalidAction", err)
	}
	if err := ContainerAction(ctx, c, "web/../x", "start"); !errors.Is(err, ErrInvalidContainerRef) {
		t.Fatalf("unsafe id = %v, want ErrInvalidContainerRef", err)
	}
	if len(fe.requests()) != before {
		t.Fatal("rejected input must not reach the Engine")
	}

	if err := RemoveContainer(ctx, c, "web", false); err != nil {
		t.Fatalf("RemoveContainer: %v", err)
	}
	if err := RemoveContainer(ctx, c, "web", true); err != nil {
		t.Fatalf("RemoveContainer force: %v", err)
	}
	requests = strings.Join(fe.requests(), "\n") + "\n"
	for _, want := range []string{"DELETE /v1.45/containers/web?v=true\n", "DELETE /v1.45/containers/web?v=true&force=true\n"} {
		if !strings.Contains(requests, want) {
			t.Fatalf("requests =\n%s\nwant %q (anonymous volumes go with the container)", requests, want)
		}
	}
	if err := RemoveContainer(ctx, c, "busy", false); StatusOf(err) != http.StatusConflict {
		t.Fatalf("RemoveContainer of a running container = %v, want 409", err)
	}
}

func TestContainerLogsDemultiplexesAndClampsTail(t *testing.T) {
	fe := newFakeEngine(t)
	c := fe.client(t)
	ctx := context.Background()
	fe.json("GET", "/containers/web/json", http.StatusOK, `{"Config":{"Tty":false}}`)
	var frames bytes.Buffer
	frames.Write(muxFrame(1, "2024-05-01T10:00:00.000000001Z out line\n"))
	frames.Write(muxFrame(2, "2024-05-01T10:00:01.000000001Z err line\n"))
	frames.Write(muxFrame(1, "2024-05-01T10:00:02.000000001Z last line\n"))
	fe.on("GET", "/containers/web/logs", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(frames.Bytes())
	})

	got, err := ContainerLogs(ctx, c, "web", 0)
	if err != nil {
		t.Fatalf("ContainerLogs: %v", err)
	}
	want := "2024-05-01T10:00:00.000000001Z out line\n2024-05-01T10:00:01.000000001Z err line\n2024-05-01T10:00:02.000000001Z last line\n"
	if got != want {
		t.Fatalf("logs = %q, want the frames without their 8-byte headers %q", got, want)
	}
	if _, err := ContainerLogs(ctx, c, "web", 99999); err != nil {
		t.Fatal(err)
	}
	if _, err := ContainerLogs(ctx, c, "web", 7); err != nil {
		t.Fatal(err)
	}
	requests := strings.Join(fe.requests(), "\n")
	for _, want := range []string{
		"GET /v1.45/containers/web/logs?stdout=true&stderr=true&timestamps=true&tail=200",
		"GET /v1.45/containers/web/logs?stdout=true&stderr=true&timestamps=true&tail=5000",
		"GET /v1.45/containers/web/logs?stdout=true&stderr=true&timestamps=true&tail=7",
	} {
		if !strings.Contains(requests+"\n", want+"\n") {
			t.Fatalf("requests =\n%s\nwant %s", requests, want)
		}
	}

	if _, err := ContainerLogs(ctx, c, "ghost", 10); StatusOf(err) != http.StatusNotFound {
		t.Fatalf("logs of an unknown container = %v, want 404", err)
	}
	if _, err := ContainerLogs(ctx, c, "web;rm", 10); !errors.Is(err, ErrInvalidContainerRef) {
		t.Fatalf("logs with an unsafe id = %v, want ErrInvalidContainerRef", err)
	}
}

func TestContainerLogsOfATTYContainerArePlainText(t *testing.T) {
	fe := newFakeEngine(t)
	c := fe.client(t)
	fe.json("GET", "/containers/shell/json", http.StatusOK, `{"Config":{"Tty":true}}`)
	// A TTY stream is plain text, even when its first bytes look like a frame header.
	tty := "\x01\x00\x00\x00\x00\x00\x00\x05 plain\r\nsecond line\r\n"
	fe.on("GET", "/containers/shell/logs", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, tty) })
	got, err := ContainerLogs(context.Background(), c, "shell", 50)
	if err != nil || got != tty {
		t.Fatalf("TTY logs = (%q, %v), want the raw stream", got, err)
	}

	// A proxy may already have demultiplexed a non-TTY stream: keep it verbatim.
	fe.json("GET", "/containers/proxied/json", http.StatusOK, `{"Config":{"Tty":false}}`)
	fe.on("GET", "/containers/proxied/logs", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "already plain text\n") })
	if got, err := ContainerLogs(context.Background(), c, "proxied", 50); err != nil || got != "already plain text\n" {
		t.Fatalf("proxied logs = (%q, %v), want them unchanged", got, err)
	}
}

func TestContainerLogsKeepTheEndOfAnOversizedStream(t *testing.T) {
	fe := newFakeEngine(t)
	c := fe.client(t)
	fe.json("GET", "/containers/chatty/json", http.StatusOK, `{"Config":{"Tty":false}}`)
	const lines = 60000 // about 1.6 MB of 28 byte lines
	fe.on("GET", "/containers/chatty/logs", func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < lines; i++ {
			_, _ = w.Write(muxFrame(1, fmt.Sprintf("2024-05-01T10:00:00Z line %07d\n", i)))
		}
	})
	got, err := ContainerLogs(context.Background(), c, "chatty", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > logResultMax+len(logOmittedNote) || len(got) < logResultMax/2 {
		t.Fatalf("result is %d bytes, want it capped near 1 MiB", len(got))
	}
	if !strings.HasPrefix(got, logOmittedNote+"2024-05-01T10:00:00Z line ") {
		t.Fatalf("result starts with %q, want the omitted note and then a whole line", got[:60])
	}
	if !strings.HasSuffix(got, fmt.Sprintf("line %07d\n", lines-1)) {
		t.Fatalf("result ends with %q, want the last line kept", got[len(got)-40:])
	}
}

func TestListImagesVolumesNetworksAndInfo(t *testing.T) {
	fe := newFakeEngine(t)
	fe.withFixtures()
	c := fe.client(t)
	ctx := context.Background()

	images, err := ListImages(ctx, c)
	if err != nil || len(images) != 4 {
		t.Fatalf("ListImages = (%d, %v)", len(images), err)
	}
	if images[0].ID != "sha256:img-nginx" || !slices.Equal(images[0].Tags, []string{"nginx:1.27", "nginx:latest"}) || images[0].Size != 187000000 ||
		images[0].Containers != -1 || images[0].Dangling {
		t.Fatalf("nginx image = %+v", images[0])
	}
	if old := images[3]; !old.Dangling || len(old.Tags) != 0 || old.Tags == nil {
		t.Fatalf("the <none>:<none> image = %+v, want dangling with an empty tag list", old)
	}

	volumes, err := ListVolumes(ctx, c)
	if err != nil || len(volumes) != 3 || volumes[0].Name != "shop_data" || volumes[0].Driver != "local" ||
		volumes[0].Labels["com.docker.compose.project"] != "shop" || volumes[0].UsedBy == nil {
		t.Fatalf("ListVolumes = (%+v, %v)", volumes, err)
	}

	networks, err := ListNetworks(ctx, c)
	if err != nil || len(networks) != 4 {
		t.Fatalf("ListNetworks = (%d, %v)", len(networks), err)
	}
	if networks[0].Name != "bridge" || !networks[0].Builtin || !slices.Equal(networks[0].Subnets, []string{"172.17.0.0/16"}) {
		t.Fatalf("bridge network = %+v", networks[0])
	}
	if networks[2].Subnets == nil || len(networks[2].Subnets) != 0 {
		t.Fatalf("a network without IPAM config must have an empty subnet list, got %#v", networks[2].Subnets)
	}
	if shop := networks[3]; shop.Builtin || !slices.Equal(shop.Subnets, []string{"172.20.0.0/16", "fd00:1::/64"}) {
		t.Fatalf("shop network = %+v", shop)
	}

	info, err := Info(ctx, c)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	want := EngineInfo{Name: "nas", ServerVersion: "27.3.1", APIVersion: "1.45", OS: "Debian GNU/Linux 12 (bookworm)", OSType: "linux",
		Kernel: "6.1.0-26-amd64", Arch: "x86_64", DockerRootDir: "/var/lib/docker", NCPU: 4, MemTotal: 8589934592,
		Containers: 3, ContainersRunning: 2, ContainersStopped: 1, Images: 4}
	if info != want {
		t.Fatalf("Info = %+v, want %+v", info, want)
	}
	raw, _ := json.Marshal(info)
	for _, key := range []string{`"server_version"`, `"api_version"`, `"os_type"`, `"docker_root_dir"`, `"ncpu"`, `"mem_total"`, `"containers_running"`, `"containers_paused"`, `"containers_stopped"`} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("EngineInfo JSON = %s, want key %s", raw, key)
		}
	}
}

func TestAnnotateCountsUsage(t *testing.T) {
	fe := newFakeEngine(t)
	fe.withFixtures()
	c := fe.client(t)
	ctx := context.Background()
	containers, _ := ListContainers(ctx, c)
	images, _ := ListImages(ctx, c)
	volumes, _ := ListVolumes(ctx, c)
	networks, _ := ListNetworks(ctx, c)

	Annotate(containers, images, volumes, networks)

	imageCounts := map[string]int{}
	for _, img := range images {
		imageCounts[img.ID] = img.Containers
	}
	if imageCounts["sha256:img-nginx"] != 1 || imageCounts["sha256:img-redis"] != 1 || imageCounts["sha256:img-old"] != 0 {
		t.Fatalf("image counts = %v", imageCounts)
	}
	usedBy := map[string][]string{}
	for _, v := range volumes {
		usedBy[v.Name] = v.UsedBy
	}
	if !slices.Equal(usedBy["shop_data"], []string{"shop-web-1"}) || !slices.Equal(usedBy["shop_pg"], []string{"shop-db-1"}) || len(usedBy["orphan"]) != 0 || usedBy["orphan"] == nil {
		t.Fatalf("volume users = %v", usedBy)
	}
	netCounts := map[string]int{}
	for _, n := range networks {
		netCounts[n.Name] = n.Containers
	}
	if netCounts["bridge"] != 2 || netCounts["shop_default"] != 2 || netCounts["host"] != 0 {
		t.Fatalf("network counts = %v", netCounts)
	}

	// Without a container list the counts stay unknown.
	unknown, _ := ListImages(ctx, c)
	Annotate(nil, unknown, nil, nil)
	if unknown[0].Containers != -1 {
		t.Fatalf("image count without containers = %d, want -1", unknown[0].Containers)
	}
}
```

- [ ] **Step 2: Run them (expected FAIL)**

```powershell
go test ./internal/dockerhost/ -run 'ListContainers|Inspect|ContainerAction|ContainerLogs|ListImages|Annotate' -count=1
```

Expected: `FAIL ... [build failed]` with `undefined: ListContainers`, `undefined: InspectContainer`, `undefined: StatusOf`, and so on.

- [ ] **Step 3: Create the Engine functions**

Create `internal/dockerhost/engine.go`:

```go
package dockerhost

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"aurago/internal/dockerutil"
	"aurago/internal/tools"
)

// StatusError is a non-2xx answer of the Engine; Status is the HTTP status
// code. Callers classify Engine answers by it (404 gone, 409 conflict), never
// by matching the message.
type StatusError struct {
	Status int
	Err    error
}

func (e *StatusError) Error() string { return e.Err.Error() }
func (e *StatusError) Unwrap() error { return e.Err }

// StatusOf returns the HTTP status carried by an Engine answer error, or 0 when
// the error is not one (no answer arrived, or the caller's own validation).
func StatusOf(err error) int {
	var se *StatusError
	if errors.As(err, &se) {
		return se.Status
	}
	return 0
}

var (
	// ErrInvalidAction is returned for a container action the Docker Manager does not offer.
	ErrInvalidAction = errors.New("invalid container action")
	// ErrInvalidContainerRef is returned for a container id or name that is unsafe in an API path.
	ErrInvalidContainerRef = errors.New("invalid container id or name")
)

var reContainerRef = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func validateContainerRef(id string) error {
	if id == "" || len(id) > 128 || !reContainerRef.MatchString(id) || strings.Contains(id, "..") {
		return ErrInvalidContainerRef
	}
	return nil
}

// engineJSON is Client.DoJSON without a request body that wraps a non-2xx
// answer in a *StatusError.
func engineJSON(ctx context.Context, c *dockerutil.Client, method, path string, out any) error {
	status, err := c.DoJSON(ctx, method, path, nil, out)
	if err != nil && status != 0 && (status < 200 || status >= 300) {
		return &StatusError{Status: status, Err: err}
	}
	return err
}

// Port is one published or exposed container port.
type Port struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort int    `json:"private"`
	PublicPort  int    `json:"public,omitempty"`
	Type        string `json:"type"`
}

// Mount is one container mount. A bind's Source keeps only its last path
// element, like the admin inspect.
type Mount struct {
	Type        string `json:"type"`
	Name        string `json:"name,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination"`
}

// ContainerSummary is one row of the container list.
type ContainerSummary struct {
	ID                string            `json:"id"` // 12 chars
	FullID            string            `json:"full_id"`
	Names             []string          `json:"names"` // leading "/" stripped
	Image             string            `json:"image"`
	ImageID           string            `json:"image_id"`
	State             string            `json:"state"`
	Status            string            `json:"status"`
	Health            string            `json:"health,omitempty"` // healthy | unhealthy | starting
	Created           int64             `json:"created"`
	Ports             []Port            `json:"ports"`
	Mounts            []Mount           `json:"mounts"`
	Networks          []string          `json:"networks"`
	ComposeProject    string            `json:"compose_project,omitempty"`
	ComposeService    string            `json:"compose_service,omitempty"`
	ComposeWorkingDir string            `json:"compose_working_dir,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"` // credential-like values masked
}

const (
	labelComposeProject    = "com.docker.compose.project"
	labelComposeService    = "com.docker.compose.service"
	labelComposeWorkingDir = "com.docker.compose.project.working_dir"
)

type engineContainer struct {
	ID      string   `json:"Id"`
	Names   []string `json:"Names"`
	Image   string   `json:"Image"`
	ImageID string   `json:"ImageID"`
	Created int64    `json:"Created"`
	Ports   []struct {
		IP          string `json:"IP"`
		PrivatePort int    `json:"PrivatePort"`
		PublicPort  int    `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
	Labels          map[string]string `json:"Labels"`
	State           string            `json:"State"`
	Status          string            `json:"Status"`
	NetworkSettings struct {
		Networks map[string]json.RawMessage `json:"Networks"`
	} `json:"NetworkSettings"`
	Mounts []struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	} `json:"Mounts"`
}

// ListContainers returns every container of the Engine, running or not.
func ListContainers(ctx context.Context, c *dockerutil.Client) ([]ContainerSummary, error) {
	var raw []engineContainer
	if err := engineJSON(ctx, c, http.MethodGet, "containers/json?all=true", &raw); err != nil {
		return nil, err
	}
	out := make([]ContainerSummary, 0, len(raw))
	for _, rc := range raw {
		s := ContainerSummary{
			FullID:            rc.ID,
			Image:             rc.Image,
			ImageID:           rc.ImageID,
			State:             rc.State,
			Status:            rc.Status,
			Health:            healthFromStatus(rc.Status),
			Created:           rc.Created,
			Names:             make([]string, 0, len(rc.Names)),
			Ports:             make([]Port, 0, len(rc.Ports)),
			Mounts:            make([]Mount, 0, len(rc.Mounts)),
			Networks:          make([]string, 0, len(rc.NetworkSettings.Networks)),
			ComposeProject:    rc.Labels[labelComposeProject],
			ComposeService:    rc.Labels[labelComposeService],
			ComposeWorkingDir: rc.Labels[labelComposeWorkingDir],
			Labels:            tools.RedactDockerLabels(rc.Labels),
		}
		s.ID = rc.ID
		if len(s.ID) > 12 {
			s.ID = s.ID[:12]
		}
		for _, name := range rc.Names {
			s.Names = append(s.Names, strings.TrimPrefix(name, "/"))
		}
		for _, p := range rc.Ports {
			s.Ports = append(s.Ports, Port{IP: p.IP, PrivatePort: p.PrivatePort, PublicPort: p.PublicPort, Type: p.Type})
		}
		slices.SortStableFunc(s.Ports, func(a, b Port) int {
			if a.PrivatePort != b.PrivatePort {
				return a.PrivatePort - b.PrivatePort
			}
			if a.Type != b.Type {
				return strings.Compare(a.Type, b.Type)
			}
			if a.PublicPort != b.PublicPort {
				return a.PublicPort - b.PublicPort
			}
			return strings.Compare(a.IP, b.IP)
		})
		for _, m := range rc.Mounts {
			s.Mounts = append(s.Mounts, Mount{
				Type:        m.Type,
				Name:        m.Name,
				Source:      tools.RedactDockerMountSource(m.Type, m.Source),
				Destination: m.Destination,
			})
		}
		for name := range rc.NetworkSettings.Networks {
			s.Networks = append(s.Networks, name)
		}
		slices.Sort(s.Networks)
		out = append(out, s)
	}
	return out, nil
}

// healthFromStatus reads the health state out of the list's status text
// ("Up 2 hours (healthy)", "Up 5 seconds (health: starting)"); the list API has
// no separate health field.
func healthFromStatus(status string) string {
	switch {
	case strings.Contains(status, "(unhealthy)"):
		return "unhealthy"
	case strings.Contains(status, "(healthy)"):
		return "healthy"
	case strings.Contains(status, "(health: starting)"):
		return "starting"
	}
	return ""
}

// InspectContainer returns the container's inspect document, trimmed and
// redacted exactly like the admin inspect: credential-like environment values,
// command-line secrets and labels are masked and bind sources are reduced to
// their last path element.
func InspectContainer(ctx context.Context, c *dockerutil.Client, id string) (json.RawMessage, error) {
	if err := validateContainerRef(id); err != nil {
		return nil, err
	}
	var raw json.RawMessage
	if err := engineJSON(ctx, c, http.MethodGet, "containers/"+url.PathEscape(id)+"/json", &raw); err != nil {
		return nil, err
	}
	projected, err := tools.ProjectDockerInspect(raw)
	if err != nil {
		return nil, err
	}
	return json.Marshal(projected)
}

// containerActionPaths maps an action to the Engine request suffix.
var containerActionPaths = map[string]string{
	"start":   "start",
	"stop":    "stop?t=10",
	"restart": "restart?t=10",
	"pause":   "pause",
	"unpause": "unpause",
	"kill":    "kill",
}

// ContainerAction runs start, stop, restart, pause, unpause or kill. The Engine
// answers 304 when the container is already in the requested state; that is
// success.
func ContainerAction(ctx context.Context, c *dockerutil.Client, id, action string) error {
	suffix, ok := containerActionPaths[action]
	if !ok {
		return ErrInvalidAction
	}
	if err := validateContainerRef(id); err != nil {
		return err
	}
	err := engineJSON(ctx, c, http.MethodPost, "containers/"+url.PathEscape(id)+"/"+suffix, nil)
	if StatusOf(err) == http.StatusNotModified {
		return nil
	}
	return err
}

// RemoveContainer deletes a container together with its anonymous volumes
// (v=true), like the admin remove; force also removes a running container.
func RemoveContainer(ctx context.Context, c *dockerutil.Client, id string, force bool) error {
	if err := validateContainerRef(id); err != nil {
		return err
	}
	query := "?v=true"
	if force {
		query += "&force=true"
	}
	return engineJSON(ctx, c, http.MethodDelete, "containers/"+url.PathEscape(id)+query, nil)
}

const (
	logTailDefault = 200
	logTailMax     = 5000
	logResultMax   = 1 << 20  // the result keeps at most the last 1 MiB
	logReadMax     = 64 << 20 // raw bytes read from the Engine
	logOmittedNote = "[earlier log lines omitted]\n"
)

// ContainerLogs returns the last tail lines (default 200, 1..5000) of a
// container's stdout and stderr with timestamps. A multiplexed stream is
// demultiplexed; a TTY container's stream is plain text. The result is capped
// at 1 MiB, keeping the end.
func ContainerLogs(ctx context.Context, c *dockerutil.Client, id string, tail int) (string, error) {
	if err := validateContainerRef(id); err != nil {
		return "", err
	}
	if tail <= 0 {
		tail = logTailDefault
	}
	if tail > logTailMax {
		tail = logTailMax
	}
	var inspect struct {
		Config struct {
			Tty bool `json:"Tty"`
		} `json:"Config"`
	}
	ref := url.PathEscape(id)
	if err := engineJSON(ctx, c, http.MethodGet, "containers/"+ref+"/json", &inspect); err != nil {
		return "", err
	}
	endpoint := dockerutil.Endpoint(fmt.Sprintf("containers/%s/logs?stdout=true&stderr=true&timestamps=true&tail=%d", ref, tail))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create Docker request: %w", err)
	}
	resp, err := c.HTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("Docker request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		message := dockerutil.SanitizeOneLine(dockerutil.EngineErrorMessage(dockerutil.ReadErrorBody(resp.Body)), 512)
		return "", &StatusError{Status: resp.StatusCode, Err: fmt.Errorf("Docker API returned %d: %s", resp.StatusCode, message)}
	}
	return readLogs(io.LimitReader(resp.Body, logReadMax), inspect.Config.Tty)
}

func readLogs(r io.Reader, tty bool) (string, error) {
	tb := &tailBuffer{max: logResultMax}
	var err error
	if tty {
		_, err = io.Copy(tb, r)
	} else {
		err = demuxLogs(r, tb)
	}
	if err != nil {
		return "", fmt.Errorf("read container logs: %w", err)
	}
	return tb.String(), nil
}

// demuxLogs copies the frames of a multiplexed Docker stream (8-byte header:
// stream 0..2, three zero bytes, big-endian payload size) to w, stdout and
// stderr in arrival order. A stream that does not start like one is copied
// verbatim, so a proxy that already demultiplexed it still works. A stream cut
// short ends quietly.
func demuxLogs(r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)
	var header [8]byte
	for {
		n, err := io.ReadFull(br, header[:])
		switch {
		case err == io.EOF: // no bytes left: a clean end
			return nil
		case err == io.ErrUnexpectedEOF: // fewer than 8 bytes cannot be a frame; keep them as text
			_, werr := w.Write(header[:n])
			return werr
		case err != nil:
			return err
		}
		if header[0] > 2 || header[1] != 0 || header[2] != 0 || header[3] != 0 {
			if _, werr := w.Write(header[:]); werr != nil {
				return werr
			}
			_, cerr := io.Copy(w, br)
			return cerr
		}
		size := int64(binary.BigEndian.Uint32(header[4:]))
		if _, err := io.CopyN(w, br, size); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// tailBuffer keeps the last max bytes written to it.
type tailBuffer struct {
	max     int
	buf     []byte
	dropped bool
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > 2*t.max {
		t.trim()
	}
	return len(p), nil
}

func (t *tailBuffer) trim() {
	if len(t.buf) > t.max {
		t.buf = append(t.buf[:0], t.buf[len(t.buf)-t.max:]...)
		t.dropped = true
	}
}

// String returns the kept text. When the start was cut, it begins at the next
// whole line behind a short note.
func (t *tailBuffer) String() string {
	t.trim()
	s := t.buf
	if !t.dropped {
		return string(s)
	}
	for len(s) > 0 && !utf8.RuneStart(s[0]) {
		s = s[1:]
	}
	if i := bytes.IndexByte(s, '\n'); i >= 0 && i < 4096 {
		s = s[i+1:]
	}
	return logOmittedNote + string(s)
}

// ImageSummary is one row of the image list.
type ImageSummary struct {
	ID         string   `json:"id"`
	Tags       []string `json:"tags"`
	Size       int64    `json:"size"`
	Created    int64    `json:"created"`
	Containers int      `json:"containers"` // computed by Annotate from ContainerSummary.ImageID; -1 unknown
	Dangling   bool     `json:"dangling"`
}

// VolumeSummary is one row of the volume list.
type VolumeSummary struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Mountpoint string            `json:"mountpoint"`
	CreatedAt  string            `json:"created_at,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	UsedBy     []string          `json:"used_by"` // container names, computed by Annotate
}

// NetworkSummary is one row of the network list.
type NetworkSummary struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Driver     string   `json:"driver"`
	Scope      string   `json:"scope"`
	Subnets    []string `json:"subnets"`
	Containers int      `json:"containers"` // computed by Annotate from ContainerSummary.Networks
	Builtin    bool     `json:"builtin"`    // bridge, host, none
}

// EngineInfo is the host overview.
type EngineInfo struct {
	Name              string `json:"name"`
	ServerVersion     string `json:"server_version"`
	APIVersion        string `json:"api_version"`
	OS                string `json:"os"`
	OSType            string `json:"os_type"`
	Kernel            string `json:"kernel"`
	Arch              string `json:"arch"`
	DockerRootDir     string `json:"docker_root_dir"`
	NCPU              int    `json:"ncpu"`
	MemTotal          int64  `json:"mem_total"`
	Containers        int    `json:"containers"`
	ContainersRunning int    `json:"containers_running"`
	ContainersPaused  int    `json:"containers_paused"`
	ContainersStopped int    `json:"containers_stopped"`
	Images            int    `json:"images"`
}

// ListImages returns the Engine's images. Containers is -1 until Annotate.
func ListImages(ctx context.Context, c *dockerutil.Client) ([]ImageSummary, error) {
	var raw []struct {
		ID          string   `json:"Id"`
		RepoTags    []string `json:"RepoTags"`
		RepoDigests []string `json:"RepoDigests"`
		Created     int64    `json:"Created"`
		Size        int64    `json:"Size"`
	}
	if err := engineJSON(ctx, c, http.MethodGet, "images/json", &raw); err != nil {
		return nil, err
	}
	out := make([]ImageSummary, 0, len(raw))
	for _, ri := range raw {
		tags := make([]string, 0, len(ri.RepoTags))
		for _, tag := range ri.RepoTags {
			if tag != "<none>:<none>" {
				tags = append(tags, tag)
			}
		}
		out = append(out, ImageSummary{
			ID:         ri.ID,
			Tags:       tags,
			Size:       ri.Size,
			Created:    ri.Created,
			Containers: -1,
			Dangling:   len(tags) == 0 && len(ri.RepoDigests) == 0,
		})
	}
	return out, nil
}

// ListVolumes returns the Engine's volumes. UsedBy is empty until Annotate.
func ListVolumes(ctx context.Context, c *dockerutil.Client) ([]VolumeSummary, error) {
	var raw struct {
		Volumes []struct {
			Name       string            `json:"Name"`
			Driver     string            `json:"Driver"`
			Mountpoint string            `json:"Mountpoint"`
			CreatedAt  string            `json:"CreatedAt"`
			Labels     map[string]string `json:"Labels"`
		} `json:"Volumes"`
	}
	if err := engineJSON(ctx, c, http.MethodGet, "volumes", &raw); err != nil {
		return nil, err
	}
	out := make([]VolumeSummary, 0, len(raw.Volumes))
	for _, rv := range raw.Volumes {
		out = append(out, VolumeSummary{
			Name:       rv.Name,
			Driver:     rv.Driver,
			Mountpoint: rv.Mountpoint,
			CreatedAt:  rv.CreatedAt,
			Labels:     tools.RedactDockerLabels(rv.Labels),
			UsedBy:     []string{},
		})
	}
	return out, nil
}

// ListNetworks returns the Engine's networks. Containers is 0 until Annotate.
func ListNetworks(ctx context.Context, c *dockerutil.Client) ([]NetworkSummary, error) {
	var raw []struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		Driver string `json:"Driver"`
		Scope  string `json:"Scope"`
		IPAM   struct {
			Config []struct {
				Subnet string `json:"Subnet"`
			} `json:"Config"`
		} `json:"IPAM"`
	}
	if err := engineJSON(ctx, c, http.MethodGet, "networks", &raw); err != nil {
		return nil, err
	}
	out := make([]NetworkSummary, 0, len(raw))
	for _, rn := range raw {
		n := NetworkSummary{
			ID:      rn.ID,
			Name:    rn.Name,
			Driver:  rn.Driver,
			Scope:   rn.Scope,
			Subnets: []string{},
			Builtin: rn.Name == "bridge" || rn.Name == "host" || rn.Name == "none",
		}
		for _, cfg := range rn.IPAM.Config {
			if cfg.Subnet != "" {
				n.Subnets = append(n.Subnets, cfg.Subnet)
			}
		}
		out = append(out, n)
	}
	return out, nil
}

// Info returns the host overview from /info plus the API version from /version.
func Info(ctx context.Context, c *dockerutil.Client) (EngineInfo, error) {
	var version struct {
		APIVersion string `json:"ApiVersion"`
	}
	if err := engineJSON(ctx, c, http.MethodGet, "version", &version); err != nil {
		return EngineInfo{}, err
	}
	var raw struct {
		Name              string `json:"Name"`
		ServerVersion     string `json:"ServerVersion"`
		OperatingSystem   string `json:"OperatingSystem"`
		OSType            string `json:"OSType"`
		KernelVersion     string `json:"KernelVersion"`
		Architecture      string `json:"Architecture"`
		DockerRootDir     string `json:"DockerRootDir"`
		NCPU              int    `json:"NCPU"`
		MemTotal          int64  `json:"MemTotal"`
		Containers        int    `json:"Containers"`
		ContainersRunning int    `json:"ContainersRunning"`
		ContainersPaused  int    `json:"ContainersPaused"`
		ContainersStopped int    `json:"ContainersStopped"`
		Images            int    `json:"Images"`
	}
	if err := engineJSON(ctx, c, http.MethodGet, "info", &raw); err != nil {
		return EngineInfo{}, err
	}
	return EngineInfo{
		Name:              raw.Name,
		ServerVersion:     raw.ServerVersion,
		APIVersion:        version.APIVersion,
		OS:                raw.OperatingSystem,
		OSType:            raw.OSType,
		Kernel:            raw.KernelVersion,
		Arch:              raw.Architecture,
		DockerRootDir:     raw.DockerRootDir,
		NCPU:              raw.NCPU,
		MemTotal:          raw.MemTotal,
		Containers:        raw.Containers,
		ContainersRunning: raw.ContainersRunning,
		ContainersPaused:  raw.ContainersPaused,
		ContainersStopped: raw.ContainersStopped,
		Images:            raw.Images,
	}, nil
}

// Annotate fills the counts that the Engine's list calls do not give: how many
// containers use each image and network, and which containers use each volume.
// A nil containers slice means the container list is unknown; image counts then
// stay -1 and the other counts stay empty. Matching is by the image id of a
// container, the volume name of a volume mount and the network names of a
// container.
func Annotate(containers []ContainerSummary, images []ImageSummary, volumes []VolumeSummary, networks []NetworkSummary) {
	if containers == nil {
		return
	}
	imageUse := map[string]int{}
	networkUse := map[string]int{}
	volumeUse := map[string][]string{}
	for _, ctr := range containers {
		imageUse[ctr.ImageID]++
		for _, name := range ctr.Networks {
			networkUse[name]++
		}
		label := ctr.ID
		if len(ctr.Names) > 0 {
			label = ctr.Names[0]
		}
		seen := map[string]bool{}
		for _, m := range ctr.Mounts {
			if m.Type == "volume" && m.Name != "" && !seen[m.Name] {
				seen[m.Name] = true
				volumeUse[m.Name] = append(volumeUse[m.Name], label)
			}
		}
	}
	for i := range images {
		images[i].Containers = imageUse[images[i].ID]
	}
	for i := range volumes {
		users := append([]string{}, volumeUse[volumes[i].Name]...)
		slices.Sort(users)
		volumes[i].UsedBy = users
	}
	for i := range networks {
		networks[i].Containers = networkUse[networks[i].Name]
	}
}
```

- [ ] **Step 4: Run them (expected PASS)**

```powershell
go test ./internal/dockerhost/ -count=1
go vet ./internal/dockerhost/
```

Expected: `ok aurago/internal/dockerhost`; vet prints nothing.

- [ ] **Step 5: Graph check and commit**

```powershell
git add internal/dockerhost/engine.go internal/dockerhost/fakeengine_test.go internal/dockerhost/engine_test.go
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): Engine reads and container actions for docker hosts" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A7: Host health: `HostStatus`, `ClassifyError`, connection probe

**Files:**
- Create: `internal/dockerhost/health.go`
- Test: `internal/dockerhost/health_test.go`

`ClassifyError` maps connection and Engine errors to `dns`, `refused`, `timeout`, `tls`, `auth`, `ssh_host_key`, `socket_missing`, `api_too_old` or `unknown`; typed errors (`*net.DNSError`, x509 and TLS error types, `*StatusError` 401/403, `syscall.ECONNREFUSED`, context and net timeouts) win over text matching, and the text rules cover the Windows and ssh wordings. `probeHost` is what `Test` and `TestDraft` run: `GET /version` (an Engine below API 1.41 is refused as `api_too_old`), then `GET /info` for the operating system and architecture (a socket proxy that blocks `/info` leaves the host ok with the `/version` values).

Source references (HEAD): the probe is `internal/dockerutil/version_transport.go:101-149` (its error text "Docker API has no supported version" is classified as `api_too_old`); SSH host-key and authentication wording comes from `remote.GetSSHConfig` (`internal/remote/remote.go:71-105`) and `remote.DialSSH` (`internal/remote/dial_ssh.go:24-56`).

- [ ] **Step 1: Write the failing tests**

Create `internal/dockerhost/health_test.go`:

```go
package dockerhost

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"
)

func TestClassifyError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"dns error type", &net.DNSError{Err: "no such host", Name: "nas.lan", IsNotFound: true}, CodeDNS},
		{"dns timeout is still dns", &net.DNSError{Err: "i/o timeout", Name: "nas.lan", IsTimeout: true}, CodeDNS},
		{"dns text", errors.New("dial tcp: lookup nas.lan: no such host"), CodeDNS},
		{"refused errno", fmt.Errorf("Docker request: %w", &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}), CodeRefused},
		{"refused text", errors.New("dial tcp 10.0.0.5:2375: connect: connection refused"), CodeRefused},
		{"refused windows text", errors.New("dial tcp 127.0.0.1:2375: connectex: No connection could be made because the target machine actively refused it."), CodeRefused},
		{"deadline", fmt.Errorf("Docker request: %w", context.DeadlineExceeded), CodeTimeout},
		{"i/o timeout text", errors.New("dial tcp 10.0.0.9:2375: i/o timeout"), CodeTimeout},
		{"http client timeout", errors.New("Client.Timeout exceeded while awaiting headers"), CodeTimeout},
		{"unknown authority", fmt.Errorf("Docker request: %w", x509.UnknownAuthorityError{}), CodeTLS},
		{"tls text", errors.New("tls: failed to verify certificate: x509: certificate signed by unknown authority"), CodeTLS},
		{"plain engine on a tls port", errors.New("http: server gave HTTP response to HTTPS client"), CodeTLS},
		{"unusable tls material", errors.New("docker_tls_ca contains no PEM certificate"), CodeTLS},
		{"ssh authentication", errors.New("docker over SSH: ssh handshake failed: ssh: handshake failed: ssh: unable to authenticate, attempted methods [none password], no supported methods remain"), CodeAuth},
		{"engine 401", &StatusError{Status: 401, Err: errors.New("Docker API returned 401: unauthorized")}, CodeAuth},
		{"engine 403 from a socket proxy", &StatusError{Status: 403, Err: errors.New("Docker API returned 403: forbidden")}, CodeAuth},
		{"socket permission", errors.New("dial unix /var/run/docker.sock: connect: permission denied"), CodeAuth},
		{"ssh unknown host key", errors.New("docker over SSH: ssh handshake failed: ssh: handshake failed: knownhosts: key is unknown"), CodeSSHHostKey},
		{"ssh missing known_hosts", errors.New("docker over SSH: failed to get ssh config: SSH host key verification failed: open known_hosts"), CodeSSHHostKey},
		{"ssh socket open rejected", errors.New("docker over SSH: open /var/run/docker.sock: ssh: rejected: connect failed (Connection refused)"), CodeSocketMissing},
		{"unix socket missing", errors.New("dial unix /var/run/docker.sock: connect: no such file or directory"), CodeSocketMissing},
		{"named pipe missing", errors.New("open //./pipe/docker_engine: The system cannot find the file specified."), CodeSocketMissing},
		{"api below the supported range", errors.New("negotiate Docker API: Docker API has no supported version (server 1.24, minimum 1.12, client v1.45)"), CodeAPITooOld},
		{"coded error wins", withCode(CodeAPITooOld, errors.New("connection refused")), CodeAPITooOld},
		{"anything else", errors.New("something odd happened"), CodeUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyError(tc.err); got != tc.want {
				t.Fatalf("ClassifyError(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestProbeHostRefusesEnginesBelowAPI141(t *testing.T) {
	for api, want := range map[string]string{"1.40": CodeAPITooOld, "1.41": "", "1.45": ""} {
		fe := newFakeEngine(t)
		fe.setVersion(api, "20.10.0")
		status := probeHost(context.Background(), fe.client(t))
		if want == "" && status.State != StateOK {
			t.Fatalf("API %s: status = %+v, want ok", api, status)
		}
		if want != "" && (status.State != StateError || status.Code != want) {
			t.Fatalf("API %s: status = %+v, want an error with code %s", api, status, want)
		}
	}
}
```

- [ ] **Step 2: Run them (expected FAIL)**

```powershell
go test ./internal/dockerhost/ -run 'ClassifyError|ProbeHost' -count=1
```

Expected: `FAIL ... [build failed]` with `undefined: ClassifyError`, `undefined: probeHost`, `undefined: withCode`.

- [ ] **Step 3: Create the health code**

Create `internal/dockerhost/health.go`:

```go
package dockerhost

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	"aurago/internal/dockerutil"
)

// Host states of a HostStatus.
const (
	StateOK      = "ok"
	StateError   = "error"
	StateUnknown = "unknown"
)

// Codes that classify why a host is unreachable.
const (
	CodeDNS           = "dns"
	CodeRefused       = "refused"
	CodeTimeout       = "timeout"
	CodeTLS           = "tls"
	CodeAuth          = "auth"
	CodeSSHHostKey    = "ssh_host_key"
	CodeSocketMissing = "socket_missing"
	CodeAPITooOld     = "api_too_old"
	CodeUnknown       = "unknown"
)

// minAPIMinor is the oldest Engine API (1.41, Docker 20.10) the Docker Manager works with.
const minAPIMinor = 41

// HostStatus is the result of the last connection test of a host.
type HostStatus struct {
	State         string    `json:"state"`          // "ok" | "error" | "unknown"
	Code          string    `json:"code,omitempty"` // dns, refused, timeout, tls, auth, ssh_host_key, socket_missing, api_too_old, unknown
	Message       string    `json:"message,omitempty"`
	CheckedAt     time.Time `json:"checked_at,omitempty"`
	EngineVersion string    `json:"engine_version,omitempty"`
	APIVersion    string    `json:"api_version,omitempty"`
	OS            string    `json:"os,omitempty"`
	Arch          string    `json:"arch,omitempty"`
}

// classifiedError carries a code chosen where the failure is understood best.
type classifiedError struct {
	code string
	err  error
}

func (e *classifiedError) Error() string { return e.err.Error() }
func (e *classifiedError) Unwrap() error { return e.err }

func withCode(code string, err error) error { return &classifiedError{code: code, err: err} }

// ClassifyError maps a connection or Engine error to one of the status codes:
// dns, refused, timeout, tls, auth, ssh_host_key, socket_missing, api_too_old,
// or unknown. nil gives "".
func ClassifyError(err error) string {
	if err == nil {
		return ""
	}
	var coded *classifiedError
	if errors.As(err, &coded) {
		return coded.code
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return CodeDNS
	}
	if status := StatusOf(err); status == http.StatusUnauthorized || status == http.StatusForbidden {
		return CodeAuth
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "knownhosts:") || strings.Contains(text, "host key"):
		return CodeSSHHostKey
	case strings.Contains(text, "unable to authenticate") || strings.Contains(text, "no supported methods remain") ||
		strings.Contains(text, "permission denied") || strings.Contains(text, "access denied"):
		return CodeAuth
	case strings.Contains(text, "no supported version") || strings.Contains(text, "unsupported docker api version"):
		return CodeAPITooOld
	case isTLSError(err, text):
		return CodeTLS
	case errors.Is(err, context.DeadlineExceeded) || isNetTimeout(err) ||
		strings.Contains(text, "timeout") || strings.Contains(text, "deadline exceeded") || strings.Contains(text, "timed out"):
		return CodeTimeout
	case strings.Contains(text, "docker over ssh: open "): // sshd refused to forward the Engine socket
		return CodeSocketMissing
	case errors.Is(err, syscall.ECONNREFUSED) || strings.Contains(text, "connection refused") || strings.Contains(text, "actively refused"):
		return CodeRefused
	case strings.Contains(text, "open /") || strings.Contains(text, "no such file or directory") ||
		strings.Contains(text, "cannot find the file") || strings.Contains(text, "cannot find the path"):
		return CodeSocketMissing
	case strings.Contains(text, "no such host") || strings.Contains(text, "server misbehaving") ||
		strings.Contains(text, "name resolution"):
		return CodeDNS
	}
	return CodeUnknown
}

func isTLSError(err error, text string) bool {
	var verify *tls.CertificateVerificationError
	var authority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalidCert x509.CertificateInvalidError
	var record tls.RecordHeaderError
	if errors.As(err, &verify) || errors.As(err, &authority) || errors.As(err, &hostname) ||
		errors.As(err, &invalidCert) || errors.As(err, &record) {
		return true
	}
	return strings.Contains(text, "tls:") || strings.Contains(text, "x509:") || strings.Contains(text, "docker_tls") ||
		strings.Contains(text, "certificate") || strings.Contains(text, "http response to https client")
}

func isNetTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// errorStatus is the HostStatus of a failed test.
func errorStatus(err error) HostStatus {
	return HostStatus{
		State:     StateError,
		Code:      ClassifyError(err),
		Message:   dockerutil.SanitizeOneLine(err.Error(), 300),
		CheckedAt: time.Now().UTC(),
	}
}

// apiMinor reads the minor number of an Engine API version such as "1.45".
func apiMinor(version string) (int, bool) {
	major, minor, found := strings.Cut(strings.TrimSpace(version), ".")
	if !found || major != "1" {
		return 0, false
	}
	n, err := strconv.Atoi(minor)
	return n, err == nil
}

// probeHost tests the connection of a client: GET /version for the Engine and
// API version (an Engine below API 1.41 is refused), then GET /info for the
// operating system and architecture. A failing /info (a socket proxy may block
// it) keeps the host ok with the /version values.
func probeHost(ctx context.Context, c *dockerutil.Client) HostStatus {
	var version struct {
		Version    string `json:"Version"`
		APIVersion string `json:"ApiVersion"`
		OS         string `json:"Os"`
		Arch       string `json:"Arch"`
	}
	if err := engineJSON(ctx, c, http.MethodGet, "version", &version); err != nil {
		return errorStatus(err)
	}
	if minor, ok := apiMinor(version.APIVersion); !ok || minor < minAPIMinor {
		return errorStatus(withCode(CodeAPITooOld, fmt.Errorf(
			"Docker Engine API %q is too old; the Docker Manager needs API 1.%d or newer (Docker 20.10+)", version.APIVersion, minAPIMinor)))
	}
	status := HostStatus{
		State:         StateOK,
		CheckedAt:     time.Now().UTC(),
		EngineVersion: version.Version,
		APIVersion:    version.APIVersion,
		OS:            version.OS,
		Arch:          version.Arch,
	}
	var info struct {
		OperatingSystem string `json:"OperatingSystem"`
		OSType          string `json:"OSType"`
		Architecture    string `json:"Architecture"`
	}
	if err := engineJSON(ctx, c, http.MethodGet, "info", &info); err == nil {
		if info.OperatingSystem != "" {
			status.OS = info.OperatingSystem
		} else if info.OSType != "" {
			status.OS = info.OSType
		}
		if info.Architecture != "" {
			status.Arch = info.Architecture
		}
	}
	return status
}
```

- [ ] **Step 4: Run them (expected PASS)**

```powershell
go test ./internal/dockerhost/ -count=1
go vet ./internal/dockerhost/
```

Expected: `ok aurago/internal/dockerhost`; vet prints nothing.

- [ ] **Step 5: Graph check and commit**

```powershell
git add internal/dockerhost/health.go internal/dockerhost/health_test.go
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): host health status and error classification" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A8: Registry, vault secrets, per-kind dialers; Python secret blocklist

**Files:**
- Create: `internal/dockerhost/registry.go`
- Test: `internal/dockerhost/registry_test.go` (also holds the in-memory `memVault`)
- Modify: `internal/tools/python_secrets.go` (one prefix in `blockedSecretPrefixes`)
- Test (modify): `internal/tools/python_secrets_test.go` (two keys in an existing table)

`Registry` ties the store, the vault and the clients together. Dialers per kind: `local` uses `dockerutil.DialContext` on the configured endpoint; `tcp` a `net.Dialer` with a 5 s timeout to `address:port` (default 2375); `tls`/`mtls` a `tls.Dialer` with `transport.TLSClientConfig` and `ServerName` = the address without brackets (default 2376); `ssh` `transport.DialEngineOverSSH` with `Socket = HostSocketPath`, a 10 s socket-open budget and `transport.SSHProbeTimeout` for the version probe. Clients are cached per host id and rebuilt when the connection fingerprint changes; `Update` and `Delete` drop them.

Source references (HEAD): `internal/tools/python_secrets.go:18-81` (`blockedSecretPrefixes`, `"nest_"` at `:26`) and `:151-162` (`IsPythonAccessibleSecret`); the vault is `internal/security/vault.go` (`ReadSecret` `:198`, `WriteSecret` `:227`, `DeleteSecret` `:385`).

- [ ] **Step 1: Impact check before touching `blockedSecretPrefixes`**

```powershell
npx gitnexus impact "IsPythonAccessibleSecret" --direction upstream --repo AuraGo
```

Fallback:

```powershell
Select-String -Path internal\tools\*.go,internal\agent\*.go,internal\server\*.go -Pattern 'IsPythonAccessibleSecret|blockedSecretPrefixes' | Where-Object { $_.Path -notmatch '_test.go' } | Select-Object Path,LineNumber
Select-String -Path internal\*\*.go,ui\*.js -Pattern '"docker_host_' -List | Select-Object Path
```

Expected: `ResolveVaultSecrets` and the Python tool dispatch use the function; the list is only read. Adding a prefix changes the answer only for vault keys that start with `docker_host_`, and the second search finds no existing key with that prefix (the `docker_host_*` hits in `ui/cfg` are i18n keys, not vault keys). Risk LOW.

- [ ] **Step 2: Write the failing tests**

Create `internal/dockerhost/registry_test.go`:

```go
package dockerhost

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	"aurago/internal/dockerhost/transport"
	"aurago/internal/dockerutil"
	"aurago/internal/security"
)

// memVault is an in-memory SecretVault. Writes to keys with a prefix in
// failWritePrefix fail.
type memVault struct {
	mu              sync.Mutex
	secrets         map[string]string
	failWritePrefix string
}

func newMemVault() *memVault { return &memVault{secrets: map[string]string{}} }

func (v *memVault) ReadSecret(key string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	value, ok := v.secrets[key]
	if !ok {
		return "", security.ErrSecretNotFound
	}
	return value, nil
}

func (v *memVault) WriteSecret(key, value string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.failWritePrefix != "" && strings.HasPrefix(key, v.failWritePrefix) {
		return io.ErrClosedPipe
	}
	v.secrets[key] = value
	return nil
}

func (v *memVault) DeleteSecret(key string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if _, ok := v.secrets[key]; !ok {
		return security.ErrSecretNotFound
	}
	delete(v.secrets, key)
	return nil
}

func (v *memVault) has(key string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	_, ok := v.secrets[key]
	return ok
}

func newTestRegistry(t *testing.T, localEndpoint string) (*Registry, *memVault) {
	t.Helper()
	store, _ := openTestStore(t)
	vault := newMemVault()
	reg := NewRegistry(store, vault, func() string { return localEndpoint })
	t.Cleanup(reg.Close)
	return reg, vault
}

func tcpHostFor(fe *fakeEngine, name string) Host {
	return Host{Name: name, Kind: KindTCP, Address: fe.host(), Port: fe.port(), Color: "teal"}
}

func TestRegistryLocalHostIsSynthesizedAndFixed(t *testing.T) {
	ctx := context.Background()
	reg, _ := newTestRegistry(t, "tcp://127.0.0.1:1")

	hosts, err := reg.Hosts(ctx)
	if err != nil || len(hosts) != 1 {
		t.Fatalf("Hosts = (%v, %v), want only the local host", hosts, err)
	}
	local := hosts[0]
	if local.ID != LocalHostID || local.Name != "Local" || local.Kind != KindLocal || local.Address != "tcp://127.0.0.1:1" ||
		local.StacksDir != DefaultStacksDir || local.HostSocketPath != DefaultHostSocketPath || local.Color != "blue" || local.Sort != -1 {
		t.Fatalf("local host = %+v", local)
	}
	if _, err := reg.Update(ctx, LocalHostID, local, SecretInput{}); !errors.Is(err, ErrLocalHostFixed) {
		t.Fatalf("Update(local) = %v, want ErrLocalHostFixed", err)
	}
	if err := reg.Delete(ctx, LocalHostID); !errors.Is(err, ErrLocalHostFixed) {
		t.Fatalf("Delete(local) = %v, want ErrLocalHostFixed", err)
	}
	if reg.SecretSet(LocalHostID) {
		t.Fatal("the local host has no secrets")
	}

	// An empty docker.host falls back to the OS default endpoint.
	fallback := NewRegistry(reg.store, newMemVault(), nil)
	if got, _ := fallback.Host(ctx, LocalHostID); got.Address != dockerutil.DefaultHost() {
		t.Fatalf("local address without docker.host = %q, want %q", got.Address, dockerutil.DefaultHost())
	}
}

func TestRegistryTCPHostLifecycle(t *testing.T) {
	ctx := context.Background()
	fe := newFakeEngine(t)
	fe.withFixtures()
	reg, vault := newTestRegistry(t, "")

	created, err := reg.Create(ctx, tcpHostFor(fe, "NAS"), SecretInput{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.StacksDir != DefaultStacksDir || created.HostSocketPath != DefaultHostSocketPath {
		t.Fatalf("defaults not applied: %+v", created)
	}
	if status := reg.Status(created.ID); status.State != StateUnknown {
		t.Fatalf("Status before a test = %+v, want unknown", status)
	}
	if reg.SecretSet(created.ID) || len(vault.secrets) != 0 {
		t.Fatal("a plain TCP host stores no secrets")
	}

	status := reg.Test(ctx, created.ID)
	if status.State != StateOK || status.EngineVersion != "27.3.1" || status.APIVersion != "1.45" ||
		status.OS != "Debian GNU/Linux 12 (bookworm)" || status.Arch != "x86_64" || status.CheckedAt.IsZero() {
		t.Fatalf("Test = %+v, want ok with versions, OS and architecture from /info", status)
	}
	if cached := reg.Status(created.ID); cached != status {
		t.Fatalf("Status = %+v, want the cached %+v", cached, status)
	}

	first, err := reg.Client(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second, _ := reg.Client(ctx, created.ID); second != first {
		t.Fatal("the client of an unchanged host must be cached")
	}
	if first.Host() != "tcp://127.0.0.1:"+strconv.Itoa(fe.port()) {
		t.Fatalf("client name = %q", first.Host())
	}
	containers, err := ListContainers(ctx, first)
	if err != nil || len(containers) != 3 {
		t.Fatalf("ListContainers through the registry client = (%d, %v)", len(containers), err)
	}

	// Editing the connection settings drops the client and the status.
	edit := created
	edit.Name = "NAS 2"
	edit.Port = fe.port() // same engine, but the stored record changes
	edit.Color = "coral"
	updated, err := reg.Update(ctx, created.ID, edit, SecretInput{})
	if err != nil || updated.Name != "NAS 2" || updated.Color != "coral" {
		t.Fatalf("Update = (%+v, %v)", updated, err)
	}
	if status := reg.Status(created.ID); status.State != StateUnknown {
		t.Fatalf("Status after Update = %+v, want unknown", status)
	}
	if rebuilt, _ := reg.Client(ctx, created.ID); rebuilt == first {
		t.Fatal("Update must drop the cached client")
	}

	if err := reg.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := reg.Host(ctx, created.ID); !errors.Is(err, ErrHostNotFound) {
		t.Fatalf("Host after Delete = %v, want ErrHostNotFound", err)
	}
	if err := reg.Delete(ctx, created.ID); !errors.Is(err, ErrHostNotFound) {
		t.Fatalf("second Delete = %v, want ErrHostNotFound", err)
	}
	if st := reg.Test(ctx, created.ID); st.State != StateError {
		t.Fatalf("Test of an unknown id = %+v, want an error", st)
	}
	if got := reg.Status(created.ID); got.State != StateUnknown {
		t.Fatalf("an unknown id must not be cached, got %+v", got)
	}
}

func TestRegistryTLSHostVerifiesTheEngineAgainstTheStoredCA(t *testing.T) {
	ctx := context.Background()
	fe := newFakeTLSEngine(t)
	reg, vault := newTestRegistry(t, "")

	host := Host{Name: "secure", Kind: KindTLS, Address: fe.host(), Port: fe.port(), Color: "purple"}
	trusted, err := reg.Create(ctx, host, SecretInput{TLS: &transport.TLSMaterial{CA: fe.caPEM()}})
	if err != nil {
		t.Fatalf("Create with CA: %v", err)
	}
	if !reg.SecretSet(trusted.ID) || !vault.has(TLSSecretKey(trusted.ID)) {
		t.Fatal("the CA must be stored in the vault under docker_host_tls_<id>")
	}
	var stored transport.TLSMaterial
	if err := json.Unmarshal([]byte(vault.secrets[TLSSecretKey(trusted.ID)]), &stored); err != nil || stored.CA == "" {
		t.Fatalf("stored TLS material = %q, %v", vault.secrets[TLSSecretKey(trusted.ID)], err)
	}
	if status := reg.Test(ctx, trusted.ID); status.State != StateOK {
		t.Fatalf("Test with the engine CA = %+v, want ok", status)
	}

	// Without the CA the self-signed engine certificate is refused, never ignored.
	host.Name = "untrusted"
	untrusted, err := reg.Create(ctx, host, SecretInput{})
	if err != nil {
		t.Fatalf("Create without CA: %v", err)
	}
	if status := reg.Test(ctx, untrusted.ID); status.State != StateError || status.Code != CodeTLS {
		t.Fatalf("Test without the CA = %+v, want error code tls", status)
	}

	// Switching to plain TCP removes the TLS entry from the vault.
	switched := trusted
	switched.Kind = KindTCP
	if _, err := reg.Update(ctx, trusted.ID, switched, SecretInput{}); err != nil {
		t.Fatalf("Update to tcp: %v", err)
	}
	if vault.has(TLSSecretKey(trusted.ID)) || reg.SecretSet(trusted.ID) {
		t.Fatal("a host that no longer uses TLS must not keep TLS material")
	}
}

func TestRegistrySecretRules(t *testing.T) {
	ctx := context.Background()
	reg, vault := newTestRegistry(t, "")
	secret := "p4ssw0rd"
	wantField := func(err error, field string) {
		t.Helper()
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != field {
			t.Fatalf("error = %v, want a ValidationError for %q", err, field)
		}
	}

	ssh := Host{Name: "pi", Kind: KindSSH, Address: "10.0.0.7", SSHUser: "pi", Color: "green"}
	_, err := reg.Create(ctx, ssh, SecretInput{})
	wantField(err, "ssh_secret")
	blank := "  "
	_, err = reg.Create(ctx, ssh, SecretInput{SSHSecret: &blank})
	wantField(err, "ssh_secret")

	created, err := reg.Create(ctx, ssh, SecretInput{SSHSecret: &secret})
	if err != nil {
		t.Fatalf("Create ssh: %v", err)
	}
	if vault.secrets[SSHSecretKey(created.ID)] != secret || !reg.SecretSet(created.ID) {
		t.Fatal("the SSH secret must be in the vault under docker_host_ssh_<id>")
	}
	client, err := reg.Client(ctx, created.ID)
	if err != nil || client.Host() != "ssh://pi@10.0.0.7:22" {
		t.Fatalf("ssh client = (%v, %v); it must build without dialing and name no secret", client, err)
	}

	// Updating without secret input keeps the stored secret; Clear removes it.
	renamed := created
	renamed.Name = "pi-2"
	if _, err := reg.Update(ctx, created.ID, renamed, SecretInput{}); err != nil || vault.secrets[SSHSecretKey(created.ID)] != secret {
		t.Fatalf("Update without secret input = %v, stored %q; want the secret kept", err, vault.secrets[SSHSecretKey(created.ID)])
	}
	_, err = reg.Update(ctx, created.ID, renamed, SecretInput{Clear: true})
	wantField(err, "ssh_secret")
	if vault.secrets[SSHSecretKey(created.ID)] != secret {
		t.Fatal("a rejected Update must leave the vault as it was")
	}

	// Other validation: names, kinds, TLS material.
	_, err = reg.Create(ctx, Host{Name: "LOCAL", Kind: KindTCP, Address: "10.0.0.8", Color: "gray"}, SecretInput{})
	wantField(err, "name")
	_, err = reg.Create(ctx, Host{Name: "mtls-box", Kind: KindMTLS, Address: "10.0.0.9", Color: "gray"}, SecretInput{})
	wantField(err, "tls")
	_, err = reg.Create(ctx, Host{Name: "bad-ca", Kind: KindTLS, Address: "10.0.0.9", Color: "gray"}, SecretInput{TLS: &transport.TLSMaterial{CA: "not a certificate"}})
	wantField(err, "tls")
	_, err = reg.Create(ctx, Host{Name: "local-kind", Kind: KindLocal, Address: "x", Color: "gray"}, SecretInput{})
	wantField(err, "kind")

	// Delete removes the vault entries.
	if err := reg.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if vault.has(SSHSecretKey(created.ID)) {
		t.Fatal("Delete must remove the host's vault entries")
	}
}

func TestRegistryCreateRollsBackWhenTheVaultWriteFails(t *testing.T) {
	ctx := context.Background()
	reg, vault := newTestRegistry(t, "")
	vault.failWritePrefix = "docker_host_ssh_"
	secret := "p4ssw0rd"
	ssh := Host{Name: "pi", Kind: KindSSH, Address: "10.0.0.7", SSHUser: "pi", Color: "green"}
	if _, err := reg.Create(ctx, ssh, SecretInput{SSHSecret: &secret}); err == nil {
		t.Fatal("Create succeeded although the vault refused the secret")
	}
	if hosts, _ := reg.store.List(ctx); len(hosts) != 0 {
		t.Fatalf("a host whose secret could not be stored must not remain: %+v", hosts)
	}
}

func TestRegistryTestDraftDoesNotSaveAndClassifiesFailures(t *testing.T) {
	ctx := context.Background()
	fe := newFakeEngine(t)
	reg, _ := newTestRegistry(t, "")

	if status := reg.TestDraft(ctx, tcpHostFor(fe, "draft"), SecretInput{}); status.State != StateOK || status.EngineVersion != "27.3.1" {
		t.Fatalf("TestDraft of a reachable engine = %+v", status)
	}
	if hosts, _ := reg.store.List(ctx); len(hosts) != 0 {
		t.Fatalf("TestDraft must not save the host: %+v", hosts)
	}

	// A port with nothing listening: refused, not a hang.
	closed, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("IPv4 loopback listener unavailable in this test environment: %v", err)
	}
	refusedPort := closed.Addr().(*net.TCPAddr).Port
	_ = closed.Close()
	dead := Host{Name: "dead", Kind: KindTCP, Address: "127.0.0.1", Port: refusedPort, Color: "gray"}
	status := reg.TestDraft(ctx, dead, SecretInput{})
	if status.State != StateError || status.Code != CodeRefused {
		t.Fatalf("TestDraft of a closed port = %+v, want error code refused", status)
	}

	// An invalid draft is reported as a status, not a panic.
	if status := reg.TestDraft(ctx, Host{Name: "x", Kind: KindTCP, Address: "tcp://x"}, SecretInput{}); status.State != StateError || !strings.Contains(status.Message, "address") {
		t.Fatalf("TestDraft of an invalid draft = %+v", status)
	}

	// A draft that names a stored host reuses its stored secrets.
	tlsEngine := newFakeTLSEngine(t)
	saved, err := reg.Create(ctx, Host{Name: "secure", Kind: KindTLS, Address: tlsEngine.host(), Port: tlsEngine.port(), Color: "purple"},
		SecretInput{TLS: &transport.TLSMaterial{CA: tlsEngine.caPEM()}})
	if err != nil {
		t.Fatal(err)
	}
	if status := reg.TestDraft(ctx, saved, SecretInput{}); status.State != StateOK {
		t.Fatalf("TestDraft of a stored host without secret input = %+v, want the stored CA used", status)
	}
}

func TestRegistryTestCachesUnreachableAndOldEngineStatus(t *testing.T) {
	ctx := context.Background()
	reg, _ := newTestRegistry(t, "")

	closed, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("IPv4 loopback listener unavailable in this test environment: %v", err)
	}
	refusedPort := closed.Addr().(*net.TCPAddr).Port
	_ = closed.Close()
	dead, err := reg.Create(ctx, Host{Name: "dead", Kind: KindTCP, Address: "127.0.0.1", Port: refusedPort, Color: "gray"}, SecretInput{})
	if err != nil {
		t.Fatal(err)
	}
	status := reg.Test(ctx, dead.ID)
	if status.State != StateError || status.Code != CodeRefused || status.Message == "" {
		t.Fatalf("Test of a closed port = %+v, want error code refused with a message", status)
	}
	if reg.Status(dead.ID) != status {
		t.Fatal("a failed test must be cached too")
	}

	old := newFakeEngine(t)
	old.setVersion("1.40", "19.03.0")
	legacy, err := reg.Create(ctx, tcpHostFor(old, "legacy"), SecretInput{})
	if err != nil {
		t.Fatal(err)
	}
	if status := reg.Test(ctx, legacy.ID); status.State != StateError || status.Code != CodeAPITooOld {
		t.Fatalf("Test of an API 1.40 engine = %+v, want error code api_too_old", status)
	}
}
```

In `internal/tools/python_secrets_test.go`, add the two vault keys to the table of `TestIsPythonAccessibleSecret_BlocksSystemVaultKeysAddedByIntegrations`:

old_string:

```go
		"here_now_api_key",
	} {
		if IsPythonAccessibleSecret(key) {
			t.Errorf("expected system-managed key
```

new_string:

```go
		"here_now_api_key",
		"docker_host_ssh_h_0123456789ab",
		"docker_host_tls_h_0123456789ab",
	} {
		if IsPythonAccessibleSecret(key) {
			t.Errorf("expected system-managed key
```

- [ ] **Step 3: Run them (expected FAIL)**

```powershell
go test ./internal/dockerhost/ -run 'Registry' -count=1
go test ./internal/tools/ -run 'TestIsPythonAccessibleSecret_BlocksSystemVaultKeysAddedByIntegrations' -count=1
```

Expected: the first fails to build (`undefined: NewRegistry`, `undefined: SSHSecretKey`, `undefined: TLSSecretKey`); the second prints `expected system-managed key "docker_host_ssh_h_0123456789ab" to be blocked, but it was allowed`.

- [ ] **Step 4: Create the registry**

Create `internal/dockerhost/registry.go`:

```go
package dockerhost

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/dockerhost/transport"
	"aurago/internal/dockerutil"
	"aurago/internal/security"
)

// SecretVault is the part of the AuraGo vault the registry needs.
type SecretVault interface {
	ReadSecret(key string) (string, error)
	WriteSecret(key, value string) error
	DeleteSecret(key string) error
}

// SSHSecretKey names the vault entry of a host's SSH private key or password.
func SSHSecretKey(id string) string { return "docker_host_ssh_" + id }

// TLSSecretKey names the vault entry of a host's TLS material (JSON ca/cert/key).
func TLSSecretKey(id string) string { return "docker_host_tls_" + id }

// LocalEndpointFunc returns the configured docker.host ("" = dockerutil.DefaultHost()).
type LocalEndpointFunc func() string

const (
	localHostName     = "Local"
	clientTimeout     = 30 * time.Second
	dialTimeout       = 5 * time.Second
	sshSocketOpenWait = 10 * time.Second
	testBudget        = 30 * time.Second
)

// secretBundle is everything the vault holds for one host.
type secretBundle struct {
	ssh string
	tls *transport.TLSMaterial
}

func (b secretBundle) tlsMaterial() transport.TLSMaterial {
	if b.tls == nil {
		return transport.TLSMaterial{}
	}
	return *b.tls
}

// tlsJSON is the vault form of the TLS material; "" means no entry.
func (b secretBundle) tlsJSON() string {
	if b.tls == nil {
		return ""
	}
	raw, _ := json.Marshal(b.tls)
	return string(raw)
}

type cachedClient struct {
	fingerprint string
	client      *dockerutil.Client
}

// Registry is the list of Docker hosts: the built-in local host plus the stored
// remote hosts, their secrets in the vault, one cached Engine client per host
// and the last connection test result of each.
type Registry struct {
	store         *Store
	vault         SecretVault
	localEndpoint LocalEndpointFunc

	mu         sync.Mutex
	clients    map[string]cachedClient
	status     map[string]HostStatus
	secretFlag map[string]bool
}

// NewRegistry returns a registry over a store and a vault. localEndpoint may be
// nil (the OS default Engine endpoint is used).
func NewRegistry(store *Store, vault SecretVault, localEndpoint LocalEndpointFunc) *Registry {
	return &Registry{
		store:         store,
		vault:         vault,
		localEndpoint: localEndpoint,
		clients:       map[string]cachedClient{},
		status:        map[string]HostStatus{},
		secretFlag:    map[string]bool{},
	}
}

func (r *Registry) localHost() Host {
	endpoint := ""
	if r.localEndpoint != nil {
		endpoint = r.localEndpoint()
	}
	return Host{
		ID:             LocalHostID,
		Name:           localHostName,
		Kind:           KindLocal,
		Address:        dockerutil.NormalizeHost(endpoint),
		StacksDir:      DefaultStacksDir,
		HostSocketPath: DefaultHostSocketPath,
		Color:          "blue",
		Sort:           -1,
	}
}

// Hosts returns the local host first, then the stored hosts.
func (r *Registry) Hosts(ctx context.Context) ([]Host, error) {
	stored, err := r.store.List(ctx)
	if err != nil {
		return nil, err
	}
	return append([]Host{r.localHost()}, stored...), nil
}

// Host returns the local host or one stored host (ErrHostNotFound).
func (r *Registry) Host(ctx context.Context, id string) (Host, error) {
	if id == LocalHostID {
		return r.localHost(), nil
	}
	return r.store.Get(ctx, id)
}

// Create validates and stores a host and its secrets. A failure while writing
// the secrets removes the host again.
func (r *Registry) Create(ctx context.Context, h Host, sec SecretInput) (Host, error) {
	h = h.Normalized()
	h.ID = ""
	if h.Color == "" {
		h.Color = r.nextColor(ctx)
	}
	if err := checkLocalName(h.Name); err != nil {
		return Host{}, err
	}
	if err := ValidateHost(h); err != nil {
		return Host{}, err
	}
	next, err := resolveSecrets(h, secretBundle{}, sec)
	if err != nil {
		return Host{}, err
	}
	created, err := r.store.Create(ctx, h)
	if err != nil {
		return Host{}, err
	}
	if _, err := r.applySecrets(created.ID, secretBundle{}, next); err != nil {
		_ = r.store.Delete(ctx, created.ID)
		return Host{}, err
	}
	return created, nil
}

// Update replaces a stored host and merges the secret input into its stored
// secrets. The cached client and the cached status are dropped.
func (r *Registry) Update(ctx context.Context, id string, h Host, sec SecretInput) (Host, error) {
	if id == LocalHostID {
		return Host{}, ErrLocalHostFixed
	}
	current, err := r.store.Get(ctx, id)
	if err != nil {
		return Host{}, err
	}
	h = h.Normalized()
	h.ID = id
	if h.Color == "" {
		h.Color = current.Color
	}
	if err := checkLocalName(h.Name); err != nil {
		return Host{}, err
	}
	if err := ValidateHost(h); err != nil {
		return Host{}, err
	}
	prev, err := r.loadBundle(id)
	if err != nil {
		return Host{}, err
	}
	next, err := resolveSecrets(h, prev, sec)
	if err != nil {
		return Host{}, err
	}
	undo, err := r.applySecrets(id, prev, next)
	if err != nil {
		return Host{}, err
	}
	updated, err := r.store.Update(ctx, h)
	if err != nil {
		undo()
		return Host{}, err
	}
	r.forget(id)
	return updated, nil
}

// Delete removes a stored host, its cached client and its vault secrets.
func (r *Registry) Delete(ctx context.Context, id string) error {
	if id == LocalHostID {
		return ErrLocalHostFixed
	}
	if err := r.store.Delete(ctx, id); err != nil {
		return err
	}
	r.forget(id)
	if r.vault == nil {
		return nil
	}
	var errs []error
	for _, key := range []string{SSHSecretKey(id), TLSSecretKey(id)} {
		if err := r.vault.DeleteSecret(key); err != nil && !errors.Is(err, security.ErrSecretNotFound) {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("host removed, but its stored secrets could not be deleted: %w", errors.Join(errs...))
	}
	return nil
}

// SecretSet reports whether the vault holds any secret for the host. The answer
// is remembered until the host is updated or deleted.
func (r *Registry) SecretSet(id string) bool {
	if id == LocalHostID || r.vault == nil {
		return false
	}
	r.mu.Lock()
	set, known := r.secretFlag[id]
	r.mu.Unlock()
	if known {
		return set
	}
	definitive := true // a vault error says nothing about the secret, so it is not remembered
	for _, key := range []string{SSHSecretKey(id), TLSSecretKey(id)} {
		value, err := r.vault.ReadSecret(key)
		if err == nil && value != "" {
			set = true
			break
		}
		if err != nil && !errors.Is(err, security.ErrSecretNotFound) {
			definitive = false
		}
	}
	if set || definitive {
		r.mu.Lock()
		r.secretFlag[id] = set
		r.mu.Unlock()
	}
	return set
}

// Client returns the Engine client of a host. It is cached per host and rebuilt
// when the host's connection settings change.
func (r *Registry) Client(ctx context.Context, id string) (*dockerutil.Client, error) {
	h, err := r.Host(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.clientFor(h)
}

func (r *Registry) clientFor(h Host) (*dockerutil.Client, error) {
	fingerprint := hostFingerprint(h)
	r.mu.Lock()
	cached, ok := r.clients[h.ID]
	r.mu.Unlock()
	if ok && cached.fingerprint == fingerprint {
		return cached.client, nil
	}
	bundle, err := r.bundleForClient(h)
	if err != nil {
		return nil, err
	}
	client, err := newClient(h, bundle)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, found := r.clients[h.ID]; found {
		if existing.fingerprint == fingerprint { // another goroutine built it first
			client.CloseIdleConnections()
			return existing.client, nil
		}
		existing.client.CloseIdleConnections() // the host's settings changed
	}
	r.clients[h.ID] = cachedClient{fingerprint: fingerprint, client: client}
	return client, nil
}

// Status returns the last connection test result of a host; State "unknown"
// before the first test.
func (r *Registry) Status(id string) HostStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	if status, ok := r.status[id]; ok {
		return status
	}
	return HostStatus{State: StateUnknown}
}

// Test checks the connection of a stored host (GET /version, then /info) and
// caches the result.
func (r *Registry) Test(ctx context.Context, id string) HostStatus {
	h, err := r.Host(ctx, id)
	if err != nil {
		return errorStatus(err) // an unknown id is not cached
	}
	ctx, cancel := context.WithTimeout(ctx, testBudget)
	defer cancel()
	var status HostStatus
	if client, err := r.clientFor(h); err != nil {
		status = errorStatus(err)
	} else {
		status = probeHost(ctx, client)
	}
	r.mu.Lock()
	r.status[id] = status
	r.mu.Unlock()
	return status
}

// TestDraft checks the connection of a host that is not saved. Secrets the
// input leaves out come from the stored host when h.ID names one. The client is
// not cached and is closed afterwards.
func (r *Registry) TestDraft(ctx context.Context, h Host, sec SecretInput) HostStatus {
	h = h.Normalized()
	if h.Color == "" {
		h.Color = Colors[0]
	}
	if err := ValidateHost(h); err != nil {
		return errorStatus(err)
	}
	var prev secretBundle
	if h.ID != "" && h.ID != LocalHostID {
		if _, err := r.store.Get(ctx, h.ID); err == nil {
			if prev, err = r.loadBundle(h.ID); err != nil {
				return errorStatus(err)
			}
		}
	}
	bundle, err := resolveSecrets(h, prev, sec)
	if err != nil {
		return errorStatus(err)
	}
	client, err := newClient(h, bundle)
	if err != nil {
		return errorStatus(err)
	}
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(ctx, testBudget)
	defer cancel()
	return probeHost(ctx, client)
}

// Close closes the idle connections of the cached clients, which also ends the
// SSH sessions they hold.
func (r *Registry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, cached := range r.clients {
		cached.client.CloseIdleConnections()
		delete(r.clients, id)
	}
}

// forget drops everything cached for a host.
func (r *Registry) forget(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cached, ok := r.clients[id]; ok {
		cached.client.CloseIdleConnections()
		delete(r.clients, id)
	}
	delete(r.status, id)
	delete(r.secretFlag, id)
}

func (r *Registry) nextColor(ctx context.Context) string {
	stored, err := r.store.List(ctx)
	if err != nil {
		return Colors[1]
	}
	return Colors[(len(stored)+1)%len(Colors)]
}

func checkLocalName(name string) error {
	if strings.EqualFold(strings.TrimSpace(name), localHostName) {
		return invalid("name", "another host already uses this name")
	}
	return nil
}

// resolveSecrets merges the input into the stored secrets for the kind of h and
// checks the result: an SSH host needs a secret, a TLS host needs usable PEM
// material. Secrets that the kind does not use are dropped.
func resolveSecrets(h Host, stored secretBundle, in SecretInput) (secretBundle, error) {
	out := stored
	if in.Clear {
		out = secretBundle{}
	}
	if in.SSHSecret != nil && strings.TrimSpace(*in.SSHSecret) != "" {
		out.ssh = *in.SSHSecret
	}
	if in.TLS != nil {
		merged := out.tlsMaterial()
		if v := strings.TrimSpace(in.TLS.CA); v != "" {
			merged.CA = v
		}
		if v := strings.TrimSpace(in.TLS.Cert); v != "" {
			merged.Cert = v
		}
		if v := strings.TrimSpace(in.TLS.Key); v != "" {
			merged.Key = v
		}
		out.tls = &merged
	}
	switch h.Kind {
	case KindSSH:
		out.tls = nil
		if strings.TrimSpace(out.ssh) == "" {
			return secretBundle{}, invalid("ssh_secret", "an SSH password or private key is required")
		}
	case KindTLS, KindMTLS:
		out.ssh = ""
		material := out.tlsMaterial()
		if h.Kind == KindTLS {
			material.Cert, material.Key = "", "" // server-only TLS never presents a client certificate
		}
		if _, err := transport.TLSClientConfig(h.Kind, material); err != nil {
			return secretBundle{}, invalid("tls", "%s", err.Error())
		}
		out.tls = &material
		if material == (transport.TLSMaterial{}) {
			out.tls = nil // system roots: nothing to store
		}
	default:
		out = secretBundle{}
	}
	return out, nil
}

// errTLSMaterialUnreadable reports a vault entry that is not valid JSON.
var errTLSMaterialUnreadable = errors.New("the stored TLS material of this host is unreadable")

func (r *Registry) readSSH(id string) (string, error) {
	if r.vault == nil {
		return "", errors.New("vault is not available")
	}
	value, err := r.vault.ReadSecret(SSHSecretKey(id))
	if errors.Is(err, security.ErrSecretNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read SSH secret: %w", err)
	}
	return value, nil
}

func (r *Registry) readTLS(id string) (*transport.TLSMaterial, error) {
	if r.vault == nil {
		return nil, errors.New("vault is not available")
	}
	raw, err := r.vault.ReadSecret(TLSSecretKey(id))
	if errors.Is(err, security.ErrSecretNotFound) || (err == nil && raw == "") {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read TLS material: %w", err)
	}
	var material transport.TLSMaterial
	if err := json.Unmarshal([]byte(raw), &material); err != nil {
		return nil, errTLSMaterialUnreadable
	}
	return &material, nil
}

// loadBundle reads both secrets of a host for an update. Unreadable TLS
// material counts as empty so that the update can replace it.
func (r *Registry) loadBundle(id string) (secretBundle, error) {
	var b secretBundle
	if r.vault == nil {
		return b, nil
	}
	ssh, err := r.readSSH(id)
	if err != nil {
		return b, err
	}
	material, err := r.readTLS(id)
	if err != nil && !errors.Is(err, errTLSMaterialUnreadable) {
		return b, err
	}
	b.ssh, b.tls = ssh, material
	return b, nil
}

// bundleForClient reads the secret a host's transport uses.
func (r *Registry) bundleForClient(h Host) (secretBundle, error) {
	var b secretBundle
	switch h.Kind {
	case KindSSH:
		secret, err := r.readSSH(h.ID)
		if err != nil {
			return b, err
		}
		b.ssh = secret
	case KindTLS, KindMTLS:
		material, err := r.readTLS(h.ID)
		if err != nil {
			if errors.Is(err, errTLSMaterialUnreadable) {
				return b, withCode(CodeTLS, err)
			}
			return b, err
		}
		b.tls = material
	}
	return b, nil
}

// applySecrets writes the difference between prev and next to the vault and
// returns a function that puts the vault back as it was. A failure part-way
// restores what was already written.
func (r *Registry) applySecrets(id string, prev, next secretBundle) (func(), error) {
	var undo []func()
	rollback := func() {
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
	}
	steps := []struct{ key, before, after string }{
		{SSHSecretKey(id), prev.ssh, next.ssh},
		{TLSSecretKey(id), prev.tlsJSON(), next.tlsJSON()},
	}
	for _, step := range steps {
		if step.before == step.after {
			continue
		}
		if r.vault == nil {
			rollback()
			return nil, errors.New("vault is not available")
		}
		if err := r.putSecret(step.key, step.after); err != nil {
			rollback()
			return nil, fmt.Errorf("store host secret: %w", err)
		}
		key, before := step.key, step.before
		undo = append(undo, func() { _ = r.putSecret(key, before) })
	}
	r.mu.Lock()
	delete(r.secretFlag, id)
	r.mu.Unlock()
	return rollback, nil
}

// putSecret writes a vault entry, or deletes it for an empty value.
func (r *Registry) putSecret(key, value string) error {
	if value == "" {
		if err := r.vault.DeleteSecret(key); err != nil && !errors.Is(err, security.ErrSecretNotFound) {
			return err
		}
		return nil
	}
	return r.vault.WriteSecret(key, value)
}

// hostFingerprint changes whenever a setting that shapes the connection
// changes, so a cached client is replaced.
func hostFingerprint(h Host) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		h.Kind, h.Address, strconv.Itoa(effectivePort(h)), h.HostSocketPath, h.SSHUser,
	}, "\x00")))
	return hex.EncodeToString(sum[:])
}

// endpointLabel names the connection without any secret; it is what
// Client.Host reports.
func endpointLabel(h Host) string {
	switch h.Kind {
	case KindLocal:
		return h.Address
	case KindSSH:
		return "ssh://" + h.SSHUser + "@" + net.JoinHostPort(bareAddress(h.Address), strconv.Itoa(effectivePort(h)))
	default:
		return h.Kind + "://" + net.JoinHostPort(bareAddress(h.Address), strconv.Itoa(effectivePort(h)))
	}
}

// newClient builds the Engine client of a host from its settings and secrets.
// Nothing is dialed until the first request.
func newClient(h Host, b secretBundle) (*dockerutil.Client, error) {
	var dial func(ctx context.Context) (net.Conn, error)
	var probeTimeout time.Duration
	hostPort := net.JoinHostPort(bareAddress(h.Address), strconv.Itoa(effectivePort(h)))
	switch h.Kind {
	case KindLocal:
		endpoint := h.Address
		dial = func(ctx context.Context) (net.Conn, error) { return dockerutil.DialContext(ctx, endpoint) }
	case KindTCP:
		dialer := &net.Dialer{Timeout: dialTimeout}
		dial = func(ctx context.Context) (net.Conn, error) { return dialer.DialContext(ctx, "tcp", hostPort) }
	case KindTLS, KindMTLS:
		config, err := transport.TLSClientConfig(h.Kind, b.tlsMaterial())
		if err != nil {
			return nil, withCode(CodeTLS, err)
		}
		config.ServerName = bareAddress(h.Address)
		dialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: dialTimeout}, Config: config}
		dial = func(ctx context.Context) (net.Conn, error) { return dialer.DialContext(ctx, "tcp", hostPort) }
	case KindSSH:
		if strings.TrimSpace(b.ssh) == "" {
			return nil, withCode(CodeAuth, errors.New("no SSH password or private key is stored for this host"))
		}
		target := transport.SSHTarget{
			Host:   bareAddress(h.Address),
			Port:   effectivePort(h),
			User:   h.SSHUser,
			Secret: []byte(b.ssh),
			Socket: h.HostSocketPath,
		}
		dial = func(ctx context.Context) (net.Conn, error) {
			return transport.DialEngineOverSSH(ctx, target, sshSocketOpenWait)
		}
		probeTimeout = transport.SSHProbeTimeout
	default:
		return nil, fmt.Errorf("unsupported host kind %q", h.Kind)
	}
	return dockerutil.NewClientWithDialer(endpointLabel(h), dial, clientTimeout, probeTimeout), nil
}
```

- [ ] **Step 5: Block the vault prefix for Python tools**

In `internal/tools/python_secrets.go`:

old_string:

```go
	"nest_",
	"auth_",
```

new_string:

```go
	"nest_",
	"docker_host_",
	"auth_",
```

- [ ] **Step 6: Run them (expected PASS)**

```powershell
go test ./internal/dockerhost/... -count=1
go test ./internal/tools/ -run 'TestIsPythonAccessibleSecret|TestResolveVaultSecrets' -count=1
go vet ./internal/dockerhost/...
```

Expected: `ok` for `aurago/internal/dockerhost`, `aurago/internal/dockerhost/transport` and `aurago/internal/tools`; vet prints nothing.

- [ ] **Step 7: Graph check and commit**

```powershell
git add internal/dockerhost/registry.go internal/dockerhost/registry_test.go internal/tools/python_secrets.go internal/tools/python_secrets_test.go
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): docker host registry with vault secrets and per-kind dialers" -m "Also blocks the docker_host_ vault prefix for Python tools." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task A9: DOX closeout and regression sweep

**Files:**
- Create: `internal/dockerhost/AGENTS.md`
- Modify: `AGENTS.md` (Child DOX index row)
- Modify: `internal/dockerutil/AGENTS.md`
- Modify: `internal/invasion/AGENTS.md`
- Modify: `internal/tools/AGENTS.md`

The repository's DOX rule: a new durable boundary gets its own guide and an index row, and changed contracts update the nearest owner. Before editing, re-read the chain (root `AGENTS.md`, then each guide below) in this session.

- [ ] **Step 1: Create the guide for the new package**

Create `internal/dockerhost/AGENTS.md`:

````markdown
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
````

- [ ] **Step 2: Add the index row to the root guide**

In `AGENTS.md`, in the Child DOX index table:

old_string:

```md
| Docker Engine | `internal/dockerutil/AGENTS.md` | Transport/API negotiation, mutation retry, streaming cancellation. |
```

new_string:

```md
| Docker Engine | `internal/dockerutil/AGENTS.md` | Transport/API negotiation, mutation retry, streaming cancellation. |
| Docker Manager hosts | `internal/dockerhost/AGENTS.md` | Host registry, vault secrets, TLS/SSH transport shared with Invasion, Engine reads and container actions behind `/api/docker-manager`. |
```

- [ ] **Step 3: Record `NewClientWithDialer` in the dockerutil guide**

In `internal/dockerutil/AGENTS.md`:

old_string:

```md
- `NewVersionTransportWithProbeTimeout` gives transports with slow connection setup (an Engine socket reached through SSH) a longer `/version` probe budget; `NewVersionTransport` keeps 5 s.
```

new_string:

```md
- `NewVersionTransportWithProbeTimeout` gives transports with slow connection setup (an Engine socket reached through SSH) a longer `/version` probe budget; `NewVersionTransport` keeps 5 s.
- `NewClientWithDialer` builds a pooled client over a caller-supplied dial function (TLS with client certificates, SSH tunnels) for the Docker Manager's hosts. Requests keep the `http://docker/v1.45/` URL for every kind, idle connections are reaped after a minute, `probeTimeout <= 0` keeps the 5 s probe and a nil dial function fails the request instead of panicking. Host records and secrets belong to `../dockerhost/AGENTS.md`.
```

- [ ] **Step 4: Record the delegation in the Invasion guide**

In `internal/invasion/AGENTS.md`:

old_string:

```md
  budget plus the socket open and `/version`); other transports keep the default probe.
```

new_string:

```md
  budget plus the socket open and `/version`); other transports keep the default probe.
- The Docker TLS configuration (`dockerTLSClientConfig`, with `DockerTLSMaterial` as a type alias) and the SSH Engine dialer (`dialDockerEngineOverSSH`) are thin wrappers over `internal/dockerhost/transport`, which the Docker Manager shares. Change TLS or tunnel behaviour there; the invasion names, the `dockerSSHSocketOpenTimeout` test hook (passed in as the socket-open budget) and every error text stay as they are.
```

- [ ] **Step 5: Record the shared projection in the tools guide**

In `internal/tools/AGENTS.md`, insert the new bullet in front of the existing bullet that starts like this:

old_string:

```md
- The agent docker tool must hide and block inspect, lifecycle, log, exec, and copy access to the compose app container `aurago` (including compose-project prefixed replicas).
```

new_string:

```md
- `docker_inspect_export.go` shares the inspect projection: `dockerInspectProjection` builds the trimmed, redacted document for `DockerInspectContainer` and for `ProjectDockerInspect`, which `internal/dockerhost` applies to the Engines of Docker Manager hosts; `RedactDockerLabels` and `RedactDockerMountSource` apply the label and bind-source rules to its container, image and volume lists. Keep them identical to the admin inspect (`TestProjectDockerInspectMatchesTheAdminInspect`); `tools` must never import `dockerhost`.
- The agent docker tool must hide and block inspect, lifecycle, log, exec, and copy access to the compose app container `aurago` (including compose-project prefixed replicas).
```

- [ ] **Step 6: Regression sweep over everything Part A touched**

```powershell
gofmt -l internal/dockerhost internal/dockerutil internal/invasion internal/tools
go vet ./internal/dockerhost/... ./internal/dockerutil/ ./internal/invasion/ ./internal/tools/ ./internal/server/
go test ./internal/dockerhost/... ./internal/dockerutil/... -count=1
go test ./internal/invasion/ -run 'Docker|TLS' -count=1
go test ./internal/tools/ -run 'TestDockerInspect|TestProjectDockerInspect|TestRedactDocker|TestIsPythonAccessibleSecret|TestResolveVaultSecrets' -count=1
git diff --stat main...HEAD -- 'internal/invasion/*_test.go'
```

Expected: `gofmt -l` prints nothing; `go vet` prints nothing; every `go test` prints `ok`; the last command prints nothing (no Invasion test file was edited anywhere in Part A).

- [ ] **Step 7: Graph check and commit**

```powershell
git add internal/dockerhost/AGENTS.md AGENTS.md internal/dockerutil/AGENTS.md internal/invasion/AGENTS.md internal/tools/AGENTS.md
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "docs(docker-manager): DOX guide for dockerhost and updated owner guides" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

# Part B: server API (`internal/server`)

## Part B: server API (`internal/server`)

Part B turns the registry from part A into the HTTP API of the contract: server wiring, the `/api/docker-manager/`
router, host management, Docker nest import, and the container, image, volume and network routes. Everything is
Go in `internal/server` (plus one route-manifest line in `internal/audit`). Paths are repo-relative; run every
command from the worktree root `.worktrees/docker-manager`.

### Part B notes

**Prerequisite.** Part A is merged in the worktree: `internal/dockerhost` (registry, store, `ValidateHost`,
`ClassifyError`, engine reads and actions), `internal/dockerhost/transport` (`TLSMaterial`) and
`dockerutil.NewClientWithDialer`. Part B calls them exactly as the contract lists them. Quick preflight:

```powershell
go build ./internal/dockerhost/... ./internal/dockerutil/
```

Expected: no output.

**How this part was checked.** Every code block below was compiled and its tests were run (`go test -overlay`,
repository untouched) against the verified code of part A (not a stand-in), one task state at a time: all six task
states pass, `go vet ./internal/server` is clean, `gofmt -l` is clean, and a full `go test ./internal/server` run
passes except `TestGetDesktopServiceReusesCanonicalConfigWithRelativeDefaults`, a known environmental failure (Go temp
dir on `D:` while the repo is on `C:`, so `filepath.Rel` fails; unrelated to Docker). The first `go test
./internal/server` of a task compiles the whole server test binary (about a minute); later runs are quick.

**Files and sizes.** `docker_manager_api.go` (router, envelope, errors), `docker_manager_service.go` (store open and
close, local endpoint, vault hand-over), `docker_manager_status.go` (background status refresh), `docker_manager_hosts.go`,
`docker_manager_nests.go`, `docker_manager_containers.go`, `docker_manager_resources.go`; each under 260 lines. Tests
sit next to them (`*_test.go`), sharing one fixture, `docker_manager_fixture_test.go`. Tasks B1 and B2 edit existing
symbols (`Server`, `Start`, `registerToolAPIRoutes`, `RouteContractManifest`) and start with an impact step; B3-B6
only replace the `route` function that B2 creates (a new symbol), so they have none.

**Auth, confirmed.** `/api/docker-manager/…` needs no auth code of its own. The generic `authMiddleware`
(`internal/server/auth.go:921`) admits a bearer token (branch `994-1003`) or a cookie session (branch `1005-1019`, with
the CSRF origin check on non-safe methods); `validRouteBearer` (`auth.go:1161-1197`) sends every path that is not
`/api/desktop/…`, `/api/go2rtc/…` or in `isAdminProtectedPath` to the final `tm.Validate(token, "admin")` (line 1195),
which is how `/api/containers` is protected too.
The new tests pin exactly that (`TestDockerManagerRoutesKeepAdminScopeThroughAuthMiddleware`,
`TestDockerManagerRoutesStayInTheAdminBearerCatchAll`, copies of the container ones). Handlers never answer 401 (the
shared fetch wrapper treats 401 as an expired login).

**Audit manifest.** `TestRouteContractManifestCoversRegisteredServerRoutes`
(`internal/audit/audit_manifests_test.go:92`) scans `internal/server` for literal `mux.Handle(Func)("…")` routes and
fails for a route no `RouteContractManifest()` pattern covers. The catch-all `/api/` entry
(`internal/audit/audit_manifests.go:183`) would cover the new route, so the test would pass without a new entry; Task
B2 still adds an explicit `/api/docker-manager/` `session-admin` entry (next to `/api/containers`, line 165) and
pins it with an assertion beside the `/api/containers` one (test line 110), so the route cannot silently drop to the
weaker `session` catch-all.

**Deviations from the contract (names and shapes kept).**

1. Extra error codes for requests that reach no feature route: `docker_hosts_unavailable` (503, `docker_manager.db`
   could not be opened, `Server.DockerHosts` is nil), `not_found` (404, unknown route), `method_not_allowed` (405 with
   an `Allow` header), `bad_request` (400, malformed JSON body or an unsafe container reference),
   `internal_error` (500, logged, generic message).
2. `GET /hosts` never waits for a host: it answers from the cached `Registry.Status` and starts one background
   `Registry.Test` (which can block up to 30 s) per host that was never tested (state `unknown`, zero `CheckedAt`) or
   whose result is older than 30 s, so the status dots fill in over the next polls without the UI calling the test
   route. `POST /hosts/{id}/test` ("Retry") stays the explicit, synchronous refresh and answers 200 even when the host
   is down (`host.status.state` is `error`); only the container, image, volume, network and overview routes answer 502
   `host_unreachable`.
3. `POST /hosts/test` (draft) accepts an optional `"id"` in the body and passes it as `Host.ID`: `TestDraft` then takes
   the secrets the draft leaves out from that stored host's vault entries, so "Test connection" works while editing
   without retyping a key. An invalid draft is a 400 `invalid_host` with the field (`TestDraft` alone would answer a
   200 error status).
4. HostWrite details: `sort` is optional. The registry normalises and validates (empty `stacks_dir` /
   `host_socket_path` get the defaults, an empty `color` takes the next palette entry, names are unique ignoring case,
   "Local" is reserved, an SSH host needs a secret, TLS material must be usable) and the API maps its
   `*ValidationError` to `invalid_host` with `field` (name, kind, address, port, stacks_dir, host_socket_path, color,
   ssh_user, ssh_secret, tls). On `PUT`, empty paths and an absent `sort` keep the stored values (the registry would
   reset paths to the defaults). Absent or blank secret members keep the stored ones; `clear_secret` empties both first.
5. `docker_error` answers carry the Engine's one-line message (sanitised, 512 bytes); no extra members.
6. Usage counts: when the container list fails, the image, volume and network lists are returned without
   `Annotate` (images keep `containers: -1`, volumes `used_by: []`, networks `containers: 0`).
7. Local protection flags reuse `adminContainerListJSON` (the Containers page's own computation) and read the result
   back by 12-character id, so the two pages can never disagree. Cost: one extra `/containers/json` request per
   local list. `containers_protection.go` and `containers_handlers.go` are not edited.
8. Remote `DELETE` finds the container in the host's container list (name, full id or id prefix) to read its names
   and labels for `firstContainerOwner`; an unknown reference keeps the reserved-name check and the Engine answers
   404. One extra list request. Stop, kill and the other actions never ask for confirmation (UI-only, as today).
9. Nest import: `docker_local` is refused with 400 `nest_not_docker` ("this nest uses the local Docker engine, which
   is already the Local host"), as are non-Docker deploy methods; `docker_remote` becomes `tcp`/`tls`/`mtls` by
   `DockerTLS`; `docker_ssh` becomes `ssh` with `ssh_user = nest.Username`. The TLS bundle is read with the existing
   `(*Server).loadNestDockerTLS`; the SSH secret from `nest.VaultSecretID`. A missing vault entry copies nothing, so an
   SSH nest without a stored secret answers 400 `invalid_host` (`ssh_secret`), and importing a nest twice answers 400
   `invalid_host` (`name`).
10. The store opens at startup unconditionally (a small SQLite file; Docker can be switched on later without a
    restart). `Registry.Close()` only closes idle Engine connections, so `Server` keeps `dockerHostStore` and
    `closeDockerHosts` closes both. The vault is passed to the registry as `*security.Vault` (it satisfies
    `dockerhost.SecretVault` as it is); a nil vault stays a nil interface so the registry's own nil checks work.

**Part A facts this part relies on** (all verified against the finished part A):

- Errors: `*dockerhost.ValidationError` (`Field`, `Message`), `ErrHostNotFound`, `ErrLocalHostFixed`,
  `ErrInvalidAction` (400 `invalid_action`), `ErrInvalidContainerRef` (400 `bad_request`). On container and resource
  calls `dockerhost.StatusOf(err) == 404` is `container_not_found`, any other Engine status `docker_error`, and status 0
  (nothing answered, or `Registry.Client` could not build a client) `host_unreachable` with
  `reason = dockerhost.ClassifyError(err)`.
- `Registry.Create/Update` normalise, validate and check the secrets before writing anything; `Update` and `Delete`
  return `ErrHostNotFound` for an unknown id and `ErrLocalHostFixed` for `local`; `SecretSet` is cached;
  `Registry.Test` and `TestDraft` are bounded by 30 s and `Test` of an unknown id caches nothing.
- The container list returns `[]` (never null) for names, ports, mounts and networks; `ListImages` starts every image at
  `containers: -1`; `Annotate` ignores a nil container list. `InspectContainer` returns the trimmed admin-inspect shape
  without a `status` key; the API wraps it as `{"status":"ok","inspect":…}`.
- `dockerhost` imports `internal/tools` (for the inspect projection), so `internal/tools` must never import
  `dockerhost`; the server package already imports both.

**For part C.** Shapes and codes are exactly the contract table plus the additions above. Handlers never return 401.
`GET /hosts` statuses are cached and refresh in the background, so poll it (it is cheap) rather than testing every
host from the UI. An unsafe container reference answers 400 `bad_request`. Protected removal is two requests:
`DELETE …/containers/{cid}` answers 409 `container_protected_confirmation_required` with `owner` and `message`; the
confirmed repeat adds `?confirm=protected` (and `&force=1` when asked).

### Task B1: Server wiring — registry opened at startup, closed at shutdown

**Files:**
- Create: `internal/server/docker_manager_service.go` (store open and close, local endpoint, vault hand-over)
- Create: `internal/server/docker_manager_status.go`
- Test: `internal/server/docker_manager_service_test.go`
- Modify: `internal/server/server.go` (import block line 40, `Server` struct after line 348, `Start` line 1094 and the shutdown closure line 957)

`Server` is the hub struct of the package and `Start` its entry point, so GitNexus will flag them as high blast
radius; the change is purely additive (three fields, two lines in `Start`), no existing behaviour moves.

- [ ] **Step 1: Impact analysis before editing `Server` and `Start`**

```powershell
npx gitnexus impact "Server" --direction upstream --repo AuraGo --file internal/server/server.go --summary-only
npx gitnexus impact "Start" --direction upstream --repo AuraGo --file internal/server/server.go --summary-only
```

Expected: `Server`, disambiguated by file, reports a small LOW set (about nine resolved dependants; an unfiltered
name query is ambiguous and reports a hub-sized, CRITICAL radius, so treat `Server` as a hub either way), and `Start`
reports `risk: UNKNOWN` with no resolved callers. Treat `UNKNOWN` as unresolved and confirm with a text search:

```powershell
git grep -n "server\.Start(" -- "*.go"
```

Expected: `cmd/aurago/main.go:1273` is the only production caller (the other two hits are a comment in `server.go` and
an unrelated `httptest` server in `internal/testutil`). The change is additive; the commit message says so.

- [ ] **Step 2: Write the failing test**

Create `internal/server/docker_manager_service_test.go`:

```go
package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitDockerHostsOpensTheStoreInTheDataDir(t *testing.T) {
	s := testContainerServer(true, false) // no vault: hosts without secrets still work
	dir := t.TempDir()
	s.Cfg.Directories.DataDir = dir
	s.Cfg.Docker.Host = "tcp://10.0.0.5:2375"

	s.initDockerHosts()
	t.Cleanup(s.closeDockerHosts)
	if s.DockerHosts == nil {
		t.Fatal("initDockerHosts left DockerHosts nil")
	}
	if _, err := os.Stat(filepath.Join(dir, "docker_manager.db")); err != nil {
		t.Fatalf("docker_manager.db is not in the data directory: %v", err)
	}
	hosts, err := s.DockerHosts.Hosts(context.Background())
	if err != nil || len(hosts) != 1 || hosts[0].ID != "local" || !strings.Contains(hosts[0].Address, "10.0.0.5") {
		t.Fatalf("hosts = %+v, %v; want only the local host following docker.host", hosts, err)
	}
	if s.DockerHosts.SecretSet("local") {
		t.Fatal("the local host reports a secret")
	}

	// docker.host is read on every use, so a config reload needs no restart.
	s.CfgMu.Lock()
	s.Cfg.Docker.Host = "tcp://10.0.0.9:2375"
	s.CfgMu.Unlock()
	hosts, err = s.DockerHosts.Hosts(context.Background())
	if err != nil || !strings.Contains(hosts[0].Address, "10.0.0.9") {
		t.Fatalf("local host after a docker.host change = %+v, %v", hosts, err)
	}
}

func TestInitDockerHostsLeavesTheRegistryNilWhenTheStoreCannotOpen(t *testing.T) {
	s := testContainerServer(true, false)
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.Cfg.Directories.DataDir = blocker // MkdirAll under a file fails

	s.initDockerHosts()
	t.Cleanup(s.closeDockerHosts)
	if s.DockerHosts != nil {
		t.Fatal("DockerHosts is set although the store directory is unusable")
	}
}
```

- [ ] **Step 3: Run the test (expected FAIL)**

```powershell
go test ./internal/server -run 'TestInitDockerHosts' -count=1
```

Expected: FAIL, build error (`s.initDockerHosts undefined`, `s.DockerHosts undefined`, `s.closeDockerHosts undefined`).

- [ ] **Step 4: Implement the service and the status refresher**

Create `internal/server/docker_manager_service.go`:

```go
package server

import (
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/config"
	"aurago/internal/dockerhost"
	"aurago/internal/security"
)

// dockerManagerDBFile is the Docker Manager's SQLite store inside the data directory.
const dockerManagerDBFile = "docker_manager.db"

// The server vault is the registry's secret store as it is: a missing key
// answers security.ErrSecretNotFound.
var _ dockerhost.SecretVault = (*security.Vault)(nil)

// dockerManagerDBPath places docker_manager.db in the configured data directory.
func dockerManagerDBPath(cfg *config.Config) string {
	dir := "data"
	if cfg != nil && strings.TrimSpace(cfg.Directories.DataDir) != "" {
		dir = cfg.Directories.DataDir
	}
	return filepath.Join(dir, dockerManagerDBFile)
}

// dockerLocalEndpoint is the dockerhost.LocalEndpointFunc: the configured
// docker.host ("" means the platform default), read under the config lock
// exactly as the Containers API does.
func (s *Server) dockerLocalEndpoint() string {
	cfg, _, _ := containerDockerConfig(s)
	return cfg.Host
}

// initDockerHosts opens docker_manager.db and builds the host registry. A
// failure is logged and leaves DockerHosts nil: the Docker Manager routes then
// answer 503 docker_hosts_unavailable while every other feature keeps running.
func (s *Server) initDockerHosts() {
	if s.Cfg == nil || s.DockerHosts != nil {
		return
	}
	path := dockerManagerDBPath(s.Cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		s.logDockerManager("Docker host store directory is not usable", err)
		return
	}
	store, err := dockerhost.OpenStore(path)
	if err != nil {
		s.logDockerManager("Docker host store could not be opened", err)
		return
	}
	// A nil *security.Vault must not become a non-nil interface: the registry
	// tests its vault against nil (hosts without secrets then still work).
	var vault dockerhost.SecretVault
	if s.Vault != nil {
		vault = s.Vault
	}
	s.dockerHostStore = store
	s.DockerHosts = dockerhost.NewRegistry(store, vault, s.dockerLocalEndpoint)
}

// closeDockerHosts stops the background status checks, releases the cached
// Engine connections and closes the store. It is safe to call without a
// registry.
func (s *Server) closeDockerHosts() {
	s.dockerStatus.close()
	if s.DockerHosts != nil {
		s.DockerHosts.Close()
	}
	if s.dockerHostStore != nil {
		_ = s.dockerHostStore.Close()
	}
}

func (s *Server) logDockerManager(message string, err error) {
	if s.Logger != nil {
		s.Logger.Warn("[DockerManager] "+message, "error", err)
	}
}
```

Create `internal/server/docker_manager_status.go`:

```go
package server

import (
	"context"
	"sync"
	"time"

	"aurago/internal/dockerhost"
)

const (
	// dockerStatusStaleAfter is the age after which GET /hosts starts a new
	// background connection test for a host.
	dockerStatusStaleAfter = 30 * time.Second
	// dockerStatusTestTimeout bounds one background test; Registry.Test limits
	// itself to 30 s.
	dockerStatusTestTimeout = 35 * time.Second
)

// dockerStatusRefresher keeps the cached host status fresh without making
// GET /hosts wait for an unreachable host: the list answers with the cached
// status at once and starts at most one background Registry.Test per stale
// host. close cancels the running tests and waits for them.
type dockerStatusRefresher struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	inflight map[string]bool
	wg       sync.WaitGroup
}

// context returns the refresher's own context; callers hold mu.
func (f *dockerStatusRefresher) context() context.Context {
	if f.ctx == nil {
		f.ctx, f.cancel = context.WithCancel(context.Background())
	}
	return f.ctx
}

// refreshStale starts a background test for every host that was never tested
// (state unknown, zero CheckedAt) or whose cached status is older than
// dockerStatusStaleAfter, and has no test running.
func (f *dockerStatusRefresher) refreshStale(reg *dockerhost.Registry, hosts []dockerhost.Host, now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	base := f.context()
	if base.Err() != nil {
		return
	}
	for _, host := range hosts {
		status := reg.Status(host.ID)
		if status.State != dockerhost.StateUnknown && now.Sub(status.CheckedAt) < dockerStatusStaleAfter {
			continue
		}
		if f.inflight == nil {
			f.inflight = map[string]bool{}
		}
		if f.inflight[host.ID] {
			continue
		}
		f.inflight[host.ID] = true
		f.wg.Add(1)
		go func(id string) {
			defer f.wg.Done()
			ctx, cancel := context.WithTimeout(base, dockerStatusTestTimeout)
			defer cancel()
			reg.Test(ctx, id)
			f.mu.Lock()
			delete(f.inflight, id)
			f.mu.Unlock()
		}(host.ID)
	}
}

// close cancels the running tests, waits for them and refuses new ones.
func (f *dockerStatusRefresher) close() {
	f.mu.Lock()
	f.context()
	f.cancel()
	f.mu.Unlock()
	f.wg.Wait()
}
```

- [ ] **Step 5: Wire the registry into `Server` and `Start`**

Four edits in `internal/server/server.go`.

(a) Import block, line 39-40 — add `dockerhost` between `discord` and `dockerutil`.

Old:

```go
	"aurago/internal/discord"
	"aurago/internal/dockerutil"
```

New:

```go
	"aurago/internal/discord"
	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
```

(b) `Server` struct, lines 348-349 — a separate field group, so gofmt alignment of the neighbouring blocks is untouched.

Old:

```go
	videoStudioConfigRevoking atomic.Bool
	// IsFirstStart is true if core_memory.md was just freshly created (no prior data).
```

New:

```go
	videoStudioConfigRevoking atomic.Bool

	// Docker Manager: the host registry and its SQLite store (docker_manager.db).
	// DockerHosts stays nil when the store could not be opened.
	DockerHosts     *dockerhost.Registry
	dockerHostStore *dockerhost.Store
	dockerStatus    dockerStatusRefresher

	// IsFirstStart is true if core_memory.md was just freshly created (no prior data).
```

(c) `Start`, next to the other per-feature initialisers (line 1094).

Old:

```go
	s.initNewspaper()
	s.initMailer(serverCtx)
```

New:

```go
	s.initNewspaper()
	s.initDockerHosts()
	s.initMailer(serverCtx)
```

(d) `Start`, the deferred shutdown closure (lines 957-959) — close next to the other per-feature stores.

Old:

```go
		if s.Newspaper != nil {
			_ = s.Newspaper.Close()
		}
```

New:

```go
		if s.Newspaper != nil {
			_ = s.Newspaper.Close()
		}
		s.closeDockerHosts()
```

- [ ] **Step 6: Run the tests and the format check (expected PASS)**

```powershell
go test ./internal/server -run 'TestInitDockerHosts' -count=1
gofmt -l internal/server
go vet ./internal/server
```

Expected: `ok  aurago/internal/server`; `gofmt -l` and `go vet` print nothing.

- [ ] **Step 7: Graph check and commit**

```powershell
git add internal/server/docker_manager_service.go internal/server/docker_manager_status.go internal/server/docker_manager_service_test.go internal/server/server.go
npx gitnexus detect-changes --scope staged --repo AuraGo
```

Expected: changed symbols limited to the new service/status files and `Server`/`Start`; if the result says
`partial` or `truncated`, re-run with `--limit 200`.

```powershell
git commit -m "feat(docker-manager): open the Docker host registry with the server" -m "Additive: three Server fields, one init call and one close in Start." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B2: `/api/docker-manager/` router skeleton, admin gate, audit entry

**Files:**
- Create: `internal/server/docker_manager_api.go`
- Create (shared fixture for B2-B6): `internal/server/docker_manager_fixture_test.go`
- Test: `internal/server/docker_manager_api_test.go`, `internal/server/docker_manager_auth_test.go`
- Modify: `internal/server/server_routes_tools.go` (after line 147)
- Modify: `internal/audit/audit_manifests.go` (after line 165)
- Modify: `internal/audit/audit_manifests_test.go` (after line 112)

This task delivers the envelope, error mapping, the Docker-enabled and registry gate, the route table (still empty:
every path answers 404 `not_found`), the mux registration, and the auth and audit pins. Tasks B3-B6 add the routes.

- [ ] **Step 1: Impact analysis before editing `registerToolAPIRoutes` and `RouteContractManifest`**

```powershell
npx gitnexus impact "registerToolAPIRoutes" --direction upstream --repo AuraGo --file internal/server/server_routes_tools.go --summary-only
npx gitnexus impact "RouteContractManifest" --direction upstream --repo AuraGo --file internal/audit/audit_manifests.go --summary-only
git grep -n "RouteContractManifest()" -- "*.go"
```

Expected: `registerToolAPIRoutes` LOW (the server's route setup and the auth-chain tests); `RouteContractManifest`
`risk: UNKNOWN` with no resolved callers, and the text search finds only `internal/audit/audit_manifests_test.go:95`
and `internal/audit/local_wikipedia_manifest_test.go:22,32`. Both edits add one line.

- [ ] **Step 2: Write the failing tests**

Create the shared fixture `internal/server/docker_manager_fixture_test.go` (a fake Engine that records calls, a
server with Docker enabled and the registry open, request helpers):

```go
package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"aurago/internal/security"
	"aurago/internal/tools"
)

// dmFakeEngine stands in for a Docker Engine: it answers the handful of
// endpoints the Docker Manager reads and records every call as
// "METHOD /path" (API version prefix and query stripped).
type dmFakeEngine struct {
	t          *testing.T
	srv        *httptest.Server
	mu         sync.Mutex
	calls      []string
	containers string
	forced     map[string]int
}

var dmEngineVersionPrefix = regexp.MustCompile(`^/v[0-9]+\.[0-9]+`)

func newDMFakeEngine(t *testing.T, containers string) *dmFakeEngine {
	t.Helper()
	e := &dmFakeEngine{t: t, containers: containers, forced: map[string]int{}}
	e.srv = httptest.NewServer(http.HandlerFunc(e.serve))
	t.Cleanup(e.srv.Close)
	return e
}

func (e *dmFakeEngine) port() int { return e.srv.Listener.Addr().(*net.TCPAddr).Port }

func (e *dmFakeEngine) endpoint() string { return "tcp://" + e.srv.Listener.Addr().String() }

func (e *dmFakeEngine) force(call string, status int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.forced[call] = status
}

func (e *dmFakeEngine) called(call string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, got := range e.calls {
		if got == call {
			return true
		}
	}
	return false
}

func (e *dmFakeEngine) serve(w http.ResponseWriter, r *http.Request) {
	path := dmEngineVersionPrefix.ReplaceAllString(r.URL.Path, "")
	call := r.Method + " " + path
	e.mu.Lock()
	e.calls = append(e.calls, call)
	status, forced := e.forced[call]
	containers := e.containers
	e.mu.Unlock()
	notFound := func() {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"No such container: missing"}`))
	}
	switch {
	case forced:
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"message":"forced answer"}`))
	case path == "/version":
		_, _ = w.Write([]byte(`{"Version":"27.0.1","ApiVersion":"1.45","MinAPIVersion":"1.25","Os":"linux","Arch":"amd64"}`))
	case path == "/info":
		_, _ = w.Write([]byte(`{"Name":"fakehost","ServerVersion":"27.0.1","OperatingSystem":"Debian","OSType":"linux","KernelVersion":"6.1","Architecture":"x86_64","DockerRootDir":"/var/lib/docker","NCPU":4,"MemTotal":8000000000,"Containers":2,"ContainersRunning":1,"ContainersPaused":0,"ContainersStopped":1,"Images":3}`))
	case path == "/containers/json":
		_, _ = w.Write([]byte(containers))
	case strings.HasPrefix(path, "/containers/missing"):
		notFound()
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/logs"):
		name := strings.Split(path, "/")[2]
		payload := []byte("2026-10-26T10:00:00Z hello from " + name + "\n")
		header := make([]byte, 8)
		header[0] = 1
		binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
		_, _ = io.Copy(w, bytes.NewReader(append(header, payload...)))
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/containers/") && strings.HasSuffix(path, "/json"):
		name := strings.Split(path, "/")[2]
		_, _ = w.Write([]byte(`{"Id":"` + strings.Repeat("a", 64) + `","Name":"/` + name + `","Config":{"Tty":false,"Env":["PATH=/bin"],"Labels":{}},"State":{"Status":"running"}}`))
	case strings.HasPrefix(path, "/containers/"):
		w.WriteHeader(http.StatusNoContent)
	case path == "/images/json":
		_, _ = w.Write([]byte(`[{"Id":"sha256:img1","RepoTags":["nginx:1.27"],"Size":1000,"Created":1700000000},{"Id":"sha256:img2","RepoTags":["ruview:1"],"Size":2000,"Created":1700000100},{"Id":"sha256:old","RepoTags":null,"Size":300,"Created":1600000000}]`))
	case path == "/volumes":
		_, _ = w.Write([]byte(`{"Volumes":[{"Name":"data","Driver":"local","Mountpoint":"/var/lib/docker/volumes/data/_data","CreatedAt":"2026-10-01T00:00:00Z","Labels":null},{"Name":"idle","Driver":"local","Mountpoint":"/var/lib/docker/volumes/idle/_data","CreatedAt":"2026-10-02T00:00:00Z","Labels":null}],"Warnings":null}`))
	case path == "/networks":
		_, _ = w.Write([]byte(`[{"Id":"n1","Name":"bridge","Driver":"bridge","Scope":"local","IPAM":{"Config":[{"Subnet":"172.17.0.0/16"}]}},{"Id":"n2","Name":"appnet","Driver":"bridge","Scope":"local","IPAM":{"Config":[{"Subnet":"172.20.0.0/16"}]}},{"Id":"n3","Name":"idle","Driver":"bridge","Scope":"local","IPAM":{"Config":[]}}]`))
	default:
		http.NotFound(w, r)
	}
}

// dmContainerJSON builds one /containers/json entry.
func dmContainerJSON(id byte, name, imageID, state, network string, labels map[string]string, mounts []map[string]any) map[string]any {
	if labels == nil {
		labels = map[string]string{}
	}
	if mounts == nil {
		mounts = []map[string]any{}
	}
	return map[string]any{
		"Id": strings.Repeat(string(id), 64), "Names": []string{"/" + name}, "Image": name + ":1", "ImageID": imageID,
		"State": state, "Status": state, "Created": 1700000000, "Labels": labels,
		"Ports":  []map[string]any{{"IP": "0.0.0.0", "PrivatePort": 80, "PublicPort": 8080, "Type": "tcp"}},
		"Mounts": mounts, "HostConfig": map[string]any{"NetworkMode": "default"},
		"NetworkSettings": map[string]any{"Networks": map[string]any{network: map[string]any{"IPAddress": "172.17.0.2"}}},
	}
}

func dmContainersJSON(t *testing.T, entries ...map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// dmFixture is a server with Docker enabled, the host registry open, a fake
// local engine (docker.host) and a fake remote engine for tcp hosts.
type dmFixture struct {
	t       *testing.T
	s       *Server
	local   *dmFakeEngine
	remote  *dmFakeEngine
	lastRaw string
}

func newDMFixture(t *testing.T) *dmFixture {
	t.Helper()
	local := newDMFakeEngine(t, dmContainersJSON(t,
		dmContainerJSON('a', "web1", "sha256:img1", "running", "bridge",
			map[string]string{"com.docker.compose.project": "shop", "com.docker.compose.service": "web"},
			[]map[string]any{{"Type": "volume", "Name": "data", "Source": "/var/lib/docker/volumes/data/_data", "Destination": "/data"}}),
		dmContainerJSON('b', "rv", "sha256:img2", "exited", "appnet", map[string]string{"aurago.managed": "ruview"}, nil),
	))
	remote := newDMFakeEngine(t, dmContainersJSON(t,
		dmContainerJSON('c', "blog", "sha256:img1", "running", "bridge", nil, nil),
		dmContainerJSON('d', "cams", "sha256:img2", "running", "bridge", map[string]string{"aurago.managed": "ruview"}, nil),
		dmContainerJSON('e', "web1", "sha256:img1", "running", "bridge", nil, nil),
	))
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)

	dir := t.TempDir()
	vault, err := security.NewVault(strings.Repeat("d", 64), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	s := testContainerServer(true, false)
	s.Cfg.Docker.Host = local.endpoint()
	s.Cfg.Directories.DataDir = dir
	s.Vault = vault
	s.initDockerHosts()
	t.Cleanup(s.closeDockerHosts)
	if s.DockerHosts == nil {
		t.Fatal("initDockerHosts left DockerHosts nil")
	}
	return &dmFixture{t: t, s: s, local: local, remote: remote}
}

// do calls the Docker Manager handler directly and decodes the JSON answer.
func (f *dmFixture) do(method, path string, body any) (int, map[string]any) {
	f.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			f.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, "/api/docker-manager"+path, reader)
	rec := httptest.NewRecorder()
	handleDockerManager(f.s)(rec, req)
	f.lastRaw = rec.Body.String()
	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		f.t.Fatalf("%s %s: answer %q is not JSON: %v", method, path, f.lastRaw, err)
	}
	return rec.Code, decoded
}

// setDocker changes the Docker switches under the config lock.
func (f *dmFixture) setDocker(enabled, readOnly bool) {
	f.s.CfgMu.Lock()
	defer f.s.CfgMu.Unlock()
	f.s.Cfg.Docker.Enabled = enabled
	f.s.Cfg.Docker.ReadOnly = readOnly
}

// addRemote registers the fake remote engine as a plain TCP host and returns its id.
func (f *dmFixture) addRemote(name string) string {
	f.t.Helper()
	code, body := f.do(http.MethodPost, "/hosts", map[string]any{"name": name, "kind": "tcp", "address": "127.0.0.1", "port": f.remote.port()})
	if code != http.StatusCreated {
		f.t.Fatalf("create remote host: %d %s", code, f.lastRaw)
	}
	return dmString(f.t, body["host"], "id")
}

func dmString(t *testing.T, v any, key string) string {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%v is not an object", v)
	}
	s, ok := m[key].(string)
	if !ok {
		t.Fatalf("member %q of %v is not a string", key, m)
	}
	return s
}

func dmObject(t *testing.T, v any, key string) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%v is not an object", v)
	}
	child, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("member %q of %v is not an object", key, m)
	}
	return child
}

func dmArray(t *testing.T, v any, key string) []map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%v is not an object", v)
	}
	items, ok := m[key].([]any)
	if !ok {
		t.Fatalf("member %q of %v is not an array", key, m)
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("element %v of %q is not an object", item, key)
		}
		out = append(out, obj)
	}
	return out
}

func dmFind(t *testing.T, items []map[string]any, key, value string) map[string]any {
	t.Helper()
	for _, item := range items {
		if item[key] == value {
			return item
		}
	}
	t.Fatalf("no element with %s=%q in %v", key, value, items)
	return nil
}

func dmExpectError(t *testing.T, f *dmFixture, code int, body map[string]any, wantStatus int, wantCode string) {
	t.Helper()
	if code != wantStatus || body["status"] != "error" || body["code"] != wantCode {
		t.Fatalf("answer = %d %s, want %d error %q", code, f.lastRaw, wantStatus, wantCode)
	}
}
```

Create `internal/server/docker_manager_api_test.go`:

```go
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/dockerhost"
)

func TestDockerManagerEngineFailureMapping(t *testing.T) {
	engine := func(status int) error {
		return &dockerhost.StatusError{Status: status, Err: fmt.Errorf("Docker API returned %d: engine said no", status)}
	}
	for _, tc := range []struct {
		name     string
		err      error
		notFound string
		status   int
		code     string
	}{
		{"engine 404 with a not-found code", engine(404), "container_not_found", http.StatusNotFound, "container_not_found"},
		{"engine 404 without a code", engine(404), "", http.StatusBadGateway, "docker_error"},
		{"engine 409", engine(409), "container_not_found", http.StatusBadGateway, "docker_error"},
		{"wrapped engine status", fmt.Errorf("stop: %w", engine(500)), "", http.StatusBadGateway, "docker_error"},
		{"nothing answered", errors.New("dial tcp 10.0.0.5:2375: connect: connection refused"), "", http.StatusBadGateway, "host_unreachable"},
		{"rejected action", dockerhost.ErrInvalidAction, "", http.StatusBadRequest, "invalid_action"},
		{"rejected container reference", fmt.Errorf("inspect: %w", dockerhost.ErrInvalidContainerRef), "", http.StatusBadRequest, "bad_request"},
	} {
		rec := httptest.NewRecorder()
		(&dmRequest{w: rec}).engineFailure(tc.err, tc.notFound)
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: answer %q is not JSON: %v", tc.name, rec.Body.String(), err)
		}
		if rec.Code != tc.status || body["status"] != "error" || body["code"] != tc.code {
			t.Errorf("%s: answer = %d %s, want %d error %q", tc.name, rec.Code, rec.Body.String(), tc.status, tc.code)
		}
		if reason, _ := body["reason"].(string); tc.code == "host_unreachable" && reason == "" {
			t.Errorf("%s: answer %s carries no reason", tc.name, rec.Body.String())
		}
	}
}

func TestDockerManagerWithoutRegistryAnswers503(t *testing.T) {
	s := testContainerServer(true, false) // no initDockerHosts: the store could not be opened
	rec := httptest.NewRecorder()
	handleDockerManager(s)(rec, httptest.NewRequest(http.MethodGet, "/api/docker-manager/hosts", nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "docker_hosts_unavailable") {
		t.Fatalf("answer = %d %s, want 503 docker_hosts_unavailable", rec.Code, rec.Body.String())
	}
}

func TestDockerManagerDisabledAnswers503(t *testing.T) {
	f := newDMFixture(t)
	f.setDocker(false, false)
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/hosts"},
		{http.MethodPost, "/hosts/test"},
		{http.MethodGet, "/nests"},
		{http.MethodGet, "/hosts/local/containers"},
		{http.MethodDelete, "/hosts/local/containers/web1"},
	} {
		code, body := f.do(tc.method, tc.path, nil)
		dmExpectError(t, f, code, body, http.StatusServiceUnavailable, "docker_disabled")
	}
}

func TestDockerManagerUnknownRoutesAnswerJSON(t *testing.T) {
	f := newDMFixture(t)
	for _, path := range []string{"/", "/nope", "/hosts/local/unknown", "/hosts//containers"} {
		code, body := f.do(http.MethodGet, path, nil)
		dmExpectError(t, f, code, body, http.StatusNotFound, "not_found")
	}
}
```

Create `internal/server/docker_manager_auth_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestDockerManagerRoutesKeepAdminScopeThroughAuthMiddleware pins that the
// Docker Manager API sits behind the same gate as /api/containers: read and
// desktop:admin tokens are refused with invalid_bearer_scope, a browser session
// and an admin token reach the handler, anonymous gets 401.
func TestDockerManagerRoutesKeepAdminScopeThroughAuthMiddleware(t *testing.T) {
	_, chain, adminToken, desktopToken, readToken := newContainerAuthChain(t)

	requests := []func() *http.Request{
		func() *http.Request { return httptest.NewRequest(http.MethodGet, "/api/docker-manager/hosts", nil) },
		func() *http.Request {
			return httptest.NewRequest(http.MethodPost, "/api/docker-manager/hosts/local/containers/web1/restart", nil)
		},
		func() *http.Request {
			return httptest.NewRequest(http.MethodDelete, "/api/docker-manager/hosts/local/containers/web1", nil)
		},
	}
	for _, build := range requests {
		for _, tc := range []struct {
			name  string
			token string
			want  int
		}{
			{"read scope", readToken, http.StatusForbidden},
			{"desktop admin scope", desktopToken, http.StatusForbidden},
			// Docker is disabled in this fixture: an admitted request reaches the
			// handler's own 503 docker_disabled.
			{"admin scope", adminToken, http.StatusServiceUnavailable},
		} {
			req := build()
			req.Header.Set("Authorization", "Bearer "+tc.token)
			rec := httptest.NewRecorder()
			chain.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("%s %s with %s: status = %d, want %d; body=%s", req.Method, req.URL.Path, tc.name, rec.Code, tc.want, rec.Body.String())
			}
			if tc.want == http.StatusForbidden && !strings.Contains(rec.Body.String(), "invalid_bearer_scope") {
				t.Fatalf("%s %s with %s: body = %s, want invalid_bearer_scope", req.Method, req.URL.Path, tc.name, rec.Body.String())
			}
			if tc.want == http.StatusServiceUnavailable && !strings.Contains(rec.Body.String(), "docker_disabled") {
				t.Fatalf("%s %s with %s: body = %s, want the handler's docker_disabled", req.Method, req.URL.Path, tc.name, rec.Body.String())
			}
		}

		session := &http.Cookie{Name: sessionCookieName, Value: createSessionValue(bearerSchemeTestSessionSecret, time.Now().Add(time.Hour))}
		req := build()
		req.AddCookie(session)
		if req.Method != http.MethodGet {
			req.Header.Set("Origin", "http://example.com")
		}
		rec := httptest.NewRecorder()
		chain.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "docker_disabled") {
			t.Fatalf("%s %s with a browser session: status = %d, want 503 docker_disabled from the handler; body=%s", req.Method, req.URL.Path, rec.Code, rec.Body.String())
		}

		req = build()
		rec = httptest.NewRecorder()
		chain.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous %s %s: status = %d, want 401", req.Method, req.URL.Path, rec.Code)
		}
	}
}

// TestDockerManagerRoutesStayInTheAdminBearerCatchAll fails when a future change
// moves the Docker Manager paths into a list that would admit weaker credentials.
func TestDockerManagerRoutesStayInTheAdminBearerCatchAll(t *testing.T) {
	s, _, adminToken, desktopToken, readToken := newContainerAuthChain(t)
	tokens := map[string]string{"read": readToken, desktopScopeAdmin: desktopToken}
	for _, scope := range []string{desktopScopeRead, desktopScopeWrite, go2RTCViewScope, "cyd"} {
		raw, _, err := s.TokenManager.Create(scope, []string{scope}, nil)
		if err != nil {
			t.Fatalf("create %s token: %v", scope, err)
		}
		tokens[scope] = raw
	}

	for _, path := range []string{"/api/docker-manager/", "/api/docker-manager/hosts", "/api/docker-manager/hosts/local/containers", "/api/docker-manager/hosts/import-nest/n1"} {
		if isDesktopScopedAPIPath(path) {
			t.Fatalf("%s must not become a desktop-scoped path: desktop:read/write tokens would reach Docker", path)
		}
		if isAuthBypassed(path) {
			t.Fatalf("%s must not bypass session authentication", path)
		}
		if isAllowedWithoutPassword(path) {
			t.Fatalf("%s must not be reachable during the password lockdown", path)
		}
		if isDesktopEmbedResourcePath(path) {
			t.Fatalf("%s must not accept desktop embed tickets", path)
		}
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
			if !validRouteBearer(s, adminToken, path, method) {
				t.Fatalf("%s %s: admin token refused", method, path)
			}
			for scope, raw := range tokens {
				if validRouteBearer(s, raw, path, method) {
					t.Fatalf("%s %s: a %q token is admitted; Docker Manager routes require the admin scope", method, path, scope)
				}
			}
		}
	}
}
```

- [ ] **Step 3: Run the tests (expected FAIL)**

```powershell
go test ./internal/server -run 'TestDockerManager' -count=1
```

Expected: FAIL, build error (`undefined: handleDockerManager`, `undefined: dmRequest`).

- [ ] **Step 4: Implement the API core**

Create `internal/server/docker_manager_api.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

// ── Docker Manager API ──────────────────────────────────────────────────────
// Everything below /api/docker-manager/. Authentication is the generic
// /api/ middleware (cookie session, or a bearer token with the admin scope),
// the same path as /api/containers. Envelope: {"status":"ok", ...} or
// {"status":"error","code":"...","message":"..."}.

const (
	dockerManagerPrefix = "/api/docker-manager/"
	dmMaxBody           = 1 << 20
	dmRegistryTimeout   = 15 * time.Second
)

// Error codes. The desktop app switches on them, so they are stable.
const (
	dmCodeDisabled          = "docker_disabled"
	dmCodeReadOnly          = "read_only"
	dmCodeHostNotFound      = "host_not_found"
	dmCodeInvalidHost       = "invalid_host"
	dmCodeLocalHostFixed    = "local_host_fixed"
	dmCodeHostUnreachable   = "host_unreachable"
	dmCodeNestNotFound      = "nest_not_found"
	dmCodeNestNotDocker     = "nest_not_docker"
	dmCodeContainerNotFound = "container_not_found"
	dmCodeInvalidAction     = "invalid_action"
	dmCodeDockerError       = "docker_error"
	// Not in the contract table; they cover requests that reach no feature route.
	dmCodeUnavailable = "docker_hosts_unavailable"
	dmCodeNotFound    = "not_found"
	dmCodeMethod      = "method_not_allowed"
	dmCodeBadRequest  = "bad_request"
	dmCodeInternal    = "internal_error"
)

// dmFields are the payload members next to "status".
type dmFields map[string]any

func dmWriteOK(w http.ResponseWriter, status int, fields dmFields) {
	body := map[string]any{"status": "ok"}
	for key, value := range fields {
		body[key] = value
	}
	containerJSON(w, status, body)
}

func dmWriteError(w http.ResponseWriter, status int, code, message string, extra dmFields) {
	body := map[string]any{"status": "error", "code": code, "message": message}
	for key, value := range extra {
		body[key] = value
	}
	containerJSON(w, status, body)
}

// dmRequest is one routed request with the Docker switches read once.
type dmRequest struct {
	s        *Server
	w        http.ResponseWriter
	r        *http.Request
	readOnly bool
}

func (dm *dmRequest) registry() *dockerhost.Registry { return dm.s.DockerHosts }

func (dm *dmRequest) context(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(dm.r.Context(), timeout)
}

// allow answers 405 unless the request method is one of methods.
func (dm *dmRequest) allow(methods ...string) bool {
	for _, method := range methods {
		if dm.r.Method == method {
			return true
		}
	}
	dm.w.Header().Set("Allow", strings.Join(methods, ", "))
	dmWriteError(dm.w, http.StatusMethodNotAllowed, dmCodeMethod, "method not allowed", nil)
	return false
}

// decode reads a JSON body of at most dmMaxBody bytes.
func (dm *dmRequest) decode(dst any) bool {
	body := http.MaxBytesReader(dm.w, dm.r.Body, dmMaxBody)
	if err := json.NewDecoder(body).Decode(dst); err != nil {
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeBadRequest, "the request body is not valid JSON", nil)
		return false
	}
	return true
}

func (dm *dmRequest) notFound() {
	dmWriteError(dm.w, http.StatusNotFound, dmCodeNotFound, "unknown Docker Manager route", nil)
}

func (dm *dmRequest) readOnlyRefusal() {
	dmWriteError(dm.w, http.StatusForbidden, dmCodeReadOnly, "Docker is in read-only mode", nil)
}

// internalFailure logs the cause and answers a generic 500.
func (dm *dmRequest) internalFailure(what string, err error) {
	dm.s.logDockerManager(what, err)
	dmWriteError(dm.w, http.StatusInternalServerError, dmCodeInternal, "The Docker Manager could not complete the request", nil)
}

// host resolves a host id; an unknown id answers 404.
func (dm *dmRequest) host(ctx context.Context, id string) (dockerhost.Host, bool) {
	host, err := dm.registry().Host(ctx, id)
	if err != nil {
		dm.hostFailure(err)
		return dockerhost.Host{}, false
	}
	return host, true
}

// engine resolves a host id to the host and its Engine client.
func (dm *dmRequest) engine(ctx context.Context, id string) (dockerhost.Host, *dockerutil.Client, bool) {
	host, ok := dm.host(ctx, id)
	if !ok {
		return dockerhost.Host{}, nil, false
	}
	client, err := dm.registry().Client(ctx, id)
	if err != nil {
		dm.engineFailure(err, "")
		return host, nil, false
	}
	return host, client, true
}

// hostFailure maps registry and validation errors onto the contract codes.
func (dm *dmRequest) hostFailure(err error) {
	var invalid *dockerhost.ValidationError
	switch {
	case errors.Is(err, dockerhost.ErrHostNotFound):
		dmWriteError(dm.w, http.StatusNotFound, dmCodeHostNotFound, "Docker host not found", nil)
	case errors.Is(err, dockerhost.ErrLocalHostFixed):
		dmWriteError(dm.w, http.StatusConflict, dmCodeLocalHostFixed, dockerhost.ErrLocalHostFixed.Error(), nil)
	case errors.As(err, &invalid):
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeInvalidHost, invalid.Message, dmFields{"field": invalid.Field})
	default:
		dm.internalFailure("Docker host operation failed", err)
	}
}

// engineFailure answers an error from an Engine call or from the client of a
// host. dockerhost.StatusOf separates the cases: a rejected action or container
// reference is a 400; 404 with notFoundCode (when given) is "no such object";
// any other Engine status is docker_error; status 0 means nothing answered
// (dial, TLS, SSH, timeout or a client that could not be built) and becomes
// host_unreachable with the classified reason.
func (dm *dmRequest) engineFailure(err error, notFoundCode string) {
	message := dockerutil.SanitizeOneLine(err.Error(), 512)
	status := dockerhost.StatusOf(err)
	switch {
	case errors.Is(err, dockerhost.ErrInvalidAction):
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeInvalidAction, message, nil)
	case errors.Is(err, dockerhost.ErrInvalidContainerRef):
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeBadRequest, message, nil)
	case status == http.StatusNotFound && notFoundCode != "":
		dmWriteError(dm.w, http.StatusNotFound, notFoundCode, message, nil)
	case status > 0:
		dmWriteError(dm.w, http.StatusBadGateway, dmCodeDockerError, message, nil)
	default:
		dmWriteError(dm.w, http.StatusBadGateway, dmCodeHostUnreachable, message, dmFields{"reason": dockerhost.ClassifyError(err)})
	}
}

// handleDockerManager serves /api/docker-manager/. Docker disabled answers 503
// for every route; docker.readonly blocks only the container mutations.
func handleDockerManager(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, enabled, readOnly := containerDockerConfig(s)
		if !enabled {
			dmWriteError(w, http.StatusServiceUnavailable, dmCodeDisabled, "Docker is not enabled", nil)
			return
		}
		if s.DockerHosts == nil {
			dmWriteError(w, http.StatusServiceUnavailable, dmCodeUnavailable, "The Docker host registry is not available", nil)
			return
		}
		dm := &dmRequest{s: s, w: w, r: r, readOnly: readOnly}
		dm.route(dockerManagerSegments(r.URL.Path))
	}
}

// dockerManagerSegments splits the path below the prefix; an empty segment
// makes the whole path unroutable.
func dockerManagerSegments(path string) []string {
	rest := strings.Trim(strings.TrimPrefix(path, dockerManagerPrefix), "/")
	if rest == "" {
		return nil
	}
	segs := strings.Split(rest, "/")
	for _, seg := range segs {
		if seg == "" {
			return nil
		}
	}
	return segs
}

// route dispatches by path shape. The fixed words "test" and "import-nest"
// come before the host-id patterns; generated host ids never equal them.
func (dm *dmRequest) route(segs []string) {
	switch {
	default:
		dm.notFound()
	}
}
```

- [ ] **Step 5: Register the route**

In `internal/server/server_routes_tools.go`, next to the Containers API (lines 145-147).

Old:

```go
		mux.HandleFunc("/api/containers/", handleContainerAction(s))
```

New:

```go
		mux.HandleFunc("/api/containers/", handleContainerAction(s))

		// ── Docker Manager API ──
		mux.HandleFunc("/api/docker-manager/", handleDockerManager(s))
```

- [ ] **Step 6: Pin the audit contract (test first, expected FAIL, then the manifest line)**

In `internal/audit/audit_manifests_test.go`, inside `TestRouteContractManifestCoversRegisteredServerRoutes`, after the
`/api/containers` check (line 110-112).

Old:

```go
	if !routeContractExists(contracts, "/api/manus/", "session") {
```

New:

```go
	if !routeContractExists(contracts, "/api/docker-manager/", "session-admin") {
		t.Fatal("/api/docker-manager/ must have an explicit session-admin route contract")
	}
	if !routeContractExists(contracts, "/api/manus/", "session") {
```

```powershell
go test ./internal/audit -run 'TestRouteContractManifestCoversRegisteredServerRoutes' -count=1
```

Expected: FAIL with `/api/docker-manager/ must have an explicit session-admin route contract`.

In `internal/audit/audit_manifests.go`, after the `/api/containers` contract (line 165).

Old:

```go
		{Pattern: "/api/containers", Methods: []string{"GET", "POST", "DELETE"}, Auth: "session-admin", Category: "integration", ContentTypes: []string{"application/json", "websocket"}},
```

New:

```go
		{Pattern: "/api/containers", Methods: []string{"GET", "POST", "DELETE"}, Auth: "session-admin", Category: "integration", ContentTypes: []string{"application/json", "websocket"}},
		{Pattern: "/api/docker-manager/", Methods: []string{"GET", "POST", "PUT", "DELETE"}, Auth: "session-admin", Category: "integration", ContentTypes: []string{"application/json"}},
```

- [ ] **Step 7: Run the tests (expected PASS)**

```powershell
go test ./internal/server -run 'TestDockerManager|TestInitDockerHosts|TestContainerRoutes' -count=1
go test ./internal/audit -run 'TestRouteContractManifest' -count=1
gofmt -l internal/server internal/audit
```

Expected: both `ok`; `gofmt -l` prints nothing.

- [ ] **Step 8: Graph check and commit**

```powershell
git add internal/server/docker_manager_api.go internal/server/docker_manager_fixture_test.go internal/server/docker_manager_api_test.go internal/server/docker_manager_auth_test.go internal/server/server_routes_tools.go internal/audit/audit_manifests.go internal/audit/audit_manifests_test.go
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): mount /api/docker-manager behind the admin gate" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B3: Host API — list, create, update, delete, connection test

**Files:**
- Create: `internal/server/docker_manager_hosts.go`
- Test: `internal/server/docker_manager_hosts_test.go`
- Modify: `internal/server/docker_manager_api.go` (the `route` function)

Routes: `GET/POST /hosts`, `POST /hosts/test` (draft), `PUT/DELETE /hosts/{id}`, `POST /hosts/{id}/test`.

- [ ] **Step 1: Write the failing tests**

Create `internal/server/docker_manager_hosts_test.go`:

```go
package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/dockerhost"
	"aurago/internal/security"
)

func TestDockerManagerHostCRUD(t *testing.T) {
	f := newDMFixture(t)

	code, body := f.do(http.MethodGet, "/hosts", nil)
	hosts := dmArray(t, body, "hosts")
	if code != http.StatusOK || len(hosts) != 1 || hosts[0]["id"] != "local" || hosts[0]["builtin"] != true || hosts[0]["kind"] != "local" {
		t.Fatalf("initial list = %d %s, want only the builtin local host", code, f.lastRaw)
	}

	code, body = f.do(http.MethodPost, "/hosts", map[string]any{"name": "NAS", "kind": "tcp", "address": "127.0.0.1", "port": f.remote.port()})
	if code != http.StatusCreated {
		t.Fatalf("create = %d %s, want 201", code, f.lastRaw)
	}
	host := dmObject(t, body, "host")
	id := dmString(t, host, "id")
	if !strings.HasPrefix(id, "h_") || host["builtin"] != false || host["plaintext"] != true || host["secret_set"] != false ||
		host["stacks_dir"] != dockerhost.DefaultStacksDir || host["color"] == "" || host["color"] == "blue" {
		t.Fatalf("created host = %s, want defaults filled, plaintext, no secret and a colour other than the local blue", f.lastRaw)
	}

	code, body = f.do(http.MethodPost, "/hosts", map[string]any{"name": " ", "kind": "tcp", "address": "127.0.0.1"})
	dmExpectError(t, f, code, body, http.StatusBadRequest, "invalid_host")
	if body["field"] != "name" {
		t.Fatalf("validation answer = %s, want field name", f.lastRaw)
	}
	code, body = f.do(http.MethodPost, "/hosts", map[string]any{"name": "Again local", "kind": "local"})
	dmExpectError(t, f, code, body, http.StatusBadRequest, "invalid_host")
	// Names are unique ignoring case, and "Local" belongs to the built-in host.
	for _, name := range []string{"nas", "LOCAL"} {
		code, body = f.do(http.MethodPost, "/hosts", map[string]any{"name": name, "kind": "tcp", "address": "127.0.0.1"})
		dmExpectError(t, f, code, body, http.StatusBadRequest, "invalid_host")
		if body["field"] != "name" {
			t.Fatalf("name %q answer = %s, want field name", name, f.lastRaw)
		}
	}

	code, body = f.do(http.MethodPut, "/hosts/"+id, map[string]any{"name": "NAS 2", "kind": "tcp", "address": "127.0.0.1", "port": f.remote.port()})
	host = dmObject(t, body, "host")
	if code != http.StatusOK || host["name"] != "NAS 2" || host["stacks_dir"] != dockerhost.DefaultStacksDir {
		t.Fatalf("update = %d %s, want the new name and the kept stacks_dir", code, f.lastRaw)
	}

	code, body = f.do(http.MethodPut, "/hosts/local", map[string]any{"name": "X", "kind": "tcp", "address": "127.0.0.1"})
	dmExpectError(t, f, code, body, http.StatusConflict, "local_host_fixed")
	code, body = f.do(http.MethodDelete, "/hosts/local", nil)
	dmExpectError(t, f, code, body, http.StatusConflict, "local_host_fixed")
	code, body = f.do(http.MethodDelete, "/hosts/h_000000000000", nil)
	dmExpectError(t, f, code, body, http.StatusNotFound, "host_not_found")

	code, body = f.do(http.MethodDelete, "/hosts/"+id, nil)
	if code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("delete = %d %s, want 200 ok", code, f.lastRaw)
	}
	_, body = f.do(http.MethodGet, "/hosts", nil)
	if got := len(dmArray(t, body, "hosts")); got != 1 {
		t.Fatalf("list after delete has %d hosts, want only local", got)
	}
}

func TestDockerManagerHostConnectionTest(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")

	code, body := f.do(http.MethodPost, "/hosts/"+id+"/test", nil)
	status := dmObject(t, dmObject(t, body, "host"), "status")
	if code != http.StatusOK || status["state"] != "ok" || status["engine_version"] != "27.0.1" {
		t.Fatalf("test = %d %s, want state ok and the engine version", code, f.lastRaw)
	}

	f.remote.srv.Close()
	code, body = f.do(http.MethodPost, "/hosts/"+id+"/test", nil)
	status = dmObject(t, dmObject(t, body, "host"), "status")
	if code != http.StatusOK || status["state"] != "error" || status["code"] == "" {
		t.Fatalf("unreachable test = %d %s, want 200 with state error and a reason code", code, f.lastRaw)
	}
	code, body = f.do(http.MethodPost, "/hosts/h_000000000000/test", nil)
	dmExpectError(t, f, code, body, http.StatusNotFound, "host_not_found")
}

func TestDockerManagerDraftTestDoesNotSave(t *testing.T) {
	f := newDMFixture(t)
	code, body := f.do(http.MethodPost, "/hosts/test", map[string]any{"name": "Draft", "kind": "tcp", "address": "127.0.0.1", "port": f.remote.port()})
	if status := dmObject(t, body, "host_status"); code != http.StatusOK || status["state"] != "ok" {
		t.Fatalf("draft test = %d %s, want host_status ok", code, f.lastRaw)
	}
	code, body = f.do(http.MethodPost, "/hosts/test", map[string]any{"name": "Draft", "kind": "tcp", "address": "http://127.0.0.1"})
	dmExpectError(t, f, code, body, http.StatusBadRequest, "invalid_host")
	if body["field"] != "address" {
		t.Fatalf("draft validation = %s, want field address", f.lastRaw)
	}
	_, body = f.do(http.MethodGet, "/hosts", nil)
	if got := len(dmArray(t, body, "hosts")); got != 1 {
		t.Fatalf("a draft test stored a host: %d hosts", got)
	}
}

func TestDockerManagerSecretsAreWriteOnly(t *testing.T) {
	f := newDMFixture(t)
	const secret = "TOPSECRET-KEY-MATERIAL"

	code, body := f.do(http.MethodPost, "/hosts", map[string]any{"name": "edge", "kind": "ssh", "address": "192.0.2.10", "ssh_user": "root", "ssh_secret": secret})
	if code != http.StatusCreated || dmObject(t, body, "host")["secret_set"] != true {
		t.Fatalf("create ssh host = %d %s, want 201 with secret_set", code, f.lastRaw)
	}
	if strings.Contains(f.lastRaw, secret) {
		t.Fatal("the create answer echoes the SSH secret")
	}
	id := dmString(t, body["host"], "id")
	f.do(http.MethodGet, "/hosts", nil)
	if strings.Contains(f.lastRaw, secret) {
		t.Fatal("the host list echoes the SSH secret")
	}
	if got, err := f.s.Vault.ReadSecret(dockerhost.SSHSecretKey(id)); err != nil || got != secret {
		t.Fatalf("vault secret = %q, %v; want the submitted secret", got, err)
	}

	// An edit without a secret (absent or blank) keeps it; a new one replaces it.
	edit := map[string]any{"name": "edge 2", "kind": "ssh", "address": "192.0.2.10", "ssh_user": "root"}
	for _, blank := range []bool{false, true} {
		if blank {
			edit["ssh_secret"] = ""
		}
		code, body = f.do(http.MethodPut, "/hosts/"+id, edit)
		if got, _ := f.s.Vault.ReadSecret(dockerhost.SSHSecretKey(id)); code != http.StatusOK || dmObject(t, body, "host")["secret_set"] != true || got != secret {
			t.Fatalf("update without secret (blank=%v) = %d %s, vault %q; want the secret kept", blank, code, f.lastRaw, got)
		}
	}
	edit["ssh_secret"] = "ROTATED-KEY"
	if code, _ = f.do(http.MethodPut, "/hosts/"+id, edit); code != http.StatusOK {
		t.Fatalf("update with a new secret = %d %s", code, f.lastRaw)
	}
	if got, _ := f.s.Vault.ReadSecret(dockerhost.SSHSecretKey(id)); got != "ROTATED-KEY" {
		t.Fatalf("vault secret after rotation = %q", got)
	}

	// An SSH host needs its secret; a kind that does not use one drops it.
	delete(edit, "ssh_secret")
	edit["clear_secret"] = true
	code, body = f.do(http.MethodPut, "/hosts/"+id, edit)
	dmExpectError(t, f, code, body, http.StatusBadRequest, "invalid_host")
	if body["field"] != "ssh_secret" {
		t.Fatalf("clearing the secret of an ssh host = %s, want field ssh_secret", f.lastRaw)
	}
	code, body = f.do(http.MethodPut, "/hosts/"+id, map[string]any{"name": "edge 2", "kind": "tcp", "address": "192.0.2.10", "clear_secret": true})
	if code != http.StatusOK || dmObject(t, body, "host")["secret_set"] != false {
		t.Fatalf("switch to tcp = %d %s, want secret_set false", code, f.lastRaw)
	}
	if _, err := f.s.Vault.ReadSecret(dockerhost.SSHSecretKey(id)); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatalf("vault secret after the switch: err = %v, want ErrSecretNotFound", err)
	}

	// Deleting a host removes its vault entries.
	code, body = f.do(http.MethodPost, "/hosts", map[string]any{"name": "edge3", "kind": "ssh", "address": "192.0.2.11", "ssh_user": "root", "ssh_secret": secret})
	if code != http.StatusCreated {
		t.Fatalf("create second ssh host = %d %s", code, f.lastRaw)
	}
	second := dmString(t, body["host"], "id")
	f.do(http.MethodDelete, "/hosts/"+second, nil)
	if _, err := f.s.Vault.ReadSecret(dockerhost.SSHSecretKey(second)); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatalf("vault secret after delete: err = %v, want ErrSecretNotFound", err)
	}
}

func TestDockerManagerHostListStartsBackgroundTests(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	f.do(http.MethodGet, "/hosts", nil)
	f.s.dockerStatus.wg.Wait()
	_, body := f.do(http.MethodGet, "/hosts", nil)
	host := dmFind(t, dmArray(t, body, "hosts"), "id", id)
	if dmObject(t, host, "status")["state"] != "ok" {
		t.Fatalf("host after a list round trip = %s, want state ok from the background test", f.lastRaw)
	}
}

func TestDockerManagerRejectsMalformedBodies(t *testing.T) {
	f := newDMFixture(t)
	for _, path := range []string{"/hosts", "/hosts/test"} {
		req := httptest.NewRequest(http.MethodPost, "/api/docker-manager"+path, strings.NewReader("{not json"))
		rec := httptest.NewRecorder()
		handleDockerManager(f.s)(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"bad_request"`) {
			t.Fatalf("POST %s with a malformed body = %d %s, want 400 bad_request", path, rec.Code, rec.Body.String())
		}
	}
}
```

- [ ] **Step 2: Run the tests (expected FAIL)**

```powershell
go test ./internal/server -run 'TestDockerManagerHost|TestDockerManagerDraft|TestDockerManagerSecrets|TestDockerManagerRejects' -count=1
```

Expected: FAIL; every test sees the `not_found` 404 answer of the empty route table.

- [ ] **Step 3: Implement the host handlers**

Create `internal/server/docker_manager_hosts.go`:

```go
package server

import (
	"net/http"
	"strings"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerhost/transport"
)

// dmHostTestTimeout bounds a connection test; Registry.Test and TestDraft limit
// themselves to 30 s.
const dmHostTestTimeout = 35 * time.Second

// dockerHostView is the HostView of the API contract: the stored host plus
// derived flags. Secrets never appear, only whether one is set.
type dockerHostView struct {
	dockerhost.Host
	Builtin   bool                  `json:"builtin"`
	SecretSet bool                  `json:"secret_set"`
	Plaintext bool                  `json:"plaintext"`
	Status    dockerhost.HostStatus `json:"status"`
}

func (dm *dmRequest) hostView(host dockerhost.Host, status dockerhost.HostStatus) dockerHostView {
	return dockerHostView{
		Host:      host,
		Builtin:   host.ID == dockerhost.LocalHostID,
		SecretSet: dm.registry().SecretSet(host.ID),
		Plaintext: host.Kind == dockerhost.KindTCP,
		Status:    status,
	}
}

func (dm *dmRequest) hostViews(hosts []dockerhost.Host) []dockerHostView {
	views := make([]dockerHostView, 0, len(hosts))
	for _, host := range hosts {
		views = append(views, dm.hostView(host, dm.registry().Status(host.ID)))
	}
	return views
}

// dockerHostWrite is the HostWrite body of POST and PUT /hosts and of the draft
// test. The secret members are write-only; the registry keeps stored values for
// absent or blank ones. ID is read by the draft test only, to reuse the secrets
// of the host that is being edited.
type dockerHostWrite struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Kind           string                 `json:"kind"`
	Address        string                 `json:"address"`
	Port           int                    `json:"port"`
	StacksDir      string                 `json:"stacks_dir"`
	HostSocketPath string                 `json:"host_socket_path"`
	Color          string                 `json:"color"`
	SSHUser        string                 `json:"ssh_user"`
	Sort           *int                   `json:"sort"`
	SSHSecret      *string                `json:"ssh_secret"`
	TLS            *transport.TLSMaterial `json:"tls"`
	ClearSecret    bool                   `json:"clear_secret"`
}

func (in dockerHostWrite) host() dockerhost.Host {
	host := dockerhost.Host{
		Name:           in.Name,
		Kind:           in.Kind,
		Address:        in.Address,
		Port:           in.Port,
		StacksDir:      in.StacksDir,
		HostSocketPath: in.HostSocketPath,
		Color:          in.Color,
		SSHUser:        in.SSHUser,
	}
	if in.Sort != nil {
		host.Sort = *in.Sort
	}
	return host
}

func (in dockerHostWrite) secrets() dockerhost.SecretInput {
	return dockerhost.SecretInput{SSHSecret: in.SSHSecret, TLS: in.TLS, Clear: in.ClearSecret}
}

// hostCollection serves GET and POST /hosts.
func (dm *dmRequest) hostCollection() {
	switch dm.r.Method {
	case http.MethodGet:
		dm.hostList()
	case http.MethodPost:
		dm.hostCreate()
	default:
		dm.allow(http.MethodGet, http.MethodPost)
	}
}

// hostList answers with the cached status of every host and starts a
// background test for stale ones, so the next poll shows fresh dots. It never
// waits for an unreachable host.
func (dm *dmRequest) hostList() {
	ctx, cancel := dm.context(dmRegistryTimeout)
	defer cancel()
	hosts, err := dm.registry().Hosts(ctx)
	if err != nil {
		dm.hostFailure(err)
		return
	}
	dm.s.dockerStatus.refreshStale(dm.registry(), hosts, time.Now())
	dmWriteOK(dm.w, http.StatusOK, dmFields{"hosts": dm.hostViews(hosts)})
}

// hostCreate stores a host. The registry normalises and validates it (empty
// paths get the defaults, an empty color the next palette entry) and checks the
// secrets before anything is written.
func (dm *dmRequest) hostCreate() {
	var in dockerHostWrite
	if !dm.decode(&in) {
		return
	}
	ctx, cancel := dm.context(dmRegistryTimeout)
	defer cancel()
	created, err := dm.registry().Create(ctx, in.host(), in.secrets())
	if err != nil {
		dm.hostFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusCreated, dmFields{"host": dm.hostView(created, dm.registry().Status(created.ID))})
}

// hostItem serves PUT and DELETE /hosts/{id}.
func (dm *dmRequest) hostItem(id string) {
	switch dm.r.Method {
	case http.MethodPut:
		dm.hostUpdate(id)
	case http.MethodDelete:
		dm.hostDelete(id)
	default:
		dm.allow(http.MethodPut, http.MethodDelete)
	}
}

// hostUpdate replaces a host. Empty paths keep the stored values (the registry
// would reset them to the defaults), an absent sort keeps the stored one.
func (dm *dmRequest) hostUpdate(id string) {
	ctx, cancel := dm.context(dmRegistryTimeout)
	defer cancel()
	current, ok := dm.host(ctx, id)
	if !ok {
		return
	}
	if current.ID == dockerhost.LocalHostID {
		dm.hostFailure(dockerhost.ErrLocalHostFixed)
		return
	}
	var in dockerHostWrite
	if !dm.decode(&in) {
		return
	}
	host := in.host()
	if strings.TrimSpace(host.StacksDir) == "" {
		host.StacksDir = current.StacksDir
	}
	if strings.TrimSpace(host.HostSocketPath) == "" {
		host.HostSocketPath = current.HostSocketPath
	}
	if in.Sort == nil {
		host.Sort = current.Sort
	}
	updated, err := dm.registry().Update(ctx, current.ID, host, in.secrets())
	if err != nil {
		dm.hostFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"host": dm.hostView(updated, dm.registry().Status(updated.ID))})
}

func (dm *dmRequest) hostDelete(id string) {
	ctx, cancel := dm.context(dmRegistryTimeout)
	defer cancel()
	if err := dm.registry().Delete(ctx, id); err != nil {
		dm.hostFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, nil)
}

// hostTest serves POST /hosts/{id}/test: it connects, caches the result and
// answers with the refreshed host. An unreachable host is still a 200; the
// failure is the test's result.
func (dm *dmRequest) hostTest(id string) {
	if !dm.allow(http.MethodPost) {
		return
	}
	ctx, cancel := dm.context(dmHostTestTimeout)
	defer cancel()
	host, ok := dm.host(ctx, id)
	if !ok {
		return
	}
	status := dm.registry().Test(ctx, host.ID)
	dmWriteOK(dm.w, http.StatusOK, dmFields{"host": dm.hostView(host, status)})
}

// hostDraftTest serves POST /hosts/test: it tests an unsaved draft. A draft
// that names a stored host by id takes the secrets it leaves out from that
// host's vault entries (done by TestDraft), so "Test connection" works while
// editing without retyping a key. An invalid draft is a 400 with the field.
func (dm *dmRequest) hostDraftTest() {
	if !dm.allow(http.MethodPost) {
		return
	}
	var in dockerHostWrite
	if !dm.decode(&in) {
		return
	}
	ctx, cancel := dm.context(dmHostTestTimeout)
	defer cancel()
	host := in.host().Normalized()
	if host.Color == "" {
		host.Color = dockerhost.Colors[0]
	}
	if err := dockerhost.ValidateHost(host); err != nil {
		dm.hostFailure(err)
		return
	}
	host.ID = strings.TrimSpace(in.ID)
	status := dm.registry().TestDraft(ctx, host, in.secrets())
	dmWriteOK(dm.w, http.StatusOK, dmFields{"host_status": status})
}
```

- [ ] **Step 4: Add the host routes**

In `internal/server/docker_manager_api.go`, replace the `route` function.

Old:

```go
func (dm *dmRequest) route(segs []string) {
	switch {
	default:
		dm.notFound()
	}
}
```

New:

```go
func (dm *dmRequest) route(segs []string) {
	switch {
	case len(segs) == 1 && segs[0] == "hosts":
		dm.hostCollection()
	case len(segs) == 2 && segs[0] == "hosts" && segs[1] == "test":
		dm.hostDraftTest()
	case len(segs) == 2 && segs[0] == "hosts":
		dm.hostItem(segs[1])
	case len(segs) == 3 && segs[0] == "hosts" && segs[2] == "test":
		dm.hostTest(segs[1])
	default:
		dm.notFound()
	}
}
```

- [ ] **Step 5: Run the tests (expected PASS)**

```powershell
go test ./internal/server -run 'TestDockerManager|TestInitDockerHosts' -count=1
gofmt -l internal/server
```

Expected: `ok`; `gofmt -l` prints nothing.

- [ ] **Step 6: Graph check and commit**

```powershell
git add internal/server/docker_manager_hosts.go internal/server/docker_manager_hosts_test.go internal/server/docker_manager_api.go
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): host list, create, update, delete and connection test API" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B4: Docker nests — list and import as hosts

**Files:**
- Create: `internal/server/docker_manager_nests.go`
- Test: `internal/server/docker_manager_nests_test.go`
- Modify: `internal/server/docker_manager_api.go` (the `route` function)

Routes: `GET /nests`, `POST /hosts/import-nest/{nestID}`. The nest list comes from `invasion.ListNests(s.InvasionDB)`
(`internal/invasion/invasion.go:427`); the TLS bundle through the existing `loadNestDockerTLS`
(`internal/server/invasion_docker_tls.go:111`); the SSH secret from `nest.VaultSecretID`, the way
`invasionTransportSecret` reads it (`invasion_docker_tls.go:163-179`).

- [ ] **Step 1: Write the failing tests**

Create `internal/server/docker_manager_nests_test.go`:

```go
package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/invasion"
)

// dmTestCAPEM returns a self-signed CA certificate in PEM form.
func dmTestCAPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "docker-manager-test-ca"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestDockerManagerNestListAndImport(t *testing.T) {
	f := newDMFixture(t)
	db, err := invasion.InitDB(filepath.Join(t.TempDir(), "invasion.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	f.s.InvasionDB = db

	const nestSecret = "NEST-SSH-PRIVATE-KEY"
	ca := dmTestCAPEM(t)
	if err := f.s.Vault.WriteSecret("nest_ssh_edge", nestSecret); err != nil {
		t.Fatal(err)
	}
	create := func(n invasion.NestRecord) string {
		id, err := invasion.CreateNest(db, n)
		if err != nil {
			t.Fatalf("CreateNest(%s): %v", n.Name, err)
		}
		return id
	}
	sshID := create(invasion.NestRecord{Name: "Edge SSH", Host: "192.0.2.20", Port: 2222, Username: "deploy", VaultSecretID: "nest_ssh_edge", DeployMethod: "docker_ssh", Active: true})
	tlsID := create(invasion.NestRecord{Name: "NAS TLS", Host: "nas.lan", Port: 2376, DeployMethod: "docker_remote", DockerTLS: "tls", Active: true})
	localID := create(invasion.NestRecord{Name: "This box", DeployMethod: "docker_local", Active: true})
	plainID := create(invasion.NestRecord{Name: "Plain SSH", Host: "192.0.2.30", DeployMethod: "ssh", Active: true})
	tlsMaterial, _ := json.Marshal(map[string]string{"ca": ca})
	if err := f.s.Vault.WriteSecret(invasion.DockerTLSVaultKey(tlsID), string(tlsMaterial)); err != nil {
		t.Fatal(err)
	}

	code, body := f.do(http.MethodGet, "/nests", nil)
	nests := dmArray(t, body, "nests")
	if code != http.StatusOK || len(nests) != 3 {
		t.Fatalf("nests = %d %s, want the three Docker nests", code, f.lastRaw)
	}
	for _, id := range []string{sshID, tlsID, localID} {
		dmFind(t, nests, "id", id)
	}
	for _, n := range nests {
		if n["id"] == plainID {
			t.Fatal("a plain ssh nest is listed as a Docker nest")
		}
	}

	// docker_ssh -> ssh host; the secret is copied, not shared.
	code, body = f.do(http.MethodPost, "/hosts/import-nest/"+sshID, nil)
	host := dmObject(t, body, "host")
	if code != http.StatusCreated || host["kind"] != "ssh" || host["ssh_user"] != "deploy" || host["port"] != float64(2222) ||
		host["address"] != "192.0.2.20" || host["name"] != "Edge SSH" || host["secret_set"] != true {
		t.Fatalf("ssh import = %d %s", code, f.lastRaw)
	}
	if strings.Contains(f.lastRaw, nestSecret) {
		t.Fatal("the import answer echoes the nest secret")
	}
	sshHost := dmString(t, host, "id")
	if got, err := f.s.Vault.ReadSecret(dockerhost.SSHSecretKey(sshHost)); err != nil || got != nestSecret {
		t.Fatalf("copied ssh secret = %q, %v", got, err)
	}
	// Host names are unique: importing the same nest twice is a validation error.
	code, body = f.do(http.MethodPost, "/hosts/import-nest/"+sshID, nil)
	dmExpectError(t, f, code, body, http.StatusBadRequest, "invalid_host")
	if body["field"] != "name" {
		t.Fatalf("second import = %s, want field name", f.lastRaw)
	}

	// docker_remote with tls -> tls host with the TLS bundle copied.
	code, body = f.do(http.MethodPost, "/hosts/import-nest/"+tlsID, nil)
	host = dmObject(t, body, "host")
	if code != http.StatusCreated || host["kind"] != "tls" || host["port"] != float64(2376) || host["secret_set"] != true {
		t.Fatalf("tls import = %d %s", code, f.lastRaw)
	}
	tlsHost := dmString(t, host, "id")
	if raw, err := f.s.Vault.ReadSecret(dockerhost.TLSSecretKey(tlsHost)); err != nil || !strings.Contains(raw, "BEGIN CERTIFICATE") {
		t.Fatalf("copied tls bundle = %q, %v", raw, err)
	}

	// Deleting the imported host leaves the nest's own secrets alone.
	f.do(http.MethodDelete, "/hosts/"+sshHost, nil)
	f.do(http.MethodDelete, "/hosts/"+tlsHost, nil)
	if got, err := f.s.Vault.ReadSecret("nest_ssh_edge"); err != nil || got != nestSecret {
		t.Fatalf("nest ssh secret after deleting the host = %q, %v", got, err)
	}
	if _, err := f.s.Vault.ReadSecret(invasion.DockerTLSVaultKey(tlsID)); err != nil {
		t.Fatalf("nest tls bundle after deleting the host: %v", err)
	}

	code, body = f.do(http.MethodPost, "/hosts/import-nest/"+localID, nil)
	dmExpectError(t, f, code, body, http.StatusBadRequest, "nest_not_docker")
	if msg, _ := body["message"].(string); !strings.Contains(msg, "already the Local host") {
		t.Fatalf("local nest message = %q", msg)
	}
	code, body = f.do(http.MethodPost, "/hosts/import-nest/"+plainID, nil)
	dmExpectError(t, f, code, body, http.StatusBadRequest, "nest_not_docker")
	code, body = f.do(http.MethodPost, "/hosts/import-nest/no-such-nest", nil)
	dmExpectError(t, f, code, body, http.StatusNotFound, "nest_not_found")
}

func TestDockerManagerNestsWithoutInvasion(t *testing.T) {
	f := newDMFixture(t)
	code, body := f.do(http.MethodGet, "/nests", nil)
	if nests := dmArray(t, body, "nests"); code != http.StatusOK || len(nests) != 0 {
		t.Fatalf("nests without Invasion Control = %d %s, want an empty list", code, f.lastRaw)
	}
	code, body = f.do(http.MethodPost, "/hosts/import-nest/anything", nil)
	dmExpectError(t, f, code, body, http.StatusNotFound, "nest_not_found")
}
```

- [ ] **Step 2: Run the tests (expected FAIL)**

```powershell
go test ./internal/server -run 'TestDockerManagerNest' -count=1
```

Expected: FAIL (`member "nests" … is not an array`, the 404 `not_found` answer).

- [ ] **Step 3: Implement nest list and import**

Create `internal/server/docker_manager_nests.go`:

```go
package server

import (
	"errors"
	"net/http"
	"strings"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerhost/transport"
	"aurago/internal/invasion"
	"aurago/internal/security"
)

// ── Invasion nest import ────────────────────────────────────────────────────
// A Docker nest (deploy method docker_remote or docker_ssh) becomes a Docker
// Manager host with one click. The vault secret is copied, never shared, so
// deleting either side leaves the other intact.

var (
	errNestLocalEngine = errors.New("this nest uses the local Docker engine, which is already the Local host")
	errNestNotDocker   = errors.New("this nest does not deploy through Docker")
)

// dockerNestView is one row of GET /nests.
type dockerNestView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DeployMethod string `json:"deploy_method"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	DockerTLS    string `json:"docker_tls"`
}

func isDockerNest(nest invasion.NestRecord) bool {
	switch nest.DeployMethod {
	case "docker_local", "docker_remote", "docker_ssh":
		return true
	}
	return false
}

// nestList serves GET /nests: the Invasion nests that deploy through Docker.
// Without Invasion Control the list is empty.
func (dm *dmRequest) nestList() {
	if !dm.allow(http.MethodGet) {
		return
	}
	views := []dockerNestView{}
	if dm.s.InvasionDB != nil {
		nests, err := invasion.ListNests(dm.s.InvasionDB)
		if err != nil {
			dm.internalFailure("list invasion nests", err)
			return
		}
		for _, nest := range nests {
			if !isDockerNest(nest) {
				continue
			}
			views = append(views, dockerNestView{
				ID: nest.ID, Name: nest.Name, DeployMethod: nest.DeployMethod,
				Host: nest.Host, Port: nest.Port, DockerTLS: nest.DockerTLS,
			})
		}
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"nests": views})
}

// findNest looks the nest up in the full list; GetNest has no typed
// not-found error.
func (dm *dmRequest) findNest(id string) (invasion.NestRecord, bool) {
	if dm.s.InvasionDB == nil {
		dmWriteError(dm.w, http.StatusNotFound, dmCodeNestNotFound, "Invasion Control is not enabled", nil)
		return invasion.NestRecord{}, false
	}
	nests, err := invasion.ListNests(dm.s.InvasionDB)
	if err != nil {
		dm.internalFailure("list invasion nests", err)
		return invasion.NestRecord{}, false
	}
	for _, nest := range nests {
		if nest.ID == id {
			return nest, true
		}
	}
	dmWriteError(dm.w, http.StatusNotFound, dmCodeNestNotFound, "nest not found", nil)
	return invasion.NestRecord{}, false
}

// dockerHostFromNest maps a nest onto a host draft; the registry fills the
// color and validates the result. docker_local is refused: it points at the
// local engine. An unsupported docker_tls value is a validation error.
func dockerHostFromNest(nest invasion.NestRecord) (dockerhost.Host, error) {
	name := dmTrimRunes(nest.Name, 64)
	if name == "" {
		name = dmTrimRunes(nest.ID, 64)
	}
	host := dockerhost.Host{
		Name:           name,
		Address:        strings.TrimSpace(nest.Host),
		Port:           nest.Port,
		StacksDir:      dockerhost.DefaultStacksDir,
		HostSocketPath: dockerhost.DefaultHostSocketPath,
	}
	switch nest.DeployMethod {
	case "docker_local":
		return dockerhost.Host{}, errNestLocalEngine
	case "docker_remote":
		switch strings.TrimSpace(nest.DockerTLS) {
		case invasion.DockerTLSOff:
			host.Kind = dockerhost.KindTCP
		case invasion.DockerTLSServer:
			host.Kind = dockerhost.KindTLS
		case invasion.DockerTLSMutual:
			host.Kind = dockerhost.KindMTLS
		default:
			return dockerhost.Host{}, &dockerhost.ValidationError{Field: "kind", Message: "the nest uses an unsupported docker_tls mode"}
		}
	case "docker_ssh":
		host.Kind = dockerhost.KindSSH
		host.SSHUser = strings.TrimSpace(nest.Username)
	default:
		return dockerhost.Host{}, errNestNotDocker
	}
	return host, nil
}

func dmTrimRunes(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) > limit {
		runes = runes[:limit]
	}
	return string(runes)
}

// nestSecrets copies the nest's credential: the TLS bundle of an encrypted
// docker_remote nest, or the SSH secret of a docker_ssh nest. A missing vault
// entry copies nothing.
func (dm *dmRequest) nestSecrets(nest invasion.NestRecord, kind string) (dockerhost.SecretInput, error) {
	var sec dockerhost.SecretInput
	if dm.s.Vault == nil {
		return sec, nil
	}
	switch kind {
	case dockerhost.KindSSH:
		if nest.VaultSecretID == "" {
			return sec, nil
		}
		value, err := dm.s.Vault.ReadSecret(nest.VaultSecretID)
		if errors.Is(err, security.ErrSecretNotFound) {
			return sec, nil
		}
		if err != nil {
			return sec, err
		}
		if value != "" {
			sec.SSHSecret = &value
		}
	case dockerhost.KindTLS, dockerhost.KindMTLS:
		material, err := dm.s.loadNestDockerTLS(nest.ID)
		if err != nil {
			return sec, err
		}
		if material != (invasion.DockerTLSMaterial{}) {
			copied := transport.TLSMaterial(material)
			sec.TLS = &copied
		}
	}
	return sec, nil
}

// nestImport serves POST /hosts/import-nest/{nestID}.
func (dm *dmRequest) nestImport(nestID string) {
	if !dm.allow(http.MethodPost) {
		return
	}
	nest, ok := dm.findNest(nestID)
	if !ok {
		return
	}
	host, err := dockerHostFromNest(nest)
	switch {
	case errors.Is(err, errNestLocalEngine), errors.Is(err, errNestNotDocker):
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeNestNotDocker, err.Error(), nil)
		return
	case err != nil:
		dm.hostFailure(err)
		return
	}
	ctx, cancel := dm.context(dmRegistryTimeout)
	defer cancel()
	sec, err := dm.nestSecrets(nest, host.Kind)
	if errors.Is(err, errDockerTLSMaterialUnreadable) {
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeInvalidHost, dockerTLSUnreadableMessage, dmFields{"field": "tls"})
		return
	}
	if err != nil {
		dm.internalFailure("read the nest credential", err)
		return
	}
	created, err := dm.registry().Create(ctx, host, sec)
	if err != nil {
		dm.hostFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusCreated, dmFields{"host": dm.hostView(created, dm.registry().Status(created.ID))})
}
```

- [ ] **Step 4: Add the nest routes**

In `internal/server/docker_manager_api.go`, replace the `route` function. `import-nest` must come before the
`/hosts/{id}/test` pattern.

Old:

```go
func (dm *dmRequest) route(segs []string) {
	switch {
	case len(segs) == 1 && segs[0] == "hosts":
		dm.hostCollection()
	case len(segs) == 2 && segs[0] == "hosts" && segs[1] == "test":
		dm.hostDraftTest()
	case len(segs) == 2 && segs[0] == "hosts":
		dm.hostItem(segs[1])
	case len(segs) == 3 && segs[0] == "hosts" && segs[2] == "test":
		dm.hostTest(segs[1])
	default:
		dm.notFound()
	}
}
```

New:

```go
func (dm *dmRequest) route(segs []string) {
	switch {
	case len(segs) == 1 && segs[0] == "hosts":
		dm.hostCollection()
	case len(segs) == 1 && segs[0] == "nests":
		dm.nestList()
	case len(segs) == 2 && segs[0] == "hosts" && segs[1] == "test":
		dm.hostDraftTest()
	case len(segs) == 3 && segs[0] == "hosts" && segs[1] == "import-nest":
		dm.nestImport(segs[2])
	case len(segs) == 2 && segs[0] == "hosts":
		dm.hostItem(segs[1])
	case len(segs) == 3 && segs[0] == "hosts" && segs[2] == "test":
		dm.hostTest(segs[1])
	default:
		dm.notFound()
	}
}
```

- [ ] **Step 5: Run the tests (expected PASS)**

```powershell
go test ./internal/server -run 'TestDockerManager|TestInitDockerHosts' -count=1
gofmt -l internal/server
```

Expected: `ok`; `gofmt -l` prints nothing.

- [ ] **Step 6: Graph check and commit**

```powershell
git add internal/server/docker_manager_nests.go internal/server/docker_manager_nests_test.go internal/server/docker_manager_api.go
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): list Docker nests and import them as hosts" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B5: Containers — list with protection, actions, inspect, logs, guarded remove

**Files:**
- Create: `internal/server/docker_manager_containers.go`
- Test: `internal/server/docker_manager_containers_test.go`
- Modify: `internal/server/docker_manager_api.go` (the `route` function)

Routes: `GET /hosts/{id}/containers`, `POST …/containers/{cid}/{start|stop|restart|pause|unpause|kill}`,
`DELETE …/containers/{cid}` (`?force=1&confirm=protected`), `GET …/containers/{cid}/inspect`,
`GET …/containers/{cid}/logs?tail=`. Protection reuses `containers_protection.go` without editing it: for the local
host `adminContainerListJSON` (list flags) and `containerActionAllowed` (remove, which calls the swappable
`containerProtectionFor`); for remote hosts `firstContainerOwner` and `containerProtection.confirmationMessage`.
`docker.readonly` is checked the way `containers_handlers.go` does (`containerDockerConfig`, third result) and
answers 403 `read_only`.

- [ ] **Step 1: Write the failing tests**

Create `internal/server/docker_manager_containers_test.go`:

```go
package server

import (
	"net/http"
	"strings"
	"testing"
)

// dmContainerNamed finds a container view by its first name.
func dmContainerNamed(t *testing.T, list []map[string]any, name string) map[string]any {
	t.Helper()
	for _, item := range list {
		if names, _ := item["names"].([]any); len(names) > 0 && names[0] == name {
			return item
		}
	}
	t.Fatalf("no container named %q in %v", name, list)
	return nil
}

func TestDockerManagerContainerListCarriesProtection(t *testing.T) {
	f := newDMFixture(t)

	// Local host: the Containers page's own classification plus the compose fields.
	code, body := f.do(http.MethodGet, "/hosts/local/containers", nil)
	if code != http.StatusOK {
		t.Fatalf("local list = %d %s", code, f.lastRaw)
	}
	list := dmArray(t, body, "containers")
	if len(list) != 2 {
		t.Fatalf("local list has %d containers, want 2: %s", len(list), f.lastRaw)
	}
	web, rv := dmContainerNamed(t, list, "web1"), dmContainerNamed(t, list, "rv")
	if web["compose_project"] != "shop" || web["protected_owner"] != nil {
		t.Fatalf("web1 = %v, want compose_project shop and no owner", web)
	}
	if rv["protected_owner"] != "ruview" || rv["state"] != "exited" {
		t.Fatalf("rv = %v, want protected_owner ruview", rv)
	}

	// Remote host: only the owner from names and labels.
	id := f.addRemote("NAS")
	code, body = f.do(http.MethodGet, "/hosts/"+id+"/containers", nil)
	if code != http.StatusOK {
		t.Fatalf("remote list = %d %s", code, f.lastRaw)
	}
	list = dmArray(t, body, "containers")
	cams, blog := dmContainerNamed(t, list, "cams"), dmContainerNamed(t, list, "blog")
	if cams["protected_owner"] != "ruview" || blog["protected_owner"] != nil || cams["self"] != nil {
		t.Fatalf("remote containers = %v / %v, want an owner only on cams and no self flag", cams, blog)
	}
}

func TestDockerManagerContainerActions(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	base := "/hosts/" + id + "/containers/"

	code, body := f.do(http.MethodPost, base+"web1/restart", nil)
	if code != http.StatusOK || body["status"] != "ok" || !f.remote.called("POST /containers/web1/restart") {
		t.Fatalf("restart = %d %s, calls %v", code, f.lastRaw, f.remote.calls)
	}
	code, body = f.do(http.MethodPost, base+"missing/stop", nil)
	dmExpectError(t, f, code, body, http.StatusNotFound, "container_not_found")
	code, body = f.do(http.MethodPost, base+"web1/explode", nil)
	dmExpectError(t, f, code, body, http.StatusBadRequest, "invalid_action")
	code, body = f.do(http.MethodGet, base+"web1/start", nil)
	dmExpectError(t, f, code, body, http.StatusMethodNotAllowed, "method_not_allowed")
	code, body = f.do(http.MethodPost, base+"bad%20name/start", nil)
	dmExpectError(t, f, code, body, http.StatusBadRequest, "bad_request")

	f.remote.force("POST /containers/web1/pause", http.StatusConflict)
	code, body = f.do(http.MethodPost, base+"web1/pause", nil)
	dmExpectError(t, f, code, body, http.StatusBadGateway, "docker_error")
}

func TestDockerManagerInspectAndLogs(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	base := "/hosts/" + id + "/containers/web1/"

	code, body := f.do(http.MethodGet, base+"inspect", nil)
	if inspect := dmObject(t, body, "inspect"); code != http.StatusOK || len(inspect) == 0 || !strings.Contains(f.lastRaw, "/web1") {
		t.Fatalf("inspect = %d %s, want the inspect object of web1", code, f.lastRaw)
	}
	code, body = f.do(http.MethodGet, base+"logs?tail=50", nil)
	logs, _ := body["logs"].(string)
	if code != http.StatusOK || !strings.Contains(logs, "hello from web1") {
		t.Fatalf("logs = %d %s, want the demultiplexed log line", code, f.lastRaw)
	}
	code, body = f.do(http.MethodGet, "/hosts/"+id+"/containers/missing/logs", nil)
	dmExpectError(t, f, code, body, http.StatusNotFound, "container_not_found")
}

func TestDockerManagerReadOnlyBlocksOnlyContainerMutations(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	f.setDocker(true, true)

	code, body := f.do(http.MethodPost, "/hosts/"+id+"/containers/web1/start", nil)
	dmExpectError(t, f, code, body, http.StatusForbidden, "read_only")
	code, body = f.do(http.MethodDelete, "/hosts/"+id+"/containers/web1", nil)
	dmExpectError(t, f, code, body, http.StatusForbidden, "read_only")
	if f.remote.called("POST /containers/web1/start") || f.remote.called("DELETE /containers/web1") {
		t.Fatalf("read-only mode reached the engine: %v", f.remote.calls)
	}
	if code, _ = f.do(http.MethodGet, "/hosts/"+id+"/containers", nil); code != http.StatusOK {
		t.Fatalf("read-only list = %d, want 200", code)
	}
	// Host management changes AuraGo's own configuration, not Docker.
	if code, _ = f.do(http.MethodPost, "/hosts", map[string]any{"name": "Second", "kind": "tcp", "address": "127.0.0.1", "port": f.remote.port()}); code != http.StatusCreated {
		t.Fatalf("read-only host create = %d %s, want 201", code, f.lastRaw)
	}
}

func TestDockerManagerRemoveAsksForProtectedConfirmation(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	base := "/hosts/" + id + "/containers/"

	code, body := f.do(http.MethodDelete, base+"cams", nil)
	dmExpectError(t, f, code, body, http.StatusConflict, containerCodeConfirmationRequired)
	if body["owner"] != "ruview" || f.remote.called("DELETE /containers/cams") {
		t.Fatalf("unconfirmed remove = %s, calls %v; want owner ruview and no engine delete", f.lastRaw, f.remote.calls)
	}
	code, body = f.do(http.MethodDelete, base+"cams?confirm=protected&force=1", nil)
	if code != http.StatusOK || !f.remote.called("DELETE /containers/cams") {
		t.Fatalf("confirmed remove = %d %s, calls %v", code, f.lastRaw, f.remote.calls)
	}
	if code, _ = f.do(http.MethodDelete, base+"blog", nil); code != http.StatusOK || !f.remote.called("DELETE /containers/blog") {
		t.Fatalf("unprotected remove = %d %s", code, f.lastRaw)
	}

	// The local host runs the full classification of the Containers API.
	defer replaceContainerProtection(containerProtection{Owner: "go2rtc"})()
	code, body = f.do(http.MethodDelete, "/hosts/local/containers/rv", nil)
	dmExpectError(t, f, code, body, http.StatusConflict, containerCodeConfirmationRequired)
	if body["owner"] != "go2rtc" || f.local.called("DELETE /containers/rv") {
		t.Fatalf("local unconfirmed remove = %s, calls %v", f.lastRaw, f.local.calls)
	}
	if code, _ = f.do(http.MethodDelete, "/hosts/local/containers/rv?confirm=protected", nil); code != http.StatusOK || !f.local.called("DELETE /containers/rv") {
		t.Fatalf("local confirmed remove = %d %s, calls %v", code, f.lastRaw, f.local.calls)
	}
}

func TestDockerManagerUnreachableHostIsABadGateway(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	f.remote.srv.Close()

	code, body := f.do(http.MethodGet, "/hosts/"+id+"/containers", nil)
	dmExpectError(t, f, code, body, http.StatusBadGateway, "host_unreachable")
	if reason, _ := body["reason"].(string); reason == "" {
		t.Fatalf("answer %s carries no reason", f.lastRaw)
	}
	code, body = f.do(http.MethodPost, "/hosts/"+id+"/containers/web1/restart", nil)
	dmExpectError(t, f, code, body, http.StatusBadGateway, "host_unreachable")
	code, body = f.do(http.MethodGet, "/hosts/h_000000000000/containers", nil)
	dmExpectError(t, f, code, body, http.StatusNotFound, "host_not_found")
}
```

- [ ] **Step 2: Run the tests (expected FAIL)**

```powershell
go test ./internal/server -run 'TestDockerManagerContainer|TestDockerManagerInspect|TestDockerManagerReadOnly|TestDockerManagerRemove|TestDockerManagerUnreachable' -count=1
```

Expected: FAIL (the 404 `not_found` answer of the missing container routes).

- [ ] **Step 3: Implement the container handlers**

Create `internal/server/docker_manager_containers.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

const (
	dmListTimeout   = 20 * time.Second
	dmReadTimeout   = 30 * time.Second
	dmActionTimeout = 60 * time.Second
)

var dmContainerActions = map[string]bool{
	"start": true, "stop": true, "restart": true, "pause": true, "unpause": true, "kill": true,
}

// dockerContainerView is the ContainerView of the API contract: the container
// plus the protection facts the UI needs before it offers a destructive action.
type dockerContainerView struct {
	dockerhost.ContainerSummary
	ProtectedOwner string `json:"protected_owner,omitempty"`
	Self           bool   `json:"self,omitempty"`
	DockerEndpoint bool   `json:"docker_endpoint,omitempty"`
	SharedNetwork  bool   `json:"shared_network,omitempty"`
}

// localProtectionFlags reuses the Containers page's own computation
// (adminContainerListJSON: self, shared network, Docker endpoint and owner) for
// the local engine, keyed by the 12-character container ID, so the two views
// can never disagree. A failed lookup returns nil; the owner then still comes
// from the container's names and labels.
func (dm *dmRequest) localProtectionFlags(ctx context.Context) map[string]adminContainerEntry {
	cfg, _, _ := containerDockerConfig(dm.s)
	var parsed struct {
		Status     string                `json:"status"`
		Containers []adminContainerEntry `json:"containers"`
	}
	if json.Unmarshal([]byte(adminContainerListJSON(ctx, dm.s, cfg)), &parsed) != nil || parsed.Status != "ok" {
		return nil
	}
	flags := make(map[string]adminContainerEntry, len(parsed.Containers))
	for _, entry := range parsed.Containers {
		flags[entry.ID] = entry
	}
	return flags
}

func (dm *dmRequest) containerViews(ctx context.Context, host dockerhost.Host, list []dockerhost.ContainerSummary) []dockerContainerView {
	var local map[string]adminContainerEntry
	if host.ID == dockerhost.LocalHostID {
		local = dm.localProtectionFlags(ctx)
	}
	views := make([]dockerContainerView, 0, len(list))
	for _, container := range list {
		view := dockerContainerView{
			ContainerSummary: container,
			ProtectedOwner:   firstContainerOwner(container.Names, container.Labels),
		}
		if flags, ok := local[container.ID]; ok {
			view.Self, view.DockerEndpoint, view.SharedNetwork = flags.Self, flags.DockerEndpoint, flags.SharedNetwork
			if flags.ProtectedOwner != "" {
				view.ProtectedOwner = flags.ProtectedOwner
			}
		}
		views = append(views, view)
	}
	return views
}

// containerList serves GET /hosts/{id}/containers.
func (dm *dmRequest) containerList(hostID string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	ctx, cancel := dm.context(dmListTimeout)
	defer cancel()
	host, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	list, err := dockerhost.ListContainers(ctx, client)
	if err != nil {
		dm.engineFailure(err, "")
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"containers": dm.containerViews(ctx, host, list)})
}

// containerSub serves /hosts/{id}/containers/{ref}/{inspect|logs|action}.
func (dm *dmRequest) containerSub(hostID, ref, sub string) {
	switch {
	case sub == "inspect":
		dm.containerInspect(hostID, ref)
	case sub == "logs":
		dm.containerLogs(hostID, ref)
	case dmContainerActions[sub]:
		dm.containerAction(hostID, ref, sub)
	default:
		dmWriteError(dm.w, http.StatusBadRequest, dmCodeInvalidAction, "unknown container action: "+dockerutil.SanitizeOneLine(sub, 32), nil)
	}
}

// containerAction runs start, stop, restart, pause, unpause or kill. These
// never ask for a protection confirmation, like /api/containers: stopping a
// protected container is confirmed in the UI only.
func (dm *dmRequest) containerAction(hostID, ref, action string) {
	if !dm.allow(http.MethodPost) {
		return
	}
	if dm.readOnly {
		dm.readOnlyRefusal()
		return
	}
	ctx, cancel := dm.context(dmActionTimeout)
	defer cancel()
	_, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	if err := dockerhost.ContainerAction(ctx, client, ref, action); err != nil {
		dm.engineFailure(err, dmCodeContainerNotFound)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, nil)
}

// containerInspect serves GET .../inspect (secret environment values are
// redacted by the engine layer).
func (dm *dmRequest) containerInspect(hostID, ref string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	ctx, cancel := dm.context(dmReadTimeout)
	defer cancel()
	_, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	raw, err := dockerhost.InspectContainer(ctx, client, ref)
	if err != nil {
		dm.engineFailure(err, dmCodeContainerNotFound)
		return
	}
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"inspect": raw})
}

// containerLogs serves GET .../logs?tail=N; the engine layer clamps N.
func (dm *dmRequest) containerLogs(hostID, ref string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	ctx, cancel := dm.context(dmReadTimeout)
	defer cancel()
	_, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	tail, _ := strconv.Atoi(dm.r.URL.Query().Get("tail"))
	logs, err := dockerhost.ContainerLogs(ctx, client, ref, tail)
	if err != nil {
		dm.engineFailure(err, dmCodeContainerNotFound)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"logs": logs})
}

// containerRemove serves DELETE .../containers/{ref}?force=1&confirm=protected.
func (dm *dmRequest) containerRemove(hostID, ref string) {
	if !dm.allow(http.MethodDelete) {
		return
	}
	if dm.readOnly {
		dm.readOnlyRefusal()
		return
	}
	ctx, cancel := dm.context(dmActionTimeout)
	defer cancel()
	host, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	if !dm.removeAllowed(ctx, host, client, ref) {
		return
	}
	force := dmTruthy(dm.r.URL.Query().Get("force"))
	if err := dockerhost.RemoveContainer(ctx, client, ref, force); err != nil {
		dm.engineFailure(err, dmCodeContainerNotFound)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, nil)
}

func dmTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// removeAllowed applies the confirmation rule of DELETE /api/containers/{id}.
// The local host runs the full classification of containerActionAllowed (self,
// Docker endpoint, shared network, owner). A remote host has only the owner
// from names and labels. It writes the 409 and returns false when the request
// stops.
func (dm *dmRequest) removeAllowed(ctx context.Context, host dockerhost.Host, client *dockerutil.Client, ref string) bool {
	if host.ID == dockerhost.LocalHostID {
		cfg, _, _ := containerDockerConfig(dm.s)
		return containerActionAllowed(dm.s, cfg, ref, "remove", dm.w, dm.r)
	}
	owner, err := remoteContainerOwner(ctx, client, ref)
	if err != nil {
		dm.engineFailure(err, "")
		return false
	}
	if owner != "" && dm.r.URL.Query().Get(containerConfirmParam) != containerConfirmProtected {
		p := containerProtection{Owner: owner}
		dmWriteError(dm.w, http.StatusConflict, containerCodeConfirmationRequired, p.confirmationMessage(), dmFields{"owner": p.label()})
		return false
	}
	return true
}

// remoteContainerOwner finds the container by name, full ID or ID prefix and
// returns the AuraGo owner its names and labels name. An unknown reference
// keeps the reserved-name check, like classifyContainerForAction; the remove
// then reports the 404.
func remoteContainerOwner(ctx context.Context, client *dockerutil.Client, ref string) (string, error) {
	list, err := dockerhost.ListContainers(ctx, client)
	if err != nil {
		return "", err
	}
	needle := strings.ToLower(ref)
	for _, container := range list {
		if dmContainerMatches(container, needle) {
			return firstContainerOwner(container.Names, container.Labels), nil
		}
	}
	return firstContainerOwner([]string{ref}, nil), nil
}

func dmContainerMatches(container dockerhost.ContainerSummary, needle string) bool {
	for _, name := range container.Names {
		if strings.ToLower(strings.TrimPrefix(name, "/")) == needle {
			return true
		}
	}
	return len(needle) >= 4 && strings.HasPrefix(strings.ToLower(container.FullID), needle)
}
```

- [ ] **Step 4: Add the container routes**

In `internal/server/docker_manager_api.go`, replace the `route` function.

Old:

```go
func (dm *dmRequest) route(segs []string) {
	switch {
	case len(segs) == 1 && segs[0] == "hosts":
		dm.hostCollection()
	case len(segs) == 1 && segs[0] == "nests":
		dm.nestList()
	case len(segs) == 2 && segs[0] == "hosts" && segs[1] == "test":
		dm.hostDraftTest()
	case len(segs) == 3 && segs[0] == "hosts" && segs[1] == "import-nest":
		dm.nestImport(segs[2])
	case len(segs) == 2 && segs[0] == "hosts":
		dm.hostItem(segs[1])
	case len(segs) == 3 && segs[0] == "hosts" && segs[2] == "test":
		dm.hostTest(segs[1])
	default:
		dm.notFound()
	}
}
```

New:

```go
func (dm *dmRequest) route(segs []string) {
	switch {
	case len(segs) == 1 && segs[0] == "hosts":
		dm.hostCollection()
	case len(segs) == 1 && segs[0] == "nests":
		dm.nestList()
	case len(segs) == 2 && segs[0] == "hosts" && segs[1] == "test":
		dm.hostDraftTest()
	case len(segs) == 3 && segs[0] == "hosts" && segs[1] == "import-nest":
		dm.nestImport(segs[2])
	case len(segs) == 2 && segs[0] == "hosts":
		dm.hostItem(segs[1])
	case len(segs) == 3 && segs[0] == "hosts" && segs[2] == "test":
		dm.hostTest(segs[1])
	case len(segs) == 3 && segs[0] == "hosts" && segs[2] == "containers":
		dm.containerList(segs[1])
	case len(segs) == 4 && segs[0] == "hosts" && segs[2] == "containers":
		dm.containerRemove(segs[1], segs[3])
	case len(segs) == 5 && segs[0] == "hosts" && segs[2] == "containers":
		dm.containerSub(segs[1], segs[3], segs[4])
	default:
		dm.notFound()
	}
}
```

- [ ] **Step 5: Run the tests (expected PASS)**

```powershell
go test ./internal/server -run 'TestDockerManager|TestInitDockerHosts|TestContainer' -count=1
gofmt -l internal/server
```

Expected: `ok` (the `TestContainer…` tests are the unchanged Containers API tests); `gofmt -l` prints nothing.

- [ ] **Step 6: Graph check and commit**

```powershell
git add internal/server/docker_manager_containers.go internal/server/docker_manager_containers_test.go internal/server/docker_manager_api.go
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): container list, actions, logs, inspect and guarded remove" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task B6: Overview, images, volumes, networks — and the part B check

**Files:**
- Create: `internal/server/docker_manager_resources.go`
- Test: `internal/server/docker_manager_resources_test.go`
- Modify: `internal/server/docker_manager_api.go` (the `route` function)

Routes: `GET /hosts/{id}/overview`, `GET /hosts/{id}/images`, `.../volumes`, `.../networks`. Usage counts come from
`dockerhost.Annotate` over the host's container list; if that list fails, images report `containers: -1`.

- [ ] **Step 1: Write the failing tests**

Create `internal/server/docker_manager_resources_test.go`:

```go
package server

import (
	"net/http"
	"testing"
)

func TestDockerManagerOverview(t *testing.T) {
	f := newDMFixture(t)
	code, body := f.do(http.MethodGet, "/hosts/local/overview", nil)
	info := dmObject(t, body, "info")
	if code != http.StatusOK || info["server_version"] != "27.0.1" || info["ncpu"] != float64(4) || info["containers_running"] != float64(1) {
		t.Fatalf("overview = %d %s, want the engine figures in snake_case", code, f.lastRaw)
	}
}

func TestDockerManagerResourceListsCarryUsage(t *testing.T) {
	f := newDMFixture(t)

	code, body := f.do(http.MethodGet, "/hosts/local/images", nil)
	images := dmArray(t, body, "images")
	if code != http.StatusOK || len(images) != 3 {
		t.Fatalf("images = %d %s", code, f.lastRaw)
	}
	if got := dmFind(t, images, "id", "sha256:img1")["containers"]; got != float64(1) {
		t.Fatalf("img1 containers = %v, want 1", got)
	}
	old := dmFind(t, images, "id", "sha256:old")
	if old["containers"] != float64(0) || old["dangling"] != true {
		t.Fatalf("old image = %v, want unused and dangling", old)
	}
	if tags, ok := old["tags"].([]any); !ok || len(tags) != 0 {
		t.Fatalf("old image tags = %v, want an empty array, never null", old["tags"])
	}

	code, body = f.do(http.MethodGet, "/hosts/local/volumes", nil)
	volumes := dmArray(t, body, "volumes")
	used, idle := dmFind(t, volumes, "name", "data"), dmFind(t, volumes, "name", "idle")
	if usedBy, _ := used["used_by"].([]any); code != http.StatusOK || len(usedBy) != 1 || usedBy[0] != "web1" {
		t.Fatalf("volume data = %v, want used_by [web1]", used)
	}
	if usedBy, ok := idle["used_by"].([]any); !ok || len(usedBy) != 0 {
		t.Fatalf("volume idle used_by = %v, want an empty array, never null", idle["used_by"])
	}

	code, body = f.do(http.MethodGet, "/hosts/local/networks", nil)
	networks := dmArray(t, body, "networks")
	bridge, spare := dmFind(t, networks, "name", "bridge"), dmFind(t, networks, "name", "idle")
	if code != http.StatusOK || bridge["containers"] != float64(1) || bridge["builtin"] != true || spare["containers"] != float64(0) {
		t.Fatalf("networks = %s, want bridge builtin with 1 container and idle with 0", f.lastRaw)
	}
}

func TestDockerManagerResourceListsRejectOtherMethods(t *testing.T) {
	f := newDMFixture(t)
	for _, path := range []string{"/hosts/local/images", "/hosts/local/volumes", "/hosts/local/networks", "/hosts/local/overview", "/hosts/local/containers"} {
		code, body := f.do(http.MethodPost, path, nil)
		dmExpectError(t, f, code, body, http.StatusMethodNotAllowed, "method_not_allowed")
	}
}

func TestDockerManagerResourceListsOfAnUnreachableHost(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	f.remote.srv.Close()
	for _, path := range []string{"/overview", "/images", "/volumes", "/networks"} {
		code, body := f.do(http.MethodGet, "/hosts/"+id+path, nil)
		dmExpectError(t, f, code, body, http.StatusBadGateway, "host_unreachable")
	}
}
```

- [ ] **Step 2: Run the tests (expected FAIL)**

```powershell
go test ./internal/server -run 'TestDockerManagerOverview|TestDockerManagerResourceLists' -count=1
```

Expected: FAIL (the 404 `not_found` answer).

- [ ] **Step 3: Implement the resource handlers**

Create `internal/server/docker_manager_resources.go`:

```go
package server

import (
	"context"
	"net/http"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

// overview serves GET /hosts/{id}/overview: the Engine's /info figures.
func (dm *dmRequest) overview(hostID string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	ctx, cancel := dm.context(dmReadTimeout)
	defer cancel()
	_, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	info, err := dockerhost.Info(ctx, client)
	if err != nil {
		dm.engineFailure(err, "")
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"info": info})
}

// resourceList serves GET /hosts/{id}/{images|volumes|networks}.
func (dm *dmRequest) resourceList(hostID, kind string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	ctx, cancel := dm.context(dmListTimeout)
	defer cancel()
	_, client, ok := dm.engine(ctx, hostID)
	if !ok {
		return
	}
	switch kind {
	case "images":
		dm.listImages(ctx, client)
	case "volumes":
		dm.listVolumes(ctx, client)
	case "networks":
		dm.listNetworks(ctx, client)
	default:
		dm.notFound()
	}
}

func (dm *dmRequest) listImages(ctx context.Context, client *dockerutil.Client) {
	images, err := dockerhost.ListImages(ctx, client)
	if err != nil {
		dm.engineFailure(err, "")
		return
	}
	// Without the container list the usage stays unknown: images keep -1.
	if containers, listErr := dockerhost.ListContainers(ctx, client); listErr == nil {
		dockerhost.Annotate(containers, images, nil, nil)
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"images": images})
}

func (dm *dmRequest) listVolumes(ctx context.Context, client *dockerutil.Client) {
	volumes, err := dockerhost.ListVolumes(ctx, client)
	if err != nil {
		dm.engineFailure(err, "")
		return
	}
	if containers, listErr := dockerhost.ListContainers(ctx, client); listErr == nil {
		dockerhost.Annotate(containers, nil, volumes, nil)
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"volumes": volumes})
}

func (dm *dmRequest) listNetworks(ctx context.Context, client *dockerutil.Client) {
	networks, err := dockerhost.ListNetworks(ctx, client)
	if err != nil {
		dm.engineFailure(err, "")
		return
	}
	if containers, listErr := dockerhost.ListContainers(ctx, client); listErr == nil {
		dockerhost.Annotate(containers, nil, nil, networks)
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"networks": networks})
}
```

- [ ] **Step 4: Add the final routes**

In `internal/server/docker_manager_api.go`, replace the `route` function.

Old:

```go
func (dm *dmRequest) route(segs []string) {
	switch {
	case len(segs) == 1 && segs[0] == "hosts":
		dm.hostCollection()
	case len(segs) == 1 && segs[0] == "nests":
		dm.nestList()
	case len(segs) == 2 && segs[0] == "hosts" && segs[1] == "test":
		dm.hostDraftTest()
	case len(segs) == 3 && segs[0] == "hosts" && segs[1] == "import-nest":
		dm.nestImport(segs[2])
	case len(segs) == 2 && segs[0] == "hosts":
		dm.hostItem(segs[1])
	case len(segs) == 3 && segs[0] == "hosts" && segs[2] == "test":
		dm.hostTest(segs[1])
	case len(segs) == 3 && segs[0] == "hosts" && segs[2] == "containers":
		dm.containerList(segs[1])
	case len(segs) == 4 && segs[0] == "hosts" && segs[2] == "containers":
		dm.containerRemove(segs[1], segs[3])
	case len(segs) == 5 && segs[0] == "hosts" && segs[2] == "containers":
		dm.containerSub(segs[1], segs[3], segs[4])
	default:
		dm.notFound()
	}
}
```

New:

```go
func (dm *dmRequest) route(segs []string) {
	switch {
	case len(segs) == 1 && segs[0] == "hosts":
		dm.hostCollection()
	case len(segs) == 1 && segs[0] == "nests":
		dm.nestList()
	case len(segs) == 2 && segs[0] == "hosts" && segs[1] == "test":
		dm.hostDraftTest()
	case len(segs) == 3 && segs[0] == "hosts" && segs[1] == "import-nest":
		dm.nestImport(segs[2])
	case len(segs) == 2 && segs[0] == "hosts":
		dm.hostItem(segs[1])
	case len(segs) == 3 && segs[0] == "hosts" && segs[2] == "test":
		dm.hostTest(segs[1])
	case len(segs) == 3 && segs[0] == "hosts" && segs[2] == "overview":
		dm.overview(segs[1])
	case len(segs) == 3 && segs[0] == "hosts" && segs[2] == "containers":
		dm.containerList(segs[1])
	case len(segs) == 4 && segs[0] == "hosts" && segs[2] == "containers":
		dm.containerRemove(segs[1], segs[3])
	case len(segs) == 5 && segs[0] == "hosts" && segs[2] == "containers":
		dm.containerSub(segs[1], segs[3], segs[4])
	case len(segs) == 3 && segs[0] == "hosts" && (segs[2] == "images" || segs[2] == "volumes" || segs[2] == "networks"):
		dm.resourceList(segs[1], segs[2])
	default:
		dm.notFound()
	}
}
```

- [ ] **Step 5: Run the Docker Manager tests (expected PASS)**

```powershell
go test ./internal/server -run 'TestDockerManager|TestInitDockerHosts' -count=1 -v
```

Expected: 26 tests print `--- PASS` (wiring 2, API core 4, auth 2, hosts 6, nests 2, containers 6, resources 4), none
prints `--- FAIL`, then `ok  aurago/internal/server`.

- [ ] **Step 6: Run the part B regression set (expected PASS)**

```powershell
go test ./internal/server -run 'TestContainer|TestAdminContainer' -count=1
go test ./internal/audit -run 'Manifest' -count=1
go vet ./internal/server ./internal/audit
gofmt -l internal/server internal/audit
```

Expected: both `go test` runs `ok`; `go vet` and `gofmt -l` print nothing. If a Go build fails with a "not enough
space on the disk" error, trim the Go build cache as described in the plan header note and rerun.

- [ ] **Step 7: Graph check and commit**

```powershell
git add internal/server/docker_manager_resources.go internal/server/docker_manager_resources_test.go internal/server/docker_manager_api.go
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): host overview and image, volume and network lists" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 8: Whole-branch graph check (once, after the last Part B commit)**

```powershell
npx gitnexus detect-changes --scope compare --base-ref main --repo AuraGo
```

Expected: the changed symbols are the new `docker_manager_*` handlers, `Server` (three fields), `Start`,
`registerToolAPIRoutes` and `RouteContractManifest`; no change to any `/api/containers` handler, `containers_protection.go`,
the `docker` agent tool or Invasion code from part B. `partial: true` or `truncated: true` is not a clean result: re-run it.

---

# Part C: desktop app

## Part C notes

Part C is the desktop app skeleton of Docker Manager phase 1: registration and the `docker` capability, the app icons in all
icon themes, the `docker_manager` strings in 16 locales, five JS modules, one stylesheet, the Node and browser tests and the
app documentation. Everything below was applied, in task order, to a scratch clone of main (`238f4d90`) and verified there:
`node scripts/test-docker-manager-ui.mjs`, the relevant `go test ./internal/desktop ./internal/server ./ui` guards, the
browser smoke (`TestDesktopDockerManagerBrowser`) in headless Chrome against the rebuilt `main.bundle.js`, the icon importer's
own offline check, `npm run check:ui`, and a separate 35-step interactive pass (keyboard, drawer, dialogs, protected remove,
narrow and light layouts, German) in real Chrome. The code in the plan is that verified code.

**Deviations from the contract (and why)**

1. **Where the `docker` capability is answered.** The contract says part C owns `internal/desktop/capabilities.go`. That file
   only holds the generic filter (`AppRequirementsMet`); the capability keys are answered by `desktopCapabilities.HasCapability`
   in `internal/server/bluetooth_live.go`. Task C1 adds one `case "docker"` there (`ConfigSnapshot().Docker.Enabled`) and a new
   test file `internal/server/docker_manager_capability_test.go`. **Part B must not edit `HasCapability`.** Docker's read-only
   switch does not hide the app (host management stays allowed; the app hides the container actions itself).
2. **`GET /hosts/{id}/overview` is not called.** The Phase 1 UI needs nothing the host status does not already carry
   (`engine_version`, `api_version`, `os`, `arch`).
3. **Icons are catalog icons, not `logo_path`.** The approved design asks for the icon in every icon theme, and the guards
   (`TestDesktopPapirusManifestMatchesBackendIconCatalog`, `TestDesktopAdditionalIconAssets`,
   `TestWindowsXPIconManifestCoversPreferredDesktopIcons`) tie every preferred name to every manifest. So `docker-manager`
   joins `desktopPreferredIconNames`; Papirus and WhiteSur get hand-written originals (a neutral container stack, never the
   Docker whale); Tela, Candy, Fluent and Buuf get the original AuraGo supplement glyph `container-stack` through the importer
   inputs; Windows XP follows from `node scripts/build-desktop-windows-xp-icons.mjs`. The full importer run needs the four pinned
   upstream archives, and Buuf's (`git.disroot.org`) was unreachable when this was written, so Task C2 uses a small offline helper
   that writes exactly what the importer writes for a supplement and then runs the importer's own `verify` (the same check as
   `python scripts/import-desktop-icon-themes.py --check`). If you have all four archives, the archive-backed `--check
   --source-dir` additionally proves byte-for-byte reproduction.
4. **Translations.** All 16 locales get real translations (192 keys each: the app name and 191 `docker_manager.*` keys), not English fallbacks. The only values identical to
   English in every language are technical or brand strings: `desktop.app_docker_manager` ("Docker Manager"), `kind_tls`
   ("TLS"), `kind_ssh` ("SSH"), `ov_id` ("ID"), `host_engine` ("Docker {{version}}") and `row_action_label`
   ("{{action}}: {{name}}"). Greek, Hindi, Japanese and Chinese were written without a native reviewer: have a native speaker
   look at `el`, `hi`, `ja` and `zh` before a release. "Nest" follows the existing `invasion.*` wording of each language.
5. **No central Node test runner exists** (`scripts/test-infra-ui.mjs` is run by hand). Task C4 adds the npm script
   `test:docker-manager`; there is no CI registry to extend.
6. **Beyond the contract's Phase 1 list:** the drawer has an Overview tab next to Logs and Inspect; group collapse, state filter
   and host scope persist in `localStorage` key `docker-manager.ui.v1`; window menus (Refresh, Manage hosts, Add host, state
   filters); toasts; a context menu on rows (right click, Shift+F10).
7. **Pinned app lists.** `TestDesktopStartMenuCategoriesStayInSync` and `TestBuiltinAppsCarryStartMenuCategories` pin the category
   list, not the app list, so they need no edit: the app joins the existing `system` category, and no other test enumerates
   the built-in app ids.

**How the UI follows the facts from parts A and B**

- Host form: `field` errors of `invalid_host` map to the inputs (`ssh_secret` and `tls` to the secret inputs, which open if they
  were tucked away). Names are checked client-side (unique ignoring case, "Local" reserved). On edit, blank secret fields keep
  the stored value: the form shows "Credentials stored" and a **Replace** button instead of empty required fields; TLS PEM
  fields are sent only when typed (the server merges them one by one); `stacks_dir` and `host_socket_path` are sent as typed
  (empty = keep on edit, default on create).
- **Test connection** posts the draft to `POST /hosts/test` with the host's `id` when editing, so stored secrets are reused. The
  button alone is busy (a test of an unreachable host can take 30 s); the dialog stays usable. The host list's **Test** and the
  banner's **Retry** use `POST /hosts/{id}/test`, which answers 200 with the host even when it is down, so the outcome is read
  from `host.status`.
- `GET /hosts` is polled with every container poll (5 s, only while the window is visible); the server answers from cached
  statuses and refreshes stale ones in the background. `checked_at` of `0001-01-01…` means "not checked yet". Container
  requests wait 35 s (the server gives up after 30 s); a browser timeout is shown as unreachable with reason `timeout`.
- Errors: `host_unreachable` shows its `reason` sentence; `read_only`, `container_not_found`, `invalid_host`, `network`,
  `timeout` and `forbidden` have their own texts; every other code (`docker_hosts_unavailable`, `not_found`,
  `method_not_allowed`, `bad_request`, `internal_error`, `docker_error`, `host_not_found`, ...) shows the generic sentence plus
  the server's `message`. `docker_disabled` (503, also on `/hosts` and `/nests`) switches the whole app to one explanatory state.
- Protected containers: stop, restart, pause, kill and remove ask first in the client (the server never confirms the first
  four). A protected remove is sent with `?confirm=protected` (and `&force=1`) once the admin confirmed; when the client did
  not know the container was protected, the 409 `container_protected_confirmation_required` answer opens the same dialog with
  the server's `message`/`owner` and the request is repeated.

**What parts A and B must know**

- Requests the UI makes: `GET /hosts`, `GET /hosts/{id}/containers`, `POST /hosts/{id}/containers/{cid}/{start|stop|restart|pause|unpause|kill}`
  (no query), `DELETE /hosts/{id}/containers/{cid}[?force=1][&confirm=protected]`, `GET .../inspect`, `GET .../logs?tail=N` with N in
  100, 200, 500, 1000, 5000, `POST /hosts`, `PUT /hosts/{id}`, `DELETE /hosts/{id}`, `POST /hosts/test` (+ optional `id`),
  `POST /hosts/{id}/test`, `GET /nests`, `POST /hosts/import-nest/{nestID}`.
- It sends `credentials: same-origin` through the desktop `api()` (cookie session). Container arrays must be `[]`, never `null`.
- The UI shows `container.status` verbatim (Docker's own English text, e.g. "Up 3 hours") as the secondary line and derives the
  exit code from `Exited (N)`; keep that text format.
- Pre-existing red tests on main (HEAD `238f4d90`) that this part neither causes nor fixes:
  `TestDashboardCardsFollowSectionPlacementAndAnatomy` and `TestFrontend_StaticI18nKeysExistInEnglishBundle`
  (missing `help.chromecast.*` keys). Compare against main before blaming this part.

**Conventions used below.** Commands are PowerShell from the worktree root `.worktrees\docker-manager`. Throwaway helper scripts
go to `disposable\` (never staged: every commit adds exact paths). "Replace ... with ..." steps are exact-text edits; the
edit fails if the old text is not found exactly once, which is the check that the file is as expected.

### Task C1: Register the app and the `docker` capability

**Files**

- Modify: `internal/desktop/types.go` (`desktopPreferredIconNames`, `BuiltinApps`)
- Modify: `internal/server/bluetooth_live.go` (`desktopCapabilities.HasCapability`)
- Test: `internal/desktop/docker_manager_app_test.go` (create)
- Test: `internal/server/docker_manager_capability_test.go` (create)

- [ ] **Step 1: Impact before editing.** Both edits only add one list element and one `case`; confirm the callers.

```powershell
npx gitnexus impact "BuiltinApps" --direction upstream --repo AuraGo
npx gitnexus impact "HasCapability" --direction upstream --repo AuraGo
git grep -n "BuiltinApps()" -- "*.go"
git grep -n "HasCapability(" -- "internal/*.go"
```

Expected: `BuiltinApps` is read by the desktop service, the bootstrap handler and tests; `HasCapability` is implemented once
(`desktopCapabilities`) and called through `AppRequirementsMet`. A HIGH/CRITICAL rating for these shared hub symbols is
expected; the edits are additive (a new app entry, a new switch case), no signature changes. If GitNexus answers
`UNKNOWN`/`Target not found` for a Go symbol, the two `git grep` commands are the fallback. Note the result in the commit body.

- [ ] **Step 2: Write the failing tests.**

Create `internal/desktop/docker_manager_app_test.go`:

```go
package desktop

import "testing"

// The Docker Manager app is a system app that stays hidden until the "docker" capability is met.
func TestBuiltinDockerManagerAppRequiresDockerCapability(t *testing.T) {
	app := testFindApp(t, BuiltinApps(), "docker-manager")
	if app.Entry != "builtin://docker-manager" || app.Name != "Docker Manager" || app.Icon != "docker-manager" || app.Category != "system" {
		t.Fatalf("docker-manager manifest = %+v", app)
	}
	if len(app.Requires) != 1 || app.Requires[0] != "docker" {
		t.Fatalf("docker-manager requires = %v, want [docker]", app.Requires)
	}
	if !app.Builtin || !app.StartVisible || !app.DockVisible || app.Deletable {
		t.Fatalf("docker-manager must be a visible, permanent builtin: %+v", app)
	}
	iconKnown := false
	for _, name := range desktopPreferredIconNames {
		if name == app.Icon {
			iconKnown = true
		}
	}
	if !iconKnown {
		t.Fatalf("icon %q is not a preferred desktop icon", app.Icon)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{}); len(got) != 0 {
		t.Fatalf("without the capability the app must stay hidden: %+v", got)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{"docker": true}); len(got) != 1 {
		t.Fatalf("with the capability the app must show: %+v", got)
	}
}
```

Create `internal/server/docker_manager_capability_test.go`:

```go
package server

import (
	"testing"

	"aurago/internal/config"
)

// The "docker" desktop capability shows the Docker Manager app exactly while Docker is enabled. Docker's read-only
// switch does not hide it: host management stays allowed, and the app itself hides the container actions.
func TestDesktopCapabilityDockerFollowsDockerEnabled(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	caps := desktopCapabilities{s: s}
	if caps.HasCapability("docker") {
		t.Fatal("the docker capability must be off while Docker is disabled")
	}
	s.Cfg.Docker.Enabled = true
	if !caps.HasCapability("docker") {
		t.Fatal("the docker capability must be on while Docker is enabled")
	}
	s.Cfg.Docker.ReadOnly = true
	if !caps.HasCapability("docker") {
		t.Fatal("Docker read-only must not hide the app")
	}
	if (desktopCapabilities{}).HasCapability("docker") {
		t.Fatal("without a server nothing is available")
	}
}
```

- [ ] **Step 3: Run them; expect FAIL.**

```powershell
go test ./internal/desktop -run TestBuiltinDockerManagerAppRequiresDockerCapability -count=1
go test ./internal/server -run TestDesktopCapabilityDockerFollowsDockerEnabled -count=1
```

Expected: the first fails with `app "docker-manager" not found`, the second with
`the docker capability must be on while Docker is enabled`.

- [ ] **Step 4: Implement.**

In `internal/desktop/types.go`:

Edit 1 of 2: replace

```text
	"dozzle",
```

with

```text
	"docker-manager",
	"dozzle",
```

Edit 2 of 2: replace

```text
		{ID: "system-world", Name: "System World"
```

with

```text
		{ID: "docker-manager", Name: "Docker Manager", Version: "1.0.0", Icon: "docker-manager", Entry: "builtin://docker-manager", Runtime: BuiltinRuntime, Category: "system", Description: "Manage Docker on this server and on remote hosts: containers, logs and host connections in one window.", Requires: []string{"docker"}},
		{ID: "system-world", Name: "System World"
```

In `internal/server/bluetooth_live.go`:

Replace

```text
	case "cyd_creator":
		return c.s.cydCreatorCapability()
```

with

```text
	case "cyd_creator":
		return c.s.cydCreatorCapability()
	case "docker":
		cfg := c.s.ConfigSnapshot()
		return cfg != nil && cfg.Docker.Enabled
```

- [ ] **Step 5: Run; expect PASS.**

```powershell
go test ./internal/desktop -count=1
go test ./internal/server -run "TestDesktopCapabilityDockerFollowsDockerEnabled|TestCYDCreatorCapabilityAndConfigReload" -count=1
gofmt -l internal/desktop internal/server/bluetooth_live.go internal/server/docker_manager_capability_test.go
```

Expected: both packages `ok`, `gofmt -l` prints nothing. The `ui` icon and name guards
(`TestDesktopPapirusManifestMatchesBackendIconCatalog`, `TestDesktopBuiltinAppNamesExistInAllLanguages`, ...) are red from here
until Tasks C2 and C3; that is expected.

- [ ] **Step 6: Detect changes and commit.**

```powershell
git add internal/desktop/types.go internal/server/bluetooth_live.go internal/desktop/docker_manager_app_test.go internal/server/docker_manager_capability_test.go
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): register the Docker Manager desktop app behind the docker capability" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Expected: detect-changes lists `BuiltinApps` and `HasCapability` as the changed symbols and nothing else.

### Task C2: App icons in every icon theme

**Files**

- Create: `ui/img/papirus/icons/docker-manager.svg`, `ui/img/whitesur/icons/docker-manager.svg`
- Modify: `ui/img/papirus/manifest.json`, `ui/img/whitesur/manifest.json`, `ui/img/windows-xp/manifest.json` (generated)
- Modify (importer inputs): `assets/desktop-icon-themes/supplements.json`, `tela.json`, `candy.json`, `fluent.json`, `buuf_nestort.json`
- Create/modify (importer outputs): `ui/img/{tela,candy,fluent,buuf-nestort}/icons/docker-manager.svg`, and each theme's `manifest.json` and `provenance.json`
- Test: existing guards (`TestDesktopPapirusManifestMatchesBackendIconCatalog`, `TestDesktopWhiteSurManifestMatchesBackendIconCatalog`, `TestDesktopAdditionalIconAssets`, `TestWindowsXPIconManifestCoversPreferredDesktopIcons`) and the importer's offline check

- [ ] **Step 1: Run the guards; expect FAIL.**

```powershell
go test ./ui -run "TestDesktopPapirusManifestMatchesBackendIconCatalog|TestDesktopWhiteSurManifestMatchesBackendIconCatalog|TestDesktopAdditionalIconAssets|TestWindowsXPIconManifestCoversPreferredDesktopIcons" -count=1
```

Expected: FAIL, each naming the preferred icon `docker-manager` as missing from a manifest (Windows XP: the manifest has 152
mappings, the backend prefers 153).

- [ ] **Step 2: Create the two original icons.** A neutral stack of three shipping containers with door seams and a base line
  (flat dark tile for Papirus, rounded gradient tile for WhiteSur, like the existing `ruview` and `tresor` icons). No whale, no
  Docker artwork.

`ui/img/papirus/icons/docker-manager.svg`:

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect x="6" y="9" width="52" height="46" rx="5" fill="#2d4358"/><g stroke="#14232f" stroke-width="1.6" stroke-linejoin="round"><rect x="15" y="14" width="34" height="9" rx="1.5" fill="#6cdec8"/><rect x="11" y="25" width="34" height="9" rx="1.5" fill="#5aa9ff"/><rect x="19" y="36" width="34" height="9" rx="1.5" fill="#a9d6ff"/></g><g stroke="#14232f" stroke-width="1.4" stroke-linecap="round" opacity=".55"><path d="M23 16v5M29 16v5M35 16v5M41 16v5"/><path d="M19 27v5M25 27v5M31 27v5M37 27v5"/><path d="M27 38v5M33 38v5M39 38v5M45 38v5"/></g><path d="M14 49h36" stroke="#e9f5f7" stroke-width="3" stroke-linecap="round" opacity=".85"/></svg>
```

`ui/img/whitesur/icons/docker-manager.svg`:

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><defs><linearGradient id="a" x2="0" y2="1"><stop stop-color="#4aa8e8"/><stop offset="1" stop-color="#1f5f9c"/></linearGradient></defs><rect x="3" y="3" width="58" height="58" rx="14" fill="url(#a)"/><g stroke="#1f5f9c" stroke-width="1.4" stroke-linejoin="round" fill="#fff"><rect x="16" y="13" width="34" height="9" rx="2" fill-opacity=".95"/><rect x="12" y="25" width="34" height="9" rx="2" fill-opacity=".8"/><rect x="18" y="37" width="34" height="9" rx="2" fill-opacity=".65"/></g><g stroke="#1f5f9c" stroke-width="1.4" stroke-linecap="round" opacity=".5"><path d="M24 15v5M30 15v5M36 15v5M42 15v5"/><path d="M20 27v5M26 27v5M32 27v5M38 27v5"/><path d="M26 39v5M32 39v5M38 39v5M44 39v5"/></g><path d="M14 51h36" stroke="#fff" stroke-width="3" stroke-linecap="round" opacity=".75"/></svg>
```

- [ ] **Step 3: Register them in the two hand-maintained manifests.**

In `ui/img/papirus/manifest.json`:

Replace

```text
    "dozzle": "img/papirus/icons/dozzle.svg",
```

with

```text
    "docker-manager": "img/papirus/icons/docker-manager.svg",
    "dozzle": "img/papirus/icons/dozzle.svg",
```

In `ui/img/whitesur/manifest.json`:

Replace

```text
                  "dozzle":  "img/whitesur/icons/dozzle.svg",
```

with

```text
                  "docker-manager": "img/whitesur/icons/docker-manager.svg",
                  "dozzle":  "img/whitesur/icons/dozzle.svg",
```

- [ ] **Step 4: Create the offline helper for the four imported themes.** It adds the supplement glyph and the mapping to the
  importer inputs, then writes the SVG, the manifest entry (with fresh input hashes) and the provenance record of each theme
  exactly as the importer does for a supplement, and finally calls the importer's `verify`. It refuses to run if an input file
  does not round-trip through its serialiser, so it cannot silently reformat a reviewed file.

Create `disposable\dm-icons\add-docker-manager-icon.py` (not committed):

```python
#!/usr/bin/env python3
"""One-off helper (not committed): registers the docker-manager icon in the four imported icon themes.

It adds the original AuraGo supplement glyph "container-stack" to assets/desktop-icon-themes/supplements.json, maps
"docker-manager" to it in the four theme files, and writes exactly what scripts/import-desktop-icon-themes.py writes for
a supplement use: the SVG, the manifest entry with fresh input hashes, and the provenance record. Every other record
stays as shipped, so no upstream archive is needed. The importer's own offline check (verify) runs at the end.

Usage: python add-docker-manager-icon.py <repo-root>
"""
import importlib.util
import json
import sys
from pathlib import Path

root = Path(sys.argv[1]).resolve()
spec = importlib.util.spec_from_file_location("icon_import", root / "scripts/import-desktop-icon-themes.py")
icons = importlib.util.module_from_spec(spec)
spec.loader.exec_module(icons)
assert icons.ROOT == root, "the importer must live inside the repository root that was passed in"

KEY = "docker-manager"
MOTIF = "container-stack"
GLYPH = ('<rect x="6" y="3" width="15" height="5" rx="1"/><rect x="3" y="9.5" width="15" height="5" rx="1"/>'
         '<rect x="7" y="16" width="14" height="5" rx="1"/><path d="M13.5 3.5v4M10.5 10v4M14 16.5v4"/>')


def dump_input(value):
    # The reviewed input files keep their own key order; json.dumps reproduces them from the parsed value.
    return (json.dumps(value, ensure_ascii=False, indent=2) + "\n").encode()


def read_json(path, dump=icons.json_bytes):
    text = path.read_text(encoding="utf-8").replace("\r\n", "\n")
    value = json.loads(text)
    # Rewriting is only safe when the serialiser reproduces the file byte for byte.
    assert dump(value).decode() == text, str(path) + " does not round-trip through its JSON writer"
    return value


def write_json(path, value, dump=icons.json_bytes):
    path.write_bytes(dump(value))


def insert_sorted(mapping, key, value):
    out, placed = {}, False
    for existing, item in mapping.items():
        if not placed and existing > key:
            out[key] = value
            placed = True
        out[existing] = item
    if not placed:
        out[key] = value
    return out


sources = json.loads((icons.INPUT / "sources.json").read_text(encoding="utf-8"))

supplements = insert_sorted(read_json(icons.INPUT / "supplements.json", dump_input), MOTIF, GLYPH)
write_json(icons.INPUT / "supplements.json", supplements, dump_input)
for theme in sources:
    mappings = read_json(icons.INPUT / (theme + ".json"), dump_input)
    mappings["icons"] = insert_sorted(mappings["icons"], KEY, {"supplement": MOTIF})
    write_json(icons.INPUT / (theme + ".json"), mappings, dump_input)

geometry = json.loads((icons.INPUT / "supplements.json").read_text(encoding="utf-8"))
for theme, theme_spec in sources.items():
    directory = "img/" + theme.replace("_", "-")
    base = root / "ui" / directory
    # The same construction as emit() in the importer for a supplement with role "icons".
    ink, accent = theme_spec["supplement_colors"]["light"]
    body = geometry[MOTIF].replace("currentColor", ink)
    gradient = '<defs><linearGradient id="aura-ink" x2="1" y2="1"><stop stop-color="' + ink + '"/><stop offset="1" stop-color="' + accent + '"/></linearGradient></defs>' if theme == "candy" else ""
    stroke = "url(#aura-ink)" if gradient else ink
    if gradient:
        body = body.replace(ink, stroke)
    if theme in ("tela", "fluent", "buuf_nestort"):
        if theme == "buuf_nestort":
            tile = '<path d="M3 3 20 2 22 19 4 22 2 8Z" fill="#e7c990" stroke="#69442b" stroke-width="1"/><path d="m4 5 14-1M5 19l13-1" stroke="#fff1cc" opacity=".65"/>'
        elif theme == "fluent":
            tile = '<rect x="2" y="3" width="20" height="19" rx="5" fill="#157bc6" opacity=".2"/><rect x="2" y="2" width="20" height="19" rx="5" fill="#c7e7ff"/>'
        else:
            tile = '<circle cx="12" cy="12" r="11" fill="#d3e8fc"/><path d="M4 18 18 4a10 10 0 0 1-14 14" fill="#5294e2" opacity=".2"/>'
        body = tile + '<g transform="translate(3 3) scale(.75)">' + body + '</g>'
    data = icons.safe_svg(('<svg xmlns="' + icons.SVG + '" viewBox="0 0 24 24" width="24" height="24">' + gradient + '<g fill="none" stroke="' + stroke + '" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">' + body + '</g></svg>').encode())
    relative = directory + "/icons/" + KEY + ".svg"
    sha = icons.digest(data)

    manifest = read_json(base / "manifest.json")
    records = read_json(base / "provenance.json")
    assert relative not in records and KEY not in manifest["icons"], theme + " already has " + KEY
    assert not any(record["sha256"] == sha for record in records.values()), theme + ": another asset has identical bytes"
    manifest["icons"][KEY] = relative
    manifest["inputs"] = icons.input_hashes(theme)
    records[relative] = {
        "license": "MIT", "authors": ["AuraGo contributors"], "sha256": sha,
        "uses": [{"key": KEY, "role": "icons", "variant": "", "source": "AuraGo original: " + MOTIF, "requested_source": None,
                  "source_sha256": icons.digest(geometry[MOTIF].encode()), "resolved_links": [],
                  "changes": ["Original AuraGo supplement; no upstream artwork used"]}]}
    (base / "icons").mkdir(parents=True, exist_ok=True)
    (root / "ui" / relative).write_bytes(data)
    write_json(base / "manifest.json", manifest)
    write_json(base / "provenance.json", records)

icons.verify(sources)
print("Registered " + KEY + " in " + str(len(sources)) + " icon themes; the importer's offline check passed.")
```

- [ ] **Step 5: Run the helper, then regenerate the Windows XP manifest.**

```powershell
New-Item -ItemType Directory -Force disposable\dm-icons | Out-Null
python disposable\dm-icons\add-docker-manager-icon.py .
node scripts/build-desktop-windows-xp-icons.mjs
git diff --stat -- ui/img/windows-xp
```

Expected: `Registered docker-manager in 4 icon themes; the importer's offline check passed.`, then
`Generated 153 preferred Windows XP icon mappings and 33 original SVG motifs.`, and the diff stat shows only
`ui/img/windows-xp/manifest.json | 1 +`. On Windows Git may also list the 33 generated XP motif SVGs as modified with
"LF will be replaced by CRLF" warnings: their content is unchanged, so restore them with
`git checkout -- ui/img/windows-xp/icons`.

- [ ] **Step 6: Run the importer checks and the guards; expect PASS.**

```powershell
python scripts/import-desktop-icon-themes.py --check
python scripts/test-desktop-icon-import.py
node scripts/test-desktop-icon-resolver.mjs
node scripts/build-desktop-windows-xp-icons.mjs --check
go test ./ui -run "TestDesktopPapirus|TestDesktopWhiteSur|TestDesktopAdditionalIcon|TestWindowsXPIcon" -count=1
```

Expected: `Verified four icon themes: ...`, `OK` (5 tests), the resolver line, `Verified 153 preferred Windows XP icon mappings ...`,
and `ok  aurago/ui`. Optional, only if you have all four archives named in `assets/desktop-icon-themes/sources.json`:
`python scripts/import-desktop-icon-themes.py --check --source-dir <archive-dir>` reproduces every output byte for byte.

- [ ] **Step 7: Detect changes and commit.**

```powershell
git add ui/img/papirus/icons/docker-manager.svg ui/img/papirus/manifest.json ui/img/whitesur/icons/docker-manager.svg ui/img/whitesur/manifest.json ui/img/windows-xp/manifest.json assets/desktop-icon-themes/supplements.json assets/desktop-icon-themes/tela.json assets/desktop-icon-themes/candy.json assets/desktop-icon-themes/fluent.json assets/desktop-icon-themes/buuf_nestort.json ui/img/tela/icons/docker-manager.svg ui/img/tela/manifest.json ui/img/tela/provenance.json ui/img/candy/icons/docker-manager.svg ui/img/candy/manifest.json ui/img/candy/provenance.json ui/img/fluent/icons/docker-manager.svg ui/img/fluent/manifest.json ui/img/fluent/provenance.json ui/img/buuf-nestort/icons/docker-manager.svg ui/img/buuf-nestort/manifest.json ui/img/buuf-nestort/provenance.json
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): add the app icon to all seven icon themes" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task C3: Strings in all 16 locales

**Files**

- Modify: `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` (append `desktop.app_docker_manager` and 191 `docker_manager.*` keys)
- Modify: `ui/i18n_lint_test.go` (register the five module files and the run-time key families)
- Test: `ui/desktop_docker_manager_i18n_test.go` (create)

**Rule for the 14 non-English, non-German locales:** every one gets a real translation of every key (no English placeholders).
The repo's guards only demand non-empty values, identical `{{placeholders}}` and the same key set; the Go test below checks
exactly that. English-identical values exist only for the six technical or brand strings listed in the Part C notes.

- [ ] **Step 1: Write the failing test.**

Create `ui/desktop_docker_manager_i18n_test.go`:

```go
package ui

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Every locale carries the same docker_manager.* keys as English, none empty, with the same {{placeholders}}.
func TestDockerManagerLocalesAreComplete(t *testing.T) {
	t.Parallel()

	placeholder := regexp.MustCompile(`\{\{(\w+)\}\}`)
	tokens := func(text string) string {
		var out []string
		for _, match := range placeholder.FindAllStringSubmatch(text, -1) {
			out = append(out, match[1])
		}
		sort.Strings(out)
		return strings.Join(out, ",")
	}
	load := func(lang string) map[string]string {
		values := map[string]string{}
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+lang+".json")), &values); err != nil {
			t.Fatalf("parse %s: %v", lang, err)
		}
		return values
	}
	owned := func(key string) bool {
		return strings.HasPrefix(key, "docker_manager.") || key == "desktop.app_docker_manager"
	}

	english := load("en")
	var keys []string
	for key := range english {
		if owned(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) < 150 {
		t.Fatalf("English has only %d Docker Manager keys", len(keys))
	}
	for _, lang := range []string{"cs", "da", "de", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values := load(lang)
		count := 0
		for key := range values {
			if owned(key) {
				count++
			}
		}
		if count != len(keys) {
			t.Errorf("%s has %d Docker Manager keys, English has %d", lang, count, len(keys))
		}
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Errorf("%s is missing %s", lang, key)
			} else if tokens(values[key]) != tokens(english[key]) {
				t.Errorf("%s changes the placeholders of %s: %q", lang, key, values[key])
			}
		}
	}
}
```

- [ ] **Step 2: Run; expect FAIL.**

```powershell
go test ./ui -run "TestDockerManagerLocalesAreComplete|TestDesktopBuiltinAppNamesExistInAllLanguages" -count=1
```

Expected: FAIL (`English has only 0 Docker Manager keys`; `missing non-empty builtin app translation for desktop.app_docker_manager`).

- [ ] **Step 3: Create the merge script** (not committed). Each row is the key followed by the 16 texts in the order
  `en, de, es, fr, it, nl, pt, pl, sv, da, no, cs, el, hi, ja, zh`. The script refuses a row with a missing text or different
  placeholders, appends the rows at the end of each locale file (LF, the file's own 2-space style) and re-parses the result.

Create `disposable\dm-i18n\merge-docker-manager-i18n.mjs`:

```js
// One-off helper, not committed: appends desktop.app_docker_manager and the docker_manager.* section to the sixteen
// ui/lang/desktop/<lang>.json files. Usage: node merge-docker-manager-i18n.mjs <repo-root>
// Every row lists the English text first, then the other fifteen languages in the order of LANGS.
import fs from 'node:fs';
import path from 'node:path';

const LANGS = ['en', 'de', 'es', 'fr', 'it', 'nl', 'pt', 'pl', 'sv', 'da', 'no', 'cs', 'el', 'hi', 'ja', 'zh'];
const same = value => LANGS.map(() => value);
const ROWS = [
    ['desktop.app_docker_manager', ...same('Docker Manager')],

    // ---- rail, toolbar, table --------------------------------------------------------------------------------------
    ['docker_manager.rail_label', 'Docker hosts', 'Docker-Hosts', 'Hosts de Docker', 'Hôtes Docker', 'Host Docker', 'Docker-hosts', 'Hosts do Docker', 'Hosty Docker', 'Docker-värdar', 'Docker-værter', 'Docker-verter', 'Hostitelé Dockeru', 'Κεντρικοί υπολογιστές Docker', 'Docker होस्ट', 'Dockerホスト', 'Docker 主机'],
    ['docker_manager.hosts_heading', 'Hosts', 'Hosts', 'Hosts', 'Hôtes', 'Host', 'Hosts', 'Hosts', 'Hosty', 'Värdar', 'Værter', 'Verter', 'Hostitelé', 'Υπολογιστές', 'होस्ट', 'ホスト', '主机'],
    ['docker_manager.all_hosts', 'All hosts', 'Alle Hosts', 'Todos los hosts', 'Tous les hôtes', 'Tutti gli host', 'Alle hosts', 'Todos os hosts', 'Wszystkie hosty', 'Alla värdar', 'Alle værter', 'Alle verter', 'Všichni hostitelé', 'Όλοι οι υπολογιστές', 'सभी होस्ट', 'すべてのホスト', '所有主机'],
    ['docker_manager.manage_hosts', 'Manage hosts…', 'Hosts verwalten…', 'Administrar hosts…', 'Gérer les hôtes…', 'Gestisci host…', 'Hosts beheren…', 'Gerenciar hosts…', 'Zarządzaj hostami…', 'Hantera värdar…', 'Administrer værter…', 'Administrer verter…', 'Spravovat hostitele…', 'Διαχείριση υπολογιστών…', 'होस्ट प्रबंधित करें…', 'ホストを管理…', '管理主机…'],
    ['docker_manager.nav_label', 'Sections', 'Bereiche', 'Secciones', 'Sections', 'Sezioni', 'Secties', 'Seções', 'Sekcje', 'Avsnitt', 'Sektioner', 'Seksjoner', 'Sekce', 'Ενότητες', 'अनुभाग', 'セクション', '分区'],
    ['docker_manager.nav_containers', 'Containers', 'Container', 'Contenedores', 'Conteneurs', 'Container', 'Containers', 'Contêineres', 'Kontenery', 'Containrar', 'Containere', 'Containere', 'Kontejnery', 'Κοντέινερ', 'कंटेनर', 'コンテナ', '容器'],
    ['docker_manager.count_running_total', '{{running}} of {{total}} running', '{{running}} von {{total}} laufen', '{{running}} de {{total}} en ejecución', '{{running}} sur {{total}} en cours', '{{running}} di {{total}} in esecuzione', '{{running}} van {{total}} actief', '{{running}} de {{total}} em execução', 'Działa {{running}} z {{total}}', '{{running}} av {{total}} körs', '{{running}} af {{total}} kører', '{{running}} av {{total}} kjører', 'Běží {{running}} z {{total}}', '{{running}} από {{total}} εκτελούνται', '{{total}} में से {{running}} चल रहे हैं', '{{total}}件中{{running}}件が実行中', '{{total}} 个中有 {{running}} 个在运行'],
    ['docker_manager.host_ok', 'Reachable', 'Erreichbar', 'Accesible', 'Joignable', 'Raggiungibile', 'Bereikbaar', 'Acessível', 'Dostępny', 'Nåbar', 'Tilgængelig', 'Tilgjengelig', 'Dostupný', 'Προσβάσιμος', 'पहुँच योग्य', '接続可能', '可访问'],
    ['docker_manager.host_error', 'Unreachable', 'Nicht erreichbar', 'No accesible', 'Injoignable', 'Non raggiungibile', 'Niet bereikbaar', 'Inacessível', 'Niedostępny', 'Inte nåbar', 'Ikke tilgængelig', 'Ikke tilgjengelig', 'Nedostupný', 'Μη προσβάσιμος', 'पहुँच से बाहर', '接続できません', '无法访问'],
    ['docker_manager.host_unknown', 'Not checked yet', 'Noch nicht geprüft', 'Aún sin comprobar', 'Pas encore vérifié', 'Non ancora controllato', 'Nog niet gecontroleerd', 'Ainda não verificado', 'Jeszcze nie sprawdzono', 'Inte kontrollerad ännu', 'Ikke kontrolleret endnu', 'Ikke sjekket ennå', 'Zatím nezkontrolováno', 'Δεν έχει ελεγχθεί ακόμη', 'अभी जाँचा नहीं गया', '未確認', '尚未检查'],
    ['docker_manager.loading', 'Loading…', 'Wird geladen…', 'Cargando…', 'Chargement…', 'Caricamento…', 'Laden…', 'Carregando…', 'Wczytywanie…', 'Läser in…', 'Indlæser…', 'Laster…', 'Načítání…', 'Φόρτωση…', 'लोड हो रहा है…', '読み込み中…', '正在加载…'],
    ['docker_manager.search_placeholder', 'Search containers, images, ports…', 'Container, Images, Ports suchen…', 'Buscar contenedores, imágenes, puertos…', 'Rechercher conteneurs, images, ports…', 'Cerca container, immagini, porte…', 'Zoek containers, images, poorten…', 'Pesquisar contêineres, imagens, portas…', 'Szukaj kontenerów, obrazów, portów…', 'Sök containrar, avbilder, portar…', 'Søg i containere, images, porte…', 'Søk i containere, images, porter…', 'Hledat kontejnery, image, porty…', 'Αναζήτηση κοντέινερ, εικόνων, θυρών…', 'कंटेनर, इमेज, पोर्ट खोजें…', 'コンテナ、イメージ、ポートを検索…', '搜索容器、镜像、端口…'],
    ['docker_manager.search_label', 'Search containers', 'Container durchsuchen', 'Buscar contenedores', 'Rechercher des conteneurs', 'Cerca container', 'Containers zoeken', 'Pesquisar contêineres', 'Szukaj kontenerów', 'Sök containrar', 'Søg i containere', 'Søk i containere', 'Hledat kontejnery', 'Αναζήτηση κοντέινερ', 'कंटेनर खोजें', 'コンテナを検索', '搜索容器'],
    ['docker_manager.filter_group', 'Filter by state', 'Nach Status filtern', 'Filtrar por estado', 'Filtrer par état', 'Filtra per stato', 'Filteren op status', 'Filtrar por estado', 'Filtruj według stanu', 'Filtrera efter status', 'Filtrer efter status', 'Filtrer etter status', 'Filtrovat podle stavu', 'Φίλτρο ανά κατάσταση', 'स्थिति के अनुसार फ़िल्टर करें', '状態で絞り込む', '按状态筛选'],
    ['docker_manager.filter_all', 'All', 'Alle', 'Todos', 'Tous', 'Tutti', 'Alle', 'Todos', 'Wszystkie', 'Alla', 'Alle', 'Alle', 'Vše', 'Όλα', 'सभी', 'すべて', '全部'],
    ['docker_manager.filter_running', 'Running', 'Laufend', 'En ejecución', 'En cours', 'In esecuzione', 'Actief', 'Em execução', 'Uruchomione', 'Körs', 'Kører', 'Kjører', 'Běží', 'Σε εκτέλεση', 'चल रहे', '実行中', '运行中'],
    ['docker_manager.filter_stopped', 'Stopped', 'Gestoppt', 'Detenidos', 'Arrêtés', 'Fermati', 'Gestopt', 'Parados', 'Zatrzymane', 'Stoppade', 'Stoppet', 'Stoppet', 'Zastavené', 'Σταματημένα', 'रुके हुए', '停止中', '已停止'],
    ['docker_manager.filter_problem', 'Problems', 'Probleme', 'Problemas', 'Problèmes', 'Problemi', 'Problemen', 'Problemas', 'Problemy', 'Problem', 'Problemer', 'Problemer', 'Problémy', 'Προβλήματα', 'समस्याएँ', '問題あり', '有问题'],
    ['docker_manager.count_containers', '{{shown}} of {{total}} containers', '{{shown}} von {{total}} Containern', '{{shown}} de {{total}} contenedores', '{{shown}} conteneurs sur {{total}}', '{{shown}} di {{total}} container', '{{shown}} van {{total}} containers', '{{shown}} de {{total}} contêineres', '{{shown}} z {{total}} kontenerów', '{{shown}} av {{total}} containrar', '{{shown}} af {{total}} containere', '{{shown}} av {{total}} containere', '{{shown}} z {{total}} kontejnerů', '{{shown}} από {{total}} κοντέινερ', '{{total}} में से {{shown}} कंटेनर', '{{total}}個中{{shown}}個のコンテナ', '共 {{total}} 个容器，显示 {{shown}} 个'],
    ['docker_manager.refresh', 'Refresh', 'Aktualisieren', 'Actualizar', 'Actualiser', 'Aggiorna', 'Vernieuwen', 'Atualizar', 'Odśwież', 'Uppdatera', 'Opdater', 'Oppdater', 'Obnovit', 'Ανανέωση', 'रीफ़्रेश करें', '更新', '刷新'],
    ['docker_manager.readonly_chip', 'Read-only', 'Schreibgeschützt', 'Solo lectura', 'Lecture seule', 'Sola lettura', 'Alleen-lezen', 'Somente leitura', 'Tylko do odczytu', 'Skrivskyddat', 'Skrivebeskyttet', 'Skrivebeskyttet', 'Jen pro čtení', 'Μόνο ανάγνωση', 'केवल पढ़ने के लिए', '読み取り専用', '只读'],
    ['docker_manager.readonly_note', 'Docker is in read-only mode. Container actions are hidden.', 'Docker ist schreibgeschützt. Container-Aktionen sind ausgeblendet.', 'Docker está en modo de solo lectura. Las acciones de contenedor están ocultas.', 'Docker est en lecture seule. Les actions sur les conteneurs sont masquées.', 'Docker è in sola lettura. Le azioni sui container sono nascoste.', 'Docker staat in alleen-lezenmodus. Containeracties zijn verborgen.', 'O Docker está em modo somente leitura. As ações de contêiner estão ocultas.', 'Docker działa w trybie tylko do odczytu. Akcje na kontenerach są ukryte.', 'Docker är skrivskyddat. Containeråtgärder är dolda.', 'Docker er skrivebeskyttet. Handlinger på containere er skjult.', 'Docker er skrivebeskyttet. Handlinger for containere er skjult.', 'Docker je v režimu jen pro čtení. Akce s kontejnery jsou skryté.', 'Το Docker είναι σε λειτουργία μόνο ανάγνωσης. Οι ενέργειες κοντέινερ είναι κρυφές.', 'Docker केवल पढ़ने के मोड में है। कंटेनर क्रियाएँ छिपी हैं।', 'Dockerは読み取り専用モードです。コンテナ操作は表示されません。', 'Docker 处于只读模式，容器操作已隐藏。'],
    ['docker_manager.readonly_desktop', 'This desktop is read-only. Changes are turned off.', 'Dieser Desktop ist schreibgeschützt. Änderungen sind deaktiviert.', 'Este escritorio es de solo lectura. Los cambios están desactivados.', 'Ce bureau est en lecture seule. Les modifications sont désactivées.', 'Questo desktop è in sola lettura. Le modifiche sono disattivate.', 'Dit bureaublad is alleen-lezen. Wijzigingen zijn uitgeschakeld.', 'Esta área de trabalho é somente leitura. As alterações estão desativadas.', 'Ten pulpit jest tylko do odczytu. Zmiany są wyłączone.', 'Det här skrivbordet är skrivskyddat. Ändringar är avstängda.', 'Dette skrivebord er skrivebeskyttet. Ændringer er slået fra.', 'Dette skrivebordet er skrivebeskyttet. Endringer er slått av.', 'Tato plocha je jen pro čtení. Změny jsou vypnuté.', 'Αυτή η επιφάνεια εργασίας είναι μόνο για ανάγνωση. Οι αλλαγές είναι απενεργοποιημένες.', 'यह डेस्कटॉप केवल पढ़ने के लिए है। बदलाव बंद हैं।', 'このデスクトップは読み取り専用です。変更はできません。', '此桌面为只读，已禁用更改。'],
    ['docker_manager.col_status', 'Status', 'Status', 'Estado', 'État', 'Stato', 'Status', 'Estado', 'Stan', 'Status', 'Status', 'Status', 'Stav', 'Κατάσταση', 'स्थिति', '状態', '状态'],
    ['docker_manager.col_name', 'Name', 'Name', 'Nombre', 'Nom', 'Nome', 'Naam', 'Nome', 'Nazwa', 'Namn', 'Navn', 'Navn', 'Název', 'Όνομα', 'नाम', '名前', '名称'],
    ['docker_manager.col_image', 'Image', 'Image', 'Imagen', 'Image', 'Immagine', 'Image', 'Imagem', 'Obraz', 'Avbild', 'Image', 'Image', 'Image', 'Εικόνα', 'इमेज', 'イメージ', '镜像'],
    ['docker_manager.col_ports', 'Ports', 'Ports', 'Puertos', 'Ports', 'Porte', 'Poorten', 'Portas', 'Porty', 'Portar', 'Porte', 'Porter', 'Porty', 'Θύρες', 'पोर्ट', 'ポート', '端口'],
    ['docker_manager.col_actions', 'Actions', 'Aktionen', 'Acciones', 'Actions', 'Azioni', 'Acties', 'Ações', 'Akcje', 'Åtgärder', 'Handlinger', 'Handlinger', 'Akce', 'Ενέργειες', 'क्रियाएँ', '操作', '操作'],
    ['docker_manager.group_single', 'Single containers', 'Einzelne Container', 'Contenedores individuales', 'Conteneurs individuels', 'Container singoli', 'Losse containers', 'Contêineres avulsos', 'Pojedyncze kontenery', 'Enskilda containrar', 'Enkeltstående containere', 'Enkeltstående containere', 'Samostatné kontejnery', 'Μεμονωμένα κοντέινερ', 'अकेले कंटेनर', '単体コンテナ', '单个容器'],
    ['docker_manager.group_problems', '{{count}} with problems', '{{count}} mit Problemen', '{{count}} con problemas', '{{count}} avec des problèmes', '{{count}} con problemi', '{{count}} met problemen', '{{count}} com problemas', '{{count}} z problemami', '{{count}} med problem', '{{count}} med problemer', '{{count}} med problemer', '{{count}} s problémy', '{{count}} με προβλήματα', '{{count}} में समस्याएँ', '問題あり: {{count}}件', '{{count}} 个有问题'],
    ['docker_manager.protected_owner', 'Protected: belongs to {{owner}}', 'Geschützt: gehört zu {{owner}}', 'Protegido: pertenece a {{owner}}', 'Protégé : appartient à {{owner}}', 'Protetto: appartiene a {{owner}}', 'Beschermd: hoort bij {{owner}}', 'Protegido: pertence a {{owner}}', 'Chroniony: należy do {{owner}}', 'Skyddad: tillhör {{owner}}', 'Beskyttet: tilhører {{owner}}', 'Beskyttet: tilhører {{owner}}', 'Chráněno: patří k {{owner}}', 'Προστατευμένο: ανήκει στο {{owner}}', 'सुरक्षित: {{owner}} का है', '保護対象: {{owner}} に属しています', '受保护：属于 {{owner}}'],
    ['docker_manager.protected_self', 'Protected: this container runs AuraGo', 'Geschützt: In diesem Container läuft AuraGo', 'Protegido: este contenedor ejecuta AuraGo', 'Protégé : ce conteneur exécute AuraGo', 'Protetto: questo container esegue AuraGo', 'Beschermd: in deze container draait AuraGo', 'Protegido: este contêiner executa o AuraGo', 'Chroniony: w tym kontenerze działa AuraGo', 'Skyddad: den här containern kör AuraGo', 'Beskyttet: denne container kører AuraGo', 'Beskyttet: denne containeren kjører AuraGo', 'Chráněno: v tomto kontejneru běží AuraGo', 'Προστατευμένο: αυτό το κοντέινερ εκτελεί το AuraGo', 'सुरक्षित: यह कंटेनर AuraGo चला रहा है', '保護対象: このコンテナで AuraGo が動作しています', '受保护：此容器运行着 AuraGo'],
    ['docker_manager.protected_endpoint', 'Protected: AuraGo uses this Docker endpoint', 'Geschützt: AuraGo nutzt diesen Docker-Endpunkt', 'Protegido: AuraGo usa este endpoint de Docker', 'Protégé : AuraGo utilise ce point d’accès Docker', 'Protetto: AuraGo usa questo endpoint Docker', 'Beschermd: AuraGo gebruikt dit Docker-eindpunt', 'Protegido: o AuraGo usa este endpoint do Docker', 'Chroniony: AuraGo korzysta z tego punktu końcowego Dockera', 'Skyddad: AuraGo använder den här Docker-slutpunkten', 'Beskyttet: AuraGo bruger dette Docker-endpoint', 'Beskyttet: AuraGo bruker dette Docker-endepunktet', 'Chráněno: AuraGo používá tento koncový bod Dockeru', 'Προστατευμένο: το AuraGo χρησιμοποιεί αυτό το endpoint του Docker', 'सुरक्षित: AuraGo इस Docker एंडपॉइंट का उपयोग करता है', '保護対象: AuraGo がこの Docker エンドポイントを使用しています', '受保护：AuraGo 使用此 Docker 端点'],
    ['docker_manager.protected_network', 'Protected: shares a network with AuraGo', 'Geschützt: teilt ein Netzwerk mit AuraGo', 'Protegido: comparte una red con AuraGo', 'Protégé : partage un réseau avec AuraGo', 'Protetto: condivide una rete con AuraGo', 'Beschermd: deelt een netwerk met AuraGo', 'Protegido: compartilha uma rede com o AuraGo', 'Chroniony: współdzieli sieć z AuraGo', 'Skyddad: delar nätverk med AuraGo', 'Beskyttet: deler netværk med AuraGo', 'Beskyttet: deler nettverk med AuraGo', 'Chráněno: sdílí síť s AuraGo', 'Προστατευμένο: μοιράζεται δίκτυο με το AuraGo', 'सुरक्षित: AuraGo के साथ नेटवर्क साझा करता है', '保護対象: AuraGo とネットワークを共有しています', '受保护：与 AuraGo 共享网络'],

    // ---- container states ------------------------------------------------------------------------------------------
    ['docker_manager.status_running', 'Running', 'Läuft', 'En ejecución', 'En cours d’exécution', 'In esecuzione', 'Actief', 'Em execução', 'Uruchomiony', 'Körs', 'Kører', 'Kjører', 'Běží', 'Σε εκτέλεση', 'चल रहा है', '実行中', '运行中'],
    ['docker_manager.status_paused', 'Paused', 'Pausiert', 'En pausa', 'En pause', 'In pausa', 'Gepauzeerd', 'Pausado', 'Wstrzymany', 'Pausad', 'Sat på pause', 'Satt på pause', 'Pozastaven', 'Σε παύση', 'रुका हुआ', '一時停止中', '已暂停'],
    ['docker_manager.status_restarting', 'Restarting', 'Startet neu', 'Reiniciando', 'Redémarrage', 'Riavvio', 'Herstart', 'Reiniciando', 'Restartuje', 'Startar om', 'Genstarter', 'Starter på nytt', 'Restartuje se', 'Επανεκκίνηση', 'पुनः आरंभ हो रहा है', '再起動中', '正在重启'],
    ['docker_manager.status_exited_code', 'Exited ({{code}})', 'Beendet ({{code}})', 'Finalizado ({{code}})', 'Arrêté ({{code}})', 'Terminato ({{code}})', 'Beëindigd ({{code}})', 'Encerrado ({{code}})', 'Zakończony ({{code}})', 'Avslutad ({{code}})', 'Afsluttet ({{code}})', 'Avsluttet ({{code}})', 'Ukončen ({{code}})', 'Τερματίστηκε ({{code}})', 'समाप्त ({{code}})', '終了 ({{code}})', '已退出（{{code}}）'],
    ['docker_manager.status_created', 'Created', 'Erstellt', 'Creado', 'Créé', 'Creato', 'Aangemaakt', 'Criado', 'Utworzony', 'Skapad', 'Oprettet', 'Opprettet', 'Vytvořen', 'Δημιουργήθηκε', 'बनाया गया', '作成済み', '已创建'],
    ['docker_manager.status_dead', 'Dead', 'Defekt', 'Muerto', 'Mort', 'Morto', 'Dood', 'Morto', 'Martwy', 'Död', 'Død', 'Død', 'Mrtvý', 'Νεκρό', 'मृत', '異常終了', '已失效'],
    ['docker_manager.status_removing', 'Removing', 'Wird entfernt', 'Eliminando', 'Suppression', 'Rimozione', 'Wordt verwijderd', 'Removendo', 'Usuwanie', 'Tas bort', 'Fjernes', 'Fjernes', 'Odstraňuje se', 'Αφαίρεση', 'हटाया जा रहा है', '削除中', '正在移除'],
    ['docker_manager.status_unhealthy', 'Unhealthy', 'Fehlerhaft', 'No saludable', 'Défectueux', 'Non integro', 'Niet gezond', 'Não saudável', 'Niezdrowy', 'Ohälsosam', 'Usund', 'Usunn', 'Nezdravý', 'Μη υγιές', 'अस्वस्थ', '異常', '不健康'],
    ['docker_manager.status_starting', 'Starting', 'Startet', 'Iniciando', 'Démarrage', 'Avvio in corso', 'Wordt gestart', 'Iniciando', 'Uruchamianie', 'Startar', 'Starter', 'Starter', 'Spouští se', 'Εκκίνηση', 'शुरू हो रहा है', '起動中', '正在启动'],
    ['docker_manager.status_unknown', 'Unknown', 'Unbekannt', 'Desconocido', 'Inconnu', 'Sconosciuto', 'Onbekend', 'Desconhecido', 'Nieznany', 'Okänd', 'Ukendt', 'Ukjent', 'Neznámý', 'Άγνωστο', 'अज्ञात', '不明', '未知'],

    // ---- actions -----------------------------------------------------------------------------------------------------
    ['docker_manager.action_start', 'Start', 'Starten', 'Iniciar', 'Démarrer', 'Avvia', 'Starten', 'Iniciar', 'Uruchom', 'Starta', 'Start', 'Start', 'Spustit', 'Έναρξη', 'शुरू करें', '開始', '启动'],
    ['docker_manager.action_stop', 'Stop', 'Stoppen', 'Detener', 'Arrêter', 'Ferma', 'Stoppen', 'Parar', 'Zatrzymaj', 'Stoppa', 'Stop', 'Stopp', 'Zastavit', 'Διακοπή', 'रोकें', '停止', '停止'],
    ['docker_manager.action_restart', 'Restart', 'Neu starten', 'Reiniciar', 'Redémarrer', 'Riavvia', 'Herstarten', 'Reiniciar', 'Uruchom ponownie', 'Starta om', 'Genstart', 'Start på nytt', 'Restartovat', 'Επανεκκίνηση', 'पुनः आरंभ करें', '再起動', '重启'],
    ['docker_manager.action_pause', 'Pause', 'Pausieren', 'Pausar', 'Mettre en pause', 'Metti in pausa', 'Pauzeren', 'Pausar', 'Wstrzymaj', 'Pausa', 'Sæt på pause', 'Sett på pause', 'Pozastavit', 'Παύση', 'विराम दें', '一時停止', '暂停'],
    ['docker_manager.action_unpause', 'Resume', 'Fortsetzen', 'Reanudar', 'Reprendre', 'Riprendi', 'Hervatten', 'Retomar', 'Wznów', 'Återuppta', 'Genoptag', 'Gjenoppta', 'Obnovit', 'Συνέχιση', 'फिर शुरू करें', '再開', '恢复'],
    ['docker_manager.action_kill', 'Kill', 'Beenden erzwingen', 'Forzar cierre', 'Tuer', 'Termina forzatamente', 'Afbreken', 'Forçar parada', 'Zabij', 'Döda', 'Dræb', 'Drep', 'Zabít', 'Τερματισμός', 'ज़बरदस्ती बंद करें', '強制終了', '强制终止'],
    ['docker_manager.action_remove', 'Remove', 'Entfernen', 'Eliminar', 'Supprimer', 'Rimuovi', 'Verwijderen', 'Remover', 'Usuń', 'Ta bort', 'Fjern', 'Fjern', 'Odebrat', 'Αφαίρεση', 'हटाएँ', '削除', '移除'],
    ['docker_manager.action_logs', 'Logs', 'Logs', 'Registros', 'Journaux', 'Log', 'Logboek', 'Logs', 'Logi', 'Loggar', 'Logs', 'Logger', 'Logy', 'Αρχεία καταγραφής', 'लॉग', 'ログ', '日志'],
    ['docker_manager.action_inspect', 'Inspect', 'Untersuchen', 'Inspeccionar', 'Inspecter', 'Ispeziona', 'Inspecteren', 'Inspecionar', 'Sprawdź', 'Inspektera', 'Inspicér', 'Inspiser', 'Prozkoumat', 'Επιθεώρηση', 'जाँचें', '詳細', '检查'],
    ['docker_manager.action_more', 'More actions', 'Weitere Aktionen', 'Más acciones', 'Plus d’actions', 'Altre azioni', 'Meer acties', 'Mais ações', 'Więcej akcji', 'Fler åtgärder', 'Flere handlinger', 'Flere handlinger', 'Další akce', 'Περισσότερες ενέργειες', 'और क्रियाएँ', 'その他の操作', '更多操作'],
    ['docker_manager.action_open_details', 'Open details', 'Details öffnen', 'Abrir detalles', 'Ouvrir les détails', 'Apri dettagli', 'Details openen', 'Abrir detalhes', 'Otwórz szczegóły', 'Öppna detaljer', 'Åbn detaljer', 'Åpne detaljer', 'Otevřít podrobnosti', 'Άνοιγμα λεπτομερειών', 'विवरण खोलें', '詳細を開く', '打开详情'],
    ['docker_manager.row_action_label', ...same('{{action}}: {{name}}')],
    ['docker_manager.row_actions_label', 'Actions for {{name}}', 'Aktionen für {{name}}', 'Acciones de {{name}}', 'Actions pour {{name}}', 'Azioni per {{name}}', 'Acties voor {{name}}', 'Ações para {{name}}', 'Akcje dla {{name}}', 'Åtgärder för {{name}}', 'Handlinger for {{name}}', 'Handlinger for {{name}}', 'Akce pro {{name}}', 'Ενέργειες για {{name}}', '{{name}} के लिए क्रियाएँ', '{{name}} の操作', '{{name}} 的操作'],
    ['docker_manager.action_done', '{{action}} succeeded for {{name}}', '{{action}} für {{name}} erfolgreich', '{{action}} completado para {{name}}', '{{action}} réussi pour {{name}}', '{{action}} riuscito per {{name}}', '{{action}} geslaagd voor {{name}}', '{{action}} concluído para {{name}}', '{{action}} zakończone dla {{name}}', '{{action}} lyckades för {{name}}', '{{action}} lykkedes for {{name}}', '{{action}} lyktes for {{name}}', '{{action}} proběhlo úspěšně pro {{name}}', 'Η ενέργεια «{{action}}» ολοκληρώθηκε για {{name}}', '{{name}} के लिए {{action}} सफल रहा', '{{name}} の{{action}}に成功しました', '{{name}}：{{action}}成功'],
    ['docker_manager.action_failed', '{{action}} failed for {{name}}: {{message}}', '{{action}} für {{name}} fehlgeschlagen: {{message}}', '{{action}} falló para {{name}}: {{message}}', 'Échec de {{action}} pour {{name}} : {{message}}', '{{action}} non riuscito per {{name}}: {{message}}', '{{action}} mislukt voor {{name}}: {{message}}', '{{action}} falhou para {{name}}: {{message}}', '{{action}} nie powiodło się dla {{name}}: {{message}}', '{{action}} misslyckades för {{name}}: {{message}}', '{{action}} mislykkedes for {{name}}: {{message}}', '{{action}} mislyktes for {{name}}: {{message}}', '{{action}} se nezdařilo pro {{name}}: {{message}}', 'Η ενέργεια «{{action}}» απέτυχε για {{name}}: {{message}}', '{{name}} के लिए {{action}} विफल रहा: {{message}}', '{{name}} の{{action}}に失敗しました: {{message}}', '{{name}}：{{action}}失败：{{message}}'],

    // ---- confirmations -----------------------------------------------------------------------------------------------
    ['docker_manager.remove_title', 'Remove container', 'Container entfernen', 'Eliminar contenedor', 'Supprimer le conteneur', 'Rimuovi container', 'Container verwijderen', 'Remover contêiner', 'Usuń kontener', 'Ta bort container', 'Fjern container', 'Fjern container', 'Odebrat kontejner', 'Αφαίρεση κοντέινερ', 'कंटेनर हटाएँ', 'コンテナを削除', '移除容器'],
    ['docker_manager.remove_body', 'Remove {{name}} from {{host}}? Its anonymous volumes are removed with it. This cannot be undone.', '{{name}} auf {{host}} entfernen? Seine anonymen Volumes werden mitgelöscht. Das lässt sich nicht rückgängig machen.', '¿Eliminar {{name}} de {{host}}? Sus volúmenes anónimos se eliminan con él. Esta acción no se puede deshacer.', 'Supprimer {{name}} de {{host}} ? Ses volumes anonymes sont supprimés avec lui. Cette action est irréversible.', 'Rimuovere {{name}} da {{host}}? I suoi volumi anonimi vengono rimossi insieme a esso. L’operazione non può essere annullata.', '{{name}} van {{host}} verwijderen? De anonieme volumes worden mee verwijderd. Dit kan niet ongedaan worden gemaakt.', 'Remover {{name}} de {{host}}? Os volumes anônimos dele são removidos junto. Isso não pode ser desfeito.', 'Usunąć {{name}} z {{host}}? Jego anonimowe woluminy zostaną usunięte razem z nim. Tej operacji nie można cofnąć.', 'Ta bort {{name}} från {{host}}? Dess anonyma volymer tas bort tillsammans med den. Det går inte att ångra.', 'Fjerne {{name}} fra {{host}}? Dens anonyme volumener fjernes sammen med den. Det kan ikke fortrydes.', 'Fjerne {{name}} fra {{host}}? De anonyme volumene fjernes sammen med den. Dette kan ikke angres.', 'Odebrat {{name}} z {{host}}? Spolu s ním se odeberou i jeho anonymní svazky. Akci nelze vrátit zpět.', 'Να αφαιρεθεί το {{name}} από το {{host}}; Οι ανώνυμοι τόμοι του αφαιρούνται μαζί του. Η ενέργεια δεν αναιρείται.', '{{host}} से {{name}} हटाएँ? इसके अनाम वॉल्यूम भी साथ में हट जाएँगे। इसे पूर्ववत नहीं किया जा सकता।', '{{host}} から {{name}} を削除しますか？匿名ボリュームも一緒に削除されます。この操作は元に戻せません。', '要从 {{host}} 移除 {{name}} 吗？其匿名卷也会一并删除，此操作无法撤销。'],
    ['docker_manager.remove_body_running', '{{name}} on {{host}} is still running. It will be stopped and removed, together with its anonymous volumes. This cannot be undone.', '{{name}} auf {{host}} läuft noch. Es wird gestoppt und samt anonymen Volumes entfernt. Das lässt sich nicht rückgängig machen.', '{{name}} en {{host}} sigue en ejecución. Se detendrá y se eliminará junto con sus volúmenes anónimos. Esta acción no se puede deshacer.', '{{name}} sur {{host}} est toujours en cours d’exécution. Il sera arrêté puis supprimé, avec ses volumes anonymes. Cette action est irréversible.', '{{name}} su {{host}} è ancora in esecuzione. Verrà fermato e rimosso insieme ai suoi volumi anonimi. L’operazione non può essere annullata.', '{{name}} op {{host}} is nog actief. Het wordt gestopt en samen met de anonieme volumes verwijderd. Dit kan niet ongedaan worden gemaakt.', '{{name}} em {{host}} ainda está em execução. Ele será parado e removido junto com os volumes anônimos. Isso não pode ser desfeito.', '{{name}} na {{host}} nadal działa. Zostanie zatrzymany i usunięty razem z anonimowymi woluminami. Tej operacji nie można cofnąć.', '{{name}} på {{host}} körs fortfarande. Den stoppas och tas bort tillsammans med sina anonyma volymer. Det går inte att ångra.', '{{name}} på {{host}} kører stadig. Den stoppes og fjernes sammen med sine anonyme volumener. Det kan ikke fortrydes.', '{{name}} på {{host}} kjører fortsatt. Den stoppes og fjernes sammen med de anonyme volumene. Dette kan ikke angres.', '{{name}} na {{host}} stále běží. Bude zastaven a odebrán spolu s anonymními svazky. Akci nelze vrátit zpět.', 'Το {{name}} στο {{host}} εκτελείται ακόμη. Θα σταματήσει και θα αφαιρεθεί μαζί με τους ανώνυμους τόμους του. Η ενέργεια δεν αναιρείται.', '{{host}} पर {{name}} अभी चल रहा है। इसे रोककर इसके अनाम वॉल्यूम सहित हटा दिया जाएगा। इसे पूर्ववत नहीं किया जा सकता।', '{{host}} 上の {{name}} はまだ実行中です。停止して、匿名ボリュームとともに削除します。この操作は元に戻せません。', '{{host}} 上的 {{name}} 仍在运行。它将被停止并与其匿名卷一起移除，此操作无法撤销。'],
    ['docker_manager.protected_confirm_title', 'Protected container', 'Geschützter Container', 'Contenedor protegido', 'Conteneur protégé', 'Container protetto', 'Beschermde container', 'Contêiner protegido', 'Chroniony kontener', 'Skyddad container', 'Beskyttet container', 'Beskyttet container', 'Chráněný kontejner', 'Προστατευμένο κοντέινερ', 'सुरक्षित कंटेनर', '保護されたコンテナ', '受保护的容器'],
    ['docker_manager.protected_confirm_body', '{{name}} is protected. “{{action}}” can disturb AuraGo or the services around it. Continue?', '{{name}} ist geschützt. „{{action}}“ kann AuraGo oder die umliegenden Dienste stören. Fortfahren?', '{{name}} está protegido. «{{action}}» puede afectar a AuraGo o a los servicios que lo rodean. ¿Continuar?', '{{name}} est protégé. « {{action}} » peut perturber AuraGo ou les services associés. Continuer ?', '{{name}} è protetto. «{{action}}» può disturbare AuraGo o i servizi collegati. Continuare?', '{{name}} is beschermd. ‘{{action}}’ kan AuraGo of de omliggende diensten verstoren. Doorgaan?', '{{name}} está protegido. “{{action}}” pode afetar o AuraGo ou os serviços ao redor. Continuar?', '{{name}} jest chroniony. Akcja „{{action}}” może zakłócić działanie AuraGo lub powiązanych usług. Kontynuować?', '{{name}} är skyddad. ”{{action}}” kan störa AuraGo eller tjänsterna runt omkring. Fortsätta?', '{{name}} er beskyttet. “{{action}}” kan forstyrre AuraGo eller de omkringliggende tjenester. Fortsæt?', '{{name}} er beskyttet. «{{action}}» kan forstyrre AuraGo eller tjenestene rundt. Fortsette?', '{{name}} je chráněn. Akce „{{action}}“ může narušit AuraGo nebo okolní služby. Pokračovat?', 'Το {{name}} είναι προστατευμένο. Η ενέργεια «{{action}}» μπορεί να επηρεάσει το AuraGo ή τις υπηρεσίες γύρω του. Συνέχεια;', '{{name}} सुरक्षित है। “{{action}}” से AuraGo या आसपास की सेवाएँ प्रभावित हो सकती हैं। जारी रखें?', '{{name}} は保護されています。「{{action}}」は AuraGo や周辺のサービスに影響する可能性があります。続行しますか？', '{{name}} 受到保护。“{{action}}”可能会影响 AuraGo 或其周边服务。要继续吗？'],
    ['docker_manager.host_delete_title', 'Delete host', 'Host löschen', 'Eliminar host', 'Supprimer l’hôte', 'Elimina host', 'Host verwijderen', 'Excluir host', 'Usuń hosta', 'Ta bort värd', 'Slet vært', 'Slett vert', 'Odstranit hostitele', 'Διαγραφή υπολογιστή', 'होस्ट हटाएँ', 'ホストを削除', '删除主机'],
    ['docker_manager.host_delete_body', 'Delete {{name}} from Docker Manager? Its stored credentials are removed too. The containers on that host are not touched.', '{{name}} aus dem Docker Manager löschen? Die gespeicherten Zugangsdaten werden ebenfalls gelöscht. Die Container auf diesem Host bleiben unberührt.', '¿Eliminar {{name}} de Docker Manager? También se eliminan sus credenciales guardadas. Los contenedores de ese host no se tocan.', 'Supprimer {{name}} de Docker Manager ? Ses identifiants enregistrés sont également supprimés. Les conteneurs de cet hôte ne sont pas modifiés.', 'Eliminare {{name}} da Docker Manager? Vengono rimosse anche le credenziali salvate. I container su quell’host non vengono toccati.', '{{name}} uit Docker Manager verwijderen? De opgeslagen inloggegevens worden ook verwijderd. De containers op die host blijven ongemoeid.', 'Excluir {{name}} do Docker Manager? As credenciais armazenadas também são removidas. Os contêineres desse host não são alterados.', 'Usunąć {{name}} z Docker Managera? Zapisane dane logowania także zostaną usunięte. Kontenery na tym hoście nie zostaną naruszone.', 'Ta bort {{name}} från Docker Manager? De sparade inloggningsuppgifterna tas också bort. Containrarna på värden berörs inte.', 'Slet {{name}} fra Docker Manager? De gemte loginoplysninger fjernes også. Containerne på værten berøres ikke.', 'Slett {{name}} fra Docker Manager? De lagrede påloggingsopplysningene fjernes også. Containerne på verten berøres ikke.', 'Odstranit {{name}} z Docker Manageru? Uložené přihlašovací údaje se také odstraní. Kontejnery na tomto hostiteli zůstanou beze změny.', 'Να διαγραφεί το {{name}} από το Docker Manager; Θα αφαιρεθούν και τα αποθηκευμένα διαπιστευτήριά του. Τα κοντέινερ σε αυτόν τον υπολογιστή δεν θα επηρεαστούν.', 'Docker Manager से {{name}} हटाएँ? सहेजे गए क्रेडेंशियल भी हट जाएँगे। उस होस्ट के कंटेनर प्रभावित नहीं होंगे।', 'Docker Manager から {{name}} を削除しますか？保存済みの認証情報も削除されます。そのホスト上のコンテナには影響しません。', '要从 Docker Manager 中删除 {{name}} 吗？其已保存的凭据也会被删除，该主机上的容器不受影响。'],

    // ---- detail drawer -----------------------------------------------------------------------------------------------
    ['docker_manager.drawer_close', 'Close details', 'Details schließen', 'Cerrar detalles', 'Fermer les détails', 'Chiudi dettagli', 'Details sluiten', 'Fechar detalhes', 'Zamknij szczegóły', 'Stäng detaljer', 'Luk detaljer', 'Lukk detaljer', 'Zavřít podrobnosti', 'Κλείσιμο λεπτομερειών', 'विवरण बंद करें', '詳細を閉じる', '关闭详情'],
    ['docker_manager.tabs_label', 'Container views', 'Container-Ansichten', 'Vistas del contenedor', 'Vues du conteneur', 'Viste del container', 'Containerweergaven', 'Visualizações do contêiner', 'Widoki kontenera', 'Containervyer', 'Containervisninger', 'Containervisninger', 'Zobrazení kontejneru', 'Προβολές κοντέινερ', 'कंटेनर दृश्य', 'コンテナの表示', '容器视图'],
    ['docker_manager.tab_overview', 'Overview', 'Übersicht', 'Resumen', 'Aperçu', 'Panoramica', 'Overzicht', 'Visão geral', 'Przegląd', 'Översikt', 'Oversigt', 'Oversikt', 'Přehled', 'Επισκόπηση', 'अवलोकन', '概要', '概览'],
    ['docker_manager.tab_logs', 'Logs', 'Logs', 'Registros', 'Journaux', 'Log', 'Logboek', 'Logs', 'Logi', 'Loggar', 'Logs', 'Logger', 'Logy', 'Αρχεία καταγραφής', 'लॉग', 'ログ', '日志'],
    ['docker_manager.tab_inspect', 'Inspect', 'Inspektion', 'Inspección', 'Inspection', 'Ispezione', 'Inspectie', 'Inspeção', 'Inspekcja', 'Inspektion', 'Inspektion', 'Inspeksjon', 'Inspekce', 'Επιθεώρηση', 'निरीक्षण', '詳細データ', '检查'],
    ['docker_manager.ov_state', 'State', 'Zustand', 'Estado', 'État', 'Stato', 'Status', 'Estado', 'Stan', 'Tillstånd', 'Tilstand', 'Tilstand', 'Stav', 'Κατάσταση', 'स्थिति', '状態', '状态'],
    ['docker_manager.ov_host', 'Host', 'Host', 'Host', 'Hôte', 'Host', 'Host', 'Host', 'Host', 'Värd', 'Vært', 'Vert', 'Hostitel', 'Υπολογιστής', 'होस्ट', 'ホスト', '主机'],
    ['docker_manager.ov_id', ...same('ID')],
    ['docker_manager.ov_created', 'Created', 'Erstellt', 'Creado', 'Créé', 'Creato', 'Aangemaakt', 'Criado', 'Utworzono', 'Skapad', 'Oprettet', 'Opprettet', 'Vytvořeno', 'Δημιουργήθηκε', 'बनाया गया', '作成日時', '创建时间'],
    ['docker_manager.ov_project', 'Compose project', 'Compose-Projekt', 'Proyecto de Compose', 'Projet Compose', 'Progetto Compose', 'Compose-project', 'Projeto do Compose', 'Projekt Compose', 'Compose-projekt', 'Compose-projekt', 'Compose-prosjekt', 'Projekt Compose', 'Έργο Compose', 'Compose प्रोजेक्ट', 'Compose プロジェクト', 'Compose 项目'],
    ['docker_manager.ov_service', 'Service', 'Dienst', 'Servicio', 'Service', 'Servizio', 'Service', 'Serviço', 'Usługa', 'Tjänst', 'Tjeneste', 'Tjeneste', 'Služba', 'Υπηρεσία', 'सेवा', 'サービス', '服务'],
    ['docker_manager.ov_workdir', 'Working directory', 'Arbeitsverzeichnis', 'Directorio de trabajo', 'Répertoire de travail', 'Directory di lavoro', 'Werkmap', 'Diretório de trabalho', 'Katalog roboczy', 'Arbetskatalog', 'Arbejdsmappe', 'Arbeidsmappe', 'Pracovní adresář', 'Κατάλογος εργασίας', 'कार्य निर्देशिका', '作業ディレクトリ', '工作目录'],
    ['docker_manager.ov_networks', 'Networks', 'Netzwerke', 'Redes', 'Réseaux', 'Reti', 'Netwerken', 'Redes', 'Sieci', 'Nätverk', 'Netværk', 'Nettverk', 'Sítě', 'Δίκτυα', 'नेटवर्क', 'ネットワーク', '网络'],
    ['docker_manager.ov_mounts', 'Mounts', 'Einhängepunkte', 'Montajes', 'Montages', 'Mount', 'Koppelingen', 'Montagens', 'Montowania', 'Monteringar', 'Monteringer', 'Monteringer', 'Připojení', 'Προσαρτήσεις', 'माउंट', 'マウント', '挂载'],
    ['docker_manager.ov_none', 'None', 'Keine', 'Ninguno', 'Aucun', 'Nessuno', 'Geen', 'Nenhum', 'Brak', 'Inga', 'Ingen', 'Ingen', 'Žádné', 'Κανένα', 'कोई नहीं', 'なし', '无'],
    ['docker_manager.logs_tail', 'Lines', 'Zeilen', 'Líneas', 'Lignes', 'Righe', 'Regels', 'Linhas', 'Wiersze', 'Rader', 'Linjer', 'Linjer', 'Řádky', 'Γραμμές', 'पंक्तियाँ', '行数', '行数'],
    ['docker_manager.logs_reload', 'Reload', 'Neu laden', 'Recargar', 'Recharger', 'Ricarica', 'Opnieuw laden', 'Recarregar', 'Wczytaj ponownie', 'Läs in igen', 'Genindlæs', 'Last på nytt', 'Znovu načíst', 'Επαναφόρτωση', 'फिर लोड करें', '再読み込み', '重新加载'],
    ['docker_manager.logs_loading', 'Loading logs…', 'Logs werden geladen…', 'Cargando registros…', 'Chargement des journaux…', 'Caricamento dei log…', 'Logboek laden…', 'Carregando logs…', 'Wczytywanie logów…', 'Läser in loggar…', 'Indlæser logs…', 'Laster logger…', 'Načítání logů…', 'Φόρτωση αρχείων καταγραφής…', 'लॉग लोड हो रहे हैं…', 'ログを読み込み中…', '正在加载日志…'],
    ['docker_manager.logs_empty', 'No log output.', 'Keine Log-Ausgabe.', 'Sin salida de registro.', 'Aucune sortie de journal.', 'Nessun output nei log.', 'Geen loguitvoer.', 'Sem saída de log.', 'Brak danych w logach.', 'Ingen loggutdata.', 'Ingen log-output.', 'Ingen loggutdata.', 'Žádný výstup v logu.', 'Δεν υπάρχει έξοδος καταγραφής.', 'कोई लॉग आउटपुट नहीं।', 'ログ出力はありません。', '没有日志输出。'],
    ['docker_manager.logs_failed', 'Could not load the logs.', 'Die Logs konnten nicht geladen werden.', 'No se pudieron cargar los registros.', 'Impossible de charger les journaux.', 'Impossibile caricare i log.', 'De logs konden niet worden geladen.', 'Não foi possível carregar os logs.', 'Nie udało się wczytać logów.', 'Det gick inte att läsa in loggarna.', 'Logs kunne ikke indlæses.', 'Kunne ikke laste loggene.', 'Logy se nepodařilo načíst.', 'Δεν ήταν δυνατή η φόρτωση των αρχείων καταγραφής.', 'लॉग लोड नहीं हो सके।', 'ログを読み込めませんでした。', '无法加载日志。'],
    ['docker_manager.inspect_loading', 'Loading inspect data…', 'Inspect-Daten werden geladen…', 'Cargando datos de inspección…', 'Chargement des données d’inspection…', 'Caricamento dei dati di ispezione…', 'Inspectiegegevens laden…', 'Carregando dados de inspeção…', 'Wczytywanie danych inspekcji…', 'Läser in inspektionsdata…', 'Indlæser inspektionsdata…', 'Laster inspeksjonsdata…', 'Načítání dat inspekce…', 'Φόρτωση δεδομένων επιθεώρησης…', 'निरीक्षण डेटा लोड हो रहा है…', '詳細データを読み込み中…', '正在加载检查数据…'],
    ['docker_manager.inspect_failed', 'Could not load the inspect data.', 'Die Inspect-Daten konnten nicht geladen werden.', 'No se pudieron cargar los datos de inspección.', 'Impossible de charger les données d’inspection.', 'Impossibile caricare i dati di ispezione.', 'De inspectiegegevens konden niet worden geladen.', 'Não foi possível carregar os dados de inspeção.', 'Nie udało się wczytać danych inspekcji.', 'Det gick inte att läsa in inspektionsdata.', 'Inspektionsdata kunne ikke indlæses.', 'Kunne ikke laste inspeksjonsdata.', 'Data inspekce se nepodařilo načíst.', 'Δεν ήταν δυνατή η φόρτωση των δεδομένων επιθεώρησης.', 'निरीक्षण डेटा लोड नहीं हो सका।', '詳細データを読み込めませんでした。', '无法加载检查数据。'],
    ['docker_manager.copy', 'Copy', 'Kopieren', 'Copiar', 'Copier', 'Copia', 'Kopiëren', 'Copiar', 'Kopiuj', 'Kopiera', 'Kopiér', 'Kopier', 'Kopírovat', 'Αντιγραφή', 'कॉपी करें', 'コピー', '复制'],
    ['docker_manager.copied', 'Copied to the clipboard.', 'In die Zwischenablage kopiert.', 'Copiado al portapapeles.', 'Copié dans le presse-papiers.', 'Copiato negli appunti.', 'Naar het klembord gekopieerd.', 'Copiado para a área de transferência.', 'Skopiowano do schowka.', 'Kopierat till urklipp.', 'Kopieret til udklipsholderen.', 'Kopiert til utklippstavlen.', 'Zkopírováno do schránky.', 'Αντιγράφηκε στο πρόχειρο.', 'क्लिपबोर्ड पर कॉपी किया गया।', 'クリップボードにコピーしました。', '已复制到剪贴板。'],
    ['docker_manager.copy_failed', 'Copying failed. Select the text and copy it by hand.', 'Kopieren fehlgeschlagen. Markieren Sie den Text und kopieren Sie ihn von Hand.', 'No se pudo copiar. Selecciona el texto y cópialo manualmente.', 'La copie a échoué. Sélectionnez le texte et copiez-le manuellement.', 'Copia non riuscita. Seleziona il testo e copialo manualmente.', 'Kopiëren mislukt. Selecteer de tekst en kopieer hem handmatig.', 'Falha ao copiar. Selecione o texto e copie manualmente.', 'Kopiowanie nie powiodło się. Zaznacz tekst i skopiuj go ręcznie.', 'Det gick inte att kopiera. Markera texten och kopiera den manuellt.', 'Kopiering mislykkedes. Markér teksten og kopiér den manuelt.', 'Kopiering mislyktes. Merk teksten og kopier den manuelt.', 'Kopírování se nezdařilo. Označte text a zkopírujte ho ručně.', 'Η αντιγραφή απέτυχε. Επιλέξτε το κείμενο και αντιγράψτε το χειροκίνητα.', 'कॉपी नहीं हो सका। टेक्स्ट चुनकर हाथ से कॉपी करें।', 'コピーに失敗しました。テキストを選択して手動でコピーしてください。', '复制失败。请选中文本后手动复制。'],

    // ---- empty states, banners, reasons, errors ------------------------------------------------------------------------
    ['docker_manager.empty_title', 'No containers', 'Keine Container', 'Sin contenedores', 'Aucun conteneur', 'Nessun container', 'Geen containers', 'Nenhum contêiner', 'Brak kontenerów', 'Inga containrar', 'Ingen containere', 'Ingen containere', 'Žádné kontejnery', 'Κανένα κοντέινερ', 'कोई कंटेनर नहीं', 'コンテナがありません', '没有容器'],
    ['docker_manager.empty_body', 'This host has no containers yet.', 'Auf diesem Host gibt es noch keine Container.', 'Este host aún no tiene contenedores.', 'Cet hôte n’a pas encore de conteneurs.', 'Questo host non ha ancora container.', 'Deze host heeft nog geen containers.', 'Este host ainda não tem contêineres.', 'Ten host nie ma jeszcze kontenerów.', 'Den här värden har inga containrar än.', 'Denne vært har endnu ingen containere.', 'Denne verten har ingen containere ennå.', 'Tento hostitel zatím nemá žádné kontejnery.', 'Αυτός ο υπολογιστής δεν έχει ακόμη κοντέινερ.', 'इस होस्ट पर अभी कोई कंटेनर नहीं है।', 'このホストにはまだコンテナがありません。', '此主机上还没有容器。'],
    ['docker_manager.empty_all_body', 'None of your hosts has containers yet.', 'Auf keinem Ihrer Hosts gibt es bisher Container.', 'Ninguno de tus hosts tiene contenedores todavía.', 'Aucun de vos hôtes n’a encore de conteneurs.', 'Nessuno dei tuoi host ha ancora container.', 'Geen van je hosts heeft al containers.', 'Nenhum dos seus hosts tem contêineres ainda.', 'Żaden z Twoich hostów nie ma jeszcze kontenerów.', 'Ingen av dina värdar har några containrar än.', 'Ingen af dine værter har containere endnu.', 'Ingen av vertene dine har containere ennå.', 'Žádný z vašich hostitelů zatím nemá kontejnery.', 'Κανένας από τους υπολογιστές σας δεν έχει ακόμη κοντέινερ.', 'आपके किसी भी होस्ट पर अभी कंटेनर नहीं हैं।', 'どのホストにもまだコンテナがありません。', '您的所有主机上都还没有容器。'],
    ['docker_manager.no_match_title', 'Nothing matches', 'Keine Treffer', 'Sin coincidencias', 'Aucun résultat', 'Nessuna corrispondenza', 'Geen resultaten', 'Nada encontrado', 'Brak wyników', 'Inga träffar', 'Ingen match', 'Ingen treff', 'Nic nenalezeno', 'Δεν υπάρχουν αποτελέσματα', 'कुछ नहीं मिला', '一致するものがありません', '没有匹配项'],
    ['docker_manager.no_match_body', 'Change the search or the state filter.', 'Ändern Sie die Suche oder den Statusfilter.', 'Cambia la búsqueda o el filtro de estado.', 'Modifiez la recherche ou le filtre d’état.', 'Cambia la ricerca o il filtro di stato.', 'Pas de zoekopdracht of het statusfilter aan.', 'Altere a pesquisa ou o filtro de estado.', 'Zmień wyszukiwanie lub filtr stanu.', 'Ändra sökningen eller statusfiltret.', 'Ændr søgningen eller statusfilteret.', 'Endre søket eller statusfilteret.', 'Změňte hledání nebo filtr stavu.', 'Αλλάξτε την αναζήτηση ή το φίλτρο κατάστασης.', 'खोज या स्थिति फ़िल्टर बदलें।', '検索または状態フィルターを変更してください。', '请修改搜索或状态筛选。'],
    ['docker_manager.clear_filters', 'Clear filters', 'Filter zurücksetzen', 'Quitar filtros', 'Effacer les filtres', 'Cancella filtri', 'Filters wissen', 'Limpar filtros', 'Wyczyść filtry', 'Rensa filter', 'Ryd filtre', 'Tøm filtre', 'Zrušit filtry', 'Εκκαθάριση φίλτρων', 'फ़िल्टर साफ़ करें', 'フィルターをクリア', '清除筛选'],
    ['docker_manager.disabled_title', 'Docker is turned off', 'Docker ist ausgeschaltet', 'Docker está desactivado', 'Docker est désactivé', 'Docker è disattivato', 'Docker staat uit', 'O Docker está desativado', 'Docker jest wyłączony', 'Docker är avstängt', 'Docker er slået fra', 'Docker er slått av', 'Docker je vypnutý', 'Το Docker είναι απενεργοποιημένο', 'Docker बंद है', 'Docker は無効です', 'Docker 已关闭'],
    ['docker_manager.disabled_body', 'Turn on Docker in the settings to manage containers here.', 'Schalten Sie Docker in den Einstellungen ein, um hier Container zu verwalten.', 'Activa Docker en los ajustes para gestionar contenedores aquí.', 'Activez Docker dans les paramètres pour gérer les conteneurs ici.', 'Attiva Docker nelle impostazioni per gestire i container qui.', 'Schakel Docker in de instellingen in om hier containers te beheren.', 'Ative o Docker nas configurações para gerenciar contêineres aqui.', 'Włącz Dockera w ustawieniach, aby zarządzać tutaj kontenerami.', 'Aktivera Docker i inställningarna för att hantera containrar här.', 'Slå Docker til i indstillingerne for at administrere containere her.', 'Slå på Docker i innstillingene for å administrere containere her.', 'Zapněte Docker v nastavení, abyste zde mohli spravovat kontejnery.', 'Ενεργοποιήστε το Docker στις ρυθμίσεις για να διαχειρίζεστε κοντέινερ εδώ.', 'यहाँ कंटेनर प्रबंधित करने के लिए सेटिंग में Docker चालू करें।', 'ここでコンテナを管理するには、設定で Docker を有効にしてください。', '请在设置中开启 Docker，以便在此管理容器。'],
    ['docker_manager.open_settings', 'Open Docker settings', 'Docker-Einstellungen öffnen', 'Abrir ajustes de Docker', 'Ouvrir les paramètres Docker', 'Apri impostazioni Docker', 'Docker-instellingen openen', 'Abrir configurações do Docker', 'Otwórz ustawienia Dockera', 'Öppna Docker-inställningar', 'Åbn Docker-indstillinger', 'Åpne Docker-innstillinger', 'Otevřít nastavení Dockeru', 'Άνοιγμα ρυθμίσεων Docker', 'Docker सेटिंग खोलें', 'Docker の設定を開く', '打开 Docker 设置'],
    ['docker_manager.load_error_title', 'Could not load Docker Manager', 'Docker Manager konnte nicht geladen werden', 'No se pudo cargar Docker Manager', 'Impossible de charger Docker Manager', 'Impossibile caricare Docker Manager', 'Docker Manager kon niet worden geladen', 'Não foi possível carregar o Docker Manager', 'Nie udało się wczytać Docker Managera', 'Det gick inte att läsa in Docker Manager', 'Docker Manager kunne ikke indlæses', 'Kunne ikke laste Docker Manager', 'Docker Manager se nepodařilo načíst', 'Δεν ήταν δυνατή η φόρτωση του Docker Manager', 'Docker Manager लोड नहीं हो सका', 'Docker Manager を読み込めませんでした', '无法加载 Docker Manager'],
    ['docker_manager.banner_title', '{{host}} is not reachable', '{{host}} ist nicht erreichbar', '{{host}} no es accesible', '{{host}} est injoignable', '{{host}} non è raggiungibile', '{{host}} is niet bereikbaar', '{{host}} está inacessível', '{{host}} jest niedostępny', '{{host}} är inte nåbar', '{{host}} er ikke tilgængelig', '{{host}} er ikke tilgjengelig', '{{host}} není dostupný', 'Το {{host}} δεν είναι προσβάσιμο', '{{host}} तक पहुँच नहीं हो पा रही', '{{host}} に接続できません', '无法访问 {{host}}'],
    ['docker_manager.banner_retry', 'Retry', 'Erneut versuchen', 'Reintentar', 'Réessayer', 'Riprova', 'Opnieuw proberen', 'Tentar novamente', 'Spróbuj ponownie', 'Försök igen', 'Prøv igen', 'Prøv igjen', 'Zkusit znovu', 'Επανάληψη', 'फिर कोशिश करें', '再試行', '重试'],
    ['docker_manager.banner_details', 'Technical details', 'Technische Details', 'Detalles técnicos', 'Détails techniques', 'Dettagli tecnici', 'Technische details', 'Detalhes técnicos', 'Szczegóły techniczne', 'Tekniska detaljer', 'Tekniske detaljer', 'Tekniske detaljer', 'Technické podrobnosti', 'Τεχνικές λεπτομέρειες', 'तकनीकी विवरण', '技術的な詳細', '技术详情'],
    ['docker_manager.stale_notice', 'Showing the last data we received.', 'Es werden die zuletzt empfangenen Daten angezeigt.', 'Se muestran los últimos datos recibidos.', 'Affichage des dernières données reçues.', 'Vengono mostrati gli ultimi dati ricevuti.', 'De laatst ontvangen gegevens worden getoond.', 'Exibindo os últimos dados recebidos.', 'Wyświetlane są ostatnio odebrane dane.', 'Visar de senast mottagna uppgifterna.', 'Viser de senest modtagne data.', 'Viser de sist mottatte dataene.', 'Zobrazují se naposledy přijatá data.', 'Εμφανίζονται τα τελευταία δεδομένα που λήφθηκαν.', 'आखिरी प्राप्त डेटा दिखाया जा रहा है।', '最後に取得したデータを表示しています。', '正在显示最后收到的数据。'],
    ['docker_manager.reason_dns', 'The address could not be resolved. Check the host name.', 'Die Adresse konnte nicht aufgelöst werden. Prüfen Sie den Host-Namen.', 'No se pudo resolver la dirección. Comprueba el nombre del host.', 'L’adresse n’a pas pu être résolue. Vérifiez le nom d’hôte.', 'Impossibile risolvere l’indirizzo. Controlla il nome dell’host.', 'Het adres kon niet worden omgezet. Controleer de hostnaam.', 'Não foi possível resolver o endereço. Verifique o nome do host.', 'Nie można rozpoznać adresu. Sprawdź nazwę hosta.', 'Adressen kunde inte slås upp. Kontrollera värdnamnet.', 'Adressen kunne ikke slås op. Kontrollér værtsnavnet.', 'Adressen kunne ikke slås opp. Kontroller vertsnavnet.', 'Adresu se nepodařilo přeložit. Zkontrolujte název hostitele.', 'Δεν ήταν δυνατή η ανάλυση της διεύθυνσης. Ελέγξτε το όνομα του υπολογιστή.', 'पता हल नहीं हो सका। होस्ट का नाम जाँचें।', 'アドレスを解決できませんでした。ホスト名を確認してください。', '无法解析该地址，请检查主机名。'],
    ['docker_manager.reason_refused', 'The host refused the connection. Is Docker listening on that port?', 'Der Host hat die Verbindung abgelehnt. Lauscht Docker auf diesem Port?', 'El host rechazó la conexión. ¿Docker está escuchando en ese puerto?', 'L’hôte a refusé la connexion. Docker écoute-t-il sur ce port ?', 'L’host ha rifiutato la connessione. Docker è in ascolto su quella porta?', 'De host weigerde de verbinding. Luistert Docker op die poort?', 'O host recusou a conexão. O Docker está escutando nessa porta?', 'Host odrzucił połączenie. Czy Docker nasłuchuje na tym porcie?', 'Värden nekade anslutningen. Lyssnar Docker på den porten?', 'Værten afviste forbindelsen. Lytter Docker på den port?', 'Verten avviste tilkoblingen. Lytter Docker på den porten?', 'Hostitel odmítl spojení. Naslouchá Docker na tomto portu?', 'Ο υπολογιστής απέρριψε τη σύνδεση. Ακούει το Docker σε αυτή τη θύρα;', 'होस्ट ने कनेक्शन अस्वीकार कर दिया। क्या Docker उस पोर्ट पर सुन रहा है?', 'ホストが接続を拒否しました。Docker はそのポートで待ち受けていますか？', '主机拒绝了连接。Docker 是否在该端口上监听？'],
    ['docker_manager.reason_timeout', 'The host did not answer in time.', 'Der Host hat nicht rechtzeitig geantwortet.', 'El host no respondió a tiempo.', 'L’hôte n’a pas répondu à temps.', 'L’host non ha risposto in tempo.', 'De host reageerde niet op tijd.', 'O host não respondeu a tempo.', 'Host nie odpowiedział na czas.', 'Värden svarade inte i tid.', 'Værten svarede ikke i tide.', 'Verten svarte ikke i tide.', 'Hostitel neodpověděl včas.', 'Ο υπολογιστής δεν απάντησε εγκαίρως.', 'होस्ट ने समय पर जवाब नहीं दिया।', 'ホストが時間内に応答しませんでした。', '主机未在规定时间内响应。'],
    ['docker_manager.reason_tls', 'The TLS handshake failed. Check the certificates.', 'Der TLS-Handshake ist fehlgeschlagen. Prüfen Sie die Zertifikate.', 'Falló el protocolo de enlace TLS. Comprueba los certificados.', 'La négociation TLS a échoué. Vérifiez les certificats.', 'Handshake TLS non riuscito. Controlla i certificati.', 'De TLS-handshake is mislukt. Controleer de certificaten.', 'Falha no handshake TLS. Verifique os certificados.', 'Uzgadnianie TLS nie powiodło się. Sprawdź certyfikaty.', 'TLS-handskakningen misslyckades. Kontrollera certifikaten.', 'TLS-håndtrykket mislykkedes. Kontrollér certifikaterne.', 'TLS-håndtrykket mislyktes. Kontroller sertifikatene.', 'Navázání spojení TLS selhalo. Zkontrolujte certifikáty.', 'Η χειραψία TLS απέτυχε. Ελέγξτε τα πιστοποιητικά.', 'TLS हैंडशेक विफल रहा। प्रमाणपत्र जाँचें।', 'TLS ハンドシェイクに失敗しました。証明書を確認してください。', 'TLS 握手失败，请检查证书。'],
    ['docker_manager.reason_auth', 'Login failed. Check the user and the key or password.', 'Die Anmeldung ist fehlgeschlagen. Prüfen Sie Benutzer und Schlüssel oder Passwort.', 'Falló el inicio de sesión. Comprueba el usuario y la clave o contraseña.', 'La connexion a échoué. Vérifiez l’utilisateur et la clé ou le mot de passe.', 'Accesso non riuscito. Controlla utente e chiave o password.', 'Inloggen is mislukt. Controleer de gebruiker en de sleutel of het wachtwoord.', 'Falha no login. Verifique o usuário e a chave ou senha.', 'Logowanie nie powiodło się. Sprawdź użytkownika oraz klucz lub hasło.', 'Inloggningen misslyckades. Kontrollera användare samt nyckel eller lösenord.', 'Login mislykkedes. Kontrollér brugeren samt nøgle eller adgangskode.', 'Pålogging mislyktes. Kontroller brukeren og nøkkelen eller passordet.', 'Přihlášení se nezdařilo. Zkontrolujte uživatele a klíč nebo heslo.', 'Η σύνδεση απέτυχε. Ελέγξτε τον χρήστη και το κλειδί ή τον κωδικό.', 'लॉगिन विफल रहा। यूज़र और कुंजी या पासवर्ड जाँचें।', 'ログインに失敗しました。ユーザーと鍵またはパスワードを確認してください。', '登录失败，请检查用户以及密钥或密码。'],
    ['docker_manager.reason_ssh_host_key', 'The SSH host key was rejected.', 'Der SSH-Host-Schlüssel wurde abgelehnt.', 'Se rechazó la clave de host SSH.', 'La clé d’hôte SSH a été rejetée.', 'La chiave host SSH è stata rifiutata.', 'De SSH-hostsleutel is geweigerd.', 'A chave de host SSH foi rejeitada.', 'Klucz hosta SSH został odrzucony.', 'SSH-värdnyckeln avvisades.', 'SSH-værtsnøglen blev afvist.', 'SSH-vertsnøkkelen ble avvist.', 'Hostitelský klíč SSH byl odmítnut.', 'Το κλειδί υπολογιστή SSH απορρίφθηκε.', 'SSH होस्ट कुंजी अस्वीकार कर दी गई।', 'SSH ホスト鍵が拒否されました。', 'SSH 主机密钥被拒绝。'],
    ['docker_manager.reason_socket_missing', 'The Docker socket was not found on the host.', 'Der Docker-Socket wurde auf dem Host nicht gefunden.', 'No se encontró el socket de Docker en el host.', 'Le socket Docker est introuvable sur l’hôte.', 'Il socket Docker non è stato trovato sull’host.', 'De Docker-socket is op de host niet gevonden.', 'O socket do Docker não foi encontrado no host.', 'Nie znaleziono gniazda Dockera na hoście.', 'Docker-socketen hittades inte på värden.', 'Docker-socketten blev ikke fundet på værten.', 'Docker-socketen ble ikke funnet på verten.', 'Na hostiteli nebyl nalezen socket Dockeru.', 'Η υποδοχή (socket) του Docker δεν βρέθηκε στον υπολογιστή.', 'होस्ट पर Docker सॉकेट नहीं मिला।', 'ホスト上に Docker ソケットが見つかりません。', '在主机上找不到 Docker 套接字。'],
    ['docker_manager.reason_api_too_old', 'This Docker engine is too old. API 1.41 or newer is required.', 'Diese Docker-Engine ist zu alt. API 1.41 oder neuer wird benötigt.', 'Este motor de Docker es demasiado antiguo. Se requiere la API 1.41 o posterior.', 'Ce moteur Docker est trop ancien. L’API 1.41 ou une version plus récente est requise.', 'Questo motore Docker è troppo vecchio. È richiesta l’API 1.41 o successiva.', 'Deze Docker-engine is te oud. API 1.41 of nieuwer is vereist.', 'Este mecanismo do Docker é muito antigo. É necessária a API 1.41 ou mais recente.', 'Ten silnik Dockera jest zbyt stary. Wymagane jest API 1.41 lub nowsze.', 'Den här Docker-motorn är för gammal. API 1.41 eller senare krävs.', 'Denne Docker-motor er for gammel. API 1.41 eller nyere kræves.', 'Denne Docker-motoren er for gammel. API 1.41 eller nyere kreves.', 'Tento engine Dockeru je příliš starý. Je vyžadováno API 1.41 nebo novější.', 'Αυτή η μηχανή Docker είναι πολύ παλιά. Απαιτείται API 1.41 ή νεότερο.', 'यह Docker इंजन बहुत पुराना है। API 1.41 या नया संस्करण ज़रूरी है।', 'この Docker エンジンは古すぎます。API 1.41 以降が必要です。', '此 Docker 引擎版本过旧，需要 API 1.41 或更高版本。'],
    ['docker_manager.reason_unknown', 'The connection failed.', 'Die Verbindung ist fehlgeschlagen.', 'La conexión falló.', 'La connexion a échoué.', 'Connessione non riuscita.', 'De verbinding is mislukt.', 'A conexão falhou.', 'Połączenie nie powiodło się.', 'Anslutningen misslyckades.', 'Forbindelsen mislykkedes.', 'Tilkoblingen mislyktes.', 'Spojení se nezdařilo.', 'Η σύνδεση απέτυχε.', 'कनेक्शन विफल रहा।', '接続に失敗しました。', '连接失败。'],
    ['docker_manager.error_read_only', 'Docker is in read-only mode.', 'Docker ist schreibgeschützt.', 'Docker está en modo de solo lectura.', 'Docker est en lecture seule.', 'Docker è in sola lettura.', 'Docker staat in alleen-lezenmodus.', 'O Docker está em modo somente leitura.', 'Docker działa w trybie tylko do odczytu.', 'Docker är skrivskyddat.', 'Docker er skrivebeskyttet.', 'Docker er skrivebeskyttet.', 'Docker je v režimu jen pro čtení.', 'Το Docker είναι σε λειτουργία μόνο ανάγνωσης.', 'Docker केवल पढ़ने के मोड में है।', 'Docker は読み取り専用モードです。', 'Docker 处于只读模式。'],
    ['docker_manager.error_container_not_found', 'That container no longer exists.', 'Dieser Container existiert nicht mehr.', 'Ese contenedor ya no existe.', 'Ce conteneur n’existe plus.', 'Quel container non esiste più.', 'Die container bestaat niet meer.', 'Esse contêiner não existe mais.', 'Ten kontener już nie istnieje.', 'Containern finns inte längre.', 'Den container findes ikke længere.', 'Den containeren finnes ikke lenger.', 'Tento kontejner už neexistuje.', 'Αυτό το κοντέινερ δεν υπάρχει πια.', 'वह कंटेनर अब मौजूद नहीं है।', 'そのコンテナはもう存在しません。', '该容器已不存在。'],
    ['docker_manager.error_invalid_host', 'Please check the host settings.', 'Bitte prüfen Sie die Host-Einstellungen.', 'Comprueba la configuración del host.', 'Vérifiez les paramètres de l’hôte.', 'Controlla le impostazioni dell’host.', 'Controleer de hostinstellingen.', 'Verifique as configurações do host.', 'Sprawdź ustawienia hosta.', 'Kontrollera värdens inställningar.', 'Kontrollér værtens indstillinger.', 'Kontroller vertens innstillinger.', 'Zkontrolujte nastavení hostitele.', 'Ελέγξτε τις ρυθμίσεις του υπολογιστή.', 'होस्ट की सेटिंग जाँचें।', 'ホストの設定を確認してください。', '请检查主机设置。'],
    ['docker_manager.error_network', 'AuraGo could not be reached.', 'AuraGo ist nicht erreichbar.', 'No se pudo contactar con AuraGo.', 'AuraGo est injoignable.', 'AuraGo non è raggiungibile.', 'AuraGo is niet bereikbaar.', 'Não foi possível alcançar o AuraGo.', 'Nie można połączyć się z AuraGo.', 'AuraGo kunde inte nås.', 'AuraGo kunne ikke nås.', 'AuraGo kunne ikke nås.', 'AuraGo není dostupné.', 'Δεν ήταν δυνατή η επικοινωνία με το AuraGo.', 'AuraGo तक पहुँचा नहीं जा सका।', 'AuraGo に接続できません。', '无法连接到 AuraGo。'],
    ['docker_manager.error_timeout', 'The request took too long.', 'Die Anfrage hat zu lange gedauert.', 'La solicitud tardó demasiado.', 'La requête a pris trop de temps.', 'La richiesta ha impiegato troppo tempo.', 'Het verzoek duurde te lang.', 'A solicitação demorou demais.', 'Żądanie trwało zbyt długo.', 'Förfrågan tog för lång tid.', 'Anmodningen tog for lang tid.', 'Forespørselen tok for lang tid.', 'Požadavek trval příliš dlouho.', 'Το αίτημα καθυστέρησε πολύ.', 'अनुरोध में बहुत समय लगा।', 'リクエストに時間がかかりすぎました。', '请求耗时过长。'],
    ['docker_manager.error_forbidden', 'You are not allowed to do this.', 'Dafür fehlt Ihnen die Berechtigung.', 'No tienes permiso para hacer esto.', 'Vous n’avez pas l’autorisation de faire cela.', 'Non hai il permesso di farlo.', 'Je hebt hiervoor geen toestemming.', 'Você não tem permissão para fazer isso.', 'Nie masz uprawnień do tej czynności.', 'Du har inte behörighet att göra detta.', 'Du har ikke tilladelse til dette.', 'Du har ikke tillatelse til dette.', 'K tomu nemáte oprávnění.', 'Δεν έχετε δικαίωμα για αυτή την ενέργεια.', 'आपको यह करने की अनुमति नहीं है।', 'この操作を行う権限がありません。', '您没有执行此操作的权限。'],
    ['docker_manager.error_unknown', 'Something went wrong.', 'Etwas ist schiefgelaufen.', 'Algo salió mal.', 'Une erreur s’est produite.', 'Qualcosa è andato storto.', 'Er is iets misgegaan.', 'Algo deu errado.', 'Coś poszło nie tak.', 'Något gick fel.', 'Noget gik galt.', 'Noe gikk galt.', 'Něco se pokazilo.', 'Παρουσιάστηκε κάποιο σφάλμα.', 'कुछ गलत हो गया।', '問題が発生しました。', '出了点问题。'],

    // ---- host manager, host form, nests ----------------------------------------------------------------------------------
    ['docker_manager.hosts_intro', 'AuraGo manages Docker on the local engine and on the remote engines you add here.', 'AuraGo verwaltet Docker auf der lokalen Engine und auf den entfernten Engines, die Sie hier hinzufügen.', 'AuraGo gestiona Docker en el motor local y en los motores remotos que añadas aquí.', 'AuraGo gère Docker sur le moteur local et sur les moteurs distants que vous ajoutez ici.', 'AuraGo gestisce Docker sul motore locale e sui motori remoti che aggiungi qui.', 'AuraGo beheert Docker op de lokale engine en op de externe engines die je hier toevoegt.', 'O AuraGo gerencia o Docker no mecanismo local e nos mecanismos remotos que você adicionar aqui.', 'AuraGo zarządza Dockerem na lokalnym silniku oraz na zdalnych silnikach dodanych tutaj.', 'AuraGo hanterar Docker på den lokala motorn och på de fjärrmotorer du lägger till här.', 'AuraGo administrerer Docker på den lokale motor og på de eksterne motorer, du tilføjer her.', 'AuraGo administrerer Docker på den lokale motoren og på de eksterne motorene du legger til her.', 'AuraGo spravuje Docker na místním enginu a na vzdálených enginech, které zde přidáte.', 'Το AuraGo διαχειρίζεται το Docker στην τοπική μηχανή και στις απομακρυσμένες μηχανές που προσθέτετε εδώ.', 'AuraGo स्थानीय इंजन और यहाँ जोड़े गए रिमोट इंजनों पर Docker प्रबंधित करता है।', 'AuraGo は、ローカルエンジンと、ここで追加したリモートエンジン上の Docker を管理します。', 'AuraGo 会管理本地引擎以及您在此添加的远程引擎上的 Docker。'],
    ['docker_manager.host_add', 'Add host', 'Host hinzufügen', 'Añadir host', 'Ajouter un hôte', 'Aggiungi host', 'Host toevoegen', 'Adicionar host', 'Dodaj hosta', 'Lägg till värd', 'Tilføj vært', 'Legg til vert', 'Přidat hostitele', 'Προσθήκη υπολογιστή', 'होस्ट जोड़ें', 'ホストを追加', '添加主机'],
    ['docker_manager.host_edit_title', 'Edit host', 'Host bearbeiten', 'Editar host', 'Modifier l’hôte', 'Modifica host', 'Host bewerken', 'Editar host', 'Edytuj hosta', 'Redigera värd', 'Rediger vært', 'Rediger vert', 'Upravit hostitele', 'Επεξεργασία υπολογιστή', 'होस्ट संपादित करें', 'ホストを編集', '编辑主机'],
    ['docker_manager.host_import', 'Import from nest…', 'Aus Nest importieren…', 'Importar desde un nido…', 'Importer depuis un nid…', 'Importa da un nido…', 'Importeren uit nest…', 'Importar de um ninho…', 'Importuj z gniazda…', 'Importera från bo…', 'Importér fra rede…', 'Importer fra rede…', 'Importovat z hnízda…', 'Εισαγωγή από φωλιά…', 'nest से आयात करें…', 'ネストからインポート…', '从巢穴导入…'],
    ['docker_manager.host_edit', 'Edit', 'Bearbeiten', 'Editar', 'Modifier', 'Modifica', 'Bewerken', 'Editar', 'Edytuj', 'Redigera', 'Rediger', 'Rediger', 'Upravit', 'Επεξεργασία', 'संपादित करें', '編集', '编辑'],
    ['docker_manager.host_test', 'Test', 'Testen', 'Probar', 'Tester', 'Prova', 'Testen', 'Testar', 'Testuj', 'Testa', 'Test', 'Test', 'Otestovat', 'Δοκιμή', 'परीक्षण', 'テスト', '测试'],
    ['docker_manager.host_delete', 'Delete', 'Löschen', 'Eliminar', 'Supprimer', 'Elimina', 'Verwijderen', 'Excluir', 'Usuń', 'Ta bort', 'Slet', 'Slett', 'Odstranit', 'Διαγραφή', 'हटाएँ', '削除', '删除'],
    ['docker_manager.host_builtin', 'Built in', 'Eingebaut', 'Integrado', 'Intégré', 'Integrato', 'Ingebouwd', 'Integrado', 'Wbudowany', 'Inbyggd', 'Indbygget', 'Innebygd', 'Vestavěný', 'Ενσωματωμένο', 'अंतर्निहित', '組み込み', '内置'],
    ['docker_manager.host_plain_badge', 'Unencrypted', 'Unverschlüsselt', 'Sin cifrar', 'Non chiffré', 'Non cifrato', 'Onversleuteld', 'Sem criptografia', 'Bez szyfrowania', 'Okrypterad', 'Ukrypteret', 'Ukryptert', 'Nešifrováno', 'Χωρίς κρυπτογράφηση', 'अनएन्क्रिप्टेड', '暗号化なし', '未加密'],
    ['docker_manager.host_secret_set', 'Credentials stored', 'Zugangsdaten gespeichert', 'Credenciales guardadas', 'Identifiants enregistrés', 'Credenziali salvate', 'Inloggegevens opgeslagen', 'Credenciais armazenadas', 'Dane logowania zapisane', 'Inloggningsuppgifter sparade', 'Loginoplysninger gemt', 'Påloggingsopplysninger lagret', 'Přihlašovací údaje uloženy', 'Τα διαπιστευτήρια είναι αποθηκευμένα', 'क्रेडेंशियल सहेजे गए', '認証情報を保存済み', '凭据已保存'],
    ['docker_manager.host_engine', ...same('Docker {{version}}')],
    ['docker_manager.host_deleted', 'Host {{name}} deleted', 'Host {{name}} gelöscht', 'Host {{name}} eliminado', 'Hôte {{name}} supprimé', 'Host {{name}} eliminato', 'Host {{name}} verwijderd', 'Host {{name}} excluído', 'Usunięto hosta {{name}}', 'Värden {{name}} har tagits bort', 'Værten {{name}} er slettet', 'Verten {{name}} er slettet', 'Hostitel {{name}} byl odstraněn', 'Ο υπολογιστής {{name}} διαγράφηκε', 'होस्ट {{name}} हटाया गया', 'ホスト {{name}} を削除しました', '已删除主机 {{name}}'],
    ['docker_manager.host_saved', 'Host {{name}} saved', 'Host {{name}} gespeichert', 'Host {{name}} guardado', 'Hôte {{name}} enregistré', 'Host {{name}} salvato', 'Host {{name}} opgeslagen', 'Host {{name}} salvo', 'Zapisano hosta {{name}}', 'Värden {{name}} har sparats', 'Værten {{name}} er gemt', 'Verten {{name}} er lagret', 'Hostitel {{name}} byl uložen', 'Ο υπολογιστής {{name}} αποθηκεύτηκε', 'होस्ट {{name}} सहेजा गया', 'ホスト {{name}} を保存しました', '已保存主机 {{name}}'],
    ['docker_manager.kind_local', 'Local engine', 'Lokale Engine', 'Motor local', 'Moteur local', 'Motore locale', 'Lokale engine', 'Mecanismo local', 'Lokalny silnik', 'Lokal motor', 'Lokal motor', 'Lokal motor', 'Místní engine', 'Τοπική μηχανή', 'स्थानीय इंजन', 'ローカルエンジン', '本地引擎'],
    ['docker_manager.kind_tcp', 'TCP (unencrypted)', 'TCP (unverschlüsselt)', 'TCP (sin cifrar)', 'TCP (non chiffré)', 'TCP (non cifrato)', 'TCP (onversleuteld)', 'TCP (sem criptografia)', 'TCP (bez szyfrowania)', 'TCP (okrypterad)', 'TCP (ukrypteret)', 'TCP (ukryptert)', 'TCP (nešifrované)', 'TCP (χωρίς κρυπτογράφηση)', 'TCP (अनएन्क्रिप्टेड)', 'TCP（暗号化なし）', 'TCP（未加密）'],
    ['docker_manager.kind_tls', ...same('TLS')],
    ['docker_manager.kind_mtls', 'TLS with client certificate', 'TLS mit Client-Zertifikat', 'TLS con certificado de cliente', 'TLS avec certificat client', 'TLS con certificato client', 'TLS met clientcertificaat', 'TLS com certificado de cliente', 'TLS z certyfikatem klienta', 'TLS med klientcertifikat', 'TLS med klientcertifikat', 'TLS med klientsertifikat', 'TLS s klientským certifikátem', 'TLS με πιστοποιητικό πελάτη', 'क्लाइंट प्रमाणपत्र के साथ TLS', 'クライアント証明書付き TLS', '带客户端证书的 TLS'],
    ['docker_manager.kind_ssh', ...same('SSH')],
    ['docker_manager.f_kind', 'Connection', 'Verbindung', 'Conexión', 'Connexion', 'Connessione', 'Verbinding', 'Conexão', 'Połączenie', 'Anslutning', 'Forbindelse', 'Tilkobling', 'Připojení', 'Σύνδεση', 'कनेक्शन', '接続方法', '连接方式'],
    ['docker_manager.f_port', 'Port', 'Port', 'Puerto', 'Port', 'Porta', 'Poort', 'Porta', 'Port', 'Port', 'Port', 'Port', 'Port', 'Θύρα', 'पोर्ट', 'ポート', '端口'],
    ['docker_manager.f_address', 'Address', 'Adresse', 'Dirección', 'Adresse', 'Indirizzo', 'Adres', 'Endereço', 'Adres', 'Adress', 'Adresse', 'Adresse', 'Adresa', 'Διεύθυνση', 'पता', 'アドレス', '地址'],
    ['docker_manager.f_address_hint', 'Host name or IP address, without http:// or a path.', 'Host-Name oder IP-Adresse, ohne http:// und ohne Pfad.', 'Nombre de host o dirección IP, sin http:// ni ruta.', 'Nom d’hôte ou adresse IP, sans http:// ni chemin.', 'Nome host o indirizzo IP, senza http:// né percorso.', 'Hostnaam of IP-adres, zonder http:// of pad.', 'Nome do host ou endereço IP, sem http:// nem caminho.', 'Nazwa hosta lub adres IP, bez http:// i ścieżki.', 'Värdnamn eller IP-adress, utan http:// eller sökväg.', 'Værtsnavn eller IP-adresse, uden http:// eller sti.', 'Vertsnavn eller IP-adresse, uten http:// eller sti.', 'Název hostitele nebo IP adresa, bez http:// a cesty.', 'Όνομα υπολογιστή ή διεύθυνση IP, χωρίς http:// και διαδρομή.', 'होस्ट नाम या IP पता, बिना http:// और पथ के।', 'ホスト名または IP アドレス（http:// やパスは不要）。', '主机名或 IP 地址，不含 http:// 和路径。'],
    ['docker_manager.f_ssh_user', 'SSH user', 'SSH-Benutzer', 'Usuario SSH', 'Utilisateur SSH', 'Utente SSH', 'SSH-gebruiker', 'Usuário SSH', 'Użytkownik SSH', 'SSH-användare', 'SSH-bruger', 'SSH-bruker', 'Uživatel SSH', 'Χρήστης SSH', 'SSH उपयोगकर्ता', 'SSH ユーザー', 'SSH 用户'],
    ['docker_manager.f_ssh_password', 'SSH password', 'SSH-Passwort', 'Contraseña SSH', 'Mot de passe SSH', 'Password SSH', 'SSH-wachtwoord', 'Senha SSH', 'Hasło SSH', 'SSH-lösenord', 'SSH-adgangskode', 'SSH-passord', 'Heslo SSH', 'Κωδικός SSH', 'SSH पासवर्ड', 'SSH パスワード', 'SSH 密码'],
    ['docker_manager.f_ssh_key', 'Or SSH private key (PEM)', 'Oder privater SSH-Schlüssel (PEM)', 'O clave privada SSH (PEM)', 'Ou clé privée SSH (PEM)', 'Oppure chiave privata SSH (PEM)', 'Of privé SSH-sleutel (PEM)', 'Ou chave privada SSH (PEM)', 'Albo klucz prywatny SSH (PEM)', 'Eller privat SSH-nyckel (PEM)', 'Eller privat SSH-nøgle (PEM)', 'Eller privat SSH-nøkkel (PEM)', 'Nebo soukromý klíč SSH (PEM)', 'Ή ιδιωτικό κλειδί SSH (PEM)', 'या SSH निजी कुंजी (PEM)', 'または SSH 秘密鍵（PEM）', '或 SSH 私钥（PEM）'],
    ['docker_manager.f_secret_stored', 'Credentials are stored. Leave the fields empty to keep them.', 'Zugangsdaten sind gespeichert. Lassen Sie die Felder leer, um sie zu behalten.', 'Las credenciales están guardadas. Deja los campos vacíos para conservarlas.', 'Les identifiants sont enregistrés. Laissez les champs vides pour les conserver.', 'Le credenziali sono salvate. Lascia vuoti i campi per mantenerle.', 'Inloggegevens zijn opgeslagen. Laat de velden leeg om ze te behouden.', 'As credenciais estão armazenadas. Deixe os campos vazios para mantê-las.', 'Dane logowania są zapisane. Zostaw pola puste, aby je zachować.', 'Inloggningsuppgifterna är sparade. Lämna fälten tomma för att behålla dem.', 'Loginoplysningerne er gemt. Lad felterne stå tomme for at beholde dem.', 'Påloggingsopplysningene er lagret. La feltene stå tomme for å beholde dem.', 'Přihlašovací údaje jsou uloženy. Nechte pole prázdná, aby zůstaly zachovány.', 'Τα διαπιστευτήρια είναι αποθηκευμένα. Αφήστε τα πεδία κενά για να τα διατηρήσετε.', 'क्रेडेंशियल सहेजे हुए हैं। उन्हें रखने के लिए फ़ील्ड खाली छोड़ें।', '認証情報は保存済みです。そのまま使う場合は空欄にしてください。', '凭据已保存。如需保留，请将这些字段留空。'],
    ['docker_manager.f_tls_ca', 'CA certificate (PEM)', 'CA-Zertifikat (PEM)', 'Certificado de CA (PEM)', 'Certificat d’AC (PEM)', 'Certificato CA (PEM)', 'CA-certificaat (PEM)', 'Certificado da CA (PEM)', 'Certyfikat CA (PEM)', 'CA-certifikat (PEM)', 'CA-certifikat (PEM)', 'CA-sertifikat (PEM)', 'Certifikát CA (PEM)', 'Πιστοποιητικό CA (PEM)', 'CA प्रमाणपत्र (PEM)', 'CA 証明書（PEM）', 'CA 证书（PEM）'],
    ['docker_manager.f_tls_cert', 'Client certificate (PEM)', 'Client-Zertifikat (PEM)', 'Certificado de cliente (PEM)', 'Certificat client (PEM)', 'Certificato client (PEM)', 'Clientcertificaat (PEM)', 'Certificado do cliente (PEM)', 'Certyfikat klienta (PEM)', 'Klientcertifikat (PEM)', 'Klientcertifikat (PEM)', 'Klientsertifikat (PEM)', 'Klientský certifikát (PEM)', 'Πιστοποιητικό πελάτη (PEM)', 'क्लाइंट प्रमाणपत्र (PEM)', 'クライアント証明書（PEM）', '客户端证书（PEM）'],
    ['docker_manager.f_tls_key', 'Client key (PEM)', 'Client-Schlüssel (PEM)', 'Clave de cliente (PEM)', 'Clé client (PEM)', 'Chiave client (PEM)', 'Clientsleutel (PEM)', 'Chave do cliente (PEM)', 'Klucz klienta (PEM)', 'Klientnyckel (PEM)', 'Klientnøgle (PEM)', 'Klientnøkkel (PEM)', 'Klientský klíč (PEM)', 'Κλειδί πελάτη (PEM)', 'क्लाइंट कुंजी (PEM)', 'クライアント鍵（PEM）', '客户端密钥（PEM）'],
    ['docker_manager.f_tls_hint', 'On a new host, an empty CA uses the system certificates. When you edit, every empty field keeps its stored value.', 'Bei einem neuen Host verwendet eine leere CA die System-Zertifikate. Beim Bearbeiten behält jedes leere Feld seinen gespeicherten Wert.', 'En un host nuevo, una CA vacía usa los certificados del sistema. Al editar, cada campo vacío conserva su valor guardado.', 'Pour un nouvel hôte, une AC vide utilise les certificats du système. En modification, chaque champ vide garde sa valeur enregistrée.', 'Con un nuovo host, una CA vuota usa i certificati di sistema. In modifica, ogni campo vuoto mantiene il valore salvato.', 'Bij een nieuwe host gebruikt een lege CA de systeemcertificaten. Bij bewerken behoudt elk leeg veld zijn opgeslagen waarde.', 'Em um host novo, uma CA vazia usa os certificados do sistema. Ao editar, cada campo vazio mantém o valor armazenado.', 'W nowym hoście puste CA oznacza certyfikaty systemowe. Podczas edycji każde puste pole zachowuje zapisaną wartość.', 'För en ny värd använder en tom CA systemets certifikat. Vid redigering behåller varje tomt fält sitt sparade värde.', 'Ved en ny vært bruger en tom CA systemets certifikater. Ved redigering beholder hvert tomt felt sin gemte værdi.', 'For en ny vert bruker en tom CA systemets sertifikater. Ved redigering beholder hvert tomme felt sin lagrede verdi.', 'U nového hostitele prázdná CA znamená systémové certifikáty. Při úpravě si každé prázdné pole ponechá uloženou hodnotu.', 'Σε νέο υπολογιστή, κενό CA σημαίνει τα πιστοποιητικά του συστήματος. Κατά την επεξεργασία, κάθε κενό πεδίο διατηρεί την αποθηκευμένη τιμή του.', 'नए होस्ट पर खाली CA का मतलब सिस्टम प्रमाणपत्र है। संपादन में हर खाली फ़ील्ड अपना सहेजा हुआ मान रखता है।', '新しいホストでは CA を空にするとシステムの証明書を使います。編集時は、空欄の項目は保存済みの値のまま残ります。', '新主机的 CA 留空即使用系统证书。编辑时，每个留空的字段都会保留其已保存的值。'],
    ['docker_manager.f_replace', 'Replace', 'Ersetzen', 'Reemplazar', 'Remplacer', 'Sostituisci', 'Vervangen', 'Substituir', 'Zastąp', 'Ersätt', 'Erstat', 'Erstatt', 'Nahradit', 'Αντικατάσταση', 'बदलें', '置き換え', '替换'],
    ['docker_manager.v_name_taken', 'Another host already uses this name.', 'Ein anderer Host verwendet diesen Namen bereits.', 'Otro host ya usa este nombre.', 'Un autre hôte utilise déjà ce nom.', 'Un altro host usa già questo nome.', 'Een andere host gebruikt deze naam al.', 'Outro host já usa este nome.', 'Inny host już używa tej nazwy.', 'En annan värd använder redan det här namnet.', 'En anden vært bruger allerede dette navn.', 'En annen vert bruker allerede dette navnet.', 'Jiný hostitel už tento název používá.', 'Ένας άλλος υπολογιστής χρησιμοποιεί ήδη αυτό το όνομα.', 'कोई दूसरा होस्ट यह नाम पहले से इस्तेमाल कर रहा है।', '別のホストがすでにこの名前を使用しています。', '已有其他主机使用此名称。'],
    ['docker_manager.v_name_reserved', '“Local” is reserved for the built-in host.', '„Local“ ist für den eingebauten Host reserviert.', '«Local» está reservado para el host integrado.', '« Local » est réservé à l’hôte intégré.', '«Local» è riservato all’host integrato.', '‘Local’ is gereserveerd voor de ingebouwde host.', '“Local” é reservado ao host integrado.', 'Nazwa „Local” jest zarezerwowana dla wbudowanego hosta.', '”Local” är reserverat för den inbyggda värden.', '“Local” er reserveret til den indbyggede vært.', '«Local» er reservert for den innebygde verten.', 'Název „Local“ je vyhrazen pro vestavěného hostitele.', 'Το «Local» είναι δεσμευμένο για τον ενσωματωμένο υπολογιστή.', '“Local” अंतर्निहित होस्ट के लिए आरक्षित है।', '「Local」は組み込みホスト用に予約されています。', '“Local”为内置主机保留。'],
    ['docker_manager.f_color', 'Color', 'Farbe', 'Color', 'Couleur', 'Colore', 'Kleur', 'Cor', 'Kolor', 'Färg', 'Farve', 'Farge', 'Barva', 'Χρώμα', 'रंग', '色', '颜色'],
    ['docker_manager.f_advanced', 'Advanced', 'Erweitert', 'Avanzado', 'Avancé', 'Avanzate', 'Geavanceerd', 'Avançado', 'Zaawansowane', 'Avancerat', 'Avanceret', 'Avansert', 'Rozšířené', 'Για προχωρημένους', 'उन्नत', '詳細設定', '高级'],
    ['docker_manager.f_advanced_hint', 'Only needed for stacks. The defaults fit most hosts.', 'Nur für Stacks nötig. Die Standardwerte passen für die meisten Hosts.', 'Solo hace falta para stacks. Los valores predeterminados sirven para la mayoría de los hosts.', 'Nécessaire uniquement pour les stacks. Les valeurs par défaut conviennent à la plupart des hôtes.', 'Servono solo per gli stack. I valori predefiniti vanno bene per la maggior parte degli host.', 'Alleen nodig voor stacks. De standaardwaarden passen bij de meeste hosts.', 'Necessário apenas para stacks. Os padrões servem para a maioria dos hosts.', 'Potrzebne tylko dla stosów. Wartości domyślne pasują do większości hostów.', 'Behövs bara för stackar. Standardvärdena passar de flesta värdar.', 'Kun nødvendigt for stacks. Standardværdierne passer til de fleste værter.', 'Trengs bare for stacks. Standardverdiene passer for de fleste verter.', 'Potřeba jen pro stacky. Výchozí hodnoty vyhovují většině hostitelů.', 'Απαραίτητο μόνο για stacks. Οι προεπιλογές ταιριάζουν στους περισσότερους υπολογιστές.', 'केवल स्टैक के लिए ज़रूरी। ज़्यादातर होस्ट के लिए डिफ़ॉल्ट काफ़ी हैं।', 'スタック用の設定です。ほとんどのホストでは初期値で問題ありません。', '仅用于堆栈。默认值适用于大多数主机。'],
    ['docker_manager.f_stacks_dir', 'Stacks folder on the host', 'Stacks-Ordner auf dem Host', 'Carpeta de stacks en el host', 'Dossier des stacks sur l’hôte', 'Cartella degli stack sull’host', 'Stacks-map op de host', 'Pasta de stacks no host', 'Katalog stosów na hoście', 'Stack-mapp på värden', 'Stacks-mappe på værten', 'Stacks-mappe på verten', 'Složka se stacky na hostiteli', 'Φάκελος stacks στον υπολογιστή', 'होस्ट पर स्टैक फ़ोल्डर', 'ホスト上のスタック用フォルダー', '主机上的堆栈文件夹'],
    ['docker_manager.f_socket', 'Docker socket path on the host', 'Pfad des Docker-Sockets auf dem Host', 'Ruta del socket de Docker en el host', 'Chemin du socket Docker sur l’hôte', 'Percorso del socket Docker sull’host', 'Pad van de Docker-socket op de host', 'Caminho do socket do Docker no host', 'Ścieżka gniazda Dockera na hoście', 'Sökväg till Docker-socketen på värden', 'Sti til Docker-socketten på værten', 'Sti til Docker-socketen på verten', 'Cesta k socketu Dockeru na hostiteli', 'Διαδρομή της υποδοχής Docker στον υπολογιστή', 'होस्ट पर Docker सॉकेट का पथ', 'ホスト上の Docker ソケットのパス', '主机上的 Docker 套接字路径'],
    ['docker_manager.f_clear_secret', 'Remove the stored credentials', 'Gespeicherte Zugangsdaten entfernen', 'Eliminar las credenciales guardadas', 'Supprimer les identifiants enregistrés', 'Rimuovi le credenziali salvate', 'Opgeslagen inloggegevens verwijderen', 'Remover as credenciais armazenadas', 'Usuń zapisane dane logowania', 'Ta bort de sparade inloggningsuppgifterna', 'Fjern de gemte loginoplysninger', 'Fjern de lagrede påloggingsopplysningene', 'Odstranit uložené přihlašovací údaje', 'Αφαίρεση των αποθηκευμένων διαπιστευτηρίων', 'सहेजे गए क्रेडेंशियल हटाएँ', '保存済みの認証情報を削除する', '删除已保存的凭据'],
    ['docker_manager.f_test', 'Test connection', 'Verbindung testen', 'Probar conexión', 'Tester la connexion', 'Verifica connessione', 'Verbinding testen', 'Testar conexão', 'Testuj połączenie', 'Testa anslutningen', 'Test forbindelsen', 'Test tilkoblingen', 'Otestovat připojení', 'Δοκιμή σύνδεσης', 'कनेक्शन जाँचें', '接続をテスト', '测试连接'],
    ['docker_manager.f_test_hint', 'Recommended before saving.', 'Empfohlen vor dem Speichern.', 'Recomendado antes de guardar.', 'Recommandé avant d’enregistrer.', 'Consigliato prima di salvare.', 'Aanbevolen vóór het opslaan.', 'Recomendado antes de salvar.', 'Zalecane przed zapisaniem.', 'Rekommenderas innan du sparar.', 'Anbefales før du gemmer.', 'Anbefales før du lagrer.', 'Doporučeno před uložením.', 'Συνιστάται πριν την αποθήκευση.', 'सहेजने से पहले अनुशंसित।', '保存前の確認をおすすめします。', '建议保存前先测试。'],
    ['docker_manager.f_testing', 'Testing…', 'Wird getestet…', 'Probando…', 'Test en cours…', 'Verifica in corso…', 'Bezig met testen…', 'Testando…', 'Testowanie…', 'Testar…', 'Tester…', 'Tester…', 'Testování…', 'Δοκιμή…', 'जाँच हो रही है…', 'テスト中…', '正在测试…'],
    ['docker_manager.f_test_ok', 'Connected. Docker {{version}} (API {{api}}) on {{os}}/{{arch}}.', 'Verbunden. Docker {{version}} (API {{api}}) auf {{os}}/{{arch}}.', 'Conectado. Docker {{version}} (API {{api}}) en {{os}}/{{arch}}.', 'Connecté. Docker {{version}} (API {{api}}) sur {{os}}/{{arch}}.', 'Connesso. Docker {{version}} (API {{api}}) su {{os}}/{{arch}}.', 'Verbonden. Docker {{version}} (API {{api}}) op {{os}}/{{arch}}.', 'Conectado. Docker {{version}} (API {{api}}) em {{os}}/{{arch}}.', 'Połączono. Docker {{version}} (API {{api}}) na {{os}}/{{arch}}.', 'Ansluten. Docker {{version}} (API {{api}}) på {{os}}/{{arch}}.', 'Forbundet. Docker {{version}} (API {{api}}) på {{os}}/{{arch}}.', 'Tilkoblet. Docker {{version}} (API {{api}}) på {{os}}/{{arch}}.', 'Připojeno. Docker {{version}} (API {{api}}) na {{os}}/{{arch}}.', 'Συνδέθηκε. Docker {{version}} (API {{api}}) σε {{os}}/{{arch}}.', 'जुड़ गया। {{os}}/{{arch}} पर Docker {{version}} (API {{api}})।', '接続しました。{{os}}/{{arch}} 上の Docker {{version}}（API {{api}}）。', '已连接。{{os}}/{{arch}} 上的 Docker {{version}}（API {{api}}）。'],
    ['docker_manager.f_test_failed', 'Connection failed: {{reason}}', 'Verbindung fehlgeschlagen: {{reason}}', 'Falló la conexión: {{reason}}', 'Échec de la connexion : {{reason}}', 'Connessione non riuscita: {{reason}}', 'Verbinding mislukt: {{reason}}', 'Falha na conexão: {{reason}}', 'Połączenie nie powiodło się: {{reason}}', 'Anslutningen misslyckades: {{reason}}', 'Forbindelsen mislykkedes: {{reason}}', 'Tilkoblingen mislyktes: {{reason}}', 'Připojení se nezdařilo: {{reason}}', 'Η σύνδεση απέτυχε: {{reason}}', 'कनेक्शन विफल रहा: {{reason}}', '接続に失敗しました: {{reason}}', '连接失败：{{reason}}'],
    ['docker_manager.warn_plain_tcp', 'Plain TCP is unencrypted and has no login. Anyone who can reach this port controls Docker on that host. Prefer TLS or SSH.', 'Reines TCP ist unverschlüsselt und hat keine Anmeldung. Wer diesen Port erreicht, steuert Docker auf diesem Host. Bevorzugen Sie TLS oder SSH.', 'TCP sin cifrar no está cifrado ni tiene inicio de sesión. Quien alcance este puerto controla Docker en ese host. Prefiere TLS o SSH.', 'Le TCP simple n’est ni chiffré ni authentifié. Quiconque atteint ce port contrôle Docker sur cet hôte. Préférez TLS ou SSH.', 'Il TCP semplice non è cifrato e non richiede accesso. Chiunque raggiunga questa porta controlla Docker su quell’host. Preferisci TLS o SSH.', 'Gewone TCP is onversleuteld en heeft geen login. Wie deze poort bereikt, bestuurt Docker op die host. Kies liever TLS of SSH.', 'TCP simples não é criptografado e não tem login. Quem alcançar esta porta controla o Docker nesse host. Prefira TLS ou SSH.', 'Zwykłe TCP nie jest szyfrowane i nie wymaga logowania. Każdy, kto połączy się z tym portem, steruje Dockerem na tym hoście. Wybierz raczej TLS lub SSH.', 'Vanlig TCP är okrypterat och saknar inloggning. Den som når porten styr Docker på värden. Föredra TLS eller SSH.', 'Almindelig TCP er ukrypteret og har ingen login. Alle, der kan nå porten, styrer Docker på værten. Foretræk TLS eller SSH.', 'Vanlig TCP er ukryptert og har ingen pålogging. Alle som når porten, styrer Docker på verten. Foretrekk TLS eller SSH.', 'Prosté TCP není šifrované a nemá přihlášení. Kdokoli, kdo se k portu dostane, ovládá Docker na tomto hostiteli. Raději použijte TLS nebo SSH.', 'Το απλό TCP δεν είναι κρυπτογραφημένο και δεν έχει σύνδεση. Όποιος φτάνει σε αυτή τη θύρα ελέγχει το Docker σε αυτόν τον υπολογιστή. Προτιμήστε TLS ή SSH.', 'सादा TCP एन्क्रिप्टेड नहीं है और इसमें लॉगिन नहीं होता। जो भी इस पोर्ट तक पहुँचे, वह उस होस्ट का Docker नियंत्रित कर सकता है। TLS या SSH को प्राथमिकता दें।', 'プレーンな TCP は暗号化もログインもありません。このポートに到達できる人は誰でもそのホストの Docker を操作できます。TLS または SSH をおすすめします。', '纯 TCP 既不加密也无需登录。任何能访问此端口的人都能控制该主机上的 Docker。建议使用 TLS 或 SSH。'],
    ['docker_manager.color_blue', 'Blue', 'Blau', 'Azul', 'Bleu', 'Blu', 'Blauw', 'Azul', 'Niebieski', 'Blå', 'Blå', 'Blå', 'Modrá', 'Μπλε', 'नीला', '青', '蓝色'],
    ['docker_manager.color_teal', 'Teal', 'Petrol', 'Verde azulado', 'Sarcelle', 'Verde acqua', 'Blauwgroen', 'Verde-azulado', 'Morski', 'Blågrön', 'Blågrøn', 'Blågrønn', 'Tyrkysová', 'Τιρκουάζ', 'हरा-नीला', 'ティール', '青绿色'],
    ['docker_manager.color_purple', 'Purple', 'Violett', 'Morado', 'Violet', 'Viola', 'Paars', 'Roxo', 'Fioletowy', 'Lila', 'Lilla', 'Lilla', 'Fialová', 'Μωβ', 'बैंगनी', '紫', '紫色'],
    ['docker_manager.color_coral', 'Coral', 'Koralle', 'Coral', 'Corail', 'Corallo', 'Koraal', 'Coral', 'Koralowy', 'Korall', 'Koral', 'Korall', 'Korálová', 'Κοραλί', 'मूंगा', 'コーラル', '珊瑚色'],
    ['docker_manager.color_pink', 'Pink', 'Rosa', 'Rosa', 'Rose', 'Rosa', 'Roze', 'Rosa', 'Różowy', 'Rosa', 'Pink', 'Rosa', 'Růžová', 'Ροζ', 'गुलाबी', 'ピンク', '粉色'],
    ['docker_manager.color_amber', 'Amber', 'Bernstein', 'Ámbar', 'Ambre', 'Ambra', 'Amber', 'Âmbar', 'Bursztynowy', 'Bärnsten', 'Rav', 'Rav', 'Jantarová', 'Κεχριμπαρί', 'एम्बर', '琥珀', '琥珀色'],
    ['docker_manager.color_green', 'Green', 'Grün', 'Verde', 'Vert', 'Verde', 'Groen', 'Verde', 'Zielony', 'Grön', 'Grøn', 'Grønn', 'Zelená', 'Πράσινο', 'हरा', '緑', '绿色'],
    ['docker_manager.color_gray', 'Gray', 'Grau', 'Gris', 'Gris', 'Grigio', 'Grijs', 'Cinza', 'Szary', 'Grå', 'Grå', 'Grå', 'Šedá', 'Γκρι', 'स्लेटी', 'グレー', '灰色'],
    ['docker_manager.v_name', 'Enter a name of up to 64 characters.', 'Geben Sie einen Namen mit höchstens 64 Zeichen ein.', 'Introduce un nombre de hasta 64 caracteres.', 'Saisissez un nom de 64 caractères au maximum.', 'Inserisci un nome di al massimo 64 caratteri.', 'Voer een naam van maximaal 64 tekens in.', 'Insira um nome de até 64 caracteres.', 'Wpisz nazwę o długości do 64 znaków.', 'Ange ett namn på högst 64 tecken.', 'Angiv et navn på højst 64 tegn.', 'Skriv inn et navn på maksimalt 64 tegn.', 'Zadejte název dlouhý nejvýše 64 znaků.', 'Εισαγάγετε όνομα έως 64 χαρακτήρων.', 'अधिकतम 64 अक्षरों का नाम दर्ज करें।', '64 文字以内の名前を入力してください。', '请输入不超过 64 个字符的名称。'],
    ['docker_manager.v_address', 'Enter the host name or IP address.', 'Geben Sie den Host-Namen oder die IP-Adresse ein.', 'Introduce el nombre de host o la dirección IP.', 'Saisissez le nom d’hôte ou l’adresse IP.', 'Inserisci il nome host o l’indirizzo IP.', 'Voer de hostnaam of het IP-adres in.', 'Insira o nome do host ou o endereço IP.', 'Wpisz nazwę hosta lub adres IP.', 'Ange värdnamn eller IP-adress.', 'Angiv værtsnavn eller IP-adresse.', 'Skriv inn vertsnavn eller IP-adresse.', 'Zadejte název hostitele nebo IP adresu.', 'Εισαγάγετε το όνομα υπολογιστή ή τη διεύθυνση IP.', 'होस्ट का नाम या IP पता दर्ज करें।', 'ホスト名または IP アドレスを入力してください。', '请输入主机名或 IP 地址。'],
    ['docker_manager.v_address_scheme', 'Remove the scheme and path, for example use 192.0.2.10 instead of tcp://192.0.2.10:2375.', 'Entfernen Sie Schema und Pfad, z. B. 192.0.2.10 statt tcp://192.0.2.10:2375.', 'Quita el esquema y la ruta; por ejemplo, usa 192.0.2.10 en lugar de tcp://192.0.2.10:2375.', 'Retirez le schéma et le chemin, par exemple 192.0.2.10 au lieu de tcp://192.0.2.10:2375.', 'Rimuovi schema e percorso, ad esempio usa 192.0.2.10 invece di tcp://192.0.2.10:2375.', 'Verwijder het schema en het pad, bijvoorbeeld 192.0.2.10 in plaats van tcp://192.0.2.10:2375.', 'Remova o esquema e o caminho, por exemplo use 192.0.2.10 em vez de tcp://192.0.2.10:2375.', 'Usuń schemat i ścieżkę, np. wpisz 192.0.2.10 zamiast tcp://192.0.2.10:2375.', 'Ta bort schema och sökväg, t.ex. 192.0.2.10 i stället för tcp://192.0.2.10:2375.', 'Fjern skema og sti, f.eks. 192.0.2.10 i stedet for tcp://192.0.2.10:2375.', 'Fjern skjema og sti, f.eks. 192.0.2.10 i stedet for tcp://192.0.2.10:2375.', 'Odstraňte schéma a cestu, např. použijte 192.0.2.10 místo tcp://192.0.2.10:2375.', 'Αφαιρέστε το σχήμα και τη διαδρομή, π.χ. χρησιμοποιήστε 192.0.2.10 αντί για tcp://192.0.2.10:2375.', 'स्कीम और पथ हटाएँ, जैसे tcp://192.0.2.10:2375 की जगह 192.0.2.10 लिखें।', 'スキームとパスを削除してください（例: tcp://192.0.2.10:2375 ではなく 192.0.2.10）。', '请去掉协议和路径，例如使用 192.0.2.10，而不是 tcp://192.0.2.10:2375。'],
    ['docker_manager.v_port', 'The port must be a number from 1 to 65535.', 'Der Port muss eine Zahl von 1 bis 65535 sein.', 'El puerto debe ser un número del 1 al 65535.', 'Le port doit être un nombre de 1 à 65535.', 'La porta deve essere un numero da 1 a 65535.', 'De poort moet een getal van 1 tot 65535 zijn.', 'A porta deve ser um número de 1 a 65535.', 'Port musi być liczbą od 1 do 65535.', 'Porten måste vara ett tal mellan 1 och 65535.', 'Porten skal være et tal fra 1 til 65535.', 'Porten må være et tall fra 1 til 65535.', 'Port musí být číslo od 1 do 65535.', 'Η θύρα πρέπει να είναι αριθμός από 1 έως 65535.', 'पोर्ट 1 से 65535 के बीच की संख्या होना चाहिए।', 'ポートは 1〜65535 の数値にしてください。', '端口必须是 1 到 65535 之间的数字。'],
    ['docker_manager.v_ssh_user', 'Enter the SSH user.', 'Geben Sie den SSH-Benutzer ein.', 'Introduce el usuario SSH.', 'Saisissez l’utilisateur SSH.', 'Inserisci l’utente SSH.', 'Voer de SSH-gebruiker in.', 'Insira o usuário SSH.', 'Wpisz użytkownika SSH.', 'Ange SSH-användaren.', 'Angiv SSH-brugeren.', 'Skriv inn SSH-brukeren.', 'Zadejte uživatele SSH.', 'Εισαγάγετε τον χρήστη SSH.', 'SSH उपयोगकर्ता दर्ज करें।', 'SSH ユーザーを入力してください。', '请输入 SSH 用户。'],
    ['docker_manager.v_ssh_secret', 'Enter an SSH password or a private key.', 'Geben Sie ein SSH-Passwort oder einen privaten Schlüssel ein.', 'Introduce una contraseña SSH o una clave privada.', 'Saisissez un mot de passe SSH ou une clé privée.', 'Inserisci una password SSH o una chiave privata.', 'Voer een SSH-wachtwoord of een privésleutel in.', 'Insira uma senha SSH ou uma chave privada.', 'Wpisz hasło SSH lub klucz prywatny.', 'Ange ett SSH-lösenord eller en privat nyckel.', 'Angiv en SSH-adgangskode eller en privat nøgle.', 'Skriv inn et SSH-passord eller en privat nøkkel.', 'Zadejte heslo SSH nebo soukromý klíč.', 'Εισαγάγετε κωδικό SSH ή ιδιωτικό κλειδί.', 'SSH पासवर्ड या निजी कुंजी दर्ज करें।', 'SSH パスワードまたは秘密鍵を入力してください。', '请输入 SSH 密码或私钥。'],
    ['docker_manager.v_tls_cert', 'Mutual TLS needs a client certificate and a client key.', 'Gegenseitiges TLS benötigt ein Client-Zertifikat und einen Client-Schlüssel.', 'TLS mutuo necesita un certificado de cliente y una clave de cliente.', 'Le TLS mutuel nécessite un certificat client et une clé client.', 'Il TLS reciproco richiede un certificato client e una chiave client.', 'Wederzijdse TLS vereist een clientcertificaat en een clientsleutel.', 'O TLS mútuo exige um certificado de cliente e uma chave de cliente.', 'Wzajemne TLS wymaga certyfikatu klienta i klucza klienta.', 'Ömsesidig TLS kräver ett klientcertifikat och en klientnyckel.', 'Gensidig TLS kræver et klientcertifikat og en klientnøgle.', 'Gjensidig TLS krever et klientsertifikat og en klientnøkkel.', 'Vzájemné TLS vyžaduje klientský certifikát a klientský klíč.', 'Το αμοιβαίο TLS απαιτεί πιστοποιητικό πελάτη και κλειδί πελάτη.', 'म्यूचुअल TLS के लिए क्लाइंट प्रमाणपत्र और क्लाइंट कुंजी चाहिए।', '相互 TLS にはクライアント証明書とクライアント鍵が必要です。', '双向 TLS 需要客户端证书和客户端密钥。'],
    ['docker_manager.v_path', 'Use an absolute path without “..”.', 'Verwenden Sie einen absoluten Pfad ohne „..“.', 'Usa una ruta absoluta sin «..».', 'Utilisez un chemin absolu sans « .. ».', 'Usa un percorso assoluto senza «..».', 'Gebruik een absoluut pad zonder “..”.', 'Use um caminho absoluto sem “..”.', 'Użyj ścieżki bezwzględnej bez „..”.', 'Använd en absolut sökväg utan ”..”.', 'Brug en absolut sti uden “..”.', 'Bruk en absolutt sti uten «..».', 'Použijte absolutní cestu bez „..“.', 'Χρησιμοποιήστε απόλυτη διαδρομή χωρίς «..».', 'बिना “..” वाला पूर्ण पथ इस्तेमाल करें।', '「..」を含まない絶対パスを指定してください。', '请使用不含“..”的绝对路径。'],
    ['docker_manager.nest_title', 'Import from a nest', 'Aus einem Nest importieren', 'Importar desde un nido', 'Importer depuis un nid', 'Importa da un nido', 'Importeren uit een nest', 'Importar de um ninho', 'Importuj z gniazda', 'Importera från ett bo', 'Importér fra en rede', 'Importer fra en rede', 'Import z hnízda', 'Εισαγωγή από φωλιά', 'nest से आयात करें', 'ネストからインポート', '从巢穴导入'],
    ['docker_manager.nest_intro', 'Nests that deploy with Docker can become hosts. The credentials are copied, not shared.', 'Nester, die per Docker bereitgestellt werden, können zu Hosts werden. Die Zugangsdaten werden kopiert, nicht geteilt.', 'Los nidos que se despliegan con Docker pueden convertirse en hosts. Las credenciales se copian, no se comparten.', 'Les nids déployés avec Docker peuvent devenir des hôtes. Les identifiants sont copiés, pas partagés.', 'I nidi distribuiti con Docker possono diventare host. Le credenziali vengono copiate, non condivise.', 'Nesten die met Docker worden uitgerold, kunnen hosts worden. De inloggegevens worden gekopieerd, niet gedeeld.', 'Ninhos implantados com Docker podem virar hosts. As credenciais são copiadas, não compartilhadas.', 'Gniazda wdrażane przez Dockera mogą zostać hostami. Dane logowania są kopiowane, a nie współdzielone.', 'Bon som distribueras med Docker kan bli värdar. Inloggningsuppgifterna kopieras, de delas inte.', 'Reder, der udrulles med Docker, kan blive til værter. Loginoplysningerne kopieres, ikke deles.', 'Reder som distribueres med Docker, kan bli verter. Påloggingsopplysningene kopieres, ikke deles.', 'Hnízda nasazená přes Docker se mohou stát hostiteli. Přihlašovací údaje se kopírují, nesdílejí.', 'Οι φωλιές που αναπτύσσονται με Docker μπορούν να γίνουν υπολογιστές. Τα διαπιστευτήρια αντιγράφονται, δεν μοιράζονται.', 'Docker से तैनात किए गए nest होस्ट बन सकते हैं। क्रेडेंशियल कॉपी होते हैं, साझा नहीं।', 'Docker でデプロイされるネストはホストにできます。認証情報はコピーされ、共有されません。', '通过 Docker 部署的巢穴可以变成主机。凭据会被复制，而不是共享。'],
    ['docker_manager.nest_empty', 'No Docker nests found.', 'Keine Docker-Nester gefunden.', 'No se encontraron nidos de Docker.', 'Aucun nid Docker trouvé.', 'Nessun nido Docker trovato.', 'Geen Docker-nesten gevonden.', 'Nenhum ninho Docker encontrado.', 'Nie znaleziono gniazd Dockera.', 'Inga Docker-bon hittades.', 'Ingen Docker-reder fundet.', 'Ingen Docker-reder funnet.', 'Nebyla nalezena žádná hnízda Dockeru.', 'Δεν βρέθηκαν φωλιές Docker.', 'कोई Docker nest नहीं मिला।', 'Docker のネストが見つかりません。', '未找到 Docker 巢穴。'],
    ['docker_manager.nest_import', 'Import', 'Importieren', 'Importar', 'Importer', 'Importa', 'Importeren', 'Importar', 'Importuj', 'Importera', 'Importér', 'Importer', 'Importovat', 'Εισαγωγή', 'आयात करें', 'インポート', '导入'],
    ['docker_manager.nest_importing', 'Importing…', 'Wird importiert…', 'Importando…', 'Importation…', 'Importazione…', 'Importeren…', 'Importando…', 'Importowanie…', 'Importerar…', 'Importerer…', 'Importerer…', 'Importuje se…', 'Εισαγωγή…', 'आयात हो रहा है…', 'インポート中…', '正在导入…'],
    ['docker_manager.nest_imported', 'Host {{name}} imported', 'Host {{name}} importiert', 'Host {{name}} importado', 'Hôte {{name}} importé', 'Host {{name}} importato', 'Host {{name}} geïmporteerd', 'Host {{name}} importado', 'Zaimportowano hosta {{name}}', 'Värden {{name}} har importerats', 'Værten {{name}} er importeret', 'Verten {{name}} er importert', 'Hostitel {{name}} byl importován', 'Ο υπολογιστής {{name}} εισήχθη', 'होस्ट {{name}} आयात किया गया', 'ホスト {{name}} をインポートしました', '已导入主机 {{name}}'],
    ['docker_manager.nest_method_docker_local', 'Local Docker', 'Lokales Docker', 'Docker local', 'Docker local', 'Docker locale', 'Lokale Docker', 'Docker local', 'Lokalny Docker', 'Lokal Docker', 'Lokal Docker', 'Lokal Docker', 'Místní Docker', 'Τοπικό Docker', 'स्थानीय Docker', 'ローカル Docker', '本地 Docker'],
    ['docker_manager.nest_method_docker_remote', 'Remote Docker', 'Entferntes Docker', 'Docker remoto', 'Docker distant', 'Docker remoto', 'Externe Docker', 'Docker remoto', 'Zdalny Docker', 'Fjärr-Docker', 'Fjern-Docker', 'Ekstern Docker', 'Vzdálený Docker', 'Απομακρυσμένο Docker', 'रिमोट Docker', 'リモート Docker', '远程 Docker'],
    ['docker_manager.nest_method_docker_ssh', 'Docker over SSH', 'Docker über SSH', 'Docker por SSH', 'Docker via SSH', 'Docker via SSH', 'Docker via SSH', 'Docker via SSH', 'Docker przez SSH', 'Docker över SSH', 'Docker via SSH', 'Docker via SSH', 'Docker přes SSH', 'Docker μέσω SSH', 'SSH पर Docker', 'SSH 経由の Docker', '通过 SSH 的 Docker']
];

// ---- merge ---------------------------------------------------------------------------------------------------------
const tokens = text => [...String(text).matchAll(/\{\{(\w+)\}\}/g)].map(match => match[1]).sort().join(',');
for (const [key, ...values] of ROWS) {
    if (values.length !== LANGS.length) throw new Error(key + ': expected ' + LANGS.length + ' texts, got ' + values.length);
    values.forEach((value, index) => {
        if (typeof value !== 'string' || value.trim() === '') throw new Error(key + ': empty text for ' + LANGS[index]);
        if (tokens(value) !== tokens(values[0])) throw new Error(key + ': placeholders of ' + LANGS[index] + ' differ from English');
    });
}
if (new Set(ROWS.map(row => row[0])).size !== ROWS.length) throw new Error('duplicate keys');

const root = path.resolve(process.argv[2] || '.');
LANGS.forEach((lang, index) => {
    const file = path.join(root, 'ui', 'lang', 'desktop', lang + '.json');
    const text = fs.readFileSync(file, 'utf8').replace(/\r\n/g, '\n');
    const data = JSON.parse(text);
    const present = ROWS.filter(row => Object.prototype.hasOwnProperty.call(data, row[0]));
    if (present.length) throw new Error(file + ' already contains ' + present[0][0] + '; restore the file before merging again');
    if (!/\n\}\s*$/.test(text)) throw new Error(file + ': unexpected ending');
    const lines = ROWS.map(row => '  ' + JSON.stringify(row[0]) + ': ' + JSON.stringify(row[index + 1]));
    const merged = text.replace(/\n\}\s*$/, ',\n' + lines.join(',\n') + '\n}\n');
    const check = JSON.parse(merged);
    for (const row of ROWS) if (check[row[0]] !== row[index + 1]) throw new Error(file + ': ' + row[0] + ' did not round-trip');
    fs.writeFileSync(file, merged, 'utf8');
});
console.log('Merged ' + ROWS.length + ' keys into ' + LANGS.length + ' desktop locale files.');
```

- [ ] **Step 4: Run the merge.**

```powershell
New-Item -ItemType Directory -Force disposable\dm-i18n | Out-Null
node disposable\dm-i18n\merge-docker-manager-i18n.mjs .
git diff --stat -- ui/lang/desktop
```

Expected: `Merged 192 keys into 16 desktop locale files.` and 16 files changed with only additions at the end of each file. Running it
a second time stops with `already contains docker_manager...; restore the file before merging again`.

- [ ] **Step 5: Register the helper files and the key families with the i18n lint.** The static lint then resolves every
  `t('key')` in the five module files against all 16 locales, and the run-time families (`t('status_' + id)`, ...) get their
  own "exists in every locale" guarantee.

In `ui/i18n_lint_test.go`:

Edit 1 of 2: replace

```text
	"cfg/ruview.js":                              {name: "tr", prefix: "ruview."},
```

with

```text
	"js/desktop/apps/docker-manager.js":          {name: "t", prefix: "docker_manager."},
	"js/desktop/apps/docker-manager-model.js":    {name: "t", prefix: "docker_manager."},
	"js/desktop/apps/docker-manager-store.js":    {name: "t", prefix: "docker_manager."},
	"js/desktop/apps/docker-manager-views.js":    {name: "t", prefix: "docker_manager."},
	"js/desktop/apps/docker-manager-dialogs.js":  {name: "t", prefix: "docker_manager."},
	"cfg/ruview.js":                              {name: "tr", prefix: "ruview."},
```

Edit 2 of 2: replace

```text
	"homepage_studio.target_",
}
```

with

```text
	"homepage_studio.target_",
	"docker_manager.status_",
	"docker_manager.action_",
	"docker_manager.reason_",
	"docker_manager.error_",
	"docker_manager.filter_",
	"docker_manager.kind_",
	"docker_manager.color_",
	"docker_manager.col_",
	"docker_manager.host_",
	"docker_manager.protected_",
	"docker_manager.tab_",
	"docker_manager.nest_method_",
	"docker_manager.v_",
}
```

```powershell
gofmt -l ui/i18n_lint_test.go
```

Expected: no output (the helper map entries are already aligned).

- [ ] **Step 6: Run the guards; expect PASS.**

```powershell
go test ./ui -run "TestDockerManagerLocalesAreComplete|TestDesktopBuiltinAppNamesExistInAllLanguages|TestDesktopUsedI18nKeysExistInAllLanguages|TestDesktopDynamicI18nPrefixKeysExistInAllLanguages|TestTranslations_|TestDesktopStartMenuCategoriesStayInSync|TestFrontend_JS_NoLiteralTranslationFallbacks" -count=1
```

Expected: `ok  aurago/ui`. (`TestFrontend_StaticI18nKeysExistInEnglishBundle` is a different, pre-existing failure.)

- [ ] **Step 7: Detect changes and commit.**

```powershell
git add ui/lang/desktop/cs.json ui/lang/desktop/da.json ui/lang/desktop/de.json ui/lang/desktop/el.json ui/lang/desktop/en.json ui/lang/desktop/es.json ui/lang/desktop/fr.json ui/lang/desktop/hi.json ui/lang/desktop/it.json ui/lang/desktop/ja.json ui/lang/desktop/nl.json ui/lang/desktop/no.json ui/lang/desktop/pl.json ui/lang/desktop/pt.json ui/lang/desktop/sv.json ui/lang/desktop/zh.json ui/i18n_lint_test.go ui/desktop_docker_manager_i18n_test.go
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): add the Docker Manager strings in all 16 desktop locales" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```


### Task C4: Model and store (pure logic) with the Node test

**Files**

- Create: `ui/js/desktop/apps/docker-manager-model.js`
- Create: `ui/js/desktop/apps/docker-manager-store.js`
- Create (test): `scripts/test-docker-manager-ui.mjs` (this task adds the model and store sections; Tasks C5 and C7 append the rest)
- Modify: `package.json` (npm script)

Design rules the code keeps (the tests pin them):

- **Model** (`window.DockerManagerModel`, no DOM, no network): container status (`ok`/`warn`/`bad`/`idle` tone plus a state id,
  always shown together with text) and the single problem reason (unhealthy, restarting, dead, or exited with a non-zero code
  read from `Exited (N)`); one row per container of the hosts in scope with a key that is unique across hosts; search (every
  word must match name, image, id, project, service, host name, ports or state); the state filters
  (all, running, stopped, problem) and their counts (counts follow the search, not the filter); grouping by compose project
  (projects first, by name then host order; the same project on two hosts stays two groups; single containers share one
  group); keyboard navigation items; host health (a failed container load beats an older "ok", the zero `checked_at` is "not
  checked yet"); the host form rules and the `HostWrite` body (secrets only when typed, TLS fields one by one); error
  normalisation.
- **Store** (`window.DockerManagerStore`, DOM only inside `attach`): every request through the `api` it is given, one request in
  flight per key (a second call joins it), a per-host container cache that survives a failed refresh, polling every 5 s only
  while the window is visible (hiding aborts the polls and records nothing), `GET /hosts` polled together with the lists,
  actions that reload the list after the engine answered, and the detail drawer's logs and inspect with stale answers
  dropped.

- [ ] **Step 1: Write the failing test.**

Create `scripts/test-docker-manager-ui.mjs`:

```js
// Docker Manager desktop app: Node tests for the pure model and the store (with a fake api). Later tasks append the load
// contract of the five modules, the stylesheet rules and the translation keys. Run: node scripts/test-docker-manager-ui.mjs
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';

const root = path.resolve(import.meta.dirname, '..');
const files = ['docker-manager-model.js', 'docker-manager-store.js'];
const read = file => fs.readFileSync(path.join(root, file), 'utf8');
const plain = value => JSON.parse(JSON.stringify(value));
// The modules run in their own vm realm, so arrays and objects they return are compared as plain JSON.
const eq = (actual, expected, message) => assert.deepStrictEqual(plain(actual), plain(expected), message);

// ---- the pure modules load without a DOM (the vm has no document) and stay under 1100 lines ------------------------------
for (const file of files) {
    const lines = read('ui/js/desktop/apps/' + file).split('\n').length;
    assert.ok(lines < 1100, `${file} has ${lines} lines, want fewer than 1100`);
}
const window = {};
vm.runInNewContext(files.map(file => read('ui/js/desktop/apps/' + file)).join('\n'), {
    window, console, setTimeout, clearTimeout, AbortController, Intl, Map, Set, JSON, Math, Date, URL, performance
});
const model = window.DockerManagerModel, Store = window.DockerManagerStore;
assert.ok(model && Store, 'the model and the store publish their global');

// ---- model fixtures -------------------------------------------------------------------------------------------------
const container = (name, extra = {}) => ({
    id: name.slice(0, 12).padEnd(12, '0'), full_id: name.padEnd(64, '0'), names: [name], image: 'nginx:1.27', image_id: 'sha256:aaa', state: 'running', status: 'Up 3 hours',
    created: 1760000000, ports: [], mounts: [], networks: ['bridge'], ...extra
});
const hostA = { id: 'local', name: 'Local', kind: 'local', address: 'unix:///var/run/docker.sock', color: 'blue', builtin: true, status: { state: 'ok' } };
const hostB = { id: 'h_nas', name: 'NAS', kind: 'ssh', address: '192.0.2.20', port: 0, color: 'teal', status: { state: 'unknown' } };

// ---- status and problems ----------------------------------------------------------------------------------------------
eq(plain(model.statusOf(container('a'))), { id: 'running', tone: 'ok', problem: '' });
eq(plain(model.statusOf(container('a', { health: 'unhealthy' }))), { id: 'unhealthy', tone: 'warn', problem: 'unhealthy' });
assert.equal(model.statusOf(container('a', { health: 'starting' })).id, 'starting');
assert.equal(model.problemOf(container('a', { health: 'starting' })), '', 'a starting health check is not a problem');
assert.equal(model.statusOf(container('a', { state: 'restarting', status: 'Restarting (1) 5 seconds ago' })).tone, 'warn');
assert.equal(model.problemOf(container('a', { state: 'restarting' })), 'restarting');
assert.equal(model.problemOf(container('a', { state: 'exited', status: 'Exited (1) 2 minutes ago' })), 'exited');
assert.equal(model.statusOf(container('a', { state: 'exited', status: 'Exited (137) 2 minutes ago' })).code, 137);
assert.equal(model.statusOf(container('a', { state: 'exited', status: 'Exited (137) 2 minutes ago' })).tone, 'bad');
assert.equal(model.problemOf(container('a', { state: 'exited', status: 'Exited (0) 2 minutes ago' })), '', 'a clean exit is no problem');
assert.equal(model.statusOf(container('a', { state: 'exited', status: 'Exited (0) 2 minutes ago' })).tone, 'idle');
assert.equal(model.problemOf(container('a', { state: 'dead' })), 'dead');
assert.equal(model.problemOf(container('a', { state: 'created' })), '');
assert.equal(model.statusOf(container('a', { state: 'paused' })).tone, 'idle');
assert.equal(model.exitCode({ status: 'Up 2 hours' }), null);

// ---- filters, search, counts ------------------------------------------------------------------------------------------
const rows = model.buildRows([hostA, hostB], {
    local: { containers: [
        container('web', { compose_project: 'shop', compose_service: 'web', ports: [{ ip: '0.0.0.0', private: 80, public: 8080, type: 'tcp' }, { ip: '::', private: 80, public: 8080, type: 'tcp' }] }),
        container('db', { compose_project: 'shop', compose_service: 'db', image: 'postgres:16', health: 'unhealthy' }),
        container('worker', { compose_project: 'shop', compose_service: 'worker', state: 'exited', status: 'Exited (2) 1 hour ago' }),
        container('tools', { state: 'exited', status: 'Exited (0) 1 hour ago' }),
        container('aurago', { self: true, protected_owner: 'AuraGo' })
    ] },
    h_nas: { containers: [
        container('web', { compose_project: 'shop', compose_service: 'web', image: 'nginx:1.26' }),
        container('media', { compose_project: 'media', compose_service: 'plex', image: 'plex:latest' }),
        container('loose')
    ] }
}, 'all');
assert.equal(rows.length, 8);
assert.equal(new Set(rows.map(row => row.key)).size, 8, 'row keys are unique across hosts, even for equal container names');
assert.equal(model.buildRows([hostA, hostB], { local: { containers: [container('x')] }, h_nas: { containers: [container('y')] } }, 'h_nas').length, 1, 'a host scope shows only that host');
assert.equal(model.buildRows([hostA], {}, 'all').length, 0, 'a host without loaded containers contributes no rows');
eq(plain(model.counts(rows, '')), { all: 8, running: 6, stopped: 2, problem: 2 });
eq(plain(model.counts(rows, 'shop')), { all: 4, running: 3, stopped: 1, problem: 2 });
eq(model.filterRows(rows, { filter: 'problem' }).map(row => model.containerName(row.c)), ['db', 'worker']);
eq(model.filterRows(rows, { filter: 'stopped' }).map(row => model.containerName(row.c)), ['worker', 'tools']);
assert.equal(model.filterRows(rows, { filter: 'running' }).length, 6);
assert.equal(model.filterRows(rows, { filter: 'all', query: '' }).length, 8);
eq(model.filterRows(rows, { query: 'postgres' }).map(row => model.containerName(row.c)), ['db'], 'search matches the image');
assert.equal(model.filterRows(rows, { query: '8080' }).length, 1, 'search matches a published port');
assert.equal(model.filterRows(rows, { query: 'nas web' }).length, 1, 'every search word must match, the host name counts');
assert.equal(model.filterRows(rows, { query: 'SHOP db' }).length, 1, 'search ignores case');
assert.equal(model.filterRows(rows, { query: 'nothing-like-this' }).length, 0);
assert.equal(model.filterRows(rows, { query: 'plex', filter: 'stopped' }).length, 0, 'search and state filter combine');

// ---- grouping by compose project --------------------------------------------------------------------------------------
const groups = model.groupRows(rows, { multiHost: true, hostOrder: ['local', 'h_nas'] });
eq(groups.map(group => group.key), ['p:h_nas|media', 'p:local|shop', 'p:h_nas|shop', 's:'], 'projects by name then host order, single containers last');
assert.equal(groups[1].hostName, 'Local');
eq(groups[1].rows.map(row => model.containerName(row.c)), ['db', 'web', 'worker'], 'rows sort by service');
eq(plain(groups[1].counts), { total: 3, running: 2, problem: 2 });
assert.equal(groups[1].tone, 'bad', 'a failed container makes the project red');
assert.equal(groups[0].tone, 'ok');
assert.equal(groups[3].kind, 'single');
eq(groups[3].rows.map(row => model.containerName(row.c)), ['aurago', 'loose', 'tools'], 'single containers of all hosts share one group');
assert.equal(model.groupRows(model.buildRows([hostA, hostB], { local: { containers: [container('a', { compose_project: 'p' })] }, h_nas: { containers: [container('a', { compose_project: 'p' })] } }, 'all'), { multiHost: true }).length, 2, 'the same project on two hosts stays two groups');
const single = model.groupRows(model.buildRows([hostA], { local: { containers: [container('a'), container('b')] } }, 'local'), {});
assert.equal(single.length, 1);
assert.equal(model.showGroupHeaders(single), false, 'one group of single containers needs no header');
eq(model.navItems(single, new Set()).map(item => item.type), ['row', 'row']);
assert.equal(model.showGroupHeaders(groups), true);
const collapsedKeys = model.navItems(groups, new Set(['p:local|shop'])).map(item => item.key);
assert.ok(collapsedKeys.includes('g:p:local|shop'), 'a collapsed group keeps its header');
assert.ok(!collapsedKeys.includes('local/' + 'db'.padEnd(64, '0')), 'a collapsed group hides its rows');
assert.ok(model.navItems(groups, new Set()).some(item => item.key === 'local/' + 'db'.padEnd(64, '0')), 'an open group lists its rows');
const partial = model.groupRows(model.buildRows([hostA], { local: { containers: [container('a', { compose_project: 'p' }), container('b', { compose_project: 'p', state: 'exited', status: 'Exited (0) 1 day ago' })] } }, 'local'), {});
assert.equal(partial[0].tone, 'warn', 'a project with a stopped service is partial');

// ---- hosts: health, address, counts -----------------------------------------------------------------------------------
assert.equal(model.hostHealth(hostA, { loaded: true }).tone, 'ok');
assert.equal(model.hostHealth(hostB, undefined).state, 'unknown');
const never = { id: 'h_new', kind: 'tcp', status: { state: 'error', code: 'refused', checked_at: '0001-01-01T00:00:00Z' } };
assert.equal(model.hostHealth(never, undefined).state, 'unknown', 'a zero checked_at means not checked yet, whatever else the status says');
assert.equal(model.hostHealth(never, undefined).tone, 'idle');
assert.equal(model.hostHealth({ status: { state: 'ok', checked_at: '2026-10-26T10:00:00Z' } }, undefined).state, 'ok', 'a real checked_at is trusted');
assert.equal(model.hostHealth(hostB, { loaded: false, error: { code: 'host_unreachable', reason: 'refused', message: 'connect refused' } }).reason, 'refused');
assert.equal(model.hostHealth(hostB, { loaded: true, error: { code: 'host_unreachable', reason: 'weird' } }).reason, 'unknown', 'an unknown reason code falls back');
assert.equal(model.hostHealth({ status: { state: 'error', code: 'tls' } }, undefined).reason, 'tls');
assert.equal(model.hostHealth(hostB, { loaded: true, error: { code: 'docker_error', message: 'boom' } }).tone, 'warn');
assert.equal(model.hostAddress(hostB), '192.0.2.20:22');
assert.equal(model.hostAddress({ kind: 'tls', address: '2001:db8::1', port: 0 }), '[2001:db8::1]:2376');
assert.equal(model.hostAddress({ kind: 'tcp', address: 'docker.lan', port: 4243 }), 'docker.lan:4243');
assert.equal(model.hostAddress(hostA), 'unix:///var/run/docker.sock');
eq(plain(model.totalCounts([hostA, hostB], { local: { containers: [container('a'), container('b', { state: 'exited' })] }, h_nas: { containers: [container('c')] } })), { running: 2, total: 3 });
assert.equal(model.nextColor([{ color: 'blue' }, { color: 'teal' }]), 'purple');
assert.equal(model.colorOf('Teal'), 'teal');
assert.equal(model.colorOf('chartreuse'), 'gray');

// ---- ports, images, actions -------------------------------------------------------------------------------------------
eq(model.portItems(rows[0].c.ports).map(item => item.label), ['8080→80/tcp'], 'IPv4 and IPv6 bindings of one port show once');
eq(model.portItems([{ private: 9000, type: 'tcp' }, { private: 80, public: 80, type: 'tcp' }]).map(item => item.label), ['80→80/tcp', '9000/tcp'], 'published ports come first');
assert.equal(model.shortImage('docker.io/library/nginx:1.27'), 'nginx:1.27');
assert.equal(model.shortImage('ghcr.io/org/app:v1'), 'ghcr.io/org/app:v1');
assert.equal(model.shortImage('sha256:0123456789abcdef0123456789abcdef'), 'sha256:0123456789ab');
eq(plain(model.actionsFor(container('a'), false)), { toggle: 'stop', restart: true, menu: ['pause', 'kill'], remove: true });
eq(plain(model.actionsFor(container('a', { state: 'exited' }), false)), { toggle: 'start', restart: false, menu: [], remove: true });
eq(plain(model.actionsFor(container('a', { state: 'paused' }), false)), { toggle: 'unpause', restart: false, menu: ['kill'], remove: true });
eq(plain(model.actionsFor(container('a'), true)), { toggle: '', restart: false, menu: [], remove: false }, 'read-only hides every mutating action');
assert.equal(model.needsProtectedConfirm(rows[4].c, 'stop'), true);
assert.equal(model.needsProtectedConfirm(rows[4].c, 'start'), false);
assert.equal(model.needsProtectedConfirm(rows[0].c, 'stop'), false);
eq(plain(model.protectedReason({ protected_owner: 'Invasion' })), { kind: 'owner', owner: 'Invasion' });
assert.equal(model.protectedReason(container('a')), null);

// ---- host form ---------------------------------------------------------------------------------------------------------
const form = overrides => Object.assign(model.emptyHostForm([]), { name: 'NAS', address: '192.0.2.20', sshUser: 'root', sshSecret: 'secret' }, overrides);
assert.equal(model.validateHostForm(form({})), null);
assert.equal(model.validateHostForm(form({ name: '  ' })).code, 'name');
assert.equal(model.validateHostForm(form({ name: 'x'.repeat(65) })).code, 'name');
assert.equal(model.validateHostForm(form({ address: '' })).code, 'address');
assert.equal(model.validateHostForm(form({ address: 'tcp://192.0.2.20:2375' })).code, 'address_scheme');
assert.equal(model.validateHostForm(form({ address: 'host/path' })).code, 'address_scheme');
assert.equal(model.validateHostForm(form({ address: '[2001:db8::1]' })), null, 'a bracketed IPv6 address is fine');
assert.equal(model.validateHostForm(form({ port: '70000' })).code, 'port');
assert.equal(model.validateHostForm(form({ port: 'abc' })).code, 'port');
assert.equal(model.validateHostForm(form({ port: '2222' })), null);
assert.equal(model.validateHostForm(form({ sshUser: '' })).code, 'ssh_user');
assert.equal(model.validateHostForm(form({ sshSecret: '' })).code, 'ssh_secret');
assert.equal(model.validateHostForm(form({ id: 'h_1', originalKind: 'ssh', secretSet: true, sshSecret: '' })), null, 'editing keeps the stored secret');
assert.equal(model.validateHostForm(form({ id: 'h_1', originalKind: 'tls', secretSet: true, sshSecret: '' })).code, 'ssh_secret', 'a stored TLS bundle is no SSH secret');
assert.equal(model.validateHostForm(form({ id: 'h_1', originalKind: 'ssh', secretSet: true, clearSecret: true, sshSecret: '' })).code, 'ssh_secret', 'removing the secret needs a new one');
assert.equal(model.validateHostForm(form({ kind: 'tcp', sshUser: '', sshSecret: '' })), null, 'plain TCP has no credentials');
assert.equal(model.validateHostForm(form({ kind: 'tls', sshUser: '', sshSecret: '' })), null, 'server TLS may use the system roots');
assert.equal(model.validateHostForm(form({ kind: 'mtls', sshUser: '', sshSecret: '' })).code, 'tls_cert');
assert.equal(model.validateHostForm(form({ kind: 'mtls', tlsCert: 'C', tlsKey: '' })).code, 'tls_cert', 'a new mutual-TLS host needs the pair');
assert.equal(model.validateHostForm(form({ id: 'h_1', originalKind: 'mtls', secretSet: true, kind: 'mtls', tlsCert: 'C', tlsKey: '' })), null, 'an edit may replace one PEM field, the server merges them');
assert.equal(model.validateHostForm(form({ name: 'local' })).code, 'name_reserved');
assert.equal(model.validateHostForm(form({ name: ' LOCAL ' })).code, 'name_reserved');
assert.equal(model.validateHostForm(form({ name: 'nas' }), [{ id: 'h_x', name: 'NAS' }]).code, 'name_taken', 'names are unique ignoring case');
assert.equal(model.validateHostForm(form({ id: 'h_x', originalKind: 'ssh', secretSet: true, sshSecret: '' }), [{ id: 'h_x', name: 'NAS' }]), null, 'a host may keep its own name');
assert.equal(model.validateHostForm(form({ kind: 'mtls', tlsCert: 'C', tlsKey: 'K' })), null);
assert.equal(model.validateHostForm(form({ stacksDir: 'stacks' })).code, 'path');
assert.equal(model.validateHostForm(form({ socketPath: '/var/../run/docker.sock' })).code, 'path');
eq(plain(model.hostPayload(form({ port: '2222', color: 'coral' }))), {
    name: 'NAS', kind: 'ssh', address: '192.0.2.20', port: 2222, stacks_dir: '/opt/stacks', host_socket_path: '/var/run/docker.sock', color: 'coral', ssh_user: 'root', sort: 0, ssh_secret: 'secret'
});
const keep = model.hostPayload(form({ id: 'h_1', originalKind: 'ssh', secretSet: true, sshSecret: '' }));
assert.ok(!('ssh_secret' in keep) && !('tls' in keep) && !('clear_secret' in keep), 'an empty secret field keeps the stored secret');
eq(plain(model.hostPayload(form({ kind: 'tls', tlsCA: 'CA' })).tls), { ca: 'CA' }, 'only the typed PEM fields are sent');
eq(plain(model.hostPayload(form({ kind: 'mtls', tlsCert: 'C', tlsKey: 'K' })).tls), { cert: 'C', key: 'K' });
assert.ok(!('tls' in model.hostPayload(form({ kind: 'tls' }))), 'no PEM typed, no tls object');
assert.equal(model.hostPayload(form({ kind: 'tcp', sshUser: 'root', sshSecret: 'x' })).ssh_user, '', 'only SSH sends a user');
assert.ok(!('ssh_secret' in model.hostPayload(form({ kind: 'tcp', sshSecret: 'x' }))), 'only SSH sends an SSH secret');
assert.equal(model.hostPayload(form({ clearSecret: true })).clear_secret, true);
eq([model.hostPayload(form({ stacksDir: '', socketPath: '' })).stacks_dir, model.hostPayload(form({ stacksDir: '', socketPath: '' })).host_socket_path], ['', ''], 'an empty folder or socket is sent empty: the server keeps the stored value on edit and applies the default on create');
const edited = model.hostToForm({ id: 'h_1', name: 'NAS', kind: 'tls', address: 'nas', port: 2376, ssh_user: '', stacks_dir: '/srv/stacks', color: 'amber', secret_set: true, sort: 3 });
eq([edited.originalKind, edited.port, edited.stacksDir, edited.secretSet, edited.sort], ['tls', '2376', '/srv/stacks', true, 3]);

// ---- errors ------------------------------------------------------------------------------------------------------------
assert.equal(model.normalizeError(Object.assign(new Error('x'), { status: 502, body: { status: 'error', code: 'host_unreachable', reason: 'dns', message: 'lookup failed' } })).reason, 'dns');
assert.equal(model.normalizeError(Object.assign(new Error('x'), { status: 409, body: { code: 'container_protected_confirmation_required', owner: 'AuraGo' } })).owner, 'AuraGo');
assert.equal(model.normalizeError(Object.assign(new Error('x'), { status: 400, body: { code: 'invalid_host', field: 'address', message: 'bad' } })).field, 'address');
assert.equal(model.normalizeError(new TypeError('Failed to fetch')).code, 'network');
assert.equal(model.normalizeError(Object.assign(new Error('HTTP 403'), { status: 403, body: {} })).code, 'forbidden');
assert.equal(model.normalizeError(Object.assign(new Error('HTTP 500'), { status: 500, body: {} })).code, 'unknown');
assert.equal(model.normalizeError({ name: 'AbortError' }).code, 'aborted');

// ---- store with a fake api -----------------------------------------------------------------------------------------------
const httpError = (status, body) => Object.assign(new Error(body.message || 'HTTP ' + status), { status, body: { status: 'error', ...body } });
function fakeApi(routes) {
    const calls = [];
    const api = (url, init = {}) => {
        const method = init.method || 'GET';
        const key = method + ' ' + url.replace(/\?.*$/, '');
        calls.push({ method, url, key, body: init.body ? JSON.parse(init.body) : undefined });
        const handler = routes[key];
        if (handler === undefined) return Promise.reject(httpError(404, { code: 'not_found', message: 'no route ' + key }));
        return new Promise((resolve, reject) => {
            const abort = () => reject(Object.assign(new Error('aborted'), { name: 'AbortError' }));
            if (init.signal?.aborted) return abort();
            init.signal?.addEventListener('abort', abort, { once: true });
            Promise.resolve().then(() => (typeof handler === 'function' ? handler(url, init) : handler)).then(resolve, reject);
        });
    };
    api.calls = calls;
    api.count = key => calls.filter(call => call.key === key).length;
    return api;
}
const tick = () => new Promise(resolve => setTimeout(resolve, 0));
const B = '/api/docker-manager';

{
    // refresh loads the host list, then one container list per host; an unreachable host keeps the others working
    let nasUp = false;
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { status: 'ok', hosts: [hostA, hostB] },
        ['GET ' + B + '/hosts/local/containers']: { status: 'ok', containers: [container('web')] },
        ['GET ' + B + '/hosts/h_nas/containers']: () => { if (!nasUp) throw httpError(502, { code: 'host_unreachable', reason: 'timeout', message: 'dial timeout' }); return { status: 'ok', containers: [container('media')] }; }
    });
    const store = Store.create({ api });
    await store.refresh();
    assert.equal(store.state.hosts.length, 2);
    assert.equal(store.state.byHost.local.containers.length, 1);
    assert.equal(store.state.byHost.h_nas.error.code, 'host_unreachable');
    assert.equal(store.state.byHost.h_nas.error.reason, 'timeout');
    assert.equal(store.state.byHost.h_nas.loaded, false);
    assert.equal(model.hostHealth(store.state.hosts[1], store.state.byHost.h_nas).tone, 'bad');
    nasUp = true;
    await store.loadContainers('h_nas');
    assert.equal(store.state.byHost.h_nas.error, null, 'a recovered host clears its error');
    nasUp = false;
    await store.loadContainers('h_nas');
    assert.equal(store.state.byHost.h_nas.containers.length, 1, 'a failed refresh keeps the last containers of that host');
    assert.equal(store.state.byHost.h_nas.error.code, 'host_unreachable');
    store.setScope('h_nas');
    assert.equal(store.state.scope, 'h_nas');
    store.setScope('does-not-exist');
    assert.equal(store.state.scope, 'all', 'an unknown host id falls back to all hosts');
    store.dispose();
}

{
    // one request in flight per key
    let release;
    const gate = new Promise(resolve => { release = resolve; });
    const api = fakeApi({ ['GET ' + B + '/hosts']: { hosts: [hostA] }, ['GET ' + B + '/hosts/local/containers']: () => gate.then(() => ({ containers: [container('slow')] })) });
    const store = Store.create({ api });
    await store.loadHosts();
    const first = store.loadContainers('local'), second = store.loadContainers('local');
    assert.equal(first, second, 'the second call joins the request in flight');
    assert.equal(api.count('GET ' + B + '/hosts/local/containers'), 1);
    assert.equal(store.pending(), true);
    release();
    await first;
    assert.equal(store.pending(), false);
    assert.equal(store.state.byHost.local.containers[0].names[0], 'slow');
    store.dispose();
}

{
    // actions: POST, then a fresh list; busy blocks a double click; read_only hides the mutating actions
    let listCalls = 0;
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { hosts: [hostA] },
        ['GET ' + B + '/hosts/local/containers']: () => ({ containers: [container('web', { state: ++listCalls > 1 ? 'exited' : 'running' })] }),
        ['POST ' + B + '/hosts/local/containers/abc/stop']: {},
        ['POST ' + B + '/hosts/local/containers/abc/kill']: () => { throw httpError(403, { code: 'read_only', message: 'read only' }); },
        ['DELETE ' + B + '/hosts/local/containers/abc']: {}
    });
    const store = Store.create({ api });
    await store.refresh();
    const pendingStop = store.act('local', 'abc', 'stop', {});
    const doubleClick = await store.act('local', 'abc', 'stop', {});
    assert.equal(doubleClick.ok, false);
    assert.equal(doubleClick.error.code, 'busy');
    assert.equal(store.state.busy.size, 1);
    assert.equal((await pendingStop).ok, true);
    assert.equal(store.state.busy.size, 0);
    assert.equal(store.state.byHost.local.containers[0].state, 'exited', 'the list is reloaded after an action');
    assert.equal(store.canMutate(), true);
    const killed = await store.act('local', 'abc', 'kill', {});
    eq([killed.ok, killed.error.code], [false, 'read_only']);
    assert.equal(store.state.dockerReadonly, true);
    assert.equal(store.canMutate(), false);
    assert.equal(store.canEditHosts(), true, 'docker.readonly does not stop host management');
    await store.remove('local', 'abc', { force: true, confirmProtected: true });
    assert.equal(api.calls.find(call => call.method === 'DELETE').url, B + '/hosts/local/containers/abc?force=1&confirm=protected');
    await store.act('local', 'abc', 'restart').catch(() => {});
    assert.equal(api.calls.filter(call => call.method === 'POST').pop().url, B + '/hosts/local/containers/abc/restart', 'actions carry no confirmation, the server never asks for one');
    store.dispose();
}

{
    // docker disabled; the desktop read-only policy
    const api = fakeApi({ ['GET ' + B + '/hosts']: () => { throw httpError(503, { code: 'docker_disabled', message: 'off' }); } });
    const store = Store.create({ api, readonly: true });
    await store.refresh();
    assert.equal(store.state.disabled, true);
    assert.equal(store.state.hostsFailed, null);
    assert.equal(store.canMutate(), false);
    assert.equal(store.canEditHosts(), false);
    store.dispose();
}

{
    // polling is gated by visibility and never overlaps
    const api = fakeApi({ ['GET ' + B + '/hosts']: { hosts: [hostA] }, ['GET ' + B + '/hosts/local/containers']: { containers: [] } });
    const store = Store.create({ api });
    assert.equal(store.pollOnce(), false, 'a hidden window does not poll');
    store.setVisible(true);
    await tick(); await tick(); await tick();
    const initial = api.count('GET ' + B + '/hosts/local/containers');
    assert.ok(initial >= 1);
    const hostCalls = api.count('GET ' + B + '/hosts');
    assert.equal(store.pollOnce(), true);
    await tick(); await tick();
    assert.equal(api.count('GET ' + B + '/hosts/local/containers'), initial + 1);
    assert.equal(api.count('GET ' + B + '/hosts'), hostCalls + 1, 'the host list is polled with the containers, the server refreshes stale statuses behind it');
    store.setVisible(false);
    assert.equal(store.pollOnce(), false);
    store.dispose();
    assert.equal(store.pollOnce(), false);
}

{
    // the browser giving up on a slow host reads as the host being unreachable (timeout), not as a generic failure
    const api = fakeApi({ ['GET ' + B + '/hosts']: { hosts: [hostB] }, ['GET ' + B + '/hosts/h_nas/containers']: () => new Promise(() => {}) });
    const store = Store.create({ api, requestTimeoutMs: 30 });
    await store.loadHosts();
    await store.loadContainers('h_nas');
    eq([store.state.byHost.h_nas.error.code, store.state.byHost.h_nas.error.reason], ['host_unreachable', 'timeout']);
    assert.equal(model.hostHealth(store.state.hosts[0], store.state.byHost.h_nas).reason, 'timeout');
    store.dispose();
}

{
    // hiding the window aborts a slow poll without recording an error
    const api = fakeApi({ ['GET ' + B + '/hosts']: { hosts: [hostA] }, ['GET ' + B + '/hosts/local/containers']: () => new Promise(() => {}) });
    const store = Store.create({ api });
    await store.loadHosts();
    store.setVisible(true);
    await tick();
    store.setVisible(false);
    await tick(); await tick();
    assert.equal(store.state.byHost.local, undefined, 'an aborted poll records nothing');
    store.dispose();
}

{
    // detail drawer: logs and inspect on demand, stale answers dropped
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { hosts: [hostA] },
        ['GET ' + B + '/hosts/local/containers']: { containers: [container('web')] },
        ['GET ' + B + '/hosts/local/containers/' + 'web'.padEnd(64, '0') + '/logs']: (url) => ({ logs: 'line from ' + url.split('tail=')[1] }),
        ['GET ' + B + '/hosts/local/containers/' + 'web'.padEnd(64, '0') + '/inspect']: { inspect: { Id: 'x' } }
    });
    const store = Store.create({ api });
    await store.refresh();
    const row = model.buildRows(store.state.hosts, store.state.byHost, 'all')[0];
    store.openDetail(row);
    assert.equal(store.state.detail.tab, 'overview');
    assert.equal(api.count('GET ' + B + '/hosts/local/containers/' + 'web'.padEnd(64, '0') + '/logs'), 0, 'logs load only when the tab opens');
    store.setDetailTab('logs');
    await tick(); await tick();
    assert.equal(store.state.detail.logs.status, 'ok');
    assert.equal(store.state.detail.logs.text, 'line from 200');
    await store.loadLogs(500);
    assert.equal(store.state.detail.logs.text, 'line from 500');
    await store.loadLogs(7);
    assert.equal(store.state.detail.logs.text, 'line from 500', 'an unsupported tail keeps the current one');
    store.setDetailTab('inspect');
    await tick(); await tick();
    eq(plain(store.state.detail.inspect.data), { Id: 'x' });
    const stale = store.loadLogs(100);
    store.closeDetail();
    await stale;
    assert.equal(store.state.detail, null, 'closing the drawer drops a late answer');
    store.dispose();
}

{
    // host management
    const api = fakeApi({
        ['GET ' + B + '/hosts']: { hosts: [hostA] },
        ['POST ' + B + '/hosts']: (url, init) => ({ host: { id: 'h_new', name: JSON.parse(init.body).name, kind: 'tcp', address: 'a', status: { state: 'unknown' } } }),
        ['GET ' + B + '/hosts/h_new/containers']: { containers: [] },
        ['POST ' + B + '/hosts/test']: { host_status: { state: 'ok', engine_version: '27.1.0', api_version: '1.46' } },
        ['DELETE ' + B + '/hosts/h_new']: {},
        ['GET ' + B + '/nests']: { nests: [{ id: 'n1', name: 'Nest', deploy_method: 'docker_ssh' }] },
        ['POST ' + B + '/hosts/import-nest/n1']: { host: { id: 'h_n1', name: 'Nest', kind: 'ssh', address: 'n', status: { state: 'unknown' } } },
        ['POST ' + B + '/hosts/h_new/test']: { host: { id: 'h_new', name: 'edge', kind: 'tcp', address: 'a', status: { state: 'ok' } } }
    });
    const store = Store.create({ api });
    await store.loadHosts();
    assert.equal((await store.testDraft({ name: 'x' })).engine_version, '27.1.0');
    assert.ok(!('id' in api.calls.find(call => call.key === 'POST ' + B + '/hosts/test').body), 'a new host is tested without an id');
    await store.testDraft({ name: 'x' }, 'h_new');
    assert.equal(api.calls.filter(call => call.key === 'POST ' + B + '/hosts/test').pop().body.id, 'h_new', 'an edited host is tested with its id so the stored secrets are reused');
    const created = await store.saveHost('', { name: 'edge', kind: 'tcp' });
    assert.equal(created.id, 'h_new');
    assert.equal(store.state.hosts.length, 2);
    assert.equal((await store.testHost('h_new')).status.state, 'ok');
    assert.equal(store.state.hosts.find(host => host.id === 'h_new').status.state, 'ok');
    assert.equal((await store.loadNests())[0].id, 'n1');
    assert.equal((await store.importNest('n1')).id, 'h_n1');
    store.setScope('h_new');
    await store.deleteHost('h_new');
    assert.equal(store.state.scope, 'all', 'deleting the scoped host falls back to all hosts');
    assert.ok(!store.state.hosts.some(host => host.id === 'h_new'));
    await assert.rejects(store.saveHost('local', { name: 'x' }), error => error.dm.code === 'not_found', 'a failed save rejects with the normalized info');
    store.dispose();
}

console.log('docker-manager ui tests: model and store ok');
```

- [ ] **Step 2: Run it; expect FAIL.**

```powershell
node scripts/test-docker-manager-ui.mjs
```

Expected: FAIL with `ENOENT: no such file or directory, open '...ui\js\desktop\apps\docker-manager-model.js'`.

- [ ] **Step 3: Create the model.**

```js
/* Docker Manager model: pure helpers, no DOM and no network. A classic script (window.DockerManagerModel) so that
   scripts/test-docker-manager-ui.mjs can run it in a Node vm. It owns the container status and problem derivation,
   the grouping by compose project, search and state filters, the multi-host merge, the host chip data, the host form
   rules and the error normalisation that the store and the views share. */
(function () {
    'use strict';

    const HOST_COLORS = ['blue', 'teal', 'purple', 'coral', 'pink', 'amber', 'green', 'gray'];
    const HOST_KINDS = ['ssh', 'tls', 'mtls', 'tcp'];
    const DEFAULT_PORTS = { tcp: 2375, tls: 2376, mtls: 2376, ssh: 22 };
    const STATE_FILTERS = ['all', 'running', 'stopped', 'problem'];
    const ACTIONS = ['start', 'stop', 'restart', 'pause', 'unpause', 'kill'];
    const REASONS = ['dns', 'refused', 'timeout', 'tls', 'auth', 'ssh_host_key', 'socket_missing', 'api_too_old', 'unknown'];
    const NEST_METHODS = ['docker_local', 'docker_remote', 'docker_ssh'];
    const PROTECTED_ACTIONS = ['stop', 'restart', 'pause', 'kill', 'remove'];
    const DEFAULT_STACKS_DIR = '/opt/stacks';
    const DEFAULT_SOCKET = '/var/run/docker.sock';
    const TONE_RANK = { bad: 3, warn: 2, ok: 1, idle: 0 };

    const text = value => (value == null ? '' : String(value));
    const lower = value => text(value).toLowerCase();
    const array = value => (Array.isArray(value) ? value : []);
    const colorOf = value => (HOST_COLORS.includes(lower(value)) ? lower(value) : 'gray');
    const compareText = (a, b) => text(a).localeCompare(text(b), undefined, { numeric: true, sensitivity: 'base' });

    // ---- containers ------------------------------------------------------------------------------------------------

    // Docker reports the exit code only inside the status text: "Exited (137) 3 minutes ago".
    function exitCode(container) {
        const match = /^exited \((-?\d+)\)/i.exec(text(container && container.status).trim());
        return match ? Number(match[1]) : null;
    }

    const containerId = container => text(container && (container.full_id || container.id));

    function containerName(container) {
        const names = array(container && container.names);
        return text(names[0] || (container && container.id) || '').replace(/^\//, '');
    }

    // Why a container is protected, or null. The server decides what is protected; the UI only explains it.
    function protectedReason(container) {
        const c = container || {};
        if (c.protected_owner) return { kind: 'owner', owner: text(c.protected_owner) };
        if (c.self) return { kind: 'self', owner: '' };
        if (c.docker_endpoint) return { kind: 'endpoint', owner: '' };
        if (c.shared_network) return { kind: 'network', owner: '' };
        return null;
    }
    const isProtected = container => !!protectedReason(container);
    const needsProtectedConfirm = (container, action) => isProtected(container) && PROTECTED_ACTIONS.includes(action);

    // The single reason a container counts as a problem: unhealthy, restarting, dead, or exited with a non-zero code.
    function problemOf(container) {
        const c = container || {};
        const state = lower(c.state);
        if (lower(c.health) === 'unhealthy' && state === 'running') return 'unhealthy';
        if (state === 'restarting') return 'restarting';
        if (state === 'dead') return 'dead';
        if (state === 'exited') {
            const code = exitCode(c);
            if (code !== null && code !== 0) return 'exited';
        }
        return '';
    }

    // tone: ok (green), warn (amber), bad (red), idle (grey). A tone is always shown together with the id as text.
    function statusOf(container) {
        const c = container || {};
        const state = lower(c.state), health = lower(c.health), problem = problemOf(c);
        if (state === 'running') {
            if (health === 'unhealthy') return { id: 'unhealthy', tone: 'warn', problem };
            if (health === 'starting') return { id: 'starting', tone: 'ok', problem };
            return { id: 'running', tone: 'ok', problem };
        }
        if (state === 'paused') return { id: 'paused', tone: 'idle', problem };
        if (state === 'restarting') return { id: 'restarting', tone: 'warn', problem };
        if (state === 'exited') {
            const code = exitCode(c);
            return { id: 'exited', tone: problem ? 'bad' : 'idle', code: code === null ? 0 : code, problem };
        }
        if (state === 'dead') return { id: 'dead', tone: 'bad', problem };
        if (state === 'created') return { id: 'created', tone: 'idle', problem };
        if (state === 'removing') return { id: 'removing', tone: 'idle', problem };
        return { id: 'unknown', tone: 'idle', problem };
    }

    function stateGroup(container) {
        const state = lower(container && container.state);
        if (state === 'running') return 'running';
        if (state === 'exited' || state === 'created' || state === 'dead') return 'stopped';
        return 'other';
    }

    function matchesFilter(container, filter) {
        switch (filter) {
            case 'running': return lower(container && container.state) === 'running';
            case 'stopped': return stateGroup(container) === 'stopped';
            case 'problem': return problemOf(container) !== '';
            default: return true;
        }
    }

    // Which buttons a row shows. Nothing in a read-only window; protected containers keep their buttons and ask first.
    function actionsFor(container, readonly) {
        if (readonly) return { toggle: '', restart: false, menu: [], remove: false };
        const state = lower(container && container.state);
        const out = { toggle: '', restart: false, menu: [], remove: state !== 'removing' };
        if (state === 'running') { out.toggle = 'stop'; out.restart = true; out.menu = ['pause', 'kill']; }
        else if (state === 'paused') { out.toggle = 'unpause'; out.menu = ['kill']; }
        else if (state === 'restarting') { out.toggle = 'stop'; out.menu = ['kill']; }
        else if (state === 'exited' || state === 'created') out.toggle = 'start';
        return out;
    }

    // ---- ports, images, dates ----------------------------------------------------------------------------------------

    // Docker lists a published port once per address family; the table shows it once, published ports first.
    function portItems(ports) {
        const seen = new Set(), items = [];
        for (const port of array(ports)) {
            const type = text(port.type || 'tcp'), published = Number(port.public) || 0, internal = Number(port.private) || 0;
            const label = (published ? published + '→' : '') + internal + '/' + type;
            if (seen.has(label)) continue;
            seen.add(label);
            items.push({ label, published, internal, type });
        }
        items.sort((a, b) => (b.published ? 1 : 0) - (a.published ? 1 : 0) || (a.published || a.internal) - (b.published || b.internal));
        return items;
    }

    function shortImage(image) {
        const value = text(image);
        if (value.startsWith('sha256:')) return value.slice(0, 19);
        return value.replace(/^docker\.io\/library\//, '').replace(/^docker\.io\//, '');
    }

    function formatCreated(seconds, lang) {
        const n = Number(seconds);
        if (!n) return '';
        const date = new Date(n * 1000);
        try { return date.toLocaleString(lang || undefined, { dateStyle: 'medium', timeStyle: 'short' }); } catch (error) { return date.toISOString(); }
    }

    function mountLabel(mount) {
        const m = mount || {};
        const from = text(m.name || m.source);
        return from ? from + ' → ' + text(m.destination) : text(m.destination);
    }

    // ---- rows, filters, groups --------------------------------------------------------------------------------------

    // One row per container of the hosts in scope ('all' or a host id). The key is stable across polls.
    function buildRows(hosts, byHost, scope) {
        const rows = [];
        for (const host of array(hosts)) {
            if (scope && scope !== 'all' && host.id !== scope) continue;
            const entry = (byHost || {})[host.id];
            for (const c of array(entry && entry.containers)) {
                rows.push({ key: host.id + '/' + containerId(c), hostId: host.id, hostName: text(host.name), hostColor: colorOf(host.color), c });
            }
        }
        return rows;
    }

    function haystack(row) {
        const c = row.c;
        const ports = array(c.ports).map(port => [port.public, port.private].filter(Boolean).join(' ')).join(' ');
        return lower([containerName(c), array(c.names).join(' '), c.image, c.id, c.full_id, c.compose_project, c.compose_service, c.state, row.hostName, ports].join(' '));
    }

    // Every word of the query must appear somewhere in the row (name, image, project, service, id, host, ports, state).
    function matchesQuery(row, query) {
        const terms = lower(query).split(/\s+/).filter(Boolean);
        if (!terms.length) return true;
        const hay = haystack(row);
        return terms.every(term => hay.includes(term));
    }

    function filterRows(rows, options) {
        const o = options || {};
        return array(rows).filter(row => matchesQuery(row, o.query) && matchesFilter(row.c, o.filter));
    }

    // The chip counts follow the search but not the state filter, so a chip always shows what it would give.
    function counts(rows, query) {
        const out = { all: 0, running: 0, stopped: 0, problem: 0 };
        for (const row of array(rows)) {
            if (!matchesQuery(row, query)) continue;
            out.all += 1;
            if (matchesFilter(row.c, 'running')) out.running += 1;
            if (matchesFilter(row.c, 'stopped')) out.stopped += 1;
            if (matchesFilter(row.c, 'problem')) out.problem += 1;
        }
        return out;
    }

    function groupCounts(rows) {
        let running = 0, problem = 0;
        for (const row of rows) {
            if (lower(row.c.state) === 'running') running += 1;
            if (problemOf(row.c)) problem += 1;
        }
        return { total: rows.length, running, problem };
    }

    // A project is green when every container runs, grey when none does, amber otherwise; a failed container makes it red.
    function groupTone(rows) {
        const tones = rows.map(row => statusOf(row.c).tone);
        if (tones.includes('bad')) return 'bad';
        if (tones.includes('warn')) return 'warn';
        const running = tones.filter(tone => tone === 'ok').length;
        if (running === rows.length) return 'ok';
        return running === 0 ? 'idle' : 'warn';
    }

    // Compose projects first (by name, then host order), then the containers without a project. In the multi-host view the
    // same project name on two hosts stays two groups; the single containers of all hosts share one group.
    function groupRows(rows, options) {
        const o = options || {};
        const multi = !!o.multiHost, hostOrder = array(o.hostOrder);
        const groups = new Map();
        for (const row of array(rows)) {
            const project = text(row.c.compose_project);
            const key = project ? 'p:' + (multi ? row.hostId + '|' : '') + project : 's:' + (multi ? '' : row.hostId);
            let group = groups.get(key);
            if (!group) {
                group = { key, kind: project ? 'project' : 'single', project, hostId: project ? row.hostId : '', hostName: project ? row.hostName : '', hostColor: project ? row.hostColor : '', rows: [] };
                groups.set(key, group);
            }
            group.rows.push(row);
        }
        const list = [...groups.values()];
        for (const group of list) {
            group.rows.sort((a, b) => compareText(a.c.compose_service || containerName(a.c), b.c.compose_service || containerName(b.c))
                || compareText(containerName(a.c), containerName(b.c)) || compareText(a.hostName, b.hostName));
            group.counts = groupCounts(group.rows);
            group.tone = group.kind === 'project' ? groupTone(group.rows) : 'idle';
        }
        const rank = id => { const index = hostOrder.indexOf(id); return index < 0 ? 999 : index; };
        list.sort((a, b) => (a.kind === b.kind ? 0 : a.kind === 'project' ? -1 : 1) || compareText(a.project, b.project) || rank(a.hostId) - rank(b.hostId));
        return list;
    }

    // A lone "single containers" group needs no header row.
    const showGroupHeaders = groups => !(array(groups).length === 1 && groups[0].kind === 'single');

    // The rows the keyboard can reach, in display order. A collapsed group hides its rows.
    function navItems(groups, collapsed) {
        const items = [], headers = showGroupHeaders(groups);
        for (const group of array(groups)) {
            if (headers) items.push({ type: 'group', key: 'g:' + group.key, group: group.key });
            if (headers && collapsed && collapsed.has(group.key)) continue;
            for (const row of group.rows) items.push({ type: 'row', key: row.key, group: group.key });
        }
        return items;
    }

    // ---- hosts -----------------------------------------------------------------------------------------------------

    const reasonCode = code => (REASONS.includes(code) ? code : 'unknown');

    // The server sends "0001-01-01T00:00:00Z" as checked_at for a host that was never tested. That means "not checked yet",
    // never a date, whatever else the status says.
    function normalizeStatus(status) {
        const s = status && typeof status === 'object' ? status : {};
        return text(s.checked_at).startsWith('0001-') ? Object.assign({}, s, { state: 'unknown', code: '' }) : s;
    }

    // The tone and the reason shown for a host. A failed container load beats an older "ok" from the host list.
    function hostHealth(host, entry) {
        const status = normalizeStatus(host && host.status);
        const failed = entry && entry.error;
        if (failed && failed.code === 'host_unreachable') return { tone: 'bad', state: 'error', reason: reasonCode(failed.reason || status.code), message: text(failed.message) };
        if (failed) return { tone: 'warn', state: 'error', reason: 'unknown', message: text(failed.message) };
        if (status.state === 'error') return { tone: 'bad', state: 'error', reason: reasonCode(status.code), message: text(status.message) };
        if (status.state === 'ok' || (entry && entry.loaded)) return { tone: 'ok', state: 'ok', reason: '', message: '' };
        return { tone: 'idle', state: 'unknown', reason: '', message: '' };
    }

    function hostAddress(host) {
        const h = host || {};
        const address = text(h.address);
        if (h.kind === 'local') return address;
        const port = Number(h.port) || DEFAULT_PORTS[h.kind] || 0;
        const bare = address.startsWith('[') || !address.includes(':') ? address : '[' + address + ']';
        return port ? bare + ':' + port : bare;
    }

    function hostCounts(hostId, byHost) {
        const list = array(byHost && byHost[hostId] && byHost[hostId].containers);
        return { running: list.filter(c => lower(c.state) === 'running').length, total: list.length };
    }

    function totalCounts(hosts, byHost) {
        const out = { running: 0, total: 0 };
        for (const host of array(hosts)) {
            const c = hostCounts(host.id, byHost);
            out.running += c.running;
            out.total += c.total;
        }
        return out;
    }

    const defaultPort = kind => DEFAULT_PORTS[kind] || 0;
    const nextColor = hosts => HOST_COLORS.find(color => !array(hosts).some(host => host.color === color)) || HOST_COLORS[array(hosts).length % HOST_COLORS.length];

    // ---- host form ---------------------------------------------------------------------------------------------------

    function emptyHostForm(hosts) {
        return {
            id: '', name: '', kind: 'ssh', originalKind: '', address: '', port: '', sshUser: '', sshSecret: '', tlsCA: '', tlsCert: '', tlsKey: '',
            clearSecret: false, stacksDir: DEFAULT_STACKS_DIR, socketPath: DEFAULT_SOCKET, color: nextColor(hosts), sort: 0, secretSet: false
        };
    }

    function hostToForm(host) {
        const h = host || {};
        return Object.assign(emptyHostForm([]), {
            id: text(h.id), name: text(h.name), kind: text(h.kind || 'ssh'), originalKind: text(h.kind), address: text(h.address), port: h.port ? String(h.port) : '',
            sshUser: text(h.ssh_user), stacksDir: text(h.stacks_dir) || DEFAULT_STACKS_DIR, socketPath: text(h.host_socket_path) || DEFAULT_SOCKET,
            color: colorOf(h.color), sort: Number(h.sort) || 0, secretSet: !!h.secret_set
        });
    }

    const secretFamily = kind => (kind === 'ssh' ? 'ssh' : kind === 'tls' || kind === 'mtls' ? 'tls' : '');

    // A stored secret can stay only for the same kind of secret and only when it is not being removed.
    function storedSecretUsable(form) {
        const f = form || {};
        return !!f.id && !!f.secretSet && !f.clearSecret && secretFamily(text(f.kind)) === secretFamily(text(f.originalKind));
    }

    // Returns null or {field, code}; code names the docker_manager.v_<code> message. `hosts` is the current host list: names
    // are unique ignoring case and "Local" is reserved. The server validates again and has the last word.
    function validateHostForm(form, hosts) {
        const f = form || {};
        const name = text(f.name).trim(), kind = text(f.kind), address = text(f.address).trim(), port = text(f.port).trim();
        if (!name || [...name].length > 64) return { field: 'name', code: 'name' };
        if (lower(name) === 'local') return { field: 'name', code: 'name_reserved' };
        if (array(hosts).some(item => item.id !== f.id && lower(item.name) === lower(name))) return { field: 'name', code: 'name_taken' };
        if (!address) return { field: 'address', code: 'address' };
        if (/:\/\/|[\/\\\s]/.test(address)) return { field: 'address', code: 'address_scheme' };
        if (port !== '' && (!/^\d+$/.test(port) || Number(port) < 1 || Number(port) > 65535)) return { field: 'port', code: 'port' };
        if (kind === 'ssh') {
            if (!text(f.sshUser).trim()) return { field: 'sshUser', code: 'ssh_user' };
            if (text(f.sshSecret) === '' && !storedSecretUsable(f)) return { field: 'sshSecret', code: 'ssh_secret' };
        }
        // The server merges the TLS fields one by one, so an edit may send any subset; a new host needs the whole pair.
        if (kind === 'mtls' && !(text(f.tlsCert).trim() && text(f.tlsKey).trim()) && !storedSecretUsable(f)) return { field: 'tlsCert', code: 'tls_cert' };
        for (const [field, value] of [['stacksDir', f.stacksDir], ['socketPath', f.socketPath]]) {
            const path = text(value).trim();
            if (path && (!path.startsWith('/') || path.split('/').includes('..'))) return { field, code: 'path' };
        }
        return null;
    }

    // The HostWrite body. Secrets are sent only when typed; an empty field keeps what the server stores.
    function hostPayload(form) {
        const f = form || {}, kind = text(f.kind);
        const payload = {
            name: text(f.name).trim(), kind, address: text(f.address).trim(), port: Number(text(f.port).trim()) || 0,
            stacks_dir: text(f.stacksDir).trim(), host_socket_path: text(f.socketPath).trim(),
            color: colorOf(f.color), ssh_user: kind === 'ssh' ? text(f.sshUser).trim() : '', sort: Number(f.sort) || 0
        };
        if (kind === 'ssh' && text(f.sshSecret) !== '') payload.ssh_secret = text(f.sshSecret);
        // Only the PEM fields that were typed are sent; the server keeps the stored value of every field that is absent.
        const tls = {};
        for (const [key, value] of [['ca', f.tlsCA], ['cert', f.tlsCert], ['key', f.tlsKey]]) if (text(value).trim()) tls[key] = text(value).trim();
        if ((kind === 'tls' || kind === 'mtls') && Object.keys(tls).length) payload.tls = tls;
        if (f.clearSecret) payload.clear_secret = true;
        return payload;
    }

    // ---- errors ------------------------------------------------------------------------------------------------------

    // One shape for every failure: {code, message, reason, owner, field, status}. The desktop api() throws an Error that
    // carries the response body and status; a failed fetch carries neither.
    function normalizeError(error) {
        const e = error || {};
        const body = e.body && typeof e.body === 'object' ? e.body : {};
        const status = Number(e.status) || 0;
        if (e.name === 'AbortError' || e.code === 'aborted') return { code: 'aborted', message: '', reason: '', owner: '', field: '', status };
        let code = text(body.code || e.code);
        if (!code) code = status === 403 ? 'forbidden' : status === 0 ? 'network' : 'unknown';
        return { code, message: text(body.message || e.message), reason: text(body.reason), owner: text(body.owner), field: text(body.field), status };
    }

    window.DockerManagerModel = {
        HOST_COLORS, HOST_KINDS, DEFAULT_PORTS, STATE_FILTERS, ACTIONS, REASONS, NEST_METHODS, DEFAULT_STACKS_DIR, DEFAULT_SOCKET, TONE_RANK,
        colorOf, compareText, exitCode, containerId, containerName, protectedReason, isProtected, needsProtectedConfirm, problemOf, statusOf, stateGroup,
        matchesFilter, actionsFor, portItems, shortImage, formatCreated, mountLabel, buildRows, matchesQuery, filterRows, counts, groupRows,
        showGroupHeaders, navItems, reasonCode, normalizeStatus, hostHealth, hostAddress, hostCounts, totalCounts, defaultPort, nextColor, emptyHostForm,
        hostToForm, storedSecretUsable, validateHostForm, hostPayload, normalizeError
    };
})();
```

- [ ] **Step 4: Create the store.**

```js
/* Docker Manager store: owns the requests to /api/docker-manager, the polling and the state the views render.
   A classic script (window.DockerManagerStore). It touches the DOM only in attach() (visibility observers), so
   scripts/test-docker-manager-ui.mjs can run it in a Node vm with a fake api. Rules it keeps:
   - every request goes through the desktop api() it is given; the Docker engine is never contacted from the browser;
   - one request in flight per key (the host list, one container list per host, logs, inspect): a second call joins it;
   - polling every 5 s only while the window is visible; hiding the window aborts the polls;
   - a failed container list keeps the last containers of that host and records the error, so one unreachable host
     never empties the table or blocks the other hosts. */
(function () {
    'use strict';

    const BASE = '/api/docker-manager';
    const POLL_MS = 5000;
    // An unreachable host can take the server up to 30 s to give up, so the browser waits longer than that.
    const REQUEST_TIMEOUT_MS = 35000;
    const TEST_TIMEOUT_MS = 45000;
    const LOG_TAILS = [100, 200, 500, 1000, 5000];
    const enc = encodeURIComponent;

    function create(options) {
        const opts = options || {};
        const model = window.DockerManagerModel;
        const api = opts.api;
        const requestTimeout = Number(opts.requestTimeoutMs) > 0 ? Number(opts.requestTimeoutMs) : REQUEST_TIMEOUT_MS;
        if (typeof api !== 'function') throw new TypeError('DockerManagerStore needs an api function');
        const lifetime = new AbortController();
        const listeners = new Set();
        const state = {
            hosts: [], hostsLoaded: false, hostsFailed: null, disabled: false,
            scope: typeof opts.scope === 'string' && opts.scope ? opts.scope : 'all', byHost: {}, detail: null, busy: new Set(),
            readonly: !!opts.readonly, dockerReadonly: false,
            visible: false, disposed: false, lastRefreshAt: 0
        };
        const inflight = new Map();
        const controllers = new Map();
        const sequence = new Map();
        let timer = 0, win = null, root = null, inView = true, detach = () => {}, detailGen = 0;

        function emit() {
            if (state.disposed) return;
            for (const listener of [...listeners]) {
                try { listener(state); } catch (error) { console.error('Docker Manager:', error); }
            }
        }
        function subscribe(listener) { listeners.add(listener); return () => listeners.delete(listener); }

        // ---- requests --------------------------------------------------------------------------------------------

        // Resolves with the response body. Rejects with an Error whose .dm is {code, message, reason, owner, field, status}.
        // A request registered under `key` can be aborted by that key (hiding the window, closing the drawer).
        function request(path, init, key) {
            const o = init || {};
            const controller = new AbortController();
            let timedOut = false;
            const onEnd = () => controller.abort();
            if (lifetime.signal.aborted) controller.abort(); else lifetime.signal.addEventListener('abort', onEnd, { once: true });
            const timeout = setTimeout(() => { timedOut = true; controller.abort(); }, o.timeout || requestTimeout);
            if (key) controllers.set(key, controller);
            return (async () => {
                try {
                    const init2 = { method: o.method || 'GET', signal: controller.signal };
                    if (o.body !== undefined) { init2.headers = { 'Content-Type': 'application/json' }; init2.body = JSON.stringify(o.body); }
                    return await api(BASE + path, init2);
                } catch (error) {
                    const info = model.normalizeError(error);
                    if (timedOut) info.code = 'timeout';
                    throw Object.assign(new Error(info.message || info.code), { dm: info });
                } finally {
                    clearTimeout(timeout);
                    lifetime.signal.removeEventListener('abort', onEnd);
                    if (key && controllers.get(key) === controller) controllers.delete(key);
                }
            })();
        }

        // Records what a failure means for the whole app and returns its info; null for an aborted request.
        function noteError(error) {
            const info = (error && error.dm) || model.normalizeError(error);
            if (info.code === 'aborted') return null;
            if (info.code === 'docker_disabled') state.disabled = true;
            if (info.code === 'read_only') state.dockerReadonly = true;
            return info;
        }

        function once(key, fn) {
            if (inflight.has(key)) return inflight.get(key);
            const promise = (async () => {
                try { return await fn(); } finally { if (inflight.get(key) === promise) inflight.delete(key); }
            })();
            inflight.set(key, promise);
            return promise;
        }
        const pending = () => [...inflight.keys()].some(key => key === 'hosts' || key.startsWith('c:'));

        // ---- hosts -----------------------------------------------------------------------------------------------

        function mergeHost(host) {
            if (!host || !host.id) return;
            const index = state.hosts.findIndex(item => item.id === host.id);
            state.hosts = index < 0 ? state.hosts.concat([host]) : state.hosts.map((item, at) => (at === index ? host : item));
        }

        function loadHosts() {
            return once('hosts', async () => {
                try {
                    const body = await request('/hosts', {}, 'hosts');
                    if (state.disposed) return false;
                    state.hosts = Array.isArray(body.hosts) ? body.hosts : [];
                    state.hostsLoaded = true;
                    state.hostsFailed = null;
                    state.disabled = false;
                    const ids = new Set(state.hosts.map(host => host.id));
                    for (const id of Object.keys(state.byHost)) if (!ids.has(id)) delete state.byHost[id];
                    if (state.scope !== 'all' && !ids.has(state.scope)) state.scope = 'all';
                    return true;
                } catch (error) {
                    const info = noteError(error);
                    if (info && !state.disabled) state.hostsFailed = info;
                    return false;
                } finally { emit(); }
            });
        }

        function loadContainers(hostId) {
            return once('c:' + hostId, async () => {
                const seq = (sequence.get(hostId) || 0) + 1;
                sequence.set(hostId, seq);
                try {
                    const body = await request('/hosts/' + enc(hostId) + '/containers', {}, 'c:' + hostId);
                    if (state.disposed || sequence.get(hostId) !== seq) return false;
                    state.byHost[hostId] = { containers: Array.isArray(body.containers) ? body.containers : [], loaded: true, error: null, at: Date.now() };
                    return true;
                } catch (error) {
                    let info = noteError(error);
                    if (state.disposed || !info || sequence.get(hostId) !== seq) return false;
                    // The browser giving up on a slow host is the same news as the server giving up: unreachable, timed out.
                    if (info.code === 'timeout') info = Object.assign({}, info, { code: 'host_unreachable', reason: 'timeout' });
                    const previous = state.byHost[hostId] || { containers: [], loaded: false, at: 0 };
                    state.byHost[hostId] = Object.assign({}, previous, { error: info });
                    return false;
                } finally { emit(); }
            });
        }

        // After an action the list must come from the engine's state after the action, never from a poll that started before.
        function reloadContainers(hostId) {
            controllers.get('c:' + hostId)?.abort();
            inflight.delete('c:' + hostId);
            return loadContainers(hostId);
        }

        function scopeIds() {
            if (state.scope === 'all') return state.hosts.map(host => host.id);
            return state.hosts.some(host => host.id === state.scope) ? [state.scope] : [];
        }

        async function refresh() {
            if (state.disposed) return;
            await loadHosts();
            if (state.disposed || state.disabled) return;
            await Promise.all(scopeIds().map(loadContainers));
            state.lastRefreshAt = Date.now();
            emit();
        }

        function setScope(scope) {
            const next = scope !== 'all' && state.hosts.some(host => host.id === scope) ? scope : 'all';
            if (next === state.scope) return;
            state.scope = next;
            emit();
            if (state.visible) void Promise.all(scopeIds().map(loadContainers));
        }

        // ---- polling and visibility -------------------------------------------------------------------------------

        function hiddenNow() {
            return state.disposed || (typeof document !== 'undefined' && document.hidden)
                || !!(root && typeof root.checkVisibility === 'function' && !root.checkVisibility());
        }
        function pollOnce() {
            if (state.disposed || !state.visible || state.disabled || hiddenNow()) return false;
            // GET /hosts answers from cached statuses and refreshes stale ones in the background, so it is polled like the lists.
            const jobs = scopeIds().map(loadContainers);
            jobs.push(loadHosts());
            void Promise.all(jobs);
            return true;
        }
        function schedule() {
            clearTimeout(timer);
            timer = 0;
            if (state.disposed || !state.visible) return;
            timer = setTimeout(() => { timer = 0; pollOnce(); schedule(); }, POLL_MS);
        }
        function abortPolls() {
            for (const [key, controller] of [...controllers]) if (key === 'hosts' || key.startsWith('c:')) controller.abort();
        }
        function setVisible(next) {
            if (state.disposed || next === state.visible) return;
            state.visible = next;
            if (!next) { clearTimeout(timer); timer = 0; abortPolls(); }
            else { void refresh(); schedule(); }
            emit();
        }
        function attach(target) {
            const t = target || {};
            win = t.win || null;
            root = t.root || null;
            const check = () => {
                const shown = !state.disposed && !document.hidden && inView
                    && (!win || (!win.hidden && !win.classList.contains('vd-space-hidden') && win.style.display !== 'none' && win.getClientRects().length > 0));
                setVisible(shown);
            };
            document.addEventListener('visibilitychange', check);
            const windowObserver = typeof MutationObserver === 'function' && win ? new MutationObserver(check) : null;
            windowObserver?.observe(win, { attributes: true, attributeFilter: ['style', 'class', 'hidden', 'data-space-hidden'] });
            const intersection = typeof IntersectionObserver === 'function' && root
                ? new IntersectionObserver(entries => { inView = !!entries[entries.length - 1]?.isIntersecting; check(); }, { threshold: 0.01 }) : null;
            if (intersection) { inView = false; intersection.observe(root); }
            const onPolicy = event => {
                const detail = (event && event.detail) || {};
                const readonly = detail.readonly === true || detail.read_only === true || detail.enabled === false ? true
                    : detail.readonly === false || detail.read_only === false || detail.enabled === true ? false : null;
                if (readonly !== null && readonly !== state.readonly) { state.readonly = readonly; emit(); }
            };
            document.addEventListener('aurago:desktop-policy', onPolicy);
            detach = () => {
                document.removeEventListener('visibilitychange', check);
                document.removeEventListener('aurago:desktop-policy', onPolicy);
                windowObserver?.disconnect();
                intersection?.disconnect();
            };
            check();
        }

        // ---- container actions -------------------------------------------------------------------------------------

        const canMutate = () => !state.readonly && !state.dockerReadonly;
        const canEditHosts = () => !state.readonly;
        const busyKey = (hostId, containerId) => hostId + '/' + containerId;

        async function mutateContainer(hostId, containerId, run) {
            const key = busyKey(hostId, containerId);
            if (state.disposed || state.busy.has(key)) return { ok: false, error: { code: 'busy' } };
            state.busy.add(key);
            emit();
            try {
                await run();
                if (state.disposed) return { ok: false, error: { code: 'aborted' } };
                await reloadContainers(hostId);
                return { ok: true };
            } catch (error) {
                return { ok: false, error: noteError(error) || { code: 'aborted' } };
            } finally {
                state.busy.delete(key);
                emit();
            }
        }

        // action: start | stop | restart | pause | unpause | kill. The server never asks to confirm these.
        function act(hostId, containerId, action) {
            return mutateContainer(hostId, containerId, () => request('/hosts/' + enc(hostId) + '/containers/' + enc(containerId) + '/' + enc(action), { method: 'POST' }));
        }

        function remove(hostId, containerId, o) {
            const params = [];
            if (o && o.force) params.push('force=1');
            if (o && o.confirmProtected) params.push('confirm=protected');
            const query = params.length ? '?' + params.join('&') : '';
            return mutateContainer(hostId, containerId, () => request('/hosts/' + enc(hostId) + '/containers/' + enc(containerId) + query, { method: 'DELETE' }));
        }

        // ---- detail drawer: overview comes from the list, logs and inspect are fetched on demand -------------------

        function abortDetail() { controllers.get('logs')?.abort(); controllers.get('inspect')?.abort(); detailGen += 1; }

        function ensureDetailData() {
            const d = state.detail;
            if (!d) return;
            if (d.tab === 'logs' && d.logs.status === 'idle') void loadLogs();
            if (d.tab === 'inspect' && d.inspect.status === 'idle') void loadInspect();
        }

        function openDetail(row, tab) {
            if (state.disposed || !row) return;
            const same = state.detail && state.detail.key === row.key;
            if (!same) {
                abortDetail();
                state.detail = {
                    key: row.key, hostId: row.hostId, id: row.c.full_id || row.c.id, tab: tab || 'overview',
                    logs: { status: 'idle', text: '', tail: 200, error: null }, inspect: { status: 'idle', data: null, error: null }
                };
            } else if (tab) state.detail.tab = tab;
            emit();
            ensureDetailData();
        }
        function closeDetail() {
            if (!state.detail) return;
            abortDetail();
            state.detail = null;
            emit();
        }
        function setDetailTab(tab) {
            if (!state.detail || state.detail.tab === tab) return;
            state.detail.tab = tab;
            emit();
            ensureDetailData();
        }

        async function loadDetailPart(part, route, apply) {
            const target = state.detail;
            if (!target) return false;
            const gen = detailGen;
            controllers.get(part)?.abort();
            target[part].status = 'loading';
            emit();
            try {
                const body = await request('/hosts/' + enc(target.hostId) + '/containers/' + enc(target.id) + route, {}, part);
                if (state.disposed || gen !== detailGen || state.detail !== target) return false;
                apply(target, body);
                target[part].status = 'ok';
                target[part].error = null;
            } catch (error) {
                const info = noteError(error);
                if (state.disposed || !info || gen !== detailGen || state.detail !== target) return false;
                target[part].status = 'error';
                target[part].error = info;
            }
            emit();
            return true;
        }
        function loadLogs(tail) {
            if (state.detail && LOG_TAILS.includes(tail)) state.detail.logs.tail = tail;
            const lines = state.detail ? state.detail.logs.tail : 200;
            return loadDetailPart('logs', '/logs?tail=' + lines, (d, body) => { d.logs.text = typeof body.logs === 'string' ? body.logs : ''; });
        }
        function loadInspect() {
            return loadDetailPart('inspect', '/inspect', (d, body) => { d.inspect.data = body.inspect === undefined ? null : body.inspect; });
        }

        // ---- host management (these reject with .dm; the dialogs show the message) ---------------------------------

        async function guarded(run) {
            try { return await run(); } catch (error) { noteError(error); emit(); throw error; }
        }
        // The draft is not saved. When it belongs to a stored host its id goes along, so the server reuses the stored secrets.
        async function testDraft(payload, id) {
            const draft = id ? Object.assign({}, payload, { id }) : payload;
            const body = await guarded(() => request('/hosts/test', { method: 'POST', body: draft, timeout: TEST_TIMEOUT_MS }));
            return body.host_status || {};
        }
        async function saveHost(id, payload) {
            const body = await guarded(() => (id
                ? request('/hosts/' + enc(id), { method: 'PUT', body: payload })
                : request('/hosts', { method: 'POST', body: payload })));
            mergeHost(body.host);
            if (body.host) { delete state.byHost[body.host.id]; void loadContainers(body.host.id); }
            emit();
            return body.host;
        }
        async function deleteHost(id) {
            await guarded(() => request('/hosts/' + enc(id), { method: 'DELETE' }));
            state.hosts = state.hosts.filter(host => host.id !== id);
            delete state.byHost[id];
            if (state.scope === id) state.scope = 'all';
            if (state.detail && state.detail.hostId === id) closeDetail();
            emit();
        }
        async function testHost(id) {
            const body = await guarded(() => request('/hosts/' + enc(id) + '/test', { method: 'POST', timeout: TEST_TIMEOUT_MS }));
            mergeHost(body.host);
            emit();
            return body.host;
        }
        async function loadNests() {
            const body = await guarded(() => request('/nests'));
            return Array.isArray(body.nests) ? body.nests : [];
        }
        async function importNest(nestId) {
            const body = await guarded(() => request('/hosts/import-nest/' + enc(nestId), { method: 'POST' }));
            mergeHost(body.host);
            if (body.host) void loadContainers(body.host.id);
            emit();
            return body.host;
        }

        function dispose() {
            if (state.disposed) return;
            state.disposed = true;
            state.visible = false;
            clearTimeout(timer);
            timer = 0;
            lifetime.abort();
            detach();
            listeners.clear();
        }

        return {
            state, subscribe, attach, setVisible, setScope, refresh, loadHosts, loadContainers, reloadContainers, pollOnce, pending,
            act, remove, openDetail, closeDetail, setDetailTab, loadLogs, loadInspect, canMutate, canEditHosts,
            testDraft, saveHost, deleteHost, testHost, loadNests, importNest, dispose
        };
    }

    window.DockerManagerStore = { create, LOG_TAILS, POLL_MS };
})();
```

- [ ] **Step 5: Run; expect PASS.**

```powershell
node --check ui/js/desktop/apps/docker-manager-model.js
node --check ui/js/desktop/apps/docker-manager-store.js
node scripts/test-docker-manager-ui.mjs
```

Expected: `docker-manager ui tests: model and store ok`.

- [ ] **Step 6: Add the npm script** (no central runner exists for these Node tests).

In `package.json`:

Replace

```text
    "test:easydrag": "node scripts/test-easydrag.mjs",
```

with

```text
    "test:easydrag": "node scripts/test-easydrag.mjs",
    "test:docker-manager": "node scripts/test-docker-manager-ui.mjs",
```

```powershell
npm run test:docker-manager
```

Expected: the same `ok` line.

- [ ] **Step 7: Detect changes and commit.**

```powershell
git add ui/js/desktop/apps/docker-manager-model.js ui/js/desktop/apps/docker-manager-store.js scripts/test-docker-manager-ui.mjs package.json
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): add the pure model and the store with their Node tests" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task C5: Views and stylesheet

**Files**

- Create: `ui/js/desktop/apps/docker-manager-views.js`
- Create: `ui/css/desktop-app-docker-manager.css`
- Test: `scripts/test-docker-manager-ui.mjs` (append the stylesheet section)

Rules the markup and the stylesheet keep:

- Every function takes a snapshot and returns a string; user data (names, images, labels, log lines) always goes through `esc()`.
  Colour comes only from `data-tone` and `data-host-color` attributes, never from inline styles.
- The table is a `treegrid` (`role="row"`, `aria-level`, `aria-expanded` on group rows, `aria-selected` on the open row); a status is
  always a dot **and** text; the header is sticky; row buttons carry `aria-label`s ("Stop: web-1"); controls carry
  `data-focus-id` so the root can restore the focus after a repaint.
- A column is squeezed to width 0 instead of `display: none`, because group rows span all five columns (hidden columns would
  otherwise keep phantom widths). The table adapts to its own width (container query on `.dm-table-scroll`), the app to the
  window width (container query on `.dm-app`).
- Host colours are eight `data-host-color` values mapped to theme tokens with `color-mix()`; no hex, no `rgb()`, no theme ids, no
  uppercase labels, monospace only for logs and technical details, and every `transition`/`animation` inside
  `@media (prefers-reduced-motion: no-preference)` on `body:not([data-animations="false"])`.

- [ ] **Step 1: Append the stylesheet guard to the test.** Add this at the end of `scripts/test-docker-manager-ui.mjs`:

```js
// ---- stylesheet rules (same contract as the Infra guard: tokens only, gated motion) --------------------------------------
{
    const css = read('ui/css/desktop-app-docker-manager.css').replace(/\/\*[\s\S]*?\*\//g, '');
    const stripMixes = value => value.replace(/color-mix\((?:[^()]|\([^()]*\))*\)/g, 'color-mix()');
    const NAMED = /(^|[\s,(])(white|black|red|green|blue|gray|grey|silver|orange|yellow|purple|pink|navy|teal|maroon|olive|lime|aqua|fuchsia)(?=$|[\s,)!;])/i;
    const COLOR_PROPS = /^(color|background|background-color|border|border-(top|right|bottom|left)|border(-(top|right|bottom|left))?-color|outline|outline-color|fill|stroke|box-shadow|text-shadow|caret-color|accent-color|text-decoration-color|column-rule)$/;
    const MOTION_PROPS = /^(transition|transition-property|transition-duration|animation|animation-name|animation-duration)$/;
    // leafRules: every rule that holds declarations, with the at-rules around it.
    const leafRules = text => {
        const rules = [], stack = [];
        let buffer = '';
        for (const ch of text) {
            if (ch === '{') { stack.push(buffer.trim()); buffer = ''; }
            else if (ch === '}') { const head = stack.pop() || ''; if (buffer.trim()) rules.push({ selector: head, body: buffer.trim(), context: [...stack] }); buffer = ''; }
            else buffer += ch;
        }
        return rules;
    };
    const splitSelectors = selector => {
        const parts = [];
        let depth = 0, current = '';
        for (const ch of selector) {
            if (ch === '(') depth += 1;
            if (ch === ')') depth -= 1;
            if (ch === ',' && depth === 0) { parts.push(current.trim()); current = ''; } else current += ch;
        }
        if (current.trim()) parts.push(current.trim());
        return parts;
    };
    const outside = stripMixes(css);
    eq(outside.match(/#[0-9a-fA-F]{3,8}\b/g) || [], [], 'the stylesheet uses hex colours outside color-mix()');
    eq(outside.match(/\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(\s*[\d.]/g) || [], [], 'the stylesheet uses literal colour functions instead of tokens');
    assert.ok(!/var\(--vd-font[,)]/.test(css), 'use --vd-font-ui, not --vd-font');
    assert.ok(!/\[data-theme[=\]]/.test(css), 'rules must not key on a theme id; use the --vd-theme-* tokens');
    assert.ok(!/(^|[^a-z-])cc-[a-z]/.test(css), 'the cc- class prefix is reserved by cookie-banner blockers; this app uses dm-');
    const named = [], labels = [], ungated = [];
    for (const rule of leafRules(css).filter(item => !item.context.some(head => head.startsWith('@keyframes')))) {
        for (const [, prop, rawValue] of rule.body.matchAll(/([a-z-]+)\s*:\s*([^;]+)/g)) {
            const name = prop.toLowerCase(), value = rawValue.trim();
            if (name.startsWith('--')) continue;
            if (COLOR_PROPS.test(name) && NAMED.test(stripMixes(value))) named.push(rule.selector + ' { ' + name + ': ' + value + ' }');
            if (name === 'text-transform' && /uppercase/i.test(value)) labels.push(rule.selector + ' { text-transform: uppercase }');
            // Logs, raw JSON and technical details are code; every other label keeps the UI font.
            if ((name === 'font' || name === 'font-family') && /monospace/i.test(value) && !/\.dm-(log|pre)\b/.test(rule.selector)) labels.push(rule.selector + ' { ' + name + ': ' + value + ' }');
            if (MOTION_PROPS.test(name) && !/^(none|0m?s)\b/i.test(value)) {
                const gated = rule.context.some(head => /prefers-reduced-motion\s*:\s*no-preference/.test(head)) && splitSelectors(rule.selector).every(part => part.includes(':not([data-animations="false"])'));
                if (!gated) ungated.push(rule.selector + ' { ' + name + ' }');
            }
        }
    }
    eq(named, [], 'the stylesheet uses named colours instead of tokens');
    eq(labels, [], 'the stylesheet has uppercase labels or monospace outside the log and technical-detail surfaces');
    eq(ungated, [], 'motion must sit inside @media (prefers-reduced-motion: no-preference) with body:not([data-animations="false"])');
}
```

- [ ] **Step 2: Run it; expect FAIL.**

```powershell
node scripts/test-docker-manager-ui.mjs
```

Expected: the model/store `ok` line, then FAIL with `ENOENT ... css\desktop-app-docker-manager.css`.

- [ ] **Step 3: Create the stylesheet.**

```css
/* Docker Manager: rail, toolbar, container table, detail drawer, dialogs and toasts. Tokens only (--vd-*, --ds-*):
   no hex or literal colour functions outside color-mix(), no theme ids, no uppercase labels. Every transition and
   animation sits inside @media (prefers-reduced-motion: no-preference) on selectors that start with
   body:not([data-animations="false"]); scripts/test-docker-manager-ui.mjs guards these rules. */
.dm-app { --dm-rail-w: 244px; --dm-ok: var(--ds-color-success-fg); --dm-warn: var(--ds-color-warning-fg); --dm-bad: var(--ds-color-danger-fg); --dm-info: var(--ds-color-info-fg); --dm-outline: color-mix(in srgb, var(--vd-text) 16%, var(--vd-theme-border)); --dm-ok-text: light-dark(color-mix(in srgb, var(--dm-ok) 30%, var(--vd-text)), var(--dm-ok)); --dm-warn-text: light-dark(color-mix(in srgb, var(--dm-warn) 30%, var(--vd-text)), var(--dm-warn)); --dm-bad-text: light-dark(color-mix(in srgb, var(--dm-bad) 34%, var(--vd-text)), color-mix(in srgb, var(--dm-bad) 88%, var(--vd-text))); --dm-link: light-dark(color-mix(in srgb, var(--vd-accent) 45%, var(--vd-text)), color-mix(in srgb, var(--vd-accent) 78%, var(--vd-text))); position: relative; width: 100%; height: 100%; min-width: 0; min-height: 360px; overflow: hidden; isolation: isolate; color: var(--vd-text); background: var(--vd-theme-app-bg); font: 14px/1.45 var(--vd-font-ui, system-ui, sans-serif); container: dm / inline-size; }
.dm-app * { box-sizing: border-box; }
.dm-app button, .dm-app input, .dm-app select, .dm-app textarea { font: inherit; color: inherit; }
.dm-app [hidden] { display: none !important; }
.dm-app :is(button, input, select, textarea, summary, [tabindex="0"], [tabindex="-1"]):focus-visible { outline: var(--ds-focus-ring-width) solid var(--ds-color-border-focus); outline-offset: var(--ds-focus-ring-offset); }
.dm-app button:disabled { opacity: .5; cursor: not-allowed; }
.dm-sr-only { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; border: 0; }
.dm-glyph { width: 16px; height: 16px; flex: 0 0 auto; fill: none; stroke: currentColor; stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; }
.dm-muted { color: var(--vd-theme-muted); }
.dm-shell { display: grid; grid-template-columns: var(--dm-rail-w) minmax(0, 1fr); width: 100%; height: 100%; min-height: 0; }

/* Host colours: the eight choices map to theme status tokens, so every theme and scheme recolours them. */
.dm-app [data-host-color] { --dm-host: var(--vd-theme-muted); }
.dm-app [data-host-color="blue"] { --dm-host: var(--ds-color-info-fg); }
.dm-app [data-host-color="teal"] { --dm-host: color-mix(in srgb, var(--ds-color-success-fg) 55%, var(--ds-color-info-fg)); }
.dm-app [data-host-color="purple"] { --dm-host: color-mix(in srgb, var(--ds-color-info-fg) 60%, var(--ds-color-danger-fg)); }
.dm-app [data-host-color="coral"] { --dm-host: color-mix(in srgb, var(--ds-color-danger-fg) 78%, var(--ds-color-warning-fg)); }
.dm-app [data-host-color="pink"] { --dm-host: color-mix(in srgb, var(--ds-color-danger-fg) 62%, var(--ds-color-info-fg)); }
.dm-app [data-host-color="amber"] { --dm-host: var(--ds-color-warning-fg); }
.dm-app [data-host-color="green"] { --dm-host: var(--ds-color-success-fg); }
.dm-app [data-host-color="gray"] { --dm-host: var(--vd-theme-muted); }
.dm-swatch { display: inline-block; flex: 0 0 auto; width: 12px; height: 12px; border-radius: 4px; background: var(--dm-host); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--vd-text) 24%, transparent); }
.dm-swatch[data-host-color="all"] { border-radius: 50%; background: conic-gradient(var(--ds-color-info-fg), var(--ds-color-success-fg), var(--ds-color-warning-fg), var(--ds-color-danger-fg), var(--ds-color-info-fg)); }
.dm-chip { display: inline-flex; align-items: center; gap: 5px; max-width: 100%; padding: 1px 8px 1px 6px; border: 1px solid color-mix(in srgb, var(--dm-host) 45%, var(--vd-theme-border)); border-radius: var(--ds-radius-full); background: color-mix(in srgb, var(--dm-host) 14%, var(--vd-theme-panel-bg)); color: var(--vd-text); font-size: 13px; line-height: 1.4; white-space: nowrap; }
.dm-chip-dot { width: 8px; height: 8px; flex: 0 0 auto; border-radius: 50%; background: var(--dm-host); }
.dm-dot { display: inline-block; width: 9px; height: 9px; flex: 0 0 auto; border-radius: 50%; background: var(--vd-theme-muted); }
.dm-dot[data-tone="ok"] { background: var(--dm-ok); }
.dm-dot[data-tone="warn"] { background: var(--dm-warn); }
.dm-dot[data-tone="bad"] { background: var(--dm-bad); }

/* Buttons */
.dm-btn { display: inline-flex; align-items: center; justify-content: center; gap: 6px; min-height: 34px; padding: 0 12px; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-sm); background: var(--vd-theme-control-bg); font-weight: 600; white-space: nowrap; cursor: pointer; }
.dm-btn:hover:not(:disabled) { background: var(--vd-theme-control-hover); }
.dm-btn.is-primary { border-color: transparent; background: var(--vd-accent); color: var(--vd-accent-contrast); }
.dm-btn.is-primary:hover:not(:disabled) { background: color-mix(in srgb, var(--vd-accent) 86%, var(--vd-text)); }
.dm-btn.is-danger { color: var(--dm-bad-text); }
.dm-btn.is-danger-solid { border-color: transparent; background: color-mix(in srgb, var(--dm-bad) 82%, var(--vd-text)); color: var(--vd-theme-app-bg); }
.dm-icon-btn { display: inline-grid; place-items: center; flex: 0 0 auto; width: 32px; height: 32px; padding: 0; border: 1px solid transparent; border-radius: var(--ds-radius-sm); background: transparent; cursor: pointer; }
.dm-icon-btn:hover:not(:disabled) { background: var(--vd-theme-control-hover); }
.dm-icon-btn.is-danger:hover:not(:disabled) { color: var(--dm-bad-text); }
.dm-link-btn { display: flex; align-items: center; gap: var(--ds-space-2); width: 100%; min-height: 38px; padding: 0 var(--ds-space-2); border: 1px solid transparent; border-radius: var(--ds-radius-sm); background: transparent; color: var(--dm-link); font-weight: 600; text-align: left; cursor: pointer; }
.dm-link-btn:hover { background: var(--vd-theme-control-hover); }
.dm-row > .dm-cell-actions { padding-right: var(--ds-space-2); }
.dm-icon-slot { display: inline-block; flex: 0 0 auto; width: 32px; height: 32px; }
@media (pointer: coarse) { .dm-col-actions { width: 212px; } .dm-icon-btn, .dm-icon-slot { width: 44px; height: 44px; } .dm-btn, .dm-filter, .dm-host-btn, .dm-link-btn { min-height: 44px; } }

/* Rail */
.dm-rail { display: flex; flex-direction: column; gap: var(--ds-space-4); min-width: 0; min-height: 0; padding: var(--ds-space-3) var(--ds-space-2); overflow: auto; border-right: 1px solid var(--vd-theme-border); background: var(--vd-theme-panel-bg); }
.dm-rail-section { min-width: 0; }
.dm-rail-title { margin: 0 0 var(--ds-space-1); padding: 0 var(--ds-space-2); color: var(--vd-theme-muted); font-size: 13px; font-weight: 600; }
.dm-hosts { display: grid; gap: 2px; }
.dm-host-btn { display: grid; grid-template-columns: 12px minmax(0, 1fr) auto; align-items: center; gap: var(--ds-space-2); width: 100%; min-height: 46px; padding: 4px var(--ds-space-2); border: 1px solid transparent; border-radius: var(--ds-radius-sm); background: transparent; text-align: left; cursor: pointer; }
.dm-host-btn:hover { background: var(--vd-theme-control-hover); }
.dm-host-btn[aria-selected="true"] { border-color: var(--vd-theme-accent-border); background: var(--vd-theme-accent-soft); }
.dm-host-text { display: grid; min-width: 0; }
.dm-host-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 600; }
.dm-host-sub { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--vd-theme-muted); font-size: 13px; font-variant-numeric: tabular-nums; }
.dm-host-sub[data-tone="bad"] { color: var(--dm-bad-text); }
.dm-host-sub[data-tone="warn"] { color: var(--dm-warn-text); }
.dm-nav-btn { display: flex; align-items: center; gap: var(--ds-space-2); width: 100%; min-height: 38px; padding: 0 var(--ds-space-2); border: 1px solid transparent; border-radius: var(--ds-radius-sm); background: transparent; font-weight: 600; text-align: left; cursor: default; }
.dm-nav-btn[aria-current="page"] { background: var(--vd-theme-control-bg); }
.dm-nav-label { flex: 1; }
.dm-count { padding: 1px 8px; border-radius: var(--ds-radius-full); background: var(--vd-theme-control-bg); color: var(--vd-theme-muted); font-size: 13px; font-variant-numeric: tabular-nums; }
.dm-rail-foot { margin-top: auto; }

/* Main column and toolbar */
.dm-main { position: relative; display: grid; grid-template-rows: auto auto minmax(0, 1fr); min-width: 0; min-height: 0; }
.dm-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2); min-height: 52px; padding: var(--ds-space-2) var(--ds-space-3); border-bottom: 1px solid var(--vd-theme-border); background: var(--vd-theme-panel-bg); }
.dm-search { display: flex; flex: 1 1 220px; align-items: center; gap: var(--ds-space-2); min-width: 160px; max-width: 420px; height: 34px; padding: 0 var(--ds-space-2); border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-md); background: var(--vd-theme-control-bg); color: var(--vd-theme-muted); }
.dm-search:focus-within { border-color: var(--ds-color-border-focus); box-shadow: 0 0 0 1px var(--ds-color-border-focus); }
.dm-app .dm-search-input, .dm-app .dm-search-input:focus-visible { flex: 1; min-width: 0; height: 100%; border: 0; outline: 0; box-shadow: none; background: transparent; color: var(--vd-text); }
.dm-filters { display: inline-flex; flex-wrap: wrap; gap: var(--ds-space-1); }
.dm-filter { display: inline-flex; align-items: center; gap: 6px; height: 32px; padding: 0 10px; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-full); background: var(--vd-theme-control-bg); cursor: pointer; }
.dm-filter:hover { background: var(--vd-theme-control-hover); }
.dm-filter[aria-pressed="true"] { border-color: var(--vd-theme-accent-border); background: var(--vd-theme-accent-soft); font-weight: 600; }
.dm-filter-count { color: var(--vd-theme-muted); font-size: 13px; font-variant-numeric: tabular-nums; }
.dm-result-count { margin-left: auto; color: var(--vd-theme-muted); font-size: 13px; font-variant-numeric: tabular-nums; white-space: nowrap; }
.dm-readonly-chip { padding: 2px 10px; border: 1px solid color-mix(in srgb, var(--dm-warn) 50%, var(--vd-theme-border)); border-radius: var(--ds-radius-full); background: color-mix(in srgb, var(--dm-warn) 10%, var(--vd-theme-panel-bg)); color: var(--dm-warn-text); font-size: 13px; font-weight: 600; }

/* Banners */
.dm-banners:empty { display: none; }
.dm-banner { --dm-banner: var(--dm-warn); display: grid; grid-template-columns: auto minmax(0, 1fr) auto; gap: var(--ds-space-3); margin: var(--ds-space-2) var(--ds-space-3) 0; padding: 10px 12px; border: 1px solid color-mix(in srgb, var(--dm-banner) 50%, var(--vd-theme-border)); border-radius: var(--vd-theme-radius-panel); background: color-mix(in srgb, var(--dm-banner) 9%, var(--vd-theme-panel-bg)); }
.dm-banner[data-tone="bad"] { --dm-banner: var(--dm-bad); }
.dm-banner-icon { padding-top: 2px; color: var(--dm-banner); }
.dm-banner h3 { margin: 0; font-size: 14px; font-weight: 650; }
.dm-banner p { margin: 2px 0 0; color: var(--vd-theme-muted); }
.dm-banner-actions { display: flex; flex-wrap: wrap; align-items: flex-start; gap: var(--ds-space-2); }
.dm-details { margin-top: 6px; }
.dm-details summary { width: fit-content; color: var(--dm-link); cursor: pointer; }
.dm-pre { margin: 6px 0 0; padding: 8px 10px; max-height: 160px; overflow: auto; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-sm); background: var(--vd-theme-app-bg); color: var(--vd-text); font: 12.5px/1.5 var(--ds-font-mono, ui-monospace, SFMono-Regular, Consolas, monospace); white-space: pre-wrap; overflow-wrap: anywhere; }

/* Table */
.dm-content { display: grid; grid-template-columns: minmax(0, 1fr) auto; min-height: 0; }
.dm-table-scroll { position: relative; min-width: 0; min-height: 0; overflow: auto; }
.dm-table { width: 100%; border-collapse: separate; border-spacing: 0; table-layout: fixed; }
.dm-table thead th { position: sticky; top: 0; z-index: 2; height: 36px; padding: 0 var(--ds-space-3); overflow: hidden; border-bottom: 1px solid var(--vd-theme-border); background: var(--vd-theme-panel-bg); color: var(--vd-theme-muted); font-size: 13px; font-weight: 600; text-align: left; text-overflow: ellipsis; white-space: nowrap; }
.dm-col-status { width: 176px; }
.dm-col-image { width: 22%; }
.dm-col-ports { width: 17%; }
.dm-col-actions { width: 160px; }
.dm-row, .dm-group-row { scroll-margin-top: 40px; }
.dm-row > td { padding: 8px var(--ds-space-3); overflow: hidden; border-bottom: 1px solid color-mix(in srgb, var(--vd-theme-border) 70%, transparent); vertical-align: middle; }
.dm-row > td:first-child { box-shadow: inset 3px 0 0 var(--row-tone, transparent); }
.dm-row[data-tone="ok"] { --row-tone: color-mix(in srgb, var(--dm-ok) 70%, transparent); }
.dm-row[data-tone="warn"] { --row-tone: var(--dm-warn); }
.dm-row[data-tone="bad"] { --row-tone: var(--dm-bad); }
.dm-row { cursor: default; }
.dm-row:hover { background: var(--vd-theme-control-hover); }
.dm-row[aria-selected="true"] { background: var(--vd-theme-accent-soft); }
.dm-row[aria-busy="true"] { opacity: .72; }
.dm-row:focus-visible, .dm-group-row:focus-visible { outline: var(--ds-focus-ring-width) solid var(--ds-color-border-focus); outline-offset: -2px; }
.dm-cell-status > * { vertical-align: middle; }
.dm-cell-status .dm-dot { margin-right: var(--ds-space-2); }
.dm-status { display: inline-grid; max-width: calc(100% - 20px); }
.dm-status-text { font-weight: 600; }
.dm-status-sub { overflow: hidden; color: var(--vd-theme-muted); font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.dm-cell-name { min-width: 0; }
.dm-name-line { display: flex; align-items: center; gap: var(--ds-space-2); min-width: 0; }
.dm-name { overflow: hidden; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }
.dm-sub { display: block; overflow: hidden; color: var(--vd-theme-muted); font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.dm-lock { display: inline-grid; place-items: center; color: var(--dm-warn-text); }
.dm-image { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.dm-cell-ports { white-space: nowrap; }
.dm-port { display: inline-block; max-width: 100%; margin-right: 4px; padding: 0 7px; overflow: hidden; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-xs); background: var(--vd-theme-control-bg); font-size: 13px; font-variant-numeric: tabular-nums; vertical-align: middle; text-overflow: ellipsis; }
.dm-port.is-more { color: var(--vd-theme-muted); }
.dm-actions { display: flex; justify-content: flex-end; gap: 2px; }
.dm-group-row { cursor: pointer; }
.dm-group-row > td { padding: 8px var(--ds-space-3); border-bottom: 1px solid var(--vd-theme-border); background: color-mix(in srgb, var(--vd-theme-control-bg) 70%, var(--vd-theme-panel-bg)); }
.dm-group-row:hover > td { background: var(--vd-theme-control-hover); }
.dm-group-main { display: inline-flex; align-items: center; gap: var(--ds-space-2); max-width: 100%; }
.dm-chevron { color: var(--vd-theme-muted); }
.dm-group-row[aria-expanded="true"] .dm-chevron { transform: rotate(90deg); }
.dm-group-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.dm-group-count { color: var(--vd-theme-muted); font-size: 13px; font-variant-numeric: tabular-nums; white-space: nowrap; }

/* Empty and loading states */
.dm-empty { display: grid; place-items: center; min-height: 240px; padding: var(--ds-space-6); }
.dm-empty-inner { display: grid; justify-items: center; gap: var(--ds-space-2); max-width: 420px; text-align: center; }
.dm-empty-inner h3 { margin: 0; font-size: 16px; font-weight: 650; }
.dm-empty-inner p { margin: 0; color: var(--vd-theme-muted); }
.dm-empty-glyph { width: 40px; height: 40px; color: var(--vd-theme-muted); }
.dm-empty-inner .dm-btn { margin-top: var(--ds-space-2); }
.dm-spinner { width: 22px; height: 22px; border: 3px solid var(--vd-theme-border); border-top-color: var(--vd-accent); border-radius: 50%; }

/* Detail drawer */
.dm-drawer { display: flex; flex-direction: column; width: clamp(340px, 38cqw, 520px); min-width: 0; min-height: 0; overflow: hidden; border-left: 1px solid var(--vd-theme-border); background: var(--vd-theme-panel-bg); }
.dm-drawer-head { display: flex; align-items: center; justify-content: space-between; gap: var(--ds-space-2); padding: var(--ds-space-3) var(--ds-space-3) 0; }
.dm-drawer-title { display: flex; align-items: center; gap: var(--ds-space-2); min-width: 0; }
.dm-drawer-title h2 { margin: 0; overflow: hidden; font-size: 17px; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }
.dm-drawer-sub { margin: 2px var(--ds-space-3) 0; color: var(--vd-theme-muted); }
.dm-drawer-actions { display: flex; flex-wrap: wrap; gap: var(--ds-space-2); padding: var(--ds-space-3) var(--ds-space-3) 0; }
.dm-tabs { display: flex; gap: var(--ds-space-1); margin-top: var(--ds-space-3); padding: 0 var(--ds-space-3); border-bottom: 1px solid var(--vd-theme-border); }
.dm-tab { position: relative; min-height: 38px; padding: 0 var(--ds-space-3); border: 0; border-bottom: 2px solid transparent; background: transparent; color: var(--vd-theme-muted); font-weight: 600; cursor: pointer; }
.dm-tab:hover { color: var(--vd-text); }
.dm-tab[aria-selected="true"] { border-bottom-color: var(--vd-accent); color: var(--vd-text); }
.dm-drawer-body { display: flex; flex: 1 1 auto; flex-direction: column; gap: var(--ds-space-3); min-height: 0; padding: var(--ds-space-3); overflow: auto; }
.dm-drawer-body[data-tab-body="logs"], .dm-drawer-body[data-tab-body="inspect"] { overflow: hidden; }
.dm-panel-tools { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2); }
.dm-inline { display: inline-flex; align-items: center; gap: var(--ds-space-2); color: var(--vd-theme-muted); }
.dm-inline select { height: 34px; padding: 0 var(--ds-space-2); border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-sm); background: var(--vd-theme-app-bg); color: var(--vd-text); }
.dm-log { flex: 1 1 auto; min-height: 160px; margin: 0; padding: 10px 12px; overflow: auto; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-sm); background: var(--vd-theme-app-bg); color: var(--vd-text); font: 12.5px/1.5 var(--ds-font-mono, ui-monospace, SFMono-Regular, Consolas, monospace); white-space: pre-wrap; overflow-wrap: anywhere; }
.dm-log.is-loading { opacity: .6; }
.dm-error { margin: 0; color: var(--dm-bad-text); }
.dm-note { display: flex; gap: var(--ds-space-2); align-items: flex-start; margin: 0; padding: 8px 10px; border: 1px solid color-mix(in srgb, var(--dm-warn) 45%, var(--vd-theme-border)); border-radius: var(--ds-radius-sm); background: color-mix(in srgb, var(--dm-warn) 8%, var(--vd-theme-panel-bg)); }
.dm-note .dm-glyph { margin-top: 2px; color: var(--dm-warn-text); }
.dm-facts { display: grid; gap: var(--ds-space-2); margin: 0; }
.dm-fact { display: grid; grid-template-columns: 120px minmax(0, 1fr); gap: var(--ds-space-3); }
.dm-fact dt { color: var(--vd-theme-muted); }
.dm-fact dd { margin: 0; overflow-wrap: anywhere; }
.dm-fact-list { margin: 0; padding: 0; list-style: none; }
.dm-fact-state { font-weight: 600; margin: 0 var(--ds-space-1); }

/* Dialog */
.dm-dialog { position: fixed; width: min(560px, calc(100vw - 28px)); max-height: min(88vh, 780px); padding: 0; overflow: auto; border: 1px solid var(--vd-theme-border); border-radius: var(--vd-theme-radius-panel); background: var(--vd-theme-panel-bg-strong); color: var(--vd-text); box-shadow: var(--vd-theme-shadow-modal); }
.dm-dialog.is-wide { width: min(740px, calc(100vw - 28px)); }
.dm-dialog::backdrop { background: var(--vd-theme-overlay-bg); }
.dm-dialog-close { position: absolute; top: 12px; right: 12px; }
.dm-dialog-title { margin: 0; padding: 18px 56px 4px 20px; font-size: 18px; font-weight: 650; }
.dm-view { padding: 8px 20px 18px; }
.dm-dialog-copy { margin: 0 0 var(--ds-space-3); color: var(--vd-theme-muted); }
.dm-dialog-actions { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2); margin-top: var(--ds-space-4); }
.dm-spacer { flex: 1; }
.dm-hostlist { display: grid; gap: var(--ds-space-2); margin: 0; padding: 0; list-style: none; }
.dm-hostrow { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: var(--ds-space-3); padding: 10px 12px; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-md); background: var(--vd-theme-panel-bg); }
.dm-hostrow-main { min-width: 0; }
.dm-hostrow-name { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2); }
.dm-hostrow-sub { overflow: hidden; color: var(--vd-theme-muted); text-overflow: ellipsis; white-space: nowrap; }
.dm-hostrow-status { display: flex; align-items: center; gap: var(--ds-space-2); margin-top: 2px; }
.dm-hostrow-actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: var(--ds-space-1); }
.dm-badge { padding: 0 8px; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-full); background: var(--vd-theme-control-bg); color: var(--vd-theme-muted); font-size: 13px; font-weight: 500; }
.dm-badge[data-tone="warn"] { border-color: color-mix(in srgb, var(--dm-warn) 50%, var(--vd-theme-border)); color: var(--dm-warn-text); }
.dm-form { display: block; }
.dm-field { display: grid; gap: 4px; margin-top: var(--ds-space-3); min-width: 0; }
.dm-field > label { font-weight: 600; }
.dm-field input, .dm-field select, .dm-field textarea { width: 100%; min-height: 36px; padding: 6px 10px; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-sm); background: var(--vd-theme-app-bg); color: var(--vd-text); }
.dm-field textarea { resize: vertical; }
.dm-field [aria-invalid="true"] { border-color: var(--dm-bad); }
.dm-field-row { display: grid; grid-template-columns: minmax(0, 1fr) 120px; gap: var(--ds-space-3); }
.dm-hint { color: var(--vd-theme-muted); font-size: 13px; }
.dm-warn { display: flex; gap: var(--ds-space-2); align-items: flex-start; margin-top: var(--ds-space-3); padding: 8px 10px; border: 1px solid color-mix(in srgb, var(--dm-warn) 55%, var(--vd-theme-border)); border-radius: var(--ds-radius-sm); background: color-mix(in srgb, var(--dm-warn) 10%, var(--vd-theme-panel-bg)); }
.dm-warn .dm-glyph { margin-top: 2px; color: var(--dm-warn-text); }
.dm-colors { margin: var(--ds-space-3) 0 0; padding: 0; border: 0; }
.dm-colors legend { margin-bottom: 6px; padding: 0; font-weight: 600; }
.dm-color-row { display: flex; flex-wrap: wrap; gap: var(--ds-space-2); }
.dm-swatch-opt { position: relative; display: inline-flex; cursor: pointer; }
.dm-swatch.is-pick { width: 26px; height: 26px; border-radius: 50%; }
.dm-swatch-opt input:checked + .is-pick { box-shadow: 0 0 0 2px var(--vd-theme-panel-bg-strong), 0 0 0 4px var(--vd-text); }
.dm-swatch-opt input:focus-visible + .is-pick { outline: var(--ds-focus-ring-width) solid var(--ds-color-border-focus); outline-offset: 4px; }
.dm-stored { display: flex; align-items: center; gap: var(--ds-space-2); margin-top: var(--ds-space-3); padding: 8px 10px; border: 1px solid var(--vd-theme-border); border-radius: var(--ds-radius-sm); background: var(--vd-theme-panel-bg); }
.dm-stored .dm-btn { min-height: 30px; margin-left: auto; }
.dm-working { width: 14px; height: 14px; vertical-align: -2px; }
.dm-advanced { margin-top: var(--ds-space-3); }
.dm-advanced summary { width: fit-content; color: var(--dm-link); cursor: pointer; }
.dm-check { display: flex; align-items: center; gap: var(--ds-space-2); margin-top: var(--ds-space-3); }
.dm-test-row { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-3); margin-top: var(--ds-space-4); }
.dm-test-result { min-height: 24px; margin-top: var(--ds-space-2); }
.dm-result { display: inline-flex; align-items: center; gap: var(--ds-space-2); font-weight: 600; }
.dm-result.is-ok { color: var(--dm-ok-text); }
.dm-result.is-bad { color: var(--dm-bad-text); }
.dm-form-error { margin: var(--ds-space-3) 0 0; color: var(--dm-bad-text); font-weight: 600; }

/* Toasts */
.dm-toasts { position: absolute; right: var(--ds-space-3); bottom: var(--ds-space-3); z-index: 40; display: grid; gap: var(--ds-space-2); max-width: min(420px, calc(100% - 24px)); pointer-events: none; }
.dm-dialog > .dm-toasts { position: sticky; right: auto; bottom: 0; max-width: none; padding: 0 20px 14px; }
.dm-toast { padding: 10px 12px; border: 1px solid var(--vd-theme-border); border-left: 4px solid var(--dm-info); border-radius: var(--ds-radius-sm); background: var(--vd-theme-panel-bg-strong); box-shadow: var(--vd-theme-shadow-panel); pointer-events: auto; overflow-wrap: anywhere; }
.dm-toast[data-tone="success"] { border-left-color: var(--dm-ok); }
.dm-toast[data-tone="error"] { border-left-color: var(--dm-bad); }

/* The table adapts to its own width (the drawer takes part of it): Image goes first, then Ports, then the status sub-line.
   A column is squeezed to zero width instead of display: none, because the group rows span all five columns. */
.dm-table-scroll { container: dmt / inline-size; }
@container dmt (max-width: 900px) {
    .dm-table .dm-col-image, .dm-table .dm-cell-image { width: 0; padding: 0; border-width: 0; overflow: hidden; visibility: hidden; }
    .dm-col-ports { width: 22%; }
}
@container dmt (max-width: 700px) {
    .dm-table .dm-col-ports, .dm-table .dm-cell-ports { width: 0; padding: 0; border-width: 0; overflow: hidden; visibility: hidden; }
}
@container dmt (max-width: 560px) {
    .dm-col-status { width: 132px; }
    .dm-status-sub { display: none; }
}

/* Narrow windows: the drawer overlays the table and the rail becomes a strip of hosts. */
@container dm (max-width: 820px) {
    .dm-shell { grid-template-columns: minmax(0, 1fr); grid-template-rows: auto minmax(0, 1fr); }
    .dm-rail { flex-direction: row; align-items: stretch; gap: var(--ds-space-2); padding: var(--ds-space-2); overflow: auto hidden; border-right: 0; border-bottom: 1px solid var(--vd-theme-border); }
    .dm-rail-section { display: flex; flex: 1 1 auto; min-width: 0; overflow: hidden; }
    .dm-rail-title, .dm-nav { display: none; }
    .dm-hosts { display: flex; gap: var(--ds-space-1); min-width: 0; overflow-x: auto; }
    .dm-host-btn { flex: 0 0 auto; width: auto; min-width: 140px; }
    .dm-rail-foot { margin-top: 0; flex: 0 0 auto; }
    .dm-link-btn { width: auto; white-space: nowrap; }
    .dm-drawer { position: absolute; inset: 0 0 0 auto; z-index: 20; width: min(100%, 460px); box-shadow: var(--vd-theme-shadow-modal); }
    .dm-fact { grid-template-columns: 96px minmax(0, 1fr); }
}
@container dm (max-width: 560px) {
    .dm-hostrow { grid-template-columns: auto minmax(0, 1fr); }
    .dm-hostrow-actions { grid-column: 1 / -1; justify-content: flex-start; }
    .dm-result-count { display: none; }
}

/* Motion: only when the window animations switch is on and the system does not ask for less. */
@media (prefers-reduced-motion: no-preference) {
    body:not([data-animations="false"]) .dm-row, body:not([data-animations="false"]) .dm-host-btn, body:not([data-animations="false"]) .dm-btn, body:not([data-animations="false"]) .dm-icon-btn, body:not([data-animations="false"]) .dm-filter { transition: background-color .12s ease, border-color .12s ease, color .12s ease; }
    body:not([data-animations="false"]) .dm-chevron { transition: transform .14s ease; }
    body:not([data-animations="false"]) .dm-spinner { animation: dm-spin .8s linear infinite; }
    body:not([data-animations="false"]) .dm-icon-btn.is-spinning .dm-glyph, body:not([data-animations="false"]) .dm-working { animation: dm-spin .8s linear infinite; }
    body:not([data-animations="false"]) .dm-toast { animation: dm-toast-in .16s ease-out; }
    @keyframes dm-spin { to { transform: rotate(360deg); } }
    @keyframes dm-toast-in { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: none; } }
}
```

- [ ] **Step 4: Create the views module.**

```js
/* Docker Manager views: HTML builders for the rail, the banners, the container table, the empty states and the detail
   drawer. A classic script (window.DockerManagerViews) with no DOM work at load. Every function takes a snapshot and
   returns a string; docker-manager.js inserts it, keeps the focus and routes the events through data-action hooks.
   User data (names, images, labels, log lines) always goes through esc(). Colour comes from data-tone and
   data-host-color attributes, never from inline styles. */
(function () {
    'use strict';

    const GLYPHS = {
        play: '<path d="M8 5l11 7-11 7z"/>',
        stop: '<rect x="6" y="6" width="12" height="12" rx="2"/>',
        restart: '<path d="M20 12a8 8 0 1 1-2.4-5.7"/><path d="M20 4v6h-6"/>',
        pause: '<path d="M9 6v12M15 6v12"/>',
        kill: '<path d="M13 3L5 14h6l-1 7 8-11h-6z"/>',
        remove: '<path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"/>',
        logs: '<path d="M6 3h9l4 4v14H6z"/><path d="M14 3v5h5M9 13h7M9 17h7"/>',
        inspect: '<circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7.5v.01"/>',
        more: '<circle cx="5" cy="12" r="1.5" fill="currentColor" stroke="none"/><circle cx="12" cy="12" r="1.5" fill="currentColor" stroke="none"/><circle cx="19" cy="12" r="1.5" fill="currentColor" stroke="none"/>',
        lock: '<rect x="5" y="11" width="14" height="9" rx="2"/><path d="M8 11V8a4 4 0 0 1 8 0v3"/>',
        plus: '<path d="M12 5v14M5 12h14"/>',
        chevron: '<path d="M9 6l6 6-6 6"/>',
        refresh: '<path d="M20 12a8 8 0 1 1-2.4-5.7"/><path d="M20 4v6h-6"/>',
        close: '<path d="M6 6l12 12M18 6L6 18"/>',
        copy: '<rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V6a2 2 0 0 1 2-2h9"/>',
        edit: '<path d="M4 20l1-4L16 5l3 3L8 19z"/>',
        server: '<rect x="3" y="4" width="18" height="6" rx="2"/><rect x="3" y="14" width="18" height="6" rx="2"/><path d="M7 7h.01M7 17h.01"/>',
        containers: '<rect x="6" y="3" width="14" height="5" rx="1"/><rect x="3" y="9.5" width="14" height="5" rx="1"/><rect x="7" y="16" width="14" height="5" rx="1"/>',
        search: '<circle cx="11" cy="11" r="6.5"/><path d="M16 16l4 4"/>',
        warning: '<path d="M12 3l10 18H2z"/><path d="M12 10v5M12 18v.01"/>',
        check: '<path d="M5 12l5 5 9-10"/>',
        download: '<path d="M12 3v12M7 10l5 5 5-5M4 20h16"/>'
    };
    const ACTION_GLYPH = { start: 'play', unpause: 'play', stop: 'stop', restart: 'restart', pause: 'pause', kill: 'kill', remove: 'remove' };
    const ERROR_KEYS = ['read_only', 'container_not_found', 'invalid_host', 'network', 'timeout', 'forbidden'];
    const DETAIL_ERROR_CODES = ['invalid_host'];
    const PORTS_SHOWN = 2;
    const TABS = ['overview', 'logs', 'inspect'];
    // Terminal colour codes in container logs would show as stray characters.
    const ANSI = /\x1b\[[0-9;?]*[ -/]*[@-~]/g;

    function create(deps) {
        const { t, esc, model, lang, uid } = deps;
        const glyph = (name, cls) => '<svg class="dm-glyph' + (cls ? ' ' + cls : '') + '" viewBox="0 0 24 24" aria-hidden="true" focusable="false">' + (GLYPHS[name] || '') + '</svg>';
        const dot = tone => '<span class="dm-dot" data-tone="' + esc(tone) + '" aria-hidden="true"></span>';
        const chip = (name, color) => '<span class="dm-chip" data-host-color="' + esc(color) + '"><span class="dm-chip-dot" aria-hidden="true"></span>' + esc(name) + '</span>';
        const keyOf = row => row.key;

        function statusLabel(status) {
            return status.id === 'exited' ? t('status_exited_code', { code: status.code }) : t('status_' + status.id);
        }
        function actionLabel(action) { return action === 'remove' ? t('action_remove') : t('action_' + action); }
        function reasonText(reason) { return t('reason_' + model.reasonCode(reason)); }
        function protectedText(container) {
            const reason = model.protectedReason(container);
            if (!reason) return '';
            return reason.kind === 'owner' ? t('protected_owner', { owner: reason.owner }) : t('protected_' + reason.kind);
        }
        // A user-facing sentence for a failure. Codes without their own text show the generic sentence plus the server's message,
        // because that message names the cause (an engine error, a validation problem).
        function errorText(info) {
            const e = info || {};
            if (e.code === 'host_unreachable') return reasonText(e.reason);
            const known = ERROR_KEYS.includes(e.code), base = known ? t('error_' + e.code) : t('error_unknown');
            return (!known || DETAIL_ERROR_CODES.includes(e.code)) && e.message ? base + ' ' + e.message : base;
        }
        function stripAnsi(value) { return String(value || '').replace(ANSI, ''); }

        // ---- rail --------------------------------------------------------------------------------------------------

        function hostLine(host, entry, health) {
            if (health.state === 'error') return '<span class="dm-host-sub" data-tone="' + health.tone + '">' + esc(t('host_error')) + '</span>';
            if (!entry || !entry.loaded) return '<span class="dm-host-sub">' + esc(health.state === 'ok' ? t('loading') : t('host_unknown')) + '</span>';
            const counts = model.hostCounts(host.id, { [host.id]: entry });
            return '<span class="dm-host-sub">' + esc(t('count_running_total', counts)) + '</span>';
        }

        function rail(snap) {
            const s = snap.state, scope = s.scope, totals = model.totalCounts(snap.hosts, s.byHost);
            const option = (id, color, name, titleText, line, tone, word) => {
                const selected = scope === id;
                return '<button type="button" role="option" class="dm-host-btn" data-action="pick-host" data-host-id="' + esc(id) + '" data-focus-id="host:' + esc(id) + '" aria-selected="' + selected
                    + '" tabindex="' + (selected ? '0' : '-1') + '" title="' + esc(titleText) + '"><span class="dm-swatch" data-host-color="' + esc(color) + '" aria-hidden="true"></span>'
                    + '<span class="dm-host-text"><span class="dm-host-name">' + esc(name) + '</span>' + line + '</span>' + (tone ? dot(tone) + '<span class="dm-sr-only">' + esc(word) + '</span>' : '') + '</button>';
            };
            const allLine = '<span class="dm-host-sub">' + esc(t('count_running_total', totals)) + '</span>';
            const items = [option('all', 'all', t('all_hosts'), t('all_hosts'), allLine, '', '')];
            for (const host of snap.hosts) {
                const entry = s.byHost[host.id], health = model.hostHealth(host, entry);
                items.push(option(host.id, model.colorOf(host.color), host.name, host.name + ' · ' + model.hostAddress(host), hostLine(host, entry, health), health.tone, t('host_' + health.state)));
            }
            return '<div class="dm-rail-section"><h2 class="dm-rail-title" id="' + uid + '-hosts">' + esc(t('hosts_heading')) + '</h2>'
                + '<div class="dm-hosts" role="listbox" aria-labelledby="' + uid + '-hosts" data-host-list>' + items.join('') + '</div></div>'
                + '<nav class="dm-nav" aria-label="' + esc(t('nav_label')) + '"><button type="button" class="dm-nav-btn" aria-current="page">' + glyph('containers')
                + '<span class="dm-nav-label">' + esc(t('nav_containers')) + '</span><span class="dm-count">' + totals.total + '</span></button></nav>'
                + '<div class="dm-rail-foot"><button type="button" class="dm-link-btn" data-action="manage-hosts" data-focus-id="manage">' + glyph('server') + '<span>' + esc(t('manage_hosts')) + '</span></button></div>';
        }

        // ---- banners (one per unreachable host in scope) ----------------------------------------------------------------

        function banners(snap) {
            const out = [];
            for (const host of snap.scopeHosts) {
                const entry = snap.state.byHost[host.id], health = model.hostHealth(host, entry);
                if (health.state !== 'error') continue;
                const stale = entry && entry.containers && entry.containers.length > 0;
                const id = uid + '-banner-' + host.id;
                out.push('<section class="dm-banner" data-tone="' + health.tone + '" aria-labelledby="' + esc(id) + '">'
                    + '<span class="dm-banner-icon">' + glyph('warning') + '</span><div class="dm-banner-body"><h3 id="' + esc(id) + '">' + esc(t('banner_title', { host: host.name })) + '</h3>'
                    + '<p>' + esc(reasonText(health.reason)) + (stale ? ' ' + esc(t('stale_notice')) : '') + '</p>'
                    + (health.message ? '<details class="dm-details"><summary>' + esc(t('banner_details')) + '</summary><pre class="dm-pre">' + esc(health.message) + '</pre></details>' : '') + '</div>'
                    + '<div class="dm-banner-actions"><button type="button" class="dm-btn" data-action="retry-host" data-host-id="' + esc(host.id) + '" data-focus-id="retry:' + esc(host.id) + '">' + glyph('refresh') + '<span>' + esc(t('banner_retry')) + '</span></button>'
                    + (snap.canEditHosts && !host.builtin ? '<button type="button" class="dm-btn" data-action="edit-host" data-host-id="' + esc(host.id) + '" data-focus-id="edit:' + esc(host.id) + '">' + glyph('edit') + '<span>' + esc(t('host_edit_title')) + '</span></button>' : '')
                    + '</div></section>');
            }
            return out.join('');
        }

        // ---- table -------------------------------------------------------------------------------------------------------

        function iconButton(action, extra, label, glyphName, o) {
            const key = o.key;
            return '<button type="button" class="dm-icon-btn' + (o.danger ? ' is-danger' : '') + '" data-action="' + action + '"' + extra + ' data-row-key="' + esc(key) + '" data-row-btn data-focus-id="' + o.focus
                + '" aria-label="' + esc(t('row_action_label', { action: label, name: o.name })) + '" title="' + esc(label) + '" tabindex="-1"' + (o.busy ? ' disabled aria-busy="true"' : '') + '>' + glyph(glyphName) + '</button>';
        }

        function portsCell(container) {
            const items = model.portItems(container.ports);
            if (!items.length) return '<span class="dm-muted" aria-hidden="true">—</span><span class="dm-sr-only">' + esc(t('ov_none')) + '</span>';
            const shown = items.slice(0, PORTS_SHOWN).map(item => '<span class="dm-port">' + esc(item.label) + '</span>').join('');
            const rest = items.slice(PORTS_SHOWN);
            return shown + (rest.length ? '<span class="dm-port is-more" title="' + esc(rest.map(item => item.label).join(', ')) + '">+' + rest.length + '</span>' : '');
        }

        function row(snap, r) {
            const c = r.c, status = model.statusOf(c), name = model.containerName(c), id = model.containerId(c);
            const actions = model.actionsFor(c, !snap.canMutate), busy = snap.state.busy.has(r.hostId + '/' + id);
            const open = !!(snap.state.detail && snap.state.detail.key === r.key);
            const sub = [c.compose_service, c.id].filter(Boolean).join(' · ');
            const lock = model.isProtected(c) ? '<span class="dm-lock" role="img" aria-label="' + esc(protectedText(c)) + '" title="' + esc(protectedText(c)) + '">' + glyph('lock') + '</span>' : '';
            // Empty slots keep the four buttons of every row in the same columns.
            const slot = '<span class="dm-icon-slot" aria-hidden="true"></span>', buttons = [];
            buttons.push(actions.toggle
                ? iconButton('act', ' data-act="' + actions.toggle + '"', actionLabel(actions.toggle), ACTION_GLYPH[actions.toggle], { key: r.key, name, busy, focus: 'act:' + esc(r.key) + ':toggle' })
                : slot);
            buttons.push(!actions.restart ? slot : iconButton('act', ' data-act="restart"', actionLabel('restart'), 'restart', { key: r.key, name, busy, focus: 'act:' + esc(r.key) + ':restart' }));
            buttons.push(iconButton('logs', '', t('action_logs'), 'logs', { key: r.key, name, busy: false, focus: 'act:' + esc(r.key) + ':logs' }));
            buttons.push(iconButton('more', '', t('action_more'), 'more', { key: r.key, name, busy: false, focus: 'act:' + esc(r.key) + ':more' }));
            return '<tr class="dm-row" role="row" data-nav-key="' + esc(r.key) + '" data-row-key="' + esc(r.key) + '" data-focus-id="row:' + esc(r.key) + '" data-tone="' + status.tone + '" aria-level="'
                + (snap.headers ? 2 : 1) + '" aria-selected="' + open + '" aria-busy="' + busy + '" tabindex="-1">'
                + '<td role="gridcell" class="dm-cell-status">' + dot(status.tone) + '<span class="dm-status"><span class="dm-status-text">' + esc(statusLabel(status)) + '</span><span class="dm-status-sub">' + esc(c.status) + '</span></span></td>'
                + '<td role="gridcell" class="dm-cell-name"><span class="dm-name-line"><span class="dm-name">' + esc(name) + '</span>' + lock + (snap.multi ? chip(r.hostName, r.hostColor) : '') + '</span><span class="dm-sub">' + esc(sub) + '</span></td>'
                + '<td role="gridcell" class="dm-cell-image"><span class="dm-image" title="' + esc(c.image) + '">' + esc(model.shortImage(c.image)) + '</span></td>'
                + '<td role="gridcell" class="dm-cell-ports">' + portsCell(c) + '</td>'
                + '<td role="gridcell" class="dm-cell-actions"><div class="dm-actions" role="group" aria-label="' + esc(t('row_actions_label', { name })) + '">' + buttons.join('') + '</div></td></tr>';
        }

        function groupRow(snap, group, collapsed) {
            const name = group.kind === 'project' ? group.project : t('group_single');
            const counts = group.counts;
            const text = t('count_running_total', { running: counts.running, total: counts.total }) + (counts.problem ? ' · ' + t('group_problems', { count: counts.problem }) : '');
            return '<tr class="dm-group-row" role="row" data-nav-key="g:' + esc(group.key) + '" data-group-key="' + esc(group.key) + '" data-focus-id="group:' + esc(group.key) + '" data-action="toggle-group" aria-level="1" aria-expanded="' + !collapsed + '" tabindex="-1">'
                + '<td role="gridcell" colspan="5"><span class="dm-group-main">' + glyph('chevron', 'dm-chevron') + (group.kind === 'project' ? dot(group.tone) : '') + '<strong class="dm-group-name">' + esc(name) + '</strong>'
                + (snap.multi && group.kind === 'project' ? chip(group.hostName, group.hostColor) : '') + '<span class="dm-group-count">' + esc(text) + '</span></span></td></tr>';
        }

        function tbody(snap) {
            const parts = [];
            for (const group of snap.groups) {
                const collapsed = snap.headers && snap.collapsed.has(group.key);
                if (snap.headers) parts.push(groupRow(snap, group, collapsed));
                if (!collapsed) for (const r of group.rows) parts.push(row(snap, r));
            }
            return parts.join('');
        }

        // ---- empty states ------------------------------------------------------------------------------------------------

        function panel(glyphName, title, body, button) {
            return '<div class="dm-empty-inner">' + glyph(glyphName, 'dm-empty-glyph') + '<h3>' + esc(title) + '</h3><p>' + esc(body) + '</p>' + (button || '') + '</div>';
        }
        const actionButton = (action, label, primary) => '<button type="button" class="dm-btn' + (primary ? ' is-primary' : '') + '" data-action="' + action + '" data-focus-id="' + action + '">' + esc(label) + '</button>';

        // '' when the table has rows. Unreachable hosts are explained by their banners, so an all-unreachable scope shows no extra message.
        function empty(snap) {
            const s = snap.state;
            if (s.disabled) return panel('server', t('disabled_title'), t('disabled_body'), actionButton('open-settings', t('open_settings'), true));
            if (!s.hostsLoaded) {
                if (s.hostsFailed) return panel('warning', t('load_error_title'), errorText(s.hostsFailed), actionButton('refresh', t('banner_retry'), true));
                return '<div class="dm-empty-inner is-loading" role="status"><span class="dm-spinner" aria-hidden="true"></span><p>' + esc(t('loading')) + '</p></div>';
            }
            if (snap.visible.length) return '';
            if (snap.rows.length === 0) {
                if (snap.scopeHosts.some(host => !s.byHost[host.id])) return '<div class="dm-empty-inner is-loading" role="status"><span class="dm-spinner" aria-hidden="true"></span><p>' + esc(t('loading')) + '</p></div>';
                if (snap.scopeHosts.every(host => !(s.byHost[host.id] && s.byHost[host.id].loaded))) return '';
                return panel('containers', t('empty_title'), snap.multi ? t('empty_all_body') : t('empty_body'));
            }
            return panel('search', t('no_match_title'), t('no_match_body'), actionButton('clear-filters', t('clear_filters'), false));
        }

        // ---- detail drawer -----------------------------------------------------------------------------------------------

        function fact(label, html) { return '<div class="dm-fact"><dt>' + esc(label) + '</dt><dd>' + html + '</dd></div>'; }
        const none = () => '<span class="dm-muted">' + esc(t('ov_none')) + '</span>';
        const list = items => (items.length ? '<ul class="dm-fact-list">' + items.map(item => '<li>' + esc(item) + '</li>').join('') + '</ul>' : none());

        function overview(r) {
            const c = r.c, status = model.statusOf(c), note = protectedText(c);
            const facts = [
                fact(t('ov_state'), dot(status.tone) + '<span class="dm-fact-state">' + esc(statusLabel(status)) + '</span> <span class="dm-muted">' + esc(c.status) + '</span>'),
                fact(t('ov_host'), chip(r.hostName, r.hostColor)),
                fact(t('col_image'), esc(c.image)),
                fact(t('ov_id'), esc(c.id)),
                fact(t('ov_created'), esc(model.formatCreated(c.created, lang)))
            ];
            if (c.compose_project) facts.push(fact(t('ov_project'), esc(c.compose_project)));
            if (c.compose_service) facts.push(fact(t('ov_service'), esc(c.compose_service)));
            if (c.compose_working_dir) facts.push(fact(t('ov_workdir'), esc(c.compose_working_dir)));
            facts.push(fact(t('col_ports'), list(model.portItems(c.ports).map(item => item.label))));
            facts.push(fact(t('ov_networks'), list(Array.isArray(c.networks) ? c.networks : [])));
            facts.push(fact(t('ov_mounts'), list((Array.isArray(c.mounts) ? c.mounts : []).map(model.mountLabel))));
            return (note ? '<p class="dm-note">' + glyph('lock') + '<span>' + esc(note) + '</span></p>' : '') + '<dl class="dm-facts">' + facts.join('') + '</dl>';
        }

        function copyButton(action) {
            return '<button type="button" class="dm-btn" data-action="' + action + '" data-focus-id="' + action + '">' + glyph('copy') + '<span>' + esc(t('copy')) + '</span></button>';
        }

        function logsPanel(detail) {
            const logs = detail.logs;
            const tails = deps.logTails.map(n => '<option value="' + n + '"' + (n === logs.tail ? ' selected' : '') + '>' + n + '</option>').join('');
            const tools = '<div class="dm-panel-tools"><label class="dm-inline"><span>' + esc(t('logs_tail')) + '</span><select data-log-tail data-focus-id="log-tail">' + tails + '</select></label>'
                + '<button type="button" class="dm-btn" data-action="logs-reload" data-focus-id="logs-reload">' + glyph('refresh') + '<span>' + esc(t('logs_reload')) + '</span></button>' + copyButton('copy-logs') + '</div>';
            let body;
            if (logs.status === 'error') body = '<p class="dm-error" role="alert">' + esc(t('logs_failed')) + ' ' + esc(errorText(logs.error)) + '</p>';
            else if (logs.status === 'ok' || (logs.status === 'loading' && logs.text)) {
                body = logs.text
                    ? '<pre class="dm-log' + (logs.status === 'loading' ? ' is-loading' : '') + '" tabindex="0" data-log aria-label="' + esc(t('tab_logs')) + '">' + esc(stripAnsi(logs.text)) + '</pre>'
                    : '<p class="dm-muted">' + esc(t('logs_empty')) + '</p>';
            } else body = '<p class="dm-muted" role="status">' + esc(t('logs_loading')) + '</p>';
            return tools + body;
        }

        function inspectPanel(detail) {
            const inspect = detail.inspect;
            let body;
            if (inspect.status === 'error') body = '<p class="dm-error" role="alert">' + esc(t('inspect_failed')) + ' ' + esc(errorText(inspect.error)) + '</p>';
            else if (inspect.status === 'ok') body = '<pre class="dm-log dm-code" tabindex="0" data-inspect aria-label="' + esc(t('tab_inspect')) + '">' + esc(JSON.stringify(inspect.data, null, 2)) + '</pre>';
            else body = '<p class="dm-muted" role="status">' + esc(t('inspect_loading')) + '</p>';
            return '<div class="dm-panel-tools">' + copyButton('copy-inspect') + '</div>' + body;
        }

        function drawerActions(snap, r) {
            const actions = model.actionsFor(r.c, !snap.canMutate), name = model.containerName(r.c);
            if (!actions.toggle && !actions.restart && !actions.menu.length && !actions.remove) return '';
            const busy = snap.state.busy.has(r.hostId + '/' + model.containerId(r.c));
            const button = (action, act, label, glyphName, danger) => '<button type="button" class="dm-btn' + (danger ? ' is-danger' : '') + '" data-action="' + action + '"' + (act ? ' data-act="' + act + '"' : '')
                + ' data-row-key="' + esc(r.key) + '" data-focus-id="d:' + (act || action) + '"' + (busy ? ' disabled aria-busy="true"' : '') + ' aria-label="' + esc(t('row_action_label', { action: label, name })) + '">' + glyph(glyphName) + '<span>' + esc(label) + '</span></button>';
            const out = [];
            if (actions.toggle) out.push(button('act', actions.toggle, actionLabel(actions.toggle), ACTION_GLYPH[actions.toggle], false));
            if (actions.restart) out.push(button('act', 'restart', actionLabel('restart'), 'restart', false));
            for (const action of actions.menu) out.push(button('act', action, actionLabel(action), ACTION_GLYPH[action], action === 'kill'));
            if (actions.remove) out.push(button('remove', '', actionLabel('remove'), 'remove', true));
            return '<div class="dm-drawer-actions">' + out.join('') + '</div>';
        }

        function drawer(snap, r) {
            const detail = snap.state.detail, c = r.c, status = model.statusOf(c), name = model.containerName(c);
            const tabs = TABS.map(tab => '<button type="button" role="tab" class="dm-tab" id="' + uid + '-tab-' + tab + '" data-action="tab" data-tab="' + tab + '" data-focus-id="tab:' + tab + '" aria-selected="' + (detail.tab === tab)
                + '" aria-controls="' + uid + '-tabpanel" tabindex="' + (detail.tab === tab ? '0' : '-1') + '">' + esc(t('tab_' + tab)) + '</button>').join('');
            let body;
            if (detail.tab === 'logs') body = logsPanel(detail);
            else if (detail.tab === 'inspect') body = inspectPanel(detail);
            else body = overview(r);
            return '<header class="dm-drawer-head"><div class="dm-drawer-title">' + dot(status.tone) + '<h2 id="' + uid + '-drawer-title">' + esc(name) + '</h2></div>'
                + '<button type="button" class="dm-icon-btn" data-action="close-detail" data-focus-id="close-detail" aria-label="' + esc(t('drawer_close')) + '" title="' + esc(t('drawer_close')) + '">' + glyph('close') + '</button></header>'
                + '<p class="dm-drawer-sub">' + esc(statusLabel(status)) + ' · ' + esc(model.shortImage(c.image)) + '</p>'
                + drawerActions(snap, r)
                + '<div class="dm-tabs" role="tablist" aria-label="' + esc(t('tabs_label')) + '">' + tabs + '</div>'
                + '<div class="dm-drawer-body" role="tabpanel" id="' + uid + '-tabpanel" aria-labelledby="' + uid + '-tab-' + detail.tab + '" data-tab-body="' + detail.tab + '">' + body + '</div>';
        }

        return { glyph, dot, chip, statusLabel, actionLabel, reasonText, protectedText, errorText, stripAnsi, rail, banners, tbody, empty, drawer, keyOf, TABS };
    }

    window.DockerManagerViews = { create, TABS, ACTION_GLYPH };
})();
```

- [ ] **Step 5: Run; expect PASS.**

```powershell
node --check ui/js/desktop/apps/docker-manager-views.js
node scripts/test-docker-manager-ui.mjs
$LASTEXITCODE
```

Expected: `docker-manager ui tests: model and store ok` and exit code `0` (the stylesheet section prints nothing when it passes).
`go test ./ui -run "TestUIAvoidsCookieConsentClassPrefix|TestShippedWebAssetsLoadNothingExternal" -count=1` also stays `ok`.

- [ ] **Step 6: Detect changes and commit.**

```powershell
git add ui/js/desktop/apps/docker-manager-views.js ui/css/desktop-app-docker-manager.css scripts/test-docker-manager-ui.mjs
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): add the table, rail, drawer views and the stylesheet" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task C6: Dialogs

**Files**

- Create: `ui/js/desktop/apps/docker-manager-dialogs.js`

This module has no pure logic of its own (the form rules and the payload are tested in Task C4). Its behaviour is covered by the
load contract added in Task C7 and by the browser smoke in Task C8; it is verified here by a syntax check, and the interactive
flows (stored credentials with Replace, failing and passing Test connection, protected remove with the server's message, nests)
were exercised in headless Chrome against a fake API when this plan was written.

What it owns: one native `<dialog>` provided by the root (the top layer, so modal focus handling is the browser's); a view stack
(`push`/`pop`) so Escape and Back go one level up and the close button or a click on the backdrop closes everything; listeners
are attached to the per-paint view element so they never pile up; the host manager list (Test, Edit, Delete per host, Add host,
Import from nest); the add/edit form (kind-dependent fields, colour swatches as a radio group, "Credentials stored" + Replace,
inline validation with `aria-invalid`, Test connection with its own busy state); confirmations (Cancel has the focus).

- [ ] **Step 1: Create the module.**

```js
/* Docker Manager dialogs: one native <dialog> (provided by docker-manager.js) that shows a stack of views: the host
   manager list, the add/edit form with "Test connection", the import-from-nest list and the confirmations (remove a
   container, act on a protected one, delete a host). Escape or Back pops one view; the close button and a click on the
   backdrop close all. A classic script (window.DockerManagerDialogs) with no DOM work at load. Secrets typed into the
   form live only in the form fields: they are sent with Save or Test and never rendered back. */
(function () {
    'use strict';

    const PROBLEM_INPUT = { name: 'name', address: 'address', port: 'port', sshUser: 'ssh_user', sshSecret: 'ssh_password', tlsCert: 'tls_cert', stacksDir: 'stacks_dir', socketPath: 'host_socket_path' };
    const SERVER_FIELD_INPUT = { name: 'name', address: 'address', port: 'port', ssh_user: 'ssh_user', ssh_secret: 'ssh_password', stacks_dir: 'stacks_dir', host_socket_path: 'host_socket_path', kind: 'kind' };
    const RUNNING_STATES = ['running', 'paused', 'restarting'];

    function create(app) {
        const { t, dt, esc, model, store, root, views, toast, announce, uid } = app;
        const dialog = root.querySelector('[data-dialog]'), content = root.querySelector('[data-dialog-content]');
        const stack = [];
        const testing = new Set();
        let opener = null, disposed = false;
        const fieldId = name => uid + '-f-' + name;

        // ---- the view stack --------------------------------------------------------------------------------------

        const top = () => stack[stack.length - 1];

        function paint() {
            const view = top();
            if (!view) return;
            content.innerHTML = '<h2 class="dm-dialog-title" id="' + uid + '-dialog-title">' + esc(view.title) + '</h2><div class="dm-view" data-view>' + view.html() + '</div>';
            dialog.classList.toggle('is-wide', !!view.wide);
            // Listeners go on the per-paint view element, which the next paint replaces together with them.
            const body = content.querySelector('[data-view]');
            view.bind?.(body);
            const first = body.querySelector('[data-autofocus]') || body.querySelector('input:not([type="hidden"]):not([type="radio"]), select, textarea, button');
            first?.focus({ preventScroll: true });
        }
        function push(view) {
            if (disposed) return null;
            stack.push(view);
            if (!dialog.open) { opener = document.activeElement; dialog.showModal(); }
            paint();
            return view;
        }
        function pop(result) {
            const view = stack.pop();
            view?.onClose?.(result);
            if (stack.length) paint(); else if (dialog.open) dialog.close();
        }
        function closeAll() {
            while (stack.length) stack.pop().onClose?.(false);
            if (dialog.open) dialog.close();
        }
        // Repaints a part of the top view in place and keeps the focus on the same control.
        function refreshPart(view, selector, html) {
            if (top() !== view) return;
            const part = content.querySelector(selector);
            if (!part) return;
            const focusId = document.activeElement?.dataset?.focusId;
            part.innerHTML = html;
            if (focusId) content.querySelector('[data-focus-id="' + CSS.escape(focusId) + '"]')?.focus({ preventScroll: true });
        }

        dialog.addEventListener('cancel', event => { event.preventDefault(); if (stack.length) pop(false); });
        dialog.addEventListener('close', () => {
            while (stack.length) stack.pop().onClose?.(false);
            const target = opener;
            opener = null;
            app.restoreFocus(target);
        });
        dialog.addEventListener('click', event => {
            if (event.target === dialog || event.target.closest('[data-dialog-dismiss]')) closeAll();
        });

        const actions = (left, right) => '<div class="dm-dialog-actions">' + (left || '') + '<span class="dm-spacer"></span>' + (right || '') + '</div>';
        const button = (act, label, o) => '<button type="button" class="dm-btn' + (o && o.primary ? ' is-primary' : '') + (o && o.danger ? ' is-danger-solid' : '') + '" data-dact="' + act + '"'
            + (o && o.id ? ' data-host-id="' + esc(o.id) + '"' : '') + (o && o.focusId ? ' data-focus-id="' + esc(o.focusId) + '"' : '') + (o && o.auto ? ' data-autofocus' : '') + (o && o.disabled ? ' disabled aria-busy="true"' : '') + '>'
            + (o && o.glyph ? views.glyph(o.glyph) : '') + '<span>' + esc(label) + '</span></button>';

        // ---- confirmations ---------------------------------------------------------------------------------------

        // Resolves true on the confirm button, false on Cancel, Escape, the close button or the backdrop. Cancel has the focus.
        function confirm(o) {
            return new Promise(resolve => {
                let settled = false;
                const settle = value => { if (!settled) { settled = true; resolve(value); } };
                push({
                    title: o.title,
                    html: () => '<p class="dm-dialog-copy">' + esc(o.message) + '</p>'
                        + (o.note ? '<p class="dm-note">' + views.glyph('lock') + '<span>' + esc(o.note) + '</span></p>' : '')
                        + actions('', button('no', dt('desktop.cancel'), { auto: true }) + button('yes', o.confirmLabel, { primary: !o.danger, danger: !!o.danger })),
                    bind(c) {
                        c.querySelector('[data-dact="no"]').addEventListener('click', () => { settle(false); pop(false); });
                        c.querySelector('[data-dact="yes"]').addEventListener('click', () => { settle(true); pop(true); });
                    },
                    onClose() { settle(false); }
                });
            });
        }

        function confirmRemove(row) {
            const c = row.c, name = model.containerName(c);
            const running = RUNNING_STATES.includes(String(c.state).toLowerCase());
            return confirm({
                title: t('remove_title'), message: running ? t('remove_body_running', { name, host: row.hostName }) : t('remove_body', { name, host: row.hostName }),
                note: views.protectedText(c), confirmLabel: t('action_remove'), danger: true
            });
        }
        // `info` is the server's 409 container_protected_confirmation_required answer when there is one: its message and owner
        // say what is protected; without it the client's own knowledge of the container explains the lock.
        function confirmProtectedAction(row, action, info) {
            const name = model.containerName(row.c), label = views.actionLabel(action), answer = info || {};
            const note = answer.message || (answer.owner ? t('protected_owner', { owner: answer.owner }) : '') || views.protectedText(row.c);
            return confirm({
                title: t('protected_confirm_title'), message: t('protected_confirm_body', { name, action: label }),
                note, confirmLabel: label, danger: true
            });
        }

        // ---- host manager --------------------------------------------------------------------------------------------

        function badge(text, tone) { return '<span class="dm-badge"' + (tone ? ' data-tone="' + tone + '"' : '') + '>' + esc(text) + '</span>'; }

        function hostRow(host, canEdit) {
            const entry = store.state.byHost[host.id], health = model.hostHealth(host, entry);
            const status = host.status || {};
            const word = health.state === 'error' ? t('host_error') + ': ' + views.reasonText(health.reason) : t('host_' + health.state);
            const meta = [t('kind_' + host.kind), model.hostAddress(host)];
            if (status.engine_version) meta.push(t('host_engine', { version: status.engine_version }));
            const badges = (host.builtin ? badge(t('host_builtin')) : '') + (host.plaintext ? badge(t('host_plain_badge'), 'warn') : '') + (host.secret_set ? badge(t('host_secret_set')) : '');
            const busy = testing.has(host.id);
            return '<li class="dm-hostrow" data-host-id="' + esc(host.id) + '"><span class="dm-swatch" data-host-color="' + esc(model.colorOf(host.color)) + '" aria-hidden="true"></span>'
                + '<div class="dm-hostrow-main"><div class="dm-hostrow-name"><strong>' + esc(host.name) + '</strong>' + badges + '</div>'
                + '<div class="dm-hostrow-sub">' + esc(meta.join(' · ')) + '</div>'
                + '<div class="dm-hostrow-status">' + views.dot(health.tone) + '<span' + (health.message ? ' title="' + esc(health.message) + '"' : '') + '>' + esc(word) + '</span></div></div>'
                + '<div class="dm-hostrow-actions">' + button('test', busy ? t('f_testing') : t('host_test'), { id: host.id, focusId: 'test:' + host.id, disabled: busy })
                + (canEdit && !host.builtin ? button('edit', t('host_edit'), { id: host.id, focusId: 'edit:' + host.id }) + button('delete', t('host_delete'), { id: host.id, focusId: 'delete:' + host.id }) : '') + '</div></li>';
        }

        function openHosts() {
            let unsubscribe = null;
            const canEdit = () => store.canEditHosts();
            const listHtml = () => store.state.hosts.map(host => hostRow(host, canEdit())).join('');
            const view = {
                title: t('rail_label'), wide: true,
                html: () => '<p class="dm-dialog-copy">' + esc(t('hosts_intro')) + '</p><ul class="dm-hostlist" data-hostlist>' + listHtml() + '</ul>'
                    + actions(canEdit() ? button('nests', t('host_import'), { glyph: 'download' }) : '', (canEdit() ? button('add', t('host_add'), { primary: true, glyph: 'plus', auto: true }) : '') + button('close', dt('desktop.close'))),
                update() { refreshPart(view, '[data-hostlist]', listHtml()); },
                bind(c) {
                    unsubscribe?.();
                    unsubscribe = store.subscribe(() => view.update());
                    c.addEventListener('click', event => {
                        const el = event.target.closest('[data-dact]');
                        if (!el) return;
                        const id = el.dataset.hostId, host = store.state.hosts.find(item => item.id === id);
                        switch (el.dataset.dact) {
                            case 'close': closeAll(); break;
                            case 'add': openHostForm(null); break;
                            case 'nests': openNests(); break;
                            case 'edit': if (host) openHostForm(host); break;
                            case 'test': if (host) void testRow(host, view); break;
                            case 'delete': if (host) void deleteRow(host); break;
                            default: break;
                        }
                    });
                },
                onClose() { unsubscribe?.(); unsubscribe = null; }
            };
            return push(view);
        }

        async function testRow(host, view) {
            testing.add(host.id);
            view.update();
            try {
                const updated = await store.testHost(host.id);
                void store.reloadContainers(host.id);
                announce(updated.status && updated.status.state === 'ok' ? t('f_test_ok', { version: updated.status.engine_version || '', api: updated.status.api_version || '', os: updated.status.os || '', arch: updated.status.arch || '' }) : t('host_error') + ': ' + host.name);
            } catch (error) {
                toast(views.errorText(error.dm), 'error');
            } finally {
                testing.delete(host.id);
                view.update();
            }
        }

        async function deleteRow(host) {
            const ok = await confirm({ title: t('host_delete_title'), message: t('host_delete_body', { name: host.name }), confirmLabel: t('host_delete'), danger: true });
            if (!ok || disposed) return;
            try {
                await store.deleteHost(host.id);
                toast(t('host_deleted', { name: host.name }), 'success');
                announce(t('host_deleted', { name: host.name }));
            } catch (error) {
                toast(views.errorText(error.dm), 'error');
            }
        }

        // ---- add / edit form ----------------------------------------------------------------------------------------

        function formHtml(form) {
            const editing = !!form.id, stored = model.storedSecretUsable(form);
            const input = (name, value, attrs, hint) => '<input id="' + fieldId(name) + '" name="' + name + '" value="' + esc(value) + '"' + (/autocomplete=/.test(attrs || '') ? '' : ' autocomplete="off"') + ' spellcheck="false" autocapitalize="off"' + (hint ? ' aria-describedby="' + fieldId(name) + '-hint"' : '') + ' ' + (attrs || '') + '>';
            const area = (name, placeholder, hint) => '<textarea id="' + fieldId(name) + '" name="' + name + '" rows="3" autocomplete="off" spellcheck="false" autocapitalize="off" placeholder="' + esc(placeholder || '') + '"' + (hint ? ' aria-describedby="' + fieldId(name) + '-hint"' : '') + '></textarea>';
            const field = (name, label, control, hint, cls, forKinds) => '<div class="dm-field' + (cls ? ' ' + cls : '') + '"' + (forKinds ? ' data-for="' + forKinds + '"' : '') + '><label for="' + fieldId(name) + '">' + esc(label) + '</label>' + control
                + (hint ? '<small class="dm-hint" id="' + fieldId(name) + '-hint">' + esc(hint) + '</small>' : '') + '</div>';
            const kinds = model.HOST_KINDS.map(kind => '<option value="' + kind + '"' + (kind === form.kind ? ' selected' : '') + '>' + esc(t('kind_' + kind)) + '</option>').join('');
            const colors = model.HOST_COLORS.map(color => '<label class="dm-swatch-opt"><input type="radio" name="color" value="' + color + '"' + (color === form.color ? ' checked' : '') + ' class="dm-sr-only"><span class="dm-swatch is-pick" data-host-color="' + color + '"></span><span class="dm-sr-only">' + esc(t('color_' + color)) + '</span></label>').join('');
            const keepHint = stored ? t('f_secret_stored') : '';
            return '<form class="dm-form" data-host-form novalidate>'
                + field('name', t('col_name'), input('name', form.name, 'maxlength="64" required data-autofocus'))
                + '<div class="dm-field-row">' + field('kind', t('f_kind'), '<select id="' + fieldId('kind') + '" name="kind">' + kinds + '</select>') + field('port', t('f_port'), input('port', form.port, 'inputmode="numeric" maxlength="5" data-port'), '', 'is-narrow') + '</div>'
                + field('address', t('f_address'), input('address', form.address, 'required', true), t('f_address_hint'))
                + '<div class="dm-warn" role="note" data-for="tcp">' + views.glyph('warning') + '<span>' + esc(t('warn_plain_tcp')) + '</span></div>'
                + field('ssh_user', t('f_ssh_user'), input('ssh_user', form.sshUser, ''), '', '', 'ssh')
                // A host with stored credentials shows "stored" and a Replace action; the secret fields appear only after Replace.
                + '<div class="dm-stored" data-for="ssh tls mtls" data-stored-row hidden>' + views.dot('ok') + '<span>' + esc(t('host_secret_set')) + '</span><button type="button" class="dm-btn" data-dact="replace" data-focus-id="replace-secret"><span>' + esc(t('f_replace')) + '</span></button></div>'
                + '<div class="dm-group" data-for="ssh" data-secret>'
                + field('ssh_password', t('f_ssh_password'), input('ssh_password', '', 'type="password" autocomplete="new-password"', !!keepHint), keepHint)
                + field('ssh_key', t('f_ssh_key'), area('ssh_key', '')) + '</div>'
                + '<div class="dm-group" data-for="tls mtls" data-secret>' + field('tls_ca', t('f_tls_ca'), area('tls_ca', '', true), t('f_tls_hint'))
                + '<div data-for="mtls">' + field('tls_cert', t('f_tls_cert'), area('tls_cert', '')) + field('tls_key', t('f_tls_key'), area('tls_key', '')) + '</div>'
                + (stored ? '<small class="dm-hint">' + esc(t('f_secret_stored')) + '</small>' : '') + '</div>'
                + '<fieldset class="dm-colors"><legend>' + esc(t('f_color')) + '</legend><div class="dm-color-row">' + colors + '</div></fieldset>'
                + '<details class="dm-advanced"' + (form.stacksDir !== model.DEFAULT_STACKS_DIR || form.socketPath !== model.DEFAULT_SOCKET ? ' open' : '') + '><summary>' + esc(t('f_advanced')) + '</summary><p class="dm-hint">' + esc(t('f_advanced_hint')) + '</p>'
                + field('stacks_dir', t('f_stacks_dir'), input('stacks_dir', form.stacksDir, '')) + field('host_socket_path', t('f_socket'), input('host_socket_path', form.socketPath, '')) + '</details>'
                + (editing && form.secretSet ? '<label class="dm-check" data-for="ssh tls mtls"><input type="checkbox" name="clear_secret"' + (form.clearSecret ? ' checked' : '') + '><span>' + esc(t('f_clear_secret')) + '</span></label>' : '')
                + '<div class="dm-test-row"><button type="button" class="dm-btn" data-dact="test-form" data-focus-id="test-form">' + views.glyph('check') + '<span>' + esc(t('f_test')) + '</span></button><span class="dm-hint">' + esc(t('f_test_hint')) + '</span></div>'
                + '<div class="dm-test-result" role="status" aria-live="polite" data-test-result></div>'
                + '<p class="dm-form-error" role="alert" data-form-error hidden></p>'
                + actions(button('back', dt('desktop.back')), '<button type="submit" class="dm-btn is-primary" data-focus-id="save-form"><span>' + esc(dt('desktop.save')) + '</span></button>') + '</form>';
        }

        function readForm(c, base) {
            const f = c.querySelector('[data-host-form]'), v = name => (f.elements[name] ? f.elements[name].value : '');
            const key = v('ssh_key').trim();
            const checked = f.querySelector('input[name="color"]:checked');
            return Object.assign({}, base, {
                name: v('name'), kind: v('kind'), address: v('address'), port: v('port'), sshUser: v('ssh_user'), sshSecret: key || v('ssh_password'),
                tlsCA: v('tls_ca'), tlsCert: v('tls_cert'), tlsKey: v('tls_key'), stacksDir: v('stacks_dir'), socketPath: v('host_socket_path'),
                color: checked ? checked.value : base.color, clearSecret: !!(f.elements.clear_secret && f.elements.clear_secret.checked)
            });
        }

        function showProblem(c, inputName, message) {
            for (const el of c.querySelectorAll('[aria-invalid]')) el.removeAttribute('aria-invalid');
            const error = c.querySelector('[data-form-error]');
            error.textContent = message;
            error.hidden = !message;
            const el = inputName ? c.querySelector('[name="' + inputName + '"]') : null;
            if (el) { el.setAttribute('aria-invalid', 'true'); el.focus({ preventScroll: false }); }
        }

        // Shows what the chosen connection needs. ui.replacing is true once the admin pressed Replace on stored credentials.
        function syncKind(c, base, ui) {
            const f = c.querySelector('[data-host-form]'), kind = f.elements.kind.value;
            for (const el of f.querySelectorAll('[data-for]')) el.hidden = !el.dataset.for.split(' ').includes(kind);
            f.elements.port.placeholder = String(model.defaultPort(kind));
            const form = readForm(c, base), stored = model.storedSecretUsable(form), tucked = stored && !ui.replacing;
            for (const el of f.querySelectorAll('[data-secret]')) el.hidden = el.hidden || tucked;
            for (const el of f.querySelectorAll('[data-stored-row]')) el.hidden = el.hidden || !tucked;
        }

        // The secret inputs are tucked away behind Replace while credentials are stored; a problem on one of them opens them.
        function revealSecrets(c, base, ui, inputName) {
            if (!ui.replacing && ['ssh_password', 'ssh_key', 'tls_ca', 'tls_cert', 'tls_key'].includes(inputName)) { ui.replacing = true; syncKind(c, base, ui); }
        }

        function resultHtml(status) {
            const s = model.normalizeStatus(status);
            if (s.state === 'ok') {
                const text = t('f_test_ok', { version: s.engine_version || '?', api: s.api_version || '?', os: s.os || '?', arch: s.arch || '?' });
                return '<span class="dm-result is-ok">' + views.dot('ok') + '<span>' + esc(text) + '</span></span>';
            }
            return '<span class="dm-result is-bad">' + views.dot('bad') + '<span>' + esc(t('f_test_failed', { reason: views.reasonText(s.code) })) + '</span></span>'
                + (s.message ? '<details class="dm-details"><summary>' + esc(t('banner_details')) + '</summary><pre class="dm-pre">' + esc(s.message) + '</pre></details>' : '');
        }

        // A test of an unreachable host can take 30 s. Only the Test button is busy; the rest of the form stays usable.
        async function testForm(c, base, ui) {
            const form = readForm(c, base), out = c.querySelector('[data-test-result]'), btn = c.querySelector('[data-dact="test-form"]');
            const problem = model.validateHostForm(form, store.state.hosts);
            if (problem) { revealSecrets(c, base, ui, PROBLEM_INPUT[problem.field]); showProblem(c, PROBLEM_INPUT[problem.field], t('v_' + problem.code)); return; }
            showProblem(c, '', '');
            btn.disabled = true;
            btn.setAttribute('aria-busy', 'true');
            out.innerHTML = '<span class="dm-muted">' + views.glyph('refresh', 'dm-working') + ' ' + esc(t('f_testing')) + '</span>';
            try {
                const status = await store.testDraft(model.hostPayload(form), form.id);
                if (out.isConnected) out.innerHTML = resultHtml(status);
            } catch (error) {
                if (out.isConnected) out.innerHTML = '<span class="dm-result is-bad">' + views.dot('bad') + '<span>' + esc(views.errorText(error.dm)) + '</span></span>';
            } finally {
                btn.disabled = false;
                btn.removeAttribute('aria-busy');
            }
        }

        // The server's field names for a rejected host; ssh_secret and tls are errors on the secret inputs.
        function serverInput(info, kind) {
            if (info.field === 'tls') return kind === 'mtls' ? 'tls_cert' : 'tls_ca';
            return SERVER_FIELD_INPUT[info.field] || '';
        }

        async function saveForm(c, base, ui) {
            const form = readForm(c, base), submit = c.querySelector('[type="submit"]');
            const problem = model.validateHostForm(form, store.state.hosts);
            if (problem) { revealSecrets(c, base, ui, PROBLEM_INPUT[problem.field]); showProblem(c, PROBLEM_INPUT[problem.field], t('v_' + problem.code)); return; }
            showProblem(c, '', '');
            submit.disabled = true;
            submit.setAttribute('aria-busy', 'true');
            try {
                const saved = await store.saveHost(form.id, model.hostPayload(form));
                pop(true);
                toast(t('host_saved', { name: saved.name }), 'success');
                announce(t('host_saved', { name: saved.name }));
            } catch (error) {
                submit.disabled = false;
                submit.removeAttribute('aria-busy');
                const info = error.dm || {};
                const input = info.code === 'invalid_host' ? serverInput(info, form.kind) : '';
                revealSecrets(c, base, ui, input);
                showProblem(c, input, views.errorText(info));
            }
        }

        function openHostForm(host) {
            if (!store.canEditHosts()) { toast(t('readonly_chip'), 'error'); return null; }
            const base = host ? model.hostToForm(host) : model.emptyHostForm(store.state.hosts);
            return push({
                title: host ? t('host_edit_title') : t('host_add'),
                html: () => formHtml(base),
                bind(c) {
                    const f = c.querySelector('[data-host-form]'), ui = { replacing: false };
                    syncKind(c, base, ui);
                    f.addEventListener('input', () => syncKind(c, base, ui));
                    f.addEventListener('change', () => syncKind(c, base, ui));
                    f.addEventListener('submit', event => { event.preventDefault(); void saveForm(c, base, ui); });
                    f.querySelector('[data-dact="back"]').addEventListener('click', () => pop(false));
                    f.querySelector('[data-dact="test-form"]').addEventListener('click', () => void testForm(c, base, ui));
                    f.querySelector('[data-dact="replace"]').addEventListener('click', () => {
                        ui.replacing = true;
                        syncKind(c, base, ui);
                        const first = f.querySelector('[data-secret]:not([hidden]) input, [data-secret]:not([hidden]) textarea');
                        if (first) first.focus();
                    });
                }
            });
        }

        // ---- import from nest -----------------------------------------------------------------------------------------

        function openNests() {
            const data = { status: 'loading', nests: [], error: null, importing: '' };
            const rowHtml = nest => {
                const method = model.NEST_METHODS.includes(nest.deploy_method) ? t('nest_method_' + nest.deploy_method) : String(nest.deploy_method || '');
                const meta = [method, nest.host ? nest.host + (nest.port ? ':' + nest.port : '') : '', nest.docker_tls ? t('kind_' + (nest.docker_tls === 'mtls' ? 'mtls' : 'tls')) : ''].filter(Boolean);
                const busy = data.importing === nest.id;
                return '<li class="dm-hostrow"><div class="dm-hostrow-main"><div class="dm-hostrow-name"><strong>' + esc(nest.name) + '</strong></div><div class="dm-hostrow-sub">' + esc(meta.join(' · ')) + '</div></div>'
                    + '<div class="dm-hostrow-actions">' + button('import', busy ? t('nest_importing') : t('nest_import'), { id: nest.id, focusId: 'import:' + nest.id, disabled: !!data.importing }) + '</div></li>';
            };
            const bodyHtml = () => {
                if (data.status === 'loading') return '<p class="dm-muted" role="status">' + esc(t('loading')) + '</p>';
                if (data.status === 'error') return '<p class="dm-error" role="alert">' + esc(views.errorText(data.error)) + '</p>';
                if (!data.nests.length) return '<p class="dm-muted">' + esc(t('nest_empty')) + '</p>';
                return '<ul class="dm-hostlist">' + data.nests.map(rowHtml).join('') + '</ul>';
            };
            const view = {
                title: t('nest_title'), wide: true,
                html: () => '<p class="dm-dialog-copy">' + esc(t('nest_intro')) + '</p><div data-nestlist>' + bodyHtml() + '</div>' + actions(button('back', dt('desktop.back'), { auto: true }), ''),
                update() { refreshPart(view, '[data-nestlist]', bodyHtml()); },
                bind(c) {
                    c.addEventListener('click', event => {
                        const el = event.target.closest('[data-dact]');
                        if (!el) return;
                        if (el.dataset.dact === 'back') pop(false);
                        if (el.dataset.dact === 'import' && !data.importing) void importNest(el.dataset.hostId);
                    });
                }
            };
            async function importNest(id) {
                data.importing = id;
                view.update();
                try {
                    const host = await store.importNest(id);
                    pop(true);
                    toast(t('nest_imported', { name: host.name }), 'success');
                    announce(t('nest_imported', { name: host.name }));
                } catch (error) {
                    data.importing = '';
                    data.status = 'error';
                    data.error = error.dm;
                    view.update();
                }
            }
            push(view);
            store.loadNests().then(nests => { data.status = 'ok'; data.nests = nests; view.update(); },
                error => { data.status = 'error'; data.error = error.dm; view.update(); });
            return view;
        }

        function dispose() {
            if (disposed) return;
            closeAll();
            disposed = true;
        }

        return { openHosts, openHostForm, openNests, confirm, confirmRemove, confirmProtectedAction, closeAll, dispose, isOpen: () => dialog.open };
    }

    window.DockerManagerDialogs = { create };
})();
```

- [ ] **Step 2: Syntax check.**

```powershell
node --check ui/js/desktop/apps/docker-manager-dialogs.js
```

Expected: no output.

- [ ] **Step 3: Detect changes and commit.**

```powershell
git add ui/js/desktop/apps/docker-manager-dialogs.js
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): add the host manager, host form and confirmation dialogs" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```


### Task C7: Composition root and the remaining Node guards

**Files**

- Create: `ui/js/desktop/apps/docker-manager.js`
- Test: `scripts/test-docker-manager-ui.mjs` (append the load contract and the translation guard)

What the root owns: the skeleton (`.dm-app` container > rail, toolbar, banners, treegrid, drawer, `<dialog>`, toasts, one polite
live region); the store subscription with one repaint per animation frame; keeping the focus on the same control when a part is
repainted (`data-focus-id`, with the row as fallback for a vanished action button); the roving tab stop of the table; the keyboard
(arrows, Home/End, Left/Right on groups, Enter, Delete, Escape, `/`, Shift+F10); the row context menu through `ctx.showContextMenu`;
the window menus; toasts (moved into the `<dialog>` while it is open, because a modal dialog sits in the top layer); the action
flows (protected confirmations, the two-request protected remove); persistence of scope, filter and collapsed groups; disposal
(abort requests, timers, menus, dialog).

- [ ] **Step 1: Append the load contract and the translation guard to the test.**

At the end of `scripts/test-docker-manager-ui.mjs` add the load contract (all five modules, each below 1100 lines, none touches
the DOM at load, each publishes its global):

```js
// ---- load contract: all five modules, each under 1100 lines, none touches the DOM at load, each publishes its global ----------
const uiFiles = ['docker-manager-views.js', 'docker-manager-dialogs.js', 'docker-manager.js'];
const full = {};
{
    const all = [...files, ...uiFiles];
    for (const file of all) {
        const lines = read('ui/js/desktop/apps/' + file).split('\n').length;
        assert.ok(lines < 1100, `${file} has ${lines} lines, want fewer than 1100`);
    }
    vm.runInNewContext(all.map(file => read('ui/js/desktop/apps/' + file)).join('\n'), {
        window: full, console, setTimeout, clearTimeout, AbortController, Intl, Map, Set, JSON, Math, Date, URL, performance
    });
    assert.ok(full.DockerManagerModel && full.DockerManagerStore && full.DockerManagerViews && full.DockerManagerDialogs && full.DockerManagerApp, 'all five modules publish their global');
    assert.equal(typeof full.DockerManagerApp.render, 'function');
    assert.equal(typeof full.DockerManagerApp.dispose, 'function');
    assert.equal(typeof full.DockerManagerViews.create, 'function');
    assert.equal(typeof full.DockerManagerDialogs.create, 'function');
}
```

then the translation guard (every key the code can ask for exists in English, in all 16 locales with the same placeholders, and
no key is unused):

```js
// ---- translations: every key the code can ask for exists in all sixteen locales with the same placeholders -------------------
{
    const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
    const locales = Object.fromEntries(LANGS.map(lang => [lang, JSON.parse(read('ui/lang/desktop/' + lang + '.json'))]));
    const en = locales.en;
    const prefix = 'docker_manager.';
    const used = new Set();
    for (const file of uiFiles) {
        const source = read('ui/js/desktop/apps/' + file);
        for (const match of source.matchAll(/(?<![\w.])t\(\s*'([a-zA-Z0-9_]+)'/g)) used.add(prefix + match[1]);
        for (const match of source.matchAll(/\bdt\(\s*'(desktop\.[a-z_]+)'/g)) used.add(match[1]);
    }
    // Keys the code builds from a prefix, taken from the lists that drive them. The prefix-only matches above are removed.
    for (const key of [...used]) if (key.endsWith('_')) used.delete(key);
    const families = {
        status_: ['running', 'paused', 'restarting', 'exited_code', 'created', 'dead', 'removing', 'unhealthy', 'starting', 'unknown'],
        action_: [...model.ACTIONS, 'remove', 'logs', 'inspect', 'more', 'open_details', 'done', 'failed'],
        reason_: [...model.REASONS],
        error_: ['read_only', 'container_not_found', 'invalid_host', 'network', 'timeout', 'forbidden', 'unknown'],
        filter_: [...model.STATE_FILTERS],
        kind_: ['local', ...model.HOST_KINDS],
        color_: [...model.HOST_COLORS],
        col_: ['status', 'name', 'image', 'ports', 'actions'],
        host_: ['ok', 'error', 'unknown'],
        protected_: ['owner', 'self', 'endpoint', 'network'],
        tab_: [...full.DockerManagerViews.TABS],
        nest_method_: [...model.NEST_METHODS],
        v_: ['name', 'name_taken', 'name_reserved', 'address', 'address_scheme', 'port', 'ssh_user', 'ssh_secret', 'tls_cert', 'path']
    };
    for (const [family, names] of Object.entries(families)) for (const name of names) used.add(prefix + family + name);
    used.add('desktop.app_docker_manager');
    assert.ok(used.size > 150, 'the key scan found only ' + used.size + ' keys');
    for (const key of used) assert.ok(typeof en[key] === 'string' && en[key].trim() !== '', 'en.json is missing ' + key);
    const tokens = value => [...String(value).matchAll(/\{\{(\w+)\}\}/g)].map(match => match[1]).sort().join(',');
    const own = Object.keys(en).filter(key => key.startsWith(prefix));
    for (const key of own) assert.ok(used.has(key), 'en.json has the unused key ' + key);
    for (const lang of LANGS) {
        const ownKeys = Object.keys(locales[lang]).filter(key => key.startsWith(prefix));
        eq(ownKeys.sort(), [...own].sort(), lang + '.json has a different docker_manager key set');
        for (const key of [...own, 'desktop.app_docker_manager']) {
            assert.ok(String(locales[lang][key] || '').trim() !== '', lang + '.json has an empty ' + key);
            assert.equal(tokens(locales[lang][key]), tokens(en[key]), lang + '.json changes the placeholders of ' + key);
        }
    }
}

console.log('docker-manager ui tests: stylesheet, load contract and translations ok');
```

- [ ] **Step 2: Run it; expect FAIL.**

```powershell
node scripts/test-docker-manager-ui.mjs
```

Expected: the model/store `ok` line, then FAIL with `ENOENT ... docker-manager.js` (the load contract reads all five files).

- [ ] **Step 3: Create the root module.**

```js
/* Docker Manager app: the composition root. It builds the skeleton (rail, toolbar, banners, container table, detail
   drawer, dialog host), wires the store to the views, keeps the focus across repaints and owns the keyboard handling,
   the context menu, the window menus and the toasts. Pure logic lives in docker-manager-model.js, requests and polling
   in docker-manager-store.js, markup in docker-manager-views.js and the dialogs in docker-manager-dialogs.js. */
(function () {
    'use strict';

    const PREFS_KEY = 'docker-manager.ui.v1';
    const instances = new Map(), warned = new Set();
    function dispose(id) { instances.get(id)?.(); instances.delete(id); }

    function loadPrefs(model) {
        const fallback = { scope: 'all', filter: 'all', collapsed: [] };
        try {
            const saved = JSON.parse(localStorage.getItem(PREFS_KEY) || '{}');
            return {
                scope: typeof saved.scope === 'string' && saved.scope ? saved.scope : 'all',
                filter: model.STATE_FILTERS.includes(saved.filter) ? saved.filter : 'all',
                collapsed: Array.isArray(saved.collapsed) ? saved.collapsed.filter(key => typeof key === 'string').slice(0, 200) : []
            };
        } catch (error) { return fallback; }
    }
    function savePrefs(prefs) { try { localStorage.setItem(PREFS_KEY, JSON.stringify(prefs)); } catch (error) { /* storage may be blocked */ } }

    function render(host, id, ctx) {
        dispose(id);
        const model = window.DockerManagerModel, Views = window.DockerManagerViews, Dialogs = window.DockerManagerDialogs, Store = window.DockerManagerStore;
        const esc = ctx.esc;
        // A missing key shows its own name and is reported once, never a misleading label.
        const t = (key, vars) => {
            const full = 'docker_manager.' + key, value = ctx.t(full, vars);
            if (value === full && !warned.has(full)) { warned.add(full); console.warn('Docker Manager: missing translation ' + full); }
            return value;
        };
        const dt = (key, vars) => ctx.t(key, vars);
        const lang = document.documentElement.lang || 'en';
        const uid = 'dm-' + String(id).replace(/[^\w-]/g, '-');
        const prefs = loadPrefs(model);
        const lifetime = new AbortController(), signal = lifetime.signal;
        const store = Store.create({ api: ctx.api, readonly: !!ctx.readonly, scope: prefs.scope });
        const state = store.state;
        const views = Views.create({ t, esc, model, lang, uid, logTails: Store.LOG_TAILS });
        const ui = { query: '', filter: prefs.filter, collapsed: new Set(prefs.collapsed), activeKey: '', rail: '', banners: '', tbody: '', empty: '', drawer: '', menus: '', logMark: '', focusDrawer: false, pendingFocus: '', opener: '' };
        let disposed = false, frame = 0, announceTimer = 0, prefsTimer = 0, lastCount = '';
        const toastTimers = new Map();

        // ---- skeleton --------------------------------------------------------------------------------------------

        const filterButtons = model.STATE_FILTERS.map(name => '<button type="button" class="dm-filter" data-filter="' + name + '" aria-pressed="false"><span>' + esc(t('filter_' + name)) + '</span><span class="dm-filter-count" data-count="' + name + '">0</span></button>').join('');
        const columns = ['status', 'name', 'image', 'ports'].map(name => '<th scope="col" role="columnheader" class="dm-col-' + name + '">' + esc(t('col_' + name)) + '</th>').join('')
            + '<th scope="col" role="columnheader" class="dm-col-actions"><span class="dm-sr-only">' + esc(t('col_actions')) + '</span></th>';
        host.replaceChildren();
        host.innerHTML = '<div class="dm-app" data-dm-app data-drawer-state="closed"><div class="dm-shell">'
            + '<aside class="dm-rail" aria-label="' + esc(t('rail_label')) + '" data-rail></aside>'
            + '<section class="dm-main" aria-label="' + esc(t('nav_containers')) + '">'
            + '<div class="dm-toolbar"><div class="dm-search">' + views.glyph('search') + '<input type="search" class="dm-search-input" data-search autocomplete="off" spellcheck="false" placeholder="' + esc(t('search_placeholder')) + '" aria-label="' + esc(t('search_label')) + '"></div>'
            + '<div class="dm-filters" role="group" aria-label="' + esc(t('filter_group')) + '">' + filterButtons + '</div>'
            + '<span class="dm-result-count" data-result-count></span><span class="dm-readonly-chip" data-readonly-chip hidden>' + esc(t('readonly_chip')) + '</span>'
            + '<button type="button" class="dm-icon-btn" data-action="refresh" data-refresh aria-label="' + esc(t('refresh')) + '" title="' + esc(t('refresh')) + '">' + views.glyph('refresh') + '</button></div>'
            + '<div class="dm-banners" data-banners></div>'
            + '<div class="dm-content"><div class="dm-table-scroll" data-table-scroll>'
            + '<table class="dm-table" role="treegrid" aria-label="' + esc(t('nav_containers')) + '" data-table hidden><thead><tr role="row">' + columns + '</tr></thead><tbody data-tbody></tbody></table>'
            + '<div class="dm-empty" data-empty hidden></div></div>'
            + '<section class="dm-drawer" data-drawer aria-labelledby="' + uid + '-drawer-title" hidden></section></div></section></div>'
            + '<dialog class="dm-dialog" data-dialog aria-labelledby="' + uid + '-dialog-title"><button type="button" class="dm-icon-btn dm-dialog-close" data-dialog-dismiss aria-label="' + esc(dt('desktop.close')) + '">' + views.glyph('close') + '</button><div data-dialog-content></div></dialog>'
            + '<div class="dm-toasts" data-toasts></div><div class="dm-sr-only" role="status" aria-live="polite" data-live></div></div>';
        const root = host.querySelector('[data-dm-app]');
        const q = selector => root.querySelector(selector);
        const railEl = q('[data-rail]'), searchEl = q('[data-search]'), bannersEl = q('[data-banners]'), scrollEl = q('[data-table-scroll]'), tableEl = q('[data-table]'), tbodyEl = q('[data-tbody]');
        const emptyEl = q('[data-empty]'), drawerEl = q('[data-drawer]'), toastsEl = q('[data-toasts]'), liveEl = q('[data-live]'), refreshEl = q('[data-refresh]');
        const countEl = q('[data-result-count]'), readonlyEl = q('[data-readonly-chip]');

        // ---- helpers ---------------------------------------------------------------------------------------------

        function announce(text) {
            if (disposed || !text) return;
            clearTimeout(announceTimer);
            announceTimer = setTimeout(() => { if (!disposed) liveEl.textContent = text; }, 150);
        }
        function removeToast(el) {
            clearTimeout(toastTimers.get(el));
            toastTimers.delete(el);
            el.remove();
        }
        function toast(message, tone) {
            if (disposed || !message) return;
            const el = document.createElement('div');
            el.className = 'dm-toast';
            el.dataset.tone = tone || 'info';
            el.setAttribute('role', tone === 'error' ? 'alert' : 'status');
            el.textContent = message;
            // A modal dialog sits in the top layer above the app, so toasts raised while it is open go inside it.
            const place = dialogs.isOpen() ? q('[data-dialog]') : root;
            if (toastsEl.parentNode !== place) place.append(toastsEl);
            toastsEl.append(el);
            while (toastsEl.children.length > 3) removeToast(toastsEl.firstElementChild);
            toastTimers.set(el, setTimeout(() => removeToast(el), tone === 'error' ? 8000 : 4000));
        }
        function savePrefsSoon() {
            clearTimeout(prefsTimer);
            prefsTimer = setTimeout(() => savePrefs({ scope: state.scope, filter: ui.filter, collapsed: [...ui.collapsed].slice(0, 200) }), 300);
        }
        // Keeps the focus on the same control when a part of the app is repainted (controls carry data-focus-id).
        function keepFocus(container, mutate) {
            const active = document.activeElement;
            const focusId = active && container.contains(active) ? active.dataset.focusId || '' : '';
            mutate();
            if (!focusId) return;
            const find = value => container.querySelector('[data-focus-id="' + CSS.escape(value) + '"]');
            let next = find(focusId);
            if (!next && focusId.startsWith('act:')) next = find('row:' + focusId.slice(4, focusId.lastIndexOf(':')));
            next?.focus({ preventScroll: true });
        }
        function restoreFocus(target) {
            if (disposed) return;
            if (target && target.isConnected && typeof target.focus === 'function') { target.focus({ preventScroll: true }); return; }
            (tbodyEl.querySelector('[data-nav-key][tabindex="0"]') || searchEl).focus({ preventScroll: true });
        }
        const findRow = key => model.buildRows(state.hosts, state.byHost, 'all').find(row => row.key === key) || null;
        const rowEl = key => tbodyEl.querySelector('[data-nav-key="' + CSS.escape(key) + '"]');
        const isTextField = el => !!el && (el.matches?.('input, textarea, select') || el.isContentEditable);

        const dialogs = Dialogs.create({ t, dt, esc, model, store, root, views, toast, announce, uid, restoreFocus });

        // ---- snapshot and painting -------------------------------------------------------------------------------

        function snapshot() {
            const hosts = state.hosts;
            const scopeHosts = state.scope === 'all' ? hosts : hosts.filter(item => item.id === state.scope);
            const rows = model.buildRows(hosts, state.byHost, state.scope);
            const visible = model.filterRows(rows, { query: ui.query, filter: ui.filter });
            const multi = state.scope === 'all' && hosts.length > 1;
            const groups = model.groupRows(visible, { multiHost: multi, hostOrder: hosts.map(item => item.id) });
            return { state, hosts, scopeHosts, rows, visible, groups, headers: model.showGroupHeaders(groups), collapsed: ui.collapsed, multi, counts: model.counts(rows, ui.query), canMutate: store.canMutate(), canEditHosts: store.canEditHosts() };
        }

        function applyRoving() {
            const items = [...tbodyEl.querySelectorAll('[data-nav-key]')];
            if (!items.length) return;
            const active = items.find(el => el.dataset.navKey === ui.activeKey) || items[0];
            ui.activeKey = active.dataset.navKey;
            for (const el of items) {
                const on = el === active;
                el.tabIndex = on ? 0 : -1;
                for (const button of el.querySelectorAll('[data-row-btn]')) button.tabIndex = on ? 0 : -1;
            }
        }

        function paintToolbar(snap) {
            for (const button of root.querySelectorAll('[data-filter]')) button.setAttribute('aria-pressed', String(button.dataset.filter === ui.filter));
            for (const el of root.querySelectorAll('[data-count]')) el.textContent = String(snap.counts[el.dataset.count]);
            const text = snap.rows.length || snap.state.hostsLoaded ? t('count_containers', { shown: snap.visible.length, total: snap.rows.length }) : '';
            countEl.textContent = text;
            readonlyEl.hidden = snap.canMutate;
            readonlyEl.title = snap.state.dockerReadonly ? t('readonly_note') : t('readonly_desktop');
            const busy = store.pending();
            refreshEl.classList.toggle('is-spinning', busy);
            refreshEl.setAttribute('aria-busy', String(busy));
            if (text && text !== lastCount && (ui.query || ui.filter !== 'all')) announce(text);
            lastCount = text;
        }

        function paintDrawer(snap) {
            const detail = state.detail;
            if (!detail) {
                if (!drawerEl.hidden) { drawerEl.hidden = true; drawerEl.innerHTML = ''; ui.drawer = ''; root.dataset.drawerState = 'closed'; }
                return;
            }
            const row = snap.rows.find(item => item.key === detail.key);
            if (!row) { store.closeDetail(); return; }
            const html = views.drawer(snap, row);
            const opening = drawerEl.hidden;
            if (html !== ui.drawer) {
                ui.drawer = html;
                keepFocus(drawerEl, () => { drawerEl.innerHTML = html; });
            }
            drawerEl.hidden = false;
            root.dataset.drawerState = 'open';
            const log = drawerEl.querySelector('[data-log]');
            const mark = detail.key + ':' + detail.logs.text.length;
            if (log && mark !== ui.logMark) { ui.logMark = mark; log.scrollTop = log.scrollHeight; }
            if (opening || ui.focusDrawer) {
                if (ui.focusDrawer) drawerEl.querySelector('[role="tab"][tabindex="0"]')?.focus({ preventScroll: true });
                ui.focusDrawer = false;
            }
        }

        function paint() {
            frame = 0;
            if (disposed) return;
            const snap = snapshot();
            root.dataset.readonly = String(!snap.canMutate);
            root.dataset.multi = String(snap.multi);
            root.dataset.disabled = String(state.disabled);
            const railHtml = views.rail(snap);
            if (railHtml !== ui.rail) { ui.rail = railHtml; keepFocus(railEl, () => { railEl.innerHTML = railHtml; }); }
            paintToolbar(snap);
            const bannerHtml = state.disabled ? '' : views.banners(snap);
            if (bannerHtml !== ui.banners) { ui.banners = bannerHtml; keepFocus(bannersEl, () => { bannersEl.innerHTML = bannerHtml; }); }
            const emptyHtml = views.empty(snap);
            const bodyHtml = emptyHtml || !snap.visible.length ? '' : views.tbody(snap);
            if (bodyHtml !== ui.tbody) { ui.tbody = bodyHtml; keepFocus(tbodyEl, () => { tbodyEl.innerHTML = bodyHtml; }); applyRoving(); }
            tableEl.hidden = !bodyHtml;
            if (emptyHtml !== ui.empty) { ui.empty = emptyHtml; keepFocus(emptyEl, () => { emptyEl.innerHTML = emptyHtml; }); }
            emptyEl.hidden = !emptyHtml;
            if (ui.pendingFocus) { rowEl(ui.pendingFocus)?.focus({ preventScroll: false }); ui.pendingFocus = ''; }
            paintDrawer(snap);
            const menuSig = ui.filter + '|' + snap.canEditHosts;
            if (menuSig !== ui.menus) { ui.menus = menuSig; syncMenus(snap); }
        }
        function schedulePaint() {
            if (disposed || frame) return;
            frame = requestAnimationFrame(paint);
        }

        function syncMenus() {
            ctx.setWindowMenus?.(id, [{ id: 'docker-manager', label: dt('desktop.app_docker_manager'), items: [
                { label: t('refresh'), icon: 'refresh', action: () => void store.refresh() },
                { type: 'separator' },
                { label: t('manage_hosts'), icon: 'server', action: () => dialogs.openHosts() },
                { label: t('host_add'), disabled: !store.canEditHosts(), action: () => dialogs.openHostForm(null) },
                { type: 'separator' },
                ...model.STATE_FILTERS.map(name => ({ label: t('filter_' + name), checked: ui.filter === name, action: () => setFilter(name) }))
            ] }]);
        }

        // ---- actions ---------------------------------------------------------------------------------------------

        function setFilter(name) {
            if (!model.STATE_FILTERS.includes(name)) return;
            ui.filter = name;
            savePrefsSoon();
            schedulePaint();
        }
        function pickHost(hostId) {
            if (state.detail && hostId !== 'all' && state.detail.hostId !== hostId) store.closeDetail();
            store.setScope(hostId);
            savePrefsSoon();
            schedulePaint();
        }
        function toggleGroup(key) {
            if (ui.collapsed.has(key)) ui.collapsed.delete(key); else ui.collapsed.add(key);
            savePrefsSoon();
            schedulePaint();
        }
        function openDetail(key, tab, focus) {
            const row = findRow(key);
            if (!row) return;
            ui.opener = key;
            ui.focusDrawer = !!focus;
            store.openDetail(row, tab);
        }
        function closeDetail() {
            if (!state.detail) return;
            const key = ui.opener || state.detail.key;
            store.closeDetail();
            requestAnimationFrame(() => { if (!disposed) (rowEl(key) || searchEl).focus({ preventScroll: true }); });
        }

        function reportFailure(label, name, error) {
            toast(t('action_failed', { action: label, name, message: views.errorText(error) }), 'error');
            announce(t('action_failed', { action: label, name, message: views.errorText(error) }));
        }

        // Stop, restart, pause and kill are never confirmed by the server; the client asks first for a protected container.
        async function performAction(key, action) {
            const row = findRow(key);
            if (!row || !store.canMutate()) return;
            const name = model.containerName(row.c), cid = model.containerId(row.c), label = views.actionLabel(action);
            if (model.needsProtectedConfirm(row.c, action) && (!await dialogs.confirmProtectedAction(row, action) || disposed)) return;
            const result = await store.act(row.hostId, cid, action);
            if (disposed) return;
            if (result.ok) { toast(t('action_done', { action: label, name }), 'success'); announce(t('action_done', { action: label, name })); } else if (result.error.code !== 'aborted' && result.error.code !== 'busy') reportFailure(label, name, result.error);
        }

        // A protected container is removed with two requests: the first answers 409 container_protected_confirmation_required
        // with the owner and a message, the repeat carries ?confirm=protected. When the client already knows the container is
        // protected, the remove dialog says so and the first request carries the confirmation.
        async function removeRow(key) {
            const row = findRow(key);
            if (!row || !store.canMutate()) return;
            const name = model.containerName(row.c), cid = model.containerId(row.c);
            if (!await dialogs.confirmRemove(row) || disposed) return;
            const rows = [...tbodyEl.querySelectorAll('.dm-row')], at = rows.findIndex(el => el.dataset.rowKey === key);
            const sibling = rows[at + 1] || rows[at - 1], neighbour = sibling ? sibling.dataset.rowKey : '';
            const running = ['running', 'paused', 'restarting'].includes(String(row.c.state).toLowerCase());
            let result = await store.remove(row.hostId, cid, { force: running, confirmProtected: model.isProtected(row.c) });
            if (disposed) return;
            if (!result.ok && result.error.code === 'container_protected_confirmation_required') {
                if (!await dialogs.confirmProtectedAction(row, 'remove', result.error) || disposed) return;
                result = await store.remove(row.hostId, cid, { force: running, confirmProtected: true });
                if (disposed) return;
            }
            const label = views.actionLabel('remove');
            if (result.ok) {
                if (state.detail && state.detail.key === key) store.closeDetail();
                const done = t('action_done', { action: label, name });
                ui.pendingFocus = neighbour;
                toast(done, 'success');
                announce(done);
                schedulePaint();
            } else if (result.error.code !== 'aborted' && result.error.code !== 'busy') reportFailure(label, name, result.error);
        }

        async function retryHost(hostId) {
            const hostRecord = state.hosts.find(item => item.id === hostId);
            if (!hostRecord) return;
            try { await store.testHost(hostId); } catch (error) { /* the banner shows the reason after the reload */ }
            await store.reloadContainers(hostId);
        }

        async function copyText(text) {
            try {
                if (!navigator.clipboard || !navigator.clipboard.writeText) throw new Error('no clipboard');
                await navigator.clipboard.writeText(text);
                toast(t('copied'), 'success');
            } catch (error) { toast(t('copy_failed'), 'error'); }
        }

        function showRowMenu(key, x, y) {
            const row = findRow(key);
            if (!row || typeof ctx.showContextMenu !== 'function') return;
            const actions = model.actionsFor(row.c, !store.canMutate());
            const glyphKey = { start: 'run', unpause: 'run', stop: 'stop', restart: 'refresh', kill: 'x' };
            const entry = (action, danger) => ({ label: views.actionLabel(action), icon: glyphKey[action], action: () => void performAction(key, action), danger: !!danger });
            const items = [
                { label: t('action_open_details'), icon: 'info', action: () => openDetail(key, 'overview', true) },
                { label: t('action_logs'), icon: 'terminal', action: () => openDetail(key, 'logs', true) },
                { label: t('action_inspect'), icon: 'info', action: () => openDetail(key, 'inspect', true) }
            ];
            const mutating = [];
            if (actions.toggle) mutating.push(entry(actions.toggle));
            if (actions.restart) mutating.push(entry('restart'));
            for (const action of actions.menu) mutating.push(entry(action, action === 'kill'));
            if (actions.remove) mutating.push({ label: views.actionLabel('remove'), icon: 'trash', action: () => void removeRow(key) });
            if (mutating.length) items.push({ separator: true }, ...mutating);
            ctx.showContextMenu(x, y, items);
        }

        // ---- events ----------------------------------------------------------------------------------------------

        function onAction(el) {
            const key = el.dataset.rowKey || '';
            switch (el.dataset.action) {
                case 'pick-host': pickHost(el.dataset.hostId); break;
                case 'manage-hosts': dialogs.openHosts(); break;
                case 'refresh': void store.refresh(); break;
                case 'toggle-group': toggleGroup(el.dataset.groupKey); break;
                case 'act': void performAction(key, el.dataset.act); break;
                case 'remove': void removeRow(key); break;
                case 'logs': openDetail(key, 'logs', true); break;
                case 'more': { const rect = el.getBoundingClientRect(); showRowMenu(key, rect.left, rect.bottom + 4); break; }
                case 'close-detail': closeDetail(); break;
                case 'tab': store.setDetailTab(el.dataset.tab); break;
                case 'logs-reload': void store.loadLogs(); break;
                case 'copy-logs': if (state.detail) void copyText(views.stripAnsi(state.detail.logs.text)); break;
                case 'copy-inspect': if (state.detail && state.detail.inspect.status === 'ok') void copyText(JSON.stringify(state.detail.inspect.data, null, 2)); break;
                case 'retry-host': void retryHost(el.dataset.hostId); break;
                case 'edit-host': { const target = state.hosts.find(item => item.id === el.dataset.hostId); if (target) dialogs.openHostForm(target); break; }
                case 'clear-filters': ui.query = ''; searchEl.value = ''; ui.filter = 'all'; savePrefsSoon(); schedulePaint(); break;
                case 'open-settings': window.open('/config#docker', '_blank', 'noopener'); break;
                default: break;
            }
        }

        root.addEventListener('click', event => {
            const el = event.target.closest('[data-action]');
            if (el && root.contains(el)) { onAction(el); return; }
            const filter = event.target.closest('[data-filter]');
            if (filter) { setFilter(filter.dataset.filter); return; }
            const rowNode = event.target.closest('.dm-row');
            if (rowNode && !event.target.closest('button, a, input, select, summary')) openDetail(rowNode.dataset.rowKey, undefined, false);
        }, { signal });
        searchEl.addEventListener('input', () => { ui.query = searchEl.value; schedulePaint(); }, { signal });
        root.addEventListener('change', event => {
            if (event.target.matches('[data-log-tail]')) void store.loadLogs(Number(event.target.value));
        }, { signal });
        tbodyEl.addEventListener('focusin', event => {
            const nav = event.target.closest('[data-nav-key]');
            if (nav && nav.dataset.navKey !== ui.activeKey) { ui.activeKey = nav.dataset.navKey; applyRoving(); }
        }, { signal });

        function moveFocus(to) {
            const items = [...tbodyEl.querySelectorAll('[data-nav-key]')];
            const target = items[Math.max(0, Math.min(items.length - 1, to))];
            if (target) target.focus();
        }
        scrollEl.addEventListener('keydown', event => {
            const nav = event.target.closest('[data-nav-key]');
            if (!nav || event.ctrlKey || event.metaKey || event.altKey) return;
            const items = [...tbodyEl.querySelectorAll('[data-nav-key]')], index = items.indexOf(nav);
            const onControl = event.target !== nav;
            const isGroup = nav.classList.contains('dm-group-row');
            switch (event.key) {
                case 'ArrowDown': event.preventDefault(); moveFocus(index + 1); break;
                case 'ArrowUp': event.preventDefault(); moveFocus(index - 1); break;
                case 'Home': event.preventDefault(); moveFocus(0); break;
                case 'End': event.preventDefault(); moveFocus(items.length - 1); break;
                case 'ArrowRight': if (isGroup && !onControl) { event.preventDefault(); if (ui.collapsed.has(nav.dataset.groupKey)) toggleGroup(nav.dataset.groupKey); } break;
                case 'ArrowLeft':
                    if (isGroup && !onControl) { event.preventDefault(); if (!ui.collapsed.has(nav.dataset.groupKey)) toggleGroup(nav.dataset.groupKey); }
                    else if (!isGroup && !onControl) {
                        const group = items.slice(0, index).reverse().find(el => el.classList.contains('dm-group-row'));
                        if (group) { event.preventDefault(); group.focus(); }
                    }
                    break;
                case 'Enter': case ' ':
                    if (onControl) break;
                    event.preventDefault();
                    if (isGroup) toggleGroup(nav.dataset.groupKey); else openDetail(nav.dataset.rowKey, undefined, true);
                    break;
                case 'Delete': if (!isGroup) { event.preventDefault(); void removeRow(nav.dataset.rowKey); } break;
                case 'ContextMenu': case 'F10':
                    if (event.key === 'F10' && !event.shiftKey) break;
                    if (!isGroup) { event.preventDefault(); const rect = nav.getBoundingClientRect(); showRowMenu(nav.dataset.rowKey, rect.left + 24, rect.bottom - 4); }
                    break;
                default: break;
            }
        }, { signal });

        railEl.addEventListener('keydown', event => {
            const option = event.target.closest('[role="option"]');
            if (!option) return;
            const options = [...railEl.querySelectorAll('[role="option"]')], index = options.indexOf(option);
            const to = { ArrowDown: index + 1, ArrowUp: index - 1, Home: 0, End: options.length - 1 }[event.key];
            if (to === undefined) return;
            event.preventDefault();
            const target = options[Math.max(0, Math.min(options.length - 1, to))];
            if (target && target !== option) { pickHost(target.dataset.hostId); requestAnimationFrame(() => railEl.querySelector('[data-focus-id="host:' + CSS.escape(target.dataset.hostId) + '"]')?.focus()); }
        }, { signal });

        drawerEl.addEventListener('keydown', event => {
            const tab = event.target.closest('[role="tab"]');
            if (!tab) return;
            const tabs = [...drawerEl.querySelectorAll('[role="tab"]')], index = tabs.indexOf(tab);
            const to = { ArrowRight: (index + 1) % tabs.length, ArrowLeft: (index - 1 + tabs.length) % tabs.length, Home: 0, End: tabs.length - 1 }[event.key];
            if (to === undefined) return;
            event.preventDefault();
            store.setDetailTab(tabs[to].dataset.tab);
            requestAnimationFrame(() => drawerEl.querySelector('[role="tab"][aria-selected="true"]')?.focus({ preventScroll: true }));
        }, { signal });

        root.addEventListener('keydown', event => {
            if (event.key === 'Escape' && !dialogs.isOpen()) {
                if (event.target === searchEl && searchEl.value) { event.preventDefault(); searchEl.value = ''; ui.query = ''; schedulePaint(); return; }
                if (state.detail && (drawerEl.contains(event.target) || scrollEl.contains(event.target))) { event.preventDefault(); event.stopPropagation(); closeDetail(); }
                return;
            }
            if (event.key === '/' && !event.ctrlKey && !event.metaKey && !event.altKey && !isTextField(event.target) && !dialogs.isOpen()) { event.preventDefault(); searchEl.focus(); }
        }, { signal });

        if (typeof ctx.wireContextMenuBoundary === 'function') {
            ctx.wireContextMenuBoundary(root, {
                onContextMenu: event => {
                    const node = event.target.closest('.dm-row');
                    if (!node) return false;
                    showRowMenu(node.dataset.rowKey, event.clientX, event.clientY);
                    return true;
                }
            });
        }

        // ---- start -----------------------------------------------------------------------------------------------

        const unsubscribe = store.subscribe(schedulePaint);
        searchEl.value = ui.query;
        paint();
        store.attach({ root, win: host.closest('.vd-window') });

        instances.set(id, () => {
            if (disposed) return;
            disposed = true;
            lifetime.abort();
            cancelAnimationFrame(frame);
            clearTimeout(announceTimer);
            clearTimeout(prefsTimer);
            savePrefs({ scope: state.scope, filter: ui.filter, collapsed: [...ui.collapsed].slice(0, 200) });
            for (const el of [...toastTimers.keys()]) removeToast(el);
            unsubscribe();
            dialogs.dispose();
            store.dispose();
            ctx.setWindowMenus?.(id, []);
            root.replaceChildren();
        });
    }

    window.DockerManagerApp = { render, dispose };
})();
```

- [ ] **Step 4: Run; expect PASS.**

```powershell
node --check ui/js/desktop/apps/docker-manager.js
node scripts/test-docker-manager-ui.mjs
go test ./ui -run "TestFrontend_JS_NoLiteralTranslationFallbacks|TestFrontend_JS_HardcodedUIToasts|TestDesktopUsedI18nKeysExistInAllLanguages|TestDesktopDynamicI18nPrefixKeysExistInAllLanguages|TestDockerManagerLocalesAreComplete|TestUIAvoidsCookieConsentClassPrefix|TestShippedWebAssetsLoadNothingExternal" -count=1
```

Expected: `docker-manager ui tests: model and store ok`, then `docker-manager ui tests: stylesheet, load contract and translations ok`,
then `ok  aurago/ui`. (The i18n lint now resolves every `t('...')` of the five files because Task C3 registered them.)

- [ ] **Step 5: Detect changes and commit.**

```powershell
git add ui/js/desktop/apps/docker-manager.js scripts/test-docker-manager-ui.mjs
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): add the app composition root with keyboard, menus and toasts" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task C8: Shell wiring, bundle and the browser smoke

**Files**

- Modify: `ui/js/desktop/core/module-loader.js` (`DESKTOP_APP_ASSETS`, `APP_I18N_SECTIONS`)
- Modify: `ui/js/desktop/core/menus-and-routing.js` (`renderAppContent`)
- Modify: `ui/js/desktop/core/desktop-foundation.js` (`appGlobalName`)
- Modify: `ui/js/desktop/core/window-shell-runtime.js` (`appWindowSize`, `appWindowMinSize`, `shouldUseMobileWideWindow`)
- Modify (generated, committed): `ui/js/desktop/bundles/main.bundle.js`
- Test: `ui/desktop_docker_manager_wiring_test.go` (create), `ui/desktop_docker_manager_browser_test.go` (create)

`main.bundle.js` is a plain concatenation of the core files and **is committed**; `npm run build:ui` regenerates it and
`npm run check:ui` proves it is current. `module-loader.js` is not part of the bundle (the page loads it directly).

- [ ] **Step 1: Impact before editing** (all five edits are additive: one map entry, one `if` branch, one size preset, one
  minimum size, one extra condition).

```powershell
npx gitnexus impact "renderAppContent" --direction upstream --repo AuraGo
npx gitnexus impact "appGlobalName" --direction upstream --repo AuraGo
npx gitnexus impact "appWindowSize" --direction upstream --repo AuraGo
npx gitnexus impact "appWindowMinSize" --direction upstream --repo AuraGo
npx gitnexus impact "shouldUseMobileWideWindow" --direction upstream --repo AuraGo
git grep -n -e "renderAppContent(" -e "appGlobalName(" -e "appWindowSize(" -e "appWindowMinSize(" -e "shouldUseMobileWideWindow(" -- ui/js
```

Expected: `renderAppContent` is the hub of app opening (callers in `window-shell-runtime.js`, session restore, `openApp`); a HIGH or
CRITICAL rating is expected and acceptable for an additive branch that only matches `appId === 'docker-manager'`; say so in the
commit body. The size helpers are called when a window opens or resizes.

- [ ] **Step 2: Write the failing tests.**

Create `ui/desktop_docker_manager_wiring_test.go`:

```go
package ui

import (
	"regexp"
	"strings"
	"testing"
)

// The loader, the shell routing, the window sizing and the generated main bundle know the Docker Manager app, and its
// five modules load in the order they depend on each other. Behaviour is covered by scripts/test-docker-manager-ui.mjs
// and TestDesktopDockerManagerBrowser.
func TestDockerManagerWiring(t *testing.T) {
	t.Parallel()

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	entry := regexp.MustCompile(`'docker-manager': \{ styles: appStyles\('/css/desktop-app-docker-manager\.css'\), scripts: \[([^\]]*)\] \}`).FindStringSubmatch(loader)
	if entry == nil {
		t.Fatal("module-loader.js must register the docker-manager assets")
	}
	var scripts []string
	for _, match := range regexp.MustCompile(`'/js/desktop/apps/([a-z-]+\.js)'`).FindAllStringSubmatch(entry[1], -1) {
		scripts = append(scripts, match[1])
	}
	want := []string{"docker-manager-model.js", "docker-manager-store.js", "docker-manager-views.js", "docker-manager-dialogs.js", "docker-manager.js"}
	if strings.Join(scripts, ",") != strings.Join(want, ",") {
		t.Fatalf("docker-manager scripts = %v, want %v", scripts, want)
	}
	if !strings.Contains(loader, "'docker-manager': ['docker_manager']") {
		t.Fatal("module-loader.js must load the docker_manager translation section for the app")
	}

	for file, markers := range map[string][]string{
		"js/desktop/core/menus-and-routing.js":    {"appId === 'docker-manager' && window.DockerManagerApp", "window.DockerManagerApp.render(contentEl(id), id,"},
		"js/desktop/core/desktop-foundation.js":   {"'docker-manager': 'DockerManagerApp'"},
		"js/desktop/core/window-shell-runtime.js": {"'docker-manager': { width: 1180, height: 760 }", "appId === 'docker-manager') return { width: 440, height: 400 }"},
		"js/desktop/bundles/main.bundle.js":       {"appId === 'docker-manager' && window.DockerManagerApp", "'docker-manager': 'DockerManagerApp'", "'docker-manager': { width: 1180, height: 760 }"},
	} {
		source := readDesktopAssetText(t, file)
		for _, marker := range markers {
			if !strings.Contains(source, marker) {
				t.Errorf("%s is missing %q (run npm run build:ui after editing the core files)", file, marker)
			}
		}
	}
}
```

Create `ui/desktop_docker_manager_browser_test.go`. It serves a fake `/api/docker-manager` (two reachable hosts, one that times
out, a never-tested status with the zero `checked_at`, an unhealthy container, compose projects on two hosts) on the real
desktop shell and checks the rail, the banner, the grouping, the host chips, the amber unhealthy tone, the state filter, the
search, keyboard Enter and Escape on the drawer, the Logs tab reading `?tail=200`, and the host manager dialog:

```go
package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// dockerManagerFixture answers /api/docker-manager like the server does, with two reachable hosts (Local, NAS) and one that
// times out (Edge). It records every request so the test can see what the app asked for.
type dockerManagerFixture struct {
	mu       sync.Mutex
	requests []string
}

func dockerManagerContainer(host, name string, extra map[string]any) map[string]any {
	full := (host + name + strings.Repeat("0", 64))[:64]
	container := map[string]any{
		"id": full[:12], "full_id": full, "names": []string{name}, "image": "nginx:1.27", "image_id": "sha256:aaa", "state": "running",
		"status": "Up 3 hours", "created": 1760000000, "ports": []any{}, "mounts": []any{}, "networks": []string{"bridge"},
	}
	for key, value := range extra {
		container[key] = value
	}
	return container
}

func (f *dockerManagerFixture) joined() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.requests, "\n")
}

func (f *dockerManagerFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI())
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	write := func(status int, body map[string]any) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/docker-manager")
	switch {
	case r.Method == http.MethodGet && path == "/hosts":
		write(http.StatusOK, map[string]any{"status": "ok", "hosts": []map[string]any{
			{"id": "local", "name": "Local", "kind": "local", "address": "unix:///var/run/docker.sock", "port": 0, "stacks_dir": "/opt/stacks", "host_socket_path": "/var/run/docker.sock", "color": "blue", "sort": -1, "builtin": true, "secret_set": false, "plaintext": false,
				"status": map[string]any{"state": "ok", "checked_at": "2026-10-26T10:00:00Z", "engine_version": "27.3.1", "api_version": "1.47", "os": "linux", "arch": "amd64"}},
			// Never tested: the server sends the zero time, which the app must read as "not checked yet".
			{"id": "h_nas", "name": "NAS", "kind": "ssh", "address": "192.0.2.20", "port": 0, "stacks_dir": "/srv/stacks", "host_socket_path": "/var/run/docker.sock", "color": "teal", "ssh_user": "root", "sort": 1, "builtin": false, "secret_set": true, "plaintext": false,
				"status": map[string]any{"state": "unknown", "checked_at": "0001-01-01T00:00:00Z"}},
			{"id": "h_edge", "name": "Edge", "kind": "tcp", "address": "192.0.2.50", "port": 2375, "stacks_dir": "/opt/stacks", "host_socket_path": "/var/run/docker.sock", "color": "amber", "sort": 2, "builtin": false, "secret_set": false, "plaintext": true,
				"status": map[string]any{"state": "error", "code": "timeout", "message": "dial tcp 192.0.2.50:2375: i/o timeout", "checked_at": "2026-10-26T10:00:00Z"}},
		}})
	case r.Method == http.MethodGet && path == "/hosts/local/containers":
		write(http.StatusOK, map[string]any{"status": "ok", "containers": []map[string]any{
			dockerManagerContainer("local", "shop-web", map[string]any{"compose_project": "shop", "compose_service": "web", "ports": []map[string]any{{"private": 80, "public": 8080, "type": "tcp"}}}),
			dockerManagerContainer("local", "shop-db", map[string]any{"compose_project": "shop", "compose_service": "db", "image": "postgres:16", "health": "unhealthy", "status": "Up 3 hours (unhealthy)"}),
			dockerManagerContainer("local", "tools", map[string]any{"state": "exited", "status": "Exited (0) 2 days ago"}),
		}})
	case r.Method == http.MethodGet && path == "/hosts/h_nas/containers":
		write(http.StatusOK, map[string]any{"status": "ok", "containers": []map[string]any{
			dockerManagerContainer("h_nas", "plex", map[string]any{"compose_project": "media", "compose_service": "plex", "image": "lscr.io/linuxserver/plex:latest"}),
		}})
	case r.Method == http.MethodGet && path == "/hosts/h_edge/containers":
		write(http.StatusBadGateway, map[string]any{"status": "error", "code": "host_unreachable", "reason": "timeout", "message": "dial tcp 192.0.2.50:2375: i/o timeout"})
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/logs"):
		write(http.StatusOK, map[string]any{"status": "ok", "logs": "2026-10-26T10:00:01Z starting\n2026-10-26T10:00:02Z listening on :80\n"})
	default:
		write(http.StatusNotFound, map[string]any{"status": "error", "code": "not_found", "message": "no route " + r.Method + " " + path})
	}
}

func TestDesktopDockerManagerBrowser(t *testing.T) {
	fixture := &dockerManagerFixture{}
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/docker-manager/": fixture})
	page.MustEval(`()=>auditTest.openApp('docker-manager')`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===4`)

	// Rail: All hosts and three hosts; the unreachable one says so and gets a banner with the reason.
	if got := page.MustEval(`()=>document.querySelectorAll('.dm-host-btn').length`).Int(); got != 4 {
		t.Fatalf("rail entries = %d, want All hosts + 3 hosts", got)
	}
	if !page.MustEval(`()=>document.querySelectorAll('.dm-banner').length===1 && /Edge/.test(document.querySelector('.dm-banner h3').textContent)`).Bool() {
		t.Fatal("the unreachable host needs exactly one banner naming it")
	}
	if page.MustEval(`()=>document.querySelector('.dm-host-btn[data-host-id="h_nas"] .dm-dot').dataset.tone`).String() != "ok" {
		t.Fatal("a host that answered must show as reachable, even when its status was never checked")
	}

	// The list is grouped by compose project, single containers last, and every multi-host row carries a host chip.
	if got := page.MustEval(`()=>[...document.querySelectorAll('.dm-group-row .dm-group-name')].slice(0,2).map(e=>e.textContent).join(',')`).String(); got != "media,shop" {
		t.Fatalf("project groups = %q, want media,shop", got)
	}
	if got := page.MustEval(`()=>document.querySelectorAll('.dm-row .dm-chip').length`).Int(); got != 4 {
		t.Fatalf("host chips = %d, want one per row", got)
	}
	if page.MustEval(`()=>[...document.querySelectorAll('.dm-row')].find(r=>r.querySelector('.dm-name').textContent==='shop-db').dataset.tone`).String() != "warn" {
		t.Fatal("an unhealthy container must be amber")
	}

	// State filter and search.
	page.MustEval(`()=>document.querySelector('[data-filter="problem"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===1`)
	page.MustEval(`()=>document.querySelector('[data-filter="all"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===4`)
	page.MustEval(`()=>{const s=document.querySelector('[data-search]');s.value='plex';s.dispatchEvent(new Event('input',{bubbles:true}))}`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===1`)
	page.MustEval(`()=>{const s=document.querySelector('[data-search]');s.value='';s.dispatchEvent(new Event('input',{bubbles:true}))}`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===4`)

	// Keyboard: Enter on a row opens the drawer, the Logs tab shows the log lines, Escape closes the drawer again.
	page.MustEval(`()=>[...document.querySelectorAll('.dm-row')].find(r=>r.querySelector('.dm-name').textContent==='shop-web').focus()`)
	if err := page.Keyboard.Type(input.Enter); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>!document.querySelector('[data-drawer]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-tab="logs"]').click()`)
	page.MustWait(`()=>{const log=document.querySelector('[data-log]');return !!log && log.textContent.includes('listening on :80')}`)
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>document.querySelector('[data-drawer]').hidden`)
	if !strings.Contains(fixture.joined(), "/logs?tail=200") {
		t.Fatalf("the Logs tab must read the last 200 lines: %s", fixture.joined())
	}

	// Host manager: lists the three hosts, Escape closes it.
	page.MustEval(`()=>document.querySelector('[data-action="manage-hosts"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('dialog[open] .dm-hostrow').length===3`)
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>!document.querySelector('dialog[open]')`)

	if errors := page.MustEval(`()=>window.fixtureErrors`).Arr(); len(errors) != 0 {
		t.Fatalf("page errors: %v", errors)
	}
}
```

- [ ] **Step 3: Run them; expect FAIL.**

```powershell
go test ./ui -run TestDockerManagerWiring -count=1
$env:AURAGO_RUN_BROWSER_SMOKE = '1'
go test ./ui -run TestDesktopDockerManagerBrowser -count=1
```

Expected: the wiring test fails with `module-loader.js must register the docker-manager assets`; the browser test fails because
the shell has no renderer for the app (a rod timeout waiting for `.dm-row`).

- [ ] **Step 4: Wire the app into the shell.**

In `ui/js/desktop/core/module-loader.js`:

Edit 1 of 2: replace

```text
        'ruview': { styles: appStyles('/css/ruview.css'), scripts: ['/js/desktop/apps/ruview-model.js', '/js/desktop/apps/ruview-store.js', '/js/desktop/apps/ruview-views.js', '/js/desktop/apps/ruview.js'] },
```

with

```text
        'ruview': { styles: appStyles('/css/ruview.css'), scripts: ['/js/desktop/apps/ruview-model.js', '/js/desktop/apps/ruview-store.js', '/js/desktop/apps/ruview-views.js', '/js/desktop/apps/ruview.js'] },
        'docker-manager': { styles: appStyles('/css/desktop-app-docker-manager.css'), scripts: ['/js/desktop/apps/docker-manager-model.js', '/js/desktop/apps/docker-manager-store.js', '/js/desktop/apps/docker-manager-views.js', '/js/desktop/apps/docker-manager-dialogs.js', '/js/desktop/apps/docker-manager.js'] },
```

Edit 2 of 2: replace

```text
        'ruview': ['ruview'],
```

with

```text
        'ruview': ['ruview'],
        'docker-manager': ['docker_manager'],
```

In `ui/js/desktop/core/menus-and-routing.js`:

Replace

```text
        if (appId === 'ha-switchboard' && window.HASwitchboardApp) {
```

with

```text
        if (appId === 'docker-manager' && window.DockerManagerApp) {
            return window.DockerManagerApp.render(contentEl(id), id, Object.assign({}, context || {}, {
                esc, t, api, setWindowMenus, clearWindowMenus, showContextMenu, wireContextMenuBoundary, readonly: desktopReadonly()
            }));
        }
        if (appId === 'ha-switchboard' && window.HASwitchboardApp) {
```

In `ui/js/desktop/core/desktop-foundation.js`:

Replace

```text
            'ruview': 'RuViewApp',
```

with

```text
            'ruview': 'RuViewApp',
            'docker-manager': 'DockerManagerApp',
```

In `ui/js/desktop/core/window-shell-runtime.js`:

Edit 1 of 3: replace

```text
            infra: { width: 1280, height: 820 },
```

with

```text
            infra: { width: 1280, height: 820 },
            'docker-manager': { width: 1180, height: 760 },
```

Edit 2 of 3: replace

```text
if (appId === 'infra' || appId === 'ha-switchboard') return true;
```

with

```text
if (appId === 'infra' || appId === 'ha-switchboard' || appId === 'docker-manager') return true;
```

Edit 3 of 3: replace

```text
        if (appId === 'infra') return { width: 360, height: 480 };
```

with

```text
        if (appId === 'infra') return { width: 360, height: 480 };
        if (appId === 'docker-manager') return { width: 440, height: 400 };
```

- [ ] **Step 5: Rebuild the bundle and check it.**

```powershell
npm run build:ui
npm run check:ui
git diff --stat -- ui/js/desktop/bundles
```

Expected: `Built ui/js/desktop/bundles/main.bundle.js (<N> parts)` among the lines, `Checked ... main.bundle.js (<N> parts)` from the
check, and the diff stat shows only `ui/js/desktop/bundles/main.bundle.js` (a handful of added lines and one replaced line:
the routing branch, the global name, the size preset, the minimum size and the wide-window condition). No other generated file
changes.

- [ ] **Step 6: Run the tests; expect PASS.**

```powershell
go test ./ui -run TestDockerManagerWiring -count=1
$env:AURAGO_RUN_BROWSER_SMOKE = '1'
go test ./ui -run TestDesktopDockerManagerBrowser -count=1 -v
```

Expected: `--- PASS: TestDockerManagerWiring` and `--- PASS: TestDesktopDockerManagerBrowser`. (If the browser test is skipped, the
environment variable did not reach the process; set it in the same shell.)

- [ ] **Step 7: Detect changes and commit** (the bundle is part of the commit).

```powershell
gofmt -l ui/desktop_docker_manager_wiring_test.go ui/desktop_docker_manager_browser_test.go
git add ui/js/desktop/core/module-loader.js ui/js/desktop/core/menus-and-routing.js ui/js/desktop/core/desktop-foundation.js ui/js/desktop/core/window-shell-runtime.js ui/js/desktop/bundles/main.bundle.js ui/desktop_docker_manager_wiring_test.go ui/desktop_docker_manager_browser_test.go
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "feat(docker-manager): wire the app into the desktop shell and rebuild the bundle" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task C9: App documentation and the final sweep

**Files**

- Modify: `ui/js/desktop/apps/AGENTS.md` (new `## Docker Manager` section before `## Synth Studio`, like the Infra section)

- [ ] **Step 1: Add the documentation section.**

In `ui/js/desktop/apps/AGENTS.md`:

Replace

```text

## Synth Studio
```

with

```text

## Docker Manager

- Load order (module-loader `docker-manager`): `docker-manager-model.js` (pure: container status and problem derivation,
  grouping by compose project, search and state filters, the multi-host merge, host health, the host form rules,
  error normalisation), `docker-manager-store.js` (every request through the desktop `api()` against `/api/docker-manager`,
  one request in flight per key, per-host container cache, polling only while the window is visible),
  `docker-manager-views.js` (HTML builders: rail, banners, table, empty states, detail drawer),
  `docker-manager-dialogs.js` (one native `<dialog>` with a view stack: host manager, add/edit form with Test connection,
  import from nest, confirmations), then `docker-manager.js` (composition root: skeleton, focus, keyboard, context menu,
  window menus, toasts). Style: `css/desktop-app-docker-manager.css`. Every file stays below 1100 lines and no file touches
  the DOM at load (`scripts/test-docker-manager-ui.mjs` runs them in a vm). Phase 1 shows Containers only; Stacks, Images,
  Volumes, Networks and Templates come in later phases, and no placeholder is rendered for them.
- The app appears only while Docker is enabled (`docker` capability, `desktopCapabilities` in `internal/server`). `ctx.readonly`
  (read-only desktop) hides container actions and host editing; a `read_only` answer from the API (`docker.readonly`) hides the
  container actions only, host management stays. Docker disabled answers 503 `docker_disabled` on every route and shows one
  explanatory state with a link to `/config#docker`.
- Repaints compare the new HTML with the last one and skip equal markup; controls carry `data-focus-id` and `keepFocus`
  restores the focus on the same control, so a poll never steals it. Do not re-render the table from a timer. The table is a
  `treegrid` with a roving tab stop: arrows move, Left/Right collapse and expand a group, Enter opens the drawer, Delete asks to
  remove, Escape closes the drawer, `/` focuses the search, Shift+F10 opens the row menu.
- A failed container list keeps the last containers of that host and shows a banner with the reason code
  (`dns`, `refused`, `timeout`, `tls`, `auth`, `ssh_host_key`, `socket_missing`, `api_too_old`, `unknown`, texts
  `docker_manager.reason_*`); the browser giving up after 35 s counts as `host_unreachable` with reason `timeout`. A host
  status whose `checked_at` is the zero time (`0001-01-01…`) is "not checked yet", never a date. `GET /hosts` answers from cached
  statuses and refreshes stale ones in the background, so it is polled together with the container lists.
- Secrets are write-only. An edited host with stored credentials shows "stored" and a Replace action instead of empty secret
  fields; blank fields keep the stored value, the TLS PEM fields are sent only when typed (the server merges them one by one),
  and Test connection posts the draft with the host's `id` so the server reuses the stored secrets. Plain TCP carries a visible
  warning. Names are unique ignoring case and "Local" is reserved (checked in the form, enforced by the server).
- Protected containers (lock icon) ask before stop, restart, pause, kill and remove. The server never confirms the first four;
  a protected remove answers 409 `container_protected_confirmation_required` with `owner` and `message`, the dialog shows that
  message and the request is repeated with `?confirm=protected` (plus `&force=1` when forcing).
- Host colours are the eight `data-host-color` values mapped to theme tokens in the stylesheet; no literal colours, no
  uppercase labels, motion only inside `prefers-reduced-motion: no-preference` on `body:not([data-animations="false"])` (the
  Node test guards this like the Infra test). A modal `<dialog>` sits in the top layer, so toasts raised while it is open are
  moved inside it.
- Strings: `desktop.app_docker_manager` and `docker_manager.*` in all 16 `lang/desktop/*.json` files, local helper `t` with the
  `docker_manager.` prefix (registered in `desktopLocalI18nHelpers`; the key families built at run time are in
  `desktopDynamicI18nPrefixes` in `ui/i18n_lint_test.go`). A new string needs its key in all 16 files with the same
  `{{placeholders}}`; `TestDockerManagerLocalesAreComplete` and the Node test check this.
- Verify: `node scripts/test-docker-manager-ui.mjs` (or `npm run test:docker-manager`),
  `go test ./ui -run DockerManager -count=1`, and
  `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopDockerManagerBrowser -count=1`. After editing a core file
  (`module-loader.js`, `menus-and-routing.js`, `desktop-foundation.js`, `window-shell-runtime.js`) run `npm run build:ui` and
  `npm run check:ui`. API contract: `internal/server` (`/api/docker-manager`), host registry: `internal/dockerhost`.

## Synth Studio
```

- [ ] **Step 2: Final verification sweep.**

```powershell
node scripts/test-docker-manager-ui.mjs
npm run check:ui
python scripts/import-desktop-icon-themes.py --check
node scripts/build-desktop-windows-xp-icons.mjs --check
go test ./internal/desktop -count=1
go test ./internal/server -run "TestDesktopCapabilityDockerFollowsDockerEnabled|TestCYDCreatorCapabilityAndConfigReload" -count=1
go test ./ui -run "DockerManager|TestDesktopPapirus|TestDesktopWhiteSur|TestDesktopAdditionalIcon|TestWindowsXPIcon|TestDesktopBuiltinAppNames|TestDesktopUsedI18n|TestDesktopDynamicI18n|TestDesktopStartMenuCategories|TestTranslations_|TestFrontend_JS|TestUIAvoidsCookie|TestShippedWebAssets|TestDesktopThemeRegistry|TestDesktopAudited" -count=1
$env:AURAGO_RUN_BROWSER_SMOKE = '1'
go test ./ui -run TestDesktopDockerManagerBrowser -count=1 -v
```

Expected: every command succeeds. Optional broad check: `go test ./ui -count=1` fails only in the two pre-existing tests named in
the Part C notes.

- [ ] **Step 3: Acceptance by a person on a real instance** (after parts A and B are merged and deployed; the user does this, the
  agent does not drive the production desktop):

  1. Enable Docker in Config; reload the desktop; Docker Manager appears in the Start menu under System. Disable Docker: it disappears.
  2. Open it: All hosts shows grouped containers with host chips; select Local, then each remote host; an unreachable host shows
     its banner with the reason, Retry and Edit host.
  3. Keyboard only: Tab to the table, arrows, Enter opens the drawer, Logs tab, Escape returns to the row; Delete asks before
     removing; `/` focuses the search.
  4. Host manager: add an SSH host (Test connection first), edit it (Credentials stored + Replace), import a Docker nest, delete a host.
  5. A protected AuraGo container asks before stop and remove and shows its lock text.
  6. Settings, Appearance: switch through the themes and both schemes; the rail, table, drawer and dialogs stay readable and
     the focus ring stays visible; enable "reduce motion" in the system and confirm nothing animates.
  7. Narrow the window below 700 px: the rail becomes a strip of hosts and the drawer overlays the table.

- [ ] **Step 4: Detect changes and commit.**

```powershell
git add ui/js/desktop/apps/AGENTS.md
npx gitnexus detect-changes --scope staged --repo AuraGo
git commit -m "docs(docker-manager): document the Docker Manager desktop app module" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
