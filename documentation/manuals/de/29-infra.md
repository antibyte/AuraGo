# Kapitel 29: Infra – der Homelab-Atlas

Infra ist eine gemeinsame Karte deines Homelabs. Sie zeigt Hardware, virtuelle Maschinen, Container, Dienste und
Netzwerksegmente als Atlas, als Topologie-Diagramm oder als Inventarliste und verknüpft jeden Eintrag mit seiner Quelle:
dem Geräteregister, Docker, Proxmox oder FRITZ!Box. Infra ist für Administratoren gedacht, die ein Gesamtbild ihrer
Umgebung wollen. Das Tool scannt dein Netzwerk nicht und überwacht Geräte nicht.

Technische Details für Admins (API, Fehlercodes, Entwicklung) stehen in der [Betriebsreferenz](../../infra.md) (englisch).

## Inhaltsverzeichnis

1. [Was du brauchst und wie du es einschaltest](#was-du-brauchst-und-wie-du-es-einschaltest)
2. [Erste Schritte](#erste-schritte)
3. [Die Karte aufbauen](#die-karte-aufbauen)
4. [Ansichten, Orte und Beziehungen](#ansichten-orte-und-beziehungen)
5. [Einträge bearbeiten und speichern](#einträge-bearbeiten-und-speichern)
6. [Quellenänderungen](#quellenänderungen)
7. [Energieaktionen](#energieaktionen)
8. [Gut zu wissen](#gut-zu-wissen)
9. [Datenschutz und Sicherheit](#datenschutz-und-sicherheit)
10. [Wenn etwas nicht klappt](#wenn-etwas-nicht-klappt)

---

## Was du brauchst und wie du es einschaltest

Infra ist nur für Administratoren. Jede Infra-Anfrage braucht eine Desktop-Sitzung mit Administratorrechten. Zwei
Schalter müssen an sein:

- **Config → Virtueller Desktop → „Virtuellen Desktop aktivieren“** (`virtual_desktop.enabled`; die mitgelieferte `config_template.yaml` schaltet ihn ein,
  eine Config ohne den Schlüssel lässt ihn aus).
- **Config → Virtueller Desktop → „Infra aktivieren“** (`virtual_desktop.infra.enabled`, ab Werk an). Er steht in der
  Gruppe **Infra** auf dieser Seite. Schaltest du ihn aus, ist der Atlas für alle geschlossen. Die gespeicherte Karte
  bleibt auf der Platte.

Drei weitere Schalter stehen in derselben Gruppe. Sie sind bewusst getrennt:

| Schalter | Config-Schlüssel | Standard | Was er erlaubt |
|---|---|---|---|
| „Schreibgeschützt“ | `virtual_desktop.infra.read_only` | aus | Nur ansehen. Sperrt lokale Änderungen, Quellenänderungen und Gerätesteuerung. |
| „Quellenänderungen erlauben“ | `virtual_desktop.infra.allow_source_edit` | aus | Unterstützte Felder im Geräteregister, bei Docker, Proxmox und FRITZ!Box ändern. |
| „Gerätesteuerung erlauben“ | `virtual_desktop.infra.allow_control` | aus | Energieaktionen: Starten, Stoppen, Neustarten, Herunterfahren, Neustart und Wake-on-LAN. |

Quellenänderungen und Gerätesteuerung brauchen jeweils mehr als ihren eigenen Schalter:

- Der Schalter **Schreibgeschützter Modus** des Desktops (`virtual_desktop.readonly`) sperrt alle Schreibvorgänge.
- Quellenänderungen brauchen eine beschreibbare Quelle: aktiviert und nicht schreibgeschützt (`docker.readonly`,
  `proxmox.readonly`, `fritzbox.network.readonly`).
- Gerätesteuerung braucht für jede Aktion die Berechtigung der Quelle. Siehe [Energieaktionen](#energieaktionen).

Für diese Schalter ist kein Neustart nötig. Infra einzuschalten schaltet Docker, Proxmox oder FRITZ!Box nicht ein.

---

## Erste Schritte

1. Schalte den Desktop und Infra ein, wie oben beschrieben, und öffne den Desktop im Browser.
2. Öffne **Infra** im Startmenü, in der Kategorie **System**. Das Fenster öffnet sich maximiert.
3. Eine leere Karte bietet **Ersten Knoten hinzufügen** und **Vorhandene Assets importieren** an. Beginne mit dem Import.

---

## Die Karte aufbauen

Infra legt nie von selbst einen Eintrag an. Du erstellst Einträge auf zwei Wegen.

**Aus einer Quelle importieren.** Klicke in der Werkzeugleiste auf **Importieren** oder wähle es im Fenstermenü
**Infra**. Der Dialog listet neue Kandidaten aus diesen Quellen:

| Quelle | Verfügbar, wenn |
|---|---|
| Geräteregister | Immer, aus der Inventar-Datenbank |
| Docker | `docker.enabled` ist an |
| Proxmox | `proxmox.enabled` ist an |
| FRITZ!Box-Hosts | FRITZ!Box ist aktiviert mit einem Host, und die Funktion Hosts im Netzwerk ist an |

Andere Integrationen (Home Assistant, TrueNAS, Tailscale) füllen Infra nicht. Siehe [Kapitel 8: Integrationen](./08-integrations.md).

Markiere die Kandidaten, lass **Als neuen Knoten importieren** stehen oder wähle **Mit vorhandenem Asset verknüpfen**,
und klicke auf **Auswahl importieren**. Gleiche Namen oder Adressen schlagen nur eine Verknüpfung vor. Einträge aus dem Geräteregister und von FRITZ!Box bleiben
verborgen, solange ihr Name leer ist, ein Standardname (`unnamed device`, `Fritz!Box host`) oder nur eine Hex-ID, UUID
oder MAC-Adresse ist, bis die Quelle einen lesbaren Namen liefert.

**Einen manuellen Eintrag anlegen.** Klicke auf **Knoten hinzufügen** (oder **Infrastrukturknoten hinzufügen** im
Fenstermenü) und gib Namen, Typ (Gerät, Virtuelle Maschine, Container, Dienst, Netzwerk, Standort, Raum, Rack oder Gruppe)
und Icon ein. Ein manuelles **Gerät** legt außerdem eine Zeile im Geräteregister mit dem Protokoll `none` an. SSH, VNC und
Zugangsdaten trägst du dort bewusst ein. Die anderen Typen legen keine Registergeräte an.

---

## Ansichten, Orte und Beziehungen

**Atlas**, **Topologie** und **Inventar** zeigen dieselben Einträge. Der Atlas ist eine Rasterkarte. Die Topologie ordnet
Einträge in Ebenen: zuerst Netzwerk und Orte, dann Geräte und Gruppen, dann virtuelle Maschinen und Container, zuletzt Dienste.

Der Lauf ist keine vierte Ansicht. Er ist eine Anzeige an jedem Eintrag: *Läuft*, *Gestoppt*, *Pausiert*, *Unbekannt*
oder *Keine Laufangabe*. Die Seitenleiste grenzt auf **Alle Assets**, **Nicht platziert** oder **Benötigt Aufmerksamkeit** ein.
Die Werkzeugleiste filtert nach Typ, Quelle und Lauf.

**Orte** sind Standorte, Räume, Racks und Gruppen. Sie stehen in der Seitenleiste unter der Überschrift **Behälter**. Ein
Ort mit platzierten Kindern zieht auf Atlas und Topologie einen Rahmen um sie.

**Platzieren.** Nicht platzierte Einträge liegen im Ablagefach unter der Karte. Ziehe sie auf die Karte oder wähle **In der
Mitte platzieren**, **Oben links platzieren** oder **Mit Tastatur bewegen**. Pfeiltasten und Enter speichern die Position.

**Anordnen** legt die Karte neu an (Raster im Atlas, Ebenen in der Topologie). Angeheftete Einträge bleiben stehen.
**Rückgängig** macht die letzte Layoutänderung rückgängig.

**Beziehungen.** Klicke auf **Verbinden** und wähle zwei Einträge, oder nutze **Beziehung hinzufügen** im Detaileditor. Typen
sind *Ethernet*, *Glasfaser*, *WLAN*, *Mitglied von*, *Läuft auf*, *Abhängig von* und *Versorgt durch*. Ein gemeinsames
Subnetz ist kein Kabel. Auf dem Atlas klappst du mit einem geöffneten Host die Dienste auf, die ausdrücklich mit ihm
verknüpft sind.

---

## Einträge bearbeiten und speichern

Der Detaileditor besteht aus: **Identität**, **Hardware & Netzwerk** (Rolle, Hersteller, Modell, Seriennummer,
Betriebssystem, Firmware, CPU, Arbeitsspeicher, Speicher, Rackposition, Adressen, Tags, Notizen, Schnittstellen, Links,
benutzerdefinierte Felder), **Lokale App-Verknüpfungen**, **Platzierung**, **Beobachtete Quellen** und **Beziehungen**.

**Speichern** legt deine Änderungen in Infra ab. Der Knopf bleibt aus, solange nichts geändert ist. Er ändert nie ein
Registergerät oder dessen Ziel für die Schnellverbindung. Dafür nutzt du die Quellenänderungen. **Verwerfen** wirft den
Entwurf weg. Schließt du Infra mit ungespeicherten Änderungen, fragt es, ob du sie verwerfen oder weiter bearbeiten willst.

Hat jemand den Eintrag inzwischen gespeichert, lehnt Infra dein Speichern ab und behält deinen Entwurf. Klicke auf
**Aktualisieren**, vergleiche unter **Aktuelle Asset-Werte prüfen** und wähle **Entwurf behalten und erneut anwenden**.

**Aus Infra entfernen** löscht nur den Infra-Eintrag. Das Quellgerät, der Container oder die VM bleiben bestehen, ebenso
ein Registergerät, das ein manueller Eintrag angelegt hat. Lösche es im Geräteregister, wenn du es nicht mehr brauchst.
Ein Eintrag mit Kindern lässt sich erst entfernen, wenn du sie verschoben oder entfernt hast.

---

## Quellenänderungen

Quellenänderungen ändern die Metadaten, die die Quelle selbst speichert. Du brauchst **Quellenänderungen erlauben** und
eine beschreibbare Quelle. Der Knopf **Quelle bearbeiten** zeigt nur die Felder, die die Quelle unterstützt:

| Quelle | Bearbeitbare Felder |
|---|---|
| Geräteregister | Name, Typ, Protokoll (*Keins*, *SSH*, *VNC*), IP-Adresse, Port, Benutzername, Anmeldedaten (aus der Liste), Beschreibung, Tags, MAC-Adresse |
| Docker | Containername. Docker benennt den Container um, er wird nicht neu erstellt. |
| Proxmox | Name bei einer VM, Hostname bei einem Container (LXC), Beschreibung, Tags |
| FRITZ!Box-Hosts | Anzeigename |

**Quellmetadaten speichern** schreibt die Änderung und liest sie zur Bestätigung zurück. Eine Proxmox-Änderung kann *ausstehend*
bleiben, bis Proxmox sie bestätigt. Eine Änderung, die inzwischen woanders passiert ist, wird über eine Konfigurationsprüfsumme
abgelehnt statt überschrieben. Von AuraGo verwaltete Container lassen sich hier nicht umbenennen.

---

## Energieaktionen

Energieaktionen schicken eine einzelne, ausdrücklich gewollte Anfrage an eine Quelle. Öffne die Quellenzeile im
Detaileditor und wähle **Infrastrukturaktionen**.

| Quelle | Aktion | Bestätigung | Zusätzlich nötig |
|---|---|---|---|
| Docker-Container | Starten | Nein | `docker.enabled` an, `docker.readonly` aus |
| Docker-Container | Stoppen, Neustart | Ja | wie bei Starten |
| Proxmox-VM oder Container | Starten | Nein | `proxmox.enabled` an, `proxmox.readonly` aus |
| Proxmox-VM oder Container | Herunterfahren, Neustart | Ja | außerdem `proxmox.allow_destructive: true` |
| FRITZ!Box-Host | Aufwecken | Nein | `fritzbox.network.sub_features.wake_on_lan` an, Netzwerk nicht schreibgeschützt |

Stoppen, Neustarten, Herunterfahren und Neustart fragen vorher nach („Stoppen für web bestätigen?“); der Server erzwingt das
ebenfalls. Von AuraGo verwaltete Container (zum Beispiel go2rtc, RuView, Security Proxy, Decisions, lokales LLM, ACE-Step,
Garage, Homepage, CYD Creator, Ollama, Piper, Browser Automation und AuraGo selbst) werden angezeigt, lassen sich aber nicht starten, stoppen, neu starten oder umbenennen.

Jede Aktion endet mit einem dieser Ausgänge:

- **Läuft**: Die Anfrage ist noch unterwegs.
- **Angenommen**: Proxmox nahm die Aufgabe an, oder das Wake-on-LAN-Paket ging raus. Der Gerätestatus ist damit nicht bestätigt.
- **Abgeschlossen**: Docker hat den Container erneut gelesen, der neue Zustand ist bestätigt.
- **Fehlgeschlagen**: Infra hat die Aktion nicht ausgeführt, oder die Quelle hat sie abgelehnt.
- **Unbekannter Ausgang**: Die Anfrage hat die Quelle vielleicht erreicht, vielleicht auch nicht.

Nach einem unbekannten Ausgang blockiert Infra diese Aktion für diesen Eintrag. Prüfe das Gerät in seinem eigenen Werkzeug, dann
klicke auf **Gerätestatus geprüft · einen neuen Versuch erlauben**. Infra wiederholt unklare Aktionen nie von selbst. Ein
Neuladen hebt die Sperre auf, prüfe das Gerät also erneut. Entzogene Berechtigungen brechen nicht gesendete Aktionen ab.
Eine gesendete Aktion lässt sich nicht zurückholen.

**Nachvollziehbarkeit.** Infra führt keinen Audit-Trail und schreibt keinen Logeintrag für Aktionen. Das Aktionsregister
speichert Anfrage-ID, Aktion, Ausgang und Proxmox-Task-ID, nicht den Benutzer. Wer was tat, steht in den Quellprotokollen.

---

## Gut zu wissen

- **Eine Karte für alle Administratoren.** Positionen und Beziehungen gelten für die ganze Instanz. Zoom und Ansicht gehören
  dem Fenster.
- **Der Lauf ist die letzte Beobachtung.** Eine veraltete Quelle hält keinen alten Laufwert fest. Bei FRITZ!Box heißt
  *Läuft*, dass der Host im LAN aktiv war, nicht dass Wake-on-LAN abgeschlossen ist.
- **Fehlen heißt nicht weg.** Nicht erreichbare oder nur teilweise antwortende Quellen machen Einträge *Veraltet*. Nur eine
  vollständige Antwort markiert einen Eintrag als *Entfernt*.

**Grenzwerte.** Infra erzwingt diese Höchstwerte:

| Element | Grenze |
|---|---|
| Name eines Eintrags | 200 Zeichen |
| Textfelder der Hardware (Hersteller, Modell, Betriebssystem usw.) | je 256 Zeichen; Notizen 8 KiB |
| Tags | 64 je Eintrag, je 64 Zeichen |
| Schnittstellen, Links, App-Verknüpfungen | je Eintrag 32, 32 und 64 |
| Benutzerdefinierte Felder | 64 je Eintrag, Werte bis 2048 Byte |
| Einträge je Quelle und Aktualisierung | 4096 (mehr hält die Quelle veraltet) |

---

## Datenschutz und Sicherheit

**Wo es gespeichert wird.** Die Karte liegt in der Inventar-Datenbank `data/inventory.db` (Config-Schlüssel
`sqlite.inventory_path`), in Tabellen `infra_*` neben dem Geräteregister. Vor der ersten Schema-Migration kopiert AuraGo
die Datenbank nach `inventory.db.infra-v<Version>-<zufällig>.bak` und behält die drei neuesten Kopien.

**Was abgefragt wird.** Beim Öffnen zeigt Infra sofort die gespeicherte Karte. Im Hintergrund liest es nur die konfigurierten
Quellen: das Geräteregister (lokal) sowie Docker, Proxmox und FRITZ!Box-Hosts, sofern aktiviert. Ein sichtbares Fenster
aktualisiert etwa alle 30 Sekunden. **Aktualisieren** wartet auf frische Ergebnisse.

**Was das Gerät verlässt.** Infra hat keinen eigenen externen Dienst. Der einzige Datenverkehr sind die Aufrufe, die die
Integrationen Docker, Proxmox und FRITZ!Box ohnehin an die von dir konfigurierten Endpunkte machen. Infra speichert eine
verkürzte Kopie: Namen, Typen, Zustand, Adressen, Hardware-Werte, Laufzustand, Tags, Container-Image-Namen, Proxmox-VM-IDs
und Knoten. Zugangsdaten, rohe Antworten der Anbieter und Docker-Umgebungsvariablen fehlen. Links müssen HTTP- oder HTTPS-Adressen ohne
Benutzername und Passwort sein.

**Zugangsdaten und Zugriff.** Ein Registergerät speichert nur einen Verweis auf die Anmeldedaten. Das Geheimnis bleibt im
Vault und in den Einstellungen der Integration. Nur Administratoren nutzen Infra. Der Agent hat kein Infra-Werkzeug und kann
den Atlas nicht lesen oder ändern. Seine Docker- und Proxmox-Werkzeuge behalten ihre eigenen Einstellungen
([Kapitel 8](./08-integrations.md)).

---

## Wenn etwas nicht klappt

| Meldung | Was hilft |
|---|---|
| „Infra ist deaktiviert oder nicht verfügbar. Eine Administration kann es in den Desktop-Einstellungen aktivieren.“ | Infra öffnen können nur Administratoren. Ein Admin schaltet **Virtuellen Desktop aktivieren** und **Infra aktivieren** unter **Config → Virtueller Desktop** ein. Die Meldung spricht von „Desktop-Einstellungen“, die Schalter stehen aber dort. |
| „Infra ist schreibgeschützt.“ | Ein Admin schaltet **Schreibgeschützt** unter **Config → Virtueller Desktop** aus und prüft, dass **Schreibgeschützter Modus** ebenfalls aus ist. |
| „Der Infra-Speicher ist nicht verfügbar.“ | Die Inventar-Datenbank lässt sich nicht nutzen. Ein Admin prüft `sqlite.inventory_path` und das AuraGo-Protokoll. |
| „Dieses Asset wurde anderswo geändert. Dein Entwurf ist noch da; aktualisiere, um die neueste Version zu prüfen.“ | **Aktualisieren** klicken, unter **Aktuelle Asset-Werte prüfen** vergleichen, dann **Entwurf behalten und erneut anwenden** wählen. |
| „Dieses Asset wurde an anderer Stelle entfernt. Dein lokaler Entwurf bleibt erhalten.“ | Den Entwurf verwerfen oder den Eintrag mit denselben Angaben neu anlegen. |
| „Die Quelle hat die Änderung nicht bestätigt. Dein Entwurf bleibt erhalten; prüfe die Quelle vor einem erneuten Versuch.“ | Den Wert in Docker, Proxmox oder FRITZ!Box prüfen, dann **Aktuelle Quelle prüfen** klicken, bevor du erneut speicherst. |
| „Die Quellenänderung steht weiterhin aus. Dein Entwurf bleibt hier erhalten.“ | Warten, bis Proxmox die Änderung bestätigt, dann **Ausstehenden Quellenstatus prüfen** klicken. |
| „Das Ergebnis der Aktion ist unklar. Prüfe den Gerätestatus, bevor du es erneut versuchst.“ | Das Gerät im eigenen Werkzeug prüfen, dann **Gerätestatus geprüft · einen neuen Versuch erlauben** klicken. |
| „Quellbearbeitung ist deaktiviert.“ | Ein Admin schaltet **Quellenänderungen erlauben** ein. |
| „Steuerungsaktionen sind deaktiviert.“ | Ein Admin schaltet **Gerätesteuerung erlauben** ein. |
| „Diese Quelle ist deaktiviert.“ | Die Integration hinter der Quelle ist in ihrer eigenen Konfiguration ausgeschaltet. |
| „Die Richtlinie der Quelle verhindert diese Aktion.“ | Für Herunterfahren oder Neustart einer Proxmox-VM setzt ein Admin `proxmox.allow_destructive: true`. |
| „Dieser Container wird von AuraGo verwaltet und kann hier nicht umbenannt werden.“ | AuraGo verwaltet diesen Container, lass den Namen also in Ruhe. Derselbe Text erscheint, wenn eine Energieaktion darauf blockiert wird. |
| „Keine Importkandidaten gefunden“ | Die konfigurierten Quellen haben nichts Neues. Prüfe, ob die Quelle aktiviert ist und ihre Einträge lesbare Namen haben. |
| „Verschiebe oder entferne zuerst untergeordnete Assets.“ | Der Eintrag enthält noch Orte oder Einträge. Verschiebe oder entferne sie zuerst. |
| „Infra konnte die Anfrage nicht ausführen.“ | Sammelmeldung für Fehlercodes ohne eigenen Text, etwa eine abgelehnte Aktion. Prüfe die Schalter oben und versuche es erneut. Die [Betriebsreferenz](../../infra.md) listet die Codes. |

---

**Siehe auch:** [Kapitel 4: Web-Oberfläche](./04-webui.md) · [Kapitel 8: Integrationen](./08-integrations.md) · [Kapitel 14: Sicherheit](./14-security.md) · [Kapitel 16: Troubleshooting](./16-troubleshooting.md)

---

**Vorheriges Kapitel:** [Kapitel 28: Mailer](./28-mailer.md)
**Nächstes Kapitel:** [Kapitel 30: RuView](./30-ruview.md)
