# Kapitel 34: Storymaker – Erklärvideos im Virtual Desktop

Storymaker ist die App im Virtual Desktop für kurze Erklärvideos. Du inszenierst Szenen mit gezeichneten Figuren,
Requisiten und Umgebungen aus einem eingebauten Katalog, schreibst den Sprechtext, wählst eine Stimme und Musik,
siehst dir eine Live-Vorschau an und exportierst ein MP4 mit Ton, Untertiteln, Untertiteldateien und getrennten
Tonspuren. Ein fertiger Export kann in Video Studio weitergehen.

Storymaker ist standardmäßig an. Der KI-Autor schreibt den ganzen Film aus einem Thema, in der App oder aus dem Chat.
Bearbeiten und Vorschau brauchen nur den Browser; der Export braucht FFmpeg und einen Headless-Browser. Der
vollständige Vertrag steht im [Storymaker-Handbuch](../../storymaker.md).

## Inhaltsverzeichnis
1. [Storymaker öffnen](#storymaker-öffnen)
2. [Einstellungen](#einstellungen)
3. [Der KI-Autor](#der-ki-autor)
4. [Stimme und Sprechtext](#stimme-und-sprechtext)
5. [Exportieren](#exportieren)
6. [In Video Studio weiterschneiden](#in-video-studio-weiterschneiden)
7. [Wenn etwas schiefgeht](#wenn-etwas-schiefgeht)

---

## Storymaker öffnen

Öffne **Storymaker** aus dem Startmenü, in der Kategorie **Kreativ**. Das Fenster öffnet sich maximiert.

Die Projektliste zeigt jede Geschichte auf dem Server, darüber liegt das Formular **Neues Projekt**. Ein neues Projekt
startet mit einer leeren Szene. Das Formular listet außerdem die Sprachen, für die eine Stimme existiert; ohne eine
für die Desktop-Sprache bietet es **Ohne Stimme anlegen** an.

In einem Projekt hat der Editor drei Spalten:

- **Szenen (links).** Eine Karte je Szene mit einem Vorschaubild, das der Player zeichnet, den ersten Worten und der
  Dauer. Füge eine Szene oder eine Code-Szene hinzu; verschiebe oder lösche eine Szene im Inspektor.
- **Bühne und Zeitleiste (Mitte).** Die Vorschau läuft in einem Sandbox-Frame. Die Zeitleiste hat einen Block je Szene
  (Code-Szenen sind schraffiert); klicke oder ziehe ihn, oder benutze die Pfeiltasten (Shift bewegt eine zweite).
- **Inspektor, Bibliothek, Musik, Stimme (rechts).** Der Inspektor hält Umgebung, Tageszeit, Sprechtext, den Übergang
  und die Dauer (automatisch heißt: so lange, wie der Text zum Lesen braucht). Die Bibliothek hält Figuren,
  Requisiten und Orte aus dem Katalog, mit Suche. Der Musik-Reiter hält die Hintergrundmusik, der Stimmen-Reiter die
  Stimme und den Sprechtext.

Die Werkzeugleiste trägt die Reiter **Storyboard** und **Code** und ein Abzeichen für **Probleme**. Das Abzeichen
zählt Fehler und Warnungen; ein Fehler blockiert den Export, und die Liste sagt, wo. Der **Code**-Reiter zeigt
`story.json` und jede Code-Szene in einem Editor. Eine Code-Szene ist ein kleines Programm,
`export default function scene(api) { ... }`, das eine Szene selbst zeichnet; es erreicht weder das Netzwerk noch
die Seite. Mit `virtual_desktop.readonly` an kannst du ansehen, aber nichts bearbeiten, speichern oder exportieren.

## Einstellungen

Unter **Konfiguration → Virtual Desktop → Storymaker** liegen die Einstellungen:

| Einstellung | Standard | Bedeutung |
| --- | --- | --- |
| `storymaker.enabled` | `true` | Zeigt die App und beantwortet ihre API. Aus: Die App ist versteckt und die API antwortet `storymaker_disabled`. |
| `storymaker.render.timeout_seconds` | `3600` | Ein Export, der länger läuft, wird gestoppt (60 bis 14400). Längere Videos bekommen automatisch mehr Zeit. |
| `storymaker.render.max_duration_seconds` | `900` | Das längste Video, das exportiert werden kann (10 bis 3600). |
| `storymaker.render.browser_path` | leer | Absoluter Pfad von Chrome oder Chromium. Leer: der auf dem System installierte Browser, sonst ein gepinnter Download. |
| `storymaker.render.transport` | leer | Wie die Bilder zu FFmpeg gelangen (`png`, `webp`, `raw`, `screenshot`). Leer ist der gemessene Standard; ändere ihn nur auf Rat. |
| `storymaker.narration.max_daily_chars` | `100000` | Zeichen, die Cloud-Stimmen an einem lokalen Tag sprechen dürfen (1000 bis 100000000). |
| `storymaker.narration.max_job_chars` | `20000` | Zeichen, die Cloud-Stimmen in einem Sprechauftrag sprechen dürfen (100 bis 10000000). |
| `storymaker.agent.enabled` | `true` | Erlaubt Agenten-Aufträge aus der App und aus dem Chat. |
| `storymaker.agent.provider` | leer | LLM-Anbieter für Agenten-Aufträge. Leer: die Hauptroute mit ihrem eingerichteten Failover. Ein gewählter Anbieter bleibt für den Auftrag fest, ohne Failover. |
| `storymaker.agent.model` | leer | Modell für Agenten-Aufträge. Leer: das Modell des Anbieters. |
| `storymaker.agent.job_timeout_seconds` | `1800` | Ein Auftrag, der länger läuft, stoppt, und seine gültigen Szenen bleiben (120 bis 7200). |
| `storymaker.agent.max_length_seconds` | `180` | Eine längere Anfrage wird auf diese Länge gekürzt (15 bis 3600). `max_duration_seconds` gilt zusätzlich. |
| `storymaker.agent.max_daily_jobs` | `10` | Neue Filme und Überarbeitungen aus App und Chat, dazu Exporte aus dem Chat, pro Tag (1 bis 1000). |

Die Schaltfläche **Export-Einrichtung prüfen** fragt den laufenden Server, ob FFmpeg und ein Browser bereit sind. Sie
startet selbst keinen Download.

## Der KI-Autor

Der KI-Autor schreibt einen Film aus einem Thema: zuerst die Dramaturgie, dann die Besetzung, dann eine Szene nach der
anderen, dann eine Prüfung mit Korrekturen, und am Ende den Sprechtext. Er läuft als isolierter Auftrag und darf nur
die Storymaker-Werkzeuge seines aktuellen Schritts benutzen.

- **In der App.** Die Projektliste hat ein Startformular. In einem Projekt öffnest du den Seitenreiter **KI-Autor** im
  Editor. Er hält die Eingabe, die Länge in Sekunden, die Sprache, **Geschichte schreiben** oder
  **Geschichte überarbeiten**, eine Fortschrittsliste, **Autor stoppen** (Abbrechen), die Ergebniskarte,
  **Ohne Stimme exportieren** und **Sprache wiederherstellen**.
- **Aus dem Chat.** Das `storymaker`-Werkzeug legt einen Film an, liest seinen Status, überarbeitet ihn, exportiert
  ihn, öffnet ihn in der App und listet die Projekte. "Mach mir ein Erklärvideo über Backups" genügt als Start. Das
  Werkzeug braucht `storymaker.enabled`, `storymaker.agent.enabled` und den Virtual Desktop (siehe [Kapitel
  22](./22-internal-tools.md)).
- **Unbeaufsichtigte Läufe.** Ist `agent.action_policy` eingeschaltet, brauchen Missionen, Flows, geplante Läufe und
  Co-Agenten eine Administratorregel, um `create`, `revise` oder `render` aufzurufen. An Chats, an denen du selbst
  beteiligt bist, braucht es keine Regel. Die Regel ist standardmäßig aus.

**Während ein Auftrag läuft.** Der Editor ist gesperrt und zeigt ein Banner. Speichern, Sprechtext und Export werden
für dieses Projekt abgelehnt. Ein Auftrag kann nicht starten, während ein Render-, Sprech- oder Musikauftrag des
Projekts läuft. Ein Auftrag läuft zurzeit, acht können warten, ein neunter wird abgelehnt.

Die Anfrage kann eine Sprache nennen. Sonst nutzt der Auftrag die Sprache der Oberfläche. Sprechtext, Titel und Text
im Bild werden in dieser Sprache geschrieben und auf Striche überall sowie auf Floskeln auf Englisch und Deutsch
geprüft; ein Treffer bekommt eine optionale Korrekturrunde und blockiert das Ergebnis nie.

| Ergebnis | Was es bedeutet |
| --- | --- |
| Deine Geschichte ist fertig. Bitte sieh sie dir einmal an. | Szenen, Requisiten und Timing wurden geprüft. Überlappungen und passender Text wurden nicht automatisch geprüft. |
| Deine Geschichte ist teilweise fertig. | Manche Szenen brauchen noch Arbeit, und gültige Szenen sind gespeichert. Das Überarbeiten-Feld ist vorbefüllt, um sie fertigzustellen. Ein Timeout oder ein Modellfehler endet hier ebenfalls. |
| Der Agent konnte diese Geschichte nicht schreiben. | Es wurde keine Dramaturgie gespeichert. Die Karte nennt die Art des Fehlers in einer Zeile. |

Sieh dir jeden Film einmal an: noch prüft nichts, ob eine Requisite ein Gesicht verdeckt oder ein Text überläuft. Ein
Film kann etwas länger laufen als gewünscht; das wird aufgeführt und ist kein Fehler. Ohne Browser wird die
Kompilierprüfung übersprungen und als `compile_unverified` gemeldet; der Auftrag läuft trotzdem fertig. Die
Ergebniskarte einer Überarbeitung listet die Szenen, die sich geändert haben, die geblieben sind und die entfernt
wurden; die Chat-Antwort auf `status` nennt dieselben drei Listen.

**Keine Stimme.** Ein Auftrag auf einer Maschine ohne fertige Stimme läuft trotzdem fertig. Der Sprechtext bleibt in
der Geschichte, und jede Szene bekommt einen kurzen Text im Bild. Um zu exportieren, richte eine Stimme ein und drücke
**Sprachausgabe erzeugen**, oder drücke **Ohne Stimme exportieren**. Diese Schaltfläche markiert die Szenen als stumm,
gibt jeder eine Lesezeit und macht aus Sprechblasen Text im Bild. **Sprache wiederherstellen** macht das rückgängig,
solange seine Kopie `story.before-silent.json` existiert; aus dem Chat tut `render` mit `silent: true` dasselbe.

**Musik.** Der KI-Autor macht keine Musik. Eine Bitte um Musik bekommt die Antwort, dass er das nicht kann, und der
Musik-Reiter bleibt bei dir. Er bietet keine Musik, KI-erzeugte Musik (nur wenn die Musikgenerierung eingerichtet ist)
oder eine WAV- oder MP3-Datei oder einen Noisemaker-Titel. Erzeugte Musik startet erst, wenn du die Schaltfläche
drückst, denn eine Generierung kann Geld kosten.

Ein Agenten-Auftrag hat kein eigenes Token-Limit. Seine Kosten sind begrenzt durch das Auftrags-Timeout (standardmäßig
1800 Sekunden), die festen Werkzeugaufruf-Grenzen je Schritt und `storymaker.agent.max_daily_jobs`. Die
Auftragsprotokolle liegen in `data/storymaker/agent/`: `jobs.json`, `usage.json` und `events/<job id>.jsonl`; sie
enthalten Werkzeugnamen, Szenen-IDs, Token-Zahlen und Status, aber keine Werkzeugargumente und keinen Modelltext.

## Stimme und Sprechtext

Das Stimmen-Panel wählt Anbieter, Stimme und Sprache. **Diese Stimme verwenden** schreibt sie in die Geschichte;
**Sprachausgabe erzeugen** und **Hörprobe abspielen** arbeiten mit der gespeicherten Stimme, eine von dir gewählte
Stimme braucht also erst diesen Klick. Phonon-Stimmen, die nicht installiert sind, stehen nicht in der Liste: lade sie
unter More voices in den Phonon-Einstellungen herunter. `not tested` heißt, dass noch niemand diesen Anbieter in
dieser Sprache gehört hat. Audio wird Satz für Satz erzeugt und nur für Sätze ohne Audio, eine Änderung kostet also
einen Satz. Das Panel zeigt die Zeichen, was noch kein Audio hat, und den Fortschritt. **Hörprobe abspielen** spielt
einen gespeicherten Satz ab, ohne einen Anbieter zu rufen.

Cloud-Stimmen zählen Zeichen pro Auftrag und pro lokalem Tag (siehe die Einstellungen oben). Lokale Stimmen (phonon,
piper, sanotts, supertonic, speech_lab) werden nicht gezählt. Cloud-Stimmen können Geld kosten: Ein fertiger Auftrag
erzeugt seinen Sprechtext am Ende selbst, unter den Zeichlimits oben. Der Sprechtext läuft mit der Vorschau, und die
Zeitleiste zeigt seine Wellenform. Sprechblasen, eingebrannte Untertitel und Lippen-Sync werden in der Vorschau und im
Export gezeichnet; ein Schalter im Stimmen-Panel schaltet die Untertitel ab. Griechischer, indischer, arabischer,
chinesischer und japanischer Text nutzt mitgelieferte Ersatzschriften, die nur bei Bedarf geladen werden. Ein Zeichen,
das keine Schrift abdeckt, etwa ein Emoji oder thailändischer und hebräischer Text, ist der Fehler `glyph_missing`.

## Exportieren

Die Schaltfläche **Exportieren** öffnet ein Panel, das zuerst sagt, was fehlt. Zum Exportieren braucht es:

1. FFmpeg (im `PATH` oder unter `video_studio.ffmpeg_path`),
2. einen Browser, der headless laufen kann (Chrome oder Chromium). Ohne einen lädt der erste Export ein gepinntes
  Chromium (etwa 200 MB, vor der Nutzung gegen eine SHA-256 geprüft) in den Datenordner; das Export-Panel sagt das
  vorher. Setze im Docker-Image `storymaker.render.browser_path` auf einen Browser im Container: das Image hat
  keine eigenen Chromium-Bibliotheken,
3. eine Geschichte ohne Fehler, die nicht länger als `max_duration_seconds` ist.

Bearbeiten und Vorschau funktionieren ohne 1 und 2. Der Export zeichnet jedes Bild in einem Headless-Browser, ein
15-Minuten-Video dauert also eine Weile; das Panel zeigt die Phase und die Bildzahl und kann abbrechen. Das Ergebnis
ist `Documents/Storymaker/<project id>/exports/<title>-<job>.mp4`; **In Dateien zeigen** öffnet diesen Ordner. Videos
enthalten Sprechtext, Musik und Soundeffekte.

Ein Export öffnet einen Dialog mit diesen Wahlmöglichkeiten:

| Wahl | Bedeutung |
| --- | --- |
| Qualität | `standard` oder `high` für ein besseres Bild und eine größere Datei. |
| Untertitel ins Bild einbrennen | Zeichnet die Untertitel ins Bild. Die Voreinstellung folgt dem Untertitelschalter der Geschichte. |
| SRT, VTT | Schreibt Untertiteldateien neben das Video. |
| Getrennte Tonspuren | Schreibt eine WAV je Audioebene: Sprechtext, Musik, Soundeffekte. Eine stumme Ebene bleibt weg. |
| In Video Studio öffnen | Schaltet die getrennten Tonspuren an, damit der fertige Export in Video Studio weitergehen kann. |

Die Dateien eines Exports:

```
Documents/Storymaker/<project id>/exports/<slug>-<job8>.mp4
Documents/Storymaker/<project id>/exports/<slug>-<job8>/narration.wav  music.wav  sfx.wav  captions.srt  captions.vtt
Documents/Storymaker/<project id>/exports/<job id>.json
Videos/Storymaker/<slug>.mp4, <slug>-2.mp4, ...
```

Die Kopie in `Videos/Storymaker/` macht den Film leicht auffindbar. Eine fehlgeschlagene Zusatzdatei lässt das MP4
stehen und zeigt eine Warnung beim fertigen Export. Die Tonspuren ergeben zusammen genau den Mix, der vor dem Kodieren
daraus gemacht wird; die AAC-Spur im MP4 unterscheidet sich leicht, etwa 23 dB unter dem Mix.

## In Video Studio weiterschneiden

Ein fertiger Export mit Tonspuren bietet **In Video Studio öffnen** an, wenn Video Studio an ist, FFmpeg bereit ist
und der Desktop beschreibbar ist. Die Aktion legt ein Video-Studio-Projekt an, kopiert das MP4 und jede Tonspur hinein
und öffnet es. Der Film liegt auf einer Videospur mit abgetrenntem Ton, und jede Tonspur bekommt eine eigene Audiospur
(siehe [Kapitel 26](./26-video-studio.md)).

Das funktioniert auch, wenn du das Projekt verlassen oder das Storymaker-Fenster geschlossen hast: öffne das Projekt
wieder und wähle **Letzten Export anzeigen** im Export-Panel. Wenn du im Dialog **In Video Studio öffnen** angekreuzt
hast und das Projekt nur während des Renders verlassen hast, startet die Übergabe von selbst, wenn du das Projekt im
selben Fenster wieder öffnest. Schlägt eine Übergabe fehl, füllt ein neuer Versuch dasselbe Video-Studio-Projekt; hält
dieses Projekt dann Clips, wird ein neues angelegt, damit deine Arbeit erhalten bleibt. Ungespeicherte Arbeit in einem
offenen Video-Studio-Projekt wird zuerst gesichert.

## Wenn etwas schiefgeht

| Problem | Was hilft |
|---------|-----------|
| Die App steht nicht im Startmenü | `storymaker.enabled` ist aus. Eine Administratorin oder ein Administrator kann sie unter **Config → Virtual Desktop → Storymaker** einschalten. |
| „Export ist noch nicht bereit (FFmpeg oder Browser fehlt)“ | Installiere FFmpeg auf dem Server oder setze seinen Pfad in den Video-Studio-Einstellungen (`video_studio.ffmpeg_path`). Fehlt der Browser, lädt der erste Export ein gepinntes Chromium; `storymaker.render.browser_path` zeigt auf ein installiertes Chrome oder Chromium, wenn der Download nicht klappt, etwa im Docker-Image. |
| Der Export wird abgelehnt | Das Panel nennt, was fehlt: Fehler in der Geschichte, ein Video länger als `max_duration_seconds`, fehlendes Sprech-Audio oder eine fehlende Musikdatei. Das Panel öffnet den Reiter, der das behebt. |
| Der Export ist sehr langsam oder wurde vom Timeout gestoppt | Der Export zeichnet jedes Bild in einem Headless-Browser. Auf einer reinen CPU-Maschine ist ein aktuell installiertes Chrome schneller als das mitgelieferte Chromium. Längere Videos bekommen automatisch mehr Zeit, ein höheres `storymaker.render.timeout_seconds` ist also die Lösung. |
| Der KI-Autor startet nicht | `storymaker.agent.enabled` ist aus, es ist kein KI-Modell verfügbar, oder das Modell ist zu klein. Die Karte nennt den Grund. Ist das Tageslimit `storymaker.agent.max_daily_jobs` aufgebraucht, versuche es morgen noch einmal oder erhöhe das Limit. |
| „Es warten schon acht Agenten-Aufträge“ | Die Auftragswarteschlange ist voll. Warte, bis einer fertig ist. |
| Der Sprechtext hört nach einer Weile auf | Das Zeichenlimit für den Tag oder den Auftrag ist erreicht. Cloud-Stimmen sind begrenzt, lokale nicht. |
| Der Sprechtext wurde nicht erzeugt oder ein Zeichen fehlt | Für diese Sprache ist keine Stimme bereit, oder der Anbieter hat versagt: richte eine Stimme ein und drücke **Sprachausgabe erzeugen**, oder drücke **Ohne Stimme exportieren**. Fehlt ein Zeichen, nennt der Fehler `glyph_missing` die Stelle; es steckt in keiner mitgelieferten Schrift, also schreibe das Wort um oder wähle eine andere Sprache. |
| Die Musik oder der Ton fehlt im Export | Die Musikdatei oder der Titel ist weg, oder es wurde nichts vorbereitet; wähle im Musik-Reiter wieder eine Quelle. Ein Film ohne Sprechtext, Musik und Soundeffekte wird mit einer stummen Spur exportiert, und das Export-Panel sagt das vorher. |
| **In Video Studio öffnen** wird nicht angeboten | Der Export hat keine Tonspuren, Video Studio ist aus, FFmpeg fehlt, oder der Desktop ist schreibgeschützt. |

---

**Siehe auch:** [Kapitel 4: Web-Oberfläche](./04-webui.md) · [Kapitel 22: Interne Tools](./22-internal-tools.md) · [Kapitel 26: Video Studio](./26-video-studio.md) · [Kapitel 14: Sicherheit](./14-security.md) · [Kapitel 16: Troubleshooting](./16-troubleshooting.md)

---

**Vorheriges Kapitel:** [Kapitel 33: Aura Drift](./33-aura-drift.md)
**Nächstes Kapitel:** [Kapitel 35: Bitte nicht das Universum füttern](./35-goldfish-adventure.md)
