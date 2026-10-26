# Shared QR codes

Writer, Tabellen and Pixel use one local QR dialog and the same native Go renderer as the agent's `qr_code` tool. No external QR service, Python, browser sidecar or shell is needed for generation. The functional reference is [Mini QR](https://github.com/lyqht/mini-qr); AuraGo uses its own implementation, presets and interface. No Mini QR source or assets are redistributed.

## Desktop use

Choose **QR-Code** in the editor's insert menu or Pixel's tools. Enter the content, choose a presentation, and wait for the current preview before inserting or exporting. The compact view can pin selected controls from the full settings. Only that display preference is remembered automatically; payloads, captions and imported images are not saved in browser storage.

- **Writer:** inserts a PNG at the captured cursor position, with dimensions in millimetres. It remains an ordinary document image, participates in undo/redo, and survives DOCX save/reopen. Inserting while text is selected retains the selected text. Read-only/suggestion mode cannot insert.
- **Tabellen:** inserts an image anchored to the selected cell. Move, resize or delete it with the image controls; undo/redo, row/column operations, print and XLSX save/reopen retain it. Existing charts and unrelated package parts remain intact. CSV cannot retain images and shows a loss warning. Conflicting file versions retain the draft instead of overwriting another save.
- **Pixel:** adds a separate raster layer at the chosen position and size. Existing layers and selections are retained. Undo/redo and ORA save/reopen preserve the layer; flattened exports preserve its visible pixels. Resize the canvas or adjust placement if the complete QR including its quiet zone does not fit.

Changing or closing the target document cancels stale dialog work. A delayed preview, file read or camera permission cannot insert into a replacement document.

## Content and appearance

| Area | Available controls |
| --- | --- |
| Content | Text, HTTP(S) URL, email with CC/BCC/subject/body, telephone, SMS, Wi-Fi, vCard 2.1/3.0/4.0, location, calendar event and EPC/SEPA payment data |
| Contacts | Names, organisation/title, generic/work/private/mobile phones, email, URL, address, birthday and note |
| QR geometry | Six module shapes, separate finder/centre shapes and colours, correction L/M/Q/H, size and quiet zone |
| Images and colour | Alpha colours, rounded background, logo with margin/coverage/background clearing, optional frame background image |
| Frames | Top/bottom/left/right, multiline text, font/size, padding, side width, border and colours |
| Presets | Original presentation and frame presets, bounded randomisation, explicit full configuration import/export |
| Exports | PNG, JPEG, SVG, ASCII, Unicode full blocks, Unicode half blocks, TXT and Markdown |
| Batch | UTF-8 CSV text/URL/vCard rows, per-row filename/caption/font, validation and small previews, complete ZIP export |
| Scan | PNG/JPEG/GIF/WebP upload, supported clipboard images, drag/drop or an explicitly started camera; recognised content can refill the form |

Imported logos/backgrounds accept those raster formats and a restricted static SVG profile. SVG scripts, external resources, styles, text, strokes and unsupported geometry are rejected; accepted imports become PNG before use. Animated images use one frame. Scan accepts raster images only. Camera and clipboard availability depend on browser permissions and secure context; file upload/download remain available. Scanning displays data and never opens links, joins Wi-Fi or submits payments automatically.

Telephone/SMS preserve an explicit international number or RFC3966 context/extension. Bare local numbers are accepted with a warning; AuraGo does not infer a country. Calendar times with an IANA zone reject ambiguous or nonexistent local times; all-day end dates are exclusive. EPC validates IBAN/reference checksums and exact decimal EUR amounts, requires correction M and the version-dependent BIC, and never initiates payment.

Captions use built-in Go Sans/Mono and verified local Liberation Serif/Noto Arabic, Devanagari, CJK and monochrome emoji resources. Unsupported glyphs or unavailable resources cause a clear error. CJK regional glyph forms follow the bundled Simplified Chinese face. No font is fetched while rendering.

PNG/JPEG exports are decoded from their final bytes. SVG exports are parsed and rasterised before verification. Text exports verify their matrix and omit frame/logo artwork. Transparent output is checked on white; JPEG uses the selected opaque flattening colour. Software decoding cannot guarantee every camera, printer, background or physical size: keep strong contrast and sufficient quiet space, and scan the final printed result before distributing it.

## Agent access

The Desktop configuration page has separate QR tool enable/read-only controls. Defaults:

```yaml
tools:
  qr_code:
    enabled: false
    readonly: true
```

Enable the tool and disable its read-only setting to create artifacts. Human editor use has its own Desktop/editor permissions. The agent discovers four operations:

| Operation | Result and grants |
| --- | --- |
| `presets` | Content/style/font/export catalog; available in tool read-only mode |
| `scan` | Untrusted decoded text and recognised fields from an authorised relative file reference; available in tool read-only mode |
| `generate` | Verified artifact from `spec` or `config_ref`; `export_config:true` instead explicitly saves a normalised `.qr.json` file |
| `batch_generate` | Complete ZIP from `csv` or `csv_ref`; any invalid row prevents publication |

```json
{"operation":"generate","output_root":"desktop","output_path":"Documents/contact-qr.png","spec":{"schema_version":1,"content":{"type":"url","url":"https://example.org/contact"},"style":{"size":512,"module_shape":"rounded"},"output":{"format":"png"}}}
```

`output_root` is `workspace` by default or `desktop`. Workspace writes require the effective filesystem-write grant, including narrower run permissions. Desktop references require current agent control; mutations also require Desktop write permission. Local paths remain relative and rooted. Notes remain available only through the Notes tool. `logo_url`/`frame_image_url` require the effective network grant and accept only public, uncredentialed HTTP(S); redirects and connections receive SSRF checks.

Outputs are create-only. Replacing an existing QR output requires its exact returned SHA-256 `version` as `expected_version`. Revoked permission, cancellation, a stale version or an incomplete render prevents publication. Results contain relative artifact references and metadata, not base64 images in model context.

For Office insertion, generate a PNG, read the target document/workbook, then call `office_document.insert_image` or `office_workbook.insert_image` with `image_ref:{root,path}` and the target read's `office_version.etag` as `expected_version`. This Office version differs from a QR artifact hash. Writer's anchor is `end`; Tabellen uses a sheet name and A1 anchor. Width/height are 96-DPI pixels. The respective Office and Desktop write grants remain necessary. These file operations do not modify an unsaved editor draft. Pixel can open the generated Desktop image using the existing open-in-app operation; inserting into an existing unsaved layer stack uses the human dialog.

Full configurations and QR images contain their payload, including possible Wi-Fi passwords/contact/payment data. They are not encrypted. Save or share a full configuration only on explicit request. Runtime diagnostics exclude these values; normal chat retention still applies to content supplied to the agent. See the [tool manual](../prompts/tools_manuals/qr_code.md).

## Interfaces and limits

`internal/qrcode` owns versioned `QRSpec`, payloads, normalisation, rendering, scanning and serial CSV processing. HTTP and native tools own permissions, image URL imports, storage and publication. The shared dialog owns transient state; editor adapters own native insertion/history. The [implementation plan](../plans/2026-10-26-shared-qr-module.md) records the accepted scope.

The authenticated `/api/qr/` routes use existing Desktop scopes and CSRF boundaries: `GET presets`; `POST render`, `scan`, `batch`, `import-image`, `config`. Render/batch/import require Desktop writes. Scan/presets/config require reads. JSON requests reject unknown fields and trailing input; binary responses have no-store, nosniff and sandbox headers. Two shared expensive requests are admitted at once; excess requests receive `429 qr_busy`. None of these routes writes a generated file automatically.

| Resource | Ceiling |
| --- | --- |
| Final output | 4096 × 4096 pixels; 32 MiB encoded artifact |
| Image/config input | 10 MiB; raster images 32 megapixels; imported normalised images 2048 × 2048 |
| CSV | 5 MiB, 500 rows |
| Batch | 128 MiB combined output/ZIP; no partial publication |
| Validation preview | At most six PNGs, 160 pixels each, 1 MiB combined |
| Caption/config | 2048 characters, 128 lines; JSON nesting 16 |
| Processing | 15 seconds per render/scan/image normalisation; two minutes per serial batch |

Desktop storage may impose a smaller file limit. Version/QR capacity limits are also enforced, and unsafe visual settings can fail exact decoding rather than being silently changed.

## Verification

Focused core and permission suites: `go test ./internal/qrcode ./internal/office`; `go test ./internal/tools ./internal/agent ./internal/server -run '^TestQRCode'`. Parser fuzz targets live beside the core. Asset verification: `go test ./internal/webassets ./cmd/assetpack` and `node scripts/check-web-assets.mjs <stripped-binary>`.

After `npm ci`, set `AURAGO_RUN_BROWSER_SMOKE=1` and run `go test ./ui -run 'TestQRCodeIndependentDecoderBrowser|TestQRDialogWriterPixelBrowser|TestDesktopSheetsImagesBrowser'`. The independent fixture uses Chromium and pinned jsQR on final PNG/JPEG/SVG bytes, including multilingual captions and a dense vCard. Host fixtures exercise real Writer DOCX, Sheets XLSX and Pixel ORA save/reopen and history, stale drafts, read-only state, camera cleanup, both themes and all 16 locales. `npm run test:pixel` covers Pixel module regressions. Real cameras, paper output and a live LLM/provider run are separate acceptance checks.
