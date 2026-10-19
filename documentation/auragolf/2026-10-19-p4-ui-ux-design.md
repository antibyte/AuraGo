# AuraGolf P4 – UI/UX (Design)

- **Datum:** 2026-10-19
- **Status:** Entwurf zur Durchsicht
- **Bezug:** [Fahrplan](2026-10-18-overhaul-roadmap-design.md), Abschnitte 3, 4.5, 5 und 7
- **Umsetzungsplan:** [P4-Plan](2026-10-19-p4-ui-ux-plan.md)

P4 baut die Oberfläche von AuraGolf neu: HUD, Menüs, Auswertung, Einführung und Hilfe. Es setzt auf P1 auf (Stärke in Bildschirm-Pixeln, Laufbahnvorschau, Abbruch des Zugs) und läuft parallel zu P2 (Look), P3 (Bahnen) und P5 (Sound). P4 ändert weder Physik noch Bahnen noch Sound. Bis P2 gerenderte Weltbilder liefert, zeigen die Weltkarten gezeichnete Platzhalter.

Die Entscheidungen in Abschnitt 2 hat die P4-Session auf Grundlage des freigegebenen Fahrplans getroffen. Wer eine davon anders haben möchte, ändert sie hier, bevor der Plan umgesetzt ist.

## 1. Ergebnis für Spielende

- Das Spielfeld zeigt immer Bahnnummer, Par, Bahnname und für jeden Spieler die Schläge auf dieser Bahn. Wer am Zug ist, ist hervorgehoben.
- Beim Zielen erscheint neben dem Ball eine große, runde Stärkeanzeige mit Prozentwert und geschätzter Rollweite („54 %“, „≈ 11 m“).
- Bei jedem Spielerwechsel blendet ein Banner „Lin ist dran“ in der Spielerfarbe ein. Zu Beginn jeder Bahn nennt es Bahn, Par und Bahnname.
- Ereignisse wie Wasser, Aus, Einlochen oder neuer Bestwert erscheinen als kurze Einblendungen und verschwinden von selbst.
- Eine Übersichtskarte zeigt den echten Bahnverlauf mit Banden, Wasser, Loch, beweglichen Hindernissen und allen Bällen.
- Das Startmenü zeigt Karten: alle 18 Bahnen, je eine Karte pro Welt mit Vorschaubild und Bestwert, Training. Training öffnet ein Bahnraster mit Bestwerten je Bahn.
- Jeder Spieler wählt Name und Farbe. Die Farbe gilt überall gleich: Ball, Leuchtspur, Geisterball, Karte, Chips, Scorecard, Siegertreppchen.
- Nach jeder Bahn baut sich die Scorecard animiert auf. Am Rundenende steht ein Siegertreppchen, bei einem Spieler eine Ergebniskarte. „Nochmal spielen“ startet dieselbe Runde mit denselben Spielern neu.
- Die erste Bahn führt Schritt für Schritt durch den ersten Schlag, direkt im Spiel statt auf einer Textseite. Die Einführung lässt sich überspringen und aus der Hilfe neu starten.
- Die Hilfe ist in jeder Fenstergröße über den ?-Knopf in der Kopfleiste und über die ?-Taste erreichbar. Nach dem Schließen geht es genau dorthin zurück, wo man war.
- Farben, Flächen, Ecken und Schrift folgen dem gewählten Desktop-Theme. Jede Welt bringt eigene Akzentfarben mit. Text ist mindestens 13 px groß.

## 2. Entscheidungen

| Thema | Entscheidung | Begründung |
|---|---|---|
| Schrift (Fahrplan §7) | Geist, bereits lokal in `ui/fonts/` (OFL-1.1, in `licences/UEBERSICHT.md`). AuraGolf nutzt `var(--vd-font-ui, 'Geist', system-ui, sans-serif)`; Ziffern mit `font-variant-numeric: tabular-nums`. | Kein Download, kein neues Lizenzpaket, kein zusätzliches Byte. Folgt Themes, die eine eigene UI-Schrift setzen. |
| Settings-Schlüssel (Fahrplan §5, §7) | Bleibt `aurago.desktop.minigolf.settings.v1`. Eine Funktion `normalizeSettings()` prüft jedes bekannte Feld einzeln; beim Schreiben bleiben unbekannte Felder erhalten (Abschnitt 9.3). | Lautstärken, Stummschaltung, Grafik und Hilfen bleiben gültig; P5 sagt zu, dass Lautstärken und Stummschaltung gespeichert bleiben. P5 ergänzt seine Felder, ohne P4-Zeilen anzufassen. |
| Spielstand und Bestwerte | Neue Schlüssel `…round.v2` und `…best.v2` (Abschnitt 9). Alte `v1`-Daten verfallen mit einmaligem Hinweis, keine Migration. | Spielstände tragen jetzt eine Spielerfarbe, Bestwerte zusätzlich je Bahn. Entspricht Fahrplan §5. |
| Namenslänge | Eine Konstante `MAX_NAME_LENGTH = 24` für Eingabefeld, Bereinigung, Modell-Prüfung und 3D-Namensschild. | Behebt 34 gegenüber 40 (Fahrplan §1.6). 24 Zeichen passen in Chips, Scorecard und Treppchen. |
| Spielerfarben | 8 Farben in einer gemeinsamen Palette in `minigolf-shared.js`, genutzt von Modell, HUD und 3D. Vorgabe für Spieler 1–4 sind die bisherigen Farben; Spieler 1 ist überall `#fff0c5`. | Behebt `#fff0c5` gegenüber `0xfff6df` (Fahrplan §1.6). Zwei Spieler können nicht dieselbe Farbe haben. |
| Theme-Anbindung | Flächen, Ränder, Texte, Ecken und Schatten kommen aus den Desktop-Tokens, Rückfallwerte jeweils als zusammenpassender Satz (Abschnitt 8.1). Keine Selektoren mehr für einzelne Theme-Namen. | Alle Desktop-Themes definieren diese Tokens, auch die sieben Themes auf `feature/desktop-themes`. Neue Themes funktionieren ohne Änderung an AuraGolf. |
| Weltakzente | Je Welt-ID ein Satz CSS-Variablen (Abschnitt 8.2) für alte (`garden`, `coast`, `night`) und neue IDs (`pirate`, `temple`, `moon`) plus neutral. | P3 tauscht die Welten, ohne CSS anfassen zu müssen. |
| Weltliste und Vorschaubilder | Die Weltkarten kommen aus `courses[].world` in der Reihenfolge des ersten Auftretens. Optional liefert `worlds: [{ id, preview }]` im Manifest ein Vorschaubild (`preview` relativ zu `/3d/minigolf/`). Fehlt es oder lädt das Bild nicht, zeigt die Karte einen gezeichneten Platzhalter. | Klare Übergabe an P2 ohne Umbau des Menüs. Das Manifest erzeugt `assets/minigolf/generate.py`; P2 muss das Feld dort ausgeben, nicht von Hand. |
| Rundenmodi | `full`, `practice` und jede Welt-ID aus dem Manifest. Das Modell prüft nur das Format der Welt-ID; welche Welten es gibt, entscheidet das Spiel anhand des Manifests. | Die Liste `garden/coast/night` ist heute an vier Stellen fest verdrahtet; P3 müsste sie sonst überall ändern. |
| Übersichtskarte | Wird aus `course.colliders` (Dreiecksnetze mit `surface`), `course.water` und `course.obstacles` gezeichnet. Boden/Bande nach derselben Regel wie die Physik (`FLOOR_SURFACES`). | Echter Bahnverlauf ohne zusätzliche Assets. Die heutige Karte erwartet `bounds` als Array und wird deshalb nie angezeigt. |
| Einführung | Nicht-modale Coach-Marks im Spiel, Version `tutorial: 2` in den Settings. | Wer die alte Textseite kannte, sieht die neue Einführung einmal. |
| Zeitsteuerung | Alle Zeiten (Banner, Einblendungen, Ausblenden der Stärkeanzeige) laufen über die Uhr der bestehenden `frame()`-Schleife; Animationen sind CSS. Kein `setTimeout`, kein `setInterval`, keine zweite rAF-Schleife. | Pausiert automatisch mit Fenster und Tab; nichts muss beim Schließen gesondert aufgeräumt werden. |
| Spiel-Bundle | Limit in `scripts/build-minigolf.mjs` sofort auf die im Fahrplan beschlossenen 6 MiB. | Heute sind ca. 177 KB frei; P2, P3, P4 und P5 bringen alle Code dazu. |

## 3. Aufbau

### 3.1 Module

Alle Module liegen in `ui/js/desktop/apps/` und werden über `minigolf-game.js` in das Spiel-Bundle `ui/js/vendor/minigolf/game.esm.js` gebaut.

| Modul | Inhalt | Status |
|---|---|---|
| `minigolf-shared.js` | Farbpalette, `MAX_NAME_LENGTH`, `cleanName()`, `uniqueNames()`, `colorValue()`, Farbstopps der Stärkeanzeige (`POWER_STOPS`), `FLOOR_SURFACES`, Regler der Auto-Qualität. Ohne three.js und Rapier, daher in Node testbar. | neu |
| `minigolf-views.js` | Grundgerüst: Kopfleiste, Bühne, Fußleiste, Panel-Verwaltung, Fokusfalle, Symbole, `dispose()` | neu geschrieben |
| `minigolf-hud.js` | Lochplakette, Spielerchips, Stärkeanzeige, Banner, Einblendungen, Hinweiszeile, Live-Region, Format von Stärke und Weite | neu |
| `minigolf-minimap.js` | Übersichtskarte, reine Funktion `laneGeometry(course)` | neu |
| `minigolf-menus.js` | Startseite mit Rundenkarten, Bahnraster, Spieler-Setup, Einstellungen, Hilfe, Pause, Credits, Fehler | neu |
| `minigolf-scorecard.js` | Scorecard, Siegertreppchen, Ergebniskarte, reine Funktion `rankPlayers()` | neu |
| `minigolf-tutorial.js` | Zustandsmaschine der Einführung und ihre Coach-Marks | neu |
| `minigolf-model.js` | Rundenformat v2 (`createRound`, `validateSave`, `cloneSave`), Farbprüfung, Modi, Bestwerte (`normalizeBest`, `recordHole`, `recordRound`) | geändert |
| `minigolf-game.js` | Verdrahtung, Speicherung, Ereignisse, Tastatur, Rücksprungziel der Panels, Fokus | geändert |
| `minigolf-scene.js` | `screenPosition()`, getrenntes Beenden und Abbrechen des Zugs, Farben, Linienabstände, Auto-Qualität | klein geändert |
| `minigolf-physics.js` | importiert `FLOOR_SURFACES` aus `minigolf-shared.js` statt einer eigenen Konstante | eine Zeile |
| `ui/css/desktop-app-minigolf.css` | komplett neu gegliedert (Abschnitt 8) | neu geschrieben |

Alle Templates bleiben Template-Strings mit `innerHTML`; jeder dynamische Text läuft durch `esc()`. Symbole sind eigene Inline-SVGs in `minigolf-views.js` (kein Icon-Font, keine externen Dateien).

### 3.2 Ebenen

```
.mg-app  (container-type: inline-size; data-world, data-panel, data-reduced)
├─ header.mg-topbar      Marke · Rundenfortschritt · Knöpfe (Scorecard, Einstellungen, Hilfe, Pause)
├─ .mg-stage
│  ├─ .mg-canvas         three.js-Canvas
│  ├─ .mg-hud            Lochplakette, Spielerchips, Karte, Stärkeanzeige, Banner, Hinweis
│  ├─ .mg-coach          Einführung (über dem HUD, unter Panels)
│  ├─ .mg-overlay        Panels (Menü, Einstellungen, Hilfe, Pause, Scorecard, Fehler)
│  ├─ .mg-toasts         Einblendungen, über den Panels
│  └─ .mg-announcer      unsichtbare Live-Region (role=status, aria-live=polite)
└─ footer.mg-controls    Stärke-Regler mit Wert · Schlagen · Zum Ball · Kamera
```

- Ist ein Panel offen (`data-panel` nicht leer), sind HUD und Coach-Marks verborgen. Einblendungen bleiben sichtbar, weil sie über den Panels liegen (z. B. der Hinweis aus 9.4 beim Start im Menü).
- Die Kopfleiste bleibt bei offenem Panel bedienbar. Panels sind `role="dialog"` ohne `aria-modal`; die Fokusfalle läuft über Panel **und** Kopfleiste. Knöpfe ohne Sinn im Zustand (Scorecard und Pause im Menü) sind deaktiviert, nicht versteckt.
- Breiten über Container-Queries auf `.mg-app`: **breit** > 800 px, **mittel** 561–800 px, **schmal** ≤ 560 px.
- `view.dispose()` entfernt Wurzel und Listener; das Spiel ruft es in seiner bestehenden Aufräumliste auf.

### 3.3 Kopfleiste

- Links Marke: Symbol und „AuraGolf“; schmal nur das Symbol.
- Mitte: Rundenfortschritt „Bahn 4/18“ und eine Fortschrittsleiste aus Segmenten (fertig, aktuell, offen) in Weltakzent; im Menü der Untertitel.
- Rechts vier Symbolknöpfe, ab breit mit sichtbarer Beschriftung, darunter nur Symbol mit `aria-label` und `title`: Scorecard, Einstellungen, Hilfe, Pause. **Der Hilfe-Knopf ist in jeder Breite sichtbar.**
- Alle Knöpfe mindestens 40 × 40 px.

### 3.4 Fußleiste

- Stärke-Regler (`data-power`, 2–100) mit Wertanzeige (`data-power-value`) im Format aus 4.3, Schlagen (primär), Zum Ball, Kamera (`aria-pressed`).
- Knöpfe bestehen aus Symbol und Beschriftungs-`span`. Das Spiel ändert sie nur über `view.setCamera(pressed)`, nie über `textContent`.
- Die Spielerleiste der Fußleiste entfällt; ihre Aufgabe übernehmen die Spielerchips.
- Schmal: Regler über volle Breite in der ersten Zeile, die drei Knöpfe darunter; Zum Ball und Kamera nur als Symbol mit `aria-label`.

## 4. HUD

### 4.1 Lochplakette (oben links)

- Zeile 1: Weltname als Überschrift in Großbuchstaben (13 px, Weltakzent als Streifen links).
- Zeile 2: „Bahn 4 · Par 3“ (18 px, fett).
- Zeile 3: Bahnname, einzeilig mit Auslassungspunkten. Er ist in jeder Breite sichtbar (behebt „Bahnname ab 800 px ausgeblendet“).

### 4.2 Spielerchips (oben rechts)

- Je Spieler ein Chip: Farbpunkt mit dunklem Rand, Name (Auslassungspunkte, max. 160 px), Schläge auf dieser Bahn.
- Am Zug: Ring in `--mg-focus` (Theme-Akzent) plus Zusatz „am Zug“. Die Spielerfarbe allein reicht nicht, weil die helle Farbe von Spieler 1 auf hellen Themes kaum sichtbar ist. Fertig: Haken und Ergebnis zu Par („Par“, „−1“, „+2“).
- Ein Spieler: ein Chip „Schläge 3“.
- Schmal: Chips als Zeile; nur der Chip am Zug zeigt den Namen, die anderen Farbpunkt und Schläge.

### 4.3 Stärkeanzeige am Ball

- Rundes SVG-Messgerät, 96 px (schmal 76 px): Hintergrundring, Wertbogen in der Farbe der 3D-Stärkeringe (gemeinsame Farbstopps `POWER_STOPS`), in der Mitte der Prozentwert (22 px, fett), darunter ein Chip mit der Weite.
- Format über `formatPower(strength, meters, lang)` mit `Intl.NumberFormat`; das Spiel übergibt `document.documentElement.lang`:
  - Stärke: `style: 'percent'` → de „54 %“, en „54%“.
  - Weite: „≈ 11 m“; „≈“, Zahl und Einheit durch geschützte Leerzeichen (U+00A0) getrennt. Rundung wie bisher: unter 1 m auf 0,1 m, unter 10 m auf 0,5 m, sonst ganze Meter.
  - Dieselben Strings stehen am Regler in der Fußleiste („54 % · ≈ 11 m“). Behebt die Abstände aus P1.
- Sichtbar, solange die Stärke gerade verändert wird (Ziehen, gehaltene Taste, Regler) und 1,2 s danach; nie während der Ball rollt, im Kameramodus oder bei offenem Panel.
- Position: Bildschirmposition des Balls aus `scene.screenPosition()`, um 56 px nach rechts und 64 px nach oben versetzt; passt es nicht, wird nach links bzw. unten gespiegelt und in die Bühne geklemmt. Ist der Ball nicht sichtbar, steht die Anzeige unten mittig über der Fußleiste. Aktualisierung per `transform` nur, solange sichtbar.
- `aria-hidden="true"`; der zugängliche Wert ist der Regler.

### 4.4 Banner

- Oben mittig, Text 20 px fett, Farbstreifen in Spielerfarbe.
- Beginn jeder Bahn: „Bahn 4 · Par 3“, darunter der Bahnname, bei mehreren Spielern zusätzlich „Ada ist dran“. 2,2 s.
- Spielerwechsel bei mindestens zwei Spielern: „Lin ist dran“. 1,6 s.
- Ein neues Banner ersetzt ein laufendes. Der Text geht zusätzlich an die Live-Region.

### 4.5 Einblendungen und Hinweiszeile

Einblendungen liegen in eigener Ebene über den Panels (3.2), unten mittig, höchstens drei gleichzeitig, die älteste weicht. Klick schließt sie. Die Einblendungen selbst sind `aria-hidden`; ihr Text geht an die Live-Region.

| Schlüssel | Art | Dauer |
|---|---|---|
| `sunk` | Erfolg | 3 s |
| `new_best` | Erfolg | 6 s; zusätzlich Abzeichen auf Scorecard, Treppchen bzw. Ergebniskarte |
| `water`, `out` | Warnung | 3,5 s |
| `limit` | Warnung | 5 s |
| `quality_changed` | Info | 4 s |
| `audio_error` | Warnung | 6 s |
| `storage_error` | Warnung | 8 s, einmal je Sitzung |
| `progress_reset` | Info | 8 s, einmalig (Abschnitt 9.4) |

- Hinweiszeile unten links (13 px, gedämpft): „Zieh vom Ball zurück und lass los.“ solange gezielt werden kann und in dieser Sitzung weniger als drei Schläge gespielt wurden. Während der Einführung ausgeblendet.
- „Lädt …“ ist keine Einblendung mehr, sondern eine Ladeanzeige in der Bühnenmitte.

### 4.6 Übersichtskarte

- Platz: breit und mittel oben links unter der Lochplakette, 140 × 140 px (mittel 120). Schmal liegen von oben nach unten Lochplakette, Chipzeile, Karte (96 × 96 px).
- Ein Knopf an der Karte (`data-action="map"`, `aria-pressed`) blendet sie ein und aus; der Zustand wird als `showMap` gespeichert. Das ist die einzige Stelle für diese Einstellung. Ausgeblendet bleibt ein runder Kartenknopf.
- Darstellung, Norden oben, x nach rechts, z nach unten (Abschlag meist unten, Loch oben):
  - Boden: Dreiecke der Mesh-Kollider, deren `surface` in `FLOOR_SURFACES` liegt (heute `grass`, `sand`) und deren Normale `|n.y| ≥ 0,55` hat (unabhängig von der Dreiecksreihenfolge). Sand in eigener Farbe.
  - Banden: Dreiecke aller anderen Mesh-Kollider mit `|n.y| ≥ 0,55`, dunkler.
  - Wasser: Rechtecke aus `course.water` (`{min, max}`).
  - Abschlag als kleines Quadrat, Loch als Ring mit Punkt.
  - Bewegliche Hindernisse: je Teil aus `obstacle.colliders` ein gedrehtes Rechteck (Teilposition und -halbmaße, gedreht mit der Physik-Pose aus `physics.getObstacles()`); ohne Teile die Halbmaße des Hindernisses. So erscheint das Windmühlentor als Kreuz.
  - Bälle der Spieler, die die Bahn noch nicht beendet haben, in Spielerfarbe mit dunklem Rand; der Ball am Zug größer und mit hellem Ring.
- Ausschnitt: Begrenzungsrahmen der Bodendreiecke plus 0,5 m, Seitenverhältnis erhalten. Ohne Bodendreiecke Rückfall auf `course.bounds` (`{min, max}`).
- Technik: zwei Canvas-Ebenen in Gerätepixeln. Die Bahn wird einmal je Bahn gezeichnet, die Ebene mit Bällen und Hindernissen nur, wenn sich gerundete Positionen geändert haben.
- `role="img"`, `aria-label` „Bahnkarte“.
- `laneGeometry(course)` ist eine reine, exportierte Funktion (Rückgabe: Rahmen, Boden-, Banden- und Wasserflächen in Weltkoordinaten).

## 5. Menüs

Das Panel `menu` hat drei Schritte: `home`, `holes`, `players`. Die Auswahl (Rundenmodus, Bahn, Spieler, Farben) lebt im Spiel und überlebt Schrittwechsel sowie Ausflüge in Hilfe und Einstellungen.

### 5.1 Startseite (Schritt `home`)

- Kopf: „AuraGolf“, Unterzeile (Tagline).
- Fortsetzen-Karte, wenn ein gültiger Spielstand existiert: „Runde fortsetzen“, darunter „Bahn 7 von 18 · Spieler: 2“ und der Rundenname; Knopf `data-action="resume"`.
- Überschrift „Runde wählen“, darunter Karten (Knöpfe mit `data-round`):
  - `data-round="full"`: „Alle 18 Bahnen“, breite Karte mit den Akzentstreifen aller Welten, Bestwert der vollen Runde.
  - Je Welt (Abschnitt 2, Weltliste) `data-round="<welt-id>"`: Vorschaubild, Weltname, „6 Bahnen · Par 18“, Bestwert oder „Noch kein Bestwert“.
  - `data-round="practice"`: „Training“ mit „Eine Bahn frei wählen“.
- Fußnote „Dein Fortschritt wird in diesem Browser gespeichert.“
- Breit: Weltkarten dreispaltig; mittel zweispaltig; schmal einspaltig mit kleinem Vorschaubild links.

### 5.2 Weltkarten und Vorschaubilder

- Vorschaufläche im Verhältnis 16:9.
- Mit Vorschaubild: `<img alt="" loading="lazy" decoding="async">` über `assetURL()`. Bei Ladefehler wird die Karte auf den Platzhalter umgeschaltet.
- Platzhalter: Farbverlauf aus den Weltakzenten, stilisierte Inselform (CSS) und ein Weltsymbol als Inline-SVG: Blume (`garden`), Welle (`coast`), Laterne (`night`), Segelschiff (`pirate`), Stufentempel (`temple`), Mondsichel mit Planet (`moon`), Stern (unbekannt, `full`).
- Fokus- und Hover-Zustand: Rahmen in Weltakzent, Anhebung um 2 px (nicht bei reduzierter Bewegung).

### 5.3 Bahnraster (Schritt `holes`)

- Kopf „Bahn zum Üben wählen“, Knopf Zurück.
- Nach Welten gruppiert, je Gruppe eine Überschrift mit Akzentstreifen.
- Kacheln (`data-hole-id="<course-id>"`): große Nummer, Bahnname (zweizeilig, abgeschnitten), „Par 3“, Bestwert der Bahn („Bestwert 2“) oder „–“.
- Breit sechs Spalten (eine Zeile je Welt), mittel vier, schmal drei.

### 5.4 Spieler-Setup (Schritt `players`)

- Kopf „Wer spielt?“, darunter die gewählte Runde („Alle 18 Bahnen“, Weltname oder „Training: 4. Sanfter Hügel“).
- Anzahl als Segmentgruppe aus Radio-Eingaben (`data-player-count`, Werte 1–4).
- Je Spieler ein `fieldset` mit Legende „Spieler 1“:
  - Farbwahl: Radiogruppe aus 8 Farbfeldern (`data-player-color`, Wert = Farb-ID), 32 px sichtbar, Trefferfläche 44 px, `aria-label` = Farbname. Farben anderer Spieler sind deaktiviert.
  - Name: Textfeld (`data-player-name`, `maxlength` 24).
- Vorbelegung: Namen und Farben der laufenden oder letzten Runde, sonst „Spieler N“ und die Standardfarbe N. Kommt ein Spieler dazu und ist seine Standardfarbe vergeben, bekommt er die erste freie Farbe.
- Knöpfe: Zurück (`data-action="back"`), Los geht’s (`data-action="start"`, primär).
- Bereinigung beim Start (`cleanName`, `uniqueNames`): Steuerzeichen entfernen, trimmen, auf 24 Codepunkte kürzen (`Array.from`), erneut trimmen, leer → „Spieler N“. Doppelte Namen (ohne Groß-/Kleinschreibung) bekommen in einer Schleife „ 2“, „ 3“ … angehängt, bis der Name eindeutig ist; der Stamm wird dafür so gekürzt, dass 24 Codepunkte nicht überschritten werden. Die Oberfläche übergibt `createRound()` nie Eingaben, die es ablehnen kann.

### 5.5 Weitere Panels

- **Einstellungen:** Gruppen „Ton“ (Musik, Titel, Weiter, Stumm, Effekte), „Grafik“ (Qualität), „Steuerung“ (Zielhilfe, Kamera hinter dem Ball, Ball folgen), „Anzeige“ (reduzierte Bewegung), Knopf Credits. Bestehende `data-*`-Haken bleiben unverändert. Die Gruppe „Ton“ rendert eine eigene Funktion `audioSettings(settings, track)`, deren Inhalt P5 gehört; P4 gestaltet sie nur. Der Einstieg bleibt `settings(settings, track)`.
- **Hilfe:** Abschnitte mit Symbol und Überschrift: „Zielen und Schlagen“, „Tastatur“ (Tabelle mit `<kbd>`-Chips, siehe Abschnitt 11), „Touch“, „Wertung“. Knopf „Einführung erneut starten“ (`data-action="tutorial-again"`, nur sichtbar, wenn eine Runde läuft) und Schließen.
- **Pause:** „Pause“, Hinweis zum Speichern, Knöpfe Weiter (primär), Hilfe, Hauptmenü.
- **Credits, Fehler:** Inhalt wie heute, neue Gestaltung.
- **Rücksprungziel:** Öffnen von Hilfe, Einstellungen oder Credits merkt sich `{ panel, menuStep }` (eine Ebene; Credits kehren zu den Einstellungen zurück, diese dann zum gemerkten Ziel). Schließen stellt genau diesen Zustand wieder her: Menüschritt mit Auswahl, Pause, Scorecard oder das laufende Spiel.
- Alle Panels behalten Fokusfalle (3.2) und Rücksprung des Fokus. Escape in `holes`/`players` führt einen Schritt zurück.

## 6. Auswertung

### 6.1 Scorecard (Panel `scorecard`)

- Kopf: „Bahn 4 geschafft“ bzw. „Runde beendet“; mitten auf der Bahn „Scorecard“.
- Tabelle: Spieler × Bahnen, dazu Summe und „±Par“. ±Par zählt nur beendete Bahnen (Summe der Ergebnisse minus Summe ihrer Par-Werte; „Par“, „−2“, „+3“ mit echtem Minuszeichen U+2212). Spaltenkopf der aktuellen Bahn hervorgehoben. Bewertungsfarben wie heute (Eagle, Birdie, Par, Bogey, darüber) plus Text im `title`.
- Erste Spalte mit Farbpunkt und Name bleibt beim waagrechten Scrollen stehen (`position: sticky`).
- Animation (nur CSS) beim Öffnen nach einer Bahn: Zeilen erscheinen versetzt (60 ms), die Zellen der gerade beendeten Bahn und die Summen ploppen auf (Skalierung 0,6 → 1, 80 ms versetzt).
- Knöpfe: Nächste Bahn (primär); mitten auf der Bahn Schließen; am Ende siehe 6.2/6.3.

### 6.2 Siegertreppchen (Rundenende, ab zwei Spielern)

- `rankPlayers(players)` sortiert nach Gesamtschlägen; Gleichstand teilt den Platz (1, 1, 3).
- Drei Stufen (2 links, 1 Mitte, 3 rechts), Höhe 64/88/48 px (schmal 48/64/36), Oberkante in Spielerfarbe, darüber Ballscheibe, Name, Summe, ±Par. Platz 4 steht als Zeile darunter.
- Überschrift „Ada gewinnt!“ bzw. „Unentschieden!“, Abzeichen „Neuer Bestwert!“ wenn gesetzt.
- Animation: Stufen wachsen nacheinander (3., 2., 1.) von unten (CSS `animation-delay`), danach erscheint der Siegername. Die Szene feuert wie heute `burst('win')`.
- Knöpfe: Nochmal spielen (`data-action="again"`, primär), Hauptmenü.

### 6.3 Ergebniskarte (Rundenende, ein Spieler)

- Volle Runde und Welt: Gesamtschläge groß, ±Par, Bestwert der Runde; „Neuer Bestwert!“ als Abzeichen, wenn er gerade gesetzt wurde. Knöpfe Nochmal spielen (`again`, primär), Hauptmenü.
- Training (auch mit mehreren Spielern statt Treppchen): Schläge auf der Bahn je Spieler, ±Par, Bestwert der Bahn. Knöpfe Nochmal (`again`, primär), Andere Bahn (`data-action="holes"`, öffnet das Bahnraster mit denselben Spielern), Hauptmenü.
- `again` legt eine neue Runde mit gleichem Modus, gleicher Bahn (Training), gleichen Namen und Farben an und startet sie ohne Umweg über das Menü.

## 7. Interaktive Einführung

Nicht-modal: Das Spiel bleibt bedienbar, `panelKind` bleibt leer, der Fokus wird nicht verschoben.

| Schritt | Text (Kern) | Tastatur-Zusatz | Anker | Weiter, wenn |
|---|---|---|---|---|
| `aim` | Zieh vom Ball nach hinten weg – wie bei einer Steinschleuder. | ←/→ zielen | über dem Ball | ein Zug beginnt (`onAim` mit Phase `drag`) oder ←/→ gedrückt wird |
| `power` | Je weiter du ziehst, desto kräftiger. Die gestrichelte Linie zeigt den Weg. | ↑/↓ Stärke | über dem Ball | die Stärke wurde durch Spielende auf mindestens 20 % gestellt (Zug, Taste oder Regler) |
| `release` | Lass los, um zu schlagen! | Leertaste schlägt | über dem Ball | ein Schlag startet |
| `watch` | Gut! Warte, bis der Ball liegt. | – | oben mittig | der Schlag ist aufgelöst (Ruhe, Loch, Wasser, Aus) |
| `done` | Geschafft! Die Hilfe findest du jederzeit hier. | – | am ?-Knopf der Kopfleiste | Knopf „Los geht’s“, der nächste Schlag oder ein geöffnetes Panel |

- Jeder Schritt hat „Überspringen“ (`data-action="tutorial-skip"`). Ein Schlag in einem früheren Schritt springt direkt zu `watch`. Abbruch eines Zugs (Phase `cancel`) setzt den Fortschritt nicht zurück.
- Schritt `aim` zeigt eine kleine Zug-Animation (Punkt, der vom Ball nach hinten gleitet); bei reduzierter Bewegung ein statischer Pfeil.
- Im Kameramodus sind die Schritte `aim` bis `release` verborgen; stattdessen steht dort „Zum Zielen den Kameramodus verlassen“.
- Ist der Ball nicht sichtbar (`screenPosition().visible` falsch), steht die Sprechblase oben mittig.
- Ablauf: Startet nach dem Laden einer Runde (Start, Fortsetzen, Nochmal), wenn `settings.tutorial < 2`, und begleitet den nächsten Schlag des Spielers am Zug. Ein neuer Rundenstart setzt eine laufende Einführung zurück. Abschluss oder Überspringen setzt `tutorial = 2` und speichert.
- „Einführung erneut starten“ in der Hilfe schließt die Hilfe und startet bei `aim`, sobald gezielt werden kann.
- Ein offenes Panel blendet die Coach-Marks aus; danach erscheinen sie wieder (außer `done`, siehe Tabelle).
- Zugänglichkeit: Container `role="status"` mit `aria-live="polite"` für den Schritttext; die Knöpfe sind normal per Tab erreichbar. Verschwindet ein Knopf, der den Fokus hatte, geht der Fokus auf die Spielfläche.

## 8. Gestaltung

### 8.1 Theme-Anbindung

Lokale Variablen auf `.mg-app`. Die erste Wahl sind die `--vd-theme-*`-Tokens der Shell, die zweite die `--vd-*`-Grundtokens aus `desktop-base.css` (dort steht `--vd-text` im selben Block wie `--vd-surface`), zuletzt warme Werte, die untereinander zusammenpassen:

| AuraGolf | 1. Wahl | 2. Wahl | Rückfall |
|---|---|---|---|
| `--mg-bg` | `--vd-theme-app-bg` | `--vd-bg` | `#c9e2d6` |
| `--mg-surface` | `--vd-theme-panel-bg` | `--vd-surface` | `#fff9ec` |
| `--mg-surface-strong` | `--vd-theme-panel-bg-strong` | `--vd-surface-strong` | `#fff9ecf2` |
| `--mg-chrome` | `--vd-theme-chrome-bg` | `--vd-surface` | `#fff9ecf2` |
| `--mg-text` | `--vd-text` | – | `#244c40` |
| `--mg-muted` | `--vd-theme-muted` | `--vd-muted` | `#55695f` |
| `--mg-border` | `--vd-theme-border` | `--vd-border` | `#285b4833` |
| `--mg-control` / `--mg-control-hover` | `--vd-theme-control-bg` / `--vd-theme-control-hover` | – | `#fff9ec` / `#f8e9ce` |
| `--mg-overlay` | `--vd-theme-overlay-bg` | – | `#203a3233` |
| `--mg-radius` / `--mg-radius-lg` | `--vd-theme-radius-panel` / `--vd-theme-radius-modal` | – | `12px` / `18px` |
| `--mg-shadow` | `--vd-theme-shadow-modal` | – | weicher Schatten |
| `--mg-focus` | `--vd-accent` | – | `#d98a3d` |
| `--mg-font` | `--vd-font-ui` | – | `'Geist', system-ui, sans-serif` |

- HUD-Elemente über der 3D-Szene nutzen `--mg-surface-strong` mit leichter Transparenz und Schatten, damit sie auf hellen und dunklen Welten lesbar bleiben.
- Die heutigen Sonderregeln für `standard` und `fruity` (zwei identische 30-Token-Blöcke) entfallen.
- Der Browser-Test lädt dieselben Shell-Styles wie der Desktop (`desktop-shell.bundle.css`), damit Text- und Flächen-Tokens zusammenpassen. Seine Inline-Regeln für Host und Fenster bleiben; der Test prüft, dass der Host weiterhin das ganze Fenster füllt.

### 8.2 Weltakzente

`data-world` auf `.mg-app` (im Spiel) bzw. auf jeder Karte und Gruppe (im Menü) setzt:

| Welt-ID | `--mg-accent` | `--mg-accent-2` | `--mg-accent-ink` | Symbol |
|---|---|---|---|---|
| neutral / `full` | `#df914f` | `#f2c46b` | `#2b1503` | Stern |
| `garden` | `#6fb35a` | `#c9e39a` | `#0f2a0c` | Blume |
| `coast` | `#2fa3b5` | `#9fdde4` | `#04232a` | Welle |
| `night` | `#8a83e0` | `#f2cf6b` | `#120e2e` | Laterne |
| `pirate` | `#12b3ad` | `#f0d08a` | `#032321` | Segelschiff |
| `temple` | `#d6a03c` | `#4f9a54` | `#2a1a03` | Stufentempel |
| `moon` | `#a66bff` | `#46dcff` | `#150530` | Mondsichel |

- Primärknöpfe: Hintergrund `--mg-accent`, Text `--mg-accent-ink`. Kontrast mindestens 4,5 : 1; einmalig bei der Umsetzung nachgerechnet.
- Weitere Einsatzorte: Streifen der Lochplakette, Fortschrittssegmente, Kartenrahmen, Bannerkante.

### 8.3 Schrift und Größen

- Geist über `--mg-font`. Mindestgröße für jeden sichtbaren Text 13 px (Fahrplan: 13–14 px). Fließtext 14 px, Panel-Überschriften 24–28 px, Titel „AuraGolf“ 44 px (schmal 36 px).
- Keine Großbuchstaben-Überschrift unter 13 px; Buchstabenabstand höchstens 0,08 em.

### 8.4 Bewegung

- Das Spiel setzt `data-reduced="true"` auf `.mg-app`, wenn reduzierte Bewegung wirksam ist (Einstellung, Betriebssystem oder `data-animations="false"` am Desktop). (Nicht `data-motion`: so heißt bereits das Kontrollkästchen in den Einstellungen.) Unter diesem Attribut und unter `@media (prefers-reduced-motion: reduce)` gibt es keine Übergänge oder Animationen: Banner, Einblendungen, Karten, Scorecard und Treppchen erscheinen sofort.
- Dauer der Animationen ≤ 400 ms, Treppchen insgesamt ≤ 1,2 s.

## 9. Spielstände, Bestwerte, Einstellungen

### 9.1 Spielstand `aurago.desktop.minigolf.round.v2`

- Wie v1, aber `version: 2` und je Spieler zusätzlich `color` (Farb-ID aus der Palette, eindeutig in der Runde).
- `createRound()` nimmt Spieler als Strings (Standardfarbe nach Position) oder als `{ name, color }`; Namen 1–24 Zeichen; `mode` ist `full`, `practice` oder eine Welt-ID im Format `/^[a-z][a-z0-9-]{0,23}$/`.
- `validateSave()` erwartet `version: 2` und das Spielerfeld `color` (die Schlüsselprüfung `safeKeys` verlangt die exakte Schlüsselmenge), `cloneSave()` kopiert `color` mit. Das Modell importiert die Palette aus `minigolf-shared.js`.
- Das Spiel verwirft beim Laden Spielstände, deren Modus weder `full` noch `practice` noch eine Welt des Manifests ist.

### 9.2 Bestwerte `aurago.desktop.minigolf.best.v2`

```json
{ "version": 2, "set": "coast-07,…,night-18", "rounds": { "full": 41, "garden": 14 }, "holes": { "garden-01": 1 } }
```

- `rounds`: beste Gesamtschläge je Rundenmodus (`full` und Welt-IDs), wie heute nur außerhalb des Trainings.
- `holes`: beste Schläge je Bahn (1–13), aus allen Modi einschließlich Training, sobald alle Spieler die Bahn beendet haben.
- `set`: die sortierten Bahn-IDs des Manifests, durch Komma getrennt. Passt `set` nicht zum aktuellen Manifest, verfallen alle `rounds` (eine volle Runde auf neuen Bahnen ist nicht vergleichbar); `holes` bleiben für Bahnen, die es noch gibt.
- `normalizeBest(raw, courses)` behält nur gültige Einträge: Bahn-IDs und Welt-IDs aus dem Manifest, Ganzzahlen in Grenzen.

### 9.3 Einstellungen `aurago.desktop.minigolf.settings.v1`

| Feld | Typ | Vorgabe | Neu |
|---|---|---|---|
| `music`, `effects` | 0–1 | 0,2 / 0,65 | |
| `quality` | `auto`/`low`/`medium`/`high` | `auto` | |
| `reducedMotion`, `follow`, `aimCamera`, `magnet` | bool | false / true / false / true | |
| `tutorial` | Ganzzahl 0–2 | 0 | ja; ersetzt `tutorialDone` (`true` wird als 1 gelesen) |
| `showMap` | bool | true | ja |

- `normalizeSettings(stored)` liefert die geprüften bekannten Felder, eine Zeile je Feld.
- Geschrieben wird `{ ...gespeicherteFelder, ...normalisiert }` ohne `tutorialDone`; unbekannte Felder anderer Teile bleiben so erhalten. Felder aus P5 ergänzt P5 als eigene Zeilen.

### 9.4 Einmaliger Hinweis

Findet das Spiel beim Start `round.v1` oder `best.v1`, aber keinen der v2-Schlüssel, zeigt es einmal die Einblendung „AuraGolf wurde überarbeitet. Alte Spielstände und Bestwerte wurden zurückgesetzt.“ und entfernt beide v1-Schlüssel. Die Einblendung ist über dem Startmenü sichtbar (3.2). Ohne Speicherzugriff entfällt der Hinweis still.

## 10. Szene, Spielablauf und Kleinfehler

- `screenPosition(position)` (neu, optional aufrufbar): `{ x, y, visible }` in CSS-Pixeln relativ zur Bühne; `visible` ist false hinter der Kamera oder außerhalb.
- `onAim(direction, power, info)`: `info.phase` ist `'drag'` beim Ziehen und `'cancel'`, wenn ein abgebrochener Zug das vorherige Ziel wiederherstellt.
- **Zug beenden und abbrechen (P1-Rest):** Beim Drücken merkt sich die Szene Richtung und Stärke. Es gibt zwei getrennte Funktionen:
  - `endDrag()` gibt nur den Zeiger frei. `up()` ruft sie bei einem gültigen Loslassen und feuert danach `onShot` mit dem gezogenen Ziel.
  - `cancelDrag()` gibt den Zeiger frei, stellt Richtung und Stärke wieder her und meldet sie mit Phase `cancel`. Aufrufer: Escape (über das Spiel), Rechtsklick und zweiter Finger in `down()`, der `!canAim`-Zweig in `move()` und Loslassen innerhalb der Totzone.
- **Linienabstände (P1-Rest):** `fillLine` berechnet die Strichabstände selbst in ein vorab angelegtes `lineDistance`-Attribut (Länge `PREVIEW_POINTS`) statt `computeLineDistances()` aufzurufen, das bei jeder Zieländerung ein neues Array anlegt.
- **Auto-Qualität (Fahrplan §1.6):** Ein kleiner reiner Regler `createQualityGovernor()` in `minigolf-shared.js` entscheidet; die Szene ruft ihn mit wenigen Zeilen auf, damit P2s Qualitätsstufen-Arbeit kaum berührt wird. Gewählte Qualität (`quality`) und wirksame Stufe (`tier`) sind getrennt. `auto` beginnt bei `medium`. Abstieg auf `low` wie heute (gemessene FPS < 38, frühestens 6 s nach der letzten Änderung, mit Einblendung). Aufstieg zurück auf `medium` ohne Einblendung, wenn ca. 10 s lang ≥ 55 FPS gemessen wurden und die letzte Änderung mindestens 12 s zurückliegt. `stats()` liefert `quality` (gewählt) und zusätzlich `tier`.
- **Farben (Fahrplan §1.6):** Die Konstante `COLORS` in der Szene entfällt. Der Schnappschuss aus `frame()` trägt die Farb-ID des Spielers am Zug (`color`), im Menü die Standardfarbe von Spieler 1. Die Szene bildet IDs über `colorValue()` an allen drei Stellen ab: `setBall()`, Leuchtspur-Partikel in `update()` und `setGhosts()` (Farbe aus `player.color`).
- **Namensschild:** kürzt mit `MAX_NAME_LENGTH`.
- **Toter Schutz (Fahrplan §1.6):** `snapTowardCup` prüft `player.done` statt der nie gesetzten Felder `holed`/`finished`.
- **Wertanzeige (P1-Rest):** Format aus Abschnitt 4.3.
- **Fokus beim Spielstart:** Start, Fortsetzen, Nochmal, nächste Bahn und Wiederholen nach Fehler setzen den Fokus auf die Spielfläche, damit Pfeiltasten und Leertaste sofort wirken. Gleiches gilt, wenn ein fokussierter Coach-Mark-Knopf verschwindet.

## 11. Bedienung

| Eingabe | Wirkung |
|---|---|
| ←/→ (gehalten) | Zielen; mit Shift fein |
| ↑/↓ (gehalten) | Stärke; mit Shift fein |
| Leertaste | Schlagen |
| Escape | Zug abbrechen; sonst Panel schließen bzw. einen Menüschritt zurück; sonst Pause |
| ? | Hilfe öffnen (nicht in Textfeldern) |
| Tab | durch Kopfleiste, HUD-Knöpfe, Fußleiste; bei offenem Panel Fokusfalle über Panel und Kopfleiste |
| Maus/Touch | wie P1: vom Ball zurückziehen; Rechtsziehen dreht die Kamera; zwei Finger zoomen/verschieben |

- Alle Bedienelemente sind per Tastatur erreichbar und haben sichtbaren Fokus (`--mg-focus`, 3 px).
- Touch-Ziele mindestens 44 × 44 px in Menüs und Fußleiste.

## 12. Texte

- Neue Schlüssel unter `minigolf.*` in allen 16 Dateien `ui/lang/desktop/*.json`, echte Übersetzungen, Deutsch mit „Du“ und Umlauten. Platzhalter `{n}`, `{name}`, `{par}`, `{hole}`, `{total}`, `{players}` bleiben unübersetzt.
- Gruppen (ca. 70 Schlüssel): Rundenwahl und Karten, Bahnraster, Spieler-Setup und 8 Farbnamen, Kopfleiste, HUD (Chips, Banner, Karte), Einblendungen (`progress_reset`), Scorecard, Treppchen und Ergebniskarte, Einführung (5 Schritte, Tastatur-Zusätze, Kamerahinweis, Überspringen, erneut starten), Hilfe-Abschnitte und Tastennamen, Einstellungsgruppen.
- Nicht mehr genutzte Schlüssel (z. B. `tutorial_body`, `round_ready`) werden in allen Sprachen entfernt.
- Weltnamen für `pirate`, `temple`, `moon` liefert P3; bis dahin gibt es diese Welten im Manifest nicht.

## 13. Tests und Abnahme

### 13.1 Node (`npm run test:minigolf`)

Neues Skript `scripts/test-minigolf-ui.mjs`, aufgenommen in `test:minigolf`. Es importiert nur Module ohne three.js und Rapier (`minigolf-shared.js`, `minigolf-hud.js`-Formatfunktionen, `minigolf-minimap.js`-Geometrie, `minigolf-scorecard.js`-Rangfolge, `minigolf-tutorial.js`-Zustandsmaschine); DOM-Code in diesen Modulen läuft erst beim Aufruf, nicht beim Import.

- Palette: 8 eindeutige Farben, Vorgaben 1–4, `colorValue()`; `cleanName()`/`uniqueNames()` mit Grenzfällen (Steuerzeichen, 24 Codepunkte, „Ada 2“/„Ada“/„Ada“).
- `formatPower()` mit `lang`: de „54 %“ und „≈ 11 m“, en „54%“; Rundung unter 1 m, unter 10 m, darüber.
- `laneGeometry()` mit einer kleinen synthetischen Bahn (Boden, Bande, Wasser, Hindernis aus zwei Teilen): Rahmen, Klassifikation, Wasser, Hindernisteile.
- `rankPlayers()`: Reihenfolge und geteilte Plätze.
- Einführung: Übergänge `aim → power → release → watch → done`, Sprung bei frühem Schlag, `cancel` ändert nichts, Kameramodus verbirgt.
- Qualitätsregler: Abstieg, kein Aufstieg vor 12 s, Aufstieg nach ca. 10 s guter FPS.

`scripts/test-minigolf-model.mjs`: Rundenformat v2 (Farben, eindeutige Farben, Namenslänge 24, Modi, `validateSave`/`cloneSave` mit `color`), `normalizeBest` (inkl. `set`), `recordHole`, `recordRound`.

### 13.2 Browser (`ui/desktop_minigolf_browser_test.go`, `AURAGO_RUN_BROWSER_SMOKE=1`)

Bestehende Abläufe auf die neuen Bedienelemente umstellen (`data-round`, `data-hole-id`, `data-player-count`, `data-player-color`, `data-player-name`, Einführung überspringen statt Textseite schließen) und ergänzen:

- Startseite mit Karte je Manifest-Welt, Bahnraster mit 18 Kacheln, Spieler-Setup mit Farbwahl (Farbe von Spieler 1 bei Spieler 2 deaktiviert).
- Im Spiel: Lochplakette mit Bahnname bei 420 und 1280 px sichtbar, Spielerchips, Banner beim Spielerwechsel (über einen Banner-Zähler in `inspect()`), Stärkeanzeige während eines Zugs innerhalb der Bühne, Karte sichtbar und umschaltbar.
- Einblendung deterministisch auslösen: `localStorage.setItem` werfen lassen → `storage_error`.
- Treppchen ohne ganze Runde: `round.v2` mit letzter Bahn und allen Spielern fertig vorbelegen, Fortsetzen, Nächste Bahn → Treppchen.
- Einführung: Start auf frischem Profil, Fortschritt durch echten Zug, Abschluss setzt `tutorial: 2`.
- Hilfe-Knopf bei 420 px sichtbar und öffnet die Hilfe; ?-Taste ebenso; Schließen kehrt zum vorherigen Menüschritt zurück.
- Layout ohne Überlauf bei 1280 × 760 und 420 × 760 für Startseite, Bahnraster, Spieler-Setup, Spiel und Scorecard. Die Theme-Matrix (`standard`, `fruity` hell/dunkel) prüft nur das Spiel und ein Panel.
- Jeder sichtbare Text ≥ 13 px.
- Reduzierte Bewegung: `root.getAnimations({ subtree: true })` auf `.mg-app` ist nach Öffnen der Scorecard leer.
- Bildschirmfotos nach `AURAGO_BROWSER_ARTIFACT_DIR` für Startseite, Bahnraster, Spieler-Setup, Spiel, Scorecard und Treppchen bei 420 und 1280 px.
- `inspect()` wird additiv erweitert um `hud` (`meter`, `bannerCount`, `banner`, `toasts`, `map`), `tutorial` (Schritt oder `null`) und `menuStep`.

### 13.3 Abnahmekriterien

- Layout bei 1280 × 760 und 420 × 760 ohne Überlauf (Browser-Test).
- Tastatur- und Touch-Bedienung aller Menüs, des HUD und der Einführung.
- Reduzierte Bewegung wird respektiert.
- Alle neuen Texte in 16 Sprachen (`TestDesktopMinigolfLocales`).
- Hilfe in jeder Breite erreichbar.
- Mindestschriftgröße 13 px.
- Kleinfehler aus Fahrplan §1.6 und P1-Reste behoben.
- `npm run test:minigolf`, `go test ./ui -run Minigolf`, `node scripts/build-minigolf.mjs --check` grün; `TestShippedWebAssetsLoadNothingExternal` und `TestUIAvoidsCookieConsentClassPrefix` grün.
- Bildschirmfotos aus dem Browser-Test durchgesehen.
- `documentation/minigolf.md` und der AuraGolf-Abschnitt in `ui/js/desktop/apps/AGENTS.md` beschreiben Menüs, HUD, Karte, Einführung, Speicherschlüssel, Settings-Felder, `inspect()`-Erweiterungen und neue Szenen-Helfer.

## 14. Schnittstellen zu anderen Teilprojekten

| Teil | Übergabe |
|---|---|
| P2 | Liefert Vorschaubilder je Welt und gibt sie aus `assets/minigolf/generate.py` als `worlds: [{ id, preview }]` ins Manifest aus. P2 ändert `minigolf-scene.js` (u. a. Qualitätsstufen); P4 hält seine Szenenänderungen klein (Abschnitt 10) und legt die Auto-Qualitäts-Logik in `minigolf-shared.js`. |
| P3 | Neue Welt-IDs funktionieren ohne UI-Änderung (Akzente und Symbole sind vorbereitet). P3 liefert Weltnamen (`minigolf.pirate` usw.) und Bahnnamen. Nach dem Dekodieren der binären Kollisionsdaten muss `course.colliders` dieselbe Form behalten (`type: 'mesh'`, `vertices`, `indices`, `surface`), sonst passt P3 `laneGeometry()` an. Neue Bodenoberflächen trägt P3 in `FLOOR_SURFACES` ein; Physik und Karte folgen gemeinsam. Alte Spielstände, Bahn-Bestwerte und Rundenbestwerte (über `set`) fallen beim Laden still heraus. |
| P5 | Ändert Audio, Credits und die Gruppe „Ton“ der Einstellungen. P4 hält `settings(settings, track)`, `audioSettings(settings, track)` und `credits(tracks)` als Einstiegspunkte stabil; Settings-Felder ergänzt P5 in `normalizeSettings()`. Da P4 `minigolf-views.js` neu schreibt, überträgt der zweite Teil, der landet, die Änderungen des anderen; beide ändern auch die Browser-Fixture. Vor dem Landen rebasen. |

## 15. Risiken

| Risiko | Gegenmaßnahme |
|---|---|
| Konflikte in `minigolf-game.js`, `minigolf-scene.js`, `minigolf-views.js`, Sprachdateien und Fixture mit P2/P3/P5 | Szenenänderungen klein; neue Logik in neuen Modulen; Notiz an die Schwester-Sessions; vor dem Landen rebasen und Bundle neu bauen |
| Lesbarkeit des HUD auf hellen und dunklen Welten | halbtransparente Flächen mit Schatten; Bildschirmfotos im Browser-Test |
| Seltene Desktop-Themes ohne einzelne Tokens | zweistufige Rückfallkette mit zusammenpassenden Sätzen |
| Stärkeanzeige verdeckt Ball oder Vorschau | Versatz neben den Ball, Spiegelung an Bühnenrändern, Ausblenden 1,2 s nach der letzten Änderung |
| Browser-Test wird zu lang oder unzuverlässig | deterministische Auslöser (Speicherfehler, vorbelegter Spielstand), Theme-Matrix nur für Spiel und ein Panel |
