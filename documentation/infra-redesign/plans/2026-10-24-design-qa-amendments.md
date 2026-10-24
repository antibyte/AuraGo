# Infra redesign: design QA amendments after Plan B

Source: review of the Plan B screenshots (1500×970, 1000×700, 430×700, Standard dark) on 2026-10-24.
These amendments are binding for Plan C and Plan D. Plan C owns A6 and A7 (inspector and host badge copy);
Plan D owns the rest and implements them as its first fix batch (Task 11 "D-fix-1"), before the general visual QA
of Task 10 runs, so the QA screenshots already show them.

| id | finding | required change | acceptance |
|---|---|---|---|
| A1 | At fit zoom on small windows (26–50 %) cards, labels and region headers are unreadable. | Semantic zoom. Below zoom 0.65 cards render in a compact form (icon 28 px + name only, no meta line, status ring kept). Card name, region header and lane labels are counter-scaled so their on-screen size never drops below 12 px (name) and 13 px (region header); above zoom 1.0 they scale normally. Edge labels are hidden below 0.75 (already) and connection handles below 0.5. Geometry (positions, hit areas, snapping) does not change with the form. | Screenshot at 430×700 fit and 1000×700 fit: every card name is legible; browser test asserts computed on-screen font size of `.infra-node-copy strong` ≥ 12 px at zoom 0.3 and ≥ 16 px at zoom 1.0. |
| A2 | Most names are truncated ("Atlas hyper…", "Camera rec…") although there is space. | Card size becomes 216×64 (was 184×60). Update the model constants, snapping/free-slot/region padding helpers and their Node tests consciously; the coordinate contract stays centre-based. Name stays one line with ellipsis plus `title`. | No truncation for names up to 20 characters at zoom 1.0 in the fixture. |
| A3 | Minimap is ~210×190, covers cards and repeats "23 angezeigt". | Minimap 168×112, panel uses `--vd-theme-overlay-bg` with the theme blur, no text inside; the visible count moves into the minimap toggle's tooltip and the zoom cluster's aria-description. Hide the minimap automatically while the whole scene fits in the viewport; the toggle still works. | Screenshot 1000×700: minimap does not overlap any card at fit; hidden when everything fits. |
| A4 | Region fill is a heavy accent tint. | Regions use a neutral fill (`color-mix(in srgb, var(--vd-theme-panel-bg) 45%, transparent)`) and `--vd-theme-border` outline; accent only for the drop target, selection and hover of the header. | Visual check in Standard, Fruity light, Minimal light/dark. |
| A5 | Edge labels collide with cards ("Gbps", "metry-" cut). | Place the label at the midpoint of the visible segment between card edges; if its box would overlap any card, hide it and show it on edge hover/selection instead. | No edge label overlaps a card in the fixture screenshots. |
| A6 | Inspector kicker shows "Version 1" (an internal CAS number). | Plan C header: kind · place breadcrumb only; the version never appears in user-facing text. | Grep: no `infra_version`-style user string rendered in the inspector header. |
| A7 | Host badge reads "▣ 1 · 0 laufen". | Badge copy "{count} guests · {running} running" with singular keys, and "{count} guests · stopped" when none run; tooltip lists the first five guest names. | i18n keys in 16 locales; browser test checks the singular form. |
| A8 | Below ~560 px the search field shrinks to ~50 px. | Below 560 px container width the search collapses to an icon button (`data-action="open-search"`) that expands the field over the command bar until blur/Escape. | 430×700 screenshot: search usable, nothing overflows (existing overflow guard). |
| A9 | The canvas focus ring is heavy in dark themes. | Viewport focus uses an inset 1px `--vd-theme-accent-border` ring plus the standard `--ds-focus-ring-*` only on keyboard focus (`:focus-visible`). Card focus keeps the standard ring. | Visual check; keyboard focus still visible (Plan D a11y test). |

## After Plan C Tasks 5–10 (2026-10-24)

Owned by Plan C (Tasks 11–17 executor), because they touch panel copy and styling:

| id | finding | required change | acceptance |
|---|---|---|---|
| A10 | Cleanup suggestion cards each carry a full-width accent-filled button ("Duplikat entfernen und ignorieren", "Engine-Host zuweisen", "Host wählen…"); a list of loud primaries violates the one-accent-per-view rule. | Suggestion actions become compact secondary buttons (`.infra-button` secondary style, accent text on hover only); "Dismiss" becomes a ghost icon button with a tooltip. The only accent-filled control in the view stays the command bar's Add button. | Screenshot of the Cleanup section: no accent-filled buttons inside the library. |
| A11 | Overview shows "1 Verbindungen". | Singular/plural keys for the connection count (and any other count rendered in the inspector or library: guests, sources, assets in a place). | Node i18n test lists the `_one` keys; browser assertion for "1 connection" in en. |
| A12 | Overview shows raw coordinates ("Platzierung x 330, y 100"). | Overview never shows coordinates. Show "On the map" / "Not placed" (and the place breadcrumb, already present). Coordinates stay available only in the Details tab's placement group for power users (read-only text). | Overview screenshot without coordinates. |

## After Plan C (2026-10-24)

Owned by Plan D. A13, A15 and A16 join A1–A5, A8 and A9 in the first fix batch (D-fix-1); A14 belongs to Plan D
Task 9 (i18n review).

| id | finding | required change | acceptance |
|---|---|---|---|
| A13 | Between 1100 and 1300 px, active filter chips are clipped by the command bar (search shrinks to ~166 px). | Chips get a fixed share of the bar: show as many chips as fit, then a `+N` overflow chip that opens the filter popover; search keeps a minimum of 220 px above 760 px container width. | Browser test at 1110 px with 3 filters: no chip clipped, `+N` visible, overflow guard passes. |
| A14 | Nine legacy infra locale keys are unused but still say node/relation (add_node, empty_map_hint, add_first, map_hint, select_asset_hint, connect_hint, relations, no_relations, import_hint) plus infra_version, infra_group, infra_no_group, infra_needs_host, infra_import_hint_short, infra_import_existing. | Delete unused keys from all 16 locales after confirming with a repo-wide text search that no code or test reads them; Plan D's unused-key check enforces it afterwards. | Node unused-key check passes. |
| A15 | A kind with several instances would show the opaque instance hash in source chips. | Never show the hash: number instances per kind in stable order ("Docker 1", "Docker 2") with the hash only in the tooltip. | Node test for the label helper. |
| A16 | `APP_LINK_TARGETS` is duplicated in infra-inspector.js and infra-menus.js. | Move it to infra-model.js and read it from both. | Grep shows one definition. |
