[← Plan index](README.md) · Spec: [plans/2026-11-13-easydrag-w1a-spec.md](../2026-11-13-easydrag-w1a-spec.md)

## Part A: Status map and tokens (FD-08, S0c row 24)

Part A adds the token families, the status module `easydrag-status.js`, and moves every status surface of EasyDrag onto it. It ends with the status lint. Spec: `plans/2026-11-13-easydrag-w1a-spec.md` §3. Contracts: `C:/disposable/easydrag-w1a-plan/BRIEF.md`. Part B starts after this part has landed.

**Shell and worktree.** Git Bash. Every command starts with `cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && …`, because the working directory resets between calls. Go commands carry `GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4` (the go1.27.3 toolchain loses its `vet.exe` now and then). Never edit, stage or commit in the main checkout. No bundle build is needed: EasyDrag scripts are not in `scripts/build-ui-bundles.js`, so a new module needs no `npm run build:ui` and no `node_modules` copy. If a Node script still says `Cannot find module`, copy `node_modules` from the main checkout (memory note `reference_worktree_node_modules.md`).

**Preamble (run once).**

- [ ] **Step 0.1: create the worktree and the scratch folders**

```
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-w1a-status -b feat/easydrag-w1a-status main && mkdir -p C:/disposable/easydrag-w1a/keys D:/tmp/go-build D:/tmp/easydrag-w1a-shots && cd .worktrees/easydrag-w1a-status && git status --short && git branch --show-current
```
Expected: `Preparing worktree …`, no status lines, then `feat/easydrag-w1a-status`.

- [ ] **Step 0.2: the T0 baseline**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 npm run test:easydrag:t0 2>&1 | tail -15
```
Expected: exit code 0 and no `FAIL` line, in about 35-55 s. Note the final lines in your report. If it fails here, the failure is not yours: tell the coordinator before you start.

- [ ] **Step 0.3: the browser baseline (once, about 5-6 minutes, sandbox off)**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v 2>&1 | tee D:/tmp/easydrag-w1a-shots/a-baseline.log | grep -c '^--- PASS'; grep -c '^--- SKIP' D:/tmp/easydrag-w1a-shots/a-baseline.log; grep '^--- FAIL' D:/tmp/easydrag-w1a-shots/a-baseline.log
```
Expected on main: `41`, `2`, no FAIL line (the brief's numbers). Keep the log; Task A.7 compares against it. Chrome cannot start from a test binary under the repo with the sandbox on (memory note `reference_chrome_launch_from_agent.md`).

**Facts verified on main 9492395de (read them, do not redo them).**

- **Radius.** `--ed-radius: var(--ds-radius-md)` is 10px (Fruity 12px), and `--ed-radius-lg: var(--ds-radius-lg)` is **14px** (Fruity 16px). The contract rule applies: the old names keep their values, and the new 12px token is named `--ed-radius-xl`. `--ed-radius` is not aliased to the new 8px `--ed-radius-md` either, for the same reason (10px today).
- **Motion.** `--ed-fast`/`--ed-base` read `--ds-motion-duration-fast/base` (120/200ms), and `desktop-theme-amiga.css:92-94` sets them to 0ms. So the new motion tokens read the same `--ds-*` values with 120/200ms fallbacks, and the old names become aliases of the new ones. A fixed 120ms would switch Amiga's "no animation" off.
- **Raw values.** Run statuses (`internal/flows/run_types.go:29-34`): `queued running waiting success error cancelled`. Step statuses (`:55-61`): `pending running success error skipped waiting cancelled`. A step may carry `effect_uncertain: true`. Durable states (`easydrag-durable.js:7`): `waiting running blocked approval_pending`.
- **The harness `t`** (`scripts/test-easydrag.mjs:48`) returns the key without `easydrag.ui.`, so `easydrag.status.ok` comes back unchanged. `ED.status.word` treats "answer equals key" as missing and falls back to the value name (`ok`). Node tests that go through the harness therefore see `ok`, `failed`, and so on.
- **The line ratchet:** `easydrag-canvas.js` 928 lines (must not grow), `easydrag-editor.js` 896, `easydrag-detail.js` 878 (existing lines only).
- **Hex ban:** `TestEasyDragStylesheetUsesThemeTokens` forbids hex colours after the `.ed-app [data-cat] {` rule (line 246). New hex values live in the token block or the band blocks above it.
- **Card ring.** `.ed-node.status-<raw>` (CSS 720-731, `classesFor` in canvas.js:515) is the card's state border, not a rendered status. Many browser tests select it. It stays and goes on the lint allow-list with that reason (Task A.7).

### The decision on the grey token (measured)

`--ed-status-neutral: color-mix(in srgb, var(--ed-text) 60%, var(--ed-panel-strong))`. Measured with the existing helpers (scratch test through `go test -overlay`, nothing in the repo changed). The 16 contexts are the 14 themes with Fruity and Minimal in both schemes. `text` is `--vd-text`, `panel` is the card material colour:

| Context | text | panel | disc at 60% | worst contrast on canvas/card/well |
|---|---|---|---|---|
| standard | 237,241,247 | 26,33,43 | 153,158,165 | 6.01 (card) |
| fruity-light | 25,35,52 | 255,255,255 | 117,123,133 | 3.67 (well) |
| amiga | 0,0,0 | 187,187,187 | 75,75,75 | 3.76 (canvas) |
| cyberpunk | 232,247,255 | 12,19,40 | 144,156,169 | 6.59 (card) |
| lollipop | 74,31,61 | 255,255,255 | 146,121,139 | 3.42 (canvas) |
| windows_xp | 28,45,69 | 250,251,254 | 117,127,143 | 3.32 (canvas) |
| voxel_world | 53,43,30 | 208,208,208 | 115,109,101 | 3.32 (card) |
| minimal-light | 22,22,22 | 255,255,255 | 115,115,115 | 4.16 (well) |
| minimal-dark | 237,237,237 | 32,32,32 | 155,155,155 | 5.86 (card) |

- **Smallest mix that works:** 55% fails in windows_xp (2.87), voxel_world (2.95) and lollipop (2.98), so **60%** is the smallest step of 5 that reaches 3:1 in all 16 contexts. The worst case is 3.32:1.
- **Glyph on the grey disc.** The mock draws white on grey. In dark themes the disc is light grey, and white on it is 2.7:1 or worse. So the glyph is drawn in the panel colour (`--ed-status-neutral-ink: var(--ed-panel-strong)`), which is 3.32:1 or better everywhere (voxel_world is the worst). A glyph is a graphic, so 3:1 is the bar; the word next to it carries the meaning. A 4.5:1 glyph would need about 75% (4.82 in voxel_world), which makes the disc nearly the text colour. See "Decisions needed".
- **Distance to the categories (coordinator decision).** The grey token is checked for contrast only. Logic and tool tiles are grey by design; at 3:1 on a light panel the grey disc is within 0.05-0.15 OKLab of the light-band tiles and no mix can change that. The four coloured status tokens keep their distance checks as today, in `TestEasyDragCategoryColourTokens`, which this part does not touch.
- **Two findings that need a fix beyond the grey token.**
  1. **The coloured discs fail 3:1 on light surfaces in four themes.** The status tokens are the bright desktop-base values there: fruity-light 1.54-2.18, amiga 1.00-1.09, windows_xp 1.47-2.08, voxel_world 1.16-1.64. Per spec §11 the colours stay and the surface side is fixed: every disc carries a 1px ring in `--ed-status-edge` (= the grey, 3.32:1 or better on all surfaces).
  2. **A fixed dark glyph ink fails on the two themes with dark status colours.** Ink `#0b1220` on lollipop's four colours is 2.86-3.50:1 and on minimal-light's 3.16-3.69:1. All other contexts give 5.55-13.29:1. Those two themes set `--ed-status-ink: #ffffff` in the band block that already carries them (CSS line 141), which gives 5.08-6.54:1.

---

### Task A.1: Tokens and the colour test

**Files:**
- Modify: `ui/css/desktop-app-easydrag.css` (token block, around lines 79-87, and the light band block at line 141)
- Create: `ui/desktop_easydrag_status_colour_test.go`

- [ ] **Step 1: write the failing colour test**

Create `ui/desktop_easydrag_status_colour_test.go`:

```go
package ui

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// W1a A (FD-08): the status disc colours. The four coloured status tokens keep the hue and OKLab distance checks of
// TestEasyDragCategoryColourTokens. The grey token (--ed-status-neutral) is checked for contrast only: the logic and tool
// tiles are grey by design and grey statuses carry their meaning through the glyph, so no mix could keep 0.20 OKLab from them.

var easyDragStatusNeutralMix = regexp.MustCompile(`^color-mix\(in srgb, var\(--ed-text\) (\d+)%, var\(--ed-panel-strong\)\)$`)

func TestEasyDragStatusColourTokens(t *testing.T) {
	sheets := map[string]cssSheet{}
	ed := easyDragS0Sheet(t, sheets, "css/desktop-app-easydrag.css")
	root := easyDragS0RuleDecls(ed, easyDragS0DarkBand)
	match := easyDragStatusNeutralMix.FindStringSubmatch(root["--ed-status-neutral"])
	if match == nil {
		t.Fatalf(".ed-app --ed-status-neutral = %q, want color-mix(in srgb, var(--ed-text) N%%, var(--ed-panel-strong))", root["--ed-status-neutral"])
	}
	percent, _ := strconv.ParseFloat(match[1], 64)
	for token, want := range map[string]string{"--ed-status-neutral-ink": "var(--ed-panel-strong)", "--ed-status-edge": "var(--ed-status-neutral)"} {
		if root[token] != want {
			t.Errorf(".ed-app %s = %q, want %s", token, root[token], want)
		}
	}
	if disc := easyDragS0RuleDecls(ed, ".ed-status-disc"); !strings.Contains(disc["box-shadow"], "var(--ed-status-edge)") {
		t.Errorf(".ed-status-disc box-shadow = %q, want a ring in var(--ed-status-edge): the coloured discs fail 3:1 on light surfaces without it", disc["box-shadow"])
	}
	for _, ctx := range easyDragS0ThemeContexts(t) {
		surfaces := easyDragS0Surfaces(t, sheets, ctx)
		text := easyDragS0HexToken(t, sheets, ctx, "--vd-text")
		card := surfaces["card"]
		var neutral [3]float64
		for i := range neutral {
			neutral[i] = math.Round(text[i]*percent/100 + card[i]*(100-percent)/100)
		}
		// the grey disc and its ring reach 3:1 on the canvas, card and well; its glyph (the panel colour) reaches 3:1 on it
		if ratio, surface := easyDragS0MinContrast(neutral, surfaces); ratio < 3 {
			t.Errorf("%s: the grey status disc %v is %.2f:1 on the %s, want at least 3:1", ctx.Name, neutral, ratio, surface)
		}
		if ratio := cssContrastRatio(card, neutral); ratio < 3 {
			t.Errorf("%s: the glyph on the grey disc is %.2f:1, want at least 3:1", ctx.Name, ratio)
		}
		// the glyph ink on the four coloured discs reaches 4.5:1 (this context's ink: dark by default, white where a band block says so)
		ink, ok := parseCSSHexColor(easyDragS0EdDecls(ed, ctx)["--ed-status-ink"])
		if !ok {
			t.Errorf("%s: --ed-status-ink is not a six-digit hex colour", ctx.Name)
			continue
		}
		for _, token := range easyDragS0StatusTokens {
			if ratio := cssContrastRatio(ink, easyDragS0HexToken(t, sheets, ctx, token)); ratio < 4.5 {
				t.Errorf("%s: the glyph ink on %s is %.2f:1, want at least 4.5:1", ctx.Name, token, ratio)
			}
		}
	}
}
```

- [ ] **Step 2: run it and see it fail for the stated reason**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go test ./ui -run 'TestEasyDragStatusColourTokens' -count=1 2>&1 | grep -v '^{' | head
```
Expected: `--- FAIL: TestEasyDragStatusColourTokens` with `.ed-app --ed-status-neutral = "", want color-mix(…`.

- [ ] **Step 3: add the tokens**

In `ui/css/desktop-app-easydrag.css`, replace the five token lines (they sit together):

```
    --ed-radius: var(--ds-radius-md);
    --ed-radius-lg: var(--ds-radius-lg);
    --ed-ease: var(--ds-motion-ease-standard);
    --ed-ease-spring: cubic-bezier(0.34, 1.56, 0.64, 1);
    --ed-fast: var(--ds-motion-duration-fast);
    --ed-base: var(--ds-motion-duration-base);
```
with:

```
    /* W1a tokens. Type scale 24/18/16/14/12/11 (owner decision: card names stay at least 13.8px and parameter lines at least 12px at fit zoom 0.86). Part B moves every font-size onto these six. */
    --ed-font-title: 24px;
    --ed-font-head: 18px;
    --ed-font-card: 16px;
    --ed-font-body: 14px;
    --ed-font-meta: 12px;
    --ed-font-label: 11px;
    /* radius: --ed-radius (10px, theme-scaled) and --ed-radius-lg (14px, theme-scaled) keep their values. The fixed 12px step is -xl because -lg is taken. */
    --ed-radius: var(--ds-radius-md);
    --ed-radius-lg: var(--ds-radius-lg);
    --ed-radius-sm: 4px;
    --ed-radius-md: 8px;
    --ed-radius-xl: 12px;
    --ed-radius-pill: 999px;
    --ed-space-1: 4px;
    --ed-space-2: 8px;
    --ed-space-3: 12px;
    --ed-space-4: 16px;
    --ed-space-5: 24px;
    --ed-space-6: 32px;
    /* motion: the durations follow the desktop's (Amiga sets them to 0), the old names are aliases */
    --ed-motion-fast: var(--ds-motion-duration-fast, 120ms);
    --ed-motion-base: var(--ds-motion-duration-base, 200ms);
    --ed-motion-slow: calc(var(--ed-motion-base) * 1.6);
    --ed-ease-out: var(--ds-motion-ease-standard, cubic-bezier(0.2, 0, 0, 1));
    --ed-ease: var(--ed-ease-out);
    --ed-ease-spring: cubic-bezier(0.34, 1.56, 0.64, 1);
    --ed-fast: var(--ed-motion-fast);
    --ed-base: var(--ed-motion-base);
    /* status disc (W1a A). Set by EasyDrag, never by a theme. The grey is a fixed mix of the text and the panel colour; 60% is the
       smallest step that reaches 3:1 on the canvas, card and well in all 16 contexts. The ring keeps a coloured disc visible where the
       status colour is too light for the surface. The glyph ink is dark on the four coloured discs and the panel colour on the grey one. */
    --ed-status-neutral: color-mix(in srgb, var(--ed-text) 60%, var(--ed-panel-strong));
    --ed-status-neutral-ink: var(--ed-panel-strong);
    --ed-status-edge: var(--ed-status-neutral);
    --ed-status-ink: #0b1220;
```
Then in the light band block (line 141), add the white ink for the two themes whose status colours are dark. Replace:

```
.desktop-body[data-theme="minimal"]:not([data-scheme="dark"]) .ed-app {
    --ed-cat-trigger: #ddd9ff;
```
with:

```
.desktop-body[data-theme="minimal"]:not([data-scheme="dark"]) .ed-app {
    --ed-status-ink: #ffffff;
    --ed-cat-trigger: #ddd9ff;
```
The test also needs the `.ed-status-disc` rule with the ring; Task A.2 adds the whole rule family. To keep this commit green, add this single rule now directly after the `.ed-status--skipped` line (CSS about line 719; A.2 extends it):

```
.ed-status-disc { box-shadow: 0 0 0 1px var(--ed-status-edge); }
```

- [ ] **Step 4: run the colour tests and the stylesheet tests**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go test ./ui -run '(?i)(TestEasyDragStatusColourTokens|TestEasyDragCategoryColourTokens|TestEasyDragStylesheetUsesThemeTokens|TestEasyDragStateMarksUseChange)' -count=1 -v 2>&1 | grep -E '^(--- |ok|FAIL)'
```
Expected: four `--- PASS` lines and `ok  	aurago/ui`. If `TestEasyDragStatusColourTokens` reports a ratio below the bar in a context, read the context name: a context other than lollipop and minimal-light failing the ink check means its status tokens are darker than measured, so give it the same `--ed-status-ink: #ffffff` line in its own band block (the contexts and their blocks are listed by `easyDragS0ThemeContexts`).

- [ ] **Step 5: gates**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && gofmt -l ui/desktop_easydrag_status_colour_test.go && GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go vet ./ui && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1
```
Expected: gofmt prints nothing, vet is silent, and `PASS render baseline (unchanged)`.

- [ ] **Step 6: commit**

Write `C:/disposable/easydrag-w1a/msg-a1.txt` with the Write tool:

```
feat(easydrag): W1a tokens and the status grey

Add the type, radius, space and motion token families and --ed-status-neutral
(60% text over the panel colour, 3.32:1 or better on every surface in all 16
theme contexts). --ed-radius-lg is 14px today, so the new 12px step is
--ed-radius-xl. The motion tokens follow the desktop durations (Amiga is 0ms);
--ed-fast, --ed-base and --ed-ease become aliases. The disc ring token and the
glyph inks come with them: two themes with dark status colours use white ink.
The grey is checked for contrast only; the four coloured tokens keep their
distance checks.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && git add ui/css/desktop-app-easydrag.css ui/desktop_easydrag_status_colour_test.go && git commit -F C:/disposable/easydrag-w1a/msg-a1.txt
```

---

### Task A.2: `easydrag-status.js`, its CSS, the keys and the Node test

**Files:**
- Create: `ui/js/desktop/apps/easydrag-status.js`
- Create: `scripts/test-easydrag-status.mjs`
- Create (outside the repo): `C:/disposable/easydrag-w1a/keys/a2.json`
- Modify: `ui/css/desktop-app-easydrag.css` (the `.ed-status-disc` family, next to the old `.ed-status` rules)
- Modify: `ui/js/desktop/core/module-loader.js:285-286`, `ui/desktop_easydrag_test.go:12`, `scripts/test-easydrag-extra2.mjs:9`, `scripts/test-easydrag-extra3.mjs:9`, `scripts/test-easydrag-extra4.mjs:12`
- Modify: `ui/lang/easydrag/*.json` (16 files, through the helper)

- [ ] **Step 1: the key file (the words and aria sentences, all 16 locales)**

The module sits after `easydrag-core.js` and before `easydrag-durable.js`. Write `C:/disposable/easydrag-w1a/keys/a2.json` with the Write tool. The words use the German list of the spec; the aria sentences say what the status means for the user. `easydrag.ui.port_uncertain` is set to the same word as `easydrag.status.uncertain`, so every locale has one word for "unclear" (the status lint checks this).

```json
{
  "easydrag.status.ok": {"cs": "Úspěšné", "da": "Lykkedes", "de": "Erfolgreich", "el": "Επιτυχία", "en": "Succeeded", "es": "Correcto", "fr": "Réussi", "hi": "सफल", "it": "Riuscito", "ja": "成功", "nl": "Geslaagd", "no": "Vellykket", "pl": "Powodzenie", "pt": "Concluído", "sv": "Lyckades", "zh": "成功"},
  "easydrag.status.ok_aria": {"cs": "Stav: úspěšné", "da": "Status: lykkedes", "de": "Status: erfolgreich", "el": "Κατάσταση: επιτυχία", "en": "Status: succeeded", "es": "Estado: correcto", "fr": "Statut : réussi", "hi": "स्थिति: सफल", "it": "Stato: riuscito", "ja": "状態: 成功", "nl": "Status: geslaagd", "no": "Status: vellykket", "pl": "Stan: powodzenie", "pt": "Estado: concluído", "sv": "Status: lyckades", "zh": "状态：成功"},
  "easydrag.status.stopped": {"cs": "Ukončeno Stopem", "da": "Afsluttet af Stop", "de": "Durch Stopp beendet", "el": "Τερματίστηκε από Διακοπή", "en": "Ended by stop", "es": "Finalizado por Stop", "fr": "Arrêté par Stop", "hi": "स्टॉप से समाप्त", "it": "Terminato da Stop", "ja": "停止で終了", "nl": "Beëindigd door Stop", "no": "Avsluttet av Stopp", "pl": "Zakończono przez Stop", "pt": "Terminado por Stop", "sv": "Avslutad av Stopp", "zh": "因停止而结束"},
  "easydrag.status.stopped_aria": {"cs": "Stav: ukončeno krokem Stop, jak bylo zamýšleno", "da": "Status: afsluttet af et stoptrin, som planlagt", "de": "Status: durch einen Stopp-Schritt wie geplant beendet", "el": "Κατάσταση: τερματίστηκε από βήμα διακοπής, όπως σχεδιάστηκε", "en": "Status: ended by a stop step, as planned", "es": "Estado: finalizado por un paso de parada, según lo previsto", "fr": "Statut : arrêté par une étape Stop, comme prévu", "hi": "स्थिति: स्टॉप चरण से योजना के अनुसार समाप्त", "it": "Stato: terminato da un passaggio di arresto, come previsto", "ja": "状態: 停止ステップにより予定どおり終了", "nl": "Status: beëindigd door een stopstap, zoals bedoeld", "no": "Status: avsluttet av et stopptrinn, som planlagt", "pl": "Stan: zakończono krokiem zatrzymania, zgodnie z planem", "pt": "Estado: terminado por um passo de paragem, como previsto", "sv": "Status: avslutad av ett stoppsteg, som planerat", "zh": "状态：按计划由停止步骤结束"},
  "easydrag.status.failed": {"cs": "Selhalo", "da": "Mislykkedes", "de": "Fehlgeschlagen", "el": "Αποτυχία", "en": "Failed", "es": "Fallido", "fr": "Échec", "hi": "विफल", "it": "Non riuscito", "ja": "失敗", "nl": "Mislukt", "no": "Mislyktes", "pl": "Niepowodzenie", "pt": "Falhou", "sv": "Misslyckades", "zh": "失败"},
  "easydrag.status.failed_aria": {"cs": "Stav: selhalo", "da": "Status: mislykkedes", "de": "Status: fehlgeschlagen", "el": "Κατάσταση: αποτυχία", "en": "Status: failed", "es": "Estado: fallido", "fr": "Statut : échec", "hi": "स्थिति: विफल", "it": "Stato: non riuscito", "ja": "状態: 失敗", "nl": "Status: mislukt", "no": "Status: mislyktes", "pl": "Stan: niepowodzenie", "pt": "Estado: falhou", "sv": "Status: misslyckades", "zh": "状态：失败"},
  "easydrag.status.running": {"cs": "Běží", "da": "Kører", "de": "Läuft", "el": "Εκτελείται", "en": "Running", "es": "En curso", "fr": "En cours", "hi": "चल रहा है", "it": "In corso", "ja": "実行中", "nl": "Bezig", "no": "Kjører", "pl": "W toku", "pt": "A executar", "sv": "Pågår", "zh": "运行中"},
  "easydrag.status.running_aria": {"cs": "Stav: běží", "da": "Status: kører", "de": "Status: läuft", "el": "Κατάσταση: εκτελείται", "en": "Status: running", "es": "Estado: en curso", "fr": "Statut : en cours", "hi": "स्थिति: चल रहा है", "it": "Stato: in corso", "ja": "状態: 実行中", "nl": "Status: bezig", "no": "Status: kjører", "pl": "Stan: w toku", "pt": "Estado: a executar", "sv": "Status: pågår", "zh": "状态：运行中"},
  "easydrag.status.waiting": {"cs": "Čeká", "da": "Venter", "de": "Wartet", "el": "Αναμονή", "en": "Waiting", "es": "En espera", "fr": "En attente", "hi": "प्रतीक्षा में", "it": "In attesa", "ja": "待機中", "nl": "Wacht", "no": "Venter", "pl": "Oczekuje", "pt": "A aguardar", "sv": "Väntar", "zh": "等待中"},
  "easydrag.status.waiting_aria": {"cs": "Stav: čeká", "da": "Status: venter", "de": "Status: wartet", "el": "Κατάσταση: σε αναμονή", "en": "Status: waiting", "es": "Estado: en espera", "fr": "Statut : en attente", "hi": "स्थिति: प्रतीक्षा में", "it": "Stato: in attesa", "ja": "状態: 待機中", "nl": "Status: wacht", "no": "Status: venter", "pl": "Stan: oczekuje", "pt": "Estado: a aguardar", "sv": "Status: väntar", "zh": "状态：等待中"},
  "easydrag.status.uncertain": {"cs": "Nejasné", "da": "Uklart", "de": "Unklar", "el": "Ασαφές", "en": "Unclear", "es": "Incierto", "fr": "Incertain", "hi": "अस्पष्ट", "it": "Incerto", "ja": "不明", "nl": "Onduidelijk", "no": "Uklart", "pl": "Niejasne", "pt": "Incerto", "sv": "Oklart", "zh": "不明确"},
  "easydrag.status.uncertain_aria": {"cs": "Stav: nejasné, účinek už mohl nastat", "da": "Status: uklart, effekten kan allerede være sket", "de": "Status: unklar, die Wirkung ist möglicherweise schon eingetreten", "el": "Κατάσταση: ασαφές, το αποτέλεσμα μπορεί να έχει ήδη συμβεί", "en": "Status: unclear, the effect may already have happened", "es": "Estado: incierto, el efecto puede haberse producido ya", "fr": "Statut : incertain, l'effet a peut-être déjà eu lieu", "hi": "स्थिति: अस्पष्ट, प्रभाव शायद पहले ही हो चुका है", "it": "Stato: incerto, l'effetto potrebbe essere già avvenuto", "ja": "状態: 不明。影響がすでに発生した可能性があります", "nl": "Status: onduidelijk, het effect kan al zijn opgetreden", "no": "Status: uklart, effekten kan allerede ha skjedd", "pl": "Stan: niejasne, skutek mógł już nastąpić", "pt": "Estado: incerto, o efeito pode já ter acontecido", "sv": "Status: oklart, effekten kan redan ha inträffat", "zh": "状态：不明确，影响可能已经发生"},
  "easydrag.status.blocked": {"cs": "Blokováno", "da": "Blokeret", "de": "Blockiert", "el": "Μπλοκαρισμένο", "en": "Blocked", "es": "Bloqueado", "fr": "Bloqué", "hi": "अवरुद्ध", "it": "Bloccato", "ja": "ブロック中", "nl": "Geblokkeerd", "no": "Blokkert", "pl": "Zablokowane", "pt": "Bloqueado", "sv": "Blockerad", "zh": "已阻止"},
  "easydrag.status.blocked_aria": {"cs": "Stav: blokováno, nejprve je potřeba tvůj zásah", "da": "Status: blokeret, du skal handle først", "de": "Status: blockiert, du musst zuerst handeln", "el": "Κατάσταση: μπλοκαρισμένο, απαιτείται πρώτα δική σου ενέργεια", "en": "Status: blocked, it needs your action first", "es": "Estado: bloqueado, necesita tu acción primero", "fr": "Statut : bloqué, une action de votre part est nécessaire", "hi": "स्थिति: अवरुद्ध, पहले आपकी कार्रवाई चाहिए", "it": "Stato: bloccato, serve prima una tua azione", "ja": "状態: ブロック中。先に操作が必要です", "nl": "Status: geblokkeerd, er is eerst jouw actie nodig", "no": "Status: blokkert, du må handle først", "pl": "Stan: zablokowane, najpierw potrzebna jest Twoja akcja", "pt": "Estado: bloqueado, precisa primeiro da tua ação", "sv": "Status: blockerad, du måste agera först", "zh": "状态：已阻止，需要你先操作"},
  "easydrag.status.skipped": {"cs": "Přeskočeno", "da": "Sprunget over", "de": "Übersprungen", "el": "Παραλείφθηκε", "en": "Skipped", "es": "Omitido", "fr": "Ignoré", "hi": "छोड़ा गया", "it": "Saltato", "ja": "スキップ", "nl": "Overgeslagen", "no": "Hoppet over", "pl": "Pominięte", "pt": "Ignorado", "sv": "Hoppades över", "zh": "已跳过"},
  "easydrag.status.skipped_aria": {"cs": "Stav: přeskočeno, tento krok nebyl dosažen", "da": "Status: sprunget over, dette trin blev ikke nået", "de": "Status: übersprungen, dieser Schritt wurde nicht erreicht", "el": "Κατάσταση: παραλείφθηκε, το βήμα δεν προσεγγίστηκε", "en": "Status: skipped, this step was not reached", "es": "Estado: omitido, no se llegó a este paso", "fr": "Statut : ignoré, cette étape n'a pas été atteinte", "hi": "स्थिति: छोड़ा गया, यह चरण नहीं पहुँचा", "it": "Stato: saltato, questo passaggio non è stato raggiunto", "ja": "状態: スキップ。このステップには到達しませんでした", "nl": "Status: overgeslagen, deze stap is niet bereikt", "no": "Status: hoppet over, dette trinnet ble ikke nådd", "pl": "Stan: pominięte, do tego kroku nie dotarto", "pt": "Estado: ignorado, este passo não foi alcançado", "sv": "Status: hoppades över, det här steget nåddes inte", "zh": "状态：已跳过，未执行到此步骤"},
  "easydrag.status.paused": {"cs": "Pozastaveno", "da": "På pause", "de": "Pausiert", "el": "Σε παύση", "en": "Paused", "es": "En pausa", "fr": "En pause", "hi": "रोका गया", "it": "In pausa", "ja": "一時停止", "nl": "Gepauzeerd", "no": "Satt på pause", "pl": "Wstrzymane", "pt": "Em pausa", "sv": "Pausad", "zh": "已暂停"},
  "easydrag.status.paused_aria": {"cs": "Stav: pozastaveno, vypnuto", "da": "Status: på pause, slået fra", "de": "Status: pausiert, ausgeschaltet", "el": "Κατάσταση: σε παύση, απενεργοποιημένο", "en": "Status: paused, switched off", "es": "Estado: en pausa, desactivado", "fr": "Statut : en pause, désactivé", "hi": "स्थिति: रोका गया, बंद किया गया", "it": "Stato: in pausa, disattivato", "ja": "状態: 一時停止中。オフになっています", "nl": "Status: gepauzeerd, uitgeschakeld", "no": "Status: satt på pause, slått av", "pl": "Stan: wstrzymane, wyłączone", "pt": "Estado: em pausa, desligado", "sv": "Status: pausad, avstängd", "zh": "状态：已暂停，已关闭"},
  "easydrag.status.simulated": {"cs": "Simulováno", "da": "Simuleret", "de": "Simuliert", "el": "Προσομοιώθηκε", "en": "Simulated", "es": "Simulado", "fr": "Simulé", "hi": "अनुकरण किया", "it": "Simulato", "ja": "シミュレーション", "nl": "Gesimuleerd", "no": "Simulert", "pl": "Symulowane", "pt": "Simulado", "sv": "Simulerad", "zh": "已模拟"},
  "easydrag.status.simulated_aria": {"cs": "Stav: simulováno, nic se doopravdy neodeslalo ani nezměnilo", "da": "Status: simuleret, intet blev rigtigt sendt eller ændret", "de": "Status: simuliert, es wurde nichts wirklich gesendet oder geändert", "el": "Κατάσταση: προσομοίωση, τίποτα δεν στάλθηκε ή άλλαξε πραγματικά", "en": "Status: simulated, nothing was really sent or changed", "es": "Estado: simulado, no se envió ni cambió nada de verdad", "fr": "Statut : simulé, rien n'a vraiment été envoyé ni modifié", "hi": "स्थिति: अनुकरण, वास्तव में कुछ भेजा या बदला नहीं गया", "it": "Stato: simulato, nulla è stato davvero inviato o modificato", "ja": "状態: シミュレーション。実際には何も送信・変更されていません", "nl": "Status: gesimuleerd, er is niets echt verzonden of gewijzigd", "no": "Status: simulert, ingenting ble egentlig sendt eller endret", "pl": "Stan: symulowane, nic naprawdę nie zostało wysłane ani zmienione", "pt": "Estado: simulado, nada foi realmente enviado ou alterado", "sv": "Status: simulerad, inget skickades eller ändrades på riktigt", "zh": "状态：已模拟，没有真正发送或更改任何内容"},
  "easydrag.status.not_run": {"cs": "Nespuštěno", "da": "Ikke kørt", "de": "Nicht gelaufen", "el": "Δεν εκτελέστηκε", "en": "Not run", "es": "No ejecutado", "fr": "Non exécuté", "hi": "नहीं चला", "it": "Non eseguito", "ja": "未実行", "nl": "Niet uitgevoerd", "no": "Ikke kjørt", "pl": "Nie uruchomiono", "pt": "Não executado", "sv": "Ej körd", "zh": "未运行"},
  "easydrag.status.not_run_aria": {"cs": "Stav: nespuštěno", "da": "Status: ikke kørt", "de": "Status: nicht gelaufen", "el": "Κατάσταση: δεν εκτελέστηκε", "en": "Status: not run", "es": "Estado: no ejecutado", "fr": "Statut : non exécuté", "hi": "स्थिति: नहीं चला", "it": "Stato: non eseguito", "ja": "状態: 未実行", "nl": "Status: niet uitgevoerd", "no": "Status: ikke kjørt", "pl": "Stan: nie uruchomiono", "pt": "Estado: não executado", "sv": "Status: ej körd", "zh": "状态：未运行"},
  "easydrag.status.cancelled": {"cs": "Zrušeno", "da": "Annulleret", "de": "Abgebrochen", "el": "Ακυρώθηκε", "en": "Cancelled", "es": "Cancelado", "fr": "Annulé", "hi": "रद्द किया गया", "it": "Annullato", "ja": "キャンセル", "nl": "Geannuleerd", "no": "Avbrutt", "pl": "Anulowane", "pt": "Cancelado", "sv": "Avbruten", "zh": "已取消"},
  "easydrag.status.cancelled_aria": {"cs": "Stav: zrušeno na žádost, není to chyba", "da": "Status: annulleret efter anmodning, ikke en fejl", "de": "Status: auf Wunsch abgebrochen, kein Fehler", "el": "Κατάσταση: ακυρώθηκε κατόπιν αιτήματος, όχι αποτυχία", "en": "Status: cancelled on request, not a failure", "es": "Estado: cancelado a petición, no es un fallo", "fr": "Statut : annulé à la demande, pas un échec", "hi": "स्थिति: अनुरोध पर रद्द, यह विफलता नहीं है", "it": "Stato: annullato su richiesta, non è un errore", "ja": "状態: 要求によりキャンセル。失敗ではありません", "nl": "Status: op verzoek geannuleerd, geen fout", "no": "Status: avbrutt på forespørsel, ikke en feil", "pl": "Stan: anulowane na życzenie, to nie błąd", "pt": "Estado: cancelado a pedido, não é uma falha", "sv": "Status: avbruten på begäran, inget fel", "zh": "状态：应请求取消，并非失败"},
  "easydrag.ui.port_uncertain": {"cs": "Nejasné", "da": "Uklart", "de": "Unklar", "el": "Ασαφές", "en": "Unclear", "es": "Incierto", "fr": "Incertain", "hi": "अस्पष्ट", "it": "Incerto", "ja": "不明", "nl": "Onduidelijk", "no": "Uklart", "pl": "Niejasne", "pt": "Incerto", "sv": "Oklart", "zh": "不明确"}
}
```

- [ ] **Step 2: write the failing Node test**

Create `scripts/test-easydrag-status.mjs`:

```js
#!/usr/bin/env node
// W1a A (FD-08): the status map. Every value has a token, a glyph, a word key and an aria key (in all 16 locales);
// of() maps today's raw run, step, effect and flow values and sends an unknown one to "uncertain", never to "ok";
// disc() carries role="img" and the aria text; label() is the disc plus the word.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const box = vm.createContext({ window: {}, console });
vm.runInContext(fs.readFileSync(new URL('easydrag-status.js', apps), 'utf8'), box, { filename: 'easydrag-status.js' });
const S = box.window.EasyDrag.status;

const VALUES = ['ok', 'stopped', 'failed', 'running', 'waiting', 'uncertain', 'blocked', 'skipped', 'paused', 'simulated', 'not_run', 'cancelled'];
assert.deepEqual(Array.from(S.VALUES), VALUES, 'the twelve values in contract order');

const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const words = lang => JSON.parse(fs.readFileSync(new URL('../../../lang/easydrag/' + lang + '.json', apps), 'utf8'));
const TOKENS = new Set(['--ed-ok', '--ed-danger', '--ed-info', '--ed-warn', '--ed-status-neutral']);
for (const value of VALUES) {
    const row = S.table[value];
    assert.ok(row, value + ' has a table row');
    assert.ok(TOKENS.has(row.token), value + ' has a known token: ' + row.token);
    assert.ok(row.glyph.startsWith('<'), value + ' has glyph markup');
    assert.equal(row.word, 'easydrag.status.' + value);
    assert.equal(row.aria, 'easydrag.status.' + value + '_aria');
}
assert.deepEqual(['ok', 'stopped'].map(v => S.table[v].token), ['--ed-ok', '--ed-ok'], 'done is green');
assert.equal(S.table.failed.token, '--ed-danger');
assert.deepEqual(['running', 'waiting'].map(v => S.table[v].token), ['--ed-info', '--ed-info'], 'under way is blue');
assert.deepEqual(['uncertain', 'blocked'].map(v => S.table[v].token), ['--ed-warn', '--ed-warn'], 'look here is amber');
assert.deepEqual(['skipped', 'paused', 'simulated', 'not_run', 'cancelled'].map(v => S.table[v].token), Array(5).fill('--ed-status-neutral'), 'did not happen is grey');

for (const lang of LOCALES) {
    const w = words(lang);
    for (const value of VALUES) {
        for (const key of [S.table[value].word, S.table[value].aria]) assert.ok(typeof w[key] === 'string' && w[key].trim(), lang + ' has ' + key);
    }
    assert.equal(w['easydrag.ui.port_uncertain'], w['easydrag.status.uncertain'], lang + ': one word for unclear');
    assert.equal(new Set(VALUES.map(v => w['easydrag.status.' + v])).size, VALUES.length, lang + ': twelve different words');
}

// of(): today's raw values
const cases = [
    ['success', 'run', 'ok'], ['error', 'run', 'failed'], ['running', 'run', 'running'], ['queued', 'run', 'waiting'], ['waiting', 'run', 'waiting'], ['cancelled', 'run', 'cancelled'],
    ['success', 'step', 'ok'], ['error', 'step', 'failed'], ['running', 'step', 'running'], ['pending', 'step', 'not_run'], ['skipped', 'step', 'skipped'], ['waiting', 'step', 'waiting'],
    ['blocked', 'run', 'blocked'], ['approval_pending', 'run', 'blocked'], ['needs_setup', 'step', 'blocked'], ['simulated', 'step', 'simulated'],
    ['effect_uncertain', 'effect', 'uncertain'], ['uncertain', 'effect', 'uncertain'], [true, 'effect', 'uncertain'], [false, 'effect', 'ok'],
    ['paused', 'flow', 'paused'], ['disabled', 'flow', 'paused'], ['off', 'flow', 'paused']
];
for (const [raw, kind, want] of cases) assert.equal(S.of(raw, kind), want, kind + ' ' + String(raw));
for (const [raw, kind] of [['bogus', 'run'], ['', 'step'], [undefined, 'run'], [null, 'effect'], ['pending', 'run'], ['constructor', 'step'], ['__proto__', 'flow'], [7, 'step'], ['success', 'flow']]) {
    assert.equal(S.of(raw, kind), 'uncertain', 'an unknown raw value is uncertain, never ok: ' + String(raw) + ' as ' + kind);
}
assert.equal(S.ofStep({ status: 'success' }, true), 'simulated', 'a step that ran in a simulation');
assert.equal(S.ofStep({ status: 'error' }, true), 'failed', 'a failed simulated step stays failed');
assert.equal(S.ofStep({ status: 'success' }, false), 'ok');
assert.equal(S.ofStep(null, false), 'uncertain');

// disc(), word(), aria(), label() with the real en texts
const en = words('en');
const t = key => en[key] || key;
const disc = S.disc('ok', 20, { t });
assert.ok(disc.startsWith('<span class="ed-status-disc" data-status="ok" role="img" aria-label="Status: succeeded"'), disc);
assert.ok(disc.includes('--ed-disc:20px') && disc.includes('<svg') && disc.includes('<path'), 'size and glyph');
assert.ok(S.disc('ok', undefined, { t }).includes('--ed-disc:18px'), 'the default size is 18');
assert.ok(S.disc('nonsense', 18, { t }).includes('data-status="uncertain"'), 'an unknown value draws uncertain');
assert.ok(S.disc('ok', 18, { t, title: 'a "b"' }).includes('title="a &quot;b&quot;"'), 'the title is escaped');
assert.ok(S.disc('ok', 18, { t, hidden: true }).includes('aria-hidden="true"') && S.disc('ok', 18, { t, hidden: true }).includes('role="img"'));
assert.equal(S.word(t, 'stopped'), 'Ended by stop');
assert.equal(S.aria(t, 'cancelled'), 'Status: cancelled on request, not a failure');
assert.equal(S.word(key => key, 'failed'), 'failed', 'a missing text falls back to the value name');
const label = S.label(t, 'failed');
assert.ok(label.startsWith('<span class="ed-status-label" data-status="failed">') && label.includes('<span class="ed-status-word">Failed</span>') && label.includes('aria-hidden="true"'), label);
assert.ok(S.label(t, 'ok', '<b>x</b>').includes('&lt;b&gt;x&lt;/b&gt;'), 'a word override is escaped');
S.bind(t);
assert.ok(S.disc('waiting', 14).includes('aria-label="Status: waiting"'), 'a bound translator serves disc() without t');
console.log('PASS easydrag status map');
```

- [ ] **Step 3: run it and see it fail**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && node scripts/test-easydrag-status.mjs 2>&1 | head -5
```
Expected: an `ENOENT` error naming `easydrag-status.js`.

- [ ] **Step 4: write the module**

Create `ui/js/desktop/apps/easydrag-status.js`:

```js
// EasyDrag status map (W1a A, FD-08): every run, step, effect and flow status maps to one of twelve values, and a value has one token, one glyph, one word and one aria sentence. Surfaces draw a status only through this module.
(function () {
    'use strict';
    const ED = window.EasyDrag = window.EasyDrag || {};
    const VALUES = ['ok', 'stopped', 'failed', 'running', 'waiting', 'uncertain', 'blocked', 'skipped', 'paused', 'simulated', 'not_run', 'cancelled'];
    // Glyphs are 16x16 line drawings in currentColor (the owner-approved mock). The disc sets the colour.
    const SQUARE = '<rect x="4.8" y="4.8" width="6.4" height="6.4" rx="1" fill="currentColor"/>';
    const GLYPHS = {
        ok: '<path d="M3.5 8.5l3 3 6-7"/>',
        stopped: SQUARE,
        failed: '<path d="M4.5 4.5l7 7M11.5 4.5l-7 7"/>',
        running: '<path d="M8 2.5a5.5 5.5 0 1 1-5.5 5.5"/>',
        waiting: '<circle cx="8" cy="8" r="5.5"/><path d="M8 5v3.2l2.2 1.4"/>',
        uncertain: '<path d="M5.8 6a2.3 2.3 0 1 1 3.2 2.1c-.7.3-1 .8-1 1.5v.4"/><circle cx="8" cy="12.3" r=".6" fill="currentColor"/>',
        blocked: '<circle cx="8" cy="8" r="5.5"/><path d="M4.2 11.8l7.6-7.6"/>',
        skipped: '<path d="M3 8h7M7.5 5l3 3-3 3M13 4.5v7"/>',
        paused: '<path d="M6 4.5v7M10 4.5v7"/>',
        simulated: '<path d="M8 2.8l5.2 5.2L8 13.2 2.8 8z"/>',
        not_run: '<circle cx="8" cy="8" r="5.2" stroke-dasharray="2 2"/>',
        cancelled: SQUARE
    };
    const TOKENS = { ok: '--ed-ok', stopped: '--ed-ok', failed: '--ed-danger', running: '--ed-info', waiting: '--ed-info', uncertain: '--ed-warn', blocked: '--ed-warn' };
    const table = {};
    VALUES.forEach(value => {
        table[value] = Object.freeze({ token: TOKENS[value] || '--ed-status-neutral', glyph: GLYPHS[value], word: 'easydrag.status.' + value, aria: 'easydrag.status.' + value + '_aria' });
    });
    Object.freeze(table);

    // Today's raw values (internal/flows/run_types.go; durable states in easydrag-durable.js). A raw value that no map knows is
    // "uncertain": an unknown status must never read as a success.
    const RAW = {
        run: { queued: 'waiting', running: 'running', waiting: 'waiting', success: 'ok', error: 'failed', cancelled: 'cancelled' },
        step: { pending: 'not_run', running: 'running', waiting: 'waiting', success: 'ok', error: 'failed', skipped: 'skipped', cancelled: 'cancelled' },
        effect: { effect_uncertain: 'uncertain', uncertain: 'uncertain', unknown: 'uncertain' },
        flow: { paused: 'paused', disabled: 'paused', inactive: 'paused', off: 'paused' }
    };
    // Raw values that mean the same in every kind: an open approval or a missing setup (blocked), a step that ran in Simulation.
    const ANY = { blocked: 'blocked', approval_pending: 'blocked', needs_setup: 'blocked', simulated: 'simulated', effect_uncertain: 'uncertain' };
    const own = (object, key) => Object.prototype.hasOwnProperty.call(object, key);
    let bound = null;

    function bind(t) { bound = typeof t === 'function' ? t : null; }
    function known(value) { return typeof value === 'string' && own(table, value) ? value : 'uncertain'; }
    function of(raw, kind) {
        if (raw === false && kind === 'effect') return 'ok';
        const key = raw === true ? 'uncertain' : typeof raw === 'string' ? raw : '';
        const maps = RAW[kind] ? [RAW[kind]] : [RAW.run, RAW.step];
        for (const map of maps) if (own(map, key)) return map[key];
        if (own(ANY, key)) return ANY[key];
        return own(table, key) && kind !== 'flow' ? key : 'uncertain';
    }
    // ofStep is the value of a step record; a step that succeeded in a simulation is "simulated".
    function ofStep(step, simulation) {
        const value = of(step && step.status, 'step');
        return simulation && value === 'ok' ? 'simulated' : value;
    }
    const escape = text => String(text).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;');
    function translate(t, key) {
        const fn = typeof t === 'function' ? t : bound;
        const text = fn ? fn(key) : '';
        return text && text !== key ? text : '';
    }
    function word(t, value) { const v = known(value); return translate(t, table[v].word) || v; }
    function aria(t, value) { const v = known(value); return translate(t, table[v].aria) || word(t, v); }
    // disc(value, size, opts): opts.t translates, opts.title adds a tooltip, opts.hidden hides the disc from screen readers
    // (label() does that: its word already says it).
    function disc(value, size, opts) {
        const v = known(value), o = opts || {};
        const px = Number.isFinite(size) && size > 0 ? Math.round(size) : 18;
        return '<span class="ed-status-disc" data-status="' + v + '" role="img" aria-label="' + escape(aria(o.t, v)) + '"' + (o.hidden ? ' aria-hidden="true"' : '') + (o.title ? ' title="' + escape(o.title) + '"' : '') +
            ' style="--ed-disc:' + px + 'px"><svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">' + table[v].glyph + '</svg></span>';
    }
    // label(t, value, text): the disc plus the word; text replaces the word when a surface has a more exact one ("Stoppt…").
    function label(t, value, text) {
        const v = known(value);
        return '<span class="ed-status-label" data-status="' + v + '">' + disc(v, 14, { t, hidden: true }) + '<span class="ed-status-word">' + escape(text || word(t, v)) + '</span></span>';
    }

    ED.status = { VALUES: Object.freeze(VALUES.slice()), table, of, ofStep, disc, word, aria, label, bind };
})();
```

- [ ] **Step 5: register the module (four places)**

Each edit puts `easydrag-status.js` right after `easydrag-core.js`.

1. `ui/js/desktop/core/module-loader.js`: replace
```
                '/js/desktop/apps/easydrag-core.js',
                '/js/desktop/apps/easydrag-durable.js',
```
with
```
                '/js/desktop/apps/easydrag-core.js',
                '/js/desktop/apps/easydrag-status.js',
                '/js/desktop/apps/easydrag-durable.js',
```
2. `ui/desktop_easydrag_test.go:12`: replace `"easydrag-core.js", "easydrag-durable.js",` with `"easydrag-core.js", "easydrag-status.js", "easydrag-durable.js",`.
3. `scripts/test-easydrag-extra2.mjs:9`, `scripts/test-easydrag-extra3.mjs:9`, `scripts/test-easydrag-extra4.mjs:12`: replace `'easydrag-core.js', 'easydrag-durable.js',` with `'easydrag-core.js', 'easydrag-status.js', 'easydrag-durable.js',` (once per file).
4. Prove no other list was missed:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && grep -rln "easydrag-durable.js" scripts ui/*.go ui/js/desktop/core | while read f; do printf '%s status=%s\n' "$f" "$(grep -c "easydrag-status.js" "$f")"; done
```
Expected: five files, each `status=1` (`module-loader.js`, `desktop_easydrag_test.go`, the three `extra*.mjs`).

- [ ] **Step 6: the disc CSS**

In `ui/css/desktop-app-easydrag.css`, replace the single line added in A.1 (`.ed-status-disc { box-shadow: 0 0 0 1px var(--ed-status-edge); }`) with the family. The old `.ed-status*` rules stay until A.3 removes them.

```
/* status map (W1a A): ED.status draws one disc per value; the word beside it is plain text. Colour never stands alone: every value has its own glyph. */
.ed-status-disc { display: inline-grid; place-items: center; flex: none; box-sizing: border-box; width: var(--ed-disc, 18px); height: var(--ed-disc, 18px); border-radius: 50%; vertical-align: middle; background: var(--ed-status-neutral); color: var(--ed-status-neutral-ink); box-shadow: 0 0 0 1px var(--ed-status-edge); }
.ed-status-disc svg { display: block; width: 72%; height: 72%; }
.ed-status-disc[data-status="ok"], .ed-status-disc[data-status="stopped"] { background: var(--ed-ok); color: var(--ed-status-ink); }
.ed-status-disc[data-status="failed"] { background: var(--ed-danger); color: var(--ed-status-ink); }
.ed-status-disc[data-status="running"], .ed-status-disc[data-status="waiting"] { background: var(--ed-info); color: var(--ed-status-ink); }
.ed-status-disc[data-status="uncertain"], .ed-status-disc[data-status="blocked"] { background: var(--ed-warn); color: var(--ed-status-ink); }
.ed-status-label { display: inline-flex; align-items: center; gap: var(--ed-space-2); vertical-align: middle; }
.ed-status-word { color: var(--ed-text); }
.ed-node-badges .ed-status-disc { box-shadow: 0 0 0 2px var(--ed-panel-strong), 0 0 0 3px var(--ed-status-edge); }
.ed-last-mark { display: inline-flex; }
```
Then, inside the `@media (forced-colors: active) {` block (line 1531), add after the old `.ed-status--running, .ed-status--queued { outline-color: Highlight; }` line (A.3 removes the old lines):

```
    /* status discs are told apart by glyph, fill and ring, not by hue */
    .ed-status-disc { background-color: Canvas; color: CanvasText; outline: 2px solid CanvasText; outline-offset: -2px; box-shadow: none; }
    .ed-status-disc[data-status="running"], .ed-status-disc[data-status="waiting"] { color: Highlight; outline-color: Highlight; }
    .ed-status-disc[data-status="skipped"], .ed-status-disc[data-status="paused"], .ed-status-disc[data-status="simulated"], .ed-status-disc[data-status="not_run"], .ed-status-disc[data-status="cancelled"] { color: GrayText; outline-color: GrayText; }
```
No pulse on `running`: Part D owns animation rules and keys them to `[data-motion="normal"]`.

- [ ] **Step 7: apply the keys, run the tests**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && node C:/disposable/easydrag-w1a/locale-apply.mjs C:/disposable/easydrag-w1a/keys/a2.json && node scripts/test-easydrag-status.mjs && git diff --stat -- ui/lang | tail -1
```
Expected: `keys set, 0 deleted, in 16 files`, then `PASS easydrag status map`, then `16 files changed`. Run it twice: the second `git diff --stat` must not change (the helper is idempotent).

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go test ./ui -run '(?i)(TestEasyDragAssetsLoadInDependencyOrder|TestEasyDragUIKeysExistInAllLocales|TestEasyDragStylesheetUsesThemeTokens|TestEasyDragStatusColourTokens|TestEasyDragModuleLines)' -count=1 2>&1 | grep -v '^{' | tail -3 && GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go test ./internal/flows -run 'TestCatalogTranslationsAreComplete' -count=1 2>&1 | tail -2
```
Expected: `ok  	aurago/ui` and `ok  	aurago/internal/flows`.

- [ ] **Step 8: gates and commit**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && node scripts/test-easydrag.mjs 2>&1 | tail -2 && node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1 && gofmt -l ui/desktop_easydrag_test.go
```
Expected: `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, no gofmt output.

Message `C:/disposable/easydrag-w1a/msg-a2.txt`:

```
feat(easydrag): status map module with the twelve values

ED.status maps run, step, effect and flow raw values to twelve status values
(unknown raw values give "uncertain", never "ok") and draws them as a disc
with a glyph, a word and an aria sentence. Keys easydrag.status.<value> and
_aria in all 16 locales; easydrag.ui.port_uncertain now uses the same word as
the status. The disc CSS family, the forced-colors rule and the module
registration (loader, Go list, three Node harness lists) come with it. No
surface uses the module yet.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && git add ui/js/desktop/apps/easydrag-status.js scripts/test-easydrag-status.mjs ui/css/desktop-app-easydrag.css ui/js/desktop/core/module-loader.js ui/desktop_easydrag_test.go scripts/test-easydrag-extra2.mjs scripts/test-easydrag-extra3.mjs scripts/test-easydrag-extra4.mjs ui/lang/easydrag && git commit -F C:/disposable/easydrag-w1a/msg-a2.txt
```

---

### Task A.3: Canvas and detail

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-canvas.js:403, 421, 436-438` (no line may be added)
- Modify: `ui/js/desktop/apps/easydrag-detail.js:274, 613, 619` (existing lines only)
- Modify: `ui/css/desktop-app-easydrag.css` (the old `.ed-status` and `.ed-status-pill` families)
- Modify: `scripts/test-easydrag-extra6.mjs:384-393`, `ui/desktop_easydrag_browser_test.go:816`

- [ ] **Step 1: IMPACT and the survey for these files**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && for s in badgesMarkup cardName paintCard; do node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$s" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo 2>&1 | grep -E '"(risk|impactedCount|message)"'; done; wc -l ui/js/desktop/apps/easydrag-canvas.js ui/js/desktop/apps/easydrag-detail.js
```
Expected: LOW or MEDIUM for all three (callers are inside canvas.js: `paintState`, `renderNode`). A HIGH or CRITICAL result: tell the coordinator before editing. If a result is `UNKNOWN`, confirm with `grep -n "badgesMarkup\|cardName(" ui/js/desktop/apps/easydrag-canvas.js`; both are local functions called only from this file. Record the line counts: `928` and `878`.

- [ ] **Step 2: canvas edits (same line count)**

1. `easydrag-canvas.js:403` (the card signature takes the run mode, because a simulation draws `simulated`). Replace
`step && !!step.effect_uncertain, issues]);` with `step && !!step.effect_uncertain, ed.run && ed.run.mode, issues]);`.
2. `:421` (`cardName`). Replace
`if (status) parts.push(core.tr(t, 'easydrag.ui.status_' + status, status));`
with
`if (status) parts.push(ED.status.word(t, ED.status.ofStep({ status }, !!ed.run && ed.run.mode === 'simulation')));`
3. `:436-438` (`badgesMarkup`, three lines become three lines). Replace
```
                const label = core.tr(t, 'easydrag.ui.status_' + status, status);
                const dur = step && step.duration_ms ? ' · ' + core.fmt.duration(step.duration_ms) : '';
                badges.push('<span class="ed-status ed-status--' + esc(status) + '" title="' + esc(label + dur) + '"><span class="ed-sr-only">' + esc(label) + '</span></span>');
```
with
```
                const value = ED.status.ofStep({ status }, !!ed.run && ed.run.mode === 'simulation');
                const dur = step && step.duration_ms ? ' · ' + core.fmt.duration(step.duration_ms) : '';
                badges.push(ED.status.disc(value, 18, { t, title: ED.status.word(t, value) + dur }));
```
4. `:515` (`classesFor`, `if (status) cls.push('status-' + status);`) stays: it is the card ring (lint allow-list, Task A.7).

Check the three scopes have `ed`: `grep -n "ed.run" ui/js/desktop/apps/easydrag-canvas.js | head -3` must show uses inside the same factory (line 383 `ed.run` is read by `statusOf`).

Prove the size did not grow:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && wc -l < ui/js/desktop/apps/easydrag-canvas.js && grep -n "status_\|ed-status" ui/js/desktop/apps/easydrag-canvas.js | grep -v "ED.status\|ed-status-disc"
```
Expected: `928`, and no output from the grep.

- [ ] **Step 3: detail edits (same line count)**

1. `easydrag-detail.js:274`. Replace
`el.querySelector('.ed-detail-status').innerHTML = step ? '<span class="ed-status-pill ed-status-pill--' + esc(step.status) + '">' + esc(core.tr(t, 'easydrag.ui.status_' + step.status, step.status)) + '</span>' : '';`
with
`el.querySelector('.ed-detail-status').innerHTML = step ? ED.status.label(t, ED.status.ofStep(step, !!ed.run && ed.run.mode === 'simulation')) : '';`
2. `:613`. Replace
`const parts = ['<span class="ed-status-pill ed-status-pill--' + esc(step.status) + '">' + esc(core.tr(t, 'easydrag.ui.status_' + step.status, step.status)) + '</span>'];`
with
`const parts = [ED.status.label(t, ED.status.ofStep(step, !!simulation))];`
3. `:619` (the effect chip: the word is the status word, the long sentence stays as the tooltip; the chip keeps the muted class that `:614` already uses). Replace
`if (step.effect_uncertain) parts.push('<span class="ed-chip ed-chip--warn" title="' + esc(t('easydrag.ui.step_effect_uncertain_hint')) + '">' + esc(t('easydrag.ui.step_effect_uncertain')) + '</span>');`
with
`if (step.effect_uncertain) parts.push('<span class="ed-chip ed-chip--muted" title="' + esc(t('easydrag.ui.step_effect_uncertain_hint')) + '">' + ED.status.label(t, ED.status.of('effect_uncertain', 'effect')) + '</span>');`

Verify `simulation` is in scope at 613 (it is read on the next line) and the counts:
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && sed -n 560,612p ui/js/desktop/apps/easydrag-detail.js | grep -n "simulation" | head -3; wc -l < ui/js/desktop/apps/easydrag-detail.js; grep -n "ed-status-pill\|status_" ui/js/desktop/apps/easydrag-detail.js | grep -v "ED.status"
```
Expected: at least one `simulation` line, `878`, no old form. If `simulation` is not declared above 613 in that function, declare nothing new; use `!!(ed.run && ed.run.mode === 'simulation')` instead.

- [ ] **Step 4: remove the old CSS families, update the tests that select them**

In `ui/css/desktop-app-easydrag.css` delete these lines (text-anchored, the line numbers drift):
```
.ed-status { width: 18px; height: 18px; border-radius: 50%; box-shadow: 0 0 0 2px var(--ed-panel-strong); background: var(--ed-border-strong); position: relative; }
.ed-status--success { background: var(--ed-ok); }
.ed-status--success::after { content: ""; position: absolute; left: 6px; top: 3px; width: 5px; height: 9px; border: solid white; border-width: 0 2px 2px 0; transform: rotate(45deg); }
.ed-status--error { background: var(--ed-danger); }
.ed-status--error::after { content: "!"; position: absolute; inset: 0; display: grid; place-items: center; color: white; font: 800 12px/1 sans-serif; }
.ed-status--running, .ed-status--queued { background: var(--ed-info); animation: ed-pulse 1s ease-in-out infinite; }
.ed-status--skipped { background: var(--ed-control); box-shadow: 0 0 0 2px var(--ed-panel-strong), inset 0 0 0 1px var(--ed-border-strong); }
```
```
.ed-status-pill { display: inline-flex; align-items: center; min-height: 20px; padding: 1px 8px; border-radius: 999px; font-size: 11px; font-weight: 700; background: var(--ed-control); color: var(--ed-muted); }
.ed-status-pill--success { background: color-mix(in srgb, var(--ed-ok) 16%, transparent); color: var(--ed-ok-ink); }
.ed-status-pill--error { background: color-mix(in srgb, var(--ed-danger) 16%, transparent); color: var(--ed-danger-ink); }
.ed-status-pill--running, .ed-status-pill--queued { background: color-mix(in srgb, var(--ed-info) 16%, transparent); color: var(--ed-info-ink); }
```
and, in the forced-colors block, the two lines
```
    .ed-status { outline: 2px solid CanvasText; outline-offset: -2px; }
    .ed-status--running, .ed-status--queued { outline-color: Highlight; }
```
Then update the tests:

- `scripts/test-easydrag-extra6.mjs`: line 384 `card.querySelector('.ed-status')` becomes `card.querySelector('.ed-status-disc')`. Lines 391-393 expected titles change, because the harness `t` returns the key unchanged and `word()` falls back to the value name: `'status_running'` to `'running'`, `'status_success · 1.2 s'` to `'ok · 1.2 s'`, `'status_error · 900 ms'` to `'failed · 900 ms'`.
- `scripts/test-easydrag-ux-batch5.mjs:664`: the card name ends in `status_error`; it is now `failed`: `'Alpha, card_state_off, badge_incomplete, status_error'` becomes `'Alpha, card_state_off, badge_incomplete, failed'`.
- `ui/desktop_easydrag_browser_test.go:816`: `sel.querySelector('.ed-status')` becomes `sel.querySelector('.ed-status-disc')`.

Run the proof greps and the tests (`ux-batch5:665` is the home-grid name, which changes in A.4):
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && grep -n "ed-status--\|ed-status-pill\|\.ed-status[ {,]" ui/css/desktop-app-easydrag.css; node scripts/test-easydrag.mjs 2>&1 | grep -E "FAIL|all EasyDrag" | head; node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1
```
Expected: no CSS output, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`. If a FAIL names `ux-batch5` line 665 text, it is the home-grid name: do its edit now with A.4 Step 4 (item 3) and re-run.

- [ ] **Step 5: commit**

Message `C:/disposable/easydrag-w1a/msg-a3.txt`:

```
refactor(easydrag): canvas and detail draw statuses through ED.status

The card badge, the card's accessible name, the detail status pill and the
effect chip use ED.status (a step that succeeded in a simulation draws
"simulated"). canvas.js keeps its 928 lines, detail.js its 878. The
.ed-status and .ed-status-pill CSS families and their forced-colors rules go;
the card ring (.ed-node.status-*) stays as the card's state border.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && git add ui/js/desktop/apps/easydrag-canvas.js ui/js/desktop/apps/easydrag-detail.js ui/css/desktop-app-easydrag.css scripts/test-easydrag-extra6.mjs scripts/test-easydrag-ux-batch5.mjs ui/desktop_easydrag_browser_test.go && git commit -F C:/disposable/easydrag-w1a/msg-a3.txt
```

---

### Task A.4: Run views: durable, testing, run drawer, header pill, home grid (and `stopped`)

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-durable.js:35-37, 146`
- Modify: `ui/js/desktop/apps/easydrag-testing.js:284-300, 312-335`
- Modify: `ui/js/desktop/apps/easydrag-run-drawer.js:29-30, 40, 193-195`
- Modify: `ui/js/desktop/apps/easydrag-editor.js:80, 155, 160` (existing lines only)
- Modify: `ui/js/desktop/apps/easydrag-header.js:207, 210`
- Modify: `ui/js/desktop/apps/easydrag-home-grid.js:50, 138`
- Modify: `ui/js/desktop/apps/easydrag.js` (one `bind` line after line 225)
- Modify: `ui/css/desktop-app-easydrag.css` (the `.ed-run-dot` family)
- Modify: `scripts/test-easydrag-stop-hint.mjs`, `scripts/test-easydrag-ux-batch5.mjs:665`, `ui/desktop_easydrag_browser_test.go:847`

**What changes in the design.** `ED.durable.value(run)` gives the status value of a run (durable wait states become `waiting` or `blocked`, an open approval is `blocked`). `ED.durable.label` still returns the word text, with the transient "Stoppt…" kept. `ED.testing.statusOf(ed, run)` adds `stopped` for a successful run a `logic.stop` ended, and `ED.testing.statusLabel(t, ed, run)` is its word. The S0c hint paragraph and the banner suffix go: the status itself says "Durch Stopp beendet", and the stop step's name is the `title` (tooltip) of the banner text and of the test panel summary, from the kept key `easydrag.ui.run_stopped_by`. An open approval used to read "Genehmigung ausstehend" in run rows; it reads "Blockiert" now (owner decision 5), and the durable section still shows the reason.

- [ ] **Step 1: IMPACT**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && for s in label renderLastRun runRow applyRunView renderBanner lastRun createResults stoppedBy; do echo "== $s"; node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "$s" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo 2>&1 | grep -E '"(risk|impactedCount|message)"'; done
```
Expected: LOW to MEDIUM, or `ambiguous`/`UNKNOWN` for the same-named functions (`label`, `lastRun`, `runRow`). Confirm those with `grep -rn "ED.durable.label\|durable.label(" ui/js/desktop/apps/easydrag-*.js`: callers are the five files above plus `easydrag-testing.js:272`. HIGH or CRITICAL: warn the coordinator first. Record `wc -l ui/js/desktop/apps/easydrag-editor.js` (896).

- [ ] **Step 2: write the failing test**

Extend `scripts/test-easydrag-stop-hint.mjs`: it already loads the modules in a VM (read the top of the file and reuse its `box`). Append, before its final `console.log`, a check of the two new functions. The VM must also load `easydrag-status.js` and `easydrag-durable.js`; add them to its module list in the same order as the loader (core, status, durable, ...). Then:

```js
const T = box.window.EasyDrag.testing, D = box.window.EasyDrag.durable;
const stopRunFull = { id: 'r1', status: 'success', steps: new Map([[STOP, step(STOP, 'success')]]), record: { id: 'r1', status: 'success' } };
assert.equal(T.statusOf({ model }, stopRunFull), 'stopped', 'a successful run a stop step ended is "stopped"');
assert.equal(T.statusOf({ model }, { id: 'r2', status: 'error', steps: new Map([[STOP, step(STOP, 'success')]]), record: { id: 'r2', status: 'error' } }), 'failed', 'an error run is not "stopped"');
assert.equal(T.statusOf({ model }, { id: 'r3', status: 'cancelled', steps: new Map(), record: { id: 'r3', status: 'cancelled' } }), 'cancelled');
assert.equal(T.statusOf({ model }, { id: 'r4', status: 'success', steps: new Map(), record: { id: 'r4', status: 'success' } }), 'ok');
assert.equal(D.value({ id: 'r5', status: 'running', record: { id: 'r5', status: 'running' } }), 'running');
assert.equal(D.value({ id: 'r6', status: 'queued', record: { id: 'r6', status: 'queued' } }), 'waiting');
assert.equal(T.statusLabel(key => key, { model }, stopRunFull), 'stopped', 'the word of a stopped run (the harness t returns the key)');
```
Check the helper names in that file (`step`, `STOP`, `model`) against its first 25 lines before pasting; use the names the file really has.

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && node scripts/test-easydrag-stop-hint.mjs 2>&1 | head -4
```
Expected: `TypeError: T.statusOf is not a function`.

- [ ] **Step 3: durable.js and testing.js**

`easydrag-durable.js`:

1. Replace the two `return` statements of `label` (lines 36-37, text-anchored). The old line 36 starts `if (!['success', 'error', 'cancelled'].includes(r.status) && s && s.state !== 'running') return t(s.state === 'approval_pending' ? 'easydrag.ui.approval_state_pending' : s.state === 'blocked' ? 'easydrag.ui.durable_blocked' : 'easydrag.ui.durable_waiting');`; replace it with
```
        if (!['success', 'error', 'cancelled'].includes(r.status) && s && s.state !== 'running') return ED.status.word(t, value(run));
```
and replace `return ED.core.tr(t, 'easydrag.ui.status_' + r.status, r.status);` with `return ED.status.word(t, value(run));`.
2. Add `value` directly above `function label(t, run) {`:
```js
    // value is the status value of a run (ED.status): a legacy checkpoint cannot override a finished run, and an open approval is "blocked".
    function value(run) {
        const r = record(run), s = info(run);
        if (r && !TERMINAL_STATES.has(r.status) && s && s.state !== 'running') return ED.status.of(s.state === 'running' ? 'running' : s.state === 'waiting' ? 'waiting' : 'blocked', 'run');
        return ED.status.of(r && r.status, 'run');
    }
```
3. Export it: in the line `ED.durable = { TYPE, REASONS, capability, status, record, info, has, label, reason, markup, supported, requires, observeRun, observe };` insert `value,` after `label,`.

(The keys `durable_blocked` and `durable_waiting` are no longer read by this file. Leave them in the locales; they are not status words of the `status_` family. Task A.6 greps them.)

`easydrag-testing.js`:

1. Replace line 284 (the summary text):
`el.querySelector('summary').textContent = (simulation ? t('easydrag.ui.simulation_result') : t('easydrag.ui.run_diagnosis')) + ' · ' + core.tr(t, 'easydrag.ui.status_' + run.status, run.status);`
with
`el.querySelector('summary').textContent = (simulation ? t('easydrag.ui.simulation_result') : t('easydrag.ui.run_diagnosis')) + ' · ' + statusLabel(t, ed, run);`
2. Line 286 keeps its `stoppedBy` call and gains the tooltip: replace `const stopped = stoppedBy({ run, steps }, ed.model);` with `const stopped = stoppedBy({ run, steps }, ed.model); el.querySelector('summary').title = stopped ? t('easydrag.ui.run_stopped_by', { step: stopped.label }) : '';` (the stop step's name is the tooltip of the status). Delete line 293 `(stopped ? '<p class="ed-hint" data-ed-stopped-by>' + esc(t('easydrag.ui.run_stopped_by', { step: stopped.label })) + '</p>' : '') +`.
3. Replace the timeline cell in line 300. Old:
`<span class="ed-run-dot ed-run-dot--' + esc(step.status) + '"></span><span>' + esc(label(step)) + '</span><span class="ed-muted">' + esc(core.tr(t, 'easydrag.ui.status_' + step.status, step.status)) + (step.duration_ms ? ' · ' + esc(core.fmt.duration(step.duration_ms)) : '') + (step.effect_uncertain ? ' · ' + esc(t('easydrag.ui.step_effect_uncertain')) : '') + '</span></button></li>'`
New:
`' + ED.status.disc(ED.status.ofStep(step, simulation), 14, { t, hidden: true }) + '<span>' + esc(label(step)) + '</span><span class="ed-muted">' + esc(ED.status.word(t, ED.status.ofStep(step, simulation))) + (step.duration_ms ? ' · ' + esc(core.fmt.duration(step.duration_ms)) : '') + (step.effect_uncertain ? ' · ' + esc(ED.status.word(t, 'uncertain')) : '') + '</span></button></li>'`
(the old text begins right after `data-ed-result-node="' + esc(step.node_id) + '">`; the new text replaces everything from the `<span class="ed-run-dot` up to the end of that map callback string as shown. Keep the quotes balanced: the new string starts with `'` + `+` joins as written).
4. Add the two functions after `stoppedBy` (before the `ED.testing = {` line) and export them:
```js
    // statusOf is the status value of a run (ED.status): the durable and plain run statuses, and "stopped" for a successful run a
    // logic.stop ended (S0c A2, W1a A). The status replaces S0c's separate hint text.
    function statusOf(ed, run) {
        const base = ED.durable ? ED.durable.value(run) : ED.status.of(run && (run.record || run).status, 'run');
        if (base !== 'ok' || !run) return base;
        const record = run.record || run;
        const steps = run.steps && typeof run.steps.values === 'function' ? Array.from(run.steps.values()) : [];
        return stoppedBy({ run: { status: record.status }, steps }, ed.model) ? 'stopped' : base;
    }
    // statusLabel is the word for statusOf; "stopping" and the durable wording stay with ED.durable.label.
    function statusLabel(t, ed, run) {
        return statusOf(ed, run) === 'stopped' ? ED.status.word(t, 'stopped') : ED.durable ? ED.durable.label(t, run) : ED.status.word(t, ED.status.of(run && (run.record || run).status, 'run'));
    }
```
and extend `ED.testing = { decorate, createResults, … localSample, stoppedBy };` with `, statusOf, statusLabel`.

- [ ] **Step 4: run drawer, banner, header, home grid**

`easydrag-run-drawer.js`:
1. Lines 29-30. Replace
```
            const row = '<button type="button" class="ed-run-row" data-ed-run="' + esc(r.id) + '"><span class="ed-run-dot ed-run-dot--' + esc(r.status) + '"></span>' +
                '<span class="ed-run-main"><span>' + esc(ED.durable ? ED.durable.label(t, r) : core.tr(t, 'easydrag.ui.status_' + r.status, r.status)) + ' · ' + esc(triggerLabel(r)) + '</span>' +
```
with
```
            const row = '<button type="button" class="ed-run-row" data-ed-run="' + esc(r.id) + '">' +
                '<span class="ed-run-main"><span>' + ED.status.label(t, ED.durable.value(r), ED.durable.label(t, r)) + ' · ' + esc(triggerLabel(r)) + '</span>' +
```
2. Line 40: replace `core.tr(t, 'easydrag.ui.status_' + r.status, r.status)` with `ED.durable.label(t, r)` (the filter matches the word the row shows).
3. Lines 193-195 (S0c's hint becomes a flag):
```
                // The hint of a run a stop step ended (S0c A2); renderBanner appends it to the banner text.
                const stopped = ED.testing && ED.testing.stoppedBy(detail, ed.model);
                ed.runView.stopHint = stopped ? t('easydrag.ui.run_stopped_by', { step: stopped.label }) : '';
```
becomes
```
                // A run a stop step ended shows "stopped" (S0c A2, W1a A); renderBanner reads it for the status word and the tooltip.
                const stopped = ED.testing && ED.testing.stoppedBy(detail, ed.model);
                ed.runView.stopped = stopped ? { label: stopped.label } : null;
```
(this document-aware check is more exact than `statusOf`, because `detail` carries the stored document).

`easydrag-editor.js` (existing lines only; 896 stays 896):
- `:80` replace `<span class="ed-run-dot" data-ed-last-dot></span>` with `<span class="ed-last-mark" data-ed-last-dot></span>`.
- `:155` replace `status: ED.durable ? ED.durable.label(t, ed.run || run) : core.tr(t, 'easydrag.ui.status_' + run.status, run.status),` with `status: ed.runView.stopped ? ED.status.word(t, 'stopped') : ED.durable.label(t, ed.run || run),`.
- `:160` replace `const shown = text + (ed.runView.stopHint ? ' · ' + ed.runView.stopHint : ''); if (node.textContent !== shown) node.textContent = shown;` with `const shown = text; if (node.textContent !== shown) node.textContent = shown; node.title = ed.runView.stopped ? t('easydrag.ui.run_stopped_by', { step: ed.runView.stopped.label }) : '';` (still one line; the step's name is the tooltip of the banner text). The run drawer's list rows carry no steps, so a row cannot know the stop step; the footer pill likewise has no title.

`easydrag-header.js`:
- `:207` replace `dot.className = 'ed-run-dot' + (r ? ' ed-run-dot--' + r.status : '');` with
`const mark = r ? ED.status.disc(ED.testing.statusOf(ed, ed.run || r), 14, { t, hidden: true }) : ''; if (dot.dataset.mark !== mark) { dot.dataset.mark = mark; dot.innerHTML = mark; }`
- `:210` replace `ED.durable ? ED.durable.label(t, ed.run || r) : core.tr(t, 'easydrag.ui.status_' + r.status, r.status)` with `ED.testing.statusLabel(t, ed, ed.run || r)`.

`easydrag-home-grid.js`:
- `:50` replace
`'<span class="ed-run-dot ed-run-dot--' + esc(r.status) + '"></span><span>' + esc(ED.durable ? ED.durable.label(t, r) : core.tr(t, 'easydrag.ui.status_' + r.status, r.status)) + ' · ' + esc(core.fmt.relative(r.started_at))`
with
`ED.status.label(t, ED.durable.value(r), ED.durable.label(t, r)) + '<span> · ' + esc(core.fmt.relative(r.started_at))`
(the rest of the line, `+ (r.durable ? …) + '</span>'`, closes the new `<span>`).
- `:138` replace `parts.push(t('easydrag.ui.status_error'))` with `parts.push(ED.status.word(t, 'failed'))`.

`easydrag.js` (the translator for `ED.status.disc(value, size)` calls that pass no `t`, such as the lab's): after the `const inst = { windowId, ctx, root, t: ctx.t, …};` line (225) add one line:
```
        if (ED.status) ED.status.bind(ctx.t);
```

- [ ] **Step 5: the old `.ed-run-dot` CSS and the tests that select it**

Delete from `ui/css/desktop-app-easydrag.css`:
```
.ed-run-dot { width: 8px; height: 8px; border-radius: 50%; flex: none; background: var(--ed-border-strong); }
.ed-run-dot--success { background: var(--ed-ok); }
.ed-run-dot--error { background: var(--ed-danger); }
.ed-run-dot--running, .ed-run-dot--queued { background: var(--ed-info); animation: ed-pulse 1.2s ease-in-out infinite; }
.ed-run-dot--cancelled { background: var(--ed-muted); }
```
and from the forced-colors block:
```
    /* state dots and badges are told apart by fill, shape and ring, not by hue */
    .ed-run-dot { border: 2px solid CanvasText; background-color: Canvas; }
    .ed-run-dot--success { background-color: CanvasText; }
    .ed-run-dot--error { background-color: CanvasText; border-radius: 1px; transform: rotate(45deg); }
    .ed-run-dot--running, .ed-run-dot--queued { border-color: Highlight; }
    .ed-run-dot--cancelled { border-color: GrayText; }
```
(the `@keyframes ed-pulse` stays: Part D owns keyframes.) Tests:
- `ui/desktop_easydrag_browser_test.go:847`: `page.MustElement(`.ed-run-row .ed-run-dot--error`).MustClick()` becomes `page.MustElement(`.ed-run-row .ed-status-disc[data-status="failed"]`).MustClick()`.
- `scripts/test-easydrag-ux-batch5.mjs:665`: `'Beta, state_published, state_inactive, status_error'` becomes `'Beta, state_published, state_inactive, failed'`.
- `scripts/test-easydrag-stop-hint.mjs`: the comment at the top says the hint is shown; change "names the logic.stop step that ended a successful run, and nothing else" is still right (the function stays); add one sentence: `// W1a A: the run shows the status "stopped" instead of a hint line; statusOf is checked at the end.`

- [ ] **Step 6: run, prove, gate**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && node scripts/test-easydrag-stop-hint.mjs 2>&1 | tail -2; node scripts/test-easydrag.mjs 2>&1 | grep -E "FAIL|all EasyDrag" | head; node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1; wc -l < ui/js/desktop/apps/easydrag-editor.js; grep -n "ed-run-dot\|status_\|stopHint" ui/js/desktop/apps/easydrag-{durable,testing,run-drawer,editor,header,home-grid}.js ui/css/desktop-app-easydrag.css | grep -v "home_bulk"
```
Expected: the stop-hint script ends with its usual pass line, `all EasyDrag checks passed`, `PASS render baseline (unchanged)`, `896`, and **no** output from the last grep. A FAIL in a Node check that reads `data-ed-stopped-by` or the banner text means a test still expects the hint: update it to the status word (the grep for `stoppedBy|stopHint|run_stopped_by|stopped-by` in `scripts/` showed only the stop-hint script on main).

- [ ] **Step 7: commit**

Message `C:/disposable/easydrag-w1a/msg-a4.txt`:

```
refactor(easydrag): run rows, header pill and test panel use ED.status; stopped replaces the stop hint

ED.durable.value and ED.testing.statusOf/statusLabel give a run's status
value; a successful run that a logic.stop ended shows "stopped" in the test
panel, the run view banner and the footer pill, which replaces S0c's separate
hint line (the stop step stays marked in the timeline). The run drawer rows,
the footer pill, the home grid and the test timeline draw discs. An open
approval reads "blocked" in run rows. editor.js keeps its 896 lines. The
.ed-run-dot CSS family and its forced-colors rules go.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && git add ui/js/desktop/apps/easydrag-durable.js ui/js/desktop/apps/easydrag-testing.js ui/js/desktop/apps/easydrag-run-drawer.js ui/js/desktop/apps/easydrag-editor.js ui/js/desktop/apps/easydrag-header.js ui/js/desktop/apps/easydrag-home-grid.js ui/js/desktop/apps/easydrag.js ui/css/desktop-app-easydrag.css scripts/test-easydrag-stop-hint.mjs scripts/test-easydrag-ux-batch5.mjs ui/desktop_easydrag_browser_test.go && git commit -F C:/disposable/easydrag-w1a/msg-a4.txt
```

---

### Task A.5: Compare, child history, batches, publish

**Files:**
- Modify: `ui/js/desktop/apps/easydrag-compare.js:67, 91`
- Modify: `ui/js/desktop/apps/easydrag-child-history.js:166, 172, 178`
- Modify: `ui/js/desktop/apps/easydrag-batches.js:74`
- Modify: `ui/js/desktop/apps/easydrag-publish.js:188`

- [ ] **Step 1: re-run the survey grep and confirm the list is complete**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && grep -n "status_'\|easydrag.ui.status_\|ed-status\|run-dot" ui/js/desktop/apps/easydrag*.js | grep -v "ED.status\|ed-status-disc" | cut -c1-110
```
Expected: exactly the sites below (compare 67 and 91, child-history 166, 172 and 178, batches 74, publish 188), plus the card ring `'status-' + status` in canvas.js, which does not match this grep. Any other hit is a surface the survey missed: migrate it the same way (`of()` with the right kind, then `label` for markup or `word` for a plain string) and add it to this task's commit.

- [ ] **Step 2: the edits (word only where the surface is a plain string)**

- `easydrag-compare.js:67` (an `<option>` cannot hold markup, so it takes the word). Replace `core.tr(t, 'easydrag.ui.status_' + record.status, record.status)` with `ED.status.word(t, ED.status.of(record.status, 'run'))`.
- `easydrag-compare.js:91`. Replace `return core.tr(t, 'easydrag.ui.status_' + data, data);` with `return ED.status.word(t, ED.status.of(data, 'step'));`.
- `easydrag-child-history.js:166` (the frame row). Replace
`'<span>' + esc(callName(row) + ' · ' + (row.status ? core.tr(t, 'easydrag.ui.status_' + row.status, row.status) : t('easydrag.ui.child_history_unknown_outcome'))) + '</span></button>'`
with
`'<span>' + esc(callName(row) + ' · ') + (row.status ? ED.status.label(t, ED.status.of(row.status, 'run')) : esc(t('easydrag.ui.child_history_unknown_outcome'))) + '</span></button>'`
- `:172`. Replace
`body += '<p class="ed-hint">' + esc(callName(f) + ' · ' + (f.status ? core.tr(t, 'easydrag.ui.status_' + f.status, f.status) : t('easydrag.ui.child_history_unknown_outcome'))) + '</p>';`
with
`body += '<p class="ed-hint">' + esc(callName(f) + ' · ') + (f.status ? ED.status.label(t, ED.status.of(f.status, 'run')) : esc(t('easydrag.ui.child_history_unknown_outcome'))) + '</p>';`
- `:178`. Replace `esc(core.tr(t, 'easydrag.ui.status_' + row.step.status, row.step.status))` with `ED.status.label(t, ED.status.of(row.step.status, 'step'))`.
- `easydrag-batches.js:74`. Replace
`'<strong>' + esc(t('easydrag.ui.batch_item_index') + ' ' + item.index + ' · ' + core.tr(t, 'easydrag.ui.status_' + item.status, item.status)) + '</strong>'`
with
`'<strong>' + esc(t('easydrag.ui.batch_item_index') + ' ' + item.index + ' · ') + ED.status.label(t, ED.status.of(item.status, 'step')) + '</strong>'`
- `easydrag-publish.js:188`. Replace
`'</h4><p>' + esc(exactTest ? (evidence.mode === 'simulation' ? t('easydrag.ui.simulation_title') : t('easydrag.ui.real_test')) + ' · ' + core.tr(t, 'easydrag.ui.status_' + evidence.status, evidence.status) : t('easydrag.ui.publish_not_tested')) + '</p></section>';`
with
`'</h4><p>' + (exactTest ? esc((evidence.mode === 'simulation' ? t('easydrag.ui.simulation_title') : t('easydrag.ui.real_test')) + ' · ') + ED.status.label(t, ED.status.of(evidence.status, 'run')) : esc(t('easydrag.ui.publish_not_tested'))) + '</p></section>';`

Checked and left alone, as spec §3.3 says: `easydrag-runs.js` (the edge `delivered` state is a wire state), `easydrag-home.js:224` (the bulk action outcome), `easydrag-approvals.js:290` and the `approval_state_*` keys (the state of an approval, not of a run), and `easydrag-pack.js:235` (a sentence key with "status" in its name).

- [ ] **Step 3: tests and proof**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && grep -rn "easydrag.ui.status_" ui/js scripts ui/*.go | grep -v "easydrag-status.js"; node scripts/test-easydrag.mjs 2>&1 | grep -E "FAIL|all EasyDrag" | head; node scripts/test-easydrag-render-baseline.mjs 2>&1 | tail -1
```
Expected: the grep prints one line only: `ui/desktop_easydrag_test.go` (the families map entry, removed in Task A.6), then `all EasyDrag checks passed` and `PASS render baseline (unchanged)`. If a Node check fails on text such as `status_success` in compare, batches or publish output, change its expected text to the harness fallback (`ok`, `failed`).

- [ ] **Step 4: commit**

Message `C:/disposable/easydrag-w1a/msg-a5.txt`:

```
refactor(easydrag): compare, child history, batches and publish use ED.status

The last four renderers of run and step statuses draw through ED.status: a
disc and word in rows, the word alone in an option and the compare table. No
reader of easydrag.ui.status_* remains in the scripts.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && git add ui/js/desktop/apps/easydrag-compare.js ui/js/desktop/apps/easydrag-child-history.js ui/js/desktop/apps/easydrag-batches.js ui/js/desktop/apps/easydrag-publish.js && git commit -F C:/disposable/easydrag-w1a/msg-a5.txt
```

---

### Task A.6: Delete the old keys

**Files:**
- Create (outside the repo): `C:/disposable/easydrag-w1a/keys/a6-delete.json`
- Modify: `ui/desktop_easydrag_test.go:215` (the `status_` family entry)
- Modify: `ui/lang/easydrag/*.json` (through the helper)

- [ ] **Step 1: prove that no code reads the old keys**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && grep -rn "easydrag.ui.status_\|'status_'" ui/js scripts ui/*.go internal | grep -v "^ui/js/vendor" | cut -c1-140
```
Expected: only `ui/desktop_easydrag_test.go:215` (the families map). Anything else is a reader: migrate it (Task A.5's rule) before you delete. The dynamic reads (`'easydrag.ui.status_' + x`) have a literal prefix, so this grep sees them.

- [ ] **Step 2: the key file and the test map**

`C:/disposable/easydrag-w1a/keys/a6-delete.json`:

```json
{
  "_delete": [
    "easydrag.ui.status_cancelled", "easydrag.ui.status_error", "easydrag.ui.status_pending", "easydrag.ui.status_queued",
    "easydrag.ui.status_running", "easydrag.ui.status_skipped", "easydrag.ui.status_success", "easydrag.ui.status_waiting"
  ]
}
```
In `ui/desktop_easydrag_test.go` delete the line
`		"easydrag.ui.status_":             {"pending", "queued", "running", "waiting", "success", "error", "skipped", "cancelled"},`
(it would demand the keys we remove). Then:

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && node C:/disposable/easydrag-w1a/locale-apply.mjs C:/disposable/easydrag-w1a/keys/a6-delete.json && grep -c "easydrag.ui.status_" ui/lang/easydrag/en.json ui/lang/easydrag/de.json && gofmt -l ui/desktop_easydrag_test.go && GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go test ./ui -run '(?i)(TestEasyDragUIKeysExistInAllLocales|TestEasyDragAssetsLoadInDependencyOrder)' -count=1 2>&1 | grep -v '^{' | tail -2 && GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go test ./internal/flows -run TestCatalogTranslationsAreComplete -count=1 2>&1 | tail -1
```
Expected: `0 keys set, 8 deleted, in 16 files`, grep counts `0` for both files, no gofmt output, `ok  	aurago/ui`, `ok  	aurago/internal/flows`.

- [ ] **Step 3: commit**

Message `C:/disposable/easydrag-w1a/msg-a6.txt`:

```
chore(easydrag): drop the easydrag.ui.status_* keys

Nothing reads them since every status goes through ED.status
(easydrag.status.<value>). run_stopped_by stays: it is the tooltip of the
"stopped" status.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && git add ui/desktop_easydrag_test.go ui/lang/easydrag && git commit -F C:/disposable/easydrag-w1a/msg-a6.txt
```

---

### Task A.7: The status lint and the full browser run

**Files:**
- Create: `ui/desktop_easydrag_status_test.go`

- [ ] **Step 1: write the lint (it should pass on the migrated tree)**

Create `ui/desktop_easydrag_status_test.go`:

```go
package ui

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// W1a A (spec §3.5): a status is drawn only through ED.status. Outside easydrag-status.js and its CSS block (.ed-status-disc,
// .ed-status-label, .ed-status-word) no EasyDrag file may carry a status class or a status word key.
var (
	easyDragStatusClass = regexp.MustCompile(`(?:^|[^-])ed-status|ed-run-dot|\bstatus-(?:success|error|running|queued|skipped|waiting|pending|cancelled)\b|'status-'\s*\+`)
	easyDragStatusKey   = regexp.MustCompile(`easydrag\.ui\.status_|easydrag\.status\.`)
	easyDragStatusOwn   = regexp.MustCompile(`ed-status-(?:disc|label|word)\b`)
	easyDragStatusOld   = regexp.MustCompile(`ed-status--|ed-status-pill|ed-run-dot`)
)

// easyDragStatusAllowed lists what may stay, as "file|needle" with the reason. A needle that matches nothing fails the test,
// so the list cannot go stale.
var easyDragStatusAllowed = map[string]string{
	"js/desktop/apps/easydrag-canvas.js|'status-' +":            "the card ring: the card's state border keyed to the step record, not a rendered status (browser tests select .ed-node.status-*)",
	"css/desktop-app-easydrag.css|.ed-node.status-":             "the card ring rules",
	"css/desktop-app-easydrag.css|.ed-node.is-selected.status-": "the selected card ring",
}

func TestEasyDragStatusesGoThroughTheStatusMap(t *testing.T) {
	used := map[string]bool{}
	allowed := func(file, line string) bool {
		for needle := range easyDragStatusAllowed {
			parts := strings.SplitN(needle, "|", 2)
			if parts[0] == file && strings.Contains(line, parts[1]) {
				used[needle] = true
				return true
			}
		}
		return false
	}
	files := []string{"css/desktop-app-easydrag.css"}
	for _, name := range easyDragModules {
		if name != "easydrag-status.js" {
			files = append(files, "js/desktop/apps/"+name)
		}
	}
	for _, file := range files {
		isCSS := strings.HasSuffix(file, ".css")
		for i, line := range strings.Split(readDesktopAssetText(t, file), "\n") {
			hit := easyDragStatusClass.MatchString(line) || (!isCSS && easyDragStatusKey.MatchString(line))
			// the disc family is the CSS block of the status module
			if !hit || (isCSS && easyDragStatusOwn.MatchString(line) && !easyDragStatusOld.MatchString(line)) || allowed(file, line) {
				continue
			}
			t.Errorf("%s:%d draws a status outside ED.status: %s", file, i+1, strings.TrimSpace(line))
		}
	}
	var stale []string
	for needle := range easyDragStatusAllowed {
		if !used[needle] {
			stale = append(stale, needle)
		}
	}
	sort.Strings(stale)
	for _, needle := range stale {
		t.Errorf("allow-list entry %q matches nothing; remove it", needle)
	}
}

// Every locale has the twelve status words and aria sentences and uses one word for "unclear": the port label and the status
// word are the same text. The old status keys are gone.
func TestEasyDragLocalesUseOneStatusWordForUncertain(t *testing.T) {
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		words := readEasyDragLang(t, lang)
		for _, value := range []string{"ok", "stopped", "failed", "running", "waiting", "uncertain", "blocked", "skipped", "paused", "simulated", "not_run", "cancelled"} {
			for _, key := range []string{"easydrag.status." + value, "easydrag.status." + value + "_aria"} {
				if strings.TrimSpace(words[key]) == "" {
					t.Errorf("%s.json misses %s", lang, key)
				}
			}
		}
		if words["easydrag.ui.port_uncertain"] != words["easydrag.status.uncertain"] {
			t.Errorf("%s.json: port_uncertain %q and status.uncertain %q must be the same word", lang, words["easydrag.ui.port_uncertain"], words["easydrag.status.uncertain"])
		}
		for key := range words {
			if strings.HasPrefix(key, "easydrag.ui.status_") {
				t.Errorf("%s.json still has %s; statuses use easydrag.status.<value>", lang, key)
			}
		}
	}
}
```

The pattern skips custom properties (`--ed-status-neutral` has a dash before `ed`). The CSS lines of the disc family match `ed-status` and `easyDragStatusOwn` but not `easyDragStatusOld`, so they pass. `easydrag.js` is in `easyDragModules`, so its `ED.status.bind` line is scanned; it carries no class or key text and passes. A JS file that writes `ed-status-disc` itself is a hit (only the module may).

- [ ] **Step 2: run it against the migrated tree**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && gofmt -l ui/desktop_easydrag_status_test.go; GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go test ./ui -run '(?i)(TestEasyDragStatusesGoThroughTheStatusMap|TestEasyDragLocalesUseOneStatusWordForUncertain)' -count=1 -v 2>&1 | grep -E '^(--- |ok|FAIL|\s+desktop)'
```
Expected: two `--- PASS` lines and `ok`. A failure line names `file:line`: it is a surface the migration missed (migrate it) or a comment that mentions an old class (reword the comment; the lint is line-based and does not know comments). Then prove the lint can fail: temporarily add `// ed-run-dot` to a line of `easydrag-home.js`, run the test, see `easydrag-home.js:N draws a status outside ED.status`, and revert with `git checkout ui/js/desktop/apps/easydrag-home.js`.

- [ ] **Step 3: the full EasyDrag browser run (once, sandbox off, about 5-6 minutes)**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_BROWSER_ARTIFACT_DIR=D:/tmp/easydrag-w1a-shots GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go test ./ui -run '(?i)easydrag' -count=1 -timeout 30m -v 2>&1 | tee D:/tmp/easydrag-w1a-shots/a-after.log | grep -c '^--- PASS'; grep -c '^--- SKIP' D:/tmp/easydrag-w1a-shots/a-after.log; grep -E '^--- FAIL|^\s+--- FAIL' D:/tmp/easydrag-w1a-shots/a-after.log
```
Expected: the same counts as the baseline (41 PASS, 2 SKIP; the new Go tests add PASS lines: expect the baseline plus 3, because `TestEasyDragStatusColourTokens` and the two lint tests are not gated), and no FAIL line. If a browser test fails, read its message:
- a selector of an old class (`.ed-status`, `.ed-run-dot`): update the selector to the disc (`.ed-status-disc[data-status="failed"]`), as A.3 and A.4 did for the three known ones;
- a layout test (the card badge now sits in the same `.ed-node-badges` slot at 18px, the run row lost its 8px dot): update only expected positions that moved because of the status markup;
- the render baseline counts (`scripts/easydrag-render-baseline.json`) change only if status markup caused it, which it should not (the counts are cards, paints and wires). If they do change, run `AURAGO_EASYDRAG_RENDER_BASELINE=update node scripts/test-easydrag-render-baseline.mjs --update`, say in the commit that status markup is the cause, and name the changed counts.

Open two screenshots from `D:/tmp/easydrag-w1a-shots` (a run-done scene and the test result) and look at the discs in one dark and one light theme before you commit.

- [ ] **Step 4: final gates and commit**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 go vet ./ui && GOTMPDIR='D:\tmp\go-build' GOTOOLCHAIN=go1.27.4 npm run test:easydrag:t0 2>&1 | tail -5; git status --short
```
Expected: vet silent, T0 green, and `git status` shows only `?? ui/desktop_easydrag_status_test.go` (plus any test selector you had to update in Step 3).

Message `C:/disposable/easydrag-w1a/msg-a7.txt`:

```
test(easydrag): status lint - no status class or word key outside ED.status

Fails on a status class (ed-status*, ed-run-dot, status-<raw>) or a status
word key (easydrag.ui.status_*, easydrag.status.*) outside easydrag-status.js
and its CSS block. The allow-list keeps the card ring (the card's state
border) with its reason and fails when an entry matches nothing. A second test
checks that every locale has the twelve status words and aria sentences, that
port_uncertain is the same word as status.uncertain, and that no
easydrag.ui.status_* key is left. Full EasyDrag browser run: counts equal the
baseline.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>
```
```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1a-status && git add ui/desktop_easydrag_status_test.go && git commit -F C:/disposable/easydrag-w1a/msg-a7.txt
```

---

### Coverage: spec item to task

| Spec item | Task |
|---|---|
| §3.1 the twelve values, tokens, glyphs, words | A.2 (table, keys), A.1 (grey, inks) |
| §3.1 the grey token, set by EasyDrag | A.1 |
| §3.2 `of`, `disc`, `word`, `aria`, `label`, locale keys | A.2 |
| §3.3 canvas card badge, step status, card name | A.3 |
| §3.3 detail 274, 613, effect chip 619 | A.3 |
| §3.3 header pill, editor 80/155, home grid, run drawer, durable, testing | A.4 |
| §3.3 batches, compare, child history, publish | A.5 |
| §3.3 runs (`delivered`), home bulk outcome | left alone, listed in A.5 Step 2 |
| §3.3 CSS: old families out, disc in, forced-colors rule | A.2 (disc, forced colors), A.3 and A.4 (old families) |
| §3.4 tokens (type, radius, space, motion, status) | A.1 |
| §3.5 Node table test | A.2 |
| §3.5 status lint | A.7 |
| §3.5 colour test with the grey and the inks | A.1 |
| §3.5 locales cover the new family; one uncertain word | A.2 (keys), A.7 (test) |
| §3.1 `stopped` replaces S0c's hint, `cancelled` run value | A.4 (`statusOf`, banner, test panel), A.2 (`of`) |
| Old `easydrag.ui.status_*` keys removed | A.6 |
| §10 acceptance: status lint green, 3:1 and 4.5:1 | A.7, A.1 |
| S0c ledger row 24 (`stopped`) | A.4 |

### Deviations from the spec

1. **`--ed-radius-xl` is the new 12px token.** `--ed-radius-lg` is 14px today (`--ds-radius-lg`; 16px in Fruity), not 12px. By the brief's rule it keeps its value and the new step is `--ed-radius-xl: 12px`. `--ed-radius` (10px, theme-scaled) is not aliased to the new 8px `--ed-radius-md` either. So "become aliases" of §3.4 holds only for the motion tokens; nothing visual changes in this part.
2. **The motion tokens read the desktop's durations** (`var(--ds-motion-duration-fast, 120ms)`), not a fixed 120/200ms, because Amiga sets them to 0. `--ed-motion-slow` is `calc(var(--ed-motion-base) * 1.6)` (320ms by default, 0 on Amiga). The old names alias the new ones.
3. **The glyph on the grey disc is the panel colour, not white**, because white on the light grey disc of a dark theme is 2.7:1 or worse. 3:1 or better in all 16 contexts (3.32 worst).
4. **A disc ring token (`--ed-status-edge`) and a per-band glyph ink (`--ed-status-ink`)** are added. The coloured discs fail 3:1 on the light surfaces of four themes (1.00-2.18:1) and the fixed dark ink fails 4.5:1 on the two themes with dark status colours (2.86-3.69:1). Both are fixed on the surface side (§11), not by changing a status colour.
5. **The step's name is not part of the stopped status word.** S0c's hint said "Durch Stopp beendet: {step}"; the status replaces the hint line (the spec says so), and the stop step's name is the `title` tooltip of the banner text and the test panel summary (owner decision). The key `run_stopped_by` is kept for that. A run row cannot carry it: list rows have no steps.
6. **An open approval reads "Blockiert" in run rows**, where it read "Genehmigung ausstehend" (owner decision 5). The durable section keeps the reason text.
7. **Extra optional arguments.** `disc(value, size, opts)` takes `{ t, title, hidden }`, `label(t, value, text)` takes a word override, and the module adds `ofStep`, `table` and `bind`. The contract calls still work. `bind(t)` is called once from `easydrag.js` so that `disc(value, size)` without `t` (the lab) has a translated aria text.
8. **The card ring keeps `status-<raw>` classes** (`classesFor`, canvas.js:515, and `.ed-node.status-*` CSS). It is the card's state border, the browser tests select it, and canvas.js may not grow. The lint allow-list records this.
9. **`easydrag.ui.port_uncertain` is rewritten** to the same word as `easydrag.status.uncertain`, so every locale has one word for "unclear" in a status (§3.5).
10. **No idle pulse on a running disc.** The old pulse animations on `.ed-run-dot--running` and `.ed-status--running` are gone; the `@keyframes ed-pulse` stays for Part D, which owns animation rules and the `[data-motion="normal"]` key.
11. **The spec's "OKLab 0.20 and hue 30 degrees" check is not applied to the grey** (coordinator decision of 2026-11-13): it keeps the 3:1 contrast check only. The four coloured tokens keep their checks in `TestEasyDragCategoryColourTokens`.

### Decisions needed

1. **The glyph on the grey disc: 3:1 (recommended) or 4.5:1.** At the 60% mix the worst glyph contrast is 3.32:1 (voxel_world, windows_xp), which meets the 3:1 bar for graphics. A 4.5:1 glyph needs about a 75% mix (4.82:1 in voxel_world). Then the grey disc is nearly the text colour in every theme and competes with the coloured discs. Recommendation: keep 60%; the word beside the disc carries the meaning.
2. **The disc ring (recommended) or per-theme status colours for the four light themes.** The ring makes every disc carry a 1px grey outline, visible on dark themes too but faint there. The alternative, darker status tokens in fruity-light, amiga, windows_xp and voxel_world, breaks the E2 rule that EasyDrag's status colours are fixed. Recommendation: the ring; judge it on the screenshots of Step 3 of A.7.
3. **Name of the 12px radius.** `--ed-radius-xl` (the brief's rule) sits oddly next to `--ed-radius-lg` at 14px. Part B and the lab should use `--ed-radius-xl` for 12px. An alternative is to rename the old `--ed-radius-lg` to `--ed-radius-theme-lg` in a later cleanup and then rename xl to lg; not now. Recommendation: keep the brief's names and note the oddity in Part B's preamble.
4. **Decided by the coordinator:** the stop step's name is a tooltip, not part of the word (see deviation 5).
