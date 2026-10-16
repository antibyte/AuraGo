# Bookshelf reader vendor

Upstream: https://github.com/johnfactotum/foliate-js

Pinned revision: `78914aef4466eb960965702401634c2cb348e9b1` (MIT).
The selected upstream sources are retained in `source/`. The vendored ZIP reader
is `@zip.js/zip.js` 2.8.22 (BSD-3-Clause); fflate is 0.8.2 (MIT).
The bundle includes the repository-pinned DOMPurify 3.4.17 (Apache-2.0 or MPL-2.0).
See the adjacent license files. The existing PDF.js 6.4.299 is loaded separately.

Build with `node scripts/build-bookshelf-reader.mjs`; verify with `--check`.
`manifest.json` records the upstream revision and every selected source/output
SHA-256. Run `npm ci` before rebuilding. No CDN is used at runtime.

AuraGo adaptation:

- `reader-engine.js` uses bounded archive loading and DOMPurify; scripts, forms,
  embedded documents and external resource loads are removed. SVG covers are
  never accepted as active markup by the desktop.
- A minimal trusted HTTP relay transfers a document-bound MessagePort to an
  intrinsically opaque `data:` engine frame. Its section `blob:` frames share
  that opaque origin. They cannot access the authenticated Desktop. The relay
  contains no book markup and accepts a port only from its direct parent.
- The reader route has a dedicated CSP permitting only the exact host and
  bundled scripts; network requests, forms, objects and base URLs are denied.
- `source/mobi.js` carries narrow bounds for PDB records, encrypted content,
  PalmDOC/HUFF expansion, dictionary cycles/depth, EXTH records, and aggregate
  MOBI6 text. Keep these changes when updating upstream. Font decompression
  uses a bounded stream in the AuraGo adapter.
- `source/paginator.js` guards late resize/font callbacks after its section
  document has been disposed, including rapid chapter/book changes.
- `source/comic-book.js` matches image extensions without case sensitivity and
  preserves the original archive paths when loading pages.
- Upstream's experimental PDF import is replaced at bundle time; AuraGo's
  existing PDF.js supplies text selection, search and passive canvas rendering.

Upgrade gate: `TestDesktopBookshelfReaderBrowser`, core hostile-archive tests,
bundle drift verification, format corpus, and the shelf/browser flow tests.
