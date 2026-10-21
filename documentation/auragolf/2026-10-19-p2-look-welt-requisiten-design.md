# AuraGolf P2 – Look & Welt-Requisiten (Design)

- **Datum:** 2026-10-19
- **Status:** umgesetzt (2026-10-21); Stil am 2026-10-19 vom Nutzer freigegeben
- **Bezug:** [Fahrplan](2026-10-18-overhaul-roadmap-design.md), Abschnitte 3, 4.3, 5, 6 und 7
- **Umsetzungsplan:** [P2-Plan](2026-10-19-p2-look-welt-requisiten-plan.md)
- **Stilproben:** [Piratenbucht](img/p2-stilprobe-pirate.webp), [Dschungeltempel](img/p2-stilprobe-temple.webp), [Mondstation](img/p2-stilprobe-moon.webp)

P2 gibt AuraGolf den Look eines stilisierten Dioramas: weich gefaste Formen, Farbverläufe aus Weltpaletten, in Vertexfarben gebackenes Ambient Occlusion, Requisiten je Welt und neue Laufzeiteffekte. P2 ändert weder Physik noch Spielregeln, Oberfläche oder Sound. Die bestehenden 18 Bahnen werden in die drei neuen Welten umgekleidet; P3 ersetzt sie später durch neue Bahnen mit demselben Generator.

## 1. Ergebnis für Spielende

- Jede Bahn liegt auf einer Diorama-Insel mit runder Oberkante und sichtbaren Erdschichten an der Schnittkante.
- Die 18 Bahnen gehören zu drei neuen Welten:
  - **Piratenbucht:** Küstenbahnen; Sand, türkisfarbene Lagune, Schiff, Palmen, Fässer, Kanone, Schatztruhe, Steg.
  - **Dschungeltempel:** Gartenbahnen; Steinkopf, Totems, Fackeln, Lianen, Farne, Wasserfall, goldenes Abendlicht.
  - **Mondstation:** Nachtbahnen; die Insel schwebt im All, dazu Habitat, Parabolschüssel, Rover, Krater, Neonbanden und die Erde am Himmel.
- Spur, Banden, Hindernisse, Abschlag und Loch liegen exakt wie bisher. Jede bekannte Linie spielt sich gleich.
- Weiches Umgebungslicht, Himmel je Welt, animiertes Wasser mit Uferschaum, Bloom je Welt.
- Kleine Bewegungen in der Szene: Palmwedel und Farne wiegen sich, Fackeln flackern, die Parabolschüssel dreht sich.
- Effekte:
  - Partikel: Sand, Spritzer, Holzsplitter, Funken, Staub.
  - Leuchtspur des Balls in Spielerfarbe.
  - Einloch-Feier mit Konfetti, goldenem Ring und wehender Fahne.
  - Siegesfeuerwerk am Rundenende.
- Beim Laden einer Bahn fliegt die Kamera die Bahn entlang. Jeder Klick, jede Taste und das Mausrad überspringen den Flug.
- Die Weltkarten im Menü zeigen gerenderte Vorschaubilder statt Platzhaltern.
- Die drei Weltnamen heißen in allen 16 Sprachen nach den neuen Welten.
- Reduzierte Bewegung schaltet aus:
  - Kamerafahrt,
  - Wiegen,
  - Flackern,
  - Partikel.
- Gleiche oder bessere Bildrate als vorher auf jeder Qualitätsstufe.

## 2. Entscheidungen

| Thema | Entscheidung | Begründung |
|---|---|---|
| Stil-Gate | Die drei Stilproben vom 2026-10-19 sind freigegeben und gelten als Farb- und Formziel, nicht als Pixelziel. Die erkannten Schwächen (kantiger Tempelkopf, Wasserfall als Stufen, Orangestich im Tempel, flacher Mondboden und Filz, flache Kristalle) werden in der Serie behoben. | Freigabe des Nutzers: „probebilder sind freigegeben“. |
| Umfang | Die 18 bestehenden Bahnen werden umgekleidet. Spur- und Bandennetze (`surface-*`, `rail-*`), Hindernisse, Abschlag, Loch und Kollision bleiben bitgleich. Alles Übrige wird neu erzeugt: Insel, Deko, Sonderteile, Materialien und Platzierungen. | P2 wird sichtbar und messbar (Benchmark auf echten Bahnen). Die P1-Routen bleiben gültig. P3 übernimmt eine erprobte Pipeline. |
| Weltzuordnung | `coast` → `pirate`, `garden` → `temple`, `night` → `moon`. Die Welt-IDs im Manifest bleiben bis P3 unverändert; neues Bahnfeld `look`. | Deckt sich mit `WORLD_ALIAS` aus P5. P4-Speicherstände, Bestwerte und Rundenmodi bleiben gültig. |
| Weltnamen | Die Werte von `minigolf.garden`, `minigolf.coast` und `minigolf.night` werden in allen 16 Sprachen auf Dschungeltempel, Piratenbucht und Mondstation geändert. Die 18 Bahnnamen bleiben bis P3. | Name passt zum sichtbaren Look. P3 führt danach eigene Schlüssel für die neuen IDs ein. |
| Mond-Gefahr | Die Insel schwebt im All. Unter ihr liegt der Sternenhimmel; das Wasservolumen der Physik bleibt unverändert und wirkt als Absturz. | Stärkstes Bild der Mondwelt (Fahrplan P3: „Wasser/Abgrund“). Der Hinweistext für Wasser nennt kein Wasser. |
| Tonemapping | `THREE.NeutralToneMapping`, in Blender passend „Khronos PBR Neutral“. Belichtung je Welt. | Hält die Palettenfarben treu; die Proben wurden so gerendert. |
| Instancing | Je Welt ein Requisiten-GLB `props-<look>.glb`. Platzierungen stehen im Manifest; die Laufzeit erzeugt je Requisiten-Primitive ein `InstancedMesh`. | Fahrplan 4.3; wenige Draw Calls; Requisiten werden einmal geladen. |
| Kontaktschatten | Beim Backen der Insel stehen die platzierten Requisiten als Verdecker in der Szene, werden aber nicht in das Bahn-GLB exportiert. | Requisiten wirken geerdet, ohne Echtzeit-AO. |
| Banden-Optik | Die Kollisionsnetze `rail-*` bleiben im GLB und werden zur Laufzeit ausgeblendet. Sichtbar ist eine Verkleidung `rail-trim-*` je Welt, deren Innenfläche deckungsgleich mit der Bandeninnenfläche liegt. Die Verkleidung ist reine Deko. | Gefaste, weltspezifische Banden ohne Eingriff in die Kollision. |
| Vorschaubilder | Cycles-Renders 640×360 WebP unter `ui/3d/minigolf/previews/<look>.webp`. Manifest `worlds: [{ id, preview }]` aus dem Generator (P4-Schnittstelle). | P4 zeigt sie ohne Codeänderung. |
| Komprimierung | Keine Draco- oder Meshopt-Kompression. Budgets werden über Dreiecksbudgets und geteilte Vertices eingehalten. Der Blender-Exporter schreibt `COLOR_0` als float32. | Kein Decoder im Bundle; Kollisions-GLBs bleiben lesbar. Blender 5.2 kann Meshopt exportieren und three.js bringt den Decoder lokal mit; das bleibt die Reserve, falls Kits das Budget sprengen. |
| Blender-Datei | `aura-golf-courses.blend` wird nicht mehr versioniert. `generate.py --save-blend` schreibt sie bei Bedarf nach `assets/minigolf/build/` (ignoriert). | Der Generator ist die Quelle (Fahrplan: Variante A); eine Datei mit Kits und gebackenen Bahnen würde das Repository bei jedem Lauf um viele MB wachsen lassen. |
| Materialien je Requisit | Höchstens 3 Primitive je Requisit, ein `hero` darf 4 (Schiff: Holz, Tuch, Licht und Metall); je Welt höchstens 8 Materialien. Emission bekommt eine Stärke je Requisit; Helligkeitsunterschiede kommen aus der Vertexfarbe. Rückseiten werden außer bei `foliage` und dünnen Flächen weggelassen. | Jede Primitive ist ein Draw Call je Instanzgruppe. Der Spike zeigte 5 Primitive bei der Fackel durch fünf Emissionsstärken. Das Metall der Schiffsbeschläge ist sichtbare Qualität und kostet bei einem einzigen Hingucker je Welt nur einen Draw Call mehr je Bahn (Nutzerregel: Budgets dürfen moderat steigen, wo es sichtbare Qualität kauft). |

## 3. Generator (`assets/minigolf/`)

### 3.1 Module

`generate.py` wird zur schlanken CLI. Die Logik liegt im Paket `kit/`, das mit den Stilproben begonnen wurde.

| Modul | Aufgabe |
|---|---|
| `generate.py` | CLI: `--only kits\|courses\|previews\|all` (Vorgabe `kits,courses`), `--course N`, `--save-blend`; schreibt das Manifest. |
| `kit/palette.py`, `colors.py`, `materials.py`, `shapes.py`, `prop.py`, `bake.py` | Paletten, Verläufe, Materialien, Formen mit Fase/Subdivision/gewichteten Normalen, Teile-Pipeline, AO-Bake (bestehend aus den Proben). |
| `kit/props_<look>.py` (+ `ship_pirate.py`, `head_temple.py`) | Requisiten je Welt mit `PROPS`-Registrierung und Metadaten (Abschnitt 3.3). |
| `kit/kits.py` | Baut je Welt alle Requisiten einzeln, backt AO, exportiert `props-<look>.glb`, liefert Metadaten für das Manifest. |
| `kit/courses_v1.py` | Die 18 bisherigen Bahndefinitionen und die unveränderte Erzeugung von Spur, Banden, Loch, Hindernissen und Sprunglücken (aus `generate.py` verschoben, Geometriecode wortgleich). |
| `kit/dress.py` (+ `dress_island.py`, `trim.py`) | Umkleiden einer Bahn: Insel (`island.py`), Bandenverkleidung, Bake mit den platzierten Requisiten als Verdeckern. |
| `kit/dress_specials.py`, `special_parts.py`, `dress_parts.py` | Sonderteile je Layout (feste Platzierungen und handgebaute Geometrie), Weltstil für Hindernisse, Vorlagen, Fahne, Loch und Abschlag. |
| `kit/placement.py`, `occluders.py` | Deterministische Platzierung (Hingucker, Rand, Streudeko, Regelprüfung) und die Kit-Objekte als Bake-Verdecker. |
| `kit/export.py` | Export, kanonische Knotennamen, Zusammenfassen statischer Deko je Material, GLB-Lesen (Dreiecke, Kollider), Manifest-Einträge. |
| `kit/previews.py` | Vorschaubilder je Welt (Cycles, `--only previews`): Piraten Bahn 7 (das Schiff ist der Hingucker), Tempel Bahn 2, Mond Bahn 18. |
| `kit/probe.py`, `scene_<look>.py`, `stage.py`, `compose.py`, `island.py`, `lane.py` | Bleiben für Proben und Feinschliff; `island.py` und `compose.py` werden von `dress.py` mitbenutzt. |

Keine Datei wächst über etwa 400 Zeilen; größere Welten teilen ihre Requisiten auf mehrere Module auf.

### 3.2 Ablauf

1. **Kits:** Für jede Welt werden alle Requisiten gebaut, einzeln mit Bodenplatte gebacken (Kontakt-AO am Fuß) und als `props-<look>.glb` exportiert.
2. **Bahnen:** Für jede der 18 Bahnen in dieser Reihenfolge:
   1. Spur, Banden, Loch und Hindernisse unverändert erzeugen.
   2. Insel, Bandenverkleidung und Sonderteile bauen.
   3. Platzierungen bestimmen.
   4. Die passenden Kit-Requisiten als Verdecker einsetzen und die Bahn-Szene backen.
   5. Die Verdecker entfernen, statische Deko je Material zusammenfassen und das GLB exportieren.
   6. Kollider zurücklesen wie bisher.
3. **Manifest:** schreiben, `.blend` speichern.
4. **Vorschaubilder:** nur mit `--only previews`, getrennt, weil Cycles-Renders nicht bitgleich sind.

Laufzeit des Generators etwa 2 Minuten auf 16 Kernen (gemessen 80–110 s). Ein Export-Abbruch durch Windows-Dateisperren wird wie bisher durch erneutes Ausführen behoben.

### 3.3 Requisiten-Vertrag

- Ein Requisit ist ein Mesh-Knoten `prop-<look>-<name>` auf oberster Ebene, Y-up nach Export, Fuß bei y = 0, Grundriss zentriert, Meter.
- Es hat höchstens drei Primitive (ein `hero` vier). Jede Primitive hat `COLOR_0` aus Palettenverlauf × AO, 8 Bit je Kanal.
- Der Ursprung eines `hero` liegt in der Mitte seines kleinsten Grundrisskreises. Der `radius` ist so klein wie möglich und die Platzierung kann ihn größer skalieren.
- Knoten-Extras `aurago_prop`:
  - `radius`: Grundrissradius,
  - `height`,
  - `tags`: Teilmenge von `hero`, `scatter`, `tall`, `span`, `flat`,
  - optional `clearance` (lichte Höhe über der Spur, nur bei `span`),
  - optional `spin: { axis, speed }` für drehende Requisiten (ganze Instanz um ihre Achse).
- Material-Extras `aurago_kind` (`matte`, `satin`, `metal`, `glass`, `emissive`, `glow`, `foliage`). Je nach Art zusätzlich:
  - `aurago_strength` für `emissive`/`glow`,
  - `aurago_sway` (Amplitude in m) für `foliage`,
  - `aurago_flicker` (0–1) für Flammen.
- Je Welt sind etwa 15–25 Requisiten vorgesehen:
  - **Piraten:** Schiff, Palme, Fass, Truhe, Kanone, Steg, Leuchtturm, Totenkopffelsen, Hafenpfosten, Krabbe, Muschel, Seestern, Grasbüschel, Kiesel, X-Markierung, Segelboje.
  - **Tempel:** Steinkopf, Stufentor (`span`), Totem, Säule, Fackel, Liane, Farn, Busch, Wasserfall, Seerose, Sonnenrad, Felsen, Steinplatte, Kiesel.
  - **Mond:** Habitat, Parabolschüssel (Kopf mit `spin`), Rover, Solarfeld, Landepad, Funkturm, Neonpfosten, Kiste, Kristall, Krater, Felsbrocken, Kiesel.

### 3.4 Umkleiden einer Bahn

**Bitgleich bleiben** (aus `courses_v1.py`, Code wortgleich):
- die Knoten `surface-grass`, `surface-sand`, `rail-left` und `rail-right`,
- ihre Exportgeometrie und Vertexreihenfolge,
- Hindernis-Ausdehnungen,
- Abschlag, Loch, Wasservolumen, `bounds` und `colliders`.

Die Spur bekommt nur ein neues Material und Vertexfarben im Punktbereich (POINT). Die Banden behalten ihre Geometrie ohne Vertexfarben, weil die Laufzeit sie ausblendet (`aurago_hidden`); das spart je Bahn etwa 30 KB. Die Flächen bleiben flach schattiert, damit der Exporter die Vertices genau wie heute trennt. Der Asset-Test prüft das über einen Kollider-Fingerabdruck (Abschnitt 7).

**Neu:**

- **Insel** (`island.py`):
  - abgerundete Oberkante, Schichten an der Schnittkante;
  - Unterseite je Welt: Piraten und Tempel tauchen in Wasser ein, der Mond zeigt eine felsige Unterseite;
  - die Oberfläche folgt der Spur als Bett, 6 cm unter der Spur, auch auf Hügeln und Steilkurven;
  - unter Sprunglücken liegt ein Graben bis unter Wasserhöhe, auf dem Mond eine Schlucht ins All;
  - die Lochfunktion `top_height(x, y)` ersetzt `island_top_height` für die Lage des Lochbodens.
- **Bandenverkleidung `rail-trim-left/right`:** Der Querschnitt ist an der Innenseite deckungsgleich mit der Bande und oben abgerundet; er steht höchstens 2 cm nach oben und 3 cm nach außen über. Je Welt:
  - Piraten: Holz mit Pfosten;
  - Tempel: Steinblöcke mit Fugen;
  - Mond: Metall mit Neonleiste (Material `emissive`) und orangen Klammern.
  - Knoten-Extras `aurago_hidden: true` auf `rail-left/right`.
- **Loch, Fahne und Bahnnummer:** im Weltstil gefast; Fahne in Weltfarbe. Die Ziffernsegmente und der Lochrand bekommen eine Fase von 8 mm (zwei Segmente); der Lochrand wird dafür 2 cm dick, seine Oberkante bleibt, wo sie war. Beides ist keine Kollisionsgrundlage.
- **Vorlagen:**
  - Der Ball wird feiner unterteilt; alle Vertices liegen exakt auf 0,16 m.
  - Der Putter bekommt einen gefasten Kopf; der Schaft reicht weiter von 0 bis 0,82 m.
- **Sonderteile:** Lage und Maße bleiben, Stil und Material folgen der Welt. Thematisch unpassende Teile werden durch Weltrequisiten ersetzt.

  | Layout | Welt | Umsetzung |
  |---|---|---|
  | `flower_arch` | temple | Stufentor (`span`, lichte Höhe ≥ 1,4 m) über der Spur |
  | `bank` | temple | Felsvorsprünge aus dem Kit |
  | `bridge` | temple | Steinbrüstung, gefast |
  | `windmill` | temple | Sonnenrad; die drehenden Teile heißen weiter `windmill-sail*` |
  | `beach_curve` | pirate | Palmen aus dem Kit |
  | `sand` | pirate | Dünengras als Streudeko |
  | `pier` | pirate | Holzplanken und Pfähle, gefast |
  | `shortjump` | pirate | Sprungrampen und Graben mit Wasser |
  | `harbor_gate` | pirate | Hafenpfosten aus dem Kit |
  | `lighthouse` | pirate | Leuchtturm aus dem Kit |
  | `lantern_curve`, `revolving_gate`, `finale` | moon | Neonpfosten statt Laternen; Funkturm statt Leuchtturm |
  | `crystals` | moon | Mondkristalle aus dem Kit (Banden bleiben `stone`) |
  | `moonbridge` | moon | Metallbrücke und Schlucht |
  | `lilyjump` | moon | Schwebe-Landepads statt Seerosen und Schlucht |
- **Platzierung (`dress.py`):** deterministisch aus einem festen Seed je Bahn.
  1. **Hingucker:** Das `hero`-Requisit der Welt kommt auf die größte freie Fläche (Kandidatenraster, höchste Freiheit, leicht nach hinten bevorzugt); es wird so groß skaliert, wie sein Kreis passt (mindestens 0,42). Bei Gleichstand gewinnt die kleinste Rasterposition. Die Insel ist nur etwa 9,8 × 10,7 m groß und die Spur teilt sie, deshalb liegt der Hingucker meist bei 0,55 bis 0,75.
  2. **Rand:** 8–14 mittlere Requisiten in Randplätzen.
  3. **Streudeko:** in freien Flächen bis zu einer Dichte von etwa 0,8 je m².
  - Kein Grundriss schneidet den Korridor um die Spurmitte, außer `span`. Der Test verlangt 1,0 m, der Generator hält 1,2 m, damit Bandenverkleidung und Pfosten frei bleiben.
  - Alles liegt innerhalb des Inselrands mit 0,3 m Abstand. Hohe Requisiten, Pflanzen und der Hingucker dürfen mit einem Anteil ihres Radius überhängen (Palmwedel, Takelage, Brauen), weil der Radius der Hüllkreis ist und dünne Teile ihn dominieren. Die Regel gilt für jede Platzierung, auch für feste Sonderteile (sie rücken nötigenfalls in Schritten zur Inselmitte), und für den Kopf der Parabolschüssel, dessen Kreis größer ist als der ihres Sockels. `placement.check` bricht den Bau ab, der Asset-Test prüft es gegen `courses[].island` (Abschnitt 4).
  - Nichts steht im Graben einer Sprunglücke; hohe Requisiten, die die Spur aus der Spielkamera verdecken würden, und alles Hohe vor dem Hingucker entfallen.
  - Die Höhe kommt aus der Inselfunktion.

### 3.5 Export und Zusammenfassen

- Funktionsknoten bleiben einzeln:
  - `surface-*`, `rail-*`, `rail-trim-*`, `flag`,
  - `ball-template`, `putter-template`,
  - Hindernisse, `windmill-sail*`,
  - `water-shore`.
- Alle übrigen statischen Teile werden je Material zu einem Knoten `scenery-<kind>` zusammengefasst. Ziel: höchstens 24 Knoten mit Mesh je Bahn-GLB, heute 135.
- Ein Bahn-GLB enthält kein offenes Wasser mehr. Es enthält nur noch die Uferzone `water-shore`: einen Streifen 0–0,9 m vor der Wasserlinie, in `TEXCOORD_0.x` der Abstand zum Ufer (0 = Ufer, 1 = außen). Das offene Wasser zeichnet die Laufzeit. Der Mond hat keine Uferzone.
- Das Material steuert die Laufzeit über die Extras aus 3.3.

### 3.6 Determinismus

- Zufall nur aus `random.Random(seed)` und `mathutils.noise`.
- Bakes auf der CPU mit festem Seed und fester Samplezahl, ohne Denoise, auf 8 Bit gerundet.
- Zwei Generatorläufe auf derselben Maschine mit Blender 5.2.2 ergeben dieselben GLB-Hashes und dasselbe Manifest. Das wird bei der Abnahme einmal geprüft.
- Jede Bahn wird nach dem Export mit Objekten, Meshes, Szene und allen verwaisten Datenblöcken gelöscht. Dadurch trägt kein Knoten- oder Mesh-Name einen Blender-Zähler (`scenery-matte.001`), und `--course N` in einem frischen Prozess schreibt dieselben Dateien wie der volle Lauf. Die vier Sonnenrad-Flügel heißen `windmill-sail-0` bis `windmill-sail-3`. Mit `--save-blend` bleibt die Szene erhalten, deshalb ist es nur zusammen mit `--course N` erlaubt.
- Vorschaubilder sind ausgenommen.

## 4. Manifest (Version bleibt 1, nur additive Felder)

| Feld | Inhalt |
|---|---|
| `courses[].look` | `pirate`, `temple` oder `moon` |
| `courses[].path` | Spurmitte (Spuroberfläche) vom Abschlag bis zum Spurende in Laufzeitkoordinaten: jeder dritte Generator-Stützpunkt plus der letzte (17–33 Punkte), 3 Nachkommastellen. Die Spur läuft hinter dem Loch (bei 91 %) noch kurz weiter, das Ende liegt bis zu 1 m vom Loch. |
| `courses[].props` | Platzierungen `[name, x, y, z, yaw, scale]`; `name` ohne Präfix, `yaw` in Radiant um Y |
| `courses[].island` | Inselumriss in Laufzeitkoordinaten: `{ center: [x, z], half: [hx, hz], power, wobble }`. Der Rand ist die Superellipse `\|dx/hx\|^power + \|dz/hz\|^power = 1` um `center`; der echte Rand skaliert ihren Radius je Winkel um höchstens `1 ± wobble` (gemessene größte Abweichung über 720 Winkel, auf Tausendstel aufgerundet). `center` und `half` in Metern; alle Zahlen mit 3 Nachkommastellen. |
| `kits` | Je Look: `{ model, bytes, sha256, triangles, props: { <name>: { triangles, radius, height, tags } } }` |
| `worlds` | `[{ id, look, preview }]` in Reihenfolge des ersten Auftretens, z. B. `{ id: 'coast', look: 'pirate', preview: 'previews/pirate.webp' }` |

Die Spielprüfung in `minigolf-game.js` (Version 1, 18 Bahnen, `course-NN.glb`) bleibt unverändert gültig.

## 5. Laufzeit (three.js 0.186.1)

### 5.1 Module (`ui/js/desktop/apps/`)

| Modul | Aufgabe |
|---|---|
| `minigolf-scene.js` | Bleibt Taktgeber und öffentliche API; lädt Bahn und Kit, verteilt `update`, reicht Qualitätsstufen weiter |
| `minigolf-looks.js` | Reine Daten und kleine Hilfen je Look: Himmel, Nebel, Sonne, Belichtung, Umgebungslicht, Bloom, Wassermodus, Partikelfarben, Kamerafahrt-Takt |
| `minigolf-sky.js` | Himmelskuppel je Look und PMREM-Umgebung aus derselben Kuppel |
| `minigolf-water.js` | Offenes Wasser und Uferzone (Lagune, Dschungel); im Mond-Look kein Wasser |
| `minigolf-props.js` | Kit laden und je Look zwischenspeichern, Instancing, Wiegen/Flackern/Drehen, Materialien aus Extras |
| `minigolf-fx.js` | Partikel-Pools, Leuchtspur, Einloch-Feier, Feuerwerk |
| `minigolf-camera.js` | Kamerafahrt entlang `path`, bestehende Turn-Überblendung, Folgen |

Alle neuen Module importieren nur `three` und `minigolf-shared.js`. Reine Funktionen werden exportiert und in Node getestet. Das sind Look-Tabelle, Flugbahn, Platzierungsmatrizen und Partikelgrenzen.

### 5.2 Licht

- **Umgebung:**
  - `PMREMGenerator.fromScene` über die Himmelskuppel des Looks plus eine helle Sonnenscheibe; einmal je Look erzeugt und zwischengespeichert.
  - Daraus `scene.environment` und `environmentIntensity` je Look (0,5–0,9).
  - Das Hemisphärenlicht entfällt.
- **Sonne:**
  - Richtung, Farbe und Stärke je Look; Piraten hoch und warmweiß, Tempel tief und orange, Mond seitlich und kaltweiß.
  - Die Schattenkamera wird auf die Insel begrenzt (±9 m statt ±18 m), das gibt schärfere Schatten bei gleicher Kartengröße.
- **Mond:** zusätzlich ein schwaches Gegenlicht ohne Schatten für Kanten.
- **Materialien aus dem Kit:**
  - `emissive`/`glow`: Emission = Vertexfarbe × exportierte Stärke (`emissiveFactor` bzw. `KHR_materials_emissive_strength`, entspricht `aurago_strength`).
  - `metal` und `glass`: nutzen die Umgebung für Reflexe.

### 5.3 Himmel

Eine Kuppel mit einem Shader je Look, ohne Texturen:

- **Piraten:** Verlauf Türkis-Hellblau, Sonnenhof, weiche Wolken aus 2 Oktaven Rauschen, langsam ziehend.
- **Tempel:** Abendverlauf Pfirsich → Petrol, große tiefe Sonne mit Hof, Dunstband am Horizont, Dschungel-Silhouette als Rauschband.
- **Mond:**
  - Schwarzblau mit Sternen (Punkte, auf „hoch“ funkelnd) und schwachem Milchstraßenband.
  - Die Erde als Kugel mit prozeduralem Shader: Kontinente aus Rauschen, Wolken, Atmosphärenrand, Tag-Nacht-Grenze passend zur Sonne.
- **Nebel:** Piraten leichter Aqua-Dunst, Tempel warmer Dunst, Mond kein Nebel.

### 5.4 Wasser

- **Offenes Wasser** (Piraten, Tempel):
  - eine große Scheibe (Radius 60 m) auf Wasserhöhe;
  - Farbe von hell (Ufer) zu tief (außen) nach Abstand zur Inselmitte;
  - Wellen aus zwei Sinusfeldern, Glanzlichter, Fresnel mit der Umgebung;
  - am Rand Übergang in den Nebel.
- **Uferzone `water-shore`:** Schaumband und Flachwasser über `TEXCOORD_0.x`, animierter Schaumrand; ersetzt die bisherigen Schaumstreifen-Boxen.
- **Mond:** kein Wasser. Die Himmelskuppel ist auch unter dem Horizont sichtbar, mit Sternen.
- Animation pausiert bei reduzierter Bewegung, das Bild bleibt.

### 5.5 Requisiten zur Laufzeit

- Das Kit des Looks wird einmal geladen (`fetch` + `GLTFLoader.parseAsync`, wie Bahnen) und je Szene zwischengespeichert. Der Kit-Fetch läuft parallel zum Bahn-Fetch.
- Je Platzierungsgruppe und Primitive entsteht ein `InstancedMesh` mit Instanzmatrix aus Position, Gier und Skalierung.
  - `castShadow` außer bei `flat`, `receiveShadow` immer.
  - Begrenzungskugel über alle Instanzen für Culling.
- Wiegen (`foliage`):
  - Vertexshader-Verschiebung `amp · (y/h)² · sin(t·f + Phase)`; die Phase kommt aus der Instanzposition.
  - Nur auf „mittel“ und „hoch“ und ohne reduzierte Bewegung.
- Flackern (`emissive` mit `aurago_flicker`): Emission × (1 − k + k·Rauschen(t + Phase)).
- Drehen (`spin`): Instanzmatrizen dieser Gruppe werden je Frame neu gesetzt (wenige Instanzen).
- Streudeko (`scatter`) entfällt auf „niedrig“.
- `rail-left/right` mit `aurago_hidden` werden unsichtbar geschaltet.

### 5.6 Partikel und Effekte

- **Pools:**
  - weiche Punkte (Sand, Staub, Spritzer, Funken, Glühwürmchen, Glanz);
  - instanzierte Vierecke (Konfetti, Splitter).
  - Feste Puffer, keine Allokation im Frame.
  - Obergrenzen nach Qualitätsstufe (Abschnitt 5.9).
- **Auslöser** (`burst(type, position, speed, info)`, `info` optional `{ surface }`):

  | Ereignis | Effekt |
  |---|---|
  | Schlag | Grashalme bzw. Sandwolke; Mond: Staub |
  | Aufprall | Holzsplitter (wood), Steinstaub (stone); Mond: Funken |
  | Rollen auf Sand | feine Sandfahne (Snapshot-Feld `surface`) |
  | Wasser | Spritzer und Ring; Mond: Staubwolke |
  | Loch | Einloch-Feier: Konfetti in Weltfarben, goldener Ring auf dem Grün (0 → 1,6 m in 0,8 s), Fahne weht 2 s kräftig, Glitzersäule |
  | Sieg | Konfetti von drei Punkten und 3 Feuerwerksraketen über der Insel |

- **Umgebung:** Piraten Lichtglitzern über dem Wasser, Tempel Glühwürmchen und Fackelfunken, Mond schwebender Staub.
- **Leuchtspur:**
  - Band aus 28 Punkten in Spielerfarbe, additiv, verjüngt und verblassend;
  - sichtbar ab 0,6 m/s Ballgeschwindigkeit;
  - aus auf „niedrig“.
- **Reduzierte Bewegung:**
  - keine Partikel, kein Feuerwerk;
  - die Einloch-Feier zeigt nur einen kurz eingeblendeten goldenen Ring ohne Skalierung;
  - die Leuchtspur bleibt, weil sie nur der Ballbewegung folgt.

### 5.7 Kamerafahrt

- Startet nach jedem `load()` ohne reduzierte Bewegung, auch beim Menü-Hintergrund.
- **Dauer:** 0,35 s je Meter Spurlänge, begrenzt auf 2,5–4,5 s.
- **Phase A (0–60 %):**
  - Die Kamera gleitet in Spielrichtung entlang `path` vom Abschlag bis über das Loch, 2,2 m hinter und 3,2 m über dem jeweiligen Bahnpunkt.
  - Sie blickt 2,5 m voraus auf die Spur, mit weichem Anfahren und Abbremsen.
- **Phase B:** weiches Ausschwenken in die bisherige Übersichtspose (`focusBall(true)`).
- **Überspringen:**
  - Zeiger-Druck auf die Fläche, Mausrad, jede Taste (`minigolf-game.js` ruft `skipIntro()` im Tasten-Handler) und `focusBall()` beenden den Flug sofort in der Endpose.
  - Eingaben werden dabei nicht verbraucht. Ein Zug oder Tastendruck wirkt danach wie bisher, weil die Endpose genau der heutigen Ladepose entspricht.
- `frameTurn` während des Flugs wird gemerkt und danach ausgeführt.
- `stats().intro` meldet den laufenden Flug.

### 5.8 Bloom

UnrealBloomPass je Look:

| Look | Stärke | Radius | Schwelle |
|---|---|---|---|
| pirate | 0,14 | 0,4 | 0,95 |
| temple | 0,22 | 0,5 | 0,85 |
| moon | 0,42 | 0,55 | 0,75 |

Auf „niedrig“ ohne Composer (wie heute).

### 5.9 Qualitätsstufen

Die Stufe folgt `tier()` (P4-Governor bei „auto“).

| | niedrig | mittel | hoch |
|---|---|---|---|
| Pixelverhältnis | 1 | ≤ 1,5 | ≤ 2 |
| Schatten | aus (Blob) | 1024, PCF | 2048, PCF weich |
| Bloom | aus | halbe Auflösung | halbe Auflösung |
| Umgebung (PMREM) | an | an | an |
| Himmel | Verlauf, Sonne, Dunst, Silhouette, Erde; ohne Wolken und Funkeln | + Wolken | + Sternfunkeln |
| Wasser | Verlauf und Uferschaum statisch | animiert | + Glanzlichter |
| Requisiten | ohne `scatter` | alle | alle |
| Wiegen/Flackern/Drehen | aus | an | an |
| Partikel max. | 60 | 160 | 320 |
| Leuchtspur | aus | an | an |

### 5.10 API-Änderungen (alle additiv)

- `scene.skipIntro()` → `boolean`.
- `scene.burst(type, position, speed, info)` mit optionalem `info.surface`.
- `scene.update(dt, snapshot)`: Snapshot darf `surface` tragen.
- `stats()` neu:
  - `look`,
  - `props` (Anzahl sichtbarer Instanzen),
  - `env` (Umgebung gesetzt),
  - `intro`,
  - `trail` (sichtbare Spurpunkte).
- `stats().triangles`/`calls` zählen den ganzen Frame (`renderer.info.autoReset = false`, Rücksetzen je Frame). Bisher zeigten sie nur den letzten Composer-Pass; Ausgangswerte dieser beiden Felder sind deshalb nicht vergleichbar, die Bildrate schon.
- `minigolf-game.js` bekommt genau vier kleine Änderungen:
  1. `skipIntro` im Tasten-Handler (ohne die Taste zu verbrauchen),
  2. `info` beim Aufprall-Burst,
  3. `info` beim Schlag-Burst,
  4. `surface` im Snapshot.

  Alle Aufrufe sind optional (`?.`).

## 6. Budgets

| Größe | Grenze | Vor P2 | Ergebnis P2 (2026-10-21) |
|---|---|---|---|
| Laufzeit-Assets gesamt (alles unter `ui/3d/minigolf/`) | ≤ 32 MiB | 9,3 MB | 21,3 MB |
| Bahn-GLB | ≤ 768 KiB und ≤ 24 000 Dreiecke (Export kostet etwa 33 B je Dreieck) | ≤ 458 KB, ≤ 5 609 Dreiecke | ≤ 755 KB, ≤ 21 149 Dreiecke |
| Knoten mit Mesh je Bahn-GLB | ≤ 24 | 135 | ≤ 18 |
| Kit-GLB je Welt | ≤ 3 MiB und ≤ 60 000 Dreiecke | – | ≤ 1,77 MB, ≤ 57 226 Dreiecke |
| Einzelnes Requisit | ≤ 15 000 Dreiecke (`hero` ≤ 20 000; der Tempelkopf braucht 16 444 für lesbare Gesichtszüge) | – | Hero ≤ 16 444 |
| Instanzen je Bahn | ≤ 160 | – | ≤ 91 |
| Vorschaubild | ≤ 80 KB, 640×360 WebP | – | ≤ 19,2 KB |
| Spiel-Bundle | ≤ 6 MiB (seit P4) | ca. 5,1 MB | 5 131 500 B |
| Bildrate je Stufe | ≥ Ausgangsmessung (Benchmark 1920×1080, gleiche Maschine) | Bildtakt-Grenze aller Stufen | gleiche Grenze, p95 je Stufe innerhalb 1 ms; Draw Calls auf „niedrig“ 149 → 47 |

Angehobene Budgets (Nutzerregel vom 2026-10-20, „hebe das Größenbudget etwas an, wenn sonst die Qualität leidet“): Primitive je `hero`-Requisit 3 → 4 (Metallbeschläge am Piratenschiff) und Dreiecke je `hero` 16 000 → 20 000 (Gesicht des Tempelkopfs). Alle übrigen Grenzen blieben unverändert.

## 7. Tests (schlank)

- **`scripts/test-minigolf-assets.mjs`:**
  - neue Budgets aus Abschnitt 6;
  - Fingerabdrücke je Bahn gleich der P1-Datei `scripts/fixtures/minigolf-p1-colliders.json`: `colliders` (SHA-256 über die Kollider-JSON) und `physics` (SHA-256 über `{ obstacles, tee, cup, cupRadius, bounds, water, par }` in dieser Schlüsselreihenfolge);
  - keine nummerierten Namen (`scenery-matte.001`) bei Knoten und Meshes der Bahn-GLBs;
  - `look` passt zur Welt;
  - `path` beginnt ≤ 0,3 m vom Abschlag und endet ≤ 1,0 m vom Loch (das Spurende liegt hinter dem Loch; der Korridor schützt so auch das Endstück);
  - jede Platzierung verweist auf ein Kit-Requisit, hält den Abstand zum Inselrand (`courses[].island` mit Wackeltoleranz; 0,3 m plus der Anteil des Radius, den das Requisit nicht überhängen darf) und hält den Spurkorridor frei (außer `span`);
  - Kits: gültig nach glTF-Validator, eingebettet, Requisitenknoten mit `COLOR_0`, Budgets, Manifest-Hash;
  - Vorschaubilder vorhanden (WebP-Kennung, Größe);
  - bestehende Prüfungen bleiben.
- **`scripts/test-minigolf-courses.mjs`:** unverändert grün, die aufgezeichneten P1-Routen ohne Neuaufnahme.
- **Neu `scripts/test-minigolf-look.mjs`** (in `npm run test:minigolf`):
  - Look-Tabelle vollständig für alle drei Looks;
  - Flugbahn: Dauer-Begrenzung, Start über dem Loch, Ende in der Übersichtspose;
  - Platzierungsmatrix: Gier und Skalierung;
  - Partikelgrenzen je Stufe.
- **Browser-Smoke `TestDesktopMinigolfBrowser`** je Welt:
  - `graphics.look`, `props > 0`, `env`;
  - Kamerafahrt läuft und endet nach `skipIntro`;
  - bei reduzierter Bewegung kein Flug;
  - keine externen Anfragen.
- **Benchmark** (`AURAGO_MINIGOLF_BENCHMARK=1`): vor und nach P2 auf derselben Maschine, Ergebnis im README.
- `TestShippedWebAssetsLoadNothingExternal`, `node scripts/build-minigolf.mjs --check`, `npm run build:ui -- --check` bleiben grün.

## 8. Risiken

| Risiko | Gegenmaßnahme |
|---|---|
| Neues Material oder Vertexfarben ändern die Vertex-Aufteilung der Kollisionsnetze | Fingerabdruck-Test. Wenn er bricht: Spur und Banden ohne Vertexfarben exportieren und nur über das Material färben |
| Bake nicht bitgleich zwischen Läufen | CPU, fester Seed, 8-Bit-Rundung, Glättung; zweiter Lauf als Abnahme |
| Kit-Dateien zu groß | Dreiecksbudget je Requisit, `densify` nur wo AO es braucht, geteilte Vertices; Kompression nur als spätere Option |
| Himmel- und Wasser-Shader kosten Bildrate | Rauschen nur auf „mittel/hoch“, Benchmark je Stufe gegen die Ausgangsmessung |
| Requisiten verdecken Ball oder Spur | Korridor-Regel im Test; `span`-Teile mit lichter Höhe ≥ 1,4 m |
| Generator dauert lange | `--course N` und `--only` für Teilläufe; voller Lauf im Hintergrund |

## 9. Nicht Teil von P2 (Übergabe an P3)

- Neue Bahnen, Bahnstücke, Mechaniken, Manifest v2, binäre Kollisionsdaten, neue Welt-IDs und Bahnnamen.
- P3 nutzt Kits, `dress.py`, `export.py`, `look`/`path`/`props`/`kits`/`worlds` und ersetzt `courses_v1.py`.
- Die Weltnamen-Schlüssel `minigolf.pirate|temple|moon` führt P3 ein; bis dahin tragen die alten Schlüssel die neuen Namen.
