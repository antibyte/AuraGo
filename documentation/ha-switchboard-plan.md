# HA Switchboard desktop app

Status: implemented and locally verified, 2026-09-10. Initial source review:
`db388b8df`; design plan: `fd85501b4`. No live HA device was actuated for tests.

## Goal and scope

Add the built-in desktop app `ha-switchboard`, displayed as **HA Switchboard**.
Use the existing configured Home Assistant integration to discover switches,
let the user select them, and give every selected entity its own physical-looking
silver lever. Selection survives window closure, browser changes and server restarts.

The supplied 1672 × 941 image is the visual reference: a dark walnut cabinet
with inset wooden panels, engraved plaques, substantial metal levers, screws,
indicator lamps and an instrument column. The user's explicit **silver switches**
require chrome/silver levers and mounting plates even though the reference's
hardware appears golden. The room, wallpaper and operating-system desktop around
the cabinet are outside the app. The original attachment remains in the planning
conversation; no repository copy of that image has been created.

V1 supports `switch.*`. A light exposed by HA as a switch works immediately.
Actual `light.*` and `input_boolean.*` entities are possible later additions;
climate, covers, locks and alarms require their own controls and are outside this
switch-only request. There is one shared board per AuraGo desktop installation,
matching the existing desktop settings scope.

## Visual contract

| Part | Required appearance and behavior |
| --- | --- |
| Cabinet | Deep brown walnut with visible grain, rounded thick frame, recessed seams, warm edge highlights and restrained wear. Preserve the reference's broad horizontal proportions. |
| Header | Engraved, widely spaced serif title **HA SWITCHBOARD**, smaller **HOME ASSISTANT** subtitle. Existing minimize/maximize/close controls sit in the wooden header and remain real shell controls. |
| Switch bays | Up to six tall, narrow inset wooden plates across the reference-width window. Every bay represents exactly one selected entity. Warm patinated nameplate above, silver mounting plate and long silver lever in the center, status lamp and readable state below. |
| Metal | Brushed silver mounting plate; polished chrome lever, rounded grip, visible hinge and screws. Bright reflected edges, dark reflections and contact shadows must make the hardware look three-dimensional. Brass accents may remain on plaques and cabinet trim. |
| On/off | Lever up = on; down = off. Green lamp for confirmed on, unlit amber glass for confirmed off. Pending uses amber indication and text; unavailable/error uses a distinct warning and text. Color is never the only state cue. |
| Instruments | Preserve the narrow right-hand instrument column. Its analog dial shows the actual on/selected count, with connection and unavailable counts below. Label it accordingly; missing states are explicitly unknown. |
| Footer | Broad recessed dark-glass status display and engraved metal plates in the same wooden base. Show connection state, confirmed switch totals, last refresh and a discreet **Manage switches** action. Additional rotary controls from the photo are outside v1 unless they receive a requested real function. |
| Typography | Existing local serif font selected against the reference; engraved highlights and readable cream labels. Labels remain DOM text, never baked into images. Long names wrap to two lines with a full accessible name. |

**Implemented assets:** the original generated walnut texture and three-pose
silver lever atlas live under `ui/img/ha-switchboard/`, with prompts, hashes and
provenance in its README. The atlas is RGB; window-local SVG clip paths isolate
the hardware without baking the cabinet or text into an image. CSS supplies
frame/panel details, inset shading and the short lever transition; the analog dial
uses an SVG scale and needle with a shared pivot and 240-degree sweep. Its text
stays clear of the ticks and never wraps. Plaques reuse Radio's existing metal
texture. There is no new runtime rendering library or external asset dependency.

Use semantic buttons with `role="switch"`, `aria-checked`, keyboard activation,
visible focus and at least 44 px touch targets. Pending controls cannot issue a
second command. Reduced motion skips the mechanical transition. A failed texture
load must leave a readable, functional control.

The reference acceptance window is approximately 1300 × 820 px, with six bays
and the instrument column. Clamp initial bounds to the available desktop.
At short desktop window heights, reduce chrome/lever sizes and omit the decorative
instrument plaques; keep switch states, the dial, counts and controls in view.
The browser regression checks two and six switches at 1300 x 540/620/700 without
vertical scrolling in all three themes. Additional rows may still scroll.
At narrower widths, reduce the number of columns while retaining lever size and
material depth; move instruments below the board. At 360 px, use one column.
More switches add rows in a vertically scrolling board.
Keep the cabinet skin identical in Standard and Fruity light/dark. Scope all
window styling to `.vd-window[data-app-id="ha-switchboard"]`.

## User flow

1. Open **HA Switchboard** from the desktop/start menu. If HA is disabled or
   incomplete, show the wooden empty board with **Set up Home Assistant**, linked
   to the existing integration configuration. Do not ask for another URL/token.
2. **Manage switches** opens a native modal dialog styled with the same wood and metal.
   Load the switch catalog once; search locally by friendly name or entity ID.
   Show names, IDs, availability and selection checkboxes. Search does not switch
   devices. Missing integration, failed connection, no switches and no search
   matches have separate messages and retry/setup actions.
3. The drawer holds a draft selection. **Apply** saves it; **Cancel** discards it.
   Existing order is retained and newly selected entities append. Up/down controls
   allow keyboard and touch reordering. An optional local display label changes
   only the plaque. Removing a bay removes its board selection, not the HA entity.
4. Read fresh HA states before enabling levers. Clicking a lever sends the explicit
   desired state. Indicate pending immediately; show confirmed success only after
   reading the target entity back. Errors preserve the last confirmed state and
   provide a clear retry path.
5. Refresh while the board is visible, initially every five seconds, with one
   in-flight refresh per window. Changes made in HA or by automations appear on
   the next successful refresh. Refresh immediately on returning to the window.
6. Pause refresh on hidden documents, minimization or inactive Spaces. Closing
   the window aborts requests and releases timers/listeners. A timeout never
   automatically repeats a write; subsequent reads reconcile uncertain results.

## Existing code to reuse

| Responsibility | Verified source and implementation consequence |
| --- | --- |
| HA transport and policy | `internal/tools/homeassistant.go`: `HAGetStates`, `HAGetState`, `HACallService`, bounded responses and shared HTTP client already exist. Pass the complete `HAConfig`, including read-only and allowed/blocked services. |
| Integration config | `internal/config/config_types.go`: `HomeAssistant` owns enablement, URL, `readonly`, service policy and vault-only token. Read a current config snapshot per request; no second connection configuration. |
| Connection test | `internal/server/integration_test_handlers.go` and `server_routes_config.go`: `/api/home-assistant/test` already tests saved settings. Reuse the existing Config action. |
| Existing HA poller | `internal/tools/homeassistant_poller.go` polls mission-selected entities every 15 seconds. It is not a desktop state feed; leave its mission behavior intact. |
| Desktop registration | `internal/desktop/types.go`: built-in manifest, icon allowlist and setting definitions. Register the new app and a matching packaged icon. |
| Lazy loading and lifecycle | `ui/js/desktop/core/module-loader.js`, `menus-and-routing.js`, `desktop-foundation.js`, `window-shell-runtime.js`: stylesheet/script loading, context, disposal and bounds. Expose `window.HASwitchboardApp = { render, dispose }` like existing apps. |
| Material and shell pattern | `ui/css/radio.css`, `ui/js/desktop/apps/radio.js`, `ui/img/radio/`: app-scoped wood/metal skin retaining the normal shell. Do not copy Radio's browser-only favorites storage. |
| Durable board | `internal/desktop/service_settings.go`, `internal/server/desktop_handlers_settings.go`, `ui/js/desktop/core/session-runtime.js`: validated SQLite settings, atomic updates, shell writer and `desktop_changed` events. Add one allowlisted setting. |
| Access controls | `internal/server/desktop_auth.go`, existing server middleware and `getDesktopService`: reuse authentication, desktop enablement, admin scope and CSRF protections. |
| UI resources | `assets/web-assets.json` already includes `ui/css`, `ui/js` and `ui/img`. Use version-bound local assets and regenerate the affected desktop bundle. |

No existing desktop HA entity-list or switch-action endpoint was found.

## API and persistence

Add a focused `internal/server/desktop_homeassistant_handlers.go` and register
its routes in `internal/server/server_routes.go`:

| Route | Contract |
| --- | --- |
| `GET /api/desktop/home-assistant/entities` | Config readiness plus a compact catalog of `switch.*`: ID, friendly name and state. No tokens, arbitrary attributes or raw HA error bodies. |
| `GET /api/desktop/home-assistant/states` | Current states for saved board selections, connection status, read timestamp and permitted actions. One existing `HAGetStates` call per refresh, filter before returning. Missing selected entities remain visible as missing. |
| `POST /api/desktop/home-assistant/switch` | Accept only `{entity_id, state: "on"\|"off"}` for a saved, supported board entity. Resolve the service server-side and call `HACallService`; return acceptance/error, then let state reads confirm the result. |

Require the established desktop admin permission for these integration routes;
do not give general generated-app read/write tokens access to home controls.
Check desktop and HA enablement on every request. Mutations must respect both
desktop read-only and `home_assistant.readonly`, plus service allow/block lists.
Keep tokens server-side. Validate a single bounded entity ID, reject extra
service/domain/payload input, cap request sizes and expose sanitized errors.
Escape all HA names when rendering. No LLM or new agent tool is involved.

The existing tool helpers return JSON strings: decode those into a small handler
projection and translate error results to proper HTTP responses. Reuse the
transport; do not add a second HA client or a general proxy. If implementation
requires request cancellation, add context support beneath the existing helpers
while retaining compatible wrappers for their current callers.

Store `ha_switchboard.board` as validated JSON in the existing desktop settings:
`{"version":1,"switches":[{"entity_id":"switch.example","label":"Desk"}]}`.
Array order is display order. Default to an empty selection. Validate structure,
unique `switch.*` IDs, label length, at most 60 entries and at most 16 KiB.
No device states or credentials are persisted in this setting. This is a new
setting key, not a schema migration. Save only this key through the existing
settings writer; a failed save retains the drawer draft. Reuse settings events
to refresh other windows; if another edit arrives during a draft, require a
reload/review instead of silently overwriting it.

Use `switch.turn_on` / `switch.turn_off`, never writes to `/api/states`.
The HA state-write endpoint changes HA's representation without operating a
device. Service responses can include unrelated changed entities, so confirmation
must check the selected entity. See the official
[REST API contract](https://developers.home-assistant.io/docs/api/rest/) and
[switch actions and states](https://www.home-assistant.io/integrations/switch/).

## Implementation sequence and acceptance

1. **Visual foundation:** build the cabinet and one complete silver switch with
   fixture data, then six bays plus instrument/footer sections. Compare beside
   the supplied image at reference size. Check wood grain, recess depth, silver
   reflections, hinge geometry, lamp glass, plaque typography and proportions.
   Flat pill toggles or generic dashboard cards fail visual acceptance.
2. **HA routes and selection:** add the narrow handlers, setting validation and
   search/selection drawer. Verify with a mock HA server: disabled/missing config,
   auth/CSRF/scope checks, read-only, allow/block policy, bad entity IDs, malformed
   replies, selection persistence and failed saves. Tests must not operate real
   home devices.
3. **Live controls:** wire explicit on/off, pending and confirmation, external
   state refresh, missing/unavailable devices, sanitized errors, stale response
   protection and cleanup. Include a test where a poll started before a command
   completes later; it must not overwrite the newer confirmed result.
4. **Desktop integration:** register/start/restore the app, verify the icon against
   real allowlists, test drag/resize/maximize/close and inactive Spaces, localize
   all controls and accessibility text in all 16 desktop locales. Update owning
   UI/app DOX contracts when implementation exists.
5. **Review:** add `ui/desktop_ha_switchboard_browser_test.go` using the real shell
   fixture for discovery, selection/reload, writes, policy failures, disconnects,
   lifecycle, keyboard/touch, long names, reduced motion and narrow widths. Check
   Standard and Fruity light/dark screenshots with 0, 1, 6 and 12 selected switches.
   Run focused Go checks and `node scripts/build-ui-bundles.js --check`. Validate
   version-bound asset packaging and keep real HA smoke acceptance distinct from
   mock/browser evidence. A real-device smoke test needs an explicitly designated
   test switch; discovery and planning do not authorize arbitrary test actuation.

Dashboard consideration: this is a desktop control surface. The dashboard already
reports HA integration enablement; no duplicate dashboard board is needed.

## Implemented sources and verification

- `internal/desktop/ha_switchboard.go` validates the one existing SQLite setting:
  version, bounded input, unique supported IDs, labels and a maximum of 60 bays.
  The built-in manifest, real icon allowlist and default setting are registered in
  `types.go`. Persistence tests close and reopen the real desktop database.
- `internal/server/desktop_homeassistant_handlers.go` implements the three narrow
  routes. It passes the complete current HA policy into context-aware wrappers
  around the existing transport. Tests cover auth scopes, CSRF, both read-only
  gates, service policy, unavailable devices, malformed input, sanitized failures
  and cancellation against a local mock HA server.
- `ui/js/desktop/apps/ha-switchboard.js` and `ui/css/ha-switchboard.css` implement
  the cabinet, selection dialog and live controls. Existing settings events plus
  a pre-save read detect concurrent edits; the shared settings API has no atomic
  compare-and-swap, so simultaneous writes after both preflights remain last-writer
  wins. Add conditional settings writes if multi-user editing requires that guarantee.
- `ui/desktop_ha_switchboard_browser_test.go` exercises the real lazy app loader,
  shell and artwork with simulated HA. It checks 0/1/6/12 bays, Standard and
  Fruity light/dark, narrow/touch layout, drag/resize/maximize/restore, search, selection/order/labels,
  failed saves, reload persistence, explicit writes, late poll rejection, keyboard
  operation, escaping, external changes, read-only, disconnect/recovery, draft
  conflicts, inactive Spaces, reduced motion and disposal.
- All 16 desktop locale files carry the 41 switchboard messages. Bundle freshness,
  app asset/lifecycle registration and the first-party JS line budget are checked
  alongside the focused browser test. Runtime packaging uses the unchanged
  version-bound resource contract in `documentation/web-assets.md`.

Visual test captures are local artifacts under `reports/ha-switchboard/` when
`AURAGO_BROWSER_ARTIFACT_DIR` is set. Local test acceptance does not establish
production deployment or real-device acceptance; those were not performed.

Defer HA area-registry lookup, WebSocket infrastructure, switch groups, scenes,
weather, house modes, master volume and automation editing until requested.
The initial search works with the name and entity ID already exposed by AuraGo.
