# Kapitel 29: Infra – die Homelab-Karte

Infra ist eine gemeinsame Karte deines Homelabs. Sie zeigt Hardware, virtuelle Maschinen, Container, Dienste und
Netzwerksegmente als Karte, als Topologie-Diagramm oder als Liste und verknüpft jedes Asset mit seiner Quelle: dem
Geräteregister, Docker, Proxmox oder FRITZ!Box. Infra ist für Administratoren gedacht, die ein Gesamtbild ihrer
Umgebung wollen. Das Tool scannt dein Netzwerk nicht und überwacht Geräte nicht; ein Gerät ändert es nur, wenn du es
ausdrücklich verlangst.

Technische Details für Admins (API, Fehlercodes, Entwicklung) stehen in der [Betriebsreferenz](../../infra.md) (englisch).

## Inhaltsverzeichnis

1. [Was du brauchst und wie du es einschaltest](#was-du-brauchst-und-wie-du-es-einschaltest)
2. [Erste Schritte](#erste-schritte)
3. [Das Infra-Fenster](#das-infra-fenster)
4. [Funde importieren](#funde-importieren)
5. [Die Karte aufbauen](#die-karte-aufbauen)
6. [Topologie und Liste](#topologie-und-liste)
7. [Aufräumen und Ignorieren](#aufräumen-und-ignorieren)
8. [Assets bearbeiten und speichern](#assets-bearbeiten-und-speichern)
9. [Quellenänderungen](#quellenänderungen)
10. [Energieaktionen](#energieaktionen)
11. [Tastenkürzel](#tastenkürzel)
12. [Gut zu wissen](#gut-zu-wissen)
13. [Datenschutz und Sicherheit](#datenschutz-und-sicherheit)
14. [Wenn etwas nicht klappt](#wenn-etwas-nicht-klappt)

---

## Was du brauchst und wie du es einschaltest

Infra ist nur für Administratoren. Jede Infra-Anfrage braucht eine Desktop-Sitzung mit Administratorrechten. Zwei
Schalter müssen an sein:

- **Config → Virtueller Desktop → „Virtuellen Desktop aktivieren“** (`virtual_desktop.enabled`; die mitgelieferte `config_template.yaml` schaltet ihn ein,
  eine Config ohne den Schlüssel lässt ihn aus).
- **Config → Virtueller Desktop → „Infra aktivieren“** (`virtual_desktop.infra.enabled`, ab Werk an). Er steht in der
  Gruppe **Infra** auf dieser Seite. Schaltest du ihn aus, ist Infra für alle geschlossen. Die gespeicherte Karte
  bleibt auf der Platte.

Drei weitere Schalter stehen in derselben Gruppe. Sie sind bewusst getrennt:

| Schalter | Config-Schlüssel | Standard | Was er erlaubt |
|---|---|---|---|
| „Schreibgeschützt“ | `virtual_desktop.infra.read_only` | aus | Nur ansehen. Sperrt lokale Änderungen, Quellenänderungen und Gerätesteuerung. |
| „Quellenänderungen erlauben“ | `virtual_desktop.infra.allow_source_edit` | aus | Unterstützte Felder im Geräteregister, bei Docker, Proxmox und FRITZ!Box ändern. |
| „Gerätesteuerung erlauben“ | `virtual_desktop.infra.allow_control` | aus | Energieaktionen: Starten, Stoppen, Neu starten, Herunterfahren und Aufwecken (Wake-on-LAN). |

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
3. Eine leere Karte zeigt **Starte deine Homelab-Karte** und drei Schritte: Quellen verbinden, die Funde importieren, die
   du verwalten willst, und Assets aus der Bibliothek auf die Karte ziehen. Die beiden Schaltflächen heißen
   **Funde importieren…** und **Neues Asset**. Beginne mit dem Import für Geräte, die schon existieren.

---

## Das Infra-Fenster

Das Fenster hat oben eine Befehlsleiste, links die **Bibliothek**, in der Mitte die Karte und rechts den Detailbereich.

- **Befehlsleiste:**
  - Ansicht umschalten: **Karte**, **Topologie** und **Liste**.
  - Suche: **Suchen oder springen…** (Strg+K). Unter 560 px Breite klappt die Suche zu einem Symbol zusammen, das das
    Feld über der Leiste öffnet.
  - **Filter** nach Typ, Quelle und Betriebszustand. Aktive Filter erscheinen als Chips, die du entfernen kannst. Passen sie nicht
    mehr alle in die Leiste, zeigt ein **+N**-Chip die restlichen.
  - Der Statuspunkt zeigt **Live**, **Synchronisiere…**, **Nicht aktuell** oder **Pausiert**.
  - **Rückgängig** und **Wiederholen**.
  - **Neues Asset** mit einem Menü daneben, das **Neuer Ort** und **Funde importieren…** anbietet.
  - **⋯** (Weitere Aktionen): **Aktualisieren**, **Anordnen** (in der Topologie **Automatisch anordnen**),
    **Tastenkürzel** und **Legende**. Die Befehle zeigen Symbole, Tastenhinweise rechts und Trennlinien zwischen den
    Gruppen. Auf der Karte heißt der Befehl **Auswahl anordnen**, wenn mehrere Assets eines Orts ausgewählt sind, sonst
    **Ganze Karte anordnen**. In der Liste sind **Anordnen** und **Legende** deaktiviert; der Hinweis dazu lautet „Nur in Karte und Topologie“.
- **Bibliotheksabschnitte:** **Zu platzieren** (in der Topologie ausgeblendet), **Auf der Karte** (in der Topologie
  **In der Topologie**), **Prüfung nötig**, **Aufräumen** und **Ignoriert**. Das **+** neben **Auf der Karte**
  legt einen Ort an.
- **Kartenwerkzeuge:** **Auswählen (V)**, **Karte verschieben (H)** und **Assets verbinden (C)**. Zoomen kannst du mit
  **Vergrößern**, **Verkleinern** und **Zoom zurücksetzen** (Klick auf den Prozentwert). Bei 25 % und 200 % sind die
  Zoom-Schaltflächen deaktiviert, und eine Zoomtaste meldet die Grenze einmal. **An Fenster anpassen (F)** zeigt alles.
  Die **Übersichtskarte** zeigt, wo du gerade bist. Sie blendet sich aus, solange die ganze Karte auf den Bildschirm
  passt; ihr Schalter zeigt dann nur einen Hinweis. Ist ein Werkzeug oder der Verschiebemodus aktiv, erscheint oben in
  der Mitte der Karte ein Hinweis, zum Beispiel **Von Asset zu Asset ziehen · Esc beendet**.
- **Detailbereich:** öffnet sich, sobald du ein Asset auswählst, und schließt sich mit Esc oder dem Schließen-Knopf.
  Verdeckt der Detailbereich das ausgewählte Asset, verschiebt sich die Karte gerade so weit, dass es sichtbar bleibt.
- **Schmale Fenster:** Unter 760 px öffnet sich die Bibliothek über der Karte, über den Schalter in der Befehlsleiste.
  Unter 1100 px legt sich der Detailbereich über die Karte.
- **Layout-Einstellungen:** Breite von Bibliothek und Detailbereich, eingeklappte Bibliothek, Übersichtskarte, Zoom und
  der gewählte Reiter bleiben in diesem Browser gespeichert.

---

## Funde importieren

Infra legt nie von selbst ein Asset an. Der Import öffnet sich neben der Karte, mit dem Titel **Importieren**. Er listet neue Funde aus diesen Quellen:

| Quelle | Verfügbar, wenn |
|---|---|
| Geräteregister | Immer, aus der Inventar-Datenbank |
| Docker | `docker.enabled` ist an |
| Proxmox | `proxmox.enabled` ist an |
| Fritz!Box-Hosts | FRITZ!Box ist aktiviert mit einem Host, und die Funktion Hosts im Netzwerk ist an |

Andere Integrationen (Home Assistant, TrueNAS, Tailscale) füllen Infra nicht. Siehe [Kapitel 8: Integrationen](./08-integrations.md).
Haben mehrere Docker-Engines dieselbe Art, heißen sie nach dem Host der Engine, etwa **Docker · aurago-test**, sobald
dieser Name bekannt ist. Sonst zählen sie als **Docker 1** und **Docker 2**. Proxmox und FRITZ!Box-Hosts bleiben bei Nummern.

- **Gruppen:** Die Funde sind in **Empfohlen** (beim Öffnen vorausgewählt), **Mögliche Duplikate** und
  **Vermutlich nicht relevant** gruppiert. Jede Zeile zeigt ihren Grund, zum Beispiel **Gleiche Adresse wie pve-01** oder
  **Gerade nicht aktiv**. Eine Gruppe ohne Funde erscheint nicht.
- **Zeilen:** Jede Zeile ist zuerst eingeklappt und zeigt Checkbox, Symbol, Name und eine Zeile mit Angaben. Mit dem
  Aufklappknopf der Zeile öffnest du die Verknüpfung, den Grund und die Knöpfe **Importieren** und **Ignorieren**.
  Mögliche Duplikate und Zeilen mit einem Fehler sind von Anfang an aufgeklappt.
- **Zeilenaktionen:** **Importieren** importiert eine Zeile. Mit **Mit vorhandenem Asset verknüpfen** verknüpfst du sie
  stattdessen mit einem Asset, das es schon gibt; die Vorgabe ist **Als neues Asset importieren**. **Ignorieren** blendet
  eine Zeile aus. Bei markierten Zeilen wirken **Auswahl importieren** und **Auswahl ignorieren**.
- **Ladefehler:** Lassen sich die Funde nicht laden, zeigt das Fenster **Funde konnten nicht geladen werden** mit dem
  Knopf **Erneut versuchen**. Die Meldung **Keine neuen Funde** erscheint erst nach einem erfolgreichen Laden ohne Funde.
- **Teilweiser Import:** Gelingt nicht jeder Fund, nennt ein Hinweis die Anzahl, zum Beispiel **1 Fund konnte nicht
  importiert werden**.
- **Per Ziehen importieren:** Ziehst du einen Fund auf die Karte, wird er in einem Schritt importiert und platziert.
  Landet er über einem Ort, gehört er danach zu diesem Ort; über dem Ort steht dann **In Serverraum ablegen**. Eine VM oder
  ein Container wird in seinen Host importiert.
- **Manuelles Asset:** **Neues Asset** öffnet einen Dialog mit dem Titel **Neues Asset** (für einen Ort **Neuer Ort**).
  Gib Namen, Typ und Symbol ein und wähle **Erstellen**. Fehlt der Name, erscheint **Gib einen Namen ein** unter dem
  Feld, und der Cursor springt dorthin. Ein manuelles **Gerät** legt außerdem eine Zeile im Geräteregister mit dem
  Protokoll `none` an. SSH, VNC und Zugangsdaten trägst du dort bewusst ein. Die anderen Typen legen keine Registergeräte an.
  Ein Klick neben den Dialog schließt ihn nur, solange du noch nichts eingetippt hast. Esc schließt ihn immer. Ist Infra
  schreibgeschützt, ist **Erstellen** deaktiviert, und der Dialog zeigt **Nur Lesezugriff: Speichern ist gerade nicht möglich**.

Registry- und FRITZ!Box-Funde, deren Name leer ist, einen Standardnamen (`unnamed device`, `Fritz!Box host`) hat oder nur
aus einer Hex-ID, UUID oder MAC-Adresse besteht, werden nicht als Funde angeboten. Gib ihnen in ihrer Quelle einen
erkennbaren Namen, dann stehen sie zur Verfügung. Dieser Filter blendet auch unberührte, nicht platzierte Importe mit
solchen Namen aus. Registrierungsdaten und gespeicherte Assets bleiben erhalten; bearbeitete, platzierte, gruppierte und
verbundene Assets bleiben sichtbar.

---

## Die Karte aufbauen

- **Platzieren:** Ziehe Zeilen aus **Zu platzieren** auf die Karte, oder wähle eine Zeile aus und drücke **P**, um sie in
  der Kartenmitte zu platzieren. Im Detailbereich bietet der Reiter **Übersicht** bei einem nicht platzierten Asset ohne
  Host **Auf der Karte platzieren** an. Ziehst du ein Asset aus einem Ort heraus, verliert es seinen Ort. Dasselbe geht über
  **An einen Ort verschieben…** im Kontextmenü.
- **Ziehen:** Während des Ziehens zeigt ein gestrichelter Umriss, wo das Asset landet; es rastet auf das 24-px-Raster ein.
  Ziehst du eine Zeile auf die Seitenleiste, bricht das Ziehen ab, und Infra meldet **Ziehen abgebrochen**. Wer ein Asset,
  eine Zeile oder einen Auswahlrahmen bis an den Kartenrand zieht, bewegt die Karte in diese Richtung weiter.
- **Orte:** Standorte, Räume, Racks und Gruppen. Einen Ort verschiebst du an seiner Kopfzeile, seine Größe änderst du an
  den Kanten. **Neuer Ort** legt einen an, ebenso das **+** neben **Auf der Karte**. Meldungen nennen einen Standort
  ebenfalls **Ort**, etwa **Ort erstellt**.
- **Gäste:** Eine VM oder ein Container liegt in seinem Host. Das Abzeichen des Hosts (zum Beispiel **2 Gäste · 1 laufend**)
  öffnet die Gästeliste. Ohne laufende Gäste steht dort **2 Gäste · gestoppt**. Beim Darüberfahren nennt der Tooltip die
  ersten Gäste.
- **Verbindungen:**
  - Fahre mit dem Mauszeiger über ein Asset und ziehe einen seiner Griffe auf ein anderes Asset, oder nutze das Werkzeug
    **Assets verbinden (C)**.
  - Das Popover **Neue Verbindung** schlägt **Ethernet** vor, bei Access Points und Mesh-Geräten **WLAN**, und bei einem
    Dienst als einem Ende **Abhängig von**.
  - Die Verbindungsarten sind **Ethernet**, **Glasfaser**, **WLAN**, **Mitglied von**, **Läuft auf**, **Abhängig von** und
    **Versorgt durch**. Ein gemeinsames Subnetz ist kein Kabel.
  - Wählst du ein Asset aus, werden seine Verbindungen hervorgehoben und die übrigen verblasst. In der Topologie erscheinen
    **Abhängig von**- und **Versorgt durch**-Verbindungen nur, solange eines ihrer Enden ausgewählt ist oder unter dem
    Mauszeiger liegt. Der Reiter **Verbindungen** im Detailbereich listet alle auf.
  - Ist das Asset das Ziel einer gerichteten Verbindung, nennt die Zeile die Gegenrichtung: **Hostet**, **Benötigt von**,
    **Versorgt** oder **Enthält**.
  - Eine Verbindung, die eine Quelle meldet, lässt sich weder bearbeiten noch entfernen. Ihre Zeile im Reiter
    **Verbindungen** zeigt stattdessen einen Hinweis wie **Von Docker gemeldet**.
- **Auswahl:** Umschalt- oder Strg-Klick nimmt ein Asset in die Auswahl auf oder entfernt es daraus. Umschalt-Ziehen über
  leere Fläche wählt einen Bereich. Ziehe ein ausgewähltes Asset, um die ganze Auswahl zu verschieben, oder drücke **M**,
  um sie mit den Pfeiltasten zu verschieben.
- **Rückgängig:** Rückgängig und Wiederholen speichern je Ansicht 50 Schritte. Platzieren aus der Bibliothek und Anordnen
  lassen sich zurücknehmen. Die Schaltflächen nennen den nächsten Schritt, zum Beispiel **Rückgängig: Verschieben (Strg+Z)**.
  Macht ein Speicherkonflikt frühere Schritte unmöglich, erscheint die Meldung **Frühere Schritte lassen sich nicht mehr
  rückgängig machen**.
- **Kontextmenü:** Rechtsklick oder Umschalt+F10 öffnet es: **Details anzeigen**, **An einen Ort verschieben…**,
  **Verbinden mit…**, **In Schnellverbindung öffnen**, die Energieaktionen des Assets als eigene Gruppe, **Von der Karte entfernen**
  (in der Topologie **Automatisch platzieren**), **Aus Infra entfernen** (mit dem Hinweis Entf) und, bei Assets mit
  Quellen, **Entfernen und ignorieren**.
- **Positionen nicht gespeichert:** Infra wiederholt einen Speicherkonflikt einmal selbst. Scheitert es erneut, bleiben
  die Karten mit **Positionen nicht gespeichert** markiert, daneben steht **Erneut versuchen**. Klicke darauf; hilft das
  nicht, klicke **Aktualisieren**.

---

## Topologie und Liste

**Topologie** zeigt vier Spuren über die ganze Breite: **Internet**, **Netzwerk**, **Server und Speicher** und
**Clients und IoT**. Die Spurbeschriftungen bleiben beim Verschieben am linken Rand stehen. Assets landen so in den Spuren:

- **Internet:** Modems, ONTs, Internet-Gateways und Cloud-Dienste.
- **Netzwerk:** Router, Firewalls, VPN, Switches, Access Points, Mesh-Geräte sowie LAN-, VLAN- und WLAN-Segmente.
- **Server und Speicher:** Server, NAS und Festplattengehäuse, Mini-PCs, Hypervisoren, Kubernetes, Datenbanken,
  Netzwerkvideorekorder, USV und Stromverteiler. Ein Asset, das Gäste betreibt, landet ebenfalls hier, wenn keine andere
  Regel passt.
- **Clients und IoT:** alles andere, etwa Kameras, Drucker, Fernseher, Telefone, Tablets, Laptops und Sensoren.
  Einplatinencomputer folgen ihrer Rolle oder ihren Gästen; ohne beides landen sie hier.

Die Zuordnung geschieht automatisch. Ziehst du ein Asset in eine andere Spur, bleibt es dort.

- **Automatisches Layout:** Assets ohne Platz in der Topologie werden automatisch angeordnet, verbundene Assets stehen
  nah beieinander. Alle sehen dasselbe Layout, und das Öffnen der Topologie speichert nichts. Gespeichert werden nur deine
  eigenen Verschiebungen und Anordnungen.
- **Automatisch anordnen** gibt die Auswahl oder alle Assets an das automatische Layout zurück. Das ist ein Schritt, den
  du rückgängig machen kannst.
- **Ein Asset zurücksetzen:** Im Kontextmenü gibt **Automatisch platzieren** ein einzelnes Asset an das automatische
  Layout zurück. Gäste bleiben in ihrem Host.
- **Linien:** Verbindungen zwischen Spuren sind Kurven. Verbindet eine Verbindung zwei Assets einer Spur und liegt ein
  anderes Asset dazwischen, bogen sie über die Reihe.
- **Herausgezoomt:** Karten werden kompakt und zeigen nur Symbol und Name. Ein zu langer Name endet mit „…“ in einer Zeile;
  der volle Name steht im Tooltip. Ein Name verschwindet nur, wenn er das Symbol einer anderen Karte überdeckt, und
  erscheint beim Darüberfahren, Fokussieren oder Auswählen wieder.

**Liste** zeigt dieselben Assets als sortierbare Tabelle. Schmale Fenster blenden Spalten aus, statt seitlich zu scrollen.
Dabei verschwinden zuerst **Betrieb**, **Hardware**, **Quelle**, **Ort**, **Typ**, **Adresse** und zuletzt **Status**. Der
Name bleibt immer sichtbar.

- Klicke einen Spaltenkopf an, um nach ihm zu sortieren (**Nach Name sortieren**). Alle Spalten lassen sich sortieren.
  Die Spalten sind **Name**, **Typ**, **Ort**, **Adresse**, **Hardware**, **Quelle**, **Status** und **Betrieb**. Gekürzte
  Texte zeigen den vollen Text im Tooltip.
- Die Spalte **Status** zeigt die Erreichbarkeit: **Live**, **Nicht erreichbar**, **Unbekannt** oder einen Strich, wenn
  kein Signal vorliegt. Ist eine Quelle veraltet oder entfernt, steht zusätzlich ein Warnhinweis „!“ dabei.
- Ein Klick irgendwo in einer Zeile öffnet das Asset im Detailbereich. Nur Checkbox, Aufklappknopf und Links sind
  ausgenommen. Ein Doppelklick auf ein nicht platziertes Asset öffnet es ebenfalls.
- Die Liste hat einen Tab-Stopp. Mit den Pfeiltasten Auf und Ab, **Pos1** und **Ende** wechselst du zwischen den Zeilen,
  **Leertaste** schaltet die Checkbox um, **Eingabe** öffnet das Asset. Die Pfeiltaste Rechts klappt einen Host auf, die
  Pfeiltaste Links klappt ihn wieder zu.
- Hosts klappen ihre Gäste auf (**Gäste von pve-01 anzeigen**).
- Markierst du Zeilen, erscheint die Leiste **Aktionen für die Auswahl** mit **An einen Ort verschieben…**,
  **Entfernen und ignorieren**, **Aus Infra entfernen** und **Auswahl aufheben**.

---

## Aufräumen und Ignorieren

- **Prüfung nötig:** Hier stehen Quellen, die veraltet, unbekannt oder nicht mehr gemeldet sind, und ungespeicherte
  Arbeit, etwa **Position nicht gespeichert** oder **Der Entwurf steht im Konflikt mit einer neueren Version**.
- **Aufräumen-Vorschläge:**
  - mögliche Duplikate: **Entfernen und ignorieren**;
  - Docker-Gäste ohne Host: **Docker-Host zuweisen**;
  - andere Gäste ohne Host: **Host wählen…**;
  - Assets mit unlesbarem Namen: **Umbenennen**.
- **Ausblenden:** Der Knopf **Ausblenden** blendet einen Vorschlag aus. **3 ausgeblendete Vorschläge anzeigen** holt
  sie zurück.
- **Entfernen und ignorieren:** löscht das Asset und ignoriert seine Quellen. Sie tauchen erst wieder als Fund auf, wenn du
  sie unter **Ignoriert** mit **Wiederherstellen** zurückholst.
- **Verknüpfte Quellen:** Eine Quelle, die noch mit einem Asset verknüpft ist, lässt sich nicht ignorieren. Löse zuerst die
  Verknüpfung oder entferne das Asset.

---

## Assets bearbeiten und speichern

- Wenn du ein Asset auswählst, öffnet sich sein Detailbereich. Der Kopf zeigt das Symbol (**Symbol ändern**), den Namen
  (anklicken, um ihn umzubenennen), den Ortspfad, Zustand und Betriebszustand sowie die Hauptaktion: **In Schnellverbindung öffnen** bei einer
  verknüpften App, sonst **Bearbeiten**.
- Der Detailbereich hat vier Reiter: **Übersicht**, **Details**, **Quellen** und **Verbindungen**.
- **Gäste im Reiter Übersicht:** Ein Host listet seine Gäste mit ihrem Betriebszustand. Ab acht Gästen blendet
  **+3 weitere** die übrigen in der Bibliothek ein.
- **Prüfung:** Ungültige CIDR-, Link-, IP-, MAC- oder VLAN-Angaben und fehlende Pflichtfelder meldet Infra unter dem Feld,
  sobald du das Feld verlässt oder speichern willst. **In Infra speichern** bleibt aktiv, solange etwas geändert ist. Ein Speichern
  mit ungültigen Feldern setzt den Cursor auf das erste und speichert nichts.
- **Gruppen im Reiter Details:** **Identität**, **Netzwerk**, **Hardware**, **Links und Apps**, **Notizen** und
  **Benutzerdefinierte Felder**. Leere Gruppen starten eingeklappt.
- **Speicherleiste:** Bei Änderungen erscheint **Ungespeicherte Änderungen** mit **Verwerfen** und **In Infra speichern**
  (oder Strg+S). Geänderte Felder sind markiert.
- **App-Verknüpfungen:** **App verknüpfen…** öffnet eine Suche. MeshCore-Unterhaltungen erscheinen nur bei Funk- und
  Mesh-Assets oder nachdem du zwei Zeichen eingegeben hast.
- **Konflikte:** Hat jemand dasselbe Asset inzwischen gespeichert, lehnt Infra dein Speichern ab und behält deinen Entwurf.
  Klicke auf **Aktualisieren**, vergleiche unter **Aktuelle Asset-Werte prüfen** und wähle **Entwurf erneut
  anwenden**, wenn deine Version gelten soll.
- **Aus Infra entfernen** löscht nur das Infra-Asset. Das Quellgerät, der Container oder die VM bleiben bestehen, ebenso ein
  Registergerät, das ein manuelles Asset angelegt hat. Lösche es im Geräteregister, wenn du es nicht mehr brauchst. Ein
  Asset mit untergeordneten Assets lässt sich erst entfernen, wenn du sie verschoben oder entfernt hast.
- Schließt du Infra mit ungespeicherten Änderungen, fragt es, ob du sie verwerfen (**Verwerfen**) oder weiter bearbeiten
  willst.
- Wechselst du das Asset oder schließt den Detailbereich mit ungespeicherten Änderungen, bleibt der Entwurf erhalten.
  Infra meldet das einmal: **Entwurf für TheBrain bleibt erhalten**.

---

## Quellenänderungen

Quellenänderungen ändern die Metadaten, die die Quelle selbst speichert. Du brauchst **Quellenänderungen erlauben** und
eine beschreibbare Quelle. Auf dem Reiter **Quellen** hat jede Quellenzeile im Detailbereich einen Knopf **Quelle
bearbeiten**, wo die Quelle das erlaubt. Das Formular zeigt nur die Felder, die die Quelle unterstützt:

| Quelle | Bearbeitbare Felder |
|---|---|
| Geräteregister | Name, Typ, Protokoll (*Keins*, *SSH*, *VNC*), IP-Adresse, Port, Benutzername, Anmeldedaten (aus der Liste oder **Keine Anmeldedaten**), Beschreibung, Tags, MAC-Adresse |
| Docker | Containername. Docker benennt den Container um, er wird nicht neu erstellt. |
| Proxmox | Name bei einer VM, Hostname bei einem Container (LXC), Beschreibung, Tags |
| Fritz!Box-Hosts | Anzeigename |

**Quellmetadaten speichern** schreibt die Änderung und liest sie zur Bestätigung zurück. Eine Proxmox-Änderung kann
*ausstehend* bleiben, bis Proxmox sie bestätigt. Bis dahin bleibt dein Entwurf hier, und Infra zeigt das an. Klicke auf
**Quellenstatus prüfen**, sobald Proxmox bestätigt hat. Eine Änderung, die inzwischen woanders passiert ist,
wird über eine Konfigurationsprüfsumme abgelehnt statt überschrieben. Von AuraGo verwaltete Container lassen sich hier
nicht umbenennen.

---

## Energieaktionen

Energieaktionen schicken eine einzelne, ausdrücklich gewollte Anfrage an eine Quelle. Öffne auf dem Reiter **Quellen** die
Quellenzeile und wähle **Infrastrukturaktionen**. Der Dialog heißt **Quellaktionen**.

| Quelle | Aktion | Bestätigung | Zusätzlich nötig |
|---|---|---|---|
| Docker-Container | Starten | Nein | `docker.enabled` an, `docker.readonly` aus |
| Docker-Container | Stoppen, Neu starten | Ja | wie bei Starten |
| Proxmox-VM oder Container | Starten | Nein | `proxmox.enabled` an, `proxmox.readonly` aus |
| Proxmox-VM oder Container | Herunterfahren, Neu starten | Ja | außerdem `proxmox.allow_destructive: true` |
| FRITZ!Box-Host | Aufwecken | Nein | `fritzbox.network.sub_features.wake_on_lan` an, Netzwerk nicht schreibgeschützt |

Stoppen, Neu starten und Herunterfahren fragen vorher nach („Stoppen für web bestätigen?“); der Server erzwingt das
ebenfalls. Von AuraGo verwaltete Container (zum Beispiel go2rtc, RuView, Security Proxy, Decisions, lokales LLM, ACE-Step,
Garage, Homepage, CYD Creator, Ollama, Piper, Browser Automation und AuraGo selbst) werden angezeigt, lassen sich aber nicht
starten, stoppen, neu starten oder umbenennen.

Jede Aktion endet mit einem dieser Ausgänge. Die Ausgänge **Abgeschlossen** und **Fehlgeschlagen** erscheinen zusätzlich als
Meldung mit Namen und Aktion, etwa **web: Neu starten abgeschlossen** oder **web: Neu starten fehlgeschlagen**:

- **Läuft**: Die Anfrage ist noch unterwegs.
- **Angenommen**: Proxmox nahm die Aufgabe an, oder das Wake-on-LAN-Paket ging raus. Der Gerätestatus ist damit nicht
  bestätigt.
- **Abgeschlossen**: Docker hat den Container erneut gelesen, der neue Zustand ist bestätigt.
- **Fehlgeschlagen**: Infra hat die Aktion nicht ausgeführt, oder die Quelle hat sie abgelehnt.
- **Unbekannter Ausgang**: Die Anfrage hat die Quelle vielleicht erreicht, vielleicht auch nicht.

Nach einem unbekannten Ausgang blockiert Infra diese Aktion für dieses Asset. Prüfe das Gerät in seinem eigenen Werkzeug,
dann klicke auf **Neuen Versuch erlauben**. Infra wiederholt unklare Aktionen nie von selbst.
Ein Neuladen hebt die Sperre auf, prüfe das Gerät also erneut. Entzogene Berechtigungen brechen nicht gesendete Aktionen
ab. Eine gesendete Aktion lässt sich nicht zurückholen.

**Nachvollziehbarkeit.** Infra führt keinen Audit-Trail und schreibt keinen Logeintrag für Aktionen. Das Aktionsregister
speichert Anfrage-ID, Aktion, Ausgang und Proxmox-Task-ID, nicht den Benutzer. Wer was tat, steht in den Quellprotokollen.

---

## Tastenkürzel

Strg ist ⌘ auf dem Mac. Die Liste findest du auch unter **⋯** → **Tastenkürzel**.

| Tasten | Aktion |
|---|---|
| Strg+K | Asset suchen oder anspringen |
| Strg+Z / Strg+Umschalt+Z (oder Strg+Y) | Rückgängig machen oder wiederholen |
| Strg+S | Geöffnete Details speichern |
| Esc | Abbrechen oder schließen. Ohne laufende Geste kehrt Esc zuerst zum Werkzeug **Auswählen** zurück und hebt danach die Auswahl auf |
| ? | Diese Liste anzeigen |
| Pfeiltasten | Zwischen Assets auf der Karte wechseln |
| Eingabe | Fokussiertes Asset auswählen und seine Details öffnen |
| Leertaste | Fokussiertes Asset zur Auswahl hinzufügen oder daraus entfernen |
| M | Auswahl verschieben: Pfeiltasten 24 px, mit Umschalt 8 px; Eingabe speichert, Esc bricht ab. Bei einem Ort wechselt R zwischen Verschieben und Größe ändern |
| P | Bibliothekszeile in der Kartenmitte platzieren |
| Strg+A | Alle sichtbaren Assets auswählen |
| Entf | Auswahl aus Infra entfernen. Ist eine Verbindung markiert und kein Asset, entfernt Entf die Verbindung |
| Umschalt+F10 oder Menütaste | Kontextmenü öffnen |
| F | Karte an das Fenster anpassen |
| + / - / 0 | Vergrößern, verkleinern oder auf 100 % zurücksetzen |
| V / H / C | Werkzeug Auswählen, Verschieben oder Verbinden |
| Umschalt+Ziehen | Einen Bereich auswählen |
| Leertaste+Ziehen, mittlere Maustaste | Karte verschieben |
| Strg+Mausrad, Zwei-Finger-Zoom | Am Mauszeiger zoomen |
| Pfeiltasten Auf/Ab, Pos1, Ende (Liste) | Zwischen den Zeilen der Liste wechseln |
| Pfeiltasten Rechts/Links (Liste) | Host aufklappen bzw. zuklappen |
| Leertaste / Eingabe (Liste) | Checkbox der Zeile umschalten / Asset öffnen |

---

## Gut zu wissen

- **Eine Karte für alle Administratoren.** Karte, Positionen und Verbindungen gelten für alle Administratoren dieser
  Instanz. Zoom, gewählte Ansicht und offene Fenster gehören jedem Fenster.
- **Positionen werden je Ansicht gespeichert.** Die Topologie wird für alle gleich berechnet. Das Öffnen der Topologie
  speichert nichts.
- **Der Betriebszustand ist die letzte Beobachtung.** Eine veraltete Quelle hält keinen alten Betriebszustand fest. Bei FRITZ!Box heißt
  *Läuft*, dass der Host im LAN aktiv war, nicht dass Wake-on-LAN abgeschlossen ist.
- **Fehlen heißt nicht weg.** Nicht erreichbare oder nur teilweise antwortende Quellen machen Assets *Veraltet*. Nur eine
  vollständige Antwort markiert ein Asset als *Entfernt*.
- **Layout-Einstellungen bleiben in diesem Browser.** Breiten, Zoom, gewählter Reiter, Sortierung der Liste und
  ausgeblendete Vorschläge werden nicht geteilt.

**Grenzwerte.** Infra erzwingt diese Höchstwerte:

| Element | Grenze |
|---|---|
| Name eines Assets | 200 Zeichen |
| Textfelder der Hardware (Hersteller, Modell, Betriebssystem usw.) | je 256 Zeichen; Notizen 8 KiB |
| Tags | 64 je Asset, je 64 Zeichen |
| Schnittstellen, Links, App-Verknüpfungen | je Asset 32, 32 und 64 |
| Benutzerdefinierte Felder | 64 je Asset, Werte bis 2048 Byte |
| Assets je Quelle und Aktualisierung | 4096 (mehr hält die Quelle veraltet) |

---

## Datenschutz und Sicherheit

**Wo es gespeichert wird.** Die Karte liegt in der Inventar-Datenbank `data/inventory.db` (Config-Schlüssel
`sqlite.inventory_path`), in Tabellen `infra_*` neben dem Geräteregister. Vor der ersten Schema-Migration kopiert AuraGo
die Datenbank nach `inventory.db.infra-v<Version>-<zufällig>.bak` und behält die drei neuesten Kopien. Ignorierte Quellen
bleiben gespeichert, damit sie nicht erneut als Fund auftauchen.

**Nur im Browser gespeichert.** Ansichtseinstellungen (Breiten der Bereiche, Zoom, gewählter Reiter, Sortierung der Liste,
ausgeblendete Vorschläge) liegen nur im Browser und nie auf dem Server.

**Was abgefragt wird.** Beim Öffnen zeigt Infra sofort die gespeicherte Karte. Im Hintergrund liest es nur die konfigurierten
Quellen: das Geräteregister (lokal) sowie Docker, Proxmox und FRITZ!Box-Hosts, sofern aktiviert. Ein sichtbares Fenster
aktualisiert etwa alle 30 Sekunden. **Aktualisieren** wartet auf frische Ergebnisse.

**Was das Gerät verlässt.** Infra hat keinen eigenen externen Dienst. Der einzige Datenverkehr sind die Aufrufe, die die
Integrationen Docker, Proxmox und FRITZ!Box ohnehin an die von dir konfigurierten Endpunkte machen. Infra speichert eine
verkürzte Kopie: Namen, Typen, Zustand, Adressen, Hardware-Werte, Betriebszustand, Tags, Container-Image-Namen, Proxmox-VM-IDs
und Proxmox-Hosts. Zugangsdaten, rohe Antworten der Anbieter und Docker-Umgebungsvariablen fehlen. Links müssen HTTP- oder
HTTPS-Adressen ohne Benutzername und Passwort sein.

**Zugangsdaten und Zugriff.** Ein Registergerät speichert nur einen Verweis auf die Anmeldedaten. Das Geheimnis bleibt im
Vault und in den Einstellungen der Integration. Nur Administratoren nutzen Infra. Der Agent hat kein Infra-Werkzeug und kann
die Karte nicht lesen oder ändern. Seine Docker- und Proxmox-Werkzeuge behalten ihre eigenen Einstellungen
([Kapitel 8](./08-integrations.md)).

---

## Wenn etwas nicht klappt

| Meldung | Was hilft |
|---|---|
| „Infra ist deaktiviert oder nicht verfügbar. Ein Administrator kann es unter Konfiguration → Virtueller Desktop → Infra aktivieren.“ | Infra ist ausgeschaltet. Ein Admin schaltet **Virtuellen Desktop aktivieren** und **Infra aktivieren** unter **Config → Virtueller Desktop** ein. |
| „Infra steht nur Administratoren zur Verfügung.“ | Deine Browser-Sitzung ist abgelaufen, oder das verwendete API-Token hat nicht den Scope `desktop:admin`. Melde dich neu an oder nutze ein Token mit diesem Scope. |
| „Infra ist schreibgeschützt.“ | Ein Admin schaltet **Schreibgeschützt** unter **Config → Virtueller Desktop** aus und prüft, dass **Schreibgeschützter Modus** ebenfalls aus ist. |
| „Der Infra-Speicher ist nicht verfügbar.“ | Die Inventar-Datenbank lässt sich nicht nutzen. Ein Admin prüft `sqlite.inventory_path` und das AuraGo-Protokoll. |
| „Dieses Asset wurde anderswo geändert. Dein Entwurf ist noch da; aktualisiere, um die neueste Version zu prüfen.“ | **Aktualisieren** klicken, unter **Aktuelle Asset-Werte prüfen** vergleichen, dann **Entwurf erneut anwenden** wählen. |
| „Dieses Asset wurde an anderer Stelle entfernt. Dein lokaler Entwurf bleibt erhalten.“ | Den Entwurf verwerfen oder das Asset mit denselben Angaben neu anlegen. |
| „Die Quelle hat die Änderung nicht bestätigt. Dein Entwurf bleibt erhalten; prüfe die Quelle vor einem erneuten Versuch.“ | Den Wert in Docker, Proxmox oder FRITZ!Box prüfen, dann **Aktuelle Quelle prüfen** klicken, bevor du erneut speicherst. |
| „Die Quellenänderung steht weiterhin aus. Dein Entwurf bleibt hier erhalten.“ | Warten, bis Proxmox die Änderung bestätigt, dann **Quellenstatus prüfen** klicken. |
| „Das Ergebnis der Aktion ist unklar. Prüfe den Gerätestatus, bevor du es erneut versuchst.“ | Das Gerät im eigenen Werkzeug prüfen, dann **Neuen Versuch erlauben** klicken. |
| „Quellbearbeitung ist deaktiviert.“ | Ein Admin schaltet **Quellenänderungen erlauben** ein. |
| „Steuerungsaktionen sind deaktiviert.“ | Ein Admin schaltet **Gerätesteuerung erlauben** ein. |
| „Diese Quelle ist deaktiviert.“ | Die Integration hinter der Quelle ist in ihrer eigenen Konfiguration ausgeschaltet. |
| „Die Richtlinie der Quelle verhindert diese Aktion.“ | Für Herunterfahren oder Neu starten einer Proxmox-VM setzt ein Admin `proxmox.allow_destructive: true`. |
| „Dieser Container wird von AuraGo verwaltet und kann hier nicht umbenannt werden.“ | AuraGo verwaltet diesen Container, lass den Namen also in Ruhe. Derselbe Text erscheint, wenn eine Energieaktion darauf blockiert wird. |
| „Keine neuen Funde“ | Die konfigurierten Quellen haben gerade nichts Neues. Prüfe, ob die Quelle aktiviert ist und ihre Assets lesbare Namen haben. |
| „Verschiebe oder entferne zuerst untergeordnete Assets.“ | Das Asset enthält noch Orte oder Assets. Verschiebe oder entferne sie zuerst. |
| „Positionen nicht gespeichert“ | Klicke auf **Erneut versuchen**. Hilft das nicht, klicke **Aktualisieren**. |
| „Frühere Schritte lassen sich nicht mehr rückgängig machen“ | Ein Speicherkonflikt hat die Karte neu geladen. Neue Schritte lassen sich wieder rückgängig machen. |
| „Funde konnten nicht geladen werden“ | Klicke im Importfenster auf **Erneut versuchen**. |
| „Kopieren nicht möglich“ | Der Browser hat das Kopieren verweigert. Markiere den Wert und kopiere ihn von Hand. |
| „Nur Lesezugriff: Speichern ist gerade nicht möglich“ | Der Dialog ist schreibgeschützt. Ein Admin schaltet **Schreibgeschützt** aus (siehe „Infra ist schreibgeschützt.“). |
| „Infra konnte nicht aktualisieren. Du siehst den letzten bekannten Stand.“ | Infra zeigt den letzten bekannten Stand und versucht es selbst erneut. Klicke **Aktualisieren**, um jetzt frische Ergebnisse zu holen. |
| „Diese Verbindung gibt es bereits.“ | Wähle ein anderes zweites Asset oder eine andere Verbindungsart. |
| „Infra konnte die Änderung nicht bestätigen. Der aktuelle Stand wurde neu geladen; prüfe ihn, bevor du es erneut versuchst.“ | Prüfe den aktuellen Stand des Assets und versuche es dann erneut. |
| „Diese Quelle ist bereits mit einem Infra-Asset verknüpft.“ | Löse die Verknüpfung oder entferne das Asset. |
| „Infra konnte die Anfrage nicht ausführen.“ | Sammelmeldung für Fehlercodes ohne eigenen Text, etwa eine abgelehnte Aktion. Prüfe die Schalter oben und versuche es erneut. Die [Betriebsreferenz](../../infra.md) listet die Codes. |

---

**Siehe auch:** [Kapitel 4: Web-Oberfläche](./04-webui.md) · [Kapitel 8: Integrationen](./08-integrations.md) · [Kapitel 14: Sicherheit](./14-security.md) · [Kapitel 16: Troubleshooting](./16-troubleshooting.md)

---

**Vorheriges Kapitel:** [Kapitel 28: Mailer](./28-mailer.md)
**Nächstes Kapitel:** [Kapitel 30: RuView](./30-ruview.md)
