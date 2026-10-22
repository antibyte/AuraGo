# Kapitel 31: Pixel – Bilder bearbeiten im Virtual Desktop

Pixel ist der Bildeditor im Virtual Desktop. Er arbeitet mit Ebenen, Mischmodi, Masken, Schnittmasken und Auswahlen. Dazu
kommen Mal- und Retuschewerkzeuge, 80 Filter und Anpassungen, 43 Generatoren für Rauschen, Texturen, Muster und mathematische
Bilder, ein Freistellwerkzeug und eine Befehlspalette. Projekte kannst du als geschichtete OpenRaster-Dateien (`.ora`)
speichern, die GIMP und Krita öffnen. Pixel bearbeitet Rastergrafiken: PNG-, JPEG- und `.ora`-Projekte.

Das Freistellen läuft standardmäßig auf deinem Gerät. Die optionalen KI-Modelle laufen auf deinem AuraGo-Server und werden
erst geladen, wenn du sie anforderst. Die älteren, providergestützten Funktionen im Bereich **KI** brauchen einen
Bildgenerierungs-Provider, wie in [Kapitel 4](./04-webui.md) und [Kapitel 21](./21-api-reference.md#pixel-bildeditor-api)
beschrieben.

## Inhalt
1. [Pixel öffnen](#pixel-öffnen)
2. [Aufbau](#aufbau)
3. [Ebenen](#ebenen)
4. [Auswahlen](#auswahlen)
5. [Malen und Retuschieren](#malen-und-retuschieren)
6. [Formen, Verläufe und Füllen](#formen-verläufe-und-füllen)
7. [Verschieben, Transformieren, Zuschneiden und Ansicht](#verschieben-transformieren-zuschneiden-und-ansicht)
8. [Filter und Anpassungen](#filter-und-anpassungen)
9. [Generatoren](#generatoren)
10. [Freistellen](#freistellen)
11. [Intelligenz](#intelligenz)
12. [Projektdateien](#projektdateien)
13. [Grenzen](#grenzen)
14. [Tastenkürzel](#tastenkürzel)
15. [Datenschutz und Sicherheit](#datenschutz-und-sicherheit)
16. [Wenn etwas schiefgeht](#wenn-etwas-schiefgeht)

---

## Pixel öffnen

Öffne **Pixel** aus dem Startmenü. Es gibt noch weitere Wege:

- **Datei → Neues Bild** legt ein leeres Dokument an. Die Generatoren (siehe [Generatoren](#generatoren)) können
  ebenfalls ein neues Dokument erzeugen.
- **Strg+O** (**Datei → Öffnen**) öffnet eine PNG-, JPEG- oder `.ora`-Datei. Der Dialog startet im Ordner Fotos. Du kannst
  auch eine Bild- oder `.ora`-Datei auf das Pixel-Fenster ziehen.
- Fotos aus der Kamera-App öffnen sich in Pixel.

**Strg+S** speichert in die Datei, die du geöffnet hast. **Strg+Umschalt+S** (**Datei → Speichern unter**) speichert unter
einem neuen Namen. **Datei → Exportieren** lädt eine reduzierte Kopie im Format deiner Wahl herunter. In einem
schreibgeschützten Virtual Desktop ist Speichern gesperrt.

## Aufbau

- **Menüleiste:** Datei, Bearbeiten, Bild, Ebene, Auswahl, Filter, Generieren, Ansicht und KI.
- **Werkzeugleiste** (links): die Werkzeuge, gruppiert als Auswahl, Malen, Retusche, Füllen, Form, Text und Ansicht. Fahre
  mit der Maus über ein Werkzeug, um Namen und Tastenkürzel zu sehen.
- **Optionsleiste** (über der Arbeitsfläche): die Einstellungen des aktiven Werkzeugs, etwa Größe, Deckkraft, Modus oder
  Toleranz.
- **Arbeitsfläche** (Mitte): dein Bild, bei Transparenz auf einem Schachbrettmuster. Halte die Leertaste gedrückt und ziehe,
  um zu verschieben. Mit Strg und dem Mausrad zoomst du.
- **Inspektor** (rechts, oben): die Reiter **Anpassen**, **Filter**, **Farben**, **Pinsel**, **Transformieren**, **Generieren**,
  **Freistellen**, **KI** und **Sticker**. Es ist immer nur ein Reiter sichtbar.
- **Untere Leiste** (rechts, unter dem Inspektor): **Ebenen** und **Verlauf**. Ziehe den Teiler zwischen Inspektor und
  unterer Leiste, um die Größen zu ändern.

In der Statuszeile siehst du beim Ziehen die Größe einer Auswahl oder Form. Die Befehlspalette öffnest du mit **Strg+K**
(siehe [Intelligenz](#intelligenz)).

## Ebenen

Jede Ebene ist ein Bild in voller Dokumentgröße. Das Gesamtbild entsteht aus allen sichtbaren Ebenen. Die Leiste
**Ebenen** zeigt oben die Einstellungen der aktiven Ebene und darunter den Ebenenstapel, oberste Ebene zuerst.

- **Mischmodus:** Normal, Multiplizieren, Abdunkeln, Negativ multiplizieren (Screen), Aufhellen, Farbig nachbelichten (Color
  burn), Farbig abwedeln (Color dodge), Ineinanderkopieren (Overlay), Weiches Licht, Hartes Licht, Differenz, Ausschluss,
  Farbton, Sättigung, Farbe und Luminanz. Der Modus wirkt auf die Ebenen darunter.
- **Deckkraft:** Schieberegler oder Zahlenfeld, von 0 bis 100 %.
- **Sperren:** **Pixel sperren** (kein Malen und kein Filtern), **Transparenz sperren** (der Alphakanal bleibt unverändert) und
  **Position sperren** (kein Verschieben). Eine gesperrte Ebene zeigt ein Schloss-Symbol. Eine Aktion auf ihr wird mit
  „Diese Ebene ist gesperrt.“ abgelehnt.
- **Schnittmaske:** Eine Schnittmaske wird auf die Transparenz (Form) der nächsttieferen Ebene beschnitten, die selbst keine
  Schnittmaske ist. Schalte sie über den Schalter in der Kopfzeile der Ebenen, über das Menü Ebene oder mit **Strg+Alt+G** ein.
- **Sichtbarkeit:** Klicke auf das Auge, um eine Ebene ein- oder auszublenden. **Alt+Klick** auf das Auge zeigt nur diese
  Ebene. Ein weiterer Alt+Klick zeigt wieder alle Ebenen.
- **Umbenennen:** Doppelklicke auf den Namen oder drücke **F2**. Mit Enter übernimmst du den Namen, mit Escape brichst du ab.
- **Reihenfolge:** Ziehe eine Zeile, oder verschiebe die aktive Ebene mit **Strg+]** und **Strg+[** nach oben und unten.
- **Auswahl aus einer Ebene:** **Strg+Klick** auf eine Vorschau wählt die sichtbaren Pixel dieser Ebene aus. Mit Umschalt
  fügst du sie der Auswahl hinzu, mit Alt ziehst du sie ab.
- **Masken:** Eine Maske blendet Teile einer Ebene aus oder ein, ohne deren Pixel zu verändern. Füge eine Maske über die
  Fußleiste oder das Menü Ebene hinzu: alles einblenden, alles ausblenden, aus der Auswahl oder die Auswahl ausblenden.
  Klicke auf die Masken-Vorschau, um auf der Maske zu malen: Weiß blendet ein, Schwarz blendet aus. **Umschalt+Klick** auf
  die Masken-Vorschau schaltet die Maske ein oder aus, **Strg+Klick** macht daraus eine Auswahl. Im Menü Ebene findest du außerdem
  **Maske invertieren**, **Maske anwenden** (schreibt die Maske in die Pixel, die Transparenz ist danach dauerhaft) und
  **Maske löschen**. Maske anwenden wird bei gesperrten Ebenen abgelehnt.

Die Fußleiste der Ebenen hat Schaltflächen für **Ebene hinzufügen** (Strg+Umschalt+Alt+N), **Ebene über Kopie** (Strg+J),
**Duplizieren**, **Ebenenmaske hinzufügen**, **Ebene löschen**, **Nach unten zusammenfügen** (Strg+E) und **Mehr**. Ein
Rechtsklick auf eine Zeile bietet dieselben Aktionen wie das Menü Ebene:

- **Anlegen:** Neue Ebene, Neue Füllebene (in der Vordergrundfarbe), Ebene über Kopie, Ebene über Ausschneiden (mit
  Auswahl), Duplizieren und Bild als Ebene einfügen…
- **Bearbeiten:** Ebene umbenennen, Ebeneneigenschaften (Name, Deckkraft, Mischmodus, Sperren und Schnittmaske in einem
  Dialog), Mit Vordergrund- oder Hintergrundfarbe füllen und Ebene leeren.
- **Zusammenführen:** Nach unten zusammenfügen (Strg+E), Sichtbare zusammenfügen (Strg+Umschalt+E) und Auf eine Ebene
  reduzieren. Das fasst alle Ebenen in einer Ebene namens Hintergrund zusammen.
- **Anordnen:** Nach ganz vorne (Strg+Umschalt+]), Nach vorne (Strg+]), Nach hinten (Strg+[), Nach ganz hinten
  (Strg+Umschalt+[), Ebene darüber wählen (Alt+]) und Ebene darunter wählen (Alt+[).
- **Ausrichten:** links, horizontal zentriert, rechts, oben, vertikal zentriert und unten. Ausgerichtet wird an der
  Auswahl, wenn es eine gibt, sonst an der Arbeitsfläche.
- **Weiteres:** Auswahl aus Ebene, Sticker rastern (für Stickerebenen) und Ebene als PNG exportieren.

Die letzte Ebene lässt sich nicht löschen. Ein Dokument kann bis zu 64 Ebenen haben.

## Auswahlen

| Werkzeug | Taste | Funktion |
|---|---|---|
| Rechteckauswahl | `V` | Ziehe ein Rechteck. Der Stil kann Normal, ein festes Seitenverhältnis (B : H) oder eine feste Größe (B × H) sein. |
| Ellipsenauswahl | `Umschalt+V` | Ziehe eine Ellipse. |
| Lasso | `L` | Ziehe eine freihändige Umrisslinie. |
| Polygon-Lasso | `Umschalt+K` | Klicke die Ecken an. Doppelklick oder Enter schließt die Form, Rücktaste entfernt den letzten Punkt, Escape bricht ab. |
| Zauberstab | `W` | Wählt einen Bereich ähnlicher Farbe. Optionen: Toleranz, Zusammenhängend und Alle Ebenen. |
| Schnellauswahl | `Umschalt+W` | Male über das Motiv. Die Auswahl wächst über ähnliche Farben und stoppt an Kanten. Alt zieht ab. |

Alle Auswahlwerkzeuge haben vier Modi: **Ersetzen**, **Hinzufügen** (Umschalt), **Subtrahieren** (Alt) und
**Schnittmenge** (Umschalt+Alt). Die Optionsleiste zeigt dieselbe Auswahl als Schalter. Weiche Kante und Kantenglättung
wirken auf den Rand der Auswahl.

**Auswahl → Farbbereich…** wählt nach Farbe im ganzen Bild. Klicke ins Bild, um Farben zu wählen. Umschalt+Klick nimmt eine
Farbe hinzu, Alt+Klick entfernt sie. Ein Schieberegler legt fest, wie nah eine Farbe sein muss. Ein rotes Overlay zeigt das
Ergebnis, bevor du es übernimmst.

Das Menü **Auswahl** enthält außerdem:

- **Alles** (Strg+A), **Auswahl aufheben** (Strg+D), **Auswahl wiederherstellen** (Strg+Umschalt+D) und **Auswahl umkehren**
  (Strg+Umschalt+I).
- **Motiv auswählen**, das die Freistellfunktion nutzt (siehe [Freistellen](#freistellen)).
- **Ändern:** Vergrößern und Verkleinern (1 bis 250 px), Rand (1 bis 200 px), Glätten (1 bis 100 px) und Weiche Kante
  (Umschalt+F6, 0,5 bis 250 px).
- **Auswahl aus Transparenz der Ebene**, **Auswahl als Ebenenmaske speichern**, **In neue Ebene kopieren** und **In neue
  Ebene ausschneiden** (Strg+Umschalt+J).
- **Auswahl umranden…** zeichnet eine Linie entlang des Umrisses, mit Breite und Lage: innen, mittig oder außen.
- **Auswahl transformieren** skaliert und dreht nur den Umriss der Auswahl, nicht die Pixel.

Füllkürzel für die Auswahl: **Alt+Rücktaste** füllt mit der Vordergrundfarbe, **Strg+Rücktaste** mit der Hintergrundfarbe.
**Umschalt+Rücktaste** öffnet die inhaltsbasierte Füllung (siehe [Intelligenz](#intelligenz)).

Um den Umriss einer Auswahl zu verschieben, ziehe mit einem Auswahlwerkzeug in die Auswahl hinein. Mit den Pfeiltasten
verschiebst du die Auswahl um 1 px, mit Umschalt um 10 px. **Escape** hebt die Auswahl auf oder bricht einen laufenden
Zuschnitt ab.

## Malen und Retuschieren

### Pinsel-Engine

**Pinsel** (`B`), **Bleistift** (`P`), **Airbrush** (`A`) und **Radierer** (`E`) nutzen dieselbe Engine. Ihre Einstellungen
stehen in der Optionsleiste, alle Einstellungen auch im Reiter **Pinsel**.

- **Größe** von 1 bis 1000 px. Mit den Tasten `[` und `]` wird der Pinsel kleiner und größer.
- **Deckkraft** und **Fluss**. Die Tasten 1 bis 9 setzen die Deckkraft auf 10 bis 90 %, die Taste 0 auf 100 %.
- **Härte** von weich bis hart. Die Tasten `Umschalt+[` und `Umschalt+]` ändern sie.
- **Abstand**, **Glättung** (ein Stabilisator, der den Strich glättet), **Winkel**, **Rundheit** und **Richtung des Strichs
  folgen**.
- **Form:** rund, quadratisch, Stern oder Spray. **Streuung**, **Tupfer pro Schritt** und Zufallsschwankungen der Größe, des
  Winkels und der Deckkraft verteilen die Tupfer. **Korn** fügt Papier- oder Kreidestruktur hinzu, mit einer Korngröße.
- **Stiftdruck:** Aus, Größe, Deckkraft oder Beides. Nur ein Stift liefert einen Druckwert. Maus- und Touch-Striche nutzen
  immer den vollen Druck.
- **Symmetrie:** Aus, Vertikal, Horizontal, Beides oder Radial mit 2 bis 24 Segmenten um die Bildmitte. Hilfslinien zeigen
  die Spiegelachsen.
- **Radierermodus:** Pinsel (weich) oder Block (eckig).

Zehn Vorgaben setzen diese Werte auf einmal: Rund hart, Rund weich, Airbrush, Bleistift, Tusche, Marker, Kreide,
Kalligrafie, Spray und Sternstreuung. Wähle eine in der Vorgabenliste der Optionsleiste.

Einige Details erleichtern das Malen:

- **Umschalt+Klick** zieht eine gerade Linie vom Ende des letzten Strichs.
- **Alt+Klick** übernimmt die Farbe unter dem Zeiger als Vordergrundfarbe.
- **Escape** bricht den laufenden Strich ab.
- Der Bleistift zeichnet harte Einzelpixel ohne Kantenglättung.
- Striche beachten die Auswahl (mit weicher Kante), die Transparenzsperre und den Maskenmodus. Im Maskenmodus malt der Pinsel
  Weiß, das einblendet, und der Radierer schreibt Schwarz, das ausblendet.

### Retuschewerkzeuge

| Werkzeug | Taste | Funktion |
|---|---|---|
| Verwischen | `Y` | Schiebt Farbe entlang des Strichs. Die Option Fingermalerei malt mit der Vordergrundfarbe. |
| Weichzeichner- / Schärfepinsel | `N` | Weichzeichnen oder Schärfen des Bereichs unter dem Pinsel. Der Modus wählt aus. |
| Abwedeln und Nachbelichten | `D` | Hellt auf (Abwedeln) oder dunkelt ab (Nachbelichten). Sättigen und Entsättigen wirken wie ein Schwamm. Der Bereich legt Tiefen, Mitteltöne oder Lichter fest. |
| Kopierstempel | `S` | Alt+Klick legt die Quelle fest. Ausgerichtet hält den Abstand zwischen Quelle und Pinsel. Alle Ebenen nimmt das ganze Bild als Quelle. |
| Reparaturpinsel | `Umschalt+J` | Kopiert aus der Quelle und passt Farbe und Helligkeit des Bereichs unter dem Pinsel an. |
| Bereichsreparatur | `J` | Male über einen Fleck und lasse los. Der Bereich wird aus seiner Umgebung gefüllt. |
| Farbe ersetzen | `Umschalt+B` | Gibt ähnlichen Farben den Farbton und die Sättigung der Vordergrundfarbe. Die Helligkeit bleibt. |
| Rote Augen | `Umschalt+E` | Klicke auf eine Pupille. Suchradius (5 bis 200 px) und Abdunkeln (0 bis 100 %) sind einstellbar. |
| Objekt entfernen | (keine) | Male über ein Objekt und lasse los. Pixel füllt die Fläche (siehe [Intelligenz](#intelligenz)). |
| Freistellpinsel | (keine) | Verfeinert eine Maske beim Freistellen (siehe [Freistellen](#freistellen)). |

## Formen, Verläufe und Füllen

**Formen** werden mit einer Außenlinie in der Vordergrundfarbe gezeichnet, optional mit einer Füllung in der
Hintergrundfarbe:

- **Rechteck** (`R`), **Abgerundetes Rechteck** (`Umschalt+R`, Eckenradius 0 bis 500 px), **Ellipse** (`O`), **Polygon**
  (`Umschalt+O`, 3 bis 24 Seiten), **Stern** (`Umschalt+S`, 3 bis 24 Zacken, Innenradius 10 bis 95 %), **Linie**
  (`Umschalt+L`) und **Pfeil** (`Umschalt+A`, mit Spitze am Anfang, am Ende oder an beiden Enden; Spitzengröße 1 bis 10).
- Strichbreite von 1 bis 200 px, Linienart durchgezogen, gestrichelt oder gepunktet, Linienenden, Kantenglättung und Deckkraft.
- Halte **Umschalt**, um einzuschränken: Quadrate, Kreise und Linien in 45-Grad-Schritten. Halte **Alt**, um von der Mitte aus
  zu zeichnen. **Escape** bricht das Ziehen ab.

Der **Farbverlauf** (`U`) hat fünf Typen: Linear, Radial, Konisch, Raute und Gespiegelt. Die Farben sind Vordergrund zu
Hintergrund, Vordergrund zu transparent oder eine von 18 Vorgaben. Umkehren und Dithering (es verringert Bandbildung) sind
Optionen. Umschalt begrenzt die Richtung auf 45 Grad, und der Verlauf bleibt innerhalb der Auswahl.

**Füllen** (`G`) ist der Farbeimer. Mit **Zusammenhängend** (standardmäßig an) füllst du nur den zusammenhängenden Bereich unter
dem Klick, ohne die Option füllst du alle passenden Pixel. Die Toleranz reicht von 0 bis 255 (Standard 32). **Alle Ebenen**
nimmt die Farbe aus dem ganzen Bild auf. Die Füllung ist die Vordergrundfarbe oder ein Muster (Karo, Streifen, Punkte oder
Raster). Kantenglättung und Deckkraft sind ebenfalls verfügbar.

Die **Pipette** (`I`) setzt die Vordergrundfarbe auf das Pixel unter dem Zeiger.

**Text** (`T`) setzt Text auf die Arbeitsfläche. Klicke, schreibe in das Feld und drücke Enter, um den Text zu setzen.
Umschalt+Enter beginnt eine neue Zeile, Escape verwirft den Text. Ein Klick an eine andere Stelle setzt den Text ebenfalls.

## Verschieben, Transformieren, Zuschneiden und Ansicht

**Verschieben** (`M`) bewegt die aktive Ebene. Mit einer Auswahl bewegst du die ausgewählten Pixel, und die Auswahl wandert mit.

- **Alt+Ziehen** kopiert die Auswahl, statt sie zu verschieben.
- **Strg+Klick** wählt die Ebene unter dem Zeiger, wenn keine Auswahl besteht. Die Option **Ebene automatisch wählen** tut das
  bei jedem Klick.
- Mit den Pfeiltasten verschiebst du um 1 px (mit Umschalt um 10 px). Escape bricht eine laufende Verschiebung ab.
- **An Hilfslinien einrasten** richtet das verschobene Bild an den Kanten und Mitten der Arbeitsfläche und der Ebenen aus, wenn
  es nahe genug kommt.

**Frei transformieren** (**Strg+T**, alternativ **Strg+Alt+T**) verändert die aktive Ebene oder die Auswahl:

- Die Ecken- und Kantengriffe skalieren. Umschalt behält die Proportionen bei, Alt skaliert von der Mitte aus.
- Ziehe außerhalb des Rahmens, um zu drehen. Umschalt dreht in 15-Grad-Schritten. Ziehe innerhalb des Rahmens, um zu
  verschieben.
- Die Optionsleiste hat Zahlenfelder für X, Y, Breite, Höhe und Winkel sowie einen Schalter für das Seitenverhältnis.
- **Enter** übernimmt die Transformation, **Escape** bricht sie ab.
- **Bearbeiten → Ebene transformieren** spiegelt die Ebene horizontal oder vertikal und dreht sie um 90 Grad im oder gegen den
  Uhrzeigersinn.
- **Auswahl → Auswahl transformieren** verändert nur den Umriss der Auswahl.

**Zuschneiden** (`C`, auch über **Bild → Zuschneiden** und den Zuschneideknopf im Reiter **Transformieren**) zeigt Griffe an einem
Zuschneiderahmen. Du ziehst die Griffe oder verschiebst den Rahmen. Das Seitenverhältnis ist Frei, Original, Benutzerdefiniert,
1:1, 4:3, 3:2, 16:9 oder 9:16. Die Überlagerung ist Drittel-Regel (Standard), ein Raster oder aus. Der Bereich außerhalb des
Zuschnitts wird abgedunkelt. Enter übernimmt den Zuschnitt, Escape bricht ihn ab.

**Ansichtswerkzeuge:**

- **Hand** (`H`) verschiebt die Ansicht per Ziehen.
- **Zoom** (`Z`): Ein Klick vergrößert, Alt+Klick verkleinert, und ein aufgezogenes Rechteck zoomt auf diesen Bereich.
- Die Leertaste mit Ziehen verschiebt die Ansicht bei jedem Werkzeug. Strg und das Mausrad zoomen.

Das Menü **Ansicht** enthält Vergrößern (Strg++), Verkleinern (Strg+-), Einpassen (Strg+0), Zoom 100 %, Bildschirm füllen,
**Pixelraster** (ab 800 % sichtbar), **Raster** (8, 16, 32, 64 oder 128 px) und **An Hilfslinien einrasten**.

## Filter und Anpassungen

Filter und Anpassungen wirken auf die aktive Ebene. Innerhalb einer Auswahl werden sie an der weichen Kante eingeblendet. Jede
Änderung zeigt eine Live-Vorschau. Die Vorschau wird im Hintergrund berechnet, bei großen Bildern zuerst auf einer kleineren
Kopie. **Übernehmen** schreibt das Ergebnis in die Ebene, **Zurücksetzen** und **Abbrechen** verwerfen es.

### Reiter Filter

- Das Suchfeld und die Chips (Alle, Favoriten und die acht Kategorien) grenzen die Galerie ein. Klicke auf den Stern einer
  Karte, um den Filter zu den **Favoriten** hinzuzufügen.
- Klicke auf eine Karte, um ihre Einstellungen zu öffnen. Unter den Parametern blenden **Stärke** (0 bis 100 %) und **Modus**
  den Filter in das Original ein. Die Modi sind Normal, Multiplizieren, Negativ multiplizieren, Ineinanderkopieren, Weiches
  Licht, Hartes Licht, Abdunkeln, Aufhellen, Differenz, Farbe und Luminanz.
- **Vorher / Nachher** zeigt das Original und das Ergebnis, getrennt durch eine Linie, die du verschieben kannst.
- **Letzten Filter wiederholen** (**Strg+F**) führt den letzten Filter mit seinen letzten Einstellungen erneut aus.
  **Letzten Filter…** (**Strg+Alt+F**) öffnet zuerst den Dialog des letzten Filters.

Die 80 Filter nach Kategorie:

| Kategorie | Anzahl | Filter |
|---|---|---|
| Anpassen | 22 | Tonwerte, Gradationskurven, Farbton/Sättigung, Farbbalance, Dynamik, Kanalmixer, Schwarzweiß, Fotofilter, Verlaufsabbildung, Schwellenwert, Posterisieren, Invertieren, Egalisieren, Auto-Tonwerte, Auto-Kontrast, Auto-Weißabgleich, Graustufen, Monochrom, Warm, Kühl, Tönung, Hoher Kontrast |
| Weichzeichnen | 8 | Gaußscher Weichzeichner, Box-Weichzeichner, Bewegungsunschärfe, Drehunschärfe, Zoomunschärfe, Objektivunschärfe, Oberflächenunschärfe, Median |
| Schärfen | 4 | Scharfzeichnen, Unscharf maskieren, Hochpass, Klarheit |
| Verzerren | 10 | Verdrehen, Quetschen / Aufblasen, Welle, Kräuseln, Polarkoordinaten, Kugelisieren, Kaleidoskop, Verschieben, Glas, Objektivverzerrung |
| Stilisieren | 15 | Ölgemälde, Halbton, Dithering, Pixelkunst, Toon, Kristallisieren, Pointillismus, Relief, Kanten finden, Leuchtende Kanten, Skizze, Comic, Neon, Mosaik, Verpixeln |
| Rauschen | 4 | Rauschen hinzufügen, Rauschen reduzieren, Staub und Kratzer, Filmkorn |
| Licht | 7 | Vignette, Bloom, Glühen, Lichtleck, Linsenreflexion, Chromatische Aberration, Tilt-Shift |
| Retro | 10 | Vintage, Verblassen, Cyanotypie, Duplex, Sepia, VHS, CRT, Glitch, Solarisation, Cross-Entwicklung |

Einige Filter haben eigene Regler:

- **Tonwerte:** Kanal (RGB, Rot, Grün oder Blau), Eingangsschwarz, Gamma und Eingangsweiß sowie Ausgangsschwarz und -weiß.
- **Gradationskurven:** eine Kurve je Kanal (RGB, R, G oder B). Klicke, um einen Punkt hinzuzufügen, ziehe ihn zum Verschieben und
  doppelklicke, um ihn zu entfernen. Vorgaben: Linear, Kontrast, Starker Kontrast, Aufhellen, Abdunkeln, Matt, Negativ und
  Cross-Entwicklung.
- **Dithering:** Floyd–Steinberg, Atkinson oder geordnet Bayer 2×2, 4×4 und 8×8. Die Palette ist 1 Bit, Game Boy, CGA, C64,
  PICO-8, EGA, Web-sicher oder Aus dem Bild mit 2 bis 64 Farben.
- **Verlaufsabbildung** und **Duplex** nutzen einen Farbverlauf beziehungsweise zwei Farben.

Im Menü **Filter** findest du alle Filter nach Kategorie sortiert.

### Reiter Anpassen

- Ein **Histogramm** oben zeigt das Bild vor und nach deinen Änderungen. Die Schaltflächen wechseln zwischen RGB, Luma, Rot,
  Grün und Blau.
- **Regler** von −100 bis +100 in drei Gruppen: Licht (Belichtung, Helligkeit, Kontrast, Lichter, Tiefen, Weiß, Schwarz),
  Farbe (Temperatur, Tönung, Dynamik, Sättigung) und Detail (Klarheit, Dunst entfernen, Schärfe, Vignette).
- **Auto** setzt Tonwerte, Kontrast, Weißabgleich und etwas Dynamik in einem Schritt. Mit **Übernehmen** behältst du das Ergebnis.
- **Vorgaben:** Lebendig, Matt, Schwarzweiß, Warmer Film, Kühler Ausklang, Hell (High Key), Dunkel (Low Key) und Punch.
- **Vergleichen** zeigt die Ansicht vorher und nachher, **Übernehmen** schreibt die Änderung, **Zurücksetzen** leert die Regler.

**Auto-Verbesserung** (**Strg+Umschalt+L**, im Menü **Filter**) wendet die automatische Korrektur sofort an, ohne die Regler.

## Generatoren

Der Reiter **Generieren**, das Menü **Generieren** und die Befehlspalette (**Generieren: …**) erstellen neue Bilder aus
Parametern. Die Generatoren laufen auf deinem Gerät, es wird nichts heruntergeladen. Derselbe Seed mit denselben Einstellungen
ergibt immer dasselbe Bild.

| Kategorie | Anzahl | Generatoren |
|---|---|---|
| Rauschen | 9 | Weißes Rauschen, Wertrauschen, Perlin-Rauschen, Simplex-Rauschen, Zellulär (Worley), Fraktales Rauschen (fBm), Gratiges Mehrfachrauschen, Turbulenz, Domain-verzerrtes Rauschen |
| Texturen | 10 | Wolken, Marmor, Holz, Plasma, Tarnmuster, Lava, Kaustik, Nebel und Sterne, Papier und Leinwand, Gebürstetes Metall |
| Muster | 10 | Schachbrett, Streifen, Raster, Punkte, Sechseckkacheln, Truchet-Kacheln, Bleiglas, Halbton-Verlauf, Ziegel und Fischgrät, Wellen |
| Mathematik | 14 | Verlauf, Mandelbrot-Menge, Julia-Menge, Burning Ship, Newton-Fraktal, Spirograph, Rosenkurve, Lissajous-Kurve, Superformel, Phyllotaxis, Moiré und Interferenz, Strömungsfeld, Reaktions-Diffusion, Labyrinth |

Gemeinsame Bedienelemente:

- **Seed** und ein Würfel für einen neuen zufälligen Seed.
- **Verlauf** färbt das Ergebnis mit einem Farbverlauf ein.
- **Kachelbar** (bei den Rauschgeneratoren) sorgt dafür, dass das Bild an den Rändern nahtlos anschließt.
- **Vorgaben:** eine bis vier je Generator. Perlin-Rauschen bietet zum Beispiel Weiche Wolken, Harter Fels und Sanfte Hügel.
- **Vorschau auf der Arbeitsfläche** zeigt das Ergebnis in Dokumentgröße, bevor du generierst. Ausschalten entfernt die Vorschau.

Du wählst, wohin das Ergebnis geht:

- **Neue Ebene** (Standard) legt eine Ebene über die aktive. Mischmodus und Deckkraft stellst du im Reiter ein.
- **Ebene ersetzen** ersetzt die Pixel der aktiven Ebene.
- **In Auswahl** füllt nur die Auswahl. Ohne Auswahl wird die ganze Ebene gefüllt.

**Generieren** rechnet in voller Größe und zeigt einen Fortschrittsbalken. **Abbrechen** stoppt die Berechnung. Ohne geöffnetes
Dokument fragt der Reiter nach Breite und Höhe (16 bis 8192 px, Standard 1024 × 1024) und legt ein neues Dokument an.

## Freistellen

Pixel kann Hintergründe auf zwei Arten entfernen: mit dem **klassischen** Verfahren, das in deinem Browser läuft, und mit
optionalen **KI-Modellen**, die auf deinem AuraGo-Server laufen. Beide erzeugen eine **Ebenenmaske**. Die Originalpixel bleiben
erhalten, du kannst den Freisteller also verfeinern oder rückgängig machen. Die Werkzeuge findest du im Reiter **Freistellen**
und im Menü **KI**.

### Verfahren

| Verfahren | Wo es läuft | Hinweise |
|---|---|---|
| Automatisch (Standard) | Hängt von der Auswahl ab | Nutzt das KI-Modell, wenn eines geladen ist, sonst das klassische Verfahren. |
| KI standard | AuraGo-Server | ISNet general-use (Apache-2.0), Download etwa 179 MB. |
| KI präzise | AuraGo-Server | BiRefNet general lite (MIT), Download etwa 224 MB. |
| Klassisch (auf dem Gerät) | Dein Browser | Trennt Motiv und Hintergrund mit Farbmodellen und einem Graph-Schnitt. Funktioniert gut, wenn das Motiv eine klare Kante hat. |
| Farbschlüssel | Dein Browser | Entfernt eine Farbe. **Aus den Kanten erkennen** (standardmäßig an) wählt sie aus, oder du legst die **Schlüsselfarbe** selbst fest. Toleranz (0 bis 100 %, Standard 40) und Weichheit (0 bis 100 %, Standard 20) steuern die Kante. |

### KI-Modelle

Die KI-Modelle gehören nicht zu AuraGo. Sie werden nur geladen, wenn du auf die Schaltfläche der Modellkarte im Reiter
**Freistellen** klickst, die **Modell laden (179 MB)** oder **Modell laden (224 MB)** heißt. Die Karte zeigt danach
**Wird geladen** mit einem Prozentwert und schließlich **Bereit**. Beim ersten Laden kommt außerdem die ONNX-Runtime für deine
Plattform dazu, falls sie fehlt. Pixel prüft jeden Download anhand der festgelegten Größe und des SHA-256-Hashwerts, bevor er
verwendet wird.

- Die KI-Engines erscheinen nur dort, wo der Server eine Laufzeit für die Plattform hat. Andernfalls zeigt der Reiter, dass das
  KI-Freistellen auf diesem Gerät nicht verfügbar ist. Das klassische Verfahren funktioniert trotzdem.
- In einem schreibgeschützten Virtual Desktop sind Modell-Downloads gesperrt.
- Auf einem Desktop-Prozessor brauchte ein Foto mit 800 × 533 Pixeln etwa 2 Sekunden mit ISNet und etwa 16 Sekunden mit
  BiRefNet. Auf deiner Hardware kann das anders ausfallen.

### Aktionen

- **Hintergrund entfernen** legt eine Ebenenmaske über die aktive Ebene. Es wird nichts gelöscht, du kannst die Maske später
  verfeinern oder entfernen.
- **Motiv auswählen** erstellt eine Auswahl um das Motiv. Das Menü Auswahl hat denselben Befehl.
- **Freigestelltes in neue Ebene** kopiert das Motiv in eine neue Ebene, der Hintergrund ist transparent und Farbsäume sind
  entfernt.
- **Hintergrund weichzeichnen** fügt eine neue Ebene mit dem unscharfen Hintergrund hinzu. **Stärke der Unschärfe** (1 bis 100 %)
  legt fest, wie stark der Weichzeichner wirkt.
- **Hintergrund ersetzen** setzt eine Volltonfarbe (standardmäßig Weiß), Transparenz oder einen generierten Hintergrund unter das
  Motiv. Generierte Hintergründe sind Generatoren wie Wolken oder Verläufe, die sich als Hintergrund eignen.

Das Menü **KI** enthält **Hintergrund entfernen**, **Freigestelltes in neue Ebene**, **Hintergrund weichzeichnen** und
**Freistellen** (öffnet den Reiter).

Der Reiter **KI** hat außerdem eine Schaltfläche **Hintergrund entfernen**. Sie ist die ältere, providergestützte Variante und
braucht einen konfigurierten Bild-Provider. Der Reiter **Freistellen** braucht keinen Provider.

### Verfeinern

Sobald die aktive Ebene eine Maske hat, ist der Bereich **Verfeinern** im Reiter **Freistellen** verfügbar:

- **Kantenverschiebung** (−10 bis +10 px) vergrößert oder verkleinert die Maske. **Weiche Kante** (0 bis 20 px) macht den Rand
  weicher, **Glätten** (0 bis 10) glättet den Umriss.
- **Farbsaum entfernen** (standardmäßig an) entfernt die Hintergrundfarbe aus den Randpixeln. **Saumradius** (1 bis 20 px,
  Standard 6) legt fest, wie weit vom Rand aus gearbeitet wird.
- **Ansicht** zeigt das Ergebnis als Normal, Schachbrett, Auf Schwarz, Auf Weiß oder Rotes Overlay.
- **Freistellpinsel** malt auf der Maske. Behalten fügt das Motiv wieder hinzu, Entfernen nimmt es weg, mit einer Größe von 2 bis
  300 px und einer Härte. Halte Alt, um zwischen Behalten und Entfernen zu wechseln. Mit **Intelligent** wird das Verfahren beim
  Loslassen erneut gerechnet, und deine Striche leiten es.
- **Maske anwenden** schreibt die verfeinerte Maske in die Pixel der Ebene, die Transparenz wird damit dauerhaft.
  **Zurücksetzen** stellt die Einstellungen der Verfeinerung auf die Standardwerte zurück und baut die Maske aus dem ursprünglichen
  Ergebnis des Verfahrens neu auf.

Der Knopf **Abbrechen** stoppt einen laufenden Freistellvorgang.

## Intelligenz

### Befehlspalette

Drücke **Strg+K** (oder **Strg+Umschalt+P**), um die Befehlspalette zu öffnen. Sie listet jeden Befehl auf: Menüeinträge,
Werkzeuge, Reiter, Filter und Generatoren, mit Tastenkürzel und Gruppe.

- **Tippe, um zu suchen.** Die Suche ist unscharf: Die Buchstaben müssen in derselben Reihenfolge vorkommen, und Treffer am
  Wortanfang zählen mehr.
- **Synonyme auf Deutsch und Englisch** funktionieren in beide Richtungen. Zum Beispiel findet `freistellen` den Befehl
  **Hintergrund entfernen**, `rauschen` findet die Rauschfilter, `weichzeichnen` die Weichzeichner und `ebene` die Ebenenbefehle.
- **Pfeiltasten** bewegen dich durch die Liste, **Enter** führt den markierten Befehl aus und **Escape** schließt die Palette.
  Befehle, die gerade nicht verfügbar sind, erscheinen ausgegraut.
- Ohne Suchbegriff zeigt die Palette zuerst deine letzten Befehle (die letzten 20), darunter Vorschläge für den aktuellen Zustand.
  Mit einer Auswahl schlägt sie Inhaltsbasierte Füllung, Weiche Kante und In neue Ebene kopieren vor. Bei einem frisch
  geöffneten Foto kommen Auto-Verbesserung und Hintergrund entfernen. Bei transparenten Rändern kommt Transparente Pixel
  zuschneiden.

### Inhaltsbasierte Füllung und Objekte entfernen

- **Bearbeiten → Inhaltsbasierte Füllung…** (**Umschalt+Rücktaste**) füllt die Auswahl aus ihrer Umgebung. **Abtastbereich** ist
  Auto oder Um die Auswahl herum; in diesem Fall liegt der Rand bei 16 bis 512 px. **Auswahl erweitern** vergrößert den Bereich um
  0 bis 24 px. **Qualität** ist Schnell, Ausgewogen oder Beste. Die Füllung läuft im Hintergrund mit Fortschrittsanzeige und
  Abbrechen-Knopf.
- **Objekt entfernen** (in der Gruppe Retusche der Werkzeugleiste) entfernt, was du übermalst. Male quer über das Objekt mit einem
  Pinsel von 4 bis 200 px und lasse los. Der Bereich wird dann aus seiner Umgebung gefüllt.

### Reiter Farben

- Ein Farbfeld mit Farbtonleiste, ein Hex-Feld und ein Deckkraft-Regler.
- **Harmonien** der Vordergrundfarbe: Komplementär, Analog, Triadisch, Split-Komplementär, Tetradisch und Monochrome
  Schattierungen. Klicke eine an, um sie zu übernehmen.
- **Farbfelder** sind gespeicherte Farben, bis zu 48. Klick setzt die Vordergrundfarbe, Rechtsklick die Hintergrundfarbe,
  Umschalt+Klick entfernt sie.
- **Farbpalette aus Bild** ermittelt die Hauptfarben des Bildes. Wähle 5, 6, 8, 10, 12 oder 16 Farben (Standard 8) und drücke
  **Ermitteln**. Das Ergebnis kannst du zu den Farbfeldern hinzufügen, als Verlauf verwenden oder als Palettenebene anlegen.
- **Vorgaben** und ein Pipetten-Knopf gehören ebenfalls zu diesem Reiter.

### Bildaktionen

Das Menü **Bild** enthält vier Aktionen:

- **Transparente Pixel zuschneiden** entfernt transparente Ränder.
- **Automatisch auf Inhalt zuschneiden** schneidet den Hintergrund weg, also die Farbe in den Ecken oder die Transparenz.
- **Leinwandgröße…** (**Strg+Alt+C**) setzt eine neue Breite und Höhe. Das alte Bild sitzt an einem Anker aus einem 3 × 3-Raster.
  Der neue Bereich ist Transparent, Vordergrund, Hintergrund, Weiß oder Schwarz. Die Leinwand darf bis zu 16384 px pro Seite und
  120 Millionen Pixel groß sein.
- **Leinwand an Ebenen anpassen** verkleinert die Leinwand auf den Bereich, den die Ebenen abdecken.

## Projektdateien

Pixel speichert drei Formate:

- **OpenRaster (`.ora`)** behält das ganze Projekt: jede Ebene mit Name, Reihenfolge, Deckkraft, Sichtbarkeit, Mischmodus, Maske,
  Schnittmaske und Sperren. Es ist eine Standard-OpenRaster-Datei, GIMP und Krita öffnen also die Ebenen. Eine zusammengefügte
  Vorschau und ein Miniaturbild sind enthalten. Sperren und Schnittmasken stehen in zusätzlichen Attributen, die andere Programme
  ignorieren. Stickerebenen werden als Bilder gespeichert.
- **PNG** und **JPEG** speichern das sichtbare Bild als ein zusammengefügtes Bild. Beim ersten Speichern eines Dokuments mit
  mehreren Ebenen auf diesem Weg zeigt Pixel den Hinweis: „Ebenen werden zusammengefügt – speichere als .ora, um sie zu behalten.“

Im Speichern-Dialog steht OpenRaster an erster Stelle, wenn das Dokument mehrere Ebenen oder Masken hat. Sonst stehen PNG und JPEG
vorn. Der Öffnen-Dialog zeigt Bilder und OpenRaster-Projekte. Auch das Ziehen einer `.ora`-Datei auf das Fenster öffnet sie.

Ein OpenRaster-Projekt mit mehr als 64 Ebenen lässt sich nicht öffnen. Ein Projekt, dessen entpackte Ebenen mehr als 1 GiB
Arbeitsspeicher brauchen, wird abgelehnt.

## Grenzen

| Element | Grenze |
|---|---|
| Ebenen pro Dokument | 64 |
| Rückgängig-Schritte (Reiter Verlauf) | 60. Ältere Schritte fallen weg, und der Speicher für die Schritte ist auf etwa 768 MiB begrenzt. |
| Leinwandgröße | 16384 px pro Seite und 120 Millionen Pixel |
| Neues Dokument aus einem Generator | 16 bis 8192 px pro Seite (Standard 1024 × 1024) |
| PNG- und JPEG-Dateien | Bis 32 MiB und 32 Megapixel. Ist im Virtual Desktop eine kleinere Dateigrößengrenze gesetzt, gilt diese. |
| OpenRaster-Projekt | Bis 32 MiB, beim Öffnen bis 64 Ebenen und bis 128 Einträge im Archiv |
| KI-Freistellen | Das Bild wird vor dem Hochladen auf höchstens 2048 px an der längeren Seite verkleinert. Ein Lauf gleichzeitig, nach 180 Sekunden abgebrochen. |
| Filtervorschau | Die Vorschau läuft bei großen Bildern zuerst auf einer kleineren Kopie |

## Tastenkürzel

Die Tabelle zeigt die im Editor registrierten Tasten. Strg und Cmd sind dieselbe Taste.

### Datei, Bearbeiten und Auswahl

| Taste | Aktion |
|---|---|
| `Strg+O`, `Strg+S`, `Strg+Umschalt+S` | Öffnen, Speichern, Speichern unter |
| `Strg+Z`, `Strg+Umschalt+Z` | Rückgängig, Wiederholen |
| `Strg+C`, `Strg+X`, `Strg+V` | Kopieren, Ausschneiden, Einfügen |
| `Entf` | Auswahl löschen (in der Ebenenliste wird die Ebene gelöscht) |
| `Strg+A`, `Strg+D`, `Strg+Umschalt+D`, `Strg+Umschalt+I` | Alles auswählen, Auswahl aufheben, Auswahl wiederherstellen, Auswahl umkehren |
| `Umschalt+F6` | Weiche Kante der Auswahl |
| `Strg+J` | Ebene über Kopie |
| `Strg+Umschalt+J` | Auswahl in eine neue Ebene ausschneiden |
| `Alt+Rücktaste`, `Strg+Rücktaste` | Auswahl mit der Vordergrund- beziehungsweise Hintergrundfarbe füllen |
| `Umschalt+Rücktaste` | Inhaltsbasierte Füllung |
| `Strg+T` oder `Strg+Alt+T` | Frei transformieren |
| `Escape` | Laufende Aktion abbrechen: Zuschnitt, Strich, Form, Transformation, Verschiebung oder Texteingabe; sonst Auswahl aufheben |
| Pfeiltasten | Auswahl um 1 px verschieben, mit Umschalt um 10 px |

### Werkzeuge

| Taste | Werkzeug |
|---|---|
| `V`, `Umschalt+V`, `L`, `Umschalt+K`, `W`, `Umschalt+W` | Rechteckauswahl, Ellipsenauswahl, Lasso, Polygon-Lasso, Zauberstab, Schnellauswahl |
| `M` | Verschieben |
| `B`, `P`, `A`, `E` | Pinsel, Bleistift, Airbrush, Radierer |
| `S`, `Umschalt+J`, `J`, `Y`, `N`, `D`, `Umschalt+B`, `Umschalt+E` | Kopierstempel, Reparaturpinsel, Bereichsreparatur, Verwischen, Weichzeichner- / Schärfepinsel, Abwedeln und Nachbelichten, Farbe ersetzen, Rote Augen |
| `G`, `U`, `I` | Füllen, Farbverlauf, Pipette |
| `R`, `Umschalt+R`, `O`, `Umschalt+O`, `Umschalt+S`, `Umschalt+L`, `Umschalt+A` | Rechteck, Abgerundetes Rechteck, Ellipse, Polygon, Stern, Linie, Pfeil |
| `T` | Text |
| `H`, `Z`, `C` | Hand, Zoom, Zuschneiden |

### Pinseleinstellungen

| Taste | Aktion |
|---|---|
| `[`, `]` | Pinsel kleiner und größer |
| `Umschalt+[`, `Umschalt+]` | Weichere und härtere Kante |
| `1` bis `9`, `0` | Deckkraft 10 % bis 90 %, bei `0` 100 % |
| Umschalt+Klick | Gerade Linie vom Ende des letzten Strichs |
| Alt+Klick | Farbe unter dem Zeiger aufnehmen (Malwerkzeuge) |

### Ansicht, Befehle und Filter

| Taste | Aktion |
|---|---|
| Leertaste und Ziehen | Verschieben |
| `Strg` und Mausrad | Zoomen |
| `Strg++`, `Strg+-`, `Strg+0` | Vergrößern, Verkleinern, Einpassen |
| `Strg+K` oder `Strg+Umschalt+P` | Befehlspalette |
| `?` oder `F1` | Tastenkürzel anzeigen |
| `Strg+Alt+C` | Leinwandgröße |
| `Strg+F` | Letzten Filter wiederholen |
| `Strg+Alt+F` | Letzter Filter mit Dialog |
| `Strg+Umschalt+L` | Auto-Verbesserung |

### Ebenen

| Taste | Aktion |
|---|---|
| `Strg+Umschalt+Alt+N` | Ebene hinzufügen |
| `F2` | Ebene umbenennen |
| `Strg+E`, `Strg+Umschalt+E` | Nach unten zusammenfügen, Sichtbare zusammenfügen |
| `Strg+Alt+G` | Schnittmaske |
| `Strg+]`, `Strg+[` | Nach vorne, nach hinten (auch `Strg+Umschalt+↑` und `Strg+Umschalt+↓`) |
| `Strg+Umschalt+]`, `Strg+Umschalt+[` | Nach ganz vorne, nach ganz hinten |
| `Alt+]`, `Alt+[` | Ebene darüber oder darunter wählen (auch `Alt+↑` und `Alt+↓`) |

Ist die Ebenenliste aktiv, wählen die Pfeiltasten Ebenen, Pos1 und Ende gehen zur obersten und untersten Ebene, die Leertaste
blendet die aktive Ebene ein oder aus, und Enter öffnet ihre Eigenschaften.

## Datenschutz und Sicherheit

- **Auf deinem Gerät.** Malen, Filter, Generatoren, das klassische Freistellverfahren, Ebenen und Speichern laufen in deinem
  Browser und auf deinen Dateien im Virtual Desktop. Pixel lädt keine externen Schriften oder Bibliotheken.
- **KI-Freistellen.** Dein Bild geht nur an deinen AuraGo-Server, der das Modell lokal ausführt. Die Modelldateien stammen aus dem
  rembg-Release auf GitHub, und nur, wenn du auf **Modell laden** klickst. Jede Datei wird anhand ihrer Größe und des
  SHA-256-Hashwerts geprüft.
- **Provider-Funktionen.** Generieren, Verbessern, Hintergrund entfernen und Hochskalieren im Reiter **KI** schicken deinen Prompt
  und dein Bild an den unter **Konfiguration** eingestellten Bild-Provider. Ohne Provider sind sie nicht verfügbar.
- **Wohin Dateien gehen.** Gespeicherte Bilder landen im Virtual-Desktop-Ordner, den du wählst. Die KI-Modelle liegen auf dem Server
  im Datenverzeichnis von AuraGo.
- **Schreibgeschützt.** In einem schreibgeschützten Virtual Desktop sind Speichern und Modell-Downloads gesperrt. Du kannst ein Bild
  im Fenster weiterhin ansehen und bearbeiten, die Änderungen lassen sich aber nicht speichern.

## Wenn etwas schiefgeht

| Meldung | Was hilft |
|---|---|
| „Diese Ebene ist gesperrt.“ | Hebe die Sperre der Ebene in der Kopfzeile der Ebenen auf. |
| „Maximale Anzahl an Ebenen erreicht (64).“ | Füge Ebenen zusammen oder lösche welche. |
| „Die letzte Ebene kann nicht gelöscht werden.“ | Ein Dokument behält immer mindestens eine Ebene. |
| „Erst eine Auswahl treffen.“ | Triff eine Auswahl, oder drücke **Strg+A**, um alles auszuwählen. |
| „Ein Freistellvorgang läuft bereits.“ | Warte, bis er fertig ist, oder drücke im Reiter **Freistellen** auf **Abbrechen**. |
| „Das KI-Freistellen ist fehlgeschlagen (…). Stattdessen wird das klassische Verfahren verwendet.“ | Der Grund steht in der Klammer: Das Modell ist nicht geladen (lade es im Reiter **Freistellen**), es ist auf diesem Gerät nicht verfügbar, ein anderer Freistellvorgang läuft, die Zeit ist abgelaufen, der Virtual Desktop ist schreibgeschützt oder der Server konnte es nicht ausführen. |
| „Hier wurden keine roten Augen gefunden.“ | Klicke näher an die Pupille oder vergrößere den Suchradius. |
| „Um die Auswahl gibt es nichts, woraus kopiert werden könnte. Wähle einen kleineren Bereich.“ | Wähle für die inhaltsbasierte Füllung einen kleineren Bereich. |
| „Das Bild ist vollständig transparent.“ | Beim Zuschneiden blieb nichts Sichtbares übrig. |
| „Diese Leinwandgröße ist nicht möglich.“ | Wähle eine kleinere Breite oder Höhe (bis 16384 px pro Seite und 120 Millionen Pixel). |
| „Generator fehlgeschlagen“ | Prüfe Breite und Höhe des neuen Dokuments, oder probiere einen anderen Seed. |
| „Das Bild konnte nicht platziert werden.“ | Prüfe, ob die Datei ein PNG- oder JPEG-Bild ist. |
| „Die Projektdatei ist kein gültiges OpenRaster-Archiv.“ | Die Datei ist beschädigt oder kein OpenRaster-Projekt. Speichere sie mit dem Programm, das sie erstellt hat, erneut. |

---

**Siehe auch:** [Kapitel 4: Web-Oberfläche](./04-webui.md) · [Kapitel 21: API-Referenz](./21-api-reference.md#pixel-bildeditor-api) · [Kapitel 14: Sicherheit](./14-security.md) · [Kapitel 16: Troubleshooting](./16-troubleshooting.md)

---

**Vorheriges Kapitel:** [Kapitel 30: RuView](./30-ruview.md)
