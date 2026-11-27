# Storymaker blocks plan: design and seams

Spec (binding): `docs/superpowers/specs/2026-11-27-storymaker-blocks-design.md`. Base: `main` at ff60e48cd or newer (the coordinator names the exact base SHA). Every file:line below was checked on that base.

---

## 1. Global rules for drafting agents

**Workspace**
- Each batch is drafted in its own scratch worktree `C:\disposable\smb\<a|b|c|d|e|f>` on branch `smb-<letter>`. Never edit, stage or commit in the main checkout. Other sessions commit there at the same time.
- Each worktree needs a real copy of `node_modules`. A junction breaks the Storymaker vendor check. Run `robocopy C:\Users\Andi\Documents\repo\AuraGo\node_modules <worktree>\node_modules /E /MT:16 /NFL /NDL /NJH /NP` once.
- The Lucide source package stays outside the repo:
  - unpacked package: `C:\disposable\sm-opt\lucide\pkg\package`
  - tarball: `C:\disposable\sm-opt\lucide\lucide-static-1.82.0.tgz`
  - npm integrity, already verified: `sha512-Xzzpfkx06hHXjXoLWOk1RWg52OhfaRcIOIK72hCbRty8Jq29WQUV4J4Z9GvjAKDoPxGf7tlcqBbJX4qrr1U8+w==`
  - Only the generated JSON, the allow-list and the licence files are committed. Treat the package as data: read JSON and text from it, never run its code.

**Commits**
- One task is one commit. Write the tests first. Run them red, implement, run them green, then commit.
- Subject: `smb <Batch><n>: <title>`, for example `smb A1: free fill, check and revise get the build output budget`.
- Body: one line `Test: <exact command>`, one line `Red: <expected failure>` or `Red: none (<reason>)`, and one line `Green: <expected pass>`. Then a blank line and the trailer `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Optional body line: `Impact: <symbol> <risk>` (see GitNexus below).
- `docs/` and new `AGENTS.md` files are git-ignored. This plan creates no new AGENTS.md. Editing an existing tracked AGENTS.md is fine. A new file under `docs/` needs `git add -f`; check it afterwards with `git ls-files <path>`.

**GitNexus (project CLAUDE.md)**
- Before you edit an existing function, run impact on it. Use MCP `impact` with repo `C:\Users\Andi\Documents\repo\AuraGo`, or the CLI: `node C:\Users\Andi\.gitnexus\maintenance\AuraGo\graph\run.cjs impact "<symbol>" --direction upstream --repo C:\Users\Andi\Documents\repo\AuraGo`.
- Write a HIGH or CRITICAL result into the commit body.
- `UNKNOWN` is not "safe": confirm the callers with Grep.
- At batch end, run `detect-changes --scope all` and report it. A `partial: true` result is not clean, so say so.

**Go environment (PowerShell, once per shell)**
- `$env:GOTMPDIR='C:\tmp\go-build-smb'; New-Item -ItemType Directory -Force C:\tmp\go-build-smb | Out-Null`
- GOTMPDIR must be a short path with backslashes. Forward slashes break 7 path tests.
- If `no such tool "vet"` appears, prefix the Go commands with `$env:GOTOOLCHAIN='go1.27.4';`.
- Group `-run` patterns as `(?i)(a|b)` and count the PASS lines as evidence.
- If another session's half-written untracked Go file breaks the build, use `go test -overlay` with an empty replacement for that file. Never touch the file itself.

**Generated files (regenerate only with these commands)**
- Player and host bundles: `npm run build:ui`, then `npm run check:ui`. Never edit `*.bundle.js` by hand. If `ui/js/vendor/storymaker/LICENSES.txt` changes after `build:ui`, run `git checkout -- ui/js/vendor/storymaker/LICENSES.txt` and run `check:ui` again.
- Catalog, i.e. `ui/img/storymaker/**` and `ui/lang/storymaker_catalog/*.json`: `node scripts/build-storymaker-catalog.mjs`, checked with `node scripts/build-storymaker-catalog.mjs --check`. It can leave LF-only phantom diffs on unchanged `ui/img/storymaker/thumbs/*.png`. `git checkout --` those, then run `--check` again.
- Icon data (`assets/storymaker/icons/*`): `node scripts/build-storymaker-icons.mjs --package C:\disposable\sm-opt\lucide\pkg\package` (new in B2).
- Ranges: no change in this plan. Run `node scripts/build-storymaker-ranges.mjs --check` once per batch.

**Line endings**
- Never convert an `eol=lf` file to CRLF. `.gitattributes` makes `*.go`, `*.md`, `*.json` and `ui/js/storymaker/*.bundle.js` LF.
- B2 adds LF rules for the new `.txt` licence files.
- Other `ui/js/storymaker/*.js` files are `text=auto`: keep whatever the file has.
- Go source: write a backslash as `\x5c`. The Write tool decodes `\u` escapes.

**Player source guards** (`scripts/storymaker-tests/10-static-guards.mjs:18-31`)
- Every player part must stay under 600 lines.
- No template strings with `${`.
- No `Math.random`, `Date.now`, `performance.now` or timers.
- Each file starts with a `// ` comment.
- This is why the blocks module is two files and why D puts its beat logic in a new file.

**Test style**
- Lean tests: the spec's tests plus real risks. No mutation, property or stress suites.
- `npm run test:storymaker` runs all `scripts/storymaker-tests/*.mjs` (about 8 s). It has no filter, so it is the JS test command of every JS task.
- Asset node tests: `node --test assets/storymaker/test/<file>.test.mjs`.

---

## 2. Binding seams

### 2.1 Stage 1 (batch A)

**Output budget** (`internal/server/storymaker_agent.go`, used at :410)
```go
// storymakerAgentOutputTokens is max_tokens of one loop. The free mode's fill, check and revise phases write code and get
// Game Maker's build reserve; every other phase keeps the general reserve.
func storymakerAgentOutputTokens(limits llm.ModelLimits, mode, phase string) int
```
- free + fill/check/revise: `gameMakerOutputTokens(limits, gameMakerBuildAnswerTokens)`
- every other case: `gameMakerOutputTokens(limits, llm.ConservativeOutputTokens)`

Values (MaxOutputTokens 131072):

| Context | free fill/check/revise | catalog, or free outline |
| --- | --- | --- |
| 204800 | 32768 | 20480 |
| 262144 | 65536 | 20480 |
| 32768 | 0 | 0 |

**Two-pass fill**
- New const `storymakerFillPasses = 2` in `storymaker_agent.go`.
- New field `pass int` on `storymakerAgentLoop` (:136-144). 0 means "not a fill pass".
- In `runPhase` (:758-781), when `loop.pass > 0`:
  - run exactly one attempt;
  - the event field `"attempt"` is `loop.pass`;
  - `noteModelError` is called only when `loop.pass == storymakerFillPasses`.
- New method `func (j *storymakerAgentJob) runFillPasses(ctx context.Context, first []string)`:
  - pass 1 runs every id of `first` once, in order;
  - then it re-reads `j.studio.Progress`;
  - pass 2 runs the ids of `first` that are still in `Unfilled`, in the order of `first`;
  - each loop is followed by `j.refreshProgress(ctx)`;
  - it stops when `ctx.Err() != nil`.
- `runCreate` (:912-918) calls `j.runFillPasses(ctx, progress.Unfilled)`.
- The job timeout does not change.

**Closing markers**
- `{/n}` (n = digits) is removed from any text the Studio saves.
- The parsers in Go and JS treat `{/n}` like a marker for whitespace, but record no cue.
- JS regex in `narration.js` parse: `/\{(\/?)(\d+)\}/g`. Record a cue only when `m[1] === ''`.
- Go: new `var narrationAnyMarker = regexp.MustCompile(\`\{(/?)(\d+)\}\`)`, used by `parseMarkers` (`narration.go:113-142`; skip the cue when group 1 is not empty) and by `cueTokens` (`cuephrase.go:25-28`).
- `narrationMarker` (`narration.go:38`) stays unchanged for every other caller.
- Go: new `var closingMarker = regexp.MustCompile(\`\{/\d+\}\`)`. `repairText` (`studio_umlauts.go:108`) applies `closingMarker.ReplaceAllString(text, "")` first, for every language, then `RepairUmlauts`.

**Safe box**
- Go, new file `internal/storymaker/safebox.go`:
```go
// SafeBox is the drawing area of a code scene in 1920x1080 design pixels for an aspect.
type SafeBox struct {
    X int `json:"x"`
    Y int `json:"y"`
    W int `json:"w"`
    H int `json:"h"`
}
const (
    captionBandTop         = 0.794 // top of the two-line caption box, 16:9 and 1:1 (layout.go comment, measured 2026-11-09)
    captionBandTopPortrait = 0.654 // 0.78 box bottom - 0.126 two-line box height; caption sizes are fractions of the height
)
func SafeBoxFor(aspect string) SafeBox // an unknown aspect is 16:9
```
- Computation (same as `stage.js` cover fit):
  - `scale = max(W/1920, H/1080)` with output sizes 16:9 1920x1080, 9:16 1080x1920, 1:1 1080x1080;
  - `visW = min(1920, W/scale)`, `visH = min(1080, H/scale)`, `visX = 960 - visW/2`, `visY = (1080 - visH)/2`;
  - zone `z = SafeZone(aspect)` (`layout.go:82-87`);
  - `left = visX + z.Left*visW`, `right = visX + (1-z.Right)*visW`, `top = visY + z.Top*visH`;
  - `bottom = min(visY + (1-z.Bottom)*visH, visY + captionTop*visH)`, where captionTop is the portrait value for 9:16 and `captionBandTop` otherwise;
  - `X = ceil(left-1e-6)`, `Y = ceil(top-1e-6)`, `W = floor(right+1e-6) - X`, `H = floor(bottom+1e-6) - Y`.
- Pinned values:

| Aspect | Safe box |
| --- | --- |
| 16:9 | `{96,54,1728,803}` |
| 9:16 | `{691,125,493,581}` |
| 1:1 | `{474,54,972,803}` |

- JS: `SM.stage.safeBox(aspect) -> {x, y, w, h}` in `stage.js`, the same numbers and the same rounding. Constants `SAFE_ZONES` and `CAPTION_TOP` carry the comment "repeat internal/storymaker/safebox.go; change both together".
- `api.safe()` (codescene.js) returns `SM.stage.safeBox(opts.aspect)`. It returns a fresh object on every call.

**Free fill packet** (`packetFreeFill`, `studio_packet.go:224-270`) adds three fields:
- `"code_rev"` (string): `studioRev(etag)` of `Projects.LoadCodeSource(ctx, projectID, sceneID)`. When the source does not exist (`fs.ErrNotExist`) it is `MustNotExist` (`"-"`).
- `"safe_box"`: `SafeBoxFor(story.Format.Aspect)` (the SafeBox value; it marshals as `{"x","y","w","h"}`).
- `"api_card"` (string): `FreeAPICard(tokens, box)`.
- `"next"` becomes exactly: `put_code with expected_rev = code_rev and data.source (get_code only to read a saved source or after rev_mismatch), then put_scene with data {"kind":"code"}`

**API card**
- New file `internal/storymaker/studio_apicard.go`:
```go
const freeAPICardText = `...`  // text below; E replaces it (section 5, E6)
// FreeAPICard is the card of a free fill packet: freeAPICardText, a palette line and a safe_box line, joined by "\n".
func FreeAPICard(tokens map[string]string, box SafeBox) string
```
- Palette line: `Palette of this film: ` + for each name in `paletteTokenNames` order (`catalog.go:214`) that has a value: `$<name> <hex>`, joined by `, `. When there are no tokens, the line is left out.
- Tokens come from the `Catalog.Palettes` entry whose `Name == story.Style.Palette`.
- Safe box line: `fmt.Sprintf("safe_box: x %d, y %d, w %d, h %d", ...)`.
- No trailing newline.
- Text after A (exact; lines joined by `\n`):
```
API card for this code scene. Read storymaker_catalog reference topic code only when this card is not enough.
export default function scene(api) { ... } runs once and builds the scene; each frame is a pure function of t.
timing = {start, dur, ease}; ease is linear, ease-in, ease-out, ease-in-out, back-out or bounce-out. api.cue(n) is the second of narration marker {n}.
api.duration(seconds)
api.safe() returns {x, y, w, h}: the drawing area of this format in 1920x1080 design pixels, above the captions.
api.text(text, 'title' or 'label', {at}) returns a handle.
api.shape('rect', 'circle' or 'line', {at, w, h, fill, stroke}) returns a handle; w and h are design pixels.
handle.moveTo(at, timing), fadeIn(timing), fadeOut(timing), scaleTo(scale, timing), rotateTo(degrees, timing)
api.tween(handle, {opacity, x, y, scale, rotation, width, height}, timing)
api.camera.zoom(factor, timing), api.camera.pan(at, timing), api.camera.shake(amount, timing)
api.random('seed') returns a seeded random function: call it while building, never in onFrame.
api.onFrame(function (t, g) { ... }): g.save, restore, translate, rotate, scale, alpha, fill, stroke, rect, strokeRect, circle, line, path(svgPath, 'fill', 'stroke' or 'both'), text(text, x, y, {size, weight, align, color}). g draws in design pixels.
at is {x, y} in percent of the safe area, or a spot: left, center, right.
Colours: the palette tokens $bg, $bg2, $fg, $accent, $accent2, $text, $outline, or #rrggbb.
Never: api.add, Math.random, Date, timers, imports, the DOM, or state that changes from frame to frame.
```
- Size bound: the whole card is at most 1600 bytes after A and at most 2400 bytes after E.

**Code scenes in the layout run**
- Go `LayoutScene` (`layout.go:38-48`) gains `Code bool \`json:"code"\`` after `Static`.
- JS `layout.js` emits one block per code scene: `{id, index, hold_s, code: true, actors: [], props: [], texts: [], caption, missing: 0, static: false}` (D adds `blocks`).
- Storyboard blocks carry `code: false`.
- Code hold frame, with `D = (end_frame - start_frame) / fps` and `lastFrame = (min(end_frame, next.start_frame) - 1 - start_frame) / fps`:
  - `hold_s = min(max(CODE_HOLD_SHARE * D, lastCue + HOLD_AFTER), lastFrame)`;
  - `CODE_HOLD_SHARE = 0.7` (matches `compileshots.go:11`), `HOLD_AFTER = 0.5` (exists);
  - D adds `lastBlockEnd + HOLD_AFTER` to the max.
- Go `LayoutIssues` (`layout.go:174`) never reports `scene_static` for a `Code` scene.
- `DecideOutcome` stays unchanged. A film with code scenes is still `layout_unverified`, because raw `g` drawing is never measured.

### 2.2 Icons (batch B)

**Committed files**
- `assets/storymaker/icons/allow-list.json`, schema below.
- `assets/storymaker/icons/lucide.json`, generated.
- `assets/storymaker/icons/LICENSE.txt`: verbatim copy of the package `LICENSE` (ISC Lucide plus the MIT Feather section).
- `assets/storymaker/icons/NOTICE.txt`, generated: header plus both licence texts.

**Delivered by the catalog build** (copies)
- `ui/img/storymaker/icons/lucide.json`, `ui/img/storymaker/icons/LICENSE.txt`, `ui/img/storymaker/icons/NOTICE.txt`.
- New manifest key `"icons": {"file": "icons/lucide.json"}`, next to `glyphs`.

**`.gitattributes`** (append):
```
assets/storymaker/icons/*.txt text eol=lf
ui/img/storymaker/icons/*.txt text eol=lf
```

**Allow-list** (`assets/storymaker/icons/allow-list.json`):
```json
{"schema":"storymaker-icon-allow-list/1","source":"lucide-static@1.82.0","categories":{"devices":[...],...}}
```
Category names, in this order: `devices, development, connectivity, security, science, medical, finance, charts, communication, people, nature, transport, weather, files, arrows, home, education, general`.

Every id below was checked with grep against `C:\disposable\sm-opt\lucide\pkg\package\icon-nodes.json`. The list has 516 unique ids. An id may appear in more than one category; the script dedupes it and unions the categories.
- **devices**: cpu, microchip, server, server-cog, server-crash, hard-drive, database, database-zap, database-search, monitor, monitor-smartphone, monitor-play, laptop, smartphone, tablet, tv, keyboard, mouse, printer, router, webcam, camera, headphones, speaker, watch, gamepad-2, joystick, calculator, projector, memory-stick, usb, cable, battery, battery-charging, plug, plug-zap, unplug, power, radio, drone
- **development**: code, code-xml, square-code, terminal, braces, brackets, bug, git-branch, git-merge, git-pull-request, git-graph, binary, file-code, file-braces, variable, webhook, workflow, blocks, box, boxes, package, puzzle, component, layers, layout-dashboard, app-window, settings, wrench, hammer, sliders-horizontal, rocket, bot, brain-circuit, sparkles, wand-sparkles, container, infinity, refresh-cw, gauge, activity, zap, cloud-cog
- **connectivity**: globe, earth, wifi, wifi-off, signal, antenna, radio-tower, rss, cast, share-2, send, network, waypoints, route, satellite, satellite-dish, nfc, cloud, cloud-off, cloud-upload, cloud-download, link, unlink, ethernet-port, bluetooth, globe-lock, server-off, arrow-left-right
- **security**: shield, shield-check, shield-exclamation-point, shield-x, shield-off, shield-user, lock, lock-open, lock-keyhole, key, key-round, fingerprint-pattern, scan-face, eye, eye-off, user-check, user-lock, badge-check, siren, triangle-exclamation-point, circle-exclamation-point, ban, file-lock, folder-lock, vault, id-card, skull, cctv, door-closed, door-open, brick-wall, brick-wall-fire, scan-search
- **science**: atom, flask-conical, flask-round, beaker, test-tube, test-tubes, microscope, dna, telescope, magnet, orbit, radiation, biohazard, pipette, thermometer, sigma, pi, lightbulb, flame, droplet, wind, brain, cone, cylinder, chart-scatter, virus
- **medical**: heart, heart-pulse, heart-handshake, stethoscope, pill, pill-bottle, syringe, bandage, hospital, ambulance, cross, bone, accessibility, hand-heart, baby, ear, apple, salad, dumbbell, bed, clipboard-plus, briefcase-medical, tablets, person-standing
- **finance**: wallet, wallet-cards, credit-card, banknote, coins, piggy-bank, receipt, landmark, dollar-sign, euro, bitcoin, percent, badge-percent, hand-coins, trending-up, trending-down, shopping-cart, shopping-bag, store, tag, tags, gift, handshake, scale, briefcase, building, factory, truck
- **charts**: chart-bar, chart-bar-increasing, chart-column, chart-column-increasing, chart-line, chart-pie, chart-area, chart-spline, chart-network, chart-gantt, presentation, table, table-2, target, goal, kanban, list-checks, list-ordered, list, funnel
- **communication**: mail, mail-open, inbox, message-circle, message-square, messages-square, message-circle-question-mark, phone, phone-call, video, mic, megaphone, bell, bell-ring, at-sign, hash, contact, book-user, languages, speech, quote, thumbs-up, thumbs-down, star, flag, bookmark, newspaper, headset, sticky-note, notebook-pen, captions, audio-waveform
- **people**: user, users, user-plus, user-minus, user-round, users-round, user-cog, user-search, user-star, user-exclamation-point, circle-user, circle-user-round, crown, hand, face-slightly-smiling, face-grinning, face-neutral, face-slightly-frowning, face-angry, shirt, graduation-cap
- **nature**: sun, moon, snowflake, droplets, waves-horizontal, mountain, mountain-snow, tree-pine, tree-deciduous, trees, tree-palm, leaf, sprout, flower, flower-2, clover, bird, fish, squirrel, rabbit, dog, cat, turtle, shell, feather, paw-print, sunrise, sunset, rainbow, tornado, recycle, wheat, carrot, citrus
- **transport**: car, car-front, bus, bike, train-front, tram-front, train-track, plane, plane-takeoff, plane-landing, helicopter, ship, sailboat, anchor, fuel, map, map-pin, map-pinned, navigation, compass, milestone, traffic-cone, forklift, tractor, cable-car, ticket
- **weather**: cloud-rain, cloud-drizzle, cloud-hail, cloud-fog, cloud-moon, cloud-sun, cloud-snow, cloud-lightning, cloudy, thermometer-sun, thermometer-snowflake, umbrella, haze, sun-dim, sun-snow, moon-star
- **files**: file, file-text, file-image, file-video-camera, file-music, file-spreadsheet, file-chart-column, file-check, file-x, file-plus, file-minus, file-search, file-archive, file-stack, files, folder, folder-open, folder-tree, archive, clipboard, clipboard-check, clipboard-list, notebook, book, book-open, library, paperclip, save, download, upload, trash, copy, sheet, image, film, eighth-notes
- **arrows**: arrow-right, arrow-left, arrow-up, arrow-down, arrow-up-right, arrow-down-right, arrow-big-right, arrow-right-left, arrow-right-to-line, arrow-down-to-line, arrow-up-from-line, move, move-right, move-horizontal, chevron-right, chevrons-right, corner-down-right, undo-2, rotate-cw, repeat, shuffle, split, merge, expand, shrink, maximize, circle-arrow-right, square-arrow-right, external-link, iteration-cw
- **home**: house, house-wifi, sofa, armchair, bath, toilet, lamp, lamp-desk, refrigerator, microwave, washing-machine, heater, fan, coffee, utensils, cooking-pot, chef-hat, fence, warehouse, hotel, church, castle, tent, alarm-clock
- **education**: school, backpack, book-open-check, library-big, pencil, pencil-ruler, pen, pen-tool, ruler, eraser, highlighter, spell-check, shapes, palette, trophy, medal, award, list-todo, square-check, circle-check, question-mark, circle-question-mark
- **general**: check, x, plus, minus, circle-x, info, exclamation-mark, search, zoom-in, clock, calendar, calendar-check, timer, hourglass, pin, equal, divide, crosshair, focus, scan, grid-3x3, circle, circle-dot, square, triangle, hexagon, diamond, sparkle, gem, party-popper, flag-triangle-right, signpost, loader, badge

An id the package lacks makes the build script fail and name it. The unique count must stay between 490 and 530.

**Icon data** (`lucide.json`; LF; one icon per line; sorted by id)
```json
{"schema":"storymaker-icons/1","source":"lucide-static@1.82.0","grid":24,"stroke":2,"icons":[
{"id":"icon.cpu","paths":["M12 20v2","M12 2v2", ...],"tags":["processor","cores", ..., "cpu","devices"]},
...
]}
```
- `id` is `icon.` + the Lucide name.
- `paths` (1..40 strings) are the stroked elements.
- Optional `fills` (strings) are the elements that have a `fill` attribute other than `none`. They are drawn filled and stroked. The package has 31 such attributes.
- `tags` is the union, in this order and deduped: Lucide `tags.json[name]` (first 20), the words of the name split at `-`, then the categories. At most 32 tags.
- Path strings use only `[MmLlHhVvCcSsQqTtAaZz0-9 .,-eE]`, at most 2000 characters each.
- The file is at most 256 KB.

**Node to path conversion** (`icon-nodes.json`: name to `[[tag, attrs], ...]`; attribute values are strings)
- `f(n) = String(Math.round(n*1000)/1000)`.
- `path d`: `d` trimmed, unchanged.
- `circle cx cy r`: `M{cx-r} {cy}A{r} {r} 0 1 0 {cx+r} {cy}A{r} {r} 0 1 0 {cx-r} {cy}Z`.
- `ellipse cx cy rx ry`: the same with `rx`, `ry`.
- `rect x y width height rx? ry?`:
  - `ry` defaults to `rx`, `rx` to `ry`, both to 0, clamped to w/2 and h/2;
  - no radius: `M{x} {y}H{x+w}V{y+h}H{x}Z`;
  - else `M{x+rx} {y}H{x+w-rx}A{rx} {ry} 0 0 1 {x+w} {y+ry}V{y+h-ry}A{rx} {ry} 0 0 1 {x+w-rx} {y+h}H{x+rx}A{rx} {ry} 0 0 1 {x} {y+h-ry}V{y+ry}A{rx} {ry} 0 0 1 {x+rx} {y}Z`.
- `line x1 y1 x2 y2`: `M{x1} {y1}L{x2} {y2}`.
- `polyline points`: `M{p0}L{p1} ...`.
- `polygon points`: the same plus `Z`.
- Any other tag, or a `transform` attribute, is an error.
- `key` attributes are ignored.

**NOTICE.txt** (generated):
```
Storymaker icons: <N> icons from Lucide (lucide-static 1.82.0, https://lucide.dev), ISC License.
Some Lucide icons derive from Feather (Cole Bemis), MIT License; both texts follow.
Package integrity: sha512-Xzzpfkx06hHXjXoLWOk1RWg52OhfaRcIOIK72hCbRty8Jq29WQUV4J4Z9GvjAKDoPxGf7tlcqBbJX4qrr1U8+w==
Converted to path data on the 24 grid by scripts/build-storymaker-icons.mjs from icon-nodes.json and tags.json.

<the package LICENSE text, verbatim>
```

**Go** (new file `internal/storymaker/icons.go`)
```go
const IconsSchema = "storymaker-icons/1"
type Icon struct {
    ID    string   `json:"id"`
    Paths []string `json:"paths"`
    Fills []string `json:"fills,omitempty"`
    Tags  []string `json:"tags"`
}
func (c *Catalog) Icon(id string) (Icon, bool)
func (c *Catalog) IconIDs() []string   // sorted
func (c *Catalog) IconsFile() string   // "" without the manifest key
```
- `catalogFile` (`catalog.go:202-212`) gains `Icons *iconsRef \`json:"icons"\``.
- `LoadCatalog` loads the file the way it loads glyphs (call next to `catalog.go:333-335`), with `maxIconsBytes = 1 << 20`.
- Invalid data is a catalog problem: wrong schema, an id not matching `^icon\.[a-z0-9-]{1,60}$`, a duplicate or unsorted id, 0 or more than 40 paths, a bad path string, or more than 32 tags.

**Kind `icon`**
- `NormalizeKind("icon")` and `NormalizeKind("icons")` return `"icon"`, through `kindAliases`. `kindPrefixes` stays unchanged.
- `Catalog.Search(query, kind, limit)` scores icons when kind is `""` or `"icon"`, after all entries in order. An icon becomes `Entry{ID, Kind:"icon", Name: ID, Tags}`, so a tie keeps catalog entries first.
- Hit `usage_example`, catalog mode: `{"do":"show","icon":"<id>","style":"card","text":"<Label>","at":"board","entry":"draw"}`, where `<Label>` is the Lucide name with its first letter in upper case and dashes as spaces.
- Free mode: `{"code":"api.icon('<id>', {style: 'card', label: '<Label>', at: 'right'})"}`.
- `describe` with an `icon.*` id answers the icon.
- Free mode: the catalog `search` operation is allowed in fill and revise, and the kind is always forced to `icon`.

### 2.3 Player blocks (batch C)

**Module name**: `blocks`, `StorymakerCore.blocks`, in two classic-script files (600-line guard):
- `ui/js/storymaker/blocks.js`: model, areas, board backdrop, measurement, serialize/revive.
- `ui/js/storymaker/blocks-draw.js`: painters; adds `draw` and `drawBoard` to `SM.blocks`.

Both go into `scripts/storymaker-bundle-parts.js` `storymakerPlayerParts`, right after `'ui/js/storymaker/bubble.js'`, in this order.

**Exports (binding for D and E)**
```js
SM.blocks = {
  BOARD_ID: 'env.board',
  AREAS: ['full', 'left', 'right', 'top', 'center', 'board'],
  KINDS: ['icon', 'diagram', 'chart', 'headline', 'list'],
  ICON_STYLES: ['line', 'badge', 'card'],
  LAYOUTS: { flow: [2, 5], cycle: [3, 6], hub: [3, 7], stack: [2, 5], compare: [2, 2] },   // node count min/max; hub = centre + 2..6
  CHARTS: { bar: [1, 8], line: [2, 24], pie: [2, 6], counter: [1, 1] },                     // value count min/max
  LIST_MAX: 4, POINTS_MAX: 3, MAX_BLOCKS: 32,
  LABEL_BASE: 0.034,   // label font, fraction of the canvas height (like text.label)
  LABEL_MIN: 0.026,    // smallest label font, fraction of the canvas short side (= text.js FONT_FLOOR)
  ITEM_SPAN: 0.5, WORD_SPAN: 0.35, FLOW_PERIOD: 1.6,
  BOARD_RECTS: { ... },                      // see "Board areas"
  isBoard(sc),                               // !!(sc && sc.env && sc.env.drawn === 'board')
  area(stage, env, variant, at),             // -> {x, y, w, h, surface} design px, or null for an unknown name
  check(def, cat),                           // -> null | {field, message}: the first problem, the message names the fix
  create(def),                               // -> entity of type 'block'
  revealSeconds(def),                        // default entry/reveal span in seconds, before the rhythm factor
  itemProgress(st, k),                       // 0..1 reveal of item k (node, line, word; k = 0 for icon/chart)
  layoutNodes(st),                           // diagram: [{id, x, y, w, h}] design px at scale 1
  parseHeadline(text),                       // -> {words: [..], mark: index of the *word* or -1}
  measure(ctx, st, rc, stage, win),          // -> {box: {x, y, w, h} design px, labelPx: canvas px of the smallest label, 0 when none}
  serialize(entities, camera),               // -> plain payload or null (no block entities)
  revive(plain, cat),                        // -> {entities, camera} or null (invalid payload)
  draw(ctx, st, rc, stage, win),             // blocks-draw.js: paints one evaluated block state
  drawBoard(ctx, opts)                       // blocks-draw.js: opts {variant, palette, W, H}; full canvas, canvas px
};
```

**Block definition `def`** (input of `check` and `create`)
```js
{
  id: 'diagram-1',
  kind: 'diagram',                                    // one of KINDS
  icon: 'icon.cpu', style: 'card', label: 'CPU',      // kind icon; style default: 'card' with a label, else 'badge'
  diagram: { layout: 'flow', flow: true,
             nodes: [{ id: 'a', label: 'Anfrage', icon: 'icon.send', points: [] }] },   // points: compare only
  chart: { type: 'bar', labels: ['Mo'], values: [2], unit: '%', title: '' },
  text: 'Ein *wichtiges* Wort',                       // headline; list: lines joined by '\n'
  area: { x, y, w, h, surface },                      // from SM.blocks.area
  t0: 1.2, dur: 2.4,                                  // scene seconds; dur = revealSeconds(def) * rhythm factor unless given
  entry: 'draw',                                      // draw | pop | fade | slide | none
  reveals: [1.2, 2.0, 2.8] | null,                    // scene seconds per node (diagram) or line (list); null = spread over dur
  hidden: true                                        // alpha starts at 0
}
```
- `check` limits:
  - icon id must exist in `cat.icons`;
  - node ids are slugs, unique within the diagram;
  - labels 1..40 characters; at most 3 points of at most 40 characters each;
  - chart: labels and values the same length, values finite; pie values at least 0 with a positive sum; counter exactly 1 value; labels at most 24 characters; unit at most 8; title at most 40;
  - list 1..4 lines of at most 40 characters; headline at most 80 characters;
  - `reveals`, when given, has one finite number per node or line.
- `revealSeconds(def)`: icon 0.8; diagram 0.5 + 0.6 × nodes; chart 1.2; headline 0.4 + 0.15 × words; list 0.5 × lines.
- `itemProgress`:
  - diagram and list item k start at `reveals[k]` or `t0 + k*dur/n`;
  - the span is `ITEM_SPAN` (`min(ITEM_SPAN, dur/n)` without reveals; words use `WORD_SPAN`);
  - eased `ease-out`, clamped to 0..1;
  - icon and chart: `(time - t0)/dur`, eased, clamped.

**Entity** (`create`)
- `base('block')` of `scene.js`, with `x`, `y` the area centre and the tracks `alpha` (init 0 when `hidden`), `scale` and `rot`.
- Fields: `block` (the def without `area`, `t0`, `dur`, `entry`, `reveals`, `hidden`), `w`, `h`, `surface`, `t0`, `dur`, `entry`, `reveals`.
- `marks` (`Intervals`; data `{node, style}`) holds node highlights.
- `highlights` (`Intervals`) is the generic highlight path that already exists.
- `baseScale` is 1.

**`SM.scene.evalEntity`** (scene.js) state for a block adds:
- `block, w, h, surface, t0, dur, entry, reveals`;
- `time: t`;
- `mark: {node, style, u} | null`.

`entityEnd` includes `t0 + dur` and the ends of `marks`.

**Catalog index**: `SM.scene.indexCatalog` adds `idx.icons`, a map from id to `{paths, fills, tags}`, built from `catalog.icon_data` (in memory only, like `glyph_data`).

**Board areas** (design px; `S = SM.stage.safeBox(stage.aspect)`; `vis = stage.vis`)
- `'full'`: S.
- `'left'`: `{S.x, S.y, 0.48*S.w, S.h}`.
- `'right'`: `{S.x + 0.52*S.w, S.y, 0.48*S.w, S.h}`.
- `'top'`: `{S.x, S.y, S.w, 0.45*S.h}`.
- `'center'`: `{S.x + 0.2*S.w, S.y + 0.1*S.h, 0.6*S.w, 0.8*S.h}`.
- `{x, y}` (percent of the safe area): centred at `stage.safePoint(x, y)`, size `0.45*S.w × 0.5*S.h`, shifted to lie inside S.
- `'board'`:
  - On `env.board`: the same as `'full'`.
  - Else, when `BOARD_RECTS[env.id]` exists: the intersection of that rect, inset by 3 % per side, with S, if that intersection is at least 50 % of the rect's area. Its `surface` is `BOARD_RECTS[id].tone[variant]`.
  - Otherwise, the fallback:
    - 16:9: the right half of S, `{max(S.x, vis.x + vis.w/2), S.y, S.x + S.w - max(..), S.h}`;
    - 9:16 and 1:1: `'top'`.

`BOARD_RECTS` is measured on the delivered raster masters (shown 16:9 at 1920x1080):
```js
{ 'env.office':    { x: 810, y: 120, w: 642, h: 312,  tone: { day: 'light', night: 'light' } },  // pinboard, office-day.png
  'env.classroom': { x: 511, y: 101, w: 1025, h: 344, tone: { day: 'dark',  night: 'dark'  } } } // chalkboard, classroom-day.png
```
- The vector art in `assets/storymaker/lib/environments.mjs` (whiteboard 1054,184 582x470; HUD 1040,170 610x480) belongs to the legacy v1 catalog. That catalog is not delivered, so those numbers are not used.
- abstract-tech, and every other raster environment, has no board in its master and uses the fallback.

**Surfaces and colours**
- `surface` is one of:
  - `'panel'`: draw a rounded panel, `palette.bg` at alpha 0.94, radius 2 % of the shorter area side, behind the block;
  - `'light'`: no panel;
  - `'dark'`: no panel.
- Every area other than an applied board rect and other than `env.board` is `'panel'`.
- `env.board` is `'light'` by day and `'dark'` by night.
- Ink:
  - light and panel: ink `palette.text`, accent `palette.accent`, soft `palette.bg2`;
  - dark: ink `#f4f5eb`, accent `palette.accent2`, soft `rgba(255,255,255,0.10)`.
- Fonts: `SM.render.familyOf(rc, {font:'default'})`, weight 700 for headlines and numbers, 600 for labels.

**env.board**
- Catalog entry, generated by `raster-catalog.mjs` after the `ENVIRONMENT_IDS` loop:
```json
{"id":"env.board","kind":"environment","name":"env.board","name_key":"storymaker.catalog.env_board",
 "tags":["board","paper","slate","diagram","chart","whiteboard", ...NAMES],"thumb":"thumbs/env.board.png",
 "usage_example":{"environment":"env.board"},"file":"environments/board.json","resources":[],"drawn":"board",
 "spots":{"far-left":{"x":12,"y":88},"left":{"x":30,"y":88},"center":{"x":50,"y":88},"right":{"x":70,"y":88},"far-right":{"x":88,"y":88},
          "full":{"x":50,"y":50},"top":{"x":50,"y":25},"board":{"x":50,"y":50}},
 "floor":{"y_near":96,"y_far":70,"scale_near":1.05,"scale_far":0.8},"variants":["day","night"],
 "aspects":{"9:16":{"focus_x":50,"spots":{"far-left":{"x":38,"y":88},"left":{"x":42,"y":88},"right":{"x":58,"y":88},"far-right":{"x":62,"y":88}}},
            "1:1":{"focus_x":50,"spots":{"far-left":{"x":25,"y":88},"far-right":{"x":75,"y":88}}}}}
```
- `environments/board.json`: `{"viewbox":[0,0,1920,1080],"drawn":"board","variants":{"day":[],"night":[]},"tokens":{"day":{"text":"#1f2a44"},"night":{"text":"#f4f5eb","bg":"#1f2a44","bg2":"#2c3a5a"}}}`.
- Thumb `thumbs/env.board.png`: 256x144, `palette.sunny.bg` with the grain, through `encodePNG`.
- `drawBoard`, deterministic:
  - fill with `palette.bg` by day and `palette.fg` by night;
  - grain: 900 dots from `SM.seed.random('board-grain')`, positions as fractions of W and H, radius `(0.6 + 1.0*r)*W/1920`;
  - dot colour `palette.fg` by day and `palette.bg` by night, `globalAlpha` 0.008, so every pixel stays within 2/255 of the fill. An empty board still counts as `first_frame_empty` (`host-render.js:30` `UNIFORM_SPREAD`).
- Go: `const BoardEnvironmentID = "env.board"` in `studio_direction_data.go`, next to `plainStageID` (:13).
- `rankEnvironments` (`studio_direction.go:167-210`) leaves `BoardEnvironmentID` out.
- `data/env_direction.json` gains: `"env.board": {"about": "A calm drawn board (light paper by day, dark slate by night) for full-frame diagrams, charts, icons and headlines.", "tags": ["board","tafel","diagramm","diagram","chart"]}`.
- `assets/storymaker/lib/names.mjs` NAMES gains, in order cs..zh: `'env.board': ['Tabule','Tavle','Tafel','Πίνακας','Board','Pizarra','Tableau','बोर्ड','Lavagna','ボード','Bord','Tavle','Tablica','Quadro','Tavla','展板']`.
- Code scenes get the scene's environment:
  - `SM.planTiming` scenes gain `environment` (same rule as `compile.js:84`) and `variant` (`'night'` or `'day'`).
  - `host-core.js` `buildScene` opts (:231-236) gain `environment` and `variant`.
  - In `codescene.build`, `built.env = opts.environment === 'env.board' ? cat.byId['env.board'] || null : null` and `built.variant = opts.variant || 'day'`. When `built.env` is set, `state.needs` gets `'catalog:environments/board.json'`.
  - In `drawFrame` (`codescene.js:275-284`), `sceneLike.env = built.env` and `sceneLike.variant = built.variant`.
  - Code scenes on any other environment keep today's palette backdrop.

**Render integration**
- Canvas2D, `render.js` `drawScene` (:275-295):
  - after `drawBackdrop`: `if (SM.blocks.isBoard(sc)) SM.blocks.drawBoard(ctx, {variant: sc.variant, palette: rc.palette, W: rc.W, H: rc.H})`;
  - after the shapes and before the texts: `SM.blocks.draw(ctx, s, rc, stage, win)` for every live block state.
- Raster, `raster.js` `Drawer.draw` (:151-213):
  - block states join `visibleOverlay` and are drawn into the overlay after the shapes and before the texts;
  - the overlay key uses `SM.blocks.cacheKey(st)`, which includes `time` only while the block animates (`time < t0 + dur`, a mark is active, or `diagram.flow`);
  - board: a second canvas `this.underlay` with a sprite added before every other node (`zIndex` −1). It is redrawn with `drawBoard` when `[variant, palette.bg, palette.fg, W, H]` changes, and visible only when `isBoard(sc)`.
- Both entry points share this code. Beats (D) and `api.*` (E) only create `type:'block'` entities through `SM.blocks.create`. Code scenes are drawn by the same `SM.render.drawScene` inside their worker (Canvas2D or raster overlay), under the `onFrame` `g` drawing.

**Serialize / revive** (code-scene layout payload)
```js
{ v: 1,
  camera: <every Track field of the camera object as TRACK>,
  blocks: [{ id, block, w, h, surface, t0, dur, entry, reveals, x: TRACK, y: TRACK, alpha: TRACK, scale: TRACK, rot: TRACK,
             marks: [{t0, t1, node, style}], highlights: [{t0, t1, style}] }] }
// TRACK = { init: number, segs: [{ t0, t1, from, to, ease, log }] }   (serialize finalizes tracks first, so `from` is concrete)
```
`revive` rejects the whole payload (returns null) when any of these fails:
- `v !== 1`;
- more than `MAX_BLOCKS` blocks;
- more than 64 segments in a track;
- a non-finite number;
- a string over 200 characters;
- an unknown ease;
- a block whose `check` fails.

### 2.4 Storyboard entry, validation, measuring (batch D)

**Go model** (`model.go`)
```go
type ChartData struct {
    Labels []string  `json:"labels"`
    Values []float64 `json:"values"`
    Unit   string    `json:"unit,omitempty"`
    Title  string    `json:"title,omitempty"`
}
type Diagram struct {
    Layout string        `json:"layout"`          // flow | cycle | hub | stack | compare
    Nodes  []DiagramNode `json:"nodes"`
    Flow   bool          `json:"flow,omitempty"`
}
type DiagramNode struct {
    ID     string   `json:"id"`
    Label  string   `json:"label"`
    Icon   string   `json:"icon,omitempty"`
    Points []string `json:"points,omitempty"`  // compare only
    Cue    int      `json:"cue,omitempty"`     // narration marker; set by the Studio from cue_phrase
}
// Beat: insert `Diagram *Diagram `json:"diagram,omitempty"`` right after Data (key order of story.json).
```
- `phase1Beats` (`model.go:490`) adds `"chart"`.
- `BlockAreas = []string{"full", "left", "right", "top", "center", "board"}`.
- `showSlug` (`defaults.go:216-229`), new order: Item, then `Diagram != nil` → `"diagram"`, then Icon → `cleanSlug(Icon, "icon")`, then Chart → `"chart"`, then Text → `"text"`.

**New issue codes** (all in `model.go`; D owns them)
```go
const (
    CodeBlockTooSmall = "block_too_small" // warning, layout (IssueLayout): model-fixable, never blocks
    CodeLabelLong     = "label_long"      // warning, advice (IssueAdvisory, F): node label > 3 words or headline > 8 words
    CodeNoBlocks      = "no_blocks"       // warning, advice (F): film of >= 5 scenes without a block
    CodeBlockRepeat   = "block_repeat"    // warning, advice (F): the same block key in 3 scenes in a row
    CodeSceneFallback = "scene_fallback"  // open check of the job (F): the scene got the fallback board
)
```
Caption overlap, offscreen and actor overlap of blocks reuse `caption_overlap`, `offscreen` and `actor_prop_overlap`.

**Story JSON** (storyboard; the agent is taught `show` only)
```json
{"do":"show","icon":"icon.cpu","style":"card","text":"CPU","at":"board","entry":"draw","cue_phrase":"Prozessor"}
{"do":"show","diagram":{"layout":"flow","flow":true,"nodes":[{"id":"req","label":"Anfrage","icon":"icon.send","cue_phrase":"Anfrage"},{"id":"srv","label":"Server","icon":"icon.server"}]},"at":"full","entry":"draw"}
{"do":"show","chart":"bar","data":{"labels":["Warten","Rechnen"],"values":[90,10],"unit":"%","title":"Zeit"},"at":"full","entry":"fade"}
{"do":"show","text":"Kleine Modelle sind *schnell*","style":"headline","at":"top","entry":"fade"}
{"do":"show","text":"Laden\nRechnen\nAntworten","style":"list","at":"right","entry":"fade"}
{"do":"highlight","target":"diagram-1.srv","style":"pulse"}
{"do":"chart","chart":"bar","data":{"labels":["a"],"values":[1]},"at":"right"}            // legacy: plays like show, entry default fade
```

**Validation** (Go, new file `validate_blocks.go`; JS warnings follow the same rules)
- **Show kinds**: count the set fields among item, text, icon, chart and diagram. Text does not count when icon is set (it is the card label). Exactly 1 is allowed.
- **Icon**: id unknown in `cat.Icon` → error `unknown_asset` at `/icon`. `style` ∉ {line, badge, card} → error `invalid_value`. `text` set with a style other than `card` → warning `invalid_value`.
- **Text with style `headline` or `list`**: skip `bareName` (`validate.go:1103-1105`).
  - headline: more than 80 characters → error `limit_exceeded`; the number of `*` is neither 0 nor 2 → warning `invalid_value`;
  - list: more than 4 non-empty lines → error `limit_exceeded`; a line over 40 characters → warning `invalid_value`.
- **Chart**, on `show` and on the legacy `do:"chart"`:
  - type ∉ {bar, line, pie, counter} → error `invalid_value`; type missing → error `missing_field`; data missing → error `missing_field`;
  - labels and values of different length → error `invalid_value`;
  - count outside `CHARTS` → error `limit_exceeded`;
  - pie with a negative value or a zero sum → error `invalid_value`;
  - label over 24 / unit over 8 / title over 40 characters → warning `invalid_value`.
- **Diagram**:
  - layout ∉ LAYOUTS → error `invalid_value`; node count outside its range → error `limit_exceeded`;
  - node id not a slug, or duplicated → error `invalid_value` / `duplicate_id`;
  - label empty → error `missing_field`; label over 40 characters → error `limit_exceeded`;
  - unknown icon → error `unknown_asset`;
  - `points` on a layout other than compare → warning `invalid_value`; more than 3 points → error `limit_exceeded`;
  - `cue` not a marker → error `unknown_cue` (same text as beats).
- **Entry**: allowed values are pop, fade, slide, none and draw. `draw` on item, text or chart → warning `invalid_value` ("draw applies to icons and diagrams; the player fades it in.").
- **Block `at`**: a point (range check as now) or a name in `BlockAreas`, else error `invalid_value`. Block shows skip the environment spot check.
- **Highlight target**: `"<diagram instance id>.<node id>"`. `sceneRefs` (`validate.go:413-419`) gains `diagrams map[string][]string`. `checkTarget` (:1147) first tries the existing rules, then splits at the last `.`. A known diagram without that node → error `unknown_name` ("The diagram %s has no node %q.").
- The `not_supported_yet` notes for icons and charts (`validate.go:1106-1108`) are removed. A `chart` beat now plays.

**Node `cue_phrase`**
- `resolveCuePhrases` (`cuephrase.go:106-170`) also resolves `beats[i].diagram.nodes[k].cue_phrase` into `cue` plus a marker, with the same refusals.
- Their `where` is `beats[%d].diagram.nodes[%d]`.
- The early exit (:108) also looks at the nodes.

**JS beats**
- New file `ui/js/storymaker/beats-blocks.js` publishes `SM.beatBlocks = { isBlockBeat(b), target(c, b, j, helpers), reveals(c, b, def, t0, dur), highlightNode(c, b, j, t0, dur, find) }`.
- `beats.js` calls it in four places:
  - `showTarget` (:429) delegates block beats;
  - `kinds.show.dur` uses `SM.blocks.revealSeconds(def) * f` for blocks (0 with entry none);
  - `kinds.chart = kinds.show`;
  - `highlight.run` (:354-358) falls back to `highlightNode` when `find` fails.
- The `not_supported_yet` skip at `beats.js:484-487` is removed.
- Block entry animation: `entryAnim` (:187) as for props. `draw` sets `alpha` to 1 at t0, and the strokes reveal through `itemProgress`.
- Node reveals use `plan.narration.cues[node.cue]` when present, else they are spread.
- Instance ids follow Go `showSlug`: icon ids become slugs with dots as dashes (`icon.cpu` → `icon-cpu-1`).

**Layout JSON** (page to Go), per scene block:
```json
{"id":"s2","index":1,"hold_s":3.1,"code":false,"actors":[...],"props":[...],"texts":[...],
 "blocks":[{"id":"diagram-1","kind":"diagram","layout":"flow","label":0.031,"box":[0.05,0.05,0.9,0.74]}],
 "caption":[...],"missing":0,"static":false}
```
- `layout` is the diagram layout, the chart type or the icon style, and `""` for headline and list.
- `label` is `labelPx / min(W,H)`, 0 when the block has no label.
- `box` is in frame fractions (`frameBox` of `measure().box`).
- Go:
```go
type LayoutBlock struct {
    ID     string    `json:"id"`
    Kind   string    `json:"kind"`
    Layout string    `json:"layout"`
    Label  float64   `json:"label"`
    Box    LayoutBox `json:"box"`
}
// LayoutScene: add `Blocks []LayoutBlock `json:"blocks"`` after Texts.
const (
    layoutBlockMinHeight    = 0.10  // a block box below this share of the frame height is too small
    layoutBlockMinLabel     = 0.025 // a label below this share of the short side is too small
    layoutBlockCaptionShare = 0.02  // this share of a block box under the caption counts as covered
)
```
Rules per block, all warnings:
- `caption_overlap`: `sc.Caption != nil` and the intersection of the caption with the box, divided by the box area, is above 0.02. This holds for every film, not only composition v2.
- `offscreen`: `outsideShare(box) > layoutTextOutside`.
- `block_too_small`: `box[3] < 0.10`, or `0 < label < 0.025`.
- `actor_prop_overlap`: an actor box overlaps the block, `overlapShare > layoutOverlapShare`.

Fix texts:
- storyboard: `Show it at "full", "left", "right", "top" or "board" with patch_scene and the whole beats list.`;
- too small, storyboard: `Show it at "full" or on a bigger area, or give it fewer nodes or shorter labels.`;
- code scene: `Keep it inside api.safe(): change at in the source with put_code.`

`IssueLayout` (`layout.go:91-97`) adds `CodeBlockTooSmall`.

**Code-scene blocks**
- `worker-scene.js` build payload (:59) gains `blocks: SM.blocks.serialize(built.entities, built.camera)`.
- `host-core.js` `buildScene` returns `blocks: reply.blocks || null`. `load` stores `codeScenes[cid] = {duration, explicit, blocks}`. `shared_code_scenes` carries it along.
- `compile.js` sets `scene.codeLayout = codeInfo && codeInfo.blocks ? SM.blocks.revive(codeInfo.blocks, cat) : null`.
- `layout.js` measures `sc.codeLayout.entities` under `sc.codeLayout.camera`.
- Inside code scenes only block boxes and the caption are measured.

**App and locales** (D owns every key)
- `ISSUE_CODES` (`storymaker-core.js:338`): append `'block_too_small', 'label_long', 'no_blocks', 'block_repeat'` (38 codes).
- `ui/desktop_storymaker_guards_test.go:160-164`: the same 38 in order; comment "thirty-eight".
- `storymaker-agent.js` `CHECK_KEYS` (:41-64): add `block_too_small: 'storymaker.agent.check_layout_found'` and `scene_fallback: 'storymaker.agent.check_scene_fallback'`.
- Locale keys in all 16 `ui/lang/storymaker/*.json`:
  - the four issue keys go after `storymaker.issue.slow_hook`;
  - the agent key goes after `storymaker.agent.check_scene_unfilled`;
  - every language has its own words, never the English text.

| Key | en | de |
| --- | --- | --- |
| `storymaker.issue.block_too_small` | Block too small | Baustein zu klein |
| `storymaker.issue.label_long` | Label too long | Beschriftung zu lang |
| `storymaker.issue.no_blocks` | No explainer blocks | Keine Erklärbausteine |
| `storymaker.issue.block_repeat` | Same block three times | Dreimal derselbe Baustein |
| `storymaker.agent.check_scene_fallback` | This scene got a plain board with a headline because the author could not finish it. Revise the story to give it a real picture. | Diese Szene hat eine einfache Tafel mit einer Überschrift bekommen, weil der Autor sie nicht fertig bekam. Überarbeite die Geschichte, damit sie ein richtiges Bild bekommt. |

**Agent tool schema** (`internal/agent/native_tools_storymaker.go:285-315`)
- Beat properties gain:
  - `icon`: string;
  - `chart`: enum bar/line/pie/counter;
  - `data`: closed object `{labels: array of string, values: array of number, unit: string, title: string}`;
  - `diagram`: closed object `{layout: enum flow/cycle/hub/stack/compare, flow: boolean, nodes: array of closed {id: string, label: string, icon: string, points: array of string, cue_phrase: string, maxLength 60}}`, required `layout` and `nodes`; each node requires `id` and `label`.
- `entry` gains `draw`.
- `do` does not gain `chart`.
- No descriptions on the new fields. The compact story schema must stay at or below 900 tokens (test at `native_tools_storymaker_test.go:138`).

### 2.5 Code entry (batch E)

API (codescene.js):

| Call | Default `at` | Default `entry` | Other options |
| --- | --- | --- | --- |
| `api.icon(id, {at, style, label, entry, start, dur})` | `'center'` | `'draw'` | — |
| `api.diagram({layout, nodes, flow}, {at, entry, start, dur, reveals})` | `'full'` | `'draw'` | — |
| `api.chart({type, labels, values, unit, title}, {at, entry, start, dur})` | `'full'` | `'fade'` | — |
| `api.headline(text, {at, entry, start, dur})` | `'full'` | `'fade'` | — |
| `api.list(items, {at, entry, start, dur, reveals})` | `'full'` | `'fade'` | items is an array of 1..4 strings |
| `api.safe()` (from A) | — | — | — |

- `start` defaults to 0, and `dur` to `revealSeconds`.
- `at` is resolved with `SM.blocks.area(stage, built.env, built.variant, at)`. An unknown value throws a RangeError that lists `AREAS`.
- A `def` that fails `check` throws a RangeError with its message.
- More than `MAX_BLOCKS` blocks in a scene throws.
- Each call returns `common(e)` plus `highlight(nodeId, timing)`:
  - default dur 1.2, style pulse;
  - on a non-diagram, or with an unknown node id, it throws a RangeError naming the node ids.
- Font needs: the default font at 600 and 700, the way `api.text` adds them (`codescene.js:177-178`).
- The placeholder `api.chart` and `api.diagram` (:190-191) and the `grow` note (:200) are removed. `grow` becomes an unknown tween property.

### 2.6 Agent guidance and finishing (batch F)

- **Suggested block**: `func SuggestedBlock(narration, language string) string` in new `studio_blocks_hint.go`.
  - It returns `"flow" | "chart" | "cycle" | "stack" | "compare" | ""` and only works for `LanguageBase` de or en.
  - Text: `StripMarkers`, lower case, words split at every non-letter and non-digit.
  - A word matches when it equals the rule word, or, for rule words of at least 5 letters, when it starts with it.
  - Phrases match on the space-joined word list.
  - The first rule that matches wins:
    1. `%` in the text, or a word starting with `prozent` or `percent` → chart;
    2. de `statt`, `anstatt`, `im gegensatz`; en `instead`, `versus` → compare;
    3. de `kreislauf`, `schleife`; en `loop`, `cycle` → cycle;
    4. de `besteht aus`, `schichten`; en `consists of`, `layers` → stack;
    5. de `zuerst`, `dann`, `danach`; en `first`, `then` → flow;
    6. a number with 2 or more digits → chart.
  - Catalog packet: `packet["direction"]["suggested_block"]`. Free packet: `packet["suggested_block"]`, because free packets have no direction (`studio_free_packet_test.go:35`). Set only when not empty.
- **Advice**: `func LintBlocks(st *Story, sources map[string]string) []Issue` in new `blocks_lint.go`.
  - `sources` maps a code scene id to its source.
  - The block key of a scene is the first block:
    - storyboard: diagram layout, `chart`, `icon`, `headline` or `list`;
    - code: the first `\bapi\s*\.\s*(icon|diagram|chart|headline|list)\s*\(`; for a diagram, the first `layout\s*:\s*['"](flow|cycle|hub|stack|compare)['"]`, else `diagram`.
  - `no_blocks` (path `/scenes`) when there are at least 5 scenes and none has a key.
  - `block_repeat` at the third scene of each run of 3 equal keys.
  - `label_long` per node label over 3 words (path `.../beats/j/diagram/nodes/k/label`) and per headline over 8 words (path `.../beats/j/text`), storyboard only.
  - All are warnings. `IssueAdvisory` (`issues.go:111-119`) adds `CodeNoBlocks`, `CodeBlockRepeat`, `CodeLabelLong`.
  - `runCheck` (`studio_check.go:349`) calls it for create jobs only, after `craftIssues`. Sources are read through `Projects.LoadCodeSource`; a read error leaves that scene out.
- **Fallback board**:
  - `func FallbackHeadline(narration, title string) string`:
    - `StripMarkers(narration)`, or the title when that is empty;
    - cut at the first `. ! ? : ; 。 ！ ？` that is followed by whitespace or the end, keeping a final `?` or `!` (and `？` `！`);
    - keep the first 8 `strings.Fields` words;
    - trim trailing `,` `–` `-` and spaces;
    - a single token over 40 characters is cut to 40 characters;
    - empty means no fallback.
  - `func ApplyFallbackBoard(sc *Scene, headline, mode string) (source string)`, pure:
    - catalog: `Environment = BoardEnvironmentID`, Actors, Props and Kind cleared, `Beats = [{Do:"show", Text:headline, Style:"headline", At:SpotAt("full"), Entry:"fade"}]`;
    - free: `Kind = "code"`, `Environment = BoardEnvironmentID`, Actors, Props and Beats nil, and the returned source is `export default function scene(api) {\n  api.headline(` + json.Marshal(headline) + `, { at: 'full', entry: 'fade' });\n}\n`.
  - `func (s *Studio) FallbackBoards(ctx context.Context, sceneIDs []string) ([]string, error)`:
    - read-only means nil;
    - for each id that is unfilled: free mode first saves the source with `SaveCodeScene` (ifMatch is the existing etag or `MustNotExist`; a failing scene is skipped);
    - then it saves the story once and returns the ids that got a board.
  - `OutcomeInput` gains `Fallback []string`. `DecideOutcome` adds `scene_fallback` per id and sets `blocked = blocked || len(in.Fallback) > 0`.
  - Server: `storymakerAgentJob` gains `outlineScenes []string`, set in `runCreate` from the progress read after outline and cast. `finish` (:1159-1162), after `DropDrafts` and the blank check, runs `fallback := j.applyFallbackBoards(final, open.Unfilled ∩ j.outlineScenes)` and passes `Fallback: fallback` to `publishOutcome`. Revise jobs never fall back.
- **Free default environment**:
  - In `studioOutline` (`studio_story.go:210-224`), free mode sets `Environment: BoardEnvironmentID` on every new scene before `ApplyDefaults` (:260). Inheritance in `ApplyDefaults` does the rest.
  - Saved environments are never changed.
  - `ApplyDefaults` itself stays mode-free: it has 20 call sites and no mode.
- **Profiles**: profile names become `storymaker/v7/<phase>` and `storymaker/v7/free/<phase>`. The free prompt must still contain no `env.`, `char.` or `prop.`.

---

## 3. File ownership table

| File | Batch: change |
| --- | --- |
| `internal/server/storymaker_agent.go` | A: output budget, fill passes. F: `outlineScenes`, fallback in `finish` |
| `internal/server/storymaker_agent_budget_test.go` (new) | A |
| `internal/server/storymaker_agent_fill_order_test.go` (new) | A |
| `internal/server/storymaker_agent_fallback_test.go` (new) | F |
| `internal/server/storymaker_agent_profile_free.go` | A: code guide bullets 1-2 and a safe-box bullet, fill workflow, fill tool examples, name v6. F: rewrite, name v7 |
| `internal/server/storymaker_agent_profile.go` | F only (name v7 for both modes, guidance) |
| `internal/server/storymaker_agent_profile*_test.go` | A: free test. F: both |
| `internal/agent/native_tools_storymaker.go` | A: free `source` description (:189). B: catalog kinds plus `icon`, free catalog tool. D: beat schema |
| `internal/agent/native_tools_storymaker_test.go` | B, D |
| `internal/storymaker/narration.go`, `narration_test.go` | A |
| `internal/storymaker/cuephrase.go` | A: `cueTokens` skip. D: node `cue_phrase` |
| `internal/storymaker/studio_umlauts.go` | A |
| `internal/storymaker/safebox.go`, `safebox_test.go` (new) | A |
| `internal/storymaker/studio_apicard.go` (new) | A: create. E: replace the text constant |
| `internal/storymaker/studio_packet.go` | A: free packet fields. F: `suggested_block`, fill example |
| `internal/storymaker/layout.go`, `layout_test.go` | A: `Code`, static skip. D: `Blocks`, rules |
| `internal/storymaker/stills.go` | A: comment only (:188-193) |
| `internal/storymaker/studio_check.go` | A: `CheckReport.LayoutScenes` comment. F: `LintBlocks` call |
| `internal/storymaker/reference/code.md`, `code-free.md` | A: safe area. E: blocks API. F: decision-table section |
| `internal/storymaker/reference/free-motion.js` | E |
| `internal/storymaker/reference/animation.md` | F |
| `internal/storymaker/icons.go`, `icons_test.go` (new) | B |
| `internal/storymaker/catalog.go` | B |
| `internal/storymaker/studio_catalog.go`, `studio.go` | B |
| `internal/storymaker/render/render.go` | B: icons file in the v1 snapshot (next to :868) |
| `internal/storymaker/testdata/catalog-mini/catalog.json` | B: `"icons"` key. C: `env.board` entry |
| `internal/storymaker/testdata/catalog-mini/icons.json` (new) | B: 6 icons copied from the generated file (`icon.cpu`, `icon.server`, `icon.database`, `icon.send`, `icon.mail`, `icon.lock`) |
| `internal/storymaker/testdata/catalog-mini/environments/board.json` (new) | C |
| `internal/storymaker/studio_direction_data.go`, `studio_direction.go`, `data/env_direction.json` | C |
| `internal/storymaker/model.go`, `defaults.go` (`showSlug`) | D |
| `internal/storymaker/validate.go`, `validate_blocks.go` (new), `validate_test.go` | D |
| `internal/storymaker/issues.go` | D: `IssueLayout`. F: `IssueAdvisory` |
| `internal/storymaker/render/compilecheck.go`, `compilecheck_layout_test.go` | D |
| `internal/storymaker/agentjob.go` | F |
| `internal/storymaker/studio_story.go` | F |
| `internal/storymaker/studio_blocks_hint.go`, `blocks_lint.go`, `fallback.go` (new) | F |
| `internal/storymaker/render/blocks_browser_test.go` (new), `testdata/stories/blocks.json`, `testdata/scenes/blocks.js` (new) | F |
| `ui/js/storymaker/narration.js` | A |
| `ui/js/storymaker/stage.js` | A |
| `ui/js/storymaker/codescene.js` | A: `api.safe`. C: board env plumbing. E: `api.*` blocks |
| `ui/js/storymaker/layout.js` | A: code blocks, hold. D: block boxes, code layout |
| `ui/js/storymaker/blocks.js`, `blocks-draw.js` (new) | C |
| `ui/js/storymaker/scene.js`, `render.js`, `raster.js`, `plan.js` | C |
| `ui/js/storymaker/host-core.js` | C: build opts `environment`/`variant`. D: `blocks` in the build result and code info |
| `ui/js/storymaker/host-render.js`, `host-app.js` | C (load and validate `icon_data`) |
| `ui/js/storymaker/beats.js`, `beats-blocks.js` (new), `compile.js`, `worker-scene.js` | D |
| `scripts/storymaker-bundle-parts.js` | C: `blocks.js`, `blocks-draw.js`. D: `beats-blocks.js` (right after `blocks-draw.js`) |
| `ui/js/storymaker/*.bundle.js` | regenerated by every batch that changes a part (A, C, D, E) |
| `ui/js/desktop/apps/storymaker-assets.js` | C (load `icon_data`) |
| `ui/js/desktop/apps/storymaker-core.js`, `storymaker-agent.js`, `ui/lang/storymaker/*.json` (16), `ui/desktop_storymaker_guards_test.go` | D only |
| `ui/storymaker_raster_catalog_test.go` | C (skip the layer check for drawn environments) |
| `scripts/storymaker-test-support.mjs` | C: `miniCatalog()` attaches `icon_data` from `catalog-mini/icons.json`; `miniFiles().envs` gains `board.json` |
| `scripts/storymaker-tests/02-stage-parts-text.mjs`, `16-narration.mjs` | A |
| `scripts/storymaker-tests/30-layout.mjs` | A, D |
| `scripts/storymaker-tests/06-codescene.mjs` | A (`api.safe`), E (:46-56) |
| `scripts/storymaker-tests/33-blocks.mjs` (new) | C |
| `scripts/storymaker-tests/08-host-app.mjs`, `23-raster-contract.mjs` | C |
| `scripts/storymaker-tests/04-compile.mjs`, `34-block-beats.mjs` (new) | D |
| `scripts/storymaker-tests/35-block-api.mjs` (new) | E |
| `scripts/build-storymaker-icons.mjs` (new), `assets/storymaker/lib/icons.mjs` (new), `assets/storymaker/test/icons.test.mjs` (new), `assets/storymaker/icons/*` (new) | B |
| `assets/storymaker/lib/raster-catalog.mjs`, `raster-validate.mjs`, `scripts/build-storymaker-catalog.mjs`, `assets/storymaker/test/catalog.test.mjs` | B: icons. C: `env.board` |
| `assets/storymaker/lib/names.mjs` | C |
| `ui/img/storymaker/**`, `ui/lang/storymaker_catalog/*.json` | regenerated in B and C |
| `.gitattributes`, `licences/THIRD_PARTY_LIBRARIES.md` | B |
| `documentation/manuals/en/34-storymaker.md`, `de/34-storymaker.md` | F |
| existing AGENTS.md (`internal/storymaker`, `internal/storymaker/reference`, `ui/js/storymaker`, `assets/storymaker`) | F |

**Merge rules**
- Shared files are only ever touched by batches in chain order A → B → C → D → E → F.
- D and E are drafted in parallel on top of C. They share no file except the generated bundles. When E is rebased onto D, regenerate the bundles with `npm run build:ui` instead of merging them by hand.
- In `native_tools_storymaker.go`, A, B and D touch different functions (:186-194; :120-143; :285-315).

**Drafting and assembly order**
1. A and B in parallel from the base.
2. The chain gets A, then B rebased onto A.
3. C on A+B.
4. D and E in parallel on C.
5. The chain gets D, then E rebased onto D. Rebuild the bundles and run the E batch-end check on the chain.
6. F on the chain.

---

## 4. Tasks

### Batch A: reliable and reviewable code scenes

**A1: free fill, check and revise get the build output budget**
- Files: `internal/server/storymaker_agent.go` (new func near :410; line 410 calls it), new `internal/server/storymaker_agent_budget_test.go`.
- Test first: `TestStorymakerAgentOutputTokens`, a table over `storymakerAgentOutputTokens(llm.ModelLimits{ContextWindow, MaxOutputTokens: 131072}, mode, phase)`:
  - (free, fill, 204800) = 32768; (free, check, 204800) = 32768; (free, revise, 204800) = 32768;
  - (free, fill, 262144) = 65536; (free, outline, 204800) = 20480;
  - (catalog, fill, 204800) = 20480; (catalog, fill, 262144) = 20480;
  - (free, fill, {32768, 32768}) = 0.
- Command: `go test ./internal/server -run '(?i)(TestStorymakerAgentOutputTokens)' -count=1`.
- Red: build failure, undefined `storymakerAgentOutputTokens`. Green: PASS.
- Notes: the signature is in section 2.1. `runLoop` (:410) passes `j.studioJob.Mode, loop.phase`.

**A2: fill pass 1 tries every scene, pass 2 retries the unfilled ones**
- Files: `storymaker_agent.go` (`storymakerAgentLoop` :136, `runPhase` :758-781, `runCreate` :912-918, new `runFillPasses` and const), new `internal/server/storymaker_agent_fill_order_test.go`.
- Test first: `TestStorymakerAgentFillPassesRetryAfterTheOthers`.
  - Catalog mode, `newSMScriptEnv(t, smScriptNativeModel, client)`, the turns of `smScriptHappyTurns()` with the fill queue replaced by `[{prose:"Ich plane."}, {prose:"Noch nicht."}, smScriptStoryCall("put_scene","s2",smScriptSceneTwo()), smScriptStoryCall("put_scene","s1",smScriptSceneOne())]`.
  - Collect the scene id of every fill request: phase from `smScriptPhase`, then `regexp.MustCompile(\`"scene":\{"id":"([a-z0-9-]+)"\`)` on the user message. Collapse consecutive duplicates.
  - Assert the order `[s1 s2 s1]`, both scenes filled (`smScriptRequireFilled`), and outcome `ready_unverified`.
- Command: `go test ./internal/server -run '(?i)(TestStorymakerAgentFillPassesRetryAfterTheOthers)' -count=1`.
- Red: order `[s1 s2]`, scene s2 unfilled. Green: PASS.
- Notes: two prose replies end an attempt (recovery nudge once, `agent_loop_retry.go:30-46`). A model error in pass 1 is not noted; one in pass 2 is.

**A3: the JS narration parser ignores closing markers**
- Files: `ui/js/storymaker/narration.js:15-34`, `scripts/storymaker-tests/16-narration.mjs`.
- Test first: check `narration: {/n} closing markers vanish and set no cue`:
  - `parse('Daten{/1}. Mehr {1}Text {/1}hier')` gives plain `Daten. Mehr Text hier` and `cues` equal to `{1: 11}`;
  - `parse('a {/2} b').cues` is `{}`.
- Command: `npm run test:storymaker`. Red: plain keeps `{/1}`. Green: all ok.
- Notes: regex in section 2.1. Regenerate the bundles (`npm run build:ui`) in this commit.

**A4: the Go parser ignores closing markers**
- Files: `internal/storymaker/narration.go:37-142`, `cuephrase.go:25-28`, `narration_test.go`.
- Test first: `TestStripMarkersDropsClosingMarkers`. The same two strings as A3. The cue offset of `{1}` from `parseMarkers` is 11, and `cueTokens("Daten{/1} hier")` gives the words `daten` and `hier` only.
- Command: `go test ./internal/storymaker -run '(?i)(TestStripMarkersDropsClosingMarkers)' -count=1`.
- Red: plain keeps `{/1}`; the tokens hold `1`. Green: PASS.

**A5: the Studio removes closing markers when it saves**
- Files: `internal/storymaker/studio_umlauts.go:108-114`, new `internal/storymaker/studio_markers_test.go`.
- Test first: `TestPutSceneRemovesClosingMarkers`. Use the Studio write rig (`newStudioWriteRig`), job language `en`. `put_scene` on s1 with narration `Daten{/1}. Mehr {1}Text` stores `Daten. Mehr {1}Text`. An outline row with `{/2}` is stored without it.
- Command: `go test ./internal/storymaker -run '(?i)(TestPutSceneRemovesClosingMarkers)' -count=1`.
- Red: the stored narration keeps `{/1}`. Green: PASS.

**A6: Go computes the safe box per format**
- Files: new `internal/storymaker/safebox.go`, `safebox_test.go`.
- Test first: `TestSafeBoxForPinsTheThreeFormats`. The values from section 2.1; `""` and `"4:3"` give the 16:9 box.
- Command: `go test ./internal/storymaker -run '(?i)(TestSafeBoxForPinsTheThreeFormats)' -count=1`.
- Red: undefined. Green: PASS.

**A7: SM.stage.safeBox returns the same boxes**
- Files: `ui/js/storymaker/stage.js` (export at :166-169), `scripts/storymaker-tests/02-stage-parts-text.mjs`.
- Test first: check `stage: safeBox pins 16:9, 9:16, 1:1 like Go` with the three literal boxes.
- Command: `npm run test:storymaker`. Red: `safeBox` is not a function. Green: all ok.

**A8: api.safe() in code scenes**
- Files: `ui/js/storymaker/codescene.js` (api, after :151), `scripts/storymaker-tests/06-codescene.mjs`.
- Test first: a scene that calls `api.safe()` while building and stores the result in a text label. `build()` with aspect `9:16` yields `{x:691,y:125,w:493,h:581}`, and changing the returned object does not change the next call.
- Command: `npm run test:storymaker`. Red: `api.safe` is not a function. Green: all ok.
- Notes: rebuild the bundles. The codescene header comment (:3-6) names `safe`.

**A9: the free fill packet carries code_rev, safe_box and the API card**
- Files: `studio_packet.go:224-270`, new `studio_apicard.go`, `studio_free_packet_test.go`.
- Test first: `TestFreeFillPacketCarriesCodeRevCardAndSafeBox`, using `newFreeStudioRig`:
  1. fill s1 without source: `code_rev` is `"-"`, `safe_box` equals `SafeBoxFor(story aspect)`, `api_card` contains `api.safe()` and `Palette of this film: $bg #`, its length is at most 1600, and `next` starts with `put_code with expected_rev = code_rev`;
  2. `put_code` with `expected_rev` set to that `code_rev` and no `get_code` succeeds;
  3. the next packet's `code_rev` is the `rev` of that answer.
- Command: `go test ./internal/storymaker -run '(?i)(TestFreeFillPacketCarriesCodeRevCardAndSafeBox|TestFreePacketsCarryNoCatalogOrCast)' -count=1`.
- Red: missing fields. Green: both PASS.

**A10: the free profile reads the card instead of the reference**
- Files: `storymaker_agent_profile_free.go:35-57, 86-92`, `storymaker_agent_profile.go:218` (name `storymaker/v6/free/`), `internal/agent/native_tools_storymaker.go:189`, `storymaker_agent_profile_free_test.go:39,46-49`.
- Test first (update the existing test):
  - name contains `storymaker/v6/free/fill/`;
  - the fill prompt contains `code_rev` and `api_card` and does not contain `before the first scene`;
  - the nudge contains `put_code` and `code_rev` and not `search`.
- Command: `go test ./internal/server -run '(?i)(TestStorymakerFreeProfilesTellTheFreeStory)' -count=1`. Red: name v5, old text. Green: PASS.
- Notes, exact new texts:
  - Code guide bullet 1: `- The fill context has api_card (the code API in short), code_rev and safe_box. Read storymaker_catalog operation reference, topic code only when the card is not enough.`
  - Code guide bullet 2: `- put_code takes expected_rev and data.source only, within 8 KiB. In a fill, expected_rev is code_rev from the context; get_code reads a saved source, and a fresh rev after rev_mismatch. Invalid code keeps the saved source.`
  - New bullet: `- Keep every drawing inside safe_box (api.safe()); the captions sit below it.`
  - Fill workflow:
```
## Fill one scene
The context names one scene, plus previous_scene, next_scene, the aspect, code_rev, safe_box, api_card and max_code_bytes. Draw this scene in code.
1. Call storymaker_story with operation put_code, the scene_id, expected_rev set to code_rev and data.source: export default function scene(api) { ... } within max_code_bytes. Choose the one motif of this scene from its narration, draw it inside safe_box and move it on the cue markers.
2. If put_code refuses, fix the listed lines and call it again; after rev_mismatch call get_code for the fresh rev.
3. Activate the scene with put_scene, the scene_id and data {"kind":"code"}. The narration stays. An optional dry_run checks without saving.
A saved scene ends this phase: stop.
```
  - Fill examples, in order: the put_code example with `"expected_rev":"<code_rev of the context>"`, then the put_scene example, then the get_code example.
  - Free schema `source` description: `put_code only; expected_rev is code_rev from the context`.

**A11: the code references name the safe area, not x=710..1210**
- Files: `reference/code.md:39`, `reference/code-free.md:37`, `studio_free_reference_test.go`.
- Test first: `TestCodeReferencesNameTheSafeArea`. Neither `codeReference` nor `codeReferenceFree` contains `710..1210`; both contain `api.safe()`.
- Command: `go test ./internal/storymaker -run '(?i)(TestCodeReferencesNameTheSafeArea|TestFree.*Reference)' -count=1`.
- Red: the range is present. Green: PASS.
- Notes: replace the sentence with `keep key drawing inside api.safe() (the safe_box of the context), which is the visible part of the format above the captions`.

**A12: code scenes get a layout block with a hold frame**
- Files: `ui/js/storymaker/layout.js:16-89`, `internal/storymaker/layout.go:38-48, 174-177`, `stills.go:188-193` (comment), `studio_check.go:39` (comment), `scripts/storymaker-tests/30-layout.mjs` (:54-60 check), `layout_test.go`.
- Test first:
  - (JS) change the check `code scenes are left out` to `code scenes get a block with code true`. The film of `30-layout.mjs:58-60` (`codeScenes: {s2:{duration:3}}`) measures 2 blocks. The code block has `code === true`, `static === false`, empty actors, props and texts, and `hold_s` within 0.05 of `0.7 * D`. The storyboard block has `code === false`.
  - (Go) `TestLayoutIssuesSkipStaticForCodeScenes`: `LayoutScene{ID:"c", Code:true, Static:true}` gives no `scene_static`.
- Commands: `npm run test:storymaker`; `go test ./internal/storymaker -run '(?i)(TestLayoutIssuesSkipStaticForCodeScenes)' -count=1`.
- Red: 1 block; static reported. Green: all ok and PASS.
- Notes: stills, contact sheet, contrast, look-alike and `storymaker_view` follow the layout blocks (`compilestills.go:26`, `compileshots.go:23`). They need no code change. Rebuild the bundles.

### Batch B: icons

**B1: node-to-path converter**
- Files: new `assets/storymaker/lib/icons.mjs`, which exports `nodesToPaths(nodes) -> {paths, fills}`, `fmt(n)` and `buildIconData(pkgDir, allowList) -> {json, notice, count, problems}`; new `assets/storymaker/test/icons.test.mjs`.
- Test first: the conversion of each node kind:
  - `["circle",{"cx":"12","cy":"12","r":"10"}]` gives `M2 12A10 10 0 1 0 22 12A10 10 0 1 0 2 12Z`;
  - `["rect",{"x":"2","y":"4","width":"20","height":"16","rx":"2"}]` gives `M4 4H20A2 2 0 0 1 22 6V18A2 2 0 0 1 20 20H4A2 2 0 0 1 2 18V6A2 2 0 0 1 4 4Z`;
  - a line, a polyline, a polygon (with `Z`), an ellipse and a path pass through;
  - `fill:"currentColor"` goes to `fills`;
  - `transform` throws.
- Command: `node --test assets/storymaker/test/icons.test.mjs`. Red: module not found. Green: pass.

**B2: build script, allow-list and committed icon data**
- Files:
  - new `scripts/build-storymaker-icons.mjs`. Usage: `--package <dir>` writes; `--check` compares; `--verify <tgz>` checks the sha512 against the constant `LUCIDE_INTEGRITY`;
  - new `assets/storymaker/icons/allow-list.json` (section 2.2);
  - generated `lucide.json`, `LICENSE.txt` (verbatim copy of the package `LICENSE`) and `NOTICE.txt`;
  - `.gitattributes`.
- Test first: `icons.test.mjs` check `the committed icon data is complete`:
  - every allow-list id appears exactly once as `icon.<id>`, sorted;
  - the count is 490..530, the schema and source are as specified, the file is at most 256 KB;
  - every icon has paths and a tag equal to each of its categories;
  - every path matches the character set;
  - `NOTICE.txt` contains `ISC` and `MIT` and the integrity string.
- Command: `node --test assets/storymaker/test/icons.test.mjs`. Red: data file missing. Green: pass.
- Notes:
  - Run `node scripts/build-storymaker-icons.mjs --verify C:\disposable\sm-opt\lucide\lucide-static-1.82.0.tgz`, then `--package C:\disposable\sm-opt\lucide\pkg\package`.
  - The script reads only `icon-nodes.json`, `tags.json` and `LICENSE`.
  - An allow-list id that the package lacks fails with its name.
  - Output is LF, one icon per line.

**B3: the catalog delivers icons, licence and notice**
- Files:
  - `assets/storymaker/lib/raster-catalog.mjs`: near :130, `put('icons/lucide.json', ...)`, `put('icons/LICENSE.txt', ...)` and `put('icons/NOTICE.txt', ...)` as Buffers read from `assets/storymaker/icons/`; `manifest.icons = {file: 'icons/lucide.json'}`;
  - `raster-validate.mjs`: check that `manifest.icons.file` is in the delivery list;
  - `scripts/build-storymaker-catalog.mjs:22`: `SUBDIRS` gains `'icons'`;
  - `assets/storymaker/test/catalog.test.mjs`;
  - regenerated `ui/img/storymaker/**`;
  - `licences/THIRD_PARTY_LIBRARIES.md`: row `| Lucide icons (lucide-static 1.82.0) | ui/img/storymaker/icons/lucide.json | ISC; Feather-derived icons MIT; texts in adjacent LICENSE.txt and NOTICE.txt | https://lucide.dev |` next to the Storymaker rows (:215-217).
- Test first: catalog test `the delivered catalog names icons/lucide.json and ships LICENSE.txt and NOTICE.txt`.
- Commands: `node --test assets/storymaker/test/catalog.test.mjs`; `node scripts/build-storymaker-catalog.mjs --check`; `go test ./ui -run '(?i)storymakercatalog' -count=1`.
- Red: no `icons` key. Green: pass; check up to date; Go PASS.
- Notes: update any test that pins the old real catalog version.

**B4: Go loads the icon file**
- Files: new `internal/storymaker/icons.go`, `catalog.go` (`catalogFile` :202-212, load call near :333), `render/render.go` (next to :868: `if icons := cat.IconsFile(); icons != "" { files[icons] = true }`), testdata mini `icons.json` and its manifest key, new `icons_test.go`.
- Test first: `TestLoadCatalogReadsIcons`:
  - mini: 6 icons, `Icon("icon.cpu")` has paths;
  - real (`loadReal`): 490..530 icons;
  - a broken copy with a duplicate id fails `LoadCatalog`.
- Command: `go test ./internal/storymaker -run '(?i)(TestLoadCatalogReadsIcons)' -count=1`. Red: undefined. Green: PASS.

**B5: catalog search and describe return icons**
- Files: `catalog.go` (`kindAliases` :39-45, `Search` :701-751), `studio_catalog.go` (`catalogHitOf` :100, `catalogDescribe` :88, `catalogKindList` :14), `internal/agent/native_tools_storymaker.go:129` (kinds plus `"icon"`).
- Test first: `TestSearchFindsIcons`, in `icons_test.go` on the real catalog:
  - `Search("processor","icon",6)` contains `icon.cpu`;
  - `Search("lock","",12)` lists `prop.lock` before `icon.lock`;
  - the Studio search answer for an icon has `kind:"icon"` and the catalog `usage_example`;
  - describe `icon.cpu` works.
- Command: `go test ./internal/storymaker -run '(?i)(TestSearchFindsIcons)' -count=1`. Red: no icon hits. Green: PASS.

**B6: free mode searches icons only**
- Files: `studio.go:104-117` (free fill and revise `ToolCatalog: {"search","reference"}`), `studio_catalog.go` (free: kind forced to `icon`; usage example `{"code": ...}`; free miss `next_action`: `No icon matches. Search one concrete English word with kind icon, or draw the motif with g.path.`), `native_tools_storymaker.go:121-128` (free catalog tool: `query`, `kind` enum `icon`, `limit`; description `Search icons (kind icon, English query) or read the code reference with operation reference and topic code. A free project has no other catalog assets.`), new `studio_free_icons_test.go`, `internal/agent/native_tools_storymaker_test.go`.
- Test first: `TestFreeSearchFindsOnlyIcons`. A free rig in fill: search kind `prop`, query `lock` returns only `icon.*` hits, the first `usage_example.code` starts with `api.icon('icon.lock'`. Plus the agent test: the free fill catalog tool's operations are `[reference search]`.
- Commands: `go test ./internal/storymaker -run '(?i)(TestFreeSearchFindsOnlyIcons)' -count=1`; `go test ./internal/agent -run '(?i)storymaker' -count=1`.
- Red: refused (`search` is not offered). Green: PASS.

### Batch C: player blocks, env.board, board area

**C1: the player indexes and loads icon data**
- Files: `scene.js:10-35` (`idx.icons`), `host-render.js:152-156` (load `catalog.icons.file` into `catalog.icon_data`, path checks like glyphs), `host-app.js:51-95` (`checkIconData`: schema `storymaker-icons/1`, at most 1000 icons, ids `/^icon\.[a-z0-9-]{1,60}$/`, at most 40 paths of at most 2000 characters, else refused like bad glyph data), `ui/js/desktop/apps/storymaker-assets.js:30-31` (`raw.icon_data = await api.catalogJSON(raw.icons.file)`), `scripts/storymaker-test-support.mjs` (`miniCatalog` attaches `icon_data`), new `scripts/storymaker-tests/33-blocks.mjs`, `08-host-app.mjs`.
- Test first:
  - `blocks: indexCatalog indexes the icons` (`t.core().scene.indexCatalog(t.catalog).icons['icon.cpu'].paths.length > 0`);
  - host-app: a load with `icon_data.schema = 'x'` is refused.
- Command: `npm run test:storymaker`. Red: `icons` undefined. Green: all ok.

**C2: blocks.js model, areas and serialize/revive**
- Files: new `ui/js/storymaker/blocks.js`, `scene.js` (`evalEntity` block branch, `entityEnd`), `scripts/storymaker-bundle-parts.js`, `33-blocks.mjs`.
- Test first:
  - `area`: in 16:9, `full` is the safe box, `board` on the office entry is the inset rect (inside S), `board` on plain-stage is the right half of S, `board` on env.board equals `full`;
  - in 9:16, `left` and `right` lie inside S;
  - an unknown name gives null;
  - `check` refuses a 6-node flow, a pie with a negative value and an unknown icon, and names the fix;
  - `create` → `serialize` → `revive` gives an equal `evalEntity` state at t = 0, 1, 2;
  - `revive` of a payload with 33 blocks gives null.
- Command: `npm run test:storymaker`. Red: `SM.blocks` undefined. Green: all ok.
- Notes: the board rects are in section 2.3. Optionally verify them by decoding `office-day.png` and `classroom-day.png` (`png.mjs` `decodePNG`) in a scratchpad script; correct each edge by at most 12 px and record the measured values in the comment.

**C3: icon painting**
- Files: new `ui/js/storymaker/blocks-draw.js`, `33-blocks.mjs`.
- Test first:
  - every icon of `ui/img/storymaker/icons/lucide.json` draws without an error (fake ctx; one `stroke(path(...))` per path, one `fill` per fill);
  - the same t gives the same log digest;
  - `entry` `draw` at u = 0.5 sets `setLineDash([48, 96])` and at u ≥ 1 `setLineDash([])`;
  - the styles `line`, `badge` and `card` (with label) produce different logs.
- Command: `npm run test:storymaker`. Red: `draw` undefined. Green: all ok.
- Notes: strokes run on `Path2D` with round caps and joins, line width `2 * size/24`. Badge: a filled accent circle, light line. Card: rounded card, icon on top, label below.

**C4: diagram layouts, arrows, flow dots, node highlight**
- Files: `blocks.js` (`layoutNodes`), `blocks-draw.js`, `33-blocks.mjs`.
- Test first:
  - for flow 5, cycle 6, hub 7, stack 5 and compare 2: `layoutNodes` boxes lie inside the area and do not overlap; hub node 0 sits at the area centre; stack node 0 is the lowest;
  - `itemProgress` of node k is 0 before its reveal and 1 after reveal + `ITEM_SPAN`;
  - flow dots: the same t gives the same digest, t and t + 0.4 give different digests, and t and t + `FLOW_PERIOD` give the same digest;
  - an active mark on node `b` changes only that node's drawing (digest differs from no mark);
  - labels shrink but never below `LABEL_MIN * short`.
- Command: `npm run test:storymaker`. Red: undefined. Green: all ok.

**C5: charts**
- Files: `blocks-draw.js`, `33-blocks.mjs`.
- Test first:
  - bar: at the end the bar heights are proportional to the values (fake-ctx rect heights);
  - at u = 0.5 they are half;
  - counter: the text at u = 0.5 shows the eased intermediate value, rounded to the value's decimals, plus the unit;
  - a pie is drawn as arcs with an inner radius (donut);
  - line: dash reveal.
- Command: `npm run test:storymaker`. Red: undefined. Green: all ok.

**C6: headline and list**
- Files: `blocks.js` (`parseHeadline`), `blocks-draw.js`, `33-blocks.mjs`.
- Test first:
  - `parseHeadline('Ein *wichtiges* Wort')` gives words `[Ein, wichtiges, Wort]`, mark 1;
  - text without a marker gives mark −1;
  - words rise in order (`itemProgress` per word);
  - the marked word draws an accent marker rect behind it;
  - a list reveals 4 lines in order.
- Command: `npm run test:storymaker`. Red: undefined. Green: all ok.

**C7: measurement**
- Files: `blocks.js` (`measure`), `33-blocks.mjs`.
- Test first: for each kind, `measure().box` lies inside the area and `labelPx` is at least `LABEL_MIN * min(W,H)` at scale 1. With the entity scale 0.5, `labelPx` halves. Headline and list boxes are the text bounds.
- Command: `npm run test:storymaker`. Red: undefined. Green: all ok.

**C8: render and raster draw blocks**
- Files: `render.js:275-295`, `raster.js:151-213`, `33-blocks.mjs`.
- Test first: `SM.render.drawScene(ctx, {id, env:null, variant:'day', camera, entities:[block], stages:{}}, 1, rc)`. The log contains the block painting after the shapes and before the texts. The same t gives the same digest.
- Command: `npm run test:storymaker`. Red: no block calls. Green: all ok. Rebuild the bundles.
- Notes: the raster overlay and underlay are covered by the F1 browser test.

**C9: env.board in the catalog, direction and Go**
- Files: `raster-catalog.mjs` (entry after :116; file; generated thumb), `raster-validate.mjs:9-10,44-47` (count `ENVIRONMENT_IDS.length + 1`; a drawn board has `variants` `{day:[],night:[]}`), `names.mjs` (row), regenerated `ui/img/storymaker/**` and `ui/lang/storymaker_catalog/*.json`, `studio_direction_data.go` (const), `studio_direction.go:167-210` (exclusion), `data/env_direction.json`, mini catalog entry plus `environments/board.json`, `ui/storymaker_raster_catalog_test.go:175-213` (drawn environments: assert `drawn:"board"` and both token maps, skip layers), `scripts/storymaker-tests/23-raster-contract.mjs:23` (environment count 13), `studio_direction_test.go`.
- Test first:
  - `TestRankEnvironmentsLeavesTheBoardOut`: `rankEnvironments` on the real catalog has no `env.board`, while `cat.Names("environment")` has it;
  - `TestEnvironmentGuideCoversTheCatalog` passes;
  - catalog node test: the `env.board` entry has `drawn:"board"` and the spots `full`, `top`, `board`.
- Commands: `go test ./internal/storymaker -run '(?i)(TestRankEnvironmentsLeavesTheBoardOut|TestEnvironmentGuideCoversTheCatalog|TestDirection)' -count=1`; `node --test assets/storymaker/test/catalog.test.mjs`; `node scripts/build-storymaker-catalog.mjs --check`; `go test ./ui -run '(?i)storymaker' -count=1`.
- Red: no `env.board`. Green: PASS.

**C10: the board backdrop is drawn for storyboard and code scenes**
- Files: `blocks-draw.js` (`drawBoard`), `render.js` (backdrop), `raster.js` (underlay), `plan.js:154-170`, `host-core.js:231-236`, `codescene.js:36-40, 275-284`, `33-blocks.mjs`, `06-codescene.mjs`.
- Test first:
  - `drawBoard` day fills `palette.bg` and night fills `palette.fg`; every grain call uses `globalAlpha` 0.008; the same arguments give the same digest;
  - a storyboard scene on env.board draws the board fill first;
  - a code scene built with `opts.environment = 'env.board'` draws the board in `drawFrame`, and its needs contain `catalog:environments/board.json`;
  - one built with `env.office` draws no board.
- Command: `npm run test:storymaker`. Red: no board fill. Green: all ok. Rebuild the bundles.

### Batch D: storyboard entry, validation, measuring, app codes

**D1: model types and codes**
- Files: `model.go:262-299, 484-492` plus the new codes and `BlockAreas`; `defaults.go:216-229` (`showSlug` order); new `blocks_model_test.go`.
- Test first: `TestDecodeStoryKeepsDiagrams`:
  - a story with a diagram show, a chart with unit and title, and a node `cue` survives `DecodeStory` → `CanonicalJSON` byte for byte;
  - `deriveShowIDs` gives `diagram-1`, `icon-cpu-1` (icon with text) and `chart-1`.
- Command: `go test ./internal/storymaker -run '(?i)(TestDecodeStoryKeepsDiagrams)' -count=1`.
- Red: unknown field `diagram`. Green: PASS.

**D2: Go validates block shows**
- Files: new `validate_blocks.go`; `validate.go:850-852, 1021-1030, 1087-1120`; `validate_test.go:186-187`; new `validate_blocks_test.go`.
- Test first:
  - `TestValidateBlocks`: a table of valid cases (each JSON of section 2.4) and invalid cases with code, severity and path (every rule of section 2.4);
  - update `validate_test.go:186-187`: the chart beat has no `not_supported_yet` (valid); `{"do":"show","icon":"warning",...}` gives error `unknown_asset` at `/scenes/0/beats/0/icon`.
- Command: `go test ./internal/storymaker -run '(?i)(TestValidateBlocks|TestValidate)' -count=1`.
- Red: `not_supported_yet` is reported. Green: PASS.

**D3: highlight on a diagram node**
- Files: `validate.go:413-419, 1147-1180`, `validate_blocks_test.go`.
- Test first: `TestValidateNodeHighlight`. `diagram-1.srv` is valid; `diagram-1.zzz` gives error `unknown_name` with a message naming `zzz`; `prop.lock` as an item still resolves.
- Command: `go test ./internal/storymaker -run '(?i)(TestValidateNodeHighlight)' -count=1`. Red: unknown target. Green: PASS.

**D4: cue_phrase on diagram nodes**
- Files: `cuephrase.go:106-170`, `cuephrase_test.go`.
- Test first: `TestCuePhraseOnDiagramNodes`. Two nodes with `cue_phrase` get `cue` 1 and 2 and markers before those words; an unknown phrase is refused with `cue_phrase_not_found` and the path `beats[0].diagram.nodes[1]`.
- Command: `go test ./internal/storymaker -run '(?i)(TestCuePhraseOnDiagramNodes)' -count=1`. Red: `cue_phrase` left in place. Green: PASS.

**D5: beats play icons, charts, diagrams, headlines and lists**
- Files: new `ui/js/storymaker/beats-blocks.js`, `beats.js` (:340-343, 352-359, 429-452, 484-487), `compile.js` (block font and glyph needs near :186-197), `storymaker-bundle-parts.js`, `04-compile.mjs:105-121`, new `scripts/storymaker-tests/34-block-beats.mjs`.
- Test first:
  - (34) each JSON of section 2.4 creates one block entity with the expected kind and area;
  - node cues give `reveals` equal to the cue times; without cues the nodes spread over dur;
  - `highlight` with `diagram-1.srv` adds a mark;
  - the legacy chart beat creates a block with entry fade;
  - the needs contain `font:default/700`.
  - (04) beats 3, 4 and 5 have no `not_supported_yet`; beat 3 has `invalid_value` (empty data), beat 4 `unknown_asset`, beat 5 `missing_field`.
- Command: `npm run test:storymaker`. Red: `not_supported_yet`. Green: all ok. Rebuild the bundles.

**D6: the layout run measures blocks, in code scenes too**
- Files: `layout.js` (`measureScene`, `snapshot` types plus `block`, code blocks), `worker-scene.js:59`, `host-core.js:243, 287`, `compile.js:139`, `30-layout.mjs`.
- Test first:
  - a storyboard scene with a diagram at `full` gives `blocks[0]` with kind `diagram`, layout `flow`, `0 < label`, and a box inside `[0,1]`;
  - a code scene compiled with `codeScenes.s2 = {duration: 4, blocks: SM.blocks.serialize([headlineEntity], camera)}` gives `code: true` and one block of kind `headline`, with `hold_s ≥ lastBlockEnd + 0.5` clamped to the scene;
  - an invalid payload gives `blocks: []`;
  - a scene whose only change is a block is not static.
- Command: `npm run test:storymaker`. Red: no `blocks` key. Green: all ok. Rebuild the bundles.

**D7: Go layout rules for blocks**
- Files: `layout.go` (types, constants, rules, `IssueLayout`), `render/compilecheck.go:302-323` (`cleanLayoutScene` bounds `Blocks` to `layoutMaxBoxes` and clips ID, Kind and Layout to 60, 20 and 20), `layout_test.go`, `compilecheck_layout_test.go`.
- Test first:
  - `TestLayoutIssuesForBlocks`: each of the four rules fires once on a crafted scene; the code-scene fix text names `api.safe()`; a block inside S with label 0.03 gives nothing;
  - `TestSummarizeCompileReportBoundsBlocks`: 70 blocks are clipped to 64.
- Command: `go test ./internal/storymaker/... -run '(?i)(TestLayoutIssuesForBlocks|TestSummarizeCompileReport)' -count=1`. Red: no findings. Green: PASS.

**D8: the agent's beat schema has the block fields**
- Files: `internal/agent/native_tools_storymaker.go:285-315`, `native_tools_storymaker_test.go`.
- Test first: `TestStorymakerBeatSchemaHasBlocks`. For compact and non-compact fill: `beats[].icon`, `chart`, `data.unit`, `diagram.nodes[].cue_phrase` exist; entry includes `draw`; the compact story schema is at most 900 tokens (the existing check).
- Command: `go test ./internal/agent -run '(?i)storymaker' -count=1`. Red: fields missing. Green: PASS.

**D9: the app knows the new codes**
- Files: `storymaker-core.js:338`, `storymaker-agent.js:41-64`, the 16 `ui/lang/storymaker/*.json`, `ui/desktop_storymaker_guards_test.go:160-164`.
- Test first: update the pinned list to 38 codes.
- Command: `go test ./ui -run '(?i)(TestStorymakerAppUsesOnlyKeysItHas|TestStorymakerLocalesCoverAllSixteenLanguages|TestStorymakerAppTranslationsAreTranslated)' -count=1`.
- Red: `ISSUE_CODES` has 34. Green: PASS.

### Batch E: code entry

**E1: api.icon and block handles**
- Files: `codescene.js`, new `scripts/storymaker-tests/35-block-api.mjs`.
- Test first:
  - `api.icon('icon.cpu', {at: 'right', style: 'card', label: 'CPU', start: 1})` creates a block entity in the right area; `fadeOut`, `moveTo` and `scaleTo` change its tracks; the needs include the 600 and 700 fonts;
  - an unknown id throws, naming `storymaker_catalog`;
  - `at: 'nowhere'` throws, listing the areas.
- Command: `npm run test:storymaker`. Red: `api.icon` is not a function. Green: all ok.

**E2: api.diagram with highlight**
- Files: `codescene.js` (replace :191), `35-block-api.mjs`.
- Test first:
  - reveals `[api.cue(1), api.cue(2)]` are stored;
  - `highlight('b', {start: 3})` adds a mark;
  - `highlight('zzz')` throws, naming the node ids;
  - `highlight` on an icon handle throws;
  - a 6-node flow throws the `check` message;
  - there is no `not_supported_yet` issue.
- Command: `npm run test:storymaker`. Red: placeholder. Green: all ok.

**E3: api.chart; the placeholder notes go**
- Files: `codescene.js` (replace :190; remove the `grow` note :200), `06-codescene.mjs:46-56`, `35-block-api.mjs`.
- Test first:
  - in 06, replace the inert chart with `api.chart({type:'bar', labels:['a','b'], values:[1,2]}, {at:'right'})`; there are 4 entities, and the issues list is `[]`;
  - `api.tween(c, {grow: 1})` throws an unknown-property error;
  - in 35, a counter chart creates a block.
- Command: `npm run test:storymaker`. Red: still a placeholder. Green: all ok.

**E4: api.headline and api.list**
- Files: `codescene.js`, `35-block-api.mjs`.
- Test first: `api.headline('Kleine Modelle sind *schnell*')` creates a headline block in `full`; `api.list(['a','b','c','d','e'])` throws (more than 4); list reveals are stored; same-t determinism holds through `drawFrame`.
- Command: `npm run test:storymaker`. Red: not a function. Green: all ok. Rebuild the bundles.

**E5: the API card lists the blocks**
- Files: `studio_apicard.go`, `studio_free_packet_test.go`.
- Test first: `TestFreeAPICardNamesTheBlocks`. The card contains `api.icon(`, `api.diagram(`, `api.chart(`, `api.headline(`, `api.list(`, `highlight(nodeId, timing)` and `kind icon`, and its length is at most 2400.
- Command: `go test ./internal/storymaker -run '(?i)(TestFreeAPICardNamesTheBlocks|TestFreeFillPacketCarriesCodeRevCardAndSafeBox)' -count=1`.
- Red: no block lines. Green: PASS.
- Notes, exact insert after the `api.onFrame` line of the A text:
```
Blocks share the storyboard renderer and return a block handle:
api.icon(id, {at, style, label, entry, start, dur}): style is line, badge or card; find ids with storymaker_catalog operation search, kind icon, one English word.
api.diagram({layout, nodes, flow}, {at, entry, start, dur, reveals}): layout is flow (2-5 nodes), cycle (3-6), hub (centre + 2-6), stack (2-5, bottom first) or compare (2 columns); a node is {id, label (at most 3 words), icon, points (compare only, up to 3)}; flow: true moves dots along the arrows.
api.chart({type, labels, values, unit, title}, {at, entry, start, dur}): type is bar (up to 8), line (2-24), pie (2-6) or counter (1 value).
api.headline('One *marked* word', {at, entry, start, dur}) and api.list(['line', ...], {at, entry, start, dur, reveals}) with up to 4 lines.
A block handle has the handle methods above and highlight(nodeId, timing). entry is draw (icons and diagrams), pop, fade or none. reveals holds one start second per node or line, for example api.cue(2).
Blocks take at as full, left, right, top, center, board or {x, y}.
```

**E6: the references teach the blocks; the free example uses them**
- Files: `reference/code.md`, `reference/code-free.md` (remove :30 and code.md :32 "api.chart and api.diagram are placeholders"; add a section `## Explainer blocks` with the signatures of section 2.5), `reference/free-motion.js` (rewritten: at most 40 lines, uses `api.headline` and `api.diagram` with cues, `api.safe()`, and at most one small `onFrame` accent), `studio_free_reference_test.go`.
- Test first: `TestFreeReferenceTeachesTheBlocks`. Both guides contain `api.diagram` and do not contain `placeholders`; `freeMotionExample` contains `api.diagram(` and is shorter than before (fewer than 40 lines). Existing bans (`api.add(`, `char.`, `prop.`) still hold.
- Commands: `go test ./internal/storymaker -run '(?i)(TestFreeReferenceTeachesTheBlocks|TestFree.*Reference|TestFreeMotionExample)' -count=1`; `npm run test:storymaker` (24-agent-reference runs the example).
- Red: placeholder text. Green: PASS and all ok.

### Batch F: guidance, hints, advice, fallback, manual

**F1: the browser fixture film shows every block**
- Files: new `internal/storymaker/render/blocks_browser_test.go`, `internal/storymaker/testdata/stories/blocks.json`, `internal/storymaker/testdata/scenes/blocks.js`.
- Story: 16:9, palette sunny, captions on, compose v2, cast `anna` = `char.hoodie`. Eight scenes:
  1. env.office: icon card at board plus a `point` gesture;
  2. env.board: flow diagram with cue nodes and a node highlight;
  3. env.board, night: bar chart;
  4. env.board: cycle at left plus pie at right;
  5. env.board: headline at left plus list at right;
  6. env.board: hub at full;
  7. env.board: stack at left plus compare at right;
  8. a code scene (`blocks.js`): `api.headline` at top, a counter `api.chart` at left, `api.icon` at right, then at 4 s `fadeOut` and an `api.diagram` flow at full with `highlight`, plus `api.list`.
- Test: `TestCompileCheckBlocksBrowser`, run for aspects 16:9 and 9:16 (rewrite `format.aspect`), with `Settings.Stills`. Assert:
  - available, no errors, `LayoutError == ""`;
  - 8 layout blocks; scenes 1-7 each have at least 1 block, and scene 8 has `Code` and at least 1 block;
  - `LayoutIssues(got.Layout, StoryLayoutOptions(story))` has no `offscreen`, `block_too_small` or block `caption_overlap`;
  - 8 stills and a non-empty sheet; sheets go to `AURAGO_STILLS_OUT` when set.
- Command: `$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/storymaker/render -run '(?i)(TestCompileCheckBlocksBrowser)' -count=1 -timeout 20m`.
- Red: none (acceptance test of C-E). Without the env var it skips. Green: PASS.

**F2: free outlines start on the board**
- Files: `studio_story.go:210-224`, `studio_story_outline_test.go`.
- Test first: `TestFreeOutlineSeedsTheBoard`. A free outline stores `env.board` in every scene; a catalog outline stores `env.plain-stage` (the default).
- Command: `go test ./internal/storymaker -run '(?i)(TestFreeOutlineSeedsTheBoard)' -count=1`. Red: plain-stage. Green: PASS.

**F3: suggested_block in the fill packet**
- Files: new `studio_blocks_hint.go`, `studio_packet.go:211-213, 263`, new `studio_blocks_hint_test.go`.
- Test first: `TestSuggestedBlock`, a table:
  - `Zuerst X, dann Y` → flow; `first we X, then Y` → flow;
  - `90 Prozent` → chart; `90%` → chart;
  - `ein Kreislauf` → cycle; `a loop` → cycle;
  - `besteht aus drei Schichten` → stack; `consists of layers` → stack;
  - `statt X` → compare; `versus` → compare;
  - `Hallo Welt` → `""`; language `fr` with `Zuerst` → `""`.
  - Plus `TestFillPacketSuggestsABlock`: the catalog direction and the free packet carry it.
- Command: `go test ./internal/storymaker -run '(?i)(TestSuggestedBlock|TestFillPacketSuggestsABlock)' -count=1`. Red: undefined. Green: PASS.

**F4: the fill example uses a block when one is suggested**
- Files: `studio_packet.go:118-164` (`packetFillExample(cast, dir, block string)`: for a non-empty block, the show beat becomes a block of that kind at `board` with `entry` draw or fade, and the actor's gesture is `point` with `target: "board"`), `studio_direction_packet_test.go`.
- Test first: the example for `flow` has a `diagram` beat with layout flow and a gesture `point`; with `""` it is unchanged.
- Command: `go test ./internal/storymaker -run '(?i)(TestFillPacket|TestDirectionPacket)' -count=1`. Red: prop example. Green: PASS.

**F5: advice findings**
- Files: new `blocks_lint.go`, `issues.go:111-119`, `studio_check.go:349`, new `blocks_lint_test.go`.
- Test first: `TestLintBlocks`:
  - 5 scenes without blocks give one `no_blocks`;
  - three flows in a row give `block_repeat` at the third;
  - a code source with `api.diagram({layout:'flow'` counts as `flow`;
  - a 4-word node label and a 9-word headline give `label_long`;
  - all are warnings, and `IssueAdvisory` is true for each.
- Command: `go test ./internal/storymaker -run '(?i)(TestLintBlocks)' -count=1`. Red: undefined. Green: PASS.

**F6: fallback board (pure and Studio)**
- Files: new `fallback.go`, `fallback_test.go`.
- Test first:
  - `TestFallbackHeadline`: `Eine Firewall prüft jedes Paket am Eingang deines Netzes. Danach…` → `Eine Firewall prüft jedes Paket am Eingang deines`; `Was ist ein Agent? Er plant.` → `Was ist ein Agent?`; empty narration → the title;
  - `TestApplyFallbackBoard` (catalog and free shapes, the source text);
  - `TestStudioFallbackBoards`: the rig, catalog and free; read-only gives nil; Validate of the saved story has no errors; the free bundle exists.
- Command: `go test ./internal/storymaker -run '(?i)(TestFallbackHeadline|TestApplyFallbackBoard|TestStudioFallbackBoards)' -count=1`.
- Red: undefined. Green: PASS.

**F7: the job ends with a board instead of an empty frame**
- Files: `agentjob.go:67-140`, `storymaker_agent.go` (`outlineScenes`, `finish` :1159-1188), `agentjob_test.go`, new `internal/server/storymaker_agent_fallback_test.go`.
- Test first:
  - `TestDecideOutcomeListsSceneFallback`: `Fallback ["s2"]` gives check `{scene_fallback s2}`, no `scene_unfilled`, outcome `incomplete`;
  - `TestStorymakerAgentFallbackBoardAtJobEnd` (scripted, catalog): the fill turns fill only s1. The job ends incomplete with an open check `scene_fallback` for s2. The stored s2 has `env.board` and a headline show beat.
- Commands: `go test ./internal/storymaker -run '(?i)(TestDecideOutcome)' -count=1`; `go test ./internal/server -run '(?i)(TestStorymakerAgentFallbackBoardAtJobEnd|TestStorymakerAgentScripted)' -count=1`.
- Red: `scene_unfilled`. Green: PASS.

**F8: profiles and references teach one visual idea per scene**
- Files: `storymaker_agent_profile.go`, `storymaker_agent_profile_free.go` (names v7), `reference/animation.md` (:54 replaced), `code.md`, `code-free.md` (section `## Choosing the picture`), the profile tests.
- Each rewrite carries:
  - the decision table: steps or a process → flow; a loop → cycle; parts of a system → hub or stack; A versus B, or before and after → compare; numbers → chart or counter; a thing or a concept → icon card or prop; the key sentence → headline;
  - the variety rules: no block type three times in a row; 1-3 full-frame boards in a 7-scene film; in catalog mode the character points at the board;
  - the advice codes;
  - the free ground rules mention icon search (`kind icon`);
  - the catalog fill examples (`storymakerToolExamples[PhaseFill]`) gain a diagram show example;
  - the stale "chart and diagram helpers are unsupported" sentences go (`storymaker_agent_profile_free.go:38`, `storymaker_agent_profile.go:82`).
- Test first (update the existing tests): names v7; the prompts contain `"diagram"`, `flow` and `headline`; the free prompt still has no `env.`, `char.` or `prop.`; the check workflow names `block_repeat`.
- Command: `go test ./internal/server -run '(?i)(TestStorymaker.*Profile)' -count=1`. Red: v6/v5. Green: PASS.

**F9: user manual, en and de**
- Files: `documentation/manuals/en/34-storymaker.md` and `de/34-storymaker.md`. Add a new `### Explainer blocks` under `## The AI Author`, after the picture-checks paragraph (en :121-130). Add one sentence each to `### Two ways` (en :50-55; free mode now draws on a board and can use icons and blocks) and to the outcome text (fallback board, `scene_fallback`). Cover the Lucide licence (ISC, local, nothing loaded from the Internet), the new findings, and `env.board`.
- Test: none (documentation). Command: `go test ./internal/audit/... -count=1`. Red: none (docs). Green: PASS.

**F10: AGENTS.md contracts**
- Files: the existing `internal/storymaker/AGENTS.md`, `internal/storymaker/reference/AGENTS.md`, `ui/js/storymaker/AGENTS.md`, `assets/storymaker/AGENTS.md` (blocks module, icon data, `env.board`, safe box, fallback, `DecideOutcome` unchanged).
- Command: `go test ./internal/audit/... -count=1`. Red: none (docs). Green: PASS.

---

## 5. Batch-end checks (run in the batch worktree, Go env set)

**Common** (every batch):
```
go build ./...
go vet ./internal/storymaker/... ./internal/server/ ./internal/agent/
go test ./internal/storymaker/... -count=1
go test ./internal/server -run '(?i)storymaker' -count=1
go test ./internal/agent -run '(?i)storymaker' -count=1
go test ./ui -run '(?i)storymaker' -count=1
npm run test:storymaker
npm run build:ui
npm run check:ui        (restore ui/js/vendor/storymaker/LICENSES.txt if build:ui changed it)
node scripts/build-storymaker-ranges.mjs --check
```

**Additional**:
- B and C: `node --test assets/storymaker/test/icons.test.mjs assets/storymaker/test/catalog.test.mjs assets/storymaker/test/environments.test.mjs`; `node scripts/build-storymaker-catalog.mjs --check`.
- B: `node scripts/build-storymaker-icons.mjs --check --package C:\disposable\sm-opt\lucide\pkg\package`.
- F: the single browser test of the plan, `$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/storymaker/render -run '(?i)(TestCompileCheckBlocksBrowser|TestCompileCheckBrowser|TestCompileCheckStillsOfTheRealRuns)' -count=1 -timeout 30m`. It needs a local Chrome (`browserSmoke`, `browser_smoke_test.go:35-47`); `AURAGO_STILLS_OUT=<dir>` keeps the contact sheets. Also `go test ./internal/audit/... -count=1`.
- Assembled chain after E: rerun the common block once on the chain.

**One combined review per batch.** Leftover headless Chrome processes from the browser test: stop only the PIDs this test started.

---

## 6. Known baseline reds

- No Storymaker red is documented in AGENTS.md or in recent commits. **Measure on the base first.** Run the common block on the base SHA in worktree `C:\disposable\smb\base` and record the failures.
- Environment-only failures known on this box:
  - With GOTMPDIR on `D:` and the checkout on `C:`, `internal/server` `TestGetDesktopServiceReusesCanonicalConfigWithRelativeDefaults` fails (the `Rel` cross-drive error). Use `C:\tmp\go-build-smb`.
  - GOTMPDIR with forward slashes makes 7 path tests fail in security, agent, server and tools.
  - `internal/personalradio` `TestProductionCancellationAndDurableQuota` flaked once under full-suite load. It is outside the focused runs.
  - A missing `vet.exe` in the go1.27.3 toolchain cache: prefix `GOTOOLCHAIN=go1.27.4`.

---

## 7. Risks and open points (recommended default in bold)

1. **Board rectangles** were measured from 1672-px views of the masters (±10 px). **Use the values in 2.3. C2 may correct each edge by at most 12 px after decoding the masters and must note it.** Whether the office pinboard reads dark at night is unverified: **keep `light`; C10 looks at `office-night.png` once.**
2. **A compact story schema above 900 tokens** after D8. **Drop every description from the beat schema first, then share one node schema. Never raise the bound.**
3. **E without F** (if F slips): the profiles still say "chart and diagram helpers are unsupported" while E has shipped them. **Accept it; land C-F together.**
4. **List lines on cues in storyboard scenes** are not in the format: list lines spread over the beat; code scenes can pass `reveals`. **Keep that. The spec text "or on cues" is met by code scenes and by diagram node cues.**
5. **Free default environment**: `ApplyDefaults` has no mode and 20 call sites. **Seed `env.board` in free outlines only (F2). User-made free projects keep plain-stage for new scenes.** Ask the user whether that is enough.
6. **`layout_unverified` stays on every film with code scenes**, even with block boxes, because raw `g` drawing is never measured. **Keep `DecideOutcome` unchanged. The manual sentence "a free film is ready_unverified at best" stays true.**
7. **Untrusted code-scene payload** (`blocks` from the scene worker; scene code can patch `SM.blocks.serialize`). **`revive` validates strictly and Go bounds the boxes again (D7). A bad payload only costs that scene's block boxes.**
8. **Blocks are drawn above actors** (overlay layer), also on the office pinboard. **Accept it; the `actor_prop_overlap` rule warns.** Drawing behind actors would need a third raster layer.
9. **The free fallback overwrites an unactivated model source** of that scene. **Accept it; such a source was never shown.**
10. **The real catalog version changes twice** (B3, C9). Tests that pin it must be updated in the same commit. **Grep for the old 12-hex version before committing.**
11. **Icon search with kind `""`** now also returns icons. **Ties keep catalog entries first (`Entry.Name = ID`). The miss alternatives stay catalog-only.**
12. **`first_frame_empty` on an empty board**: the grain is bounded to 2/255, so the probe still sees one flat colour. **Keep the grain alpha at 0.008.**
