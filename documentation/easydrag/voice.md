# EasyDrag voice guide

How EasyDrag talks to its user: the rules, the word ledger and the lints that keep them. It applies to
every text under `ui/lang/easydrag/` (16 locales) and to the texts the code builds. English and German
are written first; the other 14 locales are translated from the English text through key files and
`locale-apply.mjs`, and a native speaker should review them.

Decided on 2026-11-14 and 2026-11-15 (EasyDrag W1b, spec `plans/2026-11-15-easydrag-w1b-spec.md`).

## Rules

### 1. Outcome first

A text says first what it means for the user, then what to do. The mechanism and log hints come last.

- en: "The run was stopped. Start it again; details are in the AuraGo log."
- de: "Der Lauf ist abgebrochen. Starte ihn noch einmal; Details stehen im AuraGo-Log."
- not: "The flow execution failed unexpectedly. Details are in the AuraGo log."

### 2. A next step

Every error ends with what to do, or says that nothing needs doing.

- en: "The draft changed on the server. Reload it, then apply your edit again."
- de: "Der Entwurf hat sich auf dem Server geändert. Lade ihn neu und wende deine Änderung dann noch einmal an."
- en, nothing to do: "The flow was already paused. Nothing changed."
- de, nothing to do: "Der Flow war schon pausiert. Es hat sich nichts geändert."

### 3. One form of address

German uses the informal "du": lowercase "du", "dich", "dir", "dein", "deine", "deinem", "deinen",
"deiner", "deines". A capital only at the start of a sentence (after a full stop, "!", "?", ":" or a
line break) or at the start of the text. English addresses the user as "you".

- de: "Prüfe die Auswahl, bevor du sie anwendest."
- de, sentence start: "Du kannst den Schritt jetzt testen."
- not: "Prüfe die Auswahl, bevor Du sie anwendest."

Lint: `TestEasyDragGermanUsesLowercaseDu` (`ui/desktop_easydrag_voice_test.go`).

### 4. One word per concept

The ledger below fixes the word for each concept, in every locale. A second word for the same concept is
a bug, and so is one word for two concepts. The lint `TestEasyDragLocalesUseLedgerWords` bans the
words that the ledger replaced.

### 5. Limits

Always-visible text names no limit. A counter ("1,812 / 2,000") appears from 90 % of the limit. A rule
hint ("from 1 second to 24 hours") is not shown in advance; it becomes the error message when an input
breaks the rule. An input that is capped without a rule says "At most 120 characters." (de: "Höchstens
120 Zeichen.") when the cap is reached. Help that the user opens may name limits. The helper is
`ED.kit.limit` (`ui/js/desktop/apps/easydrag-kit.js`).

- en, always visible: "Name"; shown at the cap: "At most 120 characters."
- de, always visible: "Name"; shown at the cap: "Höchstens 120 Zeichen."
- not, always visible: "Names have up to 120 characters."

A count that describes a value ("1,204 characters" of a preview) is not a limit and stays.

### 6. The user's own actions

No visual notification when the screen already shows the result: publishing (the header shows the new
version), switching a flow on or off (the switch shows it), restoring a version (the draft shows it).
Where the result is invisible (the clipboard), a short confirmation appears at the control itself
("Copied" on the button for two seconds, without motion). Screen-reader announcements stay and replace
every removed notification. Failures, partial results and skips keep their notification.

- en: publishing sends no toast; a screen reader hears "“Daily digest” is published."
- de: "„Tagesrückblick“ ist veröffentlicht."

### 7. Only what the code guarantees

A text says "nothing has changed" or "nothing was sent" only when the error code means exactly that.
Read the code behind an error before you write its text. When the outcome is not known, say that it
is not known and what to check.

- en: "The result is not confirmed. Check the flow list before you try again."
- de: "Das Ergebnis ist nicht bestätigt. Prüfe die Flow-Liste, bevor du es noch einmal versuchst."

## Concept ledger

| Concept | de | en | Meaning | Where it appears |
|---|---|---|---|---|
| Flow | Flow | flow | one automation | home cards, header, dialogs |
| Step | Schritt | step | one card on the canvas, and the palette of step types | canvas, palette (`palette_title`), step dialog, search |
| Building block | Baustein | block | a reusable published sub-flow | blocks list (`blocks_title`), block editor, "Call block" step |
| Trigger | Auslöser | trigger | the step a flow starts from | cards, palette, issue texts |
| Draft | Entwurf | draft | the editable state | header, save state, versions |
| Published state | vN | vN | one published state (v1, v2, ...) | header, versions dialog |
| Run | Lauf | run | one execution | run drawer, footer, home cards |
| Simulation | Simulation | simulation | a test run with sample outputs and no effects | test dialog |
| Template | Vorlage | template | a flow to start from | home template cards |
| Pack | Pack | pack | a local export or import bundle | pack import and export |

Words that the ledger replaced and that the lint bans (whole words, placeholders removed first):

- de: Workflow, Knoten, Node, Block, Blöcke, Blocks, Revision, Paket (and their plurals), and compounds
  such as "Blockversion" (write "Bausteinversion").
- en: workflow, node, building block, revision, package.

"Baustein" and "block" are not banned: they are the ledger words for the reusable sub-flow. They must not
mean a step, so the lint also checks a list of step texts (the palette, the search, the unknown-step
messages) for them.

### Open in the ledger

- "Vorlage" (template) next to "Rezept" (recipe): the Wave 3 recipe cards decide which word stays.
- "Simulation" next to a real test run: W1d settles the two words when it rewrites the test loop.
- "Testbank" (test bench) is not yet a word of the user interface.

## Keeping the rules

- Locale files change only through key files for `locale-apply.mjs`; all 16 locales for every key.
- Placeholders (`{name}`) are the same in every locale.
- New texts are written against these rules. Run `go test ./ui -run '(?i)easydrag(GermanUsesLowercaseDu|LocalesUseLedgerWords)'`
  after a change of `de.json` or `en.json`.
