# Bookshelf

Bookshelf is a Virtual Desktop library and ebook reader, enabled by default.
Open **Bookshelf** from the app launcher; you can disable it in the Virtual
Desktop settings. An explicitly saved disabled setting stays disabled.
The reading salon combines a warm paper reading area with a dark wooden shelf.
Standard and Fruity retain their own Desktop window controls.

## Add books

- **Local folders** means folders on the AuraGo server within its authorized
  Desktop workspace. Select a folder and explicitly start a scan.
- **Upload** copies selected files from your browser device into Bookshelf's
  managed imports. Your original remains untouched.
- **WebDAV / Koofr** reuse the configured integration and its File Manager
  visibility grant. Select the connection, Koofr mount where applicable, and
  folder. A provider outage does not remove existing library history.
- **SMB** connects directly to an explicit server and share with named-user
  credentials. Passwords remain in the encrypted Vault. SMB1 and guest fallback
  are unsupported; message signing is required. No LAN discovery runs.

Collections, titles, authors, series, cover overrides and reading status belong
to the Bookshelf catalog. Removing a book from the catalog or disconnecting a
source never renames, moves, rewrites or deletes the original file. Other apps
keep their existing default file associations, including the normal PDF viewer.
Use **Open with Bookshelf** to select Bookshelf explicitly.

The shelf displays up to 36 books per page. Search and sorting cover the whole
catalog, with cover, spine and compact list views for different collection sizes.

## Read

EPUB, DRM-free MOBI/KF8, FB2, FB2.ZIP, CBZ, TXT and standalone HTML use the local
book renderer. PDF uses the existing local PDF.js runtime. EPUB supports both
flowing text and fixed layouts; available controls depend on the layout.
Text books offer fonts, size, line spacing, width and light/sepia/dark surfaces.
PDFs and comics offer page fit, zoom, page navigation and page thumbnails.
Text/HTML encoding can be corrected in the reader settings.

Search requires actual text: image-only comics and scanned PDFs have no OCR.
Bookmarks store a semantic position; text highlights and notes use the selected
text range or PDF page coordinates. Positions include a rendition revision.
If two windows save competing positions, Bookshelf asks which position to keep.
A changed source file does not silently reuse an old text locator.

Book scripts, external images/styles, embedded frames and forms are stripped.
Reading a standalone HTML document does not authorize its parent folder.
External links require a deliberate action. Book content and library metadata
are not sent to an LLM or a metadata provider.

## Optional conversion

CBR, RTF, DOCX, ODT, LIT, PDB and LRF may be converted to a managed EPUB copy.
Conversion is explicit; it never replaces the original and layout may change.
Ordinary EPUB/PDF reading does not require Calibre.

On a Linux host, install Calibre's `ebook-convert`, `bubblewrap` (`bwrap`) and
`prlimit` through your normal system administration process. Set
`virtual_desktop.bookshelf_converter_path` to `/usr/bin/ebook-convert` or
`/opt/calibre/ebook-convert`. Empty means disabled. Bookshelf does not install
packages, download a converter or start conversion while displaying status.

Each conversion uses a private staging directory, read-only input, a separate
network namespace, a cleared environment, resource limits and a three-minute
deadline. Only one conversion runs at a time. Namespace restrictions imposed by
the Linux host/container may still prevent a configured runtime from starting;
the job fails without an unconfined fallback. On Windows, macOS or a restricted
container, import a file converted elsewhere instead.

DRM removal is unsupported. ACSM licenses are not books. Unsupported Kindle/KFX
and DjVu files are not presented as readable merely because their extension is
recognized. Malformed, encrypted or over-budget files produce an explicit error.

## Storage and permissions

Bookshelf stores its SQLite catalog, covers, managed imports and derived
renditions below `data/bookshelf/`. Back up the catalog and managed content;
disposable read cache can be recreated. Database upgrades create a backup before
migration. Books are limited to 256 MiB and the normal read cache to 1 GiB.
Stricter existing source/desktop limits still apply.

**Keep on this AuraGo server** means an explicitly retained server copy. It does
not mean offline browser storage: the browser still needs AuraGo. Private book
responses use `Cache-Control: no-store`; the shared service worker must not cache
them. Removing a grant blocks cached access too.

Content access is checked again for every request. Closing a reader releases its
cache lease immediately. Unused reader URLs expire after five minutes; after a
successful content request they expire after 24 idle hours as a fallback for a
lost browser session. Active transfers finish without an expiry interruption,
and expired tickets release their cache leases automatically.
An unchanged scan preserves reading state even when a source provides no file
version. For these sources, preparing the book checks its bytes for changes.

Bookshelf requires Desktop administrator access. Source read-only grants permit
reading and separate library edits when Desktop permits them. Global Desktop
read-only prevents library, annotation and progress changes while retaining
authorized reading and private temporary preparation. Disable Bookshelf to
cancel its jobs and revoke its content access.

## Maintenance

The design and acceptance contract is [bookshelf-plan.md](bookshelf-plan.md).
The domain contract is `internal/bookshelf/AGENTS.md`. Reader provenance, local
patches and licenses are documented in `ui/js/vendor/foliate/README.md`.

Run the focused Bookshelf, SMB-client and converter Go tests, then the Desktop
Bookshelf browser suites with `AURAGO_RUN_BROWSER_SMOKE=1`. Rebuild and verify:

```sh
node scripts/build-bookshelf-reader.mjs
node scripts/build-bookshelf-reader.mjs --check
node scripts/test-bookshelf-reader-security.mjs
node scripts/test-bookshelf-progress.mjs
node scripts/test-bookshelf-routing.mjs
node scripts/build-ui-bundles.js
node scripts/build-ui-bundles.js --check
```

Bundle regeneration and local tests do not deploy or restart AuraGo. NAS/provider
interoperability and Linux converter execution need validation in the target
environment; the local fixtures do not establish those results.
