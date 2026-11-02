## Part E: Desktop app, registration, locales, config UI, docs

Inputs: part A (`internal/storymaker`, the config section), part C (the player under `ui/js/storymaker`, its bundles) and part D (the starter catalog under `ui/img/storymaker`, `ui/lang/storymaker_catalog`) are in the tree. Part B (the server) is not needed: the app runs against the real `internal/storymaker` package behind a test fixture, and part I (tasks I2 and I4) runs the same app against the real handlers in `internal/server`. All tasks run in `.worktrees/storymaker-p1` on branch `storymaker-p1` with plain `git add` / `git commit` (a new nested `AGENTS.md` needs `git add -f`: `.gitignore` ignores them).

Conventions of this part:

- The app modules are classic scripts in `ui/js/desktop/apps/storymaker*.js`, each under 1100 lines, all hanging off `window.StorymakerParts`; the load order is fixed by the loader entry (core, model, api, assets, player, projects, strip, inspector, code, export, then the root `storymaker.js`).
- Untrusted text (story, narration, names, server messages) reaches the page only as `textContent`. The DOM builder `S.el` has no html option; a guard test greps the app modules and `ui/js/storymaker` for innerHTML, insertAdjacentHTML, template strings (app modules), eval, dialogs and storage.
- The checkout may convert line endings (`* text=auto`): JavaScript files in the working tree can be CRLF. Edits to existing files below are given as diffs against the base commit `0248bc133`; apply them with the Edit tool or `git apply --ignore-whitespace`, never by pasting whole files over existing ones. New files are LF.
- Browser tests are gated by `AURAGO_RUN_BROWSER_SMOKE=1` and need the installed Chrome. On the Windows dev box run them through `go test` (test binaries live under `D:\tmp\go-build`) with the sandbox disabled; `GOCACHE` and `GOTMPDIR` as in the contract.
- The GitNexus rule (impact before editing a function, `detect-changes` before committing) is part of Tasks E1, E6, E9 and E11. At drafting time `impact` returned an empty answer (index not resolving Go and JS symbols), which is `UNKNOWN`, not safe: the edits are additive entries in registries (`BuiltinApps`, `DESKTOP_APP_ASSETS`, `appGlobalName`, `renderAppContent`, `appWindowSize`) and a new config block; the callers were confirmed by `grep` (`BuiltinApps()`: `internal/desktop/service.go:696`, `service_apps.go:231,423`, `internal/server/desktop_handlers.go:145`).

### Task E1: Manifest, logo and the app name in 16 languages

Registers the app in the backend: the `BuiltinApps` entry (category creative, `Requires: ["storymaker"]`, open maximized, app-owned logo), the logo, and `desktop.app_storymaker` in every desktop locale. The icon value `storymaker` stays out of the themed icon sets on purpose (as for ECalc), so every theme shows the logo.

**Files:**
- Modify: `internal/desktop/types.go` (after base line 848, the `ecalc` entry)
- Create: `ui/img/desktop-icons/storymaker.svg`
- Modify: `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` (after base line 751, `desktop.app_ecalc`)
- Test: `internal/desktop/storymaker_app_test.go`
- Test: `ui/desktop_storymaker_name_test.go`

- [ ] **Step 0: GitNexus impact (project rule)**

```bash
node C:/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs impact "BuiltinApps" --direction upstream --repo .
```
Expected: callers are `internal/desktop/service.go:696`, `service_apps.go:231,423` and `internal/server/desktop_handlers.go:145`; the edit only adds one slice entry. If the answer is empty or `UNKNOWN`, confirm with `grep -rn "BuiltinApps()" internal` and say so in the commit body.

- [ ] **Step 1: Write the failing tests**

`internal/desktop/storymaker_app_test.go`:

```go
package desktop

import "testing"

// Storymaker is a builtin creative app behind the "storymaker" capability (the storymaker.enabled switch). Its icon is
// app-owned artwork (logo_path), like Synth Studio's and ECalc's, so it needs no entry in the themed icon sets.
func TestBuiltinStorymakerAppRequiresItsCapability(t *testing.T) {
	app := testFindApp(t, BuiltinApps(), "storymaker")
	if app.Entry != "builtin://storymaker" || app.Name != "Storymaker" || app.Category != "creative" || app.Runtime != BuiltinRuntime {
		t.Fatalf("storymaker manifest = %+v", app)
	}
	if app.Icon != "storymaker" || app.Metadata["logo_path"] != "/img/desktop-icons/storymaker.svg" || app.Metadata["open_maximized"] != "true" {
		t.Fatalf("storymaker icon = %q, metadata = %v; want the app-owned logo and open_maximized", app.Icon, app.Metadata)
	}
	if len(app.Requires) != 1 || app.Requires[0] != "storymaker" {
		t.Fatalf("storymaker requires = %v, want [storymaker]", app.Requires)
	}
	if !app.Builtin || !app.StartVisible || !app.DockVisible || app.Deletable || app.Internal {
		t.Fatalf("storymaker must be a visible, permanent builtin: %+v", app)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{}); len(got) != 0 {
		t.Fatalf("without the capability the app must stay hidden: %+v", got)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{"storymaker": true}); len(got) != 1 {
		t.Fatalf("with the capability the app must show: %+v", got)
	}
}
```

`ui/desktop_storymaker_name_test.go`:

```go
package ui

import (
	"strings"
	"testing"
)

// The sixteen languages of the AuraGo UI; every Storymaker locale directory has one file per language.
var storymakerLanguages = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

func TestStorymakerAppNameExistsInAllLanguages(t *testing.T) {
	t.Parallel()
	for _, lang := range storymakerLanguages {
		if strings.TrimSpace(readMergedLangMap(t, lang)["desktop.app_storymaker"]) == "" {
			t.Errorf("%s lacks desktop.app_storymaker", lang)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/desktop ./ui -run 'TestBuiltinStorymakerApp|TestStorymakerAppNameExistsInAllLanguages' -count=1`
Expected: FAIL: `app "storymaker" not found` and `lacks desktop.app_storymaker`

Recorded at drafting time (the replay of this task on a tree without it):

```text
--- FAIL: TestBuiltinStorymakerAppRequiresItsCapability (0.00s)
    storymaker_app_test.go:8: app "storymaker" not found in [{ID:findit Name:FindIt Version:1.0.0 Icon:search Entry:builtin://findit Runtime:builtin Description:Find parts, equipment and storage locations; manage stock and proj...
FAIL
FAIL	aurago/internal/desktop	0.160s
--- FAIL: TestStorymakerAppNameExistsInAllLanguages (1.31s)
    desktop_storymaker_name_test.go:15: cs lacks desktop.app_storymaker
    desktop_storymaker_name_test.go:15: da lacks desktop.app_storymaker
    desktop_storymaker_name_test.go:15: de lacks desktop.app_storymaker
    desktop_storymaker_name_test.go:15: el lacks desktop.app_storymaker
    desktop_storymaker_name_test.go:15: en lacks desktop.app_storymaker
    desktop_storymaker_name_test.go:15: es lacks desktop.app_storymaker
    desktop_storymaker_name_test.go:15: fr lacks desktop.app_storymaker
    desktop_storymaker_name_test.go:15: hi lacks desktop.app_storymaker
    desktop_storymaker_name_test.go:15: it lacks desktop.app_storymaker
```

- [ ] **Step 3: Implement**

`internal/desktop/types.go` (after base line 848, the `ecalc` entry):

```diff
--- a/internal/desktop/types.go
+++ b/internal/desktop/types.go
@@ -846,6 +846,7 @@ func BuiltinApps() []AppManifest {
 		{ID: "game-maker-studio", Name: "Game Maker Studio", Version: "1.0.0", Icon: "gamepad", Entry: "builtin://game-maker-studio", Runtime: BuiltinRuntime, Category: "creative", Description: "Create and refine offline 2D and 3D browser games with an isolated AuraGo agent.", Permissions: []string{"notifications"}, Metadata: map[string]string{"open_maximized": "true", "logo_path": "/img/desktop-icons/game-maker-studio.svg"}},
 		{ID: "cyd-creator", Name: "CYD Creator", Version: "1.0.0", Icon: "cyd-creator", Entry: "builtin://cyd-creator", Runtime: BuiltinRuntime, Category: "dev", Description: "Design, code and build apps for Cheap Yellow Display boards with an isolated AuraGo agent.", Permissions: []string{"notifications"}, Requires: []string{"cyd_creator"}, Metadata: map[string]string{"open_maximized": "true"}},
 		{ID: "ecalc", Name: "ECalc", Version: "1.0.0", Icon: "ecalc", Entry: "builtin://ecalc", Runtime: BuiltinRuntime, Category: "dev", Description: "Electronics formula collection: interactive calculators with schematics, live results and standard resistor values.", Metadata: map[string]string{"logo_path": "/img/desktop-icons/ecalc.svg"}},
+		{ID: "storymaker", Name: "Storymaker", Version: "1.0.0", Icon: "storymaker", Entry: "builtin://storymaker", Runtime: BuiltinRuntime, Category: "creative", Description: "Turn a topic into an explainer video: stage scenes with characters, props and environments, preview them live and export an MP4.", Permissions: []string{"files:read", "files:write", "notifications"}, Requires: []string{"storymaker"}, Metadata: map[string]string{"open_maximized": "true", "logo_path": "/img/desktop-icons/storymaker.svg"}},
 		{ID: "homepage-studio", Name: "Homepage Studio", Version: "1.0.0", Icon: "globe", Entry: "builtin://homepage-studio", Runtime: BuiltinRuntime, Category: "creative", Description: "AI-powered website builder with live preview.", Permissions: []string{"notifications"}, Metadata: map[string]string{"open_maximized": "true"}},
 		{ID: "detective", Name: "Detective", Version: "1.0.0", Icon: "search", Entry: "builtin://detective", Runtime: BuiltinRuntime, Category: "ai", Description: "Research topics with cited sources and export reports as Markdown, PDF or Word.", Permissions: []string{"notifications"}, Metadata: map[string]string{"open_maximized": "true"}},
 		{ID: "newspaper", Name: "Newspaper", Version: "1.0.0", Icon: "newspaper", Entry: "builtin://newspaper", Runtime: BuiltinRuntime, Category: "ai", Description: "Read your sourced personal daily newspaper and manage delivery.", Permissions: []string{"notifications"}, Metadata: map[string]string{"open_maximized": "true"}},
```

`ui/img/desktop-icons/storymaker.svg`:

```xml
<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128" role="img" aria-label="Storymaker">
  <defs>
    <linearGradient id="g" x1="18" y1="14" x2="110" y2="116" gradientUnits="userSpaceOnUse">
      <stop stop-color="#ff8a4c"/>
      <stop offset="1" stop-color="#c2255c"/>
    </linearGradient>
  </defs>
  <rect x="8" y="8" width="112" height="112" rx="27" fill="url(#g)"/>
  <rect x="24" y="42" width="80" height="58" rx="10" fill="#fff7e6"/>
  <path d="M24 42V34a6 6 0 0 1 6-6h68a6 6 0 0 1 6 6v8z" fill="#1f2a44"/>
  <path d="M34 28h12l-8 14H26zM58 28h12l-8 14H50zM82 28h12l-8 14H74z" fill="#fff7e6"/>
  <circle cx="80" cy="62" r="7" fill="#ffb84d"/>
  <path d="M70 90h24" stroke="#36cfc9" stroke-width="4" stroke-linecap="round"/>
  <path d="M74 80h16" stroke="#1f2a44" stroke-opacity=".35" stroke-width="3" stroke-linecap="round"/>
  <circle cx="47" cy="63" r="8" fill="#1f2a44"/>
  <path d="M33 92c0-9 6-16 14-16s14 7 14 16z" fill="#36cfc9"/>
  <path d="m100 14 2.4 6.3 6.3 2.4-6.3 2.4-2.4 6.3-2.4-6.3-6.3-2.4 6.3-2.4L100 14Z" fill="#fff4a8"/>
</svg>
```

The same one-line change in each of the other 15 files `ui/lang/desktop/{cs,da,de,el,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` (the name is the product name, the same in every language; the context lines of the diff below are translated per language, so it applies to `en.json` only, and the added line is the same everywhere). Apply it to the 15 files with:

```bash
for l in cs da de el es fr hi it ja nl no pl pt sv zh; do
  sed -i 's/^  "desktop.app_ecalc": "ECalc",$/&\n  "desktop.app_storymaker": "Storymaker",/' ui/lang/desktop/$l.json
done
```

And for `en.json` apply this diff:

{% raw %}
```diff
--- a/ui/lang/desktop/en.json
+++ b/ui/lang/desktop/en.json
@@ -749,6 +749,7 @@
   "desktop.app_code_studio": "Code Studio",
   "desktop.app_delete_permanent": "Delete permanently",
   "desktop.app_ecalc": "ECalc",
+  "desktop.app_storymaker": "Storymaker",
   "desktop.app_editor": "Editor",
   "desktop.app_error_fallback": "Error",
   "desktop.app_error_renderer_missing": "{{app}} renderer is not loaded",
```

{% endraw %}
- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/desktop ./ui -run 'TestBuiltin|TestStorymakerAppNameExistsInAllLanguages|TestDesktopBuiltinAppNamesExistInAllLanguages' -count=1`
Expected: ok for both packages (`TestBuiltin` includes the start-menu category test: the app is category creative)

Recorded at drafting time:

```text
ok  	aurago/internal/desktop	0.681s
ok  	aurago/ui	3.081s
```

- [ ] **Step 5: Commit**

```bash
git add internal/desktop/storymaker_app_test.go ui/desktop_storymaker_name_test.go internal/desktop/types.go ui/img/desktop-icons/storymaker.svg ui/lang/desktop
git commit -m "feat(storymaker): register the Storymaker desktop app with its logo" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E2: App core, pure model and the player bridge

The first three app modules. `storymaker-core.js` is the namespace `window.StorymakerParts`, the DOM builder `S.el` (no html option), icons, bus, formatters and the error texts. `storymaker-model.js` is pure: JSON Pointer to text position, issue lists, the catalog index, the story edits of the inspector and the library, frame helpers. `storymaker-player.js` is the parent side of the iframe protocol (contract section 8): sandbox `allow-scripts` only, messages only from the iframe window and only in a fixed schema with caps, `load` after every `ready`.

**Files:**
- Create: `ui/js/desktop/apps/storymaker-core.js`
- Create: `ui/js/desktop/apps/storymaker-model.js`
- Create: `ui/js/desktop/apps/storymaker-player.js`
- Test: `ui/desktop_storymaker_model_test.go`

- [ ] **Step 1: Write the failing test**

`ui/desktop_storymaker_model_test.go`:

```go
package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// storymakerCode removes the comments of a script, so that a guard sees only what runs.
func storymakerCode(source string) string {
	blocks := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(source, "")
	return regexp.MustCompile(`(?m)(^|\s)//[^\n]*$`).ReplaceAllString(blocks, "")
}

// The iframe of the player: sandbox allow-scripts only, no permission delegation, messages only from its own window, a fixed
// schema with caps, posts to '*' because the origin is opaque, and no requests of its own.
func TestStorymakerPlayerBridgeIsSandboxed(t *testing.T) {
	t.Parallel()
	source := readDesktopAssetText(t, "js/desktop/apps/storymaker-player.js")
	for _, marker := range []string{
		"sandbox: 'allow-scripts'",
		"iframe.setAttribute('sandbox', 'allow-scripts');",
		"referrerpolicy: 'no-referrer'",
		"event.source !== iframe.contentWindow",
		"target.postMessage(Object.assign({ sm: 1 }, message), '*', transfer || []);",
		"const msg = checkIncoming(event.data);",
		"msg.sm !== 1",
		"LIMITS.keys",
		"LIMITS.scenes",
	} {
		if !strings.Contains(source, marker) {
			t.Errorf("storymaker-player.js lacks %q", marker)
		}
	}
	code := storymakerCode(source)
	for _, forbidden := range []string{"allow-same-origin", "allow-forms", "allow-modals", "allow-popups", "allow-top-navigation", "'allow',", "allow:"} {
		if strings.Contains(code, forbidden) {
			t.Errorf("storymaker-player.js must not contain %q", forbidden)
		}
	}
	if strings.Contains(code, "fetch(") || strings.Contains(code, "XMLHttpRequest") {
		t.Error("the bridge must not make requests itself: the parent serves the assets through the resolver")
	}
}

// The pure part of the app in Node: pointer walking in JSON text, issue lists, the catalog index, story edits and the message schema.
func TestStorymakerAppModelInNode(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-app-model.js")
	if err := os.WriteFile(script, []byte(storymakerModelScript), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	stories, _ := filepath.Abs(filepath.Join("..", "internal", "storymaker", "testdata", "stories", "firewall.json"))
	catalog, _ := filepath.Abs(filepath.Join("img", "storymaker", "catalog.json"))
	out, err := exec.Command(node, script, dir, stories, catalog).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "storymaker app model ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerModelScript = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const [dir, storyFile, catalogFile] = process.argv.slice(2);
class FakeBitmap { constructor() { this.width = 1; this.height = 1; } }
const context = { console, ImageBitmap: FakeBitmap };
context.globalThis = context; context.window = context;
vm.createContext(context);
for (const name of ['storymaker-model.js', 'storymaker-player.js']) {
  // the player module reads S.el only inside createPlayer, so the pure parts load without a DOM
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts, M = S.model;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const story = JSON.parse(fs.readFileSync(storyFile, 'utf8'));
const text = M.canonicalText(story);

// locate: a pointer finds its value, a missing field falls back to the deepest value that exists
const hit = M.locate(text, '/scenes/0/actors/0/at');
eq([hit.found, text.slice(hit.from, hit.to)], [true, '"desk"'], 'pointer to a string');
eq(M.locate(text, '/title').found, true, 'title');
const missing = M.locate(text, '/scenes/1/beats/0/expression');
eq([missing.found, missing.depth], [false, 4], 'a missing field gives the beat');
eq(M.locate(text, '/scenes/9/id').found, false, 'index out of range');
eq(M.parsePointer('/a~1b/c~0d'), ['a/b', 'c~d'], 'pointer escapes');
eq(M.lineColumn(text, M.locate(text, '/title').from), { line: 3, column: 12 }, 'line and column');
eq(M.positionFromMessage('invalid character at line 3, column 14'), { line: 3, column: 14 }, 'position from a message');
eq(M.offsetOf('ab\ncd\nef', 2, 2), 4, 'offset');
// a quote, a brace and an escape inside strings must not confuse the walker
const tricky = '{"a": "x\\"}{", "b": [1, {"c": "]"}], "d": 5}';
eq(tricky.slice(M.locate(tricky, '/d').from, M.locate(tricky, '/d').to), '5', 'string with quotes and braces');
eq(tricky.slice(M.locate(tricky, '/b/1/c').from, M.locate(tricky, '/b/1/c').to), '"]"', 'nested pointer');

// issues
const merged = M.mergeIssues([{ severity: 'minor', code: 'a', path: '/x', message: 'm' }, { severity: 'error', code: 'b', path: '', message: 'e' }], [{ severity: 'minor', code: 'a', path: '/x', message: 'm' }]);
eq(merged.map(i => i.severity), ['error', 'minor'], 'errors first, duplicates gone');
eq(M.visibleIssues([{ code: 'narration_missing' }, { code: 'x' }]).length, 1, 'narration_missing is hidden');
eq(M.sceneIndexOfIssue({ path: '/scenes/2/beats/0' }, story), 2, 'scene of an issue by path');
eq(M.sceneIndexOfIssue({ scene_id: 's2', path: '' }, story), 1, 'scene of an issue by id');

// catalog and edits
const catalog = M.indexCatalog(JSON.parse(fs.readFileSync(catalogFile, 'utf8')));
eq(M.entriesOf(catalog, 'character').length, 4, 'four characters');
const draft = M.cloneStory(story);
eq(M.addScene(draft, 0, {}), 1, 'a scene after the first');
eq(draft.scenes.map(s => s.id), ['s1', 's4', 's2', 's3'], 'scene ids');
eq(draft.scenes[1].environment, 'env.office', 'a new scene keeps the environment');
const code = M.addScene(draft, 3, { kind: 'code' });
eq([draft.scenes[code].kind, draft.scenes[code].id], ['code', 'code5'], 'a code scene');
eq(M.removeScene(draft, 0), 0, 'remove');
eq(M.moveScene(draft, 0, 2), 2, 'move');
const man = catalog.byId['char.office-man-1'];
eq(M.insertActor(draft, 0, man, 'Office man').ok, true, 'first insert');
eq(M.insertActor(draft, 0, man, 'Office man').reason, 'already_in_scene', 'the same person twice');
eq(draft.cast.filter(c => c.character === 'char.office-man-1').length, 1, 'one cast member');
const props = M.insertProp(draft, 0, catalog.byId['prop.laptop'], catalog.byId['env.office']);
eq([props.ok, draft.scenes[0].props[0].at], [true, 'right'], 'a prop takes a free spot');
eq(M.insertActor(draft, M.scenesOf(draft).length - 1, man, 'x').ok, false, 'no actors in a code scene');
const one = { schema: 'storymaker/1', scenes: [{ id: 's1' }] };
eq(M.removeScene(one, 0), -1, 'the last scene stays');
const scene = { id: 'x', narration: 'hi' };
M.setSceneField(scene, 'narration', ''); eq('narration' in scene, false, 'an empty field leaves the document');
M.setDuration(scene, false, '7.26'); eq(scene.duration, 7.3, 'fixed duration');
M.setDuration(scene, true, 0); eq(scene.duration, 'auto', 'auto duration');
const report = { frame_count: 300, fps: 30, scenes: [{ id: 'a', start_frame: 0, end_frame: 150 }, { id: 'b', start_frame: 140, end_frame: 300 }] };
eq(M.sceneAtFrame(report, 139), 0, 'frame in the first scene'); eq(M.sceneAtFrame(report, 145), 1, 'transitions overlap, the later scene wins');
eq(M.frameForScene(report, 1), 140 + Math.floor(160 * 0.4), 'frame for a scene');

// the message schema: only fixed shapes, with caps
const P = S.player.checkIncoming;
const valid = { sm: 1, type: 'compiled', report: { frame_count: 90, fps: 30, scenes: [{ id: 's1', start_frame: 0, end_frame: 90, kind: 'storyboard' }], issues: [{ severity: 'warning', code: 'x', path: '/a', message: 'm' }] } };
eq(P(valid).report.scenes.length, 1, 'a valid report');
eq(P(Object.assign({}, valid, { sm: 2 })), null, 'wrong version');
eq(P({ sm: 1, type: 'load' }), null, 'a parent-to-host type is refused');
eq(P({ sm: 1, type: 'compiled', report: { frame_count: 90, fps: 30, scenes: new Array(401).fill({ id: 's', start_frame: 0, end_frame: 1 }) } }), null, 'too many scenes');
eq(P({ sm: 1, type: 'compiled', report: { frame_count: -1, fps: 30, scenes: [] } }), null, 'negative frame count');
eq(P({ sm: 1, type: 'time', frame: 3, playing: 'yes' }), null, 'time needs a boolean');
eq(P({ sm: 1, type: 'time', frame: 3, playing: true }), { type: 'time', frame: 3, playing: true }, 'time');
eq(P({ sm: 1, type: 'need_assets', req: 'a1', keys: new Array(2001).fill('catalog:x') }), null, 'too many keys');
eq(P({ sm: 1, type: 'need_assets', req: 'a1', keys: ['catalog:rig/human.json', 7] }), null, 'a key that is not a string');
eq(P({ sm: 1, type: 'thumb', req: 't1', scene_id: 's1', bitmap: {} }), null, 'a thumb needs an ImageBitmap');
eq(P({ sm: 1, type: 'thumb', req: 't1', scene_id: 's1', bitmap: new FakeBitmap() }).scene_id, 's1', 'a thumb');
eq(P({ sm: 1, type: 'error', code: 'code_error', message: 'x'.repeat(5000), scene_id: 's3' }).message.length, 2000, 'long messages are cut');
eq(P({ sm: 1, type: 'ready', caps: { workers: true, offscreen: false }, extra: 'x' }), { type: 'ready', caps: { workers: true, offscreen: false } }, 'unknown fields are dropped');
console.log('storymaker app model ok');
`
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./ui -run 'TestStorymakerAppModelInNode|TestStorymakerPlayerBridgeIsSandboxed' -count=1`
Expected: FAIL: FAIL (`storymaker-model.js` / `storymaker-player.js` do not exist)

Recorded at drafting time (the replay of this task on a tree without it):

```text
--- FAIL: TestStorymakerPlayerBridgeIsSandboxed (0.00s)
    desktop_storymaker_model_test.go:22: read js/desktop/apps/storymaker-player.js: open js/desktop/apps/storymaker-player.js: The system cannot find the file specified.
--- FAIL: TestStorymakerAppModelInNode (0.11s)
    desktop_storymaker_model_test.go:65: node check failed: exit status 1
FAIL
FAIL	aurago/ui	0.631s
FAIL
```

- [ ] **Step 3: Implement**

`ui/js/desktop/apps/storymaker-core.js`:

```javascript
// Storymaker app core: the shared namespace window.StorymakerParts (S), the DOM builder S.el, the icon set, the cleanup bag,
// the event bus, formatters and the mapping from API errors to sentences. Everything that reaches the page goes through
// el() as text: there is deliberately no html option, no innerHTML and no template string with data in any storymaker file
// (a guard test greps for it). Loaded first; the other modules hang their parts on S. Classic script.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const SVG_NS = 'http://www.w3.org/2000/svg';

    // ---- DOM ------------------------------------------------------------------------------------------------------------

    function attrName(key) {
        return /^aria[A-Z]/.test(key) ? 'aria-' + key.slice(4).toLowerCase() : key;
    }

    function appendChildren(node, children) {
        children.forEach(child => {
            if (child == null || child === false || child === true) return;
            if (Array.isArray(child)) { appendChildren(node, child); return; }
            if (typeof child === 'string' || typeof child === 'number') { node.appendChild(document.createTextNode(String(child))); return; }
            node.appendChild(child);
        });
    }

    // el(tag, attrs, ...children): class, text (textContent), dataset, style (camelCase or --custom), on<Event> listeners,
    // aria<Name> and any other attribute; a false/null value omits the attribute, true sets it empty.
    function el(tag, attrs, ...children) {
        const node = document.createElement(tag);
        const a = attrs || {};
        Object.keys(a).forEach(key => {
            const value = a[key];
            if (value == null || value === false) return;
            if (key === 'html') throw new Error('S.el does not support html');
            if (key === 'class') { node.className = String(value); return; }
            if (key === 'text') { node.textContent = String(value); return; }
            if (key === 'dataset') {
                Object.keys(value).forEach(name => { if (value[name] != null) node.dataset[name] = String(value[name]); });
                return;
            }
            if (key === 'style') {
                Object.keys(value).forEach(name => {
                    if (name.startsWith('--')) node.style.setProperty(name, String(value[name]));
                    else node.style[name] = value[name];
                });
                return;
            }
            if (/^on[A-Z]/.test(key) && typeof value === 'function') { node.addEventListener(key.slice(2).toLowerCase(), value); return; }
            node.setAttribute(attrName(key), value === true ? '' : String(value));
        });
        appendChildren(node, children);
        return node;
    }

    function clear(node) {
        while (node.firstChild) node.removeChild(node.firstChild);
        return node;
    }

    // ---- icons: 24 x 24 strokes drawn with attributes, never with markup strings --------------------------------------------

    const ICONS = {
        play: { fill: true, d: ['M7 4.5v15l12-7.5z'] },
        pause: { d: ['M8 5v14', 'M16 5v14'] },
        stop: { fill: true, d: ['M7 7h10v10H7z'] },
        plus: { d: ['M12 5v14', 'M5 12h14'] },
        trash: { d: ['M4 7h16', 'M9 7V4h6v3', 'M6 7l1 13h10l1-13', 'M10 11v6', 'M14 11v6'] },
        up: { d: ['M6 15l6-6 6 6'] },
        down: { d: ['M6 9l6 6 6-6'] },
        left: { d: ['M15 5l-7 7 7 7'] },
        export: { d: ['M12 3v12', 'M7 10l5 5 5-5', 'M5 20h14'] },
        code: { d: ['M9 7l-5 5 5 5', 'M15 7l5 5-5 5'] },
        film: { d: ['M4 5h16v14H4z', 'M8 5v14', 'M16 5v14', 'M4 9h4', 'M4 15h4', 'M16 9h4', 'M16 15h4'] },
        user: { d: ['M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8z', 'M4 20c0-4 4-6 8-6s8 2 8 6'] },
        box: { d: ['M12 3l8 4.5v9L12 21l-8-4.5v-9z', 'M4 7.5l8 4.5 8-4.5', 'M12 12v9'] },
        scenery: { d: ['M3 19l6-10 4 6 2-3 6 7z', 'M16 6.5a1.5 1.5 0 1 0 0 .1'] },
        check: { d: ['M5 12.5l4.5 4.5L19 7'] },
        alert: { d: ['M12 4l9 16H3z', 'M12 10v4', 'M12 17v.5'] },
        info: { d: ['M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18z', 'M12 11v6', 'M12 7.5v.5'] },
        close: { d: ['M6 6l12 12', 'M18 6L6 18'] },
        refresh: { d: ['M20 11a8 8 0 1 0-2.3 6', 'M20 5v6h-6'] },
        folder: { d: ['M3 7h6l2 2h10v10H3z'] },
        copy: { d: ['M9 9h11v11H9z', 'M5 15V4h11'] },
        lock: { d: ['M6 11h12v9H6z', 'M9 11V8a3 3 0 0 1 6 0v3'] },
        sliders: { d: ['M4 7h10', 'M18 7h2', 'M4 17h2', 'M10 17h10', 'M14 5v4', 'M6 15v4'] },
        sparkle: { d: ['M12 3l2 6 6 2-6 2-2 6-2-6-6-2 6-2z'] }
    };

    function icon(name, className) {
        const spec = ICONS[name] || ICONS.info;
        const svg = document.createElementNS(SVG_NS, 'svg');
        svg.setAttribute('viewBox', '0 0 24 24');
        svg.setAttribute('class', 'stm-icon' + (className ? ' ' + className : ''));
        svg.setAttribute('aria-hidden', 'true');
        svg.setAttribute('focusable', 'false');
        svg.setAttribute('fill', spec.fill ? 'currentColor' : 'none');
        svg.setAttribute('stroke', 'currentColor');
        svg.setAttribute('stroke-width', '2');
        svg.setAttribute('stroke-linecap', 'round');
        svg.setAttribute('stroke-linejoin', 'round');
        spec.d.forEach(d => {
            const path = document.createElementNS(SVG_NS, 'path');
            path.setAttribute('d', d);
            svg.appendChild(path);
        });
        return svg;
    }

    // ---- lifecycle ----------------------------------------------------------------------------------------------------------

    function safeCall(fn) {
        try { fn(); } catch (err) { console.error('storymaker cleanup failed', err); }
    }

    // bag collects cleanups; dispose() runs them newest first, exactly once; anything added afterwards runs at once.
    function bag() {
        let fns = [];
        let disposed = false;
        const b = {
            add(fn) {
                if (typeof fn !== 'function') return fn;
                if (disposed) { safeCall(fn); return fn; }
                fns.push(fn);
                return fn;
            },
            on(target, type, fn, opts) {
                if (!target || typeof target.addEventListener !== 'function') return () => {};
                target.addEventListener(type, fn, opts);
                const off = () => target.removeEventListener(type, fn, opts);
                b.add(off);
                return off;
            },
            timeout(fn, ms) {
                const id = setTimeout(fn, ms);
                b.add(() => clearTimeout(id));
                return id;
            },
            interval(fn, ms) {
                const id = setInterval(fn, ms);
                b.add(() => clearInterval(id));
                return id;
            },
            dispose() {
                if (disposed) return;
                disposed = true;
                const list = fns.reverse();
                fns = [];
                list.forEach(safeCall);
            },
            get disposed() { return disposed; }
        };
        return b;
    }

    // bus: on(name, fn) returns the unsubscribe function; emit never throws into the emitter.
    function createBus() {
        const handlers = new Map();
        return {
            on(name, fn) {
                if (!handlers.has(name)) handlers.set(name, new Set());
                handlers.get(name).add(fn);
                return () => handlers.get(name) && handlers.get(name).delete(fn);
            },
            emit(name, payload) {
                (handlers.get(name) ? Array.from(handlers.get(name)) : []).forEach(fn => {
                    try { fn(payload); } catch (err) { console.error('storymaker handler failed', name, err); }
                });
            },
            clear() { handlers.clear(); }
        };
    }

    function debounce(fn, ms) {
        let timer = 0;
        const wrapped = (...args) => {
            clearTimeout(timer);
            timer = setTimeout(() => { timer = 0; fn(...args); }, ms);
        };
        wrapped.cancel = () => { clearTimeout(timer); timer = 0; };
        wrapped.pending = () => timer !== 0;
        return wrapped;
    }

    function clamp(value, min, max) {
        const n = Number(value);
        if (!Number.isFinite(n)) return min;
        return Math.min(max, Math.max(min, n));
    }

    function uuid() {
        if (typeof crypto !== 'undefined' && crypto && typeof crypto.randomUUID === 'function') return crypto.randomUUID();
        const bytes = new Uint8Array(16);
        if (typeof crypto !== 'undefined' && crypto && typeof crypto.getRandomValues === 'function') crypto.getRandomValues(bytes);
        else for (let i = 0; i < 16; i++) bytes[i] = Math.floor(Math.random() * 256);
        bytes[6] = (bytes[6] & 0x0f) | 0x40;
        bytes[8] = (bytes[8] & 0x3f) | 0x80;
        const hex = Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('');
        return hex.slice(0, 8) + '-' + hex.slice(8, 12) + '-' + hex.slice(12, 16) + '-' + hex.slice(16, 20) + '-' + hex.slice(20);
    }

    // ---- formatting ---------------------------------------------------------------------------------------------------------

    function locale() {
        const lang = (typeof window !== 'undefined' && window.SYSTEM_LANG) || (document.documentElement && document.documentElement.lang) || 'en';
        return String(lang || 'en');
    }

    const format = {
        // m:ss.t from a frame number
        clock(frame, fps) {
            const rate = Number(fps) > 0 ? Number(fps) : 30;
            const tenths = Math.max(0, Math.floor((Number(frame) || 0) / rate * 10));
            const minutes = Math.floor(tenths / 600);
            const seconds = Math.floor((tenths % 600) / 10);
            return minutes + ':' + String(seconds).padStart(2, '0') + '.' + (tenths % 10);
        },
        // seconds with one decimal at most, no trailing ".0"
        seconds(value) {
            const n = Math.round((Number(value) || 0) * 10) / 10;
            return String(n);
        },
        dateTime(iso) {
            const d = new Date(iso);
            if (!iso || Number.isNaN(d.getTime())) return '';
            try {
                return new Intl.DateTimeFormat(locale(), { dateStyle: 'medium', timeStyle: 'short' }).format(d);
            } catch (_) {
                return d.toISOString().slice(0, 16).replace('T', ' ');
            }
        },
        bytes(n) {
            const v = Number(n) || 0;
            if (v < 1024) return v + ' B';
            if (v < 1048576) return (v / 1024).toFixed(1) + ' KiB';
            return (v / 1048576).toFixed(1) + ' MiB';
        },
        percent(fraction) {
            return Math.round(clamp(fraction, 0, 1) * 100) + ' %';
        }
    };

    // The localized name of a catalog entry: its name_key when the language has it, else the bare name of the catalog.
    function catalogName(t, entry) {
        if (!entry) return '';
        const key = typeof entry.name_key === 'string' ? entry.name_key : '';
        if (key) {
            const text = t(key);
            if (text && text !== key) return text;
        }
        return String(entry.name || entry.id || '');
    }

    // ---- errors -------------------------------------------------------------------------------------------------------------

    // normalize turns whatever a request threw into {status, code, message, issues, network}. Server answers are flat:
    // {error, code, message, issues?}.
    function normalizeError(err) {
        if (err && err.stm) return err;
        const body = err && err.body && typeof err.body === 'object' ? err.body : {};
        const out = {
            stm: true,
            status: Number(err && err.status) || 0,
            code: String(body.code || body.error || (err && err.code) || ''),
            message: String(body.message || (err && err.message) || ''),
            issues: Array.isArray(body.issues) ? body.issues : [],
            network: !(err && err.status)
        };
        return out;
    }

    function errorFromResponse(status, body, retryAfter) {
        const b = body && typeof body === 'object' ? body : {};
        return {
            stm: true,
            status,
            code: String(b.code || b.error || ''),
            message: String(b.message || b.error || ''),
            issues: Array.isArray(b.issues) ? b.issues : [],
            retryAfter: Number(retryAfter) || 0,
            network: false
        };
    }

    // describe(t, err) is the sentence for an error; unknown ones say what happened in general terms and append the server's
    // own message as detail (shown as text only).
    function describeError(t, err) {
        const e = normalizeError(err);
        let key = '';
        if (e.network) key = 'storymaker.error.network';
        else if (e.status === 401 || e.code === 'unauthorized') key = 'storymaker.error.unauthorized';
        else if (e.code === 'desktop_scope_required') key = 'storymaker.error.scope';
        else if (e.code === 'desktop_readonly') key = 'storymaker.error.readonly';
        else if (e.code === 'storymaker_disabled') key = 'storymaker.error.disabled';
        else if (e.code === 'invalid_story' || e.status === 422) key = 'storymaker.error.invalid_story';
        else if (e.status === 412) key = 'storymaker.error.conflict';
        else if (e.status === 428) key = 'storymaker.error.required';
        else if (e.status === 413) key = 'storymaker.error.too_large';
        else if (e.status === 404) key = 'storymaker.error.not_found';
        else if (e.code === 'busy' || e.status === 429) key = 'storymaker.error.busy';
        // a refused render start answers with the code of the job error it would have ended in
        else if (JOB_ERROR_CODES.indexOf(e.code) >= 0 && e.code !== 'cancelled') key = 'storymaker.job_error.' + e.code;
        if (key) return t(key);
        const base = t('storymaker.error.generic');
        return e.message ? base + ' ' + e.message : base;
    }

    // The job error codes of the spec (Error handling) that a phase 1 render can end with.
    const JOB_ERROR_CODES = ['invalid_story', 'storymaker_disabled', 'browser_unavailable', 'ffmpeg_unavailable', 'render_failed', 'render_timeout', 'cancelled', 'busy', 'code_error'];

    function describeJobError(t, error) {
        const code = error && typeof error.code === 'string' ? error.code : '';
        if (JOB_ERROR_CODES.indexOf(code) >= 0) return t('storymaker.job_error.' + code);
        return t('storymaker.job_error.unknown');
    }

    const ISSUE_CODES = ['invalid_json', 'unknown_field', 'schema_mismatch', 'too_large', 'limit_exceeded', 'missing_field', 'invalid_value', 'duplicate_id', 'unknown_name', 'unknown_asset', 'unknown_spot', 'unknown_cast', 'unknown_cue', 'canonicalised', 'not_supported_yet', 'narration_missing', 'code_error', 'deadline', 'layout_unverified'];

    function issueTitle(t, code) {
        if (ISSUE_CODES.indexOf(code) >= 0) return t('storymaker.issue.' + code);
        return t('storymaker.issue.unknown');
    }

    S.el = el;
    S.clear = clear;
    S.icon = icon;
    S.bag = bag;
    S.createBus = createBus;
    S.debounce = debounce;
    S.clamp = clamp;
    S.uuid = uuid;
    S.format = format;
    S.catalogName = catalogName;
    S.errors = { normalize: normalizeError, fromResponse: errorFromResponse, describe: describeError, describeJob: describeJobError, issueTitle };
    S.ISSUE_CODES = ISSUE_CODES;
    S.JOB_ERROR_CODES = JOB_ERROR_CODES;
    S.ICON_NAMES = Object.keys(ICONS);
})();
```

`ui/js/desktop/apps/storymaker-model.js`:

```javascript
// Storymaker app model: the pure part. JSON Pointer to text position (S.model.locate walks the JSON text, so an issue path
// finds its line in story.json), issue lists, the catalog index, the story edits of the inspector and the library (every edit
// works on a clone and keeps the document in the shape the server expects: no unknown field is ever added) and the frame
// helpers of the scrubber. No DOM; a Go test runs it in Node. Classic script, publishes S.model.
(function (root) {
    'use strict';

    const S = root.StorymakerParts = root.StorymakerParts || {};
    const M = S.model = {};

    // ---- JSON text positions ----------------------------------------------------------------------------------------------

    // parsePointer("/scenes/1/beats/0") -> ["scenes","1","beats","0"]; "" -> []; anything that is not a pointer -> null.
    function parsePointer(pointer) {
        if (typeof pointer !== 'string') return null;
        if (pointer === '') return [];
        if (pointer.charAt(0) !== '/') return null;
        return pointer.slice(1).split('/').map(token => token.replace(/~1/g, '/').replace(/~0/g, '~'));
    }

    function skipSpace(text, i) {
        while (i < text.length) {
            const c = text.charCodeAt(i);
            if (c === 32 || c === 9 || c === 10 || c === 13) i++; else break;
        }
        return i;
    }

    // End (exclusive) of the string that starts at text[i] === '"'.
    function stringEnd(text, i) {
        i++;
        while (i < text.length) {
            const c = text.charCodeAt(i);
            if (c === 92) i += 2;
            else if (c === 34) return i + 1;
            else i++;
        }
        return text.length;
    }

    // End (exclusive) of the JSON value that starts at text[i]. Tolerant: an unterminated value ends at the end of the text.
    function valueEnd(text, i) {
        const c = text.charAt(i);
        if (c === '"') return stringEnd(text, i);
        if (c === '{' || c === '[') {
            const close = c === '{' ? '}' : ']';
            i = skipSpace(text, i + 1);
            while (i < text.length) {
                const d = text.charAt(i);
                if (d === close) return i + 1;
                if (d === ',') { i = skipSpace(text, i + 1); continue; }
                if (c === '{') {
                    if (d !== '"') return text.length;
                    i = skipSpace(text, stringEnd(text, i));
                    if (text.charAt(i) !== ':') return text.length;
                    i = skipSpace(text, i + 1);
                }
                i = skipSpace(text, valueEnd(text, i));
            }
            return text.length;
        }
        let j = i;
        while (j < text.length && ',]} \t\r\n'.indexOf(text.charAt(j)) < 0) j++;
        return j > i ? j : Math.min(text.length, i + 1);
    }

    // locate(text, pointer) -> {from, to, found, depth}: the range of the value the pointer names. A pointer that leaves the
    // document (a missing field) gives the range of the deepest value that exists, with found false.
    function locate(text, pointer) {
        const tokens = parsePointer(pointer);
        const source = String(text == null ? '' : text);
        let from = skipSpace(source, 0);
        let to = valueEnd(source, from);
        if (!tokens) return { from, to, found: false, depth: 0 };
        let depth = 0;
        for (const token of tokens) {
            const c = source.charAt(from);
            let hit = null;
            if (c === '{') {
                let i = skipSpace(source, from + 1);
                while (i < to) {
                    const d = source.charAt(i);
                    if (d === '}') break;
                    if (d === ',') { i = skipSpace(source, i + 1); continue; }
                    if (d !== '"') break;
                    const keyEnd = stringEnd(source, i);
                    let key = '';
                    try { key = JSON.parse(source.slice(i, keyEnd)); } catch (_) { key = source.slice(i + 1, keyEnd - 1); }
                    i = skipSpace(source, keyEnd);
                    if (source.charAt(i) !== ':') break;
                    i = skipSpace(source, i + 1);
                    const end = valueEnd(source, i);
                    if (key === token) { hit = { from: i, to: end }; break; }
                    i = skipSpace(source, end);
                }
            } else if (c === '[') {
                const want = /^(0|[1-9][0-9]*)$/.test(token) ? Number(token) : -1;
                let i = skipSpace(source, from + 1), index = 0;
                while (i < to && want >= 0) {
                    const d = source.charAt(i);
                    if (d === ']') break;
                    if (d === ',') { i = skipSpace(source, i + 1); continue; }
                    const end = valueEnd(source, i);
                    if (index === want) { hit = { from: i, to: end }; break; }
                    index++;
                    i = skipSpace(source, end);
                }
            }
            if (!hit) return { from, to, found: false, depth };
            from = hit.from;
            to = hit.to;
            depth++;
        }
        return { from, to, found: true, depth };
    }

    // line (1-based) and column (1-based) of a character offset
    function lineColumn(text, offset) {
        const upTo = String(text).slice(0, Math.max(0, offset));
        const lines = upTo.split('\n');
        return { line: lines.length, column: lines[lines.length - 1].length + 1 };
    }

    // The line and column a decode message names ("... at line 3, column 14"), else null.
    function positionFromMessage(message) {
        const m = /line\s+(\d+)(?:\D+column\s+(\d+))?/i.exec(String(message || ''));
        if (!m) return null;
        return { line: Number(m[1]), column: m[2] ? Number(m[2]) : 1 };
    }

    // offset of a (line, column) pair, clamped into the text
    function offsetOf(text, line, column) {
        const source = String(text);
        let offset = 0;
        for (let n = 1; n < line; n++) {
            const next = source.indexOf('\n', offset);
            if (next < 0) return source.length;
            offset = next + 1;
        }
        const end = source.indexOf('\n', offset);
        const lineEnd = end < 0 ? source.length : end;
        return Math.min(offset + Math.max(0, (column || 1) - 1), lineEnd);
    }

    // ---- issues -----------------------------------------------------------------------------------------------------------

    const SEVERITY_ORDER = { error: 0, warning: 1, minor: 2 };

    function cleanIssue(issue) {
        if (!issue || typeof issue !== 'object') return null;
        const severity = SEVERITY_ORDER[issue.severity] !== undefined ? issue.severity : 'warning';
        return {
            severity,
            code: String(issue.code || ''),
            path: typeof issue.path === 'string' ? issue.path : '',
            message: String(issue.message || ''),
            scene_id: typeof issue.scene_id === 'string' ? issue.scene_id : '',
            fix: typeof issue.fix === 'string' ? issue.fix : ''
        };
    }

    // mergeIssues(...lists): one list, no duplicates (severity, code, path, message), errors first, order kept within a severity.
    function mergeIssues() {
        const seen = new Set();
        const out = [];
        Array.prototype.slice.call(arguments).forEach(list => {
            (Array.isArray(list) ? list : []).forEach(raw => {
                const issue = cleanIssue(raw);
                if (!issue) return;
                const key = issue.severity + '|' + issue.code + '|' + issue.path + '|' + issue.message;
                if (seen.has(key)) return;
                seen.add(key);
                out.push(issue);
            });
        });
        return out
            .map((issue, order) => ({ issue, order }))
            .sort((a, b) => SEVERITY_ORDER[a.issue.severity] - SEVERITY_ORDER[b.issue.severity] || a.order - b.order)
            .map(entry => entry.issue);
    }

    // Phase 1 renders are silent, so a scene without narration audio is not worth showing.
    function visibleIssues(list) {
        return (Array.isArray(list) ? list : []).filter(issue => issue.code !== 'narration_missing');
    }

    function issueCounts(list) {
        const counts = { error: 0, warning: 0, minor: 0 };
        (Array.isArray(list) ? list : []).forEach(issue => { if (counts[issue.severity] !== undefined) counts[issue.severity]++; });
        return counts;
    }

    // The scene index an issue belongs to (scene_id first, else /scenes/N in its path), -1 when it is not about a scene.
    function sceneIndexOfIssue(issue, story) {
        const scenes = story && Array.isArray(story.scenes) ? story.scenes : [];
        if (issue && issue.scene_id) {
            const byId = scenes.findIndex(scene => scene && scene.id === issue.scene_id);
            if (byId >= 0) return byId;
        }
        const m = /^\/scenes\/(\d+)(?:\/|$)/.exec(issue && issue.path || '');
        if (m && Number(m[1]) < scenes.length) return Number(m[1]);
        return -1;
    }

    // ---- catalog ----------------------------------------------------------------------------------------------------------

    // indexCatalog(catalog) -> {version, byId, byKind}; unknown shapes give an empty index.
    function indexCatalog(catalog) {
        const index = { version: '', byId: {}, byKind: {}, fonts: [], palettes: [], stage: null, rig: null };
        if (!catalog || typeof catalog !== 'object') return index;
        index.version = String(catalog.version || '');
        index.fonts = Array.isArray(catalog.fonts) ? catalog.fonts : [];
        index.palettes = Array.isArray(catalog.palettes) ? catalog.palettes : [];
        index.stage = catalog.stage || null;
        index.rig = catalog.rig || null;
        (Array.isArray(catalog.entries) ? catalog.entries : []).forEach(entry => {
            if (!entry || typeof entry.id !== 'string' || typeof entry.kind !== 'string') return;
            index.byId[entry.id] = entry;
            (index.byKind[entry.kind] = index.byKind[entry.kind] || []).push(entry);
        });
        return index;
    }

    function entriesOf(index, kind) {
        return index && index.byKind && index.byKind[kind] ? index.byKind[kind] : [];
    }

    // The spots of an environment in the order the catalog lists them.
    function spotNames(entry) {
        return entry && entry.spots && typeof entry.spots === 'object' ? Object.keys(entry.spots) : [];
    }

    // ---- story edits ------------------------------------------------------------------------------------------------------

    function cloneStory(story) {
        return JSON.parse(JSON.stringify(story));
    }

    function canonicalText(story) {
        return JSON.stringify(story, null, 2) + '\n';
    }

    function scenesOf(story) {
        return story && Array.isArray(story.scenes) ? story.scenes : [];
    }

    // s1, s2 ... the smallest number above the count that is not taken
    function newSceneId(story, prefix) {
        const taken = new Set(scenesOf(story).map(scene => scene && scene.id));
        const base = prefix || 's';
        for (let n = scenesOf(story).length + 1; n < 100000; n++) if (!taken.has(base + n)) return base + n;
        return base + Date.now();
    }

    function addScene(story, afterIndex, options) {
        const o = options || {};
        const scenes = story.scenes = scenesOf(story);
        const at = Math.min(Math.max(afterIndex + 1, 0), scenes.length);
        const previous = scenes[Math.min(afterIndex, scenes.length - 1)];
        const scene = { id: newSceneId(story, o.kind === 'code' ? 'code' : 's') };
        if (o.kind === 'code') scene.kind = 'code';
        else if (previous && previous.environment) scene.environment = previous.environment;
        scenes.splice(at, 0, scene);
        return at;
    }

    function removeScene(story, index) {
        const scenes = scenesOf(story);
        if (index < 0 || index >= scenes.length || scenes.length <= 1) return -1;
        scenes.splice(index, 1);
        return Math.min(index, scenes.length - 1);
    }

    function moveScene(story, from, to) {
        const scenes = scenesOf(story);
        if (from < 0 || from >= scenes.length || to < 0 || to >= scenes.length || from === to) return -1;
        const [scene] = scenes.splice(from, 1);
        scenes.splice(to, 0, scene);
        return to;
    }

    // Empty values leave the document: the server's defaults (and omitted fields) stay the single source of the empty state.
    function setSceneField(scene, name, value) {
        if (value === '' || value == null) delete scene[name];
        else scene[name] = value;
    }

    // durations: "auto" or a number of seconds; anything else reads as auto
    function durationOf(scene) {
        const d = scene ? scene.duration : undefined;
        if (typeof d === 'number' && d > 0) return { auto: false, seconds: d };
        return { auto: true, seconds: 0 };
    }

    function setDuration(scene, auto, seconds) {
        if (auto) scene.duration = 'auto';
        else scene.duration = Math.max(0.5, Math.round(Number(seconds) * 10) / 10 || 5);
    }

    function castSlug(entry) {
        const raw = String(entry.id).replace(/^[a-z]+\./, '').toLowerCase().replace(/[^a-z0-9-]+/g, '-').replace(/^-+|-+$/g, '');
        return (raw || 'actor').slice(0, 48);
    }

    // insertActor(story, sceneIndex, characterEntry, displayName) -> {ok, castId, reason}; the same cast member cannot stand
    // twice in a scene (the validator calls that a duplicate id).
    function insertActor(story, sceneIndex, entry, displayName) {
        const scene = scenesOf(story)[sceneIndex];
        if (!scene || scene.kind === 'code' || !entry) return { ok: false, reason: 'no_scene' };
        story.cast = Array.isArray(story.cast) ? story.cast : [];
        let member = story.cast.find(candidate => candidate && candidate.character === entry.id);
        if (!member) {
            member = { id: castSlug(entry), name: String(displayName || entry.name || entry.id).slice(0, 80), character: entry.id };
            let n = 2;
            while (story.cast.some(candidate => candidate && candidate.id === member.id)) member.id = castSlug(entry).slice(0, 44) + '-' + n++;
            story.cast.push(member);
        }
        scene.actors = Array.isArray(scene.actors) ? scene.actors : [];
        if (scene.actors.some(actor => actor && actor.cast === member.id)) return { ok: false, reason: 'already_in_scene', castId: member.id };
        scene.actors.push({ cast: member.id });
        return { ok: true, castId: member.id };
    }

    // The first of the base spots (right, left, ...) that nothing stands on yet, else the centre.
    function freeSpot(scene, environmentEntry) {
        const used = new Set();
        (Array.isArray(scene.actors) ? scene.actors : []).forEach(actor => { if (actor && typeof actor.at === 'string') used.add(actor.at); });
        (Array.isArray(scene.props) ? scene.props : []).forEach(prop => { if (prop && typeof prop.at === 'string') used.add(prop.at); });
        const offered = spotNames(environmentEntry);
        const order = ['right', 'left', 'far-right', 'far-left', 'center'].filter(name => !offered.length || offered.indexOf(name) >= 0);
        return order.find(name => !used.has(name)) || 'center';
    }

    function insertProp(story, sceneIndex, entry, environmentEntry) {
        const scene = scenesOf(story)[sceneIndex];
        if (!scene || scene.kind === 'code' || !entry) return { ok: false, reason: 'no_scene' };
        scene.props = Array.isArray(scene.props) ? scene.props : [];
        scene.props.push({ item: entry.id, at: freeSpot(scene, environmentEntry) });
        return { ok: true };
    }

    function removeActor(story, sceneIndex, k) {
        const scene = scenesOf(story)[sceneIndex];
        if (!scene || !Array.isArray(scene.actors) || k < 0 || k >= scene.actors.length) return false;
        scene.actors.splice(k, 1);
        if (!scene.actors.length) delete scene.actors;
        return true;
    }

    function removeProp(story, sceneIndex, k) {
        const scene = scenesOf(story)[sceneIndex];
        if (!scene || !Array.isArray(scene.props) || k < 0 || k >= scene.props.length) return false;
        scene.props.splice(k, 1);
        if (!scene.props.length) delete scene.props;
        return true;
    }

    // ---- frames -----------------------------------------------------------------------------------------------------------

    // The scene that is on screen at a frame: the last one that has started (transitions overlap, so the later wins).
    function sceneAtFrame(report, frame) {
        const scenes = report && Array.isArray(report.scenes) ? report.scenes : [];
        let found = -1;
        for (let i = 0; i < scenes.length; i++) if (scenes[i].start_frame <= frame) found = i;
        return found;
    }

    // The frame a click on a scene lands on: 40 % into the scene, like the thumbnail.
    function frameForScene(report, index) {
        const scenes = report && Array.isArray(report.scenes) ? report.scenes : [];
        const scene = scenes[index];
        if (!scene) return 0;
        const length = Math.max(1, scene.end_frame - scene.start_frame);
        return Math.min(Math.max(0, report.frame_count - 1), scene.start_frame + Math.floor(length * 0.4));
    }

    function previewWidth(aspect) {
        if (aspect === '9:16') return 540;
        if (aspect === '1:1') return 720;
        return 960;
    }

    function aspectRatio(aspect) {
        if (aspect === '9:16') return 9 / 16;
        if (aspect === '1:1') return 1;
        return 16 / 9;
    }

    M.parsePointer = parsePointer;
    M.locate = locate;
    M.lineColumn = lineColumn;
    M.positionFromMessage = positionFromMessage;
    M.offsetOf = offsetOf;
    M.mergeIssues = mergeIssues;
    M.visibleIssues = visibleIssues;
    M.issueCounts = issueCounts;
    M.sceneIndexOfIssue = sceneIndexOfIssue;
    M.indexCatalog = indexCatalog;
    M.entriesOf = entriesOf;
    M.spotNames = spotNames;
    M.cloneStory = cloneStory;
    M.canonicalText = canonicalText;
    M.scenesOf = scenesOf;
    M.newSceneId = newSceneId;
    M.addScene = addScene;
    M.removeScene = removeScene;
    M.moveScene = moveScene;
    M.setSceneField = setSceneField;
    M.durationOf = durationOf;
    M.setDuration = setDuration;
    M.insertActor = insertActor;
    M.insertProp = insertProp;
    M.removeActor = removeActor;
    M.removeProp = removeProp;
    M.freeSpot = freeSpot;
    M.sceneAtFrame = sceneAtFrame;
    M.frameForScene = frameForScene;
    M.previewWidth = previewWidth;
    M.aspectRatio = aspectRatio;
})(typeof globalThis !== 'undefined' ? globalThis : window);
```

`ui/js/desktop/apps/storymaker-player.js`:

```javascript
// Storymaker app player bridge: the sandboxed iframe and the parent side of the typed protocol of contract section 8.
// The iframe is sandbox="allow-scripts" only (opaque origin, no allow-same-origin, no allow permissions) and loads
// /api/desktop/storymaker/player. The parent accepts a message only when event.source is the iframe's own window and the
// message fits a fixed schema with size caps (everything else is dropped and counted); it posts with targetOrigin '*'
// because the iframe origin is opaque, sends `load` after every `ready` (also after an iframe reload: each reload resets all
// state), never sends tokens, paths or API data, answers `need_assets` only with what the asset resolver serves, and
// reports unknown keys as missing. Classic script, publishes S.createPlayer and S.player.checkIncoming.
(function (root) {
    'use strict';

    const S = root.StorymakerParts = root.StorymakerParts || {};

    const LIMITS = { scenes: 400, issues: 2000, keys: 2000, keyLength: 240, text: 2000, id: 64, frame: 100000000, thumbWaitMs: 12000 };
    const TYPES = ['ready', 'need_assets', 'compiled', 'time', 'thumb', 'error'];

    function isObject(v) { return v !== null && typeof v === 'object' && !Array.isArray(v); }
    function isInt(v, lo, hi) { return typeof v === 'number' && isFinite(v) && Math.floor(v) === v && v >= lo && v <= hi; }
    function text(v, max) { return typeof v === 'string' ? v.slice(0, max) : ''; }
    function okReq(v) { return (typeof v === 'string' || typeof v === 'number') && String(v).length <= 64; }

    function cleanIssue(raw) {
        if (!isObject(raw)) return null;
        return {
            severity: ['error', 'warning', 'minor'].indexOf(raw.severity) >= 0 ? raw.severity : 'warning',
            code: text(raw.code, 40),
            path: text(raw.path, 400),
            message: text(raw.message, LIMITS.text),
            scene_id: text(raw.scene_id, LIMITS.id),
            fix: text(raw.fix, LIMITS.text)
        };
    }

    // checkIncoming(msg) -> a cleaned copy of a valid message, or null. The copy holds only the fields the schema names.
    function checkIncoming(msg) {
        if (!isObject(msg) || msg.sm !== 1 || TYPES.indexOf(msg.type) < 0) return null;
        switch (msg.type) {
            case 'ready':
                return { type: 'ready', caps: { workers: !!(isObject(msg.caps) && msg.caps.workers), offscreen: !!(isObject(msg.caps) && msg.caps.offscreen) } };
            case 'need_assets': {
                if (!okReq(msg.req) || !Array.isArray(msg.keys) || msg.keys.length > LIMITS.keys) return null;
                const keys = [];
                for (let i = 0; i < msg.keys.length; i++) {
                    if (typeof msg.keys[i] !== 'string' || msg.keys[i].length > LIMITS.keyLength) return null;
                    keys.push(msg.keys[i]);
                }
                return { type: 'need_assets', req: msg.req, keys };
            }
            case 'compiled': {
                const r = msg.report;
                if (!isObject(r) || !isInt(r.frame_count, 0, LIMITS.frame) || !isInt(r.fps, 1, 240) || !Array.isArray(r.scenes) || r.scenes.length > LIMITS.scenes) return null;
                const scenes = [];
                for (let i = 0; i < r.scenes.length; i++) {
                    const s = r.scenes[i];
                    if (!isObject(s) || typeof s.id !== 'string' || s.id.length > LIMITS.id || !isInt(s.start_frame, 0, LIMITS.frame) || !isInt(s.end_frame, 0, LIMITS.frame)) return null;
                    scenes.push({ id: s.id, start_frame: s.start_frame, end_frame: s.end_frame, kind: text(s.kind, 16) });
                }
                const issues = [], unverified = [];
                (Array.isArray(r.issues) ? r.issues.slice(0, LIMITS.issues) : []).forEach(raw => { const issue = cleanIssue(raw); if (issue) issues.push(issue); });
                (Array.isArray(r.unverified) ? r.unverified.slice(0, LIMITS.issues) : []).forEach(raw => { const issue = cleanIssue(raw); if (issue) unverified.push(issue); });
                return {
                    type: 'compiled',
                    report: {
                        frame_count: r.frame_count, fps: r.fps, scenes, issues, unverified,
                        width: isInt(r.width, 1, 16384) ? r.width : 0,
                        height: isInt(r.height, 1, 16384) ? r.height : 0,
                        duration_ms: isInt(r.duration_ms, 0, 1e12) ? r.duration_ms : Math.round(r.frame_count * 1000 / r.fps),
                        catalog_version: text(r.catalog_version, 64)
                    }
                };
            }
            case 'time':
                if (!isInt(msg.frame, 0, LIMITS.frame) || typeof msg.playing !== 'boolean') return null;
                return { type: 'time', frame: msg.frame, playing: msg.playing };
            case 'thumb': {
                if (!okReq(msg.req) || typeof msg.scene_id !== 'string' || msg.scene_id.length > LIMITS.id) return null;
                const isBitmap = typeof root.ImageBitmap === 'function' && msg.bitmap instanceof root.ImageBitmap;
                if (!isBitmap) return null;
                return { type: 'thumb', req: msg.req, scene_id: msg.scene_id, bitmap: msg.bitmap };
            }
            case 'error': {
                if (typeof msg.code !== 'string' || msg.code.length > 40) return null;
                const out = { type: 'error', code: msg.code, message: text(msg.message, LIMITS.text) };
                if (typeof msg.scene_id === 'string' && msg.scene_id.length <= LIMITS.id) out.scene_id = msg.scene_id;
                if (isInt(msg.line, 0, 1e7)) out.line = msg.line;
                if (isInt(msg.column, 0, 1e7)) out.column = msg.column;
                return out;
            }
            default:
                return null;
        }
    }

    // createPlayer({container, src, resolve, title}) -> player. resolve(keys) -> Promise<{assets, missing}>.
    // Events: ready(caps), loading, compiled(report), time({frame, playing}), error(error), dropped(count), unavailable.
    function createPlayer(options) {
        const o = options || {};
        const handlers = {};
        const emit = (name, payload) => (handlers[name] || []).slice().forEach(fn => { try { fn(payload); } catch (err) { console.error('storymaker player handler failed', name, err); } });
        const iframe = S.el('iframe', {
            class: 'stm-player',
            title: o.title || 'Storymaker preview',
            sandbox: 'allow-scripts',
            referrerpolicy: 'no-referrer',
            tabindex: '-1'
        });
        iframe.setAttribute('sandbox', 'allow-scripts');
        let ready = false, loaded = false, pending = null, disposed = false, dropped = 0, thumbSeq = 0, resume = 0, readyCount = 0, loadTimer = 0;
        const thumbs = new Map();

        function post(message, transfer) {
            const target = iframe.contentWindow;
            if (!target || disposed) return false;
            target.postMessage(Object.assign({ sm: 1 }, message), '*', transfer || []);
            return true;
        }

        function failThumbs(reason) {
            thumbs.forEach(entry => { clearTimeout(entry.timer); entry.reject(reason); });
            thumbs.clear();
        }

        function sendLoad() {
            if (!ready || !pending || disposed) return;
            const payload = pending;
            pending = null;
            loaded = post(Object.assign({ type: 'load' }, payload));
            if (loaded) emit('loading', payload);
        }

        function onMessage(event) {
            if (disposed || !iframe.contentWindow || event.source !== iframe.contentWindow) return;
            const msg = checkIncoming(event.data);
            if (!msg) { dropped++; emit('dropped', dropped); return; }
            switch (msg.type) {
                case 'ready':
                    // Every document announces itself once, so a ready after a reload starts from nothing: the host of the
                    // new document knows no story, and the thumbnails asked of the old one will never come.
                    ready = true;
                    loaded = false;
                    readyCount++;
                    failThumbs(new Error('reload'));
                    emit('ready', msg.caps);
                    sendLoad();
                    break;
                case 'need_assets': {
                    const answer = (assets, missing) => post({ type: 'assets', req: msg.req, assets, missing });
                    Promise.resolve(o.resolve ? o.resolve(msg.keys) : { assets: {}, missing: msg.keys }).then(
                        result => answer(result.assets || {}, result.missing || []),
                        () => answer({}, msg.keys));
                    break;
                }
                case 'compiled':
                    emit('compiled', msg.report);
                    break;
                case 'time':
                    if (resume > 0 && !msg.playing) {
                        const frame = resume;
                        resume = 0;
                        if (msg.frame !== frame) post({ type: 'seek', frame });
                    }
                    emit('time', { frame: msg.frame, playing: msg.playing });
                    break;
                case 'thumb': {
                    const entry = thumbs.get(String(msg.req));
                    if (entry) {
                        thumbs.delete(String(msg.req));
                        clearTimeout(entry.timer);
                        entry.resolve(msg.bitmap);
                    } else if (typeof msg.bitmap.close === 'function') {
                        msg.bitmap.close();
                    }
                    break;
                }
                case 'error':
                    emit('error', msg);
                    break;
                default:
                    break;
            }
        }

        // The message of the document and its load event arrive in either order, so the load event only arms a watchdog: a
        // document that does not say `ready` is reported as unavailable (CSP, a blocked script or a missing bundle).
        function onFrameLoad() {
            clearTimeout(loadTimer);
            const before = readyCount;
            loadTimer = setTimeout(() => { if (!disposed && readyCount === before && !ready) emit('unavailable'); }, 6000);
        }

        root.addEventListener('message', onMessage);
        iframe.addEventListener('load', onFrameLoad);
        iframe.src = o.src;
        if (o.container) o.container.appendChild(iframe);

        return {
            iframe,
            on(name, fn) {
                (handlers[name] = handlers[name] || []).push(fn);
                return () => { handlers[name] = (handlers[name] || []).filter(candidate => candidate !== fn); };
            },
            // load(payload, resumeFrame): the next `load` message (story, catalog, assets, fonts, code_bundles, format); the
            // frame is shown again after the compile (the host starts at frame 0).
            load(payload, resumeFrame) {
                pending = payload;
                resume = Number(resumeFrame) > 0 ? Math.floor(resumeFrame) : 0;
                failThumbs(new Error('superseded'));
                sendLoad();
            },
            play() { return loaded && post({ type: 'play' }); },
            pause() { return loaded && post({ type: 'pause' }); },
            seek(frame) { return loaded && post({ type: 'seek', frame: Math.max(0, Math.floor(frame)) }); },
            reset() { loaded = false; pending = null; failThumbs(new Error('reset')); return post({ type: 'reset' }); },
            // thumb(sceneId, width, frame?) -> Promise<ImageBitmap>
            thumb(sceneId, width, frame) {
                if (!loaded) return Promise.reject(new Error('not loaded'));
                const req = 't' + (++thumbSeq);
                return new Promise((resolve, reject) => {
                    const timer = setTimeout(() => { thumbs.delete(req); reject(new Error('timeout')); }, LIMITS.thumbWaitMs);
                    thumbs.set(req, { resolve, reject, timer });
                    const message = { type: 'thumb', req, scene_id: sceneId, width };
                    if (Number.isInteger(frame) && frame >= 0) message.frame = frame;
                    if (!post(message)) { clearTimeout(timer); thumbs.delete(req); reject(new Error('unavailable')); }
                });
            },
            isReady: () => ready,
            isLoaded: () => loaded,
            droppedCount: () => dropped,
            dispose() {
                if (disposed) return;
                failThumbs(new Error('disposed'));
                post({ type: 'reset' });
                disposed = true;
                clearTimeout(loadTimer);
                root.removeEventListener('message', onMessage);
                iframe.removeEventListener('load', onFrameLoad);
                iframe.remove();
            }
        };
    }

    S.createPlayer = createPlayer;
    S.player = { checkIncoming, LIMITS };
})(typeof globalThis !== 'undefined' ? globalThis : window);
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./ui -run 'TestStorymakerAppModelInNode|TestStorymakerPlayerBridgeIsSandboxed' -count=1`
Expected: ok

Recorded at drafting time:

```text
ok  	aurago/ui	0.545s
```

- [ ] **Step 5: Commit**

```bash
git add ui/desktop_storymaker_model_test.go ui/js/desktop/apps/storymaker-core.js ui/js/desktop/apps/storymaker-model.js ui/js/desktop/apps/storymaker-player.js
git commit -m "feat(storymaker): app core, story model and the sandboxed player bridge" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E3: API client and asset resolver

`storymaker-api.js` talks to `/api/desktop/storymaker` (contract section 5): If-Match on story and render, If-None-Match for a new code scene, an Idempotency-Key, flat errors, the project event stream. `storymaker-assets.js` builds what the player gets: the catalog, its rig, the fonts and the code bundles, and answers `need_assets` only with what the catalog lists or the story's code scenes own; every other key is reported missing.

**Files:**
- Create: `ui/js/desktop/apps/storymaker-api.js`
- Create: `ui/js/desktop/apps/storymaker-assets.js`
- Test: `ui/desktop_storymaker_api_test.go`

- [ ] **Step 1: Write the failing test**

`ui/desktop_storymaker_api_test.go`:

```go
package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The API client and the asset resolver of the Storymaker app in Node, against a fake fetch: the headers of every write (If-Match,
// If-None-Match, Idempotency-Key), the flat error shape and what the player may be given (only what the catalog lists, only the
// bundles of the story's code scenes, unknown keys reported as missing).
func TestStorymakerAppApiAndAssetsInNode(t *testing.T) {
	t.Parallel()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}
	script := filepath.Join(t.TempDir(), "storymaker-app-api.js")
	if err := os.WriteFile(script, []byte(storymakerApiScript), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, _ := filepath.Abs(filepath.Join("js", "desktop", "apps"))
	catalog, _ := filepath.Abs(filepath.Join("img", "storymaker", "catalog.json"))
	out, err := exec.Command(node, script, dir, catalog).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "storymaker app api ok") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

const storymakerApiScript = `
'use strict';
const fs = require('fs');
const vm = require('vm');
const path = require('path');
const [dir, catalogFile] = process.argv.slice(2);
const context = { console, AbortController, Response, Headers, setTimeout, clearTimeout, JSON, Promise, Map, Set, Array, Object, String, Number, Math, URL };
context.globalThis = context; context.window = context;
context.document = { documentElement: { lang: 'en' } };
vm.createContext(context);
for (const name of ['storymaker-core.js', 'storymaker-model.js', 'storymaker-api.js', 'storymaker-assets.js']) {
  vm.runInContext(fs.readFileSync(path.join(dir, name), 'utf8'), context, { filename: name });
}
const S = context.StorymakerParts;
const eq = (a, b, what) => { const x = JSON.stringify(a), y = JSON.stringify(b); if (x !== y) throw new Error(what + ': got ' + x + ', want ' + y); };
const catalogText = fs.readFileSync(catalogFile, 'utf8');
const catalog = JSON.parse(catalogText);

// A fake server: records every request, answers from a table of routes.
const calls = [];
let answers = {};
function fetcher(url, init) {
  calls.push({ url, method: init.method, headers: init.headers, body: init.body, credentials: init.credentials });
  const key = init.method + ' ' + url;
  const answer = answers[key];
  if (!answer) return Promise.resolve(new Response('{"error":"not_found","code":"not_found","message":"no route"}', { status: 404, headers: { 'Content-Type': 'application/json' } }));
  return Promise.resolve(typeof answer === 'function' ? answer() : answer);
}
// every answer is a factory: a Response body can be read once
const json = (value, status, headers) => () => new Response(JSON.stringify(value), { status: status || 200, headers: Object.assign({ 'Content-Type': 'application/json' }, headers || {}) });
const api = S.createApi({ fetch: fetcher });
const base = '/api/desktop/storymaker';
const id = '11111111-2222-4333-8444-555555555555';

async function rejects(promise, check, what) {
  try { await promise; } catch (err) { check(err); return; }
  throw new Error(what + ': did not reject');
}

(async () => {
  // writes carry their preconditions
  answers['PUT ' + base + '/projects/' + id + '/story'] = json({ etag: '"b"', issues: [], story: {} });
  await api.saveStory(id, '{"schema":"storymaker/1"}', '"a"');
  let call = calls.pop();
  eq([call.method, call.headers['If-Match'], call.headers['Content-Type'], call.body, call.credentials], ['PUT', '"a"', 'application/json', '{"schema":"storymaker/1"}', 'same-origin'], 'story save');

  answers['PUT ' + base + '/projects/' + id + '/scenes/s3/code'] = json({ etag: '"c"', ok: true, errors: [] });
  await api.putCode(id, 's3', 'export default function scene(api) {}', null);
  call = calls.pop();
  eq([call.headers['If-None-Match'], 'If-Match' in call.headers, call.headers['Content-Type']], ['*', false, 'text/javascript'], 'a new code scene is created, not replaced');
  await api.putCode(id, 's3', 'x', '"c"');
  call = calls.pop();
  eq([call.headers['If-Match'], 'If-None-Match' in call.headers], ['"c"', false], 'an existing code scene is replaced with its etag');

  answers['POST ' + base + '/projects/' + id + '/render'] = json({ job: { id: 'j1', status: 'queued' } }, 202);
  const job = await api.startRender(id, '"b"', 'key-1');
  call = calls.pop();
  eq([job.id, call.headers['If-Match'], call.headers['Idempotency-Key']], ['j1', '"b"', 'key-1'], 'render start');

  // the flat error shape
  answers['PUT ' + base + '/projects/' + id + '/story'] = json({ error: 'invalid_story', code: 'invalid_story', message: 'The story has errors.', issues: [{ severity: 'error', code: 'invalid_json', path: '', message: 'm' }] }, 422);
  await rejects(api.saveStory(id, '{', '"a"'), err => eq([err.status, err.code, err.issues.length, err.network], [422, 'invalid_story', 1, false], 'a 422 carries its issues'), 'invalid story');
  answers['PUT ' + base + '/projects/' + id + '/story'] = json({ error: 'precondition_failed', code: 'precondition_failed', message: 'stale' }, 412);
  await rejects(api.saveStory(id, '{}', '"old"'), err => eq(err.status, 412, 'a stale etag'), 'conflict');
  answers['GET ' + base + '/status'] = json({ error: 'storymaker_disabled', code: 'storymaker_disabled', message: 'off' }, 403);
  await rejects(api.status(), err => eq([err.status, err.code], [403, 'storymaker_disabled'], 'disabled'), 'disabled');
  await rejects(S.createApi({ fetch: () => Promise.reject(new TypeError('Failed to fetch')) }).status(), err => eq([err.network, err.status], [true, 0], 'a network failure'), 'network');
  const t = key => key;
  eq(S.errors.describe(t, { status: 412, code: 'precondition_failed' }), 'storymaker.error.conflict', 'text for a conflict');
  eq(S.errors.describe(t, { status: 428 }), 'storymaker.error.required', 'text for a missing precondition');
  eq(S.errors.describe(t, { status: 401, code: 'unauthorized' }), 'storymaker.error.unauthorized', 'text for signed out');
  eq(S.errors.describe(t, { status: 403, code: 'desktop_readonly' }), 'storymaker.error.readonly', 'text for read-only');
  eq(S.errors.describe(t, { status: 500, code: 'internal', message: 'boom' }), 'storymaker.error.generic boom', 'text for the rest keeps the server detail');
  eq(S.errors.describe(t, { status: 503, code: 'ffmpeg_unavailable' }), 'storymaker.job_error.ffmpeg_unavailable', 'a refused render start names the missing part');
  eq(S.errors.describe(t, { status: 429, code: 'busy' }), 'storymaker.error.busy', 'busy');

  // a missing bundle is null, not an error
  answers['GET ' + base + '/projects/' + id + '/scenes/s3/bundle'] = json({ error: 'not_found' }, 404);
  eq(await api.getBundle(id, 's3'), null, 'no bundle');

  // the asset resolver: the catalog files go out, everything else is missing
  const staticFiles = { '/img/storymaker/catalog.json': catalogText };
  const served = [];
  function staticFetch(url, init) {
    const hit = url.indexOf('/img/storymaker/') === 0 && url !== '/img/storymaker/catalog.json';
    served.push(url);
    if (staticFiles[url]) return Promise.resolve(new Response(staticFiles[url], { status: 200, headers: { 'Content-Type': 'application/json' } }));
    if (hit) return Promise.resolve(new Response(JSON.stringify({ file: url }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    if (url.indexOf('/fonts/') === 0) return Promise.resolve(new Response(new Uint8Array([1, 2, 3, 4]), { status: 200 }));
    return fetcher(url, init);
  }
  const assets = S.createAssets(S.createApi({ fetch: staticFetch }));
  const story = { style: { font: 'default' }, scenes: [{ id: 's1' }, { id: 's3', kind: 'code' }] };
  answers['GET ' + base + '/projects/' + id + '/scenes/s3/bundle'] = () => new Response('self.__smScene = function () {};', { status: 200, headers: { 'Content-Type': 'text/javascript' } });
  const prepared = await assets.prepare(story, id);
  eq([prepared.catalog.version, Object.keys(prepared.assets), prepared.fonts.map(f => f.weight), prepared.fonts[0].family, Object.keys(prepared.code_bundles)],
    [catalog.version, ['catalog:' + catalog.rig.file], [400, 500, 600, 700], 'Geist', ['s3']], 'the first load');
  eq(prepared.fonts[0].data.byteLength, 4, 'font bytes arrive as an ArrayBuffer');

  const props = catalog.entries.find(e => e.kind === 'prop');
  answers['GET ' + base + '/projects/' + id + '/scenes/s3/bundle'] = () => new Response('self.__smScene = 2;', { status: 200, headers: { 'Content-Type': 'text/javascript' } });
  const result = await assets.resolve([
    'catalog:' + props.file, 'catalog:../../etc/passwd', 'catalog:rig/other.json', 'font:default/400', 'font:default/450', 'font:nope/400',
    'bundle:s3', 'bundle:s1', 'bundle:s9', 'nonsense', 'catalog:' + 'x'.repeat(300)
  ], story, id);
  eq(Object.keys(result.assets).sort(), ['bundle:s3', 'catalog:' + props.file, 'font:default/400'].sort(), 'what is served');
  eq(result.missing.length, 8, 'everything else is reported missing, none silently');
  eq(served.some(u => u.indexOf('passwd') >= 0 || u.indexOf('other.json') >= 0), false, 'a path the catalog does not list is never requested');
  eq(result.assets['bundle:s3'], 'self.__smScene = 2;', 'the bundle is fetched fresh');
  console.log('storymaker app api ok');
})().catch(err => { console.error(err.stack || err); process.exit(1); });
`
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./ui -run 'TestStorymakerAppApiAndAssetsInNode' -count=1`
Expected: FAIL: FAIL (`storymaker-api.js` does not exist)

Recorded at drafting time (the replay of this task on a tree without it):

```text
--- FAIL: TestStorymakerAppApiAndAssetsInNode (0.12s)
    desktop_storymaker_api_test.go:28: node check failed: exit status 1
FAIL
FAIL	aurago/ui	0.407s
FAIL
```

- [ ] **Step 3: Implement**

`ui/js/desktop/apps/storymaker-api.js`:

```javascript
// Storymaker app API client: /api/desktop/storymaker (contract section 5) plus the static catalog files. Every failure is
// thrown as {stm, status, code, message, issues, network} (S.errors.normalize); the server's answers are flat
// {error, code, message, issues?}. Story saves carry If-Match, a new code scene If-None-Match: *, a render If-Match and an
// Idempotency-Key. The project event stream is an EventSource that hands every `job` event on. One AbortController per window
// cancels whatever is in flight when the window closes. Classic script, publishes S.createApi.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const BASE = '/api/desktop/storymaker';
    const CATALOG_BASE = '/img/storymaker/';
    const MAX_JSON_BYTES = 16 * 1048576;

    function createApi(options) {
        const o = options || {};
        const lifetime = new AbortController();
        const doFetch = o.fetch || ((url, init) => window.fetch(url, init));
        const makeEventSource = o.EventSource || (typeof window.EventSource === 'function' ? url => new window.EventSource(url) : null);

        async function request(method, path, init) {
            const spec = init || {};
            const headers = Object.assign({}, spec.headers || {});
            const options = { method, headers, credentials: 'same-origin', cache: 'no-store', signal: spec.signal || lifetime.signal };
            if (spec.body !== undefined) options.body = spec.body;
            let response;
            try {
                response = await doFetch(spec.raw ? path : BASE + path, options);
            } catch (err) {
                if (err && err.name === 'AbortError') throw Object.assign(S.errors.normalize({ message: 'aborted' }), { aborted: true });
                throw S.errors.normalize({ message: String(err && err.message || err) });
            }
            if (!response.ok) {
                let body = null;
                try { body = await response.json(); } catch (_) { body = null; }
                throw S.errors.fromResponse(response.status, body, response.headers.get('retry-after'));
            }
            return response;
        }

        async function json(method, path, init) {
            const response = await request(method, path, init);
            if (response.status === 204) return {};
            try {
                return await response.json();
            } catch (_) {
                throw S.errors.normalize({ message: 'The server answered with something that is not JSON.' });
            }
        }

        const jsonHeaders = extra => Object.assign({ 'Content-Type': 'application/json' }, extra || {});

        const api = {
            abort() { lifetime.abort(); },
            playerUrl() { return BASE + '/player'; },

            status() { return json('GET', '/status'); },
            async listProjects() {
                const body = await json('GET', '/projects');
                return Array.isArray(body.projects) ? body.projects : [];
            },
            createProject(spec) { return json('POST', '/projects', { headers: jsonHeaders(), body: JSON.stringify(spec) }); },
            getProject(id) { return json('GET', '/projects/' + encodeURIComponent(id)); },
            deleteProject(id) { return json('DELETE', '/projects/' + encodeURIComponent(id)); },

            // text is the story JSON; the answer is {etag, issues, defaults_applied, story}
            saveStory(id, text, etag) {
                return json('PUT', '/projects/' + encodeURIComponent(id) + '/story', { headers: jsonHeaders({ 'If-Match': etag }), body: text });
            },

            async getCode(id, scene) {
                const response = await request('GET', '/projects/' + encodeURIComponent(id) + '/scenes/' + encodeURIComponent(scene) + '/code');
                return { text: await response.text(), etag: response.headers.get('ETag') || '' };
            },
            // etag null creates the file (If-None-Match: *); the answer is {etag, ok, errors:[{line, column, message}]}
            putCode(id, scene, source, etag) {
                const headers = { 'Content-Type': 'text/javascript' };
                if (etag) headers['If-Match'] = etag; else headers['If-None-Match'] = '*';
                return json('PUT', '/projects/' + encodeURIComponent(id) + '/scenes/' + encodeURIComponent(scene) + '/code', { headers, body: source });
            },
            // the bundled scene for the player; null when the last bundle failed
            async getBundle(id, scene) {
                try {
                    const response = await request('GET', '/projects/' + encodeURIComponent(id) + '/scenes/' + encodeURIComponent(scene) + '/bundle');
                    return await response.text();
                } catch (err) {
                    if (err && err.status === 404) return null;
                    throw err;
                }
            },

            async startRender(id, etag, key) {
                const body = await json('POST', '/projects/' + encodeURIComponent(id) + '/render', { headers: { 'If-Match': etag, 'Idempotency-Key': key || S.uuid() } });
                return body.job || body;
            },
            async getJob(id) {
                const body = await json('GET', '/jobs/' + encodeURIComponent(id));
                return body.job || body;
            },
            async cancelJob(id) {
                const body = await json('POST', '/jobs/' + encodeURIComponent(id) + '/cancel');
                return body.job || body;
            },

            // events(projectId, onJob, onState) -> {close()}; onState('open' | 'error')
            events(projectId, onJob, onState) {
                if (!makeEventSource) return { close() {} };
                let source = null;
                try { source = makeEventSource(BASE + '/projects/' + encodeURIComponent(projectId) + '/events'); } catch (_) { return { close() {} }; }
                source.addEventListener('job', event => {
                    let body = null;
                    try { body = JSON.parse(event.data); } catch (_) { return; }
                    const job = body && body.job ? body.job : body;
                    if (job && typeof job === 'object' && typeof job.id === 'string') onJob(job);
                });
                source.addEventListener('open', () => onState && onState('open'));
                source.addEventListener('error', () => onState && onState('error'));
                return { close() { try { source.close(); } catch (_) { /* gone */ } } };
            },

            // static files below /img/storymaker and /fonts: the catalog, its part files and the fonts
            async catalogJSON(path) {
                const response = await request('GET', CATALOG_BASE + String(path).replace(/^\/+/, ''), { raw: true });
                const text = await response.text();
                if (text.length > MAX_JSON_BYTES) throw S.errors.normalize({ message: 'The catalog file is too large.' });
                return JSON.parse(text);
            },
            async fontBuffer(file) {
                const response = await request('GET', String(file), { raw: true });
                const buffer = await response.arrayBuffer();
                if (buffer.byteLength > 8 * 1048576) throw S.errors.normalize({ message: 'The font is too large.' });
                return buffer;
            }
        };
        return api;
    }

    S.createApi = createApi;
    S.API_BASE = BASE;
})();
```

`ui/js/desktop/apps/storymaker-assets.js`:

```javascript
// Storymaker app assets: what the parent hands the sandboxed player. The catalog (catalog.json and the part files it lists)
// and the fonts come from the static files below /img/storymaker and /fonts, the code-scene bundles from the project API.
// prepare() builds the first `load` message; resolve() answers `need_assets` and serves only what the catalog lists
// (rig.file, entries[].file, its fonts) or the bundle of a code scene of the loaded story; every other key is answered in
// `missing`, never silently. Parsed files and font bytes are cached per catalog version. Classic script, publishes
// S.createAssets.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const KEY_RE = /^(catalog|font|bundle):(\S{1,230})$/;

    function createAssets(api) {
        let catalogPromise = null;
        let current = null;           // {raw, index, paths}
        const cache = new Map();      // asset key -> parsed JSON or ArrayBuffer

        function allowedPaths(raw) {
            const paths = new Set();
            if (raw.rig && typeof raw.rig.file === 'string') paths.add(raw.rig.file);
            (raw.entries || []).forEach(entry => { if (entry && typeof entry.file === 'string') paths.add(entry.file); });
            return paths;
        }

        function loadCatalog() {
            if (!catalogPromise) {
                catalogPromise = api.catalogJSON('catalog.json').then(raw => {
                    if (!raw || raw.schema !== 'storymaker-catalog/1' || !Array.isArray(raw.entries)) throw S.errors.normalize({ message: 'The catalog has an unknown format.' });
                    if (current && current.raw.version !== raw.version) cache.clear();
                    current = { raw, index: S.model.indexCatalog(raw), paths: allowedPaths(raw) };
                    return current;
                }, err => {
                    catalogPromise = null;
                    throw err;
                });
            }
            return catalogPromise;
        }

        async function cached(key, fetcher) {
            if (cache.has(key)) return cache.get(key);
            const value = await fetcher();
            cache.set(key, value);
            return value;
        }

        function fontFaces(raw, name) {
            return (raw.fonts || []).filter(font => font && font.name === name && typeof font.file === 'string' && /^\/fonts\/[A-Za-z0-9_.-]+$/.test(font.file));
        }

        // The fonts the story uses, as the `load.fonts` entries {name, family, weight, data}.
        async function fontsFor(raw, story) {
            const name = story && story.style && story.style.font ? story.style.font : 'default';
            const faces = fontFaces(raw, name);
            return Promise.all(faces.map(async face => ({
                name: face.name,
                family: String(face.family),
                weight: Number(face.weight),
                data: await cached('font:' + face.name + '/' + face.weight, () => api.fontBuffer(face.file))
            })));
        }

        function codeSceneIds(story) {
            return S.model.scenesOf(story).filter(scene => scene && scene.kind === 'code' && typeof scene.id === 'string').map(scene => scene.id);
        }

        // prepare(story, projectId) -> the parts of the `load` message that do not depend on the story text
        async function prepare(story, projectId) {
            const { raw } = await loadCatalog();
            const assets = {};
            if (raw.rig && raw.rig.file) assets['catalog:' + raw.rig.file] = await cached('catalog:' + raw.rig.file, () => api.catalogJSON(raw.rig.file));
            const fonts = await fontsFor(raw, story);
            const bundles = {};
            await Promise.all(codeSceneIds(story).map(async id => {
                try {
                    const text = await api.getBundle(projectId, id);
                    if (typeof text === 'string') bundles[id] = text;
                } catch (_) { /* the player reports the scene without a bundle */ }
            }));
            return { catalog: raw, assets, fonts, code_bundles: bundles };
        }

        // resolve(keys, story, projectId) -> {assets, missing}
        async function resolve(keys, story, projectId) {
            const { raw, paths } = await loadCatalog();
            const assets = {};
            const missing = [];
            const codeIds = new Set(codeSceneIds(story));
            await Promise.all((Array.isArray(keys) ? keys : []).map(async key => {
                const m = typeof key === 'string' ? KEY_RE.exec(key) : null;
                if (!m) { missing.push(String(key).slice(0, 240)); return; }
                try {
                    if (m[1] === 'catalog') {
                        if (!paths.has(m[2])) { missing.push(key); return; }
                        assets[key] = await cached(key, () => api.catalogJSON(m[2]));
                    } else if (m[1] === 'font') {
                        const parts = /^([^/]+)\/(\d{3})$/.exec(m[2]);
                        const face = parts && fontFaces(raw, parts[1]).find(font => String(font.weight) === parts[2]);
                        if (!face) { missing.push(key); return; }
                        assets[key] = await cached(key, () => api.fontBuffer(face.file));
                    } else if (m[1] === 'bundle') {
                        if (!codeIds.has(m[2])) { missing.push(key); return; }
                        const text = await api.getBundle(projectId, m[2]);
                        if (typeof text === 'string') assets[key] = text; else missing.push(key);
                    }
                } catch (_) {
                    missing.push(key);
                }
            }));
            return { assets, missing };
        }

        return { loadCatalog, prepare, resolve, index: () => (current ? current.index : null), raw: () => (current ? current.raw : null) };
    }

    S.createAssets = createAssets;
})();
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./ui -run 'TestStorymakerAppApiAndAssetsInNode' -count=1`
Expected: ok

Recorded at drafting time:

```text
ok  	aurago/ui	0.388s
```

- [ ] **Step 5: Commit**

```bash
git add ui/desktop_storymaker_api_test.go ui/js/desktop/apps/storymaker-api.js ui/js/desktop/apps/storymaker-assets.js
git commit -m "feat(storymaker): app API client and asset resolver" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E4: App strings in 16 languages

All `storymaker.*` UI strings (208 keys, flat) in `ui/lang/storymaker/<lang>.json`. The directory belongs to the app; `storymaker.catalog.*` belongs to the catalog (`ui/lang/storymaker_catalog`) and is never defined here. The test checks completeness, placeholders, that nothing but the few shared names stays English, and that the two directories keep to their own keys.

**Files:**
- Create: `ui/lang/storymaker/cs.json`
- Create: `ui/lang/storymaker/da.json`
- Create: `ui/lang/storymaker/de.json`
- Create: `ui/lang/storymaker/el.json`
- Create: `ui/lang/storymaker/en.json`
- Create: `ui/lang/storymaker/es.json`
- Create: `ui/lang/storymaker/fr.json`
- Create: `ui/lang/storymaker/hi.json`
- Create: `ui/lang/storymaker/it.json`
- Create: `ui/lang/storymaker/ja.json`
- Create: `ui/lang/storymaker/nl.json`
- Create: `ui/lang/storymaker/no.json`
- Create: `ui/lang/storymaker/pl.json`
- Create: `ui/lang/storymaker/pt.json`
- Create: `ui/lang/storymaker/sv.json`
- Create: `ui/lang/storymaker/zh.json`
- Test: `ui/desktop_storymaker_i18n_test.go`

- [ ] **Step 1: Write the failing test**

`ui/desktop_storymaker_i18n_test.go`:

```go
package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var storymakerPlaceholder = regexp.MustCompile(`\{\{?\w+\}\}?`)

func storymakerLocale(t *testing.T, dir, lang string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("lang", dir, lang+".json"))
	if err != nil {
		t.Fatalf("%s/%s.json: %v", dir, lang, err)
	}
	values := map[string]string{}
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("%s/%s.json must be a flat map of strings: %v", dir, lang, err)
	}
	return values
}

func storymakerPlaceholders(text string) string {
	found := storymakerPlaceholder.FindAllString(text, -1)
	sort.Strings(found)
	return strings.Join(found, " ")
}

func TestStorymakerLocalesCoverAllSixteenLanguages(t *testing.T) {
	t.Parallel()
	english := map[string]map[string]string{
		"storymaker":         storymakerLocale(t, "storymaker", "en"),
		"storymaker_catalog": storymakerLocale(t, "storymaker_catalog", "en"),
	}
	if len(english["storymaker"]) < 150 {
		t.Fatalf("lang/storymaker/en.json holds %d keys, the app needs far more", len(english["storymaker"]))
	}
	for _, lang := range storymakerLanguages {
		for dir, base := range english {
			values := storymakerLocale(t, dir, lang)
			for key := range base {
				text := strings.TrimSpace(values[key])
				if text == "" {
					t.Errorf("%s/%s.json lacks %s", dir, lang, key)
					continue
				}
				if storymakerPlaceholders(text) != storymakerPlaceholders(base[key]) {
					t.Errorf("%s/%s.json %s has the placeholders %q, English has %q", dir, lang, key, storymakerPlaceholders(text), storymakerPlaceholders(base[key]))
				}
				if strings.ContainsRune(text, '�') {
					t.Errorf("%s/%s.json %s holds a broken character", dir, lang, key)
				}
			}
			for key := range values {
				if _, ok := base[key]; !ok {
					t.Errorf("%s/%s.json has %s, which English lacks", dir, lang, key)
				}
			}
			// The directories keep to their own keys: the catalog owns storymaker.catalog.*, the app everything else.
			for key := range values {
				isCatalog := strings.HasPrefix(key, "storymaker.catalog.")
				if (dir == "storymaker_catalog") != isCatalog || !strings.HasPrefix(key, "storymaker.") {
					t.Errorf("%s/%s.json holds %s, which belongs to the other directory or to no storymaker section", dir, lang, key)
				}
			}
		}
	}
}

// A translation that is the English text is a missed translation, except for the names and words that really are the same
// (the product, FFmpeg, Pause or Code in a few languages, the format name). The list is exact: a new key that a language copies
// from English fails here.
func TestStorymakerAppTranslationsAreTranslated(t *testing.T) {
	t.Parallel()
	english := storymakerLocale(t, "storymaker", "en")
	same := map[string]bool{
		"storymaker.title": true, "storymaker.tab_stage": true, "storymaker.strip.duration_s": true, "storymaker.export.cap_ffmpeg": true,
		"storymaker.pause": true, "storymaker.side_scene": true, "storymaker.projects.new_format": true, "storymaker.strip.scene_n": true,
		"storymaker.export.cap_browser": true, "storymaker.tab_code": true, "storymaker.code.title": true, "storymaker.aspect_9_16": true,
	}
	for _, lang := range storymakerLanguages {
		if lang == "en" {
			continue
		}
		values := storymakerLocale(t, "storymaker", lang)
		for key, text := range english {
			if values[key] == text && !same[key] {
				t.Errorf("%s keeps the English text of %s: %q", lang, key, text)
			}
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./ui -run 'TestStorymakerLocalesCoverAllSixteenLanguages|TestStorymakerAppTranslationsAreTranslated' -count=1`
Expected: FAIL: FAIL (`lang/storymaker/en.json` does not exist)

Recorded at drafting time (the replay of this task on a tree without it):

```text
--- FAIL: TestStorymakerLocalesCoverAllSixteenLanguages (0.00s)
    desktop_storymaker_i18n_test.go:37: storymaker/en.json: open lang\storymaker\en.json: The system cannot find the file specified.
--- FAIL: TestStorymakerAppTranslationsAreTranslated (0.00s)
    desktop_storymaker_i18n_test.go:80: storymaker/en.json: open lang\storymaker\en.json: The system cannot find the file specified.
FAIL
FAIL	aurago/ui	0.581s
FAIL
```

- [ ] **Step 3: Implement**

`ui/lang/storymaker/cs.json`:

```json
{
  "storymaker.title": "Storymaker",
  "storymaker.loading": "Načítání…",
  "storymaker.retry": "Zkusit znovu",
  "storymaker.close": "Zavřít",
  "storymaker.delete": "Smazat",
  "storymaker.back_to_projects": "Projekty",
  "storymaker.preview": "Náhled Storymakeru",
  "storymaker.play": "Přehrát",
  "storymaker.pause": "Pozastavit",
  "storymaker.timeline": "Časová osa",
  "storymaker.tab_stage": "Storyboard",
  "storymaker.tab_code": "Kód",
  "storymaker.side_scene": "Scéna",
  "storymaker.side_library": "Knihovna",
  "storymaker.save_saved": "Uloženo",
  "storymaker.save_dirty": "Neuložené změny",
  "storymaker.save_saving": "Ukládání…",
  "storymaker.save_error": "Neuloženo",
  "storymaker.save_offline": "Offline, neuloženo",
  "storymaker.save_conflict": "Změněno jinde",
  "storymaker.save_readonly": "Jen pro čtení",
  "storymaker.leave_title": "Odejít s neuloženými změnami?",
  "storymaker.leave_text": "Tvé poslední změny v příběhu se nepodařilo uložit. Pokud teď odejdeš, přijdeš o ně.",
  "storymaker.leave_code_text": "Soubor s kódem má neuložené změny. Pokud teď odejdeš, přijdeš o ně.",
  "storymaker.aspect_16_9": "Na šířku 16:9",
  "storymaker.aspect_9_16": "Na výšku 9:16",
  "storymaker.aspect_1_1": "Čtverec 1:1",

  "storymaker.banner.readonly": "Plocha je jen pro čtení. Náhled si můžeš prohlížet, ale úpravy, ukládání a export jsou vypnuté.",
  "storymaker.banner.conflict": "Tento příběh byl změněn jinde, například v jiném okně. Načti ho znovu, abys viděl aktuální verzi, nebo si nech svou a přepiš.",
  "storymaker.banner.conflict_reload": "Načíst znovu",
  "storymaker.banner.conflict_overwrite": "Ponechat moji",
  "storymaker.banner.player_unavailable": "Náhled se v tomto prohlížeči nepodařilo spustit. Tvůj příběh je přesto uložený a můžeš ho dál upravovat.",
  "storymaker.banner.export_unavailable": "Export zatím není k dispozici (chybí: {missing}). Úpravy a náhled fungují dál.",

  "storymaker.state.disabled_title": "Storymaker je vypnutý",
  "storymaker.state.disabled_text": "Správce ho může zapnout v konfiguraci v části Virtuální plocha.",
  "storymaker.state.unauthorized_title": "Přihlas se znovu",
  "storymaker.state.unauthorized_text": "Tvoje relace skončila. Přihlas se znovu a otevři Storymaker znovu.",
  "storymaker.state.scope_title": "Bez přístupu",
  "storymaker.state.scope_text": "Toto přihlášení nesmí používat Virtuální plochu.",
  "storymaker.state.load_failed_title": "Storymaker se nepodařilo spustit",
  "storymaker.state.load_failed_text": "Server neodpověděl podle očekávání. Zkontroluj připojení a zkus to znovu.",

  "storymaker.projects.title": "Projekty",
  "storymaker.projects.intro": "Udělej z tématu krátké vysvětlující video: nainscenuj scény, napiš komentář a exportuj soubor MP4.",
  "storymaker.projects.new_title_label": "Název",
  "storymaker.projects.new_title_placeholder": "Co je firewall?",
  "storymaker.projects.new_format": "Formát",
  "storymaker.projects.create": "Nový projekt",
  "storymaker.projects.title_required": "Zadej název.",
  "storymaker.projects.empty_title": "Zatím žádné projekty",
  "storymaker.projects.empty_text": "Vytvoř nahoře svůj první projekt. Začíná jednou prázdnou scénou.",
  "storymaker.projects.delete_label": "Smazat projekt {title}",
  "storymaker.projects.delete_title": "Smazat projekt?",
  "storymaker.projects.delete_text": "Projekt „{title}“ a všechny jeho soubory včetně exportů budou smazány. Nelze to vrátit zpět.",
  "storymaker.projects.deleted": "Projekt „{title}“ byl smazán.",

  "storymaker.strip.title": "Scény",
  "storymaker.strip.scene_n": "Scéna {n}",
  "storymaker.strip.duration_s": "{seconds} s",
  "storymaker.strip.kind_code": "Kódová scéna",
  "storymaker.strip.add_storyboard": "Přidat scénu",
  "storymaker.strip.add_code": "Přidat kódovou scénu",

  "storymaker.stage.compiling": "Připravuje se náhled…",
  "storymaker.stage.unavailable": "Náhled není k dispozici.",
  "storymaker.stage.player_error": "Náhled nahlásil problém.",
  "storymaker.stage.code_error": "Kódová scéna obsahuje chybu. Podívej se do seznamu problémů.",

  "storymaker.inspector.title": "Nastavení scény",
  "storymaker.inspector.environment": "Prostředí",
  "storymaker.inspector.variant": "Denní doba",
  "storymaker.inspector.variant_day": "Den",
  "storymaker.inspector.variant_night": "Noc",
  "storymaker.inspector.narration": "Komentář",
  "storymaker.inspector.narration_placeholder": "Co říká vypravěč v této scéně?",
  "storymaker.inspector.narration_hint": "Scéna trvá tak dlouho, jak dlouho se text čte, pokud nenastavíš délku.",
  "storymaker.inspector.transition": "Přechod do této scény",
  "storymaker.inspector.transition_first": "První scéna nemá přechod.",
  "storymaker.inspector.unsupported": "{name} (zatím nepodporováno)",
  "storymaker.inspector.duration": "Délka",
  "storymaker.inspector.duration_auto": "Automatická",
  "storymaker.inspector.duration_fixed": "Pevná",
  "storymaker.inspector.duration_seconds": "Délka v sekundách",
  "storymaker.inspector.seconds_unit": "sekund",
  "storymaker.inspector.cast_props": "Postavy a rekvizity v této scéně",
  "storymaker.inspector.no_items": "Zatím nic. Vyber postavy a rekvizity v knihovně.",
  "storymaker.inspector.remove": "Odebrat",
  "storymaker.inspector.remove_item": "Odebrat {name} ze scény",
  "storymaker.inspector.move_up": "Posunout scénu nahoru",
  "storymaker.inspector.move_down": "Posunout scénu dolů",
  "storymaker.inspector.delete_scene": "Smazat scénu",
  "storymaker.inspector.delete_scene_text": "Smazat scénu {scene}? Její komentář a vše v ní bude z příběhu odstraněno.",
  "storymaker.inspector.code_scene_note": "Tuto scénu kreslí malý program.",
  "storymaker.inspector.edit_code": "Upravit kód",

  "storymaker.library.title": "Knihovna",
  "storymaker.library.people": "Postavy",
  "storymaker.library.props": "Rekvizity",
  "storymaker.library.environments": "Místa",
  "storymaker.library.search": "Hledat v knihovně",
  "storymaker.library.add": "Přidat do scény",
  "storymaker.library.use": "Použít jako prostředí",
  "storymaker.library.empty": "Nic nevyhovuje.",
  "storymaker.library.code_scene": "Kódové scény kreslí všechno samy.",
  "storymaker.library.already_in_scene": "Už je v této scéně.",
  "storymaker.library.added_actor": "Do scény přidáno: {name}.",
  "storymaker.library.added_prop": "Do scény přidáno: {name}.",

  "storymaker.issues.title": "Problémy",
  "storymaker.issues.none": "Žádné problémy.",
  "storymaker.issues.count": "Problémy: {count}",
  "storymaker.issues.show": "Zobrazit problémy",
  "storymaker.issues.show_in_code": "Zobrazit v story.json",

  "storymaker.issue.invalid_json": "Neplatný JSON",
  "storymaker.issue.unknown_field": "Neznámé pole",
  "storymaker.issue.schema_mismatch": "Špatný formát příběhu",
  "storymaker.issue.too_large": "Příliš velké",
  "storymaker.issue.limit_exceeded": "Překročen limit",
  "storymaker.issue.missing_field": "Chybějící pole",
  "storymaker.issue.invalid_value": "Neplatná hodnota",
  "storymaker.issue.duplicate_id": "Duplicitní název",
  "storymaker.issue.unknown_name": "Neznámý název",
  "storymaker.issue.unknown_asset": "Neznámá položka katalogu",
  "storymaker.issue.unknown_spot": "Neznámé místo",
  "storymaker.issue.unknown_cast": "Neznámá postava",
  "storymaker.issue.unknown_cue": "Neznámá značka",
  "storymaker.issue.canonicalised": "Název sjednocen",
  "storymaker.issue.not_supported_yet": "Zatím nepodporováno",
  "storymaker.issue.narration_missing": "Chybí zvuk komentáře",
  "storymaker.issue.code_error": "Chyba v kódové scéně",
  "storymaker.issue.deadline": "Trvalo to příliš dlouho",
  "storymaker.issue.layout_unverified": "Rozvržení nebylo zkontrolováno",
  "storymaker.issue.unknown": "Problém",

  "storymaker.code.title": "Kód",
  "storymaker.code.files": "Soubory",
  "storymaker.code.editor_label": "Editor kódu",
  "storymaker.code.save": "Uložit",
  "storymaker.code.saved": "Uloženo",
  "storymaker.code.unsaved": "Neuložené změny",
  "storymaker.code.stale": "Změněno. Ulož pro novou kontrolu.",
  "storymaker.code.new_file": "Nový soubor, zatím neuložený",
  "storymaker.code.reload": "Načíst znovu",
  "storymaker.code.bundle_ok": "Kódová scéna uložena.",
  "storymaker.code.bundle_failed": "Kód obsahuje chyby.",
  "storymaker.code.no_problems": "Žádné problémy nenalezeny.",
  "storymaker.code.problem_count": "Problémy: {count}",
  "storymaker.code.line_col": "Řádek {line}, sloupec {column}",
  "storymaker.code.more": "a dalších {count}",

  "storymaker.export.button": "Exportovat",
  "storymaker.export.button_running": "Export {percent} %",
  "storymaker.export.title": "Export videa",
  "storymaker.export.cap_ffmpeg": "FFmpeg",
  "storymaker.export.cap_browser": "Prohlížeč",
  "storymaker.export.ready": "Připraveno",
  "storymaker.export.ready_version": "Připraveno, verze {version}",
  "storymaker.export.ffmpeg_missing": "Nenalezeno. Nainstaluj FFmpeg nebo zadej jeho cestu v nastavení Video Studia.",
  "storymaker.export.browser_ready": "Připraveno ({source})",
  "storymaker.export.browser_missing": "Pro kreslení snímků není k dispozici žádný prohlížeč.",
  "storymaker.export.source_config": "z konfigurace",
  "storymaker.export.source_system": "nainstalován v tomto systému",
  "storymaker.export.source_pinned": "stažen aplikací AuraGo",
  "storymaker.export.source_unknown": "neznámý zdroj",
  "storymaker.export.still_works": "Úpravy a náhled fungují i bez nich.",
  "storymaker.export.blocked_readonly": "Plocha je jen pro čtení.",
  "storymaker.export.blocked_ffmpeg": "Chybí FFmpeg.",
  "storymaker.export.blocked_browser": "Není k dispozici žádný prohlížeč.",
  "storymaker.export.blocked_errors": "Příběh obsahuje chyby. Nejdřív je oprav.",
  "storymaker.export.blocked_too_long": "Video je delší než limit {max} sekund.",
  "storymaker.export.blocked_not_ready": "Náhled se ještě připravuje.",
  "storymaker.export.summary": "Délka {duration}, {format}, {width} × {height} pixelů.",
  "storymaker.export.silent_note": "Tato verze exportuje videa bez zvuku. Zvuk komentáře přijde později.",
  "storymaker.export.start": "Spustit export",
  "storymaker.export.starting": "Spouštění…",
  "storymaker.export.queued": "Čeká na řadu…",
  "storymaker.export.phase_starting": "Spouštění",
  "storymaker.export.phase_compiling": "Příprava příběhu",
  "storymaker.export.phase_mixing": "Míchání zvuku",
  "storymaker.export.phase_rendering": "Kreslení snímků",
  "storymaker.export.phase_encoding": "Kódování videa",
  "storymaker.export.phase_writing": "Ukládání souboru",
  "storymaker.export.frames": "{done} z {total} snímků",
  "storymaker.export.cancel": "Zrušit export",
  "storymaker.export.cancelling": "Ruší se…",
  "storymaker.export.cancelled": "Export byl zrušen.",
  "storymaker.export.interrupted": "Export byl přerušen restartem.",
  "storymaker.export.done": "Export dokončen",
  "storymaker.export.output": "Video je ve složce tvého projektu:",
  "storymaker.export.show_in_files": "Zobrazit v Souborech",
  "storymaker.export.copy_path": "Kopírovat cestu",
  "storymaker.export.copied": "Zkopírováno",
  "storymaker.export.copy_failed": "Cestu se nepodařilo zkopírovat.",
  "storymaker.export.again": "Exportovat znovu",

  "storymaker.job_error.invalid_story": "Příběh obsahuje chyby, proto ho nelze exportovat.",
  "storymaker.job_error.storymaker_disabled": "Storymaker je vypnutý.",
  "storymaker.job_error.browser_unavailable": "Nepodařilo se spustit žádný prohlížeč pro kreslení snímků.",
  "storymaker.job_error.ffmpeg_unavailable": "FFmpeg není k dispozici.",
  "storymaker.job_error.render_failed": "Export se nezdařil.",
  "storymaker.job_error.render_timeout": "Export trval příliš dlouho a byl zastaven.",
  "storymaker.job_error.cancelled": "Export byl zrušen.",
  "storymaker.job_error.busy": "Probíhá jiný export. Zkus to za chvíli znovu.",
  "storymaker.job_error.code_error": "Kódová scéna selhala při kreslení.",
  "storymaker.job_error.unknown": "Export se nezdařil.",

  "storymaker.error.unauthorized": "Tvoje relace skončila. Přihlas se znovu.",
  "storymaker.error.scope": "Toto přihlášení to dělat nesmí.",
  "storymaker.error.readonly": "Plocha je jen pro čtení.",
  "storymaker.error.disabled": "Storymaker je vypnutý.",
  "storymaker.error.invalid_story": "Příběh nebyl uložen, protože obsahuje chyby.",
  "storymaker.error.conflict": "Příběh byl změněn jinde.",
  "storymaker.error.required": "Server potřebuje aktuální verzi příběhu. Načti ho znovu.",
  "storymaker.error.too_large": "To je příliš velké.",
  "storymaker.error.not_found": "To už neexistuje.",
  "storymaker.error.busy": "Server je zaneprázdněný. Zkus to za chvíli znovu.",
  "storymaker.error.network": "Server není dostupný.",
  "storymaker.error.generic": "Něco se pokazilo."
}
```

`ui/lang/storymaker/da.json`:

```json
{
  "storymaker.title": "Storymaker",
  "storymaker.loading": "Indlæser …",
  "storymaker.retry": "Prøv igen",
  "storymaker.close": "Luk",
  "storymaker.delete": "Slet",
  "storymaker.back_to_projects": "Projekter",
  "storymaker.preview": "Forhåndsvisning i Storymaker",
  "storymaker.play": "Afspil",
  "storymaker.pause": "Pause",
  "storymaker.timeline": "Tidslinje",
  "storymaker.tab_stage": "Storyboard",
  "storymaker.tab_code": "Kode",
  "storymaker.side_scene": "Scene",
  "storymaker.side_library": "Bibliotek",
  "storymaker.save_saved": "Gemt",
  "storymaker.save_dirty": "Ikke gemte ændringer",
  "storymaker.save_saving": "Gemmer …",
  "storymaker.save_error": "Ikke gemt",
  "storymaker.save_offline": "Offline, ikke gemt",
  "storymaker.save_conflict": "Ændret et andet sted",
  "storymaker.save_readonly": "Skrivebeskyttet",
  "storymaker.leave_title": "Forlade med ikke gemte ændringer?",
  "storymaker.leave_text": "Dine seneste ændringer i historien kunne ikke gemmes. Hvis du forlader siden nu, går de tabt.",
  "storymaker.leave_code_text": "En kodefil har ikke gemte ændringer. Hvis du forlader siden nu, går de tabt.",
  "storymaker.aspect_16_9": "Liggende 16:9",
  "storymaker.aspect_9_16": "Stående 9:16",
  "storymaker.aspect_1_1": "Kvadrat 1:1",

  "storymaker.banner.readonly": "Skrivebordet er skrivebeskyttet. Du kan se forhåndsvisningen, men redigering, gemning og eksport er slået fra.",
  "storymaker.banner.conflict": "Denne historie er ændret et andet sted, for eksempel i et andet vindue. Indlæs igen for at se den aktuelle version, eller behold din og overskriv.",
  "storymaker.banner.conflict_reload": "Indlæs igen",
  "storymaker.banner.conflict_overwrite": "Behold min",
  "storymaker.banner.player_unavailable": "Forhåndsvisningen kunne ikke starte i denne browser. Din historie er stadig gemt, og du kan blive ved med at redigere den.",
  "storymaker.banner.export_unavailable": "Eksport er ikke tilgængelig endnu ({missing} mangler). Redigering og forhåndsvisning virker stadig.",

  "storymaker.state.disabled_title": "Storymaker er slået fra",
  "storymaker.state.disabled_text": "En administrator kan slå det til i konfigurationen under Virtuelt skrivebord.",
  "storymaker.state.unauthorized_title": "Log ind igen",
  "storymaker.state.unauthorized_text": "Din session er udløbet. Log ind igen, og åbn Storymaker igen.",
  "storymaker.state.scope_title": "Ingen adgang",
  "storymaker.state.scope_text": "Denne login må ikke bruge det virtuelle skrivebord.",
  "storymaker.state.load_failed_title": "Storymaker kunne ikke starte",
  "storymaker.state.load_failed_text": "Serveren svarede ikke som forventet. Tjek forbindelsen, og prøv igen.",

  "storymaker.projects.title": "Projekter",
  "storymaker.projects.intro": "Gør et emne til en kort forklaringsvideo: iscenesæt scenerne, skriv fortællerteksten, og eksporter en MP4.",
  "storymaker.projects.new_title_label": "Titel",
  "storymaker.projects.new_title_placeholder": "Hvad er en firewall?",
  "storymaker.projects.new_format": "Format",
  "storymaker.projects.create": "Nyt projekt",
  "storymaker.projects.title_required": "Skriv en titel.",
  "storymaker.projects.empty_title": "Ingen projekter endnu",
  "storymaker.projects.empty_text": "Opret dit første projekt ovenfor. Det starter med én tom scene.",
  "storymaker.projects.delete_label": "Slet projektet {title}",
  "storymaker.projects.delete_title": "Slet projektet?",
  "storymaker.projects.delete_text": "Projektet »{title}« og alle dets filer, også eksporterne, slettes. Det kan ikke fortrydes.",
  "storymaker.projects.deleted": "»{title}« er slettet.",

  "storymaker.strip.title": "Scener",
  "storymaker.strip.scene_n": "Scene {n}",
  "storymaker.strip.duration_s": "{seconds} s",
  "storymaker.strip.kind_code": "Kodescene",
  "storymaker.strip.add_storyboard": "Tilføj scene",
  "storymaker.strip.add_code": "Tilføj kodescene",

  "storymaker.stage.compiling": "Forbereder forhåndsvisningen …",
  "storymaker.stage.unavailable": "Forhåndsvisningen er ikke tilgængelig.",
  "storymaker.stage.player_error": "Forhåndsvisningen meldte om et problem.",
  "storymaker.stage.code_error": "En kodescene har en fejl. Se listen over problemer.",

  "storymaker.inspector.title": "Sceneindstillinger",
  "storymaker.inspector.environment": "Miljø",
  "storymaker.inspector.variant": "Tid på dagen",
  "storymaker.inspector.variant_day": "Dag",
  "storymaker.inspector.variant_night": "Nat",
  "storymaker.inspector.narration": "Fortællertekst",
  "storymaker.inspector.narration_placeholder": "Hvad siger fortælleren i denne scene?",
  "storymaker.inspector.narration_hint": "Scenen varer lige så længe, som teksten tager at læse, medmindre du angiver en varighed.",
  "storymaker.inspector.transition": "Overgang ind i denne scene",
  "storymaker.inspector.transition_first": "Den første scene har ingen overgang.",
  "storymaker.inspector.unsupported": "{name} (understøttes ikke endnu)",
  "storymaker.inspector.duration": "Varighed",
  "storymaker.inspector.duration_auto": "Automatisk",
  "storymaker.inspector.duration_fixed": "Fast",
  "storymaker.inspector.duration_seconds": "Varighed i sekunder",
  "storymaker.inspector.seconds_unit": "sekunder",
  "storymaker.inspector.cast_props": "Personer og rekvisitter i denne scene",
  "storymaker.inspector.no_items": "Intet endnu. Vælg personer og rekvisitter i biblioteket.",
  "storymaker.inspector.remove": "Fjern",
  "storymaker.inspector.remove_item": "Fjern {name} fra scenen",
  "storymaker.inspector.move_up": "Flyt scenen op",
  "storymaker.inspector.move_down": "Flyt scenen ned",
  "storymaker.inspector.delete_scene": "Slet scene",
  "storymaker.inspector.delete_scene_text": "Slet scenen {scene}? Dens fortællertekst og alt i den fjernes fra historien.",
  "storymaker.inspector.code_scene_note": "Denne scene tegnes af et lille program.",
  "storymaker.inspector.edit_code": "Rediger kode",

  "storymaker.library.title": "Bibliotek",
  "storymaker.library.people": "Personer",
  "storymaker.library.props": "Rekvisitter",
  "storymaker.library.environments": "Steder",
  "storymaker.library.search": "Søg i biblioteket",
  "storymaker.library.add": "Føj til scenen",
  "storymaker.library.use": "Brug som miljø",
  "storymaker.library.empty": "Intet passer.",
  "storymaker.library.code_scene": "Kodescener tegner alt selv.",
  "storymaker.library.already_in_scene": "Er allerede i denne scene.",
  "storymaker.library.added_actor": "{name} er kommet med i scenen.",
  "storymaker.library.added_prop": "{name} er føjet til scenen.",

  "storymaker.issues.title": "Problemer",
  "storymaker.issues.none": "Ingen problemer.",
  "storymaker.issues.count": "Problemer: {count}",
  "storymaker.issues.show": "Vis problemer",
  "storymaker.issues.show_in_code": "Vis i story.json",

  "storymaker.issue.invalid_json": "Ugyldig JSON",
  "storymaker.issue.unknown_field": "Ukendt felt",
  "storymaker.issue.schema_mismatch": "Forkert historieformat",
  "storymaker.issue.too_large": "For stor",
  "storymaker.issue.limit_exceeded": "Grænsen er overskredet",
  "storymaker.issue.missing_field": "Felt mangler",
  "storymaker.issue.invalid_value": "Ugyldig værdi",
  "storymaker.issue.duplicate_id": "Navnet findes to gange",
  "storymaker.issue.unknown_name": "Ukendt navn",
  "storymaker.issue.unknown_asset": "Ukendt katalogelement",
  "storymaker.issue.unknown_spot": "Ukendt sted",
  "storymaker.issue.unknown_cast": "Ukendt person",
  "storymaker.issue.unknown_cue": "Ukendt mærke",
  "storymaker.issue.canonicalised": "Navnet blev normaliseret",
  "storymaker.issue.not_supported_yet": "Understøttes ikke endnu",
  "storymaker.issue.narration_missing": "Ingen lyd til fortællerteksten",
  "storymaker.issue.code_error": "Fejl i kodescene",
  "storymaker.issue.deadline": "Tog for lang tid",
  "storymaker.issue.layout_unverified": "Layoutet er ikke kontrolleret",
  "storymaker.issue.unknown": "Problem",

  "storymaker.code.title": "Kode",
  "storymaker.code.files": "Filer",
  "storymaker.code.editor_label": "Kodeeditor",
  "storymaker.code.save": "Gem",
  "storymaker.code.saved": "Gemt",
  "storymaker.code.unsaved": "Ikke gemte ændringer",
  "storymaker.code.stale": "Ændret. Gem for at tjekke igen.",
  "storymaker.code.new_file": "Ny fil, ikke gemt endnu",
  "storymaker.code.reload": "Indlæs igen",
  "storymaker.code.bundle_ok": "Kodescenen er gemt.",
  "storymaker.code.bundle_failed": "Koden har fejl.",
  "storymaker.code.no_problems": "Ingen problemer fundet.",
  "storymaker.code.problem_count": "Problemer: {count}",
  "storymaker.code.line_col": "Linje {line}, kolonne {column}",
  "storymaker.code.more": "og {count} mere",

  "storymaker.export.button": "Eksportér",
  "storymaker.export.button_running": "Eksporterer {percent} %",
  "storymaker.export.title": "Eksportér video",
  "storymaker.export.cap_ffmpeg": "FFmpeg",
  "storymaker.export.cap_browser": "Browser",
  "storymaker.export.ready": "Klar",
  "storymaker.export.ready_version": "Klar, version {version}",
  "storymaker.export.ffmpeg_missing": "Ikke fundet. Installer FFmpeg, eller angiv stien i indstillingerne for Video Studio.",
  "storymaker.export.browser_ready": "Klar ({source})",
  "storymaker.export.browser_missing": "Ingen browser er tilgængelig til at tegne billederne.",
  "storymaker.export.source_config": "fra konfigurationen",
  "storymaker.export.source_system": "installeret på dette system",
  "storymaker.export.source_pinned": "hentet af AuraGo",
  "storymaker.export.source_unknown": "ukendt kilde",
  "storymaker.export.still_works": "Redigering og forhåndsvisning virker også uden dem.",
  "storymaker.export.blocked_readonly": "Skrivebordet er skrivebeskyttet.",
  "storymaker.export.blocked_ffmpeg": "FFmpeg mangler.",
  "storymaker.export.blocked_browser": "Ingen browser er tilgængelig.",
  "storymaker.export.blocked_errors": "Historien har fejl. Ret dem først.",
  "storymaker.export.blocked_too_long": "Videoen er længere end grænsen på {max} sekunder.",
  "storymaker.export.blocked_not_ready": "Forhåndsvisningen forberedes stadig.",
  "storymaker.export.summary": "Varighed {duration}, {format}, {width} × {height} pixel.",
  "storymaker.export.silent_note": "Denne version eksporterer videoer uden lyd. Lyd til fortællerteksten kommer senere.",
  "storymaker.export.start": "Start eksport",
  "storymaker.export.starting": "Starter …",
  "storymaker.export.queued": "Venter på tur …",
  "storymaker.export.phase_starting": "Starter",
  "storymaker.export.phase_compiling": "Forbereder historien",
  "storymaker.export.phase_mixing": "Mikser lyden",
  "storymaker.export.phase_rendering": "Tegner billederne",
  "storymaker.export.phase_encoding": "Koder videoen",
  "storymaker.export.phase_writing": "Gemmer filen",
  "storymaker.export.frames": "{done} af {total} billeder",
  "storymaker.export.cancel": "Annuller eksporten",
  "storymaker.export.cancelling": "Annullerer …",
  "storymaker.export.cancelled": "Eksporten blev annulleret.",
  "storymaker.export.interrupted": "Eksporten blev afbrudt af en genstart.",
  "storymaker.export.done": "Eksporten er færdig",
  "storymaker.export.output": "Videoen ligger i din projektmappe:",
  "storymaker.export.show_in_files": "Vis i Filer",
  "storymaker.export.copy_path": "Kopiér sti",
  "storymaker.export.copied": "Kopieret",
  "storymaker.export.copy_failed": "Stien kunne ikke kopieres.",
  "storymaker.export.again": "Eksportér igen",

  "storymaker.job_error.invalid_story": "Historien har fejl og kan derfor ikke eksporteres.",
  "storymaker.job_error.storymaker_disabled": "Storymaker er slået fra.",
  "storymaker.job_error.browser_unavailable": "Der kunne ikke startes nogen browser til at tegne billederne.",
  "storymaker.job_error.ffmpeg_unavailable": "FFmpeg er ikke tilgængelig.",
  "storymaker.job_error.render_failed": "Eksporten mislykkedes.",
  "storymaker.job_error.render_timeout": "Eksporten tog for lang tid og blev stoppet.",
  "storymaker.job_error.cancelled": "Eksporten blev annulleret.",
  "storymaker.job_error.busy": "En anden eksport kører. Prøv igen om lidt.",
  "storymaker.job_error.code_error": "En kodescene mislykkedes under tegningen.",
  "storymaker.job_error.unknown": "Eksporten mislykkedes.",

  "storymaker.error.unauthorized": "Din session er udløbet. Log ind igen.",
  "storymaker.error.scope": "Denne login må ikke gøre det.",
  "storymaker.error.readonly": "Skrivebordet er skrivebeskyttet.",
  "storymaker.error.disabled": "Storymaker er slået fra.",
  "storymaker.error.invalid_story": "Historien blev ikke gemt, fordi den har fejl.",
  "storymaker.error.conflict": "Historien er ændret et andet sted.",
  "storymaker.error.required": "Serveren har brug for den aktuelle version af historien. Indlæs den igen.",
  "storymaker.error.too_large": "Det er for stort.",
  "storymaker.error.not_found": "Det findes ikke længere.",
  "storymaker.error.busy": "Serveren er optaget. Prøv igen om lidt.",
  "storymaker.error.network": "Serveren kunne ikke nås.",
  "storymaker.error.generic": "Noget gik galt."
}
```

`ui/lang/storymaker/de.json`:

```json
{
  "storymaker.title": "Storymaker",
  "storymaker.loading": "Wird geladen …",
  "storymaker.retry": "Erneut versuchen",
  "storymaker.close": "Schließen",
  "storymaker.delete": "Löschen",
  "storymaker.back_to_projects": "Projekte",
  "storymaker.preview": "Storymaker-Vorschau",
  "storymaker.play": "Abspielen",
  "storymaker.pause": "Pause",
  "storymaker.timeline": "Zeitleiste",
  "storymaker.tab_stage": "Storyboard",
  "storymaker.tab_code": "Code",
  "storymaker.side_scene": "Szene",
  "storymaker.side_library": "Bibliothek",
  "storymaker.save_saved": "Gespeichert",
  "storymaker.save_dirty": "Ungespeicherte Änderungen",
  "storymaker.save_saving": "Wird gespeichert …",
  "storymaker.save_error": "Nicht gespeichert",
  "storymaker.save_offline": "Offline, nicht gespeichert",
  "storymaker.save_conflict": "Anderswo geändert",
  "storymaker.save_readonly": "Schreibgeschützt",
  "storymaker.leave_title": "Mit ungespeicherten Änderungen verlassen?",
  "storymaker.leave_text": "Deine letzten Änderungen an der Geschichte konnten nicht gespeichert werden. Wenn du jetzt gehst, gehen sie verloren.",
  "storymaker.leave_code_text": "Eine Code-Datei hat ungespeicherte Änderungen. Wenn du jetzt gehst, gehen sie verloren.",
  "storymaker.aspect_16_9": "Querformat 16:9",
  "storymaker.aspect_9_16": "Hochformat 9:16",
  "storymaker.aspect_1_1": "Quadrat 1:1",

  "storymaker.banner.readonly": "Der Desktop ist schreibgeschützt. Du kannst die Vorschau ansehen, aber Bearbeiten, Speichern und Exportieren sind ausgeschaltet.",
  "storymaker.banner.conflict": "Diese Geschichte wurde woanders geändert, zum Beispiel in einem anderen Fenster. Lade neu, um die aktuelle Fassung zu sehen, oder behalte deine und überschreibe.",
  "storymaker.banner.conflict_reload": "Neu laden",
  "storymaker.banner.conflict_overwrite": "Meine behalten",
  "storymaker.banner.player_unavailable": "Die Vorschau konnte in diesem Browser nicht starten. Deine Geschichte ist trotzdem gespeichert und du kannst sie weiter bearbeiten.",
  "storymaker.banner.export_unavailable": "Der Export ist noch nicht verfügbar ({missing} fehlt). Bearbeiten und Vorschau funktionieren weiterhin.",

  "storymaker.state.disabled_title": "Storymaker ist ausgeschaltet",
  "storymaker.state.disabled_text": "Ein Administrator kann es in der Konfiguration unter Virtueller Desktop einschalten.",
  "storymaker.state.unauthorized_title": "Bitte melde dich erneut an",
  "storymaker.state.unauthorized_text": "Deine Sitzung ist beendet. Melde dich erneut an und öffne Storymaker noch einmal.",
  "storymaker.state.scope_title": "Kein Zugriff",
  "storymaker.state.scope_text": "Diese Anmeldung darf den Virtuellen Desktop nicht benutzen.",
  "storymaker.state.load_failed_title": "Storymaker konnte nicht starten",
  "storymaker.state.load_failed_text": "Der Server hat nicht wie erwartet geantwortet. Prüfe die Verbindung und versuche es erneut.",

  "storymaker.projects.title": "Projekte",
  "storymaker.projects.intro": "Mach aus einem Thema ein kurzes Erklärvideo: Szenen inszenieren, den Sprechtext schreiben und als MP4 exportieren.",
  "storymaker.projects.new_title_label": "Titel",
  "storymaker.projects.new_title_placeholder": "Was ist eine Firewall?",
  "storymaker.projects.new_format": "Format",
  "storymaker.projects.create": "Neues Projekt",
  "storymaker.projects.title_required": "Bitte gib einen Titel ein.",
  "storymaker.projects.empty_title": "Noch keine Projekte",
  "storymaker.projects.empty_text": "Lege oben dein erstes Projekt an. Es beginnt mit einer leeren Szene.",
  "storymaker.projects.delete_label": "Projekt {title} löschen",
  "storymaker.projects.delete_title": "Projekt löschen?",
  "storymaker.projects.delete_text": "Das Projekt „{title}“ und alle seine Dateien, auch die Exporte, werden gelöscht. Das lässt sich nicht rückgängig machen.",
  "storymaker.projects.deleted": "„{title}“ wurde gelöscht.",

  "storymaker.strip.title": "Szenen",
  "storymaker.strip.scene_n": "Szene {n}",
  "storymaker.strip.duration_s": "{seconds} s",
  "storymaker.strip.kind_code": "Code-Szene",
  "storymaker.strip.add_storyboard": "Szene hinzufügen",
  "storymaker.strip.add_code": "Code-Szene hinzufügen",

  "storymaker.stage.compiling": "Die Vorschau wird vorbereitet …",
  "storymaker.stage.unavailable": "Die Vorschau ist nicht verfügbar.",
  "storymaker.stage.player_error": "Die Vorschau hat ein Problem gemeldet.",
  "storymaker.stage.code_error": "Eine Code-Szene hat einen Fehler. Siehe die Liste der Probleme.",

  "storymaker.inspector.title": "Szeneneinstellungen",
  "storymaker.inspector.environment": "Umgebung",
  "storymaker.inspector.variant": "Tageszeit",
  "storymaker.inspector.variant_day": "Tag",
  "storymaker.inspector.variant_night": "Nacht",
  "storymaker.inspector.narration": "Sprechtext",
  "storymaker.inspector.narration_placeholder": "Was sagt die Sprecherin oder der Sprecher in dieser Szene?",
  "storymaker.inspector.narration_hint": "Die Szene dauert so lange, wie der Text zum Lesen braucht, solange du keine Dauer festlegst.",
  "storymaker.inspector.transition": "Übergang in diese Szene",
  "storymaker.inspector.transition_first": "Die erste Szene hat keinen Übergang.",
  "storymaker.inspector.unsupported": "{name} (noch nicht unterstützt)",
  "storymaker.inspector.duration": "Dauer",
  "storymaker.inspector.duration_auto": "Automatisch",
  "storymaker.inspector.duration_fixed": "Fest",
  "storymaker.inspector.duration_seconds": "Dauer in Sekunden",
  "storymaker.inspector.seconds_unit": "Sekunden",
  "storymaker.inspector.cast_props": "Personen und Requisiten in dieser Szene",
  "storymaker.inspector.no_items": "Noch nichts. Wähle Personen und Requisiten in der Bibliothek.",
  "storymaker.inspector.remove": "Entfernen",
  "storymaker.inspector.remove_item": "{name} aus der Szene entfernen",
  "storymaker.inspector.move_up": "Szene nach oben verschieben",
  "storymaker.inspector.move_down": "Szene nach unten verschieben",
  "storymaker.inspector.delete_scene": "Szene löschen",
  "storymaker.inspector.delete_scene_text": "Szene {scene} löschen? Ihr Sprechtext und alles darin werden aus der Geschichte entfernt.",
  "storymaker.inspector.code_scene_note": "Diese Szene wird von einem kleinen Programm gezeichnet.",
  "storymaker.inspector.edit_code": "Code bearbeiten",

  "storymaker.library.title": "Bibliothek",
  "storymaker.library.people": "Personen",
  "storymaker.library.props": "Requisiten",
  "storymaker.library.environments": "Orte",
  "storymaker.library.search": "Bibliothek durchsuchen",
  "storymaker.library.add": "Zur Szene hinzufügen",
  "storymaker.library.use": "Als Umgebung verwenden",
  "storymaker.library.empty": "Nichts passt.",
  "storymaker.library.code_scene": "Code-Szenen zeichnen alles selbst.",
  "storymaker.library.already_in_scene": "Schon in dieser Szene.",
  "storymaker.library.added_actor": "{name} ist der Szene beigetreten.",
  "storymaker.library.added_prop": "{name} wurde der Szene hinzugefügt.",

  "storymaker.issues.title": "Probleme",
  "storymaker.issues.none": "Keine Probleme.",
  "storymaker.issues.count": "Probleme: {count}",
  "storymaker.issues.show": "Probleme anzeigen",
  "storymaker.issues.show_in_code": "In story.json anzeigen",

  "storymaker.issue.invalid_json": "Kein gültiges JSON",
  "storymaker.issue.unknown_field": "Unbekanntes Feld",
  "storymaker.issue.schema_mismatch": "Falsches Format der Geschichte",
  "storymaker.issue.too_large": "Zu groß",
  "storymaker.issue.limit_exceeded": "Grenze überschritten",
  "storymaker.issue.missing_field": "Fehlendes Feld",
  "storymaker.issue.invalid_value": "Ungültiger Wert",
  "storymaker.issue.duplicate_id": "Doppelter Name",
  "storymaker.issue.unknown_name": "Unbekannter Name",
  "storymaker.issue.unknown_asset": "Unbekannter Katalogeintrag",
  "storymaker.issue.unknown_spot": "Unbekannter Platz",
  "storymaker.issue.unknown_cast": "Unbekannte Person",
  "storymaker.issue.unknown_cue": "Unbekannte Markierung",
  "storymaker.issue.canonicalised": "Name vereinheitlicht",
  "storymaker.issue.not_supported_yet": "Noch nicht unterstützt",
  "storymaker.issue.narration_missing": "Keine Sprecher-Audiodatei",
  "storymaker.issue.code_error": "Fehler in der Code-Szene",
  "storymaker.issue.deadline": "Hat zu lange gedauert",
  "storymaker.issue.layout_unverified": "Layout nicht geprüft",
  "storymaker.issue.unknown": "Problem",

  "storymaker.code.title": "Code",
  "storymaker.code.files": "Dateien",
  "storymaker.code.editor_label": "Code-Editor",
  "storymaker.code.save": "Speichern",
  "storymaker.code.saved": "Gespeichert",
  "storymaker.code.unsaved": "Ungespeicherte Änderungen",
  "storymaker.code.stale": "Geändert. Speichere, um es erneut zu prüfen.",
  "storymaker.code.new_file": "Neue Datei, noch nicht gespeichert",
  "storymaker.code.reload": "Neu laden",
  "storymaker.code.bundle_ok": "Code-Szene gespeichert.",
  "storymaker.code.bundle_failed": "Der Code hat Fehler.",
  "storymaker.code.no_problems": "Keine Probleme gefunden.",
  "storymaker.code.problem_count": "Probleme: {count}",
  "storymaker.code.line_col": "Zeile {line}, Spalte {column}",
  "storymaker.code.more": "und {count} weitere",

  "storymaker.export.button": "Exportieren",
  "storymaker.export.button_running": "Export {percent} %",
  "storymaker.export.title": "Video exportieren",
  "storymaker.export.cap_ffmpeg": "FFmpeg",
  "storymaker.export.cap_browser": "Browser",
  "storymaker.export.ready": "Bereit",
  "storymaker.export.ready_version": "Bereit, Version {version}",
  "storymaker.export.ffmpeg_missing": "Nicht gefunden. Installiere FFmpeg oder trage den Pfad in den Video-Studio-Einstellungen ein.",
  "storymaker.export.browser_ready": "Bereit ({source})",
  "storymaker.export.browser_missing": "Es ist kein Browser zum Zeichnen der Bilder verfügbar.",
  "storymaker.export.source_config": "aus der Konfiguration",
  "storymaker.export.source_system": "auf diesem System installiert",
  "storymaker.export.source_pinned": "von AuraGo heruntergeladen",
  "storymaker.export.source_unknown": "Quelle unbekannt",
  "storymaker.export.still_works": "Bearbeiten und Vorschau funktionieren auch ohne sie.",
  "storymaker.export.blocked_readonly": "Der Desktop ist schreibgeschützt.",
  "storymaker.export.blocked_ffmpeg": "FFmpeg fehlt.",
  "storymaker.export.blocked_browser": "Es ist kein Browser verfügbar.",
  "storymaker.export.blocked_errors": "Die Geschichte hat Fehler. Behebe sie zuerst.",
  "storymaker.export.blocked_too_long": "Das Video ist länger als die Grenze von {max} Sekunden.",
  "storymaker.export.blocked_not_ready": "Die Vorschau wird noch vorbereitet.",
  "storymaker.export.summary": "{duration} lang, {format}, {width} × {height} Pixel.",
  "storymaker.export.silent_note": "Diese Version exportiert stumme Videos. Sprecher-Audio kommt später.",
  "storymaker.export.start": "Export starten",
  "storymaker.export.starting": "Wird gestartet …",
  "storymaker.export.queued": "Wartet auf seinen Platz in der Schlange …",
  "storymaker.export.phase_starting": "Start",
  "storymaker.export.phase_compiling": "Geschichte wird vorbereitet",
  "storymaker.export.phase_mixing": "Ton wird gemischt",
  "storymaker.export.phase_rendering": "Bilder werden gezeichnet",
  "storymaker.export.phase_encoding": "Video wird kodiert",
  "storymaker.export.phase_writing": "Datei wird gespeichert",
  "storymaker.export.frames": "{done} von {total} Bildern",
  "storymaker.export.cancel": "Export abbrechen",
  "storymaker.export.cancelling": "Wird abgebrochen …",
  "storymaker.export.cancelled": "Der Export wurde abgebrochen.",
  "storymaker.export.interrupted": "Der Export wurde durch einen Neustart unterbrochen.",
  "storymaker.export.done": "Export abgeschlossen",
  "storymaker.export.output": "Das Video liegt in deinem Projektordner:",
  "storymaker.export.show_in_files": "In Dateien zeigen",
  "storymaker.export.copy_path": "Pfad kopieren",
  "storymaker.export.copied": "Kopiert",
  "storymaker.export.copy_failed": "Der Pfad konnte nicht kopiert werden.",
  "storymaker.export.again": "Erneut exportieren",

  "storymaker.job_error.invalid_story": "Die Geschichte hat Fehler und lässt sich deshalb nicht exportieren.",
  "storymaker.job_error.storymaker_disabled": "Storymaker ist ausgeschaltet.",
  "storymaker.job_error.browser_unavailable": "Es konnte kein Browser zum Zeichnen der Bilder gestartet werden.",
  "storymaker.job_error.ffmpeg_unavailable": "FFmpeg ist nicht verfügbar.",
  "storymaker.job_error.render_failed": "Der Export ist fehlgeschlagen.",
  "storymaker.job_error.render_timeout": "Der Export hat zu lange gedauert und wurde gestoppt.",
  "storymaker.job_error.cancelled": "Der Export wurde abgebrochen.",
  "storymaker.job_error.busy": "Ein anderer Export läuft. Versuche es gleich noch einmal.",
  "storymaker.job_error.code_error": "Eine Code-Szene ist beim Zeichnen fehlgeschlagen.",
  "storymaker.job_error.unknown": "Der Export ist fehlgeschlagen.",

  "storymaker.error.unauthorized": "Deine Sitzung ist beendet. Melde dich erneut an.",
  "storymaker.error.scope": "Diese Anmeldung darf das nicht.",
  "storymaker.error.readonly": "Der Desktop ist schreibgeschützt.",
  "storymaker.error.disabled": "Storymaker ist ausgeschaltet.",
  "storymaker.error.invalid_story": "Die Geschichte wurde nicht gespeichert, weil sie Fehler hat.",
  "storymaker.error.conflict": "Die Geschichte wurde woanders geändert.",
  "storymaker.error.required": "Der Server braucht die aktuelle Fassung der Geschichte. Lade sie neu.",
  "storymaker.error.too_large": "Das ist zu groß.",
  "storymaker.error.not_found": "Das gibt es nicht mehr.",
  "storymaker.error.busy": "Der Server ist beschäftigt. Versuche es gleich noch einmal.",
  "storymaker.error.network": "Der Server ist nicht erreichbar.",
  "storymaker.error.generic": "Etwas ist schiefgelaufen."
}
```

`ui/lang/storymaker/el.json`:

```json
{
  "storymaker.title": "Storymaker",
  "storymaker.loading": "Φόρτωση…",
  "storymaker.retry": "Δοκιμάστε ξανά",
  "storymaker.close": "Κλείσιμο",
  "storymaker.delete": "Διαγραφή",
  "storymaker.back_to_projects": "Έργα",
  "storymaker.preview": "Προεπισκόπηση του Storymaker",
  "storymaker.play": "Αναπαραγωγή",
  "storymaker.pause": "Παύση",
  "storymaker.timeline": "Χρονοδιάγραμμα",
  "storymaker.tab_stage": "Storyboard",
  "storymaker.tab_code": "Κώδικας",
  "storymaker.side_scene": "Σκηνή",
  "storymaker.side_library": "Βιβλιοθήκη",
  "storymaker.save_saved": "Αποθηκεύτηκε",
  "storymaker.save_dirty": "Μη αποθηκευμένες αλλαγές",
  "storymaker.save_saving": "Αποθήκευση…",
  "storymaker.save_error": "Δεν αποθηκεύτηκε",
  "storymaker.save_offline": "Χωρίς σύνδεση, δεν αποθηκεύτηκε",
  "storymaker.save_conflict": "Άλλαξε αλλού",
  "storymaker.save_readonly": "Μόνο για ανάγνωση",
  "storymaker.leave_title": "Έξοδος με μη αποθηκευμένες αλλαγές;",
  "storymaker.leave_text": "Οι τελευταίες σας αλλαγές στην ιστορία δεν μπόρεσαν να αποθηκευτούν. Αν φύγετε τώρα, θα χαθούν.",
  "storymaker.leave_code_text": "Ένα αρχείο κώδικα έχει μη αποθηκευμένες αλλαγές. Αν φύγετε τώρα, θα χαθούν.",
  "storymaker.aspect_16_9": "Οριζόντιο 16:9",
  "storymaker.aspect_9_16": "Κατακόρυφο 9:16",
  "storymaker.aspect_1_1": "Τετράγωνο 1:1",

  "storymaker.banner.readonly": "Η επιφάνεια εργασίας είναι μόνο για ανάγνωση. Μπορείτε να δείτε την προεπισκόπηση, αλλά η επεξεργασία, η αποθήκευση και η εξαγωγή είναι απενεργοποιημένες.",
  "storymaker.banner.conflict": "Αυτή η ιστορία άλλαξε αλλού, για παράδειγμα σε άλλο παράθυρο. Φορτώστε ξανά για να δείτε την τρέχουσα έκδοση ή κρατήστε τη δική σας και αντικαταστήστε.",
  "storymaker.banner.conflict_reload": "Φόρτωση ξανά",
  "storymaker.banner.conflict_overwrite": "Κράτηση της δικής μου",
  "storymaker.banner.player_unavailable": "Η προεπισκόπηση δεν μπόρεσε να ξεκινήσει σε αυτό το πρόγραμμα περιήγησης. Η ιστορία σας είναι ακόμα αποθηκευμένη και μπορείτε να συνεχίσετε την επεξεργασία.",
  "storymaker.banner.export_unavailable": "Η εξαγωγή δεν είναι ακόμα διαθέσιμη (λείπει: {missing}). Η επεξεργασία και η προεπισκόπηση λειτουργούν κανονικά.",

  "storymaker.state.disabled_title": "Το Storymaker είναι απενεργοποιημένο",
  "storymaker.state.disabled_text": "Ένας διαχειριστής μπορεί να το ενεργοποιήσει στις ρυθμίσεις, στην ενότητα Εικονική επιφάνεια εργασίας.",
  "storymaker.state.unauthorized_title": "Συνδεθείτε ξανά",
  "storymaker.state.unauthorized_text": "Η συνεδρία σας έληξε. Συνδεθείτε ξανά και ανοίξτε πάλι το Storymaker.",
  "storymaker.state.scope_title": "Χωρίς πρόσβαση",
  "storymaker.state.scope_text": "Αυτή η σύνδεση δεν επιτρέπεται να χρησιμοποιεί την Εικονική επιφάνεια εργασίας.",
  "storymaker.state.load_failed_title": "Το Storymaker δεν μπόρεσε να ξεκινήσει",
  "storymaker.state.load_failed_text": "Ο διακομιστής δεν απάντησε όπως αναμενόταν. Ελέγξτε τη σύνδεση και δοκιμάστε ξανά.",

  "storymaker.projects.title": "Έργα",
  "storymaker.projects.intro": "Μετατρέψτε ένα θέμα σε σύντομο επεξηγηματικό βίντεο: σκηνοθετήστε τις σκηνές, γράψτε την αφήγηση και εξαγάγετε ένα MP4.",
  "storymaker.projects.new_title_label": "Τίτλος",
  "storymaker.projects.new_title_placeholder": "Τι είναι ένα τείχος προστασίας;",
  "storymaker.projects.new_format": "Μορφή",
  "storymaker.projects.create": "Νέο έργο",
  "storymaker.projects.title_required": "Εισαγάγετε έναν τίτλο.",
  "storymaker.projects.empty_title": "Δεν υπάρχουν έργα ακόμα",
  "storymaker.projects.empty_text": "Δημιουργήστε το πρώτο σας έργο παραπάνω. Ξεκινά με μία κενή σκηνή.",
  "storymaker.projects.delete_label": "Διαγραφή του έργου {title}",
  "storymaker.projects.delete_title": "Διαγραφή έργου;",
  "storymaker.projects.delete_text": "Το έργο «{title}» και όλα τα αρχεία του, μαζί με τις εξαγωγές, θα διαγραφούν. Αυτό δεν μπορεί να αναιρεθεί.",
  "storymaker.projects.deleted": "Το «{title}» διαγράφηκε.",

  "storymaker.strip.title": "Σκηνές",
  "storymaker.strip.scene_n": "Σκηνή {n}",
  "storymaker.strip.duration_s": "{seconds} δ",
  "storymaker.strip.kind_code": "Σκηνή κώδικα",
  "storymaker.strip.add_storyboard": "Προσθήκη σκηνής",
  "storymaker.strip.add_code": "Προσθήκη σκηνής κώδικα",

  "storymaker.stage.compiling": "Προετοιμασία της προεπισκόπησης…",
  "storymaker.stage.unavailable": "Η προεπισκόπηση δεν είναι διαθέσιμη.",
  "storymaker.stage.player_error": "Η προεπισκόπηση ανέφερε πρόβλημα.",
  "storymaker.stage.code_error": "Μια σκηνή κώδικα έχει σφάλμα. Δείτε τη λίστα προβλημάτων.",

  "storymaker.inspector.title": "Ρυθμίσεις σκηνής",
  "storymaker.inspector.environment": "Περιβάλλον",
  "storymaker.inspector.variant": "Ώρα της ημέρας",
  "storymaker.inspector.variant_day": "Ημέρα",
  "storymaker.inspector.variant_night": "Νύχτα",
  "storymaker.inspector.narration": "Αφήγηση",
  "storymaker.inspector.narration_placeholder": "Τι λέει ο αφηγητής σε αυτή τη σκηνή;",
  "storymaker.inspector.narration_hint": "Η σκηνή διαρκεί όσο χρειάζεται για να διαβαστεί το κείμενο, εκτός αν ορίσετε διάρκεια.",
  "storymaker.inspector.transition": "Μετάβαση σε αυτή τη σκηνή",
  "storymaker.inspector.transition_first": "Η πρώτη σκηνή δεν έχει μετάβαση.",
  "storymaker.inspector.unsupported": "{name} (δεν υποστηρίζεται ακόμα)",
  "storymaker.inspector.duration": "Διάρκεια",
  "storymaker.inspector.duration_auto": "Αυτόματη",
  "storymaker.inspector.duration_fixed": "Σταθερή",
  "storymaker.inspector.duration_seconds": "Διάρκεια σε δευτερόλεπτα",
  "storymaker.inspector.seconds_unit": "δευτερόλεπτα",
  "storymaker.inspector.cast_props": "Πρόσωπα και αντικείμενα σε αυτή τη σκηνή",
  "storymaker.inspector.no_items": "Τίποτα ακόμα. Επιλέξτε πρόσωπα και αντικείμενα από τη βιβλιοθήκη.",
  "storymaker.inspector.remove": "Αφαίρεση",
  "storymaker.inspector.remove_item": "Αφαίρεση του «{name}» από τη σκηνή",
  "storymaker.inspector.move_up": "Μετακίνηση της σκηνής προς τα πάνω",
  "storymaker.inspector.move_down": "Μετακίνηση της σκηνής προς τα κάτω",
  "storymaker.inspector.delete_scene": "Διαγραφή σκηνής",
  "storymaker.inspector.delete_scene_text": "Διαγραφή της σκηνής {scene}; Η αφήγησή της και όλο το περιεχόμενό της θα αφαιρεθούν από την ιστορία.",
  "storymaker.inspector.code_scene_note": "Αυτή η σκηνή σχεδιάζεται από ένα μικρό πρόγραμμα.",
  "storymaker.inspector.edit_code": "Επεξεργασία κώδικα",

  "storymaker.library.title": "Βιβλιοθήκη",
  "storymaker.library.people": "Πρόσωπα",
  "storymaker.library.props": "Αντικείμενα",
  "storymaker.library.environments": "Τοποθεσίες",
  "storymaker.library.search": "Αναζήτηση στη βιβλιοθήκη",
  "storymaker.library.add": "Προσθήκη στη σκηνή",
  "storymaker.library.use": "Χρήση ως περιβάλλον",
  "storymaker.library.empty": "Δεν βρέθηκε τίποτα.",
  "storymaker.library.code_scene": "Οι σκηνές κώδικα σχεδιάζουν τα πάντα μόνες τους.",
  "storymaker.library.already_in_scene": "Υπάρχει ήδη σε αυτή τη σκηνή.",
  "storymaker.library.added_actor": "Προστέθηκε στη σκηνή: {name}.",
  "storymaker.library.added_prop": "Προστέθηκε στη σκηνή: {name}.",

  "storymaker.issues.title": "Προβλήματα",
  "storymaker.issues.none": "Κανένα πρόβλημα.",
  "storymaker.issues.count": "Προβλήματα: {count}",
  "storymaker.issues.show": "Εμφάνιση προβλημάτων",
  "storymaker.issues.show_in_code": "Εμφάνιση στο story.json",

  "storymaker.issue.invalid_json": "Μη έγκυρο JSON",
  "storymaker.issue.unknown_field": "Άγνωστο πεδίο",
  "storymaker.issue.schema_mismatch": "Λάθος μορφή ιστορίας",
  "storymaker.issue.too_large": "Πολύ μεγάλο",
  "storymaker.issue.limit_exceeded": "Υπέρβαση ορίου",
  "storymaker.issue.missing_field": "Λείπει πεδίο",
  "storymaker.issue.invalid_value": "Μη έγκυρη τιμή",
  "storymaker.issue.duplicate_id": "Διπλό όνομα",
  "storymaker.issue.unknown_name": "Άγνωστο όνομα",
  "storymaker.issue.unknown_asset": "Άγνωστο στοιχείο καταλόγου",
  "storymaker.issue.unknown_spot": "Άγνωστη θέση",
  "storymaker.issue.unknown_cast": "Άγνωστο πρόσωπο",
  "storymaker.issue.unknown_cue": "Άγνωστος δείκτης",
  "storymaker.issue.canonicalised": "Το όνομα κανονικοποιήθηκε",
  "storymaker.issue.not_supported_yet": "Δεν υποστηρίζεται ακόμα",
  "storymaker.issue.narration_missing": "Χωρίς ήχο αφήγησης",
  "storymaker.issue.code_error": "Σφάλμα στη σκηνή κώδικα",
  "storymaker.issue.deadline": "Πήρε υπερβολικά πολύ χρόνο",
  "storymaker.issue.layout_unverified": "Η διάταξη δεν ελέγχθηκε",
  "storymaker.issue.unknown": "Πρόβλημα",

  "storymaker.code.title": "Κώδικας",
  "storymaker.code.files": "Αρχεία",
  "storymaker.code.editor_label": "Επεξεργαστής κώδικα",
  "storymaker.code.save": "Αποθήκευση",
  "storymaker.code.saved": "Αποθηκεύτηκε",
  "storymaker.code.unsaved": "Μη αποθηκευμένες αλλαγές",
  "storymaker.code.stale": "Άλλαξε. Αποθηκεύστε για να ελεγχθεί ξανά.",
  "storymaker.code.new_file": "Νέο αρχείο, δεν έχει αποθηκευτεί ακόμα",
  "storymaker.code.reload": "Φόρτωση ξανά",
  "storymaker.code.bundle_ok": "Η σκηνή κώδικα αποθηκεύτηκε.",
  "storymaker.code.bundle_failed": "Ο κώδικας έχει σφάλματα.",
  "storymaker.code.no_problems": "Δεν βρέθηκαν προβλήματα.",
  "storymaker.code.problem_count": "Προβλήματα: {count}",
  "storymaker.code.line_col": "Γραμμή {line}, στήλη {column}",
  "storymaker.code.more": "και {count} ακόμα",

  "storymaker.export.button": "Εξαγωγή",
  "storymaker.export.button_running": "Εξαγωγή {percent} %",
  "storymaker.export.title": "Εξαγωγή βίντεο",
  "storymaker.export.cap_ffmpeg": "FFmpeg",
  "storymaker.export.cap_browser": "Πρόγραμμα περιήγησης",
  "storymaker.export.ready": "Έτοιμο",
  "storymaker.export.ready_version": "Έτοιμο, έκδοση {version}",
  "storymaker.export.ffmpeg_missing": "Δεν βρέθηκε. Εγκαταστήστε το FFmpeg ή ορίστε τη διαδρομή του στις ρυθμίσεις του Video Studio.",
  "storymaker.export.browser_ready": "Έτοιμο ({source})",
  "storymaker.export.browser_missing": "Δεν υπάρχει διαθέσιμο πρόγραμμα περιήγησης για τη σχεδίαση των καρέ.",
  "storymaker.export.source_config": "από τις ρυθμίσεις",
  "storymaker.export.source_system": "εγκατεστημένο σε αυτό το σύστημα",
  "storymaker.export.source_pinned": "λήφθηκε από το AuraGo",
  "storymaker.export.source_unknown": "άγνωστη πηγή",
  "storymaker.export.still_works": "Η επεξεργασία και η προεπισκόπηση λειτουργούν και χωρίς αυτά.",
  "storymaker.export.blocked_readonly": "Η επιφάνεια εργασίας είναι μόνο για ανάγνωση.",
  "storymaker.export.blocked_ffmpeg": "Λείπει το FFmpeg.",
  "storymaker.export.blocked_browser": "Δεν υπάρχει διαθέσιμο πρόγραμμα περιήγησης.",
  "storymaker.export.blocked_errors": "Η ιστορία έχει σφάλματα. Διορθώστε τα πρώτα.",
  "storymaker.export.blocked_too_long": "Το βίντεο είναι μεγαλύτερο από το όριο των {max} δευτερολέπτων.",
  "storymaker.export.blocked_not_ready": "Η προεπισκόπηση ετοιμάζεται ακόμα.",
  "storymaker.export.summary": "Διάρκεια {duration}, {format}, {width} × {height} pixel.",
  "storymaker.export.silent_note": "Αυτή η έκδοση εξάγει βίντεο χωρίς ήχο. Ο ήχος της αφήγησης θα έρθει αργότερα.",
  "storymaker.export.start": "Έναρξη εξαγωγής",
  "storymaker.export.starting": "Εκκίνηση…",
  "storymaker.export.queued": "Αναμονή για τη σειρά του…",
  "storymaker.export.phase_starting": "Εκκίνηση",
  "storymaker.export.phase_compiling": "Προετοιμασία της ιστορίας",
  "storymaker.export.phase_mixing": "Μίξη του ήχου",
  "storymaker.export.phase_rendering": "Σχεδίαση των καρέ",
  "storymaker.export.phase_encoding": "Κωδικοποίηση του βίντεο",
  "storymaker.export.phase_writing": "Αποθήκευση του αρχείου",
  "storymaker.export.frames": "{done} από {total} καρέ",
  "storymaker.export.cancel": "Ακύρωση εξαγωγής",
  "storymaker.export.cancelling": "Ακύρωση…",
  "storymaker.export.cancelled": "Η εξαγωγή ακυρώθηκε.",
  "storymaker.export.interrupted": "Η εξαγωγή διακόπηκε από επανεκκίνηση.",
  "storymaker.export.done": "Η εξαγωγή ολοκληρώθηκε",
  "storymaker.export.output": "Το βίντεο βρίσκεται στον φάκελο του έργου σας:",
  "storymaker.export.show_in_files": "Εμφάνιση στα Αρχεία",
  "storymaker.export.copy_path": "Αντιγραφή διαδρομής",
  "storymaker.export.copied": "Αντιγράφηκε",
  "storymaker.export.copy_failed": "Η διαδρομή δεν μπόρεσε να αντιγραφεί.",
  "storymaker.export.again": "Νέα εξαγωγή",

  "storymaker.job_error.invalid_story": "Η ιστορία έχει σφάλματα, γι' αυτό δεν μπορεί να εξαχθεί.",
  "storymaker.job_error.storymaker_disabled": "Το Storymaker είναι απενεργοποιημένο.",
  "storymaker.job_error.browser_unavailable": "Δεν μπόρεσε να ξεκινήσει πρόγραμμα περιήγησης για τη σχεδίαση των καρέ.",
  "storymaker.job_error.ffmpeg_unavailable": "Το FFmpeg δεν είναι διαθέσιμο.",
  "storymaker.job_error.render_failed": "Η εξαγωγή απέτυχε.",
  "storymaker.job_error.render_timeout": "Η εξαγωγή κράτησε πολύ και σταμάτησε.",
  "storymaker.job_error.cancelled": "Η εξαγωγή ακυρώθηκε.",
  "storymaker.job_error.busy": "Εκτελείται άλλη εξαγωγή. Δοκιμάστε ξανά σε λίγο.",
  "storymaker.job_error.code_error": "Μια σκηνή κώδικα απέτυχε κατά τη σχεδίαση.",
  "storymaker.job_error.unknown": "Η εξαγωγή απέτυχε.",

  "storymaker.error.unauthorized": "Η συνεδρία σας έληξε. Συνδεθείτε ξανά.",
  "storymaker.error.scope": "Αυτή η σύνδεση δεν επιτρέπεται να το κάνει.",
  "storymaker.error.readonly": "Η επιφάνεια εργασίας είναι μόνο για ανάγνωση.",
  "storymaker.error.disabled": "Το Storymaker είναι απενεργοποιημένο.",
  "storymaker.error.invalid_story": "Η ιστορία δεν αποθηκεύτηκε επειδή έχει σφάλματα.",
  "storymaker.error.conflict": "Η ιστορία άλλαξε αλλού.",
  "storymaker.error.required": "Ο διακομιστής χρειάζεται την τρέχουσα έκδοση της ιστορίας. Φορτώστε την ξανά.",
  "storymaker.error.too_large": "Είναι πολύ μεγάλο.",
  "storymaker.error.not_found": "Δεν υπάρχει πια.",
  "storymaker.error.busy": "Ο διακομιστής είναι απασχολημένος. Δοκιμάστε ξανά σε λίγο.",
  "storymaker.error.network": "Δεν ήταν δυνατή η σύνδεση με τον διακομιστή.",
  "storymaker.error.generic": "Κάτι πήγε στραβά."
}
```

`ui/lang/storymaker/en.json`:

```json
{
  "storymaker.title": "Storymaker",
  "storymaker.loading": "Loading…",
  "storymaker.retry": "Try again",
  "storymaker.close": "Close",
  "storymaker.delete": "Delete",
  "storymaker.back_to_projects": "Projects",
  "storymaker.preview": "Storymaker preview",
  "storymaker.play": "Play",
  "storymaker.pause": "Pause",
  "storymaker.timeline": "Timeline",
  "storymaker.tab_stage": "Storyboard",
  "storymaker.tab_code": "Code",
  "storymaker.side_scene": "Scene",
  "storymaker.side_library": "Library",
  "storymaker.save_saved": "Saved",
  "storymaker.save_dirty": "Unsaved changes",
  "storymaker.save_saving": "Saving…",
  "storymaker.save_error": "Not saved",
  "storymaker.save_offline": "Offline, not saved",
  "storymaker.save_conflict": "Changed elsewhere",
  "storymaker.save_readonly": "Read-only",
  "storymaker.leave_title": "Leave with unsaved changes?",
  "storymaker.leave_text": "Your latest changes to the story could not be saved. If you leave now they are lost.",
  "storymaker.leave_code_text": "A code file has unsaved changes. If you leave now they are lost.",
  "storymaker.aspect_16_9": "Landscape 16:9",
  "storymaker.aspect_9_16": "Portrait 9:16",
  "storymaker.aspect_1_1": "Square 1:1",

  "storymaker.banner.readonly": "The desktop is read-only. You can watch the preview, but editing, saving and exporting are switched off.",
  "storymaker.banner.conflict": "This story was changed somewhere else, for example in another window. Reload to see the current version, or keep yours and overwrite.",
  "storymaker.banner.conflict_reload": "Reload",
  "storymaker.banner.conflict_overwrite": "Keep mine",
  "storymaker.banner.player_unavailable": "The preview could not start in this browser. Your story is still saved and you can keep editing it.",
  "storymaker.banner.export_unavailable": "Export is not available yet ({missing} missing). Editing and the preview still work.",

  "storymaker.state.disabled_title": "Storymaker is switched off",
  "storymaker.state.disabled_text": "An administrator can switch it on in the configuration under Virtual Desktop.",
  "storymaker.state.unauthorized_title": "Please sign in again",
  "storymaker.state.unauthorized_text": "Your session has ended. Sign in again and reopen Storymaker.",
  "storymaker.state.scope_title": "No access",
  "storymaker.state.scope_text": "This sign-in is not allowed to use the Virtual Desktop.",
  "storymaker.state.load_failed_title": "Storymaker could not start",
  "storymaker.state.load_failed_text": "The server did not answer as expected. Check the connection and try again.",

  "storymaker.projects.title": "Projects",
  "storymaker.projects.intro": "Turn a topic into a short explainer video: stage the scenes, write the narration and export an MP4.",
  "storymaker.projects.new_title_label": "Title",
  "storymaker.projects.new_title_placeholder": "What is a firewall?",
  "storymaker.projects.new_format": "Format",
  "storymaker.projects.create": "New project",
  "storymaker.projects.title_required": "Please enter a title.",
  "storymaker.projects.empty_title": "No projects yet",
  "storymaker.projects.empty_text": "Create your first project above. It starts with one empty scene.",
  "storymaker.projects.delete_label": "Delete project {title}",
  "storymaker.projects.delete_title": "Delete project?",
  "storymaker.projects.delete_text": "The project \"{title}\" and all of its files, including the exports, will be deleted. This cannot be undone.",
  "storymaker.projects.deleted": "Deleted \"{title}\".",

  "storymaker.strip.title": "Scenes",
  "storymaker.strip.scene_n": "Scene {n}",
  "storymaker.strip.duration_s": "{seconds} s",
  "storymaker.strip.kind_code": "Code scene",
  "storymaker.strip.add_storyboard": "Add scene",
  "storymaker.strip.add_code": "Add code scene",

  "storymaker.stage.compiling": "Preparing the preview…",
  "storymaker.stage.unavailable": "The preview is not available.",
  "storymaker.stage.player_error": "The preview reported a problem.",
  "storymaker.stage.code_error": "A code scene has an error. See the issues list.",

  "storymaker.inspector.title": "Scene settings",
  "storymaker.inspector.environment": "Environment",
  "storymaker.inspector.variant": "Time of day",
  "storymaker.inspector.variant_day": "Day",
  "storymaker.inspector.variant_night": "Night",
  "storymaker.inspector.narration": "Narration",
  "storymaker.inspector.narration_placeholder": "What does the narrator say in this scene?",
  "storymaker.inspector.narration_hint": "The scene lasts as long as the text takes to read, unless you set a duration.",
  "storymaker.inspector.transition": "Transition into this scene",
  "storymaker.inspector.transition_first": "The first scene has no transition.",
  "storymaker.inspector.unsupported": "{name} (not supported yet)",
  "storymaker.inspector.duration": "Duration",
  "storymaker.inspector.duration_auto": "Automatic",
  "storymaker.inspector.duration_fixed": "Fixed",
  "storymaker.inspector.duration_seconds": "Duration in seconds",
  "storymaker.inspector.seconds_unit": "seconds",
  "storymaker.inspector.cast_props": "People and props in this scene",
  "storymaker.inspector.no_items": "Nothing yet. Pick people and props in the library.",
  "storymaker.inspector.remove": "Remove",
  "storymaker.inspector.remove_item": "Remove {name} from the scene",
  "storymaker.inspector.move_up": "Move scene up",
  "storymaker.inspector.move_down": "Move scene down",
  "storymaker.inspector.delete_scene": "Delete scene",
  "storymaker.inspector.delete_scene_text": "Delete the scene {scene}? Its narration and everything in it will be removed from the story.",
  "storymaker.inspector.code_scene_note": "This scene is drawn by a small program.",
  "storymaker.inspector.edit_code": "Edit code",

  "storymaker.library.title": "Library",
  "storymaker.library.people": "People",
  "storymaker.library.props": "Props",
  "storymaker.library.environments": "Places",
  "storymaker.library.search": "Search the library",
  "storymaker.library.add": "Add to scene",
  "storymaker.library.use": "Use as environment",
  "storymaker.library.empty": "Nothing matches.",
  "storymaker.library.code_scene": "Code scenes draw everything themselves.",
  "storymaker.library.already_in_scene": "Already in this scene.",
  "storymaker.library.added_actor": "{name} joined the scene.",
  "storymaker.library.added_prop": "{name} added to the scene.",

  "storymaker.issues.title": "Issues",
  "storymaker.issues.none": "No issues.",
  "storymaker.issues.count": "Issues: {count}",
  "storymaker.issues.show": "Show issues",
  "storymaker.issues.show_in_code": "Show in story.json",

  "storymaker.issue.invalid_json": "Not valid JSON",
  "storymaker.issue.unknown_field": "Unknown field",
  "storymaker.issue.schema_mismatch": "Wrong story format",
  "storymaker.issue.too_large": "Too large",
  "storymaker.issue.limit_exceeded": "Limit exceeded",
  "storymaker.issue.missing_field": "Missing field",
  "storymaker.issue.invalid_value": "Invalid value",
  "storymaker.issue.duplicate_id": "Duplicate name",
  "storymaker.issue.unknown_name": "Unknown name",
  "storymaker.issue.unknown_asset": "Unknown catalog item",
  "storymaker.issue.unknown_spot": "Unknown spot",
  "storymaker.issue.unknown_cast": "Unknown person",
  "storymaker.issue.unknown_cue": "Unknown cue",
  "storymaker.issue.canonicalised": "Name normalised",
  "storymaker.issue.not_supported_yet": "Not supported yet",
  "storymaker.issue.narration_missing": "No narration audio",
  "storymaker.issue.code_error": "Code scene error",
  "storymaker.issue.deadline": "Took too long",
  "storymaker.issue.layout_unverified": "Layout not checked",
  "storymaker.issue.unknown": "Issue",

  "storymaker.code.title": "Code",
  "storymaker.code.files": "Files",
  "storymaker.code.editor_label": "Code editor",
  "storymaker.code.save": "Save",
  "storymaker.code.saved": "Saved",
  "storymaker.code.unsaved": "Unsaved changes",
  "storymaker.code.stale": "Changed. Save to check it again.",
  "storymaker.code.new_file": "New file, not saved yet",
  "storymaker.code.reload": "Reload",
  "storymaker.code.bundle_ok": "Code scene saved.",
  "storymaker.code.bundle_failed": "The code has errors.",
  "storymaker.code.no_problems": "No problems found.",
  "storymaker.code.problem_count": "Problems: {count}",
  "storymaker.code.line_col": "Line {line}, column {column}",
  "storymaker.code.more": "and {count} more",

  "storymaker.export.button": "Export",
  "storymaker.export.button_running": "Exporting {percent} %",
  "storymaker.export.title": "Export video",
  "storymaker.export.cap_ffmpeg": "FFmpeg",
  "storymaker.export.cap_browser": "Browser",
  "storymaker.export.ready": "Ready",
  "storymaker.export.ready_version": "Ready, version {version}",
  "storymaker.export.ffmpeg_missing": "Not found. Install FFmpeg or set its path in the Video Studio settings.",
  "storymaker.export.browser_ready": "Ready ({source})",
  "storymaker.export.browser_missing": "No browser is available to draw the frames.",
  "storymaker.export.source_config": "from the configuration",
  "storymaker.export.source_system": "installed on this system",
  "storymaker.export.source_pinned": "downloaded by AuraGo",
  "storymaker.export.source_unknown": "source unknown",
  "storymaker.export.still_works": "Editing and the preview work without them.",
  "storymaker.export.blocked_readonly": "The desktop is read-only.",
  "storymaker.export.blocked_ffmpeg": "FFmpeg is missing.",
  "storymaker.export.blocked_browser": "No browser is available.",
  "storymaker.export.blocked_errors": "The story has errors. Fix them first.",
  "storymaker.export.blocked_too_long": "The video is longer than the limit of {max} seconds.",
  "storymaker.export.blocked_not_ready": "The preview is still being prepared.",
  "storymaker.export.summary": "{duration} long, {format}, {width} × {height} pixels.",
  "storymaker.export.silent_note": "This version exports silent videos. Narration audio comes later.",
  "storymaker.export.start": "Start export",
  "storymaker.export.starting": "Starting…",
  "storymaker.export.queued": "Waiting for its turn…",
  "storymaker.export.phase_starting": "Starting",
  "storymaker.export.phase_compiling": "Preparing the story",
  "storymaker.export.phase_mixing": "Mixing the sound",
  "storymaker.export.phase_rendering": "Drawing the frames",
  "storymaker.export.phase_encoding": "Encoding the video",
  "storymaker.export.phase_writing": "Saving the file",
  "storymaker.export.frames": "{done} of {total} frames",
  "storymaker.export.cancel": "Cancel export",
  "storymaker.export.cancelling": "Cancelling…",
  "storymaker.export.cancelled": "The export was cancelled.",
  "storymaker.export.interrupted": "The export was interrupted by a restart.",
  "storymaker.export.done": "Export finished",
  "storymaker.export.output": "The video is in your project folder:",
  "storymaker.export.show_in_files": "Show in Files",
  "storymaker.export.copy_path": "Copy path",
  "storymaker.export.copied": "Copied",
  "storymaker.export.copy_failed": "The path could not be copied.",
  "storymaker.export.again": "Export again",

  "storymaker.job_error.invalid_story": "The story has errors, so it cannot be exported.",
  "storymaker.job_error.storymaker_disabled": "Storymaker is switched off.",
  "storymaker.job_error.browser_unavailable": "No browser could be started to draw the frames.",
  "storymaker.job_error.ffmpeg_unavailable": "FFmpeg is not available.",
  "storymaker.job_error.render_failed": "The export failed.",
  "storymaker.job_error.render_timeout": "The export took too long and was stopped.",
  "storymaker.job_error.cancelled": "The export was cancelled.",
  "storymaker.job_error.busy": "Another export is running. Try again in a moment.",
  "storymaker.job_error.code_error": "A code scene failed while drawing.",
  "storymaker.job_error.unknown": "The export failed.",

  "storymaker.error.unauthorized": "Your session has ended. Sign in again.",
  "storymaker.error.scope": "This sign-in is not allowed to do that.",
  "storymaker.error.readonly": "The desktop is read-only.",
  "storymaker.error.disabled": "Storymaker is switched off.",
  "storymaker.error.invalid_story": "The story was not saved because it has errors.",
  "storymaker.error.conflict": "The story was changed somewhere else.",
  "storymaker.error.required": "The server needs the current version of the story. Reload it.",
  "storymaker.error.too_large": "That is too large.",
  "storymaker.error.not_found": "That does not exist any more.",
  "storymaker.error.busy": "The server is busy. Try again in a moment.",
  "storymaker.error.network": "The server could not be reached.",
  "storymaker.error.generic": "Something went wrong."
}
```

`ui/lang/storymaker/es.json`:

```json
{
  "storymaker.title": "Storymaker",
  "storymaker.loading": "Cargando…",
  "storymaker.retry": "Reintentar",
  "storymaker.close": "Cerrar",
  "storymaker.delete": "Eliminar",
  "storymaker.back_to_projects": "Proyectos",
  "storymaker.preview": "Vista previa de Storymaker",
  "storymaker.play": "Reproducir",
  "storymaker.pause": "Pausa",
  "storymaker.timeline": "Línea de tiempo",
  "storymaker.tab_stage": "Guion gráfico",
  "storymaker.tab_code": "Código",
  "storymaker.side_scene": "Escena",
  "storymaker.side_library": "Biblioteca",
  "storymaker.save_saved": "Guardado",
  "storymaker.save_dirty": "Cambios sin guardar",
  "storymaker.save_saving": "Guardando…",
  "storymaker.save_error": "No guardado",
  "storymaker.save_offline": "Sin conexión, no guardado",
  "storymaker.save_conflict": "Cambiado en otro lugar",
  "storymaker.save_readonly": "Solo lectura",
  "storymaker.leave_title": "¿Salir con cambios sin guardar?",
  "storymaker.leave_text": "No se pudieron guardar tus últimos cambios en la historia. Si sales ahora, se perderán.",
  "storymaker.leave_code_text": "Un archivo de código tiene cambios sin guardar. Si sales ahora, se perderán.",
  "storymaker.aspect_16_9": "Horizontal 16:9",
  "storymaker.aspect_9_16": "Vertical 9:16",
  "storymaker.aspect_1_1": "Cuadrado 1:1",

  "storymaker.banner.readonly": "El escritorio está en modo de solo lectura. Puedes ver la vista previa, pero editar, guardar y exportar están desactivados.",
  "storymaker.banner.conflict": "Esta historia se cambió en otro lugar, por ejemplo en otra ventana. Recarga para ver la versión actual, o conserva la tuya y sobrescribe.",
  "storymaker.banner.conflict_reload": "Recargar",
  "storymaker.banner.conflict_overwrite": "Conservar la mía",
  "storymaker.banner.player_unavailable": "La vista previa no pudo iniciarse en este navegador. Tu historia sigue guardada y puedes seguir editándola.",
  "storymaker.banner.export_unavailable": "La exportación aún no está disponible (falta {missing}). La edición y la vista previa siguen funcionando.",

  "storymaker.state.disabled_title": "Storymaker está desactivado",
  "storymaker.state.disabled_text": "Un administrador puede activarlo en la configuración, en Escritorio virtual.",
  "storymaker.state.unauthorized_title": "Vuelve a iniciar sesión",
  "storymaker.state.unauthorized_text": "Tu sesión ha terminado. Inicia sesión de nuevo y abre Storymaker otra vez.",
  "storymaker.state.scope_title": "Sin acceso",
  "storymaker.state.scope_text": "Este inicio de sesión no tiene permiso para usar el Escritorio virtual.",
  "storymaker.state.load_failed_title": "Storymaker no pudo iniciarse",
  "storymaker.state.load_failed_text": "El servidor no respondió como se esperaba. Comprueba la conexión e inténtalo de nuevo.",

  "storymaker.projects.title": "Proyectos",
  "storymaker.projects.intro": "Convierte un tema en un breve vídeo explicativo: pon en escena las escenas, escribe la narración y exporta un MP4.",
  "storymaker.projects.new_title_label": "Título",
  "storymaker.projects.new_title_placeholder": "¿Qué es un cortafuegos?",
  "storymaker.projects.new_format": "Formato",
  "storymaker.projects.create": "Proyecto nuevo",
  "storymaker.projects.title_required": "Escribe un título.",
  "storymaker.projects.empty_title": "Aún no hay proyectos",
  "storymaker.projects.empty_text": "Crea tu primer proyecto arriba. Empieza con una escena vacía.",
  "storymaker.projects.delete_label": "Eliminar el proyecto {title}",
  "storymaker.projects.delete_title": "¿Eliminar el proyecto?",
  "storymaker.projects.delete_text": "El proyecto «{title}» y todos sus archivos, incluidas las exportaciones, se eliminarán. No se puede deshacer.",
  "storymaker.projects.deleted": "«{title}» eliminado.",

  "storymaker.strip.title": "Escenas",
  "storymaker.strip.scene_n": "Escena {n}",
  "storymaker.strip.duration_s": "{seconds} s",
  "storymaker.strip.kind_code": "Escena de código",
  "storymaker.strip.add_storyboard": "Añadir escena",
  "storymaker.strip.add_code": "Añadir escena de código",

  "storymaker.stage.compiling": "Preparando la vista previa…",
  "storymaker.stage.unavailable": "La vista previa no está disponible.",
  "storymaker.stage.player_error": "La vista previa informó de un problema.",
  "storymaker.stage.code_error": "Una escena de código tiene un error. Consulta la lista de problemas.",

  "storymaker.inspector.title": "Ajustes de la escena",
  "storymaker.inspector.environment": "Entorno",
  "storymaker.inspector.variant": "Momento del día",
  "storymaker.inspector.variant_day": "Día",
  "storymaker.inspector.variant_night": "Noche",
  "storymaker.inspector.narration": "Narración",
  "storymaker.inspector.narration_placeholder": "¿Qué dice el narrador en esta escena?",
  "storymaker.inspector.narration_hint": "La escena dura lo que tarda en leerse el texto, salvo que fijes una duración.",
  "storymaker.inspector.transition": "Transición hacia esta escena",
  "storymaker.inspector.transition_first": "La primera escena no tiene transición.",
  "storymaker.inspector.unsupported": "{name} (aún no compatible)",
  "storymaker.inspector.duration": "Duración",
  "storymaker.inspector.duration_auto": "Automática",
  "storymaker.inspector.duration_fixed": "Fija",
  "storymaker.inspector.duration_seconds": "Duración en segundos",
  "storymaker.inspector.seconds_unit": "segundos",
  "storymaker.inspector.cast_props": "Personajes y accesorios de esta escena",
  "storymaker.inspector.no_items": "Todavía nada. Elige personajes y accesorios en la biblioteca.",
  "storymaker.inspector.remove": "Quitar",
  "storymaker.inspector.remove_item": "Quitar {name} de la escena",
  "storymaker.inspector.move_up": "Subir la escena",
  "storymaker.inspector.move_down": "Bajar la escena",
  "storymaker.inspector.delete_scene": "Eliminar escena",
  "storymaker.inspector.delete_scene_text": "¿Eliminar la escena {scene}? Su narración y todo su contenido se quitarán de la historia.",
  "storymaker.inspector.code_scene_note": "Esta escena la dibuja un pequeño programa.",
  "storymaker.inspector.edit_code": "Editar código",

  "storymaker.library.title": "Biblioteca",
  "storymaker.library.people": "Personajes",
  "storymaker.library.props": "Accesorios",
  "storymaker.library.environments": "Lugares",
  "storymaker.library.search": "Buscar en la biblioteca",
  "storymaker.library.add": "Añadir a la escena",
  "storymaker.library.use": "Usar como entorno",
  "storymaker.library.empty": "No hay coincidencias.",
  "storymaker.library.code_scene": "Las escenas de código lo dibujan todo ellas mismas.",
  "storymaker.library.already_in_scene": "Ya está en esta escena.",
  "storymaker.library.added_actor": "{name} se unió a la escena.",
  "storymaker.library.added_prop": "Añadido a la escena: {name}.",

  "storymaker.issues.title": "Problemas",
  "storymaker.issues.none": "Sin problemas.",
  "storymaker.issues.count": "Problemas: {count}",
  "storymaker.issues.show": "Mostrar problemas",
  "storymaker.issues.show_in_code": "Mostrar en story.json",

  "storymaker.issue.invalid_json": "JSON no válido",
  "storymaker.issue.unknown_field": "Campo desconocido",
  "storymaker.issue.schema_mismatch": "Formato de historia incorrecto",
  "storymaker.issue.too_large": "Demasiado grande",
  "storymaker.issue.limit_exceeded": "Límite superado",
  "storymaker.issue.missing_field": "Falta un campo",
  "storymaker.issue.invalid_value": "Valor no válido",
  "storymaker.issue.duplicate_id": "Nombre duplicado",
  "storymaker.issue.unknown_name": "Nombre desconocido",
  "storymaker.issue.unknown_asset": "Elemento de catálogo desconocido",
  "storymaker.issue.unknown_spot": "Lugar desconocido",
  "storymaker.issue.unknown_cast": "Personaje desconocido",
  "storymaker.issue.unknown_cue": "Marca desconocida",
  "storymaker.issue.canonicalised": "Nombre normalizado",
  "storymaker.issue.not_supported_yet": "Aún no compatible",
  "storymaker.issue.narration_missing": "Sin audio de narración",
  "storymaker.issue.code_error": "Error en la escena de código",
  "storymaker.issue.deadline": "Tardó demasiado",
  "storymaker.issue.layout_unverified": "Diseño sin comprobar",
  "storymaker.issue.unknown": "Problema",

  "storymaker.code.title": "Código",
  "storymaker.code.files": "Archivos",
  "storymaker.code.editor_label": "Editor de código",
  "storymaker.code.save": "Guardar",
  "storymaker.code.saved": "Guardado",
  "storymaker.code.unsaved": "Cambios sin guardar",
  "storymaker.code.stale": "Modificado. Guarda para comprobarlo de nuevo.",
  "storymaker.code.new_file": "Archivo nuevo, aún sin guardar",
  "storymaker.code.reload": "Recargar",
  "storymaker.code.bundle_ok": "Escena de código guardada.",
  "storymaker.code.bundle_failed": "El código tiene errores.",
  "storymaker.code.no_problems": "No se encontraron problemas.",
  "storymaker.code.problem_count": "Problemas: {count}",
  "storymaker.code.line_col": "Línea {line}, columna {column}",
  "storymaker.code.more": "y {count} más",

  "storymaker.export.button": "Exportar",
  "storymaker.export.button_running": "Exportando {percent} %",
  "storymaker.export.title": "Exportar vídeo",
  "storymaker.export.cap_ffmpeg": "FFmpeg",
  "storymaker.export.cap_browser": "Navegador",
  "storymaker.export.ready": "Listo",
  "storymaker.export.ready_version": "Listo, versión {version}",
  "storymaker.export.ffmpeg_missing": "No se encuentra. Instala FFmpeg o indica su ruta en los ajustes de Video Studio.",
  "storymaker.export.browser_ready": "Listo ({source})",
  "storymaker.export.browser_missing": "No hay ningún navegador disponible para dibujar los fotogramas.",
  "storymaker.export.source_config": "desde la configuración",
  "storymaker.export.source_system": "instalado en este sistema",
  "storymaker.export.source_pinned": "descargado por AuraGo",
  "storymaker.export.source_unknown": "origen desconocido",
  "storymaker.export.still_works": "La edición y la vista previa funcionan sin ellos.",
  "storymaker.export.blocked_readonly": "El escritorio está en modo de solo lectura.",
  "storymaker.export.blocked_ffmpeg": "Falta FFmpeg.",
  "storymaker.export.blocked_browser": "No hay ningún navegador disponible.",
  "storymaker.export.blocked_errors": "La historia tiene errores. Corrígelos primero.",
  "storymaker.export.blocked_too_long": "El vídeo supera el límite de {max} segundos.",
  "storymaker.export.blocked_not_ready": "La vista previa aún se está preparando.",
  "storymaker.export.summary": "Duración {duration}, {format}, {width} × {height} píxeles.",
  "storymaker.export.silent_note": "Esta versión exporta vídeos sin sonido. El audio de la narración llegará más adelante.",
  "storymaker.export.start": "Iniciar exportación",
  "storymaker.export.starting": "Iniciando…",
  "storymaker.export.queued": "Esperando su turno…",
  "storymaker.export.phase_starting": "Iniciando",
  "storymaker.export.phase_compiling": "Preparando la historia",
  "storymaker.export.phase_mixing": "Mezclando el sonido",
  "storymaker.export.phase_rendering": "Dibujando los fotogramas",
  "storymaker.export.phase_encoding": "Codificando el vídeo",
  "storymaker.export.phase_writing": "Guardando el archivo",
  "storymaker.export.frames": "{done} de {total} fotogramas",
  "storymaker.export.cancel": "Cancelar exportación",
  "storymaker.export.cancelling": "Cancelando…",
  "storymaker.export.cancelled": "La exportación se canceló.",
  "storymaker.export.interrupted": "Un reinicio interrumpió la exportación.",
  "storymaker.export.done": "Exportación terminada",
  "storymaker.export.output": "El vídeo está en la carpeta de tu proyecto:",
  "storymaker.export.show_in_files": "Mostrar en Archivos",
  "storymaker.export.copy_path": "Copiar ruta",
  "storymaker.export.copied": "Copiado",
  "storymaker.export.copy_failed": "No se pudo copiar la ruta.",
  "storymaker.export.again": "Exportar de nuevo",

  "storymaker.job_error.invalid_story": "La historia tiene errores, así que no se puede exportar.",
  "storymaker.job_error.storymaker_disabled": "Storymaker está desactivado.",
  "storymaker.job_error.browser_unavailable": "No se pudo iniciar ningún navegador para dibujar los fotogramas.",
  "storymaker.job_error.ffmpeg_unavailable": "FFmpeg no está disponible.",
  "storymaker.job_error.render_failed": "La exportación falló.",
  "storymaker.job_error.render_timeout": "La exportación tardó demasiado y se detuvo.",
  "storymaker.job_error.cancelled": "La exportación se canceló.",
  "storymaker.job_error.busy": "Hay otra exportación en curso. Inténtalo de nuevo en un momento.",
  "storymaker.job_error.code_error": "Una escena de código falló al dibujar.",
  "storymaker.job_error.unknown": "La exportación falló.",

  "storymaker.error.unauthorized": "Tu sesión ha terminado. Vuelve a iniciar sesión.",
  "storymaker.error.scope": "Este inicio de sesión no tiene permiso para hacerlo.",
  "storymaker.error.readonly": "El escritorio está en modo de solo lectura.",
  "storymaker.error.disabled": "Storymaker está desactivado.",
  "storymaker.error.invalid_story": "La historia no se guardó porque tiene errores.",
  "storymaker.error.conflict": "La historia se cambió en otro lugar.",
  "storymaker.error.required": "El servidor necesita la versión actual de la historia. Recárgala.",
  "storymaker.error.too_large": "Es demasiado grande.",
  "storymaker.error.not_found": "Eso ya no existe.",
  "storymaker.error.busy": "El servidor está ocupado. Inténtalo de nuevo en un momento.",
  "storymaker.error.network": "No se pudo contactar con el servidor.",
  "storymaker.error.generic": "Algo salió mal."
}
```

`ui/lang/storymaker/fr.json`:

```json
{
  "storymaker.title": "Storymaker",
  "storymaker.loading": "Chargement…",
  "storymaker.retry": "Réessayer",
  "storymaker.close": "Fermer",
  "storymaker.delete": "Supprimer",
  "storymaker.back_to_projects": "Projets",
  "storymaker.preview": "Aperçu Storymaker",
  "storymaker.play": "Lire",
  "storymaker.pause": "Pause",
  "storymaker.timeline": "Chronologie",
  "storymaker.tab_stage": "Storyboard",
  "storymaker.tab_code": "Code",
  "storymaker.side_scene": "Scène",
  "storymaker.side_library": "Bibliothèque",
  "storymaker.save_saved": "Enregistré",
  "storymaker.save_dirty": "Modifications non enregistrées",
  "storymaker.save_saving": "Enregistrement…",
  "storymaker.save_error": "Non enregistré",
  "storymaker.save_offline": "Hors ligne, non enregistré",
  "storymaker.save_conflict": "Modifié ailleurs",
  "storymaker.save_readonly": "Lecture seule",
  "storymaker.leave_title": "Quitter avec des modifications non enregistrées ?",
  "storymaker.leave_text": "Tes dernières modifications de l'histoire n'ont pas pu être enregistrées. Si tu pars maintenant, elles sont perdues.",
  "storymaker.leave_code_text": "Un fichier de code contient des modifications non enregistrées. Si tu pars maintenant, elles sont perdues.",
  "storymaker.aspect_16_9": "Paysage 16:9",
  "storymaker.aspect_9_16": "Portrait 9:16",
  "storymaker.aspect_1_1": "Carré 1:1",

  "storymaker.banner.readonly": "Le bureau est en lecture seule. Tu peux regarder l'aperçu, mais la modification, l'enregistrement et l'export sont désactivés.",
  "storymaker.banner.conflict": "Cette histoire a été modifiée ailleurs, par exemple dans une autre fenêtre. Recharge pour voir la version actuelle, ou garde la tienne et écrase l'autre.",
  "storymaker.banner.conflict_reload": "Recharger",
  "storymaker.banner.conflict_overwrite": "Garder la mienne",
  "storymaker.banner.player_unavailable": "L'aperçu n'a pas pu démarrer dans ce navigateur. Ton histoire est bien enregistrée et tu peux continuer à la modifier.",
  "storymaker.banner.export_unavailable": "L'export n'est pas encore disponible ({missing} manquant). La modification et l'aperçu fonctionnent toujours.",

  "storymaker.state.disabled_title": "Storymaker est désactivé",
  "storymaker.state.disabled_text": "Un administrateur peut l'activer dans la configuration, sous Bureau virtuel.",
  "storymaker.state.unauthorized_title": "Reconnecte-toi",
  "storymaker.state.unauthorized_text": "Ta session est terminée. Reconnecte-toi puis rouvre Storymaker.",
  "storymaker.state.scope_title": "Accès refusé",
  "storymaker.state.scope_text": "Cette connexion n'a pas le droit d'utiliser le Bureau virtuel.",
  "storymaker.state.load_failed_title": "Storymaker n'a pas pu démarrer",
  "storymaker.state.load_failed_text": "Le serveur n'a pas répondu comme prévu. Vérifie la connexion et réessaie.",

  "storymaker.projects.title": "Projets",
  "storymaker.projects.intro": "Transforme un sujet en courte vidéo explicative : mets en scène, écris le commentaire et exporte un MP4.",
  "storymaker.projects.new_title_label": "Titre",
  "storymaker.projects.new_title_placeholder": "Qu'est-ce qu'un pare-feu ?",
  "storymaker.projects.new_format": "Format",
  "storymaker.projects.create": "Nouveau projet",
  "storymaker.projects.title_required": "Saisis un titre.",
  "storymaker.projects.empty_title": "Aucun projet pour l'instant",
  "storymaker.projects.empty_text": "Crée ton premier projet ci-dessus. Il commence par une scène vide.",
  "storymaker.projects.delete_label": "Supprimer le projet {title}",
  "storymaker.projects.delete_title": "Supprimer le projet ?",
  "storymaker.projects.delete_text": "Le projet « {title} » et tous ses fichiers, exports compris, seront supprimés. Cette action est irréversible.",
  "storymaker.projects.deleted": "« {title} » a été supprimé.",

  "storymaker.strip.title": "Scènes",
  "storymaker.strip.scene_n": "Scène {n}",
  "storymaker.strip.duration_s": "{seconds} s",
  "storymaker.strip.kind_code": "Scène de code",
  "storymaker.strip.add_storyboard": "Ajouter une scène",
  "storymaker.strip.add_code": "Ajouter une scène de code",

  "storymaker.stage.compiling": "Préparation de l'aperçu…",
  "storymaker.stage.unavailable": "L'aperçu n'est pas disponible.",
  "storymaker.stage.player_error": "L'aperçu a signalé un problème.",
  "storymaker.stage.code_error": "Une scène de code contient une erreur. Voir la liste des problèmes.",

  "storymaker.inspector.title": "Réglages de la scène",
  "storymaker.inspector.environment": "Décor",
  "storymaker.inspector.variant": "Moment de la journée",
  "storymaker.inspector.variant_day": "Jour",
  "storymaker.inspector.variant_night": "Nuit",
  "storymaker.inspector.narration": "Commentaire",
  "storymaker.inspector.narration_placeholder": "Que dit le narrateur dans cette scène ?",
  "storymaker.inspector.narration_hint": "La scène dure le temps de lire le texte, sauf si tu fixes une durée.",
  "storymaker.inspector.transition": "Transition vers cette scène",
  "storymaker.inspector.transition_first": "La première scène n'a pas de transition.",
  "storymaker.inspector.unsupported": "{name} (pas encore pris en charge)",
  "storymaker.inspector.duration": "Durée",
  "storymaker.inspector.duration_auto": "Automatique",
  "storymaker.inspector.duration_fixed": "Fixe",
  "storymaker.inspector.duration_seconds": "Durée en secondes",
  "storymaker.inspector.seconds_unit": "secondes",
  "storymaker.inspector.cast_props": "Personnages et accessoires de cette scène",
  "storymaker.inspector.no_items": "Rien pour l'instant. Choisis des personnages et des accessoires dans la bibliothèque.",
  "storymaker.inspector.remove": "Retirer",
  "storymaker.inspector.remove_item": "Retirer {name} de la scène",
  "storymaker.inspector.move_up": "Monter la scène",
  "storymaker.inspector.move_down": "Descendre la scène",
  "storymaker.inspector.delete_scene": "Supprimer la scène",
  "storymaker.inspector.delete_scene_text": "Supprimer la scène {scene} ? Son commentaire et tout son contenu seront retirés de l'histoire.",
  "storymaker.inspector.code_scene_note": "Cette scène est dessinée par un petit programme.",
  "storymaker.inspector.edit_code": "Modifier le code",

  "storymaker.library.title": "Bibliothèque",
  "storymaker.library.people": "Personnages",
  "storymaker.library.props": "Accessoires",
  "storymaker.library.environments": "Lieux",
  "storymaker.library.search": "Rechercher dans la bibliothèque",
  "storymaker.library.add": "Ajouter à la scène",
  "storymaker.library.use": "Utiliser comme décor",
  "storymaker.library.empty": "Aucun résultat.",
  "storymaker.library.code_scene": "Les scènes de code dessinent tout elles-mêmes.",
  "storymaker.library.already_in_scene": "Déjà dans cette scène.",
  "storymaker.library.added_actor": "{name} a rejoint la scène.",
  "storymaker.library.added_prop": "Ajouté à la scène : {name}.",

  "storymaker.issues.title": "Problèmes",
  "storymaker.issues.none": "Aucun problème.",
  "storymaker.issues.count": "Problèmes : {count}",
  "storymaker.issues.show": "Afficher les problèmes",
  "storymaker.issues.show_in_code": "Afficher dans story.json",

  "storymaker.issue.invalid_json": "JSON non valide",
  "storymaker.issue.unknown_field": "Champ inconnu",
  "storymaker.issue.schema_mismatch": "Mauvais format d'histoire",
  "storymaker.issue.too_large": "Trop volumineux",
  "storymaker.issue.limit_exceeded": "Limite dépassée",
  "storymaker.issue.missing_field": "Champ manquant",
  "storymaker.issue.invalid_value": "Valeur non valide",
  "storymaker.issue.duplicate_id": "Nom en double",
  "storymaker.issue.unknown_name": "Nom inconnu",
  "storymaker.issue.unknown_asset": "Élément de catalogue inconnu",
  "storymaker.issue.unknown_spot": "Emplacement inconnu",
  "storymaker.issue.unknown_cast": "Personnage inconnu",
  "storymaker.issue.unknown_cue": "Repère inconnu",
  "storymaker.issue.canonicalised": "Nom normalisé",
  "storymaker.issue.not_supported_yet": "Pas encore pris en charge",
  "storymaker.issue.narration_missing": "Pas d'audio de commentaire",
  "storymaker.issue.code_error": "Erreur dans la scène de code",
  "storymaker.issue.deadline": "A pris trop de temps",
  "storymaker.issue.layout_unverified": "Mise en page non vérifiée",
  "storymaker.issue.unknown": "Problème",

  "storymaker.code.title": "Code",
  "storymaker.code.files": "Fichiers",
  "storymaker.code.editor_label": "Éditeur de code",
  "storymaker.code.save": "Enregistrer",
  "storymaker.code.saved": "Enregistré",
  "storymaker.code.unsaved": "Modifications non enregistrées",
  "storymaker.code.stale": "Modifié. Enregistre pour le vérifier à nouveau.",
  "storymaker.code.new_file": "Nouveau fichier, pas encore enregistré",
  "storymaker.code.reload": "Recharger",
  "storymaker.code.bundle_ok": "Scène de code enregistrée.",
  "storymaker.code.bundle_failed": "Le code contient des erreurs.",
  "storymaker.code.no_problems": "Aucun problème trouvé.",
  "storymaker.code.problem_count": "Problèmes : {count}",
  "storymaker.code.line_col": "Ligne {line}, colonne {column}",
  "storymaker.code.more": "et {count} de plus",

  "storymaker.export.button": "Exporter",
  "storymaker.export.button_running": "Export {percent} %",
  "storymaker.export.title": "Exporter la vidéo",
  "storymaker.export.cap_ffmpeg": "FFmpeg",
  "storymaker.export.cap_browser": "Navigateur",
  "storymaker.export.ready": "Prêt",
  "storymaker.export.ready_version": "Prêt, version {version}",
  "storymaker.export.ffmpeg_missing": "Introuvable. Installe FFmpeg ou indique son chemin dans les réglages de Video Studio.",
  "storymaker.export.browser_ready": "Prêt ({source})",
  "storymaker.export.browser_missing": "Aucun navigateur n'est disponible pour dessiner les images.",
  "storymaker.export.source_config": "depuis la configuration",
  "storymaker.export.source_system": "installé sur ce système",
  "storymaker.export.source_pinned": "téléchargé par AuraGo",
  "storymaker.export.source_unknown": "source inconnue",
  "storymaker.export.still_works": "La modification et l'aperçu fonctionnent sans eux.",
  "storymaker.export.blocked_readonly": "Le bureau est en lecture seule.",
  "storymaker.export.blocked_ffmpeg": "FFmpeg est absent.",
  "storymaker.export.blocked_browser": "Aucun navigateur n'est disponible.",
  "storymaker.export.blocked_errors": "L'histoire contient des erreurs. Corrige-les d'abord.",
  "storymaker.export.blocked_too_long": "La vidéo dépasse la limite de {max} secondes.",
  "storymaker.export.blocked_not_ready": "L'aperçu est encore en préparation.",
  "storymaker.export.summary": "Durée {duration}, {format}, {width} × {height} pixels.",
  "storymaker.export.silent_note": "Cette version exporte des vidéos muettes. L'audio du commentaire arrivera plus tard.",
  "storymaker.export.start": "Lancer l'export",
  "storymaker.export.starting": "Démarrage…",
  "storymaker.export.queued": "En attente de son tour…",
  "storymaker.export.phase_starting": "Démarrage",
  "storymaker.export.phase_compiling": "Préparation de l'histoire",
  "storymaker.export.phase_mixing": "Mixage du son",
  "storymaker.export.phase_rendering": "Dessin des images",
  "storymaker.export.phase_encoding": "Encodage de la vidéo",
  "storymaker.export.phase_writing": "Enregistrement du fichier",
  "storymaker.export.frames": "{done} images sur {total}",
  "storymaker.export.cancel": "Annuler l'export",
  "storymaker.export.cancelling": "Annulation…",
  "storymaker.export.cancelled": "L'export a été annulé.",
  "storymaker.export.interrupted": "L'export a été interrompu par un redémarrage.",
  "storymaker.export.done": "Export terminé",
  "storymaker.export.output": "La vidéo se trouve dans le dossier de ton projet :",
  "storymaker.export.show_in_files": "Afficher dans Fichiers",
  "storymaker.export.copy_path": "Copier le chemin",
  "storymaker.export.copied": "Copié",
  "storymaker.export.copy_failed": "Le chemin n'a pas pu être copié.",
  "storymaker.export.again": "Exporter à nouveau",

  "storymaker.job_error.invalid_story": "L'histoire contient des erreurs, elle ne peut donc pas être exportée.",
  "storymaker.job_error.storymaker_disabled": "Storymaker est désactivé.",
  "storymaker.job_error.browser_unavailable": "Aucun navigateur n'a pu démarrer pour dessiner les images.",
  "storymaker.job_error.ffmpeg_unavailable": "FFmpeg n'est pas disponible.",
  "storymaker.job_error.render_failed": "L'export a échoué.",
  "storymaker.job_error.render_timeout": "L'export a pris trop de temps et a été arrêté.",
  "storymaker.job_error.cancelled": "L'export a été annulé.",
  "storymaker.job_error.busy": "Un autre export est en cours. Réessaie dans un instant.",
  "storymaker.job_error.code_error": "Une scène de code a échoué pendant le dessin.",
  "storymaker.job_error.unknown": "L'export a échoué.",

  "storymaker.error.unauthorized": "Ta session est terminée. Reconnecte-toi.",
  "storymaker.error.scope": "Cette connexion n'a pas le droit de faire cela.",
  "storymaker.error.readonly": "Le bureau est en lecture seule.",
  "storymaker.error.disabled": "Storymaker est désactivé.",
  "storymaker.error.invalid_story": "L'histoire n'a pas été enregistrée car elle contient des erreurs.",
  "storymaker.error.conflict": "L'histoire a été modifiée ailleurs.",
  "storymaker.error.required": "Le serveur a besoin de la version actuelle de l'histoire. Recharge-la.",
  "storymaker.error.too_large": "C'est trop volumineux.",
  "storymaker.error.not_found": "Cela n'existe plus.",
  "storymaker.error.busy": "Le serveur est occupé. Réessaie dans un instant.",
  "storymaker.error.network": "Le serveur est injoignable.",
  "storymaker.error.generic": "Une erreur s'est produite."
}
```

`ui/lang/storymaker/hi.json`:

```json
{
  "storymaker.title": "Storymaker",
  "storymaker.loading": "लोड हो रहा है…",
  "storymaker.retry": "फिर से कोशिश करें",
  "storymaker.close": "बंद करें",
  "storymaker.delete": "हटाएँ",
  "storymaker.back_to_projects": "प्रोजेक्ट",
  "storymaker.preview": "Storymaker प्रीव्यू",
  "storymaker.play": "चलाएँ",
  "storymaker.pause": "रोकें",
  "storymaker.timeline": "टाइमलाइन",
  "storymaker.tab_stage": "स्टोरीबोर्ड",
  "storymaker.tab_code": "कोड",
  "storymaker.side_scene": "दृश्य",
  "storymaker.side_library": "लाइब्रेरी",
  "storymaker.save_saved": "सहेजा गया",
  "storymaker.save_dirty": "बिना सहेजे बदलाव",
  "storymaker.save_saving": "सहेजा जा रहा है…",
  "storymaker.save_error": "सहेजा नहीं गया",
  "storymaker.save_offline": "ऑफ़लाइन, सहेजा नहीं गया",
  "storymaker.save_conflict": "कहीं और बदला गया",
  "storymaker.save_readonly": "केवल पढ़ने के लिए",
  "storymaker.leave_title": "बिना सहेजे बदलावों के साथ बाहर निकलें?",
  "storymaker.leave_text": "कहानी में आपके ताज़ा बदलाव सहेजे नहीं जा सके। अभी बाहर निकले तो वे खो जाएँगे।",
  "storymaker.leave_code_text": "एक कोड फ़ाइल में बिना सहेजे बदलाव हैं। अभी बाहर निकले तो वे खो जाएँगे।",
  "storymaker.aspect_16_9": "लैंडस्केप 16:9",
  "storymaker.aspect_9_16": "पोर्ट्रेट 9:16",
  "storymaker.aspect_1_1": "वर्गाकार 1:1",

  "storymaker.banner.readonly": "डेस्कटॉप केवल पढ़ने के लिए है। आप प्रीव्यू देख सकते हैं, लेकिन संपादन, सहेजना और एक्सपोर्ट बंद हैं।",
  "storymaker.banner.conflict": "यह कहानी कहीं और बदली गई है, जैसे किसी दूसरी विंडो में। मौजूदा संस्करण देखने के लिए फिर से लोड करें, या अपना संस्करण रखकर उसे ओवरराइट करें।",
  "storymaker.banner.conflict_reload": "फिर से लोड करें",
  "storymaker.banner.conflict_overwrite": "मेरा रखें",
  "storymaker.banner.player_unavailable": "इस ब्राउज़र में प्रीव्यू शुरू नहीं हो सका। आपकी कहानी फिर भी सहेजी हुई है और आप उसे संपादित करते रह सकते हैं।",
  "storymaker.banner.export_unavailable": "एक्सपोर्ट अभी उपलब्ध नहीं है ({missing} नहीं मिला)। संपादन और प्रीव्यू फिर भी काम करते हैं।",

  "storymaker.state.disabled_title": "Storymaker बंद है",
  "storymaker.state.disabled_text": "व्यवस्थापक इसे कॉन्फ़िगरेशन में वर्चुअल डेस्कटॉप के अंतर्गत चालू कर सकता है।",
  "storymaker.state.unauthorized_title": "कृपया फिर से साइन इन करें",
  "storymaker.state.unauthorized_text": "आपका सत्र समाप्त हो गया है। फिर से साइन इन करें और Storymaker दोबारा खोलें।",
  "storymaker.state.scope_title": "पहुँच नहीं है",
  "storymaker.state.scope_text": "इस साइन-इन को वर्चुअल डेस्कटॉप इस्तेमाल करने की अनुमति नहीं है।",
  "storymaker.state.load_failed_title": "Storymaker शुरू नहीं हो सका",
  "storymaker.state.load_failed_text": "सर्वर ने अपेक्षित जवाब नहीं दिया। कनेक्शन जाँचें और फिर से कोशिश करें।",

  "storymaker.projects.title": "प्रोजेक्ट",
  "storymaker.projects.intro": "किसी विषय को छोटे समझाने वाले वीडियो में बदलें: दृश्य सजाएँ, वर्णन लिखें और MP4 एक्सपोर्ट करें।",
  "storymaker.projects.new_title_label": "शीर्षक",
  "storymaker.projects.new_title_placeholder": "फ़ायरवॉल क्या होता है?",
  "storymaker.projects.new_format": "फ़ॉर्मैट",
  "storymaker.projects.create": "नया प्रोजेक्ट",
  "storymaker.projects.title_required": "कृपया शीर्षक लिखें।",
  "storymaker.projects.empty_title": "अभी कोई प्रोजेक्ट नहीं",
  "storymaker.projects.empty_text": "ऊपर अपना पहला प्रोजेक्ट बनाएँ। यह एक खाली दृश्य से शुरू होता है।",
  "storymaker.projects.delete_label": "प्रोजेक्ट {title} हटाएँ",
  "storymaker.projects.delete_title": "प्रोजेक्ट हटाएँ?",
  "storymaker.projects.delete_text": "प्रोजेक्ट \"{title}\" और उसकी सभी फ़ाइलें, एक्सपोर्ट समेत, हटा दी जाएँगी। इसे वापस नहीं किया जा सकता।",
  "storymaker.projects.deleted": "\"{title}\" हटा दिया गया।",

  "storymaker.strip.title": "दृश्य",
  "storymaker.strip.scene_n": "दृश्य {n}",
  "storymaker.strip.duration_s": "{seconds} से.",
  "storymaker.strip.kind_code": "कोड दृश्य",
  "storymaker.strip.add_storyboard": "दृश्य जोड़ें",
  "storymaker.strip.add_code": "कोड दृश्य जोड़ें",

  "storymaker.stage.compiling": "प्रीव्यू तैयार हो रहा है…",
  "storymaker.stage.unavailable": "प्रीव्यू उपलब्ध नहीं है।",
  "storymaker.stage.player_error": "प्रीव्यू ने समस्या बताई।",
  "storymaker.stage.code_error": "एक कोड दृश्य में त्रुटि है। समस्याओं की सूची देखें।",

  "storymaker.inspector.title": "दृश्य की सेटिंग",
  "storymaker.inspector.environment": "परिवेश",
  "storymaker.inspector.variant": "दिन का समय",
  "storymaker.inspector.variant_day": "दिन",
  "storymaker.inspector.variant_night": "रात",
  "storymaker.inspector.narration": "वर्णन",
  "storymaker.inspector.narration_placeholder": "इस दृश्य में वर्णनकर्ता क्या कहता है?",
  "storymaker.inspector.narration_hint": "दृश्य उतनी देर चलता है जितनी देर में पाठ पढ़ा जाता है, जब तक आप अवधि तय न करें।",
  "storymaker.inspector.transition": "इस दृश्य में बदलाव",
  "storymaker.inspector.transition_first": "पहले दृश्य में कोई बदलाव नहीं होता।",
  "storymaker.inspector.unsupported": "{name} (अभी समर्थित नहीं)",
  "storymaker.inspector.duration": "अवधि",
  "storymaker.inspector.duration_auto": "स्वचालित",
  "storymaker.inspector.duration_fixed": "तय",
  "storymaker.inspector.duration_seconds": "सेकंड में अवधि",
  "storymaker.inspector.seconds_unit": "सेकंड",
  "storymaker.inspector.cast_props": "इस दृश्य के पात्र और सामान",
  "storymaker.inspector.no_items": "अभी कुछ नहीं। लाइब्रेरी से पात्र और सामान चुनें।",
  "storymaker.inspector.remove": "हटाएँ",
  "storymaker.inspector.remove_item": "दृश्य से {name} हटाएँ",
  "storymaker.inspector.move_up": "दृश्य ऊपर ले जाएँ",
  "storymaker.inspector.move_down": "दृश्य नीचे ले जाएँ",
  "storymaker.inspector.delete_scene": "दृश्य हटाएँ",
  "storymaker.inspector.delete_scene_text": "दृश्य {scene} हटाएँ? उसका वर्णन और उसमें सब कुछ कहानी से हट जाएगा।",
  "storymaker.inspector.code_scene_note": "यह दृश्य एक छोटे प्रोग्राम से बनता है।",
  "storymaker.inspector.edit_code": "कोड संपादित करें",

  "storymaker.library.title": "लाइब्रेरी",
  "storymaker.library.people": "पात्र",
  "storymaker.library.props": "सामान",
  "storymaker.library.environments": "स्थान",
  "storymaker.library.search": "लाइब्रेरी में खोजें",
  "storymaker.library.add": "दृश्य में जोड़ें",
  "storymaker.library.use": "परिवेश के रूप में इस्तेमाल करें",
  "storymaker.library.empty": "कुछ नहीं मिला।",
  "storymaker.library.code_scene": "कोड दृश्य सब कुछ खुद बनाते हैं।",
  "storymaker.library.already_in_scene": "यह पहले से इस दृश्य में है।",
  "storymaker.library.added_actor": "दृश्य में जोड़ा गया: {name}।",
  "storymaker.library.added_prop": "दृश्य में जोड़ा गया: {name}।",

  "storymaker.issues.title": "समस्याएँ",
  "storymaker.issues.none": "कोई समस्या नहीं।",
  "storymaker.issues.count": "समस्याएँ: {count}",
  "storymaker.issues.show": "समस्याएँ दिखाएँ",
  "storymaker.issues.show_in_code": "story.json में दिखाएँ",

  "storymaker.issue.invalid_json": "अमान्य JSON",
  "storymaker.issue.unknown_field": "अज्ञात फ़ील्ड",
  "storymaker.issue.schema_mismatch": "कहानी का गलत फ़ॉर्मैट",
  "storymaker.issue.too_large": "बहुत बड़ा",
  "storymaker.issue.limit_exceeded": "सीमा पार हो गई",
  "storymaker.issue.missing_field": "फ़ील्ड नहीं है",
  "storymaker.issue.invalid_value": "अमान्य मान",
  "storymaker.issue.duplicate_id": "नाम दोहराया गया",
  "storymaker.issue.unknown_name": "अज्ञात नाम",
  "storymaker.issue.unknown_asset": "कैटलॉग की अज्ञात वस्तु",
  "storymaker.issue.unknown_spot": "अज्ञात जगह",
  "storymaker.issue.unknown_cast": "अज्ञात पात्र",
  "storymaker.issue.unknown_cue": "अज्ञात संकेत",
  "storymaker.issue.canonicalised": "नाम मानक रूप में बदला गया",
  "storymaker.issue.not_supported_yet": "अभी समर्थित नहीं",
  "storymaker.issue.narration_missing": "वर्णन की ऑडियो नहीं है",
  "storymaker.issue.code_error": "कोड दृश्य में त्रुटि",
  "storymaker.issue.deadline": "बहुत ज़्यादा समय लगा",
  "storymaker.issue.layout_unverified": "लेआउट जाँचा नहीं गया",
  "storymaker.issue.unknown": "समस्या",

  "storymaker.code.title": "कोड",
  "storymaker.code.files": "फ़ाइलें",
  "storymaker.code.editor_label": "कोड एडिटर",
  "storymaker.code.save": "सहेजें",
  "storymaker.code.saved": "सहेजा गया",
  "storymaker.code.unsaved": "बिना सहेजे बदलाव",
  "storymaker.code.stale": "बदला गया। दोबारा जाँचने के लिए सहेजें।",
  "storymaker.code.new_file": "नई फ़ाइल, अभी सहेजी नहीं गई",
  "storymaker.code.reload": "फिर से लोड करें",
  "storymaker.code.bundle_ok": "कोड दृश्य सहेजा गया।",
  "storymaker.code.bundle_failed": "कोड में त्रुटियाँ हैं।",
  "storymaker.code.no_problems": "कोई समस्या नहीं मिली।",
  "storymaker.code.problem_count": "समस्याएँ: {count}",
  "storymaker.code.line_col": "पंक्ति {line}, कॉलम {column}",
  "storymaker.code.more": "और {count} और",

  "storymaker.export.button": "एक्सपोर्ट",
  "storymaker.export.button_running": "एक्सपोर्ट {percent} %",
  "storymaker.export.title": "वीडियो एक्सपोर्ट करें",
  "storymaker.export.cap_ffmpeg": "FFmpeg",
  "storymaker.export.cap_browser": "ब्राउज़र",
  "storymaker.export.ready": "तैयार",
  "storymaker.export.ready_version": "तैयार, संस्करण {version}",
  "storymaker.export.ffmpeg_missing": "नहीं मिला। FFmpeg इंस्टॉल करें या Video Studio की सेटिंग में उसका पथ दें।",
  "storymaker.export.browser_ready": "तैयार ({source})",
  "storymaker.export.browser_missing": "फ़्रेम बनाने के लिए कोई ब्राउज़र उपलब्ध नहीं है।",
  "storymaker.export.source_config": "कॉन्फ़िगरेशन से",
  "storymaker.export.source_system": "इस सिस्टम पर इंस्टॉल",
  "storymaker.export.source_pinned": "AuraGo द्वारा डाउनलोड किया गया",
  "storymaker.export.source_unknown": "स्रोत अज्ञात",
  "storymaker.export.still_works": "इनके बिना भी संपादन और प्रीव्यू काम करते हैं।",
  "storymaker.export.blocked_readonly": "डेस्कटॉप केवल पढ़ने के लिए है।",
  "storymaker.export.blocked_ffmpeg": "FFmpeg नहीं है।",
  "storymaker.export.blocked_browser": "कोई ब्राउज़र उपलब्ध नहीं है।",
  "storymaker.export.blocked_errors": "कहानी में त्रुटियाँ हैं। पहले उन्हें ठीक करें।",
  "storymaker.export.blocked_too_long": "वीडियो {max} सेकंड की सीमा से लंबा है।",
  "storymaker.export.blocked_not_ready": "प्रीव्यू अभी तैयार हो रहा है।",
  "storymaker.export.summary": "अवधि {duration}, {format}, {width} × {height} पिक्सेल।",
  "storymaker.export.silent_note": "यह संस्करण बिना आवाज़ वाले वीडियो एक्सपोर्ट करता है। वर्णन की ऑडियो बाद में आएगी।",
  "storymaker.export.start": "एक्सपोर्ट शुरू करें",
  "storymaker.export.starting": "शुरू हो रहा है…",
  "storymaker.export.queued": "अपनी बारी का इंतज़ार…",
  "storymaker.export.phase_starting": "शुरू हो रहा है",
  "storymaker.export.phase_compiling": "कहानी तैयार हो रही है",
  "storymaker.export.phase_mixing": "आवाज़ मिलाई जा रही है",
  "storymaker.export.phase_rendering": "फ़्रेम बनाए जा रहे हैं",
  "storymaker.export.phase_encoding": "वीडियो एन्कोड हो रहा है",
  "storymaker.export.phase_writing": "फ़ाइल सहेजी जा रही है",
  "storymaker.export.frames": "{total} में से {done} फ़्रेम",
  "storymaker.export.cancel": "एक्सपोर्ट रद्द करें",
  "storymaker.export.cancelling": "रद्द हो रहा है…",
  "storymaker.export.cancelled": "एक्सपोर्ट रद्द कर दिया गया।",
  "storymaker.export.interrupted": "रीस्टार्ट के कारण एक्सपोर्ट रुक गया।",
  "storymaker.export.done": "एक्सपोर्ट पूरा हुआ",
  "storymaker.export.output": "वीडियो आपके प्रोजेक्ट फ़ोल्डर में है:",
  "storymaker.export.show_in_files": "फ़ाइलें में दिखाएँ",
  "storymaker.export.copy_path": "पथ कॉपी करें",
  "storymaker.export.copied": "कॉपी हो गया",
  "storymaker.export.copy_failed": "पथ कॉपी नहीं हो सका।",
  "storymaker.export.again": "फिर से एक्सपोर्ट करें",

  "storymaker.job_error.invalid_story": "कहानी में त्रुटियाँ हैं, इसलिए उसे एक्सपोर्ट नहीं किया जा सकता।",
  "storymaker.job_error.storymaker_disabled": "Storymaker बंद है।",
  "storymaker.job_error.browser_unavailable": "फ़्रेम बनाने के लिए कोई ब्राउज़र शुरू नहीं हो सका।",
  "storymaker.job_error.ffmpeg_unavailable": "FFmpeg उपलब्ध नहीं है।",
  "storymaker.job_error.render_failed": "एक्सपोर्ट विफल रहा।",
  "storymaker.job_error.render_timeout": "एक्सपोर्ट में बहुत समय लगा और उसे रोक दिया गया।",
  "storymaker.job_error.cancelled": "एक्सपोर्ट रद्द कर दिया गया।",
  "storymaker.job_error.busy": "दूसरा एक्सपोर्ट चल रहा है। थोड़ी देर बाद फिर कोशिश करें।",
  "storymaker.job_error.code_error": "एक कोड दृश्य बनाते समय विफल हो गया।",
  "storymaker.job_error.unknown": "एक्सपोर्ट विफल रहा।",

  "storymaker.error.unauthorized": "आपका सत्र समाप्त हो गया है। फिर से साइन इन करें।",
  "storymaker.error.scope": "इस साइन-इन को यह करने की अनुमति नहीं है।",
  "storymaker.error.readonly": "डेस्कटॉप केवल पढ़ने के लिए है।",
  "storymaker.error.disabled": "Storymaker बंद है।",
  "storymaker.error.invalid_story": "कहानी सहेजी नहीं गई क्योंकि उसमें त्रुटियाँ हैं।",
  "storymaker.error.conflict": "कहानी कहीं और बदली गई है।",
  "storymaker.error.required": "सर्वर को कहानी का मौजूदा संस्करण चाहिए। उसे फिर से लोड करें।",
  "storymaker.error.too_large": "यह बहुत बड़ा है।",
  "storymaker.error.not_found": "यह अब मौजूद नहीं है।",
  "storymaker.error.busy": "सर्वर व्यस्त है। थोड़ी देर बाद फिर कोशिश करें।",
  "storymaker.error.network": "सर्वर तक पहुँचा नहीं जा सका।",
  "storymaker.error.generic": "कुछ गड़बड़ हो गई।"
}
```

`ui/lang/storymaker/it.json`:

```json
{
  "storymaker.title": "Storymaker",
  "storymaker.loading": "Caricamento…",
  "storymaker.retry": "Riprova",
  "storymaker.close": "Chiudi",
  "storymaker.delete": "Elimina",
  "storymaker.back_to_projects": "Progetti",
  "storymaker.preview": "Anteprima di Storymaker",
  "storymaker.play": "Riproduci",
  "storymaker.pause": "Pausa",
  "storymaker.timeline": "Linea del tempo",
  "storymaker.tab_stage": "Storyboard",
  "storymaker.tab_code": "Codice",
  "storymaker.side_scene": "Scena",
  "storymaker.side_library": "Libreria",
  "storymaker.save_saved": "Salvato",
  "storymaker.save_dirty": "Modifiche non salvate",
  "storymaker.save_saving": "Salvataggio…",
  "storymaker.save_error": "Non salvato",
  "storymaker.save_offline": "Offline, non salvato",
  "storymaker.save_conflict": "Modificato altrove",
  "storymaker.save_readonly": "Sola lettura",
  "storymaker.leave_title": "Uscire con modifiche non salvate?",
  "storymaker.leave_text": "Non è stato possibile salvare le tue ultime modifiche alla storia. Se esci ora, andranno perse.",
  "storymaker.leave_code_text": "Un file di codice ha modifiche non salvate. Se esci ora, andranno perse.",
  "storymaker.aspect_16_9": "Orizzontale 16:9",
  "storymaker.aspect_9_16": "Verticale 9:16",
  "storymaker.aspect_1_1": "Quadrato 1:1",

  "storymaker.banner.readonly": "Il desktop è in sola lettura. Puoi guardare l'anteprima, ma modifica, salvataggio ed esportazione sono disattivati.",
  "storymaker.banner.conflict": "Questa storia è stata modificata altrove, per esempio in un'altra finestra. Ricarica per vedere la versione attuale, oppure tieni la tua e sovrascrivi.",
  "storymaker.banner.conflict_reload": "Ricarica",
  "storymaker.banner.conflict_overwrite": "Tieni la mia",
  "storymaker.banner.player_unavailable": "L'anteprima non è partita in questo browser. La tua storia è comunque salvata e puoi continuare a modificarla.",
  "storymaker.banner.export_unavailable": "L'esportazione non è ancora disponibile (manca: {missing}). Modifica e anteprima funzionano comunque.",

  "storymaker.state.disabled_title": "Storymaker è disattivato",
  "storymaker.state.disabled_text": "Un amministratore può attivarlo nella configurazione, in Desktop virtuale.",
  "storymaker.state.unauthorized_title": "Accedi di nuovo",
  "storymaker.state.unauthorized_text": "La sessione è terminata. Accedi di nuovo e riapri Storymaker.",
  "storymaker.state.scope_title": "Nessun accesso",
  "storymaker.state.scope_text": "Questo accesso non è autorizzato a usare il Desktop virtuale.",
  "storymaker.state.load_failed_title": "Storymaker non è riuscito ad avviarsi",
  "storymaker.state.load_failed_text": "Il server non ha risposto come previsto. Controlla la connessione e riprova.",

  "storymaker.projects.title": "Progetti",
  "storymaker.projects.intro": "Trasforma un argomento in un breve video esplicativo: metti in scena, scrivi la narrazione ed esporta un MP4.",
  "storymaker.projects.new_title_label": "Titolo",
  "storymaker.projects.new_title_placeholder": "Che cos'è un firewall?",
  "storymaker.projects.new_format": "Formato",
  "storymaker.projects.create": "Nuovo progetto",
  "storymaker.projects.title_required": "Inserisci un titolo.",
  "storymaker.projects.empty_title": "Ancora nessun progetto",
  "storymaker.projects.empty_text": "Crea il tuo primo progetto qui sopra. Parte con una scena vuota.",
  "storymaker.projects.delete_label": "Elimina il progetto {title}",
  "storymaker.projects.delete_title": "Eliminare il progetto?",
  "storymaker.projects.delete_text": "Il progetto «{title}» e tutti i suoi file, esportazioni comprese, verranno eliminati. L'operazione non si può annullare.",
  "storymaker.projects.deleted": "«{title}» eliminato.",

  "storymaker.strip.title": "Scene",
  "storymaker.strip.scene_n": "Scena {n}",
  "storymaker.strip.duration_s": "{seconds} s",
  "storymaker.strip.kind_code": "Scena di codice",
  "storymaker.strip.add_storyboard": "Aggiungi scena",
  "storymaker.strip.add_code": "Aggiungi scena di codice",

  "storymaker.stage.compiling": "Preparazione dell'anteprima…",
  "storymaker.stage.unavailable": "L'anteprima non è disponibile.",
  "storymaker.stage.player_error": "L'anteprima ha segnalato un problema.",
  "storymaker.stage.code_error": "Una scena di codice ha un errore. Vedi l'elenco dei problemi.",

  "storymaker.inspector.title": "Impostazioni della scena",
  "storymaker.inspector.environment": "Ambiente",
  "storymaker.inspector.variant": "Momento del giorno",
  "storymaker.inspector.variant_day": "Giorno",
  "storymaker.inspector.variant_night": "Notte",
  "storymaker.inspector.narration": "Narrazione",
  "storymaker.inspector.narration_placeholder": "Che cosa dice il narratore in questa scena?",
  "storymaker.inspector.narration_hint": "La scena dura quanto serve per leggere il testo, a meno che tu non imposti una durata.",
  "storymaker.inspector.transition": "Transizione verso questa scena",
  "storymaker.inspector.transition_first": "La prima scena non ha transizione.",
  "storymaker.inspector.unsupported": "{name} (non ancora supportato)",
  "storymaker.inspector.duration": "Durata",
  "storymaker.inspector.duration_auto": "Automatica",
  "storymaker.inspector.duration_fixed": "Fissa",
  "storymaker.inspector.duration_seconds": "Durata in secondi",
  "storymaker.inspector.seconds_unit": "secondi",
  "storymaker.inspector.cast_props": "Personaggi e oggetti di scena in questa scena",
  "storymaker.inspector.no_items": "Ancora niente. Scegli personaggi e oggetti nella libreria.",
  "storymaker.inspector.remove": "Rimuovi",
  "storymaker.inspector.remove_item": "Rimuovi {name} dalla scena",
  "storymaker.inspector.move_up": "Sposta la scena in alto",
  "storymaker.inspector.move_down": "Sposta la scena in basso",
  "storymaker.inspector.delete_scene": "Elimina scena",
  "storymaker.inspector.delete_scene_text": "Eliminare la scena {scene}? La sua narrazione e tutto il suo contenuto verranno rimossi dalla storia.",
  "storymaker.inspector.code_scene_note": "Questa scena è disegnata da un piccolo programma.",
  "storymaker.inspector.edit_code": "Modifica codice",

  "storymaker.library.title": "Libreria",
  "storymaker.library.people": "Personaggi",
  "storymaker.library.props": "Oggetti di scena",
  "storymaker.library.environments": "Luoghi",
  "storymaker.library.search": "Cerca nella libreria",
  "storymaker.library.add": "Aggiungi alla scena",
  "storymaker.library.use": "Usa come ambiente",
  "storymaker.library.empty": "Nessun risultato.",
  "storymaker.library.code_scene": "Le scene di codice disegnano tutto da sole.",
  "storymaker.library.already_in_scene": "Già in questa scena.",
  "storymaker.library.added_actor": "Aggiunto alla scena: {name}.",
  "storymaker.library.added_prop": "Aggiunto alla scena: {name}.",

  "storymaker.issues.title": "Problemi",
  "storymaker.issues.none": "Nessun problema.",
  "storymaker.issues.count": "Problemi: {count}",
  "storymaker.issues.show": "Mostra i problemi",
  "storymaker.issues.show_in_code": "Mostra in story.json",

  "storymaker.issue.invalid_json": "JSON non valido",
  "storymaker.issue.unknown_field": "Campo sconosciuto",
  "storymaker.issue.schema_mismatch": "Formato della storia errato",
  "storymaker.issue.too_large": "Troppo grande",
  "storymaker.issue.limit_exceeded": "Limite superato",
  "storymaker.issue.missing_field": "Campo mancante",
  "storymaker.issue.invalid_value": "Valore non valido",
  "storymaker.issue.duplicate_id": "Nome duplicato",
  "storymaker.issue.unknown_name": "Nome sconosciuto",
  "storymaker.issue.unknown_asset": "Elemento di catalogo sconosciuto",
  "storymaker.issue.unknown_spot": "Posizione sconosciuta",
  "storymaker.issue.unknown_cast": "Personaggio sconosciuto",
  "storymaker.issue.unknown_cue": "Segnale sconosciuto",
  "storymaker.issue.canonicalised": "Nome normalizzato",
  "storymaker.issue.not_supported_yet": "Non ancora supportato",
  "storymaker.issue.narration_missing": "Nessun audio della narrazione",
  "storymaker.issue.code_error": "Errore nella scena di codice",
  "storymaker.issue.deadline": "Ha richiesto troppo tempo",
  "storymaker.issue.layout_unverified": "Layout non verificato",
  "storymaker.issue.unknown": "Problema",

  "storymaker.code.title": "Codice",
  "storymaker.code.files": "File",
  "storymaker.code.editor_label": "Editor di codice",
  "storymaker.code.save": "Salva",
  "storymaker.code.saved": "Salvato",
  "storymaker.code.unsaved": "Modifiche non salvate",
  "storymaker.code.stale": "Modificato. Salva per ricontrollare.",
  "storymaker.code.new_file": "Nuovo file, non ancora salvato",
  "storymaker.code.reload": "Ricarica",
  "storymaker.code.bundle_ok": "Scena di codice salvata.",
  "storymaker.code.bundle_failed": "Il codice contiene errori.",
  "storymaker.code.no_problems": "Nessun problema trovato.",
  "storymaker.code.problem_count": "Problemi: {count}",
  "storymaker.code.line_col": "Riga {line}, colonna {column}",
  "storymaker.code.more": "e altri {count}",

  "storymaker.export.button": "Esporta",
  "storymaker.export.button_running": "Esportazione {percent} %",
  "storymaker.export.title": "Esporta il video",
  "storymaker.export.cap_ffmpeg": "FFmpeg",
  "storymaker.export.cap_browser": "Browser",
  "storymaker.export.ready": "Pronto",
  "storymaker.export.ready_version": "Pronto, versione {version}",
  "storymaker.export.ffmpeg_missing": "Non trovato. Installa FFmpeg o indica il suo percorso nelle impostazioni di Video Studio.",
  "storymaker.export.browser_ready": "Pronto ({source})",
  "storymaker.export.browser_missing": "Nessun browser disponibile per disegnare i fotogrammi.",
  "storymaker.export.source_config": "dalla configurazione",
  "storymaker.export.source_system": "installato su questo sistema",
  "storymaker.export.source_pinned": "scaricato da AuraGo",
  "storymaker.export.source_unknown": "origine sconosciuta",
  "storymaker.export.still_works": "Modifica e anteprima funzionano anche senza.",
  "storymaker.export.blocked_readonly": "Il desktop è in sola lettura.",
  "storymaker.export.blocked_ffmpeg": "Manca FFmpeg.",
  "storymaker.export.blocked_browser": "Nessun browser disponibile.",
  "storymaker.export.blocked_errors": "La storia contiene errori. Correggili prima.",
  "storymaker.export.blocked_too_long": "Il video supera il limite di {max} secondi.",
  "storymaker.export.blocked_not_ready": "L'anteprima è ancora in preparazione.",
  "storymaker.export.summary": "Durata {duration}, {format}, {width} × {height} pixel.",
  "storymaker.export.silent_note": "Questa versione esporta video senza audio. L'audio della narrazione arriverà più avanti.",
  "storymaker.export.start": "Avvia esportazione",
  "storymaker.export.starting": "Avvio…",
  "storymaker.export.queued": "In attesa del proprio turno…",
  "storymaker.export.phase_starting": "Avvio",
  "storymaker.export.phase_compiling": "Preparazione della storia",
  "storymaker.export.phase_mixing": "Mixaggio dell'audio",
  "storymaker.export.phase_rendering": "Disegno dei fotogrammi",
  "storymaker.export.phase_encoding": "Codifica del video",
  "storymaker.export.phase_writing": "Salvataggio del file",
  "storymaker.export.frames": "{done} di {total} fotogrammi",
  "storymaker.export.cancel": "Annulla esportazione",
  "storymaker.export.cancelling": "Annullamento…",
  "storymaker.export.cancelled": "L'esportazione è stata annullata.",
  "storymaker.export.interrupted": "L'esportazione è stata interrotta da un riavvio.",
  "storymaker.export.done": "Esportazione completata",
  "storymaker.export.output": "Il video si trova nella cartella del tuo progetto:",
  "storymaker.export.show_in_files": "Mostra in File",
  "storymaker.export.copy_path": "Copia percorso",
  "storymaker.export.copied": "Copiato",
  "storymaker.export.copy_failed": "Non è stato possibile copiare il percorso.",
  "storymaker.export.again": "Esporta di nuovo",

  "storymaker.job_error.invalid_story": "La storia contiene errori, quindi non può essere esportata.",
  "storymaker.job_error.storymaker_disabled": "Storymaker è disattivato.",
  "storymaker.job_error.browser_unavailable": "Non è stato possibile avviare un browser per disegnare i fotogrammi.",
  "storymaker.job_error.ffmpeg_unavailable": "FFmpeg non è disponibile.",
  "storymaker.job_error.render_failed": "L'esportazione non è riuscita.",
  "storymaker.job_error.render_timeout": "L'esportazione ha richiesto troppo tempo ed è stata interrotta.",
  "storymaker.job_error.cancelled": "L'esportazione è stata annullata.",
  "storymaker.job_error.busy": "È in corso un'altra esportazione. Riprova tra un momento.",
  "storymaker.job_error.code_error": "Una scena di codice non è riuscita durante il disegno.",
  "storymaker.job_error.unknown": "L'esportazione non è riuscita.",

  "storymaker.error.unauthorized": "La sessione è terminata. Accedi di nuovo.",
  "storymaker.error.scope": "Questo accesso non è autorizzato a farlo.",
  "storymaker.error.readonly": "Il desktop è in sola lettura.",
  "storymaker.error.disabled": "Storymaker è disattivato.",
  "storymaker.error.invalid_story": "La storia non è stata salvata perché contiene errori.",
  "storymaker.error.conflict": "La storia è stata modificata altrove.",
  "storymaker.error.required": "Il server ha bisogno della versione attuale della storia. Ricaricala.",
  "storymaker.error.too_large": "È troppo grande.",
  "storymaker.error.not_found": "Non esiste più.",
  "storymaker.error.busy": "Il server è occupato. Riprova tra un momento.",
  "storymaker.error.network": "Impossibile raggiungere il server.",
  "storymaker.error.generic": "Qualcosa è andato storto."
}
```

`ui/lang/storymaker/ja.json`:

```json
{
  "storymaker.title": "Storymaker",
  "storymaker.loading": "読み込み中…",
  "storymaker.retry": "もう一度試す",
  "storymaker.close": "閉じる",
  "storymaker.delete": "削除",
  "storymaker.back_to_projects": "プロジェクト",
  "storymaker.preview": "Storymaker のプレビュー",
  "storymaker.play": "再生",
  "storymaker.pause": "一時停止",
  "storymaker.timeline": "タイムライン",
  "storymaker.tab_stage": "絵コンテ",
  "storymaker.tab_code": "コード",
  "storymaker.side_scene": "シーン",
  "storymaker.side_library": "ライブラリ",
  "storymaker.save_saved": "保存済み",
  "storymaker.save_dirty": "未保存の変更",
  "storymaker.save_saving": "保存中…",
  "storymaker.save_error": "保存されていません",
  "storymaker.save_offline": "オフラインのため未保存",
  "storymaker.save_conflict": "別の場所で変更されました",
  "storymaker.save_readonly": "読み取り専用",
  "storymaker.leave_title": "未保存の変更を残したまま離れますか？",
  "storymaker.leave_text": "ストーリーへの最新の変更を保存できませんでした。今離れると失われます。",
  "storymaker.leave_code_text": "コードファイルに未保存の変更があります。今離れると失われます。",
  "storymaker.aspect_16_9": "横長 16:9",
  "storymaker.aspect_9_16": "縦長 9:16",
  "storymaker.aspect_1_1": "正方形 1:1",

  "storymaker.banner.readonly": "デスクトップは読み取り専用です。プレビューは見られますが、編集・保存・書き出しはオフになっています。",
  "storymaker.banner.conflict": "このストーリーは別の場所（別のウィンドウなど）で変更されました。再読み込みして最新版を見るか、自分の版を残して上書きしてください。",
  "storymaker.banner.conflict_reload": "再読み込み",
  "storymaker.banner.conflict_overwrite": "自分の版を残す",
  "storymaker.banner.player_unavailable": "このブラウザではプレビューを開始できませんでした。ストーリーは保存されており、編集は続けられます。",
  "storymaker.banner.export_unavailable": "書き出しはまだ使えません（{missing} がありません）。編集とプレビューは使えます。",

  "storymaker.state.disabled_title": "Storymaker はオフになっています",
  "storymaker.state.disabled_text": "管理者が設定の「仮想デスクトップ」でオンにできます。",
  "storymaker.state.unauthorized_title": "もう一度サインインしてください",
  "storymaker.state.unauthorized_text": "セッションが終了しました。サインインし直して、Storymaker をもう一度開いてください。",
  "storymaker.state.scope_title": "アクセスできません",
  "storymaker.state.scope_text": "このサインインには仮想デスクトップを使う権限がありません。",
  "storymaker.state.load_failed_title": "Storymaker を起動できませんでした",
  "storymaker.state.load_failed_text": "サーバーが想定どおりに応答しませんでした。接続を確認してもう一度お試しください。",

  "storymaker.projects.title": "プロジェクト",
  "storymaker.projects.intro": "テーマを短い解説動画にしましょう。シーンを組み立て、ナレーションを書き、MP4 に書き出します。",
  "storymaker.projects.new_title_label": "タイトル",
  "storymaker.projects.new_title_placeholder": "ファイアウォールとは？",
  "storymaker.projects.new_format": "フォーマット",
  "storymaker.projects.create": "新しいプロジェクト",
  "storymaker.projects.title_required": "タイトルを入力してください。",
  "storymaker.projects.empty_title": "プロジェクトはまだありません",
  "storymaker.projects.empty_text": "上で最初のプロジェクトを作成してください。空のシーンが 1 つある状態から始まります。",
  "storymaker.projects.delete_label": "プロジェクト {title} を削除",
  "storymaker.projects.delete_title": "プロジェクトを削除しますか？",
  "storymaker.projects.delete_text": "プロジェクト「{title}」と、書き出し結果を含むすべてのファイルが削除されます。元に戻すことはできません。",
  "storymaker.projects.deleted": "「{title}」を削除しました。",

  "storymaker.strip.title": "シーン",
  "storymaker.strip.scene_n": "シーン {n}",
  "storymaker.strip.duration_s": "{seconds} 秒",
  "storymaker.strip.kind_code": "コードシーン",
  "storymaker.strip.add_storyboard": "シーンを追加",
  "storymaker.strip.add_code": "コードシーンを追加",

  "storymaker.stage.compiling": "プレビューを準備しています…",
  "storymaker.stage.unavailable": "プレビューは利用できません。",
  "storymaker.stage.player_error": "プレビューで問題が報告されました。",
  "storymaker.stage.code_error": "コードシーンにエラーがあります。問題の一覧を確認してください。",

  "storymaker.inspector.title": "シーンの設定",
  "storymaker.inspector.environment": "背景",
  "storymaker.inspector.variant": "時間帯",
  "storymaker.inspector.variant_day": "昼",
  "storymaker.inspector.variant_night": "夜",
  "storymaker.inspector.narration": "ナレーション",
  "storymaker.inspector.narration_placeholder": "このシーンでナレーターは何と言いますか？",
  "storymaker.inspector.narration_hint": "長さを指定しない限り、シーンはテキストを読み上げる時間だけ続きます。",
  "storymaker.inspector.transition": "このシーンへの切り替え",
  "storymaker.inspector.transition_first": "最初のシーンには切り替えがありません。",
  "storymaker.inspector.unsupported": "{name}（未対応）",
  "storymaker.inspector.duration": "長さ",
  "storymaker.inspector.duration_auto": "自動",
  "storymaker.inspector.duration_fixed": "固定",
  "storymaker.inspector.duration_seconds": "長さ（秒）",
  "storymaker.inspector.seconds_unit": "秒",
  "storymaker.inspector.cast_props": "このシーンの人物と小道具",
  "storymaker.inspector.no_items": "まだ何もありません。ライブラリから人物や小道具を選んでください。",
  "storymaker.inspector.remove": "外す",
  "storymaker.inspector.remove_item": "{name} をシーンから外す",
  "storymaker.inspector.move_up": "シーンを上へ移動",
  "storymaker.inspector.move_down": "シーンを下へ移動",
  "storymaker.inspector.delete_scene": "シーンを削除",
  "storymaker.inspector.delete_scene_text": "シーン {scene} を削除しますか？ナレーションと中身がすべてストーリーから消えます。",
  "storymaker.inspector.code_scene_note": "このシーンは小さなプログラムで描かれます。",
  "storymaker.inspector.edit_code": "コードを編集",

  "storymaker.library.title": "ライブラリ",
  "storymaker.library.people": "人物",
  "storymaker.library.props": "小道具",
  "storymaker.library.environments": "場所",
  "storymaker.library.search": "ライブラリを検索",
  "storymaker.library.add": "シーンに追加",
  "storymaker.library.use": "背景として使う",
  "storymaker.library.empty": "一致するものがありません。",
  "storymaker.library.code_scene": "コードシーンはすべて自分で描画します。",
  "storymaker.library.already_in_scene": "すでにこのシーンにあります。",
  "storymaker.library.added_actor": "{name} をシーンに追加しました。",
  "storymaker.library.added_prop": "{name} をシーンに追加しました。",

  "storymaker.issues.title": "問題",
  "storymaker.issues.none": "問題はありません。",
  "storymaker.issues.count": "問題：{count}",
  "storymaker.issues.show": "問題を表示",
  "storymaker.issues.show_in_code": "story.json で表示",

  "storymaker.issue.invalid_json": "JSON が正しくありません",
  "storymaker.issue.unknown_field": "不明なフィールド",
  "storymaker.issue.schema_mismatch": "ストーリーの形式が違います",
  "storymaker.issue.too_large": "大きすぎます",
  "storymaker.issue.limit_exceeded": "上限を超えています",
  "storymaker.issue.missing_field": "フィールドがありません",
  "storymaker.issue.invalid_value": "値が正しくありません",
  "storymaker.issue.duplicate_id": "名前が重複しています",
  "storymaker.issue.unknown_name": "不明な名前",
  "storymaker.issue.unknown_asset": "不明なカタログ項目",
  "storymaker.issue.unknown_spot": "不明な位置",
  "storymaker.issue.unknown_cast": "不明な人物",
  "storymaker.issue.unknown_cue": "不明なキュー",
  "storymaker.issue.canonicalised": "名前を正規化しました",
  "storymaker.issue.not_supported_yet": "未対応",
  "storymaker.issue.narration_missing": "ナレーション音声がありません",
  "storymaker.issue.code_error": "コードシーンのエラー",
  "storymaker.issue.deadline": "時間がかかりすぎました",
  "storymaker.issue.layout_unverified": "レイアウトは未確認です",
  "storymaker.issue.unknown": "問題",

  "storymaker.code.title": "コード",
  "storymaker.code.files": "ファイル",
  "storymaker.code.editor_label": "コードエディタ",
  "storymaker.code.save": "保存",
  "storymaker.code.saved": "保存済み",
  "storymaker.code.unsaved": "未保存の変更",
  "storymaker.code.stale": "変更されました。保存すると再チェックします。",
  "storymaker.code.new_file": "新しいファイル（未保存）",
  "storymaker.code.reload": "再読み込み",
  "storymaker.code.bundle_ok": "コードシーンを保存しました。",
  "storymaker.code.bundle_failed": "コードにエラーがあります。",
  "storymaker.code.no_problems": "問題は見つかりませんでした。",
  "storymaker.code.problem_count": "問題：{count}",
  "storymaker.code.line_col": "{line} 行 {column} 列",
  "storymaker.code.more": "ほか {count} 件",

  "storymaker.export.button": "書き出し",
  "storymaker.export.button_running": "書き出し中 {percent} %",
  "storymaker.export.title": "動画を書き出す",
  "storymaker.export.cap_ffmpeg": "FFmpeg",
  "storymaker.export.cap_browser": "ブラウザ",
  "storymaker.export.ready": "準備完了",
  "storymaker.export.ready_version": "準備完了（バージョン {version}）",
  "storymaker.export.ffmpeg_missing": "見つかりません。FFmpeg をインストールするか、Video Studio の設定でパスを指定してください。",
  "storymaker.export.browser_ready": "準備完了（{source}）",
  "storymaker.export.browser_missing": "フレームを描画するブラウザがありません。",
  "storymaker.export.source_config": "設定から",
  "storymaker.export.source_system": "このシステムにインストール済み",
  "storymaker.export.source_pinned": "AuraGo がダウンロード",
  "storymaker.export.source_unknown": "入手元不明",
  "storymaker.export.still_works": "これらがなくても、編集とプレビューは使えます。",
  "storymaker.export.blocked_readonly": "デスクトップは読み取り専用です。",
  "storymaker.export.blocked_ffmpeg": "FFmpeg がありません。",
  "storymaker.export.blocked_browser": "利用できるブラウザがありません。",
  "storymaker.export.blocked_errors": "ストーリーにエラーがあります。先に直してください。",
  "storymaker.export.blocked_too_long": "動画が上限の {max} 秒を超えています。",
  "storymaker.export.blocked_not_ready": "プレビューをまだ準備しています。",
  "storymaker.export.summary": "長さ {duration}、{format}、{width} × {height} ピクセル。",
  "storymaker.export.silent_note": "このバージョンは音声なしの動画を書き出します。ナレーション音声は今後追加されます。",
  "storymaker.export.start": "書き出しを開始",
  "storymaker.export.starting": "開始しています…",
  "storymaker.export.queued": "順番待ちです…",
  "storymaker.export.phase_starting": "開始中",
  "storymaker.export.phase_compiling": "ストーリーを準備中",
  "storymaker.export.phase_mixing": "音声をミックス中",
  "storymaker.export.phase_rendering": "フレームを描画中",
  "storymaker.export.phase_encoding": "動画をエンコード中",
  "storymaker.export.phase_writing": "ファイルを保存中",
  "storymaker.export.frames": "{total} フレーム中 {done} フレーム",
  "storymaker.export.cancel": "書き出しをキャンセル",
  "storymaker.export.cancelling": "キャンセル中…",
  "storymaker.export.cancelled": "書き出しをキャンセルしました。",
  "storymaker.export.interrupted": "再起動により書き出しが中断されました。",
  "storymaker.export.done": "書き出しが完了しました",
  "storymaker.export.output": "動画はプロジェクトフォルダにあります：",
  "storymaker.export.show_in_files": "ファイルで表示",
  "storymaker.export.copy_path": "パスをコピー",
  "storymaker.export.copied": "コピーしました",
  "storymaker.export.copy_failed": "パスをコピーできませんでした。",
  "storymaker.export.again": "もう一度書き出す",

  "storymaker.job_error.invalid_story": "ストーリーにエラーがあるため、書き出せません。",
  "storymaker.job_error.storymaker_disabled": "Storymaker はオフになっています。",
  "storymaker.job_error.browser_unavailable": "フレームを描画するブラウザを起動できませんでした。",
  "storymaker.job_error.ffmpeg_unavailable": "FFmpeg が利用できません。",
  "storymaker.job_error.render_failed": "書き出しに失敗しました。",
  "storymaker.job_error.render_timeout": "書き出しに時間がかかりすぎたため停止しました。",
  "storymaker.job_error.cancelled": "書き出しをキャンセルしました。",
  "storymaker.job_error.busy": "別の書き出しが実行中です。しばらくしてからもう一度お試しください。",
  "storymaker.job_error.code_error": "描画中にコードシーンが失敗しました。",
  "storymaker.job_error.unknown": "書き出しに失敗しました。",

  "storymaker.error.unauthorized": "セッションが終了しました。サインインし直してください。",
  "storymaker.error.scope": "このサインインにはその操作の権限がありません。",
  "storymaker.error.readonly": "デスクトップは読み取り専用です。",
  "storymaker.error.disabled": "Storymaker はオフになっています。",
  "storymaker.error.invalid_story": "エラーがあるため、ストーリーは保存されませんでした。",
  "storymaker.error.conflict": "ストーリーが別の場所で変更されました。",
  "storymaker.error.required": "サーバーにはストーリーの最新版が必要です。再読み込みしてください。",
  "storymaker.error.too_large": "大きすぎます。",
  "storymaker.error.not_found": "それはもう存在しません。",
  "storymaker.error.busy": "サーバーがビジー状態です。しばらくしてからもう一度お試しください。",
  "storymaker.error.network": "サーバーに接続できませんでした。",
  "storymaker.error.generic": "問題が発生しました。"
}
```

`ui/lang/storymaker/nl.json`:

```json
{
  "storymaker.title": "Storymaker",
  "storymaker.loading": "Laden…",
  "storymaker.retry": "Opnieuw proberen",
  "storymaker.close": "Sluiten",
  "storymaker.delete": "Verwijderen",
  "storymaker.back_to_projects": "Projecten",
  "storymaker.preview": "Storymaker-voorbeeld",
  "storymaker.play": "Afspelen",
  "storymaker.pause": "Pauze",
  "storymaker.timeline": "Tijdlijn",
  "storymaker.tab_stage": "Storyboard",
  "storymaker.tab_code": "Code",
  "storymaker.side_scene": "Scène",
  "storymaker.side_library": "Bibliotheek",
  "storymaker.save_saved": "Opgeslagen",
  "storymaker.save_dirty": "Niet-opgeslagen wijzigingen",
  "storymaker.save_saving": "Opslaan…",
  "storymaker.save_error": "Niet opgeslagen",
  "storymaker.save_offline": "Offline, niet opgeslagen",
  "storymaker.save_conflict": "Elders gewijzigd",
  "storymaker.save_readonly": "Alleen-lezen",
  "storymaker.leave_title": "Verlaten met niet-opgeslagen wijzigingen?",
  "storymaker.leave_text": "Je laatste wijzigingen aan het verhaal konden niet worden opgeslagen. Als je nu weggaat, zijn ze kwijt.",
  "storymaker.leave_code_text": "Een codebestand heeft niet-opgeslagen wijzigingen. Als je nu weggaat, zijn ze kwijt.",
  "storymaker.aspect_16_9": "Liggend 16:9",
  "storymaker.aspect_9_16": "Staand 9:16",
  "storymaker.aspect_1_1": "Vierkant 1:1",

  "storymaker.banner.readonly": "Het bureaublad is alleen-lezen. Je kunt het voorbeeld bekijken, maar bewerken, opslaan en exporteren staan uit.",
  "storymaker.banner.conflict": "Dit verhaal is elders gewijzigd, bijvoorbeeld in een ander venster. Laad opnieuw om de huidige versie te zien, of behoud de jouwe en overschrijf.",
  "storymaker.banner.conflict_reload": "Opnieuw laden",
  "storymaker.banner.conflict_overwrite": "Mijn versie behouden",
  "storymaker.banner.player_unavailable": "Het voorbeeld kon in deze browser niet starten. Je verhaal is nog steeds opgeslagen en je kunt het blijven bewerken.",
  "storymaker.banner.export_unavailable": "Exporteren is nog niet beschikbaar ({missing} ontbreekt). Bewerken en het voorbeeld werken wel.",

  "storymaker.state.disabled_title": "Storymaker staat uit",
  "storymaker.state.disabled_text": "Een beheerder kan het inschakelen in de configuratie onder Virtueel bureaublad.",
  "storymaker.state.unauthorized_title": "Log opnieuw in",
  "storymaker.state.unauthorized_text": "Je sessie is afgelopen. Log opnieuw in en open Storymaker opnieuw.",
  "storymaker.state.scope_title": "Geen toegang",
  "storymaker.state.scope_text": "Deze aanmelding mag het Virtuele bureaublad niet gebruiken.",
  "storymaker.state.load_failed_title": "Storymaker kon niet starten",
  "storymaker.state.load_failed_text": "De server antwoordde niet zoals verwacht. Controleer de verbinding en probeer het opnieuw.",

  "storymaker.projects.title": "Projecten",
  "storymaker.projects.intro": "Maak van een onderwerp een korte uitlegvideo: zet de scènes in beeld, schrijf de voice-over en exporteer een MP4.",
  "storymaker.projects.new_title_label": "Titel",
  "storymaker.projects.new_title_placeholder": "Wat is een firewall?",
  "storymaker.projects.new_format": "Formaat",
  "storymaker.projects.create": "Nieuw project",
  "storymaker.projects.title_required": "Vul een titel in.",
  "storymaker.projects.empty_title": "Nog geen projecten",
  "storymaker.projects.empty_text": "Maak hierboven je eerste project. Het begint met één lege scène.",
  "storymaker.projects.delete_label": "Project {title} verwijderen",
  "storymaker.projects.delete_title": "Project verwijderen?",
  "storymaker.projects.delete_text": "Het project \"{title}\" en al zijn bestanden, ook de exports, worden verwijderd. Dit kan niet ongedaan worden gemaakt.",
  "storymaker.projects.deleted": "\"{title}\" is verwijderd.",

  "storymaker.strip.title": "Scènes",
  "storymaker.strip.scene_n": "Scène {n}",
  "storymaker.strip.duration_s": "{seconds} s",
  "storymaker.strip.kind_code": "Codescène",
  "storymaker.strip.add_storyboard": "Scène toevoegen",
  "storymaker.strip.add_code": "Codescène toevoegen",

  "storymaker.stage.compiling": "Voorbeeld voorbereiden…",
  "storymaker.stage.unavailable": "Het voorbeeld is niet beschikbaar.",
  "storymaker.stage.player_error": "Het voorbeeld meldde een probleem.",
  "storymaker.stage.code_error": "Een codescène bevat een fout. Zie de lijst met problemen.",

  "storymaker.inspector.title": "Scène-instellingen",
  "storymaker.inspector.environment": "Omgeving",
  "storymaker.inspector.variant": "Tijdstip van de dag",
  "storymaker.inspector.variant_day": "Dag",
  "storymaker.inspector.variant_night": "Nacht",
  "storymaker.inspector.narration": "Voice-over",
  "storymaker.inspector.narration_placeholder": "Wat zegt de verteller in deze scène?",
  "storymaker.inspector.narration_hint": "De scène duurt zo lang als het lezen van de tekst kost, tenzij je een duur instelt.",
  "storymaker.inspector.transition": "Overgang naar deze scène",
  "storymaker.inspector.transition_first": "De eerste scène heeft geen overgang.",
  "storymaker.inspector.unsupported": "{name} (nog niet ondersteund)",
  "storymaker.inspector.duration": "Duur",
  "storymaker.inspector.duration_auto": "Automatisch",
  "storymaker.inspector.duration_fixed": "Vast",
  "storymaker.inspector.duration_seconds": "Duur in seconden",
  "storymaker.inspector.seconds_unit": "seconden",
  "storymaker.inspector.cast_props": "Personages en rekwisieten in deze scène",
  "storymaker.inspector.no_items": "Nog niets. Kies personages en rekwisieten in de bibliotheek.",
  "storymaker.inspector.remove": "Verwijderen",
  "storymaker.inspector.remove_item": "{name} uit de scène verwijderen",
  "storymaker.inspector.move_up": "Scène omhoog verplaatsen",
  "storymaker.inspector.move_down": "Scène omlaag verplaatsen",
  "storymaker.inspector.delete_scene": "Scène verwijderen",
  "storymaker.inspector.delete_scene_text": "Scène {scene} verwijderen? De voice-over en alles erin worden uit het verhaal gehaald.",
  "storymaker.inspector.code_scene_note": "Deze scène wordt getekend door een klein programma.",
  "storymaker.inspector.edit_code": "Code bewerken",

  "storymaker.library.title": "Bibliotheek",
  "storymaker.library.people": "Personages",
  "storymaker.library.props": "Rekwisieten",
  "storymaker.library.environments": "Plaatsen",
  "storymaker.library.search": "Zoeken in de bibliotheek",
  "storymaker.library.add": "Aan scène toevoegen",
  "storymaker.library.use": "Als omgeving gebruiken",
  "storymaker.library.empty": "Niets gevonden.",
  "storymaker.library.code_scene": "Codescènes tekenen alles zelf.",
  "storymaker.library.already_in_scene": "Staat al in deze scène.",
  "storymaker.library.added_actor": "{name} is bij de scène gekomen.",
  "storymaker.library.added_prop": "{name} aan de scène toegevoegd.",

  "storymaker.issues.title": "Problemen",
  "storymaker.issues.none": "Geen problemen.",
  "storymaker.issues.count": "Problemen: {count}",
  "storymaker.issues.show": "Problemen tonen",
  "storymaker.issues.show_in_code": "Tonen in story.json",

  "storymaker.issue.invalid_json": "Ongeldige JSON",
  "storymaker.issue.unknown_field": "Onbekend veld",
  "storymaker.issue.schema_mismatch": "Verkeerd verhaalformaat",
  "storymaker.issue.too_large": "Te groot",
  "storymaker.issue.limit_exceeded": "Limiet overschreden",
  "storymaker.issue.missing_field": "Ontbrekend veld",
  "storymaker.issue.invalid_value": "Ongeldige waarde",
  "storymaker.issue.duplicate_id": "Dubbele naam",
  "storymaker.issue.unknown_name": "Onbekende naam",
  "storymaker.issue.unknown_asset": "Onbekend catalogusitem",
  "storymaker.issue.unknown_spot": "Onbekende plek",
  "storymaker.issue.unknown_cast": "Onbekend personage",
  "storymaker.issue.unknown_cue": "Onbekende markering",
  "storymaker.issue.canonicalised": "Naam genormaliseerd",
  "storymaker.issue.not_supported_yet": "Nog niet ondersteund",
  "storymaker.issue.narration_missing": "Geen audio voor de voice-over",
  "storymaker.issue.code_error": "Fout in codescène",
  "storymaker.issue.deadline": "Duurde te lang",
  "storymaker.issue.layout_unverified": "Opmaak niet gecontroleerd",
  "storymaker.issue.unknown": "Probleem",

  "storymaker.code.title": "Code",
  "storymaker.code.files": "Bestanden",
  "storymaker.code.editor_label": "Code-editor",
  "storymaker.code.save": "Opslaan",
  "storymaker.code.saved": "Opgeslagen",
  "storymaker.code.unsaved": "Niet-opgeslagen wijzigingen",
  "storymaker.code.stale": "Gewijzigd. Sla op om opnieuw te controleren.",
  "storymaker.code.new_file": "Nieuw bestand, nog niet opgeslagen",
  "storymaker.code.reload": "Opnieuw laden",
  "storymaker.code.bundle_ok": "Codescène opgeslagen.",
  "storymaker.code.bundle_failed": "De code bevat fouten.",
  "storymaker.code.no_problems": "Geen problemen gevonden.",
  "storymaker.code.problem_count": "Problemen: {count}",
  "storymaker.code.line_col": "Regel {line}, kolom {column}",
  "storymaker.code.more": "en nog {count}",

  "storymaker.export.button": "Exporteren",
  "storymaker.export.button_running": "Exporteren {percent} %",
  "storymaker.export.title": "Video exporteren",
  "storymaker.export.cap_ffmpeg": "FFmpeg",
  "storymaker.export.cap_browser": "Browser",
  "storymaker.export.ready": "Gereed",
  "storymaker.export.ready_version": "Gereed, versie {version}",
  "storymaker.export.ffmpeg_missing": "Niet gevonden. Installeer FFmpeg of stel het pad in bij de Video Studio-instellingen.",
  "storymaker.export.browser_ready": "Gereed ({source})",
  "storymaker.export.browser_missing": "Er is geen browser beschikbaar om de beelden te tekenen.",
  "storymaker.export.source_config": "uit de configuratie",
  "storymaker.export.source_system": "op dit systeem geïnstalleerd",
  "storymaker.export.source_pinned": "gedownload door AuraGo",
  "storymaker.export.source_unknown": "bron onbekend",
  "storymaker.export.still_works": "Bewerken en het voorbeeld werken ook zonder.",
  "storymaker.export.blocked_readonly": "Het bureaublad is alleen-lezen.",
  "storymaker.export.blocked_ffmpeg": "FFmpeg ontbreekt.",
  "storymaker.export.blocked_browser": "Er is geen browser beschikbaar.",
  "storymaker.export.blocked_errors": "Het verhaal bevat fouten. Los die eerst op.",
  "storymaker.export.blocked_too_long": "De video is langer dan de limiet van {max} seconden.",
  "storymaker.export.blocked_not_ready": "Het voorbeeld wordt nog voorbereid.",
  "storymaker.export.summary": "Duur {duration}, {format}, {width} × {height} pixels.",
  "storymaker.export.silent_note": "Deze versie exporteert video's zonder geluid. De audio voor de voice-over komt later.",
  "storymaker.export.start": "Export starten",
  "storymaker.export.starting": "Starten…",
  "storymaker.export.queued": "Wacht op zijn beurt…",
  "storymaker.export.phase_starting": "Starten",
  "storymaker.export.phase_compiling": "Verhaal voorbereiden",
  "storymaker.export.phase_mixing": "Geluid mixen",
  "storymaker.export.phase_rendering": "Beelden tekenen",
  "storymaker.export.phase_encoding": "Video coderen",
  "storymaker.export.phase_writing": "Bestand opslaan",
  "storymaker.export.frames": "{done} van {total} beelden",
  "storymaker.export.cancel": "Export annuleren",
  "storymaker.export.cancelling": "Annuleren…",
  "storymaker.export.cancelled": "De export is geannuleerd.",
  "storymaker.export.interrupted": "De export is onderbroken door een herstart.",
  "storymaker.export.done": "Export voltooid",
  "storymaker.export.output": "De video staat in je projectmap:",
  "storymaker.export.show_in_files": "Tonen in Bestanden",
  "storymaker.export.copy_path": "Pad kopiëren",
  "storymaker.export.copied": "Gekopieerd",
  "storymaker.export.copy_failed": "Het pad kon niet worden gekopieerd.",
  "storymaker.export.again": "Opnieuw exporteren",

  "storymaker.job_error.invalid_story": "Het verhaal bevat fouten en kan daarom niet worden geëxporteerd.",
  "storymaker.job_error.storymaker_disabled": "Storymaker staat uit.",
  "storymaker.job_error.browser_unavailable": "Er kon geen browser worden gestart om de beelden te tekenen.",
  "storymaker.job_error.ffmpeg_unavailable": "FFmpeg is niet beschikbaar.",
  "storymaker.job_error.render_failed": "De export is mislukt.",
  "storymaker.job_error.render_timeout": "De export duurde te lang en is gestopt.",
  "storymaker.job_error.cancelled": "De export is geannuleerd.",
  "storymaker.job_error.busy": "Er loopt al een andere export. Probeer het zo opnieuw.",
  "storymaker.job_error.code_error": "Een codescène is mislukt tijdens het tekenen.",
  "storymaker.job_error.unknown": "De export is mislukt.",

  "storymaker.error.unauthorized": "Je sessie is afgelopen. Log opnieuw in.",
  "storymaker.error.scope": "Deze aanmelding mag dat niet doen.",
  "storymaker.error.readonly": "Het bureaublad is alleen-lezen.",
  "storymaker.error.disabled": "Storymaker staat uit.",
  "storymaker.error.invalid_story": "Het verhaal is niet opgeslagen omdat het fouten bevat.",
  "storymaker.error.conflict": "Het verhaal is elders gewijzigd.",
  "storymaker.error.required": "De server heeft de huidige versie van het verhaal nodig. Laad het opnieuw.",
  "storymaker.error.too_large": "Dat is te groot.",
  "storymaker.error.not_found": "Dat bestaat niet meer.",
  "storymaker.error.busy": "De server is bezig. Probeer het zo opnieuw.",
  "storymaker.error.network": "De server is niet bereikbaar.",
  "storymaker.error.generic": "Er ging iets mis."
}
```

`ui/lang/storymaker/no.json`:

```json
{
  "storymaker.title": "Storymaker",
  "storymaker.loading": "Laster …",
  "storymaker.retry": "Prøv igjen",
  "storymaker.close": "Lukk",
  "storymaker.delete": "Slett",
  "storymaker.back_to_projects": "Prosjekter",
  "storymaker.preview": "Forhåndsvisning i Storymaker",
  "storymaker.play": "Spill av",
  "storymaker.pause": "Pause",
  "storymaker.timeline": "Tidslinje",
  "storymaker.tab_stage": "Storyboard",
  "storymaker.tab_code": "Kode",
  "storymaker.side_scene": "Scene",
  "storymaker.side_library": "Bibliotek",
  "storymaker.save_saved": "Lagret",
  "storymaker.save_dirty": "Ulagrede endringer",
  "storymaker.save_saving": "Lagrer …",
  "storymaker.save_error": "Ikke lagret",
  "storymaker.save_offline": "Frakoblet, ikke lagret",
  "storymaker.save_conflict": "Endret et annet sted",
  "storymaker.save_readonly": "Skrivebeskyttet",
  "storymaker.leave_title": "Forlate med ulagrede endringer?",
  "storymaker.leave_text": "De siste endringene dine i historien kunne ikke lagres. Hvis du forlater nå, går de tapt.",
  "storymaker.leave_code_text": "En kodefil har ulagrede endringer. Hvis du forlater nå, går de tapt.",
  "storymaker.aspect_16_9": "Liggende 16:9",
  "storymaker.aspect_9_16": "Stående 9:16",
  "storymaker.aspect_1_1": "Kvadrat 1:1",

  "storymaker.banner.readonly": "Skrivebordet er skrivebeskyttet. Du kan se forhåndsvisningen, men redigering, lagring og eksport er slått av.",
  "storymaker.banner.conflict": "Denne historien ble endret et annet sted, for eksempel i et annet vindu. Last inn på nytt for å se den nåværende versjonen, eller behold din og skriv over.",
  "storymaker.banner.conflict_reload": "Last inn på nytt",
  "storymaker.banner.conflict_overwrite": "Behold min",
  "storymaker.banner.player_unavailable": "Forhåndsvisningen kunne ikke starte i denne nettleseren. Historien din er fortsatt lagret, og du kan fortsette å redigere den.",
  "storymaker.banner.export_unavailable": "Eksport er ikke tilgjengelig ennå ({missing} mangler). Redigering og forhåndsvisning fungerer likevel.",

  "storymaker.state.disabled_title": "Storymaker er slått av",
  "storymaker.state.disabled_text": "En administrator kan slå det på i konfigurasjonen under Virtuelt skrivebord.",
  "storymaker.state.unauthorized_title": "Logg inn igjen",
  "storymaker.state.unauthorized_text": "Økten din er avsluttet. Logg inn igjen og åpne Storymaker på nytt.",
  "storymaker.state.scope_title": "Ingen tilgang",
  "storymaker.state.scope_text": "Denne påloggingen har ikke lov til å bruke det virtuelle skrivebordet.",
  "storymaker.state.load_failed_title": "Storymaker kunne ikke starte",
  "storymaker.state.load_failed_text": "Serveren svarte ikke som forventet. Sjekk tilkoblingen og prøv igjen.",

  "storymaker.projects.title": "Prosjekter",
  "storymaker.projects.intro": "Gjør et tema om til en kort forklaringsvideo: sett opp scenene, skriv fortellerteksten og eksporter en MP4.",
  "storymaker.projects.new_title_label": "Tittel",
  "storymaker.projects.new_title_placeholder": "Hva er en brannmur?",
  "storymaker.projects.new_format": "Format",
  "storymaker.projects.create": "Nytt prosjekt",
  "storymaker.projects.title_required": "Skriv inn en tittel.",
  "storymaker.projects.empty_title": "Ingen prosjekter ennå",
  "storymaker.projects.empty_text": "Lag ditt første prosjekt ovenfor. Det starter med én tom scene.",
  "storymaker.projects.delete_label": "Slett prosjektet {title}",
  "storymaker.projects.delete_title": "Slette prosjektet?",
  "storymaker.projects.delete_text": "Prosjektet «{title}» og alle filene, også eksportene, blir slettet. Dette kan ikke angres.",
  "storymaker.projects.deleted": "«{title}» er slettet.",

  "storymaker.strip.title": "Scener",
  "storymaker.strip.scene_n": "Scene {n}",
  "storymaker.strip.duration_s": "{seconds} s",
  "storymaker.strip.kind_code": "Kodescene",
  "storymaker.strip.add_storyboard": "Legg til scene",
  "storymaker.strip.add_code": "Legg til kodescene",

  "storymaker.stage.compiling": "Forbereder forhåndsvisningen …",
  "storymaker.stage.unavailable": "Forhåndsvisningen er ikke tilgjengelig.",
  "storymaker.stage.player_error": "Forhåndsvisningen meldte om et problem.",
  "storymaker.stage.code_error": "En kodescene har en feil. Se listen over problemer.",

  "storymaker.inspector.title": "Sceneinnstillinger",
  "storymaker.inspector.environment": "Miljø",
  "storymaker.inspector.variant": "Tid på døgnet",
  "storymaker.inspector.variant_day": "Dag",
  "storymaker.inspector.variant_night": "Natt",
  "storymaker.inspector.narration": "Fortellertekst",
  "storymaker.inspector.narration_placeholder": "Hva sier fortelleren i denne scenen?",
  "storymaker.inspector.narration_hint": "Scenen varer like lenge som teksten tar å lese, med mindre du angir en varighet.",
  "storymaker.inspector.transition": "Overgang inn i denne scenen",
  "storymaker.inspector.transition_first": "Den første scenen har ingen overgang.",
  "storymaker.inspector.unsupported": "{name} (støttes ikke ennå)",
  "storymaker.inspector.duration": "Varighet",
  "storymaker.inspector.duration_auto": "Automatisk",
  "storymaker.inspector.duration_fixed": "Fast",
  "storymaker.inspector.duration_seconds": "Varighet i sekunder",
  "storymaker.inspector.seconds_unit": "sekunder",
  "storymaker.inspector.cast_props": "Personer og rekvisitter i denne scenen",
  "storymaker.inspector.no_items": "Ingenting ennå. Velg personer og rekvisitter i biblioteket.",
  "storymaker.inspector.remove": "Fjern",
  "storymaker.inspector.remove_item": "Fjern {name} fra scenen",
  "storymaker.inspector.move_up": "Flytt scenen opp",
  "storymaker.inspector.move_down": "Flytt scenen ned",
  "storymaker.inspector.delete_scene": "Slett scene",
  "storymaker.inspector.delete_scene_text": "Slette scenen {scene}? Fortellerteksten og alt i den fjernes fra historien.",
  "storymaker.inspector.code_scene_note": "Denne scenen tegnes av et lite program.",
  "storymaker.inspector.edit_code": "Rediger kode",

  "storymaker.library.title": "Bibliotek",
  "storymaker.library.people": "Personer",
  "storymaker.library.props": "Rekvisitter",
  "storymaker.library.environments": "Steder",
  "storymaker.library.search": "Søk i biblioteket",
  "storymaker.library.add": "Legg til i scenen",
  "storymaker.library.use": "Bruk som miljø",
  "storymaker.library.empty": "Ingenting passer.",
  "storymaker.library.code_scene": "Kodescener tegner alt selv.",
  "storymaker.library.already_in_scene": "Er allerede i denne scenen.",
  "storymaker.library.added_actor": "{name} ble med i scenen.",
  "storymaker.library.added_prop": "{name} ble lagt til i scenen.",

  "storymaker.issues.title": "Problemer",
  "storymaker.issues.none": "Ingen problemer.",
  "storymaker.issues.count": "Problemer: {count}",
  "storymaker.issues.show": "Vis problemer",
  "storymaker.issues.show_in_code": "Vis i story.json",

  "storymaker.issue.invalid_json": "Ugyldig JSON",
  "storymaker.issue.unknown_field": "Ukjent felt",
  "storymaker.issue.schema_mismatch": "Feil historieformat",
  "storymaker.issue.too_large": "For stor",
  "storymaker.issue.limit_exceeded": "Grensen er overskredet",
  "storymaker.issue.missing_field": "Felt mangler",
  "storymaker.issue.invalid_value": "Ugyldig verdi",
  "storymaker.issue.duplicate_id": "Navnet finnes to ganger",
  "storymaker.issue.unknown_name": "Ukjent navn",
  "storymaker.issue.unknown_asset": "Ukjent katalogelement",
  "storymaker.issue.unknown_spot": "Ukjent sted",
  "storymaker.issue.unknown_cast": "Ukjent person",
  "storymaker.issue.unknown_cue": "Ukjent merke",
  "storymaker.issue.canonicalised": "Navnet ble normalisert",
  "storymaker.issue.not_supported_yet": "Støttes ikke ennå",
  "storymaker.issue.narration_missing": "Ingen lyd for fortellerteksten",
  "storymaker.issue.code_error": "Feil i kodescene",
  "storymaker.issue.deadline": "Tok for lang tid",
  "storymaker.issue.layout_unverified": "Oppsettet er ikke kontrollert",
  "storymaker.issue.unknown": "Problem",

  "storymaker.code.title": "Kode",
  "storymaker.code.files": "Filer",
  "storymaker.code.editor_label": "Koderedigerer",
  "storymaker.code.save": "Lagre",
  "storymaker.code.saved": "Lagret",
  "storymaker.code.unsaved": "Ulagrede endringer",
  "storymaker.code.stale": "Endret. Lagre for å sjekke på nytt.",
  "storymaker.code.new_file": "Ny fil, ikke lagret ennå",
  "storymaker.code.reload": "Last inn på nytt",
  "storymaker.code.bundle_ok": "Kodescenen ble lagret.",
  "storymaker.code.bundle_failed": "Koden har feil.",
  "storymaker.code.no_problems": "Ingen problemer funnet.",
  "storymaker.code.problem_count": "Problemer: {count}",
  "storymaker.code.line_col": "Linje {line}, kolonne {column}",
  "storymaker.code.more": "og {count} til",

  "storymaker.export.button": "Eksporter",
  "storymaker.export.button_running": "Eksporterer {percent} %",
  "storymaker.export.title": "Eksporter video",
  "storymaker.export.cap_ffmpeg": "FFmpeg",
  "storymaker.export.cap_browser": "Nettleser",
  "storymaker.export.ready": "Klar",
  "storymaker.export.ready_version": "Klar, versjon {version}",
  "storymaker.export.ffmpeg_missing": "Ikke funnet. Installer FFmpeg eller angi stien i innstillingene for Video Studio.",
  "storymaker.export.browser_ready": "Klar ({source})",
  "storymaker.export.browser_missing": "Ingen nettleser er tilgjengelig for å tegne bildene.",
  "storymaker.export.source_config": "fra konfigurasjonen",
  "storymaker.export.source_system": "installert på dette systemet",
  "storymaker.export.source_pinned": "lastet ned av AuraGo",
  "storymaker.export.source_unknown": "ukjent kilde",
  "storymaker.export.still_works": "Redigering og forhåndsvisning fungerer også uten dem.",
  "storymaker.export.blocked_readonly": "Skrivebordet er skrivebeskyttet.",
  "storymaker.export.blocked_ffmpeg": "FFmpeg mangler.",
  "storymaker.export.blocked_browser": "Ingen nettleser er tilgjengelig.",
  "storymaker.export.blocked_errors": "Historien har feil. Rett dem først.",
  "storymaker.export.blocked_too_long": "Videoen er lengre enn grensen på {max} sekunder.",
  "storymaker.export.blocked_not_ready": "Forhåndsvisningen forberedes fortsatt.",
  "storymaker.export.summary": "Varighet {duration}, {format}, {width} × {height} piksler.",
  "storymaker.export.silent_note": "Denne versjonen eksporterer videoer uten lyd. Lyd til fortellerteksten kommer senere.",
  "storymaker.export.start": "Start eksport",
  "storymaker.export.starting": "Starter …",
  "storymaker.export.queued": "Venter på tur …",
  "storymaker.export.phase_starting": "Starter",
  "storymaker.export.phase_compiling": "Forbereder historien",
  "storymaker.export.phase_mixing": "Mikser lyden",
  "storymaker.export.phase_rendering": "Tegner bildene",
  "storymaker.export.phase_encoding": "Koder videoen",
  "storymaker.export.phase_writing": "Lagrer filen",
  "storymaker.export.frames": "{done} av {total} bilder",
  "storymaker.export.cancel": "Avbryt eksporten",
  "storymaker.export.cancelling": "Avbryter …",
  "storymaker.export.cancelled": "Eksporten ble avbrutt.",
  "storymaker.export.interrupted": "Eksporten ble avbrutt av en omstart.",
  "storymaker.export.done": "Eksporten er ferdig",
  "storymaker.export.output": "Videoen ligger i prosjektmappen din:",
  "storymaker.export.show_in_files": "Vis i Filer",
  "storymaker.export.copy_path": "Kopier sti",
  "storymaker.export.copied": "Kopiert",
  "storymaker.export.copy_failed": "Stien kunne ikke kopieres.",
  "storymaker.export.again": "Eksporter på nytt",

  "storymaker.job_error.invalid_story": "Historien har feil og kan derfor ikke eksporteres.",
  "storymaker.job_error.storymaker_disabled": "Storymaker er slått av.",
  "storymaker.job_error.browser_unavailable": "Ingen nettleser kunne startes for å tegne bildene.",
  "storymaker.job_error.ffmpeg_unavailable": "FFmpeg er ikke tilgjengelig.",
  "storymaker.job_error.render_failed": "Eksporten mislyktes.",
  "storymaker.job_error.render_timeout": "Eksporten tok for lang tid og ble stoppet.",
  "storymaker.job_error.cancelled": "Eksporten ble avbrutt.",
  "storymaker.job_error.busy": "En annen eksport pågår. Prøv igjen om et øyeblikk.",
  "storymaker.job_error.code_error": "En kodescene mislyktes under tegningen.",
  "storymaker.job_error.unknown": "Eksporten mislyktes.",

  "storymaker.error.unauthorized": "Økten din er avsluttet. Logg inn igjen.",
  "storymaker.error.scope": "Denne påloggingen har ikke lov til å gjøre det.",
  "storymaker.error.readonly": "Skrivebordet er skrivebeskyttet.",
  "storymaker.error.disabled": "Storymaker er slått av.",
  "storymaker.error.invalid_story": "Historien ble ikke lagret fordi den har feil.",
  "storymaker.error.conflict": "Historien ble endret et annet sted.",
  "storymaker.error.required": "Serveren trenger den nåværende versjonen av historien. Last den inn på nytt.",
  "storymaker.error.too_large": "Det er for stort.",
  "storymaker.error.not_found": "Det finnes ikke lenger.",
  "storymaker.error.busy": "Serveren er opptatt. Prøv igjen om et øyeblikk.",
  "storymaker.error.network": "Serveren kunne ikke nås.",
  "storymaker.error.generic": "Noe gikk galt."
}
```

`ui/lang/storymaker/pl.json`:

```json
{
  "storymaker.title": "Storymaker",
  "storymaker.loading": "Ładowanie…",
  "storymaker.retry": "Spróbuj ponownie",
  "storymaker.close": "Zamknij",
  "storymaker.delete": "Usuń",
  "storymaker.back_to_projects": "Projekty",
  "storymaker.preview": "Podgląd Storymakera",
  "storymaker.play": "Odtwórz",
  "storymaker.pause": "Pauza",
  "storymaker.timeline": "Oś czasu",
  "storymaker.tab_stage": "Storyboard",
  "storymaker.tab_code": "Kod",
  "storymaker.side_scene": "Scena",
  "storymaker.side_library": "Biblioteka",
  "storymaker.save_saved": "Zapisano",
  "storymaker.save_dirty": "Niezapisane zmiany",
  "storymaker.save_saving": "Zapisywanie…",
  "storymaker.save_error": "Nie zapisano",
  "storymaker.save_offline": "Brak połączenia, nie zapisano",
  "storymaker.save_conflict": "Zmieniono gdzie indziej",
  "storymaker.save_readonly": "Tylko do odczytu",
  "storymaker.leave_title": "Wyjść z niezapisanymi zmianami?",
  "storymaker.leave_text": "Twoich ostatnich zmian w historii nie udało się zapisać. Jeśli wyjdziesz teraz, przepadną.",
  "storymaker.leave_code_text": "Plik z kodem ma niezapisane zmiany. Jeśli wyjdziesz teraz, przepadną.",
  "storymaker.aspect_16_9": "Poziomo 16:9",
  "storymaker.aspect_9_16": "Pionowo 9:16",
  "storymaker.aspect_1_1": "Kwadrat 1:1",

  "storymaker.banner.readonly": "Pulpit jest tylko do odczytu. Możesz oglądać podgląd, ale edycja, zapisywanie i eksport są wyłączone.",
  "storymaker.banner.conflict": "Ta historia została zmieniona gdzie indziej, na przykład w innym oknie. Wczytaj ją ponownie, aby zobaczyć aktualną wersję, albo zachowaj swoją i nadpisz.",
  "storymaker.banner.conflict_reload": "Wczytaj ponownie",
  "storymaker.banner.conflict_overwrite": "Zachowaj moją",
  "storymaker.banner.player_unavailable": "Podgląd nie mógł się uruchomić w tej przeglądarce. Twoja historia jest nadal zapisana i możesz ją dalej edytować.",
  "storymaker.banner.export_unavailable": "Eksport nie jest jeszcze dostępny (brakuje: {missing}). Edycja i podgląd nadal działają.",

  "storymaker.state.disabled_title": "Storymaker jest wyłączony",
  "storymaker.state.disabled_text": "Administrator może go włączyć w konfiguracji, w sekcji Pulpit wirtualny.",
  "storymaker.state.unauthorized_title": "Zaloguj się ponownie",
  "storymaker.state.unauthorized_text": "Twoja sesja wygasła. Zaloguj się ponownie i otwórz Storymakera jeszcze raz.",
  "storymaker.state.scope_title": "Brak dostępu",
  "storymaker.state.scope_text": "To logowanie nie ma uprawnień do korzystania z Pulpitu wirtualnego.",
  "storymaker.state.load_failed_title": "Storymaker nie mógł się uruchomić",
  "storymaker.state.load_failed_text": "Serwer nie odpowiedział zgodnie z oczekiwaniami. Sprawdź połączenie i spróbuj ponownie.",

  "storymaker.projects.title": "Projekty",
  "storymaker.projects.intro": "Zamień temat w krótki film wyjaśniający: wyreżyseruj sceny, napisz tekst lektora i wyeksportuj plik MP4.",
  "storymaker.projects.new_title_label": "Tytuł",
  "storymaker.projects.new_title_placeholder": "Czym jest zapora sieciowa?",
  "storymaker.projects.new_format": "Format",
  "storymaker.projects.create": "Nowy projekt",
  "storymaker.projects.title_required": "Wpisz tytuł.",
  "storymaker.projects.empty_title": "Brak projektów",
  "storymaker.projects.empty_text": "Utwórz swój pierwszy projekt powyżej. Zaczyna się od jednej pustej sceny.",
  "storymaker.projects.delete_label": "Usuń projekt {title}",
  "storymaker.projects.delete_title": "Usunąć projekt?",
  "storymaker.projects.delete_text": "Projekt „{title}” i wszystkie jego pliki, także eksporty, zostaną usunięte. Tej operacji nie można cofnąć.",
  "storymaker.projects.deleted": "Usunięto „{title}”.",

  "storymaker.strip.title": "Sceny",
  "storymaker.strip.scene_n": "Scena {n}",
  "storymaker.strip.duration_s": "{seconds} s",
  "storymaker.strip.kind_code": "Scena z kodu",
  "storymaker.strip.add_storyboard": "Dodaj scenę",
  "storymaker.strip.add_code": "Dodaj scenę z kodu",

  "storymaker.stage.compiling": "Przygotowywanie podglądu…",
  "storymaker.stage.unavailable": "Podgląd jest niedostępny.",
  "storymaker.stage.player_error": "Podgląd zgłosił problem.",
  "storymaker.stage.code_error": "Scena z kodu zawiera błąd. Zobacz listę problemów.",

  "storymaker.inspector.title": "Ustawienia sceny",
  "storymaker.inspector.environment": "Otoczenie",
  "storymaker.inspector.variant": "Pora dnia",
  "storymaker.inspector.variant_day": "Dzień",
  "storymaker.inspector.variant_night": "Noc",
  "storymaker.inspector.narration": "Tekst lektora",
  "storymaker.inspector.narration_placeholder": "Co mówi lektor w tej scenie?",
  "storymaker.inspector.narration_hint": "Scena trwa tyle, ile zajmuje przeczytanie tekstu, chyba że ustawisz czas trwania.",
  "storymaker.inspector.transition": "Przejście do tej sceny",
  "storymaker.inspector.transition_first": "Pierwsza scena nie ma przejścia.",
  "storymaker.inspector.unsupported": "{name} (jeszcze nieobsługiwane)",
  "storymaker.inspector.duration": "Czas trwania",
  "storymaker.inspector.duration_auto": "Automatyczny",
  "storymaker.inspector.duration_fixed": "Stały",
  "storymaker.inspector.duration_seconds": "Czas trwania w sekundach",
  "storymaker.inspector.seconds_unit": "sekund",
  "storymaker.inspector.cast_props": "Postacie i rekwizyty w tej scenie",
  "storymaker.inspector.no_items": "Jeszcze nic. Wybierz postacie i rekwizyty w bibliotece.",
  "storymaker.inspector.remove": "Usuń",
  "storymaker.inspector.remove_item": "Usuń {name} ze sceny",
  "storymaker.inspector.move_up": "Przesuń scenę w górę",
  "storymaker.inspector.move_down": "Przesuń scenę w dół",
  "storymaker.inspector.delete_scene": "Usuń scenę",
  "storymaker.inspector.delete_scene_text": "Usunąć scenę {scene}? Jej tekst lektora i cała zawartość zostaną usunięte z historii.",
  "storymaker.inspector.code_scene_note": "Tę scenę rysuje mały program.",
  "storymaker.inspector.edit_code": "Edytuj kod",

  "storymaker.library.title": "Biblioteka",
  "storymaker.library.people": "Postacie",
  "storymaker.library.props": "Rekwizyty",
  "storymaker.library.environments": "Miejsca",
  "storymaker.library.search": "Przeszukaj bibliotekę",
  "storymaker.library.add": "Dodaj do sceny",
  "storymaker.library.use": "Użyj jako otoczenia",
  "storymaker.library.empty": "Nic nie pasuje.",
  "storymaker.library.code_scene": "Sceny z kodu rysują wszystko same.",
  "storymaker.library.already_in_scene": "Już jest w tej scenie.",
  "storymaker.library.added_actor": "Dodano do sceny: {name}.",
  "storymaker.library.added_prop": "Dodano do sceny: {name}.",

  "storymaker.issues.title": "Problemy",
  "storymaker.issues.none": "Brak problemów.",
  "storymaker.issues.count": "Problemy: {count}",
  "storymaker.issues.show": "Pokaż problemy",
  "storymaker.issues.show_in_code": "Pokaż w story.json",

  "storymaker.issue.invalid_json": "Nieprawidłowy JSON",
  "storymaker.issue.unknown_field": "Nieznane pole",
  "storymaker.issue.schema_mismatch": "Zły format historii",
  "storymaker.issue.too_large": "Za duże",
  "storymaker.issue.limit_exceeded": "Przekroczono limit",
  "storymaker.issue.missing_field": "Brakujące pole",
  "storymaker.issue.invalid_value": "Nieprawidłowa wartość",
  "storymaker.issue.duplicate_id": "Powtórzona nazwa",
  "storymaker.issue.unknown_name": "Nieznana nazwa",
  "storymaker.issue.unknown_asset": "Nieznany element katalogu",
  "storymaker.issue.unknown_spot": "Nieznane miejsce",
  "storymaker.issue.unknown_cast": "Nieznana postać",
  "storymaker.issue.unknown_cue": "Nieznany znacznik",
  "storymaker.issue.canonicalised": "Nazwa ujednolicona",
  "storymaker.issue.not_supported_yet": "Jeszcze nieobsługiwane",
  "storymaker.issue.narration_missing": "Brak dźwięku lektora",
  "storymaker.issue.code_error": "Błąd w scenie z kodu",
  "storymaker.issue.deadline": "Trwało zbyt długo",
  "storymaker.issue.layout_unverified": "Układ nie został sprawdzony",
  "storymaker.issue.unknown": "Problem",

  "storymaker.code.title": "Kod",
  "storymaker.code.files": "Pliki",
  "storymaker.code.editor_label": "Edytor kodu",
  "storymaker.code.save": "Zapisz",
  "storymaker.code.saved": "Zapisano",
  "storymaker.code.unsaved": "Niezapisane zmiany",
  "storymaker.code.stale": "Zmieniono. Zapisz, aby sprawdzić ponownie.",
  "storymaker.code.new_file": "Nowy plik, jeszcze nie zapisany",
  "storymaker.code.reload": "Wczytaj ponownie",
  "storymaker.code.bundle_ok": "Scena z kodu zapisana.",
  "storymaker.code.bundle_failed": "Kod zawiera błędy.",
  "storymaker.code.no_problems": "Nie znaleziono problemów.",
  "storymaker.code.problem_count": "Problemy: {count}",
  "storymaker.code.line_col": "Wiersz {line}, kolumna {column}",
  "storymaker.code.more": "i jeszcze {count}",

  "storymaker.export.button": "Eksportuj",
  "storymaker.export.button_running": "Eksport {percent} %",
  "storymaker.export.title": "Eksport wideo",
  "storymaker.export.cap_ffmpeg": "FFmpeg",
  "storymaker.export.cap_browser": "Przeglądarka",
  "storymaker.export.ready": "Gotowe",
  "storymaker.export.ready_version": "Gotowe, wersja {version}",
  "storymaker.export.ffmpeg_missing": "Nie znaleziono. Zainstaluj FFmpeg lub podaj jego ścieżkę w ustawieniach Video Studio.",
  "storymaker.export.browser_ready": "Gotowe ({source})",
  "storymaker.export.browser_missing": "Nie ma przeglądarki do rysowania klatek.",
  "storymaker.export.source_config": "z konfiguracji",
  "storymaker.export.source_system": "zainstalowana w tym systemie",
  "storymaker.export.source_pinned": "pobrana przez AuraGo",
  "storymaker.export.source_unknown": "nieznane źródło",
  "storymaker.export.still_works": "Edycja i podgląd działają bez nich.",
  "storymaker.export.blocked_readonly": "Pulpit jest tylko do odczytu.",
  "storymaker.export.blocked_ffmpeg": "Brakuje FFmpeg.",
  "storymaker.export.blocked_browser": "Brak dostępnej przeglądarki.",
  "storymaker.export.blocked_errors": "Historia zawiera błędy. Najpierw je popraw.",
  "storymaker.export.blocked_too_long": "Film jest dłuższy niż limit {max} s.",
  "storymaker.export.blocked_not_ready": "Podgląd jest jeszcze przygotowywany.",
  "storymaker.export.summary": "Długość {duration}, {format}, {width} × {height} pikseli.",
  "storymaker.export.silent_note": "Ta wersja eksportuje filmy bez dźwięku. Dźwięk lektora pojawi się później.",
  "storymaker.export.start": "Rozpocznij eksport",
  "storymaker.export.starting": "Uruchamianie…",
  "storymaker.export.queued": "Czeka na swoją kolej…",
  "storymaker.export.phase_starting": "Uruchamianie",
  "storymaker.export.phase_compiling": "Przygotowanie historii",
  "storymaker.export.phase_mixing": "Miksowanie dźwięku",
  "storymaker.export.phase_rendering": "Rysowanie klatek",
  "storymaker.export.phase_encoding": "Kodowanie wideo",
  "storymaker.export.phase_writing": "Zapisywanie pliku",
  "storymaker.export.frames": "{done} z {total} klatek",
  "storymaker.export.cancel": "Anuluj eksport",
  "storymaker.export.cancelling": "Anulowanie…",
  "storymaker.export.cancelled": "Eksport został anulowany.",
  "storymaker.export.interrupted": "Eksport został przerwany przez ponowne uruchomienie.",
  "storymaker.export.done": "Eksport zakończony",
  "storymaker.export.output": "Film jest w folderze Twojego projektu:",
  "storymaker.export.show_in_files": "Pokaż w Plikach",
  "storymaker.export.copy_path": "Kopiuj ścieżkę",
  "storymaker.export.copied": "Skopiowano",
  "storymaker.export.copy_failed": "Nie udało się skopiować ścieżki.",
  "storymaker.export.again": "Eksportuj ponownie",

  "storymaker.job_error.invalid_story": "Historia zawiera błędy, więc nie można jej wyeksportować.",
  "storymaker.job_error.storymaker_disabled": "Storymaker jest wyłączony.",
  "storymaker.job_error.browser_unavailable": "Nie udało się uruchomić przeglądarki do rysowania klatek.",
  "storymaker.job_error.ffmpeg_unavailable": "FFmpeg jest niedostępny.",
  "storymaker.job_error.render_failed": "Eksport nie powiódł się.",
  "storymaker.job_error.render_timeout": "Eksport trwał zbyt długo i został zatrzymany.",
  "storymaker.job_error.cancelled": "Eksport został anulowany.",
  "storymaker.job_error.busy": "Trwa inny eksport. Spróbuj ponownie za chwilę.",
  "storymaker.job_error.code_error": "Scena z kodu zawiodła podczas rysowania.",
  "storymaker.job_error.unknown": "Eksport nie powiódł się.",

  "storymaker.error.unauthorized": "Twoja sesja wygasła. Zaloguj się ponownie.",
  "storymaker.error.scope": "To logowanie nie ma uprawnień do tej czynności.",
  "storymaker.error.readonly": "Pulpit jest tylko do odczytu.",
  "storymaker.error.disabled": "Storymaker jest wyłączony.",
  "storymaker.error.invalid_story": "Historia nie została zapisana, bo zawiera błędy.",
  "storymaker.error.conflict": "Historia została zmieniona gdzie indziej.",
  "storymaker.error.required": "Serwer potrzebuje aktualnej wersji historii. Wczytaj ją ponownie.",
  "storymaker.error.too_large": "To jest za duże.",
  "storymaker.error.not_found": "To już nie istnieje.",
  "storymaker.error.busy": "Serwer jest zajęty. Spróbuj ponownie za chwilę.",
  "storymaker.error.network": "Nie można połączyć się z serwerem.",
  "storymaker.error.generic": "Coś poszło nie tak."
}
```

`ui/lang/storymaker/pt.json`:

```json
{
  "storymaker.title": "Storymaker",
  "storymaker.loading": "A carregar…",
  "storymaker.retry": "Tentar novamente",
  "storymaker.close": "Fechar",
  "storymaker.delete": "Eliminar",
  "storymaker.back_to_projects": "Projetos",
  "storymaker.preview": "Pré-visualização do Storymaker",
  "storymaker.play": "Reproduzir",
  "storymaker.pause": "Pausa",
  "storymaker.timeline": "Linha do tempo",
  "storymaker.tab_stage": "Storyboard",
  "storymaker.tab_code": "Código",
  "storymaker.side_scene": "Cena",
  "storymaker.side_library": "Biblioteca",
  "storymaker.save_saved": "Guardado",
  "storymaker.save_dirty": "Alterações por guardar",
  "storymaker.save_saving": "A guardar…",
  "storymaker.save_error": "Não guardado",
  "storymaker.save_offline": "Sem ligação, não guardado",
  "storymaker.save_conflict": "Alterado noutro local",
  "storymaker.save_readonly": "Só de leitura",
  "storymaker.leave_title": "Sair com alterações por guardar?",
  "storymaker.leave_text": "Não foi possível guardar as tuas últimas alterações à história. Se saíres agora, perdem-se.",
  "storymaker.leave_code_text": "Um ficheiro de código tem alterações por guardar. Se saíres agora, perdem-se.",
  "storymaker.aspect_16_9": "Horizontal 16:9",
  "storymaker.aspect_9_16": "Vertical 9:16",
  "storymaker.aspect_1_1": "Quadrado 1:1",

  "storymaker.banner.readonly": "O ambiente de trabalho está só de leitura. Podes ver a pré-visualização, mas editar, guardar e exportar estão desativados.",
  "storymaker.banner.conflict": "Esta história foi alterada noutro local, por exemplo noutra janela. Recarrega para ver a versão atual, ou mantém a tua e substitui.",
  "storymaker.banner.conflict_reload": "Recarregar",
  "storymaker.banner.conflict_overwrite": "Manter a minha",
  "storymaker.banner.player_unavailable": "A pré-visualização não conseguiu iniciar neste navegador. A tua história continua guardada e podes continuar a editá-la.",
  "storymaker.banner.export_unavailable": "A exportação ainda não está disponível (falta {missing}). A edição e a pré-visualização continuam a funcionar.",

  "storymaker.state.disabled_title": "O Storymaker está desativado",
  "storymaker.state.disabled_text": "Um administrador pode ativá-lo na configuração, em Ambiente de trabalho virtual.",
  "storymaker.state.unauthorized_title": "Inicia sessão novamente",
  "storymaker.state.unauthorized_text": "A tua sessão terminou. Inicia sessão novamente e volta a abrir o Storymaker.",
  "storymaker.state.scope_title": "Sem acesso",
  "storymaker.state.scope_text": "Este início de sessão não tem permissão para usar o Ambiente de trabalho virtual.",
  "storymaker.state.load_failed_title": "O Storymaker não conseguiu iniciar",
  "storymaker.state.load_failed_text": "O servidor não respondeu como esperado. Verifica a ligação e tenta novamente.",

  "storymaker.projects.title": "Projetos",
  "storymaker.projects.intro": "Transforma um tema num vídeo explicativo curto: encena as cenas, escreve a narração e exporta um MP4.",
  "storymaker.projects.new_title_label": "Título",
  "storymaker.projects.new_title_placeholder": "O que é uma firewall?",
  "storymaker.projects.new_format": "Formato",
  "storymaker.projects.create": "Novo projeto",
  "storymaker.projects.title_required": "Introduz um título.",
  "storymaker.projects.empty_title": "Ainda não há projetos",
  "storymaker.projects.empty_text": "Cria o teu primeiro projeto acima. Começa com uma cena vazia.",
  "storymaker.projects.delete_label": "Eliminar o projeto {title}",
  "storymaker.projects.delete_title": "Eliminar o projeto?",
  "storymaker.projects.delete_text": "O projeto «{title}» e todos os seus ficheiros, incluindo as exportações, serão eliminados. Isto não pode ser desfeito.",
  "storymaker.projects.deleted": "«{title}» eliminado.",

  "storymaker.strip.title": "Cenas",
  "storymaker.strip.scene_n": "Cena {n}",
  "storymaker.strip.duration_s": "{seconds} s",
  "storymaker.strip.kind_code": "Cena de código",
  "storymaker.strip.add_storyboard": "Adicionar cena",
  "storymaker.strip.add_code": "Adicionar cena de código",

  "storymaker.stage.compiling": "A preparar a pré-visualização…",
  "storymaker.stage.unavailable": "A pré-visualização não está disponível.",
  "storymaker.stage.player_error": "A pré-visualização comunicou um problema.",
  "storymaker.stage.code_error": "Uma cena de código tem um erro. Vê a lista de problemas.",

  "storymaker.inspector.title": "Definições da cena",
  "storymaker.inspector.environment": "Ambiente",
  "storymaker.inspector.variant": "Altura do dia",
  "storymaker.inspector.variant_day": "Dia",
  "storymaker.inspector.variant_night": "Noite",
  "storymaker.inspector.narration": "Narração",
  "storymaker.inspector.narration_placeholder": "O que diz o narrador nesta cena?",
  "storymaker.inspector.narration_hint": "A cena dura o tempo que o texto demora a ler, a menos que definas uma duração.",
  "storymaker.inspector.transition": "Transição para esta cena",
  "storymaker.inspector.transition_first": "A primeira cena não tem transição.",
  "storymaker.inspector.unsupported": "{name} (ainda não suportado)",
  "storymaker.inspector.duration": "Duração",
  "storymaker.inspector.duration_auto": "Automática",
  "storymaker.inspector.duration_fixed": "Fixa",
  "storymaker.inspector.duration_seconds": "Duração em segundos",
  "storymaker.inspector.seconds_unit": "segundos",
  "storymaker.inspector.cast_props": "Personagens e adereços desta cena",
  "storymaker.inspector.no_items": "Ainda nada. Escolhe personagens e adereços na biblioteca.",
  "storymaker.inspector.remove": "Remover",
  "storymaker.inspector.remove_item": "Remover {name} da cena",
  "storymaker.inspector.move_up": "Mover a cena para cima",
  "storymaker.inspector.move_down": "Mover a cena para baixo",
  "storymaker.inspector.delete_scene": "Eliminar cena",
  "storymaker.inspector.delete_scene_text": "Eliminar a cena {scene}? A sua narração e tudo o que contém serão removidos da história.",
  "storymaker.inspector.code_scene_note": "Esta cena é desenhada por um pequeno programa.",
  "storymaker.inspector.edit_code": "Editar código",

  "storymaker.library.title": "Biblioteca",
  "storymaker.library.people": "Personagens",
  "storymaker.library.props": "Adereços",
  "storymaker.library.environments": "Locais",
  "storymaker.library.search": "Pesquisar na biblioteca",
  "storymaker.library.add": "Adicionar à cena",
  "storymaker.library.use": "Usar como ambiente",
  "storymaker.library.empty": "Nada corresponde.",
  "storymaker.library.code_scene": "As cenas de código desenham tudo sozinhas.",
  "storymaker.library.already_in_scene": "Já está nesta cena.",
  "storymaker.library.added_actor": "{name} juntou-se à cena.",
  "storymaker.library.added_prop": "Adicionado à cena: {name}.",

  "storymaker.issues.title": "Problemas",
  "storymaker.issues.none": "Sem problemas.",
  "storymaker.issues.count": "Problemas: {count}",
  "storymaker.issues.show": "Mostrar problemas",
  "storymaker.issues.show_in_code": "Mostrar em story.json",

  "storymaker.issue.invalid_json": "JSON inválido",
  "storymaker.issue.unknown_field": "Campo desconhecido",
  "storymaker.issue.schema_mismatch": "Formato de história incorreto",
  "storymaker.issue.too_large": "Demasiado grande",
  "storymaker.issue.limit_exceeded": "Limite excedido",
  "storymaker.issue.missing_field": "Campo em falta",
  "storymaker.issue.invalid_value": "Valor inválido",
  "storymaker.issue.duplicate_id": "Nome duplicado",
  "storymaker.issue.unknown_name": "Nome desconhecido",
  "storymaker.issue.unknown_asset": "Item de catálogo desconhecido",
  "storymaker.issue.unknown_spot": "Posição desconhecida",
  "storymaker.issue.unknown_cast": "Personagem desconhecida",
  "storymaker.issue.unknown_cue": "Marca desconhecida",
  "storymaker.issue.canonicalised": "Nome normalizado",
  "storymaker.issue.not_supported_yet": "Ainda não suportado",
  "storymaker.issue.narration_missing": "Sem áudio de narração",
  "storymaker.issue.code_error": "Erro na cena de código",
  "storymaker.issue.deadline": "Demorou demasiado",
  "storymaker.issue.layout_unverified": "Disposição não verificada",
  "storymaker.issue.unknown": "Problema",

  "storymaker.code.title": "Código",
  "storymaker.code.files": "Ficheiros",
  "storymaker.code.editor_label": "Editor de código",
  "storymaker.code.save": "Guardar",
  "storymaker.code.saved": "Guardado",
  "storymaker.code.unsaved": "Alterações por guardar",
  "storymaker.code.stale": "Alterado. Guarda para verificar de novo.",
  "storymaker.code.new_file": "Ficheiro novo, ainda não guardado",
  "storymaker.code.reload": "Recarregar",
  "storymaker.code.bundle_ok": "Cena de código guardada.",
  "storymaker.code.bundle_failed": "O código tem erros.",
  "storymaker.code.no_problems": "Nenhum problema encontrado.",
  "storymaker.code.problem_count": "Problemas: {count}",
  "storymaker.code.line_col": "Linha {line}, coluna {column}",
  "storymaker.code.more": "e mais {count}",

  "storymaker.export.button": "Exportar",
  "storymaker.export.button_running": "A exportar {percent} %",
  "storymaker.export.title": "Exportar vídeo",
  "storymaker.export.cap_ffmpeg": "FFmpeg",
  "storymaker.export.cap_browser": "Navegador",
  "storymaker.export.ready": "Pronto",
  "storymaker.export.ready_version": "Pronto, versão {version}",
  "storymaker.export.ffmpeg_missing": "Não encontrado. Instala o FFmpeg ou indica o seu caminho nas definições do Video Studio.",
  "storymaker.export.browser_ready": "Pronto ({source})",
  "storymaker.export.browser_missing": "Não há nenhum navegador disponível para desenhar as imagens.",
  "storymaker.export.source_config": "da configuração",
  "storymaker.export.source_system": "instalado neste sistema",
  "storymaker.export.source_pinned": "transferido pelo AuraGo",
  "storymaker.export.source_unknown": "origem desconhecida",
  "storymaker.export.still_works": "A edição e a pré-visualização funcionam sem eles.",
  "storymaker.export.blocked_readonly": "O ambiente de trabalho está só de leitura.",
  "storymaker.export.blocked_ffmpeg": "Falta o FFmpeg.",
  "storymaker.export.blocked_browser": "Não há nenhum navegador disponível.",
  "storymaker.export.blocked_errors": "A história tem erros. Corrige-os primeiro.",
  "storymaker.export.blocked_too_long": "O vídeo é mais longo do que o limite de {max} segundos.",
  "storymaker.export.blocked_not_ready": "A pré-visualização ainda está a ser preparada.",
  "storymaker.export.summary": "Duração {duration}, {format}, {width} × {height} píxeis.",
  "storymaker.export.silent_note": "Esta versão exporta vídeos sem som. O áudio da narração chega mais tarde.",
  "storymaker.export.start": "Iniciar exportação",
  "storymaker.export.starting": "A iniciar…",
  "storymaker.export.queued": "À espera da sua vez…",
  "storymaker.export.phase_starting": "A iniciar",
  "storymaker.export.phase_compiling": "A preparar a história",
  "storymaker.export.phase_mixing": "A misturar o som",
  "storymaker.export.phase_rendering": "A desenhar as imagens",
  "storymaker.export.phase_encoding": "A codificar o vídeo",
  "storymaker.export.phase_writing": "A guardar o ficheiro",
  "storymaker.export.frames": "{done} de {total} imagens",
  "storymaker.export.cancel": "Cancelar exportação",
  "storymaker.export.cancelling": "A cancelar…",
  "storymaker.export.cancelled": "A exportação foi cancelada.",
  "storymaker.export.interrupted": "A exportação foi interrompida por um reinício.",
  "storymaker.export.done": "Exportação concluída",
  "storymaker.export.output": "O vídeo está na pasta do teu projeto:",
  "storymaker.export.show_in_files": "Mostrar em Ficheiros",
  "storymaker.export.copy_path": "Copiar caminho",
  "storymaker.export.copied": "Copiado",
  "storymaker.export.copy_failed": "Não foi possível copiar o caminho.",
  "storymaker.export.again": "Exportar novamente",

  "storymaker.job_error.invalid_story": "A história tem erros, por isso não pode ser exportada.",
  "storymaker.job_error.storymaker_disabled": "O Storymaker está desativado.",
  "storymaker.job_error.browser_unavailable": "Não foi possível iniciar nenhum navegador para desenhar as imagens.",
  "storymaker.job_error.ffmpeg_unavailable": "O FFmpeg não está disponível.",
  "storymaker.job_error.render_failed": "A exportação falhou.",
  "storymaker.job_error.render_timeout": "A exportação demorou demasiado e foi interrompida.",
  "storymaker.job_error.cancelled": "A exportação foi cancelada.",
  "storymaker.job_error.busy": "Está em curso outra exportação. Tenta novamente daqui a pouco.",
  "storymaker.job_error.code_error": "Uma cena de código falhou ao desenhar.",
  "storymaker.job_error.unknown": "A exportação falhou.",

  "storymaker.error.unauthorized": "A tua sessão terminou. Inicia sessão novamente.",
  "storymaker.error.scope": "Este início de sessão não tem permissão para fazer isso.",
  "storymaker.error.readonly": "O ambiente de trabalho está só de leitura.",
  "storymaker.error.disabled": "O Storymaker está desativado.",
  "storymaker.error.invalid_story": "A história não foi guardada porque tem erros.",
  "storymaker.error.conflict": "A história foi alterada noutro local.",
  "storymaker.error.required": "O servidor precisa da versão atual da história. Recarrega-a.",
  "storymaker.error.too_large": "Isso é demasiado grande.",
  "storymaker.error.not_found": "Isso já não existe.",
  "storymaker.error.busy": "O servidor está ocupado. Tenta novamente daqui a pouco.",
  "storymaker.error.network": "Não foi possível contactar o servidor.",
  "storymaker.error.generic": "Algo correu mal."
}
```

`ui/lang/storymaker/sv.json`:

```json
{
  "storymaker.title": "Storymaker",
  "storymaker.loading": "Läser in …",
  "storymaker.retry": "Försök igen",
  "storymaker.close": "Stäng",
  "storymaker.delete": "Ta bort",
  "storymaker.back_to_projects": "Projekt",
  "storymaker.preview": "Förhandsvisning i Storymaker",
  "storymaker.play": "Spela upp",
  "storymaker.pause": "Pausa",
  "storymaker.timeline": "Tidslinje",
  "storymaker.tab_stage": "Storyboard",
  "storymaker.tab_code": "Kod",
  "storymaker.side_scene": "Scen",
  "storymaker.side_library": "Bibliotek",
  "storymaker.save_saved": "Sparad",
  "storymaker.save_dirty": "Osparade ändringar",
  "storymaker.save_saving": "Sparar …",
  "storymaker.save_error": "Inte sparad",
  "storymaker.save_offline": "Offline, inte sparad",
  "storymaker.save_conflict": "Ändrad någon annanstans",
  "storymaker.save_readonly": "Skrivskyddad",
  "storymaker.leave_title": "Lämna med osparade ändringar?",
  "storymaker.leave_text": "Dina senaste ändringar i berättelsen kunde inte sparas. Om du lämnar nu går de förlorade.",
  "storymaker.leave_code_text": "En kodfil har osparade ändringar. Om du lämnar nu går de förlorade.",
  "storymaker.aspect_16_9": "Liggande 16:9",
  "storymaker.aspect_9_16": "Stående 9:16",
  "storymaker.aspect_1_1": "Kvadrat 1:1",

  "storymaker.banner.readonly": "Skrivbordet är skrivskyddat. Du kan titta på förhandsvisningen, men redigering, sparande och export är avstängda.",
  "storymaker.banner.conflict": "Den här berättelsen har ändrats någon annanstans, till exempel i ett annat fönster. Läs in igen för att se den aktuella versionen, eller behåll din och skriv över.",
  "storymaker.banner.conflict_reload": "Läs in igen",
  "storymaker.banner.conflict_overwrite": "Behåll min",
  "storymaker.banner.player_unavailable": "Förhandsvisningen kunde inte starta i den här webbläsaren. Din berättelse är fortfarande sparad och du kan fortsätta redigera den.",
  "storymaker.banner.export_unavailable": "Export är inte tillgänglig än ({missing} saknas). Redigering och förhandsvisning fungerar ändå.",

  "storymaker.state.disabled_title": "Storymaker är avstängt",
  "storymaker.state.disabled_text": "En administratör kan slå på det i konfigurationen under Virtuellt skrivbord.",
  "storymaker.state.unauthorized_title": "Logga in igen",
  "storymaker.state.unauthorized_text": "Din session har tagit slut. Logga in igen och öppna Storymaker på nytt.",
  "storymaker.state.scope_title": "Ingen åtkomst",
  "storymaker.state.scope_text": "Den här inloggningen får inte använda det virtuella skrivbordet.",
  "storymaker.state.load_failed_title": "Storymaker kunde inte starta",
  "storymaker.state.load_failed_text": "Servern svarade inte som förväntat. Kontrollera anslutningen och försök igen.",

  "storymaker.projects.title": "Projekt",
  "storymaker.projects.intro": "Gör ett ämne till en kort förklarande film: iscensätt scenerna, skriv berättarrösten och exportera en MP4.",
  "storymaker.projects.new_title_label": "Titel",
  "storymaker.projects.new_title_placeholder": "Vad är en brandvägg?",
  "storymaker.projects.new_format": "Format",
  "storymaker.projects.create": "Nytt projekt",
  "storymaker.projects.title_required": "Skriv en titel.",
  "storymaker.projects.empty_title": "Inga projekt än",
  "storymaker.projects.empty_text": "Skapa ditt första projekt ovan. Det börjar med en tom scen.",
  "storymaker.projects.delete_label": "Ta bort projektet {title}",
  "storymaker.projects.delete_title": "Ta bort projektet?",
  "storymaker.projects.delete_text": "Projektet ”{title}” och alla dess filer, även exporterna, tas bort. Det går inte att ångra.",
  "storymaker.projects.deleted": "”{title}” har tagits bort.",

  "storymaker.strip.title": "Scener",
  "storymaker.strip.scene_n": "Scen {n}",
  "storymaker.strip.duration_s": "{seconds} s",
  "storymaker.strip.kind_code": "Kodscen",
  "storymaker.strip.add_storyboard": "Lägg till scen",
  "storymaker.strip.add_code": "Lägg till kodscen",

  "storymaker.stage.compiling": "Förbereder förhandsvisningen …",
  "storymaker.stage.unavailable": "Förhandsvisningen är inte tillgänglig.",
  "storymaker.stage.player_error": "Förhandsvisningen rapporterade ett problem.",
  "storymaker.stage.code_error": "En kodscen har ett fel. Se listan över problem.",

  "storymaker.inspector.title": "Sceninställningar",
  "storymaker.inspector.environment": "Miljö",
  "storymaker.inspector.variant": "Tid på dygnet",
  "storymaker.inspector.variant_day": "Dag",
  "storymaker.inspector.variant_night": "Natt",
  "storymaker.inspector.narration": "Berättarröst",
  "storymaker.inspector.narration_placeholder": "Vad säger berättaren i den här scenen?",
  "storymaker.inspector.narration_hint": "Scenen varar lika länge som texten tar att läsa, om du inte anger en längd.",
  "storymaker.inspector.transition": "Övergång in i den här scenen",
  "storymaker.inspector.transition_first": "Den första scenen har ingen övergång.",
  "storymaker.inspector.unsupported": "{name} (stöds inte än)",
  "storymaker.inspector.duration": "Längd",
  "storymaker.inspector.duration_auto": "Automatisk",
  "storymaker.inspector.duration_fixed": "Fast",
  "storymaker.inspector.duration_seconds": "Längd i sekunder",
  "storymaker.inspector.seconds_unit": "sekunder",
  "storymaker.inspector.cast_props": "Personer och rekvisita i den här scenen",
  "storymaker.inspector.no_items": "Inget än. Välj personer och rekvisita i biblioteket.",
  "storymaker.inspector.remove": "Ta bort",
  "storymaker.inspector.remove_item": "Ta bort {name} från scenen",
  "storymaker.inspector.move_up": "Flytta scenen uppåt",
  "storymaker.inspector.move_down": "Flytta scenen nedåt",
  "storymaker.inspector.delete_scene": "Ta bort scen",
  "storymaker.inspector.delete_scene_text": "Ta bort scenen {scene}? Dess berättarröst och allt i den tas bort från berättelsen.",
  "storymaker.inspector.code_scene_note": "Den här scenen ritas av ett litet program.",
  "storymaker.inspector.edit_code": "Redigera kod",

  "storymaker.library.title": "Bibliotek",
  "storymaker.library.people": "Personer",
  "storymaker.library.props": "Rekvisita",
  "storymaker.library.environments": "Platser",
  "storymaker.library.search": "Sök i biblioteket",
  "storymaker.library.add": "Lägg till i scenen",
  "storymaker.library.use": "Använd som miljö",
  "storymaker.library.empty": "Inget matchar.",
  "storymaker.library.code_scene": "Kodscener ritar allt själva.",
  "storymaker.library.already_in_scene": "Finns redan i den här scenen.",
  "storymaker.library.added_actor": "{name} har kommit med i scenen.",
  "storymaker.library.added_prop": "{name} lades till i scenen.",

  "storymaker.issues.title": "Problem",
  "storymaker.issues.none": "Inga problem.",
  "storymaker.issues.count": "Problem: {count}",
  "storymaker.issues.show": "Visa problem",
  "storymaker.issues.show_in_code": "Visa i story.json",

  "storymaker.issue.invalid_json": "Ogiltig JSON",
  "storymaker.issue.unknown_field": "Okänt fält",
  "storymaker.issue.schema_mismatch": "Fel berättelseformat",
  "storymaker.issue.too_large": "För stor",
  "storymaker.issue.limit_exceeded": "Gränsen överskriden",
  "storymaker.issue.missing_field": "Fält saknas",
  "storymaker.issue.invalid_value": "Ogiltigt värde",
  "storymaker.issue.duplicate_id": "Dubblett av namn",
  "storymaker.issue.unknown_name": "Okänt namn",
  "storymaker.issue.unknown_asset": "Okänt katalogobjekt",
  "storymaker.issue.unknown_spot": "Okänd plats",
  "storymaker.issue.unknown_cast": "Okänd person",
  "storymaker.issue.unknown_cue": "Okänd markering",
  "storymaker.issue.canonicalised": "Namnet normaliserades",
  "storymaker.issue.not_supported_yet": "Stöds inte än",
  "storymaker.issue.narration_missing": "Inget ljud för berättarrösten",
  "storymaker.issue.code_error": "Fel i kodscen",
  "storymaker.issue.deadline": "Tog för lång tid",
  "storymaker.issue.layout_unverified": "Layouten är inte kontrollerad",
  "storymaker.issue.unknown": "Problem",

  "storymaker.code.title": "Kod",
  "storymaker.code.files": "Filer",
  "storymaker.code.editor_label": "Kodredigerare",
  "storymaker.code.save": "Spara",
  "storymaker.code.saved": "Sparad",
  "storymaker.code.unsaved": "Osparade ändringar",
  "storymaker.code.stale": "Ändrad. Spara för att kontrollera igen.",
  "storymaker.code.new_file": "Ny fil, inte sparad än",
  "storymaker.code.reload": "Läs in igen",
  "storymaker.code.bundle_ok": "Kodscenen sparades.",
  "storymaker.code.bundle_failed": "Koden har fel.",
  "storymaker.code.no_problems": "Inga problem hittades.",
  "storymaker.code.problem_count": "Problem: {count}",
  "storymaker.code.line_col": "Rad {line}, kolumn {column}",
  "storymaker.code.more": "och {count} till",

  "storymaker.export.button": "Exportera",
  "storymaker.export.button_running": "Exporterar {percent} %",
  "storymaker.export.title": "Exportera film",
  "storymaker.export.cap_ffmpeg": "FFmpeg",
  "storymaker.export.cap_browser": "Webbläsare",
  "storymaker.export.ready": "Redo",
  "storymaker.export.ready_version": "Redo, version {version}",
  "storymaker.export.ffmpeg_missing": "Hittades inte. Installera FFmpeg eller ange sökvägen i Video Studios inställningar.",
  "storymaker.export.browser_ready": "Redo ({source})",
  "storymaker.export.browser_missing": "Ingen webbläsare finns tillgänglig för att rita bildrutorna.",
  "storymaker.export.source_config": "från konfigurationen",
  "storymaker.export.source_system": "installerad på det här systemet",
  "storymaker.export.source_pinned": "nedladdad av AuraGo",
  "storymaker.export.source_unknown": "okänd källa",
  "storymaker.export.still_works": "Redigering och förhandsvisning fungerar även utan dem.",
  "storymaker.export.blocked_readonly": "Skrivbordet är skrivskyddat.",
  "storymaker.export.blocked_ffmpeg": "FFmpeg saknas.",
  "storymaker.export.blocked_browser": "Ingen webbläsare är tillgänglig.",
  "storymaker.export.blocked_errors": "Berättelsen har fel. Åtgärda dem först.",
  "storymaker.export.blocked_too_long": "Filmen är längre än gränsen på {max} sekunder.",
  "storymaker.export.blocked_not_ready": "Förhandsvisningen förbereds fortfarande.",
  "storymaker.export.summary": "Längd {duration}, {format}, {width} × {height} pixlar.",
  "storymaker.export.silent_note": "Den här versionen exporterar filmer utan ljud. Ljud för berättarrösten kommer senare.",
  "storymaker.export.start": "Starta export",
  "storymaker.export.starting": "Startar …",
  "storymaker.export.queued": "Väntar på sin tur …",
  "storymaker.export.phase_starting": "Startar",
  "storymaker.export.phase_compiling": "Förbereder berättelsen",
  "storymaker.export.phase_mixing": "Mixar ljudet",
  "storymaker.export.phase_rendering": "Ritar bildrutorna",
  "storymaker.export.phase_encoding": "Kodar filmen",
  "storymaker.export.phase_writing": "Sparar filen",
  "storymaker.export.frames": "{done} av {total} bildrutor",
  "storymaker.export.cancel": "Avbryt exporten",
  "storymaker.export.cancelling": "Avbryter …",
  "storymaker.export.cancelled": "Exporten avbröts.",
  "storymaker.export.interrupted": "Exporten avbröts av en omstart.",
  "storymaker.export.done": "Exporten är klar",
  "storymaker.export.output": "Filmen finns i din projektmapp:",
  "storymaker.export.show_in_files": "Visa i Filer",
  "storymaker.export.copy_path": "Kopiera sökväg",
  "storymaker.export.copied": "Kopierad",
  "storymaker.export.copy_failed": "Sökvägen kunde inte kopieras.",
  "storymaker.export.again": "Exportera igen",

  "storymaker.job_error.invalid_story": "Berättelsen har fel och kan därför inte exporteras.",
  "storymaker.job_error.storymaker_disabled": "Storymaker är avstängt.",
  "storymaker.job_error.browser_unavailable": "Ingen webbläsare kunde startas för att rita bildrutorna.",
  "storymaker.job_error.ffmpeg_unavailable": "FFmpeg är inte tillgängligt.",
  "storymaker.job_error.render_failed": "Exporten misslyckades.",
  "storymaker.job_error.render_timeout": "Exporten tog för lång tid och stoppades.",
  "storymaker.job_error.cancelled": "Exporten avbröts.",
  "storymaker.job_error.busy": "En annan export pågår. Försök igen om en stund.",
  "storymaker.job_error.code_error": "En kodscen misslyckades vid ritningen.",
  "storymaker.job_error.unknown": "Exporten misslyckades.",

  "storymaker.error.unauthorized": "Din session har tagit slut. Logga in igen.",
  "storymaker.error.scope": "Den här inloggningen får inte göra det.",
  "storymaker.error.readonly": "Skrivbordet är skrivskyddat.",
  "storymaker.error.disabled": "Storymaker är avstängt.",
  "storymaker.error.invalid_story": "Berättelsen sparades inte eftersom den har fel.",
  "storymaker.error.conflict": "Berättelsen har ändrats någon annanstans.",
  "storymaker.error.required": "Servern behöver den aktuella versionen av berättelsen. Läs in den igen.",
  "storymaker.error.too_large": "Det är för stort.",
  "storymaker.error.not_found": "Det finns inte längre.",
  "storymaker.error.busy": "Servern är upptagen. Försök igen om en stund.",
  "storymaker.error.network": "Servern kunde inte nås.",
  "storymaker.error.generic": "Något gick fel."
}
```

`ui/lang/storymaker/zh.json`:

```json
{
  "storymaker.title": "Storymaker",
  "storymaker.loading": "正在加载…",
  "storymaker.retry": "重试",
  "storymaker.close": "关闭",
  "storymaker.delete": "删除",
  "storymaker.back_to_projects": "项目",
  "storymaker.preview": "Storymaker 预览",
  "storymaker.play": "播放",
  "storymaker.pause": "暂停",
  "storymaker.timeline": "时间轴",
  "storymaker.tab_stage": "分镜",
  "storymaker.tab_code": "代码",
  "storymaker.side_scene": "场景",
  "storymaker.side_library": "素材库",
  "storymaker.save_saved": "已保存",
  "storymaker.save_dirty": "有未保存的更改",
  "storymaker.save_saving": "正在保存…",
  "storymaker.save_error": "未保存",
  "storymaker.save_offline": "已离线，未保存",
  "storymaker.save_conflict": "已在别处更改",
  "storymaker.save_readonly": "只读",
  "storymaker.leave_title": "有未保存的更改，仍要离开吗？",
  "storymaker.leave_text": "你对故事的最新更改未能保存。现在离开会丢失这些更改。",
  "storymaker.leave_code_text": "有代码文件含未保存的更改。现在离开会丢失这些更改。",
  "storymaker.aspect_16_9": "横屏 16:9",
  "storymaker.aspect_9_16": "竖屏 9:16",
  "storymaker.aspect_1_1": "方形 1:1",

  "storymaker.banner.readonly": "桌面处于只读状态。你可以观看预览，但编辑、保存和导出均已关闭。",
  "storymaker.banner.conflict": "这个故事已在别处被更改，例如在另一个窗口中。请重新加载以查看当前版本，或保留你的版本并覆盖。",
  "storymaker.banner.conflict_reload": "重新加载",
  "storymaker.banner.conflict_overwrite": "保留我的版本",
  "storymaker.banner.player_unavailable": "预览无法在此浏览器中启动。你的故事仍已保存，可以继续编辑。",
  "storymaker.banner.export_unavailable": "导出暂不可用（缺少：{missing}）。编辑和预览仍可使用。",

  "storymaker.state.disabled_title": "Storymaker 已关闭",
  "storymaker.state.disabled_text": "管理员可以在配置的“虚拟桌面”下将其打开。",
  "storymaker.state.unauthorized_title": "请重新登录",
  "storymaker.state.unauthorized_text": "你的会话已结束。请重新登录，然后再次打开 Storymaker。",
  "storymaker.state.scope_title": "无权访问",
  "storymaker.state.scope_text": "此登录无权使用虚拟桌面。",
  "storymaker.state.load_failed_title": "Storymaker 无法启动",
  "storymaker.state.load_failed_text": "服务器没有按预期响应。请检查连接后重试。",

  "storymaker.projects.title": "项目",
  "storymaker.projects.intro": "把一个主题做成简短的讲解视频：布置场景、撰写旁白，然后导出 MP4。",
  "storymaker.projects.new_title_label": "标题",
  "storymaker.projects.new_title_placeholder": "什么是防火墙？",
  "storymaker.projects.new_format": "格式",
  "storymaker.projects.create": "新建项目",
  "storymaker.projects.title_required": "请输入标题。",
  "storymaker.projects.empty_title": "还没有项目",
  "storymaker.projects.empty_text": "在上方创建你的第一个项目。它从一个空场景开始。",
  "storymaker.projects.delete_label": "删除项目 {title}",
  "storymaker.projects.delete_title": "删除项目？",
  "storymaker.projects.delete_text": "项目“{title}”及其所有文件（包括导出的视频）将被删除，且无法恢复。",
  "storymaker.projects.deleted": "已删除“{title}”。",

  "storymaker.strip.title": "场景",
  "storymaker.strip.scene_n": "场景 {n}",
  "storymaker.strip.duration_s": "{seconds} 秒",
  "storymaker.strip.kind_code": "代码场景",
  "storymaker.strip.add_storyboard": "添加场景",
  "storymaker.strip.add_code": "添加代码场景",

  "storymaker.stage.compiling": "正在准备预览…",
  "storymaker.stage.unavailable": "预览不可用。",
  "storymaker.stage.player_error": "预览报告了一个问题。",
  "storymaker.stage.code_error": "某个代码场景有错误。请查看问题列表。",

  "storymaker.inspector.title": "场景设置",
  "storymaker.inspector.environment": "环境",
  "storymaker.inspector.variant": "时间段",
  "storymaker.inspector.variant_day": "白天",
  "storymaker.inspector.variant_night": "夜晚",
  "storymaker.inspector.narration": "旁白",
  "storymaker.inspector.narration_placeholder": "旁白者在这个场景里说什么？",
  "storymaker.inspector.narration_hint": "除非你设定时长，否则场景会持续到文字读完为止。",
  "storymaker.inspector.transition": "进入此场景的转场",
  "storymaker.inspector.transition_first": "第一个场景没有转场。",
  "storymaker.inspector.unsupported": "{name}（暂不支持）",
  "storymaker.inspector.duration": "时长",
  "storymaker.inspector.duration_auto": "自动",
  "storymaker.inspector.duration_fixed": "固定",
  "storymaker.inspector.duration_seconds": "时长（秒）",
  "storymaker.inspector.seconds_unit": "秒",
  "storymaker.inspector.cast_props": "此场景中的人物和道具",
  "storymaker.inspector.no_items": "暂时没有。请从素材库中挑选人物和道具。",
  "storymaker.inspector.remove": "移除",
  "storymaker.inspector.remove_item": "将 {name} 从场景中移除",
  "storymaker.inspector.move_up": "上移场景",
  "storymaker.inspector.move_down": "下移场景",
  "storymaker.inspector.delete_scene": "删除场景",
  "storymaker.inspector.delete_scene_text": "删除场景 {scene}？它的旁白和其中的所有内容都会从故事中移除。",
  "storymaker.inspector.code_scene_note": "这个场景由一个小程序绘制。",
  "storymaker.inspector.edit_code": "编辑代码",

  "storymaker.library.title": "素材库",
  "storymaker.library.people": "人物",
  "storymaker.library.props": "道具",
  "storymaker.library.environments": "场所",
  "storymaker.library.search": "搜索素材库",
  "storymaker.library.add": "添加到场景",
  "storymaker.library.use": "用作环境",
  "storymaker.library.empty": "没有匹配项。",
  "storymaker.library.code_scene": "代码场景自行绘制一切。",
  "storymaker.library.already_in_scene": "已经在这个场景里了。",
  "storymaker.library.added_actor": "{name} 已加入场景。",
  "storymaker.library.added_prop": "已将 {name} 添加到场景。",

  "storymaker.issues.title": "问题",
  "storymaker.issues.none": "没有问题。",
  "storymaker.issues.count": "问题：{count}",
  "storymaker.issues.show": "显示问题",
  "storymaker.issues.show_in_code": "在 story.json 中显示",

  "storymaker.issue.invalid_json": "JSON 无效",
  "storymaker.issue.unknown_field": "未知字段",
  "storymaker.issue.schema_mismatch": "故事格式不正确",
  "storymaker.issue.too_large": "太大",
  "storymaker.issue.limit_exceeded": "超出上限",
  "storymaker.issue.missing_field": "缺少字段",
  "storymaker.issue.invalid_value": "值无效",
  "storymaker.issue.duplicate_id": "名称重复",
  "storymaker.issue.unknown_name": "未知名称",
  "storymaker.issue.unknown_asset": "未知的素材项",
  "storymaker.issue.unknown_spot": "未知位置",
  "storymaker.issue.unknown_cast": "未知人物",
  "storymaker.issue.unknown_cue": "未知提示点",
  "storymaker.issue.canonicalised": "名称已规范化",
  "storymaker.issue.not_supported_yet": "暂不支持",
  "storymaker.issue.narration_missing": "没有旁白音频",
  "storymaker.issue.code_error": "代码场景出错",
  "storymaker.issue.deadline": "耗时过长",
  "storymaker.issue.layout_unverified": "布局未检查",
  "storymaker.issue.unknown": "问题",

  "storymaker.code.title": "代码",
  "storymaker.code.files": "文件",
  "storymaker.code.editor_label": "代码编辑器",
  "storymaker.code.save": "保存",
  "storymaker.code.saved": "已保存",
  "storymaker.code.unsaved": "有未保存的更改",
  "storymaker.code.stale": "已更改。保存后会重新检查。",
  "storymaker.code.new_file": "新文件，尚未保存",
  "storymaker.code.reload": "重新加载",
  "storymaker.code.bundle_ok": "代码场景已保存。",
  "storymaker.code.bundle_failed": "代码有错误。",
  "storymaker.code.no_problems": "未发现问题。",
  "storymaker.code.problem_count": "问题：{count}",
  "storymaker.code.line_col": "第 {line} 行，第 {column} 列",
  "storymaker.code.more": "另有 {count} 项",

  "storymaker.export.button": "导出",
  "storymaker.export.button_running": "正在导出 {percent} %",
  "storymaker.export.title": "导出视频",
  "storymaker.export.cap_ffmpeg": "FFmpeg",
  "storymaker.export.cap_browser": "浏览器",
  "storymaker.export.ready": "就绪",
  "storymaker.export.ready_version": "就绪，版本 {version}",
  "storymaker.export.ffmpeg_missing": "未找到。请安装 FFmpeg，或在 Video Studio 设置中指定其路径。",
  "storymaker.export.browser_ready": "就绪（{source}）",
  "storymaker.export.browser_missing": "没有可用于绘制画面的浏览器。",
  "storymaker.export.source_config": "来自配置",
  "storymaker.export.source_system": "已安装在此系统上",
  "storymaker.export.source_pinned": "由 AuraGo 下载",
  "storymaker.export.source_unknown": "来源未知",
  "storymaker.export.still_works": "没有它们，编辑和预览依然可用。",
  "storymaker.export.blocked_readonly": "桌面处于只读状态。",
  "storymaker.export.blocked_ffmpeg": "缺少 FFmpeg。",
  "storymaker.export.blocked_browser": "没有可用的浏览器。",
  "storymaker.export.blocked_errors": "故事有错误。请先修复。",
  "storymaker.export.blocked_too_long": "视频超过了 {max} 秒的上限。",
  "storymaker.export.blocked_not_ready": "预览仍在准备中。",
  "storymaker.export.summary": "时长 {duration}，{format}，{width} × {height} 像素。",
  "storymaker.export.silent_note": "此版本导出无声视频。旁白音频将在之后提供。",
  "storymaker.export.start": "开始导出",
  "storymaker.export.starting": "正在启动…",
  "storymaker.export.queued": "正在排队等候…",
  "storymaker.export.phase_starting": "启动中",
  "storymaker.export.phase_compiling": "正在准备故事",
  "storymaker.export.phase_mixing": "正在混音",
  "storymaker.export.phase_rendering": "正在绘制画面",
  "storymaker.export.phase_encoding": "正在编码视频",
  "storymaker.export.phase_writing": "正在保存文件",
  "storymaker.export.frames": "共 {total} 帧，已完成 {done} 帧",
  "storymaker.export.cancel": "取消导出",
  "storymaker.export.cancelling": "正在取消…",
  "storymaker.export.cancelled": "导出已取消。",
  "storymaker.export.interrupted": "导出被重启中断。",
  "storymaker.export.done": "导出完成",
  "storymaker.export.output": "视频位于你的项目文件夹中：",
  "storymaker.export.show_in_files": "在“文件”中显示",
  "storymaker.export.copy_path": "复制路径",
  "storymaker.export.copied": "已复制",
  "storymaker.export.copy_failed": "无法复制路径。",
  "storymaker.export.again": "再次导出",

  "storymaker.job_error.invalid_story": "故事有错误，因此无法导出。",
  "storymaker.job_error.storymaker_disabled": "Storymaker 已关闭。",
  "storymaker.job_error.browser_unavailable": "无法启动浏览器来绘制画面。",
  "storymaker.job_error.ffmpeg_unavailable": "FFmpeg 不可用。",
  "storymaker.job_error.render_failed": "导出失败。",
  "storymaker.job_error.render_timeout": "导出耗时过长，已被停止。",
  "storymaker.job_error.cancelled": "导出已取消。",
  "storymaker.job_error.busy": "另一个导出正在进行。请稍后再试。",
  "storymaker.job_error.code_error": "某个代码场景在绘制时失败。",
  "storymaker.job_error.unknown": "导出失败。",

  "storymaker.error.unauthorized": "你的会话已结束。请重新登录。",
  "storymaker.error.scope": "此登录无权执行该操作。",
  "storymaker.error.readonly": "桌面处于只读状态。",
  "storymaker.error.disabled": "Storymaker 已关闭。",
  "storymaker.error.invalid_story": "故事有错误，因此未保存。",
  "storymaker.error.conflict": "故事已在别处被更改。",
  "storymaker.error.required": "服务器需要故事的当前版本。请重新加载。",
  "storymaker.error.too_large": "内容太大。",
  "storymaker.error.not_found": "它已不存在。",
  "storymaker.error.busy": "服务器正忙。请稍后再试。",
  "storymaker.error.network": "无法连接到服务器。",
  "storymaker.error.generic": "出了点问题。"
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./ui -run 'TestStorymakerLocalesCoverAllSixteenLanguages|TestStorymakerAppTranslationsAreTranslated|TestTranslations_' -count=1`
Expected: ok

Recorded at drafting time:

```text
ok  	aurago/ui	1.177s
```

- [ ] **Step 5: Commit**

```bash
git add ui/desktop_storymaker_i18n_test.go ui/lang/storymaker
git commit -m "feat(storymaker): app strings in all 16 languages" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E5: Stylesheet

`css/desktop-app-storymaker.css`: `stm-` prefix, tokens only (`--vd-mat-*`, `--vd-*`, `--ds-*`), the window as a size container (the strip folds into a row below 940 px, the inspector into a sheet below 640 px), motion gated by `prefers-reduced-motion`.

**Files:**
- Create: `ui/css/desktop-app-storymaker.css`
- Test: `ui/desktop_storymaker_css_test.go`

- [ ] **Step 1: Write the failing test**

`ui/desktop_storymaker_css_test.go`:

```go
package ui

import (
	"regexp"
	"strings"
	"testing"
)

// splitSelectorList splits a selector list at its top-level commas (the commas inside :where(...) stay).
func splitSelectorList(list string) []string {
	var parts []string
	depth, start := 0, 0
	for i, r := range list {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, list[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, list[start:])
}

// The stylesheet takes every colour from tokens, scopes its rules under .stm-, uses the app materials, answers the window width
// with container queries and gates its motion.
func TestStorymakerStylesheetUsesOnlyTokens(t *testing.T) {
	t.Parallel()
	css := readDesktopAssetText(t, "css/desktop-app-storymaker.css")
	body := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	for label, pattern := range map[string]string{
		"hex colour":      `#[0-9a-fA-F]{3,8}\b`,
		"colour function": `\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(\s*[\d.]`,
		"named colour":    `(?i)[:\s(,](?:white|black|red|green|blue|gray|grey|orange|yellow|purple)\b\s*[;,)]`,
		"external url":    `url\(`,
		"import":          `@import`,
		"theme id":        `data-theme=`,
	} {
		if match := regexp.MustCompile(pattern).FindString(body); match != "" {
			t.Errorf("css/desktop-app-storymaker.css has a %s: %q", label, match)
		}
	}
	if strings.Count(body, "!important") != 1 || !strings.Contains(body, ".stm-app [hidden] { display: none !important; }") {
		t.Error("the only !important allowed is the [hidden] rule")
	}
	for _, marker := range []string{
		"var(--vd-mat-canvas", "var(--vd-mat-bar", "var(--vd-mat-side", "var(--vd-mat-card", "var(--vd-mat-well", "var(--vd-mat-field", "var(--vd-mat-btn",
		"var(--vd-mat-primary", "var(--vd-mat-selected", "var(--ds-color-success-fg)", "var(--ds-color-danger-fg)", "var(--ds-color-border-focus)",
		"container: stm / inline-size", "@container stm (max-width: 940px)", "@container stm (max-width: 640px)", "@media (pointer: coarse)",
	} {
		if !strings.Contains(body, marker) {
			t.Errorf("css/desktop-app-storymaker.css is missing %q", marker)
		}
	}
	// Every rule that starts a line is scoped to the app: its selectors name an .stm- class.
	selector := regexp.MustCompile(`(?m)^([^\s@/}][^{]*)\{`)
	for _, match := range selector.FindAllStringSubmatch(body, -1) {
		for _, part := range splitSelectorList(match[1]) {
			part = strings.TrimSpace(part)
			if part != "" && !strings.Contains(part, ".stm-") {
				t.Errorf("the selector %q is not scoped to .stm-", part)
			}
		}
	}
	gate := strings.Index(body, "@media (prefers-reduced-motion: no-preference)")
	if gate < 0 {
		t.Fatal("css/desktop-app-storymaker.css has no reduced-motion gate")
	}
	for _, motion := range regexp.MustCompile(`\b(?:transition|animation)\s*:`).FindAllStringIndex(body, -1) {
		if motion[0] < gate {
			t.Errorf("a transition or animation at offset %d sits outside @media (prefers-reduced-motion: no-preference)", motion[0])
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./ui -run 'TestStorymakerStylesheetUsesOnlyTokens' -count=1`
Expected: FAIL: FAIL (`css/desktop-app-storymaker.css` does not exist)

Recorded at drafting time (the replay of this task on a tree without it):

```text
--- FAIL: TestStorymakerStylesheetUsesOnlyTokens (0.00s)
    desktop_storymaker_css_test.go:33: read css/desktop-app-storymaker.css: open css/desktop-app-storymaker.css: The system cannot find the file specified.
FAIL
FAIL	aurago/ui	0.291s
FAIL
```

- [ ] **Step 3: Implement**

`ui/css/desktop-app-storymaker.css`:

```css
/* Storymaker: project list, toolbar, scene strip, stage with the player iframe and the scrubber, inspector and library, code tab,
   issues panel and the export panel. Tokens only (--vd-mat-*, --vd-*, --ds-*): no hex or literal colour functions outside
   color-mix(), so the app follows every desktop skin. The window is a size container (narrow windows and phones fold the strip
   into a row and the inspector into a sheet). Every transition sits inside @media (prefers-reduced-motion: no-preference) on
   selectors that start with body:not([data-animations="false"]). */
.stm-app {
    --stm-line: var(--vd-mat-divider, var(--vd-theme-border));
    --stm-ok: var(--ds-color-success-fg);
    --stm-warn: var(--ds-color-warning-fg);
    --stm-bad: var(--ds-color-danger-fg);
    --stm-ok-text: light-dark(color-mix(in srgb, var(--stm-ok) 30%, var(--vd-text)), var(--stm-ok));
    --stm-warn-text: light-dark(color-mix(in srgb, var(--stm-warn) 30%, var(--vd-text)), var(--stm-warn));
    --stm-bad-text: light-dark(color-mix(in srgb, var(--stm-bad) 34%, var(--vd-text)), color-mix(in srgb, var(--stm-bad) 88%, var(--vd-text)));
    --stm-mono: var(--vd-mat-mono-font, ui-monospace, SFMono-Regular, Consolas, monospace);
    container: stm / inline-size;
    position: relative; width: 100%; height: 100%; min-width: 0; min-height: 320px; overflow: hidden; isolation: isolate;
    color: var(--vd-text); background: var(--vd-mat-canvas, var(--vd-theme-app-bg)); font: 14px/1.45 var(--vd-font-ui, system-ui, sans-serif);
}
.stm-app *, .stm-app *::before, .stm-app *::after { box-sizing: border-box; }
.stm-app :where(button, input, select, textarea) { font: inherit; color: inherit; }
.stm-app [hidden] { display: none !important; }
.stm-app :is(button, input, select, textarea, [tabindex]):focus-visible { outline: var(--ds-focus-ring-width) solid var(--ds-color-border-focus); outline-offset: var(--ds-focus-ring-offset); }
.stm-app button:disabled, .stm-app select:disabled, .stm-app input:disabled, .stm-app textarea:disabled { opacity: .55; cursor: not-allowed; }
.stm-view { width: 100%; height: 100%; min-height: 0; }
.stm-muted { color: var(--vd-theme-muted); }
.stm-icon { width: 16px; height: 16px; flex: 0 0 auto; }
.stm-live { position: absolute; left: 50%; top: 56px; z-index: 60; max-width: min(420px, calc(100% - 24px)); transform: translateX(-50%); padding: 6px 12px; border: 1px solid var(--vd-mat-card-border, var(--vd-theme-border)); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-card, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-card-shadow, 0 6px 18px color-mix(in srgb, var(--vd-text) 18%, transparent)); pointer-events: none; }
.stm-live:empty { display: none; }
.stm-live[data-kind="error"] { border-color: var(--stm-bad); color: var(--stm-bad-text); }

/* Buttons, fields */
.stm-btn { display: inline-flex; align-items: center; justify-content: center; gap: 6px; min-height: 32px; padding: 0 12px; border: 1px solid var(--vd-mat-btn-border, var(--vd-theme-border)); border-radius: var(--vd-mat-btn-radius, var(--ds-radius-sm)); background: var(--vd-mat-btn, var(--vd-theme-control-bg)); box-shadow: var(--vd-mat-btn-shadow, none); color: var(--vd-mat-btn-text, inherit); font-weight: 600; white-space: nowrap; cursor: pointer; }
.stm-btn:hover:not(:disabled) { background: var(--vd-mat-btn-hover, var(--vd-theme-control-hover)); }
.stm-btn-primary { border-color: var(--vd-mat-primary-border, transparent); background: var(--vd-mat-primary, var(--vd-accent)); box-shadow: var(--vd-mat-primary-shadow, none); color: var(--vd-mat-primary-text, var(--vd-accent-contrast)); }
.stm-btn-primary:hover:not(:disabled) { background: var(--vd-mat-primary-hover, color-mix(in srgb, var(--vd-accent) 86%, var(--vd-text))); }
.stm-btn-quiet { border-color: transparent; background: transparent; box-shadow: none; }
.stm-btn-icon { width: 32px; padding: 0; }
.stm-field { display: grid; gap: 4px; min-width: 0; margin: 0 0 var(--ds-space-3); padding: 0; border: 0; }
.stm-label { color: var(--vd-mat-label-color, var(--vd-theme-muted)); font-size: 12px; font-weight: 600; letter-spacing: var(--vd-mat-label-tracking, normal); text-transform: var(--vd-mat-label-transform, none); }
.stm-hint { margin: 0; color: var(--vd-theme-muted); font-size: 12px; }
.stm-input, .stm-select, .stm-textarea { width: 100%; min-width: 0; padding: 0 8px; border: 1px solid var(--vd-mat-field-border, var(--vd-theme-border)); border-radius: var(--vd-mat-field-radius, var(--ds-radius-sm)); background: var(--vd-mat-field, var(--vd-theme-control-bg)); box-shadow: var(--vd-mat-field-shadow, none); }
.stm-input, .stm-select { height: 32px; }
.stm-input-narrow { width: 84px; }
.stm-textarea { min-height: 120px; padding: 6px 8px; line-height: 1.45; resize: vertical; }
.stm-row { display: flex; align-items: center; gap: var(--ds-space-2); }
.stm-actions { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2); }
.stm-spacer { flex: 1 1 auto; }
.stm-badge { display: inline-block; padding: 0 8px; border: 1px solid var(--stm-line); border-radius: var(--ds-radius-full); color: var(--vd-theme-muted); font-size: 12px; white-space: nowrap; }
.stm-dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: var(--vd-accent); }
.stm-dot-bad { background: var(--stm-bad); }
.stm-panel-title { margin: 0; padding: var(--ds-space-3) var(--ds-space-3) var(--ds-space-2); color: var(--vd-mat-heading-color, var(--vd-text)); font-family: var(--vd-mat-heading-font, inherit); font-size: 13px; font-weight: var(--vd-mat-heading-weight, 600); letter-spacing: var(--vd-mat-heading-tracking, normal); text-transform: var(--vd-mat-heading-transform, none); }

/* State cards, loading */
.stm-loading { padding: var(--ds-space-4); }
.stm-state { display: grid; place-content: center; justify-items: center; gap: 12px; height: 100%; padding: 24px; text-align: center; }
.stm-state h2, .stm-state p { margin: 0; max-width: 46ch; }
.stm-state-icon { width: 34px; height: 34px; color: var(--vd-theme-muted); }

/* Start screen */
.stm-start { display: grid; align-content: start; gap: var(--ds-space-3); height: 100%; padding: var(--ds-space-4); overflow: auto; }
.stm-projects { display: grid; gap: var(--ds-space-4); max-width: 880px; width: 100%; margin: 0 auto; }
.stm-projects-head h2 { margin: 0; color: var(--vd-mat-heading-color, var(--vd-text)); font-family: var(--vd-mat-heading-font, inherit); font-size: 20px; font-weight: var(--vd-mat-heading-weight, 600); letter-spacing: var(--vd-mat-heading-tracking, normal); text-transform: var(--vd-mat-heading-transform, none); }
.stm-projects-head p { margin: 4px 0 0; }
.stm-new { display: grid; grid-template-columns: minmax(0, 2fr) auto auto; align-items: end; gap: var(--ds-space-3); padding: var(--ds-space-3); border: 1px solid var(--vd-mat-card-border, var(--vd-theme-border)); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-card, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-card-shadow, none); }
.stm-new .stm-field { margin: 0; }
.stm-form-error { grid-column: 1 / -1; margin: 0; color: var(--stm-bad-text); }
.stm-chips { display: flex; flex-wrap: wrap; gap: 4px; }
.stm-chip { min-height: 32px; padding: 0 12px; border: 1px solid var(--vd-mat-btn-border, var(--vd-theme-border)); border-radius: var(--ds-radius-full); background: var(--vd-mat-btn, var(--vd-theme-control-bg)); color: var(--vd-mat-btn-text, inherit); cursor: pointer; }
.stm-chip[aria-pressed="true"] { border-color: var(--vd-mat-selected-color, var(--vd-theme-accent-border)); background: var(--vd-mat-selected, var(--vd-theme-accent-soft)); box-shadow: var(--vd-mat-selected-edge, none); color: var(--vd-mat-selected-text, inherit); font-weight: 600; }
.stm-project-list { display: grid; gap: var(--ds-space-2); margin: 0; padding: 0; list-style: none; }
.stm-project { display: flex; align-items: stretch; gap: 4px; border: 1px solid var(--vd-mat-card-border, var(--vd-theme-border)); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-card, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-card-shadow, none); }
.stm-project:hover { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
.stm-project > .stm-btn { align-self: center; margin-right: var(--ds-space-2); }
.stm-project-open { display: grid; flex: 1 1 auto; min-width: 0; gap: 2px; padding: var(--ds-space-3); border: 0; border-radius: inherit; background: transparent; text-align: left; cursor: pointer; }
.stm-project-title { overflow: hidden; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.stm-project-meta { color: var(--vd-theme-muted); font-size: 12px; }
.stm-empty { display: grid; justify-items: center; gap: 6px; padding: var(--ds-space-5, 32px) var(--ds-space-3); text-align: center; color: var(--vd-theme-muted); }
.stm-empty h3, .stm-empty p { margin: 0; }
.stm-empty-icon { width: 36px; height: 36px; }

/* Editor shell */
.stm-editor-app { display: grid; grid-template-rows: auto auto minmax(0, 1fr); height: 100%; min-height: 0; }
.stm-editor-app > .stm-bar { grid-row: 1; }
.stm-editor-app > .stm-banners { grid-row: 2; }
.stm-editor-app > .stm-body { grid-row: 3; }
.stm-bar { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2) var(--ds-space-3); min-height: 48px; padding: var(--ds-space-2) var(--ds-space-3); border-bottom: 1px solid var(--stm-line); background: var(--vd-mat-bar, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-bar-edge, none); }
.stm-title-group { display: flex; align-items: center; gap: var(--ds-space-2); min-width: 0; }
.stm-title { margin: 0; overflow: hidden; color: var(--vd-mat-heading-color, var(--vd-text)); font-family: var(--vd-mat-heading-font, inherit); font-size: 15px; font-weight: var(--vd-mat-heading-weight, 600); letter-spacing: var(--vd-mat-heading-tracking, normal); text-overflow: ellipsis; text-transform: var(--vd-mat-heading-transform, none); white-space: nowrap; }
.stm-save { color: var(--vd-theme-muted); font-size: 12px; white-space: nowrap; }
.stm-save[data-state="dirty"], .stm-save[data-state="saving"] { color: var(--stm-warn-text); }
.stm-save[data-state="error"], .stm-save[data-state="conflict"], .stm-save[data-state="offline"] { color: var(--stm-bad-text); }
.stm-tabs { display: inline-flex; gap: 2px; padding: 2px; border: 1px solid var(--stm-line); border-radius: var(--vd-mat-btn-radius, var(--ds-radius-sm)); background: var(--vd-mat-well, var(--vd-theme-control-bg)); }
.stm-tab { display: inline-flex; align-items: center; gap: 6px; min-height: 28px; padding: 0 10px; border: 1px solid transparent; border-radius: var(--ds-radius-xs, 4px); background: transparent; font-weight: 500; cursor: pointer; }
.stm-tab:hover { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
.stm-tab[aria-selected="true"] { border-color: var(--vd-mat-selected-color, var(--vd-theme-accent-border)); background: var(--vd-mat-selected, var(--vd-theme-accent-soft)); box-shadow: var(--vd-mat-selected-edge, none); color: var(--vd-mat-selected-text, inherit); font-weight: 600; }
.stm-tabs-small { margin: var(--ds-space-2) var(--ds-space-3); align-self: start; }
.stm-issues-btn[data-tone="bad"] { color: var(--stm-bad-text); }
.stm-issues-btn[data-tone="warn"] { color: var(--stm-warn-text); }
.stm-banners:empty { display: none; }
.stm-banners { display: grid; gap: 1px; }
.stm-banner { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2) var(--ds-space-3); padding: 6px var(--ds-space-3); border-bottom: 1px solid var(--stm-line); background: color-mix(in srgb, var(--stm-warn) 12%, var(--vd-mat-bar, var(--vd-theme-panel-bg))); }
.stm-banner[data-tone="bad"] { background: color-mix(in srgb, var(--stm-bad) 14%, var(--vd-mat-bar, var(--vd-theme-panel-bg))); }
.stm-banner > span:first-of-type { flex: 1 1 280px; }

/* Body: strip | center | side */
.stm-body { display: grid; grid-template-columns: 208px minmax(0, 1fr) 328px; min-height: 0; }
.stm-strip { display: flex; flex-direction: column; min-height: 0; border-right: 1px solid var(--stm-line); background: var(--vd-mat-side, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-side-edge, none); }
.stm-scene-list { display: grid; flex: 1 1 auto; align-content: start; gap: 6px; margin: 0; padding: 0 var(--ds-space-2) var(--ds-space-2); overflow: auto; list-style: none; }
.stm-scene-btn { position: relative; display: grid; grid-template-columns: 1fr auto; gap: 2px 6px; width: 100%; padding: 6px; border: 1px solid transparent; border-radius: var(--ds-radius-sm); background: transparent; text-align: left; cursor: pointer; }
.stm-scene-btn:hover { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
.stm-scene-btn[aria-current="true"] { border-color: var(--vd-mat-selected-color, var(--vd-theme-accent-border)); background: var(--vd-mat-selected, var(--vd-theme-accent-soft)); box-shadow: var(--vd-mat-selected-edge, none); color: var(--vd-mat-selected-text, inherit); }
.stm-thumb { grid-column: 1 / -1; width: 100%; height: auto; aspect-ratio: 16 / 9; border-radius: var(--ds-radius-xs, 4px); background: var(--vd-mat-well, var(--vd-theme-control-bg)); object-fit: cover; }
.stm-scene-num { position: absolute; top: 10px; left: 10px; min-width: 20px; padding: 0 5px; border-radius: var(--ds-radius-full); background: color-mix(in srgb, var(--vd-text) 62%, transparent); color: var(--vd-theme-app-bg); font-size: 11px; font-weight: 700; text-align: center; }
.stm-scene-kind { position: absolute; top: 10px; right: 10px; display: inline-grid; place-items: center; width: 20px; height: 20px; border-radius: var(--ds-radius-full); background: color-mix(in srgb, var(--vd-text) 62%, transparent); color: var(--vd-theme-app-bg); }
.stm-scene-kind .stm-icon { width: 12px; height: 12px; }
.stm-scene-name { overflow: hidden; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.stm-scene-dur { font-size: 12px; opacity: .75; font-variant-numeric: tabular-nums; }
.stm-strip-add { display: grid; gap: 4px; padding: var(--ds-space-2); border-top: 1px solid var(--stm-line); }

/* Center: stage, scrubber, code */
.stm-center { position: relative; display: grid; grid-template-rows: minmax(0, 1fr); min-width: 0; min-height: 0; }
.stm-stage-wrap { display: grid; grid-template-rows: minmax(0, 1fr) auto; min-width: 0; min-height: 0; }
.stm-stage { container-type: size; position: relative; display: grid; place-items: center; min-width: 0; min-height: 0; padding: var(--ds-space-3); background: var(--vd-mat-well, var(--vd-theme-app-bg)); box-shadow: var(--vd-mat-well-shadow, none); }
.stm-frame { position: relative; width: min(100cqw, calc(100cqh * var(--stm-ar, 1.7778))); aspect-ratio: var(--stm-ar, 1.7778); overflow: hidden; border: 1px solid var(--stm-line); border-radius: var(--ds-radius-sm); background: color-mix(in srgb, var(--vd-text) 14%, transparent); }
.stm-player { position: absolute; inset: 0; width: 100%; height: 100%; border: 0; background: transparent; }
.stm-stage-note { position: absolute; left: 50%; bottom: var(--ds-space-3); z-index: 2; max-width: calc(100% - 24px); padding: 4px 12px; border: 1px solid var(--stm-line); border-radius: var(--ds-radius-full); background: var(--vd-mat-card, var(--vd-theme-panel-bg)); transform: translateX(-50%); font-size: 12px; }
.stm-transport { display: flex; align-items: center; gap: var(--ds-space-3); padding: var(--ds-space-2) var(--ds-space-3); border-top: 1px solid var(--stm-line); background: var(--vd-mat-bar, var(--vd-theme-panel-bg)); }
.stm-clock { min-width: 5ch; font: 13px/1 var(--stm-mono); font-variant-numeric: tabular-nums; }
.stm-clock-total { color: var(--vd-theme-muted); }
.stm-scrub { position: relative; flex: 1 1 auto; height: 28px; min-width: 80px; border: 1px solid var(--vd-mat-field-border, var(--vd-theme-border)); border-radius: var(--ds-radius-sm); background: var(--vd-mat-field, var(--vd-theme-control-bg)); cursor: pointer; touch-action: none; }
.stm-scrub[aria-disabled="true"] { cursor: default; opacity: .6; }
.stm-scrub-blocks { position: absolute; inset: 2px; overflow: hidden; border-radius: var(--ds-radius-xs, 4px); pointer-events: none; }
.stm-scrub-block { position: absolute; top: 0; bottom: 0; min-width: 2px; border-right: 1px solid var(--vd-mat-field, var(--vd-theme-control-bg)); background: color-mix(in srgb, var(--vd-accent) 24%, transparent); }
.stm-scrub-block:nth-child(even) { background: color-mix(in srgb, var(--vd-accent) 36%, transparent); }
.stm-scrub-block[data-kind="code"] { background: repeating-linear-gradient(135deg, color-mix(in srgb, var(--vd-accent) 34%, transparent) 0 6px, color-mix(in srgb, var(--vd-accent) 18%, transparent) 6px 12px); }
.stm-scrub-block[data-selected="true"] { background: color-mix(in srgb, var(--vd-accent) 55%, transparent); }
.stm-scrub-head { position: absolute; top: -3px; bottom: -3px; width: 3px; margin-left: -1px; border-radius: 2px; background: var(--vd-text); pointer-events: none; }

/* Side: inspector, library */
.stm-side { display: flex; flex-direction: column; min-height: 0; border-left: 1px solid var(--stm-line); background: var(--vd-mat-side, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-side-edge, none); overflow: auto; }
.stm-app [data-sidetab="scene"] .stm-library, .stm-app [data-sidetab="library"] .stm-inspector { display: none; }
.stm-inspector-body, .stm-library { padding: 0 var(--ds-space-3) var(--ds-space-3); }
.stm-inspector .stm-panel-title, .stm-library .stm-panel-title { display: none; }
.stm-inspector-head { display: flex; align-items: center; justify-content: space-between; gap: var(--ds-space-2); margin-bottom: var(--ds-space-3); }
.stm-inspector-title { display: flex; align-items: baseline; gap: var(--ds-space-2); min-width: 0; }
.stm-inspector-tools { display: flex; gap: 2px; }
.stm-note { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2); margin-bottom: var(--ds-space-3); padding: var(--ds-space-2) var(--ds-space-3); border: 1px dashed var(--stm-line); border-radius: var(--ds-radius-sm); }
.stm-note span { flex: 1 1 160px; }
.stm-items { display: grid; gap: 4px; margin: 0; padding: 0; list-style: none; }
.stm-item { display: flex; align-items: center; gap: var(--ds-space-2); padding: 2px 4px 2px 8px; border: 1px solid var(--stm-line); border-radius: var(--ds-radius-sm); background: var(--vd-mat-card, var(--vd-theme-panel-bg)); }
.stm-item-name { flex: 1 1 auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.stm-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(92px, 1fr)); gap: var(--ds-space-2); margin: var(--ds-space-2) 0 0; padding: 0; list-style: none; }
.stm-card { position: relative; display: grid; justify-items: center; gap: 4px; width: 100%; padding: 6px; border: 1px solid var(--vd-mat-card-border, var(--vd-theme-border)); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-card, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-card-shadow, none); cursor: pointer; }
.stm-card:hover:not(:disabled) { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
.stm-card[data-present="true"] { border-color: var(--vd-mat-selected-color, var(--vd-theme-accent-border)); }
.stm-card-thumb { width: 72px; height: 72px; object-fit: contain; }
.stm-card-name { max-width: 100%; overflow: hidden; font-size: 12px; text-align: center; text-overflow: ellipsis; white-space: nowrap; }
.stm-card-mark { position: absolute; top: 4px; right: 4px; display: inline-grid; place-items: center; width: 18px; height: 18px; border-radius: 50%; background: var(--vd-mat-primary, var(--vd-accent)); color: var(--vd-mat-primary-text, var(--vd-accent-contrast)); }
.stm-card-mark .stm-icon { width: 12px; height: 12px; }

/* Issues panel */
.stm-issues { position: absolute; left: 0; right: 0; bottom: 0; z-index: 20; display: grid; grid-template-rows: auto minmax(0, 1fr); max-height: min(60%, 360px); border-top: 1px solid var(--stm-line); background: var(--vd-mat-card, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-card-shadow, 0 -8px 22px color-mix(in srgb, var(--vd-text) 16%, transparent)); }
.stm-issues-head { display: flex; align-items: center; justify-content: space-between; padding: 4px var(--ds-space-3); }
.stm-issues-head h3 { margin: 0; font-size: 13px; }
.stm-issues > .stm-muted { margin: 0; padding: 0 var(--ds-space-3) var(--ds-space-3); }
.stm-issue-list { display: grid; margin: 0; padding: 0 var(--ds-space-2) var(--ds-space-2); overflow: auto; list-style: none; }
.stm-issue { display: flex; align-items: flex-start; gap: 4px; }
.stm-issue-go { display: flex; flex: 1 1 auto; align-items: flex-start; gap: var(--ds-space-2); min-width: 0; padding: 6px; border: 0; border-radius: var(--ds-radius-sm); background: transparent; text-align: left; cursor: pointer; }
.stm-issue-go:hover { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
.stm-issue[data-severity="error"] .stm-icon { color: var(--stm-bad-text); }
.stm-issue[data-severity="warning"] .stm-icon { color: var(--stm-warn-text); }
.stm-issue-body { display: flex; flex-wrap: wrap; align-items: baseline; gap: 2px var(--ds-space-2); min-width: 0; }
.stm-issue-text { flex-basis: 100%; overflow-wrap: anywhere; color: var(--vd-theme-muted); font-size: 12px; }
.stm-issue-fix { flex-basis: 100%; overflow-wrap: anywhere; font-family: var(--stm-mono); font-size: 12px; }

/* Code tab */
.stm-code { display: grid; grid-template-columns: 188px minmax(0, 1fr); min-width: 0; min-height: 0; }
.stm-code-nav { border-right: 1px solid var(--stm-line); background: var(--vd-mat-side, var(--vd-theme-panel-bg)); overflow: auto; }
.stm-code-files { display: grid; gap: 2px; margin: 0; padding: 0 var(--ds-space-2) var(--ds-space-2); list-style: none; }
.stm-code-file-btn { display: flex; align-items: center; gap: 6px; width: 100%; min-height: 32px; padding: 0 8px; border: 1px solid transparent; border-radius: var(--ds-radius-sm); background: transparent; text-align: left; cursor: pointer; }
.stm-code-file-btn:hover { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
.stm-code-file-btn[aria-current="true"] { border-color: var(--vd-mat-selected-color, var(--vd-theme-accent-border)); background: var(--vd-mat-selected, var(--vd-theme-accent-soft)); box-shadow: var(--vd-mat-selected-edge, none); color: var(--vd-mat-selected-text, inherit); }
.stm-code-file-name { flex: 1 1 auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.stm-code-main { display: grid; grid-template-rows: auto minmax(0, 1fr) auto; min-width: 0; min-height: 0; }
.stm-code-bar { display: flex; flex-wrap: wrap; align-items: center; gap: var(--ds-space-2) var(--ds-space-3); padding: var(--ds-space-2) var(--ds-space-3); border-bottom: 1px solid var(--stm-line); background: var(--vd-mat-bar, var(--vd-theme-panel-bg)); }
.stm-code-state { color: var(--vd-theme-muted); font-size: 12px; }
.stm-code-state[data-state="dirty"] { color: var(--stm-warn-text); }
.stm-editor { min-width: 0; min-height: 0; background: var(--vd-mat-well, var(--vd-theme-app-bg)); }
.stm-editor-area, .stm-editor-cm { width: 100%; height: 100%; min-height: 0; }
.stm-editor-cm .cm-editor { height: 100%; }
.stm-editor-text { display: block; width: 100%; height: 100%; padding: 8px; border: 0; background: transparent; color: var(--vd-text); font: 13px/1.5 var(--stm-mono); resize: none; tab-size: 2; }
.stm-problems { max-height: 180px; overflow: auto; border-top: 1px solid var(--stm-line); background: var(--vd-mat-card, var(--vd-theme-panel-bg)); }
.stm-problems p { margin: 0; padding: 6px var(--ds-space-3); }
.stm-problems-list { display: grid; margin: 0; padding: 0 var(--ds-space-2) var(--ds-space-2); list-style: none; }
.stm-problem-go { display: flex; align-items: flex-start; gap: var(--ds-space-2); width: 100%; padding: 4px 6px; border: 0; border-radius: var(--ds-radius-sm); background: transparent; text-align: left; cursor: pointer; }
.stm-problem-go:hover { background: var(--vd-mat-hover, var(--vd-theme-control-hover)); }
.stm-problem[data-severity="error"] .stm-icon { color: var(--stm-bad-text); }
.stm-problem[data-severity="warning"] .stm-icon { color: var(--stm-warn-text); }
.stm-problem-line { flex: 0 0 auto; color: var(--vd-theme-muted); font: 12px/1.5 var(--stm-mono); }
.stm-problem-text { min-width: 0; overflow-wrap: anywhere; }

/* Export */
.stm-export { position: relative; }
.stm-export-btn { position: relative; overflow: hidden; }
.stm-export-btn[data-state="running"]::before { content: ""; position: absolute; inset: 0 auto 0 0; width: var(--stm-export-progress, 0%); background: color-mix(in srgb, var(--vd-text) 22%, transparent); pointer-events: none; }
.stm-export-panel { position: absolute; right: 0; top: calc(100% + 8px); z-index: 40; display: grid; gap: var(--ds-space-3); width: min(380px, calc(100cqw - 16px)); max-height: min(70vh, 560px); padding: var(--ds-space-3); overflow: auto; border: 1px solid var(--vd-mat-card-border, var(--vd-theme-border)); border-radius: var(--vd-mat-card-radius, var(--ds-radius-md)); background: var(--vd-mat-card, var(--vd-theme-panel-bg)); box-shadow: var(--vd-mat-card-shadow, 0 12px 32px color-mix(in srgb, var(--vd-text) 22%, transparent)); }
.stm-export-head { display: flex; align-items: center; justify-content: space-between; }
.stm-export-head h3 { margin: 0; font-size: 15px; }
.stm-caps { display: grid; gap: var(--ds-space-2); margin: 0; padding: 0; list-style: none; }
.stm-cap { display: flex; align-items: flex-start; gap: var(--ds-space-2); }
.stm-cap[data-ok="true"] .stm-icon { color: var(--stm-ok-text); }
.stm-cap[data-ok="false"] .stm-icon { color: var(--stm-warn-text); }
.stm-cap-text { display: grid; gap: 2px; min-width: 0; }
.stm-cap-issue { display: block; padding: 4px 6px; overflow-wrap: anywhere; border-radius: var(--ds-radius-xs, 4px); background: var(--vd-mat-well, var(--vd-theme-control-bg)); font: 12px/1.4 var(--stm-mono); }
.stm-job { display: grid; gap: var(--ds-space-2); }
.stm-job p { margin: 0; }
.stm-job-line { display: flex; align-items: baseline; justify-content: space-between; gap: var(--ds-space-2); }
.stm-job-title { display: flex; align-items: center; gap: var(--ds-space-2); }
.stm-job-title .stm-icon { color: var(--stm-ok-text); }
.stm-job-bad .stm-icon { color: var(--stm-bad-text); }
.stm-blockers { display: grid; gap: 4px; margin: 0; padding-left: 18px; color: var(--stm-warn-text); }
.stm-path { display: block; padding: 6px 8px; overflow-wrap: anywhere; border-radius: var(--ds-radius-xs, 4px); background: var(--vd-mat-well, var(--vd-theme-control-bg)); font: 12px/1.4 var(--stm-mono); user-select: all; }
progress.stm-progress { width: 100%; height: 8px; border: 0; border-radius: var(--ds-radius-full); background: var(--vd-mat-well, var(--vd-theme-control-bg)); -webkit-appearance: none; appearance: none; overflow: hidden; }
progress.stm-progress::-webkit-progress-bar { background: var(--vd-mat-well, var(--vd-theme-control-bg)); }
progress.stm-progress::-webkit-progress-value { background: var(--vd-mat-primary, var(--vd-accent)); }
progress.stm-progress::-moz-progress-bar { background: var(--vd-mat-primary, var(--vd-accent)); }

/* The code tab has the whole body: the strip and the inspector belong to the storyboard */
.stm-editor-app[data-tab="code"] .stm-body { grid-template-columns: minmax(0, 1fr); grid-template-rows: minmax(0, 1fr); }
.stm-editor-app[data-tab="code"] :is(.stm-strip, .stm-side) { display: none; }

/* Medium windows: the strip becomes a row above the stage */
@container stm (max-width: 940px) {
    .stm-body { grid-template-columns: minmax(0, 1fr) 296px; grid-template-rows: auto minmax(0, 1fr); }
    .stm-strip { grid-column: 1 / -1; flex-direction: row; align-items: stretch; border-right: 0; border-bottom: 1px solid var(--stm-line); }
    .stm-strip .stm-panel-title { display: none; }
    .stm-scene-list { display: flex; padding: var(--ds-space-2); overflow-x: auto; overflow-y: hidden; }
    .stm-scene { flex: 0 0 132px; }
    .stm-strip-add { display: flex; flex-direction: column; justify-content: center; border-top: 0; border-left: 1px solid var(--stm-line); }
    .stm-code { grid-template-columns: 148px minmax(0, 1fr); }
    .stm-new { grid-template-columns: minmax(0, 1fr); }
}
/* Narrow windows and phones: one column, the inspector is a sheet below the stage */
@container stm (max-width: 640px) {
    .stm-body { grid-template-columns: minmax(0, 1fr); grid-template-rows: auto minmax(0, 1fr) minmax(120px, 42%); }
    .stm-side { grid-column: 1; border-left: 0; border-top: 1px solid var(--stm-line); }
    .stm-export { position: static; }
    .stm-export-panel { inset: auto 8px 8px 8px; width: auto; max-height: 70vh; }
    .stm-bar { gap: var(--ds-space-2); }
    .stm-bar [data-stm="back"] span { display: none; }
    .stm-transport { gap: var(--ds-space-2); }
    .stm-code { grid-template-columns: minmax(0, 1fr); grid-template-rows: auto minmax(0, 1fr); }
    .stm-code-nav { border-right: 0; border-bottom: 1px solid var(--stm-line); }
    .stm-code-nav .stm-panel-title { display: none; }
    .stm-code-files { display: flex; overflow-x: auto; }
}
@media (pointer: coarse) {
    .stm-btn, .stm-tab, .stm-chip, .stm-input, .stm-select, .stm-code-file-btn { min-height: 44px; }
    .stm-btn-icon { width: 44px; }
    .stm-scrub { height: 40px; }
}
@media (prefers-reduced-motion: no-preference) {
    body:not([data-animations="false"]) .stm-app :is(.stm-btn, .stm-tab, .stm-chip, .stm-card, .stm-scene-btn, .stm-project, .stm-issue-go, .stm-problem-go, .stm-code-file-btn) { transition: background-color .12s ease, border-color .12s ease, box-shadow .12s ease; }
    body:not([data-animations="false"]) .stm-app .stm-export-panel, body:not([data-animations="false"]) .stm-app .stm-issues { animation: stm-rise .14s ease-out; }
    body:not([data-animations="false"]) .stm-app .stm-export-btn::before { transition: width .25s linear; }
    @keyframes stm-rise { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: none; } }
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./ui -run 'TestStorymakerStylesheetUsesOnlyTokens' -count=1`
Expected: ok

Recorded at drafting time:

```text
ok  	aurago/ui	0.249s
```

- [ ] **Step 5: Commit**

```bash
git add ui/desktop_storymaker_css_test.go ui/css/desktop-app-storymaker.css
git commit -m "feat(storymaker): theme-native stylesheet for the Storymaker app" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E6: Registration in the desktop shell

Module-loader entry (scripts in load order, stylesheet, i18n section), the global name `StorymakerApp`, the render dispatch (the window hands over translation, dialogs, the read-only state, `openApp`, `setWindowBeforeClose`), window size, minimum size and the mobile-wide flag, then `npm run build:ui` for `main.bundle.js`.

**Files:**
- Modify: `ui/js/desktop/core/module-loader.js` (after base line 32 (the new entry, before `minigolf`) and after base line 677 (`APP_I18N_SECTIONS`, after `system-world`))
- Modify: `ui/js/desktop/core/desktop-foundation.js` (after base line 614, `ecalc: 'ECalcApp'`)
- Modify: `ui/js/desktop/core/menus-and-routing.js` (after base line 1770, before the `tresor` dispatch)
- Modify: `ui/js/desktop/core/window-shell-runtime.js` (after base line 1322 (size), base line 1357 (mobile-wide flag), after base line 1365 (minimum size))
- Regenerate: `ui/js/desktop/bundles/main.bundle.js` (`npm run build:ui`)
- Test: `ui/desktop_storymaker_registration_test.go`

- [ ] **Step 0: GitNexus impact (project rule)**

```bash
node C:/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs impact "renderAppContent" --direction upstream --repo .
node C:/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs impact "appWindowSize" --direction upstream --repo .
```
Expected: both are shell registries that gain one entry each; no behaviour of another app changes. Treat an empty answer as `UNKNOWN` and confirm with `grep -n "renderAppContent(" ui/js/desktop/core/*.js`.

- [ ] **Step 1: Write the failing test**

`ui/desktop_storymaker_registration_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"aurago/internal/desktop"
)

// storymakerModules is the load order of the app: the shared core first, the composition root last.
var storymakerModules = []string{"core", "model", "api", "assets", "player", "projects", "strip", "inspector", "code", "export"}

func TestDesktopStorymakerRegistration(t *testing.T) {
	t.Parallel()
	var app *desktop.AppManifest
	for _, candidate := range desktop.BuiltinApps() {
		if candidate.ID == "storymaker" {
			found := candidate
			app = &found
		}
	}
	if app == nil {
		t.Fatal("storymaker is not a builtin app")
	}
	if app.Name != "Storymaker" || app.Category != "creative" || app.Entry != "builtin://storymaker" || strings.Join(app.Requires, ",") != "storymaker" || !app.StartVisible {
		t.Fatalf("manifest = %+v", *app)
	}
	if app.Metadata["open_maximized"] != "true" {
		t.Fatalf("metadata = %v, want open_maximized", app.Metadata)
	}
	logo := app.Metadata["logo_path"]
	if logo != "/img/desktop-icons/storymaker.svg" {
		t.Fatalf("logo_path = %q", logo)
	}
	svg := readDesktopAssetText(t, strings.TrimPrefix(logo, "/"))
	if !strings.HasPrefix(svg, "<svg ") || strings.Contains(svg, "href") || strings.Contains(svg, "<script") || strings.Contains(svg, "<image") {
		t.Fatalf("the Storymaker logo must be a self-contained SVG: %.120s", svg)
	}

	loader := rawDesktopAssetText(t, "js/desktop/core/module-loader.js")
	start := strings.Index(loader, "        storymaker: {")
	if start < 0 {
		t.Fatal("module-loader.js has no storymaker entry in DESKTOP_APP_ASSETS")
	}
	entry := loader[start : start+strings.Index(loader[start:], "\n        },")]
	if !strings.Contains(entry, "appStyles('/css/desktop-app-storymaker.css')") {
		t.Error("the storymaker loader entry must load css/desktop-app-storymaker.css")
	}
	offset := 0
	for _, module := range append(append([]string{}, storymakerModules...), "") {
		script := "/js/desktop/apps/storymaker-" + module + ".js'"
		if module == "" {
			script = "/js/desktop/apps/storymaker.js'"
		}
		index := strings.Index(entry[offset:], script)
		if index < 0 {
			t.Fatalf("the storymaker loader entry lists %s out of order or not at all (the core first, the composition root last)", script)
		}
		offset += index + len(script)
	}
	if !strings.Contains(loader, "storymaker: ['storymaker'],") {
		t.Error("APP_I18N_SECTIONS has no storymaker section")
	}
	foundation := rawDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	routing := rawDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	shell := rawDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	bundle := rawDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	for _, m := range []struct{ name, source, marker string }{
		{"foundation", foundation, "storymaker: 'StorymakerApp',"},
		{"routing", routing, "if (appId === 'storymaker') {"},
		{"routing", routing, "window.AuraDesktopModules.loadAppScript('storymaker')"},
		{"routing", routing, "window.StorymakerApp.render(contentEl(id), id, Object.assign({}, context || {}, {"},
		{"size", shell, "storymaker: { width: 1320, height: 840 },"},
		{"min size", shell, "if (appId === 'storymaker') return { width: 640, height: 480 };"},
		{"mobile", shell, "storymaker: true }[appId]"},
	} {
		if !strings.Contains(m.source, m.marker) {
			t.Errorf("%s lacks %q", m.name, m.marker)
		}
		if !strings.Contains(bundle, m.marker) {
			t.Errorf("main.bundle.js lacks the %s marker %q (run npm run build:ui)", m.name, m.marker)
		}
	}
	// The window hands the app what it needs: translation, dialogs, read-only state, the way to open Files.
	for _, ctx := range []string{"readonly: desktopReadonly()", "openApp, confirmDialog", "setWindowBeforeClose:"} {
		if !strings.Contains(routing, ctx) {
			t.Errorf("the storymaker render dispatch does not pass %q", ctx)
		}
	}
	for _, lang := range storymakerLanguages {
		if strings.TrimSpace(readMergedLangMap(t, lang)["desktop.app_storymaker"]) == "" {
			t.Errorf("%s lacks desktop.app_storymaker", lang)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./ui -run 'TestDesktopStorymakerRegistration' -count=1`
Expected: FAIL: FAIL (`module-loader.js has no storymaker entry in DESKTOP_APP_ASSETS`)

Recorded at drafting time (the replay of this task on a tree without it):

```text
--- FAIL: TestDesktopStorymakerRegistration (0.00s)
    desktop_storymaker_registration_test.go:43: module-loader.js has no storymaker entry in DESKTOP_APP_ASSETS
FAIL
FAIL	aurago/ui	0.337s
FAIL
```

- [ ] **Step 3: Implement**

`ui/js/desktop/core/module-loader.js` (after base line 32 (the new entry, before `minigolf`) and after base line 677 (`APP_I18N_SECTIONS`, after `system-world`)):

```diff
--- a/ui/js/desktop/core/module-loader.js
+++ b/ui/js/desktop/core/module-loader.js
@@ -30,6 +30,10 @@
             styles: appStyles('/css/desktop-app-ecalc.css'),
             scripts: ['/js/desktop/apps/ecalc/ecalc-expr.js', '/js/desktop/apps/ecalc/ecalc-units.js', '/js/desktop/apps/ecalc/ecalc-eseries.js', '/js/desktop/apps/ecalc/ecalc-solve.js', '/js/desktop/apps/ecalc/ecalc-typeset.js', '/js/desktop/apps/ecalc/ecalc-symbols.js', '/js/desktop/apps/ecalc/ecalc-schematics.js', '/js/desktop/apps/ecalc/ecalc-store.js', '/js/desktop/apps/ecalc/ecalc-calc-page.js', '/js/desktop/apps/ecalc/ecalc-shell.js', '/js/desktop/apps/ecalc/ecalc.js']
         },
+        storymaker: {
+            styles: appStyles('/css/desktop-app-storymaker.css'),
+            scripts: ['/js/desktop/apps/storymaker-core.js', '/js/desktop/apps/storymaker-model.js', '/js/desktop/apps/storymaker-api.js', '/js/desktop/apps/storymaker-assets.js', '/js/desktop/apps/storymaker-player.js', '/js/desktop/apps/storymaker-projects.js', '/js/desktop/apps/storymaker-strip.js', '/js/desktop/apps/storymaker-inspector.js', '/js/desktop/apps/storymaker-code.js', '/js/desktop/apps/storymaker-export.js', '/js/desktop/apps/storymaker.js']
+        },
         'minigolf': {
             styles: appStyles('/css/desktop-app-minigolf.css'),
             scripts: ['/js/desktop/apps/minigolf.js']
@@ -675,6 +679,7 @@
         'bluetooth': ['bluetooth'],
         'pixel': ['pixel', 'stickers'],
         'system-world': ['sysworld'],
+        storymaker: ['storymaker'],
         'viewer': ['viewer'],
         'viewer-3d': ['viewer'],
         'zipper': ['zipper'],
```

`ui/js/desktop/core/desktop-foundation.js` (after base line 614, `ecalc: 'ECalcApp'`):

```diff
--- a/ui/js/desktop/core/desktop-foundation.js
+++ b/ui/js/desktop/core/desktop-foundation.js
@@ -612,6 +612,7 @@
             notes: 'NotesApp',
             findit: 'FindItApp',
             ecalc: 'ECalcApp',
+            storymaker: 'StorymakerApp',
             tresor: 'TresorApp',
             openscad: 'OpenSCADApp',
             looper: 'LooperApp',
```

`ui/js/desktop/core/menus-and-routing.js` (after base line 1770, before the `tresor` dispatch):

```diff
--- a/ui/js/desktop/core/menus-and-routing.js
+++ b/ui/js/desktop/core/menus-and-routing.js
@@ -1768,6 +1768,19 @@ if (appId === 'system-info') {
                 return window.ECalcApp.render(contentEl(id), id, Object.assign({}, context || {}, { esc, api, t }));
             }
         }
+        if (appId === 'storymaker') {
+            if (!window.StorymakerApp) {
+                window.AuraDesktopModules.loadAppScript('storymaker').then(() => renderAppContent(id, appId, context)).catch(err => renderAppError(id, appId, err));
+                return;
+            }
+            if (typeof window.StorymakerApp.render === 'function') {
+                return window.StorymakerApp.render(contentEl(id), id, Object.assign({}, context || {}, {
+                    esc, api, t, iconMarkup, notify: showDesktopNotification, readonly: desktopReadonly(), openApp, confirmDialog,
+                    setWindowBeforeClose: (winId, handler) => { const win = state.windows.get(winId); if (win) win.beforeClose = handler; },
+                    isActive: () => state.activeWindowId === id
+                }));
+            }
+        }
         if (appId === 'tresor') {
             if (!window.TresorApp) {
                 window.AuraDesktopModules.loadAppScript('tresor').then(() => renderAppContent(id, appId, context)).catch(err => renderAppError(id, appId, err));
```

`ui/js/desktop/core/window-shell-runtime.js` (after base line 1322 (size), base line 1357 (mobile-wide flag), after base line 1365 (minimum size)):

```diff
--- a/ui/js/desktop/core/window-shell-runtime.js
+++ b/ui/js/desktop/core/window-shell-runtime.js
@@ -1320,6 +1320,7 @@
             notes: { width: 1060, height: 680 },
             findit: { width: 1320, height: 820 },
             ecalc: { width: 1100, height: 760 },
+            storymaker: { width: 1320, height: 840 },
             tresor: { width: 1060, height: 700 },
             launchpad: { width: 1100, height: 700 },
             'system-info': { width: 800, height: 600 },
@@ -1354,7 +1355,7 @@
         return defaultWindowSize();
     }
 
-    function shouldUseMobileWideWindow(appId) { if (appId === 'kaggle' || appId === 'infra' || appId === 'ha-switchboard' || appId === 'docker-manager') return true; return !!{ meshcore: true, files: true, todo: true, radio: true, openscad: true, teevee: true, gallery: true, calendar: true, 'quick-connect': true, 'virtual-computers': true, 'network-cameras': true, 'code-studio': true, terminal: true, launchpad: true, looper: true, viewer: true, 'viewer-3d': true, bookshelf: true, findit: true, chess: true, 'retro-games': true, nasscad: true, 'mission-control': true, noisemaker: true, 'log-viewer': true, 'homepage-studio': true }[appId]; }
+    function shouldUseMobileWideWindow(appId) { if (appId === 'kaggle' || appId === 'infra' || appId === 'ha-switchboard' || appId === 'docker-manager') return true; return !!{ meshcore: true, files: true, todo: true, radio: true, openscad: true, teevee: true, gallery: true, calendar: true, 'quick-connect': true, 'virtual-computers': true, 'network-cameras': true, 'code-studio': true, terminal: true, launchpad: true, looper: true, viewer: true, 'viewer-3d': true, bookshelf: true, findit: true, chess: true, 'retro-games': true, nasscad: true, 'mission-control': true, noisemaker: true, 'log-viewer': true, 'homepage-studio': true, storymaker: true }[appId]; }
 
     function appWindowMinSize(appId) {
         if (appId === 'kaggle') return { width: 360, height: 420 };
@@ -1363,6 +1364,7 @@
         if (appId === 'ha-switchboard') return { width: 360, height: 540 };
         if (appId === 'findit') return { width: 360, height: 480 };
         if (appId === 'ecalc') return { width: 640, height: 480 };
+        if (appId === 'storymaker') return { width: 640, height: 480 };
         if (appId === 'radio') return { width: 360, height: 540 };
         if (appId === 'teevee') return { width: 1140, height: 540 }; // Keep the sidebar above its 1050px content breakpoint plus wood trim.
         if (appId === 'meshcore') return { width: 360, height: 480 };
```

`ui/js/desktop/bundles/main.bundle.js` is generated: run `npm run build:ui` and commit the result with this task (the file changes by the registry lines of this task only).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./ui -run 'TestDesktopStorymakerRegistration|TestDesktopAppMaterialsAdoptedByApps|TestDesktopAppAssetsRegistryCoversLazyCSSForAllBuiltinApps' -count=1` and `npm run check:ui`
Expected: ok

Recorded at drafting time:

```text
ok  	aurago/ui	1.437s
```

- [ ] **Step 5: Commit**

```bash
git add ui/desktop_storymaker_registration_test.go ui/js/desktop/core/module-loader.js ui/js/desktop/core/desktop-foundation.js ui/js/desktop/core/menus-and-routing.js ui/js/desktop/core/window-shell-runtime.js ui/js/desktop/bundles/main.bundle.js
git commit -m "feat(storymaker): register the app in the desktop loader, router and window presets" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E7: View modules and the static guards

The five view modules: the project list, the scene strip with the transport (scrubber), the inspector with the library, the code tab (CodeMirror) and the export panel. Untrusted text reaches the page only as `textContent`; the guards of the spec (no innerHTML or template string with data, no dialogs, the 1100-line budget, nothing external, no filtered classes, every used key exists) run over the app modules and `ui/js/storymaker`. The i18n scan of the desktop gets the `storymaker` prefix.

**Files:**
- Create: `ui/js/desktop/apps/storymaker-projects.js`
- Create: `ui/js/desktop/apps/storymaker-strip.js`
- Create: `ui/js/desktop/apps/storymaker-inspector.js`
- Create: `ui/js/desktop/apps/storymaker-code.js`
- Create: `ui/js/desktop/apps/storymaker-export.js`
- Modify: `ui/desktop_i18n_cleanup_test.go` (base line 145, `keyPrefixes`)
- Test: `ui/desktop_storymaker_guards_test.go`

- [ ] **Step 1: Write the failing test**

`ui/desktop_storymaker_guards_test.go`:

```go
package ui

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var storymakerKeyLiteral = regexp.MustCompile(`'(storymaker\.[A-Za-z0-9_.]+)'`)

func storymakerAppSources(t *testing.T) map[string]string {
	t.Helper()
	entries, err := filepath.Glob(filepath.Join("js", "desktop", "apps", "storymaker*.js"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("no storymaker app modules found: %v", err)
	}
	sources := map[string]string{}
	for _, path := range entries {
		sources[filepath.ToSlash(path)] = readDesktopAssetText(t, filepath.ToSlash(path))
	}
	return sources
}

func storymakerPlayerRealmSources(t *testing.T) map[string]string {
	t.Helper()
	sources := storymakerAppSources(t)
	entries, err := fs.ReadDir(Content, "js/storymaker")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".js") || strings.HasSuffix(entry.Name(), ".bundle.js") {
			continue
		}
		sources["js/storymaker/"+entry.Name()] = readDesktopAssetText(t, "js/storymaker/"+entry.Name())
	}
	return sources
}

// The spec's static guard: untrusted story text never reaches innerHTML, insertAdjacentHTML or a template string; no dynamic code, no
// browser dialogs; every app module keeps to the 1100-line budget and its shape.
func TestStorymakerSourcesAreSafeAndSmall(t *testing.T) {
	t.Parallel()
	forbidden := map[string]*regexp.Regexp{
		"innerHTML":          regexp.MustCompile(`\binnerHTML\b`),
		"outerHTML":          regexp.MustCompile(`\bouterHTML\b`),
		"insertAdjacentHTML": regexp.MustCompile(`\binsertAdjacentHTML\b`),
		"document.write":     regexp.MustCompile(`\bdocument\s*\.\s*write(ln)?\s*\(`),
		"eval":               regexp.MustCompile(`\beval\s*\(`),
		"new Function":       regexp.MustCompile(`\bnew\s+Function\b`),
		"alert":              regexp.MustCompile(`(^|[^.\w])(window\s*\.\s*)?alert\s*\(`),
		"confirm":            regexp.MustCompile(`(^|[^.\w])(window\s*\.\s*)?confirm\s*\(`),
		"prompt":             regexp.MustCompile(`(^|[^.\w])(window\s*\.\s*)?prompt\s*\(`),
		"template string":    regexp.MustCompile("`"),
		"localStorage":       regexp.MustCompile(`\blocalStorage\b|\bsessionStorage\b`),
		"cookie":             regexp.MustCompile(`document\s*\.\s*cookie`),
	}
	sources := storymakerPlayerRealmSources(t)
	if len(sources) < 30 {
		t.Fatalf("only %d storymaker scripts found", len(sources))
	}
	for path, source := range sources {
		code := storymakerCode(source)
		for label, pattern := range forbidden {
			if label == "template string" && strings.HasPrefix(path, "js/storymaker/") {
				continue // the player realm has its own Node guard for template strings
			}
			if pattern.MatchString(code) {
				t.Errorf("%s uses %s", path, label)
			}
		}
		if strings.HasPrefix(path, "js/desktop/apps/") {
			lines := strings.Count(source, "\n")
			if lines >= 1100 {
				t.Errorf("%s has %d lines, the desktop budget is 1100", path, lines)
			}
			if !strings.Contains(source, "'use strict'") || !strings.HasPrefix(source, "//") {
				t.Errorf("%s must start with a purpose comment and run in strict mode", path)
			}
			if !strings.Contains(source, "StorymakerParts") && !strings.Contains(source, "StorymakerApp") {
				t.Errorf("%s is not part of the StorymakerParts namespace", path)
			}
		}
	}
}

// Nothing loads from the Internet, no stylesheet or script names an external host, and the app keeps off the class names that
// cookie-banner and ad filters hide.
func TestStorymakerAppLoadsNothingExternalAndAvoidsFilteredClasses(t *testing.T) {
	t.Parallel()
	external := regexp.MustCompile(`(?i)https?://[a-z0-9.-]+\.[a-z]{2,}`)
	filtered := regexp.MustCompile(`(?i)^\.?(cc-|cookie|consent|gdpr|adsbox|ad-banner|banner-ad|sponsor)`)
	allowed := []string{"http://www.w3.org/2000/svg"}
	files := map[string]string{"css/desktop-app-storymaker.css": readDesktopAssetText(t, "css/desktop-app-storymaker.css")}
	for path, source := range storymakerAppSources(t) {
		files[path] = source
	}
	for path, source := range files {
		text := source
		for _, ok := range allowed {
			text = strings.ReplaceAll(text, ok, "")
		}
		if match := external.FindString(text); match != "" {
			t.Errorf("%s names an external address: %s", path, match)
		}
		if strings.HasSuffix(path, ".css") {
			for _, class := range regexp.MustCompile(`\.[a-z][\w-]*`).FindAllString(storymakerCode(text), -1) {
				if filtered.MatchString(class) {
					t.Errorf("%s uses the filtered class %s", path, class)
				}
			}
			continue
		}
		for _, c := range regexp.MustCompile(`class: '([^']*)'`).FindAllStringSubmatch(text, -1) {
			for _, name := range strings.Fields(c[1]) {
				if !strings.HasPrefix(name, "stm-") {
					t.Errorf("%s uses the class %q: every class of the app starts with stm-", path, name)
				}
				if filtered.MatchString(name) {
					t.Errorf("%s uses the filtered class %q", path, name)
				}
			}
		}
	}
}

// Every key the app names exists, and so do the keys it builds from a list (issue codes, job errors, export phases, browser sources).
func TestStorymakerAppUsesOnlyKeysItHas(t *testing.T) {
	t.Parallel()
	english := storymakerLocale(t, "storymaker", "en")
	used := 0
	for path, source := range storymakerAppSources(t) {
		for _, match := range storymakerKeyLiteral.FindAllStringSubmatch(source, -1) {
			key := match[1]
			if strings.HasSuffix(key, ".") || strings.HasSuffix(key, "_") {
				continue // the prefix of a family built from a list, checked below
			}
			used++
			if _, ok := english[key]; !ok {
				t.Errorf("%s uses %s, which lang/storymaker/en.json lacks", path, key)
			}
			if strings.HasPrefix(key, "storymaker.catalog.") {
				t.Errorf("%s names the catalog key %s: those belong to the catalog directory", path, key)
			}
		}
	}
	if used < 150 {
		t.Fatalf("found only %d key literals in the app modules, the pattern no longer matches", used)
	}
	core := storymakerAppSources(t)["js/desktop/apps/storymaker-core.js"]
	exporter := storymakerAppSources(t)["js/desktop/apps/storymaker-export.js"]
	families := map[string][]string{
		"storymaker.issue.":         storymakerList(t, core, "ISSUE_CODES"),
		"storymaker.job_error.":     storymakerList(t, core, "JOB_ERROR_CODES"),
		"storymaker.export.phase_":  storymakerList(t, exporter, "PHASES"),
		"storymaker.export.source_": {"config", "system", "pinned"},
	}
	// The list of issue codes is the contract's: nineteen codes, no more, no less.
	want := strings.Fields("invalid_json unknown_field schema_mismatch too_large limit_exceeded missing_field invalid_value duplicate_id unknown_name unknown_asset unknown_spot unknown_cast unknown_cue canonicalised not_supported_yet narration_missing code_error deadline layout_unverified")
	if strings.Join(families["storymaker.issue."], " ") != strings.Join(want, " ") {
		t.Errorf("ISSUE_CODES = %v, want the 19 codes of the contract", families["storymaker.issue."])
	}
	for prefix, names := range families {
		if len(names) == 0 {
			t.Fatalf("no names found for %s", prefix)
		}
		for _, name := range names {
			if _, ok := english[prefix+name]; !ok {
				t.Errorf("lang/storymaker/en.json lacks %s%s", prefix, name)
			}
		}
	}
	for _, key := range []string{"storymaker.issue.unknown", "storymaker.job_error.unknown", "storymaker.export.source_unknown"} {
		if _, ok := english[key]; !ok {
			t.Errorf("lang/storymaker/en.json lacks the fallback %s", key)
		}
	}
}

// storymakerList reads `const NAME = ['a', 'b', ...];` from a source file.
func storymakerList(t *testing.T, source, name string) []string {
	t.Helper()
	match := regexp.MustCompile(`const ` + name + ` = \[([^\]]*)\]`).FindStringSubmatch(source)
	if match == nil {
		t.Fatalf("no list %s in the source", name)
	}
	var out []string
	for _, item := range regexp.MustCompile(`'([a-z_]+)'`).FindAllStringSubmatch(match[1], -1) {
		out = append(out, item[1])
	}
	return out
}

// The loader lists exactly the storymaker scripts that exist: nothing on disk the loader skips, nothing listed that is missing.
func TestStorymakerLoaderListsEveryScript(t *testing.T) {
	t.Parallel()
	loader := rawDesktopAssetText(t, "js/desktop/core/module-loader.js")
	start := strings.Index(loader, "        storymaker: {")
	if start < 0 {
		t.Fatal("module-loader.js has no storymaker entry")
	}
	entry := loader[start : start+strings.Index(loader[start:], "\n        },")]
	listed := map[string]bool{}
	for _, match := range regexp.MustCompile(`/js/desktop/apps/(storymaker[\w-]*\.js)'`).FindAllStringSubmatch(entry, -1) {
		listed[match[1]] = true
	}
	onDisk, err := filepath.Glob(filepath.Join("js", "desktop", "apps", "storymaker*.js"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range onDisk {
		name := filepath.Base(path)
		if !listed[name] {
			t.Errorf("%s exists but the loader does not load it", name)
		}
		delete(listed, name)
	}
	for name := range listed {
		t.Errorf("the loader lists %s, which does not exist", name)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./ui -run 'TestStorymakerAppUsesOnlyKeysItHas' -count=1`
Expected: FAIL: FAIL (`no list PHASES in the source`: `storymaker-export.js` does not exist)

Recorded at drafting time (the replay of this task on a tree without it):

```text
--- FAIL: TestStorymakerAppUsesOnlyKeysItHas (0.00s)
    desktop_storymaker_guards_test.go:150: found only 14 key literals in the app modules, the pattern no longer matches
FAIL
FAIL	aurago/ui	0.241s
FAIL
```

- [ ] **Step 3: Implement**

`ui/js/desktop/apps/storymaker-projects.js`:

```javascript
// Storymaker app project list: the start screen. A new project asks for a title and a format; the list shows every project
// of the workspace (newest first) with open and delete (a confirmation through the desktop dialog). The titles are
// untrusted text and reach the page through textContent only. Classic script, publishes S.createProjects.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const ASPECTS = [['16:9', 'storymaker.aspect_16_9'], ['9:16', 'storymaker.aspect_9_16'], ['1:1', 'storymaker.aspect_1_1']];

    // inst: {t, readonly, createProject(spec), openProject(id), deleteProject(project), confirm(title, message)}
    function createProjects(inst) {
        const { el, icon } = S;
        const t = inst.t;
        let projects = null;
        let aspect = '16:9';
        let busy = false;
        let message = '';

        const title = el('input', { class: 'stm-input', type: 'text', id: inst.domId('new-title'), maxlength: '120', required: true, autocomplete: 'off', placeholder: t('storymaker.projects.new_title_placeholder'), dataset: { stm: 'new-title' } });
        const errorLine = el('p', { class: 'stm-form-error', role: 'alert', hidden: true, dataset: { stm: 'projects-error' } });
        const aspectButtons = ASPECTS.map(([value, key]) => el('button', {
            type: 'button', class: 'stm-chip', dataset: { stm: 'aspect', aspect: value }, ariaPressed: String(value === aspect),
            onClick: () => { aspect = value; syncAspect(); }
        }, t(key)));
        const create = el('button', { type: 'submit', class: 'stm-btn stm-btn-primary', dataset: { stm: 'create-project' } }, icon('plus'), el('span', { text: t('storymaker.projects.create') }));
        const form = el('form', { class: 'stm-new', onSubmit: event => { event.preventDefault(); submit(); } },
            el('label', { class: 'stm-field', for: inst.domId('new-title') }, el('span', { class: 'stm-label', text: t('storymaker.projects.new_title_label') }), title),
            el('div', { class: 'stm-field' }, el('span', { class: 'stm-label', text: t('storymaker.projects.new_format') }), el('div', { class: 'stm-chips', role: 'group', ariaLabel: t('storymaker.projects.new_format') }, aspectButtons)),
            el('div', { class: 'stm-actions' }, create),
            errorLine);
        const list = el('ul', { class: 'stm-project-list', dataset: { stm: 'project-list' } });
        const empty = el('div', { class: 'stm-empty', hidden: true, dataset: { stm: 'projects-empty' } },
            icon('film', 'stm-empty-icon'),
            el('h3', { text: t('storymaker.projects.empty_title') }),
            el('p', { text: t('storymaker.projects.empty_text') }));
        const loading = el('p', { class: 'stm-muted', role: 'status', text: t('storymaker.loading') });
        const root = el('section', { class: 'stm-projects', ariaLabel: t('storymaker.projects.title') },
            el('header', { class: 'stm-projects-head' },
                el('h2', { text: t('storymaker.projects.title') }),
                el('p', { class: 'stm-muted', text: t('storymaker.projects.intro') })),
            form, loading, empty, list);

        function syncAspect() {
            aspectButtons.forEach(button => button.setAttribute('aria-pressed', String(button.dataset.aspect === aspect)));
        }

        function syncDisabled() {
            const off = inst.readonly || busy;
            title.disabled = inst.readonly;
            aspectButtons.forEach(button => { button.disabled = off; });
            create.disabled = off;
            create.title = inst.readonly ? t('storymaker.banner.readonly') : '';
            list.querySelectorAll('[data-stm="delete-project"]').forEach(button => { button.disabled = off; });
        }

        function setMessage(text) {
            message = text || '';
            errorLine.textContent = message;
            errorLine.hidden = !message;
        }

        async function submit() {
            const value = title.value.trim();
            if (!value) { setMessage(t('storymaker.projects.title_required')); title.focus(); return; }
            busy = true;
            setMessage('');
            syncDisabled();
            try {
                await inst.createProject({ title: value, aspect });
            } catch (err) {
                setMessage(S.errors.describe(t, err));
            } finally {
                busy = false;
                syncDisabled();
            }
        }

        function card(project) {
            const meta = [project.aspect, project.fps ? project.fps + ' fps' : '', S.format.dateTime(project.updated_at)].filter(Boolean).join(' · ');
            const open = el('button', { type: 'button', class: 'stm-project-open', dataset: { stm: 'open-project', id: project.id }, onClick: () => inst.openProject(project.id) },
                el('span', { class: 'stm-project-title', text: project.title || project.id }),
                el('span', { class: 'stm-project-meta', text: meta }));
            const remove = el('button', {
                type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', dataset: { stm: 'delete-project', id: project.id },
                ariaLabel: t('storymaker.projects.delete_label', { title: project.title || project.id }), title: t('storymaker.delete'),
                onClick: async () => {
                    const ok = await inst.confirm(t('storymaker.projects.delete_title'), t('storymaker.projects.delete_text', { title: project.title || project.id }));
                    if (!ok) return;
                    busy = true;
                    syncDisabled();
                    try {
                        await inst.deleteProject(project);
                    } catch (err) {
                        setMessage(S.errors.describe(t, err));
                    } finally {
                        busy = false;
                        syncDisabled();
                    }
                }
            }, icon('trash'));
            return el('li', { class: 'stm-project' }, open, remove);
        }

        function paint() {
            S.clear(list);
            loading.hidden = projects !== null;
            const sorted = (projects || []).slice().sort((a, b) => String(b.updated_at).localeCompare(String(a.updated_at)));
            sorted.forEach(project => list.appendChild(card(project)));
            empty.hidden = projects === null || sorted.length > 0;
            syncDisabled();
        }

        paint();
        return {
            el: root,
            setProjects(next) { projects = Array.isArray(next) ? next : []; paint(); },
            setError(text) { setMessage(text); },
            refresh() { syncDisabled(); },
            focus() { title.focus(); },
            count: () => (projects ? projects.length : 0)
        };
    }

    S.createProjects = createProjects;
})();
```

`ui/js/desktop/apps/storymaker-strip.js`:

```javascript
// Storymaker app scene strip and transport. The strip lists the scenes with the thumbnails the player renders (the `thumb`
// message), the narration snippet or id, the duration of the compile report and the add buttons; it moves with the arrow
// keys. The transport is play/pause, the time and the scrubber: one block per scene of the compile report, a playhead, pointer
// dragging and the keyboard (arrows step a frame, Shift a second, Home/End the ends). The scene texts are untrusted and are
// set as text only. Classic script, publishes S.createStrip and S.createTransport.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};

    function drawBitmap(canvas, bitmap) {
        if (!bitmap) return;
        if (canvas.width !== bitmap.width) canvas.width = bitmap.width;
        if (canvas.height !== bitmap.height) canvas.height = bitmap.height;
        const ctx = canvas.getContext('2d');
        if (!ctx) return;
        ctx.clearRect(0, 0, canvas.width, canvas.height);
        ctx.drawImage(bitmap, 0, 0);
        canvas.dataset.drawn = 'true';
    }

    function snippet(text, max) {
        const one = String(text || '').replace(/\s+/g, ' ').trim();
        return one.length > max ? one.slice(0, max - 1) + '…' : one;
    }

    // inst: {t, bus, story, selected, report, thumbs, readonly, select(i, opts), addScene(kind)}
    function createStrip(inst) {
        const { el, icon } = S;
        const t = inst.t;
        const list = el('ol', { class: 'stm-scene-list', dataset: { stm: 'scene-list' }, ariaLabel: t('storymaker.strip.title') });
        const addScene = el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'add-scene' }, onClick: () => inst.addScene('storyboard') }, icon('plus'), el('span', { text: t('storymaker.strip.add_storyboard') }));
        const addCode = el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'add-code-scene' }, onClick: () => inst.addScene('code') }, icon('code'), el('span', { text: t('storymaker.strip.add_code') }));
        const root = el('aside', { class: 'stm-strip', ariaLabel: t('storymaker.strip.title') },
            el('h3', { class: 'stm-panel-title', text: t('storymaker.strip.title') }),
            list,
            el('div', { class: 'stm-strip-add' }, addScene, addCode));

        function durationText(index) {
            const report = inst.report;
            const scene = report && report.scenes ? report.scenes[index] : null;
            if (!scene || !report.fps) return '';
            return t('storymaker.strip.duration_s', { seconds: S.format.seconds((scene.end_frame - scene.start_frame) / report.fps) });
        }

        function item(scene, index) {
            const selected = index === inst.selected;
            const isCode = scene.kind === 'code';
            const canvas = el('canvas', { class: 'stm-thumb', width: '192', height: '108', dataset: { scene: scene.id } });
            const bitmap = inst.thumbs.get(scene.id);
            if (bitmap) drawBitmap(canvas, bitmap);
            const label = snippet(scene.narration, 48) || scene.id;
            const button = el('button', {
                type: 'button', class: 'stm-scene-btn', tabindex: selected ? '0' : '-1', dataset: { stm: 'scene', index, scene: scene.id },
                ariaCurrent: selected ? 'true' : null,
                ariaLabel: t('storymaker.strip.scene_n', { n: index + 1 }) + ': ' + label,
                onClick: () => inst.select(index, { seek: true }),
                onKeydown: event => {
                    const next = event.key === 'ArrowDown' || event.key === 'ArrowRight' ? index + 1 : event.key === 'ArrowUp' || event.key === 'ArrowLeft' ? index - 1 : event.key === 'Home' ? 0 : event.key === 'End' ? S.model.scenesOf(inst.story).length - 1 : -1;
                    if (next < 0 || next >= S.model.scenesOf(inst.story).length) return;
                    event.preventDefault();
                    inst.select(next, { seek: true });
                    const target = list.querySelector('[data-index="' + next + '"]');
                    if (target) target.focus();
                }
            },
                canvas,
                el('span', { class: 'stm-scene-num', text: String(index + 1) }),
                isCode ? el('span', { class: 'stm-scene-kind', title: t('storymaker.strip.kind_code') }, icon('code')) : null,
                el('span', { class: 'stm-scene-name', text: label }),
                el('span', { class: 'stm-scene-dur', text: durationText(index) }));
            return el('li', { class: 'stm-scene', dataset: { selected: String(selected), kind: isCode ? 'code' : 'storyboard' } }, button);
        }

        function paint() {
            const focused = list.contains(document.activeElement) ? document.activeElement.getAttribute('data-index') : null;
            S.clear(list);
            S.model.scenesOf(inst.story).forEach((scene, index) => list.appendChild(item(scene, index)));
            const off = inst.readonly;
            addScene.disabled = off;
            addCode.disabled = off;
            if (focused !== null) {
                const again = list.querySelector('[data-index="' + focused + '"]');
                if (again) again.focus();
            }
            const current = list.querySelector('[aria-current="true"]');
            if (current && typeof current.scrollIntoView === 'function') current.scrollIntoView({ block: 'nearest', inline: 'nearest' });
        }

        function drawScene(sceneId) {
            const bitmap = inst.thumbs.get(sceneId);
            list.querySelectorAll('canvas[data-scene]').forEach(canvas => { if (canvas.dataset.scene === sceneId) drawBitmap(canvas, bitmap); });
        }

        const off = [
            inst.bus.on('story', paint),
            inst.bus.on('selection', paint),
            inst.bus.on('report', paint),
            inst.bus.on('readonly', paint),
            inst.bus.on('thumb', drawScene)
        ];
        paint();
        return { el: root, paint, dispose() { off.forEach(fn => fn()); } };
    }

    // inst: {t, bus, report, frame, playing, readonly, play(), pause(), seek(frame)}
    function createTransport(inst) {
        const { el, icon } = S;
        const t = inst.t;
        const playButton = el('button', { type: 'button', class: 'stm-btn stm-btn-primary stm-btn-icon', dataset: { stm: 'play' }, ariaLabel: t('storymaker.play'), title: t('storymaker.play'), onClick: () => (inst.playing ? inst.pause() : inst.play()) }, icon('play'));
        const clock = el('span', { class: 'stm-clock', dataset: { stm: 'clock' }, role: 'timer', text: '0:00.0' });
        const total = el('span', { class: 'stm-clock stm-clock-total', dataset: { stm: 'clock-total' }, text: '0:00.0' });
        const blocks = el('div', { class: 'stm-scrub-blocks' });
        const head = el('div', { class: 'stm-scrub-head' });
        const scrub = el('div', {
            class: 'stm-scrub', role: 'slider', tabindex: '0', dataset: { stm: 'scrubber' },
            ariaLabel: t('storymaker.timeline'), ariaValuemin: '0', ariaValuemax: '0', ariaValuenow: '0'
        }, blocks, head);
        const root = el('div', { class: 'stm-transport' }, playButton, clock, scrub, total);
        let dragging = false;

        function frameCount() {
            return inst.report && inst.report.frame_count > 0 ? inst.report.frame_count : 0;
        }

        function frameAt(event) {
            const rect = scrub.getBoundingClientRect();
            const ratio = rect.width > 0 ? S.clamp((event.clientX - rect.left) / rect.width, 0, 1) : 0;
            return Math.round(ratio * Math.max(0, frameCount() - 1));
        }

        function paintBlocks() {
            S.clear(blocks);
            const report = inst.report;
            const count = frameCount();
            if (!report || !count) return;
            report.scenes.forEach((scene, index) => {
                const left = scene.start_frame / count * 100;
                const width = Math.max(0.4, (scene.end_frame - scene.start_frame) / count * 100);
                blocks.appendChild(el('div', {
                    class: 'stm-scrub-block', title: t('storymaker.strip.scene_n', { n: index + 1 }) + ' · ' + scene.id,
                    dataset: { index, kind: scene.kind || 'storyboard', selected: String(index === inst.selected) },
                    style: { left: left + '%', width: width + '%' }
                }));
            });
        }

        function paintTime() {
            const count = frameCount();
            const fps = inst.report ? inst.report.fps : 30;
            const frame = S.clamp(inst.frame, 0, Math.max(0, count - 1));
            head.style.left = (count > 1 ? frame / (count - 1) * 100 : 0) + '%';
            clock.textContent = S.format.clock(frame, fps);
            total.textContent = S.format.clock(count, fps);
            scrub.setAttribute('aria-valuemax', String(Math.max(0, count - 1)));
            scrub.setAttribute('aria-valuenow', String(frame));
            scrub.setAttribute('aria-valuetext', S.format.clock(frame, fps));
            const enabled = count > 0;
            playButton.disabled = !enabled;
            scrub.setAttribute('aria-disabled', String(!enabled));
            if (playButton.dataset.playing !== String(!!inst.playing)) {
                const label = t(inst.playing ? 'storymaker.pause' : 'storymaker.play');
                playButton.setAttribute('aria-label', label);
                playButton.title = label;
                playButton.dataset.playing = String(!!inst.playing);
                S.clear(playButton).appendChild(icon(inst.playing ? 'pause' : 'play'));
            }
        }

        function paintAll() {
            paintBlocks();
            paintTime();
        }

        function pointer(event) {
            if (!frameCount()) return;
            inst.seek(frameAt(event));
        }
        scrub.addEventListener('pointerdown', event => {
            if (event.button !== 0 || !frameCount()) return;
            dragging = true;
            try { scrub.setPointerCapture(event.pointerId); } catch (_) { /* synthetic pointer */ }
            scrub.focus();
            pointer(event);
        });
        scrub.addEventListener('pointermove', event => { if (dragging) pointer(event); });
        const endDrag = () => { dragging = false; };
        scrub.addEventListener('pointerup', endDrag);
        scrub.addEventListener('pointercancel', endDrag);
        scrub.addEventListener('keydown', event => {
            const count = frameCount();
            if (!count) return;
            const fps = inst.report.fps || 30;
            const step = event.shiftKey ? fps : 1;
            let target = null;
            if (event.key === 'ArrowRight' || event.key === 'ArrowUp') target = inst.frame + step;
            else if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') target = inst.frame - step;
            else if (event.key === 'Home') target = 0;
            else if (event.key === 'End') target = count - 1;
            else if (event.key === ' ' || event.key === 'Enter') { event.preventDefault(); if (inst.playing) inst.pause(); else inst.play(); return; }
            if (target === null) return;
            event.preventDefault();
            inst.seek(S.clamp(target, 0, count - 1));
        });

        const off = [
            inst.bus.on('report', paintAll),
            inst.bus.on('selection', paintBlocks),
            inst.bus.on('time', paintTime)
        ];
        paintAll();
        return { el: root, paint: paintAll, dispose() { off.forEach(fn => fn()); } };
    }

    S.createStrip = createStrip;
    S.createTransport = createTransport;
})();
```

`ui/js/desktop/apps/storymaker-inspector.js`:

```javascript
// Storymaker app inspector and library. The inspector edits the selected scene: environment and time of day (from the
// catalog), narration, the entry transition, the duration (automatic or fixed) and the actors and props that stand in it,
// and it moves or deletes the scene. The library lists the people, props and environments of the catalog with search; a card
// puts an actor or a prop into the selected scene, or sets its environment. Every edit goes through inst.editScene, which
// works on a clone of the story and saves it; narration keeps typing local until the debounce fires and no refresh ever
// replaces a control that has the focus. Classic script, publishes S.createInspector and S.createLibrary.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const THUMB_RE = /^thumbs\/[A-Za-z0-9._-]+\.svg$/;
    const SUPPORTED_TRANSITIONS = ['cut', 'fade', 'slide-left', 'wipe'];

    function option(value, label, selected) {
        return S.el('option', { value, selected: selected || null, text: label });
    }

    // inst: {t, bus, story, selected, catalog, status, readonly, editScene(id, fn, opts), select(i), moveScene(i, to), deleteScene(i), openCode(id)}
    function createInspector(inst) {
        const { el, icon } = S;
        const t = inst.t;
        const body = el('div', { class: 'stm-inspector-body' });
        const root = el('section', { class: 'stm-inspector', ariaLabel: t('storymaker.inspector.title'), dataset: { stm: 'inspector' } },
            el('h3', { class: 'stm-panel-title', text: t('storymaker.inspector.title') }), body);
        let built = { id: '', kind: '', locked: false };
        let controls = {};

        function currentScene() {
            return S.model.scenesOf(inst.story)[inst.selected] || null;
        }

        function names(kind) {
            return S.model.entriesOf(inst.catalog, kind);
        }

        function environmentSelect(scene) {
            const select = el('select', { class: 'stm-select', id: inst.domId('env'), dataset: { stm: 'environment' } });
            const current = scene.environment || '';
            const known = names('environment');
            known.forEach(entry => select.appendChild(option(entry.id, S.catalogName(t, entry), entry.id === current)));
            if (current && !known.some(entry => entry.id === current)) select.appendChild(option(current, current, true));
            select.addEventListener('change', () => inst.editScene(scene.id, draft => { draft.environment = select.value; }, { immediate: true }));
            return select;
        }

        function variantSelect(scene) {
            const current = scene.variant || 'day';
            const select = el('select', { class: 'stm-select', id: inst.domId('variant'), dataset: { stm: 'variant' } },
                option('day', t('storymaker.inspector.variant_day'), current === 'day'),
                option('night', t('storymaker.inspector.variant_night'), current === 'night'));
            if (current !== 'day' && current !== 'night') select.appendChild(option(current, t('storymaker.inspector.unsupported', { name: current }), true));
            select.addEventListener('change', () => inst.editScene(scene.id, draft => { draft.variant = select.value; }, { immediate: true }));
            return select;
        }

        function transitionSelect(scene, first) {
            const select = el('select', { class: 'stm-select', id: inst.domId('transition'), dataset: { stm: 'transition' } });
            const current = scene.transition || (first ? 'cut' : 'fade');
            const listed = names('transition').map(entry => ({ value: entry.name, label: S.catalogName(t, entry) }));
            const offered = listed.length ? listed : SUPPORTED_TRANSITIONS.map(name => ({ value: name, label: name }));
            offered.forEach(item => select.appendChild(option(item.value, item.label, item.value === current)));
            if (!offered.some(item => item.value === current)) select.appendChild(option(current, t('storymaker.inspector.unsupported', { name: current }), true));
            select.disabled = first;
            select.addEventListener('change', () => inst.editScene(scene.id, draft => { draft.transition = select.value; }, { immediate: true }));
            return select;
        }

        function durationControls(scene) {
            const state = S.model.durationOf(scene);
            const mode = el('select', { class: 'stm-select', id: inst.domId('duration-mode'), dataset: { stm: 'duration-mode' } },
                option('auto', t('storymaker.inspector.duration_auto'), state.auto),
                option('fixed', t('storymaker.inspector.duration_fixed'), !state.auto));
            const maxSeconds = inst.status && inst.status.limits && Number(inst.status.limits.max_duration_seconds) > 0 ? Number(inst.status.limits.max_duration_seconds) : 600;
            const seconds = el('input', { class: 'stm-input stm-input-narrow', type: 'number', min: '0.5', max: String(maxSeconds), step: '0.5', value: state.auto ? '5' : String(state.seconds), ariaLabel: t('storymaker.inspector.duration_seconds'), dataset: { stm: 'duration-seconds' } });
            seconds.disabled = state.auto;
            const apply = () => inst.editScene(scene.id, draft => { S.model.setDuration(draft, mode.value === 'auto', seconds.value); }, { immediate: true });
            mode.addEventListener('change', () => { seconds.disabled = mode.value === 'auto'; apply(); });
            seconds.addEventListener('change', apply);
            return { mode, seconds };
        }

        function narrationField(scene) {
            const max = inst.status && inst.status.limits && Number(inst.status.limits.max_narration_chars) > 0 ? Number(inst.status.limits.max_narration_chars) : 0;
            const area = el('textarea', { class: 'stm-textarea', id: inst.domId('narration'), rows: '6', maxlength: max || null, spellcheck: 'true', dataset: { stm: 'narration' }, placeholder: t('storymaker.inspector.narration_placeholder') });
            area.value = scene.narration || '';
            area.addEventListener('input', () => inst.editScene(scene.id, draft => { S.model.setSceneField(draft, 'narration', area.value); }, { immediate: false }));
            return area;
        }

        function field(labelKey, controlId, control, hint) {
            return el('div', { class: 'stm-field' },
                el('label', { class: 'stm-label', for: controlId, text: t(labelKey) }),
                control,
                hint ? el('p', { class: 'stm-hint', text: hint }) : null);
        }

        function itemsList(scene) {
            const list = el('ul', { class: 'stm-items', dataset: { stm: 'scene-items' } });
            const castName = id => {
                const member = (inst.story.cast || []).find(candidate => candidate && candidate.id === id);
                if (!member) return id;
                const entry = inst.catalog && inst.catalog.byId ? inst.catalog.byId[member.character] : null;
                return member.name || S.catalogName(t, entry) || id;
            };
            (Array.isArray(scene.actors) ? scene.actors : []).forEach((actor, k) => {
                list.appendChild(el('li', { class: 'stm-item', dataset: { kind: 'actor' } },
                    icon('user'), el('span', { class: 'stm-item-name', text: castName(actor.cast) }),
                    el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', dataset: { stm: 'remove-actor', index: k }, ariaLabel: t('storymaker.inspector.remove_item', { name: castName(actor.cast) }), title: t('storymaker.inspector.remove'), disabled: inst.readonly || null,
                        onClick: () => inst.editScene(scene.id, draft => { if (Array.isArray(draft.actors)) { draft.actors.splice(k, 1); if (!draft.actors.length) delete draft.actors; } }, { immediate: true }) }, icon('close'))));
            });
            (Array.isArray(scene.props) ? scene.props : []).forEach((prop, k) => {
                const entry = inst.catalog && inst.catalog.byId ? inst.catalog.byId[prop.item] : null;
                const name = S.catalogName(t, entry) || prop.item;
                list.appendChild(el('li', { class: 'stm-item', dataset: { kind: 'prop' } },
                    icon('box'), el('span', { class: 'stm-item-name', text: name }),
                    el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', dataset: { stm: 'remove-prop', index: k }, ariaLabel: t('storymaker.inspector.remove_item', { name }), title: t('storymaker.inspector.remove'), disabled: inst.readonly || null,
                        onClick: () => inst.editScene(scene.id, draft => { if (Array.isArray(draft.props)) { draft.props.splice(k, 1); if (!draft.props.length) delete draft.props; } }, { immediate: true }) }, icon('close'))));
            });
            if (!list.children.length) list.appendChild(el('li', { class: 'stm-muted', text: t('storymaker.inspector.no_items') }));
            return list;
        }

        function build(scene, index) {
            S.clear(body);
            controls = {};
            const total = S.model.scenesOf(inst.story).length;
            const isCode = scene.kind === 'code';
            const head = el('div', { class: 'stm-inspector-head' },
                el('div', { class: 'stm-inspector-title' },
                    el('strong', { text: t('storymaker.strip.scene_n', { n: index + 1 }) }),
                    el('span', { class: 'stm-muted', text: scene.id })),
                el('div', { class: 'stm-inspector-tools' },
                    el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', dataset: { stm: 'scene-up' }, ariaLabel: t('storymaker.inspector.move_up'), title: t('storymaker.inspector.move_up'), disabled: inst.readonly || index === 0 || null, onClick: () => inst.moveScene(index, index - 1) }, icon('up')),
                    el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', dataset: { stm: 'scene-down' }, ariaLabel: t('storymaker.inspector.move_down'), title: t('storymaker.inspector.move_down'), disabled: inst.readonly || index >= total - 1 || null, onClick: () => inst.moveScene(index, index + 1) }, icon('down')),
                    el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', dataset: { stm: 'scene-delete' }, ariaLabel: t('storymaker.inspector.delete_scene'), title: t('storymaker.inspector.delete_scene'), disabled: inst.readonly || total <= 1 || null, onClick: () => inst.deleteScene(index) }, icon('trash'))));
            body.appendChild(head);
            if (isCode) {
                body.appendChild(el('div', { class: 'stm-note' }, icon('code'), el('span', { text: t('storymaker.inspector.code_scene_note') }),
                    el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'edit-code' }, onClick: () => inst.openCode(scene.id) }, el('span', { text: t('storymaker.inspector.edit_code') }))));
            } else {
                controls.environment = environmentSelect(scene);
                controls.variant = variantSelect(scene);
                body.appendChild(field('storymaker.inspector.environment', inst.domId('env'), controls.environment));
                body.appendChild(field('storymaker.inspector.variant', inst.domId('variant'), controls.variant));
            }
            controls.narration = narrationField(scene);
            body.appendChild(field('storymaker.inspector.narration', inst.domId('narration'), controls.narration, t('storymaker.inspector.narration_hint')));
            controls.transition = transitionSelect(scene, index === 0);
            body.appendChild(field('storymaker.inspector.transition', inst.domId('transition'), controls.transition, index === 0 ? t('storymaker.inspector.transition_first') : ''));
            const duration = durationControls(scene);
            controls.durationMode = duration.mode;
            controls.durationSeconds = duration.seconds;
            body.appendChild(el('div', { class: 'stm-field' },
                el('label', { class: 'stm-label', for: inst.domId('duration-mode'), text: t('storymaker.inspector.duration') }),
                el('div', { class: 'stm-row' }, duration.mode, duration.seconds, el('span', { class: 'stm-muted', text: t('storymaker.inspector.seconds_unit') }))));
            if (!isCode) {
                controls.items = itemsList(scene);
                body.appendChild(el('div', { class: 'stm-field' }, el('span', { class: 'stm-label', text: t('storymaker.inspector.cast_props') }), controls.items));
            }
            applyReadonly();
        }

        function applyReadonly() {
            ['environment', 'variant', 'narration'].forEach(name => { if (controls[name]) controls[name].disabled = inst.readonly; });
            if (controls.narration) controls.narration.readOnly = inst.readonly;
            if (controls.durationMode) controls.durationMode.disabled = inst.readonly;
            if (controls.durationSeconds) controls.durationSeconds.disabled = inst.readonly || controls.durationMode.value === 'auto';
        }

        // Same scene, same kind: only the values move, and never under the user's hands.
        function refresh(scene, index) {
            const set = (control, value) => { if (control && document.activeElement !== control && control.value !== value) control.value = value; };
            set(controls.environment, scene.environment || '');
            set(controls.variant, scene.variant || 'day');
            set(controls.narration, scene.narration || '');
            const duration = S.model.durationOf(scene);
            set(controls.durationMode, duration.auto ? 'auto' : 'fixed');
            if (controls.durationSeconds && document.activeElement !== controls.durationSeconds) {
                controls.durationSeconds.value = duration.auto ? controls.durationSeconds.value : String(duration.seconds);
                controls.durationSeconds.disabled = inst.readonly || duration.auto;
            }
            set(controls.transition, scene.transition || (index === 0 ? 'cut' : 'fade'));
            if (controls.items) {
                const next = itemsList(scene);
                controls.items.replaceWith(next);
                controls.items = next;
            }
            applyReadonly();
        }

        function paint() {
            const scene = currentScene();
            if (!scene) { S.clear(body); built = { id: '', kind: '' }; return; }
            const kind = scene.kind === 'code' ? 'code' : 'storyboard';
            const total = S.model.scenesOf(inst.story).length;
            const key = scene.id + '|' + kind + '|' + inst.selected + '|' + total + '|' + inst.readonly + '|' + (inst.catalog ? inst.catalog.version : '');
            if (built.key !== key) {
                const hadFocus = body.contains(document.activeElement);
                build(scene, inst.selected);
                built = { id: scene.id, kind, key };
                if (hadFocus && controls.narration) controls.narration.focus();
            } else {
                refresh(scene, inst.selected);
            }
        }

        const off = [inst.bus.on('story', paint), inst.bus.on('selection', paint), inst.bus.on('readonly', paint)];
        paint();
        return { el: root, paint, dispose() { off.forEach(fn => fn()); } };
    }

    // inst: {t, bus, story, selected, catalog, readonly, addActor(entry), addProp(entry), setEnvironment(id), actorInScene(entry)}
    function createLibrary(inst) {
        const { el, icon } = S;
        const t = inst.t;
        const TABS = [['character', 'storymaker.library.people', 'user'], ['prop', 'storymaker.library.props', 'box'], ['environment', 'storymaker.library.environments', 'scenery']];
        let tab = 'character';
        let query = '';
        const search = el('input', { class: 'stm-input', type: 'search', id: inst.domId('library-search'), placeholder: t('storymaker.library.search'), ariaLabel: t('storymaker.library.search'), autocomplete: 'off', dataset: { stm: 'library-search' } });
        const tabs = el('div', { class: 'stm-tabs stm-tabs-small', role: 'tablist', ariaLabel: t('storymaker.library.title') },
            TABS.map(([kind, key, glyph]) => el('button', {
                type: 'button', class: 'stm-tab', role: 'tab', id: inst.domId('lib-' + kind), dataset: { stm: 'library-tab', kind }, ariaSelected: String(kind === tab),
                onClick: () => { tab = kind; paint(); }
            }, icon(glyph), el('span', { text: t(key) }))));
        const grid = el('ul', { class: 'stm-cards', dataset: { stm: 'library-grid' } });
        const empty = el('p', { class: 'stm-muted', hidden: true, text: t('storymaker.library.empty') });
        const root = el('section', { class: 'stm-library', ariaLabel: t('storymaker.library.title'), dataset: { stm: 'library' } },
            el('h3', { class: 'stm-panel-title', text: t('storymaker.library.title') }), tabs, search, grid, empty);

        function matches(entry) {
            if (!query) return true;
            const haystack = [S.catalogName(t, entry), entry.id, ...(Array.isArray(entry.tags) ? entry.tags : [])].join(' ').toLowerCase();
            return query.split(/\s+/).every(word => haystack.indexOf(word) >= 0);
        }

        function card(entry) {
            const scene = S.model.scenesOf(inst.story)[inst.selected];
            const name = S.catalogName(t, entry);
            const isCode = !scene || scene.kind === 'code';
            const present = entry.kind === 'character' ? inst.actorInScene(entry) : entry.kind === 'environment' ? !!scene && scene.environment === entry.id : false;
            const disabled = inst.readonly || isCode;
            const thumb = typeof entry.thumb === 'string' && THUMB_RE.test(entry.thumb) ? el('img', { class: 'stm-card-thumb', src: '/img/storymaker/' + entry.thumb, alt: '', width: '96', height: '96', loading: 'lazy', decoding: 'async' }) : icon('sparkle', 'stm-card-thumb');
            const action = entry.kind === 'environment' ? t('storymaker.library.use') : t('storymaker.library.add');
            return el('li', { class: 'stm-card-item' }, el('button', {
                type: 'button', class: 'stm-card', dataset: { stm: 'library-card', id: entry.id, kind: entry.kind, present: String(present) }, disabled: disabled || null,
                ariaLabel: action + ': ' + name, ariaPressed: entry.kind === 'prop' ? null : String(present),
                title: isCode ? t('storymaker.library.code_scene') : action,
                onClick: () => {
                    if (entry.kind === 'character') inst.addActor(entry);
                    else if (entry.kind === 'prop') inst.addProp(entry);
                    else inst.setEnvironment(entry.id);
                }
            }, thumb, el('span', { class: 'stm-card-name', text: name }), present ? el('span', { class: 'stm-card-mark' }, icon('check')) : null));
        }

        function paint() {
            tabs.querySelectorAll('[data-kind]').forEach(button => button.setAttribute('aria-selected', String(button.dataset.kind === tab)));
            S.clear(grid);
            const entries = S.model.entriesOf(inst.catalog, tab).filter(matches);
            entries.forEach(entry => grid.appendChild(card(entry)));
            empty.hidden = entries.length > 0;
        }

        search.addEventListener('input', () => { query = search.value.trim().toLowerCase(); paint(); });
        const off = [inst.bus.on('story', paint), inst.bus.on('selection', paint), inst.bus.on('readonly', paint), inst.bus.on('catalog', paint)];
        paint();
        return { el: root, paint, dispose() { off.forEach(fn => fn()); } };
    }

    S.createInspector = createInspector;
    S.createLibrary = createLibrary;
})();
```

`ui/js/desktop/apps/storymaker-code.js`:

```javascript
// Storymaker app code tab: story.json and the code scenes in the vendored CodeMirror (the shared ESM bundle, a dynamic import;
// when it cannot load, a plain text area stays). Saving story.json sends the text with If-Match and takes the story the server
// answers with (defaults applied) back into the editor; saving a code scene sends the source with If-Match (or
// If-None-Match: * for a new file) and shows the bundler's errors at their line and column. Validation issues are shown inline:
// the JSON Pointer of an issue is walked through the text (S.model.locate), so the mark sits on the field. Drafts survive tab
// and file switches. Issue texts are server data and appear as text only. Classic script, publishes S.createCode.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const CODEMIRROR_URL = '/js/vendor/codemirror-bundle.esm.js';
    const PROBLEMS_SHOWN = 12;
    const TEMPLATE = [
        'export default function scene(api) {',
        '  api.duration(5);',
        "  api.text('Hello', 'title');",
        '}',
        ''
    ].join('\n');
    let codeMirrorPromise = null;

    function loadCodeMirror() {
        if (!codeMirrorPromise) {
            codeMirrorPromise = import(CODEMIRROR_URL).catch(error => {
                codeMirrorPromise = null;
                throw error;
            });
        }
        return codeMirrorPromise;
    }

    // The shell theme decides between the light and the dark syntax palette, like Code Studio.
    function lightScheme() {
        const body = document.body;
        if (!body) return false;
        if (body.dataset.scheme) return body.dataset.scheme === 'light';
        if (body.dataset.theme === 'fruity') return body.dataset.fruityMode !== 'dark';
        return body.dataset.theme === 'light';
    }

    // createEditor(host, options) -> handle. A text area at once, CodeMirror swapped in when its bundle has loaded.
    // options: {label, onChange(text), onSave(), load}. Diagnostics are {from, to, severity: error|warning|info, message}.
    function createEditor(host, options) {
        const o = options || {};
        const { el } = S;
        const area = el('div', { class: 'stm-editor-area' });
        const textarea = el('textarea', { class: 'stm-editor-text', spellcheck: 'false', autocapitalize: 'off', autocomplete: 'off', wrap: 'off', ariaLabel: o.label || '', dataset: { stm: 'editor-text' } });
        area.appendChild(textarea);
        host.appendChild(area);
        let view = null, cm = null, languageSlot = null, readSlot = null, schemeSlot = null, light = lightScheme(), applying = false, disposed = false, observer = null;
        let language = 'json', readOnly = false, diagnostics = [];
        const handle = { kind: 'textarea' };

        const getText = () => (view ? view.state.doc.toString() : textarea.value);
        const changed = () => { if (!applying && !disposed && o.onChange) o.onChange(getText()); };
        textarea.addEventListener('input', changed);
        textarea.addEventListener('keydown', event => {
            if ((event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === 's') {
                event.preventDefault();
                if (o.onSave) o.onSave();
            }
        });

        function languageExtension() {
            if (!cm) return [];
            return language === 'javascript' ? cm.javascript() : cm.json();
        }
        function readExtensions() { return [cm.EditorState.readOnly.of(readOnly), cm.EditorView.editable.of(!readOnly)]; }
        function schemeExtensions() {
            if (light) return cm.syntaxHighlighting && cm.defaultHighlightStyle ? [cm.syntaxHighlighting(cm.defaultHighlightStyle)] : [];
            return cm.oneDark ? [cm.oneDark] : [];
        }
        function theme() {
            return cm.EditorView.theme({
                '&': { height: '100%', fontSize: '13px', backgroundColor: 'var(--vd-mat-well-color, var(--vd-theme-app-bg))', color: 'var(--vd-text)' },
                '.cm-scroller': { fontFamily: 'var(--vd-mat-mono-font, var(--ds-font-mono, ui-monospace, SFMono-Regular, Consolas, monospace))', lineHeight: '1.5' },
                '.cm-content': { caretColor: 'var(--vd-accent)' },
                '.cm-cursor': { borderLeftColor: 'var(--vd-accent)', borderLeftWidth: '2px' },
                '.cm-gutters': { backgroundColor: 'var(--vd-mat-side-color, var(--vd-theme-panel-bg))', color: 'var(--vd-theme-muted)', borderRight: '1px solid var(--vd-theme-border)' },
                '.cm-activeLine': { backgroundColor: 'color-mix(in srgb, var(--vd-accent) 8%, transparent)' },
                '.cm-activeLineGutter': { backgroundColor: 'color-mix(in srgb, var(--vd-accent) 12%, transparent)' },
                '&.cm-focused': { outline: 'var(--ds-focus-ring-width, 2px) solid var(--ds-color-border-focus)', outlineOffset: '-2px' },
                '.cm-tooltip': { backgroundColor: 'var(--vd-theme-panel-bg-strong)', color: 'var(--vd-text)', border: '1px solid var(--vd-theme-border)' }
            }, { dark: !light });
        }
        function applyMarks() {
            if (!view) return;
            const length = view.state.doc.length;
            const items = diagnostics.map(d => {
                const from = Math.min(Math.max(0, d.from), length);
                return { from, to: Math.min(Math.max(from, d.to), length), severity: d.severity, message: d.message, source: 'storymaker' };
            });
            view.dispatch(cm.setDiagnostics(view.state, items));
        }

        function swap(module) {
            if (disposed) return;
            cm = module;
            languageSlot = new cm.Compartment();
            readSlot = new cm.Compartment();
            schemeSlot = new cm.Compartment();
            const keys = [
                { key: 'Mod-s', preventDefault: true, run: () => { if (o.onSave) o.onSave(); return true; } },
                { key: 'Escape', run: () => { if (!o.onEscape) return false; o.onEscape(); return true; } },
                cm.indentWithTab, ...(cm.closeBracketsKeymap || []), ...(cm.defaultKeymap || []), ...(cm.searchKeymap || []), ...(cm.historyKeymap || []), ...(cm.lintKeymap || [])
            ].filter(Boolean);
            const extensions = [
                cm.lineNumbers(), cm.highlightActiveLineGutter(), cm.highlightSpecialChars(), cm.history(), cm.drawSelection(), cm.highlightActiveLine(),
                cm.indentUnit.of('  '), cm.EditorState.tabSize.of(2), cm.closeBrackets(), cm.highlightSelectionMatches(),
                cm.lintGutter(), cm.linter(null),
                languageSlot.of(languageExtension()), schemeSlot.of(schemeExtensions()), theme(), readSlot.of(readExtensions()),
                cm.EditorView.contentAttributes.of({ 'aria-label': o.label || '' }),
                cm.keymap.of(keys),
                cm.EditorView.updateListener.of(update => { if (update.docChanged && !applying) changed(); })
            ];
            const wasFocused = document.activeElement === textarea;
            const caret = textarea.selectionStart || 0;
            const cmHost = el('div', { class: 'stm-editor-cm' });
            area.appendChild(cmHost);
            view = new cm.EditorView({ state: cm.EditorState.create({ doc: textarea.value, extensions }), parent: cmHost });
            textarea.hidden = true;
            handle.kind = 'codemirror';
            host.dataset.editor = 'codemirror';
            applyMarks();
            if (wasFocused) { view.focus(); view.dispatch({ selection: { anchor: Math.min(caret, view.state.doc.length) } }); }
            if (typeof MutationObserver === 'function' && document.body) {
                observer = new MutationObserver(() => {
                    const next = lightScheme();
                    if (next === light || !view) return;
                    light = next;
                    view.dispatch({ effects: schemeSlot.reconfigure(schemeExtensions()) });
                });
                observer.observe(document.body, { attributes: true, attributeFilter: ['data-theme', 'data-fruity-mode', 'data-scheme'] });
            }
            if (o.onReady) o.onReady(handle);
        }

        Object.assign(handle, {
            getText,
            setText(next) {
                const value = String(next == null ? '' : next);
                applying = true;
                try {
                    if (view) view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } });
                    else textarea.value = value;
                } finally { applying = false; }
            },
            setLanguage(next) {
                language = next === 'javascript' ? 'javascript' : 'json';
                if (view) view.dispatch({ effects: languageSlot.reconfigure(languageExtension()) });
            },
            setReadOnly(next) {
                readOnly = !!next;
                textarea.readOnly = readOnly;
                if (view) view.dispatch({ effects: readSlot.reconfigure(readExtensions()) });
            },
            setDiagnostics(list) { diagnostics = Array.isArray(list) ? list : []; applyMarks(); },
            gotoOffset(offset) {
                if (view) {
                    const pos = Math.min(Math.max(0, offset), view.state.doc.length);
                    view.dispatch({ selection: { anchor: pos }, effects: cm.EditorView.scrollIntoView(pos, { y: 'center' }) });
                    view.focus();
                } else {
                    textarea.focus();
                    textarea.setSelectionRange(Math.min(offset, textarea.value.length), Math.min(offset, textarea.value.length));
                }
            },
            focus() { if (view) view.focus(); else textarea.focus(); },
            dispose() {
                if (disposed) return;
                disposed = true;
                if (observer) observer.disconnect();
                if (view) view.destroy();
                view = null;
                S.clear(host);
            }
        });
        handle.ready = (o.load || loadCodeMirror)().then(swap, error => { console.warn('Storymaker: CodeMirror failed to load, using a plain text area', error); });
        return handle;
    }

    // inst: {t, api, bus, story, etag, issues, report, readonly, projectId, codeStatus, saveStoryText(text), saveCodeScene(id, text, etag), flushEdits(), focusIssue(issue), notify(text)}
    function createCode(inst) {
        const { el, icon } = S;
        const t = inst.t;
        const files = new Map();       // key -> {key, label, kind, sceneId, text, base, etag, loaded, loading, isNew, errors}
        let current = 'story';
        let editor = null;
        let busy = false;
        let staleMarks = false;
        let disposed = false;

        const fileList = el('ul', { class: 'stm-code-files', dataset: { stm: 'code-files' }, ariaLabel: t('storymaker.code.files') });
        const fileLabel = el('strong', { class: 'stm-code-file', dataset: { stm: 'code-file' } });
        const stateLabel = el('span', { class: 'stm-code-state', role: 'status', dataset: { stm: 'code-state' } });
        const saveButton = el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'code-save' }, onClick: () => save() }, icon('check'), el('span', { text: t('storymaker.code.save') }));
        const reloadButton = el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'code-reload' }, onClick: () => reloadFile() }, icon('refresh'), el('span', { text: t('storymaker.code.reload') }));
        const editorHost = el('div', { class: 'stm-editor', dataset: { stm: 'code-editor' } });
        const problems = el('div', { class: 'stm-problems', dataset: { stm: 'code-problems' } });
        const root = el('section', { class: 'stm-code', ariaLabel: t('storymaker.code.title'), dataset: { stm: 'code' } },
            el('nav', { class: 'stm-code-nav' }, el('h3', { class: 'stm-panel-title', text: t('storymaker.code.files') }), fileList),
            el('div', { class: 'stm-code-main' },
                el('div', { class: 'stm-code-bar' }, fileLabel, stateLabel, el('span', { class: 'stm-spacer' }), reloadButton, saveButton),
                editorHost, problems));

        function fileFor(key) {
            if (files.has(key)) return files.get(key);
            const sceneId = key.indexOf('scene:') === 0 ? key.slice(6) : '';
            const file = { key, label: sceneId ? sceneId + '.js' : 'story.json', kind: sceneId ? 'code' : 'story', sceneId, text: '', base: '', etag: '', loaded: false, loading: false, isNew: false, errors: [] };
            files.set(key, file);
            return file;
        }

        function isDirty(file) { return file.loaded && file.text !== file.base; }

        function codeSceneIds() {
            return S.model.scenesOf(inst.story).filter(scene => scene && scene.kind === 'code').map(scene => scene.id);
        }

        function paintFiles() {
            S.clear(fileList);
            const keys = ['story'].concat(codeSceneIds().map(id => 'scene:' + id));
            keys.forEach(key => {
                const file = fileFor(key);
                const status = file.sceneId ? (inst.codeStatus || []).find(item => item.scene_id === file.sceneId) : null;
                const failed = !!file.errors.length || (status && status.ok === false);
                fileList.appendChild(el('li', {}, el('button', {
                    type: 'button', class: 'stm-code-file-btn', dataset: { stm: 'code-file-btn', key, current: String(key === current), failed: String(!!failed), dirty: String(isDirty(file)) },
                    ariaCurrent: key === current ? 'true' : null,
                    onClick: () => show(key)
                }, icon(file.kind === 'story' ? 'film' : 'code'), el('span', { class: 'stm-code-file-name', text: file.label }), isDirty(file) ? el('span', { class: 'stm-dot', title: t('storymaker.code.unsaved') }) : null, failed ? el('span', { class: 'stm-dot stm-dot-bad', title: t('storymaker.code.bundle_failed') }) : null)));
            });
        }

        function paintState() {
            const file = fileFor(current);
            fileLabel.textContent = file.label;
            const dirty = isDirty(file);
            let text = '';
            if (file.loading) text = t('storymaker.loading');
            else if (staleMarks) text = t('storymaker.code.stale');
            else if (dirty) text = t('storymaker.code.unsaved');
            else if (file.isNew) text = t('storymaker.code.new_file');
            else if (file.loaded) text = t('storymaker.code.saved');
            stateLabel.textContent = text;
            stateLabel.dataset.state = file.loading ? 'loading' : dirty ? 'dirty' : 'saved';
            saveButton.disabled = inst.readonly || busy || !file.loaded || (!dirty && !file.isNew);
            reloadButton.disabled = busy || file.loading;
            if (editor) editor.setReadOnly(inst.readonly || file.loading);
            paintFiles();
        }

        // ---- problems ----------------------------------------------------------------------------------------------------

        function storyDiagnostics(file) {
            const text = file.text;
            const list = [];
            if (staleMarks) return list;
            const issues = S.model.visibleIssues(S.model.mergeIssues(inst.issues, inst.report ? inst.report.issues : []));
            issues.forEach(issue => {
                let from = 0, to = 0, found = false;
                const where = S.model.positionFromMessage(issue.message);
                if (!issue.path && !(issue.code === 'invalid_json' && where)) return;
                if ((issue.code === 'invalid_json') && where) {
                    from = S.model.offsetOf(text, where.line, where.column);
                    to = from + 1;
                    found = true;
                } else {
                    const hit = S.model.locate(text, issue.path);
                    from = hit.from;
                    to = hit.to;
                    found = hit.found || hit.depth > 0;
                }
                if (!found && issue.path) return;
                const lineEnd = text.indexOf('\n', from);
                to = Math.min(to, lineEnd < 0 ? text.length : lineEnd);
                list.push({ from, to: Math.max(to, from), severity: issue.severity === 'error' ? 'error' : issue.severity === 'warning' ? 'warning' : 'info', message: S.errors.issueTitle(t, issue.code) + ': ' + issue.message, issue });
            });
            return list;
        }

        function codeDiagnostics(file) {
            return file.errors.map(error => {
                const from = S.model.offsetOf(file.text, error.line || 1, error.column || 1);
                const lineEnd = file.text.indexOf('\n', from);
                return { from, to: lineEnd < 0 ? file.text.length : Math.max(from, lineEnd), severity: 'error', message: error.message, line: error.line || 1, column: error.column || 1 };
            });
        }

        function paintProblems() {
            const file = fileFor(current);
            const list = file.kind === 'story' ? storyDiagnostics(file) : codeDiagnostics(file);
            if (editor) editor.setDiagnostics(list);
            S.clear(problems);
            problems.dataset.count = String(list.length);
            if (!list.length) {
                problems.appendChild(el('p', { class: 'stm-muted stm-problems-none', text: t('storymaker.code.no_problems') }));
                return;
            }
            problems.appendChild(el('p', { class: 'stm-problems-summary', role: 'status', text: t('storymaker.code.problem_count', { count: list.length }) }));
            const rows = el('ul', { class: 'stm-problems-list' });
            list.slice(0, PROBLEMS_SHOWN).forEach(item => {
                const position = S.model.lineColumn(file.text, item.from);
                rows.appendChild(el('li', { class: 'stm-problem', dataset: { severity: item.severity } },
                    el('button', { type: 'button', class: 'stm-problem-go', onClick: () => { if (editor) editor.gotoOffset(item.from); } },
                        icon(item.severity === 'error' ? 'alert' : 'info'),
                        el('span', { class: 'stm-problem-line', text: t('storymaker.code.line_col', { line: position.line, column: position.column }) }),
                        el('span', { class: 'stm-problem-text', text: item.message }))));
            });
            if (list.length > PROBLEMS_SHOWN) rows.appendChild(el('li', { class: 'stm-muted', text: t('storymaker.code.more', { count: list.length - PROBLEMS_SHOWN }) }));
            problems.appendChild(rows);
        }

        // ---- files -------------------------------------------------------------------------------------------------------

        function storyText() { return S.model.canonicalText(inst.story); }

        async function loadFile(file) {
            if (file.kind === 'story') {
                file.text = storyText();
                file.base = file.text;
                file.loaded = true;
                return;
            }
            file.loading = true;
            paintState();
            try {
                const result = await inst.api.getCode(inst.projectId, file.sceneId);
                file.text = result.text;
                file.base = result.text;
                file.etag = result.etag;
                file.isNew = false;
            } catch (err) {
                if (err && err.status === 404) {
                    file.text = TEMPLATE;
                    file.base = '';
                    file.etag = '';
                    file.isNew = true;
                } else {
                    file.loading = false;
                    inst.notify(S.errors.describe(t, err), 'error');
                    paintState();
                    return;
                }
            }
            file.loading = false;
            file.loaded = true;
            const status = (inst.codeStatus || []).find(item => item.scene_id === file.sceneId);
            file.errors = status && status.ok === false && Array.isArray(status.errors) ? status.errors : [];
        }

        function ensureEditor() {
            if (editor || disposed) return;
            editor = createEditor(editorHost, {
                label: t('storymaker.code.editor_label'),
                onChange(text) {
                    const file = fileFor(current);
                    if (!file.loaded) return;
                    file.text = text;
                    if (!staleMarks) { staleMarks = true; paintProblems(); }
                    paintState();
                },
                onSave: () => save(),
                onEscape: () => { const button = fileList.querySelector('[aria-current="true"]'); if (button) button.focus(); },
                onReady: () => { editor.setLanguage(fileFor(current).kind === 'code' ? 'javascript' : 'json'); editor.setReadOnly(inst.readonly); paintProblems(); }
            });
        }

        async function show(key) {
            const file = fileFor(key || current);
            current = file.key;
            ensureEditor();
            if (!file.loaded) {
                paintState();
                await loadFile(file);
                if (disposed || current !== file.key) return;
            } else if (file.kind === 'story' && !isDirty(file)) {
                // the story may have changed in the inspector meanwhile
                file.text = storyText();
                file.base = file.text;
            }
            staleMarks = false;
            editor.setLanguage(file.kind === 'code' ? 'javascript' : 'json');
            editor.setText(file.text);
            editor.setReadOnly(inst.readonly);
            paintState();
            paintProblems();
        }

        async function reloadFile() {
            const file = fileFor(current);
            file.loaded = false;
            file.errors = [];
            await show(file.key);
        }

        async function save() {
            const file = fileFor(current);
            if (busy || inst.readonly || !file.loaded) return false;
            busy = true;
            paintState();
            const sent = file.text;
            try {
                if (file.kind === 'story') {
                    const result = await inst.saveStoryText(sent);
                    if (result.ok) {
                        staleMarks = false;
                        file.text = storyText();
                        file.base = file.text;
                        if (fileFor(current) === file && editor && editor.getText() === sent) editor.setText(file.text);
                    } else {
                        staleMarks = false;
                    }
                } else {
                    const result = await inst.saveCodeScene(file.sceneId, sent, file.isNew ? null : file.etag);
                    file.etag = result.etag || file.etag;
                    file.base = sent;
                    file.isNew = false;
                    file.errors = result.ok ? [] : (Array.isArray(result.errors) ? result.errors : []);
                    staleMarks = false;
                }
            } catch (err) {
                inst.notify(S.errors.describe(t, err), 'error');
            } finally {
                busy = false;
                paintState();
                paintProblems();
            }
            return true;
        }

        const off = [
            inst.bus.on('story', () => { if (!disposed) { paintFiles(); if (root.dataset.visible === 'true') paintProblems(); } }),
            inst.bus.on('issues', () => { if (!disposed && root.dataset.visible === 'true') paintProblems(); }),
            inst.bus.on('readonly', () => paintState())
        ];

        return {
            el: root,
            show(key) { root.dataset.visible = 'true'; return show(key); },
            hide() { root.dataset.visible = 'false'; },
            current: () => current,
            dirty: () => Array.from(files.values()).some(isDirty),
            // a save elsewhere changed the story: an untouched story.json follows, a draft stays
            storyChanged() {
                const file = fileFor('story');
                if (file.loaded && !isDirty(file)) { file.text = storyText(); file.base = file.text; if (editor && current === 'story' && root.dataset.visible === 'true') editor.setText(file.text); }
                paintFiles();
            },
            // put the caret on the field an issue names (story.json must be the open file)
            gotoPath(issue) {
                const file = fileFor('story');
                if (!editor || current !== 'story' || !file.loaded) return false;
                const where = S.model.positionFromMessage(issue.message);
                const offset = issue.code === 'invalid_json' && where ? S.model.offsetOf(file.text, where.line, where.column) : S.model.locate(file.text, issue.path).from;
                editor.gotoOffset(offset);
                return true;
            },
            forget(sceneId) { files.delete('scene:' + sceneId); if (current === 'scene:' + sceneId) current = 'story'; paintFiles(); },
            paint() { paintState(); paintProblems(); },
            dispose() {
                disposed = true;
                off.forEach(fn => fn());
                if (editor) editor.dispose();
                editor = null;
            }
        };
    }

    S.createCode = createCode;
    S.codeTemplate = TEMPLATE;
    S.createEditor = createEditor;
})();
```

`ui/js/desktop/apps/storymaker-export.js`:

```javascript
// Storymaker app export: the toolbar button and its panel. The panel says what is missing before anything starts (FFmpeg, a
// browser, read-only, errors in the story, a story longer than the limit) and what still works; a start sends the saved story's
// ETag with an idempotency key; progress comes from the project's server-sent events (a poll of the job runs as a safety net
// while a job is active); the result is the project-folder path of the MP4 with "show in Files" and copy. Job errors are shown
// by their stable code. Server texts are shown as text only. Classic script, publishes S.createExport.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const ACTIVE = ['queued', 'running'];
    const POLL_MS = 2500;
    const PHASES = ['starting', 'compiling', 'mixing', 'rendering', 'encoding', 'writing'];

    function dirOf(path) {
        const text = String(path || '');
        const cut = text.lastIndexOf('/');
        return cut > 0 ? text.slice(0, cut) : '';
    }

    // inst: {t, api, bus, status, readonly, projectId, etag, story, report, issues, ctx, flushEdits(), showIssues(), notify(text, kind)}
    function createExport(inst) {
        const { el, icon } = S;
        const t = inst.t;
        let job = null;
        let open = false;
        let starting = false;
        let cancelling = false;
        let pollTimer = 0;
        let copied = 0;

        const buttonLabel = el('span', { class: 'stm-export-label', text: t('storymaker.export.button') });
        const button = el('button', { type: 'button', class: 'stm-btn stm-btn-primary stm-export-btn', dataset: { stm: 'export', state: 'idle' }, ariaHaspopup: 'dialog', ariaExpanded: 'false', onClick: () => toggle() }, icon('export'), buttonLabel);
        const panel = el('div', { class: 'stm-export-panel', role: 'dialog', ariaLabel: t('storymaker.export.title'), hidden: true, dataset: { stm: 'export-panel' } });
        const root = el('div', { class: 'stm-export' }, button, panel);

        function limits() { return (inst.status && inst.status.limits) || {}; }
        function isActive() { return !!job && ACTIVE.indexOf(job.status) >= 0; }

        function capabilityRow(ok, name, detail, extra) {
            return el('li', { class: 'stm-cap', dataset: { ok: String(ok) } },
                icon(ok ? 'check' : 'alert'),
                el('div', { class: 'stm-cap-text' },
                    el('strong', { text: name }),
                    el('span', { class: 'stm-muted', text: detail }),
                    extra || null));
        }

        // What blocks an export, as {key, ...vars} reasons in the order the panel lists them.
        function blockers() {
            const list = [];
            const status = inst.status || {};
            if (inst.readonly) list.push('readonly');
            if (!(status.ffmpeg && status.ffmpeg.ready)) list.push('ffmpeg');
            if (!(status.browser && status.browser.ready)) list.push('browser');
            const counts = S.model.issueCounts(S.model.visibleIssues(S.model.mergeIssues(inst.issues, inst.report ? inst.report.issues : [])));
            if (counts.error > 0) list.push('errors');
            const max = Number(limits().max_duration_seconds) || 0;
            if (max > 0 && inst.report && inst.report.duration_ms / 1000 > max) list.push('too_long');
            if (!inst.report || !(inst.report.frame_count > 0)) list.push('not_ready');
            return list;
        }

        function capabilities() {
            const status = inst.status || {};
            const ffmpeg = status.ffmpeg || {};
            const browser = status.browser || {};
            const list = el('ul', { class: 'stm-caps', dataset: { stm: 'export-caps' } });
            list.appendChild(capabilityRow(!!ffmpeg.ready, t('storymaker.export.cap_ffmpeg'),
                ffmpeg.ready ? (ffmpeg.version ? t('storymaker.export.ready_version', { version: String(ffmpeg.version).slice(0, 60) }) : t('storymaker.export.ready')) : t('storymaker.export.ffmpeg_missing'),
                !ffmpeg.ready && ffmpeg.issue ? el('code', { class: 'stm-cap-issue', text: String(ffmpeg.issue).slice(0, 400) }) : null));
            list.appendChild(capabilityRow(!!browser.ready, t('storymaker.export.cap_browser'),
                browser.ready ? t('storymaker.export.browser_ready', { source: sourceLabel(browser.source) }) : t('storymaker.export.browser_missing'),
                !browser.ready && browser.issue ? el('code', { class: 'stm-cap-issue', text: String(browser.issue).slice(0, 400) }) : null));
            return list;
        }

        function sourceLabel(source) {
            if (source === 'config' || source === 'system' || source === 'pinned') return t('storymaker.export.source_' + source);
            return t('storymaker.export.source_unknown');
        }

        function blockerText(reason) {
            const max = Number(limits().max_duration_seconds) || 0;
            switch (reason) {
                case 'readonly': return t('storymaker.export.blocked_readonly');
                case 'ffmpeg': return t('storymaker.export.blocked_ffmpeg');
                case 'browser': return t('storymaker.export.blocked_browser');
                case 'errors': return t('storymaker.export.blocked_errors');
                case 'too_long': return t('storymaker.export.blocked_too_long', { max: S.format.seconds(max) });
                default: return t('storymaker.export.blocked_not_ready');
            }
        }

        function progressView() {
            const fraction = job && Number.isFinite(job.progress) ? S.clamp(job.progress, 0, 1) : 0;
            const phase = job && PHASES.indexOf(job.phase) >= 0 ? t('storymaker.export.phase_' + job.phase) : t('storymaker.export.phase_starting');
            const frames = job && job.frames_total > 0 ? t('storymaker.export.frames', { done: job.frames_done || 0, total: job.frames_total }) : '';
            const waiting = job && job.status === 'queued';
            return el('div', { class: 'stm-job', dataset: { stm: 'export-progress', status: job ? job.status : '' } },
                el('div', { class: 'stm-job-line' }, el('strong', { text: waiting ? t('storymaker.export.queued') : phase }), el('span', { class: 'stm-muted', text: S.format.percent(fraction) })),
                el('progress', { class: 'stm-progress', max: '1', value: String(fraction), ariaLabel: t('storymaker.export.title') }),
                frames ? el('p', { class: 'stm-hint', text: frames }) : null,
                el('div', { class: 'stm-actions' },
                    el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'export-cancel' }, disabled: cancelling || null, onClick: () => cancel() }, icon('stop'), el('span', { text: cancelling ? t('storymaker.export.cancelling') : t('storymaker.export.cancel') }))));
        }

        function resultView() {
            const output = job.output || {};
            const dir = dirOf(output.path);
            return el('div', { class: 'stm-job', dataset: { stm: 'export-done', status: 'succeeded' } },
                el('p', { class: 'stm-job-title' }, icon('check'), el('strong', { text: t('storymaker.export.done') })),
                el('p', { class: 'stm-muted', text: t('storymaker.export.output') }),
                el('code', { class: 'stm-path', dataset: { stm: 'export-path' }, text: String(output.path || '') }),
                output.size ? el('p', { class: 'stm-hint', text: S.format.bytes(output.size) }) : null,
                el('div', { class: 'stm-actions' },
                    el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'export-show' }, disabled: !dir || typeof inst.ctx.openApp !== 'function' || null, onClick: () => { if (dir) inst.ctx.openApp('files', { path: dir }); } }, icon('folder'), el('span', { text: t('storymaker.export.show_in_files') })),
                    el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'export-copy' }, onClick: () => copyPath(output.path) }, icon('copy'), el('span', { text: copied && Date.now() - copied < 2500 ? t('storymaker.export.copied') : t('storymaker.export.copy_path') })),
                    el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'export-again' }, onClick: () => { job = null; paint(); } }, el('span', { text: t('storymaker.export.again') }))));
        }

        function failedView() {
            const message = job.status === 'cancelled' ? t('storymaker.export.cancelled') : job.status === 'interrupted' ? t('storymaker.export.interrupted') : S.errors.describeJob(t, job.error);
            return el('div', { class: 'stm-job', dataset: { stm: 'export-failed', status: job.status } },
                el('p', { class: 'stm-job-title stm-job-bad' }, icon('alert'), el('strong', { text: message })),
                job.error && job.error.message && job.status === 'failed' ? el('code', { class: 'stm-cap-issue', text: String(job.error.message).slice(0, 600) }) : null,
                el('div', { class: 'stm-actions' }, el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'export-again' }, onClick: () => { job = null; paint(); } }, icon('refresh'), el('span', { text: t('storymaker.export.again') }))));
        }

        function startView() {
            const reasons = blockers();
            const blocked = reasons.length > 0;
            const view = el('div', { class: 'stm-job', dataset: { stm: 'export-ready', blocked: String(blocked) } });
            if (blocked) {
                const list = el('ul', { class: 'stm-blockers', dataset: { stm: 'export-blockers' } });
                reasons.forEach(reason => list.appendChild(el('li', { dataset: { reason }, text: blockerText(reason) })));
                view.appendChild(list);
                if (reasons.indexOf('ffmpeg') >= 0 || reasons.indexOf('browser') >= 0) view.appendChild(el('p', { class: 'stm-hint', text: t('storymaker.export.still_works') }));
                if (reasons.indexOf('errors') >= 0) view.appendChild(el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'export-issues' }, onClick: () => { close(); inst.showIssues(); } }, icon('alert'), el('span', { text: t('storymaker.issues.show') })));
            } else {
                const seconds = inst.report ? inst.report.duration_ms / 1000 : 0;
                view.appendChild(el('p', { class: 'stm-muted', text: t('storymaker.export.summary', { duration: S.format.clock(inst.report.frame_count, inst.report.fps), format: (inst.story.format ? inst.story.format.aspect : '') + ' · ' + (inst.report.fps || '') + ' fps', width: inst.report.width || '', height: inst.report.height || '' }) }));
                if (seconds > 0) view.appendChild(el('p', { class: 'stm-hint', text: t('storymaker.export.silent_note') }));
            }
            view.appendChild(el('div', { class: 'stm-actions' },
                el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'export-start' }, disabled: blocked || starting || null, onClick: () => start() }, icon('film'), el('span', { text: starting ? t('storymaker.export.starting') : t('storymaker.export.start') }))));
            return view;
        }

        function paint() {
            const state = isActive() ? 'running' : job ? job.status : 'idle';
            button.dataset.state = state;
            const fraction = isActive() && job && Number.isFinite(job.progress) ? S.clamp(job.progress, 0, 1) : 0;
            button.style.setProperty('--stm-export-progress', Math.round(fraction * 100) + '%');
            buttonLabel.textContent = isActive() ? t('storymaker.export.button_running', { percent: Math.round(fraction * 100) }) : t('storymaker.export.button');
            button.setAttribute('aria-expanded', String(open));
            panel.hidden = !open;
            if (!open) return;
            S.clear(panel);
            panel.appendChild(el('div', { class: 'stm-export-head' }, el('h3', { text: t('storymaker.export.title') }),
                el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', ariaLabel: t('storymaker.close'), title: t('storymaker.close'), dataset: { stm: 'export-close' }, onClick: () => close() }, icon('close'))));
            panel.appendChild(capabilities());
            if (isActive()) panel.appendChild(progressView());
            else if (job && job.status === 'succeeded') panel.appendChild(resultView());
            else if (job) panel.appendChild(failedView());
            else panel.appendChild(startView());
        }

        // ---- job tracking ------------------------------------------------------------------------------------------------

        function schedulePoll() {
            clearTimeout(pollTimer);
            pollTimer = 0;
            if (!isActive()) return;
            pollTimer = setTimeout(async () => {
                pollTimer = 0;
                if (!job) return;
                try { apply(await inst.api.getJob(job.id)); } catch (err) {
                    if (err && err.aborted) return;
                    schedulePoll();
                }
            }, POLL_MS);
        }

        // apply(next, adopt): a job event of this project. Without a tracked job only an active one is taken up (a start by
        // another window); a finished one is shown when this panel started it (adopt).
        function apply(next, adopt) {
            if (!next || typeof next !== 'object' || typeof next.id !== 'string') return;
            if (next.project_id && inst.projectId && next.project_id !== inst.projectId) return;
            if (job && job.id !== next.id && (isActive() || ACTIVE.indexOf(next.status) < 0)) return;
            if (!job && !adopt && ACTIVE.indexOf(next.status) < 0) return;
            if (job && job.id !== next.id) job = null;
            const was = job ? job.status : '';
            job = next;
            if (!isActive()) cancelling = false;
            if (was && was !== job.status && !isActive()) {
                inst.notify(job.status === 'succeeded' ? t('storymaker.export.done') : job.status === 'cancelled' ? t('storymaker.export.cancelled') : job.status === 'interrupted' ? t('storymaker.export.interrupted') : S.errors.describeJob(t, job.error), job.status === 'succeeded' ? 'info' : 'error');
            }
            paint();
            schedulePoll();
        }

        async function start() {
            if (starting || isActive() || blockers().length) return;
            starting = true;
            paint();
            try {
                const saved = await inst.flushEdits();
                if (!saved) return;
                if (blockers().length) return;
                const created = await inst.api.startRender(inst.projectId, inst.etag, S.uuid());
                job = null;
                apply(created, true);
            } catch (err) {
                if (err && err.aborted) return;
                if (err && err.status === 412) inst.bus.emit('conflict', err);
                inst.notify(S.errors.describe(t, err), 'error');
            } finally {
                starting = false;
                paint();
            }
        }

        async function cancel() {
            if (!isActive() || cancelling) return;
            cancelling = true;
            paint();
            try {
                apply(await inst.api.cancelJob(job.id));
            } catch (err) {
                cancelling = false;
                inst.notify(S.errors.describe(t, err), 'error');
                paint();
            }
        }

        async function copyPath(path) {
            try {
                await navigator.clipboard.writeText(String(path || ''));
                copied = Date.now();
            } catch (_) {
                inst.notify(t('storymaker.export.copy_failed'), 'error');
                return;
            }
            paint();
            setTimeout(() => { if (open) paint(); }, 2600);
        }

        // ---- popover -----------------------------------------------------------------------------------------------------

        function onDocumentPointer(event) {
            if (open && !root.contains(event.target)) close(false);
        }
        function onDocumentKey(event) {
            if (open && event.key === 'Escape') { event.stopPropagation(); close(true); }
        }
        document.addEventListener('pointerdown', onDocumentPointer, true);
        document.addEventListener('keydown', onDocumentKey, true);

        function toggle() { if (open) close(true); else openPanel(); }
        function openPanel() { open = true; paint(); const first = panel.querySelector('button:not([disabled])'); if (first) first.focus(); }
        function close(restoreFocus) { open = false; paint(); if (restoreFocus) button.focus(); }

        const off = [
            inst.bus.on('job', next => apply(next, false)),
            inst.bus.on('report', () => { if (open) paint(); }),
            inst.bus.on('issues', () => { if (open) paint(); }),
            inst.bus.on('status', () => { if (open) paint(); }),
            inst.bus.on('readonly', () => { if (open) paint(); })
        ];
        paint();

        return {
            el: root,
            open: openPanel,
            close,
            job: () => job,
            apply,
            dispose() {
                clearTimeout(pollTimer);
                document.removeEventListener('pointerdown', onDocumentPointer, true);
                document.removeEventListener('keydown', onDocumentKey, true);
                off.forEach(fn => fn());
            }
        };
    }

    S.createExport = createExport;
})();
```

`ui/desktop_i18n_cleanup_test.go` (base line 145, `keyPrefixes`):

```diff
--- a/ui/desktop_i18n_cleanup_test.go
+++ b/ui/desktop_i18n_cleanup_test.go
@@ -142,7 +142,7 @@ func TestDesktopAuditedI18nUsageHasNoEnglishInlineFallbacks(t *testing.T) {
 func TestDesktopUsedI18nKeysExistInAllLanguages(t *testing.T) {
 	t.Parallel()
 
-	keyPrefixes := `desktop|common|chat|config|help|cheater|codeStudio|easydrag|galaxa|homepage_studio|missions|pixel|viewer|zipper`
+	keyPrefixes := `desktop|common|chat|config|help|cheater|codeStudio|easydrag|galaxa|homepage_studio|missions|pixel|storymaker|viewer|zipper`
 	keyPattern := regexp.MustCompile(`(?s)(?:\b(?:dt|desktopT|t|tr|translate)\s*\(\s*['"]((?:` + keyPrefixes + `)\.[^'"]+)['"]|data-i18n(?:-[a-z-]+)?\s*=\s*['"]((?:` + keyPrefixes + `)\.[^'"]+)['"]|(?:labelKey|titleKey|messageKey|placeholderKey)\s*:\s*['"]((?:` + keyPrefixes + `)\.[^'"]+)['"])`)
 	used := map[string]bool{}
 	for _, asset := range desktopI18nAssets(t) {
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./ui -run 'TestStorymakerSourcesAreSafeAndSmall|TestStorymakerAppLoadsNothingExternalAndAvoidsFilteredClasses|TestStorymakerAppUsesOnlyKeysItHas|TestDesktopUsedI18nKeysExistInAllLanguages' -count=1`
Expected: ok

Recorded at drafting time:

```text
ok  	aurago/ui	3.810s
```

- [ ] **Step 5: Commit**

```bash
git add ui/desktop_storymaker_guards_test.go ui/js/desktop/apps/storymaker-projects.js ui/js/desktop/apps/storymaker-strip.js ui/js/desktop/apps/storymaker-inspector.js ui/js/desktop/apps/storymaker-code.js ui/js/desktop/apps/storymaker-export.js ui/desktop_i18n_cleanup_test.go
git commit -m "feat(storymaker): project list, scene strip, inspector, code tab and export panel" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E8: Composition root and the browser tour

`storymaker.js` is the window: the instance, the start screen, the editor layout, the save pipeline (If-Match, one save in flight, conflict banner), the player loading and thumbnails, the event stream and the disposal. The browser tour runs the real shell, the real player iframe under the real CSP and the real catalog against `storymakerFixture`: a handler for `/api/desktop/storymaker` on top of the real `internal/storymaker` package (a MemStore, `DecodeStory`, `Validate`, the bundler) with a render job the test steps by hand. `storymakerBrowserPage` takes any handler, so the integration step runs the same tour against the real server handlers.

**Files:**
- Create: `ui/js/desktop/apps/storymaker.js`
- Test: `ui/desktop_storymaker_fixture_test.go`
- Test: `ui/desktop_storymaker_browser_test.go`

- [ ] **Step 1: Write the failing tests**

`ui/desktop_storymaker_fixture_test.go`:

```go
package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/storymaker"
	"github.com/google/uuid"
)

// storymakerFixture answers /api/desktop/storymaker like the server does (contract section 5), on top of the real
// storymaker package: the projects live in a MemStore, saves go through DecodeStory, ApplyDefaults, Validate and the
// project store, so a story that the fixture accepts is one the real server accepts, and code scenes are bundled by esbuild.
// What is faked is only the world around it: the capability flags, and the render job, which a test steps by hand
// (advance) so it can look at every state. The integration step runs the same browser tour against the real handlers by
// passing them to storymakerBrowserPage instead of this fixture.
type storymakerFixture struct {
	mu       sync.Mutex
	t        *testing.T
	store    *storymaker.MemStore
	projects *storymaker.ProjectStore
	catalog  *storymaker.Catalog
	limits   storymaker.Limits

	ffmpegReady  bool
	browserReady bool
	readOnly     bool
	disabled     bool
	unauthorized bool

	requests []string
	bodies   map[string]string // last body per "METHOD route"
	jobs     map[string]*storymakerFixtureJob
	order    []string
	watchers map[string][]chan []byte
	steps    int // advance() calls so far
}

type storymakerFixtureJob struct {
	ID, ProjectID, Status, Phase, Slug string
	Progress                           float64
	FramesDone, FramesTotal            int
	OutputPath                         string
	ErrorCode                          string
	Created                            time.Time
}

func newStorymakerFixture(t *testing.T) *storymakerFixture {
	t.Helper()
	catalog, err := storymaker.LoadCatalog(os.DirFS("."), "img/storymaker/catalog.json")
	if err != nil {
		t.Fatalf("load the catalog: %v", err)
	}
	store := storymaker.NewMemStore()
	return &storymakerFixture{
		t: t, store: store, projects: storymaker.NewProjectStore(store, "Documents/Storymaker"), catalog: catalog, limits: storymaker.DefaultLimits(),
		ffmpegReady: true, browserReady: true, bodies: map[string]string{}, jobs: map[string]*storymakerFixtureJob{}, watchers: map[string][]chan []byte{},
	}
}

func (f *storymakerFixture) set(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn()
}

func (f *storymakerFixture) joined() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.requests, "\n")
}

func (f *storymakerFixture) body(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bodies[key]
}

func (f *storymakerFixture) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (f *storymakerFixture) fail(w http.ResponseWriter, status int, code, message string, issues []storymaker.Issue) {
	body := map[string]any{"error": code, "code": code, "message": message}
	if len(issues) > 0 {
		body["issues"] = issues
	}
	f.writeJSON(w, status, body)
}

func fixtureProjectID(route string) (id, rest string) {
	trimmed := strings.TrimPrefix(route, "/projects/")
	id, rest, _ = strings.Cut(trimmed, "/")
	return id, rest
}

func (f *storymakerFixture) jobBody(job *storymakerFixtureJob) map[string]any {
	body := map[string]any{
		"id": job.ID, "project_id": job.ProjectID, "kind": "render", "status": job.Status, "progress": job.Progress, "phase": job.Phase,
		"frames_done": job.FramesDone, "frames_total": job.FramesTotal, "created_at": job.Created.UTC().Format(time.RFC3339), "updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	if job.OutputPath != "" {
		body["output"] = map[string]any{"path": job.OutputPath, "size": 1234567}
	}
	if job.ErrorCode != "" {
		body["error"] = map[string]any{"code": job.ErrorCode, "message": "fixture failure"}
	}
	return body
}

// publish sends the job to every open event stream of its project. The caller holds f.mu.
func (f *storymakerFixture) publish(job *storymakerFixtureJob) {
	payload, _ := json.Marshal(map[string]any{"job": f.jobBody(job)})
	for _, ch := range f.watchers[job.ProjectID] {
		select {
		case ch <- payload:
		default:
		}
	}
}

// advance moves the newest active job one step: queued, running (compiling), rendering, encoding, succeeded.
func (f *storymakerFixture) advance() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.steps++
	for i := len(f.order) - 1; i >= 0; i-- {
		job := f.jobs[f.order[i]]
		if job.Status != "queued" && job.Status != "running" {
			continue
		}
		switch {
		case job.Status == "queued":
			job.Status, job.Phase, job.Progress = "running", "compiling", 0.1
		case job.Phase == "compiling":
			job.Phase, job.Progress, job.FramesTotal, job.FramesDone = "rendering", 0.5, 354, 177
		case job.Phase == "rendering":
			job.Phase, job.Progress, job.FramesDone = "encoding", 0.9, 354
		default:
			job.Status, job.Phase, job.Progress = "succeeded", "writing", 1
			job.OutputPath = fmt.Sprintf("Documents/Storymaker/%s/exports/%s-%s.mp4", job.ProjectID, job.Slug, job.ID[:8])
		}
		f.publish(job)
		return
	}
}

func (f *storymakerFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route := strings.TrimPrefix(r.URL.Path, "/api/desktop/storymaker")
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	f.mu.Lock()
	f.requests = append(f.requests, r.Method+" "+route)
	f.bodies[r.Method+" "+route] = string(raw)
	disabled, readOnly, unauthorized := f.disabled, f.readOnly, f.unauthorized
	f.mu.Unlock()

	if route == "/player" {
		f.player(w, r)
		return
	}
	if unauthorized {
		f.fail(w, http.StatusUnauthorized, "unauthorized", "Sign in.", nil)
		return
	}
	if disabled {
		f.fail(w, http.StatusForbidden, "storymaker_disabled", "Storymaker is switched off.", nil)
		return
	}
	if r.Method != http.MethodGet && readOnly && !strings.HasSuffix(route, "/cancel") {
		f.fail(w, http.StatusForbidden, "desktop_readonly", "The desktop is read-only.", nil)
		return
	}
	ctx := r.Context()
	switch {
	case route == "/status" && r.Method == http.MethodGet:
		f.status(w)
	case route == "/projects" && r.Method == http.MethodGet:
		list, err := f.projects.List(ctx)
		if err != nil {
			f.fail(w, http.StatusInternalServerError, "internal", err.Error(), nil)
			return
		}
		if list == nil {
			list = []storymaker.Project{}
		}
		f.writeJSON(w, http.StatusOK, map[string]any{"projects": list})
	case route == "/projects" && r.Method == http.MethodPost:
		f.create(w, ctx, raw)
	case strings.HasPrefix(route, "/projects/"):
		f.project(w, r, route, raw)
	case strings.HasPrefix(route, "/jobs/"):
		f.job(w, r, route)
	default:
		f.fail(w, http.StatusNotFound, "not_found", "No such route.", nil)
	}
}

func (f *storymakerFixture) status(w http.ResponseWriter) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ffmpeg := map[string]any{"ready": f.ffmpegReady}
	if f.ffmpegReady {
		ffmpeg["version"] = "7.1"
	} else {
		ffmpeg["issue"] = "ffmpeg: executable file not found in $PATH"
	}
	browser := map[string]any{"ready": f.browserReady, "source": "system"}
	if !f.browserReady {
		browser["source"] = "missing"
		browser["issue"] = "no Chrome or Chromium found"
	}
	f.writeJSON(w, http.StatusOK, map[string]any{
		"enabled": true, "read_only": f.readOnly, "ffmpeg": ffmpeg, "browser": browser, "transport": "png", "catalog_version": f.catalog.Version(),
		"limits": map[string]any{"max_duration_seconds": f.limits.MaxDurationSeconds, "max_scenes": f.limits.MaxScenes, "max_narration_chars": f.limits.MaxNarrationChars},
	})
}

func (f *storymakerFixture) create(w http.ResponseWriter, ctx context.Context, raw []byte) {
	var in struct {
		Title  string `json:"title"`
		Aspect string `json:"aspect"`
		FPS    int    `json:"fps"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		f.fail(w, http.StatusBadRequest, "bad_request", err.Error(), nil)
		return
	}
	project, story, etag, err := f.projects.Create(ctx, in.Title, in.Aspect, in.FPS)
	if err != nil {
		f.fail(w, http.StatusBadRequest, "bad_request", err.Error(), nil)
		return
	}
	text, _ := storymaker.CanonicalJSON(story)
	f.writeJSON(w, http.StatusCreated, map[string]any{"project": project, "story": json.RawMessage(text), "etag": etag})
}

func (f *storymakerFixture) issuesFor(ctx context.Context, id string, data []byte) ([]storymaker.Issue, []storymaker.CodeSceneStatus) {
	story, _, err := storymaker.DecodeStory(data, f.limits)
	if err != nil {
		var decode *storymaker.DecodeError
		if errors.As(err, &decode) {
			return decode.Issues, nil
		}
		return nil, nil
	}
	storymaker.ApplyDefaults(&story)
	issues := storymaker.Validate(&story, f.catalog, f.limits)
	status, _ := f.projects.CodeSceneStatus(ctx, id)
	issues = append(issues, storymaker.CodeSceneIssues(&story, status)...)
	if status == nil {
		status = []storymaker.CodeSceneStatus{}
	}
	return storymaker.CapIssues(issues, storymaker.MaxHTTPIssues), status
}

func (f *storymakerFixture) project(w http.ResponseWriter, r *http.Request, route string, raw []byte) {
	ctx := r.Context()
	id, rest := fixtureProjectID(route)
	if !storymaker.ValidProjectID(id) {
		f.fail(w, http.StatusBadRequest, "bad_request", "invalid project id", nil)
		return
	}
	switch {
	case rest == "" && r.Method == http.MethodGet:
		project, err := f.projects.Get(ctx, id)
		if err != nil {
			f.notFound(w, err)
			return
		}
		data, etag, err := f.projects.LoadStory(ctx, id)
		if err != nil {
			f.notFound(w, err)
			return
		}
		issues, status := f.issuesFor(ctx, id, data)
		if issues == nil {
			issues = []storymaker.Issue{}
		}
		f.writeJSON(w, http.StatusOK, map[string]any{"project": project, "story": json.RawMessage(data), "etag": etag, "issues": issues, "code_scenes": status})
	case rest == "" && r.Method == http.MethodDelete:
		if err := f.projects.Delete(ctx, id); err != nil {
			f.notFound(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case rest == "story" && r.Method == http.MethodPut:
		f.saveStory(w, r, id, raw)
	case strings.HasPrefix(rest, "scenes/"):
		f.codeScene(w, r, id, strings.TrimPrefix(rest, "scenes/"), raw)
	case rest == "render" && r.Method == http.MethodPost:
		f.render(w, r, id)
	case rest == "events" && r.Method == http.MethodGet:
		f.events(w, r, id)
	default:
		f.fail(w, http.StatusNotFound, "not_found", "No such route.", nil)
	}
}

func (f *storymakerFixture) notFound(w http.ResponseWriter, err error) {
	if errors.Is(err, fs.ErrNotExist) {
		f.fail(w, http.StatusNotFound, "not_found", "No such project.", nil)
		return
	}
	f.fail(w, http.StatusInternalServerError, "internal", err.Error(), nil)
}

func (f *storymakerFixture) saveStory(w http.ResponseWriter, r *http.Request, id string, raw []byte) {
	ifMatch := r.Header.Get("If-Match")
	if ifMatch == "" {
		f.fail(w, http.StatusPreconditionRequired, "precondition_required", "If-Match is required.", nil)
		return
	}
	story, _, err := storymaker.DecodeStory(raw, f.limits)
	if err != nil {
		var decode *storymaker.DecodeError
		var issues []storymaker.Issue
		if errors.As(err, &decode) {
			issues = decode.Issues
		}
		f.fail(w, http.StatusUnprocessableEntity, "invalid_story", err.Error(), issues)
		return
	}
	applied := storymaker.ApplyDefaults(&story)
	issues := storymaker.Validate(&story, f.catalog, f.limits)
	text, err := storymaker.CanonicalJSON(story)
	if err != nil {
		f.fail(w, http.StatusInternalServerError, "internal", err.Error(), nil)
		return
	}
	etag, err := f.projects.SaveStory(r.Context(), id, text, ifMatch)
	if errors.Is(err, storymaker.ErrConflict) {
		f.fail(w, http.StatusPreconditionFailed, "precondition_failed", "The story changed.", nil)
		return
	}
	if err != nil {
		f.notFound(w, err)
		return
	}
	status, _ := f.projects.CodeSceneStatus(r.Context(), id)
	issues = append(issues, storymaker.CodeSceneIssues(&story, status)...)
	if issues == nil {
		issues = []storymaker.Issue{}
	}
	f.writeJSON(w, http.StatusOK, map[string]any{"etag": etag, "issues": storymaker.CapIssues(issues, storymaker.MaxHTTPIssues), "defaults_applied": applied, "story": json.RawMessage(text)})
}

func (f *storymakerFixture) codeScene(w http.ResponseWriter, r *http.Request, id, rest string, raw []byte) {
	ctx := r.Context()
	scene, kind, _ := strings.Cut(rest, "/")
	switch {
	case kind == "code" && r.Method == http.MethodGet:
		source, etag, err := f.projects.LoadCodeSource(ctx, id, scene)
		if err != nil {
			f.notFound(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("ETag", etag)
		_, _ = w.Write(source)
	case kind == "bundle" && r.Method == http.MethodGet:
		bundle, err := f.projects.LoadCodeBundle(ctx, id, scene)
		if err != nil {
			f.notFound(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write(bundle)
	case kind == "code" && r.Method == http.MethodPut:
		ifMatch := r.Header.Get("If-Match")
		if ifMatch == "" && r.Header.Get("If-None-Match") == "*" {
			ifMatch = storymaker.MustNotExist
		}
		if ifMatch == "" {
			f.fail(w, http.StatusPreconditionRequired, "precondition_required", "If-Match or If-None-Match is required.", nil)
			return
		}
		etag, result, err := f.projects.SaveCodeScene(ctx, id, scene, raw, ifMatch)
		if errors.Is(err, storymaker.ErrConflict) {
			f.fail(w, http.StatusPreconditionFailed, "precondition_failed", "The source changed.", nil)
			return
		}
		if err != nil {
			f.fail(w, http.StatusBadRequest, "bad_request", err.Error(), nil)
			return
		}
		errs := result.Errors
		if errs == nil {
			errs = []storymaker.BundleError{}
		}
		f.writeJSON(w, http.StatusOK, map[string]any{"etag": etag, "ok": len(errs) == 0, "errors": errs})
	default:
		f.fail(w, http.StatusNotFound, "not_found", "No such route.", nil)
	}
}

func (f *storymakerFixture) render(w http.ResponseWriter, r *http.Request, id string) {
	f.mu.Lock()
	ffmpeg, browser := f.ffmpegReady, f.browserReady
	f.mu.Unlock()
	if r.Header.Get("If-Match") == "" {
		f.fail(w, http.StatusPreconditionRequired, "precondition_required", "If-Match is required.", nil)
		return
	}
	if !ffmpeg {
		f.fail(w, http.StatusServiceUnavailable, "ffmpeg_unavailable", "FFmpeg is not available.", nil)
		return
	}
	if !browser {
		f.fail(w, http.StatusServiceUnavailable, "browser_unavailable", "No browser.", nil)
		return
	}
	project, err := f.projects.Get(r.Context(), id)
	if err != nil {
		f.notFound(w, err)
		return
	}
	job := &storymakerFixtureJob{ID: uuid.NewString(), ProjectID: id, Status: "queued", Phase: "starting", Slug: storymaker.Slugify(project.Title, id), Created: time.Now()}
	f.mu.Lock()
	f.jobs[job.ID] = job
	f.order = append(f.order, job.ID)
	body := f.jobBody(job)
	f.publish(job)
	f.mu.Unlock()
	f.writeJSON(w, http.StatusAccepted, map[string]any{"job": body})
}

func (f *storymakerFixture) job(w http.ResponseWriter, r *http.Request, route string) {
	trimmed := strings.TrimPrefix(route, "/jobs/")
	id, rest, _ := strings.Cut(trimmed, "/")
	f.mu.Lock()
	defer f.mu.Unlock()
	job := f.jobs[id]
	if job == nil {
		f.fail(w, http.StatusNotFound, "not_found", "No such job.", nil)
		return
	}
	if rest == "cancel" && r.Method == http.MethodPost {
		if job.Status == "queued" || job.Status == "running" {
			job.Status, job.ErrorCode = "cancelled", "cancelled"
			f.publish(job)
		}
	}
	f.writeJSON(w, http.StatusOK, map[string]any{"job": f.jobBody(job)})
}

func (f *storymakerFixture) events(w http.ResponseWriter, r *http.Request, id string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		f.fail(w, http.StatusInternalServerError, "internal", "no streaming", nil)
		return
	}
	ch := make(chan []byte, 16)
	f.mu.Lock()
	f.watchers[id] = append(f.watchers[id], ch)
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		list := f.watchers[id]
		for i, candidate := range list {
			if candidate == ch {
				f.watchers[id] = append(list[:i], list[i+1:]...)
				break
			}
		}
		f.mu.Unlock()
	}()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, ": open\n\n")
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case payload := <-ch:
			fmt.Fprintf(w, "event: job\ndata: %s\n\n", payload)
			flusher.Flush()
		}
	}
}

// player serves the app player host exactly like the real handler: the markup of contract section 6 under the CSP of the spec.
func (f *storymakerFixture) player(w http.ResponseWriter, r *http.Request) {
	origin := "http://" + r.Host
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", fmt.Sprintf(storymakerPlayerCSP, origin))
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, `<!doctype html><html data-sm-mode="app" data-sm-base="%[1]s"><head><meta charset="utf-8"><link rel="stylesheet" href="%[1]s/css/storymaker-player.css"></head><body><canvas id="sm-stage"></canvas><script src="%[1]s/js/storymaker/host.bundle.js"></script></body></html>`, origin)
}

// storedStory returns the story of a project as the store holds it.
func (f *storymakerFixture) storedStory(id string) (map[string]any, string) {
	data, etag, err := f.projects.LoadStory(context.Background(), id)
	if err != nil {
		f.t.Fatalf("load the stored story: %v", err)
	}
	var story map[string]any
	if err := json.Unmarshal(data, &story); err != nil {
		f.t.Fatalf("decode the stored story: %v", err)
	}
	return story, etag
}

// watcherCount is the number of open event streams of a project.
func (f *storymakerFixture) watcherCount(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.watchers[id])
}

func (f *storymakerFixture) projectIDs() []string {
	list, _ := f.projects.List(context.Background())
	ids := make([]string, 0, len(list))
	for _, project := range list {
		ids = append(ids, project.ID)
	}
	sort.Strings(ids)
	return ids
}
```

`ui/desktop_storymaker_browser_test.go`:

```go
package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
)

// The Storymaker app browser tour (spec "Testing": one browser smoke tour of the app). It runs the real desktop shell with the
// real app modules, the real player iframe (host and player bundles under the exact player CSP) and the real catalog, against
// a handler for /api/desktop/storymaker. The default handler is storymakerFixture; the integration step passes B's real
// handlers instead. Gated by AURAGO_RUN_BROWSER_SMOKE=1:
//
//	AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestStorymakerApp -count=1 -timeout 10m

func storymakerWords(t *testing.T, lang string) map[string]string {
	t.Helper()
	words := map[string]string{}
	for _, dir := range []string{"storymaker", "storymaker_catalog"} {
		data, err := os.ReadFile(filepath.Join("lang", dir, lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		part := map[string]string{}
		if err := json.Unmarshal(data, &part); err != nil {
			t.Fatal(err)
		}
		for key, value := range part {
			words[key] = value
		}
	}
	return words
}

// storymakerBrowserPage opens the desktop with the Storymaker app on the given API handler and returns the page.
func storymakerBrowserPage(t *testing.T, api http.Handler) *rod.Page {
	t.Helper()
	// The desktop harness gives its page 30 seconds; a tour of the whole app needs more.
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/storymaker/": api}).CancelTimeout().Timeout(3 * time.Minute)
	page.MustEval(`(words)=>Object.assign(fixtureWords, words)`, storymakerWords(t, "en"))
	page.MustEval(`()=>auditTest.openApp('storymaker')`)
	return page
}

func storymakerWait(t *testing.T, page *rod.Page, expr string) {
	t.Helper()
	if err := rod.Try(func() { page.MustWait(expr) }); err != nil {
		state := page.MustEval(`()=>JSON.stringify({errors: window.fixtureErrors, app: (document.querySelector('[data-stm="app"]')||{}).innerText||''}).slice(0, 1500)`).String()
		t.Fatalf("timed out waiting for %s: %v\n%s", expr, err, state)
	}
}

func storymakerText(page *rod.Page, selector string) string {
	return page.MustEval(`(s)=>{const e=document.querySelector(s);return e?e.textContent:'';}`, selector).String()
}

func storymakerClick(t *testing.T, page *rod.Page, selector string) {
	t.Helper()
	storymakerWait(t, page, `()=>!!document.querySelector(`+jsString(selector)+`)`)
	page.MustEval(`(s)=>document.querySelector(s).click()`, selector)
}

func jsString(s string) string {
	data, _ := json.Marshal(s)
	return string(data)
}

// storymakerEditorReplace swaps text in the open CodeMirror editor through the public EditorView API of the vendored bundle.
func storymakerEditorReplace(page *rod.Page, find, insert string) {
	page.MustEval(`async (find, insert) => {
		const cm = await import('/js/vendor/codemirror-bundle.esm.js');
		const view = cm.EditorView.findFromDOM(document.querySelector('[data-stm="code-editor"] .cm-editor'));
		const text = view.state.doc.toString(), at = text.indexOf(find);
		if (at < 0) throw new Error('not in the editor: ' + find);
		view.dispatch({changes: {from: at, to: at + find.length, insert}});
	}`, find, insert)
}

// storymakerShot writes a screenshot when AURAGO_BROWSER_ARTIFACT_DIR is set (a visual check by hand, not an assertion).
func storymakerShot(t *testing.T, page *rod.Page, name string) {
	t.Helper()
	dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".png"), page.MustScreenshot(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func storymakerNoErrors(t *testing.T, page *rod.Page) {
	t.Helper()
	if got := page.MustEval(`()=>JSON.stringify(window.fixtureErrors)`).String(); got != "[]" {
		t.Fatalf("script errors in the page: %s", got)
	}
}

// storymakerBackend is what the tour needs from the thing behind /api/desktop/storymaker: the handler, a view of the stored projects
// and a way to move the render job along. storymakerFixture implements it; the integration step implements it over the real handlers
// (the stored story through the project API, advance as a wait for the real job) and runs storymakerTour against them.
type storymakerBackend interface {
	http.Handler
	projectIDs() []string
	storedStory(id string) (map[string]any, string)
	advance()
	watcherCount(id string) int
	joined() string
	body(key string) string
}

func TestStorymakerAppBrowser(t *testing.T) {
	storymakerTour(t, newStorymakerFixture(t))
}

// storymakerTour opens the app, creates a project, plays it, edits narration, fills a scene from the library, works in the code tab
// (an invalid value, a code scene with a syntax error), exports with a stepped job, leaves, deletes and closes the window.
func storymakerTour(t *testing.T, fixture storymakerBackend) {
	t.Helper()
	page := storymakerBrowserPage(t, fixture)

	// Start screen: no projects yet, nothing is missing, the capability banner stays away.
	storymakerWait(t, page, `()=>!!document.querySelector('.stm-projects') && !!document.querySelector('[data-stm="projects-empty"]') && !document.querySelector('[data-stm="projects-empty"]').hidden`)
	storymakerShot(t, page, "01-start")
	if page.MustEval(`()=>!!document.querySelector('[data-banner="export_unavailable"]')`).Bool() {
		t.Fatal("the export-unavailable banner shows although FFmpeg and a browser are ready")
	}

	// Create a project: the editor opens on one scene, the player compiles it and the strip shows a drawn thumbnail.
	page.MustElement(`[data-stm="new-title"]`).MustInput("Smoke test")
	page.MustElement(`[data-stm="aspect"][data-aspect="16:9"]`).MustClick()
	page.MustElement(`[data-stm="create-project"]`).MustClick()
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="project-title"]')?.textContent==='Smoke test'`)
	storymakerWait(t, page, `()=>document.querySelectorAll('[data-stm="scene"]').length===1 && document.querySelector('[data-stm="clock-total"]').textContent!=='0:00.0'`)
	ids := fixture.projectIDs()
	if len(ids) != 1 {
		t.Fatalf("projects after create = %v", ids)
	}
	id := ids[0]
	frame := page.MustElement(`.stm-player`)
	if got := *frame.MustAttribute("sandbox"); got != "allow-scripts" {
		t.Fatalf("player sandbox = %q, want exactly allow-scripts", got)
	}
	if page.MustEval(`()=>!!document.querySelector('.stm-player').hasAttribute('allow')`).Bool() {
		t.Fatal("the player iframe must not delegate any permission")
	}
	storymakerWait(t, page, `()=>document.querySelector('canvas[data-scene="s1"]')?.dataset.drawn==='true'`)
	if !page.MustEval(`()=>{const c=document.querySelector('canvas[data-scene="s1"]'),d=c.getContext('2d').getImageData(0,0,c.width,c.height).data,seen=new Set();for(let i=0;i<d.length;i+=4*97)seen.add(d[i]+','+d[i+1]+','+d[i+2]);return seen.size>1;}`).Bool() {
		t.Fatal("the thumbnail of the first scene is a single colour: the preview rendered a blank frame")
	}
	storymakerShot(t, page, "02-editor")
	storymakerNoErrors(t, page)

	// A message that does not come from the iframe is dropped, even with a valid shape: the clock keeps its value.
	before := storymakerText(page, `[data-stm="clock-total"]`)
	page.MustEval(`()=>window.postMessage({sm:1,type:'compiled',report:{frame_count:99999,fps:30,scenes:[{id:'s1',start_frame:0,end_frame:99999,kind:'storyboard'}],issues:[]}},'*')`)
	time.Sleep(300 * time.Millisecond)
	if after := storymakerText(page, `[data-stm="clock-total"]`); after != before {
		t.Fatalf("a message from another window changed the timeline: %q -> %q", before, after)
	}

	// Play: the player host runs in real time, the clock moves and the button turns into pause; pause stops it again.
	page.MustEval(`()=>document.querySelector('[data-stm="play"]').click()`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="play"]').dataset.playing==='true'`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="clock"]').textContent!=='0:00.0'`)
	page.MustEval(`()=>document.querySelector('[data-stm="play"]').click()`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="play"]').dataset.playing==='false'`)

	// Narration: typing saves with If-Match, the server keeps the text and the compiled length follows the text.
	page.MustElement(`[data-stm="narration"]`).MustInput("Anna works from home. Suddenly somebody knocks at the door and the whole office holds its breath for a moment.")
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	story, _ := fixture.storedStory(id)
	scenes := story["scenes"].([]any)
	if narration, _ := scenes[0].(map[string]any)["narration"].(string); !strings.Contains(narration, "knocks at the door") {
		t.Fatalf("stored narration = %q", narration)
	}
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="clock-total"]').textContent!==`+jsString(before))

	// Library: a person and a prop go into the selected scene; the environment follows the card.
	storymakerClick(t, page, `[data-stm="side-tab"][data-tab="library"]`)
	storymakerClick(t, page, `[data-stm="library-card"][data-id="char.office-woman-1"]`)
	storymakerWait(t, page, `()=>document.querySelectorAll('[data-stm="scene-items"] [data-kind="actor"]').length===1`)
	storymakerClick(t, page, `[data-stm="library-tab"][data-kind="prop"]`)
	storymakerClick(t, page, `[data-stm="library-card"][data-id="prop.laptop"]`)
	storymakerClick(t, page, `[data-stm="library-tab"][data-kind="environment"]`)
	storymakerClick(t, page, `[data-stm="library-card"][data-id="env.office"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="library-card"][data-id="env.office"]')?.dataset.present==='true'`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	story, _ = fixture.storedStory(id)
	scene := story["scenes"].([]any)[0].(map[string]any)
	if scene["environment"] != "env.office" || len(scene["actors"].([]any)) != 1 || len(scene["props"].([]any)) != 1 {
		t.Fatalf("stored scene after the library clicks = %v", scene)
	}
	storymakerShot(t, page, "03-library")
	storymakerClick(t, page, `[data-stm="side-tab"][data-tab="scene"]`)
	storymakerShot(t, page, "04-scene-settings")

	// Code tab: story.json in CodeMirror; an invalid value is saved, reported by the server and marked at its line.
	storymakerClick(t, page, `[data-stm="tab"][data-tab="code"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="code-editor"] .cm-editor') && document.querySelector('[data-stm="code-editor"] .cm-content').textContent.includes('storymaker/1')`)
	storymakerEditorReplace(page, `"variant": "day"`, `"variant": "dusk"`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-state"]')?.dataset.state==='dirty'`)
	storymakerClick(t, page, `[data-stm="code-save"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-problems"]')?.dataset.count!=='0' && !!document.querySelector('[data-stm="code-editor"] .cm-lint-marker')`)
	if !strings.Contains(storymakerText(page, `[data-stm="code-problems"]`), "Invalid value") {
		t.Fatalf("the problem list does not name the issue: %q", storymakerText(page, `[data-stm="code-problems"]`))
	}
	storymakerShot(t, page, "05-code-issue")
	storymakerNoErrors(t, page)
	// Put it right again and save: the server's story comes back into the editor and the problem list is empty.
	storymakerEditorReplace(page, `"variant": "dusk"`, `"variant": "night"`)
	storymakerClick(t, page, `[data-stm="code-save"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-problems"]')?.dataset.count==='0' && document.querySelector('[data-stm="code-state"]')?.dataset.state==='saved'`)
	story, _ = fixture.storedStory(id)
	if story["scenes"].([]any)[0].(map[string]any)["variant"] != "night" {
		t.Fatalf("variant after the code save = %v", story["scenes"].([]any)[0])
	}
	storymakerClick(t, page, `[data-stm="tab"][data-tab="stage"]`)

	// A code scene: the starter program is saved as a new file (If-None-Match), bundled, and a mistake is shown at its line.
	storymakerClick(t, page, `[data-stm="add-code-scene"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-file"]')?.textContent==='code2.js' && document.querySelector('[data-stm="code-editor"] .cm-content')?.textContent.includes('export default function scene')`)
	if !strings.Contains(fixture.body("PUT /projects/"+id+"/scenes/code2/code"), "api.duration(5)") {
		t.Fatalf("the starter code was not written: %q", fixture.body("PUT /projects/"+id+"/scenes/code2/code"))
	}
	storymakerEditorReplace(page, "api.duration(5);", "api.duration(5;")
	storymakerClick(t, page, `[data-stm="code-save"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-problems"]')?.dataset.count!=='0' && document.querySelector('[data-stm="code-file-btn"][data-key="scene:code2"]')?.dataset.failed==='true'`)
	storymakerEditorReplace(page, "api.duration(5;", "api.duration(5);")
	storymakerClick(t, page, `[data-stm="code-save"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="code-problems"]')?.dataset.count==='0' && document.querySelector('[data-stm="code-file-btn"][data-key="scene:code2"]')?.dataset.failed==='false'`)
	storymakerClick(t, page, `[data-stm="tab"][data-tab="stage"]`)
	storymakerWait(t, page, `()=>document.querySelectorAll('[data-stm="scene"]').length===2 && document.querySelector('[data-stm="scene"][data-scene="code2"] .stm-scene-kind')!==null`)

	// Scrubber: Home goes to the start, a click lands in the middle, End goes to the last frame.
	page.MustEval(`()=>document.querySelector('[data-stm="scrubber"]').focus()`)
	if err := page.Keyboard.Type(input.Home); err != nil {
		t.Fatal(err)
	}
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="scrubber"]').getAttribute('aria-valuenow')==='0'`)
	page.MustElement(`[data-stm="scrubber"]`).MustClick()
	storymakerWait(t, page, `()=>Number(document.querySelector('[data-stm="scrubber"]').getAttribute('aria-valuenow'))>0`)
	page.MustEval(`()=>document.querySelector('[data-stm="scrubber"]').focus()`)
	if err := page.Keyboard.Type(input.End); err != nil {
		t.Fatal(err)
	}
	storymakerWait(t, page, `()=>{const s=document.querySelector('[data-stm="scrubber"]');return s.getAttribute('aria-valuenow')===s.getAttribute('aria-valuemax')&&s.getAttribute('aria-valuemax')!=='0'}`)

	// Inspector: the transition and the duration of the selected (code) scene, then the scene moves up and down.
	const setSelect = `(sel,value)=>{const s=document.querySelector(sel);s.value=value;s.dispatchEvent(new Event('change',{bubbles:true}));}`
	page.MustEval(setSelect, `[data-stm="transition"]`, "wipe")
	page.MustEval(setSelect, `[data-stm="duration-mode"]`, "fixed")
	page.MustEval(`()=>{const i=document.querySelector('[data-stm="duration-seconds"]');i.value='3.5';i.dispatchEvent(new Event('change',{bubbles:true}));}`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	story, _ = fixture.storedStory(id)
	second := story["scenes"].([]any)[1].(map[string]any)
	if second["transition"] != "wipe" || second["duration"] != 3.5 {
		t.Fatalf("stored transition and duration = %v, %v", second["transition"], second["duration"])
	}
	storymakerClick(t, page, `[data-stm="scene-up"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="scene"][data-index="0"]')?.dataset.scene==='code2' && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
	story, _ = fixture.storedStory(id)
	if first := story["scenes"].([]any)[0].(map[string]any)["id"]; first != "code2" {
		t.Fatalf("the first stored scene after the move = %v", first)
	}
	storymakerClick(t, page, `[data-stm="scene-down"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="scene"][data-index="1"]')?.dataset.scene==='code2' && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)

	// Export: the panel steps through the fake job and ends in the output path with "show in Files".
	storymakerClick(t, page, `[data-stm="export"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-start"]') && !document.querySelector('[data-stm="export-start"]').disabled`)
	if !page.MustEval(`()=>document.querySelectorAll('[data-stm="export-caps"] [data-ok="true"]').length===2`).Bool() {
		t.Fatal("FFmpeg and the browser should both be listed as ready")
	}
	storymakerClick(t, page, `[data-stm="export-start"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-progress"]')`)
	if fixture.watcherCount(id) == 0 {
		t.Fatal("the app should follow the job on the project's event stream")
	}
	if !strings.Contains(fixture.joined(), "POST /projects/"+id+"/render") {
		t.Fatalf("no render request: %s", fixture.joined())
	}
	fixture.advance()
	fixture.advance()
	storymakerShot(t, page, "06-export-progress")
	storymakerWait(t, page, `()=>document.querySelector('.stm-export-label')?.textContent.includes('50') && document.querySelector('progress.stm-progress')?.value===0.5`)
	fixture.advance()
	fixture.advance()
	storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-path"]')`)
	if path := storymakerText(page, `[data-stm="export-path"]`); !strings.HasPrefix(path, "Documents/Storymaker/"+id+"/exports/Smoke-test-") || !strings.HasSuffix(path, ".mp4") {
		t.Fatalf("export path = %q", path)
	}
	storymakerShot(t, page, "07-export-done")
	windows := page.MustEval(`()=>auditTest.state.windows.size`).Int()
	page.MustEval(`()=>document.querySelector('[data-stm="export-show"]').click()`)
	storymakerWait(t, page, fmt.Sprintf(`()=>auditTest.state.windows.size===%d`, windows+1))
	storymakerNoErrors(t, page)

	// Leave the project: the player iframe goes with it, the list shows the project and reopening restores the story.
	storymakerClick(t, page, `[data-stm="export-close"]`)
	storymakerClick(t, page, `[data-stm="back"]`)
	storymakerWait(t, page, `()=>document.querySelectorAll('[data-stm="open-project"]').length===1 && !document.querySelector('.stm-player')`)
	storymakerClick(t, page, `[data-stm="open-project"]`)
	storymakerWait(t, page, `()=>document.querySelector('[data-stm="narration"]')?.value.includes('knocks at the door') && document.querySelectorAll('[data-stm="scene"]').length===2`)

	// Delete asks the desktop dialog first: refusing keeps the project, confirming removes the folder.
	storymakerClick(t, page, `[data-stm="back"]`)
	storymakerClick(t, page, `[data-stm="delete-project"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('.vd-modal [data-cancel]')`)
	page.MustEval(`()=>document.querySelector('.vd-modal [data-cancel]').click()`)
	storymakerWait(t, page, `()=>!document.querySelector('.vd-modal')`)
	if len(fixture.projectIDs()) != 1 {
		t.Fatal("cancelling the dialog must keep the project")
	}
	storymakerClick(t, page, `[data-stm="delete-project"]`)
	storymakerWait(t, page, `()=>!!document.querySelector('.vd-modal [type="submit"]')`)
	page.MustEval(`()=>document.querySelector('.vd-modal [type="submit"]').click()`)
	storymakerWait(t, page, `()=>document.querySelectorAll('[data-stm="open-project"]').length===0 && !document.querySelector('[data-stm="projects-empty"]').hidden`)
	if len(fixture.projectIDs()) != 0 {
		t.Fatalf("projects after the delete = %v", fixture.projectIDs())
	}

	// Closing the window tears the app down without a script error.
	page.MustEval(`()=>auditTest.closeWindow([...auditTest.state.windows.values()].find(w=>w.appId==='storymaker').id)`)
	storymakerWait(t, page, `()=>!document.querySelector('[data-stm="app"]')`)
	storymakerNoErrors(t, page)
}

func TestStorymakerAppStatesBrowser(t *testing.T) {
	t.Run("no ffmpeg and no browser", func(t *testing.T) {
		fixture := newStorymakerFixture(t)
		fixture.set(func() { fixture.ffmpegReady, fixture.browserReady = false, false })
		page := storymakerBrowserPage(t, fixture)
		storymakerWait(t, page, `()=>!!document.querySelector('[data-banner="export_unavailable"]')`)
		page.MustElement(`[data-stm="new-title"]`).MustInput("Offline")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export"]')`)
		storymakerClick(t, page, `[data-stm="export"]`)
		storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="export-blockers"]')`)
		text := storymakerText(page, `[data-stm="export-blockers"]`)
		if !strings.Contains(text, "FFmpeg is missing") || !strings.Contains(text, "No browser is available") {
			t.Fatalf("blockers = %q", text)
		}
		if !page.MustEval(`()=>document.querySelector('[data-stm="export-start"]').disabled`).Bool() {
			t.Fatal("the start button must be disabled")
		}
		if !strings.Contains(storymakerText(page, `[data-stm="export-caps"]`), "ffmpeg: executable file not found") {
			t.Fatal("the capability list should show the reason the status gave")
		}
		// the editor itself still works
		if page.MustEval(`()=>document.querySelector('[data-stm="narration"]').disabled`).Bool() {
			t.Fatal("editing must stay possible without FFmpeg")
		}
		storymakerNoErrors(t, page)
	})

	t.Run("read-only", func(t *testing.T) {
		fixture := newStorymakerFixture(t)
		page := storymakerBrowserPage(t, fixture)
		storymakerWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
		page.MustElement(`[data-stm="new-title"]`).MustInput("Before")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="narration"]')`)
		fixture.set(func() { fixture.readOnly = true })
		// the desktop policy flips while the window is open
		page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:true}}))`)
		storymakerWait(t, page, `()=>!!document.querySelector('[data-banner="readonly"]') && document.querySelector('[data-stm="narration"]').disabled && document.querySelector('[data-stm="add-scene"]').disabled`)
		storymakerClick(t, page, `[data-stm="export"]`)
		storymakerWait(t, page, `()=>document.querySelector('[data-stm="export-blockers"] [data-reason="readonly"]')!==null && document.querySelector('[data-stm="export-start"]').disabled`)
		storymakerNoErrors(t, page)
	})

	t.Run("switched off", func(t *testing.T) {
		fixture := newStorymakerFixture(t)
		fixture.set(func() { fixture.disabled = true })
		page := storymakerBrowserPage(t, fixture)
		storymakerWait(t, page, `()=>document.querySelector('[data-stm="state"]')?.dataset.state==='storymaker.state.disabled_title'`)
		if !strings.Contains(storymakerText(page, `[data-stm="state"]`), "Storymaker is switched off") {
			t.Fatalf("state card = %q", storymakerText(page, `[data-stm="state"]`))
		}
		storymakerNoErrors(t, page)
	})

	t.Run("signed out", func(t *testing.T) {
		fixture := newStorymakerFixture(t)
		fixture.set(func() { fixture.unauthorized = true })
		page := storymakerBrowserPage(t, fixture)
		storymakerWait(t, page, `()=>document.querySelector('[data-stm="state"]')?.dataset.state==='storymaker.state.unauthorized_title'`)
		storymakerNoErrors(t, page)
	})

	t.Run("conflict", func(t *testing.T) {
		fixture := newStorymakerFixture(t)
		page := storymakerBrowserPage(t, fixture)
		storymakerWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
		page.MustElement(`[data-stm="new-title"]`).MustInput("Conflict")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="narration"]')`)
		id := fixture.projectIDs()[0]
		// somebody else saves the story first
		story, etag := fixture.storedStory(id)
		story["title"] = "Changed elsewhere"
		data, _ := json.Marshal(story)
		if _, err := fixture.projects.SaveStory(t.Context(), id, data, etag); err != nil {
			t.Fatal(err)
		}
		page.MustElement(`[data-stm="narration"]`).MustInput("My text")
		storymakerWait(t, page, `()=>!!document.querySelector('[data-banner="conflict"]')`)
		storymakerClick(t, page, `[data-stm="conflict-overwrite"]`)
		storymakerWait(t, page, `()=>!document.querySelector('[data-banner="conflict"]') && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
		stored, _ := fixture.storedStory(id)
		if narration, _ := stored["scenes"].([]any)[0].(map[string]any)["narration"].(string); narration != "My text" {
			t.Fatalf("stored narration after overwrite = %q", narration)
		}
		storymakerNoErrors(t, page)
	})
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./ui -run 'TestStorymakerLoaderListsEveryScript' -count=1`
Expected: FAIL: FAIL (`the loader lists storymaker.js, which does not exist`; the tour itself would wait for `window.StorymakerApp`)

Recorded at drafting time (the replay of this task on a tree without it):

```text
--- FAIL: TestStorymakerLoaderListsEveryScript (0.00s)
    desktop_storymaker_guards_test.go:221: the loader lists storymaker.js, which does not exist
FAIL
FAIL	aurago/ui	0.248s
FAIL
```

- [ ] **Step 3: Implement**

`ui/js/desktop/apps/storymaker.js`:

```javascript
// Storymaker window: the composition root. It owns the per-window instance (the state the other modules read and the bus they
// listen to), the start screen and the editor layout (toolbar, scene strip, stage with the player iframe and the scrubber,
// inspector and library, code tab, issues panel, banners), the save pipeline (edits are debounced, saved with If-Match and
// answered with the server's story; a conflict offers reload or overwrite), the player loading and thumbnails, the project
// event stream and the window disposal. Loaded last; publishes window.StorymakerApp = {render, dispose}. Classic script.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const instances = new Map();
    const SAVE_DELAY_MS = 700;
    const PREVIEW_DELAY_MS = 250;
    const THUMB_WIDTH = 192;
    const LIVE_CLEAR_MS = 5000;

    function createInstance(host, windowId, ctx) {
        const c = ctx || {};
        const inst = {
            windowId, host, ctx: c,
            t: (key, vars) => (typeof c.t === 'function' ? c.t(key, vars) : key),
            bus: S.createBus(),
            bag: S.bag(),
            api: S.createApi(),
            status: null,
            ctxReadonly: !!c.readonly,
            forcedReadonly: false,
            projectId: '', project: null, story: null, etag: '', issues: [], draftIssues: [], codeStatus: [], playerErrors: [],
            report: null, frame: 0, playing: false, selected: 0, tab: 'stage', sideTab: 'scene', saveState: 'saved',
            catalog: null, thumbs: new Map(), dirty: false, disposed: false, loadSeq: 0, thumbSeq: 0, conflict: false,
            wantFrame: null, seekBusy: false, seekSent: -1, seekTimer: 0,
            view: 'loading', parts: {}, root: null, content: null, liveTimer: 0
        };
        inst.assets = S.createAssets(inst.api);
        Object.defineProperty(inst, 'readonly', { get() { return inst.ctxReadonly || inst.forcedReadonly || !!(inst.status && inst.status.read_only); } });
        inst.domId = suffix => 'stm-' + String(windowId).replace(/[^A-Za-z0-9_-]/g, '_') + '-' + suffix;
        return inst;
    }

    // ---- small shared pieces ---------------------------------------------------------------------------------------------------

    function notify(inst, text, kind) {
        if (inst.disposed || !text) return;
        const live = inst.parts.live;
        if (!live) return;
        live.textContent = text;
        live.dataset.kind = kind || 'info';
        clearTimeout(inst.liveTimer);
        inst.liveTimer = setTimeout(() => { live.textContent = ''; }, LIVE_CLEAR_MS);
    }

    function stateCard(inst, titleKey, textKey, actions) {
        const { el, icon } = S;
        const t = inst.t;
        return el('div', { class: 'stm-state', role: 'status', dataset: { stm: 'state', state: titleKey } },
            icon('lock', 'stm-state-icon'),
            el('h2', { text: t(titleKey) }),
            el('p', { text: t(textKey) }),
            actions ? el('div', { class: 'stm-actions' }, actions) : null);
    }

    // The view area swaps (start screen, editor, state cards); the live region next to it stays.
    function showRoot(inst, node) {
        S.clear(inst.content);
        inst.content.appendChild(node);
    }

    function retryButton(inst, action) {
        return S.el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'retry' }, onClick: action }, S.icon('refresh'), S.el('span', { text: inst.t('storymaker.retry') }));
    }

    // Which state card an error ends in for the start: switched off, signed out, not allowed, anything else.
    function startFailure(inst, err) {
        const e = S.errors.normalize(err);
        if (e.code === 'storymaker_disabled') return stateCard(inst, 'storymaker.state.disabled_title', 'storymaker.state.disabled_text');
        if (e.status === 401 || e.code === 'unauthorized') return stateCard(inst, 'storymaker.state.unauthorized_title', 'storymaker.state.unauthorized_text');
        if (e.code === 'desktop_scope_required') return stateCard(inst, 'storymaker.state.scope_title', 'storymaker.state.scope_text');
        const card = stateCard(inst, 'storymaker.state.load_failed_title', 'storymaker.state.load_failed_text', [retryButton(inst, () => start(inst))]);
        if (e.message) card.appendChild(S.el('code', { class: 'stm-cap-issue', text: e.message.slice(0, 300) }));
        return card;
    }

    // ---- start screen ----------------------------------------------------------------------------------------------------------

    async function start(inst) {
        inst.view = 'loading';
        showRoot(inst, S.el('p', { class: 'stm-muted stm-loading', role: 'status', text: inst.t('storymaker.loading') }));
        try {
            inst.status = await inst.api.status();
            if (inst.disposed) return;
            if (inst.status && inst.status.enabled === false) { showRoot(inst, stateCard(inst, 'storymaker.state.disabled_title', 'storymaker.state.disabled_text')); return; }
            inst.bus.emit('status');
            await showProjects(inst);
        } catch (err) {
            if (inst.disposed || (err && err.aborted)) return;
            showRoot(inst, startFailure(inst, err));
        }
    }

    async function showProjects(inst) {
        closeEditor(inst);
        inst.view = 'projects';
        const view = S.createProjects({
            t: inst.t, domId: inst.domId, get readonly() { return inst.readonly; },
            createProject: spec => createProject(inst, spec),
            openProject: id => openProject(inst, id),
            deleteProject: project => deleteProject(inst, project),
            confirm: (title, message) => askConfirm(inst, title, message)
        });
        inst.parts.projects = view;
        const shell = S.el('div', { class: 'stm-start' });
        banners(inst, shell);
        shell.appendChild(view.el);
        showRoot(inst, shell);
        try {
            view.setProjects(await inst.api.listProjects());
        } catch (err) {
            if (inst.disposed || (err && err.aborted)) return;
            view.setProjects([]);
            view.setError(S.errors.describe(inst.t, err));
        }
        view.focus();
    }

    // The banner area of the start screen: read-only and the missing export parts.
    function banners(inst, host) {
        const { el, icon } = S;
        const t = inst.t;
        const area = el('div', { class: 'stm-banners', dataset: { stm: 'banners' } });
        const add = (key, tone, text) => area.appendChild(el('div', { class: 'stm-banner', role: 'status', dataset: { banner: key, tone } }, icon(tone === 'bad' ? 'alert' : 'info'), el('span', { text })));
        if (inst.readonly) add('readonly', 'info', t('storymaker.banner.readonly'));
        const status = inst.status || {};
        const ffmpegOk = !!(status.ffmpeg && status.ffmpeg.ready), browserOk = !!(status.browser && status.browser.ready);
        if (!ffmpegOk || !browserOk) {
            const missing = [!ffmpegOk ? t('storymaker.export.cap_ffmpeg') : '', !browserOk ? t('storymaker.export.cap_browser') : ''].filter(Boolean).join(', ');
            add('export_unavailable', 'info', t('storymaker.banner.export_unavailable', { missing }));
        }
        host.appendChild(area);
        return area;
    }

    async function createProject(inst, spec) {
        const result = await inst.api.createProject(spec);
        if (inst.disposed) return;
        await openProject(inst, result.project.id, result);
    }

    async function deleteProject(inst, project) {
        await inst.api.deleteProject(project.id);
        if (inst.disposed) return;
        notify(inst, inst.t('storymaker.projects.deleted', { title: project.title || project.id }));
        const view = inst.parts.projects;
        if (view) view.setProjects(await inst.api.listProjects());
    }

    async function askConfirm(inst, title, message) {
        if (typeof inst.ctx.confirmDialog !== 'function') return false;
        try { return !!(await inst.ctx.confirmDialog(title, message)); } catch (_) { return false; }
    }

    // ---- editor -----------------------------------------------------------------------------------------------------------------

    function closeEditor(inst) {
        clearTimeout(inst.saveTimer);
        clearTimeout(inst.previewTimer);
        clearTimeout(inst.seekTimer);
        inst.wantFrame = null;
        inst.seekBusy = false;
        inst.loadSeq++;
        inst.thumbSeq++;
        if (inst.events) { inst.events.close(); inst.events = null; }
        Object.keys(inst.parts).forEach(name => {
            const part = inst.parts[name];
            if (part && typeof part.dispose === 'function') part.dispose();
        });
        inst.parts = { live: inst.parts.live };
        inst.thumbs.forEach(bitmap => { if (bitmap && typeof bitmap.close === 'function') bitmap.close(); });
        inst.thumbs.clear();
        if (inst.player) inst.player.dispose();
        inst.player = null;
        inst.projectId = '';
        inst.story = null;
        inst.report = null;
        inst.dirty = false;
        inst.conflict = false;
        inst.playerErrors = [];
        inst.draftIssues = [];
        inst.bus.clear();
    }

    // true when the project is open; a failure is shown on the start screen (or as a state card) and answers false
    async function openProject(inst, id, fresh) {
        try {
            const data = fresh || await inst.api.getProject(id);
            if (inst.disposed) return;
            closeEditor(inst);
            inst.view = 'editor';
            inst.projectId = data.project.id;
            inst.project = data.project;
            inst.story = data.story;
            inst.etag = data.etag || '';
            inst.issues = Array.isArray(data.issues) ? data.issues : [];
            inst.codeStatus = Array.isArray(data.code_scenes) ? data.code_scenes : [];
            inst.selected = 0;
            inst.frame = 0;
            inst.playing = false;
            inst.tab = 'stage';
            inst.saveState = 'saved';
            buildEditor(inst);
            inst.events = inst.api.events(inst.projectId, job => inst.bus.emit('job', job));
            reloadPlayer(inst, 0);
            return true;
        } catch (err) {
            if (inst.disposed || (err && err.aborted)) return false;
            const view = inst.parts.projects;
            if (view) view.setError(S.errors.describe(inst.t, err)); else showRoot(inst, startFailure(inst, err));
            return false;
        }
    }

    function buildEditor(inst) {
        const { el, icon } = S;
        const t = inst.t;
        const bus = inst.bus;
        Object.assign(inst, {
            notify: (text, kind) => notify(inst, text, kind),
            confirm: (title, message) => askConfirm(inst, title, message),
            editScene: (sceneId, fn, options) => editScene(inst, sceneId, fn, options),
            select: (index, options) => select(inst, index, options),
            addScene: kind => addScene(inst, kind),
            moveScene: (from, to) => moveScene(inst, from, to),
            deleteScene: index => deleteScene(inst, index),
            openCode: sceneId => openCode(inst, sceneId),
            addActor: entry => addActor(inst, entry),
            addProp: entry => addProp(inst, entry),
            setEnvironment: id => { const scene = currentScene(inst); if (scene) editScene(inst, scene.id, draft => { draft.environment = id; }, { immediate: true }); },
            actorInScene: entry => actorInScene(inst, entry),
            play: () => { if (inst.player) inst.player.play(); },
            pause: () => { if (inst.player) inst.player.pause(); },
            seek: frame => seek(inst, frame),
            flushEdits: () => flushEdits(inst),
            saveStoryText: text => saveStoryText(inst, text),
            saveCodeScene: (sceneId, text, etag) => saveCodeScene(inst, sceneId, text, etag),
            showIssues: () => toggleIssues(inst, true)
        });

        const aspect = inst.story.format && inst.story.format.aspect ? inst.story.format.aspect : '16:9';
        const stageFrame = el('div', { class: 'stm-frame', dataset: { stm: 'stage-frame' } });
        const stageNote = el('div', { class: 'stm-stage-note', role: 'status', dataset: { stm: 'stage-note' }, hidden: true });
        const stage = el('div', { class: 'stm-stage', style: { '--stm-ar': String(S.model.aspectRatio(aspect)) } }, stageFrame, stageNote);

        inst.player = S.createPlayer({
            container: stageFrame, src: inst.api.playerUrl(), title: t('storymaker.preview'),
            resolve: keys => inst.assets.resolve(keys, inst.story, inst.projectId)
        });
        bindPlayer(inst, stageNote);

        const strip = S.createStrip(inst);
        const transport = S.createTransport(inst);
        const inspector = S.createInspector(inst);
        const library = S.createLibrary(inst);
        const code = S.createCode(inst);
        const exporter = S.createExport(inst);
        code.el.hidden = true;
        inst.parts.strip = strip; inst.parts.transport = transport; inst.parts.inspector = inspector; inst.parts.library = library; inst.parts.code = code; inst.parts.exporter = exporter;

        const backButton = el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'back' }, onClick: () => leaveProject(inst) }, icon('left'), el('span', { text: t('storymaker.back_to_projects') }));
        const title = el('h2', { class: 'stm-title', dataset: { stm: 'project-title' }, text: inst.project.title || inst.projectId });
        const format = el('span', { class: 'stm-badge', dataset: { stm: 'format' }, text: aspect + ' · ' + ((inst.story.format && inst.story.format.fps) || 30) + ' fps' });
        const saveLabel = el('span', { class: 'stm-save', role: 'status', dataset: { stm: 'save-state' } });
        const tabButtons = [['stage', 'storymaker.tab_stage', 'film'], ['code', 'storymaker.tab_code', 'code']].map(([name, key, glyph]) => el('button', {
            type: 'button', class: 'stm-tab', role: 'tab', dataset: { stm: 'tab', tab: name }, ariaSelected: String(name === inst.tab),
            onClick: () => setTab(inst, name)
        }, icon(glyph), el('span', { text: t(key) })));
        const issuesButton = el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-issues-btn', dataset: { stm: 'issues-toggle' }, ariaExpanded: 'false', onClick: () => toggleIssues(inst) }, icon('alert'), el('span', { class: 'stm-issues-count', dataset: { stm: 'issues-count' } }));
        const bar = el('header', { class: 'stm-bar' }, backButton, el('div', { class: 'stm-title-group' }, title, format), saveLabel,
            el('div', { class: 'stm-tabs', role: 'tablist', ariaLabel: t('storymaker.title') }, tabButtons),
            el('span', { class: 'stm-spacer' }, ''), issuesButton, exporter.el);

        const issuesPanel = el('section', { class: 'stm-issues', hidden: true, ariaLabel: t('storymaker.issues.title'), dataset: { stm: 'issues' } });
        const sideTabs = [['scene', 'storymaker.side_scene', 'sliders'], ['library', 'storymaker.side_library', 'box']].map(([name, key, glyph]) => el('button', {
            type: 'button', class: 'stm-tab', role: 'tab', dataset: { stm: 'side-tab', tab: name }, ariaSelected: String(name === inst.sideTab),
            onClick: () => setSideTab(inst, name)
        }, icon(glyph), el('span', { text: t(key) })));
        const side = el('aside', { class: 'stm-side', dataset: { stm: 'side' } }, el('div', { class: 'stm-tabs stm-tabs-small', role: 'tablist', ariaLabel: t('storymaker.inspector.title') }, sideTabs), inspector.el, library.el);
        const center = el('main', { class: 'stm-center' }, el('div', { class: 'stm-stage-wrap', dataset: { stm: 'stage-wrap' } }, stage, transport.el), code.el, issuesPanel);
        const body = el('div', { class: 'stm-body' }, strip.el, center, side);
        const bannerArea = el('div', { class: 'stm-banners', dataset: { stm: 'banners' } });
        const app = el('div', { class: 'stm-editor-app', dataset: { tab: inst.tab, sidetab: inst.sideTab } }, bar, bannerArea, body);
        Object.assign(inst.parts, { app, bannerArea, issuesPanel, issuesButton, saveLabel, tabButtons, sideTabs, stageNote, title, format });
        showRoot(inst, app);
        app.addEventListener('keydown', event => {
            if ((event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === 's' && !app.querySelector('[data-stm="code"]:not([hidden])')) {
                event.preventDefault();
                flushEdits(inst);
            }
        });

        const catalogIndex = inst.assets.index();
        if (catalogIndex) { inst.catalog = catalogIndex; }
        inst.assets.loadCatalog().then(() => { inst.catalog = inst.assets.index(); bus.emit('catalog'); bus.emit('story'); }, err => { if (!inst.disposed) notify(inst, S.errors.describe(t, err), 'error'); });

        bus.on('issues', () => paintIssues(inst));
        bus.on('report', () => paintIssues(inst));
        bus.on('status', () => { paintBanners(inst); });
        bus.on('readonly', () => { paintBanners(inst); paintSave(inst); });
        bus.on('story', () => { paintSave(inst); });
        bus.on('conflict', () => { inst.conflict = true; paintBanners(inst); paintSave(inst); });
        paintBanners(inst);
        paintSave(inst);
        paintIssues(inst);
    }

    // ---- player ---------------------------------------------------------------------------------------------------------------

    function bindPlayer(inst, note) {
        const t = inst.t;
        const showNote = text => { note.textContent = text || ''; note.hidden = !text; };
        inst.player.on('loading', () => { inst.compiling = true; showNote(t('storymaker.stage.compiling')); });
        inst.player.on('compiled', report => {
            inst.report = report;
            inst.compiling = false;
            inst.playerErrors = [];
            inst.thumbsDue = true;
            showNote('');
            if (inst.frame > report.frame_count - 1) inst.frame = Math.max(0, report.frame_count - 1);
            inst.bus.emit('report');
        });
        inst.player.on('time', ({ frame, playing }) => {
            inst.playing = playing;
            // A paused host answers every seek with a time message. The next wanted frame goes out once the last one is answered,
            // and until then the playhead stays where the user put it (an older answer must not move it back).
            if (!playing && inst.seekBusy && frame === inst.seekSent) {
                inst.seekBusy = false;
                clearTimeout(inst.seekTimer);
                pumpSeek(inst);
            }
            if (playing || (!inst.seekBusy && inst.wantFrame === null)) inst.frame = frame;
            // The first time message after a compile is the "ready to play" signal: the assets are in, so scenes can be drawn.
            if (inst.thumbsDue) { inst.thumbsDue = false; requestThumbs(inst); }
            if (playing && inst.report) {
                const index = S.model.sceneAtFrame(inst.report, frame);
                if (index >= 0 && index !== inst.selected && Date.now() - (inst.selectedAt || 0) > 600) select(inst, index, { seek: false });
            }
            inst.bus.emit('time');
        });
        inst.player.on('error', error => {
            inst.playerErrors.push(error);
            if (inst.playerErrors.length > 20) inst.playerErrors.shift();
            if (error.code === 'bad_message') return;
            showNote(error.code === 'code_error' || error.code === 'deadline' ? t('storymaker.stage.code_error') : t('storymaker.stage.player_error'));
            inst.bus.emit('issues');
        });
        inst.player.on('unavailable', () => {
            inst.playerUnavailable = true;
            showNote(t('storymaker.stage.unavailable'));
            paintBanners(inst);
        });
        inst.player.on('ready', () => { inst.playerUnavailable = false; paintBanners(inst); });
    }

    async function reloadPlayer(inst, resume) {
        if (inst.disposed || !inst.player || !inst.story) return;
        const seq = ++inst.loadSeq;
        try {
            const prepared = await inst.assets.prepare(inst.story, inst.projectId);
            if (inst.disposed || seq !== inst.loadSeq) return;
            if (!inst.catalog) { inst.catalog = inst.assets.index(); inst.bus.emit('catalog'); }
            const format = inst.story.format || {};
            const aspect = ['16:9', '9:16', '1:1'].indexOf(format.aspect) >= 0 ? format.aspect : '16:9';
            const fps = [24, 25, 30].indexOf(format.fps) >= 0 ? format.fps : 30;
            inst.player.load({
                story: inst.story, catalog: prepared.catalog, audio_info: {}, assets: prepared.assets, fonts: prepared.fonts,
                code_bundles: prepared.code_bundles, format: { aspect, fps, width: S.model.previewWidth(aspect) }
            }, resume === undefined ? inst.frame : resume);
        } catch (err) {
            if (inst.disposed || seq !== inst.loadSeq || (err && err.aborted)) return;
            const note = inst.parts.stageNote;
            if (note) { note.textContent = S.errors.describe(inst.t, err); note.hidden = false; }
        }
    }

    function schedulePreview(inst) {
        clearTimeout(inst.previewTimer);
        inst.previewTimer = setTimeout(() => reloadPlayer(inst), PREVIEW_DELAY_MS);
    }

    async function requestThumbs(inst) {
        const token = ++inst.thumbSeq;
        const report = inst.report;
        if (!report || !inst.player) return;
        const ids = report.scenes.map(scene => scene.id);
        const scenes = S.model.scenesOf(inst.story);
        const first = scenes[inst.selected] ? scenes[inst.selected].id : '';
        const order = ids.slice().sort((a, b) => (a === first ? -1 : 0) - (b === first ? -1 : 0));
        inst.thumbs.forEach((bitmap, id) => {
            if (ids.indexOf(id) < 0) { if (bitmap && typeof bitmap.close === 'function') bitmap.close(); inst.thumbs.delete(id); }
        });
        for (const id of order) {
            if (inst.disposed || token !== inst.thumbSeq) return;
            try {
                const bitmap = await inst.player.thumb(id, THUMB_WIDTH);
                if (inst.disposed || token !== inst.thumbSeq) { if (bitmap && typeof bitmap.close === 'function') bitmap.close(); return; }
                const old = inst.thumbs.get(id);
                inst.thumbs.set(id, bitmap);
                if (old && old !== bitmap && typeof old.close === 'function') old.close();
                inst.bus.emit('thumb', id);
            } catch (_) {
                if (inst.disposed || token !== inst.thumbSeq) return;
            }
        }
    }

    // requestSeek moves the playhead at once and sends only the newest wanted frame to the player, one seek at a time: the host renders
    // seeks concurrently, so two in flight could be answered in the wrong order and leave the picture on the older frame.
    function requestSeek(inst, frame) {
        if (!inst.player || !inst.report) return -1;
        const target = S.clamp(Math.floor(frame), 0, Math.max(0, inst.report.frame_count - 1));
        inst.frame = target;
        inst.wantFrame = target;
        pumpSeek(inst);
        return target;
    }

    function pumpSeek(inst) {
        if (inst.disposed || !inst.player || inst.seekBusy || inst.wantFrame === null) return;
        const target = inst.wantFrame;
        inst.wantFrame = null;
        inst.player.seek(target);
        if (inst.playing) return; // a playing host takes the new start frame without an answer
        inst.seekBusy = true;
        inst.seekSent = target;
        clearTimeout(inst.seekTimer);
        inst.seekTimer = setTimeout(() => { inst.seekBusy = false; pumpSeek(inst); }, 600);
    }

    function seek(inst, frame) {
        const target = requestSeek(inst, frame);
        if (target < 0) return;
        const index = S.model.sceneAtFrame(inst.report, target);
        if (index >= 0 && index !== inst.selected) select(inst, index, { seek: false });
        inst.bus.emit('time');
    }

    // ---- selection and structure ------------------------------------------------------------------------------------------------

    function currentScene(inst) {
        return S.model.scenesOf(inst.story)[inst.selected] || null;
    }

    function select(inst, index, options) {
        const total = S.model.scenesOf(inst.story).length;
        if (!total) return;
        const next = S.clamp(index, 0, total - 1);
        const changed = next !== inst.selected;
        inst.selected = next;
        inst.selectedAt = Date.now();
        if (changed) inst.bus.emit('selection');
        if (options && options.seek && inst.report) {
            requestSeek(inst, S.model.frameForScene(inst.report, next));
            inst.bus.emit('time');
        }
    }

    function actorInScene(inst, entry) {
        const scene = currentScene(inst);
        if (!scene || !Array.isArray(scene.actors)) return false;
        const member = (inst.story.cast || []).find(candidate => candidate && candidate.character === entry.id);
        return !!member && scene.actors.some(actor => actor && actor.cast === member.id);
    }

    function addActor(inst, entry) {
        const scene = currentScene(inst);
        if (!scene || inst.readonly) return;
        if (actorInScene(inst, entry)) { notify(inst, inst.t('storymaker.library.already_in_scene')); return; }
        const draft = S.model.cloneStory(inst.story);
        const result = S.model.insertActor(draft, inst.selected, entry, S.catalogName(inst.t, entry));
        if (!result.ok) { notify(inst, inst.t('storymaker.library.already_in_scene')); return; }
        commit(inst, draft, true);
        notify(inst, inst.t('storymaker.library.added_actor', { name: S.catalogName(inst.t, entry) }));
    }

    function addProp(inst, entry) {
        const scene = currentScene(inst);
        if (!scene || inst.readonly) return;
        const draft = S.model.cloneStory(inst.story);
        const env = inst.catalog && inst.catalog.byId ? inst.catalog.byId[scene.environment] : null;
        if (!S.model.insertProp(draft, inst.selected, entry, env).ok) return;
        commit(inst, draft, true);
        notify(inst, inst.t('storymaker.library.added_prop', { name: S.catalogName(inst.t, entry) }));
    }

    async function addScene(inst, kind) {
        if (inst.readonly) return;
        const draft = S.model.cloneStory(inst.story);
        const index = S.model.addScene(draft, inst.selected, { kind });
        const id = draft.scenes[index].id;
        if (kind === 'code') {
            try {
                await inst.api.putCode(inst.projectId, id, S.codeTemplate, null);
            } catch (err) {
                notify(inst, S.errors.describe(inst.t, err), 'error');
                return;
            }
        }
        if (inst.disposed) return;
        commit(inst, draft, true);
        inst.selected = index;
        inst.bus.emit('selection');
        if (kind === 'code') openCode(inst, id);
    }

    function moveScene(inst, from, to) {
        if (inst.readonly) return;
        const draft = S.model.cloneStory(inst.story);
        const index = S.model.moveScene(draft, from, to);
        if (index < 0) return;
        commit(inst, draft, true);
        inst.selected = index;
        inst.bus.emit('selection');
    }

    async function deleteScene(inst, index) {
        const scene = S.model.scenesOf(inst.story)[index];
        if (!scene || inst.readonly) return;
        const ok = await askConfirm(inst, inst.t('storymaker.inspector.delete_scene'), inst.t('storymaker.inspector.delete_scene_text', { scene: scene.id }));
        if (!ok || inst.disposed) return;
        const draft = S.model.cloneStory(inst.story);
        const next = S.model.removeScene(draft, index);
        if (next < 0) return;
        if (inst.parts.code && scene.kind === 'code') inst.parts.code.forget(scene.id);
        commit(inst, draft, true);
        inst.selected = Math.min(next, draft.scenes.length - 1);
        inst.bus.emit('selection');
    }

    // ---- tabs, issues, banners --------------------------------------------------------------------------------------------------

    async function setTab(inst, name) {
        if (name === inst.tab) return;
        if (name === 'code') {
            if (inst.player) inst.player.pause();
            await flushEdits(inst);
        }
        inst.tab = name;
        const app = inst.parts.app;
        app.dataset.tab = name;
        inst.parts.tabButtons.forEach(button => button.setAttribute('aria-selected', String(button.dataset.tab === name)));
        app.querySelector('[data-stm="stage-wrap"]').hidden = name === 'code';
        inst.parts.code.el.hidden = name !== 'code';
        if (name === 'code') await inst.parts.code.show(inst.codeFile || 'story'); else inst.parts.code.hide();
        inst.bus.emit('tab');
    }

    async function openCode(inst, sceneId) {
        inst.codeFile = sceneId ? 'scene:' + sceneId : 'story';
        if (inst.tab === 'code') await inst.parts.code.show(inst.codeFile); else await setTab(inst, 'code');
    }

    function setSideTab(inst, name) {
        inst.sideTab = name;
        inst.parts.app.dataset.sidetab = name;
        inst.parts.sideTabs.forEach(button => button.setAttribute('aria-selected', String(button.dataset.tab === name)));
    }

    function allIssues(inst) {
        const report = inst.report || {};
        return S.model.visibleIssues(S.model.mergeIssues(inst.issues, report.issues, report.unverified, codeRuntimeIssues(inst)));
    }

    // Errors of code scenes found at run time (the bundle exists but its frame or build failed) join the list.
    function codeRuntimeIssues(inst) {
        return inst.playerErrors.filter(error => error.scene_id && (error.code === 'code_error' || error.code === 'deadline')).map(error => ({
            severity: 'error', code: error.code, path: '', scene_id: error.scene_id, message: error.message + (error.line ? ' (' + error.line + (error.column ? ':' + error.column : '') + ')' : '')
        }));
    }

    function toggleIssues(inst, force) {
        const panel = inst.parts.issuesPanel;
        const open = typeof force === 'boolean' ? force : panel.hidden;
        if (open && inst.tab === 'code') { setTab(inst, 'stage'); }
        panel.hidden = !open;
        inst.parts.issuesButton.setAttribute('aria-expanded', String(open));
        if (open) paintIssues(inst);
    }

    function focusIssue(inst, issue) {
        const index = S.model.sceneIndexOfIssue(issue, inst.story);
        if (index >= 0) select(inst, index, { seek: true });
    }

    function paintIssues(inst) {
        if (inst.disposed || !inst.parts.issuesPanel) return;
        const { el, icon } = S;
        const t = inst.t;
        const list = allIssues(inst);
        const counts = S.model.issueCounts(list);
        const button = inst.parts.issuesButton;
        // The badge counts what needs a look (errors and warnings); the notes of severity minor are in the list only.
        const total = counts.error + counts.warning;
        button.querySelector('[data-stm="issues-count"]').textContent = String(total);
        button.dataset.tone = counts.error ? 'bad' : counts.warning ? 'warn' : 'ok';
        button.setAttribute('aria-label', t('storymaker.issues.count', { count: total }));
        const panel = inst.parts.issuesPanel;
        if (panel.hidden) return;
        S.clear(panel);
        panel.appendChild(el('div', { class: 'stm-issues-head' }, el('h3', { text: t('storymaker.issues.title') }),
            el('button', { type: 'button', class: 'stm-btn stm-btn-quiet stm-btn-icon', ariaLabel: t('storymaker.close'), title: t('storymaker.close'), onClick: () => toggleIssues(inst, false) }, icon('close'))));
        if (!list.length) { panel.appendChild(el('p', { class: 'stm-muted', text: t('storymaker.issues.none') })); return; }
        const rows = el('ul', { class: 'stm-issue-list' });
        list.slice(0, 200).forEach(issue => {
            const index = S.model.sceneIndexOfIssue(issue, inst.story);
            rows.appendChild(el('li', { class: 'stm-issue', dataset: { severity: issue.severity, code: issue.code } },
                el('button', { type: 'button', class: 'stm-issue-go', onClick: () => focusIssue(inst, issue) },
                    icon(issue.severity === 'error' ? 'alert' : 'info'),
                    el('span', { class: 'stm-issue-body' },
                        el('strong', { text: S.errors.issueTitle(t, issue.code) }),
                        index >= 0 ? el('span', { class: 'stm-badge', text: t('storymaker.strip.scene_n', { n: index + 1 }) }) : null,
                        el('span', { class: 'stm-issue-text', text: issue.message }),
                        issue.fix ? el('span', { class: 'stm-issue-fix', text: issue.fix }) : null)),
                issue.path ? el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'issue-code' }, title: t('storymaker.issues.show_in_code'), onClick: () => showIssueInCode(inst, issue) }, icon('code')) : null));
        });
        panel.appendChild(rows);
    }

    async function showIssueInCode(inst, issue) {
        inst.parts.issuesPanel.hidden = true;
        inst.codeFile = 'story';
        await setTab(inst, 'code');
        await inst.parts.code.show('story');
        return inst.parts.code.gotoPath(issue);
    }

    function paintBanners(inst) {
        if (inst.disposed || !inst.parts.bannerArea) return;
        const { el, icon } = S;
        const t = inst.t;
        const area = inst.parts.bannerArea;
        S.clear(area);
        const add = (key, tone, text, actions) => area.appendChild(el('div', { class: 'stm-banner', role: 'status', dataset: { banner: key, tone } }, icon(tone === 'bad' ? 'alert' : 'info'), el('span', { text }), actions || null));
        if (inst.readonly) add('readonly', 'info', t('storymaker.banner.readonly'));
        if (inst.conflict) {
            add('conflict', 'bad', t('storymaker.banner.conflict'), el('span', { class: 'stm-actions' },
                el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'conflict-reload' }, onClick: () => reloadProject(inst) }, el('span', { text: t('storymaker.banner.conflict_reload') })),
                el('button', { type: 'button', class: 'stm-btn', dataset: { stm: 'conflict-overwrite' }, disabled: inst.readonly || null, onClick: () => overwrite(inst) }, el('span', { text: t('storymaker.banner.conflict_overwrite') }))));
        }
        if (inst.playerUnavailable) add('player_unavailable', 'bad', t('storymaker.banner.player_unavailable'), retryButton(inst, () => reloadPlayerFrame(inst)));
        const status = inst.status || {};
        const ffmpegOk = !!(status.ffmpeg && status.ffmpeg.ready), browserOk = !!(status.browser && status.browser.ready);
        if (!ffmpegOk || !browserOk) {
            const missing = [!ffmpegOk ? t('storymaker.export.cap_ffmpeg') : '', !browserOk ? t('storymaker.export.cap_browser') : ''].filter(Boolean).join(', ');
            add('export_unavailable', 'info', t('storymaker.banner.export_unavailable', { missing }));
        }
    }

    function reloadPlayerFrame(inst) {
        if (!inst.player) return;
        inst.playerUnavailable = false;
        inst.player.iframe.src = inst.api.playerUrl();
        paintBanners(inst);
        reloadPlayer(inst, inst.frame);
    }

    function paintSave(inst) {
        if (inst.disposed || !inst.parts.saveLabel) return;
        const t = inst.t;
        let key = 'storymaker.save_saved';
        let state = 'saved';
        if (inst.conflict) { key = 'storymaker.save_conflict'; state = 'conflict'; }
        else if (inst.saveState === 'error') { key = 'storymaker.save_error'; state = 'error'; }
        else if (inst.saveState === 'offline') { key = 'storymaker.save_offline'; state = 'offline'; }
        else if (inst.saveState === 'saving') { key = 'storymaker.save_saving'; state = 'saving'; }
        else if (inst.dirty) { key = 'storymaker.save_dirty'; state = 'dirty'; }
        else if (inst.readonly) { key = 'storymaker.save_readonly'; state = 'readonly'; }
        inst.parts.saveLabel.textContent = t(key);
        inst.parts.saveLabel.dataset.state = state;
    }

    // ---- saving ---------------------------------------------------------------------------------------------------------------

    function editScene(inst, sceneId, fn, options) {
        if (inst.readonly || !inst.story) return false;
        const draft = S.model.cloneStory(inst.story);
        const scene = S.model.scenesOf(draft).find(candidate => candidate && candidate.id === sceneId);
        if (!scene) return false;
        fn(scene, draft);
        commit(inst, draft, !!(options && options.immediate));
        return true;
    }

    function commit(inst, draft, immediate) {
        inst.story = draft;
        inst.dirty = true;
        inst.saveState = 'dirty';
        inst.bus.emit('story');
        clearTimeout(inst.saveTimer);
        if (immediate) saveNow(inst);
        else inst.saveTimer = setTimeout(() => saveNow(inst), SAVE_DELAY_MS);
    }

    function saveNow(inst) {
        clearTimeout(inst.saveTimer);
        if (inst.saving) { inst.saveAgain = true; return inst.saving; }
        inst.saving = runSave(inst).finally(() => { inst.saving = null; });
        return inst.saving;
    }

    async function runSave(inst) {
        if (inst.readonly || inst.conflict) return false;
        do {
            inst.saveAgain = false;
            if (!inst.dirty) break;
            inst.saveState = 'saving';
            paintSave(inst);
            const sent = inst.story;
            const text = JSON.stringify(sent);
            try {
                const result = await inst.api.saveStory(inst.projectId, text, inst.etag);
                if (inst.disposed) return false;
                inst.etag = result.etag || inst.etag;
                inst.issues = Array.isArray(result.issues) ? result.issues : [];
                inst.draftIssues = [];
                if (inst.story === sent) {
                    if (result.story && JSON.stringify(result.story) !== text) inst.story = result.story;
                    inst.dirty = false;
                } else {
                    inst.saveAgain = true;
                }
                inst.saveState = 'saved';
                afterStorySaved(inst);
            } catch (err) {
                if (inst.disposed || (err && err.aborted)) return false;
                handleSaveError(inst, err);
                return false;
            }
        } while (inst.saveAgain);
        paintSave(inst);
        return true;
    }

    function afterStorySaved(inst) {
        const scenes = S.model.scenesOf(inst.story);
        inst.selected = S.clamp(inst.selected, 0, Math.max(0, scenes.length - 1));
        inst.bus.emit('story');
        inst.bus.emit('issues');
        if (inst.parts.code) inst.parts.code.storyChanged();
        schedulePreview(inst);
        paintSave(inst);
    }

    function handleSaveError(inst, err) {
        const e = S.errors.normalize(err);
        if (e.status === 412) { inst.conflict = true; inst.saveState = 'error'; }
        else if (e.code === 'desktop_readonly') { inst.forcedReadonly = true; inst.saveState = 'error'; inst.bus.emit('readonly'); }
        else if (e.status === 422) { inst.saveState = 'error'; inst.issues = e.issues.length ? e.issues : inst.issues; inst.bus.emit('issues'); }
        else inst.saveState = e.network ? 'offline' : 'error';
        notify(inst, S.errors.describe(inst.t, e), 'error');
        paintBanners(inst);
        paintSave(inst);
    }

    // Everything typed so far is on the server (or the reason it is not has been shown): true when the story is saved.
    async function flushEdits(inst) {
        clearTimeout(inst.saveTimer);
        if (inst.saving) await inst.saving;
        if (inst.dirty && !inst.conflict) await saveNow(inst);
        return !inst.dirty && !inst.conflict;
    }

    async function saveStoryText(inst, text) {
        await flushEdits(inst);
        if (inst.conflict) return { ok: false };
        try {
            const result = await inst.api.saveStory(inst.projectId, text, inst.etag);
            inst.etag = result.etag || inst.etag;
            inst.story = result.story || inst.story;
            inst.issues = Array.isArray(result.issues) ? result.issues : [];
            inst.draftIssues = [];
            inst.dirty = false;
            inst.saveState = 'saved';
            inst.catalogMismatch = false;
            afterStorySaved(inst);
            notify(inst, inst.t('storymaker.code.saved'));
            return { ok: true };
        } catch (err) {
            const e = S.errors.normalize(err);
            if (e.status === 422) {
                inst.draftIssues = e.issues;
                inst.bus.emit('issues');
                notify(inst, inst.t('storymaker.error.invalid_story'), 'error');
                return { ok: false, issues: e.issues };
            }
            handleSaveError(inst, e);
            return { ok: false };
        }
    }

    async function saveCodeScene(inst, sceneId, text, etag) {
        const result = await inst.api.putCode(inst.projectId, sceneId, text, etag);
        const entry = { scene_id: sceneId, ok: !!result.ok, errors: Array.isArray(result.errors) ? result.errors : [] };
        inst.codeStatus = inst.codeStatus.filter(item => item.scene_id !== sceneId).concat([entry]);
        refreshIssues(inst);
        if (result.ok) { schedulePreview(inst); notify(inst, inst.t('storymaker.code.bundle_ok')); } else notify(inst, inst.t('storymaker.code.bundle_failed'), 'error');
        inst.bus.emit('story');
        return result;
    }

    async function refreshIssues(inst) {
        try {
            const data = await inst.api.getProject(inst.projectId);
            if (inst.disposed || !inst.story) return;
            inst.issues = Array.isArray(data.issues) ? data.issues : inst.issues;
            inst.codeStatus = Array.isArray(data.code_scenes) ? data.code_scenes : inst.codeStatus;
            inst.bus.emit('issues');
        } catch (_) { /* the list stays as it was */ }
    }

    async function reloadProject(inst) {
        try {
            const data = await inst.api.getProject(inst.projectId);
            if (inst.disposed) return;
            inst.story = data.story;
            inst.etag = data.etag || '';
            inst.issues = Array.isArray(data.issues) ? data.issues : [];
            inst.codeStatus = Array.isArray(data.code_scenes) ? data.code_scenes : [];
            inst.draftIssues = [];
            inst.dirty = false;
            inst.conflict = false;
            inst.saveState = 'saved';
            inst.selected = S.clamp(inst.selected, 0, Math.max(0, S.model.scenesOf(inst.story).length - 1));
            paintBanners(inst);
            afterStorySaved(inst);
        } catch (err) {
            if (!inst.disposed && !(err && err.aborted)) notify(inst, S.errors.describe(inst.t, err), 'error');
        }
    }

    // Keep my version: take the etag of the stored story and save mine over it.
    async function overwrite(inst) {
        try {
            const data = await inst.api.getProject(inst.projectId);
            if (inst.disposed) return;
            inst.etag = data.etag || '';
            inst.conflict = false;
            inst.dirty = true;
            paintBanners(inst);
            await saveNow(inst);
        } catch (err) {
            if (!inst.disposed && !(err && err.aborted)) notify(inst, S.errors.describe(inst.t, err), 'error');
        }
    }

    async function leaveProject(inst) {
        const saved = await flushEdits(inst);
        if (!saved && inst.conflict) {
            const leave = await askConfirm(inst, inst.t('storymaker.leave_title'), inst.t('storymaker.leave_text'));
            if (!leave) return;
        }
        if (inst.parts.code && inst.parts.code.dirty()) {
            const leave = await askConfirm(inst, inst.t('storymaker.leave_title'), inst.t('storymaker.leave_code_text'));
            if (!leave) return;
        }
        if (inst.disposed) return;
        await showProjects(inst);
    }

    // The window is closing: save what is typed; unsaved code or a conflict asks first.
    async function beforeClose(inst) {
        if (inst.view !== 'editor') return true;
        const saved = await flushEdits(inst);
        const codeDirty = !!(inst.parts.code && inst.parts.code.dirty());
        if (saved && !codeDirty) return true;
        return askConfirm(inst, inst.t('storymaker.leave_title'), codeDirty ? inst.t('storymaker.leave_code_text') : inst.t('storymaker.leave_text'));
    }

    // ---- window -----------------------------------------------------------------------------------------------------------------

    function render(host, windowId, ctx) {
        dispose(windowId);
        const inst = createInstance(host, windowId, ctx);
        instances.set(windowId, inst);
        inst.content = S.el('div', { class: 'stm-view' });
        inst.parts.live = S.el('div', { class: 'stm-live', role: 'status', ariaLive: 'polite', dataset: { stm: 'live' } });
        inst.root = S.el('div', { class: 'stm-app', dataset: { stm: 'app' } }, inst.content, inst.parts.live);
        S.clear(host);
        host.appendChild(inst.root);
        inst.bag.on(document, 'aurago:desktop-policy', event => {
            const detail = event && event.detail;
            if (!detail || typeof detail.readonly !== 'boolean') return;
            inst.ctxReadonly = detail.readonly;
            if (!detail.readonly) inst.forcedReadonly = false;
            inst.bus.emit('readonly');
            if (inst.parts.projects) inst.parts.projects.refresh();
        });
        if (typeof inst.ctx.setWindowBeforeClose === 'function') inst.ctx.setWindowBeforeClose(windowId, () => beforeClose(inst));
        start(inst);
        return inst;
    }

    function dispose(windowId) {
        const inst = instances.get(windowId);
        if (!inst) return;
        instances.delete(windowId);
        inst.disposed = true;
        clearTimeout(inst.liveTimer);
        inst.api.abort();
        closeEditor(inst);
        inst.bag.dispose();
    }

    window.StorymakerApp = { render, dispose };
})();
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestStorymakerLoaderListsEveryScript|TestStorymakerApp|TestDesktopStorymakerRegistration' -count=1 -timeout 10m`
Expected: ok (the browser tests run, about 25 s)

Browser tests run with `AURAGO_RUN_BROWSER_SMOKE=1` (the installed Chrome; on the Windows dev box through `go test` with the sandbox disabled).

Recorded at drafting time:

```text
ok  	aurago/ui	18.767s
```

- [ ] **Step 5: Commit**

```bash
git add ui/desktop_storymaker_fixture_test.go ui/desktop_storymaker_browser_test.go ui/js/desktop/apps/storymaker.js
git commit -m "feat(storymaker): the Storymaker window and its browser tour" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E9: Configuration section

The Storymaker block of Configuration, Virtual Desktop: `storymaker.enabled`, `storymaker.render.timeout_seconds` (60-14400), `max_duration_seconds` (10-3600), `browser_path`, `transport`, and a check button that asks `GET /api/desktop/storymaker/status` for FFmpeg and the browser (never against unsaved settings). The 21 `config.storymaker.*` texts go into `ui/lang/config/common/<lang>.json` (16 languages). The wave-1 guard that counts the runtime-only checks of the section now counts four.

**Files:**
- Modify: `ui/cfg/virtual_desktop.js` (after base line 92 (`html += vdCfgStorymaker();` after the Retro Games block) and after base line 261, the end of the file (two functions))
- Modify: `ui/lang/config/common/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` (after base line 33, the last `config.retro_games.*` key)
- Modify: `ui/config_wave1_batch4_test.go` (base lines 114-117)
- Test: `ui/config_storymaker_test.go`

- [ ] **Step 0: GitNexus impact (project rule)**

```bash
node C:/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs impact "renderVirtualDesktopSection" --direction upstream --repo .
```
Expected: the function gains one `html += vdCfgStorymaker();` line after the Retro Games block; nothing else in the section moves. Confirm an empty answer with `grep -rn "renderVirtualDesktopSection" ui`.

- [ ] **Step 1: Write the failing test**

`ui/config_storymaker_test.go`:

```go
package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The Storymaker block of the Virtual Desktop configuration page: storymaker.enabled and storymaker.render.{timeout_seconds,
// max_duration_seconds, browser_path, transport}, the export-readiness check against /api/desktop/storymaker/status, and the
// sixteen-language texts in lang/config/common.

var storymakerConfigKeys = []string{
	"title", "help", "enabled", "timeout", "timeout_help", "max_duration", "max_duration_help", "browser_path", "browser_path_help",
	"transport", "transport_help", "transport_auto", "check", "checking", "unavailable", "save_first", "ready", "missing", "disabled",
	"part_ffmpeg", "part_browser",
}

func TestConfigStorymakerBlockIsWired(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("cfg", "virtual_desktop.js"))
	if err != nil {
		t.Fatal(err)
	}
	src := strings.ReplaceAll(string(data), "\r\n", "\n")
	for _, marker := range []string{
		"html += vdCfgStorymaker();",
		"function vdCfgStorymaker()",
		`data-path="storymaker.render.timeout_seconds"`,
		`min="60" max="14400"`,
		`data-path="storymaker.render.max_duration_seconds"`,
		`min="10" max="3600"`,
		`data-path="storymaker.render.browser_path"`,
		`data-path="storymaker.render.transport"`,
		"'storymaker.enabled'",
		"fetch('/api/desktop/storymaker/status'",
		"hasUnsavedConfigChanges()",
	} {
		if !strings.Contains(src, marker) {
			t.Errorf("virtual_desktop.js lacks %q", marker)
		}
	}
	for _, key := range storymakerConfigKeys {
		if !strings.Contains(src, "'"+key+"'") && !strings.Contains(src, "'config.storymaker."+key+"'") {
			t.Errorf("virtual_desktop.js never uses config.storymaker.%s", key)
		}
	}
	// The limits of the form are the limits of NormalizeStorymakerConfig.
	normalize, err := os.ReadFile(filepath.Join("..", "internal", "config", "storymaker.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, limit := range []string{"60", "14400", "10", "3600"} {
		if !strings.Contains(string(normalize), limit) {
			t.Errorf("internal/config/storymaker.go no longer mentions the limit %s the form promises", limit)
		}
	}
}

func TestConfigStorymakerTranslations(t *testing.T) {
	t.Parallel()
	english := map[string]string{}
	for _, lang := range storymakerLanguages {
		values := storymakerConfigLocale(t, lang)
		if lang == "en" {
			for key, value := range values {
				english[key] = value
			}
		}
		for _, key := range storymakerConfigKeys {
			full := "config.storymaker." + key
			text := strings.TrimSpace(values[full])
			if text == "" || strings.ContainsRune(text, '�') {
				t.Errorf("%s: %s is missing or broken", lang, full)
			}
			if strings.Contains(text, "{missing}") != (key == "missing") {
				t.Errorf("%s: %s has the wrong placeholders", lang, full)
			}
			if lang == "de" && (strings.Contains(" "+text+" ", " Sie ") || strings.Contains(" "+text+" ", " Ihre ") || strings.Contains(" "+text+" ", " Ihnen ")) {
				t.Errorf("de: %s uses formal address", full)
			}
		}
	}
	// Names that stay as they are in several languages.
	same := map[string]bool{"title": true, "part_ffmpeg": true, "part_browser": true}
	for _, lang := range storymakerLanguages {
		if lang == "en" {
			continue
		}
		values := storymakerConfigLocale(t, lang)
		for _, key := range storymakerConfigKeys {
			full := "config.storymaker." + key
			if values[full] == english[full] && !same[key] {
				t.Errorf("%s: %s is untranslated", lang, full)
			}
		}
	}
}

func storymakerConfigLocale(t *testing.T, lang string) map[string]string {
	t.Helper()
	return storymakerLocale(t, "config/common", lang)
}

func TestConfigStorymakerBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview").Timeout(30 * time.Second)
	page.MustWaitLoad()
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async () => {
        const originalFetch = window.fetch;
        window.storymakerStatusCalls = 0;
        window.storymakerStatus = {enabled: true, ffmpeg: {ready: false}, browser: {ready: true}};
        window.fetch = (url, options = {}) => {
            if (String(url) === '/api/desktop/storymaker/status') {
                storymakerStatusCalls++;
                return Promise.resolve(new Response(JSON.stringify(storymakerStatus), {headers: {'Content-Type': 'application/json'}}));
            }
            if (String(url) === '/api/config' && options.method === 'PUT') window.storymakerSavedConfig = JSON.parse(options.body);
            return originalFetch(url, options);
        };
        await selectSection('virtual_desktop'); resetDirtySnapshot();
    }`)
	// Defaults of a configuration that has no storymaker section: on, 3600 s, 900 s, no browser path, automatic transfer.
	if !page.MustEval(`() => document.querySelector('[data-path="storymaker.enabled"]').classList.contains('on') && document.querySelector('[data-path="storymaker.render.timeout_seconds"]').value==='3600' && document.querySelector('[data-path="storymaker.render.max_duration_seconds"]').value==='900' && document.querySelector('[data-path="storymaker.render.browser_path"]').value==='' && document.querySelector('[data-path="storymaker.render.transport"]').value===''`).Bool() {
		t.Fatal("the Storymaker defaults are not what the server applies")
	}
	page.MustEval(`() => {
        const set = (path, value) => { const input = document.querySelector('[data-path="' + path + '"]'); input.value = value; input.dispatchEvent(new Event('change', {bubbles: true})); };
        set('storymaker.render.timeout_seconds', '1800'); set('storymaker.render.max_duration_seconds', '300');
        set('storymaker.render.browser_path', '/usr/bin/chromium'); set('storymaker.render.transport', 'png');
        toggleBool(document.querySelector('[data-path="storymaker.enabled"]'));
    }`)
	page.MustEval(`async () => vdCfgTestStorymaker(document.getElementById('vd-storymaker-btn'))`)
	if !page.MustEval(`() => storymakerStatusCalls===0 && document.getElementById('vd-storymaker-result').textContent===t('config.storymaker.save_first')`).Bool() {
		t.Fatal("the check must not run against unsaved settings")
	}
	if !page.MustEval(`async () => await saveConfig()`).Bool() {
		t.Fatal("the Storymaker settings could not be saved")
	}
	if !page.MustEval(`() => { const s = storymakerSavedConfig.storymaker; return s.enabled===false && s.render.timeout_seconds===1800 && s.render.max_duration_seconds===300 && s.render.browser_path==='/usr/bin/chromium' && s.render.transport==='png'; }`).Bool() {
		t.Fatalf("saving the Desktop section omitted the Storymaker settings: %s", page.MustEval(`() => JSON.stringify((window.storymakerSavedConfig||{}).storymaker)`).String())
	}
	// The check names what is missing, then reports ready.
	page.MustEval(`async () => vdCfgTestStorymaker(document.getElementById('vd-storymaker-btn'))`)
	waitForJSBool(t, page, `() => storymakerStatusCalls===1 && document.getElementById('vd-storymaker-result').textContent===t('config.storymaker.missing').replace('{missing}', t('config.storymaker.part_ffmpeg'))`)
	page.MustEval(`() => { storymakerStatus = {enabled: true, ffmpeg: {ready: true}, browser: {ready: true}}; }`)
	page.MustEval(`async () => vdCfgTestStorymaker(document.getElementById('vd-storymaker-btn'))`)
	waitForJSBool(t, page, `() => storymakerStatusCalls===2 && document.getElementById('vd-storymaker-result').textContent===t('config.storymaker.ready')`)
	page.MustSetViewport(390, 844, 1, true)
	if page.MustEval(`() => document.documentElement.scrollWidth>innerWidth+1`).Bool() {
		t.Fatal("the Storymaker configuration overflows the narrow viewport")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./ui -run 'TestConfigStorymaker(BlockIsWired|Translations)' -count=1`
Expected: FAIL: FAIL (`virtual_desktop.js lacks "html += vdCfgStorymaker();"`)

Recorded at drafting time (the replay of this task on a tree without it):

```text
--- FAIL: TestConfigStorymakerBlockIsWired (0.00s)
    config_storymaker_test.go:42: virtual_desktop.js lacks "html += vdCfgStorymaker();"
    config_storymaker_test.go:42: virtual_desktop.js lacks "function vdCfgStorymaker()"
    config_storymaker_test.go:42: virtual_desktop.js lacks "data-path=\"storymaker.render.timeout_seconds\""
    config_storymaker_test.go:42: virtual_desktop.js lacks "min=\"60\" max=\"14400\""
    config_storymaker_test.go:42: virtual_desktop.js lacks "data-path=\"storymaker.render.max_duration_seconds\""
    config_storymaker_test.go:42: virtual_desktop.js lacks "min=\"10\" max=\"3600\""
    config_storymaker_test.go:42: virtual_desktop.js lacks "data-path=\"storymaker.render.browser_path\""
    config_storymaker_test.go:42: virtual_desktop.js lacks "data-path=\"storymaker.render.transport\""
    config_storymaker_test.go:42: virtual_desktop.js lacks "'storymaker.enabled'"
    config_storymaker_test.go:42: virtual_desktop.js lacks "fetch('/api/desktop/storymaker/status'"
    config_storymaker_test.go:47: virtual_desktop.js never uses config.storymaker.timeout_help
    config_storymaker_test.go:47: virtual_desktop.js never uses config.storymaker.max_duration
    config_storymaker_test.go:47: virtual_desktop.js never uses config.storymaker.max_duration_help
```

- [ ] **Step 3: Implement**

`ui/cfg/virtual_desktop.js` (after base line 92 (`html += vdCfgStorymaker();` after the Retro Games block) and after base line 261, the end of the file (two functions)):

```diff
--- a/ui/cfg/virtual_desktop.js
+++ b/ui/cfg/virtual_desktop.js
@@ -90,6 +90,7 @@ function renderVirtualDesktopSection(section) {
 
     html += vdCfgVideoStudio();
     html += vdCfgRetroGames();
+    html += vdCfgStorymaker();
     const infra = data.infra || {};
     html += '<h3>Infra</h3>';
     html += vdCfgToggleRow('config.infra.enabled', 'config.infra.help', infra.enabled === true, 'virtual_desktop.infra.enabled');
@@ -259,3 +260,46 @@ function vdCfgTestRetroGames(button) {
         }
     });
 }
+
+function vdCfgStorymaker() {
+    const data = configData.storymaker || (configData.storymaker = {});
+    if (typeof data.enabled !== 'boolean') data.enabled = true;
+    const render = data.render || (data.render = {});
+    const prefix = 'config.storymaker.';
+    let html = '<h3>' + t(prefix + 'title') + '</h3><p class="field-help">' + t(prefix + 'help') + '</p>';
+    html += '<div class="field-grid two-cols">';
+    html += vdCfgToggleRow(prefix + 'enabled', '', data.enabled === true, 'storymaker.enabled');
+    html += '</div><div class="field-grid two-cols">';
+    html += vdCfgField(prefix + 'timeout', prefix + 'timeout_help', '<input class="field-input" type="number" min="60" max="14400" value="' + escapeAttr(String(render.timeout_seconds || 3600)) + '" data-path="storymaker.render.timeout_seconds">');
+    html += vdCfgField(prefix + 'max_duration', prefix + 'max_duration_help', '<input class="field-input" type="number" min="10" max="3600" value="' + escapeAttr(String(render.max_duration_seconds || 900)) + '" data-path="storymaker.render.max_duration_seconds">');
+    html += vdCfgField(prefix + 'browser_path', prefix + 'browser_path_help', '<input class="field-input" type="text" spellcheck="false" autocomplete="off" value="' + escapeAttr(render.browser_path || '') + '" data-path="storymaker.render.browser_path">');
+    const transports = { '': t(prefix + 'transport_auto'), png: 'PNG', webp: 'WebP', raw: 'RGBA', screenshot: 'Screenshot' };
+    html += vdCfgField(prefix + 'transport', prefix + 'transport_help', '<select class="field-select" data-path="storymaker.render.transport">' + Object.keys(transports).map(value => '<option value="' + value + '"' + ((render.transport || '') === value ? ' selected' : '') + '>' + escapeAttr(transports[value]) + '</option>').join('') + '</select>');
+    html += '</div><div class="field-group"><button type="button" id="vd-storymaker-btn" class="btn-save dc-test-btn" data-config-action-draft="false" onclick="vdCfgTestStorymaker(this)">' + t(prefix + 'check') + '</button>' + cfgActionResult.html('vd-storymaker-result') + '</div>';
+    return html;
+}
+
+function vdCfgTestStorymaker(button) {
+    const prefix = 'config.storymaker.';
+    if (hasUnsavedConfigChanges()) {
+        cfgActionResult.show('vd-storymaker-result', 'warning', t(prefix + 'save_first'));
+        return;
+    }
+    const missing = status => [!(status.ffmpeg && status.ffmpeg.ready) ? t(prefix + 'part_ffmpeg') : '', !(status.browser && status.browser.ready) ? t(prefix + 'part_browser') : ''].filter(Boolean);
+    // The status asks FFmpeg and launches the browser once (a cached probe), so allow it some time.
+    return cfgActionResult(button, () => fetch('/api/desktop/storymaker/status', { signal: AbortSignal.timeout(60000) }), {
+        target: 'vd-storymaker-result',
+        pending: t(prefix + 'checking'),
+        failure: t(prefix + 'unavailable'),
+        interpret: outcome => {
+            if (!outcome.ok) return {};
+            const status = outcome.body || {};
+            return status.enabled === false || missing(status).length ? { state: 'warning', reason: '' } : {};
+        },
+        warning: outcome => {
+            const status = (outcome && outcome.body) || {};
+            return status.enabled === false ? t(prefix + 'disabled') : t(prefix + 'missing').replace('{missing}', missing(status).join(', '));
+        },
+        success: t(prefix + 'ready')
+    });
+}
```

`ui/lang/config/common/cs.json` (after base line 33, the last `config.retro_games.*` key):

```diff
--- a/ui/lang/config/common/cs.json
+++ b/ui/lang/config/common/cs.json
@@ -31,6 +31,27 @@
   "config.retro_games.status_building": "Index se právě vytváří",
   "config.retro_games.unavailable": "Retro Games není k dispozici",
   "config.retro_games.save_first": "Nejdřív ulož nastavení",
+  "config.storymaker.title": "Storymaker",
+  "config.storymaker.help": "Studio vysvětlujících videí na ploše. Příběhy se upravují v aplikaci Storymaker a exportují jako soubory MP4. Export potřebuje na serveru FFmpeg a prohlížeč Chrome nebo Chromium; úpravy a náhled fungují i bez nich.",
+  "config.storymaker.enabled": "Zapnout Storymaker",
+  "config.storymaker.timeout": "Časový limit exportu (sekundy)",
+  "config.storymaker.timeout_help": "Export, který trvá déle, se zastaví. Delší videa dostanou automaticky více času. Od 60 do 14400.",
+  "config.storymaker.max_duration": "Nejdelší video (sekundy)",
+  "config.storymaker.max_duration_help": "Příběhy, ze kterých by vzniklo delší video, se neexportují. Od 10 do 3600.",
+  "config.storymaker.browser_path": "Cesta k prohlížeči (volitelné)",
+  "config.storymaker.browser_path_help": "Absolutní cesta k Chrome nebo Chromium, který kreslí snímky. Prázdné znamená prohlížeč nainstalovaný v tomto systému. V Dockeru zadej cestu k prohlížeči uvnitř kontejneru.",
+  "config.storymaker.transport": "Přenos snímků",
+  "config.storymaker.transport_help": "Jak se nakreslené snímky dostanou k FFmpeg. Nech Automaticky, pokud tě podpora nepožádá o změnu.",
+  "config.storymaker.transport_auto": "Automaticky",
+  "config.storymaker.check": "Zkontrolovat nastavení exportu",
+  "config.storymaker.checking": "Kontroluje se…",
+  "config.storymaker.unavailable": "Storymaker neodpověděl.",
+  "config.storymaker.save_first": "Nejdřív ulož konfiguraci, potom zkontroluj.",
+  "config.storymaker.ready": "Připraveno: FFmpeg i prohlížeč jsou k dispozici.",
+  "config.storymaker.missing": "Export ještě není připraven. Chybí: {missing}.",
+  "config.storymaker.disabled": "Storymaker je vypnutý.",
+  "config.storymaker.part_ffmpeg": "FFmpeg",
+  "config.storymaker.part_browser": "prohlížeč",
   "config.refresh.load_error": "Tuto sekci se nepodařilo načíst.",
   "config.refresh.retry": "Zkusit znovu",
   "config.refresh.toggle_password": "Zobrazit nebo skrýt heslo",
```

`ui/lang/config/common/da.json` (after base line 33, the last `config.retro_games.*` key):

```diff
--- a/ui/lang/config/common/da.json
+++ b/ui/lang/config/common/da.json
@@ -31,6 +31,27 @@
   "config.retro_games.status_building": "Indekset opbygges lige nu",
   "config.retro_games.unavailable": "Retro Games er ikke tilgængelig",
   "config.retro_games.save_first": "Gem indstillingerne først",
+  "config.storymaker.title": "Storymaker",
+  "config.storymaker.help": "Studie til forklaringsvideoer på skrivebordet. Historier redigeres i appen Storymaker og eksporteres som MP4-filer. Eksport kræver FFmpeg og en Chrome- eller Chromium-browser på serveren; redigering og forhåndsvisning virker uden dem.",
+  "config.storymaker.enabled": "Aktivér Storymaker",
+  "config.storymaker.timeout": "Tidsgrænse for eksport (sekunder)",
+  "config.storymaker.timeout_help": "En eksport, der varer længere, stoppes. Længere videoer får automatisk mere tid. Mellem 60 og 14400.",
+  "config.storymaker.max_duration": "Længste video (sekunder)",
+  "config.storymaker.max_duration_help": "Historier, der ville give en længere video, eksporteres ikke. Mellem 10 og 3600.",
+  "config.storymaker.browser_path": "Browserens sti (valgfrit)",
+  "config.storymaker.browser_path_help": "Absolut sti til Chrome eller Chromium, som tegner billederne. Tomt betyder den browser, der er installeret på dette system. I Docker angiver du stien til en browser i containeren.",
+  "config.storymaker.transport": "Overførsel af billeder",
+  "config.storymaker.transport_help": "Hvordan de tegnede billeder når frem til FFmpeg. Behold Automatisk, medmindre supporten beder dig ændre det.",
+  "config.storymaker.transport_auto": "Automatisk",
+  "config.storymaker.check": "Tjek eksportopsætningen",
+  "config.storymaker.checking": "Tjekker …",
+  "config.storymaker.unavailable": "Storymaker svarede ikke.",
+  "config.storymaker.save_first": "Gem først konfigurationen, og tjek derefter.",
+  "config.storymaker.ready": "Klar: FFmpeg og en browser er tilgængelige.",
+  "config.storymaker.missing": "Eksporten er ikke klar endnu. Mangler: {missing}.",
+  "config.storymaker.disabled": "Storymaker er slået fra.",
+  "config.storymaker.part_ffmpeg": "FFmpeg",
+  "config.storymaker.part_browser": "browser",
   "config.refresh.load_error": "Dette område kunne ikke indlæses.",
   "config.refresh.retry": "Prøv igen",
   "config.refresh.toggle_password": "Vis eller skjul adgangskode",
```

`ui/lang/config/common/de.json` (after base line 33, the last `config.retro_games.*` key):

```diff
--- a/ui/lang/config/common/de.json
+++ b/ui/lang/config/common/de.json
@@ -31,6 +31,27 @@
   "config.retro_games.status_building": "Der Index wird gerade aufgebaut",
   "config.retro_games.unavailable": "Retro Games ist nicht verfügbar",
   "config.retro_games.save_first": "Speichere zuerst die Einstellungen",
+  "config.storymaker.title": "Storymaker",
+  "config.storymaker.help": "Erklärvideo-Studio auf dem Desktop. Geschichten werden in der App Storymaker bearbeitet und als MP4-Dateien exportiert. Der Export braucht FFmpeg und einen Chrome- oder Chromium-Browser auf dem Server; Bearbeiten und Vorschau funktionieren auch ohne.",
+  "config.storymaker.enabled": "Storymaker aktivieren",
+  "config.storymaker.timeout": "Zeitlimit für den Export (Sekunden)",
+  "config.storymaker.timeout_help": "Ein Export, der länger läuft, wird gestoppt. Längere Videos bekommen automatisch mehr Zeit. Zwischen 60 und 14400.",
+  "config.storymaker.max_duration": "Längstes Video (Sekunden)",
+  "config.storymaker.max_duration_help": "Geschichten, aus denen ein längeres Video würde, werden nicht exportiert. Zwischen 10 und 3600.",
+  "config.storymaker.browser_path": "Browser-Pfad (optional)",
+  "config.storymaker.browser_path_help": "Absoluter Pfad von Chrome oder Chromium, der die Bilder zeichnet. Leer bedeutet den auf diesem System installierten Browser. In Docker gibst du den Pfad eines Browsers im Container an.",
+  "config.storymaker.transport": "Bildübertragung",
+  "config.storymaker.transport_help": "Wie die gezeichneten Bilder zu FFmpeg gelangen. Lass Automatisch eingestellt, außer der Support bittet dich um eine Änderung.",
+  "config.storymaker.transport_auto": "Automatisch",
+  "config.storymaker.check": "Export-Einrichtung prüfen",
+  "config.storymaker.checking": "Wird geprüft…",
+  "config.storymaker.unavailable": "Storymaker hat nicht geantwortet.",
+  "config.storymaker.save_first": "Speichere zuerst die Konfiguration und prüfe dann.",
+  "config.storymaker.ready": "Bereit: FFmpeg und ein Browser sind verfügbar.",
+  "config.storymaker.missing": "Der Export ist noch nicht bereit. Es fehlt: {missing}.",
+  "config.storymaker.disabled": "Storymaker ist ausgeschaltet.",
+  "config.storymaker.part_ffmpeg": "FFmpeg",
+  "config.storymaker.part_browser": "Browser",
   "config.refresh.load_error": "Dieser Bereich konnte nicht geladen werden.",
   "config.refresh.retry": "Erneut versuchen",
   "config.refresh.toggle_password": "Passwort ein- oder ausblenden",
```

`ui/lang/config/common/el.json` (after base line 33, the last `config.retro_games.*` key):

```diff
--- a/ui/lang/config/common/el.json
+++ b/ui/lang/config/common/el.json
@@ -31,6 +31,27 @@
   "config.retro_games.status_building": "Το ευρετήριο δημιουργείται αυτή τη στιγμή",
   "config.retro_games.unavailable": "Το Retro Games δεν είναι διαθέσιμο",
   "config.retro_games.save_first": "Αποθήκευσε πρώτα τις ρυθμίσεις",
+  "config.storymaker.title": "Storymaker",
+  "config.storymaker.help": "Στούντιο επεξηγηματικών βίντεο στην επιφάνεια εργασίας. Οι ιστορίες επεξεργάζονται στην εφαρμογή Storymaker και εξάγονται ως αρχεία MP4. Η εξαγωγή χρειάζεται FFmpeg και πρόγραμμα περιήγησης Chrome ή Chromium στον διακομιστή· η επεξεργασία και η προεπισκόπηση λειτουργούν και χωρίς αυτά.",
+  "config.storymaker.enabled": "Ενεργοποίηση του Storymaker",
+  "config.storymaker.timeout": "Χρονικό όριο εξαγωγής (δευτερόλεπτα)",
+  "config.storymaker.timeout_help": "Μια εξαγωγή που διαρκεί περισσότερο σταματά. Τα μεγαλύτερα βίντεο παίρνουν αυτόματα περισσότερο χρόνο. Από 60 έως 14400.",
+  "config.storymaker.max_duration": "Μεγαλύτερο βίντεο (δευτερόλεπτα)",
+  "config.storymaker.max_duration_help": "Ιστορίες που θα έδιναν μεγαλύτερο βίντεο δεν εξάγονται. Από 10 έως 3600.",
+  "config.storymaker.browser_path": "Διαδρομή προγράμματος περιήγησης (προαιρετικό)",
+  "config.storymaker.browser_path_help": "Απόλυτη διαδρομή του Chrome ή του Chromium, που σχεδιάζει τα καρέ. Κενό σημαίνει το πρόγραμμα περιήγησης που είναι εγκατεστημένο σε αυτό το σύστημα. Στο Docker, δώστε τη διαδρομή ενός προγράμματος περιήγησης μέσα στο κοντέινερ.",
+  "config.storymaker.transport": "Μεταφορά καρέ",
+  "config.storymaker.transport_help": "Πώς τα σχεδιασμένα καρέ φτάνουν στο FFmpeg. Αφήστε το Αυτόματο, εκτός αν η υποστήριξη σας ζητήσει να το αλλάξετε.",
+  "config.storymaker.transport_auto": "Αυτόματο",
+  "config.storymaker.check": "Έλεγχος της ρύθμισης εξαγωγής",
+  "config.storymaker.checking": "Έλεγχος…",
+  "config.storymaker.unavailable": "Το Storymaker δεν απάντησε.",
+  "config.storymaker.save_first": "Αποθηκεύστε πρώτα τη ρύθμιση και μετά ελέγξτε.",
+  "config.storymaker.ready": "Έτοιμο: το FFmpeg και ένα πρόγραμμα περιήγησης είναι διαθέσιμα.",
+  "config.storymaker.missing": "Η εξαγωγή δεν είναι ακόμα έτοιμη. Λείπει: {missing}.",
+  "config.storymaker.disabled": "Το Storymaker είναι απενεργοποιημένο.",
+  "config.storymaker.part_ffmpeg": "FFmpeg",
+  "config.storymaker.part_browser": "πρόγραμμα περιήγησης",
   "config.refresh.load_error": "Δεν ήταν δυνατή η φόρτωση αυτής της ενότητας.",
   "config.refresh.retry": "Δοκιμή ξανά",
   "config.refresh.toggle_password": "Εμφάνιση ή απόκρυψη κωδικού",
```

`ui/lang/config/common/en.json` (after base line 33, the last `config.retro_games.*` key):

```diff
--- a/ui/lang/config/common/en.json
+++ b/ui/lang/config/common/en.json
@@ -31,6 +31,27 @@
   "config.retro_games.status_building": "The index is being built right now",
   "config.retro_games.unavailable": "Retro Games is not available",
   "config.retro_games.save_first": "Save the settings first",
+  "config.storymaker.title": "Storymaker",
+  "config.storymaker.help": "Explainer video studio on the desktop. Stories are edited in the Storymaker app and exported as MP4 files. Exporting needs FFmpeg and a Chrome or Chromium browser on the server; editing and the preview work without them.",
+  "config.storymaker.enabled": "Enable Storymaker",
+  "config.storymaker.timeout": "Export time limit (seconds)",
+  "config.storymaker.timeout_help": "An export that runs longer is stopped. Longer videos automatically get more time. Between 60 and 14400.",
+  "config.storymaker.max_duration": "Longest video (seconds)",
+  "config.storymaker.max_duration_help": "Stories that would make a longer video are not exported. Between 10 and 3600.",
+  "config.storymaker.browser_path": "Browser path (optional)",
+  "config.storymaker.browser_path_help": "Absolute path of Chrome or Chromium, used to draw the frames. Empty means the browser installed on this system. In Docker, enter the path of a browser inside the container.",
+  "config.storymaker.transport": "Frame transfer",
+  "config.storymaker.transport_help": "How the drawn frames reach FFmpeg. Keep Automatic unless support asks you to change it.",
+  "config.storymaker.transport_auto": "Automatic",
+  "config.storymaker.check": "Check export setup",
+  "config.storymaker.checking": "Checking…",
+  "config.storymaker.unavailable": "Storymaker did not answer.",
+  "config.storymaker.save_first": "Save the configuration first, then check.",
+  "config.storymaker.ready": "Ready: FFmpeg and a browser are available.",
+  "config.storymaker.missing": "Export is not ready yet. Missing: {missing}.",
+  "config.storymaker.disabled": "Storymaker is switched off.",
+  "config.storymaker.part_ffmpeg": "FFmpeg",
+  "config.storymaker.part_browser": "browser",
   "config.refresh.load_error": "This section could not be loaded.",
   "config.refresh.retry": "Try again",
   "config.refresh.toggle_password": "Show or hide password",
```

`ui/lang/config/common/es.json` (after base line 33, the last `config.retro_games.*` key):

```diff
--- a/ui/lang/config/common/es.json
+++ b/ui/lang/config/common/es.json
@@ -31,6 +31,27 @@
   "config.retro_games.status_building": "El índice se está creando ahora mismo",
   "config.retro_games.unavailable": "Retro Games no está disponible",
   "config.retro_games.save_first": "Guarda primero los ajustes",
+  "config.storymaker.title": "Storymaker",
+  "config.storymaker.help": "Estudio de vídeos explicativos en el escritorio. Las historias se editan en la aplicación Storymaker y se exportan como archivos MP4. La exportación necesita FFmpeg y un navegador Chrome o Chromium en el servidor; la edición y la vista previa funcionan sin ellos.",
+  "config.storymaker.enabled": "Activar Storymaker",
+  "config.storymaker.timeout": "Límite de tiempo de exportación (segundos)",
+  "config.storymaker.timeout_help": "Una exportación que dure más se detiene. Los vídeos más largos reciben más tiempo automáticamente. Entre 60 y 14400.",
+  "config.storymaker.max_duration": "Vídeo más largo (segundos)",
+  "config.storymaker.max_duration_help": "Las historias que darían un vídeo más largo no se exportan. Entre 10 y 3600.",
+  "config.storymaker.browser_path": "Ruta del navegador (opcional)",
+  "config.storymaker.browser_path_help": "Ruta absoluta de Chrome o Chromium, que se usa para dibujar los fotogramas. Vacío significa el navegador instalado en este sistema. En Docker, indica la ruta de un navegador dentro del contenedor.",
+  "config.storymaker.transport": "Transferencia de fotogramas",
+  "config.storymaker.transport_help": "Cómo llegan a FFmpeg los fotogramas dibujados. Deja Automático salvo que el soporte te pida cambiarlo.",
+  "config.storymaker.transport_auto": "Automático",
+  "config.storymaker.check": "Comprobar la configuración de exportación",
+  "config.storymaker.checking": "Comprobando…",
+  "config.storymaker.unavailable": "Storymaker no respondió.",
+  "config.storymaker.save_first": "Guarda primero la configuración y luego comprueba.",
+  "config.storymaker.ready": "Listo: FFmpeg y un navegador están disponibles.",
+  "config.storymaker.missing": "La exportación aún no está lista. Falta: {missing}.",
+  "config.storymaker.disabled": "Storymaker está desactivado.",
+  "config.storymaker.part_ffmpeg": "FFmpeg",
+  "config.storymaker.part_browser": "navegador",
   "config.refresh.load_error": "No se pudo cargar esta sección.",
   "config.refresh.retry": "Volver a intentar",
   "config.refresh.toggle_password": "Mostrar u ocultar contraseña",
```

`ui/lang/config/common/fr.json` (after base line 33, the last `config.retro_games.*` key):

```diff
--- a/ui/lang/config/common/fr.json
+++ b/ui/lang/config/common/fr.json
@@ -31,6 +31,27 @@
   "config.retro_games.status_building": "L'index est en cours de création",
   "config.retro_games.unavailable": "Retro Games n'est pas disponible",
   "config.retro_games.save_first": "Enregistre d'abord les paramètres",
+  "config.storymaker.title": "Storymaker",
+  "config.storymaker.help": "Studio de vidéos explicatives sur le bureau. Les histoires se modifient dans l'application Storymaker et s'exportent en fichiers MP4. L'export a besoin de FFmpeg et d'un navigateur Chrome ou Chromium sur le serveur ; la modification et l'aperçu fonctionnent sans eux.",
+  "config.storymaker.enabled": "Activer Storymaker",
+  "config.storymaker.timeout": "Durée maximale d'un export (secondes)",
+  "config.storymaker.timeout_help": "Un export plus long est arrêté. Les vidéos plus longues reçoivent automatiquement plus de temps. Entre 60 et 14400.",
+  "config.storymaker.max_duration": "Vidéo la plus longue (secondes)",
+  "config.storymaker.max_duration_help": "Les histoires qui donneraient une vidéo plus longue ne sont pas exportées. Entre 10 et 3600.",
+  "config.storymaker.browser_path": "Chemin du navigateur (facultatif)",
+  "config.storymaker.browser_path_help": "Chemin absolu de Chrome ou Chromium, utilisé pour dessiner les images. Vide : le navigateur installé sur ce système. Dans Docker, indique le chemin d'un navigateur dans le conteneur.",
+  "config.storymaker.transport": "Transfert des images",
+  "config.storymaker.transport_help": "Comment les images dessinées parviennent à FFmpeg. Garde Automatique, sauf si le support te demande de changer.",
+  "config.storymaker.transport_auto": "Automatique",
+  "config.storymaker.check": "Vérifier la configuration de l'export",
+  "config.storymaker.checking": "Vérification…",
+  "config.storymaker.unavailable": "Storymaker n'a pas répondu.",
+  "config.storymaker.save_first": "Enregistre d'abord la configuration, puis vérifie.",
+  "config.storymaker.ready": "Prêt : FFmpeg et un navigateur sont disponibles.",
+  "config.storymaker.missing": "L'export n'est pas encore prêt. Manque : {missing}.",
+  "config.storymaker.disabled": "Storymaker est désactivé.",
+  "config.storymaker.part_ffmpeg": "FFmpeg",
+  "config.storymaker.part_browser": "navigateur",
   "config.refresh.load_error": "Impossible de charger cette section.",
   "config.refresh.retry": "Réessayer",
   "config.refresh.toggle_password": "Afficher ou masquer le mot de passe",
```

`ui/lang/config/common/hi.json` (after base line 33, the last `config.retro_games.*` key):

```diff
--- a/ui/lang/config/common/hi.json
+++ b/ui/lang/config/common/hi.json
@@ -31,6 +31,27 @@
   "config.retro_games.status_building": "इंडेक्स अभी बन रहा है",
   "config.retro_games.unavailable": "Retro Games उपलब्ध नहीं है",
   "config.retro_games.save_first": "पहले सेटिंग्स सहेजें",
+  "config.storymaker.title": "Storymaker",
+  "config.storymaker.help": "डेस्कटॉप पर समझाने वाले वीडियो का स्टूडियो। कहानियाँ Storymaker ऐप में संपादित होती हैं और MP4 फ़ाइलों के रूप में एक्सपोर्ट होती हैं। एक्सपोर्ट के लिए सर्वर पर FFmpeg और Chrome या Chromium ब्राउज़र चाहिए; संपादन और प्रीव्यू इनके बिना भी काम करते हैं।",
+  "config.storymaker.enabled": "Storymaker चालू करें",
+  "config.storymaker.timeout": "एक्सपोर्ट की समय-सीमा (सेकंड)",
+  "config.storymaker.timeout_help": "इससे ज़्यादा चलने वाला एक्सपोर्ट रोक दिया जाता है। लंबे वीडियो को अपने आप ज़्यादा समय मिलता है। 60 से 14400 के बीच।",
+  "config.storymaker.max_duration": "सबसे लंबा वीडियो (सेकंड)",
+  "config.storymaker.max_duration_help": "जिन कहानियों से इससे लंबा वीडियो बनेगा, वे एक्सपोर्ट नहीं होतीं। 10 से 3600 के बीच।",
+  "config.storymaker.browser_path": "ब्राउज़र का पथ (वैकल्पिक)",
+  "config.storymaker.browser_path_help": "Chrome या Chromium का पूरा पथ, जो फ़्रेम बनाता है। खाली का मतलब इस सिस्टम पर इंस्टॉल किया हुआ ब्राउज़र है। Docker में कंटेनर के अंदर के किसी ब्राउज़र का पथ दें।",
+  "config.storymaker.transport": "फ़्रेम स्थानांतरण",
+  "config.storymaker.transport_help": "बनाए गए फ़्रेम FFmpeg तक कैसे पहुँचते हैं। स्वचालित ही रखें, जब तक सहायता टीम बदलने को न कहे।",
+  "config.storymaker.transport_auto": "स्वचालित",
+  "config.storymaker.check": "एक्सपोर्ट सेटअप जाँचें",
+  "config.storymaker.checking": "जाँच हो रही है…",
+  "config.storymaker.unavailable": "Storymaker ने जवाब नहीं दिया।",
+  "config.storymaker.save_first": "पहले कॉन्फ़िगरेशन सहेजें, फिर जाँचें।",
+  "config.storymaker.ready": "तैयार: FFmpeg और एक ब्राउज़र उपलब्ध हैं।",
+  "config.storymaker.missing": "एक्सपोर्ट अभी तैयार नहीं है। कमी: {missing}।",
+  "config.storymaker.disabled": "Storymaker बंद है।",
+  "config.storymaker.part_ffmpeg": "FFmpeg",
+  "config.storymaker.part_browser": "ब्राउज़र",
   "config.refresh.load_error": "यह अनुभाग लोड नहीं हो सका।",
   "config.refresh.retry": "फिर कोशिश करें",
   "config.refresh.toggle_password": "पासवर्ड दिखाएँ या छिपाएँ",
```

`ui/lang/config/common/it.json` (after base line 33, the last `config.retro_games.*` key):

```diff
--- a/ui/lang/config/common/it.json
+++ b/ui/lang/config/common/it.json
@@ -31,6 +31,27 @@
   "config.retro_games.status_building": "L'indice è in fase di creazione",
   "config.retro_games.unavailable": "Retro Games non è disponibile",
   "config.retro_games.save_first": "Salva prima le impostazioni",
+  "config.storymaker.title": "Storymaker",
+  "config.storymaker.help": "Studio di video esplicativi sul desktop. Le storie si modificano nell'app Storymaker e si esportano come file MP4. L'esportazione richiede FFmpeg e un browser Chrome o Chromium sul server; modifica e anteprima funzionano anche senza.",
+  "config.storymaker.enabled": "Attiva Storymaker",
+  "config.storymaker.timeout": "Limite di tempo dell'esportazione (secondi)",
+  "config.storymaker.timeout_help": "Un'esportazione che dura di più viene interrotta. I video più lunghi ricevono automaticamente più tempo. Tra 60 e 14400.",
+  "config.storymaker.max_duration": "Video più lungo (secondi)",
+  "config.storymaker.max_duration_help": "Le storie che darebbero un video più lungo non vengono esportate. Tra 10 e 3600.",
+  "config.storymaker.browser_path": "Percorso del browser (facoltativo)",
+  "config.storymaker.browser_path_help": "Percorso assoluto di Chrome o Chromium, usato per disegnare i fotogrammi. Vuoto significa il browser installato su questo sistema. In Docker, indica il percorso di un browser nel container.",
+  "config.storymaker.transport": "Trasferimento dei fotogrammi",
+  "config.storymaker.transport_help": "Come i fotogrammi disegnati arrivano a FFmpeg. Lascia Automatico, a meno che l'assistenza non ti chieda di cambiarlo.",
+  "config.storymaker.transport_auto": "Automatico",
+  "config.storymaker.check": "Controlla la configurazione dell'esportazione",
+  "config.storymaker.checking": "Controllo in corso…",
+  "config.storymaker.unavailable": "Storymaker non ha risposto.",
+  "config.storymaker.save_first": "Salva prima la configurazione, poi controlla.",
+  "config.storymaker.ready": "Pronto: FFmpeg e un browser sono disponibili.",
+  "config.storymaker.missing": "L'esportazione non è ancora pronta. Manca: {missing}.",
+  "config.storymaker.disabled": "Storymaker è disattivato.",
+  "config.storymaker.part_ffmpeg": "FFmpeg",
+  "config.storymaker.part_browser": "browser",
   "config.refresh.load_error": "Impossibile caricare questa sezione.",
   "config.refresh.retry": "Riprova",
   "config.refresh.toggle_password": "Mostra o nascondi password",
```

`ui/lang/config/common/ja.json` (after base line 33, the last `config.retro_games.*` key):

```diff
--- a/ui/lang/config/common/ja.json
+++ b/ui/lang/config/common/ja.json
@@ -31,6 +31,27 @@
   "config.retro_games.status_building": "インデックスを作成しています",
   "config.retro_games.unavailable": "Retro Gamesは利用できません",
   "config.retro_games.save_first": "先に設定を保存してください",
+  "config.storymaker.title": "Storymaker",
+  "config.storymaker.help": "デスクトップの解説動画スタジオです。ストーリーは Storymaker アプリで編集し、MP4 ファイルとして書き出します。書き出しにはサーバーに FFmpeg と Chrome または Chromium が必要です。編集とプレビューはなくても使えます。",
+  "config.storymaker.enabled": "Storymaker を有効にする",
+  "config.storymaker.timeout": "書き出しの制限時間（秒）",
+  "config.storymaker.timeout_help": "これより長くかかる書き出しは停止されます。長い動画には自動的により多くの時間が与えられます。60〜14400。",
+  "config.storymaker.max_duration": "最長の動画（秒）",
+  "config.storymaker.max_duration_help": "これより長い動画になるストーリーは書き出されません。10〜3600。",
+  "config.storymaker.browser_path": "ブラウザのパス（任意）",
+  "config.storymaker.browser_path_help": "フレームを描画する Chrome または Chromium の絶対パス。空欄の場合は、このシステムにインストールされているブラウザを使います。Docker ではコンテナ内のブラウザのパスを指定します。",
+  "config.storymaker.transport": "フレームの受け渡し",
+  "config.storymaker.transport_help": "描画したフレームを FFmpeg に渡す方法です。サポートから依頼されない限り「自動」のままにしてください。",
+  "config.storymaker.transport_auto": "自動",
+  "config.storymaker.check": "書き出しの設定を確認",
+  "config.storymaker.checking": "確認中…",
+  "config.storymaker.unavailable": "Storymaker が応答しませんでした。",
+  "config.storymaker.save_first": "先に設定を保存してから確認してください。",
+  "config.storymaker.ready": "準備完了：FFmpeg とブラウザが利用できます。",
+  "config.storymaker.missing": "書き出しの準備ができていません。不足：{missing}。",
+  "config.storymaker.disabled": "Storymaker はオフになっています。",
+  "config.storymaker.part_ffmpeg": "FFmpeg",
+  "config.storymaker.part_browser": "ブラウザ",
   "config.refresh.load_error": "このセクションを読み込めませんでした。",
   "config.refresh.retry": "再試行",
   "config.refresh.toggle_password": "パスワードを表示または非表示",
```

`ui/lang/config/common/nl.json` (after base line 33, the last `config.retro_games.*` key):

```diff
--- a/ui/lang/config/common/nl.json
+++ b/ui/lang/config/common/nl.json
@@ -31,6 +31,27 @@
   "config.retro_games.status_building": "De index wordt op dit moment opgebouwd",
   "config.retro_games.unavailable": "Retro Games is niet beschikbaar",
   "config.retro_games.save_first": "Sla eerst je instellingen op",
+  "config.storymaker.title": "Storymaker",
+  "config.storymaker.help": "Uitlegvideo-studio op het bureaublad. Verhalen bewerk je in de app Storymaker en exporteer je als MP4-bestanden. Exporteren heeft FFmpeg en een Chrome- of Chromium-browser op de server nodig; bewerken en het voorbeeld werken ook zonder.",
+  "config.storymaker.enabled": "Storymaker inschakelen",
+  "config.storymaker.timeout": "Tijdslimiet voor exporteren (seconden)",
+  "config.storymaker.timeout_help": "Een export die langer duurt, wordt gestopt. Langere video's krijgen automatisch meer tijd. Tussen 60 en 14400.",
+  "config.storymaker.max_duration": "Langste video (seconden)",
+  "config.storymaker.max_duration_help": "Verhalen die een langere video zouden geven, worden niet geëxporteerd. Tussen 10 en 3600.",
+  "config.storymaker.browser_path": "Browserpad (optioneel)",
+  "config.storymaker.browser_path_help": "Absoluut pad van Chrome of Chromium, gebruikt om de beelden te tekenen. Leeg betekent de browser die op dit systeem is geïnstalleerd. Geef in Docker het pad van een browser in de container op.",
+  "config.storymaker.transport": "Beeldoverdracht",
+  "config.storymaker.transport_help": "Hoe de getekende beelden bij FFmpeg komen. Laat Automatisch staan, tenzij de support vraagt het te wijzigen.",
+  "config.storymaker.transport_auto": "Automatisch",
+  "config.storymaker.check": "Exportinstelling controleren",
+  "config.storymaker.checking": "Controleren…",
+  "config.storymaker.unavailable": "Storymaker antwoordde niet.",
+  "config.storymaker.save_first": "Sla eerst de configuratie op en controleer dan.",
+  "config.storymaker.ready": "Gereed: FFmpeg en een browser zijn beschikbaar.",
+  "config.storymaker.missing": "Exporteren is nog niet gereed. Ontbreekt: {missing}.",
+  "config.storymaker.disabled": "Storymaker staat uit.",
+  "config.storymaker.part_ffmpeg": "FFmpeg",
+  "config.storymaker.part_browser": "browser",
   "config.refresh.load_error": "Dit onderdeel kon niet worden geladen.",
   "config.refresh.retry": "Opnieuw proberen",
   "config.refresh.toggle_password": "Wachtwoord tonen of verbergen",
```

`ui/lang/config/common/no.json` (after base line 33, the last `config.retro_games.*` key):

```diff
--- a/ui/lang/config/common/no.json
+++ b/ui/lang/config/common/no.json
@@ -31,6 +31,27 @@
   "config.retro_games.status_building": "Indeksen bygges akkurat nå",
   "config.retro_games.unavailable": "Retro Games er ikke tilgjengelig",
   "config.retro_games.save_first": "Lagre innstillingene først",
+  "config.storymaker.title": "Storymaker",
+  "config.storymaker.help": "Studio for forklaringsvideoer på skrivebordet. Historier redigeres i appen Storymaker og eksporteres som MP4-filer. Eksport krever FFmpeg og en Chrome- eller Chromium-nettleser på serveren; redigering og forhåndsvisning fungerer uten dem.",
+  "config.storymaker.enabled": "Slå på Storymaker",
+  "config.storymaker.timeout": "Tidsgrense for eksport (sekunder)",
+  "config.storymaker.timeout_help": "En eksport som varer lenger, stoppes. Lengre videoer får automatisk mer tid. Mellom 60 og 14400.",
+  "config.storymaker.max_duration": "Lengste video (sekunder)",
+  "config.storymaker.max_duration_help": "Historier som ville gitt en lengre video, eksporteres ikke. Mellom 10 og 3600.",
+  "config.storymaker.browser_path": "Nettleserens sti (valgfritt)",
+  "config.storymaker.browser_path_help": "Absolutt sti til Chrome eller Chromium, som tegner bildene. Tomt betyr nettleseren som er installert på dette systemet. I Docker oppgir du stien til en nettleser i containeren.",
+  "config.storymaker.transport": "Overføring av bilder",
+  "config.storymaker.transport_help": "Hvordan de tegnede bildene kommer til FFmpeg. Behold Automatisk med mindre brukerstøtten ber deg endre det.",
+  "config.storymaker.transport_auto": "Automatisk",
+  "config.storymaker.check": "Sjekk eksportoppsettet",
+  "config.storymaker.checking": "Sjekker …",
+  "config.storymaker.unavailable": "Storymaker svarte ikke.",
+  "config.storymaker.save_first": "Lagre konfigurasjonen først, og sjekk deretter.",
+  "config.storymaker.ready": "Klar: FFmpeg og en nettleser er tilgjengelig.",
+  "config.storymaker.missing": "Eksporten er ikke klar ennå. Mangler: {missing}.",
+  "config.storymaker.disabled": "Storymaker er slått av.",
+  "config.storymaker.part_ffmpeg": "FFmpeg",
+  "config.storymaker.part_browser": "nettleser",
   "config.refresh.load_error": "Denne delen kunne ikke lastes inn.",
   "config.refresh.retry": "Prøv igjen",
   "config.refresh.toggle_password": "Vis eller skjul passord",
```

`ui/lang/config/common/pl.json` (after base line 33, the last `config.retro_games.*` key):

```diff
--- a/ui/lang/config/common/pl.json
+++ b/ui/lang/config/common/pl.json
@@ -31,6 +31,27 @@
   "config.retro_games.status_building": "Indeks jest właśnie tworzony",
   "config.retro_games.unavailable": "Retro Games jest niedostępne",
   "config.retro_games.save_first": "Najpierw zapisz ustawienia",
+  "config.storymaker.title": "Storymaker",
+  "config.storymaker.help": "Studio filmów wyjaśniających na pulpicie. Historie edytuje się w aplikacji Storymaker i eksportuje jako pliki MP4. Eksport wymaga FFmpeg i przeglądarki Chrome lub Chromium na serwerze; edycja i podgląd działają bez nich.",
+  "config.storymaker.enabled": "Włącz Storymaker",
+  "config.storymaker.timeout": "Limit czasu eksportu (sekundy)",
+  "config.storymaker.timeout_help": "Eksport trwający dłużej jest zatrzymywany. Dłuższe filmy automatycznie dostają więcej czasu. Od 60 do 14400.",
+  "config.storymaker.max_duration": "Najdłuższy film (sekundy)",
+  "config.storymaker.max_duration_help": "Historie, z których powstałby dłuższy film, nie są eksportowane. Od 10 do 3600.",
+  "config.storymaker.browser_path": "Ścieżka przeglądarki (opcjonalnie)",
+  "config.storymaker.browser_path_help": "Bezwzględna ścieżka Chrome lub Chromium, który rysuje klatki. Puste oznacza przeglądarkę zainstalowaną w tym systemie. W Dockerze podaj ścieżkę przeglądarki wewnątrz kontenera.",
+  "config.storymaker.transport": "Przekazywanie klatek",
+  "config.storymaker.transport_help": "Jak narysowane klatki trafiają do FFmpeg. Zostaw Automatycznie, chyba że pomoc techniczna poprosi o zmianę.",
+  "config.storymaker.transport_auto": "Automatycznie",
+  "config.storymaker.check": "Sprawdź ustawienia eksportu",
+  "config.storymaker.checking": "Sprawdzanie…",
+  "config.storymaker.unavailable": "Storymaker nie odpowiedział.",
+  "config.storymaker.save_first": "Najpierw zapisz konfigurację, potem sprawdź.",
+  "config.storymaker.ready": "Gotowe: FFmpeg i przeglądarka są dostępne.",
+  "config.storymaker.missing": "Eksport nie jest jeszcze gotowy. Brakuje: {missing}.",
+  "config.storymaker.disabled": "Storymaker jest wyłączony.",
+  "config.storymaker.part_ffmpeg": "FFmpeg",
+  "config.storymaker.part_browser": "przeglądarki",
   "config.refresh.load_error": "Nie udało się załadować tej sekcji.",
   "config.refresh.retry": "Spróbuj ponownie",
   "config.refresh.toggle_password": "Pokaż lub ukryj hasło",
```

`ui/lang/config/common/pt.json` (after base line 33, the last `config.retro_games.*` key):

```diff
--- a/ui/lang/config/common/pt.json
+++ b/ui/lang/config/common/pt.json
@@ -31,6 +31,27 @@
   "config.retro_games.status_building": "O índice está a ser criado neste momento",
   "config.retro_games.unavailable": "O Retro Games não está disponível",
   "config.retro_games.save_first": "Guarda primeiro as definições",
+  "config.storymaker.title": "Storymaker",
+  "config.storymaker.help": "Estúdio de vídeos explicativos no ambiente de trabalho. As histórias editam-se na aplicação Storymaker e exportam-se como ficheiros MP4. A exportação precisa do FFmpeg e de um navegador Chrome ou Chromium no servidor; a edição e a pré-visualização funcionam sem eles.",
+  "config.storymaker.enabled": "Ativar o Storymaker",
+  "config.storymaker.timeout": "Limite de tempo da exportação (segundos)",
+  "config.storymaker.timeout_help": "Uma exportação que demore mais é interrompida. Os vídeos mais longos recebem automaticamente mais tempo. Entre 60 e 14400.",
+  "config.storymaker.max_duration": "Vídeo mais longo (segundos)",
+  "config.storymaker.max_duration_help": "As histórias que dariam um vídeo mais longo não são exportadas. Entre 10 e 3600.",
+  "config.storymaker.browser_path": "Caminho do navegador (opcional)",
+  "config.storymaker.browser_path_help": "Caminho absoluto do Chrome ou Chromium, usado para desenhar as imagens. Vazio significa o navegador instalado neste sistema. No Docker, indica o caminho de um navegador dentro do contentor.",
+  "config.storymaker.transport": "Transferência de imagens",
+  "config.storymaker.transport_help": "Como as imagens desenhadas chegam ao FFmpeg. Mantém Automático, a menos que o suporte te peça para mudar.",
+  "config.storymaker.transport_auto": "Automático",
+  "config.storymaker.check": "Verificar a configuração de exportação",
+  "config.storymaker.checking": "A verificar…",
+  "config.storymaker.unavailable": "O Storymaker não respondeu.",
+  "config.storymaker.save_first": "Guarda primeiro a configuração e depois verifica.",
+  "config.storymaker.ready": "Pronto: o FFmpeg e um navegador estão disponíveis.",
+  "config.storymaker.missing": "A exportação ainda não está pronta. Falta: {missing}.",
+  "config.storymaker.disabled": "O Storymaker está desativado.",
+  "config.storymaker.part_ffmpeg": "FFmpeg",
+  "config.storymaker.part_browser": "navegador",
   "config.refresh.load_error": "Não foi possível carregar esta seção.",
   "config.refresh.retry": "Tentar novamente",
   "config.refresh.toggle_password": "Mostrar ou ocultar palavra-passe",
```

`ui/lang/config/common/sv.json` (after base line 33, the last `config.retro_games.*` key):

```diff
--- a/ui/lang/config/common/sv.json
+++ b/ui/lang/config/common/sv.json
@@ -31,6 +31,27 @@
   "config.retro_games.status_building": "Indexet byggs just nu",
   "config.retro_games.unavailable": "Retro Games är inte tillgängligt",
   "config.retro_games.save_first": "Spara inställningarna först",
+  "config.storymaker.title": "Storymaker",
+  "config.storymaker.help": "Studio för förklarande filmer på skrivbordet. Berättelser redigeras i appen Storymaker och exporteras som MP4-filer. Export kräver FFmpeg och en Chrome- eller Chromium-webbläsare på servern; redigering och förhandsvisning fungerar utan dem.",
+  "config.storymaker.enabled": "Aktivera Storymaker",
+  "config.storymaker.timeout": "Tidsgräns för export (sekunder)",
+  "config.storymaker.timeout_help": "En export som tar längre tid stoppas. Längre filmer får automatiskt mer tid. Mellan 60 och 14400.",
+  "config.storymaker.max_duration": "Längsta film (sekunder)",
+  "config.storymaker.max_duration_help": "Berättelser som skulle bli en längre film exporteras inte. Mellan 10 och 3600.",
+  "config.storymaker.browser_path": "Webbläsarens sökväg (valfritt)",
+  "config.storymaker.browser_path_help": "Absolut sökväg till Chrome eller Chromium, som ritar bildrutorna. Tomt betyder webbläsaren som är installerad på det här systemet. I Docker anger du sökvägen till en webbläsare i containern.",
+  "config.storymaker.transport": "Överföring av bildrutor",
+  "config.storymaker.transport_help": "Hur de ritade bildrutorna når FFmpeg. Behåll Automatisk om inte supporten ber dig ändra.",
+  "config.storymaker.transport_auto": "Automatisk",
+  "config.storymaker.check": "Kontrollera exportinställningen",
+  "config.storymaker.checking": "Kontrollerar …",
+  "config.storymaker.unavailable": "Storymaker svarade inte.",
+  "config.storymaker.save_first": "Spara konfigurationen först och kontrollera sedan.",
+  "config.storymaker.ready": "Redo: FFmpeg och en webbläsare finns.",
+  "config.storymaker.missing": "Exporten är inte redo än. Saknas: {missing}.",
+  "config.storymaker.disabled": "Storymaker är avstängt.",
+  "config.storymaker.part_ffmpeg": "FFmpeg",
+  "config.storymaker.part_browser": "webbläsare",
   "config.refresh.load_error": "Det gick inte att läsa in det här avsnittet.",
   "config.refresh.retry": "Försök igen",
   "config.refresh.toggle_password": "Visa eller dölj lösenord",
```

`ui/lang/config/common/zh.json` (after base line 33, the last `config.retro_games.*` key):

```diff
--- a/ui/lang/config/common/zh.json
+++ b/ui/lang/config/common/zh.json
@@ -31,6 +31,27 @@
   "config.retro_games.status_building": "正在创建索引",
   "config.retro_games.unavailable": "Retro Games不可用",
   "config.retro_games.save_first": "请先保存设置",
+  "config.storymaker.title": "Storymaker",
+  "config.storymaker.help": "桌面上的讲解视频工作室。故事在 Storymaker 应用中编辑，并导出为 MP4 文件。导出需要服务器上有 FFmpeg 和 Chrome 或 Chromium 浏览器；没有它们时，编辑和预览仍可使用。",
+  "config.storymaker.enabled": "启用 Storymaker",
+  "config.storymaker.timeout": "导出时间上限（秒）",
+  "config.storymaker.timeout_help": "运行时间更长的导出会被停止。更长的视频会自动获得更多时间。范围 60 到 14400。",
+  "config.storymaker.max_duration": "最长视频（秒）",
+  "config.storymaker.max_duration_help": "会生成更长视频的故事不会被导出。范围 10 到 3600。",
+  "config.storymaker.browser_path": "浏览器路径（可选）",
+  "config.storymaker.browser_path_help": "用于绘制画面的 Chrome 或 Chromium 的绝对路径。留空表示使用此系统上已安装的浏览器。在 Docker 中，请填写容器内浏览器的路径。",
+  "config.storymaker.transport": "帧传输方式",
+  "config.storymaker.transport_help": "绘制好的画面如何传给 FFmpeg。除非技术支持要求更改，否则请保持“自动”。",
+  "config.storymaker.transport_auto": "自动",
+  "config.storymaker.check": "检查导出设置",
+  "config.storymaker.checking": "正在检查…",
+  "config.storymaker.unavailable": "Storymaker 没有响应。",
+  "config.storymaker.save_first": "请先保存配置，然后再检查。",
+  "config.storymaker.ready": "就绪：FFmpeg 和浏览器均可用。",
+  "config.storymaker.missing": "导出尚未就绪。缺少：{missing}。",
+  "config.storymaker.disabled": "Storymaker 已关闭。",
+  "config.storymaker.part_ffmpeg": "FFmpeg",
+  "config.storymaker.part_browser": "浏览器",
   "config.refresh.load_error": "无法加载此部分。",
   "config.refresh.retry": "重试",
   "config.refresh.toggle_password": "显示或隐藏密码",
```

`ui/config_wave1_batch4_test.go` (base lines 114-117):

```diff
--- a/ui/config_wave1_batch4_test.go
+++ b/ui/config_wave1_batch4_test.go
@@ -111,10 +111,10 @@ func TestConfigWave1Batch4SecretsAndRuntimeActions(t *testing.T) {
 	}
 
 	desktop := readWave0File(t, "cfg/virtual_desktop.js")
-	if got := strings.Count(desktop, `data-config-action-draft="false"`); got != 3 {
-		t.Errorf("virtual_desktop: want 3 runtime-only status checks, found %d", got)
+	if got := strings.Count(desktop, `data-config-action-draft="false"`); got != 4 {
+		t.Errorf("virtual_desktop: want 4 runtime-only status checks, found %d", got)
 	}
-	for _, want := range []string{"cfgActionResult.html('vd-cfg-result')", "cfgActionResult.html('vd-video-studio-result')", "cfgActionResult.html('vd-retro-games-result')"} {
+	for _, want := range []string{"cfgActionResult.html('vd-cfg-result')", "cfgActionResult.html('vd-video-studio-result')", "cfgActionResult.html('vd-retro-games-result')", "cfgActionResult.html('vd-storymaker-result')"} {
 		if !strings.Contains(desktop, want) {
 			t.Errorf("virtual_desktop missing %q", want)
 		}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestConfigStorymaker|TestConfigWave1Batch4SecretsAndRuntimeActions|TestConfigDerivedValidationRulesCoverTemplate' -count=1`
Expected: ok (the config browser test runs)

Browser tests run with `AURAGO_RUN_BROWSER_SMOKE=1` (the installed Chrome; on the Windows dev box through `go test` with the sandbox disabled).

Recorded at drafting time:

```text
ok  	aurago/ui	3.526s
```

- [ ] **Step 5: Commit**

```bash
git add ui/config_storymaker_test.go ui/cfg/virtual_desktop.js ui/lang/config/common ui/config_wave1_batch4_test.go
git commit -m "feat(storymaker): configuration section for the Storymaker app" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E10: Documentation

The Storymaker section of the apps contract, the root child row, the `internal/desktop` bullet, the player contract `ui/js/storymaker/AGENTS.md` (the player task delegated it here), a user page `documentation/storymaker.md` and the app rows in the Web UI manual (en, de). Documentation has no failing test; the check below only proves that every path the documents name exists.

**Files:**
- Create: `ui/js/storymaker/AGENTS.md`
- Create: `documentation/storymaker.md`
- Modify: `ui/js/desktop/apps/AGENTS.md` (after line 43, before `## Email exports`)
- Modify: `AGENTS.md` (after base line 202, the ECalc row of the Child DOX index)
- Modify: `internal/desktop/AGENTS.md` (after base line 16, the ECalc bullet)
- Modify: `documentation/manuals/en/04-webui.md` (after base line 267)
- Modify: `documentation/manuals/de/04-webui.md` (after base line 266)

- [ ] **Step 1: No test**

This task is documentation; Step 2 implements it and the check proves that the named paths exist.

- [ ] **Step 2: Implement**

`ui/js/storymaker/AGENTS.md`:

```markdown
# Storymaker Player Runtime - Child DOX Contract

The player draws a story in the browser: the app preview (a sandboxed iframe), the headless render page of the export and the
fallback in the host realm all run the same code. The app side is `../desktop/apps/AGENTS.md` (Storymaker), the story format and
catalog index are `../../../internal/storymaker/AGENTS.md`, the generated catalog is `../../img/storymaker/` (`assets/storymaker`).

## Layout and build

- Classic scripts, never modules, outside `ui/js/desktop`. `host.bundle.js` (document realm: host, app protocol, render driver,
  deadlines) and `player.bundle.js` (worker realm: compile, render, rig, code-scene API, role handlers) are generated by
  `npm run build:ui` from the part lists in `scripts/storymaker-bundle-parts.js`; never edit a bundle by hand, `npm run check:ui`
  fails when one is stale. Every part keeps its own IIFE scope; the shared parts (`schedule.js`) sit in both bundles.
- The player CSS `../../css/storymaker-player.css` is the only stylesheet of the player host page and is named in its CSP.
- Workers are blob workers: one prelude line (`self.__SM_ROLE__ = "player" | "scene"; importScripts("<base>/js/storymaker/player.bundle.js");`)
  plus, for a scene worker, the scene bundle text. A line number inside a worker minus one is a line of the scene bundle.

## Rules

- No `innerHTML`, `insertAdjacentHTML`, template string with data, `eval` or `Function` anywhere under `ui/js/storymaker`
  (`TestStorymakerSourcesAreSafeAndSmall` and `scripts/storymaker-tests/10-static-guards.mjs`). Story text is drawn on a canvas.
- Determinism: time is the frame number; no `Date`, `Math.random` or wall-clock reads in a render path (`determinism.js` shadows them
  in the code-scene realm); the same story twice renders the same frames. Do not pin pixel hashes of one version in tests.
- Deadlines: scene build 2 s, frame 250 ms (the first three frames after a load 4 s), compile 10 s, assets 20 s. A worker that
  overruns is terminated and reported (`deadline`); a code-scene frame error kills only that scene's worker and shows a placeholder.
- The app protocol (contract section 8) accepts only messages from `window.parent`, in a fixed schema with caps, and posts to the
  origin of `data-sm-base`. `load` resets everything; the first `time` message after a compile is the "ready to play" signal.
  The render page (`data-sm-mode="render"`) fetches only from its own base and posts frames in strict order (contract section 7).
- Support matrix of phase 1: beats enter exit walk move pose gesture emote look show (item and text) hide camera wait; transitions
  cut fade slide-left wipe (others fall back to fade with a `not_supported_yet` note); everything else the validator accepts is ignored.

## Verify

- `npm run test:storymaker` (Node, vm based, about 8 s), `npm run build:ui`, `npm run check:ui`.
- Browser (`AURAGO_RUN_BROWSER_SMOKE=1`, installed Chrome): `go test ./ui -run 'TestStorymaker(PlayerGate|RenderMode|FirewallRender|RenderScreenshot)Browser' -count=1`
  for the opaque-origin gate (workers, OffscreenCanvas, FontFace, ImageBitmap transfer, blocked fetch and importScripts) and the render
  page; `TestStorymakerApp*` for the app tour against the real player.
```

`documentation/storymaker.md`:

````markdown
# Storymaker

Storymaker is the Virtual Desktop app for short explainer videos. You stage scenes with cartoon people, props and
environments from a built-in catalog, write the narration text, watch a live preview and export an MP4. This page describes
the first version (phase 1): the editor, the preview and a silent export. Voices, music, captions and the agent come in later
phases.

## Switch it on

Storymaker is on by default. **Configuration → Virtual Desktop → Storymaker** holds its settings:

| Setting | Default | Meaning |
| --- | --- | --- |
| `storymaker.enabled` | `true` | Shows the app and answers its API. Off: the app is hidden and the API answers `storymaker_disabled`. |
| `storymaker.render.timeout_seconds` | `3600` | An export that runs longer is stopped (60 to 14400). Longer videos get more time automatically. |
| `storymaker.render.max_duration_seconds` | `900` | The longest video that can be exported (10 to 3600). |
| `storymaker.render.browser_path` | empty | Absolute path of Chrome or Chromium. Empty: the browser installed on the system, else a pinned download. |
| `storymaker.render.transport` | empty | How frames reach FFmpeg (`png`, `webp`, `raw`, `screenshot`). Empty is the measured default; change it only on advice. |

The **Check export setup** button asks the running server whether FFmpeg and a browser are ready. The app is read-only while
`virtual_desktop.readonly` is on: you can watch the preview, but nothing is edited, saved or exported.

## Projects

Every project is a folder in the desktop workspace, `Documents/Storymaker/<project id>/`:

```
story.json        the story document (the editor saves it with If-Match, so two windows cannot overwrite each other)
project.json      title, format and timestamps
scenes/<id>.js    code scenes (and <id>.bundle.js, the bundled copy the player runs)
exports/          the exported videos and their records
```

Delete a project in the list: the whole folder, exports included, goes away.

## The editor

- **Scenes (left).** One card per scene with a thumbnail the player draws, its first words and its duration. Add a scene or a
  code scene; move or delete a scene in the inspector.
- **Stage and timeline (centre).** The preview runs in a sandboxed frame. The timeline has one block per scene (code scenes are
  hatched); click or drag it, or use the arrow keys (Shift moves a second). Play and pause start the real-time preview.
- **Inspector (right).** Environment, time of day, narration, the transition into the scene and its duration (automatic is as
  long as the narration takes to read). The people and props in the scene can be removed there.
- **Library (right).** People, props and places from the catalog, with search. A click adds a person or a prop to the selected
  scene, or sets its environment.
- **Code tab.** `story.json` and every code scene in an editor. Problems are marked at their line and listed below; saving checks
  the story again. A code scene is a small program, `export default function scene(api) { ... }`, that draws a scene itself; it
  cannot reach the network or the page.
- **Issues.** The badge in the toolbar counts errors and warnings. An error blocks the export; the list says where.

## Export

The **Export** button opens a panel that first says what is missing. Exporting needs:

1. FFmpeg (found on `PATH` or at `video_studio.ffmpeg_path`),
2. a browser that can run headless (Chrome or Chromium). In the Docker image set `storymaker.render.browser_path` to a
   browser inside the container: the image has no Chromium libraries of its own,
3. a story without errors that is not longer than `max_duration_seconds`.

Editing and the preview work without 1 and 2. The export draws every frame in a headless browser, so a 15 minute video takes a
while; the panel shows the phase and the frame count and can cancel. The result is
`Documents/Storymaker/<project id>/exports/<title>-<job>.mp4`; **Show in Files** opens that folder. Phase 1 videos have no sound.

## Not in phase 1

Voices and narration audio, music and sound effects, captions, the agent panel and chat tool, the asset generator, charts and
diagrams, most transitions (cut, fade, slide-left and wipe exist) and drag and drop staging. The story format already accepts
the fields of later phases; the editor shows them as "not supported yet" and the player ignores them.

## For developers

`ui/js/desktop/apps/AGENTS.md` (Storymaker), `internal/storymaker/AGENTS.md` and `ui/js/storymaker/AGENTS.md` hold the
contracts. The app tour test is `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestStorymakerApp -count=1`.
````

`ui/js/desktop/apps/AGENTS.md` (after line 43, before `## Email exports`):

```diff
--- a/ui/js/desktop/apps/AGENTS.md
+++ b/ui/js/desktop/apps/AGENTS.md
@@ -41,6 +41,16 @@
 - The page never observes itself (its layout changes its own height and would loop the ResizeObserver); the shell observes the page host and the root. Field chips sit at the anchors of the schematic only when the page is as wide as the drawing plus the stage padding, otherwise they are listed under it; below 720 px the shell is one stack with a back button. Ctrl/Cmd+K focuses the search and stops propagation so the desktop Spotlight stays closed.
 - A schematic is a grid spec (20 px units) keyed by the calculator id. `TestECalcSchematicsMatchTheCatalog` checks ids, anchors and symbols against the catalog; `TestDesktopECalcBrowser` (`AURAGO_RUN_BROWSER_SMOKE=1`) checks that the chips of every schematic stay apart (and those of the divider in every desktop skin), plus live solving, locks, the star-delta set switch, the compact stack and Ctrl+K.
 
+## Storymaker
+
+- `storymaker*.js` are eleven classic scripts in this load order (module-loader entry `storymaker`, pinned by `TestDesktopStorymakerRegistration`): `storymaker-core.js` (the namespace `window.StorymakerParts`, the DOM builder `S.el` that has no html option, icons, the bus, error texts), `-model.js` (pure: JSON Pointer to text position, issue lists, the catalog index, the story edits), `-api.js`, `-assets.js` (catalog, fonts and code bundles for the player), `-player.js` (the iframe bridge), `-projects.js`, `-strip.js` (scene strip and transport), `-inspector.js` (inspector and library), `-code.js` (CodeMirror), `-export.js`, then the composition root `storymaker.js` (`window.StorymakerApp`). Style `css/desktop-app-storymaker.css`: `stm-` prefix, tokens only, container queries on the window, motion gated. The backend contract is `internal/storymaker/AGENTS.md`, the player `ui/js/storymaker/AGENTS.md`.
+- Untrusted text (story, narration, names, issue messages, server errors) reaches the page only as `textContent`. `TestStorymakerSourcesAreSafeAndSmall` greps these modules and `ui/js/storymaker` for innerHTML, insertAdjacentHTML, template strings (app modules), eval, browser dialogs and storage, and holds every module under 1100 lines.
+- The player iframe is `sandbox="allow-scripts"` and nothing else, `src` `/api/desktop/storymaker/player`. The parent accepts a message only from `iframe.contentWindow` and only in the schema of `S.player.checkIncoming` (contract section 8: fixed types, size caps, unknown fields dropped), posts with targetOrigin `'*'` (the iframe origin is opaque), sends `load` after every `ready` and answers `need_assets` only with what the catalog lists or the bundles of the story's code scenes. Ask for thumbnails after the first `time` message that follows a compile: the host cannot draw before its assets are in.
+- Edits go through `inst.editScene(id, fn, {immediate})`: it clones the story, applies `fn` and saves with If-Match (700 ms debounce while typing, one save in flight, the server's story with its defaults replaces the local one when nothing newer was typed). 412 shows the conflict banner (reload, or keep mine), 422 keeps the text and shows the issues, `desktop_readonly` turns the window read-only. Never add a field the story format lacks: the server decodes strictly.
+- The code tab keeps its drafts across tab and file switches, walks an issue's JSON Pointer to a text position (`S.model.locate`) and shows bundler errors at line and column. A new code scene is written from `S.codeTemplate` with If-None-Match before the story names it.
+- Export (`S.createExport`): capability states come from `GET /status` (FFmpeg, browser, read-only, story errors, the duration limit), progress from the project's SSE stream with a 2.5 s poll as safety net, the result is a project-folder path with "show in Files" through `ctx.openApp('files', {path})`. All copy lives in `lang/storymaker/*.json` (16 locales); the catalog names are `lang/storymaker_catalog` and are never defined here.
+- Verify: `go test ./ui -run 'Storymaker|TestDesktopStorymaker|TestConfigStorymaker' -count=1` and the browser tour `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestStorymakerApp -count=1`. The fixture `desktop_storymaker_fixture_test.go` runs the real `internal/storymaker` package behind `/api/desktop/storymaker`; `storymakerBrowserPage` takes any handler, so the same tour runs against the real server handlers. The model and the message schema run in Node inside `TestStorymakerAppModelInNode`.
+
 ## Email exports
 
 - First-party file producers/viewers register `AuraDesktopEmail.attach(windowId, toolbar, getOptions[, scope])`. The main-bundle owner is `core/email-export-runtime.js`; app contexts also expose `emailExport`. Store apps and Tresor are excluded. A scoped dialog binding must dispose with its dialog. Native modal dialogs pass `dialogHost` so format/setup choices remain in their active focus scope; QR closes through `onHandoff` after the Mailer owns the files, or `onSetup` after explicit settings navigation.
```

`AGENTS.md` (after base line 202, the ECalc row of the Child DOX index):

```diff
--- a/AGENTS.md
+++ b/AGENTS.md
@@ -200,6 +200,7 @@ $AURAGO_MASTER_KEY = ($bytes | ForEach-Object { $_.ToString("x2") }) -join ""
 | Docker Manager stacks | `internal/dockerstacks/AGENTS.md` | Stack folders per host, compose runner/lister/IO containers, file revisions, operations journal, reattach after restart, lint and `docker run` converter behind `/api/docker-manager`. |
 | Docker Manager live data | `internal/dockerwatch/AGENTS.md` | Engine events watcher per host, container stats rings and host gauges, disk-usage cache, stream budget and the stream-client hub behind `/api/docker-manager/stream`. |
 | ECalc | `internal/ecalc/AGENTS.md`, `internal/server/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md` | Electronics formula catalog, expression language, units, E-series and solver with shared Go/JS parity vectors; `GET /api/ecalc/catalog`; Desktop app and manual. |
+| Storymaker | `internal/storymaker/AGENTS.md`, `ui/js/storymaker/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `internal/server/AGENTS.md` | Explainer-video Desktop app: story document, validation and catalog index, project store, code-scene bundler, JS player (host, workers, compile, rig renderer), generated starter catalog (`assets/storymaker`, `ui/img/storymaker`), render job and listener, app modules, config section and `documentation/storymaker.md`. |
 | Managed RustFS S3 | `internal/rustfs/AGENTS.md` | Optional Docker storage lifecycle, private IAM credentials, scoped S3 leases and retained-volume recovery; server/UI/config/tool integration. |
 | Kaggle integration | `internal/kaggle/AGENTS.md`, `internal/notebooks/AGENTS.md` | Fixed native Kaggle contracts, account grants, durable jobs, project transfers, notebook preservation and managed local Jupyter; agent/server/UI/release integration. |
 | File utilities | `internal/fileutil/AGENTS.md` | Shared file replacement/free-space probe. |
```

`internal/desktop/AGENTS.md` (after base line 16, the ECalc bullet):

```diff
--- a/internal/desktop/AGENTS.md
+++ b/internal/desktop/AGENTS.md
@@ -14,6 +14,8 @@ The Service owns authorization, read-only state, mutation locks and cache invali
 
 - ECalc (`ecalc`, category `dev`) is a built-in electronics formula app with no capability gate. Its icon is app-owned artwork, `logo_path` `/img/desktop-icons/ecalc.svg`, and its `Icon` value `ecalc` is deliberately absent from `desktopPreferredIconNames` and every icon theme (as for Synth Studio), so every theme shows the logo; adding a themed `ecalc` icon would replace it. Its catalog route is `GET /api/ecalc/catalog` (`../server/AGENTS.md`). Verify `TestBuiltinECalcApp*` and `go test ./ui -run TestDesktopECalc`.
 
+- Storymaker (`storymaker`, category `creative`, `open_maximized`) is a built-in explainer-video app behind the `storymaker` capability (`Requires`, the `storymaker.enabled` switch; the capability case lives in `../server/bluetooth_live.go`). Its icon is app-owned artwork, `logo_path` `/img/desktop-icons/storymaker.svg`, and its `Icon` value `storymaker` is deliberately absent from `desktopPreferredIconNames` and every icon theme (as for ECalc). Its routes are `/api/desktop/storymaker/*` (`../server/AGENTS.md`), its projects live in `Documents/Storymaker/`. Verify `TestBuiltinStorymakerApp*` and `go test ./ui -run TestDesktopStorymaker`.
+
 - Desktop cloud transfers use `StatStoragePath` for fresh rooted metadata,
   `CreateStorageDirectory` for exclusive directory creation,
   `WriteFileStreamConditional` for bounded publication, and
```

`documentation/manuals/en/04-webui.md` (after base line 267):

```diff
--- a/documentation/manuals/en/04-webui.md
+++ b/documentation/manuals/en/04-webui.md
@@ -265,6 +265,7 @@ The Virtual Desktop opens workspace-backed apps in AuraGo's browser desktop. It
 | **Calculator / Settings / System Info / Log Viewer** | Calculator, desktop settings, system diagnostics, and a live log tail |
 | **ECalc** | Electronics formula collection: interactive calculators with schematics, typeset formulas and live results, plus E-series values for resistors. Runs in the browser and needs no setup ([Chapter 32](32-ecalc.md)) |
 | **Galaxa Deluxe / Chess / NASSCAD / System World** | Arcade shooter, chess (with agent opponent), docking game, and an explorable 3D system city with robots, telemetry, and tower voice |
+| **Storymaker** | Explainer videos: stage scenes with cartoon people, props and environments, write the narration, watch a live preview and export a silent MP4 (needs FFmpeg and a browser for the export) — see [Storymaker](../../storymaker.md) |
 | **Newspaper** | Personal daily edition: server-side research with vetted sources, sections, an archive of all editions, and optional email/Telegram delivery |
 | **Detective** | Bounded desktop research with adjustable effort, cited reports, and exports |
 | **Personal Radio** | Your own radio station: topic and music selection per station, a personal audio library, news, and speech segments |
```

`documentation/manuals/de/04-webui.md` (after base line 266):

```diff
--- a/documentation/manuals/de/04-webui.md
+++ b/documentation/manuals/de/04-webui.md
@@ -264,6 +264,7 @@ Der Virtual Desktop öffnet Workspace-basierte Apps direkt im AuraGo-Browser-Des
 | **Cheater** | Cheat-Sheet-Verwaltung mit Markdown und Anhängen |
 | **Calculator / Settings / System Info / Log Viewer** | Rechner, Desktop-Einstellungen, Systemdiagnose und Live-Logtail |
 | **ECalc** | Elektronik-Formelsammlung: interaktive Rechner mit Schaltplänen, gesetzten Formeln und Live-Ergebnissen, dazu E-Reihen-Werte für Widerstände. Läuft im Browser und braucht keine Einrichtung ([Kapitel 32](32-ecalc.md)) |
+| **Storymaker** | Erklärvideos: Szenen mit Comicfiguren, Requisiten und Umgebungen inszenieren, den Sprechtext schreiben, die Vorschau ansehen und ein stummes MP4 exportieren (der Export braucht FFmpeg und einen Browser), siehe [Storymaker](../../storymaker.md) |
 | **Galaxa Deluxe / Chess / NASSCAD / Systemwelt** | Arcade-Spiel, Schach (mit Agent-Gegner), Docking-Spiel und begehbare 3D-Systemstadt mit Robotern, Telemetrie und Turm-Stimme |
 | **Newspaper** | Persönliche Tageszeitung: serverseitige Recherche mit geprüften Quellen, Ressorts, Archiv aller Ausgaben und optionaler E-Mail-/Telegram-Zustellung |
 | **Detective** | Gebundene Desktop-Recherchen mit einstellbarer Intensität, belegten Berichten und Export |
```

- [ ] **Step 3: Run the check**

Run: `node -e "const fs=require('fs');for(const f of ['ui/js/storymaker/AGENTS.md','documentation/storymaker.md','ui/js/desktop/apps/storymaker.js','ui/css/desktop-app-storymaker.css','ui/desktop_storymaker_browser_test.go']) if(!fs.existsSync(f)) throw new Error('missing '+f); console.log('docs paths ok')"`
Expected: docs paths ok

Recorded at drafting time:

```text
docs paths ok
```

- [ ] **Step 4: Commit**

```bash
git add -f ui/js/storymaker/AGENTS.md   # .gitignore ignores nested AGENTS.md files; -f is needed for a new one
git add documentation/storymaker.md ui/js/desktop/apps/AGENTS.md AGENTS.md internal/desktop/AGENTS.md documentation/manuals/en/04-webui.md documentation/manuals/de/04-webui.md
git commit -m "docs(storymaker): apps contract, player contract, user page and manual rows" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task E11: Full verification and the graph check

**Files:** none (nothing to commit unless a step finds a problem; fix it in the task that owns the file).

- [ ] **Step 1: Regenerate and check the bundles**

```bash
npm run build:ui
npm run check:ui
git status --short
```
Expected: `check:ui` prints `Checked ...` for every bundle and `git status` shows nothing new (the bundles are committed in E6).

- [ ] **Step 2: The Storymaker tests and the neighbours they touch**

```bash
go test ./ui -run 'Storymaker|TestDesktopStorymaker|TestConfigStorymaker|TestDesktopAppMaterials|TestDesktopBuiltinAppNames|TestDesktopUsedI18nKeys|TestTranslations_|TestShippedWebAssetsLoadNothingExternal|TestConfigWave1Batch4|TestConfigDerivedValidationRulesCoverTemplate' -count=1
go test ./internal/desktop -count=1
```
Expected: ok. `go test ./ui -count=1` as a whole fails only in the two tests that already failed at the base commit: `TestChatFrontend_IntegrationsDrawerRemainsWired` and `TestFrontend_StaticI18nKeysExistInEnglishBundle` (seven keys in `findit-*.js`). Nothing in a storymaker file may appear in a failure.

- [ ] **Step 3: The browser tests**

```bash
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestStorymakerApp|TestConfigStorymakerBrowser' -count=1 -timeout 10m
```
Expected: ok, about 40 s. Visual check by hand once: `AURAGO_BROWSER_ARTIFACT_DIR=<dir>` makes the tour write `01-start.png` to `07-export-done.png`.

- [ ] **Step 4: GitNexus detect-changes (project rule, before the last commit of the part)**

```bash
node C:/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs detect-changes --scope all --repo .
```
Expected: the changed symbols are the registry entries and functions named in Tasks E1, E6 and E9. `partial: true` or `truncated: true` is not a clean check: re-run it. If the index is stale, run `node C:/Users/Andi/Documents/repo/AuraGo/.gitnexus/run.cjs analyze --index-only` first.

