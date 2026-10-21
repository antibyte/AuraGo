# Kapitel 26: Video Studio – Videos im Virtuellen Desktop schneiden

Video Studio ist der Mehrspur-Videoschnitt im Virtuellen Desktop. Du legst Clips, Musik und Bilder auf eine Zeitleiste,
fügst Titel, Effekte und Übergänge hinzu und exportierst eine MP4-Datei, die AuraGo auf dem Server mit FFmpeg rendert.
Gedacht ist es für Projekte von bis zu 60 Minuten. Es ist keine Kamera-App, kein Untertitel-Generator und kein
KI-Schnittassistent, und es ist ab Werk ausgeschaltet.

Technische Details für Admins stehen in der [Betriebsreferenz](../../video-studio.md) (englisch).

## Inhaltsverzeichnis
1. [Einschalten](#einschalten)
2. [Erste Schritte](#erste-schritte)
3. [Medien und Zeitleiste](#medien-und-zeitleiste)
4. [Keyframes und Geschwindigkeitsrampen](#keyframes-und-geschwindigkeitsrampen)
5. [Farbe, Effekte, Masken und Keying](#farbe-effekte-masken-und-keying)
6. [Übergänge und Animationen](#übergänge-und-animationen)
7. [Vorschau, Bereichsprüfung und Export](#vorschau-bereichsprüfung-und-export)
8. [Projekte und Versionen](#projekte-und-versionen)
9. [Optionale KI-Clips](#optionale-ki-clips)
10. [Gut zu wissen: Grenzwerte](#gut-zu-wissen-grenzwerte)
11. [Tastenkürzel](#tastenkürzel)
12. [Datenschutz und Sicherheit](#datenschutz-und-sicherheit)
13. [Wenn etwas nicht klappt](#wenn-etwas-nicht-klappt)

---

## Einschalten

Video Studio ist ab Werk ausgeschaltet. Ein Admin schaltet es unter **Config → Virtueller Desktop** ein:

- **Virtuellen Desktop aktivieren** muss zuerst an sein. Erst dann erscheinen die Einstellungen von Video Studio.
- **Config → Virtueller Desktop → Video Studio → „Video Studio aktivieren“** (`video_studio.enabled`, Voreinstellung
  `false`). Ein Neustart ist nicht nötig; der Server liest die gespeicherten Werte bei jeder Anfrage.
- **Projekte schreibgeschützt öffnen** (`video_studio.readonly`, Voreinstellung `false`) öffnet jedes Projekt
  schreibgeschützt, wie der **Schreibgeschützte Modus** des Virtuellen Desktops. Abspielen und das Stoppen laufender
  Aufgaben gehen weiter, Änderungen nicht.
- **Video Studio prüfen** zeigt, ob der Server bereit ist (vorher speichern). Ein gutes Ergebnis lautet „Video Studio ist
  bereit.“ und nennt den Encoder.
- **FFmpeg:** Das Docker-Image enthält FFmpeg und FFprobe. Native Installationen brauchen beide im `PATH` oder einen Pfad
  unter **FFmpeg-Programmpfad (leer: automatisch)** (`video_studio.ffmpeg_path`). Effekte und Übergänge, die das FFmpeg
  des Servers nicht darstellen kann, sind im Editor ausgegraut.
- **Agent:** Video Studio fügt dem Agenten kein Werkzeug hinzu. Er kann deine Projekte weder öffnen noch ändern. KI-Clips
  laufen über die separate Videogenerierung.

Solange es aus ist, erscheint die App im Startmenü (Kategorie **Kreativ**) und zeigt „Video Studio ist in den
Einstellungen deaktiviert.“

## Erste Schritte

1. Öffne **Video Studio** aus dem Startmenü (Kategorie **Kreativ**). Das Fenster öffnet sich maximiert.
2. Klicke **Neues Projekt**, wähle Namen, Format und Bildrate und klicke dann **Projekt anlegen**. Formate: Querformat
   720p, 1080p, 1440p oder 4K; Hochformat 720p, 1080p oder 4K; quadratisch 720p, 1080p oder 2160p; 4:5 mit
   1080 × 1350. Bildraten: 24, 25, 30, 50 oder 60 fps, dazu die NTSC-Raten 23,976, 29,97 und 59,94 fps.
3. Füge Medien mit **Hochladen** hinzu, mit **Aus Dateien** (Desktop-Dateien) oder indem du Dateien in das Fenster ziehst.
4. Drücke **+** bei einem Bibliothekseintrag, um ihn an die Abspielposition zu setzen, oder ziehe ihn auf eine Spur.
   Abspielen mit der Transportsteuerung oder mit `J`, `K` und `L`.

## Medien und Zeitleiste

Importe werden in das Projekt kopiert, deine Originaldatei ändert sich nie. Der Dateidialog bietet Video, Audio, PNG- und
WebP-Bilder sowie `.cube`-LUTs an. Gängige Formate funktionieren, zum Beispiel MP4, MOV, MKV, WebM, AVI, MP3, WAV, FLAC
und GIF. Ein Video, dessen längere Seite mehr als 16-mal so lang ist wie die kürzere, wird abgelehnt.

Die Zeitleiste hat bis zu acht Video-, acht Audio- und acht Overlay-Spuren und bis zu 2000 Clips. Ein Clip, den du auf
eine Spur ziehst, überschreibt darunterliegendes Material; mit gehaltener Taste `Strg` fügst du ihn ein. Jeder
Spurkopf hat Stumm-, Ausblende- und Sperrknöpfe und ein Menü `…` für Höhe, Einfügeziel und Löschen.

- **Werkzeuge:** Auswahlwerkzeug (`V`), Trimmwerkzeug (`T`) und Klingenwerkzeug (`B`). Mit dem Trimmwerkzeug ziehst du an
  einem Schnitt, um ihn zu verschieben, oder an einer freien Kante, um die Spur nachrücken zu lassen.
- **Bearbeiten:** `Entf` löscht, `Umschalt+Entf` löscht und schließt die Lücke. **Einrasten** (`N`) rastet an Clips,
  Abspielkopf und Ein- und Ausgangspunkten ein. **Magnetisch** schließt Lücken beim Verschieben und Löschen.
- **Ton:** `Strg+L` verknüpft die markierten Clips, damit Video und Ton zusammen bewegt werden. **Audio lösen** macht den
  Ton zu einem eigenen Clip, **Audio wieder anfügen** holt ihn zurück. Die Geschwindigkeit reicht von 0,25× bis 16×.
  Der Ton behält seine Tonhöhe bis 4×.
- **Overlays:** Staple Videospuren für Bild-in-Bild. Die Vorlagen **Titel**, **Untertitel**, **Bauchbinde** und **Abspann**
  sowie Sticker landen an der Abspielposition auf einer Overlay-Spur. Sticker sind statisch.
- **Quelle:** Ein Doppelklick auf einen Bibliothekseintrag öffnet ihn im Quellmonitor (Schalter **Quelle** und
  **Zeitleiste** über der Vorschau). `I` und `O` setzen Ein- und Ausgangspunkte, `,` fügt ein, `.` überschreibt. Mit
  **Nur Bild** oder **Nur Ton** platzierst du nur einen Teil.

## Keyframes und Geschwindigkeitsrampen

Position, Größe, Drehung, Deckkraft, Zuschnitt und Effektwerte können sich über die Zeit ändern. Klicke im Inspektor im
Bereich **Bild** auf die Stoppuhr neben einem Wert. Der aktuelle Wert wird zum ersten Keyframe, und spätere Änderungen an
der Abspielposition legen weitere an. Die Raute fügt einen Keyframe hinzu oder entfernt ihn, ◀ und ▶ springen zwischen
ihnen. Keyframes bleiben am Bild, Schnitte und Teilungen behalten sie also auf demselben Bild der Quelle. Eine
Eigenschaft hat höchstens 64 Keyframes, ein Clip höchstens 512.

Geschwindigkeitsrampen findest du im Bereich **Animation** eines Videoclips: **Beschleunigen**, **Abbremsen**,
**Zeitlupe in der Mitte**, **Eigene** und **Keine**. **Dauer beibehalten** behält die Länge des Clips. Für eine eigene
Kurve wähle **In der Zeitleiste bearbeiten** und ziehe die Punkte in der Geschwindigkeitsleiste (höchstens 16 Punkte).
Der Ton eines Clips mit Rampe ist stumm; **Audio lösen** behält ihn.

## Farbe, Effekte, Masken und Keying

**Zuschnitt** und **Spiegeln** stehen im Bereich **Bild**; **In der Vorschau zuschneiden** zieht den Zuschnitt direkt auf
dem Bild. Jeder visuelle Clip hat höchstens acht Effekte. Der Reiter **Effekte** der Bibliothek ordnet sie unter
**Farbe**, **Details**, **Stilisieren** und **Keying**:

- **Farbe:** Belichtung, Kontrast, Sättigung, Temperatur, Farbton sowie drei Farbräder für Tiefen, Mitten und Lichter.
- **LUT:** einer von acht eingebauten Looks oder eine Projekt-LUT, oder **.cube hochladen**. Nur 3D-`.cube`-Dateien
  funktionieren: 2 bis 65 Punkte je Achse, höchstens 16 MB je Datei, 32 je Projekt.
- **Weichzeichnen**, **Schärfen** und **Vignette** sind die übrigen Grundeffekte.
- **Chroma-Key:** Stelle die **Keyfarbe** ein (oder **Greenscreen** bzw. **Bluescreen**), erhöhe **Ähnlichkeit** und
  weiche die Kante mit **Weichheit** auf. **Farbsaum entfernen** beseitigt Farbränder.

Der **Mischmodus** mischt einen Clip mit den Spuren darunter: Normal, Multiplizieren, Negativ multiplizieren,
Ineinanderkopieren, Weiches Licht, Hartes Licht, Abdunkeln, Aufhellen, Differenz, Ausschluss, Addieren, Subtrahieren,
Farbig abwedeln und Farbig nachbelichten; Deckkraft und Maske gelten weiterhin. **Maskenform** bietet **Keine**, **Rechteck** und **Ellipse**. **Weiche Kante**
macht die Kante weich, **Maske umkehren** zeigt das Äußere. Die Maske folgt ihrem Clip beim Verschieben, Skalieren und
Drehen.

Die WebGL2-Vorschau zeichnet Effekte, Mischmodi, Masken und Keying mit denselben Formeln wie der Export. Die einfache
2D-Vorschau zeigt Zuschnitt und Spiegelung, aber keine Effekte, Mischmodi oder Masken, und sagt das auch.

## Übergänge und Animationen

Ein **Übergang** verbindet einen Clip mit dem nächsten Clip auf derselben Video- oder Overlay-Spur. Die beiden Clips
überlappen um die Länge des Übergangs, und der Übergang gehört zum ausgehenden Clip. Öffne im Inspektor **Übergang zum
nächsten Clip** und wähle einen von 24 Typen in vier Gruppen: **Einfache Übergänge** (Überblenden, Über Schwarz, Über
Weiß, Über Graustufen, Verpixeln), **Wischblenden**, **Schieben und Zoomen** und **Formen**. Ein neuer Übergang ist eine
halbe Sekunde lang, höchstens drei Sekunden, und kürzer als beide Clips. Typen, die das FFmpeg des Servers nicht
darstellen kann, sind ausgegraut.

Bilder, Titel und Sticker haben eine **Animation am Anfang** und eine **Animation am Ende**: **Blende**, **Nach oben
gleiten**, **Nach unten gleiten**, **Nach links gleiten**, **Nach rechts gleiten**, **Aufploppen**, **Wischen** und
**Unschärfe**. Jede dauert 0,2 bis 3 Sekunden, standardmäßig eine
halbe Sekunde.

## Vorschau, Bereichsprüfung und Export

Die Vorschau auf der Zeitleiste nutzt WebGL2, wenn der Browser es anbietet. Fehlt es oder schlägt es fehl, wechselt sie
auf eine einfachere 2D-Darstellung und zeigt einen Hinweis; Bearbeiten und Export bleiben davon unberührt.
**Vorschauqualität** (neben „Vorschau im Vollbild“) spielt die Originale in **Voll** ab oder die Vorschau-Proxys in
**Automatisch**, **Halb** oder **Viertel**. Proxys (höchstens 960 × 540 Pixel) entstehen nach dem Import im Hintergrund.
Schlägt ein Proxy fehl, läuft das Original.

**Bereich exakt rendern** rendert den Ein-/Ausgangsbereich oder fünf Sekunden um die Abspielposition durch dieselbe
Export-Pipeline. Bereiche über 60 Sekunden werden abgelehnt. Mit **Gerendert** und **Live-Vorschau** vergleichst du; `Esc`
schließt den Vergleich.

**Exportieren** und dann **Video exportieren** bietet **Gesamtes Projekt** oder **Nur der Bereich Ein/Aus** an. Das
Ergebnis ist eine MP4-Datei mit H.264 und AAC und einem Download-Link. Exporte laufen im Hintergrund in Abschnitten von
etwa zehn Sekunden, du kannst also weiterschneiden. Verfolge sie unter **Aufgaben**; **Aufgabe abbrechen** stoppt eine.
Das Schließen des Fensters bricht sie nicht ab. Der Server braucht freien Speicherplatz für Abschnitte, Ton und Datei.

Admins stellen die **Hardware-Encoding** ein (`video_studio.hardware_encoding`). **Automatisch** (Voreinstellung) prüft
die Grafikhardware und nimmt den ersten Encoder, der funktioniert; das Ergebnis gilt zehn Minuten. Die GPU kodiert nur.
Schlägt ein GPU-Export fehl, startet er einmal auf der CPU neu. **Aus (nur CPU)** nutzt immer den Prozessor.

## Projekte und Versionen

**Alle Projekte…** im Menü **Projekt** listet deine Projekte mit Größe und letzter Änderung. Dort kannst du sie öffnen,
umbenennen, duplizieren oder löschen. Eine Kopie enthält Projektdatei, Medien und Poster, aber keine Exporte oder
Versionen. Ein gelöschtes Projekt entfernt seine Medien, Exporte und Versionen endgültig und bricht seine Aufgaben ab.

Beim Speichern bleibt der bisherige Stand als automatische Version erhalten, wenn die neueste Version mindestens zehn
Minuten alt ist (bis zu 20). **Version jetzt speichern** legt eine Version mit optionaler Bezeichnung an; bis zu 10
manuelle Versionen bleiben erhalten. **Wiederherstellen** speichert zuerst deine Änderungen. Eine Version mit inzwischen
gelöschten Medien lässt sich nicht wiederherstellen.

## Optionale KI-Clips

Der Reiter **KI** in der Bibliothek erzeugt kurze Clips aus einem Textprompt über den Anbieter unter
**Config → Videogenerierung** (`video_generation.enabled`, Voreinstellung `false`). Der Schalter von Video Studio allein
reicht nicht: Die Videogenerierung muss eingeschaltet sein, mit einem Anbieter und seinem API-Schlüssel. Sonst erklärt der
Reiter, was fehlt. Der Anbieter bestimmt Dauern, Auflösungen und Startbilder. Die Erzeugung nutzt das Budget der Kategorie
`video_generation` und stoppt, wenn es aufgebraucht ist oder das Projekt seine Speichergrenze erreicht hat.

## Gut zu wissen: Grenzwerte

| Punkt | Grenze |
|---|---|
| Projektlänge | 60 Minuten |
| Spuren | 8 Video, 8 Audio, 8 Overlay |
| Clips und Marker | 2000 Clips, 500 Marker je Projekt |
| Mediendatei | standardmäßig 4096 MB (`max_asset_size_mb`, 1–32768) |
| Projektspeicher | standardmäßig 16384 MB (`max_project_size_mb`, 1–65536), inklusive Vorschauen und Versionen |
| Exportdatei | standardmäßig 8192 MB (`max_export_size_mb`, mindestens 512 MB) |
| Zeitlimit für das Rendern | standardmäßig 7200 Sekunden (`render_timeout_seconds`, 30–14400) |
| Quellbild | 16384 Pixel pro Seite, 64 Megapixel; die längere Seite höchstens 16-mal so lang wie die kürzere |
| Bildraten | 24, 25, 30, 50, 60 fps; NTSC 23,976, 29,97, 59,94 fps |
| Geschwindigkeit | 0,25× bis 16× |
| Effekte, Keyframes, Rampenpunkte | 8 Effekte je Clip; 64 Keyframes je Eigenschaft, 512 je Clip; 16 Rampenpunkte |
| Rückwärts und Bereichsprüfung | bis 5 Minuten Quelle bzw. bis 60 Sekunden |
| LUT-Dateien | 3D-`.cube`, 2–65 Punkte je Achse, 16 MB je Datei, 32 je Projekt |

- Projekte werden im Format Version 3 gespeichert. Ältere AuraGo-Versionen können sie nicht öffnen. Aktualisiere deshalb
  alle Installationen, die dieselben Projekte nutzen.
- Nicht verfügbar: animierte Sticker, automatische Untertitel und ein KI-Schnittassistent.

## Tastenkürzel

| Taste | Wirkung |
|---|---|
| `V`, `T`, `B` | Auswahl-, Trimm- und Klingenwerkzeug |
| `J`, `K`, `L` | Zurück, Stopp, Vor |
| `I`, `O` | Ein- bzw. Ausgangspunkt an der Abspielposition setzen |
| `Umschalt+I`, `Umschalt+O`, `Alt+X` | Zum Ein- bzw. Ausgangspunkt springen; beide Punkte löschen |
| `M`, `Umschalt+M`, `Strg+Umschalt+M` | Marker setzen; nächster Marker; vorheriger Marker |
| `Strg+G` | Zu einem Timecode springen |
| `N`, `S` | Einrasten umschalten; markierte Clips an der Abspielposition teilen |
| `Entf`, `Umschalt+Entf` | Löschen; mit Umschalt auch die Lücke schließen |
| `Strg+C`, `Strg+X`, `Strg+V` | Kopieren, ausschneiden, einfügen an der Abspielposition (`Strg+Umschalt+V` fügt ein) |
| `Strg+Alt+V` | Bild- oder Tonattribute auf die markierten Clips einfügen |
| `Strg+D`, `Strg+A`, `Strg+L` | Duplizieren; alles auswählen; Clips verknüpfen oder lösen |
| `Alt+←`, `Alt+→` | Auswahl um ein Bild verschieben (mit Umschalt: zehn Bilder) |
| `Umschalt+E`, `Umschalt+Z` | Markierte Clips aus- oder einschalten; Zeitleiste einpassen |
| `↑`, `↓` | Vorheriger oder nächster Schnittpunkt |
| `,`, `.` | Quellelement einfügen bzw. überschreiben |

## Datenschutz und Sicherheit

- **Lokale Verarbeitung.** Importe werden kopiert. Schnitt, Proxys, Bereichsprüfungen und Exporte laufen auf deinem
  AuraGo-Server mit FFmpeg. Der Browser zeichnet nur die Vorschau.
- **Speicherort.** Projektmedien, Poster, Exporte und Versionen liegen in den Desktop-Dateien unter
  `Documents/Video Studio/<project-id>/`, standardmäßig im Verzeichnis `agent_workspace/virtual_desktop/`. Auftragsstatus
  und Vorschaudateien liegen unter `data/video_studio/`.
- **Was den Server verlässt.** Nur eine KI-Clip-Anfrage, die du absendest: dein Prompt und das gewählte Startbild.
  API-Schlüssel landen nie in Projektdateien.
- **Audit und Agent.** Video Studio schreibt keine eigenen Audit-Einträge, und der Agent hat keinen Zugriff darauf. Die
  Schreibschutz-Schalter blockieren jede Änderung; eine laufende Aufgabe lässt sich weiterhin stoppen.

## Wenn etwas nicht klappt

| Meldung | Was hilft |
|---|---|
| „Video Studio ist in den Einstellungen deaktiviert.“ | Ein Admin schaltet **Video Studio aktivieren** und **Virtuellen Desktop aktivieren** ein. |
| „FFmpeg ist auf diesem Server nicht verfügbar.“ | FFmpeg und FFprobe installieren oder den Pfad unter **FFmpeg-Programmpfad** setzen, dann **Video Studio prüfen** klicken. |
| „Aktiviere Video Studio und installiere FFmpeg/FFprobe auf dem Server. Details findest Du in der App.“ | Die Prüfung ist fehlgeschlagen. Behebe die beiden Punkte oben, speichere und prüfe erneut. |
| „Im Nur-Lese-Modus kannst du nicht bearbeiten.“ | Projekte oder Desktop sind schreibgeschützt. Ein Admin muss das ausschalten. |
| „Speichern fehlgeschlagen“ | Verbindung prüfen und die Änderung erneut speichern. |
| „Nicht gespeichert: anderswo geändert“ | Ein anderes Fenster hat das Projekt geändert; das automatische Speichern ist pausiert. Klicke darauf und wähle die Fassung, die bleibt. |
| „Das Projekt wurde erneut geändert. Prüfe den Konflikt und versuche es noch einmal.“ | Fassung wählen und erneut versuchen. |
| „Das Projekt konnte nicht geladen werden. Prüfe die Verbindung und versuche es erneut.“ | Verbindung prüfen und das Fenster neu laden. |
| „Dieses Medium konnte nicht importiert werden.“ | Prüfen, ob ein anderer Player die Datei öffnet. Video Studio liest sie mit FFprobe. |
| „Diese Datei überschreitet das Serverlimit.“ | Eine kleinere Datei nutzen oder einen Admin bitten, die Grenze für die Mediengröße zu erhöhen. |
| „Dieses Projekt hat sein Speicherlimit erreicht. Entferne Medien oder verwende ein anderes Projekt.“ | **Ungenutzte Medien entfernen** oder alte Versionen löschen. |
| „Dieses Video ist zu breit oder zu hoch. Die längere Seite darf höchstens 16-mal so lang sein wie die kürzere.“ | Das Video außerhalb des Editors zuschneiden oder umwandeln und neu importieren. |
| „Das Projekt hat seine Grenze von 60 Minuten erreicht.“ / „Dieses Projekt hat die Clip-Grenze erreicht.“ | Die Zeitleiste kürzen bzw. ungenutzte Clips entfernen. |
| „Bereich exakt rendern funktioniert mit bis zu 60 Sekunden. Rücke Einpunkt und Auspunkt näher zusammen.“ | Den Bereich verkürzen. |
| „Das Rendern ist zwischen … und … fehlgeschlagen. Prüfe die Clips dort und versuche es erneut.“ | Die Clips in diesem Abschnitt prüfen und erneut exportieren. |
| „Das Video ist größer als die in den Video-Studio-Einstellungen festgelegte Exportgrenze.“ | Nur einen Bereich exportieren oder einen Admin bitten, die Exportgrenze zu erhöhen. |
| „Auf dem Server ist nicht genug Speicherplatz für diesen Export frei.“ | Einen Admin bitten, Speicherplatz freizugeben. |
| „Diese Aufgabe konnte nicht abgeschlossen werden.“ | **Aufgaben** öffnen, den Auftrag prüfen und erneut versuchen. |
| „Dem FFmpeg des Servers fehlt ein Filter, den dieses Projekt braucht. …“ | Den Effekt oder Übergang entfernen oder einen Admin bitten, FFmpeg zu aktualisieren. |
| „Die KI-Videoerzeugung ist nicht eingerichtet. …“ / „Das Budget für die Videoerzeugung ist vorerst aufgebraucht.“ | Ein Admin richtet die **Videogenerierung** ein. Beim Budget warten oder einen Admin bitten, es zu prüfen. |
| „Der Anbieterstatus ist unklar; … Prüfe den Status, bevor Du eine weitere kostenpflichtige Anfrage startest.“ | Den Status beim Anbieter prüfen, bevor du eine weitere kostenpflichtige Erzeugung startest. |
| „Diese .cube-Datei kann nicht verwendet werden: Zeile … ist ungültig.“ | Eine 3D-`.cube`-Datei mit 2 bis 65 Punkten je Achse verwenden. |
| „Vorschau-Proxy fehlgeschlagen; das Original wird abgespielt“ | Das Original läuft. Über **Vorschau-Proxy erstellen** im Menü des Eintrags erneut versuchen. |

Wenn die Vorschau langsam ist oder Grafikfehler zeigt, kannst du in diesem Browser nur die einfache 2D-Vorschau erzwingen.
Führe in der Entwicklerkonsole `localStorage.setItem('aurago.videoStudio.previewEngine', '2d')` aus und öffne Video Studio
neu. `localStorage.removeItem('aurago.videoStudio.previewEngine')` schaltet es wieder ab.

---

**Siehe auch:** [Kapitel 4: Web-Oberfläche](./04-webui.md) · [Kapitel 8: Integrationen](./08-integrations.md) · [Kapitel 14: Sicherheit](./14-security.md) · [Kapitel 16: Troubleshooting](./16-troubleshooting.md)

---

**Vorheriges Kapitel:** [Kapitel 25: CYD Creator](./25-cyd-creator.md)
**Nächstes Kapitel:** [Kapitel 27: Bookshelf](./27-bookshelf.md)
