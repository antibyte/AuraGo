# Kapitel 25: CYD Creator – Apps für das Cheap Yellow Display

CYD Creator baut Apps für das „Cheap Yellow Display“ und seine Verwandten: ESP32- und ESP32-S3-Boards mit
eingebautem Touch-Display. Du beschreibst die App dem Agenten oder gestaltest die Bildschirme selbst im Designer und
lässt den Agenten die Logik dahinter schreiben. AuraGo kompiliert in einem Docker-Container, und du flashst das
Ergebnis direkt aus dem Browser auf das Board. Spätere Versionen kommen auf Wunsch per WLAN aufs Gerät.

Technische Details für Admins (Konfiguration, Docker, OTA-Protokoll, Fehlercodes) stehen in der
[Betriebsreferenz](../../cyd-creator.md) (englisch).

## Was du brauchst

- **Ein unterstütztes Board.** Beim Anlegen eines Projekts zeigt die Board-Liste, was geprüft ist:

  | Board | Status |
  |---|---|
  | CYD 2.8" (ESP32-2432S028R, micro-USB), der klassische CYD | *Auf Hardware geprüft* |
  | CYD2USB 2.8" (USB-C, ST7789), CYD 2.4" resistiv und kapazitiv, CYD 3.5" resistiv und kapazitiv | *Community, ungetestet* |
  | Guition 4.3" (JC4827W543C, ESP32-S3), Sunton 4.3" und 7" (ESP32-8048S043C/S070C, ESP32-S3, 800×480) | *Community, ungetestet* |

  Community-Boards kompilieren, wurden aber noch von niemandem auf echter Hardware getestet. Das Guition-Board kann
  sein Display nicht drehen und bietet deshalb nur Querformat.
- **Einmalige Einrichtung durch einen Admin:**
  - Docker ist in AuraGo konfiguriert und nicht schreibgeschützt.
  - **Config → CYD Creator → CYD Creator aktivieren** ist eingeschaltet. Das Einschalten wirkt erst nach einem
    Neustart von AuraGo; danach erscheint CYD Creator im Startmenü.
  - Beim ersten Öffnen fehlen noch die Build-Werkzeuge. **Build-Werkzeuge installieren** lädt das
    Toolchain-Image einmalig herunter (die App nennt „etwa 3 GB“; auf der Platte belegt es etwa 2,3 GB). Solange
    das Image noch nicht veröffentlicht ist, muss der Admin es selbst bauen, siehe
    [Betriebsreferenz](../../cyd-creator.md#builder-image).
- **Zum Flashen und für den seriellen Monitor:**
  - Chrome oder Edge auf dem Rechner, an dem das Board per USB hängt. Firefox und Safari können kein Web Serial.
  - AuraGo über HTTPS oder `localhost`, CYD Creator direkt im Desktop geöffnet (nicht in einer eingebetteten
    Seite).
  - Der Admin-Schalter **Config → Virtueller Desktop → Serielle Anschlüsse an diesem Computer**
    (`virtual_desktop.serial_browser_enabled`) ist eingeschaltet. Er ist ab Werk aus und gilt nur für
    Administratoren. Auf einem schreibgeschützten Desktop gehen Flashen und Monitor nicht.
  - Fehlt etwas davon, erklärt der Flash-Dialog, was fehlt, und bietet **Image herunterladen** an: Schreibe
    `merged.bin` an Adresse 0x0, zum Beispiel mit esptool oder den ESP Web Tools.
- Unter Windows braucht der klassische CYD eventuell den CH340-Treiber.

## Die erste App

1. Öffne **CYD Creator** auf dem Desktop (Startmenü → Entwicklung).
2. Wähle **Mit dem Agenten beschreiben** oder **Im Designer starten**. Im Dialog wählst du Name, Board, Theme,
   Ausrichtung und Gerätesprache (die Sprache der Texte, die das Gerät selbst zeigt, etwa bei der
   WLAN-Einrichtung).
3. **Mit dem Agenten:** Beschreibe die App, zum Beispiel „Wetteranzeige für Berlin mit Temperatur, Luftfeuchte und
   einer Seite mit der Vorhersage“, und klicke **Erstellen und planen**. Der Agent zeigt zuerst einen Plan
   (Bildschirme, Widgets, Datenquellen, Bibliotheken, WLAN). Gebaut wird erst, wenn du **Bauen lassen** klickst;
   mit **Plan ändern** beschreibst du, was anders werden soll. Danach legt der Agent Bildschirme und Widgets an,
   schreibt den Code und kompiliert. Fehler beim Kompilieren repariert er selbst (ab Werk bis zu drei Runden).
4. **Im Designer:** Ziehe Widgets aus der Liste **Widgets** auf den Bildschirm, verschiebe sie und ändere ihre
   Größe an den Griffen. Rechts unter **Eigenschaften** stellst du Text, Werte, Farben und Ereignisse ein. Für ein
   Ereignis wählst du eine eingebaute Aktion (**Bildschirm öffnen**, **Zurück**, **WLAN-Einrichtung öffnen**) oder
   **Eigene Logik (Agent)** und schreibst in Klartext, was passieren soll. **Code aus dem Layout erzeugen** im
   Agent-Bereich lässt den Agenten diese Logik schreiben. Ist nichts ausgewählt, zeigt **Eigenschaften** die
   Projekteinstellungen: Theme, Ausrichtung, WLAN, AuraGo-Verbindung, Bibliotheken, Hintergrundbeleuchtung,
   Statusleiste und Zeitzone.
5. **Bauen** kompiliert die App. Den Fortschritt siehst du unten im **Build-Log**, Fehler und Warnungen unter
   **Probleme**. Ein Klick auf ein Problem zeigt das Widget im Designer oder öffnet die Datei an der Zeile.
6. Stecke das Board an und klicke **Flashen**, dann **Verbinden und flashen**. Wähle den Port im Browser-Dialog.
   Reagiert das Board nicht, halte beim Verbinden die BOOT-Taste gedrückt. Danach öffnet sich der serielle Monitor
   (abschaltbar im Dialog).

## Flashen: Einstellungen behalten oder vollständig installieren

Der Flash-Dialog fragt nach der **Installationsart**:

- **Einstellungen behalten** schreibt nur die App. Touch-Kalibrierung und WLAN-Daten bleiben auf dem Board. Das
  geht nur, wenn das Board schon einmal mit CYD Creator geflasht wurde.
- **Vollständige Installation (löscht Kalibrierung und WLAN)** schreibt das komplette Image ab Adresse 0x0. Danach
  richtest du Kalibrierung und WLAN neu ein.
- **Automatisch** (Voreinstellung) behält die Einstellungen nur, wenn dieser Browser dieses Board schon für dieses
  Projekt geflasht hat (gleiche MAC-Adresse, gleicher Chip, gleiches Board-Profil, unveränderte Partitionstabelle).
  Sonst installiert es vollständig. Ein anderer Browser oder ein anderes Projekt zählt als neues Board.

Der Dialog flasht mit 921600 Baud und fällt bei Problemen auf 460800 und 115200 zurück. Er prüft das Image vor dem
Schreiben (SHA-256) und danach (MD5 vom Board).

## Der serielle Monitor

Der Tab **Monitor** unten zeigt die serielle Ausgabe des Boards (Baudrate ab Werk 115200). Du kannst
**Pausieren**, **Leeren**, **Zeilen filtern** und **Zeitstempel** einblenden. **Board neu starten** setzt das Board
über die USB-Leitungen zurück. Bei einem Absturz erscheint neben der `Backtrace:`-Zeile **Dekodieren**: AuraGo
übersetzt die Adressen mit dem Build, den dieser Browser zuletzt geflasht hat, in Dateien und Zeilen. **Log
anhängen** gibt bis zu 16 KiB der markierten oder sichtbaren Zeilen an den Agenten weiter. Während eines Flash-Vorgangs pausiert der Monitor.

## WLAN auf dem Gerät

Apps mit WLAN zeigen beim ersten Start die Einrichtung:

- **Mit dem Handy:** Scanne den QR-Code auf dem Display. Das Handy verbindet sich mit dem Netz `CYD-Setup-…`
  (WPA2, das Passwort steckt im QR-Code), und die Einrichtungsseite öffnet sich. Wähle dein Netz und gib das
  Passwort ein.
- **Am Gerät:** Tippe auf **Am Gerät einrichten**, wähle das Netz aus der Liste und tippe das Passwort auf der
  Bildschirmtastatur. Versteckte Netze trägst du unter **Verstecktes Netzwerk** von Hand ein.
- Drückst du direkt nach dem Einschalten **BOOT** und hältst die Taste fünf Sekunden, vergisst das Gerät die
  WLAN-Daten. (Nicht schon beim Einschalten drücken, sonst startet der Chip im Download-Modus.)
- Boards mit resistivem Touch (wie der klassische CYD) fragen beim ersten Start nach einer Touch-Kalibrierung.
  Halte das Display während des Startbildschirms drei Sekunden gedrückt, um sie zu wiederholen.

## Updates per WLAN (OTA)

Schalte in den Projekteinstellungen **AuraGo-Verbindung (Lebenszeichen und OTA-Updates)** ein (braucht WLAN) und
baue neu. Beim Flashen meldet CYD Creator das Board dann bei AuraGo an und schreibt einen Geräteschlüssel in eine
eigene Partition des Boards. Den Schlüssel sieht niemand: AuraGo speichert nur seinen Hash, und ein Export enthält
ihn nie. Danach meldet sich das Gerät regelmäßig bei AuraGo (ab Werk jede Minute).

**Geräte** in der Werkzeugleiste listet die Boards des Projekts mit Status (*Online*, *Offline*, *Wartet auf
ersten Kontakt*), laufendem Build, letztem Kontakt, IP-Adresse und WLAN-Signal:

- **Aktualisieren auf** wählt einen erfolgreichen Build. Das Gerät holt ihn beim nächsten Kontakt, zeigt einen
  Fortschrittsbalken und startet neu.
- Stürzt ein per WLAN installierter Build dreimal hintereinander ab, bevor er sich stabil gemeldet hat, kehrt das
  Gerät von selbst zum vorigen Build zurück. Wählst du danach denselben Build erneut, passiert nichts, bis das Board
  neu startet: Baue für einen neuen Versuch neu oder schalte das Board aus und wieder ein.
- **Umbenennen** ändert den Gerätenamen. **Entfernen** sperrt den Schlüssel sofort; das Board meldet sich erst
  wieder, wenn du es neu flashst.
- Ein Projekt hat höchstens 64 Geräte. Ein Board, das sich nach dem Flashen eine Stunde lang nicht meldet,
  verschwindet von selbst aus der Liste. Flashst du ein Board aus demselben Browser erneut, ersetzt der neue Eintrag
  den alten.

Damit das klappt, muss das Board AuraGo erreichen. Die Adresse leitet AuraGo aus den Server-Einstellungen ab; in
Docker, hinter einem Reverse-Proxy oder NAT trägt ein Admin unter **Config → CYD Creator → AuraGo-Adresse für
Geräte** (`cyd_creator.ota.public_url`) die richtige ein. Der Hostname darf nur Buchstaben, Ziffern, Punkte und
Bindestriche enthalten, keine Unterstriche; IPv6-Adressen gehen nicht. Im Zweifel nimm die IPv4-Adresse, zum
Beispiel `http://192.168.1.20:8088`.

Bei HTTPS merkt sich das Board beim Flashen den Fingerabdruck des AuraGo-Zertifikats. Ändert sich das Zertifikat
(ein Let's-Encrypt-Zertifikat wird etwa alle 60 Tage erneuert, ein selbstsigniertes neu erzeugt), zeigt das Board
„AuraGo-Zertifikat geändert – bitte per USB neu flashen“. Dann hilft nur ein neuer Flash über USB. Mit `http://`
gibt es keinen Fingerabdruck, dafür geht der Schlüssel unverschlüsselt übers Netz; die Update-Images prüft das Gerät
trotzdem.

## Export und Import

**Exportieren** speichert das Projekt als ZIP: ein fertiges PlatformIO-Projekt mit `cyd.project.json`, deinem Code
unter `src/app`, den Bildern, dem erzeugten Code und einer README. Du kannst es zum Beispiel in VS Code mit
PlatformIO weiterbauen. Die Schriften im erzeugten Code stammen aus dem letzten erfolgreichen Build; baue also
einmal in CYD Creator, bevor du exportierst. WLAN-Daten und Geräteschlüssel sind nie im Export. Dein Code wird aber
so exportiert, wie er ist: **Prüfe `src/app` auf Schlüssel oder Passwörter, bevor du das Archiv weitergibst.**
Ein Export darf entpackt höchstens 128 MiB groß werden; es laufen höchstens zwei Exporte gleichzeitig.

**ZIP importieren** auf der Startseite öffnet einen CYD-Creator-Export als neues Projekt; ein bestehendes Projekt
wird nie überschrieben. Das Archiv darf höchstens 20 MiB groß sein und höchstens 500 Einträge haben; auch die
übernommenen Dateien dürfen entpackt nicht mehr als 20 MiB belegen. Erzeugte Dateien, Schriften und die README im
Archiv werden ignoriert und neu erzeugt. CYD Creator lehnt ein Archiv ab, wenn

- es kein ZIP ist oder `cyd.project.json` fehlt oder beschädigt ist,
- es zu groß ist oder zu viele Einträge hat,
- ein Pfad unsicher ist oder doppelt vorkommt,
- es eine Datei enthält, die nicht zu einem Projekt gehört,
- das Board unbekannt ist,
- eine Datei unter `src/app` zu groß, kein UTF-8-Text oder nur in der Groß- und Kleinschreibung anders als eine
  andere ist,
- ein Bild beschädigt, zu groß oder unzulässig benannt ist oder die Bilder zusammen mehr als 16 MiB belegen,
- es eine neuere Vorlagenversion braucht, als diese AuraGo-Version kennt.

## Gut zu wissen

- **Automatisch gespeichert.** Designer und Code-Editor speichern eine Sekunde nach der letzten Änderung. Wurde das
  Projekt woanders geändert, fragt CYD Creator, ob du weiterbearbeiten, den gespeicherten Stand laden oder
  überschreiben willst.
- **Während der Agent arbeitet,** sind Layout und Code gesperrt. Du kannst zusehen und mit **Job abbrechen**
  aufhören. Das Projekt bleibt dann, wie es vor dem Job war; was der Agent bis dahin gemacht hat, bleibt als
  Entwurf, den du **Fortsetzen** oder **Entwurf verwerfen** kannst. Es läuft immer nur ein Agenten-Job zur selben
  Zeit, auch über Projekte hinweg.
- **Versionen.** Vor und nach jedem Agenten-Job, beim Import und bei jeder Wiederherstellung entsteht eine Version
  (deine eigenen Änderungen im Designer und im Code nicht). Unter **Versionen** siehst du die Unterschiede und
  stellst frühere Stände wieder her; vorher sichert CYD Creator den aktuellen Stand als eigene Version. Pro Projekt
  bleiben die letzten 50 Versionen.
- **Code.** In der Ansicht **Code** bearbeitest du alles unter `src/app/` (höchstens 64 Dateien, je 256 KiB,
  zusammen 2 MiB). Was aus dem Layout erzeugt wird (`src/generated/` und die Vorlage), ist schreibgeschützt; das
  änderst du im Designer.
- **Builds.** Es läuft immer ein Build zur selben Zeit; der erste Build eines Projekts für ein Board dauert länger.
  Ein Fehler mit dem Hinweis *erzeugt* steckt im erzeugten Code: Das ist ein Problem von CYD Creator, nicht deines
  Codes. CYD Creator hebt ab Werk die letzten fünf Builds pro Projekt auf, dazu immer den neuesten erfolgreichen und
  jeden, den ein Gerät nutzt.
- **Themes.** Klar, Nacht, Terminal, Sandstorm, Dark Sun, Synthwave und Hoher Kontrast. Jedes Widget lässt sich in
  den Eigenschaften zusätzlich einzeln einfärben.
- **Sprachen.** Die Oberfläche von CYD Creator gibt es in den 16 Sprachen von AuraGo. Das Gerät selbst spricht
  Deutsch, Englisch, Dänisch, Französisch, Italienisch, Niederländisch, Norwegisch, Polnisch, Portugiesisch,
  Schwedisch, Spanisch und Tschechisch.
- **Bilder.** PNG und JPEG (SVG wird im Browser in PNG umgewandelt), höchstens 2 MiB pro Datei und 16 MiB pro
  Projekt. Auf dem Gerät zählt die entpackte Größe: Auf Boards mit 4 MB Flash dürfen die Bilder zusammen höchstens
  256 KiB belegen, auf Boards mit 16 MB Flash 4 MiB.
- **Sicherheit im App-Code.** HTTPS-Anfragen aus deiner App sind verschlüsselt, prüfen aber das Zertifikat nicht.
  Schicke darüber keine Passwörter oder Schlüssel.

## Tastenkürzel im Designer

| Taste | Wirkung |
|---|---|
| Pfeiltasten / Umschalt+Pfeil | Auswahl um 1 / 8 Pixel verschieben |
| Entf oder Rücktaste | Auswahl löschen |
| Strg+C / Strg+X / Strg+V / Strg+D | Kopieren / Ausschneiden / Einfügen / Duplizieren |
| Strg+Z / Strg+Y oder Strg+Umschalt+Z | Rückgängig / Wiederholen |
| Strg+A | Alle Widgets auf dem Bildschirm auswählen |
| Esc | Auswahl aufheben (bricht auch ein laufendes Ziehen ab) |
| + / − / 0 | Vergrößern / Verkleinern / Einpassen |
| Enter oder Leertaste (in der Widget-Liste) | Widget in der Mitte einfügen |
| F2 / Alt+Pfeil hoch oder runter (in der Bildschirmliste) | Bildschirm umbenennen / verschieben |

## Wenn etwas nicht klappt

| Meldung | Was hilft |
|---|---|
| CYD Creator fehlt im Startmenü | Ein Admin schaltet CYD Creator ein, konfiguriert Docker (nicht schreibgeschützt) und startet AuraGo neu. |
| „Die Build-Werkzeuge … sind noch nicht installiert.“ | **Build-Werkzeuge installieren** klicken. |
| „Die Installation der Build-Werkzeuge ist fehlgeschlagen.“ | Die Registry ist nicht erreichbar, die Platte voll oder das Image noch nicht veröffentlicht; siehe [Betriebsreferenz](../../cyd-creator.md#builder-image). |
| „Docker ist nicht erreichbar, deshalb kann nichts gebaut werden.“ | Ein Admin prüft den Docker-Host bzw. den Socket-Proxy. |
| „Der Build hat zu lange gedauert und wurde gestoppt.“ | Erneut bauen (der Cache ist dann wärmer) oder Zeitlimit bzw. CPU-Kerne in der Konfiguration erhöhen. |
| „Die App passt nicht in den Flash-Speicher des Boards.“ | Bilder oder Schriftgrößen entfernen oder ein Board mit mehr Flash wählen. |
| Fehler mit dem Hinweis *erzeugt* | Ein Fehler in CYD Creator, nicht in deinem Code; exportiere das Projekt und melde es. |
| „Serielle Anschlüsse sind für den Desktop ausgeschaltet …“ | Ein Admin schaltet **Config → Virtueller Desktop → Serielle Anschlüsse an diesem Computer** ein. |
| „Dieser Browser kann kein Web Serial …“ / „… nur über HTTPS …“ | Chrome oder Edge nutzen und AuraGo über HTTPS oder `localhost` öffnen; sonst **Image herunterladen**. |
| „Der serielle Anschluss wird von einem anderen Programm oder Tab benutzt.“ | Das andere Programm schließen, zum Beispiel ein Quick-Connect-Terminal, einen zweiten Tab oder einen anderen seriellen Monitor. |
| „Das angeschlossene Board hat einen anderen Chip …“ | Das Board passt nicht zum Board-Profil des Projekts. |
| „Die Verbindung zum Board wurde getrennt.“ | Erneut flashen und bei Bedarf BOOT gedrückt halten; das Board nimmt dabei keinen Schaden. |
| „AuraGo hat keine Adresse, unter der das Board es erreichen kann.“ | Ein Admin trägt die **AuraGo-Adresse für Geräte** ein (Regeln siehe oben). |
| „AuraGo konnte das HTTPS-Zertifikat … nicht lesen …“ | Ein Admin prüft, ob die eingetragene https-Adresse erreichbar ist, oder trägt eine andere ein. |
| Gerät taucht nie unter **Geräte** auf | Prüfen, ob das Board die AuraGo-Adresse aus seinem WLAN erreicht; der serielle Monitor zeigt die Versuche. |
| „AuraGo-Zertifikat geändert – bitte per USB neu flashen“ | Das Board einmal per USB neu flashen. |
| „Dieses Projekt muss nach einer unterbrochenen Veröffentlichung geprüft werden …“ | Ein Admin löst das nach der [Betriebsreferenz](../../cyd-creator.md#troubleshooting) auf. |
