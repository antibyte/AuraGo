# MeshCore-Companion einrichten

AuraGo verbindet sich mit einem Companion-Funkgerät. USB ist für Linux, Windows
und macOS implementiert, Bluetooth für natives Linux mit BlueZ. **Die praktische
Hardwareabnahme steht auf allen Plattformen noch aus.** Firmware-Flashing,
Repeater-Verwaltung und Änderungen der Funkparameter sind nicht enthalten.

## Einrichtung

1. Auf dem Gerät muss die zum Transport passende USB- oder BLE-Companion-Firmware
   installiert sein.
2. **Einstellungen → MeshCore** öffnen, Transport und seriellen Port beziehungsweise
   Bluetooth-Adresse wählen, aktivieren und speichern. USB verwendet 115200 Baud.
   Das Port-Dropdown zeigt die gefundenen seriellen Anschlüsse; **Aktualisieren**
   liest sie erneut ein. Ein zuvor gewählter, fehlender Port bleibt markiert erhalten.
3. BLE-Geräte vorher ausdrücklich koppeln. Dazu lässt sich die BLE-Adresse bei
   noch deaktivierter MeshCore-Integration speichern. Die Bluetooth-Einstellungen
   müssen Suche und Kopplung erlauben; eine Audiofreigabe ist nicht erforderlich.
   Eine eingegebene PIN gilt nur für den aktuellen Versuch und wird nicht
   gespeichert. Es findet keine automatische Kopplung statt.
4. Angezeigten vollständigen Geräteschlüssel mit dem Gerät vergleichen,
   **Diese Geräteidentität bestätigen** wählen und speichern. Der Verbindungstest
   liest ausschließlich gespeicherte Einstellungen und Gerätemetadaten; er sendet
   keine Funknachricht.
5. Vollständige öffentliche Schlüssel sicherer Nodes mit 64 Zeichen eintragen,
   einen pro Zeile. Nur eindeutige synchronisierte Chat-Kontakte dürfen direkte
   Plain-Text-Befehle auslösen. Jeder vollständige Node-Schlüssel besitzt eine
   eigene Sitzung.
   Alternativ die Nodeliste nach Name oder Schlüssel durchsuchen, unter
   **Übernehmen in** die gewünschte Freigabeliste wählen und einen Chat-Node
   anklicken. Der vollständige Schlüssel wird ohne Duplikate übernommen;
   anschließend **Speichern** wählen.
6. Antworten auf sichere Direktnachrichten bei Bedarf einschalten. Bei Kanälen
   die Zuordnung bestätigen und Empfang, Präfix (`!aura` mit anschließendem
   Leerzeichen) oder automatische Frageerkennung wählen. Einstellungen speichern.
7. Proaktives Senden separat einschalten und seine Ziel-Nodes beziehungsweise
   Kanäle freigeben. Antworten benötigen diese Freigabe nicht; das Laufzeitsystem
   bindet ihre Zieladresse unveränderlich an den Eingang.

Gespeicherte Kanalzuordnungen bleiben bei unverändertem Gerät und Kanal über
Neustarts erhalten. Nach dem Update der Fingerprint-Berechnung kann bei älteren
Zuordnungen einmalig **Kanalzuordnung bestätigen** und **Speichern** nötig sein.
Alte Verläufe bleiben erhalten; Berechtigungen werden nicht automatisch auf eine
abweichende Kanalbindung übertragen.

Unter Linux richten der One-Liner-Installer, `install_service_linux.sh` und
`update.sh` den USB-Zugriff für den systemd-Dienst automatisch über vorhandene
`dialout`-/`uucp`-Gruppen ein. Die Berechtigung gilt ab dem Dienststart; eine
erneute Benutzeranmeldung ist nicht nötig. Bei `--no-restart` gilt sie erst
nach dem nächsten Neustart des Dienstes. Manuelle Starts ohne systemd benötigen
weiterhin passende Geräteberechtigungen. Unter macOS eignen sich `/dev/cu.*`-Ports, unter Windows beispielsweise
`COM3`. Docker benötigt eine ausdrückliche Gerätedurchreichung:

```yaml
services:
  aurago:
    devices:
      - /dev/ttyACM0:/dev/ttyACM0
```

Bluetooth bleibt in Docker nicht verfügbar. Audiofreigaben, ein privilegierter
Container und ein allgemeiner Host-D-Bus-Mount sind nicht erforderlich.

## Sicherheit und Eingang

Vor jeder Verarbeitung erfolgen Format-, Duplikat- und Injection-Prüfung und
eine separate LLM-Risikoprüfung. Ein aktivierter Wächter muss eine gültige
erfolgreiche Freigabe liefern, unabhängig von globalem `fail_safe: allow`.
Bei seinem Ausfall wird nicht auf das Hauptmodell ausgewichen. Ist der Wächter
deaktiviert, prüft das Hauptmodell in einem eigenen werkzeuglosen Aufruf ohne
Verlauf oder private Erinnerungen. Timeout, ungültige oder abgeschnittene
Ausgabe und Werkzeugaufrufe führen zur Quarantäne. Auch sichere Nodes durchlaufen
diese Prüfung und unterliegen weiterhin den bestehenden AuraGo-Sperren.

Kanalantworten starten in einem frischen Minimal-Kontext ohne Core Memory, RAG,
Nutzerprofil oder Planner. Erlaubt ist allein native Brave-Websuche mit höchstens
zwei Aufrufen, sofern eingerichtet. Eine bevorzugte MCP-Suche wird nicht genutzt.
Shell, Python, Dateien, Skills, allgemeine HTTP-Aufrufe, MCP, Delegation,
Missionen und Kommunikationswerkzeuge bleiben im Dispatcher gesperrt.

Anzeigenamen und Kanalabsender begründen kein Vertrauen. Gruppennachrichten sind
nicht absendersigniert. Signed-Plain-Nachrichten und Room-Weiterleitungen gelten
ebenfalls nicht als autorisierte Direktbefehle. Slash-Commands gelangen nicht
zum globalen Befehls-Handler.

Andere oder verdächtige Eingänge bleiben im geschützten Eingang. Dauerhafte
Benachrichtigungen vom Typ `meshcore_message` enthalten ausschließlich feste
Metadaten. Beim nächsten direkten Nutzerkontakt bekommt der Agent Anzahl,
validierte Herkunftspräfixe beziehungsweise Kanalnummern und Eingangsverweise.
Fremder Nachrichtentext gelangt dabei nicht in seinen privilegierten Kontext.
Administratoren können Inhalte einsehen und eine neue Prüfung anfordern. Bereits
begonnene Befehle und unklare Ergebnisse lassen sich damit nicht wiederholen.

Gerätewechsel und geänderte Kanalzuordnungen sperren die Automatik bis zur
erneuten Bestätigung. Die Kanalbindung verwendet einen lokalen schlüsselbasierten
Fingerprint. Kanalschlüssel erscheinen weder in normalen API-Antworten noch
beim Agenten oder in Logs. Ein ausdrücklicher administrativer Einladungsexport
im Messenger ist die unten beschriebene Browser-Ausnahme; die im Gerät
hinterlegte BLE-PIN bleibt ausgeschlossen. Berechtigungsänderungen
brechen laufende Arbeit vor Veröffentlichung der neuen Konfiguration ab und
unterdrücken ausstehende Antworten. Bereits abgeschlossene Systemaktionen oder
Funkübertragungen lassen sich dadurch nicht rückgängig machen.

## Betrieb und Grenzen

- Die versionierte SQLite-Datei `meshcore.db` liegt im konfigurierten
  Datenverzeichnis. Standard: sieben Tage, 1.000 gespeicherte Nachrichten,
  Warteschlange mit 128 Einträgen, zwei automatische Läufe je Node/Kanal und
  zwölf insgesamt pro Minute. Überzählige Eingänge führen keine Aktion aus.
- Direktbefehle dürfen höchstens 600 Sekunden alt sein; 120 Sekunden Abweichung
  in die Zukunft sind erlaubt. Beides ist einstellbar. Alte Befehle bleiben lesbar.
- Vor Ausführung wird atomar reserviert. Unterbrochene Arbeit und Sendungen
  erscheinen nach Neustart als `outcome_unknown`; wartende Eingänge benötigen
  eine erneute administrative Prüfung. Ausführungsmerker bleiben unabhängig
  von gelöschten Nachrichtentexten 48 Stunden erhalten. Ein voller Merkerbestand
  mit 65.536 Einträgen sperrt weitere automatische Arbeit.
- Automatische Kanalantworten tragen die feste KI-Kennzeichnung `[AuraGo KI]`.
- Antworten passen in höchstens drei nummerierte, UTF-8-sichere Pakete. Der vom
  Gerät ergänzte Kanalname des Absenders verringert den verfügbaren Platz. Zu
  lange Texte werden abgelehnt und niemals still abgeschnitten. Es werden
  höchstens sechs Anwendungspakete pro Minute gesendet.
- `device_accepted` bedeutet Geräteannahme; `delivered` setzt eine passende
  Direktzustellbestätigung voraus. Kanalversand bestätigt keinen Empfänger.
  Teilweise oder unterbrochene Sendungen können `outcome_unknown` bleiben und
  werden nicht automatisch wiederholt. Protokollbestätigungen der Firmware
  bleiben möglich; Empfangsmodus unterbindet automatische Anwendungsantworten.
- Nach Wiederverbindung werden Kontakte, Kanäle und Nachrichten abgeglichen.
  Push-Ereignisse und begrenztes Nachfragen alle 15 Sekunden holen neue Eingänge
  ab. BLE benötigt eine ausreichende ausgehandelte MTU; möglicherweise
  abgeschnittene Frames werden abgewiesen.

Die administrativen GET-Endpunkte liegen unter
`/api/meshcore/{status,devices,contacts,channels,messages}`, POST-Aktionen unter
`/api/meshcore/{scan,pair,test,recheck}`. Nachrichten unterstützen `limit`
(höchstens 100) und `offset`. Verbindungs- und Prüffehler erscheinen im bestehenden
Operational-Issue-Lebenszyklus.

Das Agentenwerkzeug `meshcore` bietet `status`, `contacts`, `channels`,
`send_direct` mit `node_key`/`text` und `send_channel` mit `channel`/`text`.
Es bietet keine Rohprotokoll-, Schlüssel-, Firmware- oder Geräteverwaltung.

## Desktop-Messenger

Die App **MeshCore** im virtuellen Desktop verwendet dasselbe Companion-Gerät
wie die Integration. Der Browser baut keine zweite USB-/Bluetooth-Verbindung
auf. Unter **Verbindung** bleiben Geräteidentität, Anschluss und Agentenrechte.

- Gesprächsliste mit Suche, Direkt-/Kanal-/Ungelesen-Filtern, Favoriten und
  Stummschaltung; im Gespräch stehen Verlaufssuche, Kopieren und ältere Seiten
  bereit. Unter 700 Pixel Fensterbreite wechselt **Zurück** zur Liste. Eine
  vorhandene Fensterinstanz wird wiederverwendet, auch über Spaces hinweg.
  Sitzungswiederherstellung und Benachrichtigungen öffnen das passende Gespräch.
- **Enter** sendet, **Shift+Enter** fügt eine Zeile ein. Entwürfe bleiben lokal
  je Geräteidentität und Gespräch gespeichert. Bytezähler und Paketvorschau
  berücksichtigen UTF-8, maximal drei nummerierte Teile und den Kanalabsender.
  Manuelles Senden benötigt weder LLM noch proaktive Agentenfreigabe.
- Versandstatus unterscheidet laufenden Versand, Geräteannahme, bestätigte
  Direktzustellung, nicht gesendet und unklaren Ausgang. Kanäle erhalten keine
  Empfängerbestätigung. HTTP-Wiederholungen verwenden dieselbe dauerhafte
  Auftrags-ID. **Erneut senden** warnt ausdrücklich vor möglicher Doppelzustellung.
- Ungeprüfte oder quarantänisierte Nachrichten erscheinen als Platzhalter.
  **Geschützten Text anzeigen** zeigt bereinigten Klartext nur im geöffneten
  Gespräch. Das erteilt keine Freigabe und führt keine Aktion aus. Nachrichten
  werden ausschließlich als Text dargestellt.
- Kontakte lassen sich per vollständigem Schlüssel/Name/Typ oder Kontaktlink
  hinzufügen und per Link/QR-Code teilen. **Mein Node** kündigt ausdrücklich
  in direkter Reichweite oder über das Mesh an; dabei kann eine bereits im Gerät
  konfigurierte Position mitgesendet werden. Repeater, Rooms und Sensoren werden
  gekennzeichnet, erhalten aber keine Verwaltungsbefehle.
- Öffentliche, Hashtag- und private Kanäle verwenden nur freie Slots. Private
  Schlüssel werden zufällig erzeugt oder als 32 Hex-Zeichen eingegeben. Ein
  erneuter Geräteabgleich bestätigt Änderungen. Neue Kanäle starten mit reinem
  Empfang; Kontaktentfernung widerruft Agentenrechte. Unklare Änderungen bleiben
  gesperrt. Eine ausdrückliche Zuordnungsbestätigung setzt Kanalautomatik zurück.
- **Teilen → Einladung anzeigen** ist ein eigener administrativer Export. Bei
  privaten Kanälen enthält die Einladung den Schlüssel. Die Antwort ist
  `no-store`, bleibt ausschließlich im aktuellen Dialog und wird weder im
  Browser gespeichert noch protokolliert oder an den Agenten weitergereicht.
  Kopieren erfolgt nur auf Klick; Schließen entfernt den Dialoginhalt. QR-Bilder
  können mit nativem `BarcodeDetector` gelesen werden, Einladungscodes lassen
  sich überall einfügen. Nicht unterstützte Regionsoptionen werden abgelehnt.

Der Verlauf hat getrennte Standardgrenzen: **90 Tage und 10.000 Nachrichten
insgesamt**, einstellbar in der App oder über `meshcore.history_days` und
`meshcore.history_messages`. Der geschützte Eingang behält seine eigenen Grenzen
von sieben Tagen/1.000 Nachrichten. Verlauf leeren entfernt sichtbaren Chattext;
Sicherheitsreservierungen und der kurzlebige Eingang bleiben erhalten.
Geräte-/Kontaktschlüssel und Kanal-Fingerprints trennen alte Gespräche nach
Gerätewechsel oder Slot-Neubelegung. Historisch unklare Schlüsselpräfixe werden
nicht nachträglich zugeordnet; fehlende Versandbelege bleiben unbekannt.

Die Migration einer vorhandenen Datenbank legt zunächst eine private
`meshcore-v1-*.backup.db` daneben an; diese Sicherungen verwaltet der
Administrator. Manuelle Auftragskennungen überleben die Verlaufslöschung.
Die Sicherheitsgrenze von 65.536 Einträgen sperrt neue Sendungen, sobald die
Ablage voll ist, und erfordert administrative Wartung.

Die administrativen Endpunkte unter `/api/meshcore/messenger/` bieten GET für
`bootstrap`, `conversations` und `messages`; POST für `send`, `conversation`,
`reveal`, `invitation`, `manage` und `settings`. Schreibzugriffe prüfen die
Herkunft. Die Verlaufspaginierung verwendet einen exklusiven `before`-Cursor
mit bis zu 50 Nachrichten pro Seite. API-Felder stehen in der
[englischen Dokumentation](meshcore-en.md#desktop-messenger).
Desktop-Ereignisse enthalten nur Metadaten und Gesprächsverweise. Nach einer
Wiederverbindung wird der aktuelle Zustand geladen. Stummschaltung beeinflusst
ausschließlich Messenger-Benachrichtigungen, nicht die Hinweise an den Agenten.

Prüfkommandos und die festgeschriebenen Firmwarequellen stehen in der
[englischen Integrationsdokumentation](meshcore-en.md#validation-and-sources).
USB muss noch praktisch unter Linux, Windows und macOS sowie BLE unter nativem
Linux abgenommen werden.
