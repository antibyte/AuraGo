# EasyDrag Next: Living Flows (roadmap, 2026-11-09)

Status: proposed roadmap, not started. On 2026-11-10 the owner chose strategy A, craft first (§5), and the waves in this document follow it. This plan builds on `plans/2026-11-04-easydrag-flagship-plan.md`, whose workstreams A–J have largely shipped. It covers the next generation of EasyDrag: what to build, in what order, why, and what to leave out. It is not an implementation plan, a design spec or a schedule. S, M, L and XL are relative sizes. Every number comes from the current-state brief of 2026-11-09 or is marked as a target or a design bound. Nothing in this plan was measured or built.

This revision applies a completeness review and a feasibility review of the first draft, plus a fact-check of its statements about the current code: 140 claims checked, 5 corrected. The main changes:
- Every scoped item now has a wave.
- Each wave demo uses only work from the same wave or an earlier one.
- The craft phase is Waves 0 to 4, and the power phase is Waves 5 to 7b (§6).
- The epics now carry the safety rules the reviews found missing: input trust, taint on seeded runs, layering, source trust and unknown effects.

Where this revision departs from a review suggestion, §10 says so in one line.

The plan changes the order in the flagship plan's §12. That section names the J.4 operations overview as the next unit. Here, J.4 becomes part of E7 in Wave 3. First, Wave 0 frees room in modules that are close to the 1,100-line limit: runs.js has 5 lines left, editor.js 31 and home.js 51. fields.js is already at 1,010 lines, and canvas.js, model.js and template.js are above the new 900-line warning. Before any edit, implementation work must run the GitNexus impact analysis that `CLAUDE.md` requires.

Names used in this document: a short module name such as runs.js means `ui/js/desktop/apps/easydrag-runs.js`. Ids such as [UI-03] point to Appendix A. E1 to E16 are the epics in §4, and "decision N" is an open decision in §10.

## At a glance

| Wave | Theme | What the owner notices at the end | Main epics |
|---|---|---|---|
| 0 | Clear the deck | Readable cards; a clean run looks clean; tests start in Simulation; a typed token moves into the vault with one click | E1, first parts of E2 and E3 |
| 1 | Readable and honest | A first result in about a minute; one status pill and a Stop that never moves; flows first on the home; one glyph and one word for every status, in every theme | E2, E4, E5, E8, E3, the first part of E9 |
| 2 | Red to green, a run you can watch | The Testbank strip; live progress with timers and counts; from a failure notification to the failed run, to the field, to "Behoben"; living cards and semantic zoom; sample forms and live previews of mapped fields; deep links that work | E5, E6, E3, E4, E7 (deep links), the UX parts of E9 |
| 3 | The system comes to you | The "Heute" strip, run health on every card, approvals that announce themselves (durable mode on), a publish review with the next run, version compare and live rollback; recipe cards that state their needs; "Bis hier simulieren" | E7, E8, E4, E5 |
| 4 | Phone first, a canvas that handles like a design tool | The phone step list with bottom sheets and a bottom bar; edge scroll, even spacing, F2 rename and sticky notes; keyboard wiring and a screen-reader pass; peek previews; "erhält" while connecting, ">" commands in quick-add and a palette sorted by topic | E16, E3, E2, E5, the UX parts of E9 and E13 |
| 5 | Flows that listen and remember | Start conditions, time windows and an "Ein Ablauf ist fehlgeschlagen" trigger; pickers with real devices and chats, and captured test events; expressions, Retry 2.0, fallback values and generic steps that fail honestly; flow memory and "Nur neue Einträge" | E10, E9, E11, E13 |
| 6a | Real devices and chat commands | Chat commands ("/gutenacht"); Home Assistant depth, calendar, Durchsage and Desktop-Hinweis; "Mit Formular starten" and flows in Spotlight; recipe instances | E10, E13, E14, E7, E8 |
| 6b | The house talks back | Service outages, watch presets, folder intake, "Warten auf Ereignis", trigger rules with debounce and quiet hours, missed schedules | E10, E11, E8 |
| 7a | Co-author | AI that edits the open draft through reviewed change sets, with a ghost review and one undo; an AI cost cap | E15 |
| 7b | Act at scale | Effect receipts, "Ab hier fortsetzen", one action per item, long waits, approvals for effectful steps, flows as tools for Aura | E12, E14 |

Strategy: A, craft first, chosen by the owner on 2026-11-10 (§5, §6). The next unit to specify is S0, "Room, trust and legibility" (§7).

## 1. Where EasyDrag stands

EasyDrag is a serious automation editor with a strong trust layer. Things to keep:
- effects listed per step, and "Stop does not undo";
- draft and live always kept apart;
- evidence bound to the exact revision;
- an isolated Simulation;
- encrypted named scenarios;
- typed pure blocks and batches;
- a durable approval path, off by default;
- a sound canvas grammar: a distinct trigger shape, dotted error wires, path dimming, magnetic ports with reasons for refusing a connection;
- undo-first editing;
- accessibility work beyond what similar tools offer;
- 42 curated step types, 12 recipes and 16 locales.

The two screenshot critiques agree on a verdict:

| | Score |
|---|---|
| Correctness and trust | about 9/10 |
| Craft | 5–6/10 |
| Joy | 3–4/10 |

The flow, which is the product, is the weakest thing on screen:
- At the 0.8 zoom floor, labels show at about 10.4 px. The failure caption at 64 % zoom shows at about 7 px.
- Category colours reuse the status colours, so a clean run looks partly like a warning.
- Approvals arrive silently.
- The library hides the flows behind controls.
- Phones get a squeezed desktop layout.

The main power gaps:
- Only 14 triggers. There is no inbound chat, infrastructure, file or run-failed event.
- No state across runs: no counters, deduplication or digests.
- No arithmetic or comparisons in templates.
- No effect receipts, so retries and resume are unsafe.
- No effectful iteration.
- logic.wait is capped at one hour.
- Home Assistant support is shallow, and generic tool steps have no outputs and no success check.
- Run now takes no inputs, and flows are not tools for Aura.
- Approvals exist only on the durable path. That path is off by default, needs a restart to switch on, covers only manual pure-local flows, and has never been switched on in production.

Most UI fixes are in the presentation layer. Most power fixes need small, bounded engine primitives, not a new engine.

## 2. North star and experience principles

**North star.** EasyDrag is the place where an idea for the house, the homelab or the family becomes a running, trusted automation in minutes, and stays understandable months later. The canvas reads like a map: every card says what it does, when it runs and how it last went. Anything AuraGo can notice can start a flow. Flows remember, compute and act on lists, and every outward effect is accounted for. Testing is safe by default and fed with real data. Failures explain themselves and turn green in place. Approvals and problems find the owner. Delight comes from real outcomes, such as the message as it was sent or "Behoben" and "Live v3", and never at the cost of trust.

**Principles, each with a test:**

1. **Legible at a glance.** At 100 % zoom no canvas text is below 12 px. At every automatic fit, on screen:
   - card names render at 13.8 px or more;
   - parameter lines render at 12 px or more;
   - pills, port labels and the status line render at 10.3 px or more, until the FD-04 compact tier (Wave 2) hides them.

   Checked by a computed-style browser test.
2. **Colour means status, categories stay colourful.** Status colours appear only where state is shown: the card ring, wire states, chips, and the status glyph and word. Every status carries a glyph and a word from one status map. Categories are colourful, but only on the icon tile: a saturated tile with a light icon. This follows the owner's decision of 2026-11-10 for colourful categories rather than muted ones. The test, run in all 14 themes:
   - Every coloured category tile keeps an OKLCH hue distance of at least 30° from every status hue of the theme.
   - Every category tile keeps an OKLab distance of at least 0.20 from every status token. Tiles below chroma 0.05 (the neutral categories) are exempt from the hue rule.
   - Group colours, wires, effect chips, the simulation frame, and selection and focus rings use tokens that pass the same distance test.
   - A lint fails when a category colour appears outside the icon tile.

   Checked by a token test plus a forced-colors check.
3. **One next action.** Each screen has at most one filled primary button. Control positions before and after a run are equal at each width (x-positions compared at 390, 900 and 1440 px). Exactly one Stop is visible per window.
4. **Safe by default.** Simulation is preselected whenever a flow has any effect, including unknown ones. Simulation makes zero invoker, vault or network calls (instrumented test). A real test always needs one explicit confirmation that lists every effect, and no keyboard shortcut starts or confirms one.
5. **Real things, not ids.** The default sample, mapping and error views never show a node id or a bare FLOW_* code as the main text (unit and browser tests).
6. **Every problem has one action.** Each attention row, error card and notification has one primary action, and one click on it lands on the exact run, step, field or decision (scripted journey).
7. **Calm motion.** There are zero running animations inside `.ed-app` while idle. Under reduced motion every animation reaches its end state within one frame. A run plays at most one sound (browser tests).
8. **Honest power.** Every new primitive has:
   - a documented bound;
   - a Simulation path;
   - translations in all 16 locales;
   - an "unklar" state from the status map for anything it cannot observe.

   Every new source declares whether its data is trusted, and untrusted is the default. This is checked at review before merging and by a catalog test.

## 3. Signature experiences

Each experience names the wave whose exit demo it is (§6). A demo uses only items scheduled in that wave or an earlier one.

**3.1 First success** (Wave 1 demo).
- *Before:* "Neuer Flow" opens a name prompt. The test dialog preselects Real, and the result collapses into a one-line disclosure (run-done).
- *After:* "Neu" opens the canvas at once, with the title selected and four trigger tiles: "Zu einer Uhrzeit", "Wenn sich in Home Assistant etwas ändert", "Manuell" and "Webhook". From the trigger's "+", the quick-add header reads "Schritte nach: Zeitplan" and suggests "Push senden". "Testen" simulates. A light sweep runs along the path, and the result panel opens by itself: "Simulation geklappt · 3 Schritte in 0,4 s", with the message text itself. From Wave 2 on, this result appears in the docked Testbank strip. The first-run checklist reaches 3/3.

**3.2 A live run you can watch** (Wave 2 demo).
- *Before:* one card glows, and Stop is inserted mid-row, pushing Publish about 107 px to the right (run-live).
- *After:* the labelled Stop appears in its reserved header slot, so nothing moves. The docked Testbank strip shows "Schritt 2 von 5 · 1,8 s". The active card counts its own time. Finished cards show "0,4 s · 3 Einträge", and count chips land on the wires. "Zum aktiven Schritt" brings the work into view. The camera follows only if the owner switches "Folgen" on.

**3.3 Fixing a failure (07:01)** (Wave 2 demo; the notification opens the run through the OP-13 run link).
- *Before:* the raw provider text "chat not found", a 7 px caption at 64 % zoom, and a written instruction (run-error).
- *After:*
  1. The notification opens the run. The card reads "Telegram kennt diesen Chat nicht" at readable size.
  2. "Chat-ID prüfen" opens the inspector, focuses the field and flashes it.
  3. The owner picks the right chat from the chat picker and presses "Nur diesen Schritt erneut testen". The retest simulates by default and shows "Simulation geklappt".
  4. The owner chooses "Echt ausführen" and confirms the one Telegram message it lists. The card turns green: "Behoben im Entwurf · beim 2. Versuch".
  5. "Als Testfall merken" keeps the case as a regression test. After the fix is published, the 07:01 run's row reads "behoben in v4".

**3.4 An approval arriving** (Wave 3 demo, on the pure-local durable path).
- *Before:* nothing happens. The inbox fetches only when opened, and a request that expires after 10 minutes can lapse unseen (approval-inbox-standard-dark-1440).
- *After:* durable mode is switched on. A manually started pure-local flow, "Wochenplan bestätigen", builds the plan and reaches its Freigabe step.
  - A desktop notification reads "Wochenplan bestätigen wartet auf deine Freigabe · läuft in 9:58 ab", and the notification-centre badge shows 1.
  - The phone gets a link to the signed-in review, never a decision button.
  - The card leads with the question and the review data.
  - "Freigeben" waits 5 seconds with "Rückgängig" before the decision is sent.
  - The waiting step on the canvas pulses three times, "Wartet auf dich", and then stays still.

  Where durable mode is off, the library instead shows a "Freigaben einrichten" card. It says that a restart is needed and that only pure-local flows can be approved (decision 11).
- *Later (Wave 7b, if decision 11 (b) is taken):* the same moment gates a Rechnungslauf before any invoice is sent, and the card lists the reviewed effects.

**3.5 The morning library** (Wave 3 demo; the content-first toolbar ships in Wave 1).
- *Before:* a hero, five equal buttons, filters, a disclaimer and folders all come before the first flow. At 390 px no flow is visible (ux-library-standard-390).
- *After:* one toolbar, then a "Heute" line: "Alles ruhig · 7 Läufe seit gestern 22:14 · nächster Lauf 07:00". If something needs attention, at most four tiles appear instead, each opening its exact target. Every card shows its last ten live runs as marks with glyphs, plus its next run. At 390 px the Heute block is one line, "2 brauchen dich ›", above the flows, and it opens as a sheet.

**3.6 Publishing with confidence** (Wave 3 demo; the primary-action states ship in Wave 1).
- *Before:* Publish is the primary button even when nothing changed. The dialog says "Neu: 1 · Verbindungen geändert" and names nothing (publish-dialog).
- *After:* the primary button reads "Testen" until the change is tested, then "Veröffentlichen · 2 Änderungen". The review:
  - names each change and outlines it on the canvas;
  - lists the effects;
  - shows "Nächster Lauf: Mo 06:30 (Europe/Berlin)";
  - shows a schedule change as "Mo–Fr 07:00 → 06:30", and names the shift when the publish falls in a DST week.

  After the server confirms, the pill flips to "Live v3". There is no confetti.

**3.7 A flow reacting to the home** (Wave 6b demo; capture and pickers arrive in Wave 5, Durchsage in Wave 6a).
- The washing machine's power sensor flaps on MQTT. In the trigger inspector the owner presses "Auf Testereignis warten" and starts the machine. The real payload lands as the sample, and the draft starts no run.
- The owner adds "Nur starten, wenn power < 5" and "Ruhe 60 s". The card reads "startet bei 1 von 214 Ereignissen".
- A "Durchsage" step speaks "Die Wäsche ist fertig" in the kitchen, and a time condition keeps it quiet between 22:00 and 07:00. If PI-09 slips, the demo uses a Push step instead.

**3.8 A chat command** (Wave 6a demo). In bed, the owner types "/gutenacht" in Telegram. AuraGo matches the owner's private chat before any LLM call. It starts the published flow, switches off three lamps, and answers in the same chat: "Gute Nacht · 3 Lampen aus · Haustür zu". The same text from another account, from a group or as a forwarded message starts nothing and reaches Aura unchanged.

**3.9 Building with the copilot** (Wave 7a demo).
- *Before:* the proposal is goal-only. Apply deletes and re-adds every node with model-generated ids and replaces the groups.
- *After:* in "Ändern mit KI" the owner types "Um 06:30 statt 07:00, und zusätzlich per WhatsApp". The copilot asks one question, "An welchen Chat?", with the owner's configured chats as buttons. The change appears on the owner's own canvas: one changed card and one ghost card. The owner accepts both, and one Ctrl+Z restores everything. Ids, positions, groups and schedules stay intact.

**3.10 Editing on the phone** (Wave 4 demo).
- *Before:* window chrome and three or four header rows take about 30 % of the height, and the results card covers the canvas (editor-phone).
- *After:* at 390 px the flow opens as a list of steps in run order, each with its key setting. Tapping "Müll rausbringen · Push" opens a bottom sheet, and the edit is one undo step. "Testen" sits in the bottom bar, and a labelled Stop replaces it during a run.

### Wireframe A: editor with the Testbank and the new cards (1440 px; Stop sits in its reserved header slot, the Testbank shows progress only)

```
+------------------------------------------------------------------------------+
| < Morgenbriefing [Live v2 · Entwurf geändert v] <- -> Läufe [Stopp] [Testen] |
+------------------------------------------------------------------+-----------+
|  .--------------.     +----------------+     +----------------+  | Telegram  |
| ( Zeitplan       )===>| Wetter holen   |====>| KI: Zusammenf. |  | senden    |
| ( Mo-Fr 07:00    )    | wetter.de      |  3  | 2 Felder       |  |-----------|
| ( naechst: Mo 07 )    | ok 0,4 s       | -->>| ok 1,1 s       |  | Chat      |
|  '--------------'     +----------------+     +-------+--------+  | [@andi v] |
|                                                      |           | Text      |
|                                      +---------------v--------+  | {{ki.text}}
|                                      | Telegram senden [sendet]| | -> "Heute:|
|                                      | an @andi                |  | drei ..." |
|                                      | laeuft 0,8 s            |  |           |
|                                      +-------------------------+  |           |
+------------------------------------------------------------------+-----------+
| TESTBANK | (o) Simulieren  ( ) Echt ausfuehren | Schritt 4/4 · 1,9 s         |
| Testfaelle: Beispiel · Letzter Lauf · + | Simulation geklappt · 4 Schritte    |
|                                         | "Heute: drei KI-Meldungen ..."      |
+------------------------------------------------------------------------------+
```

### Wireframe B: node card anatomy (232 × 80, anchored top left)

```
 +------------------------------------------+
 | [ic] Telegram senden            [sendet] |  line 1: tile, name 16 px, effect chip
 | an @andi · "Heute: {{ki.text}}"          |  line 2: key parameter, 14 px, upright
 | (ok) 0,4 s · gesendet                    |  line 3: status, 12 px, glyph + word
 +------------------------------------------+
 in o                                    o out      error port: dotted, below
 At the 0.86 automatic-fit floor: name 13.8 px, line 2 12 px, line 3, pills
 and port labels 10.3 px. Zoomed out further, the compact tier (Wave 2) hides
 line 3 and the pills.
 Silhouettes: trigger = round cap, if/switch = diamond tile,
 batch = stacked edge, block = double frame, wait = ring that drains,
 approval = person tile, AI = inner line
```

### Wireframe C: home with the attention strip (1440 px; below 560 px the HEUTE block is one line, "2 brauchen dich ›", that opens as a sheet)

```
+------------------------------------------------------------------------------+
| [Suchen ...]  (Alle)(Aktiv)(Braucht Aufmerksamkeit) [Filter v] [##|=] [Neu v]|
+------------------------------------------------------------------------------+
| HEUTE  [Braucht dich: 1 Freigabe · laeuft in 8 Min. ab  > Ansehen]           |
|        [2 Flows mit Fehlern seit gestern                > Laeufe]            |
|        [naechster Lauf 07:00 · Morgenbriefing]                               |
+------------------------------------------------------------------------------+
| +----------------------+ +----------------------+ +----------------------+   |
| | Morgenbriefing  Live3| | Paket angekommen Live| | Heizung Nacht  Entwurf|  |
| | [mini graph]         | | [mini graph]         | | [mini graph]          |  |
| | . . . . . . . . x .  | | . . . . . . . . . .  | | (noch keine Laeufe)   |  |
| | naechst: Mo 07:00    | | wartet auf Ereignis  | |                       |  |
| +----------------------+ +----------------------+ +----------------------+   |
+------------------------------------------------------------------------------+
```

### Wireframe D: the phone step list (390 px)

```
+-----------------------------+
| <  Muell rausbringen  Live2 |
| [Liste | Karte]             |
+-----------------------------+
| (o) Zeitplan                |
|     Di 18:55            >   |
| [=] Text bauen              |
|     "Morgen: {{typ}}"   >   |
| [!] Push senden  [sendet]   |
|     an Handy Andi       >   |
| + Schritt                   |
+-----------------------------+
| [ Testen ]  Laeufe   ...    |
+-----------------------------+
```

## 4. Epics

### E1. Clear the deck: room, measurement and trust debt
Pillars: foundation, safety.

**Goal.** Make room in the full modules, measure before redesigning, and close the safety gaps that new features would otherwise build on.

**What the user gets**
- Simulation is preselected whenever a flow has effects, and the mode control has its own label.
- The AI goal box stops at 4,000 characters and shows a live counter.
- Trigger samples appear in the UI language.
- Text from mail or webhooks can no longer steer an ai.decision step.
- "Aura darf Abläufe nur ansehen" really stops agent runs.
- A typed token moves into the vault with one click, and no copy stays behind in old versions, the database file or backups.

**Scope**
- [FD-10] Module splits:
  - Split runs.js along its existing seams into run-stream, run-start, run-stop and run-drawer, behind the unchanged `ED.runs` facade.
  - Extract the header from editor.js and the grid render path from home.js.
  - Split fields.js (1,010 lines). The conditions editor moves to `easydrag-fields-conditions.js` and the template field to `easydrag-template-field.js`. fields.js stays as a field-kind registry. The inspector, sample forms (UX-08), the run form (PE-11), recipe setup (PI-16) and trigger-rule rows (PE-14) all render through that registry, so no feature builds its own form system.
  - Add a headroom ratchet that warns at 900 lines and fails at 1,100.
- Split before use. This list is checked at each wave's entry. Each split gets one commit and its own impact check, as FD-10 requires.
  - Wave 0: runs.js, editor.js, home.js and fields.js (above).
  - Wave 2 entry:
    - Card rendering moves from canvas.js (923 lines) into `easydrag-card.js`, before UI-04, UI-05 and the FD-04 tiers.
    - template.js (911) does not grow from here on, and the template-side JS for UX-05 and UX-06 goes into `easydrag-preview.js` or `easydrag-transform.js`.
  - Wave 5 entry:
    - PE-10's JS evaluator goes into a new `easydrag-expr.js`.
  - detail.js (878) takes no new code until its split at the Wave 4 entry:
    - E6 field focus goes into `easydrag-repair.js`, and E9 previews into `easydrag-preview.js`.
    - At the Wave 4 entry, detail.js splits before E16, JY-03 and the PI-14 pickers in Wave 5.
    - The E15 KI tab goes into `easydrag-aiview.js` in Wave 7a.
  - model.js (921) does not grow. New model helpers go into a new `easydrag-model-ops.js`, created at the Wave 4 entry (or earlier, if a craft wave needs a model helper first): the upstream and downstream walk for the canvas-handling track and the run-order projection for E16. E15 reuses it in Wave 7a for the client-side change-set apply.
- [FD-17, AI-16] Trust sweep:
  - Fence the string leaves inside the ai.decision `state`.
  - Bound sticky notes on the server: 100 notes, 2,048 runes each, Cf characters refused, note bytes counted toward the 2 MiB document cap. Redact registered secrets in note text in exports, packs and the run view, and keep notes out of AI payloads by allowlist. Notes stay in exports and packs, because they become a canvas feature in Wave 4 (decision 3).
  - Enforce `flows.agent.read_only` on run and run_now, and show allow_publish as unavailable [OP-17, AI-17]. The agent manual's switch section ships with this.
  - On upgrade, the startup compatibility check (`internal/flows/service_compat.go`) marks a live flow as "Republish needed" when its ai.decision mode cannot take a fenced state, and names the step.
  - Stored notes above the new limits are truncated on load with a one-time notice, instead of refusing the draft.
- [FD-12] Defect sweep:
  - In S0: goal counter, Simulation default and label, translated trigger samples (A9).
  - In the S0c hygiene track: help for schedule days 29–31, manual drift fixes, and the file.write default from decision 4.
- [FD-13] (security redesign, S0b) Move typed credentials out of drafts, versions, the database file and backups.
- [FD-18] (S0b) Verify and land the browser egress closure (S8).
- [FD-03] Seeded S, M and XL fixtures with a render-counter ratchet in CI. The baseline is S0 step 0, committed before any split or visual change. Timings are asserted only on the reference profile and are first recorded at the Wave 2 exit.
- [FD-15] (S0c) Test tiers T0 and T1. Every browser test sets reduced motion explicitly. T1 carries the Wave 1 touch audit: CDP touch emulation at 1024×768 and 390×844 for inserting on a wire, deleting a wire, card tools, dragging a mapping chip into a field, and box select.
- [FD-20] (S0c) Verify-first defect tail. Each item ends as fixed with a test, closed with evidence, or a task in a named wave:
  - logic.stop run-level semantics (defect E3; decision 13);
  - N20: help text for divisors;
  - `internal/flows/AGENTS.md`:820 still says "six" templates;
  - the seven unverified follow-ups of 2026-10-25:
    - the mission_update debounce;
    - SSE redaction per (run, seq) with a per-client cap;
    - the start outcome discarded in `StartFlowRun` (internal/server/flows_bridge.go:353, where `_, err := h.StartFlowRunReported(...)` drops the FlowStartOutcome);
    - LastRun missing omitzero;
    - the docked palette not restored after the run view below 900 px;
    - a duplicated "Vergangener Lauf";
    - U9/V4 targets under 24 px;
  - "StartResult surfaced" is an entry criterion for OP-01 (Wave 3) and PE-16 (Wave 5), fixed in S0c by the Wave 2 exit, so trigger start failures reach run_failed and OP-01;
  - "SSE redaction verified" is an entry criterion for the Wave 2 Testbank, which adds SSE consumers.
- [FD-16] (S0c) Generated manual reference blocks and an agent manual, shared with OP-17.

**Acceptance criteria**
- runs.js has at most 400 lines; editor.js, home.js and fields.js at most 900. The budget test warns at 900 and fails at 1,100. From the Wave 0 exit on, no EasyDrag module ends a wave above 1,000 lines, and the list of modules above 900 can only shrink.
- The counter baseline is committed before UI-02, and UI-02 and UI-03 leave every counter unchanged.
- An instruction-override payload in the ai.decision state reaches a stub provider inside fence markers. The existing `choice_<id>` and uncertain-port tests pass unchanged.
- A 2,049-rune note or a 101st note is refused with a translated code. A registered secret inside a note never appears in an export or a pack, and the AI proposal payload contains no note text. A stored draft with an over-long note opens with the note truncated and a one-time notice.
- Right after the upgrade, a live flow whose ai.decision mode cannot take a fenced state shows "Republish needed" with the step named.
- With read_only on, the agent's run_now on a flow mission is refused, while list and history still work.
- After "In den Tresor verschieben", a republish and a purge, no plaintext token is found in the fixture's flows.db, its WAL file or any flows backup in the data directory. Versions still pinned by a run survive until they are unpinned.
- The S8 fixture is refused: a popup to 169.254.169.254 and a WebSocket to a private address. The fixture passes once live on aurago-test.
- FD-16: a test fails when a generated reference block in the manual differs from the catalog. The agent manual describes read_only and allow_publish as they actually behave.
- FD-20: every listed item has an outcome before the Wave 2 exit.

**Size:** L (several S and M items).
**Waves:** 0 (S0 and S0b). The S0c hygiene track runs through Wave 2. Split-before-use checks run at the entries of Waves 2 and 4.
**Dependencies:** none. FD-13's new code uses the FD-19 classes once they exist (Wave 2), otherwise one translated code.

**Guardrails and invariants**
- The module moves preserve behaviour: one commit per module, a GitNexus impact check for every moved function, and the facade and bus wiring unchanged.
- FD-13:
  - Redaction widens to URL query parameters and credential-named body fields.
  - The vault write is one admin, CSRF-protected endpoint.
  - Autosave and undo snapshots never carry credential values.
  - Superseded versions are never rewritten in place. They are marked for purge, and purged only after a verified backup and once no run, approval or wait pins them.
- The FD-13 purge:
  - runs VACUUM (or secure_delete) and a WAL checkpoint;
  - encrypts the pre-purge backup with the master key, or deletes it once the purge is verified;
  - shows, in the same dialog, the older migration snapshots that contain typed credentials (`.flows-v2-backup-*` and `.flows-v3-backup-*`, store.go:274-281) and offers to delete them.
- read_only only removes abilities, and a one-time notice explains the change.
- The FD-18 proxy binds to loopback, uses credentials created per call that never appear on a command line, and fails the step closed if the proxy dies.

**Main code areas**
- New modules:
  - Wave 0: `easydrag-run-stream.js`, `easydrag-run-start.js`, `easydrag-run-stop.js`, `easydrag-run-drawer.js`, `easydrag-header.js`, `easydrag-home-grid.js`, `easydrag-fields-conditions.js`, `easydrag-template-field.js`.
  - Later: `easydrag-card.js`, `easydrag-expr.js`, `easydrag-model-ops.js`.
- Engine and config: `internal/flows/catalog_decision.go`, `model.go`, `credentials.go`, `local_pack.go`, `service_compat.go`, `store.go`; the agent dispatch flow branch; `ui/cfg/flows.js`.
- Tests and docs: `ui/desktop_easydrag_test.go`; the unmerged branch `claude/modest-northcutt-18107e` (9368239fb and c1d427ffc: an SSRF egress proxy for the shared headless browsers), to be verified and landed.

### E2. One visual language
Pillars: ui, foundation, joy.

**Goal.** Give every colour, token, status, icon, component, action and word exactly one meaning, guarded by tests across 14 themes and 16 locales.

**What the user gets**
- Red always means failed. Each category keeps its own colour on the icon tile, from hues that never touch the status colours, and a clean run looks clean (run-done).
- Every status reads the same everywhere: one glyph and one word, with "unklar" for anything uncertain.
- Info boxes no longer look like input fields. One chevron disclosure replaces the native triangles.
- Every rail icon has one meaning and a caption.
- An action has the same name, shortcut and reason for being unavailable in every menu.
- "v3" is the only name for a published state, and counts read "1 äußere Verbindung".

**Scope**
- [UI-03, FD-08] Two palettes and the token layer:
  - six saturated category hues on the icon tile only (in Standard dark: indigo for triggers, violet for AI, berry for messages, teal for web, petrol for the planner, olive for the smart home), a warm-neutral documents tile and two neutral categories (logic, tool). Hues stay outside a ±30° window around every status hue. Teal and petrol sit close together, so the icon carries that difference;
  - group colour names mapped to category hues;
  - neutral wires;
  - a neutral "unpublished" chip;
  - type, radius, space and motion tokens.
- Status map (part of FD-08, Wave 1). One enum (ok, failed, running, waiting, skipped, uncertain, simulated, not_run, paused, blocked). Each value maps to a token, a glyph, a word key and an aria text. Cards (UI-04), run marks (OP-02), the pill, the Testbank, receipts and the phone list (E16) render through it. "unklar" is the one word for uncertain, including unknown effects.
- Action registry (Wave 1). One registry, generated from the existing menu descriptors, holds for each action: id, label key, icon, shortcut, enabled state with a reason, and risk. It feeds the context menus, the selection bar, the header "⋯", Ctrl+K command mode (UX-04) and the Spotlight flows source (OP-11).
- [UI-17] At most six font sizes, plus radius tokens, with a lint ratchet. One disclosure component replaces `<details>`.
- [FD-09] Shared kit: disclosure, a dialog that becomes a sheet on phones, toolbar, callout, chip and toast. It wraps the desktop dialog and sheet primitives where they exist and adds only EasyDrag-specific parts.
- [UI-15] Icon map and rail captions. The icon map is a table of semantic aliases over the desktop icon runtime (`ui/js/desktop/core/mini-icons-runtime.js`, `icon-theme-runtime.js`), with no new glyph set. Retiring the rail [CS-08] is decision 2.
- [FD-11] Plural keys through Intl.PluralRules, locale formatting, and a pseudo-locale behind a developer flag.
- [CS-01] "vN" replaces Revision and Version in the UI.
- [UX-18, CS-20] Outcome-first copy, one form of address (decision 1), limits shown only when reached, and no notifications about the user's own actions.
- [FD-14] Conformance lab: the first scenes in Wave 1 and about 24 scenes by Wave 3. It runs contrast, target, clipping and accessibility-tree checks, and builds a contact sheet across the 14 themes.
- [FD-07] WCAG 2.2 AA closure: an NVDA smoke test of the header, the Testbank and the canvas at the Wave 3 exit, and the scripted NVDA and VoiceOver pass at the Wave 4 exit, which must still hold at the Wave 7b exit.
- [CS-17] Concept ledger and a control-count budget for the header and the home.
- [JY-01] Delight kernel: one motion gate and a "Lebendigkeit" setting (Ruhig or Normal).

**Acceptance criteria**
- Colour tokens in all 14 themes:
  - Every coloured `--ed-cat-*` tile keeps an OKLCH hue distance of at least 30° from every status hue, and every category and group token keeps an OKLab distance of at least 0.20 from every status token (§2 principle 2). A lint fails when a category colour is used outside the icon tile.
  - Effect chips use neutral ink plus a glyph and never a warning token.
  - The simulation frame passes the same test. So do selection and focus rings in every theme whose accent falls within that distance.
  - Status ink reaches 4.5:1, and category tiles and icons 3:1.
- A lint fails on any status class or status word outside the status map, and every surface says "unklar" for uncertain.
- For every action in the registry, the context menu, the selection bar and the header "⋯" show the same label, shortcut and disabled reason.
- The stylesheet uses at most six font sizes, and the checked-in count of hard-coded colours and radii can only fall.
- The publish, test, diagnosis and approval screens contain no native disclosure triangle.
- de, cs and pl render 1, 2, 5 and 22 correctly, and no de or en UI string says "Revision".
- With reduced motion or `data-animations=false`, zero animations run inside `.ed-app` while idle.

**Size:** L.
**Waves:**
- 0: UI-03, in S0.
- 1: the core, the status map, the action registry, UX-18 and CS-20, the first lab scenes, CS-17.
- 3: the NVDA smoke test.
- 4: the scripted NVDA and VoiceOver pass (FD-07), which must still hold at the Wave 7b exit.

**Dependencies:** E1 (module room); decision 1 before the copy work.

**Guardrails and invariants**
- Themes set materials only. They never set status colours, ink, the type scale, shapes or hierarchy.
- No status is shown by colour alone.
- The pseudo-locale never ships.
- The Lebendigkeit setting is stored per device and never bumps `model.version`.
- The icon map adds no glyph family; it only aliases desktop icons.
- No external fonts or assets.

**Main code areas**
- The token block at the top of `ui/css/desktop-app-easydrag.css`.
- New modules: `easydrag-kit.js`, `easydrag-icons.js`, `easydrag-status.js`, `easydrag-actions.js`, `easydrag-format.js`, `easydrag-delight.js`.
- `t()` in `easydrag-core.js`; `ui/lang/easydrag/*.json`; the lab harness as a new browser test.

### E3. A canvas you can read at any zoom
Pillars: ui, foundation, joy.

**Goal.** Make the flow the crispest thing on screen, at every zoom and up to 500 steps.

**What the user gets**
- Every card says what it does, when it runs next and how it went: "Mo–Fr 07:00", "an @andi", "0,4 s · 3 Einträge".
- Wait, batch, block, approval and AI steps have their own silhouettes.
- Zooming out moves from Detail to Compact to a calm map of named groups.
- Panels never hide the selected step. The minimap appears only when the flow does not fit.
- Wires show direction and counts and can be focused. Auto-layout glides into place, and one undo reverts it.
- The canvas handles like a design tool. It scrolls when a drag reaches the window edge, steps can be spaced evenly, F2 renames a step, and one action selects everything upstream or downstream.
- On touch, tapping a wire shows its "+" and delete.

**Scope**
- [UI-02] (Wave 0) Type sized for 100 % zoom: names 16 px, parameter lines 14 px, and captions, pills and port labels 12 px, so no canvas text is below 12 px at 100 %.
- [FD-01, FD-05, UI-20] (Wave 1) Hot-path and paint diet: wire repaints follow adjacency, groups render only when needed, the minimap uses its cache, and the running glow animates opacity.
- [UI-13] (Wave 1) Framing into the free area, a minimap that gets out of the way, and an overview popover below 900 px.
- [FD-04 with UI-01] (Wave 2) Viewport culling and three detail tiers chosen by on-screen text size, ending in a group overview tier. The compact tier hides pills, port labels and the status line.
- [UI-04] (Wave 2) Living cards at 232×80 with a key-parameter line, a status line and an effect chip, rendered by `easydrag-card.js`.
- [UI-05] (Wave 2) Silhouettes. The wait ring ships with JY-06.
- [UI-08] (Wave 2) Neutral wires with chevrons, count pills and keyboard focus.
- [FD-02, JY-16] (Wave 4) Uncapped layering, with a glide that is instant under reduced motion.
- [UI-09] (part, Wave 4) Insert-with-room, reconnect from the output end, and a ghost card while inserting.
- [UI-12] (part, Wave 4) Colour swatches and a header band for groups.
- Canvas handling (Wave 4):
  - one edge auto-scroll helper, built on visibleArea(), for drag, connect and box select;
  - distribute and equal-gap guides, with Tidy as one undo;
  - F2 inline rename;
  - select upstream and downstream.
- [UI-11] (Wave 4) Sticky notes. N, or "Notiz" in the selection bar, drops a note at the pointer. Notes look like paper cards with a folded corner and a visible "Notiz" label. They come in five paper tints that pass the colour rule of §2 principle 2; there is no yellow, because yellow means warning. They hold plain text with bold and bullet lists, and links show as plain text. They resize on the 8 px grid, are edited in place and are part of undo. Notes never connect or run and sit beneath the cards. They can be pinned to a group so they move with it, show as a small glyph in the overview tier, and are listed in the outline under "Notizen".
- [FD-06] (Wave 4) Keyboard wiring and narrated runs.
- Deferred: UI-06 (ships with E12), UI-10, knife and detach.

**Acceptance criteria**
- Text size: at 100 % zoom no canvas text is below 12 px. At every automatic fit, on screen, names render at 13.8 px or more and parameter lines at 12 px or more. Pills, port labels and the status line render at 10.3 px or more until the compact tier hides them. None of the 12 recipe layouts gains an overlapping card.
- XL fixture (500 steps, 2,000 wires):
  - at 100 %, the live DOM is at most 20 % of today's;
  - dragging repaints exactly the wires attached to the dragged selection, never a full scan (counter);
  - p95 pan frame ≤33 ms on the reference profile (target).
- Each of the 42 curated types renders a translated key-parameter line in all 16 locales. 1,000 run events cause no card re-render.
- A card shows at most one "+" affordance.
- At 25 %, a 50-step grouped flow shows every group title at 12 px or more on screen and a status glyph on every step.
- With the inspector open, previous and next always leave the selected card fully visible.
- A 120-step chain gets 120 columns, and one Ctrl+Z undoes the layout.
- Dragging a connection to the window edge scrolls the canvas and stops on release. Distribute is one Ctrl+Z.
- In touch emulation, tapping a wire shows its "+" and delete.
- Sticky notes:
  - Creating, editing and deleting a note are single undo steps and never change the scenario fingerprint.
  - Note text with `<img onerror>` or a `javascript:` link renders as plain text.
  - No note renders inside the publish review or the approval dialogs.
  - A note pinned to a group moves with the group.

**Size:** XL.
**Waves:**
- 0: UI-02.
- 1: UI-13, FD-01, FD-05.
- 2: the canvas track (FD-04, UI-04, UI-05, UI-08).
- 4: the canvas-handling track, with FD-02, JY-16, the UI-09 and UI-12 parts, and UI-11 sticky notes.
- 4: FD-06, last, so the keyboard wiring covers the new gestures.

**Dependencies:** FD-03 counters (E1); type tokens and the status map (E2); the F.2 next-run accessor (built in Wave 1 with E4, shared with E7).

**Guardrails and invariants**
- The card height change is anchored top left, so no stored position changes.
- View state stays per device. Selected, focused, hovered, running and connect-target items are never culled. Edge auto-scroll only moves the view and never bumps `model.version`.
- `card_summary` is a catalog template with typed placeholders, rendered through textContent. A secret_ref shows only its vault key name.
- The effect chip comes from CollectEffects. Unknown or unavailable effects show a neutral "unklar" chip with a glyph, never a warning colour.
- Sticky notes:
  - Their text renders through createElement and textContent only, never innerHTML.
  - A fixed note frame with a "Notiz" label keeps a note from posing as system UI.
  - Notes never reach AI payloads (allowlist test) and never affect execution, fingerprints or block pins.

**Main code areas**
- `easydrag-canvas.js`, `easydrag-wires.js`, `easydrag-groups.js`, `easydrag-interact.js`.
- New modules: `easydrag-card.js`, `easydrag-spatial.js`, `easydrag-layout.js`, `easydrag-notes.js`.
- The catalog `card_summary` field in `internal/flows`; `ui/css/desktop-app-easydrag.css`.

### E4. Calm chrome, confident publishing
Pillars: ui, ux, joy.

**Goal.** One status, one next action, one Stop, one form, one language for showing changes.

**What the user gets**
- A 48 px header with the name, one pill ("Live v2 · Entwurf geändert"), the action that follows the state, and a Stop that never moves.
- An inspector that is one sectioned form. The first field sits in the top quarter, and controls are 32 px with a mouse.
- One selection bar instead of a toolbar on every card. Every per-card action can be reached through it by tap.
- A publish review that names the changes, shows them on the canvas, and shows the next run and any schedule shift. It ends with the pill flipping to "Live v3".
- Any two versions can be compared on the canvas, cloned, or rolled back live in one reviewed step.

**Scope**
- [UI-14, CS-07] (Wave 1) Header. It ships the primary action's states: "Testen" until the change is tested, then "Veröffentlichen · N Änderungen", then nothing on a clean live flow. Its pill popover holds Aktiv and the action that completes a partial publish.
- One Stop (Wave 1). One Stop component, owned by `easydrag-run-stop.js`, renders in exactly one slot per layout: the reserved header slot on desktop and, from E16 on, the bottom bar below 560 px. The Testbank shows progress only. The runs drawer has no Stop of its own; selecting a running run puts it into the header slot.
- The F.2 next-fire accessor (Wave 1 foundation): read-only, in `timers.go`, and read by UI-04, UX-13, OP-02 and the attention endpoint.
- [UI-16, CS-12] (Wave 2) One form, with the summary line "Wenn etwas schiefgeht: Flow anhalten · keine Wiederholung".
- [UI-07, CS-16] (Wave 3) Selection bar.
- [UX-13] (Wave 3) The review content: the next-run line, change marks on the canvas, a schedule diff and a DST note (flagship F.1 and F.2).
- [CS-18] (Wave 3) One diff module for publishing, versions and AI review.
- [UX-14, OP-05] (Wave 3) Compare, clone and roll back.
- [JY-19] (Wave 3) The pill flip only, with no rocket animation.

**Acceptance criteria**
- A clean live flow shows no filled primary button.
- Starting and finishing a run moves no control at 390, 900 or 1440 px. Exactly one Stop is visible per window, and it shows its label at 320, 760 and 1440 px.
- In the detail-secret-missing fixture, the pill's issue count equals the inspector's field issues.
- With a fine pointer, inputs are 32 px with 14 px text. Focusing a template field changes no font size.
- An unnamed step shows its type name as the title. "Test step" on a trigger says why it is unavailable. Child history adds no row above the first field.
- Selecting 10 cards renders exactly one toolbar. Every per-card action can be reached by tap through the selection bar; nothing is hover-only.
- The review's next run equals Mission Control's for the same flow. A schedule change reads "Mo–Fr 07:00 → 06:30", and a publish in a DST week names the shift.
- The review has no unavailable-notes section. In publish-partial, the pill offers the action that completes the publish.
- Rollback and clone:
  - Rolling back from v5 to v3 creates v6 with v3's content and leaves the draft byte-identical.
  - A concurrent publish returns 409.
  - A source version that holds typed credentials is refused with "Zugangsdaten zuerst in den Tresor verschieben".
  - Versions marked for purge are not offered.

**Size:** L.
**Waves:**
- 1: UI-14, the one Stop, the F.2 accessor.
- 2: UI-16.
- 3: UI-07, the UX-13 review, CS-18, UX-14, JY-19.

**Dependencies:** header extraction (E1); the kit, the status map, the action registry and CS-01 (E2); a version-diff endpoint.

**Guardrails and invariants**
- The primary action always opens the full review; it never publishes directly.
- The publish request carries the reviewed revision and digest, and the server refuses a mismatch.
- Activation stays a separate, confirmed action.
- Rollback runs the full publish pipeline:
  - risky effects are confirmed again;
  - it uses CAS on the live revision;
  - timers re-arm from now without catch-up;
  - a paused flow stays paused.
- Rollback and clone run the FD-13 extraction. A source version that holds typed credentials is refused, or converted to secret_ref before the review. Versions marked for purge are never rollback or clone sources.
- The diff endpoint reuses the `diffDetails` redaction under the Versions admin scope.
- Old test evidence is labelled as history.

**Main code areas**
- `easydrag-header.js` and `easydrag-run-stop.js` (from E1).
- New modules: `easydrag-inspector-layout.js`, `easydrag-selection-bar.js`, `easydrag-diff.js`.
- `easydrag-publish.js`, and `easydrag-detail.js` (touched only to delegate until its Wave 4 split).
- The server versions and diff handlers; the read-only next-fire accessor in `timers.go`.

### E5. The Testbank
Pillars: ux, joy, power.

**Goal.** Testing is one safe click, the run can be watched, and the result is the main thing on screen.

**What the user gets**
- "Simulieren · nichts verlässt AuraGo" is preselected. "Echt ausführen" names the effects ("1 Nachricht, 1 Datei") and asks once to confirm them.
- Before a real test or a step test, the steps that will run are outlined and the rest are dimmed.
- Sample data appears as forms titled with step names and can be filled from the last run in one click. No node ids are shown, and JSON is optional.
- A docked strip pushes the canvas up instead of covering it. It shows progress (the Stop stays in its header slot), then "Hat geklappt · 5 Schritte in 0,8 s" with the result. On a phone it is a bottom sheet.
- A sweep runs along the executed path. Waits show "weiter in 4:32".
- "Alle prüfen" reports "3/3 bestanden" from Simulation.
- "Bis hier simulieren", breakpoints, and editing a captured value before continuing, all in Simulation only.
- Later: peek previews, and outcome cards that show the message as it was sent.

**Scope**
- [UX-07, CS-04] (S0 part, Wave 1) Safe test mode by default.
- [UX-19, CS-06, JY-04] (Wave 1) The result opens by itself, after the sweep, inside one honest simulation frame. It shows the run's stored, scrubbed step output preview. Until Wave 2 it opens in the results panel.
- [UX-08, CS-14] (Wave 2) Sample forms, rendered through the field-kind registry. "Testfall" becomes the one German noun for scenarios. Includes the deterministic tier of [AI-08].
- [UX-09] (Wave 2) The Testbank strip. Entry: SSE redaction verified (FD-20).
- Test path outline (Wave 2): "Echt ausführen" and step tests outline the steps that will run and dim the rest.
- [JY-05] (Wave 2) Run bar, live timers, count chips and at most 24 tokens. Following is opt-in.
- [JY-06] (Wave 2) Drain ring for waits.
- [UX-10] (Wave 3) Redesigned without a paused-run state, built on the Wave 2 pins:
  - a breakpoint ends the simulation and captures the outputs so far;
  - "Weiter" starts a new simulation seeded with those captured values.
- [JY-17] (Wave 4) Peek.
- [JY-08] (Wave 7b, after PE-05) Outcome cards, in the security-redesigned form. They show the delivered text from the receipt's scrubbed excerpt (E12), never a client rebuild.

**Acceptance criteria**
- On a flow with doc.pdf_create and notify.telegram, Test preselects Simulieren, and the real-test card names one message and one file. Enter never starts a real test.
- After a successful test, the panel is open and starts with "Hat geklappt" and the output. Each step is listed once, and the simulation label appears once.
- The default sample view shows no node id. "Alle aus letztem Lauf" fills every card, and the following simulation succeeds. Truncated or redacted outputs are refused with a reason.
- Starting a simulation opens no modal. A real test still needs one explicit confirmation that lists every effect, inline in the Testbank or as a dialog. Enter and keyboard shortcuts never reach it. No card is covered by run UI at 1440×900 or 1024×768.
- At 390×844 the Testbank opens as a bottom sheet and covers no step. At 360×480 it collapses to one 44 px bar, and the canvas keeps at least 50 % of the height.
- A step test on a branch outlines only the reachable steps.
- Toggling a breakpoint never changes `model.version`. The live and real invokers have no breakpoint code path (test).

**Size:** XL.
**Waves:**
- 1: UX-07, UX-19, CS-06, JY-04.
- 2: UX-08, UX-09, the test path outline, JY-05, JY-06.
- 3: UX-10.
- 4: JY-17.
- 7b: JY-08.

**Dependencies:** the runs.js split (E1); the kit, the status map and JY-01 (E2); neutral wire tokens (UI-03, Wave 0); count chips need UI-08 (Wave 2 canvas track); D.3 and D.4.

**Guardrails and invariants**
- Purity and effect counts come from the server (CollectEffects and PublishPreview), and unknown effects count as effectful.
- The real-test confirmation is bound to the draft revision and to the effect list the server returned.
- The frame label comes from the run's returned mode.
- Simulation evidence never satisfies the real-test requirement.
- Outcome text is escaped, and files are inert chips.
- Stop during a real run shows "unklar" (`effect_uncertain`) when the engine reports it.
- A pin is refused when it is redacted, truncated or `effect_uncertain`, or when it touched a secret_ref. Pins feed only Simulate and never appear in exports.
- Seeded continues (UX-10) use the shared seeding function of E6. Taint comes from the recorded taint roots, and an output with no recorded taint counts as untrusted.
- Captured debug values stay in window memory.

**Main code areas**
- `easydrag-testing.js`, `easydrag-scenarios.js`.
- New modules: `easydrag-testbank.js`, `easydrag-run-theatre.js`, `easydrag-samples.js`.
- `internal/flows/simulation.go` (stop point and capture); the server simulate adapter.

### E6. Red to green
Pillars: ux, foundation, joy.

**Goal.** Go from a failed run to a green "Behoben im Entwurf" in three actions plus one effect confirmation, without repeating upstream effects. Once the fix is live, the failed run says so.

**What the user gets**
- A plain headline on the failed card, a "Chat-ID prüfen" action that focuses and flashes the field, and "Nur diesen Schritt erneut testen".
- Three honest states:
  - "Simulation geklappt" after a simulated retest;
  - "Behoben im Entwurf · beim 2. Versuch" after a passing real test at the current draft revision;
  - "behoben in v4" on the original failed run's row, once that draft is published.
- "Als Testfall merken".
- No bare FLOW_* code anywhere, and no Retry button on permission errors.
- A deterministic "Warum?" explanation that needs no model call.

**Scope**
- [FD-19] (Wave 2, first, before PE-04) `error_catalog.go`: a class, a retryable flag, a field resolver and remedy keys for every FLOW_* code.
- [UX-12] One error shape everywhere.
- [UX-11] In its security-redesigned form.
- Shared seeding function: one server function seeds every run that starts from stored outputs. That covers the seeded retests here, seeded continues (UX-10, E5) and resume (PE-06, E12).
- [JY-07] The "Behoben" moment.
- [AI-05] The deterministic tier 1. The AI tier moves to E15.
- The hook for PE-06 resume when E12 lands.

**Acceptance criteria**
- A Go test fails when any FLOW_* literal lacks a registry entry, or when any of the 16 locales lacks its title or remedy.
- On a seeded "chat not found", one click on "Beheben" focuses the chat field with the step visible.
- After a real failure, a retest from the failed step runs no upstream step: the timeline lists only the failed step and its descendants.
- Retesting a failed http.request whose host template reads trigger.email is refused before the vault is read, with the same code as at publish.
- A permission failure shows no Retry; a timeout shows one.
- The scripted recovery journey reaches "Behoben im Entwurf" in 3 actions plus the effect confirmation (hard gate).
- Target (advisory): 4 of 5 session participants fix the seeded error within 2 minutes (plan §9 metric).

**Size:** L.
**Waves:** 2 (FD-19, UX-12, UX-11, JY-07, AI-05 tier 1). The resume hook is used in Wave 7b.
**Dependencies:** E1; E5 (pins and the Testbank); FD-11; the PI-14 chat slice (E9, Wave 2) for the 3.3 demo.

**Guardrails and invariants**
- A seeded real retest is allowed only when every reused output is complete and none is `effect_uncertain`.
- Seeded retests, simulated or real, take taint from the original run's recorded taint roots. An output with no recorded taint counts as untrusted. The publish-time taint and URL-target checks run on the retest graph before the vault is read.
- The retest is bound to the current draft revision.
- The server recomputes the effects of the failed step and its descendants and shows them before confirmation.
- The remedy table is static and lives in code. It never fills a field from error text or model output.
- Availability, read-only and spending checks run again at retest time.
- "Behoben im Entwurf" appears only after a passing real (not simulated) test of the same step at the current draft revision. A simulated pass shows "Simulation geklappt". "behoben in v4" appears only once the revision that passed is live.

**Main code areas**
- A new `internal/flows/error_catalog.go`; the shared seeding function in `internal/flows`.
- New modules: `easydrag-errors.js`, `easydrag-repair.js`.
- `stepErrorText` in `core.js`; an additive error-class field on run details.

### E7. The system comes to you
Pillars: ops, ux.

**Goal.** Problems, approvals and next runs reach the owner once, in plain words, with one action that lands on the exact target.

**What the user gets**
- The "Heute" strip, with at most four tiles and the overnight summary folded in [JY-09]. On a phone it is one line, "2 brauchen dich ›", that opens as a sheet.
- A run-health line on every card: ten marks with glyphs, plus the next run.
- Approvals that announce themselves, where durable mode is on:
  - a notification with a countdown;
  - the notification-centre badge;
  - a question-first inbox;
  - a 5-second undo before the decision is sent.
- Where durable mode is off, a "Freigaben einrichten" card that says a restart is needed and that only pure-local flows can be approved.
- Notification settings per flow: on error, on success, "läuft wieder", and "stumm bis …".
- Working deep links everywhere, including from dashboard cron rows.
- Flows in Spotlight (Wave 6a).
- Run search on the server, and run labels.

**Scope**
- [OP-01] A bounded attention endpoint. Its list renders in three places, with no third tab:
  - as the strip;
  - as the "Braucht Aufmerksamkeit" chip;
  - below 560 px, as one line that opens as a sheet.

  Tile kinds: failures, pending approvals, paused flows and "Neu veröffentlichen nötig" (Republish needed, from the FD-17 compatibility check).
- [UX-15], [OP-02].
- [OP-07, UX-16], built on the approval contract `plans/2026-11-05-easydrag-durable-approval-contract.md`: approve or reject only, with the review resolved once at suspension.
- [OP-13], [OP-06] (decision 12).
- [CS-03] Per decision 11 (a): where durable mode is off, the approvals entry becomes the "Freigaben einrichten" card. It fails open.
- [JY-02] Two sounds only: an approval waits, and a background run fails.
- [OP-04] Run search and labels. The label column is designed into the Wave 3 migration.
- [OP-11] (Wave 6a) Flows in Spotlight. Its run action uses the PE-11 form.
- Out of scope this cycle:
  - enabling durable mode without a restart;
  - approval forms, editable review values and outcome ports (brief §7 pattern 9; deferred in §9).
- Deferred: OP-03, OP-10, OP-12, OP-20, and the map view of OP-09.

**Acceptance criteria**
- With one failed flow and one pending approval, the strip shows both tiles, and each opens its target in one click. With nothing to report, it shows one line with the next run.
- A card with 7 good and 3 failed runs shows 10 marks with 3 cross glyphs and announces "7 von 10 erfolgreich". The page uses one batched query.
- A new pending gate produces exactly one notification and a badge within 2 s, driven by the bus event and not by polling. Deciding in one window clears the badge everywhere.
- If 10 s or less remain before expiry, "Freigeben" sends the decision right after an explicit confirmation. If the window closes during the 5-second delay, nothing is sent and the inbox shows "nicht gesendet".
- A Telegram or Push approval message contains no review data (server test). Opened without a session, its link shows the sign-in page and never a decision control.
- Every "managed in EasyDrag" cron row on the dashboard opens its flow (deep-link whitelist test).
- "Stumm bis 18:00" holds failure notices until then and lists them afterwards.
- A run search without a time window is refused. With a window, it finds runs by label, status and step.
- Spotlight finds a flow by name and opens it. Its run action opens the run form with prefilled values marked, and a risky flow asks for confirmation.
- Only the two sounds play, through the desktop sound packs, and none when desktop sounds are off.
- The attention endpoint keeps its row caps and answers within 300 ms with 200 flows of 200 runs each (target, reference profile).

**Size:** XL.
**Waves:** 2 (OP-13), 3 (everything except OP-11 and OP-13), 6a (OP-11).
**Dependencies:** E1; the E2 kit and status map; the F.2 accessor (Wave 1); H.6; the Wave 3 migration. Entry for OP-07 and UX-16: the durable path is accepted on aurago-test with durable_enabled=true.

**Guardrails and invariants**
- Decisions happen only in the admin browser session. Links open the review only.
- Remote messages carry only the flow name, the step label and the expiry.
- The badge count is computed on the server.
- Badge, strip and run marks update from the existing flows_changed bus event, extended with the reasons approval_pending, approval_decided and run_finished. No timer polls more often than every 60 s while the window is hidden.
- The summary reads stored, scrubbed fields only, with no background action (J.4).
- "Fortsetzen" uses the existing enable endpoint, one flow at a time.
- Notification settings (decision 12):
  - Routing (which channels; on error, success or recovery) lives in the flow document and is shown as an outbound effect.
  - Recipients come only from configured channels.
  - Mute and snooze are operational state in library metadata, with their own CAS. They never bump `model.version`.
- Spotlight runs use the same endpoint, and the server decides the risk. Values prefilled from Spotlight or a deep link are untrusted (E14). A flow with any unknown, unavailable or unclassified effect counts as risky.
- Search uses parameterised queries and a required time window. Labels use only the trigger, run and flow roots, are scrubbed, and never feed ids or recipients.

**Main code areas**
- New modules: `easydrag-attention.js`, `easydrag-pulse.js`.
- `easydrag-approvals.js`; `internal/server/flows_bridge_notify.go`; a new attention handler; a batched recent-runs query.
- Desktop core: a shared `desktopAppBadge`; the deep-link handling in `openInitialDesktopApp` (`ui/js/desktop/core/desktop-foundation.js`, which today passes context only for findit); a new flows source in `ui/js/desktop/core/spotlight-runtime.js`.

### E8. A home that starts things
Pillars: ux, joy, power.

**Goal.** The home shows content first, has one way in, and gets a newcomer to a first result in about a minute.

**What the user gets**
- One toolbar (Wave 1): search, chips, the cards/list switch, one "Neu ▾", and one Filter popover. The popover holds folders, tags, favorites, saved filters (which are kept) and the storage note. There is no hero.
- "Neu" opens the canvas at once with trigger tiles.
- One "Importieren…" for flows and packs. Dropping a pack file on the home opens its preview.
- Recipe cards that state the outcome, the needs ("Telegram · eingerichtet") and the blank fields, and a recipe setup form that simulates at once.
- "Starten · 1 Minute", with a checklist that ticks itself.
- Library cards that update in place.
- Blocks as a filter.

**Scope**
- [CS-09, UX-02, CS-02, CS-11] Content-first slice (Wave 1):
  - the hero goes;
  - one "Neu ▾" with a menu;
  - one "Importieren…" that recognises packs;
  - the storage disclaimer, folders and saved filters move into the Filter popover. Saved filters are kept.
- [UX-02, CS-09] The rest (Wave 3): recipe cards with outcome, needs and blank fields, and the attention line from E7.
- [CS-10] (Wave 3) Blocks as a filter chip.
- [UX-01, CS-05] (Wave 1) New flow without a modal. The automatic renaming is cut.
- [UX-03, JY-13] (Wave 1) First-run checklist.
- [JY-20] (Wave 3) Calm library updates.
- Pack drop target (Wave 3, with CS-02).
- [PI-16] (Wave 6a) Recipe instances, with the recipe link in the Wave 5 migration.
- [PI-17] (Waves 6a and 6b) New recipes, each shipping with the trigger it needs.
- [CS-15] (Wave 3) The two-minute automation, used as an acceptance journey.
- Deferred: JY-15 empty-state drawings (§9).

**Acceptance criteria**
- At 390 px, the toolbar and the first three flows are visible without scrolling (Wave 1; from Wave 3, measured with the one-line attention summary present). At 1440 px, the first card starts within 120 px of the content top.
- There is exactly one primary action and one import control. A `.easydrag-pack.json` file opens the pack preview, whether imported or dropped on the home.
- "Neu" opens the editor without a dialog, with the title selected.
- Flow cards have no permanent checkbox; selection appears on demand.
- A background refresh replays zero entrance animations, and focus survives.
- The checklist ticks only from real events, and at most one hint appears per session.
- CS-15 journey: from the home to an active, published bin reminder in 10 actions or fewer at 1440 and 390 px, including the effects review (target).

**Size:** L.
**Waves:**
- 1: the content-first slice, UX-01, UX-03.
- 3: recipe cards, CS-10, JY-20, the drop target, CS-15.
- 6a: PI-16, and the PI-17 recipes whose triggers exist.
- 6b: the remaining PI-17 recipes as their triggers land.

**Dependencies:** the home-grid extraction (E1), the E2 kit, the E7 endpoint (Wave 3 parts), and PI-14 (E9, Wave 5) for recipe instances.

**Guardrails and invariants**
- Recipes and AI drafts open only as drafts.
- Bulk pause stays behind its review, with no bulk resume.
- Recipe copies leave secrets and recipients empty.
- A recipe upgrade creates a draft with a diff.
- A dropped pack goes through the same preview and checks as an imported one, and never imports credentials.
- View state stays per device.

**Main code areas**
- New modules: `easydrag-home-toolbar.js`, `easydrag-firstrun.js`. `home.js` shrinks.
- `templates_builtin.go` (typed inputs and a revision).

### E9. Build with real things
Pillars: ux, power.

**Goal.** Authors pick real devices, chats and topics, see real values, and capture real events instead of typing ids and JSON.

**What the user gets**
- Search in the data tree and an "Einfügen in: Titel" chip. Tapping a chip inserts it without dragging.
- A list mapped into a single field asks once: "erstes", "alle" or "zusammenfügen".
- Every mapped field previews its value. "Vorschau füllen" uses Simulation only.
- The transform helper as a pipeline.
- Pickers with live labels and an explicit "Prüfen" check. The chat picker for message steps comes first (Wave 2).
- "Auf Testereignis warten" captures a real event as the sample.
- "erhält: text" while connecting, with a one-click mapping offer.
- Context-aware quick-add: "Schritte nach: X" from Wave 1; synonyms, favorites first and ">" commands from Wave 4.

**Scope**
- [UX-05] (Wave 2), with the explicit list choice first, all or join (flagship C.3).
- [UX-06] (Wave 2).
- [PI-14] Pickers:
  - Wave 2: the chat options for notify.* steps, from the configured channels and without probing;
  - Wave 5: the full family, after the detail.js split in Wave 4;
  - plus [OP-09]'s resource-kind metadata only.
- [PI-20] (Wave 5), [JY-03] (Wave 4).
- [UX-04] Quick-add:
  - Wave 1 (size S): the pin-drag header "Schritte nach: X", suggested next steps and "Alle anzeigen";
  - Wave 4: synonyms, favorites first (flagship B.2) and ">" commands on the action registry.
- [AI-07] is deferred.

**Acceptance criteria**
- Typing in the tree search filters across all upstream steps. Enter inserts at the target and returns focus to the field. Tapping a data-tree chip inserts it at the "Einfügen in" target without dragging.
- Mapping a list into a single-value field asks for "erstes", "alle" or "zusammenfügen" once, and the card shows the choice.
- A mapped field shows its resolved value without a test run, and "Vorschau füllen" calls only the simulation endpoint (browser test).
- The MQTT picker lists topics from existing subscriptions seen in the last hour, with the last payload and its age, and it never adds a wildcard subscription. A value that no longer exists shows "nicht mehr vorhanden" and a publish issue, without changing the draft.
- Listening on a topic and publishing one message shows the payload within 1 s. The draft starts no run, while a live version bound to the same topic still runs. Capture ends at the first event or after 2 minutes.
- One click fills the Telegram text, and one Ctrl+Z removes it. A catalog-wide test proves that no offer is ever made for secret_ref, URL host, instruction or SensitiveSink fields.
- From a trigger's "+", the quick-add header reads "Schritte nach: Zeitplan" and lists suggested steps first; "Alle anzeigen" opens the full palette (Wave 1). Typing ">" lists commands from the action registry, "Nachricht" finds "Telegram senden", and the synonym keys exist in all 16 locales (Wave 4).

**Size:** XL.
**Waves:**
- 1: the UX-04 header slice.
- 2: UX-05, C.3, UX-06, the PI-14 chat slice.
- 4: JY-03 and the rest of UX-04.
- 5: PI-14 (full family), PI-20.

**Dependencies:** E1 (the detail.js split before Wave 4); E2 (the action registry); E5 (samples for previews); plan C.1, C.2 and J.3.

**Guardrails and invariants**
- Picker labels go through esc(), are truncated and scrubbed, and are fetched with the admin session from bounded sources. Drafts keep ids only.
- "Prüfen" is started by the user and rate-limited, with no background probing; it revises the "ready" non-goal (§9). Folders stay inside the allowed roots.
- The MQTT picker lists only topics from existing subscriptions and never adds a wildcard subscription.
- Previews come only from scrubbed data or the isolated Simulate endpoint. The staleness badge is bound to the revision and the step hash.
- Capture:
  - requires a POST with an admin session and CSRF protection;
  - takes a copy of the event, so a live version bound to the same source still runs;
  - ends at the first event or after 2 minutes;
  - strips auth headers and cookies, scrubs secrets and caps at 256 KiB;
  - refuses MQTT wildcards;
  - marks the payload as untrusted.
- Mapping offers come from the lint metadata and are linted again after insertion.

**Main code areas**
- `easydrag-mapping.js`, `easydrag-transform.js`, `easydrag-palette.js`; `easydrag-detail.js` after its Wave 4 split.
- New modules: `easydrag-picker.js`, `easydrag-preview.js`, `easydrag-commands.js`.
- `flows_handlers_options.go`; a capture endpoint; catalog `OptionsSource` and `ResourceKind`.

### E10. Flows that listen
Pillars: power, ops.

**Goal.** Anything AuraGo can notice can start a flow, and noise never turns into runs.

**What the user gets**
- "Nur starten, wenn …", with the count "startet bei 3 von 120 Ereignissen".
- Time conditions: between two times, weekdays, "älter als".
- Trigger rules written as sentences: debounce, keyed throttle, duplicate window and quiet hours. Safety-relevant triggers carry a badge and are never held back by quiet hours.
- One recovery flow for "Ein Ablauf ist fehlgeschlagen".
- "/gutenacht" chat commands.
- Service outages from Uptime Kuma, and a "Dienste prüfen" step.
- Named watch presets (container, VM, Wi-Fi presence, printer).
- Folder intake and email attachments.
- "Warten auf Ereignis".
- Missed schedules shown, with an explicit catch-up policy.

**Scope**
- Wave 5 (cheap-power track):
  - [PE-15] and [PI-19], on the conditions editor split out in Wave 0;
  - [PE-16], merged with the failure trigger of [PI-05] and [OP-08];
  - nested condition groups (flagship G.3) are deferred (§9).
- Waves 6a and 6b:
  - [PE-14] (Wave 6b);
  - [PI-01] (decision 7; Wave 6a);
  - [PI-02];
  - [PI-03] phase 1 only (named presets, per the security redesign; phase 2 deferred);
  - [PI-04] (verify first that the email trigger carries attachments);
  - [PE-08] short waits (long waits come with PE-07 in Wave 7b);
  - [PE-18].

**Acceptance criteria**
- With "temperature > 30", MQTT samples of 25 and 31 produce one run. Ignored events add no run rows, and "Warum ignoriert?" names the condition that failed.
- 23:30 and 06:59 fall inside 22:00–07:00 and 07:00 does not, including on a DST day. Simulation uses frozen time.
- The run_failed handler starts within 1 s with the run id, step and code. A failing handler never triggers itself.
- 12 messages within 40 s and "Ruhe 30 s" start one run.
- A safety-relevant MQTT event at 23:30, inside quiet hours of 22:00–07:00, starts its run within 1 s.
- The owner's "/gutenacht" starts one run with no LLM call. Messages from a non-owner, a group or a forward start nothing. A flow command named like a built-in ("/stop", "/help") cannot be published.
- One outage transition starts one run, and the baseline poll after a restart starts none.
- The container preset "Container gestoppt" starts one run per state change of the chosen container and none for the baseline after a restart.
- Copying a 40 MB file into a watched folder starts one run, after the copy has finished. The run receives a root-relative path, size, sha256 and MIME type, never the file content.
- "Warten auf Ereignis" on a door sensor continues on its event port within 1 s of the event, and on its timeout port when the time is up.
- With catch-up set to "auslassen", a risky flow runs nothing after a 2-hour outage and shows the missed slots. "Alle nachholen" is refused for a flow with an unavailable node.

**Size:** XL.
**Waves:** 5 (PE-15, PI-19, PE-16), 6a (PI-01), 6b (the rest).

**Dependencies (per item)**
- PE-15, PI-19 and PE-16: only E1 (the fields.js split, and StartResult surfaced for PE-16) and the shared Go/JS condition vectors.
- PE-14: PE-15, and the trigger-rule rows in the field-kind registry.
- PI-01: the PI-14 pickers for chat identities (Wave 5); decision 7.
- PI-02, PI-03 and PI-04: PI-14 and PI-20 (E9, Wave 5); the watch tables in the Wave 5 migration; E7 notifications.
- PE-08 short waits: PE-19 (E11, Wave 6b). Long waits: PE-05 and PE-07 (E12, Wave 7b).
- PE-18: the missed-run table in the Wave 5 migration, and the F.2 accessor.

**Guardrails and invariants**
- Conditions use parity vectors shared by Go and JS.
- Ignored-event statistics stay in memory, bounded per trigger: a counter plus the last 50 ignored events with their reasons, labelled "seit Neustart". They never create run rows. The Wave 5 migration may persist the counter.
- Safety-relevant triggers:
  - bypass quiet hours and duplicate windows;
  - are held by debounce or throttle for at most 60 s;
  - are marked by the owner in the trigger rules, and show a badge on the card;
  - Validate warns when a start or time condition can suppress one.
- Throttle keys are hashed, and each trigger has a cap on pending bytes.
- run_failed:
  - its payload is untrusted;
  - a flow cannot select itself, enforced in the engine;
  - rate limit per source flow;
  - the shared chain depth of 10 applies.
- Chat commands:
  - only from the owner identity, in private chats, never forwarded or edited;
  - deduplicated by the platform message id;
  - arguments are untrusted;
  - the reply goes only to the originating chat;
  - risky commands need an opt-in per command and are hidden from Telegram's "/" menu;
  - built-in command names are reserved: publishing refuses a colliding flow command, and built-ins always win at dispatch.
- Uptime Kuma: the subscription is additive, the agent relay is unchanged by default, and each transition is debounced. Outage payloads are untrusted, because monitor names and messages come from the probed services.
- PI-03 phase 1:
  - only hand-reviewed (tool, operation) pairs with response projection;
  - watcher outputs are untrusted;
  - presence data is opt-in per device, pseudonymised and excluded from packs;
  - fan-out caps;
  - idle load respects the reduction in commit 4067177a3.
- "Warten auf Ereignis" outputs are untrusted.
- Intake:
  - resolves symlinks before the root checks;
  - refuses AuraGo data, vault and config paths;
  - sniffs MIME types and enforces a disk cap;
  - treats file and attachment names as untrusted;
  - passes the run a root-relative path, size, sha256 and MIME type, never the content. File content read later by a step is untrusted.
- Catch-up defaults to "auslassen" for risky flows. "Alle nachholen" is allowed for read-only flows only. A flow with any unknown, unavailable or unclassified effect counts as risky and not read-only.
- Layering:
  - internal/flows imports no agent, server or tools package.
  - The chat command registry, the Uptime Kuma subscription, intake, watchers, run_failed delivery and chat replies are server-side adapters behind flows-owned interfaces (TriggerDispatcher, ChatReplier).
  - Agent and chat code reach flows only through the server bridge.

**Main code areas**
- `catalog_triggers.go`, `conditions.go`, a new `trigger_policy.go` (injected clock).
- `flows_bridge_triggers.go`, `uptime_kuma_runtime.go`, and the chat command registry next to `commands.Handle` (server side, behind the bridge).
- A new `easydrag-trigger-rules.js`; the conditions editor in `easydrag-fields-conditions.js`.

### E11. Flows that remember and compute
Pillars: power.

**Goal.** Flows can count, compare, deduplicate and degrade gracefully, using small primitives that can be tested.

**What the user gets**
- Arithmetic, comparisons, "&" joins, "??" fallbacks and date math, with a live result such as "= 21,5".
- "Nur neue Einträge", with a "geändert" mode. The first run learns ("Kennt jetzt 48 Einträge"). Real tests show "Test: 2 neu · nicht gemerkt" and never change what the flow remembers.
- Merken and Abrufen steps, with a memory panel that shows which run wrote a value.
- Retry 2.0: growing delays, jitter, retry-on classes and Retry-After. Read-only steps retry twice by default, after 2 s and 8 s, on transient errors. The run timeline shows retry backoff and waits as lanes.
- "Mit Ersatzwert weitermachen".
- Waits that free their worker, merge deadlines ("2/3 angekommen") and concurrency keys.

**Scope**
- [PE-10] (Wave 5). The JS evaluator lives in `easydrag-expr.js`.
- [PE-04, CS-13] (Wave 5, after FD-19), with timeline lanes for retry backoff and wait spans (flagship E.1).
- [PE-17] (Wave 5).
- [PE-02, CS-19] (Wave 5): one store with HMAC keys, designed once in the Wave 5 migration.
- [PE-01] (Wave 5), then [PE-03] (Wave 7b) once the demand is clear.
- [PE-19] (Wave 6b).

**Acceptance criteria**
- `{{ (sensor.value - 32) / 1.8 | round(1) }}` gives 21.5 in Go and JS from one shared vector. An expression with more than 256 operations fails validation, and a tainted URL built with "&" is refused.
- Seeded jitter and date math give the same results in Go and JS from shared vectors, and Simulation freezes `run.started_at`.
- With the learn option, the first run over a 50-item fixture goes to "nichts_neu". After 2 items are added, the next run passes exactly those 2. If the downstream step fails, the same 2 are offered again. The store holds only keyed hashes, at most 10,000 per step.
- A real test of that flow shows "Test: 2 neu · nicht gemerkt", and the next live run still treats those 2 as new.
- A counter flow run live three times shows 3 in the panel. A run that fails after a staged write leaves the value unchanged.
- A flow with a Merken or Nur-neue step and the parallel policy fails validation until PE-19 lands.
- Retry timing:
  - A document without the new retry fields reproduces today's attempt timing exactly (golden test).
  - Read-only steps without retry settings retry twice, after 2 s and 8 s, on transient error classes.
  - Non-retryable codes never retry.
  - A Retry-After above the step's maximum delay fails the attempt as not retryable.
- "Mit Ersatzwert weitermachen" passes the typed fallback and marks the step.
- With one worker, branch A waits 60 s while branch B finishes on its own schedule.

**Size:** XL.
**Waves:** 5 (PE-10, PE-04, PE-17, PE-02, PE-01), 6b (PE-19), 7b (PE-03 if confirmed).
**Dependencies:** E1; the Wave 5 migration from the schema roadmap (§8); FD-19 classes for retry-on.

**Guardrails and invariants**
- Memory is read only through a step. There is no `vars` root at render time, so replays stay deterministic.
- Each stored value carries a taint bit.
- Writes commit in the run's terminal transaction, with CAS checked at commit. A commit conflict ends as FLOW_STATE_CONFLICT and lists the effects that already happened.
- Values are scrubbed. Caps apply per value, per flow and globally.
- An item is marked as seen only after the downstream step succeeds.
- Memory is excluded from exports and packs. Simulation reads fixtures and never writes memory. Real tests read live memory but write to a per-test overlay that is discarded at the end.
- Validate refuses the parallel policy for flows with Merken or Nur-neue steps until the PE-19 concurrency keys land (Wave 6b).
- retry_on can only narrow the retryable set. New risky steps retry only on failures that provably happened before sending.
- Retry-After is honoured only up to the step's maximum delay (at most 3,600 s, model.go) and the remaining run timeout. Above that, the attempt fails as not retryable. Jitter is seeded from the run id, node and attempt.
- Expressions have no now(). Date math uses `run.started_at`, which Simulation freezes.
- No fallback value for ai.decision, for steps that gate effects, or for steps with uncertain effects.

**Main code areas**
- `filters.go`, the template parser, `validate.go`, and the shared parity vectors (consumed by `easydrag-expr.js` and `transform.js`; `template.js` does not grow).
- A new `catalog_state.go`, `store_migration_v9.go`, the retry loop in `engine.go`, waits and merge in `nodes_logic.go`, and the run coordinator.

### E12. Effects you can trust at scale
Pillars: power, safety.

**Goal.** Every outward action gets a receipt, so retries, resume and per-item actions never send twice.

**What the user gets**
- "Gesendet · Telegram 8812 · 07:00:03", or "unklar" with the choices "Ist angekommen" and "Erneut senden".
- "Ab hier fortsetzen": only the failed card lights up again.
- "Für jeden Eintrag ausführen": one message per new item, paced, each with its own status.
- Waits of up to 30 days that survive restarts, inside one "Warten" step if decision 9 takes that option.
- Approvals for flows with outward effects, if decision 11 (b) is taken.

**Scope**
- [PE-05]: the contract starts in Wave 0; the implementation lands in Wave 7b with its own migration.
- [PE-06].
- [PE-09], in its security-redesigned form (decision 5).
- [PE-07] and the long waits of [PE-08] (decision 9).
- [UI-06] List-port glyphs, shipping with PE-09.
- Durable approvals for effectful steps (decision 11 (b)), with the reviewed effects inside the approval digest.
- The receipt excerpt that JY-08 reads.

**Acceptance criteria**
- Killing the process between dispatch and confirmation leaves a receipt marked "unklar", and nothing is resent without a decision.
- Every attempt of an http.request step sends the same Idempotency-Key.
- After a fix, resume runs only the failed step. Changing an upstream prompt refuses the resume and names that step.
- Untrusted outputs stay untrusted: resume, seeded retests and seeded continues restore the same taint roots through the shared seeding function (test).
- 40 items at 20 per minute never exceed the limit, and the real-test dialog states the item count. Item mode is refused for a flow with an unavailable node.
- A 2-day pause survives a restart and holds no worker or slot. With durable_enabled=false, its continuation is encrypted with the key derived from the master key (test).
- An approved effectful step's digest covers the reviewed effects, and changing an effect after the review voids the approval.

**Size:** XL. This is the long pole.
**Waves:** 0 (the PE-05 contract), 7b (everything else).
**Dependencies:** `plans/2026-11-04-easydrag-durable-execution-contract.md` and `plans/2026-11-05-easydrag-durable-approval-contract.md`; PE-19 (E11); E1; the shared seeding function (E6).

**Guardrails and invariants**
- A crash in the middle of a dispatch maps to "unklar". Only the operator can resend, and a resend gets a new key labelled "possibly duplicate".
- The idempotency key is a hash of the instance secret, run, path, node and item.
- Receipts store only bounded, scrubbed, non-secret provider ids, plus a scrubbed excerpt of the delivered text of at most 2 KiB for JY-08. Receipt actions need the admin session. Receipts and excerpts are excluded from exports.
- Resume:
  - refused when any reused step has an uncertain or unrecorded receipt;
  - the stored state is encrypted with the checkpoint key when durable mode is configured, and otherwise with a continuation key derived from the master key at startup;
  - bound to the run and revision;
  - excluded from exports;
  - taint roots are restored exactly, through the shared seeding function.
- Item mode:
  - pauses for a confirmation above a threshold (default 5 items);
  - enforces hard limits per destination at dispatch;
  - requires receipts;
  - runs SensitiveSink and URL-target checks per item;
  - is refused for a flow with any unknown, unavailable or unclassified effect.
- Long waits:
  - the duration is never taken from untrusted data;
  - enablement, pause state and grants are checked again on wake;
  - a single-owner lease and an idempotent wake;
  - the continuation key exists even when durable mode is off (derived from the master key; test).

**Main code areas**
- `engine.go`, the `durable_checkpoint_*` files, a new receipts store and its migration, catalog effect metadata (`safe_retry`, `idempotency_key`, `manual_reconciliation`), `flows_tool_invoker*.go`.
- A receipts view in the run-view modules.

### E13. Integration depth
Pillars: power, ux.

**Goal.** The integrations the owner uses daily become first-class steps.

**What the user gets**
- Home Assistant: read several states, browse services with a generated form, target areas.
- Generic tool steps that fail when the tool fails, with known outputs and labels translated in all 16 locales (Wave 5).
- The palette grouped by topic. "Einrichten" opens the right settings page.
- Calendar and todos as data, including a ready-made `agenda_text`.
- A "Durchsage" step with "Probehören".
- A "Desktop-Hinweis" step with "Lauf öffnen".

**Scope**
- [PI-12] (Wave 5, compute and memory track).
- [PI-06], [PI-07], [PI-09] (Wave 6a), and [PI-13] (Wave 4).
- [OP-18] (Wave 6a), merged with [PI-18]. PI-18's "Ergebnis öffnen" action comes later. Verify first how OP-18 overlaps the generic virtual_desktop tool and how that tool's effect is classified (§10).
- Deferred: PI-08, PI-10, PI-11, PI-15 and OP-19.

**Acceptance criteria**
- Choosing `light.turn_on` renders brightness fields from the service schema, and the draft stores plain values.
- Reading several HA states returns them as untrusted values, and attributes outside the allowlist are not returned.
- A generic step whose tool returns IsError goes to its error port. Published flows keep their old behaviour until the owner opts in.
- docker, proxmox and truenas appear under "Infrastruktur", and "Einrichten" opens the Proxmox section.
- "Termine abrufen" returns typed events and an `agenda_text` in the UI language.
- "Probehören" plays the text on the chosen speaker. In Simulation, a Durchsage plays nothing and shows its text. A target that is not a configured device is refused.
- In Simulation, a Desktop-Hinweis shows nothing, and markup in its text renders literally.

**Size:** L.
**Waves:** 5 (PI-12), 4 (PI-13), 6a (the rest).
**Dependencies:** PI-14 (E9); PI-19 (E10, Wave 5) for quiet hours; the media SSRF hardening branch (`claude/media-ssrf-hardening`, ac606f609, not merged yet) merged and verified before PI-09 (Wave 6a entry criterion).

**Guardrails and invariants**
- The HA script-domain refusal stays. Area targets add the effect "kann mehrere Geräte schalten". Attributes come from an allowlist. Multi-state reads are untrusted, like the ha_state trigger.
- For generic steps, the tool status stays the primary success signal. Effectful generic steps do not retry. Learned output shapes keep key names and types only.
- Calendar output is untrusted.
- Durchsage targets are literal values only, and text is capped at 1,000 characters.
- A Desktop-Hinweis is plain text prefixed with the flow name. It is rate-limited, and its click only opens the run view.
- Layering: DesktopNotifier and the media sink are server-side adapters behind flows-owned interfaces. internal/flows imports no server, agent or tools package.

**Main code areas**
- `catalog_actions_home.go`, `catalog_generic*.go`, a new `catalog_actions_media.go`, and `catalog_actions_notify.go` with a `DesktopNotifier` interface.
- `flows_tool_invoker.go`, `flows_catalog_env.go`, `easydrag-palette.js`.

### E14. Callable flows
Pillars: power, ai.

**Goal.** A flow becomes a small app with typed inputs that can be started from a form, from Spotlight or by Aura.

**What the user gets**
- "Mit Formular starten" everywhere: EasyDrag, Mission Control, Spotlight and the phone.
- Aura can call reviewed flows that the owner has opted in, as typed tools. Risky flows ask for confirmation in the signed-in session.

**Scope**
- [PE-11] (Wave 6a), rendered through the field-kind registry.
- The run action of [OP-11] uses this form (Wave 6a).
- [AI-18] Phase 1, chat only, per the security redesign (Wave 7b, after PE-05).
- Deferred: [PE-12], [PE-13], and MCP exposure (decision 8).

**Acceptance criteria**
- Submitting without the required km field shows an inline error and starts nothing. The Mission Control deep link opens the same form, and JSON mode round-trips the values.
- A value prefilled from a deep link or Spotlight and left unchanged counts as untrusted. Once the owner types or changes it, it counts as trusted (test).
- A flow appears as an agent tool only after "für Aura" is switched on through the publish review. A risky call asks for confirmation. Declining creates no run, and an API-key session cannot confirm.
- An Aura call whose argument would set the http.request host is refused before the vault is read. The same flow started from the run form with a typed value runs.
- When Aura calls a flow with an unavailable node, it asks for confirmation.
- Autonomous missions, co-agents and inner-voice turns never see flow tools (test).

**Size:** L.
**Waves:** 6a (PE-11, the OP-11 run action), 7b (AI-18).
**Dependencies:** the enforced switches from E1; E7 (OP-13); E12 receipts before risky flows are offered as tools.

**Guardrails and invariants**
- Deep links may prefill the form but never submit it. The server validates.
- The input root is untrusted for every run started by Aura or by a chat command, and for any value prefilled from a deep link or Spotlight. Only values the owner typed or changed in the run form are trusted.
- Switching "für Aura" on re-runs Validate with the input classified as untrusted. The publish review refuses exposure if an input reaches a URL scheme or host, a secret target or an instruction.
- A flow with any unknown, unavailable or unclassified effect counts as risky, so Aura must confirm.
- Each tool call gets an idempotency key bound to the conversation turn and the argument hash, and is never retried automatically.
- A confirmation is bound to the argument hash, the flow version and an expiry.
- Flow runs cannot call flow tools (plan §7, no uncontrolled recursion).
- Layering: the flow-tool registry and the agent dispatch allowlist live in internal/server. internal/flows imports no agent, server or tools package, and agent code reaches flows only through the server bridge.

**Main code areas**
- The manual-trigger inputs in `catalog_triggers.go`.
- A new `easydrag-runform.js`.
- A flow tool registry in `internal/server`, the agent dispatch allowlist and the prompt manuals.

### E15. AI co-author
Pillars: ai, ux.

**Goal.** AI drafts, explains and repairs through change sets that the owner can review, and the owner's layout is kept. Only the owner applies.

**What the user gets**
- "Ändern mit KI" on the open draft, with at most one clarifying question, answered with the owner's real chats as buttons.
- A ghost review on the owner's own canvas, with acceptance per step and one undo.
- One automatic repair round, and plain words when a proposal still fails.
- A cost line for AI steps and a cap per run.
- A KI tab per AI step: model, latency, tokens, attempts and the fenced parts.
- An optional AI explanation of a failure, with data sent only on request.
- One fallback provider, used only when the provider is unavailable.

**Scope**
- First: verify the proposal budget (flagship I.4) before AI-04's repair round adds calls.
- [AI-01], [AI-03] (on the CS-18 renderer), [AI-04], [AI-13].
- [AI-02] (decision 6).
- [AI-09], [AI-11], [AI-10], and tier 2 of [AI-05].
- Deferred: AI-06, AI-07, the AI tier of AI-08, AI-12, AI-15 and AI-20.
- Cut: AI-14 and AI-19.

**Acceptance criteria**
- An AI change to one parameter of a scheduled flow leaves every other id, position and group unchanged, and republishing keeps the schedules. The brief left the schedule-id question unverified; this test settles it.
- Unchecking one of three proposed steps applies the other two plus their valid edges, and one Ctrl+Z restores the previous state.
- An operation on a secret_ref or an unknown reference is refused with a named issue.
- A stub that answers invalid and then valid returns `repaired: true` after exactly two charged calls.
- The corpus report covers at least two real providers, and adversarial cases never pass. The gate is the pass rate on the N-case corpus. The baseline is the first run with real providers, and a drop of more than 5 percentage points from it fails.
- A run whose AI calls pass the per-run cap stops before the next call with a translated FLOW_* code. Child frames count, and the counter survives a checkpoint.
- With the pinned provider unavailable, an AI step uses the one fallback and marks the step. Any other provider error does not fall back.
- The KI tab shows model, latency, tokens, attempts and the fenced parts of every AI step, with secrets shown only as vault key names.

**Size:** XL.
**Waves:** 7a.
**Dependencies:** FD-13 and FD-17 (E1) before any draft digest reaches a model; CS-18 (E4); E2; `easydrag-model-ops.js` (E1).

**Guardrails and invariants**
- The digest reuses the scrubber, and the server maps references back to ids.
- Operations on secret, credential, URL-target and pinned-block parameters are refused.
- Full Validate, lint and CollectEffects run on the candidate.
- Apply is bound to the change-set hash and to the base revision through CAS.
- The preview is view-only, and new risky effects are listed first.
- Data context is opt-in and names the provider.
- The cost counter is atomic per run, includes child frames, and survives checkpoints.
- A remote fallback for a local primary is a data-egress effect that the owner confirms in the review.

**Main code areas**
- `flows_ai_proposal.go` and `flows_llm.go` in `internal/server`, and a new `ApplyOps` and digest builder in `internal/flows`.
- `easydrag-authoring.js`; the ghost layer in the new `easydrag-diff.js` from CS-18 (today's diff code is `diffText` and `diffMarkup` in `easydrag-publish.js`, plus `easydrag-compare.js`); a new `easydrag-aiview.js`.
- A corpus behind a build tag.

### E16. Phone first
Pillars: ux, joy.

**Goal.** On a phone, EasyDrag is a list of steps with bottom sheets and a bottom action bar, and the graph is a map view.

**What the user gets**
- At 390 px: the flow name, then the steps as rows with key-setting chips.
- Bottom-sheet editing.
- A bottom bar with Testen, Läufe and "⋯", and a labelled Stop during runs.
- A "Liste | Karte" switch and no rail.
- On the home, the one-line attention summary ("2 brauchen dich ›") above the flows, opening as a sheet (E7).

**Scope:** [UX-17, UI-19].

**Acceptance criteria**
- At 390×844, the first three steps are fully visible and there is no rail.
- Editing a field in a sheet is one undo step, and the change appears in the map view.
- Switching between list and map never changes `model.version`.
- During a test, the bottom bar shows a labelled Stop, which is the only Stop in the window, and no run UI covers the list.
- Every canvas edit action can be reached from the list.

**Size:** L.
**Waves:** 4.
**Dependencies:** the kit sheets, the status map and the action registry (E2); the selection bar, the one Stop and the header (E4); card text (E3); the Testbank as a sheet (E5); `easydrag-model-ops.js` for the run-order projection (E1).

**Guardrails and invariants**
- The list is a read-only projection that never writes positions.
- It is a container-query mode, not a user-agent switch.
- Targets are 44 px for coarse pointers.

**Main code areas**
- A new `easydrag-phone-list.js`.
- The kit sheet; the 560 px container queries in `ui/css/desktop-app-easydrag.css`.

## 5. Three sequencing strategies

**A. Craft first.** *Thesis:* fix the visual system, canvas, chrome, Testbank and home before adding power, so that every new feature lands in a finished UI.
- *Wave order:* E1, E2, E3, E4, E5, E6, E8, E7, then E9 to E15.
- *First visible result:* readable type and the colour split in Wave 0, then the calm header and the Testbank.
- *Pros:* fastest joy, the lowest UI rework, and all three judges' UI themes addressed early.
- *Cons:* the owner's most-wanted power (start conditions, memory, chat commands, Home Assistant depth) waits about four waves, and the PE-05 long pole starts late.
- *Risks:* the result is polished but not more capable, and the power-user judge's priorities sit idle.

**B. Power first.** *Thesis:* close the gaps for an "enormously powerful" tool first: E10, E11, E12 and E14 on today's UI.
- *First visible result:* start conditions and expressions in Wave 1.
- *Pros:* fast capability, and the receipts work starts at once.
- *Cons:*
  - new inspectors, cards and errors are built on the illegible canvas and then rebuilt after the tokens and kit;
  - runs.js and editor.js block most UI parts;
  - joy stays at 3–4/10 for months.
- *Risks:* high. XL engine work and new external surfaces land before measurement, the error taxonomy and the trust sweep.

**C. Journey slices on a thin foundation.** *Thesis:* a short Wave 0 clears the deck. After it, each wave delivers one or two signature experiences end to end (UI, engine and copy together), alongside a small cheap-power track and a long-pole engine design track.
- *First visible result:* type and colour in Wave 0, then first success, the calm header, a content-first home and start conditions in Wave 1.
- *Pros:* visible progress every wave, power from Wave 1, and tokens and kit always come before the screens that use them.
- *Cons:* each wave touches more modules, so coordination is harder.
- *Risks:* medium. Waves can bloat. Mitigations:
  - each wave has a demo-based exit, and a demo uses only work from the same or an earlier wave;
  - parallel tracks own disjoint modules and have their own gates;
  - relative durations follow the Appendix A sizes.

| Measure | A Craft first | B Power first | C Journey slices |
|---|---|---|---|
| Time to first visible joy | Wave 0–1 | Wave 4 | Wave 0–1 |
| Time to new power | Wave 4 | Wave 1 | Wave 1 for small primitives, with their own demo and gates; Waves 3–4b for memory, chat and watchers |
| Risk | Low | High | Medium |
| Rework | Low | High: UI rebuilt after the tokens and kit | Low to medium |
| Visible progress per wave | High early, flat later | Low early, high later | High in every wave |

**Decision (2026-11-10): the owner chose A.** Craft comes first:
- Wave 0 stays as planned.
- Waves 1–4 finish the visual system, the canvas, the chrome, the Testbank, diagnosis, the home, attention and approvals, and the phone layout.
- Waves 5–7b add the power and AI epics E9 to E15 into that finished UI (§6).

C remains the fastest way to new power. It was recommended for two reasons. It served all three judges at once, with start conditions and run_failed in Wave 1 and memory and events by Waves 3–4b. It also follows the flagship plan's advice to "build a small number of complete, exemplary journeys first" (`plans/2026-11-04-easydrag-flagship-plan.md`:45).

§5 names three risks for A: the result is polished but not more capable, the power-user judge's priorities sit idle, and the PE-05 long pole starts late. This roadmap limits them as follows:
- Wave 0 is unchanged, so the principal engineer's gates still come first: the splits, the trust sweep and the counter baseline.
- The PE-05 receipts contract and the schema roadmap still start in Wave 0. The long pole and all three migrations (Waves 3, 5 and 7b) are designed before the first craft wave, and Wave 7b can slip without delaying Wave 7a.
- The one power slice a craft demo needs ships early: the PI-14 chat slice in Wave 2, for 3.3.
- Prerequisites stay early, so the power waves start on split modules and on the finished error taxonomy:
  - the F.2 accessor in Wave 1;
  - FD-19 at the start of Wave 2;
  - StartResult (FD-20), fixed in S0c by the Wave 2 exit, which gates OP-01 in Wave 3 and PE-16 in Wave 5;
  - the detail.js split and `easydrag-model-ops.js` at the Wave 4 entry.
- The split-before-use guardrails do not relax. template.js stops growing from the Wave 2 entry, and model.js never grows.
- The pure UX parts of the power epics ship in the craft phase: UX-04, PI-13, UX-05 with C.3, UX-06 and JY-03. Authoring is finished before the pickers arrive.
- The power phase opens with the power-user judge's highest-rated items, each with its own demo and gates. PI-14, PE-15, PI-19, PE-16, PE-02 and PE-10 come in Wave 5, then PI-01, PI-06 and PE-11 in Wave 6a. Every wave that adds a trigger or reading step gates on the source-trust catalog test.
- The craft phase ends on a hard gate. Four checks must pass at the Wave 4 exit before power work begins:
  - the screenshot critiques (craft ≥8, joy ≥7);
  - the owner's Freude median (≥4/5);
  - the scripted NVDA and VoiceOver pass;
  - the full XL performance gate.
- The critiques, the screen-reader pass and the performance gate must still hold at the Wave 7b exit. The Freude median is checked at Wave 4 and again by the release gate.

The cost of A is later power and a longer roadmap:
- four waves later than under C (Wave 1 to 5): start conditions, time windows and run_failed;
- three waves later (2 to 5): expressions, Retry 2.0, fallback values and generic steps;
- two waves later (3 to 5): memory and only-new;
- one or two waves later: the work C placed in Waves 4a to 5b.

The ten waves sum to 22.5 relative units instead of 20.5, for two reasons. The power work no longer runs as small tracks beside craft waves. C's single power-and-craft migration in Wave 3 splits into two.

## 6. Roadmap (strategy A, chosen by the owner on 2026-11-10)

**Rules for every wave**
- A demo may only use items scheduled in the same wave or an earlier one.
- Every Appendix A row that names an epic names its wave in the Wave column, or points to §9. Split items name the wave of each part. A review checks this at each wave's entry.
- The plan has two phases. The craft phase is Waves 1–4 and the power phase is Waves 5–7b.
  - A power item enters the craft phase only as a slice that a craft demo needs. The one case is the PI-14 chat slice in Wave 2, for 3.3.
  - Pure UX items from the power epics stay in the craft phase: UX-04 (both slices), UX-05 with C.3, UX-06 and JY-03 (E9), and PI-13 (E13, the palette by topic, which reworks the same palette module as UX-04).
  - Prerequisites of craft and power work stay early:
    - the F.2 next-fire accessor in Wave 1 (before UI-04, UX-13 and OP-02);
    - FD-19 first in Wave 2 (before UX-12, and before PE-04 in Wave 5);
    - StartResult surfaced (FD-20), fixed in S0c by the Wave 2 exit (before OP-01 in Wave 3 and PE-16 in Wave 5).
- The split-before-use list (E1) is checked at each wave's entry. Under strategy A it reads:
  - Wave 0: runs.js, editor.js, home.js and fields.js;
  - Wave 2 entry:
    - card rendering moves into `easydrag-card.js`, before UI-04, UI-05 and the FD-04 tiers;
    - from here on, template.js (911 lines) does not grow. The template-side JS for UX-05 and UX-06 goes into `easydrag-preview.js` or `easydrag-transform.js`;
  - Wave 4 entry:
    - detail.js splits, before E16, JY-03 and the full PI-14 family in Wave 5;
    - `easydrag-model-ops.js` is created, before both the canvas-handling track (select upstream and downstream, distribute) and E16 (the run-order projection). E15 reuses it in Wave 7a for the client-side change-set apply;
  - Wave 5 entry: `easydrag-expr.js` is created for PE-10's JS evaluator;
  - standing rules:
    - model.js (921 lines) does not grow. New model helpers go into `easydrag-model-ops.js`. If a craft wave needs a model helper before the Wave 4 entry, the module is created then;
    - until its split, detail.js takes no new code. E6 field focus stays in `easydrag-repair.js` and E9 previews in `easydrag-preview.js`. The E15 KI tab goes into `easydrag-aiview.js` in Wave 7a.
- Each wave gate adds a smoke test at 390×844 and 360×480 for that wave's new surfaces.
- Parallel tracks have their own gates. A wave does not exit until they pass, or until an item is explicitly moved to the next wave with a note.
- Parallel tracks own disjoint modules. Where two tracks must change the same file, the wave names that file, its owner and the order of the changes (Wave 5).
- Every wave that adds a trigger or reading step gates on the source-trust catalog test for it (§8). From Wave 5 on this applies to Waves 5, 6a and 6b.
- Every wave that adds a gesture gates on its keyboard path (the FD-06 wiring, from Wave 4) and on CDP touch emulation (§8).
- Relative durations were re-estimated from the Appendix A sizes for strategy A and sum to 22.5 (strategy C: 20.5).
  - Wave 1 keeps 2.5: it lost only C's small parallel power track.
  - Wave 3 rises to 2.5, because it is the heaviest craft wave: six L items, a migration and the durable acceptance on aurago-test.
  - The extra 2 units have two causes. The power work that C ran as small tracks beside craft waves now runs in its own waves. C's Wave 3 migration splits into a craft migration (Wave 3) and a power migration (Wave 5).
  - The durations are not a schedule.
- This section and the Appendix A Wave column are the schedule. All wave references in §3, §4 and §7 to §10 were updated to strategy A on 2026-11-10; if one still disagrees, this section decides.

**Wave 0: Clear the deck** (relative duration 1.5; unchanged from strategy C).
- *Includes:* S0 and S0b (§7):
  - S0: the FD-03 counter baseline (step 0); the FD-10 splits of runs.js, editor.js, home.js and fields.js; the FD-17, AI-16 and OP-17 trust sweep; the FD-12 subset; UI-02 and UI-03 from E3 and E2;
  - S0b: FD-13 and FD-18.
- *Order:* the counter baseline, then the module splits, then the trust sweep, then UI-02 and UI-03. S0b runs alongside on server code.
- *Demo:* a flow opens with readable cards, a clean run looks clean, the test dialog preselects Simulation, and a typed token moves into the vault with one click.
- *Gates:*
  - the counter baseline is committed before UI-02, and UI-02 and UI-03 leave every counter unchanged;
  - runs.js ≤400 lines; editor.js, home.js and fields.js ≤900; no EasyDrag module above 1,000;
  - the fence, sticky-note and read_only tests pass;
  - the S8 fixture is refused;
  - after the FD-13 purge, no plaintext token is left in flows.db, its WAL file or any flows backup.
- *Parallel tracks:*
  - S0c, the rolling hygiene track (FD-15, FD-16, FD-20 and the rest of FD-12). It starts with short verification spikes and runs through Wave 2;
  - the PE-05 receipts contract (design only);
  - the schema roadmap (design only, §8). It now specifies three migrations: Wave 3, Wave 5 and Wave 7b.
- *Decisions due:* 1, 3 and 4.

**Wave 1: Readable and honest** (relative duration 2.5).
- *Includes:*
  - E2: FD-08 with the status map, UI-17, FD-09, FD-11, CS-01, UI-15, JY-01, the action registry, UX-18 and CS-20, the first FD-14 scenes, and the CS-17 ledger and control budgets;
  - E4: UI-14 with the primary-action states and the one Stop, plus the read-only F.2 next-fire accessor;
  - E5: UX-07, UX-19, CS-06, JY-04;
  - E8: UX-01, UX-03, and the content-first home slice (no hero, one "Neu ▾", one "Importieren…", and the disclaimer, folders and saved filters in the Filter popover);
  - E9: the UX-04 quick-add header slice;
  - E3: UI-13, FD-01, FD-05;
  - the touch audit (FD-15 tier T1).
- *Order:* the FD-08 tokens and the status map first. Then FD-09 and CS-01. Then UI-14 (after the F.2 accessor) and the home slice. UX-07 comes before UX-19.
- *Demo:* signature experience 3.1 in Standard dark and Fruity light and dark, with the result in the UX-19 results panel (the Testbank strip arrives in Wave 2).
- *Gates:*
  - zero idle animations;
  - the OKLab token checks across the 14 themes in the lab's first scenes, and the status-map lint;
  - the header and home control budgets;
  - plural tests;
  - exactly one visible Stop per window, and equal control positions before and after a run at 390, 900 and 1440 px;
  - at 390 px, the toolbar and the first three flows are visible without scrolling;
  - screenshot critiques: craft ≥7/10 and joy ≥5/10.
- *Parallel track:* S0c continues (FD-15 tiers T0 and T1, FD-16, FD-20 and the rest of FD-12).
  - StartResult surfaced is fixed with a test here or in Wave 2, and is not deferred.
  - Strategy C's cheap-power track (PE-15, PI-19, PE-16) moves to Wave 5.
- *Decisions due:* 13 by the Wave 1 exit; 2 after the Wave 1 sessions.

**Wave 2: Red to green, and a run you can watch** (relative duration 2.5).
- *Entry:*
  - SSE redaction verified (FD-20);
  - `easydrag-card.js` split out of canvas.js;
  - template.js stops growing (split-before-use list).
- *Includes:*
  - E6: FD-19, UX-12, UX-11 with the shared seeding function, JY-07, AI-05 tier 1;
  - E5: UX-08 with the deterministic tier of AI-08, UX-09, the test path outline, JY-05, JY-06;
  - E9: UX-05 with the C.3 list choice, UX-06 (their template-side JS in `easydrag-preview.js` or `easydrag-transform.js`), and the PI-14 chat slice for notify.* steps;
  - E4: UI-16 with CS-12;
  - E7: OP-13, the deep-link contract. Run links from notifications and the dashboard's "managed in EasyDrag" cron rows go through `openInitialDesktopApp`, which today passes context only for findit.
- *Order:* FD-19 first, because UX-12 needs its classes and PE-04 reads them in Wave 5. UX-09 before JY-05. FD-04 culling before UI-04 adds DOM per card. OP-13 before the 3.3 demo.
- *Demo:* signature experiences 3.2 and 3.3, run after the canvas track's UI-04 and UI-08 land.
  - 3.3 starts from the failure notification, which opens the failed run through the OP-13 run link.
  - 3.3 keeps the PI-14 chat slice in this wave rather than changing the demo, for three reasons. The slice is small: options come from the configured channels, with no probing. It is the fix that the 3.3 failure needs. Without it, the repair journey would end with typing a chat id, which goes against principle 5.
- *Gates:*
  - the FLOW_* registry test;
  - zero invoker calls from Simulation;
  - the seeded-retest taint test;
  - the recovery journey as a scripted test (3 actions plus the effect confirmation). The five-person session is advisory (target: 4 of 5 within 2 minutes);
  - the deep-link whitelist test (OP-13);
  - XL counters do not regress, and the first reference-hardware timings are recorded;
  - phone: at 360×480 the Testbank is one 44 px bar, and the canvas keeps at least 50 % of the height;
  - S0c is closed: every FD-20 item has an outcome, and StartResult is surfaced with a test.
- *Parallel tracks:*
  - canvas: FD-04 with UI-01, UI-04, UI-05, UI-08;
  - S0c closes: every FD-20 item has an outcome before the Wave 2 exit. StartResult surfaced must be fixed, not deferred, because it gates OP-01 in Wave 3.
- *Decisions due:* 10, before the human review round.

**Wave 3: The system comes to you** (relative duration 2.5).
- *Entry:*
  - StartResult surfaced (FD-20), before OP-01, so trigger start failures reach the attention strip;
  - the durable path is accepted on aurago-test with durable_enabled=true (decision 11);
  - the schema roadmap specifies the Wave 3 migration: run labels and notification mute state. The memory, recipe-link, watch and missed-run tables move to the Wave 5 migration.
- *Includes:*
  - E7: OP-01 with the "Republish needed" tile, UX-15 with JY-09, OP-02, OP-07 with UX-16, OP-06, CS-03 as the "Freigaben einrichten" card, JY-02, OP-04 (OP-13 landed in Wave 2);
  - E8: the UX-02 recipe cards, CS-10, JY-20, the pack drop target, and CS-15 as the acceptance journey;
  - E4: UX-13 (review content), CS-18, UX-14 with OP-05, JY-19, UI-07 with CS-16;
  - E5: UX-10 (stop point plus seeded continue, Simulation only, on the Wave 2 pins).
- *Order:* the migration first. The OP-01 endpoint before UX-15 and OP-02. CS-18 before UX-13 and UX-14.
- *Demo:* signature experiences 3.4 (pure-local), 3.5 and 3.6.
- *Gates:*
  - the attention endpoint stays within its caps (300 ms target);
  - the no-review-data test for remote messages;
  - the migration takes a verified backup and refuses old binaries;
  - with durable mode on aurago-test, one full round trip passes: notification, signed-in review, decision with undo;
  - the idle-load check: badge, strip and marks update from the bus, and no timer polls more often than every 60 s while the window is hidden;
  - an NVDA smoke test of the header, the Testbank and the canvas.
- *Parallel track:* UX-10 (the stop point and capture in `simulation.go`, and the Testbank modules), whose modules are disjoint from those of E7 and E4.
- *Decisions due:* 11 (a) and 12.

**Wave 4: Phone first, and a canvas that handles like a design tool** (relative duration 2).
- *Entry:*
  - detail.js is split, before E16, JY-03 and the Wave 5 pickers;
  - `easydrag-model-ops.js` is created, before the canvas-handling track and E16. It starts with the upstream and downstream walk that select upstream and downstream and E16's run-order projection both call. After the entry, each track adds only its own functions.
- *Includes:*
  - E16: UX-17 with UI-19, with the run-order projection in `easydrag-model-ops.js`;
  - E3: FD-06 (keyboard wiring and narrated runs), and the canvas-handling track below;
  - E2: FD-07, the scripted NVDA (Chrome and Firefox) and VoiceOver (Safari) pass;
  - E5: JY-17;
  - E9: the rest of UX-04 (synonyms, favorites first, ">" commands on the action registry), and JY-03 in the canvas-handling track;
  - E13: PI-13, the palette by topic, next to UX-04. Step types added from Wave 5 on declare their topic.
- *Order:*
  - the detail.js split and `easydrag-model-ops.js` first. Then E16 and the canvas-handling track run in parallel;
  - the UX-04 rest and PI-13 one after the other, because both rework `easydrag-palette.js`;
  - the canvas-handling actions (F2 rename, select upstream and downstream, distribute) are registered in the action registry as they land. E16's "every canvas edit action can be reached from the list" test runs at the wave exit;
  - FD-06, then FD-07, last, so the keyboard wiring and the scripted pass cover the new gestures.
- *Demo:*
  - signature experience 3.10;
  - a sticky note explains a group, is pinned to it and moves with it;
  - a 120-step chain laid out in 120 columns and undone with one Ctrl+Z;
  - in quick-add, ">" lists commands and "Nachricht" finds "Telegram senden", and the palette groups steps by topic.
- *Gates:*
  - phone journeys at 320 and 390 px. Every canvas edit action can be reached from the phone list;
  - the FD-06 keyboard wiring tests;
  - the sticky-note tests: undo, fingerprint, markup shown as text, no note in the review or approval dialogs;
  - the scripted NVDA and VoiceOver pass (FD-07);
  - the full XL performance gate on reference hardware (§8);
  - CDP touch emulation at 1024×768 and 390×844 for the gestures this wave adds (§8);
  - screenshot critiques: craft ≥8/10 and joy ≥7/10. The owner's median Freude on the shipped signature journeys (3.1 to 3.6 and 3.10) is at least 4/5.
  - This is the exit gate of the craft phase. Power work starts only on a UI that has passed it.
- *Parallel track:* canvas handling (one track, because these items all touch `easydrag-interact.js`):
  - FD-02 with JY-16 (FD-02 before the glide);
  - the UI-09 and UI-12 parts;
  - edge auto-scroll for drag, connect and box select;
  - distribute and equal-gap guides, F2 rename, select upstream and downstream;
  - UI-11 sticky notes in `easydrag-notes.js`, after the UI-12 group swatches so that notes can pin to groups;
  - then JY-03 ("erhält" while connecting), after the edge auto-scroll helper, because both change the connect gesture.
- *Decisions due:* none.

**Wave 5: Flows that listen and remember** (relative duration 2.5).
- *Entry:*
  - `easydrag-expr.js` created (before PE-10);
  - StartResult surfaced (FD-20) before PE-16. It was fixed in S0c by the Wave 2 exit and has gated OP-01 since Wave 3;
  - the schema roadmap specifies the Wave 5 migration: memory, the recipe link, and the watch and missed-run tables.
- *Includes:*
  - E10: PE-15 and PI-19 on the conditions editor split out in Wave 0; PE-16, merged with the failure trigger of PI-05 and OP-08;
  - E9: PI-14 in full, with OP-09's resource-kind metadata, and PI-20;
  - E11: PE-10; PE-04 with CS-13 and the timeline lanes (E.1); PE-17; the migration; PE-02 with CS-19; PE-01;
  - E13: PI-12.
- *Order:*
  - PE-15, PI-19 and PE-16 come first: they rank high with the power-user judge and depend only on E1.
  - PI-14 comes before PI-20, which shares its capture endpoint.
  - The migration comes before PE-02 and PE-01.
- *Shared files:* the tracks below own disjoint modules except `engine.go`, the run coordinator and `validate.go`. Work on these files is serialised:
  - `engine.go` and the run coordinator, in this order: the PE-16 self-selection check ("a flow cannot select itself"), the PE-04 retry loop, then the PE-02 and PE-01 commit path in the run's terminal transaction (after the migration);
  - `validate.go` has one owner, the compute-and-memory track. It lands the PE-10 operation limit and the PE-02 parallel-policy refusal, and it takes the validation changes of the other tracks.
- *Demo:*
  - a washing machine on MQTT: "Auf Testereignis warten" captures the real payload as the sample, and the MQTT picker lists the topic;
  - with "Nur starten, wenn power < 5" and the time condition "nicht zwischen 22:00 und 07:00", the flow starts one run out of 12 samples;
  - a run_failed recovery flow fires on a seeded failure;
  - a "Nur neue Einträge" flow learns its items and then passes only the new ones;
  - a mapped expression shows "= 21,5".
- *Gates:*
  - start conditions and triggers:
    - the shared Go/JS condition vectors pass;
    - ignored events add no run rows;
    - 23:30 and 06:59 fall inside 22:00–07:00 on a DST day;
    - run_failed starts within 1 s, and a flow cannot select itself;
  - expressions and steps:
    - the expression vector `(sensor.value - 32) / 1.8 | round(1)` gives 21.5 in Go and JS;
    - the seeded-jitter and date-math vectors pass;
    - the Retry 2.0 golden test passes;
    - a generic step that returns IsError takes its error port;
  - pickers and capture: the picker label-scrub test and the no-wildcard test; capture copies the event, and a live version on the same topic still runs;
  - the source-trust catalog test for every new trigger or reading step: the run_failed trigger (untrusted payload), captured test events and the generic reading steps;
  - memory:
    - the migration takes a verified backup and refuses old binaries;
    - the PE-02 learn fixture, the PE-01 counter and the real-test overlay pass;
    - the parallel policy is refused.
- *Parallel tracks:* three tracks:
  - conditions and triggers: PE-15, PI-19, PE-16;
  - compute and memory, as one track because PE-04, PE-02 and PE-01 change `engine.go` and the run coordinator, and PE-10 and PE-02 change `validate.go`: the migration, PE-10, PE-04, PE-17, PI-12, PE-02, PE-01;
  - real things: PI-14, PI-20.
- *Decisions due:* 7, before Wave 6a.

**Wave 6a: Real devices and chat commands** (relative duration 2).
- *Entry:*
  - the media SSRF hardening branch (`claude/media-ssrf-hardening`) is merged and verified, before PI-09;
  - the OP-18 overlap with the generic virtual_desktop tool is verified (§10);
  - decision 7 is taken, before PI-01.
- *Includes:*
  - E10: PI-01;
  - E13 remainder: PI-06, PI-07, PI-09, OP-18 with PI-18 (PI-13 landed in Wave 4, PI-12 in Wave 5);
  - E14: PE-11; E7: OP-11;
  - E8: PI-16, and the PI-17 recipes whose triggers exist.
- *Order:* PE-11 before OP-11. PI-16 builds on the Wave 5 pickers and recipe link.
- *Demo:*
  - signature experience 3.8;
  - the first half of 3.7: capture the real payload, pick the kitchen speaker, map the text in one click, "Probehören";
  - "Mit Formular starten" from Mission Control.
- *Gates:*
  - owner-only, group and forwarded-message tests for chat commands, and the reserved built-in names;
  - prefilled run-form values stay untrusted until the owner changes them;
  - the source-trust catalog test for every new trigger or reading step: chat commands (PI-01), multi-state reads (PI-06) and calendar data (PI-07).
- *Decisions due:* 9, the shape of the "Warten" step, before Wave 6b.

**Wave 6b: The house talks back** (relative duration 2).
- *Entry:* it is verified that the email trigger carries attachments, before PI-04 (§10).
- *Includes:*
  - E11: PE-19;
  - E10: PE-14, PI-02, PI-03 phase 1, PI-04, PE-08 (short waits), PE-18;
  - E8: the remaining PI-17 recipes as their triggers land.
- *Order:* PE-19 before PE-08. PE-14 before PI-02 and PI-03, which debounce through it.
- *Demo:* signature experience 3.7 (complete).
- *Gates:*
  - a watcher idle-load check on aurago-test;
  - the safety-relevant bypass test;
  - the source-trust catalog test for every new trigger or reading step;
  - the unknown-effect catch-up fixture;
  - new gestures (the PE-14 rule rows) have keyboard paths and pass CDP touch emulation.
- *Decisions due:* none new; 6 is due in Wave 7a.

**Wave 7a: Co-author** (relative duration 2).
- *Includes:* E15, with the I.4 budget check first.
- *Demo:* signature experience 3.9.
- *Gates:*
  - the AI corpus with real providers (pass rate against the first real-provider baseline);
  - the per-run AI cost cap;
  - new gestures (E15's per-step accept) have keyboard paths and pass CDP touch emulation.
- *Decisions due:* 6.

**Wave 7b: Act at scale** (relative duration 3).
- *Includes:*
  - E12: PE-05 with its own migration, then PE-06, PE-09 with UI-06 (if decision 5 allows it), PE-07 and the long waits of PE-08 (decision 9), and durable approvals for effectful steps (decision 11 (b));
  - E14: AI-18 phase 1, after PE-05;
  - E5: JY-08, after PE-05;
  - PE-03, if the demand is confirmed.
- *Order:* PE-05 first, then PE-06, then PE-09, PE-07 and the approvals. AI-18 and JY-08 come last.
- *Demo:*
  - a receipts fault demo;
  - Aura starting a reviewed flow from chat with a typed argument;
  - with decision 11 (b), the Rechnungslauf version of 3.4.
- *Gates:*
  - the receipts crash matrix;
  - the scripted NVDA and VoiceOver pass still holds, now including the power-phase surfaces;
  - the XL performance gate still holds on reference hardware;
  - the screenshot critiques hold craft ≥8/10 and joy ≥7/10;
  - the release gates (§8).
- *Decisions due:* 5, 8, 9 and 11 (b).

7a and 7b are the default split. 7b is the riskiest wave, because PE-05 is the long pole, and under strategy A its implementation starts two waves later than under C. The Wave 0 contract and the schema roadmap keep it designed from the start, and 7b can slip without delaying 7a.

Wave 5 is the most crowded power wave: three tracks and a migration. Its shared files (`engine.go`, the run coordinator and `validate.go`) are serialised as listed. An item that misses the exit moves to Wave 6a with a note.

## 7. First sub-project: S0 "Room, trust and legibility"

**Scope**
0. [FD-03] A counter baseline on seeded S, M and XL fixtures: wires painted per drag, group renders, `allRects` calls and card re-renders per run event. It is committed before the module splits and before UI-02.
1. [FD-10] Behaviour-preserving splits:
   - runs.js into four modules behind `ED.runs`;
   - the header out of editor.js into `easydrag-header.js`;
   - the grid render path out of home.js into `easydrag-home-grid.js`;
   - fields.js into `easydrag-fields-conditions.js` (the conditions editor), `easydrag-template-field.js` (the template field) and the remaining field-kind registry, before PE-15 extends the conditions editor;
   - a headroom ratchet test, with the load list and `DESKTOP_APP_ASSETS` updated.
2. [FD-17, AI-16, OP-17 enforcement] Trust sweep:
   - fence the ai.decision `state`;
   - add server bounds for sticky notes (100 notes, 2,048 runes each, Cf characters refused, counted toward the document cap), redact registered secrets in note text in exports, packs and the run view, and keep notes out of AI payloads by allowlist. Notes stay in exports, because they become a canvas feature in Wave 4 (decision 3). Stored notes above the bounds are truncated on load with a one-time notice;
   - enforce read_only on agent run and run_now;
   - show allow_publish as unavailable, with its reason, and update the agent manual's switch section;
   - at startup, mark live flows that the fence would fail closed as "Republish needed" (`service_compat.go`).
3. [FD-12 subset]
   - Simulation is preselected whenever any effect exists, and the mode control gets its own accessible name.
   - A 4,000-character goal counter, with the server-side check covered by a test.
   - Trigger samples translated in all 16 locales.
4. [UI-02] Card type: names 16 px, parameter lines 14 px upright, and captions, pills and port labels 12 px, so no canvas text is below 12 px at 100 %. The automatic-fit floor rises from 0.8 to 0.86.
5. [UI-03] Two palettes:
   - category, group, delivered-wire and unpublished-chip tokens move off the status hues. Categories become six saturated hues on the icon tile only, plus a warm-neutral documents tile and two neutral categories (logic, tool). Delivered wires and the unpublished chip become neutral. All of it is checked with the hue-window and distance rule (§2 principle 2; colourful categories by the owner's decision of 2026-11-10);
   - stored group colour names keep their meaning in documents; only rendering changes.

**Out of scope**
- The full token layer, the status map and the kit.
- The header redesign.
- Living cards.
- FD-13 and FD-18: they form S0b, the next plan, which runs in Wave 0 alongside S0.
- FD-19: it lands first in Wave 2, before PE-04.
- FD-15, FD-16, FD-20 and the rest of FD-12: they form S0c, the rolling hygiene track (Waves 0–2). The agent manual's switch section is the exception and ships in S0 (item 2).
- Any new engine feature, schema migration or locale rewrite beyond the touched keys.

**Success criteria**
- The counter baseline is committed first, and UI-02 and UI-03 leave every counter unchanged.
- runs.js ≤400 lines; editor.js, home.js and fields.js ≤900. All existing EasyDrag Node, static and browser tests pass, with only the load list changed.
- Text size:
  - at 100 % zoom no canvas text is below 12 px;
  - at every automatic fit, on screen, names render at 13.8 px or more and parameter lines at 12 px or more, and captions, pills and port labels at 10.3 px or more;
  - none of the 12 recipe layouts overlaps;
  - each recipe step name at 16 px fits the card, or is truncated with an ellipsis and keeps the full name in the accessible name.
- In Standard dark and Fruity light and dark, every category and group token passes the hue-window and distance rule (§2 principle 2), and no category colour appears outside the icon tile. Status ink reaches 4.5:1 and category tiles 3:1. The other 11 themes pass in a representative screenshot survey.
- On the run-done fixture, no wire or card border uses a category colour.
- The fence regression tests pass for route mode, evaluate mode and the uncertain port. The read_only refusal and the notes refusal are tested. A registered secret in a note never appears in an export or a pack, and the AI proposal payload contains no note text.
- A flow with notify.telegram opens Test with Simulieren selected, and Enter never starts a real test.

**Key design decisions**
- Split by responsibility seams, never by line count (no "runs-part2.js").
- Raise the automatic floor to 0.86, because 14 px × 0.8 = 11.2 px misses the 12 px floor for parameter lines.
  - At 0.86, captions, pills and port labels render at 10.3 px. They keep that size until the FD-04 compact tier (Wave 2) hides them.
  - Rejected alternatives: 14 px for all canvas text, which grows cards to about 84 px and means re-checking every recipe layout; and a floor of 1.0, which fits fewer cards.
  - An explicit Fit keeps zooming to any level.
- Fence each string leaf of the decision state, keeping the JSON type. A mode that cannot take a fenced state fails closed with FLOW_PARAM_INVALID. The startup check names affected live flows before their next run.
- Sticky notes are bounded and secret-scrubbed now, and rendered in Wave 4 (decision 3).
- Before enforcing read_only, confirm the actual configuration key and its default. Show a one-time notice.

**Risks**
- Regressions from the moves. Mitigation: one commit per module, a GitNexus impact check per moved function, and the full EasyDrag test set.
- At the 0.86 floor, fewer cards fit when a flow opens. Check the groups-fifty fixture.
- The colour change is visible in every flow. Mitigation: a before-and-after contact sheet reviewed by the owner.
- Colourful tiles leave few free hues. The ±30° windows around the four status hues leave three bands: olive, teal to petrol, and indigo to berry. Six hues fit there. Teal and petrol are close, so the icon carries that difference. Light themes, and themes with their own status colours, need their own tile values, computed per theme in the lab. The Standard dark values were computed on 2026-11-10: tiles at OKLab L 0.46 sit 0.28–0.33 from the nearest status token.
- The fields.js split touches the conditions editor that PE-15 extends in Wave 5. Mitigation: the split lands first, with its own impact check.
- Other sessions commit in the same checkout. Mitigation: a dedicated worktree.

**S0b and S0c**
- *S0b (Wave 0, server):* FD-13 and FD-18, accepted by the E1 lines for the purge (database, WAL and backups) and for the S8 fixture.
- *S0c (rolling hygiene track, Waves 0–2):* FD-15 tiers T0 and T1 with the Wave 1 touch audit; FD-16; FD-20, each item verify-first; and the rest of FD-12. It starts with short verification spikes. Its two entry criteria, SSE redaction verified (by the Wave 2 entry) and StartResult surfaced (by the Wave 2 exit, before OP-01 in Wave 3 and PE-16 in Wave 5), gate Wave 2 and the later items.

## 8. Quality gates and measures

**Performance**
- From Wave 0, CI asserts deterministic render counters: wires painted per drag, group renders, `allRects` calls and card re-renders per run event. The baseline is S0 step 0.
- Timings are asserted only on the reference profile:
  - p95 input ≤100 ms and p95 pan or drag frame ≤33 ms at 500 steps and 2,000 wires;
  - warm open of a 100-step flow ≤1 s.

  These are plan targets. Nothing has been measured yet. The first reference-hardware timings are recorded at the Wave 2 exit, and each later wave compares against them. The full XL gate is asserted at the Wave 4 exit and must still hold at the Wave 7b exit.
- Each feature that adds DOM per card must show an unchanged or improved counter first.

**Accessibility**
- Lab checks on every scene from Wave 1: text 4.5:1, non-text 3:1, targets of 24 px or more (44 px for coarse pointers), names present, and no state shown by colour alone.
- Accessibility-tree snapshots in CI.
- An NVDA smoke test of the header, the Testbank and the canvas at the Wave 3 exit. The scripted NVDA (Chrome and Firefox) and VoiceOver (Safari) pass is asserted at the Wave 4 exit and must still hold at the Wave 7b exit.
- Every gesture has a keyboard path, a single-pointer path and a tap path, and nothing is hover-only. Touch reachability is checked with CDP touch emulation at 1024×768 and 390×844: first in the Wave 1 touch audit, then in every wave that adds a gesture.

**Themes**
- Full journeys in Standard dark and Fruity light and dark.
- Semantic lab checks and a contact sheet for all 14 themes, reviewed by eye once per wave.
- Themes may change materials only.

**Locales**
- Every key exists in all 16 locales.
- Plural families and placeholder parity are tested.
- The pseudo-locale is used for overflow checks during development.
- A human review round per release, with pt-PT or pt-BR settled by decision 10.

**Safety**
- Simulation makes zero invoker, vault and network calls, by construction and by test.
- Layering:
  - internal/flows imports no agent, server or tools package, and a `go list` import test fails the build if it does.
  - Every new source or sink is a server-side adapter behind a flows-owned interface: the existing ToolInvoker, LLMStepper and SecretReader (`internal/flows/services.go`), plus the new TriggerDispatcher, DesktopNotifier and ChatReplier.
  - Agent and chat code reach flows only through the server bridge.
- Source trust: every new trigger or reading step declares trusted or untrusted in catalog metadata, with untrusted as the default. It is added to the untrusted-output list in `internal/flows/AGENTS.md` (the "Taint: untrusted outputs are ..." entry, lines 694-696). A catalog test fails when the declaration is missing.
- Unknown effects: some decisions depend on IsRisky or "read-only": test mode, Aura confirmation, Spotlight runs, catch-up and item mode. For each of them, a flow with any unknown, unavailable or unclassified effect counts as risky and not read-only. A fixture with an unavailable node covers each surface.
- Seeded runs: seeded retests, seeded continues and resume take taint from the recorded taint roots through one shared function. An output with no recorded taint counts as untrusted.
- Every new surface gets a secret-leak test covering exports, packs, AI payloads, notifications and labels.
- Every new FLOW_* code has a registry entry and translations.
- Every new Effect constant has an `effectOrder` entry and an explicit IsRisky decision.
- No external assets (TestShippedWebAssetsLoadNothingExternal).
- Schema:
  - A schema roadmap is written in Wave 0.
  - Each wave that needs tables takes one migration. Wave 3 adds run labels and notification mute state. Wave 5 adds memory, the recipe link, and the watch and missed-run tables. Wave 7b adds receipts.
  - FD-13 takes a migration only if its purge marks do not fit existing columns.
  - Every migration takes a verified backup and refuses older binaries.
  - Backups taken after FD-13 hold no typed credentials.

**Tests**
- T0 under 60 s: Go unit and Node tests.
- T1: a shared-browser smoke run.
- Every browser test sets reduced motion explicitly and passes both with Windows animations on and off.
- Each wave gate adds a smoke test at 390×844 and 360×480 for that wave's new surfaces.
- Essential tests only, and one combined review per batch.

**Gates per wave**

| Wave | Must pass to exit |
|---|---|
| Every wave | Layering import test; no external assets; secret-leak tests for new surfaces; earlier waves' journeys still pass; phone smoke at 390×844 and 360×480 |
| 0 | Counter baseline before UI-02, unchanged by UI-02 and UI-03; headroom ratchet (no module above 1,000); fence, notes and read_only tests; S8 fixture; FD-13 purge clean (database, WAL, backups) |
| 1 | Zero idle animations; OKLab token checks in 14 themes; status-map lint; header and home control budgets; plural tests; one visible Stop and equal control positions; 390 px home shows toolbar and three flows; critiques craft ≥7, joy ≥5 |
| 2 | FLOW_* registry test; Simulation zero-call test; seeded-retest taint test; scripted recovery journey; deep-link whitelist test (OP-13); XL counters do not regress; first reference-hardware timings recorded; Testbank bar at 360×480; S0c closed, with StartResult surfaced and tested (FD-20) |
| 3 | Attention endpoint within its caps; remote-message redaction test; migration (run labels, mute state) with verified backup and old-binary refusal; durable round trip on aurago-test; idle-load check; NVDA smoke |
| 4 | Phone journeys at 320 and 390 px; every canvas edit action reachable from the phone list; FD-06 keyboard wiring; scripted NVDA and VoiceOver pass; XL performance gate on reference hardware; touch emulation for the new gestures; critiques craft ≥8, joy ≥7; owner's Freude median ≥4/5 (craft-phase exit) |
| 5 | Shared Go/JS condition vectors; ignored events add no run rows; 23:30 and 06:59 inside 22:00–07:00 on a DST day; run_failed within 1 s and no self-selection; expression vector 21.5 in Go and JS; seeded jitter and date math; Retry 2.0 golden test; generic IsError takes its error port; picker scrub and no-wildcard tests; capture copy test; source-trust catalog test for every new trigger or reading step; migration (memory, recipe link, watch and missed-run tables) with verified backup and old-binary refusal; PE-02 learn fixture, PE-01 counter, real-test overlay, parallel policy refused |
| 6a | Owner-only, group and forwarded-message tests for chat commands; reserved command names; prefilled-input trust test; source-trust catalog test for every new trigger or reading step |
| 6b | Watcher idle-load check on aurago-test; safety-relevant bypass test; source-trust catalog test for every new trigger or reading step; unknown-effect catch-up fixture; keyboard path and touch emulation for new gestures |
| 7a | Real-provider corpus against its baseline; per-run AI cost cap; keyboard path and touch emulation for new gestures |
| 7b | Receipts crash matrix; NVDA and VoiceOver pass still holds; reference-hardware performance still holds; critiques hold craft ≥8, joy ≥7 |
| Release | 8 of 10 newcomers succeed within 5 minutes; median Freude ≥4/5 across session participants on the signature journeys |

**Checking joy without telemetry**
- *Scripted journeys (hard gates per wave):* browser tests count actions against the targets below.

  | Journey | Today (estimate from code paths) | Target |
  |---|---|---|
  | First result | about 5 actions plus a scroll | 2 |
  | New flow | 6 | 2 |
  | Mapping a field | 3–4 | 2 |
  | Safe test | 5 or more | 2 |
  | Failure to "Behoben im Entwurf" | 5–6 | 3 plus the effect confirmation |
  | Approval | about 6 | 2 |
  | Live rollback | about 6 | 3 |
  | Phone edit | about 6 | 2 |

- *Screenshot critiques (hard gates):* the two screenshot critiques are re-run with the same rubric at the exits of Waves 1, 4 and 7b. Targets:
  - Wave 1: craft ≥7/10 and joy ≥5/10 (interim targets set by this plan);
  - Wave 4: craft ≥8/10 and joy ≥7/10;
  - Wave 7b: the Wave 4 scores held.

  Missing a target blocks the wave like any other gate.
- *Freude ratings:* at the Wave 4 exit, the owner rates every signature journey shipped so far from 1 to 5. The median must be at least 4 (hard gate).
- *Usability sessions, five per wave (advisory):* the owner, two newcomers, one power user and one person on a phone. Think aloud, fixed tasks, and record success, time and errors. After each task, ask for a 1–5 "Freude" rating and one word. The release gates are the plan's 8 of 10 newcomers within 5 minutes and a median Freude of at least 4/5 across participants on the signature journeys.
- *Screenshot surveys:* the lab contact sheet at five widths. The owner marks each scene legible, calm, delightful or confusing. Any scene that gets worse blocks the wave.
- *Moment checklist:* every signature moment shows a real outcome, plays at most one sound, uses motion only with meaning, reaches the same end state under reduced motion, and uses outcome-first copy.

## 9. Cuts, non-goals and deferrals

**Cut by verification or security**
- AI-19 agent step: it conflicts with plan §7 line 269 (no uncontrolled recursive agent or tool calls), and the security review asked for a redesign.

**Cut, with reasons**
- JY-12 Spielwiese: a separate practice mode adds server state, and safe Simulation already makes trying things safe.
- JY-14 theme run flavours: decoration in 14 themes. Each theme's character comes from its materials.
- JY-10 milestone bursts: the real result and "Live v3" are the reward, and repeated celebrations wear off.
- JY-11 impact counters: vanity metrics and a schema bump. Run marks answer the real question.
- AI-14 AI-suggested names: a provider call for a two-word label.
- The automatic renaming in CS-05: it creates draft churn and complicates undo.
- Removing saved filters (the CS-11 removal): the judges rejected it, so saved filters move into the popover.
- Knife cut and Alt-drag detach (part of UI-09): a zoo of gestures with little value at personal scale.
- Archive state (part of OP-20): pausing plus a folder covers it.
- Environments and staging: draft versus live, Simulation, Testfälle and live rollback cover staging at personal scale.
- Tools inside ai.step: the same reason as the AI-19 cut (plan §7, no uncontrolled tool calls).

**Deferred** (revisit after Wave 7b or when a dependency lands)
- *Canvas and visuals:*
  - UI-10 routing;
  - UI-18 theme character;
  - super-cards for groups (part of UI-12);
  - JY-18 replay;
  - JY-15 empty-state drawings: revisit after the Wave 3 home redesign. Until then, empty states get outcome-first copy and one action.
- *Runs and data:*
  - E.2 media previews in runs: after JY-08 (Wave 7b) proves the scrubbed-outcome path. Until then, files stay inert chips.
  - H.5 compensation: after PE-05. Compensation is never presented as undo.
  - G.3 nested AND/OR condition groups: the flat all/any groups cover the start-condition and time-window cases in this plan. The condition format reserves one nesting level, so G.3 can land without a migration.
- *Approvals:* approval forms, editable review values and outcome ports (brief §7 pattern 9; brief §8.5, "approve or reject only"). The approval contract resolves the review once at suspension and has no alternate branch. Forms wait until decision 11 (b) has run on aurago-test.
- *Operations:*
  - OP-03 Tagesband;
  - OP-12 widget;
  - OP-10 integration status;
  - OP-09 map view;
  - OP-14 packs 2.0 (security redesign: export first);
  - OP-15 importer (security redesign: Home Assistant phase only);
  - OP-16 Gegenprobe;
  - OP-20 trash.
- *Integrations:*
  - PI-08 office documents;
  - PI-10 knowledge steps;
  - PI-11 paid steps (security redesign: reserve and commit, risky spend effect);
  - PI-15 and OP-19 "Automatisieren", together with "Öffnen mit" for pack files from the Files app;
  - PI-03 phase 2: site_monitor, Frigate events beyond person_detected, RTL-SDR, Bluetooth, TrueNAS and Google Calendar, after the phase 1 idle-load check;
  - the egg_hatched and nest_cleared triggers (part of PI-05). The bridge comment at internal/server/flows_bridge_triggers.go:21-24 only records that these events start no flows; it gives no design reason. They are deferred for little value at personal scale, and revisited once PE-16 wires run_failed through the same bridge path.
- *Callable flows:*
  - PE-12 API tokens;
  - PE-13 flow.call;
  - MCP exposure;
  - a webhook response body, which ships with PE-12 because a computed answer exposes flow data to whoever holds the webhook URL.
- *AI:*
  - AI-06, AI-07, the AI tier of AI-08, AI-12, AI-15 and AI-20;
  - the report step of OP-08;
  - the open-result action of PI-18.

**Contrarian recommendations accepted**
- No global "Einfach/Experte" switch.
- No "Bauen | Testen | Läufe" mode switch.
- No camera that follows a run by default: following is opt-in, and "Zum aktiven Schritt" is always there.
- Celebrations only for the first success and the publish moment.
- No layouts per theme.
- No data tables, KV store or shared variables. Only bounded per-flow memory and "Nur neue Einträge".
- logic.wait and durable_wait are not merged in the engine now, and durable steps stay hidden while durable mode is off. Decision 9 proposes one user-facing "Warten" step that picks the engine path by duration.
- No second open-ended chat surface by default (decision 6).
- AI-suggested names are dropped.
- The file.write default is decision 4, recommended unchanged.

**Contrarian recommendations not accepted**
- "No breakpoints": UX-10 is kept in a form without a paused-run state, which removes the worker-holding concern.
- "No variables": PE-01 is kept on the same bounded store, because the power-user judge rated it 9/must.
- "Retire sticky notes": the owner declined it (decision 3). "Retire the rail" stays decision 2.

**Retained non-goals**
- No framework or engine replacement.
- No WebGL renderer below about 1,000 nodes.
- No exactly-once wording, and compensation is never presented as undo.
- No arbitrary cycles.
- No decisions in chat or from deep links.
- No metric store and no automatic retry of live runs.
- No marketplace.
- No AI-written code nodes.
- Simulation never calls out.
- No nested or durable batches, and no whole-batch retry.
- Blocks contain no waits, stops, tools, models or secrets.
- The durable path stays pure-local (revised below).
- Groups never change execution. The FD-04 group overview tier is view-only, so this still holds.
- Favorites stay on the device.
- Packs exclude credentials, and there is no live collaboration.
- "Ready" means the configuration was checked, not that a connection was probed (revised below).
- Node outputs stay at most 5 MiB and are not streamed. Large binary data stays out of flow JSON (plan §7).

**Revised non-goals**
- "No effectful batches": revised for PE-09 item mode, only after PE-05 receipts and with a confirmation threshold and hard rate limits (decision 5).
- "The durable path stays pure-local": revised in two cases:
  - for PE-07 and PE-08 long waits, only after PE-05 receipts and PE-19 (decision 9). Enablement, pause state and grants are checked again on wake;
  - for approvals of effectful steps, only with decision 11 (b).
- "Ready means checked configuration": revised for PI-14. "Prüfen" is a probe the user starts. It is rate-limited and never runs in the background. OP-10 stays deferred.

**Notable overrides of the judges' scores**
- OP-07 and UX-16 (7.3 and 7.0) wait until Wave 3 and ship as decision 11 (a). Approvals exist only on the durable path, which is off by default and has never been activated in production. Gating effectful steps waits for PE-05 (decision 11 (b), Wave 7b).
- PI-14 (8.3) lands in two parts:
  - the chat picker for notify.* steps in Wave 2, because it fixes the failure shown in 3.3;
  - the full picker family in Wave 5, after the detail.js split and next to the capture endpoint it shares.
- PI-01 (7.7) waits for Wave 6a, after the pickers and event capture of Wave 5, as the power-user judge advised.
- PE-09 (7.3) moves to Wave 7b, because it needs receipts and decision 5.
- JY-05 (5.7) stays in, because it is the live-run signature experience.
- FD-03 (5.3) moves to Wave 0 as S0 step 0, because the principal engineer rated it 8/must and every later counter gate needs its baseline.
- UX-03 (5.0, one cut vote) stays in Wave 1: it closes flagship A.3 and A.4 and serves the joy target. JY-13's quest framing is dropped.
- CS-17 (4.3, one cut vote) is a gate only for the header and home control counts.
- PI-09 (5.0) stays as the payoff of 3.7. If the media SSRF branch is not merged by Wave 6a, 3.7 uses a Push step instead.
- FD-15 (4.3) and FD-16 (4.7) leave Wave 0 for the S0c hygiene track. Only the agent manual's switch section ships in S0.
- JY-15 (3.7, one cut vote) is deferred.

## 10. Open decisions for the owner

1. **German form of address.** **Decided on 2026-11-10: lowercase "du"** (the owner's choice; the proposed default was capital "Du").
   - Options were capital "Du", lowercase "du" and "Sie". A count found about 51 capital and 10 lowercase forms in `ui/lang/easydrag/de.json`; sentence-initial forms are included, so this is approximate. The desktop locale is split about evenly.
   - Consequence: S0 writes every touched or new German key with lowercase "du", "dich", "dir" and "dein". The forms keep a capital only at the start of a sentence. UX-18 (Wave 1) converts the remaining capital forms in `ui/lang/easydrag/de.json`. A lint fails on a capital "Du", "Dich", "Dir" or "Dein…" that is not at the start of a sentence.
2. **The rail.** Options: label it (UI-15), or retire it in favour of a "+ Schritt" button and menus (CS-08).
   - *Default:* label it in Wave 1, and decide on retiring it after the Wave 1 sessions.
   - *Needed by:* the Wave 2 entry.
3. **Sticky notes.** **Decided on 2026-11-10: free notes on the canvas (UI-11)**. This was the owner's choice; the proposed default was to retire the field.
   - S0 bounds notes on the server and redacts secrets in their text.
   - Wave 4 renders them (E3).
4. **file.write default.** Options:
   - keep "overwrite", which is idempotent on retry and will be shown on the card as "überschreibt";
   - switch new steps to "unique", which never overwrites but duplicates files on retry.
   - *Default:* keep overwrite and make it visible.
   - *Needed by:* Wave 0 (S0c applies it).
5. **Effectful item mode (PE-09).** It revises the non-goal "no effectful batches". Options: never, or after receipts with a confirmation threshold and hard rate limits.
   - *Default:* after PE-05, with a threshold of 5 items.
   - *Needed by:* Wave 7b.
6. **Copilot surface.** Options: an "Ändern mit KI" bar with one clarifying round, or a docked multi-turn pane (AI-02).
   - *Default:* the bar first; the pane only if the AI-13 corpus meets its target.
   - *Needed by:* Wave 7a.
7. **Scope of chat commands.** Options: all channels, or Telegram and AuraGo chat first; risky commands allowed or not.
   - *Default:* Telegram and AuraGo chat, owner-only, risky commands only with an opt-in per command.
   - *Needed by:* Wave 6a, before PI-01 (chat commands land in Wave 6a).
8. **External call surfaces.** Options: per-flow API tokens (PE-12) and MCP tools now, or later.
   - *Default:* none this cycle; Aura chat tools only.
   - *Needed by:* Wave 7b, because it bounds AI-18.
9. **Long waits.** Options:
   - (a) keep durable mode as the only persistent wait;
   - (b) build PE-07 continuations;
   - (c) as (b), and offer one user-facing "Warten" step with three modes (Dauer, Uhrzeit, Ereignis). The engine path is chosen by duration, and durable_wait stays hidden.
   - *Default:* (c). Durable mode stays off by default. The step ships with short waits in Wave 6b and gains long waits only after PE-19 and PE-05 (Wave 7b). With durable mode off, the continuation key is derived from the master key (E12).
   - *Needed by:* Wave 6b for the step shape, Wave 7b for long waits.
10. **Portuguese locale.** Options: pt-PT or pt-BR for "pt".
    - *Default:* make the choice before the Wave 2 human review, so translators work against one variant.
    - *Needed by:* the Wave 2 review round.
11. **Approvals for everyday flows.** Today approvals exist only as `logic.durable_approval` (`plans/2026-11-05-easydrag-durable-approval-contract.md`). They are off by default, switched on only with a restart, limited to manual pure-local flows, and have never been activated in production. Options:
    - (a) keep durable mode opt-in, and replace CS-03's hiding with a "Freigaben einrichten" card that says a restart is needed and that only pure-local flows can be approved;
    - (b) after PE-05 receipts, let durable_approval gate effectful steps, with the reviewed effects inside the approval digest;
    - (c) turn durable mode on by default once the receipts crash matrix passes.
    - *Default:* (a) in Wave 3 and (b) in Wave 7b. (c) is decided after (b) has run on aurago-test. Enabling durable mode without a restart is out of scope this cycle.
    - *Needed by:* (a) before Wave 3, (b) before Wave 7b.
12. **Where notification settings live (OP-06).** Options:
    - in the flow document: muting then creates "Entwurf geändert" and needs a publish;
    - in library metadata: CollectEffects then cannot show the routing as an outbound effect.
    - *Default:* split them. Routing (which channels; on error, success or recovery) lives in the document and is reviewed as an effect. Mute and snooze ("stumm bis …") are operational state in library metadata with their own CAS. They never bump `model.version` and are shown in the pill popover.
    - *Needed by:* Wave 3.
13. **logic.stop semantics (FD-20, defect E3).** Options: logic.stop ends the whole run or only its own branch, and the run then shows as succeeded or with its own stopped state.
    - *Default:* the FD-20 spike first records today's behaviour and proposes one option. Whichever is chosen gets a status-map entry and a test.
    - *Needed by:* the Wave 1 exit.

**Engineering questions** (settled by the named epic, verify first)
- Does the email trigger carry attachments? Settles PI-04 (E10), before Wave 6b.
- Can flows reach the generic virtual_desktop tool, and how is its effect classified? Settles OP-18's overlap (E13), before Wave 6a.
- Can ai.decision images reach an external provider, and how are they charged? Settles AI-09 and AI-10 (E15).
- What does a retry do when it hits FLOW_FILE_EXISTS? Settles decision 4 and PE-04's retry classes (E11).
- Is fencing the owner's own proposal goal the intended trust model? Settles the AI-01 digest (E15).
- Do FD-13's purge marks fit existing columns? Settles the schema roadmap (Wave 0).

**Where this revision departs from a review suggestion**
- The UI-09 and UI-12 parts and the canvas-handling set sit with FD-02 and E16 in Wave 4, the phone-and-canvas wave, and not in Wave 2 as proposed, because Wave 2 also gains UI-08, OP-13, the PI-14 chat slice and AI-05 tier 1.
- AI-05 tier 1 stays in Wave 2 with E6. The feasibility review proposed Wave 3; the completeness review proposed Wave 2.
- E6's simulated pass reads "Simulation geklappt", the status-map word, not the proposed "Simulation bestanden".
- The Freude gate at the Wave 4 exit (the craft-phase exit) uses the owner's own ratings, because five-person sessions are advisory per wave. The five-person median is a release gate.

## Appendix A. Idea index

Verification: P = partly exists, N = new, U = unclear, C = conflicts. Security verdict: g = OK with guardrails, r = redesign, o = OK, blank = not reviewed. "→ X" means merged into idea X.

Wave: 0 to 7b as in §6 (strategy A). S0c = the rolling hygiene track (Waves 0–2). A slash lists the waves of a split item. "–" = cut or deferred (§9). "dec. N" = waits for decision N.

| ID | Title | Epic or status | Wave | Size | Verif. | Score |
|---|---|---|---|---|---|---|
| UI-01 | Semantic zoom ladder | E3 → FD-04 (group tier) | 2 | L | P | 6.3 |
| UI-02 | Card type for 100 % zoom | E3 (S0) | 0 | S | P | 9.0 |
| UI-03 | Two palettes | E2 (S0) | 0 | S | P | 9.0 |
| UI-04 | Living node cards | E3 | 2 | L | P/g | 7.7 |
| UI-05 | Shapes per concept | E3 | 2 | M | P | 6.0 |
| UI-06 | Port glyphs | E12 (with PE-09) | 7b | M | N | 4.7 |
| UI-07 | One selection toolbar | E4 | 3 | M | N | 7.0 |
| UI-08 | Wires: direction, counts, focus | E3 | 2 | M | P | 6.3 |
| UI-09 | Wire gestures | E3 (insert with room, output-end reconnect, insert ghost); knife and detach cut | 4 | L | P | 5.3 |
| UI-10 | Orthogonal routing | Deferred | – | L | N/g | 3.3 |
| UI-11 | Sticky notes | E3 (rendering); bounds and secret scrub in E1 (S0); decision 3 | 0/4 | M | P/g | 5.3 |
| UI-12 | Groups as places | E3 (swatches, header band); super-cards deferred | 4 | M | P | 5.3 |
| UI-13 | Framing and minimap | E3 | 1 | M | P | 7.0 |
| UI-14 | Header: one pill | E4 | 1 | M | P/g | 8.0 |
| UI-15 | Labelled rail, icon map | E2; decision 2 | 1 | S | P | 5.7 |
| UI-16 | Inspector as one form | E4 | 2 | M | P | 7.3 |
| UI-17 | Token system lint | E2 | 1 | M | P | 6.0 |
| UI-18 | Theme canvas character | Deferred | – | L | P | 3.3 |
| UI-19 | Phone lane view | E16 → UX-17 | 4 | L | N | 4.0 |
| UI-20 | Render budget | E3 → FD-01/03/04/05 | 0–2 | L | N | 4.3 |
| UX-01 | New flow without modal | E8 | 1 | S | P | 7.0 |
| UX-02 | Neu split button, honest recipes | E8 (Neu menu, then recipe cards) | 1/3 | M | P | 6.7 |
| UX-03 | First-result checklist | E8 | 1 | M | P | 5.0 |
| UX-04 | Context quick-add, commands | E9 (header slice, then synonyms and commands) | 1/4 | M | P | 6.3 |
| UX-05 | Data tree search, target chip | E9 | 2 | S | P | 7.7 |
| UX-06 | Live previews, pipeline | E9 | 2 | M | P/g | 7.3 |
| UX-07 | Safe testing by default | E5 (part in S0) | 0/1 | S | P/g | 8.0 |
| UX-08 | Sample data as forms | E5 | 2 | M | P/g | 7.7 |
| UX-09 | The Testbank | E5 | 2 | L | P/g | 7.7 |
| UX-10 | Debug in Simulation | E5 (no paused state) | 3 | L | N/g | 6.0 |
| UX-11 | Red turns green | E6 (redesigned) | 2 | L | P/r | 7.7 |
| UX-12 | No raw codes | E6 | 2 | S | P | 7.7 |
| UX-13 | Publishing as a moment | E4 (review content; button states ship with UI-14) | 1/3 | M | P/g | 7.0 |
| UX-14 | Versions: compare, clone, roll back | E4 | 3 | M | P/g | 5.3 |
| UX-15 | Heute strip | E7 | 3 | M | P | 7.3 |
| UX-16 | Approvals that find you | E7 (with OP-07); decision 11 | 3 | L | P/g | 7.0 |
| UX-17 | Phone step list | E16 | 4 | L | N | 6.7 |
| UX-18 | One voice | E2; decision 1 | 1 | M | P | 6.3 |
| UX-19 | Success shows the result | E5 | 1 | S | P | 8.0 |
| PE-01 | Flow memory | E11 | 5 | L | N/g | 6.7 |
| PE-02 | Only new items | E11 | 5 | M | N/g | 7.7 |
| PE-03 | Digest basket | E11 (late, if demand is confirmed) | 7b | M | N/g | 5.7 |
| PE-04 | Retry 2.0 | E11 | 5 | S | P/g | 6.3 |
| PE-05 | Effect receipts | E12 (contract in Wave 0) | 0/7b | XL | P/g | 6.7 |
| PE-06 | Resume from failed step | E12 | 7b | L | N/g | 6.3 |
| PE-07 | Long waits | E12; decision 9 | 7b | L | P/g | 5.7 |
| PE-08 | Wait for event | E10 (short waits); long waits in E12 | 6b/7b | L | N/g | 6.7 |
| PE-09 | Item mode | E12 (redesigned); decision 5 | 7b | L | P/r | 7.3 |
| PE-10 | Expressions | E11 | 5 | M | N/g | 7.0 |
| PE-11 | Run with form | E14 | 6a | M | P/g | 7.7 |
| PE-12 | Callable API endpoint | Deferred; decision 8 | – | L | N/g | 5.3 |
| PE-13 | Call a flow | Deferred | – | XL | P/g | 4.7 |
| PE-14 | Trigger rules | E10 | 6b | M | P/g | 7.0 |
| PE-15 | Start conditions | E10 | 5 | S | N | 8.0 |
| PE-16 | run_failed trigger | E10 | 5 | S | P/g | 7.0 |
| PE-17 | Fallback value | E11 | 5 | S | N/g | 5.7 |
| PE-18 | Missed schedules | E10 | 6b | M | P/g | 5.3 |
| PE-19 | Parallel control | E11 | 6b | M | P | 5.3 |
| PI-01 | Chat commands | E10; decision 7 | 6a | L | P/g | 7.7 |
| PI-02 | Service health | E10 | 6b | L | P/g | 7.0 |
| PI-03 | Watch presets | E10 (phase 1, redesigned); phase 2 deferred | 6b | XL | N/r | 6.7 |
| PI-04 | Document intake | E10 | 6b | L | N/g | 6.3 |
| PI-05 | AuraGo events as triggers | E10 → PE-16; egg/nest deferred | 5 | M | P/g | 4.0 |
| PI-06 | Home Assistant in depth | E13 | 6a | M | P/g | 7.3 |
| PI-07 | Calendar and todos | E13 | 6a | M | P/g | 6.0 |
| PI-08 | Office documents | Deferred | – | M | P/g | 3.7 |
| PI-09 | Durchsage | E13 | 6a | M | P/g | 5.0 |
| PI-10 | Knowledge steps | Deferred | – | M | N/g | 4.7 |
| PI-11 | Paid steps with cap | Deferred (redesign) | – | L | P/r | 4.3 |
| PI-12 | Generic steps first-class | E13 | 5 | M | P/g | 7.3 |
| PI-13 | Palette by topic | E13 | 4 | S | P | 6.0 |
| PI-14 | Resource pickers | E9 (chat slice, then full) | 2/5 | L | P/g | 8.3 |
| PI-15 | Automate from other apps | Deferred | – | M | P/g | 5.0 |
| PI-16 | Recipe instances | E8 | 6a | L | P | 6.3 |
| PI-17 | Second recipe wave | E8 (with their triggers) | 6a/6b | M | N | 5.3 |
| PI-18 | Desktop notification step | E13 → OP-18 | 6a | S | P/g | 3.7 |
| PI-19 | Time conditions | E10 | 5 | S | P | 7.3 |
| PI-20 | Wait for test event | E9 | 5 | M | N/g | 7.0 |
| AI-01 | Change sets | E15 | 7a | L | N/g | 7.0 |
| AI-02 | Copilot pane | E15; decision 6 | 7a | L | N/g | 6.0 |
| AI-03 | Ghost review | E15 | 7a | M | P/o | 6.0 |
| AI-04 | One repair round | E15 | 7a | S | N/g | 5.7 |
| AI-05 | Explain a failure ("Warum?") | E6 (tier 1), E15 (tier 2) | 2/7a | M | P/g | 6.0 |
| AI-06 | Repair workshop | Deferred | – | L | N/g | 4.7 |
| AI-07 | Expressions from language | Deferred | – | M | N/g | 4.7 |
| AI-08 | Test data on demand | E5 (deterministic tier); AI tier deferred | 2 | M | N/g | 4.7 |
| AI-09 | Cost per step and run | E15 | 7a | M | P/g | 5.7 |
| AI-10 | Fallback provider | E15 | 7a | S | N/g | 4.7 |
| AI-11 | What the AI saw | E15 | 7a | M | N/o | 5.7 |
| AI-12 | Real-AI scenario bench | Deferred | – | L | N/g | 3.3 |
| AI-13 | Quality corpus | E15 | 7a | M | P/g | 4.3 |
| AI-14 | Suggested names | Cut | – | S | N/g | 3.0 |
| AI-15 | Sanitize and guard steps | Deferred | – | M | N/g | 4.7 |
| AI-16 | ai.decision fence | E1 (S0) → FD-17 | 0 | S | N/g | 7.0 |
| AI-17 | Teach Aura, honest switches | E1 → OP-17 | 0 | S | P/o | 4.3 |
| AI-18 | Flows as Aura's tools | E14 (chat-only, redesigned) | 7b | XL | N/r | 6.0 |
| AI-19 | Agent step | Cut (conflicts) | – | L | C/r | 3.3 |
| AI-20 | Save chat as automation | Deferred | – | L | N/g | 5.0 |
| JY-01 | Delight kernel | E2 | 1 | M | P | 5.7 |
| JY-02 | Desktop sounds | E7 (two sounds) | 3 | S | P | 4.3 |
| JY-03 | Snap and "erhält" | E9 | 4 | M | P/g | 6.7 |
| JY-04 | Success sweep | E5 | 1 | S | P | 5.3 |
| JY-05 | Run theatre | E5 | 2 | L | P/g | 5.7 |
| JY-06 | Hourglass waits | E5 | 2 | S | P/g | 5.7 |
| JY-07 | Behoben moment | E6 | 2 | M | P/g | 5.0 |
| JY-08 | Outcome cards | E5 (late, redesigned; reads the receipt excerpt) | 7b | L | N/r | 5.7 |
| JY-09 | Good-morning card | E7 → UX-15 | 3 | M | N/g | 4.0 |
| JY-10 | Milestone celebrations | Cut | – | M | N | 2.7 |
| JY-11 | Impact counters | Cut | – | L | N/g | 3.0 |
| JY-12 | Practice playground | Cut | – | L | N/g | 1.7 |
| JY-13 | First-result quest | E8 → UX-03 | 1 | M | P | 2.3 |
| JY-14 | Theme run flavours | Cut | – | L | P/g | 2.0 |
| JY-15 | Empty-state art | Deferred | – | M | P | 3.7 |
| JY-16 | Tactile canvas | E3 (glide, with FD-02) | 4 | M | P | 5.0 |
| JY-17 | Peek previews | E5 (late) | 4 | M | P/g | 5.3 |
| JY-18 | Run replay | Deferred | – | M | P | 3.3 |
| JY-19 | Publish moment | E4 (pill flip only) | 3 | S | P | 4.7 |
| JY-20 | Calm library | E8 | 3 | M | N | 5.7 |
| OP-01 | Attention overview (J.4) | E7 (endpoint + strip) | 3 | L | P/g | 6.3 |
| OP-02 | Cards with run health | E7 | 3 | M | N | 7.7 |
| OP-03 | Day ribbon and trends | Deferred | – | L | N | 3.7 |
| OP-04 | Run search and labels | E7 | 3 | L | N/g | 5.7 |
| OP-05 | Live rollback | E4 → UX-14 | 3 | M | N/g | 5.7 |
| OP-06 | Notifications per flow | E7; decision 12 | 3 | M | P/g | 6.3 |
| OP-07 | Approvals announce themselves | E7; decision 11 | 3 | M | P/g | 7.3 |
| OP-08 | Flow status as data | E10 (trigger → PE-16); rest deferred | 5 | L | P/g | 4.3 |
| OP-09 | Dependency map | E9 (metadata); map deferred | 5 | L | N/g | 5.0 |
| OP-10 | Integration status | Deferred | – | M | P | 5.0 |
| OP-11 | Flows in Spotlight | E7 | 6a | M | N/g | 5.7 |
| OP-12 | Status widget | Deferred | – | M | N | 4.0 |
| OP-13 | Deep-link contract | E7 | 2 | S | P | 6.7 |
| OP-14 | Packs 2.0 | Deferred (redesign) | – | L | P/r | 3.7 |
| OP-15 | Migration importer | Deferred (redesign) | – | XL | N/r | 3.3 |
| OP-16 | Check against past runs | Deferred | – | L | P/g | 5.0 |
| OP-17 | Agent manual, real switches | E1 | 0 | S | P/g | 5.0 |
| OP-18 | Desktop notice step | E13 | 6a | S | N/g | 4.3 |
| OP-19 | Automate with EasyDrag | Deferred → PI-15 | – | L | N/g | 4.3 |
| OP-20 | Trash, archive, backup | Deferred (archive cut) | – | L | N/g | 5.0 |
| FD-01 | Hot-path diet | E3 | 1 | M | N | 6.0 |
| FD-02 | Uncapped auto-layout | E3 | 4 | M | N | 5.7 |
| FD-03 | Performance gate | E1 (S0 step 0) | 0 | M | P | 5.3 |
| FD-04 | Semantic zoom and culling | E3 | 2 | L | P/g | 6.7 |
| FD-05 | Paint diet | E3 | 1 | S | N | 5.3 |
| FD-06 | Keyboard and screen-reader graph | E3 (late) | 4 | L | P/g | 5.3 |
| FD-07 | WCAG 2.2 AA closure | E2 (NVDA smoke in Wave 3) | 3/4 | M | N | 5.3 |
| FD-08 | Design-token layer | E2 (with the status map) | 1 | M | P | 7.3 |
| FD-09 | Component kit | E2 | 1 | M | P | 6.7 |
| FD-10 | Module splits | E1 (S0; split-before-use list) | 0/2/4/5 | M | N | 7.3 |
| FD-11 | i18n engine | E2 | 1 | M | N | 6.3 |
| FD-12 | Defect sweep | E1 (S0 subset; rest in S0c) | 0/S0c | S | N/g | 6.7 |
| FD-13 | Credential hygiene | E1 (S0b, redesigned) | 0 | M | N/r | 6.7 |
| FD-14 | Conformance lab | E2 (first scenes, then each wave) | 1–3 | L | N | 5.7 |
| FD-15 | Test tiers | E1 (S0c; touch audit in Wave 1) | S0c | M | U | 4.3 |
| FD-16 | Living documentation | E1 (S0c; agent-manual switches in S0) | S0c | M | N/g | 4.7 |
| FD-17 | Trust-debt sweep | E1 (S0) | 0 | S | P/g | 7.7 |
| FD-18 | Browser egress closure | E1 (S0b) | 0 | M | U/g | 6.3 |
| FD-19 | Error taxonomy | E6 | 2 | M | N | 7.7 |
| FD-20 | Engine defect tail | E1 (S0c); decision 13 | S0c | M | U/g | 5.0 |
| CS-01 | One noun "vN" | E2 | 1 | S | P | 6.3 |
| CS-02 | One import button | E8 → UX-02 (drop target in Wave 3) | 1/3 | S | P | 5.3 |
| CS-03 | Hide what cannot be used | E7; decision 11 (a) | 3 | S | P/g | 5.3 |
| CS-04 | Simulation first | E5 → UX-07 | 1 | M | P/g | 3.3 |
| CS-05 | Canvas first, automatic name | E8 → UX-01; automatic name cut | 1 | S | N | 4.3 |
| CS-06 | One simulation frame | E5 → UX-19 | 1 | S | P/g | 6.0 |
| CS-07 | Calm header | E4 → UI-14 | 1 | L | P/g | 4.3 |
| CS-08 | Retire the rail | Decision 2 | dec. 2 | M | P | 5.3 |
| CS-09 | Content-first home | E8 (content first, then attention) | 1/3 | L | P/o | 6.0 |
| CS-10 | Blocks in the library | E8 (as filter) | 3 | M | P | 4.3 |
| CS-11 | One filter popover | E8 (saved filters kept) | 1 | M | P | 3.3 |
| CS-12 | Inspector summary line | E4 → UI-16 | 2 | L | P/g | 5.3 |
| CS-13 | Retry defaults for reads | E11 → PE-04 | 5 | M | P/g | 5.3 |
| CS-14 | Testfälle noun | E5 → UX-08 | 2 | L | P/g | 5.0 |
| CS-15 | Two-minute automation | E8 (acceptance journey) | 3 | L | P/g | 5.3 |
| CS-16 | Selection toolbar | E4 → UI-07 | 3 | M | P | 3.7 |
| CS-17 | Complexity budget | E2 (ledger, control budget) | 1 | M | P | 4.3 |
| CS-18 | One diff, three places | E4 | 3 | L | P/g | 7.0 |
| CS-19 | Only if new or changed | E11 → PE-02 (HMAC) | 5 | L | N/r | 5.7 |
| CS-20 | Say less | E2 → UX-18 | 1 | M | N/g | 4.7 |

## Appendix B. External references

| Product | Pattern used | URL |
|---|---|---|
| n8n | Debug in editor, pinned data | https://docs.n8n.io/build/understand-workflows/understand-executions/debug-executions |
| Windmill | Restart from a step | https://windmill.dev/docs/flows/test_flows |
| Activepieces | Retry from failure | https://www.activepieces.com/docs/install/architecture/durable-execution |
| Dify | Variable inspection while debugging | https://dify.ai/blog/dify-1-5-0-real-time-workflow-debugging-that-actually-works |
| Home Assistant | wait_for_trigger | https://www.home-assistant.io/building-blocks/wait_for_trigger/ |
| Windmill | Debouncing | https://www.windmill.dev/docs/core_concepts/job_debouncing |
| Temporal | Schedule overlap and catch-up | https://docs.temporal.io/schedule |
| n8n | Remove Duplicates | https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-base.removeduplicates/ |
| Zapier | Digest | https://help.zapier.com/hc/en-us/articles/8496309901453 |
| n8n | Form trigger (typed inputs) | https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-base.formtrigger |
| Home Assistant | Blueprint selectors (recipe instances) | https://www.home-assistant.io/docs/blueprint/selectors |
| Windmill | Approval steps | https://windmill.dev/docs/flows/flow_approval |
| Home Assistant | Sentence triggers (chat commands) | https://www.home-assistant.io/voice_control/custom_sentences/ |
| n8n | MCP flows as tools | https://docs.n8n.io/advanced-ai/accessing-n8n-mcp-server/ |
| Make | Maia, ask while building | https://help.make.com/maia-by-make-system-card |
| n8n | Assistant on errors | https://docs.n8n.io/manage-cloud/ai-assistant/ |
| Make | AI reasoning panel | https://www.make.com/en/blog/make-ai-agents-trust-through-transparency |
| n8n | Metric evaluations | https://docs.n8n.io/advanced-ai/evaluations/metric-based-evaluations/ |
| tldraw | Viewport culling | https://tldraw.dev/sdk-features/culling |
| ComfyUI | Level of detail by font size | https://docs.comfy.org/interface/settings/lite-graph |
| tldraw | Edge auto-scroll | https://tldraw.dev/sdk-features/edge-scrolling |
| Node-RED | Status line under nodes | https://nodered.org/docs/creating-nodes/status |
| Blender | Socket shapes | https://docs.blender.org/manual/en/5.0/interface/controls/nodes/parts.html |
| Make | Run-data bubbles | https://help.make.com/view-module-run-data |
| Linear | Contextual command menu | https://linear.app/changelog/2019-10-07-contextual-command-menu |
| Unreal Engine | Pin-drag menu header | https://www.unrealengine.com/en-US/blog/blueprint-editor-tips-tricks |
| Figma | Compare changes (one diff) | https://help.figma.com/hc/en-us/articles/15023193382935 |
| Figma | Smart selection | https://www.figma.com/blog/introducing-smart-selection/ |
| Arc | Just-in-time hints | https://onboardme.substack.com/p/how-arc-browser-introduces-ai-max-feature |
| Material Design | Sound with meaning | https://m2.material.io/design/sound/about-sound.html |
| Stripe | Sandbox frame | https://docs.stripe.com/sandboxes |
| Apple Shortcuts | List-first editing | https://support.apple.com/guide/shortcuts/apdc33e4f4da/ios |
| Make | Grid dependency map | https://help.make.com/how-to-use-make-grid |
| n8n | Insights, execution data | https://docs.n8n.io/administer/observe-and-log/track-usage-with-insights |
| Temporal | Timeline view | https://docs.temporal.io/web-ui |
| Power Automate | Repair tips | https://learn.microsoft.com/en-us/power-automate/fix-flow-failures |
| Vercel | Instant rollback | https://vercel.com/docs/instant-rollback |
| Tines | Change control | https://www.tines.com/docs/stories/change-control |
| W3C | Dragging movements, target size | https://www.w3.org/WAI/WCAG22/Understanding/dragging-movements.html |