# Shared QR code core

## Purpose

Native, CGO-free QR payload serialization, styling, rendering, decoding, configuration and CSV/ZIP work for the shared Desktop dialog and native `qr_code` tool.

## Ownership

- `types.go` owns schema version 1 (`content`, `style`, `logo`, `frame`, `output`) and typed results. HTTP/tool adapters own authentication, grants, imports, paths, publication and conditional writes.
- `payload.go` and `classify.go` own template serialization and passive scan classification. `geometry.go`, `fonts.go` and `render.go` share the same layout/path representation for raster and SVG.
- `image.go` and `image_orientation.go` own bounded raster normalization, JPEG IFD0 orientation and restricted SVG input; `batch.go` owns CSV validation, safe ZIP names and complete row reports.
- `../../ui/fonts/qr/AGENTS.md` owns shipped fallback bytes/provenance. The root owns module dependency pins and third-party inventories.

## Local Contracts

- Pure byte inputs/outputs: no network, arbitrary file paths, shell/native graphics, browser, Node, Python, publication, payload logs or automatic actions on decoded data. Adapters supply fixed catalog font bytes through `FontProvider`.
- Keep one schema for human and agent entry points. Exactly one matching content template is required. Reject unknown/duplicate JSON fields, invalid options, non-finite numbers, unsupported versions and over-capacity payloads; never truncate or silently change correction.
- Templates: text, HTTP(S) URL, email (subject/body/CC/BCC), telephone/SMS (global, local and inline `ext`/`phone-context`), WiFi (`nopass`/WEP/WPA and hidden), vCard 2.1/3.0/4.0, geographic location, calendar and EPC 001/002. Bare local numbers produce `local_phone`; never infer a country. Raw text can contain an already serialized payload.
- vCards retain generic/work/private/mobile phone values and version-correct TEL types; vCard 4.0 uses text-valued TEL for bare local numbers. vCard 2.1 uses UTF-8 quoted-printable properties; 3.0/4.0 use escaped UTF-8 and byte-safe folding. `calendar.created_at` is initialized once by Normalize, stored in explicit configs and used for DTSTAMP. Adapters normalize once before render plus config export; editors retain it across previews. Calendar local times need an IANA zone; nonexistent/ambiguous local times fail, explicit offsets must match the zone, all-day ends are exclusive. UTC output avoids platform timezone dependence; stdlib timezone data is available for portable operation.
- EPC requires M, version <=13, <=331 UTF-8 bytes, checksum-valid IBAN/RF, exact decimal EUR 0.01–999999999.99 and mutually exclusive remittance fields. 001 and non-EEA beneficiary accounts require BIC. Never initiate a payment.
- QR correction comes from pinned gozxing; do not implement error correction. The general image detector may reject dense axis-aligned symbols, so scanning also tries upstream pure-symbol decoding on the actual image pixels.
- All selectable module/finder/center shapes, separate colors, quiet zone, alpha, rounded background, logo coverage/margin/clearance and four caption positions use shared geometry. SVG emits whole finder rings and integer module boundaries to avoid rasterization seams.
- Normal export succeeds only after exact PNG/JPEG byte decoding, or parsing/rasterizing serialized generated SVG, matches the complete payload. Text outputs verify the represented matrix and omit logo/frame art. Verification is software evidence, not camera/print acceptance. Transparent output is verified on white; JPEG flattens onto the explicit opaque JPEG background.
- Resource ceilings: 4096x4096 total output, 10 MiB input image/config, 32 megapixels raster input, 2048x2048 normalized imported image, 32 MiB encoded artifact, 5 MiB CSV, 500 logical CSV records, 128 MiB combined batch bytes and ZIP, 2048 caption characters/128 lines, JSON depth 16. Rendering/scanning/import each have a 15-second ceiling; a batch has two minutes and renders serially. Render admission is two; adapters share their own bounded admission for all expensive entry points.
- Restricted imported SVG supports one absolute-sized/viewBox root, groups, filled paths/rectangles/circles/ellipses/polygons/polylines, hexadecimal colors (plus black/white), numeric translate/scale/rotate/matrix transforms and per-shape opacity. Path commands include arcs. Reject scripts/events, external references, entities/DOCTYPE, styles, strokes, text, use/filter/foreign HTML, nested roots, group opacity and unsupported attributes. Limit 2048 elements, depth 16 and 30000 path commands; normalize to PNG. Do not pass untrusted SVG to a browser or shell renderer. Generated SVG has a separate internal profile permitting vetted embedded PNGs and bounded additional geometry.
- Raster imports accept PNG/JPEG/GIF/WebP; JPEG IFD0 orientation is applied after bounded downscaling and before PNG encoding; never follow GPS, thumbnail or next-IFD pointers. GIF animation becomes the first frame and all image metadata is stripped. Caption `sans`/`mono` work with Go fonts; `serif` and Noto Arabic/Devanagari/CJK/monochrome-emoji fallbacks require verified local resources. Missing glyphs fail explicitly. CJK regional glyph forms follow the shipped Simplified Chinese face.
- Config export is explicit and includes payload/caption/resource bytes; it is not encrypted. Presentation presets contain no payload or caption. No automatic persistence occurs here.
- CSV accepts text/URL/vCard columns plus `caption`, `font`, `filename` and optional `type`. Unknown/duplicate columns fail. Filenames reject traversal, separators, control/format characters, Windows device names and case/normalization collisions. No incomplete archive is returned. Dry run reports every row and includes <=6 PNG previews, <=160 pixels, <=1 MiB combined.
- All validation/diagnostic messages are static and must not contain SSIDs, passwords, payment/contact data, decoded text or imported file values.

## Work Guidance

Use existing Go/image primitives and the fixed catalog. Add no external QR service or second renderer. Preserve typed errors and cancellation before returning data. Prefer rejection over unsupported visual changes or missing-glyph boxes.

## Verification

- `CGO_ENABLED=0 go test ./internal/qrcode -count=1` covers all payloads, correction levels, shapes, framed/logo raster/SVG output, local fonts/provenance, strict config, image safety, batch/archive rules, cancellation and privacy.
- Fuzz targets: `FuzzParseConfig`, `FuzzPayload`, `FuzzBoundedCSV`, `FuzzClassify`, `FuzzJPEGOrientation`; run bounded campaigns after parser changes.
- Independent renderer/decoder acceptance belongs to the coordinator's browser integration fixture (Chromium plus jsQR); it must decode final PNG/JPEG/SVG artifacts. Core self-decoding alone is insufficient acceptance. Phase-zero prototypes additionally passed independent sharp/libvips rasterization.
- Real camera and printed-page checks remain separate and must not be claimed when no device was used.

## Child DOX Index

- Cross-component resource owner: `../../ui/fonts/qr/AGENTS.md`.
