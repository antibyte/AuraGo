# Email files from Desktop apps

Choose **Email …** beside an app's export actions, choose an available format,
and review the new Mailer draft. The file is already attached; recipients and
message text are empty. Nothing is sent until you choose **Send** in Mailer.

Mailer must be enabled, the Desktop must be writable, and a connected account
must allow sending. The last selected sending account is preferred when it is
still available. Otherwise the first usable sending account is selected.
If setup is missing, the action offers to open Mailer's settings. Enabling or
connecting an account remains an explicit action; return to the source app and
choose Email again afterwards.

## Supported outputs and verification matrix

Formats use each app's existing exporter, validation and runtime prerequisites.
The format picker selects one output format, not every format at once. Multiple
selected files or companion files become attachments in the same draft.

| App | Email output | Source selection |
| --- | --- | --- |
| Autor | DOCX, HTML, Markdown, TXT | Current editor, including unsaved edits; simpler-format warning retained |
| Tabellen | XLSX or loaded XLSM, CSV | Current calculated workbook; current sheet for CSV, with loss warning |
| Notes | Markdown, HTML, TXT | Current note, including unsaved edits; HTML embeds supported local images |
| Text editor / Code Studio | Original text filename | Current editor/tab contents |
| Cheater | Markdown | Open cheatsheet's current source |
| Pixel | PNG, JPEG, WebP, OpenRaster | Current document; JPEG/WebP quality and complete native ORA layers |
| Camera | JPEG, recorded video | Current completed photo/clip |
| Gallery | Original media | Current item or multiple selected items |
| Network Cameras | JPEG | Explicit snapshot of the selected camera through the existing snapshot operation |
| System World | PNG | Current rendered scene |
| QR dialog | Existing raster/vector/text formats, validated CSV batch ZIP | Current verified configuration/preview or validated batch |
| Synth Studio | WAV, MIDI, `.aurasynth` | Current arrangement/project |
| Noisemaker | Original audio | Selected library tracks, otherwise the current player track |
| RTL-SDR | WAV | Open completed recording result |
| Video Studio | Completed downloadable artifacts | Explicit completed job, including rendered video or still image |
| OpenSCAD | SCAD and completed render formats | Current source or explicitly selected render file |
| NASSCAD | Native project, STL/ASCII STL, 3MF, GLB/Draco GLB, OBJ, PLY, IFC, STEP, SVG/DXF section | Native exporter and its current settings |
| Layerling | Native LYL, STL, OBJ, 3MF, STEP, PNG | Current editor through its existing frame bridge |
| Detective | Advertised report formats | Selected immutable report revision |
| Newspaper | PDF | Selected edition; scheduled/confirmed delivery remains separate |
| EasyDrag | `.easydrag.json` | Flushed current draft, including pending fields |
| Game Maker | Project ZIP | Current published export after pending editor work is flushed |
| CYD Creator | Project ZIP | Current project after layout/code saves finish |
| File Manager | Original files | Current local or authorized remote selection; folders are excluded |
| File viewer / 3D viewer | Original file | Current file or archive member |
| Zipper | ZIP or selected members | Current archive/selection |
| Bookshelf | Source format or existing EPUB rendition | Selected book through its authorized prepared-content ticket |

Store apps and Tresor are excluded. A print dialog does not create an email PDF;
only formats with an existing byte exporter are offered. Rendering is initiated
through the app's normal controls before selecting a completed render artifact.
No new conversion service or mail transport is installed.

## Limits and failures

The existing outgoing attachment limit is **25 MiB combined**. A smaller configured
upload-file budget also applies; Mailer's state endpoint advertises both limits.
Files use ordinary private, revisioned Mailer draft attachments. Preparing an email
does not change the source file, grant file access, or save unsaved source edits
unless that app's native project export requires its normal save step.

Changing/closing the source cancels its pending preparation. After handoff, the
Mailer draft owns its attachments independently. Upload failures stay visible and
block sending until retried or removed. Closing the composer uses its existing
save/upload guard. Reloading the Desktop never repeats the transient handoff;
saved drafts can be reopened in Mailer.

The NASSCAD wrapper loads the version-bound adapter in existing workspace copies
as well as fresh bundles. Workspace app files are preserved. Native exporters and
file pickers are restored after each email export.

## Developer checks

- `node scripts/test-desktop-email.mjs`: preflight, permissions, account choice,
  byte/type retention, format cancellation, size boundaries, concurrent windows,
  source cancellation, bounded downloads and scoped NASSCAD capture.
- `node scripts/test-desktop-email-adapters.mjs`: execute every app's actual
  handoff callback against local exporter/download fixtures, including unsaved
  editor values, format choices, multiple files and selected revision URLs.
- `node scripts/test-nasscad-browser.mjs`: all twelve native encoders, current
  section settings, STEP settings/cancellation and absence of browser downloads.
- `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestQRDialogWriterPixelBrowser`:
  real QR exports and Pixel pixels, nested format-dialog focus/cancellation and
  unsaved canvas contents in the email handoff.
- `node scripts/test-desktop-email-browser.mjs`: actual format dialog, Mailer
  composer, revisioned multipart uploads, upload failure/retry, independent drafts,
  current text snapshots; Standard/Fruity, light/dark, 390/1280 px. Uses Playwright
  from the existing test environment (`AURAGO_PLAYWRIGHT` may select its package).
- `go test ./ui -run 'TestDesktopEmail|I18n|LineBudget'` and
  `go test ./internal/mailer ./internal/server -run 'TestAttachmentLimits|TestMailer'`.
- Existing app suites verify native DOCX/XLSX/ORA, media, project and report
  exporters. Keep every row registered in `TestDesktopEmailExporterCoverage`.
- `npm run build:ui`, `npm run check:ui`, `node scripts/vendor-nasscad.mjs --check`.

These fixtures do not enqueue or send real email. Physical camera/radio devices,
external mail providers and optional CAD/render runtimes require their existing
separate acceptance checks.
