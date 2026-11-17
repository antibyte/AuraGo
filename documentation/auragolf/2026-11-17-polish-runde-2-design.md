# AuraGolf – Polish-Runde 2 (Design)

Stand: 2026-11-17. Freigegeben vom Nutzer am 2026-11-17 („Ja, alles umsetzen“).

Ziel: mehr Spielgefühl in der bestehenden App. Dazu gehören:
- eine sichtbare Ziellinie und ein Zielen, das dem Bahnverlauf folgt;
- Rückmeldung bei Beinahe-Treffern, Hindernissen und Wandtreffern;
- Momente am Loch (Fahne, Ass, Wertungstöne);
- Fairness (hängender Ball, Schlaglimit) und Motivation (Sterne, Serien, Bestwerte).

Abgrenzung: keine neuen 3D-Modelle, keine Audiodateien, keine Abhängigkeiten. Keine Änderung an Bahnen, Manifest und den Speicherformaten `round.v2`, `best.v2` und `settings v1`. Wo dieses Dokument nichts sagt, gelten `2026-11-11-polish-runde-design.md` und die P-Specs weiter.

## 1. Feste Regeln

- Abschnitt 1 von `2026-11-11-polish-runde-design.md` gilt vollständig:
  - 13 px Text, 44 px Bedienelemente, kein Überlauf (`layoutIssues()`);
  - die Startseite wird bei 1280 × 760 nicht höher;
  - reduzierte Bewegung heißt keine Animation; die zwei Sammelregeln bleiben am Ende der CSS-Datei;
  - die bestehenden `data-*`-Haken und Klassen bleiben;
  - Farben kommen aus `--mg-*`; keine externen Ressourcen.
- Animationen dauern höchstens 400 ms. Freigegebene Ausnahmen:
  - die Kamerafahrt um das Loch beim Ass (2 s);
  - Feuerwerkspartikel (Vorbild ist das bestehende `win`);
  - der endlose Puls des Lochrings beim Zielen (wie `mg-pulse`).

  Alle anderen neuen Bewegungen bleiben darunter: Kamerafahrt 0,4 s, Ball-Pop 0,3 s, Wackeln 0,25 s, Fahne 0,25 s.
- Reduzierte Bewegung ist in der Szene `reduced`, im DOM `data-reduced=true` oder `prefers-reduced-motion`. Darunter:
  - kein Orbit, kein Wackeln, kein Pop, kein Puls;
  - Kamerawechsel springen statt zu fahren;
  - die Effekte zeigen wie heute nur die Ringe.
- Die niedrige Stufe (`tier low`) bekommt jede Funktion. Partikel begrenzen die bestehenden Pools (`TIERS.low.particles` = 60).
- Neue Töne werden nur in `minigolf-audio-synth.js` erzeugt oder kommen aus vorhandenen Samples.
- Physik: Bestehende Schüsse bleiben bitgleich. Abschnitt 3.6 sagt, warum das für jede Änderung gilt. Die Routen-Replays und der Collider-Fixture-Test müssen unverändert grün sein.
- Neue Texte kommen in alle 16 Sprachen, direkt nach `minigolf.tutorial_skip`, in der Reihenfolge von Abschnitt 8.
  - Platzhalter wie `{n}` füllt `fillText(tr(key), values)` aus `minigolf-shared.js`; `tr` selbst ersetzt nichts.
  - Deutsch duzt mit großem „Du“.
- Die Implementierer bauen das Bündel nicht. `npm run test:minigolf` schlägt bis zum Neubau an `build-minigolf.mjs --check` fehl. Daher laufen die einzelnen Skripte direkt (`node scripts/test-minigolf-*.mjs`). Bündel und Gesamtlauf macht die Hauptsitzung.
- Laut `CLAUDE.md`: vor jeder Änderung einer Funktion `impact` (GitNexus) ausführen und Aufrufer und Risiko im Bericht nennen. Die Hauptsitzung hat das am 2026-11-17 für `minigolf-physics.js`, `-mechanics.js`, `-scene.js`, `-fx.js`, `-audio.js` und `-hud.js` getan (Risiko jeweils LOW; `minigolf-game.js` UNKNOWN, per Textsuche geklärt: nur `scripts/build-minigolf.mjs` bindet es ein). Der Index liegt 860 Commits zurück; Aufrufer einer geänderten Funktion deshalb zusätzlich per Textsuche prüfen.
- Tests bleiben schlank:
  - Einheitstests nur für neue reine Funktionen und neue Physik-Ereignisse;
  - einige Zusicherungen im bestehenden Browser-Test;
  - keine neue Testsuite.

## 2. Aufteilung der Dateien

| Paket | Welle | Dateien (`ui/js/desktop/apps/`, sonst Pfad angegeben) |
|---|---|---|
| M1 Physik und reine Helfer | 1 | `minigolf-physics.js`, `minigolf-mechanics.js`, `minigolf-model.js`, `minigolf-shared.js`, `scripts/test-minigolf-physics.mjs`, `scripts/test-minigolf-model.mjs` |
| M2 Szene und Effekte | 1 | `minigolf-scene.js`, `minigolf-fx.js` (`minigolf-camera.js`, `minigolf-looks.js` nur falls nötig) |
| M4 HUD, Menüs, Stil, Texte | 1 | `minigolf-hud.js`, `minigolf-menus.js`, `minigolf-scorecard.js`, `minigolf-views.js`, `ui/css/desktop-app-minigolf.css`, `ui/lang/desktop/*.json` (16), `scripts/test-minigolf-ui.mjs` |
| M3 Audio | 2 | `minigolf-audio.js`, `minigolf-audio-synth.js`, `minigolf-audio-engine.js`, `scripts/test-minigolf-audio.mjs` |
| M5 Verdrahtung | 2 | `minigolf-game.js`, `ui/testdata/minigolf-fixture.js` (nur falls nötig), `ui/desktop_minigolf_browser_test.go` |

Welle 1 läuft sicher gegen das alte Spiel:
- `lip` und `capture` landen im alten `handleEvents` bei `resolveShot`. Das gibt für unbekannte Typen `false` zurück, und das Spiel macht mit `continue` weiter.
- Ein erzwungenes `rest` wird normal aufgelöst.

Welle 2 läuft sicher gegen das alte Audio: Unbekannte Effekt-Typen geben `false` zurück, unbekannte `ui`-Arten spielen den Standardklick.

### 2.1 Schnittstellen

| Schnittstelle | Anbieter | Nutzer |
|---|---|---|
| Ereignis `{ type:'lip', position, speed }`, nicht beendend, höchstens einmal je Schuss | M1 | M5 |
| Ereignis `{ type:'capture', position, id, muzzle }`: Kanone fängt den Ball (`position` = Eintritt) | M1 (`stepZones`) | M5 |
| `bumper`-Ereignis zusätzlich `at` (Bumper-Mitte, Kopie), `radius`, `speed` (waagrecht vor dem Stoß) | M1 (`applyBumpers`) | M5 |
| `water` und `out` zusätzlich `speed` (Betrag der Geschwindigkeit vor dem Anhalten) | M1 | M5 → M3 |
| `rest` mit `forced: true` nach `SHOT_TIMEOUT` | M1 | M5 |
| `physics.predict()` liefert zusätzlich `cup: boolean` | M1 | M2 (`setPreview`), M5 reicht durch |
| `cancelShot(round): boolean` | M1 `minigolf-model.js` | M5 |
| `underParStreak(player, round, courses): number` | M1 `minigolf-model.js` | M5 |
| `aimAlongPath(path, ball, cup, options?): [x, y, z]` | M1 `minigolf-shared.js` | M5 |
| `TERMINAL_EVENTS` (besteht) | `minigolf-mechanics.js` | M5 importiert |
| `scene.glideToBall()` | M2 | M5 |
| `scene.orbitCup(seconds = 2): boolean` | M2 | M5 |
| `scene.shake(strength)`, `strength` 0..1 | M2 | M5 |
| `scene.popBall()` | M2 | M5 |
| `scene.kickFlag(amount = 1.2)` | M2 | M5 |
| `scene.burst(type, …)` neue Arten `'bumper'`, `'ace'`, `'dust'`; `'impact'` mit `speed` = k (0..1) und `info.surface`; `'strike'` leert die Spur | M2 | M5 |
| `scene.stats()` zusätzlich `aimDots`, `cupGlow`, `flagLift`, `orbit`, `tween`, `shake`, `ballScale` | M2 | M5 (Browser-Test) |
| `hud.update(round, course, busy)` wertet `busy` aus (A3; `views.update` reicht es heute schon durch) | M4 | M5 |
| `hud.plaqueInfo({ distance, best }, lang?)`, `distance` in m oder `null`, `best` Zahl oder `null` | M4 | M5 |
| `hud.toast(key, values?)`, `values` für `fillText` | M4 | M5 |
| `TOASTS`: neu `near_miss`, `stuck`, `limit_warning`, `limit_last`, `tutorial_camera` | M4 | M5 |
| `formatDistance(meters, lang): string` (exportiert) | M4 `minigolf-hud.js` | Test |
| `holeStars(strokes, par)`, `starTally(best, courses)` (exportiert) | M4 `minigolf-menus.js` | M4, Test |
| `[data-plaque-meta]`, `.mg-hole-stars[data-stars]`, `.mg-round-stars` | M4 | M5 (Browser-Test) |
| `audio.effect(type, { speed, pan, delay })`, neue `type`: `bumper`, `capture`, `lip`, `poof`, `fanfare`, `chime`, `low`, `best`, `turn` | M3 | M5 |
| `audio.effect('cup'|'splash', { speed, pan })` skaliert mit `speed` | M3 | M5 |
| `audio.ui(kind)`: neue Arten `card`, `select`, `switch-on`, `switch-off` | M3 | M5 |
| Textschlüssel aus Abschnitt 8 | M4 | M5 |

## 3. M1 – Physik, Mechanik, reine Helfer

### 3.1 Eine Lochregel für Spiel und Vorschau (A5)

- In `createPhysics` kommen neue Konstanten dazu: `CUP_SPEED = 2.4` und `SHOT_TIMEOUT = 20` (exportiert). Dazu ein innerer Helfer `dropsIn(position, across, v)` mit `across` = waagrechter Abstand zur Lochmitte. Er gibt `true` zurück bei:
  - `position[1] < course.cup[1] - 0.1`,
  - `across < cupOpening - BALL_RADIUS * 0.1` und
  - `Math.hypot(v.x, v.z) <= CUP_SPEED`.
- `resolveConditions` nutzt `dropsIn` statt der Zeile ~297. Die Rechnung bleibt gleich: `cupRadius` und `cupOpening` haben denselben Wert.
- `predict()`:
  - Statt Zeile ~473 gilt: `if (dropsIn(position, across, ball.linvel())) { cup = true; break; }`.
  - Die Bedingung `travelled >= limit` beendet die Vorschau nur noch, wenn `across >= cupOpening` ist. Ein Ball über dem Loch fällt also zu Ende oder rollt weiter. Die Grenze von 360 Schritten bleibt.
  - Rückgabe: `{ points, contactIndex, hazard, zone, cup }`.
- Abhängigkeiten geprüft: `scripts/minigolf-route-search.mjs` und `scripts/lib/minigolf-routes.mjs` nutzen `predict` nicht. Dort zählt die Abstandsregel in `remaining()`, und die bleibt. Die Vorschau-Tests in `test-minigolf-physics.mjs` (Zeilen 288–333 und 488–514) haben das Loch bei [55, 0, 55] und sind nicht betroffen.

### 3.2 Beinahe-Treffer `lip` (B1)

- Pro Schuss gibt es die Zustände `overCup` und `lipped`. `shoot()` und `setBall()` setzen beide auf `false`.
- In `resolveConditions` nach der Lochprüfung:
  - `across < cupOpening` setzt `overCup`.
  - Ist `overCup` gesetzt, `lipped` noch nicht und `across >= cupOpening + LIP_MARGIN` (0,02 m), dann gilt `lipped = true` und es wird `emit('lip', position, { speed: Math.hypot(v.x, v.z) })` ausgelöst.
- **Abweichung:** Das Ereignis heißt „verlässt die Öffnung wieder, ohne zu fallen“ statt „zu schnell“. Grund: Es erfasst auch Kantenroller unter 2,4 m/s, die physikalisch ebenso knapp vorbeigehen.

### 3.3 Zeitlimit (D1)

- In `resolveConditions` folgt nach der `lip`-Prüfung und vor der Ruhezählung:
  - `if (!zoneState.captured && time - shotStartedAt >= SHOT_TIMEOUT - 1e-9) { emit('rest', position, { forced: true }); return; }`
- Solange die Kanone den Ball hält, zählt das Limit nicht. Die Haltezeit ist durch `delay` begrenzt. Nach der Freigabe greift es im nächsten Schritt, wenn die 20 s schon um sind.
- Der Ball bleibt dort, wo er ist, auch in der Luft. Der nächste Schuss startet von dort, die Schwerkraft wirkt.
- **Prüfpflicht:** Die Routen erlauben 30 s je Schuss (`MAX_FRAMES` in `minigolf-routes.mjs`). M1 führt `node scripts/test-minigolf-courses.mjs` aus. Scheitert eine aufgezeichnete Route am Limit, wird `SHOT_TIMEOUT = 30`. Das ist dann eine Abweichung mit Begründung im Bericht: Bei 30 s kann keine bestehende Route betroffen sein.

### 3.4 Nutzlasten der Ereignisse (B4, C1, C2)

- `water` und `out` bekommen `{ speed: speed(ballBody.linvel()) }`. Die Geschwindigkeit wird vor `emit` gelesen, weil `emit` die Geschwindigkeit nullt.
- `applyBumpers` liest die Geschwindigkeit einmal: `const v = ball.velocity()`. Dann `ball.setVelocity(bumperVelocity(bumper, position, v))` und `emit?.('bumper', position, { id, at: bumper.position.slice(), radius: bumper.radius, speed: Math.hypot(v[0], v[2]) })`.
- `stepZones`, Fangzweig der Kanone: nach `ball.hold(zone.muzzle)` folgt `emit?.('capture', position, { id: zone.id, muzzle: zone.muzzle.slice() })`. Die Vorschau ruft `stop` und erreicht diesen Zweig nie.

### 3.5 Modell und reine Helfer

- `cancelShot(round)` in `minigolf-model.js` entfernt `round` aus `activeShots` und gibt zurück, ob es dort war. Ohne das liefert `beginShot` nach einem verworfenen Ende dauerhaft `null`.
- `underParStreak(player, round, courses)` zählt Bahnen unter Par in Folge, endend mit der zuletzt beendeten Bahn.
  - Grundlage sind `player.scores` und, wenn `player.done && !round.completed && round.holeIndex < round.courseIds.length`, zusätzlich `player.strokes`.
  - Par liefert `courses.find(c => c.id === round.courseIds[i]).par`.
  - Fehlt Par oder ist eine Bahn nicht unter Par, endet die Zählung.
- `aimAlongPath(path, ball, cup, { lookAhead = 3, nearCup = 3.5, offPath = 2.5 } = {})` in `minigolf-shared.js` rechnet in der Ebene XZ und gibt einen Zielpunkt zurück.
  - Es gibt `cup.slice()` zurück, wenn: der Pfad weniger als zwei gültige Punkte hat; der Ball näher als `nearCup` am Loch liegt; der nächste Pfadpunkt weiter als `offPath` entfernt ist (Ball auf einem Nebenarm); oder vom Projektionspunkt weniger als `lookAhead / 2` Pfad übrig sind.
  - Sonst nimmt es den Pfadpunkt bei Bogenlänge `s + lookAhead` (y interpoliert).
  - Liegt das Loch näher am Ball als dieser Punkt, wird es das Loch.

### 3.6 Warum bestehende Schüsse bitgleich bleiben

- `dropsIn` ist dieselbe Rechnung in derselben Reihenfolge.
- `lip`, `capture` sowie die neuen Felder in `bumper`, `water` und `out` lesen nur oder rufen `onEvent` auf. Kein Körper wird verändert.
- `applyBumpers` ruft `ball.velocity()` einmal statt einmal inline auf; der Wert ist derselbe.
- Das Zeitlimit greift erst nach 20 s und nur, wenn die Routen-Prüfung aus 3.3 das bestätigt.
- Die Änderung an `predict()` betrifft nur die Geisterwelt (Test Zeile 312).
- Hinweis zu Zeile 305: `restAfterShot` endet bei jedem Ereignis außer `impact`. Dort gibt es weder Loch noch Mechanik, also bleibt das Verhalten gleich.

### 3.7 Tests (M1)

`test-minigolf-physics.mjs`:
- `fastCup`: genau ein `lip`. `slowCup`: kein `lip`.
- Vorschau auf `ringMesh()`, Abschlag [-1, 0.16, 0], Loch [0, 0, 0]:
  - `predict([1,0], 0.06).cup === true` (rollt etwa 1,7 m, am Rand rund 1,2 m/s);
  - `predict([1,0], 1).cup === false`.
- Bumper-Ereignis hat `at`, `radius` und `speed > 0`.
- Kanone: ein `capture` mit `id` vor dem `cannon`-Ereignis. Zeile 408 bleibt gültig, weil `capture` ein eigener Typ ist.
- Kanone mit `delay: 25`: Nach dem `cannon`-Ereignis folgt innerhalb von 3 Schritten `rest` mit `forced: true`.
- `water` trägt `speed > 0`.

`test-minigolf-model.mjs`:
- `cancelShot`: nach `beginShot` `true`, danach geht `beginShot` wieder.
- `underParStreak`: 0, 3, Abbruch bei Par, laufende Bahn zählt nur, wenn der Spieler fertig ist.
- `aimAlongPath`: gerade Bahn, L-Bahn um die Ecke, Ball in Lochnähe, Ball abseits des Pfads, leerer Pfad.

## 4. M2 – Szene, Effekte, Kamera

### 4.1 Ziellinie aus Punkten (A1)

- `previewLine` und `previewTail` (`dashedLine` / `fillLine`) entfallen. Neu ist `aimDots`, ein `THREE.InstancedMesh(new SphereGeometry(1, 10, 6), MeshBasicMaterial({ transparent: true, opacity: .92, depthWrite: false }), AIM_DOT_MAX)` mit `renderOrder = 2` und `frustumCulled = false`.
  - Kugeln statt Scheiben, weil flache Scheiben bei flachem Kamerawinkel (bis 83° Polarwinkel) verschwinden.
- Konstanten: `AIM_DOT_SPACING = .28`, `AIM_DOT_FIRST = .32`, `AIM_DOT_MAX = 96`, `AIM_DOT_RADIUS = .045`, `AIM_DOT_MAX_RADIUS = .09`, `AIM_DOT_MIN_PX = 2.5` (Radius in px, also mindestens 5 px Durchmesser), `TAIL_SCALE = .75`, `TAIL_TINT = .55`.
- Ist `previewDirty` gesetzt oder hat sich die Stärke geändert, tastet `updateAim()` die Polylinie im Abstand `AIM_DOT_SPACING` ab, beginnend bei `AIM_DOT_FIRST`.
  - Punkte ab `contactIndex` gehören zum Schweif: Instanzfarbe `TAIL_TINT`, sonst 1.
  - Die Materialfarbe ist `powerColor(aimPower)`.
- Größe in jedem Bild, solange sichtbar:
  - `pxPerMeter = host.clientHeight / (2·tan(fov/2)·Abstand)`;
  - `r = min(AIM_DOT_MAX_RADIUS, max(AIM_DOT_RADIUS, AIM_DOT_MIN_PX / pxPerMeter))`, im Schweif `× TAIL_SCALE`;
  - Mitte bei `y − .16 + r`.
- Sichtbarkeit wie bisher (`aiming && points`). `previewEnd` bleibt. Seine Farbe: Loch → `look.fx.gold`, Gefahr → `0xff6b5e`, sonst `0xffedb3`.

### 4.2 Lochring beim Zielen (A1/A5)

- Neues Netz `cupGlow`: `RingGeometry(r + .03, r + .11, 48)` liegend bei `course.cup[1] + .012`, mit `r = course.cupRadius ?? .32`. Farbe `look.fx.gold`, `depthWrite: false`.
- Sichtbar, solange gezielt wird und `preview.cup` gesetzt ist.
- Deckkraft `.55 + .35·sin(elapsed·4)`. Bei reduzierter Bewegung fest `.8`.

### 4.3 Fahne (B2, B1)

- **Abweichung:** Ein Anheben der Fahne samt Stange geht nicht.
  - `flag-pole-base` ist in `assets/minigolf/kit/export.py` kein Funktionsknoten. `join_scenery` verschmilzt die Stange daher in ein `scenery-*`-Netz.
  - Sie ist kein Physik-Collider: kein `aurago_collider`, und sie steht 0,43 m neben dem Loch (`cup_parts.py`). Es ist also nichts zu tun.
  - Schlanke Variante: Das Tuch („flag“ / `flag-cloth*`) öffnet sich.
- Ablauf:
  - Je Bild berechnet die Szene den waagrechten Abstand Ball–`course.cup`.
  - Das Ziel `flagLift` wird 1 unter `FLAG_NEAR = 1.8` m und 0 über `FLAG_FAR = 2.2` m; dazwischen bleibt es.
  - Der Wert folgt linear mit Rate `1 / .25` pro Sekunde, bei reduzierter Bewegung sofort.
- Gierwinkel des Tuchs: `flag.base + wave + flagLift · FLAG_OPEN` mit `FLAG_OPEN = .6` rad. Die Wellenamplitude wächst um `flagLift · .06`.
- `scene.kickFlag(amount)` ruft `fx.kickFlag(amount)` auf, also `flagBoost = max(flagBoost, amount)`.

### 4.4 Kamera (B5, B3, C3)

- `glideToBall()`:
  - Versatz `camera.position − controls.target`. Hat der Spieler seit dem Laden der Bahn selbst gezoomt (`userZoomed`: nach einem `start` der Steuerung beobachtet die Szene den Abstand 1,2 s lang und setzt die Marke bei mehr als 0,5 m Änderung; das Rad meldet `start` und `end` im selben Aufruf, die gedämpfte Fahrt folgt erst danach. Nicht während des Intros, einer Kamerafahrt oder des Orbits), wird die Länge auf `[controls.minDistance, controls.maxDistance]` begrenzt; sonst auf 3–8 m wie bei `focusBall()`. Grund (Screenshots vom 2026-11-17): Ohne diese Regel bliebe die Kamera nach dem Intro die ganze Bahn über in der Übersicht (25–40 m). `load()` und `focusBall()` setzen `userZoomed` zurück; `stats()` zeigt es;
  - `camTween` zum Ziel `ball.position` in 0,4 s (`smoothstep` wie heute);
  - bei reduzierter Bewegung sofort; während des Intros keine Wirkung.
  - `focusBall()` und damit der Knopf „Zum Ball“ behalten ihre Begrenzung auf 3–8 m.
- `orbitCup(seconds = 2)`:
  - ohne Wirkung bei reduzierter Bewegung (gibt `false`);
  - Mitte `course.cup`, Abstand der aktuelle Abstand begrenzt auf 4–7 m, Höhenwinkel 35°;
  - 120° Schwenk ab dem aktuellen Azimut, `smoothstep`, danach bleibt die Kamera stehen;
  - `controls` `start`, `load()` und jedes neue `camTween` beenden ihn; er setzt seinerseits `camTween = null`.
- `shake(strength)`:
  - ohne Wirkung bei reduzierter Bewegung;
  - 0,25 s Abklingzeit; jedes Bild ein zufälliger Versatz bis `.05·strength·(rest/.25)` m;
  - wird nach `controls.update()` addiert und nach dem Rendern wieder abgezogen, damit OrbitControls ihn nie übernimmt;
  - ein neuer Aufruf nimmt das Maximum der Stärken.

### 4.5 Ball-Pop (C4)

`popBall()` skaliert die Ballgruppe in 0,3 s von 0 auf 1 (easeOutBack, Überschwinger 1,08). Bei reduzierter Bewegung: Maßstab 1.

### 4.6 Effekte (`minigolf-fx.js`, C1–C5, D6)

- `strike`: `k = clamp(speed/2, 0, 1)` (das Spiel übergibt `amount·2`). Anzahl `round(Basis·(.5 + k))`, Auswurf `out × (.6 + .8k)`.
  - `scene.burst('strike')` ruft zusätzlich `fx.resetTrail()` (D6).
- `impact`: `k = clamp(speed, 0, 1)`, Anzahl `round(3 + 11k)`. Je nach `info.surface`:
  - `wood` → Holzspäne `fx.wood`;
  - `stone` → Staub `fx.stone`;
  - `metal` → Funken (glow, `0xfff1b0`);
  - `grass` / `sand` → kleiner Hauch `fx.grass` / `fx.sand` (`2 + 4k`, `up [.2,.6]`, `gravity 3`);
  - ohne Fläche: bisherige Wahl nach `fx.wall`.
- `dust` (Aus): Puffwolke am Ereignisort, Pool `soft`, 14 Teilchen `fx.stone`. Werte: `up [.2,.6]`, `out [.4,1.2]`, `size [.14,.26]`, `life [.5,.9]`, `gravity -.2`, `drag 3`, `grow 1.4`, `opacity .6`.
- `bumper`: neue dritte Ringinstanz `flash` (`createRing`), `play(x, y+.03, z, fx.gold, false, radius + .35, .3)`, dazu 10 glow-Funken mit `k = clamp(speed/8, .4, 1)`. Bei reduzierter Bewegung nichts (der Effekt kommt zu oft).
- `ace`: wie `cup`, dazu drei Raketen (Logik von `win`) bei `x ± 1.4`, `z ± .8`, Verzögerung 0, .3, .6 s. Bei reduzierter Bewegung nur der Ring.
- `kickFlag(amount)` siehe 4.3. `clear()` setzt `flash` zurück.

### 4.7 `stats()`

Neu:
- `aimDots` (Zahl sichtbarer Punkte);
- `cupGlow` (bool);
- `flagLift` (0..1);
- `orbit` (bool);
- `tween` (`!!camTween`);
- `shake` (verbleibende s);
- `ballScale`.

## 5. M4 – HUD, Menüs, Stil, Texte

### 5.1 Schlagchip zählt beim Schlag (A3)

- `hud.update(round, course, busy)`: Der Chip des aktiven, nicht fertigen Spielers zeigt `strokes + 1`, solange `busy`.
- `strokeMemory` speichert die angezeigte Zahl:
  - Beim Schlag springt der Chip einmal (`mg-bump`).
  - Beim Ruhen bleibt die Zahl gleich, also kein zweiter Sprung.
  - Wasser oder Aus (+2) springt noch einmal; so wird die Strafe sichtbar.

### 5.2 Plakette (A4)

- Neues Element nach `[data-plaque-name]`: `<span class="mg-plaque-meta" data-plaque-meta hidden></span>`.
- `plaqueInfo({ distance, best }, lang = document.documentElement.lang)` setzt den Text `fillText(tr('to_cup'), { distance: formatDistance(distance, lang) })`.
  - Mit `best` folgt ` · ` und `fillText(tr('best_value'), { n: best })`.
  - Sind beide `null`, bekommt das Element `hidden`.
  - `textContent` wird nur bei einer Änderung geschrieben.
- `formatDistance(m, lang)`: unter 10 m eine feste Nachkommastelle, sonst keine. Dazu `\xa0m`, über das vorhandene `numberFormat`.
- CSS: `.mg-plaque-meta{font-size:13px;color:var(--mg-muted);font-variant-numeric:tabular-nums;max-width:min(260px,60cqw);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}`.

### 5.3 Toasts

- `toast(key, values)`: Der Text ist `values ? fillText(tr(key), values) : tr(key)`.
- `TOASTS` bekommt:

  | Schlüssel | Art | Dauer |
  |---|---|---|
  | `near_miss` | info | 2,2 s |
  | `stuck` | info | 4 s |
  | `limit_warning` | warning | 3,5 s |
  | `limit_last` | warning | 3,5 s |
  | `tutorial_camera` | info | 3 s |

### 5.4 Sterne (D2)

- In `minigolf-menus.js` exportiert:
  - `holeStars(strokes, par)`: 0 ohne gültigen Wert, 3 unter Par, 2 bei Par, sonst 1;
  - `starTally(best, courses)`: `{ stars, max: 3·courses.length }`, gerechnet aus `best.holes`.
  - Sie liegen in `menus.js` statt im Modell, weil nur die Menüs sie brauchen; so importiert in Welle 1 kein Paket aus einem anderen.
- Bahnkachel: In `.mg-hole-foot` folgt nach `.mg-hole-best`:
  - `<span class="mg-hole-stars" data-stars="n" role="img" aria-label="…" title="…">` mit drei `icon('star')`; die ersten `n` tragen `class="mg-star-on"`.
  - Der Text ist `fillText(tr('stars'), { n })`. Ohne Bestwert entfällt das Element.
  - `.mg-hole-best` bekommt `icon('trophy')` statt `icon('star')`, damit nichts mit den Sternen verwechselt wird. `data-rank` bleibt.
- Weltkarten: `bestChip` und das neue Sternfeld stehen gemeinsam in `<span class="mg-round-foot">`:
  - CSS: `display:flex; gap:8px; align-items:center; flex-wrap:nowrap; min-width:0`. Der Bestwert-Chip bekommt `min-width:0; overflow:hidden; text-overflow:ellipsis`.
  - Sternfeld: `<span class="mg-round-stars" title aria-label>${icon('star')}<span>7/18</span></span>` mit dem Text `fillText(tr('stars_total'), { n, max })`.
  - Es erscheint nur, wenn `stars > 0`.
- Karte „Alle 18 Bahnen“: Das Sternfeld steht in `.mg-round-end` vor dem Bestwert. Unter `@container (max-width:560px)` wird `.mg-round-end` ein Raster mit zwei Zeilen: Sterne über Bestwert, Pfeil rechts über beide Zeilen.
- Bei 1280 px wird keine Karte höher, denn die Sterne stehen in einer bestehenden Zeile.
- CSS:
  - `.mg-star-on` mit `fill`/`stroke` `var(--mg-accent-2)`, sonst gedämpft;
  - Symbole 14 px (Kachel) bzw. 16 px (Karte), Text 13 px fett, `tabular-nums`.

### 5.5 Banner (D3)

Das Spiel gibt höchstens eine Zusatzzeile mit (siehe 7.4). Neu `.mg-banner [data-banner-lines] span{overflow-wrap:anywhere}`, damit lange Namen bei 420 px umbrechen statt überzulaufen.

### 5.6 Texte

Neue Schlüssel nach Abschnitt 8 in alle 16 Dateien. Außerdem in allen 16 Sprachen geändert:
- `help_aim_body` und `tutorial_power`: „gestrichelte Linie“ wird zu „gepunktete Linie“.
- Englisch: „dashed line“ wird zu „dotted line“; die übrigen Sprachen sinngemäß.

### 5.7 Tests (M4)

`test-minigolf-ui.mjs`:
- `holeStars`: alle Stufen; Ass auf Par 2 ergibt 3;
- `starTally`: unbekannte Bahn ignoriert, `max`;
- `formatDistance`: `de` 3,4 m mit Komma, 12 m ohne Nachkommastelle;
- `hud.update` mit `busy` zeigt `strokes + 1` (mit DOM-Stub wie im Test vorhanden, sonst entfällt dieser Punkt).

## 6. M3 – Audio

### 6.1 Erzeugte Töne

In `minigolf-audio-synth.js` neu `export function playCue(ctx, destination, kind, { gain = 1, pan = 0, delay = 0 } = {})`:
- nutzt `tone()` bzw. Rauschen aus `noiseBuffer`;
- bei `pan ≠ 0` mit einem `StereoPanner`;
- gibt `true` zurück, wenn etwas geplant wurde;
- `start = ctx.currentTime + .005 + delay`.

| Art | Klang (Hz, s, Spitze) |
|---|---|
| `bumper` | Dreieck 520→780 (Gleiten in .03), Dauer .22, Spitze .32·gain; dazu Sinus 1560 / .12 / .1·gain |
| `capture` | Sinus 140→70 / .14 / .45; Rausch-Tick mit Bandpass 900 / .03 / .2 |
| `lip` | Dreieck 600→420 / .35 / .16 |
| `poof` | Rauschen mit Bandpass 700→300 (gleitend) / .28 / .25 |
| `fanfare` | Dreieck 523, 659, 784, 1047 im Abstand von je .11 s, je .16 s, Spitze .22; danach Akkord aus allen vier, .6 s, je .12; gesamt etwa 1,1 s |
| `chime` | Sinus 784, dann nach .09 s 1175, je .25, Spitze .18 |
| `low` | Sinus 196→174 / .35 / .16 |
| `best` | Sinus 880, 1109, 1319 im Abstand von .07 s, je .2, Spitze .16 |
| `turn` | Sinus 523, dann nach .08 s 784, je .12, Spitze .1 |

### 6.2 Effekte

- `EFFECT_TYPES` bekommt die neun Arten. In `fire()` ruft jede `playCue(engine.context(), engine.bus('effects'), kind, { gain, pan, delay })` auf.
- `bumper`: `gain = .5 + .5·min(1, speed/8)`.
- `fanfare`: zusätzlich `music.duck(1.6, .3)`.
- `options.delay` wird auf 0..2 s begrenzt.

### 6.3 Loch und Wasser (B4)

- `cup`: `n = Number.isFinite(speed) ? clamp(speed/4, 0, 1) : .5`.
  - Rand: Lautstärke `.3 + .4n`.
  - Klappern: `1 + round(2n) + floor(random()·2)` Treffer mit Lautstärke `(.3 + .3n)·.72^i`.
  - Ohne `speed` bleiben es 2–3 Treffer; der Test in Zeile 273 erwartet 2–4.
- `splash` skaliert schon mit `speed`. Neu ist nur, dass das Spiel den echten Wert übergibt.

### 6.4 Bedienklänge (D6)

- `UI_KIND` wird erweitert: `card` und `select` → `'click'`, `'switch-on'` → `'on'`, `'switch-off'` → `'off'`.
- `playUi`: `on` = Sinus 1200→1500, .04 s, .18; `off` = 1500→1100, .04 s, .18.

### 6.5 Blende vor der Pause (D6)

- `engine.suspend({ fade = 0, before } = {})` läuft in der `serial`-Aufgabe:
  - Läuft der Kontext und ist `fade > 0`: `graph.master.gain.setTargetAtTime(0, now, fade/3)`, dann `fade` Sekunden warten (per `setTimeout`).
  - Danach `before?.()`. Liefert das `false`, wird der Master-Pegel auf .9 zurückgesetzt und nicht suspendiert.
  - Sonst `ctx.suspend()`.
  - `before` läuft auch, wenn der Kontext nicht läuft.
- `resume()` und `start()` setzen nach dem Wiederanlauf `master.gain` mit Zeitkonstante .02 auf .9.
- `audio.pause()`:
  - setzt sofort `paused` und `pausedPosition`;
  - ruft `engine.suspend({ fade: .12, before: () => paused && (engine.stopVoices(0), stopRolling(0), stopAmbience(0), true) })` auf.
  - Ein schnelles `resume()` während der Blende lässt so nichts abreißen.
- `engine.suspend()` ohne Argumente (Unlock-Pfad) verhält sich wie heute.

### 6.6 Tests (M3)

`test-minigolf-audio.mjs`:
- jede neue Art gibt nach dem Unlock `true` zurück und erzeugt Oszillatoren oder Quellen;
- `cup` mit `speed .2` ergibt höchstens 2 Klapper-Treffer, mit `speed 4` mindestens 3;
- nach `pause()` steht vor `suspend` ein Gain-Ereignis `target 0` am Master;
- die bestehenden Prüfungen in den Zeilen 333–337 bleiben unverändert grün.

## 7. M5 – Verdrahtung (`minigolf-game.js`)

### 7.1 Zielen und Schlag

- **A2:** `setTurn()` nimmt `const t = aimAlongPath(course.path, player.position, course.cup)` und rechnet `angle = atan2(t[0] − p[0], −(t[2] − p[2]))`. Danach `scene.resetTrail?.()` (D6) und die Plakette (A4).
- **A4:**
  - `updatePlaque(position)` ruft `view.hud.plaqueInfo({ distance, best: best?.holes?.[course.id] ?? null })` auf, mit dem waagrechten Abstand von `position` zu `course.cup`.
  - Sind alle Spieler fertig, ist `distance` `null`.
  - In `frame()` wird alle 0,25 s aufgefrischt, solange `busy` gilt, mit `snapshot.position`.
- **A5/A1:** Keine Änderung nötig; `predict` liefert `cup`.
- **D5:**
  - `shoot()`: `if (cameraMode) { if (allowAim()) message('tutorial_camera'); return; }` steht vor der bisherigen Prüfung.
  - Pfeiltasten in `key()` und gehaltene Tasten in `frame()` gelten nur `!cameraMode`.
  - `actions.camera` ruft `clearHeld()` auf.
- `shoot()` setzt `shotLipped = false`.
- **D6:** `sessionShots = 0` in `startRound()` und `resume()`.

### 7.2 Ereignisse (`handleEvents`), Reihenfolge

1. `MECHANIC_EVENTS` um `capture` erweitert:
   - `bumper` → `audio.effect('bumper', { speed, pan })` und `scene.burst('bumper', event.at ?? event.position, event.speed, { radius: event.radius })`;
   - `capture` → `audio.effect('capture', { pan })` und `scene.burst('spark', event.position, .5)`;
   - `cannon` → wie heute, dazu `scene.shake(.8)`.
2. `lip` → `if (busy) shotLipped = true; continue`.
3. `impact`:
   - Funken mit `k = min(1, speed/8)`.
   - Ist `busy`, die Fläche nicht in `FLOOR_SURFACES` und `speed > 6` (`HARD_HIT`), dann `scene.shake(min(1, .35 + (speed − 6)/6))`.
4. Ist der Typ nicht in `TERMINAL_EVENTS`, gilt `continue`. Bei `!busy || !round` ebenfalls `continue`.
5. Vor dem Auflösen merken: `finishingPlayer`, `before`, `previousBest = best?.holes?.[course.id]`.
6. Gibt `resolveShot` `false` zurück (D1):
   - `cancelShot(round); busy = false; audio.rolling(0, lastSurface); setTurn(); continue`;
   - `setTurn` ruft `physics.setBall`, das hebt `shotLocked` auf.

### 7.3 Nach dem Auflösen

- `water`: `audio.effect('splash', { speed: event.speed, pan })`, Effekt `water`, Toast `water`.
- `out`: `audio.effect('poof', { pan })`, `scene.burst('dust', event.position)`, Toast `out`.
  - **Abweichung:** Kein Wassergeräusch. Die Wasservolumen der Bahnen reichen über die ganze XZ-Fläche der Grenzen. „Aus“ heißt daher meist Flug über den Rand oder ins Leere (Mond).
- Bei beiden setzt `scene.setRestMark(finishingPlayer.position)` die Restmarke an die Lage des bestraften Spielers. Das behebt den Fehler im Mehrspieler-Spiel.
- `cup`:
  - `audio.effect('cup', { speed: event.speed, pan })`;
  - `grade = holeGrade(...)`; Effekt `'ace'` bei `grade === 'ace'`, sonst `'cup'`; danach das Banner (7.4);
  - Wertungston mit `delay .35`: `ace` → `fanfare`, `eagle`/`birdie` → `chime`, `bogey`/`over` → `low`, `par` → keiner.
- `rest` mit `forced` → Toast `stuck`.
- Sonst Beinahe-Treffer, wenn `shotLipped` gesetzt ist oder `event.position` waagrecht näher als `NEAR_MISS_RADIUS = .7` m an der Lochmitte liegt (Ballrand höchstens rund 22 cm vom Lochrand): Toast `near_miss`, `audio.effect('lip', { pan })`, `scene.kickFlag(1.2)`.
- D4: Ist der Spieler nicht fertig und war das Ereignis nicht `cup`:
  - nach `shots === 10` Toast `limit_warning` mit `{ n: 2 }`;
  - nach `shots === 11` Toast `limit_last`;
  - `limit` bei 12 wie heute.
- Alle fertig:
  - Verzögerung `ACE_SCORE_DELAY_MS = 2200`, wenn der zuletzt fertige Spieler ein Ass hatte, sonst 900;
  - bei Ass und ohne reduzierte Bewegung `scene.orbitCup(2)`;
  - bei reduzierter Bewegung sofort wie heute.
- Sonst:
  - `setTurn()` und das Zugbanner wie heute;
  - wechselt der Zug bei mehreren Spielern und war es kein `cup`: `audio.effect('turn', { delay: .25 })`;
  - blieb derselbe Spieler am Zug nach `water`/`out`: `scene.popBall()`;
  - dann `scene.glideToBall?.(); scene.frameTurn?.(angle, position)` ohne Prüfung auf `reduced` (B5). Die Szene springt bei reduzierter Bewegung; `frameTurn` danach gewinnt, wenn die Zielkamera an ist.

### 7.4 Banner (D3)

`gradeBanner(player, before, extra)` hängt `extra` als eigene Zeile vor „{name} ist dran“ an. `extra` setzt sich zusammen aus:
- `tr('new_best')`, wenn `round.mode !== 'practice'`, `previousBest` existiert und `player.strokes < previousBest`; dazu `audio.effect('best', { delay: .9 })`;
- `fillText(tr('streak'), { n })`, wenn `n = underParStreak(player, round, manifest.courses) ≥ 3`.

Beide Teile werden mit ` · ` verbunden. Das Banner hat damit höchstens vier Zeilen.

### 7.5 Bedienklänge (D6)

- `click()`: Rundenkarte und Bahnkachel → `audio.ui('card')`.
- `change()`:
  - Kontrollkästchen → `audio.ui(element.checked ? 'switch-on' : 'switch-off')`;
  - `[data-quality]`, `[data-player-count]`, `[data-player-color]` → `audio.ui('select')`.

## 8. Neue Texte

| Schlüssel | en | de |
|---|---|---|
| `minigolf.to_cup` | {distance} to the cup | {distance} bis zum Loch |
| `minigolf.near_miss` | So close! | So knapp! |
| `minigolf.stuck` | The ball took too long. It stays where it is. | Der Ball war zu lange unterwegs und bleibt hier liegen. |
| `minigolf.limit_warning` | Only {n} strokes left on this hole. | Nur noch {n} Schläge auf dieser Bahn. |
| `minigolf.limit_last` | Last stroke on this hole! | Letzter Schlag auf dieser Bahn! |
| `minigolf.streak` | {n} holes in a row under par! | {n} Bahnen in Folge unter Par! |
| `minigolf.stars` | {n} of 3 stars | {n} von 3 Sternen |
| `minigolf.stars_total` | {n} of {max} stars | {n} von {max} Sternen |
| `minigolf.help_aim_body` (geändert) | Drag back from the ball like a slingshot. The further you pull, the harder the putt; the dotted line shows the path. Let go to putt. | Zieh vom Ball nach hinten weg wie bei einer Steinschleuder. Je weiter Du ziehst, desto kräftiger wird der Schlag; die gepunktete Linie zeigt den Weg. Lass los, um zu schlagen. |
| `minigolf.tutorial_power` (geändert) | The further you pull, the harder the putt. The dotted line shows the path. | Je weiter Du ziehst, desto kräftiger. Die gepunktete Linie zeigt den Weg. |

Wiederverwendet werden `new_best` („Neuer Bestwert!“), `best_value` und `tutorial_camera` (Kamera-Hinweis beim Schlagversuch).

## 9. Tests und Abnahme

- Einheitstests nach 3.7, 5.7 und 6.6, je Paket mit `node scripts/test-minigolf-<name>.mjs`.
- M1 zusätzlich `node scripts/test-minigolf-courses.mjs` (Prüfung des Zeitlimits aus 3.3).
- Browser-Test `TestDesktopMinigolfBrowser` (M5), neue Zusicherungen:
  - **A1:** Nach `tutorial === 'aim'` (Zeile ~380) wird `graphics.aimDots >= 3` per `MustWait` erreicht.
  - **A4:** In der HUD-Prüfung (Zeile ~231) ist `[data-plaque-meta]` sichtbar und enthält eine Ziffer und „m“.
  - **B5:** Vor dem Schlag über die Fußleiste (Zeile ~446) mit `page.Mouse.MustMoveTo(cameraX, cameraY).MustScroll(0, 600)` herauszoomen, höchstens fünfmal, bis `distance > 14`.
  - **A3:** Bei `busy` zeigt `.mg-chip[data-active] .mg-chip-score` den Wert `round.players[turn].strokes + 1`.
  - **B5, Fortsetzung:** Nach `!busy` und 600 ms gelten `distance > 8.5`, Ball mittig (`centered`) und `graphics.tween === false`.
  - **D5:** Danach Kamera-Knopf, Leertaste: `hud.toasts` enthält `tutorial_camera`, `busy` bleibt `false`; Kamera-Knopf zurück.
  - **D2:** Vor `golfFixture.reopen()` (Zeile ~474) erst `golfFixture.failStorage(false)`, dann `best.v2 = {version:2,set:'',rounds:{},holes:{'pirate-01':1}}` setzen.
    - Beim ersten `openHoles()`: `[data-hole-id="pirate-01"] .mg-hole-stars[data-stars="3"]` existiert.
    - Auf der Startseite gibt es `[data-round="pirate"] .mg-round-stars`.
  - Die bestehenden `layoutOK`-Läufe (1280 und 420 px, alle Themes) decken Plakette, Chips, Banner und Sternfelder ab. Der Test mit „Zum Ball“ (`distance <= 8.001`) bleibt unverändert grün.
- Hauptsitzung: `npm run build:minigolf` (Bündel unter 6 MiB), `npm run test:minigolf`, `go test ./ui/ -run 'TestDesktopMinigolf'`, Browser-Test mit `AURAGO_RUN_BROWSER_SMOKE=1`.
- Abnahme von Hand, Screenshots in 1280 und 420 px:
  - Ziellinie aus der Übersicht;
  - Lochring;
  - Beinahe-Treffer;
  - Ass mit Orbit und Feuerwerk;
  - Aus mit Pop;
  - Sterne auf Kacheln und Karten;
  - Banner mit Bestwert und Serie bei 420 px;
  - alles einmal unter reduzierter Bewegung.

## 10. Nachtrag: Hüpfer an der Lochkante

- Ursache (gemessen in Node gegen die echten Kurs-Collider): Die ferne Kante der Kollisions-Tasse ist die scharfe Oberkante einer 0,30 m tiefen, senkrechten Wand. Ein Ball, der das Loch zu schnell überquert, sinkt dabei ein paar Zentimeter und trifft die Kante unterhalb seiner Mitte. In einem festen Schritt macht der Solver aus Vorwärts- in Aufwärtsgeschwindigkeit (moon-13, 34 % Stärke: vx 4,49 auf 2,25, vy -0,48 auf +2,17 m/s). Bei Schwerkraft 4 ist das ein 0,73 m hoher Hüpfer; er überfliegt die 0,24 m hohe Kante am Ende der Bahn, und der Ball fällt von der Insel (`out`). Auf dem Mond ab etwa 30 % Stärke, mit Schwerkraft 9,81 ab etwa 45 % (`out` oder `water`, Strafschlag).
- Regel: `LIP_HOP = 0.03` (m, neben `CUP_SPEED` in `minigolf-physics.js`); in `createPhysics` ist `lipHopSpeed = sqrt(2 * gravity * LIP_HOP)`. Nach dem Welt-Schritt eines festen Schritts wird die Position VOR diesem Schritt betrachtet. Lag sie waagerecht über der Öffnung (Abstand zur Lochmitte < `cupOpening`), die Ballmitte über der Kantenebene (`y > cup[1]`) und höchstens bei `cup[1] + BALL_RADIUS + 0.04`, und ist die senkrechte Geschwindigkeit jetzt größer als `lipHopSpeed`, wird sie auf `lipHopSpeed` gesetzt. Horizontale Geschwindigkeit und Drall bleiben. Vor-Schritt-Position, weil ein Ball bei voller Kraft bis zu 0,14 m pro Schritt zurücklegt und nach dem Kick-Schritt schon hinter der Öffnung sein kann; das Höhenfenster lässt das Klappern im Loch (unter der Kantenebene) und Bälle nach einem Sprung unberührt.
- Sitz: ein gemeinsamer Helfer `capLipHop`, in `step()` nach `world.step`, Bumpern, Rollen und Zonen und vor `resolveConditions()` (nur im aktiven Schlag und nicht, solange eine Kanone den Ball hält), und in `predict()` an derselben Stelle der Schleife mit der Vor-Schritt-Position des Geisterballs, damit die Vorschau zeigt, was der echte Schlag tut.
- Vorher -> nachher (Endereignis; die Zahl ist die Stärke am Regler, 0 bis 1):
  - moon-13, Ball bei [13.5, 0.16, 0] auf das Loch gezielt: 0,12 / 0,16 / 0,20 `cup` -> `cup`; 0,25 `rest` -> `rest` (näher am Loch); 0,30 `out` -> `rest`; 0,34 `out` -> `rest`; 0,45 `out` -> `cup`; 0,60 `out` -> `rest`.
  - moon-16, Ball bei [8.65, 0.16, -13.29]: 0,45 `out` -> `rest`; 0,60 `out` -> `rest`.
  - pirate-01 (Schwerkraft 9,81), Ball bei [7.7, 0.16, -4.07]: 0,40 `rest` -> `rest`; 0,60 `water` -> `rest`; 0,80 `out` -> `rest`.
  - Langsame Bälle (Einfall ins Loch) sind unberührt: die Regel greift nur über der Kantenebene bei Aufwärtsgeschwindigkeit über `lipHopSpeed`.
- Tests: `scripts/test-minigolf-physics.mjs` (Becher-Fixture mit 0,30 m tiefer Wand, Schwerkraft 4: der Ball steigt nach Erreichen der Öffnung höchstens `LIP_HOP + 0.02` über die Rollhöhe, live und in `predict()`) und `scripts/test-minigolf-courses.mjs` (Überschüsse moon-13 mit 0,30 / 0,34 / 0,45 / 0,60, moon-16 mit 0,45 / 0,60, pirate-01 mit 0,60 / 0,80 enden in `rest` oder `cup`; alle 18 aufgezeichneten Routen unverändert).
