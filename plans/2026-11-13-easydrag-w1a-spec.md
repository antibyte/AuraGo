# EasyDrag W1a "Foundations": design spec

- **Status:** design spec, 2026-11-13. The owner approved the design in the brainstorming session on 2026-11-12/13. Implementation plan: [`plans/2026-11-13-easydrag-w1a/README.md`](2026-11-13-easydrag-w1a/README.md). Its "Amendments found while planning" take precedence until they are copied in here.
- **Roadmap:** `plans/2026-11-09-easydrag-next-roadmap.md`:
  - §6, Wave 1 "Readable and honest" (lines 1412-1434);
  - E2 "One visual language" (lines 350-416);
  - the principles in §2, numbers 1, 2 and 7.
- **Predecessors:** S0, S0b and S0c closed Wave 0. S0c landed at 92761a38a. Its spec, `plans/2026-11-12-easydrag-s0c-spec.md`, hands rows 23 and 24 of its §7 ledger to this sub-project.

## 1. Goals and owner decisions

Wave 1 is too large for one spec, so it is split into six sub-projects (§2). W1a is the first. It builds the foundations that the other five render through:
- one status map;
- the type, radius, space and motion tokens;
- a small component kit;
- locale-aware plurals and formatting;
- an icon map and a labelled rail;
- one motion gate;
- the first scenes of the conformance lab.

Owner decisions (2026-11-12/13):

| # | Question | Decision |
|---|---|---|
| 1 | How to cut Wave 1 | **Six sub-projects, foundations first** (§2). |
| 2 | How a status looks | **The disc.** The glyph sits in a filled circle of the status colour, and the word is in normal text. |
| 3 | Which status gets which colour | **Five groups.** Green means done, red means failed, blue means under way, amber means look here, grey means did not happen (§3.1). |
| 4 | A run the user cancelled | **Its own run-level value, `cancelled` ("Abgebrochen").** It is grey with the square glyph and is never shown as failed. |
| 5 | An approval that is still open | **`blocked`** (amber). It is not shown as `waiting`. |
| 6 | Callout (info box) style | **A tinted surface with the status disc in front and no border**, so it can never be mistaken for an input field. |
| 7 | Rail captions | **The caption sits under the icon.** The rail grows from 48 px to 64 px, and the captions are always visible. |
| 8 | How to cut W1a | **Four parts in parallel, then the lab.** A, C and D run in parallel, B runs after A, and E runs last (§9). |

Defaults set in the session, which the owner did not object to:
- "Lebendigkeit" defaults to **Normal**.
- Zero idle animations is a rule at all times, not only under reduced motion (§2 principle 7).
- The collective noun "Versionen" stays; every reference to one published state reads "vN" (§5.3).

Roadmap decision 13 (logic.stop) was taken in S0c (`plans/2026-11-12-easydrag-s0c-spec.md` §1, decision 1). W1a adds its status-map entry `stopped`. The roadmap's §10 is updated to record the decision as taken.

## 2. Wave 1 split and scope

| Sub-project | Items | Starts after |
|---|---|---|
| **W1a Foundations** (this spec) | FD-08 with the status map (+ S0c row 24 "stopped"), FD-09, UI-17, FD-11 (+ S0c row 23), CS-01, UI-15, JY-01, FD-14 first scenes | — |
| W1b Voice and actions | action registry, UX-18, CS-20, CS-17 ledger and budgets | W1a |
| W1c Header and one Stop | F.2 next-fire accessor, UI-14 with CS-07, the one Stop, UX-04 header slice | W1a, W1b |
| W1d Test loop | the rest of UX-07, UX-19, CS-06, JY-04, S0c row 25 (step-history panel over the canvas) | W1a |
| W1e Canvas | UI-13, FD-01, FD-05, S0c rows 18 (box select by touch) and 22 (`.ed-port-add` over the wire midpoint) | W1a |
| W1f Entry and demo | UX-01, UX-03, the content-first home slice, signature experience 3.1 | W1a, W1c |

S0c row 26 (long-press feel on a real touch screen) is a human session in the Wave 1 sessions; it gates nothing.

**In W1a:** §3 to §7.

**Not in W1a:**
- the action registry, the copy work (UX-18, CS-20) and the control budgets (W1b);
- the header, the one Stop and the primary-action states (W1c);
- the results panel and the sweep (W1d);
- framing and the paint diets (W1e);
- the home page and the demo (W1f);
- retiring the rail (decision 2, after the Wave 1 sessions).

**The Wave 1 gates that W1a closes:**
- zero idle animations;
- the OKLab token checks across the 14 themes in the lab's first scenes;
- the status-map lint;
- the plural tests.

**The Wave 1 gates that belong to later sub-projects:**
- the header and home control budgets (W1b);
- one visible Stop and equal control positions (W1c);
- at 390 px, the toolbar and the first three flows visible (W1f);
- the screenshot critiques (the Wave 1 exit).

## 3. Part A: the status map and the tokens (FD-08)

### 3.1 The values

| Group | Value | Glyph | Word (de) | Today's raw values |
|---|---|---|---|---|
| Done, green (`--ed-ok`) | `ok` | check | Erfolgreich | run and step `success` |
| Done, green | `stopped` (run only) | filled square | Durch Stopp beendet | a successful run that a logic.stop ended (`ED.testing.stoppedBy`) |
| Failed, red (`--ed-danger`) | `failed` | cross | Fehlgeschlagen | `error` |
| Under way, blue (`--ed-info`) | `running` | three-quarter ring | Läuft | `running` |
| Under way, blue | `waiting` | clock | Wartet | `queued`; a durable wait (timer, wait node) |
| Look here, amber (`--ed-warn`) | `uncertain` | question mark | Unklar | `effect_uncertain`, and any unknown effect |
| Look here, amber | `blocked` | circle with a slash | Blockiert | an open approval; a flow or step that needs the user first (a missing secret or setup found before a run). A start that already failed stays a failed run (`FLOW_START_FAILED`, S0c A1) |
| Did not happen, grey | `skipped` | arrow to a bar | Übersprungen | `skipped` |
| Did not happen, grey | `paused` | two bars | Pausiert | a flow that is switched off |
| Did not happen, grey | `simulated` | diamond | Simuliert | a step that ran in Simulation |
| Did not happen, grey | `not_run` | dashed circle | Nicht gelaufen | `pending`, never reached |
| Did not happen, grey | `cancelled` (run only) | filled square | Abgebrochen | `cancelled` |

- The grey is a new token, `--ed-status-neutral`. It is a fixed mix of the text colour and the panel, so it reads as "nothing happened" in every theme.
- Like the other status tokens, it is set by EasyDrag and never by a theme (E2 guardrail).
- The glyph is drawn in a fixed dark ink on the four coloured discs, and in white on the grey disc.

### 3.2 The module

New `ui/js/desktop/apps/easydrag-status.js`:
- **One table** maps each value to its token, glyph (inline SVG, 16×16 view box), word key and aria key.
- **`ED.status.of(raw, kind)`** maps today's raw values to a status value. `kind` is `run`, `step`, `effect` or `flow`. Unknown raw values map to `uncertain`, never to `ok`.
- **`ED.status.disc(value, size)`** returns the disc markup with `role="img"` and the aria text.
- **`ED.status.word(t, value)`** and **`ED.status.aria(t, value)`** return the translated texts.
- **`ED.status.label(t, value)`** returns the disc plus the word, the form most surfaces use.
- **Locale keys:** the new family `easydrag.status.<value>` and `easydrag.status.<value>_aria`, in all 16 locales. The old `easydrag.ui.status_*` keys are removed once no code reads them.

### 3.3 Surfaces that move to the map

Every place that renders a status today goes through `ED.status`. The 2026-11-13 survey found:
- `easydrag-canvas.js`: the card badge and step status (around 403, 421, 436-438, 515). canvas.js must not grow: the badge code is replaced by a call, not wrapped.
- `easydrag-detail.js`: 274 and 613, plus the `effect_uncertain` chip at 619.
- `easydrag-header.js`: 207-210, the pill.
- `easydrag-editor.js`: 80 and 155. Only existing lines change, because editor.js is at 896.
- `easydrag-home-grid.js`: 50.
- `easydrag-run-drawer.js`: 29-30 and 40, the run rows.
- `easydrag-durable.js`: 38.
- `easydrag-testing.js`: 284 and 300.
- `easydrag-batches.js`: 74.
- `easydrag-compare.js`: 43-118.
- `easydrag-child-history.js`: 172.
- `easydrag-publish.js`: 188.
- `easydrag-runs.js`: the edge `delivered` state stays a wire state, not a status.

The home page's bulk outcome (`home.js:224`: enabled, disabled, missing, mismatch, unavailable) is a result of an action, not a run status. It stays outside the map.

The plan re-runs the survey grep first and adds any surface it missed.

**CSS:**
- The ad-hoc families go: `.ed-status--*` (around 713-719), `.ed-status-pill--*` (959-961), `.ed-run-dot--*` (545-548), and the node badge pseudo-element glyphs (715, 717).
- New: one `.ed-status-disc` with `[data-status]` variants. The card ring keeps its state colour through the same tokens.
- The high-contrast shape rule (around 1557-1562) moves to the disc.

### 3.4 Tokens

New token families at the top of `ui/css/desktop-app-easydrag.css`:
- **Type:** `--ed-font-title` 24px, `--ed-font-head` 18px, `--ed-font-card` 16px, `--ed-font-body` 14px, `--ed-font-meta` 12px, `--ed-font-label` 11px. Part B moves every declaration onto them (§4.1). The owner first approved 15 and 13 for card and body. Planning found that principle 1 needs card names at 13.8px or more and parameter lines at 12px or more at fit zoom 0.86, so the owner chose 16 and 14 on 2026-11-13.
- **Radius:** `--ed-radius-sm` 4px, `--ed-radius-md` 8px, `--ed-radius-xl` 12px, `--ed-radius-pill` 999px.
  - Today's `--ed-radius` (10px) and `--ed-radius-lg` (14px, 16px in Fruity) keep their values, so nothing changes visually in Part A.
  - The 12px token is therefore named `-xl` (found while planning).
- **Space:** `--ed-space-1` to `--ed-space-6`: 4, 8, 12, 16, 24 and 32px. New code uses them; old declarations are not migrated in W1a.
- **Motion:** `--ed-motion-fast` 120ms, `--ed-motion-base` 200ms, `--ed-motion-slow` 320ms, `--ed-ease-out`. Today's `--ed-fast`, `--ed-base` and `--ed-ease` become aliases.
- **Status:** `--ed-status-neutral` (§3.1).

Themes set none of these (E2 guardrail: themes set materials only).

### 3.5 Tests (Part A)

- **Node, the table** (`scripts/test-easydrag-status.mjs`):
  - every value has a token, a glyph, a word key and an aria key;
  - `of()` maps each raw value in §3.1 correctly, and an unknown value maps to `uncertain`;
  - `disc()` carries `role="img"` and the aria text.
- **Go, the status lint** (`ui/desktop_easydrag_status_test.go`). It fails on any of these outside `easydrag-status.js` and its CSS block:
  - a status class (`ed-status-`, `ed-run-dot--`, `status-success` and the like);
  - a status word key (`easydrag.ui.status_`, `easydrag.status.`).

  Every locale must use one word for uncertain: the value of `easydrag.status.uncertain` is the only translation of "unclear" for a status.
- **Go, the colour test** (`ui/desktop_easydrag_colour_test.go`). It adds the grey token to the status set and checks, in all 14 themes:
  - the disc colours against the panel and canvas surfaces at 3:1;
  - the status ink at 4.5:1;
  - the existing hue (30°) and OKLab (0.20) distances to the category tokens.
- **Locales:** `TestEasyDragUIKeysExistInAllLocales` covers the new key family.

## 4. Part B: the kit and the type scale (FD-09, UI-17)

### 4.1 Type scale

- Every `font-size` in `desktop-app-easydrag.css` becomes a `var(--ed-font-*)`. That is about 94 declarations, 11 distinct values today.
- Mapping:
  - 10 and 11px go to label (11). A label that sits on the canvas goes to meta (12), because of principle 1;
  - 12 goes to meta;
  - 12.5, 13 and 14 go to body (14), and so do summaries and parameter lines;
  - card names and canvas titles, heading 14, 15 and 16 go to card (16), and so do the 16px touch input rules, so iOS does not zoom into a field;
  - 17 and 20 go to head (18);
  - 24 goes to title.
- The plan decides each declaration by its use and records the ones that move by more than 1px.
- **Text on the canvas is never below 12px at 100% zoom** (principle 1).
- **At fit zoom 0.86, card names stay at 13.76px or more and summaries at 12.04px or more**, so the thresholds of `TestDesktopEasyDragLegibilityBrowser` stay unchanged.
- Layout shifts are expected. The render baseline and the browser layout tests are updated where only the font size changed.

### 4.2 Radius and colour ratchets

- **Radius:** new radius declarations use `--ed-radius-*`. Theme material radii (`--vd-mat-*-radius`) keep applying to material surfaces, as they do today.
- **Ratchet test** (Go): it counts literal `border-radius` values and colour literals (hex, rgb, rgba, hsl, `white`, `black`) outside the token block. It compares them with checked-in numbers, which may only fall. A change that lowers the counts lowers the numbers in the same commit.
- **Font sizes** (Go): every `font-size` is a `var(--ed-font-*)`, so there are at most six sizes.

### 4.3 The kit

New `ui/js/desktop/apps/easydrag-kit.js`, `ED.kit`:
- **`disclosure({ title, count, open, body })`**
  - a button row with a chevron (right when closed, down when open), the title and an optional count on the right;
  - `aria-expanded` and `aria-controls`, Enter and Space toggle it;
  - no native `<details>`.
- **`callout({ status, text, action })`**
  - a tinted surface with the status disc in front (owner decision 6), with no border and no input-like inset;
  - the tint comes from the status token mixed into the panel, and the text stays in normal ink;
  - at most one action.
- **`chip({ text, glyph })`**
  - a neutral chip. Effect chips use neutral ink plus a glyph, never a warning token (E2 acceptance).
- **`toast({ status, text, action })`**
  - wraps `ed.ctx.notify` and carries the status disc and at most one action, such as "Rückgängig";
  - the undo toast in `easydrag-history.js:25-84` moves onto it.
- **`toolbar(items)`**
  - a row of controls with equal height and spacing, for W1c and W1f;
  - W1a builds it and uses it nowhere new.
- **`sheet`**
  - `modal()` (`easydrag-core.js:439`, about 30 callers) gets a phone layout: below 600px window width, a dialog opens as a bottom sheet with a grab bar and full width;
  - callers do not change.

All kit parts use the tokens of §3.4.

### 4.4 Disclosure migration

Every `<details>` in `ui/js/desktop/apps/easydrag-*.js` moves to `ED.kit.disclosure`. The 2026-11-13 survey found:
- `authoring.js` 132 and `publish.js` 183 (publish);
- `scenarios.js` 21, `testing.js` 170 and 255, and `transform.js` 187-190 (test);
- `detail.js` 77 (the JSON tree) and `child-history.js` 158 and 191 (diagnosis);
- `block-fields.js` 20, 33 and 230, `groups.js` 106, and `home.js` 109 and 122.

The plan re-runs the grep first. The JSON tree keeps its rule of opening levels below depth 2.

### 4.5 Tests (Part B)

- **Go:** no `<details>` or `<summary>` in any `easydrag-*.js`; the font-size rule; the two ratchets.
- **Node** (`scripts/test-easydrag-kit.mjs`):
  - a disclosure toggles `aria-expanded` and shows its body;
  - a callout carries the disc and no border;
  - `modal()` below 600px gets the sheet class.
- The existing browser tests that open publish, test and diagnosis screens keep passing with the new disclosures.

## 5. Part C: language and numbers (FD-11, CS-01, S0c row 23)

### 5.1 Plurals and formatting

New `ui/js/desktop/apps/easydrag-format.js`, `ED.format`:
- **`plural(t, key, count, vars)`**
  - asks `Intl.PluralRules` for the UI locale's category (`zero`, `one`, `two`, `few`, `many` or `other`);
  - uses `key + '_' + category`, and falls back to `key` (which is the `other` form);
  - today's `_one` keys keep working;
  - `core.pluralKey` and `core.stepsText` become thin wrappers.
- **`number`, `duration`, `date`, `relative`, `list`:** `Intl` formatters for the UI locale. `duration(400)` gives "0,4 s" in German. They replace:
  - the hard-coded `Intl.DateTimeFormat('en-GB')` in `easydrag-template.js` (around 378-407);
  - the locale-less `toLocaleDateString()` in `easydrag-credentials-purge.js:60`;
  - the separate formatters in `easydrag-core.js` (244-273), `easydrag-fields.js:502` and `easydrag-authoring.js:13`, which become calls to `ED.format`.
- **The UI locale** is the one EasyDrag's `t` uses. The plan finds where the host passes it.

### 5.2 Plural forms in the locales

- Every key whose value contains `{count}` gets the forms its locale needs, under the `_<category>` suffixes:
  - cs: `one`, `few`, `other`;
  - pl: `one`, `few`, `many`, `other`;
  - the other locales get their own categories per `Intl.PluralRules`.
- S0c row 23 is part of this: `folder_flows`, `cred_purge_versions` and `cred_purge_pinned`.
- A test lists which locales need which forms, so a missing `few` in Polish fails.

### 5.3 "vN" (CS-01)

- Every UI string that names one published state uses "vN": "v3 ist live", "Änderungen seit v3", "Zurück zu v2", "Freigegeben für v4".
- That is about 46 German and 7 English values (survey), plus the matching values in the other 14 locales.
- The collective noun stays "Versionen" (for example the list title).
- **No German or English EasyDrag value contains "Revision"** (E2 acceptance). The test checks de and en; the other locales are fixed in the same commit.

### 5.4 The pseudo-locale

- **Switch:** `localStorage['aurago.easydrag.pseudo'] = '1'`. When it is set, EasyDrag wraps its `t` so that every translated string comes back:
  - accented, for example "Ŝçħŕîţţé";
  - about 35% longer;
  - in brackets.
- Hard-coded strings and clipping become visible at once. The lab uses it (§7).
- **It never ships:** it is not a language in any list, and a test asserts that no language list contains it (E2 guardrail).

### 5.5 Tests (Part C)

- **Node** (`scripts/test-easydrag-format.mjs`):
  - de, cs and pl render 1, 2, 5 and 22 correctly through `plural`;
  - `duration`, `number` and `date` follow the locale;
  - the pseudo wrapper lengthens and brackets a string.
- **Go:**
  - every `{count}` key has the plural forms of every locale;
  - no de or en EasyDrag value contains "Revision";
  - the pseudo-locale is absent from every language list.

## 6. Part D: the rail, the icons and motion (UI-15, JY-01)

### 6.1 The rail

- `.ed-rail` grows from 48 to 64px, and every button shows its caption under the icon at `--ed-font-label` (owner decision 7). The rail is outside the canvas, so 11px is allowed.
- The captions are short new keys `easydrag.ui.rail_<id>`, in all 16 locales:

  | Button | Caption (de) |
  |---|---|
  | palette | Schritte |
  | home | Start |
  | templates | Vorlagen |
  | groups | Gruppen |
  | outline | Liste |
  | focus | Fokus |
  | keys | Tasten |

  The plan reads the rail's full button list (`easydrag-editor.js:71-77`, `easydrag-workspace.js:85-87`) and covers every button.
- Tooltips keep the long name and the shortcut.
- At narrow widths the rail keeps its captions. Below 900px it already moves (CSS around 906). The plan checks that the captions fit there and at 390px, where the rail is a row.

### 6.2 The icon map

New `ui/js/desktop/apps/easydrag-icons.js`, `ED.icons`:
- A table maps each meaning to one glyph name of the existing set (`easydrag-core.js` `ICONS`, 78 glyphs), for example `home`, `groups`, `templates`, `palette`, `outline`, `focus`, `keys`, `run`, `stop`, `publish`, `test`, `settings` and `more`.
- `ED.icons.of(meaning)` returns the markup through `core.icon`. No new glyph family, no new glyph set (E2 guardrail).
- **"Start" and "Gruppen" share the grid glyph today.** Gruppen gets its own glyph from the existing set, or a new glyph in the same `ICONS` style if none fits. The plan shows both candidates in the lab scene before it commits.
- The rail and the header read their icons through the map.

### 6.3 The motion gate and "Lebendigkeit"

New `ui/js/desktop/apps/easydrag-delight.js`, `ED.delight`:
- **`allow(kind)`** returns whether a motion of that kind may run. It is false for everything when:
  - "Lebendigkeit" is "Ruhig";
  - the system asks for reduced motion;
  - the desktop's `data-animations` is `false` (`desktop-foundation.js:714`, `animationsEnabled()`).

  `.ed-app` gets `data-motion="calm|normal"`, and the CSS keys every animation to `[data-motion="normal"]`.
- **The "Lebendigkeit" setting:**
  - two values, Ruhig and Normal; Normal is the default;
  - stored per device in `localStorage['aurago.easydrag.liveliness']`;
  - it never bumps `model.version` (E2 guardrail);
  - the UI is one menu entry in EasyDrag's app menu (the plan picks the existing app-level menu), with new keys in all 16 locales.
- **Zero idle animations** (principle 7). The plan audits the 8 keyframes and about 16 `animation` declarations.
  - Every animation runs only while something happens: a run, a drag, a just-inserted card, a loading request.
  - Any animation that runs in an idle editor or home page is removed or bound to that event.
  - Under "Ruhig", a running step shows the static `running` disc, and spinners are replaced by the static glyph.

### 6.4 Tests (Part D)

- **Node:**
  - the icon map has no glyph with two meanings;
  - every rail button has a caption key and a map entry;
  - `allow()` is false under each of the three conditions.
- **Browser (T1), `TestEasyDragMotionIsCalmWhenIdle`:**
  - in the main editor fixture after it settles, `document.getAnimations()` restricted to `.ed-app` is empty;
  - with "Ruhig" and a running-step fixture it is empty as well;
  - with "Normal" and the running fixture, at least one animation runs, so the test is not vacuous.

  The helper sets reduced motion by default; this test opts out with `Motion: true` and sets the conditions itself.
- **Browser:** the rail at 1440 and 900px shows every caption unclipped.

## 7. Part E: the lab, first scenes (FD-14)

`TestEasyDragLabBrowser` (`ui/desktop_easydrag_lab_browser_test.go`):
- It runs in T1 through `startEasyDragSmokeWith`.
- It grows out of the contact sheet (`ui/desktop_easydrag_contact_sheet_browser_test.go`). The sheet's screenshots stay behind `AURAGO_EASYDRAG_CONTACT_SHEET`, but the checks always run under `AURAGO_RUN_BROWSER_SMOKE=1`.

**Scenes,** each in all 14 themes:
1. **Status sheet:** every value of §3.1 as a disc plus word on the panel and on the canvas surface, plus one callout per status colour.
2. **Category tiles:** the nine category tiles and the six group colours.
3. **Idle editor:** the main editor fixture with the labelled rail. It is run a second time with the pseudo-locale.
4. **Run done:** the editor after a finished run, with status discs on the cards (the contact sheet's run-done seed).
5. **Test result:** the test panel with disclosures and a callout.
6. **Home:** the start page.

**Checks** in every scene and theme:
- **Contrast:** the page returns the rendered colours of the scene's status words, discs, tiles and their surfaces, and Go computes the ratios with the existing helpers in `desktop_easydrag_colour_test.go`. Status text needs at least 4.5:1; discs, tiles and icons at least 3:1.
- **Distance:** rendered category colours keep 30° hue and 0.20 OKLab from the rendered status colours.
- **Targets:** every visible control is at least 24×24px, with the documented exceptions of `TestEasyDragTargetSizes`.
- **Clipping:** in the pseudo-locale run, no text element inside `.ed-app` has `scrollWidth > clientWidth` unless it carries an ellipsis rule and a `title`.
- **Names:** every button, link, input and status disc has an accessible name (`aria-label`, label text or `role="img"` with `aria-label`).

Failures name the theme, the scene, the element and the measured value. With `AURAGO_BROWSER_ARTIFACT_DIR` set, the run writes the contact sheet as today.

Runtime: about 84 scene renders. If the lab takes more than 4 minutes in T1, the plan moves the pseudo-locale run to a separate test and records it.

## 8. Testing

Essential tests only (the owner's standing preference).

| Part | Node (T0) | Go (T0) | Browser (T1) |
|---|---|---|---|
| A | status table and `of()` | status lint, colour test with grey, locale keys | existing tests updated |
| B | kit | no `<details>`, font-size rule, two ratchets | publish/test/diagnosis screens still pass |
| C | plurals de/cs/pl, formatters, pseudo wrapper | plural forms per locale, no "Revision", pseudo never listed | — |
| D | icon map, motion gate | — | idle motion test, rail captions |
| E | — | — | the lab |

Every new Node script matches `scripts/test-easydrag*.mjs` and every new Go test matches `(?i)easydrag`, so T0 picks them up.

## 9. Order and parallel work

- **A, C and D** run in parallel, each in its own worktree.
- **B** starts after A has landed, because the callout and the toast use the status disc.
- **E** starts after A, B, C and D have landed.
- One combined review per part, Critical and Important findings only.

**File overlap:**
- A owns the status surfaces, the status CSS, the token block and `easydrag-status.js`.
- B owns the kit, the font-size migration, `modal()` and the disclosure call sites.
- C owns `easydrag-format.js`, the formatter call sites and the locale value rewrites.
- D owns the rail markup and CSS, `easydrag-icons.js`, `easydrag-delight.js` and the animation rules.
- **The locale files are shared by A, C and D.** Each part keeps its new keys and value changes in its own key file for the locale helper script. On a rebase conflict in `ui/lang/easydrag/*.json`, the part takes main's files and re-applies its key file.
- **CSS is shared:** A touches the token block and the status rules, and D the rail and animation rules. A conflict there is resolved by hand; the parts touch different rules.

**Line ratchet:**
- `easydrag-canvas.js` (928) must not grow; Part A replaces its badge code with calls.
- `easydrag-editor.js` (896) changes existing lines only.
- New modules are registered in `module-loader.js`, the Go list `easyDragModules` and the Node harness lists.

## 10. Acceptance checklist

- [ ] Every status on every surface renders through `ED.status`, as a disc with a glyph and a word. The status lint is green.
- [ ] The grey token and the four status colours pass 3:1 for discs and 4.5:1 for text in all 14 themes, and keep the hue and OKLab distances to the categories.
- [ ] `cancelled` and `stopped` are run-level values; an open approval shows `blocked`.
- [ ] Every `font-size` is one of six tokens, no `<details>` remains, and both ratchets are green.
- [ ] Callouts are tinted surfaces with a disc. A dialog is a bottom sheet below 600px.
- [ ] de, cs and pl render 1, 2, 5 and 22 correctly. Every `{count}` key has its locale's forms, and S0c row 23 is closed.
- [ ] No de or en EasyDrag string says "Revision", and published states read "vN".
- [ ] The pseudo-locale works behind its switch and is in no language list.
- [ ] The rail is 64px with a caption under every icon. No glyph carries two meanings.
- [ ] Zero animations run in an idle editor, and "Ruhig" removes every animation while a step runs.
- [ ] The lab's six scenes pass in all 14 themes, and the contact sheet still builds.
- [ ] The roadmap's §10 records decision 13 as taken. Its Wave 1 status lists W1a with the six-part split, and S0c rows 23 and 24 point at this spec.

## 11. Risks

- **Status surfaces are many.** About 15 renderers and three CSS families move. Browser tests that select the old classes break and are updated in the same commit, and the plan re-runs the survey grep first.
- **A theme's material fails a contrast check.** Status colours are fixed (E2 guardrail), so a failing theme is fixed in its EasyDrag surface tokens, not by changing the status colour. If a material blocks that, the finding goes to the owner.
- **The type scale moves layouts.** Fourteen-px body text becomes 13 and 16-px headings become 15, so some panels shift. The render baseline and the layout tests are updated only where the font size is the cause.
- **Three parts edit the locale files.** The key-file rule (§9) makes a conflict a re-run instead of a hand merge.
- **The lab is slow or flaky.** 84 renders on a shared Chrome. Every check reports its measured value, a flaky check is re-run alone before it counts, and the time budget in §7 applies.
- **Removing an idle animation removes a cue someone relies on**, for example a pulsing "unsaved" dot. Each removed animation is listed in Part D's report, and a cue that matters becomes a static mark.
