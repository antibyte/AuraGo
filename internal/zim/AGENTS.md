# ZIM reader

## Purpose

Read-only, CGO-free reader for openZIM archives (Kiwix Wikipedia editions) used by Local Wikipedia.

## Ownership

`internal/zim` owns ZIM parsing, cluster decompression, the decompressed-cluster cache and title listings.
`internal/zim/zimtest` owns the synthetic test writer and the on-demand real fixture.
Callers (`internal/zim/xapian`, `internal/localwiki`) own archive lifetime (refcounting, `Close`) and never parse ZIM structures themselves.

## Local Contracts

- The exported API and error variables are the slice-1 contract in `docs/superpowers/plans/2026-10-09-local-wikipedia-overview.md`; changes need every dependent slice plan updated.
- All file access uses `ReadAt` with `int64` offsets; no global lock; `GOOS=linux GOARCH=arm` must build.
- Limits: decompressed cluster ≤ 32 MiB (`ErrUnsupported`), zstd window ≤ 32 MiB (the cluster limit; libzim writes 8 MiB windows) through pooled synchronous decoders, xz dictionary capped at the cluster limit, directory entry ≤ 64 KiB, MIME list ≤ 64 KiB, metadata value ≤ 1 MiB (`ErrUnsupported`), redirect chain ≤ 8 hops (`ErrRedirectLoop`), cluster cache 64 MiB by default (a cluster above the whole budget is served uncached). Validate sizes before allocating.
- Error classes (test with `errors.Is`): bad magic or too small → `ErrNotZIM`; major ≠ 5/6, discontinued zlib/bzip2 clusters, xz filter chains and limit violations → `ErrUnsupported`; every other structural problem, including a short read or unexpected EOF → `ErrCorrupt`. An underlying I/O error from `ReadAt` is wrapped with `%w` and is not `ErrCorrupt`. Missing entry or index, and the zero `Entry` (one no `Archive` returned) passed to `Open` or `Resolve` → `ErrNotFound`; `Open` on a redirect → `ErrIsRedirect`; any read after `Close` → `ErrClosed`.
- `Close` is idempotent and final: reads through the archive and through already returned sections (file sections and sections over cached clusters) fail with `ErrClosed`, the cluster cache is emptied, and a cluster that finishes loading after `Close` is returned to its caller but not cached.
- Blobs in uncompressed clusters are sections directly on the file (the Xapian slice relies on this for `X/fulltext/xapian` and `X/title/xapian`); they fail after `Close`.
- Compressed clusters are stream-decoded once (singleflight per cluster) into the LRU. xz support is the single-block, single-filter LZMA2 stream libzim wrote; the stream and block headers are CRC-checked, reserved flags are `ErrCorrupt`, filter chains are `ErrUnsupported`, and the declared dictionary is capped at the cluster limit.
- Header: the main page index must be inside the entry table or `0xFFFFFFFF`; the checksum position must match the file size and the path and cluster pointer lists must fit inside the file when the archive opens (the v0 title list when it is used).
- Title listing: `X/listing/titleOrdered/v1` is used when it sits in an uncompressed cluster and holds at most `EntryCount()` positions. A compressed or oversized v1 listing is ignored in favour of the header's v0 list (restricted to the content namespace block); a damaged v1 without a v0 list is `ErrCorrupt`; an archive with no usable listing opens with `ArticleCount() == 0`. Every position read through `ArticleAt` or `TitlePrefix` must point into the content namespace, otherwise `ErrCorrupt`; positions of a v1 listing must also be content or redirect entries (the v0 block may hold deprecated entries in old archives). `TitlePrefix` is byte-wise and case-sensitive.
- `Entry` values carry private cluster/blob numbers and are only valid for the `Archive` that returned them.
- libzim and Xapian are GPL: read them for format knowledge only, never copy code.
- `zimtest` must not import `internal/zim`; production code must not import `zimtest`. Never commit zim-testing-suite files; the real fixture is downloaded on demand (pinned commit and SHA-256) only with `AURAGO_ZIM_REAL_FIXTURE=1`.

## Work Guidance

- Add a mutated-archive test in `invalid_test.go` and a fuzz seed for every new parser guard.
- Fuzz runs write crashers to `internal/zim/testdata/fuzz/<Target>/`; fix the parser and commit the crasher file as a regression seed with the fix.

## Verification

- `go test ./internal/zim/...`
- `GOOS=linux GOARCH=arm go build ./internal/zim/...` and `GOOS=linux GOARCH=arm go vet ./internal/zim/...`
- `go test ./internal/zim/ -run '^$' -fuzz '^FuzzOpenArchive$' -fuzztime 20s -fuzzminimizetime 2s` (and the other `Fuzz*` targets: `FuzzParseHeader`, `FuzzParseDirent`, `FuzzDecodeCluster`)
- `AURAGO_ZIM_REAL_FIXTURE=1 go test ./internal/zim/ -run TestRealFixture -count=1`
- `-race` needs cgo: run `go test -race -count=1 ./internal/zim/... ./internal/fileutil` on aurago-test when the local box has no C compiler.

## Child DOX Index

None.
