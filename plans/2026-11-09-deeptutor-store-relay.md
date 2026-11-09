# DeepTutor in the Software Store with an AuraGo LLM relay

Date: 2026-11-09. Status: design approved by the user (relay variant, official pinned image).

## Goal

Install DeepTutor (HKUDS, Apache-2.0) from the Software Store as a Docker app and have it use the
AuraGo LLM without any manual setup. The real provider key never leaves AuraGo: DeepTutor talks to a
token-gated, OpenAI-compatible relay inside AuraGo.

## Upstream facts (verified from the upstream repo on 2026-11-09, re-verify against the pulled image)

- Image `ghcr.io/hkuds/deeptutor:1.7.2` (amd64 + arm64). UI/API port **3782** (the frontend proxies `/api` and
  `/ws` to the in-container backend on 8001; 8001 is not published). Persistent state: `/app/data`.
- Docker runs are **JSON-driven**: env names such as `NEXT_PUBLIC_API_BASE`, `CORS_ORIGINS`, `AUTH_*` are unset by
  the image entrypoint. LLM, embedding and search providers live only in
  `/app/data/user/settings/model_catalog.json` (plaintext keys; the Settings UI rewrites the file).
- Entrypoint `/app/entrypoint.sh` starts supervisord as root, remaps to user `deeptutor` (PUID/PGID, default 1000)
  and runs `chown -R deeptutor:deeptutor /app/data`. Healthcheck `python /app/healthcheck.py` -> `/health/ready`,
  start period 60 s. Auth is off by default. No `X-Frame-Options`/`frame-ancestors` is sent.
- Catalog schema (`deeptutor/services/config/model_catalog.py`): top level `version` (1), `connections`,
  `services{llm,task,embedding,search,tts,stt,imagegen,videogen}`. Non-search service: `active_profile_id`,
  `active_model_id`, `profiles[]`. Profile: `id` (`<service>-profile-xxxxxxxx`), `name`, `binding`, `base_url`,
  `api_key`, `api_version`, `extra_headers`, `models[]` (llm also `api_format`, `wire_api`). Model: `id`
  (`<service>-model-xxxxxxxx`), `name`, `model`, llm `capabilities{tools,vision,json_output,reasoning}`,
  embedding `dimension`/`supported_dimensions`. Binding for any OpenAI-compatible endpoint: `custom`.
  Embedding `base_url` is the **full endpoint URL** (`.../embeddings`). A corrupt file is silently replaced by
  defaults, so every write must be valid JSON and atomic.

## Architecture

1. **Catalog entry** `deeptutor` (category `ai`, icon `book`, image pinned to `1.7.2`, primary port 3782, one named
   volume `data` at `/app/data`, `open_maximized`). No hardening opt-in (root + supervisord). No new locale keys:
   the English description is the fallback, and it states that DeepTutor has no login of its own.
2. **LLM relay** in AuraGo (`internal/server/store_llm_relay.go`), prefix `/api/store-llm/deeptutor/v1`:
   `GET /models`, `POST /chat/completions` (JSON and SSE), `POST /embeddings`.
   - Auth: `Authorization: Bearer <store token>`, checked inside the handler (path is in `authBypassSubtrees`
     like `/webhook/`). The token is a Store generated secret (`llm_relay_token`, Vault, never exposed). The
     check (`desktopstore.Service.VerifyRelayToken`) is constant-time and passes only while the app is installed.
     No cookie/session fallback, no other route accepts this token.
   - Chat goes through `s.LLMClient` (the `llm.ChatClient` with failover/provider quirks) with the request model
     replaced by the current AuraGo main model. Tools, `response_format`, `temperature`, `max_tokens`, streaming are
     passed through; responses use the go-openai wire types. Upstream errors become OpenAI-style error JSON without
     provider keys or internal URLs. Embeddings are forwarded only to an external OpenAI-compatible AuraGo embedding
     provider (404 `embeddings_unavailable` otherwise).
   - Limits: body cap, concurrency cap, request timeout, no logging of prompts or tokens.
3. **Auto configuration** via a start wrapper. `ContainerSpec` gets an optional `Entrypoint`. For `deeptutor` the
   Store sets entrypoint `/bin/sh -c 'python -c "$AURAGO_DT_MERGE_PY" ...; exec /app/entrypoint.sh "$@"'`.
   The embedded Python script merges a managed profile named `AuraGo` (fixed ids) into `model_catalog.json`:
   relay URL, token, model alias `aurago`, capabilities (tools true, vision from AuraGo `llm.multimodal`), plus an
   embedding profile when the relay reports embeddings. It activates the AuraGo profile only when the service has
   no active profile, keeps every other profile and file content, writes atomically, and records a fingerprint
   (url|token|flags) in `/app/data/user/settings/.aurago-managed.json` so a restart does not undo a user's
   deletion or choice. A changed fingerprint (new relay URL/token after a Store update) re-applies the connection
   data only. A script failure logs to stderr and still starts DeepTutor.
4. **Reachability**: `desktopstore.LLMRelayProvider` (implemented by the server) returns the container-reachable
   AuraGo origin: shared Docker network -> `http://<aurago container hostname>:<port>`; otherwise
   `http://host.docker.internal:<port>` plus `ExtraHosts host.docker.internal:host-gateway`. On Linux with AuraGo
   bound to loopback only, or with an HTTPS-only listener, install fails early with a clear message.
5. **Security notes**: relay token is the only credential in the container (visible as env and in the catalog
   file inside the data volume); LAN bind exposes an unauthenticated DeepTutor that can spend the AuraGo LLM
   (documented, shown in the description). Uninstall without `delete_data` keeps the Vault token but the relay
   refuses it because the app is gone. Rotate by `delete_data` uninstall.

## Contracts shared by both tracks

```go
// package desktopstore (internal/desktopstore/deeptutor.go)
const DeepTutorAppID = "deeptutor"
const DeepTutorRelayPath = "/api/store-llm/deeptutor/v1"
type RelayInfo struct {
    BaseURL             string   // container-reachable AuraGo origin, no trailing slash, e.g. http://host.docker.internal:8088
    ExtraHosts          []string // e.g. {"host.docker.internal:host-gateway"}
    Vision              bool
    EmbeddingsAvailable bool
    ContextWindow       int      // 0 = unknown
}
type LLMRelayProvider interface {
    RelayInfo(ctx context.Context, networkMode string) (RelayInfo, error)
}
// Config gets: LLMRelay LLMRelayProvider
func (s *Service) VerifyRelayToken(ctx context.Context, appID, token string) bool
```

The server package depends only on `VerifyRelayToken` (through a one-method interface) and implements
`LLMRelayProvider`.

## Tasks

Track A (desktopstore, one batch): A1 `ContainerSpec.Entrypoint` + `dockerCreatePayload`; A2 `deeptutor.go`
(constants, interface, env resolution in `resolveEnv`/`runtimeContainerSpec`/update path, token secret,
`VerifyRelayToken`), embedded merge script (`deeptutor_assets/aurago_catalog_merge.py`, `//go:embed`); A3 catalog
entry; A4 tests + `internal/desktopstore/AGENTS.md` contract + `documentation/deeptutor.md`.

Track B (server, one batch): B1 relay handlers + auth bypass + limits; B2 `LLMRelayProvider` implementation and
wiring in `getDesktopStoreService`; B3 tests (auth, streaming, model override, tools passthrough, embeddings
unavailable, token revoked after uninstall, bypass does not widen).

Track C (live, after A+B): pull image, run with the wrapper on local Docker Desktop, confirm DeepTutor accepts the
merged catalog (settings API shows the AuraGo profile active), then one chat call through a relay harness.

## Acceptance

- `go test ./internal/desktopstore ./internal/server -run 'DeepTutor|StoreLLMRelay|Catalog'` plus the audit and
  security lint packages pass; the full `internal/desktopstore` suite passes.
- Live: merged catalog accepted by the real image; restart keeps a user-changed active profile; a chat request made
  by DeepTutor reaches the relay with the store token and the real key never appears in the container.
- Not claimed: arm64 run, Linux host-gateway behavior, real provider traffic.
