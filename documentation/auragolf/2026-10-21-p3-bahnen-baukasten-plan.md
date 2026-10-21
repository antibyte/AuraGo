# AuraGolf P3 – Bahnen-Baukasten & 18 Bahnen (Umsetzungsplan)

> **Für ausführende Agenten:** Aufgabe für Aufgabe abarbeiten (superpowers:subagent-driven-development). Schritte mit `- [ ]`. Nach jeder Aufgabe committen; nur eigene Pfade stagen (andere Sessions arbeiten im selben Checkout).

**Ziel:** 18 neue AuraGolf-Bahnen aus einem Bahnstück-Baukasten mit exakter Kollision, neue Mechaniken, Manifest v2 und GLB-gelesene Kollision, gemäß [P3-Design](2026-10-21-p3-bahnen-baukasten-design.md).

**Architektur:** Laufzeit (JS) und Generator (Blender-Python) entstehen getrennt und treffen sich an drei Verträgen: dem Kollisionsknoten-Vertrag im GLB (`aurago_collider`, Digest), dem Manifest-v2-Schema und den Mechanik-/Hindernisfeldern. Bis zur Umstellung (Aufgabe 11) schreibt der Generator in den ignorierten Ordner `assets/minigolf/build/p3/`; das Spiel und alle Tests bleiben auf den P2-Bahnen grün. Die Umstellung ersetzt dann alles in einem Schritt.

**Technik:** Blender 5.2.2 headless (`D:\Blender 5.2\blender.exe`), Python ohne Fremdpakete, Rapier 0.21.0 (`@dimforge/rapier3d-compat`), three.js 0.186.1, Node-Testskripte, Go-Browsertests.

**Arbeitsregeln (gelten für jede Aufgabe):**
- Vor dem Editieren die DOX-Kette lesen: Root `AGENTS.md`, `ui/AGENTS.md` (Abschnitt AuraGolf in `ui/js/desktop/apps/AGENTS.md`), `assets/minigolf/kit/AGENTS.md`.
- Keine Prozesse nach Abbildnamen beenden (`taskkill /IM`, `pkill -f`); nur eigene PIDs.
- Zur Laufzeit nichts von außen laden; keine `cc-`-Klassen.
- Module klein halten (≈ 400 Zeilen Python, JS-Dateien nicht über ≈ 600 Zeilen).
- Schlanke Tests: nur die genannten Prüfungen.
- Windows: Blender-Exporte können an einer Dateisperre scheitern → Befehl wiederholen.
- Temporäres in `disposable/_p3/` oder im Scratch-Ordner; Renders zur Sichtprüfung nicht committen.

---

## Dateiübersicht

**Laufzeit (`ui/js/desktop/apps/`)**
- Neu `minigolf-collision.js` – GLB lesen, Kollisionsknoten sammeln, verschweißen, Digest.
- Neu `minigolf-mechanics.js` – Hindernis-Posen, Mechanik-Zonen, Bumper-Stoß (rein, ohne Rapier-Import).
- Ändern `minigolf-physics.js` – Schwerkraft, typisierte Arrays, neue Kollisionsformen, Mechaniken, nicht-terminale Ereignisse, Vorschau.
- Ändern `minigolf-scene.js` – Kollider aus dem GLB, Rahmung nach Inselgröße, `aurago_spin`, `resetTrail`.
- Ändern `minigolf-camera.js` – reine Funktion `overviewFor(course)`.
- Ändern `minigolf-fx.js` – Burst-Typ `spark`.
- Ändern `minigolf-game.js` – Manifest v1/v2 (Übergang), Kollider an Physik und Karte, Mechanik-Ereignisse.
- Ändern `minigolf-minimap.js` – typisierte Arrays.
- Ändern (Umstellung) `minigolf-looks.js`, `minigolf-audio.js`, `minigolf-menus.js` – Altwelt-IDs entfernen.
- Ändern `minigolf-shared.js` – nichts außer bei Bedarf `FLOOR_SURFACES` (bleibt `grass`, `sand`).

**Skripte**
- Neu `scripts/test-minigolf-collision.mjs`, `scripts/lib/minigolf-routes.mjs`, `scripts/minigolf-route-search.mjs` (Entwickler-CLI).
- Ändern `scripts/test-minigolf-physics.mjs`, `scripts/test-minigolf-look.mjs`, (Umstellung) `scripts/test-minigolf-courses.mjs`, `scripts/test-minigolf-assets.mjs`, `package.json` (`test:minigolf`).
- Löschen (Umstellung) `scripts/fixtures/minigolf-p1-colliders.json`.

**Generator (`assets/minigolf/kit/`)**
- Neu `track.py`, `pieces.py`, `pieces_moon.py`, `pads.py`, `pads_funnel.py`, `layout.py`, `build_track.py`, `basic_parts.py`, `courses.py`, `mechanic_parts.py`, `obstacle_parts.py`, `course_data_pirate.py`, `course_data_temple.py`, `course_data_moon.py`, `track_selftest.py`.
- Ändern `export.py`, `dress.py`, `dress_island.py`, `island.py` (nur falls nötig), `trim.py`, `placement.py`, `dress_parts.py`, `../generate.py`, `previews.py`.
- Löschen (Umstellung) `courses_v1.py`, `dress_specials.py`, `special_parts.py` (soweit nichts mehr davon genutzt wird).

**Daten und Texte**
- `ui/3d/minigolf/` (Umstellung): `course-01.glb` … `course-18.glb`, `manifest.json` v2, `previews/*.webp`.
- `ui/lang/desktop/*.json` (16 Dateien): `minigolf.pirate|temple|moon`, `minigolf.course_1` … `course_18`.

**Doku**
- `documentation/minigolf.md`, `assets/minigolf/README.md`, `assets/minigolf/kit/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md` (AuraGolf), Root `AGENTS.md` (Kind-Index-Zeile für `assets/minigolf/kit/AGENTS.md`).

---

## Reihenfolge und Parallelität

```
Stufe A (JS)          Stufe B (Python)
 1 Kollisionsleser     5 Baukasten-Kern
 2 Mechaniken/Physik   6 Mondstücke, Trichter, Gabelung, Sprung
 3 Spiel/Szene         7 Bahn-Pipeline, Diorama, Manifest v2
 4 Routensuche         8 Mechanik- und Hindernisteile
        \                 /
         Stufe C: 9 Inhalte (Piraten → Tempel → Mond)
         Stufe D: 10 Namen in 16 Sprachen
                  11 Umstellung (Assets, Tests, Vorschaubilder)
                  12 Doku, Gesamtprüfung, Landung
```

A und B laufen parallel (getrennte Dateien). Höchstens zwei Agenten gleichzeitig. Je Stufe eine gemeinsame Prüfung (test-agent) nur auf kritische und wichtige Befunde.

---

## Verträge (verbindlich für alle Aufgaben)

### V1 Kollisionsknoten im GLB

- Ein Kollisionsknoten ist ein Mesh-Knoten mit Extra `aurago_collider` ∈ {`grass`, `sand`, `wood`, `stone`, `metal`}.
- Kollisionsobjekte haben in Blender eine Einheits-Transformation (Vertices in Weltkoordinaten). Der Leser beherrscht trotzdem TRS, Matrix und Elternketten.
- Bodenknoten (`floor-*`) sind sichtbar. Bandenknoten (`rail-*`) tragen zusätzlich `aurago_hidden: true`.
- **Verschweißen:** Je Oberfläche alle Dreiecke aller Knoten in Knotenreihenfolge (Reihenfolge der `nodes` im GLB, dann Primitive) sammeln. Vertex-Schlüssel = die drei Float32-Bitmuster nach Transformation. Erste Vorkommen behalten Reihenfolge. Dreiecke mit zwei gleichen Indizes nach dem Verschweißen entfallen.
- **Digest:** SHA-256 über, je Oberfläche in der Reihenfolge `grass, sand, wood, stone, metal` (leere auslassen): UTF-8 Name, Byte `0x00`, Uint32 LE Vertexzahl, Uint32 LE Indexzahl, Float32 LE Vertices, Uint32 LE Indizes.
- Manifest: `collision: { nodes, triangles, digest }` (`nodes` = Zahl der Kollisionsknoten, `triangles` = Summe nach dem Verschweißen).

### V2 Manifest v2

Wie Design Abschnitt 6. Pflichtfelder je Bahn: `id, number, world, par, model, sha256, bytes, triangles, tee, cup, cupRadius, gravity, cycle, length, bounds, water, collision, obstacles, mechanics, path, corridors, props, island`. Kein `colliders`, kein `look`. Wurzel: `version: 2, courses, kits, worlds: [{id, preview}]`.

### V3 Mechaniken und Hindernisse

Wie Design 5.3 und 5.4. Koordinaten in Laufzeit-Koordinaten (Y oben). `yaw` in Radiant um +Y. Richtungen `[dx, dz]` normiert. Jede Mechanik und jedes Hindernis hat eine eindeutige `id` (`^[a-z][a-z0-9-]{0,31}$`). Hindernisknoten heißen `obstacle-<id>`; das Hindernis-Feld `node` nennt ihn.

---

## Stufe A – Laufzeit

### Aufgabe 1: Kollisionsleser `minigolf-collision.js`

**Dateien:**
- Neu: `ui/js/desktop/apps/minigolf-collision.js`
- Neu: `scripts/test-minigolf-collision.mjs`
- Ändern: `ui/js/desktop/apps/minigolf-physics.js` (Mesh-Validierung), `ui/js/desktop/apps/minigolf-minimap.js` (`laneGeometry`), `package.json` (`test:minigolf` um das neue Skript ergänzen, direkt nach dem Physiktest)

- [ ] **Schritt 1: Test schreiben.** `scripts/test-minigolf-collision.mjs` baut im Test ein kleines GLB von Hand (JSON-Chunk + BIN-Chunk, 4-Byte-Ausrichtung): Knoten 0 Elternteil mit `translation [1,0,0]` und `rotation` 90° um Y, Kind Knoten 1 mit Mesh (zwei Dreiecke, ein Quadrat) und `extras.aurago_collider: "grass"`; Knoten 2 mit `matrix` und einem Dreieck, dessen zwei Vertices bitgleich mit Knoten-1-Vertices in Weltkoordinaten liegen, ebenfalls `grass`; Knoten 3 ohne Extra (wird ignoriert); Knoten 4 `rail` mit `aurago_collider: "wood"`. Prüfen:
  - zwei Gruppen in Reihenfolge `grass`, `wood`; `vertices` ist `Float32Array`, `indices` `Uint32Array`;
  - Weltkoordinaten eines Vertex von Knoten 1 nach Eltern-TRS (erwarteter Wert von Hand gerechnet);
  - Verschweißen: `grass` hat 4 + 1 Vertices statt 4 + 3;
  - ein degeneriertes Dreieck (zwei gleiche Vertices) entfällt;
  - `await colliderDigest(groups)` ist 64 Hex-Zeichen und stabil (Wert nach dem ersten grünen Lauf festschreiben);
  - Fehler bei kaputtem GLB (falsche Magie, fehlender BIN-Chunk) und unbekannter Oberfläche.
- [ ] **Schritt 2: Test laufen lassen,** erwartet: Fehler „Cannot find module …minigolf-collision.js“.
- [ ] **Schritt 3: Implementieren** (Kernstücke):

```js
export const COLLIDER_SURFACES = Object.freeze(['grass', 'sand', 'wood', 'stone', 'metal']);

export function parseGlb(buffer) {
    const view = new DataView(buffer);
    if (view.byteLength < 20 || view.getUint32(0, true) !== 0x46546C67) throw new TypeError('Invalid GLB');
    let offset = 12, json = null, bin = null;
    while (offset + 8 <= view.byteLength) {
        const length = view.getUint32(offset, true), kind = view.getUint32(offset + 4, true);
        const start = offset + 8;
        if (kind === 0x4E4F534A) json = JSON.parse(new TextDecoder().decode(new Uint8Array(buffer, start, length)).replace(/[\0 ]+$/, ''));
        else if (kind === 0x004E4942) bin = new Uint8Array(buffer, start, length);
        offset = start + length;
    }
    if (!json || !bin) throw new TypeError('GLB is missing chunks');
    return { json, bin };
}
```

  - Transformation je Knoten wie `export.glb_fixed_colliders` in Python: Matrix (spaltenweise) oder TRS mit derselben Rechenreihenfolge (Skalierung, dann Quaternion-Rotation per `t = 2·(q × p)`, `p + w·t + q × t`, dann Translation); Elternkette vom Knoten zur Wurzel. In Float64 rechnen, beim Schreiben in `Float32Array` runden.
  - Accessoren: `FLOAT` VEC3 Positionen, Indizes `UNSIGNED_BYTE|SHORT|INT`, `byteStride` beachten; ohne Indizes 0…n−1. Nur `mode` 4 (Dreiecke).
  - Verschweißen per `Map` mit Schlüssel aus den drei `Uint32`-Bitmustern (`new Uint32Array(new Float32Array([x,y,z]).buffer)`), dann degenerierte Dreiecke entfernen.
  - `readColliders(buffer)` → `[{ type: 'mesh', surface, vertices, indices, nodes }]` in `COLLIDER_SURFACES`-Reihenfolge; unbekannte Oberfläche → `TypeError`.
  - `colliderDigest(groups)` → `crypto.subtle.digest('SHA-256', bytes)` → Hex. Byte-Layout nach V1.
  - Datei importiert nichts (kein three.js, kein Rapier).
- [ ] **Schritt 4: Physik und Karte auf typisierte Arrays umstellen.** In `minigolf-physics.js` die Mesh-Prüfung ersetzen:

```js
const numbers = value => Array.isArray(value) || value instanceof Float32Array;
const integers = value => Array.isArray(value) || value instanceof Uint32Array;
// … numbers(shape.vertices) && integers(shape.indices), Längen- und Wertebereiche wie bisher
const desc = RAPIER.ColliderDesc.trimesh(Float32Array.from(shape.vertices), Uint32Array.from(shape.indices), RAPIER.TriMeshFlags.FIX_INTERNAL_EDGES)
```

  In `minigolf-minimap.js` `Array.isArray(shape.vertices)` durch dieselbe Prüfung ersetzen.
- [ ] **Schritt 5: Tests laufen lassen:** `node scripts/test-minigolf-collision.mjs` und `npm run test:minigolf` → alles grün.
- [ ] **Schritt 6: Commit** `feat(minigolf): read collision meshes from course GLBs`.

### Aufgabe 2: Mechaniken und Physik

**Dateien:**
- Neu: `ui/js/desktop/apps/minigolf-mechanics.js`
- Ändern: `ui/js/desktop/apps/minigolf-physics.js`
- Test: `scripts/test-minigolf-physics.mjs`

- [ ] **Schritt 1: Tests schreiben** (an `scripts/test-minigolf-physics.mjs` anhängen; Hilfsfunktion `course(extra)` gibt es dort schon). Jeder Fall mit eigener Physikinstanz und `dispose()`:
  1. **Schwerkraft:** Kurs mit `gravity: 4`, Ball 2 m über dem Boden per `setBall`, kleiner Schlag (`0.01`); nach 0,9 s noch nicht gelandet, nach 1,2 s gelandet (Fallzeit √(2·1,84/4) ≈ 0,96 s). Ungültige Schwerkraft (`0`, `-1`, `25`, `NaN`) → `TypeError`.
  2. **Nicht-terminale Ereignisse:** `boost`-Zone quer auf der Bodenplatte, Schlag mit 0,1 durch sie hindurch → genau ein `boost`-Ereignis, danach waagerecht ≥ `speed − 0,3`, Schuss endet später mit `rest` oder `out`, nicht mit `boost`.
  3. **Teleporter:** Scheibe bei x = 2, Ausgang `[10, 0.17, 0]`, Richtung `[1, 0]`, `minSpeed 2`: Ereignis `teleport`, Ball danach bei x ≥ 10, kein zweites Auslösen am Ausgang.
  4. **Kanone:** Zone bei x = 2, Mündung `[2, 1.2, 0]`, `velocity [4, 3, 0]`, `delay 0.6`: Ball liegt 0,5 s nach dem Fang an der Mündung (± 1 mm), danach Ereignis `cannon` genau einmal; zwei Läufe ergeben bitgleiche Ruhepositionen.
  5. **Bumper:** Zylinder Radius 0,3 bei x = 2; Schlag 0,08 frontal → Ereignis `bumper`, waagerechte Geschwindigkeit danach ≥ 0,95 · `kick`, weg vom Bumper.
  6. **Hindernisarten:** `obstaclePose` aus `minigolf-mechanics.js` für `spin` (Winkel = speed·t + phase), `gate` (Plateau bei ±amplitude, wenn `sharp·sin` jenseits ±1), `boulder` (Weg = amplitude·sin, Drehwinkel = Weg/radius um `up × axis`) gegen die Formeln; Kollisionsteile `ball` und `cylinder` werden angelegt (Ball prallt an einem Kugelteil ab).
  7. **Vorschau:** mit Boost-Zone vor dem Ball ist die vorhergesagte Strecke länger als ohne; vor einem Teleporter endet sie mit `zone: 'teleport'`.
  8. **Looping-Prototyp (Machbarkeit):** Funktion `loopCourse()` im Test erzeugt ein Trimesh: 8 m Anlauf (12 Reihen, 1,72 m), Looping Radius 1,3 m, seitlicher Versatz 2,2 m nach Design 4.3 (Formel in Aufgabe 6, Schritt 3), 6 m Auslauf, Banden 0,24 m entlang der Bodennormale; `gravity 4`, Boost-Zone (9 m/s) am Anlauf. Schlag 0,3 → Ball erreicht den Auslauf (x-Versatz ≈ 2,2 m, hinter dem Looping). Ohne Boost und mit Schlag 0,15 → Ball kommt nicht über den Scheitel und endet im Anlauf.
  9. **Naht:** zwei getrennt erzeugte, aber an der Naht bitgleiche flache Streifen (je 4 m, 12 Reihen) als zwei Mesh-Kollider derselben Oberfläche, nach `readColliders`-Verschweißung (oder direkt verschweißt) ein Trimesh; Schlag 0,4 über die Naht: Ball-y weicht nie mehr als 1 cm von 0,16 + Bodenhöhe ab.
- [ ] **Schritt 2: Tests laufen lassen** → Fehlschläge (fehlendes Modul / Felder).
- [ ] **Schritt 3: `minigolf-mechanics.js` implementieren** (reine Funktionen, keine Imports):

```js
const TAU = Math.PI * 2;
export const OBSTACLE_KINDS = Object.freeze(['rotate', 'slide', 'spin', 'gate', 'boulder']);
export const MECHANIC_KINDS = Object.freeze(['bumper', 'boost', 'teleport', 'cannon']);
export const TERMINAL_EVENTS = Object.freeze(new Set(['rest', 'cup', 'water', 'out']));

export function axisAngle(axis, angle) { /* Quaternion [x,y,z,w] */ }
export function multiply(a, b) { /* Quaternion-Produkt a·b */ }

export function obstaclePose(spec, time) {
    const phase = time * spec.speed + (spec.phase || 0);
    const axis = normalize(spec.axis || [0, 1, 0]);
    const base = spec.rotation || [0, 0, 0, 1];
    const at = offset => spec.position.map((value, i) => value + axis[i] * offset);
    switch (spec.kind) {
    case 'rotate': return { position: spec.position, rotation: multiply(axisAngle(axis, spec.amplitude * Math.sin(phase)), base) };
    case 'spin': return { position: spec.position, rotation: multiply(axisAngle(axis, phase), base) };
    case 'slide': return { position: at(spec.amplitude * Math.sin(phase)), rotation: base };
    case 'gate': return { position: at(spec.amplitude * Math.max(-1, Math.min(1, (spec.sharp || 2) * Math.sin(phase)))), rotation: base };
    case 'boulder': {
        const offset = spec.amplitude * Math.sin(phase);
        const roll = normalize([axis[2], 0, -axis[0]]); // up × axis
        return { position: at(offset), rotation: multiply(axisAngle(roll, offset / spec.radius), base) };
    }
    }
}

// Zonen: liefert, ob der Ballmittelpunkt in der Zone liegt.
export function inBox(zone, p) { /* lokal: (p − position) um −yaw drehen, |lokal| ≤ half */ }
export function inDisc(zone, p, height = 0.4) { /* waagerechter Abstand ≤ radius, y in [pos.y − 0.05, pos.y + height] */ }
export const boostVelocity = (zone, v) => { const s = Math.max(zone.speed, Math.hypot(v[0], v[2])); return [zone.direction[0] * s, v[1], zone.direction[1] * s]; };
export const teleportVelocity = (zone, v) => { const s = Math.max(zone.minSpeed || 0, Math.hypot(v[0], v[2])); return [zone.direction[0] * s, 0, zone.direction[1] * s]; };
export function bumperVelocity(bumper, ballPosition, v) {
    let nx = ballPosition[0] - bumper.position[0], nz = ballPosition[2] - bumper.position[2];
    const length = Math.hypot(nx, nz) || 1; nx /= length; nz /= length;
    const along = v[0] * nx + v[2] * nz;
    const add = Math.max(0, bumper.kick - along);
    return [v[0] + nx * add, v[1], v[2] + nz * add];
}
export function validateObstacle(spec) { /* Felder, Arten, Teile box|ball|cylinder; wirft TypeError */ }
export function validateMechanic(spec) { /* Felder je Art; wirft TypeError */ }
```

- [ ] **Schritt 4: `minigolf-physics.js` umbauen:**
  - `obstaclePose`/`rotationAt` durch den Import aus `minigolf-mechanics.js` ersetzen; Validierung über `validateObstacle`.
  - Schwerkraft: `const gravity = course.gravity ?? 9.81;` gültig, wenn endlich, > 0 und ≤ 20; `new RAPIER.World({ x: 0, y: -gravity, z: 0 })`.
  - `WALL_RESTITUTION.metal = 0.80`, `surfaceFriction.metal = 0.5`.
  - Hindernisteile: `shape` `box` (Vorgabe), `ball` (`ColliderDesc.ball(radius)`), `cylinder` (`ColliderDesc.cylinder(halfHeight, radius)`), jeweils mit `position`/`rotation` relativ zum Körper.
  - Bumper: je Mechanik `bumper` ein fester Zylinder (`cylinder(height/2, radius)`, Mitte `position + [0, height/2, 0]`), Restitution 1,0 mit `Max`, Reibung 0, Oberfläche `bumper.surface`; `bumperByHandle` merken.
  - `emit(type, …)`: nur `TERMINAL_EVENTS` beenden den Schlag; andere Typen (`impact`, `bumper`, `boost`, `teleport`, `cannon`) melden nur.
  - Zustand je Schlag: `inside` (Set), `teleportUntil`, `captured` (`{ id, release, muzzle, velocity }`); bei `shoot`, `setBall`, `resetTime`, terminalem Ereignis zurücksetzen.
  - Pro Festschritt nach `world.step` und dem Ereignisabruf: offene Bumper-Kontakte (Kontaktbeginn mit Bumper-Handle) → `bumperVelocity` setzen + Ereignis `bumper`; dann `applyRolling` (nicht während `captured`); dann Zonen in Reihenfolge Kanone, Teleporter, Boost; dann `resolveConditions` (Ruhe-Zähler läuft nicht während `captured` und in den 0,5 s nach einem Teleport).
  - Kanone: beim Fang Körpertyp `Fixed`, Position = `muzzle`, Geschwindigkeiten 0, `resetRenderPose()`; ab `time ≥ release` `Dynamic`, `setLinvel(velocity)`, `setAngvel(0)`, Ereignis `cannon`.
  - Teleporter: `setTranslation(exit)`, `setLinvel(teleportVelocity)`, Drall wie `launch` passend zur neuen Geschwindigkeit, `resetRenderPose()`, `teleportUntil = time + 0.5`, Ereignis `{ type: 'teleport', position: entry, exit }`.
  - `predict()`: dieselben Funktionen auf die Geisterwelt anwenden (Bumper-Stoß, Boost); an Teleporter-/Kanonen-Zonen abbrechen und `zone` mitgeben. Rückgabe `{ points, contactIndex, hazard, zone }`.
  - Datei bleibt unter ≈ 600 Zeilen; wenn nicht, die Zonenschleife als `stepMechanics(…)` nach `minigolf-mechanics.js` auslagern (ohne Rapier-Import: Körper wird als Adapter mit `translation()`, `linvel()`, `setLinvel()`, … übergeben).
- [ ] **Schritt 5: Tests laufen lassen:** `node scripts/test-minigolf-physics.mjs`, danach `npm run test:minigolf` (die alten 18 Bahnen und Routen müssen unverändert grün bleiben).
- [ ] **Schritt 6: Commit** `feat(minigolf): bumpers, boost pads, teleporters, cannon, new obstacle kinds and per-course gravity`.

> Ergebnis des Looping-Prototyps (Fall 8) in der Commit-Nachricht festhalten: Mindestschlag mit Boost, Verhalten ohne Boost. Scheitert der Prototyp, vor Aufgabe 6 Radius/Boost anpassen und im Design 4.4 nachziehen.

### Aufgabe 3: Spiel und Szene

**Dateien:**
- Ändern: `ui/js/desktop/apps/minigolf-scene.js`, `minigolf-game.js`, `minigolf-camera.js`, `minigolf-fx.js`
- Test: `scripts/test-minigolf-look.mjs`

- [ ] **Schritt 1: Test schreiben** in `scripts/test-minigolf-look.mjs`: `overviewFor({ island: { center: [0, 0], half: [4.9, 5.35] }, tee, cup })` ergibt dieselbe Pose wie die heutige feste Übersicht (Ziel = Mitte zwischen Abschlag und Loch, Versatz (10, 13, 14)); bei `half: [16, 9]` ist der Abstand größer, das Ziel ist die Inselmitte (y = Mittel aus Abschlag- und Loch-Höhe), und `shadowHalf` ≥ 17,5. Ohne `island` gilt die alte Pose.
- [ ] **Schritt 2: Test laufen lassen** → Fehler (`overviewFor` fehlt).
- [ ] **Schritt 3: Implementieren:**
  - `minigolf-camera.js`: `export function overviewFor(course)` → `{ position, target, shadowHalf, waterRadius }`. Formel: `span = max(half[0], half[1])`; wenn `span ≤ 5.5`: alte Pose, `shadowHalf 9`; sonst Ziel = `[center[0], (tee[1]+cup[1])/2, center[1]]`, Richtung `normalize([10, 13, 14])`, Abstand `clamp(span · 3.4, 21.6, 60)`, `shadowHalf = max(9, span + 1.5)`, `waterRadius = max(60, span · 5)`.
  - `minigolf-scene.js`:
    - `load(next)`: nach dem Abruf der Bytes `const colliders = next.collision ? readColliders(bytes) : next.colliders;` (Import aus `minigolf-collision.js`), vor `GLTFLoader.parseAsync`. Rückgabe `{ colliders }` statt `true`. Fehler beim Lesen → `throw Error('course_collision')`.
    - `overviewPose()` nutzt `overviewFor(course)`; die Sonnen-Schattenkamera nimmt `shadowHalf` (Grenzen ± shadowHalf, `updateProjectionMatrix`); die offene Wasserscheibe wird mit `waterRadius` skaliert, wenn `createWater` einen Radius annimmt (sonst `scale` der Gruppe).
    - Knoten mit `userData.aurago_spin` (`{ axis: [x,y,z], speed }`) in einer Liste sammeln; in `update()` ohne reduzierte Bewegung `node.rotateOnAxis(axis, speed · dt)`.
    - `resetTrail()` (optional aufrufbar) leert die Leuchtspur über `fx`.
  - `minigolf-fx.js`: Burst-Typ `spark` (Funken aus dem vorhandenen Glühpartikel-Pool, Farbe je Look aus der FX-Palette, 18 Teilchen), Effektgrenzen wie `impact`.
  - `minigolf-game.js`:
    - Manifest: `version` 1 **oder** 2 annehmen (Übergang; Aufgabe 11 macht daraus nur 2). Bei v2 zusätzlich `typeof c.collision?.digest === 'string'`.
    - `loadCourse`: `const loaded = await scene.load(next); if (!loaded || …) return; const playable = { ...next, colliders: loaded.colliders };` → `createPhysics(playable, …)`, `view.hud.map.setCourse(playable)`.
    - `handleEvents`: `boost` → `audio.effect('boost', { speed, pan })`, `scene.burst('spark', position)`; `teleport` → `audio.effect('teleport', { pan })`, `scene.burst('spark', position)`, `scene.burst('spark', event.exit)`, `scene.resetTrail?.()`; `cannon` → `audio.effect('cannon', { pan })`, `scene.burst('spark', position, 2)`; `bumper` → `scene.burst('impact', position, …)` (Klang kommt über `impact`). Diese Ereignisse beenden `busy` nicht.
- [ ] **Schritt 4: Tests:** `npm run test:minigolf`, `node scripts/build-minigolf.mjs` (Bundle neu bauen) und `node scripts/build-minigolf.mjs --check`, `go test ./ui -run 'TestDesktopMinigolf' -count=1`, Browser-Smoke mit `AURAGO_RUN_BROWSER_SMOKE=1` (`TestDesktopMinigolfBrowser`).
- [ ] **Schritt 5: Commit** `feat(minigolf): GLB collision, mechanic events and size-aware framing in the game`.

### Aufgabe 4: Routensuche als Bibliothek

**Dateien:**
- Neu: `scripts/lib/minigolf-routes.mjs`, `scripts/minigolf-route-search.mjs`

- [ ] **Schritt 1: Bibliothek** `scripts/lib/minigolf-routes.mjs` mit:
  - `loadCourses(dir)` → Manifest lesen; bei v2 je Bahn `colliders = readColliders(GLB)`; bei v1 unverändert.
  - `runShot(physics, position, shot, events, { resetTime })` (aus dem heutigen Kurstest übernommen; Abbruch nach 1800 Bildern à 1/60 s, weil Kanonenflüge länger dauern).
  - `progress(course, position)`: Projektion auf die nächstgelegene Polylinie aus `corridors` (sonst `path`) → zurückgelegte Länge entlang der Hauptroute bzw. bei Zweigen die Restlänge bis zum Zweigende plus Restlänge der Hauptroute ab dort; Bewertung `remaining = course.length − progress` (Loch: 0).
  - `findRoute(physics, course, events)`: Strahlsuche wie heute, aber
    - Zielrichtungen je Zustand: Richtung zum Loch, und zu Punkten 2, 4, 7 und 11 m voraus entlang der nächsten Korridorlinie; dazu Winkelversätze `[-60, -30, -15, -7, 0, 7, 15, 30, 60]`;
    - Stärken `[0.15, 0.25, 0.35, 0.45, 0.55, 0.65, 0.75, 0.85, 0.925, 1]`;
    - Sortierung nach `remaining` + 0,8 m je Schlag; Strahlbreite 6; Tiefe ≤ Par + 2;
    - Zellen-Duplikate wie heute (0,28 m).
  - `timedRoute(physics, course, path, events)`: wie heute, aber Takt = `course.cycle` (wenn gesetzt) statt der Einzelperiode.
  - `replayRoute(physics, course, route, events)` → `{ ok, shots, positions }`.
- [ ] **Schritt 2: CLI** `scripts/minigolf-route-search.mjs [--dir assets/minigolf/build/p3] [--course pirate-04] [--json]`: sucht je Bahn, prüft die gefundene Route mit Wiederholung, gibt Schläge, Route (JSON) und Laufzeit aus. Ohne Treffer: beste Endlage und `remaining` je Tiefe ausgeben (Hilfe beim Abstimmen).
- [ ] **Schritt 3: Prüfen** gegen die heutigen Bahnen: `node scripts/minigolf-route-search.mjs --dir ui/3d/minigolf --course temple-06`, Erwartung: Route gefunden (v1-Manifest; ohne `corridors` greift `path`).
- [ ] **Schritt 4: Commit** `test(minigolf): reusable route search with corridor progress`.

**Prüfung Stufe A (test-agent):** Diff von Aufgabe 1–4 gegen Design 5–7 und die Verträge V1/V3; `npm run test:minigolf`, Bundle-Check, `TestDesktopMinigolf*`, Browser-Smoke. Nur kritische/wichtige Befunde beheben.

---

## Stufe B – Generator

Gemeinsame Konstanten in `track.py`: `ROWS = 12`, `LANE_WIDTH = 1.72`, `SPACING = LANE_WIDTH / ROWS`, `WALL_HEIGHT = 0.24`, `WALL_DEPTH = 0.045` (unter Bodenhöhe), `WALL_THICKNESS = 0.14`, `CUP_RADIUS = 0.32`, `BALL_RADIUS = 0.16`, `MAX_STEP = 0.30` (Querschnittabstand in Bögen), `GROUNDED_MAX = 0.30`.

Blender-Koordinaten: X/Y waagerecht, Z oben, Meter. Laufzeit = `(x, z, −y)` wie bisher (`runtime()`), Funktionen dafür ziehen von `courses_v1.py` nach `basic_parts.py`.

### Aufgabe 5: Baukasten-Kern

**Dateien:**
- Neu: `assets/minigolf/kit/track.py`, `pieces.py`, `pads.py`, `layout.py`, `build_track.py`, `basic_parts.py`, `track_selftest.py`
- Ändern: `assets/minigolf/kit/export.py`

- [ ] **Schritt 1: `track.py` – Datentypen** (reine Python-Klassen, ohne bpy):

```python
class Cursor:   # Endzustand eines Stücks
    __slots__ = ("x", "y", "z", "heading")        # heading in Radiant, 0 = +X
    def forward(self): return (math.cos(self.heading), math.sin(self.heading))
    def left(self): return (-math.sin(self.heading), math.cos(self.heading))

class Section:
    """Querschnitt: ROWS+1 Punkte von links nach rechts, Bodennormale, Bandenflags."""
    __slots__ = ("points", "center", "heading", "normal", "walls", "wall_up", "surface", "look", "grounded", "flat")
    # walls = (left: bool, right: bool); wall_up = (left_vec, right_vec) oder None (dann normal)

class Strip:      # Folge von Sections eines Stücks (erste Section = letzte des Vorgängers)
    __slots__ = ("sections", "kind", "cover", "deck")
class Pad:        # Rasterfläche mit Anschlüssen
    __slots__ = ("kind", "grid", "faces", "surface", "look", "walls", "ports", "cup", "holes")
class Track:
    __slots__ = ("strips", "pads", "tee", "cup", "path", "corridors", "hazards", "anchors", "length", "mechanics", "obstacles", "cycle", "gravity")
```

  Regeln: `make_section(center, heading, z_profile, bank=0, width=LANE_WIDTH)` erzeugt Punkte `center + left·a·cos(b) + up·(−a·sin(b))` für `a = width/2 − i·SPACING`; `flat` ist wahr bei Bank 0 und Normale (0,0,1). Ein Stück erhält `prev` (Section) und gibt neue Sections ohne `prev` zurück; `Strip.sections = [prev] + neu`. Punkte sind Tupel; der nächste Strip verwendet dasselbe `prev`-Objekt (keine Neuberechnung).
- [ ] **Schritt 2: `pieces.py`:** `straight`, `curve`, `bank`, `ramp`, `hill`, `steps`, `gap` mit Signatur `piece(prev, cursor, **params) -> (sections, cursor, extras)`; `extras` trägt z. B. Gefahrenvolumen (`gap`) oder Anker. Formeln:
  - `curve`: Mittelpunkt links (Winkel > 0) oder rechts; `n = ceil(|angle|·radius / MAX_STEP)` Schritte.
  - `bank`: Neigung `b(s) = bank · sin²(π s)`, äußere Kante oben.
  - `ramp`: `z(s) = rise · (3s² − 2s³)`; mit `lip` (Grad) ersetzt das letzte Fünftel durch eine Gerade mit Steigung `tan(lip)`, stetig angeschlossen.
  - `hill`: `z(s) = height · (1 − cos 2πs) / 2`, Querschnitte alle ≤ 0,30 m.
  - `steps`: `count` Absätze à `tread` m, je Stufe zwei Sections an derselben XY-Stelle (oben, unten = −`drop`), die Setzstufe wird ein senkrechtes Quad im Bodennetz.
  - `gap`: beendet den Strip; der nächste Strip beginnt nach `length` mit einer neu berechneten, flachen Section; `extras.hazard = {kind, rect}` mit dem Rechteck unter der Lücke (± Breite/2 + 0,3 m).
- [ ] **Schritt 3: `pads.py`:** `tee`, `green`, `plaza`. Raster im Pad-Rahmen (Ursprung = Mitte der Eingangs-Section, x vorwärts, y links); quer `SPACING`, längs ≤ 0,25 m. Breite muss ein Vielfaches von `SPACING` sein, sonst `ValueError`. Eingangsanschluss: Rasterpunkte der hinteren Kante in den Spalten des Anschlusses sind **die Punktobjekte der Eingangs-Section**; Pads akzeptieren nur `flat`-Eingänge. Ausgangsanschlüsse liefern eine neue Section aus den Rasterpunkten (als `prev` des nächsten Stücks). `green`: Flächen mit Mittelpunkt im Abstand < `CUP_RADIUS + 0.13` vom Loch entfallen; optional `slope` (Grad längs) und `contour` (Liste von Höckern `(x, y, r, h)`). `plaza`: `pools` als Polygone; Flächen mit Mittelpunkt im Polygon entfallen; je Pool ein Gefahrenvolumen (`water` oder `void`). Banden an allen Randkanten außer Anschlüssen; Pool-Ränder ohne Bande.
- [ ] **Schritt 4: `layout.py`:** `build(course_data, world) -> Track`. Liest `start`, läuft die Stückliste ab, verwaltet Anker, sammelt `path` (Mittelpunkte der Hauptroute), `corridors` (je Route eine Polylinie), `length` (Länge der Hauptroute vom Abschlag bis zum Loch), Gefahren, Mechaniken, Hindernisse, `cycle`, `gravity` (Welttabelle `{pirate: 9.81, temple: 9.81, moon: 4.0}`). Prüft die Regeln aus Design 4.4 (außer Insel/Bounds, die prüft Aufgabe 7) und wirft `ValueError` mit Bahnnummer und Stückindex.
- [ ] **Schritt 5: `build_track.py`:** erzeugt aus einem `Track` Blender-Objekte:
  - Bodenknoten je Optik `floor-<look>` (Vertex-Verschmelzung über ein `dict` mit dem exakten Tupel als Schlüssel), Custom Property `aurago_collider` = Oberfläche, Material aus der Weltpalette (Felt-Punktfarben wie P2 `Dresser._paint_lane`, verallgemeinert). Ein Knoten je (Optik, Oberfläche)-Paar, wenn beide variieren (`floor-felt`, `floor-sand`, `floor-deck` …).
  - Bandenläufe: aus Sections und Pad-Rändern zusammenhängende Läufe `[(inner_point, outward, up)]`; Bandenknoten `rail-<surface>` mit `aurago_collider` und `aurago_hidden`. Querschnitt wie P1 (innen unten `−WALL_DEPTH`, innen oben `+WALL_HEIGHT`, außen um `WALL_THICKNESS`), Endkappen an Laufenden. Die Läufe werden auch an `trim.py` gegeben (Aufgabe 7).
  - Alle Kollisionsobjekte mit Einheits-Transformation.
  - `basic_parts.py`: aus `courses_v1.py` übernommen (`runtime`, `blender`, `q`, `material`, `mesh_object`, `ring`, `box_mesh`, `cylinder`, `ico_sphere`, `cup_lining`, `hole_number`, `add_templates`), unverändert im Verhalten.
- [ ] **Schritt 6: `export.py` erweitern:** `glb_collision(path)` liest wie `glb_fixed_colliders` alle Knoten mit `extras.aurago_collider`, verschweißt und berechnet Digest nach V1 (Float32 per `struct.pack('<f')`, Uint32 LE) → `{"groups": [...], "nodes": n, "triangles": t, "digest": hex}`. `canonical_export_names` bekommt die neuen Funktionsnamen (`floor-*`, `rail-wood|stone|metal`, `obstacle-*`, `mechanic-*`) über Präfixe statt fester Liste; `FUNCTIONAL_PREFIXES` um `floor-`, `mechanic-` ergänzen.
- [ ] **Schritt 7: Selbsttest** `track_selftest.py` (headless): baut eine Probebahn aus `tee, straight 3, curve r3 90°, bank r4 −60° 20°, ramp 3 +0.4, hill 3 −0.25, steps 3×0.2, straight 2, green` und prüft
  - jede Stücknaht: die Punkt-Tupel von `prev` und erster Section sind identische Objekte;
  - exportiert nach `--out` (Vorgabe Scratch) als GLB; `glb_collision` liefert Gruppen `grass` und die Weltbande; kein Bodendreieck im Lochradius;
  - schreibt `selftest.json` mit Digest.
  Danach im Repo: `node -e "import('./ui/js/desktop/apps/minigolf-collision.js').then(async m=>{const fs=await import('node:fs');const b=fs.readFileSync(process.argv[1]);const g=m.readColliders(b.buffer.slice(b.byteOffset,b.byteOffset+b.byteLength));console.log(await m.colliderDigest(g))})" <glb>` → gleicher Digest wie `selftest.json`.
- [ ] **Schritt 8: Ausführen:** `& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/minigolf/kit/track_selftest.py -- --out disposable/_p3/selftest` → „selftest ok“; Digest-Vergleich gleich.
- [ ] **Schritt 9: Commit** `feat(minigolf): track kit core with shared seams, pads and GLB collision digest`.

### Aufgabe 6: Mondstücke, Trichter, Gabelung, Sprung

**Dateien:**
- Neu: `assets/minigolf/kit/pieces_moon.py`, `pads_funnel.py`
- Ändern: `pads.py` (Gabelung), `layout.py`, `track_selftest.py`

- [ ] **Schritt 1: Halfpipe** (`pieces_moon.halfpipe`): U-Profil aus linkem Viertelkreis (Radius `radius`), flachem Boden (`width`), rechtem Viertelkreis; die `ROWS+1` Punkte liegen in gleichen Bogenlängenabständen auf dem Profil. Übergang über `blend ≥ 1.5` m: Punkt = `lerp(flach, U, smoothstep(s/blend))`, am Ende umgekehrt. Mittellinie gerade (`length`) oder Bogen (`radius_path`, `angle`). Banden auf den Lippen mit `wall_up = (0, 0, 1)` (senkrecht), `outward` waagerecht nach außen.
- [ ] **Schritt 2: Looping** (`pieces_moon.loop`): Mittellinie für θ ∈ [0, 2π], `s = θ/2π`:

```python
p(θ) = start + f·(R·sin θ) + up·(R·(1 − cos θ)) + side·(offset·(3s² − 2s³))
T = normalize(dp/dθ);  across = normalize(side − (side·T)·T);  n = normalize(T × across) mit n(0) = +Z
```

  Mindestens 48 Querschnitte; erste Section = `prev` (muss `flat` sein, Richtung = Looping-Richtung), letzte Section flach mit seitlichem Versatz `offset`. Banden mit `wall_up = n`. Lichtraum-Regel für den Looping: nur gegen fremde Geometrie prüfen.
- [ ] **Schritt 3: Trichter** (`pads_funnel.funnel`): Polares Netz um `cup`; Höhe `z(r) = z_cup + depth · (r/R)^1.5`, Rand `z(R)` = Höhe der Eingangs-Section. Äußerer Ring: die `ROWS+1` Punkte der Eingangs-Section (exakte Objekte) plus gleichmäßig verteilte Randpunkte über den übrigen Bogen (Winkelabstand ≈ wie der Eingang); innere Ringe regelmäßig bei `r_i`, innerster Ring bei `CUP_RADIUS + 0.13` (Loch offen). Bänder zwischen Ringen mit unterschiedlicher Punktzahl per Reißverschluss (Punkte nach Winkel zusammenführen). Bande am äußeren Ring außer im Eingangsbogen. Die Eingangs-Section muss vollständig im Abstand `[R − 0.25, R]` liegen, sonst `ValueError`.
- [ ] **Schritt 4: Gabelung** (`pads.fork`): Teilungsfläche (wie `plaza`, Länge ≥ 1,5 m) mit zwei Ausgängen im Querabstand `spread` (Vielfaches von `SPACING`); die Zweige `left`/`right` werden mit `layout` rekursiv gebaut; die Zusammenführung entsteht aus beiden Zweigenden (Prüfung: Richtung ± 0,01°, Höhe ± 1 mm, gleiche Vorwärtskoordinate ± 1 mm, Querabstand Vielfaches von `SPACING`), ihre hintere Rasterzeile = linke Section + Zwischenpunkte + rechte Section, Ausgang mittig (oder `exit_offset`). Hauptroute über `main: 'left'|'right'`; `corridors` erhält beide Zweige.
- [ ] **Schritt 5: Sprung**: `ramp` mit `lip` + `gap` + Landefläche ist in `pieces.py` schon möglich; im Selbsttest eine Sprungstrecke ergänzen (Rampe 2 m auf 0,35 m, Lippe 20°, Lücke 1,6 m `water`, Landung flach).
- [ ] **Schritt 6: Selbsttest erweitern:** zweite Probebahn (Mond) mit `halfpipe`, `loop` (R 1,3, Versatz 2,2), `fork`, `funnel`; dieselben Naht- und Digestprüfungen. Zusätzlich: Probebahn in die Physik laden (Node, mit `readColliders`) und einen festen Schlag pro Stück testen: Looping mit Boost-Zone (aus `track.mechanics`) wird durchlaufen; Trichter: ein Schlag mit 0,35 aus 2 m vor dem Rand endet im Loch oder am Lochrand (≤ 0,6 m). Dafür ein kleines Node-Skript `disposable/_p3/probe-physics.mjs` (nicht committen).
- [ ] **Schritt 7: Commit** `feat(minigolf): half-pipe, loop, funnel, fork and jump pieces`.

### Aufgabe 7: Bahn-Pipeline, Diorama, Manifest v2

**Dateien:**
- Neu: `assets/minigolf/kit/courses.py`
- Ändern: `assets/minigolf/kit/dress.py`, `dress_island.py`, `trim.py`, `placement.py`, `dress_parts.py`, `assets/minigolf/generate.py`

- [ ] **Schritt 1: `generate.py`:** Option `--out DIR` (Vorgabe `ui/3d/minigolf`); Kits werden ebenfalls dorthin gebaut. Neue Option `--set p3` (Vorgabe bis zur Umstellung `v1`): mit `p3` baut `courses.py` aus den Datenmodulen und schreibt Manifest v2; `--course N` patcht den Eintrag im Manifest unter `--out`. Mit `--set p3` und ohne `--out` bricht der Lauf ab, bis Aufgabe 11 die Vorgabe umstellt.
- [ ] **Schritt 2: `courses.py`:** `build_course(number, data, world, out, keep_scene=False) -> entry`:
  1. `layout.build` → `Track`;
  2. neue Szene, `build_track` (Boden, Banden), Abschlag-Pad-Marke, Loch, Lochboden (aus `terrain.top_height`), Fahne, Bahnnummer, Vorlagen über `basic_parts`;
  3. `Dresser(number, track, world)`: Insel, Bandenverkleidung, Mechanik- und Hindernisteile (Aufgabe 8), Platzierung, `restyle`, Bake;
  4. `export.join_scenery`, Export nach `out/course-NN.glb`, `glb_collision`;
  5. Eintrag nach V2: `bounds` = Inselumriss ± 1,5 m (xz), y von `min(Wasserspiegel − 0.4, tiefster Boden − 4.0)` bis `max(höchster Boden, höchster Kanonenbogen) + 6.0`; `water` = Meer (Piraten/Tempel) plus Teich- und Lückenvolumen; Mond nur Teiche (keine).
- [ ] **Schritt 3: `dress_island.Terrain` verallgemeinern:** statt einer Spurpolylinie eine Liste geerdeter Bodendreiecke (2D-Raster-Hash, Zelle 1 m). `top_height(x, y)` = Basisgrund, darunter ein Bett 6 cm unter dem nächstgelegenen geerdeten Boden (Glättung 1,1–2,0 m wie P2); Mulden unter Teichen (Tiefe 0,45 m unter der Wasserfläche) und Gräben/Schluchten unter `gap`-Gefahren (Piraten/Tempel bis unter die Wasserfläche, Mond durch die Insel). Erhöhte Stücke (`grounded` falsch) beeinflussen den Grund nicht.
- [ ] **Schritt 4: Insel nach Layout:** `Outline(center, half, power=3.0, wobble=0.035)` mit `center`/`half` aus der Grundfläche (alle Boden-Punkte, Pads, Hingucker-Kreis, feste Requisiten) plus Rand 2,5 m (Halbachse höchstens 20 m). Rastergröße der Insel `nx = ceil(2·half_x / 0.35)`, entsprechend `ny`.
- [ ] **Schritt 5: `trim.py` verallgemeinern:** `build_trims(world, runs, …)` nimmt die Bandenläufe aus `build_track` (statt `courses_v1.rail_edge`). Verkleidung bündig an der Bandeninnenseite, höchstens 2 cm über und 3 cm außen über der Bande, folgt `wall_up` (Looping, Halfpipe).
- [ ] **Schritt 6: Stützen und Brücken:** unter Sections mit `grounded` falsch alle ≈ 2 m Stützen im Weltstil (Holzpfahl, Steinsäule, Metallstrebe) vom Boden-Unterrand bis `top_height`; `deck: bridge` bekommt Geländerpfosten an den Bandenläufen (außerhalb des Lichtraums). `cover: tunnel` bekommt Bögen (Piraten: Totenkopffelsen als Portal, Tempel: Steinkopf-Portal) mit Öffnung ≥ 1,9 m × 1,0 m.
- [ ] **Schritt 7: Platzierung:** `placement.Field` nimmt `corridors` (Liste von Polylinien) statt `path`; Korridorregel 1,2 m gegen alle. Neuer Ablauf: feste Teile → Hingucker an `data["hero"]` mit Maßstab 1,0 (bei Konflikt mit Rand oder Korridor in 0,05er-Schritten bis 0,9, darunter Abbruch) → `data["props"]` → Rand → Streudeko. `LIMIT` 220. `placement.check` wie P2, gegen alle Korridore.
- [ ] **Schritt 8: Wasserflächen:** Alle Teiche einer Bahn sind Objekte `pond-water` mit dem Kit-Material der Art `water`; `join_scenery` fasst sie zu einem Knoten `scenery-water` zusammen. Wasserfläche 0,25 m unter dem umgebenden Boden, Gefahrenvolumen von der Muldensohle bis 0,02 m unter die Wasserfläche.
- [ ] **Schritt 9: Probe:** eine einfache Datenbahn (Kopie der Selbsttest-Bahn, Welt `pirate`, Hingucker gesetzt) in `course_data_pirate.py` als Platzhalter Nr. 1 eintragen; `--set p3 --out assets/minigolf/build/p3 --course 1` baut sie; `node scripts/minigolf-route-search.mjs --dir assets/minigolf/build/p3 --course pirate-01` findet eine Route; Cycles-Probebild mit `--save-blend` und einem Scratch-Render ansehen (Insel, Verkleidung, Hingucker in voller Größe).
- [ ] **Schritt 10: Commit** `feat(minigolf): P3 course pipeline with layout-sized dioramas and manifest v2`.

### Aufgabe 8: Mechanik- und Hindernisteile

**Dateien:**
- Neu: `assets/minigolf/kit/mechanic_parts.py`, `obstacle_parts.py`
- Ändern: `courses.py`, `dress.py`

- [ ] **Schritt 1:** `mechanic_parts.build(world, mechanic) -> [objects]` für die Tabelle in Design 4.6: Bumper (sichtbarer Kern = Zylinder mit genau `radius`/`height`), Boost-Platte (Material `glow`, Pfeil-Relief, bündig 5 mm über dem Boden, außerhalb der Kollision), Teleporter (Eingang/Ausgang, Ring als Knoten `mechanic-<id>-ring` mit `aurago_spin`), Kanone (großes Rohr, Fangtrichter am Verschluss über der Zone, Mündung bei `muzzle`). Statische Teile werden Szenerie (Bake, Zusammenfassen).
- [ ] **Schritt 2:** Kanonenbahn berechnen: `velocity = (target − muzzle − ½·g·T²) / T` (Laufzeit-Koordinaten, `g = [0, −gravity, 0]`), `T` aus den Daten (Vorgabe 1,4 s); prüfen, dass der Bogen (20 Stützpunkte) 0,5 m Abstand zu fremder Geometrie und innerhalb von `bounds` liegt.
- [ ] **Schritt 3:** `obstacle_parts.build(world, obstacle) -> object` als Knoten `obstacle-<id>` mit Ursprung im Drehpunkt/Mittelpunkt; Kinder nur Deko innerhalb der Kollisionsform; Kollisionsteile im Manifest aus denselben Maßen. Arten: Ankerpendel (Kette als Deko, Anker-Kern = Quader), Baumstamm-Pendel (Zylinder quer), Zugbrücke/Steintür/Schotttor (`gate`, Quader), Rollstein (`boulder`, Kugel), Radararm (`spin`, Quader).
- [ ] **Schritt 4:** In der Probebahn je Welt ein Beispiel (Bumper, Boost, Teleporter, Kanone, Pendel, Tor, Rollstein, Arm) bauen; Physik-Probe mit festen Schüssen (Kanone landet auf dem Ziel ± 0,5 m, Teleporter versetzt, Boost meldet). Probebild ansehen.
- [ ] **Schritt 5: Commit** `feat(minigolf): mechanic and obstacle parts per world`.

**Prüfung Stufe B (test-agent):** Selbsttests, Digest-Gleichheit Python/JS, Determinismus (`--course 1` zweimal → gleiche GLB- und Manifest-Hashes), Naht- und Lichtraumregeln, Modulgrößen. Nur kritische/wichtige Befunde.

---

## Stufe C – Die 18 Bahnen

### Aufgabe 9: Bahnen bauen und abstimmen (je Welt eine Teilaufgabe: 9a Piraten, 9b Tempel, 9c Mond)

**Dateien:** `assets/minigolf/kit/course_data_<welt>.py` (und Korrekturen im Generator, falls ein Stück fehlt; diese getrennt committen).

Für jede Bahn der Welt (Tabelle in Design 3):

- [ ] **Schritt 1: Daten anlegen** mit Stückfolge, Hingucker (`hero`), festen Requisiten (inklusive der bisher ungenutzten Kit-Requisiten der Welt), Mechaniken, Hindernissen und `cycle`. Laufweg und Par gemäß Tabelle.
- [ ] **Schritt 2: Bauen:** `& 'D:\Blender 5.2\blender.exe' --background --factory-startup --python assets/minigolf/generate.py -- --set p3 --out assets/minigolf/build/p3 --course N` (beim ersten Mal ohne `--course` mit `--only kits,courses`).
- [ ] **Schritt 3: Lösbarkeit:** `node scripts/minigolf-route-search.mjs --dir assets/minigolf/build/p3 --course <id>`. Ziel: Route mit ≤ Par + 1 Schlägen; Einführungsbahnen (1, 7, 13) mit ≤ Par. Abkürzung mit festen Testschüssen gegenprüfen (sie muss mit einem bestimmten Schuss klappen und bei falscher Stärke ins Wasser/den Abgrund führen).
- [ ] **Schritt 4: Sichtprüfung:** Szene mit `--save-blend` öffnen und einen Scratch-Render (Cycles, 64 Samples, 1280×720, Kamera wie das Spiel in Übersichtspose) ansehen. Prüfen: Hingucker in voller Größe, nichts verdeckt die Spur, Verkleidung bündig, Stützen unter erhöhten Stücken, Wasser/Abgrund lesbar.
- [ ] **Schritt 5:** Abstimmen bis Schritt 3 und 4 passen; Tabelle im Design (Laufweg, Par, Stücke) nachziehen, falls abweichend.
- [ ] **Schritt 6: Commit je Bahn** `feat(minigolf): <id> <englischer Name>`.

Abschluss je Welt: alle sechs Bahnen gebaut, Suche für alle sechs grün, Budgets (Bahn ≤ 1,25 MiB, ≤ 36 000 Dreiecke, ≤ 32 Mesh-Knoten) eingehalten. Die Welten nacheinander (9a → 9b → 9c), weil Generator-Korrekturen sonst kollidieren.

**Prüfung Stufe C (test-agent):** je Welt Sichtprüfung von zwei Renders, Routen, Budgets, Schwierigkeitsverlauf, jedes Kit-Requisit platziert.

---

## Stufe D – Umstellung und Abschluss

### Aufgabe 10: Namen in 16 Sprachen

**Dateien:** `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`

- [ ] **Schritt 1:** `minigolf.pirate`, `minigolf.temple`, `minigolf.moon` mit den heutigen Werten von `minigolf.coast`, `minigolf.garden`, `minigolf.night` anlegen; `minigolf.course_1` … `course_18` mit den neuen Namen (en/de aus Design 3; übrige Sprachen echt übersetzt, kein Englisch; Deutsch mit echten Umlauten; Spanisch ohne `usted`-Formen).
- [ ] **Schritt 2:** Alte Schlüssel `minigolf.garden|coast|night` entfernen, nachdem `rg "minigolf\.(garden|coast|night)" ui` keine Verwendung mehr zeigt (Menüs nutzen `minigolf.<worldId>`).
- [ ] **Schritt 3:** `go test ./ui -run 'TestFrontend_StaticI18nKeysExistInEnglishBundle|I18N|Translations' -count=1` grün; JSON mit `python -m json.tool` prüfen, BOM-frei, LF.
- [ ] **Schritt 4: Commit** `feat(minigolf): course and world names for the new courses in 16 locales`.

### Aufgabe 11: Umstellung

**Dateien:** `assets/minigolf/generate.py`, `ui/3d/minigolf/*`, `scripts/test-minigolf-courses.mjs`, `scripts/test-minigolf-assets.mjs`, `ui/js/desktop/apps/minigolf-{game,looks,audio,menus}.js`, `ui/desktop_minigolf_browser_test.go` (falls Welt-IDs/Bahn-IDs fest verdrahtet sind), `assets/minigolf/kit/previews.py`; Löschen: `assets/minigolf/kit/courses_v1.py`, nicht mehr genutzte `dress_specials.py`/`special_parts.py`, `scripts/fixtures/minigolf-p1-colliders.json`.

- [ ] **Schritt 1:** `generate.py`: Vorgabe `--set p3`, `--out ui/3d/minigolf`; Option `--set v1` und der Altcode entfallen. Voller Lauf `-- --only kits,courses` (Wiederholen bei Dateisperre bis „Wrote 18 courses“); zweiter voller Lauf → identische Hashes (Determinismus).
- [ ] **Schritt 2:** `previews.py`: Vorschaubahnen `pirate-06`, `temple-09`, `moon-18` (oder die schönste Bahn je Welt), `-- --only previews`.
- [ ] **Schritt 3: `scripts/test-minigolf-courses.mjs`** auf `scripts/lib/minigolf-routes.mjs` umstellen: Manifest v2, 18 Bahnen (6/6/6, IDs `pirate-01` … `moon-18`), Kollision aus dem GLB, Digest gleich Manifest, `ROUTES` mit den in Aufgabe 9 aufgezeichneten Routen (`AURAGO_MINIGOLF_SEARCH=1` sucht neu und gibt die Routen aus; `AURAGO_MINIGOLF_COURSE=<id>` beschränkt), je Bahn Route ≤ Par + 1, Hindernisse bewegen sich, Abschlagschuss berührt `grass`.
- [ ] **Schritt 4: `scripts/test-minigolf-assets.mjs`** auf v2 umbauen (Prüfliste in Design 9): Budgets (Bahn ≤ 1,25 MiB, ≤ 36 000 Dreiecke, ≤ 32 Mesh-Knoten; Manifest ≤ 256 KB; gesamt ≤ 32 MiB), Digest, sichtbare `floor-*`/unsichtbare `rail-*`, offenes Loch, Abschlag über Boden, `path`/`corridors`, Platzierungen gegen `island` und alle `corridors`, Hingucker ≥ 0,9, jedes Kit-Requisit ≥ 1×, Mechaniken/Hindernisse in `bounds`, `cycle` ganzzahlig, keine nummerierten Namen, Vorschaubilder, glTF-Validator wie bisher. P1-Fingerabdruck-Prüfung und Fixture entfernen.
- [ ] **Schritt 5: Laufzeit:** Spiel nimmt nur noch `version === 2` an; `LEGACY_LOOK` in `minigolf-looks.js`, `WORLD_ALIAS` in `minigolf-audio.js` (Export und Nutzung; Audiotests anpassen), Altwelt-Einträge in `WORLD_ICONS` und den Akzent-Tabellen sowie der v1-Zweig in `scene.load` entfallen. Bundle neu bauen.
- [ ] **Schritt 6: Browser-Test** an Bahn-/Welt-IDs anpassen (eine Bahn je Welt laden, `graphics.look` = Welt, Kollision vorhanden, keine externen Anfragen).
- [ ] **Schritt 7: Prüfen:** `npm run test:minigolf`, `node scripts/build-minigolf.mjs --check`, `npm run build:ui -- --check`, `go test ./ui -run 'TestDesktopMinigolf|TestShippedWebAssetsLoadNothingExternal|TestFrontend_StaticI18nKeysExistInEnglishBundle' -count=1`, `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopMinigolfBrowser -count=1`, optional `AURAGO_MINIGOLF_BENCHMARK=1` (Bildrate gegenüber P2).
- [ ] **Schritt 8: Commits** getrennt: Assets (`feat(minigolf): 18 new P3 courses and manifest v2`), Tests, Laufzeit-Aufräumen.

### Aufgabe 12: Doku, Gesamtprüfung, Landung

- [ ] **Schritt 1 (docs-agent):** `documentation/minigolf.md` (Welten, 18 Bahnen, Mechaniken, Budgets, Tests), `assets/minigolf/README.md` (Modulkarte, Befehle, Größen), `assets/minigolf/kit/AGENTS.md` (Baukasten-Verträge: Sections teilen, Pads, Kollisionsknoten, Digest, Datenmodule, Platzierung mit Hingucker), `ui/js/desktop/apps/AGENTS.md` (AuraGolf: Manifest v2, `minigolf-collision.js`, `minigolf-mechanics.js`, Ereignisse, Rahmung, Welt-IDs), Root `AGENTS.md` Kind-Index-Zeile `assets/minigolf/kit/AGENTS.md`. Design-Status auf „umgesetzt“.
- [ ] **Schritt 2:** GitNexus `detect_changes` (Scope all) und Prüfung des gestageten Diffs (`git diff --cached`, `--check`, keine Geheimnisse).
- [ ] **Schritt 3:** Gesamtprüfung (test-agent) nur auf kritische/wichtige Befunde; beheben.
- [ ] **Schritt 4: Landung:** Branch auf aktuelles `main` rebasen (`git rebase main` im Worktree; Konflikte in Konfliktzonen klein lösen), Tests aus Aufgabe 11 Schritt 7 erneut, dann im Haupt-Checkout `git merge --ff-only claude/hungry-cerf-e0e587`. Vorher im Haupt-Checkout prüfen, ob Root `AGENTS.md` als assume-unchanged markiert ist und lokale Änderungen hat; dann die Root-Zeile in einem eigenen Commit erst nach Rücksprache landen oder als offenen Punkt melden. Nicht pushen.
- [ ] **Schritt 5:** Übergabe-Notiz `p3-courses-interfaces.md` im gemeinsamen Notizordner; Memory-Eintrag aktualisieren.

---

## Selbstprüfung des Plans

- Design 1–2: Aufgaben 2, 3, 5–11. Design 3 (18 Bahnen): Aufgabe 9, Namen Aufgabe 10. Design 4 (Baukasten): Aufgaben 5–8. Design 5 (Physik): Aufgabe 2. Design 6 (Manifest v2): Aufgaben 1, 7, 11. Design 7 (Spiel/Szene/Texte): Aufgaben 3, 10, 11. Design 8 (Budgets): Aufgaben 9, 11. Design 9 (Tests): Aufgaben 1–4, 11. Design 10 (Risiken): Looping-Prototyp in Aufgabe 2, Naht-Test, Determinismus in Stufe B.
- Namen konsistent: `readColliders`, `colliderDigest`, `obstaclePose`, `overviewFor`, `glb_collision`, `build_course`, `layout.build`, `Section`/`Strip`/`Pad`/`Track`, `aurago_collider`, `aurago_spin`, `corridors`, `cycle`, `gravity`.
