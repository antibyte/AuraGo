{% raw %}
# Part A: SFX, mixer, music and encode

Batch 2C. Run the tasks in order. `outline.md` holds the batch rules, the test matrices and the execution rules ("outline section N"). `interfaces.md` holds the frozen interfaces ("frozen section N"), and its section 10 and `reconcile.md` win over them. A group id such as `G-A-5` names a block of tasks inside a part; the index lists them.

<!-- G-A-1 -->

### Task A1: SFX files and provenance
**Batch:** 2C  **Depends on:** none
**Files:**
Create: `assets/storymaker/sfx/*.wav` (25, copied from `docs/superpowers/plans/2026-11-04-storymaker-phase2/spikes/sfx/set/`)
Modify: `assets/storymaker/README.md`
Modify: `.gitattributes`
Modify: `licences/THIRD_PARTY_LIBRARIES.md`
Test: `assets/storymaker/test/sfx.test.mjs`

- [ ] **Step 1: Write the failing test**

Create `assets/storymaker/test/sfx.test.mjs`:

```js
// Task A1: the 25 source WAVs of the Storymaker sound effects and their provenance table.
// Run: node --test assets/storymaker/test/sfx.test.mjs

import test from 'node:test';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const sfxDir = path.join(here, '..', 'sfx');
const readme = path.join(here, '..', 'README.md');

// Frozen catalog sfx entry: 4 + 4 + 4 + 3 + 2 + 2 + 6 = 25 takes.
const TAKES = { whoosh: 4, pop: 4, click: 4, ding: 3, success: 2, error: 2, typing: 6 };
const names = Object.entries(TAKES).flatMap(([n, k]) => Array.from({ length: k }, (_, i) => `${n}_${i + 1}.wav`));

function wavFormat(buf) {
  assert.equal(buf.toString('latin1', 0, 4), 'RIFF');
  assert.equal(buf.toString('latin1', 8, 12), 'WAVE');
  const out = { dataBytes: 0 };
  for (let at = 12; at + 8 <= buf.length;) {
    const id = buf.toString('latin1', at, at + 4);
    const size = buf.readUInt32LE(at + 4);
    if (id === 'fmt ') {
      out.format = buf.readUInt16LE(at + 8);
      out.channels = buf.readUInt16LE(at + 10);
      out.rate = buf.readUInt32LE(at + 12);
      out.bits = buf.readUInt16LE(at + 22);
    } else if (id === 'data') {
      out.dataBytes = size;
    }
    at += 8 + size + (size % 2);
  }
  return out;
}

function readmeRows() {
  const rows = new Map();
  const pattern = /^\| `sfx\/([a-z]+_\d+\.wav)` \| (\d+) \| ([0-9a-f]{64}) \|\r?$/gm;
  for (const m of fs.readFileSync(readme, 'utf8').matchAll(pattern)) rows.set(m[1], { bytes: Number(m[2]), sha256: m[3] });
  return rows;
}

test('the folder holds the 25 takes as 48 kHz mono PCM16', () => {
  assert.equal(names.length, 25);
  assert.deepEqual(fs.readdirSync(sfxDir).sort(), [...names].sort());
  for (const name of names) {
    const f = wavFormat(fs.readFileSync(path.join(sfxDir, name)));
    assert.deepEqual({ format: f.format, channels: f.channels, rate: f.rate, bits: f.bits }, { format: 1, channels: 1, rate: 48000, bits: 16 }, name);
    assert.ok(f.dataBytes > 0, name);
  }
});

test('every WAV has the sha256 and the size of its README row', () => {
  const rows = readmeRows();
  assert.deepEqual([...rows.keys()].sort(), [...names].sort());
  for (const name of names) {
    const bytes = fs.readFileSync(path.join(sfxDir, name));
    assert.equal(crypto.createHash('sha256').update(bytes).digest('hex'), rows.get(name).sha256, name);
    assert.equal(bytes.length, rows.get(name).bytes, name);
  }
});
```

- [ ] **Step 2: Run it**

```powershell
node --test assets/storymaker/test/sfx.test.mjs
```

Expected: both tests fail. The first fails with `ENOENT ... scandir` on `assets/storymaker/sfx`, because the folder does not exist yet. The second fails on `deepEqual`, because the README has no rows.

- [ ] **Step 3: Implement**

Copy the WAVs:

```powershell
New-Item -ItemType Directory -Force assets/storymaker/sfx | Out-Null
Copy-Item docs/superpowers/plans/2026-11-04-storymaker-phase2/spikes/sfx/set/*.wav assets/storymaker/sfx/
```

Make these edits in `assets/storymaker/README.md`. The sentences describe the state after A3, and the batch lands as a whole.

Replace:
```
see `THIRD_PARTY_LIBRARIES.md`); the catalog only references it by path.
```
with:
```
see `THIRD_PARTY_LIBRARIES.md`); the catalog only references it by path. The sound effects have their own
provenance, see Sound effects.
```

Replace:
```
LF-normalised bytes of every file below `ui/img/storymaker/` except `catalog.json`, taken in sorted path order.
```
with:
```
bytes of every file below `ui/img/storymaker/` except `catalog.json`, taken in sorted path order: text files
LF-normalised, WAV files raw.
```

Replace:
```
`thumbs/<id>.svg` (one per entry; characters are the rig posed in `stand`, three-quarter view, resolved colours),
```
with:
```
`sfx/<name>_<k>.wav` (byte copies of `assets/storymaker/sfx`),
`thumbs/<id>.svg` (one per entry except sfx; characters are the rig posed in `stand`, three-quarter view, resolved colours),
```

Replace:
```
All outputs are `.json` or `.svg` below `ui/img` and `ui/lang`, which the asset-set allowlist ships
```
with:
```
All outputs are `.json`, `.svg` or `.wav` below `ui/img` and `ui/lang`, which the asset-set allowlist ships
```

Replace:
```
The catalog stays within 30 MiB and 2000 files (spec); it currently is about 2.4 MiB in 105 files.
```
with:
```
The catalog stays within 30 MiB and 2000 files (spec); it has 134 files, 25 of them WAVs (1,058,520 bytes).
```

Replace:
```
## Style rules
```
with:
```md
## Sound effects

`sfx/*.wav` holds 25 takes of seven sounds. They are not downloads. Each take was rendered once at development time with
earshot 0.4.0 (git 591fa3e, bank forge_v2r6+forge04r1+fit1+pluck1). `scripts/build-storymaker-catalog.mjs` copies them
to `ui/img/storymaker/sfx/` and never renders, so the build needs no Python. The committed WAV is the source of truth,
because earshot promises byte-identical renders only on the machine that made them.

All takes are 48 kHz, mono, PCM16. Each take is normalised on its own: the loudest 50 ms (K-weighted, earshot's
`kit.loud_lufs`) go to -18 LUFS, with a sample-peak ceiling of -1.5 dBFS. Takes that hit the ceiling stay below -18 LUFS
(`click_2`, `click_4`, `typing_2`, `typing_5`, `typing_6`). The measured values of every take are in `SFX` in
`lib/catalog.mjs`.

| Sound | Takes | Length | Align | Recipe | Origin |
| --- | --- | --- | --- | --- | --- |
| whoosh | 4 | 477 ms | peak | `fit/whoosh.short/bamboo_swish` | fit of "Short Whoosh, 13x" by Kinoton, [fs-427979](https://freesound.org/people/Kinoton/sounds/427979/), CC0 1.0 |
| pop | 4 | 200 ms | peak | `pop/0` | forge, code-made, no recording |
| click | 4 | 132 ms | peak | `fit/click/floss_lid` | fit of "Plastic Clicks.flac" by qubodup, [fs-167829](https://freesound.org/people/qubodup/sounds/167829/), CC0 1.0 |
| ding | 3 | 900 ms | peak | `fit/bell/small_bell`, `dur` 0.9 | fit of "Small Bell #2" by steffcaffrey, [fs-452379](https://freesound.org/people/steffcaffrey/sounds/452379/), CC0 1.0 |
| success | 2 | 1120 ms | onset | three `fit/bell/small_bell` layers: G7 at 0 ms (`dur` 0.8), B7 at 110 ms (`dur` 0.8), D8 at 220 ms (`dur` 0.9) | same recording as ding |
| error | 2 | 500 ms | onset | `pro_zip/down`, `dur` 0.5 | forge, code-made, no recording |
| typing | 6 | 277 to 337 ms | peak | `typing_key/thock`, one key per take | forge, code-made, no recording |

Take k, counted from 0, renders every layer on the bank seed of its sound plus k. A take that fails `sfx.check` renders
again on seed + 1000; this happened once each for `error_2` and `typing_3`. The bank fixes `vary` at 0.3 for all fits.
The builder and its input are `build_set.py` and `set/recipe.json` in
`docs/superpowers/plans/2026-11-04-storymaker-phase2/spikes/sfx/`.

Rights: earshot's MIT licence covers its code and says nothing about rendered output, so the rights reading of the
renders stays open. A fit stores the structure of a recording, not its samples, and keeps the recording's licence.

Nobody has listened to this set. `error` reads synthetic (p_real 0.01) and `success` is uncertain (p_real 0.6).

`test/sfx.test.mjs` compares the files with this table.

| File | Bytes | SHA-256 |
| --- | --- | --- |
<!-- sfx-sha256 -->

## Style rules
```

Write this script to your session scratchpad as `sfx-table.mjs`. `sheet.json` holds only the first 16 hex digits of each hash, so the script computes the full hash, checks it against that prefix and fills the table:

```js
// Fills the SHA-256 table of assets/storymaker/README.md. Run from the worktree root.
import crypto from 'node:crypto';
import fs from 'node:fs';

const set = 'docs/superpowers/plans/2026-11-04-storymaker-phase2/spikes/sfx/set/';
const sheet = JSON.parse(fs.readFileSync(set + 'sheet.json', 'utf8'));
const rows = [];
for (const sound of Object.values(sheet)) {
  for (const take of sound.takes) {
    const bytes = fs.readFileSync('assets/storymaker/sfx/' + take.file);
    const sha = crypto.createHash('sha256').update(bytes).digest('hex');
    if (!sha.startsWith(take.sha256) || bytes.length !== take.bytes) throw new Error(take.file + ' differs from sheet.json');
    rows.push(`| \`sfx/${take.file}\` | ${bytes.length} | ${sha} |`);
  }
}
const readme = 'assets/storymaker/README.md';
const text = fs.readFileSync(readme, 'utf8');
if (!text.includes('<!-- sfx-sha256 -->')) throw new Error('the marker is missing');
fs.writeFileSync(readme, text.replace('<!-- sfx-sha256 -->', rows.join('\n')));
console.log(rows.length + ' rows');
```

Run it from the worktree root, with `<scratchpad>` as your session scratchpad path:

```powershell
node <scratchpad>/sfx-table.mjs
```

Expected output: `25 rows`. The marker is gone from the README afterwards.

In `.gitattributes`, replace:
```
bin/config-merger_linux_arm64 binary
```
with:
```
bin/config-merger_linux_arm64 binary

# Storymaker SFX: the README table and the catalog version pin their raw bytes.
*.wav   binary
```

In `licences/THIRD_PARTY_LIBRARIES.md`, replace:
```
https://commons.wikimedia.org/wiki/File:Pedestrians_using_crosswalk_(Unsplash).jpg |
```
with:
```
https://commons.wikimedia.org/wiki/File:Pedestrians_using_crosswalk_(Unsplash).jpg |
| Storymaker whoosh takes, rendered with earshot 0.4.0 from a fit of "Short Whoosh, 13x" by Kinoton | `assets/storymaker/sfx/whoosh_*.wav`, `ui/img/storymaker/sfx/whoosh_*.wav` | CC0 1.0 Universal | https://freesound.org/people/Kinoton/sounds/427979/ |
| Storymaker click takes, rendered with earshot 0.4.0 from a fit of "Plastic Clicks.flac" by qubodup | `assets/storymaker/sfx/click_*.wav`, `ui/img/storymaker/sfx/click_*.wav` | CC0 1.0 Universal | https://freesound.org/people/qubodup/sounds/167829/ |
| Storymaker ding and success takes, rendered with earshot 0.4.0 from a fit of "Small Bell #2" by steffcaffrey | `assets/storymaker/sfx/ding_*.wav`, `assets/storymaker/sfx/success_*.wav`, the same names in `ui/img/storymaker/sfx/` | CC0 1.0 Universal | https://freesound.org/people/steffcaffrey/sounds/452379/ |
| Storymaker pop, error and typing takes, code-made with earshot 0.4.0 forge sounds, no recording | `assets/storymaker/sfx/pop_*.wav`, `assets/storymaker/sfx/error_*.wav`, `assets/storymaker/sfx/typing_*.wav`, the same names in `ui/img/storymaker/sfx/` | earshot is MIT; the rights reading of rendered output is open, see `assets/storymaker/README.md` | earshot 0.4.0 (git 591fa3e) |
```

- [ ] **Step 4: Run**

```powershell
node --test assets/storymaker/test/sfx.test.mjs
git check-attr binary -- assets/storymaker/sfx/pop_1.wav
npm run check:storymaker-catalog
```

Expected:
- 2 tests pass.
- `git check-attr` prints `assets/storymaker/sfx/pop_1.wav: binary: set`.
- `--check` stays green, because the catalog does not read `assets/storymaker/sfx` yet.

- [ ] **Step 5: Commit**

Run `detect_changes` first. A PARTIAL answer on new files is accepted with a grep confirmation in the commit body (outline section 5).

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add 'assets/storymaker/sfx/*.wav' assets/storymaker/README.md .gitattributes licences/THIRD_PARTY_LIBRARIES.md assets/storymaker/test/sfx.test.mjs
git commit -m "feat(storymaker): add the SFX source WAVs and their provenance" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A4: Go catalog fields, on a test catalog
**Batch:** 2C  **Depends on:** none
**Files:**
Modify: `internal/storymaker/catalog.go`
Modify: `internal/storymaker/render/render.go`
Test: `internal/storymaker/catalog_test.go`

- [ ] **Step 1: Impact**

Run from the main checkout `C:/Users/Andi/Documents/repo/AuraGo`, because `.worktrees/*` is not registered:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "Entry" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "checkEntry" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "VerifyFiles" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "snapshot" --direction upstream --repo .
```

Outline section 5 applies: report callers, processes and risk, warn on HIGH or CRITICAL, and confirm UNKNOWN with a text search. `Entry` and `snapshot` have namesakes in other packages, so read only the answers for `internal/storymaker/catalog.go` and `internal/storymaker/render/render.go`.

- [ ] **Step 2: Write the failing test**

In `internal/storymaker/catalog_test.go`, replace:
```go
func TestCatalogVerifyFiles(t *testing.T) {
```
with:
```go
// sfxTestEntry is one sfx entry in the frozen shape, with an empty thumb and two takes.
const sfxTestEntry = `{"id":"sfx.pop","kind":"sfx","name":"sfx.pop","name_key":"storymaker.catalog.sfx_pop","tags":["pop"],"thumb":"",` +
	`"usage_example":{"do":"sfx","sound":"sfx.pop"},"file":"sfx/pop_1.wav","impact_ms":3.75,"align":"peak",` +
	`"hit":{"over_db":4.5,"dip_db":3},"source":{"kind":"forge","basis":"earshot 0.4.0 pop/0","url":""},` +
	`"takes":[{"file":"sfx/pop_1.wav","dur_ms":200,"onset_ms":0,"peak_ms":3.5,"impact_ms":3.5,"l50_lufs":-18,"true_peak_dbtp":-10},` +
	`{"file":"sfx/pop_2.wav","dur_ms":200,"onset_ms":0,"peak_ms":4.5,"impact_ms":4.5,"l50_lufs":-18,"true_peak_dbtp":-10.7}]}`

// sfxTestFS is a catalog with nothing but that entry, after an edit of its text.
func sfxTestFS(edit func(string) string) fstest.MapFS {
	doc := `{"schema":"storymaker-catalog/1","version":"sfx000000000",` +
		`"stage":{"width":1920,"height":1080,"safe":{"x":5,"y":5,"w":90,"h":90}},` +
		`"rig":{"id":"rig.human/1","file":"rig/human.json"},` +
		`"fonts":[{"name":"default","family":"Geist","weight":400,"file":"/fonts/Geist-Regular.woff2","licence":"OFL-1.1"}],` +
		`"palettes":[],"entries":[` + edit(sfxTestEntry) + `]}`
	return fstest.MapFS{"catalog.json": {Data: []byte(doc)}}
}

func TestCatalogSFXEntryLoads(t *testing.T) {
	c, err := LoadCatalog(sfxTestFS(func(s string) string { return s }), "catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	e, ok := c.Lookup("sfx.pop")
	if !ok || e.Thumb != "" || e.File != "sfx/pop_1.wav" || e.ImpactMS != 3.75 || e.Align != "peak" {
		t.Fatalf("entry: %+v %v", e, ok)
	}
	if len(e.Takes) != 2 || e.Takes[1].File != "sfx/pop_2.wav" || e.Takes[1].ImpactMS != 4.5 || e.Takes[1].TruePeakDBTP != -10.7 {
		t.Fatalf("takes: %+v", e.Takes)
	}
	if e.Hit == nil || e.Hit.OverDB != 4.5 || e.Hit.DipDB != 3 || e.Pattern != nil || e.Source == nil || e.Source.Kind != "forge" {
		t.Fatalf("hit %+v, pattern %+v, source %+v", e.Hit, e.Pattern, e.Source)
	}
	withPattern := func(s string) string {
		return strings.Replace(s, `"hit":{"over_db":4.5,"dip_db":3},`, `"pattern":{"spacing_ms":[90,140],"default_duration_ms":1200},`, 1)
	}
	c, err = LoadCatalog(sfxTestFS(withPattern), "catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	if e, _ = c.Lookup("sfx.pop"); e.Hit != nil || e.Pattern == nil || e.Pattern.SpacingMS != [2]float64{90, 140} || e.Pattern.DefaultDurationMS != 1200 {
		t.Fatalf("pattern: %+v", e.Pattern)
	}
}

func TestCatalogSFXEntryRefusals(t *testing.T) {
	cases := []struct {
		name string
		edit func(string) string
		want string
	}{
		{"one take", func(s string) string {
			return strings.Replace(s, `,{"file":"sfx/pop_2.wav","dur_ms":200,"onset_ms":0,"peak_ms":4.5,"impact_ms":4.5,"l50_lufs":-18,"true_peak_dbtp":-10.7}`, ``, 1)
		}, "at least two takes"},
		{"no align", func(s string) string { return strings.Replace(s, `"align":"peak",`, ``, 1) }, "align must be"},
		{"no source", func(s string) string {
			return strings.Replace(s, `"source":{"kind":"forge","basis":"earshot 0.4.0 pop/0","url":""},`, ``, 1)
		}, "source is required"},
		{"file is not the first take", func(s string) string {
			return strings.Replace(s, `"file":"sfx/pop_1.wav","impact_ms"`, `"file":"sfx/pop_2.wav","impact_ms"`, 1)
		}, "takes[0].file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.edit(sfxTestEntry) == sfxTestEntry {
				t.Fatal("the edit changed nothing")
			}
			_, err := LoadCatalog(sfxTestFS(tc.edit), "catalog.json")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v, want one containing %q", err, tc.want)
			}
		})
	}
}

func TestCatalogSFXVerifyFiles(t *testing.T) {
	c, err := LoadCatalog(sfxTestFS(func(s string) string { return s }), "catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{
		"img/storymaker/rig/human.json": {Data: []byte("{}")},
		"fonts/Geist-Regular.woff2":     {Data: []byte("font")},
		"img/storymaker/sfx/pop_1.wav":  {Data: []byte("RIFF")},
	}
	// pop_2.wav is the only absent file: the empty thumb is skipped, it would otherwise name the catalog directory.
	err = c.VerifyFiles(fsys, "img/storymaker")
	if err == nil || !strings.Contains(err.Error(), "1 missing files") || !strings.Contains(err.Error(), "img/storymaker/sfx/pop_2.wav") {
		t.Fatalf("a missing take must be the only missing file: %v", err)
	}
	fsys["img/storymaker/sfx/pop_2.wav"] = &fstest.MapFile{Data: []byte("RIFF")}
	if err := c.VerifyFiles(fsys, "img/storymaker"); err != nil {
		t.Fatalf("complete tree rejected: %v", err)
	}
}

func TestCatalogVerifyFiles(t *testing.T) {
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker -run 'TestCatalogSFX' -count=1
```

Expected: the package does not build, with `e.Takes undefined (type Entry has no field or method Takes)`, because `Entry` has no sfx fields yet.

- [ ] **Step 4: Implement**

In `internal/storymaker/catalog.go`, replace:
```go
// Entry is one catalog entry. Common fields and the environment data are typed; Raw keeps
```
with:
```go
// SFXTake is one rendered take of a sound effect, with its measured values.
type SFXTake struct {
	File         string  `json:"file"`
	DurMS        float64 `json:"dur_ms"`
	OnsetMS      float64 `json:"onset_ms"`
	PeakMS       float64 `json:"peak_ms"`
	ImpactMS     float64 `json:"impact_ms"`
	L50LUFS      float64 `json:"l50_lufs"`
	TruePeakDBTP float64 `json:"true_peak_dbtp"`
}

// SFXHit is the level of a sound above the bed and the dip it makes in the bed.
type SFXHit struct {
	OverDB float64 `json:"over_db"`
	DipDB  float64 `json:"dip_db"`
}

// SFXPattern makes a sound repeat as spaced events, like typing keys.
type SFXPattern struct {
	SpacingMS         [2]float64 `json:"spacing_ms"`
	DefaultDurationMS float64    `json:"default_duration_ms"`
}

// SFXSource records where a sound comes from.
type SFXSource struct {
	Kind  string `json:"kind"`
	Basis string `json:"basis"`
	URL   string `json:"url,omitempty"`
}

// Entry is one catalog entry. Common fields and the environment data are typed; Raw keeps
```

Replace:
```go
	Aspects      map[string]AspectOverride `json:"aspects,omitempty"`
	raw          json.RawMessage
```
with:
```go
	Aspects      map[string]AspectOverride `json:"aspects,omitempty"`
	Takes        []SFXTake                 `json:"takes,omitempty"`
	ImpactMS     float64                   `json:"impact_ms,omitempty"` // nominal; the player and the mix read the take's
	Align        string                    `json:"align,omitempty"`     // "peak" | "onset"
	Hit          *SFXHit                   `json:"hit,omitempty"`
	Pattern      *SFXPattern               `json:"pattern,omitempty"`
	Source       *SFXSource                `json:"source,omitempty"`
	raw          json.RawMessage
```

Replace:
```go
	if !safeRelPath(e.Thumb) || !strings.HasSuffix(e.Thumb, ".svg") {
```
with:
```go
	// An sfx entry has no image, so its thumb is empty.
	if e.Kind != "sfx" && (!safeRelPath(e.Thumb) || !strings.HasSuffix(e.Thumb, ".svg")) {
```

Replace:
```go
	case "sfx":
		if !safeRelPath(e.File) {
			fail("file must be a relative path")
		}
```
with:
```go
	case "sfx":
		if !safeRelPath(e.File) {
			fail("file must be a relative path")
		}
		if len(e.Takes) < 2 {
			fail("at least two takes are required")
		} else if e.File != e.Takes[0].File {
			fail("file must equal takes[0].file")
		}
		for j, take := range e.Takes {
			if !safeRelPath(take.File) {
				fail("takes[%d].file must be a relative path", j)
			}
		}
		if e.Align != "peak" && e.Align != "onset" {
			fail("align must be peak or onset")
		}
		if e.Source == nil {
			fail("source is required")
		}
```

Replace:
```go
		if e.File != "" {
			check(path.Join(dir, e.File))
		}
		check(path.Join(dir, e.Thumb))
```
with:
```go
		if e.File != "" {
			check(path.Join(dir, e.File))
		}
		for _, take := range e.Takes {
			if take.File != e.File {
				check(path.Join(dir, take.File))
			}
		}
		if e.Thumb != "" {
			check(path.Join(dir, e.Thumb))
		}
```

In `internal/storymaker/render/render.go`, replace:
```go
	for _, e := range cat.Entries() {
		if e.File != "" {
```
with:
```go
	for _, e := range cat.Entries() {
		if e.File != "" && e.Kind != "sfx" { // the page never reads WAVs
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker -run '(?i)catalog' -count=1
go build ./internal/storymaker/...
gofmt -l internal/storymaker
```

Expected: the tests pass, the build is silent and `gofmt -l` prints nothing.

- [ ] **Step 6: Commit**

Run `detect_changes` first. If the tool crashes, closes the connection, answers `not found`, or prints only its banner, use `graft` callers plus grep and say so in the commit body (outline section 5).

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/catalog.go internal/storymaker/render/render.go internal/storymaker/catalog_test.go
git commit -m "feat(storymaker): read and verify sfx fields in the Go catalog" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A2: Binary files in the catalog tooling, no output change
**Batch:** 2C  **Depends on:** A1
**Files:**
Modify: `assets/storymaker/lib/catalog.mjs`
Modify: `scripts/build-storymaker-catalog.mjs`
Test: `assets/storymaker/test/catalog.test.mjs`

`.mjs` files may check out with CRLF on this box (`scripts/build-storymaker-catalog.mjs` is CRLF in the main checkout), so every old text below is one line.

- [ ] **Step 1: Impact**

Run from the main checkout `C:/Users/Andi/Documents/repo/AuraGo`, because `.worktrees/*` is not registered:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "versionOf" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "write" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "check" --direction upstream --repo .
```

Outline section 5 applies: report callers, processes and risk, warn on HIGH or CRITICAL, and confirm UNKNOWN with a text search. `write` and `check` have many namesakes, so read only the answers for `scripts/build-storymaker-catalog.mjs`.

- [ ] **Step 2: Write the failing test**

In `assets/storymaker/test/catalog.test.mjs`, make these edits. The last edit fits the test to Buffer values, which A3 brings.

Replace:
```js
import { spawnSync } from 'node:child_process';
```
with:
```js
import crypto from 'node:crypto';
import { spawnSync } from 'node:child_process';
```

Replace:
```js
import { buildOutputs, versionOf } from '../lib/catalog.mjs';
```
with:
```js
import { buildOutputs, sameOutput, versionOf } from '../lib/catalog.mjs';
```

Replace:
```js
  for (const [p, text] of a.files) assert.equal(text, b.files.get(p), p);
```
with:
```js
  for (const [p, content] of a.files) assert.deepEqual(content, b.files.get(p), p);
```

Replace:
```js
  for (const text of a.files.values()) assert.ok(!text.includes('\r'), 'LF only');
```
with:
```js
  for (const text of a.files.values()) if (!Buffer.isBuffer(text)) assert.ok(!text.includes('\r'), 'LF only');
```

Replace:
```js
test('the generated catalog passes the build validator', () => {
```
with:
```js
test('versionOf hashes a Buffer raw and text LF-normalised, and sameOutput follows the same rule', () => {
  const hash = (bytes) => crypto.createHash('sha256').update(bytes).digest('hex').slice(0, 12);
  const crlf = Buffer.from([0x41, 0x0d, 0x0a, 0x42]); // the bytes of A CR LF B: in binary data that pair is not a line end
  const lfOnly = Buffer.from([0x41, 0x0a, 0x42]);
  assert.notEqual(hash(crlf), hash(lfOnly));
  assert.equal(versionOf(new Map([['sfx/a.wav', crlf]])), hash(crlf));
  assert.equal(versionOf(new Map([['a.json', 'A\r\nB']])), hash(lfOnly));
  assert.equal(sameOutput(crlf, Buffer.from(crlf)), true);
  assert.equal(sameOutput(lfOnly, crlf), false);
  assert.equal(sameOutput(crlf, 'A\nB'), true); // a text output survives a CRLF checkout
  assert.equal(sameOutput(Buffer.from('A\nC'), 'A\nB'), false);
});

test('the generated catalog passes the build validator', () => {
```

- [ ] **Step 3: Run it**

```powershell
node --test assets/storymaker/test/catalog.test.mjs
```

Expected: the file fails at load with `SyntaxError: The requested module '../lib/catalog.mjs' does not provide an export named 'sameOutput'`.

- [ ] **Step 4: Implement**

In `assets/storymaker/lib/catalog.mjs`, replace:
```js
/** sha256 over the LF-normalised bytes of every output except catalog.json in sorted path order, first 12 hex. */
```
with:
```js
/** True when the bytes on disk match an expected output: a Buffer byte for byte, text after LF normalisation. */
export function sameOutput(onDisk, expected) {
  return Buffer.isBuffer(expected) ? onDisk.equals(expected) : lf(onDisk.toString('utf8')) === expected;
}

/** sha256 over every output except catalog.json in sorted path order, text LF-normalised and Buffers raw, first 12 hex. */
```

Replace:
```js
  for (const p of [...files.keys()].filter((k) => k !== 'catalog.json').sort()) h.update(Buffer.from(lf(files.get(p)), 'utf8'));
```
with:
```js
  for (const p of [...files.keys()].filter((k) => k !== 'catalog.json').sort()) {
    const content = files.get(p);
    h.update(Buffer.isBuffer(content) ? content : Buffer.from(lf(content), 'utf8'));
  }
```

In `scripts/build-storymaker-catalog.mjs`, replace:
```js
import { buildOutputs } from '../assets/storymaker/lib/catalog.mjs';
```
with:
```js
import { buildOutputs, sameOutput } from '../assets/storymaker/lib/catalog.mjs';
```

Replace:
```js
const SUBDIRS = ['rig', 'props', 'environments', 'thumbs'];
```
with:
```js
const SUBDIRS = ['rig', 'props', 'environments', 'sfx', 'thumbs'];
```

Delete this whole line, because `check` was its only user:
```js
const lf = (s) => s.replace(/\r\n/g, '\n');
```

Replace:
```js
    fs.writeFileSync(p, text, 'utf8');
```
with:
```js
    fs.writeFileSync(p, text); // a string is written as UTF-8, a Buffer byte for byte
```

Replace:
```js
    else if (lf(fs.readFileSync(p, 'utf8')) !== text) problems.push(`differs: ${path.relative(root, p)}`);
```
with:
```js
    else if (!sameOutput(fs.readFileSync(p), text)) problems.push(`differs: ${path.relative(root, p)}`);
```

`listExisting` walks `SUBDIRS`, so it needs no edit, and it skips a folder that does not exist. `Buffer.byteLength` in `write` accepts a Buffer as it stands.

- [ ] **Step 5: Run**

```powershell
node --test assets/storymaker/test/catalog.test.mjs
node scripts/build-storymaker-catalog.mjs
git status --short ui/img/storymaker ui/lang/storymaker_catalog
npm run check:storymaker-catalog
```

Expected:
- All tests pass.
- `git status --short` prints nothing, so the build wrote the same bytes and `version` did not move.
- `--check` reports the catalog as up to date.

- [ ] **Step 6: Commit**

Run `detect_changes` first. If the tool crashes, closes the connection, answers `not found`, or prints only its banner, use `graft` callers plus grep and say so in the commit body (outline section 5).

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add assets/storymaker/lib/catalog.mjs scripts/build-storymaker-catalog.mjs assets/storymaker/test/catalog.test.mjs
git commit -m "feat(storymaker): let the catalog tooling hash, write and check binary files" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A3: Catalog sfx entries and the WAV copy
**Batch:** 2C  **Depends on:** A1, A2, A4
**Files:**
Modify: `assets/storymaker/lib/catalog.mjs`
Modify: `assets/storymaker/lib/names.mjs`
Modify: `assets/storymaker/lib/validate.mjs`
Modify: `internal/server/storymaker_parity_browser_test.go` (not in the outline row, see the issues block)
Create: `ui/img/storymaker/sfx/*.wav` (25, generated)
Modify: `ui/img/storymaker/catalog.json` (generated)
Modify: `ui/lang/storymaker_catalog/*.json` (16, generated)
Test: `assets/storymaker/test/catalog.test.mjs`
Test: `assets/storymaker/test/glyphs.test.mjs`
Test: `ui/storymaker_catalog_test.go`
Test: `internal/storymaker/catalog_real_test.go`
Test: `internal/storymaker/validate_test.go`
Test: `internal/storymaker/render/render_test.go`

`.mjs` files may check out with CRLF on this box, so every old text in an `.mjs` file below is one line. Go files are always LF.

- [ ] **Step 1: Impact**

Run from the main checkout `C:/Users/Andi/Documents/repo/AuraGo`, because `.worktrees/*` is not registered:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "buildEntries" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "buildThumbs" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "buildOutputs" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "validateOutputs" --direction upstream --repo .
```

Outline section 5 applies: report callers, processes and risk, warn on HIGH or CRITICAL, and confirm UNKNOWN with a text search.

- [ ] **Step 2: Write the failing test**

In `assets/storymaker/test/catalog.test.mjs`, as A2 and S5 left it, replace:
```js
import { spawnSync } from 'node:child_process';
```
with:
```js
import fs from 'node:fs';
import { spawnSync } from 'node:child_process';
```

Replace (S5 set this pin):
```js
  assert.equal(a.files.size, 109);
```
with:
```js
  assert.equal(a.files.size, 134); // 109 after batch 2B plus 25 WAVs
```

Replace (S5 set this pin):
```js
  assert.equal(entries.length, 79);
```
with:
```js
  assert.equal(entries.length, 86); // 79 after batch 2B plus 7 sfx
```

Replace:
```js
test('node scripts/build-storymaker-catalog.mjs --check is green on the committed outputs', () => {
```
with:
```js
test('the sfx entries follow the frozen shape', () => {
  const { entries, files } = buildOutputs();
  const sfx = entries.filter((e) => e.kind === 'sfx');
  // frozen catalog sfx entry: seven names, 4 + 4 + 4 + 3 + 2 + 2 + 6 = 25 takes
  assert.deepEqual(sfx.map((e) => [e.id, e.takes.length]), [['sfx.whoosh', 4], ['sfx.pop', 4], ['sfx.click', 4], ['sfx.ding', 3], ['sfx.success', 2], ['sfx.error', 2], ['sfx.typing', 6]]);
  const byId = Object.fromEntries(sfx.map((e) => [e.id, e]));
  assert.deepEqual(byId['sfx.typing'].pattern, { spacing_ms: [90, 140], default_duration_ms: 1200 });
  assert.equal(byId['sfx.typing'].hit, undefined);
  assert.equal(byId['sfx.error'].align, 'onset');
  assert.equal(byId['sfx.success'].align, 'onset');
  assert.equal(byId['sfx.pop'].align, 'peak');
  assert.deepEqual(byId['sfx.pop'].hit, { over_db: 4.5, dip_db: 3 });
  assert.deepEqual(byId['sfx.ding'].hit, { over_db: 4.5, dip_db: 3 });
  assert.deepEqual(byId['sfx.success'].hit, { over_db: 4.5, dip_db: 4 });
  assert.deepEqual(byId['sfx.error'].hit, { over_db: 4.5, dip_db: 4 });
  assert.equal(byId['sfx.whoosh'].hit, undefined);
  assert.equal(byId['sfx.click'].hit, undefined);
  assert.equal(byId['sfx.pop'].impact_ms, 3.75); // (3.5 + 4.5 + 3.5 + 3.5) / 4
  for (const e of sfx) {
    assert.equal(e.thumb, '', e.id);
    assert.equal(e.file, e.takes[0].file, e.id);
    for (const t of e.takes) assert.ok(Buffer.isBuffer(files.get(t.file)), t.file);
  }
});

test('--check fails when one byte of a copied WAV changes', () => {
  const wav = path.join(root, 'ui', 'img', 'storymaker', 'sfx', 'pop_1.wav');
  const original = fs.readFileSync(wav);
  const changed = Buffer.from(original);
  changed[changed.length - 1] ^= 1;
  fs.writeFileSync(wav, changed);
  try {
    const r = spawnSync(process.execPath, [path.join(root, 'scripts', 'build-storymaker-catalog.mjs'), '--check'], { cwd: root, encoding: 'utf8' });
    assert.equal(r.status, 1, r.stdout + r.stderr);
    assert.match(r.stderr, /differs: .*pop_1\.wav/);
  } finally {
    fs.writeFileSync(wav, original);
  }
});

test('node scripts/build-storymaker-catalog.mjs --check is green on the committed outputs', () => {
```

In `ui/storymaker_catalog_test.go`, replace:
```go
	"transition":  {"trans.cut", "trans.fade", "trans.slide-left", "trans.wipe"},
```
with:
```go
	"transition":  {"trans.cut", "trans.fade", "trans.slide-left", "trans.wipe"},
	"sfx":         {"sfx.whoosh", "sfx.pop", "sfx.click", "sfx.ding", "sfx.success", "sfx.error", "sfx.typing"},
```

Replace:
```go
	"mouth": "mouth.", "cycle": "cycle.", "text_style": "text.", "transition": "trans.",
```
with:
```go
	"mouth": "mouth.", "cycle": "cycle.", "text_style": "text.", "transition": "trans.", "sfx": "sfx.",
```

Replace:
```go
		if smStr(e, "thumb") != "thumbs/"+id+".svg" {
			t.Errorf("%s: thumb %q", id, smStr(e, "thumb"))
		} else if _, err := os.Stat(filepath.Join(storymakerCatalogDir, "thumbs", id+".svg")); err != nil {
```
with:
```go
		if kind == "sfx" {
			if smStr(e, "thumb") != "" {
				t.Errorf("%s: an sfx entry has no thumb, got %q", id, smStr(e, "thumb"))
			}
		} else if smStr(e, "thumb") != "thumbs/"+id+".svg" {
			t.Errorf("%s: thumb %q", id, smStr(e, "thumb"))
		} else if _, err := os.Stat(filepath.Join(storymakerCatalogDir, "thumbs", id+".svg")); err != nil {
```

Replace:
```go
			"cycle": {"period", "keys"}, "text_style": {"font", "size", "weight", "color", "align", "max_lines"}, "transition": {"duration"},
```
with:
```go
			"cycle": {"period", "keys"}, "text_style": {"font", "size", "weight", "color", "align", "max_lines"}, "transition": {"duration"}, "sfx": {"file", "takes", "impact_ms", "align", "source"},
```

Replace:
```go
		h.Write(smLF(data))
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".json" && ext != ".svg" {
			t.Errorf("%s: extension %s is not .json or .svg", p, ext)
		}
```
with:
```go
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".wav" {
			h.Write(data) // the build hashes WAV files raw
		} else {
			h.Write(smLF(data))
		}
		if ext != ".json" && ext != ".svg" && ext != ".wav" {
			t.Errorf("%s: extension %s is not .json, .svg or .wav", p, ext)
		}
```

Replace:
```go
		referenced[smStr(e, "thumb")] = true
		if f := smStr(e, "file"); f != "" {
			referenced[f] = true
		}
```
with:
```go
		referenced[smStr(e, "thumb")] = true
		if f := smStr(e, "file"); f != "" {
			referenced[f] = true
		}
		takes, _ := e["takes"].([]any)
		for _, raw := range takes {
			take, _ := raw.(map[string]any)
			referenced[smStr(take, "file")] = true
		}
```

Replace:
```go
		for _, ext := range []string{".json", ".svg"} {
```
with:
```go
		for _, ext := range []string{".json", ".svg", ".wav"} {
```

In `internal/storymaker/catalog_real_test.go`, replace:
```go
		"env.home-office", "env.abstract-tech", "prop.lock", "prop.shield", "pose.typing", "gesture.point", "expr.surprised", "cycle.walk", "text.caption", "trans.wipe"} {
```
with:
```go
		"env.home-office", "env.abstract-tech", "prop.lock", "prop.shield", "pose.typing", "gesture.point", "expr.surprised", "cycle.walk", "text.caption", "trans.wipe", "sfx.pop", "sfx.typing"} {
```

Replace:
```go
	for _, kind := range []string{"pose", "gesture", "expression"} {
```
with:
```go
	if e, _ := cat.Lookup("sfx.typing"); e.Pattern == nil || e.Hit != nil || len(e.Takes) != 6 {
		t.Errorf("sfx.typing: pattern %+v, hit %+v, %d takes", e.Pattern, e.Hit, len(e.Takes))
	}
	for _, kind := range []string{"pose", "gesture", "expression", "sfx"} {
```

In `internal/storymaker/validate_test.go`, replace:
```go
func TestValidateIssueShape(t *testing.T) {
```
with:
```go
// With sfx entries in the real catalog, the guard at validator.checkAsset no longer skips sounds.
func TestValidateSoundOnTheRealCatalog(t *testing.T) {
	cat := loadReal(t)
	s := storyOf(t, `{"id":"s1","beats":[{"do":"sfx","sound":"sfx.nope"}]}`)
	expectIssue(t, Validate(&s, cat, DefaultLimits()), CodeUnknownAsset, SeverityError, "/scenes/0/beats/0/sound")
	s = storyOf(t, `{"id":"s1","beats":[{"do":"sfx","sound":"sfx.pop"}]}`)
	if is := findIssue(Validate(&s, cat, DefaultLimits()), CodeUnknownAsset, SeverityError, "/scenes/0/beats/0/sound"); is != nil {
		t.Fatalf("sfx.pop is in the catalog: %+v", is)
	}
}

func TestValidateIssueShape(t *testing.T) {
```

In `internal/storymaker/render/render_test.go`, replace:
```go
func TestRunRendersAndPublishesTheVideo(t *testing.T) {
```
with:
```go
func TestSnapshotHoldsNoWAV(t *testing.T) {
	ui := os.DirFS(uiDirForTests)
	cat, err := storymaker.LoadCatalog(ui, "img/storymaker/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cat.Lookup("sfx.pop"); !ok {
		t.Fatal("the real catalog has no sfx.pop")
	}
	run := &renderRun{r: NewRunner(Deps{UI: ui, Catalog: cat}), plan: &Plan{StoryJSON: []byte("{}")}}
	snap, err := run.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snap.CatalogFiles["catalog.json"]; !ok {
		t.Error("the snapshot lost catalog.json")
	}
	for rel := range snap.CatalogFiles {
		if strings.HasSuffix(rel, ".wav") {
			t.Errorf("the snapshot holds %s", rel)
		}
	}
}

func TestRunRendersAndPublishesTheVideo(t *testing.T) {
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
node --test assets/storymaker/test/catalog.test.mjs
go test ./internal/storymaker -run 'TestValidateSoundOnTheRealCatalog' -count=1
go test ./internal/storymaker/render -run 'TestSnapshotHoldsNoWAV' -count=1
go test ./ui -run '(?i)storymakercatalog' -count=1
```

Expected failures:
- **Node:** `files.size` is 109, not 134. `entries.length` is 79, not 86. The sfx list is empty. The `--check` test fails with `ENOENT` on `ui/img/storymaker/sfx/pop_1.wav`.
- **Validate test:** no `unknown_asset` at `/scenes/0/beats/0/sound`, because the guard in `checkAsset` returns while the catalog has no sfx names.
- **Snapshot test:** `the real catalog has no sfx.pop`.
- **`./ui`:** `required sfx sfx.whoosh missing`, and the same for the other six.

- [ ] **Step 4: Implement**

In `assets/storymaker/lib/catalog.mjs`, replace:
```js
import crypto from 'node:crypto';
```
with:
```js
import crypto from 'node:crypto';
import { readFileSync as readSfxFile } from 'node:fs';
```

Replace:
```js
/** All manifest entries in a stable order. */
```
with:
```js
// The measured takes of assets/storymaker/sfx, copied from the render sheet (README, "Sound effects").
// One row per take: [dur_ms, onset_ms, peak_ms, impact_ms, l50_lufs, true_peak_dbtp]. impact_ms is the peak, or the onset with align "onset".
const SMALL_BELL = 'https://freesound.org/people/steffcaffrey/sounds/452379/';
const SFX = {
  whoosh: {
    tags: ['whoosh', 'swish'], align: 'peak',
    source: { kind: 'fit', basis: 'earshot 0.4.0 fit/whoosh.short/bamboo_swish', url: 'https://freesound.org/people/Kinoton/sounds/427979/' },
    takes: [[477, 87, 129.5, 129.5, -18, -11.5], [477, 89, 129.5, 129.5, -18, -12.2], [477, 88.8, 129.5, 129.5, -18, -12.2], [477, 89.2, 130.5, 130.5, -18, -11.5]],
  },
  pop: {
    tags: ['pop'], align: 'peak', hit: { over_db: 4.5, dip_db: 3 },
    source: { kind: 'forge', basis: 'earshot 0.4.0 pop/0', url: '' },
    takes: [[200, 0, 3.5, 3.5, -18, -10], [200, 0, 4.5, 4.5, -18, -10.7], [200, 0, 3.5, 3.5, -18, -11], [200, 0, 3.5, 3.5, -18, -10]],
  },
  click: {
    tags: ['click'], align: 'peak',
    source: { kind: 'fit', basis: 'earshot 0.4.0 fit/click/floss_lid', url: 'https://freesound.org/people/qubodup/sounds/167829/' },
    takes: [[132, 4, 7.5, 7.5, -18, -2.8], [132, 4.2, 7.5, 7.5, -18.4, -1.5], [132, 4.5, 7.5, 7.5, -18, -1.6], [132, 4.5, 7.5, 7.5, -18.62, -1.5]],
  },
  ding: {
    tags: ['ding', 'bell'], align: 'peak', hit: { over_db: 4.5, dip_db: 3 },
    source: { kind: 'fit', basis: 'earshot 0.4.0 fit/bell/small_bell', url: SMALL_BELL },
    takes: [[900, 1.2, 32.5, 32.5, -18, -17.8], [900, 1, 32.5, 32.5, -18, -17.1], [900, 1.2, 32.5, 32.5, -18, -17.7]],
  },
  success: {
    tags: ['success', 'chime'], align: 'onset', hit: { over_db: 4.5, dip_db: 4 },
    source: { kind: 'fit', basis: 'earshot 0.4.0 fit/bell/small_bell at G7, B7, D8', url: SMALL_BELL },
    takes: [[1120, 1.2, 138.5, 1.2, -18, -17.2], [1120, 1.2, 28.5, 1.2, -18, -16.3]],
  },
  error: {
    tags: ['error', 'buzz'], align: 'onset', hit: { over_db: 4.5, dip_db: 4 },
    source: { kind: 'forge', basis: 'earshot 0.4.0 pro_zip/down', url: '' },
    takes: [[500, 0.8, 41.5, 0.8, -18, -20.5], [500, 0.2, 9.5, 0.2, -18, -19.3]],
  },
  typing: {
    tags: ['typing', 'keyboard'], align: 'peak', pattern: { spacing_ms: [90, 140], default_duration_ms: 1200 },
    source: { kind: 'forge', basis: 'earshot 0.4.0 typing_key/thock', url: '' },
    takes: [[336.5, 8.8, 13.5, 13.5, -18, -3.2], [311.5, 8.5, 12.5, 12.5, -18.33, -1.1], [308, 8, 12.5, 12.5, -18, -4.2], [298.5, 7.8, 12.5, 12.5, -18, -2.6], [276.5, 8, 11.5, 11.5, -19.03, -1], [307.6, 8, 12.5, 12.5, -19.21, -1.2]],
  },
};

function sfxEntry(name, s) {
  const id = `sfx.${name}`;
  const takes = s.takes.map(([dur_ms, onset_ms, peak_ms, impact_ms, l50_lufs, true_peak_dbtp], i) => ({ file: `sfx/${name}_${i + 1}.wav`, dur_ms, onset_ms, peak_ms, impact_ms, l50_lufs, true_peak_dbtp }));
  const extra = { thumb: '', file: takes[0].file, impact_ms: takes.reduce((sum, t) => sum + t.impact_ms, 0) / takes.length, align: s.align };
  if (s.hit) extra.hit = s.hit;
  if (s.pattern) extra.pattern = s.pattern;
  extra.source = s.source;
  extra.takes = takes;
  return entry(id, 'sfx', id, s.tags, { do: 'sfx', sound: id }, extra);
}

/** All manifest entries in a stable order. */
```

Replace:
```js
  for (const [n, t] of Object.entries(TRANSITIONS)) out.push(entry(`trans.${n}`, 'transition', n, t.tags, { transition: n }, { duration: t.duration }));
```
with:
```js
  for (const [n, t] of Object.entries(TRANSITIONS)) out.push(entry(`trans.${n}`, 'transition', n, t.tags, { transition: n }, { duration: t.duration }));
  for (const [n, s] of Object.entries(SFX)) out.push(sfxEntry(n, s));
```

Replace:
```js
    const id = e.id;
```
with:
```js
    if (e.kind === 'sfx') continue; // an sfx entry has no image
    const id = e.id;
```

Replace:
```js
/** Every output file below ui/img/storymaker/ as a Map(relative path -> text). */
```
with:
```js
/** Every output file below ui/img/storymaker/ as a Map(relative path -> text, or a Buffer for a WAV). */
```

Replace:
```js
  for (const [p, svg] of buildThumbs(rig, entries)) files.set(p, svg);
```
with:
```js
  for (const [p, svg] of buildThumbs(rig, entries)) files.set(p, svg);
  for (const e of entries) if (e.kind === 'sfx') for (const t of e.takes) files.set(t.file, readSfxFile(new URL(`../${t.file}`, import.meta.url)));
```

In `assets/storymaker/lib/names.mjs`, replace:
```js
  // palettes
```
with:
```js
  // sound effects
  'sfx.whoosh': ['Svist', 'Sus', 'Wusch', 'Σφύριγμα', 'Whoosh', 'Silbido', 'Souffle', 'सरसराहट', 'Fruscio', 'シュッという音', 'Zoef', 'Sus', 'Świst', 'Zunido', 'Sus', '嗖声'],
  'sfx.pop': ['Lupnutí', 'Plop', 'Plopp', 'Σκάσιμο', 'Pop', 'Chasquido', 'Éclatement', 'पॉप ध्वनि', 'Schiocco', 'ポンという音', 'Plop', 'Plopp', 'Pyknięcie', 'Estalo', 'Plopp', '啵声'],
  'sfx.click': ['Kliknutí', 'Klik', 'Klick', 'Κλικ', 'Click', 'Clic', 'Clic', 'क्लिक', 'Clic', 'クリック音', 'Klik', 'Klikk', 'Kliknięcie', 'Clique', 'Klick', '点击声'],
  'sfx.ding': ['Cinknutí', 'Kling', 'Ding', 'Κουδούνισμα', 'Ding', 'Campanilla', 'Tintement', 'घंटी की आवाज़', 'Rintocco', 'チーンという音', 'Ping', 'Pling', 'Dzwonek', 'Toque de sino', 'Pling', '叮声'],
  'sfx.success': ['Úspěch', 'Succes', 'Erfolg', 'Επιτυχία', 'Success', 'Éxito', 'Réussite', 'सफलता', 'Successo', '成功音', 'Succes', 'Suksess', 'Sukces', 'Sucesso', 'Framgång', '成功音'],
  'sfx.error': ['Chyba', 'Fejl', 'Fehler', 'Σφάλμα', 'Error', 'Error', 'Erreur', 'त्रुटि', 'Errore', 'エラー音', 'Fout', 'Feil', 'Błąd', 'Erro', 'Fel', '错误音'],
  'sfx.typing': ['Psaní na klávesnici', 'Tastetryk', 'Tippen', 'Πληκτρολόγηση', 'Typing', 'Tecleo', 'Frappe au clavier', 'टाइपिंग', 'Digitazione', 'タイピング音', 'Typen', 'Tastetrykk', 'Pisanie na klawiaturze', 'Digitação', 'Tangentnedslag', '打字声'],
  // palettes
```

In `assets/storymaker/lib/validate.mjs`, replace:
```js
  transition: ['trans.cut', 'trans.fade', 'trans.slide-left', 'trans.wipe'],
```
with:
```js
  transition: ['trans.cut', 'trans.fade', 'trans.slide-left', 'trans.wipe'],
  sfx: ['whoosh', 'pop', 'click', 'ding', 'success', 'error', 'typing'].map((n) => `sfx.${n}`),
```

Replace:
```js
transition: 'trans.' };
```
with:
```js
transition: 'trans.', sfx: 'sfx.' };
```

Replace:
```js
    if (!files.has(e.thumb)) add(`${e.id}: missing thumbnail ${e.thumb}`);
```
with:
```js
    if (e.kind !== 'sfx' && !files.has(e.thumb)) add(`${e.id}: missing thumbnail ${e.thumb}`);
```

Replace:
```js
  // props: palette tokens only
```
with:
```js
  // sfx: at least two takes, each a copied WAV; file is the first take
  for (const e of entries.filter((x) => x.kind === 'sfx')) {
    if (e.thumb !== '') add(`${e.id}: an sfx entry has no thumbnail`);
    if (!Array.isArray(e.takes) || e.takes.length < 2) {
      add(`${e.id}: needs at least two takes`);
    } else {
      if (e.file !== e.takes[0].file) add(`${e.id}: file must equal takes[0].file`);
      for (const t of e.takes) if (!Buffer.isBuffer(files.get(t.file))) add(`${e.id}: missing take ${t.file}`);
    }
    if (e.align !== 'peak' && e.align !== 'onset') add(`${e.id}: align must be peak or onset`);
    if (!e.source || !e.source.kind || !e.source.basis) add(`${e.id}: source needs kind and basis`);
  }

  // props: palette tokens only
```

In `internal/server/storymaker_parity_browser_test.go`, replace:
```
catalog.entries.filter(e => e.file).map(e => e.file)
```
with:
```
catalog.entries.filter(e => e.file && e.kind !== 'sfx').map(e => e.file)
```

The parent there reads every listed file through `getJSON`, so a WAV would throw and stop the parity test. A26 runs that test.

In `assets/storymaker/test/glyphs.test.mjs`, replace:
```js
    copy('assets/storymaker/lib');
```
with:
```js
    copy('assets/storymaker/lib');
    copy('assets/storymaker/sfx');
```

`buildOutputs` now reads the source WAVs, so the private tree of the test `--check fails when glyphs.json differs` needs them too.

The generated files come from the build in the next step and are never pasted.

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
node scripts/build-storymaker-catalog.mjs
node --test assets/storymaker/test/catalog.test.mjs
node --test assets/storymaker/test/glyphs.test.mjs
npm run check:storymaker-catalog
npm run test:storymaker
go test ./ui -run '(?i)storymakercatalog' -count=1
go test ./internal/storymaker -run '(?i)(catalog|SoundOnTheRealCatalog)' -count=1
go test ./internal/storymaker/render -run '(?i)(SnapshotHoldsNoWAV|RunRendersAndPublishesTheVideo)' -count=1
gofmt -l ui/storymaker_catalog_test.go internal/storymaker internal/server/storymaker_parity_browser_test.go
```

Expected:
- The build prints `134 files`.
- All tests pass, and the `--check` line reports the catalog as up to date, which also shows the byte-flip test restored the file.
- `gofmt -l` prints nothing.

- [ ] **Step 6: Commit**

Run `detect_changes` first. A PARTIAL answer on new files is accepted with a grep confirmation in the commit body (outline section 5).

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add assets/storymaker/lib/catalog.mjs assets/storymaker/lib/names.mjs assets/storymaker/lib/validate.mjs assets/storymaker/test/catalog.test.mjs assets/storymaker/test/glyphs.test.mjs ui/img/storymaker/catalog.json 'ui/img/storymaker/sfx/*.wav' 'ui/lang/storymaker_catalog/*.json' ui/storymaker_catalog_test.go internal/storymaker/catalog_real_test.go internal/storymaker/validate_test.go internal/storymaker/render/render_test.go internal/server/storymaker_parity_browser_test.go
git commit -m "feat(storymaker): add the sfx catalog entries and the WAV copy" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

<!-- G-A-2 -->

### Task A5: Music fields, keys, music store, `auto_sfx`, `sfx` beat accepted
**Batch:** 2C  **Depends on:** none
**Files:**
- Create: `internal/storymaker/music.go`
- Modify: `internal/storymaker/validate.go`
- Modify: `internal/storymaker/model.go`
- Modify: `internal/storymaker/decode.go`
- Test: `internal/storymaker/music_test.go` (new)
- Test: `internal/storymaker/validate_test.go` (lines 177, 359, 367)

- [ ] **Step 1: Impact**

Run this from the main checkout for each of `music` (method of `validator`), `Style`, `phase1Beats`, `fieldHints`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "music" --direction upstream --repo .
```

Report callers, processes and risk. For HIGH, CRITICAL and UNKNOWN, follow the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

Create `internal/storymaker/music_test.go`:

```go
package storymaker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func validateMusic(t *testing.T, m *Music) []Issue {
	t.Helper()
	s := storyOf(t, `{"id":"s1"}`)
	s.Music = m
	return Validate(&s, loadMini(t), DefaultLimits())
}

func TestValidateMusicFields(t *testing.T) {
	cases := []struct {
		name       string
		m          *Music
		code, path string
	}{
		{"file and track", &Music{Mode: "file", File: "Documents/Music/a.mp3", Track: "5"}, CodeInvalidValue, "/music/track"},
		{"track is text", &Music{Mode: "file", Track: "abc"}, CodeInvalidValue, "/music/track"},
		{"track is zero", &Music{Mode: "file", Track: "0"}, CodeInvalidValue, "/music/track"},
		{"track has a sign", &Music{Mode: "file", Track: "+5"}, CodeInvalidValue, "/music/track"},
		{"generated without mood or prompt", &Music{Mode: "generated"}, CodeMissingField, "/music/mood"},
		{"mood outside the presets", &Music{Mode: "generated", Mood: "sad-trombone"}, CodeInvalidValue, "/music/mood"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectIssue(t, validateMusic(t, tc.m), tc.code, SeverityError, tc.path)
		})
	}
	for _, good := range []*Music{
		{Mode: "none"},
		{Mode: "file", Track: "42"},
		{Mode: "file", File: "Documents/Music/a.mp3"},
		{Mode: "generated", Mood: "lo-fi"},
		{Mode: "generated", Prompt: "soft piano"},
	} {
		if issues := validateMusic(t, good); len(issues) != 0 {
			t.Errorf("%+v:%s", good, dump(issues))
		}
	}
}

func TestMusicKeysExcludeLength(t *testing.T) {
	hash := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}
	// 1700000000 s plus 123 ns is the decimal UnixNano 1700000000000000123. The hash input has no length field.
	at := time.Unix(1700000000, 123)
	if got := MusicKeyGenerated("lo-fi", "soft"); got != hash("generated|lo-fi|soft") {
		t.Errorf("generated key %s", got)
	}
	if got := MusicKeyFile("Documents/a.mp3", 7, at); got != hash("file|Documents/a.mp3|7|1700000000000000123") {
		t.Errorf("file key %s", got)
	}
	if got := MusicKeyTrack("42", 7, at); got != hash("track|42|7|1700000000000000123") {
		t.Errorf("track key %s", got)
	}
}

func TestMusicPromptAndVolume(t *testing.T) {
	template := "Lo-fi background music, mellow electric piano, soft vinyl texture, relaxed slow beat"
	if got := MusicPrompt("lo-fi", ""); got != template {
		t.Errorf("mood only: %q", got)
	}
	if got := MusicPrompt("lo-fi", "with rain"); got != template+". with rain" {
		t.Errorf("mood and prompt: %q", got)
	}
	if got := MusicPrompt("", " soft piano "); got != " soft piano " {
		t.Errorf("a prompt without a mood stays as written: %q", got)
	}
	for _, mood := range MusicMoods {
		if MusicPrompt(mood, "") == "" {
			t.Errorf("mood %s has no template", mood)
		}
	}
	half := 0.5
	if MusicVolume(nil) != 0.25 || MusicVolume(&Music{}) != 0.25 || MusicVolume(&Music{Volume: &half}) != 0.5 {
		t.Error("MusicVolume")
	}
}

func TestMusicStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	mem := NewMemStore()
	ps := NewProjectStore(mem, "Documents/Storymaker")
	proj, _, _, err := ps.Create(ctx, "T", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	key := MusicKeyGenerated("lo-fi", "")

	if meta, ext, err := ps.MusicInfo(ctx, proj.ID, key); err != nil || meta.V != 0 || ext != "" {
		t.Fatalf("empty: %+v %q %v", meta, ext, err)
	}
	if err := ps.WriteMusic(ctx, proj.ID, key, "mp3", []byte("ID3")); err != nil {
		t.Fatal(err)
	}
	if err := ps.WriteMusic(ctx, proj.ID, key, "mp3", []byte("ID3")); err != nil {
		t.Fatalf("a hash name that exists is not an error: %v", err)
	}
	if meta, ext, err := ps.MusicInfo(ctx, proj.ID, key); err != nil || meta.V != 0 || ext != "mp3" {
		t.Fatalf("audio without meta: %+v %q %v", meta, ext, err)
	}
	want := MusicMeta{V: 1, Ext: "mp3", DurationMS: 61000}
	if err := ps.WriteMusicMeta(ctx, proj.ID, key, want); err != nil {
		t.Fatal(err)
	}
	if meta, ext, err := ps.MusicInfo(ctx, proj.ID, key); err != nil || meta != want || ext != "mp3" {
		t.Fatalf("round trip: %+v %q %v", meta, ext, err)
	}

	metaPath := ps.musicFile(proj.ID, key+".meta.json")
	for _, bad := range []string{
		`{`,
		`{"v":2,"ext":"mp3","duration_ms":1000}`,
		`{"v":1,"ext":"ogg","duration_ms":1000}`,
		`{"v":1,"ext":"mp3","duration_ms":0}`,
		`{"v":1,"ext":"mp3","duration_ms":3600001}`,
		`{"v":1,"ext":"wav","duration_ms":1000}`,
	} {
		if _, err := mem.WriteFile(ctx, metaPath, []byte(bad), ""); err != nil {
			t.Fatal(err)
		}
		if meta, ext, err := ps.MusicInfo(ctx, proj.ID, key); err != nil || meta.V != 0 || ext != "mp3" {
			t.Errorf("meta %s: %+v %q %v", bad, meta, ext, err)
		}
	}

	if err := ps.WriteMusic(ctx, proj.ID, "../x", "mp3", nil); err == nil {
		t.Error("a bad key was written")
	}
	if err := ps.WriteMusic(ctx, proj.ID, key, "ogg", nil); err == nil {
		t.Error("a bad extension was written")
	}
	if err := ps.WriteMusicMeta(ctx, proj.ID, key, MusicMeta{V: 1, Ext: "mp3"}); err == nil {
		t.Error("a meta without a duration was written")
	}
}

func TestStyleAutoSFXIsAccepted(t *testing.T) {
	doc := func(style string) string {
		return `{"schema":"storymaker/1","title":"T","format":{"aspect":"16:9","fps":30},` +
			`"style":{"palette":"sunny","font":"default","captions":true` + style + `},"cast":[],"scenes":[{"id":"s1"}]}`
	}
	s := mustDecode(t, doc(`,"auto_sfx":true`))
	if !s.Style.AutoSFX {
		t.Fatal("auto_sfx was not read")
	}
	if issues := Validate(&s, loadMini(t), DefaultLimits()); len(issues) != 0 {
		t.Fatalf("issues:%s", dump(issues))
	}
	data, err := CanonicalJSON(s)
	if err != nil || !strings.Contains(string(data), `"auto_sfx": true`) {
		t.Fatalf("canonical with auto_sfx: %s %v", data, err)
	}
	s.Style.AutoSFX = false
	data, err = CanonicalJSON(s)
	if err != nil || strings.Contains(string(data), "auto_sfx") {
		t.Fatalf("Go never writes the default: %s %v", data, err)
	}
	_, issues, err := DecodeStory([]byte(doc(`,"sfx":true`)), DefaultLimits())
	if err == nil || len(issues) != 1 || issues[0].Code != CodeUnknownField || !strings.Contains(issues[0].Fix, "auto_sfx") {
		t.Fatalf("hint: %+v %v", issues, err)
	}
}

func TestValidateAcceptsSFXBeat(t *testing.T) {
	if issues := validateScenes(t, `{"id":"s1","beats":[{"do":"sfx","sound":"sfx.pop"}]}`); len(issues) != 0 {
		t.Fatalf("an sfx beat is played now:%s", dump(issues))
	}
}
```

In `internal/storymaker/validate_test.go`, delete this table row:

```go
		{"sfx beat", `{"id":"s1","beats":[{"do":"sfx","sound":"sfx.pop"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0"},
```

In the same file, replace:

```go
	// Valid voice ids and a music setting that is only a note.
```

with:

```go
	// Valid voice ids and a generated music setting without any note.
```

Then replace:

```go
	expectIssue(t, issues, CodeNotSupportedYet, SeverityMinor, "/music")
```

with:

```go
	if is := findIssue(issues, CodeNotSupportedYet, SeverityMinor, "/music"); is != nil {
		t.Fatalf("music is still a note: %+v", is)
	}
```

- [ ] **Step 3: Run it**

```powershell
go test ./internal/storymaker -run '(?i)(music|autosfx|sfxbeat)' -count=1
```

Expected: the test build fails with `undefined: MusicKeyGenerated`, `undefined: MusicPrompt` and `undefined: MusicMeta`, because `internal/storymaker/music.go` does not exist yet.

- [ ] **Step 4: Implement**

Create `internal/storymaker/music.go`:

```go
package storymaker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"
)

// MusicMoods are the presets of music.mood. MusicPrompt has one template for each.
var MusicMoods = []string{"upbeat-corporate", "calm-ambient", "inspiring", "playful", "tech-pulse", "dramatic", "lo-fi", "acoustic"}

// musicTemplates are first guesses. They feed a paid call and the music key, so changing one makes
// every generated track of that mood missing.
var musicTemplates = map[string]string{
	"upbeat-corporate": "Upbeat corporate background music, bright piano and clean guitar, steady light drums, positive and professional",
	"calm-ambient":     "Calm ambient background music, soft pads and slow textures, no drums, quiet and spacious",
	"inspiring":        "Inspiring cinematic background music, warm strings and piano, gentle build, hopeful",
	"playful":          "Playful light background music, pizzicato strings, marimba and soft percussion, cheerful and bouncy",
	"tech-pulse":       "Modern technology background music, pulsing synth bass, clean electronic drums, focused and forward moving",
	"dramatic":         "Dramatic cinematic background music, low strings and deep percussion, tense and serious",
	"lo-fi":            "Lo-fi background music, mellow electric piano, soft vinyl texture, relaxed slow beat",
	"acoustic":         "Warm acoustic background music, fingerpicked guitar and light percussion, friendly and natural",
}

// MusicPrompt builds the generation prompt: the mood's template, then ". " and the user's prompt
// when both exist. A prompt without a mood stays as written.
func MusicPrompt(mood, prompt string) string {
	template, ok := musicTemplates[mood]
	switch {
	case !ok:
		return prompt
	case prompt == "":
		return template
	}
	return template + ". " + prompt
}

func musicKey(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

// MusicKeyGenerated is the key of generated music. The film length is not part of any key.
func MusicKeyGenerated(mood, prompt string) string {
	return musicKey("generated", mood, prompt)
}

// MusicKeyFile is the key of a desktop file source.
func MusicKeyFile(path string, size int64, mtime time.Time) string {
	return musicKey("file", path, strconv.FormatInt(size, 10), strconv.FormatInt(mtime.UnixNano(), 10))
}

// MusicKeyTrack is the key of a media registry track.
func MusicKeyTrack(id string, size int64, mtime time.Time) string {
	return musicKey("track", id, strconv.FormatInt(size, 10), strconv.FormatInt(mtime.UnixNano(), 10))
}

// MusicVolume is the bed volume of a story, 0.25 when absent.
func MusicVolume(m *Music) float64 {
	if m == nil || m.Volume == nil {
		return 0.25
	}
	return *m.Volume
}

// MusicMeta is audio/music/<key>.meta.json. It holds the duration, so /narration never decodes a track.
type MusicMeta struct {
	V          int    `json:"v"`           // 1
	Ext        string `json:"ext"`         // wav or mp3
	DurationMS int    `json:"duration_ms"` // 1..3600000
}

func validMusicMeta(m MusicMeta) bool {
	return m.V == 1 && validMusicExt(m.Ext) && m.DurationMS >= 1 && m.DurationMS <= 3600000
}

func validMusicExt(ext string) bool { return ext == "wav" || ext == "mp3" }

func validMusicKey(key string) bool {
	if len(key) != 64 {
		return false
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// validMusicTrack reports a positive decimal media id. ParseInt alone would accept a sign.
func validMusicTrack(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return err == nil && n > 0
}

func (p *ProjectStore) musicFile(id, name string) string { return p.file(id, "audio", "music", name) }

func (p *ProjectStore) checkMusic(id, key string) error {
	if err := p.checkID(id); err != nil {
		return err
	}
	if !validMusicKey(key) {
		return fmt.Errorf("%w: music key %q", ErrInvalidArgument, clip(key, 80))
	}
	return nil
}

// WriteMusic writes audio/music/<key>.<ext>. The name is a content key, so an existing file is not
// an error. Store.WriteFile takes bytes: this serves SeedMusic and small tracks.
func (p *ProjectStore) WriteMusic(ctx context.Context, id, key, ext string, data []byte) error {
	if err := p.checkMusic(id, key); err != nil {
		return err
	}
	if !validMusicExt(ext) {
		return fmt.Errorf("%w: music extension %q", ErrInvalidArgument, clip(ext, 20))
	}
	if _, err := p.st.WriteFile(ctx, p.musicFile(id, key+"."+ext), data, MustNotExist); err != nil && !errors.Is(err, ErrConflict) {
		return fmt.Errorf("storymaker: write music: %w", err)
	}
	return nil
}

// WriteMusicMeta writes audio/music/<key>.meta.json and replaces an older one.
func (p *ProjectStore) WriteMusicMeta(ctx context.Context, id, key string, m MusicMeta) error {
	if err := p.checkMusic(id, key); err != nil {
		return err
	}
	if !validMusicMeta(m) {
		return fmt.Errorf("%w: music meta %+v", ErrInvalidArgument, m)
	}
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("storymaker: encode music meta: %w", err)
	}
	if _, err := p.st.WriteFile(ctx, p.musicFile(id, key+".meta.json"), append(data, '\n'), ""); err != nil {
		return fmt.Errorf("storymaker: write music meta: %w", err)
	}
	return nil
}

// MusicInfo reads the meta file and finds the audio file. meta.V is 0 when the meta is absent or
// invalid, or names another extension than the file on disk; audioExt is "" without audio.
func (p *ProjectStore) MusicInfo(ctx context.Context, id, key string) (meta MusicMeta, audioExt string, err error) {
	if err := p.checkMusic(id, key); err != nil {
		return MusicMeta{}, "", err
	}
	data, _, err := p.st.ReadFile(ctx, p.musicFile(id, key+".meta.json"))
	switch {
	case err == nil:
		if json.Unmarshal(data, &meta) != nil || !validMusicMeta(meta) {
			meta = MusicMeta{}
		}
	case !errors.Is(err, fs.ErrNotExist):
		return MusicMeta{}, "", fmt.Errorf("storymaker: read music meta: %w", err)
	}
	names, err := p.st.List(ctx, p.file(id, "audio", "music"))
	if err != nil {
		return MusicMeta{}, "", fmt.Errorf("storymaker: list music: %w", err)
	}
	has := func(ext string) bool { return containsName(names, key+"."+ext) }
	switch {
	case meta.V != 0 && has(meta.Ext):
		audioExt = meta.Ext
	case has("wav"):
		audioExt = "wav"
	case has("mp3"):
		audioExt = "mp3"
	}
	if audioExt != "" && meta.V != 0 && meta.Ext != audioExt {
		meta = MusicMeta{}
	}
	return meta, audioExt, nil
}
```

In `internal/storymaker/validate.go`, replace this text. No 2A or 2B task touches `music()`:

```go
	if m.Mode == "file" {
		switch {
		case m.File == "" && m.Track == "":
			v.errorf(CodeMissingField, "/music/file", `The music mode "file" needs a file or a track.`, `Add "file": "Documents/Music/theme.mp3".`)
		case m.File != "":
			if why := badDesktopPath(m.File); why != "" {
				v.errorf(CodeInvalidValue, "/music/file", fmt.Sprintf("The music file %q is not a desktop path: %s.", clip(m.File, 60), why),
					`Use a rooted relative path inside the desktop, for example "Documents/Music/theme.mp3".`)
			}
		}
	}
	if m.Mode != "none" {
		v.notYet("/music", "Background music is accepted but not played yet.")
	}
}
```

with:

```go
	if m.Mode == "file" {
		switch {
		case m.File == "" && m.Track == "":
			v.errorf(CodeMissingField, "/music/file", `The music mode "file" needs a file or a track.`, `Add "file": "Documents/Music/theme.mp3".`)
		case m.File != "" && m.Track != "":
			v.errorf(CodeInvalidValue, "/music/track", `The music mode "file" takes a file or a track, not both.`, `Remove "track" or "file".`)
		case m.File != "":
			if why := badDesktopPath(m.File); why != "" {
				v.errorf(CodeInvalidValue, "/music/file", fmt.Sprintf("The music file %q is not a desktop path: %s.", clip(m.File, 60), why),
					`Use a rooted relative path inside the desktop, for example "Documents/Music/theme.mp3".`)
			}
		}
	}
	if m.Track != "" && !validMusicTrack(m.Track) {
		v.errorf(CodeInvalidValue, "/music/track", fmt.Sprintf("The music track %q is not a media id.", clip(m.Track, 40)),
			`Use the decimal id of a generated track, for example "track": "42".`)
	}
	if m.Mood != "" && !containsName(MusicMoods, m.Mood) {
		alts := closestNames(m.Mood, MusicMoods, 5)
		v.errorf(CodeInvalidValue, "/music/mood", fmt.Sprintf("The music mood %q is not known.", clip(m.Mood, 40)),
			"Use one of: "+listOf(alts)+". Example: "+exampleJSON("mood", alts[0]))
	}
	if m.Mode == "generated" && m.Mood == "" && m.Prompt == "" {
		v.errorf(CodeMissingField, "/music/mood", `The music mode "generated" needs a mood or a prompt.`, `Add "mood": "upbeat-corporate".`)
	}
}
```

In `internal/storymaker/model.go`, add the field. M1 leaves `Rhythm` as the last field of `Style`. Replace:

```go
}

// UnmarshalJSON defaults captions to true when a style object omits the key.
```

with:

```go
	AutoSFX  bool   `json:"auto_sfx,omitempty"`
}

// UnmarshalJSON defaults captions to true when a style object omits the key.
```

In the same file, replace this prefix of the `phase1Beats` line, which M12 and G5 extended behind it:

```go
	phase1Beats       = nameSet(
```

with:

```go
	phase1Beats       = nameSet("sfx", 
```

In `internal/storymaker/decode.go`, replace:

```go
`Use "captions": true or false.`,
```

with:

```go
`Use "captions": true or false.`,
	"Style.sfx":           `Use "auto_sfx": true or false.`,
	"Style.sound_effects": `Use "auto_sfx": true or false.`,
	"Style.autosfx":       `Use "auto_sfx": true or false.`,
```

- [ ] **Step 5: Run**

```powershell
gofmt -w internal/storymaker/music.go internal/storymaker/music_test.go internal/storymaker/decode.go internal/storymaker/model.go
go test ./internal/storymaker -count=1
```

Expected: `ok  aurago/internal/storymaker`.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5. A PARTIAL answer on the new file needs a grep note in the commit body.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/music.go internal/storymaker/music_test.go internal/storymaker/validate.go internal/storymaker/validate_test.go internal/storymaker/model.go internal/storymaker/decode.go
git commit -m "feat(storymaker): music fields, keys and store, auto_sfx, sfx beat accepted" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A6: Sfx beat: impact and takes
**Batch:** 2C  **Depends on:** A3, A4
**Files:**
- Create: `scripts/storymaker-tests/21-sfx.mjs`
- Modify: `ui/js/storymaker/beats.js`
- Modify: `ui/js/storymaker/scene.js`
- Modify: `ui/js/storymaker/motion.js`
- Modify: `ui/js/storymaker/compile.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated by `npm run build:ui`)
- Modify: `internal/storymaker/testdata/catalog-mini/catalog.json`
- Test: `scripts/storymaker-tests/04-compile.mjs` (line 112; line 105 keeps its beats, so the indexes stay)
- Test: `scripts/storymaker-tests/00-fixtures.mjs` (lines 29, 31)
- Test: `internal/storymaker/catalog_test.go` (lines 231-233, 236-244)
- Test: `ui/storymaker_player_gate_browser_test.go` (line 75)

- [ ] **Step 1: Impact**

Run from the main checkout for `buildStoryboard` and `compile`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "buildStoryboard" --direction upstream --repo .
```

Report callers, processes and risk. For HIGH, CRITICAL and UNKNOWN, follow the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

Add the fixture entries first, because the tests read them. In `internal/storymaker/testdata/catalog-mini/catalog.json`, replace:

```json
 "entries":[
```

with:

```json
 "entries":[
  {"id":"sfx.whoosh","kind":"sfx","name":"sfx.whoosh","name_key":"storymaker.catalog.sfx_whoosh","tags":["whoosh","swish"],"thumb":"","usage_example":{"do":"sfx","sound":"sfx.whoosh"},"file":"sfx/whoosh_1.wav","impact_ms":128,"align":"peak","source":{"kind":"forge","basis":"test fixture","url":""},"takes":[{"file":"sfx/whoosh_1.wav","dur_ms":477,"onset_ms":0,"peak_ms":130,"impact_ms":130,"l50_lufs":-18,"true_peak_dbtp":-1.5},{"file":"sfx/whoosh_2.wav","dur_ms":477,"onset_ms":0,"peak_ms":126,"impact_ms":126,"l50_lufs":-18,"true_peak_dbtp":-1.5}]},
  {"id":"sfx.pop","kind":"sfx","name":"sfx.pop","name_key":"storymaker.catalog.sfx_pop","tags":["pop"],"thumb":"","usage_example":{"do":"sfx","sound":"sfx.pop"},"file":"sfx/pop_1.wav","impact_ms":3.75,"align":"peak","hit":{"over_db":4.5,"dip_db":3},"source":{"kind":"forge","basis":"test fixture","url":""},"takes":[{"file":"sfx/pop_1.wav","dur_ms":200,"onset_ms":0,"peak_ms":3.5,"impact_ms":3.5,"l50_lufs":-18,"true_peak_dbtp":-1.5},{"file":"sfx/pop_2.wav","dur_ms":200,"onset_ms":0,"peak_ms":4,"impact_ms":4,"l50_lufs":-18,"true_peak_dbtp":-1.5}]},
  {"id":"sfx.typing","kind":"sfx","name":"sfx.typing","name_key":"storymaker.catalog.sfx_typing","tags":["keyboard","keys"],"thumb":"","usage_example":{"do":"sfx","sound":"sfx.typing"},"file":"sfx/typing_1.wav","impact_ms":12,"align":"peak","pattern":{"spacing_ms":[90,140],"default_duration_ms":1200},"source":{"kind":"forge","basis":"test fixture","url":""},"takes":[{"file":"sfx/typing_1.wav","dur_ms":300,"onset_ms":0,"peak_ms":12,"impact_ms":12,"l50_lufs":-18,"true_peak_dbtp":-1.5},{"file":"sfx/typing_2.wav","dur_ms":300,"onset_ms":0,"peak_ms":11.5,"impact_ms":11.5,"l50_lufs":-18,"true_peak_dbtp":-1.5},{"file":"sfx/typing_3.wav","dur_ms":300,"onset_ms":0,"peak_ms":13,"impact_ms":13,"l50_lufs":-18,"true_peak_dbtp":-1.5}]},
```

Create `scripts/storymaker-tests/21-sfx.mjs`:

```js
// Part A: the sfx beat. Impact placement, take rotation, typing keys and auto-SFX against the mini catalog.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog } = t;
const SM = t.core();
const takesOf = id => catalog.entries.find(e => e.id === id).takes;
const sfxEvents = report => plain(report.audio_events).filter(e => e.kind === 'sfx');

check('sfx: a cued whoosh starts at the marker minus the impact of its take', () => {
    const story = { scenes: [{ id: 'a', transition: 'cut', narration: 'eins zwei drei {1}vier sechs sieben', beats: [{ do: 'sfx', sound: 'sfx.whoosh', cue: 1 }] }] };
    const marker = plain(SM.planTiming(story, {}, catalog)).scenes.a.cues['1'];
    const out = SM.compile(story, {}, catalog);
    const [ev] = sfxEvents(out.report);
    assert.equal(ev.ref, 'sfx.whoosh');
    assert.equal(ev.scene_id, 'a');
    // Frozen 4.6 step 6a: start = marker + delay - impact. Scene a starts at frame 0, so start_ms is the local time.
    near(ev.start_ms, (marker - takesOf('sfx.whoosh')[ev.take].impact_ms / 1000) * 1000, 1);
    assert.ok(!plain(out.report.issues).some(i => i.code === 'not_supported_yet' || i.code === 'cue_late'));
});

check('sfx: with:"prev" puts the impact on the landing of the previous beat', () => {
    const story = { scenes: [{ id: 'a', transition: 'cut', beats: [
        { do: 'show', item: 'prop.lock', at: 'right', entry: 'pop' },
        { do: 'sfx', sound: 'sfx.pop', with: 'prev' }
    ] }] };
    const [ev] = sfxEvents(SM.compile(story, {}, catalog).report);
    // The show starts at 0 and lasts the default 0.5 s. A pop on a prop lands at the spring's lead fraction (frozen 4.5).
    const landing = SM.motion.leadFraction('spring') * 0.5;
    near(ev.start_ms, (landing - takesOf('sfx.pop')[ev.take].impact_ms / 1000) * 1000, 1);
});

check('sfx: a repeat never takes the same take, across scenes too', () => {
    const whoosh = { do: 'sfx', sound: 'sfx.whoosh' };
    const sceneA = { id: 'a', transition: 'cut', beats: [whoosh, whoosh, whoosh] };
    const inA = sfxEvents(SM.compile({ scenes: [sceneA] }, {}, catalog).report).map(e => e.take);
    assert.equal(inA.length, 3);
    for (let i = 1; i < inA.length; i++) assert.notEqual(inA[i], inA[i - 1], 'inside a scene');
    // Pick a second scene whose unrotated first take equals the last take of scene a. Only a film-wide lastTake can change it.
    const count = takesOf('sfx.whoosh').length;
    let id = null;
    for (let n = 0; n < 64 && id === null; n++) if (SM.seed.takeIndex('b' + n, 0, 'sfx.whoosh', 0, count) === inA[2]) id = 'b' + n;
    assert.ok(id, 'a scene id with that first take exists');
    const both = sfxEvents(SM.compile({ scenes: [sceneA, { id, transition: 'cut', beats: [whoosh] }] }, {}, catalog).report).map(e => e.take);
    assert.notEqual(both[3], both[2], 'across scenes');
});

check('sfx: a clamp at scene time 0 does not warn', () => {
    const story = { scenes: [{ id: 'a', transition: 'cut', narration: '{1}eins zwei drei', beats: [{ do: 'sfx', sound: 'sfx.whoosh', cue: 1 }] }] };
    const out = SM.compile(story, {}, catalog);
    assert.equal(sfxEvents(out.report)[0].start_ms, 0);
    assert.ok(!plain(out.report.issues).some(i => i.code === 'cue_late'));
});

check('sfx: a cued sfx in scene 2 has start_ms from the scene start plus its local time', () => {
    const story = { scenes: [
        { id: 'a', transition: 'cut', duration: 2 },
        { id: 'b', transition: 'cut', narration: 'eins zwei {1}drei vier', beats: [{ do: 'sfx', sound: 'sfx.pop', cue: 1 }] }
    ] };
    const marker = plain(SM.planTiming(story, {}, catalog)).scenes.b.cues['1'];
    const report = SM.compile(story, {}, catalog).report;
    const [ev] = sfxEvents(report);
    const base = report.scenes[1].start_frame / 30;
    assert.ok(base > 0, 'scene b starts after scene a');
    assert.equal(ev.scene_id, 'b');
    near(ev.start_ms, (base + marker - takesOf('sfx.pop')[ev.take].impact_ms / 1000) * 1000, 1);
});
```

In `scripts/storymaker-tests/04-compile.mjs`, line 112 as S7 leaves it, with highlight and say gone from the list. Replace:

```js
    for (const j of [2, 3]) assert.equal(codes('/scenes/0/beats/' + j, 'not_supported_yet').length, 1, 'beat ' + j);
```

with:

```js
    assert.equal(codes('/scenes/0/beats/2', 'not_supported_yet').length, 0, 'the sfx beat plays');
    assert.equal(codes('/scenes/0/beats/3', 'not_supported_yet').length, 1, 'the chart beat');
```

In `scripts/storymaker-tests/00-fixtures.mjs`, line 29 as S5 leaves it. Replace:

```js
assert.equal(catalog.entries.length, 23); // 20 phase 1 entries plus text.speech, text.thought, text.subtitle
```

with:

```js
assert.equal(catalog.entries.length, 26); // 23 with the speech styles, plus sfx.whoosh, sfx.pop, sfx.typing
```

In the same file, replace:

```js
    for (const e of catalog.entries) if (e.file) assert.ok(typeof readJSON(MINI + e.file) === 'object', e.file + ' exists and parses');
```

with:

```js
    for (const e of catalog.entries) if (e.file && !e.file.endsWith('.wav')) assert.ok(typeof readJSON(MINI + e.file) === 'object', e.file + ' exists and parses');
```

In `internal/storymaker/catalog_test.go`, as S5 leaves it, replace:

```go
	fsys := fstest.MapFS{}
	err := c.VerifyFiles(fsys, "img/storymaker")
	// 23 thumbs, 5 prop and environment files, the rig and the font: 30
	if err == nil || !strings.Contains(err.Error(), "img/storymaker/rig/human.json") || !strings.Contains(err.Error(), "30 missing files") {
```

with:

```go
	fsys := fstest.MapFS{}
	// Take 1 is also the entry's file. It exists from the start, so the count does not depend on how often VerifyFiles names it.
	for _, e := range c.Entries() {
		if e.Kind == "sfx" {
			fsys["img/storymaker/"+e.File] = &fstest.MapFile{Data: []byte("RIFF")}
		}
	}
	err := c.VerifyFiles(fsys, "img/storymaker")
	// Rig 1, font 1, 23 thumbs and 5 entry files give 30 (S5). The sfx entries have no thumb, and their
	// other takes add whoosh 1, pop 1 and typing 2, so 34.
	if err == nil || !strings.Contains(err.Error(), "img/storymaker/rig/human.json") || !strings.Contains(err.Error(), "34 missing files") {
```

In the same file, replace:

```go
		fsys["img/storymaker/"+e.Thumb] = &fstest.MapFile{Data: []byte("<svg/>")}
```

with:

```go
		for _, take := range e.Takes {
			fsys["img/storymaker/"+take.File] = &fstest.MapFile{Data: []byte("RIFF")}
		}
		if e.Thumb != "" {
			fsys["img/storymaker/"+e.Thumb] = &fstest.MapFile{Data: []byte("<svg/>")}
		}
```

In `ui/storymaker_player_gate_browser_test.go`, the parent page fetches every entry file as JSON, and the mini WAVs do not exist. Replace:

```js
  const paths = [catalog.rig.file].concat(catalog.entries.filter(e => e.file).map(e => e.file));
```

with:

```js
  const paths = [catalog.rig.file].concat(catalog.entries.filter(e => e.file && e.kind !== 'sfx').map(e => e.file));
```

- [ ] **Step 3: Run it**

```powershell
npm run test:storymaker
```

Expected: five `FAIL sfx:` lines, because the beat still produces no event, plus `FAIL compile: the phase-1 support matrix`, because M9's stub still reports `not_supported_yet` for beat 2. Nothing else fails.

- [ ] **Step 4: Implement**

In `ui/js/storymaker/scene.js`, add the lookup. Entries stay in `byId` (frozen 4.3). Replace:

```js
    function base(type, id, x, y) {
```

with:

```js
    // sfxEntry(cat, id) = the catalog entry of a sound that has takes, or null.
    function sfxEntry(cat, id) {
        var e = cat && cat.byId ? cat.byId[id] : null;
        return e && e.kind === 'sfx' && Array.isArray(e.takes) && e.takes.length ? e : null;
    }

    function base(type, id, x, y) {
```

In the same file, replace:

```js
        indexCatalog: indexCatalog,
```

with:

```js
        indexCatalog: indexCatalog, sfxEntry: sfxEntry,
```

In `ui/js/storymaker/motion.js`, replace the last line of the file:

```js
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

with:

```js

    // pickTake(sceneId, j, entry, key, lastTake) = {take, impact in seconds}. It rotates through seed.takeIndex and
    // remembers the take in lastTake (soundId -> take), which spans the film. A plain sfx uses key 0.
    function pickTake(sceneId, j, entry, key, lastTake) {
        var take = SM.seed.takeIndex(sceneId, j, entry.id, key, entry.takes.length, lastTake.get(entry.id));
        lastTake.set(entry.id, take);
        return { take: take, impact: (entry.takes[take].impact_ms || 0) / 1000 };
    }

    SM.motion.pickTake = pickTake;
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

In `ui/js/storymaker/beats.js`, make seven edits.

First, delete the `sfx` key M9 added to the `kinds` table. The stub and its minor `not_supported_yet` are replaced by the code below. Replace:

```js
            sfx: {
                dur: function () { return 0; },
                run: function (b, j) { warn('minor', beatPath(j), 'not_supported_yet', 'Beat "sfx" is not supported yet; it is ignored.'); }
            },
            say: {
```

with:

```js
            say: {
```

Second, replace:

```js
        var kinds = {
```

with:

```js
        // ---- sfx (frozen 4.6): the take is chosen at step 5, the event is written when the beat runs.
        var sfxPicks = {}, sfxEvents = [], lastTake = c.lastTake || new Map();

        function sfxImpact(b, j) {
            var entry = S.sfxEntry(cat, b.sound);
            if (!entry) {
                warn('warning', beatPath(j, 'sound'), 'unknown_asset', 'Unknown sound "' + b.sound + '"; the beat is silent.');
                return 0;
            }
            sfxPicks[j] = SM.motion.pickTake(sid, j, entry, 0, lastTake);
            return sfxPicks[j].impact;
        }

        var kinds = {
```

Third, replace:

```js
        function textPosition(at, path) {
```

with:

```js
        kinds.sfx = {
            dur: function () { return 0; },
            run: function (b, j, t0) {
                var pick = sfxPicks[j];
                if (pick) sfxEvents.push({ ref: b.sound, t: t0, take: pick.take });
            }
        };

        function textPosition(at, path) {
```

Fourth, in the scheduler at frozen step 5, M9 leaves the impact at 0 for every beat. Replace:

```js
impact = 0
```

with:

```js
impact = b.do === 'sfx' ? sfxImpact(b, j) : 0
```

Fifth, correct M9's comment above that line, because it no longer holds. Replace:

```js
            // impact stays 0 until the catalog has sfx entries
```

with:

```js
            // impact: the chosen take's impact for an sfx, 0 for a sound the catalog lacks and for every other beat
```

Sixth, correct the same claim in M9's header comment. Replace:

```js
// and text) hide camera wait; sfx is placed with impact 0; the other beats are reported as minor not_supported_yet and
```

with:

```js
// and text) hide camera wait; sfx lands by its take's impact; the other beats are reported as minor not_supported_yet and
```

Finally, add the field to the `buildStoryboard` return, on the line of fields that M8 and M10 leave. Replace:

```js
actorsPlaced: placed, log: log,
```

with:

```js
actorsPlaced: placed, log: log, sfxEvents: sfxEvents,
```

In `ui/js/storymaker/compile.js`, make three edits. Replace:

```js
        var styleFont = story.style && story.style.font || 'default';
```

with:

```js
        var styleFont = story.style && story.style.font || 'default';
        var lastTake = new Map(); // soundId -> take, spans the film (frozen 4.3)
```

Replace:

```js
castMap: castMap
```

with:

```js
castMap: castMap, lastTake: lastTake
```

Replace:

```js
            var base = scene.start_frame / fps;
```

with:

```js
            var base = scene.start_frame / fps;
            // sfx events: film-global start from the scene-local event time (frozen section 3).
            (built.sfxEvents || []).forEach(function (ev) {
                audioEvents.push({ kind: 'sfx', ref: ev.ref, start_ms: Math.round((base + ev.t) * 1000), scene_id: sc.id, take: ev.take });
            });
```

- [ ] **Step 5: Run**

```powershell
npm run build:ui
npm run test:storymaker
go test ./internal/storymaker -run '(?i)(catalog|validate)' -count=1
```

Expected: `0 failed` on the last line of the player suite, and `ok  aurago/internal/storymaker`.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5. A PARTIAL answer on the new file needs a grep note in the commit body.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add scripts/storymaker-tests/21-sfx.mjs scripts/storymaker-tests/04-compile.mjs scripts/storymaker-tests/00-fixtures.mjs ui/js/storymaker/beats.js ui/js/storymaker/scene.js ui/js/storymaker/motion.js ui/js/storymaker/compile.js ui/js/storymaker/player.bundle.js internal/storymaker/testdata/catalog-mini/catalog.json internal/storymaker/catalog_test.go ui/storymaker_player_gate_browser_test.go
git commit -m "feat(storymaker): sfx beat lands by its take's impact, takes rotate across the film" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A7: `sfx.typing` expansion
**Batch:** 2C  **Depends on:** A6
**Files:**
- Modify: `ui/js/storymaker/motion.js`
- Modify: `ui/js/storymaker/beats.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated by `npm run build:ui`)
- Test: `scripts/storymaker-tests/21-sfx.mjs`

- [ ] **Step 1: Impact**

Run from the main checkout:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "buildStoryboard" --direction upstream --repo .
```

Report callers, processes and risk. For HIGH, CRITICAL and UNKNOWN, follow the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

Append to the end of `scripts/storymaker-tests/21-sfx.mjs`:

```js

const typingStory = id => ({ scenes: [{ id, transition: 'cut', beats: [{ do: 'sfx', sound: 'sfx.typing', duration: 2 }] }] });

check('typing: keys sit 90 to 140 ms apart inside the duration', () => {
    const x = sfxEvents(SM.compile(typingStory('a'), {}, catalog).report).map(e => e.start_ms + takesOf('sfx.typing')[e.take].impact_ms);
    // The loop stops only when x reaches 2000 ms. x starts at the first take's impact (at most 13 ms in the mini catalog)
    // and grows by at most 140 ms, so there are at least 15 keys (13+140k < 2000 holds up to k = 14). It grows by at least
    // 90 ms, so there are at most floor(2000/90)+1 = 23.
    assert.ok(x.length >= 15 && x.length <= 23, 'key count ' + x.length);
    // start_ms is a rounded whole ms, which moves one gap by at most 1 ms.
    for (let i = 1; i < x.length; i++) {
        const gap = x[i] - x[i - 1];
        assert.ok(gap >= 89 && gap <= 141, 'gap ' + gap);
    }
    assert.ok(x[x.length - 1] < 2001, 'the last key sits inside the duration');
});

check('typing: takes are seeded and two compiles match', () => {
    const once = sfxEvents(SM.compile(typingStory('a'), {}, catalog).report);
    assert.deepEqual(sfxEvents(SM.compile(typingStory('a'), {}, catalog).report), once);
    for (let i = 1; i < once.length; i++) assert.notEqual(once[i].take, once[i - 1].take, 'no take twice in a row');
    const other = sfxEvents(SM.compile(typingStory('b'), {}, catalog).report);
    assert.notDeepEqual(other.map(e => e.start_ms), once.map(e => e.start_ms), 'the scene id seeds the spacing');
});

check('typing: the default duration scales with rhythm', () => {
    const frames = rhythm => SM.compile({ style: { rhythm }, scenes: [{ id: 'a', transition: 'cut', beats: [{ do: 'sfx', sound: 'sfx.typing' }] }] }, {}, catalog).report.scenes[0].end_frame;
    // 1200 ms times the factor at 30 fps: calm 1.5 s = 45 frames, standard 1.2 s = 36 frames. Lively is 0.96 s, under the
    // 1.0 s minimum of a scene without speech, so it stays at 30 frames.
    assert.equal(frames('calm'), 45);
    assert.equal(frames('standard'), 36);
    assert.equal(frames('lively'), 30);
});
```

- [ ] **Step 3: Run it**

```powershell
npm run test:storymaker
```

Expected: three `FAIL typing:` lines. A pattern sfx still has duration 0 by default and writes one event, so the key count is 1 and the frames stay at 30.

- [ ] **Step 4: Implement**

In `ui/js/storymaker/motion.js`, using the text A6 left, replace:

```js
    SM.motion.pickTake = pickTake;
```

with:

```js
    var MAX_TYPING_KEYS = 32768; // the report's event ceiling; a story that never saw Go must not stall compile

    // typingKeys(o) = the key events of a pattern sfx (frozen 4.6 Typing). o = {sceneId, j, entry, first, start, dur, lastTake}.
    // first is the take of step 5. x starts at the beat's start plus that take's impact; each key starts at x minus the
    // impact of its own take; x advances by a seeded spacing while x < start+dur.
    function typingKeys(o) {
        var spacing = o.entry.pattern.spacing_ms, lo = spacing[0], span = spacing[1] - spacing[0];
        var prng = SM.seed.random(o.sceneId + ':' + o.j + ':' + o.entry.id);
        var pick = o.first, x = o.start + pick.impact, end = o.start + o.dur, key = 0, out = [];
        while (x < end && out.length < MAX_TYPING_KEYS) {
            out.push({ ref: o.entry.id, t: x - pick.impact, take: pick.take });
            x += (lo + span * prng()) / 1000;
            if (x >= end) break;
            key++;
            pick = pickTake(o.sceneId, o.j, o.entry, key, o.lastTake);
        }
        return out;
    }

    SM.motion.pickTake = pickTake;
    SM.motion.typingKeys = typingKeys;
```

In `ui/js/storymaker/beats.js`, using the text A6 left, replace:

```js
        kinds.sfx = {
            dur: function () { return 0; },
            run: function (b, j, t0) {
                var pick = sfxPicks[j];
                if (pick) sfxEvents.push({ ref: b.sound, t: t0, take: pick.take });
            }
        };
```

with:

```js
        var typingBeats = [];

        kinds.sfx = {
            // Frozen 4.6 step 4: 0, or for a pattern its default length times the rhythm factor.
            dur: function (b) {
                var entry = S.sfxEntry(cat, b.sound);
                return entry && entry.pattern ? entry.pattern.default_duration_ms / 1000 * SM.motion.factor((c.story && c.story.style) || {}) : 0;
            },
            run: function (b, j, t0, dur) {
                var pick = sfxPicks[j], entry = S.sfxEntry(cat, b.sound);
                if (!pick) return;
                if (entry.pattern) typingBeats.push({ j: j, entry: entry, first: pick, start: t0, dur: dur });
                else sfxEvents.push({ ref: b.sound, t: t0, take: pick.take });
            }
        };
```

In the same file, expand the typing beats right after the beat loop, where every beat has run. Replace:

```js
        var motionEnd = SM.camera.end(cam);
```

with:

```js
        typingBeats.forEach(function (o) {
            sfxEvents = sfxEvents.concat(SM.motion.typingKeys({ sceneId: sid, j: o.j, entry: o.entry, first: o.first, start: o.start, dur: o.dur, lastTake: lastTake }));
        });

        var motionEnd = SM.camera.end(cam);
```

- [ ] **Step 5: Run**

```powershell
npm run build:ui
npm run test:storymaker
```

Expected: `0 failed` on the last line.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/storymaker/motion.js ui/js/storymaker/beats.js ui/js/storymaker/player.bundle.js scripts/storymaker-tests/21-sfx.mjs
git commit -m "feat(storymaker): sfx.typing expands into seeded key events" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A8: Auto-SFX
**Batch:** 2C  **Depends on:** A6
**Files:**
- Modify: `ui/js/storymaker/motion.js`
- Modify: `ui/js/storymaker/beats.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated by `npm run build:ui`)
- Test: `scripts/storymaker-tests/21-sfx.mjs`

- [ ] **Step 1: Impact**

Run from the main checkout:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "buildStoryboard" --direction upstream --repo .
```

Report callers, processes and risk. For HIGH, CRITICAL and UNKNOWN, follow the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

Append to the end of `scripts/storymaker-tests/21-sfx.mjs`:

```js

const autoStory = (style, beats, narration) => ({ style, scenes: [{ id: 'a', transition: 'cut', narration, beats }] });

check('auto-sfx: each pop entry gets sfx.pop on its landing, and absent means off', () => {
    const beats = [
        { do: 'show', item: 'prop.lock', at: 'right', entry: 'pop' },
        { do: 'show', text: 'Hallo', at: 'center', entry: 'fade' },
        { do: 'show', item: 'prop.shield', at: 'left', entry: 'pop' }
    ];
    const events = sfxEvents(SM.compile(autoStory({ auto_sfx: true }, beats), {}, catalog).report);
    assert.equal(events.length, 2, 'the fade entry gets none');
    // Each show lasts 0.5 s. Another target starts at 40 % of the previous beat (frozen 4.6 rule c): lock 0, text 0.2,
    // shield 0.4. A pop on a prop lands at the spring's lead fraction of its duration.
    const lead = SM.motion.leadFraction('spring') * 0.5;
    [lead, 0.4 + lead].forEach((landing, i) => {
        assert.equal(events[i].ref, 'sfx.pop');
        near(events[i].start_ms, (landing - takesOf('sfx.pop')[events[i].take].impact_ms / 1000) * 1000, 1);
    });
    assert.notEqual(events[0].take, events[1].take, 'the rotation covers auto pops');
    assert.equal(sfxEvents(SM.compile(autoStory({}, beats), {}, catalog).report).length, 0, 'absent');
    assert.equal(sfxEvents(SM.compile(autoStory({ auto_sfx: false }, beats), {}, catalog).report).length, 0, 'false');
});

check('auto-sfx: an sfx beat on the same cue or with:"prev" on the show replaces the pop', () => {
    const text = 'eins zwei {1}drei vier {2}sieben acht';
    const refs = beats => sfxEvents(SM.compile(autoStory({ auto_sfx: true }, beats, text), {}, catalog).report).map(e => e.ref);
    const shown = cue => ({ do: 'show', item: 'prop.lock', at: 'right', entry: 'pop', cue });
    assert.deepEqual(refs([shown(1), { do: 'sfx', sound: 'sfx.whoosh', cue: 1 }]), ['sfx.whoosh']);
    assert.deepEqual(refs([shown(1), { do: 'sfx', sound: 'sfx.whoosh', cue: 2 }]).sort(), ['sfx.pop', 'sfx.whoosh'], 'another cue keeps the pop');
    assert.deepEqual(refs([{ do: 'show', item: 'prop.lock', at: 'right', entry: 'pop' }, { do: 'sfx', sound: 'sfx.whoosh', with: 'prev' }]), ['sfx.whoosh']);
});
```

- [ ] **Step 3: Run it**

```powershell
npm run test:storymaker
```

Expected: two `FAIL auto-sfx:` lines. No pop event exists yet, so the count is 0 instead of 2, and the "another cue" list lacks `sfx.pop`.

- [ ] **Step 4: Implement**

In `ui/js/storymaker/motion.js`, using the text A7 left, replace:

```js
    SM.motion.typingKeys = typingKeys;
```

with:

```js
    SM.motion.typingKeys = typingKeys;

    function hasCue(b) { return b.cue !== undefined && b.cue !== null; }

    // autoSfxEvents(o) = one sfx.pop event on the landing of each show with entry pop (frozen 4.6 Auto-SFX), unless an sfx
    // beat has the same cue or runs with:"prev" on it. o = {sceneId, log, beats, entry, lastTake}. Log rows are in beat order.
    function autoSfxEvents(o) {
        var cued = {}, out = [];
        o.log.forEach(function (row) {
            var b = o.beats[row.j];
            if (b.do === 'sfx' && hasCue(b)) cued[b.cue] = true;
        });
        o.log.forEach(function (row, k) {
            var b = o.beats[row.j], next = o.log[k + 1], after = next ? o.beats[next.j] : null;
            if (b.do !== 'show' || b.entry !== 'pop') return;
            if (hasCue(b) && cued[b.cue]) return;
            if (after && after.do === 'sfx' && after.with === 'prev' && !hasCue(after)) return;
            var pick = pickTake(o.sceneId, row.j, o.entry, 0, o.lastTake);
            out.push({ ref: o.entry.id, t: Math.max(0, row.landing - pick.impact), take: pick.take });
        });
        return out;
    }

    SM.motion.autoSfxEvents = autoSfxEvents;
```

In `ui/js/storymaker/beats.js`, using the text A7 left, replace:

```js
        var motionEnd = SM.camera.end(cam);
```

with:

```js
        var popEntry = c.story && c.story.style && c.story.style.auto_sfx === true ? S.sfxEntry(cat, 'sfx.pop') : null;
        if (popEntry) sfxEvents = sfxEvents.concat(SM.motion.autoSfxEvents({ sceneId: sid, log: log, beats: beats, entry: popEntry, lastTake: lastTake }));

        var motionEnd = SM.camera.end(cam);
```

`log` is the beat log of frozen 4.3 that M8 keeps in `buildStoryboard`.

- [ ] **Step 5: Run**

```powershell
npm run build:ui
npm run test:storymaker
```

Expected: `0 failed` on the last line.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/storymaker/motion.js ui/js/storymaker/beats.js ui/js/storymaker/player.bundle.js scripts/storymaker-tests/21-sfx.mjs
git commit -m "feat(storymaker): auto_sfx adds a pop on each pop entry's landing" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A9: Report validation for sfx
**Batch:** 2C  **Depends on:** A4, A7, A8
**Files:**
- Modify: `internal/storymaker/render/report.go`
- Modify: `internal/storymaker/render/render.go`
- Test: `internal/storymaker/render/report_sfx_test.go` (new)

- [ ] **Step 1: Impact**

Run from the main checkout for `parseReport`, `reportExpect` and `onReport`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "parseReport" --direction upstream --repo .
```

Report callers, processes and risk. For HIGH, CRITICAL and UNKNOWN, follow the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

Create `internal/storymaker/render/report_sfx_test.go`:

```go
package render

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/storymaker"
)

func sfxMiniCatalog(t *testing.T) *storymaker.Catalog {
	t.Helper()
	cat, err := storymaker.LoadCatalog(os.DirFS("../testdata/catalog-mini"), "catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func sfxReportEvent(ref string, startMS, take int) map[string]any {
	return map[string]any{"kind": "sfx", "ref": ref, "start_ms": startMS, "scene_id": "s1", "take": take}
}

func TestParseReportChecksSFXEvents(t *testing.T) {
	cat := sfxMiniCatalog(t)
	entry, ok := cat.Lookup("sfx.pop")
	if !ok {
		t.Fatal("the mini catalog has no sfx.pop")
	}
	want := reportExpectation()
	want.Catalog = cat
	want.MaxEvents = 8
	parse := func(ev map[string]any) *reportRefusal {
		m := goodReport()
		m["audio_events"] = []any{ev}
		body, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		_, refusal := parseReport(body, want)
		return refusal
	}
	if refusal := parse(sfxReportEvent("sfx.pop", 100, len(entry.Takes)-1)); refusal != nil {
		t.Fatalf("the last take was refused: %+v", refusal)
	}
	for name, ev := range map[string]map[string]any{
		"take too large": sfxReportEvent("sfx.pop", 100, len(entry.Takes)),
		"negative take":  sfxReportEvent("sfx.pop", 100, -1),
		"unknown sound":  sfxReportEvent("sfx.nope", 100, 0),
		"not a sound":    sfxReportEvent("prop.lock", 100, 0),
	} {
		if refusal := parse(ev); refusal == nil || refusal.Code != CodeRenderFailed {
			t.Errorf("%s: refusal = %+v", name, refusal)
		}
	}
}

func TestParseReportEventCeiling(t *testing.T) {
	want := reportExpectation()
	want.Catalog = sfxMiniCatalog(t)
	want.MaxEvents = 1 << 20 // the ceiling wins over a larger bound
	build := func(n int) []byte {
		events := make([]any, n)
		for i := range events {
			events[i] = sfxReportEvent("sfx.pop", i%10000, 0) // goodReport lasts 10000 ms
		}
		m := goodReport()
		m["audio_events"] = events
		body, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	if _, refusal := parseReport(build(32768), want); refusal != nil {
		t.Fatalf("32768 events: %+v", refusal)
	}
	if _, refusal := parseReport(build(32769), want); refusal == nil || refusal.Code != CodeRenderFailed {
		t.Fatalf("32769 events: %+v", refusal)
	}
}

func TestSFXEventBound(t *testing.T) {
	cat := sfxMiniCatalog(t)
	story := storymaker.Story{Scenes: []storymaker.Scene{
		{ID: "a", Beats: []storymaker.Beat{
			{Do: "sfx", Sound: "sfx.typing"},              // floor(1200*1.25/90)+1 = floor(16.67)+1 = 17
			{Do: "sfx", Sound: "sfx.pop"},                 // 1
			{Do: "sfx", Sound: "sfx.typing", Duration: 3}, // max(3000, 1500): floor(3000/90)+1 = 33+1 = 34
			{Do: "show", Text: "x", Entry: "pop"},         // 1 with auto_sfx
			{Do: "show", Text: "y", Entry: "fade"},        // 0
		}},
		{ID: "c", Kind: "code", Beats: []storymaker.Beat{{Do: "sfx", Sound: "sfx.pop"}}}, // a code scene's beats never run
	}}
	if got := sfxEventBound(&story, cat); got != 17+1+34 {
		t.Errorf("without auto_sfx: %d", got)
	}
	story.Style.AutoSFX = true
	if got := sfxEventBound(&story, cat); got != 17+1+34+1 {
		t.Errorf("with auto_sfx: %d", got)
	}
}

func TestMaxRhythmFactorMatchesMotion(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(uiDirForTests, "js", "storymaker", "motion.js"))
	if err != nil {
		t.Fatal(err)
	}
	table := regexp.MustCompile(`RHYTHM\s*=\s*\{([^}]*)\}`).FindSubmatch(src)
	if table == nil {
		t.Fatal("motion.js has no RHYTHM table")
	}
	largest := 0.0
	for _, m := range regexp.MustCompile(`:\s*([0-9.]+)`).FindAllSubmatch(table[1], -1) {
		v, err := strconv.ParseFloat(string(m[1]), 64)
		if err != nil {
			t.Fatal(err)
		}
		largest = max(largest, v)
	}
	if largest != maxRhythmFactor {
		t.Fatalf("largest RHYTHM value %v, maxRhythmFactor %v", largest, maxRhythmFactor)
	}
}

func TestReportAcceptsRealTypingAndAutoSFX(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for the real compile")
	}
	cat, err := storymaker.LoadCatalog(os.DirFS(uiDirForTests), "img/storymaker/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	scene := func(id string) string {
		return `{"id":"` + id + `","environment":"env.plain-stage","transition":"cut","beats":[` +
			`{"do":"sfx","sound":"sfx.typing"},{"do":"show","text":"Eins","at":"center","entry":"pop"}]}`
	}
	storyJSON := `{"schema":"storymaker/1","title":"T","format":{"aspect":"16:9","fps":30},` +
		`"style":{"palette":"sunny","font":"default","captions":true,"rhythm":"calm","auto_sfx":true},"voice":null,"music":null,"cast":[],"scenes":[` +
		strings.Join([]string{scene("s1"), scene("s2"), scene("s3"), scene("s4")}, ",") + `]}`
	story, _, err := storymaker.DecodeStory([]byte(storyJSON), storymaker.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	script := `import fs from 'node:fs';
import { loadPlayer, realCatalog } from './scripts/storymaker-test-support.mjs';
const story = JSON.parse(fs.readFileSync(0, 'utf8'));
process.stdout.write(JSON.stringify(loadPlayer().StorymakerCore.compile(story, {}, realCatalog()).report));`
	cmd := exec.Command(node, "--input-type=module", "-e", script)
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(storyJSON)
	body, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			t.Fatalf("node: %v\n%s", err, ee.Stderr)
		}
		t.Fatalf("node: %v", err)
	}

	bound := sfxEventBound(&story, cat)
	if bound != 4*(17+1) { // each scene: floor(1200*1.25/90)+1 = 17 keys and one auto pop
		t.Fatalf("bound %d", bound)
	}
	want := reportExpect{FPS: 30, Width: 1920, Height: 1080, MaxFrames: 27000, CatalogVersion: cat.Version(),
		SceneIDs: []string{"s1", "s2", "s3", "s4"}, Catalog: cat, SFXEvents: bound}
	rep, refusal := parseReport(body, want)
	if refusal != nil {
		t.Fatalf("a real compile was refused: %+v", refusal)
	}

	typing, ok := cat.Lookup("sfx.typing")
	if !ok {
		t.Fatal("the real catalog has no sfx.typing")
	}
	keys, pops := map[string]int{}, map[string]int{}
	lastX := 0.0
	for _, ev := range rep.AudioEvents {
		switch ev.Ref {
		case "sfx.typing":
			keys[ev.SceneID]++
			if ev.SceneID == "s1" {
				lastX = max(lastX, float64(ev.StartMS)+typing.Takes[ev.Take].ImpactMS)
			}
		case "sfx.pop":
			pops[ev.SceneID]++
		}
	}
	for _, id := range want.SceneIDs {
		// Calm lasts 1500 ms. x starts at most 13.5 ms in and grows by 90 to 140 ms, so at least 11 keys and at most 17.
		if keys[id] < 11 || keys[id] > 17 || pops[id] != 1 {
			t.Errorf("scene %s: %d keys, %d pops", id, keys[id], pops[id])
		}
	}
	// The loop stops only when x reaches 1500 ms, with a step of at most 140 ms, so the last x is at least 1360 ms.
	// Rounding start_ms takes off at most 1 ms. This is past the 1200 ms a standard rhythm would stop at.
	if lastX < 1359 {
		t.Errorf("the calm typing beat stopped early: last key at %v ms", lastX)
	}
}
```

- [ ] **Step 3: Run it**

```powershell
go test ./internal/storymaker/render -run '(?i)(sfx|eventceiling|rhythmfactor)' -count=1
```

Expected: the test build fails with `undefined: sfxEventBound` and `unknown field SFXEvents in struct literal of type reportExpect`.

- [ ] **Step 4: Implement**

In `internal/storymaker/render/report.go`, as N21 leaves it, make these edits.

First, add the fields. N21 declares neither of them. Replace:

```go
	SceneIDs       []string
```

with:

```go
	SceneIDs       []string
	Catalog        *storymaker.Catalog // sfx refs
	SFXEvents      int                 // sfx share of MaxEvents, from sfxEventBound
```

Second, fold the sfx share into the bound at the top of `parseReport`, under N21's ceiling `maxReportEvents`. This keeps N21's own count check as it is. Replace:

```go
	var rep compileReport
```

with:

```go
	want.MaxEvents = min(want.MaxEvents+want.SFXEvents, maxReportEvents)
	var rep compileReport
```

Third, replace:

```go
	return rep, nil
}
```

with:

```go
	if msg := checkSFXEvents(rep.AudioEvents, want.Catalog); msg != "" {
		return bad("%s", msg)
	}
	return rep, nil
}
```

Fourth, N21's event loop refuses kind `sfx` outright. Let the event pass: `checkSFXEvents` now owns the ref and the take. N21's time bound stays in force for every kind. Replace:

```go
		case "sfx":
			// The catalog has no sound effects yet, so no such event can be valid.
			return fmt.Sprintf("audio event %d is a sound effect, which the catalog does not have", i)
```

with:

```go
		case "sfx":
			// checkSFXEvents owns the ref and the take.
```

Fifth, append to the end of the file:

```go

// sfxEventBound is the sfx share of MaxEvents: one event per sfx beat, for a pattern
// floor(max(duration, default duration at the largest rhythm)/smallest spacing)+1, and one per show
// with entry pop when auto_sfx is on. The beats of a code scene never run.
func sfxEventBound(s *storymaker.Story, cat *storymaker.Catalog) int {
	n := 0
	for i := range s.Scenes {
		if s.Scenes[i].Kind == "code" {
			continue
		}
		for j := range s.Scenes[i].Beats {
			b := &s.Scenes[i].Beats[j]
			switch {
			case b.Do == "sfx":
				n += sfxBeatBound(b, cat)
			case b.Do == "show" && b.Entry == "pop" && s.Style.AutoSFX:
				n++
			}
		}
	}
	return n
}

func sfxBeatBound(b *storymaker.Beat, cat *storymaker.Catalog) int {
	entry, ok := cat.Lookup(b.Sound)
	if !ok || entry.Pattern == nil || entry.Pattern.SpacingMS[0] <= 0 {
		return 1
	}
	span := max(b.Duration*1000, entry.Pattern.DefaultDurationMS*maxRhythmFactor)
	return int(span/entry.Pattern.SpacingMS[0]) + 1 // a positive float truncates to its floor
}

// checkSFXEvents says why an sfx event is refused, or returns "". Its ref must be a catalog sfx and
// its take an index into that entry's takes. A nil catalog refuses every sfx event.
func checkSFXEvents(events []reportAudioEvent, cat *storymaker.Catalog) string {
	for i := range events {
		ev := &events[i]
		if ev.Kind != "sfx" {
			continue
		}
		entry, ok := cat.Lookup(ev.Ref)
		if !ok || entry.Kind != "sfx" {
			return fmt.Sprintf("the sfx event %d names the unknown sound %q", i, clip(ev.Ref, 80))
		}
		if ev.Take < 0 || ev.Take >= len(entry.Takes) {
			return fmt.Sprintf("the sfx event %d of %q has take %d of %d", i, ev.Ref, ev.Take, len(entry.Takes))
		}
	}
	return ""
}
```

Sixth, N21 does not declare the rhythm constant. Append to the end of the file:

```go

const maxRhythmFactor = 1.25 // pinned against RHYTHM in motion.js by a test
```

In `internal/storymaker/render/render.go`, inside the `reportExpect` literal of `onReport`, replace:

```go
CatalogVersion: run.catVersion
```

with:

```go
CatalogVersion: run.catVersion, Catalog: run.r.deps.Catalog, SFXEvents: sfxEventBound(&run.plan.Story, run.r.deps.Catalog)
```

- [ ] **Step 5: Run**

```powershell
gofmt -w internal/storymaker/render/report.go internal/storymaker/render/render.go internal/storymaker/render/report_sfx_test.go
go test ./internal/storymaker/render -run '(?i)(report|sfx|eventceiling|rhythmfactor)' -count=1
```

Expected: `ok  aurago/internal/storymaker/render`. Count the PASS lines with `-v` when this run serves as evidence: all five new tests must pass, and none may skip on this box, where `node` is installed.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5. A PARTIAL answer on the new test file needs a grep note in the commit body.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/render/report.go internal/storymaker/render/render.go internal/storymaker/render/report_sfx_test.go
git commit -m "feat(storymaker): report validation for sfx events, takes and event bound" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

<!-- G-A-3 -->

### Task A10: Meter
**Batch:** 2C  **Depends on:** none
**Files:**
- Create: `internal/storymaker/render/meter.go`
- Test: `internal/storymaker/render/meter_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/render/meter_test.go`:

```go
package render

import (
	"context"
	"math"
	"reflect"
	"testing"
)

// Stored FFmpeg 8.1.1 output: a default framelog line and the ebur128 summary from the spike's
// speech.ebu.txt, and the loudnorm JSON of the same file from speech.pass2.txt. The frame line says
// I -17.3 and the JSON says -16.69, so only the summary line gives -16.6.
const meterStderrSpeech = `[Parsed_ebur128_0 @ 000002499d844d40] t: 1.599979   TARGET:-23 LUFS    M: -29.6 S:-120.7     I: -17.3 LUFS       LRA:   0.0 LU  FTPK:  -inf  -inf dBFS  TPK:  -2.4  -2.4 dBFS
[Parsed_ebur128_0 @ 000002499d844d40] Summary:

  Integrated loudness:
    I:         -16.6 LUFS
    Threshold: -27.1 LUFS

  Loudness range:
    LRA:         2.7 LU
    Threshold: -37.5 LUFS
    LRA low:   -18.5 LUFS
    LRA high:  -15.8 LUFS

  True peak:
    Peak:       -1.0 dBFS
[Parsed_loudnorm_0 @ 00000185953e53c0] 
{
	"input_i" : "-16.69",
	"input_tp" : "-1.00",
	"input_lra" : "2.10",
	"input_thresh" : "-27.16",
	"output_i" : "-16.29",
	"output_tp" : "-1.00",
	"output_lra" : "1.60",
	"output_thresh" : "-26.79",
	"normalization_type" : "dynamic",
	"target_offset" : "0.29"
}
[out#0/null @ 00000185953e4800] video:0KiB audio:20628KiB subtitle:0KiB other streams:0KiB global headers:0KiB muxing overhead: unknown
`

// The JSON is stored from the spike's silent.pass1.txt. ebur128 reads silence as -70.0 (mixreal/ebur.txt).
const meterStderrSilent = `[Parsed_ebur128_0 @ 000002499d844d40] Summary:

  Integrated loudness:
    I:         -70.0 LUFS
[Parsed_loudnorm_0 @ 0000015e6e443a80] 
{
	"input_i" : "-inf",
	"input_tp" : "-inf",
	"input_lra" : "0.00",
	"input_thresh" : "-70.00",
	"output_i" : "-inf",
	"output_tp" : "-inf",
	"output_lra" : "0.00",
	"output_thresh" : "-70.00",
	"normalization_type" : "dynamic",
	"target_offset" : "inf"
}
`

func TestMeterArgsPinTheGraph(t *testing.T) {
	want := []string{
		"-hide_banner", "-nostdin", "-nostats", "-loglevel", "info",
		"-protocol_whitelist", "file,pipe", "-format_whitelist", "f32le",
		"-f", "f32le", "-ar", "48000", "-ac", "2", "-i", "pipe:0",
		"-af", "ebur128=framelog=quiet,loudnorm=I=-16:TP=-1:LRA=11:print_format=json", "-f", "null", "-",
	}
	if got := MeterArgs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("MeterArgs() = %q", got)
	}
}

func TestMeterParseTakesIFromTheSummaryAndTPFromTheJSON(t *testing.T) {
	got, err := parseMeterOutput(meterStderrSpeech)
	if err != nil {
		t.Fatalf("parseMeterOutput: %v", err)
	}
	if got.I != -16.6 || got.TP != -1.0 || got.Silent {
		t.Fatalf("reading = %+v, want I -16.6, TP -1, not silent", got)
	}
}

func TestMeterParseReadsMinusInfAsSilent(t *testing.T) {
	got, err := parseMeterOutput(meterStderrSilent)
	if err != nil {
		t.Fatalf("parseMeterOutput: %v", err)
	}
	if !got.Silent || !math.IsInf(got.I, -1) || !math.IsInf(got.TP, -1) {
		t.Fatalf("reading = %+v, want silent with -Inf levels", got)
	}
}

func TestMeterParseRefusesOutputWithoutBothMeters(t *testing.T) {
	if _, err := parseMeterOutput("    I:         -16.6 LUFS\n"); err == nil {
		t.Fatal("output without the JSON was accepted")
	}
	if _, err := parseMeterOutput(`{"input_i":"-16.69","input_tp":"-1.00"}`); err == nil {
		t.Fatal("output without the summary was accepted")
	}
}

func runMeterOnSine(t *testing.T, ffmpeg string, seconds int, amp float64) MeterReading {
	t.Helper()
	m, err := StartMeter(context.Background(), ffmpeg)
	if err != nil {
		t.Fatalf("StartMeter: %v", err)
	}
	const rate, blockFrames = 48000, 4800
	block := make([]float32, blockFrames*2)
	for start := 0; start < seconds*rate; start += blockFrames {
		for i := 0; i < blockFrames; i++ {
			v := float32(amp * math.Sin(2*math.Pi*1000*float64(start+i)/rate))
			block[i*2], block[i*2+1] = v, v
		}
		if err := m.Write(block); err != nil {
			_, closeErr := m.Close()
			t.Fatalf("Write: %v (Close: %v)", err, closeErr)
		}
	}
	reading, err := m.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	return reading
}

func TestMeterRealGraph(t *testing.T) {
	ffmpeg := testBinary(t, "ffmpeg")

	// EBU Tech 3341 case 1: a 1 kHz stereo sine reads its per-channel peak level in LUFS. So amplitude
	// 0.1 (-20 dBFS) reads -20 LUFS, and the true peak of a sine is its peak, -20 dBTP.
	loud := runMeterOnSine(t, ffmpeg, 5, 0.1)
	if loud.Silent || math.Abs(loud.I+20) > 0.5 || math.Abs(loud.TP+20) > 0.5 {
		t.Fatalf("sine reading = %+v, want about -20 LUFS and -20 dBTP", loud)
	}

	silent := runMeterOnSine(t, ffmpeg, 5, 0)
	if !silent.Silent {
		t.Fatalf("silence reading = %+v, want silent", silent)
	}
}
```

- [ ] **Step 2: Run it**

In `.worktrees/storymaker-p2c`:

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/render -run '(?i)meter' -count=1
```

Expected: the test package does not build, with `undefined: MeterArgs`, `undefined: parseMeterOutput`, `undefined: StartMeter`, because `meter.go` does not exist yet.

- [ ] **Step 3: Implement**

Create `internal/storymaker/render/meter.go`:

```go
package render

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// meterFilter runs both meters in one graph. I comes from ebur128, because loudnorm under-reads
// short films. TP comes from loudnorm.
const meterFilter = "ebur128=framelog=quiet,loudnorm=I=-16:TP=-1:LRA=11:print_format=json"

// meterErrorBytes is how much of FFmpeg's closing text a failed meter keeps.
const meterErrorBytes = 400

// meterSummaryI matches the I: line of ebur128's summary. A per-frame line starts with the filter
// name, so it never matches.
var meterSummaryI = regexp.MustCompile(`(?m)^\s*I:\s+(\S+)\s+LUFS\s*$`)

// MeterReading is one measurement. A silent reading holds -Inf in both levels.
type MeterReading struct {
	I, TP  float64
	Silent bool // loudnorm reported -inf
}

// Meter is a running FFmpeg process that measures float samples written to its stdin.
type Meter struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr *tailBuffer
	buf    []byte
	closed bool
	result MeterReading
	err    error
}

// MeterArgs returns the FFmpeg arguments of a meter. -loglevel info is needed, because both reports
// vanish at error.
func MeterArgs() []string {
	return []string{
		"-hide_banner", "-nostdin", "-nostats", "-loglevel", "info",
		"-protocol_whitelist", "file,pipe", "-format_whitelist", "f32le",
		"-f", "f32le", "-ar", "48000", "-ac", "2", "-i", "pipe:0",
		"-af", meterFilter, "-f", "null", "-",
	}
}

// StartMeter starts the process. ctx ends it.
func StartMeter(ctx context.Context, ffmpegPath string) (*Meter, error) {
	cmd := exec.CommandContext(ctx, ffmpegPath, MeterArgs()...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("storymaker: open the meter input: %w", err)
	}
	m := &Meter{cmd: cmd, stdin: stdin, stderr: &tailBuffer{limit: ffmpegStderrKeep}}
	cmd.Stderr = m.stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("storymaker: start the FFmpeg meter: %w", err)
	}
	return m, nil
}

// Write sends interleaved stereo samples to the meter.
func (m *Meter) Write(interleaved []float32) error {
	need := len(interleaved) * 4
	if cap(m.buf) < need {
		m.buf = make([]byte, need)
	}
	b := m.buf[:need]
	for i, v := range interleaved {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(v))
	}
	if _, err := m.stdin.Write(b); err != nil {
		return fmt.Errorf("storymaker: write to the FFmpeg meter: %w", err)
	}
	return nil
}

// Close ends the input, waits for FFmpeg and parses its reports. A second call returns the first answer.
func (m *Meter) Close() (MeterReading, error) {
	if m.closed {
		return m.result, m.err
	}
	m.closed = true
	_ = m.stdin.Close()
	waitErr := m.cmd.Wait()
	text := m.stderr.String()
	if waitErr != nil {
		tail := strings.TrimSpace(text)
		if len(tail) > meterErrorBytes {
			tail = tail[len(tail)-meterErrorBytes:]
		}
		m.err = fmt.Errorf("storymaker: the FFmpeg meter failed: %w: %s", waitErr, tail)
		return m.result, m.err
	}
	m.result, m.err = parseMeterOutput(text)
	return m.result, m.err
}

// parseMeterOutput reads I from the last I: line of ebur128's summary and TP from the loudnorm JSON
// between the last { and the last }. The JSON values are strings.
func parseMeterOutput(stderr string) (MeterReading, error) {
	lines := meterSummaryI.FindAllStringSubmatch(stderr, -1)
	if len(lines) == 0 {
		return MeterReading{}, errors.New("storymaker: the meter printed no loudness summary")
	}
	integrated, err := strconv.ParseFloat(lines[len(lines)-1][1], 64)
	if err != nil || math.IsNaN(integrated) {
		return MeterReading{}, errors.New("storymaker: the meter printed an unreadable loudness")
	}
	open, end := strings.LastIndex(stderr, "{"), strings.LastIndex(stderr, "}")
	if open < 0 || end < open {
		return MeterReading{}, errors.New("storymaker: the meter printed no peak report")
	}
	var report struct {
		InputI  string `json:"input_i"`
		InputTP string `json:"input_tp"`
	}
	if err := json.Unmarshal([]byte(stderr[open:end+1]), &report); err != nil {
		return MeterReading{}, fmt.Errorf("storymaker: read the meter's peak report: %w", err)
	}
	inputI, err := strconv.ParseFloat(report.InputI, 64)
	if err != nil {
		return MeterReading{}, fmt.Errorf("storymaker: read the meter's input level: %w", err)
	}
	truePeak, err := strconv.ParseFloat(report.InputTP, 64)
	if err != nil || math.IsNaN(truePeak) {
		return MeterReading{}, errors.New("storymaker: the meter printed an unreadable peak")
	}
	if math.IsInf(inputI, -1) || math.IsInf(truePeak, -1) {
		return MeterReading{I: math.Inf(-1), TP: math.Inf(-1), Silent: true}, nil
	}
	return MeterReading{I: integrated, TP: truePeak}, nil
}
```

- [ ] **Step 4: Run on FFmpeg 8.1.1**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
gofmt -w internal/storymaker/render/meter.go internal/storymaker/render/meter_test.go
go vet ./internal/storymaker/render
go test ./internal/storymaker/render -run '(?i)meter' -count=1 -v
```

Expected: five `--- PASS` lines: `TestMeterArgsPinTheGraph`, `TestMeterParseTakesIFromTheSummaryAndTPFromTheJSON`, `TestMeterParseReadsMinusInfAsSilent`, `TestMeterParseRefusesOutputWithoutBothMeters`, `TestMeterRealGraph`. A `--- SKIP` of `TestMeterRealGraph` is not evidence: set `AURAGO_TEST_FFMPEG_DIR` to the FFmpeg folder and run again.

If `TestMeterRealGraph` fails on this version with an option error on `framelog`, go to Step 6 first.

- [ ] **Step 5: Run on FFmpeg 5.1.9**

Use one `debian:12 --rm` container of your own, with the shell sandbox off. Start Docker Desktop first if it is off. Touch no other container.

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
New-Item -ItemType Directory -Force D:\tmp\go-build\sm-meter | Out-Null
$env:GOOS='linux'; $env:CGO_ENABLED='0'
go test -c -o D:\tmp\go-build\sm-meter\render.test ./internal/storymaker/render
Remove-Item Env:GOOS; Remove-Item Env:CGO_ENABLED
docker run --rm -v D:\tmp\go-build\sm-meter:/t debian:12 sh -c "apt-get update -qq && apt-get install -y -qq ffmpeg >/dev/null && ffmpeg -version | head -n 1 && cp /t/render.test /tmp/render.test && chmod +x /tmp/render.test && cd /tmp && ./render.test -test.run TestMeterRealGraph -test.v -test.count=1"
Remove-Item -Recurse -Force D:\tmp\go-build\sm-meter
```

Expected: a first line that names `ffmpeg version 5.1.9`, then `--- PASS: TestMeterRealGraph`.

- [ ] **Step 6: Only if `framelog=quiet` is refused**

Do this step when Step 4 or Step 5 fails with an FFmpeg message that names `framelog` or `quiet`. Skip it otherwise.

In `internal/storymaker/render/meter.go`, replace

```go
const meterFilter = "ebur128=framelog=quiet,loudnorm=I=-16:TP=-1:LRA=11:print_format=json"
```

with

```go
//
// framelog=quiet is refused by FFmpeg 5.1. framelog=verbose prints nothing at -loglevel info.
const meterFilter = "ebur128=framelog=verbose,loudnorm=I=-16:TP=-1:LRA=11:print_format=json"
```

In `internal/storymaker/render/meter_test.go`, replace

```go
		"-af", "ebur128=framelog=quiet,loudnorm=I=-16:TP=-1:LRA=11:print_format=json", "-f", "null", "-",
```

with

```go
		"-af", "ebur128=framelog=verbose,loudnorm=I=-16:TP=-1:LRA=11:print_format=json", "-f", "null", "-",
```

Then rerun Step 4 and Step 5. Both must pass. Add this line to the commit body in Step 7: `framelog=quiet is refused on FFmpeg 5.1.9, so the graph uses framelog=verbose.`

- [ ] **Step 7: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git grep -n "StartMeter" -- internal
git add internal/storymaker/render/meter.go internal/storymaker/render/meter_test.go
git commit -m "feat(storymaker): loudness meter over an FFmpeg pipe" -m "detect_changes answered PARTIAL on the new files; grep finds StartMeter only in meter.go and meter_test.go." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Drop the middle `-m` if `detect-changes` answers without `partial`. If the tool crashes, closes the connection, answers `not found`, or prints only its banner, use `graft` callers plus grep and say so in that line instead.

### Task A11: Mix core: clips, bed, loop, stems
**Batch:** 2C  **Depends on:** none
**Files:**
- Create: `internal/storymaker/render/mix.go`
- Test: `internal/storymaker/render/mix_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/render/mix_test.go`:

```go
package render

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func mixTestSine(frames int, amp, hz float64) []int16 {
	pcm := make([]int16, frames*mixChannels)
	for i := 0; i < frames; i++ {
		v := int16(math.Round(amp * 32767 * math.Sin(2*math.Pi*hz*float64(i)/MixRate)))
		pcm[i*2], pcm[i*2+1] = v, v
	}
	return pcm
}

func mixTestConst(frames int, v int16) []int16 {
	pcm := make([]int16, frames*mixChannels)
	for i := range pcm {
		pcm[i] = v
	}
	return pcm
}

func mixTestWriteRaw(t *testing.T, path string, pcm []int16) {
	t.Helper()
	raw := make([]byte, len(pcm)*2)
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(raw[i*2:], uint16(v))
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func mixTestReadWAV(t *testing.T, path string) []int16 {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < wavHeaderBytes || string(raw[:4]) != "RIFF" || string(raw[36:40]) != "data" {
		t.Fatalf("%s is not a canonical WAV", path)
	}
	data := raw[wavHeaderBytes:]
	if got := binary.LittleEndian.Uint32(raw[40:]); int(got) != len(data) {
		t.Fatalf("%s: the header says %d data bytes, the file has %d", path, got, len(data))
	}
	pcm := make([]int16, len(data)/2)
	for i := range pcm {
		pcm[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
	}
	return pcm
}

func mixTestCollect(t *testing.T, in *MixInput, pass mixPass) (narration, music, sfx []float64) {
	t.Helper()
	err := runMixPass(context.Background(), in, pass, func(b *mixBlock) error {
		narration = append(narration, b.narration...)
		music = append(music, b.music...)
		sfx = append(sfx, b.sfx...)
		return nil
	})
	if err != nil {
		t.Fatalf("runMixPass: %v", err)
	}
	return narration, music, sfx
}

func mixTestAssertStemsSum(t *testing.T, out MixOutputs, frames int64) {
	t.Helper()
	mix := mixTestReadWAV(t, out.Mix)
	narration := mixTestReadWAV(t, out.Narration)
	music := mixTestReadWAV(t, out.Music)
	sfx := mixTestReadWAV(t, out.SFX)
	if int64(len(mix)) != frames*mixChannels {
		t.Fatalf("mix has %d samples, want %d", len(mix), frames*mixChannels)
	}
	if len(narration) != len(mix) || len(music) != len(mix) || len(sfx) != len(mix) {
		t.Fatalf("stem lengths %d, %d, %d differ from the mix %d", len(narration), len(music), len(sfx), len(mix))
	}
	var sounding [3]int
	for i := range mix {
		if int(mix[i]) != int(narration[i])+int(music[i])+int(sfx[i]) {
			t.Fatalf("sample %d: mix %d, stems %d + %d + %d", i, mix[i], narration[i], music[i], sfx[i])
		}
		for k, v := range [3]int16{narration[i], music[i], sfx[i]} {
			if v != 0 {
				sounding[k]++
			}
		}
	}
	for k, n := range sounding {
		if n == 0 {
			t.Errorf("stem %d is silent, so the sum proved nothing for it", k)
		}
	}
}

func TestMixStemsSumToTheMix(t *testing.T) {
	dir := t.TempDir()
	clipPath := filepath.Join(dir, "piece.raw")
	bedPath := filepath.Join(dir, "bed.raw")
	mixTestWriteRaw(t, clipPath, mixTestSine(MixRate, 0.4, 330))
	// One second of bed under a longer film, so the stems also cross the loop.
	mixTestWriteRaw(t, bedPath, mixTestSine(MixRate, 0.3, 110))
	// The 1234 extra frames make the last block a short one.
	frames := int64(3*MixRate + 1234)
	in := &MixInput{
		Frames:    frames,
		Narration: []MixClip{{Key: "piece", Path: clipPath, Len: MixRate, Start: MixRate / 2, Voice: "p|v"}},
		SFX:       []MixClip{{Key: "sfx.pop", PCM: mixTestSine(MixRate/4, 0.2, 880), Len: MixRate / 4, Start: MixRate, GainDB: -3}},
		Bed:       &MixBed{Path: bedPath, Frames: MixRate, Volume: 0.25},
	}
	out := MixOutputs{
		Mix:       filepath.Join(dir, "mix.wav"),
		Narration: filepath.Join(dir, "stem-narration.wav"),
		Music:     filepath.Join(dir, "stem-music.wav"),
		SFX:       filepath.Join(dir, "stem-sfx.wav"),
	}
	w, err := newStemWriter(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := runMixPass(context.Background(), in, mixPass{bedGainDB: -6, masterDB: -2}, w.write); err != nil {
		t.Fatal(err)
	}
	if err := w.close(); err != nil {
		t.Fatal(err)
	}
	if w.clamps != 0 {
		t.Fatalf("%d clamps at these low levels", w.clamps)
	}
	mixTestAssertStemsSum(t, out, frames)
}

func TestMixLoopCrossfadeIsEqualPowerAndAtMostHalfTheBed(t *testing.T) {
	cases := []struct {
		name              string
		bed, film         int64
		wantPeriod, wantX int64
	}{
		// 2 s is below half of 6 s, so the crossfade is 2 s and the period 6 - 2 = 4 s.
		{"long bed", 6 * MixRate, 12 * MixRate, 4 * MixRate, 2 * MixRate},
		// Half of 2 s is 1 s, below the 2 s crossfade, so the crossfade is 1 s and the period 1 s.
		{"short bed", 2 * MixRate, 6 * MixRate, 1 * MixRate, 1 * MixRate},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			period, xfade := loopShape(c.bed, c.film)
			if period != c.wantPeriod || xfade != c.wantX {
				t.Fatalf("loopShape = %d, %d, want %d, %d", period, xfade, c.wantPeriod, c.wantX)
			}
			// The head of the bed holds 0.25 and its tail 0.5, so the blend shows both weights.
			pcm := make([]int16, c.bed*mixChannels)
			for i := int64(0); i < c.bed; i++ {
				v := int16(0)
				if i < xfade {
					v = 8192
				} else if i >= period {
					v = 16384
				}
				pcm[i*2], pcm[i*2+1] = v, v
			}
			path := filepath.Join(t.TempDir(), "bed.raw")
			mixTestWriteRaw(t, path, pcm)
			in := &MixInput{Frames: c.film, Bed: &MixBed{Path: path, Frames: c.bed, Volume: 0.25}}
			_, music, _ := mixTestCollect(t, in, mixPass{rawBed: true})
			for _, pos := range []int64{0, xfade / 4, xfade / 2, xfade * 3 / 4} {
				// Equal power: the rising head has the sine weight and the falling tail the cosine,
				// so at the middle the sum is (0.25+0.5)*sqrt(0.5), not (0.25+0.5)/2.
				u := float64(pos) / float64(xfade) * math.Pi / 2
				want := 0.25*math.Sin(u) + 0.5*math.Cos(u)
				for _, n := range []int64{period + pos, 2*period + pos} {
					if got := music[n*mixChannels]; math.Abs(got-want) > 1e-9 {
						t.Errorf("frame %d = %.6f, want %.6f", n, got, want)
					}
				}
			}
		})
	}
	// A bed as long as the film never loops.
	if period, xfade := loopShape(6*MixRate, 6*MixRate); period != 6*MixRate || xfade != 0 {
		t.Fatalf("a bed of film length loops: %d, %d", period, xfade)
	}
}

func TestMixTrimmedClipFades5ms(t *testing.T) {
	// 5 ms at 48 kHz is 240 frames. The clip reads 0.5, and its head before In reads -0.5.
	const fade = clipFadeMS * MixRate / 1000
	pcm := append(mixTestConst(100, -16384), mixTestConst(4800, 16384)...)
	in := &MixInput{
		Frames: 12000,
		Narration: []MixClip{
			{Key: "trimmed", PCM: pcm, In: 100, Len: 4800, Start: 1000, Fade: fade},
			{Key: "whole", PCM: mixTestConst(1000, 16384), Len: 1000, Start: 7000},
		},
	}
	narration, _, _ := mixTestCollect(t, in, mixPass{})
	for _, c := range []struct {
		offset int64
		want   float64
	}{
		{0, 0},
		{fade / 2, 0.25},
		{fade, 0.5},
		{2400, 0.5},
		{4799 - fade/2, 0.25},
		{4799, 0},
	} {
		if got := narration[(1000+c.offset)*mixChannels]; math.Abs(got-c.want) > 1e-9 {
			t.Errorf("offset %d = %.6f, want %.6f", c.offset, got, c.want)
		}
	}
	if got := narration[7000*mixChannels]; got != 0.5 {
		t.Errorf("a clip without Fade starts at %.6f, want 0.5", got)
	}
}

func TestMixDropsAClipPastTheFilmEnd(t *testing.T) {
	// The path does not exist, so opening this clip would fail the pass.
	in := &MixInput{
		Frames: 9600,
		Narration: []MixClip{
			{Key: "late", Path: filepath.Join(t.TempDir(), "missing.raw"), Len: 4800, Start: 9600},
			{Key: "cut", PCM: mixTestConst(4800, 16384), Len: 4800, Start: 7200},
		},
	}
	narration, _, _ := mixTestCollect(t, in, mixPass{})
	if len(narration) != 9600*mixChannels {
		t.Fatalf("the pass gave %d samples", len(narration))
	}
	// 9600 - 7200 = 2400 frames of the second clip remain.
	usable := usableClips(in.Narration, in.Frames)
	if len(usable) != 1 || usable[0].Key != "cut" || usable[0].Len != 2400 {
		t.Fatalf("usable clips = %+v", usable)
	}
	// The cut edge is a trim, so it fades to 0 on the last frame.
	if got := narration[9599*mixChannels]; got != 0 {
		t.Fatalf("the last frame of the cut clip = %.6f, want 0", got)
	}
}

func TestMixBedGainAndFades(t *testing.T) {
	// 4 s of a constant 0.5, at -6.02 dB, so the flat part reads 0.25. The fade in is 0.5 s
	// (24000 frames) and the fade out 1.5 s (72000 frames), so their midpoints read 0.125.
	const frames = 4 * MixRate
	path := filepath.Join(t.TempDir(), "bed.raw")
	mixTestWriteRaw(t, path, mixTestConst(frames, 16384))
	in := &MixInput{Frames: frames, Bed: &MixBed{Path: path, Frames: frames, Volume: 0.25}}
	_, music, _ := mixTestCollect(t, in, mixPass{bedGainDB: 20 * math.Log10(0.5)})
	for _, c := range []struct {
		n    int64
		want float64
	}{
		{0, 0},
		{12000, 0.125},
		{60000, 0.25},
		{frames - 36000, 0.125},
	} {
		if got := music[c.n*mixChannels]; math.Abs(got-c.want) > 1e-9 {
			t.Errorf("frame %d = %.6f, want %.6f", c.n, got, c.want)
		}
	}
}

func TestMixPassStopsAtABlockBoundaryOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	blocks := 0
	err := runMixPass(ctx, &MixInput{Frames: mixBlockFrames * 10}, mixPass{}, func(b *mixBlock) error {
		blocks++
		if blocks == 3 {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if blocks != 3 {
		t.Fatalf("%d blocks ran, want the 3 before the cancel", blocks)
	}
}
```

- [ ] **Step 2: Run it**

In `.worktrees/storymaker-p2c`:

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/render -run '(?i)mix' -count=1
```

Expected: the test package does not build, with `undefined: mixChannels`, `undefined: runMixPass`, `undefined: newStemWriter`, because `mix.go` does not exist yet.

- [ ] **Step 3: Implement**

Create `internal/storymaker/render/mix.go`:

```go
package render

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"sort"
)

const (
	MixRate          = 48000
	mixChannels      = 2
	mixBlockFrames   = 4800
	mixTargetLUFS    = -16.0
	mixCeilingDBTP   = -1.0
	mixStemGuardDBFS = -0.1
	bedBelowNarrLU   = -12.0
	bedNoNarrLUFS    = -18.0
	bedMaxGainDB     = 24.0
	voiceMatchCapDB  = 6.0
	duckDepthDB      = 6.0
	duckAttackMS     = 50.0
	duckReleaseMS    = 400.0
	duckLookaheadMS  = 50.0
	duckThresholdDB  = -45.0
	duckPeakDecayMS  = 20.0
	carveHz          = 2200.0
	carveQ           = 1.0
	carveDB          = -5.0
	carveRampMS      = 20.0
	dipAttackFrames  = 480
	dipHoldFrames    = 3840
	dipReturnFrames  = 14400
	loopXfadeS       = 2.0
	musicFadeInS     = 0.5
	musicFadeOutS    = 1.5
	clipFadeMS       = 5

	clipFadeFrames   = clipFadeMS * MixRate / 1000
	mixBytesPerFrame = mixChannels * 2
	mixFullScale     = 32768.0
	pcmWindowFrames  = 1 << 16
	wavHeaderBytes   = 44
)

// Span is a stretch of speech in film frames, half open.
type Span struct{ From, To int64 }

// Impact is a sound hit that dips the bed.
type Impact struct {
	At      int64
	DepthDB float64
}

// MixClip is one narration piece or one sound effect on the film timeline.
type MixClip struct {
	Key           string
	Path          string  // raw s16le 48 kHz stereo file in staging; "" when PCM is set
	PCM           []int16 // interleaved stereo, SFX read once into memory
	In            int64   // used range start in frames
	Len           int64   // used length in frames
	Start         int64   // film position in frames
	GainDB        float64 // narration: written by the voice match; sfx: empty on input, written in pass 2
	LevelOffsetDB float64 // sfx only: over_db - take.l50_lufs, over_db 0 without a hit
	Fade          int64   // frames faded at both edges when the piece was trimmed
	Voice         string  // provider|voice, narration only
}

// MixBed is the music under the film. Path is a raw s16le 48 kHz stereo file in staging.
type MixBed struct {
	Path   string
	Frames int64
	Volume float64 // above 0; volume 0 never reaches the mixer
}

// MixInput is everything one mix needs.
type MixInput struct {
	Frames    int64 // round(frame_count*48000/fps)
	Narration []MixClip
	SFX       []MixClip
	Bed       *MixBed
	Spans     []Span
	Impacts   []Impact
}

// MixOutputs names the WAV files of a mix. Mix is required; "" skips a stem.
type MixOutputs struct{ Mix, Narration, Music, SFX string }

// MixResult reports what a mix measured and applied. A silent reading is stored as -70, so the
// result stays valid JSON.
type MixResult struct {
	NarrationI, BedI, PreI, PreTP, BedGainDB, MasterGainDB float64
	VoiceGainDB                                            map[string]float64
	StemPeak                                               [3]float64 // narration, music, sfx after the master gain, linear
	Clamps                                                 int64
	BedDropped                                             bool // the bed read silent
}

// mixPass selects what one run over the film computes.
type mixPass struct {
	bedGainDB float64
	masterDB  float64
	rawBed    bool // pass 1: narration and the bare loop at unity, no sfx
}

// mixBlock is one block of a pass: interleaved stems as floats, after all gains.
type mixBlock struct {
	frames                int
	narration, music, sfx []float64
}

func dbToLin(db float64) float64 { return math.Pow(10, db/20) }

// mixIOError keeps the cause of a file error and drops the staging path from its text.
func mixIOError(what string, err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		err = pathErr.Err
	}
	return fmt.Errorf("storymaker: %s: %w", what, err)
}

// runMixPass streams the film through the graph and hands each block to sink. It stops on ctx at
// block boundaries. The narration of the next block is kept ready, because the duck looks ahead.
func runMixPass(ctx context.Context, in *MixInput, pass mixPass, sink func(*mixBlock) error) error {
	if in.Frames <= 0 {
		return nil
	}
	size := mixBlockFrames * mixChannels
	cur, next := make([]float64, size), make([]float64, size)
	narration, music, sfx := make([]float64, size), make([]float64, size), make([]float64, size)

	narrClips := newClipMixer(usableClips(in.Narration, in.Frames))
	sfxClips := newClipMixer(usableClips(in.SFX, in.Frames))
	var loop *bedLoop
	if in.Bed != nil {
		var err error
		if loop, err = openBedLoop(in.Bed, in.Frames); err != nil {
			return err
		}
		defer loop.close()
	}
	shape := newBedShape(in, pass)
	master := dbToLin(pass.masterDB)

	if err := narrClips.fill(0, int(min(mixBlockFrames, in.Frames)), next); err != nil {
		return err
	}
	block := &mixBlock{}
	for n0 := int64(0); n0 < in.Frames; n0 += mixBlockFrames {
		if err := ctx.Err(); err != nil {
			return err
		}
		f := int(min(mixBlockFrames, in.Frames-n0))
		samples := f * mixChannels
		cur, next = next, cur
		clear(next)
		if rest := in.Frames - n0 - mixBlockFrames; rest > 0 {
			if err := narrClips.fill(n0+mixBlockFrames, int(min(mixBlockFrames, rest)), next); err != nil {
				return err
			}
		}
		if pass.rawBed {
			clear(sfx[:samples])
		} else if err := sfxClips.fill(n0, f, sfx); err != nil {
			return err
		}
		clear(music[:samples])
		if loop != nil {
			if err := loop.fill(n0, f, music); err != nil {
				return err
			}
			if !pass.rawBed {
				shape.apply(n0, f, cur, next, music)
			}
		}
		for i := 0; i < samples; i++ {
			narration[i] = cur[i] * master
			music[i] *= master
			sfx[i] *= master
		}
		block.frames = f
		block.narration, block.music, block.sfx = narration[:samples], music[:samples], sfx[:samples]
		if err := sink(block); err != nil {
			return err
		}
	}
	return nil
}

// usableClips drops clips that never sound, truncates at the film end and sorts by start. A cut edge
// is a trim, so it gets the clip fade.
func usableClips(clips []MixClip, frames int64) []MixClip {
	out := make([]MixClip, 0, len(clips))
	for _, c := range clips {
		if c.Len <= 0 || c.Start >= frames {
			continue
		}
		if c.Start+c.Len > frames {
			c.Len = frames - c.Start
			if c.Fade < clipFadeFrames {
				c.Fade = clipFadeFrames
			}
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

// loadClipPCM returns the used range of a clip. FFmpeg may decode a few frames fewer than the plan
// measured, so a short file gives a short clip rather than an error.
func loadClipPCM(c MixClip) ([]int16, error) {
	if c.Path == "" {
		last := (c.In + c.Len) * mixChannels
		if c.In < 0 || last > int64(len(c.PCM)) {
			return nil, fmt.Errorf("storymaker: the clip %s is outside its samples", c.Key)
		}
		return c.PCM[c.In*mixChannels : last], nil
	}
	f, err := os.Open(c.Path)
	if err != nil {
		return nil, mixIOError("open a decoded clip", err)
	}
	defer f.Close()
	raw := make([]byte, c.Len*mixBytesPerFrame)
	n, err := f.ReadAt(raw, c.In*mixBytesPerFrame)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, mixIOError("read a decoded clip", err)
	}
	pcm := make([]int16, n/mixBytesPerFrame*mixChannels)
	for i := range pcm {
		pcm[i] = int16(binary.LittleEndian.Uint16(raw[i*2:]))
	}
	return pcm, nil
}

// clipMixer opens clips when they start and drops them when they end.
type clipMixer struct {
	clips  []MixClip
	next   int
	active []*activeClip
}

type activeClip struct {
	data   []int16
	start  int64
	end    int64
	length int64
	fade   int64
	gain   float64
}

func newClipMixer(clips []MixClip) *clipMixer { return &clipMixer{clips: clips} }

// fill writes the sum of the clips into out for the frames n0 up to n0+f.
func (m *clipMixer) fill(n0 int64, f int, out []float64) error {
	clear(out[:f*mixChannels])
	n1 := n0 + int64(f)
	for m.next < len(m.clips) && m.clips[m.next].Start < n1 {
		c := m.clips[m.next]
		m.next++
		data, err := loadClipPCM(c)
		if err != nil {
			return err
		}
		length := int64(len(data) / mixChannels)
		m.active = append(m.active, &activeClip{
			data:   data,
			start:  c.Start,
			end:    c.Start + length,
			length: length,
			fade:   min(c.Fade, length/2),
			gain:   dbToLin(c.GainDB),
		})
	}
	keep := m.active[:0]
	for _, a := range m.active {
		a.mixInto(n0, n1, out)
		if a.end > n1 {
			keep = append(keep, a)
		}
	}
	clear(m.active[len(keep):])
	m.active = keep
	return nil
}

func (a *activeClip) mixInto(n0, n1 int64, out []float64) {
	for t := max(a.start, n0); t < min(a.end, n1); t++ {
		i := t - a.start
		g := a.gain * clipEdge(i, a.length, a.fade) / mixFullScale
		o := (t - n0) * mixChannels
		out[o] += float64(a.data[i*mixChannels]) * g
		out[o+1] += float64(a.data[i*mixChannels+1]) * g
	}
}

// clipEdge is the linear fade at both edges of a clip: 0 on its first and last frame.
func clipEdge(i, length, fade int64) float64 {
	if fade <= 0 {
		return 1
	}
	if i < fade {
		return float64(i) / float64(fade)
	}
	if tail := length - 1 - i; tail < fade {
		return float64(tail) / float64(fade)
	}
	return 1
}

// pcmWindow reads a raw stereo file through a sequential read-ahead window.
type pcmWindow struct {
	f      *os.File
	frames int64
	base   int64
	count  int64
	raw    []byte
	pcm    []int16
}

func openPCMWindow(path string) (*pcmWindow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, mixIOError("open the music", err)
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, mixIOError("read the music", err)
	}
	return &pcmWindow{
		f:      f,
		frames: st.Size() / mixBytesPerFrame,
		raw:    make([]byte, pcmWindowFrames*mixBytesPerFrame),
		pcm:    make([]int16, pcmWindowFrames*mixChannels),
	}, nil
}

func (w *pcmWindow) load(frame int64) error {
	count := min(pcmWindowFrames, w.frames-frame)
	if frame < 0 || count <= 0 {
		return errors.New("storymaker: the music ended early")
	}
	raw := w.raw[:count*mixBytesPerFrame]
	if _, err := w.f.ReadAt(raw, frame*mixBytesPerFrame); err != nil {
		return mixIOError("read the music", err)
	}
	for i := int64(0); i < count*mixChannels; i++ {
		w.pcm[i] = int16(binary.LittleEndian.Uint16(raw[i*2:]))
	}
	w.base, w.count = frame, count
	return nil
}

func (w *pcmWindow) at(frame int64) (float64, float64, error) {
	if frame < w.base || frame >= w.base+w.count {
		if err := w.load(frame); err != nil {
			return 0, 0, err
		}
	}
	i := (frame - w.base) * mixChannels
	return float64(w.pcm[i]) / mixFullScale, float64(w.pcm[i+1]) / mixFullScale, nil
}

// bedLoop streams the bare looped bed through two read windows, because one window thrashes in the
// crossfade.
type bedLoop struct {
	main, tail    *pcmWindow
	period, xfade int64
}

// loopShape returns the loop period and the crossfade, both in frames. The crossfade is at most half
// the bed. A bed as long as the film never loops, so it needs no crossfade.
func loopShape(bedFrames, film int64) (period, xfade int64) {
	if film <= bedFrames {
		return bedFrames, 0
	}
	xfade = min(int64(loopXfadeS*MixRate), bedFrames/2)
	return bedFrames - xfade, xfade
}

func openBedLoop(bed *MixBed, film int64) (*bedLoop, error) {
	main, err := openPCMWindow(bed.Path)
	if err != nil {
		return nil, err
	}
	tail, err := openPCMWindow(bed.Path)
	if err != nil {
		_ = main.f.Close()
		return nil, err
	}
	frames := min(bed.Frames, main.frames)
	if frames <= 0 {
		_ = main.f.Close()
		_ = tail.f.Close()
		return nil, errors.New("storymaker: the music file is empty")
	}
	period, xfade := loopShape(frames, film)
	return &bedLoop{main: main, tail: tail, period: period, xfade: xfade}, nil
}

func (b *bedLoop) close() {
	_ = b.main.f.Close()
	_ = b.tail.f.Close()
}

// fill writes the looped bed into out: an equal-power crossfade from the bed's tail to its head.
func (b *bedLoop) fill(n0 int64, f int, out []float64) error {
	for i := 0; i < f; i++ {
		n := n0 + int64(i)
		pos := n % b.period
		l, r, err := b.main.at(pos)
		if err != nil {
			return err
		}
		if n >= b.period && pos < b.xfade {
			tailL, tailR, err := b.tail.at(pos + b.period)
			if err != nil {
				return err
			}
			u := float64(pos) / float64(b.xfade) * math.Pi / 2
			rise, fall := math.Sin(u), math.Cos(u)
			l, r = l*rise+tailL*fall, r*rise+tailR*fall
		}
		out[i*mixChannels], out[i*mixChannels+1] = l, r
	}
	return nil
}

// bedShape turns the raw loop into the bed stem: the gain and the fades.
type bedShape struct {
	gain    float64
	film    int64
	fadeIn  int64
	fadeOut int64
}

func newBedShape(in *MixInput, pass mixPass) *bedShape {
	return &bedShape{
		gain:    dbToLin(pass.bedGainDB),
		film:    in.Frames,
		fadeIn:  int64(musicFadeInS * MixRate),
		fadeOut: int64(musicFadeOutS * MixRate),
	}
}

func (s *bedShape) fade(n int64) float64 {
	fade := 1.0
	if n < s.fadeIn {
		fade = float64(n) / float64(s.fadeIn)
	}
	if left := s.film - n; left < s.fadeOut {
		fade = math.Min(fade, float64(left)/float64(s.fadeOut))
	}
	return fade
}

// apply shapes one block of the raw loop in place.
func (s *bedShape) apply(n0 int64, f int, cur, next, bed []float64) {
	for i := 0; i < f; i++ {
		g := s.gain * s.fade(n0+int64(i))
		bed[i*mixChannels] *= g
		bed[i*mixChannels+1] *= g
	}
}

// wavWriter writes a canonical 48 kHz stereo s16 WAV. close writes the header.
type wavWriter struct {
	f      *os.File
	bw     *bufio.Writer
	frames int64
	buf    []byte
}

func newWavWriter(path string) (*wavWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, mixIOError("create a mix file", err)
	}
	w := &wavWriter{f: f, bw: bufio.NewWriterSize(f, 1<<20), buf: make([]byte, mixBlockFrames*mixBytesPerFrame)}
	if _, err := w.bw.Write(make([]byte, wavHeaderBytes)); err != nil {
		_ = f.Close()
		return nil, mixIOError("write a mix file", err)
	}
	return w, nil
}

func (w *wavWriter) write(samples []int16) error {
	b := w.buf[:len(samples)*2]
	for i, v := range samples {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(v))
	}
	w.frames += int64(len(samples) / mixChannels)
	if _, err := w.bw.Write(b); err != nil {
		return mixIOError("write a mix file", err)
	}
	return nil
}

// close is safe on a nil writer and safe to call twice.
func (w *wavWriter) close() error {
	if w == nil || w.f == nil {
		return nil
	}
	f := w.f
	w.f = nil
	err := w.bw.Flush()
	if err == nil {
		_, err = f.WriteAt(wavHeader(w.frames), 0)
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return mixIOError("finish a mix file", err)
	}
	return nil
}

func wavHeader(frames int64) []byte {
	data := uint32(frames * mixBytesPerFrame)
	h := make([]byte, wavHeaderBytes)
	copy(h[0:], "RIFF")
	binary.LittleEndian.PutUint32(h[4:], 36+data)
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1)
	binary.LittleEndian.PutUint16(h[22:], mixChannels)
	binary.LittleEndian.PutUint32(h[24:], MixRate)
	binary.LittleEndian.PutUint32(h[28:], MixRate*mixBytesPerFrame)
	binary.LittleEndian.PutUint16(h[32:], mixBytesPerFrame)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], data)
	return h
}

func quantiseS16(v float64) (int16, bool) {
	r := math.Round(v * mixFullScale)
	switch {
	case r > math.MaxInt16:
		return math.MaxInt16, true
	case r < math.MinInt16:
		return math.MinInt16, true
	}
	return int16(r), false
}

// stemWriter quantises each stem after the gains and writes the mix as the clamp of their integer
// sum, so the stems sum to the mix exactly.
type stemWriter struct {
	mix    *wavWriter
	stems  [3]*wavWriter
	pcm    [3][]int16
	mixPCM []int16
	peak   [3]float64
	clamps int64
}

func newStemWriter(out MixOutputs) (*stemWriter, error) {
	size := mixBlockFrames * mixChannels
	w := &stemWriter{mixPCM: make([]int16, size)}
	for k := range w.pcm {
		w.pcm[k] = make([]int16, size)
	}
	var err error
	if w.mix, err = newWavWriter(out.Mix); err != nil {
		return nil, err
	}
	for k, path := range [3]string{out.Narration, out.Music, out.SFX} {
		if path == "" {
			continue
		}
		if w.stems[k], err = newWavWriter(path); err != nil {
			_ = w.close()
			return nil, err
		}
	}
	return w, nil
}

func (w *stemWriter) write(b *mixBlock) error {
	samples := b.frames * mixChannels
	for i := 0; i < samples; i++ {
		sum := 0
		for k, v := range [3]float64{b.narration[i], b.music[i], b.sfx[i]} {
			if a := math.Abs(v); a > w.peak[k] {
				w.peak[k] = a
			}
			q, clamped := quantiseS16(v)
			if clamped {
				w.clamps++
			}
			w.pcm[k][i] = q
			sum += int(q)
		}
		if sum > math.MaxInt16 {
			sum = math.MaxInt16
			w.clamps++
		} else if sum < math.MinInt16 {
			sum = math.MinInt16
			w.clamps++
		}
		w.mixPCM[i] = int16(sum)
	}
	if err := w.mix.write(w.mixPCM[:samples]); err != nil {
		return err
	}
	for k, stem := range w.stems {
		if stem == nil {
			continue
		}
		if err := stem.write(w.pcm[k][:samples]); err != nil {
			return err
		}
	}
	return nil
}

// close finishes every file and returns the first error. It is safe to call twice.
func (w *stemWriter) close() error {
	err := w.mix.close()
	for _, stem := range w.stems {
		if closeErr := stem.close(); err == nil {
			err = closeErr
		}
	}
	return err
}
```

- [ ] **Step 4: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
gofmt -w internal/storymaker/render/mix.go internal/storymaker/render/mix_test.go
go vet ./internal/storymaker/render
go test ./internal/storymaker/render -run '(?i)mix' -count=1 -v
```

Expected: `--- PASS` for `TestMixStemsSumToTheMix`, `TestMixLoopCrossfadeIsEqualPowerAndAtMostHalfTheBed`, `TestMixTrimmedClipFades5ms`, `TestMixDropsAClipPastTheFilmEnd`, `TestMixBedGainAndFades` and `TestMixPassStopsAtABlockBoundaryOnCancel`.

- [ ] **Step 5: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git grep -n "runMixPass" -- internal
git add internal/storymaker/render/mix.go internal/storymaker/render/mix_test.go
git commit -m "feat(storymaker): streaming mix core with exact stems" -m "detect_changes answered PARTIAL on the new files; grep finds runMixPass only in mix.go and mix_test.go." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Drop the middle `-m` if `detect-changes` answers without `partial`. If the tool crashes, closes the connection, answers `not found`, or prints only its banner, use `graft` callers plus grep and say so in that line instead.

### Task A12: Ducking, carve, impact dips
**Batch:** 2C  **Depends on:** A11
**Files:**
- Modify: `internal/storymaker/render/mix.go`
- Test: `internal/storymaker/render/mix_test.go`

- [ ] **Step 1: Impact**

From the main checkout, for each of `bedShape`, `newBedShape` and `apply`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "bedShape" --direction upstream --repo .
```

Report callers, processes and risk. Warn on HIGH or CRITICAL. These symbols are new in this batch and the index is stale, so `UNKNOWN` is likely: apply the rule of outline section 5 and confirm with `git grep -n "bedShape" -- internal/storymaker` in the worktree. Only `runMixPass` may call them.

- [ ] **Step 2: Write the failing test**

Append to `internal/storymaker/render/mix_test.go`. No new imports are needed.

```go
// mixTestConstBed builds an input whose bed is a constant 0.5, so the music stem reads the gain directly.
func mixTestConstBed(t *testing.T, frames int64) *MixInput {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bed.raw")
	mixTestWriteRaw(t, path, mixTestConst(int(frames), 16384))
	return &MixInput{Frames: frames, Bed: &MixBed{Path: path, Frames: frames, Volume: 0.25}}
}

func mixTestAssertGainDB(t *testing.T, music []float64, n int64, want float64) {
	t.Helper()
	got := 20 * math.Log10(music[n*mixChannels]/0.5)
	if math.Abs(got-want) > 0.02 {
		t.Errorf("gain at frame %d = %.3f dB, want %.3f", n, got, want)
	}
}

// mixTestRMSDB is the RMS level of the left channel over the frames from up to to.
func mixTestRMSDB(stem []float64, from, to int64) float64 {
	sum := 0.0
	for n := from; n < to; n++ {
		v := stem[n*mixChannels]
		sum += v * v
	}
	return 10 * math.Log10(sum/float64(to-from))
}

func TestMixDuckReachesDepthAtSpeechAndReleasesOver400ms(t *testing.T) {
	const (
		speechStart = int64(3 * MixRate)
		speechLen   = int64(MixRate)
		lookahead   = int64(duckLookaheadMS * MixRate / 1000) // 50 ms is 2400 frames
		release     = int64(duckReleaseMS * MixRate / 1000)   // 400 ms is 19200 frames
		decayFrames = duckPeakDecayMS * MixRate / 1000        // 20 ms is 960 frames
	)
	// The film is 6 s, so the bed is flat from 0.5 s to 6 - 1.5 = 4.5 s, past the end of the release.
	in := mixTestConstBed(t, 6*MixRate)
	in.Narration = []MixClip{{Key: "speech", PCM: mixTestConst(MixRate, 16384), Len: speechLen, Start: speechStart}}
	_, music, _ := mixTestCollect(t, in, mixPass{})

	// The detector sees speech 50 ms early. The attack is a 50 ms ramp, so it ends as speech starts.
	duckStart := speechStart - lookahead
	mixTestAssertGainDB(t, music, duckStart-1, 0)
	mixTestAssertGainDB(t, music, duckStart+lookahead/2-1, -3)
	mixTestAssertGainDB(t, music, speechStart-1, -6)
	mixTestAssertGainDB(t, music, speechStart, -6)

	// The detector sees the last speech sample 50 ms early. Its peak then falls from 0.5 by
	// exp(-k/960) and stays above the -45 dB threshold for hold more frames. Then the 400 ms ramp runs.
	lastSeen := speechStart + speechLen - 1 - lookahead
	hold := int64(decayFrames * math.Log(0.5/dbToLin(duckThresholdDB)))
	releaseStart := lastSeen + hold + 1
	mixTestAssertGainDB(t, music, releaseStart-1, -6)
	mixTestAssertGainDB(t, music, releaseStart+release/2-1, -3)
	mixTestAssertGainDB(t, music, releaseStart+release-1, 0)
	mixTestAssertGainDB(t, music, releaseStart+release+100, 0)
}

func TestMixCarveCutsOnlyInsideSpans(t *testing.T) {
	// The film is 4 s, so the bed is flat from 24000 to 192000 - 72000 = 120000.
	const frames = int64(4 * MixRate)
	path := filepath.Join(t.TempDir(), "bed.raw")
	mixTestWriteRaw(t, path, mixTestSine(int(frames), 0.25, carveHz))
	in := &MixInput{
		Frames: frames,
		Bed:    &MixBed{Path: path, Frames: frames, Volume: 0.25},
		Spans:  []Span{{From: 60000, To: 72000}},
	}
	_, music, _ := mixTestCollect(t, in, mixPass{})

	// 4800 frames hold 220 whole cycles of 2.2 kHz. The RMS of a sine of amplitude a is a/sqrt(2).
	// A peaking filter has exactly its gain at its centre, so inside the span the level is 5 dB lower.
	// The ramps reach 20 ms (960 frames) past each edge, and these windows stay clear of them.
	full := 20 * math.Log10(0.25/math.Sqrt2)
	for _, c := range []struct {
		name string
		from int64
		want float64
	}{
		{"before the span", 40000, full},
		{"inside the span", 62000, full + carveDB},
		{"after the span", 90000, full},
	} {
		if got := mixTestRMSDB(music, c.from, c.from+4800); math.Abs(got-c.want) > 0.05 {
			t.Errorf("%s: %.3f dB, want %.3f", c.name, got, c.want)
		}
	}
}

func TestMixImpactDipFollowsAttackHoldReturn(t *testing.T) {
	// 10 ms is 480 frames, 80 ms is 3840 frames and 0.3 s is 14400 frames. 60000 + 18720 stays inside
	// the flat part of a 4 s film, which ends at 120000.
	const at = int64(60000)
	in := mixTestConstBed(t, 4*MixRate)
	in.Impacts = []Impact{{At: at, DepthDB: 3}}
	_, music, _ := mixTestCollect(t, in, mixPass{})

	mixTestAssertGainDB(t, music, at-1, 0)
	mixTestAssertGainDB(t, music, at+dipAttackFrames/2, -1.5)
	mixTestAssertGainDB(t, music, at+dipAttackFrames, -3)
	mixTestAssertGainDB(t, music, at+dipAttackFrames+dipHoldFrames-1, -3)
	mixTestAssertGainDB(t, music, at+dipAttackFrames+dipHoldFrames+dipReturnFrames/2, -1.5)
	mixTestAssertGainDB(t, music, at+dipAttackFrames+dipHoldFrames+dipReturnFrames, 0)
}
```

- [ ] **Step 3: Run it**

In `.worktrees/storymaker-p2c`:

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/render -run '(?i)(MixDuck|MixCarve|MixImpactDip)' -count=1
```

Expected: three failures, `TestMixDuckReachesDepthAtSpeechAndReleasesOver400ms`, `TestMixCarveCutsOnlyInsideSpans` and `TestMixImpactDipFollowsAttackHoldReturn`, because `bedShape` applies only the gain and the fades. So the bed reads 0 dB where the tests expect -6, -5 and -3 dB.

- [ ] **Step 4: Implement**

In `internal/storymaker/render/mix.go`, as Task A11 left it, replace

```go
// bedShape turns the raw loop into the bed stem: the gain and the fades.
type bedShape struct {
	gain    float64
	film    int64
	fadeIn  int64
	fadeOut int64
}

func newBedShape(in *MixInput, pass mixPass) *bedShape {
	return &bedShape{
		gain:    dbToLin(pass.bedGainDB),
		film:    in.Frames,
		fadeIn:  int64(musicFadeInS * MixRate),
		fadeOut: int64(musicFadeOutS * MixRate),
	}
}
```

with

```go
// biquad is a stateful direct-form-I filter.
type biquad struct {
	b0, b1, b2, a1, a2 float64
	x1, x2, y1, y2     float64
}

// peakingBiquad is the RBJ peaking filter at the mix rate.
func peakingBiquad(hz, q, gainDB float64) biquad {
	a := math.Pow(10, gainDB/40)
	w := 2 * math.Pi * hz / MixRate
	alpha := math.Sin(w) / (2 * q)
	a0 := 1 + alpha/a
	return biquad{
		b0: (1 + alpha*a) / a0,
		b1: -2 * math.Cos(w) / a0,
		b2: (1 - alpha*a) / a0,
		a1: -2 * math.Cos(w) / a0,
		a2: (1 - alpha/a) / a0,
	}
}

func (b *biquad) run(x float64) float64 {
	y := b.b0*x + b.b1*b.x1 + b.b2*b.x2 - b.a1*b.y1 - b.a2*b.y2
	b.x2, b.x1, b.y2, b.y1 = b.x1, x, b.y1, y
	return y
}

// dipGainDB is the impact dip in dB, dt frames after the hit.
func dipGainDB(dt int64, depth float64) float64 {
	switch {
	case dt < 0:
		return 0
	case dt < dipAttackFrames:
		return -depth * float64(dt) / dipAttackFrames
	case dt < dipAttackFrames+dipHoldFrames:
		return -depth
	case dt < dipAttackFrames+dipHoldFrames+dipReturnFrames:
		return -depth * (1 - float64(dt-dipAttackFrames-dipHoldFrames)/dipReturnFrames)
	}
	return 0
}

// bedShape turns the raw loop into the bed stem: the gain to the reference and the fades, the duck
// keyed by the narration, the carve inside the speech spans, and the impact dips.
type bedShape struct {
	gain    float64
	film    int64
	fadeIn  int64
	fadeOut int64

	lookahead   int
	attackStep  float64
	releaseStep float64
	peakDecay   float64
	threshold   float64
	peak        float64
	duck        float64

	carve    [mixChannels]biquad
	spans    []Span
	nextSpan int
	rampLead int64
	rampStep float64
	wet      float64

	impacts    []Impact
	nextImpact int
	activeHits []Impact
}

func newBedShape(in *MixInput, pass mixPass) *bedShape {
	spans := append([]Span(nil), in.Spans...)
	sort.Slice(spans, func(i, j int) bool { return spans[i].From < spans[j].From })
	impacts := append([]Impact(nil), in.Impacts...)
	sort.Slice(impacts, func(i, j int) bool { return impacts[i].At < impacts[j].At })
	carve := peakingBiquad(carveHz, carveQ, carveDB)
	return &bedShape{
		gain:        dbToLin(pass.bedGainDB),
		film:        in.Frames,
		fadeIn:      int64(musicFadeInS * MixRate),
		fadeOut:     int64(musicFadeOutS * MixRate),
		lookahead:   int(duckLookaheadMS * MixRate / 1000),
		attackStep:  1 / (duckAttackMS * MixRate / 1000),
		releaseStep: 1 / (duckReleaseMS * MixRate / 1000),
		peakDecay:   math.Exp(-1 / (duckPeakDecayMS * MixRate / 1000)),
		threshold:   dbToLin(duckThresholdDB),
		carve:       [mixChannels]biquad{carve, carve},
		spans:       spans,
		rampLead:    int64(carveRampMS * MixRate / 1000),
		rampStep:    1 / (carveRampMS * MixRate / 1000),
		impacts:     impacts,
	}
}
```

Then replace

```go
// apply shapes one block of the raw loop in place.
func (s *bedShape) apply(n0 int64, f int, cur, next, bed []float64) {
	for i := 0; i < f; i++ {
		g := s.gain * s.fade(n0+int64(i))
		bed[i*mixChannels] *= g
		bed[i*mixChannels+1] *= g
	}
}
```

with

```go
// apply shapes one block of the raw loop in place. cur and next are the narration of this block and
// the next one, before the master gain, so the duck is the same in pass 2 and pass 3.
func (s *bedShape) apply(n0 int64, f int, cur, next, bed []float64) {
	for i := 0; i < f; i++ {
		n := n0 + int64(i)
		duckDB := s.stepDuck(i, f, cur, next)
		dipDB := s.stepDip(n)
		wet := s.stepCarve(n)
		g := s.gain * s.fade(n) * dbToLin(duckDB+dipDB)
		for ch := 0; ch < mixChannels; ch++ {
			x := bed[i*mixChannels+ch]
			y := s.carve[ch].run(x)
			bed[i*mixChannels+ch] = (x + wet*(y-x)) * g
		}
	}
}

// stepDuck is the duck in dB for one frame. The detector reads the narration 50 ms ahead and holds its
// peak with a 20 ms decay. Attack and release are linear ramps in dB, so the duck starts 50 ms before
// speech, is at full depth when speech starts, and returns over exactly 400 ms.
func (s *bedShape) stepDuck(i, f int, cur, next []float64) float64 {
	j := i + s.lookahead
	var l, r float64
	if j < f {
		l, r = cur[j*mixChannels], cur[j*mixChannels+1]
	} else if j-f < mixBlockFrames {
		l, r = next[(j-f)*mixChannels], next[(j-f)*mixChannels+1]
	}
	s.peak = math.Max(math.Abs(l+r)*0.5, s.peak*s.peakDecay)
	if s.peak > s.threshold {
		s.duck = math.Min(1, s.duck+s.attackStep)
	} else {
		s.duck = math.Max(0, s.duck-s.releaseStep)
	}
	return -duckDepthDB * s.duck
}

// stepCarve is the wet share of the carve for one frame: fully wet inside a span, with a 20 ms ramp
// before it and after it.
func (s *bedShape) stepCarve(n int64) float64 {
	for s.nextSpan < len(s.spans) && s.spans[s.nextSpan].To <= n {
		s.nextSpan++
	}
	if s.nextSpan < len(s.spans) && n >= s.spans[s.nextSpan].From-s.rampLead {
		s.wet = math.Min(1, s.wet+s.rampStep)
	} else {
		s.wet = math.Max(0, s.wet-s.rampStep)
	}
	return s.wet
}

// stepDip is the deepest active impact dip in dB for one frame.
func (s *bedShape) stepDip(n int64) float64 {
	for s.nextImpact < len(s.impacts) && s.impacts[s.nextImpact].At <= n {
		s.activeHits = append(s.activeHits, s.impacts[s.nextImpact])
		s.nextImpact++
	}
	deepest := 0.0
	kept := s.activeHits[:0]
	for _, hit := range s.activeHits {
		deepest = math.Min(deepest, dipGainDB(n-hit.At, hit.DepthDB))
		if n-hit.At < dipAttackFrames+dipHoldFrames+dipReturnFrames {
			kept = append(kept, hit)
		}
	}
	s.activeHits = kept
	return deepest
}
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
gofmt -w internal/storymaker/render/mix.go internal/storymaker/render/mix_test.go
go vet ./internal/storymaker/render
go test ./internal/storymaker/render -run '(?i)mix' -count=1 -v
```

Expected: `--- PASS` for the six tests of Task A11, plus `TestMixDuckReachesDepthAtSpeechAndReleasesOver400ms`, `TestMixCarveCutsOnlyInsideSpans` and `TestMixImpactDipFollowsAttackHoldReturn`. The stems still sum, because the duck, carve and dips run before quantising.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git grep -n "bedShape" -- internal
git add internal/storymaker/render/mix.go internal/storymaker/render/mix_test.go
git commit -m "feat(storymaker): duck, carve and impact dips on the bed" -m "detect_changes answered PARTIAL on the new symbols; grep finds bedShape only in mix.go." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Drop the middle `-m` if `detect-changes` answers without `partial`. If the tool crashes, closes the connection, answers `not found`, or prints only its banner, use `graft` callers plus grep and say so in that line instead.

### Task A13: Passes and gains
**Batch:** 2C  **Depends on:** A10, A12
**Files:**
- Modify: `internal/storymaker/render/mix.go`
- Test: `internal/storymaker/render/mix_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/storymaker/render/mix_test.go`, as Task A12 left it, replace

```go
import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
)
```

with

```go
import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
)
```

Then append to the same file:

```go
func mixTestNewMeter(t *testing.T) func(context.Context) (*Meter, error) {
	t.Helper()
	ffmpeg := testBinary(t, "ffmpeg")
	return func(ctx context.Context) (*Meter, error) { return StartMeter(ctx, ffmpeg) }
}

func mixTestOutputs(dir string) MixOutputs {
	return MixOutputs{
		Mix:       filepath.Join(dir, "mix.wav"),
		Narration: filepath.Join(dir, "stem-narration.wav"),
		Music:     filepath.Join(dir, "stem-music.wav"),
		SFX:       filepath.Join(dir, "stem-sfx.wav"),
	}
}

func TestMixVoiceMatchLandsTwoVoicesWithinOneDB(t *testing.T) {
	const seconds = 2 * MixRate
	// The first half of the quiet clip is silence outside the spans. If it counted, the voice would
	// read 3 dB lower and its gain would land 3 dB too high.
	quiet := append(make([]int16, seconds*mixChannels), mixTestSine(seconds, 0.4*dbToLin(-4.5), 220)...)
	quietStart := int64(seconds + MixRate/2)
	quietSpeech := quietStart + seconds
	in := &MixInput{
		Frames: quietStart + 2*seconds,
		Narration: []MixClip{
			{Key: "loud", PCM: mixTestSine(seconds, 0.4, 220), Len: seconds, Start: 0, Voice: "p|loud"},
			{Key: "quiet", PCM: quiet, Len: 2 * seconds, Start: quietStart, Voice: "p|quiet"},
		},
		Spans: []Span{{From: 0, To: seconds}, {From: quietSpeech, To: quietSpeech + seconds}},
	}
	gains, err := matchVoices(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	// gain = loudest - voice: 0 for the loud voice and 4.5 dB for the quiet one.
	if math.Abs(gains["p|loud"]) > 0.05 || math.Abs(gains["p|quiet"]-4.5) > 0.05 {
		t.Fatalf("gains = %v, want 0 and 4.5", gains)
	}
	if in.Narration[1].GainDB != gains["p|quiet"] {
		t.Fatalf("the clip holds %.3f dB, the voice %.3f dB", in.Narration[1].GainDB, gains["p|quiet"])
	}
	narration, _, _ := mixTestCollect(t, in, mixPass{})
	loudDB := mixTestRMSDB(narration, 0, seconds)
	quietDB := mixTestRMSDB(narration, quietSpeech, quietSpeech+seconds)
	if math.Abs(loudDB-quietDB) > 1 {
		t.Fatalf("the voices land at %.2f and %.2f dB", loudDB, quietDB)
	}
}

func TestMixVoiceMatchIsCappedAtSixDB(t *testing.T) {
	// 0.5 and 0.05 are 20 dB apart, above the +6 dB cap.
	in := &MixInput{
		Frames: 2 * MixRate,
		Narration: []MixClip{
			{Key: "a", PCM: mixTestSine(MixRate, 0.5, 220), Len: MixRate, Start: 0, Voice: "a"},
			{Key: "b", PCM: mixTestSine(MixRate, 0.05, 220), Len: MixRate, Start: MixRate, Voice: "b"},
		},
		Spans: []Span{{From: 0, To: 2 * MixRate}},
	}
	gains, err := matchVoices(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if gains["b"] != voiceMatchCapDB {
		t.Fatalf("gain = %.3f, want the cap", gains["b"])
	}
}

func TestMixLevels(t *testing.T) {
	silent := &MeterReading{I: math.Inf(-1), TP: math.Inf(-1), Silent: true}
	twice := 20 * math.Log10(2) // volume 0.5 is 20*log10(0.5/0.25), about +6.02 dB
	cases := []struct {
		name        string
		narrI       float64
		hasNarr     bool
		bed         *MeterReading
		volume      float64
		wantBedGain float64
		wantDropped bool
		wantSFXRef  float64
	}{
		// Reference -22-12 = -34; gain -34-(-30) = -4; the sfx reference is the actual bed level.
		{"narration and bed", -22, true, &MeterReading{I: -30}, 0.25, -4, false, -34},
		{"volume 0.5 is +6 dB", -22, true, &MeterReading{I: -30}, 0.5, -4 + twice, false, -34 + twice},
		// Reference -18 without narration; gain -18-(-30) = +12.
		{"no narration", 0, false, &MeterReading{I: -30}, 0.25, 12, false, -18},
		// No music: the sfx reference is narration I-12, or -18 without narration.
		{"no music", -22, true, nil, 0, 0, false, -34},
		{"no music, no narration", 0, false, nil, 0, 0, false, -18},
		// Gain -34-(-60) = 26 stops at +24, so the bed really sits at -60+24 = -36.
		{"quiet bed", -22, true, &MeterReading{I: -60}, 0.25, 24, false, -36},
		{"silent bed", -22, true, silent, 0.25, 0, true, -34},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := computeMixLevels(MeterReading{I: c.narrI}, c.hasNarr, c.bed, c.volume)
			if math.Abs(got.bedGainDB-c.wantBedGain) > 1e-9 || got.bedDropped != c.wantDropped || math.Abs(got.sfxRefDB-c.wantSFXRef) > 1e-9 {
				t.Fatalf("levels = %+v, want bed gain %.3f, dropped %v, sfx reference %.3f", got, c.wantBedGain, c.wantDropped, c.wantSFXRef)
			}
		})
	}
}

func TestMixMasterGainIsTheMinOfThreeBounds(t *testing.T) {
	cases := []struct {
		name string
		pre  MeterReading
		peak float64
		want float64
	}{
		// Loudness -16-(-18) = 2; peak -1-(-10) = 9; stem -0.1-(-20) = 19.9.
		{"loudness binds", MeterReading{I: -18, TP: -10}, 0.1, 2},
		// Loudness -16-(-20) = 4; peak -1-(-3) = 2; stem -0.1+6.02 = 5.92.
		{"true peak binds", MeterReading{I: -20, TP: -3}, 0.5, 2},
		// Loudness 14; peak 19; stem -0.1-20*log10(0.99).
		{"stem peak binds", MeterReading{I: -30, TP: -20}, 0.99, -0.1 - 20*math.Log10(0.99)},
		{"silent", MeterReading{I: math.Inf(-1), TP: math.Inf(-1), Silent: true}, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := masterGainDB(c.pre, c.peak); math.Abs(got-c.want) > 1e-9 {
				t.Fatalf("G = %.4f, want %.4f", got, c.want)
			}
		})
	}
}

func TestMixPassesApplyTheGains(t *testing.T) {
	newMeter := mixTestNewMeter(t)
	dir := t.TempDir()
	clipPath := filepath.Join(dir, "piece.raw")
	bedPath := filepath.Join(dir, "bed.raw")
	mixTestWriteRaw(t, clipPath, mixTestSine(4*MixRate, 0.3, 440))
	mixTestWriteRaw(t, bedPath, mixTestSine(5*MixRate/2, 0.2, 110))
	const offset = 22.5 // over_db 4.5 minus l50_lufs -18, the pop of the SFX set
	frames := int64(6 * MixRate)
	in := &MixInput{
		Frames:    frames,
		Narration: []MixClip{{Key: "piece", Path: clipPath, Len: 4 * MixRate, Start: MixRate, Voice: "p|v"}},
		SFX:       []MixClip{{Key: "sfx.pop", PCM: mixTestSine(MixRate/5, 0.2, 880), Len: MixRate / 5, Start: 3 * MixRate, LevelOffsetDB: offset}},
		Bed:       &MixBed{Path: bedPath, Frames: 5 * MixRate / 2, Volume: 0.5},
		Spans:     []Span{{From: MixRate, To: 5 * MixRate}},
		Impacts:   []Impact{{At: 3 * MixRate, DepthDB: 3}},
	}
	out := mixTestOutputs(dir)
	last := 0.0
	res, err := Mix(context.Background(), in, newMeter, out, func(p float64) {
		if p < last {
			t.Errorf("progress went back from %.3f to %.3f", last, p)
		}
		last = p
	})
	if err != nil {
		t.Fatalf("Mix: %v", err)
	}
	if last != 1 {
		t.Errorf("progress ended at %.3f", last)
	}
	if res.Clamps != 0 || res.BedDropped {
		t.Errorf("clamps %d, dropped %v", res.Clamps, res.BedDropped)
	}
	if res.VoiceGainDB["p|v"] != 0 {
		t.Errorf("a single voice got %.3f dB", res.VoiceGainDB["p|v"])
	}
	// Bed gain: the reference narration I-12+20*log10(0.5/0.25), minus the measured bed.
	wantBed := res.NarrationI + bedBelowNarrLU + 20*math.Log10(2) - res.BedI
	if math.Abs(res.BedGainDB-wantBed) > 1e-9 {
		t.Errorf("bed gain %.3f, want %.3f", res.BedGainDB, wantBed)
	}
	// Sfx gain: the actual bed level plus LevelOffsetDB.
	if want := res.BedI + res.BedGainDB + offset; math.Abs(in.SFX[0].GainDB-want) > 1e-9 {
		t.Errorf("sfx gain %.3f, want %.3f", in.SFX[0].GainDB, want)
	}
	// StemPeak is after the master gain, so the pass 2 peak is that value without it.
	peak := max(res.StemPeak[0], res.StemPeak[1], res.StemPeak[2])
	prePeakDB := 20*math.Log10(peak) - res.MasterGainDB
	wantG := min(mixTargetLUFS-res.PreI, mixCeilingDBTP-res.PreTP, mixStemGuardDBFS-prePeakDB)
	if math.Abs(res.MasterGainDB-wantG) > 0.001 {
		t.Errorf("G = %.4f, want %.4f", res.MasterGainDB, wantG)
	}
	mixTestAssertStemsSum(t, out, frames)
}

func TestMixSilentNarrationFails(t *testing.T) {
	newMeter := mixTestNewMeter(t)
	out := mixTestOutputs(t.TempDir())
	in := &MixInput{
		Frames:    2 * MixRate,
		Narration: []MixClip{{Key: "piece", PCM: make([]int16, MixRate*mixChannels), Len: MixRate, Voice: "p|v"}},
		Spans:     []Span{{From: 0, To: MixRate}},
	}
	_, err := Mix(context.Background(), in, newMeter, out, nil)
	var jobErr *JobError
	if !errors.As(err, &jobErr) || jobErr.Code != CodeNarrationFailed {
		t.Fatalf("err = %v, want %s", err, CodeNarrationFailed)
	}
	if _, statErr := os.Stat(out.Mix); !os.IsNotExist(statErr) {
		t.Fatalf("a failed mix left its file: %v", statErr)
	}
}

func TestMixDropsASilentBed(t *testing.T) {
	newMeter := mixTestNewMeter(t)
	dir := t.TempDir()
	bedPath := filepath.Join(dir, "bed.raw")
	mixTestWriteRaw(t, bedPath, make([]int16, 3*MixRate*mixChannels))
	in := &MixInput{
		Frames:    3 * MixRate,
		Narration: []MixClip{{Key: "piece", PCM: mixTestSine(2*MixRate, 0.3, 440), Len: 2 * MixRate, Voice: "p|v"}},
		Bed:       &MixBed{Path: bedPath, Frames: 3 * MixRate, Volume: 0.25},
		Spans:     []Span{{From: 0, To: 2 * MixRate}},
	}
	out := mixTestOutputs(dir)
	res, err := Mix(context.Background(), in, newMeter, out, nil)
	if err != nil {
		t.Fatalf("Mix: %v", err)
	}
	if !res.BedDropped || res.BedGainDB != 0 || res.StemPeak[1] != 0 {
		t.Fatalf("result = %+v, want a dropped bed with no gain", res)
	}
	if _, statErr := os.Stat(out.Music); !os.IsNotExist(statErr) {
		t.Fatalf("a dropped bed left a music stem: %v", statErr)
	}
	if _, statErr := os.Stat(out.Narration); statErr != nil {
		t.Fatalf("narration stem: %v", statErr)
	}
}

func TestMixSilentFilmKeepsUnityGain(t *testing.T) {
	newMeter := mixTestNewMeter(t)
	dir := t.TempDir()
	bedPath := filepath.Join(dir, "bed.raw")
	mixTestWriteRaw(t, bedPath, make([]int16, 2*MixRate*mixChannels))
	in := &MixInput{Frames: 2 * MixRate, Bed: &MixBed{Path: bedPath, Frames: 2 * MixRate, Volume: 0.25}}
	out := mixTestOutputs(dir)
	res, err := Mix(context.Background(), in, newMeter, out, nil)
	if err != nil {
		t.Fatalf("no narration events, so a silent film must not fail: %v", err)
	}
	if res.MasterGainDB != 0 || !res.BedDropped {
		t.Fatalf("result = %+v, want G 0 and a dropped bed", res)
	}
	// The job record stores these fields, and JSON has no -Inf.
	if _, err := json.Marshal(res); err != nil {
		t.Fatalf("the result is not valid JSON: %v", err)
	}
	if got := len(mixTestReadWAV(t, out.Mix)); got != 2*MixRate*mixChannels {
		t.Fatalf("mix has %d samples", got)
	}
}
```

- [ ] **Step 2: Run it**

In `.worktrees/storymaker-p2c`:

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/render -run '(?i)mix' -count=1
```

Expected: the test package does not build, with `undefined: matchVoices`, `undefined: computeMixLevels`, `undefined: masterGainDB`, `undefined: Mix`, because the passes do not exist yet.

- [ ] **Step 3: Implement**

Append to the end of `internal/storymaker/render/mix.go`. It needs no new imports.

```go
// mixSilentLUFS is how a silent reading is stored in MixResult, because JSON has no -Inf.
const mixSilentLUFS = -70.0

func finiteDB(v float64) float64 {
	if math.IsInf(v, -1) {
		return mixSilentLUFS
	}
	return v
}

// mixLevels holds the gains that pass 1 decides.
type mixLevels struct {
	bedGainDB  float64
	bedDropped bool
	sfxRefDB   float64
}

// computeMixLevels applies the level rules. bed is nil without music. The sfx reference is the actual
// bed level when music is on, narration I-12 without music, and -18 without narration. A silent bed
// counts as no music, because its gain would be infinite.
func computeMixLevels(narration MeterReading, hasNarration bool, bed *MeterReading, volume float64) mixLevels {
	base := bedNoNarrLUFS
	if hasNarration {
		base = narration.I + bedBelowNarrLU
	}
	levels := mixLevels{sfxRefDB: base}
	if bed == nil {
		return levels
	}
	if bed.Silent {
		levels.bedDropped = true
		return levels
	}
	reference := base + 20*math.Log10(volume/0.25)
	levels.bedGainDB = math.Min(reference-bed.I, bedMaxGainDB)
	levels.sfxRefDB = bed.I + levels.bedGainDB
	return levels
}

// masterGainDB is G = min(-16-I, -1-TP, -0.1-max stem peak). A silent mix keeps unity gain.
func masterGainDB(pre MeterReading, stemPeak float64) float64 {
	if pre.Silent {
		return 0
	}
	g := math.Min(mixTargetLUFS-pre.I, mixCeilingDBTP-pre.TP)
	if stemPeak > 0 {
		g = math.Min(g, mixStemGuardDBFS-20*math.Log10(stemPeak))
	}
	return g
}

// matchVoices measures each voice over the speech spans and writes its gain into the clips:
// gain = loudest RMS - voice RMS, capped at +6 dB. A voice with no speech inside the spans keeps 0 dB.
func matchVoices(ctx context.Context, in *MixInput) (map[string]float64, error) {
	type energy struct {
		sum   float64
		count int64
	}
	voices := map[string]*energy{}
	for _, c := range usableClips(in.Narration, in.Frames) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := loadClipPCM(c)
		if err != nil {
			return nil, err
		}
		e := voices[c.Voice]
		if e == nil {
			e = &energy{}
			voices[c.Voice] = e
		}
		end := c.Start + int64(len(data)/mixChannels)
		for _, span := range in.Spans {
			for t := max(span.From, c.Start); t < min(span.To, end); t++ {
				i := (t - c.Start) * mixChannels
				l, r := float64(data[i])/mixFullScale, float64(data[i+1])/mixFullScale
				e.sum += l*l + r*r
				e.count += mixChannels
			}
		}
	}
	levels := map[string]float64{}
	loudest := math.Inf(-1)
	for voice, e := range voices {
		if e.sum > 0 {
			db := 10 * math.Log10(e.sum/float64(e.count))
			levels[voice] = db
			loudest = math.Max(loudest, db)
		}
	}
	gains := make(map[string]float64, len(voices))
	for voice := range voices {
		gain := 0.0
		if db, ok := levels[voice]; ok {
			gain = math.Min(loudest-db, voiceMatchCapDB)
		}
		gains[voice] = gain
	}
	for i := range in.Narration {
		in.Narration[i].GainDB = gains[in.Narration[i].Voice]
	}
	return gains, nil
}

// mixProgress reports the share of the three passes that is done.
type mixProgress struct {
	report      func(float64)
	total, done int64
}

func (p *mixProgress) step(frames int64) {
	p.done += frames
	if p.report != nil {
		p.report(float64(p.done) / float64(p.total))
	}
}

func mixFloat32(dst []float32, src []float64) []float32 {
	dst = dst[:len(src)]
	for i, v := range src {
		dst[i] = float32(v)
	}
	return dst
}

// measureMixSources is pass 1: the narration stem and the raw looped bed, each into its own meter.
// bed is nil without music.
func measureMixSources(ctx context.Context, in *MixInput, hasNarration bool, newMeter func(context.Context) (*Meter, error), prog *mixProgress) (narration MeterReading, bed *MeterReading, err error) {
	var narrMeter, bedMeter *Meter
	defer func() {
		if narrMeter != nil {
			_, _ = narrMeter.Close()
		}
		if bedMeter != nil {
			_, _ = bedMeter.Close()
		}
	}()
	if hasNarration {
		if narrMeter, err = newMeter(ctx); err != nil {
			return narration, nil, err
		}
	}
	if in.Bed != nil {
		if bedMeter, err = newMeter(ctx); err != nil {
			return narration, nil, err
		}
	}
	if narrMeter == nil && bedMeter == nil {
		prog.step(in.Frames)
		return narration, nil, nil
	}
	scratch := make([]float32, mixBlockFrames*mixChannels)
	err = runMixPass(ctx, in, mixPass{rawBed: true}, func(b *mixBlock) error {
		if narrMeter != nil {
			if err := narrMeter.Write(mixFloat32(scratch, b.narration)); err != nil {
				return err
			}
		}
		if bedMeter != nil {
			if err := bedMeter.Write(mixFloat32(scratch, b.music)); err != nil {
				return err
			}
		}
		prog.step(int64(b.frames))
		return nil
	})
	if err != nil {
		return narration, nil, err
	}
	if narrMeter != nil {
		if narration, err = narrMeter.Close(); err != nil {
			return narration, nil, err
		}
	}
	if bedMeter != nil {
		reading, err := bedMeter.Close()
		if err != nil {
			return narration, nil, err
		}
		bed = &reading
	}
	return narration, bed, nil
}

// measurePreMix is pass 2: the mix before the master gain into a meter, with the stem peaks.
func measurePreMix(ctx context.Context, in *MixInput, bedGainDB float64, newMeter func(context.Context) (*Meter, error), prog *mixProgress) (MeterReading, [3]float64, error) {
	var peak [3]float64
	m, err := newMeter(ctx)
	if err != nil {
		return MeterReading{}, peak, err
	}
	defer func() { _, _ = m.Close() }()
	scratch := make([]float32, mixBlockFrames*mixChannels)
	err = runMixPass(ctx, in, mixPass{bedGainDB: bedGainDB}, func(b *mixBlock) error {
		sum := scratch[:b.frames*mixChannels]
		for i := range sum {
			narration, music, sfx := b.narration[i], b.music[i], b.sfx[i]
			sum[i] = float32(narration + music + sfx)
			peak[0] = max(peak[0], math.Abs(narration))
			peak[1] = max(peak[1], math.Abs(music))
			peak[2] = max(peak[2], math.Abs(sfx))
		}
		prog.step(int64(b.frames))
		return m.Write(sum)
	})
	if err != nil {
		return MeterReading{}, peak, err
	}
	reading, err := m.Close()
	return reading, peak, err
}

// Mix runs the three passes and writes the mix and the stems. It stops on ctx at block boundaries. A
// stem with no source gets no file, so its StemPeak is 0 and its path stays empty on disk.
func Mix(ctx context.Context, in *MixInput, newMeter func(context.Context) (*Meter, error), out MixOutputs, progress func(float64)) (res MixResult, err error) {
	defer func() {
		// A cancel kills the meters, and their broken pipes must not hide it.
		if err != nil && ctx.Err() != nil {
			err = ctx.Err()
		}
	}()
	if in == nil || in.Frames <= 0 || out.Mix == "" {
		return res, errors.New("storymaker: the mix has no film length or no output")
	}
	prog := &mixProgress{report: progress, total: 3 * in.Frames}
	hasNarration := len(usableClips(in.Narration, in.Frames)) > 0

	// Pass 1: voice match, then the two source meters.
	if res.VoiceGainDB, err = matchVoices(ctx, in); err != nil {
		return res, err
	}
	narration, bed, err := measureMixSources(ctx, in, hasNarration, newMeter, prog)
	if err != nil {
		return res, err
	}
	if hasNarration && narration.Silent {
		return res, Fail(CodeNarrationFailed, "The narration is silent, so the film cannot be mixed. Generate the narration again.")
	}
	volume := 0.0
	if in.Bed != nil {
		volume = in.Bed.Volume
	}
	levels := computeMixLevels(narration, hasNarration, bed, volume)

	// Pass 2: the sfx gains, then the pre-gain mix.
	working := *in
	if levels.bedDropped {
		working.Bed = nil
	}
	for i := range working.SFX {
		working.SFX[i].GainDB = levels.sfxRefDB + working.SFX[i].LevelOffsetDB
	}
	pre, peak, err := measurePreMix(ctx, &working, levels.bedGainDB, newMeter, prog)
	if err != nil {
		return res, err
	}
	master := masterGainDB(pre, max(peak[0], peak[1], peak[2]))

	// Pass 3: the stems and the mix after the master gain.
	stems := out
	if !hasNarration {
		stems.Narration = ""
	}
	if working.Bed == nil {
		stems.Music = ""
	}
	if len(usableClips(in.SFX, in.Frames)) == 0 {
		stems.SFX = ""
	}
	writer, err := newStemWriter(stems)
	if err != nil {
		return res, err
	}
	defer func() { _ = writer.close() }()
	err = runMixPass(ctx, &working, mixPass{bedGainDB: levels.bedGainDB, masterDB: master}, func(b *mixBlock) error {
		prog.step(int64(b.frames))
		return writer.write(b)
	})
	if err != nil {
		return res, err
	}
	if err := writer.close(); err != nil {
		return res, err
	}

	res.NarrationI, res.BedI = mixSilentLUFS, mixSilentLUFS
	if hasNarration {
		res.NarrationI = finiteDB(narration.I)
	}
	if bed != nil && !bed.Silent {
		res.BedI = bed.I
	}
	res.PreI, res.PreTP = finiteDB(pre.I), finiteDB(pre.TP)
	res.BedGainDB, res.MasterGainDB = levels.bedGainDB, master
	res.BedDropped = levels.bedDropped
	res.StemPeak, res.Clamps = writer.peak, writer.clamps
	return res, nil
}
```

- [ ] **Step 4: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
gofmt -w internal/storymaker/render/mix.go internal/storymaker/render/mix_test.go
go vet ./internal/storymaker/render
go test ./internal/storymaker/render -run '(?i)(mix|meter)' -count=1 -v
```

Expected: `--- PASS` for the five meter tests, the nine mix tests of Tasks A11 and A12, and `TestMixVoiceMatchLandsTwoVoicesWithinOneDB`, `TestMixVoiceMatchIsCappedAtSixDB`, `TestMixLevels`, `TestMixMasterGainIsTheMinOfThreeBounds`, `TestMixPassesApplyTheGains`, `TestMixSilentNarrationFails`, `TestMixDropsASilentBed` and `TestMixSilentFilmKeepsUnityGain`. A `--- SKIP` is not evidence: set `AURAGO_TEST_FFMPEG_DIR` to the FFmpeg folder and run again.

- [ ] **Step 5: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git grep -n "func Mix(" -- internal
git add internal/storymaker/render/mix.go internal/storymaker/render/mix_test.go
git commit -m "feat(storymaker): mix passes with voice match, bed level and master gain" -m "detect_changes answered PARTIAL on the new symbols; grep finds Mix only in mix.go, with no caller yet." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

Drop the middle `-m` if `detect-changes` answers without `partial`. If the tool crashes, closes the connection, answers `not found`, or prints only its banner, use `graft` callers plus grep and say so in that line instead.

<!-- G-A-4 -->

### Task A14: Seeded-audio helper
**Batch:** 2C  **Depends on:** N5, A5
**Files:**
Create: `internal/storymaker/storymakertest/seed.go`
Test: `internal/storymaker/storymakertest/seed_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/storymakertest/seed_test.go`:

```go
package storymakertest

import (
	"context"
	"io"
	"testing"

	"aurago/internal/storymaker"
)

func seedProject(t *testing.T) (*storymaker.ProjectStore, string) {
	t.Helper()
	p := storymaker.NewProjectStore(storymaker.NewMemStore(), "Documents/Storymaker")
	proj, _, _, err := p.Create(context.Background(), "Seed", "16:9", 30)
	if err != nil {
		t.Fatal(err)
	}
	return p, proj.ID
}

func seedStory(voice *storymaker.Voice) *storymaker.Story {
	return &storymaker.Story{
		Schema: storymaker.Schema, Title: "Seed",
		Format: storymaker.Format{Aspect: "16:9", FPS: 30},
		Voice:  voice,
		Scenes: []storymaker.Scene{{ID: "s1", Narration: "Das ist der erste ganze Satz hier. Das ist der zweite ganze Satz hier."}},
	}
}

func TestSeedNarrationFillsEveryPiece(t *testing.T) {
	ctx := context.Background()
	p, id := seedProject(t)
	story := seedStory(&storymaker.Voice{Provider: "Phonon", Voice: "thorsten", Language: "de"})

	before, err := p.AudioAnswer(ctx, id, story, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Missing) != 1 || before.Chars.Missing == 0 {
		t.Fatalf("before the seed: missing %+v, chars %+v", before.Missing, before.Chars)
	}

	if err := SeedNarration(ctx, p, id, story, 1); err != nil {
		t.Fatal(err)
	}
	answer, err := p.AudioAnswer(ctx, id, story, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(answer.Missing) != 0 || answer.Chars.Missing != 0 {
		t.Fatalf("after the seed: missing %+v, chars %+v", answer.Missing, answer.Chars)
	}
	blocks := answer.Narration["s1"].Blocks
	if len(blocks) != 1 || !blocks[0].SpeechOK {
		t.Fatalf("blocks = %+v", blocks)
	}
	for _, piece := range blocks[0].Pieces {
		// Speech starts at 100 ms and in_ms is start minus the 40 ms margin.
		if piece.InMS != 60 {
			t.Errorf("in_ms = %d, want 60", piece.InMS)
		}
		rc, _, err := p.OpenAudio(ctx, id, "narration", piece.Hash+"."+piece.Ext)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		decoded, ext, err := storymaker.DecodeAudio(data)
		if err != nil || ext != "wav" || decoded.Rate != 24000 || decoded.Channels != 1 {
			t.Fatalf("decoded rate %d channels %d ext %q err %v", decoded.Rate, decoded.Channels, ext, err)
		}
		env := storymaker.ComputeEnvelope(decoded)
		if env.DurationMS != 1000 {
			t.Fatalf("duration = %d, want 1000", env.DurationMS)
		}
		runs := env.SpeechRuns()
		// The last speech window covers 890 to 900 ms, and SpeechRuns may name either edge of it.
		if len(runs) != 1 || runs[0][0] != 100 || runs[0][1] < 890 || runs[0][1] > 900 {
			t.Fatalf("speech runs = %v, want one from 100 to the end minus 100", runs)
		}
		// -20 dBFS is (-20+60)*255/60 = 170. Window 50 lies at 500 ms, window 0 in the silent margin.
		if level := env.Levels[50]; level < 169 || level > 171 {
			t.Errorf("speech level = %d, want 170", level)
		}
		if env.Levels[0] != 0 {
			t.Errorf("margin level = %d, want 0", env.Levels[0])
		}
	}
}

func TestSeedNarrationSeedsAStoryWithoutAVoice(t *testing.T) {
	ctx := context.Background()
	p, id := seedProject(t)
	story := seedStory(nil)
	block := storymaker.Blocks(story)[0]
	if block.Voice.Provider != "" {
		t.Fatalf("voice = %+v, want the empty voice", block.Voice)
	}
	if err := SeedNarration(ctx, p, id, story, 1); err != nil {
		t.Fatal(err)
	}
	answer, err := p.AudioAnswer(ctx, id, story, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(answer.Missing) != 0 {
		t.Fatalf("missing = %+v", answer.Missing)
	}
	files, err := p.PieceFiles(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, piece := range storymaker.PiecesOf(block) {
		f, ok := files[storymaker.PieceKey(storymaker.Voice{}, piece.Text)]
		if !ok || f.Ext != "wav" || !f.HasEnv {
			t.Errorf("piece %q: file %+v, found %v", piece.Text, f, ok)
		}
	}
}

func TestSeedMusicLeavesAFileAndAValidMeta(t *testing.T) {
	ctx := context.Background()
	p, id := seedProject(t)
	key := storymaker.MusicKeyGenerated("calm-ambient", "")
	if err := SeedMusic(ctx, p, id, key, 2); err != nil {
		t.Fatal(err)
	}
	meta, audioExt, err := p.MusicInfo(ctx, id, key)
	if err != nil {
		t.Fatal(err)
	}
	if meta.V != 1 || meta.Ext != "wav" || meta.DurationMS != 2000 || audioExt != "wav" {
		t.Fatalf("meta = %+v, audio ext %q", meta, audioExt)
	}
	rc, _, err := p.OpenAudio(ctx, id, "music", key+".wav")
	if err != nil {
		t.Fatal(err)
	}
	_ = rc.Close()
}
```

- [ ] **Step 2: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/storymakertest -count=1
```

Expected: FAIL to build with `undefined: SeedNarration` and `undefined: SeedMusic`, because the package has only the test file.

- [ ] **Step 3: Implement**

Create `internal/storymaker/storymakertest/seed.go`:

```go
// Package storymakertest writes generated audio into a project, for tests that need a narrated or
// scored story without a provider. No production code imports it.
package storymakertest

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"aurago/internal/storymaker"
)

const (
	seedRate     = 24000
	seedMarginMS = 100
	// seedSpeechHz has 4 whole cycles in a 10 ms envelope window, so every full window has the same power.
	seedSpeechHz = 400
	seedMusicHz  = 220
	// seedPeak is the peak of a sine with an RMS of -20 dBFS: 10^(-20/20) * sqrt(2).
	seedPeak = 0.1 * math.Sqrt2
)

// SeedNarration writes a generated 24 kHz mono s16 WAV and its envelope for every missing piece,
// with speech at -20 dBFS from 100 ms to the end minus 100 ms. Used by the fixtures that the
// narration_incomplete gate would otherwise break.
func SeedNarration(ctx context.Context, p *storymaker.ProjectStore, id string, s *storymaker.Story, seconds float64) error {
	if seconds*1000 <= 2*seedMarginMS {
		return fmt.Errorf("storymakertest: a piece of %v s has no room for speech", seconds)
	}
	frames := int(math.Round(seconds * seedRate))
	margin := seedMarginMS * seedRate / 1000
	data := toneWAV(frames, margin, frames-margin, seedSpeechHz)
	decoded, ext, err := storymaker.DecodeAudio(data)
	if err != nil {
		return fmt.Errorf("storymakertest: decode the seeded piece: %w", err)
	}
	env := storymaker.ComputeEnvelope(decoded)
	files, err := p.PieceFiles(ctx, id)
	if err != nil {
		return err
	}
	if files == nil {
		files = map[string]storymaker.PieceFile{}
	}
	for _, block := range storymaker.Blocks(s) {
		for _, piece := range storymaker.PiecesOf(block) {
			hash := storymaker.PieceKey(block.Voice, piece.Text)
			if f, ok := files[hash]; ok && f.HasEnv {
				continue
			}
			if err := p.WritePiece(ctx, id, hash, ext, data, env); err != nil {
				return err
			}
			files[hash] = storymaker.PieceFile{Ext: ext, HasEnv: true}
		}
	}
	return nil
}

// SeedMusic writes a generated WAV and its meta through WriteMusic and WriteMusicMeta.
func SeedMusic(ctx context.Context, p *storymaker.ProjectStore, id string, key string, seconds float64) error {
	if seconds < 1 {
		return fmt.Errorf("storymakertest: music needs at least 1 s, got %v", seconds)
	}
	frames := int(math.Round(seconds * seedRate))
	data := toneWAV(frames, 0, frames, seedMusicHz)
	ext, durationMS, err := storymaker.ProbeAudio(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("storymakertest: probe the seeded music: %w", err)
	}
	if err := p.WriteMusic(ctx, id, key, ext, data); err != nil && !errors.Is(err, storymaker.ErrConflict) {
		return err
	}
	return p.WriteMusicMeta(ctx, id, key, storymaker.MusicMeta{V: 1, Ext: ext, DurationMS: durationMS})
}

// toneWAV is a 24 kHz mono PCM16 WAV of frames samples with a sine in [from, to) and silence around it.
func toneWAV(frames, from, to int, hz float64) []byte {
	const header = 44
	size := frames * 2
	data := make([]byte, header+size)
	copy(data[0:], "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+size))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1) // PCM
	binary.LittleEndian.PutUint16(data[22:], 1) // mono
	binary.LittleEndian.PutUint32(data[24:], seedRate)
	binary.LittleEndian.PutUint32(data[28:], seedRate*2)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(size))
	for i := from; i < to; i++ {
		s := int16(math.Round(seedPeak * 32767 * math.Sin(2*math.Pi*hz*float64(i)/seedRate)))
		binary.LittleEndian.PutUint16(data[header+2*i:], uint16(s))
	}
	return data
}
```

- [ ] **Step 4: Run**

```powershell
go test ./internal/storymaker/storymakertest -count=1
gofmt -l internal/storymaker
go vet ./internal/storymaker/storymakertest
```

Expected: PASS, and `gofmt -l` prints nothing.

- [ ] **Step 5: Commit**

Run detect_changes first. Outline section 5 says what to do with a PARTIAL answer on new files and with a crash of the tool, a closed connection, a `not found` answer or a banner-only answer.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/storymakertest/seed.go internal/storymaker/storymakertest/seed_test.go
git commit -m "test(storymaker): seeded narration and music helper" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A15: Music key resolver and `Plan.Music`
**Batch:** 2C  **Depends on:** A5, A14
**Files:**
Create: `internal/server/storymaker_music_key.go`
Modify: `internal/server/storymaker_audio_handlers.go`
Modify: `internal/storymaker/narration.go`
Modify: `internal/storymaker/render/render.go`
Test: `internal/server/storymaker_music_key_test.go`

- [ ] **Step 1: Impact**

From the main checkout, run this for `storymakerMusicKey`, `AudioAnswer`, `Prepare` and `Plan`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<symbol>" --direction upstream --repo .
```

Report callers, processes and risk. HIGH, CRITICAL and UNKNOWN follow the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

Create `internal/server/storymaker_music_key_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/storymaker/storymakertest"
	"aurago/internal/tools"
)

func musicKeyPutStory(t *testing.T, s *Server, token, id, etag, body string) string {
	t.Helper()
	w := storymakerRequest(s, token, http.MethodPut, "/projects/"+id+"/story", strings.NewReader(body), map[string]string{"If-Match": etag})
	if w.Code != http.StatusOK {
		t.Fatalf("save story: %d %s", w.Code, w.Body.String())
	}
	var saved struct {
		ETag string `json:"etag"`
	}
	decodeBody(t, w, &saved)
	return saved.ETag
}

func musicKeyAnswer(t *testing.T, s *Server, token, id string) storymaker.AudioAnswer {
	t.Helper()
	w := storymakerRequest(s, token, http.MethodGet, "/projects/"+id+"/narration", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("narration: %d %s", w.Code, w.Body.String())
	}
	var answer storymaker.AudioAnswer
	decodeBody(t, w, &answer)
	return answer
}

func TestStorymakerMusicKeyRule(t *testing.T) {
	s, _, _ := storymakerAPIFixture(t)
	key := func(m *storymaker.Music) string {
		t.Helper()
		k, err := s.storymakerMusicKey(context.Background(), m)
		if err != nil {
			t.Fatalf("key of %+v: %v", m, err)
		}
		return k
	}
	calm := key(&storymaker.Music{Mode: "generated", Mood: "calm-ambient"})
	lofi := key(&storymaker.Music{Mode: "generated", Mood: "lo-fi"})
	if len(calm) != 64 || calm == lofi {
		t.Fatalf("keys: calm %q, lo-fi %q", calm, lofi)
	}
	zero := 0.0
	if got := key(&storymaker.Music{Mode: "generated", Mood: "calm-ambient", Volume: &zero}); got != "" {
		t.Errorf("volume 0 gave key %q", got)
	}
	if got := key(&storymaker.Music{Mode: "none"}); got != "" {
		t.Errorf("mode none gave key %q", got)
	}
	if got := key(nil); got != "" {
		t.Errorf("no music gave key %q", got)
	}
	if got := key(&storymaker.Music{Mode: "file"}); got != "" {
		t.Errorf("mode file without a source gave key %q", got)
	}
}

func TestStorymakerMusicKeyFollowsATrack(t *testing.T) {
	s, _, _ := storymakerAPIFixture(t)
	ctx := context.Background()
	db, err := tools.InitMediaRegistryDB(filepath.Join(t.TempDir(), "media.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s.MediaRegistryDB = db
	audioDir := filepath.Join(s.Cfg.Directories.DataDir, "audio")
	if err := os.MkdirAll(audioDir, 0o700); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(audioDir, "song.wav")
	outside := filepath.Join(t.TempDir(), "song.wav")
	for _, p := range []string{inside, outside} {
		if err := os.WriteFile(p, []byte("one"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	register := func(mediaType, path string) string {
		t.Helper()
		id, _, err := tools.RegisterMedia(db, tools.MediaItem{MediaType: mediaType, SourceTool: "manual", Filename: filepath.Base(path), FilePath: path, Format: "wav"})
		if err != nil {
			t.Fatal(err)
		}
		return strconv.FormatInt(id, 10)
	}

	good := register("music", inside)
	first, err := s.storymakerMusicKey(ctx, &storymaker.Music{Mode: "file", Track: good})
	if err != nil || len(first) != 64 {
		t.Fatalf("track key = %q, %v", first, err)
	}
	// The size is part of the key.
	if err := os.WriteFile(inside, []byte("a longer file"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := s.storymakerMusicKey(ctx, &storymaker.Music{Mode: "file", Track: good})
	if err != nil || second == first {
		t.Fatalf("key after the file changed = %q, %v", second, err)
	}

	for _, bad := range []string{register("music", outside), register("image", inside), "999"} {
		key, err := s.storymakerMusicKey(ctx, &storymaker.Music{Mode: "file", Track: bad})
		if key != "" || !errors.Is(err, storymaker.ErrMusicSourceMissing) {
			t.Errorf("track %s: key %q, err %v", bad, key, err)
		}
	}
}

func TestStorymakerNarrationMusicAnswerReadsTheMeta(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, writeToken)
	id := p.Project.ID
	ctx := context.Background()
	const story = `{"schema":"storymaker/1","title":"Musik","music":{"mode":"generated","mood":"calm-ambient"},"scenes":[{"id":"s1","duration":%SECONDS%}]}`
	etag := musicKeyPutStory(t, s, writeToken, id, p.ETag, strings.ReplaceAll(story, "%SECONDS%", "3"))
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	key := storymaker.MusicKeyGenerated("calm-ambient", "")
	base := render.ProjectsRoot + "/" + id + "/audio/music/" + key

	if answer := musicKeyAnswer(t, s, readToken, id); answer.Music != nil {
		t.Fatalf("nothing stored: music = %+v", answer.Music)
	}
	// The audio bytes are not audio, so a decode would fail. Only the meta can supply the duration.
	if err := rt.projects.WriteMusic(ctx, id, key, "wav", []byte("this is not audio")); err != nil {
		t.Fatal(err)
	}
	if answer := musicKeyAnswer(t, s, readToken, id); answer.Music != nil {
		t.Fatalf("a file without meta: music = %+v", answer.Music)
	}
	if _, err := rt.store.WriteFile(ctx, base+".meta.json", []byte(`{"v":9}`), ""); err != nil {
		t.Fatal(err)
	}
	if answer := musicKeyAnswer(t, s, readToken, id); answer.Music != nil {
		t.Fatalf("an invalid meta: music = %+v", answer.Music)
	}
	if err := rt.projects.WriteMusicMeta(ctx, id, key, storymaker.MusicMeta{V: 1, Ext: "wav", DurationMS: 61000}); err != nil {
		t.Fatal(err)
	}
	answer := musicKeyAnswer(t, s, readToken, id)
	if answer.Music == nil || answer.Music.Key != key || answer.Music.Ext != "wav" || answer.Music.DurationMS != 61000 {
		t.Fatalf("music = %+v, want the meta's values", answer.Music)
	}
	// A longer film keeps the key.
	musicKeyPutStory(t, s, writeToken, id, etag, strings.ReplaceAll(story, "%SECONDS%", "9"))
	if after := musicKeyAnswer(t, s, readToken, id); after.Music == nil || after.Music.Key != key {
		t.Fatalf("after a length change: music = %+v, want key %s", after.Music, key)
	}
	// A meta without its file counts as missing too.
	if err := rt.store.Remove(ctx, base+".wav"); err != nil {
		t.Fatal(err)
	}
	if answer := musicKeyAnswer(t, s, readToken, id); answer.Music != nil {
		t.Fatalf("a meta without a file: music = %+v", answer.Music)
	}
}

func TestStorymakerNarrationReportsAMovedMusicSource(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, writeToken)
	id := p.Project.ID
	ctx := context.Background()
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	const source = "Documents/Music/theme.wav"
	if _, err := rt.store.WriteFile(ctx, source, []byte("source bytes"), ""); err != nil {
		t.Fatal(err)
	}
	body := `{"schema":"storymaker/1","title":"Musik","voice":{"provider":"phonon","voice":"thorsten","language":"de"},` +
		`"music":{"mode":"file","file":"` + source + `"},"scenes":[{"id":"s1","narration":"Das ist ein ganzer Satz zur Firewall."}]}`
	musicKeyPutStory(t, s, writeToken, id, p.ETag, body)
	var story storymaker.Story
	if err := json.Unmarshal([]byte(body), &story); err != nil {
		t.Fatal(err)
	}
	if err := storymakertest.SeedNarration(ctx, rt.projects, id, &story, 1); err != nil {
		t.Fatal(err)
	}
	answer := musicKeyAnswer(t, s, readToken, id)
	if answer.MusicIssue != "" || answer.Music != nil || len(answer.Missing) != 0 {
		t.Fatalf("source in place: %+v", answer)
	}
	if err := rt.store.Remove(ctx, source); err != nil {
		t.Fatal(err)
	}
	answer = musicKeyAnswer(t, s, readToken, id)
	if answer.MusicIssue != "source_missing" || answer.Music != nil {
		t.Fatalf("source moved: issue %q, music %+v", answer.MusicIssue, answer.Music)
	}
	if len(answer.Narration["s1"].Blocks) != 1 || len(answer.Missing) != 0 {
		t.Fatalf("the narration was lost: %+v, missing %+v", answer.Narration, answer.Missing)
	}
}

// Choosing the mode "File or track" saves a story with no source yet. That is an unfinished choice, not a source that is
// gone: the save reports it, and the audio answer stays quiet.
func TestStorymakerNarrationHasNoIssueBeforeASourceIsChosen(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, writeToken)
	const body = `{"schema":"storymaker/1","title":"Musik","music":{"mode":"file"},"scenes":[{"id":"s1"}]}`
	w := storymakerRequest(s, writeToken, http.MethodPut, "/projects/"+p.Project.ID+"/story", strings.NewReader(body), map[string]string{"If-Match": p.ETag})
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "/music/file") {
		t.Fatalf("save story: %d %s", w.Code, w.Body.String())
	}
	answer := musicKeyAnswer(t, s, readToken, p.Project.ID)
	if answer.Music != nil || answer.MusicIssue != "" {
		t.Fatalf("no source chosen: music %+v, issue %q", answer.Music, answer.MusicIssue)
	}
}

func TestStorymakerRenderPlanHoldsTheMusicRef(t *testing.T) {
	plans := make(chan *render.Plan, 1)
	s, token, p := renderFixture(t, func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		plan, _ := payload.(*render.Plan)
		plans <- plan
		return render.Output{}, nil
	})
	id := p.Project.ID
	etag := musicKeyPutStory(t, s, token, id, p.ETag, `{"schema":"storymaker/1","title":"Musik","music":{"mode":"generated","mood":"calm-ambient"},"scenes":[{"id":"s1"}]}`)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	key := storymaker.MusicKeyGenerated("calm-ambient", "")
	if err := storymakertest.SeedMusic(context.Background(), rt.projects, id, key, 2); err != nil {
		t.Fatal(err)
	}
	w := storymakerRequest(s, token, http.MethodPost, "/projects/"+id+"/render", nil, map[string]string{"If-Match": etag})
	if w.Code != http.StatusAccepted {
		t.Fatalf("render = %d %s", w.Code, w.Body.String())
	}
	select {
	case plan := <-plans:
		if plan == nil || plan.Music == nil || plan.Music.Key != key || plan.Music.Ext != "wav" || plan.Music.DurationMS != 2000 {
			t.Fatalf("plan = %+v", plan)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the job did not start")
	}
}
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/server -run '(?i)(StorymakerMusicKey|StorymakerNarrationMusicAnswer|StorymakerNarrationReportsAMoved|StorymakerNarrationHasNoIssue|StorymakerRenderPlanHolds)' -count=1
```

Expected: FAIL to build with `plan.Music undefined (type *render.Plan has no field or method Music)`.

- [ ] **Step 4: Implement the store side**

In `internal/storymaker/narration.go`, make these two replacements. The first replaces the signature, whose text is frozen in section 1.1. N5's body becomes `narrationAnswer` and its code stays as it is. The second replaces N5's comment inside that body, because the wrapper now does the lookup the comment announces. No caller changes, because `AudioAnswer` keeps its name and signature.

Old:

```go
func (p *ProjectStore) AudioAnswer(ctx context.Context, id string, s *Story, resolve MusicKeyResolver) (AudioAnswer, error) {
```

New:

```go
func (p *ProjectStore) AudioAnswer(ctx context.Context, id string, s *Story, resolve MusicKeyResolver) (AudioAnswer, error) {
	key := ""
	capture := resolve
	if resolve != nil {
		capture = func(ctx context.Context, m *Music) (string, error) {
			k, err := resolve(ctx, m)
			if err == nil {
				key = k
			}
			return k, err
		}
	}
	answer, err := p.narrationAnswer(ctx, id, s, capture)
	if err != nil || key == "" {
		return answer, err
	}
	// Music stays null unless the key's audio file and a valid meta file both exist. Only the meta
	// is read, so this answer never decodes a track.
	meta, audioExt, err := p.MusicInfo(ctx, id, key)
	if err != nil {
		return AudioAnswer{}, err
	}
	if meta.V != 0 && audioExt != "" {
		answer.Music = &MusicRef{Key: key, Ext: audioExt, DurationMS: meta.DurationMS}
	}
	return answer, nil
}

// narrationAnswer collects the narration part of the answer and the music issue.
func (p *ProjectStore) narrationAnswer(ctx context.Context, id string, s *Story, resolve MusicKeyResolver) (AudioAnswer, error) {
```

Old:

```go
	// Only the resolver's error counts here, so a key leaves Music null. Batch 2C wraps this body with
	// the lookup of the key's audio and meta files.
```

New:

```go
	// Only the resolver's error counts here. AudioAnswer looks up the key's audio and meta files and
	// sets Music.
```

- [ ] **Step 5: Implement the resolver**

In `internal/server/storymaker_audio_handlers.go`, delete N15's stub and its two-line doc comment. Both blocks quote the doc comment of the next function, which stays, so the edit has an anchor. Every other function in the file keeps using the file's imports.

Old:

```go
// storymakerMusicKey resolves the key of the story's music. Batch 2B has no music, so nothing is to
// be played. A15 writes the full rule.
func (s *Server) storymakerMusicKey(ctx context.Context, m *storymaker.Music) (string, error) {
	return "", nil
}

// handleStorymakerNarration answers GET /projects/{id}/narration. It works while the desktop is read-only.
```

New:

```go
// handleStorymakerNarration answers GET /projects/{id}/narration. It works while the desktop is read-only.
```

Create `internal/server/storymaker_music_key.go`:

```go
package server

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strconv"

	"aurago/internal/storymaker"
	"aurago/internal/tools"
)

// storymakerMusicKey returns the key of the story's music. "" means nothing is to be played: no
// music, mode none, volume 0, or mode file with no source chosen yet. Only a file or track source
// that was set and no longer exists returns storymaker.ErrMusicSourceMissing. The film length is
// not part of any key.
func (s *Server) storymakerMusicKey(ctx context.Context, m *storymaker.Music) (string, error) {
	if m == nil || storymaker.MusicVolume(m) <= 0 {
		return "", nil
	}
	switch m.Mode {
	case "generated":
		return storymaker.MusicKeyGenerated(m.Mood, m.Prompt), nil
	case "file":
		if m.Track != "" {
			rel, err := s.storymakerTrackPath(m.Track)
			if err != nil {
				return "", err
			}
			info, err := s.storymakerMusicSource(ctx, "Music/"+rel)
			if err != nil {
				return "", err
			}
			return storymaker.MusicKeyTrack(m.Track, info.Size, info.ModTime), nil
		}
		if m.File == "" {
			return "", nil
		}
		info, err := s.storymakerMusicSource(ctx, m.File)
		if err != nil {
			return "", err
		}
		return storymaker.MusicKeyFile(m.File, info.Size, info.ModTime), nil
	}
	return "", nil
}

// storymakerTrackPath returns the path of a registered music track relative to DataDir/audio. A
// track that is gone, not music or outside that folder counts as a missing source.
func (s *Server) storymakerTrackPath(track string) (string, error) {
	id, err := strconv.ParseInt(track, 10, 64)
	if err != nil || id <= 0 {
		return "", storymaker.ErrMusicSourceMissing
	}
	item, err := tools.GetMedia(s.MediaRegistryDB, id)
	if err != nil || item.MediaType != "music" {
		return "", storymaker.ErrMusicSourceMissing
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return "", errors.New("configuration unavailable")
	}
	root, err := filepath.Abs(filepath.Join(cfg.Directories.DataDir, "audio"))
	if err != nil {
		return "", err
	}
	full, err := filepath.Abs(item.FilePath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, full)
	if err != nil || !filepath.IsLocal(rel) {
		return "", storymaker.ErrMusicSourceMissing
	}
	return filepath.ToSlash(rel), nil
}

// storymakerMusicSource reads the size and time of a music source through the desktop store. It
// takes the desktop service directly, because a music job calls this too and a job must not call
// storymakerRuntime (frozen section 1.4).
func (s *Server) storymakerMusicSource(ctx context.Context, desktopPath string) (storymaker.FileInfo, error) {
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		return storymaker.FileInfo{}, err
	}
	st := &storymakerStore{}
	st.use(svc)
	r, info, err := st.Open(ctx, desktopPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return storymaker.FileInfo{}, storymaker.ErrMusicSourceMissing
		}
		return storymaker.FileInfo{}, err
	}
	_ = r.Close()
	return info, nil
}
```

- [ ] **Step 6: Implement `Plan.Music`**

In `internal/storymaker/render/render.go`, make these two replacements. N20 leaves the `StoryETag` line and the start of the `Plan` literal in place. `answer` is the variable that holds the result of `AudioAnswer` in `Prepare` as N20 leaves it.

Old:

```go
	StoryETag string
```

New:

```go
	StoryETag string
	Music     *storymaker.MusicRef // nil unless the key's file and meta exist; volume 0 has no key
```

Old:

```go
Bundles: bundles, StoryETag: etag
```

New:

```go
Bundles: bundles, StoryETag: etag, Music: answer.Music
```

- [ ] **Step 7: Run**

```powershell
gofmt -w internal/storymaker/render/render.go
go test ./internal/storymaker/... -count=1
go test ./internal/server -run Storymaker -count=1 -timeout 20m
gofmt -l internal/storymaker internal/server
go vet ./internal/storymaker/... ./internal/server
```

Expected: PASS. `gofmt -l` prints nothing, because the `gofmt -w` line aligned the new `Music` field with its neighbours.

- [ ] **Step 8: Commit**

Run detect_changes first, under the rule in outline section 5.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/server/storymaker_music_key.go internal/server/storymaker_music_key_test.go internal/server/storymaker_audio_handlers.go internal/storymaker/narration.go internal/storymaker/render/render.go
git commit -m "feat(storymaker): music key resolver and Plan.Music" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A16: Mix plan, input build and disk check
**Batch:** 2C  **Depends on:** A13, A15
**Files:**
Create: `internal/storymaker/render/mixinput.go`
Modify: `internal/storymaker/render/mix.go`
Test: `internal/storymaker/render/mixinput_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/storymaker/render/mixinput_test.go`:

```go
package render

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/storymakertest"
)

// mixTestStory has two scenes with one sentence, so both narration blocks share one piece hash.
func mixTestStory() *storymaker.Story {
	const sentence = "Das ist ein ganzer Satz zur Firewall."
	return &storymaker.Story{
		Schema: storymaker.Schema, Title: "Mix",
		Format: storymaker.Format{Aspect: "16:9", FPS: 30},
		Voice:  &storymaker.Voice{Provider: "phonon", Voice: "thorsten", Language: "de"},
		Scenes: []storymaker.Scene{{ID: "s1", Narration: sentence}, {ID: "s2", Narration: sentence}},
	}
}

// seededMixRun seeds 2 s pieces and returns a run whose report places them at 500 ms and 3000 ms
// of a 5 s film (150 frames at 30 fps).
func seededMixRun(t *testing.T, fx *runFixture, story *storymaker.Story) (*renderRun, storymaker.AudioBlock) {
	t.Helper()
	ctx := context.Background()
	if err := storymakertest.SeedNarration(ctx, fx.projects, fx.projectID, story, 2); err != nil {
		t.Fatal(err)
	}
	answer, err := fx.projects.AudioAnswer(ctx, fx.projectID, story, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(answer.Missing) != 0 {
		t.Fatalf("missing after the seed: %+v", answer.Missing)
	}
	block := answer.Narration["s1"].Blocks[0]
	hash := block.Pieces[0].Hash
	rep := compileReport{FrameCount: 150, AudioEvents: []reportAudioEvent{
		{Kind: "narration", Ref: hash, StartMS: 500, SceneID: "s1", BlockID: "s1:n"},
		{Kind: "narration", Ref: hash, StartMS: 3000, SceneID: "s2", BlockID: "s2:n"},
	}}
	plan := &Plan{ProjectID: fx.projectID, Story: *story, Blocks: storymaker.Blocks(story), AudioInfo: answer.Info()}
	return &renderRun{r: fx.runner, job: Job{ID: "mix-job"}, plan: plan, st: fx.settings, fps: 30, report: rep}, block
}

func TestDecodeArgsAreExact(t *testing.T) {
	got := decodeArgs("SRC.mp3", "DST.s16", 3.5)
	// 3.5 s at 192000 bytes per second is 672000 bytes, and times 1.1 that is 739200.
	want := []string{"-hide_banner", "-nostdin", "-loglevel", "error",
		"-protocol_whitelist", "file", "-format_whitelist", "wav,mp3", "-i", "SRC.mp3",
		"-map", "0:a:0", "-ar", "48000", "-ac", "2", "-f", "s16le",
		"-t", "3.500", "-fs", "739200", "-y", "DST.s16"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args =\n%v\nwant\n%v", got, want)
	}
}

func TestEstimateMixBytes(t *testing.T) {
	// input 100 + decoded 1000 + 48000 frames * 4 bytes = 193100.
	if got := EstimateMixBytes(1000, 100, 48000, false); got != 193100 {
		t.Errorf("without stems = %d", got)
	}
	// Stems add 48000 * 4 * 3 * 2 = 1152000.
	if got := EstimateMixBytes(1000, 100, 48000, true); got != 1345100 {
		t.Errorf("with stems = %d", got)
	}
}

func TestHasAudio(t *testing.T) {
	measured := storymaker.AudioInfo{Narration: map[string]storymaker.SceneAudio{"s1": {Blocks: []storymaker.AudioBlock{{ID: "s1:n"}}}}}
	cases := []struct {
		name string
		plan *Plan
		rep  compileReport
		want bool
	}{
		{"nothing", &Plan{}, compileReport{}, false},
		{"a measured block", &Plan{AudioInfo: measured}, compileReport{}, true},
		{"music", &Plan{Music: &storymaker.MusicRef{Key: "k"}}, compileReport{}, true},
		{"an sfx event", &Plan{}, compileReport{AudioEvents: []reportAudioEvent{{Kind: "sfx", Ref: "sfx.pop"}}}, true},
		{"a narration event alone decides nothing", &Plan{}, compileReport{AudioEvents: []reportAudioEvent{{Kind: "narration", Ref: "x"}}}, false},
	}
	for _, tc := range cases {
		if got := HasAudio(tc.plan, tc.rep); got != tc.want {
			t.Errorf("%s: got %v", tc.name, got)
		}
	}
}

func TestBuildMixInputDecodesEachHashOnceWithItsLimits(t *testing.T) {
	fx := newRunFixture(t)
	ctx := context.Background()
	story := mixTestStory()
	story.Music = &storymaker.Music{Mode: "generated", Mood: "calm-ambient"}
	run, block := seededMixRun(t, fx, story)
	key := storymaker.MusicKeyGenerated("calm-ambient", "")
	if err := storymakertest.SeedMusic(ctx, fx.projects, fx.projectID, key, 3); err != nil {
		t.Fatal(err)
	}
	run.plan.Music = &storymaker.MusicRef{Key: key, Ext: "wav", DurationMS: 3000}

	var calls []string
	orig := decodeCommand
	decodeCommand = func(ctx context.Context, ffmpeg string, args []string) error {
		calls = append(calls, strings.Join(args, " "))
		return orig(ctx, ffmpeg, args)
	}
	t.Cleanup(func() { decodeCommand = orig })

	plan, err := run.planMixInput(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	in, err := run.buildMixInput(ctx, plan, dir)
	if err != nil {
		t.Fatal(err)
	}

	// Both scenes hold one hash: one decode for the piece, one for the music.
	if len(calls) != 2 {
		t.Fatalf("decodes = %d: %v", len(calls), calls)
	}
	piece := block.Pieces[0]
	// out_ms/1000+1 seconds at 192000 bytes per second, times 1.1.
	maxPiece := float64(piece.OutMS)/1000 + 1
	wantPiece := "-t " + strconv.FormatFloat(maxPiece, 'f', 3, 64) + " -fs " + strconv.FormatInt(int64(maxPiece*192000)*11/10, 10)
	if !strings.Contains(calls[0], wantPiece) {
		t.Errorf("piece decode lacks %q: %s", wantPiece, calls[0])
	}
	// The film is 150/30 = 5 s, so music gets 7 s: 7*192000 = 1344000, times 1.1 = 1478400.
	if !strings.Contains(calls[1], "-t 7.000 -fs 1478400") {
		t.Errorf("music decode: %s", calls[1])
	}
	for _, c := range calls {
		if !strings.Contains(c, "-protocol_whitelist file -format_whitelist wav,mp3") {
			t.Errorf("decode without whitelists: %s", c)
		}
	}

	// 150 frames * 48000 / 30.
	if in.Frames != 240000 {
		t.Errorf("frames = %d", in.Frames)
	}
	if len(in.Narration) != 2 {
		t.Fatalf("narration clips = %+v", in.Narration)
	}
	for i, startMS := range []int64{500, 3000} {
		c := in.Narration[i]
		if c.Start != startMS*48 || c.In != int64(piece.InMS)*48 || c.Voice != "phonon|thorsten" {
			t.Errorf("clip %d = %+v", i, c)
		}
		// The decode may end up to 10 ms early, which is 480 frames.
		wantLen := int64(piece.OutMS-piece.InMS) * 48
		if c.Len > wantLen || wantLen-c.Len > 480 {
			t.Errorf("clip %d: len %d, want about %d", i, c.Len, wantLen)
		}
		// The piece is trimmed at its start, because in_ms is 60. 5 ms at 48 frames per ms is 240.
		if c.Fade != 240 {
			t.Errorf("clip %d: fade %d", i, c.Fade)
		}
		if filepath.Dir(c.Path) != filepath.Join(dir, "dec") {
			t.Errorf("clip %d: path %s", i, c.Path)
		}
		if info, err := os.Stat(c.Path); err != nil || info.Size() == 0 || info.Size()%4 != 0 {
			t.Errorf("clip %d: raw file %v, %v", i, info, err)
		}
	}
	span := block.Spans[0]
	if len(in.Spans) != 2 || in.Spans[0].From != (500+int64(span[0]))*48 || in.Spans[1].From != (3000+int64(span[0]))*48 {
		t.Errorf("spans = %+v, block span %v", in.Spans, span)
	}
	// 3 s of music at 48000 frames per second, within 10 ms.
	if in.Bed == nil || in.Bed.Volume != 0.25 || math.Abs(float64(in.Bed.Frames-144000)) > 480 {
		t.Errorf("bed = %+v", in.Bed)
	}
}

func TestBuildMixInputReadsTheTakeOfTheEvent(t *testing.T) {
	fx := newRunFixture(t)
	ctx := context.Background()
	entry, ok := fx.deps.Catalog.Lookup("sfx.pop")
	if !ok || len(entry.Takes) < 3 || entry.Hit == nil {
		t.Fatalf("catalog entry = %+v", entry)
	}
	take := entry.Takes[2]
	rep := compileReport{FrameCount: 150, AudioEvents: []reportAudioEvent{{Kind: "sfx", Ref: "sfx.pop", StartMS: 1000, SceneID: "s1", Take: 2}}}
	story := mixTestStory()
	run := &renderRun{r: fx.runner, job: Job{ID: "mix-job"}, plan: &Plan{ProjectID: fx.projectID, Story: *story}, st: fx.settings, fps: 30, report: rep}
	plan, err := run.planMixInput(ctx)
	if err != nil {
		t.Fatal(err)
	}
	in, err := run.buildMixInput(ctx, plan, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(in.SFX) != 1 {
		t.Fatalf("sfx = %+v", in.SFX)
	}
	c := in.SFX[0]
	if c.Key != take.File || c.Start != 48000 || c.LevelOffsetDB != entry.Hit.OverDB-take.L50LUFS {
		t.Errorf("clip key %q start %d offset %v, take %+v", c.Key, c.Start, c.LevelOffsetDB, take)
	}
	// Stereo from a mono file: the two channels of a frame are equal.
	if c.Len == 0 || int64(len(c.PCM)) != c.Len*2 || c.PCM[0] != c.PCM[1] {
		t.Errorf("clip len %d, pcm %d", c.Len, len(c.PCM))
	}
	if math.Abs(float64(c.Len)-take.DurMS*48) > 240 {
		t.Errorf("clip len %d, take lasts %v ms", c.Len, take.DurMS)
	}
	wantAt := c.Start + int64(math.Round(take.ImpactMS*48))
	if len(in.Impacts) != 1 || in.Impacts[0].At != wantAt || in.Impacts[0].DepthDB != entry.Hit.DipDB {
		t.Errorf("impacts = %+v, want at %d", in.Impacts, wantAt)
	}
}

func TestBuildMixInputRefusesLowDiskBeforeAnyCopy(t *testing.T) {
	fx := newRunFixture(t)
	ctx := context.Background()
	run, _ := seededMixRun(t, fx, mixTestStory())
	orig := freeDiskBytes
	freeDiskBytes = func(string) (int64, error) { return 1 << 20, nil }
	t.Cleanup(func() { freeDiskBytes = orig })
	plan, err := run.planMixInput(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	_, err = run.buildMixInput(ctx, plan, dir)
	var je *JobError
	if !errors.As(err, &je) || je.Code != CodeRenderFailed {
		t.Fatalf("err = %v", err)
	}
	needed := EstimateMixBytes(plan.decodedBytes, plan.inputBytes, plan.filmFrames, plan.stems) + minFreeStagingByte
	wantMB := strconv.FormatInt((needed+(1<<20)-1)>>20, 10) + " MB"
	if !strings.Contains(je.Message, wantMB) {
		t.Errorf("message %q lacks %q", je.Message, wantMB)
	}
	if entries, err := os.ReadDir(filepath.Join(dir, "dec")); err == nil && len(entries) != 0 {
		t.Errorf("dec holds %v", entries)
	}
}

func TestBuildMixInputComparesTheMP3Timelines(t *testing.T) {
	fx := newRunFixture(t)
	ctx := context.Background()
	data, err := os.ReadFile("../testdata/narration/one.mp3")
	if err != nil {
		t.Fatal(err)
	}
	decoded, ext, err := storymaker.DecodeAudio(data)
	if err != nil || ext != "mp3" {
		t.Fatalf("decode: ext %q, %v", ext, err)
	}
	env := storymaker.ComputeEnvelope(decoded)
	goRuns := env.SpeechRuns()
	if len(goRuns) == 0 {
		t.Fatal("one.mp3 has no speech run")
	}
	story := mixTestStory()
	story.Scenes = story.Scenes[:1]
	first := storymaker.Blocks(story)[0]
	hash := storymaker.PieceKey(first.Voice, storymaker.PiecesOf(first)[0].Text)
	if err := fx.projects.WritePiece(ctx, fx.projectID, hash, ext, data, env); err != nil {
		t.Fatal(err)
	}
	answer, err := fx.projects.AudioAnswer(ctx, fx.projectID, story, nil)
	if err != nil {
		t.Fatal(err)
	}
	piece := answer.Narration["s1"].Blocks[0].Pieces[0]
	rep := compileReport{FrameCount: 300, AudioEvents: []reportAudioEvent{{Kind: "narration", Ref: hash, StartMS: 0, SceneID: "s1", BlockID: "s1:n"}}}
	run := &renderRun{r: fx.runner, job: Job{ID: "mix-job"}, plan: &Plan{ProjectID: fx.projectID, Story: *story, Blocks: storymaker.Blocks(story), AudioInfo: answer.Info()}, st: fx.settings, fps: 30, report: rep}
	plan, err := run.planMixInput(ctx)
	if err != nil {
		t.Fatal(err)
	}
	in, err := run.buildMixInput(ctx, plan, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ffStart, found, err := rawFirstSpeechMS(in.Narration[0].Path, goRuns[0][0]+mp3SearchMS)
	if err != nil || !found {
		t.Fatalf("FFmpeg copy: found %v, %v", found, err)
	}
	diff := ffStart - goRuns[0][0]
	t.Logf("first speech: go-mp3 %d ms, FFmpeg %d ms, difference %d ms", goRuns[0][0], ffStart, diff)
	wantIn := piece.InMS
	if diff > mp3ToleranceMS || diff < -mp3ToleranceMS {
		if plan.mp3OffsetMS[hash] != diff {
			t.Errorf("recorded offset %d, measured %d", plan.mp3OffsetMS[hash], diff)
		}
		wantIn = max(0, piece.InMS+diff)
	} else if _, recorded := plan.mp3OffsetMS[hash]; recorded {
		t.Errorf("an offset within the tolerance was recorded: %v", plan.mp3OffsetMS)
	}
	if in.Narration[0].In != int64(wantIn)*48 {
		t.Errorf("clip in = %d, want %d", in.Narration[0].In, int64(wantIn)*48)
	}
}
```

- [ ] **Step 2: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/render -run '(?i)(DecodeArgs|EstimateMix|HasAudio|BuildMixInput)' -count=1
```

Expected: FAIL to build with `undefined: decodeArgs`, `undefined: EstimateMixBytes`, `undefined: HasAudio` and `run.planMixInput undefined`.

- [ ] **Step 3: Implement the input build**

Create `internal/storymaker/render/mixinput.go`:

```go
package render

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"aurago/internal/fileutil"
	"aurago/internal/storymaker"
)

const (
	mixBytesPerSecond = 192000 // 48000 frames, 2 channels, 2 bytes each
	mixBytesPerMS     = 192
	mixFramesPerMS    = MixRate / 1000
	mp3ToleranceMS    = 10
	mp3SearchMS       = 2000 // FFmpeg's copy is searched for speech up to this far past the go-mp3 start
)

// freeDiskBytes and decodeCommand are variables so a test can report a full disk and count decodes.
var (
	freeDiskBytes = fileutil.FreeDiskBytes
	decodeCommand = runDecodeCommand
)

// mixPlan describes the inputs of a mix before any file is copied.
type mixPlan struct {
	filmFrames  int64
	filmSeconds float64
	voices      []plannedVoice
	sfx         []plannedSFX
	sources     map[string]*plannedSource // by piece hash
	order       []string                  // hashes in the order of their first event
	music       *plannedSource
	spans       []Span

	inputBytes   int64
	decodedBytes int64
	stems        bool

	// mp3OffsetMS holds FFmpeg's first speech minus go-mp3's for each stored MP3 beyond the tolerance.
	// buildMixInput fills it, because the value needs the decoded file.
	mp3OffsetMS map[string]int
}

// plannedVoice is one narration or dialogue event.
type plannedVoice struct {
	hash    string
	voice   string
	startMS int64
	inMS    int
	outMS   int
}

// plannedSFX is one sfx event with the take the report named.
type plannedSFX struct {
	take    storymaker.SFXTake
	hit     *storymaker.SFXHit
	startMS int64
}

// plannedSource is one stored audio file to stage and decode.
type plannedSource struct {
	folder     string // narration or music
	name       string // piece hash or music key
	ext        string
	size       int64
	maxSeconds float64
	rawPath    string // set by buildMixInput
	rawFrames  int64  // set by buildMixInput
}

// mixStems says whether this job writes stems. Stems are asked for through the render options,
// which do not exist yet, so no job writes them.
func (run *renderRun) mixStems() bool { return false }

func msToMixFrames(ms int64) int64 { return ms * mixFramesPerMS }

// decodeByteCap is the decode limit: maxSeconds of 48 kHz stereo s16, plus 10 percent.
func decodeByteCap(maxSeconds float64) int64 {
	return int64(maxSeconds*mixBytesPerSecond) * 11 / 10
}

// decodeArgs are the FFmpeg arguments of one decode. The limits stop a hand-replaced low-bitrate
// file from decoding to many GiB.
func decodeArgs(src, dst string, maxSeconds float64) []string {
	return []string{"-hide_banner", "-nostdin", "-loglevel", "error",
		"-protocol_whitelist", "file", "-format_whitelist", "wav,mp3", "-i", src,
		"-map", "0:a:0", "-ar", "48000", "-ac", "2", "-f", "s16le",
		"-t", strconv.FormatFloat(maxSeconds, 'f', 3, 64),
		"-fs", strconv.FormatInt(decodeByteCap(maxSeconds), 10),
		"-y", dst}
}

func runDecodeCommand(ctx context.Context, ffmpeg string, args []string) error {
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	stderr := &tailBuffer{limit: ffmpegStderrKeep}
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil {
		return nil
	}
	if msg := strings.TrimSpace(stderr.String()); msg != "" {
		return errors.New(msg)
	}
	return err
}

// decodeToS16 decodes a staged file to raw s16le at 48 kHz stereo. Its error names files by base name only.
func decodeToS16(ctx context.Context, ffmpeg, src, dst string, maxSeconds float64) error {
	err := decodeCommand(ctx, ffmpeg, decodeArgs(src, dst, maxSeconds))
	if err == nil {
		return nil
	}
	msg := strings.ReplaceAll(err.Error(), src, filepath.Base(src))
	msg = strings.ReplaceAll(msg, dst, filepath.Base(dst))
	msg = strings.ReplaceAll(msg, ffmpeg, "FFmpeg")
	return errors.New(clip(msg, 300))
}

// planMixInput computes clip frames and file sizes and copies nothing.
func (run *renderRun) planMixInput(ctx context.Context) (*mixPlan, error) {
	plan := &mixPlan{
		filmFrames:  int64(math.Round(float64(run.report.FrameCount) * MixRate / float64(run.fps))),
		filmSeconds: float64(run.report.FrameCount) / float64(run.fps),
		sources:     map[string]*plannedSource{},
		stems:       run.mixStems(),
		mp3OffsetMS: map[string]int{},
	}
	blocks := map[string]storymaker.AudioBlock{}
	for _, scene := range run.plan.AudioInfo.Narration {
		for _, b := range scene.Blocks {
			blocks[b.ID] = b
		}
	}
	voices := map[string]string{}
	for _, b := range run.plan.Blocks {
		voices[b.ID] = b.Voice.Provider + "|" + b.Voice.Voice
	}
	placed := map[string]bool{}
	for _, ev := range run.report.AudioEvents {
		if ev.Kind == "sfx" {
			entry, ok := run.r.deps.Catalog.Lookup(ev.Ref)
			if !ok || ev.Take < 0 || ev.Take >= len(entry.Takes) {
				return nil, Fail(CodeRenderFailed, "The compile report names an unknown sound %q.", clip(ev.Ref, 80))
			}
			plan.sfx = append(plan.sfx, plannedSFX{take: entry.Takes[ev.Take], hit: entry.Hit, startMS: ev.StartMS})
			continue
		}
		block, ok := blocks[ev.BlockID]
		if !ok || ev.Piece < 0 || ev.Piece >= len(block.Pieces) {
			return nil, Fail(CodeRenderFailed, "The compile report names an unknown piece of block %q.", clip(ev.BlockID, 80))
		}
		piece := block.Pieces[ev.Piece]
		plan.voices = append(plan.voices, plannedVoice{hash: piece.Hash, voice: voices[block.ID], startMS: ev.StartMS, inMS: piece.InMS, outMS: piece.OutMS})
		if !placed[block.ID] {
			placed[block.ID] = true
			base := ev.StartMS - int64(piece.AtMS)
			for _, sp := range block.Spans {
				plan.spans = append(plan.spans, Span{From: msToMixFrames(base + int64(sp[0])), To: msToMixFrames(base + int64(sp[1]))})
			}
		}
		if plan.sources[piece.Hash] != nil {
			continue
		}
		src, err := run.planSource(ctx, plan, "narration", piece.Hash, piece.Ext, float64(piece.OutMS)/1000+1)
		if err != nil {
			return nil, err
		}
		plan.decodedBytes += int64(piece.OutMS+1000) * mixBytesPerMS
		plan.sources[piece.Hash] = src
		plan.order = append(plan.order, piece.Hash)
	}
	if m := run.plan.Music; m != nil {
		maxSeconds := plan.filmSeconds + 2
		src, err := run.planSource(ctx, plan, "music", m.Key, m.Ext, maxSeconds)
		if err != nil {
			return nil, err
		}
		plan.decodedBytes += decodeByteCap(maxSeconds)
		plan.music = src
	}
	return plan, nil
}

func (run *renderRun) planSource(ctx context.Context, plan *mixPlan, folder, name, ext string, maxSeconds float64) (*plannedSource, error) {
	file := name + "." + ext
	r, info, err := run.r.deps.Projects.OpenAudio(ctx, run.plan.ProjectID, folder, file)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, Fail(CodeRenderFailed, "The stored audio file %s is missing. Generate it again.", file)
	}
	_ = r.Close()
	plan.inputBytes += info.Size
	return &plannedSource{folder: folder, name: name, ext: ext, size: info.Size, maxSeconds: maxSeconds}, nil
}

// buildMixInput stages and decodes the planned files below dir/dec and returns the mixer's input.
// dir is the job's staging folder. The disk check runs first, so a refusal leaves no file behind.
func (run *renderRun) buildMixInput(ctx context.Context, plan *mixPlan, dir string) (*MixInput, error) {
	needed := EstimateMixBytes(plan.decodedBytes, plan.inputBytes, plan.filmFrames, plan.stems) + minFreeStagingByte
	if free, err := freeDiskBytes(dir); err == nil && free < needed {
		return nil, Fail(CodeRenderFailed, "The mix needs %d MB of free disk space, %d MB are free.", (needed+(1<<20)-1)>>20, free>>20)
	}
	dec := filepath.Join(dir, "dec")
	if err := os.MkdirAll(dec, 0o700); err != nil {
		return nil, Fail(CodeRenderFailed, "The mix folder could not be created.")
	}
	for _, hash := range plan.order {
		if err := run.decodeSource(ctx, dec, plan.sources[hash], plan); err != nil {
			return nil, err
		}
	}
	if plan.music != nil {
		if err := run.decodeSource(ctx, dec, plan.music, plan); err != nil {
			return nil, err
		}
	}

	in := &MixInput{Frames: plan.filmFrames, Spans: mergeSpans(plan.spans, plan.filmFrames)}
	for _, v := range plan.voices {
		src := plan.sources[v.hash]
		inMS := v.inMS + plan.mp3OffsetMS[v.hash]
		if inMS < 0 {
			inMS = 0
		}
		clipIn := msToMixFrames(int64(inMS))
		clipLen := msToMixFrames(int64(v.outMS - v.inMS))
		if clipIn >= src.rawFrames {
			continue
		}
		if clipIn+clipLen > src.rawFrames {
			clipLen = src.rawFrames - clipIn
		}
		var fade int64
		if clipIn > 0 || clipIn+clipLen < src.rawFrames {
			fade = clipFadeFrames
		}
		in.Narration = append(in.Narration, MixClip{Key: v.hash, Path: src.rawPath, In: clipIn, Len: clipLen, Start: msToMixFrames(v.startMS), Fade: fade, Voice: v.voice})
	}
	pcm := map[string][]int16{}
	for _, e := range plan.sfx {
		samples, ok := pcm[e.take.File]
		if !ok {
			var err error
			if samples, err = run.readSFX(e.take.File); err != nil {
				return nil, err
			}
			pcm[e.take.File] = samples
		}
		over := 0.0
		if e.hit != nil {
			over = e.hit.OverDB
		}
		start := msToMixFrames(e.startMS)
		in.SFX = append(in.SFX, MixClip{Key: e.take.File, PCM: samples, Len: int64(len(samples) / 2), Start: start, LevelOffsetDB: over - e.take.L50LUFS})
		if e.hit != nil && e.hit.DipDB > 0 {
			at := start + int64(math.Round(e.take.ImpactMS*MixRate/1000))
			if at < plan.filmFrames {
				in.Impacts = append(in.Impacts, Impact{At: at, DepthDB: e.hit.DipDB})
			}
		}
	}
	sort.SliceStable(in.Narration, func(i, j int) bool { return in.Narration[i].Start < in.Narration[j].Start })
	sort.SliceStable(in.SFX, func(i, j int) bool { return in.SFX[i].Start < in.SFX[j].Start })
	sort.SliceStable(in.Impacts, func(i, j int) bool { return in.Impacts[i].At < in.Impacts[j].At })
	if plan.music != nil {
		in.Bed = &MixBed{Path: plan.music.rawPath, Frames: plan.music.rawFrames, Volume: storymaker.MusicVolume(run.plan.Story.Music)}
	}
	return in, nil
}

// decodeSource copies one stored file into dec, so FFmpeg sees only the copy, and decodes it.
func (run *renderRun) decodeSource(ctx context.Context, dec string, src *plannedSource, plan *mixPlan) error {
	file := src.name + "." + src.ext
	staged := filepath.Join(dec, file)
	raw := filepath.Join(dec, src.name+".s16")
	fail := func(step string, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		run.r.deps.Logger.Warn("[Storymaker] A mix input failed", "file", file, "step", step, "error", err)
		return Fail(CodeRenderFailed, "The audio file %s could not be %s for the mix.", file, step)
	}
	if err := run.stageSource(ctx, src, staged); err != nil {
		return fail("copied", err)
	}
	if err := decodeToS16(ctx, run.st.FFmpegPath, staged, raw, src.maxSeconds); err != nil {
		return fail("decoded", err)
	}
	info, err := os.Stat(raw)
	if err != nil {
		return fail("decoded", err)
	}
	if info.Size() < 4 {
		return fail("decoded", errors.New("the decode is empty"))
	}
	src.rawPath, src.rawFrames = raw, info.Size()/4
	if src.folder == "narration" && src.ext == "mp3" {
		offset, err := mp3TimelineOffsetMS(staged, raw)
		if err != nil {
			run.r.deps.Logger.Warn("[Storymaker] An MP3 timeline could not be compared", "file", file, "error", err)
		} else if offset > mp3ToleranceMS || offset < -mp3ToleranceMS {
			plan.mp3OffsetMS[src.name] = offset
		}
	}
	return nil
}

func (run *renderRun) stageSource(ctx context.Context, src *plannedSource, dst string) error {
	r, _, err := run.r.deps.Projects.OpenAudio(ctx, run.plan.ProjectID, src.folder, src.name+"."+src.ext)
	if err != nil {
		return err
	}
	defer r.Close()
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, io.LimitReader(r, src.size))
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// mp3TimelineOffsetMS compares the first speech of a stored MP3 on go-mp3's timeline, which its
// envelope came from, with the same speech in FFmpeg's decode. go-mp3 has no Xing or LAME handling,
// so the two may differ. The result is FFmpeg minus go-mp3, 0 when either side has no speech.
func mp3TimelineOffsetMS(staged, raw string) (int, error) {
	data, err := os.ReadFile(staged)
	if err != nil {
		return 0, err
	}
	decoded, _, err := storymaker.DecodeAudio(data)
	if err != nil {
		return 0, err
	}
	runs := storymaker.ComputeEnvelope(decoded).SpeechRuns()
	if len(runs) == 0 {
		return 0, nil
	}
	start, found, err := rawFirstSpeechMS(raw, runs[0][0]+mp3SearchMS)
	if err != nil || !found {
		return 0, err
	}
	return start - runs[0][0], nil
}

// rawFirstSpeechMS returns where speech starts in the first upToMS of a raw s16le stereo file, by the envelope rule.
func rawFirstSpeechMS(file string, upToMS int) (int, bool, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, false, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, int64(upToMS)*mixBytesPerMS))
	if err != nil {
		return 0, false, err
	}
	samples := make([]float32, len(data)/2)
	for i := range samples {
		samples[i] = float32(int16(binary.LittleEndian.Uint16(data[2*i:]))) / 32768
	}
	runs := storymaker.ComputeEnvelope(storymaker.DecodedAudio{Rate: MixRate, Channels: mixChannels, Samples: samples}).SpeechRuns()
	if len(runs) == 0 {
		return 0, false, nil
	}
	return runs[0][0], true, nil
}

// readSFX reads one catalog take and expands it to stereo.
func (run *renderRun) readSFX(file string) ([]int16, error) {
	data, err := fs.ReadFile(run.r.deps.UI, path.Join("img/storymaker", file))
	if err != nil {
		return nil, Fail(CodeRenderFailed, "The sound %s could not be read.", file)
	}
	samples, err := parseSFXWAV(data)
	if err != nil {
		return nil, Fail(CodeRenderFailed, "The sound %s is not usable: %v", file, err)
	}
	return samples, nil
}

// parseSFXWAV reads a 48 kHz mono PCM16 WAV and returns interleaved stereo.
func parseSFXWAV(data []byte) ([]int16, error) {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, errors.New("it is not a WAV file")
	}
	formatOK := false
	for off := 12; off+8 <= len(data); {
		id := string(data[off : off+4])
		size := int(binary.LittleEndian.Uint32(data[off+4:]))
		body := off + 8
		if size < 0 || body+size > len(data) {
			return nil, errors.New("a chunk is cut off")
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return nil, errors.New("the format chunk is too short")
			}
			tag := binary.LittleEndian.Uint16(data[body:])
			channels := binary.LittleEndian.Uint16(data[body+2:])
			rate := binary.LittleEndian.Uint32(data[body+4:])
			bits := binary.LittleEndian.Uint16(data[body+14:])
			if tag != 1 || channels != 1 || rate != MixRate || bits != 16 {
				return nil, fmt.Errorf("it is format %d, %d channels, %d Hz, %d bits, not 48 kHz mono PCM16", tag, channels, rate, bits)
			}
			formatOK = true
		case "data":
			if !formatOK {
				return nil, errors.New("the data chunk comes before the format chunk")
			}
			frames := size / 2
			out := make([]int16, frames*2)
			for i := 0; i < frames; i++ {
				s := int16(binary.LittleEndian.Uint16(data[body+2*i:]))
				out[2*i], out[2*i+1] = s, s
			}
			return out, nil
		}
		off = body + size + size%2
	}
	return nil, errors.New("it has no data chunk")
}

// mergeSpans clips the spans to the film, sorts them and joins the overlapping ones.
func mergeSpans(spans []Span, frames int64) []Span {
	sorted := append([]Span(nil), spans...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].From < sorted[j].From })
	var out []Span
	for _, s := range sorted {
		if s.To > frames {
			s.To = frames
		}
		if s.From >= s.To {
			continue
		}
		if n := len(out); n > 0 && s.From <= out[n-1].To {
			if s.To > out[n-1].To {
				out[n-1].To = s.To
			}
			continue
		}
		out = append(out, s)
	}
	return out
}
```

- [ ] **Step 4: Implement the two mix functions**

Append to the end of `internal/storymaker/render/mix.go`:

```go
// EstimateMixBytes is the staging space a mix needs: the copied inputs, the decoded files and the
// mix WAV. Stems count twice, because they are staged and then published.
func EstimateMixBytes(decodedBytes, inputBytes, filmFrames int64, stems bool) int64 {
	total := inputBytes + decodedBytes + filmFrames*4
	if stems {
		total += filmFrames * 4 * 3 * 2
	}
	return total
}

// HasAudio says whether a job has anything to mix: a measured block or music in the plan, or an sfx
// event in the report. Narration and music are never decided from the report, so a report cannot
// switch them off. Sfx exist only in the report.
func HasAudio(plan *Plan, rep compileReport) bool {
	if plan.Music != nil {
		return true
	}
	for _, scene := range plan.AudioInfo.Narration {
		if len(scene.Blocks) > 0 {
			return true
		}
	}
	for _, ev := range rep.AudioEvents {
		if ev.Kind == "sfx" {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: Run**

```powershell
go test ./internal/storymaker/render -run '(?i)(DecodeArgs|EstimateMix|HasAudio|BuildMixInput)' -count=1
go test ./internal/storymaker/render -run '^TestBuildMixInputComparesTheMP3Timelines$' -count=1 -v
gofmt -l internal/storymaker
go vet ./internal/storymaker/...
```

Expected: PASS. The `-v` line prints `first speech: go-mp3 <n> ms, FFmpeg <n> ms, difference <n> ms`. Write that line into the commit body, because frozen section 1.2 wants the measured value recorded. A difference above 10 ms is not a failure: `plan.mp3OffsetMS` carries it and the clip starts shift by it.

- [ ] **Step 6: Commit**

Run detect_changes first, under the rule in outline section 5.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/render/mixinput.go internal/storymaker/render/mix.go internal/storymaker/render/mixinput_test.go
git commit -m "feat(storymaker): mix plan, input build and disk check" -m "<the logged first speech line>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A17: FFmpeg arguments, AAC 192k, `ResolveFFprobe`
**Batch:** 2C  **Depends on:** none
**Files:**
Modify: `internal/storymaker/render/ffmpeg.go`
Modify: `internal/videostudio/encoder_export.go`
Modify: `internal/storymaker/render/render.go`
Modify: `internal/server/storymaker_service.go`
Test: `internal/storymaker/render/ffmpeg_test.go`
Test: `internal/storymaker/render/render_test.go`
Test: `internal/videostudio/encoder_export_test.go`

- [ ] **Step 1: Impact**

From the main checkout, run this for `BuildFFmpegArgs`, `FFmpegConfig`, `Message`, `Settings`, `execute`, `storymakerSettings` and `ResolveFFmpeg`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "<symbol>" --direction upstream --repo .
```

Report callers, processes and risk. HIGH, CRITICAL and UNKNOWN follow the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

Append to the end of `internal/storymaker/render/ffmpeg_test.go`. The file's imports already cover these tests.

```go
func TestBuildFFmpegArgsAudioInputs(t *testing.T) {
	base := FFmpegConfig{Path: "ffmpeg", Transport: TransportPNG, Width: 1920, Height: 1080, FPS: 30, Frames: 90, Output: "out.mp4"}
	silent, err := BuildFFmpegArgs(base)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(silent, " ")
	// 90 frames at 30 fps is 3 s.
	for _, want := range []string{
		"-protocol_whitelist file,pipe -format_whitelist lavfi -f lavfi -t 3.000000 -i anullsrc=r=48000:cl=stereo",
		"-c:a aac -b:a 192k -ar 48000 -ac 2",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("silent args lack %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "-shortest") {
		t.Errorf("silent args keep -shortest:\n%s", joined)
	}

	withMix := base
	withMix.AudioPath = "staging/mix.wav"
	mixed, err := BuildFFmpegArgs(withMix)
	if err != nil {
		t.Fatal(err)
	}
	joined = strings.Join(mixed, " ")
	for _, want := range []string{
		"-protocol_whitelist file,pipe -format_whitelist wav -i staging/mix.wav",
		"-map 0:v:0 -map 1:a:0",
		"-c:a aac -b:a 192k -ar 48000 -ac 2",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("mix args lack %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "anullsrc") || strings.Contains(joined, "-shortest") {
		t.Errorf("mix args:\n%s", joined)
	}
}

func TestFFmpegMessageHoldsNoStagingPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "job")
	out := filepath.Join(dir, "out.mp4")
	mix := filepath.Join(dir, "mix.wav")
	f := &FFmpeg{output: out, stderr: &tailBuffer{limit: ffmpegStderrKeep}}
	_, _ = f.stderr.Write([]byte(mix + ": Invalid data\n" + out + ": No such file\n" + filepath.ToSlash(mix) + ": again\n"))
	msg := f.Message()
	if strings.Contains(msg, dir) || strings.Contains(msg, filepath.ToSlash(dir)) {
		t.Fatalf("message holds the staging path: %s", msg)
	}
	if !strings.Contains(msg, "mix.wav") || !strings.Contains(msg, "<output>") {
		t.Fatalf("message lost its content: %s", msg)
	}
}
```

In `internal/storymaker/render/render_test.go`, make these two replacements.

Old:

```go
	fx.settings = Settings{TimeoutSeconds: 60, MaxDurationSeconds: 900, FFmpegPath: ffmpeg, EncoderName: "libx264", Limits: lim}
```

New:

```go
	fx.settings = Settings{TimeoutSeconds: 60, MaxDurationSeconds: 900, FFmpegPath: ffmpeg, FFprobePath: testBinary(t, "ffprobe"), EncoderName: "libx264", Limits: lim}
```

Old:

```go
	fx = newRunFixture(t)
	fx.deps.Browsers.lookPath = func() (string, bool) { return "", false }
```

New:

```go
	fx = newRunFixture(t)
	fx.settings.FFprobePath = ""
	fx.playing(playOptions{frames: 10})
	if job := fx.render(t); job.Status != StatusFailed || job.Error.Code != CodeFFmpegUnavailable {
		t.Fatalf("no ffprobe: %+v %+v", job, job.Error)
	}

	fx = newRunFixture(t)
	fx.deps.Browsers.lookPath = func() (string, bool) { return "", false }
```

In `internal/videostudio/encoder_export_test.go`, replace:

Old:

```go
import (
	"path/filepath"
```

New:

```go
import (
	"os"
	"path/filepath"
```

Then append to the end of the same file:

```go
func TestResolveFFprobeFindsTheBinaryBesideFFmpeg(t *testing.T) {
	dir := t.TempDir()
	probe := filepath.Join(dir, "ffprobe.exe")
	if err := os.WriteFile(probe, []byte("stand-in"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveFFprobe(filepath.Join(dir, "ffmpeg.exe"))
	if err != nil || got != probe {
		t.Fatalf("ResolveFFprobe = %q, %v, want %q", got, err, probe)
	}
}
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/render ./internal/videostudio -run '(?i)(BuildFFmpegArgs|FFmpegMessage|ReportsCapability|ResolveFFprobe)' -count=1
```

Expected: FAIL to build. The render package reports `withMix.AudioPath undefined` and `unknown field FFprobePath in struct literal`. The videostudio package reports `undefined: ResolveFFprobe`.

- [ ] **Step 4: Implement in `ffmpeg.go`**

In `internal/storymaker/render/ffmpeg.go`, make these replacements.

Old:

```go
	"os/exec"
	"strconv"
```

New:

```go
	"os/exec"
	"path/filepath"
	"strconv"
```

Old:

```go
	silentAudioRate     = 48000
	silentAudioBitrate  = "64k"
	silentAudioLavfiSrc = "anullsrc=r=48000:cl=stereo"
```

New:

```go
	audioRate           = 48000
	audioBitrate        = "192k"
	silentAudioLavfiSrc = "anullsrc=r=48000:cl=stereo"
```

Old:

```go
	Frames    int
	Output    string
}
```

New:

```go
	Frames    int
	Output    string
	AudioPath string // "" keeps anullsrc with -t; else the mix WAV as input 1
}
```

Old:

```go
// BuildFFmpegArgs returns the FFmpeg arguments of a render: frames on stdin (video), a silent
// AAC track from anullsrc (so the MP4 has the stream layout later phases mix into), BT.709
```

New:

```go
// BuildFFmpegArgs returns the FFmpeg arguments of a render: frames on stdin (video), the mix WAV or
// a silent track from anullsrc as the AAC audio, BT.709
```

Old:

```go
	seconds := strconv.FormatFloat(float64(cfg.Frames)/float64(cfg.FPS), 'f', 6, 64)
	args = append(args, "-protocol_whitelist", "file,pipe", "-format_whitelist", "lavfi",
		"-f", "lavfi", "-t", seconds, "-i", silentAudioLavfiSrc)
```

New:

```go
	if cfg.AudioPath != "" {
		args = append(args, "-protocol_whitelist", "file,pipe", "-format_whitelist", "wav", "-i", cfg.AudioPath)
	} else {
		seconds := strconv.FormatFloat(float64(cfg.Frames)/float64(cfg.FPS), 'f', 6, 64)
		args = append(args, "-protocol_whitelist", "file,pipe", "-format_whitelist", "lavfi",
			"-f", "lavfi", "-t", seconds, "-i", silentAudioLavfiSrc)
	}
```

Old:

```go
	args = append(args, "-c:a", "aac", "-b:a", silentAudioBitrate, "-ar", strconv.Itoa(silentAudioRate), "-ac", "2",
		"-shortest", "-movflags", "+faststart", "-f", "mp4", "-y", cfg.Output)
```

New:

```go
	args = append(args, "-c:a", "aac", "-b:a", audioBitrate, "-ar", strconv.Itoa(audioRate), "-ac", "2",
		"-movflags", "+faststart", "-f", "mp4", "-y", cfg.Output)
```

Old:

```go
// Message is the tail of FFmpeg's error output with the output path scrubbed.
func (f *FFmpeg) Message() string {
	text := strings.TrimSpace(f.stderr.String())
	if f.output != "" {
		text = strings.ReplaceAll(text, f.output, "<output>")
	}
	return clip(text, 600)
}
```

New:

```go
// Message is the tail of FFmpeg's error output with the output path and the staging folder scrubbed.
func (f *FFmpeg) Message() string {
	text := strings.TrimSpace(f.stderr.String())
	if f.output != "" {
		text = strings.ReplaceAll(text, f.output, "<output>")
		text = scrubStaging(text, filepath.Dir(f.output))
	}
	return clip(text, 600)
}

// scrubStaging removes the staging folder from a tool's message, in both slash forms, so no
// absolute path reaches a job error.
func scrubStaging(text, dir string) string {
	if dir == "" || dir == "." {
		return text
	}
	text = strings.ReplaceAll(text, dir, "<staging>")
	return strings.ReplaceAll(text, filepath.ToSlash(dir), "<staging>")
}
```

- [ ] **Step 5: Implement `ResolveFFprobe` and the settings**

In `internal/videostudio/encoder_export.go`, replace:

Old:

```go
	return resolveBinary(configuredPath, "ffmpeg")
}
```

New:

```go
	return resolveBinary(configuredPath, "ffmpeg")
}

// ResolveFFprobe resolves the FFprobe binary that goes with a resolved FFmpeg path: the adjacent
// binary, else PATH.
func ResolveFFprobe(ffmpegPath string) (string, error) {
	return resolveProbeBinary(ffmpegPath)
}
```

In `internal/storymaker/render/render.go`, make these two replacements.

Old:

```go
	FFmpegPath         string
```

New:

```go
	FFmpegPath         string
	FFprobePath        string // empty fails the job with ffmpeg_unavailable
```

Old:

```go
	if run.st.FFmpegPath == "" {
		return Output{}, Fail(CodeFFmpegUnavailable, "FFmpeg was not found.")
	}
```

New:

```go
	if run.st.FFmpegPath == "" {
		return Output{}, Fail(CodeFFmpegUnavailable, "FFmpeg was not found.")
	}
	if run.st.FFprobePath == "" {
		return Output{}, Fail(CodeFFmpegUnavailable, "FFprobe was not found.")
	}
```

In `internal/server/storymaker_service.go`, make these two replacements.

Old:

```go
	enc := videoStudioEncoder(ctx, cfg)
	return render.Settings{
```

New:

```go
	ffprobe, err := videostudio.ResolveFFprobe(ffmpeg)
	if err != nil {
		return render.Settings{}, fmt.Errorf("FFprobe is not available: %v", err)
	}
	enc := videoStudioEncoder(ctx, cfg)
	return render.Settings{
```

Old:

```go
		FFmpegPath:         ffmpeg,
```

New:

```go
		FFmpegPath:         ffmpeg,
		FFprobePath:        ffprobe,
```

- [ ] **Step 6: Run**

```powershell
go test ./internal/storymaker/render -count=1 -timeout 20m
go test ./internal/videostudio -run ResolveFFprobe -count=1
go test ./internal/server -run '(?i)(StorymakerSettings|StorymakerRenderEndpoint)' -count=1 -timeout 15m
gofmt -l internal/storymaker internal/server internal/videostudio
go vet ./internal/storymaker/... ./internal/server ./internal/videostudio
```

Expected: PASS, and `gofmt -l` prints nothing.

- [ ] **Step 7: Commit**

Run detect_changes first, under the rule in outline section 5.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/render/ffmpeg.go internal/videostudio/encoder_export.go internal/storymaker/render/render.go internal/server/storymaker_service.go internal/storymaker/render/ffmpeg_test.go internal/storymaker/render/render_test.go internal/videostudio/encoder_export_test.go
git commit -m "feat(storymaker): mix input, AAC 192k and FFprobe resolution for the encode" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A18: Frame gate in `finish()`
**Batch:** 2C  **Depends on:** A17
**Files:**
Create: `internal/storymaker/render/gate.go`
Modify: `internal/storymaker/render/render.go`
Modify: `internal/storymaker/render/ffmpeg.go`
Test: `internal/storymaker/render/gate_test.go`
Test: `internal/storymaker/render/ffmpeg_test.go`
Test: `internal/storymaker/render/ingest_test.go`

- [ ] **Step 1: Impact**

From the main checkout, run this for `finish` and `Wait`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "finish" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "Wait" --direction upstream --repo .
```

`Wait` is a common name, so read the answer for the `FFmpeg` method in `internal/storymaker/render/ffmpeg.go`. A text search for its callers finds `finish`, `encodeFrames` in `internal/storymaker/render/ffmpeg_test.go` and one test in `internal/storymaker/render/ingest_test.go`.

Report callers, processes and risk. HIGH, CRITICAL and UNKNOWN follow the GitNexus rule in outline section 5.

- [ ] **Step 2: Write the failing test**

Create `internal/storymaker/render/gate_test.go`:

```go
package render

import (
	"context"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckOutputRefusesWrongCountsAndExtraAudio(t *testing.T) {
	// Packets must equal the frames, and audio must lie within 25 ms of video.
	good := ProbeResult{VideoPackets: 90, VideoSeconds: 3, AudioSeconds: 3.01}
	if err := CheckOutput(good, 90); err != nil {
		t.Errorf("a good file was refused: %v", err)
	}
	if err := CheckOutput(good, 91); err == nil {
		t.Error("a wrong packet count passed")
	}
	if err := CheckOutput(ProbeResult{VideoPackets: 90, VideoSeconds: 3, AudioSeconds: 4}, 90); err == nil {
		t.Error("1 s of extra audio passed")
	}
	if err := CheckOutput(ProbeResult{VideoPackets: 90, VideoSeconds: 3, AudioSeconds: 2.9}, 90); err == nil {
		t.Error("audio 100 ms short passed")
	}
}

func TestProbeOutputHasBothWhitelistsAndItsBounds(t *testing.T) {
	orig := runGateProbe
	t.Cleanup(func() { runGateProbe = orig })
	var calls []string
	runGateProbe = func(ctx context.Context, path string, args []string, limit int) ([]byte, error) {
		deadline, ok := ctx.Deadline()
		// The timeout is the caller's: 150 s is not the 120 s floor.
		if !ok || time.Until(deadline) > 150*time.Second || time.Until(deadline) < 130*time.Second {
			t.Errorf("deadline = %v, %v", deadline, ok)
		}
		if limit != 8<<20 {
			t.Errorf("output cap = %d", limit)
		}
		joined := strings.Join(args, " ")
		calls = append(calls, joined)
		switch {
		case strings.Contains(joined, "-count_packets"):
			return []byte("90\n"), nil
		case strings.Contains(joined, "v:0"):
			return []byte("3.000000\n"), nil
		}
		return []byte("3.000400\n"), nil
	}
	got, err := ProbeOutput(context.Background(), "ffprobe", "out.mp4", 150*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got.VideoPackets != 90 || got.VideoSeconds != 3 || got.AudioSeconds != 3.0004 {
		t.Errorf("result = %+v", got)
	}
	want := []string{
		"-v error -protocol_whitelist file -format_whitelist mov -count_packets -select_streams v:0 -show_entries stream=nb_read_packets -of csv=p=0 -i out.mp4",
		"-v error -protocol_whitelist file -format_whitelist mov -select_streams v:0 -show_entries stream=duration -of csv=p=0 -i out.mp4",
		"-v error -protocol_whitelist file -format_whitelist mov -select_streams a:0 -show_entries stream=duration -of csv=p=0 -i out.mp4",
	}
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("calls =\n%s\nwant\n%s", strings.Join(calls, "\n"), strings.Join(want, "\n"))
	}
}

func TestProbeOutputReadsARealFile(t *testing.T) {
	ffmpeg, ffprobe := testBinary(t, "ffmpeg"), testBinary(t, "ffprobe")
	const w, h, fps, n = 160, 90, 30, 45
	dir := t.TempDir()
	out := filepath.Join(dir, "out.mp4")
	red := solidPNG(t, w, h, color.NRGBA{R: 255, A: 255})
	frames := make([][]byte, n)
	for i := range frames {
		frames[i] = red
	}
	ff, err := encodeFrames(t, FFmpegConfig{Path: ffmpeg, Transport: TransportPNG, Width: w, Height: h, FPS: fps, Frames: n, Output: out}, frames)
	if err != nil {
		t.Fatalf("encode: %v (%s)", err, ff.Message())
	}
	got, err := ProbeOutput(context.Background(), ffprobe, out, postEncodeMinLimit)
	if err != nil {
		t.Fatal(err)
	}
	if got.VideoPackets != n {
		t.Errorf("packets = %d, want %d", got.VideoPackets, n)
	}
	if err := CheckOutput(got, n); err != nil {
		t.Errorf("a normal encode was refused: %v (%+v)", err, got)
	}

	bad := filepath.Join(dir, "bad.mp4")
	if err := os.WriteFile(bad, []byte("this is not a video"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = ProbeOutput(context.Background(), ffprobe, bad, postEncodeMinLimit)
	if err == nil {
		t.Fatal("a broken file passed the probe")
	}
	if strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), filepath.ToSlash(dir)) {
		t.Errorf("the error holds the staging path: %v", err)
	}
}

func TestPostEncodeLimitScalesWithTheFileSize(t *testing.T) {
	// The larger of 120 s and the size at 10 MiB/s: they meet at 1200 MiB, and 2 GiB, the largest file allowed, gets 204.8 s.
	for _, c := range []struct {
		size int64
		want time.Duration
	}{
		{0, 120 * time.Second},
		{1200 << 20, 120 * time.Second},
		{1500 << 20, 150 * time.Second},
		{maxOutputBytes, 204800 * time.Millisecond},
		{maxOutputBytes * 8, 204800 * time.Millisecond},
	} {
		if got := postEncodeLimit(c.size); got != c.want {
			t.Errorf("postEncodeLimit(%d) = %v, want %v", c.size, got, c.want)
		}
	}
}

func TestFFmpegWaitEndsAtTheGivenLimit(t *testing.T) {
	ffmpeg := testBinary(t, "ffmpeg")
	ff, err := StartFFmpeg(context.Background(), FFmpegConfig{Path: ffmpeg, Transport: TransportPNG, Width: 64, Height: 64, FPS: 30, Frames: 5, Output: filepath.Join(t.TempDir(), "x.mp4")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ff.Discard)
	// The frame input stays open, so FFmpeg cannot end by itself and only the limit stops the wait.
	start := time.Now()
	err = ff.Wait(context.Background(), 300*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "did not finish") {
		t.Fatalf("wait = %v", err)
	}
	if took := time.Since(start); took > 30*time.Second {
		t.Errorf("the wait took %v", took)
	}
}

func TestRunPublishesNothingWhenTheFrameGateRefuses(t *testing.T) {
	orig := runGateProbe
	t.Cleanup(func() { runGateProbe = orig })
	runGateProbe = func(ctx context.Context, path string, args []string, limit int) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "-count_packets") {
			// One packet short of the 24 frames the page sends.
			return []byte("23\n"), nil
		}
		return orig(ctx, path, args, limit)
	}
	fx := newRunFixture(t)
	fx.playing(playOptions{frames: 24})
	job := fx.render(t)
	if job.Status != StatusFailed || job.Error == nil || job.Error.Code != CodeRenderFailed || !strings.Contains(job.Error.Message, "23") {
		t.Fatalf("job = %+v error %+v", job, job.Error)
	}
	if names := fx.pub.names(); len(names) != 0 {
		t.Errorf("a refused file published %v", names)
	}
}
```

The existing tests call `FFmpeg.Wait`, which now takes its limit as an argument.

In `internal/storymaker/render/ffmpeg_test.go`, replace:

Old:

```go
	return ff, ff.Wait(context.Background())
```

New:

```go
	return ff, ff.Wait(context.Background(), postEncodeMinLimit)
```

In `internal/storymaker/render/ingest_test.go`, replace:

Old:

```go
	if err := soft.Wait(context.Background()); err != nil {
```

New:

```go
	if err := soft.Wait(context.Background(), postEncodeMinLimit); err != nil {
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/render -run '(?i)(CheckOutput|ProbeOutput|FrameGate|PostEncodeLimit|FFmpegWait)' -count=1
```

Expected: FAIL to build. The compiler stops at ten errors, so the output shows some of `undefined: ProbeResult`, `undefined: CheckOutput`, `undefined: ProbeOutput`, `undefined: runGateProbe`, `undefined: postEncodeLimit`, `undefined: postEncodeMinLimit` and `too many arguments in call to ff.Wait`.

- [ ] **Step 4: Implement the gate**

Create `internal/storymaker/render/gate.go`:

```go
package render

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/mediabins"
)

const (
	postEncodeMinLimit       = 120 * time.Second
	postEncodeBytesPerSecond = 10 << 20
	gateProbeOutputCap       = 8 << 20
	gateAudioDelta           = 0.025 // seconds
)

// postEncodeLimit is the time FFmpeg's last step and the frame gate each get for a file of this size: 120 s,
// or the size at 10 MiB/s when that is longer. The +faststart rewrite and -count_packets go through the whole
// file, so a fixed limit would throw away a long film on a slow disk after hours of rendering. A file over
// maxOutputBytes is refused anyway, so it gets no more time, and the single worker stays bounded.
func postEncodeLimit(size int64) time.Duration {
	if size > maxOutputBytes {
		size = maxOutputBytes
	}
	limit := time.Duration(size) * time.Second / postEncodeBytesPerSecond
	if limit < postEncodeMinLimit {
		return postEncodeMinLimit
	}
	return limit
}

// runGateProbe runs ffprobe; a variable so a test can read the bounds and force an answer.
var runGateProbe = mediabins.Run

// ProbeResult is what the frame gate reads from a finished MP4.
type ProbeResult struct {
	VideoPackets               int
	VideoSeconds, AudioSeconds float64
}

// ProbeOutput reads the packet count and the stream durations of a finished MP4 within timeout, which covers
// all three runs. finish runs after wait with no overall timer, so the timeout and the output cap keep ffprobe
// from holding the single worker.
func ProbeOutput(ctx context.Context, ffprobe, file string, timeout time.Duration) (ProbeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	run := func(args ...string) (string, error) {
		full := append([]string{"-v", "error", "-protocol_whitelist", "file", "-format_whitelist", "mov"}, args...)
		full = append(full, "-of", "csv=p=0", "-i", file)
		out, err := runGateProbe(ctx, ffprobe, full, gateProbeOutputCap)
		if err != nil {
			msg := strings.ReplaceAll(err.Error(), ffprobe, "ffprobe")
			return "", fmt.Errorf("ffprobe: %s", scrubStaging(msg, filepath.Dir(file)))
		}
		return firstCSVField(out), nil
	}
	var result ProbeResult
	packets, err := run("-count_packets", "-select_streams", "v:0", "-show_entries", "stream=nb_read_packets")
	if err != nil {
		return ProbeResult{}, err
	}
	if result.VideoPackets, err = strconv.Atoi(packets); err != nil {
		return ProbeResult{}, fmt.Errorf("ffprobe reported the packet count %q", clip(packets, 40))
	}
	if result.VideoSeconds, err = probeStreamSeconds(run, "v:0"); err != nil {
		return ProbeResult{}, err
	}
	if result.AudioSeconds, err = probeStreamSeconds(run, "a:0"); err != nil {
		return ProbeResult{}, err
	}
	return result, nil
}

func probeStreamSeconds(run func(args ...string) (string, error), stream string) (float64, error) {
	text, err := run("-select_streams", stream, "-show_entries", "stream=duration")
	if err != nil {
		return 0, err
	}
	seconds, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 {
		return 0, fmt.Errorf("ffprobe reported the duration %q for stream %s", clip(text, 40), stream)
	}
	return seconds, nil
}

// firstCSVField returns the first field of the first line of ffprobe's csv output.
func firstCSVField(out []byte) string {
	line := strings.TrimSpace(string(out))
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}
	if i := strings.IndexByte(line, ','); i >= 0 {
		line = line[:i]
	}
	return strings.TrimSpace(line)
}

// CheckOutput is the frame gate: the video packets equal the frames, and the audio lies within 25 ms
// of the video, because the packet count does not notice a wrong audio length.
func CheckOutput(r ProbeResult, frames int) error {
	if r.VideoPackets != frames {
		return fmt.Errorf("the file has %d video packets, the compile report has %d frames", r.VideoPackets, frames)
	}
	if delta := math.Abs(r.AudioSeconds - r.VideoSeconds); delta >= gateAudioDelta {
		return fmt.Errorf("the audio and the video differ by %.0f ms", delta*1000)
	}
	return nil
}
```

- [ ] **Step 5: Implement the limit on `FFmpeg.Wait` and the calls in `finish()`**

In `internal/storymaker/render/ffmpeg.go`, make these two replacements.

Old:

```go
	ffmpegWriteTimeout = 30 * time.Second
	ffmpegWaitAfterEnd = 120 * time.Second
	ffmpegStderrKeep   = 8 << 10
```

New:

```go
	ffmpegWriteTimeout = 30 * time.Second
	ffmpegStderrKeep   = 8 << 10
```

Old:

```go
// Wait waits for the process to end after Finish and returns its error with FFmpeg's message.
func (f *FFmpeg) Wait(ctx context.Context) error {
	timer := time.NewTimer(ffmpegWaitAfterEnd)
```

New:

```go
// Wait waits at most limit for the process to end after Finish and returns its error with FFmpeg's message.
func (f *FFmpeg) Wait(ctx context.Context, limit time.Duration) error {
	timer := time.NewTimer(limit)
```

In `internal/storymaker/render/render.go`, make these two replacements.

Old:

```go
	if err := ff.Wait(ctx); err != nil {
```

New:

```go
	// The file is nearly complete here. What is left is the encoder's tail and the +faststart rewrite of all of it.
	// A failed stat leaves the 120 s floor, and the check below reports a missing file.
	var written int64
	if partial, err := os.Stat(run.output); err == nil {
		written = partial.Size()
	}
	if err := ff.Wait(ctx, postEncodeLimit(written)); err != nil {
```

Old:

```go
	if info.Size() > maxOutputBytes {
		return Output{}, Fail(CodeRenderFailed, "The video is larger than %d GB.", maxOutputBytes>>30)
	}

	h.SetPhase(PhaseWriting)
```

New:

```go
	if info.Size() > maxOutputBytes {
		return Output{}, Fail(CodeRenderFailed, "The video is larger than %d GB.", maxOutputBytes>>30)
	}
	probe, err := ProbeOutput(ctx, run.st.FFprobePath, run.output, postEncodeLimit(info.Size()))
	if err != nil {
		if ctx.Err() != nil {
			return Output{}, ctx.Err()
		}
		return Output{}, Fail(CodeRenderFailed, "The video could not be checked: %v", err)
	}
	if err := CheckOutput(probe, run.report.FrameCount); err != nil {
		return Output{}, Fail(CodeRenderFailed, "The video failed the frame check: %v", err)
	}

	h.SetPhase(PhaseWriting)
```

- [ ] **Step 6: Run**

```powershell
go test ./internal/storymaker/render -run '(?i)(CheckOutput|ProbeOutput|FrameGate|PostEncodeLimit|FFmpegWait)' -count=1
go test ./internal/storymaker/render -count=1 -timeout 20m
gofmt -l internal/storymaker
go vet ./internal/storymaker/...
```

Expected: PASS, and `gofmt -l` prints nothing. Every `TestRun*` test now passes through the gate, so the silent `anullsrc` track is checked against the 25 ms rule on each of them.

- [ ] **Step 7: Commit**

Run detect_changes first, under the rule in outline section 5.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/render/gate.go internal/storymaker/render/render.go internal/storymaker/render/ffmpeg.go internal/storymaker/render/gate_test.go internal/storymaker/render/ffmpeg_test.go internal/storymaker/render/ingest_test.go
git commit -m "feat(storymaker): frame gate before an export is published" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

<!-- G-A-5 -->

### Task A19: Deferred start. Wire: Go+JS

**Batch:** 2C  **Depends on:** N18, A17

**Files:**
- Modify: `internal/storymaker/render/render.go`
- Modify: `internal/storymaker/render/listener.go`
- Modify: `ui/js/storymaker/host-render.js`
- Modify (generated by `npm run build:ui`): `ui/js/storymaker/host.bundle.js`
- Modify (generated by `npm run build:ui`): `ui/js/storymaker/player.bundle.js`
- Test: `internal/storymaker/render/render_test.go`
- Test: `internal/storymaker/render/listener_test.go`
- Test: `scripts/storymaker-tests/09-host-render.mjs`

- [ ] **Step 1: Impact**

Run this from the main checkout. Per outline section 5: report callers, processes and risk, warn on HIGH or CRITICAL, confirm UNKNOWN with a text search.

```powershell
foreach ($symbol in 'onReport','wait','execute','ReportAnswer','Hooks','serve','count','mountRender') { node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact $symbol --direction upstream --repo . }
```

- [ ] **Step 2: Write the failing test**

Append to the end of `internal/storymaker/render/listener_test.go`. It uses the real `onStart` as the hook, so this file needs no new import.

```go
// /start needs the token, answers 404 until a report was accepted, holds at most startHold and then
// answers pending, start or the failure of the start.
func TestListenerDeferredStartRoute(t *testing.T) {
	old := startHold
	startHold = 150 * time.Millisecond
	t.Cleanup(func() { startHold = old })
	const token = "0123456789abcdef0123456789abcdef"
	listen := func(run *renderRun) (*listenerFixture, string) {
		t.Helper()
		prefix, err := NewPrefix()
		if err != nil {
			t.Fatal(err)
		}
		l, err := StartListener(prefix, &Snapshot{}, PageParams{Width: 1920, Height: 1080, FPS: 30, Transport: TransportPNG, FrameLimit: 900, Token: token}, Hooks{Start: run.onStart})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = l.Close() })
		client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
		return &listenerFixture{l: l, client: client}, l.Base() + "/start"
	}

	run := &renderRun{startCh: make(chan struct{})}
	fx, start := listen(run)
	if res, body := fx.do(t, http.MethodGet, start+"?t=wrong", nil, nil); res.StatusCode != http.StatusForbidden || !strings.Contains(string(body), `"error":"bad_token"`) {
		t.Errorf("wrong token = %d %s", res.StatusCode, body)
	}
	if res, _ := fx.do(t, http.MethodPost, start+"?t="+token, nil, nil); res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST /start = %d", res.StatusCode)
	}
	if res, _ := fx.do(t, http.MethodGet, start+"?t="+token, nil, nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("/start before a report = %d, want 404", res.StatusCode)
	}

	// A report was accepted and the start still runs: the call holds, then answers pending.
	run.mu.Lock()
	run.frames = 30
	run.mu.Unlock()
	began := time.Now()
	res, body := fx.do(t, http.MethodGet, start+"?t="+token, nil, nil)
	if res.StatusCode != http.StatusOK || string(body) != `{"pending":true}` {
		t.Errorf("held /start = %d %s", res.StatusCode, body)
	}
	if held := time.Since(began); held < startHold {
		t.Errorf("/start answered after %s, it must hold %s", held, startHold)
	}

	// A hold far longer than the client's 5 s timeout: the answer comes when the start is done, not at the end of the hold.
	startHold = 10 * time.Second
	time.AfterFunc(50*time.Millisecond, func() { close(run.startCh) })
	res, body = fx.do(t, http.MethodGet, start+"?t="+token, nil, nil)
	if res.StatusCode != http.StatusOK || string(body) != `{"start":true}` {
		t.Errorf("/start after the start = %d %s", res.StatusCode, body)
	}
	if _, body := fx.do(t, http.MethodGet, start+"?t="+token, nil, nil); string(body) != `{"start":true}` {
		t.Errorf("a repeated /start = %s", body)
	}
	if fx.l.Counts()["GET start"] == 0 {
		t.Errorf("counts = %v, want the route start", fx.l.Counts())
	}

	failed := &renderRun{startCh: make(chan struct{}), frames: 30, startErr: Fail(CodeFFmpegUnavailable, "no encoder")}
	close(failed.startCh)
	fx2, start2 := listen(failed)
	res, body = fx2.do(t, http.MethodGet, start2+"?t="+token, nil, nil)
	if res.StatusCode != http.StatusOK || string(body) != `{"error":"ffmpeg_unavailable","message":"no encoder"}` {
		t.Errorf("failed start = %d %s", res.StatusCode, body)
	}
}
```

In `internal/storymaker/render/render_test.go`, add the import the panic test below needs.

Old text:

```go
	"io"
	"net/http"
```

New text:

```go
	"io"
	"log/slog"
	"net/http"
```

Then, in the same file, teach the fake page to poll. N18 does not touch these lines.

Old text:

```go
	onRefused   func(answer ReportAnswer)
```

New text:

```go
	onRefused   func(answer ReportAnswer)
	onAnswer    func(answer ReportAnswer)
```

Old text:

```go
			if !answer.Start {
				if o.onRefused != nil {
					o.onRefused(answer)
				}
				return
			}
```

New text:

```go
			if o.onAnswer != nil {
				o.onAnswer(answer)
			}
			if !answer.Start && !answer.Pending {
				if o.onRefused != nil {
					o.onRefused(answer)
				}
				return
			}
			if answer.Pending {
				started := waitForStart(ctx, base, fakePageBootToken(bootBytes))
				if !started.Start {
					if o.onRefused != nil {
						o.onRefused(ReportAnswer{Error: started.Error, Message: started.Message})
					}
					return
				}
			}
```

Append to the end of `internal/storymaker/render/render_test.go`:

```go
// fakePageBootToken reads the listener token from the boot document the fake page fetched.
func fakePageBootToken(bootBytes []byte) string {
	var doc struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(bootBytes, &doc)
	return doc.Token
}

// waitForStart long-polls /start like the page does. It returns on any answer but pending, or when the job ended.
func waitForStart(ctx context.Context, base, token string) StartAnswer {
	for ctx.Err() == nil {
		out, status, err := httpDo(ctx, "GET", base+"/start?t="+token, "", nil, nil)
		if err != nil || status != 200 {
			return StartAnswer{Error: "start_unreachable"}
		}
		var a StartAnswer
		_ = json.Unmarshal(out, &a)
		if !a.Pending {
			return a
		}
	}
	return StartAnswer{Error: "cancelled"}
}

// enqueueRender queues a render of the fixture's project and returns the job at once.
func (fx *runFixture) enqueueRender(t *testing.T) Job {
	t.Helper()
	plan, err := fx.runner.Prepare(context.Background(), fx.projectID, "", fx.settings)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	job, _, err := fx.mgr.Enqueue(EnqueueRequest{ProjectID: fx.projectID, Payload: plan})
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func TestRunDeferredStartAnswersPendingAndHoldsTheWatchdog(t *testing.T) {
	old := noFrameWatchdog
	noFrameWatchdog = 600 * time.Millisecond
	t.Cleanup(func() { noFrameWatchdog = old })
	fx := newRunFixture(t)
	release := make(chan struct{})
	fx.deps.mixHook = func(ctx context.Context, _ *renderRun) error {
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	fx.rebuild(t)
	answers := make(chan ReportAnswer, 1)
	fx.playing(playOptions{frames: 12, onAnswer: func(a ReportAnswer) { answers <- a }})
	job := fx.enqueueRender(t)
	select {
	case a := <-answers:
		if a.Start || !a.Pending || a.Error != "" {
			t.Fatalf("the report answer = %+v, want pending", a)
		}
	case <-time.After(time.Minute):
		t.Fatal("the report was not answered while the start was held")
	}
	// Twice the watchdog passes inside the mix. The job still runs, because the watchdog starts with FFmpeg.
	time.Sleep(2 * noFrameWatchdog)
	if j, _ := fx.mgr.Get(job.ID); j.Status != StatusRunning || j.Phase != PhaseMixing {
		t.Fatalf("while the mix holds: %+v %+v", j, j.Error)
	}
	close(release)
	done := waitJobFor(t, fx.mgr, job.ID, terminal, 2*time.Minute)
	if done.Status != StatusSucceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
}

// No report means nothing was added to mixWG, so the wait at the end of execute returns at once.
func TestRunDeferredStartReportTimeoutEndsAtOnce(t *testing.T) {
	old := reportWait
	reportWait = 200 * time.Millisecond
	t.Cleanup(func() { reportWait = old })
	fx := newRunFixture(t)
	fx.page.script = func(ctx context.Context, _ string) { <-ctx.Done() }
	started := time.Now()
	job := fx.render(t)
	if job.Status != StatusFailed || job.Error == nil || job.Error.Code != CodeRenderFailed || !strings.Contains(job.Error.Message, "did not report") {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	if took := time.Since(started); took > 20*time.Second {
		t.Fatalf("the job took %s to end", took)
	}
}

func TestRunDeferredStartCancelBeforeTheReportEndsAtOnce(t *testing.T) {
	fx := newRunFixture(t)
	opened := make(chan struct{})
	fx.page.script = func(ctx context.Context, _ string) {
		close(opened)
		<-ctx.Done()
	}
	job := fx.enqueueRender(t)
	select {
	case <-opened:
	case <-time.After(time.Minute):
		t.Fatal("the page was never opened")
	}
	fx.mgr.Cancel(job.ID)
	done := waitJob(t, fx.mgr, job.ID, terminal)
	if done.Status != StatusCancelled {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
}

// A panic on the start goroutine is out of reach of Manager.safeRun, so without a recover here one project's stored audio
// could end the whole server. The job fails with the fixed text, the log keeps the panic, and /start gets its answer.
func TestRunDeferredStartPanicFailsTheJob(t *testing.T) {
	fx := newRunFixture(t)
	var logged bytes.Buffer
	fx.deps.Logger = slog.New(slog.NewTextHandler(&logged, nil))
	var held atomic.Pointer[renderRun]
	fx.deps.mixHook = func(_ context.Context, run *renderRun) error {
		held.Store(run)
		panic("mix index out of range")
	}
	fx.rebuild(t)
	fx.playing(playOptions{frames: 12})
	job := fx.render(t)
	if job.Status != StatusFailed || job.Error == nil || job.Error.Code != CodeRenderFailed || job.Error.Message != "The render stopped unexpectedly." {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	if !strings.Contains(logged.String(), "mix index out of range") {
		t.Errorf("the log does not name the panic: %s", logged.String())
	}
	run := held.Load()
	select {
	case <-run.startCh:
		if run.startErr == nil || run.startErr.Code != CodeRenderFailed {
			t.Errorf("startErr = %+v", run.startErr)
		}
	default:
		t.Error("startCh is still open, so /start would answer pending until the job ended")
	}
}

// wait returns on a crash with the job ctx still live, so only cancelMix can stop the mix.
func TestRunDeferredStartPageCrashStartsNoFFmpeg(t *testing.T) {
	fx := newRunFixture(t)
	var held atomic.Pointer[renderRun]
	inMix := make(chan struct{})
	fx.deps.mixHook = func(ctx context.Context, run *renderRun) error {
		held.Store(run)
		close(inMix)
		<-ctx.Done() // the real mix checks the same ctx at each block
		return ctx.Err()
	}
	fx.rebuild(t)
	fx.playing(playOptions{frames: 12})
	job := fx.enqueueRender(t)
	select {
	case <-inMix:
	case <-time.After(time.Minute):
		t.Fatal("the mix never started")
	}
	started := time.Now()
	close(fx.page.crashed)
	done := waitJob(t, fx.mgr, job.ID, terminal)
	if done.Status != StatusFailed || done.Error == nil || !strings.Contains(done.Error.Message, "crashed") {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if took := time.Since(started); took > 5*time.Second {
		t.Errorf("the crash took %s to end the job", took)
	}
	run := held.Load()
	run.mu.Lock()
	ff := run.ff
	run.mu.Unlock()
	if ff != nil {
		t.Error("FFmpeg started after the crash")
	}
	if entries, _ := os.ReadDir(StagingRoot(fx.dir)); len(entries) != 0 {
		t.Errorf("staging not cleaned: %v", entries)
	}
}
```

In `scripts/storymaker-tests/09-host-render.mjs`:

Old text:

```js
        gets.push(url);
```

New text:

```js
        gets.push(url);
        if (url.split('?')[0] === RBASE + '/start') return res(opts.start ? opts.start(posts) : { start: true });
```

Old text:

```js
    const resumed = await runRender({ story: minimal, noCode: true, report: () => ({ start: true, from_frame: 10 }) });
    assert.equal(resumed.frames[0].n, 10);
```

New text:

```js
    const ignored = await runRender({ story: minimal, noCode: true, report: () => ({ start: true, from_frame: 10 }) });
    assert.equal(ignored.frames[0].n, 0, 'from_frame is gone: a render always starts at frame 0');
```

Old text:

```js
check('render mode: a code scene that fails is reported in the report issues and by POST /error', async () => {
```

New text:

```js
check('render mode: a pending report long-polls /start and renders only after start', async () => {
    let polls = 0;
    const framesAtPoll = [];
    const r = await runRender({
        story: minimal, noCode: true, report: () => ({ start: false, pending: true }),
        start: posts => {
            polls++;
            framesAtPoll.push(posts.filter(p => p.path.startsWith('/frame')).length);
            return polls < 3 ? { pending: true } : { start: true };
        }
    });
    assert.equal(r.status.phase, 'done', JSON.stringify(r.status));
    assert.equal(polls, 3, 'every pending answer polls again');
    assert.deepEqual(framesAtPoll, [0, 0, 0], 'no frame before the start');
    assert.equal(r.frames[0].n, 0);
    assert.equal(r.frames.length, r.server.posts[0].report.frame_count);
});

check('render mode: an error from /start and an answer with neither start nor pending end in refused', async () => {
    const pending = () => ({ start: false, pending: true });
    const failed = await runRender({ story: minimal, noCode: true, report: pending, start: () => ({ error: 'ffmpeg_unavailable', message: 'no ffmpeg' }) });
    assert.equal(failed.status.phase, 'refused'); assert.equal(failed.status.error, 'ffmpeg_unavailable'); assert.equal(failed.frames.length, 0);
    const neither = await runRender({ story: minimal, noCode: true, report: () => ({ start: false }) });
    assert.equal(neither.status.phase, 'refused'); assert.equal(neither.frames.length, 0);
    assert.equal(neither.server.gets.filter(u => u.includes('/start')).length, 0);
});

check('render mode: a stand-in that answers {start:true} renders without polling', async () => {
    const r = await runRender({ story: minimal, noCode: true });
    assert.equal(r.status.phase, 'done');
    assert.equal(r.server.gets.filter(u => u.includes('/start')).length, 0);
});

check('render mode: /start carries the token of boot.json, and no t without one', async () => {
    const token = '0123456789abcdef0123456789abcdef';
    const pending = () => ({ start: false, pending: true });
    const withToken = await runRender({ story: minimal, noCode: true, report: pending, boot: { token } });
    assert.ok(withToken.server.gets.includes(RBASE + '/start?t=' + token));
    const without = await runRender({ story: minimal, noCode: true, report: pending });
    assert.ok(without.server.gets.includes(RBASE + '/start'));
});

check('render mode: a code scene that fails is reported in the report issues and by POST /error', async () => {
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/render -run '(?i)deferred' -count=1
npm run test:storymaker
```

Expected:
- Go fails to compile: `undefined: startHold`, `undefined: StartAnswer`, `fx.deps.mixHook undefined`, `run.onStart undefined`.
- Node fails: the pending run ends in phase `refused` instead of `done`, and `ignored.frames[0].n` is 10.

- [ ] **Step 4: Implement**

**File `internal/storymaker/render/listener.go`.** None of these lines is changed by N18, N19 or N20, apart from the import block, which is matched at its first line only.

Old text:

```go
import (
	"crypto/rand"
```

New text:

```go
import (
	"context"
	"crypto/rand"
```

Old text:

```go
// SnapshotFont is one catalog font face as the render page receives it.
```

New text:

```go
// startHold is how long GET /start holds one call. A variable, so the tests can shorten it.
var startHold = 20 * time.Second

// SnapshotFont is one catalog font face as the render page receives it.
```

Old text:

```go
type ReportAnswer struct {
	Start     bool   `json:"start"`
	FromFrame int    `json:"from_frame,omitempty"`
	Error     string `json:"error,omitempty"`
	Message   string `json:"message,omitempty"`
}
```

New text:

```go
type ReportAnswer struct {
	Start   bool   `json:"start"`
	Pending bool   `json:"pending,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// StartAnswer is the reply to GET /start.
type StartAnswer struct {
	Pending bool   `json:"pending,omitempty"`
	Start   bool   `json:"start,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}
```

Old text:

```go
	// Error receives a page failure.
	Error func(e PageError)
```

New text:

```go
	// Error receives a page failure.
	Error func(e PageError)
	// Start blocks at most startHold. status is the HTTP status: 404 before an accepted report, else 200.
	Start func(ctx context.Context) (answer StartAnswer, status int)
```

Old text:

```go
	case rest == "report":
		l.report(w, r)
```

New text:

```go
	case rest == "report":
		l.report(w, r)
	case rest == "start":
		l.start(w, r)
```

Old text (N18 does not touch `count()`, so the route `start` lands here in A19):

```go
	case "report", "frame", "error", "boot.json":
```

New text:

```go
	case "report", "start", "frame", "error", "boot.json":
```

Old text:

```go
func (l *Listener) frame(w http.ResponseWriter, r *http.Request) {
```

New text:

```go
// start is the long poll of the deferred start (contract section 7).
func (l *Listener) start(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("t")), []byte(l.params.Token)) != 1 {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "bad_token"})
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET, OPTIONS")
		return
	}
	if l.hooks.Start == nil {
		notFound(w)
		return
	}
	answer, status := l.hooks.Start(r.Context())
	writeJSON(w, status, answer)
}

func (l *Listener) frame(w http.ResponseWriter, r *http.Request) {
```

**File `internal/storymaker/render/render.go`.** N18, N19, N20, N21, A9, A17 and A18 leave all of these lines as quoted.

Old text:

```go
	"path/filepath"
	"strings"
```

New text:

```go
	"path/filepath"
	"runtime/debug"
	"strings"
```

Old text:

```go
	// ListenerObserver, when set, receives the request counts of the listener when the job ends.
```

New text:

```go
	// mixHook, when set, runs where the mix runs, on the mix context. Tests hold the start with it.
	mixHook func(ctx context.Context, run *renderRun) error
	// ListenerObserver, when set, receives the request counts of the listener when the job ends.
```

Old text:

```go
	failCh   chan *JobError
	reportCh chan struct{}
	doneCh   chan struct{}
```

New text:

```go
	failCh   chan *JobError
	reportCh chan struct{}
	doneCh   chan struct{}

	// page is set under mu before Navigate, because driveScreenshots needs it.
	page browserPage
	// mixCtx derives from the job ctx and is created in execute before the page opens.
	mixCtx    context.Context
	cancelMix context.CancelFunc
	// onReport adds to mixWG under mu before it starts the goroutine.
	mixWG sync.WaitGroup
	// startCh is closed once FFmpeg runs or the start failed; startErr is read after that.
	startCh  chan struct{}
	startErr *JobError
```

Old text:

```go
		Hooks{Report: run.onReport, Frame: run.onFrame, Error: run.onPageError})
```

New text:

```go
		Hooks{Report: run.onReport, Frame: run.onFrame, Error: run.onPageError, Start: run.onStart})
```

Old text:

```go
	page, err := r.deps.OpenPage(ctx, opts)
```

New text:

```go
	run.startCh = make(chan struct{})
	run.mixCtx, run.cancelMix = context.WithCancel(ctx)
	page, err := r.deps.OpenPage(ctx, opts)
```

Old text:

```go
		if ff != nil {
			ff.Discard()
		}
	}()

	h.SetPhase(PhaseCompiling)
```

New text:

```go
		if ff != nil {
			ff.Discard()
		}
	}()
	// Runs before the discard above: stop the mix and wait, so FFmpeg never starts or is in run.ff.
	defer func() {
		run.cancelMix()
		run.mixWG.Wait()
	}()
	run.mu.Lock()
	run.page = page
	run.mu.Unlock()

	h.SetPhase(PhaseCompiling)
```

Old text:

```go
			overall.Reset(RenderTimeout(run.st.TimeoutSeconds, run.report.FrameCount, run.transport))
			if run.transport == TransportScreenshot {
				go run.driveScreenshots(ctx, page)
			}
		case <-reportTimeout:
```

New text:

```go
			overall.Reset(RenderTimeout(run.st.TimeoutSeconds, run.report.FrameCount, run.transport))
		case <-reportTimeout:
```

Old text:

```go
// onReport is the hook behind POST /report: it validates the report, starts FFmpeg and tells the
// page to begin. The ingest starts after the run lock is released (the ingest calls back into the
// run while it holds its own lock, so the lock order is always ingest, then run).
```

New text:

```go
// onReport is the hook behind POST /report: it validates the report and answers pending at once. The
// mix and the encoder start in startAfterReport, because three timers would cut a mix inside this request.
```

Old text:

```go
	ff, err := run.startFFmpeg(run.encoder, rep.FrameCount)
	if err != nil {
		run.mu.Unlock()
		run.fail(Fail(CodeFFmpegUnavailable, "%v", err))
		return ReportAnswer{Error: CodeFFmpegUnavailable, Message: err.Error()}
	}
	run.report = rep
	run.frames = rep.FrameCount
	run.ff = ff
	run.watchdog = time.AfterFunc(noFrameWatchdog, func() {
		run.fail(Fail(CodeCodeError, "The page rendered no frame for %s. A code scene may be stuck.", noFrameWatchdog))
	})
	run.mu.Unlock()

	run.ingest.Start(rep.FrameCount, ff, run.recoverEncoder, run.onFrameAccepted)
	run.h.SetPhase(PhaseRendering)
	run.h.SetFrames(0, rep.FrameCount)
	close(run.reportCh)
	return ReportAnswer{Start: true}
}
```

New text:

```go
	run.report = rep
	run.frames = rep.FrameCount
	run.mixWG.Add(1)
	close(run.reportCh)
	go run.startAfterReport()
	run.mu.Unlock()
	return ReportAnswer{Pending: true}
}

// onStart is the hook behind GET /start: 404 until a report was accepted, then it holds at most
// startHold and answers pending, start or the failure of the start.
func (run *renderRun) onStart(ctx context.Context) (StartAnswer, int) {
	run.mu.Lock()
	accepted := run.frames > 0
	run.mu.Unlock()
	if !accepted {
		return StartAnswer{}, 404
	}
	timer := time.NewTimer(startHold)
	defer timer.Stop()
	select {
	case <-run.startCh:
		if run.startErr != nil {
			return StartAnswer{Error: run.startErr.Code, Message: run.startErr.Message}, 200
		}
		return StartAnswer{Start: true}, 200
	case <-timer.C:
	case <-ctx.Done():
	}
	return StartAnswer{Pending: true}, 200
}

// failStart ends the start with err: the job fails and the next /start answer carries the error.
func (run *renderRun) failStart(err *JobError) {
	run.startErr = err
	run.fail(err)
	close(run.startCh)
}

// recoverStart is deferred by startAfterReport. This goroutine is out of reach of Manager.safeRun, so without it one bad
// stored file would end the whole server. Only this goroutine closes startCh, so the check cannot race.
func (run *renderRun) recoverStart() {
	rec := recover()
	if rec == nil {
		return
	}
	run.r.deps.Logger.Error("[Storymaker] Recovered from a panic while starting a render", "job", run.job.ID, "panic", rec, "stack", string(debug.Stack()))
	err := Fail(CodeRenderFailed, "The render stopped unexpectedly.")
	select {
	case <-run.startCh:
		run.fail(err)
	default:
		run.failStart(err)
	}
}

// startEncoder starts FFmpeg and the watchdog under run.mu. It returns no encoder and no error when the job is already
// over. The unlock is deferred because a panic must not leave run.mu held: recoverStart fails the job, and execute then
// takes the lock on its way out.
func (run *renderRun) startEncoder() (*FFmpeg, error) {
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.mixCtx.Err() != nil {
		return nil, nil
	}
	ff, err := run.startFFmpeg(run.encoder, run.frames)
	if err != nil {
		return nil, err
	}
	run.ff = ff
	run.watchdog = time.AfterFunc(noFrameWatchdog, func() {
		run.fail(Fail(CodeCodeError, "The page rendered no frame for %s. A code scene may be stuck.", noFrameWatchdog))
	})
	return ff, nil
}

// startAfterReport runs after an accepted report. It prepares the audio, starts FFmpeg, the watchdog and the ingest, and
// then releases the page. The ingest starts after the run lock is released, because the ingest calls back into the run
// while it holds its own lock.
func (run *renderRun) startAfterReport() {
	defer run.mixWG.Done()
	defer run.recoverStart()
	run.h.SetPhase(PhaseMixing)
	if hook := run.r.deps.mixHook; hook != nil {
		if err := hook(run.mixCtx, run); err != nil && run.mixCtx.Err() == nil {
			run.failStart(Fail(CodeRenderFailed, "%v", err))
			return
		}
	}

	ff, err := run.startEncoder()
	if err != nil {
		run.failStart(Fail(CodeFFmpegUnavailable, "%v", err))
		return
	}
	if ff == nil {
		// The job is over. FFmpeg must not start: on Windows RemoveAll fails while it holds a file.
		run.startErr = Fail(CodeRenderFailed, "The render stopped before it started.")
		close(run.startCh)
		return
	}
	run.mu.Lock()
	page, frames := run.page, run.frames
	run.mu.Unlock()

	run.ingest.Start(frames, ff, run.recoverEncoder, run.onFrameAccepted)
	run.h.SetPhase(PhaseRendering)
	run.h.SetFrames(0, frames)
	close(run.startCh)
	if run.transport == TransportScreenshot {
		go run.driveScreenshots(run.ctx, page)
	}
}
```

**File `ui/js/storymaker/host-render.js`.** N18 leaves these lines as quoted. `token` is the closure variable N18 declares (`var token = '';`), and it stays empty when `boot.json` has no token.

The first edit corrects N18's comment, because `waitForStart` reads the token as well.

Old text:

```js
        // The token of the first boot.json answer stays here, and only post() reads it. A failure text names a label or a route and
```

New text:

```js
        // The token of the first boot.json answer stays here, and post() and waitForStart() read it. A failure text names a label or a route and
```

Old text:

```js
        async function postError(err, sceneId) {
```

New text:

```js
        // The deferred start: Go holds each GET at most 20 s and answers pending, start or an error, so the page
        // needs no timer of its own. A failure text names the route only, never the token.
        async function waitForStart() {
            for (;;) {
                var res = await doFetch(base + '/start' + (token ? '?t=' + token : ''), { cache: 'no-store' });
                if (!res.ok) throw H.fail('render_failed', 'GET /start answered ' + res.status);
                var reply = await res.json();
                if (!reply.pending) return reply;
            }
        }
        async function postError(err, sceneId) {
```

Old text:

```js
            if (!answer.start) {
                status.phase = 'refused';
                status.error = answer.error || '';
                status.message = answer.message || '';
                return status;
            }
```

New text:

```js
            // Go answers at once and mixes in the background. A stand-in may still answer {start:true}.
            if (answer.pending && !answer.start && !answer.error) {
                status.phase = 'waiting';
                answer = await waitForStart();
            }
            if (!answer.start) {
                status.phase = 'refused';
                status.error = answer.error || '';
                status.message = answer.message || '';
                return status;
            }
```

Old text:

```js
            var n = answer.from_frame > 0 ? answer.from_frame : 0;
```

New text:

```js
            var n = 0;
```

- [ ] **Step 5: Run**

Expected: everything passes. The last line shows four top-level `--- PASS` lines, one for each of `TestStorymakerPlayerGateBrowser`, `TestStorymakerRenderModeBrowser`, `TestStorymakerFirewallRenderBrowser` and `TestStorymakerRenderScreenshotBrowser`. Run the browser line with the shell sandbox off.

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
gofmt -w internal/storymaker/render/render.go internal/storymaker/render/listener.go internal/storymaker/render/render_test.go internal/storymaker/render/listener_test.go
npm run build:ui
npm run check:ui
npm run test:storymaker
go vet ./internal/storymaker/render
go test ./internal/storymaker/render -count=1 -timeout 20m
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./ui -v -run '^TestStorymaker(PlayerGate|RenderMode|FirewallRender|RenderScreenshot)Browser$' -count=1 -timeout 20m; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
```

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/render/render.go internal/storymaker/render/listener.go internal/storymaker/render/render_test.go internal/storymaker/render/listener_test.go ui/js/storymaker/host-render.js ui/js/storymaker/host.bundle.js ui/js/storymaker/player.bundle.js scripts/storymaker-tests/09-host-render.mjs
git commit -m "feat(storymaker): defer the render start behind GET /start" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A20: Mix wired into the job

**Batch:** 2C  **Depends on:** A16, A18, A19

**Files:**
- Modify: `internal/storymaker/render/render.go`
- Modify: `internal/storymaker/render/jobs.go`
- Test: `internal/storymaker/render/render_test.go`

- [ ] **Step 1: Impact**

Run this from the main checkout. Per outline section 5: report callers, processes and risk, warn on HIGH or CRITICAL, confirm UNKNOWN with a text search.

```powershell
foreach ($symbol in 'startFFmpeg','recoverEncoder','Job','Handle') { node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact $symbol --direction upstream --repo . }
```

- [ ] **Step 2: Write the failing test**

In `internal/storymaker/render/render_test.go`:

Old text:

```go
	"os"
	"path/filepath"
```

New text:

```go
	"os"
	"os/exec"
	"path/filepath"
```

Old text:

```go
	"aurago/internal/storymaker"
	"aurago/internal/videostudio"
```

New text:

```go
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/storymakertest"
	"aurago/internal/videostudio"
```

Append to the end of the file:

```go
const narratedStoryDoc = `{"schema":"storymaker/1","title":"Mix","scenes":[{"id":"s1","narration":"Hallo Welt, das ist ein kurzer Test."}]}`

// seedNarratedStory saves doc, seeds its audio and returns the audio events an honest page reports for it.
func (fx *runFixture) seedNarratedStory(t *testing.T, doc string, seconds float64) []any {
	t.Helper()
	ctx := context.Background()
	if _, err := fx.projects.SaveStory(ctx, fx.projectID, []byte(doc), ""); err != nil {
		t.Fatal(err)
	}
	data, _, err := fx.projects.LoadStory(ctx, fx.projectID)
	if err != nil {
		t.Fatal(err)
	}
	story, _, err := storymaker.DecodeStory(data, fx.settings.Limits)
	if err != nil {
		t.Fatal(err)
	}
	storymaker.ApplyDefaults(&story)
	if err := storymakertest.SeedNarration(ctx, fx.projects, fx.projectID, &story, seconds); err != nil {
		t.Fatal(err)
	}
	answer, err := fx.projects.AudioAnswer(ctx, fx.projectID, &story, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(answer.Missing) != 0 {
		t.Fatalf("seeded story still misses %+v", answer.Missing)
	}
	events := []any{}
	for sceneID, scene := range answer.Narration {
		for _, block := range scene.Blocks {
			for i, piece := range block.Pieces {
				events = append(events, map[string]any{"kind": block.Kind, "ref": piece.Hash, "start_ms": piece.AtMS, "scene_id": sceneID, "block_id": block.ID, "piece": i})
			}
		}
	}
	return events
}

// publishedMP4 writes the job's published MP4 to a temp file.
func (fx *runFixture) publishedMP4(t *testing.T, job Job) string {
	t.Helper()
	if job.Output == nil {
		t.Fatalf("job %+v has no output", job)
	}
	mp4, ok := fx.pub.get(job.Output.Path)
	if !ok {
		t.Fatalf("%s was not published", job.Output.Path)
	}
	tmp := filepath.Join(t.TempDir(), "check.mp4")
	if err := os.WriteFile(tmp, mp4, 0o600); err != nil {
		t.Fatal(err)
	}
	return tmp
}

// meterFile reads the loudness of a file's audio with the meter's own filter graph.
func meterFile(t *testing.T, ffmpeg, file string) MeterReading {
	t.Helper()
	graph := ""
	args := MeterArgs()
	for i, a := range args {
		if a == "-af" && i+1 < len(args) {
			graph = args[i+1]
		}
	}
	if graph == "" {
		t.Fatalf("MeterArgs has no -af: %v", args)
	}
	cmd := exec.Command(ffmpeg, "-hide_banner", "-nostdin", "-nostats", "-loglevel", "info", "-i", file, "-map", "0:a:0", "-af", graph, "-f", "null", "-")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("meter: %v\n%s", err, stderr.String())
	}
	reading, err := parseMeterOutput(stderr.String())
	if err != nil {
		t.Fatalf("meter output: %v\n%s", err, stderr.String())
	}
	return reading
}

// wantLandedMix checks the level of the AAC track: the target is -16 LUFS (mixTargetLUFS), and the seeded tone
// sits far below the -1 dBTP ceiling, so the loudness bound decides the gain.
func wantLandedMix(t *testing.T, r MeterReading) {
	t.Helper()
	if r.Silent || r.I < -18.5 || r.I > -15.5 || r.TP > -1.0 {
		t.Errorf("mix landed at %+v, want -18.5..-15.5 LUFS at -1 dBTP or below", r)
	}
}

func hasWarning(job Job, code string) bool {
	for _, w := range job.Warnings {
		if w.Code == code {
			return true
		}
	}
	return false
}

// silentWAV is a 48 kHz stereo s16 WAV of digital silence.
func silentWAV(seconds int) []byte {
	const rate, channels, sampleBytes = 48000, 2, 2
	dataLen := seconds * rate * channels * sampleBytes
	le32 := func(v int) []byte { return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)} }
	le16 := func(v int) []byte { return []byte{byte(v), byte(v >> 8)} }
	var b bytes.Buffer
	b.WriteString("RIFF")
	b.Write(le32(36 + dataLen))
	b.WriteString("WAVEfmt ")
	b.Write(le32(16))
	b.Write(le16(1))
	b.Write(le16(channels))
	b.Write(le32(rate))
	b.Write(le32(rate * channels * sampleBytes))
	b.Write(le16(channels * sampleBytes))
	b.Write(le16(16))
	b.WriteString("data")
	b.Write(le32(dataLen))
	b.Write(make([]byte, dataLen))
	return b.Bytes()
}

func TestRunMixSeededStoryHasAudibleAAC(t *testing.T) {
	ffmpeg, ffprobe := testBinary(t, "ffmpeg"), testBinary(t, "ffprobe")
	fx := newRunFixture(t)
	// 120 frames at 30 fps are 4 s, so the 3 s piece fits inside the film.
	events := fx.seedNarratedStory(t, narratedStoryDoc, 3)
	fx.playing(playOptions{frames: 120, mutateRep: func(rep map[string]any) { rep["audio_events"] = events }})
	job := fx.render(t)
	if job.Status != StatusSucceeded {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	file := fx.publishedMP4(t, job)
	aac := false
	for _, s := range ffprobeFile(t, ffprobe, file).Streams {
		aac = aac || (s.CodecType == "audio" && s.CodecName == "aac" && s.SampleRate == "48000")
	}
	if !aac {
		t.Error("the MP4 has no 48 kHz AAC track")
	}
	wantLandedMix(t, meterFile(t, ffmpeg, file))
}

func TestRunMixSurvivesTheSoftwareRestart(t *testing.T) {
	ffmpeg := testBinary(t, "ffmpeg")
	fx := newRunFixture(t)
	fx.settings.Encoder = videostudio.Encoder{Codec: videostudio.CodecVAAPI, Device: "/dev/dri/does-not-exist"}
	fx.settings.EncoderName = videostudio.CodecVAAPI
	// 60 frames at 30 fps are 2 s, so the 1.5 s piece fits inside the film.
	events := fx.seedNarratedStory(t, narratedStoryDoc, 1.5)
	var restarts atomic.Int32
	fx.playing(playOptions{frames: 60, onRestart: func() { restarts.Add(1) }, mutateRep: func(rep map[string]any) { rep["audio_events"] = events }})
	job := fx.render(t)
	if job.Status != StatusSucceeded {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	if restarts.Load() != 1 {
		t.Fatalf("the page was told to restart %d times", restarts.Load())
	}
	wantLandedMix(t, meterFile(t, ffmpeg, fx.publishedMP4(t, job)))
}

func TestRunMixDropsASilentBedWithAWarning(t *testing.T) {
	ffmpeg := testBinary(t, "ffmpeg")
	fx := newRunFixture(t)
	key := strings.Repeat("a", 64)
	fx.deps.MusicKey = func(context.Context, *storymaker.Music) (string, error) { return key, nil }
	fx.rebuild(t)
	const doc = `{"schema":"storymaker/1","title":"Mix","music":{"mode":"generated","mood":"calm-ambient"},"scenes":[{"id":"s1","narration":"Hallo Welt, das ist ein kurzer Test."}]}`
	events := fx.seedNarratedStory(t, doc, 1.5)
	ctx := context.Background()
	if err := fx.projects.WriteMusic(ctx, fx.projectID, key, "wav", silentWAV(2)); err != nil {
		t.Fatal(err)
	}
	if err := fx.projects.WriteMusicMeta(ctx, fx.projectID, key, storymaker.MusicMeta{V: 1, Ext: "wav", DurationMS: 2000}); err != nil {
		t.Fatal(err)
	}
	fx.playing(playOptions{frames: 60, mutateRep: func(rep map[string]any) { rep["audio_events"] = events }})
	job := fx.render(t)
	if job.Status != StatusSucceeded {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	if !hasWarning(job, "music_silent") {
		t.Errorf("warnings = %+v, want music_silent", job.Warnings)
	}
	wantLandedMix(t, meterFile(t, ffmpeg, fx.publishedMP4(t, job)))
}

func TestRunMixSilentStoryKeepsAnullsrc(t *testing.T) {
	ffmpeg := testBinary(t, "ffmpeg")
	fx := newRunFixture(t)
	fx.playing(playOptions{frames: 24})
	job := fx.render(t)
	if job.Status != StatusSucceeded || len(job.Warnings) != 0 {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	if r := meterFile(t, ffmpeg, fx.publishedMP4(t, job)); !r.Silent {
		t.Errorf("a silent story measured %+v", r)
	}
}

func TestRunMixCancelEndsTheJob(t *testing.T) {
	fx := newRunFixture(t)
	inMix := make(chan struct{})
	fx.deps.mixHook = func(ctx context.Context, _ *renderRun) error {
		close(inMix)
		<-ctx.Done()
		return ctx.Err()
	}
	fx.rebuild(t)
	events := fx.seedNarratedStory(t, narratedStoryDoc, 1.5)
	fx.playing(playOptions{frames: 60, mutateRep: func(rep map[string]any) { rep["audio_events"] = events }})
	job := fx.enqueueRender(t)
	select {
	case <-inMix:
	case <-time.After(time.Minute):
		t.Fatal("the mix never started")
	}
	fx.mgr.Cancel(job.ID)
	done := waitJob(t, fx.mgr, job.ID, terminal)
	if done.Status != StatusCancelled {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if len(fx.pub.names()) != 0 {
		t.Errorf("a cancelled mix published %v", fx.pub.names())
	}
	if entries, _ := os.ReadDir(StagingRoot(fx.dir)); len(entries) != 0 {
		t.Errorf("staging not cleaned: %v", entries)
	}
}
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/render -run '(?i)TestRunMix' -count=1 -timeout 15m
```

Expected: compile failure, `job.Warnings undefined`.

- [ ] **Step 4: Implement**

**File `internal/storymaker/render/jobs.go`.**

Old text:

```go
	Output      *Output    `json:"output,omitempty"`
```

New text:

```go
	Output      *Output    `json:"output,omitempty"`
	Warnings    []JobWarning `json:"warnings,omitempty"`
```

Old text:

```go
// Job is the public job document (contract section 5).
```

New text:

```go
// Warning codes of a job that still succeeded.
const (
	warnStemFailed     = "stem_failed"
	warnSubtitleFailed = "subtitle_failed"
	warnLibraryFailed  = "library_failed"
	warnMusicSilent    = "music_silent"
)

// JobWarning is a problem that did not fail the job.
type JobWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Job is the public job document (contract section 5).
```

Old text:

```go
func clip(s string, n int) string {
```

New text:

```go
// AddWarning records a warning (a state change: it is persisted and announced at once). A reader may hold the
// old slice, so the append always copies.
func (h *Handle) AddWarning(code, message string) {
	h.update(func(r *record) {
		r.Warnings = append(r.Warnings[:len(r.Warnings):len(r.Warnings)], JobWarning{Code: code, Message: clip(message, 400)})
	}, true)
}

func clip(s string, n int) string {
```

**File `internal/storymaker/render/render.go`.** The first two old texts are as A19 leaves them.

Old text:

```go
	// startCh is closed once FFmpeg runs or the start failed; startErr is read after that.
	startCh  chan struct{}
	startErr *JobError
```

New text:

```go
	// startCh is closed once FFmpeg runs or the start failed; startErr is read after that.
	startCh  chan struct{}
	startErr *JobError
	// audioPath is set under mu before the first FFmpeg start. startFFmpeg reads it, so recoverEncoder keeps the mix.
	audioPath string
	// mixResult and warnings are guarded by mu.
	mixResult MixResult
	warnings  []JobWarning
```

Old text:

```go
			run.failStart(Fail(CodeRenderFailed, "%v", err))
			return
		}
	}

	ff, err := run.startEncoder()
```

New text:

```go
			run.failStart(Fail(CodeRenderFailed, "%v", err))
			return
		}
	}
	if HasAudio(run.plan, run.report) && run.mixCtx.Err() == nil {
		mixPath, res, err := run.mixAudio(run.mixCtx)
		if err != nil && run.mixCtx.Err() == nil {
			run.failStart(run.mixFailure(err))
			return
		}
		if err == nil {
			run.mu.Lock()
			run.audioPath, run.mixResult = mixPath, res
			run.mu.Unlock()
			if res.BedDropped {
				run.warn(warnMusicSilent, "The music track is silent, so the film has no music.")
			}
		}
	}

	ff, err := run.startEncoder()
```

Old text:

```go
func (run *renderRun) startFFmpeg(enc videostudio.Encoder, frames int) (*FFmpeg, error) {
```

New text:

```go
// mixAudio writes staging/mix.wav. ctx is the mix context, so a cancel stops it at the next block.
func (run *renderRun) mixAudio(ctx context.Context) (string, MixResult, error) {
	staging := filepath.Dir(run.output)
	plan, err := run.planMixInput(ctx)
	if err != nil {
		return "", MixResult{}, err
	}
	in, err := run.buildMixInput(ctx, plan, staging)
	if err != nil {
		return "", MixResult{}, err
	}
	mix := filepath.Join(staging, "mix.wav")
	newMeter := func(c context.Context) (*Meter, error) { return StartMeter(c, run.st.FFmpegPath) }
	res, err := Mix(ctx, in, newMeter, MixOutputs{Mix: mix}, func(float64) {})
	if err != nil {
		return "", MixResult{}, err
	}
	return mix, res, nil
}

// mixFailure maps a mix error to the job error: a *JobError keeps its code, anything else is render_failed.
func (run *renderRun) mixFailure(err error) *JobError {
	var je *JobError
	if errors.As(err, &je) {
		return je
	}
	return Fail(CodeRenderFailed, "The audio could not be mixed: %s", run.scrubbed(err))
}

// scrubbed is the text of err without the staging folder, because no absolute path may reach the user.
func (run *renderRun) scrubbed(err error) string {
	return clip(strings.ReplaceAll(err.Error(), filepath.Dir(run.output), "<staging>"), 400)
}

// warn adds a warning to the job and keeps it for the export record.
func (run *renderRun) warn(code, message string) {
	run.mu.Lock()
	run.warnings = append(run.warnings, JobWarning{Code: code, Message: clip(message, 400)})
	run.mu.Unlock()
	run.h.AddWarning(code, message)
}

func (run *renderRun) startFFmpeg(enc videostudio.Encoder, frames int) (*FFmpeg, error) {
```

Old text:

```go
		Width: run.width, Height: run.height, FPS: run.fps, Frames: frames, Output: run.output,
```

New text:

```go
		Width: run.width, Height: run.height, FPS: run.fps, Frames: frames, Output: run.output,
		AudioPath: run.audioPath,
```

- [ ] **Step 5: Run**

Expected: all pass.

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
gofmt -w internal/storymaker/render/render.go internal/storymaker/render/jobs.go internal/storymaker/render/render_test.go
go vet ./internal/storymaker/render
go test ./internal/storymaker/render -count=1 -timeout 20m
```

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/render/render.go internal/storymaker/render/jobs.go internal/storymaker/render/render_test.go
git commit -m "feat(storymaker): mix the audio into the export" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A21: Render options

**Batch:** 2C  **Depends on:** N6, A20

**Files:**
- Create: `internal/storymaker/render/options.go`
- Modify: `internal/storymaker/render/render.go`
- Modify: `internal/storymaker/render/mixinput.go`
- Modify: `internal/server/storymaker_handlers.go`
- Test: `internal/storymaker/render/options_test.go`
- Test: `internal/storymaker/render/render_test.go`
- Test: `internal/storymaker/render/report_test.go`
- Test: `internal/server/storymaker_api_test.go`

- [ ] **Step 1: Impact**

Run this from the main checkout. Per outline section 5: report callers, processes and risk, warn on HIGH or CRITICAL, confirm UNKNOWN with a text search.

```powershell
foreach ($symbol in 'Prepare','execute','handleStorymakerRender','mixStems') { node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact $symbol --direction upstream --repo . }
```

- [ ] **Step 2: Write the failing test**

Create `internal/storymaker/render/options_test.go`:

```go
package render

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"aurago/internal/videostudio"
)

func TestParseRenderOptions(t *testing.T) {
	for _, empty := range [][]byte{nil, {}, []byte("  \n")} {
		opts, err := ParseRenderOptions(empty)
		if err != nil || opts.Quality != "standard" || opts.SRT || opts.VTT || opts.Stems || opts.BurnInCaptions != nil {
			t.Errorf("empty body %q = %+v, %v", empty, opts, err)
		}
	}
	opts, err := ParseRenderOptions([]byte(`{"burn_in_captions":false,"srt":true,"vtt":true,"stems":true,"quality":"high"}`))
	if err != nil || opts.BurnInCaptions == nil || *opts.BurnInCaptions || !opts.SRT || !opts.VTT || !opts.Stems || opts.Quality != "high" {
		t.Errorf("full body = %+v, %v", opts, err)
	}
	if opts, err := ParseRenderOptions([]byte(`{"srt":true}`)); err != nil || opts.Quality != "standard" {
		t.Errorf("an absent quality = %+v, %v", opts, err)
	}
	for _, bad := range []string{`{"handoff":true}`, `{"quality":"ultra"}`, `{"quality":""}`, `[]`, `{"srt":"yes"}`, `{"srt":true} {"srt":false}`} {
		if _, err := ParseRenderOptions([]byte(bad)); err == nil {
			t.Errorf("%s was accepted", bad)
		}
	}
}

func TestRenderOptionsFingerprint(t *testing.T) {
	hash := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}
	yes := true
	if got, want := (RenderOptions{BurnInCaptions: &yes, SRT: true, Stems: true, Quality: "high"}).Fingerprint(), hash("burn=true;srt=true;vtt=false;stems=true;quality=high"); got != want {
		t.Errorf("fingerprint = %s, want %s", got, want)
	}
	if got, want := (RenderOptions{Quality: "standard"}).Fingerprint(), hash("burn=null;srt=false;vtt=false;stems=false;quality=standard"); got != want {
		t.Errorf("fingerprint of the defaults = %s, want %s", got, want)
	}
}

func TestRenderOptionsEncoderQuality(t *testing.T) {
	base := videostudio.Encoder{Codec: videostudio.CodecNVENC, Quality: 30}
	if got := (RenderOptions{Quality: "standard"}).encoder(base); got != base {
		t.Errorf("standard changed the encoder: %+v", got)
	}
	if got := (RenderOptions{}).encoder(base); got != base {
		t.Errorf("zero options changed the encoder: %+v", got)
	}
	if got := (RenderOptions{Quality: "high"}).encoder(base); got.Quality != 18 || got.Codec != base.Codec {
		t.Errorf("high = %+v, want quality 18 on the same codec", got)
	}
}

// The stems count in the disk check of buildMixInput, which reads plan.stems.
func TestRenderOptionsStemsReachTheMixPlan(t *testing.T) {
	for _, stems := range []bool{false, true} {
		run := &renderRun{plan: &Plan{Options: RenderOptions{Stems: stems}}, fps: 30, report: compileReport{FrameCount: 30}}
		plan, err := run.planMixInput(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if plan.stems != stems {
			t.Errorf("plan.stems = %v with the option %v", plan.stems, stems)
		}
	}
}
```

Append to the end of `internal/storymaker/render/render_test.go`:

```go
// renderWith runs a render with options and waits for its end.
func (fx *runFixture) renderWith(t *testing.T, opts RenderOptions) Job {
	t.Helper()
	plan, err := fx.runner.Prepare(context.Background(), fx.projectID, "", fx.settings, opts)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	job, _, err := fx.mgr.Enqueue(EnqueueRequest{ProjectID: fx.projectID, Payload: plan})
	if err != nil {
		t.Fatal(err)
	}
	return waitJobFor(t, fx.mgr, job.ID, terminal, 8*time.Minute)
}

func TestRunHighQualitySetsCRF18(t *testing.T) {
	fx := newRunFixture(t)
	var encoder videostudio.Encoder
	fx.deps.mixHook = func(_ context.Context, run *renderRun) error {
		encoder = run.encoder
		return nil
	}
	fx.rebuild(t)
	fx.playing(playOptions{frames: 6})
	job := fx.renderWith(t, RenderOptions{Quality: "high"})
	if job.Status != StatusSucceeded {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	args, err := BuildFFmpegArgs(FFmpegConfig{Encoder: encoder, Transport: TransportPNG, Width: 1920, Height: 1080, FPS: 30, Frames: 6, Output: "out.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(args, " "), "-crf 18") {
		t.Errorf("arguments = %v, want -crf 18", args)
	}
}

// The override runs before Validate. A Thai narration needs no glyphs with captions off, and is glyph_missing when burned in.
func TestPrepareBurnInOverrideRunsBeforeTheGlyphCheck(t *testing.T) {
	fx := newRunFixture(t)
	ctx := context.Background()
	thai := string([]rune{0x0E2A, 0x0E27, 0x0E31, 0x0E2A, 0x0E14, 0x0E35})
	doc := `{"schema":"storymaker/1","title":"Thai","style":{"palette":"` + storymaker.DefaultPalette + `","captions":false},"scenes":[{"id":"s1","narration":"` + thai + `"}]}`
	fx.seedNarratedStory(t, doc, 1)
	if _, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings, RenderOptions{}); err != nil {
		t.Fatalf("captions off: %v", err)
	}
	burn := true
	_, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings, RenderOptions{BurnInCaptions: &burn})
	var pe *PrepareError
	if !errors.As(err, &pe) || pe.Code != CodeInvalidStory {
		t.Fatalf("burned Thai captions = %v", err)
	}
	found := false
	for _, is := range pe.Issues {
		found = found || is.Code == storymaker.CodeGlyphMissing
	}
	if !found {
		t.Errorf("issues = %+v, want glyph_missing", pe.Issues)
	}
}
```

Update every existing `Prepare` call in `internal/storymaker/render/render_test.go`. These old texts are not unique on purpose, so replace all occurrences of each. The counts include the helper from A19.

| Old text | Occurrences | New text |
| --- | --- | --- |
| `fx.runner.Prepare(context.Background(), fx.projectID, "", fx.settings)` | 3 (two older calls and `enqueueRender`) | `fx.runner.Prepare(context.Background(), fx.projectID, "", fx.settings, RenderOptions{})` |
| `fx.runner.Prepare(ctx, fx.projectID, "", fx.settings)` | 8 (five older calls and N20's three) | `fx.runner.Prepare(ctx, fx.projectID, "", fx.settings, RenderOptions{})` |
| ``fx.runner.Prepare(ctx, fx.projectID, `"stale"`, fx.settings)`` | 1 | ``fx.runner.Prepare(ctx, fx.projectID, `"stale"`, fx.settings, RenderOptions{})`` |
| `fx.runner.Prepare(ctx, fx.projectID, plan.StoryETag, fx.settings)` | 1 | `fx.runner.Prepare(ctx, fx.projectID, plan.StoryETag, fx.settings, RenderOptions{})` |
| `fx.runner.Prepare(ctx, "33333333-3333-4333-8333-333333333333", "", fx.settings)` | 1 | `fx.runner.Prepare(ctx, "33333333-3333-4333-8333-333333333333", "", fx.settings, RenderOptions{})` |

`internal/storymaker/render/report_test.go` holds the one other call, in N21's `TestRunRefusesAReportThatDropsMeasuredNarration`. A15 and A16 add no caller.

Old text:

```go
	plan, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings)
```

New text:

```go
	plan, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings, RenderOptions{})
```

Append to the end of `internal/server/storymaker_api_test.go`:

```go
func TestStorymakerRenderOptions(t *testing.T) {
	plans := make(chan *render.Plan, 8)
	s, token, p := renderFixture(t, func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		plans <- payload.(*render.Plan)
		return render.Output{Path: "Documents/Storymaker/" + job.ProjectID + "/exports/x.mp4", Size: 3}, nil
	})
	path := "/projects/" + p.Project.ID + "/render"
	headers := func(key string) map[string]string { return map[string]string{"If-Match": p.ETag, "Idempotency-Key": key} }
	nextPlan := func() *render.Plan {
		t.Helper()
		select {
		case plan := <-plans:
			return plan
		case <-time.After(10 * time.Second):
			t.Fatal("the job did not run")
			return nil
		}
	}

	// A strict body: unknown fields, a bad quality and more than 16 KiB are 400.
	for name, body := range map[string]string{
		"unknown field": `{"handoff":true}`,
		"bad quality":   `{"quality":"ultra"}`,
		"too large":     `{"srt":true}` + strings.Repeat(" ", 16<<10),
	} {
		w := storymakerRequest(s, token, http.MethodPost, path, strings.NewReader(body), headers("bad"))
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"code":"invalid_request"`) {
			t.Errorf("%s = %d %s", name, w.Code, w.Body.String())
		}
	}

	// An empty body gives the defaults.
	w := storymakerRequest(s, token, http.MethodPost, path, nil, headers("one"))
	var first struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &first)
	if w.Code != http.StatusAccepted || first.Job.ID == "" {
		t.Fatalf("empty body = %d %s", w.Code, w.Body.String())
	}
	if plan := nextPlan(); plan.Options != (render.RenderOptions{Quality: "standard"}) {
		t.Errorf("default options = %+v", plan.Options)
	}

	// The same key returns the job with the same options and is 409 with other options.
	w = storymakerRequest(s, token, http.MethodPost, path, nil, headers("one"))
	var replay struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &replay)
	if w.Code != http.StatusAccepted || replay.Job.ID != first.Job.ID {
		t.Errorf("replay = %d %s", w.Code, w.Body.String())
	}
	w = storymakerRequest(s, token, http.MethodPost, path, strings.NewReader(`{"srt":true}`), headers("one"))
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "idempotency_conflict") {
		t.Errorf("same key, other options = %d %s", w.Code, w.Body.String())
	}

	// The options reach the snapshot, and the override reaches the story there.
	body := `{"burn_in_captions":false,"srt":true,"vtt":true,"stems":true,"quality":"high"}`
	w = storymakerRequest(s, token, http.MethodPost, path, strings.NewReader(body), headers("two"))
	if w.Code != http.StatusAccepted {
		t.Fatalf("full options = %d %s", w.Code, w.Body.String())
	}
	plan := nextPlan()
	o := plan.Options
	if o.BurnInCaptions == nil || *o.BurnInCaptions || !o.SRT || !o.VTT || !o.Stems || o.Quality != "high" || plan.Story.Style.Captions {
		t.Errorf("plan options = %+v, captions %v", o, plan.Story.Style.Captions)
	}
}
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/render -run '(?i)(RenderOptions|HighQuality|BurnInOverride)' -count=1
go test ./internal/server -run '^TestStorymakerRenderOptions$' -count=1
```

Expected: compile failure, `undefined: RenderOptions`, `undefined: ParseRenderOptions`, `too many arguments in call to fx.runner.Prepare`, and `render.RenderOptions` undefined in the server package.

- [ ] **Step 4: Implement**

Create `internal/storymaker/render/options.go`:

```go
package render

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"aurago/internal/videostudio"
)

const (
	qualityStandard = "standard"
	qualityHigh     = "high"
	// highQualityCRF is the encoder quality of "high". "standard" keeps the selected encoder's value.
	highQualityCRF = 18
)

// RenderOptions are the export options of POST /projects/{id}/render.
type RenderOptions struct {
	BurnInCaptions *bool  `json:"burn_in_captions,omitempty"` // nil means style.captions
	SRT            bool   `json:"srt"`
	VTT            bool   `json:"vtt"`
	Stems          bool   `json:"stems"`
	Quality        string `json:"quality"` // standard | high
}

// ParseRenderOptions reads the request body. An empty body gives the defaults, and an unknown field,
// trailing data or an unknown quality is an error.
func ParseRenderOptions(body []byte) (RenderOptions, error) {
	opts := RenderOptions{Quality: qualityStandard}
	if len(bytes.TrimSpace(body)) == 0 {
		return opts, nil
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&opts); err != nil {
		return RenderOptions{}, fmt.Errorf("the options are not valid: %w", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return RenderOptions{}, errors.New("the options are followed by more data")
	}
	if opts.Quality != qualityStandard && opts.Quality != qualityHigh {
		return RenderOptions{}, fmt.Errorf("quality must be %s or %s", qualityStandard, qualityHigh)
	}
	return opts, nil
}

// Fingerprint is the canonical hash of the options, for the idempotency check.
func (o RenderOptions) Fingerprint() string {
	burn := "null"
	if o.BurnInCaptions != nil {
		burn = strconv.FormatBool(*o.BurnInCaptions)
	}
	text := "burn=" + burn + ";srt=" + strconv.FormatBool(o.SRT) + ";vtt=" + strconv.FormatBool(o.VTT) +
		";stems=" + strconv.FormatBool(o.Stems) + ";quality=" + o.Quality
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// encoder returns the encoder the options ask for.
func (o RenderOptions) encoder(base videostudio.Encoder) videostudio.Encoder {
	if o.Quality == qualityHigh {
		base.Quality = highQualityCRF
	}
	return base
}
```

**File `internal/storymaker/render/render.go`.** The function splits in two: the old body, with the long `return &Plan{...}` that N20 and A15 leave, becomes `prepareStory`, and the wrapper sets `Plan.Options`.

Old text:

```go
func (r *Runner) Prepare(ctx context.Context, projectID, ifMatch string, st Settings) (*Plan, error) {
```

New text:

```go
func (r *Runner) Prepare(ctx context.Context, projectID, ifMatch string, st Settings, opts RenderOptions) (*Plan, error) {
	plan, err := r.prepareStory(ctx, projectID, ifMatch, st, opts)
	if err != nil {
		return nil, err
	}
	plan.Options = opts
	return plan, nil
}

func (r *Runner) prepareStory(ctx context.Context, projectID, ifMatch string, st Settings, opts RenderOptions) (*Plan, error) {
```

Old text:

```go
	storymaker.ApplyDefaults(&story)
	issues = storymaker.Validate(&story, r.deps.Catalog, st.Limits)
```

New text:

```go
	storymaker.ApplyDefaults(&story)
	storymaker.ApplyCaptionOverride(&story, opts.BurnInCaptions)
	issues = storymaker.Validate(&story, r.deps.Catalog, st.Limits)
```

`Plan` ends with N20's `AudioJSON` field, and A15's `Music` field sits above it, so the new field goes last.

Old text:

```go
	AudioJSON []byte
}
```

New text:

```go
	AudioJSON []byte
	Options   RenderOptions
}
```

Old text:

```go
	run.encoder, run.encName = run.st.Encoder, run.st.EncoderName
```

New text:

```go
	run.encoder, run.encName = run.plan.Options.encoder(run.st.Encoder), run.st.EncoderName
```

**File `internal/storymaker/render/mixinput.go`.** A16 left a stub here. `run.plan` is the job's `*Plan`, so `Options` is reachable, and the disk check inside `buildMixInput` then covers the stems.

Old text:

```go
// mixStems says whether this job writes stems. Stems are asked for through the render options,
// which do not exist yet, so no job writes them.
func (run *renderRun) mixStems() bool { return false }
```

New text:

```go
// mixStems says whether this job writes stems. The disk check of buildMixInput counts them.
func (run *renderRun) mixStems() bool { return run.plan.Options.Stems }
```

**File `internal/server/storymaker_handlers.go`.**

Old text:

```go
		writeStorymakerError(w, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must be 1 to 128 printable characters.", nil)
		return
	}
```

New text:

```go
		writeStorymakerError(w, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must be 1 to 128 printable characters.", nil)
		return
	}
	const maxRenderOptionsBytes = 16 << 10
	optionsBody, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRenderOptionsBytes))
	if err != nil {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_request", "The export options are larger than 16 KiB.", nil)
		return
	}
	opts, err := render.ParseRenderOptions(optionsBody)
	if err != nil {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_request", "The export options are invalid: "+err.Error(), nil)
		return
	}
```

Old text:

```go
	plan, err := acc.rt.runner.Prepare(ctx, id, match, settings)
```

New text:

```go
	plan, err := acc.rt.runner.Prepare(ctx, id, match, settings, opts)
```

Old text:

```go
Fingerprint: id + "\x00" + match,
```

New text:

```go
Fingerprint: id + "\x00" + match + "\x00" + opts.Fingerprint(),
```

- [ ] **Step 5: Run**

Expected: all pass.

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
gofmt -w internal/storymaker/render/options.go internal/storymaker/render/options_test.go internal/storymaker/render/mixinput.go internal/storymaker/render/render.go internal/storymaker/render/render_test.go internal/storymaker/render/report_test.go internal/server/storymaker_handlers.go internal/server/storymaker_api_test.go
go vet ./internal/storymaker/render ./internal/server
go test ./internal/storymaker/render -count=1 -timeout 20m
go test ./internal/server -run '^TestStorymakerRender' -count=1 -timeout 10m
```

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/render/options.go internal/storymaker/render/options_test.go internal/storymaker/render/mixinput.go internal/storymaker/render/render.go internal/storymaker/render/render_test.go internal/storymaker/render/report_test.go internal/server/storymaker_handlers.go internal/server/storymaker_api_test.go
git commit -m "feat(storymaker): render options on POST /render" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A22: Subtitles and export extras

**Batch:** 2C  **Depends on:** A20, A21

**Files:**
- Create: `internal/storymaker/render/subtitles.go`
- Modify: `internal/storymaker/render/render.go`
- Modify: `internal/storymaker/render/jobs.go`
- Modify: `internal/server/storymaker_service.go`
- Test: `internal/storymaker/render/subtitles_test.go`
- Test: `internal/storymaker/render/render_test.go`

- [ ] **Step 1: Impact**

Run this from the main checkout. Per outline section 5: report callers, processes and risk, warn on HIGH or CRITICAL, confirm UNKNOWN with a text search.

```powershell
foreach ($symbol in 'finish','Output','Deps','newStorymakerRuntime') { node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact $symbol --direction upstream --repo . }
```

- [ ] **Step 2: Write the failing test**

Create `internal/storymaker/render/subtitles_test.go`:

```go
package render

import "testing"

func subtitleTestCaptions() []reportCaption {
	return []reportCaption{
		{StartMS: 500, EndMS: 2600, Lines: []string{"a --> b", "<b>&</b>"}, SceneID: "s1", BlockID: "s1:n"},
		// 61001 ms = 60000 + 1000 + 1. 3723004 ms = 3600000 + 120000 + 3000 + 4.
		{StartMS: 61001, EndMS: 3723004, Lines: []string{"  zwei\t\t  Worte\x01 ", "a\xc2\xa0b"}, SceneID: "s1", BlockID: "s1:n"},
		{StartMS: 3723004, EndMS: 3723500, Lines: []string{" \x01 "}, SceneID: "s1", BlockID: "s1:n"},
	}
}

// SubRip defines no entities. & stays as it is, and < > become U+2039 and U+203A, so no player reads a tag or an arrow.
func TestSubtitlesSRTKeepsAmpersandsAndBreaksTags(t *testing.T) {
	const lt, gt = "\xe2\x80\xb9", "\xe2\x80\xba"
	want := "1\n00:00:00,500 --> 00:00:02,600\na --" + gt + " b\n" + lt + "b" + gt + "&" + lt + "/b" + gt + "\n\n" +
		"2\n00:01:01,001 --> 01:02:03,004\nzwei Worte\na b\n\n"
	if got := string(WriteSRT(subtitleTestCaptions())); got != want {
		t.Errorf("SRT:\n%q\nwant:\n%q", got, want)
	}
}

func TestSubtitlesVTTBreaksArrowsAndEscapes(t *testing.T) {
	want := "WEBVTT\n\n00:00:00.500 --> 00:00:02.600\na --&gt; b\n&lt;b&gt;&amp;&lt;/b&gt;\n\n" +
		"00:01:01.001 --> 01:02:03.004\nzwei Worte\na b\n\n"
	if got := string(WriteVTT(subtitleTestCaptions())); got != want {
		t.Errorf("VTT:\n%q\nwant:\n%q", got, want)
	}
}
```

In `internal/storymaker/render/render_test.go`, let the test publisher fail one file:

Old text:

```go
type memPublisher struct {
	mu    sync.Mutex
	files map[string][]byte
	fail  error
}
```

New text:

```go
type memPublisher struct {
	mu     sync.Mutex
	files  map[string][]byte
	fail   error
	failOn func(path string) bool // one file fails alone
}
```

Old text:

```go
func (p *memPublisher) WriteStream(_ context.Context, path string, r io.Reader, max int64) error {
	if p.fail != nil {
		return p.fail
	}
```

New text:

```go
func (p *memPublisher) WriteStream(_ context.Context, path string, r io.Reader, max int64) error {
	if p.fail != nil {
		return p.fail
	}
	if p.failOn != nil && p.failOn(path) {
		return errors.New("disk full for this file")
	}
```

Old text:

```go
func (p *memPublisher) WriteBytes(_ context.Context, path string, data []byte) error {
	p.mu.Lock()
```

New text:

```go
func (p *memPublisher) WriteBytes(_ context.Context, path string, data []byte) error {
	if p.failOn != nil && p.failOn(path) {
		return errors.New("disk full for this file")
	}
	p.mu.Lock()
```

Append to the end of the file:

```go
// extrasFixture is a seeded narrated story whose report holds one caption.
func extrasFixture(t *testing.T) *runFixture {
	t.Helper()
	fx := newRunFixture(t)
	// 60 frames at 30 fps are 2 s, so the 1.5 s piece and the caption end at 1500 ms fit inside the film.
	events := fx.seedNarratedStory(t, narratedStoryDoc, 1.5)
	fx.playing(playOptions{frames: 60, mutateRep: func(rep map[string]any) {
		rep["audio_events"] = events
		rep["captions"] = []any{map[string]any{"start_ms": 100, "end_ms": 1500, "lines": []any{"Hallo Welt,"}, "scene_id": "s1", "block_id": "s1:n"}}
	}})
	return fx
}

func TestRunPublishesExtrasBesideTheMP4(t *testing.T) {
	fx := extrasFixture(t)
	job := fx.renderWith(t, RenderOptions{SRT: true, VTT: true, Stems: true, Quality: "standard"})
	if job.Status != StatusSucceeded || job.Output == nil || job.Output.Stems == nil {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	// The title "Mix" gives the slug "Mix".
	dir := "Documents/Storymaker/" + fx.projectID + "/exports/Mix-" + job.ID[:8]
	if job.Output.Stems.Narration != dir+"/narration.wav" || job.Output.Stems.Music != "" || job.Output.Stems.SFX != "" {
		t.Errorf("stems = %+v", job.Output.Stems)
	}
	if data, ok := fx.pub.get(dir + "/narration.wav"); !ok || !bytes.HasPrefix(data, []byte("RIFF")) {
		t.Error("the narration stem is not a WAV")
	}
	for _, silent := range []string{"/music.wav", "/sfx.wav"} {
		if _, ok := fx.pub.get(dir + silent); ok {
			t.Errorf("the silent stem %s was published", silent)
		}
	}
	if job.Output.SRT != dir+"/captions.srt" || job.Output.VTT != dir+"/captions.vtt" {
		t.Errorf("subtitles = %q %q", job.Output.SRT, job.Output.VTT)
	}
	if data, _ := fx.pub.get(job.Output.SRT); !strings.Contains(string(data), "Hallo Welt,") {
		t.Errorf("SRT = %q", data)
	}
	if data, _ := fx.pub.get(job.Output.VTT); !bytes.HasPrefix(data, []byte("WEBVTT\n")) {
		t.Errorf("VTT = %q", data)
	}
	raw, ok := fx.pub.get("Documents/Storymaker/" + fx.projectID + "/exports/" + job.ID + ".json")
	if !ok {
		t.Fatal("no export record")
	}
	var rec map[string]any
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	if rec["srt"] != job.Output.SRT || rec["vtt"] != job.Output.VTT {
		t.Errorf("record = %v", rec)
	}
	if stems, _ := rec["stems"].(map[string]any); stems["narration"] != job.Output.Stems.Narration {
		t.Errorf("record stems = %v", rec["stems"])
	}
	if _, present := rec["cue_precision"]; !present {
		t.Errorf("record has no cue_precision: %v", rec)
	}
	loudness, _ := rec["loudness"].(map[string]any)
	if i, _ := loudness["i"].(float64); i < -18.5 || i > -15.5 {
		t.Errorf("record loudness = %v", rec["loudness"])
	}
	if _, present := loudness["tp"]; !present {
		t.Errorf("record loudness = %v", rec["loudness"])
	}
	if _, present := loudness["master_gain_db"]; !present {
		t.Errorf("record loudness = %v", rec["loudness"])
	}
}

func TestRunFailedExtrasKeepTheMP4(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, fx *runFixture)
		opts  RenderOptions
		codes []string
		check func(t *testing.T, out *Output)
	}{
		{
			name: "a failed stem",
			setup: func(_ *testing.T, fx *runFixture) {
				fx.pub.failOn = func(p string) bool { return strings.HasSuffix(p, "/narration.wav") }
			},
			opts:  RenderOptions{Stems: true, Quality: "standard"},
			codes: []string{"stem_failed"},
			check: func(t *testing.T, out *Output) {
				if out.Stems == nil || out.Stems.Narration != "" {
					t.Errorf("stems = %+v", out.Stems)
				}
			},
		},
		{
			name: "a failed subtitle",
			setup: func(_ *testing.T, fx *runFixture) {
				fx.pub.failOn = func(p string) bool { return strings.HasSuffix(p, "/captions.srt") }
			},
			opts:  RenderOptions{SRT: true, VTT: true, Quality: "standard"},
			codes: []string{"subtitle_failed"},
			check: func(t *testing.T, out *Output) {
				if out.SRT != "" || out.VTT == "" {
					t.Errorf("subtitles = %q %q", out.SRT, out.VTT)
				}
			},
		},
		{
			name: "low workspace space",
			setup: func(t *testing.T, fx *runFixture) {
				fx.deps.WorkspaceFree = func() (int64, error) { return 1, nil }
				fx.rebuild(t)
			},
			opts:  RenderOptions{SRT: true, Stems: true, Quality: "standard"},
			codes: []string{"stem_failed", "subtitle_failed"},
			check: func(t *testing.T, out *Output) {
				if out.SRT != "" || out.Stems == nil || out.Stems.Narration != "" {
					t.Errorf("output = %+v", out)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := extrasFixture(t)
			tc.setup(t, fx)
			job := fx.renderWith(t, tc.opts)
			if job.Status != StatusSucceeded || job.Output == nil {
				t.Fatalf("job = %+v %+v", job, job.Error)
			}
			if _, ok := fx.pub.get(job.Output.Path); !ok {
				t.Error("the MP4 is gone")
			}
			for _, code := range tc.codes {
				if !hasWarning(job, code) {
					t.Errorf("warnings = %+v, want %s", job.Warnings, code)
				}
			}
			tc.check(t, job.Output)
		})
	}
}

func TestRunStemsOfASilentFilmAreAnEmptyObject(t *testing.T) {
	fx := newRunFixture(t)
	fx.playing(playOptions{frames: 12})
	job := fx.renderWith(t, RenderOptions{Stems: true, Quality: "standard"})
	if job.Status != StatusSucceeded || job.Output == nil || len(job.Warnings) != 0 {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	data, err := json.Marshal(job.Output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"stems":{}`) {
		t.Errorf("output = %s", data)
	}
}
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/render -run '(?i)(subtitles|extras|stems)' -count=1 -timeout 15m
```

Expected: compile failure, `undefined: WriteSRT`, `undefined: WriteVTT`, `job.Output.Stems undefined`, `fx.deps.WorkspaceFree undefined`.

- [ ] **Step 4: Implement**

Create `internal/storymaker/render/subtitles.go`:

```go
package render

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	"aurago/internal/storymaker"
)

// SubRip defines no entities, so many players would show "&amp;" as written, and they do read tags. So & stays as it is,
// and < and > become the close look-alikes U+2039 and U+203A, which also breaks -->.
var srtEscape = strings.NewReplacer("<", "\xe2\x80\xb9", ">", "\xe2\x80\xba")

// WebVTT defines entities, and its cue text needs them for & < >. That also breaks -->.
var vttEscape = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// WriteSRT writes the report's captions as SubRip. A caption with nothing left after the cleaning is dropped.
func WriteSRT(caps []reportCaption) []byte {
	var b bytes.Buffer
	n := 0
	for _, c := range caps {
		lines := subtitleLines(c.Lines, srtEscape)
		if len(lines) == 0 {
			continue
		}
		n++
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", n, subtitleTime(c.StartMS, ','), subtitleTime(c.EndMS, ','), strings.Join(lines, "\n"))
	}
	return b.Bytes()
}

// WriteVTT writes the report's captions as WebVTT.
func WriteVTT(caps []reportCaption) []byte {
	var b bytes.Buffer
	b.WriteString("WEBVTT\n\n")
	for _, c := range caps {
		lines := subtitleLines(c.Lines, vttEscape)
		if len(lines) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s --> %s\n%s\n\n", subtitleTime(c.StartMS, '.'), subtitleTime(c.EndMS, '.'), strings.Join(lines, "\n"))
	}
	return b.Bytes()
}

func subtitleLines(raw []string, escape *strings.Replacer) []string {
	var out []string
	for _, line := range raw {
		if clean := subtitleLine(line, escape); clean != "" {
			out = append(out, clean)
		}
	}
	return out
}

// subtitleLine strips controls, collapses runs of JS whitespace to one space and applies the format's escape.
func subtitleLine(s string, escape *strings.Replacer) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		switch {
		case storymaker.IsJSSpace(r):
			space = b.Len() > 0
			continue
		case unicode.IsControl(r):
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	return escape.Replace(b.String())
}

func subtitleTime(ms int64, sep byte) string {
	if ms < 0 {
		ms = 0
	}
	return fmt.Sprintf("%02d:%02d:%02d%c%03d", ms/3600000, ms/60000%60, ms/1000%60, sep, ms%1000)
}
```

**File `internal/storymaker/render/jobs.go`.**

Old text:

```go
// Output is the file a successful render wrote: its desktop path and size.
type Output struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}
```

New text:

```go
// OutputStems names the published stems. A silent stem is omitted.
type OutputStems struct {
	Narration string `json:"narration,omitempty"`
	Music     string `json:"music,omitempty"`
	SFX       string `json:"sfx,omitempty"`
}

// Output is what a successful render wrote. Stems is present ("{}") whenever stems were asked for.
type Output struct {
	Path    string       `json:"path"`
	Size    int64        `json:"size"`
	Stems   *OutputStems `json:"stems,omitempty"`
	SRT     string       `json:"srt,omitempty"`
	VTT     string       `json:"vtt,omitempty"`
	Library string       `json:"library,omitempty"`
}
```

**File `internal/storymaker/render/render.go`.** The `mixHook` and `mixAudio` lines are as A19 and A20 leave them. The `finish` lines are as A18 leaves them.

Old text:

```go
	// mixHook, when set, runs where the mix runs, on the mix context. Tests hold the start with it.
```

New text:

```go
	// WorkspaceFree returns the free bytes of the desktop workspace; nil skips the check.
	WorkspaceFree func() (int64, error)
	// mixHook, when set, runs where the mix runs, on the mix context. Tests hold the start with it.
```

Old text:

```go
	res, err := Mix(ctx, in, newMeter, MixOutputs{Mix: mix}, func(float64) {})
```

New text:

```go
	outputs := MixOutputs{Mix: mix}
	if run.plan.Options.Stems {
		outputs.Narration = filepath.Join(staging, "stem-narration.wav")
		outputs.Music = filepath.Join(staging, "stem-music.wav")
		outputs.SFX = filepath.Join(staging, "stem-sfx.wav")
	}
	res, err := Mix(ctx, in, newMeter, outputs, func(float64) {})
```

Old text:

```go
	meta := map[string]any{
```

New text:

```go
	published := Output{Path: outPath, Size: info.Size()}
	run.publishExtras(ctx, slug, exportDir, &published)
	meta := map[string]any{
```

Old text:

```go
	if data, err := json.MarshalIndent(meta, "", "  "); err == nil {
```

New text:

```go
	run.addRecordExtras(meta, published)
	if data, err := json.MarshalIndent(meta, "", "  "); err == nil {
```

Old text:

```go
	return Output{Path: outPath, Size: info.Size()}, nil
```

New text:

```go
	return published, nil
```

Old text:

```go
func (run *renderRun) currentEncoderName() string {
```

New text:

```go
// publishExtras writes the stems and subtitles into exports/<slug>-<job8>/. A failed extra leaves the MP4
// alone and becomes a warning.
func (run *renderRun) publishExtras(ctx context.Context, slug, exportDir string, out *Output) {
	opts := run.plan.Options
	dir := exportDir + "/" + slug + "-" + run.job.ID[:8]
	if opts.Stems {
		out.Stems = run.publishStems(ctx, dir)
	}
	if opts.SRT {
		out.SRT = run.publishSubtitle(ctx, dir+"/captions.srt", WriteSRT(run.report.Captions))
	}
	if opts.VTT {
		out.VTT = run.publishSubtitle(ctx, dir+"/captions.vtt", WriteVTT(run.report.Captions))
	}
}

// workspaceHolds refuses an extra the desktop workspace has no room for. The stream path has no check of its own.
func (run *renderRun) workspaceHolds(size int64) error {
	free := run.r.deps.WorkspaceFree
	if free == nil {
		return nil
	}
	n, err := free()
	if err != nil {
		return err
	}
	if n < size+minFreeStagingByte {
		return fmt.Errorf("only %d MB are free on the desktop workspace", n>>20)
	}
	return nil
}

func (run *renderRun) publishStems(ctx context.Context, dir string) *OutputStems {
	run.mu.Lock()
	peaks := run.mixResult.StemPeak
	run.mu.Unlock()
	stems := &OutputStems{}
	for i, stem := range []struct {
		name   string
		target *string
	}{{"narration", &stems.Narration}, {"music", &stems.Music}, {"sfx", &stems.SFX}} {
		if peaks[i] <= 0 {
			continue // a silent stem is omitted
		}
		target := dir + "/" + stem.name + ".wav"
		if err := run.publishStem(ctx, stem.name, target); err != nil {
			run.warn(warnStemFailed, "The "+stem.name+" stem could not be saved: "+run.scrubbed(err))
			continue
		}
		*stem.target = target
	}
	return stems
}

func (run *renderRun) publishStem(ctx context.Context, name, target string) error {
	file, err := os.Open(filepath.Join(filepath.Dir(run.output), "stem-"+name+".wav"))
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() > maxOutputBytes {
		return fmt.Errorf("the file is larger than %d GB", maxOutputBytes>>30)
	}
	if err := run.workspaceHolds(info.Size()); err != nil {
		return err
	}
	return run.r.deps.Publisher.WriteStream(ctx, target, file, maxOutputBytes)
}

func (run *renderRun) publishSubtitle(ctx context.Context, target string, data []byte) string {
	err := run.workspaceHolds(int64(len(data)))
	if err == nil {
		err = run.r.deps.Publisher.WriteBytes(ctx, target, data)
	}
	if err != nil {
		run.warn(warnSubtitleFailed, "The subtitle file "+path.Base(target)+" could not be saved: "+run.scrubbed(err))
		return ""
	}
	return target
}

// addRecordExtras adds the phase 2 fields to the export record. The loudness is the level after the master
// gain. MixResult stores a silent reading as -70, so the values are finite and JSON can hold them.
func (run *renderRun) addRecordExtras(meta map[string]any, out Output) {
	if out.Stems != nil {
		meta["stems"] = out.Stems
	}
	if out.SRT != "" {
		meta["srt"] = out.SRT
	}
	if out.VTT != "" {
		meta["vtt"] = out.VTT
	}
	if out.Library != "" {
		meta["library"] = out.Library
	}
	run.mu.Lock()
	warnings := append([]JobWarning(nil), run.warnings...)
	mixed, res := run.audioPath != "", run.mixResult
	run.mu.Unlock()
	if len(warnings) > 0 {
		meta["warnings"] = warnings
	}
	if mixed {
		meta["loudness"] = map[string]float64{"i": res.PreI + res.MasterGainDB, "tp": res.PreTP + res.MasterGainDB, "master_gain_db": res.MasterGainDB}
	}
	meta["cue_precision"] = run.report.CuePrecision
}

func (run *renderRun) currentEncoderName() string {
```

**File `internal/server/storymaker_service.go`.**

Old text:

```go
FrameObserver: s.storymakerFrameObserver,
```

New text:

```go
FrameObserver: s.storymakerFrameObserver,
		WorkspaceFree: s.storymakerWorkspaceFree,
```

Old text:

```go
// storymakerAvailable is the capability behind Requires: ["storymaker"]: the feature flag and an
```

New text:

```go
// storymakerWorkspaceFree is the free space of the desktop workspace, checked before each export extra.
func (s *Server) storymakerWorkspaceFree() (int64, error) {
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return 0, errors.New("configuration unavailable")
	}
	return fileutil.FreeDiskBytes(cfg.VirtualDesktop.WorkspaceDir)
}

// storymakerAvailable is the capability behind Requires: ["storymaker"]: the feature flag and an
```

The new function needs `fileutil` in the import block. N12 imports it in `storymaker_narrate.go` only, so this file has none yet.

Old text:

```go
	"aurago/internal/config"
	"aurago/internal/storymaker"
```

New text:

```go
	"aurago/internal/config"
	"aurago/internal/fileutil"
	"aurago/internal/storymaker"
```

- [ ] **Step 5: Run**

Expected: all pass.

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
gofmt -w internal/storymaker/render/subtitles.go internal/storymaker/render/subtitles_test.go internal/storymaker/render/render.go internal/storymaker/render/jobs.go internal/storymaker/render/render_test.go internal/server/storymaker_service.go
go vet ./internal/storymaker/render ./internal/server
go test ./internal/storymaker/render -count=1 -timeout 20m
go test ./internal/server -run '^TestStorymakerRender' -count=1 -timeout 10m
```

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/render/subtitles.go internal/storymaker/render/subtitles_test.go internal/storymaker/render/render.go internal/storymaker/render/jobs.go internal/storymaker/render/render_test.go internal/server/storymaker_service.go
git commit -m "feat(storymaker): subtitles and stems beside the export" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A23: Library copy

**Batch:** 2C  **Depends on:** A22

**Files:**
- Modify: `internal/desktop/service_media.go`
- Modify: `internal/storymaker/render/render.go`
- Modify: `internal/server/storymaker_store.go`
- Modify: `internal/server/storymaker_service.go`
- Test: `internal/desktop/service_storymaker_video_test.go`
- Test: `internal/storymaker/render/render_test.go`

- [ ] **Step 1: Impact**

Run this from the main checkout. Per outline section 5: report callers, processes and risk, warn on HIGH or CRITICAL, confirm UNKNOWN with a text search. `publishExtras` came with A22, so the stale index may not know it. Confirm that one with a text search.

```powershell
foreach ($symbol in 'publishExtras','Deps','newStorymakerRuntime') { node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact $symbol --direction upstream --repo . }
```

- [ ] **Step 2: Write the failing test**

Create `internal/desktop/service_storymaker_video_test.go`:

```go
package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func storymakerVideoDir(svc *Service) string {
	return filepath.Join(svc.Config().DataDir, "generated_videos", "Storymaker")
}

func TestWriteStorymakerVideoCreatesTheFolderAndAvoidsCollisions(t *testing.T) {
	svc := testMediaService(t)
	ctx := context.Background()
	dir := storymakerVideoDir(svc)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("a fresh data dir already has the folder: %v", err)
	}
	for i, want := range []string{"Videos/Storymaker/clip.mp4", "Videos/Storymaker/clip-2.mp4", "Videos/Storymaker/clip-3.mp4"} {
		body := strings.Repeat("v", i+1)
		got, err := svc.WriteStorymakerVideo(ctx, "clip", strings.NewReader(body), int64(len(body)), SourceUser)
		if err != nil || got != want {
			t.Fatalf("copy %d = %q, %v, want %q", i+1, got, err, want)
		}
		data, err := os.ReadFile(filepath.Join(dir, filepath.Base(want)))
		if err != nil || string(data) != body {
			t.Fatalf("copy %d holds %q, %v", i+1, data, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 3 {
		t.Fatalf("the folder holds %v, %v, want the three copies and no staging file", entries, err)
	}
}

func TestWriteStorymakerVideoRefusals(t *testing.T) {
	svc := testMediaService(t)
	ctx := context.Background()
	for _, slug := range []string{"", "../clip", "a/b", `a\b`, "a b", "clip.mp4", strings.Repeat("a", 81)} {
		if _, err := svc.WriteStorymakerVideo(ctx, slug, strings.NewReader("v"), 1, SourceUser); err == nil {
			t.Errorf("slug %q was accepted", slug)
		}
	}
	// 2^62 bytes are more than any test box has free, and the limit is reached before a byte is read.
	if _, err := svc.WriteStorymakerVideo(ctx, "clip", strings.NewReader("v"), int64(1)<<62, SourceUser); err == nil || !strings.Contains(err.Error(), "free space") {
		t.Errorf("low disk = %v", err)
	}
	if _, err := svc.WriteStorymakerVideo(ctx, "clip", strings.NewReader("short"), 99, SourceUser); err == nil {
		t.Error("a source shorter than its size was accepted")
	}
	if _, err := svc.WriteStorymakerVideo(ctx, "clip", strings.NewReader("too long"), 3, SourceUser); err == nil {
		t.Error("a source longer than its size was accepted")
	}
	svc.SetReadOnly(true)
	if _, err := svc.WriteStorymakerVideo(ctx, "clip", strings.NewReader("v"), 1, SourceUser); err == nil {
		t.Error("a read-only desktop accepted the copy")
	}
	svc.SetReadOnly(false)
	entries, _ := os.ReadDir(storymakerVideoDir(svc))
	if len(entries) != 0 {
		t.Errorf("refused copies left %v", entries)
	}
}
```

Append to the end of `internal/storymaker/render/render_test.go`:

```go
func TestRunLibraryCopy(t *testing.T) {
	fx := newRunFixture(t)
	var slug string
	var size, read int64
	fx.deps.Library = func(_ context.Context, s string, r io.Reader, n int64) (string, error) {
		copied, err := io.Copy(io.Discard, r)
		slug, size, read = s, n, copied
		return "Videos/Storymaker/" + s + ".mp4", err
	}
	fx.rebuild(t)
	fx.playing(playOptions{frames: 12})
	job := fx.render(t)
	if job.Status != StatusSucceeded || job.Output == nil {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	// The title of the fixture, "Was ist eine Firewall?", gives this slug (see TestRunRendersAndPublishesTheVideo).
	if slug != "Was-ist-eine-Firewall" || job.Output.Library != "Videos/Storymaker/Was-ist-eine-Firewall.mp4" {
		t.Errorf("slug %q, library %q", slug, job.Output.Library)
	}
	if size != job.Output.Size || read != job.Output.Size {
		t.Errorf("the copy got size %d and %d bytes, the MP4 has %d", size, read, job.Output.Size)
	}
	raw, _ := fx.pub.get("Documents/Storymaker/" + fx.projectID + "/exports/" + job.ID + ".json")
	if !strings.Contains(string(raw), `"library": "Videos/Storymaker/Was-ist-eine-Firewall.mp4"`) {
		t.Errorf("record = %s", raw)
	}
}

func TestRunFailedLibraryCopyKeepsTheMP4(t *testing.T) {
	fx := newRunFixture(t)
	fx.deps.Library = func(context.Context, string, io.Reader, int64) (string, error) {
		return "", errors.New("the library disk is full")
	}
	fx.rebuild(t)
	fx.playing(playOptions{frames: 12})
	job := fx.render(t)
	if job.Status != StatusSucceeded || job.Output == nil {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
	if job.Output.Library != "" || !hasWarning(job, "library_failed") {
		t.Errorf("library %q, warnings %+v", job.Output.Library, job.Warnings)
	}
	if _, ok := fx.pub.get(job.Output.Path); !ok {
		t.Error("the MP4 is gone")
	}
}

func TestRunLowWorkspaceSkipsTheLibraryCopy(t *testing.T) {
	fx := newRunFixture(t)
	var called atomic.Bool
	fx.deps.Library = func(context.Context, string, io.Reader, int64) (string, error) {
		called.Store(true)
		return "Videos/Storymaker/x.mp4", nil
	}
	fx.deps.WorkspaceFree = func() (int64, error) { return 1, nil }
	fx.rebuild(t)
	fx.playing(playOptions{frames: 12})
	job := fx.render(t)
	if job.Status != StatusSucceeded || !hasWarning(job, "library_failed") || called.Load() {
		t.Fatalf("job = %+v %+v, library called %v", job, job.Error, called.Load())
	}
}
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/desktop -run WriteStorymakerVideo -count=1
go test ./internal/storymaker/render -run '(?i)library' -count=1 -timeout 10m
```

Expected: compile failure, `svc.WriteStorymakerVideo undefined`, `fx.deps.Library undefined`.

- [ ] **Step 4: Implement**

**File `internal/desktop/service_media.go`.**

Old text:

```go
import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/dbutil"
)
```

New text:

```go
import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"aurago/internal/dbutil"
	"aurago/internal/fileutil"
)

const (
	storymakerVideoFolder = "Storymaker"
	// storymakerVideoMargin is the free space a library copy must leave.
	storymakerVideoMargin = int64(64) << 20
)

var storymakerVideoSlug = regexp.MustCompile(`^[A-Za-z0-9-]{1,80}$`)
```

Old text:

```go
func mediaRelativePath(mount mediaMount, absPath string) string {
```

New text:

```go
// WriteStorymakerVideo copies r into Videos/Storymaker/<slug>.mp4, or <slug>-2.mp4, <slug>-3.mp4 and so on.
// slug must match ^[A-Za-z0-9-]{1,80}$; the path is built here. It returns the desktop path. Slow source I/O
// never holds the shared mutation lock. RegisterMedia is skipped on purpose.
func (s *Service) WriteStorymakerVideo(ctx context.Context, slug string, r io.Reader, size int64, source string) (string, error) {
	if !storymakerVideoSlug.MatchString(slug) {
		return "", fmt.Errorf("invalid storymaker video name")
	}
	if r == nil || size <= 0 {
		return "", fmt.Errorf("invalid storymaker video size")
	}
	if err := s.ensureReady(ctx); err != nil {
		return "", err
	}
	if s.Config().ReadOnly {
		return "", fmt.Errorf("virtual desktop is read-only")
	}
	mount, _, _, mounted, err := s.resolveMediaMount("Videos/" + storymakerVideoFolder)
	if err != nil {
		return "", err
	}
	if !mounted {
		return "", fmt.Errorf("the Videos media mount is not available")
	}
	free, err := fileutil.FreeDiskBytes(mount.Dir)
	if err != nil {
		return "", fmt.Errorf("check free space: %w", err)
	}
	if free <= size+storymakerVideoMargin {
		return "", fmt.Errorf("not enough free space for the library copy (%d MB free)", free>>20)
	}
	root, err := os.OpenRoot(mount.Dir)
	if err != nil {
		return "", fmt.Errorf("open videos root: %w", err)
	}
	defer root.Close()
	// Only the mount root exists at start, so the first copy creates the folder.
	if err := root.MkdirAll(storymakerVideoFolder, 0o700); err != nil {
		return "", fmt.Errorf("create the storymaker video folder: %w", err)
	}
	temp := filepath.Join(storymakerVideoFolder, ".aurago-write-"+rand.Text())
	out, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("stage the library copy: %w", err)
	}
	defer root.Remove(temp)
	n, writeErr := io.Copy(out, io.LimitReader(desktopContextReader{ctx, r}, size+1))
	if writeErr == nil && n != size {
		writeErr = fmt.Errorf("the source has %d bytes, expected %d", n, size)
	}
	if writeErr == nil {
		writeErr = out.Sync()
	}
	closeErr := out.Close()
	if writeErr != nil {
		return "", fmt.Errorf("write the library copy: %w", writeErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close the library copy: %w", closeErr)
	}

	desktopMutationMu.Lock()
	defer desktopMutationMu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s.Config().ReadOnly {
		return "", fmt.Errorf("virtual desktop is read-only")
	}
	name, err := freeStorymakerVideoName(root, slug)
	if err != nil {
		return "", err
	}
	if err := fileutil.RenameRootContext(ctx, root, temp, filepath.Join(storymakerVideoFolder, name)); err != nil {
		return "", fmt.Errorf("publish the library copy: %w", err)
	}
	desktopPath := "Videos/" + storymakerVideoFolder + "/" + name
	s.invalidateListCache()
	_ = s.Audit(ctx, "write_file", desktopPath, map[string]interface{}{"bytes": n}, source)
	return desktopPath, nil
}

// freeStorymakerVideoName finds the first name that is not taken. The caller holds desktopMutationMu.
func freeStorymakerVideoName(root *os.Root, slug string) (string, error) {
	for i := 1; i <= 100000; i++ {
		name := slug + ".mp4"
		if i > 1 {
			name = fmt.Sprintf("%s-%d.mp4", slug, i)
		}
		_, err := root.Lstat(filepath.Join(storymakerVideoFolder, name))
		if os.IsNotExist(err) {
			return name, nil
		}
		if err != nil {
			return "", fmt.Errorf("stat the library copy: %w", err)
		}
	}
	return "", fmt.Errorf("no free library name for %s", slug)
}

func mediaRelativePath(mount mediaMount, absPath string) string {
```

**File `internal/storymaker/render/render.go`.** The old texts are as A22 leaves them.

Old text:

```go
	// WorkspaceFree returns the free bytes of the desktop workspace; nil skips the check.
```

New text:

```go
	// Library copies the finished MP4 into the video library and returns its desktop path; nil skips the copy.
	Library func(ctx context.Context, slug string, r io.Reader, size int64) (string, error)
	// WorkspaceFree returns the free bytes of the desktop workspace; nil skips the check.
```

Old text:

```go
	if opts.VTT {
		out.VTT = run.publishSubtitle(ctx, dir+"/captions.vtt", WriteVTT(run.report.Captions))
	}
}
```

New text:

```go
	if opts.VTT {
		out.VTT = run.publishSubtitle(ctx, dir+"/captions.vtt", WriteVTT(run.report.Captions))
	}
	if run.r.deps.Library != nil {
		if err := run.publishLibrary(ctx, slug, out); err != nil {
			run.warn(warnLibraryFailed, "The copy in Videos/Storymaker could not be written: "+run.scrubbed(err))
		}
	}
}

func (run *renderRun) publishLibrary(ctx context.Context, slug string, out *Output) error {
	if err := run.workspaceHolds(out.Size); err != nil {
		return err
	}
	file, err := os.Open(run.output)
	if err != nil {
		return err
	}
	defer file.Close()
	target, err := run.r.deps.Library(ctx, slug, file, out.Size)
	if err != nil {
		return err
	}
	out.Library = target
	return nil
}
```

**File `internal/server/storymaker_store.go`.**

Old text:

```go
// WriteBytes implements render.Publisher: a small file, replaced when it exists.
```

New text:

```go
// WriteLibraryVideo is render.Deps.Library: the create-only copy in Videos/Storymaker.
func (st *storymakerStore) WriteLibraryVideo(ctx context.Context, slug string, r io.Reader, size int64) (string, error) {
	svc, err := st.service()
	if err != nil {
		return "", err
	}
	return svc.WriteStorymakerVideo(ctx, slug, r, size, desktop.SourceUser)
}

// WriteBytes implements render.Publisher: a small file, replaced when it exists.
```

**File `internal/server/storymaker_service.go`.** The old text is as A22 leaves it.

Old text:

```go
		WorkspaceFree: s.storymakerWorkspaceFree,
```

New text:

```go
		WorkspaceFree: s.storymakerWorkspaceFree,
		Library:       store.WriteLibraryVideo,
```

- [ ] **Step 5: Run**

Expected: all pass.

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'; $env:GOTMPDIR = 'D:\tmp\go-build'
gofmt -w internal/desktop/service_media.go internal/desktop/service_storymaker_video_test.go internal/storymaker/render/render.go internal/storymaker/render/render_test.go internal/server/storymaker_store.go internal/server/storymaker_service.go
go vet ./internal/desktop ./internal/storymaker/render ./internal/server
go test ./internal/desktop -run WriteStorymakerVideo -count=1
go test ./internal/storymaker/render -count=1 -timeout 20m
go test ./internal/server -run Storymaker -count=1 -timeout 20m
```

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/desktop/service_media.go internal/desktop/service_storymaker_video_test.go internal/storymaker/render/render.go internal/storymaker/render/render_test.go internal/server/storymaker_store.go internal/server/storymaker_service.go
git commit -m "feat(storymaker): library copy in Videos/Storymaker" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

<!-- G-A-6 -->

### Task A24: `narration_incomplete` and `music_incomplete` gates
**Batch:** 2C  **Depends on:** A14, A15, A21
**Files:**
- Modify: `internal/storymaker/render/render.go`
- Modify: `internal/storymaker/render/jobs.go`
- Modify: `internal/server/storymaker_handlers.go`
- Test: `internal/server/storymaker_api_test.go`
- Test: `internal/server/storymaker_e2e_test.go`
- Test: `internal/server/storymaker_parity_browser_test.go`
- Test: `internal/storymaker/render/browser_smoke_test.go`
- Test: `internal/storymaker/render/render_test.go`
- Test: `internal/storymaker/render/report_test.go`

- [ ] **Step 1: Impact**

Run from the main checkout `C:/Users/Andi/Documents/repo/AuraGo`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "Prepare" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "PrepareError" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "handleStorymakerRender" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "loadFirewall" --direction upstream --repo .
```

Report callers, processes and risk. Warn on HIGH or CRITICAL, and confirm `risk: UNKNOWN` with a text search (outline section 5, GitNexus).

- [ ] **Step 2: Write the failing test**

In `internal/server/storymaker_api_test.go`, add the import.

Old text:

```go
	"aurago/internal/storymaker/render"
```

New text:

```go
	"aurago/internal/storymaker/render"
	"aurago/internal/storymaker/storymakertest"
```

Append to the end of `internal/server/storymaker_api_test.go`:

```go
// seedStoryNarrationForRender stores generated audio for every missing piece of the project's story, because
// POST /render refuses a narrated story without it. With 2 s per piece firewall.json stays above the 250 frames the
// real renders pin: s1 139 frames, s2 at least 85, s3 150, minus two 15-frame transition overlaps.
func seedStoryNarrationForRender(t *testing.T, s *Server, id string) {
	t.Helper()
	ctx := context.Background()
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	data, _, err := rt.projects.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	story, _, err := storymaker.DecodeStory(data, storymaker.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	storymaker.ApplyDefaults(&story)
	if err := storymakertest.SeedNarration(ctx, rt.projects, id, &story, 2); err != nil {
		t.Fatal(err)
	}
}

func TestStorymakerRenderRefusesMissingAudio(t *testing.T) {
	s, token, p := renderFixture(t, func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		return render.Output{}, nil
	})
	id := p.Project.ID
	etag := p.ETag
	save := func(doc string) {
		t.Helper()
		w := storymakerRequest(s, token, http.MethodPut, "/projects/"+id+"/story", strings.NewReader(doc), map[string]string{"If-Match": etag})
		var saved struct {
			ETag string `json:"etag"`
		}
		decodeBody(t, w, &saved)
		if w.Code != http.StatusOK {
			t.Fatalf("save: %d %s", w.Code, w.Body.String())
		}
		etag = saved.ETag
	}
	post := func() *httptest.ResponseRecorder {
		return storymakerRequest(s, token, http.MethodPost, "/projects/"+id+"/render", nil, map[string]string{"If-Match": etag})
	}
	type refusal struct {
		Code       string                    `json:"code"`
		Missing    []storymaker.MissingBlock `json:"missing"`
		Chars      storymaker.CharCounts     `json:"chars"`
		Key        *string                   `json:"key"`
		MusicIssue string                    `json:"music_issue"`
	}
	refused := func(want string) refusal {
		t.Helper()
		w := post()
		var got refusal
		decodeBody(t, w, &got)
		if w.Code != http.StatusConflict || got.Code != want {
			t.Fatalf("render = %d %s, want 409 %s", w.Code, w.Body.String(), want)
		}
		return got
	}
	accepted := func() {
		t.Helper()
		if w := post(); w.Code != http.StatusAccepted {
			t.Fatalf("render = %d %s, want 202", w.Code, w.Body.String())
		}
	}

	// A narrated story without audio. "Hallo." is one piece of 6 code points in block s1:n.
	save(`{"schema":"storymaker/1","title":"Gate","scenes":[{"id":"s1","narration":"Hallo."}]}`)
	got := refused("narration_incomplete")
	if len(got.Missing) != 1 || got.Missing[0] != (storymaker.MissingBlock{SceneID: "s1", BlockID: "s1:n", Chars: 6}) {
		t.Fatalf("missing = %+v", got.Missing)
	}
	if got.Chars != (storymaker.CharCounts{Total: 6, Missing: 6}) {
		t.Fatalf("chars = %+v", got.Chars)
	}

	// Seeded, the same story passes.
	seedStoryNarrationForRender(t, s, id)
	accepted()

	// A silent scene has no narration block, so its text needs no audio.
	save(`{"schema":"storymaker/1","title":"Gate","scenes":[{"id":"s1","silent":true,"narration":"Diese Szene bleibt stumm."}]}`)
	accepted()

	// Music on without its file names the key.
	key := storymaker.MusicKeyGenerated("calm-ambient", "")
	save(`{"schema":"storymaker/1","title":"Gate","music":{"mode":"generated","mood":"calm-ambient"},"scenes":[{"id":"s1"}]}`)
	got = refused("music_incomplete")
	if got.Key == nil || *got.Key != key || got.MusicIssue != "" {
		t.Fatalf("music refusal = %+v", got)
	}
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if err := storymakertest.SeedMusic(context.Background(), rt.projects, id, key, 2); err != nil {
		t.Fatal(err)
	}
	accepted()

	// A moved source has no key, and says why.
	save(`{"schema":"storymaker/1","title":"Gate","music":{"mode":"file","file":"Documents/gone.wav"},"scenes":[{"id":"s1"}]}`)
	got = refused("music_incomplete")
	if got.Key == nil || *got.Key != "" || got.MusicIssue != "source_missing" {
		t.Fatalf("moved source refusal = %+v", got)
	}

	// Volume 0 plays nothing, so it needs no file. This mood has no stored file under the key of "dramatic".
	save(`{"schema":"storymaker/1","title":"Gate","music":{"mode":"generated","mood":"dramatic","volume":0},"scenes":[{"id":"s1"}]}`)
	accepted()
}
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/server -run '^TestStorymakerRenderRefusesMissingAudio$' -count=1
```

Expected: FAIL with `render = 202 ..., want 409 narration_incomplete`, because no gate exists yet.

- [ ] **Step 4: Implement**

**`internal/storymaker/render/jobs.go`.** This task adds these two codes. No earlier task declares them.

Old text:

```go
// ErrBusy means the render queue already holds DefaultMaxQueued waiting jobs.
```

New text:

```go
// Codes of the audio gates on POST /render.
const (
	CodeNarrationIncomplete = "narration_incomplete"
	CodeMusicIncomplete     = "music_incomplete"
)

// ErrBusy means the render queue already holds DefaultMaxQueued waiting jobs.
```

**`internal/storymaker/render/render.go`, the error type.**

Old text:

```go
	// Stale is true when the story changed since the caller read it (HTTP 412).
	Stale bool
}
```

New text:

```go
	// Stale is true when the story changed since the caller read it (HTTP 412).
	Stale bool
	// Missing and Chars go with CodeNarrationIncomplete.
	Missing []storymaker.MissingBlock
	Chars   storymaker.CharCounts
	// MusicKey and MusicIssue go with CodeMusicIncomplete. A moved source has no key and the issue "source_missing".
	MusicKey   string
	MusicIssue string
}
```

**`internal/storymaker/render/render.go`, N20's `AudioAnswer` call.** The music refusal names the key, and `AudioAnswer` has no field that returns it. So the key comes from the one resolve this call makes, and the gate needs no second one.

Old text:

```go
	answer, err := r.deps.Projects.AudioAnswer(ctx, projectID, &story, r.deps.MusicKey)
```

New text:

```go
	// The gate names the music key in its refusal, and the answer has no key while Music is null. Keep what the one
	// resolve of this answer returned.
	musicKey := ""
	resolve := r.deps.MusicKey
	if resolve != nil {
		resolve = func(ctx context.Context, m *storymaker.Music) (string, error) {
			key, err := r.deps.MusicKey(ctx, m)
			musicKey = key
			return key, err
		}
	}
	answer, err := r.deps.Projects.AudioAnswer(ctx, projectID, &story, resolve)
```

**`internal/storymaker/render/render.go`, the gate in `Prepare`.** This line sits after validation and before the bundles, and N20, A15 and A21 leave it as it is. N20's `answer` is in scope there.

Old text:

```go
	bundles := map[string][]byte{}
```

New text:

```go
	if err := checkAudioGates(answer, musicKey); err != nil {
		return nil, err
	}
	bundles := map[string][]byte{}
```

**`internal/storymaker/render/render.go`, the gate itself.**

Old text:

```go
// CanvasSize returns the export size of an aspect: 1920x1080, 1080x1920 or 1080x1080.
```

New text:

```go
// checkAudioGates refuses a story whose stored audio is not complete: narration first, then music. It reads nothing
// itself and judges the answer the plan is built from, so a music job that ends during Prepare cannot pass this gate
// and leave the plan without music. musicKey is what the resolver returned for that answer.
// Volume 0 has one owner, the resolver: it returns "", so that music needs nothing.
func checkAudioGates(answer storymaker.AudioAnswer, musicKey string) error {
	if len(answer.Missing) > 0 {
		return &PrepareError{
			Code:    CodeNarrationIncomplete,
			Message: fmt.Sprintf("%d narration blocks have no audio yet. Generate the narration first.", len(answer.Missing)),
			Missing: answer.Missing,
			Chars:   answer.Chars,
		}
	}
	if answer.MusicIssue != "" {
		return &PrepareError{Code: CodeMusicIncomplete, Message: "The music source no longer exists. Choose another file.", MusicIssue: answer.MusicIssue}
	}
	if answer.Music != nil || musicKey == "" {
		return nil
	}
	return &PrepareError{Code: CodeMusicIncomplete, Message: "The background music is not stored yet. Generate it first.", MusicKey: musicKey}
}

// CanvasSize returns the export size of an aspect: 1920x1080, 1080x1920 or 1080x1080.
```

**`internal/server/storymaker_handlers.go`.** The capability checks above this switch stay ahead of `Prepare`.

Old text:

```go
			case pe.Code == render.CodeCodeError:
```

New text:

```go
			case pe.Code == render.CodeNarrationIncomplete:
				writeStorymakerErrorWith(w, http.StatusConflict, render.CodeNarrationIncomplete, pe.Message, map[string]any{"missing": pe.Missing, "chars": pe.Chars})
			case pe.Code == render.CodeMusicIncomplete:
				extra := map[string]any{"key": pe.MusicKey}
				if pe.MusicIssue != "" {
					extra["music_issue"] = pe.MusicIssue
				}
				writeStorymakerErrorWith(w, http.StatusConflict, render.CodeMusicIncomplete, pe.Message, extra)
			case pe.Code == render.CodeCodeError:
```

**`internal/server/storymaker_api_test.go`, the render test.** Its story has narration, and the capability checks at the two `Conflict` requests above this point stay ahead of the gate.

Old text:

```go
	// A valid request queues a job; the same Idempotency-Key returns it again.
```

New text:

```go
	// A valid request queues a job; the same Idempotency-Key returns it again.
	seedStoryNarrationForRender(t, s, p.Project.ID)
```

**`internal/server/storymaker_e2e_test.go`.**

Old text:

```go
	// Follow the export on the event stream, opened before the render starts.
```

New text:

```go
	// The render gate needs stored narration.
	seedStoryNarrationForRender(t, s, id)

	// Follow the export on the event stream, opened before the render starts.
```

**`internal/server/storymaker_parity_browser_test.go`.** This test stays red in its browser-only run until A26, which sends the same audio to the app side. No matrix runs it before then.

Old text:

```go
	// The export.
```

New text:

```go
	// The render gate needs stored narration.
	seedStoryNarrationForRender(t, s, p.Project.ID)

	// The export.
```

`internal/server/storymaker_app_browser_test.go` needs no edit. E1k's tour stores the audio through the real narrate job right before its export, so the gate passes.

**`internal/storymaker/render/browser_smoke_test.go`, the import.**

Old text:

```go
	"aurago/internal/storymaker"
```

New text:

```go
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/storymakertest"
```

**`internal/storymaker/render/browser_smoke_test.go`, `loadFirewall`.**

Old text:

```go
	if _, res, err := fx.projects.SaveCodeScene(ctx, fx.projectID, "s3", lf(src), ""); err != nil || len(res.Errors) != 0 {
		t.Fatalf("SaveCodeScene: %v %+v", err, res)
	}
}
```

New text:

```go
	if _, res, err := fx.projects.SaveCodeScene(ctx, fx.projectID, "s3", lf(src), ""); err != nil || len(res.Errors) != 0 {
		t.Fatalf("SaveCodeScene: %v %+v", err, res)
	}
	fx.seedStoredNarration(t)
}

// seedStoredNarration stores generated audio for every missing piece of the project's story, because Prepare refuses
// a narrated story without it. With 2 s per piece firewall.json stays above the 250 frames TestRealRenderOfTheFirewallStory pins:
// s1 139 frames, s2 at least 85, s3 150, minus two 15-frame transition overlaps.
func (fx *runFixture) seedStoredNarration(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	data, _, err := fx.projects.LoadStory(ctx, fx.projectID)
	if err != nil {
		t.Fatal(err)
	}
	story, _, err := storymaker.DecodeStory(data, fx.settings.Limits)
	if err != nil {
		t.Fatal(err)
	}
	storymaker.ApplyDefaults(&story)
	if err := storymakertest.SeedNarration(ctx, fx.projects, fx.projectID, &story, 2); err != nil {
		t.Fatal(err)
	}
}
```

**`internal/storymaker/render/render_test.go`, `TestPrepareCollectsCodeSceneBundles`.** The first `Prepare` of that test still fails on the missing code scene, which validation reports before the gate.

Old text:

```go
	if _, res, err := fx.projects.SaveCodeScene(ctx, fx.projectID, "s3", bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n")), ""); err != nil || len(res.Errors) != 0 {
		t.Fatalf("SaveCodeScene: %v %+v", err, res)
	}
```

New text:

```go
	if _, res, err := fx.projects.SaveCodeScene(ctx, fx.projectID, "s3", bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n")), ""); err != nil || len(res.Errors) != 0 {
		t.Fatalf("SaveCodeScene: %v %+v", err, res)
	}
	fx.seedStoredNarration(t)
```

**`internal/storymaker/render/render_test.go`, `TestBootCarriesThePlanAudioInfo`.** The gate refuses the unseeded story, so the test takes its blocks from the decoded story and not from a plan.

Old text:

```go
	unsynthesised, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings, RenderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if string(unsynthesised.AudioJSON) != `{"narration":{}}` {
		t.Errorf("AudioJSON without audio = %s", unsynthesised.AudioJSON)
	}
	if len(unsynthesised.Blocks) != 1 || unsynthesised.Blocks[0].ID != "s1:n" {
		t.Fatalf("Blocks = %+v", unsynthesised.Blocks)
	}
	seedPieces(t, fx, unsynthesised.Blocks)
```

New text:

```go
	// The gate refuses the story while its pieces have no audio.
	_, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings, RenderOptions{})
	var pe *PrepareError
	if !errors.As(err, &pe) || pe.Code != CodeNarrationIncomplete {
		t.Fatalf("Prepare without audio = %v, want %s", err, CodeNarrationIncomplete)
	}
	story, _, err := storymaker.DecodeStory([]byte(narratedStoryJSON), fx.settings.Limits)
	if err != nil {
		t.Fatal(err)
	}
	storymaker.ApplyDefaults(&story)
	blocks := storymaker.Blocks(&story)
	if len(blocks) != 1 || blocks[0].ID != "s1:n" {
		t.Fatalf("Blocks = %+v", blocks)
	}
	seedPieces(t, fx, blocks)
```

**`internal/storymaker/render/report_test.go`, the import.** N21 and S3 leave no `storymaker` import in this file.

Old text:

```go
	"github.com/go-rod/rod/lib/proto"
```

New text:

```go
	"aurago/internal/storymaker"

	"github.com/go-rod/rod/lib/proto"
```

**`internal/storymaker/render/report_test.go`, `TestRunRefusesAReportThatDropsMeasuredNarration`.** The pieces are seeded before the first `Prepare`, which `fx.render` now makes.

Old text:

```go
	plan, err := fx.runner.Prepare(ctx, fx.projectID, "", fx.settings, RenderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	seedPieces(t, fx, plan.Blocks)
```

New text:

```go
	story, _, err := storymaker.DecodeStory([]byte(narratedStoryJSON), fx.settings.Limits)
	if err != nil {
		t.Fatal(err)
	}
	storymaker.ApplyDefaults(&story)
	seedPieces(t, fx, storymaker.Blocks(&story))
```

**`internal/storymaker/render/report_test.go`, `TestRunAcceptsCaptionsOfAnEstimatedBlock`.** Delete this test and its comment, because the gate ends the path where a narrated story without audio reaches a render. N21's `TestParseReportAcceptsHonestAudioReports` keeps pinning the parser case "a caption of an estimated block".

Old text:

```go
// The page fills captions for estimated blocks too, and a narrated story without audio still exports silently.
func TestRunAcceptsCaptionsOfAnEstimatedBlock(t *testing.T) {
	fx := newRunFixture(t)
	if _, err := fx.projects.SaveStory(context.Background(), fx.projectID, []byte(narratedStoryJSON), ""); err != nil {
		t.Fatal(err)
	}
	// 12 frames at 30 fps give duration_ms 400, so the caption ends with the film.
	fx.playing(playOptions{frames: 12, mutateRep: func(rep map[string]any) {
		rep["captions"] = []any{map[string]any{"start_ms": 0, "end_ms": 400, "lines": []any{"Hallo Welt, das ist der erste Satz."}, "scene_id": "s1", "block_id": "s1:n"}}
	}})
	if job := fx.render(t); job.Status != StatusSucceeded {
		t.Fatalf("job = %+v %+v", job, job.Error)
	}
}

func TestEgressGuardDecisions(t *testing.T) {
```

New text:

```go
func TestEgressGuardDecisions(t *testing.T) {
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/server -run '^TestStorymaker(RenderRefusesMissingAudio|RenderEndpointChecksAndQueue|RenderQueueIsBounded)$' -count=1
go test ./internal/storymaker/render -count=1 -timeout 20m
go vet ./internal/server ./internal/storymaker/render
gofmt -l internal/server internal/storymaker/render
```

Expected: PASS on both test lines, no vet finding, and `gofmt -l` prints nothing. `go vet` compiles the browser tests that this task edited.

- [ ] **Step 6: Commit**

Run `detect_changes` first. A PARTIAL answer on new files needs a grep confirmation in the commit body (outline section 5).

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/render/render.go internal/storymaker/render/jobs.go internal/server/storymaker_handlers.go internal/server/storymaker_api_test.go internal/server/storymaker_e2e_test.go internal/server/storymaker_parity_browser_test.go internal/storymaker/render/browser_smoke_test.go internal/storymaker/render/render_test.go internal/storymaker/render/report_test.go
git commit -m "feat(storymaker): refuse a render without stored narration or music" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A25a: Music route and `music_configured`
**Batch:** 2C  **Depends on:** A15, N15
**Files:**
- Modify: `internal/server/storymaker_audio_handlers.go`
- Modify: `internal/server/storymaker_handlers.go`
- Modify: `internal/server/storymaker_narrate.go`
- Modify: `internal/storymaker/render/jobs.go`
- Test: `internal/server/storymaker_audio_test.go`

The job is a stub that fails with `music_failed` until A25b.

- [ ] **Step 1: Impact**

Run from the main checkout `C:/Users/Andi/Documents/repo/AuraGo`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "handleStorymaker" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "handleStorymakerStatus" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "storymakerAudioRun" --direction upstream --repo .
```

Report callers, processes and risk. Warn on HIGH or CRITICAL, and confirm `risk: UNKNOWN` with a text search (outline section 5, GitNexus).

- [ ] **Step 2: Write the failing test**

In `internal/server/storymaker_audio_test.go`, add the imports. N15 left the block these edits apply to.

Old text:

```go
	"context"
	"errors"
```

New text:

```go
	"context"
	"encoding/binary"
	"errors"
```

Old text:

```go
	"aurago/internal/storymaker/render"
	"aurago/internal/tools"
```

New text:

```go
	"aurago/internal/storymaker/render"
	"aurago/internal/storymaker/storymakertest"
	"aurago/internal/tools"
```

Append to the end of the file:

```go
// musicRouteWAV builds a silent 8 kHz mono PCM16 WAV. The route and the job read its header only.
func musicRouteWAV(seconds float64) []byte {
	const rate = 8000
	frames := int(seconds * rate)
	data := make([]byte, 44+frames*2)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+frames*2))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], rate)
	binary.LittleEndian.PutUint32(data[28:], rate*2)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(frames*2))
	return data
}

func writeMusicSource(t *testing.T, s *Server, rel string, data []byte) {
	t.Helper()
	target := filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func musicRouteStory(t *testing.T, s *Server, token string, p *apiProject, music string) {
	t.Helper()
	doc := `{"schema":"storymaker/1","title":"Music","music":` + music + `,"scenes":[{"id":"s1"}]}`
	w := storymakerRequest(s, token, http.MethodPut, "/projects/"+p.Project.ID+"/story", strings.NewReader(doc), map[string]string{"If-Match": p.ETag})
	var saved struct {
		ETag string `json:"etag"`
	}
	decodeBody(t, w, &saved)
	if w.Code != http.StatusOK {
		t.Fatalf("save story: %d %s", w.Code, w.Body.String())
	}
	p.ETag = saved.ETag
}

func postMusic(s *Server, token, id, body string) *httptest.ResponseRecorder {
	return storymakerRequest(s, token, http.MethodPost, "/projects/"+id+"/music", strings.NewReader(body), nil)
}

func TestStorymakerMusicRouteRefusesBadSources(t *testing.T) {
	s, _, token := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, token)
	id := p.Project.ID

	// The bytes decide, never the name: OggS is neither WAV nor MP3.
	writeMusicSource(t, s, "Documents/theme.ogg", append([]byte("OggS"), make([]byte, 64)...))
	// One byte over the 1 GiB limit. Truncate writes nothing, so this takes no time.
	writeMusicSource(t, s, "Documents/big.wav", musicRouteWAV(0.1))
	big, err := os.OpenFile(filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, "Documents", "big.wav"), os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := big.Truncate(1<<30 + 1); err != nil {
		t.Fatal(err)
	}
	if err := big.Close(); err != nil {
		t.Fatal(err)
	}
	// 0.5 s is under the 1 s minimum.
	writeMusicSource(t, s, "Documents/short.wav", musicRouteWAV(0.5))

	type answer struct {
		Code       string `json:"code"`
		MusicIssue string `json:"music_issue"`
	}
	for _, file := range []string{"theme.ogg", "big.wav", "short.wav"} {
		musicRouteStory(t, s, token, &p, `{"mode":"file","file":"Documents/`+file+`"}`)
		w := postMusic(s, token, id, "")
		var got answer
		decodeBody(t, w, &got)
		if w.Code != http.StatusUnprocessableEntity || got.Code != "invalid_music_file" || got.MusicIssue != "" {
			t.Fatalf("%s: %d %s", file, w.Code, w.Body.String())
		}
	}

	musicRouteStory(t, s, token, &p, `{"mode":"file","file":"Documents/gone.wav"}`)
	w := postMusic(s, token, id, "")
	var got answer
	decodeBody(t, w, &got)
	if w.Code != http.StatusUnprocessableEntity || got.Code != "invalid_music_file" || got.MusicIssue != "source_missing" {
		t.Fatalf("missing source: %d %s", w.Code, w.Body.String())
	}
}

func TestStorymakerMusicRouteGeneratedMode(t *testing.T) {
	s, _, token := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, token)
	id := p.Project.ID
	musicRouteStory(t, s, token, &p, `{"mode":"generated","mood":"calm-ambient"}`)
	code := func(w *httptest.ResponseRecorder) string {
		t.Helper()
		var got struct {
			Code string `json:"code"`
		}
		decodeBody(t, w, &got)
		return got.Code
	}

	// Empty means 0. The range is 1000 to max_duration_seconds*1000.
	tooLong := fmt.Sprintf(`{"duration_ms":%d}`, s.Cfg.Storymaker.Render.MaxDurationSeconds*1000+1)
	for _, body := range []string{"", `{"duration_ms":999}`, tooLong} {
		w := postMusic(s, token, id, body)
		if w.Code != http.StatusBadRequest || code(w) != "invalid_argument" {
			t.Fatalf("body %q: %d %s", body, w.Code, w.Body.String())
		}
	}
	if w := postMusic(s, token, id, `{"seconds":5}`); w.Code != http.StatusBadRequest || code(w) != "invalid_request" {
		t.Fatalf("unknown field: %d %s", w.Code, w.Body.String())
	}

	// Only the Generate click asks for a generation. Any other call queues nothing, and the panel offers the button.
	w := postMusic(s, token, id, `{"duration_ms":15000}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"job":null`) {
		t.Fatalf("no click: %d %s", w.Code, w.Body.String())
	}

	// The fixture configures no music provider.
	w = postMusic(s, token, id, `{"duration_ms":15000,"generate":true}`)
	if w.Code != http.StatusConflict || code(w) != "music_unavailable" {
		t.Fatalf("not configured: %d %s", w.Code, w.Body.String())
	}

	// A stored file with its meta needs no job, whatever the provider state is.
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	key := storymaker.MusicKeyGenerated("calm-ambient", "")
	if err := storymakertest.SeedMusic(context.Background(), rt.projects, id, key, 2); err != nil {
		t.Fatal(err)
	}
	w = postMusic(s, token, id, `{"duration_ms":15000,"generate":true}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"job":null`) {
		t.Fatalf("stored key: %d %s", w.Code, w.Body.String())
	}
}

func TestStorymakerMusicRouteQueuesAFileSource(t *testing.T) {
	s, _, token := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, token)
	writeMusicSource(t, s, "Documents/theme.wav", musicRouteWAV(2))
	musicRouteStory(t, s, token, &p, `{"mode":"file","file":"Documents/theme.wav"}`)
	w := postMusic(s, token, p.Project.ID, "")
	var got struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &got)
	if w.Code != http.StatusAccepted || got.Job.Kind != "music" || got.Job.ProjectID != p.Project.ID {
		t.Fatalf("queue: %d %s", w.Code, w.Body.String())
	}
}

func TestStorymakerStatusReportsMusicConfigured(t *testing.T) {
	s, readToken, _ := storymakerAPIFixture(t)
	read := func() bool {
		t.Helper()
		w := storymakerRequest(s, readToken, http.MethodGet, "/status", nil, nil)
		var got struct {
			MusicConfigured bool `json:"music_configured"`
		}
		decodeBody(t, w, &got)
		if w.Code != http.StatusOK {
			t.Fatalf("status: %d %s", w.Code, w.Body.String())
		}
		return got.MusicConfigured
	}
	if read() {
		t.Fatal("music_configured is true with no provider")
	}
	s.Cfg.MusicGeneration.Enabled = true
	s.Cfg.MusicGeneration.APIKey = "test-key"
	if !read() {
		t.Fatal("music_configured is false with a provider")
	}
}
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/server -run '^TestStorymaker(MusicRoute|StatusReportsMusic)' -count=1
```

Expected: FAIL. The music route answers 404 `not_found`, and `music_configured` is false in both reads.

- [ ] **Step 4: Implement**

**`internal/storymaker/render/jobs.go`.** A24 left the block below. This task adds `CodeMusicFailed` and `PhaseGenerating`. `KindMusic` is N9's, and N10 lists it in the narrate manager.

Old text:

```go
// Codes of the audio gates on POST /render.
const (
	CodeNarrationIncomplete = "narration_incomplete"
	CodeMusicIncomplete     = "music_incomplete"
)
```

New text:

```go
// Codes of the audio gates on POST /render and of the music job, and the phase of that job.
const (
	CodeNarrationIncomplete = "narration_incomplete"
	CodeMusicIncomplete     = "music_incomplete"
	CodeMusicFailed         = "music_failed"

	PhaseGenerating = "generating"
)
```

**`internal/server/storymaker_handlers.go`, the dispatch.**

Old text:

```go
			case len(parts) == 3 && parts[2] == "render":
```

New text:

```go
			case len(parts) == 3 && parts[2] == "music":
				handleStorymakerMusic(s, w, r, id)
			case len(parts) == 3 && parts[2] == "render":
```

**`internal/server/storymaker_handlers.go`, the status.**

Old text:

```go
	status["browser"] = browser
```

New text:

```go
	status["browser"] = browser
	status["music_configured"] = cfg.MusicConfigured()
```

**`internal/server/storymaker_narrate.go`, the dispatch.** It sits before `beginDesktopRun`, so a music job never gets a narrate failure code. The signature is the frozen one.

Old text:

```go
func (s *Server) storymakerAudioRun(ctx context.Context, rt *storymakerRuntime, job render.Job, payload any, h *render.Handle) (render.Output, error) {
```

New text:

```go
func (s *Server) storymakerAudioRun(ctx context.Context, rt *storymakerRuntime, job render.Job, payload any, h *render.Handle) (render.Output, error) {
	if music, ok := payload.(storymakerMusicPayload); ok {
		return s.storymakerMusicRun(ctx, rt, job, music, h)
	}
```

**`internal/server/storymaker_narrate.go`, the imports.** N12 left the block below.

Old text:

```go
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
```

New text:

```go
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
```

**`internal/server/storymaker_narrate.go`, the new code.** Append to the end of the file. `storymakerMusicPayload` lands here, because N10 does not declare it.

```go
const (
	maxMusicSourceBytes = int64(1) << 30 // 1 GiB
	minMusicDurationMS  = 1000
	maxMusicDurationMS  = 3600000
)

// storymakerMusicPayload is the payload of a music job. DurationMS is the film length, for a generated track. GenerateKey
// is the music key a Generate click named. Only POST /music with generate:true sets it, and the job calls a provider only
// for that key, because the story can change while the job waits.
type storymakerMusicPayload struct {
	ProjectID   string
	DurationMS  int
	GenerateKey string
}

// storymakerMusicRefusal is a music source that POST /music and the job refuse. Its text is safe to show.
type storymakerMusicRefusal struct{ message string }

func (e *storymakerMusicRefusal) Error() string { return e.message }

func storymakerMusicDurationOK(ms int) bool {
	return ms >= minMusicDurationMS && ms <= maxMusicDurationMS
}

// storymakerMusicAudioRel returns file relative to DataDir/audio in slash form, and refuses a file outside it.
func storymakerMusicAudioRel(cfg *config.Config, file string) (string, error) {
	root, err := filepath.Abs(filepath.Join(cfg.Directories.DataDir, "audio"))
	if err != nil {
		return "", fmt.Errorf("resolve the audio folder: %w", err)
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return "", fmt.Errorf("resolve the music file: %w", err)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || !filepath.IsLocal(rel) {
		return "", &storymakerMusicRefusal{message: "The music file is not in the audio folder."}
	}
	return filepath.ToSlash(rel), nil
}

// storymakerMusicTrackPath turns a media id into its desktop path below the Music mount.
func (s *Server) storymakerMusicTrackPath(cfg *config.Config, track string) (string, error) {
	id, err := strconv.ParseInt(track, 10, 64)
	if err != nil || id <= 0 {
		return "", &storymakerMusicRefusal{message: "The music track id is not valid."}
	}
	item, err := tools.GetMedia(s.MediaRegistryDB, id)
	if err != nil {
		return "", fmt.Errorf("%w: track %d", storymaker.ErrMusicSourceMissing, id)
	}
	if item.MediaType != "music" {
		return "", &storymakerMusicRefusal{message: "This media item is not a music track."}
	}
	rel, err := storymakerMusicAudioRel(cfg, item.FilePath)
	if err != nil {
		return "", err
	}
	return "Music/" + rel, nil
}

// storymakerMusicSourceOpen opens the story's file or track. Every source goes through Store.Open, so the desktop's
// mount and symlink checks hold. A missing source wraps storymaker.ErrMusicSourceMissing.
func (s *Server) storymakerMusicSourceOpen(ctx context.Context, rt *storymakerRuntime, cfg *config.Config, m *storymaker.Music) (io.ReadSeekCloser, storymaker.FileInfo, error) {
	source := m.File
	if m.Track != "" {
		trackPath, err := s.storymakerMusicTrackPath(cfg, m.Track)
		if err != nil {
			return nil, storymaker.FileInfo{}, err
		}
		source = trackPath
	}
	r, info, err := rt.store.Open(ctx, source)
	switch {
	case err == nil:
		return r, info, nil
	case errors.Is(err, fs.ErrNotExist):
		return nil, storymaker.FileInfo{}, fmt.Errorf("%w: %w", storymaker.ErrMusicSourceMissing, err)
	case ctx.Err() != nil:
		return nil, storymaker.FileInfo{}, ctx.Err()
	}
	return nil, storymaker.FileInfo{}, &storymakerMusicRefusal{message: "The music source could not be opened."}
}

// storymakerCheckMusicSource makes the checks that read no more than the head of a source: size, format by its bytes
// and free disk. It leaves r at the start and returns the format.
func storymakerCheckMusicSource(r io.ReadSeeker, size int64, workspaceDir string) (string, error) {
	if size > maxMusicSourceBytes {
		return "", &storymakerMusicRefusal{message: "The music file is larger than 1 GiB."}
	}
	head := make([]byte, 12)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", fmt.Errorf("read the music source: %w", err)
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind the music source: %w", err)
	}
	ext := storymaker.SniffAudio(head[:n])
	if ext != "wav" && ext != "mp3" {
		return "", &storymakerMusicRefusal{message: "The music file is not WAV or MP3."}
	}
	free, err := fileutil.FreeDiskBytes(workspaceDir)
	if err != nil {
		return "", fmt.Errorf("read the free disk space: %w", err)
	}
	if free < size+render.MinFreeDiskBytes {
		return "", &storymakerMusicRefusal{message: fmt.Sprintf("The disk has %d MiB free, and this music file needs %d MiB.", free>>20, (size+render.MinFreeDiskBytes)>>20)}
	}
	return ext, nil
}

// storymakerMusicSourceProblem runs the checks POST /music makes before any job exists.
func (s *Server) storymakerMusicSourceProblem(ctx context.Context, rt *storymakerRuntime, cfg *config.Config, m *storymaker.Music) error {
	src, info, err := s.storymakerMusicSourceOpen(ctx, rt, cfg, m)
	if err != nil {
		return err
	}
	defer src.Close()
	if _, err := storymakerCheckMusicSource(src, info.Size, cfg.VirtualDesktop.WorkspaceDir); err != nil {
		return err
	}
	_, durationMS, err := storymaker.ProbeAudio(src)
	if err != nil {
		return &storymakerMusicRefusal{message: "The music file could not be read as audio."}
	}
	if !storymakerMusicDurationOK(durationMS) {
		return &storymakerMusicRefusal{message: "The music file must last between 1 second and 1 hour."}
	}
	return nil
}

// storymakerMusicRun is the music job. A25b replaces this stub with the copy of a file or track source.
func (s *Server) storymakerMusicRun(ctx context.Context, rt *storymakerRuntime, job render.Job, p storymakerMusicPayload, h *render.Handle) (render.Output, error) {
	return render.Output{}, render.Fail(render.CodeMusicFailed, "Music jobs cannot run yet.")
}
```

**`internal/server/storymaker_audio_handlers.go`, the imports.** N15 and N16 leave the block below, and A15 does not change it. N15 does not import `encoding/json` here, and the new handler needs it and `io`.

Old text:

```go
	"errors"
	"fmt"
	"net/http"
	"regexp"
```

New text:

```go
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
```

**`internal/server/storymaker_audio_handlers.go`, the new code.** Append to the end of the file:

```go
// handleStorymakerMusic answers POST /projects/{id}/music: it checks the source before any job exists and queues a job
// only when the key's file or meta is missing. A generation starts only from a call with generate:true, which only the
// Generate button sends.
func handleStorymakerMusic(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeStorymakerMethodError(w, "POST")
		return
	}
	acc, ok := storymakerAuthorize(s, w, r, desktopScopeWrite, desktopWrite, true)
	if !ok {
		return
	}
	var req struct {
		DurationMS int  `json:"duration_ms"`
		Generate   bool `json:"generate"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, storymakerJSONLimit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeStorymakerError(w, http.StatusBadRequest, "invalid_request", `The request body must be empty or {"duration_ms": ..., "generate": true}.`, nil)
		return
	}
	ctx := r.Context()
	data, _, err := acc.rt.projects.LoadStory(ctx, id)
	if err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return
	}
	story, _, err := storymaker.DecodeStory(data, storymakerLimits(acc.cfg))
	if err != nil {
		mapStorymakerError(w, err, "project_not_found")
		return
	}
	storymaker.ApplyDefaults(&story)
	key, err := s.storymakerMusicKey(ctx, story.Music)
	if err != nil {
		writeStorymakerMusicSourceError(w, err)
		return
	}
	if key == "" {
		writeStorymakerJSON(w, http.StatusOK, map[string]any{"job": nil})
		return
	}
	if story.Music.Mode == "generated" {
		limit := acc.cfg.Storymaker.Render.MaxDurationSeconds * 1000
		if req.DurationMS < minMusicDurationMS || req.DurationMS > limit {
			writeStorymakerError(w, http.StatusBadRequest, "invalid_argument", fmt.Sprintf("duration_ms must be between %d and %d.", minMusicDurationMS, limit), nil)
			return
		}
	}
	meta, audioExt, err := acc.rt.projects.MusicInfo(ctx, id, key)
	if err != nil {
		writeStorymakerError(w, http.StatusInternalServerError, "storage_failed", "The project storage failed.", nil)
		return
	}
	if meta.V == 1 && audioExt != "" {
		writeStorymakerJSON(w, http.StatusOK, map[string]any{"job": nil})
		return
	}
	// A stored file without valid meta needs neither its source nor a provider: the job only probes it.
	if audioExt == "" {
		if story.Music.Mode == "generated" {
			// A generation costs money, so a call without the click queues nothing, and the panel offers the button. This also
			// stops a file job's automatic call that lands right after a switch to generated.
			if !req.Generate {
				writeStorymakerJSON(w, http.StatusOK, map[string]any{"job": nil})
				return
			}
			if !acc.cfg.MusicConfigured() {
				writeStorymakerError(w, http.StatusConflict, "music_unavailable", "No music provider is configured.", nil)
				return
			}
			if !acc.cfg.UsesLocalMusic() && s.BudgetTracker.IsBlocked("music_generation") {
				writeStorymakerError(w, http.StatusConflict, render.CodeBudgetExceeded, "The daily budget is used up, so no music can be generated.", nil)
				return
			}
		} else if err := s.storymakerMusicSourceProblem(ctx, acc.rt, acc.cfg, story.Music); err != nil {
			writeStorymakerMusicSourceError(w, err)
			return
		}
	}
	payload := storymakerMusicPayload{ProjectID: id, DurationMS: req.DurationMS}
	if story.Music.Mode == "generated" && req.Generate {
		payload.GenerateKey = key
	}
	job, replayed, err := acc.rt.narrate.Enqueue(render.EnqueueRequest{
		ProjectID: id, Kind: render.KindMusic, Coalesce: true, Payload: payload,
	})
	switch {
	case errors.Is(err, render.ErrBusy):
		writeStorymakerError(w, http.StatusTooManyRequests, render.CodeBusy, "Too many audio jobs are waiting. Try again when one has finished.", nil)
	case err != nil:
		writeStorymakerError(w, http.StatusInternalServerError, "job_failed", "The music job could not be queued.", nil)
	case replayed:
		writeStorymakerJSON(w, http.StatusOK, map[string]any{"job": job})
	default:
		writeStorymakerJSON(w, http.StatusAccepted, map[string]any{"job": job})
	}
}

func writeStorymakerMusicSourceError(w http.ResponseWriter, err error) {
	var refusal *storymakerMusicRefusal
	switch {
	case errors.Is(err, storymaker.ErrMusicSourceMissing):
		writeStorymakerErrorWith(w, http.StatusUnprocessableEntity, "invalid_music_file", "The music source no longer exists.", map[string]any{"music_issue": "source_missing"})
	case errors.As(err, &refusal):
		writeStorymakerError(w, http.StatusUnprocessableEntity, "invalid_music_file", refusal.message, nil)
	default:
		writeStorymakerError(w, http.StatusInternalServerError, "storage_failed", "The project storage failed.", nil)
	}
}
```

N10 created the narrate manager with `Kinds: []string{render.KindNarrate, render.KindMusic}`, so `internal/server/storymaker_service.go` needs no edit.

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/server -run '^TestStorymaker(MusicRoute|StatusReportsMusic)' -count=1
go vet ./internal/server ./internal/storymaker/render
gofmt -l internal/server internal/storymaker/render
```

Expected: PASS, no vet finding, and `gofmt -l` prints nothing.

- [ ] **Step 6: Commit**

Run `detect_changes` first. A PARTIAL answer on new files needs a grep confirmation in the commit body (outline section 5).

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/server/storymaker_audio_handlers.go internal/server/storymaker_handlers.go internal/server/storymaker_narrate.go internal/storymaker/render/jobs.go internal/server/storymaker_audio_test.go
git commit -m "feat(storymaker): add the music route and music_configured" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A25b: Music job: file and track sources
**Batch:** 2C  **Depends on:** A25a
**Files:**
- Modify: `internal/server/storymaker_narrate.go`
- Test: `internal/server/storymaker_audio_test.go`

- [ ] **Step 1: Impact**

Run from the main checkout `C:/Users/Andi/Documents/repo/AuraGo`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "storymakerMusicRun" --direction upstream --repo .
```

Report callers, processes and risk. Warn on HIGH or CRITICAL, and confirm `risk: UNKNOWN` with a text search (outline section 5, GitNexus). The stale index does not know this A25a symbol, so confirm with a grep that only `storymakerAudioRun` calls it.

- [ ] **Step 2: Write the failing test**

In `internal/server/storymaker_audio_test.go`, add the import.

Old text:

```go
	"fmt"
	"net"
```

New text:

```go
	"fmt"
	"io"
	"net"
```

Append to the end of the file:

```go
func waitStorymakerMusicJob(t *testing.T, rt *storymakerRuntime, id string) render.Job {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if job, ok := rt.narrate.Get(id); ok && job.Terminal() {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the music job %s did not end", id)
	return render.Job{}
}

// queueMusicJob skips POST /music, because these tests reach job checks that the route stops first.
func queueMusicJob(t *testing.T, rt *storymakerRuntime, id string) render.Job {
	t.Helper()
	job, _, err := rt.narrate.Enqueue(render.EnqueueRequest{ProjectID: id, Kind: render.KindMusic, Coalesce: true, Payload: storymakerMusicPayload{ProjectID: id, DurationMS: 15000}})
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func musicFileFixture(t *testing.T, seconds float64) (*Server, string, string, *storymakerRuntime, string) {
	t.Helper()
	s, _, token := storymakerAPIFixture(t)
	p := createStorymakerProject(t, s, token)
	writeMusicSource(t, s, "Documents/theme.wav", musicRouteWAV(seconds))
	musicRouteStory(t, s, token, &p, `{"mode":"file","file":"Documents/theme.wav"}`)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	key, err := s.storymakerMusicKey(context.Background(), &storymaker.Music{Mode: "file", File: "Documents/theme.wav"})
	if err != nil || key == "" {
		t.Fatalf("music key = %q, %v", key, err)
	}
	return s, token, p.Project.ID, rt, key
}

func TestStorymakerMusicTrackSourceRules(t *testing.T) {
	s, _, token := storymakerAPIFixture(t)
	createStorymakerProject(t, s, token) // binds the store to the desktop service
	db, err := tools.InitMediaRegistryDB(filepath.Join(t.TempDir(), "media.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s.MediaRegistryDB = db
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := normalizedStorymakerConfig(s)
	if err != nil {
		t.Fatal(err)
	}
	audioDir := filepath.Join(s.Cfg.Directories.DataDir, "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(audioDir, "song.wav")
	outside := filepath.Join(s.Cfg.Directories.DataDir, "secret.wav")
	for _, file := range []string{inside, outside} {
		if err := os.WriteFile(file, musicRouteWAV(2), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	register := func(mediaType, file string) string {
		t.Helper()
		id, _, err := tools.RegisterMedia(db, tools.MediaItem{MediaType: mediaType, SourceTool: "test", Filename: filepath.Base(file), FilePath: file})
		if err != nil {
			t.Fatal(err)
		}
		return strconv.FormatInt(id, 10)
	}
	open := func(track string) error {
		src, _, err := s.storymakerMusicSourceOpen(context.Background(), rt, cfg, &storymaker.Music{Mode: "file", Track: track})
		if err == nil {
			_ = src.Close()
		}
		return err
	}
	if err := open(register("music", inside)); err != nil {
		t.Fatalf("a music track in the audio folder: %v", err)
	}
	var refusal *storymakerMusicRefusal
	if err := open(register("audio", inside)); !errors.As(err, &refusal) {
		t.Fatalf("another media type: %v", err)
	}
	if err := open(register("music", outside)); !errors.As(err, &refusal) {
		t.Fatalf("a path outside the audio folder: %v", err)
	}
	if err := open("999999"); !errors.Is(err, storymaker.ErrMusicSourceMissing) {
		t.Fatalf("an unknown track: %v", err)
	}
	// A plain os.Open would follow this link. Store.Open refuses it, because it leaves the Music mount.
	link := filepath.Join(audioDir, "link.wav")
	if err := os.Symlink(outside, link); err != nil {
		t.Logf("this box cannot create a symlink, so that case is not run: %v", err)
		return
	}
	if err := open(register("music", link)); err == nil {
		t.Fatal("a symlink out of the audio folder was opened, so the source did not go through Store.Open")
	}
}

func TestStorymakerMusicJobCopiesAFileAndWritesMeta(t *testing.T) {
	s, token, id, rt, key := musicFileFixture(t, 2)
	ctx := context.Background()
	w := postMusic(s, token, id, "")
	var got struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &got)
	if w.Code != http.StatusAccepted {
		t.Fatalf("queue: %d %s", w.Code, w.Body.String())
	}
	done := waitStorymakerMusicJob(t, rt, got.Job.ID)
	if done.Status != render.StatusSucceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	// ProbeAudio over the stored copy: 2 s of WAV.
	meta, ext, err := rt.projects.MusicInfo(ctx, id, key)
	if err != nil || ext != "wav" || meta != (storymaker.MusicMeta{V: 1, Ext: "wav", DurationMS: 2000}) {
		t.Fatalf("music info = %+v %q %v", meta, ext, err)
	}
	stored, _, err := rt.projects.OpenAudio(ctx, id, "music", key+".wav")
	if err != nil {
		t.Fatal(err)
	}
	defer stored.Close()
	data, err := io.ReadAll(stored)
	if err != nil || !bytes.Equal(data, musicRouteWAV(2)) {
		t.Fatalf("the stored copy differs from the source: %d bytes, %v", len(data), err)
	}
	if w := postMusic(s, token, id, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"job":null`) {
		t.Fatalf("a second call: %d %s", w.Code, w.Body.String())
	}
}

func TestStorymakerMusicStoreCountsAPathConflictAsSuccess(t *testing.T) {
	_, _, id, rt, _ := musicFileFixture(t, 2)
	ctx := context.Background()
	key := strings.Repeat("a", 64)
	first := musicRouteWAV(2)
	if err := storymakerStoreMusic(ctx, rt, id, key, "wav", bytes.NewReader(first)); err != nil {
		t.Fatal(err)
	}
	if err := storymakerStoreMusic(ctx, rt, id, key, "wav", bytes.NewReader(musicRouteWAV(3))); err != nil {
		t.Fatalf("a second copy under the same key: %v", err)
	}
	stored, _, err := rt.projects.OpenAudio(ctx, id, "music", key+".wav")
	if err != nil {
		t.Fatal(err)
	}
	defer stored.Close()
	data, err := io.ReadAll(stored)
	if err != nil || !bytes.Equal(data, first) {
		t.Fatalf("the first copy was replaced: %d bytes, %v", len(data), err)
	}
}

func TestStorymakerMusicJobWritesMetaForAStoredCopy(t *testing.T) {
	s, token, id, rt, key := musicFileFixture(t, 2)
	ctx := context.Background()
	// The stored copy lasts 3 s and the source 2 s, so 3000 proves the job probed the copy and left it in place.
	if err := rt.projects.WriteMusic(ctx, id, key, "wav", musicRouteWAV(3)); err != nil {
		t.Fatal(err)
	}
	w := postMusic(s, token, id, "")
	var got struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &got)
	if w.Code != http.StatusAccepted {
		t.Fatalf("queue: %d %s", w.Code, w.Body.String())
	}
	done := waitStorymakerMusicJob(t, rt, got.Job.ID)
	if done.Status != render.StatusSucceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	meta, _, err := rt.projects.MusicInfo(ctx, id, key)
	if err != nil || meta.V != 1 || meta.DurationMS != 3000 {
		t.Fatalf("meta = %+v, %v", meta, err)
	}
}

func TestStorymakerMusicJobRemovesACopyOfTheWrongLength(t *testing.T) {
	_, _, id, rt, key := musicFileFixture(t, 0.5)
	done := waitStorymakerMusicJob(t, rt, queueMusicJob(t, rt, id).ID)
	if done.Status != render.StatusFailed || done.Error == nil || done.Error.Code != render.CodeMusicFailed {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	meta, ext, err := rt.projects.MusicInfo(context.Background(), id, key)
	if err != nil || meta.V != 0 || ext != "" {
		t.Fatalf("the copy of 500 ms stayed: %+v %q %v", meta, ext, err)
	}
	for ms, want := range map[int]bool{999: false, 1000: true, 3600000: true, 3600001: false} {
		if storymakerMusicDurationOK(ms) != want {
			t.Errorf("storymakerMusicDurationOK(%d) != %v", ms, want)
		}
	}
}

func TestStorymakerMusicJobStopsWhenReadOnly(t *testing.T) {
	s, _, id, rt, key := musicFileFixture(t, 2)
	ro := *s.ConfigSnapshot()
	ro.VirtualDesktop.ReadOnly = true
	s.replaceConfigSnapshot(&ro)
	done := waitStorymakerMusicJob(t, rt, queueMusicJob(t, rt, id).ID)
	if done.Status != render.StatusFailed || done.Error == nil || done.Error.Code != render.CodeMusicFailed || !strings.Contains(done.Error.Message, "read-only") {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if _, ext, err := rt.projects.MusicInfo(context.Background(), id, key); err != nil || ext != "" {
		t.Fatalf("a read-only stop stored %q, %v", ext, err)
	}
}
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/server -run '^TestStorymakerMusic(TrackSource|Job|Store)' -count=1
```

Expected: FAIL to compile with `undefined: storymakerStoreMusic`.

- [ ] **Step 4: Implement**

In `internal/server/storymaker_narrate.go`, add the import.

Old text:

```go
	"aurago/internal/config"
	"aurago/internal/fileutil"
```

New text:

```go
	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/fileutil"
```

In `internal/server/storymaker_narrate.go`, replace the stub that A25a left.

Old text:

```go
// storymakerMusicRun is the music job. A25b replaces this stub with the copy of a file or track source.
func (s *Server) storymakerMusicRun(ctx context.Context, rt *storymakerRuntime, job render.Job, p storymakerMusicPayload, h *render.Handle) (render.Output, error) {
	return render.Output{}, render.Fail(render.CodeMusicFailed, "Music jobs cannot run yet.")
}
```

New text:

```go
// storymakerMusicRun is the music job: it stores the story's music under its key and writes the meta. A ctx error becomes
// music_failed here, and runJob still reports a cancel request or a shutdown as such.
func (s *Server) storymakerMusicRun(ctx context.Context, rt *storymakerRuntime, job render.Job, p storymakerMusicPayload, h *render.Handle) (render.Output, error) {
	ctx, release, err := s.beginDesktopRun(ctx)
	if err != nil {
		return render.Output{}, storymakerMusicStopped()
	}
	defer release()
	err = s.storymakerMusicStore(ctx, rt, p, h)
	var je *render.JobError
	var refusal *storymakerMusicRefusal
	switch {
	case err == nil:
		return render.Output{}, nil
	case ctx.Err() != nil:
		return render.Output{}, storymakerMusicStopped()
	case errors.As(err, &je):
		return render.Output{}, je
	case errors.As(err, &refusal):
		return render.Output{}, render.Fail(render.CodeMusicFailed, "%s", refusal.message)
	case errors.Is(err, storymaker.ErrMusicSourceMissing):
		return render.Output{}, render.Fail(render.CodeMusicFailed, "The music source no longer exists.")
	}
	s.storymakerMusicLogger().Warn("[Storymaker] The music job failed", "project", p.ProjectID, "error", err)
	return render.Output{}, render.Fail(render.CodeMusicFailed, "The music could not be stored.")
}

// storymakerMusicStopped names both causes, because a revoked run can see its cancelled ctx before the new
// configuration is published (server.go revokes first and stores the snapshot later).
func storymakerMusicStopped() *render.JobError {
	return render.Fail(render.CodeMusicFailed, "The desktop became read-only or its configuration changed, so the music was not stored.")
}

func (s *Server) storymakerMusicLogger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.Default()
}

// storymakerMusicStore re-reads the story, so an edit while the job waited is picked up, and brings the key's file and
// meta into place. A stored file without valid meta needs no source: it is only probed. A paid generation runs only under
// the key in the payload, so an edit while the job waited cannot start one.
func (s *Server) storymakerMusicStore(ctx context.Context, rt *storymakerRuntime, p storymakerMusicPayload, h *render.Handle) error {
	cfg, err := normalizedStorymakerConfig(s)
	if err != nil {
		return err
	}
	if !cfg.Storymaker.Enabled || !cfg.VirtualDesktop.Enabled {
		return render.Fail(render.CodeDisabled, "Storymaker was switched off before the music was stored.")
	}
	data, _, err := rt.projects.LoadStory(ctx, p.ProjectID)
	if err != nil {
		return err
	}
	story, _, err := storymaker.DecodeStory(data, storymakerLimits(cfg))
	if err != nil {
		return err
	}
	storymaker.ApplyDefaults(&story)
	key, err := s.storymakerMusicKey(ctx, story.Music)
	if err != nil {
		return err
	}
	if key == "" {
		return nil
	}
	meta, ext, err := rt.projects.MusicInfo(ctx, p.ProjectID, key)
	if err != nil {
		return err
	}
	if meta.V == 1 && ext != "" {
		return nil
	}
	if ext == "" {
		if ext, err = s.storymakerCopyMusicSource(ctx, rt, cfg, p, key, story.Music, h); err != nil {
			return err
		}
	}
	return storymakerFinishMusic(ctx, rt, p.ProjectID, key, ext)
}

// storymakerCopyMusicSource copies the story's source to the key's file and returns its format.
func (s *Server) storymakerCopyMusicSource(ctx context.Context, rt *storymakerRuntime, cfg *config.Config, p storymakerMusicPayload, key string, m *storymaker.Music, h *render.Handle) (string, error) {
	if m.Mode == "generated" {
		return "", &storymakerMusicRefusal{message: "Generated music cannot run yet."}
	}
	h.SetPhase(render.PhaseWriting)
	src, info, err := s.storymakerMusicSourceOpen(ctx, rt, cfg, m)
	if err != nil {
		return "", err
	}
	defer src.Close()
	ext, err := storymakerCheckMusicSource(src, info.Size, cfg.VirtualDesktop.WorkspaceDir)
	if err != nil {
		return "", err
	}
	return ext, storymakerStoreMusic(ctx, rt, p.ProjectID, key, ext, src)
}

// storymakerStoreMusic writes the key's create-only file. A file that already exists counts as success, because the key
// says it has this content.
func storymakerStoreMusic(ctx context.Context, rt *storymakerRuntime, id, key, ext string, r io.Reader) error {
	target, err := rt.projects.AudioPath(id, "music", key+"."+ext)
	if err != nil {
		return err
	}
	err = rt.store.WriteStream(ctx, target, r, maxMusicSourceBytes)
	var conflict *desktop.PathConflict
	if errors.As(err, &conflict) {
		return nil
	}
	return err
}

// storymakerFinishMusic probes the stored copy on every path, because not every provider reports a duration. A copy of
// the wrong length is removed. The meta comes after the file, so a file without meta counts as missing.
func storymakerFinishMusic(ctx context.Context, rt *storymakerRuntime, id, key, ext string) error {
	file := key + "." + ext
	r, _, err := rt.projects.OpenAudio(ctx, id, "music", file)
	if err != nil {
		return err
	}
	_, durationMS, probeErr := storymaker.ProbeAudio(r)
	// Close first: Windows cannot remove an open file.
	_ = r.Close()
	if probeErr != nil || !storymakerMusicDurationOK(durationMS) {
		target, err := rt.projects.AudioPath(id, "music", file)
		if err != nil {
			return err
		}
		if err := rt.store.Remove(ctx, target); err != nil {
			return err
		}
		return &storymakerMusicRefusal{message: "The music must be readable audio that lasts between 1 second and 1 hour."}
	}
	return rt.projects.WriteMusicMeta(ctx, id, key, storymaker.MusicMeta{V: 1, Ext: ext, DurationMS: durationMS})
}
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/server -run '^TestStorymaker(Music|StatusReportsMusic)' -count=1
go vet ./internal/server
gofmt -l internal/server
```

Expected: PASS, no vet finding, and `gofmt -l` prints nothing.

- [ ] **Step 6: Commit**

Run `detect_changes` first. A PARTIAL answer on new files needs a grep confirmation in the commit body (outline section 5).

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/server/storymaker_narrate.go internal/server/storymaker_audio_test.go
git commit -m "feat(storymaker): store music from a file or a media track" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A25c: Music job: generated source
**Batch:** 2C  **Depends on:** A25b
**Files:**
- Modify: `internal/server/storymaker_narrate.go`
- Modify: `internal/server/server.go`
- Test: `internal/server/storymaker_audio_test.go`

- [ ] **Step 1: Impact**

Run from the main checkout `C:/Users/Andi/Documents/repo/AuraGo`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "storymakerCopyMusicSource" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "storymakerFinishMusic" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "Server" --direction upstream --repo .
```

Report callers, processes and risk. Warn on HIGH or CRITICAL, and confirm `risk: UNKNOWN` with a text search (outline section 5, GitNexus). `Server` will report a large blast radius, so state that warning. The edit only adds one unexported test-seam field.

- [ ] **Step 2: Write the failing test**

In `internal/server/storymaker_audio_test.go`, add the imports.

Old text:

```go
	"io"
	"net"
```

New text:

```go
	"io"
	"io/fs"
	"log/slog"
	"net"
```

Old text:

```go
	"os"
	"path/filepath"
```

New text:

```go
	"os"
	"path"
	"path/filepath"
```

Old text:

```go
	"aurago/internal/config"
```

New text:

```go
	"aurago/internal/budget"
	"aurago/internal/config"
```

Old text:

```go
	"aurago/internal/realtimespeech"
```

New text:

```go
	"aurago/internal/realtimespeech"
	"aurago/internal/security"
```

Append to the end of the file:

```go
type generatedMusicFixture struct {
	s      *Server
	token  string
	id     string
	key    string
	rt     *storymakerRuntime
	seam   *storymakerMusicSeam
	calls  atomic.Int32
	params tools.MusicGenParams
}

func (fx *generatedMusicFixture) audioFile(t *testing.T, name string, seconds float64) string {
	t.Helper()
	dir := filepath.Join(fx.s.Cfg.Directories.DataDir, "audio")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, name)
	if err := os.WriteFile(file, musicRouteWAV(seconds), 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

// newGeneratedMusicFixture has a configured provider whose stand-in returns a 20 s WAV with no DurationMs.
func newGeneratedMusicFixture(t *testing.T) *generatedMusicFixture {
	t.Helper()
	s, _, token := storymakerAPIFixture(t)
	s.Cfg.MusicGeneration.Enabled = true
	s.Cfg.MusicGeneration.APIKey = "test-key"
	p := createStorymakerProject(t, s, token)
	musicRouteStory(t, s, token, &p, `{"mode":"generated","mood":"calm-ambient"}`)
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	fx := &generatedMusicFixture{s: s, token: token, id: p.Project.ID, key: storymaker.MusicKeyGenerated("calm-ambient", ""), rt: rt}
	file := fx.audioFile(t, "generated.wav", 20)
	fx.seam = &storymakerMusicSeam{generate: func(ctx context.Context, cfg *config.Config, params tools.MusicGenParams) tools.MusicGenResult {
		fx.calls.Add(1)
		fx.params = params
		return tools.MusicGenResult{Status: "ok", FilePath: file, CostEstimate: 0.06}
	}}
	s.storymakerMusicOverride = fx.seam
	return fx
}

func (fx *generatedMusicFixture) post(t *testing.T, body string) render.Job {
	t.Helper()
	w := postMusic(fx.s, fx.token, fx.id, body)
	var got struct {
		Job render.Job `json:"job"`
	}
	decodeBody(t, w, &got)
	if w.Code != http.StatusAccepted {
		t.Fatalf("queue: %d %s", w.Code, w.Body.String())
	}
	return got.Job
}

// queue skips POST /music, as queueMusicJob does, and sets the key a Generate click would pin.
func (fx *generatedMusicFixture) queue(t *testing.T, generateKey string) render.Job {
	t.Helper()
	job, _, err := fx.rt.narrate.Enqueue(render.EnqueueRequest{ProjectID: fx.id, Kind: render.KindMusic, Coalesce: true, Payload: storymakerMusicPayload{ProjectID: fx.id, DurationMS: 15000, GenerateKey: generateKey}})
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func musicTestTracker(t *testing.T, limit float64, enforcement string) *budget.Tracker {
	t.Helper()
	cfg := &config.Config{}
	cfg.Budget.Enabled = true
	cfg.Budget.DailyLimitUSD = limit
	cfg.Budget.Enforcement = enforcement
	cfg.Budget.WarningThreshold = 0.8
	tracker := budget.NewTracker(cfg, slog.Default(), t.TempDir())
	t.Cleanup(tracker.Flush)
	return tracker
}

func TestStorymakerMusicSeconds(t *testing.T) {
	for _, row := range []struct {
		durationMS, profileMax int
		want                   float64
	}{
		{15000, 0, 17},    // ceil(15) + 2
		{1000, 0, 10},     // 1 + 2 = 3, raised to the 10 s minimum
		{300000, 240, 240}, // 300 + 2 = 302, cut to the profile limit
		{900000, 0, 600},  // 900 + 2 = 902, cut to 600 without a profile
		{900000, 900, 600}, // a profile above 600 does not raise the limit
	} {
		if got := storymakerMusicSeconds(row.durationMS, row.profileMax); got != row.want {
			t.Errorf("storymakerMusicSeconds(%d, %d) = %v, want %v", row.durationMS, row.profileMax, got, row.want)
		}
	}
}

func TestStorymakerMusicGeneratedAsksForTheProfileLimitAndProbesTheResult(t *testing.T) {
	fx := newGeneratedMusicFixture(t)
	fx.s.Cfg.MusicGeneration.Provider = config.LocalMusicProviderID
	fx.seam.profileMax = 240
	// A 300 s film asks for 300 + 2 = 302 s, which is above the 240 s profile limit.
	done := waitStorymakerMusicJob(t, fx.rt, fx.post(t, `{"duration_ms":300000,"generate":true}`).ID)
	if done.Status != render.StatusSucceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if fx.params.DurationSeconds != 240 || !fx.params.Instrumental || fx.params.Prompt != storymaker.MusicPrompt("calm-ambient", "") {
		t.Fatalf("params = %+v", fx.params)
	}
	// The result has no DurationMs, so the meta comes from ProbeAudio over the 20 s file.
	meta, ext, err := fx.rt.projects.MusicInfo(context.Background(), fx.id, fx.key)
	if err != nil || ext != "wav" || meta != (storymaker.MusicMeta{V: 1, Ext: "wav", DurationMS: 20000}) {
		t.Fatalf("music info = %+v %q %v", meta, ext, err)
	}
}

func TestStorymakerMusicGeneratedRefusedByTheBudget(t *testing.T) {
	fx := newGeneratedMusicFixture(t)
	tracker := musicTestTracker(t, 1, "full")
	tracker.RecordCostForCategory("chat", 2) // 2 USD over a limit of 1 USD: exceeded, and "full" blocks every category
	fx.s.BudgetTracker = tracker
	w := postMusic(fx.s, fx.token, fx.id, `{"duration_ms":15000,"generate":true}`)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), `"code":"budget_exceeded"`) {
		t.Fatalf("route: %d %s", w.Code, w.Body.String())
	}
	done := waitStorymakerMusicJob(t, fx.rt, fx.queue(t, fx.key).ID)
	if done.Status != render.StatusFailed || done.Error == nil || done.Error.Code != render.CodeBudgetExceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if fx.calls.Load() != 0 {
		t.Fatalf("the provider was called %d times", fx.calls.Load())
	}
}

func TestStorymakerMusicGeneratedNeverPaysWithoutAClickOnThisKey(t *testing.T) {
	fx := newGeneratedMusicFixture(t)
	// No click: the route queues nothing, also with a provider configured.
	w := postMusic(fx.s, fx.token, fx.id, `{"duration_ms":15000}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"job":null`) {
		t.Fatalf("no click: %d %s", w.Code, w.Body.String())
	}
	// A file job that finds a generated story when it runs has no key. A click whose mood or prompt changed while it waited
	// has an older one.
	for _, generateKey := range []string{"", strings.Repeat("a", 64)} {
		done := waitStorymakerMusicJob(t, fx.rt, fx.queue(t, generateKey).ID)
		if done.Status != render.StatusFailed || done.Error == nil || done.Error.Code != render.CodeMusicFailed || !strings.Contains(done.Error.Message, "Click Generate") {
			t.Fatalf("key %q: job = %+v %+v", generateKey, done, done.Error)
		}
	}
	if fx.calls.Load() != 0 {
		t.Fatalf("the provider was called %d times", fx.calls.Load())
	}
}

func TestStorymakerMusicGeneratedStopsWhenReadOnly(t *testing.T) {
	fx := newGeneratedMusicFixture(t)
	started := make(chan struct{})
	fx.seam.generate = func(ctx context.Context, cfg *config.Config, params tools.MusicGenParams) tools.MusicGenResult {
		close(started)
		<-ctx.Done()
		return tools.MusicGenResult{Status: "error", Error: ctx.Err().Error()}
	}
	job := fx.post(t, `{"duration_ms":15000,"generate":true}`)
	select {
	case <-started:
	case <-time.After(20 * time.Second):
		t.Fatal("the provider call did not start")
	}
	ro := *fx.s.ConfigSnapshot()
	ro.VirtualDesktop.ReadOnly = true
	fx.s.replaceConfigSnapshot(&ro)
	// The check right before a provider call sees the new state.
	if je := fx.s.storymakerMusicPreflight(); je == nil || je.Code != render.CodeMusicFailed || !strings.Contains(je.Message, "read-only") {
		t.Fatalf("preflight = %+v", je)
	}
	// The call in flight is cancelled and stores nothing.
	done := waitStorymakerMusicJob(t, fx.rt, job.ID)
	if done.Status != render.StatusFailed || done.Error == nil || done.Error.Code != render.CodeMusicFailed || !strings.Contains(done.Error.Message, "read-only") {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if _, ext, err := fx.rt.projects.MusicInfo(context.Background(), fx.id, fx.key); err != nil || ext != "" {
		t.Fatalf("stored %q, %v", ext, err)
	}
}

func TestStorymakerMusicRerunReadsPendingWithoutAProviderCall(t *testing.T) {
	fx := newGeneratedMusicFixture(t)
	ctx := context.Background()
	fx.audioFile(t, "left.wav", 5)
	target, err := fx.rt.projects.AudioPath(fx.id, "music", fx.key+".wav")
	if err != nil {
		t.Fatal(err)
	}
	pending := path.Join(path.Dir(target), fx.key+".pending.json")
	if err := fx.rt.store.WriteBytes(ctx, pending, []byte(`{"path":"left.wav"}`)); err != nil {
		t.Fatal(err)
	}
	done := waitStorymakerMusicJob(t, fx.rt, fx.post(t, `{"duration_ms":15000,"generate":true}`).ID)
	if done.Status != render.StatusSucceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if fx.calls.Load() != 0 {
		t.Fatalf("the provider was called %d times", fx.calls.Load())
	}
	// 5000 is the 5 s of left.wav, not the 20 s the stand-in would return.
	meta, _, err := fx.rt.projects.MusicInfo(ctx, fx.id, fx.key)
	if err != nil || meta.DurationMS != 5000 {
		t.Fatalf("meta = %+v, %v", meta, err)
	}
	if _, _, err := fx.rt.store.ReadFile(ctx, pending); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the pending file stayed: %v", err)
	}
}

func TestStorymakerMusicGeneratedScrubsTheProviderFailure(t *testing.T) {
	fx := newGeneratedMusicFixture(t)
	const sentinel = "sm-music-sentinel-0123456789abcdef"
	defer security.RegisterScopedSensitiveExact(sentinel)()
	// The sentinel crosses the 300-rune cut, so a scrub after the clip would no longer match it and would leave its head.
	fx.seam.generate = func(ctx context.Context, cfg *config.Config, params tools.MusicGenParams) tools.MusicGenResult {
		return tools.MusicGenResult{Status: "error", Error: strings.Repeat("x", 280) + sentinel + " echoed by the provider"}
	}
	done := waitStorymakerMusicJob(t, fx.rt, fx.post(t, `{"duration_ms":15000,"generate":true}`).ID)
	if done.Status != render.StatusFailed || done.Error == nil || done.Error.Code != render.CodeMusicFailed {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if msg := done.Error.Message; strings.Contains(msg, sentinel[:8]) || !strings.Contains(msg, "The music provider failed: ") || !strings.Contains(msg, "[redacted]") {
		t.Fatalf("message = %q", msg)
	}
}

func TestStorymakerMusicGeneratedRecordsItsCostOnce(t *testing.T) {
	fx := newGeneratedMusicFixture(t)
	tracker := musicTestTracker(t, 100, "warn")
	fx.s.BudgetTracker = tracker
	done := waitStorymakerMusicJob(t, fx.rt, fx.post(t, `{"duration_ms":15000,"generate":true}`).ID)
	if done.Status != render.StatusSucceeded {
		t.Fatalf("job = %+v %+v", done, done.Error)
	}
	if w := postMusic(fx.s, fx.token, fx.id, `{"duration_ms":15000,"generate":true}`); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"job":null`) {
		t.Fatalf("a second call: %d %s", w.Code, w.Body.String())
	}
	// The stand-in reports 0.06 USD, and the second call queued nothing.
	if got := tracker.CategorySpendUSD("music_generation"); got != 0.06 || fx.calls.Load() != 1 {
		t.Fatalf("spend = %v after %d calls", got, fx.calls.Load())
	}
}
```

Run `gofmt -w internal/server/storymaker_audio_test.go` after the paste. The comments in the table above do not line up by hand.

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/server -run '^TestStorymakerMusic(Seconds|Generated|Rerun)' -count=1
```

Expected: FAIL to compile with `undefined: storymakerMusicSeam`.

- [ ] **Step 4: Implement**

**`internal/server/server.go`.** The comment line starts its own gofmt alignment block.

Old text:

```go
	storymakerFrameObserver func(n int, transport string, data []byte)
```

New text:

```go
	storymakerFrameObserver func(n int, transport string, data []byte)
	// storymakerMusicOverride replaces the provider call of a generated music job in tests.
	storymakerMusicOverride *storymakerMusicSeam
```

**`internal/server/storymaker_narrate.go`, the imports.** A25b left the block below. `aurago/internal/security` is already in the file since N11, and the refusal in the generated branch uses it.

Old text:

```go
	"os"
	"path/filepath"
```

New text:

```go
	"os"
	"path"
	"path/filepath"
```

Old text:

```go
	"aurago/internal/config"
```

New text:

```go
	"aurago/internal/acestep"
	"aurago/internal/config"
```

**`internal/server/storymaker_narrate.go`, the generated branch.** A25b left the stub below.

Old text:

```go
	if m.Mode == "generated" {
		return "", &storymakerMusicRefusal{message: "Generated music cannot run yet."}
	}
```

New text:

```go
	if m.Mode == "generated" {
		return s.storymakerGenerateMusic(ctx, rt, cfg, p, key, m, h)
	}
```

**`internal/server/storymaker_narrate.go`, the end of `storymakerFinishMusic`.** A25b left the text below. The pending file goes in both outcomes, or a bad generated file would fail every rerun with no new call.

Old text:

```go
		if err := rt.store.Remove(ctx, target); err != nil {
			return err
		}
		return &storymakerMusicRefusal{message: "The music must be readable audio that lasts between 1 second and 1 hour."}
	}
	return rt.projects.WriteMusicMeta(ctx, id, key, storymaker.MusicMeta{V: 1, Ext: ext, DurationMS: durationMS})
}
```

New text:

```go
		if err := rt.store.Remove(ctx, target); err != nil {
			return err
		}
		if err := storymakerRemoveMusicPending(ctx, rt, id, key); err != nil {
			return err
		}
		return &storymakerMusicRefusal{message: "The music must be readable audio that lasts between 1 second and 1 hour."}
	}
	if err := rt.projects.WriteMusicMeta(ctx, id, key, storymaker.MusicMeta{V: 1, Ext: ext, DurationMS: durationMS}); err != nil {
		return err
	}
	return storymakerRemoveMusicPending(ctx, rt, id, key)
}
```

**`internal/server/storymaker_narrate.go`, the new code.** Append to the end of the file:

```go
// storymakerMusicSeam replaces the provider call and the ACE-Step profile limit in tests.
type storymakerMusicSeam struct {
	generate   func(ctx context.Context, cfg *config.Config, params tools.MusicGenParams) tools.MusicGenResult
	profileMax int
}

// storymakerMusicPending is <key>.pending.json: where a paid result sits, relative to DataDir/audio.
type storymakerMusicPending struct {
	Path string `json:"path"`
}

func storymakerMusicPendingPath(rt *storymakerRuntime, id, key string) (string, error) {
	audio, err := rt.projects.AudioPath(id, "music", key+".wav")
	if err != nil {
		return "", err
	}
	return path.Join(path.Dir(audio), key+".pending.json"), nil
}

func storymakerRemoveMusicPending(ctx context.Context, rt *storymakerRuntime, id, key string) error {
	pendingPath, err := storymakerMusicPendingPath(rt, id, key)
	if err != nil {
		return err
	}
	return rt.store.Remove(ctx, pendingPath)
}

// storymakerReadMusicPending returns the stored result path, or "" without a usable file. It is an ordinary workspace
// file, so its path is not trusted.
func storymakerReadMusicPending(ctx context.Context, rt *storymakerRuntime, pendingPath string) (string, error) {
	data, _, err := rt.store.ReadFile(ctx, pendingPath)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var pending storymakerMusicPending
	if json.Unmarshal(data, &pending) != nil || !filepath.IsLocal(filepath.FromSlash(pending.Path)) {
		return "", nil
	}
	return pending.Path, nil
}

// storymakerMusicSeconds asks for the film plus 2 s, at least 10 s and at most 600 s or the ACE-Step limit. A value above
// that limit fails instead of shortening, and the mix loops the track for the rest of a longer film.
func storymakerMusicSeconds(durationMS, profileMax int) float64 {
	limit := 600
	if profileMax > 0 && profileMax < limit {
		limit = profileMax
	}
	seconds := (durationMS+999)/1000 + 2
	if seconds < 10 {
		seconds = 10
	}
	if seconds > limit {
		seconds = limit
	}
	return float64(seconds)
}

// storymakerMusicProfileMax is the ACE-Step profile limit, 0 when there is none.
func (s *Server) storymakerMusicProfileMax(cfg *config.Config) int {
	if !cfg.UsesLocalMusic() {
		return 0
	}
	if s.storymakerMusicOverride != nil {
		return s.storymakerMusicOverride.profileMax
	}
	manager := acestep.Default()
	if manager == nil {
		return 0
	}
	if profile := manager.Status().Profile; profile != nil {
		return profile.MaxDuration
	}
	return 0
}

// storymakerMusicPreflight reads the configuration again right before a paid call.
func (s *Server) storymakerMusicPreflight() *render.JobError {
	cfg := s.ConfigSnapshot()
	if cfg == nil || cfg.VirtualDesktop.ReadOnly {
		return storymakerMusicStopped()
	}
	if !cfg.UsesLocalMusic() && s.BudgetTracker.IsBlocked("music_generation") {
		return render.Fail(render.CodeBudgetExceeded, "The daily budget is used up, so no music was generated.")
	}
	return nil
}

func (s *Server) storymakerCallMusicProvider(ctx context.Context, cfg *config.Config, params tools.MusicGenParams) tools.MusicGenResult {
	if seam := s.storymakerMusicOverride; seam != nil && seam.generate != nil {
		return seam.generate(ctx, cfg, params)
	}
	return tools.GenerateMusicResult(ctx, cfg, s.MediaRegistryDB, s.storymakerMusicLogger(), params)
}

func storymakerMusicClip(text string) string {
	runes := []rune(text)
	if len(runes) > 300 {
		return string(runes[:300]) + "..."
	}
	return text
}

// storymakerCopyGeneratedMusic copies a result below the Music mount to the key's file and returns its format.
func storymakerCopyGeneratedMusic(ctx context.Context, rt *storymakerRuntime, cfg *config.Config, id, key, rel string) (string, error) {
	src, info, err := rt.store.Open(ctx, "Music/"+rel)
	if err != nil {
		return "", err
	}
	defer src.Close()
	ext, err := storymakerCheckMusicSource(src, info.Size, cfg.VirtualDesktop.WorkspaceDir)
	if err != nil {
		return "", err
	}
	return ext, storymakerStoreMusic(ctx, rt, id, key, ext, src)
}

// storymakerGenerateMusic runs the provider once and copies the result. The pending file comes right after the call, so a
// failed copy or a restart does not pay again while the result sits in DataDir/audio.
func (s *Server) storymakerGenerateMusic(ctx context.Context, rt *storymakerRuntime, cfg *config.Config, p storymakerMusicPayload, key string, m *storymaker.Music, h *render.Handle) (string, error) {
	pendingPath, err := storymakerMusicPendingPath(rt, p.ProjectID, key)
	if err != nil {
		return "", err
	}
	rel, err := storymakerReadMusicPending(ctx, rt, pendingPath)
	if err != nil {
		return "", err
	}
	if rel != "" {
		h.SetPhase(render.PhaseWriting)
		ext, err := storymakerCopyGeneratedMusic(ctx, rt, cfg, p.ProjectID, key, rel)
		if !errors.Is(err, fs.ErrNotExist) {
			return ext, err
		}
		// The result is gone, so only a new call can bring it back.
	}
	// A paid call needs the click that named this mood and prompt. A file job that finds a generated story has no key, and a
	// click whose story changed while the job waited has an older one. A pending result above is already paid for.
	if p.GenerateKey != key {
		return "", &storymakerMusicRefusal{message: "The music settings changed while the job waited, so nothing was generated. Click Generate to start it."}
	}
	if je := s.storymakerMusicPreflight(); je != nil {
		return "", je
	}
	h.SetPhase(render.PhaseGenerating)
	result := s.storymakerCallMusicProvider(ctx, cfg, tools.MusicGenParams{
		Prompt:          storymaker.MusicPrompt(m.Mood, m.Prompt),
		Instrumental:    true,
		DurationSeconds: storymakerMusicSeconds(p.DurationMS, s.storymakerMusicProfileMax(cfg)),
	})
	if result.Status != "ok" {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		// The body can echo a key. Scrub first, because a clip through a secret would no longer match it.
		return "", &storymakerMusicRefusal{message: "The music provider failed: " + storymakerMusicClip(security.Scrub(result.Error))}
	}
	s.BudgetTracker.RecordCostForCategory("music_generation", result.CostEstimate)
	rel, err = storymakerMusicAudioRel(cfg, result.FilePath)
	if err != nil {
		return "", err
	}
	pending, err := json.Marshal(storymakerMusicPending{Path: rel})
	if err != nil {
		return "", err
	}
	if err := rt.store.WriteBytes(ctx, pendingPath, pending); err != nil {
		return "", err
	}
	h.SetPhase(render.PhaseWriting)
	return storymakerCopyGeneratedMusic(ctx, rt, cfg, p.ProjectID, key, rel)
}
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/server -run '^TestStorymaker(Music|StatusReportsMusic)' -count=1
go vet ./internal/server
gofmt -l internal/server
```

Expected: PASS, no vet finding, and `gofmt -l` prints nothing.

- [ ] **Step 6: Commit**

Run `detect_changes` first. A PARTIAL answer on new files needs a grep confirmation in the commit body (outline section 5).

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/server/storymaker_narrate.go internal/server/server.go internal/server/storymaker_audio_test.go
git commit -m "feat(storymaker): generate music once, with budget and read-only checks" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task A26: Parity and export with audio
**Batch:** 2C  **Depends on:** A20, A24
**Files:**
- Test: `internal/server/storymaker_parity_browser_test.go`
- Test: `internal/server/storymaker_e2e_test.go`

Both files are tests. The behaviour shipped in A20 and A24, and this task proves it on the real server.

- [ ] **Step 1: Impact**

Run from the main checkout `C:/Users/Andi/Documents/repo/AuraGo`:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "storymakerFFprobe" --direction upstream --repo .
```

Report callers, processes and risk. Warn on HIGH or CRITICAL, and confirm `risk: UNKNOWN` with a text search (outline section 5, GitNexus). The test function bodies in this task have no callers.

- [ ] **Step 2: Write the failing test**

**`internal/server/storymaker_parity_browser_test.go`, the result fields.** A multi-line field ends a gofmt alignment block, so these two line up on their own.

Old text:

```go
			FrameCount int `json:"frame_count"`
		} `json:"report"`
	}
```

New text:

```go
			FrameCount int `json:"frame_count"`
		} `json:"report"`
		Narrated int `json:"narrated"`
		Missing  int `json:"missing"`
	}
```

**`internal/server/storymaker_parity_browser_test.go`, the check.**

Old text:

```go
	if res.Report.FrameCount != job.FramesTotal {
```

New text:

```go
	// firewall.json has narration in all 3 scenes and none is silent, so the app got 3 measured scenes and nothing missing.
	if res.Narrated != 3 || res.Missing != 0 {
		t.Fatalf("the app got %d narrated scenes and %d missing blocks", res.Narrated, res.Missing)
	}
	if res.Report.FrameCount != job.FramesTotal {
```

**`internal/server/storymaker_e2e_test.go`, the imports.**

Old text:

```go
	"io"
	"net/http"
```

New text:

```go
	"io"
	"math"
	"net/http"
```

Old text:

```go
	"path/filepath"
	"strconv"
```

New text:

```go
	"path/filepath"
	"regexp"
	"strconv"
```

**`internal/server/storymaker_e2e_test.go`, the stream duration.**

Old text:

```go
		PixFmt       string `json:"pix_fmt"`
```

New text:

```go
		PixFmt       string `json:"pix_fmt"`
		Duration     string `json:"duration"`
```

Old text:

```go
"stream=codec_type,codec_name,width,height,avg_frame_rate,nb_frames,pix_fmt:format=duration"
```

New text:

```go
"stream=codec_type,codec_name,width,height,avg_frame_rate,nb_frames,pix_fmt,duration:format=duration"
```

**`internal/server/storymaker_e2e_test.go`, the level helper.**

Old text:

```go
type storymakerSSEEvent struct {
```

New text:

```go
// storymakerMeasureLUFS reads the integrated loudness of a file's first audio stream from the last I: line of FFmpeg's
// ebur128 summary. framelog=verbose prints no frame lines at the default log level.
func storymakerMeasureLUFS(t *testing.T, ffmpeg, file string) float64 {
	t.Helper()
	out, err := exec.Command(ffmpeg, "-hide_banner", "-nostdin", "-nostats", "-i", file, "-map", "0:a:0", "-af", "ebur128=framelog=verbose", "-f", "null", "-").CombinedOutput()
	if err != nil {
		t.Fatalf("ebur128 over %s: %v\n%s", file, err, out)
	}
	matches := regexp.MustCompile(`I:\s+(-?[0-9.]+) LUFS`).FindAllSubmatch(out, -1)
	if len(matches) == 0 {
		t.Fatalf("ebur128 printed no integrated loudness:\n%s", out)
	}
	lufs, err := strconv.ParseFloat(string(matches[len(matches)-1][1]), 64)
	if err != nil {
		t.Fatal(err)
	}
	return lufs
}

type storymakerSSEEvent struct {
```

**`internal/server/storymaker_e2e_test.go`, the checks.**

Old text:

```go
	sidecar, err := os.ReadFile(filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, filepath.FromSlash(prefix+accepted.Job.ID+".json")))
```

New text:

```go
	// The frame gate held: the audio ends with the video, within the 25 ms of gate.go.
	var videoSeconds, audioSeconds float64
	for _, st := range doc.Streams {
		d, _ := strconv.ParseFloat(st.Duration, 64)
		switch st.CodecType {
		case "video":
			videoSeconds = d
		case "audio":
			audioSeconds = d
		}
	}
	if videoSeconds <= 0 || audioSeconds <= 0 || math.Abs(audioSeconds-videoSeconds) >= 0.025 {
		t.Errorf("audio %v s against video %v s", audioSeconds, videoSeconds)
	}
	// The seeded speech sits near -20 dBFS, so the master gain is bound by the -16 LUFS target, not by the peak ceiling.
	// The window is the one A20 pins on a seeded story.
	if lufs := storymakerMeasureLUFS(t, s.Cfg.VideoStudio.FFmpegPath, mp4); lufs < -18.5 || lufs > -15.5 {
		t.Errorf("the export measures %v LUFS, want -18.5 to -15.5", lufs)
	}
	sidecar, err := os.ReadFile(filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, filepath.FromSlash(prefix+accepted.Job.ID+".json")))
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/server -run '^TestStorymaker(ParityAppPlayerAndRenderPage|ExportEndToEnd)$' -v -count=1 -timeout 15m; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
```

Expected: `TestStorymakerParityAppPlayerAndRenderPage` FAILS with `the app got 0 narrated scenes and 0 missing blocks`, because the parent page still sends `audio_info: {}` and never reads `/narration`. `TestStorymakerExportEndToEnd` passes, because the behaviour it pins shipped in A20 and A24.

- [ ] **Step 4: Implement**

Edit the parent page in `internal/server/storymaker_parity_browser_test.go`. It is a format string for `fmt.Fprintf`, so the new text uses no percent sign.

Old text:

```go
      for (const p of [catalog.rig.file].concat(catalog.entries.filter(e => e.file && e.kind !== 'sfx').map(e => e.file))) files['catalog:' + p] = await getJSON('/img/storymaker/' + p);
```

New text:

```go
      for (const p of [catalog.rig.file].concat(catalog.entries.filter(e => e.file && e.kind !== 'sfx').map(e => e.file))) files['catalog:' + p] = await getJSON('/img/storymaker/' + p);
      if (catalog.glyphs) catalog.glyph_data = await getJSON('/img/storymaker/' + catalog.glyphs.file);
```

Old text:

```go
      post({ type: 'load', story: project.story, catalog, audio_info: {}, assets: files, fonts, code_bundles: bundles, format: { aspect: '16:9', fps: 30, width: 1920 } });
```

New text:

```go
      const answer = await getJSON(API + '/projects/' + PROJECT + '/narration');
      par.narrated = Object.keys(answer.narration).length;
      par.missing = answer.missing.length;
      post({ type: 'load', story: project.story, catalog, audio_info: { narration: answer.narration }, assets: files, fonts, code_bundles: bundles, format: { aspect: '16:9', fps: 30, width: 1920 } });
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
$env:AURAGO_RUN_BROWSER_SMOKE='1'; go test ./internal/server -run '^TestStorymaker(ParityAppPlayerAndRenderPage|ExportEndToEnd)$' -v -count=1 -timeout 15m; Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
go vet ./internal/server
gofmt -l internal/server
```

Expected: 2 `--- PASS` lines, with the log line `frames identical between the app player host and the render page`. There is no vet finding, and `gofmt -l` prints nothing. Keep the shell sandbox off, and stop only the browser PIDs you started yourself.

- [ ] **Step 6: Commit**

Run `detect_changes` first. A PARTIAL answer on new files needs a grep confirmation in the commit body (outline section 5).

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/server/storymaker_parity_browser_test.go internal/server/storymaker_e2e_test.go
git commit -m "test(storymaker): prove parity and an audible export with seeded audio" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

{% endraw %}
