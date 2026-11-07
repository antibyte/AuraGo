{% raw %}
# Part M: Motion craft

Batch 2A. Run the tasks in order. `outline.md` holds the batch rules, the test matrices and the execution rules ("outline section N"). `interfaces.md` holds the frozen interfaces ("frozen section N"), and its section 10 and `reconcile.md` win over them. A group id such as `G-A-5` names a block of tasks inside a part; the index lists them.

<!-- G-M-1 -->

### Task M0: Make the refusal test reliable
**Batch:** 2A  **Depends on:** none
**Files:**
- Test: `internal/storymaker/render/render_test.go`

`TestRunRefusesReportsThatDisagreeWithTheJob` already fails on main, on most runs and with varying subtests, so every matrix of this phase would start red. The product code is right: `onReport` refuses, the job ends with the right error. The test has two faults:
- The fake page's report POST uses the job context. `jobs.go` cancels that context as soon as `Run` returns, so the POST ends with `context canceled` and the answer never reaches the page.
- The fake page runs in its own goroutine, and the test reads `refused` as soon as the job ends, with no synchronisation.

- [ ] **Step 1: See it fail**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker/render -run '^TestRunRefusesReportsThatDisagreeWithTheJob$' -count=3
```

Expected: FAIL in most runs, with `the page was told {Start:false ...}` at the line that reads `refused`. If it passes three times on this box, go on: the fix also removes the race.

- [ ] **Step 2: Fix the test**

In `internal/storymaker/render/render_test.go`:

Old text:

```go
			out, st, err := httpDo(ctx, "POST", base+"/report", "application/json", nil, body)
```

New text:

```go
			out, st, err := httpDo(context.WithoutCancel(ctx), "POST", base+"/report", "application/json", nil, body)
```

Old text:

```go
			fx.playing(playOptions{frames: 24, mutateRep: tc.mut, onRefused: func(a ReportAnswer) { refused = a }})
			job := fx.render(t)
```

New text:

```go
			told := make(chan struct{})
			fx.playing(playOptions{frames: 24, mutateRep: tc.mut, onRefused: func(a ReportAnswer) { refused = a; close(told) }})
			job := fx.render(t)
			select {
			case <-told:
			case <-time.After(10 * time.Second):
				t.Fatal("the page was never told about the refusal")
			}
```

- [ ] **Step 3: Run**

```powershell
go test ./internal/storymaker/render -run '^TestRunRefusesReportsThatDisagreeWithTheJob$' -count=5
go test ./internal/storymaker/render -count=1 -timeout 15m
gofmt -l internal/storymaker/render
```

Expected: both test lines pass, and `gofmt -l` prints nothing.

- [ ] **Step 4: Commit**

Only a test changes, so `detect_changes` has nothing to map beyond the test file. This is the first run of the command form in outline section 5. Run the `node` line alone from the root of `.worktrees/storymaker-p2a`: the answer must count one file, as `Changes: 1 files` or `Diff touched 1 file(s)`. Anything else means the form failed, and M1 Step 6 says what to do then.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/render/render_test.go
git commit -m "test(storymaker): make the report refusal test reliable" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task M1: `style.rhythm` in Go
**Batch:** 2A  **Depends on:** none
**Files:**
- Modify: `internal/storymaker/model.go`
- Modify: `internal/storymaker/validate.go`
- Modify: `internal/storymaker/decode.go`
- Test: `internal/storymaker/validate_test.go`
- Test: `internal/storymaker/decode_test.go`

- [ ] **Step 1: Impact**

From the main checkout:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "Style" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "style" --direction upstream --repo .
```

Report callers, processes and risk. Warn on HIGH or CRITICAL, and treat UNKNOWN as unresolved with a text search, as the GitNexus rule in outline section 5 says.

- [ ] **Step 2: Write the failing tests**

In `internal/storymaker/validate_test.go`, replace

```go
func TestValidateVoiceProvider(t *testing.T) {
```

with

```go
func TestValidateStyleRhythm(t *testing.T) {
	cat := loadMini(t)
	check := func(rhythm string) []Issue {
		s := storyOf(t, `{"id":"s1"}`)
		s.Style.Rhythm = rhythm
		return Validate(&s, cat, DefaultLimits())
	}
	expectIssue(t, check("fast"), CodeInvalidValue, SeverityError, "/style/rhythm")
	for _, rhythm := range []string{"", "calm", "standard", "lively"} {
		if is := findIssue(check(rhythm), CodeInvalidValue, SeverityError, "/style/rhythm"); is != nil {
			t.Errorf("rhythm %q refused: %+v", rhythm, is)
		}
	}
}

func TestValidateVoiceProvider(t *testing.T) {
```

In `internal/storymaker/decode_test.go`, replace

```go
func TestCanonicalJSONIsStableAndRoundTrips(t *testing.T) {
```

with

```go
func TestStyleRhythmIsOnlyWrittenWhenSet(t *testing.T) {
	without := `{"schema":"storymaker/1","title":"x","style":{"palette":"sunny","font":"default","captions":true},"scenes":[]}`
	out, err := CanonicalJSON(mustDecode(t, without))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "rhythm") {
		t.Fatalf("a canonical save without a rhythm writes the key:\n%s", out)
	}
	with := `{"schema":"storymaker/1","title":"x","style":{"palette":"sunny","font":"default","captions":true,"rhythm":"lively"},"scenes":[]}`
	s := mustDecode(t, with)
	if s.Style.Rhythm != "lively" {
		t.Fatalf("rhythm lost: %+v", s.Style)
	}
	out, err = CanonicalJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"rhythm": "lively"`) {
		t.Fatalf("rhythm not written:\n%s", out)
	}
}

func TestDecodeRhythmHints(t *testing.T) {
	issues := decodeFail(t, `{"schema":"storymaker/1","title":"x","rhythm":"calm","style":{"tempo":"fast"},"scenes":[{"id":"s1","rhythm":"calm"}]}`)
	want := map[string]string{"/rhythm": `"style"`, "/style/tempo": `"rhythm"`, "/scenes/0/rhythm": `"style"`}
	if len(issues) != len(want) {
		t.Fatalf("want %d issues, got %+v", len(want), issues)
	}
	for _, is := range issues {
		hint, ok := want[is.Path]
		if !ok || is.Code != CodeUnknownField || !strings.Contains(is.Fix, hint) {
			t.Errorf("issue %+v, want an unknown_field whose fix has %s", is, hint)
		}
	}
}

func TestCanonicalJSONIsStableAndRoundTrips(t *testing.T) {
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker -run '^(TestValidateStyleRhythm|TestStyleRhythmIsOnlyWrittenWhenSet|TestDecodeRhythmHints)$' -count=1
```

Expected: the test package does not build, with `s.Style.Rhythm undefined (type Style has no field or method Rhythm)`, because the field does not exist yet.

- [ ] **Step 4: Implement**

In `internal/storymaker/model.go`, replace

```go
// Style names a catalog palette and font and switches captions.
```

with

```go
// Style names a catalog palette and font, switches captions and sets the rhythm.
```

and replace

```go
	Captions bool   `json:"captions"`
```

with

```go
	Captions bool   `json:"captions"`
	Rhythm   string `json:"rhythm,omitempty"` // "" | calm | standard | lively; Go never writes a default
```

In `internal/storymaker/validate.go`, replace

```go
			"Use one of: "+listOf(alts)+". Example: "+exampleJSON("font", alts[0]))
	}
}
```

with

```go
			"Use one of: "+listOf(alts)+". Example: "+exampleJSON("font", alts[0]))
	}
	switch st.Rhythm {
	case "", "calm", "standard", "lively":
	default:
		v.errorf(CodeInvalidValue, "/style/rhythm", fmt.Sprintf("The rhythm %q is not supported.", clip(st.Rhythm, 20)), `Use "calm", "standard" or "lively": {"rhythm":"calm"}.`)
	}
}
```

In `internal/storymaker/decode.go`, replace

```go
	"Story.name":        `Use "title".`,
```

with

```go
	"Story.name":        `Use "title".`,
	"Story.rhythm":      `The rhythm belongs to the style: "style": {"rhythm":"calm"}.`,
```

replace

```go
	"Scene.framing":     `The camera is "camera": {"framing":"wide"}.`,
```

with

```go
	"Scene.framing":     `The camera is "camera": {"framing":"wide"}.`,
	"Scene.rhythm":      `The rhythm is set once for the story: "style": {"rhythm":"calm"}.`,
```

and replace

```go
	"Style.subtitles": `Use "captions": true or false.`,
```

with

```go
	"Style.subtitles": `Use "captions": true or false.`,
	"Style.tempo":     `Use "rhythm": "calm", "standard" or "lively".`,
	"Style.pace":      `Use "rhythm": "calm", "standard" or "lively".`,
```

- [ ] **Step 5: Run**

```powershell
go test ./internal/storymaker -run '^(TestValidateStyleRhythm|TestStyleRhythmIsOnlyWrittenWhenSet|TestDecodeRhythmHints)$' -count=1
go test ./internal/storymaker/... -count=1
gofmt -l internal/storymaker
```

Expected: both test lines pass, and `gofmt -l` prints nothing. `style.auto_sfx` stays `unknown_field`.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5. A PARTIAL answer on a new file needs a grep confirmation in the commit body, above the trailer.

One-time check of the command form, on the first commit that edits tracked source files. Run the `node` line alone from the root of `.worktrees/storymaker-p2a` and read its answer before `git add`. The answer must be about M1's five files: `Changes: 5 files` with symbol lines under `internal/storymaker/model.go` and the other files of this task, or `Diff touched 5 file(s)` if no indexed symbol overlaps the hunks. Other sessions' dirty files or a much larger count mean it read the main checkout. If it answers `not found`, prints only its banner or reads the wrong tree, call MCP `detect_changes` with `repo` set to `C:/Users/Andi/Documents/repo/AuraGo`, `scope` set to `all` and `worktree` set to the absolute worktree path, and use that call in place of the `node` line from here on. If that fails too, take the fallback in outline section 5 and say so in the commit body. Do not gate on `$LASTEXITCODE`: a `partial: true` answer exits 1 on purpose.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/model.go internal/storymaker/validate.go internal/storymaker/decode.go internal/storymaker/validate_test.go internal/storymaker/decode_test.go
git commit -m "feat(storymaker): style.rhythm in the story model" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task M2: Default transition by environment
**Batch:** 2A  **Depends on:** none
**Files:**
- Modify: `internal/storymaker/defaults.go`
- Test: `internal/storymaker/defaults_test.go`

- [ ] **Step 1: Impact**

From the main checkout:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "ApplyDefaults" --direction upstream --repo .
```

Report callers, processes and risk. Warn on HIGH or CRITICAL, and treat UNKNOWN as unresolved with a text search, as the GitNexus rule in outline section 5 says.

- [ ] **Step 2: Write the failing test**

In `internal/storymaker/defaults_test.go`, change the pin of the second scene. It moves from the default `env.plain-stage` to `env.office`, so it now gets `wipe`. The third scene inherits `env.office` and stays `fade`, so that pin needs no edit. Replace

```go
		"/scenes/1/variant": "day", "/scenes/1/speaker": "narrator", "/scenes/1/transition": "fade", "/scenes/1/duration": "auto",
```

with

```go
		"/scenes/1/variant": "day", "/scenes/1/speaker": "narrator", "/scenes/1/transition": "wipe", "/scenes/1/duration": "auto",
```

Then add the new test. Replace

```go
func TestApplyDefaultsIsIdempotentAndNeverTouchesTiming(t *testing.T) {
```

with

```go
func TestApplyDefaultsTransitionFollowsTheEnvironment(t *testing.T) {
	s := mustDecode(t, `{"schema":"storymaker/1","title":"T","scenes":[
	  {"id":"s1","environment":"env.office"},
	  {"id":"s2","environment":"env.office"},
	  {"id":"s3","environment":"env.plain-stage"},
	  {"id":"s4"},
	  {"id":"s5","environment":"env.office","transition":"fade"}]}`)
	m := appliedMap(ApplyDefaults(&s))
	for path, want := range map[string]string{
		"/scenes/0/transition": "cut",  // first scene, whatever its environment
		"/scenes/1/transition": "fade", // env.office again
		"/scenes/2/transition": "wipe", // env.office to env.plain-stage
		"/scenes/3/transition": "fade", // no environment: inherits env.plain-stage
	} {
		if m[path] != want {
			t.Errorf("%s = %#v, want %q", path, m[path], want)
		}
	}
	// s5 changes the environment, but its stored fade stays and is not listed as a default.
	if _, listed := m["/scenes/4/transition"]; listed || s.Scenes[4].Transition != "fade" {
		t.Errorf("stored fade changed: listed=%v value=%q", listed, s.Scenes[4].Transition)
	}
}

func TestApplyDefaultsIsIdempotentAndNeverTouchesTiming(t *testing.T) {
```

- [ ] **Step 3: Run it**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker -run '^TestApplyDefaults' -count=1
```

Expected: `TestApplyDefaultsTable` fails with `/scenes/1/transition = "fade", want "wipe"`, and `TestApplyDefaultsTransitionFollowsTheEnvironment` fails with `/scenes/2/transition = "fade", want "wipe"`. Today every later scene gets `fade`.

- [ ] **Step 4: Implement**

In `internal/storymaker/defaults.go`, replace

```go
		prevEnv = sc.Environment
```

with

```go
		envChanged := i > 0 && sc.Environment != prevEnv
		prevEnv = sc.Environment
```

and replace

```go
			sc.Transition = "fade"
			if i == 0 {
				sc.Transition = "cut"
			}
```

with

```go
			switch {
			case i == 0:
				sc.Transition = "cut"
			case envChanged:
				sc.Transition = "wipe"
			default:
				sc.Transition = "fade"
			}
```

- [ ] **Step 5: Run**

```powershell
go test ./internal/storymaker -run '^TestApplyDefaults' -count=1
go test ./internal/storymaker/... -count=1
gofmt -l internal/storymaker
```

Expected: both test lines pass, and `gofmt -l` prints nothing.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5. A PARTIAL answer on a new file needs a grep confirmation in the commit body, above the trailer.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/defaults.go internal/storymaker/defaults_test.go
git commit -m "feat(storymaker): default transition follows the environment" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task M3: Range tables from one generator (Wire: Go+JS)
**Batch:** 2A  **Depends on:** none
**Files:**
- Create: `scripts/build-storymaker-ranges.mjs`
- Create: `scripts/storymaker-ucd-16.0.0.txt` (written by `--extract`)
- Create: `ui/js/storymaker/ranges.js` (generated)
- Create: `internal/storymaker/ranges_gen.go` (generated)
- Modify: `scripts/storymaker-bundle-parts.js`
- Modify: `package.json`
- Modify: `ui/js/storymaker/player.bundle.js` (generated by `npm run build:ui`)
- Test: `scripts/storymaker-tests/12-ranges.mjs`

- [ ] **Step 1: Write the failing test**

Create `scripts/storymaker-tests/12-ranges.mjs`:

```js
// Task M3: the generated code point tables and the text rules on them, in JS and in Go.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { root, read } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check } = t;
const SM = t.core();
const GENERATOR = path.join(root, 'scripts', 'build-storymaker-ranges.mjs');
const JS_OUT = 'ui/js/storymaker/ranges.js', GO_OUT = 'internal/storymaker/ranges_gen.go';
const text = (...cps) => String.fromCodePoint(...cps);
const HAN = 0x6F22, ZI = 0x5B57, KA = 0x915, VIRAMA = 0x94D, SSA = 0x937;

function goTable(src, name) {
    const at = src.indexOf('var ' + name + ' = []ucdRange{');
    assert.ok(at >= 0, name + ' is in ' + GO_OUT);
    const body = src.slice(at, src.indexOf('\n}', at));
    return [...body.matchAll(/\{0x([0-9A-F]+), 0x([0-9A-F]+)\}/g)].map(m => [parseInt(m[1], 16), parseInt(m[2], 16)]);
}

check('ranges: canBreakAfter keeps surrogate pairs, marks, conjuncts and kinsoku together', () => {
    const R = SM.ranges;
    assert.equal(R.canBreakAfter(0xD83D, 0xDE00), false, 'inside a surrogate pair');
    assert.equal(R.canBreakAfter(0x41, 0x300), false, 'before a combining mark');
    assert.equal(R.canBreakAfter(KA, VIRAMA), false, 'before the virama, which is a mark');
    assert.equal(R.canBreakAfter(VIRAMA, SSA), false, 'after U+094D');
    assert.equal(R.canBreakAfter(0x200D, 0x1F600), false, 'after U+200D');
    assert.equal(R.canBreakAfter(HAN, 0x3002), false, 'no line starts with an ideographic full stop');
    assert.equal(R.canBreakAfter(0x300C, HAN), false, 'no line ends with an opening corner bracket');
    assert.equal(R.canBreakAfter(HAN, ZI), true);
    assert.equal(R.cjkBreak(HAN, ZI), true);
    assert.equal(R.cjkBreak(0x41, HAN), false, 'only between two CJK word characters');
    assert.equal(R.cjkBreak(HAN, 0x3002), false, 'kinsoku also holds for cjkBreak');
});

check('ranges: wordCount and width', () => {
    const R = SM.ranges;
    assert.equal(R.wordCount(text(HAN, ZI, HAN, ZI)), 2, '4 Han: ceil(4/2)');
    assert.equal(R.wordCount(text(HAN, ZI, HAN)), 2, '3 Han: ceil(3/2)');
    assert.equal(R.wordCount('two words'), 2, 'each run of other characters counts 1');
    assert.equal(R.wordCount('abc' + text(HAN, ZI)), 2, 'abc is one run, the Han pair is ceil(2/2)');
    assert.equal(R.wordCount(text(HAN, ZI, 0x3000, HAN, ZI)), 2, 'U+3000 is whitespace, not a CJK word character');
    assert.equal(R.wordCount(text(0x20, 0xA0)), 0, 'whitespace alone');
    assert.equal(R.width('a' + text(HAN)), 3, '1 + 2');
    assert.equal(R.width(text(0x20000)), 2, 'a wide character outside the BMP counts once');
    assert.equal(R.width(text(0x1F600)), 1, 'not in the wide list');
});

check('ranges: dirOf takes the first strong character', () => {
    const R = SM.ranges, arabic = text(0x645, 0x631, 0x62D, 0x628, 0x627), hebrew = text(0x5E9, 0x5DC, 0x5D5, 0x5DD);
    assert.equal(R.dirOf(arabic), 'rtl');
    assert.equal(R.dirOf(hebrew), 'rtl');
    assert.equal(R.dirOf('123 ' + arabic), 'rtl', 'digits are not strong');
    assert.equal(R.dirOf('Hi ' + arabic), 'ltr');
    assert.equal(R.dirOf('123'), 'ltr', 'no strong character');
    assert.equal(R.dirOf(''), 'ltr');
});

check('ranges: the sets of frozen section 4.2', () => {
    const R = SM.ranges;
    assert.deepEqual(Object.keys(R).sort(), ['canBreakAfter', 'cjkBreak', 'dirOf', 'isCjkWord', 'isDefaultIgnorable', 'isMark', 'isSpace', 'isWide', 'width', 'wordCount']);
    for (const cp of [0x9, 0xD, 0x20, 0xA0, 0x1680, 0x2000, 0x200A, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF]) assert.equal(R.isSpace(cp), true, cp.toString(16));
    assert.equal(R.isSpace(0x200B), false, 'U+200B is not JS whitespace');
    assert.equal(R.isMark(0x300), true);
    assert.equal(R.isMark(VIRAMA), true);
    assert.equal(R.isMark(0x41), false);
    assert.equal(R.isDefaultIgnorable(0x200C), true);
    assert.equal(R.isDefaultIgnorable(0x200D), true);
    assert.equal(R.isDefaultIgnorable(0x41), false);
    assert.equal(R.isWide(0xAC00), true, 'Hangul is wide');
    assert.equal(R.isCjkWord(0xAC00), false, 'but not a CJK word character');
    assert.equal(R.isCjkWord(0x3042), true, 'kana');
});

check('outputs: both name Unicode 16.0.0, and the Go tables hold the same code points as the JS functions', () => {
    assert.match(read(JS_OUT).split('\n', 2).join('\n'), /Unicode 16\.0\.0/);
    const go = read(GO_OUT);
    assert.match(go.split('\n', 1)[0], /Unicode 16\.0\.0/);
    for (const [name, fn] of [['ucdEastAsianWide', SM.ranges.isWide], ['ucdCJKWord', SM.ranges.isCjkWord], ['ucdDefaultIgnorable', SM.ranges.isDefaultIgnorable]]) {
        const table = goTable(go, name), inGo = new Uint8Array(0x110000);
        assert.ok(table.length > 0, name + ' has ranges');
        for (const [lo, hi] of table) inGo.fill(1, lo, hi + 1);
        for (let cp = 0; cp < 0x110000; cp++) {
            if (fn(cp) !== (inGo[cp] === 1)) assert.fail(name + ' differs from the JS table at ' + cp.toString(16));
        }
    }
});

check('generator: --check accepts its own output and refuses one changed byte in either file', () => {
    const run = (...args) => spawnSync(process.execPath, [GENERATOR, ...args], { encoding: 'utf8' });
    assert.equal(run('--check').status, 0, 'the committed outputs match the data file');
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'sm-ranges-'));
    try {
        assert.equal(run('--out', dir).status, 0);
        assert.equal(run('--check', '--out', dir).status, 0);
        for (const rel of [JS_OUT, GO_OUT]) {
            const file = path.join(dir, rel), good = fs.readFileSync(file), bad = Buffer.from(good);
            bad[bad.length >> 1] ^= 1;
            fs.writeFileSync(file, bad);
            assert.equal(run('--check', '--out', dir).status, 1, rel + ' with one changed byte');
            fs.writeFileSync(file, good);
            assert.equal(run('--check', '--out', dir).status, 0, rel + ' restored');
        }
    } finally {
        fs.rmSync(dir, { recursive: true, force: true });
    }
});
```

- [ ] **Step 2: Run it**

```powershell
npm run test:storymaker
```

Expected: exit 1, and only the checks of `scripts/storymaker-tests/12-ranges.mjs` print `FAIL`. The `ranges:` checks fail with `Cannot read properties of undefined`, because `SM.ranges` does not exist. The `outputs:` check fails with `ENOENT`, and the `generator:` check fails because the script does not exist.

- [ ] **Step 3: Implement**

The three outputs are generated. Never paste them, and never edit them by hand. A wrong output means a fix in the generator.

Create `scripts/build-storymaker-ranges.mjs`:

```js
// Generates the Storymaker code point tables from one committed data file, so Chrome and Node ICU never decide at run
// time. One source, two outputs: ui/js/storymaker/ranges.js (StorymakerCore.ranges) and internal/storymaker/ranges_gen.go.
//
//   node scripts/build-storymaker-ranges.mjs                  write both outputs
//   node scripts/build-storymaker-ranges.mjs --check          exit 1 when an output differs (CRLF checkouts compare as LF)
//   node scripts/build-storymaker-ranges.mjs --out <dir>      use <dir> as the root of the outputs (the tests do)
//   node scripts/build-storymaker-ranges.mjs --extract <dir>  rewrite the data file from the three Unicode files in <dir>
//
// No regex property escapes here: ECMAScript has no Bidi_Class, and category escapes would tie --check to the Node version.
import fs from 'node:fs';
import path from 'node:path';

const VERSION = '16.0.0';
const repo = path.resolve(import.meta.dirname, '..');
const DATA_FILE = path.join(repo, 'scripts', 'storymaker-ucd-' + VERSION + '.txt');
const JS_OUT = 'ui/js/storymaker/ranges.js';
const GO_OUT = 'internal/storymaker/ranges_gen.go';

// The JS whitespace set. Go repeats it by hand in IsJSSpace, and the shared narration fixture compares the two.
const SPACE = [[0x9, 0xD], [0x20, 0x20], [0xA0, 0xA0], [0x1680, 0x1680], [0x2000, 0x200A], [0x2028, 0x2029], [0x202F, 0x202F], [0x205F, 0x205F], [0x3000, 0x3000], [0xFEFF, 0xFEFF]];
const EAST_ASIAN_WIDE = [[0x1100, 0x115F], [0x2E80, 0x303E], [0x3041, 0x33FF], [0x3400, 0x4DBF], [0x4E00, 0x9FFF], [0xA000, 0xA4CF], [0xAC00, 0xD7A3], [0xF900, 0xFAFF], [0xFE30, 0xFE6F], [0xFF00, 0xFF60], [0xFFE0, 0xFFE6], [0x20000, 0x3FFFD]];
const CJK_WORD = [[0x3000, 0x30FF], [0x3400, 0x4DBF], [0x4E00, 0x9FFF], [0xF900, 0xFAFF], [0xFF00, 0xFF65], [0x20000, 0x2FFFF]];
// Kinsoku. No line start: ideographic full stop and comma, fullwidth comma, full stop, exclamation and question mark,
// fullwidth right parenthesis, right corner, white corner, black lenticular, angle and double angle brackets, fullwidth
// colon and semicolon, katakana middle dot, prolonged sound mark, iteration mark, horizontal ellipsis, two dot leader.
const NO_LINE_START = [0x3002, 0x3001, 0xFF0C, 0xFF0E, 0xFF01, 0xFF1F, 0xFF09, 0x300D, 0x300F, 0x3011, 0x3009, 0x300B, 0xFF1A, 0xFF1B, 0x30FB, 0x30FC, 0x3005, 0x2026, 0x2025];
// No line end: the left forms of corner, white corner, fullwidth parenthesis, black lenticular, angle and double angle brackets.
const NO_LINE_END = [0x300C, 0x300E, 0xFF08, 0x3010, 0x3008, 0x300A];

const JS_LOGIC = `
    // inTable(table, cp): table is a flat sorted list lo, hi, lo, hi, ... of inclusive ranges.
    function inTable(table, cp) {
        var lo = 0, hi = table.length / 2 - 1;
        while (lo <= hi) {
            var mid = (lo + hi) >> 1;
            if (cp < table[2 * mid]) hi = mid - 1;
            else if (cp > table[2 * mid + 1]) lo = mid + 1;
            else return true;
        }
        return false;
    }

    function isSpace(cp) { return inTable(SPACE, cp); }
    function isWide(cp) { return inTable(EAST_ASIAN_WIDE, cp); }
    function isCjkWord(cp) { return inTable(CJK_WORD, cp); }
    function isMark(cp) { return inTable(MARK, cp); }
    function isDefaultIgnorable(cp) { return inTable(DEFAULT_IGNORABLE, cp); }

    // width(text): same as Go TextWidth. A wide code point counts 2, any other 1.
    function width(text) {
        var n = 0;
        for (var i = 0; i < text.length; i++) {
            var cp = text.codePointAt(i);
            if (cp > 0xFFFF) i++;
            n += isWide(cp) ? 2 : 1;
        }
        return n;
    }

    // wordCount(text): same as Go WordCount. Whitespace separates; a run of n CJK word characters counts ceil(n/2); a run of
    // anything else counts 1.
    function wordCount(text) {
        var words = 0, cjk = 0, other = false;
        for (var i = 0; i < text.length; i++) {
            var cp = text.codePointAt(i);
            if (cp > 0xFFFF) i++;
            if (isSpace(cp)) {
                words += Math.ceil(cjk / 2);
                cjk = 0;
                other = false;
            } else if (isCjkWord(cp)) {
                cjk++;
                other = false;
            } else {
                words += Math.ceil(cjk / 2);
                cjk = 0;
                if (!other) words++;
                other = true;
            }
        }
        return words + Math.ceil(cjk / 2);
    }

    // dirOf(text): the first strong character decides; no strong character is ltr.
    function dirOf(text) {
        for (var i = 0; i < text.length; i++) {
            var cp = text.codePointAt(i);
            if (cp > 0xFFFF) i++;
            if (inTable(STRONG_RTL, cp)) return 'rtl';
            if (inTable(STRONG_LTR, cp)) return 'ltr';
        }
        return 'ltr';
    }

    // canBreakAfter(prevCp, cp): false inside a surrogate pair, before a mark, after U+200D or U+094D, and by kinsoku.
    function canBreakAfter(prevCp, cp) {
        if (prevCp >= 0xD800 && prevCp <= 0xDBFF && cp >= 0xDC00 && cp <= 0xDFFF) return false;
        if (isMark(cp) || prevCp === 0x200D || prevCp === 0x094D) return false;
        return !inTable(NO_LINE_START, cp) && !inTable(NO_LINE_END, prevCp);
    }

    function cjkBreak(prevCp, cp) { return isCjkWord(prevCp) && isCjkWord(cp) && canBreakAfter(prevCp, cp); }

    SM.ranges = {
        isSpace: isSpace, isWide: isWide, isCjkWord: isCjkWord, isMark: isMark, isDefaultIgnorable: isDefaultIgnorable,
        width: width, wordCount: wordCount, dirOf: dirOf, canBreakAfter: canBreakAfter, cjkBreak: cjkBreak
    };
`;

const hex = n => n.toString(16).toUpperCase();

function merge(ranges) {
    const out = [];
    for (const [lo, hi] of ranges.slice().sort((a, b) => a[0] - b[0])) {
        const last = out[out.length - 1];
        if (last && lo <= last[1] + 1) last[1] = Math.max(last[1], hi);
        else out.push([lo, hi]);
    }
    return out;
}

const singles = cps => merge(cps.map(cp => [cp, cp]));

// wrapItems(items, indent, width): items joined by one space, a new line (starting with indent) before width is passed.
function wrapItems(items, indent, width) {
    const lines = [];
    let line = indent;
    for (const item of items) {
        if (line.length + item.length > width && line !== indent) {
            lines.push(line.trimEnd());
            line = indent;
        }
        line += item + ' ';
    }
    if (line !== indent) lines.push(line.trimEnd());
    return lines;
}

// extract(dir): reads DerivedGeneralCategory.txt, DerivedCoreProperties.txt and DerivedBidiClass.txt and rewrites the data file.
function extract(dir) {
    const rows = name => {
        const text = fs.readFileSync(path.join(dir, name), 'utf8');
        const first = text.slice(0, text.indexOf('\n'));
        if (!first.includes(name.replace('.txt', '-' + VERSION + '.txt'))) throw new Error(name + ' is not the Unicode ' + VERSION + ' file: ' + first);
        return text.split(/\r?\n/).map(l => l.split('#')[0].trim()).filter(Boolean).map(l => l.split(';').map(f => f.trim()));
    };
    const range = spec => {
        const [a, b] = spec.split('..');
        return [parseInt(a, 16), parseInt(b || a, 16)];
    };
    const pick = (name, test) => merge(rows(name).filter(test).map(r => range(r[0])));
    const packed = (key, ranges) => wrapItems(ranges.map(([lo, hi]) => (lo === hi ? hex(lo) : hex(lo) + '-' + hex(hi))), key + ' ', 110);
    const text = [
        '# Storymaker code point data. Written once by scripts/build-storymaker-ranges.mjs --extract from the Unicode ' + VERSION,
        '# files DerivedGeneralCategory.txt, DerivedCoreProperties.txt and extracted/DerivedBidiClass.txt. Hex, inclusive.',
        '# M: general category Mn, Mc, Me. DI: Default_Ignorable_Code_Point. R, AL, L: Bidi_Class.',
        'version ' + VERSION,
        ...packed('M', pick('DerivedGeneralCategory.txt', r => r[1] === 'Mn' || r[1] === 'Mc' || r[1] === 'Me')),
        ...packed('DI', pick('DerivedCoreProperties.txt', r => r[1] === 'Default_Ignorable_Code_Point')),
        ...packed('R', pick('DerivedBidiClass.txt', r => r[1] === 'R')),
        ...packed('AL', pick('DerivedBidiClass.txt', r => r[1] === 'AL')),
        ...packed('L', pick('DerivedBidiClass.txt', r => r[1] === 'L')),
        ''
    ].join('\n');
    fs.writeFileSync(DATA_FILE, text);
    console.log('Wrote ' + path.relative(repo, DATA_FILE));
}

function parseData(text) {
    const tables = { M: [], DI: [], R: [], AL: [], L: [] };
    let version = '';
    for (const raw of text.split(/\r?\n/)) {
        if (!raw || raw.startsWith('#')) continue;
        const parts = raw.split(' ');
        if (parts[0] === 'version') {
            version = parts[1];
            continue;
        }
        const table = tables[parts[0]];
        if (!table) throw new Error('Unknown table in the data file: ' + parts[0]);
        for (const item of parts.slice(1)) {
            const [a, b] = item.split('-');
            table.push([parseInt(a, 16), parseInt(b || a, 16)]);
        }
    }
    if (version !== VERSION) throw new Error('The data file has version ' + version + ', the generator wants ' + VERSION);
    for (const [name, table] of Object.entries(tables)) {
        if (!table.length) throw new Error('The data file has no ' + name + ' table');
    }
    return tables;
}

function jsTable(name, ranges) {
    const items = ranges.map(([lo, hi]) => '0x' + hex(lo) + ',0x' + hex(hi) + ',');
    return ['    var ' + name + ' = [', ...wrapItems(items, '        ', 118), '    ];'];
}

function renderJS(data) {
    return [
        '// Storymaker player: code point tables and the text rules on them. Generated by scripts/build-storymaker-ranges.mjs from',
        '// scripts/storymaker-ucd-' + VERSION + '.txt (Unicode ' + VERSION + '). Do not edit. Classic script; publishes StorymakerCore.ranges.',
        '(function (root) {',
        "    'use strict';",
        '    var SM = root.StorymakerCore || (root.StorymakerCore = {});',
        '',
        ...jsTable('SPACE', SPACE),
        ...jsTable('EAST_ASIAN_WIDE', EAST_ASIAN_WIDE),
        ...jsTable('CJK_WORD', CJK_WORD),
        ...jsTable('MARK', data.M),
        ...jsTable('DEFAULT_IGNORABLE', data.DI),
        ...jsTable('STRONG_RTL', merge([...data.R, ...data.AL])),
        ...jsTable('STRONG_LTR', data.L),
        ...jsTable('NO_LINE_START', singles(NO_LINE_START)),
        ...jsTable('NO_LINE_END', singles(NO_LINE_END)),
        '',
        JS_LOGIC.replace(/^\n+|\n+$/g, ''),
        "})(typeof globalThis !== 'undefined' ? globalThis : self);",
        ''
    ].join('\n');
}

function goTable(name, doc, ranges) {
    const items = ranges.map(([lo, hi]) => '{0x' + hex(lo) + ', 0x' + hex(hi) + '},');
    return ['// ' + doc, 'var ' + name + ' = []ucdRange{', ...wrapItems(items, '\t', 100), '}', ''];
}

function renderGo(data) {
    return [
        '// Code generated by scripts/build-storymaker-ranges.mjs from scripts/storymaker-ucd-' + VERSION + '.txt (Unicode ' + VERSION + '). DO NOT EDIT.',
        '',
        'package storymaker',
        '',
        '// ucdVersion is the Unicode version of the data behind these tables.',
        'const ucdVersion = "' + VERSION + '"',
        '',
        '// ucdRange is an inclusive code point range. A table is sorted and its ranges do not overlap.',
        'type ucdRange struct{ lo, hi rune }',
        '',
        ...goTable('ucdEastAsianWide', 'ucdEastAsianWide is EAST_ASIAN_WIDE: a code point here counts 2 in TextWidth.', EAST_ASIAN_WIDE),
        ...goTable('ucdCJKWord', 'ucdCJKWord is CJK_WORD: a run of n of these counts ceil(n/2) in WordCount.', CJK_WORD),
        ...goTable('ucdDefaultIgnorable', 'ucdDefaultIgnorable is Default_Ignorable_Code_Point.', data.DI),
        '// inUCDTable reports whether r lies in one of the ranges of table.',
        'func inUCDTable(table []ucdRange, r rune) bool {',
        '\tlo, hi := 0, len(table)-1',
        '\tfor lo <= hi {',
        '\t\tmid := (lo + hi) / 2',
        '\t\tswitch {',
        '\t\tcase r < table[mid].lo:',
        '\t\t\thi = mid - 1',
        '\t\tcase r > table[mid].hi:',
        '\t\t\tlo = mid + 1',
        '\t\tdefault:',
        '\t\t\treturn true',
        '\t\t}',
        '\t}',
        '\treturn false',
        '}',
        ''
    ].join('\n');
}

const args = process.argv.slice(2);
const valueOf = flag => args[args.indexOf(flag) + 1];

if (args.includes('--extract')) {
    extract(path.resolve(valueOf('--extract')));
} else {
    const outRoot = args.includes('--out') ? path.resolve(valueOf('--out')) : repo;
    const data = parseData(fs.readFileSync(DATA_FILE, 'utf8'));
    const outputs = [[JS_OUT, renderJS(data)], [GO_OUT, renderGo(data)]];
    if (args.includes('--check')) {
        const stale = outputs.filter(([rel, text]) => {
            const file = path.join(outRoot, rel);
            return !fs.existsSync(file) || fs.readFileSync(file, 'utf8').replace(/\r\n/g, '\n') !== text;
        });
        for (const [rel] of stale) console.error('Outdated ' + rel + '; run node scripts/build-storymaker-ranges.mjs');
        if (stale.length) process.exit(1);
        console.log('storymaker ranges (Unicode ' + VERSION + ') are up to date');
    } else {
        for (const [rel, text] of outputs) {
            const file = path.join(outRoot, rel);
            fs.mkdirSync(path.dirname(file), { recursive: true });
            fs.writeFileSync(file, text);
            console.log('Built ' + rel);
        }
    }
}
```

Fetch the three Unicode files and write the data file. This is a one-time development step, and nothing fetches at run time. If a download fails, stop and report it. Do not build the data from Node's ICU.

```powershell
$ucd = Join-Path $env:TEMP ('storymaker-ucd-' + [guid]::NewGuid())
New-Item -ItemType Directory $ucd | Out-Null
$base = 'https://www.unicode.org/Public/16.0.0/ucd'
Invoke-WebRequest "$base/extracted/DerivedGeneralCategory.txt" -OutFile "$ucd\DerivedGeneralCategory.txt"
Invoke-WebRequest "$base/DerivedCoreProperties.txt" -OutFile "$ucd\DerivedCoreProperties.txt"
Invoke-WebRequest "$base/extracted/DerivedBidiClass.txt" -OutFile "$ucd\DerivedBidiClass.txt"
node scripts/build-storymaker-ranges.mjs --extract $ucd
Remove-Item -LiteralPath $ucd -Recurse -Force
node scripts/build-storymaker-ranges.mjs
```

Expected: `Wrote scripts...storymaker-ucd-16.0.0.txt`, then `Built ui/js/storymaker/ranges.js` and `Built internal/storymaker/ranges_gen.go`. `--extract` throws if a file's first line does not name version 16.0.0.

In `scripts/storymaker-bundle-parts.js`, replace

```js
  'ui/js/storymaker/timeline.js',
```

with

```js
  'ui/js/storymaker/ranges.js',
  'ui/js/storymaker/timeline.js',
```

In `package.json`, replace

```json
    "check:storymaker-catalog": "node scripts/build-storymaker-catalog.mjs --check"
```

with

```json
    "check:storymaker-catalog": "node scripts/build-storymaker-catalog.mjs --check",
    "build:storymaker-ranges": "node scripts/build-storymaker-ranges.mjs",
    "check:storymaker-ranges": "node scripts/build-storymaker-ranges.mjs --check"
```

- [ ] **Step 4: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
node scripts/build-storymaker-ranges.mjs --check
npm run build:ui
npm run check:ui
npm run test:storymaker
go build ./internal/storymaker/...
go vet ./internal/storymaker
gofmt -l internal/storymaker
go test ./ui -run '^TestStorymakerSourcesAreSafeAndSmall$' -count=1
```

Expected:
- `--check` says the ranges are up to date.
- `npm run check:ui` passes.
- `npm run test:storymaker` ends with `0 failed`. Its static guard also holds `ui/js/storymaker/ranges.js` under 600 lines with a comment on the first line.
- The Go lines pass, and `gofmt -l` prints nothing. If it names `internal/storymaker/ranges_gen.go`, fix `renderGo` in the generator and regenerate.

- [ ] **Step 5: Commit**

Run `detect_changes` per outline section 5. A PARTIAL answer on the new files needs a grep confirmation in the commit body, above the trailer.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add scripts/build-storymaker-ranges.mjs scripts/storymaker-ucd-16.0.0.txt ui/js/storymaker/ranges.js internal/storymaker/ranges_gen.go scripts/storymaker-bundle-parts.js package.json scripts/storymaker-tests/12-ranges.mjs ui/js/storymaker/player.bundle.js
git commit -m "feat(storymaker): generated Unicode range tables for player and Go" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task M4: `seed.js`, code scenes delegate to it
**Batch:** 2A  **Depends on:** none
**Files:**
- Create: `ui/js/storymaker/seed.js`
- Modify: `ui/js/storymaker/codescene.js`
- Modify: `scripts/storymaker-bundle-parts.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated by `npm run build:ui`)
- Test: `scripts/storymaker-tests/06-codescene.mjs`

- [ ] **Step 1: Impact**

From the main checkout:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "hashSeed" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "random" --direction upstream --repo .
```

Report callers, processes and risk. Warn on HIGH or CRITICAL, and treat UNKNOWN as unresolved with a text search, as the GitNexus rule in outline section 5 says.

- [ ] **Step 2: Write the failing test**

In `scripts/storymaker-tests/06-codescene.mjs`, replace

```js
check('code scene frames: the onFrame hook draws through a restricted facade', () => {
```

with

```js
check('seed: the generator moved to SM.seed without changing a code scene, and takeIndex never repeats the last take', () => {
    // The generator exactly as it stood in codescene.js before the move. It stays here as the reference.
    const before = seed => {
        const s = String(seed === undefined ? 0 : seed);
        let h = 2166136261;
        for (let i = 0; i < s.length; i++) { h ^= s.charCodeAt(i); h = Math.imul(h, 16777619); }
        let a = h >>> 0;
        return () => {
            a = (a + 0x6D2B79F5) | 0;
            let x = Math.imul(a ^ (a >>> 15), 1 | a);
            x = (x + Math.imul(x ^ (x >>> 7), 61 | x)) ^ x;
            return ((x ^ (x >>> 14)) >>> 0) / 4294967296;
        };
    };
    for (const seed of [undefined, 0, 42, '42', 's1:0:sfx.pop', 'x'.repeat(40)]) {
        const ref = before(seed), viaSeed = SM.seed.random(seed), viaScene = SM.codescene.random(seed);
        for (let i = 0; i < 16; i++) {
            const want = ref();
            assert.equal(viaSeed(), want);
            assert.equal(viaScene(), want);
        }
    }
    // Published FNV-1a 32-bit test vectors.
    assert.equal(SM.seed.hash(''), 0x811C9DC5);
    assert.equal(SM.seed.hash('a'), 0xE40C292C);
    // A scene that draws from api.random still gets the same picture: x is percent / 100 * 1920.
    const { build } = runScene(`export default function scene(api) {
      const box = api.shape('rect', { at: { x: 50, y: 50 }, w: 200, h: 100 });
      api.tween(box, { x: api.random(7)() * 100 }, { start: 0, dur: 1, ease: 'linear' });
    }`, 'seeded');
    near(build().entities[0].x.at(1), before(7)() * 1920, 1e-6, 'x from api.random');
    // takeIndex: the rule of frozen section 4.2, as a property over many keys.
    for (const count of [2, 3, 4]) {
        for (let key = 0; key < 50; key++) {
            const fresh = SM.seed.takeIndex('s1', 3, 'sfx.pop', key, count, -1);
            assert.ok(fresh >= 0 && fresh < count, 'in range');
            assert.equal(SM.seed.takeIndex('s1', 3, 'sfx.pop', key, count, fresh), (fresh + 1) % count, 'never the last take');
        }
    }
    assert.equal(SM.seed.takeIndex('s1', 3, 'sfx.pop', 0, 1, 0), 0, 'a sound with one take repeats it');
});

check('code scene frames: the onFrame hook draws through a restricted facade', () => {
```

- [ ] **Step 3: Run it**

```powershell
npm run test:storymaker
```

Expected: exit 1, and the new `seed:` check is the only `FAIL`, with `Cannot read properties of undefined (reading 'random')`, because `SM.seed` does not exist.

- [ ] **Step 4: Implement**

Create `ui/js/storymaker/seed.js`:

```js
// Storymaker player: seeded numbers. FNV-1a over UTF-16 units, mulberry32 and the rule that picks a sound take. Every seed is
// a string built from ids, so no compile input needs a project seed. Classic script; publishes StorymakerCore.seed.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    // hash(str): FNV-1a over UTF-16 units. Any value is turned into a string first; undefined hashes like 0.
    function hash(str) {
        var s = String(str === undefined ? 0 : str), h = 2166136261;
        for (var i = 0; i < s.length; i++) { h ^= s.charCodeAt(i); h = Math.imul(h, 16777619); }
        return h >>> 0;
    }

    // random(seedStr): mulberry32; the returned function gives values in [0, 1).
    function random(seedStr) {
        var a = hash(seedStr);
        return function () {
            a = (a + 0x6D2B79F5) | 0;
            var t = Math.imul(a ^ (a >>> 15), 1 | a);
            t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }

    // takeIndex(sceneId, beatIndex, soundId, key, count, last): the take of one sound event; never last while count > 1.
    function takeIndex(sceneId, beatIndex, soundId, key, count, last) {
        var i = hash(sceneId + ':' + beatIndex + ':' + soundId + ':' + key) % count;
        if (count > 1 && i === last) i = (i + 1) % count;
        return i;
    }

    SM.seed = { hash: hash, random: random, takeIndex: takeIndex };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

In `ui/js/storymaker/codescene.js`, replace these lines. The file may check out with CRLF, so match the lines and keep the file's endings.

```js
    function hashSeed(seed) {
        var s = String(seed === undefined ? 0 : seed), h = 2166136261;
        for (var i = 0; i < s.length; i++) { h ^= s.charCodeAt(i); h = Math.imul(h, 16777619); }
        return h >>> 0;
    }

    // random(seed): mulberry32, the only randomness a scene has.
    function random(seed) {
        var a = hashSeed(seed);
        return function () {
            a = (a + 0x6D2B79F5) | 0;
            var t = Math.imul(a ^ (a >>> 15), 1 | a);
            t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }
```

with

```js
    // random(seed): mulberry32, the only randomness a scene has. SM.seed owns the generator.
    function random(seed) { return SM.seed.random(seed); }
```

In `scripts/storymaker-bundle-parts.js`, as Task M3 left it, replace

```js
  'ui/js/storymaker/ranges.js',
```

with

```js
  'ui/js/storymaker/seed.js',
  'ui/js/storymaker/ranges.js',
```

- [ ] **Step 5: Run**

```powershell
npm run build:ui
npm run check:ui
npm run test:storymaker
```

Expected: `npm run check:ui` passes, and `npm run test:storymaker` ends with `0 failed`.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5. A PARTIAL answer on the new file needs a grep confirmation in the commit body, above the trailer.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/storymaker/seed.js ui/js/storymaker/codescene.js scripts/storymaker-bundle-parts.js scripts/storymaker-tests/06-codescene.mjs ui/js/storymaker/player.bundle.js
git commit -m "refactor(storymaker): seed module, code scenes delegate to it" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task M5: Internal eases, log zoom, eased pose blend
**Batch:** 2A  **Depends on:** none
**Files:**
- Modify: `ui/js/storymaker/ease.js`
- Modify: `ui/js/storymaker/timeline.js`
- Modify: `ui/js/storymaker/camera.js`
- Modify: `ui/js/storymaker/player.bundle.js` (generated by `npm run build:ui`)
- Test: `scripts/storymaker-tests/01-ease-timeline.mjs`
- Test: `scripts/storymaker-tests/02-stage-parts-text.mjs`

- [ ] **Step 1: Impact**

From the main checkout:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "isEase" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "evalSeg" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "Track" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "blendAt" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "addZoom" --direction upstream --repo .
```

Report callers, processes and risk. Warn on HIGH or CRITICAL, and treat UNKNOWN as unresolved with a text search, as the GitNexus rule in outline section 5 says. These are prototype methods and plain-object calls, so expect UNKNOWN.

- [ ] **Step 2: Write the failing tests**

In `scripts/storymaker-tests/01-ease-timeline.mjs`, replace

```js
check('tracks evaluate segments, overlapping segments and step tracks', () => {
```

with

```js
check('internal curves: quart-out, expo-out and spring exist for compile and stay out of the public names', () => {
    for (const name of ['quart-out', 'expo-out', 'spring']) {
        assert.equal(SM.isEase(name), false, name + ' is not a public ease');
        near(SM.ease(name, 0), 0, 1e-12, name + '(0)');
        assert.equal(SM.ease(name, 1), 1, name + '(1)');
    }
    for (const name of plain(SM.easeNames)) assert.equal(SM.isEase(name), true, name);
    near(SM.ease('quart-out', 0.5), 0.9375, 1e-12, '1 - 0.5^4');
    near(SM.ease('expo-out', 0.1), 0.5, 1e-12, '1 - 2^-1');
    // spring first crosses 1 where cos(wd*u) + (zeta*w0/wd)*sin(wd*u) = 0, which frozen 4.5 gives as u = 0.4812
    assert.ok(SM.ease('spring', 0.48) < 1 && SM.ease('spring', 0.49) > 1, 'spring crosses 1 at 0.4812');
});

check('tracks evaluate segments, overlapping segments and step tracks', () => {
```

and replace

```js
    assert.equal(blend.a, 'c'); assert.equal(blend.b, 'b'); near(blend.u, 0.25, 1e-9);
```

with

```js
    // the step began at t=2 and lasts 1 s, so the raw u is 0.25; ease-in-out gives 4 * 0.25^3 = 0.0625
    assert.equal(blend.a, 'c'); assert.equal(blend.b, 'b'); near(blend.u, 0.0625, 1e-9);
```

In `scripts/storymaker-tests/02-stage-parts-text.mjs`, replace

```js
    near(mid.zoom, 1.5, 1e-9); near(mid.cx, 1080, 1e-9); near(mid.cy, 570, 1e-9);
```

with

```js
    // zoom runs in log space: 1 * (2 / 1)^0.5. The pan stays linear.
    near(mid.zoom, Math.SQRT2, 1e-9); near(mid.cx, 1080, 1e-9); near(mid.cy, 570, 1e-9);
```

and replace

```js
    near(SM.camera.end(cam), 4, 1e-9);
```

with

```js
    near(SM.camera.end(cam), 4, 1e-9);
    const four = SM.camera.create(1, 960, 540);
    SM.camera.addZoom(four, 0, 1, 4, 'linear');
    near(SM.camera.at(four, 0.5).zoom, 2, 1e-9, 'log zoom: 1 * (4 / 1)^0.5');
```

- [ ] **Step 3: Run it**

```powershell
npm run test:storymaker
```

Expected: exit 1, with three `FAIL` lines.
- `internal curves` fails because `SM.ease('quart-out', 0.5)` is still linear, or on the `isEase` loop.
- `tracks evaluate` fails on `blend.u` with `0.25 vs 0.0625`.
- `camera: framings` fails on `mid.zoom` with `1.5 vs 1.414...`.

- [ ] **Step 4: Implement**

In `ui/js/storymaker/ease.js`, replace

```js
    var FNS = {
```

with

```js
    // spring(u): damped oscillator, damping 0.75, w0 7.6. It overshoots 1 once and ends on exactly 1.
    var SPRING_ZETA = 0.75, SPRING_W0 = 7.6, SPRING_WD = SPRING_W0 * Math.sqrt(1 - SPRING_ZETA * SPRING_ZETA);
    function spring(u) {
        if (u >= 1) return 1;
        var a = SPRING_ZETA * SPRING_W0;
        return 1 - Math.exp(-a * u) * (Math.cos(SPRING_WD * u) + (a / SPRING_WD) * Math.sin(SPRING_WD * u));
    }

    var FNS = {
```

replace

```js
        'bounce-out': bounceOut
```

with

```js
        'bounce-out': bounceOut,
        // Internal curves: compile picks them by role, and a story or a code scene cannot name them (isEase below).
        'quart-out': function (t) { var u = 1 - t; return 1 - u * u * u * u; },
        'expo-out': function (t) { return t >= 1 ? 1 : 1 - Math.pow(2, -10 * t); },
        'spring': spring
```

and replace

```js
    function isEase(name) { return Object.prototype.hasOwnProperty.call(FNS, name); }
```

with

```js
    // isEase(name): only the six public names.
    function isEase(name) { return NAMES.indexOf(name) >= 0; }
```

In `ui/js/storymaker/timeline.js`, replace

```js
        return lerp(seg.from, seg.to, SM.ease(seg.ease, (t - seg.t0) / (seg.t1 - seg.t0)));
```

with

```js
        var s = SM.ease(seg.ease, (t - seg.t0) / (seg.t1 - seg.t0));
        return seg.log ? seg.from * Math.pow(seg.to / seg.from, s) : lerp(seg.from, seg.to, s);
```

replace

```js
    Track.prototype.add = function (t0, t1, to, ease, from) {
```

with

```js
    // log: interpolate in log space (camera zoom), so a zoom reads at an even speed.
    Track.prototype.add = function (t0, t1, to, ease, from, log) {
```

replace

```js
        this.segs.push({ t0: t0, t1: t1 < t0 ? t0 : t1, to: to, ease: ease || 'linear', from: from, order: this.segs.length });
```

with

```js
        this.segs.push({ t0: t0, t1: t1 < t0 ? t0 : t1, to: to, ease: ease || 'linear', from: from, log: !!log, order: this.segs.length });
```

replace

```js
    // blendAt(t): for a track whose steps carry a blend duration in x: {a: previous value, b: value, u: 0..1}.
```

with

```js
    // blendAt(t): for a track whose steps carry a blend duration in x: {a: previous value, b: value, u: 0..1}, u eased by ease-in-out.
```

and replace

```js
        return u >= 1 ? { a: step.v, b: step.v, u: 1 } : { a: prev, b: step.v, u: u };
```

with

```js
        return u >= 1 ? { a: step.v, b: step.v, u: 1 } : { a: prev, b: step.v, u: SM.ease('ease-in-out', u) };
```

In `ui/js/storymaker/camera.js`, replace

```js
    function addZoom(cam, t0, t1, zoom, ease) { cam.zoom.add(t0, t1, clampZoom(zoom), ease); }
```

with

```js
    // addZoom: always in log space, code scenes included.
    function addZoom(cam, t0, t1, zoom, ease) { cam.zoom.add(t0, t1, clampZoom(zoom), ease, undefined, true); }
```

- [ ] **Step 5: Run**

```powershell
npm run build:ui
npm run check:ui
npm run test:storymaker
```

Expected: `npm run check:ui` passes, and `npm run test:storymaker` ends with `0 failed`. The firewall timing pins do not move here, because no default duration changed.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/storymaker/ease.js ui/js/storymaker/timeline.js ui/js/storymaker/camera.js scripts/storymaker-tests/01-ease-timeline.mjs scripts/storymaker-tests/02-stage-parts-text.mjs ui/js/storymaker/player.bundle.js
git commit -m "feat(storymaker): internal eases, log zoom, eased pose blend" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

<!-- G-M-2 -->

### Task M6: `motion.js`: factors, defaults, lead fractions, holds
**Batch:** 2A  **Depends on:** M3, M5
**Files:**
Create: `ui/js/storymaker/motion.js`
Modify: `scripts/storymaker-bundle-parts.js`
Modify: `ui/js/storymaker/player.bundle.js` (generated by `npm run build:ui`)
Test: `scripts/storymaker-tests/13-motion.mjs`

- [ ] **Step 1: Write the failing test**

Create `scripts/storymaker-tests/13-motion.mjs`:

```js
// Tasks M6 to M10: the motion rules and the beat scheduler.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog } = t;
const SM = t.core();

// firstReach(name, level): the first u in 0..1 where the ease reaches level, searched in steps of 0.00001.
function firstReach(name, level) {
    for (let k = 0; k <= 100000; k++) if (SM.ease(name, k / 100000) >= level) return k / 100000;
    return 1;
}

check('motion: rhythm factors, beat defaults and the overlap', () => {
    assert.deepEqual(plain(SM.motion.RHYTHM), { calm: 1.25, standard: 1, lively: 0.8 });
    assert.equal(SM.motion.factor({ rhythm: 'calm' }), 1.25);
    assert.equal(SM.motion.factor({ rhythm: 'lively' }), 0.8);
    assert.equal(SM.motion.factor({}), 1);
    assert.equal(SM.motion.factor(undefined), 1);
    assert.deepEqual(plain(SM.motion.DEFAULTS), { enter: 0.6, exit: 0.6, move: 0.8, pose: 0.3, emote: 0.25, look: 0.3, show: 0.5, hide: 0.35, camera: 0.8, shake: 0.5, wait: 1, highlight: 0.8 });
    assert.equal(SM.motion.OVERLAP, 0.4);
});

check('motion: each lead fraction matches a numeric search of its ease to 0.001', () => {
    for (const name of ['linear', 'ease-in', 'ease-out', 'ease-in-out', 'back-out', 'bounce-out', 'quart-out', 'expo-out']) {
        near(SM.motion.leadFraction(name), firstReach(name, 0.9), 0.001, name);
    }
    // spring: the first crossing of the target value 1, not the 90 % point
    near(SM.motion.leadFraction('spring'), firstReach('spring', 1), 0.001, 'spring');
    assert.equal(SM.motion.leadFraction('nope'), 0.9, 'an unknown name counts as linear');
});

check('motion: holds count words through the range tables and never shrink with rhythm', () => {
    // 4 Han characters are one CJK run: ceil(4 / 2) = 2 words, so 0.5 + 0.3 * 2 = 1.1 s
    near(SM.motion.holdSeconds(String.fromCharCode(0x6f22, 0x5b57, 0x6f22, 0x5b57)).base, 1.1, 1e-9, 'Han');
    // 4 words: 0.5 + 0.3 * 4 = 1.7 s
    near(SM.motion.holdSeconds('eins zwei drei vier').base, 1.7, 1e-9, '4 words');
    // 1 word: 0.5 + 0.3 = 0.8 s, raised to 1.2 s only for a secondary line
    near(SM.motion.holdSeconds('eins').base, 0.8, 1e-9, 'plain line');
    near(SM.motion.holdSeconds('eins', { secondary: true }).base, 1.2, 1e-9, 'secondary line');
    near(SM.motion.holdSeconds('eins zwei drei vier', { factor: 1.25 }).scaled, 1.7 * 1.25, 1e-9, 'calm');
    near(SM.motion.holdSeconds('eins zwei drei vier', { factor: 0.8 }).scaled, 1.7, 1e-9, 'lively keeps the base');
});
```

- [ ] **Step 2: Run it**

```powershell
npm run test:storymaker
```

Expected: three `FAIL motion:` lines with `TypeError: Cannot read properties of undefined`, because `SM.motion` does not exist yet. All other checks pass.

- [ ] **Step 3: Implement**

Create `ui/js/storymaker/motion.js`:

```js
// Storymaker player: motion rules. Rhythm factors, default beat durations, lead fractions and text holds. Pure placement
// helpers live here so that beats.js stays under its line budget. Classic script; publishes StorymakerCore.motion.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var RHYTHM = { calm: 1.25, standard: 1, lively: 0.8 };
    // Seconds. wait is never scaled by the rhythm, every other default is.
    var DEFAULTS = { enter: 0.6, exit: 0.6, move: 0.8, pose: 0.3, emote: 0.25, look: 0.3, show: 0.5, hide: 0.35, camera: 0.8, shake: 0.5, wait: 1, highlight: 0.8 };
    // Fraction of the duration until the value first reaches 0.9, or for spring until it first crosses 1.
    var LEAD = {
        'linear': 0.9, 'ease-in': 0.9655, 'ease-out': 0.5358, 'ease-in-out': 0.7076, 'bounce-out': 0.3450,
        'quart-out': 0.4377, 'expo-out': 0.3322, 'spring': 0.4812, 'back-out': 0.2955
    };
    var OVERLAP = 0.4;
    var SECONDARY_HOLD = 1.2;

    // factor(style): the rhythm factor of a story style. No rhythm, or an unknown one, means 1.
    function factor(style) {
        var f = style ? RHYTHM[style.rhythm] : 0;
        return f > 0 ? f : 1;
    }

    // leadFraction(easeName): see LEAD. An unknown name counts as linear, as in SM.ease.
    function leadFraction(easeName) {
        var v = LEAD[easeName];
        return v > 0 ? v : LEAD.linear;
    }

    // holdSeconds(text, opts) = {base, scaled}. opts.secondary marks a caption or label in a scene that shows a title;
    // opts.factor is the rhythm factor. The scaled hold is never shorter than the base.
    function holdSeconds(text, opts) {
        opts = opts || {};
        var base = 0.5 + 0.3 * SM.ranges.wordCount(String(text));
        if (opts.secondary && base < SECONDARY_HOLD) base = SECONDARY_HOLD;
        return { base: base, scaled: base * Math.max(1, opts.factor > 0 ? opts.factor : 1) };
    }

    SM.motion = { RHYTHM: RHYTHM, DEFAULTS: DEFAULTS, OVERLAP: OVERLAP, factor: factor, leadFraction: leadFraction, holdSeconds: holdSeconds };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

In `scripts/storymaker-bundle-parts.js`, replace this line. It occurs once, in `storymakerPlayerParts`.

Old:
```js
  'ui/js/storymaker/plan.js',
```
New:
```js
  'ui/js/storymaker/plan.js',
  'ui/js/storymaker/motion.js',
```

- [ ] **Step 4: Run**

```powershell
npm run build:ui
npm run check:ui
npm run test:storymaker
```

Expected: `check:ui` passes and `test:storymaker` ends with `0 failed`. `host.bundle.js` does not change, because `motion.js` is not a host part.

- [ ] **Step 5: Commit**

Run `detect_changes` per outline section 5:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/storymaker/motion.js scripts/storymaker-bundle-parts.js scripts/storymaker-tests/13-motion.mjs ui/js/storymaker/player.bundle.js
git commit -m "feat(storymaker): add motion rules module (rhythm, defaults, lead fractions, holds)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task M7: Role eases and scaled defaults (frozen 4.4, 4.7)
**Batch:** 2A  **Depends on:** M6
**Files:**
Modify: `ui/js/storymaker/beats.js`
Modify: `ui/js/storymaker/motion.js`
Modify: `ui/js/storymaker/player.bundle.js` (generated by `npm run build:ui`)
Test: `scripts/storymaker-tests/13-motion.mjs`

- [ ] **Step 1: Impact**

Run this from the main checkout. Report callers, processes and risk, and treat HIGH, CRITICAL and UNKNOWN as outline section 5 says.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "buildStoryboard" --direction upstream --repo .
```

The edited nested functions `easeOf`, `entryAnim` and `moveBeat` sit inside `buildStoryboard`.

- [ ] **Step 2: Write the failing test**

Append to the end of `scripts/storymaker-tests/13-motion.mjs`:

```js
// ---- role eases and scaled defaults

const CAST = [{ id: 'anna', character: 'char.office-woman-1' }];

// one(beats, extra): a single storyboard scene at T = 0 (a first scene has no entry transition), compiled with the mini catalog.
function one(beats, extra = {}) {
    const scene = { id: 's', environment: 'env.office', actors: extra.actors || [{ cast: 'anna', at: 'left' }], beats };
    if (extra.narration) scene.narration = extra.narration;
    if (extra.duration) scene.duration = extra.duration;
    const story = { cast: CAST, scenes: [scene] };
    if (extra.style) story.style = extra.style;
    const out = SM.compile(story, {}, extra.catalog || catalog);
    const built = out.timeline.scenes[0];
    return {
        out, scene: built, issues: out.report.issues, end: out.report.scenes[0].end_frame,
        anna: built.entities.find(e => e.id === 'anna'), text: built.entities.find(e => e.type === 'text'),
        of: id => built.entities.find(e => e.id === id), item: id => built.entities.find(e => e.item === id)
    };
}
// lastSeg(track): the last segment of a track, with its `from` filled in.
const lastSeg = tr => { tr.at(0); return tr.segs[tr.segs.length - 1]; };

check('role eases: every default of the table', () => {
    const enter = one([{ do: 'enter', who: 'anna', to: 'center' }]).anna;
    assert.equal(lastSeg(enter.x).ease, 'linear');
    assert.equal(lastSeg(enter.y).ease, 'linear');
    const appear = lastSeg(enter.alpha);
    assert.equal(appear.ease, 'linear');
    assert.equal(appear.t1, appear.t0, 'enter: the alpha is a step at the start');
    const exit = one([{ do: 'exit', who: 'anna', to: 'door' }]).anna;
    assert.equal(lastSeg(exit.x).ease, 'linear');
    assert.equal(lastSeg(exit.y).ease, 'linear');
    const gone = lastSeg(exit.alpha);
    assert.equal(gone.ease, 'ease-in');
    near(gone.t1 - gone.t0, 0.2, 1e-9, 'exit: the alpha runs over the last 0.2 s');
    assert.equal(lastSeg(one([{ do: 'walk', who: 'anna', to: 'center' }]).anna.x).ease, 'linear');
    assert.equal(lastSeg(one([{ do: 'move', who: 'anna', to: 'center' }]).anna.x).ease, 'quart-out');
    const show = entry => one([{ do: 'show', item: 'prop.lock', at: 'right', entry }]).item('prop.lock');
    assert.equal(lastSeg(show('fade').alpha).ease, 'quart-out');
    const slide = show('slide');
    assert.equal(lastSeg(slide.alpha).ease, 'quart-out');
    assert.equal(lastSeg(slide.x).ease, 'quart-out');
    const pop = show('pop');
    assert.equal(lastSeg(pop.alpha).ease, 'quart-out');
    assert.equal(lastSeg(pop.scale).ease, 'spring');
    assert.equal(lastSeg(pop.scale).from, 0.6);
    const popText = one([{ do: 'show', text: 'Hallo', entry: 'pop' }], { actors: [] }).text;
    assert.equal(lastSeg(popText.alpha).ease, 'quart-out');
    assert.equal(popText.scale.segs.length, 0, 'a text pop has no scale entry');
    const hidden = one([{ do: 'show', item: 'prop.lock', id: 'lock' }, { do: 'hide', target: 'lock' }], { actors: [] }).of('lock');
    assert.equal(lastSeg(hidden.alpha).ease, 'ease-in');
    const camera = beat => one([beat]).scene.camera;
    const focus = camera({ do: 'camera', op: 'focus', target: 'anna' });
    for (const tr of [focus.zoom, focus.cx, focus.cy]) assert.equal(lastSeg(tr).ease, 'expo-out');
    const zoom = camera({ do: 'camera', op: 'zoom', target: 'anna' });
    for (const tr of [zoom.zoom, zoom.cx, zoom.cy]) assert.equal(lastSeg(tr).ease, 'ease-in-out');
    const pan = camera({ do: 'camera', op: 'pan', target: 'anna' });
    for (const tr of [pan.cx, pan.cy]) assert.equal(lastSeg(tr).ease, 'ease-in-out');
    // pose: the blend is eased by ease-in-out; a quarter of the 0.3 s blend gives 4 * 0.25^3 = 0.0625
    const posed = one([{ do: 'pose', who: 'anna', pose: 'typing' }]).anna;
    near(posed.pose.blendAt(0.075).u, 0.0625, 1e-9, 'pose');
    // gesture: ease-in-out per key segment, a smoothstep in the rig: 0.25^2 * (3 - 2 * 0.25) = 0.15625
    near(SM.rig.sampleKeys([{ t: 0, bones: { a: 0 } }, { t: 1, bones: { a: 1 } }], 0.25).bones.a, 0.15625, 1e-9, 'gesture');
});

check('role eases: an explicit ease wins', () => {
    const e = 'bounce-out';
    assert.equal(lastSeg(one([{ do: 'enter', who: 'anna', to: 'center', ease: e }]).anna.x).ease, e);
    assert.equal(lastSeg(one([{ do: 'exit', who: 'anna', to: 'door', ease: e }]).anna.x).ease, e);
    assert.equal(lastSeg(one([{ do: 'walk', who: 'anna', to: 'center', ease: e }]).anna.x).ease, e);
    assert.equal(lastSeg(one([{ do: 'move', who: 'anna', to: 'center', ease: e }]).anna.x).ease, e);
    const show = entry => one([{ do: 'show', item: 'prop.lock', at: 'right', entry, ease: e }]).item('prop.lock');
    assert.equal(lastSeg(show('fade').alpha).ease, e);
    assert.equal(lastSeg(show('slide').x).ease, e);
    assert.equal(lastSeg(show('pop').scale).ease, e);
    const hidden = one([{ do: 'show', item: 'prop.lock', id: 'lock' }, { do: 'hide', target: 'lock', ease: e }], { actors: [] }).of('lock');
    assert.equal(lastSeg(hidden.alpha).ease, e);
    assert.equal(lastSeg(one([{ do: 'camera', op: 'focus', target: 'anna', ease: e }]).scene.camera.zoom).ease, e);
});

check('rhythm: defaults scale, hide takes 0.35 s, explicit values and wait do not scale', () => {
    const span = seg => seg.t1 - seg.t0;
    const lively = { rhythm: 'lively' }, calm = { rhythm: 'calm' };
    const hide = style => lastSeg(one([{ do: 'show', item: 'prop.lock', id: 'lock' }, { do: 'hide', target: 'lock' }], { actors: [], style }).of('lock').alpha);
    near(span(hide()), 0.35, 1e-9, 'hide');
    near(span(hide(lively)), 0.35 * 0.8, 1e-9, 'lively hide');
    const walk = style => one([{ do: 'walk', who: 'anna', to: 'center' }], { style }).anna.walks.list[0];
    near(span(walk(lively)), span(walk()) * 0.8, 1e-9, 'lively walk');
    near(span(lastSeg(one([{ do: 'move', who: 'anna', to: 'center', duration: 2 }], { style: lively }).anna.x)), 2, 1e-9, 'an explicit duration is not scaled');
    assert.equal(one([{ do: 'wait', seconds: 2 }], { style: calm }).end, 60, '2 s at 30 fps, never times 1.25');
    assert.equal(one([{ do: 'wait' }], { style: calm }).end, 30, 'the default wait stays 1 s; scaled it would give 38 frames');
});

check('pop: opacity never passes 1, also with an explicit overshooting ease', () => {
    const maxAlpha = e => { let m = 0; for (let k = 0; k <= 200; k++) m = Math.max(m, e.alpha.at(k / 200)); return m; };
    for (const ease of [null, 'back-out']) {
        const prop = { do: 'show', item: 'prop.lock', at: 'right', entry: 'pop' }, text = { do: 'show', text: 'Hallo', entry: 'pop' };
        if (ease) { prop.ease = ease; text.ease = ease; }
        assert.ok(maxAlpha(one([prop]).item('prop.lock')) <= 1, 'prop with ' + ease);
        assert.ok(maxAlpha(one([text], { actors: [] }).text) <= 1, 'text with ' + ease);
    }
});
```

- [ ] **Step 3: Run it**

```powershell
npm run test:storymaker
```

Expected:
- `FAIL role eases: every default of the table` with `'ease-out' !== 'linear'`.
- `FAIL role eases: an explicit ease wins`, because `entryAnim` ignores `b.ease`.
- `FAIL rhythm: defaults scale, ...`, with hide at 0.3 s.

The pop check already passes. It guards the new fade code. All other checks pass.

- [ ] **Step 4: Implement**

In `ui/js/storymaker/motion.js`, replace the export line.

Old:
```js
    SM.motion = { RHYTHM: RHYTHM, DEFAULTS: DEFAULTS, OVERLAP: OVERLAP, factor: factor, leadFraction: leadFraction, holdSeconds: holdSeconds };
```
New:
```js
    function isTextShow(b) { return b.text !== undefined && b.text !== null && b.item === undefined; }

    // defaultEase(b): the ease by role of a beat that sets no explicit ease. Walking (enter, exit, walk) is linear.
    function defaultEase(b) {
        if (b.do === 'move') return 'quart-out';
        if (b.do === 'hide') return 'ease-in';
        if (b.do === 'camera') return b.op === 'focus' ? 'expo-out' : 'ease-in-out';
        if (b.do === 'show') return b.entry === 'pop' && !isTextShow(b) ? 'spring' : 'quart-out';
        return 'linear';
    }

    // fadeEase(b): the ease of a show's opacity. Opacity never overshoots, and canvas ignores a globalAlpha above 1. So a
    // pop fades on quart-out whatever eases its scale, and an explicit back-out falls back to quart-out.
    function fadeEase(b) {
        if (b.entry === 'pop' && !isTextShow(b)) return 'quart-out';
        var ease = SM.isEase(b.ease) ? b.ease : 'quart-out';
        return ease === 'back-out' ? 'quart-out' : ease;
    }

    SM.motion = {
        RHYTHM: RHYTHM, DEFAULTS: DEFAULTS, OVERLAP: OVERLAP, factor: factor, leadFraction: leadFraction, holdSeconds: holdSeconds,
        defaultEase: defaultEase, fadeEase: fadeEase
    };
```

Apply these replacements in `ui/js/storymaker/beats.js`. The file has CRLF line endings in the checkout. Keep them. Old text that spans lines is shown with plain line breaks.

Edit 1. Old:
```js
        var placed = 0;
```
New:
```js
        var placed = 0;
        // f scales compile defaults only; explicit durations, delays, wait and cue times stay as written.
        var f = SM.motion.factor(c.story && c.story.style), DEF = SM.motion.DEFAULTS;
```

Edit 2. Old:
```js
            enter: {
                dur: function () { return 0.6; },
```
New:
```js
            enter: {
                dur: function () { return DEF.enter * f; },
```

Edit 3. Old:
```js
                    a.x.add(t0, t0 + dur, to.x, easeOf(b, 'ease-out'), from.x);
```
New:
```js
                    a.x.add(t0, t0 + dur, to.x, easeOf(b), from.x);
```

Edit 4. Old:
```js
                    a.y.add(t0, t0 + dur, to.y, easeOf(b, 'ease-out'), from.y);
```
New:
```js
                    a.y.add(t0, t0 + dur, to.y, easeOf(b), from.y);
```

Edit 5. Old:
```js
            exit: {
                dur: function () { return 0.6; },
```
New:
```js
            exit: {
                dur: function () { return DEF.exit * f; },
```

Edit 6. Old:
```js
                    moveTo(a, t0, t0 + dur, to, easeOf(b, 'ease-in'));
```
New:
```js
                    moveTo(a, t0, t0 + dur, to, easeOf(b));
```

Edit 7. Old:
```js
                    if (!e || !to) return 0.6;
```
New:
```js
                    if (!e || !to) return 0.6 * f;
```

Edit 8. Old:
```js
                    return walkSeconds({ x: e.x.at(t0), y: e.y.at(t0) }, to);
```
New:
```js
                    return walkSeconds({ x: e.x.at(t0), y: e.y.at(t0) }, to) * f;
```

Edit 9. Old:
```js
            move: {
                dur: function () { return 0.8; },
```
New:
```js
            move: {
                dur: function () { return DEF.move * f; },
```

Edit 10. Old:
```js
            pose: {
                dur: function () { return 0.3; },
```
New:
```js
            pose: {
                dur: function () { return DEF.pose * f; },
```

Edit 11. Old:
```js
                    return (entry && entry.duration > 0 ? entry.duration : 0.5) * count;
```
New:
```js
                    return (entry && entry.duration > 0 ? entry.duration : 0.5) * count * f;
```

Edit 12. Old:
```js
                dur: function () { return 0.25; },
```
New:
```js
                dur: function () { return DEF.emote * f; },
```

Edit 13. Old:
```js
            look: {
                dur: function () { return 0.3; },
```
New:
```js
            look: {
                dur: function () { return DEF.look * f; },
```

Edit 14. Old:
```js
                dur: function (b) { return (b.entry || 'fade') === 'none' ? 0 : 0.5; },
```
New:
```js
                dur: function (b) { return (b.entry || 'fade') === 'none' ? 0 : DEF.show * f; },
```

Edit 15. Old:
```js
            hide: {
                dur: function () { return 0.3; },
```
New:
```js
            hide: {
                dur: function () { return DEF.hide * f; },
```

Edit 16. Old:
```js
                    e.alpha.add(t0, t0 + dur, 0, easeOf(b, 'ease-in'));
```
New:
```js
                    e.alpha.add(t0, t0 + dur, 0, easeOf(b));
```

Edit 17. Old:
```js
                dur: function (b) { return b.op === 'shake' ? 0.5 : 0.8; },
```
New:
```js
                dur: function (b) { return (b.op === 'shake' ? DEF.shake : DEF.camera) * f; },
```

Edit 18. Old:
```js
                    var ease = easeOf(b, 'ease-in-out'), z = cam.zoom.at(t0), f = b.target ? focusOf(b.target, t0) : null;
```
New:
```js
                    var ease = easeOf(b), z = cam.zoom.at(t0), f = b.target ? focusOf(b.target, t0) : null;
```

Edit 19. Old:
```js
        function easeOf(b, fallback) { return SM.isEase(b.ease) ? b.ease : fallback; }
```
New:
```js
        function easeOf(b) { return SM.isEase(b.ease) ? b.ease : SM.motion.defaultEase(b); }
```

Edit 20. Old:
```js
            var entry = b.entry || 'fade';
            if (entry === 'none' || dur <= 0) { e.alpha.add(t0, t0, 1, 'linear'); return; }
            if (entry === 'pop' && e.type !== 'text') {
                e.alpha.add(t0, t0 + dur, 1, 'ease-out');
                e.scale.add(t0, t0 + dur, 1, 'back-out', 0.6);
            } else if (entry === 'slide') {
                e.alpha.add(t0, t0 + dur, 1, 'ease-out');
                e.x.add(t0, t0 + dur, pos.x, 'ease-out', pos.x - 140);
            } else {
                e.alpha.add(t0, t0 + dur, 1, 'ease-out');
            }
```
New:
```js
            var entry = b.entry || 'fade', fade = SM.motion.fadeEase(b);
            if (entry === 'none' || dur <= 0) { e.alpha.add(t0, t0, 1, 'linear'); return; }
            if (entry === 'pop' && e.type !== 'text') {
                e.alpha.add(t0, t0 + dur, 1, fade);
                e.scale.add(t0, t0 + dur, 1, easeOf(b), 0.6);
            } else if (entry === 'slide') {
                e.alpha.add(t0, t0 + dur, 1, fade);
                e.x.add(t0, t0 + dur, pos.x, easeOf(b), pos.x - 140);
            } else {
                e.alpha.add(t0, t0 + dur, 1, fade);
            }
```

Edit 21. Old:
```js
            var ease = easeOf(b, walking ? 'linear' : 'ease-in-out');
```
New:
```js
            var ease = easeOf(b);
```

The inner `f` in the `gesture`, `look` and `camera` `run` functions shadows the factor on purpose. Those `run` functions never read the factor, and the `dur` functions are separate closures.

- [ ] **Step 5: Run**

```powershell
npm run build:ui
npm run check:ui
npm run test:storymaker
```

Expected: `check:ui` passes and `test:storymaker` ends with `0 failed`.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/storymaker/beats.js ui/js/storymaker/motion.js scripts/storymaker-tests/13-motion.mjs ui/js/storymaker/player.bundle.js
git commit -m "feat(storymaker): role eases and rhythm-scaled beat defaults" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task M8: Scheduler: targets, floors, overlap, cursor, anchor (frozen 4.6 steps 1-4, 6b-6d, 7, 10-13)
**Batch:** 2A  **Depends on:** M7
**Files:**
Modify: `ui/js/storymaker/beats.js`
Modify: `ui/js/storymaker/player.bundle.js` (generated by `npm run build:ui`)
Test: `scripts/storymaker-tests/13-motion.mjs`

- [ ] **Step 1: Impact**

Run this from the main checkout. Report callers, processes and risk, and treat HIGH, CRITICAL and UNKNOWN as outline section 5 says.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "buildStoryboard" --direction upstream --repo .
```

- [ ] **Step 2: Write the failing test**

Append to the end of `scripts/storymaker-tests/13-motion.mjs`. The helpers `one` and `lastSeg` are already in the file. `one(beats, extra)` compiles one scene at T = 0 and returns `{out, scene, issues, end, anna, text, of(id), item(id)}`.

```js
// ---- the scheduler: targets, floors, overlap, cursor, anchor

const lock = (id, extra = {}) => ({ do: 'show', item: 'prop.lock', id, ...extra });
const startOf = e => e.alpha.segs[0].t0;
const WALK = { do: 'walk', who: 'anna', to: 'center' };

check('scheduler: a beat on another target starts at 0.4 of the anchor, a same-target beat follows, a wait breaks overlap', () => {
    // two 0.5 s shows of one item are two instances, so two targets: the second starts at 0.4 * 0.5 = 0.2 s
    const two = one([lock('a'), lock('b')], { actors: [] });
    near(startOf(two.of('a')), 0, 1e-9, 'first');
    near(startOf(two.of('b')), 0.2, 1e-9, 'overlap');
    // same target: the emote waits for the walk
    const same = one([WALK, { do: 'emote', who: 'anna', expression: 'happy' }]);
    near(same.anna.expr.steps[0].t, same.anna.walks.list[0].t1, 1e-9, 'same target');
    // a wait has no target: it starts at the cursor (0.5 s), lasts 1 s, and the next show follows it
    const gap = one([lock('a'), { do: 'wait', seconds: 1 }, lock('b')], { actors: [] });
    near(startOf(gap.of('b')), 1.5, 1e-9, 'a wait breaks overlap');
    // an unsupported beat leaves the state untouched: the overlap still reads the first show
    const skipped = one([lock('a'), { do: 'chart', chart: 'bar', data: {} }, { do: 'show', icon: 'icon.x' }, lock('b')], { actors: [] });
    near(startOf(skipped.of('b')), 0.2, 1e-9, 'unsupported beats');
});

check('scheduler: targets compare as instances, not as strings', () => {
    // one instance under two names is one target: the second move follows the first, at 0.5 + 0.8 = 1.3 s
    const named = one([lock('a'), { do: 'move', target: 'a', to: 'left' }, { do: 'move', target: 'prop.lock', to: 'center' }], { actors: [] });
    near(named.of('a').x.segs[1].t0, 1.3, 1e-9, 'one instance, two names');
    // a camera on a spot has no instance, so no target: it starts at the cursor (0.5 s) for 0.8 s, and the next show follows
    const spot = one([lock('a'), { do: 'camera', op: 'focus', target: 'board' }, lock('b')], { actors: [] });
    near(startOf(spot.of('b')), 1.3, 1e-9, 'a spot is not a target');
});

check('scheduler: a short with-prev beat leaves the anchor on the longer beat; floors and the cursor hold', () => {
    // the walk lasts W; the lock runs with it for 0.5 s and does not replace the anchor, so the shield starts at 0.4 W
    const group = one([WALK, lock('a', { with: 'prev' }), { do: 'show', item: 'prop.shield', id: 'b' }]);
    const walk = group.anna.walks.list[0], W = walk.t1 - walk.t0;
    near(startOf(group.of('a')), 0, 1e-9, 'with prev');
    near(startOf(group.of('b')), 0.4 * W, 1e-9, 'the anchor stays on the walk');
    // floor: the emote overlaps the lock by rule c, but anna is busy until W
    const floored = one([WALK, lock('a'), { do: 'emote', who: 'anna', expression: 'happy' }]);
    near(floored.anna.expr.steps[0].t, W, 1e-9, 'floor');
    // cursor after with prev: the maximum of the group, so a wait starts at W and the next show at W + 1
    const after = one([WALK, lock('a', { with: 'prev' }), { do: 'wait', seconds: 1 }, lock('b')]);
    near(startOf(after.of('b')), W + 1, 1e-9, 'cursor');
});

check('scheduler: firewall s2: the gesture runs with the walk on the same actor, the shield overlaps, the camera follows the shield', () => {
    const s2 = t.sceneById('s2'), anna = s2.entities.find(e => e.id === 'anna');
    const walk = anna.walks.list[0], gesture = anna.gestures.list[0], shield = s2.entities.find(e => e.item === 'prop.shield');
    near(walk.t0, 0.5, 1e-9, 'T of a fade is 0.5 s');
    near(gesture.t0, 0.5, 1e-9, 'with prev has no floor');
    near(startOf(shield), 0.5 + 0.4 * (walk.t1 - walk.t0), 1e-9, 'shield');
    near(s2.camera.zoom.segs[0].t0, shield.alpha.segs[0].t1, 1e-9, 'a camera without a target starts at the cursor');
});
```

- [ ] **Step 3: Run it**

```powershell
npm run test:storymaker
```

Expected: three `FAIL scheduler:` lines. The first reads `overlap: 0.5 vs 0.2`, because the old loop starts every plain beat at the cursor. `scheduler: targets compare as instances, not as strings` is not among them: it already passes on the loop M7 leaves and stays as a guard.

- [ ] **Step 4: Implement**

Apply these replacements in `ui/js/storymaker/beats.js`, as M7 leaves it. The file has CRLF line endings in the checkout. Keep them.

Edit 1. Old (the `dur` line is the text M7 left):
```js
            show: {
                dur: function (b) { return (b.entry || 'fade') === 'none' ? 0 : DEF.show * f; },
                run: function (b, j, t0, dur) {
                    if (b.chart || b.icon) {
                        warn('minor', beatPath(j), 'not_supported_yet', 'Show with ' + (b.chart ? 'a chart' : 'an icon') + ' is not supported yet; the beat is ignored.');
                        return;
                    }
                    if (b.text !== undefined && b.text !== null && b.item === undefined) {
                        var pos = textPosition(b.at, beatPath(j, 'at'));
                        var style = b.style && cat.textStyles[b.style] ? b.style : 'title';
                        if (b.style && !cat.textStyles[b.style] && !P.contains(['speech', 'thought'], b.style)) {
                            warn('warning', beatPath(j, 'style'), 'unknown_name', 'Unknown text style "' + b.style + '"; the player uses "title".');
                        }
                        var t = S.createText({ id: instanceId('text.text', b.id), text: b.text, style: style, x: pos.x, y: pos.y, hidden: true });
                        register(t, null);
                        entryAnim(t, b, t0, dur, pos);
                        return;
                    }
                    if (!b.item) { warn('warning', beatPath(j), 'missing_field', 'A show beat needs item, text, icon or chart.'); return; }
                    var entry = cat.byId[b.item];
                    if (!entry || entry.kind !== 'prop') { warn('warning', beatPath(j, 'item'), 'unknown_asset', 'Unknown prop "' + b.item + '".'); return; }
                    var at = resolveAt(b.at, beatPath(j, 'at'), 'center');
                    needFile(entry);
                    var prop = S.createProp({ id: instanceId(b.item, b.id), item: b.item, x: at.x, y: at.y, scale: b.scale, layer: b.layer, hidden: true, depth: at.depth });
                    register(prop, b.item);
                    entryAnim(prop, b, t0, dur, at);
                }
            },
```
New:
```js
            show: {
                dur: function (b) { return (b.entry || 'fade') === 'none' ? 0 : DEF.show * f; },
                run: function (b, j, t0, dur, tgt) { if (tgt) entryAnim(tgt, b, t0, dur, { x: tgt.x.init }); }
            },
```

Edit 2. Old:
```js
        function moveBeat(b, j, t0, dur, walking) {
```
New:
```js
        // showTarget(b, j): the instance a show beat creates. It must exist before the beat is placed, because the scheduler
        // compares instances. Returns null when the beat names nothing it can show.
        function showTarget(b, j) {
            if (b.text !== undefined && b.text !== null && b.item === undefined) {
                var pos = textPosition(b.at, beatPath(j, 'at'));
                var style = b.style && cat.textStyles[b.style] ? b.style : 'title';
                if (b.style && !cat.textStyles[b.style] && !P.contains(['speech', 'thought'], b.style)) {
                    warn('warning', beatPath(j, 'style'), 'unknown_name', 'Unknown text style "' + b.style + '"; the player uses "title".');
                }
                var t = S.createText({ id: instanceId('text.text', b.id), text: b.text, style: style, x: pos.x, y: pos.y, hidden: true });
                register(t, null);
                return t;
            }
            if (!b.item) { warn('warning', beatPath(j), 'missing_field', 'A show beat needs item, text, icon or chart.'); return null; }
            var entry = cat.byId[b.item];
            if (!entry || entry.kind !== 'prop') { warn('warning', beatPath(j, 'item'), 'unknown_asset', 'Unknown prop "' + b.item + '".'); return null; }
            var at = resolveAt(b.at, beatPath(j, 'at'), 'center');
            needFile(entry);
            var prop = S.createProp({ id: instanceId(b.item, b.id), item: b.item, x: at.x, y: at.y, scale: b.scale, layer: b.layer, hidden: true, depth: at.depth });
            register(prop, b.item);
            return prop;
        }

        function moveBeat(b, j, t0, dur, walking) {
```

Edit 3. Old:
```js
        var cursor = plan.T, prevStart = plan.T, beatsEnd = 0;
        beats.forEach(function (b, j) {
            if (!b || typeof b !== 'object') return;
            var spec = kinds[b.do];
            if (!spec) {
                warn('minor', beatPath(j), 'not_supported_yet', 'Beat "' + b.do + '" is not supported yet; it is ignored.');
                return;
            }
            var t0 = cursor, usedPrev = b.with === 'prev' && j > 0;
            if (usedPrev) t0 = prevStart;
            else if (b.cue !== undefined && b.cue !== null) {
                var ct = plan.narration.cues[b.cue];
                if (ct === undefined) warn('warning', beatPath(j, 'cue'), 'unknown_cue', 'Narration has no cue marker {' + b.cue + '}; the beat follows the previous one.');
                else t0 = ct;
            }
            t0 += b.delay > 0 ? b.delay : 0;
            var dur = typeof b.duration === 'number' && b.duration >= 0 ? b.duration : spec.dur(b, t0);
            spec.run(b, j, t0, dur);
            var end = t0 + dur;
            prevStart = t0;
            cursor = usedPrev ? Math.max(cursor, end) : end;
            if (end > beatsEnd) beatsEnd = end;
        });

        var lastEnd = SM.camera.end(cam);
        for (var k = 0; k < entities.length; k++) { var ee = S.entityEnd(entities[k]); if (ee > lastEnd) lastEnd = ee; }
        return { entities: entities, camera: cam, beatsEnd: beatsEnd, motionEnd: lastEnd, framing: framing, actorsPlaced: placed };
```
New:
```js
        // targetOf(b, j): the instance a beat acts on, compared by identity. A show's target is the instance it creates. A
        // string that resolves to no instance (a camera on a spot) has none.
        function targetOf(b, j) {
            if (b.do === 'show') return showTarget(b, j);
            return b.do === 'wait' ? null : find(b.who || b.target);
        }

        var cursor = plan.T, prev = null, anchor = null, beatsEnd = 0, lastEnd = new Map(), log = [];
        beats.forEach(function (b, j) {
            if (!b || typeof b !== 'object') return;
            var spec = kinds[b.do];
            if (!spec) {
                warn('minor', beatPath(j), 'not_supported_yet', 'Beat "' + b.do + '" is not supported yet; it is ignored.');
                return;
            }
            if (b.do === 'show' && (b.chart || b.icon)) {
                warn('minor', beatPath(j), 'not_supported_yet', 'Show with ' + (b.chart ? 'a chart' : 'an icon') + ' is not supported yet; the beat is ignored.');
                return;
            }
            var tgt = targetOf(b, j);
            var floor = tgt ? (lastEnd.get(tgt) || 0) : 0;
            // a walk measures its distance from the target's position at floor: only same-target beats move that track
            var dur = typeof b.duration === 'number' && b.duration >= 0 ? b.duration : spec.dur(b, floor);
            var delay = b.delay > 0 ? b.delay : 0, cueT, byPrev = false, t;
            if (b.cue !== undefined && b.cue !== null) {
                cueT = plan.narration.cues[b.cue];
                if (cueT === undefined) warn('warning', beatPath(j, 'cue'), 'unknown_cue', 'Narration has no cue marker {' + b.cue + '}; the beat follows the previous one.');
            }
            if (cueT !== undefined) t = cueT + delay;
            else if (b.with === 'prev' && prev) { byPrev = true; t = prev.start + delay; }
            else if (prev && prev.tgt && tgt && prev.tgt !== tgt) t = anchor.start + SM.motion.OVERLAP * anchor.dur + delay;
            else t = cursor + delay;
            // with prev has no floor: it keeps running together with the previous beat, also on the same target
            var start = byPrev ? Math.max(0, t) : Math.max(0, floor, t), end = start + dur;
            spec.run(b, j, start, dur, tgt);
            if (tgt) lastEnd.set(tgt, Math.max(lastEnd.get(tgt) || 0, end));
            var me = { j: j, kind: b.do, tgt: tgt, start: start, dur: dur, end: end, landing: null, cued: false };
            log.push(me);
            cursor = byPrev ? Math.max(cursor, end) : end;
            if (!byPrev || end >= anchor.end) anchor = me;
            prev = me;
            if (end > beatsEnd) beatsEnd = end;
        });

        var motionEnd = SM.camera.end(cam);
        for (var k = 0; k < entities.length; k++) { var ee = S.entityEnd(entities[k]); if (ee > motionEnd) motionEnd = ee; }
        return { entities: entities, camera: cam, beatsEnd: beatsEnd, motionEnd: motionEnd, framing: framing, actorsPlaced: placed, log: log };
```

- [ ] **Step 5: Run**

```powershell
npm run build:ui
npm run check:ui
npm run test:storymaker
```

Expected: `check:ui` passes and every `scheduler:` check passes. `test:storymaker` exits 1 with exactly these seven `FAIL` lines. Five are the firewall pins that M11 lists. The last two come from the same timing: with `with: prev` on the walk gesture, s2 is 92 frames, so the no-workers frame count is 296 (not above 300) and the failed code scene s3 ends at 296. M11 re-derives both pins. The outline allows all seven from M7 to M10.

- `compile: beat scheduling with "with: prev", cue, delay and duration`
- `player worker: hello, plan, compile, assets, render, in that protocol`
- `host app: ready, load, compile report, first frame drawn, target origin`
- `host app: seek, play with frame dropping, pause and the end of the video`
- `render mode: boot.json, report, then every frame in order (png)`
- `host app: without workers the storyboard renders in the host realm and code scenes show a placeholder`
- `host app: a code scene that fails to build is reported, its scene shows a placeholder`

Any other `FAIL` is a real failure. Send it to debug-agent.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/storymaker/beats.js scripts/storymaker-tests/13-motion.mjs ui/js/storymaker/player.bundle.js
git commit -m "feat(storymaker): schedule beats by target with overlap, floors and anchor" -m "The firewall timing pins stay red until the re-pin task of this batch." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task M9: Cue lead and `cue_late` (steps 5, 6a, 7), sfx impact 0
**Batch:** 2A  **Depends on:** M8
**Files:**
Modify: `ui/js/storymaker/beats.js`
Modify: `ui/js/storymaker/motion.js`
Modify: `internal/storymaker/model.go`
Modify: `ui/js/storymaker/player.bundle.js` (generated by `npm run build:ui`)
Test: `scripts/storymaker-tests/13-motion.mjs`

- [ ] **Step 1: Impact**

Run this from the main checkout. Report callers, processes and risk, and treat HIGH, CRITICAL and UNKNOWN as outline section 5 says.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "buildStoryboard" --direction upstream --repo .
```

- [ ] **Step 2: Write the failing test**

Append to the end of `scripts/storymaker-tests/13-motion.mjs`. The helpers `one`, `lastSeg` and `WALK` are already in the file.

```js
// ---- cue lead and cue_late

// The marker of LONG sits at 20 / 31 of 16 / 2.3 s, about 4.5 s, so no lead in the table is clamped at 0.
const LONG = 'a b c d e f g h i j {1}k l m n o p';
const cueOf = narration => SM.planTiming({ scenes: [{ id: 's', narration }] }, {}, catalog).scenes.s.cues['1'];

// trackReach(tr, level): the first scene time at which the last segment of tr has come level of its way, in 0.1 ms steps.
function trackReach(tr, level) {
    const seg = lastSeg(tr);
    for (let k = 0; ; k++) {
        const x = seg.t0 + k * 1e-4;
        if (x > seg.t1) return seg.t1;
        if ((tr.at(x) - seg.from) / (seg.to - seg.from) >= level) return x;
    }
}

const ARRIVALS = [
    ['enter', { do: 'enter', who: 'anna', to: 'center' }, o => o.anna.x, 0.9],
    ['walk', { do: 'walk', who: 'anna', to: 'center' }, o => o.anna.x, 0.9],
    ['move', { do: 'move', who: 'anna', to: 'center' }, o => o.anna.x, 0.9],
    ['show fade', { do: 'show', item: 'prop.lock', at: 'right', entry: 'fade' }, o => o.item('prop.lock').alpha, 0.9],
    ['show slide', { do: 'show', item: 'prop.lock', at: 'right', entry: 'slide' }, o => o.item('prop.lock').x, 0.9],
    // the spring lands on its first crossing of the target
    ['show pop', { do: 'show', item: 'prop.lock', at: 'right', entry: 'pop' }, o => o.item('prop.lock').scale, 1],
    ['camera focus', { do: 'camera', op: 'focus', target: 'anna' }, o => o.scene.camera.cx, 0.9],
    ['camera zoom', { do: 'camera', op: 'zoom', target: 'anna' }, o => o.scene.camera.cx, 0.9]
];

check('cue lead: an arrival lands on its marker plus delay within 1 ms', () => {
    const cue = cueOf(LONG);
    for (const [name, beat, trackOf, level] of ARRIVALS) {
        for (const delay of [0, 0.3]) {
            const o = one([{ ...beat, cue: 1, delay }], { narration: LONG });
            near(trackReach(trackOf(o), level), cue + delay, 0.001, name + ' with delay ' + delay);
            assert.ok(!o.issues.some(i => i.code === 'cue_late'), name + ' is not late');
        }
    }
});

check('cue lead: a clamp at scene time 0 does not warn, a busy target gives cue_late, a cue beats with', () => {
    // marker at 0 and a lead of 0.54 s: the beat would start before 0, so it starts at 0 without a warning
    const early = one([{ do: 'enter', who: 'anna', to: 'center', cue: 1 }], { narration: '{1}a b c d e f g h' });
    near(lastSeg(early.anna.x).t0, 0, 1e-9, 'clamp');
    assert.ok(!early.issues.some(i => i.code === 'cue_late'), 'a clamp at 0 never warns');
    // the walk holds anna until its end; the move on the same target cannot start before that
    const busy = one([WALK, { do: 'move', who: 'anna', to: 'right', cue: 1 }], { narration: 'a {1}b c d e f g h' });
    const walkEnd = busy.anna.walks.list[0].t1;
    near(lastSeg(busy.anna.x).t0, walkEnd, 1e-9, 'the beat waits for the previous beat on its target');
    const late = busy.issues.filter(i => i.code === 'cue_late');
    assert.equal(late.length, 1);
    assert.equal(late[0].severity, 'warning');
    assert.equal(late[0].path, '/scenes/0/beats/1/cue');
    assert.equal(late[0].scene_id, 's');
    // at_seconds is the landing: the start plus 0.4377 of the 0.8 s move
    near(late[0].at_seconds, walkEnd + 0.4377 * 0.8, 1e-6, 'at_seconds');
    // a show with entry none has no lead: with a cue it starts on the marker, not with the walk
    const both = one([WALK, { do: 'show', item: 'prop.lock', at: 'right', entry: 'none', with: 'prev', cue: 1 }], { narration: LONG });
    near(lastSeg(both.item('prop.lock').alpha).t0, cueOf(LONG), 1e-9, 'a cue beats with');
});

check('cue lead: an sfx has no target and impact 0: with prev it sits on the landing, with a cue on the marker plus delay', () => {
    // The show runs with the sfx, so its start shows the sfx start. The id is in no catalog, so its impact stays 0.
    // enter lasts 0.6 s and is linear, so its landing is 0.9 * 0.6 = 0.54 s
    const onLanding = one([{ do: 'enter', who: 'anna', to: 'center' }, { do: 'sfx', sound: 'sfx.absent', with: 'prev' }, { do: 'show', item: 'prop.lock', with: 'prev' }]);
    near(lastSeg(onLanding.item('prop.lock').alpha).t0, 0.54, 1e-9, 'on the landing');
    const onCue = one([{ do: 'sfx', sound: 'sfx.absent', cue: 1, delay: 0.2 }, { do: 'show', item: 'prop.lock', with: 'prev' }], { narration: LONG });
    near(lastSeg(onCue.item('prop.lock').alpha).t0, cueOf(LONG) + 0.2, 1e-9, 'on the marker');
});
```

- [ ] **Step 3: Run it**

```powershell
npm run test:storymaker
```

Expected: three `FAIL cue lead:` lines, plus the seven firewall `FAIL` lines that M8 left. The first new line reads `enter with delay 0: ...`, because the beat still starts on the marker and lands 0.54 s late.

- [ ] **Step 4: Implement**

In `ui/js/storymaker/motion.js`, as M7 leaves it, apply two replacements.

Edit 1. Old:
```js
    SM.motion = {
```
New:
```js
    // isArrival(b): true for a beat that lands. Its lead counts against a cue, and sfx with prev reads its landing.
    function isArrival(b) {
        if (b.do === 'enter' || b.do === 'walk' || b.do === 'move') return true;
        if (b.do === 'show') return (b.entry || 'fade') !== 'none';
        return b.do === 'camera' && (b.op === 'focus' || (b.op === 'zoom' && !!b.target));
    }

    // landingEase(b): the ease the landing is read from. A fade reads its alpha, the others their moving track.
    function landingEase(b) {
        if (b.do === 'show' && b.entry !== 'slide' && !(b.entry === 'pop' && !isTextShow(b))) return fadeEase(b);
        return SM.isEase(b.ease) ? b.ease : defaultEase(b);
    }

    // leadOf(b, dur): the seconds from the start of an arrival to its landing, 0 for every other beat.
    function leadOf(b, dur) { return isArrival(b) ? leadFraction(landingEase(b)) * dur : 0; }

    SM.motion = {
```

Edit 2. Old:
```js
        defaultEase: defaultEase, fadeEase: fadeEase
```
New:
```js
        defaultEase: defaultEase, fadeEase: fadeEase, isArrival: isArrival, leadOf: leadOf
```

Apply these replacements in `ui/js/storymaker/beats.js`, as M8 leaves it. The file has CRLF line endings in the checkout. Keep them.

Edit 3. Old:
```js
// Storymaker player: storyboard scenes. buildStoryboard turns one scene (actors, props, camera, beats) into entities
// with tracks. Beats run in order; "with": "prev" starts a beat together with the previous one; "cue": n starts it at
// narration marker {n}; delay, duration and ease are optional. Timing defaults are owned here (enter 0.6 s, walk from the
// distance at the catalog's walk speed, ...). Phase-1 support matrix: enter exit walk move pose gesture emote look show
// (item and text) hide camera wait; the other beats are reported as minor not_supported_yet and ignored. Classic script;
// publishes StorymakerCore.beats.
```
New:
```js
// Storymaker player: storyboard scenes. buildStoryboard turns one scene (actors, props, camera, beats) into entities
// with tracks. The scheduler: a beat on another target starts at 0.4 of the anchor, a beat on the same target follows it,
// "with": "prev" runs together with the previous beat, and a cued arrival lands on its marker. Timing defaults and role
// eases live in motion.js and are scaled by style.rhythm. Beats: enter exit walk move pose gesture emote look show (item
// and text) hide camera wait; sfx is placed with impact 0; the other beats are reported as minor not_supported_yet and
// ignored. Classic script; publishes StorymakerCore.beats.
```

Edit 4. Old:
```js
            wait: {
```
New:
```js
            sfx: {
                dur: function () { return 0; },
                run: function (b, j) { warn('minor', beatPath(j), 'not_supported_yet', 'Beat "sfx" is not supported yet; it is ignored.'); }
            },
            wait: {
```

Edit 5. Old:
```js
            return b.do === 'wait' ? null : find(b.who || b.target);
```
New:
```js
            return b.do === 'wait' || b.do === 'sfx' ? null : find(b.who || b.target);
```

Edit 6. Old:
```js
            var delay = b.delay > 0 ? b.delay : 0, cueT, byPrev = false, t;
```
New:
```js
            // impact stays 0 until the catalog has sfx entries
            var delay = b.delay > 0 ? b.delay : 0, lead = SM.motion.leadOf(b, dur), impact = 0, cueT, byPrev = false, t;
```

Edit 7. Old:
```js
            if (cueT !== undefined) t = cueT + delay;
```
New:
```js
            if (cueT !== undefined) t = cueT + delay - lead - impact;
```

Edit 8. Old:
```js
            else if (b.with === 'prev' && prev) { byPrev = true; t = prev.start + delay; }
```
New:
```js
            else if (b.with === 'prev' && prev) {
                byPrev = true;
                t = b.do === 'sfx' ? (prev.landing !== null ? prev.landing : prev.start) + delay - impact : prev.start + delay;
            }
```

Edit 9. Old:
```js
            var start = byPrev ? Math.max(0, t) : Math.max(0, floor, t), end = start + dur;
```
New:
```js
            var start = byPrev ? Math.max(0, t) : Math.max(0, floor, t), end = start + dur;
            // floor > 0: a clamp at scene time 0 never warns, only a previous beat on the target does
            if (cueT !== undefined && floor > 0 && floor > t + 1e-6) {
                issues.push(P.issue('warning', sid, beatPath(j, 'cue'), 'cue_late', 'The previous beat on this target still runs at marker {' + b.cue + '}; this beat lands at ' + (start + lead).toFixed(2) + ' s.', { at_seconds: start + lead }));
            }
```

Edit 10. Old:
```js
            var me = { j: j, kind: b.do, tgt: tgt, start: start, dur: dur, end: end, landing: null, cued: false };
```
New:
```js
            var me = { j: j, kind: b.do, tgt: tgt, start: start, dur: dur, end: end, landing: SM.motion.isArrival(b) ? start + lead : null, cued: cueT !== undefined };
```

In `internal/storymaker/model.go`, add the code in a block of its own.

Old:
```go
	CodeLayoutUnverified = "layout_unverified"
)
```
New:
```go
	CodeLayoutUnverified = "layout_unverified"
)

// Issue codes of phase 2 that only the player emits. The desktop app localises them as storymaker.issue.<code>.
const (
	CodeCueLate = "cue_late" // warning, player
)
```

- [ ] **Step 5: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
npm run build:ui
npm run check:ui
npm run test:storymaker
go build ./internal/storymaker
gofmt -l internal/storymaker
```

Expected:
- `check:ui` passes.
- Every `cue lead:` and `scheduler:` check passes.
- `test:storymaker` exits 1 with only the seven firewall `FAIL` lines of the allowed red state: `compile: beat scheduling with "with: prev", cue, delay and duration`, `player worker: hello, plan, compile, assets, render, in that protocol`, `host app: ready, load, compile report, first frame drawn, target origin`, `host app: seek, play with frame dropping, pause and the end of the video`, `render mode: boot.json, report, then every frame in order (png)`, `host app: without workers the storyboard renders in the host realm and code scenes show a placeholder`, `host app: a code scene that fails to build is reported, its scene shows a placeholder`.
- `go build` passes and `gofmt -l` prints nothing.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/storymaker/beats.js ui/js/storymaker/motion.js internal/storymaker/model.go scripts/storymaker-tests/13-motion.mjs ui/js/storymaker/player.bundle.js
git commit -m "feat(storymaker): land cued arrivals on the marker and report cue_late" -m "The firewall timing pins stay red until the re-pin task of this batch." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task M10: Holds, beat log, scene length
**Batch:** 2A  **Depends on:** M9
**Files:**
Modify: `ui/js/storymaker/beats.js`
Modify: `ui/js/storymaker/compile.js`
Modify: `ui/js/storymaker/player.bundle.js` (generated by `npm run build:ui`)
Test: `scripts/storymaker-tests/13-motion.mjs`

- [ ] **Step 1: Impact**

Run these from the main checkout. Report callers, processes and risk, and treat HIGH, CRITICAL and UNKNOWN as outline section 5 says.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "buildStoryboard" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "compile" --direction upstream --repo .
```

- [ ] **Step 2: Write the failing test**

Append to the end of `scripts/storymaker-tests/13-motion.mjs`. The helpers `one` and `CAST` are already in the file.

```js
// ---- holds, scene length, beat log

// the mini catalog has only the title style; a secondary line needs a caption
const withCaption = { ...catalog, entries: [...catalog.entries, { id: 'text.caption', kind: 'text_style', name: 'caption', font: 'default', size: 0.042, weight: 400, color: '$text', align: 'center', max_lines: 2 }] };

check('holds: an auto scene lasts until the text has been read, an explicit duration is never extended', () => {
    const title = { do: 'show', text: 'eins zwei drei vier' };
    // a 0.5 s fade lands at 0.4377 * 0.5 = 0.219 s; 4 words hold 0.5 + 0.3 * 4 = 1.7 s; 1.919 s * 30 = 57.57, rounds to 58 frames
    assert.equal(one([title], { actors: [] }).end, 58);
    assert.equal(one([title], { actors: [], duration: 1 }).end, 30, 'an explicit duration stays');
    // lively: a 0.4 s fade lands at 0.175 s; the hold stays 1.7 s; 1.875 s * 30 = 56.25, rounds to 56 frames
    assert.equal(one([title], { actors: [], style: { rhythm: 'lively' } }).end, 56, 'lively keeps the base hold');
    // calm: a 0.625 s fade lands at 0.274 s; the hold is 1.7 * 1.25 = 2.125 s; 2.399 s * 30 = 71.96, rounds to 72 frames
    assert.equal(one([title], { actors: [], style: { rhythm: 'calm' } }).end, 72, 'calm scales the hold');
    // a hide removes the hold: show 0.5 s, hide 0.35 s, so the 1.0 s minimum decides: 30 frames
    assert.equal(one([{ ...title, id: 'x' }, { do: 'hide', target: 'x' }], { actors: [] }).end, 30, 'a hidden text holds nothing');
});

check('holds: a secondary line holds at least 1.2 s beside a title, not alone', () => {
    const title = { do: 'show', text: 'Titel' }, line = { do: 'show', text: 'eins', style: 'caption' };
    // the title fades over 0..0.5 s. The caption is another target: it starts at 0.4 * 0.5 = 0.2 s and lands at 0.2 + 0.219 = 0.419 s.
    // 1 word is 0.8 s, raised to 1.2 s: 1.619 s * 30 = 48.57, rounds to 49 frames
    assert.equal(one([title, line], { actors: [], catalog: withCaption }).end, 49, 'beside a title');
    // alone it is no secondary line: 0.219 + 0.8 = 1.019 s; 1.019 s * 30 = 30.57, rounds to 31 frames
    assert.equal(one([line], { actors: [], catalog: withCaption }).end, 31, 'alone');
});

check('scene length: a code scene without a usable duration gets a number, not NaN', () => {
    // no code-scene info, so the automatic length falls to the 1.0 s minimum: 30 frames
    assert.equal(SM.compile({ scenes: [{ id: 'c', kind: 'code' }] }, {}, catalog).report.scenes[0].end_frame, 30);
});

// storyboardOf(sc): buildStoryboard for one scene, because compile does not hand out its log.
function storyboardOf(sc) {
    const story = { cast: CAST, scenes: [sc] }, cat = SM.scene.indexCatalog(catalog), fmt = SM.plan.normaliseFormat(story, {}), issues = [];
    const plan = SM.plan.planScene(story, {}, cat, fmt, 0, issues);
    const stage = SM.stage.create({ aspect: fmt.aspect, width: fmt.width, height: fmt.height, env: cat.byId[sc.environment], safe: catalog.stage.safe });
    return SM.beats.buildStoryboard({ story, scene: sc, index: 0, plan, cat, fmt, stage, issues, needs: {}, castMap: { anna: CAST[0] } });
}

check('beat log: every placed beat with its target, times and landing', () => {
    const o = storyboardOf({ id: 's', environment: 'env.office', actors: [{ cast: 'anna', at: 'left' }], beats: [{ do: 'enter', who: 'anna', to: 'center' }, { do: 'emote', who: 'anna', expression: 'happy' }, { do: 'dance' }] });
    assert.equal(o.log.length, 2, 'an unknown kind is not logged');
    const enter = o.log[0], emote = o.log[1];
    assert.equal(enter.j, 0);
    assert.equal(enter.kind, 'enter');
    assert.equal(enter.tgt, o.entities[0], 'the target is the instance');
    assert.equal(enter.cued, false);
    near(enter.start, 0, 1e-9, 'enter start');
    near(enter.end, 0.6, 1e-9, 'enter end');
    near(enter.landing, 0.54, 1e-9, 'linear: 0.9 of 0.6 s');
    near(emote.start, 0.6, 1e-9, 'emote start');
    assert.equal(emote.landing, null, 'an emote is no arrival');
    assert.equal(o.holdsEnd, 0);
    assert.equal(o.blocksEnd, 0);
    assert.deepEqual(plain(o.audioEvents), []);
});
```

- [ ] **Step 3: Run it**

```powershell
npm run test:storymaker
```

Expected: these new failures, plus the seven firewall `FAIL` lines from M8.
- `FAIL holds: an auto scene ...` with `30 !== 58`.
- `FAIL holds: a secondary line ...` with `30 !== 49`.
- `FAIL beat log: ...`, because `holdsEnd` is undefined.

The code-scene check passes under the old formula. It guards the new one.

- [ ] **Step 4: Implement**

Apply these replacements in `ui/js/storymaker/beats.js`, as M9 leaves it. The file has CRLF line endings in the checkout. Keep them.

Edit 1. Old:
```js
    function buildStoryboard(c) {
```
New:
```js
    // holdsEndOf(log, factor): the largest landing plus scaled hold of a shown text that no hide removes. A text without a
    // landing (entry none) holds from its start.
    function holdsEndOf(log, factor) {
        var hidden = new Set(), showsTitle = false, out = 0;
        function isText(l) { return l.kind === 'show' && l.tgt && l.tgt.type === 'text'; }
        log.forEach(function (l) {
            if (l.kind === 'hide' && l.tgt) hidden.add(l.tgt);
            if (isText(l) && l.tgt.style === 'title') showsTitle = true;
        });
        log.forEach(function (l) {
            if (!isText(l) || hidden.has(l.tgt)) return;
            var secondary = showsTitle && (l.tgt.style === 'caption' || l.tgt.style === 'label');
            var hold = SM.motion.holdSeconds(l.tgt.text, { secondary: secondary, factor: factor }).scaled;
            var end = (l.landing !== null ? l.landing : l.start) + hold;
            if (end > out) out = end;
        });
        return out;
    }

    function buildStoryboard(c) {
```

Edit 2. Old (the line M8 left):
```js
        return { entities: entities, camera: cam, beatsEnd: beatsEnd, motionEnd: motionEnd, framing: framing, actorsPlaced: placed, log: log };
```
New:
```js
        return {
            entities: entities, camera: cam, beatsEnd: beatsEnd, motionEnd: motionEnd, framing: framing, actorsPlaced: placed, log: log,
            holdsEnd: holdsEndOf(log, f), blocksEnd: 0, audioEvents: []
        };
```

Apply these replacements in `ui/js/storymaker/compile.js`. The file has CRLF line endings in the checkout. Keep them.

Edit 3. Old:
```js
            var built = { entities: [], camera: SM.camera.create(1, stage.vis.x + stage.vis.w / 2, stage.vis.y + stage.vis.h / 2), beatsEnd: 0, motionEnd: 0 };
```
New:
```js
            var built = { entities: [], camera: SM.camera.create(1, stage.vis.x + stage.vis.w / 2, stage.vis.y + stage.vis.h / 2), beatsEnd: 0, motionEnd: 0, blocksEnd: 0, holdsEnd: 0 };
```

Edit 4. Old:
```js
            var autoD = Math.max(T + n.seconds + tail, built.beatsEnd, T + (n.seconds > 0 ? 0 : P.MIN_SCENE_SECONDS));
```
New:
```js
            var autoD = Math.max(T + n.seconds + tail, built.blocksEnd > 0 ? built.blocksEnd + SM.narration.TAIL_SECONDS : 0, built.beatsEnd, built.holdsEnd, T + (n.seconds > 0 ? 0 : P.MIN_SCENE_SECONDS));
```

- [ ] **Step 5: Run**

```powershell
npm run build:ui
npm run check:ui
npm run test:storymaker
```

Expected:
- `check:ui` passes.
- Every check in `scripts/storymaker-tests/13-motion.mjs` passes.
- `test:storymaker` exits 1 with only the seven firewall `FAIL` lines of the allowed red state: `compile: beat scheduling with "with: prev", cue, delay and duration`, `player worker: hello, plan, compile, assets, render, in that protocol`, `host app: ready, load, compile report, first frame drawn, target origin`, `host app: seek, play with frame dropping, pause and the end of the video`, `render mode: boot.json, report, then every frame in order (png)`, `host app: without workers the storyboard renders in the host realm and code scenes show a placeholder`, `host app: a code scene that fails to build is reported, its scene shows a placeholder`.

- [ ] **Step 6: Commit**

Run `detect_changes` per outline section 5:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/storymaker/beats.js ui/js/storymaker/compile.js scripts/storymaker-tests/13-motion.mjs ui/js/storymaker/player.bundle.js
git commit -m "feat(storymaker): text holds extend the automatic scene length" -m "The firewall timing pins stay red until the re-pin task of this batch." -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task M11: Re-pin the firewall timing
**Batch:** 2A  **Depends on:** M7, M8, M9, M10
**Files:**
Test: `scripts/storymaker-tests/04-compile.mjs`
Test: `scripts/storymaker-tests/07-workers.mjs`
Test: `scripts/storymaker-tests/08-host-app.mjs`
Test: `scripts/storymaker-tests/09-host-render.mjs`
Test: `ui/storymaker_player_gate_browser_test.go`

Every number below comes from the rules, as worked out here. With the mini catalog (walk speed 12) and 30 fps:

| Item | Derivation | Result |
| --- | --- | --- |
| s1 length | 9 words / 2.3 + 0.4 s tail = 4.313 s | 129 frames, range 0..129 |
| s1 beats | end at 2.346 s, before the narration | no effect |
| s2 start | fade 15 frames, 129 - 15 | frame 114 |
| s2 length | 0.5 s + 5 words / 2.3 + 0.4 s tail = 3.074 s | 92 frames, range 114..206 |
| s2 walk | starts at 0.5 s, 384 px / (0.12 * 1920 * 0.9733) | 1.712 s |
| s2 shield | starts at 0.5 + 0.4 * 1.712 = 1.185 s, lasts 0.5 s | ends 1.685 s |
| s2 camera | starts at the cursor, lasts 0.8 s | ends 2.485 s, before 3.074 s |
| s3 start | 206 - 15 | frame 191 |
| s3 with `api.duration(5)` | 150 frames | ends 341, last frame 340 |
| s3 with `api.duration(4)` | 120 frames | ends 311 |
| s3 with no built duration (no-workers fallback, or a failed build) | narration 6 words / 2.3 = 2.609 s, plus T 0.5 s and tail 0.4 s = 3.509 s = 105 frames | range 191..296, so frame_count 296 |

- [ ] **Step 1: Impact**

Run these from the main checkout. Report callers, processes and risk, and treat HIGH, CRITICAL and UNKNOWN as outline section 5 says.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "TestStorymakerPlayerGateBrowser" --direction upstream --repo .
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "TestStorymakerFirewallRenderBrowser" --direction upstream --repo .
```

- [ ] **Step 2: Run the failing state**

The failing tests already exist: the old pins, against the rules that M8 to M10 landed.

```powershell
npm run build:ui
npm run test:storymaker
```

Expected: exit 1 with exactly these seven `FAIL` lines.

- `compile: beat scheduling with "with: prev", cue, delay and duration` (shield start at line 73 of `scripts/storymaker-tests/04-compile.mjs`)
- `player worker: hello, plan, compile, assets, render, in that protocol` (`341 !== 354` at line 43 of `scripts/storymaker-tests/07-workers.mjs`)
- `host app: ready, load, compile report, first frame drawn, target origin` (line 64 of `scripts/storymaker-tests/08-host-app.mjs`)
- `host app: seek, play with frame dropping, pause and the end of the video` (a timeout at line 137 of the same file)
- `host app: without workers the storyboard renders in the host realm and code scenes show a placeholder` (line 264 of the same file: `frame_count` is 296, not above 300)
- `host app: a code scene that fails to build is reported, its scene shows a placeholder` (a timeout at line 227 of the same file, waiting for frame 300, past the end at 296)
- `render mode: boot.json, report, then every frame in order (png)` (line 94 of `scripts/storymaker-tests/09-host-render.mjs`)

If any other check fails, stop and send it to debug-agent. It is not a firewall pin.

- [ ] **Step 3: Implement**

The `.mjs` files have CRLF line endings in the checkout. Keep them.

In `scripts/storymaker-tests/04-compile.mjs`. Old:
```js
    near(shield.alpha.segs[0].t0, walk.t1, 1e-9, 'the next plain beat follows the longest of the group');
```
New:
```js
    // the shield's target differs from the gesture's, so it starts at 0.4 of the anchor; the anchor stays on the walk, the longest beat of the with-prev group
    near(shield.alpha.segs[0].t0, walk.t0 + 0.4 * (walk.t1 - walk.t0), 1e-9, 'the next beat on another target overlaps the anchor');
```

In `scripts/storymaker-tests/07-workers.mjs`. Old:
```js
    assert.equal(comp.ok, true); assert.equal(comp.report.frame_count, 354);
```
New:
```js
    // s1 is 129 frames: 9 words / 2.3 + 0.4 s tail = 4.313 s. s2 starts at 129 - 15 = 114. Its narration decides its length:
    // 0.5 s + 5 words / 2.3 + 0.4 s tail = 3.074 s = 92 frames, so it ends at 206. Its beats end earlier: walk 1.712 s from 0.5 s,
    // shield at 0.5 + 0.4 * 1.712 = 1.185 s for 0.5 s, camera 0.8 s until 2.485 s. s3 starts at 206 - 15 = 191 and lasts api.duration(5) = 150 frames: 341.
    assert.equal(comp.ok, true); assert.equal(comp.report.frame_count, 341);
```

In `scripts/storymaker-tests/08-host-app.mjs`, three replacements.

Edit 1. Old:
```js
    assert.equal(rep.frame_count, 354); assert.equal(rep.width, 480); assert.equal(rep.height, 270);
```
New:
```js
    // 341 frames: s1 0..129, s2 114..206 (0.5 s + 5 words / 2.3 + 0.4 s tail = 92 frames), s3 191..341 (206 - 15, then api.duration(5) = 150 frames)
    assert.equal(rep.frame_count, 341); assert.equal(rep.width, 480); assert.equal(rep.height, 270);
```

Edit 2. Old:
```js
    await until(() => a.sent('time').some(m => m.frame === 353), 'seek clamps to the last frame');
```
New:
```js
    await until(() => a.sent('time').some(m => m.frame === 340), 'seek clamps to the last frame, 341 - 1');
```

Edit 3. Old:
```js
    assert.equal(a.sent('time').at(-1).frame, 353); assert.equal(a.sent('time').at(-1).playing, false);
```
New:
```js
    assert.equal(a.sent('time').at(-1).frame, 340); assert.equal(a.sent('time').at(-1).playing, false);
```

The same file gets five more replacements, Edits 4 to 8. They are firewall pins that M11 missed: without workers, and for a failed build, the code scene is not built, so it takes its length from its narration, and s3 now starts at 191.

Edit 4. Old:
```js
    assert.equal(rep.scenes.length, 3); assert.ok(rep.frame_count > 300);
```
New:
```js
    // 296 frames: s1 0..129, s2 114..206 (0.5 s + 5 words / 2.3 + 0.4 s tail = 92 frames), s3 191..296. Without workers the code scene
    // is not built, so it takes its narration: 0.5 s + 6 words / 2.3 + 0.4 s tail = 3.509 s = 105 frames
    assert.equal(rep.scenes.length, 3); assert.equal(rep.frame_count, 296);
```

Edit 5. Old:
```js
        a.send({ type: 'seek', frame: 300 });
        await until(() => a.sent('time').some(m => m.frame === 300), 'a frame inside the failed code scene');
```
New:
```js
        // s3 is 191..296 here: the failed scene has no built duration, so it takes its narration (see Edit 4)
        const inside = rep.scenes[2].start_frame + 20;
        a.send({ type: 'seek', frame: inside });
        await until(() => a.sent('time').some(m => m.frame === inside), 'a frame inside the failed code scene');
```

Edit 6. The late-failure test pins s3 at 204, the start from before M8. It still passes, because 219 lies inside s3 either way, but the pin is stale. Old:
```js
    a.send({ type: 'seek', frame: 204 + 15 });
    await until(() => a.sent('time').some(m => m.frame === 219), 'the early code scene frame');
```
New:
```js
    // s3 starts at 191 (206 - 15, s2 ends at 206) and lasts api.duration(4) = 120 frames, so it ends at 311
    a.send({ type: 'seek', frame: 191 + 15 });
    await until(() => a.sent('time').some(m => m.frame === 206), 'the early code scene frame');
```

Edit 7. Old:
```js
    a.send({ type: 'seek', frame: 204 + 60 });
```
New:
```js
    a.send({ type: 'seek', frame: 191 + 60 });
```

Edit 8. Old:
```js
    a.send({ type: 'seek', frame: 204 + 70 });
    await until(() => a.sent('time').some(m => m.frame === 274), 'the next frame still renders');
```
New:
```js
    a.send({ type: 'seek', frame: 191 + 70 });
    await until(() => a.sent('time').some(m => m.frame === 261), 'the next frame still renders');
```

In `scripts/storymaker-tests/09-host-render.mjs`, two replacements.

Edit 1. Old:
```js
    assert.equal(rep.frame_count, 354); assert.equal(rep.width, 192); assert.equal(rep.height, 108);
```
New:
```js
    // 341 frames: s1 0..129, s2 114..206 (0.5 s + 5 words / 2.3 + 0.4 s tail = 92 frames), s3 191..341 (206 - 15, then api.duration(5) = 150 frames)
    assert.equal(rep.frame_count, 341); assert.equal(rep.width, 192); assert.equal(rep.height, 108);
```

Edit 2. Old:
```js
    assert.equal(r.frames.length, 354);
```
New:
```js
    assert.equal(r.frames.length, 341);
```

In `ui/storymaker_player_gate_browser_test.go`, three replacements.

Edit 1. Old:
```go
	// compiled report: scene ranges and frame count (s1 129 frames, s2 114..219, s3 204..324 with api.duration(4))
	if res.Report.FrameCount != 324 || res.Report.FPS != 30 || res.Report.Width != 640 || res.Report.Height != 360 {
		t.Errorf("report = %+v, want 324 frames at 30 fps, 640x360", res.Report)
	}
	want := [][3]int{{0, 129, 0}, {114, 219, 0}, {204, 324, 0}}
```
New:
```go
	// compiled report: scene ranges and frame count. s1: 9 words / 2.3 + 0.4 s tail = 4.313 s = 129 frames. s2 starts 15 frames
	// earlier, at 114. Its narration decides its length, 0.5 s + 5 words / 2.3 + 0.4 s tail = 3.074 s = 92 frames, so it ends at 206.
	// Its beats end earlier: walk 1.712 s from 0.5 s, shield at 0.5 + 0.4 * 1.712 = 1.185 s for 0.5 s, camera 0.8 s until 2.485 s.
	// s3 starts at 206 - 15 = 191 and lasts api.duration(4) = 120 frames, so it ends at 311.
	if res.Report.FrameCount != 311 || res.Report.FPS != 30 || res.Report.Width != 640 || res.Report.Height != 360 {
		t.Errorf("report = %+v, want 311 frames at 30 fps, 640x360", res.Report)
	}
	want := [][3]int{{0, 129, 0}, {114, 206, 0}, {191, 311, 0}}
```

Edit 2. Old:
```go
		if in.report == nil || int(in.report["frame_count"].(float64)) != 354 || len(in.frames) != 354 {
			t.Fatalf("report = %v, frames = %d, want 354", in.report, len(in.frames))
```
New:
```go
		// s1 0..129, s2 114..206 (0.5 s + 5 words / 2.3 + 0.4 s tail = 92 frames), s3 191..341 (206 - 15, then api.duration(5) = 150 frames)
		if in.report == nil || int(in.report["frame_count"].(float64)) != 341 || len(in.frames) != 341 {
			t.Fatalf("report = %v, frames = %d, want 341", in.report, len(in.frames))
```

Edit 3. Old:
```go
		t.Errorf("only %d distinct frames out of 354: the story barely moves", len(distinct))
```
New:
```go
		t.Errorf("only %d distinct frames out of 341: the story barely moves", len(distinct))
```

- [ ] **Step 4: Run**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
npm run test:storymaker
gofmt -l ui/storymaker_player_gate_browser_test.go
$env:AURAGO_RUN_BROWSER_SMOKE='1'
go test ./ui -run '^(TestStorymakerPlayerGateBrowser|TestStorymakerRenderModeBrowser|TestStorymakerFirewallRenderBrowser|TestStorymakerRenderScreenshotBrowser)$' -count=1 -v -timeout 15m
Remove-Item Env:AURAGO_RUN_BROWSER_SMOKE
```

Expected:
- `test:storymaker` ends with `0 failed`.
- `gofmt -l` prints nothing.
- `go test` shows four top-level `--- PASS` lines, one per named test, and none skipped. Count them, because this run is the evidence. No matrix runs these four tests.

Run this with the shell sandbox off, because repo-built executables cannot start Chrome on this box.

- [ ] **Step 5: Commit**

Run `detect_changes` per outline section 5:

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add scripts/storymaker-tests/04-compile.mjs scripts/storymaker-tests/07-workers.mjs scripts/storymaker-tests/08-host-app.mjs scripts/storymaker-tests/09-host-render.mjs ui/storymaker_player_gate_browser_test.go
git commit -m "test(storymaker): re-pin the firewall timing from the new scheduler rules" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

<!-- G-M-3 -->

### Task M12: Highlight beat
**Batch:** 2A  **Depends on:** M8
**Files:**
- Modify: `internal/storymaker/model.go`
- Modify: `ui/js/storymaker/beats.js`
- Modify: `ui/js/storymaker/scene.js`
- Modify: `ui/js/storymaker/render.js`
- Modify: `ui/js/storymaker/plan.js`
- Modify (generated by `npm run build:ui`): `ui/js/storymaker/player.bundle.js`
- Test: `internal/storymaker/validate_test.go`
- Test: `scripts/storymaker-tests/04-compile.mjs`
- Test: `scripts/storymaker-tests/05-render.mjs`

- [ ] **Step 1: Impact**

Run this from the main checkout, once per symbol: `buildStoryboard`, `createActor`, `createProp`, `createText`, `evalEntity`, `entityEnd`, `drawText`, `drawScene`, `phase1Beats`.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "buildStoryboard" --direction upstream --repo .
```

Report callers, processes and risk. HIGH, CRITICAL and UNKNOWN follow the GitNexus rule of outline section 5.

- [ ] **Step 2: Write the failing tests**

In `internal/storymaker/validate_test.go`, delete this whole line from `TestValidateNames`:

```go
		{"highlight beat", `{"id":"s1","props":[{"id":"a","item":"prop.lock"}],"beats":[{"do":"highlight","target":"a","style":"pulse"}]}`, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0"},
```

In the same file, replace:

```go
func TestValidateCanonicalisesInPlace(t *testing.T) {
```

with:

```go
// The highlight beat is played from phase 2 on, so Validate no longer marks it.
func TestValidateHighlightIsSupported(t *testing.T) {
	for _, style := range []string{"pulse", "outline", "arrow"} {
		issues := validateScenes(t, `{"id":"s1","props":[{"id":"a","item":"prop.lock"}],"beats":[{"do":"highlight","target":"a","style":"`+style+`"}]}`)
		if is := findIssue(issues, CodeNotSupportedYet, SeverityMinor, "/scenes/0/beats/0"); is != nil {
			t.Errorf("%s still reports not_supported_yet:%s", style, dump(issues))
		}
	}
}

func TestValidateCanonicalisesInPlace(t *testing.T) {
```

In `scripts/storymaker-tests/04-compile.mjs`, replace:

```js
    for (const j of [0, 1, 2, 3]) assert.equal(codes('/scenes/0/beats/' + j, 'not_supported_yet').length, 1, 'beat ' + j);
```

with:

```js
    assert.equal(codes('/scenes/0/beats/0', 'not_supported_yet').length, 0, 'highlight is played');
    for (const j of [1, 2, 3]) assert.equal(codes('/scenes/0/beats/' + j, 'not_supported_yet').length, 1, 'beat ' + j);
```

Line 105 of that file needs no edit, because the highlight beat keeps index 0.

In `scripts/storymaker-tests/05-render.mjs`, replace:

```js
check('rig state: a gesture in progress, the seated typing pose, the walk cycle in profile', () => {
```

with:

```js
check('highlight: pulse peaks at 1.08 on a prop and a text, outline strokes 6 design pixels in $accent, arrow draws above the entity', () => {
    const r3 = v => String(Math.round(v * 1000) / 1000);
    const onProp = style => SM.compile({ scenes: [{ id: 'a', transition: 'cut', props: [{ id: 'lock', item: 'prop.lock', at: 'center' }], beats: [{ do: 'highlight', target: 'lock', style }] }] }, {}, catalog).timeline;
    const onText = style => SM.compile({ scenes: [{ id: 'a', transition: 'cut', beats: [{ do: 'show', id: 't1', text: 'Hallo', entry: 'none' }, { do: 'highlight', target: 't1', style }] }] }, {}, catalog).timeline;
    // The beat starts at scene time 0 and lasts the default 0.8 s, so u = t / 0.8. sin(2 PI 2 u) peaks at u = 1/8, which is
    // frame 3 at 30 fps, and bottoms at u = 3/8, which is frame 9.
    let peak = 0;
    for (let i = 0; i <= 800; i++) peak = Math.max(peak, SM.render.pulseOf({ highlight: { style: 'pulse', u: i / 800 } }));
    near(peak, 1.08, 1e-9);
    // The lock: height 0.14 of 1080 at depth 0.8 + 0.25 x 18/26 (spot y 88 % on the plain stage floor 70..96), viewbox 100 x 120,
    // bottom centre on (960, 950.4).
    const depth = 0.8 + 0.25 * (18 / 26), boxH = 0.14 * 1080 * depth, boxW = boxH * 100 / 120, top = 0.88 * 1080 - boxH;
    const scaled = 'setTransform(' + r3(boxH / 120 * 1.08) + ',0,0,' + r3(boxH / 120 * 1.08) + ',';
    assert.ok(renderLog(onProp('pulse'), 3).some(l => l.startsWith(scaled)), 'the prop is drawn at 1.08 times its size at the peak');
    assert.ok(!renderLog(onProp('pulse'), 0).some(l => l.startsWith(scaled)), 'and at its own size at u = 0');
    // A text: the transform scales around the anchor (960, 302.4), which is 28 % of 1080 on the centre line.
    const textLog = renderLog(onText('pulse'), 3);
    const tr = textLog.indexOf('setTransform(1.08,0,0,1.08,' + r3(960 * -0.08) + ',' + r3(302.4 * -0.08) + ')');
    assert.ok(tr >= 0, 'drawText applies the pulse as a canvas transform');
    assert.ok(textLog.findIndex(l => l.startsWith('fillText(Hallo')) > tr, 'the text is drawn under it');
    // Outline: the strokes run in design pixels under the stage matrix, in the mini palette's accent.
    const outline = renderLog(onProp('outline'), 3);
    const at = outline.indexOf('strokeRect(' + [960 - boxW / 2, top, boxW, boxH].map(r3).join(',') + ')');
    assert.ok(at > 0, 'the stroke follows the entity box');
    assert.deepEqual(outline.slice(at - 2, at), ['set strokeStyle=#ff7a45', 'set lineWidth=6']);
    // A text box lives in canvas pixels, so at half size 6 design pixels are 3 canvas pixels.
    const half = renderLog(onText('outline'), 3, undefined, 960);
    assert.equal(half[half.findIndex(l => l.startsWith('strokeRect(')) - 1], 'set lineWidth=3');
    // Arrow: the tip sits 40 px above the box, plus 8 px at the sine's peak and minus 8 px at its low.
    const high = renderLog(onProp('arrow'), 3), tip = high.indexOf('moveTo(960,' + r3(top - 48) + ')');
    assert.ok(tip > high.findLastIndex(l => l.includes('M28,52')), 'the arrow is above the lock and drawn after it');
    assert.equal(high[tip - 2], 'set fillStyle=#ff7a45');
    assert.ok(renderLog(onProp('arrow'), 9).includes('moveTo(960,' + r3(top - 32) + ')'), 'the arrow bobs 8 px');
});

check('rig state: a gesture in progress, the seated typing pose, the walk cycle in profile', () => {
```

- [ ] **Step 3: Run the tests and see them fail**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker -run '^TestValidate(HighlightIsSupported|Names)$' -count=1
npm run test:storymaker
```

Expected:
- Go: `TestValidateHighlightIsSupported` fails for all three styles, because `phase1Beats` lacks `highlight`. `TestValidateNames` passes.
- Node: `FAIL highlight: ...`, because `SM.render.pulseOf` is not a function.
- Node: `FAIL compile: the phase-1 support matrix ...` with `highlight is played`.

- [ ] **Step 4: Implement**

In `internal/storymaker/model.go`, replace:

```go
	phase1Beats       = nameSet("enter", "exit", "walk", "move", "pose", "gesture", "emote", "look", "show", "hide", "camera", "wait")
```

with:

```go
	phase1Beats       = nameSet("enter", "exit", "walk", "move", "pose", "gesture", "emote", "look", "show", "hide", "highlight", "camera", "wait")
```

In `ui/js/storymaker/plan.js`, replace:

```js
    var SUPPORTED_BEATS = ['enter', 'exit', 'walk', 'move', 'pose', 'gesture', 'emote', 'look', 'show', 'hide', 'camera', 'wait'];
```

with:

```js
    var SUPPORTED_BEATS = ['enter', 'exit', 'walk', 'move', 'pose', 'gesture', 'emote', 'look', 'show', 'hide', 'highlight', 'camera', 'wait'];
```

In `ui/js/storymaker/beats.js`, replace this line, which M7 to M10 leave as it is:

```js
            camera: {
```

with:

```js
            highlight: {
                dur: function () { return SM.motion.DEFAULTS.highlight * SM.motion.factor(c.story.style); },
                run: function (b, j, t0, dur) {
                    var e = find(b.target);
                    if (!e) { warn('warning', beatPath(j, 'target'), 'unknown_name', 'Nothing named "' + b.target + '" to highlight.'); return; }
                    e.highlights.add(t0, t0 + dur, { style: b.style === 'outline' || b.style === 'arrow' ? b.style : 'pulse' });
                }
            },
            camera: {
```

In `ui/js/storymaker/scene.js`, make these five single-line replacements.

Old:

```js
        e.walks = new t.Intervals();
```

New:

```js
        e.walks = new t.Intervals();
        e.highlights = new t.Intervals();
```

Old:

```js
        e.layer = def.layer === 'back' ? 'back' : 'front';
```

New:

```js
        e.layer = def.layer === 'back' ? 'back' : 'front';
        e.highlights = new (T().Intervals)();
```

Old:

```js
        e.style = def.style || 'title';
```

New:

```js
        e.style = def.style || 'title';
        e.highlights = new (T().Intervals)();
```

Old:

```js
        return s;
```

New:

```js
        var h = e.highlights ? e.highlights.active(t) : null;
        if (h) s.highlight = { style: h.data.style, u: h.t1 > h.t0 ? (t - h.t0) / (h.t1 - h.t0) : 1 };
        return s;
```

Old:

```js
        return end;
```

New:

```js
        var marks = e.highlights ? e.highlights.list : [];
        for (var m = 0; m < marks.length; m++) if (marks[m].t1 > end) end = marks[m].t1;
        return end;
```

In `ui/js/storymaker/render.js`, make these six single-line replacements.

Old:

```js
    var SHADOW_ALPHA = 0.16;
```

New:

```js
    var SHADOW_ALPHA = 0.16;
    var PULSE_AMOUNT = 0.08, PULSE_CYCLES = 2, OUTLINE_PX = 6, ARROW_PX = 40, ARROW_BOB_PX = 8, ARROW_SIZE_PX = 28;
    var ACTOR_BOX_WIDTH = 0.42;   // of the actor's height: the rig has no width of its own
```

Old:

```js
    // drawText: tokens (optional) are the environment's colour tokens; they win over the palette, so a dark environment can carry
```

New:

```js
    // pulseOf(st): the scale factor of a running pulse highlight, else 1. It peaks at u = 1/8.
    function pulseOf(st) {
        var h = st.highlight;
        return h && h.style === 'pulse' ? 1 + PULSE_AMOUNT * Math.sin(2 * Math.PI * PULSE_CYCLES * h.u) : 1;
    }

    function layText(ctx, st, style, rc, x, align) {
        return SM.text.layout(ctx, st.text, style, { canvasH: rc.H, maxWidth: SM.text.maxWidthFor(rc.W, rc.safe, x, align), family: familyOf(rc, style) });
    }

    // entityBox(st, rc, stage): the box of an actor or a prop in design pixels, or null when its art is missing.
    function entityBox(st, rc, stage) {
        var depth = stage.depthScale(st.y, st.depth), h, w;
        if (st.type === 'prop') {
            var entry = rc.cat.byId[st.item], part = entry && rc.assets.props && rc.assets.props[entry.file];
            if (!part) return null;
            var vb = part.viewbox || [0, 0, 100, 100], anchor = entry.anchor || { x: 0.5, y: 1 };
            h = (entry.size && entry.size.h > 0 ? entry.size.h : 0.15) * SM.stage.DESIGN_H * depth * st.scale;
            w = h * (vb[2] || 100) / (vb[3] || 100);
            return { x: st.x - anchor.x * w, y: st.y - anchor.y * h, w: w, h: h };
        }
        var rig = rc.assets.rig;
        if (!rig) return null;
        h = R().scaleFor(rig, SM.stage.DESIGN_H, depth, st.scale, R().bodyOf(rig, st.params.body)) * (rig.units_height || 400);
        w = h * ACTOR_BOX_WIDTH;
        return { x: st.x - w / 2, y: st.y - h, w: w, h: h };
    }

    // textBox(ctx, st, rc, stage, bw): the box of a text in canvas pixels, padding included.
    function textBox(ctx, st, rc, stage, bw) {
        if (!st.text) return null;
        var style = rc.cat.textStyles[st.style] || rc.cat.textStyles.title || FALLBACK_TEXT_STYLE;
        var pos = stage.toCanvas(st.x, st.y, bw), align = style.align || 'center';
        var lay = layText(ctx, st, style, rc, pos.x, align);
        var left = align === 'left' ? pos.x : (align === 'right' ? pos.x - lay.width : pos.x - lay.width / 2);
        return { x: left - lay.pad, y: pos.y - lay.height / 2 - lay.pad, w: lay.width + lay.pad * 2, h: lay.height + lay.pad * 2 };
    }

    // drawHighlight: the outline or arrow of a running highlight, on top of every entity. Actors and props are drawn through
    // the camera matrix, so their sizes are design pixels; a text is in canvas pixels, so its sizes follow the stage scale.
    function drawHighlight(ctx, st, rc, stage, bw, Mv) {
        var h = st.highlight;
        if (!h || h.style === 'pulse' || !(st.alpha > 0)) return;
        var box, unit = 1;
        if (st.type === 'text') {
            ctx.setTransform(1, 0, 0, 1, 0, 0);
            box = textBox(ctx, st, rc, stage, bw);
            unit = stage.scale;
        } else {
            box = entityBox(st, rc, stage);
            setM(ctx, Mv);
        }
        if (!box) return;
        var accent = SM.parts.makeResolver([rc.palette])('$accent');
        ctx.globalAlpha = st.alpha > 1 ? 1 : st.alpha;
        if (h.style === 'outline') {
            ctx.strokeStyle = accent;
            ctx.lineWidth = OUTLINE_PX * unit;
            ctx.strokeRect(box.x, box.y, box.w, box.h);
            return;
        }
        var cx = box.x + box.w / 2, tip = box.y - (ARROW_PX + ARROW_BOB_PX * Math.sin(2 * Math.PI * PULSE_CYCLES * h.u)) * unit;
        ctx.fillStyle = accent;
        ctx.beginPath();
        ctx.moveTo(cx, tip);
        ctx.lineTo(cx - ARROW_SIZE_PX / 2 * unit, tip - ARROW_SIZE_PX * unit);
        ctx.lineTo(cx + ARROW_SIZE_PX / 2 * unit, tip - ARROW_SIZE_PX * unit);
        ctx.closePath();
        ctx.fill();
    }

    // drawText: tokens (optional) are the environment's colour tokens; they win over the palette, so a dark environment can carry
```

Old:

```js
        var lay = SM.text.layout(ctx, st.text, style, { canvasH: rc.H, maxWidth: SM.text.maxWidthFor(rc.W, rc.safe, pos.x, align), family: familyOf(rc, style) });
```

New:

```js
        var lay = layText(ctx, st, style, rc, pos.x, align), k = pulseOf(st);
        // a text reads no scale otherwise: a pulse scales it around its anchor
        if (k !== 1) ctx.setTransform(k, 0, 0, k, pos.x * (1 - k), pos.y * (1 - k));
```

Old:

```js
        SM.text.draw(ctx, lay, style, pos.x, pos.y, SM.parts.makeResolver([tokens, rc.palette]), st.alpha);
```

New:

```js
        SM.text.draw(ctx, lay, style, pos.x, pos.y, SM.parts.makeResolver([tokens, rc.palette]), st.alpha);
        if (k !== 1) ctx.setTransform(1, 0, 0, 1, 0, 0);
```

Old:

```js
        var states = sc.entities.map(function (e, k) { var s = SM.scene.evalEntity(e, t); s.k = k; return s; });
```

New:

```js
        var states = sc.entities.map(function (e, k) { var s = SM.scene.evalEntity(e, t); s.k = k; s.scale *= pulseOf(s); return s; });
```

Old:

```js
        states.filter(function (s) { return s.type === 'text'; }).forEach(function (s) { drawText(ctx, s, rc, stage, bw, tokens); });
```

New:

```js
        states.filter(function (s) { return s.type === 'text'; }).forEach(function (s) { drawText(ctx, s, rc, stage, bw, tokens); });
        states.forEach(function (s) { drawHighlight(ctx, s, rc, stage, bw, Mv); });
```

Also replace the export line. Old:

```js
        drawBackdrop: drawBackdrop, drawText: drawText, stageFor: stageFor
```

New:

```js
        drawBackdrop: drawBackdrop, drawText: drawText, stageFor: stageFor, pulseOf: pulseOf
```

- [ ] **Step 5: Run the tests and rebuild the bundles**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./internal/storymaker -run '^TestValidate(HighlightIsSupported|Names)$' -count=1
gofmt -l internal/storymaker
npm run build:ui
npm run check:ui
npm run test:storymaker
```

Expected: Go `ok`, `gofmt` prints nothing, `check:ui` is green, and the player suite ends in `0 failed`.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add internal/storymaker/model.go internal/storymaker/validate_test.go ui/js/storymaker/beats.js ui/js/storymaker/scene.js ui/js/storymaker/render.js ui/js/storymaker/plan.js ui/js/storymaker/player.bundle.js scripts/storymaker-tests/04-compile.mjs scripts/storymaker-tests/05-render.mjs
git commit -m "feat(storymaker): play the highlight beat" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task M13: Shrink to fit, balance and the layout memo
**Batch:** 2A  **Depends on:** M3
**Files:**
- Modify: `ui/js/storymaker/text.js`
- Modify: `ui/js/storymaker/render.js`
- Modify (generated by `npm run build:ui`): `ui/js/storymaker/player.bundle.js`
- Test: `scripts/storymaker-tests/02-stage-parts-text.mjs`
- Test: `scripts/storymaker-tests/05-render.mjs`

- [ ] **Step 1: Impact**

Run this from the main checkout for `layout` and for `layText`. `layout` has namesakes, so read the entry for `ui/js/storymaker/text.js`. `layText` is new since M12, so the stale index may not know it. Confirm it with a text search.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "layout" --direction upstream --repo .
```

Report callers, processes and risk. HIGH, CRITICAL and UNKNOWN follow the GitNexus rule of outline section 5.

- [ ] **Step 2: Write the failing tests**

In `scripts/storymaker-tests/02-stage-parts-text.mjs`, make these three replacements. The old pins assumed that an over-long title keeps its size. `fit` must come before `lay`, because the next assertion measures at the font `lay` leaves on `ctx`.

Old:

```js
    const lay = SM.text.layout(ctx, 'Ein ziemlich langer Titel, der umbrochen werden muss, weil er nicht passt', style, { canvasH: 1080, maxWidth: 900, family: 'Storymaker Default' });
```

New:

```js
    const fit = SM.text.layout(ctx, 'Hallo Welt', style, { canvasH: 1080, maxWidth: 900, family: 'Storymaker Default' });
    assert.equal(fit.px, Math.round(0.07 * 1080));
    assert.equal(fit.font, '700 76px "Storymaker Default", sans-serif');
    const lay = SM.text.layout(ctx, 'Ein ziemlich langer Titel, der umbrochen werden muss, weil er nicht passt', style, { canvasH: 1080, maxWidth: 900, family: 'Storymaker Default' });
```

Old:

```js
    assert.equal(lay.px, Math.round(0.07 * 1080));
```

New:

```js
    assert.ok(lay.px < fit.px, 'the long title shrinks');
```

Old:

```js
    assert.equal(lay.font, '700 76px "Storymaker Default", sans-serif');
```

New:

```js
    assert.equal(lay.overflow, false);
```

In the same file, replace:

```js
check('camera: framings, zoom and pan tweens, shake bursts decay and never leave the zoom range', () => {
```

with:

```js
check('text: an over-long title shrinks by 0.94 to its floor, reports shrunk, and overflow only when still truncated; balance never adds a line', () => {
    const measure = s => s.length * 5;
    const style = t.byId('text.title'), ctx = makeContext(null), opts = w => ({ canvasH: 1080, short: 1080, maxWidth: w, family: 'Storymaker Default' });
    const text = 'Ein ziemlich langer Titel, der umbrochen werden muss, weil er nicht passt';
    // 73 characters with the fake 0.5 em font. Two lines at width 900 need the 40 characters up to "umbrochen" on line 1, because
    // a break after "der" leaves 42 characters for line 2. A line holds 1800 / px characters, so px <= 45:
    // 76 x 0.94^8 = 46.33 is too wide, 76 x 0.94^9 = 43.55 fits.
    const fits = SM.text.layout(ctx, text, style, opts(900));
    near(fits.px, 76 * 0.94 ** 9, 1e-9);
    assert.equal(fits.shrunk, true); assert.equal(fits.overflow, false); assert.equal(fits.truncated, false);
    assert.equal(fits.lines.length, 2);
    // Floor: max(0.45 x 76, 0.026 x 1080) = max(34.2, 28.08) = 34.2. At width 200 the text never fits two lines.
    const stuck = SM.text.layout(ctx, text, style, opts(200));
    near(stuck.px, 34.2, 1e-9);
    assert.equal(stuck.shrunk, true); assert.equal(stuck.overflow, true); assert.equal(stuck.truncated, true);
    // A small style stops at 2.6 % of the short side: px0 = round(32.4) = 32, floor max(14.4, 28.08) = 28.08.
    near(SM.text.layout(ctx, text, { ...style, size: 0.03 }, opts(200)).px, 28.08, 1e-9);
    const small = SM.text.layout(ctx, 'Hallo Welt', style, opts(900));
    assert.equal(small.shrunk, false); assert.equal(small.overflow, false);
    assert.deepEqual(plain(SM.text.balance(measure, ['aaaa bbbb cccc', 'dddd'], 70, 2)), ['aaaa bbbb', 'cccc dddd']);
    assert.deepEqual(plain(SM.text.balance(measure, ['aaa bbb', 'ccc'], 40, 2)), ['aaa bbb', 'ccc'], 'no narrower width keeps the line count');
    assert.deepEqual(plain(SM.text.balance(measure, ['abcdefgh', 'ijklmnop'], 40, 5)), ['abcdefgh', 'ijklmnop'], 'a broken long word stays as it is');
    const named = { ...opts(900), canvasW: 1920, styleName: 'title' };
    assert.equal(SM.text.layout(ctx, 'Memo', style, named), SM.text.layout(ctx, 'Memo', style, named), 'the second call is a memo hit');
});

check('camera: framings, zoom and pan tweens, shake bursts decay and never leave the zoom range', () => {
```

In `scripts/storymaker-tests/05-render.mjs`, replace:

```js
import { plain, renderAssets, FakeCanvas, makeContext, digest } from '../storymaker-test-support.mjs';
```

with:

```js
import { plain, renderAssets, FakeCanvas, makeContext, digest, loadPlayer, coreOf } from '../storymaker-test-support.mjs';
```

In the same file, replace:

```js
check('rig state: a gesture in progress, the seated typing pose, the walk cycle in profile', () => {
```

with:

```js
check('layout memo: a sliding text renders the same at its frames in both orders', () => {
    // The slide moves the anchor and the anchor sets maxWidth, so the wrap changes during the entry. Two fresh realms start with
    // empty memos, so a key without maxWidth would freeze the first wrap each realm sees, and the two orders would differ.
    const slide = { scenes: [{ id: 'a', transition: 'cut', beats: [{ do: 'show', text: 'Ein kurzer Titel hier', at: 'left', entry: 'slide' }] }] };
    const run = frames => {
        const S = coreOf(loadPlayer()), tl = S.compile(slide, {}, catalog).timeline, out = {};
        for (const n of frames) {
            const cv = new FakeCanvas(tl.width, tl.height);
            S.renderFrame(tl, n, cv.ctx, renderAssets());
            out[n] = cv.ctx.log.map(l => l.replace(/canvas\d+/g, 'canvas'));
        }
        return out;
    };
    const frames = Array.from({ length: 16 }, (_, n) => n);
    const forward = run(frames), backward = run([...frames].reverse());
    for (const n of frames) assert.deepEqual(backward[n], forward[n], 'frame ' + n);
    // The title is 76 px, so with the fake 0.5 em font the text is 21 x 38 = 798 px wide. The anchor slides from 436 to 576. At frame 1
    // (u = 1/15) it is below 470, so maxWidth = 2 x (x - 96) is below 750: two lines. At frame 15 it is 576, so maxWidth is 960: one line.
    const lines = n => forward[n].filter(l => l.startsWith('fillText(')).length;
    assert.equal(lines(1), 2);
    assert.equal(lines(15), 1);
});

check('rig state: a gesture in progress, the seated typing pose, the walk cycle in profile', () => {
```

- [ ] **Step 3: Run the tests and see them fail**

```powershell
npm run test:storymaker
```

Expected:
- `FAIL text: wrapping with measureText ...`, because `lay.px` is still 76.
- `FAIL text: an over-long title shrinks ...`, because `layout` never shrinks.
- The `layout memo` check already passes, because no memo exists yet. It guards the key.

- [ ] **Step 4: Implement**

In `ui/js/storymaker/text.js`, make these eight single-line replacements.

Old:

```js
    var LINE_HEIGHT = 1.22;
```

New:

```js
    var LINE_HEIGHT = 1.22;
    var SHRINK_STEP = 0.94, SHRINK_FLOOR = 0.45, FONT_FLOOR = 0.026, BALANCE_STEPS = 8, MEMO_LIMIT = 512;
    var memo = Object.create(null), memoSize = 0;
```

Old:

```js
    // layout(ctx, text, style, opts): opts = {canvasH, maxWidth, family}. Sets ctx.font to measure. Returns the lines
```

New:

```js
    // balance(measure, lines, maxWidth, maxLines): the same text, re-wrapped at the narrowest width (by bisection) that does not
    // add a line. A word never breaks here, because the width stays at least as wide as the widest word.
    function balance(measure, lines, maxWidth, maxLines) {
        if (lines.length < 2) return lines;
        var joined = lines.join(' '), best = lines, hi = maxWidth, lo = 0;
        joined.split(' ').forEach(function (word) { var w = measure(word); if (w > lo) lo = w; });
        if (lo > maxWidth) return lines;
        for (var i = 0; i < BALANCE_STEPS; i++) {
            var mid = (lo + hi) / 2, tried = wrap(measure, joined, mid, maxLines);
            if (!tried.truncated && tried.lines.length <= lines.length) { best = tried.lines; hi = mid; } else lo = mid;
        }
        return best;
    }

    // compute(ctx, text, style, opts): opts = {canvasH, maxWidth, family, short?}; short is the short side of the canvas, canvasH
    // when absent. Sets ctx.font to measure. px steps by 0.94 while the wrap truncates, down to max(0.45 x px0, 0.026 x short), and
    // the lines are then balanced. shrunk says px moved; overflow says the text is still truncated. Returns the lines
```

Old:

```js
    function layout(ctx, text, style, opts) {
```

New:

```js
    function compute(ctx, text, style, opts) {
```

Old:

```js
        var px = Math.max(8, Math.round((style.size > 0 ? style.size : 0.05) * opts.canvasH));
```

New:

```js
        var px0 = Math.max(8, Math.round((style.size > 0 ? style.size : 0.05) * opts.canvasH));
        var floor = Math.max(SHRINK_FLOOR * px0, FONT_FLOOR * (opts.short > 0 ? opts.short : opts.canvasH));
        var px = px0;
```

Old:

```js
        var width = 0;
```

New:

```js
        while (wrapped.truncated && px > floor) {
            px = Math.max(floor, px * SHRINK_STEP);
            ctx.font = fontString(style.weight, px, opts.family);
            wrapped = wrap(measure, text, opts.maxWidth, style.max_lines);
        }
        // an explicit line break stays, and a truncated text has lost words that balance cannot bring back
        var lines = wrapped.truncated || /\n/.test(String(text)) ? wrapped.lines : balance(measure, wrapped.lines, opts.maxWidth, style.max_lines);
        var width = 0;
```

Old:

```js
        wrapped.lines.forEach(function (l) { var w = measure(l); if (w > width) width = w; });
```

New:

```js
        lines.forEach(function (l) { var w = measure(l); if (w > width) width = w; });
```

Old:

```js
            lines: wrapped.lines, truncated: wrapped.truncated, px: px, lineH: px * LINE_HEIGHT,
```

New:

```js
            lines: lines, truncated: wrapped.truncated, shrunk: px < px0, overflow: wrapped.truncated, px: px, lineH: px * LINE_HEIGHT,
```

Old:

```js
            width: width, height: wrapped.lines.length * px * LINE_HEIGHT, pad: pad, radius: radius, font: ctx.font
```

New:

```js
            width: width, height: lines.length * px * LINE_HEIGHT, pad: pad, radius: radius, font: ctx.font
```

Then two more replacements in the same file.

Old:

```js
    function roundRect(ctx, x, y, w, h, r) {
```

New:

```js
    // layout(ctx, text, style, opts): compute, memoised when opts names the style (opts.styleName, opts.canvasW). The key holds
    // maxWidth because the anchor moves during a slide or a move, and frame n must stay a pure function of n. A hit sets the same
    // font a miss leaves behind, so the canvas state does not depend on the history.
    function layout(ctx, text, style, opts) {
        if (!opts.styleName) return compute(ctx, text, style, opts);
        var key = JSON.stringify([String(text), opts.styleName, opts.canvasW, opts.canvasH, opts.family, opts.maxWidth]);
        var hit = memo[key];
        if (hit) { ctx.font = hit.font; return hit; }
        if (memoSize >= MEMO_LIMIT) { memo = Object.create(null); memoSize = 0; }
        memoSize++;
        return (memo[key] = compute(ctx, text, style, opts));
    }

    function roundRect(ctx, x, y, w, h, r) {
```

Old:

```js
    SM.text = { fontString: fontString, wrap: wrap, layout: layout, draw: draw, maxWidthFor: maxWidthFor, roundRect: roundRect };
```

New:

```js
    SM.text = { fontString: fontString, wrap: wrap, balance: balance, layout: layout, draw: draw, maxWidthFor: maxWidthFor, roundRect: roundRect };
```

In `ui/js/storymaker/render.js`, replace this line, quoted as M12 leaves it:

```js
        return SM.text.layout(ctx, st.text, style, { canvasH: rc.H, maxWidth: SM.text.maxWidthFor(rc.W, rc.safe, x, align), family: familyOf(rc, style) });
```

with:

```js
        return SM.text.layout(ctx, st.text, style, { canvasH: rc.H, canvasW: rc.W, short: Math.min(rc.W, rc.H), styleName: st.style || 'title', maxWidth: SM.text.maxWidthFor(rc.W, rc.safe, x, align), family: familyOf(rc, style) });
```

- [ ] **Step 5: Run the tests and rebuild the bundles**

```powershell
npm run build:ui
npm run check:ui
npm run test:storymaker
```

Expected: `check:ui` is green and the suite ends in `0 failed`.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/storymaker/text.js ui/js/storymaker/render.js ui/js/storymaker/player.bundle.js scripts/storymaker-tests/02-stage-parts-text.mjs scripts/storymaker-tests/05-render.mjs
git commit -m "feat(storymaker): shrink text to fit, balance lines, memoise layouts" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task M14: Static lint and its Go codes
**Batch:** 2A  **Depends on:** M10, M12, M13
**Files:**
- Create: `ui/js/storymaker/lint.js`
- Modify: `ui/js/storymaker/compile.js`
- Modify: `scripts/storymaker-bundle-parts.js`
- Modify: `internal/storymaker/model.go`
- Modify (generated by `npm run build:ui`): `ui/js/storymaker/player.bundle.js`
- Create: `scripts/storymaker-tests/14-lint.mjs`
- Test: `scripts/storymaker-tests/11-real-catalog.mjs`

- [ ] **Step 1: Impact**

Run this from the main checkout for `compile`, and read the entry for `ui/js/storymaker/compile.js`.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "compile" --direction upstream --repo .
```

Report callers, processes and risk. HIGH, CRITICAL and UNKNOWN follow the GitNexus rule of outline section 5.

- [ ] **Step 2: Write the failing tests**

Create `scripts/storymaker-tests/14-lint.mjs`:

```js
// Task M14: the static motion lint: a fixture per code, and a clean story that fires none.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog } = t;
const SM = t.core();
const LINT = ['line_unread', 'dead_stretch', 'first_frame_empty', 'text_too_small', 'fade_between_worlds', 'entry_monotony', 'transition_variety'];
const cast = [{ id: 'anna', character: 'char.office-woman-1' }];
const actors = [{ cast: 'anna', at: 'center' }];
// the mini catalog has only the title style
const styled = { ...catalog, entries: [...catalog.entries,
    { id: 'text.caption', kind: 'text_style', name: 'caption', font: 'default', size: 0.042, weight: 500, color: '$text', align: 'center', max_lines: 2 },
    { id: 'text.tiny', kind: 'text_style', name: 'tiny', font: 'default', size: 0.02, weight: 400, color: '$text', align: 'center', max_lines: 1 }] };
const lintOf = (story, cat) => plain(SM.compile(story, {}, cat || catalog).report.issues).filter(i => LINT.includes(i.code));
const named = (list, code) => list.filter(i => i.code === code);
const one = (list, code) => {
    const found = named(list, code);
    assert.equal(found.length, 1, code + ' in ' + JSON.stringify(list));
    return found[0];
};

check('lint: line_unread fires when a text leaves before its hold, at its landing', () => {
    // 4 words hold 0.5 + 0.3 x 4 = 1.7 s and the scene lasts 1 s. A fade show lasts 0.5 s and lands at 0.4377 x 0.5 s.
    const late = one(lintOf({ cast, scenes: [{ id: 'a', duration: 1, actors, beats: [{ do: 'show', text: 'vier kurze Worte hier', entry: 'fade' }] }] }), 'line_unread');
    assert.equal(late.severity, 'warning');
    assert.equal(late.path, '/scenes/0/beats/0');
    near(late.at_seconds, 0.4377 * 0.5, 1e-3);
    // a hide 0.2 s after a 2-word text, which holds 1.1 s
    const hidden = one(lintOf({ cast, scenes: [{ id: 'a', actors, beats: [{ do: 'show', id: 't', text: 'Hallo Welt', entry: 'none' }, { do: 'hide', target: 't', delay: 0.2 }] }] }), 'line_unread');
    assert.equal(hidden.path, '/scenes/0/beats/0');
    // Beside a title, a 1-word caption holds 1.2 s instead of 0.8 s. The 1-word title needs only 0.8 s of this 1 s scene.
    const second = one(lintOf({ cast, scenes: [{ id: 'a', duration: 1, actors, beats: [{ do: 'show', text: 'Titel', entry: 'none' }, { do: 'show', text: 'Wort', style: 'caption', at: { x: 50, y: 80 }, entry: 'none', with: 'prev' }] }] }, styled), 'line_unread');
    assert.equal(second.path, '/scenes/0/beats/1');
});

check('lint: dead_stretch fires on more than 3 s without a beat, and a wait is no beat', () => {
    const list = lintOf({ cast, scenes: [{ id: 'a', actors, beats: [{ do: 'wait', seconds: 4 }] }] });
    assert.deepEqual(list.map(i => i.code), ['dead_stretch']);
    assert.equal(list[0].severity, 'warning');
    assert.equal(list[0].path, '/scenes/0');
    near(list[0].at_seconds, 0, 1e-9);
});

check('lint: first_frame_empty fires when only an off-stage actor exists at T', () => {
    // the enter beat starts at scene time 0 with its alpha on, at x = -220, outside the visible slice
    const empty = one(lintOf({ cast, scenes: [{ id: 'a', beats: [{ do: 'enter', who: 'anna', from: 'left', to: 'center' }] }] }), 'first_frame_empty');
    assert.equal(empty.severity, 'warning');
    assert.equal(empty.path, '/scenes/0');
});

check('lint: text_too_small reads the style size against the short side', () => {
    const tiny = aspect => ({ cast, format: { aspect, fps: 30 }, scenes: [{ id: 'a', actors, beats: [{ do: 'show', text: 'Klein', style: 'tiny', entry: 'none' }] }] });
    // 16:9: 0.02 x 1080 / 1080 = 0.02 is below 0.026
    const small = one(lintOf(tiny('16:9'), styled), 'text_too_small');
    assert.equal(small.severity, 'warning');
    assert.equal(small.path, '/scenes/0/beats/0/style');
    // 9:16: 0.02 x 1920 / 1080 = 0.0356 clears it
    assert.equal(named(lintOf(tiny('9:16'), styled), 'text_too_small').length, 0);
});

check('lint: fade_between_worlds reads the transition the player plays, and a variant alone is no new world', () => {
    const worlds = (transition, second) => ({ cast, scenes: [{ id: 'a', environment: 'env.office', actors }, { id: 'b', transition, actors, ...second }] });
    const other = { environment: 'env.abstract-tech' };
    const stored = one(lintOf(worlds('fade', other)), 'fade_between_worlds');
    assert.equal(stored.severity, 'warning');
    assert.equal(stored.path, '/scenes/1/transition');
    assert.equal(named(lintOf(worlds('zoom', other)), 'fade_between_worlds').length, 1, 'zoom plays as fade');
    assert.equal(named(lintOf(worlds('wipe', other)), 'fade_between_worlds').length, 0);
    assert.equal(named(lintOf(worlds('fade', { environment: 'env.office', variant: 'night' })), 'fade_between_worlds').length, 0);
});

check('lint: entry_monotony needs at least 3 show beats with one entry', () => {
    const shows = entries => ({ cast, scenes: [{ id: 'a', actors, beats: entries.map((entry, k) => ({ do: 'show', text: 'Wort ' + k, entry, at: { x: 20 + 30 * k, y: 20 }, ...(k ? { with: 'prev' } : {}) })) }] });
    const same = one(lintOf(shows(['fade', 'fade', 'fade'])), 'entry_monotony');
    assert.equal(same.severity, 'minor');
    assert.equal(same.path, '/scenes');
    assert.equal(named(lintOf(shows(['fade', 'fade', 'pop'])), 'entry_monotony').length, 0);
    assert.equal(named(lintOf(shows(['fade', 'fade'])), 'entry_monotony').length, 0);
});

check('lint: transition_variety fires above 5 kinds', () => {
    const kinds = ['cut', 'fade', 'slide-left', 'wipe', 'zoom', 'iris'];
    const film = n => ({ cast, scenes: [{ id: 's0', actors }, ...kinds.slice(0, n).map((k, i) => ({ id: 's' + (i + 1), transition: k, actors }))] });
    const many = one(lintOf(film(6)), 'transition_variety');
    assert.equal(many.severity, 'minor');
    assert.equal(many.path, '/scenes');
    assert.equal(named(lintOf(film(5)), 'transition_variety').length, 0);
});

check('lint: a clean story fires none, and a text counts as activity until its hold ends', () => {
    // Scene a has no narration. Its 13-word title holds 0.5 + 0.3 x 13 = 4.4 s, and the beats end at 0.5 s, so without the
    // hold the stretch from 0.5 s to the scene end at about 4.63 s would be longer than 3 s.
    const clean = { cast, scenes: [
        { id: 'a', environment: 'env.office', actors, beats: [
            { do: 'show', text: 'Zwoelf Worte brauchen etwas mehr als vier Sekunden und bleiben so lange stehen', entry: 'fade' },
            { do: 'show', item: 'prop.lock', at: 'right', entry: 'pop', with: 'prev' }] },
        { id: 'b', environment: 'env.abstract-tech', transition: 'wipe', narration: 'Eine Firewall prueft jedes Paket.', actors: [{ cast: 'anna', at: 'left' }], beats: [{ do: 'walk', who: 'anna', to: 'center' }] }
    ] };
    assert.deepEqual(lintOf(clean), []);
});
```

In `scripts/storymaker-tests/11-real-catalog.mjs`, make these three replacements.

Old:

```js
check('real catalog: firewall, minimal and the cast sheet compile with only the issues of a silent phase 1', () => {
```

New:

```js
check('real catalog: firewall, minimal and the cast sheet compile with only the issues of a silent film and the lint they earn', () => {
    // firewall s2 fades into another environment, minimal shows nothing on its first frame, the cast sheet: see below
```

Old:

```js
    const allowed = new Set(['narration_missing', 'not_supported_yet', 'layout_unverified']);
```

New:

```js
    const allowed = new Set(['narration_missing', 'not_supported_yet', 'layout_unverified', 'fade_between_worlds', 'first_frame_empty', 'line_unread', 'entry_monotony']);
```

Old:

```js
    assert.equal(compileReal(castsheet).report.issues.length, 0, 'the cast sheet has no issue at all');
```

New:

```js
    // A 4-word label holds 1.7 s in the sheet's 1.2 s scenes, and every show has entry none, so only these two codes fire.
    assert.deepEqual([...new Set(plain(compileReal(castsheet).report.issues).map(i => i.code))].sort(), ['entry_monotony', 'line_unread']);
```

In the same file, replace:

```js
// ---- the remaining rules of the catalog: views, walking, bodies, spots with a depth, fonts -------------------------------
```

with:

```js
check('real catalog: title, caption and label clear the 2.6 % floor in every aspect', () => {
    // Sizes are fractions of the canvas height: 0.07, 0.042, 0.034. The label is tightest at 0.034 x 1080 / 1080 = 3.4 %, and 9:16 raises each by 1920 / 1080.
    for (const name of ['title', 'caption', 'label']) assert.ok(entry('text.' + name).size >= 0.026, name);
    for (const aspect of ['16:9', '9:16', '1:1']) {
        const out = compileReal({ format: { aspect, fps: 30 }, scenes: [{ id: 'a', beats: [
            { do: 'show', text: 'a', entry: 'none' }, { do: 'show', text: 'b', style: 'caption', at: { x: 50, y: 80 }, entry: 'none', with: 'prev' },
            { do: 'show', text: 'c', style: 'label', at: { x: 20, y: 50 }, entry: 'none', with: 'prev' }] }] });
        assert.deepEqual(plain(out.report.issues).filter(i => i.code === 'text_too_small'), [], aspect);
    }
});

// ---- the remaining rules of the catalog: views, walking, bodies, spots with a depth, fonts -------------------------------
```

- [ ] **Step 3: Run the tests and see them fail**

```powershell
npm run test:storymaker
```

Expected:
- `FAIL lint: line_unread ...`, and the same for dead_stretch, first_frame_empty, text_too_small, fade_between_worlds, entry_monotony and transition_variety, because compile emits no lint yet.
- `FAIL real catalog: firewall, minimal and the cast sheet ...`, because the cast sheet has no issues yet.
- `lint: a clean story` and the 2.6 % check pass already. They guard against false positives.

- [ ] **Step 4: Implement**

Create `ui/js/storymaker/lint.js`:

```js
// Storymaker player: the static motion lint of phase 2. It reads the trusted schedule of a storyboard scene (built.log) and the
// plans; nothing is sampled, so each finding is exact. A finding is a warning or a minor issue and never blocks a render. Code
// scenes have no trusted schedule and get no scene lint. Phase 3 moves this into checks.js. Classic script; publishes
// StorymakerCore.lint.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var DEAD_STRETCH_SECONDS = 3, FONT_FLOOR = 0.026, MONOTONY_SHOWS = 3, MAX_TRANSITION_KINDS = 5, EPSILON = 1e-6;
    var SECONDARY_STYLES = { caption: true, label: true };

    function seconds(v) { return v.toFixed(1); }

    function landingOf(entry) { return entry.landing === null || entry.landing === undefined ? entry.start : entry.landing; }

    // holdOf(text entity, hasTitle, factor) = {base, scaled}: a caption or label beside a title is a secondary line.
    function holdOf(e, hasTitle, factor) {
        return SM.motion.holdSeconds(e.text, { secondary: hasTitle && !!SECONDARY_STYLES[e.style], factor: factor });
    }

    // scene(c): c = {scene (compiled, with its frame range), index, plan, built, stage, cat, fmt, envChanged, factor}.
    function scene(c) {
        var out = [], sc = c.scene, index = c.index, log = c.built.log || [];
        if (sc.kind === 'code') return out;
        var T = c.plan.T, n = c.plan.narration, fps = c.fmt.fps, D = (sc.end_frame - sc.start_frame) / fps, vis = c.stage.vis;
        var scenePath = '/scenes/' + index;

        function warn(path, code, message, at) {
            out.push(SM.plan.issue('warning', sc.id, path, code, message, at === undefined ? null : { at_seconds: at }));
        }

        var shows = log.filter(function (l) { return l.kind === 'show' && l.tgt && l.tgt.type === 'text'; });
        var hasTitle = shows.some(function (l) { return l.tgt.style === 'title'; });
        var busy = [];

        shows.forEach(function (l) {
            var landing = landingOf(l), hold = holdOf(l.tgt, hasTitle, c.factor), until = D;
            log.forEach(function (o) { if (o.kind === 'hide' && o.tgt === l.tgt && o.start >= l.start && o.start < until) until = o.start; });
            // the scene end is a whole frame, at most half a frame below the hold that compile asked for
            if (until - landing < hold.base - 0.5 / fps - EPSILON) {
                warn(scenePath + '/beats/' + l.j, 'line_unread', 'This text is on screen for ' + seconds(until - landing) + ' s; it needs ' + seconds(hold.base) + ' s to be read.', landing);
            }
            busy.push([landing, landing + hold.scaled]);
        });

        log.forEach(function (l) { if (l.kind !== 'wait' && l.end > l.start) busy.push([l.start, l.end]); });
        if (n.seconds > 0) busy.push([n.start, n.start + n.seconds]);
        busy.sort(function (a, b) { return a[0] - b[0]; });
        var reached = T;
        function gap(to) {
            if (to - reached > DEAD_STRETCH_SECONDS + EPSILON) warn(scenePath, 'dead_stretch', 'Nothing moves for ' + seconds(to - reached) + ' s here.', reached);
        }
        busy.forEach(function (span) {
            gap(Math.min(span[0], D));
            if (span[1] > reached) reached = span[1];
        });
        gap(D);

        var seen = c.built.entities.some(function (e) {
            if (!(e.alpha.at(T) > 0)) return false;
            if (e.type !== 'actor') return true;
            var x = e.x.at(T), y = e.y.at(T);
            return x >= vis.x && x <= vis.x + vis.w && y >= vis.y && y <= vis.y + vis.h;
        });
        if (!seen) warn(scenePath, 'first_frame_empty', 'The first frame after the transition shows no actor, prop or text.', T);

        var flagged = {};
        shows.forEach(function (l) {
            var style = c.cat.textStyles[l.tgt.style];
            if (!style || flagged[l.tgt.style]) return;
            var size = style.size > 0 ? style.size : 0.05;
            if (size * c.fmt.height / Math.min(c.fmt.width, c.fmt.height) < FONT_FLOOR) {
                flagged[l.tgt.style] = true;
                warn(scenePath + '/beats/' + l.j + '/style', 'text_too_small', 'The text style "' + l.tgt.style + '" is below 2.6 % of the short side of the frame.');
            }
        });

        if (c.plan.transition === 'fade' && index > 0 && c.envChanged) {
            warn(scenePath + '/transition', 'fade_between_worlds', 'This scene fades into another environment. Use cut or wipe between worlds.');
        }
        return out;
    }

    // film(c): c = {scenesIn, plans}. The first scene plays no transition (plan.js transitionFor), so it does not count as a kind.
    function film(c) {
        var out = [], entries = [], kinds = {};
        c.scenesIn.forEach(function (sc, i) {
            if (i > 0 && c.plans[i]) kinds[c.plans[i].requested] = true;
            if (!sc || sc.kind === 'code') return;
            (sc.beats || []).forEach(function (b) {
                if (b && b.do === 'show' && !b.chart && !b.icon) entries.push(b.entry || 'fade');
            });
        });
        if (entries.length >= MONOTONY_SHOWS && entries.every(function (e) { return e === entries[0]; })) {
            out.push(SM.plan.issue('minor', null, '/scenes', 'entry_monotony', 'All ' + entries.length + ' show beats use the entry "' + entries[0] + '".'));
        }
        var count = Object.keys(kinds).length;
        if (count > MAX_TRANSITION_KINDS) {
            out.push(SM.plan.issue('minor', null, '/scenes', 'transition_variety', 'The film uses ' + count + ' transition kinds; ' + MAX_TRANSITION_KINDS + ' or fewer read as a style.'));
        }
        return out;
    }

    SM.lint = { scene: scene, film: film };
})(typeof globalThis !== 'undefined' ? globalThis : self);
```

In `scripts/storymaker-bundle-parts.js`, replace:

```js
  'ui/js/storymaker/checks.js',
```

with:

```js
  'ui/js/storymaker/lint.js',
  'ui/js/storymaker/checks.js',
```

In `ui/js/storymaker/compile.js`, make these four replacements. M10 leaves all four lines as they are.

Old:

```js
        var styleFont = story.style && story.style.font || 'default';
```

New:

```js
        var styleFont = story.style && story.style.font || 'default';
        var rhythmFactor = SM.motion.factor(story.style);
```

Old:

```js
            envPrev = envId;
```

New:

```js
            var envChanged = i > 0 && envId !== envPrev;
            envPrev = envId;
```

Old:

```js
            scenes.push(scene);
```

New:

```js
            scenes.push(scene);
            issues.push.apply(issues, SM.lint.scene({ scene: scene, index: i, plan: plan, built: built, stage: stage, cat: cat, fmt: fmt, envChanged: envChanged, factor: rhythmFactor }));
```

Old:

```js
        var frameCount = 0;
```

New:

```js
        issues.push.apply(issues, SM.lint.film({ scenesIn: scenesIn, plans: plans }));

        var frameCount = 0;
```

In `internal/storymaker/model.go`, replace:

```go
// Issue is one validation finding. Path is a JSON Pointer (RFC 6901) from the story root.
```

with:

```go
// Issue codes of the motion lint. Only the player's compile report emits them, never Validate.
const (
	CodeLineUnread        = "line_unread"         // warning
	CodeDeadStretch       = "dead_stretch"        // warning
	CodeFirstFrameEmpty   = "first_frame_empty"   // warning
	CodeTextTooSmall      = "text_too_small"      // warning
	CodeFadeBetweenWorlds = "fade_between_worlds" // warning
	CodeEntryMonotony     = "entry_monotony"      // minor
	CodeTransitionVariety = "transition_variety"  // minor
)

// Issue is one validation finding. Path is a JSON Pointer (RFC 6901) from the story root.
```

- [ ] **Step 5: Run the tests and rebuild the bundles**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
npm run build:ui
npm run check:ui
npm run test:storymaker
go test ./internal/storymaker -count=1
gofmt -l internal/storymaker
```

Expected: `check:ui` is green, the suite ends in `0 failed`, Go prints `ok`, and `gofmt` prints nothing.

- [ ] **Step 6: Commit**

`detect_changes` may answer PARTIAL for the new `ui/js/storymaker/lint.js`. Confirm with a text search for `SM.lint` and say so in the commit body.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/storymaker/lint.js ui/js/storymaker/compile.js ui/js/storymaker/player.bundle.js scripts/storymaker-bundle-parts.js internal/storymaker/model.go scripts/storymaker-tests/14-lint.mjs scripts/storymaker-tests/11-real-catalog.mjs
git commit -m "feat(storymaker): static motion lint in the compile report" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task M15: 2A strings: `ISSUE_CODES`, pinned list, 16 locales
**Batch:** 2A  **Depends on:** M14
**Files:**
- Modify: `ui/js/desktop/apps/storymaker-core.js`
- Modify: `ui/lang/storymaker/cs.json`
- Modify: `ui/lang/storymaker/da.json`
- Modify: `ui/lang/storymaker/de.json`
- Modify: `ui/lang/storymaker/el.json`
- Modify: `ui/lang/storymaker/en.json`
- Modify: `ui/lang/storymaker/es.json`
- Modify: `ui/lang/storymaker/fr.json`
- Modify: `ui/lang/storymaker/hi.json`
- Modify: `ui/lang/storymaker/it.json`
- Modify: `ui/lang/storymaker/ja.json`
- Modify: `ui/lang/storymaker/nl.json`
- Modify: `ui/lang/storymaker/no.json`
- Modify: `ui/lang/storymaker/pl.json`
- Modify: `ui/lang/storymaker/pt.json`
- Modify: `ui/lang/storymaker/sv.json`
- Modify: `ui/lang/storymaker/zh.json`
- Test: `ui/desktop_storymaker_guards_test.go`

- [ ] **Step 1: Impact**

Run this from the main checkout for `TestStorymakerAppUsesOnlyKeysItHas` and `issueTitle`. `ISSUE_CODES` is a plain list that the index may not resolve, so also confirm it with a text search.

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "issueTitle" --direction upstream --repo .
```

Report callers, processes and risk. HIGH, CRITICAL and UNKNOWN follow the GitNexus rule of outline section 5.

- [ ] **Step 2: Write the failing test**

In `ui/desktop_storymaker_guards_test.go`, make these three replacements.

Old:

```go
	// The list of issue codes is the contract's: nineteen codes, no more, no less.
```

New:

```go
	// The list of issue codes is the contract's: twenty-seven codes, no more, no less.
```

Old:

```go
	want := strings.Fields("invalid_json unknown_field schema_mismatch too_large limit_exceeded missing_field invalid_value duplicate_id unknown_name unknown_asset unknown_spot unknown_cast unknown_cue canonicalised not_supported_yet narration_missing code_error deadline layout_unverified")
```

New:

```go
	want := strings.Fields("invalid_json unknown_field schema_mismatch too_large limit_exceeded missing_field invalid_value duplicate_id unknown_name unknown_asset unknown_spot unknown_cast unknown_cue canonicalised not_supported_yet narration_missing code_error deadline layout_unverified line_unread dead_stretch first_frame_empty text_too_small fade_between_worlds entry_monotony transition_variety cue_late")
```

Old:

```go
		t.Errorf("ISSUE_CODES = %v, want the 19 codes of the contract", families["storymaker.issue."])
```

New:

```go
		t.Errorf("ISSUE_CODES = %v, want the 27 codes of the contract", families["storymaker.issue."])
```

- [ ] **Step 3: Run the test and see it fail**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^TestStorymakerAppUsesOnlyKeysItHas$' -count=1
```

Expected: FAIL with `ISSUE_CODES = [...], want the 27 codes of the contract`, because the list still has 19 codes.

- [ ] **Step 4: Implement**

In `ui/js/desktop/apps/storymaker-core.js`, replace:

```js
    const ISSUE_CODES = ['invalid_json', 'unknown_field', 'schema_mismatch', 'too_large', 'limit_exceeded', 'missing_field', 'invalid_value', 'duplicate_id', 'unknown_name', 'unknown_asset', 'unknown_spot', 'unknown_cast', 'unknown_cue', 'canonicalised', 'not_supported_yet', 'narration_missing', 'code_error', 'deadline', 'layout_unverified'];
```

with:

```js
    const ISSUE_CODES = ['invalid_json', 'unknown_field', 'schema_mismatch', 'too_large', 'limit_exceeded', 'missing_field', 'invalid_value', 'duplicate_id', 'unknown_name', 'unknown_asset', 'unknown_spot', 'unknown_cast', 'unknown_cue', 'canonicalised', 'not_supported_yet', 'narration_missing', 'code_error', 'deadline', 'layout_unverified', 'line_unread', 'dead_stretch', 'first_frame_empty', 'text_too_small', 'fade_between_worlds', 'entry_monotony', 'transition_variety', 'cue_late'];
```

Write this script with the Write tool as `m15-locales.mjs` in your session scratchpad. It puts the new keys right after `storymaker.issue.layout_unverified` and changes nothing else.

```js
// Adds the eight 2A issue headings to ui/lang/storymaker/<lang>.json. Run from the worktree root.
import fs from 'node:fs';

const CODES = ['line_unread', 'dead_stretch', 'first_frame_empty', 'text_too_small', 'fade_between_worlds', 'entry_monotony', 'transition_variety', 'cue_late'];
const TEXTS = {
    en: ['Text not on screen long enough', 'Nothing happens for too long', 'Empty first frame', 'Text too small', 'Fade between different places', 'All entries look the same', 'Too many transition kinds', 'Cue lands late'],
    cs: ['Text je vidět příliš krátce', 'Příliš dlouho se nic neděje', 'První snímek je prázdný', 'Text je příliš malý', 'Prolínání mezi různými místy', 'Všechny nástupy vypadají stejně', 'Příliš mnoho druhů přechodů', 'Značka přichází pozdě'],
    da: ['Teksten vises for kort', 'Der sker ikke noget for længe', 'Første billede er tomt', 'Teksten er for lille', 'Overtoning mellem forskellige steder', 'Alle indgange ser ens ud', 'For mange slags overgange', 'Markeringen kommer for sent'],
    de: ['Text zu kurz sichtbar', 'Zu lange passiert nichts', 'Erstes Bild ist leer', 'Text zu klein', 'Überblendung zwischen verschiedenen Orten', 'Alle Auftritte sehen gleich aus', 'Zu viele Übergangsarten', 'Einsatz kommt zu spät'],
    el: ['Το κείμενο φαίνεται για πολύ λίγο', 'Δεν συμβαίνει τίποτα για πολλή ώρα', 'Το πρώτο καρέ είναι κενό', 'Το κείμενο είναι πολύ μικρό', 'Σβήσιμο ανάμεσα σε διαφορετικούς χώρους', 'Όλες οι είσοδοι μοιάζουν ίδιες', 'Πάρα πολλά είδη μεταβάσεων', 'Το σημείο καθυστερεί'],
    es: ['El texto se ve muy poco tiempo', 'No pasa nada durante demasiado tiempo', 'Primer fotograma vacío', 'Texto demasiado pequeño', 'Fundido entre lugares distintos', 'Todas las entradas son iguales', 'Demasiados tipos de transición', 'La marca llega tarde'],
    fr: ['Texte affiché trop peu de temps', 'Rien ne se passe trop longtemps', 'Première image vide', 'Texte trop petit', 'Fondu entre des lieux différents', 'Toutes les entrées se ressemblent', 'Trop de types de transition', 'Le repère arrive en retard'],
    hi: ['टेक्स्ट बहुत कम समय दिखता है', 'बहुत देर तक कुछ नहीं होता', 'पहला फ़्रेम खाली है', 'टेक्स्ट बहुत छोटा है', 'अलग-अलग जगहों के बीच फ़ेड', 'सभी एंट्री एक जैसी हैं', 'बहुत तरह के ट्रांज़िशन', 'संकेत देर से आता है'],
    it: ['Testo visibile troppo poco', 'Non succede nulla troppo a lungo', 'Primo fotogramma vuoto', 'Testo troppo piccolo', 'Dissolvenza tra luoghi diversi', 'Tutte le entrate sono uguali', 'Troppi tipi di transizione', 'Il segnale arriva in ritardo'],
    ja: ['テキストの表示時間が短すぎます', '何も起こらない時間が長すぎます', '最初のフレームが空です', 'テキストが小さすぎます', '異なる場所の間のフェード', '登場の仕方がすべて同じです', 'トランジションの種類が多すぎます', 'キューが遅れて着地します'],
    nl: ['Tekst te kort in beeld', 'Er gebeurt te lang niets', 'Eerste beeld is leeg', 'Tekst te klein', 'Overvloeier tussen verschillende plekken', 'Alle entrees zien er hetzelfde uit', 'Te veel soorten overgangen', 'Cue komt te laat'],
    no: ['Teksten vises for kort', 'Det skjer ingenting for lenge', 'Første bilde er tomt', 'Teksten er for liten', 'Overtoning mellom ulike steder', 'Alle innganger ser like ut', 'For mange typer overganger', 'Markeringen kommer for sent'],
    pl: ['Tekst widoczny zbyt krótko', 'Zbyt długo nic się nie dzieje', 'Pierwsza klatka jest pusta', 'Tekst jest za mały', 'Przenikanie między różnymi miejscami', 'Wszystkie wejścia wyglądają tak samo', 'Zbyt wiele rodzajów przejść', 'Znacznik wypada za późno'],
    pt: ['Texto visível por muito pouco tempo', 'Nada acontece durante demasiado tempo', 'Primeiro fotograma vazio', 'Texto demasiado pequeno', 'Desvanecimento entre locais diferentes', 'Todas as entradas são iguais', 'Demasiados tipos de transição', 'A marca chega atrasada'],
    sv: ['Texten visas för kort tid', 'Inget händer på för länge', 'Första bilden är tom', 'Texten är för liten', 'Övertoning mellan olika platser', 'Alla entréer ser likadana ut', 'För många sorters övergångar', 'Markeringen kommer för sent'],
    zh: ['文字显示时间太短', '太长时间没有变化', '第一帧为空', '文字太小', '不同场景之间使用了淡入淡出', '所有入场方式都相同', '转场类型过多', '提示点落后']
};

for (const [lang, texts] of Object.entries(TEXTS)) {
    const file = 'ui/lang/storymaker/' + lang + '.json';
    const source = fs.readFileSync(file, 'utf8');
    if (source.includes('storymaker.issue.line_unread')) continue;
    const eol = source.includes('\r\n') ? '\r\n' : '\n';
    const anchor = source.match(/^  "storymaker\.issue\.layout_unverified": .*$/m);
    if (!anchor) throw new Error(file + ' has no layout_unverified line');
    const added = CODES.map((code, i) => '  "storymaker.issue.' + code + '": ' + JSON.stringify(texts[i]) + ',').join(eol);
    const out = source.replace(anchor[0], () => anchor[0] + eol + added);
    JSON.parse(out);
    fs.writeFileSync(file, out);
    console.log(lang, 'ok');
}
```

Run it from the worktree root, with your session scratchpad path in place of the folder:

```powershell
node C:/path/to/your/session/scratchpad/m15-locales.mjs
```

Expected: 16 lines ending in `ok`.

- [ ] **Step 5: Run the tests**

```powershell
$env:GOCACHE = 'D:\dev-cache\go-build'
$env:GOTMPDIR = 'D:\tmp\go-build'
go test ./ui -run '^TestStorymaker(AppUsesOnlyKeysItHas|LocalesCoverAllSixteenLanguages|AppTranslationsAreTranslated)$' -v -count=1
```

Expected: three `--- PASS` lines. Count them, because the run is evidence.

- [ ] **Step 6: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add ui/js/desktop/apps/storymaker-core.js ui/desktop_storymaker_guards_test.go ui/lang/storymaker/cs.json ui/lang/storymaker/da.json ui/lang/storymaker/de.json ui/lang/storymaker/el.json ui/lang/storymaker/en.json ui/lang/storymaker/es.json ui/lang/storymaker/fr.json ui/lang/storymaker/hi.json ui/lang/storymaker/it.json ui/lang/storymaker/ja.json ui/lang/storymaker/nl.json ui/lang/storymaker/no.json ui/lang/storymaker/pl.json ui/lang/storymaker/pt.json ui/lang/storymaker/sv.json ui/lang/storymaker/zh.json
git commit -m "feat(storymaker): issue headings for the motion lint in 16 locales" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task M16: Contract slice, child docs, lint rates
**Batch:** 2A  **Depends on:** M15
**Files:**
- Modify: `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
- Modify: `ui/js/storymaker/AGENTS.md`
- Modify: `internal/storymaker/AGENTS.md`
- Modify: `documentation/storymaker.md`
- Create: `docs/superpowers/plans/2026-11-04-storymaker-phase2/measured.md`

This is a docs task, so it has no failing test. Each edit matches a single line by its quoted text. Line 553 of the contract stays, because 2A adds no host part.

- [ ] **Step 1: Edit the contract header and section 2**

In `docs/superpowers/specs/2026-11-01-storymaker-contract.md`, make these replacements.

Old:

```markdown
# Storymaker phase 1: interface contract
```

New:

```markdown
# Storymaker: interface contract
```

Old:

```markdown
**Status.** Phase 1 (foundation). Matches the phase 1 plan `docs/superpowers/plans/2026-11-01-storymaker-phase1-foundation.md`. The parent document is the design spec `docs/superpowers/specs/2026-11-01-storymaker-design.md`. Where the spec and this document differ for phase 1, this document is the reference.
```

New:

```markdown
**Status.** Phases 1 and 2. Matches the phase 1 and phase 2 plans. The parent document is the design spec `docs/superpowers/specs/2026-11-01-storymaker-design.md`. Where the spec and this document differ for phases 1 and 2, this document is the reference.
```

Old:

```text
Style       {palette:string, font:string, captions:bool}
```

New:

```text
Style       {palette:string, font:string, captions:bool, rhythm?:"calm"|"standard"|"lively"}
```

Old:

```markdown
- `kind` is not defaulted. An absent kind is a storyboard scene.
```

New:

```markdown
- `kind` is not defaulted. An absent kind is a storyboard scene.
- `style.rhythm` is not written when absent.
```

Old:

```markdown
### Phase 1 support
```

New:

```markdown
### Phase 2 support
```

Old:

```markdown
| Item | Phase 1 behaviour |
```

New:

```markdown
| Item | Behaviour |
```

Old:

```markdown
| Beats the player plays | enter, exit, walk, move, pose, gesture, emote, look, show (with `item` or `text` only), hide, camera, wait |
```

New:

```markdown
| Beats the player plays | enter, exit, walk, move, pose, gesture, emote, look, show (with `item` or `text` only), hide, highlight, camera, wait |
```

Old:

```markdown
| Beats accepted and ignored | say, highlight, sfx, chart, and show with `icon` or `chart`. Each gets a minor `not_supported_yet` issue. |
```

New:

```markdown
| Beats accepted and ignored | say, sfx, chart, and show with `icon` or `chart`. Each gets a minor `not_supported_yet` issue. |
```

Old:

```markdown
| `/scenes/<i>/transition` | `"fade"`; scene 1 gets `"cut"` |
```

New:

```markdown
| `/scenes/<i>/transition` | `cut` on scene 1; `wipe` when the environment differs from the previous scene's; else `fade` |
```

Old:

```markdown
| `not_supported_yet` | minor | Go Validate and player: see "Phase 1 support" |
```

New:

```markdown
| `not_supported_yet` | minor | Go Validate and player: see "Phase 2 support" |
```

Old:

```markdown
| `layout_unverified` | minor | Player compile report: one per code scene (layout inside code is not checked) |
```

New:

```markdown
| `layout_unverified` | minor | Player compile report: one per code scene (layout inside code is not checked) |
| `line_unread`, `dead_stretch`, `first_frame_empty`, `text_too_small`, `fade_between_worlds`, `cue_late` | warning | Player compile report, storyboard scenes |
| `entry_monotony`, `transition_variety` | minor | Player compile report |
```

- [ ] **Step 2: Edit the contract sections 4 and 6**

Old:

```text
// Code<CamelCase> constants for the 19 issue codes of section 2
```

New:

```text
// Code<CamelCase> constants for the 27 issue codes of section 2
```

Old:

```markdown
`ui/js/storymaker/player.bundle.js` (21 parts) runs in workers
```

New:

```markdown
`ui/js/storymaker/player.bundle.js` (25 parts, with `seed.js`, `ranges.js`, `motion.js` and `lint.js` from phase 2) runs in workers
```

Old:

```markdown
(with `--check`). Node runs the tests with a vm-based runner.
```

New:

```markdown
(with `--check`). `node scripts/build-storymaker-ranges.mjs` generates `ui/js/storymaker/ranges.js` and `internal/storymaker/ranges_gen.go`; with `--check` it fails on a stale output. Node runs the tests with a vm-based runner.
```

Old:

```markdown
- Scene time 0 is the start of the entry transition. Narration starts when the transition ends, and so do the beats.
```

New:

```markdown
- Scene time 0 is the start of the entry transition. Narration starts when the transition ends. A beat without a cue starts there too; a cued arrival may start earlier, down to scene time 0.
```

Old:

```markdown
- Automatic scene length is the largest of: the end of the narration plus 0.4 s, the end of the last beat, and 1.0 s for a scene without narration.
```

New:

```markdown
- Automatic scene length is the largest of: narration end plus 0.4 s, the end of the last beat, the end of the last text hold, and 1.0 s for a scene without speech.
```

Old:

```markdown
- Beat defaults: enter and exit 0.6 s (enter walks in from the left or right edge, or from the `door` spot); walk takes its distance at `cycle.walk.speed`, at least 0.3 s; move 0.8 s; pose 0.3 s; gesture = catalog duration × `count`; emote 0.25 s; look 0.3 s; show 0.5 s (0 with `entry: "none"`); hide 0.3 s; camera 0.8 s (shake 0.5 s); wait `seconds` (default 1).
```

New:

```markdown
- Beat defaults, scaled by the rhythm factor unless explicit: enter and exit 0.6 s (they walk); walk distance at `cycle.walk.speed`, at least 0.3 s; move 0.8 s; pose 0.3 s; gesture = catalog duration × `count`; emote 0.25 s; look 0.3 s; show 0.5 s (0 with `entry: "none"`); hide 0.35 s; camera 0.8 s (shake 0.5 s); highlight 0.8 s; sfx 0. `wait` is `seconds` (default 1), never scaled.
```

Old:

```markdown
- `with: "prev"` starts together with the previous beat. A `cue` beat re-anchors the cursor at its own end. `delay` shifts the start.
```

New:

```markdown
- Scheduling, in beat order. State: `cursor = T`, `prev`, `anchor`, the last end per target instance, `beatsEnd`. `f` is the rhythm factor.
  1. A non-object is skipped. An unknown kind, `chart`, or a show with `icon` or `chart` gets its minor issue and leaves the state untouched.
  2. A beat's target is the resolved instance of `who`, else `target`. A show's target is the instance it creates. `wait`, `sfx`, `chart`, a camera without a target, and a string that resolves to no instance (a camera on a spot) have none. Targets compare as instances, never as strings.
  3. `floor` is the end of the last beat on the same target, else 0.
  4. `dur`: an explicit `duration` wins. `wait` is `seconds`, else 1, never scaled. Otherwise it is the default above times `f`. `walk` is `max(0.3, distance ÷ speed) × f`, with the distance from the target's position at `max(floor, 0)`.
  5. An arrival (`enter`, `walk`, `move`, a show with an entry other than `none`, camera `focus`, camera `zoom` with a target) has `lead = leadFraction(ease) × dur`: the share of the duration until the ease first reaches 0.9, or for the pop spring until it first crosses 1. Every other beat has lead 0.
  6. Start time `t`: (a) a cue that resolves gives `t = cue time + delay - lead`, and a cue beats `with`; (b) else `with: "prev"` with a previous beat gives `t = prev.start + delay`; (c) else, when the previous beat and this beat both have a target and the targets differ, `t = anchor.start + 0.4 × anchor.dur + delay`; (d) else `t = cursor + delay`. An unresolved cue warns `unknown_cue` and falls through to b, c or d.
  7. Rules a, c, d: `start = max(0, floor, t)`. Rule b: `start = max(0, t)`, with no floor, so a `with: "prev"` beat stays together with the previous beat, also on the same target. A cued beat held back by its target gets a `cue_late` warning at `<beat>/cue` with `at_seconds = start + lead`. A clamp at 0 never warns.
  8. `landing = start + lead` for an arrival, else none. `end = start + dur`. `cursor`: rule b gives `max(cursor, end)`, the others give `end`. `anchor`: under rule b it moves only when `end` reaches the anchor's end, otherwise it is this beat. `prev` is always this beat.
  9. After the loop, `holdsEnd` is the largest `landing + scaled hold` of a text that no `hide` removes. A show with entry `none` has no landing and holds from its `start`. A hold is `0.5 + 0.3 × words`, at least 1.2 s for a caption or label in a scene that shows a title, and `max(1, f)` times that when scaled.
```

- [ ] **Step 3: Edit the child docs**

In `ui/js/storymaker/AGENTS.md`, make these replacements.

Old:

```markdown
- Support matrix of phase 1: beats enter exit walk move pose gesture emote look show (item and text) hide camera wait; transitions
```

New:

```markdown
- Support matrix: beats enter exit walk move pose gesture emote look show (item and text) hide highlight camera wait; transitions
```

Old:

```markdown
  cut fade slide-left wipe (others fall back to fade with a `not_supported_yet` note); everything else the validator accepts is ignored.
```

New:

```markdown
  cut fade slide-left wipe (others fall back to fade with a `not_supported_yet` note); everything else the validator accepts is ignored.
- Motion: `motion.js` holds rhythm factors, beat defaults, lead fractions and holds; `beats.js` runs the scheduler; `lint.js` holds the
  static lint, which reads `built.log` and never blocks a render. Code-scene API defaults are not scaled.
- Text: compile never measures. Shrink to fit and balance run in `text.layout`. The layout memo is keyed on text, style name, canvas
  size, family and `maxWidth`, because `maxWidth` follows the anchor, so frame n stays a pure function of n.
- Tables and seeds: `ranges.js` is generated by `scripts/build-storymaker-ranges.mjs`; never edit it by hand. No `Intl.Segmenter`,
  `\p{...}` or `normalize` in compile or the wrap. Every seed comes from strings (`seed.js`), because no project seed exists.
```

Old:

```markdown
- `npm run test:storymaker` (Node, vm based, about 8 s), `npm run build:ui`, `npm run check:ui`.
```

New:

```markdown
- `npm run test:storymaker` (Node, vm based, about 8 s), `npm run build:ui`, `npm run check:ui`, `node scripts/build-storymaker-ranges.mjs --check`.
```

In `internal/storymaker/AGENTS.md`, make these replacements.

Old:

```markdown
  `BundleCodeScene`; `project.go` + `memstore.go` + `slug.go` storage, `MemStore` (test helper), `Slugify`.
```

New:

```markdown
  `BundleCodeScene`; `project.go` + `memstore.go` + `slug.go` storage, `MemStore` (test helper), `Slugify`. `ranges_gen.go` is
  generated by `scripts/build-storymaker-ranges.mjs`; never edit it by hand.
```

Old:

```markdown
  the app localises them as `storymaker.issue.<code>`. Model-facing answers carry at most `MaxModelIssues` (`CapIssues`).
```

New:

```markdown
  the app localises them as `storymaker.issue.<code>`. Model-facing answers carry at most `MaxModelIssues` (`CapIssues`).
  The motion lint codes and `cue_late` are constants too, but only the player's compile report emits them; `Validate` never does.
```

Old:

```markdown
  transition out of "not supported yet" there when the player gains it.
```

New:

```markdown
  transition out of "not supported yet" there when the player gains it. `highlight` moved in phase 2. `style.rhythm` is `calm`,
  `standard`, `lively` or absent, and Go never writes a default.
```

Old:

```markdown
  `ease`) is never defaulted: the player's compile step owns it. It is idempotent.
```

New:

```markdown
  `ease`) is never defaulted: the player's compile step owns it. It is idempotent. The default transition is `cut` on scene 1,
  `wipe` when the environment differs from the previous scene's, else `fade`; a stored value stays.
```

- [ ] **Step 4: Edit the user page**

In `documentation/storymaker.md`, replace:

```markdown
## Not in phase 1
```

with:

```markdown
## Motion

The player times a story by motion rules. They also change how existing projects play: a text stays on screen long enough to be
read, a hide takes 0.35 s, and a beat on another actor starts while the previous beat is still running.

- **Rhythm.** `style.rhythm` is `calm`, `standard` or `lively`. Edit it in the Code tab. It scales default beat durations and
  lengthens text holds. It never changes an explicit `duration`, `delay` or `wait`.
- **Cues.** An arrival with a cue lands on its narration marker, so it starts a little earlier. If its actor is still busy, it lands
  late and the issues list says so.
- **Text.** A text that does not fit shrinks in steps, at most to 45 % of its size and never below 2.6 % of the short side of the
  frame. Its lines are then balanced.
- **Highlight.** The `highlight` beat draws attention to an actor, a prop or a text. Its style is `pulse`, `outline` or `arrow`.
- **Default transition.** A new scene gets a wipe when its environment differs from the previous scene's, and a fade otherwise.
  Scenes you already saved keep their transition.

The issues list also shows motion hints. A hint never blocks an export.

| Code | Meaning |
| --- | --- |
| `line_unread` | A text leaves before it can be read. |
| `dead_stretch` | Nothing moves and nothing is spoken for more than 3 s. |
| `first_frame_empty` | The first frame after the transition shows no actor, prop or text. |
| `text_too_small` | A text style is below 2.6 % of the short side of the frame. |
| `fade_between_worlds` | A scene fades into another environment. Use a cut or a wipe. |
| `cue_late` | An arrival lands after its marker, because its actor is still busy. |
| `entry_monotony` | Three or more show beats, all with the same entry. |
| `transition_variety` | More than five transition kinds in one film. |

## Not in phase 1
```

- [ ] **Step 5: Record the lint rates**

Write this script with the Write tool as `m16-rates.mjs` in your session scratchpad. It creates the plan's Measured list, `docs/superpowers/plans/2026-11-04-storymaker-phase2/measured.md`, with the real numbers, so none are written by hand. No earlier task writes this file, and E1j appends to it in batch 2B.

```js
// Creates the plan's Measured list with the rate of each motion code on the fixture stories. Usage: node m16-rates.mjs <worktree root>
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const root = path.resolve(process.argv[2] || '.');
const support = await import(pathToFileURL(path.join(root, 'scripts/storymaker-test-support.mjs')).href);
const SM = support.coreOf(support.loadPlayer());
const catalog = support.realCatalog();
const stories = { firewall: support.story('firewall'), minimal: support.story('minimal'), castsheet: support.playerStory('castsheet') };
const CODES = ['line_unread', 'dead_stretch', 'first_frame_empty', 'text_too_small', 'fade_between_worlds', 'entry_monotony', 'transition_variety', 'cue_late'];
const total = Object.fromEntries(CODES.map(code => [code, 0]));
const rows = [];
let scenes = 0, cued = 0;
for (const [name, story] of Object.entries(stories)) {
    const codeScenes = {};
    for (const s of story.scenes) {
        if (s.kind === 'code') codeScenes[s.id] = { duration: 5 };
        for (const b of s.beats || []) if (b && b.cue) cued++;
    }
    scenes += story.scenes.length;
    const counts = Object.fromEntries(CODES.map(code => [code, 0]));
    for (const issue of SM.compile(story, {}, catalog, { codeScenes }).report.issues) {
        if (issue.code in counts) { counts[issue.code]++; total[issue.code]++; }
    }
    rows.push('| ' + name + ' | ' + story.scenes.length + ' | ' + CODES.map(code => counts[code]).join(' | ') + ' |');
}
const lines = [
    '', '## Motion lint rates (2A, real catalog, no audio)', '',
    '| Story | Scenes | ' + CODES.join(' | ') + ' |',
    '| --- | --- | ' + CODES.map(() => '---').join(' | ') + ' |',
    ...rows,
    '| all | ' + scenes + ' | ' + CODES.map(code => total[code]).join(' | ') + ' |', '',
    'Rate per scene: ' + CODES.map(code => code + ' ' + (total[code] / scenes).toFixed(3)).join(', ') + '. Cued beats: ' + cued + ', with cue_late: ' + total.cue_late + '.', ''
];
const file = path.join(root, 'docs/superpowers/plans/2026-11-04-storymaker-phase2/measured.md');
fs.mkdirSync(path.dirname(file), { recursive: true });
fs.writeFileSync(file, '# Storymaker phase 2: measured\n');
fs.appendFileSync(file, lines.join('\n'));
console.log(lines.join('\n'));
```

Run it from the worktree root, with your session scratchpad path in place of the folder:

```powershell
node C:/path/to/your/session/scratchpad/m16-rates.mjs .
```

Expected: a table with three stories and an `all` row. Each hand-checked code fires on its fixture: `fade_between_worlds` on firewall, `first_frame_empty` on minimal, and `line_unread` and `entry_monotony` on castsheet.

- [ ] **Step 6: Check every cited path**

```powershell
node -e "const fs=require('fs');const cited=['docs/superpowers/specs/2026-11-01-storymaker-contract.md','docs/superpowers/specs/2026-11-01-storymaker-design.md','docs/superpowers/plans/2026-11-04-storymaker-phase2/measured.md','ui/js/storymaker/seed.js','ui/js/storymaker/ranges.js','ui/js/storymaker/motion.js','ui/js/storymaker/lint.js','ui/js/storymaker/text.js','internal/storymaker/ranges_gen.go','internal/storymaker/model.go','scripts/build-storymaker-ranges.mjs','scripts/storymaker-ucd-16.0.0.txt','scripts/storymaker-tests/14-lint.mjs','documentation/storymaker.md'];const gone=cited.filter(p=>!fs.existsSync(p));if(gone.length){console.error('missing',gone);process.exit(1)}console.log('ok',cited.length)"
```

Expected: `ok 14`.

- [ ] **Step 7: Commit**

```powershell
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs detect-changes --scope all --repo C:/Users/Andi/Documents/repo/AuraGo # run alone, read the answer before git add
git add -f docs/superpowers/specs/2026-11-01-storymaker-contract.md docs/superpowers/plans/2026-11-04-storymaker-phase2/measured.md ui/js/storymaker/AGENTS.md internal/storymaker/AGENTS.md
git add documentation/storymaker.md
git commit -m "docs(storymaker): contract slice, child docs and lint rates for batch 2A" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

{% endraw %}
