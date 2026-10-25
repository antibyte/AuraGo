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
4. [Status und Boards lesen](#status-und-boards-lesen)
5. [Ein ESP32-Board einrichten](#ein-esp32-board-einrichten)
6. [Das Labor](#das-labor)
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
**Dienst starten**. Eine externe Instanz startet AuraGo nie.

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
   **Kommunikation**. Das Öffnen der App startet den Dienst nicht. Solange der Tab **Boards** offen ist, fragt AuraGo alle drei Sekunden die Board-Liste beim Dienst ab. Die Ansicht **Status** fragt sie nur bei einem verwalteten ESP32-Dienst ab. Ein verwalteter Dienst, der nicht läuft, wird nicht angesprochen.
4. Klicke in der Ansicht **Status** der App bei einer verwalteten Laufzeit auf **Dienst starten**. Auf einem nativen Host klickst du
   vorher auf **Installieren**. Die Überschrift lautet „RuView läuft und wartet auf Daten der Boards.“, sobald die Laufzeit antwortet, und beginnt mit „Läuft:“, sobald Daten ankommen.

## Die Betriebsart wählen

| Betriebsart | Wofür | Hinweise |
|---|---|---|
| **Verwaltetes Docker** | ESP32-CSI auf einem Linux-Sensing-Server oder eine Demo | AuraGo holt das festgelegte Image, startet einen eigenen, markierten Container und bindet ihn an den Loopback. Docker darf nicht schreibgeschützt sein. |
| **Native Host-Laufzeit** | Host-WLAN-RSSI unter Windows oder macOS | Klicke einmal auf **Installieren** (oder auf **Dienst installieren** im Menü **⋯**). Dieses Release enthält ein geprüftes Paket nur für Windows (amd64). Unter macOS bricht die Installation ab, weil es für diese Plattform in diesem Release kein geprüftes Paket gibt. |
| **Externe Instanz** | Ein RuView-Dienst, den du selbst betreibst | Trage die **Dienstadresse** als reine Origin ein, zum Beispiel `http://192.168.1.20:3000`, ohne Pfad, Abfrage oder Zugangsdaten. Private oder Loopback-Adressen brauchen **Privaten Netzwerkzugriff erlauben**. AuraGo installiert, startet, stoppt oder aktualisiert eine externe Instanz nie. |

Ein Container sieht den WLAN-Adapter des Hosts nicht. Läuft AuraGo in einem Container, lehnt die Konfigurationsprüfung die
native Laufzeit ab. Betreibe RuView dann als externe Instanz auf dem Rechner mit dem Adapter. Host-WLAN-RSSI braucht
Windows oder macOS. Unter Linux nutzt du ESP32-Boards. Die **Ausdrückliche Demo** ist zum Ausprobieren gedacht. Es gibt
keinen automatischen Ersatz durch Demodaten.

## Status und Boards lesen

Die Befehlsleiste hat drei Ansichten: **Status**, **Boards** und **Labor**. **Boards** erscheint nur für die Quelle
ESP32-CSI. **Labor** erscheint, solange RuView eingeschaltet ist. In einem schmalen Fenster rutschen die Ansichten in
ein Menü.

Die Ansicht **Status** beantwortet zuerst eine Frage: Läuft es? Lies zuerst die Überschrift. Sie ist ein Satz. Darunter
stehen:

- **Nächster Schritt.** Ein Knopf, wenn es einen gibt. Ist er ausgegraut, nennt die Zeile darunter den Grund.
- **Einrichtung.** Bei einer neuen Installation zeigt eine Liste, was noch fehlt: *Eingeschaltet*, *Änderungen erlaubt*,
  *Dienst läuft*, *Empfänger und UDP eingerichtet*, *Erstes Board* und *Daten kommen an*. Jeder offene Schritt hat einen
  eigenen Knopf. Die Liste verschwindet, sobald echte Sensordaten angekommen sind.
- **Details.** Standardmäßig eingeklappt. Sie zeigen Dienst und Version, Quelle, Speicher im Verhältnis zum Budget, die
  letzte Aufnahme-Aktion, die diese AuraGo-Sitzung bestätigt hat, und offene Hinweise. **Diagnose kopieren** kopiert
  dieselben Angaben als reine `key=value`-Zeilen. Darin stehen kein Token, keine URL und kein Dateipfad.

Die Pille in der Befehlsleiste zeigt den Zustand in Worten, mit Symbol und Farbe, zum Beispiel *Live*, *Wartet*,
*Problem* oder *Schreibgeschützt*.

**Installieren** wird nur für die native Host-Laufzeit angeboten. Ein verwalteter Dienst, der nicht läuft, zeigt „Der RuView-Dienst läuft nicht.“
und bietet **Dienst starten** an. Die Zeile
darunter nennt den Grund, zum Beispiel „Von Dir um 14:02 gestoppt.“ Das Öffnen der App startet nie einen Dienst.

Die Ansicht Status fragt im Hintergrund nie einen externen Dienst ab. Bei einer externen Instanz klickst du auf
**Jetzt prüfen** im Menü **⋯** oder im nächsten Schritt. Das Ergebnis erscheint in der Überschrift. Zehn Sekunden später
nennt sie, wann der Dienst zuletzt geprüft wurde, und bietet wieder **Jetzt prüfen** an. Das gilt nicht, wenn die Prüfung
fehlgeschlagen ist.

Ein verbundener Dienst, der noch keinen ersten Frame gesendet hat, zeigt „RuView läuft und wartet auf Daten der Boards.“ Das ist ein Wartezustand, kein Erfolg. Ein Stopp, der nicht bestätigt werden konnte, oder ein Dienst, der von selbst beendet wurde, bleibt angezeigt, bis du erneut **Dienst starten** oder **Dienst stoppen** klickst.

**Änderungen erlauben …** erscheint, wenn der Schreibschutz einen Start blockiert. Vorher bestätigst du die Abfrage. Danach speichert
AuraGo nur den Schalter für den Schreibschutz über das übliche Speichern der Administrator-Einstellungen. Nichts anderes
wird gesendet. Auf einem schreibgeschützten Desktop öffnet der Knopf stattdessen die Einstellungen.

Der Tab **Boards** listet alle Boards, die AuraGo kennt, und liest die Liste alle drei Sekunden, solange der Tab sichtbar
ist. **Letzte Messung** zeigt das Alter des letzten Werts. Werte, die älter als zehn Sekunden sind, gelten als veraltet.
Still gewordene Boards bleiben in der Liste. **Dieses Board einrichten** öffnet den Board-Assistenten für genau diese
Knoten-ID, **Board hinzufügen** öffnet ihn für eine neue ID. Ist die Liste leer, werden bereits eingerichtete Boards beim
nächsten Datenempfang wieder übernommen. Ein erneutes Flashen ist dafür nicht nötig. Lässt sich die Liste nicht lesen,
nennt der Tab den Grund. Läuft eine verwaltete Laufzeit nicht, steht dort „Der RuView-Dienst läuft nicht. Gespeicherte Boards werden ohne Live-Daten angezeigt.“ Der Tab **Boards** ist bei WLAN- und Demoquellen ausgeblendet.

## Ein ESP32-Board einrichten

Vorab:

- Öffne AuraGo über HTTPS oder `localhost` in Chrome oder Edge. Firefox und Safari kennen kein Web Serial.
- Stecke das Board in den Rechner, auf dem der Browser läuft, mit einem Kabel, das Daten überträgt. Schließe andere
  Programme, die denselben seriellen Port nutzen.
- Stelle **Quelle** auf **ESP32-CSI** und schalte **Nur lesen** aus. Der Board-Bereich ist bei anderen Quellen
  ausgeblendet.

Den Assistenten öffnest du über **Board einrichten** in der Konfiguration, über **Board hinzufügen** im Tab **Boards**
oder über **Dieses Board einrichten** bei einem bekannten Board.

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

## Das Labor

**Labor** enthält die eigenen Webseiten von RuView: **Observatorium**, **Pose-Fusion** und das Original-RuView-Dashboard.
Jede Seite hat eine Karte, die ihre Voraussetzung nennt. Wähle **Öffnen**, um eine Seite zu laden. Sie öffnet sich in
einem abgeschotteten Rahmen. **← Labor** oder **Schließen** entfernt den Rahmen, ebenso das Verlassen der Ansicht. Die
Seiten sind englisch und bleiben experimentell. **Einstellungen** im Menü **⋯** öffnet die Konfiguration.

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
Dateiliste zeigt es ebenfalls nicht. Die Details zeigen deshalb nur den letzten Start oder Stopp, den AuraGo abgeschickt
und bestätigt bekommen hat. Nach einem Neustart von AuraGo steht dort wieder „Aufnahmestatus unbekannt“.

Verwaltete Aufnahmen und Modelle liegen in `data/ruview/data` im Datenverzeichnis von AuraGo. Das **Speicherbudget (MiB)**
begrenzt sie. AuraGo misst den Ordner höchstens alle 30 Sekunden. Ist das Budget erreicht, lehnt AuraGo neue Aufnahmestarts ab (HTTP 507)
und bittet den Dienst, die laufende Aufnahme zu stoppen. Daten, die zwischen zwei Messungen entstehen, können das Budget
überschreiten. **Details** zeigt den belegten Speicher im Verhältnis zum Budget. Die Ansicht Status zeigt den Chip
**Speicher voll · Aufnahme gestoppt**, wenn das Budget erreicht ist, oder **Speicher nicht prüfbar**, wenn der Ordner nicht
gemessen werden kann. Dann lehnt AuraGo Aufnahmestarts mit HTTP 503 ab; das Budget gilt dann nicht als voll.

AuraGo löscht nie automatisch Aufnahmen. Entferne nicht mehr benötigte Aufnahmen selbst. Im Lesemodus ist Löschen gesperrt.
Externe Instanzen haben ihren eigenen Speicher, für sie gilt das Budget nicht.

## Gut zu wissen

- **Experimentell.** Verbindung, Empfang und ein frischer Status zeigen, dass Daten fließen. Sie zeigen nicht, dass eine
  Messung, eine Präsenz oder eine Pose stimmt. Konfidenzwerte sind nicht kalibriert.
- **Signale.** RSSI braucht Windows oder macOS mit einer nativen oder externen Instanz. CSI braucht ESP32-Boards.
- **Demodaten** sind gekennzeichnet. Die Überschrift lautet „Demo-Modus: simulierte Daten, keine Messungen.“ Fällt ein Sensor aus, wechselt AuraGo nicht auf Demodaten.
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
| „Installieren gibt es nur für die native Laufzeit.“ | **Installieren** (oder **Dienst installieren** im Menü **⋯**) gibt es nur für die native Host-Laufzeit. Verwaltetes Docker startest du mit **Dienst starten**. |
| „Eine externe Instanz startest und stoppst Du an ihrem eigenen Rechner.“ | AuraGo startet und stoppt externe Instanzen nicht. Arbeite auf deren Rechner. |
| „RuView wurde unerwartet beendet.“ | Klicke auf **Erneut starten**. AuraGo startet den Dienst nicht selbst neu. |
| „Stoppen nicht bestätigt. RuView läuft eventuell noch.“ | Klicke auf **Erneut stoppen**. Bleibt es so, prüfe den Dienst auf seinem Rechner. |
| „RuView startet nicht: Die Registry verweigert das Dienst-Image.“ | Das Image muss veröffentlicht und für AuraGo lesbar sein. Eine andere Empfängeradresse behebt das nicht. |
| „RuView startet nicht: Das Dienst-Image konnte nicht geladen werden.“ | Prüfe das Netzwerk und den Docker-Host, dann klicke auf **Start wiederholen**. |
| „RuView startet nicht: Docker ist nicht erreichbar.“ | Prüfe, ob Docker läuft und AuraGo es erreichen kann, dann klicke auf **Start wiederholen**. |
| „Ein Container namens aurago-ruview gehört zu einer anderen AuraGo-Installation. Entferne oder benenne ihn in Docker um und starte dann erneut.“ | AuraGo entfernt nie einen Container, den es nicht selbst angelegt hat. Benenne ihn in Docker um oder entferne ihn selbst und starte dann neu. |
| „RuView ist mit einem anderen Start oder einer Installation beschäftigt. Warte, bis sie fertig ist.“ | Warte, bis der andere Vorgang fertig ist, und prüfe dann den Status erneut. |
| „RuView hat nicht rechtzeitig geantwortet.“ | Prüfe den Dienst und klicke auf **Erneut prüfen** oder **Start wiederholen**. |
| „RuView hat das Zugriffstoken von AuraGo abgelehnt.“ | Prüfe das **API-Token (Vault)** der externen Instanz in den Einstellungen. |
| „RuView nutzt eine andere Signalquelle als in AuraGo eingestellt.“ | Stelle die **Quelle** in AuraGo auf die Quelle ein, die RuView nutzt, und prüfe dann erneut. |
| „RuView sendet simulierte Daten, obwohl echte Messung eingestellt ist.“ | Prüfe die Quellen-Einstellung von RuView. Echte Messungen dürfen keine simulierten Daten enthalten. |
| Schritt **Empfänger und UDP eingerichtet** ist offen (Einrichtung im Status). | Trage die **IPv4 des RuView-Empfängers** ein und schalte **UDP freigeben** mit einer LAN-Bind-Adresse und einer erlaubten Quelle ein. Speichere und prüfe dann erneut. |
| „Bootloader nicht erreichbar. Halte BOOT beim erneuten Verbinden gedrückt, lasse BOOT danach los und schließe andere Programme am seriellen Port.“ | Halte BOOT beim erneuten Verbinden gedrückt, lasse es los und schließe andere serielle Programme. |
| „Nutze HTTPS oder localhost und einen Browser mit freigegebenem Web Serial. Schließe andere serielle Werkzeuge und nutze ein USB-Datenkabel. Halte bei Erkennungsproblemen BOOT beim erneuten Verbinden gedrückt.“ | Öffne AuraGo über HTTPS oder `localhost` in Chrome oder Edge und nutze ein Datenkabel. |
| „Erkannt: {detected} MB Flash. Gewähltes Profil: {expected} MB. Wähle das passende Profil; es wurde nichts geschrieben.“ | Wähle das Profil zur erkannten Flashgröße. |
| „USB-Anschluss und Boardprofil passen nicht zusammen. Wähle für „USB JTAG/serial debug unit“ ein Profil mit „cdc“; für eine USB-UART-Brücke ein Profil mit „uart“.“ | Wähle `cdc` für den nativen USB-Anschluss oder `uart` für eine Brücke. |
| „Dieses Board hat ein anderes Partitionslayout. Aktiviere „Alle Einstellungen und Aufnahmen auf diesem Board löschen“, um es zu ersetzen; dabei werden Einstellungen und Aufnahmen auf dem Board gelöscht.“ | Setze das Häkchen beim Löschen und klicke erneut auf **Firmware schreiben**. Dabei werden die eigenen Einstellungen des Boards gelöscht. |
| „Prüfung fehlgeschlagen. Es gibt keine automatische Wiederholung. Prüfe Firmwaredateien, USB-Verbindung und Board vor einem weiteren Versuch.“ | Es wurde nichts wiederholt. Prüfe Kabel und Board und starte dann selbst neu. |
| „Verbinde das Board nach dem Neustart erneut und bestätige“ | Verbinde das Board nach dem Neustart erneut und klicke auf **Nach Neustart bestätigen**. |
| „Knoten-ID: 1–255; IPv4 des RuView-Empfängers: 10/8, 172.16/12, 192.168/16; UDP: 1–65535.“ | Korrigiere den Wert: Knoten-ID von 1 bis 255, private IPv4 für den Empfänger, UDP-Port im Bereich. |
| „Alle IDs (1–255) sind als belegt gemeldet. Gib eine bestehende ID nur ein, um dieses Board erneut einzurichten.“ | Gib eine bestehende ID nur ein, um dieses Board erneut einzurichten. |
| **Speicher voll · Aufnahme gestoppt** (Chip im Status) | Es starten keine neuen Aufnahmen. Entferne nicht mehr benötigte Aufnahmen oder erhöhe das **Speicherbudget (MiB)**. |
| **Speicher nicht prüfbar** (Chip im Status) | Der Datenordner lässt sich nicht messen: Berechtigung, Verknüpfung, Sonderdatei oder mehr als 200.000 Einträge. Aufnahmen starten erst, wenn er gemessen werden kann. Prüfe die Rechte des Ordners und entferne Dateien, die du nicht mehr brauchst. |
| „RuView läuft, aber die Signalquelle ist getrennt.“ | Prüfe das Board oder den WLAN-Adapter. Der Tab **Boards** zeigt das Alter der letzten Messung jedes Boards. |
| „Seit {age} keine neuen Daten.“ | Die Frames sind älter als zehn Sekunden. Prüfe das WLAN des Boards und den RuView-Dienst. Die Überschrift verschwindet, sobald neue Daten kommen. |
| „Externer RuView-Dienst, noch nicht geprüft.“ oder „Externer RuView-Dienst, zuletzt vor {age} geprüft.“ | Klicke auf **Verbindung prüfen** bzw. **Jetzt prüfen**. Nur eine Prüfung fragt einen externen Dienst ab. |
| **Warte auf erste Daten** (Verbindung testen) oder „RuView läuft und wartet auf Daten der Boards.“ (Status) | Der Dienst ist verbunden, hat aber noch keinen Sensorframe gesendet. Er wird als Warten angezeigt, nicht als Erfolg. Prüfe das WLAN des Boards, die **IPv4 des RuView-Empfängers** und **UDP freigeben** und teste dann erneut. |
| „RuView-Status nicht abrufbar.“ (Ansicht **Status**) | Der letzte bekannte Stand bleibt sichtbar. Die Ansicht wiederholt die Abfrage alle drei Sekunden, solange sie offen ist. Bleibt es so, prüfe, ob AuraGo läuft und erreichbar ist. |
| „Die Einstellung konnte nicht gespeichert werden.“ (**Änderungen erlauben …**) | Prüfe die RuView-Einstellungen. Ist ein anderer Konfigurationsabschnitt ungültig, schlägt das Speichern fehl. Korrigiere ihn in den **Einstellungen** und versuche es erneut. |

**Technische Details** stehen unter der Überschrift. Sie zeigen den bereinigten technischen Text von RuView oder Docker, ohne Tokens, URLs oder Dateipfade. **Diagnose kopieren** (im Menü **⋯** oder unter **Details**) kopiert Zeilen im Format `key=value` mit Zustand, Fehlercode und diesem Text. Füge sie als reinen Text in eine Meldung ein. Ist die Zwischenablage gesperrt, erscheint der Text in einem Feld, das du von Hand kopieren kannst.

---

**Siehe auch:** [Kapitel 4: Web-Oberfläche](./04-webui.md) · [Kapitel 8: Integrationen](./08-integrations.md#ruview-wi-fi-sensing) · [Kapitel 14: Sicherheit](./14-security.md) · [Kapitel 16: Troubleshooting](./16-troubleshooting.md)

---

**Vorheriges Kapitel:** [Kapitel 29: Infra](./29-infra.md)
