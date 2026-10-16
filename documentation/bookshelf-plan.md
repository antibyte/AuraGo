# Bookshelf desktop app plan

Status: proposed product and implementation plan, 2026-10-16. No application
code has been implemented. The user selected the third displayed concept,
**The Reading Salon**, as the visual target. The reference is an illustration,
not a screenshot of a working app: [selected design](../assets/bookshelf/design-reference.png).

## Product contract

Bookshelf is a built-in Virtual Desktop app for collecting, organizing and
reading ebooks. The bookshelf is the primary experience: beautiful covers,
convincing materials, excellent typography and fast, predictable interaction.
Library management and reading must remain useful with thousands of books.

The user explicitly selected **library management without modifying original
files**. Collections, metadata overrides, covers, bookmarks and reading state
belong to Bookshelf. Source files are never renamed, moved, rewritten or deleted.
Removing a book removes its library membership; removing a source preserves
reading history until the user explicitly removes that Bookshelf data.

Required sources are local files, WebDAV, SMB and Koofr. A source can be linked
without importing its entire contents. Bookshelf has a default-off activation toggle and
uses Desktop administrator authorization. No new agent tools, model calls,
cloud account, book store or external metadata service are required.

## Shelf and interaction design

The chosen material direction is applied consistently inside the app; Standard
and Fruity keep their own native window chrome, menus, icons and controls.
Ship one selected shelf design, not three independent themes.

The selected wide-window composition pairs a warm, open-book reading surface
on the left (about 38%) with a dark smoked-oak shelf on the right (about 62%).
Soft shelf lighting, shallow cover depth, quiet copper accents and parchment
contrast define its character. Two face-out rows and an optional compact spine
row establish the reference hierarchy. Spines remain accessible book targets,
not unreadable decorative substitutes. Avoid scaling this entire image as a UI.
Text, controls, book objects and shelf rows must be real responsive components.

The reading preview shows the selected or last-read book; before any book has
been read, use a welcoming empty page with Add books. Full reader mode expands
the page to a comfortable line length and hides the shelf. Below the width needed
for two useful panes, switch between library and reader while preserving both
states. A narrow screen never squeezes text into the reference's 38% column.

| Element | Design and behavior contract |
| --- | --- |
| Book objects | Original embedded or user-selected covers, uncropped and aspect-correct. Subtle spine, paper edge, binding highlight and contact shadow create depth. Preserve genuine cover artwork; do not generate replacement covers automatically. |
| Shelves | Shallow perspective, coherent light direction and restrained texture. Covers and readable titles remain dominant. Use layered local assets and CSS; a WebGL scene is unnecessary for the core shelf. |
| Layout | Compact top toolbar with library/collection navigation, search, Add books and Sources. The left reading surface provides Continue reading; the right shelf scrolls vertically. Large covers by default; a density control and compact list handle large collections. |
| Book identity | A real text title and author remain accessible outside the cover image. Long titles have bounded wrapping and a full accessible name. Missing covers receive a deterministic typographic cover with title and author. |
| Primary action | Activating a book opens or resumes it. A separate, clearly labelled details action exposes metadata and collections. No mandatory double-click, hover-only control or drag gesture. |
| Details | Side panel on wide windows; full-width sheet on narrow windows. Show formats, source, availability, description, metadata overrides and cover choice. One obvious Read/Continue action. |
| Search | Search title, author, series and tags; optional format, source and reading-state filters live in a compact filter area. Preserve the search, scroll anchor and selected book when returning from reading. |
| Collections | User-created shelves with explicit ordering. Drag-and-drop is optional convenience, with equivalent Move up/down and Add to collection controls. A book may belong to several collections. |
| Motion | Short 120–180 ms focus/selection transitions and a restrained reader transition. No idle motion, forced page-curl animation or layout movement while covers load. Reduced motion removes spatial animation. |
| Responsive behavior | Design reference 1440 x 1024; verify 1280 x 800, 1024 x 768 and 390 x 844. Collapse sidebars before shrinking text; preserve 44 px touch targets. Shelf rows adapt to window width. |
| Accessibility | Keyboard navigation, visible focus, logical reading order, named controls, adequate text contrast and text-backed status. Zoom to 200% without losing core actions. Virtualization must preserve focus and screen-reader access. |

The generated concepts establish composition and material quality only. Product
name remains **Bookshelf**; incidental image text, fabricated publisher marks
and unrelated menu labels are not implementation requirements. Production
textures need provenance; book illustrations in the concepts are not bundled
sample content.

## Reading experience

- Reflowable books: paginated or continuous reading, font family/size, line
  height, text width, margins and light/sepia/dark reading surfaces. Keep the
  reader palette independent of the Desktop theme. Respect document language,
  right-to-left progression, footnotes and original chapter structure.
- Fixed-layout books, PDFs and comics: fit page/width, zoom, thumbnails,
  single-page/spread modes and explicit reading direction. Do not pretend
  fixed-layout documents can use reflow typography controls.
- All applicable readers: table of contents, in-book search, bookmarks, resume
  position, fullscreen and keyboard/touch controls. Text highlights and notes
  use supported text locators; image-only comics receive page bookmarks rather
  than fake text selection. Search in scanned pages requires an existing text
  layer; OCR is outside the initial scope.
- Save a semantic locator, not merely a percentage: EPUB CFI or an equivalent
  section/text anchor, PDF page plus normalized position, comic page index.
  Include the rendition revision. Font or window changes must not lose place.
- Persist progress through a serialized, revisioned save queue. Older windows
  cannot silently overwrite newer progress, and closing a reader cannot publish
  state for a different book. Conflicts offer the saved position or the current
  session position; do not choose the largest percentage automatically.
- Reader open, preparation, conversion, cancellation, malformed book, unsupported
  protection and lost source each have distinct feedback. Display saved versus
  unsaved progress when persistence fails.

## Format support and dependency decision

Support is a tested capability matrix, not an unconditional claim to open every
file extension. Detect the actual container and protection state before selecting
a reader. Different formats of the same title can share catalog metadata but
retain separate rendition locators.

| Format | Planned reading path | Acceptance boundary |
| --- | --- | --- |
| EPUB 2 and EPUB 3 | Pinned, locally vendored foliate-js | Test reflow, fixed layout, fonts, SVG, footnotes, navigation and RTL. Scripted publications are deliberately unsupported. |
| PDF | Existing locally packaged PDF.js | Preserve page layout, text layer, search, annotations/bookmarks and required fonts/CMaps/WASM. Password prompts are session-local; do not persist passwords. |
| MOBI, KF8/AZW3 | foliate-js | DRM-free fixtures, including mixed MOBI/KF8 containers. Bound decompression and handle unsupported variants explicitly. |
| AZW | Inspect the container, then supported MOBI/KF8 path or conversion | Extension alone does not establish readability. No blanket Kindle/KFX claim. |
| FB2 | foliate-js | Test metadata, cover and Cyrillic content. FB2.ZIP additionally needs a validated bounded archive wrapper; mark it unsupported until its fixture passes. |
| CBZ | foliate-js comic support | Natural page order, mixed image dimensions, direction, spread handling and image budgets. |
| TXT, HTML/HTM | Small adapter to the same reflow reader | Encoding detection with a user correction option; HTML defaults to a self-contained document. No live web-page execution. |
| CBR, RTF, DOCX, ODT and legacy ebook containers | Explicit optional local conversion to a managed EPUB/PDF copy | The converter must be enabled and available. Source stays unchanged; display that this is a converted rendition and may differ from the original. |
| DjVu | Separate evaluated capability, not a generic ebook promise | Text-bearing conversion does not preserve arbitrary image-only scans. Full DjVu rendering needs its own validated engine; show unsupported until that exists. |
| DRM-protected EPUB/Kindle, ACSM, unsupported KFX | Visible unsupported-protection/format state | No DRM removal, account scraping or claim that an ACSM license is a book. Retain catalog records where possible. |

Recommended baseline: **foliate-js plus the existing PDF.js**. foliate-js exposes
multiple book parsers and reading primitives; AuraGo supplies its own reader UI.
Its API is not stable, so pin a reviewed commit, inventory transitive assets and
keep an upgrade corpus. epub.js remains an EPUB-only fallback if the first spike
rejects foliate-js; shipping both is not the default design.

Calibre's `ebook-convert` is the candidate for optional conversion. It is a
separate runtime, never a mandatory dependency for ordinary EPUB/PDF reading.
Use an explicitly configured installation or a separately enabled, digest-pinned
managed image compatible with AuraGo's existing pull-only Docker default. Do not
install packages, build images or run conversions on passive status checks.
No shell interpolation or book-supplied command options. Jobs need bounded
resources, private staging, no network, cancellation and atomic publication.
If safe execution is unavailable on a platform, explain the limitation and offer
import of a previously converted file. Review redistribution obligations before
bundling a converter. Final runtime mode is chosen in the compatibility spike.

This format design is based on current primary documentation:
[foliate-js](https://github.com/johnfactotum/foliate-js),
[PDF.js](https://mozilla.github.io/pdf.js/getting_started/),
[calibre format support](https://manual.calibre-ebook.com/faq.html#what-formats-does-calibre-support-conversion-to-from)
and [ebook-convert](https://manual.calibre-ebook.com/generated/en/ebook-convert.html).

## Source model and synchronization

Distinguish files on the AuraGo server from files on the browser user's device.
Server-local books use authorized Desktop roots. A browser file picker or drop
uploads an explicit copy into managed Bookshelf imports; it does not give the
server general access to the user's computer. Preserve the user's original.

Reuse configured WebDAV and Koofr connections and Vault credentials. Keep
`provider`, `mount_id` and relative `path` separate; never flatten remote paths
into local editor APIs. Koofr includes all readable connected mounts. The current
Desktop storage visibility/grant is honored rather than bypassed with a second
set of credentials. Source setup explains disabled integration, missing grant,
expired authorization and temporarily unavailable storage separately.

SMB is a required new client capability: no existing client was found in
`internal/` or `go.mod`. The `internal/networkshares` service manages shares
exported by the host and cannot browse a remote library. Add a bounded read-only
adapter backed by a reviewed pure-Go SMB2/3 library, selected and pinned in the
spike. A mounted-share-only workaround is insufficient for the source chooser.
Configure server/share/root and Vault credentials separately from host share
publication. No SMB1, automatic LAN discovery, guest fallback or silent security
downgrade. Verify authentication, signing/encryption capabilities, cancellation,
path containment and NAS interoperability before committing to that dependency.
Adding an SMB source may make one explicit bounded connection test; passive
capability displays do not probe the network.

Source onboarding: choose a configured location, select a folder, choose whether
to include subfolders, preview recognized files, then explicitly add/scan.
Browsing source choices is passive. Initial scans are cancellable, incremental
and resumable; one provider's failure does not block other sources or existing
library reads. Default refresh is explicit; scheduled refresh is opt-in and uses
the same bounded job path.

Discover filenames first and enrich metadata/covers in a bounded queue. Do not
call `StatStoragePath` for every local listing merely to discover it: it hashes
the entire regular file. Reuse rooted listing metadata for discovery. Prioritize
visible and recently read books. Do not download every large remote book simply
to display the shelf. EPUB/FB2 metadata and covers can use bounded ZIP/XML parsing;
PDF metadata reuses available PDF infrastructure. Other parsers and on-demand
browser-derived metadata are evaluated in the spike, validated on receipt and
never allowed to overwrite user overrides automatically.

Use source identity plus a provider revision for change detection. Compute a
content digest opportunistically when bytes are already read. ISBN/title is a
merge suggestion, not a unique key. Offer duplicate grouping without silently
merging editions, translations or reading progress. A source rescan never erases
books because a listing was partial, timed out or denied. Mark absent entries
only after a successful complete scan; keep history and manual overrides.

## Content preparation and cache

All source traffic stays server-side and inherits its source policy and context.
The browser receives a Bookshelf book/rendition ID, not a credential, arbitrary
remote URL or host path. Recheck live authorization before opening content and
on active source revocation, including for cached copies.

Prefer local random-access file handles for PDF and ZIP readers. Existing
`CloudStorage.Open` and Desktop remote content handlers stream whole files;
the HTTP route has `no-store` and does not forward ranges. WebDAV applies
`If-Match`, but Koofr content reads do not currently enforce the metadata hash.
Use a bounded cache instead of claiming these handlers already support random
access. Spool one source version to private disk, verify complete
length and revision, then publish the ready file atomically and serve local HTTP
ranges. Deduplicate concurrent preparation of the same source version; closing
one reader releases its lease without cancelling another reader's active use.
Never present a truncated or changing download as a complete book. For Koofr,
compare fresh metadata before and after download and validate a content hash
when the provider's documented hash semantics permit it. If no strong version
can be proven, keep it as an explicitly version-unverified snapshot, not a fresh
shared cache hit. User-visible pinned copies state when they were fetched.

Cache keys include source identity, source generation, content revision and
conversion engine version. LRU eviction skips active leases and explicitly pinned
copies. Show preparation progress and cache size; reject work before disk space
is exhausted. Start with a proposed 1 GiB cache budget, two source transfers and
one conversion job; tune in the spike. Existing stricter Desktop/provider limits
still win. Metadata-only indexing must have smaller independent byte budgets.

Name source-unavailable cache functionality **Keep on this AuraGo server**.
It works while the browser can reach AuraGo. This is not browser-offline support.
Do not cache private book bytes in the global service worker or promise reading
after AuraGo is unreachable. Device-offline storage, quota and logout erasure
would be a separate explicit feature.

## Persistence, authorization and untrusted books

Use a small `internal/bookshelf` service and SQLite catalog under an explicitly
ignored `data/bookshelf/` directory. Keep original imports, derived covers,
converted copies and disposable cache separate. Proposed records: sources,
books, renditions/locations, collections/membership, metadata overrides,
reading state and annotations. Use the current Desktop installation scope;
do not invent multiuser isolation that the host does not provide. Catalog and
annotations are backup data; disposable cache is not.

Every schema change has a version, backup-before-migration and staging migration
test. Resetting cache must preserve catalog/history. Source credentials stay in
the existing Vault. No book content or private library metadata is sent to an
LLM or external metadata service by default.

Source read-only grants allow reading and ordinary Bookshelf catalog edits where
Desktop permits them. Global Desktop read-only blocks catalog/progress/annotation
mutations and explicit persistent imports; communicate this without blocking
permitted reads. Disable/revocation cancels source jobs and prevents stale results
from publishing. Private temporary read preparation follows the established read
path policy; it is not a loophole for persistent user-file writes.

Books are untrusted documents. Before accepting the chosen reader, prove a
restricted rendering boundary with CSP, sanitized markup/resources and an
isolated frame. Do not grant executable book content both scripts and same-origin
privileges. A trusted shell mediates authorized resources and a narrow,
document-bound message channel; book content cannot call Desktop APIs, navigate
the parent, open popups or fetch tracking resources. Strip scripts, event handlers,
forms and external loads; validate SVG, CSS URLs and embedded fonts too.
Explicit external links leave the reader through the existing safe-link flow.
Reader isolation is a release gate, not an assumption about foliate-js defaults.

For HTML, accept a standalone document with validated embedded resources by
default. Linked images/styles require the user to add an explicit publication
folder or archive. Resolve every relative resource beneath that selected root;
reject traversal, symlinks, absolute/drive paths and network URLs. A loose HTML
file never implicitly authorizes its parent directory or the rest of a share.
Render missing resources with a clear explanation instead of fetching them.

Archive/XML/image parsing needs file-count, entry-size, aggregate expansion,
compression-ratio, depth, pixel and time limits. Reject traversal, links, duplicate
normalized entries and malformed containers. Cancellation frees workers, buffers,
object URLs, renderer documents and temporary files. Cover endpoints return
validated raster data rather than active source SVG/HTML. No private book data
belongs in diagnostic logs or committed fixtures.

## Implementation sequence and acceptance

| Phase | Deliverable | Exit condition |
| --- | --- | --- |
| 0. Visual and reader spike | Translate the selected third concept into a disposable shelf interaction study and a securely isolated representative reader; verify SMB client and converter mode | Browser comparison of shelf quality, resizing, keyboard/touch and reduced motion; EPUB/PDF/MOBI/CBZ corpus and malicious-book tests establish viable engines. Record unresolved formats honestly. |
| 1. Catalog and local sources | Activation/config, SQLite, rooted imports/links, metadata/cover queue, source scans, collections and duplicates | Original bytes unchanged; interrupted scans retain catalog; migration and concurrent edits tested. |
| 2. Polished shelf | Selected materials, cover rendering, search, sorting, details, collections, loading/empty/error states and compact view | Standard and Fruity light/dark at all target sizes, 200% zoom, keyboard and screen-reader review; missing covers and long multilingual metadata remain polished. |
| 3. Complete reader | EPUB/PDF/MOBI/FB2/CBZ/TXT/HTML paths, locations, contents/search, bookmarks/highlights/notes as applicable | Resume survives restart, resize and typography changes; simultaneous readers cannot overwrite each other's state; malformed/protected files have actionable feedback. |
| 4. All remote sources | WebDAV, SMB and Koofr folder selection, incremental scans, authenticated content preparation and cache | Each provider passes real list/read/reconnect tests; readonly, revocation, source changes, partial downloads, cancellation and inaccessible NAS tested. No new source mutations. |
| 5. Conversion and release polish | Optional extended formats, final matrix, all locales, packaging and performance | Conversion corpus, resource limits and cancellation; local resource integrity and vendor checks; visual review and measured performance targets satisfied. |

These are implementation increments of the requested app, not permission to call
an EPUB/PDF-only local prototype feature-complete. Complete delivery includes all
three named remote providers and the declared supported-format paths.

Proposed performance acceptance uses a fixed recorded reference machine and
browser: a 10,000-record catalog with only viewport-adjacent cover rows mounted,
cached first shelf visible within 1 second, metadata search within 200 ms after
debounce, and responsive scrolling without repeated long tasks over 50 ms.
These are targets, not measured results. Large-book memory remains bounded by
explicit limits and the active page/chapter window. Reading starts independently
of a full-library scan.

Verification combines Go service/handler tests, deterministic reader fixtures,
browser flow tests, accessibility inspection and side-by-side visual review.
Include source timeout/auth failure, crash/restart, offline source with cached
copy, stale revision, disk full, large comics, corrupted archives and active
policy changes. Keep all 16 locales translated and test Standard/Fruity.
Run relevant UI regressions, `node scripts/build-ui-bundles.js --check`, vendor
checks and web-resource packaging checks. Live NAS/WebDAV/Koofr acceptance and
deployment remain separate from local fixtures.

## Existing integration points

- `internal/desktop/types.go::BuiltinApps`: built-in manifest, settings and icon allowlist.
- `ui/js/desktop/core/module-loader.js` and `menus-and-routing.js`: lazy app
  loading, translations, menus, context and file routing; use the existing
  per-window render/dispose lifecycle.
- `desktop-foundation.js` and `window-shell-runtime.js` under the same core
  directory: global app/disposal mapping, window reuse, close handling and bounds.
- `ui/js/desktop/apps/editor-filemenu.js`, File Manager `core-render.js` and
  `actions-operations.js`, core `menus-and-routing.js` and
  `spotlight-runtime.js`: file opening and both Open with menus. The current
  `files.default_apps` choice is read by Spotlight while ordinary file opening
  also contains hardcoded routing. Add Bookshelf consistently to every affected
  entry point; a saved choice must work on double-click as well as in Spotlight.
- `ui/js/desktop/apps/viewer.js` and
  `internal/server/desktop_viewer_handlers.go`: existing PDF.js and rooted
  document delivery. Reuse the packaged PDF engine without replacing the general
  Viewer or assuming its current UI is a complete ebook reader.
- `internal/server/desktop_storage*.go`, `internal/tools/cloud_storage.go` and
  `documentation/desktop-cloud-storage.md`: typed WebDAV/Koofr locations,
  authorization, streaming, revocation and provider errors.
- `internal/desktop/AGENTS.md`, `internal/networkshares/AGENTS.md`,
  `internal/server/AGENTS.md`, `ui/AGENTS.md` and
  `ui/js/desktop/apps/AGENTS.md`: canonical cross-component constraints.
- `internal/webassets/AGENTS.md`, `assets/web-assets.json`,
  `documentation/dependencies.md` and UI bundle/vendor scripts: locally packaged,
  version-bound reader code, workers, fonts, textures and licenses. No CDN.
- `ui/desktop_local_wikipedia_registration_test.go`,
  `ui/desktop_local_wikipedia_browser_test.go` and
  `ui/desktop_log_viewer_test.go`: existing registration/lifecycle test patterns.
  Theme contrast/consistency tests remain applicable to the new app.

Proposed new implementation ownership: `internal/bookshelf/` owns catalog,
scans, read preparation/cache and reader-state validation; server Bookshelf
handlers own HTTP authorization and source capability injection. A narrow
read-only source interface needs list, stat and open only. Reuse typed Desktop
and CloudStorage calls directly; do not loop back through AuraGo HTTP routes.
Keep SMB transport reusable under an integration package, not inside UI handlers.
Frontend modules under `ui/js/desktop/apps/bookshelf/` own shelf, reader and
window state, loaded by a small entry module; CSS and vendor assets load lazily.

The `/api/desktop/bookshelf/` API groups capabilities/sources, library queries,
metadata/collections, scan jobs, content preparation, covers/content, reading
state and annotations. Mutations use existing same-origin/CSRF rules and bounded
requests. Content reads use opaque book/rendition IDs and source authorization;
catalog mutations use version preconditions. Metadata overrides never invoke a
source write method. Return structured errors and job progress rather than raw
provider/process messages.

Add a canonical `internal/bookshelf/AGENTS.md` when implementation establishes
the domain, and route cross-component Bookshelf edits to it from the root
contract table. Register Open with Bookshelf for supported formats and preserve
the existing PDF default unless the user selects Bookshelf. Add the app's icon
to both theme manifests and its lazy locale section to all sixteen languages.

## Planning verification and remaining decisions

Source contracts, current cloud-storage documentation, existing PDF.js use and
upstream reader/converter capabilities were inspected for this plan. GitNexus's
planning impact query for the existing `loadPdf` reported no resolved callers
and UNKNOWN risk, with an index 85 commits behind HEAD; that is not evidence of
zero implementation impact. Refresh and rerun impact analysis before changing
runtime symbols. No runtime change is made by this document.

Remaining gates: validate reader isolation and format fidelity; confirm the
exact remote SMB adapter and optional conversion
runtime. Browser-offline reading, external metadata lookup, full-text library
indexing, OCR, TTS, AI features, DRM processing and source-file management are
outside the first implementation. None is necessary for the requested core.
