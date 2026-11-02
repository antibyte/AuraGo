# FindIt

FindIt is AuraGo's local physical inventory for electronic parts, PC components, devices, tools and general supplies. It keeps item specifications separate from stock at each storage location.

## Enable and find things

Open **FindIt** from the Desktop. An administrator enables it in the app's settings; the separate **Agent access** setting exposes read operations only. Both settings default off. Desktop read-only and FindIt read-only each prevent human mutations. Agent access does not require permission to control Desktop windows. External A2A agents cannot use FindIt.

The list combines text search with category, template, package, location, availability and minimum-stock filters. Engineering values such as `4k7` and `4,7 kΩ` search numeric specifications; SI prefixes distinguish `M` (mega) from `m` (milli) and `u`/`µ` (micro). Select a template when a custom field name has different types or units across templates. Select an item to inspect its properties and stock. Locations may be nested, for example Workshop → Shelf A → Organizer → Drawer 12. A small window uses one pane at a time. List and card views use the same records.

Every filter stays available in a narrow window: open **Filters** to narrow by category, location, package and template. The location list in the filter shows the full storage path, so nested places stay distinguishable when the sidebar is hidden. The status bar reports when the inventory data was last loaded, and the arrow keys move through the list once it has focus.

Templates provide typed fields without inventing specifications. Electronic function, package family, pin count, pitch and dimensions are independent. Check technical values against the actual manufacturer datasheet; miniature artwork illustrates a family, not an exact footprint. Missing values remain unknown. Custom templates retain stable field keys and do not overwrite existing article values.

Archiving preserves history. An archived custom template remains readable on existing items but cannot be assigned to new ones. Items used by an active project BOM must be removed from that BOM before archiving. Empty stock positions remain available for returns and inventory corrections.

## Stock and projects

Stock quantities use exact decimals with up to six fractional digits. Pieces require integers; serialized stock is one piece per unique serial number. An item has one unit. On-hand minus active reservations is available stock.

Use Receive, Issue, Transfer, Adjust or Return to change stock. Adjust means the counted target quantity. Transfers preserve the item's specifications, lot, condition and serial. Movements remain in the ledger. If an existing stock position changes after the Desktop preview, its version check rejects the stale booking; review the refreshed quantities before retrying. A repeated request with the same request ID replays its receipt; changing the request under that ID is rejected.

If a response to stock or reservation changes is lost, FindIt keeps the exact pending request across app windows in the current browser session. Retry confirms the original values and reuses its receipt ID. Resolve it before making another change for that item or project; pending opening stock also pauses new item entry. Definitive rejection keeps the saved item and permits a new booking with corrected values. Returning an issued serial uses Return; archived storage locations cannot receive stock, and archived projects cannot change status.

Create a project and its bill of materials, then release it before reserving stock. Reservations use specific stock positions and never reduce on-hand quantity. Partial reservation is supported. Consume reduces both stock and reservation; Release returns held quantity to availability. Cancelling releases outstanding reservations. Completing never consumes implicitly and requires outstanding reservations to be resolved. Reopen a released project to edit its BOM revision after resolving holds. Substitutions require explicit item selection.

## CSV

CSV import is available for catalog items and project lines. It accepts UTF-8 (including a BOM), comma or semicolon separators, and explicit column mappings. Preview every row before import. Duplicate items require an explicit skip; ambiguous project matches require explicit resolution. Invalid or unresolved rows block the entire import. Catalog import does not alter stock. Items, stock and project lines can be exported as CSV.

Imports accept at most 10 MiB, 25,000 rows and 256 columns. Exported text that could be interpreted as a spreadsheet formula receives a literal apostrophe prefix. The reserved `findit_csv_encoding=literal-v1` column declares this encoding, allowing FindIt to recover the original values on reimport. Keep the original export and its marker for a round trip; spreadsheet editors may alter that encoding when saving. Ordinary unmarked CSV retains its apostrophes unchanged.

## Photos and labels

Upload or capture a JPEG, PNG or WebP image (maximum 10 MiB). Images are normalized locally, stripped of metadata and scaled to at most 2048 pixels; thumbnails are at most 256 pixels. Photos take precedence over chosen sprites and template/package defaults. Datasheet URLs are stored as links; FindIt does not fetch them automatically.

Print item or location labels in the default 50 × 30 mm format, individually or on A4. QR links contain the record identifier, not an access credential; normal AuraGo login is required. A confirmed scan navigates to a record and never books stock. Image upload and manual identifier entry remain available when the camera is unavailable. Enter the printed `I-` (item) or `L-` (location) code; if a short identifier matches several records, select the intended record explicitly. Browser camera capture requires a secure context and the user's camera permission. Printing needs Desktop write permission; FindIt's inventory read-only mode still permits printing.

## Agent reads

The native `findit` tool has four operations: `search`, `get`, `availability` and `shortages`. Search/get cover items, projects, locations and templates, including custom field definitions. Answers can include current quantities, location paths and shortages. Both the agent run's original permissions and the current settings must permit access; enabling access does not retroactively grant it to a running turn, while disabling it suppresses pending results. The service owns all calculations. The tool never writes stock or records and has no SQL interface.

## Persistence and delivery

The store is `data/findit.db` relative to the configured data directory; normalized images are in `data/findit-images`. The existing `.ago` backup includes both under a common capture boundary. Restore follows AuraGo's staged restore/restart workflow. Schema upgrades back up the existing database before migration. Data-directory changes require a restart.

Runtime JavaScript, CSS, translations and original sprite assets ship in the verified version-bound web resource set. Ordinary operation needs no extra server, container or internet service. This version does not implement purchasing, supplier catalog synchronization, native CAD import, automatic datasheet extraction or offline write queues.

## Verification

Run the FindIt domain, server, tool and UI suites; the SQLite backup coverage check; the Desktop theme/material/loader checks; `npm run build:ui` and `npm run check:ui`. Regenerate original artwork with `py -3.13 scripts/export-findit-assets.py`, then run it with `--check`. Browser fixtures use disposable inventory data. Real phone cameras and printed labels require separate hardware acceptance.
