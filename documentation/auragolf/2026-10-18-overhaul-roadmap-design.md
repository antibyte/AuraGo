# AuraGolf-Überarbeitung – Fahrplan (Design)

- **Datum:** 2026-10-18
- **Status:** freigegeben (Fahrplan)
- **Folgedokumente:** je Teilprojekt eine eigene Spec + Plan in `documentation/auragolf/`

Dieses Dokument ist der freigegebene Fahrplan für die Überarbeitung von AuraGolf, dem 3D-Minigolf der AuraGo-Desktopoberfläche. Es hält Ausgangslage, Ziele, Entscheidungen, Teilprojekte, übergreifende Regeln und Risiken fest. Technische Festlegungen erfolgen in den Teil-Specs.

## 1. Ausgangslage (gemessen, 2026-10-18)

### 1.1 Betroffener Bestand

- `ui/js/desktop/apps/minigolf-*.js` (Physik, Szene, Spiel, Ansichten, Audio)
- `assets/minigolf/generate.py` (Blender-5.2.2-Generator)
- `ui/3d/minigolf/` (GLB-Dateien und `manifest.json`)
- `ui/css/desktop-app-minigolf.css`
- `scripts/test-minigolf-*.mjs`, `ui/desktop_minigolf*_test.go`
- `documentation/minigolf.md` (Nutzerdokumentation)

### 1.2 Schlag und Physik

`minigolf-physics.js`:

- `shoot()`: Abschuss mit 12 m/s × Stärke, linear. Der Ball startet ohne Drall (`angvel` 0). Der Gleit-Roll-Übergang kostet sofort rund 30 %: gemessen 12 → ca. 8,5 m/s nach 0,1 s auf der geraden Bahn, ca. 6,9 m/s auf gewundenen Bahnen.
- Ball: `linearDamping` 0,9/s, `angularDamping` 0,18 (wirkt wie eine Luftbremse), Restitution 0,12.
- Banden (Trimesh wood/stone): Restitution 0,02 (Mesh) bzw. 0,03 (Box). Rapier kombiniert per Mittelwert mit dem Ball, daher vernichtet Bandenkontakt fast die gesamte Geschwindigkeit.
- Trimesh-Kollider ohne `TriMeshFlags.FIX_INTERNAL_EDGES`: Hopser und Verluste an Dreiecksnähten.
- Messung „volle Stärke direkt Richtung Loch“ über alle 18 Bahnen: gerade Bahn 8,2 m. Auf 9 von 18 Bahnen bleibt der Ball nach 4,0–5,4 m liegen (≈ halbe Strecke). Halbe Stärke: 2,2–4,9 m.
- Experiment: Mit Drall beim Abschuss und `FIX_INTERNAL_EDGES` rollt der volle Schlag ca. 10,6 m (gerade Bahn).
- Steuerung (`minigolf-scene.js`, `move()`): Stärke = `min(1, Zugweite / 3 m Weltkoordinaten)`. Ziellinie und Pfeil: max. 0,5 + 2 × Stärke = 2,5 m. Der Zug muss innerhalb von 46 px vom Ball starten.

### 1.3 Bahnen

- Alle 18 Bahnen sind dieselbe 12,3 × 12,3 m große Insel mit einer 1,72 m breiten, 6 Reihen breiten Spur.
- Abschlag → Loch: 8,15–8,45 m.
- Nur 3 bewegliche Hindernisse: garden-06, coast-11, night-16.
- Identischer Wasserring.
- Welten: `garden`, `coast`, `night`.

### 1.4 Look

- Flache Materialfarben ohne Texturen.
- Requisiten aus Grundkörpern: Ikosaeder-Felsen und -Baumkronen, 6–8-seitige Zylinder.
- Keine Umgebungsbeleuchtung; nur Hemisphere-Licht, Sonne und leichter Bloom.

### 1.5 Oberfläche

`minigolf-views.js`, `desktop-app-minigolf.css`:

- Templates per innerHTML.
- Keine Stärkeanzeige außer Prozentwert und Ringgröße.
- Keine Laufbahnvorschau.
- Hilfe-Button unter 560 px Containerbreite ausgeblendet und sonst nirgends erreichbar.
- Bahnname ab 800 px ausgeblendet.
- Schrift 11–12 px.
- Theme-Anbindung nur für die Desktop-Themes `standard` und `fruity`.
- Automatische Qualität schaltet nach Absinken nie wieder hoch.

### 1.6 Bekannte Kleinfehler

- `minigolf-game.js:58` prüft `player.holed` / `player.finished`. Das Modell setzt diese Felder nie; der Schutz ist tot.
- Namensfeld `maxlength` 40; abgeschnitten wird still auf 34.
- Spielerfarbe Spieler 1: HUD `#fff0c5`, 3D `0xfff6df`.

### 1.7 Sound

`minigolf-audio.js`:

- Alle Effekte sind General-MIDI-Synthtöne der Synth-Studio-Engine.
- Rollgeräusch: ein Ton, der alle 560 ms neu angeschlagen wird.
- Kaum Rausch- oder Transientenanteile.
- Musik: 3 CC0-MIDI-Stücke, live per Synth Studio.

### 1.8 Grenzen heute

| Grenze | Ist-Stand | Quelle |
|---|---|---|
| Spiel-Bundle `ui/js/vendor/minigolf/game.esm.js` | 5.063.316 B bei Limit 5 MiB (ca. 175 KB frei) | `scripts/build-minigolf.mjs:12` |
| Laufzeit-Assets | ca. 8,9 MiB bei Testlimit 12 MiB | `scripts/test-minigolf-assets.mjs` |
| Manifest | ca. 3 MB JSON wegen Kollisionsarrays | `ui/3d/minigolf/manifest.json` |

Die Tests pinnen derzeit:

- 18 Bahnen (6/6/6) und Weltnamen,
- aufgezeichnete Routen (`scripts/test-minigolf-courses.mjs`),
- Physikziele („voller Schlag > 10 m in < 10 s“, Sand < 0,65 × Gras),
- Musik-Tempi.

## 2. Ziel und Nicht-Ziele

### 2.1 Ziel

Arcade-Minigolf im Look eines stilisierten Dioramas, mit:

- drei neuen Welten und 18 neu gebauten Bahnen,
- kräftigem Schlag mit Laufbahnvorschau,
- überarbeiteter Oberfläche,
- deutlich besserem Sound.

Alles läuft weiterhin lokal. Zur Laufzeit wird nichts von außen geladen.

### 2.2 Nicht-Ziele

- Online-Mehrspieler. Bleibt lokales Hot-Seat für 1–4 Spieler.
- Bahneditor für Nutzer.
- Migration alter Spielstände.
- Änderungen an Synth Studio.
- Neue Server-Endpunkte.

## 3. Getroffene Entscheidungen

| Thema | Entscheidung | Folge |
|---|---|---|
| Spielgefühl | Arcade-Minigolf: kräftiger Schlag, langer gleichmäßiger Auslauf, federnde Banden, Rampen/Sprünge | Volle Stärke ≈ 28 m auf ebener Bahn |
| Look | Stilisiertes Diorama: weiche, abgerundete Formen statt kantigem Low-Poly, Farbverläufe, gebackenes AO, viele Details | Kleine Dateien, auch für schwächere GPUs |
| Welten | Drei neue Welten ersetzen `garden`/`coast`/`night`: Piratenbucht (ID `pirate`, sonnig, Türkis/Sand), Dschungeltempel (ID `temple`, goldenes Abendlicht, Grün/Gold), Mondstation (ID `moon`, Nacht/Weltraum, Neon-Akzente). Je 6 Bahnen, insgesamt 18. | Umsetzung in P2 (Requisiten) und P3 (Bahnen) |
| Budget | Laufzeit-Assets ≤ 32 MiB, Spiel-Bundle ≤ 6 MiB | Testgrenzen, siehe Abschnitt 5 |
| Soundeffekte | Hybrid: CC0-Aufnahmen für körperliche Geräusche; UI-Klänge und Ambiente lokal synthetisiert | P5 |
| Musik | 4 loopbare CC0-Audiotracks (OGG, je ca. 1–1,5 MB): Menü, Piraten, Tempel, Mond | P5 |
| Steuerung | Zurückziehen-und-Loslassen bleibt und wird verbessert | P1 |
| Asset-Pipeline | Variante A: prozeduraler Baukasten in Blender | Verworfene Alternativen siehe unten |
| Vorgehen | Fahrplan plus fünf Teilprojekte mit je eigener Spec und eigenem Plan | Folgedokumente in `documentation/auragolf/` |

Verworfene Alternativen der Asset-Pipeline:

- **B** – handmodellierte `.blend` als Quelle. Verworfen: nicht reproduzierbar, Kollision driftet.
- **C** – Aufbau zur Laufzeit. Verworfen: kein gebackenes Licht, zu viel Bundle-Code.

## 4. Teilprojekte

### 4.1 Abhängigkeiten und Reihenfolge

| Teilprojekt | Abhängig von | Ausführung |
|---|---|---|
| P1 Schlag & Physik | – | Zuerst |
| P2 Look & Welt-Requisiten | P1 | Nach P1, vor P3 |
| P3 Bahnen-Baukasten & 18 Bahnen | P1, P2 | Nach P2, nacheinander mit P2 (gemeinsamer Generator) |
| P4 UI/UX | P1; finale Weltkarten-Bilder aus P2 | Parallel zu P2/P3 in eigener Session; bis P2 fertig Platzhalter |
| P5 Sound & Musik | – | Parallel in eigener Session |

```
P1 ──┬──► P2 ──► P3
     └──► P4 (parallel zu P2/P3)
P5 (unabhängig, parallel)
```

### 4.2 P1 Schlag & Physik

#### Inhalt

- Ball startet rollend; Drall passend zur Abschussgeschwindigkeit.
- Lineare Dämpfung entfällt weitgehend. Stattdessen Rollwiderstand je Untergrund (Filz/Gras, Sand, Holz, Metall) als annähernd konstante Verzögerung. Der Ball läuft so gleichmäßig aus und stoppt sauber. Hänge wirken weiter über die Schwerkraft.
- Banden federnd: Restitution ca. 0,7–0,8. Kombinationsregel so gewählt, dass der Bandenwert gilt.
- `FIX_INTERNAL_EDGES` für alle Trimesh-Kollider.
- Steuerung:
  - Stärke aus der Zugweite in Bildschirm-Pixeln, unabhängig vom Kameraabstand.
  - Weiche Kurve für Feingefühl bei kurzen Putts.
  - Zurückziehen bis zum Ball bricht den Schlag ab.
  - Tastatur (Pfeile, Shift, Space) und Stärke-Regler bleiben erhalten.
- Vorschau:
  - Gestrichelte Laufbahn bis zur ersten Bande (Shape-Cast der Ballkugel), danach kurzes Segment in Abprallrichtung.
  - Ohne Bandenkontakt: Markierung des voraussichtlichen Ausrollpunkts auf ebener Bahn.

#### Betroffene Bereiche

- `minigolf-physics.js`
- `minigolf-scene.js` (Eingabe, Vorschau)
- `minigolf-game.js`
- `scripts/test-minigolf-physics.mjs`
- `scripts/test-minigolf-courses.mjs` (Routen der alten Bahnen neu aufzeichnen)
- `documentation/minigolf.md`

#### Abnahmekriterien

- Voller Schlag auf ebener Filzbahn: 28 m ± 10 %.
- Ruhe ≤ 6 s nach vollem Schlag auf ebener Bahn.
- Voller Schlag: Geschwindigkeit nach 0,1 s ≥ 95 % der Abschussgeschwindigkeit (kein Gleit-Roll-Verlust mehr).
- 45°-Bandenschlag behält ≥ 65 % der Geschwindigkeit.
- Kein Hopser > 1 cm an Dreiecksnähten.
- Kürzester sinnvoller Putt ≤ 0,3 m.
- Alle 18 alten Bahnen mit neu aufgezeichneten Routen lösbar.
- Browser-Smoke-Test grün.

P1 wird direkt auf den alten Bahnen ausgeliefert (dort erreicht ein halber Zug das Loch).

### 4.3 P2 Look & Welt-Requisiten

#### Inhalt

- **Stil-Gate:** Zuerst drei Probe-Renders aus Blender, je Welt ein Bahnausschnitt mit Hingucker. Serienproduktion erst nach Freigabe durch den Nutzer.
- **Generator:** Der Blender-Generator wird modular, z. B. Module für Materialien/Paletten, Requisiten je Welt, Bake und Export, statt einer 783-Zeilen-Datei. Er bleibt deterministisch und per CLI (Blender 5.2.2) reproduzierbar. Blender-MCP wird nur für Vorschau und Feinschliff genutzt, nie als Quelle.
- **Modelle:** Requisiten mit Bevel, Subdivision und gewichteten Normalen.
- **Farben:** Paletten-Farbverläufe; Ambient Occlusion per Cycles, in Vertex-Farben gebacken.
- **Instancing:** Je Welt ein gemeinsames Requisiten-GLB, das einmal geladen und instanziert wiederverwendet wird.
- **Requisiten-Beispiele:**
  - Piraten: Schiff, Kanone, Schatztruhe, Fässer, Palmen, Steg, Totenkopffelsen.
  - Tempel: Stufentor, Steinkopf mit Tunnelmaul, Rollstein, Totems, Fackeln, Lianen, Wasserfall.
  - Mond: Stationsmodule, Parabolschüssel, Rover, Krater/Felsen, Neonbanden, Erde am Himmel.
- **Laufzeit (three.js):**
  - Weiches Umgebungslicht aus lokal erzeugter Umgebung (PMREM, kein Download).
  - Himmel je Welt (Wolken/Dunst/Sternenfeld+Planet).
  - Wasser- und Lagunen-Shader.
  - Partikel: Sand, Spritzer, Funken, Konfetti.
  - Leuchtspur des Balls.
  - Einloch-Feier.
  - Kamerafahrt entlang der Bahn.
  - Bloom je Welt.
  - Qualitätsstufen.

#### Betroffene Bereiche

- `assets/minigolf/generate.py` und neue Generator-Module in `assets/minigolf/`
- `ui/3d/minigolf/` (Requisiten-GLBs)
- `minigolf-scene.js` (Look, Kamerafahrt, Bloom, Qualitätsstufen)
- Laufzeitcode für Himmel, Wasser, Partikel und Licht (Modulzuschnitt im Teil-Plan)
- `scripts/test-minigolf-assets.mjs` (Budgets)
- `documentation/minigolf.md`, `assets/minigolf/README.md`

#### Abnahmekriterien

- Freigabe der Probe-Renders durch den Nutzer.
- Requisiten-GLBs und Shader innerhalb der Budgets (Festlegung in der P2-Spec).
- Gleiche oder bessere FPS je Qualitätsstufe gegenüber heute (Benchmark-Modus des Browser-Tests).
- Keine externen Ressourcen.

### 4.4 P3 Bahnen-Baukasten & 18 Bahnen

#### Inhalt

- **Bahnstücke** mit exakt passender Kollision: Gerade, Kurve, Steilkurve, Rampe/Sprung, Hügel/Senke, Trichter, Tunnel, Gabelung (zwei Routen), Brücke, Stufen. Auf dem Mond zusätzlich Looping und Halfpipe.
- **Welt-Mechaniken:**
  - Bumper, Boost-Felder, Teleporter.
  - Kanone als Abschussrampe (Piraten).
  - Bewegliche Hindernisse: Rollstein, Pendel, Tore. Die bestehende rotate-/slide-Kinematik wird erweitert.
  - Geringere Schwerkraft auf dem Mond.
  - Wasser/Abgrund als Gefahr mit Strafschlag wie heute.
- **Bahnregeln:**
  - Laufweg 10–35 m, Par 2–5.
  - Jede Bahn hat einen Hingucker und möglichst eine riskante Abkürzung.
  - Die Schwierigkeit steigt innerhalb jeder Welt. Die erste Bahn der ersten Welt dient als sanfte Einführung.
  - Jede Bahn ist ein eigenes, auf ihr Layout zugeschnittenes Diorama.
- **Datenformat:**
  - Bahnen sind Daten: Abfolge von Bahnstücken plus Requisiten-Platzierung.
  - Kollision wird weiterhin aus dem exportierten GLB zurückgelesen.
  - Kollisionsdaten binär statt ca. 3 MB JSON.
  - Manifest-Version 2 mit Welt-IDs `pirate`, `temple`, `moon`.

#### Betroffene Bereiche

- Generator-Module für Bahnstücke und Export (`assets/minigolf/`)
- `ui/3d/minigolf/` (GLBs, Manifest v2, binäre Kollisionsdaten)
- `minigolf-scene.js`, `minigolf-game.js` (Mechaniken, Welt-Logik)
- `scripts/test-minigolf-courses.mjs` (18 Bahnen, Routen), `scripts/test-minigolf-assets.mjs` (Budgets)
- Sprachdateien `ui/lang/desktop/*.json` (18 Bahnnamen)
- `documentation/minigolf.md`, `assets/minigolf/README.md`

#### Abnahmekriterien

- 18 Bahnen (6/6/6), jede per aufgezeichneter Route lösbar.
- Suchmodus `AURAGO_MINIGOLF_SEARCH=1` findet Routen neu.
- Asset-Test mit neuen Budgets grün.
- 18 neue Bahnnamen in allen 16 Sprachen.

### 4.5 P4 UI/UX

#### Inhalt

- **HUD:** Bahn, Par und Schläge je Spieler; große Stärkeanzeige am Ball; Banner „Spieler N ist dran“; Hinweise als kurze Einblendungen; Übersichtskarte mit echtem Bahnverlauf.
- **Menüs:**
  - Weltauswahl als Karten mit Vorschaubildern (aus Blender gerendert; bis P2 abgeschlossen ist, Platzhalter).
  - Bahnraster mit Bestwerten.
  - Spieler-Setup mit Farbwahl.
- **Auswertung:** Scorecard mit Animation; Siegertreppchen am Rundenende.
- **Einführung:** Interaktive Einführung auf der ersten Bahn statt Textseite.
- **Hilfe:** Immer erreichbar, auch in schmalen Fenstern.
- **Gestaltung:** Weltbezogene Farben und Akzente; folgt allen Desktop-Themes.
- **Schrift:** Mindestens 13–14 px. Eine eigene Schrift wird höchstens lokal mitgeliefert (OFL), nie aus dem Internet geladen.
- **Kleinfehler:** Behebt die Punkte aus Abschnitt 1.6 (toter Schutz in `minigolf-game.js:58`, Namenslänge 34 gegenüber 40, Spielerfarbe, Auto-Qualität schaltet nach Absinken wieder hoch).

#### Betroffene Bereiche

- `minigolf-views.js` (Templates für HUD, Menüs, Auswertung), `ui/css/desktop-app-minigolf.css`
- `minigolf-game.js`
- Sprachdateien `ui/lang/desktop/*.json`
- Bestehende Browser-Prüfungen in `scripts/test-minigolf-*.mjs` und `ui/desktop_minigolf*_test.go` (anpassen)
- `documentation/minigolf.md`

#### Abnahmekriterien

- Layout bei 1280×760 und 420 px ohne Überlauf (bestehende Browser-Prüfungen angepasst).
- Tastatur- und Touch-Bedienung.
- Reduzierte Bewegung wird respektiert.
- Alle neuen Texte in 16 Sprachen.

### 4.6 P5 Sound & Musik

#### Inhalt

- **Audio-Engine:** Eigene kleine Engine für AuraGolf auf Web Audio, mit Sample-Player (Varianten, zufällige Tonhöhe und Lautstärke). AuraGolf lädt Synth Studio nicht mehr. Synth Studio selbst bleibt unverändert.
- **Effekte:**
  - Rollgeräusch als durchgehende Schleife; Tempo, Lautstärke und Filter folgen Geschwindigkeit und Untergrund.
  - Aufprall je Material und Stärke.
  - Lochfall mit Randklappern.
  - Platschen, Kanone, Boost, Teleporter.
  - UI-Klänge synthetisiert.
- **Ambiente je Welt:** Wellen/Möwen, Dschungel-Insekten/Vögel, Stationssummen/Piepen.
- **Raum und Pegel:** Hall je Welt (lokal erzeugte Impulsantwort); Limiter.
- **Musik:** 4 CC0-OGG-Tracks mit Überblendung und Absenkung bei Ereignissen (Einlochen, Sieg).
- **Quellen und Nachweis:**
  - Ausschließlich CC0.
  - Je Datei Quelle, Autor, Lizenz und SHA-256 in Manifest und README, wie heute bei den MIDI-Dateien.
  - Vor jedem Download wird dem Nutzer die Dateiliste (Name, Quelle, Größe) zur Freigabe vorgelegt.

#### Betroffene Bereiche

- `minigolf-audio.js` (Engine; Synth-Studio-Abhängigkeit entfällt)
- `ui/js/desktop/core/module-loader.js:24` (lädt heute `gm-data`, `synth-studio-voices` und `synth-studio-audio` für AuraGolf mit; diese Einträge entfallen)
- `minigolf-game.js` (Ereignisanbindung)
- Audio-Assets und Manifest-Einträge (Ablage im Teil-Plan festzulegen)
- Musik-Tests (Tempi) in `scripts/test-minigolf-*.mjs`
- `documentation/minigolf.md`, Musik-README

#### Abnahmekriterien

- Effekte ca. 1–2 MB, Musik ca. 5 MB, innerhalb des 32-MiB-Budgets.
- Kein Aussetzer im 20-s-Browser-Test.
- Stummschalten und Lautstärken bleiben gespeichert.
- Keine externen Ressourcen.

## 5. Übergreifende Regeln

- **Spielstände, Bestwerte, Einstellungen:** Neue Schlüssel mit den Endungen `round.v2` und `best.v2`. Für Einstellungen gilt: bleiben v1-kompatibel oder werden v2; Entscheidung in der P4-Spec. Alter Fortschritt verfällt mit einmaligem Hinweis; keine Migration.
- **Budgets in den Tests:** Assets ≤ 32 MiB, Bundle ≤ 6 MiB. Das Dreiecksbudget je Bahn wird in P2/P3 neu festgelegt.
- **Keine externen Ressourcen:** Zur Laufzeit wird nichts von außen geladen. Der Guard-Test `TestShippedWebAssetsLoadNothingExternal` bleibt grün. Keine Klassen mit Präfix `cc-`.
- **i18n:** Alle neuen Texte in allen 16 Sprachdateien `ui/lang/desktop/*.json`.
- **Leistung:** Auf gleicher Qualitätsstufe nicht schlechter als heute.
- **Tests schlank:** Nur wesentliche Tests je Teilprojekt, plus Browser-Smoke-Test (`AURAGO_RUN_BROWSER_SMOKE=1`).
- **Lizenzen:** Eigene Modelle und Generator unter MIT wie bisher. Audio nur CC0, mit Nachweis.
- **Dokumentation:** `documentation/minigolf.md`, `assets/minigolf/README.md` und Musik-README werden je Teilprojekt aktualisiert.
- **Arbeitsweise:** Umsetzung in eigenem Worktree-Branch je Teilprojekt. Commit nach jeder Änderung. Landung auf `main` per Rebase und Fast-Forward. Andere Sessions arbeiten parallel im selben Checkout.
- **Konfliktzonen bei Parallelarbeit:**
  - `minigolf-game.js` (P1, P3, P4, P5)
  - `minigolf-scene.js` (P1, P2, P3)
  - Sprachdateien (P3, P4)
  - Änderungen dort klein halten und vor dem Landen rebasen.

## 6. Risiken und Gegenmaßnahmen

| Risiko | Gegenmaßnahme |
|---|---|
| Schnelle Bälle durch dünne Wände und Loopings | CCD bleibt aktiv; 120-Hz-Schritt (bei 14 m/s ca. 0,12 m pro Schritt, kleiner als der Ballradius von 0,16 m); Looping-Mindestgeschwindigkeit im Bahn-Design einplanen |
| Bundle-Spielraum knapp | Neue Laufzeitlogik schlank halten; Bundle-Limit 6 MiB; three.js-Addons gezielt importieren |
| Qualität verfügbarer CC0-Audios | Auswahl vor dem Download vorlegen; Synth-Fallback für einzelne Effekte |
| GPU-Last durch mehr Requisiten | Instancing; gebackenes AO statt Echtzeit-AO; Qualitätsstufen |
| Blender-Export unter Windows bricht gelegentlich wegen Dateisperre ab (bekannt) | Export wiederholen; Determinismus per Hash prüfen |
| Routensuche nach Geometrieänderungen dauert | Suchmodus gezielt je Bahn ausführen; Routen versionieren |

## 7. In den Teil-Specs festzulegen

| Festlegung | Teil-Spec |
|---|---|
| Exakte Physikkonstanten | P1 |
| Liste und Namen der 18 Bahnen | P3 |
| Dreiecks- und Texturbudgets je Bahn/Welt | P2/P3 |
| Konkrete CC0-Quellen und Dateien | P5 |
| Schriftwahl | P4 |
| Settings-Schlüsselversion | P4 |
