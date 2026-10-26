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

## Consequences of A2 for the rest of Plan D (D-fix-1)

The card is 216×64 and the model constants moved with it: `CARD_W = 216`, `CARD_H = 64`, `freeSlot` gap 8 (a stacked card lands 72 px away), `compactGrid` cells 240×96, Arrange grid 252×160 from (126, 100).
Plan D Task 1 was written for 184×60 cards: its `LANE_CELL_W = 216` leaves no gap between neighbours. Use `LANE_CELL_W = 240`; `LANE_ROW_H = 96` stays (64 plus 32). Every pinned x in the Task 1 and Task 4 tests that assumes a 216 px column pitch changes accordingly (centre ± 120 for two cards, 240 apart for a row).

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

## After Plan D Tasks 1–5 (2026-10-25)

Source: review of the Topology screenshots (1500×970 fit and 100 %, 1000×700, 430×700, drag lane highlight) and of the
D-fix-1 screenshots. Owned by Plan D fix batch D-fix-2, which runs before the Task 6–9 polish. These amendments
supersede Plan D decisions 3, 5 and 6 and the `LANE_H`/`LANE_ROW_H` values of the lane contract where they conflict.

| id | finding | required change | acceptance |
|---|---|---|---|
| A17 | Lanes are 480 px tall with mostly empty space; fit is 39 % at 1500×970. Compact names of the first auto row touch the second row's boxes (row pitch 96). | `LANE_H = 384`, `LANE_ROW_H = 144`, first row centre at lane top + 96, so two auto rows per lane (centres +96 and +240). A lane needing more than two rows widens its rows (columns per row = max(8, ceil(n / 2))). `laneAt(y) = clamp(floor(y / 384), 0, 3)`. Recompute every pinned number in the Node and browser tests consciously. | Fit at 1500×970 ≥ 45 %; at 1000×700 fit no compact name overlaps another card box (browser assertion over all card/name rects). |
| A18 | Lane bands are boxes around the content; labels sit at the band's left edge and leave the view at 100 %. | Swimlanes: each lane is a full-width stripe (drawn far beyond the content, not part of scene bounds, fit or minimap), alternating a subtle neutral fill with a 1 px `--vd-theme-border` separator. Lane labels are sticky: clamped to the viewport's left edge + 12 px while the lane is in view, counter-scaled to ≥ 12 px on screen. The drag highlight tints the whole stripe. | At 100 % zoom the label of every lane in view is visible; at 430×700 no label is cut. |
| A19 | Inter-lane connections are straight lines that run through cards of the lanes in between and bundle in one point. | In Topology, a connection between cards in different lanes is a cubic curve from the bottom edge of the upper card to the top edge of the lower card (vertical tangents, control offset max(48, dy / 2)). Start points of several connections leaving the same card edge spread over the middle 60 % of that edge, ordered by the other end's x; the same for arrival points. Same-lane connections stay straight between facing sides. Labels sit at the curve midpoint and keep the A5 rule; hit testing and selection use the same path. The Map view stays straight. | Node test for the path helper (anchors, spread order); screenshot shows curves. |
| A20 | The auto layout produces avoidable crossings. | Order cards inside each lane by the barycentre of their connected neighbours (sweep top to bottom using the lanes above, then one sweep bottom to top for the lanes that have no neighbour above), ties broken by the existing deterministic order; unconnected cards go last. | Node test: on the realistic fixture the crossing count is lower than with connection order, and the order is deterministic. |
| A21 | Only 3 of 22 fixture assets land in "Servers and storage"; NAS shelves, recorders, databases, UPS and PDU fall into Clients and IoT. | Compute lane also takes storage and rack infrastructure icons and role words (nas, disk shelf, storage, database, nvr/recorder, server, rack, ups, pdu, hypervisor); Network also takes vpn, firewall, gateway, modem and ont. Check the real icon ids in the model; keep cameras, printers, TVs, phones, tablets, laptops and sensors in Clients and IoT. | Node test per icon group. |
| A22 | Saved `pinned: false` topology placements from the retired layered Arrange freeze assets in the top lanes and hide the auto layout. | Topology ignores saved placements with `pinned: false` and draws those assets at their auto slots. Arrange in Topology hands the selection (or every card) back to the automatic layout: one undoable `PUT /layout` of unplace operations; undo restores the previous placements. Without `store.features.unplace`, Arrange is disabled in Topology. Drag and keyboard moves keep saving `pinned: true`. Auto cards that change slot after a poll move with the existing transform transition when animations are enabled. | Node tests for the rule; browser test: Arrange in Topology unplaces a dragged card and undo restores it. |
| A23 | The library in Topology says "On the map" and "Everything is on the map". | In Topology the To place section is hidden and the placed section is titled "In topology" (new key, 16 locales). | Browser assertion in Topology. |
| A24 (Q1) | Compact cards read as empty boxes; the 28 px icon is ~12 px on screen at 44 %. | In compact form the icon is counter-scaled to min(20 px, 0.8 × on-screen card height) on screen; geometry unchanged. | Browser assertion at zoom 0.44: icon ≥ 18 px on screen. |
| A25 (Q2) | Minimal dark: the region outline is invisible. | Region and lane separators use a border colour that stays visible in all 8 QA themes (for example `color-mix(in srgb, var(--vd-theme-text) 16%, var(--vd-theme-border))`); Task 8's theme test asserts a contrast ≥ 1.4:1 against the canvas. | Theme test. |
| A26 (Q3) | With only filters active the empty map offers "Clear search"; library sections show bare "0" headers. | The empty-state action matches the cause: query only → "Clear search", filters only → "Reset filters", both → "Clear search and filters" (new keys where missing). Library sections under an active query or filter show one muted "No matches" line instead of a bare header. | Browser assertion for the filters-only case. |
| A27 (Q4) | At 1110 px the search placeholder truncates because of the Ctrl K badge. | Hide the shortcut badge whenever the field is narrower than placeholder + badge (container query or measured class); the placeholder never truncates at ≥ 760 px container width in en and de. | Browser sweep 760–1500 px in en and de. |

Resolution of A21: the `modem-ont` icon and the modem/ont/isp/wan/dsl role words stay in the Internet lane (existing
contract; the amendment text was wrong there). `vpn` joins Network.

## After D-fix-2 (2026-10-25)

Source: D-fix-2 screenshots (Topology 1500×970 fit and 100 %, 1000×700; Map filters-only empty state). Owned by fix
batch D-fix-3, which runs together with Plan D Tasks 6 and 7.

| id | finding | required change | acceptance |
|---|---|---|---|
| A28 | A same-lane connection between non-adjacent cards runs straight behind the cards in between, so every pair of neighbours looks connected by a short stub. | Same-lane connections between cards that are not direct neighbours in their row are arcs above the row: from the top edge of one card to the top edge of the other, arc height clamp(24 + 0.12 × dx, 32, 56) world px, using the A19 slot spread on the top edges. Direct neighbours keep the straight segment. No connection is ever drawn behind an intermediate card in Topology. | Node test for the path helper (arc for a gap, straight for neighbours); screenshot without stubs between unconnected neighbours. |
| A29 | Power and dependency connections inside a lane (UPS to every server) add clutter to the network view. | In Topology, network connections (ethernet, fiber, wifi) are always drawn; `powered_by` and `depends_on` connections are drawn only while one of their ends is selected or hovered (and always in the Map). Check which edge kinds the canvas draws today and keep membership kinds as they are. The legend or connection count must not claim hidden connections are missing: the inspector's Connections tab still lists them all. | Browser assertion: a power connection appears in Topology only when its end is selected. |
| A30 | With a card selected, its connections look like every other connection; with filters active, connections of dimmed cards stay bright and run over the empty-state message. | Selection emphasis in both views: the selected card's connections use the accent stroke, sit above the others and the other connections fade to 35 % opacity; without a selection nothing changes. A connection with a dimmed (filtered out) end is dimmed like the cards. The empty-state message sits above the connection layer on its own surface. | Browser assertions for emphasis and for dimmed connections under a filter. |
| A31 | Connections run over the lane labels. | Lane labels are drawn above the connection layer. | Visual check. |
