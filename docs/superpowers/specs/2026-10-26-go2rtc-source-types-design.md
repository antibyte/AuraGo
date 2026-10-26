# go2rtc source types for Network Cameras and the config page — design

Date: 2026-10-26
Status: approved in brainstorming, awaiting spec review

## Goal

Network Cameras (desktop app) and the go2rtc section of the config page can
configure every go2rtc source type that only opens network connections —
Tuya first, plus Tapo, Kasa, RTMP, DVRIP, Ring, Nest and the rest — through a
type picker with guided forms. Today both surfaces accept only
`rtsp`, `rtsps`, `rtspx`, `http`, `https` and `onvif`
(`ValidateGo2RTCSource`, `internal/tools/go2rtc.go:852`).

## Non-goals

- Sources that execute programs or touch the host: `exec:`, `echo:`, `expr:`,
  `ffmpeg:device?…`, `device:`, local paths and `file:` stay blocked.
- Proxying go2rtc's own discovery/login helpers (`/api/tuya`, `/api/xiaomi`,
  HomeKit pairing). Users take device IDs or paired URLs from the vendor app
  or the go2rtc web UI (already reachable through the admin proxy when
  `web_ui_enabled` is on).
- Re-showing stored source details. Sources stay vault-only; editing a camera
  means keeping the source or entering a complete new one, as today.
- An agent tool for adding cameras.

## Constraints from the existing code

- Sources are vault-only (`Go2RTCStreamConfig.Source` is `yaml:"-" json:"-"`)
  and masked in every response; tests assert that no source fragment leaks.
- Three server paths validate sources: `validateGo2RTCSettings`
  (`internal/server/config_handlers_main.go:1898`), `handleGo2RTCCreateStream`
  and `handleGo2RTCStreamMutation` (`internal/server/go2rtc_app_handlers.go`),
  plus the runtime patch in `internal/tools/go2rtc.go:454`. All four keep
  calling `ValidateGo2RTCSource`; only its rules change.
- The bundled image is `alexxit/go2rtc:1.9.14`, which supports every type below.
- The managed sidecar has normal outbound network access, so cloud types
  (Tuya, Ring, Nest, …) work without Docker changes.

## Architecture (approach A)

```
internal/tools/go2rtc_source_types.go   catalog + per-type validation (source of truth)
        │
        ├── ValidateGo2RTCSource(raw)    → looks the scheme up in the catalog
        ├── Go2RTCSourceTypeOf(raw)      → non-secret type id ("tuya", "rtsp", …)
        └── GET /api/go2rtc/source-types → form description, no secrets
                │
ui/js/shared/go2rtc-source-form.js      type picker + form + URL builder
        ├── Network Cameras setup ("Camera type" method) and manage dialog
        └── config page camera cards (ui/cfg/go2rtc.js)
```

The browser builds the URL; the server validates it exactly as it does today.
The vault, the save endpoints and the masking stay unchanged.

### 1. Catalog (`internal/tools/go2rtc_source_types.go`)

One entry per type:

| Field | Meaning |
|---|---|
| `ID` | stable id, e.g. `tuya`, `tapo`, `rtsp` |
| `Schemes` | accepted URL schemes (`rtsp` entry also lists `rtsps`, `rtspx`) |
| `LabelKey`, `HelpKey` | i18n keys |
| `DocsURL` | go2rtc README anchor for the type |
| `Mode` | `form` (structured fields) or `url` (paste URL with template) |
| `Variants` | optional named field sets (Tuya: `smart`, `cloud`) |
| `Fields` | ordered field specs (see below) |
| `Template` | example URL shown for `url` mode and as a hint |
| `Validate` | per-type check on the parsed URL |

Field spec: `Name`, `LabelKey`, `Kind` (`text`, `host`, `port`, `secret`,
`select`, `number`), `Required`, `Options` (for `select`, value + label key),
`Default`, and `Placement` — where the value goes in the URL: `userinfo.user`,
`userinfo.password`, `host`, `port`, `path`, `query:<name>`. Secrets are
always `Kind: secret`, so the form uses password inputs.

Types and how they are entered:

| Type | Mode | Form fields / rule |
|---|---|---|
| `rtsp` (`rtsp`, `rtsps`, `rtspx`) | form | host, port, user, password, path |
| `http` (`http`, `https`) | form | URL kind (JPEG/MJPEG/FLV/TS hint only), host, port, user, password, path |
| `onvif` | form | host, port, user, password (existing discovery/address flows remain) |
| `rtmp` | form | host, port, path |
| `tuya` | form, variants | **smart:** region (EU Central `protect-eu`, EU East `protect-we`, US West `protect-us`, US East `protect-ue`, China `protect`, India `protect-in` — all `.ismartlife.me`), device ID, email, password, resolution `hd`/`sd`. **cloud:** region (`openapi.tuyaus.com`, `openapi.tuyaeu.com`, `openapi.tuyacn.com`, `openapi.tuyain.com`, `openapi-ueaz.tuyaus.com`, `openapi-weaz.tuyaeu.com`), device ID, UID, client ID, client secret, resolution |
| `tapo` | form | host, cloud password (or `admin` + hash as advanced user/password), `subtype` |
| `kasa` | form | host, port (default 19443), email, password; path fixed `/https/stream/mixed` |
| `dvrip` | form | host, port (default 34567), user, password, channel, subtype |
| `bubble` | form | host, port (default 34567), user, password, channel, stream |
| `isapi` | form | host, port, user, password (two-way audio only, help says so) |
| `eseecloud` | form | host, port, user, password, path |
| `ring` | form | device ID, refresh token (secret) |
| `nest` | form | client ID, client secret, refresh token, project ID, device ID |
| `hass` | form | Home Assistant host + port, entity ID, token; the host-less `hass:Camera1` form is rejected (AuraGo's sidecar is not a HA add-on) |
| `homekit`, `roborock`, `wyze`, `xiaomi`, `doorbird`, `gopro`, `ivideon`, `webrtc`, `webtorrent` | url | paste URL with template + docs link |
| `ffmpeg` wrapper | form | inner source (any allowed type above) + transcode options, see §2 |

Per-type `Validate` rules:

- Host-based types require a non-empty host and no `#raw`/`#exec` style
  fragment keys (fragments are allowed only from a per-type allowlist, e.g.
  `backchannel`, `timeout`, `media` for RTSP; `header` for HTTP).
- `tuya` requires the host to be one of the listed region hosts and the query
  to carry the variant's required keys.
- `ring`, `nest`, `webtorrent` are host-less (`ring:?…`) and require their
  query keys.
- `webrtc` requires an inner `http`, `https`, `ws` or `wss` URL with a host.
- `ivideon` requires the opaque `ivideon:<id>` form.
- Anything not in the catalog is rejected with
  `source scheme "x" is not allowed`, as today.

### 2. `ffmpeg:` transcoding wrapper

`ffmpeg:<inner>#opt=…` is allowed only when `<inner>` passes
`ValidateGo2RTCSource` itself and is not another `ffmpeg:`. Allowed options:
`video`, `audio`, `rotate`, `width`, `height`, `hardware`. Every other
option — notably `raw`, `input`, `output`, `device`, `drawtext` — is rejected, because `#raw=` passes arbitrary FFmpeg arguments
(local file reads, extra outputs) and `#input=` selects an FFmpeg template.
Option values must match `^[a-z0-9_.,=-]+$`.

In the forms, transcoding is an "Advanced: transcode with FFmpeg" panel under
any `form`-mode type (video codec `copy`/`h264`/`h265`/`mjpeg`, audio
`copy`/`opus`/`aac`/`pcmu`/none, rotate 0/90/180/270, hardware on/off).

### 3. Non-secret type per camera

`Go2RTCStreamConfig` gains `SourceType string \`yaml:"-" json:"source_type"\``,
computed next to `SourceConfigured` when the vault is hydrated, via
`Go2RTCSourceTypeOf(source)` (for an `ffmpeg:` wrapper: the inner type, plus
`source_transcoded: true`). Nothing is written to config.yaml; existing
cameras get their type for free. The value is only the type id, never host,
user or query. Both UIs show it as a small badge ("Tuya", "RTSP", …).

### 4. Catalog endpoint

`GET /api/go2rtc/source-types` returns the catalog without `Validate`:
ids, schemes, label/help keys, docs URLs, modes, variants, fields, templates,
ffmpeg option lists. Static data, no secrets, `Cache-Control: private,
max-age=300`. Auth: the same middleware as the existing camera-management
routes (the plan pins the exact one); the config page and the desktop app
both call it.

### 5. Shared form module (`ui/js/shared/go2rtc-source-form.js`)

Pure functions plus a small binder, no framework:

- `markup(catalog, state, t, esc)` — type select, variant radios, fields,
  advanced FFmpeg panel, template/docs hint for `url` mode.
- `buildURL(catalog, state)` → `{ url }` or `{ error: '<i18n key>' }`; does
  the encoding (`encodeURIComponent` for userinfo and query values, IPv6
  hosts in brackets).
- `bind(root, state, onChange)` — wires inputs; keeps secrets only in memory,
  never in `localStorage`.
- "Custom URL" is always the last entry in the type select and posts the raw
  text, which the server validates.

The module is plain enough to unit-test in Node `vm` like the existing
`test-ui-regressions.mjs` contracts.

### 6. Network Cameras app

- Setup step 1 gets a fourth method, **Camera type**, next to Discover,
  Address and URL. Step 2 shows the shared form; step 4 (name/ID) is
  unchanged. The `url` method stays as the quick paste path.
- The manage dialog's "Replace source" password field becomes a
  "Replace source…" button that expands the shared form; empty means keep.
  The existing replace confirmation stays.
- The camera list shows the `source_type` badge.
- The module loader entry for `network-cameras` loads the shared module
  before `network-cameras.js`; `main.bundle.js` is rebuilt if the hook
  changes.

### 7. Config page (`ui/cfg/go2rtc.js`)

Each camera card replaces the single `go2rtcSecret(base + '.source', …)`
with: type badge (when configured) + "Replace source…" toggle that renders
the shared form. On change it writes the built URL into
`go2rtc.streams.N.source` exactly where the secret field wrote before, so
the vault masking and save path stay unchanged. A build error shows inline
and blocks writing the value.

### 8. i18n

New keys under `config.go2rtc.source_*` and the Network Cameras app
namespace for type names, field labels, help, region names and errors, in
all 16 locales.

## Error handling

- Client: missing required field or invalid port → inline message per field;
  nothing is written.
- Server: unchanged response shapes; messages name the type and the rule
  (`tuya source needs device_id`, `ffmpeg option "raw" is not allowed`),
  never echo the URL.
- A type the running go2rtc rejects later still surfaces through the
  existing runtime status/snapshot errors.

## Testing

- Go table tests in `internal/tools/go2rtc_source_types_test.go`: one valid
  and at least one invalid URL per type; `ffmpeg:` with `#raw`, `#input`,
  nested `ffmpeg:`, `exec:` inner; `hass:Camera1`; Tuya with a foreign host;
  existing `TestValidateGo2RTCSourceAllowsOnlyNetworkSources` updated
  (`ffmpeg:rtsp://camera.local/live` moves to allowed).
- `Go2RTCSourceTypeOf` tests, and a handler test that the catalog endpoint
  and app state contain `source_type` but no source text.
- JS: `buildURL` per type (Tuya both variants and regions, encoding of
  `@`/`:` in passwords, IPv6), `test-ui-regressions.mjs` contracts for the
  new setup method, manage dialog and config card.
- Manual: add a Tuya camera on the local scratch instance and on
  aurago-test (user's Tuya account), snapshot + live view.

## Open points for the plan

- Exact auth middleware for the catalog route.
- Exact allowed fragment keys per host-based type, checked against go2rtc
  1.9.14 source.
