# AuraGolf – Polish-Runde (Design)

Stand: 2026-11-11. Freigegeben vom Nutzer am 2026-11-11 („Ja, alles umsetzen“).

Ziel: Detailverbesserungen, optische Aufwertung, UI und UX der bestehenden App. Keine neuen 3D-Modelle, keine
Änderungen an Physik, Sound oder Bahnen. Grundlage ist P4 (`2026-10-19-p4-ui-ux-design.md`); wo dieses Dokument
nichts sagt, gilt P4 weiter.

## 1. Feste Regeln (aus P4 und den Tests)

- Jeder sichtbare Text ist mindestens 13 px groß. Bedienelemente in Panels sind mindestens 44 px hoch.
- Kein Element ragt waagrecht aus `.mg-app`; außerhalb von Panels auch nicht senkrecht (`layoutIssues()` in
  `ui/testdata/minigolf-fixture.js`).
- Die Startseite passt bei 1280 × 760 ohne Scrollen in das Panel. Heute ist das Panel 649 px hoch, Platz sind 663 px.
  Die Startseite darf nicht höher werden.
- Unter `data-reduced="true"` und `prefers-reduced-motion: reduce` läuft keine Animation und kein Übergang. Die zwei
  Sammelregeln am Ende der CSS-Datei bleiben die letzten Regeln. Auf der Scorecard zählt der Browser-Test laufende
  Animationen und erwartet 0.
- Alle `data-*`-Haken, `data-action`-Werte, `.mg-plaque-name`, `.mg-chip`, `.mg-chip-name` (mit `overflow:hidden` und
  `text-overflow:ellipsis`), `.mg-podium`, `.mg-step`, `.mg-winner`, `.mg-controls`, `.mg-panel`, `.mg-meter`,
  `.mg-canvas canvas` bleiben erhalten.
- Keine externen Schriften, Bilder oder Bibliotheken. Symbole sind Inline-SVG aus `ICONS` in `minigolf-views.js`.
- Farben und Flächen kommen aus den `--mg-*`-Variablen; die Oberfläche muss in hellen und dunklen Themes lesbar sein.
- Dauer von Animationen höchstens 400 ms (Ausnahme: der Puls der aktuellen Bahn, siehe 3.2).

## 2. Aufteilung der Dateien

| Bereich | Dateien |
|---|---|
| S (Stil und Menüs) | `ui/css/desktop-app-minigolf.css`, `ui/js/desktop/apps/minigolf-menus.js`, `ui/js/desktop/apps/minigolf-views.js` |
| G (Spiel, HUD, Szene, Auswertung) | `ui/js/desktop/apps/minigolf-game.js`, `minigolf-hud.js`, `minigolf-scene.js`, `minigolf-scorecard.js`, `ui/lang/desktop/*.json`, `scripts/test-minigolf-ui.mjs` |

Das Bündel `ui/js/vendor/minigolf/` baut die Hauptsitzung nach beiden Bereichen (`npm run build:minigolf`).

### 2.1 Schnittstellen zwischen S und G

| Schnittstelle | Anbieter | Nutzer |
|---|---|---|
| `icon(name)` kennt zusätzlich `flag`, `chevron`, `alert`, `info`, `replay` | S (`ICONS`) | G |
| `createScorecard({ showPanel, tr, esc, icon })` | G | S ruft so auf |
| `createHud(elements, { tr, esc, icon })` unverändert | G | S |
| `view.setCourses(courses)` – Liste aus dem Manifest; `progress()` liest daraus die Welt je Bahn | S | G ruft einmal nach dem Laden des Manifests |
| `view.pause(focus, info)` mit `info = { world, worldId, hole, name }` (fertige Texte, `worldId` für `data-world`) | S | G |
| `hud.banner(title, lines, color, seconds, grade)` – `grade` ist `''`, `ace`, `eagle`, `birdie`, `par`, `bogey` oder `over` und steht als `data-grade` am Banner | G | G |
| `root.dataset.ended` = `"true"`, solange die Runde beendet ist, sonst `"false"` | G (in `setPanel`) | S (CSS blendet `.mg-controls` aus) |
| CSS-Klassen aus Abschnitt 4 und 5 | S | G erzeugt das Markup |

## 3. Paket A – Optik-Grundlage (S)

### 3.1 Variablen und Flächen

Neue Variablen auf `.mg-app`:

- `--mg-glass`: `color-mix(in srgb, var(--mg-surface-strong) 80%, transparent)`
- `--mg-glass-blur`: `blur(8px) saturate(1.15)` (eine Stelle zum Abschalten)
- `--mg-hairline`: `color-mix(in srgb, var(--mg-text) 14%, transparent)`
- `--mg-elev-1`: `0 4px 14px #0b161233`, `--mg-elev-2`: `0 10px 28px #0b161247`
- `--mg-ease`: `cubic-bezier(.2,.7,.2,1)`

HUD-Flächen (`.mg-plaque`, `.mg-chip`, `.mg-map-frame`, `.mg-map-toggle`, `.mg-banner`, `.mg-toast`, `.mg-hint`,
`.mg-coach-bubble`, die Ladeanzeige): Hintergrund `--mg-glass`, `backdrop-filter: var(--mg-glass-blur)`, Rand 1 px
`--mg-hairline`, Schatten `--mg-elev-1` (Banner und Toasts `--mg-elev-2`). Die Akzentstreifen von Plakette und Banner
bleiben. Panels (`.mg-panel`) bleiben deckend.

### 3.2 Kopfleiste

- `.mg-top-button`: Hintergrund und Rand durchsichtig; bei Hover, Fokus und `:active` Fläche `--mg-control-hover`.
  Deaktiviert: Deckkraft 0,4.
- Fortschritt: Jedes Segment trägt `data-world` der Bahn (aus `view.setCourses`). Erledigte Segmente füllt
  `--mg-accent` der Welt, offene `--mg-hairline`. Höhe 5 px, Radius 3 px. Das aktuelle Segment pulsiert
  (`mg-pulse`, Deckkraft 1 → 0,55 → 1 in 1,6 s, endlos).

### 3.3 Knöpfe

- Alle Knöpfe: `transition` für Hintergrund, Rand, Schatten und `transform` (120 ms, `--mg-ease`).
  `:active:not(:disabled)`: `transform: translateY(1px)`.
- `.mg-primary`: zusätzlich Glanzkante und Tiefe:
  `box-shadow: inset 0 1px 0 #ffffff47, 0 2px 0 color-mix(in srgb, var(--mg-accent) 62%, #000), 0 8px 18px color-mix(in srgb, var(--mg-accent) 30%, transparent)`;
  gedrückt nur die Glanzkante. Gilt nicht für `button[aria-pressed=true]` (bleibt flach).
- Ein einzelner Knopf in `.mg-panel-actions` (`button:only-child`) dehnt sich nicht: `flex: 0 1 auto`,
  `min-width: 180px`, rechtsbündig (`margin-left: auto`).

### 3.4 Panels

- `showPanel()` gibt der Sektion die Klasse `mg-panel-in`, wenn das Overlay vorher verborgen war oder sich die
  Beschriftung (`label`) gegenüber dem letzten Aufruf geändert hat; sonst nicht (ein erneutes Rendern desselben
  Schritts, etwa beim Wechsel der Spielerzahl oder beim Stummschalten, blendet nicht neu ein). War das Overlay
  verborgen, bekommt es zusätzlich `mg-overlay-in`; sonst wird die Klasse entfernt.
- `mg-panel-in`: Deckkraft 0 → 1 und `translateY(10px) scale(.985)` → keine Verschiebung, 200 ms, `--mg-ease`.
  `mg-overlay-in`: Deckkraft 0 → 1, 160 ms.
- Panelrand oben mit feiner Glanzkante (`box-shadow: inset 0 1px 0 #ffffff14` zusätzlich zum Schatten).
- Scrollleisten im Panel schmal (`scrollbar-width: thin`, Farbe `--mg-border`).

### 3.5 Regler und Schalter

- `input[type=range]` in der App: eigenes Aussehen (`appearance: none`). Spur 8 px hoch, Radius 4 px. Griff 22 px,
  rund, `--mg-surface`-Füllung mit 2 px Rand in `--mg-accent` und `--mg-elev-1`. WebKit und Firefox
  (`::-webkit-slider-runnable-track`, `::-webkit-slider-thumb`, `::-moz-range-track`, `::-moz-range-thumb`).
- Füllstand über die Variable `--mg-value` (0 bis 1) am Eingabefeld. Die Spur zeichnet zwei Ebenen: oben die leere
  Spur (`--mg-hairline`) rechtsbündig mit Breite `calc(100% - (var(--mg-value) * (100% - 22px) + 11px))`, darunter die
  Füllung.
  - Stärkeregler (`[data-power]`): Füllung `linear-gradient(90deg, #7fe0a6, #ffd789 50%, #ff8f77)` (die Stopps aus
    `POWER_STOPS`).
  - Lautstärke: Füllung `--mg-accent`.
- `minigolf-views.js` setzt `--mg-value`: in `power()`, nach jedem `showPanel()` für alle Regler im Panel und über
  einen eigenen `input`-Listener am Wurzelelement. Wert = `(value - min) / (max - min)`.
- Lautstärkeregler zeigen ihren Wert: Kopfzeile `<span class="mg-field-head"><span>Musik</span><output>20 %</output></span>`;
  der Text kommt aus `Intl.NumberFormat(document.documentElement.lang, { style: 'percent' })` und folgt dem
  `input`-Ereignis. `data-music` und `data-effects` bleiben am Eingabefeld.
- `.mg-check`: Text links, Schalter rechts (`justify-content: space-between`). Das Kontrollkästchen wird ein Schalter
  44 × 26 px (`appearance: none`, Radius 13 px, Fläche `--mg-hairline`, Knopf 20 px über `::before`; angehakt Fläche
  `--mg-accent`, Knopf rechts). Fokusring bleibt sichtbar.

### 3.6 Fußleiste und Toasts

- `.mg-app[data-ended=true] .mg-controls { display: none }` (wie im Hauptmenü).
- Ist ein Panel offen (`.mg-app:not([data-panel=""])`), stehen die Toasts oben mittig in der Bühne
  (`top: 10px; bottom: auto`), damit sie keine Panel-Knöpfe verdecken.
- Toast-Markup (G): `<div class="mg-toast" data-toast data-kind>` enthält `icon(...)` und `<span>Text</span>`;
  S legt beides nebeneinander (`display: flex; gap: 10px; align-items: center`). Das Symbol trägt die Farbe der Art.

### 3.7 Karte in Weltfarben

Kartenfarben je Welt als Variablen an `.mg-map`:

| Welt | Bahn | Sand | Bande | Wasser |
|---|---|---|---|---|
| `pirate` | `#58c46f` | `#ecd9a0` | `#8a5a33` | `#4fb6d6` |
| `temple` | `#3f9a63` | `#d9c58a` | `#7d6a4a` | `#3f8fa8` |
| `moon` | `#4254b8` | `#7a7fa6` | `#c9cde6` | `#141833` |

Abschlag und Loch auf dem Mond: `--mg-map-tee: #ffffff`, `--mg-map-cup: #0d1020`. G setzt `root.dataset.world`
in `loadCourse()` vor `map.setCourse()`, damit die Karte die Farben der neuen Welt liest.

## 4. Paket B – Menüs (S)

### 4.1 Startseite

- Kopf: `<header class="mg-menu-head"><img class="mg-logo" src="…" width="56" height="56" alt=""><div><h1 class="mg-title">AuraGolf</h1><p class="mg-tagline">…</p></div></header>`.
  Die Bildadresse ist `host.dataset.golfIcon`; `createView` reicht sie als `logo` an `createMenus`. Titel 40 px
  (schmal 32 px). Unter dem Kopf ein 4 px hoher Markenstreifen in den drei Weltfarben (`.mg-brand-bar` mit drei `<i data-world>`).
- Karte „Alle 18 Bahnen“ (`.mg-round-full`): statt `.mg-round-stripes` eine Bildreihe
  `<span class="mg-round-collage" aria-hidden="true">` mit je Welt einem `.mg-preview.mg-preview-mini` (Platzhalter,
  Symbol, Bild wie auf den Weltkarten). Breit: Raster `auto minmax(0,1fr) auto`, Bildreihe links (drei Bilder je
  96 × 54 px, 4 px Abstand), daneben Titel und Detail, rechts Bestwert und ein `icon('chevron')`. Schmal (≤ 560 px):
  Bildreihe über die volle Breite als drei gleiche Spalten, Text darunter.
- Weltkarten: Bild zoomt bei Hover und Fokus auf 1,05 (300 ms). `.mg-round-best` mit Bestwert bekommt
  `icon('trophy')` und `data-has-best`; ohne Bestwert bleibt der gedämpfte Text ohne Symbol.
- Training: `icon('flag')` links, Titel und Detail, `icon('chevron')` rechts.
- Fortsetzen-Karte unverändert im Aufbau, Flächen wie die Karten.

### 4.2 Bahnraster

Kachel: Nummer, Name, darunter eine Zeile `.mg-hole-foot` mit `<span class="mg-hole-par">Par 3</span>` als Chip und
dem Bestwert `<span class="mg-hole-best" data-rank="good|plain" title="Bestwert 2">` mit `icon('star')` und der Zahl.
`good`, wenn der Bestwert Par oder besser ist (Stern gefüllt in `--mg-accent-2`), sonst `plain` (gedämpft). Ohne
Bestwert entfällt das Element. Gruppentitel bekommen das Weltsymbol (`WORLD_ICONS`).

### 4.3 Spieler-Setup

- `.mg-player-card > legend` liegt im Kasten (`float: left; width: 100%`, das folgende Element `clear: both`).
- Farbfelder als kleine Bälle: Glanzpunkt per `radial-gradient(circle at 34% 30%, #ffffffb3 0 10%, transparent 42%)`
  über der Farbe, innerer Schatten unten rechts.
- Startknopf: `icon('play')` vor dem Text.
- Enter im Namensfeld startet die Runde (G, siehe 5.6).

### 4.4 Pause

`pause(focus, info)`: unter der Überschrift ein Block `.mg-pause-status` mit `data-world="${info.worldId}"`:
Weltname (`.mg-eyebrow`), `info.hole` (fett, 18 px), `info.name`. Linker Akzentstreifen wie die Plakette. Fehlt
`info`, entfällt der Block. Knöpfe mit Symbol: Fortsetzen (`play`), Hilfe (`help`), Hauptmenü (`home`).

## 5. Paket C und D – Spielgefühl und Auswertung (G; Stil in S)

### 5.1 Wertung beim Einlochen

- `minigolf-scorecard.js` exportiert `holeGrade(strokes, par)`: `ace` bei genau einem Schlag, sonst `eagle` (≤ −2),
  `birdie` (−1), `par` (0), `bogey` (+1), `over`.
- Beim Ereignis `cup` zeigt das Spiel statt des Toasts `sunk` ein Banner (2,6 s, `grade` gesetzt):
  - Titel: `ace` → `tr('hole_in_one')`, `eagle` → `tr('eagle')`, `birdie` → `tr('birdie')`, `par` → `tr('par')`,
    `bogey` → `tr('bogey')`, `over` → `tr('sunk')`.
  - Zeilen: bei mehreren Spielern der Name; dann `Schläge: n` (`tr('strokes') + ': ' + n`), bei Abweichung von Par
    angehängt ` · −1` bzw. ` · +2` (`formatToPar`); ist danach ein anderer Spieler dran, zusätzlich „{name} ist dran“
    (ersetzt das eigene Zugbanner in diesem Fall).
- Neuer Text `minigolf.hole_in_one` in allen 16 Sprachen, direkt nach `minigolf.sunk`:
  cs, da, de, el, en, nl, no, pl, pt, sv „Hole-in-one!“; es „¡Hoyo en uno!“; fr „Trou en un !“ (Leerzeichen vor dem
  Ausrufezeichen wie in `minigolf.sunk` derselben Datei); hi „होल-इन-वन!“; it „Buca in uno!“; ja „ホールインワン！“;
  zh „一杆进洞！“.
- Stil (S): `.mg-banner[data-grade]` färbt Streifen und Titel: `ace` und `eagle` `#2f9e63`, `birdie` `#3f9e6b`,
  `par` bleibt Spielerfarbe, `bogey` `#c07a2e`, `over` `#d05a41`. Bei `ace` zusätzlich ein Stern (`icon('star')`)
  vor dem Titel; G setzt ihn in ein `<span class="mg-banner-icon">`.

### 5.2 Kurze Pause vor der Scorecard

Haben alle Spieler die Bahn beendet, öffnet die Scorecard (bzw. im Training der nächste Schritt) 900 ms später, damit
Ball, Konfetti und Banner zu sehen sind. Bei reduzierter Bewegung sofort wie bisher. Die Wartezeit läuft über die
Bildtaktung in `frame()` (pausiert mit dem Fenster) und nur, solange kein Panel offen ist. Öffnet in der Zwischenzeit
ein Panel, verfällt der Wartevorgang; `closePanel()` führt dann wie heute zur Scorecard. `loadCourse()` löscht ihn.

### 5.3 Spielerchips

- Am Zug: Ring 2 px in `--mg-focus` plus Leuchten `0 0 0 5px color-mix(in srgb, var(--mg-focus) 24%, transparent)`
  (statt 3 px Umriss). Der Zusatz „am Zug“ bleibt.
- Ändert sich die Schlagzahl eines Spielers gegenüber dem letzten `update()` derselben Bahn, bekommt
  `.mg-chip-score` die Klasse `mg-bump` (Skalierung 1 → 1,25 → 1 in 280 ms).

### 5.4 Namensschilder der Mitspieler (Szene)

- Die Schilder behalten ihre Weltposition, ihre Höhe am Bildschirm liegt aber zwischen 20 und 30 px: je Bild
  `pxProMeter = Bühnenhöhe / (2 · tan(fov/2) · Abstand zur Kamera)`, Welt-Höhe = `clamp(0,34 m, 20/pxProMeter, 30/pxProMeter)`.
- Ein Schild ist unsichtbar, solange sein Ball waagrecht weniger als 0,55 m vom Ball am Zug entfernt liegt.

### 5.5 Mauszeiger (Szene)

Über dem Ball (Maus oder Stift, innerhalb von 46 px, Zielen möglich, kein Kameramodus): `grab`. Während des Ziehens:
`grabbing`. Sonst der Standardzeiger. `canvas.style.cursor` wird nur bei einer Änderung geschrieben.

### 5.6 Kleinigkeiten im Spiel

- Enter in `[data-player-name]` im Schritt `players` löst `start` aus.
- Kein Toast `new_best` mehr, wenn gleichzeitig ein Ergebnis-Panel mit Abzeichen erscheint (Rundenende, Training);
  der Text geht weiter an die Live-Region (`hud.announce`).
- `view.setCourses(manifest.courses)` nach dem Laden des Manifests; `view.pause(focus, info)` an beiden Aufrufstellen.

### 5.7 Scorecard

- Zweite Kopfzeile `<tr class="mg-par-row">`: „Par“, Par je Bahn, Summe, leere Zelle.
- Legende unter der Tabelle: `<ul class="mg-legend" aria-hidden="true">` mit `eagle`, `birdie`, `bogey`, `over`
  (je `<li class="mg-grade-…">Text</li>`); die Zellen tragen den Text weiter im `title`.
- Führung: Bei mindestens zwei Spielern und wenn jeder mindestens eine beendete Bahn hat, tragen die Zeilen mit dem
  niedrigsten Stand zu Par `data-leader`; führen alle gemeinsam, keine. Die Namenszelle zeigt dann `icon('trophy')`
  in `.mg-leader`.
- „±Par“ zeigt „–“, solange ein Spieler noch keine Bahn beendet hat (auch die laufende nicht).
- Ein Spieler, Bahn gerade beendet: unter der Überschrift `<p class="mg-grade-hero" data-grade="…">` mit dem Titel
  aus 5.1 und `Schläge: n`.
- Stil (S): `.mg-par-row` gedämpft, 13 px; `.mg-legend` als Zeile kleiner Chips (umbrechend); `.mg-leader` in
  `--mg-accent-2`; `.mg-grade-hero` als großer Chip (20 px, fett) in den Farben aus 5.1.

### 5.8 Rundenende

- Ergebniskarte (ein Spieler): unter dem Bestwert `<ul class="mg-grade-summary">` mit je vorkommender Wertung
  `<li class="mg-grade-…">2× Birdie</li>` (Reihenfolge `ace`, `eagle`, `birdie`, `par`, `bogey`, `over`; `ace` nutzt
  `tr('hole_in_one')`, `over` nutzt `tr('over')`).
- Treppchen: Der erste Platz bekommt über der Ballscheibe `<span class="mg-step-crown">` mit `icon('trophy')`
  (`--mg-accent-2`, 24 px). Bei Gleichstand bekommen ihn alle ersten Plätze.
- Knöpfe am Rundenende mit Symbol: Nochmal (`replay`), Hauptmenü (`home`), Andere Bahn (`flag`), Nächste Bahn (`chevron`, hinter dem Text).

## 6. Tests und Abnahme

- `scripts/test-minigolf-ui.mjs`: `holeGrade` (alle sechs Stufen, Ass vor Eagle), ±Par-Strich über eine exportierte
  reine Funktion `finishedHoles(player, round)`.
- `npm run test:minigolf`, `go test ./ui/ -run 'TestDesktopMinigolf'`, Browser-Test
  `TestDesktopMinigolfBrowser` (mit `AURAGO_RUN_BROWSER_SMOKE=1`), dazu Vorher/Nachher-Screenshots in 1280 und 420 px.
- Der Browser-Test bekommt Zusicherungen für: Par-Zeile vorhanden, Fußleiste am Rundenende ausgeblendet, Regler
  tragen `--mg-value`.
