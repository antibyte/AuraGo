# Xapian index reader

## Purpose

Read-only, CGO-free reader for the Xapian 1.4 glass indexes that libzim embeds in ZIM files (`X/fulltext/xapian`, `X/title/xapian`), plus libzim-compatible query analysis, BM25 search and title suggestions for Local Wikipedia.

## Ownership

`internal/zim/xapian` owns the glass B-tree format, posting/document-length/value decoding, query normalisation and tokenisation, Snowball stemmer selection, BM25 ranking and suggestion scoring.
`scripts/localwiki/fixtures/` owns the generated fixtures (`internal/zim/testdata/fixture_*.zim`) and the golden JSON in `testdata/`.
Callers (`internal/localwiki`) choose the analyzer language, drop unknown query words (`ExistingTerms`), resolve hit paths through `internal/zim` and degrade to title search when `Open` fails or `FulltextSupported()` is false.

## Local Contracts

- The exported API is the slice-2 contract in `docs/superpowers/plans/2026-10-09-local-wikipedia-overview.md` (local, git-ignored) plus `Normalize`, `ExistingTerms`, `Analyzer.Stem/Language`, `Database.LastDocID/TotalLength/Metadata`, `ErrCorrupt`, `ErrDocNotFound`.
- The byte format and libzim's analysis rules are documented in `FORMAT.md` (this directory); keep it in step with the code.
- Supported: single-file glass, format 2016-03-14, 2–64 KiB blocks. Error classes (test with `errors.Is`): anything this reader does not implement (other backend, other format version, block size outside 2–64 KiB or not a power of two, mixed block sizes, blob under 2 KiB) → `ErrUnsupportedFormat`; structural damage (version fields out of range, block or tag overruns, level mismatch, B-tree keys that are not strictly increasing, bad deflate, non-increasing docids) → `ErrCorrupt`; an unknown docid → `ErrDocNotFound`. An I/O error from the `io.ReaderAt` is wrapped with `%w` and is neither class. Callers treat every `Open` error as "full-text unavailable". Never panic on index data.
- Memory is bounded independently of the index: a tag is at most 16 MiB (before and after inflating), the block cache holds 4 MiB of block data per `Database` (512 blocks at 8 KiB, 64 at 64 KiB, at least 16), a tree has at most 10 levels, `Search` ranks at most `MaxSearchWindow` (10,000) documents, `Suggest` streams candidates in docid order through a top-k of at most `min(limit, MaxSearchWindow)` collapsed hits (never a list of all matching titles) and opens at most 100 expansion lists per partial word; posting lists and value readers read their current chunk in place (`tagView`), so besides the cursor path each keeps only that leaf block alive. Per-candidate work (title re-tokenisation for phrase checks) allocates transient garbage only. Validate sizes before allocating; range-check `uint64` values before narrowing to `int` (32-bit builds).
- Tree walks require strictly increasing `(key, component)`; do not weaken that check, it is what keeps a damaged branch block from making a walk revisit items. Seeks cannot be checked that way (crafted separators can steer them left while every walk stays ordered), so every seek inside a walk must be checked for progress: `jumpTo` must land on the last chunk starting at or before the target, never before the current chunk, which keeps one `skipTo` to one seek plus one chunk.
- Ranking must reproduce libzim 9.8 (Xapian 1.4.23): `Search` = default `BM25Weight`, ties by docid; `Suggest` = `BM25Weight(0.001,0,1,1,0.5)`, sort by score then title, collapse on value slot 1. Change ranking only together with regenerated goldens.
- Normalisation is ICU "Lower; NFD; [:M:] remove; NFC": all marks (Mn, Mc, Me) are removed, Greek final sigma applies.
- Stemmers come from `github.com/blevesearch/snowballstem` and only for languages whose parity with Xapian is proven by golden tests (da de en es fr it nl no/nb/nn pt sv). Other Xapian-stemmed languages report `FulltextSupported() == false`. Japanese and Chinese keep full-text search (Xapian CJK n-grams, `TestCJKFulltextDecision`); `cjkFulltext` in `analyzer.go` is the switch.
- A `Database` is safe for concurrent use (`TestConcurrentReaders`).
- `Search` and `Suggest` check `ctx` on entry and every `ctxCheckEvery` loop iterations (posting-list steps, not matches); new loops over postings or candidates use `poller`.
- Xapian and libzim are GPL: read them for format knowledge only, never copy code.
- Golden data and fixtures are generated, never hand-edited: `bash scripts/localwiki/fixtures/generate.sh` (Docker; pinned python-libzim 3.13.1 and Debian `python3-xapian`).

## Work Guidance

- Every new parser guard gets a unit test with a synthetic block or version block and a fuzz seed.
- Fuzz runs write crashers to `testdata/fuzz/<Target>/`; fix the parser and commit the crasher file as a regression seed with the fix.
- When libzim or Xapian versions change, regenerate fixtures and goldens first; a replica/libzim mismatch in `make_goldens.py` must be understood before goldens are committed.

## Verification

- `go test ./internal/zim/xapian/ -count=1`
- Ranking speed: `go test ./internal/zim/xapian/ -run '^$' -bench BenchmarkSearch -benchmem` (`BenchmarkSearchScale` is a synthetic 2.9M-document index); scoring streams the document length list in docid order, never `DocLength` per hit.
- `AURAGO_ZIM_REAL_FIXTURE=1 go test ./internal/zim/xapian/ -run TestRealFixtureIndexes -count=1 -v`
- `go test ./internal/zim/xapian/ -run '^$' -fuzz '^FuzzDatabase$' -fuzztime 60s` (and `FuzzUnpack`, `FuzzParseVersion`)
- `GOOS=linux GOARCH=arm go vet ./internal/zim/xapian/`
- `-race` needs cgo: run `go test -race -count=1 ./internal/zim/...` on aurago-test when the local box has no C compiler.

## Child DOX Index

None.
