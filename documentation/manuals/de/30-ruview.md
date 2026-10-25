# Kapitel 30: RuView – Wi-Fi-Sensing (experimentell)

RuView ist die experimentelle Wi-Fi-Sensing-App von AuraGo. Sie kapselt das Upstream-Projekt RuView v3122 von ruvnet mit
seinen Ansichten Dashboard, Observatorium, Pose-Fusion, Kalibrierung, Aufnahme und Training. Sie liest die Signalstärke
eines verbundenen Host-WLAN-Adapters (RSSI) oder Channel State Information (CSI) von ESP32-Boards aus. Nutze sie zum
Experimentieren. Sie ist kein validierter Präsenz- oder Posensensor: Ein verbundener Dienst belegt keine Genauigkeit,
und eine Simulation ist kein Sensor.

Technische Details für Admins (Laufzeiten, Netzregeln, Speicherlayout, Artefakt-Pins) stehen in der
[Betriebsreferenz](../../ruview.md) (englisch). Die kurze Übersicht für Admins steht in
[Kapitel 8: Integrationen](08-integrations.md#ruview-wi-fi-sensing).

## Inhaltsverzeichnis

1. [Was du brauchst und wie du es einschaltest](#was-du-brauchst-und-wie-du-es-einschaltest)
2. [Erste Schritte](#erste-schritte)
3. [Die Betriebsart wählen](#die-betriebsart-wählen)
4. [Übersicht und Geräte lesen](#übersicht-und-geräte-lesen)
5. [Ein ESP32-Board einrichten](#ein-esp32-board-einrichten)
6. [Die Upstream-Ansichten](#die-upstream-ansichten)
7. [Aufnahmen und Speicher](#aufnahmen-und-speicher)
8. [Gut zu wissen](#gut-zu-wissen)
9. [Datenschutz und Sicherheit](#datenschutz-und-sicherheit)
10. [Wenn etwas nicht klappt](#wenn-etwas-nicht-klappt)

---

## Was du brauchst und wie du es einschaltest

RuView findest du unter **Config → RuView**, in der Gruppe **Netzwerk & Remote**. Der Block `ruview:` in der
`config_template.yaml` enthält diese Einstellungen:

| Einstellung (UI-Beschriftung) | Config-Schlüssel | Standard | Wirkung |
|---|---|---|---|
| **Integration aktivieren** | `ruview.enabled` | aus | Schaltet RuView ein. Nach einer Änderung speichern. |
| **Betriebsart** (*Verwaltetes Docker*, *Externe Instanz*, *Native Host-Laufzeit*) | `ruview.mode` | `docker` | Wo der RuView-Dienst läuft. Siehe [Die Betriebsart wählen](#die-betriebsart-wählen). |
| **Quelle** (*ESP32-CSI*, *Host-WLAN-RSSI*, *Ausdrückliche Demo*) | `ruview.source` | `esp32` | Woher die Daten kommen. Host-WLAN wird bei verwaltetem Docker nicht angeboten. |
| **Nur lesen** | `ruview.readonly` | an | Sperrt Starten, Ändern und Löschen. Administratoren dürfen Dienst, Aufnahmen und Training weiter stoppen. |
| **Agent-Lesezugriff erlauben** (unter **Berechtigungen**) | `ruview.agent_access` | aus | Erlaubt dem Agenten, Status und Messungen zu lesen. Siehe [Datenschutz und Sicherheit](#datenschutz-und-sicherheit). |

Änderungen wirken nach **Speichern**. RuView einzuschalten erfordert keinen Neustart von AuraGo. Beim Start startet
AuraGo eine aktivierte und schreibbare verwaltete Laufzeit im Hintergrund. Läuft AuraGo bereits, drückst du in der App
**Starten**. Eine externe Instanz startet AuraGo nie.

**Nur lesen** ist standardmäßig an. Solange es an ist, kann nichts starten oder sich ändern. Schalte es aus und speichere.
Das Formular zeigt dann: „Der Schreibschutz sperrt Starten, Ändern und Löschen. Administratoren dürfen Dienst, Aufnahme
und Training stoppen.“ **Verbindung testen** funktioniert auch im Lesemodus. Erfolg meldet der Test nur bei frischen Sensordaten. Ein verbundener Dienst, der noch keinen ersten Frame gesendet hat, zeigt **Warte auf erste Daten** als Warnung.

Unter **Verbindung** findest du einen eingeklappten Bereich mit **HTTP-Port** (`ruview.http_port`, Standard 3022, nur auf
dem Loopback) und **Speicherbudget (MiB)** (`ruview.storage_mb`, Standard 1024).

## Erste Schritte

1. Öffne **Config → RuView**, schalte **Integration aktivieren** ein, wähle **Betriebsart** und **Quelle** und klicke
   auf **Speichern**.
2. Für eine verwaltete Laufzeit schalte **Nur lesen** aus und speichere erneut.
3. Klicke auf **RuView öffnen**. Die App öffnet sich in einem neuen Tab. Im Startmenü steht sie außerdem unter
   **Kommunikation**. Das Öffnen der App startet den Dienst nicht. Solange die **Übersicht** oder der Tab **Geräte** offen ist, fragt AuraGo bei der Quelle ESP32-CSI alle drei Sekunden die Board-Liste beim Dienst ab. Ein verwalteter Dienst, der nicht läuft, wird dabei nicht angesprochen.
4. Klicke in der **Übersicht** der App bei einer verwalteten Laufzeit auf **Starten**. Auf einem nativen Host klickst du
   vorher auf **Laufzeit installieren**. Die Karte **Dienst** zeigt **Bereit**, sobald die Laufzeit antwortet.

## Die Betriebsart wählen

| Betriebsart | Wofür | Hinweise |
|---|---|---|
| **Verwaltetes Docker** | ESP32-CSI auf einem Linux-Sensing-Server oder eine Demo | AuraGo holt das festgelegte Image, startet einen eigenen, markierten Container und bindet ihn an den Loopback. Docker darf nicht schreibgeschützt sein. |
| **Native Host-Laufzeit** | Host-WLAN-RSSI unter Windows oder macOS | Klicke einmal auf **Laufzeit installieren**. Dieses Release enthält ein geprüftes Paket nur für Windows (amd64). Unter macOS bricht die Installation ab, weil es für diese Plattform in diesem Release kein geprüftes Paket gibt. |
| **Externe Instanz** | Ein RuView-Dienst, den du selbst betreibst | Trage die **Dienstadresse** als reine Origin ein, zum Beispiel `http://192.168.1.20:3000`, ohne Pfad, Abfrage oder Zugangsdaten. Private oder Loopback-Adressen brauchen **Privaten Netzwerkzugriff erlauben**. AuraGo installiert, startet, stoppt oder aktualisiert eine externe Instanz nie. |

Ein Container sieht den WLAN-Adapter des Hosts nicht. Läuft AuraGo in einem Container, lehnt die Konfigurationsprüfung die
native Laufzeit ab. Betreibe RuView dann als externe Instanz auf dem Rechner mit dem Adapter. Host-WLAN-RSSI braucht
Windows oder macOS. Unter Linux nutzt du ESP32-Boards. Die **Ausdrückliche Demo** ist zum Ausprobieren gedacht. Es gibt
keinen automatischen Ersatz durch Demodaten.

## Übersicht und Geräte lesen

Die **Übersicht** zeigt die Karten **Dienst**, **Erfassung**, **Quelle**, **Artefakt** und **Speicher** sowie die letzte
Aufnahme-Aktion, die in dieser Sitzung bestätigt wurde. Die Karte **Artefakt** zeigt **Geprüftes Artefakt** nur, wenn
AuraGo die festgelegten Laufzeitdateien selbst gestartet hat. Darunter stehen fünf Prüfungen mit den Zuständen
*Erledigt*, *Offen*, *Noch nicht geprüft* oder, wenn die Quelle die Prüfung nicht braucht, *Für diese Quelle nicht nötig*:

| Prüfung | Worauf sie achtet |
|---|---|
| **Dienst** | Die Laufzeit meldet sich bereit. |
| **UDP-Empfang** | Bei WLAN- und Demoquellen zeigt sie *Für diese Quelle nicht nötig*. Eine externe Instanz regelt UDP selbst. Bei einer verwalteten Laufzeit ist die Prüfung *Erledigt*, wenn UDP an eine private LAN-Adresse mit mindestens einer erlaubten Quelle gebunden ist. *Offen* steht sie, wenn UDP ausgeschaltet ist oder wenn der Dienst bereit ist, aber keine LAN-Bind-Adresse oder keine erlaubten Quellen hat. Ist UDP eingeschaltet, der Dienst aber nicht bereit, steht sie auf *Noch nicht geprüft*. |
| **Empfängeradresse** | Eine private IPv4-Adresse des Empfängers ist gespeichert. Für WLAN und Demo nicht nötig. |
| **Bekanntes Board** | AuraGo kennt mindestens ein Board. |
| **Frisches CSI** | Ein Board hat in den letzten zehn Sekunden CSI-Daten gesendet. |

Ein Punkt mit *Erledigt* heißt nicht, dass eine Person erkannt wurde. **Frisches CSI** zeigt nur, dass ein Board kürzlich
Daten gesendet hat.

Bei WLAN- und Demoquellen zeigen **UDP-Empfang**, **Bekanntes Board** und **Frisches CSI** *Für diese Quelle nicht nötig*. Läuft eine verwaltete Laufzeit nicht, bleiben beide auf *Noch nicht geprüft*, weil AuraGo den Dienst für die Board-Liste nicht anfragt. Die Karte **Erfassung** zeigt **Warte auf erste Daten**, solange ein verbundener Dienst noch keinen ersten Frame gesendet hat. Ein Stopp, der nicht bestätigt werden konnte, oder ein beendeter Dienst bleibt angezeigt, bis du erneut **Starten** oder **Stoppen** klickst.

Der Tab **Geräte** listet alle Boards, die AuraGo kennt. Er aktualisiert sich alle drei Sekunden, solange er sichtbar ist.
**Letzte Messung** zeigt das Alter des letzten Werts. Werte, die älter als zehn Sekunden sind, gelten als veraltet. Still
gewordene Boards bleiben in der Liste. **Dieses Board einrichten** öffnet den Board-Assistenten für genau diese
Knoten-ID. Ist die Liste leer, werden bereits eingerichtete Boards beim nächsten Datenempfang wieder übernommen. Ein
erneutes Flashen ist dafür nicht nötig. Gibt es keine Live-Daten, nennt der Tab den Grund. Bei WLAN- und Demoquellen steht dort „Die Board-Liste wird nur mit ESP32-Boards verwendet.“ Läuft eine verwaltete Laufzeit nicht, steht dort „Der RuView-Dienst läuft nicht. Gespeicherte Boards werden ohne Live-Daten angezeigt.“

## Ein ESP32-Board einrichten

Vorab:

- Öffne AuraGo über HTTPS oder `localhost` in Chrome oder Edge. Firefox und Safari kennen kein Web Serial.
- Stecke das Board in den Rechner, auf dem der Browser läuft, mit einem Kabel, das Daten überträgt. Schließe andere
  Programme, die denselben seriellen Port nutzen.
- Stelle **Quelle** auf **ESP32-CSI** und schalte **Nur lesen** aus. Der Board-Bereich ist bei anderen Quellen
  ausgeblendet.

Den Assistenten öffnest du über **Board einrichten** (in der Konfiguration oder unten im Tab **Geräte**), über
**Firmware schreiben** in der Übersicht oder über **Dieses Board einrichten** bei einem bekannten Board.

**Schritt 1: Firmware schreiben.** Wähle das **Boardprofil**, das zu Chip, Flashgröße und USB-Anschluss passt. Nimm `cdc`
für „USB JTAG/serial debug unit“ und `uart` für eine USB-UART-Brücke. Ein ESP32-S3-WROOM-1 N16R8 hat 16 MB Flash und 8 MB
PSRAM, also wählst du ein 16-MB-Profil. Die Firmware nutzt den PSRAM nicht. Setze das Häkchen bei **Alle Einstellungen und
Aufnahmen auf diesem Board löschen** nur, wenn der Dialog es verlangt. Das geschieht nur bei einer anderen Partitionierung. Ein neuer, leerer Chip wird ohne Löschen geschrieben. Das Label
nennt Aufnahmen, doch Aufnahmen von AuraGo liegen auf dem Host, nicht auf dem Board. Klicke auf **Firmware schreiben**.
Der Dialog zeigt jede Phase: Dateiprüfung, Verbindung, Löschen, Schreiben mit Prozentangabe, Prüfung und Neustart.
Meldet er **Schritt 1 abgeschlossen**, klickst du auf **Weiter zu Schritt 2**.

**Schritt 2: WLAN und Empfänger einrichten.** Trage **WLAN-Netz (SSID)** und **WLAN-Passwort** ein oder setze **WLAN-Einstellungen
beibehalten**, um die bestehenden Einstellungen des Boards zu behalten. Wähle eine **Knoten-ID** von 1 bis 255. Der Dialog
schlägt die kleinste freie ID unter den Boards vor, die AuraGo kennt. Der Vorschlag reserviert nichts, und Boards, die
AuraGo nie gesehen hat, kann er nicht berücksichtigen. Trage die **IPv4 des RuView-Empfängers** und den **UDP-Port** ein
(Standard 5005) und klicke auf **Board einrichten**. Nach dem Neustart des Boards verbindest du es erneut und klickst auf
**Nach Neustart bestätigen**.

Der Empfänger ist die Adresse des RuView-Rechners, die das Board im WLAN erreicht. Bei verwaltetem Docker ist das die
LAN-Adresse des Docker-Hosts, nicht eine Container-Adresse. **LAN-Adresse übernehmen** (mit der erkannten IP) füllt nur
leere Felder. Dabei wird weder UDP eingeschaltet noch etwas gespeichert.

**Schritt 3: Sensordaten prüfen.** Klicke auf **Sensordaten prüfen**. Die Prüfung gelingt nur mit frischen CSI-Daten des
eingerichteten Knotens. Erfolgreiches Flashen und eine bestätigte Konfiguration beweisen noch nicht, dass UDP bei AuraGo
ankommt.

Damit Schritt 3 gelingen kann, muss ein Admin UDP freigeben. Schalte **UDP freigeben** ein, setze **LAN-Bind-Adresse (IPv4)**
auf eine konkrete private Adresse des RuView-Rechners (niemals `0.0.0.0`) und trage unter **Erlaubte Quell-IPs/CIDRs,
kommagetrennt** die Board-Adressen ein, zum Beispiel `192.168.1.50` oder `192.168.1.0/24`. Der Standardport ist 5005. NAT in
Docker und die Firewall des Hosts müssen den UDP-Weg durchlassen.

WLAN-Zugangsdaten bleiben im Arbeitsspeicher des Browsers, gehen direkt an das Board und werden beim Schließen des Dialogs
gelöscht. AuraGo speichert sie nie. Deine Eingaben bleiben erhalten, wenn ein Schritt scheitert. Ein Schreibvorgang, der
unklar abgebrochen ist, wird nie automatisch wiederholt. Meldet der Dialog, dass der Bootloader nicht erreichbar ist, halte
BOOT beim erneuten Verbinden gedrückt, lasse es danach los und versuche es noch einmal. Lade die Seite nach einem
AuraGo-Update neu, bevor du das nächste Board einrichtest.

## Die Upstream-Ansichten

Die App hat die Tabs **Übersicht**, **Geräte**, **Dashboard**, **Observatorium** und **Pose-Fusion**. **Einstellungen**
öffnet die Konfiguration. Die drei Upstream-Tabs sind die eigenen Weboberflächen von RuView, geladen in einem abgeschotteten
Rahmen.

- **Sprache.** Die Upstream-Ansichten behalten ihre eigene Sprache (Englisch, mit einer polnischen Option in ihrer
  Oberfläche). AuraGo übersetzt seine Verwaltungsseiten und die Board-Einrichtung, nicht die Upstream-Ansichten.
- **Kalibrierung, Aufnahme und Training** sind Schreibaktionen. Starts und Änderungen brauchen **Nur lesen** aus. Stoppen
  bleibt erlaubt. Aufnahme und Training findest du im Dashboard in den Tabs **Live Demo** und **Training**.
- **Pose-Fusion** startet im Modus **CSI Only (WiFi)**. Kamera- und Videomodi gibt es in AuraGo nicht. Der Tab sagt:
  „Pose-Fusion verwendet untrainierte Beispielmodelle. Darstellung und Konfidenzwerte sind keine validierten Messungen.“
- **Gesperrte Funktionen.** Laden und Entladen von Modellen, SONA- und LoRA-Aktivierung, Posenstatistiken und Demo-Zonen gibt
  es in diesem Stand nicht, und AuraGo blockiert sie. Zeitgesteuerte Aufnahmen werden abgelehnt.
- **Ehrliche Anzeige.** Der Tab Hardware der Upstream-Ansicht zeigt keine simulierte CSI-Messung mehr. Amplitude und Phase stehen auf „—“. Antennen- und WLAN-Werte sind als Beispielkonfiguration gekennzeichnet, die nicht aus deiner Hardware gelesen wird. Die Platzhalter für Signalstärke und Latenz im Tab Live Demo zeigen „—“. Die Schaltfläche **Demo** erscheint nur, solange die RuView-Quelle **Ausdrückliche Demo** ist. Sie verschwindet, wenn sich die Quelle ändert, der Stream stoppt oder du den Tab verlässt. Werbe- und Benchmark-Abschnitte tragen den Hinweis „Upstream RuView project claims — not measured or verified by AuraGo.“ Die Hinweise bleiben englisch, weil die Upstream-Ansichten ihre Sprache behalten.
- **Tour.** Ob du die Dashboard-Tour übersprungen oder beendet hast, merkt sich dieser Browser. Um sie erneut zu sehen,
  öffne im Dashboard den Einstellungsknopf (**Settings**), wähle **Reset onboarding** und klicke auf **Reset**.

## Aufnahmen und Speicher

Die Tabs **Live Demo** und **Training** im Dashboard haben die Schaltflächen **Start Recording** und **Stop Recording**. Eine
Aufnahme läuft, bis du sie stoppst,
auch wenn du die Ansicht schließt. Nach dem Neuladen kann die Ansicht nicht sagen, ob eine Aufnahme läuft, und die
Dateiliste zeigt es ebenfalls nicht. Die Übersicht zeigt deshalb nur den letzten Start oder Stopp, den AuraGo abgeschickt
und bestätigt bekommen hat. Nach einem Neustart von AuraGo steht dort wieder „Aufnahmestatus unbekannt“.

Verwaltete Aufnahmen und Modelle liegen in `data/ruview/data` im Datenverzeichnis von AuraGo. Das **Speicherbudget (MiB)**
begrenzt sie. AuraGo prüft den Ordner jede Sekunde. Ist das Budget erreicht, lehnt AuraGo neue Aufnahmestarts ab (HTTP 507)
und bittet den Dienst, die laufende Aufnahme zu stoppen. Daten, die zwischen zwei Prüfungen entstehen, können das Budget
überschreiten. Die Karte **Speicher** zeigt **Budget erreicht** oder **Im Budget**.

AuraGo löscht nie automatisch Aufnahmen. Entferne nicht mehr benötigte Aufnahmen selbst. Im Lesemodus ist Löschen gesperrt.
Externe Instanzen haben ihren eigenen Speicher, für sie gilt das Budget nicht.

## Gut zu wissen

- **Experimentell.** Verbindung, Empfang und ein frischer Status zeigen, dass Daten fließen. Sie zeigen nicht, dass eine
  Messung, eine Präsenz oder eine Pose stimmt. Konfidenzwerte sind nicht kalibriert.
- **Signale.** RSSI braucht Windows oder macOS mit einer nativen oder externen Instanz. CSI braucht ESP32-Boards.
- **Demodaten** sind gekennzeichnet (die Karte zeigt **Demo**). Fällt ein Sensor aus, wechselt AuraGo nicht auf Demodaten.
- **Veraltete Daten.** Frames, die älter als zehn Sekunden sind, verlieren den Status „frisch“. Werte in diesem Alter gelten
  als veraltet.
- **Unterstützte Boards.** Nur die acht Firmware-Profile des Katalogs: ESP32-S3 mit 4, 8 oder 16 MB Flash und ESP32-C6 mit
  4 MB Flash, jeweils als UART-Brücke (`uart`) oder native USB-Variante (`cdc`). Andere Chips, auch der klassische ESP32,
  werden abgelehnt, bevor etwas geschrieben wird. Die Firmware hat Version 0.8.12.
- **Nicht in diesem Stand:** Laden und Entladen von Modellen, SONA und LoRA, Posenstatistiken, Demo-Zonen, Kamera- und
  Videomodi sowie zeitgesteuerte Aufnahmen.
- **Plattformen.** Dieses Release enthält ein natives Paket nur für Windows (amd64). macOS-Pakete stehen noch aus. Das
  verwaltete Image gibt es für amd64 und arm64, der arm64-Build lief aber noch nicht auf einem arm64-Rechner. Der verwaltete
  Container bekommt zwei CPUs, 2 GiB Speicher und ein schreibgeschütztes Root-Dateisystem.
- **Hardware-Abnahme** ist für dieses Release nicht abgeschlossen. Echte Erstinstallationen auf S3 und C6, Updates mit
  erhaltenen Einstellungen, Neukonfiguration und RSSI mit echten Adaptern sind noch offen.

## Datenschutz und Sicherheit

- **Was den Rechner verlässt.** Verwaltetes Docker lädt das festgelegte Image `ghcr.io/antibyte/aurago-ruview` aus der
  GitHub Container Registry. Die Windows-Laufzeit und die Firmware stammen aus dem eigenen Ressourcenpaket von AuraGo und
  werden vor der Nutzung per Hash geprüft. Bei einer externen Instanz spricht AuraGo nur mit der gespeicherten URL.
  Anfragen mit Zugangsdaten folgen nie Weiterleitungen.
- **WLAN-Zugangsdaten** gehen vom Browser direkt über USB an das Board. Die APIs von AuraGo bekommen sie nie, und sie werden
  nicht im Browserspeicher abgelegt.
- **Abschottung.** Die Upstream-Ansichten haben keine eigene Netzverbindung und keine Berechtigung für serielle Ports, Kamera
  oder Mikrofon. AuraGo leitet nur die geprüften Aufrufe weiter.
- **Wo Daten liegen:**

  | Daten | Speicherort |
  |---|---|
  | Verwaltete Aufnahmen und Modelle | `data/ruview/data` |
  | Dateien der nativen Laufzeit | `data/ruview/runtime/<revision>/<platform>` |
  | Bekannte Board-IDs (keine Messwerte, keine WLAN-Zugangsdaten) | `data/ruview/`, eine Datei je Empfänger |

- **Vault-Schlüssel.** `ruview_api_token` enthält das **API-Token (Vault)** einer externen Instanz. `ruview_managed_token`
  entsteht beim ersten Start der verwalteten Laufzeit und sichert die Verbindung zwischen AuraGo und dem Dienst. Keiner der
  beiden Schlüssel wird an Python-Skripte oder den Agenten exportiert.
- **Audit.** RuView hat in diesem Release kein eigenes Audit-Protokoll. Änderungen laufen über die Administrator-Anmeldung von
  AuraGo, die Origin-Prüfung und den Schreibschutz.
- **Der Agent.** Ist **Agent-Lesezugriff erlauben** an und die Integration aktiv, kann der Agent das Werkzeug `ruview` mit
  `status`, `nodes`, `sensing`, `model_info`, `recordings` und `presence` aufrufen. Er kann nichts installieren, starten,
  flashen, einrichten, kalibrieren, aufnehmen oder trainieren. Jede Antwort nennt Quelle, Zeit und Frische. `fresh: false`
  belegt keine Anwesenheit, und eine Simulation ist kein Sensor.
- **Personen im Raum.** Sensorik kann zeigen, dass sich jemand in einem Raum bewegt. Informiere die betroffenen Personen,
  entscheide, ob das vertretbar ist, und lösche Aufnahmen, die du nicht mehr brauchst.

## Wenn etwas nicht klappt

| Meldung | Was hilft |
|---|---|
| „Aktiviere RuView und speichere die Einstellungen, bevor Du fortfährst.“ | Schalte **Integration aktivieren** ein und klicke auf **Speichern**. |
| „Speichere Änderungen vor Verbindungstest oder Board-Einrichtung.“ | Klicke auf **Speichern**. Tests und Board-Einrichtung nutzen die gespeicherten Einstellungen. |
| „Schalte oben „Nur lesen“ aus und speichere, um ein Board einzurichten.“ | Schalte **Nur lesen** aus, speichere und versuche es erneut. |
| „Host-WLAN benötigt eine native oder externe Instanz. Wähle diese Betriebsart oder eine andere Datenquelle.“ | Wähle als **Betriebsart** **Native Host-Laufzeit** oder **Externe Instanz** oder eine andere **Quelle**. |
| „Installieren gibt es nur für die native Laufzeit.“ | **Laufzeit installieren** gibt es nur für die native Host-Laufzeit. Verwaltetes Docker startest du mit **Starten**. |
| „Eine externe Instanz startest und stoppst Du an ihrem eigenen Rechner.“ | AuraGo startet und stoppt externe Instanzen nicht. Arbeite auf deren Rechner. |
| „Der Dienst wurde beendet. Du kannst ihn erneut starten.“ | Klicke auf **Starten**. AuraGo startet den Dienst nicht selbst neu. |
| „Das Stoppen konnte nicht bestätigt werden. Prüfe den Dienst.“ | Der Stopp ist nicht bestätigt. Prüfe den Dienst auf seinem Rechner. |
| „Noch keine frischen CSI-Daten von diesem Board. Prüfe WLAN, Empfängeradresse und UDP-Freigabe.“ | Prüfe das WLAN des Boards, die **IPv4 des RuView-Empfängers** und die Einstellungen für **UDP freigeben**. |
| „UDP-Empfang ist noch ausgeschaltet. Aktiviere ihn in den RuView-Einstellungen und erlaube die Board-Adressen, bevor Du Sensordaten prüfst.“ | Schalte **UDP freigeben** ein, setze **LAN-Bind-Adresse (IPv4)** und **Erlaubte Quell-IPs/CIDRs**, speichere und prüfe erneut. |
| „Bootloader nicht erreichbar. Halte BOOT beim erneuten Verbinden gedrückt, lasse BOOT danach los und schließe andere Programme am seriellen Port.“ | Halte BOOT beim erneuten Verbinden gedrückt, lasse es los und schließe andere serielle Programme. |
| „Nutze HTTPS oder localhost und einen Browser mit freigegebenem Web Serial. Schließe andere serielle Werkzeuge und nutze ein USB-Datenkabel. Halte bei Erkennungsproblemen BOOT beim erneuten Verbinden gedrückt.“ | Öffne AuraGo über HTTPS oder `localhost` in Chrome oder Edge und nutze ein Datenkabel. |
| „Erkannt: {detected} MB Flash. Gewähltes Profil: {expected} MB. Wähle das passende Profil; es wurde nichts geschrieben.“ | Wähle das Profil zur erkannten Flashgröße. |
| „USB-Anschluss und Boardprofil passen nicht zusammen. Wähle für „USB JTAG/serial debug unit“ ein Profil mit „cdc“; für eine USB-UART-Brücke ein Profil mit „uart“.“ | Wähle `cdc` für den nativen USB-Anschluss oder `uart` für eine Brücke. |
| „Dieses Board hat ein anderes Partitionslayout. Aktiviere „Alle Einstellungen und Aufnahmen auf diesem Board löschen“, um es zu ersetzen; dabei werden Einstellungen und Aufnahmen auf dem Board gelöscht.“ | Setze das Häkchen beim Löschen und klicke erneut auf **Firmware schreiben**. Dabei werden die eigenen Einstellungen des Boards gelöscht. |
| „Prüfung fehlgeschlagen. Es gibt keine automatische Wiederholung. Prüfe Firmwaredateien, USB-Verbindung und Board vor einem weiteren Versuch.“ | Es wurde nichts wiederholt. Prüfe Kabel und Board und starte dann selbst neu. |
| „Verbinde das Board nach dem Neustart erneut und bestätige“ | Verbinde das Board nach dem Neustart erneut und klicke auf **Nach Neustart bestätigen**. |
| „Knoten-ID: 1–255; IPv4 des RuView-Empfängers: 10/8, 172.16/12, 192.168/16; UDP: 1–65535.“ | Korrigiere den Wert: Knoten-ID von 1 bis 255, private IPv4 für den Empfänger, UDP-Port im Bereich. |
| „Alle IDs (1–255) sind als belegt gemeldet. Gib eine bestehende ID nur ein, um dieses Board erneut einzurichten.“ | Gib eine bestehende ID nur ein, um dieses Board erneut einzurichten. |
| **Budget erreicht** (Karte **Speicher**) | Es starten keine neuen Aufnahmen. Entferne nicht mehr benötigte Aufnahmen oder erhöhe das **Speicherbudget (MiB)**. |
| **Getrennt**, **Veraltet** oder **Nicht geprüft** (Ergebnis von **Verbindung testen**) | Der Dienst hat nicht geantwortet, der letzte Frame ist älter als zehn Sekunden oder es gibt noch keine Daten. Prüfe den Dienst oder klicke auf **Starten**. |
| **Warte auf erste Daten** (Ergebnis von **Verbindung testen**) | Der Dienst ist verbunden, hat aber noch keinen Sensorframe gesendet. Das ist eine Warnung, kein Erfolg. Prüfe das WLAN des Boards, die **IPv4 des RuView-Empfängers** und **UDP freigeben** und teste dann erneut. |
| **Status nicht abrufbar** (**Übersicht**) | Die erste Statusabfrage ist fehlgeschlagen. Die Übersicht wiederholt sie alle drei Sekunden, solange sie offen ist. Bleibt es so, prüfe, ob AuraGo läuft und erreichbar ist. |

---

**Siehe auch:** [Kapitel 4: Web-Oberfläche](./04-webui.md) · [Kapitel 8: Integrationen](./08-integrations.md#ruview-wi-fi-sensing) · [Kapitel 14: Sicherheit](./14-security.md) · [Kapitel 16: Troubleshooting](./16-troubleshooting.md)

---

**Vorheriges Kapitel:** [Kapitel 29: Infra](./29-infra.md)
