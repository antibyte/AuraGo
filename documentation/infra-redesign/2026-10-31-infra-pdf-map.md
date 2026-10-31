# Infra: homelab map as PDF (2026-10-31)

User request: "die infra app soll eine karte des homelab als pdf erstellen können. schön aufbereitet und optimal auf
eine oder mehrere seiten verteilt mit wunderschöner darstellung".

## Decisions

- **Server-side vector PDF** with `github.com/phpdave11/gofpdf` (already used by `internal/newspaper`,
  `internal/detective`). No browser, no external service, vector output, unit-testable, reusable later by the agent.
- **Source of truth is the Map (atlas) view**: the user's own arrangement of places (site/room/rack/group regions) and
  cards. Unplaced top-level assets are not dropped: they get an auto-laid-out "Not on the map" block.
- **Same data as the app**: `store.Snapshot` + `desktopInfraDecorateSnapshot` (visible nodes, host projection, power).
- **Language**: server UI language (`s.Cfg.Server.UILanguage`) through `i18n.T` with keys `desktop.infra_pdf_*` in all 16
  `ui/lang/desktop/*.json`; English defaults in Go (like `internal/persondetect/alerts.go` `alertDefaultTexts`).
- **Landscape pages** throughout (maps are wide, tables get more columns). Paper: A4 (default), A3, Letter.
- **Sections** (all on by default): map (overview + detail pages), hosts & guests, inventory + connections.

## Package layout

New package `internal/infra/atlaspdf` (rendering only, no store/server imports beyond `aurago/internal/infra` types):

```go
type Paper string // "a4", "a3", "letter"
type Options struct {
    Paper    Paper
    Map, Hosts, Inventory bool       // sections; all false is invalid -> caller defaults to all true
    Text     func(key string) string // translation; blank or key itself -> English default
    Icons    fs.FS                   // root containing "img/infra/<id>.webp"; nil or missing file -> vector fallback
    CJKFonts fs.FS                   // namespace with NotoSansSC-Regular.ttf / -Bold.ttf (newspaper/fonts); may be nil
    Now      time.Time
    Title    string                  // optional; default label "Homelab map"
}
func Render(snapshot infra.Snapshot, opts Options) ([]byte, error)
```

Files (suggested): `atlaspdf.go` (Render, options, document chrome), `scene.go` (geometry port), `paginate.go` (page
planning), `draw_map.go`, `draw_hosts.go`, `draw_tables.go`, `style.go` (palette, fonts, text fitting), `icons.go`
(webp -> PNG cache), `labels.go` (keys + English defaults), tests.

Bounds: at most 5000 nodes / 10000 edges rendered (more -> `ErrTooLarge`); strings truncated to fit cells; never panic
on odd data (NaN placements, cycles in parent ids, missing hosts).

## Geometry (port of `ui/js/desktop/apps/infra-model.js` `buildScene`, world units = canvas px)

- `CARD_W=216, CARD_H=64`; a card's placement `x,y` (view `atlas`) is its **centre**.
- Place kinds `site, room, rack, group` are regions: placement `x,y` = top-left, `w,h` (0 -> 360×240). Drawn rect =
  union(stored rect, every child card rect and nested region rect padded by `REGION_PAD=48`), computed bottom-up by
  `parent_id` (cycle-safe).
- Guests: `vm`, `container`, `service` whose `host_id` (or first sorted `runs_on` edge target) is another node ride in
  their host's card (no card of their own). Mutual hosting -> drop the larger id's link (same as JS `guestIndex`).
- Lines: every edge except `runs_on` that repeats the nesting; an endpoint that is a guest maps to its host; skip when
  both ends map to the same shape or an end is not drawn.
- **Unplaced block**: top-level nodes that are not guests, not places, have no atlas placement and are not hostless
  `vm`/`container` (those go only to the inventory table) are laid out in a grid of cards (240×96 cells, columns =
  ceil(sqrt(n)) capped by aspect ~ page ratio) placed 160 units below the scene bounds, framed as a dashed pseudo-region
  titled `desktop.infra_pdf_unplaced`. If nothing is placed at all, the block is the whole scene (no dashed frame,
  arranged grouped by kind: networks, devices, services).

## Page planning ("optimal on one or more pages")

Units mm. Page margins 12; header band 10 (top), footer 8. Map area = rest. Scale `s` in mm per world unit.

- `readable` = card width ≥ 34 mm (s ≥ 34/216); `compact` threshold 16 mm.
- **Overview page** (always when Map section is on): title block + stat tiles at the top (~34 mm), legend strip at the
  bottom (~9 mm), map in between, fitted with aspect preserved, centred, s capped at 0.30 (so tiny labs do not look
  bloated). If `s ≥ readable` the overview is the only map page and draws full cards.
- Otherwise the overview draws **compact cards** (icon + one-line name, ≥ 5 pt; below compact threshold: coloured
  dot-pill with name if it fits, else dot only) and region headers, and **detail pages** follow:
  1. Partition into clusters: each top-level region (depth 0) is a cluster; loose top-level cards are grouped into
     clusters by proximity (single-link, gap ≤ 600 units); the unplaced block is its own cluster.
  2. Greedy packing: sort clusters by y then x; try to put neighbouring clusters on one page when their combined
     bounding box still fits at `readable` scale (merge while it fits) — fewer pages, related things together.
  3. A cluster that does not fit one page at `readable` is tiled: choose cols×rows (cols,rows ≤ 6) minimising page
     count with s ≥ readable for the page aspect; tiles overlap by 4 % and are clipped; tile pages are titled
     "<region> · Part 2/4".
  4. Each detail page scale = min(fit, 0.42) so a small cluster is not blown up beyond ~1.9× readable.
- **Locator inset** on each detail page (bottom-right, 52×34 mm, white with 0.3 pt border): whole scene as region
  outlines + grey card dots, the page's world rect highlighted in accent colour.
- **Off-page connections**: an edge with one end on the page and the other not drawn there is drawn from the card
  border toward the other end, clipped at the map-area border with an arrow head and a small pill label
  "→ <name> · p. N" (N = page where the other end appears first; overview excluded). Two-pass: plan pages first,
  then draw.
- Page order: overview, detail pages (in cluster order), hosts & guests, inventory, connections.
- Every page header: left "<title> · <section/page title>", right date; footer: centre "Page N / M", left
  "AuraGo Infra", right generation timestamp. Use gofpdf `AliasNbPages`.

## Visual design

Fonts: Go Regular / Go Medium / Go Bold for text, Go Mono for IP/MAC/ports (`golang.org/x/image/font/gofont/...`). If
any text (labels + names) is not covered by the Go fonts, use NotoSansSC (regular for all text styles that are
missing, bold for bold) from `CJKFonts`; still uncovered runes become `?` (do not fail). If the translated labels
themselves are not coverable (e.g. Devanagari, unshaped by gofpdf) use the English labels.

Paper is white; the map area gets a very light dot grid (dots 0.18 mm, #E3E7EE, every 6 mm) clipped to the map area.

Palette (RGB hex):

| use | colour |
| --- | --- |
| ink / headings | #111827 |
| secondary text | #4B5563 |
| tertiary text | #8A93A3 |
| hairlines | #D9DEE7 |
| accent (title rule, locator highlight) | #4F46E5 |
| device | #4F46E5 |
| vm | #7C3AED |
| container | #0891B2 |
| service | #D97706 |
| network | #059669 |
| site region fill / stroke | #F5F7FB / #CBD3E1 |
| room region fill / stroke | #EEF6FF / #B9D4F2 |
| rack region fill / stroke | #F6F4F0 / #D6CFC2 |
| group region fill / stroke | #FFFFFF / #C9CFDA (dashed 1.2/0.9) |
| power running / paused / stopped / unknown | #16A34A / #F59E0B / #9CA3AF / #D1D5DB |

Edges: ethernet solid #64748B 0.45 pt; fiber solid #EA580C 0.6 pt; wifi dashed #0EA5E9 0.45 pt (1.4/1.0);
depends_on dotted #9CA3AF 0.35 pt; powered_by dashed #DC2626 0.35 pt (0.8/0.8); member_of solid #CBD5E1 0.3 pt; other
#94A3B8. Curves: cubic Bézier between the two shape borders (control points 40 % along the dominant axis, like the
app's `edgeCurve`); drawn under cards. Edge label (port/speed/label) as a white pill at the midpoint only when
s ≥ readable and the label fits.

Region: rounded rect r=3 mm, fill + stroke 0.4 pt; header inside top-left: place icon 5 mm + name (Go Bold, 8.5 pt
scaled with s, clamp 6–11 pt) + muted kind label and asset count on the right side of the header ("Rack · 7").
Nested regions get the next fill so depth stays readable.

Full card (s ≥ readable): rounded rect r=2 mm, white fill, 0.35 pt stroke #D9DEE7, 1.4 mm left colour bar in kind
colour, soft shadow (same rect offset 0.5 mm in #EEF0F4 drawn first). Icon (webp illustration) square at left, height
= 70 % of card height. Text column: name (Go Bold, size from card height, ellipsis), second line IP (Go Mono) or role
/ model (tertiary colour), third line guest summary pill(s) "12 containers · 2 VMs" in kind colours when the card has
guests. Power dot (1.6 mm, with white 0.4 mm ring) at the card's top-right when the node has power.
Compact card: pill with icon disc + name only.

Overview title block: "Homelab map" (Go Bold 22 pt) + subtitle "<date> · <N> assets" (secondary). Stat tiles in a row
(right of title or below on narrow paper): devices, VMs, containers, services, networks, connections, running — each
tile 26×16 mm, white, hairline border, 1 mm top bar in kind colour, big number (Go Bold 14 pt) and small label.
Legend strip: kind swatches, edge styles (short line samples), power dots.

**Hosts & guests pages** (title `desktop.infra_pdf_hosts_title`): one card per host that has guests, sorted by guest
count desc then name. Masonry into 3 columns (A3: 4) with 6 mm gutters: card header (icon 9 mm, name bold 10 pt,
IP mono, OS / CPU / RAM line, place path "Site › Room › Rack" tertiary), then guests as rows (2 sub-columns when > 8
guests): power dot, kind micro-icon (4 mm webp), name, IP or first port, kind colour text. A card taller than the
remaining column height continues in the next column/page with "(continued)" header. Greedy shortest-column placement
with cards sorted tallest first gives balanced pages. Hostless VMs/containers listed in a final card
`desktop.infra_pdf_hostless`.

**Inventory table** (title `desktop.infra_pdf_inventory_title`): grouped by place path (top-level places first, "No
place" last); group heading row with place icon + path; columns: name (with 4 mm icon), kind, address (mono), MAC
(mono), host, OS / model, sources, power. Zebra rows #F8FAFC, header row ink on #EEF1F6 repeated on each page,
column widths proportional for the paper. **Connections table**: from, port, to, port, kind (coloured line sample),
speed, label.

Document metadata: title, author "AuraGo Infra", subject, creator; compression on.

## Icons

`Icons` fs: read `img/infra/<icon>.webp` (unknown icon -> `generic-device`, places use `site`/`room`/`rack`/`group`),
decode with `golang.org/x/image/webp`, downscale to 128 px (good-quality box/bilinear via `golang.org/x/image/draw`
CatmullRom), encode PNG with alpha, register once per document (`RegisterImageOptionsReader`, name = icon id + size).
Process-wide cache (`sync.Map` of id -> png bytes) is fine. Missing/undecodable -> vector fallback: kind-coloured
rounded square with the kind initial in white.

## HTTP

`GET /api/desktop/infra/export/pdf?paper=a4|a3|letter&sections=map,hosts,inventory` (any subset; empty -> all; unknown
values -> 400 `invalid_request`). Read access like `snapshot` (`desktopInfraAccess(..., write=false)`), no provider
refresh. Builds `store.Snapshot(ctx)` + `desktopInfraDecorateSnapshot`, renders with `Text = i18n.T(lang, key)`,
`Icons = uiFiles`, `CJKFonts = webassets.Namespace("newspaper/fonts")`. Response 200 `application/pdf`,
`Content-Disposition: attachment; filename="homelab-map-YYYY-MM-DD.pdf"`, `Cache-Control: no-store`. Render error ->
500 JSON error (no partial body). Route case in `handleDesktopInfra` (`path == "export/pdf" && GET`).

## UI

- More menu (`[data-menu="more"]` in `ui/js/desktop/apps/infra.js`): new item `data-action="export-pdf"`
  "Export map as PDF…" (always enabled, also read-only).
- Opens a small dialog (existing dialog host / `infra-dialogs.js` patterns): paper radio (A4 / A3 / Letter) and three
  checkboxes (map, hosts & guests, inventory & connections), primary "Create PDF", secondary "Cancel". Create is
  disabled when no section is checked. While busy: button shows "Creating PDF…", `aria-busy`.
- Fetch the endpoint (`credentials: 'same-origin'`), on success download the blob as the server's filename (see
  `newspaper.js` download pattern) and show a success toast; on error show the server's error message toast.
- Remember the last paper/sections per browser in `localStorage` (try/catch).

## Locale keys (all 16 `ui/lang/desktop/*.json`, real translations)

UI: `desktop.infra_export_pdf` (menu + dialog title), `desktop.infra_export_pdf_paper`, `desktop.infra_export_pdf_sections`,
`desktop.infra_export_pdf_section_map`, `desktop.infra_export_pdf_section_hosts`, `desktop.infra_export_pdf_section_inventory`,
`desktop.infra_export_pdf_create`, `desktop.infra_export_pdf_busy`, `desktop.infra_export_pdf_done`,
`desktop.infra_export_pdf_failed`.

PDF (English defaults in `labels.go`):

| key | en |
| --- | --- |
| desktop.infra_pdf_title | Homelab map |
| desktop.infra_pdf_assets | {count} assets |
| desktop.infra_pdf_overview | Overview |
| desktop.infra_pdf_part | Part {n}/{total} |
| desktop.infra_pdf_unplaced | Not on the map |
| desktop.infra_pdf_page | Page {n} / {total} |
| desktop.infra_pdf_page_ref | p. {n} |
| desktop.infra_pdf_generated | Generated {time} |
| desktop.infra_pdf_stat_devices | Devices |
| desktop.infra_pdf_stat_vms | VMs |
| desktop.infra_pdf_stat_containers | Containers |
| desktop.infra_pdf_stat_services | Services |
| desktop.infra_pdf_stat_networks | Networks |
| desktop.infra_pdf_stat_connections | Connections |
| desktop.infra_pdf_stat_running | Running |
| desktop.infra_pdf_legend | Legend |
| desktop.infra_pdf_hosts_title | Hosts & guests |
| desktop.infra_pdf_hostless | Without host |
| desktop.infra_pdf_continued | continued |
| desktop.infra_pdf_guests | {count} guests |
| desktop.infra_pdf_inventory_title | Inventory |
| desktop.infra_pdf_connections_title | Connections |
| desktop.infra_pdf_no_place | No place |
| desktop.infra_pdf_col_name | Name |
| desktop.infra_pdf_col_kind | Type |
| desktop.infra_pdf_col_address | Address |
| desktop.infra_pdf_col_mac | MAC |
| desktop.infra_pdf_col_host | Host |
| desktop.infra_pdf_col_system | System / model |
| desktop.infra_pdf_col_sources | Sources |
| desktop.infra_pdf_col_power | Power |
| desktop.infra_pdf_col_from | From |
| desktop.infra_pdf_col_to | To |
| desktop.infra_pdf_col_port | Port |
| desktop.infra_pdf_col_speed | Speed |
| desktop.infra_pdf_col_label | Label |
| desktop.infra_pdf_empty | No assets yet. Add devices in the Infra app to draw your map. |

Kind, edge-kind and power names reuse the existing keys `desktop.infra_kind_<kind>`, `desktop.infra_edge_<kind>` (check
the real key names in `en.json`) and `desktop.infra_power_<power>`; the Go side needs English defaults for those too.

## Tests (essential only)

- `atlaspdf`: geometry (region union bottom-up, guest folding, unplaced block), page planning (small lab -> 1 map page;
  large lab -> overview + detail pages, every placed card appears on ≥ 1 detail page, tile scale ≥ readable), Render
  smoke on a realistic fixture (valid `%PDF`, page count > 1, parses with `github.com/ledongthuc/pdf` or pdfcpu and
  contains the title text), empty snapshot renders the empty page, bad data (NaN, parent cycle) does not panic.
- Preview helper: `INFRA_PDF_PREVIEW=<dir>` makes the fixture test write `small.pdf` and `large.pdf` (A4 + A3) there,
  icons from `os.DirFS("../../../ui")`.
- Server: route test for 200 + content type + 400 on bad paper; locale test that every `desktop.infra_pdf_*` /
  `desktop.infra_export_pdf*` key exists in all 16 desktop locales.
- UI: extend `scripts/test-infra-ui.mjs` only if a cheap check fits (menu item present, dialog disables Create with no
  sections).
