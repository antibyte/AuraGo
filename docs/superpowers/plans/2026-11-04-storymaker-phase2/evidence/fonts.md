Storymaker phase 2 glyph and fallback-font spike: all four questions measured. I could not save `report.md`, because the tool refused to write a report file, so this message is the only copy of the findings. Everything below was run in Chrome 156.0.8078.80, except the (a) and (b) numbers, which come from Node cmap scripts. No repo file was touched and no worktree was created.

Everything is under `C:/Users/Andi/AppData/Local/Temp/claude/babd9307b9bef681/f2c97a7f-addd-4aa7-bd49-85d6c9b3f2a1/scratchpad/p2/spikes/fonts/`.

**Repeat it**
- (a) and (b): `node cmap.mjs C:/Users/Andi/Documents/repo/AuraGo out`, then `node glyphs.mjs` and `node greek.mjs`.
- (c): build the Go module in `gospike/`, then `sh run-all.sh`.
  - The Go module is built with `GOFLAGS=-mod=mod GOPROXY=off` and `-o D:/tmp/go-build/fontspike/fontspike.exe`.
  - The pages are in `web/`. The sandboxed iframe is served with the exact `StorymakerPlayerCSP` string.
  - Results are in `out/q1` to `out/q4` (PNGs and JSON) and `out/m` (memory runs).

**(a) Coverage**
- **Geist:** all five weights have the same 728 codepoints. Latin is 190/191, Latin Ext-A/B 136/336, Cyrillic 134/256, and Greek only 6/144 (Λ Ω λ μ π ω). Geist does not cover Greek.
- **Noto faces:**

| Face | Codepoints | Coverage |
|---|---|---|
| CJK | 44,810 | Han 20,976/20,992, kana 189/192, Hangul 11,172/11,172, Greek 49/144 (no tonos letters) |
| Devanagari | 551 | 128/128 |
| Arabic | 1,560 | 256/256 |

- **Greek choice:** Inter, OFL-1.1 (listed in `THIRD_PARTY_LIBRARIES.md:257`) and already shipped in `ui/fonts`.
  - The files are `UcC73FwrK3iLTeHuS_nVMrMxCp50SjIa1pL7SUc.woff2` (Greek, 19 KB) and `…2ZL7SUc.woff2` (Greek Extended, 11 KB).
  - Together they cover all 68 core Greek letters including tonos, and they are a variable font for weights 300 to 700.
  - `ui/fonts` has no `Inter-LICENSE.txt`, so add the OFL text with the change.
- **Trap:** with the order Geist, then Greek, Chrome draws ω as a capital Ω shape and ώ as Ω plus an accent. It decomposes ώ and uses Geist's own omega. The Inter file also contains U+0020, U+0041, U+00A0 and U+00C4, so it must not simply go first.
- **Fix, verified in `out/q4`:** give the Greek FontFace `unicodeRange` U+0370-03FF (and U+1F00-1FFF for the extended file) and list it first. Greek then renders correctly and Latin is unchanged. So `glyphs.json` must store the effective range per face, and Geist's Greek must be excluded.

**(b) Coverage file**
- **Size:** the delta format (`[gap, len, …]` per face) is 11,181 B raw and 3,498 B gzip. The flat start/end format is 30,349 B raw and 7,225 B gzip. Without CJK, flat is 2,591 B raw and 925 B gzip.
- **Format:** `{"schema":"storymaker-glyphs/1","faces":[{"id","family","file","weight","unicode_range","max_bytes"?,"d":[…]}]}`.
  - Faces are in fallback order.
  - The first `d` value is the start of the first range. After that, gap is the start minus the previous range's end, and len is end minus start.
  - A codepoint is covered if it is in the face's cmap and inside its `unicode_range`.
- **Go loader:** mirror `LoadCatalog`. Take an `fs.FS` and a path, use `io.LimitReader` (1 MiB), `json.Unmarshal`, check the schema string and that the ranges are monotonic, decode to sorted `[2]rune` per face, and use binary search.
- **Build:** add `glyphs.json` to the catalog `version` hash and regenerate it in `--check`.

**(c) Browser spike**
- **CSP and fonts:** a blob worker with OffscreenCanvas runs under the player CSP. FontFace from ArrayBuffers works. The fonts are posted in from the parent, because the iframe has `connect-src` none.
- **Fallback and shaping:** per-character fallback works with a family list in `ctx.font`. Devanagari conjuncts and Arabic joining are shaped correctly.
- **Direction:** Arabic-first strings render the same under ltr, rtl and inherit. A Latin-first mixed string differs under rtl, and inherit behaves as ltr. Set `ctx.direction` explicitly per text.
- **Measure vs draw:** ink lies within `actualBoundingBox` by 1 to 2 px on all ten lines.
- **Determinism:** pixels were identical in a second draw, across 4 workers, and across separate browser processes (hash `c4b9be3c`), on this machine only.
- **Silent host fallback:** yes. Thai, Hebrew, Ethiopic, colour emoji and math symbols draw from host fonts with no error, and an unassigned codepoint draws a tofu box. Without the CJK face loaded, Han also falls to the host. The static check is therefore the only guard.
- **Load time:** CJK takes 292 to 330 ms per worker, the other faces 4 to 15 ms each. Total iframe load is 61 to 66 ms without CJK and 415 to 543 ms with it.
- **Memory:** I measured the whole Chrome process tree via Win32_Process, not the renderer alone. These are private MiB at base, loaded, and after 30 frames, one run per cell, perhaps ±20 MiB:

| Config | Base | Loaded | After 30 frames |
|---|---|---|---|
| 1 worker, no CJK | 152 | 174 | 204 |
| 4 workers, no CJK | 158 | 222 | 342 |
| 1 worker, CJK | 157 | 296 | 287 |
| 4 workers, CJK | 145 | 547 | 507 |

  CJK costs roughly 80 to 120 MiB per worker.

**(d) Recommendations**
1. **CJK cap:** keep the 8 MiB per-font cap and add one named exception for `NotoSansCJKsc.otf` (17 MiB, `max_bytes` in `glyphs.json`). Do not subset in phase 2, since a subset breaks "every Han char is validated". The five faces total about 18 MB, under the 32 MiB total cap.
2. **Load on need:** yes. Compute the needed faces from `glyphs.json`, taking the first face in order that covers each codepoint.
3. **Family order:** `"SMGreek"` (with `unicodeRange`), `"Geist"`, `"SMDeva"`, `"SMArab"`, `"SMCJK"`, then `sans-serif`. CJK is last because it also covers Latin.
4. **text.js wrap:**
   - `fontString` must accept a family list.
   - Han and kana arrive as one "word", and `breakWord` cuts per UTF-16 unit. That splits surrogate pairs and Devanagari clusters, so it needs grapheme-aware cutting.
   - Add a break opportunity between CJK characters with kinsoku rules: nothing like 。、，！？）」 at line start, and no 「（ at line end.
   - Extend the `fitEllipsis` trim set with Arabic and CJK punctuation.
   - For RTL, space wrapping works. Set direction per text and decide whether align means physical or start/end.
5. **Validator:** it must use the same effective ranges. Emoji, Thai and Hebrew are `glyph_missing`.

**Not verified**
- Renderer-only memory via CDP.
- Other Chrome versions or machines.
- `Intl.Segmenter` in workers.
- A CJK subset and its size.
- The real `host-render.js` page path (the spike used its own pages).
- Japanese-specific glyph forms (the face is SC) and real bold (CJK bold is synthetic).
- Whether Geist lacking Latin characters leads to NFD fallback.
- The upstream Inter licence text.