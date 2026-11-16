# Virtual Desktop everyday workflow roadmap

Status: implementation in `codex/desktop-shell-everyday`, based on committed
`6d753d4e1127447f4e953dc9a71244891648195a`. The first delivery implements complete
snap/guarded window actions, shared search/default apps, browser-local notification
policy/history and shell text size/high contrast. Wave 1 passed focused Node, Go,
browser, encoding and deterministic bundle checks locally on Windows, including
snap/session restoration and notification storage boundaries. Wave 2 implements
shared file history, Quick settings, a month calendar and media observation.
Wave 2 passed focused Node and Windows Go tests, integrated browser flows,
encoding and deterministic bundle checks, plus Linux/macOS compilation. Browser
acceptance includes partial/disposed-window moves, cross-window history, late
responses after logout, month navigation/retry, and constrained/forced-color views.
Workspace personalization, session controls and grouped
window switching remain pending. Full Linux CI and real assistive-technology
acceptance are separate.

Improve frequent desktop interactions by extending the existing shell. Preserve the
distinct Standard taskbar and Fruity dock, the vanilla JavaScript architecture,
server permissions, application close guards, and existing session data. Ship
small vertical slices; operating-system feature parity is not the acceptance goal.

## Baseline before implementation (2026-11-15)

- Snap already supports left/right halves and upper quarters. `top-half` geometry
  exists but has no user entry point. The lower quarters are absent.
- Standard has separate pins and window buttons. Fruity already deduplicates app
  icons, marks running apps, and collapses to its launcher when windows overlap it.
- Taskbar and Spaces overview share `windowPreviewMarkup`. Ctrl/Meta+Tab has only
  icons and titles. Its current candidate filter also excludes minimized windows.
- Start search matches app names; its Recent category already includes files.
  Spotlight searches apps/files but indexes only five settings categories, while
  Settings has eight categories and its own row search.
- Notification history is an in-memory list capped at 40. Items can open an app
  with context. Opening the center marks all items read; separate actions and
  per-app delivery controls are absent.
- `AuraDesktopMediaSession` already arbitrates media ownership and OS controls.
  `sound.volume` controls shell effects, not all application audio or device volume.
- Logout exists in the shared radial menu when authentication is enabled. A
  screensaver is present; a protected lock workflow is not.
- File Manager already has local move/rename undo and redo. Shell trash helpers
  return actual moves and handle restore conflicts. Desktop operations do not
  participate in a shared undo history.
- Settings persistence, default file associations, fixed three-space sessions,
  per-space built-in wallpapers, reduced motion, and 16 locales already exist.

The dated claim-by-claim review belongs in ignored
`reports/desktop-shell-review-20261115/claims.md`; it is evidence, not a runtime contract.

## Delivery order

Effort is relative: S = localized UI work; M = several coordinated modules;
L = shared state, backend contracts, or migration. These are scope estimates,
not elapsed-time promises. Accessibility requirements apply to every delivery.

| Order | Deliverable | Priority / effort | Depends on |
| --- | --- | --- | --- |
| 1 | Complete snap and safe window actions | P1 / M | Existing window manager |
| 2 | Unified search and default-app settings | P1 / M | Existing settings/search APIs |
| 3a | Notification policy and reload recovery | P1 / M | Existing notification dispatch |
| 3b | Quick settings, month calendar and now playing | P1 / M | 3a for Do Not Disturb |
| 4 | Shared file-operation undo | P1 / L | Rooted file/trash services |
| 5 | Text size and high contrast | P1 / M | Existing theme tokens; may ship earlier |
| 6 | Grouped taskbar and preview switcher | P2 / M | 1 close guards and preview work |
| 7a | Custom wallpaper | P2 / M | Existing file picker and authorized reads |
| 7b | Editable workspaces | P2 / L | Session/settings migration |
| 8 | Discoverable session actions | P2 / S | Existing authentication/logout |

Recommended first release: 1, 2, 3a and 5. Follow with 3b and 4, then 6–8.
Do not make the first release wait for workspace migration, capture permissions,
or a new authentication state machine.

## 1. Complete snap and safe window actions

Owners: [window interactions](../ui/js/desktop/core/window-interactions-runtime.js),
[window shell](../ui/js/desktop/core/window-shell-runtime.js),
[menus](../ui/js/desktop/core/menus-and-routing.js),
[session runtime](../ui/js/desktop/core/session-runtime.js), window CSS.

- Put rectangle calculation in one small helper used by overlay and placement.
  Add lower-left/lower-right quarters and expose the existing upper half.
  Keep top-edge maximization; expose the upper half through the layout menu.
  Offer lower half in the same menu for a complete horizontal pair.
- Add a keyboard-accessible layout menu on the maximize control, available by
  explicit context menu/long press as well as optional delayed hover. A normal
  click still maximizes/restores. Disable layouts that cannot fit an app's minimum
  size instead of presenting overlapping quarters as an exact layout.
- Add directional snap transitions: left/right establishes a half, then up/down
  chooses its upper/lower quarter. Keep a visible Restore action. Save the original
  unsnapped bounds once across a chain of snap changes and preserve them on resize.
- Add “Close other windows” for the current workspace and “Close all windows of
  this app” with an explicit all-workspace scope. Snapshot targets and await each
  existing asynchronous close guard sequentially; stop on cancellation. Never
  bypass dirty-editor prompts or close newly created windows.
- After the basic layouts pass, offer a dismissible partner chooser for a free
  half, using existing window previews. Selecting a candidate explicitly applies
  the second snap; no window is rearranged just by hovering.

Acceptance: every offered rectangle matches its preview; repeated snap/restore
returns to the original geometry; maximized windows clear the bar; narrow screens
keep reachable controls; cancellation leaves the remaining batch open. Test actual
keyboard delivery in browser/fullscreen as well as handler logic: Ctrl+Tab and
Meta+Arrow can be intercepted by the browser or host OS, so menus remain complete.

## 2. Unified search and default-app settings

Owners: [Spotlight](../ui/js/desktop/core/spotlight-runtime.js), Start rendering in
[window shell](../ui/js/desktop/core/window-shell-runtime.js),
[Settings](../ui/js/desktop/apps/settings.js),
[search HTTP handler](../internal/server/desktop_handlers_search.go),
[file search](../internal/desktop/service_files.go),
[file associations](../ui/js/desktop/core/menus-and-routing.js).

- Share the existing app/file result builders between Start and Spotlight;
  retain two presentations and one cancellation/generation mechanism per open
  surface. Show apps/settings immediately while file results are pending.
- Extract lightweight settings descriptors from `settingsSections` so search can
  list every visible setting without mounting Settings. Selecting a result opens
  the correct category and focuses its row. Respect feature/theme availability.
- Add a fixed list of shell actions such as opening Trash, showing Spaces and
  emptying Trash. Route each through the existing action and permission checks;
  destructive actions keep confirmation. Search itself performs no mutation.
- Add type, modified-date and size filters to the existing search route with
  backward-compatible optional parameters. Filter on the server before limiting
  results. Add cancellation, a global result/traversal budget, and a truthful
  truncated-results indicator; the current `WalkDir`/`SkipDir` pattern is not a
  reliable global bound. Do not add a separate search index initially.
- Add “Default apps” under Files: show existing extension associations, change or
  reset individual entries using `files.default_apps`, and display unavailable apps
  without silently discarding their mappings. Reuse Open With's app eligibility.
- Calculator results are a small follow-up: share the existing pure calculator
  parser, never `eval`. Begin with arithmetic. General unit conversion can wait;
  assess ECalc's existing unit support before adding another engine.

Acceptance: equivalent queries produce the same results in both entry points;
late requests never replace a newer query; filters work beyond the first result
page; read-only mode cannot execute writes; invalid associations cannot be saved;
every setting result focuses a usable control in all 16 locales.

## 3. Notifications and quick access

Owners: [shell chrome](../ui/js/desktop/core/shell-chrome-runtime.js),
[notification dispatch](../ui/js/desktop/core/sdk-events-bootstrap.js),
[sound](../ui/js/desktop/core/sound-runtime.js),
[media ownership](../ui/js/desktop/core/media-session-runtime.js), Settings,
[calendar helpers](../ui/js/desktop/apps/calendar-views.js), desktop settings service.

### 3a. Notification policy and reload recovery

- Add Do Not Disturb and per-app toast/sound controls at the common dispatch
  point, before sound, bell animation and toast creation. Keep muted notifications
  in the center. An app cannot grant itself an urgent bypass; call prompts retain
  their separate existing workflow.
- Keep the 40-item limit initially; increasing it is not a feature by itself.
  Derive unread count from retained records. Mark an item read on activation and
  offer explicit “Mark all read”; opening the panel alone changes nothing.
- Persist a versioned, bounded history in this tab's `sessionStorage`, surviving
  reload without introducing a server-wide notification database. Preserve only
  safe titles/previews and validated app/context identifiers; sensitive sources
  can opt out. Purge on logout/session expiry and reject records from a different
  authenticated session using a non-secret scope identifier. With auth disabled,
  state remains local to this browser tab. Storage denial falls back to memory.
- This first slice does not promise history after tab/browser closure or on
  another device. Durable cross-session history needs a separate product/privacy
  decision. Do not persist executable callbacks or notification HTML.
- Add a small set of typed actions, beginning with Open and Dismiss. Destructive
  domain actions use the app's normal confirmation/authorization flow. Retain
  WebSocket/SSE deduplication and bounded payload validation.

Acceptance: reload retains the tab's eligible history; repeated WS/SSE events
produce one entry; DND suppresses sound/toasts while history grows; app muting is
independent; malformed storage is discarded; logout removes the history; an expired
action cannot mutate an app or reopen protected content.

### 3b. Quick settings, month calendar and media

- Add one tray panel for shell-sound volume, mute, DND, animation preference,
  supported theme light/dark mode, and a link to Settings. Reuse the serialized
  settings writer and show failures. Label the slider “Desktop sounds”; a device
  master-volume slider would be misleading.
- Extend the clock popup with a navigable month and selected-day agenda using
  Calendar's date/month helpers and existing appointments API. Keep the calendar
  app responsible for editing. Handle local dates, week starts, midnight rollover
  and request races consistently.
- Expose a small subscribe/snapshot/action surface from `AuraDesktopMediaSession`
  and render the currently selected owner's title and supported transport controls.
  Emit snapshots even when the browser lacks `navigator.mediaSession`. Reuse the
  current priority/order arbitration; no second player or audio decoder.

Acceptance: panel actions and Settings stay synchronized; keyboard focus returns
to the opener; media follows the active owner and disappears on release; unsupported
controls are absent; the calendar can navigate across year boundaries and DST dates.

## 4. Shared file-operation undo

Owners: [shell file actions](../ui/js/desktop/core/menus-and-routing.js),
[desktop drops](../ui/js/desktop/core/desktop-file-drops.js), File Manager
`actions-input.js` / `actions-operations.js`,
[trash service](../internal/desktop/service_trash.go), rooted move/file handlers.

- Introduce one bounded in-memory file-operation history shared by Desktop and
  File Manager, reusing existing move, trash-restore and conflict helpers. Replace
  the current per-window move/rename stack rather than maintaining competing lists.
- Record only server-confirmed moves/renames and moves to Trash, with actual
  source/destination paths and identity/version evidence. Keep existing redo for
  supported operations. Permanent delete and Empty Trash remain irreversible.
- Use the dedicated Trash API for deletion undo, including Notes paths. Generic
  reverse moves must not bypass Notes protection. If existing responses cannot
  identify the moved entry reliably, add narrow receipts/preconditions to the
  service before exposing shared undo.
- Refuse stale-source or occupied-destination reversal; offer the existing conflict
  UI where appropriate, never overwrite silently. Keep uncompleted batch members
  available after partial failure; do not retry an uncertain mutation automatically.
- Offer a toast Undo button and Ctrl+Z only when Desktop/File Manager owns the
  action. Editor text undo, iframe shortcuts and dialogs retain their own scope.

Acceptance: delete/undo preserves original nested paths and contents; move/undo
works across Desktop and two File Manager windows; competing edits, collisions,
read-only changes, Notes, cancellation and partial failure cannot lose data.
Reload intentionally clears operation history; Trash restore remains available.

## 5. Accessibility

Owners: theme tokens and desktop CSS, Settings, settings definitions, shell
keyboard handlers, existing app responsive styles.

- Add bounded text-size choices and high-contrast presentation using shared
  tokens. Keep content scrolling and window controls reachable at enlarged text
  and 200% browser zoom. Do not scale the whole desktop with a CSS transform.
- Respect forced colors and reduced motion. Use semantic controls, visible focus,
  proper accessible names, focus return/trapping and live status for every new panel.
- Keep the existing shortcut help current and list browser/OS conflicts. Defer a
  shortcut editor until a single validated command map exists; reject conflicting
  mappings and preserve text-editing keys when that feature is added.
- Refer users to browser/OS magnification and on-screen keyboard initially.
  A second simulated keyboard or screen magnifier is not part of this roadmap.

Acceptance: keyboard-only operation of each delivered flow, high-contrast/forced-
colors checks, reduced-motion checks, long translations, 200% zoom, and Standard/
Fruity phone/tablet/desktop smoke coverage. Do not claim all embedded apps support
new text-size tokens until their own views have been verified.

## 6. Grouped taskbar and preview switcher

Owners: window-shell reconciliation, taskbar thumbnails, shell-chrome switcher,
Spaces visibility, session pins and shared menus.

- Give Standard one app entry combining pinned and running state, with a running
  count/indicator. Preserve ordered pins and existing launcher behavior. Fruity's
  existing grouping remains; add access to all of an app's open windows.
- Reuse one window-list/preview component for app groups, the switcher and snap
  assistance. Include minimized windows; activate them through `focusWindow`.
  Offer current-workspace/all-workspaces scope with workspace labels.
- Add close buttons through the same guarded close path as delivery 1. Preview
  controls sit outside inert preview content. Preserve fallback cards for iframe,
  canvas/WebGL or sensitive windows; do not promise live screenshots of them.
- Audit preview cloning for duplicate IDs, live media and sensitive form state
  before reusing it more widely. Generate previews on demand with bounded work;
  never poll screenshots for every background window.
- Start app context menus with Open, New window, window list, Pin and Close app.
  Add recent-file jump entries only when reliable per-app ownership is available.
  Generic progress/badge APIs wait until a real producer is integrated end to end.

Acceptance: three windows of one app produce one app entry; pin order survives
restart; minimized/other-space windows are reachable; cancelled closes preserve
the preview; closed windows release listeners; iframe fallbacks stay usable.

## 7. Personal workspaces

### 7a. Custom wallpaper

- Reuse the desktop file picker to select a workspace image and store a validated
  rooted path, not an arbitrary remote URL or a base64 image in settings. Resolve
  the current authorized media URL when rendering; do not persist expiring tokens.
- Preserve existing wallpaper IDs and per-space overrides. Validate file type,
  decoded dimensions and byte limits; fall back safely when a file disappears or
  access is revoked. Offer Fit/Fill and keep labels readable.
- Defer slideshows until custom images work reliably. A later bounded playlist
  pauses on hidden tabs, reduced motion and covered wallpapers, reusing existing
  wallpaper visibility logic.

### 7b. Editable workspaces

- Replace `SPACE_IDS` with a validated ordered list of stable IDs and names.
  Start with the existing three and bound the list to 1–8. Add, rename, reorder
  and delete through the current overview, with keyboard equivalents to dragging.
- Update `normalizeSpaceId`, pager, overview, move menus, shortcuts, wallpaper
  mapping, settings validation and session restore together. Preserve IDs `1`,
  `2`, `3` while importing old state; order changes never change IDs.
- Deleting a workspace moves its windows to an explicit remaining destination;
  it never closes apps or deletes files. Stage and atomically save the new list,
  reassignment and wallpaper map; reject stale concurrent updates.
- Compact mode keeps its existing single-surface behavior and logical assignments.
  Defer hot corners and workspace swipes; current touch swipes already serve bar
  navigation, and app scroll/drag gestures must keep ownership.

Owners: spaces runtime/overview, session runtime, menus, wallpaper rendering,
Settings, `internal/desktop/types.go` and `service_settings.go`.
Acceptance: old sessions migrate without disappearing windows; removing an active
space preserves drafts/focus; names survive reorder/reload; corrupt or stale state
fails safely; custom images survive reload and missing-image fallback is usable.

## 8. Session controls and explicit deferrals

- Put the existing logout action in a discoverable Start session menu when login
  protection is enabled. Reuse `performLogout`; resolve dirty-window guards before
  navigation. Display only identity actually supplied by authentication; otherwise
  use an honest “Local session” label rather than inventing a user profile.
- A protected lock is a separate auth project, not a visual overlay: define
  revocation/reauthentication for HTTP, open WebSockets/SSE, generated-app bridges,
  other tabs, cached content and drafts. With authentication disabled, do not offer
  a purported secure lock. The screensaver remains decorative until that work is
  complete. Prefer existing logout for the first secure session-ending action.

| Deferred feature | Decision |
| --- | --- |
| Adjustable taskbar | Add size and explicit always-visible/auto-hide settings after geometry work; first preserve existing layout defaults. Arbitrary edge placement requires workspace bounds, popovers, gestures and all theme layouts to move together and is lower priority. |
| Night light | Existing theme light/dark modes can enter Quick settings. An OS blue-light filter cannot be implemented by a browser shell; defer a shell-only warm overlay, which would also distort creative work. |
| Clipboard history | Optional later, default off, bounded in-memory text captured from explicit AuraGo copy/paste or an explicit import button. Never poll the OS clipboard or retain password/Tresor content. Keep Win+V owned by the host; add a visible shell entry point. |
| Screenshot/snipping | Optional later through feature-detected `getDisplayMedia`, an explicit browser source chooser, one captured frame and local crop/export. Stop every track on success, cancel or failure. Reuse the file saver and offer Pixel handoff. No silent capture, no general screen recording in this slice. |
| Jump lists, progress, badges | Add only with actual app-level recent-item/status producers; no speculative SDK framework. |
| Power menu | No browser-host shutdown/restart/sleep controls. AuraGo server operations belong in existing authenticated administration, clearly separate from the client desktop. |
| Wi-Fi, battery, client Bluetooth | Keep device controls with the client OS. AuraGo already has a separate app for the server's Bluetooth adapter; do not conflate it with browser-client hardware. |
| Multiple physical monitors | Excluded from the single-page shell. An optional multi-browser-window project would need permissions, browser support and cross-window state ownership. |

Browser boundaries: [screen capture](https://developer.mozilla.org/en-US/docs/Web/API/MediaDevices/getDisplayMedia)
requires a secure context, user activation and a fresh chooser; the
[Clipboard API](https://developer.mozilla.org/en-US/docs/Web/API/Clipboard_API)
has browser-dependent permission/activation rules. The
[Window Management API](https://developer.mozilla.org/en-US/docs/Web/API/Window_Management_API)
can position separate browser windows on screens, but does not give DOM windows
inside one page native multi-monitor placement.

## Integration and verification gates

- Read the current root and owning DOX chains before each implementation slice.
  UI ownership stays in `ui/AGENTS.md` and `ui/js/desktop/apps/AGENTS.md`; backend
  settings/files in `internal/desktop/AGENTS.md`; auth/API changes additionally
  require server/security owners. Update actual contracts when implemented.
- Add persisted settings to `DesktopSettingDefinitions` and validation together.
  Its upstream users include defaults, settings reads/writes, bootstrap, UI and
  agent tools. Never introduce frontend-only persistent values rejected by the
  server. New agent operations require manuals, strict schema and training parity;
  UI-only additions do not automatically need new tools.
- Use existing settings/data stores. Backward-compatible optional keys usually
  need no database schema migration. Any necessary schema migration still needs
  the repository's backup and existing-data acceptance workflow.
- Keep new shell parts at function boundaries, register them in
  `scripts/build-ui-bundles.js`, regenerate the affected bundles, and check asset
  packaging inputs. No new UI framework or dependency is planned.
- Translate all visible strings in all 16 locales and update F1 help/operator
  documentation. Validate permissions, disabled/error states and cleanup paths.
- Run syntax/encoding/diff checks, focused Node/Go tests and `npm run build:ui --
  --check`. Existing useful checks include `TestDesktopWindowTopSnapRequiresSideProximity`,
  `TestDesktopWindowSnapKeyboardUsesMetaArrows`, `TestDesktopShellChromeAndSpotlight`,
  `TestDesktopSessionRestoreSettingsAndRuntime`, `TestDesktopContextMenuAndClipboardAssets`
  and `TestDesktopSoundSettingsUIMarkers`; update obsolete assertions deliberately.
- For implemented flows run opt-in browser tests with
  `AURAGO_RUN_BROWSER_SMOKE=1`, including `TestDesktopAuditShellBrowser`,
  `TestDesktopSessionRestoreBrowser`, mobile and maximized-window regressions.
  Add focused behavioral cases for the acceptance criteria above. Synthetic key
  events do not replace a manual check of browser-reserved shortcuts.
- Release only after applicable Security Gates and version-bound resource checks.
  Capture/clipboard permission dialogs, real assistive technology and browser/OS
  shortcut conflicts require explicit real-client acceptance beyond local fixtures.

Implementation updates the owning UI, Desktop service, server and script guides
with the shipped contracts. The root guide retains responsibility for this plan;
its global ownership and permission rules are unchanged. Verification evidence and
remaining acceptance checks are recorded in ignored `reports/desktop-shell-everyday/`.
