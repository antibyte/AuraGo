# AuraGolf P1 – Schlag & Physik (Design)

- **Datum:** 2026-10-18
- **Status:** Entwurf zur Durchsicht
- **Bezug:** [Fahrplan](2026-10-18-overhaul-roadmap-design.md), Abschnitt 4.2
- **Umsetzungsplan:** [P1-Plan](2026-10-18-p1-schlag-physik-plan.md)

P1 macht den Schlag kräftig und berechenbar, ohne Bahnen, Look, HUD-Gestaltung oder Sound anzufassen. Es wird auf den bestehenden 18 Bahnen ausgeliefert. Alle Zahlen unten sind mit einem Prototyp gegen die echte Rapier-Engine gemessen (Abschnitt 6).

## 1. Ergebnis für Spielende

- Volle Stärke rollt auf ebener Bahn 28 m und liegt nach gut 5 s still. Ein kurzer Zug ergibt Putts ab etwa 0,1 m.
- Die Stärke hängt nur davon ab, wie weit man **auf dem Bildschirm** zieht, nicht vom Kameraabstand.
- Banden federn: Ein Bandenschlag ist ein echtes Spielmittel.
- Beim Zielen zeigt eine gestrichelte Laufbahn den Weg bis zur ersten Bande und ein kurzes Stück danach. Neben der Stärke steht die geschätzte Rollweite auf ebener Bahn („54 % · ≈ 11 m“).
- Zurückziehen bis zum Ball, Rechtsklick oder Escape während des Ziehens bricht den Schlag ab.

Folge für die alten Bahnen: Die meisten werden mit einem Schlag lösbar (gemessen: 14 von 18 als Hole-in-One). Das ist bis zum Austausch der Bahnen in P3 bewusst akzeptiert.

## 2. Physikmodell (`ui/js/desktop/apps/minigolf-physics.js`)

### 2.1 Rollwiderstand statt Luftbremse

Der Ball rollt mit einer Verzögerung aus konstantem Anteil `a` (Rollwiderstand) und geschwindigkeitsabhängigem Anteil `k` (Filz-/Rasenbremse):

```
dv/dt = −(a · n_y + k · v)          (nur bei Bodenkontakt, n = Bodennormale)
Rollweite d(v0) = v0/k − a/k² · ln(1 + k·v0/a)
```

| Untergrund | a (m/s²) | k (1/s) | Hinweis |
|---|---|---|---|
| `grass` (Filz/Rasen) | 0,8 | 0,45 | Referenz für alle Distanzangaben |
| `sand` | 2,6 | 1,3 | rollt ca. ein Drittel so weit |
| `wood` (als Boden) | 0,7 | 0,42 | heute nur Banden; für Stege/Brücken in P3 |
| `stone` (als Boden) | 0,65 | 0,40 | heute nur Banden; für P3 |

- **Bodenerkennung:** Pro Physikschritt ein Ray vom Ballmittelpunkt nach unten, Länge `BALL_RADIUS + 0,04`, Ball ausgeschlossen. Ein Treffer mit `normal.y ≥ 0,55` zählt als Boden; die Oberfläche kommt aus der Kollider-Zuordnung.
- **Anwendung:** Die tangentiale Geschwindigkeit `s` wird auf `max(0, s − (a·n_y + k·s)·Δt)` gesetzt, die Normalkomponente bleibt. Der Drall wird auf Rollen ohne Rutschen gesetzt: `ω = (n × v_t) / r`.
- **Hänge:** Die Schwerkraft wirkt unverändert. Bis zu einer Neigung von `atan(a/g)` (Rasen ≈ 4,7°) bleibt der Ball liegen, steilere Hänge rollt er hinab.
- Der bisherige Sonderfall „Sand dämpft mit 0,85/s“ entfällt, weil die Tabelle ihn abdeckt.

### 2.2 Ball

| Eigenschaft | Alt | Neu | Grund |
|---|---|---|---|
| `linearDamping` | 0,9 | 0 | wirkte wie Luftbremse |
| `angularDamping` | 0,18 | 0 | Drall wird vom Rollmodell gesetzt |
| Reibung | 0,75 (Average) | 0 mit `CoefficientCombineRule.Min` | Rapier-Kontaktreibung kämpfte gegen den Rolldrall und kostete 40 % Weite; alle Tangentialeffekte kommen jetzt aus dem Rollmodell |
| Contact Skin | – | 0,005 m | Ohne Skin bremst die Eindringkorrektur an inneren Dreieckskanten die Position um 12 %. Mit Skin rollt der Ball auf Dreiecksnetzen exakt so weit wie auf einer Box. Der Ball schwebt dafür bis zu 5 mm über der Fläche, was nicht sichtbar ist. |
| Restitution | 0,12 | 0,12 | unverändert |
| CCD | an | an | unverändert; bei 16,8 m/s und 120 Hz sind es 0,14 m pro Schritt, weniger als der Ballradius |

### 2.3 Wände und Böden

- Kollider mit `surface` in {`grass`, `sand`} sind **Boden**: Restitution wie bisher (0,02/0,03).
- Alle anderen Kollider (heute `wood`/`stone`: Banden, Hindernisse) sind **Wände**: Restitution `wood` 0,78 und `stone` 0,82, beide mit `CoefficientCombineRule.Max`.
- Alle Trimesh-Kollider bekommen `TriMeshFlags.FIX_INTERNAL_EDGES`. Ohne den Fix springt der Ball auf einem fein triangulierten Boden bei vollem Schlag über 5 m hoch.
- In P3 wird die Rolle (`floor`/`wall`) explizit im Manifest stehen. Bis dahin gilt die Ableitung aus `surface`.

### 2.4 Abschuss und Stärke

- `shoot(direction, power)` bleibt die API. **`power` bedeutet neu den Anteil an der maximalen Rollweite** auf ebenem Rasen: `power · 28 m`.
- Die Abschussgeschwindigkeit ergibt sich aus der Umkehrung von `d(v0)` für Rasen per Bisektion (`launchSpeedFor(power)`). Volle Stärke entspricht 16,77 m/s.
- Der Ball startet mit Rolldrall `ω = (ŷ × v0)/r`. Damit entfällt der Gleit-Roll-Verlust von 30 %.
- Neue Exporte: `MAX_ROLL` (28), `rollDistance(speed, surface)`, `launchSpeedFor(power)`, `strengthToPower(strength)`.

### 2.5 Ruheerkennung

Unverändert ist die Bedingung „30 Schritte lang Geschwindigkeit < 0,075 m/s“. Die Drehgrenze passt sich dem Rolldrall an: `< 0,075/r + 0,05` statt `< 0,45`.

### 2.6 Loch, Wasser, Aus

Unverändert: Einfang bei horizontaler Geschwindigkeit ≤ 2,4 m/s unterhalb der Lochkante; Wasser und Aus mit Strafschlag.

## 3. Steuerung (`minigolf-scene.js`, `minigolf-game.js`)

### 3.1 Begriffe

- **Stärke `f`** (0–1): was Spielende einstellen. Zugweite, Regler, Pfeiltasten und Prozentanzeige arbeiten mit `f`.
- **`power`** (0–1): Anteil der Rollweite für die Physik: `power = strengthToPower(f) = f^1,5`.

Mit der Kurve gilt: 10 % Stärke ≈ 0,9 m, 25 % ≈ 3,5 m, 50 % ≈ 9,9 m, 75 % ≈ 18,2 m, 100 % = 28 m. Kurze Putts lassen sich also fein dosieren.

### 3.2 Ziehen mit Maus und Touch

- **Start:** wie bisher. Bei der Maus muss der Zug innerhalb von 46 px vom Ball beginnen, im Touch-Modus „Zielen“ überall.
- **Stärke:** `dragPx` ist der Abstand in CSS-Pixeln zwischen Zeiger und projiziertem Ballmittelpunkt. Daraus ergibt sich `f = clamp((dragPx − 14) / (maxPx − 14), 0, 1)` mit `maxPx = clamp(0,3 · min(Canvasbreite, Canvashöhe), 140, 320)`.
- **Richtung:** wie bisher aus dem Bodenebenen-Raycast (Ball minus Trefferpunkt). Trifft der Ray nicht, wird die Bildschirmrichtung über die Kamera-Yaw-Achse abgebildet.
- **Abbruch:** Loslassen mit `dragPx < 14` schießt nicht. Rechtsklick oder Escape während des Ziehens beenden den Zug ohne Schuss.
- Der Browser-Test „Tippen schießt nicht, 60 px Zug schießt“ bleibt gültig.

### 3.3 Tastatur und Regler

Pfeiltasten, Shift und Space bleiben wie bisher, wirken aber auf `f`. Der Regler (`data-power`, 2–100) zeigt `f` in Prozent. Der Standardwert pro Zug bleibt 35 %.

### 3.4 Anzeige

`view.power(f, meters)` zeigt „35 % · ≈ 6 m“. Die Meterzahl ist `power · 28`, unter 10 m gerundet auf 0,5 m, darüber auf ganze Meter. Der Text ist sprachneutral; es gibt keine neuen Sprachschlüssel. Die eigentliche HUD-Gestaltung folgt in P4.

## 4. Laufbahnvorschau

### 4.1 Berechnung (`physics.predict`)

- Beim ersten Aufruf nach dem Laden einer Bahn entsteht eine **Geisterwelt**: `RAPIER.World.restoreSnapshot(world.takeSnapshot())`. Sie bleibt bis zum Bahnwechsel oder bis `dispose()` bestehen. Die Handles von Ball und Kollidern sind in der Kopie identisch, die Oberflächenzuordnung gilt also weiter.
- Ablauf pro Vorhersage:
  1. Hindernis-Posen auf die aktuelle Spielzeit setzen.
  2. Den Geisterball auf die aktuelle Ballposition setzen und mit derselben Abschusslogik starten.
  3. Mit demselben Rollmodell schrittweise simulieren (gemeinsame Funktion, kein Code-Duplikat).
- Abbruch, sobald eines davon eintritt:
  - nach der ersten Berührung einer Wand (Kollisionsereignis mit Wand-Kollider) weitere 0,9 m Weg,
  - Weglänge ≥ `max(1,5 m, 0,45 · power · 28 m)` (die Vorschau verrät nie das ganze Ausrollen eines langen Schlags),
  - Ruhe, Wasser, Aus,
  - Ball innerhalb des Lochradius (die Linie endet dort ohne Hinweis auf „Treffer“),
  - spätestens 360 Schritte (3 s).
- Rückgabe: `{ points: [[x,y,z], …], contactIndex, hazard }`. Ein Punkt wird je ≥ 0,08 m Weg aufgezeichnet. `contactIndex` ist der Punktindex der ersten Wandberührung oder `-1`, `hazard` ist `'water' | 'out' | null`.
- `predict` sendet keine Ereignisse und verändert die Live-Welt nicht. Während eines laufenden Schlags gibt es `null` zurück.
- Kosten (gemessen in Node): Snapshot und Wiederherstellung 1–11 ms einmal pro Bahn, 240 Schritte etwa 6 ms. Das Spiel ruft `predict` höchstens alle 66 ms auf, wenn sich Richtung oder Stärke ändern, und auf Bahnen mit beweglichen Hindernissen zusätzlich alle 150 ms im Stillstand.

### 4.2 Darstellung (`minigolf-scene.js`)

- Pfeil (`ArrowHelper`) und gerade Ziellinie werden durch eine gestrichelte Linie entlang `points` ersetzt. Bis `contactIndex` ist sie in der Stärkefarbe, danach mit halber Deckkraft gezeichnet.
- Am Ende sitzt ein kleiner Punkt, bei `hazard` in Rot (`0xff6b5e`).
- Der Stärkering am Ball bleibt und skaliert mit `f`.
- Die Striche sind statisch, damit gilt die Darstellung unverändert auch bei reduzierter Bewegung.
- Die Zielhilfe „Magnet“ (Einrasten bis 4° zum Loch) wirkt auch auf die Vorschau, damit sie dem tatsächlichen Schuss entspricht.

## 5. Tests

`scripts/test-minigolf-physics.mjs`:

| Prüfung | Grenze |
|---|---|
| Voller Schlag, ebene Box | 28 m ± 10 %, Ruhe ≤ 6 s |
| Voller Schlag, Geschwindigkeit nach 0,1 s | ≥ 95 % von `v0` |
| Voller Schlag auf fein trianguliertem Trimesh (0,25-m-Zellen) | Weite innerhalb 5 % der Box-Weite, Höhenabweichung ≤ 1 cm |
| `power` 0,01 | Weite ≤ 0,3 m |
| `power` 0,1 / 0,25 / 0,5 | Weite innerhalb 5 % von `power · 28 m` |
| 45°-Bandenschlag (wood und stone) | ≥ 65 % Geschwindigkeit erhalten |
| Hang 6,9° | Ball rollt hinab |
| Hang 2,5° | Ball bleibt liegen (Ruhe-Ereignis) |
| Sand gegen Rasen | Weite < 0,65 × |
| `launchSpeedFor` | streng monoton, `launchSpeedFor(1)` ≈ 16,77 m/s |
| `predict` an einer Wand | endet ≤ 1,0 m nach der ersten Wandberührung, `contactIndex ≥ 0` |
| `predict` ist nebenwirkungsfrei | Live-Ballpose, Zeit und Ereignisse unverändert |

Bestehende Prüfungen bleiben. Grenzen, die an die alte Höchstgeschwindigkeit von 12 m/s gebunden waren, werden auf `launchSpeedFor(power)` umgestellt (Renderpose höchstens ein Fixschritt hinter der Physikpose, Interpolationssprung pro 144-Hz-Frame). Die Hopser-Grenze beim flachen Putt erhält die Contact Skin als Toleranz.

`scripts/test-minigolf-courses.mjs`:

- Die Routen aller 18 Bahnen werden mit `AURAGO_MINIGOLF_SEARCH=1` neu aufgezeichnet.
- Der Suchmodus prüft jede gefundene Route sofort durch eine durchgehende Wiederholung. Weil die Suche jeden Schlag bei Hindernis-Zeit 0 beginnt, bekommt eine Route hinter einem beweglichen Hindernis vor jedem Schlag eine Wartezeit, bis das Hindernis wieder in dieser Phase steht (ganze Perioden `2π/speed`). Der Prototyp zeigte: garden-06 und coast-11 brauchen das; nur den gescheiterten Schlag zu verzögern reicht nicht, weil schon der Schlag davor an anderer Stelle liegen bleibt.

Browser-Smoke (`TestDesktopMinigolfBrowser`):

- Der bestehende Tipp-/Zieh-Test läuft unverändert.
- Neu: Während des Zielens liefert `golfFixture.inspect().graphics.preview` mindestens 2 Punkte.

## 6. Messungen des Prototyps (2026-10-18)

Gemessen mit den Werten aus Abschnitt 2 gegen Rapier 0.21.0 bei 120 Hz:

| Messung | Ergebnis |
|---|---|
| Voller Schlag, Box | 28,02 m, Ruhe nach 5,36 s, 95,1 % nach 0,1 s |
| `power` 0,5 / 0,25 / 0,1 / 0,03 / 0,01 | 14,01 / 7,01 / 2,81 / 0,84 / 0,28 m |
| Voller Schlag, Trimesh-Gitter | 28,02 m, max. 0,49 cm Höhenabweichung (Skin) |
| Ohne `FIX_INTERNAL_EDGES` | Ball springt 5,2 m hoch |
| Ohne Contact Skin | 24,6 m auf dem Gitter (−12 %) |
| Ballreibung 0,75 statt 0 | 17,4 m (−38 %) |
| 45°-Bande wood / stone | 86 % / 88 % erhalten |
| Frontal an die Bande, volle Stärke | 76 % erhalten, kein Hochklettern (0,5 cm) |
| Hang 6,9° / 2,5° | rollt / bleibt liegen |
| Sand / Rasen bei `power` 0,5 | 0,34 |
| Alte 18 Bahnen, Routensuche | alle lösbar, 14 als Hole-in-One; garden-06 und coast-11 brauchen Wartezeiten |
| Vorschau: Snapshot+Restore / 240 Schritte | 1–11 ms einmalig / ca. 6 ms |

## 7. Nicht Teil von P1

- HUD-Gestaltung, Stärkeanzeige am Ball als eigenes Element, Einstellungen für die Vorschau (P4).
- Neue Bahnen, Welt-Mechaniken, Manifest v2 mit expliziter Kollider-Rolle (P3).
- Sound (P5). Der Schlagklang bleibt wie bisher an die Stärke gekoppelt (`speed = f · 12`).
- Änderungen am Lochfang, an Strafschlägen oder an Spielständen.
