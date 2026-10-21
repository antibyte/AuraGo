# AuraGolf P3 – Bahnen-Baukasten & 18 Bahnen (Design)

- **Datum:** 2026-10-21
- **Status:** Entwurf, Grundlage der Umsetzung
- **Bezug:** [Fahrplan](2026-10-18-overhaul-roadmap-design.md), Abschnitte 3, 4.4, 5, 6 und 7; [P2-Design](2026-10-19-p2-look-welt-requisiten-design.md), Abschnitt 9
- **Umsetzungsplan:** [P3-Plan](2026-10-21-p3-bahnen-baukasten-plan.md)

P3 ersetzt die 18 umgekleideten P1-Bahnen durch 18 neu gebaute Bahnen: 6 Piratenbucht, 6 Dschungeltempel, 6 Mondstation. Grundlage ist ein Baukasten aus Bahnstücken mit exakt passender Kollision, neue Welt-Mechaniken in der Physik, Manifest Version 2 mit den Welt-IDs `pirate`, `temple`, `moon` und Kollision, die direkt aus dem Bahn-GLB gelesen wird. Die P1-Physikkonstanten bleiben: voller Schlag rollt 28 m auf ebenem Filz.

Die Entscheidungen in Abschnitt 2 hat die P3-Session auf Grundlage des freigegebenen Fahrplans getroffen. Wer eine davon anders haben möchte, ändert sie hier, bevor der Plan umgesetzt ist.

## 1. Ergebnis für Spielende

- 18 neue Bahnen mit 12–35 m Laufweg und Par 2–5. Jede Welt beginnt sanft und endet mit einer Finalbahn.
- Jede Bahn ist ein eigenes Diorama, so groß wie ihr Layout. Das Schiff, der Steinkopf und das Habitat stehen in voller modellierter Größe.
- Neue Bahnstücke: Gerade, Kurve, Steilkurve, Rampe mit Sprung, Hügel und Senke, Trichter, Tunnel, Gabelung mit zwei Routen, Brücke, Stufen; auf dem Mond zusätzlich Looping und Halfpipe.
- Neue Mechaniken: Bumper, Boost-Felder, Teleporter, eine Kanone als Abschussrampe, Rollstein, Pendel, Tore und ein drehender Arm. Auf dem Mond ist die Schwerkraft geringer.
- Wasser und Abgrund sind Gefahren. Wer hineinfällt, bekommt einen Strafschlag und spielt von der letzten Lage weiter, wie heute.
- Fast jede Bahn hat eine riskante Abkürzung: ein schmaler Steg ohne Bande, ein Sprung über Wasser, ein Tunnel hinter einem Tor oder ein Teleporter.
- Boost, Teleporter und Kanone haben eigene Klänge (aus P5) und Funken.
- Die Kamera rahmt größere Bahnen vollständig: Übersicht, Schatten und Wasser passen sich der Diorama-Größe an.
- Die Bahn- und Weltnamen sind in allen 16 Sprachen neu.

## 2. Entscheidungen

| Thema | Entscheidung | Begründung |
|---|---|---|
| Reihenfolge der Welten | Bahnen 1–6 Piratenbucht, 7–12 Dschungeltempel, 13–18 Mondstation | Reihenfolge des Fahrplans; die sonnige Bucht führt ein, der Mond mit Looping und geringer Schwerkraft schließt ab |
| Bahn-IDs | `<welt>-<NN>`, z. B. `pirate-01`, `moon-18` | Wie bisher Welt plus Nummer |
| Kollisionsdaten | Kein Kollisionsarray mehr im Manifest. Laufzeit und Tests lesen die Kollisionsnetze aus dem Binärteil des Bahn-GLB (Knoten mit Extra `aurago_collider`). Das Manifest trägt nur einen Fingerabdruck. | Eine Quelle für Bild und Physik, kein Duplikat (spart rund 3,2 MB Manifest), keine zweite Datei je Bahn. Erfüllt „Kollision weiter aus dem exportierten GLB, binär statt JSON“. |
| Verschweißen | Die Laufzeit fasst alle Kollisionsknoten derselben Oberfläche zu einem Trimesh zusammen und verschweißt bitgleiche Vertices | Nähte zwischen Bahnstücken liegen dann innerhalb eines Trimesh; `FIX_INTERNAL_EDGES` glättet sie |
| Bahnen als Daten | Je Welt ein reines Datenmodul `kit/course_data_<welt>.py` (nur Literale): Stückfolge, Mechaniken, Hindernisse, Hingucker und feste Requisiten | Kommentierbar, ohne Logik, vom Baukasten geprüft |
| Querschnitt | Einheitlich 12 Reihen über 1,72 m Spurbreite (Reihenabstand 0,1433 m), Bande 0,24 m hoch und 0,14 m dick | P1-Spurmaß; 12 Reihen reichen für Steilkurve, Halfpipe und Looping |
| Flächen (Pads) | Abschlag, Grün, Platz, Trichter, Gabelung und Zusammenführung sind Pads auf einem Raster im Reihenabstand; ihre Anschlüsse sind exakt die Querschnitte der Spur | Verschweißbare Anschlüsse ohne T-Stöße |
| Bodenoberflächen | Weiter nur `grass` (Filz) und `sand`. Holzstege, Steinplatten und Mond-Deck sind optisch eigene Knoten mit Kollisionsfläche `grass` | Keine neuen Rollwerte, keine neuen Rollgeräusche; Abstimmung mit P1/P5 entfällt |
| Bandenoberflächen | `wood` (Piraten), `stone` (Tempel), neu `metal` (Mond, Restitution 0,80) | P5 kennt `metal` bereits als Aufprallklang |
| Schwerkraft | Piraten und Tempel 9,81 m/s², Mond 4,0 m/s² (Feld `gravity` je Bahn) | Spürbar schwebend, Rollweite unverändert, Looping mit Boost sicher machbar |
| Hindernis-Takt | Alle beweglichen Hindernisse einer Bahn sind periodisch mit einem gemeinsamen Takt `cycle` (Sekunden) | Routensuche richtet Wartezeiten an einem Takt aus |
| Diorama | Superellipse wie in P2, aber Mittelpunkt und Halbachsen aus dem Layout (Grundfläche plus Requisitenrand); ein Platz für den Hingucker ist je Bahn fest vorgegeben, Maßstab ≥ 0,9 | Bestehende Insel-, Platzierungs- und Testregeln gelten weiter; Hingucker in voller Größe |
| Wasser im Inneren | Teiche und Kanäle sind eigene Wasserflächen (Material `water`) über einer Mulde, mit eigenem Gefahrenvolumen | Keine 1,2 m tiefen Gräben bis zum Meeresspiegel |
| Abgrund | Mond: Fallen über eine offene Kante führt unter `bounds.min.y` und zählt als `out` | Bestehende Regel, kein neuer Ereignistyp |
| Alte Bahnen | `kit/courses_v1.py` und `scripts/fixtures/minigolf-p1-colliders.json` entfallen | P2-Übergabe |

## 3. Die 18 Bahnen

Laufweg = Länge der Hauptroute (Mittellinie) vom Abschlag zum Loch. „Abkürzung“ ist die riskante Route.

### 3.1 Piratenbucht (`pirate`, Schwerkraft 9,81)

| Nr. | ID | Name (de / en) | Par | Laufweg | Stücke | Mechanik | Hingucker | Abkürzung |
|---|---|---|---|---|---|---|---|---|
| 1 | pirate-01 | Ankerbucht / Anchor Cove | 2 | ≈ 12 m | Gerade, Kurve, Grün | – | Schiff vor Anker neben dem Grün | – (Einführung) |
| 2 | pirate-02 | Fasskegeln / Barrel Bumpers | 3 | ≈ 17 m | Sandgerade, Sand-Steilkurve, Holzplatz mit seitlichem Ausgang, Kurve, Grün | 3 Fass-Bumper auf dem Platz | Kanone und Fässer | Bandenschuss zwischen den Bumpern |
| 3 | pirate-03 | Plankensteg / Plank Walk | 3 | ≈ 20 m | Gabelung, Brücke, Kurve, Sandbunker | – | Steg mit Booten und Leuchtturm | Planke ohne Bande quer über die Lagune |
| 4 | pirate-04 | Kanonenbucht / Cannon Cove | 4 | ≈ 26 m | Hügel, Stufen, Kurve, Rampe | Kanone | Schiff und Kanonenstellung | Präziser Schuss in die Kanone, die über die Lagune aufs Grün feuert |
| 5 | pirate-05 | Schmugglerhöhle / Smuggler's Cave | 4 | ≈ 28 m | Tunnel durch den Totenkopffelsen, Steilkurve | Ankerpendel, Teleporter (Luke → Fass) | Totenkopffelsen | Luke kurz hinter dem Abschlag |
| 6 | pirate-06 | Krakenstrudel / Kraken's Whirlpool | 5 | ≈ 34 m | Rampe mit Sprung, Brücke, Platz, Trichter | Zugbrücke (Tor), 2 Bumper | Schiff und Strudel | Sprung über die Lagune |

### 3.2 Dschungeltempel (`temple`, Schwerkraft 9,81)

| Nr. | ID | Name (de / en) | Par | Laufweg | Stücke | Mechanik | Hingucker | Abkürzung |
|---|---|---|---|---|---|---|---|---|
| 7 | temple-07 | Tempelstufen / Temple Steps | 3 | ≈ 16 m | Stufen (3 Absätze), Kurve, Grün | – | Steinkopf über den Stufen | Bandenschuss über die Stufenkante |
| 8 | temple-08 | Rollender Stein / Rolling Stone | 3 | ≈ 19 m | Gabelung (Umweg links, Senke rechts), Gerade, Kurve | Rollstein quer durch die Senke | Stufentor über dem Abschlag | Timing durch die Senke statt Umweg |
| 9 | temple-09 | Steinkopf / Stone Head | 4 | ≈ 24 m | Gabelung, Tunnel, Kurve | Steintor im Tunnel | Steinkopf mit Tunnelmaul | Tunnel durchs Maul hinter dem Tor |
| 10 | temple-10 | Pendelstämme / Swinging Logs | 4 | ≈ 26 m | Brücke über den Kanal, Steilkurve, Hügel | 2 Baumstamm-Pendel | Wasserfall | Brücke ohne Bande am Rand des Kanals |
| 11 | temple-11 | Heiliger Brunnen / Sacred Well | 4 | ≈ 26 m | Stufen, Kurve, Trichter | 2 Götzen-Portale (Teleporter) | Brunnen mit Totems | Portal über den Teich |
| 12 | temple-12 | Goldener Thron / Golden Throne | 5 | ≈ 34 m | Rampe hinauf, Steilkurve, Hügel, Stufen, Grün | 2 Tore im Wechseltakt, Rollstein | Steinkopf als Thron | Sprung von der oberen Terrasse |

### 3.3 Mondstation (`moon`, Schwerkraft 4,0)

| Nr. | ID | Name (de / en) | Par | Laufweg | Stücke | Mechanik | Hingucker | Abkürzung |
|---|---|---|---|---|---|---|---|---|
| 13 | moon-13 | Aufsetzer / Touchdown | 2 | ≈ 14 m | Gerade, Hügel, Grün | geringe Schwerkraft | Habitat | – (Einführung in die Schwerkraft) |
| 14 | moon-14 | Boost-Spur / Boost Lane | 3 | ≈ 22 m | Gerade, Steilkurve, Platz | 2 Boost-Felder, 2 Neon-Bumper | Parabolschüssel | Bandenschuss über die offene Kante |
| 15 | moon-15 | Kraterrinne / Crater Pipe | 3 | ≈ 22 m | Halfpipe in der Kurve, Gerade | – | Kraterwand und Rover | Sprung über den Abgrund |
| 16 | moon-16 | Beam-Station / Beam Station | 4 | ≈ 26 m | Platz, Kurve, Brücke | 2 Teleporter, drehender Radararm | Funkturm | Teleporter direkt aufs Grün |
| 17 | moon-17 | Schwerelos / Zero-G Leap | 4 | ≈ 30 m | Rampe mit Sprung, Kurve, Platz | Schotttore, 2 Bumper | Habitat und Solarfeld | Langer Sprung über die Schlucht |
| 18 | moon-18 | Orbit-Looping / Orbit Loop | 5 | ≈ 35 m | Boost, Looping, Gabelung, Trichter | Boost-Feld, Schotttor | Habitat, Rover, Erde | Halfpipe-Abkürzung statt Gabelung |

Jedes Bahnstück aus Abschnitt 4.3 kommt mindestens einmal vor, jede Mechanik aus Abschnitt 5.3 ebenfalls. Die genaue Stückfolge, Maße und Takte legen die Datenmodule fest; der Plan beschreibt das Abstimmen. Par und Laufweg dürfen beim Abstimmen innerhalb der Fahrplan-Grenzen (Laufweg 10–35 m, Par 2–5) abweichen; die Tabelle wird dann nachgezogen.

## 4. Bahnstück-Baukasten (`assets/minigolf/kit/`)

### 4.1 Module

| Modul | Aufgabe |
|---|---|
| `track.py` | Datentypen: `Cursor` (Position, Richtung, Höhe), `Section` (Querschnittpunkte, Bodennormale, Bande links/rechts), `Strip` (Folge von Sections), `Pad` (Rastergitter mit Höhenfunktion, Anschlüssen, Aussparungen), `Track` (alles einer Bahn); Netz-Ausgabe für Boden und Banden |
| `pieces.py` | Spurstücke: `straight`, `curve`, `bank`, `ramp`, `hill`, `steps`, `gap` |
| `pieces_moon.py` | `halfpipe`, `loop` |
| `pads.py` | `tee`, `green`, `plaza`, `funnel`, `fork` (Teilen und Zusammenführen) |
| `layout.py` | Liest eine Bahn aus dem Datenmodul, baut den `Track`, prüft die Regeln (4.4) und liefert Abschlag, Loch, Hauptroute, Korridore, Gefahren, Mechanik- und Hindernisdaten |
| `course_data_pirate.py`, `course_data_temple.py`, `course_data_moon.py` | Die 18 Bahnen als Daten |
| `build_track.py` | Baut aus einem `Track` die Blender-Objekte: Boden je Oberfläche und Optik, Banden, Loch, Abschlag, Fahne, Bahnnummer |
| `mechanic_parts.py`, `obstacle_parts.py` | Sichtbare Teile der Mechaniken und der beweglichen Hindernisse je Welt |
| `courses.py` | Ersetzt `courses_v1.py`: baut, kleidet, backt und exportiert eine Bahn, liefert den Manifest-Eintrag |

Bestehende Module werden verallgemeinert statt ersetzt: `dress.py`, `dress_island.py` (Terrain), `trim.py` und `placement.py` nehmen einen `Track` statt der alten Einzelspur; `export.py` liest Kollider über `aurago_collider`. Kein Modul wächst über etwa 400 Zeilen.

### 4.2 Datenmodell einer Bahn

```python
COURSES = [{
    "number": 1, "par": 2, "name": "Anchor Cove",
    "start": {"at": (0.0, 0.0, 0.0), "heading": 90},     # Blender XY in m, Grad
    "pieces": [
        ("tee", {}),
        ("straight", {"length": 4.0}),
        ("curve", {"radius": 3.5, "angle": 70}),          # positiv = links
        ("green", {"length": 3.6, "width": 3.44, "cup": (0.0, 2.2)}),
    ],
    "hero": {"at": (-4.5, 6.0), "yaw": 200},
    "props": [("dock", (5.0, -1.0), 90, 1.0)],            # feste Platzierungen
    "mechanics": [], "obstacles": [], "cycle": None,
}]
```

- `pieces` ist die Hauptroute vom Abschlag zum Loch. Eine Gabelung trägt ihre zwei Zweige als eigene Stücklisten (`("fork", {"left": [...], "right": [...], ...})`); beide Zweige enden parallel, auf gleicher Höhe und gleichem Vorlauf, sonst bricht der Bau mit einer Meldung ab.
- Optionale Stückfelder: `surface` (`grass`/`sand`), `look` (`felt`, `deck`, `slab`, `metal` – nur Optik), `walls` (`both`, `left`, `right`, `none`), `cover` (`tunnel`), `deck` (`bridge`), `grounded` (Erde folgt der Spur; Vorgabe ja bis 0,3 m über Grund).
- Mechaniken und Hindernisse verweisen auf benannte Anker (`("anchor", {"name": "a1"})` in der Stückfolge) oder auf feste Koordinaten.
- Freie Teiche: `"ponds": [{"polygon": [(x, y), ...], "kind": "water", "z": 0.0}]` legt eine Lagune in Weltkoordinaten neben, zwischen oder unter die Spur (z. B. unter einen Steg); sie wirkt wie ein Platz-Teich (Mulde, Wasserfläche, Gefahrenvolumen), darf aber keine Spur schneiden, die weniger als 0,5 m über dem Grund liegt.

### 4.3 Bahnstücke

Alle Spurstücke fegen einen Querschnitt entlang einer Mittellinie. Ein Stück übernimmt den letzten Querschnitt seines Vorgängers als ersten eigenen Querschnitt (dasselbe Objekt, keine Neuberechnung), deshalb liegen die Vertices an Stücknähten bitgleich übereinander.

| Stück | Parameter | Geometrie |
|---|---|---|
| `straight` | `length` | 2 Querschnitte, bei Höhenverlauf alle ≤ 0,35 m |
| `curve` | `radius` ≥ 2,0, `angle` | Kreisbogen, Querschnitt alle ≤ 0,30 m Bogenlänge |
| `bank` | wie `curve` plus `bank` (Querneigung in Grad, ≤ 30) | Neigung steigt sanft an und fällt bis zum Ende auf 0 |
| `ramp` | `length`, `rise` (±), optional `lip` (Absprungwinkel) | S-förmiger Höhenverlauf; mit `lip` endet sie steiler für Sprünge |
| `hill` | `length`, `height` (positiv Hügel, negativ Senke) | Cosinus-Höcker |
| `steps` | `count`, `drop`, `tread` | Ebene Absätze mit senkrechten Setzstufen (Teil des Bodennetzes) |
| `gap` | `length`, `hazard` (`water`/`void`) | Kein Boden; der Cursor springt weiter; darunter ein Gefahrenvolumen |
| `halfpipe` (Mond) | `length` oder `radius`/`angle`, `width`, `depth` | U-Querschnitt; Übergang vom flachen Querschnitt über ≥ 1,5 m; Banden auf den Lippen |
| `loop` (Mond) | `radius` (1,2–1,5 m), `offset` (seitlich ≥ 2,2 m) | Senkrechter Kreis mit seitlichem Versatz; Banden zeigen entlang der Bodennormale |

Pads:

| Pad | Parameter | Geometrie |
|---|---|---|
| `tee` | `length` | Kurzes ebenes Startfeld, Abschlagmarke |
| `green` | `length`, `width`, `cup`, optional `slope`, `contour` | Rasterfläche mit Loch; Flächen im Lochradius + 0,13 m entfallen wie in P1 |
| `plaza` | `length`, `width`, `pools` (Aussparungen mit Wasser/Abgrund) | Breite Fläche für Bumper und Zwei-Wege-Plätze |
| `funnel` | `radius`, `depth`, `cup` (Mitte) | Polares Netz; der äußere Ring trägt im Eingangsbogen genau die Punkte des ankommenden Querschnitts |
| `fork` | `left`, `right`, `spread` | Teilungsfläche, zwei Zweige, Zusammenführungsfläche aus den Zweigenden |

Pad-Raster laufen im Reihenabstand der Spur; Anschlüsse liegen auf Rasterlinien und verwenden die Querschnittpunkte der Spur. Banden entstehen an jeder Kante ohne Anschluss, außer wo `walls: none` gilt.

### 4.4 Regeln, die der Baukasten prüft

- Ball-Lichtraum: über jeder Bodenfläche mindestens 0,5 m frei bis zur nächsten Geometrie (außer Tunneldach ≥ 0,45 m).
- Kurvenradius ≥ 2,0 m, Steilkurve ≤ 30°, Rampen ≤ 22° (mit Lippe ≤ 30°), Höhenstufe einer Treppe ≤ 0,35 m.
- Looping: Radius 1,2–1,5 m, seitlicher Versatz ≥ Spurbreite + 2 × Bande + 0,2 m; davor ein Boost-Feld oder ≥ 8 m gerader Anlauf.
  Messung (Physik-Prototyp, 48 Querschnitte, Schwerkraft 4,0, Radius 1,3 m): Das Boost-Feld (9 m/s) muss höchstens etwa 1,5 m vor dem Looping liegen; dann gelingt Schlag 0,2 (Mindestschlag, um das Feld zu erreichen) ebenso wie 0,3. Sechs Meter davor reicht der Schwung nicht (der Ball löst sich vor dem Scheitel). Ohne Boost trägt erst Schlag 0,55 über 8 m Anlauf; Schlag 0,15 bleibt im Anlauf liegen. Ein Boost von 8 m/s reicht knapp, 7 m/s nicht; 9 m/s lässt Spielraum. Radius 1,2 und 1,5 m verhalten sich gleich.
- Gabelungszweige enden parallel (Richtung ± 0,01°), auf gleicher Höhe (± 1 mm) und mit Querabstand als Vielfaches des Reihenabstands.
- Laufweg 10–35 m, Loch auf einem `green` oder in einem `funnel`.
- Alles liegt innerhalb der Insel; Abschlag, Loch, Mechaniken, Landepunkte und Hindernisse innerhalb von `bounds`.

### 4.5 Ausgabe in Blender

- **Boden:** je Optik ein Knoten (`floor-felt`, `floor-sand`, `floor-deck`, `floor-slab`, `floor-metal`) mit Extra `aurago_collider` = `grass` oder `sand`. Felt bekommt wie in P2 Punktfarben, Deck Bretterfugen, Slab Plattenfugen, Metal Riffelblech als Farbverlauf. Flach schattiert; Vertices an Nähten bitgleich.
- **Banden:** je Oberfläche ein Kollisionsknoten `rail-wood`, `rail-stone` oder `rail-metal` mit `aurago_collider` und `aurago_hidden` (unsichtbar), dazu die sichtbare Bandenverkleidung `rail-trim-*` aus `trim.py`, bündig an der Innenseite wie in P2. Banden folgen der Bodennormale, auch im Looping und in der Halfpipe.
- **Brücken und Tunnel:** Brückenpfeiler, Geländerpfosten und Tunnelbögen sind Deko aus `special_parts.py`/Kit; sie liegen außerhalb des Ball-Lichtraums. Der Totenkopffelsen und der Steinkopf werden als Tunnelportale über die Spur gesetzt (Kit-Requisit bzw. handgebaute Hülle mit Öffnung ≥ 1,9 m breit, ≥ 1,0 m hoch).
- **Loch, Abschlag, Fahne, Bahnnummer, Vorlagen:** wie P2 (`dress_parts.restyle`), Lochboden aus der Inselfunktion.
- **Insel:** `island.Outline` (Superellipse) mit Mittelpunkt und Halbachsen aus der Grundfläche aller Spurstücke und Pads plus 2,5–4 m Rand; Halbachsen bis 20 m. Rastergröße der Insel wächst mit der Fläche (Zelle ≈ 0,35 m). Die Oberfläche folgt als Bett 6 cm unter Spuren mit `grounded`; unter erhöhten Stücken (Brücke, Rampenende, Looping) bleibt der Grund auf Basishöhe und Stützen tragen die Spur.
- **Gefahren:** Teiche und Kanäle sind Mulden im Inselgrund mit einer Wasserfläche (Material `water`) und einem Gefahrenvolumen bis 2 cm unter die Wasserfläche. Auf dem Mond sind Schluchten Löcher durch die Insel; darunter liegt nur `bounds`.
- **Platzierung:** Hingucker an der vorgegebenen Stelle, Maßstab 1,0 (mindestens 0,9, wenn Rand und Korridor es verlangen), danach feste Requisiten, Rand und Streudeko wie in P2. Der Korridor gilt für alle Routen (Hauptroute, Zweige, Abkürzungen). Jedes Kit-Requisit wird in mindestens einer Bahn platziert; auch die in P2 ungenutzten (Piraten `dock`, `buoy`, `flagpole`, `pebble`; Tempel `islet`, `lilypad`, `sunwheel`, `stairs`, `vines`; Mond `beacon`, `pad`).
- **Export:** wie P2 (`join_scenery`, kanonische Namen, Bake mit Kit-Verdeckern). Funktionsknoten bleiben einzeln: `floor-*`, `rail-*`, `rail-trim-*`, `obstacle-*`, `mechanic-*` (nur wenn animiert), `flag`, Vorlagen, `water-shore`. Höchstens 32 Mesh-Knoten je Bahn.

### 4.6 Mechanik- und Hindernisteile je Welt

| Teil | Piraten | Tempel | Mond |
|---|---|---|---|
| Bumper | Fass mit Eisenringen | Steinpilz mit Goldband | Neon-Pylon |
| Boost-Feld | – | – | Leuchtende Pfeilplatte (Material `glow`) |
| Teleporter | Luke im Boden → Fass mit Öffnung | Götzen-Portal (Steinbogen) | Transporterpad mit Ring |
| Kanone | Große Kanone mit Fangtrichter am Verschluss | – | – |
| Rollstein | – | Steinkugel | – |
| Pendel | Anker an Kette | Baumstamm an Lianen | – |
| Tor | Zugbrücke (Hebetor) | Steintür | Schotttor |
| Drehender Arm | – | – | Radararm |

Jedes bewegliche Teil ist ein Knoten `obstacle-<id>`; seine Kollisionsform steht im Manifest (Abschnitt 6) und ist exakt die Grundform des sichtbaren Kerns (Quader, Kugel, Zylinder). Gefasen bleiben innerhalb dieser Form.

## 5. Physik und Mechaniken (`ui/js/desktop/apps/`)

### 5.1 Module

| Modul | Aufgabe |
|---|---|
| `minigolf-collision.js` (neu) | Liest aus einem GLB (`ArrayBuffer`) alle Knoten mit `aurago_collider`, rechnet Weltkoordinaten (Knotenhierarchie), gruppiert je Oberfläche, verschweißt bitgleiche Vertices und liefert `[{ type: 'mesh', surface, vertices: Float32Array, indices: Uint32Array }]`. Ohne DOM und three.js, in Node testbar. Dazu `colliderDigest()` für den Fingerabdruck. |
| `minigolf-mechanics.js` (neu) | Reine Funktionen für Hindernis-Posen (`obstaclePose(spec, t)`) und Mechanik-Zonen (Boost, Teleporter, Kanone, Bumper-Stoß). Live-Schritt und Vorschau nutzen denselben Code. |
| `minigolf-physics.js` | Welt, Schritt, Schlag, Vorschau; nimmt typisierte Arrays an; Schwerkraft aus `course.gravity` (Vorgabe 9,81); nicht-terminale Ereignisse. |

### 5.2 Ereignisse

Terminal bleiben `rest`, `cup`, `water`, `out`. Neu und nicht terminal: `bumper`, `boost`, `teleport`, `cannon` (`{ type, position, id }`), zusätzlich zu `impact`. Der Schlag bleibt dabei aktiv.

### 5.3 Mechaniken (Manifest `mechanics`)

| Art | Felder | Verhalten |
|---|---|---|
| `bumper` | `id`, `position` (Fußmitte), `radius`, `height`, `surface`, `kick` | Zylinder-Kollider, Restitution 1,0 (Max-Regel). Nach dem Kontakt hat der Ball waagerecht mindestens `kick` m/s (typisch 5) vom Bumper weg. Ereignis `bumper`. |
| `boost` | `id`, `position`, `half` [hx, hy, hz], `yaw`, `direction` [dx, dz], `speed` | Gedrehte Box-Zone. Beim Eintritt wird die waagerechte Geschwindigkeit auf `direction · max(speed, |v|)` gesetzt; die senkrechte bleibt. Einmal je Eintritt. Ereignis `boost`. |
| `teleport` | `id`, `position`, `radius`, `exit` [x, y, z], `direction`, `minSpeed` | Scheiben-Zone (Höhe 0,4 m). Der Ball erscheint bei `exit` mit waagerechter Geschwindigkeit `direction · max(minSpeed, |v|)`; Darstellungs-Interpolation wird zurückgesetzt; 0,5 s keine erneute Auslösung. Ereignis `teleport`. |
| `cannon` | `id`, `position`, `radius`, `muzzle`, `velocity` [vx, vy, vz], `delay` | Scheiben-Zone. Der Ball wird gefangen, `delay` s (typisch 0,6) an der Mündung gehalten und mit `velocity` abgefeuert. Der Generator berechnet `velocity` aus Flugzeit und Landepunkt. Ereignis `cannon` beim Abfeuern. |

Die Vorschau (`predict`) wendet Boost und Bumper an und endet an Teleporter- oder Kanonen-Zonen mit `{ zone: 'teleport'|'cannon' }`.

### 5.4 Bewegliche Hindernisse (Manifest `obstacles`)

| Art | Bewegung |
|---|---|
| `rotate` | Pendeln um `axis`: Winkel = `amplitude · sin(speed·t + phase)` (Pendel: waagerechte Achse, Teile unterhalb des Drehpunkts) |
| `slide` | Schieben entlang `axis`: `amplitude · sin(speed·t + phase)` |
| `spin` (neu) | Drehen um `axis`: Winkel = `speed·t + phase` |
| `gate` (neu) | Schieben mit Haltephasen: `amplitude · clamp(sharp · sin(speed·t + phase), −1, 1)`, `sharp` 1–4 |
| `boulder` (neu) | Schieben wie `slide`; Drehung um die waagerechte Querachse mit Winkel = Weg / `radius` |

Kollisionsteile `colliders[]`: `{ shape: 'box', position, halfExtents, rotation? }`, `{ shape: 'ball', position, radius }` oder `{ shape: 'cylinder', position, radius, halfHeight, rotation? }`; ohne `shape` gilt wie heute ein Quader. Für jede Bahn mit Hindernissen gilt `cycle · speed / (2π)` ganzzahlig für jedes Hindernis.

### 5.5 Schwerkraft und Rollen

- `gravity` setzt die Weltschwerkraft; Rollwiderstand und Abschussgeschwindigkeit bleiben (28 m auf ebenem Filz auch auf dem Mond).
- Im Looping und an steilen Halfpipe-Wänden gibt es keinen Bodenkontakt nach unten; der Ball gleitet dort ohne Rollwiderstand. Das ist gewollt (kein Energieverlust an der Wand).

## 6. Manifest Version 2 (`ui/3d/minigolf/manifest.json`)

```jsonc
{
  "version": 2,
  "courses": [{
    "id": "pirate-01", "number": 1, "world": "pirate", "par": 2,
    "model": "course-01.glb", "sha256": "…", "bytes": 0, "triangles": 0,
    "tee": [x, y, z], "cup": [x, y, z], "cupRadius": 0.32,
    "gravity": 9.81, "cycle": null, "length": 12.4,
    "bounds": { "min": [], "max": [] },
    "water": [{ "min": [], "max": [] }],
    "collision": { "nodes": 3, "triangles": 0, "digest": "sha256…" },
    "obstacles": [ … ], "mechanics": [ … ],
    "path": [[x, y, z], …], "corridors": [[[x, y, z], …], …],
    "props": [[name, x, y, z, yaw, scale], …],
    "island": { "center": [x, z], "half": [hx, hz], "power": 2.6, "wobble": 0.03 }
  }],
  "kits": { … },
  "worlds": [{ "id": "pirate", "preview": "previews/pirate.webp" }, …]
}
```

- Entfällt: `colliders`, `look` (die Welt-ID ist der Look). `lookOf()` liest `world` direkt.
- `collision.digest`: SHA-256 über die gruppierten, verschweißten Kollider in Oberflächen-Reihenfolge `grass, sand, wood, stone, metal`: je Gruppe Oberflächenname, Vertexzahl, Indexzahl (Uint32 LE), Float32-Vertices LE, Uint32-Indizes LE. Python (nach dem Export) und `minigolf-collision.js` rechnen ihn identisch.
- `corridors`: Mittellinien aller befahrbaren Routen (Hauptroute, Zweige, Abkürzungen); für Platzierungs- und Asset-Test. `path` bleibt die Hauptroute für die Kamerafahrt.
- `length`: Laufweg der Hauptroute in m.
- Das Spiel prüft `version === 2`, 18 Bahnen und `course-NN.glb`.

## 7. Spiel, Szene, Audio und Texte

- **Laden:** `scene.load(course)` holt das GLB, liest daraus mit `minigolf-collision.js` die Kollider und gibt `{ colliders }` zurück (wahr). Das Spiel erzeugt die Physik mit `{ ...course, colliders }` und gibt dasselbe Objekt an die Karte (`minimap` akzeptiert typisierte Arrays).
- **Ereignisse:** `boost`, `teleport`, `cannon` rufen `audio.effect(type, …)` und einen Funken-Effekt (`scene.burst('spark', …)`) auf; `teleport` bricht die Leuchtspur ab; `bumper` nutzt den vorhandenen Aufprall.
- **Rahmung:** Übersichtspose, Schattenkamera (mindestens ±9 m) und Wasserscheibe richten sich nach `island` bzw. `bounds`. Die Kamerafahrt bleibt bei 0,35 s/m, höchstens 4,5 s.
- **Drehende Bahnteile:** Knoten mit Extra `aurago_spin: { axis, speed }` drehen sich ohne Physik (Teleporterring, Radarschüssel), außer bei reduzierter Bewegung.
- **Welt-IDs:** `WORLD_ALIAS` in `minigolf-audio.js` und `LEGACY_LOOK` in `minigolf-looks.js` entfallen. `WORLD_ICONS` und Akzente behalten nur `pirate`, `temple`, `moon`.
- **Texte:** neue Schlüssel `minigolf.pirate`, `minigolf.temple`, `minigolf.moon` (die alten `minigolf.garden|coast|night` entfallen), neue Werte für `minigolf.course_1` bis `course_18` in allen 16 Sprachen, echte Übersetzungen statt Englisch. Deutsch mit echten Umlauten.
- **Spielstände:** Neue Bahn-IDs machen alte `round.v2`-Stände ungültig; sie fallen wie bisher still weg. `best.v2` verwirft Runden über `set`. Keine Migration.

## 8. Budgets

| Größe | Grenze |
|---|---|
| Laufzeit-Assets gesamt | ≤ 32 MiB (bis 40 MiB nur mit Begründung im README, wenn sonst die Qualität leidet) |
| Bahn-GLB | ≤ 1,25 MiB, ≤ 36 000 Dreiecke, ≤ 32 Mesh-Knoten (angehoben für Looping- und Halfpipe-Bahnen; die Gesamtgrenze bleibt) |
| Manifest | ≤ 256 KB |
| Instanzen je Bahn | ≤ 220 |
| Kit-GLB, Requisit, Vorschaubild | wie P2 |
| Spiel-Bundle | ≤ 6 MiB |
| Bildrate je Stufe | nicht schlechter als nach P2 (Benchmark, gleiche Maschine) |

## 9. Tests (schlank)

- **`scripts/test-minigolf-physics.mjs`** ergänzt: Schwerkraft je Bahn; Bumper-Stoß; Boost setzt die Geschwindigkeit; Teleporter versetzt und meldet; Kanone fängt, hält und feuert deterministisch; neue Hindernisarten folgen ihrer Formel und der Vorschau; Looping-Prototyp (Ball mit Boost durchläuft, ohne Boost rollt er zurück); Ereignisse bleiben nicht terminal.
- **`scripts/test-minigolf-collision.mjs`** (neu, in `npm run test:minigolf`): Leser gegen ein kleines Test-GLB (Hierarchie, Matrix/TRS, Verschweißen, Digest).
- **`scripts/test-minigolf-courses.mjs`:** 18 Bahnen aus Manifest v2 und GLB-Kollision; je Bahn eine aufgezeichnete Route bis ins Loch, Routenlänge ≤ Par + 1. Die Suche (`AURAGO_MINIGOLF_SEARCH=1`, optional `AURAGO_MINIGOLF_COURSE=<id>`) bewertet Fortschritt entlang `corridors`/`path` statt Luftlinie und richtet Wartezeiten am `cycle` aus.
- **`scripts/test-minigolf-assets.mjs`:** Budgets; Manifest v2; Digest aus dem GLB gleich Manifest; Kollisionsknoten unsichtbar bzw. Boden sichtbar; offenes Loch (kein Bodendreieck im Lochradius); Abschlag über Boden; `path` und `corridors`; Platzierungen (Insel, Korridor, bekanntes Requisit, Hingucker ≥ 0,9); jedes Kit-Requisit mindestens einmal platziert; Mechaniken und Hindernisse in `bounds`; `cycle` ganzzahlig; keine nummerierten Namen; Vorschaubilder.
- **Browser-Smoke `TestDesktopMinigolfBrowser`:** Manifest v2 lädt, je Welt eine Bahn mit Kollision aus dem GLB, Kamerafahrt, keine externen Anfragen; vorhandene Prüfungen angepasst.
- `npm run test:minigolf`, `node scripts/build-minigolf.mjs --check`, `npm run build:ui -- --check`, `TestDesktopMinigolf*`, `TestShippedWebAssetsLoadNothingExternal`, i18n-Prüfungen bleiben grün.

## 10. Risiken

| Risiko | Gegenmaßnahme |
|---|---|
| Ball springt an Stücknähten | Querschnitte werden weitergereicht statt neu berechnet; Laufzeit verschweißt je Oberfläche; Test „kein Hopser > 1 cm“ auf einer Nahtstrecke |
| Looping hält den Ball nicht | Prototyp-Messung zuerst (Plan Aufgabe 2); Boost davor; Radius ≤ 1,5 m |
| Routensuche findet keine Route | Fortschrittsbewertung entlang der Route; Suche je Bahn; Abstimmen von Sprungweiten und Kanonenbahn mit festen Testschüssen |
| Größere Dioramen kosten Bildrate | Instanzgrenze, Zellgröße der Insel, Benchmark je Stufe |
| Schatten werden auf großen Inseln grob | Schattenkamera auf die Insel zugeschnitten; Stufe „hoch“ darf die Schattenkarte vergrößern |
| Generator dauert länger | `--course N`; voller Lauf im Hintergrund |
| Konflikte mit parallelen Sessions | Eigene Pfade stagen, vor dem Landen rebasen; Konfliktzonen klein halten (`minigolf-game.js`, `minigolf-scene.js`, Sprachdateien) |

## 11. Nicht Teil von P3

- Neue Rollgeräusche für Holz- oder Metallböden (Böden bleiben `grass`/`sand`).
- Bahneditor, Online-Modus, Migration alter Stände.
- Neue Kit-Requisiten über den Bedarf der Mechaniken hinaus.
