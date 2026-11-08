# Kapitel 36: Synth Studio – Synthesizer und DAW im Virtual Desktop

Synth Studio ist ein Synthesizer und eine kleine DAW im Virtual Desktop. Der Ton entsteht auf dem Rechner, der den Browser
ausführt; der AuraGo-Server speichert nur die Projektdateien über die reguläre Desktop-Datei-API. Einen Cloud-Musikdienst und
einen Audio-Nebenprozess brauchst du nicht.

Du ordnest Spuren und Clips an, designst eigene Klänge, spielst mit Akkordpads und Arpeggios, programmierst Patterns live in
Strudel und mischst das Ergebnis. Der vollständige Vertrag steht im [Synth-Studio-Handbuch](../../synth-studio.md).

## Inhaltsverzeichnis
1. [Synth Studio öffnen](#synth-studio-öffnen)
2. [Die fünf Arbeitsmodi](#die-fünf-arbeitsmodi)
3. [Die Bibliothek](#die-bibliothek)
4. [Regler, Fader und Kurven](#regler-fader-und-kurven)
5. [Der Master-Streifen](#der-master-streifen)
6. [Strudel-Livecoding](#strudel-livecoding)
7. [Musikalische Werkzeuge](#musikalische-werkzeuge)
8. [MIDI-Tastatur und Dateien](#midi-tastatur-und-dateien)
9. [Speichern und Exportieren](#speichern-und-exportieren)
10. [Wenn kein Ton kommt](#wenn-kein-ton-kommt)

---

## Synth Studio öffnen

Öffne **Synth Studio** aus dem Startmenü, in der Kategorie **Kreativ**. Das Fenster öffnet sich maximiert.

Die Transportleiste sitzt über der Modusleiste: Abspielen, Pause, Stopp und Aufnahme, die Positionsanzeige, das Temposfeld,
das Metronom, den Loop-Bereich, Rückgängig und Wiederherstellen, die Schaltfläche **Master** und den Analyser. Die Leertaste
spielt und pausiert, Strg/Cmd+S speichert und Strg/Cmd+Z macht rückgängig.

Ein Projekt nutzt 4/4 und ein konstantes Tempo von 40–240 BPM, mit höchstens fünfzehn melodischen Spuren und einer Schlagspur.
Die Schaltfläche **Demo** lädt „Neon Sketch“, einen Synthwave-Song in a-Moll mit 112 BPM über 32 Takte; die Vorlagen
Electronic, Chill und Retro sind Sechzehntakt-Sätze, die pausiert starten. Ein verborgenes Browser-Tab pausiert die
Wiedergabe; wenn du nur das Desktop-Fenster minimierst, läuft sie weiter.

## Die fünf Arbeitsmodi

Die Transportleiste bleibt über einer Modusleiste mit fünf Modi sichtbar; nur der aktive Modus wird angezeigt.

| Modus | Wofür er da ist |
|-------|-----------------|
| **Arrangieren** | Die Arrangierung mit Spuren und Clips. Der Noteneditor (Piano-Roll oder Schlagraster) erscheint unter der Arrangierung nur, solange ein Clip ausgewählt ist. |
| **Sound-Design** | Das Klangpanel der gewählten Spur, mit dem programmierbaren Synthesizer und der Effektkette. |
| **Spielen** | Akkordpads, Tonleitenhilfe, Arpeggiator, Variationen, Szenen und die momentanen Live FX. |
| **Strudel** | Der Livecoding-Editor. |
| **Mixer** | Der Master-Streifen mit den Effekt-Einschüben. |

Spielen gruppiert seine Regler in Karten: Szenen, das XY-Pad mit Tone-/Reverb-Reglern, der Take-Aufnehmer, Live FX, der
Automatisierungseditor und der Komponist, eine Karte je Generator. Doppelklicke eine freie Spur, um einen Clip anzulegen, und
wähle einen Clip aus, um die Noten im Piano-Roll oder im Schlagraster zu bearbeiten.

## Die Bibliothek

Die Klangbibliothek ist eine Schublade, die von der Seite hereinfährt. Standardmäßig ist sie zu; öffne und schließe sie mit
der Bibliotheksschaltfläche im Kopf, und der Zustand der Schublade wird gemerkt.

- **Werksbänke.** Fünf Bänke mit je vierundzwanzig Synthesizer-Voreinstellungen — Drums, Bass, Leads, Pads und Keys/Plucks —
  insgesamt 120 Werksklänge. Es sind kompakte synthetisierte Näherungen, keine aufgenommenen akustischen Orchester.
- **General MIDI.** Die zusätzliche General-MIDI-Bank enthält 128 synthetisierte Klänge und ein Schlagzeug.
- **Meine Synths.** Programmiere mit dem eingebauten Synthesizer einen eigenen Klang, dann landet er in der Bank „Meine
  Synths“, erscheint in den Instrumentlisten wie eine Werksbank und bleibt in diesem Browser. Es werden höchstens
  vierundzwanzig eigene Synths behalten.
- **Suche und Schlagworte.** Die Bibliothek trennt Auswahl, Vorschau und Einfügen und sucht in allen Bänken, bis du eine
  Kategorie wählst. Die Suche trifft Namen und Schlagworte; Favoriten sowie eine Liste der zuletzt benutzten Klänge bleiben
  pro Browser.
- Das **Synth Studio Kit** deckt die 47 Schlagnoten von 35 bis 81 mit einer stimmigen Stimme ab. Das separate
  Instrument **GM Drums** folgt stattdessen der General-MIDI-Schlagkarte.

## Regler, Fader und Kurven

Bereichsschieberegler werden als 3D-Knobs gezeichnet, die Spur Lautstärke als waagerechter Fader. Ziehe einen Knob nach oben
oder unten, um ihn zu ändern; mit Umschalt sind die Schritte feiner. Hat ein Knob den Tastaturfokus, ändert das Mausrad ihn,
und die Pfeiltasten verschieben ihn in Schritten. Ein Doppelklick löst die Schaltfläche Reset der Zeile aus, wenn es sie gibt.

Der Hüllkurven-Abschnitt des Sound-Designers benutzt vier senkrechte Fader (Anstieg, Abfall, Halten, Ausklang) neben einer
kleinen Hüllkurvenform, die deinen Zug vorwegnimmt. Ein senkrechter Fader verhält sich wie die anderen Regler: Track ziehen
oder anklicken, Mausrad und Pfeiltasten im Fokus, und ein Doppelklick bringt den Wert auf den Standard des Patches zurück. Der
Designer bearbeitet einen Browser-Entwurf, ein Faderzug ändert also den Entwurf und die Vorschau, erzeugt aber nie einen
Undo-Schritt im Projekt.

Das Klangpanel bietet **Design sound** an und macht daraus einen vollen Synthesizer über dem Patch-Modell der Stimmen-Engine:
Oszillatoren, Sub-Oszillator und Rauschen, drei FM-Modulatoren, das Filter, die Lautstärkehüllkurve, die Tonhöhenkurve und ein
LFO. Jede Änderung meldet sofort ein Vorschaumodul neu an, sodass die Anhörtasten immer den aktuellen Entwurf spielen.

## Der Master-Streifen

Die Schaltfläche **Master** in der Transportleiste öffnet den Master-Streifen: Ausgangspegel, den Kompressor mit Schwellwert,
Verhältnis, Knie, Anstieg und Ausklang, eine dreikanalige EQ, einen Brickwall-Limiter, die Stereo-Breite und den Charakter der
gemeinsamen Plate- und Echo-Busse.

Der Ausgangspegel ist ein großer Hero-Knob mit Dezibel-Anzeige (20 × log10 der Verstärkung, `-inf` bei null) und daneben eine
16-segmentige LED-Anzeige (grün, gelb, rot über 48 dB). Die Anzeige liest denselben Analyser-Peak, läuft nur, solange der
Streifen offen ist, und verändert den Ton nie.

Die EQ-Gruppe zeigt den live berechneten Frequenzgang der drei Bänder, und der Sound-Designer zeigt den Frequenzgang des
Stimmenfilters (Typ, Grenzfrequenz, Resonanz) mit einer Grenzfrequenzmarke. Die Kurven sind reines SVG, berechnet aus
denselben Filtertypen, Frequenzen und Grenzfrequenzabbildungen, die auch die Audio-Engine nutzt.

Jede Spur trägt eine geordnete Kette aus bis zu vier Effekt-Einschüben plus zwei Sends. Zur Auswahl stehen Chorus, Phaser,
Tremolo, Flanger, Rotary, Bitcrush, Soft Clip, dreikanalige EQ, Stereo-Breite, Cabinet, Delay, Reverb, Tape, Auto Pan und Ring
Mod. Jeder Einschub stellt seine eigenen Charakterparameter — Mix, Rate, Tiefe, Feedback, Ton und typabhängige Werte — statt
eines einzigen festen Klangs. Die Kette läuft nach dem Spur-Pan, und die Reverb- und Delay-Sends verlassen den Fader, weshalb
die Spurlautstärke auch ihre eigenen Ausklänge regelt.

## Strudel-Livecoding

Wechsle in den Modus **Strudel**, um Patterns mit der lokalen Strudel-1.3.0-Engine zu schreiben und laufen zu lassen.
**Run / Update** oder Strg+Enter wertet den aktuellen Code aus; Updates behalten die laufende Uhr. Die Stopp-Schaltfläche und
Stopp/Pause der Haupttransportleiste halten ihn sofort an. Ein Start der Arrangierung stoppt Strudel, und Strudel startet die
Arrangierung nicht. Das Schließen des Fensters, das Verbergen des Browser-Tabs oder das Entziehen der Desktop-Schreibrechte
stoppen ebenfalls den Evaluator.

Die Beispiele Melody, Groove und Ambient benutzen eingebaute Synthesizer. Tutorial-Patterns wie `s("bd sd cp hh")`
funktionieren mit lokal synthetisierten Kick, Snare, Clap und Hi-Hat statt mit den Samples des Online-REPLs. Es wird nichts
aus dem Netz geladen und kein Sample heruntergeladen. Externe Sample-Bänke (`samples(...)`), MIDI und Mikrofonzugriff sind im
Evaluator gesperrt.

Der Skizzenentwurf (höchstens 64 KiB) bleibt in diesem Browser. **Open** importiert eine `.strudel`-Textdatei nach einer
Rückfrage; **Download code** speichert eine portable Kopie. Dieser Code ist getrennt von der `.aurasynth`-Datei der
Arrangierung, der Undo-Historie und den WAV-, MIDI- und Spur-Exporten, also exportiere ihn, bevor du den Browser-Speicher
leerst. Ist Autoplay blockiert, klicke auf die Sound-Schaltfläche in der Workbench. Fehler erscheinen über dem Editor und
halten den Evaluator an; behebst du den Code und startest ihn neu, beginnt eine frische Instanz. Der Evaluator hat einen
undurchsichtigen Sandbox-Origin und kann weder das DOM noch den Speicher, die Zugangsdaten oder die API-Antworten des Desktops
lesen. Strudel und seine Integration stehen unter AGPL-3.0-or-later.

## Musikalische Werkzeuge

Der Modus Spielen bietet acht diatonische Akkordpads, Tonleitenhilfe für Dur und Moll, einen live spielbaren und druckbaren
Arpeggiator, gesetzte melodische und rhythmische Variationen und einen euklidischen Schlaggenerator. Halte die Pads oder
benutze die Bildschirm-, Tastatur- oder MIDI-Tastatur für live gespielte Arpeggios; der Latch-Schalter lässt den letzten
Akkord nach dem Loslassen weiterlaufen.

Alle Generatoren folgen demselben Ablauf: Clip wählen, Vorschau vorbereiten, Original und Variante in einer Schleife
vergleichen, dann Apply oder Cancel. Apply schreibt gewöhnliche Noten als einen Undo-Schritt. Der sichtbare Startwert, die
eingegebenen Noten und die Regler bestimmen das wiederholbare Ergebnis. Tonleiterbeschränkungen wirken nur auf neue Noten.
Neue Projekte starten in C-Dur ohne Beschränkungen.

| Werkzeug | Was es tut |
|----------|------------|
| **Groove-Labor** | Quantisiert, swingt, humanisiert und transponiert einen vorhandenen Clip in einem deterministischen, gesetzten Durchgang. |
| **Akkordfolge-Generator** | Schreibt eine ganze diatonische Akkordfolge in den Clip: Pop, Ballade, Jazz, Moll, Episch oder Kanon. |
| **Basslinien-Generator** | Folgt derselben Akkordfolge mit Root-, Oktav-, Walking- oder Driving-Eighth-Mustern. |
| **Clip umkehren** | Spiegelt den gewählten Clip in der Zeit. |

Die Live FX liegen hinter dem Master-Streifen und sind momentan: **Filter-Drop** zieht den Master-Tiefpass über 1,5 Beats nach
unten und wieder zurück, **Stutter** zerhackt den Mix sechzehnmal über einen Takt. Sie folgen dem aktuellen Tempo, sind reine
Performance und kommen weder ins Projekt noch in den WAV-Mixdown noch in den Spur-Export.

Lege bis zu acht benannte Szenen aus den aktuellen Loop-Grenzen an. Eine Szene wiederholt diesen Arrangierungsbereich; ein
weiteres Pad reiht sie für den nächsten Takt ein, und weitere Pads ersetzen diese wartende Wahl. Stopp wirkt sofort und
verwirft wartende Aktionen. Das XY-Pad regelt den Ton der gewählten Spur waagerecht und die Reverb senkrecht. Aktiviere Record
automation vor einer Geste während der Wiedergabe, um Ton-/Reverb-Punkte zu schreiben; eine Geste ist ein Undo-Schritt.

## MIDI-Tastatur und Dateien

Öffne das MIDI-Panel und verbinde eine Tastatur ausdrücklich. Web MIDI braucht einen unterstützenden Browser und einen
sicheren Kontext (HTTPS oder eine vom Browser anerkannte lokale Herkunft). Ein fehlendes Gerät oder eine verweigerte
Berechtigung blockiert das Bearbeiten mit der Maus nicht, und angefordert wird nur MIDI-Eingabe; SysEx und MIDI-Ausgabe werden
nicht benutzt.

Wähle die Aufnahmespur, aktiviere das optionale Ein-Takt-Einzählen und nimm auf. Die Quantisierung folgt dem aktuellen Raster.
Der Latenzversatz von -200 bis +200 ms gleicht das Eingabegerät und den Browser aus; stelle ihn mit deiner echten Tastatur
ein. Eine Aufnahme ist auf 64 Takte begrenzt; erreicht sie das Grenzwert für Ereignisse, Dateigröße oder Zeitleiste, enden die
bereits aufgenommenen Noten, und teile längere Darbietungen in aufeinanderfolgende Clips auf.

Der Import akzeptiert begrenzte Standard-MIDI-Dateien vom Typ 0 und Typ 1 mit konstantem unterstütztem Tempo und 4/4-Takt.
Nicht unterstützte Controller-Nachrichten, wechselndes Tempo oder Taktmaß, Typ 2 und SMPTE-Zeitplanung führen zu einem Fehler,
statt die Arrangierung still zu verändern. MIDI-Exporte benutzen General-MIDI-Näherungen für die eigenen
Synth-Studio-Voreinstellungen; nimm WAV für den tatsächlichen Synthesizerklang und die Ton-/Reverb-Automatisierung.

## Speichern und Exportieren

Projekte sind versionierte `.aurasynth`-JSON-Dateien im Desktop-Speicher; der Standardordner ist `Documents/Synth Studio`.
Neue Projekte nutzen Format v3: Harmonie auf Projektebene, bis zu acht benannte Szenen mit Taktbereich, Automatisierungspunkte
auf Spur-Ebene, eine Effektkette je Spur und ein Master-Streifen auf Projektebene. Die Prüfung akzeptiert weiterhin v1 und v2
unverändert. Der Editor hebt eine Kopie im Speicher an; das erste v3-Speichern verlangt im regulären Dialog einen neuen
Dateinamen, so bleibt die Originaldatei geschützt.

Bearbeiten erzeugt lokale Wiederherstellungsentwürfe; benannte Projekte speichern automatisch. Ein fehlgeschlagenes Speichern
lässt das Dokument geändert und behält einen rettbaren Entwurf. Im schreibgeschützten Desktop-Modus kannst du Dateien öffnen
und abspielen, aber nicht bearbeiten, aufnehmen oder speichern.

Das Export-Menü bietet WAV, MIDI und Spuren.

- **WAV** lädt Stereo-44,1-kHz-PCM16 mit demselben Renderer wie die Wiedergabe herunter, einschließlich der Spureffekte. Es
  rendert einen endlichen Durchgang der Arrangierung und hängt einen Effektausgang an; die Transportschleife und das
  Metronom werden nicht exportiert. Der gesamte Render einschließlich Ausgang ist auf fünf Minuten, 12.000 Noten und 256
  überlappende Stimmen begrenzt.
- **Spuren** rendern je eine WAV pro Spur nach dem Fader und vor dem Master, mit Noten, und packen sie als store-only ZIP mit
  UTF-8-Namen. Spuren ohne Noten werden übersprungen; Masterpegel, EQ, Kompression, Limiting und Breite werden
  neutralisiert, während der Plate-/Echo-Charakter und die Sends hinter dem Fader erhalten bleiben, damit die Spuren
  anderswo gemischt werden können.
- **MIDI** exportiert die Arrangierung für andere Programme, mit General-MIDI-Näherungen für deine eigenen Synths.

## Wenn kein Ton kommt

| Symptom | Was du prüfst |
|---------|---------------|
| Es ist überhaupt nichts zu hören | Öffne den Master-Streifen mit der Schaltfläche **Master** und lies den Ausgangspegel ab: der Hero-Knob zeigt den Dezibelwert und liest bei null `-inf`. Drehe ihn hoch. |
| Die LED-Anzeige bleibt dunkel | Die Anzeige läuft nur, solange der Master-Streifen offen ist, und liest den Analyser-Peak. Öffne den Streifen und sieh dir den Analyser in der Transportleiste an. |
| Eine Spur ist stumm | Prüfe den Spurkopf: Dort liegen Lautstärke, Stummschaltung und Solo. Die Spurlautstärke ist ein waagerechter Fader, und Reverb- und Delay-Sends verlassen den Fader, sie regelt also auch ihre eigenen Ausklänge. |
| Der Ton bricht ab, wenn du in eine andere App wechselst | Ein verborgenes Browser-Tab pausiert die Wiedergabe; wenn du nur das Desktop-Fenster minimierst, läuft sie weiter. |
| Der Modus Strudel ist stumm oder das Pattern hört von selbst auf | Ist Autoplay blockiert, klicke auf die Sound-Schaltfläche in der Workbench. Das Schließen des Fensters, das Verbergen des Browser-Tabs oder das Entziehen der Desktop-Schreibrechte stoppen den Evaluator. |
| Die MIDI-Aufnahme kommt zu früh oder zu spät | Stelle den Latenzversatz im MIDI-Panel von -200 bis +200 ms ein und kalibriere ihn mit deiner Tastatur. |
| Eine lange Aufnahme hört zu früh auf | Ein Take ist auf 64 Takte begrenzt und endet auch an der Grenze für Ereignisse, Dateigröße oder Zeitleiste. Teile längere Darbietungen in aufeinanderfolgende Clips auf. |
| Der Export ist stumm oder bricht ab | WAV rendert einen endlichen Durchgang der Arrangierung und hängt einen Effektausgang an; die Transportschleife und das Metronom werden nicht exportiert. Der gesamte Render einschließlich Ausgang ist auf fünf Minuten, 12.000 Noten und 256 überlappende Stimmen begrenzt. |
| Es lässt sich nichts speichern | Im schreibgeschützten Desktop-Modus funktionieren Öffnen und Abspielen, aber Bearbeiten, Aufnehmen und Speichern sind abgeschaltet. |
| Dein eigener Synth fehlt in einem anderen Browser | Eigene Synths liegen nur im Browser. Ein Projekt aus einem anderen Browser öffnet trotzdem und behält seine Spur, der Synth erscheint dort aber als „Missing synth“, bis du ihn dort neu designst. |

---

**Siehe auch:** [Kapitel 4: Web-Oberfläche](./04-webui.md) · [Kapitel 22: Interne Tools](./22-internal-tools.md) · [Kapitel 26: Video Studio](./26-video-studio.md) · [Kapitel 14: Sicherheit](./14-security.md) · [Kapitel 16: Troubleshooting](./16-troubleshooting.md)

---

**Vorheriges Kapitel:** [Kapitel 35: Bitte nicht das Universum füttern](./35-goldfish-adventure.md)
