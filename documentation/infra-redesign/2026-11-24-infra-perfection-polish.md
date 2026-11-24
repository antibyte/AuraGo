# Infra perfection polish (2026-11-24)

Fourth polish round of the Infra desktop app. Goal: no visible defect and no avoidable friction in the Map,
Topology, List, inspector, import and dialogs. The round changes the client only (`ui/js/desktop/apps/infra*.js`,
`ui/css/desktop-app-infra*.css`, `ui/lang/desktop/*.json`, tests, docs). No route, storage or permission changes.

Source of the findings: a live review in a local preview with a 65-asset home lab (German UI, Standard dark,
1440×900, 720 px and 414 px wide windows) and four code reads. Items marked **(live)** were seen in the running
app; items marked **(code)** were read in the source and must be confirmed while implementing. If a **(code)**
item turns out to be wrong, skip it and say so in the report.

## Rules for every package

- Read the Infra section of `ui/js/desktop/apps/AGENTS.md` (from "Load order (module-loader `infra`)" to the end
  of the Infra notes) before editing. Its invariants stay valid unless an item below replaces one explicitly.
- Every `infra*.js` file stays below 1100 lines and touches no DOM at load time. A new module must be added to the
  module-loader list and to the file list of `scripts/test-infra-ui.mjs` in the documented load order.
- CSS: theme tokens only (no hex, no literal colour functions outside `color-mix()`, no named colours);
  animations and transitions only inside `@media (prefers-reduced-motion: no-preference)`; durations from the
  existing tokens, never longer than 200 ms. `.infra-card:hover` never changes `transform`.
- Text: every new key exists in all 16 files of `ui/lang/desktop/` with a real translation. German uses "du",
  the glossary of package 6 and no trailing full stop on single-clause toasts.
- Feedback rule: a result the user cannot see where they are looking (a removal, ignore or restore, a background
  save, any failure) gets a visible toast and the live-region announcement. A result that happens under the
  pointer (a card lands where it was dropped, a card moves) gets the announcement only.
- Tests stay lean: update every assertion that pins behaviour an item replaces, and add one assertion per new
  behaviour that matters, in the nearest existing `ui/desktop_infra_*_browser_test.go` or in
  `scripts/test-infra-ui.mjs`. Do not weaken unrelated assertions.
- Do not normalise line endings. Do not use `\u` escapes in new source text; write the character.

## Package 1: inspector

Files: `infra-inspector.js`, `infra-inspector-forms.js`, inspector rules in `desktop-app-infra.css`.

- **1.1 Tabs fit (live).** At the default 360 px the four tabs need 396 px in German, so a scrollbar shows under
  the strip and a label is cut. Required: at 360 px the strip does not overflow in German and English (tighter tab
  padding and count badge, tabs share the width). Where it still overflows (320 px, long locales) the strip keeps
  scrolling and revealing the active tab as today, but shows no scrollbar; a fade at the cut edge marks the
  overflow. Touch targets on coarse pointers stay 44 px.
- **1.2 Header action (live).** "In Schnellverbindung öffnen" wraps to two lines as a full-width primary block.
  Required: one compact button on one line with the short label of the target app ("Schnellverbindung", "Quick
  Connect") and the full sentence as `title` and `aria-label`; same size and row for the "Bearbeiten" variant, both
  left aligned under the status pills.
- **1.3 Breadcrumb (live).** A wrapped breadcrumb starts its second line with "›". Required: a crumb and the
  separator after it never split, so a new line starts with a place name.
- **1.4 Source cards (live).** A source with two pills (state and power) pushes one pill over the timestamp.
  Required: pills wrap in their own area at the right of the title; the timestamp sits below on its own line and
  nothing overlaps at 320 px.
- **1.5 Connection direction (live).** A host lists its guests as "Läuft auf Home Assistant". Required: when the
  inspected asset is the target of a directed connection the row uses the inverse label: `runs_on` "Hostet"
  ("Hosts"), `depends_on` "Benötigt von" ("Required by"), `powered_by` "Versorgt" ("Powers"), `member_of` "Enthält"
  ("Contains"). Undirected kinds keep one label.
- **1.6 Source-reported connections (code).** Edit and remove are disabled without a reason. Required: no dead
  buttons; the row shows a short note "Von {source} gemeldet" instead.
- **1.7 Connections heading (live).** The panel repeats "Verbindungen 7" under the tab of the same name.
  Required: keep the heading for assistive technology only; the add button sits at the top of the panel.
- **1.8 Guests in the overview (live).** Guest chips stack in a right-aligned column, one per line. Required: the
  label sits above and the chips wrap from the left across the full width, each with its power dot; after eight
  chips a "+N weitere" control reveals the rest in the library, like the map shelf.
- **1.9 Fact labels (live).** "Hersteller und Modell" wraps. Required: fact labels are one line ("Hardware").
- **1.10 Validation (code).** Invalid CIDR, link or custom key only disables Save; no message is shown. IP, MAC
  and VLAN are not checked. Required: Save stays enabled while the draft is dirty. A field shows its message
  below it after blur or after a save attempt (`aria-invalid`, `aria-describedby`). A save attempt with invalid
  fields focuses the first one and saves nothing. Add client checks for IP (v4 or v6), MAC (six octets, `:` or
  `-`) and VLAN (1 to 4094) that are never stricter than `internal/infra/validate.go`.
- **1.11 Focus after Save and Discard (code).** The panel repaint drops focus to the document. Required: focus
  returns to the same control, or to the active tab when that control is gone.
- **1.12 Copy feedback (code).** "Kopiert" shows even when the clipboard write failed. Required: success toast
  only on success, otherwise "Kopieren nicht möglich".
- **1.13 Move hint (code).** The overview always says "select it on the map and press M". Required: only in a
  map view of a writable window.
- **1.14 Icon picker (code).** A search without a match shows an empty grid. Required: "Kein Symbol passt".
- **1.15 Leaving a dirty draft (code).** Closing the inspector or selecting another asset keeps the draft
  silently. Required: one toast per asset, "Entwurf für {name} bleibt erhalten", when a dirty draft is left.
- **1.16 Inline connection form (code).** It has no Cancel. Required: Cancel beside Save restores the stored
  values.
- **1.17 Small targets and states (code).** Copy buttons 24 px, list remove 26 px, fact links 26 px, crumbs
  22 px. Required: at least 28 px (crumbs 24 px) on fine pointers; list-editor add and remove buttons get a hover
  state; `summary` heads get the focus ring.
- **1.18 Type scale (code).** Connection and selection headers use 17 px, the asset header 18 px; inspector
  inputs use 15 px, other inputs 14 px. Required: 18 px headers, 14 px inputs.

## Package 2: map

Files: `infra-canvas.js`, `infra-gestures.js`, `infra-library.js` (drag source), `infra-lanes.js`,
`desktop-app-infra-canvas.css`.

- **2.1 Selection stays visible (live).** The inspector covers a selected card near the right edge; in a 720 px
  window it covers almost every card. Required: the map still does not move when the inspector opens, unless the
  selected card would be covered by the inspector, the library sheet or the import sheet. Then it pans by the
  smallest distance that shows the card with 24 px clearance (animated, at most 200 ms, instant with reduced
  motion or shell animations off).
- **2.2 Focus stays visible (live).** Tab can put focus on a card outside the view (arrow keys already reveal
  it). Required: a card or region header that receives focus outside the uncovered view is revealed the same way.
- **2.3 Library drag preview (live).** The ghost hangs right and below the pointer while the card lands centred
  on the pointer, so the hint can name another place than the ghost covers. Required: the ghost is centred on the
  pointer and drawn at the current zoom (clamped to 0.5 to 1.25). A dashed slot outline shows where the card will
  land after grid snap and free-slot search.
- **2.4 Invalid drop (code).** Dropping a library row on the sidebar looks valid and does nothing. Required: the
  ghost shows a not-allowed state there and a cancelled drag is announced.
- **2.5 Edge auto-pan (code).** Nothing pans when a card, a library ghost, a marquee or a connection is dragged
  to the edge. Required: within 36 px of an edge the map pans towards it, faster the closer the pointer is, and
  stops when the pointer leaves the band or the drag ends.
- **2.6 Compact names (live).** Below 65 % zoom a long name wraps to two lines and runs over the card below
  ("FRITZ!Box 7590 AX", "Kamera Garten" behind "Wetterstation"). Required: a compact name is one line with an
  ellipsis and never taller than the space down to the next grid row; the full name is the `title`.
- **2.7 Guest badge and connection labels (live).** The guest badge under a host hides a connection label
  ("MQTT"). Required: the badge rectangle is an obstacle for connection labels.
- **2.8 Zoom limits (code).** Zoom buttons and keys do nothing at 25 % and 200 % without feedback. Required: the
  button at its limit is disabled with a title; a key press at the limit announces it once.
- **2.9 Minimap switch (code).** While the whole map fits, the minimap is hidden and its switch seems dead.
  Required: pressing it then says "Die ganze Karte ist sichtbar. Die Übersichtskarte erscheint beim Hineinzoomen"
  as an info toast.
- **2.10 Mode banner (code).** The Connect tool and the keyboard move mode have no visible mode hint. Required:
  a small banner at the top centre of the map while a mode is active: "Von Asset zu Asset ziehen · Esc beendet"
  and "Mit Pfeiltasten verschieben · Enter übernimmt · Esc bricht ab". It never takes pointer events.
- **2.11 Escape and tools (code).** Escape clears the selection but leaves the Connect or Hand tool armed.
  Required: with no gesture in progress Escape first returns to the Select tool, then clears the selection.
- **2.12 Connect cursor (code).** In the Connect tool the card body starts a connection but shows an arrow.
  Required: `crosshair` over cards in that tool, `grabbing` while a card is dragged.
- **2.13 Placing with the plus button (code).** The pointer click on "+" in "Zu platzieren" starts the hidden
  keyboard move mode, and the next click announces "Verschieben abgebrochen". Required: a pointer click places and
  selects only; the keyboard path (P) may enter move mode, with the banner of 2.10.
- **2.14 Delete on a connection (code).** Delete does nothing for a selected connection. Required: Delete or
  Backspace with a selected connection and no selected card removes it the way the inspector does, with the toast
  of 3.5.
- **2.15 Arrange scope (code).** "Anordnen" lays out the whole map unless several cards of one place are
  selected, and the label does not say so. Required: the command reads "Auswahl anordnen" when it will arrange
  the selection and "Ganze Karte anordnen" otherwise, in the menu and the context menu.
- **2.16 Topology lanes (live).** "Cloudflare Tunnel" (`cloud-service`) sits in "Clients und IoT" and a
  single-board computer in "Server und Speicher". Required: check the lane icon sets against the lane titles and
  fix evident mismatches (`cloud-service` to Internet, `sbc` to Clients und IoT) unless the lane notes in
  `AGENTS.md` give a reason.
- **2.17 Connection hover (code).** The hover colour of a connection snaps. Required: the token transition.

## Package 3: import, dialogs, menus, feedback

Files: `infra-import.js`, `infra-dialogs.js`, `infra-menus.js`, `infra-cleanup.js`, `infra-store.js`,
`infra-model.js` (`sourceLabels`), `infra.js`, their rules in `desktop-app-infra.css`.

- **3.1 Compact discovery rows (live).** Every discovery row is 177 px high because it always shows the link
  select and two buttons: 14 discoveries need 2713 px in a 389 px list. Required: a collapsed row of at most
  64 px with checkbox, icon, name and one meta line, and a disclosure button (`aria-expanded`) that opens the link
  select and the Import and Ignore buttons of that row. Rows of "Mögliche Duplikate" and rows with an inline
  error start open. Dragging a row to the map, bulk actions and the DOM hooks of the existing tests keep working.
  A section without rows is not rendered.
- **3.2 Source names (live).** Two Docker engines appear as "Docker 1" and "Docker 2". Required: when a kind has
  several instances the label is "Docker · aurago-test" (instance name, shortened with an ellipsis after 22
  characters, full text as title); numbers only when an instance has no name. This applies wherever
  `sourceLabels` is used.
- **3.3 Import load failure (code).** A failed load shows the empty state "no discoveries". Required: an inline
  error with a retry button; the empty state only after a successful load without discoveries.
- **3.4 Create dialog (live, code).** Button "Neues Asset", title "Infrastruktur-Asset hinzufügen", submit "Asset
  erstellen"; a name of spaces is ignored silently; the rule under the title stops before the close button.
  Required: title "Neues Asset" or "Neuer Ort", submit "Erstellen", a visible message and focus on an empty name,
  a rule across the full width.
- **3.5 Visible feedback (code).** Removing a connection, ignoring and restoring sources, removing a duplicate,
  assigning an engine host or a guest host and a partial deletion only announce. Required: the feedback rule
  above. Toasts and dialog titles name a place as "Ort" ("Ort erstellt", "Ort entfernen"), not as asset.
- **3.6 Backdrop click (code).** A click beside the create or connection dialog discards typed input. Required:
  a backdrop click closes only an untouched form; Escape and Cancel close as before.
- **3.7 Read-only while a dialog is open (code).** Save then does nothing. Required: the submit button is
  disabled and the dialog shows the read-only notice.
- **3.8 PDF export failure (code).** A network failure toasts the browser's "Failed to fetch". Required: always
  the translated failure text.
- **3.9 Power actions (code).** Running and accepted states show "check the device state"; the result toast is
  the bare word "Abgeschlossen" or "Fehlgeschlagen"; an unknown status shows a raw key. Required: "Wird
  ausgeführt…" while running, a toast "{name}: {action} abgeschlossen" or "… fehlgeschlagen", and the generic
  status text for an unknown status.
- **3.10 Restore ignored (code).** No busy guard. Required: one request per click, button busy meanwhile.
- **3.11 Toolbar menus (live).** The "Weitere Aktionen" and add menus are bare text lists; map-only commands are
  offered in the List. Required: glyphs, shortcut hints (`?`, `F`) right aligned, separators between groups
  (refresh | arrange, legend | shortcuts | PDF), and "Anordnen" and "Legende" disabled with a reason in the List.
- **3.12 Card context menu (live).** "Öffnen" is vague, the two removal items share an icon, power actions are
  missing. Required: "Details anzeigen", the asset's available power actions as their own group, the Delete
  shortcut hint on "Aus Infra entfernen", distinct icons.
- **3.13 Undo labels (code).** The undo and redo buttons never say what they will undo. Required: title and
  `aria-label` name the command ("Rückgängig: Verschieben (Strg+Z)").
- **3.14 Undo history cleared (code).** A layout conflict clears the history without a word. Required: a toast
  that earlier steps can no longer be undone.
- **3.15 Undo announcement (code).** Undo of an optimistic entry announces "Rückgängig" even when nothing
  changed. Required: announce only what happened; confirm the finding first.

## Package 4: List

Files: `infra-table.js`, table rules in `desktop-app-infra.css`.

- **4.1 No sideways overflow (live).** At a 1131 px table the eight-column layout needs 1203 px: a scrollbar
  shows and the last header reads "Betriebs". Required: at every width from 360 px the columns fit; a layout
  step applies only where it fits; every header is readable; the name column gets the spare room.
- **4.2 Row click (code).** The whole row looks clickable, only the name reacts. Required: a click anywhere in a
  row except checkbox, expander and links selects the asset and opens the inspector; a double click on an
  unplaced asset does the same instead of nothing.
- **4.3 Row keys (code).** Rows have three Tab stops each and no arrow keys. Required: one Tab stop for the
  body; Up and Down, Home and End move between rows; Space toggles the checkbox; Enter opens the inspector.
- **4.4 Hardware sort (code).** Hardware is the only column without sorting. Required: sortable like the rest.
- **4.5 Truncated cells (code).** Name, Ort, Hardware and Quelle are cut without a tooltip. Required: `title`
  with the full text.
- **4.6 Status column (live).** Manual assets show a "Manuell" pill although the source column says the same.
  Required: the status column shows reachability (Live, Offline, Unbekannt, or a dash without a signal) and adds
  a warning pill when a source is stale or removed; no "Manuell" pill. Sorting by status follows that order.
- **4.7 Expander gutter (live).** Every row keeps about 60 px empty before the icon. Required: 24 px.

## Package 5: shell polish

Files: `infra.js`, `desktop-app-infra.css`.

- **5.1 Filter popover (live).** The three selects start at different x positions. Required: one label column
  and selects of equal width.
- **5.2 Layers (code).** Toasts (z-index 25) sit below popovers (30). Required: toasts above popovers.
- **5.3 Small targets (code).** Status reset 26 px, discovery checkbox 22 px, row checkbox 18 px. Required: a
  hit area of at least 24 px.
- **5.4 Checkboxes (live).** Row checkboxes in the import sheet look different from the section checkbox.
  Required: one checkbox look in the library, the import sheet and the List.
- **5.5 Dead rules (code).** `.infra-detail-more`, `.infra-detail-actions`, `.infra-quick-place`,
  `.infra-relations`, `.infra-hosted-services`, `.infra-target-list`, `.infra-target-option` match nothing.
  Required: confirm each against `ui/js` and `ui/*.html`, then delete.
- **5.6 Radius literals (code).** 4, 5, 6, 7, 9 and 11 px literals beside the radius tokens. Required: replace a
  literal with a token only where the token has the same value in the Standard theme.

## Package 6: texts

Files: `ui/lang/desktop/*.json`.

German glossary: Asset; Karte, Topologie, Liste; Ort (Standort, Raum, Rack, Gruppe); Fund; Quelle; Verbindung
and Verbindungstyp; importieren; ignorieren; Aufräumen; "auf der Karte" and "nicht auf der Karte"; Inventar;
"ungespeichert"; Betriebszustand; Esc.

- **6.1** Apply the German corrections of the copy review (table in the plan for this package): one term per
  concept, the next step in error texts, shorter buttons, "Neu starten" for restart and reboot, "Betrieb"
  instead of "Strom", the quoted button name in `infra_source_reviewed` matching its button.
- **6.2** Single-clause toasts end without a full stop in German and English.
- **6.3** `_one` variants for `infra_pdf_assets` and `infra_pdf_guests`, used by their call sites.
- **6.4** Empty discovery group title "Wahrscheinlich kein Homelab" becomes "Vermutlich nicht relevant".
- **6.5** Review every key the packages 1 to 5 added, in German and English, against the glossary.

## Out of scope

Marquee selection without Shift, undo for create and delete, a reachability filter, touch long-press dragging,
window-resize refit, a place field in the create dialog. They change habits or need server work.

## Verification

`node scripts/test-infra-ui.mjs`, the UI line budget, i18n and asset guards, `npm run check:ui`, and
`AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopInfra`. Before and after screenshots of the reviewed
scenes from the live preview kit. One visual pass over the other desktop themes for the changed surfaces.
